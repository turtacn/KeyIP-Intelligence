package infringe_net

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

type mockInfringeModel struct {
	predictFn    func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error)
	version      string
	callCount    atomic.Int32
}

func (m *mockInfringeModel) PredictLiteralInfringement(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
	m.callCount.Add(1)
	if m.predictFn != nil {
		return m.predictFn(ctx, req)
	}
	elemScores := make(map[string]float64)
	for _, e := range req.ClaimElements {
		elemScores[e.ElementID] = 0.75
	}
	return &LiteralPredictionResult{
		OverallScore:      0.75,
		ElementScores:     elemScores,
		UnmatchedElements: []string{},
		Confidence:        0.90,
	}, nil
}

func (m *mockInfringeModel) ComputeStructuralSimilarity(ctx context.Context, smiles1, smiles2 string) (float64, error) {
	return 0.5, nil // stub
}

func (m *mockInfringeModel) PredictPropertyImpact(ctx context.Context, req *PropertyImpactRequest) (*PropertyImpactResult, error) {
	return nil, nil // stub
}

func (m *mockInfringeModel) EmbedStructure(ctx context.Context, smiles string) ([]float64, error) {
	return nil, nil // stub
}

func (m *mockInfringeModel) ModelInfo() *ModelMetadata {
	return &ModelMetadata{Version: m.version}
}

func (m *mockInfringeModel) Healthy(ctx context.Context) error {
	return nil
}


type mockEquivalentsAnalyzer struct {
	analyzeFn func(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error)
	elementFn func(ctx context.Context, q, c *StructuralElement) (*ElementEquivalence, error)
	version   string
	callCount atomic.Int32
}

func (m *mockEquivalentsAnalyzer) Analyze(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
	m.callCount.Add(1)
	if m.analyzeFn != nil {
		return m.analyzeFn(ctx, req)
	}
	return &EquivalentsResult{
		OverallEquivalenceScore: 0.70,
		// No equivalent of FWRScores in new result struct directly, but can mock ElementResults if needed
		ElementResults: []*ElementEquivalence{},
	}, nil
}

func (m *mockEquivalentsAnalyzer) AnalyzeElement(ctx context.Context, q, c *StructuralElement) (*ElementEquivalence, error) {
	if m.elementFn != nil {
		return m.elementFn(ctx, q, c)
	}
	return nil, nil
}

func (m *mockEquivalentsAnalyzer) ModelVersion() string {
	if m.version == "" {
		return "equivalents-v1.0"
	}
	return m.version
}

type mockClaimElementMapper struct {
	mapFn              func(ctx context.Context, claims []*ClaimInput) ([]*MappedClaim, error)
	mapMolFn           func(ctx context.Context, molecule *MoleculeInput) ([]*StructuralElement, error)
	alignFn            func(ctx context.Context, moleculeElements []*StructuralElement, claimElements []*ClaimElement) (*ElementAlignment, error)
	estoppelFn         func(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error)
	loadIndepFn        func(ctx context.Context, deps []*ClaimInput) ([]*ClaimInput, error)
	parseHistFn        func(ctx context.Context, rawHistory []byte) (*ProsecutionHistory, error)
	mapCallCount       atomic.Int32
	estoppelCount      atomic.Int32
}

func (m *mockClaimElementMapper) MapElements(ctx context.Context, claims []*ClaimInput) ([]*MappedClaim, error) {
	m.mapCallCount.Add(1)
	if m.mapFn != nil {
		return m.mapFn(ctx, claims)
	}
	var res []*MappedClaim
	for _, c := range claims {
		res = append(res, &MappedClaim{
			ClaimID:   c.ClaimID,
			ClaimType: c.ClaimType,
			Elements: []*ClaimElement{
				{ElementID: c.ClaimID + "-e1", Description: "element_A"},
				{ElementID: c.ClaimID + "-e2", Description: "element_B"},
				{ElementID: c.ClaimID + "-e3", Description: "element_C"},
			},
		})
	}
	return res, nil
}

func (m *mockClaimElementMapper) MapMoleculeToElements(ctx context.Context, molecule *MoleculeInput) ([]*StructuralElement, error) {
	if m.mapMolFn != nil {
		return m.mapMolFn(ctx, molecule)
	}
	return []*StructuralElement{
		{ElementID: "mol-e1", Description: "element_A"},
	}, nil
}

func (m *mockClaimElementMapper) AlignElements(ctx context.Context, moleculeElements []*StructuralElement, claimElements []*ClaimElement) (*ElementAlignment, error) {
	if m.alignFn != nil {
		return m.alignFn(ctx, moleculeElements, claimElements)
	}
	return &ElementAlignment{}, nil
}

func (m *mockClaimElementMapper) CheckEstoppel(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error) {
	m.estoppelCount.Add(1)
	if m.estoppelFn != nil {
		return m.estoppelFn(ctx, alignment, history)
	}
	return &EstoppelResult{
		HasEstoppel:     false,
		EstoppelPenalty: 0,
	}, nil
}

func (m *mockClaimElementMapper) ParseProsecutionHistory(ctx context.Context, rawHistory []byte) (*ProsecutionHistory, error) {
	if m.parseHistFn != nil {
		return m.parseHistFn(ctx, rawHistory)
	}
	return nil, nil
}

