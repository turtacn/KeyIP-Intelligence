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
LDFLAGS         := -s -w \
                   -X $(MODULE)/internal/config.Version=$(VERSION) \
                   -X $(MODULE)/internal/config.GitCommit=$(GIT_COMMIT) \
                   -X $(MODULE)/internal/config.GitBranch=$(GIT_BRANCH) \
                   -X $(MODULE)/internal/config.BuildTime=$(BUILD_TIME)

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
PROTOC_GEN_GO    := $(shell which protoc-gen-go 2>/dev/null)
PROTOC_GEN_GRPC  := $(shell which protoc-gen-go-grpc 2>/dev/null)

# Migration settings
MIGRATE_DSN      ?= $(shell grep -A5 'database:' configs/config.yaml | grep 'dsn:' | awk '{print $$2}' 2>/dev/null || echo "postgres://postgres:password@localhost:5432/keyip?sslmode=disable")
MIGRATIONS_DIR   := internal/infrastructure/database/postgres/migrations

# Linting
GOLANGCI_LINT    := $(shell which golangci-lint 2>/dev/null)
GOLANGCI_VERSION := v1.56.2

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
        test test-unit test-integration test-e2e test-cover \
        lint fmt vet tidy \
        proto proto-clean proto-deps \
        migrate-up migrate-down migrate-status migrate-create \
        docker-build docker-push docker-up docker-down docker-logs \
        clean clean-bin clean-cover \
        seed run-apiserver run-worker \
        check-tools install-tools \
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

## build-keyip: Compile the CLI binary
build-keyip:
	@echo "$(BLUE)>> Building keyip CLI...$(NC)"
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_KEYIP) ./$(CMD_KEYIP)/...
	@echo "$(GREEN)>> Built: $(BIN_KEYIP) (version=$(VERSION))$(NC)"

## build-race: Build with race detector enabled (for development)
build-race:
	@echo "$(YELLOW)>> Building with race detector (development only)...$(NC)"
	@mkdir -p $(BINARY_DIR)
	$(GO) build $(GOFLAGS) -race -ldflags "$(LDFLAGS)" -o $(BIN_APISERVER)-race ./$(CMD_APISERVER)/...
	$(GO) build $(GOFLAGS) -race -ldflags "$(LDFLAGS)" -o $(BIN_WORKER)-race ./$(CMD_WORKER)/...
	$(GO) build $(GOFLAGS) -race -ldflags "$(LDFLAGS)" -o $(BIN_KEYIP)-race ./$(CMD_KEYIP)/...

# -----------------------------------------------------------------------------
# Test Targets
# -----------------------------------------------------------------------------

## test: Run all unit tests with race detector and verbose output
test: test-unit

## test-unit: Run unit tests for all packages (excludes integration and e2e)
test-unit:
	@echo "$(BLUE)>> Running unit tests...$(NC)"
	$(GO) test $(GOFLAGS) $(TEST_FLAGS) \
		-timeout $(TEST_TIMEOUT) \
		-coverprofile=$(COVERAGE_FILE) \
		-covermode=atomic \
		$(shell $(GO) list ./... | grep -v '/test/integration' | grep -v '/test/e2e') 2>&1 | tee /tmp/test-unit.log
	@echo "$(GREEN)>> Unit tests passed.$(NC)"
	@$(GO) tool cover -func=$(COVERAGE_FILE) | tail -1 | awk '{print "$(GREEN)>> Total coverage: "$$3"$(NC)"}'

## test-integration: Run integration tests (requires running infrastructure)
test-integration:
	@echo "$(BLUE)>> Running integration tests (requires DB/Redis/Kafka/etc.)...$(NC)"
	$(GO) test $(GOFLAGS) $(TEST_FLAGS) \
		-timeout $(INT_TEST_TIMEOUT) \
		-tags=integration \
		./test/integration/... 2>&1 | tee /tmp/test-integration.log
	@echo "$(GREEN)>> Integration tests passed.$(NC)"

