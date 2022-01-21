package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/pkg/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "kubectl-actuator",
	Short: "Control your Spring Boot applications via Actuator",
}

var loggerCmd = &cobra.Command{
	Use:   "logger",
	Short: "Inspect and manipulate your applications loggers",
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	configFlags := genericclioptions.NewConfigFlags(true)

	// Add k8s config flags and hide them
	configFlags.AddFlags(rootCmd.PersistentFlags())
	rootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		flag.Hidden = true
	})

	rootCmd.AddCommand(loggerCmd)
	loggerCmd.AddCommand(
		cmd.NewLoggerGetCommand(configFlags),
		cmd.NewLoggerSetCommand(configFlags),
	)
}
