package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	testenv "github.com/deviceinsight/kubectl-actuator/test"
)

func main() {
	// Build the Spring Boot test app Docker image
	if err := testenv.BuildSpringAppImage(); err != nil {
		fmt.Printf("Failed to build Spring Boot image: %v\n", err)
		os.Exit(1)
	}

	// Set up test environment
	fmt.Println("Setting up test environment...")
	env := testenv.SetupTestEnvironment()
	if env == nil {
		fmt.Println("Failed to set up test environment")
		os.Exit(1)
	}

	// Write kubeconfig to temp file
	tmpfile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		fmt.Printf("Failed to create temp kubeconfig: %v\n", err)
		os.Exit(1)
	}
	kubeconfigPath := tmpfile.Name()
	defer func() { _ = os.Remove(kubeconfigPath) }()

	if _, err := tmpfile.Write([]byte(env.Kubeconfig)); err != nil {
		fmt.Printf("Failed to write kubeconfig: %v\n", err)
		os.Exit(1)
	}
	if err := tmpfile.Close(); err != nil {
		fmt.Printf("Failed to close kubeconfig file: %v\n", err)
		os.Exit(1)
	}

	// Print instructions
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Test environment is ready!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\nKubeconfig: %s\n\n", kubeconfigPath)
	fmt.Println("Useful commands:")
	fmt.Printf("  export KUBECONFIG=%s\n", kubeconfigPath)
	fmt.Println("  kubectl port-forward service/test-actuator-app http")
	fmt.Println("\nPress Ctrl+C to tear down the environment...")
	fmt.Println(strings.Repeat("=", 70))

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Cleanup
	fmt.Println("\nTearing down test environment...")
	if err := env.K3sContainer.Terminate(env.Ctx); err != nil {
		fmt.Printf("Failed to terminate K3s container: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Environment cleaned up successfully!")
	os.Exit(0)
}
