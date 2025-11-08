package cmd

import (
	"strings"
	"testing"
)

func TestLoggerValidation(t *testing.T) {
	tests := []struct {
		name        string
		pods        []string
		targetLevel string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid level TRACE",
			pods:        []string{"pod-1"},
			targetLevel: "TRACE",
			wantErr:     false,
		},
		{
			name:        "valid level DEBUG",
			pods:        []string{"pod-1"},
			targetLevel: "DEBUG",
			wantErr:     false,
		},
		{
			name:        "valid level INFO",
			pods:        []string{"pod-1"},
			targetLevel: "INFO",
			wantErr:     false,
		},
		{
			name:        "valid level WARN",
			pods:        []string{"pod-1"},
			targetLevel: "WARN",
			wantErr:     false,
		},
		{
			name:        "valid level ERROR",
			pods:        []string{"pod-1"},
			targetLevel: "ERROR",
			wantErr:     false,
		},
		{
			name:        "valid level FATAL",
			pods:        []string{"pod-1"},
			targetLevel: "FATAL",
			wantErr:     false,
		},
		{
			name:        "valid level OFF",
			pods:        []string{"pod-1"},
			targetLevel: "OFF",
			wantErr:     false,
		},
		{
			name:        "invalid level VERBOSE",
			pods:        []string{"pod-1"},
			targetLevel: "VERBOSE",
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name:        "invalid level lowercase debug - should be uppercase before validation",
			pods:        []string{"pod-1"},
			targetLevel: "debug",
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name:        "invalid level FINE",
			pods:        []string{"pod-1"},
			targetLevel: "FINE",
			wantErr:     true,
			errContains: "invalid log level",
		},
		{
			name:        "empty level when just getting loggers",
			pods:        []string{"pod-1"},
			targetLevel: "",
			wantErr:     false,
		},
		{
			name:        "no pods specified",
			pods:        []string{},
			targetLevel: "INFO",
			wantErr:     true,
			errContains: "no pods selected",
		},
		{
			name:        "no pods and no level",
			pods:        []string{},
			targetLevel: "",
			wantErr:     true,
			errContains: "no pods selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &loggerCommandOperations{
				baseOperations: baseOperations{pods: tt.pods},
				targetLevel:    tt.targetLevel,
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

func TestLoggerLevelParsing(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLevel string
		wantName  string
	}{
		{
			name:      "no args - get all loggers",
			args:      []string{},
			wantLevel: "",
			wantName:  "",
		},
		{
			name:      "one arg - get specific logger",
			args:      []string{"com.example.Logger"},
			wantLevel: "",
			wantName:  "com.example.Logger",
		},
		{
			name:      "two args - set logger level",
			args:      []string{"com.example.Logger", "DEBUG"},
			wantLevel: "DEBUG",
			wantName:  "com.example.Logger",
		},
		{
			name:      "lowercase level converted to uppercase",
			args:      []string{"ROOT", "debug"},
			wantLevel: "DEBUG",
			wantName:  "ROOT",
		},
		{
			name:      "mixed case level converted to uppercase",
			args:      []string{"com.example", "Info"},
			wantLevel: "INFO",
			wantName:  "com.example",
		},
		{
			name:      "RESET level converted to empty string",
			args:      []string{"com.example", "RESET"},
			wantLevel: "",
			wantName:  "com.example",
		},
		{
			name:      "lowercase reset converted to empty string",
			args:      []string{"com.example", "reset"},
			wantLevel: "",
			wantName:  "com.example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &loggerCommandOperations{}

			// Simulate the argument parsing logic from complete()
			if len(tt.args) >= 1 {
				ops.loggerName = tt.args[0]
			}
			if len(tt.args) >= 2 {
				level := strings.ToUpper(tt.args[1])
				if level == "RESET" {
					ops.targetLevel = ""
					ops.isSettingLevel = true
				} else {
					ops.targetLevel = level
					ops.isSettingLevel = true
				}
			}

			if ops.loggerName != tt.wantName {
				t.Errorf("loggerName = %v, want %v", ops.loggerName, tt.wantName)
			}
			if ops.targetLevel != tt.wantLevel {
				t.Errorf("targetLevel = %v, want %v", ops.targetLevel, tt.wantLevel)
			}
		})
	}
}

func TestValidArgsLogLevel(t *testing.T) {
	ops := &loggerCommandOperations{}

	levels, directive := ops.validArgsLogLevel()

	// Should return all supported levels
	expectedLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF", "RESET"}

	if len(levels) != len(expectedLevels) {
		t.Errorf("got %d levels, want %d", len(levels), len(expectedLevels))
	}

	// Check all expected levels are present
	levelMap := make(map[string]bool)
	for _, level := range levels {
		levelMap[level] = true
	}

	for _, expected := range expectedLevels {
		if !levelMap[expected] {
			t.Errorf("expected level %s not found in completion", expected)
		}
	}

	// Check directive (cobra.ShellCompDirectiveNoFileComp = 4)
	if directive != 4 {
		t.Errorf("unexpected directive %v, want 4 (ShellCompDirectiveNoFileComp)", directive)
	}
}

func TestSupportedLevelsConstant(t *testing.T) {
	expectedLevels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF", "RESET"}

	if len(supportedLevels) != len(expectedLevels) {
		t.Errorf("supportedLevels has %d elements, expected %d", len(supportedLevels), len(expectedLevels))
	}

	levelMap := make(map[string]bool)
	for _, level := range supportedLevels {
		levelMap[level] = true
	}

	for _, expected := range expectedLevels {
		if !levelMap[expected] {
			t.Errorf("expected level %s not found in supportedLevels", expected)
		}
	}
}

func TestLoggerValidationErrorMessages(t *testing.T) {
	tests := []struct {
		name            string
		pods            []string
		targetLevel     string
		wantErrContains []string
	}{
		{
			name:        "no pods error mentions pods",
			pods:        []string{},
			targetLevel: "DEBUG",
			wantErrContains: []string{
				"no pods selected",
				"--pod",
			},
		},
		{
			name:        "invalid level shows unsupported level",
			pods:        []string{"pod-1"},
			targetLevel: "INVALID",
			wantErrContains: []string{
				"invalid log level",
				"INVALID",
			},
		},
		{
			name:        "invalid level shows supported levels",
			pods:        []string{"pod-1"},
			targetLevel: "VERBOSE",
			wantErrContains: []string{
				"invalid log level",
				"VERBOSE",
				"Valid levels",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &loggerCommandOperations{
				baseOperations: baseOperations{pods: tt.pods},
				targetLevel:    tt.targetLevel,
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

func TestLoggerOperationsMultiplePods(t *testing.T) {
	tests := []struct {
		name        string
		pods        []string
		targetLevel string
		wantErr     bool
	}{
		{
			name:        "single pod valid",
			pods:        []string{"pod-1"},
			targetLevel: "DEBUG",
			wantErr:     false,
		},
		{
			name:        "multiple pods valid",
			pods:        []string{"pod-1", "pod-2", "pod-3"},
			targetLevel: "INFO",
			wantErr:     false,
		},
		{
			name:        "many pods valid",
			pods:        []string{"pod-1", "pod-2", "pod-3", "pod-4", "pod-5"},
			targetLevel: "WARN",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &loggerCommandOperations{
				baseOperations: baseOperations{pods: tt.pods},
				targetLevel:    tt.targetLevel,
			}

			err := ops.validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
