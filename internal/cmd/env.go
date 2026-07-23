package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const maxEnvValueLength = 100

type envCommandOperations struct {
	baseOperations
	filter       string
	output       string
	propertyName string
}

func NewEnvCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &envCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "env [PROPERTY]",
		Short: "Get environment properties and configuration",
		Long: `Get environment properties and configuration from Spring Boot Actuator.

Without arguments, shows all property sources and active profiles.
With a property name argument, shows details for that specific property,
including its full untruncated value.`,
		Example: `  # Show all properties of a deployment's pods
  kubectl actuator -d my-app env

  # Show properties whose name contains 'spring'
  kubectl actuator -d my-app env -f spring

  # Show one property with its full value and origin
  kubectl actuator -d my-app env spring.datasource.url

  # List matching property names only
  kubectl actuator -d my-app env -f datasource -o name`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			client, ok := operations.completionClient(cmd)
			if !ok {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			envResponse, err := client.GetEnv()
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return collectPropertyNames(envResponse, ""), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) >= 1 {
				operations.propertyName = args[0]
			}
			if err := operations.validateFlags(); err != nil {
				return err
			}
			return operations.runEndpoint(cmd, "get env", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.filter, "filter", "f", "", "Filter properties by name (case-insensitive substring)")
	addOutputFlag(cmd, &operations.output, "", OutputFormatName, OutputFormatJSON, OutputFormatYAML)
	markNoFileFlags(cmd, "filter")

	return cmd
}

func (o *envCommandOperations) validateFlags() error {
	if err := validateOutputFormat(o.output, OutputFormatName, OutputFormatJSON, OutputFormatYAML); err != nil {
		return err
	}
	if o.propertyName != "" && o.filter != "" {
		return fmt.Errorf("--filter cannot be combined with a property name argument")
	}
	return nil
}

func (o *envCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	if o.propertyName != "" {
		return client.GetRaw("env/" + url.PathEscape(o.propertyName))
	}
	data, err := client.GetRaw("env")
	if err != nil {
		return nil, err
	}
	return filterEnvJSON(data, o.filter)
}

// filterEnvJSON applies the property name filter to the raw /env response,
// keeping the propertySources nesting and all other fields intact.
func filterEnvJSON(data json.RawMessage, filter string) (json.RawMessage, error) {
	if filter == "" {
		return data, nil
	}
	tree, err := decodeTree(data)
	if err != nil {
		return nil, err
	}
	sources, ok := tree["propertySources"].([]any)
	if !ok {
		return data, nil
	}
	for _, sourceValue := range sources {
		sourceMap, ok := sourceValue.(map[string]any)
		if !ok {
			continue
		}
		properties, ok := sourceMap["properties"].(map[string]any)
		if !ok {
			continue
		}
		for name := range properties {
			if !matchesFilter(name, filter) {
				delete(properties, name)
			}
		}
	}
	return encodeTree(tree)
}

func (o *envCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	if o.propertyName != "" {
		return o.displayProperty(client)
	}
	return o.displayEnv(client)
}

func (o *envCommandOperations) displayEnv(client actuator.Client) error {
	envResponse, err := client.GetEnv()
	if err != nil {
		return err
	}

	if o.output == OutputFormatName {
		return o.displayEnvNames(envResponse)
	}
	return o.displayEnvTable(envResponse)
}

// printNoPropertiesMatch reports an empty filter result on stderr, so pipes
// stay clean and "no match" is distinguishable from an empty environment.
func printNoPropertiesMatch(filter string) {
	_, _ = fmt.Fprintf(os.Stderr, "No properties match filter %q\n", filter)
}

// collectPropertyNames returns the deduplicated, sorted property names of
// all property sources, restricted to those matching the filter.
func collectPropertyNames(envResponse *actuator.EnvResponse, filter string) []string {
	propertyNamesSet := make(map[string]struct{})
	for _, source := range envResponse.PropertySources {
		for propName := range source.Properties {
			if matchesFilter(propName, filter) {
				propertyNamesSet[propName] = struct{}{}
			}
		}
	}

	propertyNames := make([]string, 0, len(propertyNamesSet))
	for propName := range propertyNamesSet {
		propertyNames = append(propertyNames, propName)
	}
	sort.Strings(propertyNames)
	return propertyNames
}

func (o *envCommandOperations) displayEnvNames(envResponse *actuator.EnvResponse) error {
	propertyNames := collectPropertyNames(envResponse, o.filter)

	if len(propertyNames) == 0 && o.filter != "" {
		printNoPropertiesMatch(o.filter)
		return nil
	}

	for _, propName := range propertyNames {
		fmt.Println(propName)
	}
	return nil
}

func (o *envCommandOperations) displayEnvTable(envResponse *actuator.EnvResponse) error {
	type tableEntry struct {
		name    string
		details actuator.PropertyDetails
		source  string
	}

	// Property sources keep their response order, which reflects Spring's
	// real precedence; properties within a source are sorted so output is
	// stable across runs.
	var entries []tableEntry
	for _, source := range envResponse.PropertySources {
		names := make([]string, 0, len(source.Properties))
		for propName := range source.Properties {
			if matchesFilter(propName, o.filter) {
				names = append(names, propName)
			}
		}
		sort.Strings(names)
		for _, propName := range names {
			entries = append(entries, tableEntry{name: propName, details: source.Properties[propName], source: source.Name})
		}
	}

	if len(entries) == 0 && o.filter != "" {
		printNoPropertiesMatch(o.filter)
		return nil
	}

	profiles := strings.Join(envResponse.ActiveProfiles, ", ")
	if profiles == "" {
		profiles = "-"
	}
	fmt.Printf("Active Profiles: %s\n\n", profiles)

	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "NAME\tVALUE\tORIGIN")

	for _, entry := range entries {
		origin := entry.details.Origin
		if origin == "" {
			origin = entry.source
		}

		value := truncateString(escapeValue(fmt.Sprintf("%v", entry.details.Value)), maxEnvValueLength)

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.name, value, origin)
	}

	return nil
}

// propertyOrigin returns the first origin recorded in the property's
// sources; Spring reports one only for sources that track origins.
func propertyOrigin(sources []actuator.PropertySourceReference) string {
	for _, source := range sources {
		propMap, ok := source.Property.(map[string]any)
		if !ok {
			continue
		}
		if origin, exists := propMap["origin"]; exists {
			return fmt.Sprintf("%v", origin)
		}
	}
	return ""
}

func (o *envCommandOperations) displayProperty(client actuator.Client) error {
	property, err := client.GetEnvProperty(o.propertyName)
	if err != nil {
		return err
	}

	value := escapeValue(fmt.Sprintf("%v", property.Property.Value))
	source := property.Property.Source
	origin := propertyOrigin(property.PropertySources)

	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintf(w, "NAME:\t%s\n", o.propertyName)
	_, _ = fmt.Fprintf(w, "VALUE:\t%s\n", value)
	_, _ = fmt.Fprintf(w, "SOURCE:\t%s\n", source)
	if origin != "" {
		_, _ = fmt.Fprintf(w, "ORIGIN:\t%s\n", origin)
	}
	return nil
}
