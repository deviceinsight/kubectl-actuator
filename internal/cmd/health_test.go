package cmd

import (
	"strings"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

func TestDisplayHealthTable(t *testing.T) {
	tests := []struct {
		name     string
		health   *actuator.HealthResponse
		expected []string
	}{
		{
			name: "simple flat components",
			health: &actuator.HealthResponse{
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"diskSpace": {
						Status: "UP",
						Details: map[string]interface{}{
							"total": 254431723520,
							"free":  4280823808,
						},
					},
					"ping": {
						Status: "UP",
					},
					"livenessState": {
						Status: "UP",
					},
				},
			},
			expected: []string{
				"COMPONENT",
				"STATUS",
				"diskSpace",
				"UP",
				"livenessState",
				"ping",
				"[overall]",
			},
		},
		{
			name: "nested components",
			health: &actuator.HealthResponse{
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"broker": {
						Status: "UP",
						Components: map[string]actuator.HealthComponent{
							"us1": {
								Status: "UP",
								Details: map[string]interface{}{
									"version": "1.0.2",
								},
							},
							"eu1": {
								Status: "DOWN",
								Details: map[string]interface{}{
									"error": "connection timeout",
								},
							},
						},
					},
					"ping": {
						Status: "UP",
					},
				},
			},
			expected: []string{
				"COMPONENT",
				"STATUS",
				"broker",
				"UP",
				"broker/eu1",
				"DOWN",
				"broker/us1",
				"UP",
				"ping",
				"[overall]",
			},
		},
		{
			name: "deeply nested components",
			health: &actuator.HealthResponse{
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"database": {
						Status: "UP",
						Components: map[string]actuator.HealthComponent{
							"primary": {
								Status: "UP",
							},
							"replica": {
								Status: "UP",
								Components: map[string]actuator.HealthComponent{
									"replica1": {
										Status: "UP",
									},
									"replica2": {
										Status: "DOWN",
									},
								},
							},
						},
					},
				},
			},
			expected: []string{
				"COMPONENT",
				"STATUS",
				"database",
				"UP",
				"database/primary",
				"database/replica",
				"database/replica/replica1",
				"database/replica/replica2",
				"DOWN",
				"[overall]",
			},
		},
		{
			name: "empty components",
			health: &actuator.HealthResponse{
				Status:     "UP",
				Components: map[string]actuator.HealthComponent{},
			},
			expected: []string{
				"COMPONENT",
				"STATUS",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				if err := displayHealthTable(tt.health); err != nil {
					t.Errorf("displayHealthTable() error = %v", err)
				}
			})

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("displayHealthTable() output missing expected value:\n  want: %s\n  got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestDisplayHealthWide(t *testing.T) {
	tests := []struct {
		name        string
		health      *actuator.HealthResponse
		expected    []string
		notExpected []string
	}{
		{
			name: "components with details",
			health: &actuator.HealthResponse{
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"diskSpace": {
						Status: "UP",
						Details: map[string]interface{}{
							"total": float64(254431723520),
							"free":  float64(4280823808),
						},
					},
					"ping": {
						Status: "UP",
					},
				},
			},
			expected: []string{
				"COMPONENT",
				"STATUS",
				"DETAILS",
				"diskSpace",
				"UP",
				"total",
				"free",
				"ping",
				"-", // ping has no details
				"[overall]",
			},
		},
		{
			name: "nested components with details",
			health: &actuator.HealthResponse{
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"broker": {
						Status: "UP",
						Components: map[string]actuator.HealthComponent{
							"us1": {
								Status: "UP",
								Details: map[string]interface{}{
									"version": "1.0.2",
								},
							},
							"eu1": {
								Status: "DOWN",
								Details: map[string]interface{}{
									"error": "connection timeout",
								},
							},
						},
					},
				},
			},
			expected: []string{
				"COMPONENT",
				"STATUS",
				"DETAILS",
				"broker",
				"broker/us1",
				"broker/eu1",
				"version",
				"1.0.2",
				"error",
				"connection timeout",
				"[overall]",
			},
		},
		{
			name: "components without details show dash",
			health: &actuator.HealthResponse{
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"ping": {
						Status: "UP",
					},
					"livenessState": {
						Status: "UP",
					},
				},
			},
			expected: []string{
				"ping",
				"UP",
				"-",
				"livenessState",
				"[overall]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				if err := displayHealthWide(tt.health); err != nil {
					t.Errorf("displayHealthWide() error = %v", err)
				}
			})

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("displayHealthWide() output missing expected value:\n  want: %s\n  got:\n%s", expected, output)
				}
			}

			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("displayHealthWide() output contains unexpected value:\n  don't want: %s\n  got:\n%s", notExpected, output)
				}
			}
		})
	}
}

func TestHealthComponentsSorting(t *testing.T) {
	// Test that components are sorted alphabetically in table output
	health := &actuator.HealthResponse{
		Status: "UP",
		Components: map[string]actuator.HealthComponent{
			"zeta":  {Status: "UP"},
			"alpha": {Status: "UP"},
			"beta":  {Status: "UP"},
			"gamma": {
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"gamma2": {Status: "UP"},
					"gamma1": {Status: "UP"},
				},
			},
		},
	}

	output := captureOutput(func() {
		if err := displayHealthTable(health); err != nil {
			t.Errorf("displayHealthTable() error = %v", err)
		}
	})

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Skip header line
	if len(lines) < 2 {
		t.Fatal("Expected at least 2 lines of output (header + data)")
	}

	componentLines := lines[1:]

	// Extract component names (first column), skip "[overall]" row
	var components []string
	for _, line := range componentLines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] != "[overall]" {
			components = append(components, fields[0])
		}
	}

	// Verify sorted order
	expected := []string{"alpha", "beta", "gamma", "gamma/gamma1", "gamma/gamma2", "zeta"}
	if len(components) != len(expected) {
		t.Errorf("Expected %d components, got %d", len(expected), len(components))
	}

	for i, comp := range components {
		if i < len(expected) && comp != expected[i] {
			t.Errorf("Component at position %d: got %q, want %q", i, comp, expected[i])
		}
	}
}

func TestHealthNestedPathFormatting(t *testing.T) {
	// Test that nested components use "/" separator
	health := &actuator.HealthResponse{
		Status: "UP",
		Components: map[string]actuator.HealthComponent{
			"parent": {
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"child": {
						Status: "UP",
						Components: map[string]actuator.HealthComponent{
							"grandchild": {
								Status: "DOWN",
							},
						},
					},
				},
			},
		},
	}

	output := captureOutput(func() {
		if err := displayHealthTable(health); err != nil {
			t.Errorf("displayHealthTable() error = %v", err)
		}
	})

	expectedPaths := []string{
		"parent",
		"parent/child",
		"parent/child/grandchild",
	}

	for _, path := range expectedPaths {
		if !strings.Contains(output, path) {
			t.Errorf("Output missing expected path %q:\n%s", path, output)
		}
	}
}
