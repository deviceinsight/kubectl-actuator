package cmd

import (
	"strings"
	"testing"
)

func TestLoggerValidateFlags(t *testing.T) {
	t.Run("every supported level is accepted", func(t *testing.T) {
		for _, level := range supportedLevels {
			if level == "RESET" {
				continue // parseArgs turns RESET into the reset flag, never a target level
			}
			ops := &loggerCommandOperations{isSettingLevel: true, targetLevel: level}
			if err := ops.validateFlags(); err != nil {
				t.Errorf("level %s: %v", level, err)
			}
		}
	})

	t.Run("get mode needs no level", func(t *testing.T) {
		ops := &loggerCommandOperations{}
		if err := ops.validateFlags(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	invalid := []struct {
		name        string
		level       string
		errContains []string
	}{
		{"invalid level VERBOSE", "VERBOSE", []string{"invalid log level", "VERBOSE", "Valid levels"}},
		{"lowercase level is rejected; parseArgs uppercases before validation", "debug", []string{"invalid log level"}},
		{"invalid level FINE", "FINE", []string{"invalid log level"}},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			ops := &loggerCommandOperations{isSettingLevel: true, targetLevel: tt.level}
			err := ops.validateFlags()
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, want := range tt.errContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("expected error containing %q, got %q", want, err)
				}
			}
		})
	}

	t.Run("resetting ROOT is rejected", func(t *testing.T) {
		ops := &loggerCommandOperations{isSettingLevel: true, reset: true, loggerName: "ROOT"}
		if err := ops.validateFlags(); err == nil || !strings.Contains(err.Error(), "cannot reset ROOT") {
			t.Errorf("expected the ROOT reset error, got %v", err)
		}
	})

	t.Run("--output conflicts with setting a level", func(t *testing.T) {
		ops := &loggerCommandOperations{isSettingLevel: true, targetLevel: "INFO", output: OutputFormatJSON}
		if err := ops.validateFlags(); err == nil || !strings.Contains(err.Error(), "--output") {
			t.Errorf("expected the --output conflict error, got %v", err)
		}
	})

	t.Run("--all conflicts with setting a level", func(t *testing.T) {
		ops := &loggerCommandOperations{isSettingLevel: true, targetLevel: "INFO", showAllLoggers: true}
		if err := ops.validateFlags(); err == nil || !strings.Contains(err.Error(), "--all") {
			t.Errorf("expected the --all conflict error, got %v", err)
		}
	})
}

func TestLoggerParseArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantLevel   string
		wantName    string
		wantSetting bool
		wantReset   bool
	}{
		{
			name: "no args - get all loggers",
			args: []string{},
		},
		{
			name:     "one arg - get specific logger",
			args:     []string{"com.example.Logger"},
			wantName: "com.example.Logger",
		},
		{
			name:        "two args - set logger level",
			args:        []string{"com.example.Logger", "DEBUG"},
			wantLevel:   "DEBUG",
			wantName:    "com.example.Logger",
			wantSetting: true,
		},
		{
			name:        "lowercase level converted to uppercase",
			args:        []string{"ROOT", "debug"},
			wantLevel:   "DEBUG",
			wantName:    "ROOT",
			wantSetting: true,
		},
		{
			name:        "mixed case level converted to uppercase",
			args:        []string{"com.example", "Info"},
			wantLevel:   "INFO",
			wantName:    "com.example",
			wantSetting: true,
		},
		{
			name:        "RESET becomes the reset flag, not a level",
			args:        []string{"com.example", "RESET"},
			wantName:    "com.example",
			wantSetting: true,
			wantReset:   true,
		},
		{
			name:        "lowercase reset becomes the reset flag too",
			args:        []string{"com.example", "reset"},
			wantName:    "com.example",
			wantSetting: true,
			wantReset:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &loggerCommandOperations{}

			ops.parseArgs(tt.args)

			if ops.loggerName != tt.wantName {
				t.Errorf("loggerName = %v, want %v", ops.loggerName, tt.wantName)
			}
			if ops.targetLevel != tt.wantLevel {
				t.Errorf("targetLevel = %v, want %v", ops.targetLevel, tt.wantLevel)
			}
			if ops.isSettingLevel != tt.wantSetting {
				t.Errorf("isSettingLevel = %v, want %v", ops.isSettingLevel, tt.wantSetting)
			}
			if ops.reset != tt.wantReset {
				t.Errorf("reset = %v, want %v", ops.reset, tt.wantReset)
			}
		})
	}
}
