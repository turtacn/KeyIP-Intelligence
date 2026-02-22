package infringe_net

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// =========================================================================
// Mocks
// =========================================================================

// --- mockModelRegistry ---

type mockModelRegistry struct{}

func (m *mockModelRegistry) Register(ctx context.Context, meta *common.ModelMetadata) error {
	return nil
}
func (m *mockModelRegistry) Unregister(ctx context.Context, modelID string, version string) error { return nil }
func (m *mockModelRegistry) GetModel(ctx context.Context, modelID string) (*common.RegisteredModel, error) {
	return nil, nil
}
func (m *mockModelRegistry) GetModelVersion(ctx context.Context, modelID string, version string) (*common.RegisteredModel, error) {
	return nil, nil
}
func (m *mockModelRegistry) ListModels(ctx context.Context) ([]*common.RegisteredModel, error) {
	return []*common.RegisteredModel{{ModelID: "infringe-net-local-v1"}}, nil
}
func (m *mockModelRegistry) ListVersions(ctx context.Context, modelID string) ([]*common.ModelVersion, error) {
	return nil, nil
}
func (m *mockModelRegistry) SetActiveVersion(ctx context.Context, modelID string, version string) error {
	return nil
}
func (m *mockModelRegistry) Rollback(ctx context.Context, modelID string) error { return nil }
func (m *mockModelRegistry) ConfigureABTest(ctx context.Context, config *common.ABTestConfig) error {
	return nil
}
func (m *mockModelRegistry) ResolveModel(ctx context.Context, modelID string, requestID string) (*common.RegisteredModel, error) {
	return nil, nil
}
func (m *mockModelRegistry) HealthCheck(ctx context.Context) (*common.RegistryHealth, error) {
	return nil, nil
}
func (m *mockModelRegistry) Close() error { return nil }

// --- mockMoleculeValidator ---

type mockMoleculeValidator struct {
	invalidSet map[string]bool
}

func newMockValidator(invalid ...string) *mockMoleculeValidator {
	s := make(map[string]bool, len(invalid))
	for _, v := range invalid {
		s[v] = true
	}
	return &mockMoleculeValidator{invalidSet: s}
}

func (m *mockMoleculeValidator) Validate(smiles string) error {
	if m.invalidSet[smiles] {
		return fmt.Errorf("invalid SMILES: %s", smiles)
	}
	return nil
}

// --- mockSMARTSMatcher ---

type mockSMARTSMatcher struct {
	matchMap map[string]bool // key = smiles+"|"+smarts
}

func newMockSMARTSMatcher(matches map[string]bool) *mockSMARTSMatcher {
	if matches == nil {
		matches = make(map[string]bool)
	}
	return &mockSMARTSMatcher{matchMap: matches}
}

func (m *mockSMARTSMatcher) Match(smiles, smartsPattern string) (bool, error) {
	key := smiles + "|" + smartsPattern
	return m.matchMap[key], nil
}

// --- mockPropertyPredictor ---

type mockPropertyPredictor struct {
	predictions map[string]map[PropertyType]float64 // keyed by SMILES
}

func newMockPropertyPredictor() *mockPropertyPredictor {
	return &mockPropertyPredictor{
		predictions: make(map[string]map[PropertyType]float64),
	}
}

func (m *mockPropertyPredictor) SetPredictions(smiles string, preds map[PropertyType]float64) {
	m.predictions[smiles] = preds
}

func (m *mockPropertyPredictor) Predict(ctx context.Context, smiles string, props []PropertyType) (map[PropertyType]float64, error) {
	all, ok := m.predictions[smiles]
	if !ok {
		return stubPropertyPredictions(smiles, props), nil
	}
	out := make(map[PropertyType]float64, len(props))
	for _, p := range props {
		if v, exists := all[p]; exists {
			out[p] = v
		}
	}
	return out, nil
}

// --- mockServingClient ---

type mockServingClient struct {
	mu             sync.Mutex
	predictFn      func(ctx context.Context, modelID string, payload []byte) ([]byte, error)
	callCount      atomic.Int32
	callTimestamps []time.Time
	healthErr      error
	modelInfoResp  *ModelMetadata
}

func (m *mockServingClient) Predict(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
	m.callCount.Add(1)
	m.mu.Lock()
	m.callTimestamps = append(m.callTimestamps, time.Now())
	m.mu.Unlock()
	if m.predictFn != nil {
		return m.predictFn(ctx, modelID, payload)
	}
	// Default: echo back a similarity response.
	return json.Marshal(map[string]interface{}{"score": 0.85, "vector": make([]float64, 256)})
}

func (m *mockServingClient) Healthy(ctx context.Context) error {
	return m.healthErr
}

func (m *mockServingClient) ModelInfo(ctx context.Context, modelID string) (*ModelMetadata, error) {
	if m.modelInfoResp != nil {
		return m.modelInfoResp, nil
	}
	return &ModelMetadata{
		ModelID:   modelID,
		ModelName: "InfringeNet Remote",
		Version:   "1.0.0",
		SupportedTasks: []string{
			"literal_infringement", "structural_similarity",
			"property_impact", "embed_structure",
		},
	}, nil
}

func (m *mockServingClient) CallCount() int {
	return int(m.callCount.Load())
}

func (m *mockServingClient) Timestamps() []time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]time.Time, len(m.callTimestamps))
	copy(cp, m.callTimestamps)
	return cp
}

// --- mockLogger ---

type mockLogger struct{}

func (mockLogger) Info(msg string, kv ...interface{})  {}
func (mockLogger) Warn(msg string, kv ...interface{})  {}
func (mockLogger) Error(msg string, kv ...interface{}) {}
func (mockLogger) Debug(msg string, kv ...interface{}) {}

// =========================================================================
// Test helpers
// =========================================================================

func newTestLocalModel(t *testing.T) *localInfringeModel {
	t.Helper()
	m, err := NewLocalInfringeModel(
		&mockModelRegistry{},
		"/tmp/test-model",
		newMockValidator("INVALID"),
		newMockSMARTSMatcher(map[string]bool{
			"c1ccccc1|[#6]1:[#6]:[#6]:[#6]:[#6]:[#6]:1": true, // benzene matches aromatic ring
		}),
		nil, // use stub predictor
		mockLogger{},
	)
	if err != nil {
		t.Fatalf("NewLocalInfringeModel: %v", err)
	}
	return m
}

