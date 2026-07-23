package cmd

import (
	"strings"
	"testing"
)

func TestThreadDumpValidateFlags(t *testing.T) {
	t.Run("every valid state is accepted, in any case", func(t *testing.T) {
		for _, state := range validThreadStates {
			for _, variant := range []string{state, strings.ToLower(state)} {
				ops := &threaddumpCommandOperations{stateFilter: variant}
				if err := ops.validateFlags(); err != nil {
					t.Errorf("state %q: %v", variant, err)
				}
			}
		}
	})

	t.Run("empty state filter means no filtering", func(t *testing.T) {
		ops := &threaddumpCommandOperations{}
		if err := ops.validateFlags(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	invalid := []struct {
		name        string
		state       string
		errContains []string
	}{
		{"invalid state INVALID", "INVALID", []string{"invalid thread state", "INVALID", "Valid states", "RUNNABLE"}},
		{"invalid state RUNNING", "RUNNING", []string{"invalid thread state"}},
		{"invalid state SLEEPING", "SLEEPING", []string{"invalid thread state"}},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			ops := &threaddumpCommandOperations{stateFilter: tt.state}
			err := ops.validateFlags()
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, want := range tt.errContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected error containing %q, got %q", want, err)
				}
			}
		})
	}

	t.Run("--summary conflicts with structured output", func(t *testing.T) {
		ops := &threaddumpCommandOperations{output: OutputFormatJSON, summary: true}
		if err := ops.validateFlags(); err == nil || !strings.Contains(err.Error(), "--summary") {
			t.Errorf("expected the --summary conflict error, got %v", err)
		}
	})

	t.Run("--no-stacktrace conflicts with structured output", func(t *testing.T) {
		ops := &threaddumpCommandOperations{output: OutputFormatYAML, noStacktrace: true}
		if err := ops.validateFlags(); err == nil || !strings.Contains(err.Error(), "--no-stacktrace") {
			t.Errorf("expected the --no-stacktrace conflict error, got %v", err)
		}
	})
}
