package actuator

import (
	"encoding/json"
	"strconv"
	"testing"
)

func TestActuatorClientGetScheduledTasks(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  string
		mockStatus    int
		mockErr       error
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

func TestScheduledTasksResponseUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		jsonInput string
		wantErr   bool
		validate  func(*testing.T, *ScheduledTasksResponse)
	}{
		{
			name: "complete cron task with all fields",
			jsonInput: `{
				"cron": [{
					"runnable": {"target": "com.example.Scheduler.method"},
					"expression": "0 0 12 * * ?",
					"nextExecution": {"time": "2025-10-22T12:00:00Z"},
					"lastExecution": {
						"time": "2025-10-21T12:00:00Z",
						"status": "SUCCESS"
					}
				}],
				"fixedDelay": [],
				"fixedRate": [],
				"custom": []
			}`,
			wantErr: false,
			validate: func(t *testing.T, resp *ScheduledTasksResponse) {
				if len(resp.Cron) != 1 {
					t.Errorf("expected 1 cron task, got %d", len(resp.Cron))
				}
				if resp.Cron[0].Expression != "0 0 12 * * ?" {
					t.Errorf("expected expression '0 0 12 * * ?', got '%s'", resp.Cron[0].Expression)
				}
			},
		},
		{
			name: "fixedDelay task with exception",
			jsonInput: `{
				"cron": [],
				"fixedDelay": [{
					"runnable": {"target": "com.example.Task.execute"},
					"initialDelay": 1000,
					"interval": 5000,
					"nextExecution": {"time": "2025-10-22T12:00:05Z"},
					"lastExecution": {
						"time": "2025-10-22T12:00:00Z",
						"status": "ERROR",
						"exception": {
							"message": "Database connection failed",
							"type": "java.sql.SQLException"
						}
					}
				}],
				"fixedRate": [],
				"custom": []
			}`,
			wantErr: false,
			validate: func(t *testing.T, resp *ScheduledTasksResponse) {
				if len(resp.FixedDelay) != 1 {
					t.Errorf("expected 1 fixedDelay task, got %d", len(resp.FixedDelay))
				}
				task := resp.FixedDelay[0]
				if task.LastExecution == nil {
					t.Fatal("expected lastExecution, got nil")
				}
				if task.LastExecution.Exception == nil {
					t.Fatal("expected exception, got nil")
				}
				if task.LastExecution.Exception.Message != "Database connection failed" {
					t.Errorf("unexpected exception message: %s", task.LastExecution.Exception.Message)
				}
			},
		},
		{
			name: "fixedRate task with STARTED status and no nextExecution",
			jsonInput: `{
				"cron": [],
				"fixedDelay": [],
				"fixedRate": [{
					"runnable": {"target": "com.example.LongRunningTask.process"},
					"initialDelay": 0,
					"interval": 30000,
					"lastExecution": {
						"time": "2025-10-22T12:00:00Z",
						"status": "STARTED"
					}
				}],
				"custom": []
			}`,
			wantErr: false,
			validate: func(t *testing.T, resp *ScheduledTasksResponse) {
				if len(resp.FixedRate) != 1 {
					t.Errorf("expected 1 fixedRate task, got %d", len(resp.FixedRate))
				}
				task := resp.FixedRate[0]
				if task.NextExecution != nil {
					t.Error("expected nil nextExecution for STARTED task")
				}
				if task.LastExecution.Status != "STARTED" {
					t.Errorf("expected status STARTED, got %s", task.LastExecution.Status)
				}
			},
		},
		{
			name: "custom task",
			jsonInput: `{
				"cron": [],
				"fixedDelay": [],
				"fixedRate": [],
				"custom": [{
					"runnable": {"target": "com.example.CustomTask.run"},
					"nextExecution": {"time": "2025-10-22T12:00:00Z"},
					"lastExecution": {
						"time": "2025-10-22T11:00:00Z",
						"status": "SUCCESS"
					}
				}]
			}`,
			wantErr: false,
			validate: func(t *testing.T, resp *ScheduledTasksResponse) {
				if len(resp.Custom) != 1 {
					t.Errorf("expected 1 custom task, got %d", len(resp.Custom))
				}
			},
		},
		{
			name: "all nulls",
			jsonInput: `{
				"cron": [{
					"runnable": {"target": "com.example.Task.run"},
					"expression": "* * * * * *",
					"nextExecution": null,
					"lastExecution": null
				}],
				"fixedDelay": [],
				"fixedRate": [],
				"custom": []
			}`,
			wantErr: false,
			validate: func(t *testing.T, resp *ScheduledTasksResponse) {
				if len(resp.Cron) != 1 {
					t.Errorf("expected 1 cron task, got %d", len(resp.Cron))
				}
				task := resp.Cron[0]
				if task.NextExecution != nil {
					t.Error("expected nil nextExecution")
				}
				if task.LastExecution != nil {
					t.Error("expected nil lastExecution")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response ScheduledTasksResponse
			err := json.Unmarshal([]byte(tt.jsonInput), &response)

			if (err != nil) != tt.wantErr {
				t.Errorf("json.Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, &response)
			}
		})
	}
}
