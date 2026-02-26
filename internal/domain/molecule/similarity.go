package molecule

import (
	"context"
	"fmt"
	"math"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// SimilarityMetric defines the algorithm for comparing fingerprints.
type SimilarityMetric string

const (
	MetricTanimoto  SimilarityMetric = "tanimoto"
	MetricDice      SimilarityMetric = "dice"
	MetricCosine    SimilarityMetric = "cosine"
	MetricEuclidean SimilarityMetric = "euclidean"
	MetricManhattan SimilarityMetric = "manhattan"
	MetricSoergel   SimilarityMetric = "soergel"
)

func (m SimilarityMetric) IsValid() bool {
	switch m {
	case MetricTanimoto, MetricDice, MetricCosine, MetricEuclidean, MetricManhattan, MetricSoergel:
		return true
	default:
		return false
	}
}

func (m SimilarityMetric) String() string {
	return string(m)
}

func ParseSimilarityMetric(s string) (SimilarityMetric, error) {
	m := SimilarityMetric(s)
	if m.IsValid() {
		return m, nil
	}
	return "", errors.New(errors.ErrCodeInvalidInput, "invalid similarity metric: "+s)
}

// SimilarityThresholds
const (
	ThresholdIdentical          = 0.99
	ThresholdHighSimilarity     = 0.85
	ThresholdModerateSimilarity = 0.70
	ThresholdLowSimilarity      = 0.50
)

// ClassifySimilarity returns a descriptive classification of the similarity score.
func ClassifySimilarity(score float64) string {
	if score >= ThresholdIdentical {
		return "identical"
	}
	if score >= ThresholdHighSimilarity {
		return "high"
	}
	if score >= ThresholdModerateSimilarity {
		return "moderate"
	}
	if score >= ThresholdLowSimilarity {
		return "low"
	}
	return "dissimilar"
}

// SimilarityCalculator defines the interface for calculating similarity.
type SimilarityCalculator interface {
	Calculate(fp1, fp2 *Fingerprint) (float64, error)
	Metric() SimilarityMetric
	SupportsEncoding(encoding FingerprintEncoding) bool
}

// TanimotoCalculator implements Tanimoto similarity.
type TanimotoCalculator struct{}

func (c *TanimotoCalculator) Metric() SimilarityMetric { return MetricTanimoto }

func (c *TanimotoCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingBitVector || encoding == EncodingDenseVector
}

func (c *TanimotoCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil {
		return 0, errors.New(errors.ErrCodeInvalidInput, "fingerprints cannot be nil")
	}
	if fp1.Type != fp2.Type {
		return 0, errors.New(errors.ErrCodeInvalidInput, "fingerprint types mismatch")
	}

	if fp1.IsBitVector() && fp2.IsBitVector() {
		if fp1.NumBits != fp2.NumBits {
			return 0, errors.New(errors.ErrCodeInvalidInput, "fingerprint length mismatch")
		}

		// Use helpers from fingerprint.go
		andBits, err := BitAnd(fp1.Bits, fp2.Bits)
		if err != nil { return 0, err }
		orBits, err := BitOr(fp1.Bits, fp2.Bits)
		if err != nil { return 0, err }

		intersection := PopCount(andBits)
		union := PopCount(orBits)

		if union == 0 {
			return 0.0, nil // Both empty
		}
		return float64(intersection) / float64(union), nil
	}

	if fp1.IsDenseVector() && fp2.IsDenseVector() {
		if len(fp1.Vector) != len(fp2.Vector) {
			return 0, errors.New(errors.ErrCodeInvalidInput, "vector dimension mismatch")
		}
		// Generalized Tanimoto: Sum(min(Ai, Bi)) / Sum(max(Ai, Bi))
		sumMin := 0.0
		sumMax := 0.0
		for i := range fp1.Vector {
			v1 := float64(fp1.Vector[i])
			v2 := float64(fp2.Vector[i])
			sumMin += math.Min(v1, v2)
			sumMax += math.Max(v1, v2)
		}
		if sumMax == 0 {
			return 0.0, nil
		}
		return sumMin / sumMax, nil
	}

	return 0, errors.New(errors.ErrCodeInvalidInput, "unsupported or mismatched encodings")
}

// DiceCalculator implements Dice similarity.
type DiceCalculator struct{}

func (c *DiceCalculator) Metric() SimilarityMetric { return MetricDice }
func (c *DiceCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingBitVector
}

func (c *DiceCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil { return 0, errors.New(errors.ErrCodeInvalidInput, "fingerprints cannot be nil") }
	if !fp1.IsBitVector() || !fp2.IsBitVector() {
		return 0, errors.New(errors.ErrCodeInvalidInput, "Dice requires bit vectors")
	}
	if fp1.Type != fp2.Type { return 0, errors.New(errors.ErrCodeInvalidInput, "type mismatch") }
	if fp1.NumBits != fp2.NumBits { return 0, errors.New(errors.ErrCodeInvalidInput, "length mismatch") }

	andBits, _ := BitAnd(fp1.Bits, fp2.Bits)
	intersection := PopCount(andBits)
	pop1 := fp1.BitCount()
	pop2 := fp2.BitCount()

	denom := pop1 + pop2
	if denom == 0 {
		return 0.0, nil
	}
	return (2.0 * float64(intersection)) / float64(denom), nil
}

// CosineCalculator implements Cosine similarity.
type CosineCalculator struct{}

func (c *CosineCalculator) Metric() SimilarityMetric { return MetricCosine }
func (c *CosineCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingDenseVector || encoding == EncodingBitVector
}

func (c *CosineCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil { return 0, errors.New(errors.ErrCodeInvalidInput, "fingerprints cannot be nil") }

	// Convert to dense vectors
	v1 := fp1.ToFloat32Slice()
	v2 := fp2.ToFloat32Slice()

	if len(v1) != len(v2) {
		return 0, errors.New(errors.ErrCodeInvalidInput, "dimension mismatch")
	}

	dotProduct := 0.0
	norm1 := 0.0
	norm2 := 0.0

	for i := range v1 {
		val1 := float64(v1[i])
		val2 := float64(v2[i])
		dotProduct += val1 * val2
		norm1 += val1 * val1
		norm2 += val2 * val2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0, nil
	}

	cosine := dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
	// Normalize to [0, 1] range: (cosine + 1) / 2
	// But spec says "Standard Cosine similarity... values [-1, 1] (normalized to [0, 1])"
	// For bit vectors (all non-negative), cosine is already [0, 1].
	// For dense vectors (GNN embeddings), they can have negative components.
	// So normalization is appropriate if we want similarity score consistent with Tanimoto/Dice [0, 1].
	// However, if vectors are non-negative (e.g. counts), cosine is [0, 1].
	// If vectors are embeddings, they are usually centered.
	// Let's implement normalization:
	normalized := (cosine + 1.0) / 2.0

	// Clamp just in case of float errors
	if normalized > 1.0 { normalized = 1.0 }
	if normalized < 0.0 { normalized = 0.0 }

	return normalized, nil
}

// EuclideanCalculator implements Euclidean similarity.
type EuclideanCalculator struct{}

func (c *EuclideanCalculator) Metric() SimilarityMetric { return MetricEuclidean }
func (c *EuclideanCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingDenseVector
}

func (c *EuclideanCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil { return 0, errors.New(errors.ErrCodeInvalidInput, "fingerprints cannot be nil") }
	if !fp1.IsDenseVector() || !fp2.IsDenseVector() {
		return 0, errors.New(errors.ErrCodeInvalidInput, "Euclidean requires dense vectors")
	}
	if len(fp1.Vector) != len(fp2.Vector) {
		return 0, errors.New(errors.ErrCodeInvalidInput, "dimension mismatch")
	}

	sumSqDiff := 0.0
	for i := range fp1.Vector {
		diff := float64(fp1.Vector[i] - fp2.Vector[i])
		sumSqDiff += diff * diff
	}

	dist := math.Sqrt(sumSqDiff)
	return 1.0 / (1.0 + dist), nil
}

// NewSimilarityCalculator factory
func NewSimilarityCalculator(metric SimilarityMetric) (SimilarityCalculator, error) {
	switch metric {
	case MetricTanimoto:
		return &TanimotoCalculator{}, nil
	case MetricDice:
		return &DiceCalculator{}, nil
	case MetricCosine:
		return &CosineCalculator{}, nil
	case MetricEuclidean:
		return &EuclideanCalculator{}, nil
	case MetricManhattan, MetricSoergel:
		return nil, errors.New(errors.ErrCodeNotImplemented, "metric not implemented: "+string(metric))
	default:
		return nil, errors.New(errors.ErrCodeInvalidInput, "unknown metric: "+string(metric))
	}
}

// SimilarityResult
type SimilarityResult struct {
	MoleculeID      string          `json:"molecule_id"`
	SMILES          string          `json:"smiles"`
	Score           float64         `json:"score"`
	Metric          SimilarityMetric `json:"metric"`
	FingerprintType FingerprintType `json:"fingerprint_type"`
	Rank            int             `json:"rank"`
}

func (r *SimilarityResult) String() string {
	return fmt.Sprintf("Result{id=%s, score=%.4f, metric=%s}", r.MoleculeID, r.Score, r.Metric)
}

// SimilarityEngine interface
type SimilarityEngine interface {
	SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error)
	ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error)
	BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error)
	RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error)
}

