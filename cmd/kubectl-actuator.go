package cmd

import (
	"github.com/spf13/cobra"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/internal/cmd"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-actuator",
	Short: "Control your Spring Boot applications via Actuator",
}

func Execute() {
	err := rootCmd.Execute()
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
	cmd.AddCommands(rootCmd)
}
