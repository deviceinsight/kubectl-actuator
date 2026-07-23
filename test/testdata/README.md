# Text-Based Integration Tests

This directory contains text-based integration tests for kubectl-actuator.

## Format

Tests use a txtar-inspired format with the following structure:

```
-- test: test name --
-- command --
kubectl-actuator --pod {{pod}} logger
-- command --
kubectl-actuator --pod {{pod}} logger com.example.testapp DEBUG
-- expect --
substring to find in output
-- expect:regex --
regex.*pattern
```

## Sections

- **`-- test: <name> --`**: Starts a new test case
- **`-- command --`**: Defines a command to execute (can have multiple). Arguments support shell-style quoting, so values may contain spaces: `--tag "id=G1 Old Gen"`
- **`-- expect --`**: Defines a substring that must appear in the last command's combined output (stdout + stderr)
- **`-- expect:stdout --`** / **`-- expect:stderr --`**: Substring that must appear on that specific stream
- **`-- expect:regex --`**: Defines a regex pattern that must match the last command's output
- **`-- expect:not --`**: Defines a substring that must NOT appear in the output (negated match)
- **`-- expect:not:stdout --`** / **`-- expect:not:stderr --`**: Substring that must NOT appear on that specific stream
- **`-- expect:error --`**: Marks the command as expected to fail (non-zero exit code) and validates error output
- **`-- expect:jsonpath --`**: Validates JSON output (stdout) using JSON path queries with comparisons

Unknown section names and empty sections are parse errors: a silently dropped
expectation would make a test pass without asserting anything.

## Template Variables

The following variables are automatically substituted, in commands and
in expectation patterns:

- `{{pod}}` or `{{pod[0]}}` - First pod name
- `{{pod[1]}}` - Second pod name (for multi-pod tests)
- `{{deployment}}` - Deployment name (test-actuator-app)
- `{{namespace}}` - Test namespace (default)

Anything that still looks like a `{{template}}` after substitution (a typo,
or a pod index beyond the pod count) fails the test with a harness error.

## Expectations

- Each `expect` section is a separate assertion
- An `expect` block can span multiple lines (treated as one substring match)
- Multiple `expect` blocks = multiple independent assertions
- Expectations apply to the **immediately preceding command**
- You can interleave commands and expectations to validate output at each step

## Example

```
-- test: logger list --
-- command --
kubectl-actuator --pod {{pod}} logger
-- expect --
ROOT
-- expect --
com.example.testapp

-- test: set and verify logger level --
-- command --
kubectl-actuator --pod {{pod}} logger com.example.testapp DEBUG
-- command --
kubectl-actuator --pod {{pod}} logger
-- expect --
DEBUG

-- test: interleaved commands and expectations --
-- command --
kubectl-actuator --pod {{pod}} info
-- expect --
test-actuator-app
-- command --
kubectl-actuator --pod {{pod}} logger
-- expect --
ROOT
-- command --
kubectl-actuator --pod {{pod}} scheduledtasks
-- expect --
cron
```

In the interleaved example:
- First command checks info endpoint, expects to find "test-actuator-app"
- Second command checks logger endpoint, expects to find "ROOT"
- Third command checks scheduledtasks endpoint, expects to find "cron"

## Testing Error Scenarios

Use `expect:error` to test commands that are expected to fail:

```
-- test: invalid context --
-- command --
kubectl-actuator --context invalid-context --pod {{pod}} health
-- expect:error --
Error:
-- expect:error --
context "invalid-context" does not exist
```

When using `expect:error`:
- The command must return a non-zero exit code, otherwise the test fails
- Expectations validate the error output (stdout + stderr combined)
- You can have multiple `expect:error` assertions to validate different parts of the error message
- `expect:error` checks combined output; pair it with `expect:stderr` when the stream matters

## JSON Path Validation

Use `expect:jsonpath` for robust JSON field validation using [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md):

### Field Existence
Check if a field exists and has a truthy value (not `false` or `null`):
```
-- test: health status exists --
-- command --
kubectl-actuator --pod {{pod}} health
-- expect:jsonpath --
status
-- expect:jsonpath --
components.diskSpace
```

### Exact Value Match
Validate a field has the expected value:
```
-- test: health is UP --
-- command --
kubectl-actuator --pod {{pod}} health
-- expect:jsonpath --
status == UP
```

The `==` needs a space on each side; a bare `status==UP` is rejected.

### Nested Fields
Access fields at any depth:
```
-- test: nested field access --
-- command --
kubectl-actuator --pod {{pod}} health
-- expect:jsonpath --
components.diskSpace.status == UP
```

### Array Access
Use gjson's array syntax:
```
-- test: metrics measurement --
-- command --
kubectl-actuator --pod {{pod}} metrics jvm.memory.used
-- expect:jsonpath --
measurements.0.statistic == VALUE
```

### Examples

**String values:**
```
-- expect:jsonpath --
app.name == test-actuator-app
```

**Numeric values (as strings):**
```
-- expect:jsonpath --
build.version == 1.0.0
```

**Boolean values:**
```
-- expect:jsonpath --
enabled == true
```

### When to Use Each Method

- **`expect:jsonpath`** - Validate JSON fields exist or have specific values
- **`expect`** - Simple substring checks, works on any output
- **`expect:regex`** - Pattern matching for flexible validation

## File Organization

Tests are grouped by topic, one file per area (for example `logger-tests.txt`
or `multi-pod-tests.txt`); the runner discovers every `.txt` file in this
directory automatically.

## Running Tests

The integration tests require Docker: they start a k3s cluster via testcontainers and build the Spring Boot test app image locally.

```bash
cd test
go test -v
```

The test runner will automatically discover and run all `.txt` files in this directory.
