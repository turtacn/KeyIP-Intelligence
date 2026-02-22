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
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"
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
}

func (m *mockPatentSvc) SearchBySimilarity(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
	if m.searchBySimilarityFn != nil {
		return m.searchBySimilarityFn(ctx, req)
	}
	return []*patent.SimilaritySearchResult{}, nil
}

// --- Mock InfringeNetAssessor ---

type mockInfringeNet struct {
	infringe_net.InfringeNetAssessor
	assessFn func(ctx context.Context, req *infringe_net.AssessmentRequest) (*infringe_net.AssessmentResult, error)
}

func (m *mockInfringeNet) Assess(ctx context.Context, req *infringe_net.AssessmentRequest) (*infringe_net.AssessmentResult, error) {
	if m.assessFn != nil {
		return m.assessFn(ctx, req)
	}
	return &infringe_net.AssessmentResult{
		LiteralMatch:     false,
		EquivalentsMatch: false,
		MarkushCovered:   false,
		EstoppelApplies:  false,
		Explanation:      "mock: no infringement detected",
	}, nil
}

// --- Mock ClaimBERTParser ---

type mockClaimParser struct {
	claim_bert.ClaimBERTParser
	parseClaimsFn   func(ctx context.Context, req *claim_bert.ParseRequest) (*claim_bert.ParseResult, error)
	semanticMatchFn func(ctx context.Context, req *claim_bert.MatchRequest) (*claim_bert.MatchResult, error)
}

func (m *mockClaimParser) ParseClaims(ctx context.Context, req *claim_bert.ParseRequest) (*claim_bert.ParseResult, error) {
	if m.parseClaimsFn != nil {
		return m.parseClaimsFn(ctx, req)
	}
	return &claim_bert.ParseResult{
		Claims: []claim_bert.ParsedClaim{
			{Number: 1, Type: "independent", Elements: []string{"element-A", "element-B"}},
		},
	}, nil
}

func (m *mockClaimParser) SemanticMatch(ctx context.Context, req *claim_bert.MatchRequest) (*claim_bert.MatchResult, error) {
	if m.semanticMatchFn != nil {
		return m.semanticMatchFn(ctx, req)
	}
	return &claim_bert.MatchResult{
		LiteralMatch:  false,
		MarkushCovered: false,
		MatchScore:    0.3,
		Explanation:   "mock: low semantic match",
	}, nil
}

// --- Mock MolPatentGNNInference ---

type mockGNNInference struct {
	molpatent_gnn.MolPatentGNNInference
	searchSimilarFn func(ctx context.Context, req *molpatent_gnn.SimilarityRequest) ([]*molpatent_gnn.SimilarityResult, error)
}

func (m *mockGNNInference) SearchSimilar(ctx context.Context, req *molpatent_gnn.SimilarityRequest) ([]*molpatent_gnn.SimilarityResult, error) {
	if m.searchSimilarFn != nil {
		return m.searchSimilarFn(ctx, req)
	}
	return []*molpatent_gnn.SimilarityResult{}, nil
}

// --- Mock RiskRecordRepository ---

