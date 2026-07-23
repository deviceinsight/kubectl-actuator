package actuator

import (
	"strings"
	"testing"
)

func TestActuatorClientGetHealth(t *testing.T) {
	tests := []struct {
		name             string
		mockResponse     string
		mockStatus       int
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
			name:         "503 without a health body is an error",
			mockResponse: ``,
			mockStatus:   503,
			wantErr:      true,
		},
		{
			// Spring returns 503 with the full health document when the
			// aggregate status is DOWN; that is a successful check of an
			// unhealthy app, not a failure.
			name: "503 with DOWN health body renders the document",
			mockResponse: `{
				"status": "DOWN",
				"components": {
					"db": {
						"status": "DOWN",
						"details": {"error": "Connection refused"}
					}
				}
			}`,
			mockStatus:       503,
			wantErr:          false,
			wantStatus:       "DOWN",
			wantComponentCnt: 1,
		},
		{
			name:         "503 with non-health JSON body is an error",
			mockResponse: `{"error": "Service Unavailable"}`,
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
			mockClient := newEndpointMock(t, "/health", respondWith(tt.mockStatus, tt.mockResponse))

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetHealth("")

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

func TestActuatorClientGetHealthGroup(t *testing.T) {
	mockClient := newEndpointMock(t, "/health/liveness", respondWith(200, `{"status": "UP"}`))

	client := &actuatorClient{httpClient: mockClient}
	result, err := client.GetHealth("liveness")
	if err != nil {
		t.Fatalf("GetHealth(liveness) error = %v", err)
	}
	if result.Status != "UP" {
		t.Errorf("status = %v, want UP", result.Status)
	}
}

func TestActuatorClientGetHealthUnknownGroup(t *testing.T) {
	// The health endpoint is exposed (the index probe finds it), so a 404 on
	// a group path must be reported as an unknown group, not a missing
	// endpoint.
	mockClient := &MockHTTPClient{
		GetFunc: func(path string) (*Response, error) {
			switch path {
			case "":
				return &Response{
					Body:       []byte(`{"_links": {"health": {"href": "http://x/actuator/health"}}}`),
					StatusCode: 200,
					Status:     "200",
				}, nil
			case "/health/nope":
				return &Response{StatusCode: 404, Status: "404 Not Found"}, nil
			}
			t.Errorf("unexpected path: %s", path)
			return &Response{StatusCode: 500, Status: "500"}, nil
		},
	}

	client := &actuatorClient{httpClient: mockClient}
	_, err := client.GetHealth("nope")
	if err == nil {
		t.Fatal("expected error for unknown group")
	}
	want := `no health group or component named "nope"`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v, want containing %q", err, want)
	}
}
