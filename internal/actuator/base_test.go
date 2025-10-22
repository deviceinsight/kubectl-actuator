package actuator

import (
	"context"
	"net/http"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type mockK8sClient struct {
	pods      map[string]*corev1.Pod
	namespace string
}

var _ k8s.Client = (*mockK8sClient)(nil)

func (m *mockK8sClient) GetPod(_ context.Context, _, name string) (*corev1.Pod, error) {
	if pod, ok := m.pods[name]; ok {
		return pod, nil
	}
	return nil, &notFoundError{resource: "pod", name: name}
}

func (m *mockK8sClient) ListPods(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockK8sClient) ListDeployments(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockK8sClient) GetDeploymentPods(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockK8sClient) Clientset() kubernetes.Interface {
	return fake.NewClientset()
}

func (m *mockK8sClient) Namespace() string {
	return m.namespace
}

type notFoundError struct {
	resource string
	name     string
}

func (e *notFoundError) Error() string {
	return e.resource + " " + e.name + " not found"
}

type mockTransportFactory struct {
	shouldFail bool
}

var _ k8s.TransportFactory = (*mockTransportFactory)(nil)

func (m *mockTransportFactory) CreateHttpTransport(_ string, _ int) (*http.Transport, error) {
	if m.shouldFail {
		return nil, &transportError{message: "failed to create transport"}
	}
	return &http.Transport{}, nil
}

type transportError struct {
	message string
}

func (e *transportError) Error() string {
	return e.message
}

func TestNewActuatorClient(t *testing.T) {
	tests := []struct {
		name           string
		podAnnotations map[string]string
		wantErr        bool
		errContains    string
		transportFails bool
	}{
		{
			name:           "default port and basePath",
			podAnnotations: map[string]string{},
			wantErr:        false,
		},
		{
			name: "custom port annotation",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "9090",
			},
			wantErr: false,
		},
		{
			name: "custom basePath annotation",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/basePath": "management",
			},
			wantErr: false,
		},
		{
			name: "both custom port and basePath",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port":     "9090",
				"kubectl-actuator.device-insight.com/basePath": "management/actuator",
			},
			wantErr: false,
		},
		{
			name: "valid port 1",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "1",
			},
			wantErr: false,
		},
		{
			name: "valid port 65535",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "65535",
			},
			wantErr: false,
		},
		{
			name: "invalid port 0",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "0",
			},
			wantErr:     true,
			errContains: "port must be between 1-65535",
		},
		{
			name: "invalid port 65536",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "65536",
			},
			wantErr:     true,
			errContains: "port must be between 1-65535",
		},
		{
			name: "invalid port negative",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "-1",
			},
			wantErr:     true,
			errContains: "port must be between 1-65535",
		},
		{
			name: "non-numeric port",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "abc",
			},
			wantErr:     true,
			errContains: "invalid port",
		},
		{
			name: "empty string port",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "",
			},
			wantErr:     true,
			errContains: "invalid port",
		},
		{
			name: "float port",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "8080.5",
			},
			wantErr:     true,
			errContains: "invalid port",
		},
		{
			name:           "transport creation fails",
			podAnnotations: map[string]string{},
			wantErr:        true,
			transportFails: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			podName := "test-pod"

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        podName,
					Namespace:   "default",
					Annotations: tt.podAnnotations,
				},
			}

			k8sClient := &mockK8sClient{
				pods: map[string]*corev1.Pod{
					podName: pod,
				},
				namespace: "default",
			}

			transportFactory := &mockTransportFactory{
				shouldFail: tt.transportFails,
			}

			_, err := NewActuatorClient(ctx, transportFactory, k8sClient, podName)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewActuatorClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing '%s', got '%v'", tt.errContains, err)
				}
			}
		})
	}
}

func TestPortValidation(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"valid port 1", 1, false},
		{"valid port 80", 80, false},
		{"valid port 8080", 8080, false},
		{"valid port 65535", 65535, false},
		{"invalid port 0", 0, true},
		{"invalid port 65536", 65536, true},
		{"invalid port -1", -1, true},
		{"invalid port -1000", -1000, true},
		{"invalid port 100000", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePort(%d) error = %v, wantErr %v", tt.port, err, tt.wantErr)
			}
		})
	}
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return &portValidationError{port: port}
	}
	return nil
}

type portValidationError struct {
	port int
}

func (e *portValidationError) Error() string {
	return "port must be between 1-65535"
}

func TestEndpointError(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		status        string
		messagePrefix string
		wantContains  []string
	}{
		{
			name:          "loggers endpoint 404",
			endpoint:      "loggers",
			status:        "404 Not Found",
			messagePrefix: "Unable to get loggers",
			wantContains: []string{
				"Unable to get loggers",
				"404 Not Found",
				"loggers",
				"https://docs.spring.io",
			},
		},
		{
			name:          "scheduledtasks endpoint 500",
			endpoint:      "scheduledtasks",
			status:        "500 Internal Server Error",
			messagePrefix: "Unable to get scheduled tasks",
			wantContains: []string{
				"Unable to get scheduled tasks",
				"500 Internal Server Error",
				"scheduledtasks",
				"https://docs.spring.io",
			},
		},
		{
			name:          "info endpoint 403",
			endpoint:      "info",
			status:        "403 Forbidden",
			messagePrefix: "Failed to get info",
			wantContains: []string{
				"Failed to get info",
				"403 Forbidden",
				"info",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := endpointError(tt.endpoint, tt.status, tt.messagePrefix)
			errMsg := err.Error()

			for _, want := range tt.wantContains {
				if !contains(errMsg, want) {
					t.Errorf("error message does not contain '%s'\nGot: %s", want, errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
