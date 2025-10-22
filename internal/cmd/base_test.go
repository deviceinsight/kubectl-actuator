package cmd

import (
	"context"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type mockK8sClient struct {
	pods        map[string][]string            // namespace -> pod names
	deployments map[string]map[string][]string // namespace -> deployment name -> pod names
}

var _ k8s.Client = (*mockK8sClient)(nil)

func newMockK8sClient() *mockK8sClient {
	return &mockK8sClient{
		pods:        make(map[string][]string),
		deployments: make(map[string]map[string][]string),
	}
}

func (m *mockK8sClient) GetPod(_ context.Context, _, _ string) (*corev1.Pod, error) {
	return nil, nil
}

func (m *mockK8sClient) ListPods(_ context.Context, namespace, _ string) ([]string, error) {
	if pods, ok := m.pods[namespace]; ok {
		return pods, nil
	}
	return []string{}, nil
}

func (m *mockK8sClient) ListDeployments(context.Context, string) ([]string, error) {
	return nil, nil
}

func (m *mockK8sClient) GetDeploymentPods(_ context.Context, namespace, deploymentName string) ([]string, error) {
	if deploys, ok := m.deployments[namespace]; ok {
		if pods, ok := deploys[deploymentName]; ok {
			return pods, nil
		}
	}
	return []string{}, nil
}

func (m *mockK8sClient) Clientset() kubernetes.Interface {
	return fake.NewClientset()
}

func (m *mockK8sClient) Namespace() string {
	return "default"
}

func TestFlagsPodResolver(t *testing.T) {
	tests := []struct {
		name            string
		podFlags        []string
		deploymentFlags []string
		selectorFlags   []string
		setupMock       func(*mockK8sClient)
		wantPods        []string
		wantErr         bool
	}{
		{
			name:     "single pod flag",
			podFlags: []string{"pod-1"},
			wantPods: []string{"pod-1"},
		},
		{
			name:     "multiple pod flags",
			podFlags: []string{"pod-1", "pod-2", "pod-3"},
			wantPods: []string{"pod-1", "pod-2", "pod-3"},
		},
		{
			name:            "single deployment flag",
			deploymentFlags: []string{"app-deployment"},
			setupMock: func(m *mockK8sClient) {
				m.deployments["default"] = map[string][]string{
					"app-deployment": {"app-pod-1", "app-pod-2"},
				}
			},
			wantPods: []string{"app-pod-1", "app-pod-2"},
		},
		{
			name:            "multiple deployment flags",
			deploymentFlags: []string{"app-1", "app-2"},
			setupMock: func(m *mockK8sClient) {
				m.deployments["default"] = map[string][]string{
					"app-1": {"app-1-pod-1", "app-1-pod-2"},
					"app-2": {"app-2-pod-1"},
				}
			},
			wantPods: []string{"app-1-pod-1", "app-1-pod-2", "app-2-pod-1"},
		},
		{
			name:          "single label selector",
			selectorFlags: []string{"app=myapp"},
			setupMock: func(m *mockK8sClient) {
				m.pods["default"] = []string{"myapp-pod-1", "myapp-pod-2"}
			},
			wantPods: []string{"myapp-pod-1", "myapp-pod-2"},
		},
		{
			name:          "multiple label selectors",
			selectorFlags: []string{"app=myapp", "tier=backend"},
			setupMock: func(m *mockK8sClient) {
				m.pods["default"] = []string{"pod-1", "pod-2", "pod-3"}
			},
			wantPods: []string{"pod-1", "pod-2", "pod-3"},
		},
		{
			name:            "combination of pod and deployment",
			podFlags:        []string{"manual-pod"},
			deploymentFlags: []string{"app-deployment"},
			setupMock: func(m *mockK8sClient) {
				m.deployments["default"] = map[string][]string{
					"app-deployment": {"app-pod-1", "app-pod-2"},
				}
			},
			wantPods: []string{"manual-pod", "app-pod-1", "app-pod-2"},
		},
		{
			name:            "combination of pod, deployment, and selector",
			podFlags:        []string{"manual-pod"},
			deploymentFlags: []string{"app-deployment"},
			selectorFlags:   []string{"app=myapp"},
			setupMock: func(m *mockK8sClient) {
				m.deployments["default"] = map[string][]string{
					"app-deployment": {"app-pod-1", "app-pod-2"},
				}
				m.pods["default"] = []string{"myapp-pod-1"}
			},
			wantPods: []string{"manual-pod", "app-pod-1", "app-pod-2", "myapp-pod-1"},
		},
		{
			name:            "deduplication - same pod from different sources",
			podFlags:        []string{"duplicate-pod"},
			deploymentFlags: []string{"app-deployment"},
			setupMock: func(m *mockK8sClient) {
				m.deployments["default"] = map[string][]string{
					"app-deployment": {"duplicate-pod", "other-pod"},
				}
			},
			wantPods: []string{"duplicate-pod", "other-pod"}, // Only unique pods
		},
		{
			name:            "deduplication - multiple deployments with same pods",
			deploymentFlags: []string{"app-1", "app-2"},
			setupMock: func(m *mockK8sClient) {
				m.deployments["default"] = map[string][]string{
					"app-1": {"shared-pod", "pod-1"},
					"app-2": {"shared-pod", "pod-2"},
				}
			},
			wantPods: []string{"shared-pod", "pod-1", "pod-2"},
		},
		{
			name:     "empty string pods are filtered",
			podFlags: []string{"pod-1", "", "pod-2", ""},
			wantPods: []string{"pod-1", "pod-2"},
		},
		{
			name:     "no flags returns empty list",
			wantPods: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			mockClient := newMockK8sClient()
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			rootCmd := &cobra.Command{Use: "root"}
			rootCmd.PersistentFlags().StringArray("pod", nil, "pod flag")
			rootCmd.PersistentFlags().StringArray("deployment", nil, "deployment flag")
			rootCmd.PersistentFlags().StringArray("selector", nil, "selector flag")

			for _, pod := range tt.podFlags {
				if err := rootCmd.PersistentFlags().Set("pod", pod); err != nil {
					t.Fatalf("Failed to set pod flag: %v", err)
				}
			}
			for _, dep := range tt.deploymentFlags {
				if err := rootCmd.PersistentFlags().Set("deployment", dep); err != nil {
					t.Fatalf("Failed to set deployment flag: %v", err)
				}
			}
			for _, sel := range tt.selectorFlags {
				if err := rootCmd.PersistentFlags().Set("selector", sel); err != nil {
					t.Fatalf("Failed to set selector flag: %v", err)
				}
			}

			cmd := &cobra.Command{Use: "test"}
			rootCmd.AddCommand(cmd)

			pods, err := FlagsPodResolver(ctx, mockClient, cmd)

			if (err != nil) != tt.wantErr {
				t.Errorf("FlagsPodResolver() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check pod count
			if len(pods) != len(tt.wantPods) {
				t.Errorf("got %d pods, want %d\nGot: %v\nWant: %v", len(pods), len(tt.wantPods), pods, tt.wantPods)
				return
			}

			// Create maps for comparison (order doesn't matter due to deduplication)
			gotMap := make(map[string]bool)
			for _, p := range pods {
				gotMap[p] = true
			}
			wantMap := make(map[string]bool)
			for _, p := range tt.wantPods {
				wantMap[p] = true
			}

			// Check all expected pods are present
			for pod := range wantMap {
				if !gotMap[pod] {
					t.Errorf("expected pod %s not found in result", pod)
				}
			}

			// Check no extra pods
			for pod := range gotMap {
				if !wantMap[pod] {
					t.Errorf("unexpected pod %s in result", pod)
				}
			}
		})
	}
}

func TestFlagsPodResolverWithFakeK8s(t *testing.T) {
	tests := []struct {
		name            string
		pods            []*corev1.Pod
		deployments     []*appsv1.Deployment
		podFlags        []string
		deploymentFlags []string
		selectorFlags   []string
		wantPodCount    int
		wantPodNames    []string
	}{
		{
			name: "resolve deployment to pods via label selector",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-pod-1",
						Namespace: "default",
						Labels:    map[string]string{"app": "myapp"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-pod-2",
						Namespace: "default",
						Labels:    map[string]string{"app": "myapp"},
					},
				},
			},
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "myapp",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "myapp"},
						},
					},
				},
			},
			deploymentFlags: []string{"myapp"},
			wantPodCount:    2,
			wantPodNames:    []string{"app-pod-1", "app-pod-2"},
		},
		{
			name: "resolve label selector to pods",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backend-1",
						Namespace: "default",
						Labels:    map[string]string{"tier": "backend"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backend-2",
						Namespace: "default",
						Labels:    map[string]string{"tier": "backend"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "frontend-1",
						Namespace: "default",
						Labels:    map[string]string{"tier": "frontend"},
					},
				},
			},
			selectorFlags: []string{"tier=backend"},
			wantPodCount:  2,
			wantPodNames:  []string{"backend-1", "backend-2"},
		},
		{
			name: "combined pod flags and deployment resolution",
			pods: []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "manual-pod",
						Namespace: "default",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-pod-1",
						Namespace: "default",
						Labels:    map[string]string{"app": "myapp"},
					},
				},
			},
			deployments: []*appsv1.Deployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "myapp",
						Namespace: "default",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "myapp"},
						},
					},
				},
			},
			podFlags:        []string{"manual-pod"},
			deploymentFlags: []string{"myapp"},
			wantPodCount:    2,
			wantPodNames:    []string{"manual-pod", "app-pod-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create fake clientset with test data
			objs := make([]runtime.Object, 0)
			for _, pod := range tt.pods {
				objs = append(objs, pod)
			}
			for _, dep := range tt.deployments {
				objs = append(objs, dep)
			}
			clientset := fake.NewClientset(objs...)

			// Create wrapper that implements K8sClient
			k8sClient := &fakeK8sClientWrapper{
				clientset: clientset,
				namespace: "default",
			}

			// Create command structure
			rootCmd := &cobra.Command{Use: "root"}
			rootCmd.PersistentFlags().StringArray("pod", nil, "pod flag")
			rootCmd.PersistentFlags().StringArray("deployment", nil, "deployment flag")
			rootCmd.PersistentFlags().StringArray("selector", nil, "selector flag")

			for _, pod := range tt.podFlags {
				if err := rootCmd.PersistentFlags().Set("pod", pod); err != nil {
					t.Fatalf("Failed to set pod flag: %v", err)
				}
			}
			for _, dep := range tt.deploymentFlags {
				if err := rootCmd.PersistentFlags().Set("deployment", dep); err != nil {
					t.Fatalf("Failed to set deployment flag: %v", err)
				}
			}
			for _, sel := range tt.selectorFlags {
				if err := rootCmd.PersistentFlags().Set("selector", sel); err != nil {
					t.Fatalf("Failed to set selector flag: %v", err)
				}
			}

			cmd := &cobra.Command{Use: "test"}
			rootCmd.AddCommand(cmd)

			// Call FlagsPodResolver
			pods, err := FlagsPodResolver(ctx, k8sClient, cmd)
			if err != nil {
				t.Errorf("FlagsPodResolver() error = %v", err)
				return
			}

			if len(pods) != tt.wantPodCount {
				t.Errorf("got %d pods, want %d", len(pods), tt.wantPodCount)
			}

			// Check expected pod names
			podMap := make(map[string]bool)
			for _, p := range pods {
				podMap[p] = true
			}

			for _, wantPod := range tt.wantPodNames {
				if !podMap[wantPod] {
					t.Errorf("expected pod %s not found in results", wantPod)
				}
			}
		})
	}
}

