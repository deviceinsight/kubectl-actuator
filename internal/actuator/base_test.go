package actuator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockPodConnection struct {
	pods           map[string]*corev1.Pod
	transportFails bool
}

var _ PodConnection = (*mockPodConnection)(nil)

func (m *mockPodConnection) GetPod(_ context.Context, name string) (*corev1.Pod, error) {
	if pod, ok := m.pods[name]; ok {
		return pod, nil
	}
	return nil, fmt.Errorf("pod %q not found", name)
}

func (m *mockPodConnection) CreateHTTPTransport(_ string, _ int) (*http.Transport, error) {
	if m.transportFails {
		return nil, errors.New("failed to create transport")
	}
	return &http.Transport{}, nil
}

// connWithRunningPod builds a connection serving one running pod named
// "test-pod" with the given annotations.
func connWithRunningPod(annotations map[string]string) *mockPodConnection {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Namespace:   "default",
			Annotations: annotations,
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	return &mockPodConnection{pods: map[string]*corev1.Pod{"test-pod": pod}}
}

func TestNewClient(t *testing.T) {
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
			errContains: "port must be between 1 and 65535",
		},
		{
			name: "invalid port 65536",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "65536",
			},
			wantErr:     true,
			errContains: "port must be between 1 and 65535",
		},
		{
			name: "invalid port negative",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "-1",
			},
			wantErr:     true,
			errContains: "port must be between 1 and 65535",
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
			conn := connWithRunningPod(tt.podAnnotations)
			conn.transportFails = tt.transportFails

			_, err := NewClient(context.Background(), conn, "test-pod", 0, "", 0)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err)
				}
			}
		})
	}
}

func TestNewClientRejectsNotRunningPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-pod", Namespace: "default"},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
	conn := &mockPodConnection{pods: map[string]*corev1.Pod{"pending-pod": pod}}

	_, err := NewClient(context.Background(), conn, "pending-pod", 0, "", 0)
	if err == nil {
		t.Fatal("expected error for pending pod, got nil")
	}
	want := `pod "pending-pod" is not running (phase: Pending)`
	if !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v, want containing %q", err, want)
	}
}

func TestNewClientRejectsNegativeTimeout(t *testing.T) {
	conn := connWithRunningPod(nil)

	_, err := NewClient(context.Background(), conn, "test-pod", 0, "", -time.Second)
	if err == nil || !strings.Contains(err.Error(), "--timeout") {
		t.Errorf("expected a --timeout error, got: %v", err)
	}
}

