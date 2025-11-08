package actuator

import (
	"strconv"
	"testing"
)

func TestActuatorClientGetHealth(t *testing.T) {
	tests := []struct {
		name             string
		mockResponse     string
		mockStatus       int
		mockErr          error
		wantErr          bool
		wantStatus       string
		wantComponentCnt int
	}{
		{
			name: "successful response with UP status",
			mockResponse: `{
				"status": "UP",
				"components": {
					"db": {
						"status": "UP",
						"details": {
							"database": "PostgreSQL",
							"validationQuery": "isValid()"
						}
					},
					"diskSpace": {
						"status": "UP",
						"details": {
							"total": 107374182400,
							"free": 53687091200,
							"threshold": 10485760
						}
					}
				}
			}`,
			mockStatus:       200,
			wantErr:          false,
			wantStatus:       "UP",
			wantComponentCnt: 2,
		},
		{
			name: "DOWN status with failed component",
			mockResponse: `{
				"status": "DOWN",
				"components": {
					"db": {
						"status": "DOWN",
						"details": {
							"error": "Connection refused"
						}
					},
					"diskSpace": {
						"status": "UP"
					}
				}
			}`,
			mockStatus:       200,
			wantErr:          false,
			wantStatus:       "DOWN",
			wantComponentCnt: 2,
		},
		{
			name:         "503 service unavailable",
			mockResponse: ``,
			mockStatus:   503,
			wantErr:      true,
		},
		{
			name:             "simple UP status without components",
			mockResponse:     `{"status": "UP"}`,
			mockStatus:       200,
			wantErr:          false,
			wantStatus:       "UP",
			wantComponentCnt: 0,
		},
		{
			name: "status with groups",
			mockResponse: `{
				"status": "UP",
				"groups": ["liveness", "readiness"]
			}`,
			mockStatus:       200,
			wantErr:          false,
			wantStatus:       "UP",
			wantComponentCnt: 0,
		},
		{
			name:         "404 endpoint not found",
			mockResponse: ``,
			mockStatus:   404,
			wantErr:      true,
		},
		{
			name:         "500 internal server error",
			mockResponse: ``,
			mockStatus:   500,
			wantErr:      true,
		},
		{
			name:         "malformed JSON",
			mockResponse: `{"status": invalid}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					if path != "/health" {
						t.Errorf("unexpected path: %s", path)
					}
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return &Response{
						Body:       []byte(tt.mockResponse),
						StatusCode: tt.mockStatus,
						Status:     strconv.Itoa(tt.mockStatus),
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetHealth()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetHealth() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result.Status != tt.wantStatus {
					t.Errorf("status = %v, want %v", result.Status, tt.wantStatus)
				}
				if len(result.Components) != tt.wantComponentCnt {
					t.Errorf("got %d components, want %d", len(result.Components), tt.wantComponentCnt)
				}
			}
		})
	}
}

func TestHealthResponseParsing(t *testing.T) {
	tests := []struct {
		name     string
		response string
		validate func(*testing.T, *HealthResponse)
	}{
		{
			name: "component with details",
			response: `{
				"status": "UP",
				"components": {
					"db": {
						"status": "UP",
						"details": {
							"database": "PostgreSQL",
							"version": "14.5"
						}
					}
				}
			}`,
			validate: func(t *testing.T, resp *HealthResponse) {
				db, ok := resp.Components["db"]
				if !ok {
					t.Fatal("expected db component")
				}
				if db.Status != "UP" {
					t.Errorf("expected db status 'UP', got '%s'", db.Status)
				}
				if db.Details["database"] != "PostgreSQL" {
					t.Errorf("expected database 'PostgreSQL', got '%v'", db.Details["database"])
				}
			},
		},
		{
			name: "nested components",
			response: `{
				"status": "UP",
				"components": {
					"db": {
						"status": "UP",
						"components": {
							"primary": {
								"status": "UP",
								"details": {"connection": "active"}
							},
							"replica": {
								"status": "UP",
								"details": {"connection": "active"}
							}
						}
					}
				}
			}`,
			validate: func(t *testing.T, resp *HealthResponse) {
				db := resp.Components["db"]
				if len(db.Components) != 2 {
					t.Errorf("expected 2 nested components, got %d", len(db.Components))
				}
				primary, ok := db.Components["primary"]
				if !ok {
					t.Fatal("expected primary component")
				}
				if primary.Status != "UP" {
					t.Errorf("expected primary status 'UP', got '%s'", primary.Status)
				}
			},
		},
		{
			name: "various health statuses",
			response: `{
				"status": "UP",
				"components": {
					"healthy": {"status": "UP"},
					"unhealthy": {"status": "DOWN"},
					"unknown": {"status": "UNKNOWN"},
					"outOfService": {"status": "OUT_OF_SERVICE"}
				}
			}`,
			validate: func(t *testing.T, resp *HealthResponse) {
				if resp.Components["healthy"].Status != "UP" {
					t.Error("expected healthy status UP")
				}
				if resp.Components["unhealthy"].Status != "DOWN" {
					t.Error("expected unhealthy status DOWN")
				}
				if resp.Components["unknown"].Status != "UNKNOWN" {
					t.Error("expected unknown status UNKNOWN")
				}
				if resp.Components["outOfService"].Status != "OUT_OF_SERVICE" {
					t.Error("expected outOfService status OUT_OF_SERVICE")
				}
			},
		},
		{
			name: "component with numeric details",
			response: `{
				"status": "UP",
				"components": {
					"diskSpace": {
						"status": "UP",
						"details": {
							"total": 107374182400,
							"free": 53687091200,
							"threshold": 10485760,
							"exists": true
						}
					}
				}
			}`,
			validate: func(t *testing.T, resp *HealthResponse) {
				disk := resp.Components["diskSpace"]
				if disk.Details["total"] != float64(107374182400) {
					t.Errorf("unexpected total: %v", disk.Details["total"])
				}
				if disk.Details["exists"] != true {
					t.Errorf("unexpected exists: %v", disk.Details["exists"])
				}
			},
		},
		{
			name: "health groups",
			response: `{
				"status": "UP",
				"groups": ["liveness", "readiness", "custom"]
			}`,
			validate: func(t *testing.T, resp *HealthResponse) {
				if len(resp.Groups) != 3 {
					t.Errorf("expected 3 groups, got %d", len(resp.Groups))
				}
				expectedGroups := []string{"liveness", "readiness", "custom"}
				for i, group := range expectedGroups {
					if resp.Groups[i] != group {
						t.Errorf("group[%d] = %s, want %s", i, resp.Groups[i], group)
					}
				}
			},
		},
		{
			name: "empty components",
			response: `{
				"status": "UP",
				"components": {}
			}`,
			validate: func(t *testing.T, resp *HealthResponse) {
				if len(resp.Components) != 0 {
					t.Errorf("expected 0 components, got %d", len(resp.Components))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					return &Response{
						Body:       []byte(tt.response),
						StatusCode: 200,
						Status:     "200",
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetHealth()

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.validate(t, result)
		})
	}
}