type fakeK8sClientWrapper struct {
	clientset interface{}
	namespace string
}

var _ k8s.Client = (*fakeK8sClientWrapper)(nil)

func (f *fakeK8sClientWrapper) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	cs := f.clientset.(*fake.Clientset)
	return cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (f *fakeK8sClientWrapper) ListPods(ctx context.Context, namespace, labelSelector string) ([]string, error) {
	cs := f.clientset.(*fake.Clientset)
	list, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, pod := range list.Items {
		names = append(names, pod.Name)
	}
	return names, nil
}

func (f *fakeK8sClientWrapper) ListDeployments(ctx context.Context, namespace string) ([]string, error) {
	cs := f.clientset.(*fake.Clientset)
	list, err := cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, dep := range list.Items {
		names = append(names, dep.Name)
	}
	return names, nil
}

func (f *fakeK8sClientWrapper) GetDeploymentPods(ctx context.Context, namespace, deploymentName string) ([]string, error) {
	cs := f.clientset.(*fake.Clientset)
	deployment, err := cs.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}

	podList, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}

	var podNames []string
	for _, pod := range podList.Items {
		podNames = append(podNames, pod.Name)
	}
	return podNames, nil
}

func (f *fakeK8sClientWrapper) Clientset() kubernetes.Interface {
	return f.clientset.(*fake.Clientset)
}

func (f *fakeK8sClientWrapper) Namespace() string {
	return f.namespace
}
