# =============================================================================
# KeyIP-Intelligence - Project Build Automation
# =============================================================================
# Usage:
#   make <target>
#
# Targets are grouped by function. Run `make help` to see all targets.
# =============================================================================

# -----------------------------------------------------------------------------
# Variables
# -----------------------------------------------------------------------------

GO              := go
GOFLAGS         := -mod=mod
BINARY_DIR      := bin
MODULE          := github.com/turtacn/KeyIP-Intelligence
VERSION         := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH      := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_TIME      := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GO_VERSION      := $(shell go version | cut -d' ' -f3)
LDFLAGS         := -s -w \
                   -X $(MODULE)/internal/config.Version=$(VERSION) \
                   -X $(MODULE)/internal/config.CommitSHA=$(GIT_COMMIT) \
                   -X $(MODULE)/internal/config.BuildTime=$(BUILD_TIME) \
                   -X $(MODULE)/internal/config.GoVersion=$(GO_VERSION)

# Build targets
CMD_APISERVER   := cmd/apiserver
CMD_WORKER      := cmd/worker
CMD_KEYIP       := cmd/keyip
BIN_APISERVER   := $(BINARY_DIR)/apiserver
BIN_WORKER      := $(BINARY_DIR)/worker
BIN_KEYIP       := $(BINARY_DIR)/keyip

# Test settings
TEST_FLAGS      := -v -race -count=1
COVERAGE_FILE   := coverage.out
COVERAGE_HTML   := coverage.html
TEST_TIMEOUT    := 120s
INT_TEST_TIMEOUT := 300s
E2E_TEST_TIMEOUT := 600s

# Docker settings
DOCKER_REGISTRY  := ghcr.io/turtacn
IMAGE_APISERVER  := $(DOCKER_REGISTRY)/keyip-apiserver:$(VERSION)
IMAGE_WORKER     := $(DOCKER_REGISTRY)/keyip-worker:$(VERSION)
DOCKER_COMPOSE   := docker-compose -f deployments/docker/docker-compose.yml

# Proto settings
PROTO_DIR        := api/proto/v1
PROTO_OUT        := internal/interfaces/grpc/generated

# Migration settings
MIGRATE_DSN      ?= $(shell grep -A5 'database:' configs/config.yaml | grep 'dsn:' | awk '{print $$2}' 2>/dev/null || echo "postgres://postgres:password@localhost:5432/keyip?sslmode=disable")
MIGRATIONS_DIR   := internal/infrastructure/database/postgres/migrations

# Linting
GOLANGCI_LINT    := $(shell which golangci-lint 2>/dev/null)
GOLANGCI_VERSION := v1.56.2

# Security scanning tools
GOVULNCHECK_VERSION := v1.1.3
GOSEC_VERSION       := v2.21.4

