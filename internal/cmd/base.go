package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type ActuatorClientFactory struct {
	conn     *k8s.Connection
	port     int
	basePath string
	timeout  time.Duration
}

var _ actuator.ClientFactory = (*ActuatorClientFactory)(nil)

func NewActuatorClientFactory(conn *k8s.Connection, cmd *cobra.Command) *ActuatorClientFactory {
	root := cmd.Root()
	port, _ := root.PersistentFlags().GetInt("port")
	basePath, _ := root.PersistentFlags().GetString("base-path")
	timeout, _ := root.PersistentFlags().GetDuration("timeout")

	return &ActuatorClientFactory{
		conn:     conn,
		port:     port,
		basePath: basePath,
		timeout:  timeout,
	}
}

func (f *ActuatorClientFactory) NewClient(ctx context.Context, podName string) (actuator.Client, error) {
	return actuator.NewClient(ctx, f.conn, podName, f.port, f.basePath, f.timeout)
}

type baseOperations struct {
	k8sCliFlags           *genericclioptions.ConfigFlags
	podResolver           PodResolver
	actuatorClientFactory actuator.ClientFactory
	pods                  []string
	commandName           string
}

// resolvePods connects to the cluster and turns the global target flags into
// the pod list, failing with guidance when nothing was selected. It is the
// front half of every command pipeline.
func (b *baseOperations) resolvePods(cmd *cobra.Command) error {
	b.commandName = cmd.Name()

	connection, err := k8s.NewConnection(b.k8sCliFlags)
	if err != nil {
		return err
	}

	pods, err := b.podResolver(cmd.Context(), connection, cmd)
	if err != nil {
		return err
	}
	b.pods = pods

	b.actuatorClientFactory = NewActuatorClientFactory(connection, cmd)

	return b.validatePods()
}

// runEndpoint executes the pipeline shared by every read command: resolve
// the target pods, then fetch from each one, as a single document for
// -o json/yaml or as per-pod tables otherwise. Commands validate their
// flags before calling this.
func (b *baseOperations) runEndpoint(cmd *cobra.Command, action, output string, structuredForPod PodDataFunc, forPod PodFunc) error {
	if err := b.resolvePods(cmd); err != nil {
		return err
	}
	if isStructuredOutput(output) {
		return b.runStructured(cmd.Context(), output, action, structuredForPod)
	}
	return b.runForEachPod(cmd.Context(), action, forPod)
}

// The no-pods error carries a runnable example because it is the first thing
// every new user sees.
func (b *baseOperations) validatePods() error {
	if len(b.pods) == 0 {
		name := b.commandName
		if name == "" {
			// Only reachable for operations built without resolvePods, i.e.
			// in unit tests; the example must still be runnable.
			name = "health"
		}
		return fmt.Errorf("%w, e.g. 'kubectl actuator -d my-app %s'", ErrNoPodsSelected, name)
	}
	return nil
}

// requireSingleTargetForOutputFile rejects --output-file with multiple pods:
// the downloads would overwrite each other. Unlike the other commands' flag
// validation it runs after pod resolution, because the rule depends on how
// many pods were actually selected.
func (b *baseOperations) requireSingleTargetForOutputFile(outputFile string) error {
	if outputFile != "" && len(b.pods) > 1 {
		return fmt.Errorf("--output-file requires a single target pod, but %d pods are selected", len(b.pods))
	}
	return nil
}

// completionClient builds an actuator client for the first selected pod, for
// use in shell completion functions. Failures are swallowed: completion must
// stay silent when the cluster is unavailable.
func (b *baseOperations) completionClient(cmd *cobra.Command) (actuator.Client, bool) {
	connection, err := k8s.NewConnection(b.k8sCliFlags)
	if err != nil {
		return nil, false
	}
	pods, err := b.podResolver(cmd.Context(), connection, cmd)
	if err != nil || len(pods) == 0 {
		return nil, false
	}
	client, err := NewActuatorClientFactory(connection, cmd).NewClient(cmd.Context(), pods[0])
	if err != nil {
		return nil, false
	}
	return client, true
}
