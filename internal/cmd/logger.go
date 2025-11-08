package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type loggerCommandOperations struct {
	baseOperations
	showAllLoggers bool
	loggerName     string
	targetLevel    string
	isSettingLevel bool
}

var supportedLevels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF", "RESET"}

func NewLoggerCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &loggerCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "logger [logger-name [LEVEL]]",
		Short: "Manage loggers",
		Long: `View and configure logger levels.

Without arguments, shows all loggers with explicitly configured levels.
With a logger name, shows loggers matching that prefix.
With a logger name and level, sets the logger to that level.

Use RESET to clear the configured level and inherit from parent.

Valid levels: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, OFF, RESET`,
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd, args); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			if operations.isSettingLevel {
				return RunForEachPod(cmd.Context(), operations.pods, "set logger level", operations.runSetForPod)
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get loggers", operations.runForPod)
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			err := operations.complete(cmd, args)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			if len(args) == 0 {
				return operations.validArgsLogger(cmd.Context())
			} else if len(args) == 1 {
				return operations.validArgsLogLevel()
			} else {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
		},
	}

	cmd.Flags().BoolVar(&operations.showAllLoggers, "all-loggers", false, "Show all loggers")

	return cmd
}

func (o *loggerCommandOperations) complete(cmd *cobra.Command, args []string) error {
	if err := o.baseOperations.complete(cmd); err != nil {
		return err
	}

	if len(args) >= 1 {
		o.loggerName = args[0]
	}

	if len(args) >= 2 {
		level := strings.ToUpper(args[1])
		if level == "RESET" {
			o.targetLevel = "" // Empty string signals reset
		} else {
			o.targetLevel = level
		}
		o.isSettingLevel = true
	}

	return nil
}

func (o *loggerCommandOperations) validate() error {
	if err := o.validatePods(); err != nil {
		return err
	}

	if o.targetLevel != "" && !slices.Contains(supportedLevels, o.targetLevel) {
		return fmt.Errorf("invalid log level '%s'\nValid levels: %v", o.targetLevel, supportedLevels)
	}

	if o.isSettingLevel && o.targetLevel == "" && strings.EqualFold(o.loggerName, "ROOT") {
		return fmt.Errorf("cannot reset ROOT logger: it has no parent to inherit from")
	}

	return nil
}

func (o *loggerCommandOperations) validArgsLogger(ctx context.Context) ([]string, cobra.ShellCompDirective) {
	if len(o.pods) == 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client, err := o.actuatorClientFactory.NewClient(ctx, o.pods[0])
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	loggers, err := client.GetLoggers()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var loggerNames []string
	for _, logger := range loggers {
		loggerNames = append(loggerNames, logger.Name)
	}

	return loggerNames, cobra.ShellCompDirectiveNoFileComp
}

func (o *loggerCommandOperations) validArgsLogLevel() ([]string, cobra.ShellCompDirective) {
	return supportedLevels, cobra.ShellCompDirectiveNoFileComp
}
