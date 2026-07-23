package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	maxBeanTypeLength        = 80
	maxBeanNameLength        = 70
	maxDependenciesToDisplay = 5
)

type beansCommandOperations struct {
	baseOperations
	filter string
	output string
}

func NewBeansCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &beansCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "beans",
		Short: "Get Spring application beans",
		Long: `Get Spring application beans from Spring Boot Actuator.

Displays information about all Spring beans in the application context,
including their scope, type, and dependencies.`,
		Example: `  # List all beans of a deployment's pods
  kubectl actuator -d my-app beans

  # Show beans whose name contains 'service', with full details
  kubectl actuator -d my-app beans -f service -o wide

  # List matching bean names only
  kubectl actuator -d my-app beans -f repository -o name`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.validateFlags(); err != nil {
				return err
			}
			return operations.runEndpoint(cmd, "get beans", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.filter, "filter", "f", "", "Filter beans by name (case-insensitive substring)")
	addOutputFlag(cmd, &operations.output, "", OutputFormatWide, OutputFormatName, OutputFormatJSON, OutputFormatYAML)
	markNoFileFlags(cmd, "filter")

	return cmd
}

func (o *beansCommandOperations) validateFlags() error {
	return validateOutputFormat(o.output, OutputFormatWide, OutputFormatName, OutputFormatJSON, OutputFormatYAML)
}

func (o *beansCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	data, err := client.GetRaw("beans")
	if err != nil {
		return nil, err
	}
	return filterBeansJSON(data, o.filter)
}

// filterBeansJSON applies the bean name filter to the raw /beans response,
// keeping the contexts/beans nesting and all other fields intact.
func filterBeansJSON(data json.RawMessage, filter string) (json.RawMessage, error) {
	if filter == "" {
		return data, nil
	}
	tree, err := decodeTree(data)
	if err != nil {
		return nil, err
	}
	contexts, ok := tree["contexts"].(map[string]any)
	if !ok {
		return data, nil
	}
	for _, contextValue := range contexts {
		contextMap, ok := contextValue.(map[string]any)
		if !ok {
			continue
		}
		beans, ok := contextMap["beans"].(map[string]any)
		if !ok {
			continue
		}
		for name := range beans {
			if !matchesFilter(name, filter) {
				delete(beans, name)
			}
		}
	}
	return encodeTree(tree)
}

func (o *beansCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	beansResponse, err := client.GetBeans()
	if err != nil {
		return err
	}

	switch o.output {
	case OutputFormatName:
		return displayBeansNames(beansResponse, o.filter)
	case OutputFormatWide:
		return displayBeansWide(beansResponse, o.filter)
	default:
		return displayBeansTable(beansResponse, o.filter)
	}
}

// printNoBeansMatch reports an empty result on stderr, keeping stdout clean
// for pipes.
func printNoBeansMatch(filter string) {
	if filter != "" {
		_, _ = fmt.Fprintf(os.Stderr, "No beans match filter %q\n", filter)
	} else {
		_, _ = fmt.Fprintln(os.Stderr, "No beans found")
	}
}

func displayBeansNames(beansResponse *actuator.BeansResponse, filter string) error {
	var beanNames []string
	for _, appCtx := range beansResponse.Contexts {
		for beanName := range appCtx.Beans {
			if matchesFilter(beanName, filter) {
				beanNames = append(beanNames, beanName)
			}
		}
	}

	if len(beanNames) == 0 {
		printNoBeansMatch(filter)
		return nil
	}

	sort.Strings(beanNames)

	for _, beanName := range beanNames {
		fmt.Println(beanName)
	}

	return nil
}

