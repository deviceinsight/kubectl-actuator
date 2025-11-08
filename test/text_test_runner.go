package test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDefinition represents a single test case
type TestDefinition struct {
	Name  string
	Steps []TestStep
}

// TestStep represents a command and its expectations
type TestStep struct {
	Command       string
	Expectations  []Expectation
	ExpectFailure bool // If true, command is expected to fail (non-zero exit)
}

// Expectation represents an expected output pattern
type Expectation struct {
	Pattern    string
	IsRegex    bool
	Negate     bool // If true, pattern should NOT match
	IsJSON     bool // If true, validate as JSON structure
	IsJSONPath bool // If true, validate using JSON path query
}

// TemplateContext holds variables for template substitution
type TemplateContext struct {
	Pods       []string
	Deployment string
	Namespace  string
}

// ParseTestFile parses a test definition file in txtar-inspired format
func ParseTestFile(filePath string) ([]TestDefinition, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	var tests []TestDefinition
	var currentTest *TestDefinition
	var currentStep *TestStep
	var currentSection string
	var sectionContent []string

	lines := strings.Split(string(content), "\n")

	finishSection := func() {
		if currentTest == nil || currentSection == "" {
			return
		}

		content := strings.TrimSpace(strings.Join(sectionContent, "\n"))
		if content == "" {
			sectionContent = nil
			return
		}

		switch currentSection {
		case "command":
			// Finish previous step if it exists
			if currentStep != nil {
				currentTest.Steps = append(currentTest.Steps, *currentStep)
			}
			// Start new step with this command
			currentStep = &TestStep{
				Command: content,
			}
		case "expect:error":
			if currentStep == nil {
				currentStep = &TestStep{}
			}
			currentStep.ExpectFailure = true
			currentStep.Expectations = append(currentStep.Expectations, Expectation{
				Pattern: content,
				IsRegex: false,
			})
		case "expect":
			if currentStep == nil {
				// Create a step with empty command if expect comes first (error case)
				currentStep = &TestStep{}
			}
			currentStep.Expectations = append(currentStep.Expectations, Expectation{
				Pattern: content,
				IsRegex: false,
			})
		case "expect:regex":
			if currentStep == nil {
				currentStep = &TestStep{}
			}
			currentStep.Expectations = append(currentStep.Expectations, Expectation{
				Pattern: content,
				IsRegex: true,
			})
		case "expect:not":
			if currentStep == nil {
				currentStep = &TestStep{}
			}
			currentStep.Expectations = append(currentStep.Expectations, Expectation{
				Pattern: content,
				IsRegex: false,
				Negate:  true,
			})
		case "expect:json":
			if currentStep == nil {
				currentStep = &TestStep{}
			}
			currentStep.Expectations = append(currentStep.Expectations, Expectation{
				Pattern: content,
				IsJSON:  true,
			})
		case "expect:jsonpath":
			if currentStep == nil {
				currentStep = &TestStep{}
			}
			currentStep.Expectations = append(currentStep.Expectations, Expectation{
				Pattern:    content,
				IsJSONPath: true,
			})
		}

		sectionContent = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for test header
		if strings.HasPrefix(trimmed, "-- test:") && strings.HasSuffix(trimmed, "--") {
			// Save previous step and test
			finishSection()
			if currentStep != nil && currentTest != nil {
				currentTest.Steps = append(currentTest.Steps, *currentStep)
				currentStep = nil
			}
			if currentTest != nil {
				tests = append(tests, *currentTest)
			}

			// Start new test
			testName := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "-- test:"), "--"))
			currentTest = &TestDefinition{Name: testName}
			currentStep = nil
			currentSection = ""
			continue
		}

		// Check for section headers
		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			finishSection()

			sectionName := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "--"), "--"))
			currentSection = sectionName
			continue
		}

		// Accumulate section content (including empty lines)
		if currentSection != "" {
			sectionContent = append(sectionContent, line)
		}
	}

	// Save last section and test
	finishSection()
	if currentStep != nil && currentTest != nil {
		currentTest.Steps = append(currentTest.Steps, *currentStep)
	}
	if currentTest != nil {
		tests = append(tests, *currentTest)
	}

	return tests, nil
}

