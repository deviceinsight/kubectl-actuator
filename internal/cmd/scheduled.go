package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/liggitt/tabwriter"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const maxStatusMessageLength = 80

type scheduledTasksOperations struct {
	k8sCliFlags      *genericclioptions.ConfigFlags
	k8sClient        k8s.Client
	transportFactory k8s.TransportFactory
	podResolver      PodResolver

	pods     []string
	output   string
	wideMode bool
}

func NewScheduledTasksCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &scheduledTasksOperations{k8sCliFlags: configFlags, podResolver: podResolver}

	cmd := &cobra.Command{
		Use:   "scheduled-tasks",
		Short: "Show scheduled tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return operations.run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&operations.output, "output", "o", "", "Output format. One of: wide")

	return cmd
}

func (o *scheduledTasksOperations) complete(cmd *cobra.Command) error {
	connection, err := k8s.NewK8sConnection(o.k8sCliFlags)
	if err != nil {
		return err
	}
	o.k8sClient = connection
	o.transportFactory = connection

	pods, err := o.podResolver(cmd.Context(), connection, cmd)
	if err != nil {
		return err
	}
	o.pods = pods

	o.wideMode = o.output == "wide"
	return nil
}

func (o *scheduledTasksOperations) validate() error {
	if len(o.pods) == 0 {
		return errors.New("No pods specified. Please specify at least one pod")
	}
	if o.output != "" && o.output != "wide" {
		return fmt.Errorf("invalid output format %q. Supported formats: wide", o.output)
	}
	return nil
}

func (o *scheduledTasksOperations) run(ctx context.Context) error {
	size := len(o.pods)
	for i, pod := range o.pods {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if size > 1 {
			fmt.Printf("%s:\n", pod)
		}

		err := o.printScheduledForPod(ctx, pod)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		if i != size-1 {
			fmt.Println()
		}
	}
	return nil
}

func (o *scheduledTasksOperations) printScheduledForPod(ctx context.Context, podName string) error {
	client, err := actuator.NewActuatorClient(ctx, o.transportFactory, o.k8sClient, podName)
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
	tw := tabwriter.NewWriter(os.Stdout, 6, 4, 3, ' ', 0)
	_, _ = fmt.Fprintln(tw, "TYPE\tTARGET\tSCHEDULE\tNEXT\tLAST\tSTATUS")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Type, r.Target, r.Schedule, r.Next, r.Last, r.Status)
	}
	_ = tw.Flush()
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
			return "in " + friendlyDuration(d)
		}
		return friendlyDuration(-d) + " ago"
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
			return friendlyDuration(d) + " ago"
		}
		return "in " + friendlyDuration(-d)
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
	return friendlyDuration(d)
}

func friendlyDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	secs := int64((d + time.Second/2) / time.Second)
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60

	out := ""
	if h > 0 {
		out += fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		out += fmt.Sprintf("%dm", m)
	}
	if s > 0 || out == "" {
		out += fmt.Sprintf("%ds", s)
	}
	return out
}