func displayBeansWide(beansResponse *actuator.BeansResponse, filter string) error {
	contextNames := make([]string, 0, len(beansResponse.Contexts))
	for contextName := range beansResponse.Contexts {
		contextNames = append(contextNames, contextName)
	}
	sort.Strings(contextNames)

	matchedAny := false
	for _, contextName := range contextNames {
		appCtx := beansResponse.Contexts[contextName]
		matchingBeans := make(map[string]actuator.Bean)

		for beanName, bean := range appCtx.Beans {
			if matchesFilter(beanName, filter) {
				matchingBeans[beanName] = bean
			}
		}

		if len(matchingBeans) == 0 {
			continue
		}
		matchedAny = true

		fmt.Printf("Context: %s\n", contextName)
		fmt.Printf("Beans: %d\n\n", len(matchingBeans))

		beanNames := make([]string, 0, len(matchingBeans))
		for beanName := range matchingBeans {
			beanNames = append(beanNames, beanName)
		}
		sort.Strings(beanNames)

		for _, beanName := range beanNames {
			bean := matchingBeans[beanName]
			fmt.Printf("Bean: %s\n", beanName)
			if len(bean.Aliases) > 0 {
				fmt.Printf("  Aliases: %s\n", strings.Join(bean.Aliases, ", "))
			}
			fmt.Printf("  Type: %s\n", bean.Type)
			if bean.Scope != "" {
				fmt.Printf("  Scope: %s\n", bean.Scope)
			}
			if bean.Resource != "" {
				fmt.Printf("  Resource: %s\n", bean.Resource)
			}
			if len(bean.Dependencies) > 0 {
				fmt.Printf("  Dependencies (%d):\n", len(bean.Dependencies))
				displayCount := maxDependenciesToDisplay
				if len(bean.Dependencies) < displayCount {
					displayCount = len(bean.Dependencies)
				}
				for i := 0; i < displayCount; i++ {
					fmt.Printf("    - %s\n", bean.Dependencies[i])
				}
				if len(bean.Dependencies) > displayCount {
					fmt.Printf("    ... and %d more\n", len(bean.Dependencies)-displayCount)
				}
			}
			fmt.Println()
		}
	}

	if !matchedAny {
		printNoBeansMatch(filter)
	}

	return nil
}

func displayBeansTable(beansResponse *actuator.BeansResponse, filter string) error {
	type beanInfo struct {
		name    string
		context string
		bean    actuator.Bean
	}
	var allBeans []beanInfo

	for contextName, appCtx := range beansResponse.Contexts {
		for beanName, bean := range appCtx.Beans {
			if matchesFilter(beanName, filter) {
				allBeans = append(allBeans, beanInfo{
					name:    beanName,
					context: contextName,
					bean:    bean,
				})
			}
		}
	}

	if len(allBeans) == 0 {
		printNoBeansMatch(filter)
		return nil
	}

	sort.Slice(allBeans, func(i, j int) bool {
		return allBeans[i].name < allBeans[j].name
	})

	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "NAME\tTYPE\tSCOPE\tDEPENDENCIES")

	for _, info := range allBeans {
		bean := info.bean
		scope := bean.Scope
		if scope == "" {
			scope = "singleton"
		}

		typeName := shortenType(bean.Type, maxBeanTypeLength)
		beanName := smartTruncate(info.name, maxBeanNameLength)

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", beanName, typeName, scope, len(bean.Dependencies))
	}

	return nil
}

// smartTruncate shortens s to maxLen runes, keeping the segment after the
// last dot intact where possible since it carries the most meaning.
func smartTruncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	lastDot := strings.LastIndex(s, ".")
	var suffix []rune

	if lastDot != -1 && lastDot < len(s)-1 {
		suffix = []rune(s[lastDot+1:])
	} else {
		// No dot to anchor on: keep a tail of just under half the budget and
		// leave the rest for the prefix and the ellipsis.
		suffixLen := (maxLen - 3) / 2
		if suffixLen > len(runes) {
			suffixLen = len(runes)
		}
		suffix = runes[len(runes)-suffixLen:]
	}

	// The kept segment alone busts the budget: show as much of its tail as
	// fits behind an ellipsis.
	if len(suffix) > maxLen-1 {
		return "…" + string(suffix[len(suffix)-(maxLen-1):])
	}

	prefixLen := maxLen - len(suffix) - 1
	if prefixLen <= 0 {
		return "…" + string(suffix)
	}
	return string(runes[:prefixLen]) + "…" + string(suffix)
}

func shortenType(fullType string, maxLen int) string {
	lastDot := strings.LastIndex(fullType, ".")
	if lastDot == -1 {
		return truncateString(fullType, maxLen)
	}

	packagePath := fullType[:lastDot]
	className := fullType[lastDot+1:]

	segments := strings.Split(packagePath, ".")
	abbreviated := make([]string, len(segments))
	for i, segment := range segments {
		if len(segment) > 0 {
			abbreviated[i] = segment[:1]
		}
	}

	result := strings.Join(abbreviated, ".") + "." + className
	return truncateString(result, maxLen)
}
