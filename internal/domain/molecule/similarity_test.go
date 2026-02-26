package molecule

import (
	"math"
	"testing"
)

func TestSimilarityMetric_IsValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		metric SimilarityMetric
		valid  bool
	}{
		{MetricTanimoto, true},
		{MetricDice, true},
		{MetricCosine, true},
		{MetricEuclidean, true},
		{MetricManhattan, true},
		{MetricSoergel, true},
		{SimilarityMetric("invalid"), false},
	}
	for _, tt := range tests {
		if got := tt.metric.IsValid(); got != tt.valid {
			t.Errorf("SimilarityMetric(%s).IsValid() = %v, want %v", tt.metric, got, tt.valid)
		}
	}
}

func makeDenseFP(t *testing.T, vec []float32) *Fingerprint {
	// Pad to 32 if needed
	if len(vec) < 32 {
		padded := make([]float32, 32)
		copy(padded, vec)
		vec = padded
	}
	fp, err := NewDenseFingerprint(vec, "v1")
	if err != nil {
		t.Fatalf("NewDenseFingerprint failed: %v", err)
	}
	return fp
}

func TestTanimotoCalculator(t *testing.T) {
	t.Parallel()
	calc := &TanimotoCalculator{}

	// Bit vectors
	// 1111 (0x0F) vs 1111 (0x0F) -> 1.0
	fp1, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x0F}, 8, 2)
	fp2, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x0F}, 8, 2)
	score, err := calc.Calculate(fp1, fp2)
	if err != nil { t.Fatalf("Calculate failed: %v", err) }
	if math.Abs(score-1.0) > testEpsilon { t.Errorf("Identical score = %f, want 1.0", score) }

	// 1111 (0x0F) vs 0000 (0x00) -> 0.0
	fp3, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x00}, 8, 2)
	score, _ = calc.Calculate(fp1, fp3)
	if math.Abs(score-0.0) > testEpsilon { t.Errorf("Disjoint score = %f, want 0.0", score) }

	// 1111 (0x0F) vs 0011 (0x03) -> 0.5 (Intersection 2, Union 4)
	fp4, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x03}, 8, 2)
	score, _ = calc.Calculate(fp1, fp4)
	if math.Abs(score-0.5) > testEpsilon { t.Errorf("Overlap score = %f, want 0.5", score) }

	// Dense vectors (Generalized Tanimoto)
	// [1.0, 0.5] vs [0.5, 1.0] (padded with zeros)
	// Min: [0.5, 0.5] + zeros -> sum=1.0
	// Max: [1.0, 1.0] + zeros -> sum=2.0
	// Score = 0.5
	dfp1 := makeDenseFP(t, []float32{1.0, 0.5})
	dfp2 := makeDenseFP(t, []float32{0.5, 1.0})

	score, err = calc.Calculate(dfp1, dfp2)
	if err != nil { t.Fatalf("Calculate dense failed: %v", err) }
	if math.Abs(score-0.5) > testEpsilon { t.Errorf("Dense score = %f, want 0.5", score) }
}

func TestDiceCalculator(t *testing.T) {
	t.Parallel()
	calc := &DiceCalculator{}

	// 1111 (0x0F) vs 0011 (0x03)
	// Intersection = 2
	// Pop1 = 4, Pop2 = 2
	// Dice = 2*2 / (4+2) = 4/6 = 0.666...
	fp1, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x0F}, 8, 2)
	fp2, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x03}, 8, 2)

	score, err := calc.Calculate(fp1, fp2)
	if err != nil { t.Fatalf("Calculate failed: %v", err) }
	if math.Abs(score-0.6666666667) > testEpsilon { t.Errorf("Dice score = %f, want ~0.667", score) }

	// Dense not supported
	// use makeDenseFP to get a valid dense FP to test rejection
	denseFP := makeDenseFP(t, []float32{1.0})
	_, err = calc.Calculate(denseFP, denseFP)
	if err == nil { t.Error("Dice calculated on dense vector unexpectedly") }
}

