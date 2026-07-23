package cmd

import (
	"slices"
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
						Details: map[string]any{
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
								Details: map[string]any{
									"version": "1.0.2",
								},
							},
							"eu1": {
								Status: "DOWN",
								Details: map[string]any{
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
				if err := displayHealth(tt.health, false); err != nil {
					t.Errorf("displayHealth() error = %v", err)
				}
			})

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("displayHealth() output missing expected value:\n  want: %s\n  got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestDisplayHealthWide(t *testing.T) {
	tests := []struct {
		name     string
		health   *actuator.HealthResponse
		expected []string
	}{
		{
			name: "components with details",
			health: &actuator.HealthResponse{
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"diskSpace": {
						Status: "UP",
						Details: map[string]any{
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
								Details: map[string]any{
									"version": "1.0.2",
								},
							},
							"eu1": {
								Status: "DOWN",
								Details: map[string]any{
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
				if err := displayHealth(tt.health, true); err != nil {
					t.Errorf("displayHealth() error = %v", err)
				}
			})

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("displayHealth() output missing expected value:\n  want: %s\n  got:\n%s", expected, output)
				}
			}
		})
	}
}

// TestDisplayHealthRowAssociation pins statuses to their components: the
// substring checks above would still pass if DOWN moved to a different row.
func TestDisplayHealthRowAssociation(t *testing.T) {
	health := &actuator.HealthResponse{
		Status: "UP",
		Components: map[string]actuator.HealthComponent{
			"broker": {
				Status: "UP",
				Components: map[string]actuator.HealthComponent{
					"us1": {Status: "UP"},
					"eu1": {Status: "DOWN"},
				},
			},
		},
	}

	output := captureOutput(func() {
		if err := displayHealth(health, false); err != nil {
			t.Errorf("displayHealth() error = %v", err)
		}
	})

	for _, wantRow := range []string{"broker/eu1 DOWN", "broker/us1 UP"} {
		found := false
		for _, line := range strings.Split(output, "\n") {
			if strings.Join(strings.Fields(line), " ") == wantRow {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no output line reads %q:\n%s", wantRow, output)
		}
	}
}

// TestHealthComponentsSorting pins the row order: entries are sorted by
// path, so nested components follow their parent and output is stable
// across runs regardless of map iteration order.
func TestHealthComponentsSorting(t *testing.T) {
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
		if err := displayHealth(health, false); err != nil {
			t.Errorf("displayHealth() error = %v", err)
		}
	})

	var components []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "[overall]" {
			continue
		}
		components = append(components, fields[0])
	}

	want := []string{"alpha", "beta", "gamma", "gamma/gamma1", "gamma/gamma2", "zeta"}
	if !slices.Equal(components, want) {
		t.Errorf("component order = %v, want %v", components, want)
	}
}