## test-e2e: Run end-to-end tests against a fully deployed stack
test-e2e:
	@echo "$(BLUE)>> Running E2E tests (requires full stack via docker-compose)...$(NC)"
	@$(MAKE) docker-up
	@sleep 15
	$(GO) test $(GOFLAGS) $(TEST_FLAGS) \
		-timeout $(E2E_TEST_TIMEOUT) \
		-tags=e2e \
		./test/e2e/... 2>&1 | tee /tmp/test-e2e.log
	@echo "$(GREEN)>> E2E tests passed.$(NC)"

## test-cover: Generate and open HTML coverage report
test-cover: test-unit
	@echo "$(BLUE)>> Generating coverage report...$(NC)"
	$(GO) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "$(GREEN)>> Coverage report: $(COVERAGE_HTML)$(NC)"
	@which open > /dev/null 2>&1 && open $(COVERAGE_HTML) || true
	@which xdg-open > /dev/null 2>&1 && xdg-open $(COVERAGE_HTML) || true

## test-pkg: Run tests for a specific package. Usage: make test-pkg PKG=./internal/domain/patent/...
test-pkg:
	@echo "$(BLUE)>> Running tests for package: $(PKG)$(NC)"
	$(GO) test $(GOFLAGS) $(TEST_FLAGS) -timeout $(TEST_TIMEOUT) $(PKG)

# -----------------------------------------------------------------------------
# Code Quality Targets
# -----------------------------------------------------------------------------

## fmt: Format all Go source files using gofmt and goimports
fmt:
	@echo "$(BLUE)>> Formatting code...$(NC)"
	@which goimports > /dev/null 2>&1 || $(GO) install golang.org/x/tools/cmd/goimports@latest
	gofmt -s -w $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
	goimports -w -local $(MODULE) $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
	@echo "$(GREEN)>> Formatting complete.$(NC)"

## vet: Run go vet on all packages
vet:
	@echo "$(BLUE)>> Running go vet...$(NC)"
	$(GO) vet $(GOFLAGS) ./...
	@echo "$(GREEN)>> go vet passed.$(NC)"

## lint: Run golangci-lint static analysis
lint:
	@echo "$(BLUE)>> Running golangci-lint...$(NC)"
	@if [ -z "$(GOLANGCI_LINT)" ]; then \
		echo "$(YELLOW)>> golangci-lint not found. Installing $(GOLANGCI_VERSION)...$(NC)"; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_VERSION); \
	fi
	golangci-lint run --timeout=5m ./...
	@echo "$(GREEN)>> Lint passed.$(NC)"

## tidy: Tidy and verify go modules
tidy:
	@echo "$(BLUE)>> Tidying go modules...$(NC)"
	$(GO) mod tidy
	$(GO) mod verify
	@echo "$(GREEN)>> go mod tidy complete.$(NC)"

# -----------------------------------------------------------------------------
# Proto Targets
# -----------------------------------------------------------------------------

## proto: Compile all Protocol Buffer definitions to Go code
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
	@echo "$(GREEN)>> Proto compilation complete. Output: $(PROTO_OUT)$(NC)"

## proto-clean: Remove generated proto files
proto-clean:
	@echo "$(YELLOW)>> Cleaning generated proto files...$(NC)"
	rm -rf $(PROTO_OUT)
	@echo "$(GREEN)>> Proto files cleaned.$(NC)"

## proto-deps: Install protoc plugin dependencies
proto-deps:
	@echo "$(BLUE)>> Installing protoc Go plugins...$(NC)"
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32.0
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	@echo "$(GREEN)>> Proto plugins installed.$(NC)"

# -----------------------------------------------------------------------------
# Database Migration Targets
# -----------------------------------------------------------------------------

