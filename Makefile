.PHONY: build test test-integration start-testenvironment clean help

# Version information
VERSION ?= dev
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X github.com/deviceinsight/kubectl-actuator/internal/cmd.Version=$(VERSION) \
           -X github.com/deviceinsight/kubectl-actuator/internal/cmd.GitCommit=$(GIT_COMMIT) \
           -X github.com/deviceinsight/kubectl-actuator/internal/cmd.BuildDate=$(BUILD_DATE)

# Build the kubectl-actuator binary
build:
	go build -ldflags "$(LDFLAGS)" -o kubectl-actuator .

# Run unit tests
test:
	go test -v ./...

# Build Spring Boot test app and run integration tests
test-integration: build
	cd test && go test -v -timeout 15m

# Start a manual test environment (blocks until Ctrl+C)
start-testenvironment:
	cd test && go run cmd/manual_env/main.go

# Clean build artifacts
clean:
	rm -f kubectl-actuator
	rm -f test/kubectl-actuator
	cd test/spring-app && mvn clean || true

# Show help
help:
	@echo "Available targets:"
	@echo "  build                  - Build kubectl-actuator binary"
	@echo "  test                   - Run unit tests"
	@echo "  test-integration       - Build test app and run integration tests"
	@echo "  start-testenvironment  - Start manual test environment (Ctrl+C to stop)"
	@echo "  clean                  - Clean build artifacts"
	@echo "  help                   - Show this help message"
