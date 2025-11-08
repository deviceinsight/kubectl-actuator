package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

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
		Use:   "env [property-name]",
		Short: "Get environment properties and configuration",
		Long: `Get environment properties and configuration from Spring Boot Actuator.

Without arguments, shows all property sources and active profiles.
With a property name argument, shows details for that specific property.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd, args); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get env", operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.filter, "filter", "f", "", "Filter properties by name pattern")
	cmd.Flags().StringVarP(&operations.output, "output", "o", "", "Output format. One of: name")

	return cmd
}

func (o *envCommandOperations) complete(cmd *cobra.Command, args []string) error {
	if err := o.baseOperations.complete(cmd); err != nil {
		return err
	}

	if len(args) >= 1 {
		o.propertyName = args[0]
	}

	return nil
}

func (o *envCommandOperations) validate() error {
	if err := o.validatePods(); err != nil {
		return err
	}
	return validateOutputFormat(o.output, OutputFormatName)
}

func (o *envCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

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

func (o *envCommandOperations) displayEnvNames(envResponse *actuator.EnvResponse) error {
	propertyNamesSet := make(map[string]struct{})
	for _, source := range envResponse.PropertySources {
		for propName := range source.Properties {
			if o.filter == "" || strings.Contains(propName, o.filter) {
				propertyNamesSet[propName] = struct{}{}
			}
		}
	}

	propertyNames := make([]string, 0, len(propertyNamesSet))
	for propName := range propertyNamesSet {
		propertyNames = append(propertyNames, propName)
	}
	sort.Strings(propertyNames)

	for _, propName := range propertyNames {
		fmt.Println(propName)
	}
	return nil
}

func (o *envCommandOperations) displayEnvTable(envResponse *actuator.EnvResponse) error {
	fmt.Printf("Active Profiles: %v\n\n", envResponse.ActiveProfiles)

	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "NAME\tVALUE\tORIGIN")

	for _, source := range envResponse.PropertySources {
		for propName, propDetails := range source.Properties {
			if o.filter != "" && !strings.Contains(propName, o.filter) {
				continue
			}

			origin := propDetails.Origin
			if origin == "" {
				origin = source.Name
			}

			value := escapeValue(fmt.Sprintf("%v", propDetails.Value))

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", propName, value, origin)
		}
	}

	return nil
}

func escapeValue(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func (o *envCommandOperations) displayProperty(client actuator.Client) error {
	property, err := client.GetEnvProperty(o.propertyName)
	if err != nil {
		return err
	}

	value := escapeValue(fmt.Sprintf("%v", property.Property.Value))
	source := property.Property.Source

	// Find the origin from the property sources if available
	origin := ""
	for _, ps := range property.PropertySources {
		if ps.Property != nil {
			if propMap, ok := ps.Property.(map[string]interface{}); ok {
				if o, exists := propMap["origin"]; exists {
					origin = fmt.Sprintf("%v", o)
					break
				}
			}
		}
	}

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
