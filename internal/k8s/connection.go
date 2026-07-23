package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	// register auth providers (oidc, exec, ...) for kubeconfigs that use them
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

type Connection struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
	restClient *rest.RESTClient
	namespace  string
}

var _ Client = (*Connection)(nil)

func NewConnection(options *genericclioptions.ConfigFlags) (*Connection, error) {
	restConfig, err := options.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig: %w", err)
	}
	// RESTClientFor needs the API path, group/version, and codec that
	// ToRESTConfig leaves unset; point it at core/v1 for the
	// pods/*/portforward subresource the port-forward transport posts to.
	restConfig.APIPath = "/api"
	restConfig.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Clientset: %w", err)
	}

	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	namespace, _, err := options.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}

	return &Connection{clientset: clientset, restConfig: restConfig, restClient: restClient, namespace: namespace}, nil
}

// Namespace returns the namespace this connection is scoped to, for error
// messages that state where a lookup came up empty.
func (c *Connection) Namespace() string {
	return c.namespace
}

func (c *Connection) GetPod(ctx context.Context, name string) (*corev1.Pod, error) {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("pod %q not found in namespace %q", name, c.namespace)
		}
		return nil, err
	}
	return pod, nil
}

func (c *Connection) ListPods(ctx context.Context, labelSelector string) ([]string, error) {
	list, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var podNames []string
	for _, pod := range list.Items {
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}

func (c *Connection) ListNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaceNames []string
	for _, namespace := range list.Items {
		namespaceNames = append(namespaceNames, namespace.Name)
	}

	return namespaceNames, nil
}

func (c *Connection) ListDeployments(ctx context.Context) ([]string, error) {
	list, err := c.clientset.AppsV1().Deployments(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var deploymentNames []string
	for _, deployment := range list.Items {
		deploymentNames = append(deploymentNames, deployment.Name)
	}

	return deploymentNames, nil
}

func (c *Connection) GetDeploymentPods(ctx context.Context, deploymentName string) ([]string, error) {
	deployment, err := c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("deployment %q not found in namespace %q", deploymentName, c.namespace)
		}
		return nil, err
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}

	podList, err := c.clientset.CoreV1().Pods(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}

	var podNames []string
	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}