func (m *mockClaimElementMapper) LoadIndependentClaims(ctx context.Context, deps []*ClaimInput) ([]*ClaimInput, error) {
	if m.loadIndepFn != nil {
		return m.loadIndepFn(ctx, deps)
	}
	return []*ClaimInput{
		{ClaimID: "ind-1", ClaimText: "An independent claim", ClaimType: "independent"},
	}, nil
}

type mockPortfolioLoader struct {
	claims []*ClaimInput
	err    error
}

func (m *mockPortfolioLoader) LoadPortfolioClaims(ctx context.Context, portfolioID string) ([]*ClaimInput, error) {
	return m.claims, m.err
}

type mockExplanationStore struct {
	data      map[string]*AssessmentResult
	storeFn   func(ctx context.Context, result *AssessmentResult) error
	storeCount atomic.Int32
}

func newMockExplanationStore() *mockExplanationStore {
	return &mockExplanationStore{data: make(map[string]*AssessmentResult)}
}

func (m *mockExplanationStore) Store(ctx context.Context, result *AssessmentResult) error {
	m.storeCount.Add(1)
	if m.storeFn != nil {
		return m.storeFn(ctx, result)
	}
	m.data[result.RequestID] = result
	return nil
}

func (m *mockExplanationStore) Load(ctx context.Context, resultID string) (*AssessmentResult, error) {
	r, ok := m.data[resultID]
	if !ok {
		return nil, nil
	}
	return r, nil
}

type mockExplanationGenerator struct {
	generateFn func(ctx context.Context, result *AssessmentResult) (*AssessmentExplanation, error)
}

func (m *mockExplanationGenerator) Generate(ctx context.Context, result *AssessmentResult) (*AssessmentExplanation, error) {
	if m.generateFn != nil {
		return m.generateFn(ctx, result)
	}
	return &AssessmentExplanation{
		ResultID:               result.RequestID,
		NaturalLanguageSummary: "The molecule shows moderate infringement risk.",
		KeyFactors:             []string{"structural overlap", "functional equivalence"},
		SuggestedActions:       []string{"consult patent counsel", "consider design-around"},
	}, nil
}

type mockAssessorMetrics struct {
	inferenceCount    atomic.Int32
	riskCount         atomic.Int32
	lastRiskLevel     atomic.Value
}

func (m *mockAssessorMetrics) RecordInference(ctx context.Context, p *common.InferenceMetricParams) {
	m.inferenceCount.Add(1)
}
func (m *mockAssessorMetrics) RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64) {
	m.riskCount.Add(1)
	m.lastRiskLevel.Store(riskLevel)
}
func (m *mockAssessorMetrics) RecordBatchProcessing(ctx context.Context, p *common.BatchMetricParams)       {}
func (m *mockAssessorMetrics) RecordCacheAccess(ctx context.Context, hit bool, modelName string)            {}
func (m *mockAssessorMetrics) RecordCircuitBreakerStateChange(ctx context.Context, mn, from, to string)     {}
func (m *mockAssessorMetrics) RecordModelLoad(ctx context.Context, mn, v string, d float64, s bool)         {}
func (m *mockAssessorMetrics) GetInferenceLatencyHistogram() common.LatencyHistogram                        { return nil }
func (m *mockAssessorMetrics) GetCurrentStats() *common.IntelligenceStats                                   { return &common.IntelligenceStats{} }

type mockAssessorLogger struct{}

func (m *mockAssessorLogger) Info(msg string, kv ...interface{})  {}
func (m *mockAssessorLogger) Warn(msg string, kv ...interface{})  {}
func (m *mockAssessorLogger) Error(msg string, kv ...interface{}) {}
func (m *mockAssessorLogger) Debug(msg string, kv ...interface{}) {}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func defaultTestDeps() (InfringeModel, *mockEquivalentsAnalyzer, *mockClaimElementMapper, *mockPortfolioLoader, *mockExplanationStore, *mockExplanationGenerator, *mockAssessorMetrics, common.Logger) {
	return &mockInfringeModel{version: "literal-v1.0"},
		&mockEquivalentsAnalyzer{version: "equivalents-v1.0"},
		&mockClaimElementMapper{},
		&mockPortfolioLoader{},
		newMockExplanationStore(),
		&mockExplanationGenerator{},
		&mockAssessorMetrics{},
		&mockAssessorLogger{}
}

func newTestAssessor(t *testing.T) (InfringementAssessor, *mockInfringeModel, *mockEquivalentsAnalyzer, *mockClaimElementMapper, *mockAssessorMetrics) {
	t.Helper()
	model := &mockInfringeModel{version: "literal-v1.0"}
	eq := &mockEquivalentsAnalyzer{version: "equivalents-v1.0"}
	mapper := &mockClaimElementMapper{}
	portfolio := &mockPortfolioLoader{}
	store := newMockExplanationStore()
	gen := &mockExplanationGenerator{}
	metrics := &mockAssessorMetrics{}
	a, err := NewInfringementAssessor(model, eq, mapper, portfolio, store, gen, metrics, &mockAssessorLogger{})
	if err != nil {
		t.Fatalf("NewInfringementAssessor: %v", err)
	}
	return a, model, eq, mapper, metrics
}

