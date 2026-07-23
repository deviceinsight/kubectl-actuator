package cmd

import (
	"encoding/json"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type rawCommandOperations struct {
	baseOperations
	endpoint string
	output   string
}

func NewRawCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &rawCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "raw ENDPOINT",
		Short: "Get raw response from any actuator endpoint",
		Long: `Get raw JSON response from any actuator endpoint.

Useful for accessing endpoints not directly supported by this tool,
or for scripting and automation.`,
		Example: `  # Fetch the request mappings endpoint
  kubectl actuator -d my-app raw mappings

  # Fetch a sub-resource of an endpoint
  kubectl actuator -d my-app raw loggers/com.example

  # List the endpoints the application exposes
  kubectl actuator -d my-app raw /`,
		Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			client, ok := operations.completionClient(cmd)
			if !ok {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			endpoints, err := client.GetAvailableEndpoints()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return endpoints, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			operations.parseArgs(args)
			if err := operations.validateFlags(); err != nil {
				return err
			}
			if err := operations.resolvePods(cmd); err != nil {
				return err
			}
			return operations.runStructured(cmd.Context(), operations.output, "get raw endpoint", operations.structuredForPod)
		},
	}

	// The default and the first allowed format are both json: raw output is
	// always structured.
	addOutputFlag(cmd, &operations.output, OutputFormatJSON, OutputFormatJSON, OutputFormatYAML)

	return cmd
}

func (o *rawCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	return client.GetRaw(o.endpoint)
}

func (o *rawCommandOperations) parseArgs(args []string) {
	if len(args) >= 1 {
		// Normalize "/" to "" - both should return the actuator index
		if args[0] == "/" {
			o.endpoint = ""
		} else {
			o.endpoint = args[0]
		}
	}
}

func (o *rawCommandOperations) validateFlags() error {
	return validateOutputFormat(o.output, OutputFormatJSON, OutputFormatYAML)
}
