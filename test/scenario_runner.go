package test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Output streams an expectation can match against.
const (
	StreamCombined = ""
	StreamStdout   = "stdout"
	StreamStderr   = "stderr"
)

type Scenario struct {
	Name  string
	Steps []TestStep
}

type TestStep struct {
	Command       string
	Line          int // line of the -- command -- header in the scenario file
	Expectations  []Expectation
	ExpectFailure bool // command must exit non-zero
}

type Expectation struct {
	Pattern    string
	Line       int    // line of the section header in the scenario file
	Stream     string // StreamCombined, StreamStdout, or StreamStderr
	IsRegex    bool
	Negate     bool
	IsJSONPath bool
}

type TemplateContext struct {
	Pods       []string
	Deployment string
	Namespace  string
}

// sectionSpec describes how an expectation section maps onto an Expectation.
var sectionSpecs = map[string]Expectation{
	"expect":            {},
	"expect:stdout":     {Stream: StreamStdout},
	"expect:stderr":     {Stream: StreamStderr},
	"expect:not":        {Negate: true},
	"expect:not:stdout": {Negate: true, Stream: StreamStdout},
	"expect:not:stderr": {Negate: true, Stream: StreamStderr},
	"expect:regex":      {IsRegex: true},
	"expect:jsonpath":   {IsJSONPath: true},
	"expect:error":      {}, // additionally sets ExpectFailure on the step
}

// ParseScenarioFile parses a scenario file in txtar-inspired format.
// Unknown section names and empty sections are errors: a silently dropped
// expectation makes a test pass without asserting anything.
func ParseScenarioFile(filePath string) ([]Scenario, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	fileName := filepath.Base(filePath)

	var scenarios []Scenario
	var currentScenario *Scenario
	var currentStep *TestStep
	var currentSection string
	var sectionLine int
	var sectionContent []string

	lines := strings.Split(string(content), "\n")

	finishStep := func() {
		if currentStep != nil && currentScenario != nil {
			currentScenario.Steps = append(currentScenario.Steps, *currentStep)
		}
		currentStep = nil
	}

	finishSection := func() error {
		if currentSection == "" {
			return nil
		}

		section := currentSection
		content := strings.TrimSpace(strings.Join(sectionContent, "\n"))
		currentSection = ""
		sectionContent = nil

		if content == "" {
			return fmt.Errorf("%s:%d: section %q is empty", fileName, sectionLine, section)
		}
		if currentScenario == nil {
			return fmt.Errorf("%s:%d: section %q outside of a test", fileName, sectionLine, section)
		}

		if section == "command" {
			finishStep()
			currentStep = &TestStep{Command: content, Line: sectionLine}
			return nil
		}

		spec, known := sectionSpecs[section]
		if !known {
			return fmt.Errorf("%s:%d: unknown section %q", fileName, sectionLine, section)
		}
		if currentStep == nil {
			return fmt.Errorf("%s:%d: section %q before any command", fileName, sectionLine, section)
		}

		if section == "expect:error" {
			currentStep.ExpectFailure = true
		}
		expectation := spec
		expectation.Pattern = content
		expectation.Line = sectionLine
		currentStep.Expectations = append(currentStep.Expectations, expectation)
		return nil
	}

	finishScenario := func() error {
		if err := finishSection(); err != nil {
			return err
		}
		finishStep()
		if currentScenario != nil {
			scenarios = append(scenarios, *currentScenario)
		}
		currentScenario = nil
		return nil
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "-- test:") && strings.HasSuffix(trimmed, "--") {
			if err := finishScenario(); err != nil {
				return nil, err
			}

			testName := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "-- test:"), "--"))
			currentScenario = &Scenario{Name: testName}
			continue
		}

		if strings.HasPrefix(trimmed, "-- ") && strings.HasSuffix(trimmed, " --") {
			if err := finishSection(); err != nil {
				return nil, err
			}

			currentSection = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "--"), "--"))
			sectionLine = i + 1
			continue
		}

		// Content keeps empty lines: a multi-line pattern may span them.
		if currentSection != "" {
			sectionContent = append(sectionContent, line)
			continue
		}

		// Anything else outside a section is an error, for the same reason
		// unknown sections are: a mistyped header like '--- expect ---' must
		// not silently swallow the expectation below it.
		if trimmed != "" {
			return nil, fmt.Errorf("%s:%d: content outside of a section: %q", fileName, i+1, trimmed)
		}
	}

	if err := finishScenario(); err != nil {
		return nil, err
	}

	return scenarios, nil
}

