package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deviceinsight/kubectl-actuator/internal/actuator"
)

func mustDecode(t *testing.T, data []byte) map[string]any {
	t.Helper()
	tree, err := decodeTree(data)
	if err != nil {
		t.Fatalf("failed to decode result: %v", err)
	}
	return tree
}

func TestMarshalStructured(t *testing.T) {
	errMsg := "connection refused"
	output := structuredOutput{Pods: []podResult{
		{Name: "pod-1", Data: json.RawMessage(`{"status":"UP"}`)},
		{Name: "pod-2", Error: &errMsg},
	}}

	jsonBytes, err := marshalStructured(output, OutputFormatJSON)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	var roundTrip structuredOutput
	if err := json.Unmarshal(jsonBytes, &roundTrip); err != nil {
		t.Fatalf("json output is not valid JSON: %v", err)
	}
	if len(roundTrip.Pods) != 2 || roundTrip.Pods[0].Name != "pod-1" {
		t.Errorf("unexpected round trip: %+v", roundTrip)
	}
	if roundTrip.Pods[1].Error == nil || *roundTrip.Pods[1].Error != errMsg {
		t.Errorf("error field lost in round trip: %+v", roundTrip.Pods[1])
	}
	if !strings.HasSuffix(string(jsonBytes), "\n") {
		t.Error("json output should end with a newline")
	}

	yamlBytes, err := marshalStructured(output, OutputFormatYAML)
	if err != nil {
		t.Fatalf("yaml marshal failed: %v", err)
	}
	yamlStr := string(yamlBytes)
	for _, want := range []string{"pods:", "name: pod-1", "status: UP", "error: connection refused"} {
		if !strings.Contains(yamlStr, want) {
			t.Errorf("yaml output missing %q\nGot:\n%s", want, yamlStr)
		}
	}
}

func TestFilterEnvJSON(t *testing.T) {
	data := json.RawMessage(`{
		"activeProfiles": ["prod"],
		"propertySources": [
			{
				"name": "systemProperties",
				"customField": "preserved",
				"properties": {
					"spring.application.name": {"value": "my-app"},
					"java.version": {"value": "21"}
				}
			}
		]
	}`)

	result, err := filterEnvJSON(data, "SPRING")
	if err != nil {
		t.Fatalf("filterEnvJSON failed: %v", err)
	}

	tree := mustDecode(t, result)
	sources := tree["propertySources"].([]any)
	source := sources[0].(map[string]any)
	properties := source["properties"].(map[string]any)

	if _, ok := properties["spring.application.name"]; !ok {
		t.Error("case-insensitive match spring.application.name was dropped")
	}
	if _, ok := properties["java.version"]; ok {
		t.Error("non-matching java.version was kept")
	}
	if source["customField"] != "preserved" {
		t.Error("unknown field customField was not preserved")
	}
	if profiles, ok := tree["activeProfiles"].([]any); !ok || len(profiles) != 1 {
		t.Error("activeProfiles was not preserved")
	}

	// No filter: bytes pass through untouched
	passthrough, err := filterEnvJSON(data, "")
	if err != nil || string(passthrough) != string(data) {
		t.Error("empty filter should pass data through unchanged")
	}
}

func TestFilterBeansJSON(t *testing.T) {
	data := json.RawMessage(`{
		"contexts": {
			"my-app": {
				"parentId": null,
				"beans": {
					"userController": {"type": "com.example.UserController", "dependencies": []},
					"objectMapper": {"type": "com.fasterxml.jackson.databind.ObjectMapper", "dependencies": []}
				}
			}
		}
	}`)

	result, err := filterBeansJSON(data, "controller")
	if err != nil {
		t.Fatalf("filterBeansJSON failed: %v", err)
	}

	tree := mustDecode(t, result)
	appContext := tree["contexts"].(map[string]any)["my-app"].(map[string]any)
	beans := appContext["beans"].(map[string]any)

	if _, ok := beans["userController"]; !ok {
		t.Error("matching userController was dropped")
	}
	if _, ok := beans["objectMapper"]; ok {
		t.Error("non-matching objectMapper was kept")
	}
	if _, ok := appContext["parentId"]; !ok {
		t.Error("parentId field was not preserved")
	}
}

func TestFilterMetricNamesJSON(t *testing.T) {
	data := json.RawMessage(`{"names": ["jvm.memory.used", "jvm.memory.max", "http.server.requests"]}`)

	result, err := filterMetricNamesJSON(data, "jvm.memory")
	if err != nil {
		t.Fatalf("filterMetricNamesJSON failed: %v", err)
	}

	tree := mustDecode(t, result)
	names := tree["names"].([]any)
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d: %v", len(names), names)
	}
	for _, name := range names {
		if !strings.HasPrefix(name.(string), "jvm.memory") {
			t.Errorf("unexpected name kept: %v", name)
		}
	}
}

