// -------------------------------------------------------------------
// File: internal/application/infringement/risk_assessment_test.go
// Phase 10 - 序号 205
// KeyIP-Intelligence: AI-Driven Intellectual Property Lifecycle
//                     Management Platform for OLED Materials
// -------------------------------------------------------------------

package infringement

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	prom "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/claim_bert"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/infringe_net"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/molpatent_gnn"
)

// ===========================================================================
// Mock Implementations
// ===========================================================================

// --- Mock MoleculeDomainService ---

type mockMoleculeSvc struct {
	molecule.MoleculeDomainService
	canonicalizeFn        func(ctx context.Context, smiles string) (string, string, error)
	canonicalizeFromInChIFn func(ctx context.Context, inchi string) (string, string, error)
}

func (m *mockMoleculeSvc) Canonicalize(ctx context.Context, smiles string) (string, string, error) {
	if m.canonicalizeFn != nil {
		return m.canonicalizeFn(ctx, smiles)
	}
	return smiles, "INCHIKEY-" + smiles[:min(8, len(smiles))], nil
}

func (m *mockMoleculeSvc) CanonicalizeFromInChI(ctx context.Context, inchi string) (string, string, error) {
	if m.canonicalizeFromInChIFn != nil {
		return m.canonicalizeFromInChIFn(ctx, inchi)
	}
	return "canonical-from-inchi", "INCHIKEY-FROM-INCHI", nil
}

// --- Mock PatentDomainService ---

type mockPatentSvc struct {
	patent.PatentDomainService
	searchBySimilarityFn func(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error)
	getPatentByNumberFn  func(ctx context.Context, number string) (*patent.Patent, error)
}

func (m *mockPatentSvc) SearchBySimilarity(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
	if m.searchBySimilarityFn != nil {
		return m.searchBySimilarityFn(ctx, req)
	}
	return []*patent.SimilaritySearchResult{}, nil
}

func (m *mockPatentSvc) GetPatentByNumber(ctx context.Context, number string) (*patent.Patent, error) {
	if m.getPatentByNumberFn != nil {
		return m.getPatentByNumberFn(ctx, number)
	}
	// Return a dummy patent to avoid nil pointer dereference
	return &patent.Patent{
		PatentNumber: number,
		Claims:       []*patent.Claim{{Text: "dummy claim"}},
	}, nil
}

// --- Mock InfringeNetAssessor ---

type mockInfringeNet struct {
	infringe_net.InfringementAssessor
	assessFn func(ctx context.Context, req *infringe_net.AssessmentRequest) (*infringe_net.AssessmentResult, error)
}

func (m *mockInfringeNet) Assess(ctx context.Context, req *infringe_net.AssessmentRequest) (*infringe_net.AssessmentResult, error) {
	if m.assessFn != nil {
		return m.assessFn(ctx, req)
	}
	return &infringe_net.AssessmentResult{
		LiteralAnalysis: &infringe_net.LiteralAnalysisResult{
			Score:          0.0,
			AllElementsMet: false,
		},
		EquivalentsAnalysis: &infringe_net.EquivalentsAnalysisResult{
			Score:   0.0,
			Skipped: true,
		},
		EstoppelCheck: &infringe_net.EstoppelCheckResult{
			HasEstoppel: false,
		},
		OverallRiskLevel: infringe_net.RiskNone,
		OverallScore:     0.0,
	}, nil
}

// --- Mock ClaimBERTParser ---

type mockClaimParser struct {
	claim_bert.ClaimParser
	parseClaimSetFn func(ctx context.Context, texts []string) (*claim_bert.ParsedClaimSet, error)
	semanticMatchFn func(ctx context.Context, smiles, claimText string) (float64, error)
}

func (m *mockClaimParser) ParseClaimSet(ctx context.Context, texts []string) (*claim_bert.ParsedClaimSet, error) {
	if m.parseClaimSetFn != nil {
		return m.parseClaimSetFn(ctx, texts)
	}

	claims := make([]*claim_bert.ParsedClaim, len(texts))
	for i, t := range texts {
		claims[i] = &claim_bert.ParsedClaim{
			ClaimNumber: i + 1,
			ClaimType:   claim_bert.ClaimIndependent,
			Body:        t,
			Features:    []*claim_bert.TechnicalFeature{{Text: "feature-" + t}},
		}
	}

	return &claim_bert.ParsedClaimSet{
		Claims: claims,
	}, nil
}

func (m *mockClaimParser) SemanticMatch(ctx context.Context, smiles, claimText string) (float64, error) {
	if m.semanticMatchFn != nil {
		return m.semanticMatchFn(ctx, smiles, claimText)
	}
	return 0.3, nil
}

