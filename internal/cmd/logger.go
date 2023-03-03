package cmd

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/internal/acuator"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/internal/k8s"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"strings"
)

type loggerCommandOperations struct {
	k8sCliFlags *genericclioptions.ConfigFlags
	connection  *k8s.Connection
	podResolver PodResolver

	pods           []string
	showAllLoggers bool
	loggerName     string
	targetLevel    string
}

var supportedLevels = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF"}

func NewLoggerCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &loggerCommandOperations{k8sCliFlags: configFlags, podResolver: podResolver}

	cmd := &cobra.Command{
		Use:   "logger [com.example.logger LEVEL]",
		Short: "Manage loggers",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := operations.complete(cmd, args)
			if err != nil {
				return err
			}

			err = operations.validate()
			if err != nil {
				return err
			}

			if operations.targetLevel != "" {
				err = operations.runSetLogger()
			} else {
				err = operations.runGetLogger()
			}
			if err != nil {
				return err
			}

			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			err := operations.complete(cmd, args)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			if len(args) == 0 {
				return operations.validArgsLogger()
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
	connection, err := k8s.NewK8sConnection(o.k8sCliFlags)
	if err != nil {
		return err
	}
	o.connection = connection

	pods, err := o.podResolver(connection, cmd)
	if err != nil {
		return err
	}
	o.pods = pods

	if len(args) >= 1 {
		o.loggerName = args[0]
	}

	if len(args) >= 2 {
		o.targetLevel = strings.ToUpper(args[1])
	}

	return nil
}

func (o *loggerCommandOperations) validate() error {
	if len(o.pods) == 0 {
		return errors.New("No pods specified. Please specify at least one pod")
	}

	var found = false
	for _, supportedLevel := range supportedLevels {
		if o.targetLevel == supportedLevel {
			found = true
		}
	}

	if o.targetLevel != "" && !found {
		return fmt.Errorf("unsupported log level: %s. Supported levels: %v", o.targetLevel, supportedLevels)
	}

	return nil
}

func (o *loggerCommandOperations) validArgsLogger() ([]string, cobra.ShellCompDirective) {
	client, err := acuator.NewActuatorClient(o.connection, o.pods[0])
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
