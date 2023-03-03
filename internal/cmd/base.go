package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/internal/k8s"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"strings"
)

type PodResolver func(connection *k8s.Connection, cmd *cobra.Command) ([]string, error)

func AddCommands(rootCmd *cobra.Command) {
	configFlags := genericclioptions.NewConfigFlags(true)

	// Add k8s config flags and hide them
	configFlags.AddFlags(rootCmd.PersistentFlags())
	rootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		flag.Hidden = true
	})

	addPodCmd(
		rootCmd, configFlags,
		"pod pod-name", "Execute actuator command for a pod", []string{"po"},
		PodPodResolver,
		k8s.Connection.ListPods,
	)

	addPodCmd(
		rootCmd, configFlags,
		"deployment deployment-name", "Execute actuator command for a deployment", []string{"deploy"},
		DeploymentPodResolver,
		k8s.Connection.ListDeployments,
	)
}

func addPodCmd(
	parentCmd *cobra.Command,
	configFlags *genericclioptions.ConfigFlags,
	use string,
	short string,
	aliases []string,
	podResolver PodResolver,
	workloadListFunction func(k8s.Connection) ([]string, error),
) {
	var subCommandsCreators = []func(*genericclioptions.ConfigFlags, PodResolver) *cobra.Command{NewLoggerCommand}

	var childCmd = &cobra.Command{
		Use:     use,
		Short:   short,
		Aliases: aliases,
		Args:    cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			connection, err := k8s.NewK8sConnection(configFlags)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			workloadName, err := workloadListFunction(*connection)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			return workloadName, cobra.ShellCompDirectiveNoFileComp
		},
	}

	// Stupid hack: Add child command with name of the selected pod.
	var wrapperCmd = &cobra.Command{Use: getSubcommandWorkloadName()}
	for _, commandCreator := range subCommandsCreators {
		wrapperCmd.AddCommand(commandCreator(configFlags, podResolver))
	}
	childCmd.AddCommand(wrapperCmd)

	parentCmd.AddCommand(childCmd)
}

func getSubcommandWorkloadName() string {
	var position = 0
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-") {
			// XXX: This doesn't really work
			continue
		}

		if position == 1 && (arg == "help" || arg == cobra.ShellCompRequestCmd || arg == cobra.ShellCompNoDescRequestCmd) {
			position = 0
		}

		if position == 2 && arg != "" {
			return arg
		}
		position++
	}
	return ""
}

func PodPodResolver(_ *k8s.Connection, cmd *cobra.Command) ([]string, error) {
	return []string{cmd.Parent().Name()}, nil
}

func DeploymentPodResolver(connection *k8s.Connection, cmd *cobra.Command) ([]string, error) {
	deployment, err := connection.Clientset.
		AppsV1().
		Deployments(connection.Namespace).
		Get(context.Background(), cmd.Parent().Name(), v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	selector, err := v1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}

	podList, err := connection.Clientset.
		CoreV1().
		Pods(connection.Namespace).
		List(context.Background(), v1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}

	var podNames []string
	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}
