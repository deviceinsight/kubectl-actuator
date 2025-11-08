package actuator

import (
	"strconv"
	"testing"
)

func TestActuatorClientGetRaw(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		mockResponse string
		mockStatus   int
		mockErr      error
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
		},
		{
			name:         "500 internal server error",
			endpoint:     "info",
			mockResponse: ``,
			mockStatus:   500,
			wantErr:      true,
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
			var capturedPath string
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					capturedPath = path
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
			result, err := client.GetRaw(tt.endpoint)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRaw() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPath != "" && capturedPath != tt.wantPath {
				t.Errorf("GET path = %v, want %v", capturedPath, tt.wantPath)
			}

			if !tt.wantErr && tt.wantBody != "" {
				if string(result) != tt.wantBody {
					t.Errorf("body = %v, want %v", string(result), tt.wantBody)
				}
			}
		})
	}
}

func TestGetRawEndpointNormalization(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		expectedPath string
	}{
		{
			name:         "no leading slash",
			endpoint:     "info",
			expectedPath: "/info",
		},
		{
			name:         "with leading slash",
			endpoint:     "/info",
			expectedPath: "/info",
		},
		{
			name:         "empty string",
			endpoint:     "",
			expectedPath: "",
		},
		{
			name:         "nested path no leading slash",
			endpoint:     "health/liveness",
			expectedPath: "/health/liveness",
		},
		{
			name:         "nested path with leading slash",
			endpoint:     "/health/liveness",
			expectedPath: "/health/liveness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					capturedPath = path
					return &Response{
						Body:       []byte(`{}`),
						StatusCode: 200,
						Status:     "200",
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			_, _ = client.GetRaw(tt.endpoint)

			if capturedPath != tt.expectedPath {
				t.Errorf("path = %v, want %v", capturedPath, tt.expectedPath)
			}
		})
	}
}
