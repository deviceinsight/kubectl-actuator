package test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Shared test environment across all tests
var sharedEnv *TestEnvironment

func TestMain(m *testing.M) {
	var exitCode int

	// Build the kubectl-actuator binary
	if err := BuildBinary(); err != nil {
		fmt.Printf("Failed to build binary: %v\n", err)
		os.Exit(1)
	}

	// Build the Spring Boot test app Docker image
	if err := BuildSpringAppImage(); err != nil {
		fmt.Printf("Failed to build Spring Boot image: %v\n", err)
		os.Exit(1)
	}

	// Set up shared test environment once
	fmt.Println("Setting up shared test environment...")
	sharedEnv = SetupTestEnvironment()
	if sharedEnv == nil {
		fmt.Println("Failed to set up test environment")
		os.Exit(1)
	}

	// Run tests
	exitCode = m.Run()

	// Clean up shared environment
	fmt.Println("Cleaning up shared test environment...")
	if err := sharedEnv.K3sContainer.Terminate(sharedEnv.Ctx); err != nil {
		fmt.Printf("Failed to terminate K3s container: %v\n", err)
	}

	os.Exit(exitCode)
}

func TestTextBasedTests(t *testing.T) {
	// Find all test definition files
	testFiles, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatalf("Failed to find test files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Fatal("No test files found in testdata/")
	}

	// Get template context once for all tests
	ctx, err := sharedEnv.GetTemplateContext()
	if err != nil {
		t.Fatalf("Failed to get template context: %v", err)
	}

	// Run tests from each file
	for _, testFile := range testFiles {
		fileName := filepath.Base(testFile)

		// Parse test definitions
		tests, err := ParseTestFile(testFile)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", fileName, err)
		}

		// Run each test as a subtest
		for _, test := range tests {
			testName := fileName + "/" + test.Name
			t.Run(testName, func(t *testing.T) {
				err := sharedEnv.RunTextBasedTest(test, ctx)
				if err != nil {
					t.Errorf("%v", err)
				}
			})
		}
	}
}
