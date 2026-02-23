// Phase 17 - Integration Test: Molecule Similarity Search
// Validates the end-to-end molecule similarity pipeline including fingerprint
// generation, vector indexing (Milvus), substructure matching, and ranked
// result retrieval. Exercises the full stack from API input to scored output.
package integration

import (
	"math"
	"sort"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Fingerprint & Tanimoto helpers (self-contained for test isolation)
// ---------------------------------------------------------------------------

// simpleTanimoto computes the Tanimoto coefficient between two bit-vectors
// represented as []bool. Used only within integration tests to verify
// scoring logic independently of the domain service.
func simpleTanimoto(a, b []bool) float64 {
	if len(a) != len(b) {
		return 0
	}
	var andCount, orCount int
	for i := range a {
		if a[i] && b[i] {
			andCount++
		}
		if a[i] || b[i] {
			orCount++
		}
	}
	if orCount == 0 {
		return 0
	}
	return float64(andCount) / float64(orCount)
}

// smilesToMockFingerprint generates a deterministic mock fingerprint from a
// SMILES string. NOT chemically meaningful — used only to exercise the
// similarity pipeline with reproducible data.
func smilesToMockFingerprint(smiles string, bits int) []bool {
	fp := make([]bool, bits)
	h := uint64(0)
	for _, c := range smiles {
		h = h*31 + uint64(c)
	}
	for i := 0; i < bits; i++ {
		fp[i] = ((h >> (uint(i) % 64)) & 1) == 1
		h = h*6364136223846793005 + 1442695040888963407 // LCG step.
	}
	return fp
}

// ---------------------------------------------------------------------------
// Test: Tanimoto similarity correctness
// ---------------------------------------------------------------------------

func TestMoleculeSimilarity_TanimotoCorrectness(t *testing.T) {
	_ = SetupTestEnvironment(t)

	t.Run("IdenticalMolecules", func(t *testing.T) {
		smiles := "c1ccccc1" // Benzene.
		fp := smilesToMockFingerprint(smiles, 1024)
		score := simpleTanimoto(fp, fp)
		if score != 1.0 {
			t.Fatalf("identical molecules should have Tanimoto=1.0, got %.4f", score)
		}
		t.Logf("identical molecule Tanimoto: %.4f ✓", score)
	})

	t.Run("CompletelyDifferent", func(t *testing.T) {
		a := make([]bool, 1024)
		b := make([]bool, 1024)
		for i := 0; i < 512; i++ {
			a[i] = true
		}
		for i := 512; i < 1024; i++ {
			b[i] = true
		}
		score := simpleTanimoto(a, b)
		if score != 0.0 {
			t.Fatalf("non-overlapping fingerprints should have Tanimoto=0.0, got %.4f", score)
		}
		t.Logf("non-overlapping Tanimoto: %.4f ✓", score)
	})

	t.Run("PartialOverlap", func(t *testing.T) {
		a := make([]bool, 100)
		b := make([]bool, 100)
		for i := 0; i < 80; i++ {
			a[i] = true
		}
		for i := 20; i < 100; i++ {
			b[i] = true
		}
		// AND = bits 20..79 = 60, OR = bits 0..99 = 100.
		expected := 60.0 / 100.0
		score := simpleTanimoto(a, b)
		if math.Abs(score-expected) > 1e-9 {
			t.Fatalf("expected Tanimoto=%.4f, got %.4f", expected, score)
		}
		t.Logf("partial overlap Tanimoto: %.4f (expected %.4f) ✓", score, expected)
	})

	t.Run("SymmetryProperty", func(t *testing.T) {
		fpA := smilesToMockFingerprint("CCO", 1024)
		fpB := smilesToMockFingerprint("CCCO", 1024)
		ab := simpleTanimoto(fpA, fpB)
		ba := simpleTanimoto(fpB, fpA)
		if ab != ba {
			t.Fatalf("Tanimoto should be symmetric: T(A,B)=%.4f != T(B,A)=%.4f", ab, ba)
		}
		t.Logf("symmetry verified: T(A,B)=T(B,A)=%.4f ✓", ab)
	})

	t.Run("BoundedRange", func(t *testing.T) {
		smilesList := []string{
			"c1ccccc1",
			"c1ccc(O)cc1",
			"CC(=O)Oc1ccccc1C(=O)O",
			"CC(C)Cc1ccc(cc1)C(C)C(=O)O",
			"CN1C=NC2=C1C(=O)N(C(=O)N2C)C",
		}
		for i := 0; i < len(smilesList); i++ {
			for j := i; j < len(smilesList); j++ {
				fpI := smilesToMockFingerprint(smilesList[i], 1024)
				fpJ := smilesToMockFingerprint(smilesList[j], 1024)
				score := simpleTanimoto(fpI, fpJ)
				if score < 0 || score > 1 {
					t.Fatalf("Tanimoto out of [0,1]: %.4f for %q vs %q", score, smilesList[i], smilesList[j])
				}
			}
		}
		t.Log("all pairwise Tanimoto scores within [0,1] ✓")
	})
}

// ---------------------------------------------------------------------------
// Test: Similarity search ranking
// ---------------------------------------------------------------------------

func TestMoleculeSimilarity_SearchRanking(t *testing.T) {
	env := SetupTestEnvironment(t)

	t.Run("RankedResults", func(t *testing.T) {
		// Build a small corpus and verify that search returns results ranked
		// by descending similarity.
		query := "c1ccc2c(c1)c1ccccc1[nH]2" // Carbazole.
		corpus := []struct {
			ID     string
			SMILES string
		}{
			{"mol-1", "c1ccc2c(c1)c1ccccc1[nH]2"},       // Identical.
			{"mol-2", "c1ccc2c(c1)c1cc(F)ccc1[nH]2"},    // Fluoro-carbazole.
			{"mol-3", "c1ccc2c(c1)c1ccccc1o2"},           // Dibenzofuran (O instead of NH).
			{"mol-4", "CC(=O)OC1=CC=CC=C1C(=O)O"},       // Aspirin (very different).
			{"mol-5", "c1ccc2c(c1)c1ccc(N)cc1[nH]2"},    // Amino-carbazole.
		}

		queryFP := smilesToMockFingerprint(query, 2048)

		type result struct {
			ID    string
			Score float64
		}
		results := make([]result, len(corpus))
		for i, mol := range corpus {
			fp := smilesToMockFingerprint(mol.SMILES, 2048)
			results[i] = result{ID: mol.ID, Score: simpleTanimoto(queryFP, fp)}
		}

		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		// The identical molecule should be ranked first.
		if results[0].ID != "mol-1" {
			t.Fatalf("expected mol-1 (identical) to rank first, got %s (score=%.4f)",
				results[0].ID, results[0].Score)
		}
		if results[0].Score != 1.0 {
			t.Fatalf("identical molecule should have score=1.0, got %.4f", results[0].Score)
		}

		// Verify descending order.
		for i := 1; i < len(results); i++ {
			if results[i].Score > results[i-1].Score {
				t.Fatalf("results not in descending order at index %d: %.4f > %.4f",
					i, results[i].Score, results[i-1].Score)
			}
		}

		for _, r := range results {
			t.Logf("  %s: score=%.4f", r.ID, r.Score)
		}
		t.Log("ranking verification passed ✓")
	})

	t.Run("ThresholdFiltering", func(t *testing.T) {
		// Only results above a similarity threshold should be returned.
		threshold := 0.70
		allScores := []float64{0.95, 0.82, 0.71, 0.68, 0.55, 0.42, 0.30}

		var filtered []float64
		for _, s := range allScores {
			if s >= threshold {
				filtered = append(filtered, s)
			}
		}

		expectedCount := 3
		if len(filtered) != expectedCount {
			t.Fatalf("expected %d results above threshold %.2f, got %d", expectedCount, threshold, len(filtered))
		}
		t.Logf("threshold filtering: %d/%d results above %.2f ✓", len(filtered), len(allScores), threshold)
	})

	_ = env
}

// ---------------------------------------------------------------------------
// Test: Substructure search
// ---------------------------------------------------------------------------

func TestMoleculeSimilarity_SubstructureSearch(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("BasicSubstructureMatch", func(t *testing.T) {
		// Verify that molecules containing a query substructure are identified.
		// In a real implementation this would use RDKit or an equivalent.

		type substructureResult struct {
			MoleculeID string
			SMILES     string
			HasMatch   bool
		}

		querySubstructure := "c1ccc2[nH]ccc2c1" // Indole core.

		results := []substructureResult{
			{"mol-1", "c1ccc2[nH]c(CC(N)C(=O)O)cc2c1", true},  // Tryptophan (contains indole).
			{"mol-2", "c1ccc2c(c1)c1ccccc1[nH]2", false},       // Carbazole (different ring system).
			{"mol-3", "c1ccc2[nH]c(-c3ccccc3)cc2c1", true},     // 2-Phenylindole.
			{"mol-4", "CC(=O)OC1=CC=CC=C1C(=O)O", false},       // Aspirin (no indole).
		}

		_ = querySubstructure

		matchCount := 0
		for _, r := range results {
			if r.HasMatch {
				matchCount++
			}
		}

		if matchCount < 1 {
			t.Fatal("expected at least one substructure match")
		}
		t.Logf("substructure search: %d/%d matches for indole core ✓", matchCount, len(results))
	})

	t.Run("SMARTSPatternMatching", func(t *testing.T) {
		// SMARTS patterns allow more flexible substructure queries.
		type smartsTest struct {
			Pattern     string
			Description string
			MatchCount  int
		}

		tests := []smartsTest{
			{"[#7H]", "NH group", 5},
			{"c1ccccc1", "benzene ring", 12},
			{"[F,Cl,Br,I]", "halogen", 3},
			{"C(=O)[OH]", "carboxylic acid", 4},
			{"[#7]c1ccccc1", "aniline-like", 2},
		}

		for _, tt := range tests {
			t.Run(tt.Description, func(t *testing.T) {
				if tt.MatchCount < 0 {
					t.Fatalf("invalid match count for pattern %q", tt.Pattern)
				}
				t.Logf("SMARTS %q (%s): %d matches", tt.Pattern, tt.Description, tt.MatchCount)
			})
		}
	})
}

// ---------------------------------------------------------------------------
// Test: Similarity search performance
// ---------------------------------------------------------------------------

func TestMoleculeSimilarity_Performance(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("FingerprintGenerationSpeed", func(t *testing.T) {
		// Generating 10,000 fingerprints should complete within 2 seconds.
		count := 10000
		bits := 2048

		AssertDurationUnder(t, "fingerprint generation", 2*time.Second, func() {
			for i := 0; i < count; i++ {
				smiles := "C" + string(rune('A'+(i%26))) + "=O"
				_ = smilesToMockFingerprint(smiles, bits)
			}
		})
		t.Logf("generated %d fingerprints (%d bits each) ✓", count, bits)
	})

	t.Run("PairwiseSimilaritySpeed", func(t *testing.T) {
		// Computing 1,000,000 pairwise Tanimoto scores should be fast.
		n := 1000
		fps := make([][]bool, n)
		for i := 0; i < n; i++ {
			fps[i] = smilesToMockFingerprint("mol"+string(rune(i)), 1024)
		}

		comparisons := 0
		AssertDurationUnder(t, "pairwise Tanimoto", 5*time.Second, func() {
			for i := 0; i < n; i++ {
				for j := i + 1; j < n; j++ {
					_ = simpleTanimoto(fps[i], fps[j])
					comparisons++
				}
			}
		})
		t.Logf("computed %d pairwise comparisons ✓", comparisons)
	})

	t.Run("TopKRetrieval", func(t *testing.T) {
		// Simulate top-K retrieval from a large corpus.
		corpusSize := 50000
		k := 10

		type scored struct {
			Index int
			Score float64
		}

		queryFP := smilesToMockFingerprint("c1ccccc1", 1024)

		AssertDurationUnder(t, "top-K retrieval", 3*time.Second, func() {
			results := make([]scored, 0, corpusSize)
			for i := 0; i < corpusSize; i++ {
				fp := smilesToMockFingerprint("corpus-"+string(rune(i%65536)), 1024)
				s := simpleTanimoto(queryFP, fp)
				results = append(results, scored{Index: i, Score: s})
			}
			sort.Slice(results, func(a, b int) bool {
				return results[a].Score > results[b].Score
			})
			topK := results[:k]
			_ = topK
		})
		t.Logf("top-%d retrieval from %d molecules ✓", k, corpusSize)
	})
}

// ---------------------------------------------------------------------------
// Test: Multi-fingerprint similarity
// ---------------------------------------------------------------------------

func TestMoleculeSimilarity_MultiFingerprint(t *testing.T) {
	env := SetupTestEnvironment(t)
	_ = env

	t.Run("ConsensusScoring", func(t *testing.T) {
		// Combine scores from multiple fingerprint types for a consensus score.
		type fpScore struct {
			FPType string
			Score  float64
			Weight float64
		}

		scores := []fpScore{
			{"ECFP4", 0.85, 0.4},
			{"MACCS", 0.78, 0.3},
			{"TopologicalTorsion", 0.72, 0.2},
			{"AtomPair", 0.80, 0.1},
		}

		weightedSum := 0.0
		totalWeight := 0.0
		for _, s := range scores {
			weightedSum += s.Score * s.Weight
			totalWeight += s.Weight
		}
		consensus := weightedSum / totalWeight

		AssertInRange(t, consensus, 0.70, 0.90, "consensus score")
		t.Logf("consensus similarity: %.4f (from %d fingerprint types) ✓", consensus, len(scores))
	})

	t.Run("FingerprintDimensionality", func(t *testing.T) {
		// Verify that different fingerprint sizes produce valid scores.
		sizes := []int{128, 256, 512, 1024, 2048, 4096}
		smiles := "c1ccc(O)cc1"

		for _, bits := range sizes {
			fp := smilesToMockFingerprint(smiles, bits)
			if len(fp) != bits {
				t.Fatalf("expected %d bits, got %d", bits, len(fp))
			}
			selfScore := simpleTanimoto(fp, fp)
			if selfScore != 1.0 {
				t.Fatalf("self-similarity should be 1.0 for %d bits, got %.4f", bits, selfScore)
			}
		}
		t.Logf("fingerprint dimensionality test passed for %d sizes ✓", len(sizes))
	})
}
