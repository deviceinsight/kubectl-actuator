package actuator

import (
	"strings"
	"testing"
)

func TestActuatorClientGetMetrics(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse string
		mockStatus   int
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
			mockClient := newEndpointMock(t, "/metrics", respondWith(tt.mockStatus, tt.mockResponse))

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
		name             string
		metricName       string
		mockResponse     string
		mockStatus       int
		wantErr          bool
		wantPath         string
		wantDescription  string
		wantBaseUnit     string
		wantMeasurements int
		wantTags         int
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
			mockStatus:       200,
			wantErr:          false,
			wantPath:         "/metrics/jvm.memory.used",
			wantDescription:  "The amount of used memory",
			wantBaseUnit:     "bytes",
			wantMeasurements: 1,
			wantTags:         2,
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
			mockStatus:       200,
			wantErr:          false,
			wantPath:         "/metrics/http.server.requests",
			wantDescription:  "HTTP server request statistics",
			wantBaseUnit:     "seconds",
			wantMeasurements: 3,
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
			mockStatus:       200,
			wantErr:          false,
			wantPath:         "/metrics/cache.gets%7Bcache=myCache%7D",
			wantMeasurements: 1,
		},
		{
			name:       "empty measurements and tags",
			metricName: "test.metric",
			mockResponse: `{
				"name": "test.metric",
				"measurements": [],
				"availableTags": []
			}`,
			mockStatus: 200,
			wantErr:    false,
			wantPath:   "/metrics/test.metric",
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
			var requestedPath string
			respond := respondWith(tt.mockStatus, tt.mockResponse)
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					requestedPath = path
					return respond()
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetMetric(tt.metricName, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetMetric() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPath != "" && requestedPath != tt.wantPath {
				t.Errorf("GET path = %v, want %v", requestedPath, tt.wantPath)
			}

			if !tt.wantErr && result != nil {
				if tt.wantDescription != "" && result.Description != tt.wantDescription {
					t.Errorf("description = %v, want %v", result.Description, tt.wantDescription)
				}
				if tt.wantBaseUnit != "" && result.BaseUnit != tt.wantBaseUnit {
					t.Errorf("baseUnit = %v, want %v", result.BaseUnit, tt.wantBaseUnit)
				}
				if len(result.Measurements) != tt.wantMeasurements {
					t.Errorf("got %d measurements, want %d", len(result.Measurements), tt.wantMeasurements)
				}
				if len(result.AvailableTags) != tt.wantTags {
					t.Errorf("got %d tags, want %d", len(result.AvailableTags), tt.wantTags)
				}
			}
		})
	}
}

func TestActuatorClientGetMetricWithTags(t *testing.T) {
	var requestedPath string
	respond := respondWith(200, `{"name": "jvm.memory.used", "measurements": [], "availableTags": []}`)
	mockClient := &MockHTTPClient{
		GetFunc: func(path string) (*Response, error) {
			requestedPath = path
			return respond()
		},
	}

	client := &actuatorClient{httpClient: mockClient}
	_, err := client.GetMetric("jvm.memory.used", []string{"area:heap", "id:G1 Old Gen"})
	if err != nil {
		t.Fatalf("GetMetric() error = %v", err)
	}

	want := "/metrics/jvm.memory.used?tag=area%3Aheap&tag=id%3AG1+Old+Gen"
	if requestedPath != want {
		t.Errorf("requested path = %q, want %q", requestedPath, want)
	}
}

func TestActuatorClientGetMetricWithTagsBadRequest(t *testing.T) {
	mockClient := &MockHTTPClient{
		GetFunc: func(path string) (*Response, error) {
			return &Response{StatusCode: 400, Status: "400 Bad Request"}, nil
		},
	}

	client := &actuatorClient{httpClient: mockClient}
	_, err := client.GetMetric("jvm.memory.used", []string{"bogus:tag"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
	if !strings.Contains(err.Error(), "bogus:tag") {
		t.Errorf("error should mention the tags, got: %v", err)
	}
}