func TestStatusErrorDiagnosis(t *testing.T) {
	index := `{"_links": {"self": {"href": "/actuator"}, "health": {"href": "/actuator/health"}}}`

	newClient := func(indexStatus int, indexBody string) *actuatorClient {
		return &actuatorClient{
			httpClient: &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					if path == "" {
						return &Response{Body: []byte(indexBody), StatusCode: indexStatus, Status: "index"}, nil
					}
					return &Response{StatusCode: 404, Status: "404"}, nil
				},
			},
			port:           8081,
			portSource:     "annotation",
			basePath:       "management",
			basePathSource: "flag",
		}
	}

	t.Run("404 with endpoint missing from index gives activation guidance", func(t *testing.T) {
		err := newClient(200, index).statusError("beans", 404, "404 Not Found", "failed to get beans")
		for _, want := range []string{"not exposed", "management.endpoints.web.exposure.include", "https://docs.spring.io"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should contain %q\nGot: %s", want, err)
			}
		}
	})

	t.Run("heapdump guidance includes access property", func(t *testing.T) {
		err := newClient(200, index).statusError("heapdump", 404, "404", "failed to get heap dump")
		if !strings.Contains(err.Error(), "management.endpoint.heapdump.access") {
			t.Errorf("heapdump guidance missing access property, got: %s", err)
		}
	})

	t.Run("logfile guidance includes logging.file", func(t *testing.T) {
		err := newClient(200, index).statusError("logfile", 404, "404", "failed to get log file")
		if !strings.Contains(err.Error(), "logging.file.name") {
			t.Errorf("logfile guidance missing logging.file.name, got: %s", err)
		}
	})

	t.Run("404 with unreachable index blames configuration not exposure", func(t *testing.T) {
		err := newClient(404, "").statusError("health", 404, "404 Not Found", "failed to get health")
		for _, want := range []string{"no Spring Boot Actuator found", "port 8081 (annotation)", `base path "management" (flag)`, "--port", "--base-path"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should contain %q\nGot: %s", want, err)
			}
		}
		if strings.Contains(err.Error(), "not exposed by this application") {
			t.Errorf("misconfiguration must not be reported as a disabled endpoint, got: %s", err)
		}
	})

	t.Run("unreachable index hints at default base path when it responds", func(t *testing.T) {
		client := &actuatorClient{
			httpClient: &MockHTTPClient{
				GetFunc: func(path string) (*Response, error) {
					if path == portForwardBaseURL+"/actuator" {
						return &Response{Body: []byte(index), StatusCode: 200, Status: "200"}, nil
					}
					return &Response{StatusCode: 404, Status: "404"}, nil
				},
			},
			port: 8080, portSource: "default", basePath: "management", basePathSource: "flag",
		}
		err := client.statusError("health", 404, "404", "failed to get health")
		if !strings.Contains(err.Error(), `Hint: an actuator index responds at the default base path "actuator"`) {
			t.Errorf("expected default base path hint, got: %s", err)
		}
	})

	t.Run("404 with endpoint present in index is not blamed on exposure", func(t *testing.T) {
		err := newClient(200, index).statusError("health", 404, "404 Not Found", "failed to get health")
		if !strings.Contains(err.Error(), `although the "health" endpoint is exposed`) {
			t.Errorf("unexpected message: %s", err)
		}
	})

	t.Run("401 reports security not exposure", func(t *testing.T) {
		err := newClient(200, index).statusError("health", 401, "401 Unauthorized", "failed to get health")
		if !strings.Contains(err.Error(), "secured") || strings.Contains(err.Error(), "not exposed") {
			t.Errorf("unexpected message: %s", err)
		}
	})

	t.Run("500 reports server error not exposure", func(t *testing.T) {
		err := newClient(200, index).statusError("health", 500, "500 Internal Server Error", "failed to get health")
		if !strings.Contains(err.Error(), "server error") || strings.Contains(err.Error(), "exposed") {
			t.Errorf("unexpected message: %s", err)
		}
	})

	t.Run("connection errors point at the port", func(t *testing.T) {
		client := &actuatorClient{port: 9999, portSource: "flag"}
		err := client.connectionError(fmt.Errorf("dial refused"))
		for _, want := range []string{"port 9999 (flag)", "dial refused", "--port"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should contain %q\nGot: %s", want, err)
			}
		}
	})

	t.Run("canceled context is not reported as a connection problem", func(t *testing.T) {
		client := &actuatorClient{port: 8080, portSource: "default"}
		err := client.connectionError(fmt.Errorf("Get \"http://x/health\": %w", context.Canceled))
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
		if strings.Contains(err.Error(), "--port") {
			t.Errorf("cancellation must not carry port guidance, got: %s", err)
		}
	})

	t.Run("root base path is displayed as / in diagnosis", func(t *testing.T) {
		client := &actuatorClient{
			httpClient: &MockHTTPClient{GetFunc: func(path string) (*Response, error) {
				return &Response{StatusCode: 404, Status: "404"}, nil
			}},
			port: 8080, portSource: "default", basePath: "", basePathSource: "flag",
		}
		err := client.statusError("health", 404, "404", "failed to get health")
		if !strings.Contains(err.Error(), `base path "/" (flag)`) {
			t.Errorf("root base path should be shown as '/', got: %s", err)
		}
	})
}

