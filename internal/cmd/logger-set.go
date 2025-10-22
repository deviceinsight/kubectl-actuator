package cmd

import (
	"context"
	"fmt"
	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

func (o *loggerCommandOperations) runSetLogger(ctx context.Context) error {
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

		err := o.setLoggerForPod(ctx, pod)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		if i != size-1 {
			fmt.Println()
		}
	}

	return nil
}

func (o *loggerCommandOperations) setLoggerForPod(ctx context.Context, podName string) error {
	client, err := actuator.NewActuatorClient(ctx, o.transportFactory, o.k8sClient, podName)
	if err != nil {
		return err
	}

	err = client.SetLoggerLevel(o.loggerName, o.targetLevel)
	if err != nil {
		return err
	}

	return nil
}