// SubstituteTemplates fills the {{pod}}, {{pod[i]}}, {{deployment}} and
// {{namespace}} templates in commands and expectation patterns. A template
// left over after substitution (a typo, or a pod index beyond the pod count)
// is an error: it would otherwise surface as a baffling CLI failure about a
// pod literally named "{{deployement}}".
func SubstituteTemplates(text string, ctx TemplateContext) (string, error) {
	result := strings.ReplaceAll(text, "{{pod}}", ctx.Pods[0])
	for i, pod := range ctx.Pods {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{pod[%d]}}", i), pod)
	}
	result = strings.ReplaceAll(result, "{{deployment}}", ctx.Deployment)
	result = strings.ReplaceAll(result, "{{namespace}}", ctx.Namespace)

	if start := strings.Index(result, "{{"); start != -1 {
		leftover := result[start:]
		if end := strings.Index(leftover, "}}"); end != -1 {
			leftover = leftover[:end+2]
		}
		return "", fmt.Errorf("unknown template %q (known: {{pod}}, {{pod[i]}} with %d pods, {{deployment}}, {{namespace}})",
			leftover, len(ctx.Pods))
	}
	return result, nil
}

// splitCommand splits a command line into arguments, honoring shell-style
// quoting so arguments may contain spaces.
func splitCommand(command string) ([]string, error) {
	return shellquote.Split(command)
}

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

// formatOutput renders both streams for failure messages, labeling stderr
// only when it carries anything.
func formatOutput(stdout, stderr string) string {
	if stderr == "" {
		return stdout
	}
	return fmt.Sprintf("%s\n--- stderr ---\n%s", stdout, stderr)
}

// matchTarget selects the output an expectation is checked against.
func matchTarget(expect Expectation, stdout, stderr, combined string) string {
	// JSON documents are written to stdout; stderr noise must not break them.
	if expect.IsJSONPath {
		return stdout
	}
	switch expect.Stream {
	case StreamStdout:
		return stdout
	case StreamStderr:
		return stderr
	default:
		return combined
	}
}

// RunScenario executes a parsed scenario: each step's command runs
// with templates substituted, then its expectations are checked. Failure
// messages always name the substituted command so the author can rerun it
// by hand.
func (env *TestEnvironment) RunScenario(scenario Scenario, ctx TemplateContext) error {
	for stepIdx, step := range scenario.Steps {
		substitutedCmd, err := SubstituteTemplates(step.Command, ctx)
		if err != nil {
			return fmt.Errorf("step %d (line %d): %w", stepIdx+1, step.Line, err)
		}

		parts, err := splitCommand(substitutedCmd)
		if err != nil {
			return fmt.Errorf("step %d (line %d): failed to split command %q: %w", stepIdx+1, step.Line, substitutedCmd, err)
		}
		if len(parts) == 0 {
			return fmt.Errorf("step %d (line %d): empty command", stepIdx+1, step.Line)
		}

		// testdata names the binary by convention; point that at the built artifact
		if parts[0] == "kubectl-actuator" {
			parts[0] = env.BinaryPath
		}

		stdout, stderr, err := env.executeCommand(parts[0], parts[1:]...)

		if err != nil {
			if !step.ExpectFailure {
				return fmt.Errorf("step %d (line %d) command failed: %w\nCommand: %s\nOutput:\n%s",
					stepIdx+1, step.Line, err, substitutedCmd, formatOutput(stdout, stderr))
			}
			// Failed as expected; expectations still run against the error output.
		} else if step.ExpectFailure {
			return fmt.Errorf("step %d (line %d): command was expected to fail but succeeded\nCommand: %s\nOutput:\n%s",
				stepIdx+1, step.Line, substitutedCmd, formatOutput(stdout, stderr))
		}

		for expectIdx, expect := range step.Expectations {
			pattern, err := SubstituteTemplates(expect.Pattern, ctx)
			if err != nil {
				return fmt.Errorf("step %d expectation %d (line %d): %w", stepIdx+1, expectIdx+1, expect.Line, err)
			}
			if err := checkExpectation(expect, pattern, stdout, stderr); err != nil {
				return fmt.Errorf("step %d expectation %d (line %d): %w\nCommand: %s", stepIdx+1, expectIdx+1, expect.Line, err, substitutedCmd)
			}
		}
	}

	return nil
}

