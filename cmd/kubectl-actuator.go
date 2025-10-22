package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/deviceinsight/kubectl-actuator/internal/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "kubectl-actuator",
	Annotations: map[string]string{
		// https://github.com/spf13/cobra/blob/7da941c3547e93b8c9f70bbd3befca79c6335388/site/content/user_guide.md#creating-a-plugin
		cobra.CommandDisplayNameAnnotation: "kubectl actuator",
	},
	Short: "Control your Spring Boot applications via Actuator",
}

func Execute() {
	// Create a context that can be cancelled by interrupt signals
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

func PrintCompletion() {
	var args []string
	args = append(args, cobra.ShellCompRequestCmd)
	args = append(args, os.Args[1:]...)

	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddCommands(rootCmd)
}
