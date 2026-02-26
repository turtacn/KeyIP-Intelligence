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
	return "", errors.New(errors.ErrCodeValidation, "invalid similarity metric: "+s)
}

// SimilarityCalculator defines the strategy for calculating similarity.
type SimilarityCalculator interface {
	Calculate(fp1, fp2 *Fingerprint) (float64, error)
	Metric() SimilarityMetric
	SupportsEncoding(encoding FingerprintEncoding) bool
}

// TanimotoCalculator implements Tanimoto similarity.
type TanimotoCalculator struct{}

func (c *TanimotoCalculator) Metric() SimilarityMetric {
	return MetricTanimoto
}

func (c *TanimotoCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingBitVector || encoding == EncodingDenseVector
}

func (c *TanimotoCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprints cannot be nil")
	}
	if fp1.Type != fp2.Type {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprint types mismatch")
	}
	if fp1.NumBits != fp2.NumBits && fp1.Dimension() != fp2.Dimension() {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprint dimensions mismatch")
	}

	if fp1.IsBitVector() && fp2.IsBitVector() {
		intersection, err := BitAnd(fp1.Bits, fp2.Bits)
		if err != nil {
			return 0, err
		}
		union, err := BitOr(fp1.Bits, fp2.Bits)
		if err != nil {
			return 0, err
		}

		popInter := float64(PopCount(intersection))
		popUnion := float64(PopCount(union))

		if popUnion == 0 {
			return 0.0, nil // Both are empty/zero vectors
		}
		return popInter / popUnion, nil
	}

	if fp1.IsDenseVector() && fp2.IsDenseVector() {
		// Generalized Tanimoto for continuous variables (Ruzicka similarity)
		// sum(min(a,b)) / sum(max(a,b))
		var num, den float64
		for i := 0; i < len(fp1.Vector); i++ {
			v1 := fp1.Vector[i]
			v2 := fp2.Vector[i]
			num += float64(min(v1, v2))
			den += float64(max(v1, v2))
		}
		if den == 0 {
			return 0.0, nil
		}
		return num / den, nil
	}

	return 0, errors.New(errors.ErrCodeValidation, "unsupported encoding or encoding mismatch")
}

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// DiceCalculator implements Dice coefficient.
type DiceCalculator struct{}

func (c *DiceCalculator) Metric() SimilarityMetric {
	return MetricDice
}

func (c *DiceCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingBitVector
}

func (c *DiceCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprints cannot be nil")
	}
	if !fp1.IsBitVector() || !fp2.IsBitVector() {
		return 0, errors.New(errors.ErrCodeValidation, "Dice only supports bit vectors")
	}
	if fp1.NumBits != fp2.NumBits {
		return 0, errors.New(errors.ErrCodeValidation, "fingerprint dimensions mismatch")
	}

	intersection, err := BitAnd(fp1.Bits, fp2.Bits)
	if err != nil { return 0, err }

	popInter := float64(PopCount(intersection))
	popA := float64(PopCount(fp1.Bits))
	popB := float64(PopCount(fp2.Bits))
	denom := popA + popB

	if denom == 0 {
		return 0.0, nil
	}
	return (2.0 * popInter) / denom, nil
}

// CosineCalculator implements Cosine similarity.
type CosineCalculator struct{}

func (c *CosineCalculator) Metric() SimilarityMetric {
	return MetricCosine
}

func (c *CosineCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingDenseVector || encoding == EncodingBitVector
}

