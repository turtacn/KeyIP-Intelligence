package molpatent_gnn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessEmbedding_Success(t *testing.T) {
	processor := NewGNNPostprocessor(&GNNModelConfig{})
	input := []float32{1.0, 2.0, 2.0} // Norm = 3
	meta := &InferenceMeta{
		SMILES:           "C",
		ModelID:          "v1",
		BackendLatencyMs: 10,
	}

	result, err := processor.ProcessEmbedding(input, meta)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, float32(1.0/3.0), result.Vector[0])
	assert.Equal(t, float32(2.0/3.0), result.Vector[1])
	assert.Equal(t, float32(1.0), result.L2Norm)
}

func TestProcessEmbedding_ZeroVector(t *testing.T) {
	processor := NewGNNPostprocessor(&GNNModelConfig{})
	input := []float32{0, 0, 0}
	meta := &InferenceMeta{}

	_, err := processor.ProcessEmbedding(input, meta)
	assert.ErrorIs(t, err, ErrZeroVector)
}

func TestProcessEmbedding_DimensionMismatch(t *testing.T) {
	processor := NewGNNPostprocessor(&GNNModelConfig{})
	var input []float32
	meta := &InferenceMeta{}

	_, err := processor.ProcessEmbedding(input, meta)
	assert.ErrorIs(t, err, ErrDimensionMismatch)
}

func TestComputeCosineSimilarity(t *testing.T) {
	processor := NewGNNPostprocessor(&GNNModelConfig{})

	// Identical
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim, err := processor.ComputeCosineSimilarity(a, b)
	assert.NoError(t, err)
	assert.InDelta(t, 1.0, sim, 1e-6)

	// Orthogonal
	a = []float32{1, 0, 0}
	b = []float32{0, 1, 0}
	sim, err = processor.ComputeCosineSimilarity(a, b)
	assert.NoError(t, err)
	assert.InDelta(t, 0.0, sim, 1e-6)

	// Opposite
	a = []float32{1, 0, 0}
	b = []float32{-1, 0, 0}
	sim, err = processor.ComputeCosineSimilarity(a, b)
	assert.NoError(t, err)
	assert.InDelta(t, -1.0, sim, 1e-6)
}

func TestFuseScores(t *testing.T) {
	processor := NewGNNPostprocessor(&GNNModelConfig{})

	weights := map[string]float64{
		"A": 0.5,
		"B": 0.5,
	}

	// All present
	scores := map[string]float64{
		"A": 1.0,
		"B": 0.0,
	}
	fused, err := processor.FuseScores(scores, weights)
	assert.NoError(t, err)
	assert.InDelta(t, 0.5, fused, 1e-6)

	// Missing one
	scores = map[string]float64{
		"A": 1.0,
	}
	// Weight sum is 0.5. Re-normalized weight for A is 0.5/0.5 = 1.0
	// Fused score = 1.0 * 1.0 = 1.0
	fused, err = processor.FuseScores(scores, weights)
	assert.NoError(t, err)
	assert.InDelta(t, 1.0, fused, 1e-6)
}

func TestClassifySimilarity(t *testing.T) {
	processor := NewGNNPostprocessor(&GNNModelConfig{})

	assert.Equal(t, SimilarityHigh, processor.ClassifySimilarity(0.9))
	assert.Equal(t, SimilarityMedium, processor.ClassifySimilarity(0.75))
	assert.Equal(t, SimilarityLow, processor.ClassifySimilarity(0.6))
	assert.Equal(t, SimilarityNone, processor.ClassifySimilarity(0.3))
}

//Personal.AI order the ending
