# Development Guide

> Version: 0.1.0-alpha | Last Updated: 2026-05

This guide covers the architecture, project structure, and common development workflows for contributing to KeyIP-Intelligence.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Project Structure Walkthrough](#project-structure-walkthrough)
3. [How to Add a New API Endpoint](#how-to-add-a-new-api-endpoint)
4. [How to Add a New Domain Entity](#how-to-add-a-new-domain-entity)
5. [How to Add a New AI Model](#how-to-add-a-new-ai-model)
6. [Testing Guide](#testing-guide)
7. [Debugging Tips](#debugging-tips)
8. [Common Workflows](#common-workflows)

---

## Architecture Overview

KeyIP-Intelligence follows a **hexagonal (ports-and-adapters) architecture** with **Domain-Driven Design** principles, organized into four layers:

```
┌──────────────────────────────────────────────────────┐
│                   Interfaces Layer                    │
│     HTTP (gin)   │   gRPC   │   CLI (cobra)          │
├──────────────────────────────────────────────────────┤
│                Application Layer                      │
│     Use cases, workflow orchestration                 │
├──────────────────────────────────────────────────────┤
│              Intelligence Layer                       │
│     AI/ML model serving, inference pipelines          │
├──────────────────────────────────────────────────────┤
│                 Domain Layer                          │
│     Business entities, value objects, domain services │
├──────────────────────────────────────────────────────┤
│             Infrastructure Layer                      │
│     DB adapters, search, messaging, auth, storage     │
└──────────────────────────────────────────────────────┘
```

### Layer Dependency Rules

- **Application** depends on **Intelligence** and **Domain** (via ports)
- **Intelligence** depends on **Domain** (via ports)
- **Domain** has zero external dependencies
- **Infrastructure** implements ports defined by Domain and Application

No reverse dependencies and no layer skipping.

### Key Technology Choices

| Component            | Choice          | Rationale                                   |
| :------------------- | :-------------- | :------------------------------------------ |
| Language             | Go 1.22         | Concurrency, fast compilation, CGo for ONNX |
| HTTP framework       | Gin             | Performance, middleware ecosystem           |
| CLI framework        | Cobra           | De facto standard for Go CLIs               |
| Configuration        | Viper           | File + env + remote config support          |
| Database migrations  | golang-migrate  | SQL-based, versioned, PostgreSQL driver     |
| Relational DB        | PostgreSQL 16   | ACID, JSONB, pgvector extension             |
| Graph DB             | Neo4j 5         | Cypher, graph algorithms, APOC plugin       |
| Vector DB            | Milvus 2.4      | Billion-scale ANN, GPU support              |
| Search engine        | OpenSearch 2.14 | Apache 2.0, CJK analyzers, hybrid search    |
| Message broker       | Kafka 3.x       | Durable log, exactly-once delivery          |
| Object storage       | MinIO           | S3-compatible, self-hosted                  |
| Auth                 | Keycloak 24     | Multi-tenant OIDC, RBAC, open-source        |
| ML serving           | ONNX Runtime    | Language-agnostic, single binary            |
| Frontend             | React + TS      | Rich visualization libraries                |
| Logging              | zap             | High-performance structured logging         |

---

## Project Structure Walkthrough

```
KeyIP-Intelligence/
├── api/
│   └── openapi/v1/
│       └── keyip.yaml              # OpenAPI 3.0 specification
├── cmd/
│   ├── apiserver/main.go           # API server entry point
│   ├── keyip/main.go               # CLI entry point
│   └── worker/main.go              # Background worker entry point
├── configs/
│   ├── config.yaml                 # Active configuration
│   └── config.example.yaml         # Reference configuration
├── deployments/
│   ├── docker/
│   │   ├── Dockerfile.apiserver    # Multi-stage apiserver image
│   │   ├── Dockerfile.worker       # Multi-stage worker image
│   │   └── docker-compose.yml      # Infrastructure services
│   └── kubernetes/
│       ├── base/                   # Kustomize base manifests
│       └── overlays/               # Dev/prod overlays
├── docs/                           # Project documentation
├── internal/
│   ├── domain/                     # Core business logic
│   │   ├── patent/                 # Patent aggregate, claims, Markush
│   │   ├── molecule/               # Molecule entity, fingerprints
│   │   ├── portfolio/              # Portfolio valuation, gap analysis
│   │   ├── lifecycle/              # Deadlines, annuities, jurisdictions
│   │   └── collaboration/          # Workspaces, permissions
│   ├── application/                # Use case orchestration
│   │   ├── patent_mining/          # Similarity search, patentability
│   │   ├── infringement/           # Risk assessment, monitoring
│   │   ├── portfolio/              # Valuation, gap analysis
│   │   ├── lifecycle/              # Deadlines, annuities, legal status
│   │   ├── collaboration/          # Workspace management
│   │   ├── reporting/              # FTO, infringement, portfolio reports
│   │   └── query/                  # Natural language, KG search
│   ├── intelligence/               # AI/ML model serving
│   │   ├── molpatent_gnn/          # Graph neural network for molecules
│   │   ├── claim_bert/             # Patent claim parsing
│   │   ├── strategy_gpt/           # LLM-based strategy generation
│   │   ├── chem_extractor/         # Chemical entity extraction
│   │   ├── infringe_net/           # Infringement risk assessment
│   │   └── common/                 # Model registry, serving, batch
│   ├── infrastructure/             # External adapters
│   │   ├── database/               # PostgreSQL, Neo4j, Redis
│   │   ├── search/                 # OpenSearch, Milvus
│   │   ├── messaging/              # Kafka producer/consumer
│   │   ├── storage/                # MinIO object storage
│   │   ├── auth/                   # Keycloak auth, JWT
│   │   └── monitoring/             # Prometheus, logging
│   ├── interfaces/                 # Entry points
│   │   ├── http/                   # Gin handlers, middleware, router
│   │   ├── grpc/                   # gRPC services
│   │   └── cli/                    # Cobra commands
│   └── config/                     # Configuration structs and loading
├── pkg/                            # Public libraries
│   ├── client/                     # Go SDK client
│   ├── types/                      # Shared type definitions
│   └── errors/                     # Error codes and types
├── scripts/                        # Build and utility scripts
├── test/
│   ├── integration/                # Integration tests
│   ├── e2e/                        # End-to-end tests
│   └── testdata/                   # Fixtures and test data
└── web/                            # React + TypeScript frontend
```

### Domain Module Pattern

Each domain module (e.g., `internal/domain/patent/`) follows a standard structure:

```go
// entity.go - Aggregate root and value objects
type Patent struct {
    ID              uuid.UUID
    PatentNumber    string
    Title           string
    Abstract        string
    FilingDate      time.Time
    PublicationDate time.Time
    GrantDate       *time.Time
    LegalStatus     LegalStatus
    Jurisdiction    Jurisdiction
    Claims          []Claim            // Value objects
    Assignee        *Assignee          // Value object
    Inventors       []Inventor         // Value objects
    IPCCodes        []string
}

type LegalStatus string
const (
    LegalStatusPending   LegalStatus = "pending"
    LegalStatusGranted   LegalStatus = "granted"
    LegalStatusExpired   LegalStatus = "expired"
    LegalStatusRejected  LegalStatus = "rejected"
)

// repository.go - Port interface (implemented by infrastructure layer)
type Repository interface {
    FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error)
    FindByMoleculeID(ctx context.Context, moleculeID uuid.UUID, limit int) ([]*Patent, error)
    Save(ctx context.Context, patent *Patent) error
    UpdateLegalStatus(ctx context.Context, patentNumber string, status LegalStatus) error
}

// service.go - Domain service (business logic)
type Service struct {
    repo Repository
}

func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}

func (s *Service) ValidatePatentNumber(ctx context.Context, number string) error {
    // Jurisdiction-specific validation logic
    if len(number) == 0 {
        return ErrEmptyPatentNumber
    }
    return nil
}
```

### Application Service Pattern

Application services orchestrate domain logic and infrastructure:

```go
// internal/application/patent_mining/similarity_search.go
type SimilaritySearchUseCase struct {
    molRepo    molecule.Repository
    patRepo    patent.Repository
    molSearch  search.MoleculeSearchPort   // Milvus adapter
    patSearch  search.PatentSearchPort     // OpenSearch adapter
    aiModel    intelligence.MolPatentGNN   // ML model
}

func NewSimilaritySearchUseCase(
    molRepo molecule.Repository,
    patRepo patent.Repository,
    molSearch search.MoleculeSearchPort,
    patSearch search.PatentSearchPort,
    aiModel intelligence.MolPatentGNN,
) *SimilaritySearchUseCase {
    return &SimilaritySearchUseCase{
        molRepo: molRepo, patRepo: patRepo,
        molSearch: molSearch, patSearch: patSearch,
        aiModel: aiModel,
    }
}

func (uc *SimilaritySearchUseCase) Execute(
    ctx context.Context, req *molecule.SimilaritySearchRequest,
) (*molecule.SimilaritySearchResult, error) {
    // 1. Normalize and validate the query molecule
    mol, err := uc.molRepo.FindBySMILES(ctx, req.SMILES)
    if err != nil {
        return nil, fmt.Errorf("finding molecule: %w", err)
    }

    // 2. Generate GNN embedding
    emb, err := uc.aiModel.GenerateEmbedding(ctx, mol)
    if err != nil {
        return nil, fmt.Errorf("generating embedding: %w", err)
    }

    // 3. Vector search in Milvus
    vectorResults, err := uc.molSearch.SearchByVector(ctx, emb, req.Threshold, req.MaxResults)
    if err != nil {
        return nil, fmt.Errorf("vector search: %w", err)
    }

    // 4. Fetch full patent data
    patents, err := uc.patRepo.FindByIDs(ctx, vectorResults.PatentIDs())
    if err != nil {
        return nil, fmt.Errorf("fetching patents: %w", err)
    }

    // 5. Return ranked results
    return buildResults(vectorResults, patents), nil
}
```

---

## How to Add a New API Endpoint

This guide walks through adding a new endpoint `GET /api/v1/molecules/{id}/similar` to return similar molecules for a given molecule ID.

### Step 1: Define the Use Case (Application Layer)

If the logic does not exist, add it in `internal/application/`:

```go
// internal/application/patent_mining/similar_molecules.go
type GetSimilarMoleculesUseCase struct {
    molRepo   molecule.Repository
    molSearch search.MoleculeSearchPort
}

func (uc *GetSimilarMoleculesUseCase) Execute(
    ctx context.Context, moleculeID uuid.UUID, limit int,
) ([]*molecule.SimilarityHit, error) {
    mol, err := uc.molRepo.FindByID(ctx, moleculeID)
    if err != nil {
        return nil, fmt.Errorf("finding molecule: %w", err)
    }
    results, err := uc.molSearch.SearchByFingerprint(ctx, mol.Fingerprint, 0.7, limit)
    if err != nil {
        return nil, fmt.Errorf("similarity search: %w", err)
    }
    return results, nil
}
```

### Step 2: Add the Handler (Interface Layer)

Add a handler in `internal/interfaces/http/handlers/`:

```go
// internal/interfaces/http/handlers/molecule_handler.go

// GetSimilarMolecules handles GET /api/v1/molecules/:id/similar
func (h *MoleculeHandler) GetSimilarMolecules(c *gin.Context) {
    idStr := c.Param("id")
    id, err := uuid.Parse(idStr)
    if err != nil {
        respondError(c, http.StatusBadRequest, codes.ErrInvalidUUID, "invalid molecule ID")
        return
    }

    limit := 20
    if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 100 {
        limit = l
    }

    results, err := h.getSimilarMolecules.Execute(c.Request.Context(), id, limit)
    if err != nil {
        h.logger.Error("failed to get similar molecules", "error", err, "id", idStr)
        respondError(c, http.StatusInternalServerError, codes.ErrInternal, "search failed")
        return
    }

    respondOK(c, gin.H{
        "molecule_id": idStr,
        "hits":        results,
        "total":       len(results),
    })
}
```

### Step 3: Register the Route

Add the route in `internal/interfaces/http/router.go`:

```go
func SetupRouter(h *Handlers, mw *Middleware) *gin.Engine {
    r := gin.New()
    v1 := r.Group("/api/v1")
    {
        molecules := v1.Group("/molecules")
        molecules.Use(mw.Auth())
        {
            molecules.POST("/similarity-search", h.Molecule.SimilaritySearch)
            molecules.POST("/substructure-search", h.Molecule.SubstructureSearch)
            molecules.GET("/:id", h.Molecule.GetByID)
            molecules.GET("/:id/similar", h.Molecule.GetSimilarMolecules) // NEW
        }
    }
}
```

### Step 4: Wire Dependencies

In `internal/interfaces/http/server.go` or `cmd/apiserver/main.go`, pass the use case:

```go
getSimilar := application.NewGetSimilarMoleculesUseCase(molRepo, molSearch)
handler := handlers.NewMoleculeHandler(getSimilar, /* ... */)
```

### Step 5: Update the OpenAPI Spec

Add the endpoint to `api/openapi/v1/keyip.yaml`:

```yaml
/molecules/{id}/similar:
  get:
    summary: Find molecules structurally similar to a given molecule
    parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
          format: uuid
      - name: limit
        in: query
        schema:
          type: integer
          default: 20
    responses:
      "200":
        description: Similar molecules
```

### Step 6: Add Tests

```go
// internal/interfaces/http/handlers/molecule_handler_test.go
func TestGetSimilarMolecules(t *testing.T) {
    // Mock setup
    mockUseCase := new(mocks.GetSimilarMoleculesUseCase)
    mockUseCase.On("Execute", mock.Anything, testMoleculeID, 20).
        Return(testResults, nil)

    handler := NewMoleculeHandler(mockUseCase, /* ... */)

    // HTTP test
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Params = gin.Params{{Key: "id", Value: testMoleculeID.String()}}

    handler.GetSimilarMolecules(c)

    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Body.String(), `"hits"`)
}
```

---

## How to Add a New Domain Entity

This guide walks through adding a `Litigation` entity to track patent litigation cases.

### Step 1: Create the Domain Module

```go
// internal/domain/litigation/entity.go
package litigation

import (
    "time"
    "github.com/google/uuid"
)

type LitigationStatus string

const (
    StatusFiled    LitigationStatus = "filed"
    StatusActive   LitigationStatus = "active"
    StatusSettled  LitigationStatus = "settled"
    StatusAppealed LitigationStatus = "appealed"
    StatusClosed   LitigationStatus = "closed"
)

type Litigation struct {
    ID              uuid.UUID
    PatentNumber    string
    Jurisdiction    string
    Court           string
    CaseNumber      string
    FilingDate      time.Time
    Status          LitigationStatus
    Plaintiff       string
    Defendant       string
    AssertedClaims  []string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type Repository interface {
    FindByID(ctx context.Context, id uuid.UUID) (*Litigation, error)
    FindByPatentNumber(ctx context.Context, patentNumber string) ([]*Litigation, error)
    Save(ctx context.Context, litigation *Litigation) error
}

type Service struct {
    repo Repository
}

func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}
```

### Step 2: Create Domain Events (if needed)

```go
// internal/domain/litigation/events.go
type LitigationFiledEvent struct {
    LitigationID uuid.UUID
    PatentNumber string
    FilingDate   time.Time
}

func (e LitigationFiledEvent) EventName() string { return "litigation.filed" }
```

### Step 3: Add Migration

```sql
-- internal/infrastructure/database/postgres/migrations/007_create_litigation.sql
CREATE TABLE litigations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_number   VARCHAR(64) NOT NULL REFERENCES patents(patent_number),
    jurisdiction    VARCHAR(10) NOT NULL,
    court           TEXT NOT NULL,
    case_number     TEXT NOT NULL,
    filing_date     DATE NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'filed',
    plaintiff       TEXT NOT NULL,
    defendant       TEXT NOT NULL,
    asserted_claims TEXT[],
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_litigations_patent_number ON litigations(patent_number);
```

```bash
make migrate-up
```

### Step 4: Implement the Repository

```go
// internal/infrastructure/database/postgres/repositories/litigation_repo.go
type litigationRepository struct {
    db *pgxpool.Pool
}

func NewLitigationRepository(db *pgxpool.Pool) litigation.Repository {
    return &litigationRepository{db: db}
}

func (r *litigationRepository) FindByPatentNumber(
    ctx context.Context, patentNumber string,
) ([]*litigation.Litigation, error) {
    rows, err := r.db.Query(ctx, `
        SELECT id, patent_number, jurisdiction, court, case_number,
               filing_date, status, plaintiff, defendant, asserted_claims,
               created_at, updated_at
        FROM litigations
        WHERE patent_number = $1
        ORDER BY filing_date DESC
    `, patentNumber)
    if err != nil {
        return nil, fmt.Errorf("querying litigations: %w", err)
    }
    defer rows.Close()

    var results []*litigation.Litigation
    for rows.Next() {
        l := &litigation.Litigation{}
        if err := rows.Scan(
            &l.ID, &l.PatentNumber, &l.Jurisdiction, &l.Court,
            &l.CaseNumber, &l.FilingDate, &l.Status, &l.Plaintiff,
            &l.Defendant, &l.AssertedClaims, &l.CreatedAt, &l.UpdatedAt,
        ); err != nil {
            return nil, fmt.Errorf("scanning litigation: %w", err)
        }
        results = append(results, l)
    }
    return results, nil
}
```

### Step 5: Expose via Application Service

```go
// internal/application/litigation/get_litigations.go
type GetLitigationsUseCase struct {
    litRepo litigation.Repository
}

func (uc *GetLitigationsUseCase) Execute(
    ctx context.Context, patentNumber string,
) ([]*litigation.Litigation, error) {
    return uc.litRepo.FindByPatentNumber(ctx, patentNumber)
}
```

### Step 6: Add HTTP Handler (optional)

```go
// internal/interfaces/http/handlers/litigation_handler.go
func (h *LitigationHandler) GetLitigations(c *gin.Context) {
    patentNumber := c.Param("patent_number")
    results, err := h.getLitigations.Execute(c.Request.Context(), patentNumber)
    if err != nil {
        respondError(c, http.StatusInternalServerError, codes.ErrInternal, err.Error())
        return
    }
    respondOK(c, gin.H{"litigations": results})
}
```

### Step 7: Register Route

```go
patents.GET("/:patent_number/litigations", h.Litigation.GetLitigations)
```

---

## How to Add a New AI Model

This guide walks through adding a new `PropertyPredictor` model that predicts OLED material properties from molecular structure.

### Step 1: Create the Intelligence Module

```go
// internal/intelligence/property_predictor/model.go
package property_predictor

import (
    "context"
    "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
)

// PropertyPredictor predicts OLED material properties from molecular structure.
// Uses a graph neural network trained on a proprietary dataset of OLED material
// characterization results.
type PropertyPredictor struct {
    modelPath string
    device    string  // "cpu" or "cuda"
    session   *ort.DynamicAdvancedOptions  // ONNX Runtime session
}

type PredictedProperties struct {
    HOMOLevel     float64
    LUMOLevel     float64
    TripletEnergy float64
    ElectronMobility  float64
    HoleMobility      float64
    ThermalStability  float64
}

func New(modelPath string, device string) (*PropertyPredictor, error) {
    // Load ONNX model
    session, err := ort.NewAdvancedSession(modelPath, /* ... */)
    if err != nil {
        return nil, fmt.Errorf("loading property predictor model: %w", err)
    }
    return &PropertyPredictor{
        modelPath: modelPath,
        device:    device,
        session:   session,
    }, nil
}

func (p *PropertyPredictor) Predict(
    ctx context.Context, mol *molecule.Molecule,
) (*PredictedProperties, error) {
    // 1. Preprocess molecule into tensor
    input, err := p.preprocess(mol)
    if err != nil {
        return nil, fmt.Errorf("preprocessing molecule: %w", err)
    }

    // 2. Run inference
    output, err := p.session.Run(ctx, []*ort.ArbitraryValue{input})
    if err != nil {
        return nil, fmt.Errorf("model inference: %w", err)
    }

    // 3. Postprocess output into structured properties
    return p.postprocess(output)
}
```

### Step 2: Register in the Model Registry

```go
// internal/intelligence/common/model_registry.go
type ModelRegistry struct {
    models map[string]interface{}
}

func (r *ModelRegistry) Register(name string, model interface{}) {
    r.models[name] = model
}

func (r *ModelRegistry) GetPropertyPredictor() *property_predictor.PropertyPredictor {
    return r.models["property_predictor"].(*property_predictor.PropertyPredictor)
}
```

### Step 3: Add Configuration

```yaml
# configs/config.yaml
intelligence:
  property_predictor:
    model_path: "property_predictor.pt"
    batch_size: 32
    timeout: 30s
    device: "cpu"
```

Add Go config struct:

```go
// internal/config/config.go
type PropertyPredictorConfig struct {
    ModelPath string `mapstructure:"model_path"`
    BatchSize int    `mapstructure:"batch_size"`
    Timeout   time.Duration `mapstructure:"timeout"`
    Device    string `mapstructure:"device"`
}
```

### Step 4: Wire Dependencies

In `cmd/apiserver/main.go`:

```go
propPredictor, err := property_predictor.New(
    cfg.Intelligence.PropertyPredictor.ModelPath,
    cfg.Intelligence.PropertyPredictor.Device,
)
if err != nil {
    log.Fatalf("failed to init property predictor: %v", err)
}

registry.Register("property_predictor", propPredictor)
```

### Step 5: Use in Application Layer

```go
// internal/application/patent_mining/property_prediction.go
type PredictPropertiesUseCase struct {
    molRepo  molecule.Repository
    predictor *property_predictor.PropertyPredictor
}

func (uc *PredictPropertiesUseCase) Execute(
    ctx context.Context, smiles string,
) (*property_predictor.PredictedProperties, error) {
    mol, err := uc.molRepo.FindBySMILES(ctx, smiles)
    if err != nil {
        return nil, err
    }
    return uc.predictor.Predict(ctx, mol)
}
```

### Step 6: Add Tests with Mock Model

```go
// internal/intelligence/property_predictor/model_test.go
func TestPropertyPredictor(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping model test in short mode")
    }

    predictor, err := New("testdata/property_predictor_test.pt", "cpu")
    require.NoError(t, err)

    mol := &molecule.Molecule{
        SMILES: "c1ccc2c(c1)c1ccccc1n2-c1ccccc1",
    }

    props, err := predictor.Predict(context.Background(), mol)
    require.NoError(t, err)
    assert.Greater(t, props.HOMOLevel, -10.0)
    assert.Less(t, props.HOMOLevel, 0.0)
}
```

---

## Testing Guide

### Test Organization

Tests use Go build tags for level separation:

| Tag           | File Pattern            | Dependencies                  |
| :------------ | :---------------------- | :---------------------------- |
| `unit`        | `*_test.go`             | None (mocked)                 |
| `integration` | `test/integration/*.go` | Docker services               |
| `e2e`         | `test/e2e/*.go`         | Full stack running            |

A file header declares the tag:

```go
// go:build unit
package molecule_test
```

### Writing Unit Tests

- Mock all external dependencies (database, search, messaging, auth).
- Use `testify/assert` for assertions and `testify/mock` for mocks.
- Use `sqlmock` for PostgreSQL repository tests.
- Use `miniredis` for Redis cache tests.
- Test domain logic exhaustively.

```go
// go:build unit

package molecule_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFingerprintGeneration(t *testing.T) {
    mol := &molecule.Molecule{SMILES: "c1ccccc1"}
    fp, err := molecule.GenerateMorganFingerprint(mol, 2, 2048)

    require.NoError(t, err)
    assert.NotNil(t, fp)
    assert.Equal(t, 2048, fp.BitCount())
    assert.True(t, fp.IsPopulated())
}
```

### Writing Integration Tests

Integration tests verify that adapters work correctly against real services. They are in `test/integration/`:

```go
// go:build integration

package integration

func TestPostgresPatentRepository(t *testing.T) {
    db := setupPostgres(t)
    defer db.Close()

    repo := postgres.NewPatentRepository(db)

    patent := &patent.Patent{
        PatentNumber: "CN123456789A",
        Title:        "Test Patent",
        Jurisdiction: "CN",
    }

    err := repo.Save(context.Background(), patent)
    require.NoError(t, err)

    found, err := repo.FindByPatentNumber(context.Background(), "CN123456789A")
    require.NoError(t, err)
    assert.Equal(t, "CN", found.Jurisdiction)
}
```

### Running Tests

```bash
# Unit tests only (fast)
make test

# With verbose output
go test -tags=unit -v ./internal/domain/...

# Integration tests (requires Docker services up)
make test-integration

# Full test suite
go test -tags=unit,integration ./...

# Coverage
go test -tags=unit -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -html=coverage.out -o coverage.html
```

### Test Fixtures

Fixture data is stored in `test/testdata/`:

- `test/testdata/fixtures/molecule_fixtures.json`
- `test/testdata/fixtures/patent_fixtures.json`
- `test/testdata/fixtures/portfolio_fixtures.json`

Use the `scripts/seed.sh` script to load fixtures into databases:

```bash
./scripts/seed.sh --target postgres
./scripts/seed.sh --target all
```

---

## Debugging Tips

### Local Development

#### 1. Configure Log Level

Set the log level to `debug` in `configs/config.yaml` for verbose output:

```yaml
monitoring:
  logging:
    level: "debug"
    format: "text"    # Human-readable vs JSON
```

#### 2. Use the Health Endpoint

```bash
curl -s http://localhost:8080/api/v1/health | jq .
```

#### 3. Check Database Directly

```bash
# PostgreSQL
docker exec -it keyip-postgres psql -U keyip -d keyip_dev

# Neo4j (browser)
open http://localhost:7474
# Connection: bolt://localhost:7687, username: neo4j, password: neo4j_dev

# Redis
docker exec -it keyip-redis redis-cli

# OpenSearch
curl -s http://localhost:9200/_cat/indices?v
```

#### 4. Kafka Debugging

```bash
# List topics
docker exec keyip-kafka kafka-topics --bootstrap-server localhost:9092 --list

# Consume messages
docker exec keyip-kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic patent.new \
  --from-beginning

# Produce a test message
docker exec keyip-kafka kafka-console-producer \
  --bootstrap-server localhost:9092 \
  --topic patent.new
```

#### 5. Milvus Debugging

```bash
# Install pymilvus and inspect
pip install pymilvus
python3 -c "
from pymilvus import connections, utility
connections.connect(host='localhost', port='19530')
print(utility.list_collections())
"
```

#### 6. MinIO Debugging

```bash
# Open console
open http://localhost:9002
# Username: minioadmin, Password: minioadmin

# Using mc client
docker exec -it keyip-minio-init sh
mc alias set local http://minio:9000 minioadmin minioadmin
mc ls local/keyip-documents/
```

### Common Debugging Scenarios

#### HTTP Handler Returns Wrong Status

Check the middleware chain in `internal/interfaces/http/router.go`. Middleware order matters:

```go
r.Use(mw.Logger())    // 1. Logging first
r.Use(mw.CORS())      // 2. CORS
r.Use(mw.RateLimit()) // 3. Rate limiting
r.Use(mw.Auth())      // 4. Auth
```

#### Database Connection Issues

```bash
# Verify PostgreSQL is running
docker ps | grep keyip-postgres

# Test connection
docker exec keyip-postgres pg_isready -U keyip

# Check pg_hba.conf (PostgreSQL 16 default allows local connections)
```

#### Race Conditions

Run tests with the race detector:

```bash
go test -race ./internal/domain/...
```

#### Profiling

The API server has pprof endpoints (if enabled):

```bash
# Heap profile
go tool pprof http://localhost:8080/debug/pprof/heap

# CPU profile
go tool pprof http://localhost:8080/debug/pprof/profile

# Goroutine dump
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

### IDE Setup

#### VS Code

Recommended extensions:

- **Go** (gopls language server)
- **Go Test Explorer**
- **Docker**
- **YAML** (by Red Hat)

Configure `.vscode/settings.json`:

```json
{
    "go.buildTags": "unit",
    "go.testTags": "unit",
    "go.lintTool": "golangci-lint",
    "go.lintFlags": ["--fast"],
    "go.formatTool": "goimports",
    "[go]": {
        "editor.formatOnSave": true
    }
}
```

#### GoLand

- Enable "Go Modules" in settings.
- Set build tags to `unit` for development.
- Use the built-in test runner and coverage tools.

---

## Common Workflows

### Regenerating gRPC Code

```bash
make proto
```

This compiles proto files from `api/proto/v1/` into `internal/interfaces/grpc/generated/`.

### Generating Mocks

```bash
make mock
```

Uses `go generate` directives in source files:

```go
//go:generate mockgen -source=repository.go -destination=mock_repository.go -package=patent
type Repository interface { ... }
```

### Adding a Database Migration

```bash
make migrate-create NAME=add_litigation_table
```

Edit the generated file in `internal/infrastructure/database/postgres/migrations/`, then:

```bash
make migrate-up
```

### Building for a Specific Platform

```bash
./scripts/build.sh --target apiserver --os linux --arch arm64
```

### Full Local Reset

```bash
make docker-compose-down
make docker-compose-up
sleep 10
make migrate-up
make seed
make build
```
