// Phase 10 - File 223 of 349
// Tests for ConstellationService application service.
// Covers: construction validation, GenerateConstellation orchestration flow,
// GetTechDomainDistribution aggregation, CompareWithCompetitor comparison logic,
// GetCoverageHeatmap density computation, caching interactions, and error handling.

package portfolio

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainmol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/molpatent_gnn"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// -----------------------------------------------------------------------
// Mock: Logger
// -----------------------------------------------------------------------

type mockLoggerConstellation struct{}

func (m *mockLoggerConstellation) Debug(msg string, fields ...logging.Field) {}
func (m *mockLoggerConstellation) Info(msg string, fields ...logging.Field)  {}
func (m *mockLoggerConstellation) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLoggerConstellation) Error(msg string, fields ...logging.Field) {}
func (m *mockLoggerConstellation) Fatal(msg string, fields ...logging.Field) {}
func (m *mockLoggerConstellation) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockLoggerConstellation) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockLoggerConstellation) WithError(err error) logging.Logger { return m }
func (m *mockLoggerConstellation) Sync() error { return nil }

var _ logging.Logger = (*mockLoggerConstellation)(nil)

// -----------------------------------------------------------------------
// Mock: ConstellationCache
// -----------------------------------------------------------------------

type mockConstellationCache struct {
	store    map[string]interface{}
	getErr   error
	setErr   error
	getCalls int
	setCalls int
}

func newMockConstellationCache() *mockConstellationCache {
	return &mockConstellationCache{store: make(map[string]interface{})}
}

func (m *mockConstellationCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.getCalls++
	if m.getErr != nil {
		return m.getErr
	}
	if _, ok := m.store[key]; !ok {
		return fmt.Errorf("cache miss")
	}
	return nil
}

func (m *mockConstellationCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.setCalls++
	if m.setErr != nil {
		return m.setErr
	}
	m.store[key] = value
	return nil
}

func (m *mockConstellationCache) Delete(ctx context.Context, key string) error {
	delete(m.store, key)
	return nil
}

// -----------------------------------------------------------------------
// Mock: Portfolio Domain Service
// -----------------------------------------------------------------------

type mockPortfolioService struct {
	portfolio   domainportfolio.Portfolio
	getByIDErr  error
}

func (m *mockPortfolioService) GetByID(ctx context.Context, id string) (domainportfolio.Portfolio, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	return m.portfolio, nil
}

// Stub remaining interface methods.
func (m *mockPortfolioService) Create(ctx context.Context, p domainportfolio.Portfolio) error { return nil }
func (m *mockPortfolioService) Update(ctx context.Context, p domainportfolio.Portfolio) error { return nil }
func (m *mockPortfolioService) Delete(ctx context.Context, id string) error                  { return nil }
func (m *mockPortfolioService) List(ctx context.Context) ([]domainportfolio.Portfolio, error) { return nil, nil }

var _ domainportfolio.Service = (*mockPortfolioService)(nil)

// -----------------------------------------------------------------------
// Mock: Portfolio entity
// -----------------------------------------------------------------------

type mockPortfolio struct {
	id   string
	name string
}

func (p *mockPortfolio) GetID() string   { return p.id }
func (p *mockPortfolio) GetName() string { return p.name }

var _ domainportfolio.Portfolio = (*mockPortfolio)(nil)

// -----------------------------------------------------------------------
// Mock: Patent Repository
// -----------------------------------------------------------------------

type mockPatentRepoConstellation struct {
	byPortfolio map[string][]domainpatent.Patent
	byAssignee  map[string][]domainpatent.Patent
	byIDs       map[string]domainpatent.Patent
	findErr     error
}

func newMockPatentRepoConstellation() *mockPatentRepoConstellation {
	return &mockPatentRepoConstellation{
		byPortfolio: make(map[string][]domainpatent.Patent),
		byAssignee:  make(map[string][]domainpatent.Patent),
		byIDs:       make(map[string]domainpatent.Patent),
	}
}

func (m *mockPatentRepoConstellation) FindByPortfolioID(ctx context.Context, portfolioID string) ([]domainpatent.Patent, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.byPortfolio[portfolioID], nil
}

func (m *mockPatentRepoConstellation) FindByAssignee(ctx context.Context, assignee string) ([]domainpatent.Patent, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.byAssignee[assignee], nil
}

