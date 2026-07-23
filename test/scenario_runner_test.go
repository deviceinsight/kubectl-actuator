package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample-tests.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

func TestParseScenarioFileStreams(t *testing.T) {
	path := writeTestFile(t, `
-- test: stream sections --
-- command --
kubectl-actuator health
-- expect --
combined
-- expect:stdout --
on stdout
-- expect:stderr --
on stderr
-- expect:not:stdout --
not on stdout
-- expect:error --
fails
`)

	scenarios, err := ParseScenarioFile(path)
	if err != nil {
		t.Fatalf("ParseScenarioFile() error = %v", err)
	}
	if len(scenarios) != 1 || len(scenarios[0].Steps) != 1 {
		t.Fatalf("expected 1 scenario with 1 step, got %+v", scenarios)
	}

	step := scenarios[0].Steps[0]
	if !step.ExpectFailure {
		t.Error("expect:error should set ExpectFailure")
	}
	if step.Line != 3 {
		t.Errorf("step line = %d, want 3", step.Line)
	}

	want := []Expectation{
		{Pattern: "combined", Line: 5},
		{Pattern: "on stdout", Stream: StreamStdout, Line: 7},
		{Pattern: "on stderr", Stream: StreamStderr, Line: 9},
		{Pattern: "not on stdout", Stream: StreamStdout, Negate: true, Line: 11},
		{Pattern: "fails", Line: 13},
	}
	if len(step.Expectations) != len(want) {
		t.Fatalf("expected %d expectations, got %d: %+v", len(want), len(step.Expectations), step.Expectations)
	}
	for i, expectation := range want {
		if step.Expectations[i] != expectation {
			t.Errorf("expectation %d = %+v, want %+v", i, step.Expectations[i], expectation)
		}
	}
}

func TestParseScenarioFileRejectsUnknownSection(t *testing.T) {
	path := writeTestFile(t, `
-- test: typo --
-- command --
kubectl-actuator health
-- expect:sderr --
oops
`)

	_, err := ParseScenarioFile(path)
	if err == nil {
		t.Fatal("expected error for unknown section")
	}
	if !strings.Contains(err.Error(), "unknown section") || !strings.Contains(err.Error(), "expect:sderr") {
		t.Errorf("error should name the unknown section, got: %v", err)
	}
	if !strings.Contains(err.Error(), ":5:") {
		t.Errorf("error should include the line number, got: %v", err)
	}
}

func TestParseScenarioFileRejectsEmptySection(t *testing.T) {
	path := writeTestFile(t, `
-- test: empty expectation --
-- command --
kubectl-actuator health
-- expect --
-- expect --
something
`)

	_, err := ParseScenarioFile(path)
	if err == nil {
		t.Fatal("expected error for empty section")
	}
	if !strings.Contains(err.Error(), "is empty") {
		t.Errorf("error should mention the empty section, got: %v", err)
	}
}

func TestParseScenarioFileRejectsContentOutsideSection(t *testing.T) {
	path := writeTestFile(t, `
-- test: near-miss header --
--- expect ---
this line and the header above belong to no section
`)

	_, err := ParseScenarioFile(path)
	if err == nil {
		t.Fatal("expected error for content outside a section")
	}
	if !strings.Contains(err.Error(), "outside of a section") || !strings.Contains(err.Error(), ":3:") {
		t.Errorf("error should name the stray content and its line, got: %v", err)
	}
}

func TestSubstituteTemplates(t *testing.T) {
	ctx := TemplateContext{
		Pods:       []string{"pod-a", "pod-b"},
		Deployment: "my-app",
		Namespace:  "default",
	}

	t.Run("known templates are substituted", func(t *testing.T) {
		got, err := SubstituteTemplates("-p {{pod}},{{pod[1]}} -d {{deployment}} -n {{namespace}}", ctx)
		if err != nil {
			t.Fatalf("SubstituteTemplates() error = %v", err)
		}
		want := "-p pod-a,pod-b -d my-app -n default"
		if got != want {
			t.Errorf("SubstituteTemplates() = %q, want %q", got, want)
		}
	})

	t.Run("typo'd template is an error naming the leftover", func(t *testing.T) {
		_, err := SubstituteTemplates("-d {{deployement}} health", ctx)
		if err == nil || !strings.Contains(err.Error(), "{{deployement}}") {
			t.Errorf("expected an error naming the unknown template, got: %v", err)
		}
	})

	t.Run("pod index beyond the pod count is an error", func(t *testing.T) {
		_, err := SubstituteTemplates("-p {{pod[9]}} health", ctx)
		if err == nil || !strings.Contains(err.Error(), "{{pod[9]}}") {
			t.Errorf("expected an error naming the unknown template, got: %v", err)
		}
	})
}

func TestParseScenarioFileRejectsExpectationBeforeCommand(t *testing.T) {
	path := writeTestFile(t, `
-- test: no command --
-- expect --
something
`)

	_, err := ParseScenarioFile(path)
	if err == nil {
		t.Fatal("expected error for expectation before any command")
	}
	if !strings.Contains(err.Error(), "before any command") {
		t.Errorf("error should mention missing command, got: %v", err)
	}
}

func TestParseScenarioFileExistingDefinitionsStillParse(t *testing.T) {
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil || len(files) == 0 {
		t.Fatalf("failed to glob testdata: %v", err)
	}
	for _, file := range files {
		if _, err := ParseScenarioFile(file); err != nil {
			t.Errorf("ParseScenarioFile(%s) error = %v", file, err)
		}
	}
}

func TestSplitCommandQuoting(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{
			input: `kubectl-actuator --pod my-pod health`,
			want:  []string{"kubectl-actuator", "--pod", "my-pod", "health"},
		},
		{
			input: `kubectl-actuator metrics jvm.memory.used --tag "id=G1 Old Gen"`,
			want:  []string{"kubectl-actuator", "metrics", "jvm.memory.used", "--tag", "id=G1 Old Gen"},
		},
		{
			input: `kubectl-actuator logger 'com.example.My Logger' DEBUG`,
			want:  []string{"kubectl-actuator", "logger", "com.example.My Logger", "DEBUG"},
		},
		{
			input:   `kubectl-actuator "unterminated`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := splitCommand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("splitCommand(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitCommand(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("arg %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMatchTarget(t *testing.T) {
	stdout, stderr, combined := "out", "err", "outerr"

	tests := []struct {
		name   string
		expect Expectation
		want   string
	}{
		{"combined default", Expectation{}, combined},
		{"stdout stream", Expectation{Stream: StreamStdout}, stdout},
		{"stderr stream", Expectation{Stream: StreamStderr}, stderr},
		{"jsonpath reads stdout", Expectation{IsJSONPath: true}, stdout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchTarget(tt.expect, stdout, stderr, combined); got != tt.want {
				t.Errorf("matchTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}
