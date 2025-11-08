package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type healthCommandOperations struct {
	baseOperations
	output string
}

func NewHealthCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &healthCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Get application health status",
		Long: `Get application health status from Spring Boot Actuator.

Displays the overall health status and individual health indicators.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get health", operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.output, "output", "o", "", "Output format. One of: wide")

	return cmd
}

func (o *healthCommandOperations) validate() error {
	if err := o.validatePods(); err != nil {
		return err
	}
	return validateOutputFormat(o.output, OutputFormatWide)
}

func (o *healthCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

	health, err := client.GetHealth()
	if err != nil {
		return err
	}

	if o.output == OutputFormatWide {
		return displayHealthWide(health)
	}
	return displayHealthTable(health)
}

type componentEntry struct {
	path    string
	status  string
	details string
}

func collectComponents(components map[string]actuator.HealthComponent, prefix string) []componentEntry {
	var entries []componentEntry
	collectComponentsRecursive(components, prefix, &entries)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	return entries
}

func collectComponentsRecursive(components map[string]actuator.HealthComponent, prefix string, entries *[]componentEntry) {
	for name, component := range components {
		path := name
		if prefix != "" {
			path = prefix + "/" + name
		}

		details := "-"
		if len(component.Details) > 0 {
			if detailsJSON, err := json.Marshal(component.Details); err == nil {
				details = string(detailsJSON)
			}
		}

		*entries = append(*entries, componentEntry{
			path:    path,
			status:  component.Status,
			details: details,
		})

		if len(component.Components) > 0 {
			collectComponentsRecursive(component.Components, path, entries)
		}
	}
}

func displayHealthTable(health *actuator.HealthResponse) error {
	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "COMPONENT\tSTATUS")

	entries := collectComponents(health.Components, "")

	for _, entry := range entries {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", entry.path, entry.status)
	}

	_, _ = fmt.Fprintf(w, "[overall]\t%s\n", health.Status)

	return nil
}

func displayHealthWide(health *actuator.HealthResponse) error {
	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "COMPONENT\tSTATUS\tDETAILS")

	entries := collectComponents(health.Components, "")

	for _, entry := range entries {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.path, entry.status, entry.details)
	}

	_, _ = fmt.Fprintf(w, "[overall]\t%s\t-\n", health.Status)

	return nil
}