func (m *mockPatentRepoConstellation) FindByIDs(ctx context.Context, ids []string) ([]domainpatent.Patent, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := make([]domainpatent.Patent, 0, len(ids))
	for _, id := range ids {
		if p, ok := m.byIDs[id]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}

// Stub remaining interface methods.
func (m *mockPatentRepoConstellation) Save(ctx context.Context, p domainpatent.Patent) error   { return nil }
func (m *mockPatentRepoConstellation) GetByID(ctx context.Context, id string) (domainpatent.Patent, error) { return nil, nil }
func (m *mockPatentRepoConstellation) Delete(ctx context.Context, id string) error              { return nil }

var _ domainpatent.Repository = (*mockPatentRepoConstellation)(nil)

// -----------------------------------------------------------------------
// Mock: Patent entity
// -----------------------------------------------------------------------

type mockPatent struct {
	id          string
	number      string
	techDomain  string
	legalStatus string
	assignee    string
	filingDate  time.Time
	valueScore  float64
	moleculeIDs []string
}

func (p *mockPatent) GetID() string               { return p.id }
func (p *mockPatent) GetPatentNumber() string      { return p.number }
func (p *mockPatent) GetPrimaryTechDomain() string { return p.techDomain }
func (p *mockPatent) GetLegalStatus() string       { return p.legalStatus }
func (p *mockPatent) GetAssignee() string          { return p.assignee }
func (p *mockPatent) GetFilingDate() time.Time     { return p.filingDate }
func (p *mockPatent) GetValueScore() float64       { return p.valueScore }
func (p *mockPatent) GetMoleculeIDs() []string     { return p.moleculeIDs }

var _ domainpatent.Patent = (*mockPatent)(nil)

// -----------------------------------------------------------------------
// Mock: Molecule Repository
// -----------------------------------------------------------------------

type mockMoleculeRepo struct {
	molecules map[string]domainmol.Molecule
	findErr   error
}

func newMockMoleculeRepo() *mockMoleculeRepo {
	return &mockMoleculeRepo{molecules: make(map[string]domainmol.Molecule)}
}

func (m *mockMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]domainmol.Molecule, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	result := make([]domainmol.Molecule, 0, len(ids))
	for _, id := range ids {
		if mol, ok := m.molecules[id]; ok {
			result = append(result, mol)
		}
	}
	return result, nil
}

// Stub remaining interface methods.
func (m *mockMoleculeRepo) Save(ctx context.Context, mol domainmol.Molecule) error { return nil }
func (m *mockMoleculeRepo) GetByID(ctx context.Context, id string) (domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeRepo) Delete(ctx context.Context, id string) error             { return nil }

var _ domainmol.Repository = (*mockMoleculeRepo)(nil)

// -----------------------------------------------------------------------
// Mock: Molecule entity
// -----------------------------------------------------------------------

type mockMolecule struct {
	id     string
	smiles string
}

func (m *mockMolecule) GetID() string     { return m.id }
func (m *mockMolecule) GetSMILES() string { return m.smiles }

var _ domainmol.Molecule = (*mockMolecule)(nil)

// -----------------------------------------------------------------------
// Mock: Molecule Domain Service
// -----------------------------------------------------------------------

type mockMoleculeService struct{}

func (m *mockMoleculeService) GetByID(ctx context.Context, id string) (domainmol.Molecule, error) { return nil, nil }
func (m *mockMoleculeService) Create(ctx context.Context, mol domainmol.Molecule) error           { return nil }

var _ domainmol.Service = (*mockMoleculeService)(nil)

// -----------------------------------------------------------------------
// Mock: GNN Inference Engine
// -----------------------------------------------------------------------

type mockGNNInference struct {
	embeddingVec []float64
	embedErr     error
	reducedVecs  [][]float64
	reduceErr    error
}

func (m *mockGNNInference) GenerateEmbedding(ctx context.Context, req *molpatent_gnn.EmbeddingRequest) (*molpatent_gnn.EmbeddingResponse, error) {
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	vec := m.embeddingVec
	if vec == nil {
		vec = []float64{0.1, 0.2, 0.3, 0.4}
	}
	return &molpatent_gnn.EmbeddingResponse{Vector: vec}, nil
}

func (m *mockGNNInference) ReduceDimensions(ctx context.Context, req *molpatent_gnn.ReductionRequest) (*molpatent_gnn.ReductionResponse, error) {
	if m.reduceErr != nil {
		return nil, m.reduceErr
	}
	if m.reducedVecs != nil {
		return &molpatent_gnn.ReductionResponse{Reduced: m.reducedVecs}, nil
	}
	// Generate synthetic 2D coordinates for each input vector.
	reduced := make([][]float64, len(req.Vectors))
	for i := range req.Vectors {
		reduced[i] = []float64{float64(i) * 1.5, float64(i) * 0.8}
	}
	return &molpatent_gnn.ReductionResponse{Reduced: reduced}, nil
}

func (m *mockGNNInference) Predict(ctx context.Context, req *molpatent_gnn.PredictionRequest) (*molpatent_gnn.PredictionResponse, error) {
	return nil, nil
}

var _ molpatent_gnn.InferenceEngine = (*mockGNNInference)(nil)

// -----------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------

