package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	K3sImage       = "rancher/k3s:v1.28.5-k3s1"
	SpringAppImage = "test-actuator-app:latest"
	Namespace      = "default"
	DeploymentName = "test-actuator-app"
)

type TestEnvironment struct {
	Ctx          context.Context
	K3sContainer *k3s.K3sContainer
	Kubeconfig   string
	Clientset    *kubernetes.Clientset
	BinaryPath   string
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

func SetupTestEnvironment() *TestEnvironment {
	ctx := context.Background()

	// Start K3s container using the module
	fmt.Println("Starting K3s container...")
	k3sContainer, err := k3s.Run(ctx, K3sImage,
		k3s.WithManifest("k8s/deployment.yaml"),
		testcontainers.CustomizeRequest(testcontainers.GenericContainerRequest{
			ContainerRequest: testcontainers.ContainerRequest{
				Cmd: []string{
					"server",
					"--disable=traefik",
					"--tls-san=127.0.0.1",
					"--kubelet-arg=eviction-hard=nodefs.available<1%",
					"--kubelet-arg=eviction-minimum-reclaim=nodefs.available=1%",
				},
			},
		}),
	)
	if err != nil {
		fmt.Printf("Failed to start K3s container: %v\n", err)
		return nil
	}

	// Get kubeconfig
	kubeConfigYaml, err := k3sContainer.GetKubeConfig(ctx)
	if err != nil {
		fmt.Printf("Failed to get kubeconfig: %v\n", err)
		return nil
	}

	// Create Kubernetes client
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	if err != nil {
		fmt.Printf("Failed to create REST config: %v\n", err)
		return nil
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Failed to create clientset: %v\n", err)
		return nil
	}

	// Import Spring Boot image into K3s
	fmt.Println("Importing Spring Boot image into K3s...")
	if err := k3sContainer.LoadImages(ctx, SpringAppImage); err != nil {
		fmt.Printf("Failed to import image: %v\n", err)
		return nil
	}

	// Wait for pods to be ready
	fmt.Println("Waiting for pods to be ready...")
	if err := WaitForPodsReady(ctx, clientset, Namespace, "app="+DeploymentName, 2, 120*time.Second); err != nil {
		fmt.Printf("Pods did not become ready: %v\n", err)
		return nil
	}

	binaryPath, _ := filepath.Abs("kubectl-actuator")

	return &TestEnvironment{
		Ctx:          ctx,
		K3sContainer: k3sContainer,
		Kubeconfig:   string(kubeConfigYaml),
		Clientset:    clientset,
		BinaryPath:   binaryPath,
	}
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
