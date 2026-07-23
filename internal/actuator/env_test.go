package actuator

import "testing"

func TestActuatorClientGetEnv(t *testing.T) {
	tests := []struct {
		name               string
		mockResponse       string
		mockStatus         int
		wantErr            bool
		wantProfilesCnt    int
		wantPropSourcesCnt int
	}{
		{
			name: "successful response with profiles and property sources",
			mockResponse: `{
				"activeProfiles": ["prod", "kubernetes"],
				"propertySources": [
					{
						"name": "systemProperties",
						"properties": {
							"java.version": {"value": "17.0.1"},
							"user.timezone": {"value": "UTC"}
						}
					},
					{
						"name": "applicationConfig: [classpath:/application.yml]",
						"properties": {
							"server.port": {"value": "8080", "origin": "class path resource [application.yml]"}
						}
					}
				]
			}`,
			mockStatus:         200,
			wantErr:            false,
			wantProfilesCnt:    2,
			wantPropSourcesCnt: 2,
		},
		{
			name: "empty profiles",
			mockResponse: `{
				"activeProfiles": [],
				"propertySources": [
					{
						"name": "systemEnvironment",
						"properties": {
							"PATH": {"value": "/usr/bin"}
						}
					}
				]
			}`,
			mockStatus:         200,
			wantErr:            false,
			wantProfilesCnt:    0,
			wantPropSourcesCnt: 1,
		},
		{
			name:               "empty response",
			mockResponse:       `{"activeProfiles": [], "propertySources": []}`,
			mockStatus:         200,
			wantErr:            false,
			wantProfilesCnt:    0,
			wantPropSourcesCnt: 0,
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
			mockResponse: `{"activeProfiles": invalid}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newEndpointMock(t, "/env", respondWith(tt.mockStatus, tt.mockResponse))

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetEnv()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.ActiveProfiles) != tt.wantProfilesCnt {
					t.Errorf("got %d profiles, want %d", len(result.ActiveProfiles), tt.wantProfilesCnt)
				}
				if len(result.PropertySources) != tt.wantPropSourcesCnt {
					t.Errorf("got %d property sources, want %d", len(result.PropertySources), tt.wantPropSourcesCnt)
				}
			}
		})
	}
}

func TestActuatorClientGetEnvProperty(t *testing.T) {
	tests := []struct {
		name         string
		propertyName string
		mockResponse string
		mockStatus   int
		wantErr      bool
		wantPath     string
		wantValue    any
	}{
		{
			name:         "successful property lookup",
			propertyName: "server.port",
			mockResponse: `{
				"property": {
					"source": "applicationConfig: [classpath:/application.yml]",
					"value": "8080"
				},
				"activeProfiles": ["prod"],
				"defaultProfiles": ["default"],
				"propertySources": [
					{"name": "applicationConfig: [classpath:/application.yml]", "property": {"value": "8080"}}
				]
			}`,
			mockStatus: 200,
			wantErr:    false,
			wantPath:   "/env/server.port",
			wantValue:  "8080",
		},
		{
			name:         "property with special characters",
			propertyName: "spring.datasource.url",
			mockResponse: `{
				"property": {
					"source": "systemEnvironment",
					"value": "jdbc:postgresql://localhost:5432/db"
				},
				"activeProfiles": [],
				"defaultProfiles": ["default"],
				"propertySources": []
			}`,
			mockStatus: 200,
			wantErr:    false,
			wantPath:   "/env/spring.datasource.url",
			wantValue:  "jdbc:postgresql://localhost:5432/db",
		},
		{
			name:         "property with URL encoding needed",
			propertyName: "my.property[0]",
			mockResponse: `{
				"property": {
					"source": "test",
					"value": "value"
				},
				"activeProfiles": [],
				"defaultProfiles": [],
				"propertySources": []
			}`,
			mockStatus: 200,
			wantErr:    false,
			wantPath:   "/env/my.property%5B0%5D",
		},
		{
			name:         "property not found",
			propertyName: "nonexistent.property",
			mockResponse: ``,
			mockStatus:   404,
			wantErr:      true,
		},
		{
			name:         "500 internal server error",
			propertyName: "server.port",
			mockResponse: ``,
			mockStatus:   500,
			wantErr:      true,
		},
		{
			name:         "malformed JSON",
			propertyName: "server.port",
			mockResponse: `{"property": invalid}`,
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
			result, err := client.GetEnvProperty(tt.propertyName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetEnvProperty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPath != "" && requestedPath != tt.wantPath {
				t.Errorf("GET path = %v, want %v", requestedPath, tt.wantPath)
			}

			if !tt.wantErr && tt.wantValue != nil {
				if result.Property.Value != tt.wantValue {
					t.Errorf("value = %v, want %v", result.Property.Value, tt.wantValue)
				}
			}
		})
	}
}
