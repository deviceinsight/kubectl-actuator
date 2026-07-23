package actuator

import "testing"

func TestActuatorClientGetInfo(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse string
		mockStatus   int
		wantErr      bool
		validate     func(*testing.T, map[string]any)
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
			validate: func(t *testing.T, info map[string]any) {
				build, ok := info["build"].(map[string]any)
				if !ok {
					t.Fatal("expected build info")
				}
				if build["artifact"] != "my-app" {
					t.Errorf("expected artifact 'my-app', got %v", build["artifact"])
				}
				k8s, ok := info["kubernetes"].(map[string]any)
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
			validate: func(t *testing.T, info map[string]any) {
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
			validate: func(t *testing.T, info map[string]any) {
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
			validate: func(t *testing.T, info map[string]any) {
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
			mockClient := newEndpointMock(t, "/info", respondWith(tt.mockStatus, tt.mockResponse))

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