// SubstituteTemplates replaces template variables in the command string
func SubstituteTemplates(command string, ctx TemplateContext) string {
	result := command

	// Replace {{pod}} with first pod
	result = strings.ReplaceAll(result, "{{pod}}", ctx.Pods[0])

	// Replace {{pod[0]}}, {{pod[1]}}, etc.
	for i, pod := range ctx.Pods {
		placeholder := fmt.Sprintf("{{pod[%d]}}", i)
		result = strings.ReplaceAll(result, placeholder, pod)
	}

	// Replace {{deployment}}
	result = strings.ReplaceAll(result, "{{deployment}}", ctx.Deployment)

	// Replace {{namespace}}
	result = strings.ReplaceAll(result, "{{namespace}}", ctx.Namespace)

	return result
}

// generateDiff creates a unified diff between expected and actual output
func generateDiff(expected, actual string) string {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(expected),
		B:        difflib.SplitLines(actual),
		FromFile: "Expected",
		ToFile:   "Actual",
		Context:  3,
	}
	result, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Sprintf("(error generating diff: %v)\nExpected:\n%s\n\nActual:\n%s", err, expected, actual)
	}
	return result
}

// RunTextBasedTest executes a single test definition
func (env *TestEnvironment) RunTextBasedTest(test TestDefinition, ctx TemplateContext) error {
	// Execute each step (command + its expectations)
	for stepIdx, step := range test.Steps {
		// Substitute templates in command
		substitutedCmd := SubstituteTemplates(step.Command, ctx)

		// Split command into binary and args
		parts := strings.Fields(substitutedCmd)
		if len(parts) == 0 {
			return fmt.Errorf("step %d: empty command", stepIdx+1)
		}

		// The binary name should be replaced with full path
		if parts[0] == "kubectl-actuator" {
			parts[0] = env.BinaryPath
		}

		// Execute command
		var output, err = env.executeCommand(parts[0], parts[1:]...)

		// Check if failure was expected
		if err != nil {
			if !step.ExpectFailure {
				return fmt.Errorf("step %d command failed: %w\nOutput: %s", stepIdx+1, err, output)
			}
			// Command failed as expected, continue with expectations on error output
		} else if step.ExpectFailure {
			return fmt.Errorf("step %d: command was expected to fail but succeeded\nOutput: %s", stepIdx+1, output)
		}

		// Validate expectations for this command
		for expectIdx, expect := range step.Expectations {
			// Substitute templates in expected pattern too
			expectedPattern := SubstituteTemplates(expect.Pattern, ctx)

			var matched bool
			var validationErr error

			if expect.IsJSONPath {
				// Validate using JSON path
				matched, validationErr = validateJSONPath(output, expectedPattern)
			} else if expect.IsJSON {
				// Validate JSON structure
				matched, validationErr = validateJSON(output, expectedPattern)
			} else if expect.IsRegex {
				re, err := regexp.Compile(expectedPattern)
				if err != nil {
					return fmt.Errorf("step %d expectation %d: invalid regex: %w", stepIdx+1, expectIdx+1, err)
				}
				matched = re.MatchString(output)
			} else {
				matched = strings.Contains(output, expectedPattern)
			}

			if validationErr != nil {
				return fmt.Errorf("step %d expectation %d: %w", stepIdx+1, expectIdx+1, validationErr)
			}

			// Handle negation
			if expect.Negate {
				matched = !matched
			}

			if !matched {
				if expect.Negate {
					return fmt.Errorf("step %d expectation %d failed:\nPattern should NOT be present but was found: %s\nOutput:\n%s",
						stepIdx+1, expectIdx+1, expectedPattern, output)
				} else if expect.IsRegex {
					return fmt.Errorf("step %d expectation %d failed:\nRegex pattern did not match: %s\nOutput:\n%s",
						stepIdx+1, expectIdx+1, expectedPattern, output)
				} else if expect.IsJSONPath {
					return fmt.Errorf("step %d expectation %d failed:\nJSON path validation failed: %s\nOutput:\n%s",
						stepIdx+1, expectIdx+1, expectedPattern, output)
				} else if expect.IsJSON {
					return fmt.Errorf("step %d expectation %d failed:\nJSON validation failed for path: %s\nOutput:\n%s",
						stepIdx+1, expectIdx+1, expectedPattern, output)
				} else {
					// For substring matches, show a diff
					diff := generateDiff(expectedPattern, output)
					return fmt.Errorf("step %d expectation %d failed:\nExpected substring not found in output.\n\nDiff (- expected, + actual):\n%s",
						stepIdx+1, expectIdx+1, diff)
				}
			}
		}
	}

	return nil
}

