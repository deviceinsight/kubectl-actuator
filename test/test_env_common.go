package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	K3sImage       = "rancher/k3s:v1.36.2-k3s1"
	SpringAppImage = "test-actuator-app:latest"
	Namespace      = "default"
	DeploymentName = "test-actuator-app"
)

type TestEnvironment struct {
	Ctx            context.Context
	K3sContainer   *k3s.K3sContainer
	Kubeconfig     string
	KubeconfigPath string
	Clientset      *kubernetes.Clientset
	BinaryPath     string
}

// Teardown terminates the cluster and removes the kubeconfig written by
// SetupTestEnvironment.
func (env *TestEnvironment) Teardown() error {
	_ = os.Remove(env.KubeconfigPath)
	return env.K3sContainer.Terminate(env.Ctx)
}

func BuildBinary() error {
	fmt.Println("Building kubectl-actuator binary...")
	cmd := exec.Command("go", "build", "-o", "test/kubectl-actuator", ".")
	cmd.Dir = filepath.Join("..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func BuildSpringAppImage() error {
	fmt.Println("Building Spring Boot test app Docker image...")
	cmd := exec.Command("docker", "build", "-t", SpringAppImage, "spring-app")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func SetupTestEnvironment() (*TestEnvironment, error) {
	ctx := context.Background()

	fmt.Println("Starting K3s container...")
	k3sContainer, err := k3s.Run(ctx, K3sImage,
		k3s.WithManifest("k8s/deployment.yaml"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Cmd: []string{
					"server",
					"--disable=traefik",
					"--tls-san=127.0.0.1",
					// Keep the kubelet from evicting the test pods on a
					// nearly-full host disk.
					"--kubelet-arg=eviction-hard=nodefs.available<1%",
					"--kubelet-arg=eviction-minimum-reclaim=nodefs.available=1%",
				},
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start K3s container: %w", err)
	}

	terminate := func() { _ = k3sContainer.Terminate(ctx) }

	kubeConfigYaml, err := k3sContainer.GetKubeConfig(ctx)
	if err != nil {
		terminate()
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		terminate()
		return nil, fmt.Errorf("failed to create REST config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		terminate()
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	fmt.Println("Importing Spring Boot image into K3s...")
	if err := k3sContainer.LoadImages(ctx, SpringAppImage); err != nil {
		terminate()
		return nil, fmt.Errorf("failed to import image: %w", err)
	}

	fmt.Println("Waiting for pods to be ready...")
	if err := WaitForPodsReady(ctx, clientset, Namespace, "app="+DeploymentName, 2, 120*time.Second); err != nil {
		DumpClusterDiagnostics(ctx, clientset, Namespace)
		terminate()
		return nil, fmt.Errorf("pods did not become ready: %w", err)
	}

	// Written once for the lifetime of the environment; every command
	// invocation points KUBECONFIG at it. Removed by Teardown.
	kubeconfigPath, err := writeKubeconfig(kubeConfigYaml)
	if err != nil {
		terminate()
		return nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	binaryPath, _ := filepath.Abs("kubectl-actuator")

	return &TestEnvironment{
		Ctx:            ctx,
		K3sContainer:   k3sContainer,
		Kubeconfig:     string(kubeConfigYaml),
		KubeconfigPath: kubeconfigPath,
		Clientset:      clientset,
		BinaryPath:     binaryPath,
	}, nil
}

func writeKubeconfig(content []byte) (string, error) {
	tmpfile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := tmpfile.Write(content); err != nil {
		_ = tmpfile.Close()
		_ = os.Remove(tmpfile.Name())
		return "", err
	}
	if err := tmpfile.Close(); err != nil {
		_ = os.Remove(tmpfile.Name())
		return "", err
	}
	return tmpfile.Name(), nil
}

// DumpClusterDiagnostics prints node conditions, pod statuses, and recent
// events so a setup failure is diagnosable without re-running the suite.
func DumpClusterDiagnostics(ctx context.Context, clientset *kubernetes.Clientset, namespace string) {
	fmt.Println("\n=== Cluster diagnostics ===")

	if nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err != nil {
		fmt.Printf("failed to list nodes: %v\n", err)
	} else {
		for _, node := range nodes.Items {
			fmt.Printf("Node %s:\n", node.Name)
			for _, cond := range node.Status.Conditions {
				// Print only conditions signaling problems.
				var problem bool
				if cond.Type == corev1.NodeReady {
					problem = cond.Status != corev1.ConditionTrue
				} else {
					problem = cond.Status == corev1.ConditionTrue
				}
				if problem {
					fmt.Printf("  condition %s=%s: %s %s\n", cond.Type, cond.Status, cond.Reason, cond.Message)
				}
			}
		}
	}

	if pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		fmt.Printf("failed to list pods: %v\n", err)
	} else {
		for _, pod := range pods.Items {
			fmt.Printf("Pod %s: phase=%s\n", pod.Name, pod.Status.Phase)
			for _, cond := range pod.Status.Conditions {
				if cond.Status != corev1.ConditionTrue {
					fmt.Printf("  condition %s=%s: %s %s\n", cond.Type, cond.Status, cond.Reason, cond.Message)
				}
			}
			for _, cs := range pod.Status.ContainerStatuses {
				state, detail := "running", ""
				if cs.State.Waiting != nil {
					state, detail = "waiting", cs.State.Waiting.Reason+" "+cs.State.Waiting.Message
				}
				if cs.State.Terminated != nil {
					state = "terminated"
					detail = fmt.Sprintf("%s exit=%d %s", cs.State.Terminated.Reason, cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
				}
				fmt.Printf("  container %s: ready=%t restarts=%d state=%s %s\n", cs.Name, cs.Ready, cs.RestartCount, state, detail)
			}
		}
	}

	if events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		fmt.Printf("failed to list events: %v\n", err)
	} else {
		items := events.Items
		sort.Slice(items, func(i, j int) bool {
			return items[i].LastTimestamp.Before(&items[j].LastTimestamp)
		})
		if len(items) > 20 {
			items = items[len(items)-20:]
		}
		fmt.Println("Recent events:")
		for _, event := range items {
			fmt.Printf("  %s %s %s/%s: %s\n",
				event.Type, event.Reason, event.InvolvedObject.Kind, event.InvolvedObject.Name, event.Message)
		}
	}

	fmt.Println("=== End diagnostics ===")
}

func WaitForPodsReady(ctx context.Context, clientset *kubernetes.Clientset, namespace, labelSelector string, expectedCount int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
		if err != nil {
			return err
		}

		readyCount := 0
		for _, pod := range pods.Items {
			if isPodReady(pod) {
				readyCount++
			} else {
				// Fail fast on obvious container failures
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
						return fmt.Errorf("pod %s container %s failed: %s (exit code %d)",
							pod.Name, cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
					}
					if cs.State.Waiting != nil && (cs.State.Waiting.Reason == "CrashLoopBackOff" || cs.State.Waiting.Reason == "ImagePullBackOff") {
						return fmt.Errorf("pod %s container %s: %s", pod.Name, cs.Name, cs.State.Waiting.Reason)
					}
				}
			}
		}

		if readyCount >= expectedCount {
			fmt.Printf("Pods ready: %d/%d\n", readyCount, expectedCount)
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for %d pods to be ready", expectedCount)
}

func isPodReady(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
