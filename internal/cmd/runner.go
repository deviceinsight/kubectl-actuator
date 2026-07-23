package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

// PodFunc processes a single pod using a ready actuator client.
type PodFunc func(ctx context.Context, client actuator.Client, podName string) error

// runForEachPod executes fn for each selected pod with a ready actuator
// client, printing pod headers for multi-pod output and aggregating failures.
// Per-pod errors go to stderr so stdout carries only command output; a
// single-pod failure is returned directly so it is reported exactly once.
// An interrupt stops the loop immediately and is never counted as a pod
// failure.
func (b *baseOperations) runForEachPod(ctx context.Context, action string, fn PodFunc) error {
	return b.runForEachPodTo(ctx, os.Stdout, action, fn)
}

// runForEachPodTo is runForEachPod with the pod headers and separators sent
// to headerDst; download commands pass stderr so stdout carries nothing but
// payload data.
func (b *baseOperations) runForEachPodTo(ctx context.Context, headerDst io.Writer, action string, fn PodFunc) error {
	size := len(b.pods)
	var failedPods []string

	for i, pod := range b.pods {
		if ctx.Err() != nil {
			return ErrInterrupted
		}

		if size > 1 {
			_, _ = fmt.Fprintf(headerDst, "%s:\n", pod)
		}

		client, err := b.actuatorClientFactory.NewClient(ctx, pod)
		if err == nil {
			err = fn(ctx, client, pod)
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return ErrInterrupted
			}
			if size == 1 {
				return err
			}
			_, _ = fmt.Fprintf(os.Stderr, "Error (%s): %v\n", pod, err)
			failedPods = append(failedPods, pod)
		}

		if i != size-1 {
			_, _ = fmt.Fprintln(headerDst)
		}
	}

	if len(failedPods) > 0 {
		return fmt.Errorf("%s failed on %d of %d pods: %s", action, len(failedPods), size, strings.Join(failedPods, ", "))
	}

	return nil
}
