package actuator

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestActuatorClientGetInfo(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse string
		mockStatus   int
		mockErr      error
		wantErr      bool
		validate     func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful response with build and kubernetes info",
			mockResponse: `{
				"build": {
					"artifact": "my-app",
					"name": "my-app",
					"time": "2025-10-21T22:34:55.709Z",
					"version": "1.0.0-SNAPSHOT",
					"group": "com.example"
				},
				"kubernetes": {
					"nodeName": "node-1",
					"podIp": "10.0.0.23",
					"hostIp": "10.0.0.10",
					"namespace": "default",
					"podName": "my-app-85664c5584-abc12",
					"serviceAccount": "my-app",
					"inside": true
				}
			}`,
			mockStatus: 200,
			wantErr:    false,
			validate: func(t *testing.T, info map[string]interface{}) {
				build, ok := info["build"].(map[string]interface{})
				if !ok {
					t.Fatal("expected build info")
				}
				if build["artifact"] != "my-app" {
					t.Errorf("expected artifact 'my-app', got %v", build["artifact"])
				}
				k8s, ok := info["kubernetes"].(map[string]interface{})
				if !ok {
					t.Fatal("expected kubernetes info")
				}
				if k8s["podName"] != "my-app-85664c5584-abc12" {
					t.Errorf("expected podName 'my-app-85664c5584-abc12', got %v", k8s["podName"])
				}
			},
		},
		{
			name: "successful response with build info only",
			mockResponse: `{
				"build": {
					"artifact": "standalone-app",
					"name": "standalone-app",
					"time": "2025-10-15T10:00:00.000Z",
					"version": "2.5.0",
					"group": "org.example"
				}
			}`,
			mockStatus: 200,
			wantErr:    false,
			validate: func(t *testing.T, info map[string]interface{}) {
				_, hasBuild := info["build"]
				if !hasBuild {
					t.Error("expected build info")
				}
				_, hasK8s := info["kubernetes"]
				if hasK8s {
					t.Error("did not expect kubernetes info")
				}
			},
		},
		{
			name: "successful response with custom fields",
			mockResponse: `{
				"build": {
					"version": "1.0.0"
				},
				"git": {
					"branch": "main",
					"commit": {
						"id": "abc123",
						"time": "2025-10-20T15:30:00Z"
					}
				},
				"app": {
					"name": "My Application",
					"description": "A sample app"
				}
			}`,
			mockStatus: 200,
			wantErr:    false,
			validate: func(t *testing.T, info map[string]interface{}) {
				if _, ok := info["git"]; !ok {
					t.Error("expected git info")
				}
				if _, ok := info["app"]; !ok {
					t.Error("expected app info")
				}
			},
		},
		{
			name:         "empty response",
			mockResponse: `{}`,
			mockStatus:   200,
			wantErr:      false,
			validate: func(t *testing.T, info map[string]interface{}) {
				if len(info) != 0 {
					t.Errorf("expected empty info, got %d keys", len(info))
				}
			},
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
			mockResponse: `{"build": invalid}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
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
			result, err := client.GetInfo()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestGetInfoJSONParsing(t *testing.T) {
	tests := []struct {
		name      string
		jsonInput string
		wantErr   bool
		wantKeys  []string
	}{
		{
			name: "nested structures",
			jsonInput: `{
				"build": {
					"version": "1.0.0",
					"time": "2025-10-22T10:00:00Z"
				},
				"git": {
					"commit": {
						"id": {
							"abbrev": "abc123",
							"full": "abc123def456"
						}
					}
				}
			}`,
			wantErr:  false,
			wantKeys: []string{"build", "git"},
		},
		{
			name:      "empty object",
			jsonInput: `{}`,
			wantErr:   false,
			wantKeys:  []string{},
		},
		{
			name: "arrays in response",
			jsonInput: `{
				"profiles": ["prod", "kubernetes"],
				"dependencies": [
					{"name": "spring-boot-starter-web", "version": "3.0.0"},
					{"name": "spring-boot-starter-actuator", "version": "3.0.0"}
				]
			}`,
			wantErr:  false,
			wantKeys: []string{"profiles", "dependencies"},
		},
		{
			name: "null values",
			jsonInput: `{
				"build": {
					"version": "1.0.0",
					"description": null
				}
			}`,
			wantErr:  false,
			wantKeys: []string{"build"},
		},
		{
			name: "boolean and numeric values",
			jsonInput: `{
				"kubernetes": {
					"inside": true,
					"port": 8080,
					"replicas": 3
				}
			}`,
			wantErr:  false,
			wantKeys: []string{"kubernetes"},
		},
		{
			name:      "invalid JSON - missing quotes",
			jsonInput: `{build: {version: 1.0.0}}`,
			wantErr:   true,
		},
		{
			name:      "invalid JSON - trailing comma",
			jsonInput: `{"build": {"version": "1.0.0",}}`,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var info map[string]interface{}
			err := json.Unmarshal([]byte(tt.jsonInput), &info)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				for _, key := range tt.wantKeys {
					if _, ok := info[key]; !ok {
						t.Errorf("expected key '%s' in response", key)
					}
				}
				if len(info) != len(tt.wantKeys) {
					t.Errorf("expected %d keys, got %d", len(tt.wantKeys), len(info))
				}
			}
		})
	}
}
