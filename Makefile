# LocalGo Makefile

# Variables
BINARY_NAME = localgo-cli
BINARY_PATH = /tmp/$(BINARY_NAME)
BUILD_DIR = cmd/localgo-cli
GO_FILES = $(shell find . -type f -name '*.go')
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)"

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build: $(BINARY_PATH)

$(BINARY_PATH): $(GO_FILES)
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_PATH) ./$(BUILD_DIR)
	@echo "Binary built: $(BINARY_PATH)"

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Run tests
.PHONY: test
test: build
	@echo "Running tests..."
	go test -v -timeout 60s ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage: build
	@echo "Running tests with coverage..."
	go test -v -timeout 60s -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run integration tests specifically
.PHONY: test-integration
test-integration: build
	@echo "Running integration tests..."
	go test -v -timeout 60s ./cmd/localgo-cli/

# Run unit tests only
.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	go test -v ./pkg/...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_PATH)
	go clean -cache
	go clean -testcache

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	@echo "Linting code..."
	golangci-lint run

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	go vet ./...

# Run code quality checks
.PHONY: check
check: fmt vet lint

# Install the binary to GOPATH/bin
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./$(BUILD_DIR)

# Run the server
.PHONY: serve
serve: build
	@echo "Starting LocalGo server..."
	$(BINARY_PATH) serve

# Run the server with HTTP (for testing)
.PHONY: serve-http
serve-http: build
	@echo "Starting LocalGo server with HTTP..."
	$(BINARY_PATH) serve --http

# Discover devices
.PHONY: discover
discover: build
	@echo "Discovering devices..."
	$(BINARY_PATH) discover

# Scan network
.PHONY: scan
scan: build
	@echo "Scanning network..."
	$(BINARY_PATH) scan

# Send a test file (requires FILE and TO variables)
.PHONY: send
send: build
	@if [ -z "$(FILE)" ] || [ -z "$(TO)" ]; then \
		echo "Usage: make send FILE=<file_path> TO=<device_alias>"; \
		exit 1; \
	fi
	@echo "Sending file $(FILE) to $(TO)..."
	$(BINARY_PATH) send --file $(FILE) --to $(TO)

# Create a test file for sending
.PHONY: create-test-file
create-test-file:
	@echo "Creating test file..."
	echo "Hello from LocalGo!" > /tmp/test-file.txt
	@echo "Test file created: /tmp/test-file.txt"

# Run a full test cycle (build, test, clean)
.PHONY: ci
ci: deps check test clean

# Show help
.PHONY: help
help:
	@echo "LocalGo Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Build Targets:"
	@echo "  build              Build the CLI binary"
	@echo "  install            Install binary to GOPATH/bin"
	@echo "  clean              Clean build artifacts"
	@echo ""
	@echo "Development Targets:"
	@echo "  deps               Install dependencies"
	@echo "  fmt                Format code"
	@echo "  lint               Lint code (requires golangci-lint)"
	@echo "  vet                Vet code"
	@echo "  check              Run all code quality checks"
	@echo ""
	@echo "Testing Targets:"
	@echo "  test               Run all tests"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  test-integration   Run integration tests only"
	@echo "  test-unit          Run unit tests only"
	@echo ""
	@echo "Runtime Targets:"
	@echo "  serve              Run the server (HTTPS)"
	@echo "  serve-http         Run the server (HTTP)"
	@echo "  discover           Discover devices"
	@echo "  scan               Scan network"
	@echo "  send               Send file (requires FILE and TO)"
	@echo ""
	@echo "Utility Targets:"
	@echo "  create-test-file   Create a test file"
	@echo "  info               Show build information"
	@echo "  version            Show version"
	@echo "  ci                 Run CI pipeline"
	@echo "  help               Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make test-coverage"
	@echo "  make serve-http"
	@echo "  make send FILE=/tmp/test-file.txt TO=MyDevice"
	@echo ""
	@echo "Environment Variables:"
	@echo "  LOCALSEND_ALIAS          Device alias"
	@echo "  LOCALSEND_PORT           Default port"
	@echo "  LOCALSEND_DOWNLOAD_DIR   Download directory"

# Show version information
.PHONY: version
version: build
	@$(BINARY_PATH) version

# Show build information
.PHONY: info
info:
	@echo "Build Information:"
	@echo "  Binary Name: $(BINARY_NAME)"
	@echo "  Binary Path: $(BINARY_PATH)"
	@echo "  Build Dir:   $(BUILD_DIR)"
	@echo "  Version:     $(VERSION)"
	@echo "  Git Commit:  $(GIT_COMMIT)"
	@echo "  Build Date:  $(BUILD_DATE)"
	@echo "  LDFLAGS:     $(LDFLAGS)"
	@echo "  Go Files:    $(words $(GO_FILES)) files"

# Debug target to show variables
.PHONY: debug
debug: info
	@echo ""
	@echo "Detailed Information:"
	@echo "  BINARY_NAME: $(BINARY_NAME)"
	@echo "  BINARY_PATH: $(BINARY_PATH)"
	@echo "  BUILD_DIR: $(BUILD_DIR)"
	@echo "  VERSION: $(VERSION)"
	@echo "  GIT_COMMIT: $(GIT_COMMIT)"
	@echo "  BUILD_DATE: $(BUILD_DATE)"
	@echo "  LDFLAGS: $(LDFLAGS)"
	@echo "  GO_FILES: $(GO_FILES)"

# Quick demo - start server and send a test file
.PHONY: demo
demo: build create-test-file
	@echo "Starting demo..."
	@echo "This will start a server and send a test file to itself"
	@echo "Starting server in background..."
	@LOCALSEND_ALIAS=DemoDevice $(BINARY_PATH) serve --http --port 8080 &
	@sleep 3
	@echo "Sending test file..."
	@LOCALSEND_ALIAS=DemoSender $(BINARY_PATH) send --file /tmp/test-file.txt --to DemoDevice --port 8080 || true
	@echo "Demo complete"
	@pkill -f "$(BINARY_PATH) serve" || true
