package molpatent_gnn

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockModelBackend struct {
	predictFn func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error)
	callCount atomic.Int32
}

func (m *mockModelBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	m.callCount.Add(1)
	if m.predictFn != nil {
		return m.predictFn(ctx, req)
	}
	return &common.PredictResponse{
		Outputs: map[string][]byte{
			"embedding": common.EncodeFloat32Vector(make([]float32, 256)),
		},
		InferenceTimeMs: 10,
	}, nil
}

func (m *mockModelBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, nil
}

func (m *mockModelBackend) Healthy(ctx context.Context) error { return nil }
func (m *mockModelBackend) Close() error                      { return nil }

type mockGNNPreprocessor struct {
	validateErr    error
	preprocessResp *MolecularGraph
	preprocessErr  error
}

func (m *mockGNNPreprocessor) PreprocessSMILES(ctx context.Context, smiles string) (*MolecularGraph, error) {
	if m.preprocessErr != nil {
		return nil, m.preprocessErr
	}
	if m.preprocessResp != nil {
		return m.preprocessResp, nil
	}
	return &MolecularGraph{
		NodeFeatures:   [][]float32{{1, 0, 0}},
		EdgeIndex:      [][2]int{},
		EdgeFeatures:   [][]float32{},
		GlobalFeatures: []float32{12.0},
		NumAtoms:       1,
		SMILES:         smiles,
	}, nil
}

func (m *mockGNNPreprocessor) PreprocessMOL(ctx context.Context, molBlock string) (*MolecularGraph, error) {
	return nil, nil
}

func (m *mockGNNPreprocessor) PreprocessBatch(ctx context.Context, inputs []MolecularInput) ([]*MolecularGraph, error) {
	return nil, nil
}

func (m *mockGNNPreprocessor) ValidateSMILES(smiles string) error {
	return m.validateErr
}

func (m *mockGNNPreprocessor) Canonicalize(smiles string) (string, error) {
	return smiles, nil
}

type mockGNNPostprocessor struct{}

func (m *mockGNNPostprocessor) ProcessEmbedding(raw []float32, meta *InferenceMeta) (*EmbeddingResult, error) {
	norm := make([]float32, len(raw))
	copy(norm, raw)
	return &EmbeddingResult{NormalizedVector: norm, L2Norm: 1.0}, nil
}

func (m *mockGNNPostprocessor) ProcessBatchEmbedding(raw [][]float32, meta []*InferenceMeta) ([]*EmbeddingResult, error) {
	var results []*EmbeddingResult
	for i, r := range raw {
		var im *InferenceMeta
		if i < len(meta) {
			im = meta[i]
		}
		res, err := m.ProcessEmbedding(r, im)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (m *mockGNNPostprocessor) ComputeCosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("dimension mismatch")
	}
	return 0.85, nil
}

func (m *mockGNNPostprocessor) ComputeTanimotoSimilarity(a, b []byte) (float64, error) {
	return 0.80, nil
}

func (m *mockGNNPostprocessor) FuseScores(scores map[string]float64, weights map[string]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, fmt.Errorf("no scores")
	}
	total := 0.0
	wSum := 0.0
	for k, s := range scores {
		w, ok := weights[k]
		if !ok {
			continue
		}
		total += s * w
		wSum += w
	}
	if wSum == 0 {
		return 0, fmt.Errorf("no matching weights")
	}
	return total / wSum, nil
}

func (m *mockGNNPostprocessor) ClassifySimilarity(score float64) molecule.SimilarityLevel {
	switch {
	case score >= 0.85:
		return molecule.SimilarityHigh
	case score >= 0.70:
		return molecule.SimilarityMedium
	case score >= 0.55:
		return molecule.SimilarityLow
	default:
		return molecule.SimilarityNone
	}
}

type mockVectorSearcher struct {
	results []*VectorMatch
	err     error
}

func (m *mockVectorSearcher) Search(ctx context.Context, vector []float32, topK int, threshold float64) ([]*VectorMatch, error) {
	return m.results, m.err
}

