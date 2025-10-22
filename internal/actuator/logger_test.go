package actuator

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestParseLoggersResponse(t *testing.T) {
	tests := []struct {
		name          string
		jsonInput     string
		wantLoggerCnt int
		wantErr       bool
	}{
		{
			name: "valid response with ROOT and custom loggers",
			jsonInput: `{
				"loggers": {
					"ROOT": {
						"configuredLevel": "INFO",
						"effectiveLevel": "INFO"
					},
					"com.example.app": {
						"configuredLevel": "DEBUG",
						"effectiveLevel": "DEBUG"
					}
				}
			}`,
			wantLoggerCnt: 2,
			wantErr:       false,
		},
		{
			name: "logger with null configured level",
			jsonInput: `{
				"loggers": {
					"org.springframework": {
						"configuredLevel": null,
						"effectiveLevel": "INFO"
					}
				}
			}`,
			wantLoggerCnt: 1,
			wantErr:       false,
		},
		{
			name:          "empty loggers",
			jsonInput:     `{"loggers": {}}`,
			wantLoggerCnt: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response loggersResponse
			err := json.Unmarshal([]byte(tt.jsonInput), &response)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(response.Loggers) != tt.wantLoggerCnt {
				t.Errorf("got %d loggers, want %d", len(response.Loggers), tt.wantLoggerCnt)
			}
		})
	}
}

func TestResponseIsErrorStatus(t *testing.T) {
	tests := []struct {
		statusCode int
		wantError  bool
	}{
		{200, false},
		{201, false},
		{204, false},
		{400, true},
		{401, true},
		{404, true},
		{500, true},
		{502, true},
	}

	for _, tt := range tests {
		t.Run("status_"+string(rune(tt.statusCode)), func(t *testing.T) {
			resp := &Response{StatusCode: tt.statusCode}
			got := resp.IsErrorStatus()
			if got != tt.wantError {
				t.Errorf("Response.IsErrorStatus() with status %d = %v, want %v", tt.statusCode, got, tt.wantError)
			}
		})
	}
}

type MockHTTPClient struct {
	GetFunc  func(path string) (*Response, error)
	PostFunc func(path string, body interface{}) (*Response, error)
}

func (m *MockHTTPClient) Get(path string) (*Response, error) {
	if m.GetFunc != nil {
		return m.GetFunc(path)
	}
	return &Response{Body: nil, StatusCode: 200, Status: "200 OK"}, nil
}

func (m *MockHTTPClient) Post(path string, body interface{}) (*Response, error) {
	if m.PostFunc != nil {
		return m.PostFunc(path, body)
	}
	return &Response{Body: nil, StatusCode: 200, Status: "200 OK"}, nil
}

func TestActuatorClientGetLoggers(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  string
		mockStatus    int
		mockErr       error
		wantErr       bool
		wantLoggerCnt int
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
			loggers, err := client.GetLoggers()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetLoggers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(loggers) != tt.wantLoggerCnt {
				t.Errorf("got %d loggers, want %d", len(loggers), tt.wantLoggerCnt)
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
			var capturedPath string
			mockClient := &MockHTTPClient{
				PostFunc: func(path string, body interface{}) (*Response, error) {
					capturedPath = path
					return &Response{
						Body:       []byte{},
						StatusCode: tt.mockStatus,
						Status:     strconv.Itoa(tt.mockStatus),
					}, nil
				},
			}

			client := &actuatorClient{httpClient: mockClient}
			err := client.SetLoggerLevel(tt.loggerName, tt.level)

			if (err != nil) != tt.wantErr {
				t.Errorf("SetLoggerLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && capturedPath != tt.wantPath {
				t.Errorf("POST path = %v, want %v", capturedPath, tt.wantPath)
			}
		})
	}
}
