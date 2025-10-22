package cmd

import (
	"context"
	"fmt"
	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/liggitt/tabwriter"
	"os"
	"sort"
	"strings"
)

func (o *loggerCommandOperations) runGetLogger(ctx context.Context) error {
	size := len(o.pods)
	for i, pod := range o.pods {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if size > 1 {
			fmt.Printf("%s:\n", pod)
		}

		err := o.printLoggerForPod(ctx, pod)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		if i != size-1 {
			fmt.Println()
		}
	}

	return nil
}

func (o *loggerCommandOperations) printLoggerForPod(ctx context.Context, podName string) error {
	client, err := actuator.NewActuatorClient(ctx, o.transportFactory, o.k8sClient, podName)
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

	printer := tabwriter.NewWriter(os.Stdout, 6, 4, 3, ' ', 0)

	_, err = fmt.Fprintln(printer, "LOGGER\tLEVEL")
	if err != nil {
		return err
	}
	var skippedFiltered = 0
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

		_, err = fmt.Fprintf(printer, "%v\t%v\n", logger.Name, level)
		if err != nil {
			return err
		}
	}
	err = printer.Flush()
	if err != nil {
		return err
	}

	if skippedFiltered > 0 {
		fmt.Println(skippedFiltered, "non-matching loggers omitted")
	}

	return nil
}
