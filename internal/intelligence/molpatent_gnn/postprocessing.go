package molpatent_gnn

import (
	"fmt"
	"math"
	"math/bits"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Domain Types (Interfaces & Structs)
// ---------------------------------------------------------------------------

// SimilarityLevel classifies the similarity between two embeddings.
type SimilarityLevel string

const (
	SimilarityHigh   SimilarityLevel = "HIGH"
	SimilarityMedium SimilarityLevel = "MEDIUM"
	SimilarityLow    SimilarityLevel = "LOW"
	SimilarityNone   SimilarityLevel = "NONE"
)

// ---------------------------------------------------------------------------
// Similarity classification thresholds (configurable via PostprocessorConfig)
// ---------------------------------------------------------------------------

// PostprocessorConfig holds tunable parameters for the postprocessor.
type PostprocessorConfig struct {
	// ExpectedDim is the expected embedding dimensionality. 0 means skip check.
	ExpectedDim int `json:"expected_dim" yaml:"expected_dim"`

	// Similarity thresholds — score >= threshold maps to the level.
	ThresholdHigh   float64 `json:"threshold_high" yaml:"threshold_high"`
	ThresholdMedium float64 `json:"threshold_medium" yaml:"threshold_medium"`
	ThresholdLow    float64 `json:"threshold_low" yaml:"threshold_low"`

	// FusionWeightTolerance is the maximum acceptable deviation of the weight
	// sum from 1.0 before we force renormalization.
	FusionWeightTolerance float64 `json:"fusion_weight_tolerance" yaml:"fusion_weight_tolerance"`
}

// DefaultPostprocessorConfig returns production-grade defaults.
func DefaultPostprocessorConfig() *PostprocessorConfig {
	return &PostprocessorConfig{
		ExpectedDim:           256,
		ThresholdHigh:         0.85,
		ThresholdMedium:       0.70,
		ThresholdLow:          0.55,
		FusionWeightTolerance: 1e-6,
	}
}

// ---------------------------------------------------------------------------
// Concrete implementation
// ---------------------------------------------------------------------------

// gnnPostprocessorImpl is the production implementation of GNNPostprocessor.
type gnnPostprocessorImpl struct {
	config *PostprocessorConfig
}

// NewGNNPostprocessor creates a new postprocessor with the given config.
// If config is nil, DefaultPostprocessorConfig is used.
func NewGNNPostprocessor(config *PostprocessorConfig) GNNPostprocessor {
	if config == nil {
		config = DefaultPostprocessorConfig()
	}
	return &gnnPostprocessorImpl{config: config}
}

// ---------------------------------------------------------------------------
// ProcessEmbedding — single vector post-processing
// ---------------------------------------------------------------------------
//
// Pipeline:
//   1. Validate that the raw vector is non-empty.
//   2. If ExpectedDim > 0, verify dimensionality.
//   3. Compute L2 norm.
//   4. Reject zero vectors (norm < epsilon).
//   5. Divide every element by the norm → unit vector on the hypersphere.
//   6. Return EmbeddingResult with the normalised vector and the original norm.
//
// Mathematical basis:
//   v_norm = v / ||v||₂
//   After normalisation, cosine similarity degenerates to the dot product,
//   which is significantly cheaper to compute in vector databases (Milvus IP).

func (p *gnnPostprocessorImpl) ProcessEmbedding(raw []float32, meta *InferenceMeta) (*EmbeddingResult, error) {
	if len(raw) == 0 {
		return nil, errors.NewInvalidInputError("raw embedding vector is empty")
	}

	// Dimension check
	if p.config.ExpectedDim > 0 && len(raw) != p.config.ExpectedDim {
		return nil, errors.NewInvalidInputError(
			fmt.Sprintf("embedding dimension mismatch: expected %d, got %d",
				p.config.ExpectedDim, len(raw)))
	}

	// L2 norm
	norm := l2Norm(raw)

	const epsilon = 1e-12
	if norm < epsilon {
		return nil, errors.NewInvalidInputError("cannot normalise zero vector (L2 norm < epsilon)")
	}

	// Normalise in-place copy
	normalised := make([]float32, len(raw))
	invNorm := float32(1.0 / norm)
	for i, v := range raw {
		normalised[i] = v * invNorm
	}

	return &EmbeddingResult{
		NormalizedVector: normalised,
		L2Norm:           norm,
	}, nil
}

// ---------------------------------------------------------------------------
// ProcessBatchEmbedding — batch wrapper
// ---------------------------------------------------------------------------

func (p *gnnPostprocessorImpl) ProcessBatchEmbedding(raw [][]float32, meta []*InferenceMeta) ([]*EmbeddingResult, error) {
	if len(raw) == 0 {
		return []*EmbeddingResult{}, nil
	}

	results := make([]*EmbeddingResult, len(raw))
	for i, vec := range raw {
		var m *InferenceMeta
		if meta != nil && i < len(meta) {
			m = meta[i]
		}
		res, err := p.ProcessEmbedding(vec, m)
		if err != nil {
			return nil, fmt.Errorf("batch item %d: %w", i, err)
		}
		results[i] = res
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// ComputeCosineSimilarity
// ---------------------------------------------------------------------------
//
// cos(θ) = (a · b) / (||a||₂ × ||b||₂)
//
// Edge cases:
//   - Dimension mismatch → error
//   - Either vector is zero → error (division by zero)
//   - Identical vectors → 1.0
//   - Orthogonal vectors → 0.0
//   - Opposite vectors → -1.0
//
// If both vectors are already L2-normalised (which they should be after
// ProcessEmbedding), the denominator is 1.0 and this reduces to a dot product.

func (p *gnnPostprocessorImpl) ComputeCosineSimilarity(a, b []float32) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, errors.NewInvalidInputError("vectors must be non-empty")
	}
	if len(a) != len(b) {
		return 0, errors.NewInvalidInputError(
			fmt.Sprintf("dimension mismatch: %d vs %d", len(a), len(b)))
	}

	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		ai := float64(a[i])
		bi := float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	const epsilon = 1e-12
	if normA < epsilon || normB < epsilon {
		return 0, errors.NewInvalidInputError("cannot compute cosine similarity with zero vector")
	}

	sim := dot / (normA * normB)

	// Clamp to [-1, 1] to guard against floating-point drift.
	if sim > 1.0 {
		sim = 1.0
	}
	if sim < -1.0 {
		sim = -1.0
	}

	return sim, nil
}

// ---------------------------------------------------------------------------
// ComputeTanimotoSimilarity
// ---------------------------------------------------------------------------
//
// The Tanimoto coefficient (a.k.a. Jaccard index for binary sets) is the
// standard metric for comparing binary molecular fingerprints:
//
//   T(A, B) = c / (a + b - c)
//
// where:
//   a = popcount(A)   — number of ON bits in fingerprint A
//   b = popcount(B)   — number of ON bits in fingerprint B
//   c = popcount(A & B) — number of bits ON in both
//
// Edge cases:
//   - Both fingerprints all-zero → T = 0.0 (no structural features)
//   - Length mismatch → error
//   - Identical fingerprints → T = 1.0

func (p *gnnPostprocessorImpl) ComputeTanimotoSimilarity(a, b []byte) (float64, error) {
	if len(a) == 0 || len(b) == 0 {
		return 0, errors.NewInvalidInputError("fingerprints must be non-empty")
	}
	if len(a) != len(b) {
		return 0, errors.NewInvalidInputError(
			fmt.Sprintf("fingerprint length mismatch: %d vs %d", len(a), len(b)))
	}

	var popA, popB, popAB uint64
	for i := 0; i < len(a); i++ {
		popA += uint64(bits.OnesCount8(a[i]))
		popB += uint64(bits.OnesCount8(b[i]))
		popAB += uint64(bits.OnesCount8(a[i] & b[i]))
	}

	denominator := popA + popB - popAB
	if denominator == 0 {
		// Both fingerprints are all-zero — no structural features to compare.
		return 0.0, nil
	}

	return float64(popAB) / float64(denominator), nil
}

// ---------------------------------------------------------------------------
// FuseScores — weighted multi-fingerprint fusion
// ---------------------------------------------------------------------------
//
// Given a map of fingerprint-type → similarity score and a map of
// fingerprint-type → weight, compute:
//
//   FusedScore = Σ(w_i * s_i) / Σ(w_i)   (for all types present in scores)
//
// When a fingerprint type exists in the weight map but is absent from the
// scores map, its weight is excluded and the remaining weights are
// renormalised so that they still sum to 1.0.
//
// Validation:
//   - At least one score must be present.
//   - Every score key must have a corresponding weight (or we skip it with
//     a warning-level tolerance).
//   - After filtering, the effective weight sum must be > 0.

func (p *gnnPostprocessorImpl) FuseScores(scores map[string]float64, weights map[string]float64) (float64, error) {
	if len(scores) == 0 {
		return 0, errors.NewInvalidInputError("scores map is empty")
	}
	if len(weights) == 0 {
		return 0, errors.NewInvalidInputError("weights map is empty")
	}

	// Collect effective (present) weights and their scores.
	var weightedSum float64
	var effectiveWeightSum float64

	for fpType, score := range scores {
		w, ok := weights[fpType]
		if !ok {
			// This fingerprint type has no configured weight — skip it.
			continue
		}
		if w < 0 {
			return 0, errors.NewInvalidInputError(
				fmt.Sprintf("negative weight for fingerprint type %q: %f", fpType, w))
		}
		weightedSum += w * score
		effectiveWeightSum += w
	}

	if effectiveWeightSum < 1e-15 {
		return 0, errors.NewInvalidInputError(
			"no matching weights for the provided scores — effective weight sum is zero")
	}

	// Renormalise: divide by the effective weight sum so the result is in [0, 1]
	// (assuming individual scores are in [0, 1]).
	fused := weightedSum / effectiveWeightSum

	// Clamp to [0, 1] for safety.
	if fused > 1.0 {
		fused = 1.0
	}
	if fused < 0.0 {
		fused = 0.0
	}

	return fused, nil
}

// ---------------------------------------------------------------------------
// ClassifySimilarity — threshold-based level assignment
// ---------------------------------------------------------------------------

func (p *gnnPostprocessorImpl) ClassifySimilarity(score float64) SimilarityLevel {
	switch {
	case score >= p.config.ThresholdHigh:
		return SimilarityHigh
	case score >= p.config.ThresholdMedium:
		return SimilarityMedium
	case score >= p.config.ThresholdLow:
		return SimilarityLow
	default:
		return SimilarityNone
	}
}

// ---------------------------------------------------------------------------
// Pure math helpers
// ---------------------------------------------------------------------------

// l2Norm computes the Euclidean norm of a float32 vector.
// Computation is done in float64 to minimise accumulation error.
func l2Norm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		f := float64(x)
		sum += f * f
	}
	return math.Sqrt(sum)
}

// dotProduct computes the dot product of two float32 vectors of equal length.
// No bounds check — caller must guarantee len(a) == len(b).
func dotProduct(a, b []float32) float64 {
	var sum float64
	for i := 0; i < len(a); i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

