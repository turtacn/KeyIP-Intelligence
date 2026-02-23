// File: internal/application/portfolio/valuation_test.go
// Tests for patent portfolio valuation application service.

package portfolio

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Mock: PatentRepository
// ---------------------------------------------------------------------------

type mockPatentRepo struct {
	patents     map[string]*patent.Patent
	byPortfolio map[string][]*patent.Patent
	byAssignee  map[string][]*patent.Patent
	err         error
}

func newMockPatentRepo() *mockPatentRepo {
	return &mockPatentRepo{patents: make(map[string]*patent.Patent)}
}

func (m *mockPatentRepo) FindByID(ctx context.Context, id string) (*patent.Patent, error) {
	if m.err != nil {
		return nil, m.err
	}
	p, ok := m.patents[id]
	if !ok {
		return nil, fmt.Errorf("patent %s not found", id)
	}
	return p, nil
}

func (m *mockPatentRepo) Save(ctx context.Context, p *patent.Patent) error   { return m.err }
func (m *mockPatentRepo) Update(ctx context.Context, p *patent.Patent) error { return m.err }
func (m *mockPatentRepo) Delete(ctx context.Context, id string) error        { return m.err }
func (m *mockPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*patent.Patent, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*patent.Patent
	for _, id := range ids {
		if p, ok := m.patents[id]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}
func (m *mockPatentRepo) Search(ctx context.Context, query string, limit, offset int) ([]*patent.Patent, int, error) {
	return nil, 0, nil
}
func (m *mockPatentRepo) AssociateMolecule(ctx context.Context, patentID, moleculeID string) error {
	return m.err
}
func (m *mockPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*patent.Patent, error) {
	if m.err != nil {
		return nil, m.err
	}
	if patents, ok := m.byPortfolio[portfolioID]; ok {
		return patents, nil
	}
	return nil, nil
}
func (m *mockPatentRepo) BatchCreate(ctx context.Context, patents []*patent.Patent) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return len(patents), nil
}
func (m *mockPatentRepo) BatchCreateClaims(ctx context.Context, claims []*patent.Claim) error {
	return m.err
}
func (m *mockPatentRepo) BatchUpdateStatus(ctx context.Context, updates []patent.StatusUpdate) error {
	return m.err
}

// ---------------------------------------------------------------------------
// Mock: PortfolioRepository
// ---------------------------------------------------------------------------

type mockPortfolioRepo struct {
	portfolios map[string]*domainportfolio.Portfolio
	err        error
}

func newMockPortfolioRepo() *mockPortfolioRepo {
	return &mockPortfolioRepo{portfolios: make(map[string]*domainportfolio.Portfolio)}
}

func (m *mockPortfolioRepo) FindByID(ctx context.Context, id string) (*domainportfolio.Portfolio, error) {
	if m.err != nil {
		return nil, m.err
	}
	p, ok := m.portfolios[id]
	if !ok {
		return nil, fmt.Errorf("portfolio %s not found", id)
	}
	return p, nil
}

