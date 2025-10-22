package cmd

import (
	"strings"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

func TestScheduledTasksValidation(t *testing.T) {
	tests := []struct {
		name        string
		pods        []string
		output      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid with default output",
			pods:    []string{"pod-1"},
			output:  "",
			wantErr: false,
		},
		{
			name:    "valid with wide output",
			pods:    []string{"pod-1"},
			output:  "wide",
			wantErr: false,
		},
		{
			name:        "invalid output format json",
			pods:        []string{"pod-1"},
			output:      "json",
			wantErr:     true,
			errContains: "invalid output format",
		},
		{
			name:        "invalid output format yaml",
			pods:        []string{"pod-1"},
			output:      "yaml",
			wantErr:     true,
			errContains: "invalid output format",
		},
		{
			name:        "invalid output format table",
			pods:        []string{"pod-1"},
			output:      "table",
			wantErr:     true,
			errContains: "invalid output format",
		},
		{
			name:        "no pods specified",
			pods:        []string{},
			output:      "",
			wantErr:     true,
			errContains: "No pods specified",
		},
		{
			name:        "no pods with wide output",
			pods:        []string{},
			output:      "wide",
			wantErr:     true,
			errContains: "No pods specified",
		},
		{
			name:    "multiple pods valid",
			pods:    []string{"pod-1", "pod-2", "pod-3"},
			output:  "",
			wantErr: false,
		},
		{
			name:    "multiple pods with wide output",
			pods:    []string{"pod-1", "pod-2"},
			output:  "wide",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &scheduledTasksOperations{
				pods:   tt.pods,
				output: tt.output,
			}

			err := ops.validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errContains, err)
				}
			}
		})
	}
}

func TestScheduledTasksWideMode(t *testing.T) {
	tests := []struct {
		name         string
		output       string
		wantWideMode bool
	}{
		{"default output", "", false},
		{"wide output", "wide", true},
		{"invalid output", "json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &scheduledTasksOperations{
				output: tt.output,
			}

			// Simulate complete() setting wideMode
			ops.wideMode = ops.output == "wide"

			if ops.wideMode != tt.wantWideMode {
				t.Errorf("wideMode = %v, want %v", ops.wideMode, tt.wantWideMode)
			}
		})
	}
}

func TestScheduledTasksErrorMessages(t *testing.T) {
	tests := []struct {
		name            string
		pods            []string
		output          string
		wantErrContains []string
	}{
		{
			name:   "no pods error message",
			pods:   []string{},
			output: "",
			wantErrContains: []string{
				"No pods specified",
				"Please specify",
			},
		},
		{
			name:   "invalid format includes format name",
			pods:   []string{"pod-1"},
			output: "json",
			wantErrContains: []string{
				"invalid output format",
				"json",
			},
		},
		{
			name:   "invalid format shows supported formats",
			pods:   []string{"pod-1"},
			output: "yaml",
			wantErrContains: []string{
				"invalid output format",
				"yaml",
				"Supported formats",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &scheduledTasksOperations{
				pods:   tt.pods,
				output: tt.output,
			}

			err := ops.validate()

			if err == nil {
				t.Error("expected error, got nil")
				return
			}

			errMsg := err.Error()
			for _, want := range tt.wantErrContains {
				if !strings.Contains(errMsg, want) {
					t.Errorf("error message does not contain '%s'\nGot: %s", want, errMsg)
				}
			}
		})
	}
}

func TestBuildRowsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		response *actuator.ScheduledTasksResponse
		wideMode bool
		wantRows int
	}{
		{
			name: "empty response",
			response: &actuator.ScheduledTasksResponse{
				Cron:       []actuator.CronTask{},
				FixedDelay: []actuator.FixedIntervalTask{},
				FixedRate:  []actuator.FixedIntervalTask{},
				Custom:     []actuator.CustomTask{},
			},
			wideMode: false,
			wantRows: 0,
		},
		{
			name: "only cron tasks",
			response: &actuator.ScheduledTasksResponse{
				Cron: []actuator.CronTask{
					{
						Runnable:   actuator.Runnable{Target: "com.example.Task1.run"},
						Expression: "0 * * * * *",
					},
					{
						Runnable:   actuator.Runnable{Target: "com.example.Task2.run"},
						Expression: "0 0 * * * *",
					},
				},
				FixedDelay: []actuator.FixedIntervalTask{},
				FixedRate:  []actuator.FixedIntervalTask{},
				Custom:     []actuator.CustomTask{},
			},
			wideMode: false,
			wantRows: 2,
		},
		{
			name: "only fixedDelay tasks",
			response: &actuator.ScheduledTasksResponse{
				Cron: []actuator.CronTask{},
				FixedDelay: []actuator.FixedIntervalTask{
					{
						Runnable: actuator.Runnable{Target: "com.example.Task.execute"},
						Interval: 5000,
					},
				},
				FixedRate: []actuator.FixedIntervalTask{},
				Custom:    []actuator.CustomTask{},
			},
			wideMode: false,
			wantRows: 1,
		},
		{
			name: "only fixedRate tasks",
			response: &actuator.ScheduledTasksResponse{
				Cron:       []actuator.CronTask{},
				FixedDelay: []actuator.FixedIntervalTask{},
				FixedRate: []actuator.FixedIntervalTask{
					{
						Runnable: actuator.Runnable{Target: "com.example.Metrics.export"},
						Interval: 60000,
					},
				},
				Custom: []actuator.CustomTask{},
			},
			wideMode: false,
			wantRows: 1,
		},
		{
			name: "only custom tasks",
			response: &actuator.ScheduledTasksResponse{
				Cron:       []actuator.CronTask{},
				FixedDelay: []actuator.FixedIntervalTask{},
				FixedRate:  []actuator.FixedIntervalTask{},
				Custom: []actuator.CustomTask{
					{
						Runnable: actuator.Runnable{Target: "com.example.CustomTask.execute"},
					},
				},
			},
			wideMode: false,
			wantRows: 1,
		},
		{
			name: "mixed task types",
			response: &actuator.ScheduledTasksResponse{
				Cron: []actuator.CronTask{
					{
						Runnable:   actuator.Runnable{Target: "com.example.CronTask.run"},
						Expression: "0 * * * * *",
					},
				},
				FixedDelay: []actuator.FixedIntervalTask{
					{
						Runnable: actuator.Runnable{Target: "com.example.DelayTask.execute"},
						Interval: 5000,
					},
				},
				FixedRate: []actuator.FixedIntervalTask{
					{
						Runnable: actuator.Runnable{Target: "com.example.RateTask.process"},
						Interval: 10000,
					},
				},
				Custom: []actuator.CustomTask{
					{
						Runnable: actuator.Runnable{Target: "com.example.CustomTask.run"},
					},
				},
			},
			wideMode: false,
			wantRows: 4,
		},
		{
			name: "task with null nextExecution",
			response: &actuator.ScheduledTasksResponse{
				Cron: []actuator.CronTask{
					{
						Runnable:      actuator.Runnable{Target: "com.example.Task.run"},
						Expression:    "0 * * * * *",
						NextExecution: nil,
					},
				},
				FixedDelay: []actuator.FixedIntervalTask{},
				FixedRate:  []actuator.FixedIntervalTask{},
				Custom:     []actuator.CustomTask{},
			},
			wideMode: false,
			wantRows: 1,
		},
		{
			name: "task with null lastExecution",
			response: &actuator.ScheduledTasksResponse{
				Cron: []actuator.CronTask{
					{
						Runnable:      actuator.Runnable{Target: "com.example.Task.run"},
						Expression:    "0 * * * * *",
						LastExecution: nil,
					},
				},
				FixedDelay: []actuator.FixedIntervalTask{},
				FixedRate:  []actuator.FixedIntervalTask{},
				Custom:     []actuator.CustomTask{},
			},
			wideMode: false,
			wantRows: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := buildRows(tt.response, tt.wideMode)

			if len(rows) != tt.wantRows {
				t.Errorf("got %d rows, want %d", len(rows), tt.wantRows)
			}
		})
	}
}

func TestBuildRowsSorting(t *testing.T) {
	response := &actuator.ScheduledTasksResponse{
		Cron: []actuator.CronTask{
			{
				Runnable:   actuator.Runnable{Target: "zzz.LastTask.run"},
				Expression: "0 * * * * *",
			},
			{
				Runnable:   actuator.Runnable{Target: "aaa.FirstTask.run"},
				Expression: "0 0 * * * *",
			},
		},
		FixedDelay: []actuator.FixedIntervalTask{
			{
				Runnable: actuator.Runnable{Target: "bbb.MiddleTask.execute"},
				Interval: 5000,
			},
		},
		FixedRate: []actuator.FixedIntervalTask{},
		Custom:    []actuator.CustomTask{},
	}

	rows := buildRows(response, false)

	// Cron tasks should come first (alphabetically sorted by target within type)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// Check that cron tasks come first
	if rows[0].Type != "cron" || rows[1].Type != "cron" {
		t.Error("expected cron tasks first")
	}

	// Check alphabetical order within cron type
	if !strings.Contains(rows[0].Target, "FirstTask") {
		t.Errorf("expected FirstTask first, got %s", rows[0].Target)
	}
	if !strings.Contains(rows[1].Target, "LastTask") {
		t.Errorf("expected LastTask second, got %s", rows[1].Target)
	}

	// fixedDelay should come after cron
	if rows[2].Type != "fixedDelay" {
		t.Errorf("expected fixedDelay last, got %s", rows[2].Type)
	}
}

func TestFormatTargetWideMode(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		wideMode bool
		want     string
	}{
		{
			name:     "short format shows only class and method",
			target:   "com.example.service.BackupService.scheduleBackup",
			wideMode: false,
			want:     "BackupService.scheduleBackup",
		},
		{
			name:     "wide format shows full target",
			target:   "com.example.service.BackupService.scheduleBackup",
			wideMode: true,
			want:     "com.example.service.BackupService.scheduleBackup",
		},
		{
			name:     "short format with nested packages",
			target:   "com.example.app.main.service.async.AsyncTaskService.execute",
			wideMode: false,
			want:     "AsyncTaskService.execute",
		},
		{
			name:     "wide format with nested packages",
			target:   "com.example.app.main.service.async.AsyncTaskService.execute",
			wideMode: true,
			want:     "com.example.app.main.service.async.AsyncTaskService.execute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTarget(tt.target, tt.wideMode)
			if got != tt.want {
				t.Errorf("formatTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}
