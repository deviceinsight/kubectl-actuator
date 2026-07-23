package cmd

import (
	"strings"
	"testing"
)

func TestRequireSingleTargetForOutputFile(t *testing.T) {
	tests := []struct {
		name       string
		pods       []string
		outputFile string
		wantErr    bool
	}{
		{name: "single pod default filename", pods: []string{"pod-1"}},
		{name: "single pod explicit file", pods: []string{"pod-1"}, outputFile: "dump.hprof"},
		{name: "multiple pods default filenames", pods: []string{"pod-1", "pod-2"}},
		{name: "multiple pods with explicit file", pods: []string{"pod-1", "pod-2"}, outputFile: "dump.hprof", wantErr: true},
		{name: "multiple pods with stdout", pods: []string{"pod-1", "pod-2"}, outputFile: "-", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &baseOperations{pods: tt.pods}

			err := ops.requireSingleTargetForOutputFile(tt.outputFile)
			if (err != nil) != tt.wantErr {
				t.Fatalf("requireSingleTargetForOutputFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "single target pod") {
				t.Errorf("expected error containing %q, got %q", "single target pod", err.Error())
			}
		})
	}
}

func TestLogFileValidateFlags(t *testing.T) {
	t.Run("negative tail bytes are rejected", func(t *testing.T) {
		ops := &logfileCommandOperations{tailBytes: -1}
		if err := ops.validateFlags(); err == nil || !strings.Contains(err.Error(), "cannot be negative") {
			t.Errorf("expected the negative tail-bytes error, got %v", err)
		}
	})

	t.Run("zero and positive tail bytes are valid", func(t *testing.T) {
		for _, tail := range []int64{0, 65536} {
			ops := &logfileCommandOperations{tailBytes: tail}
			if err := ops.validateFlags(); err != nil {
				t.Errorf("tailBytes %d: %v", tail, err)
			}
		}
	})
}
