package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// ErrNoPodsSelected is returned when no pods are selected via --pod, --deployment, or --selector flags
var ErrNoPodsSelected = errors.New("no pods selected: specify --pod, --deployment, or --selector")

// ErrSelectorMatchedNoPods is returned when a selector was provided but matched no pods
type ErrSelectorMatchedNoPods struct {
	Selectors []string
}

func (e *ErrSelectorMatchedNoPods) Error() string {
	return fmt.Sprintf("selector %s matched no pods", strings.Join(e.Selectors, ", "))
}

type PodResolver func(ctx context.Context, k8sClient k8s.Client, cmd *cobra.Command) ([]string, error)

func AddCommands(rootCmd *cobra.Command) {
	configFlags := genericclioptions.NewConfigFlags(true)

	// Add k8s config flags
	configFlags.AddFlags(rootCmd.PersistentFlags())
	rootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		// Keep namespace, context visible, hide others
		if flag.Name != "namespace" && flag.Name != "context" {
			flag.Hidden = true
		}
	})

	// Global target selection
	rootCmd.PersistentFlags().StringArrayP("pod", "p", nil, "Select target pod(s)")
	rootCmd.PersistentFlags().StringArrayP("deployment", "d", nil, "Select target deployment(s)")
	rootCmd.PersistentFlags().StringArrayP("selector", "l", nil, "Select target pod(s) by label selector")

	// Actuator configuration overrides
	rootCmd.PersistentFlags().IntP("port", "", 0, "Override actuator port")
	rootCmd.PersistentFlags().StringP("base-path", "", "", "Override actuator base path")

	// Shell completion
	_ = rootCmd.RegisterFlagCompletionFunc("pod", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		k8sClient, err := k8s.NewK8sConnection(configFlags)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		podNames, err := k8sClient.ListPods(cmd.Context(), k8sClient.Namespace(), "")
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return podNames, cobra.ShellCompDirectiveNoFileComp
	})

	_ = rootCmd.RegisterFlagCompletionFunc("deployment", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		k8sClient, err := k8s.NewK8sConnection(configFlags)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		deploymentNames, err := k8sClient.ListDeployments(cmd.Context(), k8sClient.Namespace())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return deploymentNames, cobra.ShellCompDirectiveNoFileComp
	})

	// Actuator subcommands
	rootCmd.AddCommand(NewLoggerCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewScheduledTasksCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewInfoCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewHealthCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewMetricsCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewEnvCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewThreadDumpCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewBeansCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewRawCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewVersionCommand())
}

// FlagsPodResolver resolves pods based on global --pod/--deployment/--selector flags
func FlagsPodResolver(ctx context.Context, k8sClient k8s.Client, cmd *cobra.Command) ([]string, error) {
	root := cmd.Root()
	pods, err := root.PersistentFlags().GetStringArray("pod")
	if err != nil {
		return nil, err
	}
	deployments, err := root.PersistentFlags().GetStringArray("deployment")
	if err != nil {
		return nil, err
	}
	selectors, err := root.PersistentFlags().GetStringArray("selector")
	if err != nil {
		return nil, err
	}

	// Track if any target selection was provided
	hasTargetSelection := len(pods) > 0 || len(deployments) > 0 || len(selectors) > 0

	// Expand deployments to pods
	for _, d := range deployments {
		names, err := k8sClient.GetDeploymentPods(ctx, k8sClient.Namespace(), d)
		if err != nil {
			return nil, err
		}
		pods = append(pods, names...)
	}

	// Expand selectors to pods
	var selectorsWithNoMatches []string
	for _, s := range selectors {
		names, err := k8sClient.ListPods(ctx, k8sClient.Namespace(), s)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			selectorsWithNoMatches = append(selectorsWithNoMatches, s)
		}
		pods = append(pods, names...)
	}

	// Deduplicate
	seen := map[string]struct{}{}
	var result []string
	for _, p := range pods {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			result = append(result, p)
		}
	}

	// If selectors were provided but resulted in no pods, return specific error
	if len(result) == 0 && hasTargetSelection && len(selectorsWithNoMatches) > 0 {
		return nil, &ErrSelectorMatchedNoPods{Selectors: selectorsWithNoMatches}
	}

	return result, nil
}

// ActuatorClientFactory creates actuator clients with pre-configured overrides
type ActuatorClientFactory struct {
	conn     *k8s.Connection
	port     int
	basePath string
}

// NewActuatorClientFactory creates a factory configured with command-line overrides
func NewActuatorClientFactory(conn *k8s.Connection, cmd *cobra.Command) *ActuatorClientFactory {
	root := cmd.Root()
	port, _ := root.PersistentFlags().GetInt("port")
	basePath, _ := root.PersistentFlags().GetString("base-path")

	return &ActuatorClientFactory{
		conn:     conn,
		port:     port,
		basePath: basePath,
	}
}

// NewClient creates an actuator client for the specified pod
func (f *ActuatorClientFactory) NewClient(ctx context.Context, podName string) (actuator.Client, error) {
	return actuator.NewActuatorClient(ctx, f.conn, f.conn, podName, f.port, f.basePath)
}

// baseOperations contains common fields and methods shared by all command operations
type baseOperations struct {
	k8sCliFlags           *genericclioptions.ConfigFlags
	podResolver           PodResolver
	actuatorClientFactory *ActuatorClientFactory
	pods                  []string
}

// complete initializes the k8s connection, resolves pods, and creates the actuator client factory
func (b *baseOperations) complete(cmd *cobra.Command) error {
	connection, err := k8s.NewK8sConnection(b.k8sCliFlags)
	if err != nil {
		return err
	}

	pods, err := b.podResolver(cmd.Context(), connection, cmd)
	if err != nil {
		return err
	}
	b.pods = pods

	b.actuatorClientFactory = NewActuatorClientFactory(connection, cmd)

	return nil
}

// validatePods checks that at least one pod was selected
func (b *baseOperations) validatePods() error {
	if len(b.pods) == 0 {
		return ErrNoPodsSelected
	}
	return nil
}

// validateOutputFormat checks that the output format is one of the allowed values
func validateOutputFormat(output string, allowed ...string) error {
	if output == "" {
		return nil
	}
	for _, a := range allowed {
		if output == a {
			return nil
		}
	}
	return fmt.Errorf("output format %q not recognized. Allowed formats: %s", output, strings.Join(allowed, ", "))
}