func (m *mockPortfolioRepo) Save(ctx context.Context, p *domainportfolio.Portfolio) error {
	return m.err
}
func (m *mockPortfolioRepo) Update(ctx context.Context, p *domainportfolio.Portfolio) error {
	return m.err
}
func (m *mockPortfolioRepo) Delete(ctx context.Context, id string) error { return m.err }
func (m *mockPortfolioRepo) FindByOwner(ctx context.Context, ownerID string) ([]*domainportfolio.Portfolio, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock: AssessmentRepository
// ---------------------------------------------------------------------------

type mockAssessmentRepo struct {
	records    map[string]*AssessmentRecord
	byPatent   map[string][]*AssessmentRecord
	byPortfolio map[string][]*AssessmentRecord
	err        error
}

func newMockAssessmentRepo() *mockAssessmentRepo {
	return &mockAssessmentRepo{
		records:     make(map[string]*AssessmentRecord),
		byPatent:    make(map[string][]*AssessmentRecord),
		byPortfolio: make(map[string][]*AssessmentRecord),
	}
}

func (m *mockAssessmentRepo) Save(ctx context.Context, record *AssessmentRecord) error {
	if m.err != nil {
		return m.err
	}
	m.records[record.ID] = record
	m.byPatent[record.PatentID] = append(m.byPatent[record.PatentID], record)
	if record.PortfolioID != "" {
		m.byPortfolio[record.PortfolioID] = append(m.byPortfolio[record.PortfolioID], record)
	}
	return nil
}

func (m *mockAssessmentRepo) FindByID(ctx context.Context, id string) (*AssessmentRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	r, ok := m.records[id]
	if !ok {
		return nil, fmt.Errorf("assessment %s not found", id)
	}
	return r, nil
}

func (m *mockAssessmentRepo) FindByPatentID(ctx context.Context, patentID string, limit, offset int) ([]*AssessmentRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	recs := m.byPatent[patentID]
	if offset >= len(recs) {
		return nil, nil
	}
	end := offset + limit
	if end > len(recs) {
		end = len(recs)
	}
	return recs[offset:end], nil
}

func (m *mockAssessmentRepo) FindByPortfolioID(ctx context.Context, portfolioID string) ([]*AssessmentRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.byPortfolio[portfolioID], nil
}

func (m *mockAssessmentRepo) FindByIDs(ctx context.Context, ids []string) ([]*AssessmentRecord, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*AssessmentRecord
	for _, id := range ids {
		if r, ok := m.records[id]; ok {
			result = append(result, r)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Mock: IntelligenceValueScorer
// ---------------------------------------------------------------------------

type mockAIScorer struct {
	scores map[AssessmentDimension]map[string]float64
	err    error
	calls  int32
}

func newMockAIScorer() *mockAIScorer {
	return &mockAIScorer{
		scores: make(map[AssessmentDimension]map[string]float64),
	}
}

func (m *mockAIScorer) ScorePatent(ctx context.Context, pat *patent.Patent, dim AssessmentDimension) (map[string]float64, error) {
	atomic.AddInt32(&m.calls, 1)
	if m.err != nil {
		return nil, m.err
	}
	if s, ok := m.scores[dim]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("no AI scores configured for dimension %s", dim)
}

// ---------------------------------------------------------------------------
// Mock: CitationRepository
// ---------------------------------------------------------------------------

type mockCitationRepo struct {
	forwardCounts map[string]int
	maxInDomain   map[string]int
	err           error
}

func newMockCitationRepo() *mockCitationRepo {
	return &mockCitationRepo{
		forwardCounts: make(map[string]int),
		maxInDomain:   make(map[string]int),
	}
}

func (m *mockCitationRepo) CountForwardCitations(ctx context.Context, patentID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.forwardCounts[patentID], nil
}

func (m *mockCitationRepo) CountBackwardCitations(ctx context.Context, patentID string) (int, error) {
	return 0, nil
}

func (m *mockCitationRepo) MaxForwardCitationsInDomain(ctx context.Context, domain string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.maxInDomain[domain], nil
}

// ---------------------------------------------------------------------------
// Mock: Cache
// ---------------------------------------------------------------------------

type mockCache struct {
	store map[string][]byte
	err   error
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string][]byte)}
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	v, ok := m.store[key]
	if !ok {
		return nil, fmt.Errorf("cache miss")
	}
	return v, nil
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.err != nil {
		return m.err
	}
	m.store[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	delete(m.store, key)
	return nil
}

// ---------------------------------------------------------------------------
// Mock: Logger
// ---------------------------------------------------------------------------

type mockLogger struct{}

func (mockLogger) Debug(msg string, fields ...logging.Field) {}
func (mockLogger) Info(msg string, fields ...logging.Field)  {}
func (mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (mockLogger) Error(msg string, fields ...logging.Field) {}
func (mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (mockLogger) With(fields ...logging.Field) logging.Logger { return mockLogger{} }
func (mockLogger) WithContext(ctx context.Context) logging.Logger { return mockLogger{} }
func (mockLogger) WithError(err error) logging.Logger { return mockLogger{} }
func (mockLogger) Sync() error { return nil }

// ---------------------------------------------------------------------------
// Mock: Domain Services (minimal stubs)
// ---------------------------------------------------------------------------

type mockPortfolioDomainSvc struct{}

func (mockPortfolioDomainSvc) CreatePortfolio(ctx context.Context, name, ownerID string) (*domainportfolio.Portfolio, error) {
	return nil, nil
}
func (mockPortfolioDomainSvc) AddPatent(ctx context.Context, portfolioID, patentID string) error {
	return nil
}
func (mockPortfolioDomainSvc) RemovePatent(ctx context.Context, portfolioID, patentID string) error {
	return nil
}

type mockValuationDomainSvc struct{}

// ---------------------------------------------------------------------------
// Test Fixtures
// ---------------------------------------------------------------------------

func makeTestPatent(id, title, status string, claimCount int, ipcCount int, filingYearsAgo float64) *patent.Patent {
	claims := make([]patent.Claim, claimCount)
	for i := 0; i < claimCount; i++ {
		claims[i] = patent.Claim{
			Number:        i + 1,
			Text:          fmt.Sprintf("A method comprising step %d of performing an operation on a device", i+1),
			IsIndependent: i < 3, // first 3 are independent
		}
	}

	ipcs := make([]string, ipcCount)
	for i := 0; i < ipcCount; i++ {
		ipcs[i] = fmt.Sprintf("G06F%d/00", 17+i)
	}

	filingDate := time.Now().AddDate(0, 0, -int(filingYearsAgo*365.25))

	return &patent.Patent{
		ID:                 id,
		Title:              title,
		Abstract:           "An improved method to enhance performance and reduce latency in distributed systems 提高效率 优化性能",
		Description:        "This invention relates to a novel approach for distributed computing that significantly improves throughput. " + string(make([]byte, 6000)),
		Status:             status,
		Claims:             claims,
		IPCClassifications: ipcs,
		FilingDate:         filingDate,
		FamilyMembers:      []string{"US123", "EP456", "JP789", "CN101", "KR202"},
	}
}

func buildTestService(
	patentRepo *mockPatentRepo,
	portfolioRepo *mockPortfolioRepo,
	assessmentRepo *mockAssessmentRepo,
	aiScorer *mockAIScorer,
	citationRepo *mockCitationRepo,
	cache *mockCache,
) ValuationService {
	if patentRepo == nil {
		patentRepo = newMockPatentRepo()
	}
	if portfolioRepo == nil {
		portfolioRepo = newMockPortfolioRepo()
	}
	if assessmentRepo == nil {
		assessmentRepo = newMockAssessmentRepo()
	}
	if cache == nil {
		cache = newMockCache()
	}

	return NewValuationService(
		mockPortfolioDomainSvc{},
		mockValuationDomainSvc{},
		patentRepo,
		portfolioRepo,
		assessmentRepo,
		aiScorer,
		citationRepo,
		mockLogger{},
		cache,
		nil, // metrics: will use noopMetrics
		&ValuationServiceConfig{Concurrency: 5, CacheTTL: time.Minute},
	)
}

// ---------------------------------------------------------------------------
// Tests: Enumerations & Helpers
// ---------------------------------------------------------------------------

func TestTierFromScore(t *testing.T) {
	tests := []struct {
		score    float64
		expected PatentTier
	}{
		{100, TierS},
		{95, TierS},
		{90, TierS},
		{89.99, TierA},
		{85, TierA},
		{80, TierA},
		{79.99, TierB},
		{70, TierB},
		{65, TierB},
		{64.99, TierC},
		{55, TierC},
		{50, TierC},
		{49.99, TierD},
		{30, TierD},
		{0, TierD},
		{-5, TierD},
	}

	for _, tt := range tests {
		got := TierFromScore(tt.score)
		if got != tt.expected {
			t.Errorf("TierFromScore(%.2f) = %s, want %s", tt.score, got, tt.expected)
		}
	}
}

func TestTierDescription(t *testing.T) {
	tiers := []PatentTier{TierS, TierA, TierB, TierC, TierD, PatentTier("X")}
	for _, tier := range tiers {
		desc := TierDescription(tier)
		if desc == "" {
			t.Errorf("TierDescription(%s) returned empty string", tier)
		}
	}
}

func TestAllDimensions(t *testing.T) {
	dims := AllDimensions()
	if len(dims) != 4 {
		t.Fatalf("AllDimensions() returned %d dimensions, want 4", len(dims))
	}
	expected := []AssessmentDimension{
		DimensionTechnicalValue, DimensionLegalValue,
		DimensionCommercialValue, DimensionStrategicValue,
	}
	for i, d := range dims {
		if d != expected[i] {
			t.Errorf("AllDimensions()[%d] = %s, want %s", i, d, expected[i])
		}
	}
}

func TestClampScore(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-10, 0},
		{0, 0},
		{50.555, 50.56},
		{100, 100},
		{150, 100},
	}
	for _, tt := range tests {
		got := clampScore(tt.input)
		if got != tt.expected {
			t.Errorf("clampScore(%.3f) = %.2f, want %.2f", tt.input, got, tt.expected)
		}
	}
}

func TestDimensionWeightsSum(t *testing.T) {
	var total float64
	for _, w := range DimensionWeights {
		total += w
	}
	if total < 0.999 || total > 1.001 {
		t.Errorf("DimensionWeights sum = %.4f, want 1.0", total)
	}
}

func TestFactorWeightsPerDimension(t *testing.T) {
	for dim, factors := range dimensionFactors {
		var total float64
		for _, f := range factors {
			total += f.Weight
		}
		if total < 0.999 || total > 1.001 {
			t.Errorf("Factor weights for %s sum = %.4f, want 1.0", dim, total)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Request Validation
// ---------------------------------------------------------------------------

func TestSinglePatentAssessmentRequest_Validate(t *testing.T) {
	t.Run("missing patent_id", func(t *testing.T) {
		req := &SinglePatentAssessmentRequest{}
		err := req.Validate()
		if err == nil {
			t.Fatal("expected validation error for missing patent_id")
		}
	})

	t.Run("unknown dimension", func(t *testing.T) {
		req := &SinglePatentAssessmentRequest{
			PatentID:   "P001",
			Dimensions: []AssessmentDimension{"unknown_dim"},
		}
		err := req.Validate()
		if err == nil {
			t.Fatal("expected validation error for unknown dimension")
		}
	})

	t.Run("defaults applied", func(t *testing.T) {
		req := &SinglePatentAssessmentRequest{PatentID: "P001"}
		err := req.Validate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Dimensions) != 4 {
			t.Errorf("expected 4 default dimensions, got %d", len(req.Dimensions))
		}
		if req.Context == nil {
			t.Fatal("expected default context to be set")
		}
		if req.Context.MaxPatentLifeYrs != 20 {
			t.Errorf("expected MaxPatentLifeYrs=20, got %d", req.Context.MaxPatentLifeYrs)
		}
	})

	t.Run("negative max life corrected", func(t *testing.T) {
		req := &SinglePatentAssessmentRequest{
			PatentID: "P001",
			Context:  &AssessmentContext{MaxPatentLifeYrs: -5},
		}
		err := req.Validate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req.Context.MaxPatentLifeYrs != 20 {
			t.Errorf("expected MaxPatentLifeYrs corrected to 20, got %d", req.Context.MaxPatentLifeYrs)
		}
	})
}

func TestPortfolioAssessmentRequest_Validate(t *testing.T) {
	t.Run("missing both ids", func(t *testing.T) {
		req := &PortfolioAssessmentRequest{}
		err := req.Validate()
		if err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("portfolio_id only", func(t *testing.T) {
		req := &PortfolioAssessmentRequest{PortfolioID: "PF001"}
		err := req.Validate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(req.Dimensions) != 4 {
			t.Errorf("expected 4 default dimensions, got %d", len(req.Dimensions))
		}
	})

	t.Run("patent_ids only", func(t *testing.T) {
		req := &PortfolioAssessmentRequest{PatentIDs: []string{"P1", "P2"}}
		err := req.Validate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCompareAssessmentsRequest_Validate(t *testing.T) {
	t.Run("too few ids", func(t *testing.T) {
		req := &CompareAssessmentsRequest{AssessmentIDs: []string{"A1"}}
		err := req.Validate()
		if err == nil {
			t.Fatal("expected validation error for < 2 ids")
		}
	})

	t.Run("valid", func(t *testing.T) {
		req := &CompareAssessmentsRequest{AssessmentIDs: []string{"A1", "A2"}}
		err := req.Validate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: AssessPatent
// ---------------------------------------------------------------------------

func TestAssessPatent_Success(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P001", "Distributed Cache System", "granted", 15, 4, 3)
	patentRepo.patents["P001"] = pat

	citationRepo := newMockCitationRepo()
	citationRepo.forwardCounts["P001"] = 25
	citationRepo.maxInDomain["G06F17/00"] = 100

	assessmentRepo := newMockAssessmentRepo()
	cache := newMockCache()

	svc := buildTestService(patentRepo, nil, assessmentRepo, nil, citationRepo, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P001"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("AssessPatent failed: %v", err)
	}

	// Verify response structure
	if resp.PatentID != "P001" {
		t.Errorf("PatentID = %s, want P001", resp.PatentID)
	}
	if resp.PatentTitle != pat.Title {
		t.Errorf("PatentTitle = %s, want %s", resp.PatentTitle, pat.Title)
	}
	if len(resp.Scores) != 4 {
		t.Errorf("expected 4 dimension scores, got %d", len(resp.Scores))
	}
	if resp.OverallScore == nil {
		t.Fatal("OverallScore is nil")
	}
	if resp.OverallScore.Score < 0 || resp.OverallScore.Score > 100 {
		t.Errorf("OverallScore.Score = %.2f, out of [0,100] range", resp.OverallScore.Score)
	}
	if resp.OverallScore.Tier == "" {
		t.Error("OverallScore.Tier is empty")
	}
	if resp.OverallScore.TierDescription == "" {
		t.Error("OverallScore.TierDescription is empty")
	}
	if resp.OverallScore.WeightedCalculation == "" {
		t.Error("OverallScore.WeightedCalculation is empty")
	}
	if resp.AssessedAt.IsZero() {
		t.Error("AssessedAt is zero")
	}
	if resp.AssessorType == "" {
		t.Error("AssessorType is empty")
	}

	// Verify each dimension has factors
	for dim, ds := range resp.Scores {
		if ds.MaxScore != 100 {
			t.Errorf("dimension %s MaxScore = %.0f, want 100", dim, ds.MaxScore)
		}
		if ds.Score < 0 || ds.Score > 100 {
			t.Errorf("dimension %s Score = %.2f, out of range", dim, ds.Score)
		}
		if len(ds.Factors) == 0 {
			t.Errorf("dimension %s has no factors", dim)
		}
		if ds.Explanation == "" {
			t.Errorf("dimension %s has empty explanation", dim)
		}
		for fname, fs := range ds.Factors {
			if fs.Score < 0 || fs.Score > 100 {
				t.Errorf("factor %s/%s Score = %.2f, out of range", dim, fname, fs.Score)
			}
			if fs.Weight <= 0 || fs.Weight > 1 {
				t.Errorf("factor %s/%s Weight = %.2f, out of range", dim, fname, fs.Weight)
			}
		}
	}

	// Verify recommendations generated
	if len(resp.Recommendations) == 0 {
		t.Error("expected at least one recommendation")
	}

	// Verify assessment was persisted
	if len(assessmentRepo.records) == 0 {
		t.Error("expected assessment record to be persisted")
	}

	// Verify result was cached
	cacheKey := cacheKeyPrefixAssess + "P001"
	if _, exists := cache.store[cacheKey]; !exists {
		t.Error("expected assessment result to be cached")
	}
}

func TestAssessPatent_CacheHit(t *testing.T) {
	cache := newMockCache()

	// Pre-populate cache
	cachedResp := &SinglePatentAssessmentResponse{
		PatentID:     "P001",
		PatentTitle:  "Cached Patent",
		OverallScore: &OverallValuation{Score: 88, Tier: TierA, TierDescription: "cached"},
		AssessedAt:   time.Now(),
	}
	data, _ := json.Marshal(cachedResp)
	cache.store[cacheKeyPrefixAssess+"P001"] = data

	svc := buildTestService(nil, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P001"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("AssessPatent with cache hit failed: %v", err)
	}
	if resp.PatentTitle != "Cached Patent" {
		t.Errorf("expected cached response, got title %s", resp.PatentTitle)
	}
	if resp.OverallScore.Score != 88 {
		t.Errorf("expected cached score 88, got %.2f", resp.OverallScore.Score)
	}
}

func TestAssessPatent_PatentNotFound(t *testing.T) {
	patentRepo := newMockPatentRepo() // empty
	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss") // force cache miss

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "NONEXISTENT"}
	_, err := svc.AssessPatent(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent patent")
	}
}

func TestAssessPatent_ValidationError(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	req := &SinglePatentAssessmentRequest{} // missing PatentID
	_, err := svc.AssessPatent(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAssessPatent_WithAIScorer(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P002", "AI Enhanced Patent", "granted", 20, 5, 1)
	patentRepo.patents["P002"] = pat

	aiScorer := newMockAIScorer()
	aiScorer.scores[DimensionTechnicalValue] = map[string]float64{
		"novelty":                 95,
		"inventive_step":          90,
		"technical_breadth":       85,
		"performance_improvement": 88,
		"citation_impact":         92,
	}
	aiScorer.scores[DimensionLegalValue] = map[string]float64{
		"claim_breadth":        88,
		"claim_clarity":        85,
		"prosecution_strength": 90,
		"remaining_life_years": 95,
		"family_coverage":      80,
	}
	aiScorer.scores[DimensionCommercialValue] = map[string]float64{
		"market_relevance":      92,
		"product_coverage":      88,
		"licensing_potential":   90,
		"cost_of_design_around": 85,
		"industry_adoption":     87,
	}
	aiScorer.scores[DimensionStrategicValue] = map[string]float64{
		"portfolio_centrality":              90,
		"blocking_power":                    92,
		"negotiation_leverage":              88,
		"technology_trajectory_alignment":   95,
		"competitive_differentiation":       85,
	}

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, aiScorer, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P002"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("AssessPatent with AI scorer failed: %v", err)
	}

	// With high AI scores, should be Tier S or A
	if resp.OverallScore.Tier != TierS && resp.OverallScore.Tier != TierA {
		t.Errorf("expected Tier S or A with high AI scores, got %s (score: %.2f)",
			resp.OverallScore.Tier, resp.OverallScore.Score)
	}

	// Verify AI scorer was called
	if atomic.LoadInt32(&aiScorer.calls) == 0 {
		t.Error("expected AI scorer to be called")
	}

	// Verify assessor type is hybrid
	if resp.AssessorType != AssessorHybrid {
		t.Errorf("expected AssessorType=hybrid, got %s", resp.AssessorType)
	}
}

func TestAssessPatent_AIFallbackOnError(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P003", "Fallback Patent", "granted", 10, 3, 5)
	patentRepo.patents["P003"] = pat

	aiScorer := newMockAIScorer()
	aiScorer.err = fmt.Errorf("AI service unavailable")

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, aiScorer, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P003"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("AssessPatent should succeed with AI fallback, got: %v", err)
	}

	// Should still produce valid scores via rule-based fallback
	if resp.OverallScore == nil {
		t.Fatal("OverallScore should not be nil on AI fallback")
	}
	if resp.OverallScore.Score < 0 || resp.OverallScore.Score > 100 {
		t.Errorf("score out of range on fallback: %.2f", resp.OverallScore.Score)
	}
}

func TestAssessPatent_NoAIScorer(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P004", "Rule Only Patent", "pending", 8, 2, 8)
	patentRepo.patents["P004"] = pat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	// nil AI scorer
	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P004"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without AI scorer, assessor type should reflect that
	// The implementation sets AssessorHuman when aiScorer is nil
	if resp.AssessorType != AssessorHuman {
		t.Errorf("expected AssessorType=human without AI scorer, got %s", resp.AssessorType)
	}
}

func TestAssessPatent_SpecificDimensions(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P005", "Partial Dimensions", "granted", 12, 3, 4)
	patentRepo.patents["P005"] = pat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{
		PatentID:   "P005",
		Dimensions: []AssessmentDimension{DimensionTechnicalValue, DimensionLegalValue},
	}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Scores) != 2 {
		t.Errorf("expected 2 dimension scores, got %d", len(resp.Scores))
	}
	if _, ok := resp.Scores[DimensionTechnicalValue]; !ok {
		t.Error("missing technical_value dimension")
	}
	if _, ok := resp.Scores[DimensionLegalValue]; !ok {
		t.Error("missing legal_value dimension")
	}
	if _, ok := resp.Scores[DimensionCommercialValue]; ok {
		t.Error("commercial_value should not be present")
	}
}

func TestAssessPatent_PersistenceFailureNonFatal(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P006", "Persist Fail", "granted", 10, 3, 2)
	patentRepo.patents["P006"] = pat

	assessmentRepo := newMockAssessmentRepo()
	assessmentRepo.err = fmt.Errorf("database write error")

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, assessmentRepo, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P006"}
	resp, err := svc.AssessPatent(context.Background(), req)
	// Should succeed even if persistence fails
	if err != nil {
		t.Fatalf("expected success despite persistence failure, got: %v", err)
	}
	if resp.OverallScore == nil {
		t.Fatal("OverallScore should not be nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: Tier Boundary Verification
// ---------------------------------------------------------------------------

func TestTierBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected PatentTier
	}{
		{"exact S boundary", 90.0, TierS},
		{"just below S", 89.99, TierA},
		{"exact A boundary", 80.0, TierA},
		{"just below A", 79.99, TierB},
		{"exact B boundary", 65.0, TierB},
		{"just below B", 64.99, TierC},
		{"exact C boundary", 50.0, TierC},
		{"just below C", 49.99, TierD},
		{"zero", 0.0, TierD},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TierFromScore(tt.score)
			if got != tt.expected {
				t.Errorf("TierFromScore(%.2f) = %s, want %s", tt.score, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: AssessPortfolio
// ---------------------------------------------------------------------------

func TestAssessPortfolio_Success(t *testing.T) {
	patentRepo := newMockPatentRepo()
	patentRepo.patents["P1"] = makeTestPatent("P1", "Patent One", "granted", 20, 5, 1)
	patentRepo.patents["P2"] = makeTestPatent("P2", "Patent Two", "granted", 10, 3, 5)
	patentRepo.patents["P3"] = makeTestPatent("P3", "Patent Three", "pending", 5, 1, 10)

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{
		PortfolioID:             "PF001",
		PatentIDs:               []string{"P1", "P2", "P3"},
		IncludeCostOptimization: true,
		Context: &AssessmentContext{
			CurrencyCode:     "CNY",
			MaxPatentLifeYrs: 20,
			TechFocusAreas:   []string{"distributed", "cache"},
		},
	}

	resp, err := svc.AssessPortfolio(context.Background(), req)
	if err != nil {
		t.Fatalf("AssessPortfolio failed: %v", err)
	}

	if resp.PortfolioID != "PF001" {
		t.Errorf("PortfolioID = %s, want PF001", resp.PortfolioID)
	}
	if len(resp.Assessments) != 3 {
		t.Errorf("expected 3 assessments, got %d", len(resp.Assessments))
	}

	// Verify summary
	if resp.Summary == nil {
		t.Fatal("Summary is nil")
	}
	if resp.Summary.TotalAssessed != 3 {
		t.Errorf("TotalAssessed = %d, want 3", resp.Summary.TotalAssessed)
	}
	if resp.Summary.AverageScore <= 0 {
		t.Error("AverageScore should be positive")
	}

	// Verify tier distribution sums to total
	tierSum := 0
	for _, count := range resp.Summary.TierDistribution {
		tierSum += count
	}
	if tierSum != 3 {
		t.Errorf("tier distribution sum = %d, want 3", tierSum)
	}

	// Verify cost optimization present
	if resp.CostOptimization == nil {
		t.Fatal("CostOptimization should be present when requested")
	}
	if resp.CostOptimization.CurrentAnnualCost <= 0 {
		t.Error("CurrentAnnualCost should be positive")
	}
	if resp.CostOptimization.Currency != "CNY" {
		t.Errorf("Currency = %s, want CNY", resp.CostOptimization.Currency)
	}
	if len(resp.CostOptimization.Recommendations) != 3 {
		t.Errorf("expected 3 cost recommendations, got %d", len(resp.CostOptimization.Recommendations))
	}

	if resp.AssessedAt.IsZero() {
		t.Error("AssessedAt is zero")
	}
}

func TestAssessPortfolio_FromPortfolioID(t *testing.T) {
	patentRepo := newMockPatentRepo()
	patentRepo.patents["P10"] = makeTestPatent("P10", "Portfolio Patent", "granted", 12, 3, 2)

	portfolioRepo := newMockPortfolioRepo()
	portfolioRepo.portfolios["PF100"] = &domainportfolio.Portfolio{
		ID:        "PF100",
		PatentIDs: []string{"P10"},
	}

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, portfolioRepo, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{PortfolioID: "PF100"}
	resp, err := svc.AssessPortfolio(context.Background(), req)
	if err != nil {
		t.Fatalf("AssessPortfolio from portfolio ID failed: %v", err)
	}
	if len(resp.Assessments) != 1 {
		t.Errorf("expected 1 assessment, got %d", len(resp.Assessments))
	}
}

func TestAssessPortfolio_PortfolioNotFound(t *testing.T) {
	portfolioRepo := newMockPortfolioRepo() // empty

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(nil, portfolioRepo, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{PortfolioID: "NONEXISTENT"}
	_, err := svc.AssessPortfolio(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent portfolio")
	}
}

func TestAssessPortfolio_EmptyPatentList(t *testing.T) {
	portfolioRepo := newMockPortfolioRepo()
	portfolioRepo.portfolios["PF_EMPTY"] = &domainportfolio.Portfolio{
		ID:        "PF_EMPTY",
		PatentIDs: []string{},
	}

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(nil, portfolioRepo, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{PortfolioID: "PF_EMPTY"}
	_, err := svc.AssessPortfolio(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty patent list")
	}
}

func TestAssessPortfolio_ValidationError(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	req := &PortfolioAssessmentRequest{} // missing both IDs
	_, err := svc.AssessPortfolio(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAssessPortfolio_ConcurrencyControl(t *testing.T) {
	patentRepo := newMockPatentRepo()
	// Create 20 patents to test concurrency (config concurrency = 5)
	ids := make([]string, 20)
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("PC%03d", i)
		ids[i] = id
		patentRepo.patents[id] = makeTestPatent(id, fmt.Sprintf("Concurrent Patent %d", i), "granted", 10, 3, 3)
	}

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{
		PortfolioID: "PF_CONC",
		PatentIDs:   ids,
	}

	resp, err := svc.AssessPortfolio(context.Background(), req)
	if err != nil {
		t.Fatalf("concurrent portfolio assessment failed: %v", err)
	}
	if len(resp.Assessments) != 20 {
		t.Errorf("expected 20 assessments, got %d", len(resp.Assessments))
	}
}

func TestAssessPortfolio_PartialFailure(t *testing.T) {
	patentRepo := newMockPatentRepo()
	patentRepo.patents["P_OK"] = makeTestPatent("P_OK", "Good Patent", "granted", 10, 3, 2)
	// P_FAIL is not in the repo, so it will fail

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{
		PortfolioID: "PF_PARTIAL",
		PatentIDs:   []string{"P_OK", "P_FAIL"},
	}

	resp, err := svc.AssessPortfolio(context.Background(), req)
	if err != nil {
		t.Fatalf("expected partial success, got error: %v", err)
	}
	// Only P_OK should succeed
	if len(resp.Assessments) != 1 {
		t.Errorf("expected 1 successful assessment, got %d", len(resp.Assessments))
	}
	if resp.Summary.TotalAssessed != 1 {
		t.Errorf("TotalAssessed = %d, want 1", resp.Summary.TotalAssessed)
	}
}

func TestAssessPortfolio_WithoutCostOptimization(t *testing.T) {
	patentRepo := newMockPatentRepo()
	patentRepo.patents["P20"] = makeTestPatent("P20", "No Cost Opt", "granted", 10, 3, 2)

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{
		PortfolioID:             "PF_NOCOST",
		PatentIDs:               []string{"P20"},
		IncludeCostOptimization: false,
	}

	resp, err := svc.AssessPortfolio(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CostOptimization != nil {
		t.Error("CostOptimization should be nil when not requested")
	}
}

// ---------------------------------------------------------------------------
// Tests: Cost Optimization Logic
// ---------------------------------------------------------------------------

func TestCostOptimization_TierBasedActions(t *testing.T) {
	patentRepo := newMockPatentRepo()
	// Create patents that will land in different tiers based on their characteristics
	// High-value: many claims, many IPCs, granted, recent, large family
	patentRepo.patents["PS"] = makeTestPatent("PS", "S-Tier Patent", "granted", 25, 6, 0.5)
	// Low-value: few claims, few IPCs, pending, old, small family
	lowPat := makeTestPatent("PD", "D-Tier Patent", "pending", 2, 1, 18)
	lowPat.FamilyMembers = nil
	lowPat.Abstract = "simple method"
	lowPat.Description = "short"
	patentRepo.patents["PD"] = lowPat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &PortfolioAssessmentRequest{
		PortfolioID:             "PF_COST",
		PatentIDs:               []string{"PS", "PD"},
		IncludeCostOptimization: true,
		Context:                 &AssessmentContext{CurrencyCode: "CNY", MaxPatentLifeYrs: 20},
	}

	resp, err := svc.AssessPortfolio(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.CostOptimization == nil {
		t.Fatal("CostOptimization should be present")
	}

	co := resp.CostOptimization
	if co.SavingsPotential < 0 {
		t.Errorf("SavingsPotential should be >= 0, got %.2f", co.SavingsPotential)
	}
	if co.OptimizedAnnualCost > co.CurrentAnnualCost {
		t.Error("OptimizedAnnualCost should not exceed CurrentAnnualCost")
	}

	// Verify each recommendation has valid action
	for _, rec := range co.Recommendations {
		switch rec.Action {
		case CostContinueMaintain, CostReduceScope, CostAbandon, CostLicense:
			// valid
		default:
			t.Errorf("unexpected cost action: %s", rec.Action)
		}
		if rec.PatentID == "" {
			t.Error("cost recommendation missing PatentID")
		}
		if rec.Reason == "" {
			t.Error("cost recommendation missing Reason")
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: GetAssessmentHistory
// ---------------------------------------------------------------------------

func TestGetAssessmentHistory_Success(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()

	// Insert records at different times
	now := time.Now()
	for i := 0; i < 5; i++ {
		rec := &AssessmentRecord{
			ID:           fmt.Sprintf("AR%03d", i),
			PatentID:     "P_HIST",
			OverallScore: float64(60 + i*5),
			Tier:         TierFromScore(float64(60 + i*5)),
			AssessedAt:   now.Add(time.Duration(i) * time.Hour),
			AssessorType: AssessorHybrid,
		}
		_ = assessmentRepo.Save(context.Background(), rec)
	}

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	records, err := svc.GetAssessmentHistory(context.Background(), "P_HIST")
	if err != nil {
		t.Fatalf("GetAssessmentHistory failed: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}

	// Verify sorted descending by assessed_at
	for i := 1; i < len(records); i++ {
		if records[i].AssessedAt.After(records[i-1].AssessedAt) {
			t.Error("records not sorted descending by assessed_at")
			break
		}
	}
}

func TestGetAssessmentHistory_WithLimit(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	for i := 0; i < 10; i++ {
		rec := &AssessmentRecord{
			ID:           fmt.Sprintf("ARL%03d", i),
			PatentID:     "P_LIM",
			OverallScore: 70,
			Tier:         TierB,
			AssessedAt:   time.Now().Add(time.Duration(i) * time.Hour),
			AssessorType: AssessorAI,
		}
		_ = assessmentRepo.Save(context.Background(), rec)
	}

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	records, err := svc.GetAssessmentHistory(context.Background(), "P_LIM", WithLimit(3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records with limit, got %d", len(records))
	}
}

func TestGetAssessmentHistory_EmptyPatentID(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetAssessmentHistory(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty patent_id")
	}
}

func TestGetAssessmentHistory_RepoError(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	assessmentRepo.err = fmt.Errorf("database connection lost")

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	_, err := svc.GetAssessmentHistory(context.Background(), "P_ERR")
	if err == nil {
		t.Fatal("expected error from repository")
	}
}

// ---------------------------------------------------------------------------
// Tests: CompareAssessments
// ---------------------------------------------------------------------------

func TestCompareAssessments_Success(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()

	now := time.Now()
	rec1 := &AssessmentRecord{
		ID:           "CMP1",
		PatentID:     "P_CMP",
		OverallScore: 60,
		Tier:         TierC,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  55,
			DimensionLegalValue:      65,
			DimensionCommercialValue: 58,
			DimensionStrategicValue:  62,
		},
		AssessedAt:   now.Add(-30 * 24 * time.Hour),
		AssessorType: AssessorHybrid,
	}
	rec2 := &AssessmentRecord{
		ID:           "CMP2",
		PatentID:     "P_CMP",
		OverallScore: 78,
		Tier:         TierB,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  72,
			DimensionLegalValue:      80,
			DimensionCommercialValue: 75,
			DimensionStrategicValue:  82,
		},
		AssessedAt:   now,
		AssessorType: AssessorHybrid,
	}
	_ = assessmentRepo.Save(context.Background(), rec1)
	_ = assessmentRepo.Save(context.Background(), rec2)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	req := &CompareAssessmentsRequest{AssessmentIDs: []string{"CMP1", "CMP2"}}
	resp, err := svc.CompareAssessments(context.Background(), req)
	if err != nil {
		t.Fatalf("CompareAssessments failed: %v", err)
	}

	if len(resp.Comparisons) != 1 {
		t.Fatalf("expected 1 comparison (same patent), got %d", len(resp.Comparisons))
	}

	comp := resp.Comparisons[0]
	if comp.PatentID != "P_CMP" {
		t.Errorf("PatentID = %s, want P_CMP", comp.PatentID)
	}
	if comp.Trend != "improving" {
		t.Errorf("Trend = %s, want improving (60→78)", comp.Trend)
	}
	if len(comp.Assessments) != 2 {
		t.Errorf("expected 2 assessments in comparison, got %d", len(comp.Assessments))
	}

	// Verify delta analysis
	if resp.DeltaAnalysis == nil {
		t.Fatal("DeltaAnalysis is nil")
	}
	if resp.DeltaAnalysis.ImprovedCount != 1 {
		t.Errorf("ImprovedCount = %d, want 1", resp.DeltaAnalysis.ImprovedCount)
	}

	// Technical value changed from 55 to 72 = ~30.9% change → significant
	foundTechChange := false
	for _, sc := range resp.DeltaAnalysis.SignificantChanges {
		if sc.Dimension == DimensionTechnicalValue {
			foundTechChange = true
			if sc.OldScore != 55 || sc.NewScore != 72 {
				t.Errorf("unexpected tech scores: old=%.1f new=%.1f", sc.OldScore, sc.NewScore)
			}
		}
	}
	if !foundTechChange {
		t.Error("expected significant change for technical_value dimension")
	}
}

func TestCompareAssessments_Declining(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()

	now := time.Now()
	rec1 := &AssessmentRecord{
		ID: "DEC1", PatentID: "P_DEC", OverallScore: 85, Tier: TierA,
		DimensionScores: map[AssessmentDimension]float64{},
		AssessedAt: now.Add(-60 * 24 * time.Hour), AssessorType: AssessorAI,
	}
	rec2 := &AssessmentRecord{
		ID: "DEC2", PatentID: "P_DEC", OverallScore: 55, Tier: TierC,
		DimensionScores: map[AssessmentDimension]float64{},
		AssessedAt: now, AssessorType: AssessorAI,
	}
	_ = assessmentRepo.Save(context.Background(), rec1)
	_ = assessmentRepo.Save(context.Background(), rec2)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	req := &CompareAssessmentsRequest{AssessmentIDs: []string{"DEC1", "DEC2"}}
	resp, err := svc.CompareAssessments(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Comparisons[0].Trend != "declining" {
		t.Errorf("Trend = %s, want declining (85→55)", resp.Comparisons[0].Trend)
	}
	if resp.DeltaAnalysis.DeclinedCount != 1 {
		t.Errorf("DeclinedCount = %d, want 1", resp.DeltaAnalysis.DeclinedCount)
	}
}

func TestCompareAssessments_Stable(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()

	now := time.Now()
	rec1 := &AssessmentRecord{
		ID: "STB1", PatentID: "P_STB", OverallScore: 72, Tier: TierB,
		DimensionScores: map[AssessmentDimension]float64{},
		AssessedAt: now.Add(-10 * 24 * time.Hour), AssessorType: AssessorHybrid,
	}
	rec2 := &AssessmentRecord{
		ID: "STB2", PatentID: "P_STB", OverallScore: 74, Tier: TierB,
		DimensionScores: map[AssessmentDimension]float64{},
		AssessedAt: now, AssessorType: AssessorHybrid,
	}
	_ = assessmentRepo.Save(context.Background(), rec1)
	_ = assessmentRepo.Save(context.Background(), rec2)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	req := &CompareAssessmentsRequest{AssessmentIDs: []string{"STB1", "STB2"}}
	resp, err := svc.CompareAssessments(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Comparisons[0].Trend != "stable" {
		t.Errorf("Trend = %s, want stable (72→74, diff=2 < 5)", resp.Comparisons[0].Trend)
	}
}

func TestCompareAssessments_ValidationError(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	req := &CompareAssessmentsRequest{AssessmentIDs: []string{"ONLY_ONE"}}
	_, err := svc.CompareAssessments(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error for < 2 IDs")
	}
}

func TestCompareAssessments_NotEnoughRecords(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	// Only one record exists
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "LONE", PatentID: "P_LONE", OverallScore: 70, Tier: TierB,
		AssessedAt: time.Now(), AssessorType: AssessorAI,
	})

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	req := &CompareAssessmentsRequest{AssessmentIDs: []string{"LONE", "MISSING"}}
	_, err := svc.CompareAssessments(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when not enough records found")
	}
}

// ---------------------------------------------------------------------------
// Tests: ExportAssessment
// ---------------------------------------------------------------------------

func TestExportAssessment_JSON(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID:           "EXP_JSON",
		PatentID:     "P_EXP",
		OverallScore: 75,
		Tier:         TierB,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue: 70,
			DimensionLegalValue:     80,
		},
		AssessedAt:   time.Now(),
		AssessorType: AssessorHybrid,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	data, err := svc.ExportAssessment(context.Background(), "EXP_JSON", ExportJSON)
	if err != nil {
		t.Fatalf("ExportAssessment JSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported JSON is empty")
	}

	// Verify it's valid JSON
	var parsed AssessmentRecord
	if jsonErr := json.Unmarshal(data, &parsed); jsonErr != nil {
		t.Fatalf("exported data is not valid JSON: %v", jsonErr)
	}
	if parsed.ID != "EXP_JSON" {
		t.Errorf("parsed ID = %s, want EXP_JSON", parsed.ID)
	}
	if parsed.OverallScore != 75 {
		t.Errorf("parsed OverallScore = %.2f, want 75", parsed.OverallScore)
	}
}

func TestExportAssessment_CSV(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID:           "EXP_CSV",
		PatentID:     "P_CSV",
		PortfolioID:  "PF_CSV",
		OverallScore: 82,
		Tier:         TierA,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  80,
			DimensionLegalValue:      85,
			DimensionCommercialValue: 78,
			DimensionStrategicValue:  84,
		},
		AssessedAt:   time.Now(),
		AssessorType: AssessorAI,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	data, err := svc.ExportAssessment(context.Background(), "EXP_CSV", ExportCSV)
	if err != nil {
		t.Fatalf("ExportAssessment CSV failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported CSV is empty")
	}

	csvStr := string(data)
	// Should contain header and one data row
	lines := strings.Split(strings.TrimSpace(csvStr), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 CSV lines (header + row), got %d", len(lines))
	}
	// Header should contain standard fields
	if !strings.Contains(lines[0], "ID") {
		t.Error("CSV header missing ID column")
	}
	if !strings.Contains(lines[0], "OverallScore") {
		t.Error("CSV header missing OverallScore column")
	}
	// Data row should contain the record ID
	if !strings.Contains(lines[1], "EXP_CSV") {
		t.Error("CSV data row missing record ID")
	}
}

func TestExportAssessment_PDF_Fallback(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID:           "EXP_PDF",
		PatentID:     "P_PDF",
		OverallScore: 70,
		Tier:         TierB,
		AssessedAt:   time.Now(),
		AssessorType: AssessorHuman,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	data, err := svc.ExportAssessment(context.Background(), "EXP_PDF", ExportPDF)
	if err != nil {
		t.Fatalf("ExportAssessment PDF failed: %v", err)
	}
	// PDF falls back to JSON in current implementation
	if len(data) == 0 {
		t.Fatal("exported PDF fallback is empty")
	}
}

func TestExportAssessment_UnsupportedFormat(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID: "EXP_BAD", PatentID: "P_BAD", OverallScore: 50, Tier: TierC,
		AssessedAt: time.Now(), AssessorType: AssessorAI,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	_, err := svc.ExportAssessment(context.Background(), "EXP_BAD", ExportFormat("xml"))
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestExportAssessment_NotFound(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	_, err := svc.ExportAssessment(context.Background(), "NONEXISTENT", ExportJSON)
	if err == nil {
		t.Fatal("expected error for non-existent assessment")
	}
}

func TestExportAssessment_EmptyID(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.ExportAssessment(context.Background(), "", ExportJSON)
	if err == nil {
		t.Fatal("expected validation error for empty assessment_id")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetTierDistribution
// ---------------------------------------------------------------------------

func TestGetTierDistribution_Success(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()

	now := time.Now()
	records := []*AssessmentRecord{
		{ID: "TD1", PatentID: "P1", PortfolioID: "PF_TD", OverallScore: 95, Tier: TierS, AssessedAt: now},
		{ID: "TD2", PatentID: "P2", PortfolioID: "PF_TD", OverallScore: 85, Tier: TierA, AssessedAt: now},
		{ID: "TD3", PatentID: "P3", PortfolioID: "PF_TD", OverallScore: 70, Tier: TierB, AssessedAt: now},
		{ID: "TD4", PatentID: "P4", PortfolioID: "PF_TD", OverallScore: 55, Tier: TierC, AssessedAt: now},
		{ID: "TD5", PatentID: "P5", PortfolioID: "PF_TD", OverallScore: 30, Tier: TierD, AssessedAt: now},
	}
	for _, r := range records {
		_ = assessmentRepo.Save(context.Background(), r)
	}

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, cache)

	td, err := svc.GetTierDistribution(context.Background(), "PF_TD")
	if err != nil {
		t.Fatalf("GetTierDistribution failed: %v", err)
	}

	if td.PortfolioID != "PF_TD" {
		t.Errorf("PortfolioID = %s, want PF_TD", td.PortfolioID)
	}
	if td.Total != 5 {
		t.Errorf("Total = %d, want 5", td.Total)
	}
	if td.Distribution[TierS] != 1 {
		t.Errorf("TierS count = %d, want 1", td.Distribution[TierS])
	}
	if td.Distribution[TierA] != 1 {
		t.Errorf("TierA count = %d, want 1", td.Distribution[TierA])
	}
	if td.Distribution[TierB] != 1 {
		t.Errorf("TierB count = %d, want 1", td.Distribution[TierB])
	}
	if td.Distribution[TierC] != 1 {
		t.Errorf("TierC count = %d, want 1", td.Distribution[TierC])
	}
	if td.Distribution[TierD] != 1 {
		t.Errorf("TierD count = %d, want 1", td.Distribution[TierD])
	}
	if td.ComputedAt.IsZero() {
		t.Error("ComputedAt is zero")
	}

	// Verify cached
	cacheKey := cacheKeyPrefixTierDist + "PF_TD"
	if _, exists := cache.store[cacheKey]; !exists {
		t.Error("tier distribution should be cached")
	}
}

func TestGetTierDistribution_Deduplication(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()

	now := time.Now()
	// Same patent assessed twice; only latest should count
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "DD1", PatentID: "P_DUP", PortfolioID: "PF_DD", OverallScore: 55, Tier: TierC,
		AssessedAt: now.Add(-24 * time.Hour), AssessorType: AssessorAI,
	})
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "DD2", PatentID: "P_DUP", PortfolioID: "PF_DD", OverallScore: 85, Tier: TierA,
		AssessedAt: now, AssessorType: AssessorAI,
	})

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, cache)

	td, err := svc.GetTierDistribution(context.Background(), "PF_DD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if td.Total != 1 {
		t.Errorf("Total = %d, want 1 (deduplicated)", td.Total)
	}
	if td.Distribution[TierA] != 1 {
		t.Errorf("expected latest assessment (TierA), got distribution: %v", td.Distribution)
	}
	if td.Distribution[TierC] != 0 {
		t.Errorf("old assessment (TierC) should not appear, got count %d", td.Distribution[TierC])
	}
}

func TestGetTierDistribution_CacheHit(t *testing.T) {
	cache := newMockCache()
	cached := &TierDistribution{
		PortfolioID:  "PF_CACHED",
		Distribution: map[PatentTier]int{TierS: 2, TierA: 3, TierB: 5, TierC: 1, TierD: 0},
		Total:        11,
		ComputedAt:   time.Now(),
	}
	data, _ := json.Marshal(cached)
	cache.store[cacheKeyPrefixTierDist+"PF_CACHED"] = data

	svc := buildTestService(nil, nil, nil, nil, nil, cache)

	td, err := svc.GetTierDistribution(context.Background(), "PF_CACHED")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if td.Total != 11 {
		t.Errorf("expected cached Total=11, got %d", td.Total)
	}
}

func TestGetTierDistribution_EmptyPortfolioID(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetTierDistribution(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty portfolio_id")
	}
}

// ---------------------------------------------------------------------------
// Tests: RecommendActions
// ---------------------------------------------------------------------------

func TestRecommendActions_TierS(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "RA_S", PatentID: "P_S", OverallScore: 95, Tier: TierS,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue: 95, DimensionLegalValue: 92,
			DimensionCommercialValue: 96, DimensionStrategicValue: 97,
		},
		AssessedAt: time.Now(), AssessorType: AssessorHybrid,
	})

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	recs, err := svc.RecommendActions(context.Background(), "RA_S")
	if err != nil {
		t.Fatalf("RecommendActions failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation for Tier S")
	}

	// Should include maintain action with critical priority
	foundMaintain := false
	for _, r := range recs {
		if r.Type == ActionMaintain && r.Priority == PriorityCritical {
			foundMaintain = true
		}
	}
	if !foundMaintain {
		t.Error("expected critical maintain recommendation for Tier S patent")
	}

	// Verify sorted by priority
	priorityOrder := map[ActionPriority]int{
		PriorityCritical: 0, PriorityHigh: 1, PriorityMedium: 2, PriorityLow: 3,
	}
	for i := 1; i < len(recs); i++ {
		if priorityOrder[recs[i].Priority] < priorityOrder[recs[i-1].Priority] {
			t.Error("recommendations not sorted by priority")
			break
		}
	}
}

func TestRecommendActions_TierD(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "RA_D", PatentID: "P_D", OverallScore: 30, Tier: TierD,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue: 25, DimensionLegalValue: 30,
			DimensionCommercialValue: 35, DimensionStrategicValue: 28,
		},
		AssessedAt: time.Now(), AssessorType: AssessorAI,
	})

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	recs, err := svc.RecommendActions(context.Background(), "RA_D")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundAbandon := false
	for _, r := range recs {
		if r.Type == ActionAbandon {
			foundAbandon = true
		}
	}
	if !foundAbandon {
		t.Error("expected abandon recommendation for Tier D patent")
	}
}

func TestRecommendActions_TierA_WithWeakDimension(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "RA_A_WEAK", PatentID: "P_A_WEAK", OverallScore: 82, Tier: TierA,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  90,
			DimensionLegalValue:      85,
			DimensionCommercialValue: 45, // weak
			DimensionStrategicValue:  88,
		},
		AssessedAt: time.Now(), AssessorType: AssessorHybrid,
	})

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	recs, err := svc.RecommendActions(context.Background(), "RA_A_WEAK")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundStrengthen := false
	for _, r := range recs {
		if r.Type == ActionStrengthen {
			foundStrengthen = true
			if !strings.Contains(r.Action, "commercial_value") {
				t.Errorf("strengthen recommendation should mention commercial_value, got: %s", r.Action)
			}
		}
	}
	if !foundStrengthen {
		t.Error("expected strengthen recommendation for weak commercial dimension in Tier A patent")
	}
}

func TestRecommendActions_TierB_HighCommercial(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "RA_B_LIC", PatentID: "P_B_LIC", OverallScore: 72, Tier: TierB,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  65,
			DimensionLegalValue:      68,
			DimensionCommercialValue: 82, // high commercial
			DimensionStrategicValue:  70,
		},
		AssessedAt: time.Now(), AssessorType: AssessorHybrid,
	})

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	recs, err := svc.RecommendActions(context.Background(), "RA_B_LIC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundLicense := false
	for _, r := range recs {
		if r.Type == ActionLicense {
			foundLicense = true
		}
	}
	if !foundLicense {
		t.Error("expected license recommendation for Tier B patent with high commercial value")
	}
}

func TestRecommendActions_CrossDimensional_FileNew(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "RA_FN", PatentID: "P_FN", OverallScore: 68, Tier: TierB,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  70,
			DimensionLegalValue:      65,
			DimensionCommercialValue: 40, // low commercial
			DimensionStrategicValue:  85, // high strategic
		},
		AssessedAt: time.Now(), AssessorType: AssessorHybrid,
	})

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	recs, err := svc.RecommendActions(context.Background(), "RA_FN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundFileNew := false
	for _, r := range recs {
		if r.Type == ActionFileNew {
			foundFileNew = true
		}
	}
	if !foundFileNew {
		t.Error("expected file_new recommendation when strategic is high but commercial is low")
	}
}

func TestRecommendActions_NotFound(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	_, err := svc.RecommendActions(context.Background(), "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent assessment")
	}
}

func TestRecommendActions_EmptyID(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.RecommendActions(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty assessment_id")
	}
}

// ---------------------------------------------------------------------------
// Tests: Rule-Based Factor Scoring
// ---------------------------------------------------------------------------

func TestRuleTechnicalFactor_Novelty(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// Patent with 3 independent out of 15 total claims
	pat := makeTestPatent("PT1", "Test", "granted", 15, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionTechnicalValue, "novelty", assessCtx)
	if score < 20 || score > 95 {
		t.Errorf("novelty score = %.2f, out of expected range [20, 95]", score)
	}

	// Patent with 0 claims
	patNoClaims := makeTestPatent("PT2", "No Claims", "granted", 0, 3, 2)
	score0 := svc.ruleBasedFactorScore(ctx, patNoClaims, DimensionTechnicalValue, "novelty", assessCtx)
	if score0 != 40 {
		t.Errorf("novelty score for 0 claims = %.2f, want 40", score0)
	}
}

func TestRuleTechnicalFactor_InventiveStep(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// Long description (>10000 chars from makeTestPatent)
	pat := makeTestPatent("PT3", "Long Desc", "granted", 10, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionTechnicalValue, "inventive_step", assessCtx)
	if score != 80 {
		t.Errorf("inventive_step for long desc = %.2f, want 80", score)
	}

	// Short description
	patShort := makeTestPatent("PT4", "Short", "granted", 10, 3, 2)
	patShort.Description = "brief"
	scoreShort := svc.ruleBasedFactorScore(ctx, patShort, DimensionTechnicalValue, "inventive_step", assessCtx)
	if scoreShort != 35 {
		t.Errorf("inventive_step for short desc = %.2f, want 35", scoreShort)
	}
}

func TestRuleTechnicalFactor_TechnicalBreadth(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	tests := []struct {
		ipcCount int
		minScore float64
		maxScore float64
	}{
		{0, 30, 30},
		{1, 50, 50},
		{3, 70, 70},
		{5, 90, 90},
		{8, 90, 90},
	}

	for _, tt := range tests {
		pat := makeTestPatent("PTB", "Breadth", "granted", 10, tt.ipcCount, 2)
		score := svc.ruleBasedFactorScore(ctx, pat, DimensionTechnicalValue, "technical_breadth", assessCtx)
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("technical_breadth(ipc=%d) = %.2f, want [%.0f, %.0f]", tt.ipcCount, score, tt.minScore, tt.maxScore)
		}
	}
}

func TestRuleTechnicalFactor_PerformanceImprovement(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// makeTestPatent abstract contains "improve", "enhance", "reduce", "提高", "优化" → 5 matches
	pat := makeTestPatent("PTP", "Perf", "granted", 10, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionTechnicalValue, "performance_improvement", assessCtx)
	if score < 60 {
		t.Errorf("performance_improvement with keywords = %.2f, expected >= 60", score)
	}

	// No keywords
	patNoKw := makeTestPatent("PTP2", "Plain", "granted", 10, 3, 2)
	patNoKw.Abstract = "a method for data processing"
	scoreNoKw := svc.ruleBasedFactorScore(ctx, patNoKw, DimensionTechnicalValue, "performance_improvement", assessCtx)
	if scoreNoKw != 20 {
		t.Errorf("performance_improvement without keywords = %.2f, want 20", scoreNoKw)
	}
}

func TestRuleLegalFactor_ClaimBreadth(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// makeTestPatent creates claims with ~14 words each, first 3 independent
	pat := makeTestPatent("PLB", "Claim Breadth", "granted", 15, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionLegalValue, "claim_breadth", assessCtx)
	// 14 words < 50 → score should be 90
	if score != 90 {
		t.Errorf("claim_breadth for short claims = %.2f, want 90", score)
	}
}

func TestRuleLegalFactor_ProsecutionStrength(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	tests := []struct {
		status   string
		expected float64
	}{
		{"granted", 85},
		{"active", 85},
		{"pending", 55},
		{"rejected", 15},
		{"withdrawn", 15},
		{"unknown", 50},
	}

	for _, tt := range tests {
		pat := makeTestPatent("PPS", "Prosecution", tt.status, 10, 3, 2)
		score := svc.ruleBasedFactorScore(ctx, pat, DimensionLegalValue, "prosecution_strength", assessCtx)
		if score != tt.expected {
			t.Errorf("prosecution_strength(%s) = %.2f, want %.2f", tt.status, score, tt.expected)
		}
	}
}

func TestRuleLegalFactor_RemainingLife(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// 3 years old → 17/20 remaining → ~85
	pat := makeTestPatent("PRL", "Remaining Life", "granted", 10, 3, 3)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionLegalValue, "remaining_life_years", assessCtx)
	if score < 80 || score > 90 {
		t.Errorf("remaining_life(3yr old) = %.2f, expected ~85", score)
	}

	// 18 years old → 2/20 remaining → ~10
	patOld := makeTestPatent("PRL2", "Old Patent", "granted", 10, 3, 18)
	scoreOld := svc.ruleBasedFactorScore(ctx, patOld, DimensionLegalValue, "remaining_life_years", assessCtx)
	if scoreOld > 15 {
		t.Errorf("remaining_life(18yr old) = %.2f, expected <= 15", scoreOld)
	}
}

func TestRuleLegalFactor_FamilyCoverage(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// makeTestPatent has 5 family members → score 75
	pat := makeTestPatent("PFC", "Family", "granted", 10, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionLegalValue, "family_coverage", assessCtx)
	if score != 75 {
		t.Errorf("family_coverage(5 members) = %.2f, want 75", score)
	}

	// No family
	patNoFam := makeTestPatent("PFC2", "No Family", "granted", 10, 3, 2)
	patNoFam.FamilyMembers = nil
	scoreNoFam := svc.ruleBasedFactorScore(ctx, patNoFam, DimensionLegalValue, "family_coverage", assessCtx)
	if scoreNoFam != 20 {
		t.Errorf("family_coverage(0 members) = %.2f, want 20", scoreNoFam)
	}
}

func TestRuleCommercialFactor_MarketRelevance(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()

	// With matching focus areas
	assessCtx := &AssessmentContext{
		MaxPatentLifeYrs: 20,
		TechFocusAreas:   []string{"distributed", "cache", "performance"},
	}
	pat := makeTestPatent("PMR", "Distributed Cache System", "granted", 10, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionCommercialValue, "market_relevance", assessCtx)
	if score < 60 {
		t.Errorf("market_relevance with matching areas = %.2f, expected >= 60", score)
	}

	// Without focus areas → neutral 60
	assessCtxEmpty := &AssessmentContext{MaxPatentLifeYrs: 20}
	scoreEmpty := svc.ruleBasedFactorScore(ctx, pat, DimensionCommercialValue, "market_relevance", assessCtxEmpty)
	if scoreEmpty != 60 {
		t.Errorf("market_relevance without focus areas = %.2f, want 60", scoreEmpty)
	}
}

func TestRuleStrategicFactor_TechTrajectory(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// Recent patent (0.5 years ago) → high trajectory alignment
	patRecent := makeTestPatent("PTT1", "Recent Tech", "granted", 10, 3, 0.5)
	scoreRecent := svc.ruleBasedFactorScore(ctx, patRecent, DimensionStrategicValue, "technology_trajectory_alignment", assessCtx)
	if scoreRecent != 90 {
		t.Errorf("tech_trajectory(0.5yr) = %.2f, want 90", scoreRecent)
	}

	// 3 years old → 75
	pat3yr := makeTestPatent("PTT2", "3yr Tech", "granted", 10, 3, 3)
	score3yr := svc.ruleBasedFactorScore(ctx, pat3yr, DimensionStrategicValue, "technology_trajectory_alignment", assessCtx)
	if score3yr != 75 {
		t.Errorf("tech_trajectory(3yr) = %.2f, want 75", score3yr)
	}

	// 7 years old → 55
	pat7yr := makeTestPatent("PTT3", "7yr Tech", "granted", 10, 3, 7)
	score7yr := svc.ruleBasedFactorScore(ctx, pat7yr, DimensionStrategicValue, "technology_trajectory_alignment", assessCtx)
	if score7yr != 55 {
		t.Errorf("tech_trajectory(7yr) = %.2f, want 55", score7yr)
	}

	// 15 years old → 35
	pat15yr := makeTestPatent("PTT4", "15yr Tech", "granted", 10, 3, 15)
	score15yr := svc.ruleBasedFactorScore(ctx, pat15yr, DimensionStrategicValue, "technology_trajectory_alignment", assessCtx)
	if score15yr != 35 {
		t.Errorf("tech_trajectory(15yr) = %.2f, want 35", score15yr)
	}
}

func TestRuleStrategicFactor_BlockingPower(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// 3 independent claims + 4 IPC classes → 40 + 30 + 20 = 90
	pat := makeTestPatent("PBP", "Blocking", "granted", 15, 4, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionStrategicValue, "blocking_power", assessCtx)
	if score < 70 {
		t.Errorf("blocking_power(3 indep, 4 ipc) = %.2f, expected >= 70", score)
	}

	// Minimal patent: 1 claim (independent), 1 IPC → 40 + 10 + 5 = 55
	patMin := makeTestPatent("PBP2", "Minimal", "granted", 1, 1, 2)
	scoreMin := svc.ruleBasedFactorScore(ctx, patMin, DimensionStrategicValue, "blocking_power", assessCtx)
	if scoreMin < 50 || scoreMin > 60 {
		t.Errorf("blocking_power(1 indep, 1 ipc) = %.2f, expected ~55", scoreMin)
	}
}

func TestRuleStrategicFactor_NegotiationLeverage(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// Granted + 5 family members + 20 claims → 30 + 25 + 25 + 15 = 95
	pat := makeTestPatent("PNL", "Leverage", "granted", 20, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionStrategicValue, "negotiation_leverage", assessCtx)
	if score < 80 {
		t.Errorf("negotiation_leverage(granted, 5 fam, 20 claims) = %.2f, expected >= 80", score)
	}

	// Pending + no family + few claims → low leverage
	patWeak := makeTestPatent("PNL2", "Weak Leverage", "pending", 3, 1, 2)
	patWeak.FamilyMembers = nil
	scoreWeak := svc.ruleBasedFactorScore(ctx, patWeak, DimensionStrategicValue, "negotiation_leverage", assessCtx)
	if scoreWeak > 40 {
		t.Errorf("negotiation_leverage(pending, 0 fam, 3 claims) = %.2f, expected <= 40", scoreWeak)
	}
}

func TestRuleStrategicFactor_CompetitiveDifferentiation(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()

	// With competitors mentioned in patent text
	assessCtx := &AssessmentContext{
		MaxPatentLifeYrs: 20,
		Competitors:      []string{"distributed", "latency"},
	}
	pat := makeTestPatent("PCD", "Competitive", "granted", 10, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionStrategicValue, "competitive_differentiation", assessCtx)
	if score < 70 {
		t.Errorf("competitive_diff with matching competitors = %.2f, expected >= 70", score)
	}

	// Without competitors → neutral 60
	assessCtxNone := &AssessmentContext{MaxPatentLifeYrs: 20}
	scoreNone := svc.ruleBasedFactorScore(ctx, pat, DimensionStrategicValue, "competitive_differentiation", assessCtxNone)
	if scoreNone != 60 {
		t.Errorf("competitive_diff without competitors = %.2f, want 60", scoreNone)
	}
}

func TestRuleCommercialFactor_ProductCoverage(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	tests := []struct {
		claimCount int
		minScore   float64
		maxScore   float64
	}{
		{2, 35, 35},
		{5, 55, 55},
		{10, 70, 70},
		{20, 85, 85},
		{30, 85, 85},
	}

	for _, tt := range tests {
		pat := makeTestPatent("PPC", "Product", "granted", tt.claimCount, 3, 2)
		score := svc.ruleBasedFactorScore(ctx, pat, DimensionCommercialValue, "product_coverage", assessCtx)
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("product_coverage(claims=%d) = %.2f, want [%.0f, %.0f]", tt.claimCount, score, tt.minScore, tt.maxScore)
		}
	}
}

func TestRuleCommercialFactor_LicensingPotential(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// Granted + many claims + large family → high licensing potential
	pat := makeTestPatent("PLP", "Licensing", "granted", 15, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionCommercialValue, "licensing_potential", assessCtx)
	// 50 + 20 (granted) + 15 (>10 claims) + 10 (>3 family) = 95
	if score < 85 {
		t.Errorf("licensing_potential(granted, 15 claims, 5 fam) = %.2f, expected >= 85", score)
	}

	// Pending + few claims + no family → low
	patLow := makeTestPatent("PLP2", "Low License", "pending", 3, 1, 2)
	patLow.FamilyMembers = nil
	scoreLow := svc.ruleBasedFactorScore(ctx, patLow, DimensionCommercialValue, "licensing_potential", assessCtx)
	if scoreLow > 55 {
		t.Errorf("licensing_potential(pending, 3 claims, 0 fam) = %.2f, expected <= 55", scoreLow)
	}
}

func TestRuleCommercialFactor_CostOfDesignAround(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	// Many IPCs + many claims → hard to design around
	pat := makeTestPatent("PCDA", "Design Around", "granted", 20, 5, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionCommercialValue, "cost_of_design_around", assessCtx)
	// 40 + 5*8 + 20*1.5 = 40 + 40 + 30 = 110 → clamped to 100
	if score != 100 {
		t.Errorf("cost_of_design_around(5 ipc, 20 claims) = %.2f, want 100 (clamped)", score)
	}

	// Minimal patent
	patMin := makeTestPatent("PCDA2", "Easy Around", "granted", 2, 1, 2)
	scoreMin := svc.ruleBasedFactorScore(ctx, patMin, DimensionCommercialValue, "cost_of_design_around", assessCtx)
	// 40 + 1*8 + 2*1.5 = 40 + 8 + 3 = 51
	if scoreMin < 45 || scoreMin > 55 {
		t.Errorf("cost_of_design_around(1 ipc, 2 claims) = %.2f, expected ~51", scoreMin)
	}
}

func TestRuleBasedFactorScore_UnknownDimension(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	pat := makeTestPatent("PU", "Unknown", "granted", 10, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, AssessmentDimension("unknown_dim"), "any_factor", assessCtx)
	if score != 50 {
		t.Errorf("unknown dimension score = %.2f, want 50 (neutral)", score)
	}
}

func TestRuleBasedFactorScore_UnknownFactor(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)
	ctx := context.Background()
	assessCtx := &AssessmentContext{MaxPatentLifeYrs: 20}

	pat := makeTestPatent("PUF", "Unknown Factor", "granted", 10, 3, 2)
	score := svc.ruleBasedFactorScore(ctx, pat, DimensionTechnicalValue, "nonexistent_factor", assessCtx)
	if score != 50 {
		t.Errorf("unknown factor score = %.2f, want 50 (neutral)", score)
	}
}

// ---------------------------------------------------------------------------
// Tests: Citation Impact
// ---------------------------------------------------------------------------

func TestComputeCitationImpact_WithRepo(t *testing.T) {
	citationRepo := newMockCitationRepo()
	citationRepo.forwardCounts["P_CIT"] = 40
	citationRepo.maxInDomain["G06F17/00"] = 100

	svc := buildTestService(nil, nil, nil, nil, citationRepo, nil).(*valuationServiceImpl)

	pat := makeTestPatent("P_CIT", "Citation Test", "granted", 10, 3, 2)
	score := svc.computeCitationImpact(context.Background(), pat)
	// 40/100 * 100 = 40
	if score != 40 {
		t.Errorf("citation impact = %.2f, want 40", score)
	}
}

func TestComputeCitationImpact_NilRepo(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	pat := makeTestPatent("P_NOCIT", "No Citation Repo", "granted", 10, 3, 2)
	score := svc.computeCitationImpact(context.Background(), pat)
	if score != 50 {
		t.Errorf("citation impact without repo = %.2f, want 50 (neutral)", score)
	}
}

func TestComputeCitationImpact_RepoError(t *testing.T) {
	citationRepo := newMockCitationRepo()
	citationRepo.err = fmt.Errorf("citation service down")

	svc := buildTestService(nil, nil, nil, nil, citationRepo, nil).(*valuationServiceImpl)

	pat := makeTestPatent("P_CITERR", "Citation Error", "granted", 10, 3, 2)
	score := svc.computeCitationImpact(context.Background(), pat)
	if score != 50 {
		t.Errorf("citation impact on error = %.2f, want 50 (neutral)", score)
	}
}

func TestComputeCitationImpact_AbsoluteThresholds(t *testing.T) {
	citationRepo := newMockCitationRepo()
	// maxInDomain returns 0 → use absolute thresholds
	citationRepo.maxInDomain["G06F17/00"] = 0

	svc := buildTestService(nil, nil, nil, nil, citationRepo, nil).(*valuationServiceImpl)

	tests := []struct {
		fwdCount int
		minScore float64
		maxScore float64
	}{
		{0, 30, 30},
		{3, 30, 30},
		{5, 55, 55},
		{10, 55, 55},
		{20, 75, 75},
		{30, 75, 75},
		{50, 95, 95},
		{100, 95, 95},
	}

	for _, tt := range tests {
		patID := fmt.Sprintf("P_ABS_%d", tt.fwdCount)
		citationRepo.forwardCounts[patID] = tt.fwdCount
		pat := makeTestPatent(patID, "Absolute", "granted", 10, 3, 2)
		score := svc.computeCitationImpact(context.Background(), pat)
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("citation_impact(fwd=%d, max=0) = %.2f, want [%.0f, %.0f]", tt.fwdCount, score, tt.minScore, tt.maxScore)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Overall Valuation Computation
// ---------------------------------------------------------------------------

func TestComputeOverallValuation(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue:  {Score: 80, MaxScore: 100},
		DimensionLegalValue:      {Score: 70, MaxScore: 100},
		DimensionCommercialValue: {Score: 90, MaxScore: 100},
		DimensionStrategicValue:  {Score: 85, MaxScore: 100},
	}

	overall := svc.computeOverallValuation(scores)

	if overall == nil {
		t.Fatal("overall valuation is nil")
	}
	if overall.Score < 0 || overall.Score > 100 {
		t.Errorf("overall score = %.2f, out of range", overall.Score)
	}
	if overall.Tier == "" {
		t.Error("tier is empty")
	}
	if overall.TierDescription == "" {
		t.Error("tier description is empty")
	}
	if overall.WeightedCalculation == "" {
		t.Error("weighted calculation is empty")
	}

	// Verify weighted calculation: 80*0.30 + 70*0.25 + 90*0.25 + 85*0.20
	// = 24 + 17.5 + 22.5 + 17 = 81
	expectedScore := 80*0.30 + 70*0.25 + 90*0.25 + 85*0.20
	if overall.Score < expectedScore-0.5 || overall.Score > expectedScore+0.5 {
		t.Errorf("overall score = %.2f, expected ~%.2f", overall.Score, expectedScore)
	}
}

func TestComputeOverallValuation_PartialDimensions(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	// Only 2 dimensions
	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue: {Score: 80, MaxScore: 100},
		DimensionLegalValue:     {Score: 60, MaxScore: 100},
	}

	overall := svc.computeOverallValuation(scores)
	if overall == nil {
		t.Fatal("overall valuation is nil")
	}
	// Should normalize: (80*0.30 + 60*0.25) / (0.30 + 0.25) = (24 + 15) / 0.55 ≈ 70.91
	if overall.Score < 65 || overall.Score > 75 {
		t.Errorf("partial overall score = %.2f, expected ~70.91", overall.Score)
	}
}

func TestComputeOverallValuation_EmptyScores(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	scores := map[AssessmentDimension]*DimensionScore{}
	overall := svc.computeOverallValuation(scores)
	if overall == nil {
		t.Fatal("overall valuation is nil")
	}
	if overall.Score != 0 {
		t.Errorf("empty scores overall = %.2f, want 0", overall.Score)
	}
}

// ---------------------------------------------------------------------------
// Tests: Fallback Dimension Score
// ---------------------------------------------------------------------------

func TestFallbackDimensionScore(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	for _, dim := range AllDimensions() {
		ds := svc.fallbackDimensionScore(dim)
		if ds.Score != 50 {
			t.Errorf("fallback %s score = %.2f, want 50", dim, ds.Score)
		}
		if ds.MaxScore != 100 {
			t.Errorf("fallback %s MaxScore = %.0f, want 100", dim, ds.MaxScore)
		}
		if len(ds.Factors) == 0 {
			t.Errorf("fallback %s has no factors", dim)
		}
		for _, fs := range ds.Factors {
			if fs.Score != 50 {
				t.Errorf("fallback factor %s score = %.2f, want 50", fs.Name, fs.Score)
			}
		}
	}
}

func TestFallbackDimensionScore_UnknownDimension(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	ds := svc.fallbackDimensionScore(AssessmentDimension("nonexistent"))
	if ds.Score != 50 {
		t.Errorf("fallback unknown dim score = %.2f, want 50", ds.Score)
	}
	if len(ds.Factors) != 0 {
		t.Errorf("fallback unknown dim should have 0 factors, got %d", len(ds.Factors))
	}
}

// ---------------------------------------------------------------------------
// Tests: Recommendation Generation
// ---------------------------------------------------------------------------

func TestGenerateRecommendations_TierC(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	overall := &OverallValuation{Score: 55, Tier: TierC}
	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue:  {Score: 55},
		DimensionLegalValue:      {Score: 50},
		DimensionCommercialValue: {Score: 60},
		DimensionStrategicValue:  {Score: 55},
	}

	recs := svc.generateRecommendations(nil, overall, scores)
	if len(recs) == 0 {
		t.Fatal("expected recommendations for Tier C")
	}

	foundLicense := false
	foundAbandon := false
	for _, r := range recs {
		if r.Type == ActionLicense {
			foundLicense = true
		}
		if r.Type == ActionAbandon {
			foundAbandon = true
		}
	}
	if !foundLicense {
		t.Error("expected license recommendation for Tier C")
	}
	if !foundAbandon {
		t.Error("expected abandon/scope-reduction recommendation for Tier C")
	}
}