func buildTestConfig(overrides ...func(*ConstellationServiceConfig)) ConstellationServiceConfig {
	patentRepo := newMockPatentRepoConstellation()
	molRepo := newMockMoleculeRepo()

	// Seed test data: 3 patents, each with 1 molecule.
	patents := []domainpatent.Patent{
		&mockPatent{
			id: "pat-1", number: "US1234", techDomain: "A61K", legalStatus: "granted",
			assignee: "OwnCorp", filingDate: time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC),
			valueScore: 8.5, moleculeIDs: []string{"mol-1"},
		},
		&mockPatent{
			id: "pat-2", number: "US5678", techDomain: "C07D", legalStatus: "granted",
			assignee: "OwnCorp", filingDate: time.Date(2021, 7, 20, 0, 0, 0, 0, time.UTC),
			valueScore: 7.2, moleculeIDs: []string{"mol-2"},
		},
		&mockPatent{
			id: "pat-3", number: "US9012", techDomain: "A61K", legalStatus: "pending",
			assignee: "OwnCorp", filingDate: time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC),
			valueScore: 9.1, moleculeIDs: []string{"mol-3"},
		},
	}
	patentRepo.byPortfolio["portfolio-1"] = patents

	// Competitor patents.
	compPatents := []domainpatent.Patent{
		&mockPatent{
			id: "comp-1", number: "EP1111", techDomain: "A61K", legalStatus: "granted",
			assignee: "CompetitorInc", filingDate: time.Date(2019, 5, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 6.0, moleculeIDs: []string{"mol-c1"},
		},
		&mockPatent{
			id: "comp-2", number: "EP2222", techDomain: "G16B", legalStatus: "granted",
			assignee: "CompetitorInc", filingDate: time.Date(2022, 11, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 8.0, moleculeIDs: []string{"mol-c2"},
		},
	}
	patentRepo.byAssignee["CompetitorInc"] = compPatents

	// Molecules.
	molRepo.molecules["mol-1"] = &mockMolecule{id: "mol-1", smiles: "CCO"}
	molRepo.molecules["mol-2"] = &mockMolecule{id: "mol-2", smiles: "c1ccccc1"}
	molRepo.molecules["mol-3"] = &mockMolecule{id: "mol-3", smiles: "CC(=O)O"}

	cfg := ConstellationServiceConfig{
		PortfolioService:   &mockPortfolioService{portfolio: &mockPortfolio{id: "portfolio-1", name: "Test Portfolio"}},
		MoleculeService:    &mockMoleculeService{},
		PatentRepository:   patentRepo,
		MoleculeRepository: molRepo,
		GNNInference:       &mockGNNInference{},
		Logger:             &mockLoggerConstellation{},
		Cache:              newMockConstellationCache(),
		CacheTTL:           5 * time.Minute,
	}

	for _, override := range overrides {
		override(&cfg)
	}

	return cfg
}

// -----------------------------------------------------------------------
// Tests: Construction
// -----------------------------------------------------------------------

func TestNewConstellationService_Success(t *testing.T) {
	cfg := buildTestConfig()
	svc, err := NewConstellationService(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewConstellationService_MissingDependencies(t *testing.T) {
	tests := []struct {
		name     string
		override func(*ConstellationServiceConfig)
	}{
		{"nil PortfolioService", func(c *ConstellationServiceConfig) { c.PortfolioService = nil }},
		{"nil MoleculeService", func(c *ConstellationServiceConfig) { c.MoleculeService = nil }},
		{"nil PatentRepository", func(c *ConstellationServiceConfig) { c.PatentRepository = nil }},
		{"nil MoleculeRepository", func(c *ConstellationServiceConfig) { c.MoleculeRepository = nil }},
		{"nil GNNInference", func(c *ConstellationServiceConfig) { c.GNNInference = nil }},
		{"nil Logger", func(c *ConstellationServiceConfig) { c.Logger = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := buildTestConfig(tt.override)
			svc, err := NewConstellationService(cfg)
			if err == nil {
				t.Fatal("expected error for missing dependency")
			}
			if svc != nil {
				t.Fatal("expected nil service on error")
			}
		})
	}
}

func TestNewConstellationService_DefaultCacheTTL(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.CacheTTL = 0
	})
	svc, err := NewConstellationService(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	impl := svc.(*constellationServiceImpl)
	if impl.cacheTTL != 30*time.Minute {
		t.Errorf("expected default TTL 30m, got %v", impl.cacheTTL)
	}
}

func TestNewConstellationService_NilCacheAllowed(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.Cache = nil
	})
	svc, err := NewConstellationService(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service even without cache")
	}
}

// -----------------------------------------------------------------------
// Tests: GenerateConstellation
// -----------------------------------------------------------------------

