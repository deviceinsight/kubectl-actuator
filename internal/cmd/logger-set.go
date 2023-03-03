package cmd

import (
	"fmt"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/internal/acuator"
)

func (o *loggerCommandOperations) runSetLogger() error {
	size := len(o.pods)
	for i, pod := range o.pods {
		if size > 1 {
			fmt.Println(pod + ": ")
		}

		err := o.setLoggerForPod(pod)
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

func (o *loggerCommandOperations) setLoggerForPod(podName string) error {
	actuator, err := acuator.NewActuatorClient(o.connection, podName)
	if err != nil {
		return err
	}

	err = actuator.SetLoggerLevel(o.loggerName, o.targetLevel)
	if err != nil {
		return err
	}

	return nil
}