func TestGenerateRecommendationsFromRecord(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	record := &AssessmentRecord{
		ID:           "REC_GEN",
		PatentID:     "P_GEN",
		OverallScore: 92,
		Tier:         TierS,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  95,
			DimensionLegalValue:      90,
			DimensionCommercialValue: 88,
			DimensionStrategicValue:  94,
		},
	}

	recs := svc.generateRecommendationsFromRecord(record)
	if len(recs) == 0 {
		t.Fatal("expected recommendations from record")
	}

	foundMaintain := false
	for _, r := range recs {
		if r.Type == ActionMaintain && r.Priority == PriorityCritical {
			foundMaintain = true
		}
	}
	if !foundMaintain {
		t.Error("expected critical maintain recommendation for Tier S record")
	}
}

// ---------------------------------------------------------------------------
// Tests: Portfolio Summary
// ---------------------------------------------------------------------------

func TestBuildPortfolioSummary(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	assessments := []*SinglePatentAssessmentResponse{
		{PatentID: "P1", OverallScore: &OverallValuation{Score: 95, Tier: TierS}},
		{PatentID: "P2", OverallScore: &OverallValuation{Score: 82, Tier: TierA}},
		{PatentID: "P3", OverallScore: &OverallValuation{Score: 70, Tier: TierB}},
		{PatentID: "P4", OverallScore: &OverallValuation{Score: 55, Tier: TierC}},
		{PatentID: "P5", OverallScore: &OverallValuation{Score: 30, Tier: TierD}},
	}

	assessCtx := &AssessmentContext{
		CurrencyCode:   "CNY",
		TechFocusAreas: []string{"AI", "blockchain", "IoT"},
	}

	summary := svc.buildPortfolioSummary(assessments, assessCtx)

	if summary.TotalAssessed != 5 {
		t.Errorf("TotalAssessed = %d, want 5", summary.TotalAssessed)
	}
	if summary.Currency != "CNY" {
		t.Errorf("Currency = %s, want CNY", summary.Currency)
	}

	expectedAvg := (95.0 + 82 + 70 + 55 + 30) / 5
	if summary.AverageScore < expectedAvg-0.5 || summary.AverageScore > expectedAvg+0.5 {
		t.Errorf("AverageScore = %.2f, expected ~%.2f", summary.AverageScore, expectedAvg)
	}

	if summary.TierDistribution[TierS] != 1 {
		t.Errorf("TierS = %d, want 1", summary.TierDistribution[TierS])
	}
	if summary.TierDistribution[TierD] != 1 {
		t.Errorf("TierD = %d, want 1", summary.TierDistribution[TierD])
	}

	if summary.TotalMaintenanceCost <= 0 {
		t.Error("TotalMaintenanceCost should be positive")
	}

	// 2 high-tier patents (S+A), 3 focus areas → 1 gap
	if summary.StrategicGapsIdentified != 1 {
		t.Errorf("StrategicGapsIdentified = %d, want 1", summary.StrategicGapsIdentified)
	}
	if summary.RecommendedNewFilings != 1 {
		t.Errorf("RecommendedNewFilings = %d, want 1", summary.RecommendedNewFilings)
	}
}