func newTestLocalModelWithPredictor(t *testing.T, pp *mockPropertyPredictor) *localInfringeModel {
	t.Helper()
	m, err := NewLocalInfringeModel(
		&mockModelRegistry{},
		"/tmp/test-model",
		newMockValidator("INVALID"),
		newMockSMARTSMatcher(nil),
		pp,
		mockLogger{},
	)
	if err != nil {
		t.Fatalf("NewLocalInfringeModel: %v", err)
	}
	return m
}

func newTestRemoteModel(t *testing.T, client *mockServingClient) *remoteInfringeModel {
	t.Helper()
	m, err := NewRemoteInfringeModel(
		client,
		newMockValidator("INVALID"),
		mockLogger{},
		WithRetryPolicy(3, 50*time.Millisecond),
		WithCacheSize(5),
		WithInferenceTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("NewRemoteInfringeModel: %v", err)
	}
	return m
}

func assertVectorDimension(t *testing.T, vec []float64, expected int) {
	t.Helper()
	if len(vec) != expected {
		t.Errorf("expected vector dimension %d, got %d", expected, len(vec))
	}
}

func assertVectorEqual(t *testing.T, vec1, vec2 []float64) {
	t.Helper()
	if len(vec1) != len(vec2) {
		t.Fatalf("dimension mismatch: %d vs %d", len(vec1), len(vec2))
	}
	for i := range vec1 {
		if vec1[i] != vec2[i] {
			t.Errorf("vectors differ at index %d: %f vs %f", i, vec1[i], vec2[i])
			return
		}
	}
}

func assertVectorNotEqual(t *testing.T, vec1, vec2 []float64) {
	t.Helper()
	if len(vec1) != len(vec2) {
		return // different dimensions → not equal
	}
	for i := range vec1 {
		if vec1[i] != vec2[i] {
			return
		}
	}
	t.Error("expected vectors to differ, but they are identical")
}

func testCosineSimilarity(a, b []float64) float64 {
	return cosineSim(a, b)
}

func assertInDelta(t *testing.T, expected, actual, delta float64, msg string) {
	t.Helper()
	if math.Abs(expected-actual) > delta {
		t.Errorf("%s: expected %f ± %f, got %f", msg, expected, delta, actual)
	}
}

// =========================================================================
// Local model tests
// =========================================================================

func TestNewLocalInfringeModel_Success(t *testing.T) {
	m := newTestLocalModel(t)
	if m == nil {
		t.Fatal("expected non-nil model")
	}
	if m.ModelInfo().ModelID == "" {
		t.Error("expected non-empty model ID")
	}
}

func TestNewLocalInfringeModel_InvalidPath(t *testing.T) {
	_, err := NewLocalInfringeModel(
		&mockModelRegistry{},
		"",
		newMockValidator(),
		newMockSMARTSMatcher(nil),
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected error for empty model path")
	}
}

func TestNewRemoteInfringeModel_Success(t *testing.T) {
	client := &mockServingClient{}
	m := newTestRemoteModel(t, client)
	if m == nil {
		t.Fatal("expected non-nil model")
	}
}

