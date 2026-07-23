package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type infoCommandOperations struct {
	baseOperations
	output string
}

func NewInfoCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &infoCommandOperations{
		baseOperations: baseOperations{
			k8sCliFlags: configFlags,
			podResolver: podResolver,
		},
	}

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get application info",
		Long: `Get application info from Spring Boot Actuator.

Displays build information, git details, and other application
information.`,
		Example: `  # Show build and git info of a pod
  kubectl actuator -p my-app-7d4b9c-xk2pq info

  # Show info of all pods in a deployment, as JSON
  kubectl actuator -d my-app info -o json`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.validateFlags(); err != nil {
				return err
			}
			return operations.runEndpoint(cmd, "get info", operations.output, operations.structuredForPod, operations.runForPod)
		},
	}

	addOutputFlag(cmd, &operations.output, "", OutputFormatJSON, OutputFormatYAML)

	return cmd
}

func (o *infoCommandOperations) validateFlags() error {
	return validateOutputFormat(o.output, OutputFormatJSON, OutputFormatYAML)
}

func (o *infoCommandOperations) structuredForPod(client actuator.Client) (json.RawMessage, error) {
	return client.GetRaw("info")
}

func (o *infoCommandOperations) runForPod(ctx context.Context, client actuator.Client, podName string) error {
	info, err := client.GetInfo()
	if err != nil {
		return err
	}

	formatInfo(info)

	return nil
}

// curatedInfoSections are rendered first with dedicated layouts; all other
// sections are rendered generically after them.
var curatedInfoSections = []string{"app", "build", "git"}

func formatInfo(info map[string]any) {
	firstSection := true

	printSeparator := func() {
		if !firstSection {
			fmt.Println()
		}
		firstSection = false
	}

	for _, section := range curatedInfoSections {
		data, ok := info[section]
		if !ok {
			continue
		}
		printSeparator()

		switch section {
		case "app":
			formatAppSection(data)
		case "build":
			formatBuildSection(data)
		case "git":
			formatGitSection(data)
		}
	}

	var remaining []string
	for key := range info {
		if !slices.Contains(curatedInfoSections, key) {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)

	for _, key := range remaining {
		printSeparator()
		formatGenericSection(key, info[key])
	}
}

// formatGenericSection renders an info section this tool has no dedicated
// layout for, e.g. Spring Boot's java/os info or custom InfoContributors.
func formatGenericSection(key string, data any) {
	fmt.Printf("%s:\n", sectionTitle(key))
	if sectionMap, ok := data.(map[string]any); ok {
		printGenericMap(sectionMap, "  ")
		return
	}
	fmt.Printf("  %s\n", formatGenericValue(data))
}

func printGenericMap(section map[string]any, indent string) {
	keys := make([]string, 0, len(section))
	for key := range section {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	labelWidth := 0
	for _, key := range keys {
		if _, nested := section[key].(map[string]any); nested {
			continue
		}
		if width := len(titleCaseKey(key)) + 1; width > labelWidth {
			labelWidth = width
		}
	}

	for _, key := range keys {
		value := section[key]
		if nested, ok := value.(map[string]any); ok {
			fmt.Printf("%s%s:\n", indent, titleCaseKey(key))
			printGenericMap(nested, indent+"  ")
			continue
		}
		fmt.Printf("%s%-*s%s\n", indent, labelWidth+2, titleCaseKey(key)+":", formatGenericValue(value))
	}
}

func formatGenericValue(value any) string {
	if items, ok := value.([]any); ok {
		parts := make([]string, 0, len(items))
		for _, item := range items {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	}
	return fmt.Sprintf("%v", value)
}

func sectionTitle(key string) string {
	if strings.EqualFold(key, "os") {
		return "OS"
	}
	return titleCaseKey(key)
}

func formatAppSection(data any) {
	appMap, ok := data.(map[string]any)
	if !ok {
		return
	}

	fmt.Println("Application:")
	if name, ok := appMap["name"].(string); ok {
		fmt.Printf("  %-14s%s\n", "Name:", name)
	}
	if description, ok := appMap["description"].(string); ok {
		fmt.Printf("  %-14s%s\n", "Description:", description)
	}

	extraKeys := make([]string, 0, len(appMap))
	for key := range appMap {
		if key != "name" && key != "description" {
			extraKeys = append(extraKeys, key)
		}
	}
	sort.Strings(extraKeys)
	for _, key := range extraKeys {
		fmt.Printf("  %-14s%v\n", capitalizeFirst(key)+":", appMap[key])
	}
}

func formatBuildSection(data any) {
	buildMap, ok := data.(map[string]any)
	if !ok {
		return
	}

	fmt.Println("Build:")
	if group, ok := buildMap["group"].(string); ok {
		fmt.Printf("  %-14s%s\n", "Group:", group)
	}
	if artifact, ok := buildMap["artifact"].(string); ok {
		fmt.Printf("  %-14s%s\n", "Artifact:", artifact)
	}
	if name, ok := buildMap["name"].(string); ok && name != buildMap["artifact"] {
		fmt.Printf("  %-14s%s\n", "Name:", name)
	}
	if version, ok := buildMap["version"].(string); ok {
		fmt.Printf("  %-14s%s\n", "Version:", version)
	}
	if time, ok := buildMap["time"]; ok {
		fmt.Printf("  %-14s%v\n", "Time:", time)
	}
}

func formatGitSection(data any) {
	gitMap, ok := data.(map[string]any)
	if !ok {
		return
	}

	fmt.Println("Git:")
	if branch, ok := gitMap["branch"].(string); ok {
		fmt.Printf("  %-14s%s\n", "Branch:", branch)
	}

	if commit, ok := gitMap["commit"].(map[string]any); ok {
		commitID := ""
		commitTime := ""

		if id, ok := commit["id"].(string); ok {
			commitID = id
		}

		if time, ok := commit["time"]; ok {
			commitTime = fmt.Sprintf("%v", time)
		}

		if commitID != "" {
			if commitTime != "" {
				fmt.Printf("  %-14s%s (%s)\n", "Commit:", commitID, commitTime)
			} else {
				fmt.Printf("  %-14s%s\n", "Commit:", commitID)
			}
		}
	}
}
