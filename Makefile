# =============================================================================
# LocalGo Makefile
# =============================================================================

# ---------------------------------------------------------------------------
# Variables
# ---------------------------------------------------------------------------
BINARY_NAME    := localgo-cli
BUILD_DIR      := ./cmd/localgo-cli
DIST_DIR       := dist
COVER_DIR      := .coverage

GO             := go
GOFLAGS        ?=

VERSION        := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE     := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LD_BASE        := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)
LDFLAGS        := -ldflags "$(LD_BASE)"
LDFLAGS_STRIP  := -ldflags "-s -w $(LD_BASE)"

# Cross-compile targets (GOOS/GOARCH pairs)
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	android/arm \
	android/arm64

# Colour helpers — silently degrade when not a tty
ifeq ($(TERM),)
  _BOLD   :=
  _RESET  :=
  _GREEN  :=
  _CYAN   :=
  _YELLOW :=
else
  _BOLD   := $(shell tput bold    2>/dev/null)
  _RESET  := $(shell tput sgr0    2>/dev/null)
  _GREEN  := $(shell tput setaf 2 2>/dev/null)
  _CYAN   := $(shell tput setaf 6 2>/dev/null)
  _YELLOW := $(shell tput setaf 3 2>/dev/null)
endif

define log
  @printf '%s==> %s%s\n' '$(_BOLD)$(_CYAN)' '$(1)' '$(_RESET)'
endef

# ---------------------------------------------------------------------------
# Default
# ---------------------------------------------------------------------------
.DEFAULT_GOAL := help

.PHONY: all
all: fmt vet build ## Format, vet, and build

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
##@ Build

.PHONY: build
build: ## Build the binary for the current platform
	$(call log,Building $(BINARY_NAME) $(VERSION))
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) $(BUILD_DIR)
	@printf '%s  binary: ./%s%s\n' '$(_GREEN)' '$(BINARY_NAME)' '$(_RESET)'

.PHONY: build-fast
build-fast: ## Build without ldflags (faster iteration, skips version embedding)
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) $(BUILD_DIR)