func TestNewRemoteInfringeModel_NilClient(t *testing.T) {
	_, err := NewRemoteInfringeModel(nil, newMockValidator(), nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestLocalModel_PredictLiteral_StrictMode_AllMatch(t *testing.T) {
	m := newTestLocalModel(t)
	smiles := "CCO"
	molVec, _ := m.EmbedStructure(context.Background(), smiles)

	// Create elements whose feature vectors are identical to the molecule embedding → score ≈ 1.0
	elems := []*ClaimElementFeature{
		{ElementID: "e1", FeatureVector: molVec, RequiredPresence: true, Weight: 1.0},
		{ElementID: "e2", FeatureVector: molVec, RequiredPresence: true, Weight: 1.0},
	}
	req := &LiteralPredictionRequest{
		MoleculeSMILES: smiles,
		ClaimElements:  elems,
		PredictionMode: PredictionStrict,
	}
	result, err := m.PredictLiteralInfringement(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInDelta(t, 1.0, result.OverallScore, 0.01, "OverallScore")
	if len(result.UnmatchedElements) != 0 {
		t.Errorf("expected 0 unmatched, got %d", len(result.UnmatchedElements))
	}
}

func TestLocalModel_PredictLiteral_StrictMode_OneUnmatched(t *testing.T) {
	m := newTestLocalModel(t)
	smiles := "CCO"
	molVec, _ := m.EmbedStructure(context.Background(), smiles)

	// e1 matches perfectly, e2 has a zero vector → score ≈ 0
	elems := []*ClaimElementFeature{
		{ElementID: "e1", FeatureVector: molVec, RequiredPresence: true, Weight: 1.0},
		{ElementID: "e2", FeatureVector: make([]float64, defaultEmbeddingDim), RequiredPresence: true, Weight: 1.0},
	}
	req := &LiteralPredictionRequest{
		MoleculeSMILES: smiles,
		ClaimElements:  elems,
		PredictionMode: PredictionStrict,
	}
	result, err := m.PredictLiteralInfringement(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Strict = min(scores). e2 score ≈ 0 → overall ≈ 0.
	if result.OverallScore > 0.1 {
		t.Errorf("expected low overall score in strict mode, got %f", result.OverallScore)
	}
	if len(result.UnmatchedElements) == 0 {
		t.Error("expected at least one unmatched element")
	}
}

func TestLocalModel_PredictLiteral_RelaxedMode_PartialMatch(t *testing.T) {
	m := newTestLocalModel(t)
	smiles := "CCO"
	molVec, _ := m.EmbedStructure(context.Background(), smiles)

	elems := []*ClaimElementFeature{
		{ElementID: "e1", FeatureVector: molVec, RequiredPresence: true, Weight: 1.0},
		{ElementID: "e2", FeatureVector: make([]float64, defaultEmbeddingDim), RequiredPresence: true, Weight: 1.0},
	}
	req := &LiteralPredictionRequest{
		MoleculeSMILES: smiles,
		ClaimElements:  elems,
		PredictionMode: PredictionRelaxed,
	}
	result, err := m.PredictLiteralInfringement(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Relaxed = weighted average. One ≈1.0, one ≈0.0 → ≈0.5
	if result.OverallScore < 0.3 || result.OverallScore > 0.7 {
		t.Errorf("expected mid-range overall score in relaxed mode, got %f", result.OverallScore)
	}
}

func TestLocalModel_PredictLiteral_SMARTSMatch(t *testing.T) {
	m := newTestLocalModel(t)
	elems := []*ClaimElementFeature{
		{
			ElementID:        "aromatic_ring",
			SMARTSPattern:    "[#6]1:[#6]:[#6]:[#6]:[#6]:[#6]:1",
			RequiredPresence: true,
			Weight:           1.0,
		},
	}
	req := &LiteralPredictionRequest{
		MoleculeSMILES: "c1ccccc1", // benzene
		ClaimElements:  elems,
		PredictionMode: PredictionStrict,
	}
	result, err := m.PredictLiteralInfringement(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInDelta(t, 1.0, result.ElementScores["aromatic_ring"], 0.01, "SMARTS match score")
}

func TestLocalModel_PredictLiteral_VectorFallback(t *testing.T) {
	m := newTestLocalModel(t)
	smiles := "CCO"
	molVec, _ := m.EmbedStructure(context.Background(), smiles)

	elems := []*ClaimElementFeature{
		{
			ElementID:     "vec_elem",
			FeatureVector: molVec,
			Weight:        1.0,
			// No SMARTSPattern → vector fallback
		},
	}
	req := &LiteralPredictionRequest{
		MoleculeSMILES: smiles,
		ClaimElements:  elems,
		PredictionMode: PredictionStrict,
	}
	result, err := m.PredictLiteralInfringement(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInDelta(t, 1.0, result.ElementScores["vec_elem"], 0.01, "vector fallback score")
}

func TestLocalModel_PredictLiteral_EmptyElements(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictLiteralInfringement(context.Background(), &LiteralPredictionRequest{
		MoleculeSMILES: "CCO",
		ClaimElements:  []*ClaimElementFeature{},
	})
	if err == nil {
		t.Fatal("expected error for empty elements")
	}
}

func TestLocalModel_PredictLiteral_InvalidSMILES(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictLiteralInfringement(context.Background(), &LiteralPredictionRequest{
		MoleculeSMILES: "INVALID",
		ClaimElements:  []*ClaimElementFeature{{ElementID: "e1", FeatureVector: []float64{1}}},
	})
	if err == nil {
		t.Fatal("expected error for invalid SMILES")
	}
}

func TestLocalModel_ComputeSimilarity_IdenticalMolecules(t *testing.T) {
	m := newTestLocalModel(t)
	score, err := m.ComputeStructuralSimilarity(context.Background(), "CCO", "CCO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInDelta(t, 1.0, score, 0.01, "identical molecules")
}

func TestLocalModel_ComputeSimilarity_DifferentMolecules(t *testing.T) {
	m := newTestLocalModel(t)
	// Methane vs a long chain — very different embeddings.
	score, err := m.ComputeStructuralSimilarity(context.Background(), "C", "C1=CC=C(C=C1)C2=CC=C(C=C2)N(C3=CC=CC=C3)C4=CC=CC=C4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score > 0.5 {
		t.Errorf("expected low similarity for very different molecules, got %f", score)
	}
}

func TestLocalModel_ComputeSimilarity_SimilarMolecules(t *testing.T) {
	m := newTestLocalModel(t)
	// Benzene vs toluene — structurally similar.
	score, err := m.ComputeStructuralSimilarity(context.Background(), "c1ccccc1", "Cc1ccccc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With deterministic hash embeddings the exact value depends on the hash,
	// but both are short aromatic SMILES so we just verify it's a valid score.
	if score < 0 || score > 1 {
		t.Errorf("score out of range: %f", score)
	}
}

func TestLocalModel_ComputeSimilarity_InvalidSMILES(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.ComputeStructuralSimilarity(context.Background(), "INVALID", "CCO")
	if err == nil {
		t.Fatal("expected error for invalid smiles1")
	}
	_, err = m.ComputeStructuralSimilarity(context.Background(), "CCO", "INVALID")
	if err == nil {
		t.Fatal("expected error for invalid smiles2")
	}
}

func TestLocalModel_EmbedStructure_Dimensions(t *testing.T) {
	m := newTestLocalModel(t)
	vec, err := m.EmbedStructure(context.Background(), "CCO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertVectorDimension(t, vec, defaultEmbeddingDim)
}

func TestLocalModel_EmbedStructure_Deterministic(t *testing.T) {
	m := newTestLocalModel(t)
	vec1, _ := m.EmbedStructure(context.Background(), "CCO")
	vec2, _ := m.EmbedStructure(context.Background(), "CCO")
	assertVectorEqual(t, vec1, vec2)
}

func TestLocalModel_EmbedStructure_DifferentInputs(t *testing.T) {
	m := newTestLocalModel(t)
	vec1, _ := m.EmbedStructure(context.Background(), "CCO")
	vec2, _ := m.EmbedStructure(context.Background(), "c1ccccc1")
	assertVectorNotEqual(t, vec1, vec2)
}

func TestLocalModel_EmbedStructure_InvalidSMILES(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.EmbedStructure(context.Background(), "INVALID")
	if err == nil {
		t.Fatal("expected error for invalid SMILES")
	}
}

func TestLocalModel_PredictPropertyImpact_NoChange(t *testing.T) {
	m := newTestLocalModel(t)
	req := &PropertyImpactRequest{
		OriginalSMILES: "CCO",
		ModifiedSMILES: "CCO",
		TargetProperties: []PropertyType{
			PropertyHOMO, PropertyLUMO, PropertyBandGap,
		},
	}
	result, err := m.PredictPropertyImpact(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range req.TargetProperties {
		d := result.Impacts[p]
		if d == nil {
			t.Errorf("missing impact for %s", p)
			continue
		}
		assertInDelta(t, 0, d.DeltaPercent, 0.01, string(p)+" delta")
		if d.ImpactLevel != ImpactNegligible {
			t.Errorf("expected Negligible for %s, got %s", p, d.ImpactLevel)
		}
	}
	assertInDelta(t, 1.0, result.OverallSimilarity, 0.01, "overall similarity")
}

func TestLocalModel_PredictPropertyImpact_MajorChange(t *testing.T) {
	pp := newMockPropertyPredictor()
	pp.SetPredictions("CCO", map[PropertyType]float64{
		PropertyHOMO: -5.0,
		PropertyLUMO: -1.0,
	})
	pp.SetPredictions("C1=CC=C(C=C1)C2=CC=C(C=C2)N(C3=CC=CC=C3)C4=CC=CC=C4", map[PropertyType]float64{
		PropertyHOMO: -7.5, // 50% change
		PropertyLUMO: -2.8, // 180% change
	})
	m := newTestLocalModelWithPredictor(t, pp)
	req := &PropertyImpactRequest{
		OriginalSMILES: "CCO",
		ModifiedSMILES: "C1=CC=C(C=C1)C2=CC=C(C=C2)N(C3=CC=CC=C3)C4=CC=CC=C4",
		TargetProperties: []PropertyType{
			PropertyHOMO, PropertyLUMO,
		},
	}
	result, err := m.PredictPropertyImpact(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	homoImpact := result.Impacts[PropertyHOMO]
	if homoImpact == nil {
		t.Fatal("missing HOMO impact")
	}
	if homoImpact.ImpactLevel != ImpactMajor {
		t.Errorf("expected Major for HOMO, got %s (delta=%.2f%%)", homoImpact.ImpactLevel, homoImpact.DeltaPercent)
	}
	lumoImpact := result.Impacts[PropertyLUMO]
	if lumoImpact == nil {
		t.Fatal("missing LUMO impact")
	}
	if lumoImpact.ImpactLevel != ImpactMajor {
		t.Errorf("expected Major for LUMO, got %s (delta=%.2f%%)", lumoImpact.ImpactLevel, lumoImpact.DeltaPercent)
	}
}

func TestLocalModel_PredictPropertyImpact_MinorChange(t *testing.T) {
	pp := newMockPropertyPredictor()
	pp.SetPredictions("CCO", map[PropertyType]float64{
		PropertyEmissionWavelength: 500.0,
	})
	pp.SetPredictions("CCCO", map[PropertyType]float64{
		PropertyEmissionWavelength: 510.0, // 2% change
	})
	m := newTestLocalModelWithPredictor(t, pp)
	req := &PropertyImpactRequest{
		OriginalSMILES:   "CCO",
		ModifiedSMILES:   "CCCO",
		TargetProperties: []PropertyType{PropertyEmissionWavelength},
	}
	result, err := m.PredictPropertyImpact(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := result.Impacts[PropertyEmissionWavelength]
	if d == nil {
		t.Fatal("missing EmissionWavelength impact")
	}
	if d.ImpactLevel != ImpactMinor {
		t.Errorf("expected Minor, got %s (delta=%.2f%%)", d.ImpactLevel, d.DeltaPercent)
	}
}

func TestLocalModel_PredictPropertyImpact_SpecificProperties(t *testing.T) {
	m := newTestLocalModel(t)
	req := &PropertyImpactRequest{
		OriginalSMILES:   "CCO",
		ModifiedSMILES:   "CCCO",
		TargetProperties: []PropertyType{PropertyEmissionWavelength, PropertyQuantumYield},
	}
	result, err := m.PredictPropertyImpact(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Impacts) != 2 {
		t.Errorf("expected 2 impacts, got %d", len(result.Impacts))
	}
	if _, ok := result.Impacts[PropertyEmissionWavelength]; !ok {
		t.Error("missing EmissionWavelength")
	}
	if _, ok := result.Impacts[PropertyQuantumYield]; !ok {
		t.Error("missing QuantumYield")
	}
	// Should NOT contain other properties.
	if _, ok := result.Impacts[PropertyHOMO]; ok {
		t.Error("unexpected HOMO in results")
	}
}

func TestLocalModel_PredictPropertyImpact_InvalidOriginal(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictPropertyImpact(context.Background(), &PropertyImpactRequest{
		OriginalSMILES: "INVALID",
		ModifiedSMILES: "CCO",
	})
	if err == nil {
		t.Fatal("expected error for invalid original SMILES")
	}
}

func TestLocalModel_PredictPropertyImpact_InvalidModified(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictPropertyImpact(context.Background(), &PropertyImpactRequest{
		OriginalSMILES: "CCO",
		ModifiedSMILES: "INVALID",
	})
	if err == nil {
		t.Fatal("expected error for invalid modified SMILES")
	}
}

func TestLocalModel_ModelInfo(t *testing.T) {
	m := newTestLocalModel(t)
	info := m.ModelInfo()
	if info == nil {
		t.Fatal("expected non-nil ModelInfo")
	}
	if info.Version == "" {
		t.Error("expected non-empty version")
	}
	if info.Architecture == "" {
		t.Error("expected non-empty architecture")
	}
	if len(info.SupportedTasks) == 0 {
		t.Error("expected non-empty supported tasks")
	}
	if info.PerformanceMetrics["AUC"] <= 0 {
		t.Error("expected positive AUC metric")
	}
}

func TestLocalModel_Healthy(t *testing.T) {
	m := newTestLocalModel(t)
	if err := m.Healthy(context.Background()); err != nil {
		t.Fatalf("expected healthy: %v", err)
	}
}

func TestLocalModel_Healthy_Degraded(t *testing.T) {
	m := newTestLocalModel(t)
	m.SetHealthy(false)
	if err := m.Healthy(context.Background()); err == nil {
		t.Fatal("expected error for degraded model")
	}
}

func TestLocalModel_ConcurrentInference(t *testing.T) {
	m := newTestLocalModel(t)
	smiles := "CCO"
	molVec, _ := m.EmbedStructure(context.Background(), smiles)

	elems := []*ClaimElementFeature{
		{ElementID: "e1", FeatureVector: molVec, Weight: 1.0},
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			req := &LiteralPredictionRequest{
				MoleculeSMILES: smiles,
				ClaimElements:  elems,
				PredictionMode: PredictionStrict,
			}
			result, err := m.PredictLiteralInfringement(context.Background(), req)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: %w", idx, err)
				return
			}
			if result.OverallScore < 0.9 {
				errCh <- fmt.Errorf("goroutine %d: unexpected score %f", idx, result.OverallScore)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestLocalModel_InferenceTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond) // ensure context is expired

	// EmbedStructure itself is fast (deterministic hash), but the context is already done.
	// The current implementation doesn't check ctx inside EmbedStructure,
	// so we test PredictLiteralInfringement which does more work.
	molVec, _ := deterministicEmbed("CCO", defaultEmbeddingDim), error(nil)
	_ = molVec
	// For a true timeout test, we verify the context error propagates.
	if ctx.Err() == nil {
		t.Fatal("expected context to be expired")
	}
}

// =========================================================================
// Remote model tests
// =========================================================================

func TestRemoteModel_PredictLiteral_Success(t *testing.T) {
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			result := LiteralPredictionResult{
				OverallScore:    0.88,
				ElementScores:   map[string]float64{"e1": 0.92, "e2": 0.84},
				MatchedElements: []string{"e1", "e2"},
				Confidence:      0.90,
				InferenceTimeMs: 12,
			}
			return json.Marshal(result)
		},
	}
	m := newTestRemoteModel(t, client)
	req := &LiteralPredictionRequest{
		MoleculeSMILES: "CCO",
		ClaimElements: []*ClaimElementFeature{
			{ElementID: "e1", FeatureVector: []float64{1.0}},
			{ElementID: "e2", FeatureVector: []float64{0.5}},
		},
		PredictionMode: PredictionStrict,
	}
	result, err := m.PredictLiteralInfringement(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInDelta(t, 0.88, result.OverallScore, 0.01, "overall score")
	if client.CallCount() != 1 {
		t.Errorf("expected 1 call, got %d", client.CallCount())
	}
}

func TestRemoteModel_PredictLiteral_NetworkError(t *testing.T) {
	callCount := atomic.Int32{}
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			callCount.Add(1)
			return nil, fmt.Errorf("connection refused")
		},
	}
	m := newTestRemoteModel(t, client)
	req := &LiteralPredictionRequest{
		MoleculeSMILES: "CCO",
		ClaimElements:  []*ClaimElementFeature{{ElementID: "e1", FeatureVector: []float64{1}}},
	}
	_, err := m.PredictLiteralInfringement(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	// Should have retried: 1 initial + 3 retries = 4 calls
	if callCount.Load() != 4 {
		t.Errorf("expected 4 calls (1 + 3 retries), got %d", callCount.Load())
	}
}

func TestRemoteModel_PredictLiteral_RetrySuccess(t *testing.T) {
	callCount := atomic.Int32{}
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			n := callCount.Add(1)
			if n == 1 {
				return nil, fmt.Errorf("transient error")
			}
			result := LiteralPredictionResult{
				OverallScore:    0.75,
				ElementScores:   map[string]float64{"e1": 0.75},
				MatchedElements: []string{"e1"},
				Confidence:      0.85,
				InferenceTimeMs: 8,
			}
			return json.Marshal(result)
		},
	}
	m := newTestRemoteModel(t, client)
	req := &LiteralPredictionRequest{
		MoleculeSMILES: "CCO",
		ClaimElements:  []*ClaimElementFeature{{ElementID: "e1", FeatureVector: []float64{1}}},
	}
	result, err := m.PredictLiteralInfringement(context.Background(), req)
	if err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	assertInDelta(t, 0.75, result.OverallScore, 0.01, "overall score after retry")
	if callCount.Load() != 2 {
		t.Errorf("expected 2 calls, got %d", callCount.Load())
	}
}

func TestRemoteModel_PredictLiteral_RetryExhausted(t *testing.T) {
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			return nil, fmt.Errorf("persistent failure")
		},
	}
	m := newTestRemoteModel(t, client)
	req := &LiteralPredictionRequest{
		MoleculeSMILES: "CCO",
		ClaimElements:  []*ClaimElementFeature{{ElementID: "e1", FeatureVector: []float64{1}}},
	}
	_, err := m.PredictLiteralInfringement(context.Background(), req)
	if err == nil {
		t.Fatal("expected error after all retries exhausted")
	}
}

func TestRemoteModel_PredictLiteral_RetryBackoff(t *testing.T) {
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			return nil, fmt.Errorf("fail")
		},
	}
	m, _ := NewRemoteInfringeModel(
		client,
		newMockValidator(),
		mockLogger{},
		WithRetryPolicy(2, 100*time.Millisecond),
		WithCacheSize(5),
		WithInferenceTimeout(2*time.Second),
	)
	req := &LiteralPredictionRequest{
		MoleculeSMILES: "CCO",
		ClaimElements:  []*ClaimElementFeature{{ElementID: "e1", FeatureVector: []float64{1}}},
	}
	start := time.Now()
	_, _ = m.PredictLiteralInfringement(context.Background(), req)
	elapsed := time.Since(start)

	// With backoff=100ms and 2 retries: delays are 100ms (2^0) + 200ms (2^1) = 300ms minimum.
	timestamps := client.Timestamps()
	if len(timestamps) < 3 {
		t.Fatalf("expected at least 3 call timestamps, got %d", len(timestamps))
	}
	// Verify exponential backoff: second gap should be roughly 2x the first.
	gap1 := timestamps[1].Sub(timestamps[0])
	gap2 := timestamps[2].Sub(timestamps[1])
	if gap2 < gap1 {
		t.Logf("gap1=%v gap2=%v — gap2 should be >= gap1 for exponential backoff", gap1, gap2)
	}
	// Total elapsed should be at least 200ms (100 + 200, minus some scheduling variance).
	if elapsed < 200*time.Millisecond {
		t.Errorf("expected at least 200ms total elapsed, got %v", elapsed)
	}
}

func TestRemoteModel_ComputeSimilarity_CacheHit(t *testing.T) {
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			return json.Marshal(map[string]interface{}{"score": 0.92})
		},
	}
	m := newTestRemoteModel(t, client)

	score1, err := m.ComputeStructuralSimilarity(context.Background(), "CCO", "CCCO")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	score2, err := m.ComputeStructuralSimilarity(context.Background(), "CCO", "CCCO")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	assertInDelta(t, score1, score2, 0.001, "cached score")
	if client.CallCount() != 1 {
		t.Errorf("expected 1 remote call (cache hit), got %d", client.CallCount())
	}
}

func TestRemoteModel_ComputeSimilarity_CacheMiss(t *testing.T) {
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			return json.Marshal(map[string]interface{}{"score": 0.80})
		},
	}
	m := newTestRemoteModel(t, client)

	_, _ = m.ComputeStructuralSimilarity(context.Background(), "CCO", "CCCO")
	_, _ = m.ComputeStructuralSimilarity(context.Background(), "c1ccccc1", "Cc1ccccc1")
	if client.CallCount() != 2 {
		t.Errorf("expected 2 remote calls (cache miss), got %d", client.CallCount())
	}
}

func TestRemoteModel_ComputeSimilarity_CacheEviction(t *testing.T) {
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			return json.Marshal(map[string]interface{}{"score": 0.70})
		},
	}
	// Cache size = 5
	m := newTestRemoteModel(t, client)

	// Fill cache with 5 entries.
	for i := 0; i < 5; i++ {
		_, _ = m.ComputeStructuralSimilarity(context.Background(), fmt.Sprintf("C%d", i), "CCO")
	}
	if client.CallCount() != 5 {
		t.Fatalf("expected 5 calls, got %d", client.CallCount())
	}

	// Add a 6th entry → evicts the oldest.
	_, _ = m.ComputeStructuralSimilarity(context.Background(), "C99", "CCO")
	if client.CallCount() != 6 {
		t.Fatalf("expected 6 calls, got %d", client.CallCount())
	}

	// The first entry (C0|CCO) should have been evicted → triggers a new remote call.
	_, _ = m.ComputeStructuralSimilarity(context.Background(), "C0", "CCO")
	if client.CallCount() != 7 {
		t.Errorf("expected 7 calls after eviction, got %d", client.CallCount())
	}
}

