# Contributing to KeyIP-Intelligence

Thank you for considering contributing to KeyIP-Intelligence. This project sits at the intersection of chemistry, patent law, materials science, and software engineering. Whether you are a developer, chemist, patent professional, or materials scientist, your expertise is valuable.

---

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Development Environment Setup](#development-environment-setup)
3. [Project Overview](#project-overview)
4. [Code Style Guide](#code-style-guide)
5. [Commit Conventions](#commit-conventions)
6. [Pull Request Process](#pull-request-process)
7. [Issue Reporting](#issue-reporting)
8. [Testing Guidelines](#testing-guidelines)
9. [Documentation](#documentation)

---

## Code of Conduct

This project adheres to the [Contributor Covenant](https://www.contributor-covenant.org/) code of conduct. By participating, you are expected to uphold this code.

Key expectations:

- Use welcoming and inclusive language.
- Respect differing viewpoints and experiences.
- Accept constructive criticism gracefully.
- Focus on what is best for the community.
- Show empathy towards other community members.

---

## Development Environment Setup

### Prerequisites

| Tool          | Version   | Purpose                                |
| :------------ | :-------- | :------------------------------------- |
| Go            | 1.22+     | Primary development language           |
| Docker        | 24+       | Infrastructure services (Docker Compose) |
| Docker Compose| 2.x       | Local service orchestration            |
| Bun           | 1.x       | Frontend build tool                    |
| Make          | any       | Build automation                       |

### Step 1: Clone the Repository

```bash
git clone https://github.com/turtacn/KeyIP-Intelligence.git
cd KeyIP-Intelligence
```

### Step 2: Install Development Tools

```bash
make install-tools
```

This installs:

- `goimports` -- Code formatting with import sorting
- `golangci-lint` -- Linting (v1.56.2)
- `golang-migrate` -- Database migrations with PostgreSQL driver
- `protoc-gen-go` / `protoc-gen-go-grpc` -- gRPC code generation
- `mockgen` -- Mock generation for tests

### Step 3: Start Infrastructure Services

```bash
# Start all services (PostgreSQL, Neo4j, Redis, OpenSearch, Milvus, Kafka, MinIO)
make docker-compose-up

# Verify all services are healthy
docker compose -f deployments/docker/docker-compose.yml ps
```

This starts 14 containers defined in `deployments/docker/docker-compose.yml`:

| Service           | Port(s)         | Purpose                                   |
| :---------------- | :-------------- | :---------------------------------------- |
| PostgreSQL 16     | 5432            | Primary relational store                  |
| Neo4j 5           | 7474, 7687      | Patent knowledge graph                    |
| Redis 7           | 6379            | Cache, session store, rate limiting       |
| OpenSearch 2.14   | 9200, 9600      | Full-text search                          |
| OpenSearch Dashboards | 5601        | Search UI (optional)                      |
| Milvus 2.4        | 19530           | Vector similarity search                  |
| Milvus etcd       | --              | Milvus metadata                           |
| Milvus MinIO      | 9000, 9001      | Milvus object storage                     |
| MinIO (standalone)| 9002, 9003      | Patent document storage                   |
| Kafka 7.6         | 9092            | Event streaming                           |
| MailHog           | 1025, 8025      | Email testing                             |

### Step 4: Configure

Copy the example config and review it:

```bash
cp configs/config.example.yaml configs/config.yaml
```

The defaults in `configs/config.example.yaml` match the Docker Compose service credentials.

### Step 5: Run Database Migrations

```bash
make migrate-up
```

This applies all SQL migration files from `internal/infrastructure/database/postgres/migrations/`:

- `001_create_patents.sql`
- `002_create_molecules.sql`
- `003_create_portfolios.sql`
- `004_create_lifecycle.sql`
- `005_create_users.sql`
- `006_create_workspaces.sql`

### Step 6: Seed Test Data (Optional)

```bash
make seed
```

This loads fixture data from `test/testdata/` into PostgreSQL, Neo4j, OpenSearch, and Milvus.

### Step 7: Build and Run

```bash
# Build all binaries
make build

# Run the API server
make run-apiserver   # or: bin/apiserver

# Run the background worker (in a separate terminal)
make run-worker      # or: bin/worker
```

The API server listens on `http://localhost:8080` by default.

### Step 8: Build the Frontend

```bash
cd web
bun install
bun run dev
```

The frontend dev server runs on `http://localhost:5173` and proxies API requests to `http://localhost:8080`.

### Quick Verification

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Expected response (200 OK):
# {"status":"ok","version":"dev","uptime":"..."}
```

---

## Project Overview

KeyIP-Intelligence follows a **four-layer hexagonal architecture**:

```
internal/
├── domain/            # Core business logic (zero external dependencies)
├── application/       # Use case orchestration
├── intelligence/      # AI/ML model serving
├── infrastructure/    # External adapters (DB, search, messaging, auth)
└── interfaces/        # HTTP, gRPC, CLI entry points
```

Each domain module (patent, molecule, portfolio, lifecycle, collaboration) has:

- `entity.go` -- Domain aggregate root and value objects
- `repository.go` -- Repository interface (port)
- `service.go` -- Domain service
- `events.go` -- Domain events (where applicable)

See [docs/architecture.md](docs/architecture.md) for the complete design.

---

## Code Style Guide

### Go

- **Formatting**: Use `gofmt` and `goimports`. Run `make fmt` before committing.
- **Naming**: Follow [Go naming conventions](https://go.dev/doc/effective_go#names).
  - Use camelCase for unexported identifiers.
  - Use PascalCase for exported identifiers.
  - Acronyms should be uppercase: `HTTPHandler`, `APIServer`, `DBConnection`.
- **Error handling**: Always check errors. Use `fmt.Errorf("context: %w", err)` for error wrapping.
- **Imports**: Group in three blocks separated by blank lines:
  1. Standard library
  2. Third-party packages
  3. Internal packages (`github.com/turtacn/KeyIP-Intelligence/...`)
- **Line length**: Aim for under 120 characters. Not enforced strictly.
- **Comments**: Document all exported symbols. Use complete sentences.
- **Zero values**: Prefer zero-value initialization over constructor functions.

```go
// Good
type Patent struct {
    ID        uuid.UUID `json:"id"`
    Number    string    `json:"patent_number"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"`
}

// Good error wrapping
if err := repo.Save(ctx, patent); err != nil {
    return fmt.Errorf("saving patent %s: %w", patent.Number, err)
}
```

### TypeScript / React

- Use TypeScript strict mode.
- Prefer functional components with hooks.
- Use named exports for components.
- Use kebab-case for file names (e.g., `patent-search.tsx`).
- Run `bunx tsc --noEmit` to type-check.

### General

- Avoid magic strings. Define constants.
- Use dependency injection. Avoid global state.
- Write tests alongside production code.

---

## Commit Conventions

We follow [Conventional Commits](https://www.conventionalcommits.org/) with a domain prefix.

### Format

```
<type>(<scope>): <short description>

<optional body>
```

### Types

| Type       | Usage                                        |
| :--------- | :------------------------------------------- |
| `feat`     | A new feature                                |
| `fix`      | A bug fix                                    |
| `docs`     | Documentation only changes                   |
| `style`    | Formatting, missing semicolons, etc.         |
| `refactor` | Code change that neither fixes nor adds      |
| `test`     | Adding or correcting tests                   |
| `chore`    | Build process, tooling, dependency changes   |
| `perf`     | Performance improvement                      |
| `ci`       | CI/CD configuration changes                  |

### Scopes

| Scope            | Area                                      |
| :--------------- | :---------------------------------------- |
| `patent`         | Patent domain, repository, handlers       |
| `molecule`       | Molecule domain, fingerprint, similarity  |
| `portfolio`      | Portfolio valuation, gap analysis         |
| `lifecycle`      | Deadline management, annuities            |
| `collaboration`  | Workspaces, sharing                       |
| `intelligence`   | AI models (GNN, BERT, LLM, etc.)         |
| `api`            | OpenAPI spec, API design                  |
| `frontend`       | React/TypeScript UI                       |
| `infra`          | Docker, Kubernetes, CI/CD                 |
| `deps`           | Dependency updates                        |

### Examples

```
feat(molecule): add Morgan fingerprint computation for similarity search
fix(patent): handle missing assignee in legal status sync
docs(api): add request examples for infringement assessment
refactor(lifecycle): extract deadline escalation logic into domain service
test(portfolio): add valuation edge cases for empty portfolios
ci: add gosec security scanning step
```

### Commit Body

When the change is non-trivial, add a body explaining the motivation:

```
feat(infrastructure): implement Milvus vector collection manager

Add CreateCollection, DropCollection, and ListCollections operations
for managing Milvus collections. This enables runtime management of
molecular fingerprint indices without manual intervention.

Closes #142
```

---

## Pull Request Process

### Before Submitting

1. Ensure all existing tests pass:

```bash
make test
```

2. Run the linter:

```bash
make lint
```

3. Add tests for new functionality. Coverage should not decrease.

4. Ensure the frontend compiles:

```bash
cd web && bun run build && cd -
```

5. Run `make check` (fmt + vet + lint) for a final sanity check.

### Step-by-Step

1. **Create a feature branch** from `main`:

```bash
git checkout main
git pull origin main
git checkout -b feat/my-feature
```

2. **Make your changes** with small, focused commits following the [commit conventions](#commit-conventions).

3. **Push your branch** and open a pull request against `main`:

```bash
git push origin feat/my-feature
```

4. **Fill in the PR template** with:
   - A clear description of the change and motivation.
   - Any design decisions or trade-offs made.
   - Screenshots or API examples for UI/API changes.
   - Testing instructions.

5. **Ensure CI passes**. The pipeline runs:
   - `golangci-lint` (Go code quality)
   - `tsc --noEmit` (TypeScript check)
   - `go test -race` (Go tests with race detection)
   - `go build` (Go compilation)
   - `bun run build` (Frontend build)
   - `gosec` (Security scan)
   - Codecov (Coverage upload)

6. **Request review** from at least one maintainer.

### Review Criteria

Maintainers evaluate PRs on:

- **Correctness**: Does the code do what it claims?
- **Test coverage**: Are there tests for new and changed code?
- **Design**: Does it fit the architecture? Is it maintainable?
- **Performance**: Are there obvious inefficiencies?
- **Security**: Does it introduce vulnerabilities?

### Merging

- PRs require at least one approval from a maintainer.
- All CI checks must pass.
- The reviewer merges using **Squash and Merge** to keep a clean history.
- After merging, delete the feature branch.

---

## Issue Reporting

### Bug Reports

Use the GitHub issue tracker and include:

- **Summary**: A clear, concise description of the bug.
- **Steps to reproduce**: Minimal, complete steps.
- **Expected behavior**: What should happen.
- **Actual behavior**: What actually happens.
- **Environment**: Go version, OS, Docker version, relevant config.
- **Logs/Traces**: Relevant error output, stack traces, or log snippets.
- **Possible fix**: Optional, if you have a suggestion.

### Feature Requests

- **Problem statement**: What problem are you trying to solve?
- **Proposed solution**: Describe the desired behavior or API.
- **Alternatives considered**: Any workarounds you have tried.
- **Context**: How this fits into the OLED IP management domain.

### Security Issues

Do not open a public issue. Email the maintainers directly or use GitHub's private vulnerability reporting.

---

## Testing Guidelines

### Test Levels

Tests are organized using Go build tags:

| Tag             | Location                   | Purpose                          |
| :-------------- | :------------------------- | :------------------------------- |
| `unit`          | `*_test.go` (in-package)   | Fast, no external dependencies   |
| `integration`   | `test/integration/`        | Tests requiring real services    |
| `e2e`           | `test/e2e/`                | Full workflow tests              |

### Running Tests

```bash
make test                # Unit tests only
make test-integration    # Integration tests (requires Docker)
make test-e2e            # E2E tests
make test-coverage       # Unit tests with coverage HTML report
make test-race           # Tests with race detection
```

Use build tags for granular execution:

```bash
# Run unit tests for a specific package
go test -tags=unit ./internal/domain/patent/...

# Run all integration tests
go test -tags=integration ./test/integration/...
```

### Writing Tests

- Use `github.com/stretchr/testify/assert` and `require` for assertions.
- Use `github.com/DATA-DOG/go-sqlmock` for PostgreSQL repository tests.
- Use `github.com/alicebob/miniredis/v2` for Redis cache tests.
- Use `github.com/stretchr/testify/mock` (or mockgen) for interface mocks.
- Use `go.uber.org/mock/mockgen` for generating mocks from interfaces.

```go
func TestSimilaritySearch(t *testing.T) {
    // Arrange
    mockMilvus := new(mocks.MilvusClient)
    mockMilvus.On("Search", mock.Anything, mock.Anything).
        Return([]milvus.SearchResult{{ID: "mol-001", Score: 0.95}}, nil)

    svc := molecule.NewService(mockMilvus)

    // Act
    results, err := svc.SimilaritySearch(context.Background(), "c1ccccc1", 0.7)

    // Assert
    require.NoError(t, err)
    assert.Len(t, results, 1)
    assert.InDelta(t, 0.95, results[0].Score, 0.01)
}
```

### Test Fixtures

Fixture data is in `test/testdata/`:

```
test/testdata/
├── fixtures/
│   ├── molecule_fixtures.json
│   ├── patent_fixtures.json
│   └── portfolio_fixtures.json
├── molecules/           # SMILES datasets
└── patents/             # Full patent documents (CN/EP/US)
```

---

## Documentation

- Architecture decisions go in [docs/architecture.md](docs/architecture.md).
- API reference is in [docs/apis.md](docs/apis.md).
- Development guide is in [docs/development.md](docs/development.md).
- Deployment guide is in [docs/deployment.md](docs/deployment.md).

When you add or change API endpoints, update `docs/apis.md` and the OpenAPI spec at `api/openapi/v1/keyip.yaml`.

When you add a new package or restructure existing code, update the file listing in `docs/architecture.md`.

---

## Getting Help

- **Slack/Discord**: Coming soon.
- **GitHub Issues**: For bugs and feature requests.
- **Documentation**: Start with the docs/ directory.
- **Code reviews**: Open a draft PR early for feedback on design.