func TestNonJSONResponseDiagnosis(t *testing.T) {
	client := &actuatorClient{
		httpClient: &MockHTTPClient{GetFunc: func(path string) (*Response, error) {
			return &Response{Body: []byte("<!doctype html><h1>App</h1>"), StatusCode: 200, Status: "200 OK"}, nil
		}},
		port: 8080, portSource: "default", basePath: "actuator", basePathSource: "default",
	}
	_, err := client.GetHealth("")
	if err == nil {
		t.Fatal("expected an error for a 200 response that is not JSON")
	}
	for _, want := range []string{"failed to get health", "not valid JSON", "port 8080 (default)", `base path "actuator" (default)`, "--base-path"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should contain %q\nGot: %s", want, err)
		}
	}
}

func TestResourceNotFoundError(t *testing.T) {
	err := resourceNotFoundError("metric", "jvm.memory.unknown", "404 Not Found")
	want := `metric "jvm.memory.unknown" not found: 404 Not Found`
	if err.Error() != want {
		t.Errorf("resourceNotFoundError() = %q, want %q", err, want)
	}
}

func TestNewClientWithOverrides(t *testing.T) {
	tests := []struct {
		name             string
		podAnnotations   map[string]string
		portOverride     int
		basePathOverride string
		wantErr          bool
		wantPort         int
		wantBasePath     string
	}{
		{
			name: "port override takes precedence over annotation",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port": "9090",
			},
			portOverride: 8888,
			wantPort:     8888,
			wantBasePath: "actuator",
		},
		{
			name: "basePath override takes precedence over annotation",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/basePath": "management",
			},
			basePathOverride: "custom/path",
			wantPort:         8080,
			wantBasePath:     "custom/path",
		},
		{
			name: "both overrides take precedence",
			podAnnotations: map[string]string{
				"kubectl-actuator.device-insight.com/port":     "9090",
				"kubectl-actuator.device-insight.com/basePath": "management",
			},
			portOverride:     7777,
			basePathOverride: "override/path",
			wantPort:         7777,
			wantBasePath:     "override/path",
		},
		{
			name:         "override with invalid port",
			portOverride: 99999,
			wantErr:      true,
		},
		{
			name:         "override with valid edge case port 1",
			portOverride: 1,
			wantPort:     1,
			wantBasePath: "actuator",
		},
		{
			name:         "override with valid edge case port 65535",
			portOverride: 65535,
			wantPort:     65535,
			wantBasePath: "actuator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := connWithRunningPod(tt.podAnnotations)

			client, err := NewClient(context.Background(), conn, "test-pod", tt.portOverride, tt.basePathOverride, 0)

			if (err != nil) != tt.wantErr {
				t.Fatalf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			resolved := client.(*actuatorClient)
			if resolved.port != tt.wantPort {
				t.Errorf("resolved port = %d, want %d", resolved.port, tt.wantPort)
			}
			if resolved.basePath != tt.wantBasePath {
				t.Errorf("resolved basePath = %q, want %q", resolved.basePath, tt.wantBasePath)
			}
		})
	}
}

func TestResolveBasePath(t *testing.T) {
	tests := []struct {
		name       string
		override   string
		annotation string
		want       string
	}{
		{"default", "", "", "actuator"},
		{"override plain", "management", "", "management"},
		{"override with leading slash", "/actuator", "", "actuator"},
		{"override with trailing slash", "management/", "", "management"},
		{"override with both slashes", "/internal/actuator/", "", "internal/actuator"},
		{"annotation with leading slash", "", "/management", "management"},
		{"override beats annotation", "flag", "annotation", "flag"},
		{"explicit slash selects the root base path", "/", "", ""},
		{"root override beats annotation", "/", "management", ""},
		{"annotation slash selects the root base path", "", "/", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := resolveBasePath(tt.override, tt.annotation); got != tt.want {
				t.Errorf("resolveBasePath(%q, %q) = %q, want %q", tt.override, tt.annotation, got, tt.want)
			}
		})
	}
}