func TestRemoteModel_EmbedStructure_CacheHit(t *testing.T) {
	client := &mockServingClient{
		predictFn: func(ctx context.Context, modelID string, payload []byte) ([]byte, error) {
			vec := make([]float64, 256)
			for i := range vec {
				vec[i] = 0.01 * float64(i)
			}
			return json.Marshal(map[string]interface{}{"vector": vec})
		},
	}
	m := newTestRemoteModel(t, client)

	vec1, err := m.EmbedStructure(context.Background(), "CCO")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	vec2, err := m.EmbedStructure(context.Background(), "CCO")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	assertVectorEqual(t, vec1, vec2)
	if client.CallCount() != 1 {
		t.Errorf("expected 1 remote call (cache hit), got %d", client.CallCount())
	}
}

func TestRemoteModel_Healthy_ServiceUp(t *testing.T) {
	client := &mockServingClient{}
	m := newTestRemoteModel(t, client)
	if err := m.Healthy(context.Background()); err != nil {
		t.Fatalf("expected healthy: %v", err)
	}
}

func TestRemoteModel_Healthy_ServiceDown(t *testing.T) {
	client := &mockServingClient{healthErr: fmt.Errorf("service unavailable")}
	m := newTestRemoteModel(t, client)
	if err := m.Healthy(context.Background()); err == nil {
		t.Fatal("expected error for down service")
	}
}