## migrate-up: Apply all pending database migrations
migrate-up:
	@echo "$(BLUE)>> Applying database migrations...$(NC)"
	@which migrate > /dev/null 2>&1 || $(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" up
	@echo "$(GREEN)>> Migrations applied successfully.$(NC)"

## migrate-down: Roll back the last database migration
migrate-down:
	@echo "$(YELLOW)>> Rolling back last migration...$(NC)"
	@which migrate > /dev/null 2>&1 || $(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" down 1
	@echo "$(GREEN)>> Migration rolled back.$(NC)"

## migrate-status: Show migration status
migrate-status:
	@echo "$(BLUE)>> Migration status:$(NC)"
	@which migrate > /dev/null 2>&1 || $(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	migrate -path $(MIGRATIONS_DIR) -database "$(MIGRATE_DSN)" version

## migrate-create: Create a new migration file. Usage: make migrate-create NAME=add_index_patents
migrate-create:
	@[ -n "$(NAME)" ] || (echo "$(RED)>> ERROR: NAME is required. Usage: make migrate-create NAME=<migration_name>$(NC)" && exit 1)
	@echo "$(BLUE)>> Creating migration: $(NAME)$(NC)"
	@which migrate > /dev/null 2>&1 || $(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(NAME)
	@echo "$(GREEN)>> Migration files created in $(MIGRATIONS_DIR)$(NC)"

# -----------------------------------------------------------------------------
# Docker Targets
# -----------------------------------------------------------------------------

## docker-build: Build Docker images for apiserver and worker
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
	@echo "$(GREEN)>> Docker images built: $(IMAGE_APISERVER), $(IMAGE_WORKER)$(NC)"

## docker-push: Push Docker images to registry
docker-push: docker-build
	@echo "$(BLUE)>> Pushing Docker images to registry...$(NC)"
	docker push $(IMAGE_APISERVER)
	docker push $(IMAGE_WORKER)
	docker push $(DOCKER_REGISTRY)/keyip-apiserver:latest
	docker push $(DOCKER_REGISTRY)/keyip-worker:latest
	@echo "$(GREEN)>> Docker images pushed.$(NC)"

## docker-up: Start all services via docker-compose (infrastructure + app)
docker-up:
	@echo "$(BLUE)>> Starting services via docker-compose...$(NC)"
	$(DOCKER_COMPOSE) up -d --build
	@echo "$(GREEN)>> Services started. Use 'make docker-logs' to view logs.$(NC)"

## docker-down: Stop and remove all docker-compose services
docker-down:
	@echo "$(YELLOW)>> Stopping docker-compose services...$(NC)"
	$(DOCKER_COMPOSE) down --remove-orphans
	@echo "$(GREEN)>> Services stopped.$(NC)"

## docker-logs: Tail logs from all docker-compose services
docker-logs:
	$(DOCKER_COMPOSE) logs -f --tail=100

## docker-infra-up: Start only infrastructure services (postgres, redis, kafka, etc.)
docker-infra-up:
	@echo "$(BLUE)>> Starting infrastructure services only...$(NC)"
	$(DOCKER_COMPOSE) up -d postgres redis kafka opensearch milvus minio neo4j
	@echo "$(GREEN)>> Infrastructure services started.$(NC)"

# -----------------------------------------------------------------------------
# Run Targets (Development)
# -----------------------------------------------------------------------------

## run-apiserver: Run the API server in development mode
run-apiserver: build-apiserver
	@echo "$(BLUE)>> Starting API server (version=$(VERSION))...$(NC)"
	CONFIG_FILE=configs/config.yaml $(BIN_APISERVER)

## run-worker: Run the background worker in development mode
run-worker: build-worker
	@echo "$(BLUE)>> Starting background worker (version=$(VERSION))...$(NC)"
	CONFIG_FILE=configs/config.yaml $(BIN_WORKER)

## seed: Seed the database with development/test fixtures
seed:
	@echo "$(BLUE)>> Seeding database with test fixtures...$(NC)"
	bash scripts/seed.sh
	@echo "$(GREEN)>> Database seeded.$(NC)"

# -----------------------------------------------------------------------------
# Tool Installation Targets
# -----------------------------------------------------------------------------

## install-tools: Install all required development tools
install-tools:
	@echo "$(BLUE)>> Installing development tools...$(NC)"
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
	$(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.32.0
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
	$(GO) install github.com/swaggo/swag/cmd/swag@latest
	@echo "$(GREEN)>> All tools installed.$(NC)"

## check-tools: Verify all required tools are available
check-tools:
	@echo "$(BLUE)>> Checking required tools...$(NC)"
	@which go > /dev/null 2>&1       && echo "$(GREEN)  ✓ go$(NC)"          || echo "$(RED)  ✗ go (required)$(NC)"
	@which docker > /dev/null 2>&1   && echo "$(GREEN)  ✓ docker$(NC)"      || echo "$(YELLOW)  ✗ docker (optional)$(NC)"
	@which protoc > /dev/null 2>&1   && echo "$(GREEN)  ✓ protoc$(NC)"      || echo "$(YELLOW)  ✗ protoc (optional, for proto gen)$(NC)"
	@which migrate > /dev/null 2>&1  && echo "$(GREEN)  ✓ migrate$(NC)"     || echo "$(YELLOW)  ✗ migrate (run: make install-tools)$(NC)"
	@which golangci-lint > /dev/null 2>&1 && echo "$(GREEN)  ✓ golangci-lint$(NC)" || echo "$(YELLOW)  ✗ golangci-lint (run: make install-tools)$(NC)"
	@which goimports > /dev/null 2>&1 && echo "$(GREEN)  ✓ goimports$(NC)"  || echo "$(YELLOW)  ✗ goimports (run: make install-tools)$(NC)"

# -----------------------------------------------------------------------------
# Clean Targets
# -----------------------------------------------------------------------------

## clean: Remove all build artifacts and generated files
clean: clean-bin clean-cover
	@echo "$(GREEN)>> Clean complete.$(NC)"

## clean-bin: Remove compiled binaries
clean-bin:
	@echo "$(YELLOW)>> Removing binaries...$(NC)"
	rm -rf $(BINARY_DIR)

## clean-cover: Remove coverage reports
clean-cover:
	@echo "$(YELLOW)>> Removing coverage files...$(NC)"
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)

# -----------------------------------------------------------------------------
# CI Targets (used in GitHub Actions workflows)
# -----------------------------------------------------------------------------

## ci: Full CI pipeline: tidy, fmt check, vet, lint, test, build
ci: tidy fmt-check vet lint test build
	@echo "$(GREEN)>> CI pipeline passed.$(NC)"

## fmt-check: Check formatting without modifying files (for CI)
fmt-check:
	@echo "$(BLUE)>> Checking code formatting...$(NC)"
	@unformatted=$$(gofmt -l $(shell find . -name '*.go' -not -path './vendor/*')); \
	if [ -n "$$unformatted" ]; then \
		echo "$(RED)>> Unformatted files:$(NC)"; \
		echo "$$unformatted"; \
		exit 1; \
	fi
	@echo "$(GREEN)>> All files properly formatted.$(NC)"

# -----------------------------------------------------------------------------
# Help Target
# -----------------------------------------------------------------------------

## help: Display this help message with all available targets
help:
	@echo ""
	@echo "$(BLUE)KeyIP-Intelligence Build System$(NC)"
	@echo "$(BLUE)================================$(NC)"
	@echo "Version: $(VERSION)  |  Commit: $(GIT_COMMIT)  |  Branch: $(GIT_BRANCH)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "$(YELLOW)Available targets:$(NC)"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //' | awk 'BEGIN {FS=":"} {printf "  $(GREEN)%-28s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Examples:$(NC)"
	@echo "  make build                          # Build all binaries"
	@echo "  make test                           # Run unit tests"
	@echo "  make test-integration               # Run integration tests"
	@echo "  make test-e2e                       # Run E2E tests"
	@echo "  make docker-up                      # Start full stack"
	@echo "  make migrate-up                     # Apply DB migrations"
	@echo "  make migrate-create NAME=add_index  # Create new migration"
	@echo "  make test-pkg PKG=./internal/...    # Test specific package"
	@echo ""

#Personal.AI order the ending