func TestGenerateConstellation_Success(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	resp, err := svc.GenerateConstellation(context.Background(), &ConstellationRequest{
		PortfolioID:        "portfolio-1",
		IncludeWhiteSpaces: true,
		Reduction: DimensionReduction{
			Algorithm:  ReductionUMAP,
			Dimensions: 2,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != "portfolio-1" {
		t.Errorf("expected portfolio_id 'portfolio-1', got '%s'", resp.PortfolioID)
	}
	if len(resp.Points) == 0 {
		t.Error("expected at least one constellation point")
	}
	if resp.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
	if resp.CoverageStats.TotalPoints != len(resp.Points) {
		t.Errorf("stats total %d != points len %d", resp.CoverageStats.TotalPoints, len(resp.Points))
	}
}

func TestGenerateConstellation_NilRequest(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GenerateConstellation(context.Background(), nil)
	if err == nil {
		t.Fatal("expected validation error for nil request")
	}
}

func TestGenerateConstellation_EmptyPortfolioID(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GenerateConstellation(context.Background(), &ConstellationRequest{})
	if err == nil {
		t.Fatal("expected validation error for empty portfolio_id")
	}
}

func TestGenerateConstellation_PortfolioNotFound(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.PortfolioService = &mockPortfolioService{portfolio: nil}
	})
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GenerateConstellation(context.Background(), &ConstellationRequest{
		PortfolioID: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestGenerateConstellation_EmptyPatents(t *testing.T) {
	patentRepo := newMockPatentRepoConstellation()
	// No patents for this portfolio.
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.PatentRepository = patentRepo
	})
	svc, _ := NewConstellationService(cfg)

	resp, err := svc.GenerateConstellation(context.Background(), &ConstellationRequest{
		PortfolioID: "portfolio-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Points) != 0 {
		t.Errorf("expected 0 points for empty portfolio, got %d", len(resp.Points))
	}
}

func TestGenerateConstellation_CacheHit(t *testing.T) {
	cache := newMockConstellationCache()
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.Cache = cache
	})
	svc, _ := NewConstellationService(cfg)

	// First call populates cache.
	req := &ConstellationRequest{PortfolioID: "portfolio-1"}
	_, err := svc.GenerateConstellation(context.Background(), req)
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if cache.setCalls == 0 {
		t.Error("expected cache Set to be called")
	}

	// Second call should attempt cache Get.
	initialGetCalls := cache.getCalls
	_, err = svc.GenerateConstellation(context.Background(), req)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if cache.getCalls <= initialGetCalls {
		t.Error("expected cache Get to be called on second invocation")
	}
}

func TestGenerateConstellation_GNNEmbeddingError(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.GNNInference = &mockGNNInference{embedErr: fmt.Errorf("gnn unavailable")}
	})
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GenerateConstellation(context.Background(), &ConstellationRequest{
		PortfolioID: "portfolio-1",
	})
	if err == nil {
		t.Fatal("expected error when GNN embedding fails")
	}
}