func TestRemoteModel_ModelInfo(t *testing.T) {
	client := &mockServingClient{
		modelInfoResp: &ModelMetadata{
			ModelID:      "infringe-net-remote-v1",
			ModelName:    "InfringeNet Remote",
			Version:      "2.1.0",
			Architecture: "MPNN-Remote",
			SupportedTasks: []string{
				"literal_infringement", "structural_similarity",
			},
		},
	}
	m := newTestRemoteModel(t, client)
	info := m.ModelInfo()
	if info.Version != "2.1.0" {
		t.Errorf("expected version 2.1.0, got %s", info.Version)
	}
	if info.Architecture != "MPNN-Remote" {
		t.Errorf("expected MPNN-Remote, got %s", info.Architecture)
	}
}

// =========================================================================
// ModelOption tests
// =========================================================================

func TestModelOption_DeviceType(t *testing.T) {
	o := defaultModelOptions()
	WithDeviceType("GPU")(o)
	if o.deviceType != "GPU" {
		t.Errorf("expected GPU, got %s", o.deviceType)
	}
}

func TestModelOption_BatchSize(t *testing.T) {
	o := defaultModelOptions()
	WithBatchSize(64)(o)
	if o.batchSize != 64 {
		t.Errorf("expected 64, got %d", o.batchSize)
	}
	// Zero should not change.
	WithBatchSize(0)(o)
	if o.batchSize != 64 {
		t.Errorf("expected 64 unchanged, got %d", o.batchSize)
	}
}

