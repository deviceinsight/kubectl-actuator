package actuator

import "testing"

func TestActuatorClientGetLoggers(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  string
		mockStatus    int
		wantErr       bool
		wantLoggerCnt int
		wantGroupCnt  int
	}{
		{
			name: "successful response",
			mockResponse: `{
				"loggers": {
					"ROOT": {"configuredLevel": "INFO", "effectiveLevel": "INFO"},
					"com.example": {"configuredLevel": "DEBUG", "effectiveLevel": "DEBUG"}
				}
			}`,
			mockStatus:    200,
			wantErr:       false,
			wantLoggerCnt: 2,
		},
		{
			name: "groups are parsed",
			mockResponse: `{
				"loggers": {
					"ROOT": {"configuredLevel": "INFO", "effectiveLevel": "INFO"}
				},
				"groups": {
					"web": {"configuredLevel": "DEBUG", "members": ["org.springframework.http", "org.springframework.web"]},
					"sql": {"members": ["org.hibernate.SQL"]}
				}
			}`,
			mockStatus:    200,
			wantErr:       false,
			wantLoggerCnt: 1,
			wantGroupCnt:  2,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newEndpointMock(t, "/loggers", respondWith(tt.mockStatus, tt.mockResponse))

			client := &actuatorClient{httpClient: mockClient}
			response, err := client.GetLoggers()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetLoggers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(response.Loggers) != tt.wantLoggerCnt {
					t.Errorf("got %d loggers, want %d", len(response.Loggers), tt.wantLoggerCnt)
				}
				if len(response.Groups) != tt.wantGroupCnt {
					t.Errorf("got %d groups, want %d", len(response.Groups), tt.wantGroupCnt)
				}
			}
		})
	}
}

func TestActuatorClientSetLoggerLevel(t *testing.T) {
	tests := []struct {
		name       string
		loggerName string
		level      string
		mockStatus int
		wantErr    bool
		wantPath   string
	}{
		{
			name:       "successful set",
			loggerName: "com.example.app",
			level:      "DEBUG",
			mockStatus: 204,
			wantErr:    false,
			wantPath:   "/loggers/com.example.app",
		},
		{
			name:       "ROOT logger",
			loggerName: "ROOT",
			level:      "WARN",
			mockStatus: 204,
			wantErr:    false,
			wantPath:   "/loggers/ROOT",
		},
		{
			name:       "404 logger not found",
			loggerName: "invalid.logger",
			level:      "DEBUG",
			mockStatus: 404,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestedPath string
			respond := respondWith(tt.mockStatus, "")
			mockClient := &MockHTTPClient{
				PostFunc: func(path string, body any) (*Response, error) {
					requestedPath = path
					return respond()
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			err := client.SetLoggerLevel(tt.loggerName, tt.level)

			if (err != nil) != tt.wantErr {
				t.Errorf("SetLoggerLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && requestedPath != tt.wantPath {
				t.Errorf("POST path = %v, want %v", requestedPath, tt.wantPath)
			}
		})
	}
}
