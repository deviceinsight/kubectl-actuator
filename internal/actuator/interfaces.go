package actuator

import (
	"context"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
)

// PodConnection provides what the actuator client needs from Kubernetes:
// the pod's metadata (for configuration annotations) and an HTTP transport
// that port-forwards to it. *k8s.Connection satisfies this.
type PodConnection interface {
	GetPod(ctx context.Context, name string) (*corev1.Pod, error)
	CreateHTTPTransport(podName string, podPort int) (*http.Transport, error)
}

// Client is one pod's Spring Boot Actuator.
type Client interface {
	// Discovery.
	GetAvailableEndpoints() ([]string, error)

	// Whole-endpoint reads.
	GetBeans() (*BeansResponse, error)
	GetEnv() (*EnvResponse, error)
	GetHealth(group string) (*HealthResponse, error)
	GetInfo() (map[string]any, error)
	GetLoggers() (*LoggersResponse, error)
	GetMetrics() (*MetricsListResponse, error)
	GetScheduledTasks() (*ScheduledTasksResponse, error)
	GetThreadDump() (*ThreadDumpResponse, error)

	// Single-resource drill-downs.
	GetEnvProperty(propertyName string) (*EnvPropertyResponse, error)
	GetMetric(metricName string, tags []string) (*MetricResponse, error)

	// Verbatim passthrough for -o json/yaml and the raw command.
	GetHealthRaw(group string) ([]byte, error)
	GetRaw(endpoint string) ([]byte, error)

	// Mutation.
	SetLoggerLevel(logger string, level string) error

	// Streamed downloads; never time-limited.
	DownloadHeapDump(writer io.Writer, live *bool) (int64, error)
	DownloadLogFile(writer io.Writer, tailBytes int64) (int64, error)
}

type HTTPClient interface {
	Get(path string) (*Response, error)
	Post(path string, body any) (*Response, error)
	Stream(path string, headers map[string]string) (*StreamResponse, error)
}

// ClientFactory creates an actuator client for a pod; the cmd layer's
// factory implements it with pre-resolved flag overrides.
type ClientFactory interface {
	NewClient(ctx context.Context, podName string) (Client, error)
}