func TestBuildPortfolioSummary_Empty(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	summary := svc.buildPortfolioSummary(nil, &AssessmentContext{CurrencyCode: "USD"})
	if summary.TotalAssessed != 0 {
		t.Errorf("TotalAssessed = %d, want 0", summary.TotalAssessed)
	}
	if summary.AverageScore != 0 {
		t.Errorf("AverageScore = %.2f, want 0", summary.AverageScore)
	}
}

func TestBuildPortfolioSummary_NoGaps(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	assessments := []*SinglePatentAssessmentResponse{
		{PatentID: "P1", OverallScore: &OverallValuation{Score: 95, Tier: TierS}},
		{PatentID: "P2", OverallScore: &OverallValuation{Score: 85, Tier: TierA}},
		{PatentID: "P3", OverallScore: &OverallValuation{Score: 92, Tier: TierS}},
	}

	assessCtx := &AssessmentContext{
		CurrencyCode:   "USD",
		TechFocusAreas: []string{"AI", "ML"},
	}

	summary := svc.buildPortfolioSummary(assessments, assessCtx)
	// 3 high-tier patents >= 2 focus areas → 0 gaps
	if summary.StrategicGapsIdentified != 0 {
		t.Errorf("StrategicGapsIdentified = %d, want 0 (no gaps)", summary.StrategicGapsIdentified)
	}
}

