package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type podResult struct {
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"data"`
	Error *string         `json:"error"`
}

type rawOutput struct {
	Pods []podResult `json:"pods"`
}

type rawCommandOperations struct {
	baseOperations
	endpoint string
}

func NewRawCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &rawCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "raw <endpoint>",
		Short: "Get raw response from any actuator endpoint",
		Long: `Get raw JSON response from any actuator endpoint.

Useful for accessing endpoints not directly supported by this tool,
or for scripting and automation.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd, args); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return operations.run(cmd.Context())
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return operations.validArgsEndpoint(cmd)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
	}

	return cmd
}

func (o *rawCommandOperations) complete(cmd *cobra.Command, args []string) error {
	if err := o.baseOperations.complete(cmd); err != nil {
		return err
	}

	if len(args) >= 1 {
		// Normalize "/" to "" - both should return the actuator index
		if args[0] == "/" {
			o.endpoint = ""
		} else {
			o.endpoint = args[0]
		}
	}

	return nil
}

func (o *rawCommandOperations) validate() error {
	return o.validatePods()
}

func (o *rawCommandOperations) run(ctx context.Context) error {
	output := rawOutput{Pods: make([]podResult, 0, len(o.pods))}

	for _, pod := range o.pods {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		result := podResult{Name: pod}

		client, err := o.actuatorClientFactory.NewClient(ctx, pod)
		if err != nil {
			errMsg := fmt.Sprintf("failed to create actuator client: %v", err)
			result.Error = &errMsg
			output.Pods = append(output.Pods, result)
			continue
		}

		data, err := client.GetRaw(o.endpoint)
		if err != nil {
			errMsg := err.Error()
			result.Error = &errMsg
			output.Pods = append(output.Pods, result)
			continue
		}

		result.Data = data
		output.Pods = append(output.Pods, result)
	}

	prettyJSON, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}

	fmt.Println(string(prettyJSON))
	return nil
}

func (o *rawCommandOperations) validArgsEndpoint(cmd *cobra.Command) ([]string, cobra.ShellCompDirective) {
	ctx := cmd.Context()

	connection, err := k8s.NewK8sConnection(o.k8sCliFlags)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	pods, err := o.podResolver(ctx, connection, cmd)
	if err != nil || len(pods) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	factory := NewActuatorClientFactory(connection, cmd)
	client, err := factory.NewClient(ctx, pods[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	endpoints, err := client.GetAvailableEndpoints()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return endpoints, cobra.ShellCompDirectiveNoFileComp
}
