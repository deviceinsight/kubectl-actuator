package actuator

import (
	"context"
	"fmt"
	"strconv"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/go-resty/resty/v2"
)

type actuatorClient struct {
	httpClient HTTPClient
}

var _ Client = (*actuatorClient)(nil)

var basePathAnnotation = "kubectl-actuator.device-insight.com/basePath"
var portAnnotation = "kubectl-actuator.device-insight.com/port"

func NewActuatorClient(ctx context.Context, transportFactory k8s.TransportFactory, k8sClient k8s.Client, podName string) (Client, error) {
	pod, err := k8sClient.GetPod(ctx, k8sClient.Namespace(), podName)
	if err != nil {
		return nil, err
	}

	basePath, ok := pod.Annotations[basePathAnnotation]
	if !ok {
		basePath = "actuator"
	}

	actuatorPortStr, ok := pod.Annotations[portAnnotation]
	if !ok {
		actuatorPortStr = "8080"
	}

	actuatorPort, err := strconv.Atoi(actuatorPortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port (%s annotation): %w", portAnnotation, err)
	}
	if actuatorPort < 1 || actuatorPort > 65535 {
		return nil, fmt.Errorf("port must be between 1-65535, got %d (%s annotation)", actuatorPort, portAnnotation)
	}

	transport, err := transportFactory.CreateHttpTransport(podName, actuatorPort)
	if err != nil {
		return nil, err
	}

	restyClient := resty.New().
		SetTransport(transport).
		SetScheme("http").
		SetBaseURL("http://port-forwarded-actuator/" + basePath)

	httpClient := newRestyHTTPClient(restyClient)
	return &actuatorClient{httpClient: httpClient}, nil
}

func endpointError(endpoint string, status string, messagePrefix string) error {
	return fmt.Errorf("%s: %s\nMake sure the '%s' endpoint is exposed in your Spring Boot configuration: https://docs.spring.io/spring-boot/reference/actuator/endpoints.html", messagePrefix, status, endpoint)
}
