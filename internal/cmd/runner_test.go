package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunForEachPod(t *testing.T) {
	tests := []struct {
		name           string
		pods           []string
		fnResults      map[string]error
		wantErr        bool
		errContains    string
		wantOutContain []string
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
			name:        "single pod failure",
			pods:        []string{"pod-1"},
			fnResults:   map[string]error{"pod-1": errors.New("connection failed")},
			wantErr:     true,
			errContains: "test failed on 1 pod(s)",
			wantOutContain: []string{
				"Error: connection failed",
			},
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
			errContains: "test failed on 1 pod(s)",
			wantOutContain: []string{
				"pod-2:",
				"Error: timeout",
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
			errContains: "test failed on 2 pod(s)",
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
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			fn := func(ctx context.Context, pod string) error {
				return tt.fnResults[pod]
			}

			err := RunForEachPod(context.Background(), tt.pods, "test", fn)

			// Restore stdout and read captured output
			_ = w.Close()
			os.Stdout = oldStdout
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			if (err != nil) != tt.wantErr {
				t.Errorf("RunForEachPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want containing %q", err, tt.errContains)
				}
			}

			for _, want := range tt.wantOutContain {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q\nGot: %s", want, output)
				}
			}
		})
	}
}

func TestRunForEachPodContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	callCount := 0
	fn := func(ctx context.Context, pod string) error {
		callCount++
		return nil
	}

	err := RunForEachPod(ctx, []string{"pod-1", "pod-2"}, "test", fn)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	if callCount != 0 {
		t.Errorf("expected 0 calls, got %d", callCount)
	}
}