// DefaultSimilarityEngine implementation
type DefaultSimilarityEngine struct {
	calculators map[SimilarityMetric]SimilarityCalculator
}

func NewDefaultSimilarityEngine() *DefaultSimilarityEngine {
	calcs := make(map[SimilarityMetric]SimilarityCalculator)
	calcs[MetricTanimoto], _ = NewSimilarityCalculator(MetricTanimoto)
	calcs[MetricDice], _ = NewSimilarityCalculator(MetricDice)
	calcs[MetricCosine], _ = NewSimilarityCalculator(MetricCosine)
	calcs[MetricEuclidean], _ = NewSimilarityCalculator(MetricEuclidean)
	return &DefaultSimilarityEngine{calculators: calcs}
}

func (e *DefaultSimilarityEngine) ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error) {
	calc, ok := e.calculators[metric]
	if !ok {
		return 0, errors.New(errors.ErrCodeInvalidInput, "metric not supported: "+string(metric))
	}
	return calc.Calculate(fp1, fp2)
}

func (e *DefaultSimilarityEngine) BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error) {
	calc, ok := e.calculators[metric]
	if !ok {
		return nil, errors.New(errors.ErrCodeInvalidInput, "metric not supported: "+string(metric))
	}

	results := make([]float64, len(candidates))
	for i, cand := range candidates {
		score, err := calc.Calculate(target, cand)
		if err != nil {
			return nil, err
		}
		results[i] = score
	}
	return results, nil
}