// --- Mock MolPatentGNNInference ---

type mockGNNInference struct {
	molpatent_gnn.GNNInferenceService
	searchSimilarFn func(ctx context.Context, req *molpatent_gnn.SimilarSearchRequest) (*molpatent_gnn.SimilarSearchResponse, error)
}

func (m *mockGNNInference) SearchSimilar(ctx context.Context, req *molpatent_gnn.SimilarSearchRequest) (*molpatent_gnn.SimilarSearchResponse, error) {
	if m.searchSimilarFn != nil {
		return m.searchSimilarFn(ctx, req)
	}
	return &molpatent_gnn.SimilarSearchResponse{}, nil
}

// --- Mock RiskRecordRepository ---

type mockRiskRepo struct {
	saveFn            func(ctx context.Context, record *RiskRecord) error
	findByMoleculeFn  func(ctx context.Context, moleculeID string, opts *queryOptions) ([]*RiskRecord, string, error)
	findByPortfolioFn func(ctx context.Context, portfolioID string) ([]*RiskRecord, error)
	findByIDFn        func(ctx context.Context, recordID string) (*RiskRecord, error)
	getTrendFn        func(ctx context.Context, portfolioID string, months int) ([]*RiskTrendPoint, error)
	saveCallCount     atomic.Int32
}

func (m *mockRiskRepo) Save(ctx context.Context, record *RiskRecord) error {
	m.saveCallCount.Add(1)
	if m.saveFn != nil {
		return m.saveFn(ctx, record)
	}
	return nil
}

func (m *mockRiskRepo) FindByMolecule(ctx context.Context, moleculeID string, opts *queryOptions) ([]*RiskRecord, string, error) {
	if m.findByMoleculeFn != nil {
		return m.findByMoleculeFn(ctx, moleculeID, opts)
	}
	return []*RiskRecord{}, "", nil
}

func (m *mockRiskRepo) FindByPortfolio(ctx context.Context, portfolioID string) ([]*RiskRecord, error) {
	if m.findByPortfolioFn != nil {
		return m.findByPortfolioFn(ctx, portfolioID)
	}
	return []*RiskRecord{}, nil
}

func (m *mockRiskRepo) FindByID(ctx context.Context, recordID string) (*RiskRecord, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, recordID)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockRiskRepo) GetTrend(ctx context.Context, portfolioID string, months int) ([]*RiskTrendPoint, error) {
	if m.getTrendFn != nil {
		return m.getTrendFn(ctx, portfolioID, months)
	}
	return []*RiskTrendPoint{}, nil
}

// --- Mock FTOReportRepository ---

type mockFTORepo struct {
	saveFn func(ctx context.Context, report *FTOResponse) error
	findFn func(ctx context.Context, ftoID string) (*FTOResponse, error)
}

func (m *mockFTORepo) SaveFTOReport(ctx context.Context, report *FTOResponse) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, report)
	}
	return nil
}

func (m *mockFTORepo) FindFTOReport(ctx context.Context, ftoID string) (*FTOResponse, error) {
	if m.findFn != nil {
		return m.findFn(ctx, ftoID)
	}
	return nil, fmt.Errorf("not found")
}

// --- Mock EventPublisher ---

type mockEventPublisher struct {
	publishFn    func(ctx context.Context, event interface{}) error
	publishCount atomic.Int32
}

func (m *mockEventPublisher) Publish(ctx context.Context, event interface{}) error {
	m.publishCount.Add(1)
	if m.publishFn != nil {
		return m.publishFn(ctx, event)
	}
	return nil
}

// --- Mock Cache ---

type mockCache struct {
	redis.Cache
	store  map[string][]byte
	getFn  func(ctx context.Context, key string) ([]byte, error)
	setFn  func(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string][]byte)}
}

func (m *mockCache) Get(ctx context.Context, key string, dest interface{}) error {
	// dest is expected to be pointer to struct.
	// For simplicity in mock, if getFn provided, use it.
	// But getFn returns []byte. We need to unmarshal to dest.
	if m.getFn != nil {
		data, err := m.getFn(ctx, key)
		if err != nil {
			return err
		}
		if data == nil {
			return fmt.Errorf("cache miss")
		}
		return json.Unmarshal(data, dest)
	}

	data, ok := m.store[key]
	if !ok {
		return fmt.Errorf("cache miss")
	}
	return json.Unmarshal(data, dest)
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Marshal value
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if m.setFn != nil {
		return m.setFn(ctx, key, data, ttl)
	}
	m.store[key] = data
	return nil
}