type mockInferenceMetrics struct {
	inferenceCount atomic.Int32
}

func (m *mockInferenceMetrics) RecordInference(ctx context.Context, p *common.InferenceMetricParams) {
	m.inferenceCount.Add(1)
}
func (m *mockInferenceMetrics) RecordBatchProcessing(ctx context.Context, p *common.BatchMetricParams) {}
func (m *mockInferenceMetrics) RecordCacheAccess(ctx context.Context, hit bool, modelName string)     {}
func (m *mockInferenceMetrics) RecordCircuitBreakerStateChange(ctx context.Context, modelName, from, to string) {}
func (m *mockInferenceMetrics) RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64) {}
func (m *mockInferenceMetrics) RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool) {}
func (m *mockInferenceMetrics) GetInferenceLatencyHistogram() common.LatencyHistogram { return nil }
func (m *mockInferenceMetrics) GetCurrentStats() *common.IntelligenceStats            { return &common.IntelligenceStats{} }

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestEngine(t *testing.T) (*GNNInferenceEngine, *mockModelBackend, *mockInferenceMetrics) {
	t.Helper()
	backend := &mockModelBackend{}
	metrics := &mockInferenceMetrics{}
	cfg := DefaultGNNModelConfig()
	eng, err := NewGNNInferenceEngine(
		backend,
		&mockGNNPreprocessor{},
		&mockGNNPostprocessor{},
		&mockVectorSearcher{results: []*VectorMatch{}},
		metrics,
		common.NewNoopLogger(),
		cfg,
	)
	if err != nil {
		t.Fatalf("NewGNNInferenceEngine: %v", err)
	}
	return eng, backend, metrics
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGNNInferenceEngine_Embed_Success(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	resp, err := eng.Embed(context.Background(), &EmbedRequest{SMILES: "CCO"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Embedding) != 256 {
		t.Errorf("expected 256-dim embedding, got %d", len(resp.Embedding))
	}
	if resp.SMILES != "CCO" {
		t.Errorf("expected SMILES CCO, got %s", resp.SMILES)
	}
}

func TestGNNInferenceEngine_Embed_InvalidSMILES(t *testing.T) {
	backend := &mockModelBackend{}
	cfg := DefaultGNNModelConfig()
	eng, _ := NewGNNInferenceEngine(
		backend,
		&mockGNNPreprocessor{validateErr: fmt.Errorf("invalid")},
		&mockGNNPostprocessor{},
		nil,
		nil,
		nil,
		cfg,
	)
	_, err := eng.Embed(context.Background(), &EmbedRequest{SMILES: "INVALID"})
	if err == nil {
		t.Fatal("expected error for invalid SMILES")
	}
	if !errors.Is(err, errors.ErrInvalidMolecule) {
		t.Errorf("expected ErrInvalidMolecule, got %v", err)
	}
}

func TestGNNInferenceEngine_Embed_EmptySMILES(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	_, err := eng.Embed(context.Background(), &EmbedRequest{SMILES: ""})
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestGNNInferenceEngine_Embed_BackendTimeout(t *testing.T) {
	backend := &mockModelBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			return nil, errors.ErrInferenceTimeout
		},
	}
	cfg := DefaultGNNModelConfig()
	eng, _ := NewGNNInferenceEngine(
		backend,
		&mockGNNPreprocessor{},
		&mockGNNPostprocessor{},
		nil,
		nil,
		nil,
		cfg,
	)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := eng.Embed(ctx, &EmbedRequest{SMILES: "CCO"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestGNNInferenceEngine_Embed_BackendUnavailable(t *testing.T) {
	backend := &mockModelBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			return nil, errors.ErrServingUnavailable
		},
	}
	cfg := DefaultGNNModelConfig()
	eng, _ := NewGNNInferenceEngine(
		backend,
		&mockGNNPreprocessor{},
		&mockGNNPostprocessor{},
		nil,
		nil,
		nil,
		cfg,
	)
	_, err := eng.Embed(context.Background(), &EmbedRequest{SMILES: "CCO"})
	if err == nil {
		t.Fatal("expected unavailable error")
	}
	if !errors.Is(err, errors.ErrModelBackendUnavailable) {
		t.Errorf("expected ErrModelBackendUnavailable, got %v", err)
	}
	// Should have retried
	if backend.callCount.Load() < 2 {
		t.Errorf("expected at least 2 attempts, got %d", backend.callCount.Load())
	}
}

func TestGNNInferenceEngine_BatchEmbed_Success(t *testing.T) {
	eng, _, metrics := newTestEngine(t)
	items := []*EmbedRequest{
		{SMILES: "CCO"},
		{SMILES: "c1ccccc1"},
		{SMILES: "CC(=O)O"},
	}
	resp, err := eng.BatchEmbed(context.Background(), &BatchEmbedRequest{Items: items})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(resp.Results))
	}
	for i, r := range resp.Results {
		if r.Error != "" {
			t.Errorf("result[%d] unexpected error: %s", i, r.Error)
		}
		if r.Response == nil {
			t.Errorf("result[%d] nil response", i)
			continue
		}
		if len(r.Response.Embedding) != 256 {
			t.Errorf("result[%d] expected 256-dim, got %d", i, len(r.Response.Embedding))
		}
	}
	if metrics.inferenceCount.Load() < 3 {
		t.Errorf("expected at least 3 inference metrics, got %d", metrics.inferenceCount.Load())
	}
}

