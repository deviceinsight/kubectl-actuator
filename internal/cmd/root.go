package cmd

import (
	"context"
	"sort"
	"time"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// AddCommands wires global flags, shell completion, and all subcommands onto
// the root command.
func AddCommands(rootCmd *cobra.Command) {
	configFlags := genericclioptions.NewConfigFlags(true)

	configFlags.AddFlags(rootCmd.PersistentFlags())
	// genericclioptions registers a dozen kubeconfig flags that would drown
	// --help; only the three a plugin user actually reaches for stay visible.
	rootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		switch flag.Name {
		case "namespace", "context", "kubeconfig":
		default:
			flag.Hidden = true
		}
	})

	// Global target selection. Pods and deployments accept comma-separated values;
	// selectors must not be comma-split because commas are AND-syntax within a selector.
	rootCmd.PersistentFlags().StringSliceP("pod", "p", nil, "Select target pod(s)")
	rootCmd.PersistentFlags().StringSliceP("deployment", "d", nil, "Select target deployment(s)")
	rootCmd.PersistentFlags().StringArrayP("selector", "l", nil, "Select target pod(s) by label selector")

	rootCmd.PersistentFlags().IntP("port", "", 0, "Actuator port (default 8080, or the pod's kubectl-actuator.device-insight.com/port annotation)")
	rootCmd.PersistentFlags().StringP("base-path", "", "", "Actuator base path (default 'actuator', or the pod's kubectl-actuator.device-insight.com/basePath annotation; use '/' for endpoints at the root)")
	rootCmd.PersistentFlags().Duration("timeout", 30*time.Second, "Timeout for each actuator request (0 for no limit; downloads are never time-limited)")

	registerClusterListCompletion(rootCmd, configFlags, "pod", func(ctx context.Context, k8sClient k8s.Client) ([]string, error) {
		return k8sClient.ListPods(ctx, "")
	})
	registerClusterListCompletion(rootCmd, configFlags, "deployment", func(ctx context.Context, k8sClient k8s.Client) ([]string, error) {
		return k8sClient.ListDeployments(ctx)
	})
	registerClusterListCompletion(rootCmd, configFlags, "namespace", func(ctx context.Context, k8sClient k8s.Client) ([]string, error) {
		return k8sClient.ListNamespaces(ctx)
	})

	_ = rootCmd.RegisterFlagCompletionFunc("context", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		config, err := configFlags.ToRawKubeConfigLoader().RawConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		contextNames := make([]string, 0, len(config.Contexts))
		for name := range config.Contexts {
			contextNames = append(contextNames, name)
		}
		sort.Strings(contextNames)
		return contextNames, cobra.ShellCompDirectiveNoFileComp
	})

	markNoFileFlags(rootCmd, "port", "base-path", "timeout", "selector")

	rootCmd.AddCommand(NewEndpointsCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewLoggerCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewScheduledTasksCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewInfoCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewHealthCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewMetricsCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewEnvCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewThreadDumpCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewBeansCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewHeapDumpCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewLogFileCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewRawCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewVersionCommand())
}

// registerClusterListCompletion wires a flag's completion to a cluster
// listing. All failures leave the completion empty: completion must stay
// silent when the cluster is unavailable.
func registerClusterListCompletion(rootCmd *cobra.Command, configFlags *genericclioptions.ConfigFlags, flagName string, list func(ctx context.Context, k8sClient k8s.Client) ([]string, error)) {
	_ = rootCmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		k8sClient, err := k8s.NewConnection(configFlags)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names, err := list(cmd.Context(), k8sClient)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})
}
