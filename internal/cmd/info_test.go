package cmd

import (
	"slices"
	"strings"
	"testing"
)

func TestFormatInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     map[string]any
		expected []string // lines that should be present
	}{
		{
			name: "all sections present",
			info: map[string]any{
				"app": map[string]any{
					"name":        "test-app",
					"description": "Test Application",
				},
				"build": map[string]any{
					"group":    "com.example",
					"artifact": "test-app",
					"version":  "1.0.0",
				},
				"git": map[string]any{
					"branch": "main",
					"commit": map[string]any{
						"id":   "abc123",
						"time": "2024-11-30T10:00:00Z",
					},
				},
			},
			expected: []string{
				"Application:",
				"  Name:         test-app",
				"  Description:  Test Application",
				"Build:",
				"  Group:        com.example",
				"  Artifact:     test-app",
				"  Version:      1.0.0",
				"Git:",
				"  Branch:       main",
				"  Commit:       abc123 (2024-11-30T10:00:00Z)",
			},
		},
		{
			name: "only app section",
			info: map[string]any{
				"app": map[string]any{
					"name": "test-app",
				},
			},
			expected: []string{
				"Application:",
				"  Name:         test-app",
			},
		},
		{
			name: "only build section",
			info: map[string]any{
				"build": map[string]any{
					"version": "2.0.0",
				},
			},
			expected: []string{
				"Build:",
				"  Version:      2.0.0",
			},
		},
		{
			name:     "empty info",
			info:     map[string]any{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				formatInfo(tt.info)
			})

			if len(tt.expected) == 0 && output != "" {
				t.Errorf("formatInfo() expected no output, got:\n%s", output)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("formatInfo() output missing expected line:\n  want: %s\n  got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestFormatAppSection(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected []string
	}{
		{
			name: "complete app info",
			data: map[string]any{
				"name":        "my-app",
				"description": "My Application",
			},
			expected: []string{
				"Application:",
				"  Name:         my-app",
				"  Description:  My Application",
			},
		},
		{
			name: "app with custom fields",
			data: map[string]any{
				"name":    "my-app",
				"version": "1.0",
				"author":  "John Doe",
			},
			expected: []string{
				"Application:",
				"  Name:         my-app",
				"  Author:       John Doe",
				"  Version:      1.0",
			},
		},
		{
			name: "only name",
			data: map[string]any{
				"name": "simple-app",
			},
			expected: []string{
				"Application:",
				"  Name:         simple-app",
			},
		},
		{
			name:     "invalid data type",
			data:     "not a map",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				formatAppSection(tt.data)
			})

			if len(tt.expected) == 0 && output != "" {
				t.Errorf("formatAppSection() expected no output, got:\n%s", output)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("formatAppSection() output missing expected line:\n  want: %s\n  got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestFormatBuildSection(t *testing.T) {
	tests := []struct {
		name        string
		data        any
		expected    []string
		notExpected []string
	}{
		{
			name: "complete build info",
			data: map[string]any{
				"group":    "com.example",
				"artifact": "my-artifact",
				"name":     "my-artifact",
				"version":  "1.0.0",
				"time":     "2024-11-30T10:00:00Z",
			},
			expected: []string{
				"Build:",
				"  Group:        com.example",
				"  Artifact:     my-artifact",
				"  Version:      1.0.0",
				"  Time:         2024-11-30T10:00:00Z",
			},
			// Should skip name when it's same as artifact
			notExpected: []string{
				"  Name:",
			},
		},
		{
			name: "build with different name",
			data: map[string]any{
				"artifact": "my-artifact",
				"name":     "different-name",
			},
			expected: []string{
				"Build:",
				"  Artifact:     my-artifact",
				"  Name:         different-name",
			},
		},
		{
			name: "minimal build info",
			data: map[string]any{
				"version": "1.0.0",
			},
			expected: []string{
				"Build:",
				"  Version:      1.0.0",
			},
		},
		{
			name:     "invalid data type",
			data:     123,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				formatBuildSection(tt.data)
			})

			if len(tt.expected) == 0 && output != "" {
				t.Errorf("formatBuildSection() expected no output, got:\n%s", output)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("formatBuildSection() output missing expected line:\n  want: %s\n  got:\n%s", expected, output)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("formatBuildSection() output contains unexpected line:\n  don't want: %s\n  got:\n%s", notExpected, output)
				}
			}
		})
	}
}