// ---------------------------------------------------------------------------
// Tests: Cost Optimization Analysis
// ---------------------------------------------------------------------------

func TestAnalyzeCostOptimization(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	assessments := []*SinglePatentAssessmentResponse{
		{PatentID: "P_S", OverallScore: &OverallValuation{Score: 95, Tier: TierS}},
		{PatentID: "P_A", OverallScore: &OverallValuation{Score: 85, Tier: TierA}},
		{PatentID: "P_B", OverallScore: &OverallValuation{Score: 70, Tier: TierB}},
		{PatentID: "P_C", OverallScore: &OverallValuation{Score: 55, Tier: TierC}},
		{PatentID: "P_D", OverallScore: &OverallValuation{Score: 30, Tier: TierD}},
	}

	assessCtx := &AssessmentContext{CurrencyCode: "CNY"}

	result := svc.analyzeCostOptimization(assessments, assessCtx)

	if result.Currency != "CNY" {
		t.Errorf("Currency = %s, want CNY", result.Currency)
	}
	if result.CurrentAnnualCost <= 0 {
		t.Error("CurrentAnnualCost should be positive")
	}
	if result.OptimizedAnnualCost > result.CurrentAnnualCost {
		t.Error("OptimizedAnnualCost should not exceed CurrentAnnualCost")
	}
	if result.SavingsPotential < 0 {
		t.Error("SavingsPotential should be >= 0")
	}
	if result.SavingsPotential != result.CurrentAnnualCost-result.OptimizedAnnualCost {
		t.Error("SavingsPotential should equal CurrentAnnualCost - OptimizedAnnualCost")
	}
	if len(result.Recommendations) != 5 {
		t.Errorf("expected 5 cost recommendations, got %d", len(result.Recommendations))
	}

	// Verify tier-specific actions
	actionMap := make(map[string]CostAction)
	for _, rec := range result.Recommendations {
		actionMap[rec.PatentID] = rec.Action
	}

	if actionMap["P_S"] != CostContinueMaintain {
		t.Errorf("P_S action = %s, want continue_maintain", actionMap["P_S"])
	}
	if actionMap["P_A"] != CostContinueMaintain {
		t.Errorf("P_A action = %s, want continue_maintain", actionMap["P_A"])
	}
	if actionMap["P_B"] != CostContinueMaintain {
		t.Errorf("P_B action = %s, want continue_maintain", actionMap["P_B"])
	}
	if actionMap["P_C"] != CostReduceScope {
		t.Errorf("P_C action = %s, want reduce_scope", actionMap["P_C"])
	}
	if actionMap["P_D"] != CostAbandon {
		t.Errorf("P_D action = %s, want abandon", actionMap["P_D"])
	}

	// Verify savings come from C (40%) and D (100%)
	var cSaving, dSaving float64
	for _, rec := range result.Recommendations {
		if rec.PatentID == "P_C" {
			cSaving = rec.EstimatedSaving
		}
		if rec.PatentID == "P_D" {
			dSaving = rec.EstimatedSaving
		}
	}
	if cSaving <= 0 {
		t.Error("P_C should have positive estimated saving")
	}
	if dSaving <= 0 {
		t.Error("P_D should have positive estimated saving")
	}
	if result.SavingsPotential != cSaving+dSaving {
		t.Errorf("SavingsPotential = %.2f, want %.2f (cSaving + dSaving)", result.SavingsPotential, cSaving+dSaving)
	}
}

