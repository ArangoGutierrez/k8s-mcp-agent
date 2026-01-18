# Copyright 2026 k8s-gpu-mcp-server contributors
# SPDX-License-Identifier: Apache-2.0

# Core utilities
DOCKER   ?= docker
MKDIR    ?= mkdir
DIST_DIR ?= $(CURDIR)/dist
BIN_DIR  ?= $(CURDIR)/bin

include $(CURDIR)/versions.mk

MODULE := github.com/ArangoGutierrez/k8s-gpu-mcp-server

# Registry configuration
ifeq ($(IMAGE_NAME),)
REGISTRY ?= ghcr.io/arangogutierrez
IMAGE_NAME = $(REGISTRY)/k8s-gpu-mcp-server
endif

BUILDIMAGE_TAG ?= golang$(GOLANG_VERSION)
BUILDIMAGE ?= $(IMAGE_NAME)-build:$(BUILDIMAGE_TAG)

# Version injection
ifeq ($(VERSION),)
CLI_VERSION = $(LIB_VERSION)$(if $(LIB_TAG),-$(LIB_TAG))
else
CLI_VERSION = $(VERSION)
endif
CLI_VERSION_PACKAGE = $(MODULE)/internal/info

# Commands
CMDS := $(patsubst ./cmd/%/,%,$(sort $(dir $(wildcard ./cmd/*/))))
CMD_TARGETS := $(patsubst %,cmd-%, $(CMDS))

# Check targets
CHECK_TARGETS := lint vet fmt-check
MAKE_TARGETS := all build binaries cmds check test coverage fmt goimports \
                licenses vendor mod-tidy mod-download clean help

TARGETS := $(MAKE_TARGETS) $(CMD_TARGETS)
DOCKER_TARGETS := $(patsubst %,docker-%, $(TARGETS))

.PHONY: $(TARGETS) $(DOCKER_TARGETS)

# Default target
.DEFAULT_GOAL := help

##@ General

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

all: check test build binaries ## Run all checks, tests, and build

##@ Build

build: ## Build all Go packages
	go build ./...

binaries: cmds ## Build all binaries (alias for cmds)

ifneq ($(PREFIX),)
cmd-%: COMMAND_BUILD_OPTIONS = -o $(PREFIX)/$(*)
else
cmd-%: COMMAND_BUILD_OPTIONS = -o $(BIN_DIR)/$(*)
endif

# CGO is required for NVML bindings
export CGO_ENABLED=1

# Platform-specific linker flags for NVML dynamic loading
ifneq ($(shell uname),Darwin)
EXTLDFLAGS = -Wl,--export-dynamic -Wl,--unresolved-symbols=ignore-in-object-files -Wl,-z,lazy
else
EXTLDFLAGS = -Wl,-undefined,dynamic_lookup
endif

cmds: $(CMD_TARGETS) ## Build all command binaries

$(CMD_TARGETS): cmd-%:
	@$(MKDIR) -p $(BIN_DIR)
	go build \
		-ldflags "-s -w '-extldflags=$(EXTLDFLAGS)' \
		-X $(CLI_VERSION_PACKAGE).gitCommit=$(GIT_COMMIT) \
		-X $(CLI_VERSION_PACKAGE).version=$(CLI_VERSION)" \
		$(COMMAND_BUILD_OPTIONS) \
		$(MODULE)/cmd/$(*)
	@echo "✓ Built $(BIN_DIR)/$(*)"

agent: cmd-agent ## Build agent binary (convenience target)

##@ Code Quality

check: $(CHECK_TARGETS) ## Run all code quality checks

fmt: ## Apply gofmt formatting to all Go files
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs gofmt -s -l -w
	@echo "✓ Code formatted"

fmt-check: ## Check if code is formatted (CI-friendly)
	@UNFORMATTED=$$(gofmt -s -l . 2>&1); \
	if [ -n "$$UNFORMATTED" ]; then \
		echo "✗ Code is not formatted. Run 'make fmt'"; \
		echo "$$UNFORMATTED"; \
		exit 1; \
	fi
	@echo "✓ Code formatting check passed"

goimports: ## Apply goimports with local module prefix
	@if ! command -v goimports >/dev/null 2>&1; then \
		echo "Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	fi
	go list -f '{{.Dir}}' $(MODULE)/... \
		| xargs goimports -local $(MODULE) -w
	@echo "✓ Imports organized"

vet: ## Run go vet static analysis
	go vet ./...
	@echo "✓ go vet passed"

lint: ## Run golangci-lint
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
			sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi
	golangci-lint run ./...
	@echo "✓ Linting passed"

##@ Testing

COVERAGE_FILE := coverage.out

test: build ## Run unit tests
	go test -v -count=1 -race ./...
	@echo "✓ Tests passed"

test-short: ## Run unit tests without race detector (faster)
	go test -v -count=1 -short ./...

test-integration: build ## Run integration tests (requires GPU)
	go test -v -count=1 -tags=integration ./...
	@echo "✓ Integration tests passed"

coverage: ## Run tests with coverage report
	go test -coverprofile=$(COVERAGE_FILE) ./...
	go tool cover -func=$(COVERAGE_FILE)
	@echo "✓ Coverage report generated: $(COVERAGE_FILE)"

coverage-html: coverage ## Generate HTML coverage report
	go tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "✓ HTML coverage report: coverage.html"

##@ Dependencies

mod-download: ## Download Go module dependencies
	go mod download
	@echo "✓ Dependencies downloaded"

mod-tidy: ## Tidy Go module dependencies
	go mod tidy
	@echo "✓ go.mod tidied"

mod-verify: ## Verify Go module dependencies
	go mod verify
	@echo "✓ Modules verified"

vendor: mod-tidy mod-download ## Vendor dependencies (if needed)
	go mod vendor
	@echo "✓ Dependencies vendored"

check-vendor: vendor ## Check if vendor directory is up to date
	git diff --exit-code HEAD -- go.mod go.sum vendor

licenses: ## Generate license report
	@if ! command -v go-licenses >/dev/null 2>&1; then \
		echo "Installing go-licenses..."; \
		go install github.com/google/go-licenses@latest; \
	fi
	go-licenses csv $(MODULE)/...

##@ Container

DOCKERFILE ?= $(CURDIR)/deployment/Containerfile
IMAGE_TAG ?= $(CLI_VERSION)

image: ## Build container image
	$(DOCKER) build \
		-f $(DOCKERFILE) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		--build-arg VERSION=$(CLI_VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		.
	@echo "✓ Image built: $(IMAGE_NAME):$(IMAGE_TAG)"

image-push: image ## Push container image to registry
	$(DOCKER) push $(IMAGE_NAME):$(IMAGE_TAG)
	$(DOCKER) push $(IMAGE_NAME):latest
	@echo "✓ Image pushed: $(IMAGE_NAME):$(IMAGE_TAG)"

##@ Code Generation

generate: ## Run go generate
	go generate ./...
	@echo "✓ Code generated"

##@ Cleanup

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR) $(DIST_DIR) $(COVERAGE_FILE) coverage.html
	@echo "✓ Cleaned build artifacts"

clean-all: clean ## Clean all generated files including vendor
	rm -rf vendor
	@echo "✓ Cleaned all generated files"

##@ Docker-based Builds

# Generate an image for containerized builds
.PHONY: .build-image
.build-image:
	$(DOCKER) build \
		-f $(CURDIR)/deployment/Containerfile.build \
		-t $(BUILDIMAGE) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		.

$(DOCKER_TARGETS): docker-%: .build-image
	@echo "Running 'make $(*)' in container image $(BUILDIMAGE)"
	$(DOCKER) run \
		--rm \
		-e GOCACHE=/tmp/.cache/go \
		-e GOMODCACHE=/tmp/.cache/gomod \
		-e GOLANGCI_LINT_CACHE=/tmp/.cache/golangci-lint \
		-v $(PWD):/work \
		-w /work \
		--user $$(id -u):$$(id -g) \
		$(BUILDIMAGE) \
			make $(*)

# Start an interactive shell using the development image
.PHONY: shell
shell: .build-image ## Start interactive shell in build container
	$(DOCKER) run \
		--rm \
		-ti \
		-e GOCACHE=/tmp/.cache/go \
		-e GOMODCACHE=/tmp/.cache/gomod \
		-e GOLANGCI_LINT_CACHE=/tmp/.cache/golangci-lint \
		-v $(PWD):/work \
		-w /work \
		--user $$(id -u):$$(id -g) \
		$(BUILDIMAGE)

##@ Development

run: agent ## Build and run the agent (stdio mode)
	$(BIN_DIR)/agent --mode=read-only

run-operator: agent ## Build and run the agent (operator mode)
	$(BIN_DIR)/agent --mode=operator

watch: ## Watch for changes and rebuild (requires entr)
	@if ! command -v entr >/dev/null 2>&1; then \
		echo "✗ entr not found. Install with: brew install entr (macOS) or apt install entr (Ubuntu)"; \
		exit 1; \
	fi
	@echo "Watching for changes... (Ctrl+C to stop)"
	find . -name '*.go' | entr -r make agent

##@ Release

dist: clean ## Build release binaries for all platforms
	@$(MKDIR) -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(MAKE) cmd-agent PREFIX=$(DIST_DIR)/linux-amd64
	GOOS=linux GOARCH=arm64 $(MAKE) cmd-agent PREFIX=$(DIST_DIR)/linux-arm64
	@echo "✓ Release binaries built in $(DIST_DIR)"

dist-checksums: dist ## Generate checksums for release binaries
	cd $(DIST_DIR) && \
		find . -type f -name 'agent' -exec sha256sum {} \; > SHA256SUMS
	@echo "✓ Checksums generated: $(DIST_DIR)/SHA256SUMS"

##@ Validation

.PHONY: validate-rbac
validate-rbac: ## Validate RBAC manifests (dry-run)
	@echo "Validating standalone RBAC manifests..."
	@kubectl apply --dry-run=client -f deployment/rbac/ 2>&1 || \
		(echo "✗ Standalone RBAC validation failed"; exit 1)
	@echo "✓ Standalone RBAC manifests valid"
	@echo "Validating Helm RBAC templates (agent.rbac.create=true)..."
	@helm template test ./deployment/helm/k8s-gpu-mcp-server \
		--set agent.rbac.create=true 2>/dev/null | \
		kubectl apply --dry-run=client -f - 2>&1 || \
		(echo "✗ Helm agent RBAC validation failed"; exit 1)
	@echo "✓ Helm agent RBAC templates valid"
	@echo "Validating Helm RBAC templates (gateway.enabled=true)..."
	@helm template test ./deployment/helm/k8s-gpu-mcp-server \
		--set gateway.enabled=true 2>/dev/null | \
		kubectl apply --dry-run=client -f - 2>&1 || \
		(echo "✗ Helm gateway RBAC validation failed"; exit 1)
	@echo "✓ Helm gateway RBAC templates valid"
	@echo "Validating Helm RBAC templates (operator mode)..."
	@helm template test ./deployment/helm/k8s-gpu-mcp-server \
		--set agent.mode=operator \
		--set agent.rbac.create=true 2>/dev/null | \
		kubectl apply --dry-run=client -f - 2>&1 || \
		(echo "✗ Helm operator RBAC validation failed"; exit 1)
	@echo "✓ Helm operator RBAC templates valid"
	@echo "✓ All RBAC validations passed"

##@ E2E Testing

test-e2e: build ## Run E2E tests (requires kind, helm, kubectl)
	E2E_TEST=1 go test -v -timeout=10m ./test/e2e/...

test-e2e-short: build ## Run E2E tests without resilience tests
	E2E_TEST=1 go test -v -timeout=10m -short ./test/e2e/...

test-e2e-setup: ## Setup Kind cluster for E2E tests
	@if ! kind get clusters 2>/dev/null | grep -q '^e2e-gpu-mcp$$'; then \
		echo "Creating Kind cluster 'e2e-gpu-mcp'..."; \
		kind create cluster --name e2e-gpu-mcp --config test/e2e/testdata/kind-config.yaml --wait 120s; \
	else \
		echo "Kind cluster 'e2e-gpu-mcp' already exists, skipping create"; \
	fi
	@if ! helm list -n gpu-diagnostics -q 2>/dev/null | grep -q '^e2e-test$$'; then \
		echo "Installing Helm release 'e2e-test'..."; \
		helm install e2e-test deployment/helm/k8s-gpu-mcp-server \
			--namespace gpu-diagnostics \
			--create-namespace \
			--set agent.nvmlMode=mock \
			--set gateway.enabled=true \
			--set gpu.runtimeClass.enabled=false \
			--set gpu.resourceRequest.enabled=false \
			--wait --timeout 180s; \
	else \
		echo "Helm release 'e2e-test' already exists, skipping install"; \
	fi
	@echo "✓ E2E environment ready"

test-e2e-teardown: ## Teardown Kind cluster
	kind delete cluster --name e2e-gpu-mcp
	@echo "✓ E2E environment cleaned up"

##@ Information

info: ## Display build information
	@echo "Module:        $(MODULE)"
	@echo "Version:       $(CLI_VERSION)"
	@echo "Git Commit:    $(GIT_COMMIT)"
	@echo "Git Tag:       $(GIT_TAG)"
	@echo "Go Version:    $(shell go version)"
	@echo "Build Image:   $(BUILDIMAGE)"
	@echo "Image Name:    $(IMAGE_NAME):$(IMAGE_TAG)"
	@echo "CGO Enabled:   $(CGO_ENABLED)"

