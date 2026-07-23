package cmd

import (
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

func TestSortLoggersRootFirst(t *testing.T) {
	loggers := []actuator.LoggerConfiguration{
		{Name: "com.example"},
		{Name: "AUDIT"}, // sorts before "ROOT" alphabetically
		{Name: "org.springframework"},
		{Name: "ROOT"},
	}

	sortLoggers(loggers)

	want := []string{"ROOT", "AUDIT", "com.example", "org.springframework"}
	got := make([]string, len(loggers))
	for i, logger := range loggers {
		got[i] = logger.Name
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wrong order at position %d: got %v, want %v", i, got, want)
		}
	}
}
