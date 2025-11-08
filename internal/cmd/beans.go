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
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get beans", operations.runForPod)
		},
	}

	cmd.Flags().StringVarP(&operations.filter, "filter", "f", "", "Filter beans by name pattern")
	cmd.Flags().StringVarP(&operations.output, "output", "o", "", "Output format. One of: wide, name")

	return cmd
}

func (o *beansCommandOperations) validate() error {
	if err := o.validatePods(); err != nil {
		return err
	}
	return validateOutputFormat(o.output, OutputFormatWide, OutputFormatName)
}

func (o *beansCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

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

func displayBeansNames(beansResponse *actuator.BeansResponse, filter string) error {
	var beanNames []string
	for _, appCtx := range beansResponse.Contexts {
		for beanName := range appCtx.Beans {
			if filter == "" || strings.Contains(strings.ToLower(beanName), strings.ToLower(filter)) {
				beanNames = append(beanNames, beanName)
			}
		}
	}

	sort.Strings(beanNames)

	for _, beanName := range beanNames {
		fmt.Println(beanName)
	}

	if filter != "" {
		fmt.Printf("\nTotal matching beans: %d\n", len(beanNames))
	}

	return nil
}

func displayBeansWide(beansResponse *actuator.BeansResponse, filter string) error {
	for contextName, appCtx := range beansResponse.Contexts {
		matchingBeans := make(map[string]actuator.Bean)

		for beanName, bean := range appCtx.Beans {
			if filter == "" || strings.Contains(strings.ToLower(beanName), strings.ToLower(filter)) {
				matchingBeans[beanName] = bean
			}
		}

		if len(matchingBeans) == 0 {
			continue
		}

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
				fmt.Printf("  Aliases: %v\n", bean.Aliases)
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
			if filter == "" || strings.Contains(strings.ToLower(beanName), strings.ToLower(filter)) {
				allBeans = append(allBeans, beanInfo{
					name:    beanName,
					context: contextName,
					bean:    bean,
				})
			}
		}
	}

	if len(allBeans) == 0 {
		if filter != "" {
			fmt.Printf("No beans matching filter: %s\n", filter)
		} else {
			fmt.Println("No beans found")
		}
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

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return s[:maxLen]
	}
	return s[:maxLen-1] + "…"
}

func smartTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	lastDot := strings.LastIndex(s, ".")
	var suffix string

	if lastDot != -1 && lastDot < len(s)-1 {
		suffix = s[lastDot+1:]
	} else {
		suffixLen := (maxLen - 3) / 2
		if suffixLen > len(s) {
			suffixLen = len(s)
		}
		suffix = s[len(s)-suffixLen:]
	}

	if len(suffix) > maxLen-1 {
		return "…" + suffix[len(suffix)-(maxLen-1):]
	}

	prefixLen := maxLen - len(suffix) - 1
	if prefixLen < 0 {
		prefixLen = 0
	}

	if prefixLen == 0 {
		return "…" + suffix
	}
	return s[:prefixLen] + "…" + suffix
}

func shortenType(fullType string, maxLen int) string {
	lastDot := strings.LastIndex(fullType, ".")
	if lastDot == -1 {
		return truncateString(fullType, maxLen) // No package, just truncate
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
