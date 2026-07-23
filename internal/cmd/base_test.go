package cmd

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/k8s"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

type mockK8sClient struct {
	pods        []string
	deployments map[string][]string // deployment name -> pod names
}

var _ k8s.Client = (*mockK8sClient)(nil)

func newMockK8sClient() *mockK8sClient {
	return &mockK8sClient{
		deployments: make(map[string][]string),
	}
}

func (m *mockK8sClient) GetPod(_ context.Context, _ string) (*corev1.Pod, error) {
	return nil, nil
}

func (m *mockK8sClient) ListPods(_ context.Context, _ string) ([]string, error) {
	return m.pods, nil
}

func (m *mockK8sClient) ListNamespaces(context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockK8sClient) ListDeployments(context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockK8sClient) GetDeploymentPods(_ context.Context, deploymentName string) ([]string, error) {
	if pods, ok := m.deployments[deploymentName]; ok {
		return pods, nil
	}
	return []string{}, nil
}

func (m *mockK8sClient) Namespace() string {
	return "test-namespace"
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
		wantErrContains string
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
				m.deployments = map[string][]string{
					"app-deployment": {"app-pod-1", "app-pod-2"},
				}
			},
			wantPods: []string{"app-pod-1", "app-pod-2"},
		},
		{
			name:            "multiple deployment flags",
			deploymentFlags: []string{"app-1", "app-2"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{
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
				m.pods = []string{"myapp-pod-1", "myapp-pod-2"}
			},
			wantPods: []string{"myapp-pod-1", "myapp-pod-2"},
		},
		{
			name:          "multiple label selectors",
			selectorFlags: []string{"app=myapp", "tier=backend"},
			setupMock: func(m *mockK8sClient) {
				m.pods = []string{"pod-1", "pod-2", "pod-3"}
			},
			wantPods: []string{"pod-1", "pod-2", "pod-3"},
		},
		{
			name:            "combination of pod and deployment",
			podFlags:        []string{"manual-pod"},
			deploymentFlags: []string{"app-deployment"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{
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
				m.deployments = map[string][]string{
					"app-deployment": {"app-pod-1", "app-pod-2"},
				}
				m.pods = []string{"myapp-pod-1"}
			},
			wantPods: []string{"manual-pod", "app-pod-1", "app-pod-2", "myapp-pod-1"},
		},
		{
			name:            "deduplication - same pod from different sources",
			podFlags:        []string{"duplicate-pod"},
			deploymentFlags: []string{"app-deployment"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{
					"app-deployment": {"duplicate-pod", "other-pod"},
				}
			},
			wantPods: []string{"duplicate-pod", "other-pod"}, // Only unique pods
		},
		{
			name:            "deduplication - multiple deployments with same pods",
			deploymentFlags: []string{"app-1", "app-2"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{
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
			name:     "comma-separated pod flag",
			podFlags: []string{"pod-1,pod-2", "pod-3"},
			wantPods: []string{"pod-1", "pod-2", "pod-3"},
		},
		{
			name:     "no flags returns empty list",
			wantPods: []string{},
		},
		{
			name:            "deployment with no pods is named in the error",
			deploymentFlags: []string{"empty-deployment"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{"empty-deployment": {}}
			},
			wantErr:         true,
			wantErrContains: `deployment "empty-deployment" has no pods in namespace "test-namespace"`,
		},
		{
			name:            "selector with no matches is named in the error",
			selectorFlags:   []string{"app=missing"},
			wantErr:         true,
			wantErrContains: `selector "app=missing" matched no pods in namespace "test-namespace"`,
		},
		{
			name:            "empty deployment and empty selector are both named",
			deploymentFlags: []string{"empty-deployment"},
			selectorFlags:   []string{"app=missing"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{"empty-deployment": {}}
			},
			wantErr:         true,
			wantErrContains: `deployment "empty-deployment" has no pods; selector "app=missing" matched no pods in namespace "test-namespace"`,
		},
		{
			name:            "multiple empty deployments use plural wording",
			deploymentFlags: []string{"empty-1", "empty-2"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{"empty-1": {}, "empty-2": {}}
			},
			wantErr:         true,
			wantErrContains: `deployments "empty-1", "empty-2" have no pods in namespace "test-namespace"`,
		},
		{
			name:            "empty deployment alongside a matching pod flag is not an error",
			podFlags:        []string{"pod-1"},
			deploymentFlags: []string{"empty-deployment"},
			setupMock: func(m *mockK8sClient) {
				m.deployments = map[string][]string{"empty-deployment": {}}
			},
			wantPods: []string{"pod-1"},
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
			rootCmd.PersistentFlags().StringSlice("pod", nil, "pod flag")
			rootCmd.PersistentFlags().StringSlice("deployment", nil, "deployment flag")
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

			if tt.wantErr {
				if tt.wantErrContains != "" && !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("error %q does not contain %q", err, tt.wantErrContains)
				}
				return
			}

			// The resolver promises order: pod flags, then deployment pods,
			// then selector matches, first occurrence winning on duplicates.
			if !slices.Equal(pods, tt.wantPods) {
				t.Errorf("pods = %v, want %v", pods, tt.wantPods)
			}
		})
	}
}

func TestValidatePods(t *testing.T) {
	t.Run("no pods yields guidance with a runnable example", func(t *testing.T) {
		b := &baseOperations{commandName: "env"}
		err := b.validatePods()
		if !errors.Is(err, ErrNoPodsSelected) {
			t.Fatalf("expected ErrNoPodsSelected, got %v", err)
		}
		for _, phrase := range []string{
			"no pods selected",
			"-p <pod>",
			"-d <deployment>",
			"-l <label-selector>",
			"e.g. 'kubectl actuator -d my-app env'",
		} {
			if !strings.Contains(err.Error(), phrase) {
				t.Errorf("error message does not contain %q\nGot: %s", phrase, err)
			}
		}
	})

	t.Run("selected pods pass", func(t *testing.T) {
		b := &baseOperations{pods: []string{"pod-1"}}
		if err := b.validatePods(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
