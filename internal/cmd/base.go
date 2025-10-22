package cmd

import (
	"context"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type PodResolver func(ctx context.Context, k8sClient k8s.Client, cmd *cobra.Command) ([]string, error)

func AddCommands(rootCmd *cobra.Command) {
	configFlags := genericclioptions.NewConfigFlags(true)

	// Add k8s config flags and hide them
	configFlags.AddFlags(rootCmd.PersistentFlags())
	rootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		flag.Hidden = true
	})

	// Global target selection
	rootCmd.PersistentFlags().StringArrayP("pod", "p", nil, "Select target pod(s)")
	rootCmd.PersistentFlags().StringArrayP("deployment", "d", nil, "Select target deployment(s)")
	rootCmd.PersistentFlags().StringArrayP("selector", "l", nil, "Select target pod(s) by label selector")

	// Shell completion
	_ = rootCmd.RegisterFlagCompletionFunc("pod", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		k8sClient, err := k8s.NewK8sConnection(configFlags)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		workloadNames, err := k8sClient.ListPods(cmd.Context(), k8sClient.Namespace(), "")
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return workloadNames, cobra.ShellCompDirectiveNoFileComp
	})

	_ = rootCmd.RegisterFlagCompletionFunc("deployment", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		k8sClient, err := k8s.NewK8sConnection(configFlags)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		workloadNames, err := k8sClient.ListDeployments(cmd.Context(), k8sClient.Namespace())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return workloadNames, cobra.ShellCompDirectiveNoFileComp
	})

	// Actuator subcommands
	rootCmd.AddCommand(NewLoggerCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewScheduledTasksCommand(configFlags, FlagsPodResolver))
	rootCmd.AddCommand(NewInfoCommand(configFlags, FlagsPodResolver))
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

	// Expand deployments to pods
	for _, d := range deployments {
		names, err := k8sClient.GetDeploymentPods(ctx, k8sClient.Namespace(), d)
		if err != nil {
			cmd.SilenceUsage = true
			return nil, err
		}
		pods = append(pods, names...)
	}

	// Expand selectors to pods
	for _, s := range selectors {
		names, err := k8sClient.ListPods(ctx, k8sClient.Namespace(), s)
		if err != nil {
			cmd.SilenceUsage = true
			return nil, err
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

	return result, nil
}
