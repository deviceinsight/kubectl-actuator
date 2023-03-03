package k8s

import (
	"context"
	"github.com/pkg/errors"
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
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
	RestClient *rest.RESTClient
	Namespace  string
}

func NewK8sConnection(options *genericclioptions.ConfigFlags) (*Connection, error) {
	restConfig, err := options.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read kubeconfig")
	}
	restConfig.APIPath = "/api"
	restConfig.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	restConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Clientset")
	}

	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	namespace, _, err := options.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, err
	}

	return &Connection{Clientset: clientset, RestConfig: restConfig, RestClient: restClient, Namespace: namespace}, nil
}

func (c Connection) ListPods() ([]string, error) {
	list, err := c.Clientset.CoreV1().Pods(c.Namespace).List(context.Background(), metav1.ListOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}

	var foundResources []string
	for _, pod := range list.Items {
		foundResources = append(foundResources, pod.Name)
	}

	return foundResources, nil
}

func (c Connection) ListDeployments() ([]string, error) {
	list, err := c.Clientset.AppsV1().Deployments(c.Namespace).List(context.Background(), metav1.ListOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}

	var foundResources []string
	for _, pod := range list.Items {
		foundResources = append(foundResources, pod.Name)
	}

	return foundResources, nil
}
