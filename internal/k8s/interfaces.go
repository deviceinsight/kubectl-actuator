package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// Client provides namespace-scoped access to the Kubernetes resources this
// tool works with. The namespace is fixed at connection time from the
// kubeconfig or the --namespace flag.
type Client interface {
	GetPod(ctx context.Context, name string) (*corev1.Pod, error)
	ListPods(ctx context.Context, labelSelector string) ([]string, error)
	ListNamespaces(ctx context.Context) ([]string, error)
	ListDeployments(ctx context.Context) ([]string, error)
	GetDeploymentPods(ctx context.Context, deploymentName string) ([]string, error)
	Namespace() string
}