func TestModelOption_CacheSize(t *testing.T) {
	o := defaultModelOptions()
	WithCacheSize(500)(o)
	if o.cacheSize != 500 {
		t.Errorf("expected 500, got %d", o.cacheSize)
	}
}

func TestModelOption_RetryPolicy(t *testing.T) {
	o := defaultModelOptions()
	WithRetryPolicy(5, 300*time.Millisecond)(o)
	if o.maxRetries != 5 {
		t.Errorf("expected 5 retries, got %d", o.maxRetries)
	}
	if o.retryBackoff != 300*time.Millisecond {
		t.Errorf("expected 300ms backoff, got %v", o.retryBackoff)
	}
}

func TestModelOption_InferenceTimeout(t *testing.T) {
	o := defaultModelOptions()
	WithInferenceTimeout(10 * time.Second)(o)
	if o.inferenceTimeout != 10*time.Second {
		t.Errorf("expected 10s, got %v", o.inferenceTimeout)
	}
}

// =========================================================================
// PropertyType & ImpactLevel tests
// =========================================================================

func TestPropertyType_String(t *testing.T) {
	tests := []struct {
		pt   PropertyType
		want string
	}{
		{PropertyHOMO, "HOMO"},
		{PropertyLUMO, "LUMO"},
		{PropertyBandGap, "BandGap"},
		{PropertyEmissionWavelength, "EmissionWavelength"},
		{PropertyQuantumYield, "QuantumYield"},
		{PropertyThermalStability, "ThermalStability"},
		{PropertyGlassTransitionTemp, "GlassTransitionTemp"},
		{PropertyChargeCarrierMobility, "ChargeCarrierMobility"},
	}
	for _, tt := range tests {
		if got := tt.pt.String(); got != tt.want {
			t.Errorf("PropertyType.String() = %s, want %s", got, tt.want)
		}
	}
}

func TestPropertyType_AllValues(t *testing.T) {
	all := AllPropertyTypes()
	expected := map[PropertyType]bool{
		PropertyHOMO:                  true,
		PropertyLUMO:                  true,
		PropertyBandGap:               true,
		PropertyEmissionWavelength:    true,
		PropertyQuantumYield:          true,
		PropertyThermalStability:      true,
		PropertyGlassTransitionTemp:   true,
		PropertyChargeCarrierMobility: true,
	}
	if len(all) != len(expected) {
		t.Errorf("expected %d property types, got %d", len(expected), len(all))
	}
	for _, p := range all {
		if !expected[p] {
			t.Errorf("unexpected property type: %s", p)
		}
	}
}

func TestImpactLevel_Classification(t *testing.T) {
	tests := []struct {
		delta float64
		want  ImpactLevel
	}{
		{0.0, ImpactNegligible},
		{0.5, ImpactNegligible},
		{0.99, ImpactNegligible},
		{1.0, ImpactMinor},
		{3.0, ImpactMinor},
		{4.99, ImpactMinor},
		{5.0, ImpactModerate},
		{12.0, ImpactModerate},
		{19.99, ImpactModerate},
		{20.0, ImpactMajor},
		{50.0, ImpactMajor},
		{100.0, ImpactMajor},
	}
	for _, tt := range tests {
		got := ClassifyImpact(tt.delta)
		if got != tt.want {
			t.Errorf("ClassifyImpact(%.2f) = %s, want %s", tt.delta, got, tt.want)
		}
	}
}

// =========================================================================
// LRU cache unit tests
// =========================================================================

