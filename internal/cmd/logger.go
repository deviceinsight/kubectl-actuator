package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type loggerCommandOperations struct {
	baseOperations
	showAllLoggers bool
	loggerName     string
	targetLevel    string
	reset          bool
	isSettingLevel bool
	output         string
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
		Use:     "logger [LOGGER [LEVEL]]",
		Aliases: []string{"loggers"},
		Short:   "Manage loggers",
		Long: `View and configure logger levels.

Without arguments, shows all loggers with explicitly configured levels,
followed by the application's logger groups. With a logger name, shows
loggers and groups matching that prefix. With a logger name and level,
sets the logger to that level.

Setting a group (e.g. 'web' or 'sql') changes all its members at once.
Use RESET to clear the configured level and inherit from parent.

Valid levels: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, OFF, RESET`,
		Example: `  # Show loggers with explicitly configured levels
  kubectl actuator -d my-app logger

  # Show loggers under a package prefix
  kubectl actuator -d my-app logger com.example

  # Set a logger to DEBUG
  kubectl actuator -d my-app logger com.example.service DEBUG

  # Clear the configured level and inherit from the parent again
  kubectl actuator -d my-app logger com.example.service RESET`,
		Args: cobra.MaximumNArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 1 {
				return supportedLevels, cobra.ShellCompDirectiveNoFileComp
			}
			if len(args) > 1 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			client, ok := operations.completionClient(cmd)
			if !ok {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			response, err := client.GetLoggers()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			names := make([]string, 0, len(response.Loggers)+len(response.Groups))
			for _, group := range response.Groups {
				names = append(names, group.Name)
			}
			for _, loggerConfig := range response.Loggers {
				names = append(names, loggerConfig.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			operations.parseArgs(args)
			if err := operations.validateFlags(); err != nil {
				return err
			}
			if operations.isSettingLevel {
				if err := operations.resolvePods(cmd); err != nil {
					return err
				}
				return operations.runForEachPod(cmd.Context(), "set logger level", operations.runSetForPod)
			}
			return operations.runEndpoint(cmd, "get loggers", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	cmd.Flags().BoolVar(&operations.showAllLoggers, "all", false, "Show all loggers, not only those with configured levels")
	addOutputFlag(cmd, &operations.output, "", OutputFormatJSON, OutputFormatYAML)

	return cmd
}

func (o *loggerCommandOperations) parseArgs(args []string) {
	if len(args) >= 1 {
		o.loggerName = args[0]
	}

	if len(args) >= 2 {
		o.isSettingLevel = true
		level := strings.ToUpper(args[1])
		if level == "RESET" {
			o.reset = true
			return
		}
		o.targetLevel = level
	}
}

func (o *loggerCommandOperations) validateFlags() error {
	if err := validateOutputFormat(o.output, OutputFormatJSON, OutputFormatYAML); err != nil {
		return err
	}

	if o.isSettingLevel && o.output != "" {
		return fmt.Errorf("--output cannot be used when setting a logger level")
	}

	if o.isSettingLevel && o.showAllLoggers {
		return fmt.Errorf("--all cannot be used when setting a logger level")
	}

	if o.isSettingLevel && !o.reset && !slices.Contains(supportedLevels, o.targetLevel) {
		return fmt.Errorf("invalid log level %q\nValid levels: %s", o.targetLevel, strings.Join(supportedLevels, ", "))
	}

	if o.reset && strings.EqualFold(o.loggerName, "ROOT") {
		return fmt.Errorf("cannot reset ROOT logger: it has no parent to inherit from")
	}

	return nil
}

func (o *loggerCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	response, err := client.GetLoggers()
	if err != nil {
		return err
	}

	loggers := response.Loggers
	sortLoggers(loggers)

	w := newTableWriter()

	_, _ = fmt.Fprintln(w, "LOGGER\tLEVEL")
	skippedFiltered := 0
	// Same selection as filterLoggersJSON: configured loggers (or all with
	// --all), restricted to the name prefix; an exact name match always
	// shows. Unconfigured loggers do not count as filtered, since they are
	// hidden by default, not by the prefix.
	for _, logger := range loggers {
		if logger.ConfiguredLevel == nil && !o.showAllLoggers && !strings.EqualFold(logger.Name, o.loggerName) {
			continue
		}
		if o.loggerName != "" && !hasPrefixIgnoreCase(logger.Name, o.loggerName) {
			skippedFiltered++
			continue
		}

		level := ""
		if logger.ConfiguredLevel != nil {
			level = *logger.ConfiguredLevel
		} else if logger.EffectiveLevel != nil {
			level = *logger.EffectiveLevel + " (effective)"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\n", logger.Name, level)
	}

	_ = w.Flush()

	o.displayGroups(response.Groups)

	if skippedFiltered > 0 {
		_, _ = fmt.Fprintln(os.Stderr, skippedFiltered, "non-matching loggers omitted")
	}

	return nil
}

const maxGroupMembersLength = 100

// displayGroups appends the logger groups to the listing; a group's level
// can be set like any logger's and changes all members at once. The prefix
// filter applies to group names too.
func (o *loggerCommandOperations) displayGroups(groups []actuator.LoggerGroup) {
	matching := make([]actuator.LoggerGroup, 0, len(groups))
	for _, group := range groups {
		if o.loggerName != "" && !hasPrefixIgnoreCase(group.Name, o.loggerName) {
			continue
		}
		matching = append(matching, group)
	}
	if len(matching) == 0 {
		return
	}

	sort.Slice(matching, func(i, j int) bool { return matching[i].Name < matching[j].Name })

	fmt.Println()
	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "GROUP\tCONFIGURED\tMEMBERS")
	for _, group := range matching {
		configured := "-"
		if group.ConfiguredLevel != nil {
			configured = *group.ConfiguredLevel
		}
		members := truncateString(strings.Join(group.Members, ", "), maxGroupMembersLength)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", group.Name, configured, members)
	}
}

// sortLoggers orders loggers alphabetically with ROOT always first.
func sortLoggers(loggers []actuator.LoggerConfiguration) {
	sort.Slice(loggers, func(i, j int) bool {
		if loggers[i].Name == "ROOT" {
			return true
		}
		if loggers[j].Name == "ROOT" {
			return false
		}
		return loggers[i].Name < loggers[j].Name
	})
}

func (o *loggerCommandOperations) runSetForPod(ctx context.Context, client actuator.Client, podName string) error {
	level := o.targetLevel
	if o.reset {
		level = "" // The actuator API resets a logger by posting a null level.
	}
	if err := client.SetLoggerLevel(o.loggerName, level); err != nil {
		return err
	}

	if o.reset {
		fmt.Printf("Logger %q reset to default\n", o.loggerName)
	} else {
		fmt.Printf("Logger %q set to %s\n", o.loggerName, o.targetLevel)
	}

	return nil
}

func (o *loggerCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	data, err := client.GetRaw("loggers")
	if err != nil {
		return nil, err
	}
	return filterLoggersJSON(data, o.loggerName, o.showAllLoggers)
}

// filterLoggersJSON applies the table view's selection to the raw /loggers
// response: keep configured loggers (or all with showAll), restricted to the
// given name prefix; a logger matching the name exactly is always kept.
// Everything outside the loggers map passes through untouched.
func filterLoggersJSON(data json.RawMessage, loggerName string, showAll bool) (json.RawMessage, error) {
	if showAll && loggerName == "" {
		return data, nil
	}
	tree, err := decodeTree(data)
	if err != nil {
		return nil, err
	}
	loggers, ok := tree["loggers"].(map[string]any)
	if !ok {
		return data, nil
	}
	for name, loggerValue := range loggers {
		loggerMap, _ := loggerValue.(map[string]any)
		configured := loggerMap != nil && loggerMap["configuredLevel"] != nil
		exactMatch := loggerName != "" && strings.EqualFold(name, loggerName)
		matchesPrefix := loggerName == "" || hasPrefixIgnoreCase(name, loggerName)
		keep := exactMatch || ((configured || showAll) && matchesPrefix)
		if !keep {
			delete(loggers, name)
		}
	}
	return encodeTree(tree)
}
