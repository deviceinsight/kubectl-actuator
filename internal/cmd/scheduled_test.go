package cmd

import (
	"testing"
	"time"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

func TestFormatMs(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"zero", 0, "0s"},
		{"1 second", 1000, "1s"},
		{"5 seconds", 5000, "5s"},
		{"1 minute", 60000, "1m"},
		{"5 minutes", 300000, "5m"},
		{"1 minute 30 seconds", 90000, "1m30s"},
		{"1 hour", 3600000, "1h"},
		{"1 hour 30 minutes", 5400000, "1h30m"},
		{"2 hours 15 minutes 30 seconds", 8130000, "2h15m30s"},
		{"24 hours", 86400000, "24h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMs(tt.ms)
			if got != tt.want {
				t.Errorf("formatMs(%d) = %v, want %v", tt.ms, got, tt.want)
			}
		})
	}
}

func TestFriendlyDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"0 seconds", 0 * time.Second, "0s"},
		{"1 second", 1 * time.Second, "1s"},
		{"30 seconds", 30 * time.Second, "30s"},
		{"1 minute", 1 * time.Minute, "1m"},
		{"2 minutes 15 seconds", 2*time.Minute + 15*time.Second, "2m15s"},
		{"1 hour", 1 * time.Hour, "1h"},
		{"1 hour 5 minutes", 1*time.Hour + 5*time.Minute, "1h5m"},
		{"negative duration", -5 * time.Minute, "5m"},
		{"subsecond rounds down", 500 * time.Millisecond, "1s"}, // rounds to nearest second
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := friendlyDuration(tt.d)
			if got != tt.want {
				t.Errorf("friendlyDuration(%v) = %v, want %v", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormatTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		showFull bool
		want     string
	}{
		{
			name:     "short format - fully qualified",
			target:   "com.example.app.service.BackupScheduler.scheduleBackups",
			showFull: false,
			want:     "BackupScheduler.scheduleBackups",
		},
		{
			name:     "short format - two parts",
			target:   "ClassName.methodName",
			showFull: false,
			want:     "ClassName.methodName",
		},
		{
			name:     "short format - single part",
			target:   "simpleMethod",
			showFull: false,
			want:     "simpleMethod",
		},
		{
			name:     "full format",
			target:   "com.example.app.service.BackupScheduler.scheduleBackups",
			showFull: true,
			want:     "com.example.app.service.BackupScheduler.scheduleBackups",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTarget(tt.target, tt.showFull)
			if got != tt.want {
				t.Errorf("formatTarget(%q, %v) = %v, want %v", tt.target, tt.showFull, got, tt.want)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
	}{
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
		{
			name:    "valid RFC3339",
			input:   "2025-10-22T19:00:00Z",
			wantNil: false,
		},
		{
			name:    "valid RFC3339 with nanoseconds",
			input:   "2025-10-22T19:00:00.123456789Z",
			wantNil: false,
		},
		{
			name:    "invalid format",
			input:   "not-a-time",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTime(tt.input)
			if (got == nil) != tt.wantNil {
				t.Errorf("parseTime(%q) nil = %v, want nil = %v", tt.input, got == nil, tt.wantNil)
			}
		})
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name           string
		execution      *Execution
		showFullStatus bool
		want           string
	}{
		{
			name:           "nil execution",
			execution:      nil,
			showFullStatus: false,
			want:           "-",
		},
		{
			name: "SUCCESS status",
			execution: &Execution{
				Time:   "2025-10-22T19:00:00Z",
				Status: "SUCCESS",
			},
			showFullStatus: false,
			want:           "SUCCESS",
		},
		{
			name: "ERROR with short message",
			execution: &Execution{
				Time:   "2025-10-22T19:00:00Z",
				Status: "ERROR",
				Exception: &Exception{
					Message: "Connection timeout",
					Type:    "java.net.SocketTimeoutException",
				},
			},
			showFullStatus: false,
			want:           "ERROR - Connection timeout",
		},
		{
			name: "ERROR with long message truncated",
			execution: &Execution{
				Time:   "2025-10-22T19:00:00Z",
				Status: "ERROR",
				Exception: &Exception{
					Message: "This is a very long error message that exceeds the 80 character limit and should be truncated with an ellipsis",
					Type:    "java.lang.Exception",
				},
			},
			showFullStatus: false,
			want:           "ERROR - This is a very long error message that exceeds the 80 character limit and should…",
		},
		{
			name: "ERROR with long message not truncated in full mode",
			execution: &Execution{
				Time:   "2025-10-22T19:00:00Z",
				Status: "ERROR",
				Exception: &Exception{
					Message: "This is a very long error message that exceeds the 80 character limit but should not be truncated in full mode",
					Type:    "java.lang.Exception",
				},
			},
			showFullStatus: true,
			want:           "ERROR - This is a very long error message that exceeds the 80 character limit but should not be truncated in full mode",
		},
		{
			name: "empty status",
			execution: &Execution{
				Time:   "2025-10-22T19:00:00Z",
				Status: "",
			},
			showFullStatus: false,
			want:           "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to actuator.Execution type
			var actuatorExec *actuator.Execution
			if tt.execution != nil {
				actuatorExec = &actuator.Execution{
					Time:   tt.execution.Time,
					Status: tt.execution.Status,
				}
				if tt.execution.Exception != nil {
					actuatorExec.Exception = &actuator.Exception{
						Message: tt.execution.Exception.Message,
						Type:    tt.execution.Exception.Type,
					}
				}
			}

			got := formatStatus(actuatorExec, tt.showFullStatus)
			if got != tt.want {
				t.Errorf("formatStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

type Execution struct {
	Time      string
	Status    string
	Exception *Exception
}

type Exception struct {
	Message string
	Type    string
}
