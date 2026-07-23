package cmd

import (
	"slices"
	"strings"
	"testing"
)

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{input: "area=heap", want: "area:heap"},
		{input: "area:heap", want: "area:heap"},
		{input: "id=G1 Old Gen", want: "id:G1 Old Gen"},
		{input: "uri=/api/v1?x=1", want: "uri:/api/v1?x=1"},
		{input: "noseparator", wantErr: true},
		{input: "=value", wantErr: true},
		{input: "key=", wantErr: true},
		{input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeTag(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeTag(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("normalizeTag(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeTags(t *testing.T) {
	got, err := normalizeTags([]string{"area=heap", "id=G1 Old Gen"})
	if err != nil {
		t.Fatalf("normalizeTags() error = %v", err)
	}
	want := []string{"area:heap", "id:G1 Old Gen"}
	if !slices.Equal(got, want) {
		t.Errorf("normalizeTags() = %v, want %v", got, want)
	}

	if _, err := normalizeTags([]string{"area=heap", "malformed"}); err == nil {
		t.Error("expected an error for a malformed tag in the list")
	}
}

func TestMetricsValidateFlags(t *testing.T) {
	tests := []struct {
		name        string
		metricName  string
		filter      string
		tags        []string
		output      string
		wantErr     bool
		errContains string
	}{
		{
			name:       "no tags",
			metricName: "jvm.memory.used",
		},
		{
			name:       "tags with metric name",
			metricName: "jvm.memory.used",
			tags:       []string{"area=heap", "id=G1 Old Gen"},
		},
		{
			name:        "tags without metric name",
			tags:        []string{"area=heap"},
			wantErr:     true,
			errContains: "--tag requires a metric name",
		},
		{
			name:        "invalid tag",
			metricName:  "jvm.memory.used",
			tags:        []string{"badtag"},
			wantErr:     true,
			errContains: "invalid tag",
		},
		{
			name:        "invalid output format",
			output:      "wide",
			wantErr:     true,
			errContains: "invalid output format",
		},
		{
			name:        "filter with metric name",
			metricName:  "jvm.memory.used",
			filter:      "jvm",
			wantErr:     true,
			errContains: "--filter cannot be combined with a metric name argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &metricsCommandOperations{
				metricName: tt.metricName,
				filter:     tt.filter,
				tags:       tt.tags,
				output:     tt.output,
			}

			err := ops.validateFlags()

			if (err != nil) != tt.wantErr {
				t.Fatalf("validateFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			}
		})
	}
}