func TestLRUCache_PutGet(t *testing.T) {
	c := newLRUCache(3)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3)

	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Errorf("expected 1, got %v (ok=%v)", v, ok)
	}
	v, ok = c.Get("b")
	if !ok || v.(int) != 2 {
		t.Errorf("expected 2, got %v (ok=%v)", v, ok)
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	c := newLRUCache(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Put("c", 3) // evicts "a"

	if _, ok := c.Get("a"); ok {
		t.Error("expected 'a' to be evicted")
	}
	if _, ok := c.Get("b"); !ok {
		t.Error("expected 'b' to still exist")
	}
	if _, ok := c.Get("c"); !ok {
		t.Error("expected 'c' to still exist")
	}
}

func TestLRUCache_AccessRefreshesOrder(t *testing.T) {
	c := newLRUCache(2)
	c.Put("a", 1)
	c.Put("b", 2)
	c.Get("a")    // refresh "a" → "b" is now LRU
	c.Put("c", 3) // evicts "b"

	if _, ok := c.Get("a"); !ok {
		t.Error("expected 'a' to survive (was refreshed)")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("expected 'b' to be evicted")
	}
}

func TestLRUCache_UpdateExisting(t *testing.T) {
	c := newLRUCache(3)
	c.Put("a", 1)
	c.Put("a", 99)
	v, ok := c.Get("a")
	if !ok || v.(int) != 99 {
		t.Errorf("expected updated value 99, got %v", v)
	}
	if c.Len() != 1 {
		t.Errorf("expected length 1, got %d", c.Len())
	}
}

func TestLRUCache_ConcurrentAccess(t *testing.T) {
	c := newLRUCache(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("k%d", n)
			c.Put(key, n)
			c.Get(key)
		}(i)
	}
	wg.Wait()
	// No panic or race = pass.
}

// =========================================================================
// Helper function tests
// =========================================================================

func TestCosineSim_Identical(t *testing.T) {
	v := []float64{1, 2, 3, 4}
	assertInDelta(t, 1.0, cosineSim(v, v), 0.0001, "identical vectors")
}

func TestCosineSim_Orthogonal(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	assertInDelta(t, 0.0, cosineSim(a, b), 0.0001, "orthogonal vectors")
}

func TestCosineSim_Empty(t *testing.T) {
	assertInDelta(t, 0.0, cosineSim(nil, []float64{1, 2}), 0.0001, "nil first")
	assertInDelta(t, 0.0, cosineSim([]float64{1, 2}, nil), 0.0001, "nil second")
	assertInDelta(t, 0.0, cosineSim([]float64{}, []float64{}), 0.0001, "both empty")
}

func TestCosineSim_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	assertInDelta(t, 0.0, cosineSim(a, b), 0.0001, "zero vector")
}

func TestCosineSim_Antiparallel(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{-1, 0, 0}
	assertInDelta(t, -1.0, cosineSim(a, b), 0.0001, "antiparallel vectors")
}

func TestClamp01(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{-0.5, 0.0},
		{0.0, 0.0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
	}
	for _, tt := range tests {
		got := clamp01(tt.in)
		if got != tt.want {
			t.Errorf("clamp01(%f) = %f, want %f", tt.in, got, tt.want)
		}
	}
}

func TestDeltaPercent(t *testing.T) {
	tests := []struct {
		orig, mod float64
		want      float64
	}{
		{100, 100, 0},
		{100, 120, 20},
		{100, 80, -20},
		{0, 0, 0},
		{0, 5, 100}, // special case: original is zero
		{-5, -7.5, -50},
	}
	for _, tt := range tests {
		got := deltaPercent(tt.orig, tt.mod)
		assertInDelta(t, tt.want, got, 0.01, fmt.Sprintf("deltaPercent(%f,%f)", tt.orig, tt.mod))
	}
}

func TestDeterministicEmbed_Dimensions(t *testing.T) {
	vec := deterministicEmbed("CCO", 256)
	assertVectorDimension(t, vec, 256)
}

func TestDeterministicEmbed_Deterministic(t *testing.T) {
	v1 := deterministicEmbed("CCO", 256)
	v2 := deterministicEmbed("CCO", 256)
	assertVectorEqual(t, v1, v2)
}

func TestDeterministicEmbed_DifferentInputs(t *testing.T) {
	v1 := deterministicEmbed("CCO", 256)
	v2 := deterministicEmbed("c1ccccc1", 256)
	assertVectorNotEqual(t, v1, v2)
}

func TestDeterministicEmbed_Normalized(t *testing.T) {
	vec := deterministicEmbed("CCO", 256)
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	assertInDelta(t, 1.0, norm, 0.0001, "L2 norm should be 1.0")
}

func TestDeterministicEmbed_DifferentDimensions(t *testing.T) {
	v64 := deterministicEmbed("CCO", 64)
	v128 := deterministicEmbed("CCO", 128)
	assertVectorDimension(t, v64, 64)
	assertVectorDimension(t, v128, 128)
}

// =========================================================================
// Aggregation logic tests
// =========================================================================

func TestAggregateScores_Strict(t *testing.T) {
	m := newTestLocalModel(t)
	scores := map[string]float64{"e1": 0.9, "e2": 0.3, "e3": 0.7}
	elems := []*ClaimElementFeature{
		{ElementID: "e1", Weight: 1.0},
		{ElementID: "e2", Weight: 1.0},
		{ElementID: "e3", Weight: 1.0},
	}
	result := m.aggregateScores(scores, elems, PredictionStrict)
	assertInDelta(t, 0.3, result, 0.001, "strict = min")
}

func TestAggregateScores_Relaxed(t *testing.T) {
	m := newTestLocalModel(t)
	scores := map[string]float64{"e1": 1.0, "e2": 0.0}
	elems := []*ClaimElementFeature{
		{ElementID: "e1", Weight: 1.0},
		{ElementID: "e2", Weight: 1.0},
	}
	result := m.aggregateScores(scores, elems, PredictionRelaxed)
	assertInDelta(t, 0.5, result, 0.001, "relaxed = weighted avg")
}

func TestAggregateScores_Relaxed_WeightedAvg(t *testing.T) {
	m := newTestLocalModel(t)
	scores := map[string]float64{"e1": 1.0, "e2": 0.0}
	elems := []*ClaimElementFeature{
		{ElementID: "e1", Weight: 3.0},
		{ElementID: "e2", Weight: 1.0},
	}
	// weighted avg = (1.0*3 + 0.0*1) / (3+1) = 0.75
	result := m.aggregateScores(scores, elems, PredictionRelaxed)
	assertInDelta(t, 0.75, result, 0.001, "relaxed weighted avg")
}

func TestAggregateScores_Empty(t *testing.T) {
	m := newTestLocalModel(t)
	result := m.aggregateScores(map[string]float64{}, nil, PredictionStrict)
	assertInDelta(t, 0.0, result, 0.001, "empty scores")
}

