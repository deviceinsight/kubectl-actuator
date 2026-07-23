package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

// fakeClientFactory returns nil clients (fine for callbacks that ignore
// them) and configurable per-pod creation errors.
type fakeClientFactory struct {
	errors map[string]error
}

func (f *fakeClientFactory) NewClient(_ context.Context, podName string) (actuator.Client, error) {
	if err, ok := f.errors[podName]; ok {
		return nil, err
	}
	return nil, nil
}

func TestRunForEachPod(t *testing.T) {
	tests := []struct {
		name              string
		pods              []string
		fnResults         map[string]error
		clientErrors      map[string]error
		wantErr           bool
		errContains       string
		wantOutContain    []string
		wantStderrContain []string
	}{
		{
			name:      "single pod success",
			pods:      []string{"pod-1"},
			fnResults: map[string]error{"pod-1": nil},
			wantErr:   false,
		},
		{
			name:      "multiple pods all success",
			pods:      []string{"pod-1", "pod-2", "pod-3"},
			fnResults: map[string]error{"pod-1": nil, "pod-2": nil, "pod-3": nil},
			wantErr:   false,
			wantOutContain: []string{
				"pod-1:",
				"pod-2:",
				"pod-3:",
			},
		},
		{
			// A single-pod failure is returned directly so it is reported
			// exactly once, by cobra, instead of stderr plus an aggregate.
			name:        "single pod failure returns the concrete error",
			pods:        []string{"pod-1"},
			fnResults:   map[string]error{"pod-1": errors.New("connection failed")},
			wantErr:     true,
			errContains: "connection failed",
		},
		{
			name: "multiple pods partial failure",
			pods: []string{"pod-1", "pod-2", "pod-3"},
			fnResults: map[string]error{
				"pod-1": nil,
				"pod-2": errors.New("timeout"),
				"pod-3": nil,
			},
			wantErr:     true,
			errContains: "test failed on 1 of 3 pods: pod-2",
			wantOutContain: []string{
				"pod-2:",
			},
			wantStderrContain: []string{
				"Error (pod-2): timeout",
			},
		},
		{
			name: "multiple pods all failure",
			pods: []string{"pod-1", "pod-2"},
			fnResults: map[string]error{
				"pod-1": errors.New("error 1"),
				"pod-2": errors.New("error 2"),
			},
			wantErr:     true,
			errContains: "test failed on 2 of 2 pods: pod-1, pod-2",
		},
		{
			name:         "client creation failure counts as pod failure",
			pods:         []string{"pod-1", "pod-2"},
			fnResults:    map[string]error{"pod-2": nil},
			clientErrors: map[string]error{"pod-1": errors.New("pod not found")},
			wantErr:      true,
			errContains:  "test failed on 1 of 2 pods: pod-1",
			wantStderrContain: []string{
				"Error (pod-1): pod not found",
			},
		},
		{
			name:      "empty pods list",
			pods:      []string{},
			fnResults: map[string]error{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &baseOperations{
				pods:                  tt.pods,
				actuatorClientFactory: &fakeClientFactory{errors: tt.clientErrors},
			}

			fn := func(ctx context.Context, client actuator.Client, pod string) error {
				return tt.fnResults[pod]
			}

			var err error
			var output string
			stderrOutput := captureStderr(func() {
				output = captureOutput(func() {
					err = ops.runForEachPod(context.Background(), "test", fn)
				})
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("runForEachPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want containing %q", err, tt.errContains)
				}
			}

			for _, want := range tt.wantOutContain {
				if !strings.Contains(output, want) {
					t.Errorf("stdout missing %q\nGot: %s", want, output)
				}
			}

			for _, want := range tt.wantStderrContain {
				if !strings.Contains(stderrOutput, want) {
					t.Errorf("stderr missing %q\nGot: %s", want, stderrOutput)
				}
			}
		})
	}
}

func TestRunForEachPodContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	callCount := 0
	fn := func(ctx context.Context, client actuator.Client, pod string) error {
		callCount++
		return nil
	}

	ops := &baseOperations{
		pods:                  []string{"pod-1", "pod-2"},
		actuatorClientFactory: &fakeClientFactory{},
	}

	err := ops.runForEachPod(ctx, "test", fn)

	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("expected ErrInterrupted, got %v", err)
	}

	if callCount != 0 {
		t.Errorf("expected 0 calls, got %d", callCount)
	}
}

func TestRunForEachPodCancellationMidLoopIsInterrupt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	fn := func(ctx context.Context, client actuator.Client, pod string) error {
		cancel()
		return context.Canceled
	}

	ops := &baseOperations{
		pods:                  []string{"pod-1", "pod-2"},
		actuatorClientFactory: &fakeClientFactory{},
	}

	var err error
	stderr := captureStderr(func() {
		err = ops.runForEachPod(ctx, "test", fn)
	})

	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("expected ErrInterrupted, got %v", err)
	}
	if strings.Contains(stderr, "Error") {
		t.Errorf("an interrupt must not be reported as a pod failure, got stderr: %s", stderr)
	}
}