func TestAnalyzeCostOptimization_Empty(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	result := svc.analyzeCostOptimization(nil, &AssessmentContext{CurrencyCode: "USD"})
	if result.CurrentAnnualCost != 0 {
		t.Errorf("empty cost = %.2f, want 0", result.CurrentAnnualCost)
	}
	if result.SavingsPotential != 0 {
		t.Errorf("empty savings = %.2f, want 0", result.SavingsPotential)
	}
	if len(result.Recommendations) != 0 {
		t.Errorf("expected 0 recommendations, got %d", len(result.Recommendations))
	}
}

func TestAnalyzeCostOptimization_AllHighTier(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	assessments := []*SinglePatentAssessmentResponse{
		{PatentID: "P1", OverallScore: &OverallValuation{Score: 95, Tier: TierS}},
		{PatentID: "P2", OverallScore: &OverallValuation{Score: 92, Tier: TierS}},
	}

	result := svc.analyzeCostOptimization(assessments, &AssessmentContext{CurrencyCode: "CNY"})
	// All high tier → no savings
	if result.SavingsPotential != 0 {
		t.Errorf("all high-tier savings = %.2f, want 0", result.SavingsPotential)
	}
	if result.OptimizedAnnualCost != result.CurrentAnnualCost {
		t.Error("optimized should equal current when all high-tier")
	}
}

