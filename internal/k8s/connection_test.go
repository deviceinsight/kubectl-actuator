package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetPod(t *testing.T) {
	tests := []struct {
		name      string
		pods      []*corev1.Pod
		namespace string
		podName   string
		wantErr   bool
	}{
		{
			name: "get existing pod",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
				},
			},
			namespace: "default",
			podName:   "test-pod",
			wantErr:   false,
		},
		{
			name:      "get non-existent pod",
			pods:      []*corev1.Pod{},
			namespace: "default",
			podName:   "missing-pod",
			wantErr:   true,
		},
		{
			name: "get pod from specific namespace",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-pod",
						Namespace: "production",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-pod",
						Namespace: "staging",
					},
				},
			},
			namespace: "production",
			podName:   "app-pod",
			wantErr:   false,
		},
		{
			name: "get pod from wrong namespace",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-pod",
						Namespace: "production",
					},
				},
			},
			namespace: "staging",
			podName:   "app-pod",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake clientset with test pods
			objs := make([]runtime.Object, len(tt.pods))
			for i, pod := range tt.pods {
				objs[i] = pod
			}
			clientset := fake.NewClientset(objs...)

			conn := &Connection{
				clientset: clientset,
				namespace: "default",
			}

			pod, err := conn.GetPod(ctx, tt.namespace, tt.podName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetPod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if pod == nil {
					t.Error("expected pod, got nil")
					return
				}
				if pod.Name != tt.podName {
					t.Errorf("expected pod name %s, got %s", tt.podName, pod.Name)
				}
				if pod.Namespace != tt.namespace {
					t.Errorf("expected namespace %s, got %s", tt.namespace, pod.Namespace)
				}
			}
		})
	}
}

func TestListPods(t *testing.T) {
	tests := []struct {
		name          string
		pods          []*corev1.Pod
		namespace     string
		labelSelector string
		wantCount     int
		wantPodNames  []string
	}{
		{
			name: "list all pods in namespace",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-2",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-3",
						Namespace: "other",
					},
				},
			},
			namespace:     "default",
			labelSelector: "",
			wantCount:     2,
			wantPodNames:  []string{"pod-1", "pod-2"},
		},
		{
			name: "list pods with single label",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-1",
						Namespace: "default",
						Labels:    map[string]string{"app": "myapp"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-2",
						Namespace: "default",
						Labels:    map[string]string{"app": "myapp"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-3",
						Namespace: "default",
						Labels:    map[string]string{"app": "other"},
					},
				},
			},
			namespace:     "default",
			labelSelector: "app=myapp",
			wantCount:     2,
			wantPodNames:  []string{"app-1", "app-2"},
		},
		{
			name: "list pods with multiple labels",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						Labels: map[string]string{
							"app": "myapp",
							"env": "prod",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-2",
						Namespace: "default",
						Labels: map[string]string{
							"app": "myapp",
							"env": "staging",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-3",
						Namespace: "default",
						Labels: map[string]string{
							"app": "other",
							"env": "prod",
						},
					},
				},
			},
			namespace:     "default",
			labelSelector: "app=myapp,env=prod",
			wantCount:     1,
			wantPodNames:  []string{"pod-1"},
		},
		{
			name:          "empty namespace returns no pods",
			pods:          []*corev1.Pod{},
			namespace:     "default",
			labelSelector: "",
			wantCount:     0,
			wantPodNames:  []string{},
		},
		{
			name: "label selector with no matches",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-1",
						Namespace: "default",
						Labels:    map[string]string{"app": "myapp"},
					},
				},
			},
			namespace:     "default",
			labelSelector: "app=nonexistent",
			wantCount:     0,
			wantPodNames:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake clientset with test pods
			objs := make([]runtime.Object, len(tt.pods))
			for i, pod := range tt.pods {
				objs[i] = pod
			}
			clientset := fake.NewClientset(objs...)

			conn := &Connection{
				clientset: clientset,
				namespace: tt.namespace,
			}

			podNames, err := conn.ListPods(ctx, tt.namespace, tt.labelSelector)
			if err != nil {
				t.Errorf("ListPods() error = %v", err)
				return
			}

			if len(podNames) != tt.wantCount {
				t.Errorf("got %d pods, want %d", len(podNames), tt.wantCount)
			}

			// Check that all expected pod names are present
			podMap := make(map[string]bool)
			for _, name := range podNames {
				podMap[name] = true
			}

			for _, wantName := range tt.wantPodNames {
				if !podMap[wantName] {
					t.Errorf("expected pod %s not found in results", wantName)
				}
			}
		})
	}
}