func TestFilterThreadDumpJSON(t *testing.T) {
	data := json.RawMessage(`{
		"threads": [
			{"threadName": "main", "threadState": "RUNNABLE", "lockedMonitors": [{"className": "a.B"}]},
			{"threadName": "http-nio-8080-exec-1", "threadState": "WAITING"},
			{"threadName": "http-nio-8080-exec-2", "threadState": "RUNNABLE", "blockedTime": 254431723520}
		]
	}`)

	result, err := filterThreadDumpJSON(data, "RUNNABLE", "http-nio")
	if err != nil {
		t.Fatalf("filterThreadDumpJSON failed: %v", err)
	}

	tree := mustDecode(t, result)
	threads := tree["threads"].([]any)
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	thread := threads[0].(map[string]any)
	if thread["threadName"] != "http-nio-8080-exec-2" {
		t.Errorf("wrong thread kept: %v", thread["threadName"])
	}
	if !strings.Contains(string(result), "254431723520") {
		t.Error("large number was not preserved verbatim")
	}

	// Name filter only, verifying unknown fields survive
	result, err = filterThreadDumpJSON(data, "", "main")
	if err != nil {
		t.Fatalf("filterThreadDumpJSON failed: %v", err)
	}
	if !strings.Contains(string(result), "lockedMonitors") {
		t.Error("unknown field lockedMonitors was not preserved")
	}
}

func TestFilterLoggersJSON(t *testing.T) {
	data := json.RawMessage(`{
		"levels": ["OFF", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"],
		"loggers": {
			"ROOT": {"configuredLevel": "INFO", "effectiveLevel": "INFO"},
			"com.example.app": {"configuredLevel": "DEBUG", "effectiveLevel": "DEBUG"},
			"com.example.app.service": {"configuredLevel": null, "effectiveLevel": "DEBUG"},
			"org.springframework": {"configuredLevel": null, "effectiveLevel": "INFO"}
		}
	}`)

	// Default: only configured loggers
	result, err := filterLoggersJSON(data, "", false)
	if err != nil {
		t.Fatalf("filterLoggersJSON failed: %v", err)
	}
	tree := mustDecode(t, result)
	loggers := tree["loggers"].(map[string]any)
	if len(loggers) != 2 {
		t.Errorf("expected 2 configured loggers, got %d: %v", len(loggers), loggers)
	}
	if _, ok := tree["levels"]; !ok {
		t.Error("levels field was not preserved")
	}

	// Prefix filter keeps configured loggers under the prefix
	result, err = filterLoggersJSON(data, "com.example", false)
	if err != nil {
		t.Fatalf("filterLoggersJSON failed: %v", err)
	}
	loggers = mustDecode(t, result)["loggers"].(map[string]any)
	if len(loggers) != 1 {
		t.Errorf("expected 1 logger for prefix, got %d: %v", len(loggers), loggers)
	}
	if _, ok := loggers["com.example.app"]; !ok {
		t.Error("com.example.app missing from prefix result")
	}

	// Exactly named unconfigured logger is always kept
	result, err = filterLoggersJSON(data, "com.example.app.service", false)
	if err != nil {
		t.Fatalf("filterLoggersJSON failed: %v", err)
	}
	loggers = mustDecode(t, result)["loggers"].(map[string]any)
	if _, ok := loggers["com.example.app.service"]; !ok {
		t.Error("exactly named unconfigured logger was dropped")
	}

	// showAll without name: passthrough
	result, err = filterLoggersJSON(data, "", true)
	if err != nil || string(result) != string(data) {
		t.Error("showAll without name should pass data through unchanged")
	}
}

func TestRunStructuredInterruptStillMarshalsEnvelope(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ops := &baseOperations{
		pods:                  []string{"pod-1", "pod-2"},
		actuatorClientFactory: &fakeClientFactory{},
	}

	var err error
	output := captureOutput(func() {
		err = ops.runStructured(ctx, OutputFormatJSON, "test", func(client actuator.Client) (json.RawMessage, error) {
			t.Fatal("no pod may be queried after an interrupt")
			return nil, nil
		})
	})

	if !errors.Is(err, ErrInterrupted) {
		t.Errorf("expected ErrInterrupted, got %v", err)
	}

	var envelope structuredOutput
	if unmarshalErr := json.Unmarshal([]byte(output), &envelope); unmarshalErr != nil {
		t.Fatalf("interrupted run must still print a valid envelope, got: %s", output)
	}
	if len(envelope.Pods) != 2 {
		t.Fatalf("expected 2 pods in envelope, got %d", len(envelope.Pods))
	}
	for _, pod := range envelope.Pods {
		if pod.Error == nil || !strings.Contains(*pod.Error, "interrupted") {
			t.Errorf("pod %s should be marked interrupted, got %+v", pod.Name, pod.Error)
		}
	}
}

func TestRunStructuredSinglePodFailureReturnsConcreteError(t *testing.T) {
	ops := &baseOperations{
		pods:                  []string{"pod-1"},
		actuatorClientFactory: &fakeClientFactory{errors: map[string]error{"pod-1": errors.New("pod \"pod-1\" not found in namespace \"default\"")}},
	}

	var err error
	_ = captureOutput(func() {
		err = ops.runStructured(context.Background(), OutputFormatJSON, "test", func(client actuator.Client) (json.RawMessage, error) {
			return nil, nil
		})
	})

	if err == nil || strings.Contains(err.Error(), "failed on") {
		t.Errorf("single-pod failure must return the concrete error, got %v", err)
	}
}
