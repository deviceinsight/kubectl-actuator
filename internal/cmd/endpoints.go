package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type endpointsCommandOperations struct {
	baseOperations
	output string
}

func NewEndpointsCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &endpointsCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "List actuator endpoints and whether the application exposes them",
		Long: `List actuator endpoints and whether the application exposes them.

Reads the actuator index. AVAILABLE tells whether this application
exposes the endpoint; endpoints this plugin supports are always
listed, even when the application does not expose them. KUBECTL
ACTUATOR SUPPORT names the command that serves the endpoint, or '-'
when there is none. Any exposed endpoint can still be queried with
'raw'.`,
		Example: `  # See what a deployment's pods expose
  kubectl actuator -d my-app endpoints

  # Endpoint ids only, for scripting
  kubectl actuator -d my-app endpoints -o name`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.validateFlags(); err != nil {
				return err
			}
			return operations.runEndpoint(cmd, "get endpoints", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	addOutputFlag(cmd, &operations.output, "", OutputFormatName, OutputFormatJSON, OutputFormatYAML)

	return cmd
}

func (o *endpointsCommandOperations) validateFlags() error {
	return validateOutputFormat(o.output, OutputFormatName, OutputFormatJSON, OutputFormatYAML)
}

// structuredForPod passes the actuator index through: the raw discovery
// document is this command's structured form.
func (o *endpointsCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	return client.GetRaw("")
}

func (o *endpointsCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	endpoints, err := client.GetAvailableEndpoints()
	if err != nil {
		return err
	}

	if o.output == OutputFormatName {
		if len(endpoints) == 0 {
			_, _ = fmt.Fprintln(os.Stderr, "No endpoints found")
			return nil
		}
		sort.Strings(endpoints)
		for _, endpoint := range endpoints {
			fmt.Println(endpoint)
		}
		return nil
	}

	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "ENDPOINT\tAVAILABLE\tKUBECTL ACTUATOR SUPPORT")
	for _, row := range buildEndpointRows(endpoints) {
		_, _ = fmt.Fprintf(w, "%s\t%t\t%s\n", row.id, row.available, supportForEndpoint(row.id))
	}

	return nil
}

// endpointCommands maps actuator endpoint ids to the plugin command that
// serves them; ids not listed here are reachable via raw.
var endpointCommands = map[string]string{
	"beans":          "beans",
	"env":            "env",
	"health":         "health",
	"heapdump":       "heapdump",
	"info":           "info",
	"logfile":        "logfile",
	"loggers":        "logger",
	"metrics":        "metrics",
	"scheduledtasks": "scheduledtasks",
	"threaddump":     "threaddump",
}

// supportForEndpoint names the plugin command that serves an endpoint id,
// or "-" when the plugin has no dedicated command for it. The raw escape
// hatch does not count as support.
func supportForEndpoint(endpoint string) string {
	if cmd, ok := endpointCommands[endpoint]; ok {
		return cmd
	}
	return "-"
}

type endpointRow struct {
	id        string
	available bool
}

// buildEndpointRows merges the endpoints this plugin supports with what the
// application actually exposes: supported endpoints are always listed, even
// when the application does not expose them, and exposed endpoints are
// always listed, even without a dedicated command.
func buildEndpointRows(available []string) []endpointRow {
	availableSet := make(map[string]bool, len(available))
	for _, id := range available {
		availableSet[id] = true
	}

	ids := make(map[string]struct{}, len(endpointCommands)+len(available))
	for id := range endpointCommands {
		ids[id] = struct{}{}
	}
	for _, id := range available {
		ids[id] = struct{}{}
	}

	rows := make([]endpointRow, 0, len(ids))
	for id := range ids {
		rows = append(rows, endpointRow{id: id, available: availableSet[id]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	return rows
}
