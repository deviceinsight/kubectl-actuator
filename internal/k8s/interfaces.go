package k8s

import (
	"context"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Client interface {
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)
	ListPods(ctx context.Context, namespace, labelSelector string) ([]string, error)
	ListDeployments(ctx context.Context, namespace string) ([]string, error)
	GetDeploymentPods(ctx context.Context, namespace, deploymentName string) ([]string, error)
	Clientset() kubernetes.Interface
	Namespace() string
}

type TransportFactory interface {
	CreateHttpTransport(podName string, podPort int) (*http.Transport, error)
}
