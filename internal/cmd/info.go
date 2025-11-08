package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type infoCommandOperations struct {
	baseOperations
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
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := operations.complete(cmd); err != nil {
				return err
			}
			if err := operations.validate(); err != nil {
				return err
			}
			return RunForEachPod(cmd.Context(), operations.pods, "get info", operations.runForPod)
		},
	}

	return cmd
}

func (o *infoCommandOperations) validate() error {
	return o.validatePods()
}

func (o *infoCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

	info, err := client.GetInfo()
	if err != nil {
		return err
	}

	formatInfo(info)

	return nil
}

func formatInfo(info map[string]interface{}) {
	sections := []string{"app", "build", "git"}
	firstSection := true

	for _, section := range sections {
		if data, ok := info[section]; ok {
			if !firstSection {
				fmt.Println()
			}
			firstSection = false

			switch section {
			case "app":
				formatAppSection(data)
			case "build":
				formatBuildSection(data)
			case "git":
				formatGitSection(data)
			}
		}
	}
}

func formatAppSection(data interface{}) {
	appMap, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	fmt.Println("Application:")
	if name, ok := appMap["name"].(string); ok {
		fmt.Printf("  Name:         %s\n", name)
	}
	if description, ok := appMap["description"].(string); ok {
		fmt.Printf("  Description:  %s\n", description)
	}

	for key, value := range appMap {
		if key != "name" && key != "description" {
			fmt.Printf("  %s:  %v\n", capitalizeFirst(key), value)
		}
	}
}

func formatBuildSection(data interface{}) {
	buildMap, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	fmt.Println("Build:")
	if group, ok := buildMap["group"].(string); ok {
		fmt.Printf("  Group:        %s\n", group)
	}
	if artifact, ok := buildMap["artifact"].(string); ok {
		fmt.Printf("  Artifact:     %s\n", artifact)
	}
	if name, ok := buildMap["name"].(string); ok && name != buildMap["artifact"] {
		fmt.Printf("  Name:         %s\n", name)
	}
	if version, ok := buildMap["version"].(string); ok {
		fmt.Printf("  Version:      %s\n", version)
	}
	if time, ok := buildMap["time"]; ok {
		fmt.Printf("  Time:         %v\n", time)
	}
}

func formatGitSection(data interface{}) {
	gitMap, ok := data.(map[string]interface{})
	if !ok {
		return
	}

	fmt.Println("Git:")
	if branch, ok := gitMap["branch"].(string); ok {
		fmt.Printf("  Branch:       %s\n", branch)
	}

	if commit, ok := gitMap["commit"].(map[string]interface{}); ok {
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
				fmt.Printf("  Commit:       %s (%s)\n", commitID, commitTime)
			} else {
				fmt.Printf("  Commit:       %s\n", commitID)
			}
		}
	}
}