func TestCosineCalculator(t *testing.T) {
	t.Parallel()
	calc := &CosineCalculator{}

	// [1, 0] vs [0, 1] -> Orthogonal -> 0.0 -> Normalized (0+1)/2 = 0.5
	// padded with zeros
	dfpA := makeDenseFP(t, []float32{1.0, 0.0})
	dfpB := makeDenseFP(t, []float32{0.0, 1.0})

	score, err := calc.Calculate(dfpA, dfpB)
	if err != nil { t.Fatalf("Calculate failed: %v", err) }
	if math.Abs(score-0.5) > testEpsilon { t.Errorf("Orthogonal score = %f, want 0.5", score) }

	// Identical -> 1.0 -> Normalized 1.0
	score, _ = calc.Calculate(dfpA, dfpA)
	if math.Abs(score-1.0) > testEpsilon { t.Errorf("Identical score = %f, want 1.0", score) }

	// Anti-parallel -> -1.0 -> Normalized 0.0
	// Need negative value.
	dfpC := makeDenseFP(t, []float32{-1.0, 0.0})

	score, _ = calc.Calculate(dfpA, dfpC)
	if math.Abs(score-0.0) > testEpsilon { t.Errorf("Anti-parallel score = %f, want 0.0", score) }
}

func TestEuclideanCalculator(t *testing.T) {
	t.Parallel()
	calc := &EuclideanCalculator{}

	// Identical -> dist 0 -> score 1.0
	dfpA := makeDenseFP(t, []float32{0.0})
	score, err := calc.Calculate(dfpA, dfpA)
	if err != nil { t.Fatalf("Calculate failed: %v", err) }
	if math.Abs(score-1.0) > testEpsilon { t.Errorf("Identical score = %f, want 1.0", score) }

	// Dist 1.0 -> score 0.5
	// [0,0...] vs [1,0...]
	dfpB := makeDenseFP(t, []float32{1.0})

	score, _ = calc.Calculate(dfpA, dfpB)
	// dist = sqrt((0-1)^2) = 1.0
	// score = 1/(1+1) = 0.5
	if math.Abs(score-0.5) > testEpsilon { t.Errorf("Dist 1 score = %f, want 0.5", score) }
}

func TestDefaultSimilarityEngine(t *testing.T) {
	t.Parallel()
	engine := NewDefaultSimilarityEngine()

	fp1, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x0F}, 8, 2)
	fp2, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x0F}, 8, 2)

	score, err := engine.ComputeSimilarity(fp1, fp2, MetricTanimoto)
	if err != nil { t.Fatalf("ComputeSimilarity failed: %v", err) }
	if math.Abs(score-1.0) > testEpsilon { t.Errorf("Score = %f, want 1.0", score) }

	// Unsupported metric (if removed from map or added new enum but not init)
	_, err = engine.ComputeSimilarity(fp1, fp2, MetricManhattan)
	if err == nil { t.Error("ComputeSimilarity allowed unsupported metric") }

	// Batch
	scores, err := engine.BatchComputeSimilarity(fp1, []*Fingerprint{fp2, fp2}, MetricTanimoto)
	if err != nil { t.Fatalf("BatchComputeSimilarity failed: %v", err) }
	if len(scores) != 2 || scores[0] != 1.0 { t.Error("BatchComputeSimilarity results incorrect") }
}

func TestClassifySimilarity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		score float64
		want  string
	}{
		{1.0, "identical"},
		{0.99, "identical"},
		{0.90, "high"},
		{0.85, "high"},
		{0.80, "moderate"},
		{0.70, "moderate"},
		{0.60, "low"},
		{0.50, "low"},
		{0.40, "dissimilar"},
		{0.0, "dissimilar"},
	}
	for _, tt := range tests {
		if got := ClassifySimilarity(tt.score); got != tt.want {
			t.Errorf("ClassifySimilarity(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

// Fuzz Tests
func FuzzTanimotoSymmetry(f *testing.F) {
	// Seed corpus
	f.Add([]byte{0xFF}, []byte{0x00})
	f.Add([]byte{0xAA}, []byte{0x55})

	calc := &TanimotoCalculator{}

	f.Fuzz(func(t *testing.T, b1 []byte, b2 []byte) {
		if len(b1) == 0 || len(b2) == 0 || len(b1) != len(b2) {
			return // Skip invalid inputs for fuzzing
		}

		// Create fingerprints
		// Use dummy type and bit length
		fp1 := &Fingerprint{Type: FingerprintMorgan, Encoding: EncodingBitVector, Bits: b1, NumBits: len(b1)*8}
		fp2 := &Fingerprint{Type: FingerprintMorgan, Encoding: EncodingBitVector, Bits: b2, NumBits: len(b2)*8}

		s1, err1 := calc.Calculate(fp1, fp2)
		s2, err2 := calc.Calculate(fp2, fp1)

		if err1 != nil || err2 != nil {
			return // Skip errors
		}

		if math.Abs(s1-s2) > testEpsilon {
			t.Errorf("Tanimoto asymmetry: %f != %f", s1, s2)
		}
		if s1 < 0.0 || s1 > 1.0 {
			t.Errorf("Tanimoto out of bounds: %f", s1)
		}
	})
}
//Personal.AI order the ending
