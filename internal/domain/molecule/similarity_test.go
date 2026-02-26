package molecule

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimilarityMetric_IsValid(t *testing.T) {
	assert.True(t, MetricTanimoto.IsValid())
	assert.True(t, MetricDice.IsValid())
	assert.True(t, MetricCosine.IsValid())
	assert.True(t, MetricEuclidean.IsValid())
	assert.True(t, MetricManhattan.IsValid())
	assert.True(t, MetricSoergel.IsValid())
	assert.False(t, SimilarityMetric("invalid").IsValid())
}

func TestSimilarityMetric_String(t *testing.T) {
	assert.Equal(t, "tanimoto", MetricTanimoto.String())
}

func TestParseSimilarityMetric(t *testing.T) {
	m, err := ParseSimilarityMetric("dice")
	assert.NoError(t, err)
	assert.Equal(t, MetricDice, m)

	_, err = ParseSimilarityMetric("invalid")
	assert.Error(t, err)
}

func TestTanimotoCalculator_Calculate(t *testing.T) {
	calc := &TanimotoCalculator{}
	assert.Equal(t, MetricTanimoto, calc.Metric())
	assert.True(t, calc.SupportsEncoding(EncodingBitVector))

	tests := []struct {
		name    string
		fp1     *Fingerprint
		fp2     *Fingerprint
		want    float64
		wantErr bool
	}{
		{
			name: "identical_bits",
			fp1:  mustBitFP([]byte{0xFF, 0xFF}),
			fp2:  mustBitFP([]byte{0xFF, 0xFF}),
			want: 1.0,
		},
		{
			name: "completely_different",
			fp1:  mustBitFP([]byte{0xF0, 0xF0}),
			fp2:  mustBitFP([]byte{0x0F, 0x0F}),
			want: 0.0,
		},
		{
			name: "half_overlap",
			fp1:  mustBitFP([]byte{0xFF, 0x00}), // 8 bits set
			fp2:  mustBitFP([]byte{0xFF, 0xFF}), // 16 bits set
			// And = 8 bits. Or = 16 bits. 8/16 = 0.5
			want: 0.5,
		},
		{
			name: "both_zero",
			fp1:  mustBitFP([]byte{0x00}),
			fp2:  mustBitFP([]byte{0x00}),
			want: 0.0,
		},
		{
			name: "different_types",
			fp1:  mustBitFP([]byte{0xFF}), // Morgan
			fp2:  mustBitFPMACCS([]byte{0xFF}), // MACCS - actually hard to construct valid MACCS with just 1 byte, but assume type check
			wantErr: true,
		},
		{
			name: "dense_identical",
			fp1: mustDenseFP(makeVec(32, 0.5), "v1"),
			fp2: mustDenseFP(makeVec(32, 0.5), "v1"),
			want: 1.0,
		},
		{
			name: "dense_zero",
			fp1: mustDenseFP(makeVec(32, 0.0), "v1"),
			fp2: mustDenseFP(makeVec(32, 0.0), "v1"),
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calc.Calculate(tt.fp1, tt.fp2)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.InDelta(t, tt.want, got, testEpsilon)
			}
		})
	}
}

func TestDiceCalculator_Calculate(t *testing.T) {
	calc := &DiceCalculator{}
	assert.Equal(t, MetricDice, calc.Metric())

	fp1 := mustBitFP([]byte{0xFF, 0x00}) // 8 bits
	fp2 := mustBitFP([]byte{0xFF, 0xFF}) // 16 bits
	// And = 8. Dice = 2*8 / (8+16) = 16/24 = 2/3 = 0.6666
	got, err := calc.Calculate(fp1, fp2)
	assert.NoError(t, err)
	assert.InDelta(t, 2.0/3.0, got, testEpsilon)

	// Dice >= Tanimoto check
	tanimotoCalc := &TanimotoCalculator{}
	tanimoto, _ := tanimotoCalc.Calculate(fp1, fp2) // 0.5
	assert.True(t, got >= tanimoto)
}

func TestCosineCalculator_Calculate(t *testing.T) {
	calc := &CosineCalculator{}
	assert.Equal(t, MetricCosine, calc.Metric())

	// Orthogonal
	// 32 dim vector
	v1 := makeVec(32, 0.0)
	v2 := makeVec(32, 0.0)
	v1[0] = 1.0
	v2[1] = 1.0
	fp1 := mustDenseFP(v1, "v1")
	fp2 := mustDenseFP(v2, "v1")
	got, err := calc.Calculate(fp1, fp2)
	assert.NoError(t, err)
	// Cosine = 0. Normalized = (0+1)/2 = 0.5
	assert.InDelta(t, 0.5, got, testEpsilon)

	// Parallel
	v3 := makeVec(32, 1.0)
	v4 := makeVec(32, 2.0)
	fp3 := mustDenseFP(v3, "v1")
	fp4 := mustDenseFP(v4, "v1")
	got, err = calc.Calculate(fp3, fp4)
	assert.NoError(t, err)
	// Cosine = 1. Normalized = 1.0
	assert.InDelta(t, 1.0, got, testEpsilon)
}

