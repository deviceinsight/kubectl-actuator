package cmd

import (
	"strings"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

func TestScheduledTasksValidateFlags(t *testing.T) {
	t.Run("every allowed output format is accepted", func(t *testing.T) {
		for _, output := range []string{"", OutputFormatWide, OutputFormatJSON, OutputFormatYAML} {
			ops := &scheduledTasksCommandOperations{output: output}
			if err := ops.validateFlags(); err != nil {
				t.Errorf("output %q: %v", output, err)
			}
		}
	})

	t.Run("unknown output format is rejected", func(t *testing.T) {
		ops := &scheduledTasksCommandOperations{output: "table"}
		err := ops.validateFlags()
		if err == nil {
			t.Fatal("expected an error")
		}
		for _, want := range []string{"invalid output format", "table", "must be one of"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("expected error containing %q, got %q", want, err)
			}
		}
	})
}

func TestBuildTaskRowsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		response *actuator.ScheduledTasksResponse
		wantRows int
	}{
		{
			name:     "empty response",
			response: &actuator.ScheduledTasksResponse{},
			wantRows: 0,
		},
		{
			name: "only cron tasks",
			response: &actuator.ScheduledTasksResponse{
				Cron: []actuator.CronTask{
					{Runnable: actuator.Runnable{Target: "com.example.Task1.run"}, Expression: "0 * * * * *"},
					{Runnable: actuator.Runnable{Target: "com.example.Task2.run"}, Expression: "0 0 * * * *"},
				},
			},
			wantRows: 2,
		},
		{
			name: "only fixedDelay tasks",
			response: &actuator.ScheduledTasksResponse{
				FixedDelay: []actuator.FixedIntervalTask{
					{Runnable: actuator.Runnable{Target: "com.example.Task.execute"}, Interval: 5000},
				},
			},
			wantRows: 1,
		},
		{
			name: "only fixedRate tasks",
			response: &actuator.ScheduledTasksResponse{
				FixedRate: []actuator.FixedIntervalTask{
					{Runnable: actuator.Runnable{Target: "com.example.Metrics.export"}, Interval: 60000},
				},
			},
			wantRows: 1,
		},
		{
			name: "only custom tasks",
			response: &actuator.ScheduledTasksResponse{
				Custom: []actuator.CustomTask{
					{Runnable: actuator.Runnable{Target: "com.example.CustomTask.execute"}},
				},
			},
			wantRows: 1,
		},
		{
			name: "mixed task types",
			response: &actuator.ScheduledTasksResponse{
				Cron: []actuator.CronTask{
					{Runnable: actuator.Runnable{Target: "com.example.CronTask.run"}, Expression: "0 * * * * *"},
				},
				FixedDelay: []actuator.FixedIntervalTask{
					{Runnable: actuator.Runnable{Target: "com.example.DelayTask.execute"}, Interval: 5000},
				},
				FixedRate: []actuator.FixedIntervalTask{
					{Runnable: actuator.Runnable{Target: "com.example.RateTask.process"}, Interval: 10000},
				},
				Custom: []actuator.CustomTask{
					{Runnable: actuator.Runnable{Target: "com.example.CustomTask.run"}},
				},
			},
			wantRows: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := buildTaskRows(tt.response, false)

			if len(rows) != tt.wantRows {
				t.Errorf("got %d rows, want %d", len(rows), tt.wantRows)
			}
		})
	}
}

// A task without execution tracking (never ran, or Spring Boot < 3.4) has nil
// NextExecution and LastExecution; the columns must render as dashes.
func TestBuildTaskRowsNullExecutionsRenderDashes(t *testing.T) {
	response := &actuator.ScheduledTasksResponse{
		Cron: []actuator.CronTask{
			{Runnable: actuator.Runnable{Target: "com.example.Task.run"}, Expression: "0 * * * * *"},
		},
	}

	rows := buildTaskRows(response, false)

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.Next != "-" || row.Last != "-" || row.Status != "-" {
		t.Errorf("nil executions should render as dashes, got Next=%q Last=%q Status=%q", row.Next, row.Last, row.Status)
	}
}

func TestBuildTaskRowsSortsByTypeThenTarget(t *testing.T) {
	response := &actuator.ScheduledTasksResponse{
		Cron: []actuator.CronTask{
			{Runnable: actuator.Runnable{Target: "zzz.LastTask.run"}, Expression: "0 * * * * *"},
			{Runnable: actuator.Runnable{Target: "aaa.FirstTask.run"}, Expression: "0 0 * * * *"},
		},
		FixedDelay: []actuator.FixedIntervalTask{
			{Runnable: actuator.Runnable{Target: "bbb.MiddleTask.execute"}, Interval: 5000},
		},
	}

	rows := buildTaskRows(response, false)

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].Type != "cron" || rows[1].Type != "cron" || rows[2].Type != "fixedDelay" {
		t.Errorf("expected cron, cron, fixedDelay; got %s, %s, %s", rows[0].Type, rows[1].Type, rows[2].Type)
	}
	if !strings.Contains(rows[0].Target, "FirstTask") || !strings.Contains(rows[1].Target, "LastTask") {
		t.Errorf("expected targets sorted within type, got %s before %s", rows[0].Target, rows[1].Target)
	}
}