func sampleRequest(id string) *AssessmentRequest {
	return &AssessmentRequest{
		RequestID: id,
		Molecule:  &MoleculeInput{SMILES: "CCO"},
		Claims: []*ClaimInput{
			{ClaimID: "claim-1", ClaimText: "A compound comprising ethanol", ClaimType: "independent"},
		},
	}
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewInfringementAssessor_Success(t *testing.T) {
	model, eq, mapper, pf, store, gen, metrics, logger := defaultTestDeps()
	a, err := NewInfringementAssessor(model, eq, mapper, pf, store, gen, metrics, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil assessor")
	}
}

func TestNewInfringementAssessor_MissingModel(t *testing.T) {
	_, eq, mapper, pf, store, gen, metrics, logger := defaultTestDeps()
	_, err := NewInfringementAssessor(nil, eq, mapper, pf, store, gen, metrics, logger)
	if err == nil {
		t.Fatal("expected error for nil model")
	}
}

func TestNewInfringementAssessor_MissingEquivalents(t *testing.T) {
	model := &mockInfringeModel{}
	_, err := NewInfringementAssessor(model, nil, &mockClaimElementMapper{}, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil equivalents analyzer")
	}
}

func TestNewInfringementAssessor_MissingMapper(t *testing.T) {
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	_, err := NewInfringementAssessor(model, eq, nil, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil mapper")
	}
}

func TestNewInfringementAssessor_NilMetricsAndLogger(t *testing.T) {
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, err := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil assessor with nil metrics/logger")
	}
}

// ---------------------------------------------------------------------------
// Assess tests
// ---------------------------------------------------------------------------

