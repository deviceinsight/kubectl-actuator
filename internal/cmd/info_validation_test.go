package cmd

import (
	"strings"
	"testing"
)

func TestInfoCommandValidation(t *testing.T) {
	tests := []struct {
		name        string
		pods        []string
		wantErr     bool
		errContains string
	}{
		{
			name:    "single pod valid",
			pods:    []string{"pod-1"},
			wantErr: false,
		},
		{
			name:    "multiple pods valid",
			pods:    []string{"pod-1", "pod-2", "pod-3"},
			wantErr: false,
		},
		{
			name:        "no pods specified",
			pods:        []string{},
			wantErr:     true,
			errContains: "no pods selected",
		},
		{
			name:        "nil pods",
			pods:        nil,
			wantErr:     true,
			errContains: "no pods selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &infoCommandOperations{
				baseOperations: baseOperations{pods: tt.pods},
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

func TestInfoCommandErrorMessages(t *testing.T) {
	ops := &infoCommandOperations{
		baseOperations: baseOperations{pods: []string{}},
	}

	err := ops.validate()

	if err == nil {
		t.Fatal("expected error for no pods, got nil")
	}

	errMsg := err.Error()
	expectedPhrases := []string{
		"no pods selected",
		"--pod",
		"--deployment",
		"--selector",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(errMsg, phrase) {
			t.Errorf("error message does not contain '%s'\nGot: %s", phrase, errMsg)
		}
	}
}

func TestInfoCommandWithDifferentPodCounts(t *testing.T) {
	tests := []struct {
		name     string
		podCount int
		wantErr  bool
	}{
		{"zero pods", 0, true},
		{"one pod", 1, false},
		{"two pods", 2, false},
		{"five pods", 5, false},
		{"ten pods", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pods := make([]string, tt.podCount)
			for i := 0; i < tt.podCount; i++ {
				pods[i] = "pod-" + string(rune('a'+i))
			}

			ops := &infoCommandOperations{
				baseOperations: baseOperations{pods: pods},
			}

			err := ops.validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("validate() with %d pods: error = %v, wantErr %v", tt.podCount, err, tt.wantErr)
			}
		})
	}
}