// executeCommand runs a command with the test environment's kubeconfig
func (env *TestEnvironment) executeCommand(binary string, args ...string) (string, error) {
	// Write kubeconfig to temp file
	tmpfile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpfile.Name()) }()

	if _, err := tmpfile.Write([]byte(env.Kubeconfig)); err != nil {
		return "", err
	}
	if err := tmpfile.Close(); err != nil {
		return "", err
	}

	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", tmpfile.Name()))

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// validateJSON validates that the output contains valid JSON and optionally checks a JSON path
func validateJSON(output, jsonPath string) (bool, error) {
	// First check if output is valid JSON
	var data interface{}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return false, fmt.Errorf("output is not valid JSON: %w", err)
	}

	// If no path specified, just validate that it's JSON
	if jsonPath == "" {
		return true, nil
	}

	// Simple JSON path checking - looks for "key": value patterns
	// For more complex path validation, could integrate a JSON path library
	if strings.Contains(output, jsonPath) {
		return true, nil
	}

	return false, nil
}

// validateJSONPath validates JSON output using gjson path queries
// Supports two modes:
//   - "status" - checks if field exists and has a truthy value
//   - "status == UP" - checks exact value match
func validateJSONPath(output, query string) (bool, error) {
	// First check if output is valid JSON
	if !gjson.Valid(output) {
		return false, fmt.Errorf("output is not valid JSON")
	}

	query = strings.TrimSpace(query)

	// Check for == operator
	if idx := strings.Index(query, " == "); idx != -1 {
		path := strings.TrimSpace(query[:idx])
		expected := strings.TrimSpace(query[idx+4:])
		// Remove quotes from expected value if present
		expected = strings.Trim(expected, "\"'")

		result := gjson.Get(output, path)
		if !result.Exists() {
			return false, nil
		}

		// Compare as strings (works for strings, numbers, booleans)
		return result.String() == expected, nil
	}

	// No operator - just check if the path exists and has a truthy value
	result := gjson.Get(output, query)
	if !result.Exists() {
		return false, nil
	}

	// For existence checks, any non-false, non-null value is truthy
	if result.Type == gjson.False || result.Type == gjson.Null {
		return false, nil
	}

	return true, nil
}

// GetTemplateContext creates a template context from the test environment
func (env *TestEnvironment) GetTemplateContext() (TemplateContext, error) {
	// Get pods
	pods, err := env.Clientset.CoreV1().Pods(Namespace).List(env.Ctx, metav1.ListOptions{
		LabelSelector: "app=" + DeploymentName,
	})
	if err != nil {
		return TemplateContext{}, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return TemplateContext{}, fmt.Errorf("no pods found")
	}

	podNames := make([]string, len(pods.Items))
	for i, pod := range pods.Items {
		podNames[i] = pod.Name
	}

	return TemplateContext{
		Pods:       podNames,
		Deployment: DeploymentName,
		Namespace:  Namespace,
	}, nil
}

// RunTestsFromFile loads and runs all tests from a file
func (env *TestEnvironment) RunTestsFromFile(filePath string) ([]string, []error) {
	tests, err := ParseTestFile(filePath)
	if err != nil {
		return nil, []error{fmt.Errorf("failed to parse test file: %w", err)}
	}

	ctx, err := env.GetTemplateContext()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to get template context: %w", err)}
	}

	var passed []string
	var failed []error

	for _, test := range tests {
		testName := filepath.Base(filePath) + "/" + test.Name
		err := env.RunTextBasedTest(test, ctx)
		if err != nil {
			failed = append(failed, fmt.Errorf("%s: %w", testName, err))
		} else {
			passed = append(passed, testName)
		}
	}

	return passed, failed
}
