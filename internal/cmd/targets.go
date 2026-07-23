package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
)

var ErrNoPodsSelected = errors.New("no pods selected: use -p <pod>, -d <deployment>, or -l <label-selector>")

// ErrTargetsMatchedNoPods is returned when target flags were provided but
// none of them resolved to a pod; it names the namespace that was searched,
// the deployments and selectors that came up empty so the user is not told
// to pass flags they already did.
type ErrTargetsMatchedNoPods struct {
	Deployments []string
	Selectors   []string
	Namespace   string
}

func (e *ErrTargetsMatchedNoPods) Error() string {
	var parts []string
	if len(e.Deployments) > 0 {
		noun, verb := "deployment", "has"
		if len(e.Deployments) > 1 {
			noun, verb = "deployments", "have"
		}
		parts = append(parts, fmt.Sprintf("%s %s %s no pods", noun, quoteList(e.Deployments), verb))
	}
	if len(e.Selectors) > 0 {
		noun := "selector"
		if len(e.Selectors) > 1 {
			noun = "selectors"
		}
		parts = append(parts, fmt.Sprintf("%s %s matched no pods", noun, quoteList(e.Selectors)))
	}
	msg := strings.Join(parts, "; ")
	if e.Namespace != "" {
		msg += fmt.Sprintf(" in namespace %q", e.Namespace)
	}
	return msg
}

func quoteList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}

// PodResolver turns the command's target flags into a list of pod names.
type PodResolver func(ctx context.Context, k8sClient k8s.Client, cmd *cobra.Command) ([]string, error)

// FlagsPodResolver resolves pods based on global --pod/--deployment/--selector flags
func FlagsPodResolver(ctx context.Context, k8sClient k8s.Client, cmd *cobra.Command) ([]string, error) {
	root := cmd.Root()
	pods, err := root.PersistentFlags().GetStringSlice("pod")
	if err != nil {
		return nil, err
	}
	deployments, err := root.PersistentFlags().GetStringSlice("deployment")
	if err != nil {
		return nil, err
	}
	selectors, err := root.PersistentFlags().GetStringArray("selector")
	if err != nil {
		return nil, err
	}

	var deploymentsWithNoPods []string
	for _, d := range deployments {
		names, err := k8sClient.GetDeploymentPods(ctx, d)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			deploymentsWithNoPods = append(deploymentsWithNoPods, d)
		}
		pods = append(pods, names...)
	}

	var selectorsWithNoMatches []string
	for _, s := range selectors {
		names, err := k8sClient.ListPods(ctx, s)
		if err != nil {
			return nil, err
		}
		if len(names) == 0 {
			selectorsWithNoMatches = append(selectorsWithNoMatches, s)
		}
		pods = append(pods, names...)
	}

	seen := map[string]struct{}{}
	var result []string
	for _, p := range pods {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			result = append(result, p)
		}
	}

	// If targets were provided but resolved to no pods, name the empty ones
	if len(result) == 0 && (len(deploymentsWithNoPods) > 0 || len(selectorsWithNoMatches) > 0) {
		return nil, &ErrTargetsMatchedNoPods{
			Deployments: deploymentsWithNoPods,
			Selectors:   selectorsWithNoMatches,
			Namespace:   k8sClient.Namespace(),
		}
	}

	return result, nil
}
