package cmd

import (
	"slices"
	"testing"
)

func TestBuildEndpointRows(t *testing.T) {
	rows := buildEndpointRows([]string{"health", "custom-endpoint", "loggers", "configprops"})

	byID := make(map[string]endpointRow, len(rows))
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		byID[row.id] = row
		ids = append(ids, row.id)
	}

	if !slices.IsSorted(ids) {
		t.Errorf("rows are not sorted: %v", ids)
	}

	for id, wantAvailable := range map[string]bool{
		"health":          true,
		"loggers":         true,
		"custom-endpoint": true, // exposed endpoints are listed even without a dedicated command
		"configprops":     true,
		"beans":           false, // supported endpoints are listed even when not exposed
		"logfile":         false,
	} {
		row, ok := byID[id]
		if !ok {
			t.Errorf("row for %q missing", id)
			continue
		}
		if row.available != wantAvailable {
			t.Errorf("%q available = %t, want %t", id, row.available, wantAvailable)
		}
	}

	// Unsupported endpoints appear only when the application exposes them.
	if _, ok := byID["mappings"]; ok {
		t.Error("mappings is neither supported nor exposed and must not be listed")
	}
}

func TestSupportForEndpoint(t *testing.T) {
	tests := []struct {
		endpoint string
		want     string
	}{
		{"health", "health"},
		{"loggers", "logger"},
		{"scheduledtasks", "scheduledtasks"},
		{"heapdump", "heapdump"},
		{"configprops", "-"},
		{"quartz", "-"},
	}

	for _, tt := range tests {
		if got := supportForEndpoint(tt.endpoint); got != tt.want {
			t.Errorf("supportForEndpoint(%q) = %q, want %q", tt.endpoint, got, tt.want)
		}
	}
}
