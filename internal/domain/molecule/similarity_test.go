package molecule

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testEpsilon = 1e-9

func makeBitFingerprint(t *testing.T, fpType FingerprintType, bits []byte, numBits int) *Fingerprint {
	radius := 0
	if fpType == FingerprintMorgan || fpType == FingerprintFCFP {
		radius = 2
	}
	fp, err := NewBitFingerprint(fpType, bits, numBits, radius)
	if err != nil {
		t.Fatalf("failed to create bit fingerprint: %v", err)
	}
	return fp
}

func makeDenseFingerprint(t *testing.T, vector []float32) *Fingerprint {
	fp, err := NewDenseFingerprint(vector, "v1")
	if err != nil {
		t.Fatalf("failed to create dense fingerprint: %v", err)
	}
	return fp
}

func assertFloat64Near(t *testing.T, actual, expected float64) {
	if math.Abs(actual-expected) > testEpsilon {
		t.Errorf("expected %f to be near %f", actual, expected)
	}
}

func TestTanimotoCalculator(t *testing.T) {
	calc := &TanimotoCalculator{}

	t.Run("BitVector_Identical", func(t *testing.T) {
		fp1 := makeBitFingerprint(t, FingerprintMorgan, []byte{0xFF, 0xFF}, 16)
		fp2 := makeBitFingerprint(t, FingerprintMorgan, []byte{0xFF, 0xFF}, 16)
		score, _ := calc.Calculate(fp1, fp2)
		assert.Equal(t, 1.0, score)
	})

	t.Run("BitVector_HalfOverlap", func(t *testing.T) {
		fp1 := makeBitFingerprint(t, FingerprintMorgan, []byte{0x0F}, 8)
		fp2 := makeBitFingerprint(t, FingerprintMorgan, []byte{0x3F}, 8)
		// 00001111 AND 00111111 = 00001111 (4 bits)
		// 00001111 OR 00111111 = 00111111 (6 bits)
		// 4/6 = 0.666666...
		score, _ := calc.Calculate(fp1, fp2)
		assertFloat64Near(t, score, 4.0/6.0)
	})

	t.Run("DenseVector_Identical", func(t *testing.T) {
		fp1 := makeDenseFingerprint(t, []float32{1.0, 0.5, 0.0, 0.8, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0})
		fp2 := makeDenseFingerprint(t, []float32{1.0, 0.5, 0.0, 0.8, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0})
		score, _ := calc.Calculate(fp1, fp2)
		assert.Equal(t, 1.0, score)
	})

	t.Run("DenseVector_GeneralizedTanimoto", func(t *testing.T) {
		v1 := make([]float32, 32)
		v2 := make([]float32, 32)
		v1[0], v1[1], v1[2], v1[3] = 1.0, 0.5, 0.0, 0.5 // Change to exact representations in float32
		v2[0], v2[1], v2[2], v2[3] = 0.5, 0.5, 0.0, 0.5
		// min sum = 0.5 + 0.5 + 0.0 + 0.5 = 1.5
		// max sum = 1.0 + 0.5 + 0.0 + 0.5 = 2.0
		// 1.5 / 2.0 = 0.75
		fp1 := makeDenseFingerprint(t, v1)
		fp2 := makeDenseFingerprint(t, v2)
		score, _ := calc.Calculate(fp1, fp2)
		assert.InDelta(t, 0.75, score, testEpsilon)
	})
}

func TestDiceCalculator(t *testing.T) {
	calc := &DiceCalculator{}
	fp1 := makeBitFingerprint(t, FingerprintMorgan, []byte{0x0F}, 8)
	fp2 := makeBitFingerprint(t, FingerprintMorgan, []byte{0x3F}, 8)
	// intersection = 4, fp1 count = 4, fp2 count = 6
	// 2 * 4 / (4 + 6) = 0.8
	score, _ := calc.Calculate(fp1, fp2)
	assert.Equal(t, 0.8, score)
}

func TestCosineCalculator(t *testing.T) {
	calc := &CosineCalculator{}
	t.Run("Identical", func(t *testing.T) {
		fp1 := makeDenseFingerprint(t, make([]float32, 32))
		fp1.Vector[0] = 1.0
		fp2 := makeDenseFingerprint(t, make([]float32, 32))
		fp2.Vector[0] = 1.0
		score, _ := calc.Calculate(fp1, fp2)
		assert.Equal(t, 1.0, score)
	})

	t.Run("Orthogonal", func(t *testing.T) {
		fp1 := makeDenseFingerprint(t, make([]float32, 32))
		fp1.Vector[0] = 1.0
		fp2 := makeDenseFingerprint(t, make([]float32, 32))
		fp2.Vector[1] = 1.0
		score, _ := calc.Calculate(fp1, fp2)
		assert.Equal(t, 0.5, score) // (0+1)/2
	})
}

func TestEuclideanCalculator(t *testing.T) {
	calc := &EuclideanCalculator{}
	fp1 := makeDenseFingerprint(t, make([]float32, 32))
	fp2 := makeDenseFingerprint(t, make([]float32, 32))
	fp2.Vector[0] = 1.0
	// distance = 1.0
	// similarity = 1/(1+1) = 0.5
	score, _ := calc.Calculate(fp1, fp2)
	assert.Equal(t, 0.5, score)
}

func TestClassifySimilarity(t *testing.T) {
	assert.Equal(t, "identical", ClassifySimilarity(0.99))
	assert.Equal(t, "high", ClassifySimilarity(0.85))
	assert.Equal(t, "moderate", ClassifySimilarity(0.70))
	assert.Equal(t, "low", ClassifySimilarity(0.50))
	assert.Equal(t, "dissimilar", ClassifySimilarity(0.49))
}

func TestSimilaritySearchOptions_Validate(t *testing.T) {
	opts := DefaultSimilaritySearchOptions()
	assert.NoError(t, opts.Validate())

	opts.Threshold = 1.1
	assert.Error(t, opts.Validate())
}

func BenchmarkTanimoto_2048Bits(b *testing.B) {
	bits1 := make([]byte, 256)
	bits2 := make([]byte, 256)
	for i := range bits1 {
		bits1[i] = 0xAA
		bits2[i] = 0x55
	}
	fp1, _ := NewBitFingerprint(FingerprintMorgan, bits1, 2048, 2)
	fp2, _ := NewBitFingerprint(FingerprintMorgan, bits2, 2048, 2)
	calc := &TanimotoCalculator{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calc.Calculate(fp1, fp2)
	}
}

func FuzzTanimotoSymmetry(f *testing.F) {
	f.Add([]byte{0xAA, 0x55}, []byte{0xCC, 0x33})
	f.Fuzz(func(t *testing.T, a, b []byte) {
		if len(a) == 0 || len(b) == 0 {
			return
		}
		numBits := len(a) * 8
		if len(b)*8 < numBits {
			numBits = len(b) * 8
		}
		if numBits == 0 {
			return
		}
		fp1, err := NewBitFingerprint(FingerprintMorgan, a, numBits, 2)
		if err != nil {
			return
		}
		fp2, err := NewBitFingerprint(FingerprintMorgan, b, numBits, 2)
		if err != nil {
			return
		}

		calc := &TanimotoCalculator{}
		s1, _ := calc.Calculate(fp1, fp2)
		s2, _ := calc.Calculate(fp2, fp1)
		if math.Abs(s1-s2) > 1e-9 {
			t.Errorf("Tanimoto not symmetric: %f != %f", s1, s2)
		}
	})
}

//Personal.AI order the ending