// checkExpectation evaluates one expectation against the command output,
// returning nil on a match and a descriptive failure otherwise.
func checkExpectation(expect Expectation, pattern, stdout, stderr string) error {
	target := matchTarget(expect, stdout, stderr, stdout+stderr)

	var matched bool
	switch {
	case expect.IsJSONPath:
		// No expect:not:jsonpath section exists, so Negate never applies
		// here: a jsonpath failure is final.
		if err := validateJSONPath(target, pattern); err != nil {
			return fmt.Errorf("%w\nOutput:\n%s", err, formatOutput(stdout, stderr))
		}
		matched = true
	case expect.IsRegex:
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
		matched = re.MatchString(target)
	default:
		matched = strings.Contains(target, pattern)
	}

	if expect.Negate {
		matched = !matched
	}
	if matched {
		return nil
	}

	streamLabel := "output"
	if expect.Stream != StreamCombined {
		streamLabel = expect.Stream
	}

	switch {
	case expect.Negate:
		return fmt.Errorf("pattern should NOT be present in %s but was found: %s\n%s:\n%s",
			streamLabel, pattern, streamLabel, target)
	case expect.IsRegex:
		return fmt.Errorf("regex pattern did not match: %s\nOutput:\n%s",
			pattern, formatOutput(stdout, stderr))
	default:
		// A diff only helps for multi-line patterns; for a one-liner it would
		// render the whole output as noise around a single minus line.
		if strings.Contains(pattern, "\n") {
			return fmt.Errorf("expected substring not found in %s\n\nDiff (- expected, + actual):\n%s",
				streamLabel, generateDiff(pattern, target))
		}
		return fmt.Errorf("expected substring %q not found in %s:\n%s",
			pattern, streamLabel, target)
	}
}

// executeCommand runs a command with the test environment's kubeconfig,
// capturing stdout and stderr separately.
func (env *TestEnvironment) executeCommand(binary string, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", env.KubeconfigPath))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// validateJSONPath validates JSON output using gjson path queries.
// Supports two modes:
//   - "status" - checks if field exists and has a truthy value
//   - "status == UP" - checks exact value match (spaces around == required)
//
// A nil return means the expectation holds; a non-nil error names what was
// found instead.
func validateJSONPath(output, query string) error {
	if !gjson.Valid(output) {
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("jsonpath %q: expected JSON on stdout, but stdout is empty", query)
		}
		return fmt.Errorf("jsonpath %q: stdout is not valid JSON:\n%s", query, output)
	}

	query = strings.TrimSpace(query)

	if idx := strings.Index(query, " == "); idx != -1 {
		path := strings.TrimSpace(query[:idx])
		expected := strings.TrimSpace(query[idx+4:])
		expected = strings.Trim(expected, "\"'")

		result := gjson.Get(output, path)
		if !result.Exists() {
			return fmt.Errorf("jsonpath: path %q not found", path)
		}

		// Compare as strings; covers strings, numbers, and booleans alike.
		if result.String() != expected {
			return fmt.Errorf("jsonpath: path %q = %q, want %q", path, result.String(), expected)
		}
		return nil
	}

	// gjson's own query syntax (#(...)) may legitimately contain ==, but a
	// bare a==b is almost certainly a value match missing its spaces - and
	// would otherwise be misread as an existence check of a weird path.
	if strings.Contains(query, "==") && !strings.Contains(query, "#(") {
		return fmt.Errorf("jsonpath %q: value matches need spaces around ==, e.g. 'status == UP'", query)
	}

	result := gjson.Get(output, query)
	if !result.Exists() {
		return fmt.Errorf("jsonpath: path %q not found", query)
	}

	// For existence checks, any non-false, non-null value is truthy
	if result.Type == gjson.False || result.Type == gjson.Null {
		return fmt.Errorf("jsonpath: path %q = %s, want a truthy value", query, result.Raw)
	}

	return nil
}

func (env *TestEnvironment) GetTemplateContext() (TemplateContext, error) {
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
