package molecule

import (
	"context"
	"fmt"
	"math"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// SimilarityMetric defines the algorithm used for molecular similarity measurement.
type SimilarityMetric string

const (
	MetricTanimoto  SimilarityMetric = "tanimoto"
	MetricDice      SimilarityMetric = "dice"
	MetricCosine    SimilarityMetric = "cosine"
	MetricEuclidean SimilarityMetric = "euclidean"
	MetricManhattan SimilarityMetric = "manhattan"
	MetricSoergel   SimilarityMetric = "soergel"
)

// IsValid checks if the similarity metric is valid.
func (m SimilarityMetric) IsValid() bool {
	switch m {
	case MetricTanimoto, MetricDice, MetricCosine, MetricEuclidean, MetricManhattan, MetricSoergel:
		return true
	default:
		return false
	}
}

// String returns the string representation of the similarity metric.
func (m SimilarityMetric) String() string {
	return string(m)
}

// ParseSimilarityMetric parses a string into a SimilarityMetric.
func ParseSimilarityMetric(s string) (SimilarityMetric, error) {
	m := SimilarityMetric(s)
	if m.IsValid() {
		return m, nil
	}
	return "", errors.New(errors.ErrCodeValidation, "unsupported similarity metric: "+s)
}

// SimilarityCalculator defines the interface for calculating similarity between fingerprints.
type SimilarityCalculator interface {
	Calculate(fp1, fp2 *Fingerprint) (float64, error)
	Metric() SimilarityMetric
	SupportsEncoding(encoding FingerprintEncoding) bool
}

// TanimotoCalculator implements Tanimoto similarity (Jaccard index).
type TanimotoCalculator struct{}

// Calculate computes Tanimoto similarity.
func (c *TanimotoCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1.Type != fp2.Type || fp1.NumBits != fp2.NumBits {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprints must have same type and dimension")
	}

	if fp1.IsBitVector() && fp2.IsBitVector() {
		intersection := 0
		union := 0
		for i := range fp1.Bits {
			b1 := fp1.Bits[i]
			b2 := fp2.Bits[i]
			intersection += PopCountByte(b1 & b2)
			union += PopCountByte(b1 | b2)
		}
		if union == 0 {
			return 0.0, nil
		}
		return float64(intersection) / float64(union), nil
	}

	if fp1.IsDenseVector() && fp2.IsDenseVector() {
		var sumMin, sumMax float32
		for i := range fp1.Vector {
			v1 := fp1.Vector[i]
			v2 := fp2.Vector[i]
			if v1 < v2 {
				sumMin += v1
				sumMax += v2
			} else {
				sumMin += v2
				sumMax += v1
			}
		}
		if sumMax == 0 {
			return 0.0, nil
		}
		return float64(sumMin / sumMax), nil
	}

	return 0, errors.New(errors.ErrCodeValidation, "unsupported encoding for Tanimoto")
}

// Metric returns MetricTanimoto.
func (c *TanimotoCalculator) Metric() SimilarityMetric { return MetricTanimoto }

// SupportsEncoding returns true for BitVector and DenseVector.
func (c *TanimotoCalculator) SupportsEncoding(e FingerprintEncoding) bool {
	return e == EncodingBitVector || e == EncodingDenseVector
}

// DiceCalculator implements Dice similarity.
type DiceCalculator struct{}

// Calculate computes Dice similarity.
func (c *DiceCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1.Type != fp2.Type || fp1.NumBits != fp2.NumBits {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprints must have same type and dimension")
	}

	if fp1.IsBitVector() && fp2.IsBitVector() {
		intersection := 0
		for i := range fp1.Bits {
			intersection += PopCountByte(fp1.Bits[i] & fp2.Bits[i])
		}
		denominator := fp1.BitCount() + fp2.BitCount()
		if denominator == 0 {
			return 0.0, nil
		}
		return 2.0 * float64(intersection) / float64(denominator), nil
	}

	return 0, errors.New(errors.ErrCodeValidation, "Dice similarity only supports bit vectors")
}

// Metric returns MetricDice.
func (c *DiceCalculator) Metric() SimilarityMetric { return MetricDice }

// SupportsEncoding returns true for BitVector.
func (c *DiceCalculator) SupportsEncoding(e FingerprintEncoding) bool {
	return e == EncodingBitVector
}

// CosineCalculator implements Cosine similarity.
type CosineCalculator struct{}

// Calculate computes Cosine similarity.
func (c *CosineCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1.NumBits != fp2.NumBits {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprints must have same dimension")
	}

	v1 := fp1.ToFloat32Slice()
	v2 := fp2.ToFloat32Slice()

	var dotProduct, norm1, norm2 float64
	for i := range v1 {
		f1 := float64(v1[i])
		f2 := float64(v2[i])
		dotProduct += f1 * f2
		norm1 += f1 * f1
		norm2 += f2 * f2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0, nil
	}

	cosine := dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
	// Normalize to [0, 1]
	return (cosine + 1) / 2, nil
}

// Metric returns MetricCosine.
func (c *CosineCalculator) Metric() SimilarityMetric { return MetricCosine }

// SupportsEncoding returns true for BitVector and DenseVector.
func (c *CosineCalculator) SupportsEncoding(e FingerprintEncoding) bool {
	return e == EncodingBitVector || e == EncodingDenseVector
}

// EuclideanCalculator implements Euclidean distance based similarity.
type EuclideanCalculator struct{}

// Calculate computes Euclidean similarity.
func (c *EuclideanCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1.NumBits != fp2.NumBits {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprints must have same dimension")
	}
	if !fp1.IsDenseVector() || !fp2.IsDenseVector() {
		return 0, errors.New(errors.ErrCodeValidation, "Euclidean similarity only supports dense vectors")
	}

	var distSq float64
	for i := range fp1.Vector {
		diff := float64(fp1.Vector[i] - fp2.Vector[i])
		distSq += diff * diff
	}
	distance := math.Sqrt(distSq)
	return 1.0 / (1.0 + distance), nil
}

// Metric returns MetricEuclidean.
func (c *EuclideanCalculator) Metric() SimilarityMetric { return MetricEuclidean }

// SupportsEncoding returns true for DenseVector.
func (c *EuclideanCalculator) SupportsEncoding(e FingerprintEncoding) bool {
	return e == EncodingDenseVector
}

// NewSimilarityCalculator factory function.
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
	default:
		return nil, errors.New(errors.ErrCodeValidation, "unsupported similarity metric: "+string(metric))
	}
}

