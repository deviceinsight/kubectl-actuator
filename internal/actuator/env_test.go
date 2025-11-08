package actuator

import (
	"strconv"
	"testing"
)

func TestActuatorClientGetEnv(t *testing.T) {
	tests := []struct {
		name               string
		mockResponse       string
		mockStatus         int
		mockErr            error
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
			mockClient := &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					if path != "/env" {
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
		mockErr      error
		wantErr      bool
		wantPath     string
		wantValue    interface{}
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
			result, err := client.GetEnvProperty(tt.propertyName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetEnvProperty() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPath != "" && capturedPath != tt.wantPath {
				t.Errorf("GET path = %v, want %v", capturedPath, tt.wantPath)
			}

			if !tt.wantErr && tt.wantValue != nil {
				if result.Property.Value != tt.wantValue {
					t.Errorf("value = %v, want %v", result.Property.Value, tt.wantValue)
				}
			}
		})
	}
}

func TestEnvResponseParsing(t *testing.T) {
	tests := []struct {
		name     string
		response string
		validate func(*testing.T, *EnvResponse)
	}{
		{
			name: "property with origin",
			response: `{
				"activeProfiles": [],
				"propertySources": [
					{
						"name": "applicationConfig",
						"properties": {
							"server.port": {
								"value": "8080",
								"origin": "class path resource [application.yml]:1:14"
							}
						}
					}
				]
			}`,
			validate: func(t *testing.T, resp *EnvResponse) {
				if len(resp.PropertySources) != 1 {
					t.Fatalf("expected 1 property source, got %d", len(resp.PropertySources))
				}
				ps := resp.PropertySources[0]
				prop, ok := ps.Properties["server.port"]
				if !ok {
					t.Fatal("expected server.port property")
				}
				if prop.Origin != "class path resource [application.yml]:1:14" {
					t.Errorf("unexpected origin: %s", prop.Origin)
				}
			},
		},
		{
			name: "property with numeric value",
			response: `{
				"activeProfiles": [],
				"propertySources": [
					{
						"name": "systemProperties",
						"properties": {
							"java.specification.version": {"value": 17}
						}
					}
				]
			}`,
			validate: func(t *testing.T, resp *EnvResponse) {
				ps := resp.PropertySources[0]
				prop := ps.Properties["java.specification.version"]
				if prop.Value != float64(17) {
					t.Errorf("expected value 17, got %v (type %T)", prop.Value, prop.Value)
				}
			},
		},
		{
			name: "property with boolean value",
			response: `{
				"activeProfiles": [],
				"propertySources": [
					{
						"name": "applicationConfig",
						"properties": {
							"spring.jpa.show-sql": {"value": true}
						}
					}
				]
			}`,
			validate: func(t *testing.T, resp *EnvResponse) {
				ps := resp.PropertySources[0]
				prop := ps.Properties["spring.jpa.show-sql"]
				if prop.Value != true {
					t.Errorf("expected value true, got %v", prop.Value)
				}
			},
		},
		{
			name: "multiple property sources",
			response: `{
				"activeProfiles": ["dev"],
				"propertySources": [
					{"name": "commandLineArgs", "properties": {}},
					{"name": "systemProperties", "properties": {}},
					{"name": "systemEnvironment", "properties": {}},
					{"name": "applicationConfig", "properties": {}}
				]
			}`,
			validate: func(t *testing.T, resp *EnvResponse) {
				if len(resp.PropertySources) != 4 {
					t.Errorf("expected 4 property sources, got %d", len(resp.PropertySources))
				}
				if len(resp.ActiveProfiles) != 1 || resp.ActiveProfiles[0] != "dev" {
					t.Errorf("unexpected active profiles: %v", resp.ActiveProfiles)
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
			result, err := client.GetEnv()

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tt.validate(t, result)
		})
	}
}
