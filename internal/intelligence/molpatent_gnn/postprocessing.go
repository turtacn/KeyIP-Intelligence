package molpatent_gnn

import (
	"errors"
	"math"

)

// SimilarityLevel defines the level of similarity between two molecules.
type SimilarityLevel string

const (
	SimilarityHigh   SimilarityLevel = "HIGH"
	SimilarityMedium SimilarityLevel = "MEDIUM"
	SimilarityLow    SimilarityLevel = "LOW"
	SimilarityNone   SimilarityLevel = "NONE"
)

// GNNPostprocessor defines the interface for GNN model post-processing.
type GNNPostprocessor interface {
	ProcessEmbedding(raw []float32, meta *InferenceMeta) (*EmbeddingResult, error)
	ProcessBatchEmbedding(raw [][]float32, meta []*InferenceMeta) ([]*EmbeddingResult, error)
	ComputeCosineSimilarity(a, b []float32) (float64, error)
	ComputeTanimotoSimilarity(a, b []byte) (float64, error)
	FuseScores(scores map[string]float64, weights map[string]float64) (float64, error)
	ClassifySimilarity(score float64) SimilarityLevel
}

// EmbeddingResult represents the processed embedding result.
type EmbeddingResult struct {
	Vector       []float32
	L2Norm       float32
	InferenceMs  int64
	ModelVersion string
}

// InferenceMeta contains metadata about the inference request.
type InferenceMeta struct {
	SMILES       string
	ModelID      string
	Timestamp    int64 // Unix milli
	BackendLatencyMs int64
}

// gnnPostprocessorImpl implements GNNPostprocessor.
type gnnPostprocessorImpl struct {
	config *GNNModelConfig
}

// NewGNNPostprocessor creates a new GNNPostprocessor.
func NewGNNPostprocessor(cfg *GNNModelConfig) GNNPostprocessor {
	return &gnnPostprocessorImpl{
		config: cfg,
	}
}

var (
	ErrZeroVector         = errors.New("zero vector cannot be normalized")
	ErrDimensionMismatch  = errors.New("vector dimension mismatch")
	ErrNoScoresProvided   = errors.New("no scores provided for fusion")
	ErrInvalidWeights     = errors.New("invalid weights sum")
	ErrInvalidScore       = errors.New("invalid score value")
)

func (p *gnnPostprocessorImpl) ProcessEmbedding(raw []float32, meta *InferenceMeta) (*EmbeddingResult, error) {
	if len(raw) == 0 {
		return nil, ErrDimensionMismatch
	}

	// Calculate L2 Norm
	var sumSq float64
	for _, v := range raw {
		sumSq += float64(v) * float64(v)
	}
	l2Norm := float32(math.Sqrt(sumSq))

	if l2Norm == 0 {
		return nil, ErrZeroVector
	}

	// Normalize
	normalized := make([]float32, len(raw))
	for i, v := range raw {
		normalized[i] = v / l2Norm
	}

	return &EmbeddingResult{
		Vector:       normalized,
		L2Norm:       1.0, // Normalized vector has L2 norm of 1
		InferenceMs:  meta.BackendLatencyMs,
		ModelVersion: meta.ModelID,
	}, nil
}

func (p *gnnPostprocessorImpl) ProcessBatchEmbedding(raw [][]float32, meta []*InferenceMeta) ([]*EmbeddingResult, error) {
	if len(raw) != len(meta) {
		return nil, errors.New("input length mismatch")
	}

	results := make([]*EmbeddingResult, len(raw))
	for i, vec := range raw {
		res, err := p.ProcessEmbedding(vec, meta[i])
		if err != nil {
			return nil, err
		}
		results[i] = res
	}
	return results, nil
}

func (p *gnnPostprocessorImpl) ComputeCosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrDimensionMismatch
	}
	if len(a) == 0 {
		return 0, ErrDimensionMismatch
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0, errors.New("cannot compute similarity for zero vector")
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)), nil
}

func (p *gnnPostprocessorImpl) ComputeTanimotoSimilarity(a, b []byte) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrDimensionMismatch
	}
	if len(a) == 0 {
		return 0, errors.New("empty fingerprint")
	}

	// Assuming a and b are byte arrays representing bit vectors
	// We need to count bits set in A, B and (A & B)

	popCount := func(b byte) int {
		count := 0
		for b > 0 {
			if b&1 == 1 {
				count++
			}
			b >>= 1
		}
		return count
	}

	countA := 0
	countB := 0
	countIntersection := 0

	for i := 0; i < len(a); i++ {
		byteA := a[i]
		byteB := b[i]

		countA += popCount(byteA)
		countB += popCount(byteB)
		countIntersection += popCount(byteA & byteB)
	}

	union := countA + countB - countIntersection
	if union == 0 {
		return 0, nil // Both empty, similarity is 0 (or 1 depending on definition, but usually 0 for empty vs empty in chemical context implies no common features)
	}

	return float64(countIntersection) / float64(union), nil
}

func (p *gnnPostprocessorImpl) FuseScores(scores map[string]float64, weights map[string]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, ErrNoScoresProvided
	}

	// Validate weights sum
	var weightSum float64
	for _, w := range weights {
		weightSum += w
	}

	if math.Abs(weightSum-1.0) > 1e-6 {
		return 0, ErrInvalidWeights
	}

	var fusedScore float64
	var activeWeightSum float64

	for k, score := range scores {
		if score < 0 {
			return 0, ErrInvalidScore
		}
		if w, ok := weights[k]; ok {
			fusedScore += score * w
			activeWeightSum += w
		}
	}

	if activeWeightSum == 0 {
		// Should not happen if weights match scores keys and sum to 1
		// But if scores keys are a subset of weights keys, we need to re-normalize
		// Wait, the logic description says: "when a fingerprint type is missing, re-normalize weights"

		// The loop above iterates over available scores.
		// If a score is available, we add it.
		// activeWeightSum tracks the weights of available scores.

		if len(scores) < len(weights) {
			// Some weights were not used because scores were missing
			// We need to normalize fusedScore by activeWeightSum
			if activeWeightSum > 0 {
				return fusedScore / activeWeightSum, nil
			}
		}
	}

	// Case where all weights are present
	if math.Abs(activeWeightSum-1.0) > 1e-6 && activeWeightSum > 0 {
		return fusedScore / activeWeightSum, nil
	}

	return fusedScore, nil
}

func (p *gnnPostprocessorImpl) ClassifySimilarity(score float64) SimilarityLevel {
	if score >= 0.85 {
		return SimilarityHigh
	}
	if score >= 0.70 {
		return SimilarityMedium
	}
	if score >= 0.55 {
		return SimilarityLow
	}
	return SimilarityNone
}

//Personal.AI order the ending