func TestAnalyzeCostOptimization_AllLowTier(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	assessments := []*SinglePatentAssessmentResponse{
		{PatentID: "PD1", OverallScore: &OverallValuation{Score: 20, Tier: TierD}},
		{PatentID: "PD2", OverallScore: &OverallValuation{Score: 15, Tier: TierD}},
	}

	result := svc.analyzeCostOptimization(assessments, &AssessmentContext{CurrencyCode: "CNY"})
	// All D tier → abandon all → optimized = 0
	if result.OptimizedAnnualCost != 0 {
		t.Errorf("all D-tier optimized cost = %.2f, want 0", result.OptimizedAnnualCost)
	}
	if result.SavingsPotential != result.CurrentAnnualCost {
		t.Error("all D-tier savings should equal current cost")
	}
}

// ---------------------------------------------------------------------------
// Tests: ExportAssessment
// ---------------------------------------------------------------------------

func TestExportAssessment_JSON_V2(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID:           "EXP_JSON",
		PatentID:     "P_EXP",
		OverallScore: 75,
		Tier:         TierB,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue: 70,
			DimensionLegalValue:     80,
		},
		AssessedAt:   time.Now(),
		AssessorType: AssessorHybrid,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	data, err := svc.ExportAssessment(context.Background(), "EXP_JSON", ExportJSON)
	if err != nil {
		t.Fatalf("ExportAssessment JSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported JSON is empty")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if jsonErr := json.Unmarshal(data, &parsed); jsonErr != nil {
		t.Fatalf("exported data is not valid JSON: %v", jsonErr)
	}
	if parsed["id"] != "EXP_JSON" {
		t.Errorf("exported id = %v, want EXP_JSON", parsed["id"])
	}
	if parsed["patent_id"] != "P_EXP" {
		t.Errorf("exported patent_id = %v, want P_EXP", parsed["patent_id"])
	}
}