# Colors for output
RED    := \033[0;31m
GREEN  := \033[0;32m
YELLOW := \033[0;33m
BLUE   := \033[0;34m
NC     := \033[0m # No Color

# -----------------------------------------------------------------------------
# Default target
# -----------------------------------------------------------------------------

.DEFAULT_GOAL := help

# Phony targets declaration
.PHONY: all build build-apiserver build-worker build-keyip \
        test test-unit test-integration test-e2e test-e2e-report test-coverage test-race \
        lint fmt vet check \
        security-scan security-scan-go security-scan-code security-scan-frontend \
        install-security-tools \
        proto proto-clean proto-deps \
        migrate-up migrate-down migrate-down-all migrate-status migrate-create \
        docker-build docker-push docker-compose-up docker-compose-down docker-logs \
        clean clean-bin clean-cover clean-all \
        seed run-apiserver run-worker \
        check-tools install-tools deps tools \
        generate openapi mock \
        help

# -----------------------------------------------------------------------------
# Build Targets
# -----------------------------------------------------------------------------

## all: Build all binaries (default development build)
all: fmt vet build

## build: Compile all three binaries (apiserver, worker, keyip)
build: build-apiserver build-worker build-keyip

## build-apiserver: Compile the API server binary
build-apiserver:
	@echo "$(BLUE)>> Building apiserver...$(NC)"
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_APISERVER) ./$(CMD_APISERVER)/...
	@echo "$(GREEN)>> Built: $(BIN_APISERVER) (version=$(VERSION))$(NC)"

## build-worker: Compile the background worker binary
build-worker:
	@echo "$(BLUE)>> Building worker...$(NC)"
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_WORKER) ./$(CMD_WORKER)/...
	@echo "$(GREEN)>> Built: $(BIN_WORKER) (version=$(VERSION))$(NC)"

## build-cli: Compile the CLI binary
build-cli: build-keyip

## build-keyip: Compile the CLI binary
build-keyip:
	@echo "$(BLUE)>> Building keyip CLI...$(NC)"
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_KEYIP) ./$(CMD_KEYIP)/...
	@echo "$(GREEN)>> Built: $(BIN_KEYIP) (version=$(VERSION))$(NC)"

# -----------------------------------------------------------------------------
# Test Targets
# -----------------------------------------------------------------------------

## test: Run all unit tests
test: test-unit

## test-unit: Run unit tests for all packages (with race detection)
test-unit:
	@echo "$(BLUE)>> Running unit tests with race detection...$(NC)"
	bash scripts/test.sh --level unit --race --coverage
	@echo "$(GREEN)>> Unit tests passed.$(NC)"

## test-integration: Run integration tests (with race detection)
test-integration:
	@echo "$(BLUE)>> Running integration tests with race detection...$(NC)"
	bash scripts/test.sh --level integration --race
	@echo "$(GREEN)>> Integration tests passed.$(NC)"

## test-e2e: Run end-to-end tests (with race detection)
test-e2e:
	@echo "$(BLUE)>> Running E2E tests with race detection...$(NC)"
	bash scripts/test.sh --level e2e --race
	@echo "$(GREEN)>> E2E tests passed.$(NC)"

## test-e2e-report: Run E2E tests and generate a detailed summary report
test-e2e-report:
	@echo "$(BLUE)>> Running E2E tests and generating report...$(NC)"
	bash scripts/generate-test-report.sh
	@echo "$(GREEN)>> E2E test report generated.$(NC)"

## test-coverage: Generate and open HTML coverage report
test-coverage:
	@echo "$(BLUE)>> Generating coverage report...$(NC)"
	bash scripts/test-coverage.sh
	@echo "$(GREEN)>> Coverage report generated: $(COVERAGE_HTML)$(NC)"

## test-race: Run tests with race detection
test-race:
	@echo "$(BLUE)>> Running tests with race detection...$(NC)"
	bash scripts/test.sh --level unit --race

# -----------------------------------------------------------------------------
# Code Quality Targets
# -----------------------------------------------------------------------------

## fmt: Format all Go source files
fmt:
	@echo "$(BLUE)>> Formatting code...$(NC)"
	@which goimports > /dev/null 2>&1 || $(GO) install golang.org/x/tools/cmd/goimports@latest
	gofmt -s -w $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
	goimports -w -local $(MODULE) $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
	@echo "$(GREEN)>> Formatting complete.$(NC)"

## vet: Run go vet
vet:
	@echo "$(BLUE)>> Running go vet...$(NC)"
	$(GO) vet $(GOFLAGS) ./...
	@echo "$(GREEN)>> go vet passed.$(NC)"

## lint: Run golangci-lint
lint:
	@echo "$(BLUE)>> Running golangci-lint...$(NC)"
	@if [ -z "$(GOLANGCI_LINT)" ]; then \
		echo "$(YELLOW)>> golangci-lint not found. Installing $(GOLANGCI_VERSION)...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_VERSION); \
	fi
	golangci-lint run --timeout=5m ./...
	@echo "$(GREEN)>> Lint passed.$(NC)"

## check: Run fmt, vet, and lint
check: fmt vet lint

# -----------------------------------------------------------------------------
# Security Targets
# -----------------------------------------------------------------------------

## security-scan: Run all security scanners (govulncheck, gosec, npm audit)
security-scan: security-scan-go security-scan-code security-scan-frontend

## security-scan-go: Run govulncheck on Go dependencies
security-scan-go:
	@echo "$(BLUE)>> Running Govulncheck (Go vulnerability scan)...$(NC)"
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	@echo "$(GREEN)>> Govulncheck passed.$(NC)"

## security-scan-code: Run gosec on Go source code
security-scan-code:
	@echo "$(BLUE)>> Running Gosec (Go code security scan)...$(NC)"
	go run github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION) ./...
	@echo "$(GREEN)>> Gosec passed.$(NC)"

## security-scan-frontend: Run npm audit on frontend dependencies
security-scan-frontend:
	@echo "$(BLUE)>> Running npm audit (frontend dependency scan)...$(NC)"
	cd web && npm audit --audit-level=high
	@echo "$(GREEN)>> npm audit passed.$(NC)"

## install-security-tools: Install govulncheck and gosec binaries
install-security-tools:
	@echo "$(BLUE)>> Installing security scanning tools...$(NC)"
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GO) install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
	@echo "$(GREEN)>> Security tools installed.$(NC)"

# -----------------------------------------------------------------------------
# Database Targets
# -----------------------------------------------------------------------------

## migrate-up: Apply all pending database migrations
migrate-up:
	bash scripts/migrate.sh up

## migrate-down: Roll back the last database migration
migrate-down:
	bash scripts/migrate.sh down

## migrate-down-all: Roll back all database migrations
migrate-down-all:
	bash scripts/migrate.sh down-all

## migrate-status: Show migration status
migrate-status:
	bash scripts/migrate.sh status

## migrate-create: Create a new migration file (NAME required)
migrate-create:
	bash scripts/migrate.sh create $(NAME)

## seed: Seed the database with test fixtures
seed:
	bash scripts/seed.sh --target all

# -----------------------------------------------------------------------------
# Docker Targets
# -----------------------------------------------------------------------------

