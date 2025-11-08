package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
)

type Connection struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
	restClient *rest.RESTClient
	namespace  string
}

// Ensure Connection implements K8sClient and TransportFactory
var _ Client = (*Connection)(nil)
var _ TransportFactory = (*Connection)(nil)

func NewK8sConnection(options *genericclioptions.ConfigFlags) (*Connection, error) {
	restConfig, err := options.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig: %w", err)
	}
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

func (c *Connection) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	return c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Connection) ListPods(ctx context.Context, namespace, labelSelector string) ([]string, error) {
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
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

func (c *Connection) ListDeployments(ctx context.Context, namespace string) ([]string, error) {
	list, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var deploymentNames []string
	for _, deployment := range list.Items {
		deploymentNames = append(deploymentNames, deployment.Name)
	}

	return deploymentNames, nil
}

func (c *Connection) GetDeploymentPods(ctx context.Context, namespace, deploymentName string) ([]string, error) {
	deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}

	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
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

func (c *Connection) Clientset() kubernetes.Interface {
	return c.clientset
}

func (c *Connection) Namespace() string {
	return c.namespace
}
