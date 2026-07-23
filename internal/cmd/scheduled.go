package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const maxStatusMessageLength = 80

type scheduledTasksCommandOperations struct {
	baseOperations
	output string
}

func NewScheduledTasksCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &scheduledTasksCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:     "scheduledtasks",
		Aliases: []string{"scheduled-tasks"},
		Short:   "Get scheduled tasks",
		Long: `Get scheduled tasks from Spring Boot Actuator.

Displays scheduled tasks configured in your application. Execution
tracking (NEXT, LAST, STATUS) requires Spring Boot 3.4 or later.`,
		Example: `  # Show scheduled tasks with their schedules and last outcome
  kubectl actuator -d my-app scheduledtasks

  # Show full target names and untruncated error messages
  kubectl actuator -d my-app scheduledtasks -o wide`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.validateFlags(); err != nil {
				return err
			}
			return operations.runEndpoint(cmd, "get scheduled tasks", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	addOutputFlag(cmd, &operations.output, "", OutputFormatWide, OutputFormatJSON, OutputFormatYAML)

	return cmd
}

func (o *scheduledTasksCommandOperations) validateFlags() error {
	return validateOutputFormat(o.output, OutputFormatWide, OutputFormatJSON, OutputFormatYAML)
}

func (o *scheduledTasksCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	return client.GetRaw("scheduledtasks")
}

func (o *scheduledTasksCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	resp, err := client.GetScheduledTasks()
	if err != nil {
		return err
	}

	rows := buildTaskRows(resp, o.output == OutputFormatWide)
	printTaskRows(rows)
	return nil
}

type taskRow struct {
	Type     string
	Target   string
	Schedule string
	Next     string
	Last     string
	Status   string
}

func buildTaskRows(r *actuator.ScheduledTasksResponse, wide bool) []taskRow {
	var rows []taskRow
	for _, t := range r.Cron {
		rows = append(rows, taskRow{
			Type:     "cron",
			Target:   formatTaskTarget(t.Runnable.Target, wide),
			Schedule: fmt.Sprintf("cron(%s)", t.Expression),
			Next:     formatNextRun(t.NextExecution),
			Last:     formatLastRun(t.LastExecution),
			Status:   formatLastRunStatus(t.LastExecution, wide),
		})
	}
	for _, t := range r.FixedDelay {
		schedule := fmt.Sprintf("fixedDelay=%s", formatTaskInterval(t.Interval))
		if t.InitialDelay > 0 {
			schedule += fmt.Sprintf(" initialDelay=%s", formatTaskInterval(t.InitialDelay))
		}
		rows = append(rows, taskRow{
			Type:     "fixedDelay",
			Target:   formatTaskTarget(t.Runnable.Target, wide),
			Schedule: schedule,
			Next:     formatNextRun(t.NextExecution),
			Last:     formatLastRun(t.LastExecution),
			Status:   formatLastRunStatus(t.LastExecution, wide),
		})
	}
	for _, t := range r.FixedRate {
		schedule := fmt.Sprintf("fixedRate=%s", formatTaskInterval(t.Interval))
		if t.InitialDelay > 0 {
			schedule += fmt.Sprintf(" initialDelay=%s", formatTaskInterval(t.InitialDelay))
		}
		rows = append(rows, taskRow{
			Type:     "fixedRate",
			Target:   formatTaskTarget(t.Runnable.Target, wide),
			Schedule: schedule,
			Next:     formatNextRun(t.NextExecution),
			Last:     formatLastRun(t.LastExecution),
			Status:   formatLastRunStatus(t.LastExecution, wide),
		})
	}
	for _, t := range r.Custom {
		rows = append(rows, taskRow{
			Type:     "custom",
			Target:   formatTaskTarget(t.Runnable.Target, wide),
			Schedule: "-",
			Next:     formatNextRun(t.NextExecution),
			Last:     formatLastRun(t.LastExecution),
			Status:   formatLastRunStatus(t.LastExecution, wide),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Type == rows[j].Type {
			return rows[i].Target < rows[j].Target
		}
		return rows[i].Type < rows[j].Type
	})
	return rows
}

func formatTaskTarget(target string, wide bool) string {
	if wide {
		return target
	}
	parts := strings.Split(target, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return target
}

func printTaskRows(rows []taskRow) {
	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "TYPE\tTARGET\tSCHEDULE\tNEXT\tLAST\tSTATUS")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Type, r.Target, r.Schedule, r.Next, r.Last, r.Status)
	}
}

func parseTaskTime(s string) *time.Time {
	// RFC3339Nano also accepts timestamps without fractional seconds.
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return &t
	}
	_, _ = fmt.Fprintf(os.Stderr, "Warning: unable to parse time %q, expected RFC3339 format\n", s)
	return nil
}

// formatRelativeToNow renders an RFC3339 timestamp as its distance from now,
// e.g. "in 4m33s" or "27s ago". Unparseable timestamps pass through verbatim.
func formatRelativeToNow(timestamp string) string {
	if timestamp == "" {
		return "-"
	}
	t := parseTaskTime(timestamp)
	if t == nil {
		return timestamp
	}
	d := time.Until(*t)
	if d >= 0 {
		return "in " + formatDurationCompact(d)
	}
	return formatDurationCompact(-d) + " ago"
}

// formatNextRun and formatLastRun feed the NEXT and LAST columns; tasks that
// never ran (or Spring Boot < 3.4 without execution tracking) have neither.
func formatNextRun(next *actuator.TimeOnly) string {
	if next == nil {
		return "-"
	}
	return formatRelativeToNow(next.Time)
}

func formatLastRun(last *actuator.Execution) string {
	if last == nil {
		return "-"
	}
	return formatRelativeToNow(last.Time)
}

func formatLastRunStatus(ex *actuator.Execution, wide bool) string {
	if ex == nil {
		return "-"
	}
	if ex.Status == "ERROR" && ex.Exception != nil && ex.Exception.Message != "" {
		// Exception messages can contain newlines that would break the row.
		msg := escapeValue(ex.Exception.Message)
		if !wide {
			msg = truncateString(msg, maxStatusMessageLength)
		}
		return ex.Status + " - " + msg
	}
	if ex.Status == "" {
		return "-"
	}
	return ex.Status
}

func formatTaskInterval(ms int64) string {
	if ms == 0 {
		return "0s"
	}
	d := time.Duration(ms) * time.Millisecond
	return formatDurationCompact(d)
}
