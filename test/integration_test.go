package test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Shared test environment, built on first use so that harness unit tests can
// run without Docker or a cluster.
var (
	sharedEnv    *TestEnvironment
	sharedEnvErr error
	setupOnce    sync.Once
)

func getSharedEnv(t *testing.T) *TestEnvironment {
	t.Helper()

	setupOnce.Do(func() {
		if err := BuildBinary(); err != nil {
			sharedEnvErr = fmt.Errorf("failed to build binary: %w", err)
			return
		}

		if err := BuildSpringAppImage(); err != nil {
			sharedEnvErr = fmt.Errorf("failed to build Spring Boot image: %w", err)
			return
		}

		fmt.Println("Setting up shared test environment...")
		sharedEnv, sharedEnvErr = SetupTestEnvironment()
	})

	if sharedEnvErr != nil {
		t.Fatalf("failed to set up test environment: %v", sharedEnvErr)
	}
	return sharedEnv
}

func TestMain(m *testing.M) {
	exitCode := m.Run()

	if sharedEnv != nil {
		fmt.Println("Cleaning up shared test environment...")
		if err := sharedEnv.Teardown(); err != nil {
			fmt.Printf("Failed to terminate K3s container: %v\n", err)
		}
	}

	os.Exit(exitCode)
}

func TestTextBasedTests(t *testing.T) {
	testFiles, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatalf("Failed to find test files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Fatal("No test files found in testdata/")
	}

	// Parse every scenario file before booting the cluster: a scenario typo
	// should fail in milliseconds, not after minutes of environment setup.
	type parsedFile struct {
		name      string
		scenarios []Scenario
	}
	var files []parsedFile
	for _, testFile := range testFiles {
		fileName := filepath.Base(testFile)
		scenarios, err := ParseScenarioFile(testFile)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", fileName, err)
		}
		files = append(files, parsedFile{name: fileName, scenarios: scenarios})
	}

	env := getSharedEnv(t)

	ctx, err := env.GetTemplateContext()
	if err != nil {
		t.Fatalf("Failed to get template context: %v", err)
	}

	for _, file := range files {
		for _, scenario := range file.scenarios {
			t.Run(file.name+"/"+scenario.Name, func(t *testing.T) {
				if err := env.RunScenario(scenario, ctx); err != nil {
					t.Errorf("%v", err)
				}
			})
		}
	}
}