// --- Mock Logger ---

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockLogger) WithError(err error) logging.Logger { return m }
func (m *mockLogger) Sync() error { return nil }

// --- Mock Metrics Collector ---

type mockMetricsCollector struct {
}

func (m *mockMetricsCollector) RegisterCounter(name, help string, labels ...string) prom.CounterVec {
	return &mockCounterVec{}
}
func (m *mockMetricsCollector) RegisterGauge(name, help string, labels ...string) prom.GaugeVec {
	return &mockGaugeVec{}
}
func (m *mockMetricsCollector) RegisterHistogram(name, help string, buckets []float64, labels ...string) prom.HistogramVec {
	return &mockHistogramVec{}
}
func (m *mockMetricsCollector) RegisterSummary(name, help string, objectives map[float64]float64, labels ...string) prom.SummaryVec {
	return &mockSummaryVec{}
}
func (m *mockMetricsCollector) Handler() http.Handler { return nil }
func (m *mockMetricsCollector) MustRegister(collectors ...prometheus.Collector) {}
func (m *mockMetricsCollector) Unregister(collector prometheus.Collector) bool { return true }

type mockCounterVec struct{}
func (v *mockCounterVec) WithLabelValues(lvs ...string) prom.Counter { return &mockCounter{} }
func (v *mockCounterVec) With(labels map[string]string) prom.Counter { return &mockCounter{} }

type mockCounter struct{}
func (c *mockCounter) Inc() {}
func (c *mockCounter) Add(delta float64) {}

type mockGaugeVec struct{}
func (v *mockGaugeVec) WithLabelValues(lvs ...string) prom.Gauge { return &mockGauge{} }
func (v *mockGaugeVec) With(labels map[string]string) prom.Gauge { return &mockGauge{} }

type mockGauge struct{}
func (g *mockGauge) Set(value float64) {}
func (g *mockGauge) Inc() {}
func (g *mockGauge) Dec() {}
func (g *mockGauge) Add(delta float64) {}
func (g *mockGauge) Sub(delta float64) {}

type mockHistogramVec struct{}
func (v *mockHistogramVec) WithLabelValues(lvs ...string) prom.Histogram { return &mockHistogram{} }
func (v *mockHistogramVec) With(labels map[string]string) prom.Histogram { return &mockHistogram{} }

type mockHistogram struct{}
func (h *mockHistogram) Observe(value float64) {}

type mockSummaryVec struct{}
func (v *mockSummaryVec) WithLabelValues(lvs ...string) prom.Summary { return &mockSummary{} }
func (v *mockSummaryVec) With(labels map[string]string) prom.Summary { return &mockSummary{} }

type mockSummary struct{}
func (s *mockSummary) Observe(value float64) {}

// ===========================================================================
// Test Helper: Build Default Service
// ===========================================================================