func TestFormatGitSection(t *testing.T) {
	tests := []struct {
		name     string
		data     any
		expected []string
	}{
		{
			name: "complete git info with commit time",
			data: map[string]any{
				"branch": "main",
				"commit": map[string]any{
					"id":   "abc123def456",
					"time": "2024-11-30T10:00:00Z",
				},
			},
			expected: []string{
				"Git:",
				"  Branch:       main",
				"  Commit:       abc123def456 (2024-11-30T10:00:00Z)",
			},
		},
		{
			name: "git info without commit time",
			data: map[string]any{
				"branch": "develop",
				"commit": map[string]any{
					"id": "xyz789",
				},
			},
			expected: []string{
				"Git:",
				"  Branch:       develop",
				"  Commit:       xyz789",
			},
		},
		{
			name: "only branch",
			data: map[string]any{
				"branch": "feature/new-feature",
			},
			expected: []string{
				"Git:",
				"  Branch:       feature/new-feature",
			},
		},
		{
			name: "commit without id",
			data: map[string]any{
				"branch": "main",
				"commit": map[string]any{
					"time": "2024-11-30T10:00:00Z",
				},
			},
			expected: []string{
				"Git:",
				"  Branch:       main",
			},
		},
		{
			name:     "invalid data type",
			data:     []string{"invalid"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				formatGitSection(tt.data)
			})

			if len(tt.expected) == 0 && output != "" {
				t.Errorf("formatGitSection() expected no output, got:\n%s", output)
			}
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("formatGitSection() output missing expected line:\n  want: %s\n  got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestFormatInfoGenericSections(t *testing.T) {
	info := map[string]any{
		"build": map[string]any{
			"version": "1.0.0",
		},
		"java": map[string]any{
			"version": "21.0.1",
			"vendor": map[string]any{
				"name": "Eclipse Adoptium",
			},
		},
		"os": map[string]any{
			"name": "Linux",
			"arch": "amd64",
		},
		"kubernetes": map[string]any{
			"namespace":      "default",
			"podName":        "my-app-xk7pq",
			"podIP":          "10.0.0.23",
			"serviceAccount": "my-app",
		},
		"custom-scalar": "just a value",
		"tags":          []any{"a", "b"},
	}

	output := captureOutput(func() {
		formatInfo(info)
	})

	for _, expected := range []string{
		"Build:",
		"Java:",
		"  Version:  21.0.1",
		"  Vendor:",
		"    Name:  Eclipse Adoptium",
		"OS:",
		"  Arch:  amd64",
		"Kubernetes:",
		"  Namespace:        default",
		"  Pod Name:         my-app-xk7pq",
		"  Pod IP:           10.0.0.23",
		"  Service Account:  my-app",
		"Custom-scalar:",
		"  just a value",
		"Tags:",
		"  a, b",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("formatInfo() output missing expected line:\n  want: %q\n  got:\n%s", expected, output)
		}
	}

	// Curated sections come before generic ones
	if strings.Index(output, "Build:") > strings.Index(output, "Java:") {
		t.Errorf("curated Build section should precede generic sections:\n%s", output)
	}
}

func TestTitleCaseKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"podName", "Pod Name"},
		{"hostIP", "Host IP"},
		{"serviceAccount", "Service Account"},
		{"version", "Version"},
		{"nodeName", "Node Name"},
		{"IP", "IP"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := titleCaseKey(tt.input); got != tt.want {
				t.Errorf("titleCaseKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "hello", "Hello"},
		{"already capitalized", "World", "World"},
		{"single char", "a", "A"},
		{"empty string", "", ""},
		{"multiple words", "hello world", "Hello world"},
		{"number", "123", "123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := capitalizeFirst(tt.input)
			if got != tt.want {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatInfoSectionSeparation(t *testing.T) {
	info := map[string]any{
		"app": map[string]any{
			"name": "test-app",
		},
		"build": map[string]any{
			"version": "1.0.0",
		},
	}

	output := captureOutput(func() {
		formatInfo(info)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	if !slices.Contains(lines, "Application:") {
		t.Fatalf("Application: section missing from output:\n%s", output)
	}
	buildIndex := slices.Index(lines, "Build:")
	if buildIndex == -1 {
		t.Fatalf("Build: section missing from output:\n%s", output)
	}
	// Sections are separated by a blank line.
	if buildIndex == 0 || lines[buildIndex-1] != "" {
		t.Errorf("expected a blank line before the Build: section, got:\n%s", output)
	}
}

func TestFormatInfoNoTrailingNewline(t *testing.T) {
	info := map[string]any{
		"build": map[string]any{
			"version": "1.0.0",
		},
	}

	output := captureOutput(func() {
		formatInfo(info)
	})

	// Should not end with double newline
	if strings.HasSuffix(output, "\n\n") {
		t.Error("Output should not have trailing blank line")
	}
}
