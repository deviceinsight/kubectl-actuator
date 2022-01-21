package cmd

import (
	"context"
	"fmt"
	"github.com/liggitt/tabwriter"
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
	"os"
	"sort"
	"strings"
)

type LoggerGetOptions struct {
	configFlags *genericclioptions.ConfigFlags
	clientset   *kubernetes.Clientset
	restConfig  *rest.Config
	restClient  *rest.RESTClient

	namespace      string
	pods           []string
	showAllLoggers bool
}

func NewLoggerGetOptions(configFlags *genericclioptions.ConfigFlags) *LoggerGetOptions {
	return &LoggerGetOptions{configFlags: configFlags}
}

func NewLoggerGetCommand(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	options := NewLoggerGetOptions(configFlags)

	cmd := &cobra.Command{
		Use:   "get --pod=my-pod [--pod=other-pod ...] [--all-loggers]",
		Short: "Display the logger level",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := options.Complete(); err != nil {
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
	cmd.Flags().BoolVar(&options.showAllLoggers, "all-loggers", false, "Show all loggers")

	return cmd
}

func (options *LoggerGetOptions) Complete() error {
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

	options.clientset = clientset
	options.namespace = namespace
	options.restClient = restClient
	options.restConfig = restConfig
	return nil
}

func (options *LoggerGetOptions) Validate() error {
	if len(options.pods) == 0 {
		return errors.New("No pods specified. Please specify at least one pod")
	}

	return nil
}

func (options *LoggerGetOptions) Run() error {
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

func (options *LoggerGetOptions) runForPod(podName string) error {
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
	defer printer.Flush()

	fmt.Fprintln(printer, "LOGGER\tLEVEL")
	for _, logger := range loggers {
		level := ""

		if logger.EffectiveLevel != nil {
			level = *logger.EffectiveLevel + " (effective)"
		}

		if logger.ConfiguredLevel != nil {
			level = *logger.ConfiguredLevel
		}

		if logger.ConfiguredLevel != nil || options.showAllLoggers {
			fmt.Fprintf(printer, "%v\t%v\n", logger.Name, level)
		}
	}

	return nil
}
