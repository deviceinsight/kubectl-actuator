package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	testenv "github.com/deviceinsight/kubectl-actuator/test"
)

func fail(msg string, err error) {
	fmt.Printf("%s: %v\n", msg, err)
	os.Exit(1)
}

func main() {
	if err := testenv.BuildSpringAppImage(); err != nil {
		fail("Failed to build Spring Boot image", err)
	}

	fmt.Println("Setting up test environment...")
	env, err := testenv.SetupTestEnvironment()
	if err != nil {
		fail("Failed to set up test environment", err)
	}

	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println("Test environment is ready!")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("\nKubeconfig: %s\n\n", env.KubeconfigPath)
	fmt.Println("Useful commands:")
	fmt.Printf("  export KUBECONFIG=%s\n", env.KubeconfigPath)
	fmt.Println("  kubectl port-forward service/test-actuator-app http")
	fmt.Println("\nPress Ctrl+C to tear down the environment...")
	fmt.Println(strings.Repeat("=", 70))

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nTearing down test environment...")
	if err := env.Teardown(); err != nil {
		fail("Failed to terminate K3s container", err)
	}

	fmt.Println("Environment cleaned up successfully!")
}
