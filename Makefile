.PHONY: build lint test test-integration start-testenvironment clean help

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

# Run static analysis (vet + gofmt) on both modules
lint:
	go vet ./...
	cd test && go vet ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

# Run unit tests
test:
	go test -v ./...

# Run integration tests (requires Docker). The harness builds its own binary
# (test/kubectl-actuator), so this deliberately does not depend on build.
test-integration:
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
	@echo "  lint                   - Run go vet and gofmt checks"
	@echo "  test                   - Run unit tests"
	@echo "  test-integration       - Run integration tests (requires Docker)"
	@echo "  start-testenvironment  - Start manual test environment (Ctrl+C to stop)"
	@echo "  clean                  - Clean build artifacts"
	@echo "  help                   - Show this help message"
