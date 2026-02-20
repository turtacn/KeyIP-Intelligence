// Package molecule provides molecular similarity computation algorithms for
// chemical structure comparison in the KeyIP-Intelligence platform.
package molecule

import (
	"fmt"
	"math"
	"math/bits"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ─────────────────────────────────────────────────────────────────────────────
// Tanimoto Similarity
// ─────────────────────────────────────────────────────────────────────────────

// TanimotoSimilarity computes the Tanimoto coefficient (Jaccard index) between
// two molecular fingerprints.  This is the most commonly used similarity metric
// in cheminformatics.
//
// Formula: |A ∩ B| / |A ∪ B| = |A ∩ B| / (|A| + |B| - |A ∩ B|)
//
// Returns a value in [0.0, 1.0] where:
//   - 1.0 indicates identical fingerprints
//   - 0.0 indicates no common bits set
//   - ≥ 0.85 is typically considered "highly similar" in patent analysis
//
// Both fingerprints must have the same length and type.
func TanimotoSimilarity(fp1, fp2 *Fingerprint) (float64, error) {
	if err := validateFingerprints(fp1, fp2); err != nil {
		return 0, err
	}

	// Handle edge case: both fingerprints are all-zeros
	if fp1.NumOnBits == 0 && fp2.NumOnBits == 0 {
		return 1.0, nil // Consider two empty sets as identical
	}

	// Calculate intersection (AND operation)
	intersection := andCount(fp1.Bits, fp2.Bits)

	// Calculate union using inclusion-exclusion principle
	union := fp1.NumOnBits + fp2.NumOnBits - intersection

	if union == 0 {
		return 0.0, nil
	}

	return float64(intersection) / float64(union), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Cosine Similarity
// ─────────────────────────────────────────────────────────────────────────────

// CosineSimilarity computes the cosine of the angle between two fingerprint
// vectors.  This treats fingerprints as binary vectors in high-dimensional space.
//
// Formula: (A · B) / (||A|| × ||B||)
//
// For binary vectors: (A · B) = |A ∩ B|
// ||A|| = sqrt(|A|), ||B|| = sqrt(|B|)
//
// Returns a value in [0.0, 1.0].
func CosineSimilarity(fp1, fp2 *Fingerprint) (float64, error) {
	if err := validateFingerprints(fp1, fp2); err != nil {
		return 0, err
	}

	if fp1.NumOnBits == 0 || fp2.NumOnBits == 0 {
		return 0.0, nil
	}

	intersection := andCount(fp1.Bits, fp2.Bits)

	// Cosine = dot_product / (norm1 * norm2)
	// For binary vectors: dot_product = intersection count
	// norm = sqrt(number of 1s)
	norm1 := math.Sqrt(float64(fp1.NumOnBits))
	norm2 := math.Sqrt(float64(fp2.NumOnBits))

	return float64(intersection) / (norm1 * norm2), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Dice Similarity
// ─────────────────────────────────────────────────────────────────────────────

// DiceSimilarity computes the Dice coefficient (Sørensen–Dice index) between
// two fingerprints.  This metric gives more weight to the intersection than
// Tanimoto.
//
// Formula: 2 × |A ∩ B| / (|A| + |B|)
//
// Returns a value in [0.0, 1.0].  Dice similarity is always ≥ Tanimoto similarity
// for the same fingerprint pair.
func DiceSimilarity(fp1, fp2 *Fingerprint) (float64, error) {
	if err := validateFingerprints(fp1, fp2); err != nil {
		return 0, err
	}

	if fp1.NumOnBits == 0 && fp2.NumOnBits == 0 {
		return 1.0, nil
	}

	denominator := fp1.NumOnBits + fp2.NumOnBits
	if denominator == 0 {
		return 0.0, nil
	}

	intersection := andCount(fp1.Bits, fp2.Bits)

	return 2.0 * float64(intersection) / float64(denominator), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Tversky Similarity
// ─────────────────────────────────────────────────────────────────────────────

// TverskySimilarity computes the asymmetric Tversky index, which generalizes
// both Tanimoto and Dice coefficients.
//
// Formula: |A ∩ B| / (|A ∩ B| + α|A - B| + β|B - A|)
//
// Where:
//   - α, β ∈ [0, 1] are weighting parameters
//   - α = β = 0.5 gives Tanimoto similarity
//   - α = β = 1.0 gives Dice similarity
//   - α > β emphasizes features in A
//   - α < β emphasizes features in B
//
// This is useful for asymmetric similarity queries where one molecule (e.g., a
// patent claim) is considered more important than the other (e.g., a prior art).
func TverskySimilarity(fp1, fp2 *Fingerprint, alpha, beta float64) (float64, error) {
	if err := validateFingerprints(fp1, fp2); err != nil {
		return 0, err
	}

	if alpha < 0 || beta < 0 {
		return 0, errors.InvalidParam("alpha and beta must be non-negative").
			WithDetail(fmt.Sprintf("alpha=%f, beta=%f", alpha, beta))
	}

	intersection := andCount(fp1.Bits, fp2.Bits)

	// |A - B| = |A| - |A ∩ B|
	aMinusB := fp1.NumOnBits - intersection
	// |B - A| = |B| - |A ∩ B|
	bMinusA := fp2.NumOnBits - intersection

	denominator := float64(intersection) + alpha*float64(aMinusB) + beta*float64(bMinusA)

	if denominator == 0 {
		if fp1.NumOnBits == 0 && fp2.NumOnBits == 0 {
			return 1.0, nil
		}
		return 0.0, nil
	}

	return float64(intersection) / denominator, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper Functions
// ─────────────────────────────────────────────────────────────────────────────

// validateFingerprints checks that two fingerprints are compatible for similarity
// comparison (same length and type).
func validateFingerprints(fp1, fp2 *Fingerprint) error {
	if fp1 == nil || fp2 == nil {
		return errors.InvalidParam("fingerprints cannot be nil")
	}

	if fp1.Length != fp2.Length {
		return errors.InvalidParam("fingerprints must have same length").
			WithDetail(fmt.Sprintf("fp1=%d, fp2=%d", fp1.Length, fp2.Length))
	}

	if fp1.Type != fp2.Type {
		return errors.InvalidParam("fingerprints must have same type").
			WithDetail(fmt.Sprintf("fp1=%s, fp2=%s", fp1.Type, fp2.Type))
	}

	return nil
}

// popcount counts the number of set bits in a byte slice using the built-in
// bits.OnesCount8 function for efficiency.
func popcount(data []byte) int {
	count := 0
	for _, b := range data {
		count += bits.OnesCount8(uint8(b))
	}
	return count
}

// andCount computes the number of bits set in the bitwise AND of two byte slices.
// This represents the size of the intersection of two binary sets.
func andCount(a, b []byte) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	count := 0
	for i := 0; i < minLen; i++ {
		count += bits.OnesCount8(uint8(a[i] & b[i]))
	}
	return count
}

//Personal.AI order the ending