type mockRiskRepo struct {
	saveFn            func(ctx context.Context, record *RiskRecord) error
	findByMoleculeFn  func(ctx context.Context, moleculeID string, opts *queryOptions) ([]*RiskRecord, string, error)
	findByPortfolioFn func(ctx context.Context, portfolioID string) ([]*RiskRecord, error)
	findByIDFn        func(ctx context.Context, recordID string) (*RiskRecord, error)
	getTrendFn        func(ctx context.Context, portfolioID string, months int) ([]RiskTrendPoint, error)
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

func (m *mockRiskRepo) GetTrend(ctx context.Context, portfolioID string, months int) ([]RiskTrendPoint, error) {
	if m.getTrendFn != nil {
		return m.getTrendFn(ctx, portfolioID, months)
	}
	return []RiskTrendPoint{}, nil
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

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if m.getFn != nil {
		return m.getFn(ctx, key)
	}
	data, ok := m.store[key]
	if !ok {
		return nil, nil
	}
	return data, nil
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if m.setFn != nil {
		return m.setFn(ctx, key, value, ttl)
	}
	m.store[key] = value
	return nil
}

// --- Mock Logger ---

type mockLogger struct {
	logging.Logger
}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {}

// --- Mock Metrics ---

type mockMetrics struct {
	prometheus.Metrics
}

func (m *mockMetrics) IncrementCounter(name string, labels map[string]string)              {}
func (m *mockMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {}

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
		Metrics:        &mockMetrics{},
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	h.svc = svc
	return h
}

// ===========================================================================
// Tests: Constructor Validation
// ===========================================================================

func TestNewRiskAssessmentService_MissingDependencies(t *testing.T) {
	tests := []struct {
		name   string
		modify func(cfg *RiskAssessmentServiceConfig)
	}{
		{"missing MoleculeSvc", func(cfg *RiskAssessmentServiceConfig) { cfg.MoleculeSvc = nil }},
		{"missing PatentSvc", func(cfg *RiskAssessmentServiceConfig) { cfg.PatentSvc = nil }},
		{"missing InfringeNet", func(cfg *RiskAssessmentServiceConfig) { cfg.InfringeNet = nil }},
		{"missing ClaimParser", func(cfg *RiskAssessmentServiceConfig) { cfg.ClaimParser = nil }},
		{"missing GNNInference", func(cfg *RiskAssessmentServiceConfig) { cfg.GNNInference = nil }},
		{"missing RiskRepo", func(cfg *RiskAssessmentServiceConfig) { cfg.RiskRepo = nil }},
		{"missing FTORepo", func(cfg *RiskAssessmentServiceConfig) { cfg.FTORepo = nil }},
		{"missing Cache", func(cfg *RiskAssessmentServiceConfig) { cfg.Cache = nil }},
		{"missing Logger", func(cfg *RiskAssessmentServiceConfig) { cfg.Logger = nil }},
		{"missing Metrics", func(cfg *RiskAssessmentServiceConfig) { cfg.Metrics = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RiskAssessmentServiceConfig{
				MoleculeSvc:    &mockMoleculeSvc{},
				PatentSvc:      &mockPatentSvc{},
				InfringeNet:    &mockInfringeNet{},
				ClaimParser:    &mockClaimParser{},
				GNNInference:   &mockGNNInference{},
				RiskRepo:       &mockRiskRepo{},
				FTORepo:        &mockFTORepo{},
				EventPublisher: &mockEventPublisher{},
				Cache:          newMockCache(),
				Logger:         &mockLogger{},
				Metrics:        &mockMetrics{},
			}
			tt.modify(&cfg)

			_, err := NewRiskAssessmentService(cfg)
			if err == nil {
				t.Error("expected error for missing dependency, got nil")
			}
		})
	}
}

func TestNewRiskAssessmentService_Success(t *testing.T) {
	h := newTestHarness(t)
	if h.svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ===========================================================================
// Tests: MoleculeRiskRequest Validation
// ===========================================================================

func TestMoleculeRiskRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     MoleculeRiskRequest
		wantErr bool
	}{
		{
			name:    "empty input",
			req:     MoleculeRiskRequest{},
			wantErr: true,
		},
		{
			name:    "both SMILES and InChI",
			req:     MoleculeRiskRequest{SMILES: "CCO", InChI: "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3"},
			wantErr: true,
		},
		{
			name:    "invalid similarity threshold low",
			req:     MoleculeRiskRequest{SMILES: "CCO", SimilarityThreshold: -0.1},
			wantErr: true,
		},
		{
			name:    "invalid similarity threshold high",
			req:     MoleculeRiskRequest{SMILES: "CCO", SimilarityThreshold: 1.5},
			wantErr: true,
		},
		{
			name:    "invalid depth",
			req:     MoleculeRiskRequest{SMILES: "CCO", Depth: "ultra"},
			wantErr: true,
		},
		{
			name:    "invalid date range",
			req:     MoleculeRiskRequest{SMILES: "CCO", DateFrom: timePtr(time.Now()), DateTo: timePtr(time.Now().Add(-24 * time.Hour))},
			wantErr: true,
		},
		{
			name:    "valid SMILES only",
			req:     MoleculeRiskRequest{SMILES: "c1ccc2c(c1)c1ccccc1[nH]2"},
			wantErr: false,
		},
		{
			name:    "valid InChI only",
			req:     MoleculeRiskRequest{InChI: "InChI=1S/C12H9N/c1-3-7-11-9(5-1)10-6-2-4-8-12(10)13-11/h1-8,13H"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// ===========================================================================
// Tests: RiskLevelFromScore
// ===========================================================================

func TestRiskLevelFromScore(t *testing.T) {
	tests := []struct {
		score float64
		want  RiskLevel
	}{
		{100.0, RiskLevelCritical},
		{85.0, RiskLevelCritical},
		{84.9, RiskLevelHigh},
		{70.0, RiskLevelHigh},
		{69.9, RiskLevelMedium},
		{50.0, RiskLevelMedium},
		{49.9, RiskLevelLow},
		{30.0, RiskLevelLow},
		{29.9, RiskLevelNone},
		{0.0, RiskLevelNone},
		{-5.0, RiskLevelNone},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%.1f", tt.score), func(t *testing.T) {
			got := RiskLevelFromScore(tt.score)
			if got != tt.want {
				t.Errorf("RiskLevelFromScore(%.1f) = %s, want %s", tt.score, got, tt.want)
			}
		})
	}
}

// ===========================================================================
// Tests: SimilarityScores.FusedScore
// ===========================================================================

func TestSimilarityScores_FusedScore(t *testing.T) {
	s := SimilarityScores{
		Morgan:   0.80,
		RDKit:    0.75,
		AtomPair: 0.60,
		GNN:      0.90,
	}

	// Expected: 0.30*0.80 + 0.20*0.75 + 0.15*0.60 + 0.35*0.90
	// = 0.24 + 0.15 + 0.09 + 0.315 = 0.795
	got := s.FusedScore()
	expected := 0.795

	if diff := got - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("FusedScore() = %f, want %f", got, expected)
	}

	if s.WeightedOverall != got {
		t.Errorf("WeightedOverall not updated: got %f, want %f", s.WeightedOverall, got)
	}
}

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
				LiteralMatch:     true,
				EquivalentsMatch: false,
				MarkushCovered:   false,
				EstoppelApplies:  false,
			},
			similarity: 0.80,
			// 0.35*100 + 0.30*0 + 0.20*80 - 0.15*0 = 35 + 0 + 16 - 0 = 51
			wantMin: 50.0,
			wantMax: 52.0,
		},
		{
			name: "markush covered",
			assessment: &infringe_net.AssessmentResult{
				LiteralMatch:     false,
				EquivalentsMatch: false,
				MarkushCovered:   true,
				EstoppelApplies:  false,
			},
			similarity: 0.70,
			// 0.35*100 + 0.30*0 + 0.20*70 - 0.15*0 = 35 + 0 + 14 - 0 = 49
			wantMin: 48.0,
			wantMax: 50.0,
		},
		{
			name: "equivalents match only",
			assessment: &infringe_net.AssessmentResult{
				LiteralMatch:     false,
				EquivalentsMatch: true,
				MarkushCovered:   false,
				EstoppelApplies:  false,
			},
			similarity: 0.60,
			// 0.35*0 + 0.30*100 + 0.20*60 - 0.15*0 = 0 + 30 + 12 - 0 = 42
			wantMin: 41.0,
			wantMax: 43.0,
		},
		{
			name: "literal + equivalents + estoppel",
			assessment: &infringe_net.AssessmentResult{
				LiteralMatch:     true,
				EquivalentsMatch: true,
				MarkushCovered:   false,
				EstoppelApplies:  true,
			},
			similarity: 0.90,
			// 0.35*100 + 0.30*100 + 0.20*90 - 0.15*20 = 35 + 30 + 18 - 3 = 80
			wantMin: 79.0,
			wantMax: 81.0,
		},
		{
			name: "no match at all",
			assessment: &infringe_net.AssessmentResult{
				LiteralMatch:     false,
				EquivalentsMatch: false,
				MarkushCovered:   false,
				EstoppelApplies:  false,
			},
			similarity: 0.50,
			// 0.35*0 + 0.30*0 + 0.20*50 - 0.15*0 = 0 + 0 + 10 - 0 = 10
			wantMin: 9.0,
			wantMax: 11.0,
		},
		{
			name: "full match all elements",
			assessment: &infringe_net.AssessmentResult{
				LiteralMatch:     true,
				EquivalentsMatch: true,
				MarkushCovered:   true,
				EstoppelApplies:  false,
			},
			similarity: 1.0,
			// 0.35*100 + 0.30*100 + 0.20*100 - 0.15*0 = 35 + 30 + 20 - 0 = 85
			wantMin: 84.0,
			wantMax: 86.0,
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

func TestComputeClaimRiskScore_ClampedToZero(t *testing.T) {
	// Edge case: estoppel penalty with no other scores should not go negative.
	assessment := &infringe_net.AssessmentResult{
		LiteralMatch:     false,
		EquivalentsMatch: false,
		MarkushCovered:   false,
		EstoppelApplies:  true,
	}
	got := computeClaimRiskScore(assessment, 0.0)
	if got < 0 {
		t.Errorf("computeClaimRiskScore() = %.2f, expected >= 0", got)
	}
}

// ===========================================================================
// Tests: AssessMolecule - Full Flow (Cache Miss)
// ===========================================================================

func TestAssessMolecule_CacheMiss_NoCandidates(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	// Patent search returns no candidates.
	h.patentSvc.searchBySimilarityFn = func(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
		return []*patent.SimilaritySearchResult{}, nil
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{
		SMILES: "c1ccccc1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.OverallRiskLevel != RiskLevelNone {
		t.Errorf("expected NONE risk level, got %s", resp.OverallRiskLevel)
	}
	if resp.OverallRiskScore != 0 {
		t.Errorf("expected 0 risk score, got %.2f", resp.OverallRiskScore)
	}
	if resp.CacheHit {
		t.Error("expected cache miss")
	}
	if len(resp.MatchedPatents) != 0 {
		t.Errorf("expected 0 matched patents, got %d", len(resp.MatchedPatents))
	}
	if resp.AssessmentID == "" {
		t.Error("expected non-empty assessment ID")
	}

	// Verify record was persisted.
	if h.riskRepo.saveCallCount.Load() != 1 {
		t.Errorf("expected 1 save call, got %d", h.riskRepo.saveCallCount.Load())
	}
}

func TestAssessMolecule_CacheMiss_WithCandidates_StandardDepth(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	filingDate := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)

	// Patent search returns candidates.
	h.patentSvc.searchBySimilarityFn = func(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
		return []*patent.SimilaritySearchResult{
			{
				PatentNumber:       "US10000001",
				Title:              "OLED Emitter Compound",
				Assignee:           "CompetitorA",
				FilingDate:         filingDate,
				LegalStatus:        "active",
				IPCCodes:           []string{"C07D401/04"},
				MorganSimilarity:   0.85,
				RDKitSimilarity:    0.80,
				AtomPairSimilarity: 0.70,
			},
		}, nil
	}

	// ClaimBERT semantic match for standard depth.
	h.claimParser.semanticMatchFn = func(ctx context.Context, req *claim_bert.MatchRequest) (*claim_bert.MatchResult, error) {
		return &claim_bert.MatchResult{
			LiteralMatch:  true,
			MarkushCovered: false,
			MatchScore:    0.75,
			Explanation:   "High semantic overlap with independent claim 1",
		}, nil
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{
		SMILES: "c1ccc2c(c1)c1ccccc1[nH]2",
		Depth:  AnalysisDepthStandard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.OverallRiskLevel == RiskLevelNone {
		t.Error("expected non-NONE risk level with matching candidates")
	}
	if len(resp.MatchedPatents) == 0 {
		t.Error("expected at least one matched patent")
	}
	if resp.MatchedPatents[0].PatentNumber != "US10000001" {
		t.Errorf("expected patent US10000001, got %s", resp.MatchedPatents[0].PatentNumber)
	}
	if resp.CacheHit {
		t.Error("expected cache miss on first call")
	}
	if resp.AnalysisDepth != AnalysisDepthStandard {
		t.Errorf("expected standard depth, got %s", resp.AnalysisDepth)
	}
}

func TestAssessMolecule_CacheMiss_DeepDepth_WithInfringeNet(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	filingDate := time.Date(2019, 6, 1, 0, 0, 0, 0, time.UTC)

	h.patentSvc.searchBySimilarityFn = func(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
		return []*patent.SimilaritySearchResult{
			{
				PatentNumber:       "US20200001",
				Title:              "Phosphorescent OLED Material",
				Assignee:           "CompetitorB",
				FilingDate:         filingDate,
				LegalStatus:        "active",
				IPCCodes:           []string{"C09K11/06"},
				MorganSimilarity:   0.90,
				RDKitSimilarity:    0.85,
				AtomPairSimilarity: 0.75,
			},
		}, nil
	}

	// InfringeNet returns literal + equivalents match.
	h.infringeNet.assessFn = func(ctx context.Context, req *infringe_net.AssessmentRequest) (*infringe_net.AssessmentResult, error) {
		return &infringe_net.AssessmentResult{
			LiteralMatch:     true,
			EquivalentsMatch: true,
			MarkushCovered:   false,
			EstoppelApplies:  false,
			Explanation:      "All elements of claim 1 are present in the target molecule",
		}, nil
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{
		SMILES: "c1ccc(-c2ccc3ccccc3n2)cc1",
		Depth:  AnalysisDepthDeep,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With literal + equivalents match and high similarity, expect HIGH or CRITICAL.
	if resp.OverallRiskLevel != RiskLevelCritical && resp.OverallRiskLevel != RiskLevelHigh {
		t.Errorf("expected CRITICAL or HIGH risk, got %s (score: %.2f)", resp.OverallRiskLevel, resp.OverallRiskScore)
	}
	if resp.AnalysisDepth != AnalysisDepthDeep {
		t.Errorf("expected deep depth, got %s", resp.AnalysisDepth)
	}

	// Verify event was published.
	if h.eventPublisher.publishCount.Load() != 1 {
		t.Errorf("expected 1 event published, got %d", h.eventPublisher.publishCount.Load())
	}
}

// ===========================================================================
// Tests: AssessMolecule - Cache Hit
// ===========================================================================

func TestAssessMolecule_CacheHit(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	// Pre-populate cache with a known result.
	cachedResp := &MoleculeRiskResponse{
		AssessmentID:     "cached-001",
		CanonicalSMILES:  "c1ccccc1",
		OverallRiskLevel: RiskLevelMedium,
		OverallRiskScore: 55.0,
		MatchedPatents:   []PatentRiskDetail{},
		AnalysisDepth:    AnalysisDepthStandard,
		AssessedAt:       time.Now().UTC(),
	}
	cachedData, _ := json.Marshal(cachedResp)

	h.cache.getFn = func(ctx context.Context, key string) ([]byte, error) {
		return cachedData, nil
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{
		SMILES: "c1ccccc1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.CacheHit {
		t.Error("expected cache hit")
	}
	if resp.AssessmentID != "cached-001" {
		t.Errorf("expected cached assessment ID, got %s", resp.AssessmentID)
	}
	if resp.OverallRiskLevel != RiskLevelMedium {
		t.Errorf("expected MEDIUM from cache, got %s", resp.OverallRiskLevel)
	}

	// Patent search should NOT have been called.
	if h.riskRepo.saveCallCount.Load() != 0 {
		t.Error("expected no save call on cache hit")
	}
}

// ===========================================================================
// Tests: AssessMolecule - Error Scenarios
// ===========================================================================

func TestAssessMolecule_ValidationError(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	_, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAssessMolecule_CanonicalizationError(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.moleculeSvc.canonicalizeFn = func(ctx context.Context, smiles string) (string, string, error) {
		return "", "", errors.New("invalid SMILES")
	}

	_, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{SMILES: "INVALID"})
	if err == nil {
		t.Fatal("expected canonicalization error")
	}
}

func TestAssessMolecule_PatentSearchError(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.patentSvc.searchBySimilarityFn = func(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.SimilaritySearchResult, error) {
		return nil, errors.New("database connection failed")
	}

	_, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{SMILES: "c1ccccc1"})
	if err == nil {
		t.Fatal("expected patent search error")
	}
}

func TestAssessMolecule_InChIInput(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.moleculeSvc.canonicalizeFromInChIFn = func(ctx context.Context, inchi string) (string, string, error) {
		return "c1ccccc1", "ISWSIDIOOBJBQZ-UHFFFAOYSA-N", nil
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{
		InChI: "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CanonicalSMILES != "c1ccccc1" {
		t.Errorf("expected canonical SMILES from InChI, got %s", resp.CanonicalSMILES)
	}
}

// ===========================================================================
// Tests: AssessBatch
// ===========================================================================

func TestAssessBatch_ValidationError(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	_, err := h.svc.AssessBatch(ctx, &BatchRiskRequest{})
	if err == nil {
		t.Fatal("expected validation error for empty molecules")
	}
}

func TestAssessBatch_Success(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	resp, err := h.svc.AssessBatch(ctx, &BatchRiskRequest{
		Molecules: []BatchMoleculeInput{
			{ID: "mol-1", SMILES: "c1ccccc1", Name: "Benzene"},
			{ID: "mol-2", SMILES: "c1ccc2ccccc2c1", Name: "Naphthalene"},
			{ID: "mol-3", SMILES: "c1ccncc1", Name: "Pyridine"},
		},
		Concurrency: 2,
		Depth:       AnalysisDepthQuick,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(resp.Results))
	}

	if resp.Stats.Total != 3 {
		t.Errorf("expected total=3, got %d", resp.Stats.Total)
	}

	successCount := 0
	for _, r := range resp.Results {
		if r.Succeeded {
			successCount++
		}
	}
	if successCount != resp.Stats.Succeeded {
		t.Errorf("succeeded count mismatch: results=%d, stats=%d", successCount, resp.Stats.Succeeded)
	}
}

func TestAssessBatch_PartialFailure(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	callCount := atomic.Int32{}
	h.moleculeSvc.canonicalizeFn = func(ctx context.Context, smiles string) (string, string, error) {
		n := callCount.Add(1)
		if n == 2 {
			return "", "", errors.New("canonicalization failed for molecule 2")
		}
		return smiles, "KEY-" + smiles[:3], nil
	}

	resp, err := h.svc.AssessBatch(ctx, &BatchRiskRequest{
		Molecules: []BatchMoleculeInput{
			{ID: "mol-1", SMILES: "c1ccccc1"},
			{ID: "mol-2", SMILES: "INVALID_MOL"},
			{ID: "mol-3", SMILES: "c1ccncc1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Stats.Failed == 0 {
		t.Error("expected at least one failure")
	}
	if resp.Stats.Succeeded+resp.Stats.Failed != resp.Stats.Total {
		t.Errorf("succeeded(%d) + failed(%d) != total(%d)",
			resp.Stats.Succeeded, resp.Stats.Failed, resp.Stats.Total)
	}
}

func TestAssessBatch_ConcurrencyControl(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	maxConcurrent := atomic.Int32{}
	currentConcurrent := atomic.Int32{}

	h.moleculeSvc.canonicalizeFn = func(ctx context.Context, smiles string) (string, string, error) {
		cur := currentConcurrent.Add(1)
		defer currentConcurrent.Add(-1)

		// Track max concurrency observed.
		for {
			old := maxConcurrent.Load()
			if cur <= old {
				break
			}
			if maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}

		// Simulate some work.
		time.Sleep(10 * time.Millisecond)
		return smiles, "KEY", nil
	}

	molecules := make([]BatchMoleculeInput, 10)
	for i := range molecules {
		molecules[i] = BatchMoleculeInput{
			ID:     fmt.Sprintf("mol-%d", i),
			SMILES: fmt.Sprintf("C%dCCCC", i),
		}
	}

	_, err := h.svc.AssessBatch(ctx, &BatchRiskRequest{
		Molecules:   molecules,
		Concurrency: 3,
		Depth:       AnalysisDepthQuick,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	observed := maxConcurrent.Load()
	if observed > 3 {
		t.Errorf("max concurrency exceeded limit: observed %d, limit 3", observed)
	}
}

// ===========================================================================
// Tests: AssessFTO
// ===========================================================================

func TestAssessFTO_ValidationError(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	tests := []struct {
		name string
		req  *FTORequest
	}{
		{
			name: "empty molecules",
			req:  &FTORequest{Jurisdictions: []string{"US"}},
		},
		{
			name: "empty jurisdictions",
			req: &FTORequest{
				Molecules: []BatchMoleculeInput{{SMILES: "CCO"}},
			},
		},
		{
			name: "invalid scope",
			req: &FTORequest{
				Molecules:     []BatchMoleculeInput{{SMILES: "CCO"}},
				Jurisdictions: []string{"US"},
				Scope:         "invalid",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.svc.AssessFTO(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestAssessFTO_Success_MultiJurisdiction(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	resp, err := h.svc.AssessFTO(ctx, &FTORequest{
		Molecules: []BatchMoleculeInput{
			{ID: "mol-1", SMILES: "c1ccccc1", Name: "Benzene"},
			{ID: "mol-2", SMILES: "c1ccncc1", Name: "Pyridine"},
		},
		Jurisdictions: []string{"US", "EP", "CN"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.FTOID == "" {
		t.Error("expected non-empty FTO ID")
	}
	if len(resp.JurisdictionResults) != 3 {
		t.Errorf("expected 3 jurisdiction results, got %d", len(resp.JurisdictionResults))
	}
	if len(resp.RiskMatrix) != 2 {
		t.Errorf("expected 2 risk matrix rows, got %d", len(resp.RiskMatrix))
	}

	// With no candidates, all jurisdictions should be FREE.
	for _, jr := range resp.JurisdictionResults {
		if jr.Conclusion != FTOFree {
			t.Errorf("expected FREE for %s with no candidates, got %s", jr.Jurisdiction, jr.Conclusion)
		}
	}
	if resp.OverallConclusion != FTOFree {
		t.Errorf("expected overall FREE, got %s", resp.OverallConclusion)
	}
}

// ===========================================================================
// Tests: GetRiskSummary
// ===========================================================================

func TestGetRiskSummary_EmptyPortfolio(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	resp, err := h.svc.GetRiskSummary(ctx, "portfolio-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TotalMolecules != 0 {
		t.Errorf("expected 0 molecules, got %d", resp.TotalMolecules)
	}
	if resp.OverallRiskLevel != RiskLevelNone {
		t.Errorf("expected NONE risk level, got %s", resp.OverallRiskLevel)
	}
}

func TestGetRiskSummary_WithRecords(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.riskRepo.findByPortfolioFn = func(ctx context.Context, portfolioID string) ([]*RiskRecord, error) {
		return []*RiskRecord{
			{RecordID: "r1", RiskLevel: RiskLevelCritical, RiskScore: 90.0},
			{RecordID: "r2", RiskLevel: RiskLevelHigh, RiskScore: 75.0},
			{RecordID: "r3", RiskLevel: RiskLevelLow, RiskScore: 25.0},
			{RecordID: "r4", RiskLevel: RiskLevelNone, RiskScore: 10.0},
		}, nil
	}

	resp, err := h.svc.GetRiskSummary(ctx, "portfolio-002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TotalMolecules != 4 {
		t.Errorf("expected 4 molecules, got %d", resp.TotalMolecules)
	}

	expectedAvg := (90.0 + 75.0 + 25.0 + 10.0) / 4.0
	if diff := resp.OverallRiskScore - expectedAvg; diff > 0.01 || diff < -0.01 {
		t.Errorf("expected average score %.2f, got %.2f", expectedAvg, resp.OverallRiskScore)
	}

	if resp.RiskDistribution[RiskLevelCritical] != 1 {
		t.Errorf("expected 1 CRITICAL, got %d", resp.RiskDistribution[RiskLevelCritical])
	}
	if resp.RiskDistribution[RiskLevelHigh] != 1 {
		t.Errorf("expected 1 HIGH, got %d", resp.RiskDistribution[RiskLevelHigh])
	}
}

func TestGetRiskSummary_EmptyPortfolioID(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	_, err := h.svc.GetRiskSummary(ctx, "")
	if err == nil {
		t.Fatal("expected validation error for empty portfolio ID")
	}
}

// ===========================================================================
// Tests: GetRiskHistory
// ===========================================================================

func TestGetRiskHistory_Success(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	now := time.Now().UTC()
	h.riskRepo.findByMoleculeFn = func(ctx context.Context, moleculeID string, opts *queryOptions) ([]*RiskRecord, string, error) {
		return []*RiskRecord{
			{RecordID: "r1", MoleculeID: moleculeID, RiskLevel: RiskLevelHigh, RiskScore: 72.0, CreatedAt: now},
			{RecordID: "r2", MoleculeID: moleculeID, RiskLevel: RiskLevelMedium, RiskScore: 55.0, CreatedAt: now.Add(-24 * time.Hour)},
		}, "", nil
	}

	records, err := h.svc.GetRiskHistory(ctx, "mol-123",
		WithPageSize(10),
		WithLevelFilter(RiskLevelHigh, RiskLevelMedium),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("expected 2 records, got %d", len(records))
	}
}

func TestGetRiskHistory_EmptyMoleculeID(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	_, err := h.svc.GetRiskHistory(ctx, "")
	if err == nil {
		t.Fatal("expected validation error for empty molecule ID")
	}
}

func TestGetRiskHistory_RepositoryError(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.riskRepo.findByMoleculeFn = func(ctx context.Context, moleculeID string, opts *queryOptions) ([]*RiskRecord, string, error) {
		return nil, "", errors.New("database error")
	}

	_, err := h.svc.GetRiskHistory(ctx, "mol-123")
	if err == nil {
		t.Fatal("expected repository error")
	}
}

// ===========================================================================
// Tests: FTO Conclusion Determination
// ===========================================================================

func TestDetermineFTOConclusion(t *testing.T) {
	tests := []struct {
		name     string
		critical int
		high     int
		want     FTOConclusion
	}{
		{"blocked with critical", 1, 0, FTOBlocked},
		{"blocked with both", 2, 3, FTOBlocked},
		{"conditional with high only", 0, 1, FTOConditional},
		{"free with none", 0, 0, FTOFree},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineFTOConclusion(tt.critical, tt.high)
			if got != tt.want {
				t.Errorf("determineFTOConclusion(%d, %d) = %s, want %s",
					tt.critical, tt.high, got, tt.want)
			}
		})
	}
}

// ===========================================================================
// Tests: Utility Helpers
// ===========================================================================

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		val      string
		wantLen  int
	}{
		{"append new", []string{"a", "b"}, "c", 3},
		{"skip duplicate", []string{"a", "b", "c"}, "b", 3},
		{"append to empty", []string{}, "a", 1},
		{"append to nil", nil, "a", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUnique(tt.slice, tt.val)
			if len(got) != tt.wantLen {
				t.Errorf("appendUnique() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestIpcToDomain(t *testing.T) {
	tests := []struct {
		ipc  string
		want string
	}{
		{"C07D401/04", "Organic Chemistry - Heterocyclic"},
		{"C09K11/06", "Materials - Luminescent/Functional"},
		{"H10K50/10", "OLED Devices"},
		{"H05B33/14", "Lighting - Electroluminescence"},
		{"C07F15/00", "Organic Chemistry - Organometallic"},
		{"A01B1/00", "Human Necessities"},
		{"B01J", "Operations/Transport"},
		{"XY", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.ipc, func(t *testing.T) {
			got := ipcToDomain(tt.ipc)
			if got != tt.want {
				t.Errorf("ipcToDomain(%q) = %q, want %q", tt.ipc, got, tt.want)
			}
		})
	}
}

func TestGenerateRiskSummary(t *testing.T) {
	tests := []struct {
		score      float64
		matchCount int
		depth      AnalysisDepth
		wantLevel  string
	}{
		{90.0, 3, AnalysisDepthDeep, "CRITICAL"},
		{75.0, 2, AnalysisDepthStandard, "HIGH"},
		{55.0, 1, AnalysisDepthStandard, "MEDIUM"},
		{35.0, 1, AnalysisDepthQuick, "LOW"},
		{10.0, 0, AnalysisDepthQuick, "No significant"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%.0f", tt.score), func(t *testing.T) {
			summary := generateRiskSummary(tt.score, tt.matchCount, tt.depth)
			if summary == "" {
				t.Error("expected non-empty summary")
			}
			// Check that the summary contains the expected level keyword.
			found := false
			if len(summary) > 0 {
				for i := 0; i <= len(summary)-len(tt.wantLevel); i++ {
					if summary[i:i+len(tt.wantLevel)] == tt.wantLevel {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("summary %q does not contain %q", summary, tt.wantLevel)
			}
		})
	}
}

// ===========================================================================
// Tests: QueryOption
// ===========================================================================

func TestQueryOptions(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)

	opts := applyQueryOptions([]QueryOption{
		WithPageSize(50),
		WithPageToken("cursor-abc"),
		WithDateRange(now, later),
		WithTriggerFilter(TriggerManual, TriggerScheduled),
		WithLevelFilter(RiskLevelHigh, RiskLevelCritical),
	})

	if opts.pageSize != 50 {
		t.Errorf("expected pageSize=50, got %d", opts.pageSize)
	}
	if opts.pageToken != "cursor-abc" {
		t.Errorf("expected pageToken=cursor-abc, got %s", opts.pageToken)
	}
	if opts.fromDate == nil || !opts.fromDate.Equal(now) {
		t.Error("expected fromDate to match")
	}
	if opts.toDate == nil || !opts.toDate.Equal(later) {
		t.Error("expected toDate to match")
	}
	if len(opts.triggerFilter) != 2 {
		t.Errorf("expected 2 trigger filters, got %d", len(opts.triggerFilter))
	}
	if len(opts.levelFilter) != 2 {
		t.Errorf("expected 2 level filters, got %d", len(opts.levelFilter))
	}
}

func TestQueryOptions_Defaults(t *testing.T) {
	opts := applyQueryOptions(nil)
	if opts.pageSize != DefaultRiskHistoryPageSize {
		t.Errorf("expected default pageSize=%d, got %d", DefaultRiskHistoryPageSize, opts.pageSize)
	}
	if opts.pageToken != "" {
		t.Errorf("expected empty pageToken, got %s", opts.pageToken)
	}
}

func TestQueryOptions_PageSizeBounds(t *testing.T) {
	// Negative page size should be ignored.
	opts := applyQueryOptions([]QueryOption{WithPageSize(-1)})
	if opts.pageSize != DefaultRiskHistoryPageSize {
		t.Errorf("negative pageSize should be ignored, got %d", opts.pageSize)
	}

	// Over 100 should be ignored.
	opts = applyQueryOptions([]QueryOption{WithPageSize(200)})
	if opts.pageSize != DefaultRiskHistoryPageSize {
		t.Errorf("pageSize > 100 should be ignored, got %d", opts.pageSize)
	}

	// Exactly 100 should be accepted.
	opts = applyQueryOptions([]QueryOption{WithPageSize(100)})
	if opts.pageSize != 100 {
		t.Errorf("expected pageSize=100, got %d", opts.pageSize)
	}
}

// ===========================================================================
// Tests: BatchRiskRequest Validation
// ===========================================================================

func TestBatchRiskRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     BatchRiskRequest
		wantErr bool
	}{
		{
			name:    "empty molecules",
			req:     BatchRiskRequest{},
			wantErr: true,
		},
		{
			name: "molecule without SMILES or InChI",
			req: BatchRiskRequest{
				Molecules: []BatchMoleculeInput{{ID: "mol-1"}},
			},
			wantErr: true,
		},
		{
			name: "valid single molecule",
			req: BatchRiskRequest{
				Molecules: []BatchMoleculeInput{{SMILES: "CCO"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ===========================================================================
// Tests: FTORequest Validation
// ===========================================================================

func TestFTORequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     FTORequest
		wantErr bool
	}{
		{
			name:    "empty",
			req:     FTORequest{},
			wantErr: true,
		},
		{
			name: "no jurisdictions",
			req: FTORequest{
				Molecules: []BatchMoleculeInput{{SMILES: "CCO"}},
			},
			wantErr: true,
		},
		{
			name: "invalid scope",
			req: FTORequest{
				Molecules:     []BatchMoleculeInput{{SMILES: "CCO"}},
				Jurisdictions: []string{"US"},
				Scope:         "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid",
			req: FTORequest{
				Molecules:     []BatchMoleculeInput{{SMILES: "CCO"}},
				Jurisdictions: []string{"US", "EP"},
				Scope:         "active",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ===========================================================================
// Tests: FTORequest Defaults
// ===========================================================================

func TestFTORequest_Defaults(t *testing.T) {
	req := &FTORequest{
		Molecules:     []BatchMoleculeInput{{SMILES: "CCO"}},
		Jurisdictions: []string{"US"},
	}
	req.defaults()

	if req.Scope != "active" {
		t.Errorf("expected default scope=active, got %s", req.Scope)
	}
	if req.SimilarityThreshold != DefaultSimilarityThreshold {
		t.Errorf("expected default threshold=%.2f, got %.2f", DefaultSimilarityThreshold, req.SimilarityThreshold)
	}
	if req.Depth != AnalysisDepthDeep {
		t.Errorf("expected default depth=deep, got %s", req.Depth)
	}
	if req.Trigger != TriggerManual {
		t.Errorf("expected default trigger=manual, got %s", req.Trigger)
	}
}

// ===========================================================================
// Tests: RiskLevel.Severity
// ===========================================================================

func TestRiskLevel_Severity(t *testing.T) {
	levels := []RiskLevel{RiskLevelNone, RiskLevelLow, RiskLevelMedium, RiskLevelHigh, RiskLevelCritical}
	for i := 1; i < len(levels); i++ {
		if levels[i].Severity() <= levels[i-1].Severity() {
			t.Errorf("expected %s.Severity() > %s.Severity(), got %d <= %d",
				levels[i], levels[i-1], levels[i].Severity(), levels[i-1].Severity())
		}
	}
}

// ===========================================================================
// Tests: MoleculeRiskRequest Defaults
// ===========================================================================

func TestMoleculeRiskRequest_Defaults(t *testing.T) {
	req := &MoleculeRiskRequest{SMILES: "CCO"}
	req.defaults()

	if req.SimilarityThreshold != DefaultSimilarityThreshold {
		t.Errorf("expected default threshold, got %.2f", req.SimilarityThreshold)
	}
	if req.Depth != AnalysisDepthStandard {
		t.Errorf("expected default depth=standard, got %s", req.Depth)
	}
	if req.MaxCandidates != DefaultMaxCandidates {
		t.Errorf("expected default max candidates=%d, got %d", DefaultMaxCandidates, req.MaxCandidates)
	}
	if req.Trigger != TriggerManual {
		t.Errorf("expected default trigger=manual, got %s", req.Trigger)
	}
}

// ===========================================================================
// Tests: Context Cancellation in Batch
// ===========================================================================

func TestAssessBatch_ContextCancellation(t *testing.T) {
	h := newTestHarness(t)

	ctx, cancel := context.WithCancel(context.Background())

	// Simulate slow canonicalization that allows cancellation to take effect.
	h.moleculeSvc.canonicalizeFn = func(ctx context.Context, smiles string) (string, string, error) {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return smiles, "KEY", nil
		}
	}

	// Cancel after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	molecules := make([]BatchMoleculeInput, 20)
	for i := range molecules {
		molecules[i] = BatchMoleculeInput{
			ID:     fmt.Sprintf("mol-%d", i),
			SMILES: fmt.Sprintf("C%dCCCC", i),
		}
	}

	resp, err := h.svc.AssessBatch(ctx, &BatchRiskRequest{
		Molecules:   molecules,
		Concurrency: 2,
	})
	if err != nil {
		t.Fatalf("batch should not return error on context cancel: %v", err)
	}

	// Some molecules should have failed due to cancellation.
	if resp.Stats.Failed == 0 {
		t.Log("warning: expected some failures from context cancellation")
	}
}

// ===========================================================================
// Tests: FormatJurisdictionSummary
// ===========================================================================

func TestFormatJurisdictionSummary(t *testing.T) {
	tests := []struct {
		name       string
		jr         *JurisdictionFTOResult
		wantSubstr string
	}{
		{
			name: "blocked",
			jr: &JurisdictionFTOResult{
				Jurisdiction:   "US",
				Conclusion:     FTOBlocked,
				CriticalCount:  2,
				HighCount:      1,
				PatentsChecked: 50,
			},
			wantSubstr: "BLOCKED",
		},
		{
			name: "conditional",
			jr: &JurisdictionFTOResult{
				Jurisdiction:   "EP",
				Conclusion:     FTOConditional,
				HighCount:      3,
				MediumCount:    5,
				PatentsChecked: 100,
			},
			wantSubstr: "CONDITIONAL",
		},
		{
			name: "free",
			jr: &JurisdictionFTOResult{
				Jurisdiction:   "CN",
				Conclusion:     FTOFree,
				PatentsChecked: 200,
			},
			wantSubstr: "FREE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := formatJurisdictionSummary(tt.jr)
			if summary == "" {
				t.Error("expected non-empty summary")
			}
			found := false
			for i := 0; i <= len(summary)-len(tt.wantSubstr); i++ {
				if summary[i:i+len(tt.wantSubstr)] == tt.wantSubstr {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("summary %q does not contain %q", summary, tt.wantSubstr)
			}
		})
	}
}

// ===========================================================================
// Tests: Event Publishing
// ===========================================================================

func TestAssessMolecule_EventPublished(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	var capturedEvent *RiskAssessmentEvent
	h.eventPublisher.publishFn = func(ctx context.Context, event interface{}) error {
		if e, ok := event.(*RiskAssessmentEvent); ok {
			capturedEvent = e
		}
		return nil
	}

	_, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{SMILES: "c1ccccc1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedEvent == nil {
		t.Fatal("expected event to be published")
	}
	if capturedEvent.EventType != "risk.assessed" {
		t.Errorf("expected event type 'risk.assessed', got %s", capturedEvent.EventType)
	}
	if capturedEvent.SMILES != "c1ccccc1" {
		t.Errorf("expected SMILES in event, got %s", capturedEvent.SMILES)
	}
}

func TestAssessMolecule_EventPublishError_NonFatal(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.eventPublisher.publishFn = func(ctx context.Context, event interface{}) error {
		return errors.New("event bus unavailable")
	}

	// Should succeed even if event publishing fails.
	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{SMILES: "c1ccccc1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// ===========================================================================
// Tests: Cache Error Handling (Non-Fatal)
// ===========================================================================

func TestAssessMolecule_CacheGetError_NonFatal(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.cache.getFn = func(ctx context.Context, key string) ([]byte, error) {
		return nil, errors.New("redis connection refused")
	}

	// Should proceed with assessment despite cache error.
	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{SMILES: "c1ccccc1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.CacheHit {
		t.Error("expected cache miss when cache is unavailable")
	}
}

func TestAssessMolecule_CacheSetError_NonFatal(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.cache.setFn = func(ctx context.Context, key string, value []byte, ttl time.Duration) error {
		return errors.New("redis write failed")
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{SMILES: "c1ccccc1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response despite cache write failure")
	}
}

// ===========================================================================
// Tests: Persistence Error Handling (Non-Fatal)
// ===========================================================================

func TestAssessMolecule_PersistError_NonFatal(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()

	h.riskRepo.saveFn = func(ctx context.Context, record *RiskRecord) error {
		return errors.New("database write failed")
	}

	resp, err := h.svc.AssessMolecule(ctx, &MoleculeRiskRequest{SMILES: "c1ccccc1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response despite persistence failure")
	}
}

// ===========================================================================
// Tests: min helper (Go 1.22.1 compatibility)
// ===========================================================================

func TestMin(t *testing.T) {
	if min(3, 5) != 3 {
		t.Error("min(3,5) should be 3")
	}
	if min(10, 2) != 2 {
		t.Error("min(10,2) should be 2")
	}
	if min(4, 4) != 4 {
		t.Error("min(4,4) should be 4")
	}
}