func (e *DefaultSimilarityEngine) SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
	// Not implemented in default engine (requires full DB scan or vector DB)
	return nil, errors.New(errors.ErrCodeNotImplemented, "SearchSimilar requires vector database implementation")
}

func (e *DefaultSimilarityEngine) RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error) {
	// Not implemented in default engine
	return nil, errors.New(errors.ErrCodeNotImplemented, "RankBySimilarity requires repository access or vector DB")
}

// SimilaritySearchOptions
type SimilaritySearchOptions struct {
	Metric          SimilarityMetric
	FingerprintType FingerprintType
	Threshold       float64
	Limit           int
	UseVectorDB     bool
	FusionStrategy  FingerprintFusionStrategy
	FusionTypes     []FingerprintType
	FusionWeights   map[FingerprintType]float64
}

func DefaultSimilaritySearchOptions() *SimilaritySearchOptions {
	return &SimilaritySearchOptions{
		Metric:          MetricTanimoto,
		FingerprintType: FingerprintMorgan,
		Threshold:       0.7,
		Limit:           100,
		UseVectorDB:     true,
	}
}

func (o *SimilaritySearchOptions) Validate() error {
	if !o.Metric.IsValid() {
		return errors.New(errors.ErrCodeInvalidInput, "invalid metric")
	}
	if !o.FingerprintType.IsValid() {
		return errors.New(errors.ErrCodeInvalidInput, "invalid fingerprint type")
	}
	if o.Threshold < 0.0 || o.Threshold > 1.0 { // Allow 0 and 1
		// Actually if threshold > 1.0 it's impossible unless metric is distance (unnormalized).
		// Assuming normalized similarity [0, 1].
		return errors.New(errors.ErrCodeInvalidInput, "threshold must be between 0.0 and 1.0")
	}
	if o.Limit <= 0 {
		return errors.New(errors.ErrCodeInvalidInput, "limit must be positive")
	}
	return nil
}

//Personal.AI order the ending
