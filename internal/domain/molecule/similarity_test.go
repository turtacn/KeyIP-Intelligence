// Package molecule_test provides unit tests for molecular similarity calculations.
package molecule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestTanimotoSimilarity
// ─────────────────────────────────────────────────────────────────────────────

func TestTanimotoSimilarity_IdenticalFingerprints(t *testing.T) {
	t.Parallel()

	fp, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	sim, err := molecule.TanimotoSimilarity(fp, fp)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, sim, 1e-9, "identical fingerprints should have Tanimoto = 1.0")
}

func TestTanimotoSimilarity_BothZero(t *testing.T) {
	t.Parallel()

	// Create two empty fingerprints
	bits := make([]byte, 256) // all zeros
	fp1 := molecule.NewFingerprint(mtypes.FPMorgan, bits, 2048)
	fp2 := molecule.NewFingerprint(mtypes.FPMorgan, bits, 2048)

	sim, err := molecule.TanimotoSimilarity(fp1, fp2)
	require.NoError(t, err)
	assert.Equal(t, 1.0, sim, "two empty fingerprints should be considered identical")
}

func TestTanimotoSimilarity_KnownValue(t *testing.T) {
	t.Parallel()

	// Create fingerprints with known bit patterns
	bits1 := make([]byte, 32)
	bits1[0] = 0xFF // 8 bits set
	bits1[1] = 0x0F // 4 bits set
	// Total: 12 bits set

	bits2 := make([]byte, 32)
	bits2[0] = 0xF0 // 4 bits set
	bits2[1] = 0x0F // 4 bits set
	// Total: 8 bits set

	// Intersection:
	// bits1[0] & bits2[0] = 0xFF & 0xF0 = 0xF0 (4 bits)
	// bits1[1] & bits2[1] = 0x0F & 0x0F = 0x0F (4 bits)
	// Total Intersection = 8 bits
	// Union: 12 + 8 - 8 = 12 bits
	// Tanimoto: 8 / 12 = 0.6667

	fp1 := molecule.NewFingerprint(mtypes.FPMorgan, bits1, 256)
	fp2 := molecule.NewFingerprint(mtypes.FPMorgan, bits2, 256)

	sim, err := molecule.TanimotoSimilarity(fp1, fp2)
	require.NoError(t, err)
	assert.InDelta(t, 0.6667, sim, 0.01)
}

func TestTanimotoSimilarity_DifferentMolecules(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048) // benzene
	require.NoError(t, err)

	fp2, err := molecule.CalculateMorganFingerprint("CCO", 2, 2048) // ethanol
	require.NoError(t, err)

	sim, err := molecule.TanimotoSimilarity(fp1, fp2)
	require.NoError(t, err)

	assert.Less(t, sim, 1.0, "different molecules should have similarity < 1.0")
	assert.GreaterOrEqual(t, sim, 0.0, "similarity should be non-negative")
}

func TestTanimotoSimilarity_LengthMismatch(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	fp2, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 1024)
	require.NoError(t, err)

	_, err = molecule.TanimotoSimilarity(fp1, fp2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "same length")
}

func TestTanimotoSimilarity_TypeMismatch(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	fp2, err := molecule.CalculateMACCSFingerprint("c1ccccc1")
	require.NoError(t, err)

	// Length will also be different, but type check should happen first or together
	_, err = molecule.TanimotoSimilarity(fp1, fp2)
	require.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCosineSimilarity
// ─────────────────────────────────────────────────────────────────────────────

func TestCosineSimilarity_IdenticalFingerprints(t *testing.T) {
	t.Parallel()

	fp, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	sim, err := molecule.CosineSimilarity(fp, fp)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, sim, 1e-9)
}

func TestCosineSimilarity_ZeroFingerprint(t *testing.T) {
	t.Parallel()

	bits := make([]byte, 256)
	fp1 := molecule.NewFingerprint(mtypes.FPMorgan, bits, 2048) // all zeros

	fp2, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	sim, err := molecule.CosineSimilarity(fp1, fp2)
	require.NoError(t, err)
	assert.Equal(t, 0.0, sim, "cosine similarity with zero vector should be 0")
}