func TestListDeployments(t *testing.T) {
	tests := []struct {
		name            string
		deployments     []*appsv1.Deployment
		namespace       string
		wantCount       int
		wantDeployments []string
	}{
		{
			name: "list all deployments",
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-1",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-2",
						Namespace: "default",
					},
				},
			},
			namespace:       "default",
			wantCount:       2,
			wantDeployments: []string{"app-1", "app-2"},
		},
		{
			name: "list deployments in specific namespace",
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prod-app",
						Namespace: "production",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "staging-app",
						Namespace: "staging",
					},
				},
			},
			namespace:       "production",
			wantCount:       1,
			wantDeployments: []string{"prod-app"},
		},
		{
			name:            "empty namespace",
			deployments:     []*appsv1.Deployment{},
			namespace:       "default",
			wantCount:       0,
			wantDeployments: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake clientset with test deployments
			objs := make([]runtime.Object, len(tt.deployments))
			for i, dep := range tt.deployments {
				objs[i] = dep
			}
			clientset := fake.NewClientset(objs...)

			conn := &Connection{
				clientset: clientset,
				namespace: tt.namespace,
			}

			deploymentNames, err := conn.ListDeployments(ctx, tt.namespace)
			if err != nil {
				t.Errorf("ListDeployments() error = %v", err)
				return
			}

			if len(deploymentNames) != tt.wantCount {
				t.Errorf("got %d deployments, want %d", len(deploymentNames), tt.wantCount)
			}

			// Check that all expected deployments are present
			depMap := make(map[string]bool)
			for _, name := range deploymentNames {
				depMap[name] = true
			}

			for _, wantName := range tt.wantDeployments {
				if !depMap[wantName] {
					t.Errorf("expected deployment %s not found in results", wantName)
				}
			}
		})
	}
}

func TestGetDeploymentPods(t *testing.T) {
	tests := []struct {
		name           string
		deployment     *appsv1.Deployment
		pods           []*corev1.Pod
		namespace      string
		deploymentName string
		wantErr        bool
		wantPodCount   int
		wantPodNames   []string
	}{
		{
			name: "deployment with multiple pods",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "my-app",
						},
					},
				},
			},
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-abc123",
						Namespace: "default",
						Labels:    map[string]string{"app": "my-app"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-app-def456",
						Namespace: "default",
						Labels:    map[string]string{"app": "my-app"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-app-xyz789",
						Namespace: "default",
						Labels:    map[string]string{"app": "other-app"},
					},
				},
			},
			namespace:      "default",
			deploymentName: "my-app",
			wantErr:        false,
			wantPodCount:   2,
			wantPodNames:   []string{"my-app-abc123", "my-app-def456"},
		},
		{
			name: "deployment with no pods",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-app",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "empty-app",
						},
					},
				},
			},
			pods:           []*corev1.Pod{},
			namespace:      "default",
			deploymentName: "empty-app",
			wantErr:        false,
			wantPodCount:   0,
			wantPodNames:   []string{},
		},
		{
			name:           "non-existent deployment",
			deployment:     nil,
			pods:           []*corev1.Pod{},
			namespace:      "default",
			deploymentName: "missing-deployment",
			wantErr:        true,
		},
		{
			name: "deployment with complex label selector",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "complex-app",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "complex-app",
							"tier": "backend",
						},
					},
				},
			},
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "complex-app-1",
						Namespace: "default",
						Labels: map[string]string{
							"app":  "complex-app",
							"tier": "backend",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "complex-app-2",
						Namespace: "default",
						Labels: map[string]string{
							"app": "complex-app",
							// Missing "tier" label
						},
					},
				},
			},
			namespace:      "default",
			deploymentName: "complex-app",
			wantErr:        false,
			wantPodCount:   1,
			wantPodNames:   []string{"complex-app-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake clientset with test data
			objs := make([]runtime.Object, 0)
			if tt.deployment != nil {
				objs = append(objs, tt.deployment)
			}
			for _, pod := range tt.pods {
				objs = append(objs, pod)
			}
			clientset := fake.NewClientset(objs...)

			conn := &Connection{
				clientset: clientset,
				namespace: tt.namespace,
			}

			podNames, err := conn.GetDeploymentPods(ctx, tt.namespace, tt.deploymentName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeploymentPods() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(podNames) != tt.wantPodCount {
					t.Errorf("got %d pods, want %d", len(podNames), tt.wantPodCount)
				}

				// Check that all expected pods are present
				podMap := make(map[string]bool)
				for _, name := range podNames {
					podMap[name] = true
				}

				for _, wantName := range tt.wantPodNames {
					if !podMap[wantName] {
						t.Errorf("expected pod %s not found in results", wantName)
					}
				}
			}
		})
	}
}

func TestClientsetAccessor(t *testing.T) {
	clientset := fake.NewClientset()
	conn := &Connection{
		clientset: clientset,
		namespace: "default",
	}

	result := conn.Clientset()
	if result == nil {
		t.Error("Clientset() returned nil")
	}

	// Verify it's the same clientset
	if result != clientset {
		t.Error("Clientset() returned different clientset instance")
	}
}

func TestNamespaceAccessor(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		wantNamespace string
	}{
		{"default namespace", "default", "default"},
		{"custom namespace", "production", "production"},
		{"empty namespace", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{
				clientset: fake.NewClientset(),
				namespace: tt.namespace,
			}

			result := conn.Namespace()
			if result != tt.wantNamespace {
				t.Errorf("Namespace() = %v, want %v", result, tt.wantNamespace)
			}
		})
	}
}