func TestExportAssessment_CSV_V2(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID:           "EXP_CSV",
		PatentID:     "P_CSV",
		OverallScore: 68,
		Tier:         TierB,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  65,
			DimensionLegalValue:      70,
			DimensionCommercialValue: 72,
			DimensionStrategicValue:  66,
		},
		AssessedAt:   time.Now(),
		AssessorType: AssessorAI,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	data, err := svc.ExportAssessment(context.Background(), "EXP_CSV", ExportCSV)
	if err != nil {
		t.Fatalf("ExportAssessment CSV failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported CSV is empty")
	}

	csvStr := string(data)
	// Should contain header and at least one data row
	if !contains(csvStr, "id") || !contains(csvStr, "patent_id") {
		t.Error("CSV missing expected headers")
	}
	if !contains(csvStr, "EXP_CSV") {
		t.Error("CSV missing assessment ID")
	}
	if !contains(csvStr, "P_CSV") {
		t.Error("CSV missing patent ID")
	}
}

func TestExportAssessment_NotFound_V2(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	_, err := svc.ExportAssessment(context.Background(), "NONEXISTENT", ExportJSON)
	if err == nil {
		t.Fatal("expected error for non-existent assessment")
	}
}

func TestExportAssessment_EmptyID_V2(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.ExportAssessment(context.Background(), "", ExportJSON)
	if err == nil {
		t.Fatal("expected validation error for empty ID")
	}
}

func TestExportAssessment_UnsupportedFormat_V2(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID:           "EXP_UNK",
		PatentID:     "P_UNK",
		OverallScore: 70,
		Tier:         TierB,
		AssessedAt:   time.Now(),
		AssessorType: AssessorHuman,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	_, err := svc.ExportAssessment(context.Background(), "EXP_UNK", ExportFormat("xml"))
	if err == nil {
		t.Fatal("expected error for unsupported export format")
	}
}

// ---------------------------------------------------------------------------
// Tests: RecommendActions
// ---------------------------------------------------------------------------

func TestRecommendActions_Success(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID:           "RA_001",
		PatentID:     "P_RA",
		OverallScore: 45,
		Tier:         TierD,
		DimensionScores: map[AssessmentDimension]float64{
			DimensionTechnicalValue:  40,
			DimensionLegalValue:      50,
			DimensionCommercialValue: 42,
			DimensionStrategicValue:  48,
		},
		AssessedAt:   time.Now(),
		AssessorType: AssessorHybrid,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	recs, err := svc.RecommendActions(context.Background(), "RA_001")
	if err != nil {
		t.Fatalf("RecommendActions failed: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("expected recommendations for D-tier patent")
	}

	// D-tier should have abandon recommendation
	foundAbandon := false
	for _, r := range recs {
		if r.Type == ActionAbandon {
			foundAbandon = true
		}
	}
	if !foundAbandon {
		t.Error("expected abandon recommendation for D-tier")
	}
}

func TestRecommendActions_EmptyID_V2(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.RecommendActions(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty assessment ID")
	}
}

func TestRecommendActions_NotFound_V2(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	svc := buildTestService(nil, nil, assessmentRepo, nil, nil, nil)

	_, err := svc.RecommendActions(context.Background(), "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent assessment")
	}
}

// ---------------------------------------------------------------------------
// Tests: Recommendation Generation for All Tiers
// ---------------------------------------------------------------------------

func TestGenerateRecommendations_TierS(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	overall := &OverallValuation{Score: 95, Tier: TierS}
	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue:  {Score: 95},
		DimensionLegalValue:      {Score: 92},
		DimensionCommercialValue: {Score: 96},
		DimensionStrategicValue:  {Score: 94},
	}

	recs := svc.generateRecommendations(nil, overall, scores)
	if len(recs) < 2 {
		t.Fatalf("expected at least 2 recommendations for Tier S, got %d", len(recs))
	}

	foundMaintain := false
	foundEnforce := false
	for _, r := range recs {
		if r.Type == ActionMaintain && r.Priority == PriorityCritical {
			foundMaintain = true
		}
		if r.Type == ActionEnforce && r.Priority == PriorityHigh {
			foundEnforce = true
		}
	}
	if !foundMaintain {
		t.Error("Tier S: expected critical maintain recommendation")
	}
	if !foundEnforce {
		t.Error("Tier S: expected high-priority enforce recommendation")
	}
}

func TestGenerateRecommendations_TierA_WithWeakDimension(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	overall := &OverallValuation{Score: 82, Tier: TierA}
	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue:  {Score: 90},
		DimensionLegalValue:      {Score: 88},
		DimensionCommercialValue: {Score: 50}, // weak
		DimensionStrategicValue:  {Score: 85},
	}

	recs := svc.generateRecommendations(nil, overall, scores)

	foundStrengthen := false
	for _, r := range recs {
		if r.Type == ActionStrengthen {
			foundStrengthen = true
		}
	}
	if !foundStrengthen {
		t.Error("Tier A with weak dimension: expected strengthen recommendation")
	}
}

func TestGenerateRecommendations_TierB_HighCommercial(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	overall := &OverallValuation{Score: 72, Tier: TierB}
	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue:  {Score: 65},
		DimensionLegalValue:      {Score: 68},
		DimensionCommercialValue: {Score: 85}, // high commercial
		DimensionStrategicValue:  {Score: 70},
	}

	recs := svc.generateRecommendations(nil, overall, scores)

	foundLicense := false
	for _, r := range recs {
		if r.Type == ActionLicense {
			foundLicense = true
		}
	}
	if !foundLicense {
		t.Error("Tier B with high commercial: expected license recommendation")
	}
}

func TestGenerateRecommendations_TierD(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	overall := &OverallValuation{Score: 30, Tier: TierD}
	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue:  {Score: 25},
		DimensionLegalValue:      {Score: 35},
		DimensionCommercialValue: {Score: 28},
		DimensionStrategicValue:  {Score: 32},
	}

	recs := svc.generateRecommendations(nil, overall, scores)

	foundAbandon := false
	foundLicense := false
	for _, r := range recs {
		if r.Type == ActionAbandon && r.Priority == PriorityHigh {
			foundAbandon = true
		}
		if r.Type == ActionLicense {
			foundLicense = true
		}
	}
	if !foundAbandon {
		t.Error("Tier D: expected high-priority abandon recommendation")
	}
	if !foundLicense {
		t.Error("Tier D: expected license-before-abandon recommendation")
	}
}

func TestGenerateRecommendations_CrossDimensional_FileNew(t *testing.T) {
	svc := buildTestService(nil, nil, nil, nil, nil, nil).(*valuationServiceImpl)

	overall := &OverallValuation{Score: 65, Tier: TierB}
	scores := map[AssessmentDimension]*DimensionScore{
		DimensionTechnicalValue:  {Score: 70},
		DimensionLegalValue:      {Score: 65},
		DimensionCommercialValue: {Score: 40}, // low commercial
		DimensionStrategicValue:  {Score: 80}, // high strategic
	}

	recs := svc.generateRecommendations(nil, overall, scores)

	foundFileNew := false
	for _, r := range recs {
		if r.Type == ActionFileNew {
			foundFileNew = true
		}
	}
	if !foundFileNew {
		t.Error("high strategic + low commercial: expected file_new recommendation")
	}
}

// ---------------------------------------------------------------------------
// Tests: Context Cancellation
// ---------------------------------------------------------------------------

func TestAssessPatent_ContextCancelled(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P_CTX", "Context Cancel", "granted", 10, 3, 2)
	patentRepo.patents["P_CTX"] = pat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := &SinglePatentAssessmentRequest{PatentID: "P_CTX"}
	_, err := svc.AssessPatent(ctx, req)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestAssessPortfolio_ContextCancelled(t *testing.T) {
	patentRepo := newMockPatentRepo()
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("PCTX%d", i)
		patentRepo.patents[id] = makeTestPatent(id, "Ctx Patent", "granted", 10, 3, 2)
	}

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &PortfolioAssessmentRequest{
		PortfolioID: "PF_CTX",
		PatentIDs:   []string{"PCTX0", "PCTX1", "PCTX2", "PCTX3", "PCTX4"},
	}
	_, err := svc.AssessPortfolio(ctx, req)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Tests: Edge Cases
// ---------------------------------------------------------------------------

func TestAssessPatent_ZeroClaims(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P_ZERO", "Zero Claims", "granted", 0, 0, 5)
	pat.Claims = nil
	pat.IPCClassifications = nil
	pat.FamilyMembers = nil
	pat.Abstract = ""
	pat.Description = ""
	patentRepo.patents["P_ZERO"] = pat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P_ZERO"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error for zero-claims patent: %v", err)
	}
	if resp.OverallScore == nil {
		t.Fatal("OverallScore should not be nil")
	}
	// Should still produce valid scores (low but valid)
	if resp.OverallScore.Score < 0 || resp.OverallScore.Score > 100 {
		t.Errorf("score out of range: %.2f", resp.OverallScore.Score)
	}
}

func TestAssessPatent_ZeroFilingDate(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P_NODATE", "No Date", "granted", 10, 3, 0)
	pat.FilingDate = time.Time{} // zero value
	patentRepo.patents["P_NODATE"] = pat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P_NODATE"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OverallScore.Score < 0 || resp.OverallScore.Score > 100 {
		t.Errorf("score out of range: %.2f", resp.OverallScore.Score)
	}
}

func TestAssessPatent_ExpiredPatent(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P_EXP", "Expired Patent", "expired", 10, 3, 25) // 25 years ago
	patentRepo.patents["P_EXP"] = pat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P_EXP"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expired patent should score low on remaining_life_years
	if legalScore, ok := resp.Scores[DimensionLegalValue]; ok {
		if rlFactor, ok2 := legalScore.Factors["remaining_life_years"]; ok2 {
			if rlFactor.Score > 10 {
				t.Errorf("remaining_life_years for expired patent = %.2f, expected <= 10", rlFactor.Score)
			}
		}
	}
}

func TestAssessPatent_RejectedPatent(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := makeTestPatent("P_REJ", "Rejected Patent", "rejected", 10, 3, 3)
	patentRepo.patents["P_REJ"] = pat

	cache := newMockCache()
	cache.err = fmt.Errorf("cache miss")

	svc := buildTestService(patentRepo, nil, nil, nil, nil, cache)

	req := &SinglePatentAssessmentRequest{PatentID: "P_REJ"}
	resp, err := svc.AssessPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Rejected patent should have low prosecution_strength
	if legalScore, ok := resp.Scores[DimensionLegalValue]; ok {
		if psFactor, ok2 := legalScore.Factors["prosecution_strength"]; ok2 {
			if psFactor.Score > 20 {
				t.Errorf("prosecution_strength for rejected patent = %.2f, expected <= 20", psFactor.Score)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Error Types
// ---------------------------------------------------------------------------

func TestErrorTypes(t *testing.T) {
	_ = pkgerrors.ErrNotFound
	_ = pkgerrors.ErrValidation
	_ = pkgerrors.ErrInternal
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// WithLimit is a functional option for GetAssessmentHistory.
type HistoryOption func(*historyOptions)

type historyOptions struct {
	limit  int
	offset int
}

func WithHistoryLimit(limit int) HistoryOption {
	return func(o *historyOptions) {
		o.limit = limit
	}
}

func WithHistoryOffset(offset int) HistoryOption {
	return func(o *historyOptions) {
		o.offset = offset
	}
}

//Personal.AI order the ending