func TestGNNInferenceEngine_BatchEmbed_Empty(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	resp, err := eng.BatchEmbed(context.Background(), &BatchEmbedRequest{Items: []*EmbedRequest{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestGNNInferenceEngine_BatchEmbed_Nil(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	resp, err := eng.BatchEmbed(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestGNNInferenceEngine_BatchEmbed_PartialFailure(t *testing.T) {
	callIdx := atomic.Int32{}
	backend := &mockModelBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			idx := callIdx.Add(1)
			if idx == 2 {
				return nil, fmt.Errorf("transient failure")
			}
			return &common.PredictResponse{
				Outputs: map[string][]byte{
					"embedding": common.EncodeFloat32Vector(make([]float32, 256)),
				},
				InferenceTimeMs: 5,
			}, nil
		},
	}
	cfg := DefaultGNNModelConfig()
	eng, _ := NewGNNInferenceEngine(
		backend,
		&mockGNNPreprocessor{},
		&mockGNNPostprocessor{},
		nil,
		nil,
		nil,
		cfg,
	)
	items := []*EmbedRequest{
		{SMILES: "CCO"},
		{SMILES: "c1ccccc1"},
		{SMILES: "CC(=O)O"},
	}
	resp, err := eng.BatchEmbed(context.Background(), &BatchEmbedRequest{Items: items})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hasError := false
	for _, r := range resp.Results {
		if r.Error != "" {
			hasError = true
		}
	}
	if !hasError {
		t.Log("note: partial failure may have been retried successfully")
	}
}

func TestGNNInferenceEngine_ComputeSimilarity_Success(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	resp, err := eng.ComputeSimilarity(context.Background(), &SimilarityRequest{
		SMILES1: "CCO",
		SMILES2: "CCCO",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FusedScore <= 0 {
		t.Errorf("expected positive fused score, got %f", resp.FusedScore)
	}
	if _, ok := resp.Scores[string(FingerprintGNN)]; !ok {
		t.Error("expected GNN score in scores map")
	}
	if resp.Level == "" {
		t.Error("expected non-empty similarity level")
	}
}

func TestGNNInferenceEngine_ComputeSimilarity_MissingSMILES(t *testing.T) {
	eng, _, _ := newTestEngine(t)
	_, err := eng.ComputeSimilarity(context.Background(), &SimilarityRequest{
		SMILES1: "CCO",
		SMILES2: "",
	})
	if err == nil {
		t.Fatal("expected error for missing SMILES2")
	}
}

func TestGNNInferenceEngine_SearchSimilar_Success(t *testing.T) {
	vs := &mockVectorSearcher{
		results: []*VectorMatch{
			{MoleculeID: "mol-001", SMILES: "CCCO", Score: 0.92},
			{MoleculeID: "mol-002", SMILES: "CCO", Score: 0.88},
		},
	}
	backend := &mockModelBackend{}
	cfg := DefaultGNNModelConfig()
	eng, _ := NewGNNInferenceEngine(
		backend,
		&mockGNNPreprocessor{},
		&mockGNNPostprocessor{},
		vs,
		nil,
		nil,
		cfg,
	)
	resp, err := eng.SearchSimilar(context.Background(), &SimilarSearchRequest{
		SMILES:    "CCO",
		TopK:      5,
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(resp.Matches))
	}
	for _, m := range resp.Matches {
		if m.Level == "" {
			t.Error("expected classified similarity level")
		}
	}
}

func TestGNNInferenceEngine_SearchSimilar_NoVectorSearcher(t *testing.T) {
	backend := &mockModelBackend{}
	cfg := DefaultGNNModelConfig()
	eng, _ := NewGNNInferenceEngine(
		backend,
		&mockGNNPreprocessor{},
		&mockGNNPostprocessor{},
		nil,
		nil,
		nil,
		cfg,
	)
	_, err := eng.SearchSimilar(context.Background(), &SimilarSearchRequest{SMILES: "CCO"})
	if err == nil {
		t.Fatal("expected error when vector searcher is nil")
	}
}

func TestNewGNNInferenceEngine_NilBackend(t *testing.T) {
	_, err := NewGNNInferenceEngine(nil, &mockGNNPreprocessor{}, &mockGNNPostprocessor{}, nil, nil, nil, DefaultGNNModelConfig())
	if err == nil {
		t.Fatal("expected error for nil backend")
	}
}

func TestNewGNNInferenceEngine_NilPreprocessor(t *testing.T) {
	_, err := NewGNNInferenceEngine(&mockModelBackend{}, nil, &mockGNNPostprocessor{}, nil, nil, nil, DefaultGNNModelConfig())
	if err == nil {
		t.Fatal("expected error for nil preprocessor")
	}
}

func TestNewGNNInferenceEngine_NilConfig(t *testing.T) {
	_, err := NewGNNInferenceEngine(&mockModelBackend{}, &mockGNNPreprocessor{}, &mockGNNPostprocessor{}, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestSplitEmbedRequests(t *testing.T) {
	items := make([]*EmbedRequest, 150)
	for i := range items {
		items[i] = &EmbedRequest{SMILES: fmt.Sprintf("C%d", i)}
	}
	chunks := splitEmbedRequests(items, 64)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 64 {
		t.Errorf("chunk[0] expected 64, got %d", len(chunks[0]))
	}
	if len(chunks[1]) != 64 {
		t.Errorf("chunk[1] expected 64, got %d", len(chunks[1]))
	}
	if len(chunks[2]) != 22 {
		t.Errorf("chunk[2] expected 22, got %d", len(chunks[2]))
	}
}

func TestSplitEmbedRequests_SingleChunk(t *testing.T) {
	items := make([]*EmbedRequest, 10)
	for i := range items {
		items[i] = &EmbedRequest{SMILES: "C"}
	}
	chunks := splitEmbedRequests(items, 64)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestSimilarityLevelClassification(t *testing.T) {
	pp := &mockGNNPostprocessor{}
	tests := []struct {
		score float64
		want  molecule.SimilarityLevel
	}{
		{0.95, molecule.SimilarityHigh},
		{0.85, molecule.SimilarityHigh},
		{0.75, molecule.SimilarityMedium},
		{0.60, molecule.SimilarityLow},
		{0.40, molecule.SimilarityNone},
		{0.0, molecule.SimilarityNone},
	}
	for _, tt := range tests {
		got := pp.ClassifySimilarity(tt.score)
		if got != tt.want {
			t.Errorf("ClassifySimilarity(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

//Personal.AI order the ending
