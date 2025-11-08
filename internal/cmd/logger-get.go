package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

func (o *loggerCommandOperations) runForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

	loggers, err := client.GetLoggers()
	if err != nil {
		return err
	}

	sort.Slice(loggers, func(i, j int) bool {
		// Make sure the ROOT logger is always first
		if loggers[j].Name == "ROOT" {
			return false
		}

		return strings.Compare(loggers[i].Name, loggers[j].Name) < 0
	})

	w := newTableWriter()
	defer func() { _ = w.Flush() }()

	_, _ = fmt.Fprintln(w, "LOGGER\tLEVEL")
	skippedFiltered := 0
	for _, logger := range loggers {
		level := ""

		if logger.EffectiveLevel != nil {
			level = *logger.EffectiveLevel + " (effective)"
		}

		if logger.ConfiguredLevel != nil {
			level = *logger.ConfiguredLevel
		}

		if logger.ConfiguredLevel == nil && !o.showAllLoggers && logger.Name != o.loggerName {
			continue
		}

		if o.loggerName != "" && !strings.HasPrefix(logger.Name, o.loggerName) {
			skippedFiltered++
			continue
		}

		_, _ = fmt.Fprintf(w, "%v\t%v\n", logger.Name, level)
	}

	if skippedFiltered > 0 {
		defer fmt.Println(skippedFiltered, "non-matching loggers omitted")
	}

	return nil
}