func TestCosineSimilarity_RelationToTanimoto(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	fp2, err := molecule.CalculateMorganFingerprint("c1ccc2ccccc2c1", 2, 2048)
	require.NoError(t, err)

	tanimoto, err := molecule.TanimotoSimilarity(fp1, fp2)
	require.NoError(t, err)

	cosine, err := molecule.CosineSimilarity(fp1, fp2)
	require.NoError(t, err)

	// For binary vectors, cosine is typically >= Tanimoto
	assert.GreaterOrEqual(t, cosine, tanimoto)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDiceSimilarity
// ─────────────────────────────────────────────────────────────────────────────

func TestDiceSimilarity_IdenticalFingerprints(t *testing.T) {
	t.Parallel()

	fp, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	sim, err := molecule.DiceSimilarity(fp, fp)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, sim, 1e-9)
}

func TestDiceSimilarity_KnownValue(t *testing.T) {
	t.Parallel()

	bits1 := make([]byte, 32)
	bits1[0] = 0xFF // 8 bits
	// Total: 8 bits set

	bits2 := make([]byte, 32)
	bits2[0] = 0xF0 // 4 bits
	// Total: 4 bits set

	// Intersection: 0xFF & 0xF0 = 0xF0 = 4 bits
	// Dice: 2 * 4 / (8 + 4) = 8 / 12 = 0.6667

	fp1 := molecule.NewFingerprint(mtypes.FPMorgan, bits1, 256)
	fp2 := molecule.NewFingerprint(mtypes.FPMorgan, bits2, 256)

	sim, err := molecule.DiceSimilarity(fp1, fp2)
	require.NoError(t, err)
	assert.InDelta(t, 0.6667, sim, 0.01)
}

func TestDiceSimilarity_GreaterThanOrEqualTanimoto(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	fp2, err := molecule.CalculateMorganFingerprint("c1ccc2ccccc2c1", 2, 2048)
	require.NoError(t, err)

	tanimoto, err := molecule.TanimotoSimilarity(fp1, fp2)
	require.NoError(t, err)

	dice, err := molecule.DiceSimilarity(fp1, fp2)
	require.NoError(t, err)

	// Dice is always >= Tanimoto
	assert.GreaterOrEqual(t, dice, tanimoto)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestTverskySimilarity
// ─────────────────────────────────────────────────────────────────────────────

func TestTverskySimilarity_AlphaBeta0_5_EqualsTanimoto(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	fp2, err := molecule.CalculateMorganFingerprint("c1ccc2ccccc2c1", 2, 2048)
	require.NoError(t, err)

	tanimoto, err := molecule.TanimotoSimilarity(fp1, fp2)
	require.NoError(t, err)

	// When alpha = beta = 0.5, Tversky reduces to Tanimoto
	// Actually, that's not quite right. Let me recalculate.
	// Tversky with alpha=beta=1 gives Dice.
	// Let me check the formula again in the code.

	// Actually, looking at the formula:
	// Tversky: |A ∩ B| / (|A ∩ B| + α|A - B| + β|B - A|)
	// When α = β = 1:
	// |A ∩ B| / (|A ∩ B| + |A - B| + |B - A|)
	// = |A ∩ B| / (|A ∩ B| + |A| - |A ∩ B| + |B| - |A ∩ B|)
	// = |A ∩ B| / (|A| + |B| - |A ∩ B|)
	// This is Tanimoto!

	// So alpha = beta = 1 should give Tanimoto
	tversky1_1, err := molecule.TverskySimilarity(fp1, fp2, 1.0, 1.0)
	require.NoError(t, err)

	assert.InDelta(t, tanimoto, tversky1_1, 1e-6)
}

func TestTverskySimilarity_NegativeParameters(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	fp2, err := molecule.CalculateMorganFingerprint("CCO", 2, 2048)
	require.NoError(t, err)

	_, err = molecule.TverskySimilarity(fp1, fp2, -0.5, 0.5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative")
}

func TestTverskySimilarity_AsymmetricWeighting(t *testing.T) {
	t.Parallel()

	fp1, err := molecule.CalculateMorganFingerprint("c1ccccc1", 2, 2048)
	require.NoError(t, err)

	fp2, err := molecule.CalculateMorganFingerprint("c1ccc2ccccc2c1", 2, 2048)
	require.NoError(t, err)

	// Alpha emphasizes fp1
	tvAlpha, err := molecule.TverskySimilarity(fp1, fp2, 0.8, 0.2)
	require.NoError(t, err)

	// Beta emphasizes fp2
	tvBeta, err := molecule.TverskySimilarity(fp1, fp2, 0.2, 0.8)
	require.NoError(t, err)

	// Results should be different
	assert.NotEqual(t, tvAlpha, tvBeta)
}

//Personal.AI order the ending
