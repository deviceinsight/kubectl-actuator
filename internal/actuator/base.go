package actuator

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/go-resty/resty/v2"
)

type actuatorClient struct {
	httpClient HTTPClient
}

var _ Client = (*actuatorClient)(nil)

const (
	basePathAnnotation = "kubectl-actuator.device-insight.com/basePath"
	portAnnotation     = "kubectl-actuator.device-insight.com/port"

	defaultPort        = 8080
	defaultBasePath    = "actuator"
	defaultHTTPTimeout = 30 * time.Second
)

func NewActuatorClient(ctx context.Context, transportFactory k8s.TransportFactory, k8sClient k8s.Client, podName string, portOverride int, basePathOverride string) (Client, error) {
	pod, err := k8sClient.GetPod(ctx, k8sClient.Namespace(), podName)
	if err != nil {
		return nil, err
	}

	// Determine basePath: CLI flag > annotation > default
	basePath := basePathOverride
	if basePath == "" {
		basePath = pod.Annotations[basePathAnnotation]
	}
	if basePath == "" {
		basePath = defaultBasePath
	}

	// Determine port: CLI flag > annotation > default
	actuatorPort := portOverride
	if actuatorPort == 0 {
		portStr, hasAnnotation := pod.Annotations[portAnnotation]
		if hasAnnotation {
			actuatorPort, err = strconv.Atoi(portStr)
			if err != nil {
				return nil, fmt.Errorf("invalid port (%s annotation): %w", portAnnotation, err)
			}
		} else {
			actuatorPort = defaultPort
		}
	}

	if actuatorPort < 1 || actuatorPort > 65535 {
		return nil, fmt.Errorf("port must be between 1-65535, got %d", actuatorPort)
	}

	transport, err := transportFactory.CreateHttpTransport(podName, actuatorPort)
	if err != nil {
		return nil, err
	}

	restyClient := resty.New().
		SetTransport(transport).
		SetScheme("http").
		SetBaseURL("http://port-forwarded-actuator/" + basePath).
		SetTimeout(defaultHTTPTimeout)

	httpClient := newRestyHTTPClient(restyClient)
	return &actuatorClient{httpClient: httpClient}, nil
}

func endpointError(endpoint string, status string, messagePrefix string) error {
	return fmt.Errorf("%s: %s\nMake sure the '%s' endpoint is exposed in your Spring Boot configuration: https://docs.spring.io/spring-boot/reference/actuator/endpoints.html", messagePrefix, status, endpoint)
}

func resourceNotFoundError(resourceType string, resourceName string, status string) error {
	return fmt.Errorf("%s '%s' not found: %s", resourceType, resourceName, status)
}

func (c *actuatorClient) isEndpointAccessible(endpoint string) bool {
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return false
	}
	return resp.StatusCode != 404
}
