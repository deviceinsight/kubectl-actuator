package cmd

import (
	"fmt"
	"github.com/liggitt/tabwriter"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/internal/acuator"
	"os"
	"sort"
	"strings"
)

func (o *loggerCommandOperations) runGetLogger() error {
	size := len(o.pods)
	for i, pod := range o.pods {
		if size > 1 {
			fmt.Println(pod + ": ")
		}

		err := o.printLoggerForPod(pod)
		if err != nil {
			fmt.Println("Error: " + err.Error())
		}

		if i != size-1 {
			// Add new line if it is not the last element
			fmt.Println()
		}
	}

	return nil
}

func (o *loggerCommandOperations) printLoggerForPod(podName string) error {
	actuator, err := acuator.NewActuatorClient(o.connection, podName)
	if err != nil {
		return err
	}

	loggers, err := actuator.GetLoggers()
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