func (c *CosineCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil { return 0, errors.New(errors.ErrCodeValidation, "nil fingerprints") }

	// Convert both to float vectors if one is bit vector
	v1 := fp1.ToFloat32Slice()
	v2 := fp2.ToFloat32Slice()

	if len(v1) != len(v2) {
		return 0, errors.New(errors.ErrCodeValidation, "dimensions mismatch")
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(v1); i++ {
		val1 := float64(v1[i])
		val2 := float64(v2[i])
		dotProduct += val1 * val2
		normA += val1 * val1
		normB += val2 * val2
	}

	if normA == 0 || normB == 0 {
		return 0.0, nil
	}

	cosine := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	// Normalize to [0, 1]
	return (cosine + 1.0) / 2.0, nil
}

// EuclideanCalculator implements Euclidean similarity.
type EuclideanCalculator struct{}

func (c *EuclideanCalculator) Metric() SimilarityMetric {
	return MetricEuclidean
}

func (c *EuclideanCalculator) SupportsEncoding(encoding FingerprintEncoding) bool {
	return encoding == EncodingDenseVector
}

func (c *EuclideanCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil { return 0, errors.New(errors.ErrCodeValidation, "fingerprints cannot be nil") }
	if !fp1.IsDenseVector() || !fp2.IsDenseVector() {
		return 0, errors.New(errors.ErrCodeValidation, "Euclidean only supports dense vectors")
	}
	v1 := fp1.Vector
	v2 := fp2.Vector
	if len(v1) != len(v2) {
		return 0, errors.New(errors.ErrCodeValidation, "dimensions mismatch")
	}

	var sumSq float64
	for i := 0; i < len(v1); i++ {
		diff := float64(v1[i] - v2[i])
		sumSq += diff * diff
	}
	dist := math.Sqrt(sumSq)
	return 1.0 / (1.0 + dist), nil
}

// ManhattanCalculator implements Manhattan similarity.
type ManhattanCalculator struct{}

func (c *ManhattanCalculator) Metric() SimilarityMetric { return MetricManhattan }
func (c *ManhattanCalculator) SupportsEncoding(encoding FingerprintEncoding) bool { return encoding == EncodingDenseVector }
func (c *ManhattanCalculator) Calculate(fp1, fp2 *Fingerprint) (float64, error) {
	if fp1 == nil || fp2 == nil { return 0, errors.New(errors.ErrCodeValidation, "fingerprints cannot be nil") }
	if !fp1.IsDenseVector() || !fp2.IsDenseVector() { return 0, errors.New(errors.ErrCodeValidation, "Manhattan only supports dense vectors") }
	v1 := fp1.Vector
	v2 := fp2.Vector
	if len(v1) != len(v2) { return 0, errors.New(errors.ErrCodeValidation, "dimensions mismatch") }

	var sumDiff float64
	for i := 0; i < len(v1); i++ {
		sumDiff += math.Abs(float64(v1[i] - v2[i]))
	}
	return 1.0 / (1.0 + sumDiff), nil
}

// NewSimilarityCalculator factory.
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
	case MetricManhattan:
		return &ManhattanCalculator{}, nil
	default:
		return nil, errors.New(errors.ErrCodeValidation, "unsupported metric: "+string(metric))
	}
}

// SimilarityResult represents a match in a similarity search.
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

// SimilarityEngine defines the interface for similarity operations.
type SimilarityEngine interface {
	SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error)
	ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error)
	BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error)
	RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error)
}

// SimilaritySearchOptions defines options for search.
type SimilaritySearchOptions struct {
	Metric                SimilarityMetric
	FingerprintType       FingerprintType
	Threshold             float64
	Limit                 int
	UseVectorDB           bool
	FusionStrategy        FingerprintFusionStrategy
	FusionFingerprintTypes []FingerprintType
	FusionWeights         map[FingerprintType]float64
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
		return errors.New(errors.ErrCodeValidation, "invalid metric")
	}
	if !o.FingerprintType.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid fingerprint type")
	}
	if o.Limit <= 0 {
		return errors.New(errors.ErrCodeValidation, "limit must be positive")
	}
	if o.Threshold < 0 || o.Threshold > 1 { // Allow 0 and 1
		return errors.New(errors.ErrCodeValidation, "threshold must be between 0 and 1")
	}
	return nil
}

// DefaultSimilarityEngine implements SimilarityEngine.
type DefaultSimilarityEngine struct {
	calculators map[SimilarityMetric]SimilarityCalculator
}

func NewDefaultSimilarityEngine() *DefaultSimilarityEngine {
	e := &DefaultSimilarityEngine{
		calculators: make(map[SimilarityMetric]SimilarityCalculator),
	}
	// Pre-register known calculators
	e.calculators[MetricTanimoto] = &TanimotoCalculator{}
	e.calculators[MetricDice] = &DiceCalculator{}
	e.calculators[MetricCosine] = &CosineCalculator{}
	e.calculators[MetricEuclidean] = &EuclideanCalculator{}
	return e
}

func (e *DefaultSimilarityEngine) ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error) {
	calc, ok := e.calculators[metric]
	if !ok {
		// Try to create on the fly or fail
		var err error
		calc, err = NewSimilarityCalculator(metric)
		if err != nil {
			return 0, err
		}
		e.calculators[metric] = calc
	}
	return calc.Calculate(fp1, fp2)
}

func (e *DefaultSimilarityEngine) BatchComputeSimilarity(target *Fingerprint, candidates []*Fingerprint, metric SimilarityMetric) ([]float64, error) {
	results := make([]float64, len(candidates))
	for i, c := range candidates {
		score, err := e.ComputeSimilarity(target, c, metric)
		if err != nil {
			return nil, err
		}
		results[i] = score
	}
	return results, nil
}

func (e *DefaultSimilarityEngine) SearchSimilar(ctx context.Context, target *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error) {
	// Default implementation returns error as this requires DB access or full scan which is not handled here.
	return nil, errors.New(errors.ErrCodeNotImplemented, "in-memory search not implemented")
}

func (e *DefaultSimilarityEngine) RankBySimilarity(ctx context.Context, target *Fingerprint, candidateIDs []string, metric SimilarityMetric) ([]*SimilarityResult, error) {
	return nil, errors.New(errors.ErrCodeNotImplemented, "ranking not implemented")
}

// Threshold constants
const (
	ThresholdIdentical          = 0.99
	ThresholdHighSimilarity     = 0.85
	ThresholdModerateSimilarity = 0.70
	ThresholdLowSimilarity      = 0.50
)

func ClassifySimilarity(score float64) string {
	if score < 0 {
		return "dissimilar"
	}
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