// SimilarityEngine defines high-level operations for molecular similarity search.
type SimilarityEngine interface {
	SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error)
	ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error)
	BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error)
	RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error)
}

// SimilarityResult represents a single match in a similarity search.
type SimilarityResult struct {
	MoleculeID      string          `json:"molecule_id"`
	SMILES          string          `json:"smiles"`
	Score           float64         `json:"score"`
	Metric          SimilarityMetric `json:"metric"`
	FingerprintType FingerprintType `json:"fingerprint_type"`
	Rank            int             `json:"rank"`
}

func (r *SimilarityResult) String() string {
	return fmt.Sprintf("SimilarityResult{id=%s, score=%.4f, metric=%s}", r.MoleculeID, r.Score, r.Metric)
}

// SimilaritySearchOptions defines configuration for similarity search.
type SimilaritySearchOptions struct {
	Metric                  SimilarityMetric
	FingerprintType         FingerprintType
	Threshold               float64
	Limit                   int
	UseVectorDB             bool
	FusionStrategy          FingerprintFusionStrategy
	FusionFingerprintTypes  []FingerprintType
	FusionWeights           map[FingerprintType]float64
}

// DefaultSimilaritySearchOptions returns default search options.
func DefaultSimilaritySearchOptions() *SimilaritySearchOptions {
	return &SimilaritySearchOptions{
		Metric:          MetricTanimoto,
		FingerprintType: FingerprintMorgan,
		Threshold:       0.7,
		Limit:           100,
		UseVectorDB:     true,
	}
}

// Validate checks if the search options are valid.
func (o *SimilaritySearchOptions) Validate() error {
	if !o.Metric.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid similarity metric")
	}
	if !o.FingerprintType.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid fingerprint type")
	}
	if o.Threshold < 0 || o.Threshold > 1 {
		return errors.New(errors.ErrCodeValidation, "threshold must be between 0 and 1")
	}
	if o.Limit <= 0 {
		return errors.New(errors.ErrCodeValidation, "limit must be positive")
	}
	return nil
}

// DefaultSimilarityEngine implements SimilarityEngine.
type DefaultSimilarityEngine struct {
	calculators map[SimilarityMetric]SimilarityCalculator
}

// NewDefaultSimilarityEngine constructs a new DefaultSimilarityEngine.
func NewDefaultSimilarityEngine() *DefaultSimilarityEngine {
	calcs := make(map[SimilarityMetric]SimilarityCalculator)
	calcs[MetricTanimoto] = &TanimotoCalculator{}
	calcs[MetricDice] = &DiceCalculator{}
	calcs[MetricCosine] = &CosineCalculator{}
	calcs[MetricEuclidean] = &EuclideanCalculator{}
	return &DefaultSimilarityEngine{calculators: calcs}
}

// ComputeSimilarity computes similarity between two fingerprints.
func (e *DefaultSimilarityEngine) ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error) {
	calc, ok := e.calculators[metric]
	if !ok {
		return 0, errors.New(errors.ErrCodeValidation, "unsupported similarity metric")
	}
	return calc.Calculate(fp1, fp2)
}

// BatchComputeSimilarity computes similarity between target and multiple candidates.
func (e *DefaultSimilarityEngine) BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error) {
	res := make([]float64, len(candidates))
	for i, c := range candidates {
		score, err := e.ComputeSimilarity(target, c, metric)
		if err != nil {
			return nil, err
		}
		res[i] = score
	}
	return res, nil
}

// SearchSimilar is not implemented in the default engine (requires vector DB).
func (e *DefaultSimilarityEngine) SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
	return nil, errors.New(errors.ErrCodeNotImplemented, "SearchSimilar requires vector database implementation")
}

// RankBySimilarity is not implemented in the default engine.
func (e *DefaultSimilarityEngine) RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error) {
	return nil, errors.New(errors.ErrCodeNotImplemented, "RankBySimilarity requires vector database implementation")
}

// Similarity Threshold Constants
const (
	ThresholdIdentical           = 0.99
	ThresholdHighSimilarity      = 0.85
	ThresholdModerateSimilarity  = 0.70
	ThresholdLowSimilarity       = 0.50
)

// ClassifySimilarity returns a classification label for a given similarity score.
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

//Personal.AI order the ending
