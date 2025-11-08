package cmd

import (
	"strings"
	"testing"
)

func TestThreadDumpStateValidation(t *testing.T) {
	tests := []struct {
		name        string
		pods        []string
		stateFilter string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid state RUNNABLE",
			pods:        []string{"pod-1"},
			stateFilter: "RUNNABLE",
			wantErr:     false,
		},
		{
			name:        "valid state WAITING",
			pods:        []string{"pod-1"},
			stateFilter: "WAITING",
			wantErr:     false,
		},
		{
			name:        "valid state BLOCKED",
			pods:        []string{"pod-1"},
			stateFilter: "BLOCKED",
			wantErr:     false,
		},
		{
			name:        "valid state TIMED_WAITING",
			pods:        []string{"pod-1"},
			stateFilter: "TIMED_WAITING",
			wantErr:     false,
		},
		{
			name:        "valid state NEW",
			pods:        []string{"pod-1"},
			stateFilter: "NEW",
			wantErr:     false,
		},
		{
			name:        "valid state TERMINATED",
			pods:        []string{"pod-1"},
			stateFilter: "TERMINATED",
			wantErr:     false,
		},
		{
			name:        "lowercase state is normalized to uppercase",
			pods:        []string{"pod-1"},
			stateFilter: "runnable",
			wantErr:     false,
		},
		{
			name:        "mixed case state is normalized",
			pods:        []string{"pod-1"},
			stateFilter: "Blocked",
			wantErr:     false,
		},
		{
			name:        "invalid state INVALID",
			pods:        []string{"pod-1"},
			stateFilter: "INVALID",
			wantErr:     true,
			errContains: "invalid thread state",
		},
		{
			name:        "invalid state RUNNING",
			pods:        []string{"pod-1"},
			stateFilter: "RUNNING",
			wantErr:     true,
			errContains: "invalid thread state",
		},
		{
			name:        "invalid state SLEEPING",
			pods:        []string{"pod-1"},
			stateFilter: "SLEEPING",
			wantErr:     true,
			errContains: "invalid thread state",
		},
		{
			name:        "empty state filter is valid (no filtering)",
			pods:        []string{"pod-1"},
			stateFilter: "",
			wantErr:     false,
		},
		{
			name:        "no pods specified",
			pods:        []string{},
			stateFilter: "RUNNABLE",
			wantErr:     true,
			errContains: "no pods selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &threaddumpCommandOperations{
				baseOperations: baseOperations{pods: tt.pods},
				stateFilter:    tt.stateFilter,
			}

			err := ops.validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errContains, err)
				}
			}
		})
	}
}

func TestThreadDumpStateNormalization(t *testing.T) {
	tests := []struct {
		name          string
		inputState    string
		expectedState string
	}{
		{
			name:          "uppercase stays uppercase",
			inputState:    "RUNNABLE",
			expectedState: "RUNNABLE",
		},
		{
			name:          "lowercase normalized to uppercase",
			inputState:    "blocked",
			expectedState: "BLOCKED",
		},
		{
			name:          "mixed case normalized to uppercase",
			inputState:    "Waiting",
			expectedState: "WAITING",
		},
		{
			name:          "timed_waiting lowercase",
			inputState:    "timed_waiting",
			expectedState: "TIMED_WAITING",
		},
		{
			name:          "empty state stays empty",
			inputState:    "",
			expectedState: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &threaddumpCommandOperations{
				baseOperations: baseOperations{pods: []string{"pod-1"}},
				stateFilter:    tt.inputState,
			}

			err := ops.validate()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if ops.stateFilter != tt.expectedState {
				t.Errorf("stateFilter = %q, want %q", ops.stateFilter, tt.expectedState)
			}
		})
	}
}

func TestThreadDumpValidationErrorMessages(t *testing.T) {
	tests := []struct {
		name            string
		pods            []string
		stateFilter     string
		wantErrContains []string
	}{
		{
			name:        "invalid state error shows the invalid value",
			pods:        []string{"pod-1"},
			stateFilter: "INVALID_STATE",
			wantErrContains: []string{
				"invalid thread state",
				"INVALID_STATE",
			},
		},
		{
			name:        "invalid state error shows valid states",
			pods:        []string{"pod-1"},
			stateFilter: "WRONG",
			wantErrContains: []string{
				"Valid states",
				"RUNNABLE",
				"BLOCKED",
				"WAITING",
			},
		},
		{
			name:        "no pods error mentions pods",
			pods:        []string{},
			stateFilter: "RUNNABLE",
			wantErrContains: []string{
				"no pods selected",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &threaddumpCommandOperations{
				baseOperations: baseOperations{pods: tt.pods},
				stateFilter:    tt.stateFilter,
			}

			err := ops.validate()

			if err == nil {
				t.Error("expected error, got nil")
				return
			}

			errMsg := err.Error()
			for _, want := range tt.wantErrContains {
				if !strings.Contains(errMsg, want) {
					t.Errorf("error message does not contain '%s'\nGot: %s", want, errMsg)
				}
			}
		})
	}
}

func TestValidThreadStatesConstant(t *testing.T) {
	expectedStates := []string{"NEW", "RUNNABLE", "BLOCKED", "WAITING", "TIMED_WAITING", "TERMINATED"}

	if len(validThreadStates) != len(expectedStates) {
		t.Errorf("validThreadStates has %d elements, expected %d", len(validThreadStates), len(expectedStates))
	}

	stateMap := make(map[string]bool)
	for _, state := range validThreadStates {
		stateMap[state] = true
	}

	for _, expected := range expectedStates {
		if !stateMap[expected] {
			t.Errorf("expected state %s not found in validThreadStates", expected)
		}
	}
}
