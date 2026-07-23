package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	commands "github.com/deviceinsight/kubectl-actuator/internal/cmd"
)

var rootCmd = &cobra.Command{
	Use: "kubectl-actuator",
	Annotations: map[string]string{
		// https://github.com/spf13/cobra/blob/7da941c3547e93b8c9f70bbd3befca79c6335388/site/content/user_guide.md#creating-a-plugin
		cobra.CommandDisplayNameAnnotation: "kubectl actuator",
	},
	Short: "Control your Spring Boot applications via Actuator",
	Long: `Control your Spring Boot applications via their Actuator endpoints.

Select target pods with -p/--pod, -d/--deployment, or -l/--selector.

The actuator is expected on port 8080 under the base path 'actuator'.
Override this per invocation with --port and --base-path, or per pod with
the kubectl-actuator.device-insight.com/port and .../basePath annotations.
Flags take precedence over annotations, annotations over defaults.`,
	Example: `  # Check health of every pod in a deployment
  kubectl actuator -d my-app health

  # Set a logger level on one pod
  kubectl actuator -p my-app-7d4b9c-xk2pq logger com.example.service DEBUG

  # Discover which endpoints an application exposes
  kubectl actuator -d my-app endpoints

  # Query any other actuator endpoint as JSON
  kubectl actuator -d my-app raw mappings`,
	PersistentPreRunE: func(c *cobra.Command, args []string) error {
		// Silence usage for subcommands after args are validated - runtime errors shouldn't show help
		if c.HasParent() {
			c.SilenceUsage = true
		}
		return nil
	},
	RunE: func(c *cobra.Command, args []string) error {
		return c.Help()
	},
}

func Execute() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return
	}
	var exitErr *commands.ExitCodeError
	switch {
	case errors.As(err, &exitErr):
		os.Exit(exitErr.Code)
	case errors.Is(err, commands.ErrInterrupted) || errors.Is(err, context.Canceled):
		os.Exit(130)
	default:
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
	// Pre-register --version without a shorthand: cobra's auto-registered
	// version flag would claim -v, which kubectl users know as verbosity.
	rootCmd.Flags().Bool("version", false, "Print version information")
	rootCmd.Version = commands.Version
	rootCmd.SetVersionTemplate("kubectl actuator version {{.Version}}\n")
	commands.AddCommands(rootCmd)
}
