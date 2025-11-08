package cmd

import (
	"context"
	"fmt"
)

// PodFunc is a function that processes a single pod and returns an error if it fails.
type PodFunc func(ctx context.Context, pod string) error

// RunForEachPod executes the given function for each pod, handling context cancellation,
// pod headers for multi-pod output, and error aggregation.
// Returns an error with the count of failed pods if any pod fails.
func RunForEachPod(ctx context.Context, pods []string, action string, fn PodFunc) error {
	size := len(pods)
	var failedPods []string

	for i, pod := range pods {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if size > 1 {
			fmt.Printf("%s:\n", pod)
		}

		if err := fn(ctx, pod); err != nil {
			fmt.Printf("Error: %v\n", err)
			failedPods = append(failedPods, pod)
		}

		if i != size-1 {
			fmt.Println()
		}
	}

	if len(failedPods) > 0 {
		return fmt.Errorf("%s failed on %d pod(s)", action, len(failedPods))
	}

	return nil
}
