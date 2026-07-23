package actuator

import "testing"

func TestActuatorClientGetRaw(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		mockResponse string
		mockStatus   int
		wantErr      bool
		wantPath     string
		wantBody     string
	}{
		{
			name:         "successful raw endpoint call",
			endpoint:     "info",
			mockResponse: `{"app":{"name":"test-app"}}`,
			mockStatus:   200,
			wantErr:      false,
			wantPath:     "/info",
			wantBody:     `{"app":{"name":"test-app"}}`,
		},
		{
			name:         "endpoint with leading slash",
			endpoint:     "/health",
			mockResponse: `{"status":"UP"}`,
			mockStatus:   200,
			wantErr:      false,
			wantPath:     "/health",
			wantBody:     `{"status":"UP"}`,
		},
		{
			name:         "nested endpoint path",
			endpoint:     "health/liveness",
			mockResponse: `{"status":"UP"}`,
			mockStatus:   200,
			wantErr:      false,
			wantPath:     "/health/liveness",
			wantBody:     `{"status":"UP"}`,
		},
		{
			name:         "nested endpoint path with leading slash",
			endpoint:     "/health/liveness",
			mockResponse: `{"status":"UP"}`,
			mockStatus:   200,
			wantErr:      false,
			wantPath:     "/health/liveness",
			wantBody:     `{"status":"UP"}`,
		},
		{
			name:         "empty endpoint for root",
			endpoint:     "",
			mockResponse: `{"_links":{"self":{"href":"http://localhost:8080/actuator"}}}`,
			mockStatus:   200,
			wantErr:      false,
			wantPath:     "",
			wantBody:     `{"_links":{"self":{"href":"http://localhost:8080/actuator"}}}`,
		},
		{
			name:         "endpoint not found",
			endpoint:     "nonexistent",
			mockResponse: ``,
			mockStatus:   404,
			wantErr:      true,
			wantPath:     "/nonexistent",
		},
		{
			name:         "500 internal server error",
			endpoint:     "info",
			mockResponse: ``,
			mockStatus:   500,
			wantErr:      true,
			wantPath:     "/info",
		},
		{
			name:         "non-JSON response",
			endpoint:     "prometheus",
			mockResponse: "# HELP jvm_memory_used_bytes\njvm_memory_used_bytes{area=\"heap\"} 123456789",
			mockStatus:   200,
			wantErr:      false,
			wantPath:     "/prometheus",
			wantBody:     "# HELP jvm_memory_used_bytes\njvm_memory_used_bytes{area=\"heap\"} 123456789",
		},
		{
			name:         "endpoint with query parameters in path",
			endpoint:     "metrics/jvm.memory.used?tag=area:heap",
			mockResponse: `{"name":"jvm.memory.used"}`,
			mockStatus:   200,
			wantErr:      false,
			wantPath:     "/metrics/jvm.memory.used?tag=area:heap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedPaths []string
			respond := respondWith(tt.mockStatus, tt.mockResponse)
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					requestedPaths = append(requestedPaths, path)
					return respond()
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetRaw(tt.endpoint)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRaw() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// The first request must hit the endpoint path itself; any later
			// requests are the 404 diagnosis probing the actuator index.
			if len(requestedPaths) == 0 || requestedPaths[0] != tt.wantPath {
				t.Errorf("GET paths = %q, want %q first", requestedPaths, tt.wantPath)
			}

			if !tt.wantErr && tt.wantBody != "" {
				if string(result) != tt.wantBody {
					t.Errorf("body = %v, want %v", string(result), tt.wantBody)
				}
			}
		})
	}
}
