package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"sigs.k8s.io/yaml"
)

// podResult is one pod's entry in the structured output envelope shared by
// all commands that support -o json/yaml, including the raw command.
type podResult struct {
	Name  string          `json:"name"`
	Data  json.RawMessage `json:"data"`
	Error *string         `json:"error"`
}

type structuredOutput struct {
	Pods []podResult `json:"pods"`
}

// PodDataFunc fetches the structured payload for a single pod.
type PodDataFunc func(client actuator.Client) (json.RawMessage, error)

// runStructured executes fn for each pod and prints all results as a single
// JSON or YAML document. Per-pod failures are recorded in the envelope's
// error field; if any pod failed, an error is returned after printing so
// the exit code reflects the failure. An interrupt still marshals the
// envelope, with pods not yet queried marked accordingly.
func (b *baseOperations) runStructured(ctx context.Context, format, action string, fn PodDataFunc) error {
	output := structuredOutput{Pods: make([]podResult, 0, len(b.pods))}
	var failedPods []string
	var firstErr error
	interrupted := false

	for _, pod := range b.pods {
		result := podResult{Name: pod}

		if interrupted || ctx.Err() != nil {
			interrupted = true
			msg := "interrupted before this pod was queried"
			result.Error = &msg
			output.Pods = append(output.Pods, result)
			continue
		}

		data, err := b.podData(ctx, pod, fn)
		switch {
		case err == nil:
			result.Data = data
		case errors.Is(err, context.Canceled):
			interrupted = true
			msg := ErrInterrupted.Error()
			result.Error = &msg
		default:
			msg := err.Error()
			result.Error = &msg
			failedPods = append(failedPods, pod)
			if firstErr == nil {
				firstErr = err
			}
		}
		output.Pods = append(output.Pods, result)
	}

	rendered, err := marshalStructured(output, format)
	if err != nil {
		return fmt.Errorf("failed to marshal output: %w", err)
	}
	fmt.Print(string(rendered))

	if interrupted {
		return ErrInterrupted
	}
	if len(b.pods) == 1 && firstErr != nil {
		return firstErr
	}
	if len(failedPods) > 0 {
		return fmt.Errorf("%s failed on %d of %d pods: %s", action, len(failedPods), len(b.pods), strings.Join(failedPods, ", "))
	}
	return nil
}

func (b *baseOperations) podData(ctx context.Context, pod string, fn PodDataFunc) (json.RawMessage, error) {
	client, err := b.actuatorClientFactory.NewClient(ctx, pod)
	if err != nil {
		return nil, err
	}
	return fn(client)
}

// marshalStructured renders the envelope as indented JSON or as YAML derived
// from the same JSON bytes, so both formats always carry identical data.
func marshalStructured(output structuredOutput, format string) ([]byte, error) {
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, err
	}
	if format == OutputFormatYAML {
		return yaml.JSONToYAML(jsonBytes)
	}
	return append(jsonBytes, '\n'), nil
}

// decodeTree parses raw endpoint JSON into a generic tree for filtering.
// Numbers stay json.Number so re-encoding does not alter their
// representation, and fields unknown to this tool survive the round trip.
func decodeTree(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var tree map[string]any
	if err := decoder.Decode(&tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func encodeTree(tree map[string]any) (json.RawMessage, error) {
	return json.Marshal(tree)
}