func TestGenerateConstellation_WithFilters(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	resp, err := svc.GenerateConstellation(context.Background(), &ConstellationRequest{
		PortfolioID: "portfolio-1",
		Filters: ConstellationFilters{
			TechDomains: []string{"A61K"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only include patents with tech domain A61K (pat-1 and pat-3).
	for _, pt := range resp.Points {
		if pt.TechDomain != "A61K" {
			t.Errorf("expected all points to have domain A61K, got '%s'", pt.TechDomain)
		}
	}
}

// -----------------------------------------------------------------------
// Tests: GetTechDomainDistribution
// -----------------------------------------------------------------------

func TestGetTechDomainDistribution_Success(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	dist, err := svc.GetTechDomainDistribution(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist == nil {
		t.Fatal("expected non-nil distribution")
	}
	if dist.PortfolioID != "portfolio-1" {
		t.Errorf("expected portfolio_id 'portfolio-1', got '%s'", dist.PortfolioID)
	}
	if dist.TotalCount != 3 {
		t.Errorf("expected total count 3, got %d", dist.TotalCount)
	}
	if len(dist.Domains) == 0 {
		t.Fatal("expected at least one domain entry")
	}

	// Verify A61K has 2 patents and C07D has 1.
	domainCounts := make(map[string]int)
	for _, d := range dist.Domains {
		domainCounts[d.DomainCode] = d.PatentCount
	}
	if domainCounts["A61K"] != 2 {
		t.Errorf("expected A61K count 2, got %d", domainCounts["A61K"])
	}
	if domainCounts["C07D"] != 1 {
		t.Errorf("expected C07D count 1, got %d", domainCounts["C07D"])
	}

	// Verify percentages sum to ~100.
	totalPct := 0.0
	for _, d := range dist.Domains {
		totalPct += d.Percentage
	}
	if totalPct < 99.9 || totalPct > 100.1 {
		t.Errorf("expected percentages to sum to ~100, got %.2f", totalPct)
	}

	// Verify sorted by count descending.
	for i := 1; i < len(dist.Domains); i++ {
		if dist.Domains[i].PatentCount > dist.Domains[i-1].PatentCount {
			t.Error("expected domains sorted by patent count descending")
			break
		}
	}
}

func TestGetTechDomainDistribution_EmptyPortfolioID(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GetTechDomainDistribution(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty portfolio_id")
	}
}

func TestGetTechDomainDistribution_PortfolioNotFound(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.PortfolioService = &mockPortfolioService{portfolio: nil}
	})
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GetTechDomainDistribution(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestGetTechDomainDistribution_NoPatents(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.PatentRepository = newMockPatentRepoConstellation()
	})
	svc, _ := NewConstellationService(cfg)

	dist, err := svc.GetTechDomainDistribution(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist.TotalCount != 0 {
		t.Errorf("expected 0 total count, got %d", dist.TotalCount)
	}
}

func TestGetTechDomainDistribution_ValuePercentages(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	dist, err := svc.GetTechDomainDistribution(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	totalValuePct := 0.0
	for _, d := range dist.Domains {
		totalValuePct += d.ValuePercent
		if d.ValueSum <= 0 {
			t.Errorf("expected positive value sum for domain %s", d.DomainCode)
		}
	}
	if totalValuePct < 99.9 || totalValuePct > 100.1 {
		t.Errorf("expected value percentages to sum to ~100, got %.2f", totalValuePct)
	}
}

// -----------------------------------------------------------------------
// Tests: CompareWithCompetitor
// -----------------------------------------------------------------------

func TestCompareWithCompetitor_Success(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	resp, err := svc.CompareWithCompetitor(context.Background(), &CompetitorCompareRequest{
		PortfolioID:    "portfolio-1",
		CompetitorName: "CompetitorInc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != "portfolio-1" {
		t.Errorf("expected portfolio_id 'portfolio-1', got '%s'", resp.PortfolioID)
	}
	if resp.CompetitorName != "CompetitorInc" {
		t.Errorf("expected competitor 'CompetitorInc', got '%s'", resp.CompetitorName)
	}

	// A61K should be an overlap zone (both own and competitor have patents there).
	foundA61KOverlap := false
	for _, oz := range resp.OverlapZones {
		if oz.TechDomain == "A61K" {
			foundA61KOverlap = true
			if oz.OwnCount != 2 {
				t.Errorf("expected own count 2 in A61K overlap, got %d", oz.OwnCount)
			}
			if oz.CompCount != 1 {
				t.Errorf("expected comp count 1 in A61K overlap, got %d", oz.CompCount)
			}
			if oz.Intensity <= 0 || oz.Intensity > 1.0 {
				t.Errorf("expected intensity in (0,1], got %f", oz.Intensity)
			}
		}
	}
	if !foundA61KOverlap {
		t.Error("expected A61K to appear as overlap zone")
	}

	// C07D should be own exclusive.
	foundC07DExcl := false
	for _, ez := range resp.OwnExclusive {
		if ez.TechDomain == "C07D" {
			foundC07DExcl = true
			if ez.PatentCount != 1 {
				t.Errorf("expected 1 patent in C07D exclusive, got %d", ez.PatentCount)
			}
		}
	}
	if !foundC07DExcl {
		t.Error("expected C07D to appear as own exclusive zone")
	}

	// G16B should be competitor exclusive.
	foundG16BExcl := false
	for _, ez := range resp.CompExclusive {
		if ez.TechDomain == "G16B" {
			foundG16BExcl = true
		}
	}
	if !foundG16BExcl {
		t.Error("expected G16B to appear as competitor exclusive zone")
	}

	// Summary validation.
	if resp.Summary.TotalOwnPatents != 3 {
		t.Errorf("expected 3 own patents in summary, got %d", resp.Summary.TotalOwnPatents)
	}
	if resp.Summary.TotalCompPatents != 2 {
		t.Errorf("expected 2 comp patents in summary, got %d", resp.Summary.TotalCompPatents)
	}
	if resp.Summary.OverlapDomainCount != len(resp.OverlapZones) {
		t.Error("summary overlap count mismatch")
	}
	if resp.Summary.OverallAdvantage == "" {
		t.Error("expected non-empty overall advantage")
	}
	if resp.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
}

func TestCompareWithCompetitor_NilRequest(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.CompareWithCompetitor(context.Background(), nil)
	if err == nil {
		t.Fatal("expected validation error for nil request")
	}
}

func TestCompareWithCompetitor_EmptyPortfolioID(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.CompareWithCompetitor(context.Background(), &CompetitorCompareRequest{
		CompetitorName: "CompetitorInc",
	})
	if err == nil {
		t.Fatal("expected validation error for empty portfolio_id")
	}
}

func TestCompareWithCompetitor_EmptyCompetitorName(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.CompareWithCompetitor(context.Background(), &CompetitorCompareRequest{
		PortfolioID: "portfolio-1",
	})
	if err == nil {
		t.Fatal("expected validation error for empty competitor_name")
	}
}

func TestCompareWithCompetitor_WithTechDomainFilter(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	resp, err := svc.CompareWithCompetitor(context.Background(), &CompetitorCompareRequest{
		PortfolioID:    "portfolio-1",
		CompetitorName: "CompetitorInc",
		TechDomains:    []string{"A61K"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With A61K filter, only A61K domain should appear.
	for _, oz := range resp.OverlapZones {
		if oz.TechDomain != "A61K" {
			t.Errorf("expected only A61K in filtered comparison, got '%s'", oz.TechDomain)
		}
	}
	// No own exclusive (C07D filtered out), no comp exclusive (G16B filtered out).
	if len(resp.OwnExclusive) != 0 {
		t.Errorf("expected 0 own exclusive with A61K filter, got %d", len(resp.OwnExclusive))
	}
	if len(resp.CompExclusive) != 0 {
		t.Errorf("expected 0 comp exclusive with A61K filter, got %d", len(resp.CompExclusive))
	}
}

func TestCompareWithCompetitor_StrengthIndex(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	resp, err := svc.CompareWithCompetitor(context.Background(), &CompetitorCompareRequest{
		PortfolioID:    "portfolio-1",
		CompetitorName: "CompetitorInc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Own has 3 patents with higher total value, competitor has 2.
	// Strength index should be positive (own advantage).
	if resp.StrengthIndex <= 0 {
		t.Errorf("expected positive strength index (own advantage), got %f", resp.StrengthIndex)
	}
	if resp.StrengthIndex < -1.0 || resp.StrengthIndex > 1.0 {
		t.Errorf("strength index out of range [-1,1]: %f", resp.StrengthIndex)
	}
}

func TestCompareWithCompetitor_PatentRepoError(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		repo := newMockPatentRepoConstellation()
		repo.findErr = fmt.Errorf("db connection lost")
		c.PatentRepository = repo
	})
	svc, _ := NewConstellationService(cfg)

	_, err := svc.CompareWithCompetitor(context.Background(), &CompetitorCompareRequest{
		PortfolioID:    "portfolio-1",
		CompetitorName: "CompetitorInc",
	})
	if err == nil {
		t.Fatal("expected error when patent repo fails")
	}
}

// -----------------------------------------------------------------------
// Tests: GetCoverageHeatmap
// -----------------------------------------------------------------------

func TestGetCoverageHeatmap_Success(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	heatmap, err := svc.GetCoverageHeatmap(context.Background(), "portfolio-1", WithResolution(50))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if heatmap == nil {
		t.Fatal("expected non-nil heatmap")
	}
	if heatmap.PortfolioID != "portfolio-1" {
		t.Errorf("expected portfolio_id 'portfolio-1', got '%s'", heatmap.PortfolioID)
	}
	if heatmap.Resolution != 50 {
		t.Errorf("expected resolution 50, got %d", heatmap.Resolution)
	}
	if len(heatmap.Grid) != 50 {
		t.Errorf("expected grid rows 50, got %d", len(heatmap.Grid))
	}
	for i, row := range heatmap.Grid {
		if len(row) != 50 {
			t.Errorf("expected grid row %d cols 50, got %d", i, len(row))
			break
		}
	}
	if heatmap.MaxDensity <= 0 {
		t.Error("expected positive max density")
	}
	if heatmap.XRange[0] >= heatmap.XRange[1] {
		t.Error("expected xRange[0] < xRange[1]")
	}
	if heatmap.YRange[0] >= heatmap.YRange[1] {
		t.Error("expected yRange[0] < yRange[1]")
	}
	if heatmap.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
}

func TestGetCoverageHeatmap_EmptyPortfolioID(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GetCoverageHeatmap(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty portfolio_id")
	}
}

func TestGetCoverageHeatmap_DefaultResolution(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	heatmap, err := svc.GetCoverageHeatmap(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if heatmap.Resolution != 100 {
		t.Errorf("expected default resolution 100, got %d", heatmap.Resolution)
	}
}

func TestGetCoverageHeatmap_NoPatents(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.PatentRepository = newMockPatentRepoConstellation()
	})
	svc, _ := NewConstellationService(cfg)

	heatmap, err := svc.GetCoverageHeatmap(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(heatmap.Grid) != 0 {
		t.Errorf("expected empty grid for no patents, got %d rows", len(heatmap.Grid))
	}
}

func TestGetCoverageHeatmap_CacheInteraction(t *testing.T) {
	cache := newMockConstellationCache()
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.Cache = cache
	})
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GetCoverageHeatmap(context.Background(), "portfolio-1", WithResolution(20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cache.setCalls == 0 {
		t.Error("expected cache Set to be called")
	}
}

func TestGetCoverageHeatmap_WithDensityRange(t *testing.T) {
	cfg := buildTestConfig()
	svc, _ := NewConstellationService(cfg)

	heatmap, err := svc.GetCoverageHeatmap(context.Background(), "portfolio-1",
		WithResolution(10),
		WithDensityRange(0.0, 5.0),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if heatmap.MaxDensity != 5.0 {
		t.Errorf("expected max density override 5.0, got %f", heatmap.MaxDensity)
	}
}

func TestGetCoverageHeatmap_GNNReduceError(t *testing.T) {
	cfg := buildTestConfig(func(c *ConstellationServiceConfig) {
		c.GNNInference = &mockGNNInference{reduceErr: fmt.Errorf("reduction failed")}
	})
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GetCoverageHeatmap(context.Background(), "portfolio-1")
	if err == nil {
		t.Fatal("expected error when dimension reduction fails")
	}
}

// -----------------------------------------------------------------------
// Tests: Helper Functions
// -----------------------------------------------------------------------

func TestApplyReductionDefaults(t *testing.T) {
	// Empty input should get defaults.
	r := applyReductionDefaults(DimensionReduction{})
	if r.Algorithm != ReductionUMAP {
		t.Errorf("expected default algorithm UMAP, got '%s'", r.Algorithm)
	}
	if r.Dimensions != 2 {
		t.Errorf("expected default dimensions 2, got %d", r.Dimensions)
	}
	if r.Neighbors != 15 {
		t.Errorf("expected default neighbors 15, got %d", r.Neighbors)
	}

	// tSNE should get perplexity default.
	r2 := applyReductionDefaults(DimensionReduction{Algorithm: ReductionTSNE})
	if r2.Perplexity != 30.0 {
		t.Errorf("expected default perplexity 30.0, got %f", r2.Perplexity)
	}

	// Dimensions clamped to [2, 3].
	r3 := applyReductionDefaults(DimensionReduction{Dimensions: 1})
	if r3.Dimensions != 2 {
		t.Errorf("expected dimensions clamped to 2, got %d", r3.Dimensions)
	}
	r4 := applyReductionDefaults(DimensionReduction{Dimensions: 5})
	if r4.Dimensions != 3 {
		t.Errorf("expected dimensions clamped to 3, got %d", r4.Dimensions)
	}
}

func TestToStringSet(t *testing.T) {
	set := toStringSet([]string{"a", "b", "c", "a"})
	if len(set) != 3 {
		t.Errorf("expected 3 unique items, got %d", len(set))
	}
	if _, ok := set["a"]; !ok {
		t.Error("expected 'a' in set")
	}

	nilSet := toStringSet(nil)
	if nilSet != nil {
		t.Error("expected nil for nil input")
	}

	emptySet := toStringSet([]string{})
	if emptySet != nil {
		t.Error("expected nil for empty input")
	}
}

func TestResolveDomainName(t *testing.T) {
	if name := resolveDomainName("A61K"); name != "Preparations for Medical Purposes" {
		t.Errorf("unexpected name for A61K: '%s'", name)
	}
	if name := resolveDomainName("UNKNOWN"); name != "UNKNOWN" {
		t.Errorf("expected unknown code returned as-is, got '%s'", name)
	}
}

func TestComputeBoundingBox(t *testing.T) {
	points := [][]float64{
		{1.0, 2.0},
		{-3.0, 5.0},
		{4.0, -1.0},
	}
	xMin, xMax, yMin, yMax := computeBoundingBox(points)
	if xMin != -3.0 {
		t.Errorf("expected xMin -3.0, got %f", xMin)
	}
	if xMax != 4.0 {
		t.Errorf("expected xMax 4.0, got %f", xMax)
	}
	if yMin != -1.0 {
		t.Errorf("expected yMin -1.0, got %f", yMin)
	}
	if yMax != 5.0 {
		t.Errorf("expected yMax 5.0, got %f", yMax)
	}

	// Empty input.
	xMin2, xMax2, yMin2, yMax2 := computeBoundingBox(nil)
	if xMin2 != 0 || xMax2 != 0 || yMin2 != 0 || yMax2 != 0 {
		t.Error("expected all zeros for empty input")
	}
}

func TestComputeZoneStrength(t *testing.T) {
	patents := []domainpatent.Patent{
		&mockPatent{
			valueScore: 8.0,
			filingDate: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		&mockPatent{
			valueScore: 6.0,
			filingDate: time.Date(2018, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	strength := computeZoneStrength(patents)
	if strength <= 0 {
		t.Errorf("expected positive strength, got %f", strength)
	}

	// Empty patents should return 0.
	if s := computeZoneStrength(nil); s != 0.0 {
		t.Errorf("expected 0 for empty patents, got %f", s)
	}
}

func TestComputeStrengthIndex_Balanced(t *testing.T) {
	own := []domainpatent.Patent{
		&mockPatent{valueScore: 5.0},
	}
	comp := []domainpatent.Patent{
		&mockPatent{valueScore: 5.0},
	}
	overlap := []OverlapZone{
		{OwnCount: 1, CompCount: 1},
	}

	index := computeStrengthIndex(own, comp, overlap)
	// Should be close to 0 (balanced).
	if index < -0.1 || index > 0.1 {
		t.Errorf("expected near-zero index for balanced portfolios, got %f", index)
	}
}

func TestComputeStrengthIndex_OwnAdvantage(t *testing.T) {
	own := []domainpatent.Patent{
		&mockPatent{valueScore: 10.0},
		&mockPatent{valueScore: 10.0},
		&mockPatent{valueScore: 10.0},
	}
	comp := []domainpatent.Patent{
		&mockPatent{valueScore: 2.0},
	}

	index := computeStrengthIndex(own, comp, nil)
	if index <= 0 {
		t.Errorf("expected positive index for own advantage, got %f", index)
	}
}

func TestComputeStrengthIndex_Empty(t *testing.T) {
	index := computeStrengthIndex(nil, nil, nil)
	if index != 0.0 {
		t.Errorf("expected 0 for empty portfolios, got %f", index)
	}
}

func TestWithResolution_Bounds(t *testing.T) {
	cfg := &heatmapConfig{Resolution: 100}

	// Valid resolution.
	WithResolution(50)(cfg)
	if cfg.Resolution != 50 {
		t.Errorf("expected 50, got %d", cfg.Resolution)
	}

	// Zero should not change.
	WithResolution(0)(cfg)
	if cfg.Resolution != 50 {
		t.Errorf("expected unchanged 50, got %d", cfg.Resolution)
	}

	// Over 500 should not change.
	WithResolution(501)(cfg)
	if cfg.Resolution != 50 {
		t.Errorf("expected unchanged 50, got %d", cfg.Resolution)
	}

	// Negative should not change.
	WithResolution(-1)(cfg)
	if cfg.Resolution != 50 {
		t.Errorf("expected unchanged 50, got %d", cfg.Resolution)
	}
}

func TestWithDensityRange_Bounds(t *testing.T) {
	cfg := &heatmapConfig{}

	// Valid range.
	WithDensityRange(0.0, 10.0)(cfg)
	if cfg.MinDensity != 0.0 || cfg.MaxDensity != 10.0 {
		t.Errorf("expected range [0, 10], got [%f, %f]", cfg.MinDensity, cfg.MaxDensity)
	}

	// Invalid: min >= max.
	WithDensityRange(5.0, 5.0)(cfg)
	if cfg.MaxDensity != 10.0 {
		t.Error("expected unchanged max density for invalid range")
	}

	// Invalid: negative min.
	WithDensityRange(-1.0, 5.0)(cfg)
	if cfg.MinDensity != 0.0 {
		t.Error("expected unchanged min density for negative min")
	}
}

func TestFilterByTechDomains(t *testing.T) {
	patents := []domainpatent.Patent{
		&mockPatent{id: "p1", techDomain: "A61K"},
		&mockPatent{id: "p2", techDomain: "C07D"},
		&mockPatent{id: "p3", techDomain: "A61K"},
		&mockPatent{id: "p4", techDomain: "G16B"},
	}

	filtered := filterByTechDomains(patents, []string{"A61K", "G16B"})
	if len(filtered) != 3 {
		t.Errorf("expected 3 filtered patents, got %d", len(filtered))
	}
	for _, p := range filtered {
		domain := p.GetPrimaryTechDomain()
		if domain != "A61K" && domain != "G16B" {
			t.Errorf("unexpected domain in filtered result: '%s'", domain)
		}
	}
}

func TestGroupByDomain(t *testing.T) {
	patents := []domainpatent.Patent{
		&mockPatent{id: "p1", techDomain: "A61K"},
		&mockPatent{id: "p2", techDomain: "C07D"},
		&mockPatent{id: "p3", techDomain: "A61K"},
		&mockPatent{id: "p4", techDomain: ""},
	}

	groups := groupByDomain(patents)
	if len(groups["A61K"]) != 2 {
		t.Errorf("expected 2 patents in A61K, got %d", len(groups["A61K"]))
	}
	if len(groups["C07D"]) != 1 {
		t.Errorf("expected 1 patent in C07D, got %d", len(groups["C07D"]))
	}
	if len(groups["unclassified"]) != 1 {
		t.Errorf("expected 1 patent in unclassified, got %d", len(groups["unclassified"]))
	}
}

func TestMergeKeys(t *testing.T) {
	a := map[string][]domainpatent.Patent{
		"A61K": {},
		"C07D": {},
	}
	b := map[string][]domainpatent.Patent{
		"A61K": {},
		"G16B": {},
	}

	keys := mergeKeys(a, b)
	if len(keys) != 3 {
		t.Errorf("expected 3 merged keys, got %d", len(keys))
	}
	// Should be sorted.
	for i := 1; i < len(keys); i++ {
		if keys[i] < keys[i-1] {
			t.Error("expected sorted keys")
			break
		}
	}
}

// Ensure unused imports are referenced.
var _ = errors.NewValidation

//Personal.AI order the ending
