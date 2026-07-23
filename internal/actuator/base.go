package actuator

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	corev1 "k8s.io/api/core/v1"
)

// actuatorClient keeps the resolved connection settings and their source
// (flag/annotation/default) so error messages can state what was actually
// used.
type actuatorClient struct {
	httpClient     HTTPClient
	port           int
	portSource     string
	basePath       string
	basePathSource string
	timeout        time.Duration
}

var _ Client = (*actuatorClient)(nil)

const (
	basePathAnnotation = "kubectl-actuator.device-insight.com/basePath"
	portAnnotation     = "kubectl-actuator.device-insight.com/port"

	defaultPort     = 8080
	defaultBasePath = "actuator"

	// portForwardBaseURL is a placeholder host; the port-forward transport
	// ignores it and dials the pod directly.
	portForwardBaseURL = "http://port-forwarded-actuator"
)

// NewClient builds a client for one pod's actuator. A timeout of 0 disables
// the per-request limit; streamed downloads are never time-limited.
func NewClient(ctx context.Context, conn PodConnection, podName string, portOverride int, basePathOverride string, timeout time.Duration) (Client, error) {
	pod, err := conn.GetPod(ctx, podName)
	if err != nil {
		return nil, err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("pod %q is not running (phase: %s); actuator commands need a running pod", podName, pod.Status.Phase)
	}

	basePath, basePathSource := resolveBasePath(basePathOverride, pod.Annotations[basePathAnnotation])

	portAnnotationValue, hasPortAnnotation := pod.Annotations[portAnnotation]
	actuatorPort, portSource, err := resolvePort(portOverride, portAnnotationValue, hasPortAnnotation)
	if err != nil {
		return nil, err
	}

	if actuatorPort < 1 || actuatorPort > 65535 {
		return nil, fmt.Errorf("port must be between 1 and 65535, got %d (%s)", actuatorPort, portSource)
	}

	if timeout < 0 {
		return nil, fmt.Errorf("invalid --timeout %s: must be 0 (no limit) or positive", timeout)
	}

	transport, err := conn.CreateHTTPTransport(podName, actuatorPort)
	if err != nil {
		return nil, err
	}

	restyClient := resty.New().
		SetTransport(transport).
		SetScheme("http").
		SetBaseURL(portForwardBaseURL + "/" + basePath).
		SetTimeout(timeout)

	// The stream client deliberately has no timeout: it would cover reading
	// the entire body and abort long downloads (see restyHTTPClient.stream).
	streamClient := resty.New().
		SetTransport(transport).
		SetScheme("http").
		SetBaseURL(portForwardBaseURL + "/" + basePath)

	httpClient := newRestyHTTPClient(ctx, restyClient, streamClient)
	return &actuatorClient{
		httpClient:     httpClient,
		port:           actuatorPort,
		portSource:     portSource,
		basePath:       basePath,
		basePathSource: basePathSource,
		timeout:        timeout,
	}, nil
}

// resolveBasePath picks the actuator base path (flag > annotation > default)
// and strips surrounding slashes so "/actuator" and "actuator" behave the
// same. An explicit "/" selects the root base path and is returned as "".
// The second return value names the source of the chosen value.
func resolveBasePath(override, annotation string) (string, string) {
	if override != "" {
		return strings.Trim(override, "/"), "flag"
	}
	if annotation != "" {
		return strings.Trim(annotation, "/"), "annotation"
	}
	return defaultBasePath, "default"
}

// resolvePort picks the actuator port (flag > annotation > default). The
// second return value names the source of the chosen value.
func resolvePort(override int, annotation string, hasAnnotation bool) (int, string, error) {
	if override != 0 {
		return override, "flag", nil
	}
	if hasAnnotation {
		port, err := strconv.Atoi(annotation)
		if err != nil {
			return 0, "", fmt.Errorf("invalid port (%s annotation): %w", portAnnotation, err)
		}
		return port, "annotation", nil
	}
	return defaultPort, "default", nil
}

// getAndParse fetches path and decodes the JSON response into target,
// translating error statuses into diagnosed user-facing errors.
func (c *actuatorClient) getAndParse(path, endpointID, errorPrefix string, target any) error {
	resp, err := c.httpClient.Get(path)
	if err != nil {
		return c.connectionError(err)
	}
	if resp.IsErrorStatus() {
		return c.statusError(endpointID, resp.StatusCode, resp.Status, errorPrefix)
	}
	if err := parseJSON(resp.Body, target); err != nil {
		return c.notJSONError(err, errorPrefix)
	}
	return nil
}
