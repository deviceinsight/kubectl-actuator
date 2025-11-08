package cmd

import (
	"context"
	"fmt"
)

func (o *loggerCommandOperations) runSetForPod(ctx context.Context, podName string) error {
	client, err := o.actuatorClientFactory.NewClient(ctx, podName)
	if err != nil {
		return err
	}

	err = client.SetLoggerLevel(o.loggerName, o.targetLevel)
	if err != nil {
		return err
	}

	if o.targetLevel == "" {
		fmt.Printf("Logger '%s' reset to default\n", o.loggerName)
	} else {
		fmt.Printf("Logger '%s' set to %s\n", o.loggerName, o.targetLevel)
	}

	return nil
}
