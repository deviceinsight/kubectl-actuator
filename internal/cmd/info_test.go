package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFormatInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     map[string]interface{}
		expected []string // lines that should be present
	}{
		{
			name: "all sections present",
			info: map[string]interface{}{
				"app": map[string]interface{}{
					"name":        "test-app",
					"description": "Test Application",
				},
				"build": map[string]interface{}{
					"group":    "com.example",
					"artifact": "test-app",
					"version":  "1.0.0",
				},
				"git": map[string]interface{}{
					"branch": "main",
					"commit": map[string]interface{}{
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
			info: map[string]interface{}{
				"app": map[string]interface{}{
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
			info: map[string]interface{}{
				"build": map[string]interface{}{
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
			info:     map[string]interface{}{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				formatInfo(tt.info)
			})

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
		data     interface{}
		expected []string
	}{
		{
			name: "complete app info",
			data: map[string]interface{}{
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
			data: map[string]interface{}{
				"name":    "my-app",
				"version": "1.0",
				"author":  "John Doe",
			},
			expected: []string{
				"Application:",
				"  Name:         my-app",
				"  Version:  1.0",
				"  Author:  John Doe",
			},
		},
		{
			name: "only name",
			data: map[string]interface{}{
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
		data        interface{}
		expected    []string
		notExpected []string
	}{
		{
			name: "complete build info",
			data: map[string]interface{}{
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
			data: map[string]interface{}{
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
			data: map[string]interface{}{
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
		data     interface{}
		expected []string
	}{
		{
			name: "complete git info with commit time",
			data: map[string]interface{}{
				"branch": "main",
				"commit": map[string]interface{}{
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
			data: map[string]interface{}{
				"branch": "develop",
				"commit": map[string]interface{}{
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
			data: map[string]interface{}{
				"branch": "feature/new-feature",
			},
			expected: []string{
				"Git:",
				"  Branch:       feature/new-feature",
			},
		},
		{
			name: "commit without id",
			data: map[string]interface{}{
				"branch": "main",
				"commit": map[string]interface{}{
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

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("formatGitSection() output missing expected line:\n  want: %s\n  got:\n%s", expected, output)
				}
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
	info := map[string]interface{}{
		"app": map[string]interface{}{
			"name": "test-app",
		},
		"build": map[string]interface{}{
			"version": "1.0.0",
		},
	}

	output := captureOutput(func() {
		formatInfo(info)
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Check that sections are separated by blank lines
	appIndex := -1
	buildIndex := -1

	for i, line := range lines {
		if strings.Contains(line, "Application:") {
			appIndex = i
		}
		if strings.Contains(line, "Build:") {
			buildIndex = i
		}
	}

	if appIndex == -1 || buildIndex == -1 {
		t.Fatal("Missing expected sections in output")
	}

	// There should be a blank line between sections
	if buildIndex <= appIndex+2 {
		t.Errorf("Expected blank line between sections, got:\n%s", output)
	}

	// Output should not end with blank line
	if lines[len(lines)-1] == "" {
		t.Error("Output should not end with a blank line")
	}
}

func TestFormatInfoNoTrailingNewline(t *testing.T) {
	info := map[string]interface{}{
		"build": map[string]interface{}{
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

// captureOutput captures stdout during the execution of a function
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}
