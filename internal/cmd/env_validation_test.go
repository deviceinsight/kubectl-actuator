package cmd

import (
	"strings"
	"testing"
)

func TestEnvValidateFlags(t *testing.T) {
	t.Run("--filter conflicts with a property argument", func(t *testing.T) {
		ops := &envCommandOperations{filter: "spring", propertyName: "spring.datasource.url"}
		err := ops.validateFlags()
		if err == nil || !strings.Contains(err.Error(), "--filter cannot be combined") {
			t.Errorf("expected the conflict error, got %v", err)
		}
	})

	t.Run("filter alone and property alone are valid", func(t *testing.T) {
		for _, ops := range []*envCommandOperations{
			{filter: "spring"},
			{propertyName: "spring.datasource.url"},
		} {
			if err := ops.validateFlags(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}
	})
}
