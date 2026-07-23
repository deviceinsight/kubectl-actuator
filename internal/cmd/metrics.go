package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type metricsCommandOperations struct {
	baseOperations
	filter     string
	metricName string
	output     string
	tags       []string
}

func NewMetricsCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &metricsCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "metrics [METRIC]",
		Short: "Get application metrics",
		Long: `Get application metrics from Spring Boot Actuator.

Without arguments, lists all available metrics.
With a metric name argument, shows details for that specific metric.
Use --tag key=value to drill down into a metric's dimensions; the
available tags are listed in the metric's detail output.`,
		Example: `  # List all metric names
  kubectl actuator -d my-app metrics

  # Show one metric with its measurements and available tags
  kubectl actuator -d my-app metrics jvm.memory.used

  # Drill down into a tag dimension
  kubectl actuator -d my-app metrics jvm.memory.used --tag area=heap`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			client, ok := operations.completionClient(cmd)
			if !ok {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			metricsResponse, err := client.GetMetrics()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return metricsResponse.Names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 {
				operations.metricName = args[0]
			}
			if err := operations.validateFlags(); err != nil {
				return err
			}
			return operations.runEndpoint(cmd, "get metrics", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.filter, "filter", "f", "", "Filter metrics by name (case-insensitive substring)")
	addOutputFlag(cmd, &operations.output, "", OutputFormatJSON, OutputFormatYAML)
	markNoFileFlags(cmd, "filter")
	cmd.Flags().StringArrayVar(&operations.tags, "tag", nil, "Drill down by tag (key=value); repeatable")

	_ = cmd.RegisterFlagCompletionFunc("tag", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		client, ok := operations.completionClient(cmd)
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		metric, err := client.GetMetric(args[0], nil)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if key, _, found := strings.Cut(toComplete, "="); found {
			for _, tag := range metric.AvailableTags {
				if tag.Tag != key {
					continue
				}
				completions := make([]string, 0, len(tag.Values))
				for _, value := range tag.Values {
					completions = append(completions, key+"="+value)
				}
				return completions, cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		keys := make([]string, 0, len(metric.AvailableTags))
		for _, tag := range metric.AvailableTags {
			keys = append(keys, tag.Tag+"=")
		}
		return keys, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	})

	return cmd
}

func (o *metricsCommandOperations) validateFlags() error {
	if err := validateOutputFormat(o.output, OutputFormatJSON, OutputFormatYAML); err != nil {
		return err
	}

	if o.metricName != "" && o.filter != "" {
		return fmt.Errorf("--filter cannot be combined with a metric name argument")
	}

	if len(o.tags) > 0 && o.metricName == "" {
		return fmt.Errorf("--tag requires a metric name argument")
	}
	_, err := normalizeTags(o.tags)
	return err
}

// normalizeTag converts a key=value (or key:value) CLI tag into the
// key:value form the actuator API expects.
func normalizeTag(tag string) (string, error) {
	key, value, found := strings.Cut(tag, "=")
	if !found {
		key, value, found = strings.Cut(tag, ":")
	}
	if !found || key == "" || value == "" {
		return "", fmt.Errorf("invalid tag %q: expected key=value", tag)
	}
	return key + ":" + value, nil
}

// normalizeTags converts all --tag flags for the actuator API.
func normalizeTags(tags []string) ([]string, error) {
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		apiTag, err := normalizeTag(tag)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, apiTag)
	}
	return normalized, nil
}

func (o *metricsCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	if o.metricName != "" {
		apiTags, err := normalizeTags(o.tags)
		if err != nil {
			return nil, err
		}
		return client.GetRaw(actuator.MetricPath(o.metricName, apiTags))
	}
	data, err := client.GetRaw("metrics")
	if err != nil {
		return nil, err
	}
	return filterMetricNamesJSON(data, o.filter)
}

// filterMetricNamesJSON applies the metric name filter to the raw /metrics
// response, keeping all other fields intact.
func filterMetricNamesJSON(data json.RawMessage, filter string) (json.RawMessage, error) {
	if filter == "" {
		return data, nil
	}
	tree, err := decodeTree(data)
	if err != nil {
		return nil, err
	}
	names, ok := tree["names"].([]any)
	if !ok {
		return data, nil
	}
	filtered := make([]any, 0, len(names))
	for _, nameValue := range names {
		if name, ok := nameValue.(string); ok && matchesFilter(name, filter) {
			filtered = append(filtered, nameValue)
		}
	}
	tree["names"] = filtered
	return encodeTree(tree)
}

func (o *metricsCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
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

	matched := 0
	for _, name := range metricsResponse.Names {
		if matchesFilter(name, o.filter) {
			fmt.Println(name)
			matched++
		}
	}

	// Empty results are reported on stderr so "no match" is visible without
	// polluting pipes, and distinguishable from an app exposing no metrics.
	if matched == 0 {
		if o.filter != "" {
			_, _ = fmt.Fprintf(os.Stderr, "No metrics match filter %q\n", o.filter)
		} else {
			_, _ = fmt.Fprintln(os.Stderr, "No metrics found")
		}
	}

	return nil
}

func (o *metricsCommandOperations) displayMetric(client actuator.Client) error {
	apiTags, err := normalizeTags(o.tags)
	if err != nil {
		return err
	}

	metric, err := client.GetMetric(o.metricName, apiTags)
	if err != nil {
		return err
	}

	return displayMetricFormatted(metric, apiTags)
}

func displayMetricFormatted(metric *actuator.MetricResponse, appliedTags []string) error {
	w := newTableWriter()
	_, _ = fmt.Fprintf(w, "NAME\t%s\n", metric.Name)
	_, _ = fmt.Fprintf(w, "DESCRIPTION\t%s\n", metric.Description)
	_, _ = fmt.Fprintf(w, "BASE UNIT\t%s\n", metric.BaseUnit)
	if len(appliedTags) > 0 {
		_, _ = fmt.Fprintf(w, "TAGS\t%s\n", strings.Join(appliedTags, ", "))
	}
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
