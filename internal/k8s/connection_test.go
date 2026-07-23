package k8s

import (
	"context"
	"slices"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func pod(name, namespace string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}}
}

func deployment(name, namespace string, selector map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: selector}},
	}
}

func newConn(namespace string, objs ...runtime.Object) *Connection {
	return &Connection{clientset: fake.NewClientset(objs...), namespace: namespace}
}

// sameNames asserts got and want hold the same names. The list wrappers
// return names in whatever order the API server yields, so order is not
// part of their contract.
func sameNames(t *testing.T, got, want []string) {
	t.Helper()
	got = slices.Clone(got)
	want = slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetPod(t *testing.T) {
	tests := []struct {
		name      string
		pods      []runtime.Object
		namespace string
		podName   string
		wantErr   bool
	}{
		{
			name:      "existing pod",
			pods:      []runtime.Object{pod("test-pod", "default", nil)},
			namespace: "default",
			podName:   "test-pod",
		},
		{
			name:      "non-existent pod",
			namespace: "default",
			podName:   "missing-pod",
			wantErr:   true,
		},
		{
			name:      "pod from specific namespace",
			pods:      []runtime.Object{pod("app-pod", "production", nil), pod("app-pod", "staging", nil)},
			namespace: "production",
			podName:   "app-pod",
		},
		{
			name:      "pod from wrong namespace",
			pods:      []runtime.Object{pod("app-pod", "production", nil)},
			namespace: "staging",
			podName:   "app-pod",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newConn(tt.namespace, tt.pods...)
			got, err := conn.GetPod(context.Background(), tt.podName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetPod() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Name != tt.podName || got.Namespace != tt.namespace {
				t.Errorf("got pod %s/%s, want %s/%s", got.Namespace, got.Name, tt.namespace, tt.podName)
			}
		})
	}
}

func TestListPods(t *testing.T) {
	tests := []struct {
		name      string
		pods      []runtime.Object
		namespace string
		selector  string
		want      []string
	}{
		{
			name:      "all pods in namespace",
			pods:      []runtime.Object{pod("pod-1", "default", nil), pod("pod-2", "default", nil), pod("pod-3", "other", nil)},
			namespace: "default",
			want:      []string{"pod-1", "pod-2"},
		},
		{
			name: "single label",
			pods: []runtime.Object{
				pod("app-1", "default", map[string]string{"app": "myapp"}),
				pod("app-2", "default", map[string]string{"app": "myapp"}),
				pod("app-3", "default", map[string]string{"app": "other"}),
			},
			namespace: "default",
			selector:  "app=myapp",
			want:      []string{"app-1", "app-2"},
		},
		{
			name: "multiple labels",
			pods: []runtime.Object{
				pod("pod-1", "default", map[string]string{"app": "myapp", "env": "prod"}),
				pod("pod-2", "default", map[string]string{"app": "myapp", "env": "staging"}),
				pod("pod-3", "default", map[string]string{"app": "other", "env": "prod"}),
			},
			namespace: "default",
			selector:  "app=myapp,env=prod",
			want:      []string{"pod-1"},
		},
		{
			name:      "namespace with no pods",
			namespace: "default",
		},
		{
			name:      "label selector with no matches",
			pods:      []runtime.Object{pod("pod-1", "default", map[string]string{"app": "myapp"})},
			namespace: "default",
			selector:  "app=nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newConn(tt.namespace, tt.pods...)
			got, err := conn.ListPods(context.Background(), tt.selector)
			if err != nil {
				t.Fatalf("ListPods() error = %v", err)
			}
			sameNames(t, got, tt.want)
		})
	}
}

func TestListDeployments(t *testing.T) {
	tests := []struct {
		name        string
		deployments []runtime.Object
		namespace   string
		want        []string
	}{
		{
			name:        "all deployments",
			deployments: []runtime.Object{deployment("app-1", "default", nil), deployment("app-2", "default", nil)},
			namespace:   "default",
			want:        []string{"app-1", "app-2"},
		},
		{
			name:        "deployments in specific namespace",
			deployments: []runtime.Object{deployment("prod-app", "production", nil), deployment("staging-app", "staging", nil)},
			namespace:   "production",
			want:        []string{"prod-app"},
		},
		{
			name:      "namespace with no deployments",
			namespace: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newConn(tt.namespace, tt.deployments...)
			got, err := conn.ListDeployments(context.Background())
			if err != nil {
				t.Fatalf("ListDeployments() error = %v", err)
			}
			sameNames(t, got, tt.want)
		})
	}
}

func TestGetDeploymentPods(t *testing.T) {
	tests := []struct {
		name           string
		objs           []runtime.Object
		deploymentName string
		wantErr        bool
		want           []string
	}{
		{
			name: "deployment with multiple pods",
			objs: []runtime.Object{
				deployment("my-app", "default", map[string]string{"app": "my-app"}),
				pod("my-app-abc123", "default", map[string]string{"app": "my-app"}),
				pod("my-app-def456", "default", map[string]string{"app": "my-app"}),
				pod("other-app-xyz789", "default", map[string]string{"app": "other-app"}),
			},
			deploymentName: "my-app",
			want:           []string{"my-app-abc123", "my-app-def456"},
		},
		{
			name:           "deployment with no pods",
			objs:           []runtime.Object{deployment("empty-app", "default", map[string]string{"app": "empty-app"})},
			deploymentName: "empty-app",
		},
		{
			name:           "non-existent deployment",
			deploymentName: "missing-deployment",
			wantErr:        true,
		},
		{
			name: "deployment with complex label selector",
			objs: []runtime.Object{
				deployment("complex-app", "default", map[string]string{"app": "complex-app", "tier": "backend"}),
				pod("complex-app-1", "default", map[string]string{"app": "complex-app", "tier": "backend"}),
				pod("complex-app-2", "default", map[string]string{"app": "complex-app"}),
			},
			deploymentName: "complex-app",
			want:           []string{"complex-app-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := newConn("default", tt.objs...)
			got, err := conn.GetDeploymentPods(context.Background(), tt.deploymentName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GetDeploymentPods() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			sameNames(t, got, tt.want)
		})
	}
}
