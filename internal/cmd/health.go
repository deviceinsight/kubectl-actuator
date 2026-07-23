package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const maxHealthDetailsLength = 100

type healthCommandOperations struct {
	baseOperations
	output   string
	group    string
	sawNonUp bool
}

func NewHealthCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &healthCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "health [GROUP]",
		Short: "Get application health status",
		Long: `Get application health status from Spring Boot Actuator.

Displays the overall health status and individual health indicators.
With a GROUP argument, queries a health group (e.g. liveness, readiness)
or a single component instead.

Exit codes: 0 if every targeted pod is UP, 1 if at least one pod reports
a status other than UP, 2 if the check itself failed.`,
		Example: `  # Check health of all pods in a deployment
  kubectl actuator -d my-app health

  # Query the readiness health group
  kubectl actuator -d my-app health readiness

  # Include component details
  kubectl actuator -d my-app health -o wide`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			client, ok := operations.completionClient(cmd)
			if !ok {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			health, err := client.GetHealth("")
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return health.Groups, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				operations.group = args[0]
			}
			if err := operations.run(cmd); err != nil {
				if errors.Is(err, ErrInterrupted) {
					return err
				}
				return &ExitCodeError{Code: 2, Err: err}
			}
			if operations.sawNonUp {
				// The non-UP status is already on screen; exit 1 without a
				// redundant error line.
				cmd.SilenceErrors = true
				return &ExitCodeError{Code: 1}
			}
			return nil
		},
	}

	addOutputFlag(cmd, &operations.output, "", OutputFormatWide, OutputFormatJSON, OutputFormatYAML)

	return cmd
}

func (o *healthCommandOperations) run(cmd *cobra.Command) error {
	if err := o.validateFlags(); err != nil {
		return err
	}
	return o.runEndpoint(cmd, "get health", o.output, o.structuredForPod, o.runForPod)
}

func (o *healthCommandOperations) validateFlags() error {
	return validateOutputFormat(o.output, OutputFormatWide, OutputFormatJSON, OutputFormatYAML)
}

func (o *healthCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	data, err := client.GetHealthRaw(o.group)
	if err != nil {
		return nil, err
	}
	var probe struct {
		Status string `json:"status"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.Status != "" && probe.Status != "UP" {
		o.sawNonUp = true
	}
	return data, nil
}

func (o *healthCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	health, err := client.GetHealth(o.group)
	if err != nil {
		return err
	}

	if health.Status != "" && health.Status != "UP" {
		o.sawNonUp = true
	}

	return displayHealth(health, o.output == OutputFormatWide)
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

func displayHealth(health *actuator.HealthResponse, wide bool) error {
	w := newTableWriter()
	printRow := func(component, status, details string) {
		if wide {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", component, status, details)
		} else {
			_, _ = fmt.Fprintf(w, "%s\t%s\n", component, status)
		}
	}

	printRow("COMPONENT", "STATUS", "DETAILS")
	for _, entry := range collectComponents(health.Components, "") {
		printRow(entry.path, entry.status, truncateString(entry.details, maxHealthDetailsLength))
	}
	printRow("[overall]", health.Status, "-")
	_ = w.Flush()

	printHealthGroups(health.Groups)

	return nil
}

func printHealthGroups(groups []string) {
	if len(groups) == 0 {
		return
	}
	fmt.Printf("\nGroups: %s\n", strings.Join(groups, ", "))
}
