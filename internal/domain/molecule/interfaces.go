package molecule

import (
	"context"
)

// StructuralIdentifiers holds computed chemical identifiers.
type StructuralIdentifiers struct {
	CanonicalSMILES string
	InChI           string
	InChIKey        string
	Formula         string
	Weight          float64
}

// FingerprintCalcOptions defines parameters for fingerprint generation.
type FingerprintCalcOptions struct {
	Radius int
	Bits   int
}

// DefaultFingerprintCalcOptions returns default options.
func DefaultFingerprintCalcOptions() FingerprintCalcOptions {
	return FingerprintCalcOptions{Radius: 2, Bits: 2048}
}

// FingerprintCalculator defines the interface for calculating fingerprints.
type FingerprintCalculator interface {
	Standardize(ctx context.Context, smiles string) (*StructuralIdentifiers, error)
	Calculate(ctx context.Context, smiles string, fpType FingerprintType, opts FingerprintCalcOptions) (*Fingerprint, error)
	BatchCalculate(ctx context.Context, smilesList []string, fpType FingerprintType, opts FingerprintCalcOptions) ([]*Fingerprint, error)
}

// SimilarityMetric defines the algorithm for comparing fingerprints.
type SimilarityMetric string

const (
	MetricTanimoto SimilarityMetric = "tanimoto"
	MetricDice     SimilarityMetric = "dice"
	MetricCosine   SimilarityMetric = "cosine"
)

// SimilarityResult represents a match in a similarity search.
type SimilarityResult struct {
	MoleculeID string  `json:"molecule_id"`
	Score      float64 `json:"score"`
	SMILES     string  `json:"smiles"`
}

// SimilarityEngine defines the interface for similarity operations.
type SimilarityEngine interface {
	ComputeSimilarity(fp1, fp2 *Fingerprint, metric SimilarityMetric) (float64, error)
	SearchSimilar(ctx context.Context, queryFP *Fingerprint, metric SimilarityMetric, threshold float64, limit int) ([]*SimilarityResult, error)
}

// FingerprintEncoding is a placeholder for encoding type if used in similarity
type FingerprintEncoding int

// FingerprintFusionStrategy defines how to combine multiple similarity scores.
type FingerprintFusionStrategy interface {
	Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error)
}

// WeightedAverageFusion implements FingerprintFusionStrategy.
type WeightedAverageFusion struct{}

func (f *WeightedAverageFusion) Fuse(scores map[FingerprintType]float64, weights map[FingerprintType]float64) (float64, error) {
	var totalScore, totalWeight float64
	for t, s := range scores {
		w := 1.0
		if weights != nil {
			if val, ok := weights[t]; ok {
				w = val
			}
		}
		totalScore += s * w
		totalWeight += w
	}
	if totalWeight == 0 {
		return 0, nil
	}
	return totalScore / totalWeight, nil
}

const (
	FingerprintMorgan FingerprintType = "morgan"
	FingerprintMACCS  FingerprintType = "maccs"
	FingerprintGNN    FingerprintType = "gnn"
)

func (fp *Fingerprint) IsDenseVector() bool {
	return len(fp.Vector) > 0
}

//Personal.AI order the ending