## docker-build: Build Docker images
docker-build:
	@echo "$(BLUE)>> Building Docker images...$(NC)"
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f deployments/docker/Dockerfile.apiserver \
		-t $(IMAGE_APISERVER) \
		-t $(DOCKER_REGISTRY)/keyip-apiserver:latest \
		.
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f deployments/docker/Dockerfile.worker \
		-t $(IMAGE_WORKER) \
		-t $(DOCKER_REGISTRY)/keyip-worker:latest \
		.
	@echo "$(GREEN)>> Docker images built.$(NC)"

## docker-push: Push Docker images
docker-push: docker-build
	@echo "$(BLUE)>> Pushing Docker images...$(NC)"
	docker push $(IMAGE_APISERVER)
	docker push $(IMAGE_WORKER)

## docker-compose-up: Start local development environment
docker-compose-up:
	@echo "$(BLUE)>> Starting services...$(NC)"
	$(DOCKER_COMPOSE) up -d --build
	@echo "$(GREEN)>> Services started.$(NC)"

## docker-compose-down: Stop local development environment
docker-compose-down:
	@echo "$(YELLOW)>> Stopping services...$(NC)"
	$(DOCKER_COMPOSE) down --remove-orphans
	@echo "$(GREEN)>> Services stopped.$(NC)"

## docker-logs: Tail logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f --tail=100

# -----------------------------------------------------------------------------
# Code Generation
# -----------------------------------------------------------------------------

## generate: Run go generate
generate:
	$(GO) generate ./...

## proto: Compile protobuf files
proto: proto-deps
	@echo "$(BLUE)>> Compiling proto files...$(NC)"
	@mkdir -p $(PROTO_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(PROTO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/*.proto

## openapi: Verify OpenAPI specs (placeholder)
openapi:
	@echo "OpenAPI verification not implemented yet"

## mock: Generate mock files
mock:
	@echo "$(BLUE)>> Generating mocks...$(NC)"
	$(GO) generate ./...

# -----------------------------------------------------------------------------
# Clean Targets
# -----------------------------------------------------------------------------

## clean: Remove build artifacts
clean: clean-bin clean-cover

## clean-all: Remove all artifacts including cache and generated files
clean-all: clean
	@echo "$(YELLOW)>> Cleaning all artifacts...$(NC)"
	rm -rf .testcache
	$(GO) clean -cache -modcache

## clean-bin: Remove compiled binaries
clean-bin:
	rm -rf $(BINARY_DIR)

## clean-cover: Remove coverage reports
clean-cover:
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)

# -----------------------------------------------------------------------------
# Helper Targets
# -----------------------------------------------------------------------------

## deps: Download dependencies
deps:
	$(GO) mod download

## tools: Install development tools
tools: install-tools

## install-tools: Install required tools
install-tools:
	@echo "$(BLUE)>> Installing tools...$(NC)"
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
	$(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32.0
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	$(GO) install go.uber.org/mock/mockgen@latest
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GO) install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)

## help: Display this help message
help:
	@echo ""
	@echo "$(BLUE)KeyIP-Intelligence Build System$(NC)"
	@echo "$(BLUE)================================$(NC)"
	@echo "Version: $(VERSION)  |  Commit: $(GIT_COMMIT)  |  Branch: $(GIT_BRANCH)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@awk 'BEGIN { \
		group["Build Targets"] = "Build"; \
		group["Test Targets"] = "Test"; \
		group["Code Quality Targets"] = "Development"; \
		group["Code Generation"] = "Development"; \
		group["Clean Targets"] = "Development"; \
		group["Database Targets"] = "Deploy"; \
		group["Docker Targets"] = "Deploy"; \
		group["Security Targets"] = "Security"; \
		group["Helper Targets"] = "Tools"; \
		hcolor["Build"] = "\033[1;34m"; \
		hcolor["Test"] = "\033[1;32m"; \
		hcolor["Development"] = "\033[1;33m"; \
		hcolor["Deploy"] = "\033[1;35m"; \
		hcolor["Security"] = "\033[1;31m"; \
		hcolor["Tools"] = "\033[1;36m"; \
		order["Build"] = 1; \
		order["Test"] = 2; \
		order["Development"] = 3; \
		order["Deploy"] = 4; \
		order["Security"] = 5; \
		order["Tools"] = 6; \
	} \
	/^# (Build|Test|Code Quality|Security|Database|Docker|Clean|Helper) Targets/ { section = $$0; gsub(/^# +/, "", section); gsub(/ +$$/, "", section); } \
	/^# Code Generation/ { section = "Code Generation"; } \
	/^## / { \
		sub(/^## /, ""); \
		split($$0, parts, ": "); \
		t = parts[1]; d = parts[2]; \
		if (section in group) { \
			g = group[section]; \
			items[g] = items[g] sprintf("  \033[36m%-28s\033[0m %s\n", t, d); \
		} \
	} \
	END { \
		for (i = 1; i <= 6; i++) { \
			for (g in order) { \
				if (order[g] == i) { \
					print hcolor[g] "  + " g "\033[0m"; \
					print items[g]; \
				} \
			} \
		} \
	}' $(MAKEFILE_LIST)
	@echo ""

# //Personal.AI order the ending
