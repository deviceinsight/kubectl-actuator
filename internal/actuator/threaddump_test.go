package actuator

import "testing"

func TestActuatorClientGetThreadDump(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   string
		mockStatus     int
		wantErr        bool
		wantThreadsCnt int
	}{
		{
			name: "successful response with threads",
			mockResponse: `{
				"threads": [
					{
						"threadName": "main",
						"threadId": 1,
						"threadState": "RUNNABLE",
						"blockedCount": 0,
						"blockedTime": -1,
						"waitedCount": 0,
						"waitedTime": -1,
						"lockOwnerId": -1,
						"daemon": false,
						"inNative": false,
						"suspended": false,
						"priority": 5,
						"stackTrace": [
							{
								"className": "java.lang.Thread",
								"methodName": "sleep",
								"fileName": "Thread.java",
								"lineNumber": 340,
								"nativeMethod": true
							}
						],
						"lockedMonitors": [],
						"lockedSynchronizers": []
					},
					{
						"threadName": "http-nio-8080-exec-1",
						"threadId": 42,
						"threadState": "WAITING",
						"blockedCount": 5,
						"blockedTime": -1,
						"waitedCount": 100,
						"waitedTime": -1,
						"lockOwnerId": -1,
						"daemon": true,
						"inNative": false,
						"suspended": false,
						"priority": 5,
						"stackTrace": [],
						"lockedMonitors": [],
						"lockedSynchronizers": []
					}
				]
			}`,
			mockStatus:     200,
			wantErr:        false,
			wantThreadsCnt: 2,
		},
		{
			name:           "empty threads list",
			mockResponse:   `{"threads": []}`,
			mockStatus:     200,
			wantErr:        false,
			wantThreadsCnt: 0,
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
			mockResponse: `{"threads": invalid}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newEndpointMock(t, "/threaddump", respondWith(tt.mockStatus, tt.mockResponse))

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetThreadDump()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetThreadDump() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Threads) != tt.wantThreadsCnt {
					t.Errorf("got %d threads, want %d", len(result.Threads), tt.wantThreadsCnt)
				}
			}
		})
	}
}