func TestEuclideanCalculator_Calculate(t *testing.T) {
	calc := &EuclideanCalculator{}
	v1 := makeVec(32, 0.0)
	v2 := makeVec(32, 0.0)
	v2[0] = 1.0 // Distance = 1.0
	fp1 := mustDenseFP(v1, "v1")
	fp2 := mustDenseFP(v2, "v1")
	// Dist = 1. Sim = 1/(1+1) = 0.5
	got, err := calc.Calculate(fp1, fp2)
	assert.NoError(t, err)
	assert.InDelta(t, 0.5, got, testEpsilon)

	// Identical
	got, err = calc.Calculate(fp1, fp1)
	assert.NoError(t, err)
	assert.InDelta(t, 1.0, got, testEpsilon)
}

func makeVec(size int, val float32) []float32 {
	v := make([]float32, size)
	for i := range v {
		v[i] = val
	}
	return v
}

func TestNewSimilarityCalculator(t *testing.T) {
	c, err := NewSimilarityCalculator(MetricTanimoto)
	assert.NoError(t, err)
	assert.IsType(t, &TanimotoCalculator{}, c)

	_, err = NewSimilarityCalculator(SimilarityMetric("invalid"))
	assert.Error(t, err)
}

func TestDefaultSimilarityEngine_Methods(t *testing.T) {
	engine := NewDefaultSimilarityEngine()
	fp1 := mustBitFP([]byte{0xFF})
	fp2 := mustBitFP([]byte{0x0F})

	// ComputeSimilarity
	score, err := engine.ComputeSimilarity(fp1, fp2, MetricTanimoto)
	assert.NoError(t, err)
	assert.InDelta(t, 0.5, score, testEpsilon)

	// BatchComputeSimilarity
	candidates := []*Fingerprint{fp1, fp2}
	scores, err := engine.BatchComputeSimilarity(fp1, candidates, MetricTanimoto)
	assert.NoError(t, err)
	assert.Len(t, scores, 2)
	assert.InDelta(t, 1.0, scores[0], testEpsilon)
	assert.InDelta(t, 0.5, scores[1], testEpsilon)

	// Not implemented methods
	_, err = engine.SearchSimilar(context.Background(), fp1, MetricTanimoto, 0.7, 10)
	assert.Error(t, err)

	_, err = engine.RankBySimilarity(context.Background(), fp1, []string{"id"}, MetricTanimoto)
	assert.Error(t, err)
}

func TestClassifySimilarity(t *testing.T) {
	assert.Equal(t, "identical", ClassifySimilarity(1.0))
	assert.Equal(t, "identical", ClassifySimilarity(0.99))
	assert.Equal(t, "high", ClassifySimilarity(0.98))
	assert.Equal(t, "high", ClassifySimilarity(0.85))
	assert.Equal(t, "moderate", ClassifySimilarity(0.84))
	assert.Equal(t, "moderate", ClassifySimilarity(0.70))
	assert.Equal(t, "low", ClassifySimilarity(0.69))
	assert.Equal(t, "low", ClassifySimilarity(0.50))
	assert.Equal(t, "dissimilar", ClassifySimilarity(0.49))
	assert.Equal(t, "dissimilar", ClassifySimilarity(0.0))
}

// Helpers
func mustBitFP(bits []byte) *Fingerprint {
	fp, _ := NewBitFingerprint(FingerprintMorgan, bits, len(bits)*8, 2)
	return fp
}

func mustBitFPMACCS(bits []byte) *Fingerprint {
	// Hack to create MACCS for testing type mismatch, ignoring length check if possible or mock it
	// But NewBitFingerprint enforces 166 bits for MACCS.
	// So let's just manually construct struct to test logic
	return &Fingerprint{
		Type:     FingerprintMACCS,
		Encoding: EncodingBitVector,
		Bits:     bits,
		NumBits:  len(bits) * 8,
	}
}

func mustDenseFP(vec []float32, ver string) *Fingerprint {
	fp, _ := NewDenseFingerprint(vec, ver)
	return fp
}

//Personal.AI order the ending
