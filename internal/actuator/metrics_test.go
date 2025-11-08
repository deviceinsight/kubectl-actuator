package actuator

import (
	"strconv"
	"testing"
)

func TestActuatorClientGetMetrics(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse string
		mockStatus   int
		mockErr      error
		wantErr      bool
		wantNames    []string
	}{
		{
			name: "successful response with metrics",
			mockResponse: `{
				"names": [
					"jvm.memory.used",
					"jvm.memory.max",
					"process.cpu.usage",
					"http.server.requests"
				]
			}`,
			mockStatus: 200,
			wantErr:    false,
			wantNames:  []string{"jvm.memory.used", "jvm.memory.max", "process.cpu.usage", "http.server.requests"},
		},
		{
			name:         "empty metrics list",
			mockResponse: `{"names": []}`,
			mockStatus:   200,
			wantErr:      false,
			wantNames:    []string{},
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
			mockResponse: `{"names": [invalid]}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					if path != "/metrics" {
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
			result, err := client.GetMetrics()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Names) != len(tt.wantNames) {
					t.Errorf("got %d metrics, want %d", len(result.Names), len(tt.wantNames))
				}
				for i, name := range tt.wantNames {
					if result.Names[i] != name {
						t.Errorf("metric[%d] = %s, want %s", i, result.Names[i], name)
					}
				}
			}
		})
	}
}

func TestActuatorClientGetMetric(t *testing.T) {
	tests := []struct {
		name            string
		metricName      string
		mockResponse    string
		mockStatus      int
		mockErr         error
		wantErr         bool
		wantPath        string
		wantDescription string
		wantBaseUnit    string
	}{
		{
			name:       "successful metric detail response",
			metricName: "jvm.memory.used",
			mockResponse: `{
				"name": "jvm.memory.used",
				"description": "The amount of used memory",
				"baseUnit": "bytes",
				"measurements": [
					{"statistic": "VALUE", "value": 123456789}
				],
				"availableTags": [
					{"tag": "area", "values": ["heap", "nonheap"]},
					{"tag": "id", "values": ["G1 Eden Space", "G1 Old Gen"]}
				]
			}`,
			mockStatus:      200,
			wantErr:         false,
			wantPath:        "/metrics/jvm.memory.used",
			wantDescription: "The amount of used memory",
			wantBaseUnit:    "bytes",
		},
		{
			name:       "metric with multiple measurements",
			metricName: "http.server.requests",
			mockResponse: `{
				"name": "http.server.requests",
				"description": "HTTP server request statistics",
				"baseUnit": "seconds",
				"measurements": [
					{"statistic": "COUNT", "value": 100},
					{"statistic": "TOTAL_TIME", "value": 5.5},
					{"statistic": "MAX", "value": 0.25}
				],
				"availableTags": []
			}`,
			mockStatus:      200,
			wantErr:         false,
			wantPath:        "/metrics/http.server.requests",
			wantDescription: "HTTP server request statistics",
			wantBaseUnit:    "seconds",
		},
		{
			name:       "metric with special characters in name",
			metricName: "cache.gets{cache=myCache}",
			mockResponse: `{
				"name": "cache.gets",
				"description": "Cache gets",
				"baseUnit": null,
				"measurements": [{"statistic": "COUNT", "value": 50}],
				"availableTags": []
			}`,
			mockStatus: 200,
			wantErr:    false,
			wantPath:   "/metrics/cache.gets%7Bcache=myCache%7D",
		},
		{
			name:         "metric not found",
			metricName:   "nonexistent.metric",
			mockResponse: ``,
			mockStatus:   404,
			wantErr:      true,
		},
		{
			name:         "500 internal server error",
			metricName:   "jvm.memory.used",
			mockResponse: ``,
			mockStatus:   500,
			wantErr:      true,
		},
		{
			name:         "malformed JSON",
			metricName:   "jvm.memory.used",
			mockResponse: `{"name": invalid}`,
			mockStatus:   200,
			wantErr:      true,
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
			result, err := client.GetMetric(tt.metricName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetric() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPath != "" && capturedPath != tt.wantPath {
				t.Errorf("GET path = %v, want %v", capturedPath, tt.wantPath)
			}

			if !tt.wantErr && result != nil {
				if tt.wantDescription != "" && result.Description != tt.wantDescription {
					t.Errorf("description = %v, want %v", result.Description, tt.wantDescription)
				}
				if tt.wantBaseUnit != "" && result.BaseUnit != tt.wantBaseUnit {
					t.Errorf("baseUnit = %v, want %v", result.BaseUnit, tt.wantBaseUnit)
				}
			}
		})
	}
}

func TestMetricResponseMeasurements(t *testing.T) {
	tests := []struct {
		name             string
		mockResponse     string
		wantMeasurements int
		wantTags         int
	}{
		{
			name: "single measurement",
			mockResponse: `{
				"name": "test.metric",
				"measurements": [{"statistic": "VALUE", "value": 42.5}],
				"availableTags": []
			}`,
			wantMeasurements: 1,
			wantTags:         0,
		},
		{
			name: "multiple measurements and tags",
			mockResponse: `{
				"name": "test.metric",
				"measurements": [
					{"statistic": "COUNT", "value": 100},
					{"statistic": "TOTAL_TIME", "value": 10.5},
					{"statistic": "MAX", "value": 0.5}
				],
				"availableTags": [
					{"tag": "method", "values": ["GET", "POST"]},
					{"tag": "status", "values": ["200", "404", "500"]}
				]
			}`,
			wantMeasurements: 3,
			wantTags:         2,
		},
		{
			name: "empty measurements and tags",
			mockResponse: `{
				"name": "test.metric",
				"measurements": [],
				"availableTags": []
			}`,
			wantMeasurements: 0,
			wantTags:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					return &Response{
						Body:       []byte(tt.mockResponse),
						StatusCode: 200,
						Status:     "200",
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetMetric("test.metric")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Measurements) != tt.wantMeasurements {
				t.Errorf("got %d measurements, want %d", len(result.Measurements), tt.wantMeasurements)
			}

			if len(result.AvailableTags) != tt.wantTags {
				t.Errorf("got %d tags, want %d", len(result.AvailableTags), tt.wantTags)
			}
		})
	}
}
