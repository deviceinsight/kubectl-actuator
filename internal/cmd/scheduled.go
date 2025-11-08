package cmd

import (
	"context"
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
	output   string
	wideMode bool
}

func NewScheduledTasksCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &scheduledTasksCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "scheduled-tasks",
		Short: "Show scheduled tasks",
		Long: `Show scheduled tasks from Spring Boot Actuator.

Displays scheduled tasks configured in your application.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get scheduled tasks", operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.output, "output", "o", "", "Output format. One of: wide")

	return cmd
}

func (o *scheduledTasksCommandOperations) complete(cmd *cobra.Command) error {
	if err := o.baseOperations.complete(cmd); err != nil {
		return err
	}

	o.wideMode = o.output == OutputFormatWide

	return nil
}

func (o *scheduledTasksCommandOperations) validate() error {
	if err := o.validatePods(); err != nil {
		return err
	}
	return validateOutputFormat(o.output, OutputFormatWide)
}

func (o *scheduledTasksCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

	resp, err := client.GetScheduledTasks()
	if err != nil {
		return err
	}

	rows := buildRows(resp, o.wideMode)
	printRows(rows)
	return nil
}

type tableRow struct {
	Type     string
	Target   string
	Schedule string
	Next     string
	Last     string
	Status   string
}

func buildRows(r *actuator.ScheduledTasksResponse, wideMode bool) []tableRow {
	var rows []tableRow
	for _, t := range r.Cron {
		rows = append(rows, tableRow{
			Type:     "cron",
			Target:   formatTarget(t.Runnable.Target, wideMode),
			Schedule: fmt.Sprintf("cron(%s)", t.Expression),
			Next:     formatRelativeTime(t.NextExecution),
			Last:     formatRelativeTimeExec(t.LastExecution),
			Status:   formatStatus(t.LastExecution, wideMode),
		})
	}
	for _, t := range r.FixedDelay {
		schedule := fmt.Sprintf("fixedDelay=%s", formatMs(t.Interval))
		if t.InitialDelay > 0 {
			schedule += fmt.Sprintf(" initialDelay=%s", formatMs(t.InitialDelay))
		}
		rows = append(rows, tableRow{
			Type:     "fixedDelay",
			Target:   formatTarget(t.Runnable.Target, wideMode),
			Schedule: schedule,
			Next:     formatRelativeTime(t.NextExecution),
			Last:     formatRelativeTimeExec(t.LastExecution),
			Status:   formatStatus(t.LastExecution, wideMode),
		})
	}
	for _, t := range r.FixedRate {
		schedule := fmt.Sprintf("fixedRate=%s", formatMs(t.Interval))
		if t.InitialDelay > 0 {
			schedule += fmt.Sprintf(" initialDelay=%s", formatMs(t.InitialDelay))
		}
		rows = append(rows, tableRow{
			Type:     "fixedRate",
			Target:   formatTarget(t.Runnable.Target, wideMode),
			Schedule: schedule,
			Next:     formatRelativeTime(t.NextExecution),
			Last:     formatRelativeTimeExec(t.LastExecution),
			Status:   formatStatus(t.LastExecution, wideMode),
		})
	}
	for _, t := range r.Custom {
		rows = append(rows, tableRow{
			Type:     "custom",
			Target:   formatTarget(t.Runnable.Target, wideMode),
			Schedule: "-",
			Next:     formatRelativeTime(t.NextExecution),
			Last:     formatRelativeTimeExec(t.LastExecution),
			Status:   formatStatus(t.LastExecution, wideMode),
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

func formatTarget(target string, showFull bool) string {
	if showFull {
		return target
	}
	parts := strings.Split(target, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return target
}

func printRows(rows []tableRow) {
	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "TYPE\tTARGET\tSCHEDULE\tNEXT\tLAST\tSTATUS")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Type, r.Target, r.Schedule, r.Next, r.Last, r.Status)
	}
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return &t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	_, _ = fmt.Fprintf(os.Stderr, "Warning: unable to parse time %q, expected RFC3339 format\n", s)
	return nil
}

func formatRelativeTime(ti *actuator.TimeOnly) string {
	if ti == nil || ti.Time == "" {
		return "-"
	}
	if t := parseTime(ti.Time); t != nil {
		d := time.Until(*t)
		if d >= 0 {
			return "in " + formatDurationCompact(d)
		}
		return formatDurationCompact(-d) + " ago"
	}
	return ti.Time
}

func formatRelativeTimeExec(ex *actuator.Execution) string {
	if ex == nil || ex.Time == "" {
		return "-"
	}
	if t := parseTime(ex.Time); t != nil {
		d := time.Since(*t)
		if d >= 0 {
			return formatDurationCompact(d) + " ago"
		}
		return "in " + formatDurationCompact(-d)
	}
	return ex.Time
}

func formatStatus(ex *actuator.Execution, showFullStatus bool) string {
	if ex == nil {
		return "-"
	}
	if ex.Status == "ERROR" && ex.Exception != nil && ex.Exception.Message != "" {
		msg := ex.Exception.Message
		if !showFullStatus {
			runes := []rune(msg)
			if len(runes) > maxStatusMessageLength {
				msg = string(runes[:maxStatusMessageLength]) + "…"
			}
		}
		return ex.Status + " - " + msg
	}
	if ex.Status == "" {
		return "-"
	}
	return ex.Status
}

func formatMs(ms int64) string {
	if ms == 0 {
		return "0s"
	}
	d := time.Duration(ms) * time.Millisecond
	return formatDurationCompact(d)
}
