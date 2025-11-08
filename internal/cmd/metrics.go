package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type metricsCommandOperations struct {
	baseOperations
	filter     string
	metricName string
}

func NewMetricsCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &metricsCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "metrics [metric-name]",
		Short: "Get application metrics",
		Long: `Get application metrics from Spring Boot Actuator.

Without arguments, lists all available metrics.
With a metric name argument, shows details for that specific metric.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd, args); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get metrics", operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.filter, "filter", "f", "", "Filter metrics by name pattern")

	return cmd
}

func (o *metricsCommandOperations) complete(cmd *cobra.Command, args []string) error {
	if err := o.baseOperations.complete(cmd); err != nil {
		return err
	}

	if len(args) >= 1 {
		o.metricName = args[0]
	}

	return nil
}

func (o *metricsCommandOperations) validate() error {
	return o.validatePods()
}

func (o *metricsCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

	if o.metricName != "" {
		return o.displayMetric(client)
	}
	return o.listMetrics(client)
}

func (o *metricsCommandOperations) listMetrics(client actuator.Client) error {
	metricsResponse, err := client.GetMetrics()
	if err != nil {
		return err
	}

	for _, name := range metricsResponse.Names {
		if o.filter == "" || strings.Contains(name, o.filter) {
			fmt.Println(name)
		}
	}

	return nil
}

func (o *metricsCommandOperations) displayMetric(client actuator.Client) error {
	metric, err := client.GetMetric(o.metricName)
	if err != nil {
		return err
	}

	return displayMetricFormatted(metric)
}

func displayMetricFormatted(metric *actuator.MetricResponse) error {
	w := newTableWriter()
	_, _ = fmt.Fprintf(w, "NAME\t%s\n", metric.Name)
	_, _ = fmt.Fprintf(w, "DESCRIPTION\t%s\n", metric.Description)
	_, _ = fmt.Fprintf(w, "BASE UNIT\t%s\n", metric.BaseUnit)
	_ = w.Flush()
	fmt.Println()

	fmt.Println("MEASUREMENTS")
	w = newTableWriter()
	_, _ = fmt.Fprintln(w, "STATISTIC\tVALUE")
	for _, m := range metric.Measurements {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", m.Statistic, formatMetricValue(m.Value, metric.BaseUnit))
	}
	_ = w.Flush()

	if len(metric.AvailableTags) > 0 {
		fmt.Println()
		fmt.Println("AVAILABLE TAGS")
		tagWriter := newTableWriter()
		_, _ = fmt.Fprintln(tagWriter, "TAG\tVALUES")
		for _, tag := range metric.AvailableTags {
			_, _ = fmt.Fprintf(tagWriter, "%s\t%s\n", tag.Tag, strings.Join(tag.Values, ", "))
		}
		_ = tagWriter.Flush()
	}

	return nil
}

func formatMetricValue(value float64, unit string) string {
	switch unit {
	case "bytes":
		return formatBytesHuman(value)
	case "seconds":
		return formatSecondsHuman(value)
	default:
		return fmt.Sprintf("%.2f", value)
	}
}
