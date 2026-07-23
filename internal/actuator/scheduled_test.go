package actuator

import "testing"

func TestActuatorClientGetScheduledTasks(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  string
		mockStatus    int
		wantErr       bool
		wantCronCnt   int
		wantFixedDCnt int
		wantFixedRCnt int
		wantCustomCnt int
	}{
		{
			name: "successful response with all task types",
			mockResponse: `{
				"cron": [
					{
						"runnable": {"target": "com.example.app.service.BackupScheduler.scheduleBackups"},
						"expression": "0 * * * * *",
						"nextExecution": {"time": "2025-10-22T17:35:59.999232070Z"},
						"lastExecution": {"time": "2025-10-22T17:35:00.000099506Z", "status": "SUCCESS"}
					}
				],
				"fixedDelay": [
					{
						"runnable": {"target": "com.example.app.service.StatusWatcher.checkStatus"},
						"initialDelay": 0,
						"interval": 5000,
						"nextExecution": {"time": "2025-10-22T17:35:50.863291470Z"},
						"lastExecution": {"time": "2025-10-22T17:35:45.792556698Z", "status": "SUCCESS"}
					},
					{
						"runnable": {"target": "com.example.app.service.CleanupService.cleanup"},
						"initialDelay": 900000,
						"interval": 43200000,
						"nextExecution": {"time": "2025-10-23T03:13:09.159317970Z"},
						"lastExecution": {
							"exception": {"message": "Connection timeout", "type": "java.net.SocketTimeoutException"},
							"time": "2025-10-22T15:12:44.057682493Z",
							"status": "ERROR"
						}
					}
				],
				"fixedRate": [
					{
						"runnable": {"target": "com.example.app.service.MetricsService.exportMetrics"},
						"initialDelay": 0,
						"interval": 60000,
						"lastExecution": {"time": "2025-10-22T17:35:43.421032561Z", "status": "STARTED"}
					}
				],
				"custom": []
			}`,
			mockStatus:    200,
			wantErr:       false,
			wantCronCnt:   1,
			wantFixedDCnt: 2,
			wantFixedRCnt: 1,
			wantCustomCnt: 0,
		},
		{
			name: "empty response",
			mockResponse: `{
				"cron": [],
				"fixedDelay": [],
				"fixedRate": [],
				"custom": []
			}`,
			mockStatus:    200,
			wantErr:       false,
			wantCronCnt:   0,
			wantFixedDCnt: 0,
			wantFixedRCnt: 0,
			wantCustomCnt: 0,
		},
		{
			name: "tasks with null nextExecution",
			mockResponse: `{
				"cron": [],
				"fixedDelay": [
					{
						"runnable": {"target": "com.example.app.service.OneTimeTask.execute"},
						"initialDelay": 0,
						"interval": 5000,
						"nextExecution": null,
						"lastExecution": {"time": "2025-10-22T17:35:45.792556698Z", "status": "SUCCESS"}
					}
				],
				"fixedRate": [],
				"custom": []
			}`,
			mockStatus:    200,
			wantErr:       false,
			wantFixedDCnt: 1,
		},
		{
			name: "tasks with null lastExecution",
			mockResponse: `{
				"cron": [
					{
						"runnable": {"target": "com.example.app.service.NewTask.run"},
						"expression": "0 0 * * * *",
						"nextExecution": {"time": "2025-10-22T18:00:00.000000000Z"},
						"lastExecution": null
					}
				],
				"fixedDelay": [],
				"fixedRate": [],
				"custom": []
			}`,
			mockStatus:  200,
			wantErr:     false,
			wantCronCnt: 1,
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
			mockResponse: `{"cron": [invalid json}`,
			mockStatus:   200,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := newEndpointMock(t, "/scheduledtasks", respondWith(tt.mockStatus, tt.mockResponse))

			client := &actuatorClient{httpClient: mockClient}
			result, err := client.GetScheduledTasks()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetScheduledTasks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(result.Cron) != tt.wantCronCnt {
					t.Errorf("got %d cron tasks, want %d", len(result.Cron), tt.wantCronCnt)
				}
				if len(result.FixedDelay) != tt.wantFixedDCnt {
					t.Errorf("got %d fixedDelay tasks, want %d", len(result.FixedDelay), tt.wantFixedDCnt)
				}
				if len(result.FixedRate) != tt.wantFixedRCnt {
					t.Errorf("got %d fixedRate tasks, want %d", len(result.FixedRate), tt.wantFixedRCnt)
				}
				if len(result.Custom) != tt.wantCustomCnt {
					t.Errorf("got %d custom tasks, want %d", len(result.Custom), tt.wantCustomCnt)
				}
			}
		})
	}
}