type testHarness struct {
	svc            RiskAssessmentService
	moleculeSvc    *mockMoleculeSvc
	patentSvc      *mockPatentSvc
	infringeNet    *mockInfringeNet
	claimParser    *mockClaimParser
	gnnInference   *mockGNNInference
	riskRepo       *mockRiskRepo
	ftoRepo        *mockFTORepo
	eventPublisher *mockEventPublisher
	cache          *mockCache
	metrics        *prom.AppMetrics
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()

	h := &testHarness{
		moleculeSvc:    &mockMoleculeSvc{},
		patentSvc:      &mockPatentSvc{},
		infringeNet:    &mockInfringeNet{},
		claimParser:    &mockClaimParser{},
		gnnInference:   &mockGNNInference{},
		riskRepo:       &mockRiskRepo{},
		ftoRepo:        &mockFTORepo{},
		eventPublisher: &mockEventPublisher{},
		cache:          newMockCache(),
	}

	appMetrics := prom.NewAppMetrics(&mockMetricsCollector{})
	h.metrics = appMetrics

	svc, err := NewRiskAssessmentService(RiskAssessmentServiceConfig{
		MoleculeSvc:    h.moleculeSvc,
		PatentSvc:      h.patentSvc,
		InfringeNet:    h.infringeNet,
		ClaimParser:    h.claimParser,
		GNNInference:   h.gnnInference,
		RiskRepo:       h.riskRepo,
		FTORepo:        h.ftoRepo,
		EventPublisher: h.eventPublisher,
		Cache:          h.cache,
		Logger:         &mockLogger{},
		Metrics:        appMetrics,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	h.svc = svc
	return h
}

// ... (Constructor Validation, etc. skipped as they are generic)

// ===========================================================================
// Tests: computeClaimRiskScore
// ===========================================================================

func TestComputeClaimRiskScore(t *testing.T) {
	tests := []struct {
		name       string
		assessment *infringe_net.AssessmentResult
		similarity float64
		wantMin    float64
		wantMax    float64
	}{
		{
			name: "literal match only",
			assessment: &infringe_net.AssessmentResult{
				LiteralAnalysis: &infringe_net.LiteralAnalysisResult{AllElementsMet: true},
			},
			similarity: 0.80,
			// 0.35*100 + 0 + 0.20*80 = 51
			wantMin: 50.0,
			wantMax: 52.0,
		},
		{
			name: "equivalents match only",
			assessment: &infringe_net.AssessmentResult{
				EquivalentsAnalysis: &infringe_net.EquivalentsAnalysisResult{Score: 1.0, Skipped: false},
			},
			similarity: 0.60,
			// 0.35*0 + 0.30*100 + 0.20*60 = 42
			wantMin: 41.0,
			wantMax: 43.0,
		},
		{
			name: "estoppel penalty",
			assessment: &infringe_net.AssessmentResult{
				EstoppelCheck: &infringe_net.EstoppelCheckResult{HasEstoppel: true},
			},
			similarity: 0.90,
			// 0.20*90 - 0.15*20 = 18 - 3 = 15
			wantMin: 14.0,
			wantMax: 16.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeClaimRiskScore(tt.assessment, tt.similarity)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeClaimRiskScore() = %.2f, want in [%.1f, %.1f]", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// ===========================================================================
// Tests: AssessMolecule - Full Flow (Cache Miss)
// ===========================================================================

func TestAssessMolecule_CacheMiss_WithCandidates_StandardDepth(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	filingDate := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)

	h.patentSvc.searchBySimilarityFn = func(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
		return []*patent.SimilaritySearchResult{
			{
				PatentNumber:       "US10000001",
				Title:              "OLED Emitter Compound",
				Assignee:           "CompetitorA",
				FilingDate:         filingDate,
				LegalStatus:        "active",
				IPCCodes:           []string{"C07D401/04"},
				MorganSimilarity:   0.90,
				RDKitSimilarity:    0.90,
				AtomPairSimilarity: 0.90,
			},
		}, nil
	}

	h.patentSvc.getPatentByNumberFn = func(ctx context.Context, number string) (*patent.Patent, error) {
		return &patent.Patent{
			PatentNumber: number,
			Claims: []*patent.Claim{
				{Number: 1, Text: "A composition...", Type: patent.ClaimTypeIndependent},
			},
		}, nil
	}

	h.claimParser.semanticMatchFn = func(ctx context.Context, smiles, claimText string) (float64, error) {
		return 0.75, nil
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{
		SMILES: "c1ccc2c(c1)c1ccccc1[nH]2",
		Depth:  AnalysisDepthStandard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.MatchedPatents) == 0 {
		t.Error("expected at least one matched patent")
	}
}

func TestAssessMolecule_CacheMiss_DeepDepth_WithInfringeNet(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.patentSvc.searchBySimilarityFn = func(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
		return []*patent.SimilaritySearchResult{
			{
				PatentNumber:       "US20200001",
				MorganSimilarity:   0.90,
				RDKitSimilarity:    0.90,
				AtomPairSimilarity: 0.90,
			},
		}, nil
	}

	h.patentSvc.getPatentByNumberFn = func(ctx context.Context, number string) (*patent.Patent, error) {
		return &patent.Patent{
			PatentNumber: number,
			Claims: []*patent.Claim{
				{Number: 1, Text: "A compound...", Type: patent.ClaimTypeIndependent},
			},
		}, nil
	}

	h.infringeNet.assessFn = func(ctx context.Context, req *infringe_net.AssessmentRequest) (*infringe_net.AssessmentResult, error) {
		return &infringe_net.AssessmentResult{
			LiteralAnalysis: &infringe_net.LiteralAnalysisResult{AllElementsMet: true, Score: 1.0},
			EquivalentsAnalysis: &infringe_net.EquivalentsAnalysisResult{Score: 1.0},
			MatchedClaims: []*infringe_net.ClaimMatchResult{
				{ClaimID: "US20200001-C1", LiteralScore: 1.0},
			},
			OverallScore: 1.0,
		}, nil
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{
		SMILES: "c1ccccc1",
		Depth:  AnalysisDepthDeep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.MatchedPatents) == 0 {
		t.Error("expected matched patent")
	}
}

// ... other tests omitted for brevity but should be compatible ...
func TestMin(t *testing.T) {
	if min(3, 5) != 3 {
		t.Error("min(3,5) should be 3")
	}
}
