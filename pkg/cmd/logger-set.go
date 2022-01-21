package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/pkg/acuator"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"strings"
)

type LoggerSetOptions struct {
	configFlags *genericclioptions.ConfigFlags
	clientset   *kubernetes.Clientset
	restConfig  *rest.Config
	restClient  *rest.RESTClient

	namespace string
	pods      []string
	logger    string
	newLevel  *string
}

func NewLoggerSetOptions(configFlags *genericclioptions.ConfigFlags) *LoggerSetOptions {
	return &LoggerSetOptions{configFlags: configFlags}
}

func NewLoggerSetCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	options := NewLoggerSetOptions(configFlags)

	cmd := &cobra.Command{
		Use:   "set --pod=my-pod [--pod=other-pod ...] LOGGER LEVEL",
		Short: "Set the level of a logger",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Complete(args); err != nil {
				return err
			}

			if err := options.Validate(); err != nil {
				return err
			}

			cmd.SilenceUsage = true
			if err := options.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&options.pods, "pod", []string{}, "Comma separated list of pods")

	return cmd
}

func (options *LoggerSetOptions) Complete(args []string) error {
	restConfig, err := options.configFlags.ToRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to read kubeconfig")
	}
	restConfig.APIPath = "/api"
	restConfig.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create clientset")
	}

	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return err
	}

	namespace, _, err := options.configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if len(args) < 2 {
		return errors.New("Missing LOGGER and/or LEVEL argument")
	}

	if len(args) > 2 {
		return errors.New("Too many arguments")
	}

	options.logger = args[0]
	options.newLevel = &args[1]
	if strings.EqualFold(*options.newLevel, "NULL") {
		options.newLevel = nil
	}

	options.clientset = clientset
	options.namespace = namespace
	options.restClient = restClient
	options.restConfig = restConfig
	return nil
}

func (options *LoggerSetOptions) Validate() error {
	if len(options.pods) == 0 {
		return errors.New("No pods specified. Please specify at least one pod")
	}

	// TODO: Validate level (and logger?)

	return nil
}

func (options *LoggerSetOptions) Run() error {
	size := len(options.pods)
	for i, pod := range options.pods {
		if size > 1 {
			fmt.Println(pod + ": ")
		}

		err := options.runForPod(pod)
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

func (options *LoggerSetOptions) runForPod(podName string) error {
	podsClient := options.clientset.CoreV1().Pods(options.namespace)
	pod, err := podsClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	transport, err := util.CreateHttpTransport(pod, options.restClient, options.restConfig)
	if err != nil {
		return err
	}

	actuator := acuator.BuildClient(transport, "actuator")
	err = actuator.SetLoggerLevel(options.logger, options.newLevel)
	if err != nil {
		return err
	}

	_ = actuator

	return nil
}
