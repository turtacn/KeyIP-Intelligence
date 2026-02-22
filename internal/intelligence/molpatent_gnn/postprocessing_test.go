package molpatent_gnn

import (
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

func newTestPostprocessor() GNNPostprocessor {
	return NewGNNPostprocessor(DefaultPostprocessorConfig())
}

// ---------------------------------------------------------------------------
// L2 Normalisation — ProcessEmbedding
// ---------------------------------------------------------------------------

func TestProcessEmbedding_KnownVector(t *testing.T) {
	pp := newTestPostprocessor()

	// Vector [3, 4, 0, 0, ...] padded to 256 dims.
	// L2 norm = 5.0, normalised = [0.6, 0.8, 0, 0, ...]
	raw := make([]float32, 256)
	raw[0] = 3.0
	raw[1] = 4.0

	res, err := pp.ProcessEmbedding(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !almostEqual(res.L2Norm, 5.0, 1e-6) {
		t.Errorf("L2Norm: expected 5.0, got %f", res.L2Norm)
	}
	if !almostEqual(float64(res.NormalizedVector[0]), 0.6, 1e-6) {
		t.Errorf("normalised[0]: expected 0.6, got %f", res.NormalizedVector[0])
	}
	if !almostEqual(float64(res.NormalizedVector[1]), 0.8, 1e-6) {
		t.Errorf("normalised[1]: expected 0.8, got %f", res.NormalizedVector[1])
	}

	// Verify the normalised vector has unit norm.
	normAfter := l2Norm(res.NormalizedVector)
	if !almostEqual(normAfter, 1.0, 1e-6) {
		t.Errorf("normalised vector norm: expected 1.0, got %f", normAfter)
	}
}

func TestProcessEmbedding_AllOnes(t *testing.T) {
	pp := newTestPostprocessor()
	raw := make([]float32, 256)
	for i := range raw {
		raw[i] = 1.0
	}

	res, err := pp.ProcessEmbedding(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedNorm := math.Sqrt(256.0)
	if !almostEqual(res.L2Norm, expectedNorm, 1e-6) {
		t.Errorf("L2Norm: expected %f, got %f", expectedNorm, res.L2Norm)
	}

	expectedVal := 1.0 / expectedNorm
	if !almostEqual(float64(res.NormalizedVector[0]), expectedVal, 1e-6) {
		t.Errorf("normalised[0]: expected %f, got %f", expectedVal, res.NormalizedVector[0])
	}
}

func TestProcessEmbedding_ZeroVector(t *testing.T) {
	pp := newTestPostprocessor()
	raw := make([]float32, 256)

	_, err := pp.ProcessEmbedding(raw, nil)
	if err == nil {
		t.Fatal("expected error for zero vector")
	}
}

func TestProcessEmbedding_EmptyVector(t *testing.T) {
	pp := newTestPostprocessor()
	_, err := pp.ProcessEmbedding([]float32{}, nil)
	if err == nil {
		t.Fatal("expected error for empty vector")
	}
}

func TestProcessEmbedding_DimensionMismatch(t *testing.T) {
	pp := newTestPostprocessor()
	raw := make([]float32, 128) // config expects 256
	raw[0] = 1.0

	_, err := pp.ProcessEmbedding(raw, nil)
	if err == nil {
		t.Fatal("expected error for dimension mismatch")
	}
}

func TestProcessEmbedding_NoDimCheck(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{
		ExpectedDim:     0, // skip dimension check
		ThresholdHigh:   0.85,
		ThresholdMedium: 0.70,
		ThresholdLow:    0.55,
	})
	raw := []float32{1.0, 0.0, 0.0}
	res, err := pp.ProcessEmbedding(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(float64(res.NormalizedVector[0]), 1.0, 1e-6) {
		t.Errorf("expected 1.0, got %f", res.NormalizedVector[0])
	}
}

func TestProcessEmbedding_NegativeValues(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{ExpectedDim: 0})
	raw := []float32{-3.0, 4.0}
	res, err := pp.ProcessEmbedding(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(res.L2Norm, 5.0, 1e-6) {
		t.Errorf("L2Norm: expected 5.0, got %f", res.L2Norm)
	}
	if !almostEqual(float64(res.NormalizedVector[0]), -0.6, 1e-6) {
		t.Errorf("normalised[0]: expected -0.6, got %f", res.NormalizedVector[0])
	}
}

func TestProcessEmbedding_DoesNotMutateInput(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{ExpectedDim: 0})
	raw := []float32{3.0, 4.0}
	original0 := raw[0]
	original1 := raw[1]

	_, err := pp.ProcessEmbedding(raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if raw[0] != original0 || raw[1] != original1 {
		t.Error("ProcessEmbedding mutated the input slice")
	}
}

// ---------------------------------------------------------------------------
// ProcessBatchEmbedding
// ---------------------------------------------------------------------------

func TestProcessBatchEmbedding_Success(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{ExpectedDim: 0})
	batch := [][]float32{
		{1.0, 0.0},
		{0.0, 2.0},
		{3.0, 4.0},
	}
	results, err := pp.ProcessBatchEmbedding(batch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		normAfter := l2Norm(r.NormalizedVector)
		if !almostEqual(normAfter, 1.0, 1e-6) {
			t.Errorf("result[%d] norm: expected 1.0, got %f", i, normAfter)
		}
	}
}

func TestProcessBatchEmbedding_Empty(t *testing.T) {
	pp := newTestPostprocessor()
	results, err := pp.ProcessBatchEmbedding([][]float32{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestProcessBatchEmbedding_PropagatesError(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{ExpectedDim: 0})
	batch := [][]float32{
		{1.0, 0.0},
		{0.0, 0.0}, // zero vector → error
	}
	_, err := pp.ProcessBatchEmbedding(batch, nil)
	if err == nil {
		t.Fatal("expected error for zero vector in batch")
	}
}

// ---------------------------------------------------------------------------
// Cosine Similarity
// ---------------------------------------------------------------------------

func TestCosineSimilarity_IdenticalVectors(t *testing.T) {
	pp := newTestPostprocessor()
	v := []float32{1.0, 2.0, 3.0}
	sim, err := pp.ComputeCosineSimilarity(v, v)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(sim, 1.0, 1e-9) {
		t.Errorf("expected 1.0, got %f", sim)
	}
}

func TestCosineSimilarity_OrthogonalVectors(t *testing.T) {
	pp := newTestPostprocessor()
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{0.0, 1.0, 0.0}
	sim, err := pp.ComputeCosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(sim, 0.0, 1e-9) {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func TestCosineSimilarity_OppositeVectors(t *testing.T) {
	pp := newTestPostprocessor()
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{-1.0, -2.0, -3.0}
	sim, err := pp.ComputeCosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(sim, -1.0, 1e-9) {
		t.Errorf("expected -1.0, got %f", sim)
	}
}

func TestCosineSimilarity_KnownAngle(t *testing.T) {
	pp := newTestPostprocessor()
	// 45-degree angle in 2D: cos(45°) ≈ 0.7071
	a := []float32{1.0, 0.0}
	b := []float32{1.0, 1.0}
	sim, err := pp.ComputeCosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 1.0 / math.Sqrt(2.0)
	if !almostEqual(sim, expected, 1e-6) {
		t.Errorf("expected %f, got %f", expected, sim)
	}
}

func TestCosineSimilarity_NormalisedVectors(t *testing.T) {
	// After L2 normalisation, cosine sim should equal dot product.
	pp := NewGNNPostprocessor(&PostprocessorConfig{ExpectedDim: 0})
	rawA := []float32{3.0, 4.0}
	rawB := []float32{1.0, 0.0}

	resA, _ := pp.ProcessEmbedding(rawA, nil)
	resB, _ := pp.ProcessEmbedding(rawB, nil)

	sim, err := pp.ComputeCosineSimilarity(resA.NormalizedVector, resB.NormalizedVector)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dot := dotProduct(resA.NormalizedVector, resB.NormalizedVector)
	if !almostEqual(sim, dot, 1e-6) {
		t.Errorf("cosine sim (%f) should equal dot product (%f) for unit vectors", sim, dot)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	pp := newTestPostprocessor()
	a := []float32{0.0, 0.0, 0.0}
	b := []float32{1.0, 2.0, 3.0}
	_, err := pp.ComputeCosineSimilarity(a, b)
	if err == nil {
		t.Fatal("expected error for zero vector")
	}
}

func TestCosineSimilarity_DimensionMismatch(t *testing.T) {
	pp := newTestPostprocessor()
	a := []float32{1.0, 2.0}
	b := []float32{1.0, 2.0, 3.0}
	_, err := pp.ComputeCosineSimilarity(a, b)
	if err == nil {
		t.Fatal("expected error for dimension mismatch")
	}
}

func TestCosineSimilarity_EmptyVectors(t *testing.T) {
	pp := newTestPostprocessor()
	_, err := pp.ComputeCosineSimilarity([]float32{}, []float32{})
	if err == nil {
		t.Fatal("expected error for empty vectors")
	}
}

// ---------------------------------------------------------------------------
// Tanimoto Similarity
// ---------------------------------------------------------------------------

func TestTanimotoSimilarity_Identical(t *testing.T) {
	pp := newTestPostprocessor()
	fp := []byte{0xFF, 0xAA, 0x55}
	sim, err := pp.ComputeTanimotoSimilarity(fp, fp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(sim, 1.0, 1e-9) {
		t.Errorf("expected 1.0, got %f", sim)
	}
}

func TestTanimotoSimilarity_Disjoint(t *testing.T) {
	pp := newTestPostprocessor()
	a := []byte{0xF0, 0x00} // bits: 11110000 00000000
	b := []byte{0x0F, 0x00} // bits: 00001111 00000000
	sim, err := pp.ComputeTanimotoSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// popA=4, popB=4, popAB=0 → T = 0 / (4+4-0) = 0
	if !almostEqual(sim, 0.0, 1e-9) {
		t.Errorf("expected 0.0, got %f", sim)
	}
}

func TestTanimotoSimilarity_KnownValue(t *testing.T) {
	pp := newTestPostprocessor()
	// A = 11110000 → popA = 4
	// B = 11100000 → popB = 3
	// A&B = 11100000 → popAB = 3
	// T = 3 / (4 + 3 - 3) = 3/4 = 0.75
	a := []byte{0xF0}
	b := []byte{0xE0}
	sim, err := pp.ComputeTanimotoSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(sim, 0.75, 1e-9) {
		t.Errorf("expected 0.75, got %f", sim)
	}
}

func TestTanimotoSimilarity_BothZero(t *testing.T) {
	pp := newTestPostprocessor()
	a := []byte{0x00, 0x00}
	b := []byte{0x00, 0x00}
	sim, err := pp.ComputeTanimotoSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(sim, 0.0, 1e-9) {
		t.Errorf("expected 0.0 for all-zero fingerprints, got %f", sim)
	}
}

func TestTanimotoSimilarity_LengthMismatch(t *testing.T) {
	pp := newTestPostprocessor()
	a := []byte{0xFF}
	b := []byte{0xFF, 0x00}
	_, err := pp.ComputeTanimotoSimilarity(a, b)
	if err == nil {
		t.Fatal("expected error for length mismatch")
	}
}

func TestTanimotoSimilarity_Empty(t *testing.T) {
	pp := newTestPostprocessor()
	_, err := pp.ComputeTanimotoSimilarity([]byte{}, []byte{0xFF})
	if err == nil {
		t.Fatal("expected error for empty fingerprint")
	}
}

func TestTanimotoSimilarity_PartialOverlap(t *testing.T) {
	pp := newTestPostprocessor()
	// A = 11111111 00000000 → popA = 8
	// B = 11110000 11110000 → popB = 8
	// A&B = 11110000 00000000 → popAB = 4
	// T = 4 / (8 + 8 - 4) = 4/12 = 1/3 ≈ 0.3333
	a := []byte{0xFF, 0x00}
	b := []byte{0xF0, 0xF0}
	sim, err := pp.ComputeTanimotoSimilarity(a, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 1.0 / 3.0
	if !almostEqual(sim, expected, 1e-9) {
		t.Errorf("expected %f, got %f", expected, sim)
	}
}

// ---------------------------------------------------------------------------
// FuseScores — multi-fingerprint weighted fusion
// ---------------------------------------------------------------------------

func TestFuseScores_AllPresent(t *testing.T) {
	pp := newTestPostprocessor()
	scores := map[string]float64{
		"morgan":   0.80,
		"rdkit":    0.70,
		"atompair": 0.60,
		"gnn":      0.90,
	}
	weights := map[string]float64{
		"morgan":   0.30,
		"rdkit":    0.20,
		"atompair": 0.15,
		"gnn":      0.35,
	}
	// Weighted sum = 0.30*0.80 + 0.20*0.70 + 0.15*0.60 + 0.35*0.90
	//             = 0.24 + 0.14 + 0.09 + 0.315 = 0.785
	// Weight sum  = 1.0
	// Fused       = 0.785 / 1.0 = 0.785
	fused, err := pp.FuseScores(scores, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(fused, 0.785, 1e-6) {
		t.Errorf("expected 0.785, got %f", fused)
	}
}

func TestFuseScores_MissingFingerprint_Renormalize(t *testing.T) {
	pp := newTestPostprocessor()
	// Only GNN and Morgan available; rdkit and atompair missing.
	scores := map[string]float64{
		"morgan": 0.80,
		"gnn":    0.90,
	}
	weights := map[string]float64{
		"morgan":   0.30,
		"rdkit":    0.20,
		"atompair": 0.15,
		"gnn":      0.35,
	}
	// Effective weights: morgan=0.30, gnn=0.35, sum=0.65
	// Weighted sum = 0.30*0.80 + 0.35*0.90 = 0.24 + 0.315 = 0.555
	// Fused = 0.555 / 0.65 ≈ 0.853846
	fused, err := pp.FuseScores(scores, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := 0.555 / 0.65
	if !almostEqual(fused, expected, 1e-6) {
		t.Errorf("expected %f, got %f", expected, fused)
	}
}

func TestFuseScores_SingleScore(t *testing.T) {
	pp := newTestPostprocessor()
	scores := map[string]float64{
		"gnn": 0.92,
	}
	weights := map[string]float64{
		"gnn": 0.35,
	}
	// Only one score: fused = 0.35*0.92 / 0.35 = 0.92
	fused, err := pp.FuseScores(scores, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(fused, 0.92, 1e-9) {
		t.Errorf("expected 0.92, got %f", fused)
	}
}

func TestFuseScores_ScoreWithNoWeight(t *testing.T) {
	pp := newTestPostprocessor()
	// Score key "custom" has no weight → skipped.
	scores := map[string]float64{
		"custom": 0.99,
		"gnn":    0.80,
	}
	weights := map[string]float64{
		"gnn": 1.0,
	}
	fused, err := pp.FuseScores(scores, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only gnn contributes: 1.0*0.80 / 1.0 = 0.80
	if !almostEqual(fused, 0.80, 1e-9) {
		t.Errorf("expected 0.80, got %f", fused)
	}
}

func TestFuseScores_NoMatchingWeights(t *testing.T) {
	pp := newTestPostprocessor()
	scores := map[string]float64{
		"custom": 0.99,
	}
	weights := map[string]float64{
		"gnn": 1.0,
	}
	_, err := pp.FuseScores(scores, weights)
	if err == nil {
		t.Fatal("expected error when no weights match any score")
	}
}

func TestFuseScores_EmptyScores(t *testing.T) {
	pp := newTestPostprocessor()
	_, err := pp.FuseScores(map[string]float64{}, map[string]float64{"gnn": 1.0})
	if err == nil {
		t.Fatal("expected error for empty scores")
	}
}

func TestFuseScores_EmptyWeights(t *testing.T) {
	pp := newTestPostprocessor()
	_, err := pp.FuseScores(map[string]float64{"gnn": 0.9}, map[string]float64{})
	if err == nil {
		t.Fatal("expected error for empty weights")
	}
}

func TestFuseScores_NegativeWeight(t *testing.T) {
	pp := newTestPostprocessor()
	scores := map[string]float64{"gnn": 0.9}
	weights := map[string]float64{"gnn": -0.5}
	_, err := pp.FuseScores(scores, weights)
	if err == nil {
		t.Fatal("expected error for negative weight")
	}
}

func TestFuseScores_ClampAboveOne(t *testing.T) {
	pp := newTestPostprocessor()
	// Artificially create a score > 1.0 to test clamping.
	scores := map[string]float64{
		"gnn": 1.5,
	}
	weights := map[string]float64{
		"gnn": 1.0,
	}
	fused, err := pp.FuseScores(scores, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fused != 1.0 {
		t.Errorf("expected clamped to 1.0, got %f", fused)
	}
}

func TestFuseScores_EqualWeights(t *testing.T) {
	pp := newTestPostprocessor()
	scores := map[string]float64{
		"morgan": 0.60,
		"gnn":    0.80,
	}
	weights := map[string]float64{
		"morgan": 0.50,
		"gnn":    0.50,
	}
	// (0.50*0.60 + 0.50*0.80) / 1.0 = 0.70
	fused, err := pp.FuseScores(scores, weights)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEqual(fused, 0.70, 1e-9) {
		t.Errorf("expected 0.70, got %f", fused)
	}
}

// ---------------------------------------------------------------------------
// ClassifySimilarity — threshold boundary tests
// ---------------------------------------------------------------------------

func TestClassifySimilarity_Levels(t *testing.T) {
	pp := newTestPostprocessor()
	tests := []struct {
		score float64
		want  SimilarityLevel
	}{
		{1.00, SimilarityHigh},
		{0.95, SimilarityHigh},
		{0.85, SimilarityHigh},   // exact boundary
		{0.849, SimilarityMedium},
		{0.75, SimilarityMedium},
		{0.70, SimilarityMedium}, // exact boundary
		{0.699, SimilarityLow},
		{0.60, SimilarityLow},
		{0.55, SimilarityLow},    // exact boundary
		{0.549, SimilarityNone},
		{0.30, SimilarityNone},
		{0.00, SimilarityNone},
		{-0.10, SimilarityNone},
	}
	for _, tt := range tests {
		got := pp.ClassifySimilarity(tt.score)
		if got != tt.want {
			t.Errorf("ClassifySimilarity(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestClassifySimilarity_CustomThresholds(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{
		ThresholdHigh:   0.90,
		ThresholdMedium: 0.75,
		ThresholdLow:    0.60,
	})
	tests := []struct {
		score float64
		want  SimilarityLevel
	}{
		{0.95, SimilarityHigh},
		{0.90, SimilarityHigh},
		{0.89, SimilarityMedium},
		{0.75, SimilarityMedium},
		{0.74, SimilarityLow},
		{0.60, SimilarityLow},
		{0.59, SimilarityNone},
	}
	for _, tt := range tests {
		got := pp.ClassifySimilarity(tt.score)
		if got != tt.want {
			t.Errorf("ClassifySimilarity(%f) = %s, want %s (custom thresholds)", tt.score, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// l2Norm helper
// ---------------------------------------------------------------------------

func TestL2Norm_KnownValues(t *testing.T) {
	tests := []struct {
		v    []float32
		want float64
	}{
		{[]float32{3.0, 4.0}, 5.0},
		{[]float32{1.0, 0.0, 0.0}, 1.0},
		{[]float32{0.0, 0.0, 0.0}, 0.0},
		{[]float32{1.0, 1.0, 1.0, 1.0}, 2.0},
		{[]float32{-3.0, 4.0}, 5.0},
	}
	for _, tt := range tests {
		got := l2Norm(tt.v)
		if !almostEqual(got, tt.want, 1e-9) {
			t.Errorf("l2Norm(%v) = %f, want %f", tt.v, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// dotProduct helper
// ---------------------------------------------------------------------------

func TestDotProduct_KnownValues(t *testing.T) {
	tests := []struct {
		a, b []float32
		want float64
	}{
		{[]float32{1, 0}, []float32{0, 1}, 0.0},
		{[]float32{1, 2, 3}, []float32{4, 5, 6}, 32.0},
		{[]float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{[]float32{-1, 2}, []float32{3, -4}, -11.0},
	}
	for _, tt := range tests {
		got := dotProduct(tt.a, tt.b)
		if !almostEqual(got, tt.want, 1e-9) {
			t.Errorf("dotProduct(%v, %v) = %f, want %f", tt.a, tt.b, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration: Embed → Cosine pipeline
// ---------------------------------------------------------------------------

func TestIntegration_EmbedThenCosine(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{ExpectedDim: 0})

	rawA := []float32{1.0, 2.0, 3.0}
	rawB := []float32{1.0, 2.0, 3.0}

	resA, err := pp.ProcessEmbedding(rawA, nil)
	if err != nil {
		t.Fatalf("embed A: %v", err)
	}
	resB, err := pp.ProcessEmbedding(rawB, nil)
	if err != nil {
		t.Fatalf("embed B: %v", err)
	}

	sim, err := pp.ComputeCosineSimilarity(resA.NormalizedVector, resB.NormalizedVector)
	if err != nil {
		t.Fatalf("cosine: %v", err)
	}
	if !almostEqual(sim, 1.0, 1e-9) {
		t.Errorf("identical vectors after normalisation should have sim=1.0, got %f", sim)
	}
}

func TestIntegration_EmbedThenClassify(t *testing.T) {
	pp := NewGNNPostprocessor(&PostprocessorConfig{
		ExpectedDim:     0,
		ThresholdHigh:   0.85,
		ThresholdMedium: 0.70,
		ThresholdLow:    0.55,
	})

	rawA := []float32{1.0, 0.0}
	rawB := []float32{0.7071, 0.7071} // ~45 degrees

	resA, _ := pp.ProcessEmbedding(rawA, nil)
	resB, _ := pp.ProcessEmbedding(rawB, nil)

	sim, _ := pp.ComputeCosineSimilarity(resA.NormalizedVector, resB.NormalizedVector)
	level := pp.ClassifySimilarity(sim)

	// cos(45°) ≈ 0.7071 → MEDIUM
	if level != SimilarityMedium {
		t.Errorf("expected MEDIUM for cos(45°)≈0.707, got %s (sim=%f)", level, sim)
	}
}

//Personal.AI order the ending