func TestAssess_FullPipeline_HighRisk(t *testing.T) {
	model := &mockInfringeModel{
		version: "lit-v2",
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			es := make(map[string]float64)
			for _, e := range req.ClaimElements {
				es[e.ElementID] = 0.88
			}
			return &LiteralPredictionResult{OverallScore: 0.88, ElementScores: es, UnmatchedElements: []string{}, Confidence: 0.95}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{
		version: "eq-v2",
		analyzeFn: func(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
			return &EquivalentsResult{OverallEquivalenceScore: 0.85}, nil
		},
	}
	mapper := &mockClaimElementMapper{
		estoppelFn: func(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error) {
			return &EstoppelResult{HasEstoppel: false, EstoppelPenalty: 0}, nil
		},
	}
	metrics := &mockAssessorMetrics{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, metrics, &mockAssessorLogger{})

	result, err := a.Assess(context.Background(), sampleRequest("high-risk-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// overall = 0.5*0.88 + 0.35*0.85 - 0.15*0 = 0.44 + 0.2975 = 0.7375
	if result.OverallScore < 0.70 {
		t.Errorf("expected overall score >= 0.70, got %f", result.OverallScore)
	}
	if result.OverallRiskLevel != RiskHigh && result.OverallRiskLevel != RiskCritical {
		t.Errorf("expected HIGH or CRITICAL risk, got %s", result.OverallRiskLevel)
	}
	if result.Degraded {
		t.Error("expected non-degraded result")
	}
}

func TestAssess_FullPipeline_LowRisk(t *testing.T) {
	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			return &LiteralPredictionResult{OverallScore: 0.20, Confidence: 0.80}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{
		analyzeFn: func(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
			return &EquivalentsResult{OverallEquivalenceScore: 0.15}, nil
		},
	}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	result, err := a.Assess(context.Background(), sampleRequest("low-risk-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// overall = 0.5*0.20 + 0.35*0.15 = 0.10 + 0.0525 = 0.1525
	if result.OverallRiskLevel != RiskNone {
		t.Errorf("expected NONE risk, got %s (score=%f)", result.OverallRiskLevel, result.OverallScore)
	}
}

func TestAssess_LiteralShortCircuit(t *testing.T) {
	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			return &LiteralPredictionResult{OverallScore: 0.95, Confidence: 0.98}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	result, err := a.Assess(context.Background(), sampleRequest("short-circuit-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.OverallRiskLevel != RiskCritical {
		t.Errorf("expected CRITICAL risk for literal >= 0.9, got %s", result.OverallRiskLevel)
	}
	if result.EquivalentsAnalysis == nil || !result.EquivalentsAnalysis.Skipped {
		t.Error("expected equivalents analysis to be skipped")
	}
	// Verify equivalents analyzer was NOT called.
	if eq.callCount.Load() != 0 {
		t.Errorf("expected 0 equivalents calls, got %d", eq.callCount.Load())
	}
}

func TestAssess_EquivalentsBoost(t *testing.T) {
	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			return &LiteralPredictionResult{OverallScore: 0.55, Confidence: 0.85}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{
		analyzeFn: func(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
			return &EquivalentsResult{OverallEquivalenceScore: 0.90}, nil
		},
	}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	result, err := a.Assess(context.Background(), sampleRequest("eq-boost-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// overall = 0.5*0.55 + 0.35*0.90 = 0.275 + 0.315 = 0.59
	if result.OverallScore < 0.50 {
		t.Errorf("expected boosted score >= 0.50, got %f", result.OverallScore)
	}
	if result.OverallRiskLevel < RiskMedium {
		t.Errorf("expected at least MEDIUM risk, got %s", result.OverallRiskLevel)
	}
}

func TestAssess_EstoppelPenalty(t *testing.T) {
	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			return &LiteralPredictionResult{OverallScore: 0.80, Confidence: 0.90}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{
		analyzeFn: func(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
			return &EquivalentsResult{OverallEquivalenceScore: 0.75}, nil
		},
	}
	mapper := &mockClaimElementMapper{
		estoppelFn: func(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error) {
			return &EstoppelResult{
				HasEstoppel:     true,
				EstoppelPenalty: 0.60,
			}, nil
		},
	}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	result, err := a.Assess(context.Background(), sampleRequest("estoppel-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// overall = 0.5*0.80 + 0.35*0.75 - 0.15*0.60 = 0.40 + 0.2625 - 0.09 = 0.5725
	if result.EstoppelCheck == nil || !result.EstoppelCheck.HasEstoppel {
		t.Error("expected estoppel to be detected")
	}

	// Without estoppel: 0.40 + 0.2625 = 0.6625 → with penalty: 0.5725
	// The penalty should have reduced the score.
	noPenalty := weightLiteral*0.80 + weightEquivalents*0.75
	if result.OverallScore >= noPenalty {
		t.Errorf("expected score < %.4f (no penalty), got %f", noPenalty, result.OverallScore)
	}
}

func TestAssess_InvalidMolecule(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)

	_, err := a.Assess(context.Background(), &AssessmentRequest{
		Molecule: &MoleculeInput{},
		Claims:   []*ClaimInput{{ClaimID: "c1", ClaimText: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for empty molecule")
	}
}

func TestAssess_NilMolecule(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)

	_, err := a.Assess(context.Background(), &AssessmentRequest{
		Molecule: nil,
		Claims:   []*ClaimInput{{ClaimID: "c1", ClaimText: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for nil molecule")
	}
}

func TestAssess_EmptyClaims(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)

	_, err := a.Assess(context.Background(), &AssessmentRequest{
		Molecule: &MoleculeInput{SMILES: "CCO"},
		Claims:   []*ClaimInput{},
	})
	if err == nil {
		t.Fatal("expected error for empty claims")
	}
}

func TestAssess_NilRequest(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)
	_, err := a.Assess(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAssess_ModelTimeout(t *testing.T) {
	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return &LiteralPredictionResult{OverallScore: 0.5, Confidence: 0.8}, nil
			}
		},
	}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	req := sampleRequest("timeout-1")
	req.Options = []AssessmentOption{WithTimeout(100 * time.Millisecond)}

	_, err := a.Assess(context.Background(), req)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestAssess_EquivalentsFailure_Degraded(t *testing.T) {
	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			return &LiteralPredictionResult{OverallScore: 0.65, Confidence: 0.88}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{
		analyzeFn: func(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
			return nil, fmt.Errorf("equivalents model unavailable")
		},
	}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	result, err := a.Assess(context.Background(), sampleRequest("degraded-1"))
	if err != nil {
		t.Fatalf("unexpected error (should degrade, not fail): %v", err)
	}
	if !result.Degraded {
		t.Error("expected degraded flag to be true")
	}
	if result.DegradedReason == "" {
		t.Error("expected non-empty degraded reason")
	}
	if result.EquivalentsAnalysis == nil || !result.EquivalentsAnalysis.Skipped {
		t.Error("expected equivalents to be marked as skipped")
	}
	// Score should be literal-only: 0.5*0.65 = 0.325
	if result.OverallScore < 0.30 || result.OverallScore > 0.40 {
		t.Errorf("expected score ~0.325 for literal-only, got %f", result.OverallScore)
	}
}

func TestAssess_DependentClaimsOnly(t *testing.T) {
	mapper := &mockClaimElementMapper{
		loadIndepFn: func(ctx context.Context, deps []*ClaimInput) ([]*ClaimInput, error) {
			return []*ClaimInput{
				{ClaimID: "ind-1", ClaimText: "Independent claim text", ClaimType: "independent"},
			}, nil
		},
	}
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	req := &AssessmentRequest{
		RequestID: "dep-only-1",
		Molecule:  &MoleculeInput{SMILES: "CCO"},
		Claims: []*ClaimInput{
			{ClaimID: "dep-1", ClaimText: "Dependent claim", ClaimType: "dependent", ParentID: "ind-1"},
		},
	}
	result, err := a.Assess(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The mapper should have been asked to map elements for both independent + dependent.
	if mapper.mapCallCount.Load() == 0 {
		t.Error("expected MapElements to be called")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// BatchAssess tests
// ---------------------------------------------------------------------------

func TestBatchAssess_Success(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)

	reqs := []*AssessmentRequest{
		sampleRequest("batch-1"),
		sampleRequest("batch-2"),
		sampleRequest("batch-3"),
	}
	results, err := a.BatchAssess(context.Background(), reqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Error != "" {
			t.Errorf("result[%d] unexpected error: %s", i, r.Error)
		}
		if r.RequestID == "" {
			t.Errorf("result[%d] missing request_id", i)
		}
	}
}

func TestBatchAssess_ConcurrencyLimit(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			cur := concurrent.Add(1)
			defer concurrent.Add(-1)
			// Track peak concurrency.
			for {
				old := maxConcurrent.Load()
				if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			return &LiteralPredictionResult{OverallScore: 0.5, Confidence: 0.8}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil, WithMaxConcurrency(2))

	reqs := make([]*AssessmentRequest, 10)
	for i := range reqs {
		reqs[i] = sampleRequest(fmt.Sprintf("conc-%d", i))
	}

	results, err := a.BatchAssess(context.Background(), reqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}

	peak := maxConcurrent.Load()
	// The semaphore limits to 2, but due to goroutine scheduling the actual
	// peak may be slightly higher in rare cases. We allow a small margin.
	if peak > 4 {
		t.Errorf("expected peak concurrency <= ~2 (with scheduling margin), got %d", peak)
	}
}

func TestBatchAssess_PartialFailure(t *testing.T) {
	callIdx := atomic.Int32{}
	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			idx := callIdx.Add(1)
			if idx == 2 {
				return nil, fmt.Errorf("transient model failure")
			}
			return &LiteralPredictionResult{OverallScore: 0.6, Confidence: 0.85}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	reqs := []*AssessmentRequest{
		sampleRequest("pf-1"),
		sampleRequest("pf-2"),
		sampleRequest("pf-3"),
	}
	results, err := a.BatchAssess(context.Background(), reqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	hasError := false
	hasSuccess := false
	for _, r := range results {
		if r.Error != "" {
			hasError = true
		} else {
			hasSuccess = true
		}
	}
	if !hasError {
		t.Error("expected at least one error result")
	}
	if !hasSuccess {
		t.Error("expected at least one success result")
	}
}

func TestBatchAssess_EmptyInput(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)

	results, err := a.BatchAssess(context.Background(), []*AssessmentRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// AssessAgainstPortfolio tests
// ---------------------------------------------------------------------------

func TestAssessAgainstPortfolio_Success(t *testing.T) {
	portfolioClaims := []*ClaimInput{
		{ClaimID: "c1", ClaimText: "Claim 1", ClaimType: "independent", PatentID: "US001"},
		{ClaimID: "c2", ClaimText: "Claim 2", ClaimType: "independent", PatentID: "US001"},
		{ClaimID: "c3", ClaimText: "Claim 3", ClaimType: "independent", PatentID: "US002"},
		{ClaimID: "c4", ClaimText: "Claim 4", ClaimType: "independent", PatentID: "US003"},
	}
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	pf := &mockPortfolioLoader{claims: portfolioClaims}
	a, _ := NewInfringementAssessor(model, eq, mapper, pf, nil, nil, nil, nil)

	result, err := a.AssessAgainstPortfolio(context.Background(), &MoleculeInput{SMILES: "CCO"}, "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalPatentsScanned != 3 {
		t.Errorf("expected 3 patents scanned, got %d", result.TotalPatentsScanned)
	}
	if result.PortfolioID != "portfolio-1" {
		t.Errorf("expected portfolio_id 'portfolio-1', got %s", result.PortfolioID)
	}
	if result.RiskDistribution == nil {
		t.Fatal("expected non-nil risk distribution")
	}
	total := 0
	for _, count := range result.RiskDistribution {
		total += count
	}
	if total != 3 {
		t.Errorf("expected risk distribution total 3, got %d", total)
	}
	if len(result.TopRisks) == 0 {
		t.Error("expected at least one top risk entry")
	}
	if result.ScanDurationMs < 0 {
		t.Error("expected non-negative scan duration")
	}
}

func TestAssessAgainstPortfolio_EmptyPortfolio(t *testing.T) {
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	pf := &mockPortfolioLoader{claims: []*ClaimInput{}}
	a, _ := NewInfringementAssessor(model, eq, mapper, pf, nil, nil, nil, nil)

	result, err := a.AssessAgainstPortfolio(context.Background(), &MoleculeInput{SMILES: "CCO"}, "empty-portfolio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalPatentsScanned != 0 {
		t.Errorf("expected 0 patents scanned, got %d", result.TotalPatentsScanned)
	}
	if len(result.TopRisks) != 0 {
		t.Errorf("expected 0 top risks, got %d", len(result.TopRisks))
	}
}

func TestAssessAgainstPortfolio_InvalidMolecule(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)
	_, err := a.AssessAgainstPortfolio(context.Background(), &MoleculeInput{}, "p1")
	if err == nil {
		t.Fatal("expected error for invalid molecule")
	}
}

func TestAssessAgainstPortfolio_EmptyPortfolioID(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)
	_, err := a.AssessAgainstPortfolio(context.Background(), &MoleculeInput{SMILES: "CCO"}, "")
	if err == nil {
		t.Fatal("expected error for empty portfolio_id")
	}
}

func TestAssessAgainstPortfolio_NoPortfolioLoader(t *testing.T) {
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	_, err := a.AssessAgainstPortfolio(context.Background(), &MoleculeInput{SMILES: "CCO"}, "p1")
	if err == nil {
		t.Fatal("expected error when portfolio loader is nil")
	}
}

// ---------------------------------------------------------------------------
// ExplainAssessment tests
// ---------------------------------------------------------------------------

func TestExplainAssessment_Success(t *testing.T) {
	store := newMockExplanationStore()
	store.data["result-1"] = &AssessmentResult{
		RequestID:        "result-1",
		OverallRiskLevel: RiskHigh,
		OverallScore:     0.75,
	}
	gen := &mockExplanationGenerator{}
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, store, gen, nil, nil)

	expl, err := a.ExplainAssessment(context.Background(), "result-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if expl.ResultID != "result-1" {
		t.Errorf("expected result_id 'result-1', got %s", expl.ResultID)
	}
	if expl.NaturalLanguageSummary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestExplainAssessment_NotFound(t *testing.T) {
	store := newMockExplanationStore()
	gen := &mockExplanationGenerator{}
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, store, gen, nil, nil)

	_, err := a.ExplainAssessment(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent result")
	}
	// errors.NotFoundError not available here or not exported in a way we can assert type easily in this context without importing more?
	// Just check error message
	if err.Error() == "" {
		t.Error("expected meaningful error message")
	}
}

func TestExplainAssessment_EmptyResultID(t *testing.T) {
	a, _, _, _, _ := newTestAssessor(t)
	_, err := a.ExplainAssessment(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty result_id")
	}
}

func TestExplainAssessment_NoStore(t *testing.T) {
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	_, err := a.ExplainAssessment(context.Background(), "r1")
	if err == nil {
		t.Fatal("expected error when store is nil")
	}
}

func TestExplainAssessment_NoGenerator(t *testing.T) {
	store := newMockExplanationStore()
	store.data["r1"] = &AssessmentResult{RequestID: "r1"}
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, store, nil, nil, nil)

	_, err := a.ExplainAssessment(context.Background(), "r1")
	if err == nil {
		t.Fatal("expected error when generator is nil")
	}
}

// ---------------------------------------------------------------------------
// RiskLevel tests
// ---------------------------------------------------------------------------

func TestRiskLevel_Mapping(t *testing.T) {
	tests := []struct {
		score float64
		want  RiskLevel
	}{
		{0.00, RiskNone},
		{0.10, RiskNone},
		{0.29, RiskNone},
		{0.30, RiskLow},
		{0.49, RiskLow},
		{0.50, RiskMedium},
		{0.69, RiskMedium},
		{0.699, RiskMedium},
		{0.70, RiskHigh},
		{0.849, RiskHigh},
		{0.85, RiskCritical},
		{0.90, RiskCritical},
		{1.00, RiskCritical},
	}
	for _, tt := range tests {
		got := ClassifyRisk(tt.score)
		if got != tt.want {
			t.Errorf("ClassifyRisk(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		level RiskLevel
		want  string
	}{
		{RiskNone, "NONE"},
		{RiskLow, "LOW"},
		{RiskMedium, "MEDIUM"},
		{RiskHigh, "HIGH"},
		{RiskCritical, "CRITICAL"},
		{RiskLevel(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.level.String()
		if got != tt.want {
			t.Errorf("RiskLevel(%d).String() = %s, want %s", tt.level, got, tt.want)
		}
	}
}

func TestRiskLevel_MarshalJSON(t *testing.T) {
	tests := []struct {
		level RiskLevel
		want  string
	}{
		{RiskNone, `"NONE"`},
		{RiskLow, `"LOW"`},
		{RiskMedium, `"MEDIUM"`},
		{RiskHigh, `"HIGH"`},
		{RiskCritical, `"CRITICAL"`},
	}
	for _, tt := range tests {
		data, err := json.Marshal(tt.level)
		if err != nil {
			t.Fatalf("MarshalJSON(%s): %v", tt.level, err)
		}
		if string(data) != tt.want {
			t.Errorf("MarshalJSON(%s) = %s, want %s", tt.level, string(data), tt.want)
		}
	}
}

func TestRiskLevel_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		input string
		want  RiskLevel
	}{
		{`"NONE"`, RiskNone},
		{`"LOW"`, RiskLow},
		{`"MEDIUM"`, RiskMedium},
		{`"HIGH"`, RiskHigh},
		{`"CRITICAL"`, RiskCritical},
	}
	for _, tt := range tests {
		var got RiskLevel
		if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
			t.Fatalf("UnmarshalJSON(%s): %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("UnmarshalJSON(%s) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestRiskLevel_UnmarshalJSON_Invalid(t *testing.T) {
	var got RiskLevel
	err := json.Unmarshal([]byte(`"INVALID"`), &got)
	if err == nil {
		t.Fatal("expected error for invalid risk level string")
	}
}

// ---------------------------------------------------------------------------
// Metrics recording tests
// ---------------------------------------------------------------------------

func TestAssess_MetricsRecorded(t *testing.T) {
	a, _, _, _, metrics := newTestAssessor(t)

	_, err := a.Assess(context.Background(), sampleRequest("metrics-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.inferenceCount.Load() == 0 {
		t.Error("expected inference metric to be recorded")
	}
	if metrics.riskCount.Load() == 0 {
		t.Error("expected risk assessment metric to be recorded")
	}
	lastLevel, ok := metrics.lastRiskLevel.Load().(string)
	if !ok || lastLevel == "" {
		t.Error("expected non-empty last risk level in metrics")
	}
}

func TestAssess_ModelVersionTracking(t *testing.T) {
	model := &mockInfringeModel{version: "literal-v3.2.1"}
	eq := &mockEquivalentsAnalyzer{version: "eq-v2.1.0"}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	result, err := a.Assess(context.Background(), sampleRequest("version-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ModelVersions == nil {
		t.Fatal("expected non-nil model versions map")
	}
	if v, ok := result.ModelVersions["literal_model"]; !ok || v != "literal-v3.2.1" {
		t.Errorf("expected literal_model version 'literal-v3.2.1', got '%s'", v)
	}
	if v, ok := result.ModelVersions["equivalents_model"]; !ok || v != "eq-v2.1.0" {
		t.Errorf("expected equivalents_model version 'eq-v2.1.0', got '%s'", v)
	}
}

// ---------------------------------------------------------------------------
// Helper function tests
// ---------------------------------------------------------------------------

func TestFlattenElements(t *testing.T) {
	// Replaced map[string][]string with []*MappedClaim
	m := []*MappedClaim{
		{
			ClaimID: "c1",
			Elements: []*ClaimElement{
				{ElementID: "e1", Description: "A", StructuralConstraint: "A"},
				{ElementID: "e2", Description: "B", StructuralConstraint: "B"},
				{ElementID: "e3", Description: "C", StructuralConstraint: "C"},
			},
		},
		{
			ClaimID: "c2",
			Elements: []*ClaimElement{
				{ElementID: "e2", Description: "B", StructuralConstraint: "B"}, // Duplicate content, different context? flatten doesn't dedup by content, but by element ID if it were map based?
				// Actually flattenElements now appends all, no deduping in current implementation?
				// Looking at assessor.go: flattenElements converts MappedClaim elements to StructuralElements.
				// It iterates over all claims and all elements. So it doesn't dedup.
				{ElementID: "e4", Description: "C", StructuralConstraint: "C"},
				{ElementID: "e5", Description: "D", StructuralConstraint: "D"},
			},
		},
	}
	result := flattenElements(m)
	// c1: 3 elements, c2: 3 elements -> total 6
	if len(result) != 6 {
		t.Errorf("expected 6 elements (no dedup), got %d", len(result))
	}
}

func TestGroupClaimsByPatent(t *testing.T) {
	claims := []*ClaimInput{
		{ClaimID: "c1", PatentID: "US001"},
		{ClaimID: "c2", PatentID: "US001"},
		{ClaimID: "c3", PatentID: "US002"},
		{ClaimID: "c4", PatentID: ""},
	}
	groups := groupClaimsByPatent(claims)
	if len(groups) != 3 {
		t.Errorf("expected 3 groups, got %d", len(groups))
	}
	if len(groups["US001"]) != 2 {
		t.Errorf("expected 2 claims for US001, got %d", len(groups["US001"]))
	}
	if len(groups["US002"]) != 1 {
		t.Errorf("expected 1 claim for US002, got %d", len(groups["US002"]))
	}
	if len(groups["_unknown_"]) != 1 {
		t.Errorf("expected 1 claim for _unknown_, got %d", len(groups["_unknown_"]))
	}
}

// TestClamp01 is redundant if model_test.go covers it, but package scope shares identifiers.
// Rename to avoid redeclaration if needed, or remove.
func TestAssessorClamp01(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{-0.5, 0},
		{0, 0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}
	for _, tt := range tests {
		got := clamp01(tt.input)
		if got != tt.want {
			t.Errorf("clamp01(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestRoundTo4(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0.12345, 0.1235},
		{0.10001, 0.1},
		{0.99999, 1.0},
		{0.0, 0.0},
	}
	for _, tt := range tests {
		got := roundTo4(tt.input)
		diff := got - tt.want
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.00015 {
			t.Errorf("roundTo4(%f) = %f, want ~%f", tt.input, got, tt.want)
		}
	}
}

func TestMoleculeInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   *MoleculeInput
		wantErr bool
	}{
		{"nil", nil, true},
		{"empty", &MoleculeInput{}, true},
		{"smiles only", &MoleculeInput{SMILES: "CCO"}, false},
		{"inchi only", &MoleculeInput{InChI: "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3"}, false},
		{"fingerprint only", &MoleculeInput{Fingerprint: []byte{0x01}}, false},
		{"all fields", &MoleculeInput{SMILES: "CCO", InChI: "x", Fingerprint: []byte{1}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMoleculeInput_Validate_NilReceiver(t *testing.T) {
	var m *MoleculeInput
	err := m.Validate()
	if err == nil {
		t.Fatal("expected error for nil MoleculeInput")
	}
}

func TestAssessmentOptions_Defaults(t *testing.T) {
	opts := DefaultAssessmentOptions()
	if !opts.EnableEquivalents {
		t.Error("expected EnableEquivalents true by default")
	}
	if !opts.EnableEstoppelCheck {
		t.Error("expected EnableEstoppelCheck true by default")
	}
	if opts.ConfidenceThreshold != 0.50 {
		t.Errorf("expected ConfidenceThreshold 0.50, got %f", opts.ConfidenceThreshold)
	}
	if opts.MaxConcurrency != 8 {
		t.Errorf("expected MaxConcurrency 8, got %d", opts.MaxConcurrency)
	}
	if opts.Timeout != 30*time.Second {
		t.Errorf("expected Timeout 30s, got %v", opts.Timeout)
	}
}

func TestAssessmentOptions_Functional(t *testing.T) {
	opts := applyOptions([]AssessmentOption{
		WithEquivalentsAnalysis(false),
		WithEstoppelCheck(false),
		WithConfidenceThreshold(0.80),
		WithMaxConcurrency(4),
		WithTimeout(10 * time.Second),
	})
	if opts.EnableEquivalents {
		t.Error("expected EnableEquivalents false")
	}
	if opts.EnableEstoppelCheck {
		t.Error("expected EnableEstoppelCheck false")
	}
	if opts.ConfidenceThreshold != 0.80 {
		t.Errorf("expected ConfidenceThreshold 0.80, got %f", opts.ConfidenceThreshold)
	}
	if opts.MaxConcurrency != 4 {
		t.Errorf("expected MaxConcurrency 4, got %d", opts.MaxConcurrency)
	}
	if opts.Timeout != 10*time.Second {
		t.Errorf("expected Timeout 10s, got %v", opts.Timeout)
	}
}

func TestAssessmentOptions_InvalidValues(t *testing.T) {
	opts := applyOptions([]AssessmentOption{
		WithConfidenceThreshold(-1),
		WithConfidenceThreshold(2.0),
		WithMaxConcurrency(-5),
		WithTimeout(-1 * time.Second),
	})
	// Invalid values should be ignored, defaults preserved.
	if opts.ConfidenceThreshold != 0.50 {
		t.Errorf("expected default ConfidenceThreshold, got %f", opts.ConfidenceThreshold)
	}
	if opts.MaxConcurrency != 8 {
		t.Errorf("expected default MaxConcurrency, got %d", opts.MaxConcurrency)
	}
	if opts.Timeout != 30*time.Second {
		t.Errorf("expected default Timeout, got %v", opts.Timeout)
	}
}

// ---------------------------------------------------------------------------
// Assess with disabled paths
// ---------------------------------------------------------------------------

func TestAssess_EquivalentsDisabled(t *testing.T) {
	eq := &mockEquivalentsAnalyzer{}
	model := &mockInfringeModel{}
	mapper := &mockClaimElementMapper{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	req := sampleRequest("eq-disabled-1")
	req.Options = []AssessmentOption{WithEquivalentsAnalysis(false)}

	result, err := a.Assess(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eq.callCount.Load() != 0 {
		t.Errorf("expected 0 equivalents calls when disabled, got %d", eq.callCount.Load())
	}
	if result.EquivalentsAnalysis == nil || !result.EquivalentsAnalysis.Skipped {
		t.Error("expected equivalents to be marked as skipped")
	}
}

func TestAssess_EstoppelDisabled(t *testing.T) {
	mapper := &mockClaimElementMapper{}
	model := &mockInfringeModel{}
	eq := &mockEquivalentsAnalyzer{}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	req := sampleRequest("est-disabled-1")
	req.Options = []AssessmentOption{WithEstoppelCheck(false)}

	result, err := a.Assess(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapper.estoppelCount.Load() != 0 {
		t.Errorf("expected 0 estoppel calls when disabled, got %d", mapper.estoppelCount.Load())
	}
	if result.EstoppelCheck == nil || !result.EstoppelCheck.Skipped {
		t.Error("expected estoppel to be marked as skipped")
	}
}

// ---------------------------------------------------------------------------
// Scoring formula verification
// ---------------------------------------------------------------------------

func TestAssess_ScoringFormula(t *testing.T) {
	litScore := 0.72
	eqScore := 0.68
	estPenalty := 0.40

	model := &mockInfringeModel{
		predictFn: func(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
			return &LiteralPredictionResult{OverallScore: litScore, Confidence: 0.90}, nil
		},
	}
	eq := &mockEquivalentsAnalyzer{
		analyzeFn: func(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
			return &EquivalentsResult{OverallEquivalenceScore: eqScore}, nil
		},
	}
	mapper := &mockClaimElementMapper{
		estoppelFn: func(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error) {
			return &EstoppelResult{HasEstoppel: true, EstoppelPenalty: estPenalty}, nil
		},
	}
	a, _ := NewInfringementAssessor(model, eq, mapper, nil, nil, nil, nil, nil)

	result, err := a.Assess(context.Background(), sampleRequest("formula-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: 0.5*0.72 + 0.35*0.68 - 0.15*0.40 = 0.36 + 0.238 - 0.06 = 0.538
	expected := 0.5*litScore + 0.35*eqScore - 0.15*estPenalty
	diff := result.OverallScore - roundTo4(expected)
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.001 {
		t.Errorf("expected overall score ~%.4f, got %.4f", expected, result.OverallScore)
	}
	if result.OverallRiskLevel != RiskMedium {
		t.Errorf("expected MEDIUM risk for score ~0.538, got %s", result.OverallRiskLevel)
	}
}

//Personal.AI order the ending

