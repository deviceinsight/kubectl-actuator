package acuator

import (
	"context"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"gitlab.device-insight.com/mwa/kubectl-actuator-plugin/internal/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

type ActuatorClient struct {
	resty *resty.Client
}

var basePathAnnotation = "kubectl-actuator.device-insight.com/basePath"
var portAnnotation = "kubectl-actuator.device-insight.com/port"

func NewActuatorClient(connection *k8s.Connection, podName string) (*ActuatorClient, error) {
	podsClient := connection.Clientset.CoreV1().Pods(connection.Namespace)
	pod, err := podsClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	basePath, ok := pod.Annotations[basePathAnnotation]
	if !ok {
		basePath = "actuator"
	}

	actuatorPortStr, ok := pod.Annotations[portAnnotation]
	if !ok {
		actuatorPortStr = "9090"
	}

	actuatorPort, err := strconv.Atoi(actuatorPortStr)
	if err != nil {
		return nil, errors.WithMessagef(err, "Invalid port (%s annotation)", portAnnotation)
	}

	transport, err := connection.CreateHttpTransport(podName, actuatorPort)
	if err != nil {
		return nil, err
	}

	client := resty.New().
		SetTransport(transport).
		SetScheme("http").
		SetBaseURL("http://port-forwarded-actuator/" + basePath)

	return &ActuatorClient{resty: client}, nil
}