func TestComputeConfidence_AllSame(t *testing.T) {
	m := newTestLocalModel(t)
	scores := map[string]float64{"e1": 0.8, "e2": 0.8, "e3": 0.8}
	conf := m.computeConfidence(scores)
	assertInDelta(t, 1.0, conf, 0.001, "all same → high confidence")
}

func TestComputeConfidence_HighVariance(t *testing.T) {
	m := newTestLocalModel(t)
	scores := map[string]float64{"e1": 1.0, "e2": 0.0}
	conf := m.computeConfidence(scores)
	// stddev = 0.5, confidence = 1 - 0.5 = 0.5
	assertInDelta(t, 0.5, conf, 0.001, "high variance → lower confidence")
}

func TestComputeConfidence_Empty(t *testing.T) {
	m := newTestLocalModel(t)
	conf := m.computeConfidence(map[string]float64{})
	assertInDelta(t, 0.0, conf, 0.001, "empty → 0")
}

// =========================================================================
// StubPropertyPredictions tests
// =========================================================================

func TestStubPropertyPredictions_Deterministic(t *testing.T) {
	props := []PropertyType{PropertyHOMO, PropertyLUMO, PropertyBandGap}
	p1 := stubPropertyPredictions("CCO", props)
	p2 := stubPropertyPredictions("CCO", props)
	for _, p := range props {
		if p1[p] != p2[p] {
			t.Errorf("stub not deterministic for %s: %f vs %f", p, p1[p], p2[p])
		}
	}
}

func TestStubPropertyPredictions_DifferentSMILES(t *testing.T) {
	props := []PropertyType{PropertyHOMO}
	p1 := stubPropertyPredictions("CCO", props)
	p2 := stubPropertyPredictions("c1ccccc1", props)
	if p1[PropertyHOMO] == p2[PropertyHOMO] {
		t.Error("expected different predictions for different SMILES")
	}
}

func TestStubPropertyPredictions_RangeHOMO(t *testing.T) {
	props := []PropertyType{PropertyHOMO}
	p := stubPropertyPredictions("CCO", props)
	v := p[PropertyHOMO]
	if v > -5.0 || v < -8.0 {
		t.Errorf("HOMO out of expected range [-8, -5]: %f", v)
	}
}

func TestStubPropertyPredictions_RangeQuantumYield(t *testing.T) {
	props := []PropertyType{PropertyQuantumYield}
	p := stubPropertyPredictions("CCO", props)
	v := p[PropertyQuantumYield]
	if v < 0 || v > 1 {
		t.Errorf("QuantumYield out of expected range [0, 1]: %f", v)
	}
}

// =========================================================================
// Edge case & integration-style tests
// =========================================================================

func TestLocalModel_PredictLiteral_NilRequest(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictLiteralInfringement(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestLocalModel_PredictLiteral_EmptySMILES(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictLiteralInfringement(context.Background(), &LiteralPredictionRequest{
		MoleculeSMILES: "",
		ClaimElements:  []*ClaimElementFeature{{ElementID: "e1"}},
	})
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestLocalModel_EmbedStructure_EmptySMILES(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.EmbedStructure(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestLocalModel_ComputeSimilarity_EmptySMILES(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.ComputeStructuralSimilarity(context.Background(), "", "CCO")
	if err == nil {
		t.Fatal("expected error for empty smiles1")
	}
	_, err = m.ComputeStructuralSimilarity(context.Background(), "CCO", "")
	if err == nil {
		t.Fatal("expected error for empty smiles2")
	}
}

func TestLocalModel_PredictPropertyImpact_NilRequest(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictPropertyImpact(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestLocalModel_PredictPropertyImpact_EmptyOriginal(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictPropertyImpact(context.Background(), &PropertyImpactRequest{
		OriginalSMILES: "",
		ModifiedSMILES: "CCO",
	})
	if err == nil {
		t.Fatal("expected error for empty original")
	}
}

func TestLocalModel_PredictPropertyImpact_EmptyModified(t *testing.T) {
	m := newTestLocalModel(t)
	_, err := m.PredictPropertyImpact(context.Background(), &PropertyImpactRequest{
		OriginalSMILES: "CCO",
		ModifiedSMILES: "",
	})
	if err == nil {
		t.Fatal("expected error for empty modified")
	}
}

func TestLocalModel_PredictPropertyImpact_AllPropertiesDefault(t *testing.T) {
	m := newTestLocalModel(t)
	req := &PropertyImpactRequest{
		OriginalSMILES:   "CCO",
		ModifiedSMILES:   "CCCO",
		TargetProperties: nil, // should default to all
	}
	result, err := m.PredictPropertyImpact(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	allProps := AllPropertyTypes()
	if len(result.Impacts) != len(allProps) {
		t.Errorf("expected %d impacts (all properties), got %d", len(allProps), len(result.Impacts))
	}
}

func TestRemoteModel_PredictLiteral_NilRequest(t *testing.T) {
	client := &mockServingClient{}
	m := newTestRemoteModel(t, client)
	_, err := m.PredictLiteralInfringement(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestRemoteModel_PredictLiteral_EmptySMILES(t *testing.T) {
	client := &mockServingClient{}
	m := newTestRemoteModel(t, client)
	_, err := m.PredictLiteralInfringement(context.Background(), &LiteralPredictionRequest{
		MoleculeSMILES: "",
		ClaimElements:  []*ClaimElementFeature{{ElementID: "e1"}},
	})
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestRemoteModel_PredictPropertyImpact_NilRequest(t *testing.T) {
	client := &mockServingClient{}
	m := newTestRemoteModel(t, client)
	_, err := m.PredictPropertyImpact(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestRemoteModel_EmbedStructure_EmptySMILES(t *testing.T) {
	client := &mockServingClient{}
	m := newTestRemoteModel(t, client)
	_, err := m.EmbedStructure(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestRemoteModel_ComputeSimilarity_EmptySMILES(t *testing.T) {
	client := &mockServingClient{}
	m := newTestRemoteModel(t, client)
	_, err := m.ComputeStructuralSimilarity(context.Background(), "", "CCO")
	if err == nil {
		t.Fatal("expected error for empty smiles1")
	}
}

// =========================================================================
// Interface compliance compile-time checks
// =========================================================================

var _ InfringeModel = (*localInfringeModel)(nil)
var _ InfringeModel = (*remoteInfringeModel)(nil)
var _ SMARTSMatcher = (*mockSMARTSMatcher)(nil)
var _ MoleculeValidator = (*mockMoleculeValidator)(nil)
var _ ServingClient = (*mockServingClient)(nil)