.PHONY: release
release: clean-dist ## Cross-compile for all platforms into dist/
	$(call log,Cross-compiling release binaries)
	@mkdir -p $(DIST_DIR)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; \
		arch=$${p#*/}; \
		out_os=$$os; \
		out_arch=$$arch; \
		ext=""; \
		env=""; \
		[ "$$os" = "darwin" ] && out_os="macos"; \
		[ "$$os" = "windows" ] && ext=".exe"; \
		if [ "$$os" = "android" ]; then \
			if [ "$$arch" = "arm" ]; then \
				out_arch="armv7"; env="GOARM=7"; \
				os="linux"; \
			elif [ "$$arch" = "arm64" ]; then \
				out_arch="armv8"; \
			fi; \
		fi; \
		out="$(DIST_DIR)/$(BINARY_NAME)-$${out_os}-$${out_arch}$${ext}"; \
		printf '  %-40s' "$$p"; \
		env $$env GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build $(LDFLAGS_STRIP) -o "$$out" $(BUILD_DIR) \
		&& printf '%sdone%s\n' '$(_GREEN)' '$(_RESET)' \
		|| printf '%sFAILED%s\n' '$(_YELLOW)' '$(_RESET)'; \
	done
	@ls -lh $(DIST_DIR)/

.PHONY: install
install: ## Install binary to $(GOPATH)/bin
	$(call log,Installing $(BINARY_NAME))
	$(GO) install $(GOFLAGS) $(LDFLAGS) $(BUILD_DIR)

# ---------------------------------------------------------------------------
# Dependencies
# ---------------------------------------------------------------------------
##@ Dependencies

.PHONY: deps
deps: ## Download and tidy Go modules
	$(call log,Syncing dependencies)
	$(GO) mod download
	$(GO) mod tidy

.PHONY: deps-upgrade
deps-upgrade: ## Upgrade all direct dependencies to latest
	$(call log,Upgrading dependencies)
	$(GO) get -u ./...
	$(GO) mod tidy

# ---------------------------------------------------------------------------
# Development
# ---------------------------------------------------------------------------
##@ Development

.PHONY: dev
dev: ## Live-reload dev server (requires: go install github.com/air-verse/air@latest)
	$(call log,Starting dev server with live reload)
	@command -v air >/dev/null 2>&1 || { \
	  printf '%s air not found — install with: go install github.com/air-verse/air@latest%s\n' \
	    '$(_YELLOW)' '$(_RESET)'; exit 1; }
	air

.PHONY: run
run: build ## Build then run the server (HTTPS)
	./$(BINARY_NAME) serve

.PHONY: run-http
run-http: build ## Build then run the server (HTTP, no TLS)
	./$(BINARY_NAME) serve --http

# ---------------------------------------------------------------------------
# Code quality
# ---------------------------------------------------------------------------
##@ Code quality

.PHONY: fmt
fmt: ## Format all Go source files
	$(call log,Formatting)
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(call log,Vetting)
	$(GO) vet ./...

.PHONY: lint
lint: ## Run golangci-lint (install: https://golangci-lint.run/usage/install/)
	$(call log,Linting)
	@command -v golangci-lint >/dev/null 2>&1 || { \
	  printf '%s golangci-lint not found — see https://golangci-lint.run/usage/install/%s\n' \
	    '$(_YELLOW)' '$(_RESET)'; exit 1; }
	golangci-lint run

.PHONY: check
check: fmt vet lint ## Run all code-quality checks (fmt + vet + lint)

# ---------------------------------------------------------------------------
# Testing
# ---------------------------------------------------------------------------
##@ Testing

.PHONY: test
test: ## Run all tests with race detector
	$(call log,Testing)
	$(GO) test -race -timeout 60s ./...

.PHONY: test-short
test-short: ## Run tests, skipping slow ones (-short)
	$(GO) test -short -race -timeout 30s ./...

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	$(GO) test -v -race -timeout 60s ./...

.PHONY: test-pkg
test-pkg: ## Run unit tests for pkg/ only
	$(call log,Testing pkg/)
	$(GO) test -race -timeout 60s ./pkg/...

.PHONY: test-cover
test-cover: ## Run tests and generate HTML coverage report
	$(call log,Coverage)
	@mkdir -p $(COVER_DIR)
	$(GO) test -race -timeout 60s \
	  -coverprofile=$(COVER_DIR)/coverage.out \
	  -covermode=atomic ./...
	$(GO) tool cover -html=$(COVER_DIR)/coverage.out -o $(COVER_DIR)/coverage.html
	@printf '%s  report: %s/coverage.html%s\n' '$(_GREEN)' '$(COVER_DIR)' '$(_RESET)'

.PHONY: test-cover-func
test-cover-func: test-cover ## Show per-function coverage in the terminal
	$(GO) tool cover -func=$(COVER_DIR)/coverage.out

.PHONY: bench
bench: ## Run benchmarks
	$(GO) test -run='^$$' -bench=. -benchmem ./...

# ---------------------------------------------------------------------------
# CI
# ---------------------------------------------------------------------------
##@ CI

.PHONY: ci
ci: deps check test build ## Full CI pipeline: deps → check → test → build

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------
##@ Clean

.PHONY: clean
clean: ## Remove local binary and coverage artefacts
	@rm -f $(BINARY_NAME)
	@rm -rf $(COVER_DIR)
	$(GO) clean -testcache

.PHONY: clean-dist
clean-dist: ## Remove dist/ directory
	@rm -rf $(DIST_DIR)

.PHONY: clean-all
clean-all: clean clean-dist ## Remove everything (binary, coverage, dist, go build cache)
	$(GO) clean -cache

# ---------------------------------------------------------------------------
# Docker / Podman
# ---------------------------------------------------------------------------
##@ Docker / Podman

CONTAINER_IMAGE ?= localgo:latest

.PHONY: docker-build
docker-build: ## Build Docker image
	$(call log,Building Docker image $(CONTAINER_IMAGE))
	docker build -t $(CONTAINER_IMAGE) .

.PHONY: docker-run
docker-run: ## Run Docker container in the foreground
	docker run --rm --network=host \
	  -v "$$(pwd)/downloads:/app/downloads" \
	  -v "$$(pwd)/config:/app/config" \
	  $(CONTAINER_IMAGE)

.PHONY: docker-up
docker-up: ## Start via Docker Compose (detached)
	PUID=$$(id -u) PGID=$$(id -g) docker compose up -d

.PHONY: docker-down
docker-down: ## Stop Docker Compose stack
	docker compose down

.PHONY: docker-logs
docker-logs: ## Follow Docker Compose logs
	docker compose logs -f

.PHONY: docker-clean
docker-clean: docker-down ## Stop stack and remove the image
	docker rmi $(CONTAINER_IMAGE) 2>/dev/null || true

.PHONY: podman-build
podman-build: ## Build Podman image (rootless)
	$(call log,Building Podman image $(CONTAINER_IMAGE))
	podman build -f Containerfile -t $(CONTAINER_IMAGE) .

.PHONY: podman-run
podman-run: ## Run Podman container (rootless, foreground)
	podman run --rm --userns=keep-id --network=host \
	  -v "$$(pwd)/downloads:/app/downloads" \
	  -v "$$(pwd)/config:/app/config" \
	  $(CONTAINER_IMAGE)

# ---------------------------------------------------------------------------
# Utilities
# ---------------------------------------------------------------------------
##@ Utilities

.PHONY: discover
discover: build ## Discover LocalGo devices on the network
	./$(BINARY_NAME) discover

.PHONY: scan
scan: build ## Scan network for LocalGo devices (HTTP)
	./$(BINARY_NAME) scan

.PHONY: send
send: build ## Send a file  →  make send FILE=<path> TO=<alias>
	@test -n "$(FILE)" || { echo 'Usage: make send FILE=<path> TO=<alias>'; exit 1; }
	@test -n "$(TO)"   || { echo 'Usage: make send FILE=<path> TO=<alias>'; exit 1; }
	./$(BINARY_NAME) send --file "$(FILE)" --to "$(TO)"

.PHONY: version
version: build ## Print the binary version string
	@./$(BINARY_NAME) version

.PHONY: info
info: ## Print Makefile build variables
	@printf '%sBuild info%s\n' '$(_BOLD)' '$(_RESET)'
	@printf '  %-16s %s\n' 'Binary'   '$(BINARY_NAME)'
	@printf '  %-16s %s\n' 'Version'  '$(VERSION)'
	@printf '  %-16s %s\n' 'Commit'   '$(GIT_COMMIT)'
	@printf '  %-16s %s\n' 'Date'     '$(BUILD_DATE)'
	@printf '  %-16s %s\n' 'GOFLAGS'  '$(GOFLAGS)'
	@printf '  %-16s %s\n' 'LDFLAGS'  '$(LDFLAGS)'

# ---------------------------------------------------------------------------
# Help  (auto-generated from ## / ##@ comments)
# ---------------------------------------------------------------------------
##@ Help

.PHONY: help
help: ## Show this help
	@printf '\n%sLocalGo — available targets%s\n' '$(_BOLD)' '$(_RESET)'
	@awk 'BEGIN {FS = ":.*##"} \
	  /^##@/ { printf "\n%s%s%s\n", "$(_BOLD)", substr($$0, 5), "$(_RESET)"; next } \
	  /^[a-zA-Z0-9_-]+:.*##/ { \
	    printf "  %s%-20s%s %s\n", "$(_CYAN)", $$1, "$(_RESET)", $$2 }' \
	  $(MAKEFILE_LIST)
	@printf '\n%sVariables you can override:%s\n' '$(_BOLD)' '$(_RESET)'
	@printf '  %s%-20s%s %s\n' '$(_CYAN)' 'GOFLAGS'         '$(_RESET)' 'Extra flags for go build / go test'
	@printf '  %s%-20s%s %s\n' '$(_CYAN)' 'CONTAINER_IMAGE' '$(_RESET)' 'Docker/Podman image tag  (default: localgo:latest)'
	@printf '  %s%-20s%s %s\n' '$(_CYAN)' 'FILE / TO'       '$(_RESET)' 'Required by: make send'
	@printf '\n'
