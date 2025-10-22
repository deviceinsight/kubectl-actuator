package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type infoCommandOperations struct {
	k8sCliFlags      *genericclioptions.ConfigFlags
	k8sClient        k8s.Client
	transportFactory k8s.TransportFactory
	podResolver      PodResolver

	pods []string
}

func NewInfoCommand(configFlags *genericclioptions.ConfigFlags, podResolver PodResolver) *cobra.Command {
	operations := &infoCommandOperations{k8sCliFlags: configFlags, podResolver: podResolver}

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Get application info",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := operations.complete(cmd)
			if err != nil {
				return err
			}

			err = operations.validate()
			if err != nil {
				return err
			}

			err = operations.runGetInfo(cmd.Context())
			if err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func (o *infoCommandOperations) complete(cmd *cobra.Command) error {
	connection, err := k8s.NewK8sConnection(o.k8sCliFlags)
	if err != nil {
		return err
	}
	o.k8sClient = connection
	o.transportFactory = connection

	pods, err := o.podResolver(cmd.Context(), connection, cmd)
	if err != nil {
		return err
	}
	o.pods = pods

	return nil
}

func (o *infoCommandOperations) validate() error {
	if len(o.pods) == 0 {
		return errors.New("No pods specified. Please specify at least one pod")
	}

	return nil
}

func (o *infoCommandOperations) runGetInfo(ctx context.Context) error {
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

		err := o.printInfoForPod(ctx, pod)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		if i != size-1 {
			fmt.Println()
		}
	}

	return nil
}

func (o *infoCommandOperations) printInfoForPod(ctx context.Context, podName string) error {
	client, err := actuator.NewActuatorClient(ctx, o.transportFactory, o.k8sClient, podName)
	if err != nil {
		return err
	}

	info, err := client.GetInfo()
	if err != nil {
		return err
	}

	jsonOutput, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonOutput))

	return nil
}
