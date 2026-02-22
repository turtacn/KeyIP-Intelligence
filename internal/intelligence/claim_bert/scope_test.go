package claim_bert

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
)

// ============================================================================
// Test helpers & factories
// ============================================================================

// mockClaimEmbedder is a test double for ClaimEmbedder.
type mockClaimEmbedder struct {
	featureEmbeddings map[string][]float32
	claimEmbeddings   map[int][]float32
	embedDim          int
}

func newMockClaimEmbedder(dim int) *mockClaimEmbedder {
	return &mockClaimEmbedder{
		featureEmbeddings: make(map[string][]float32),
		claimEmbeddings:   make(map[int][]float32),
		embedDim:          dim,
	}
}

func (m *mockClaimEmbedder) EmbedFeature(_ context.Context, feat *TechnicalFeature) ([]float32, error) {
	if feat == nil {
		return nil, fmt.Errorf("nil feature")
	}
	if emb, ok := m.featureEmbeddings[feat.ID]; ok {
		return emb, nil
	}
	// Generate a deterministic embedding based on the feature ID hash.
	emb := make([]float32, m.embedDim)
	hash := uint32(0)
	for _, c := range feat.ID {
		hash = hash*31 + uint32(c)
	}
	for i := range emb {
		hash = hash*1103515245 + 12345
		emb[i] = float32(hash%1000) / 1000.0
	}
	// Normalize to unit vector.
	var norm float64
	for _, v := range emb {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range emb {
			emb[i] = float32(float64(emb[i]) / norm)
		}
	}
	return emb, nil
}

func (m *mockClaimEmbedder) EmbedClaim(_ context.Context, claim *ParsedClaim) ([]float32, error) {
	if claim == nil {
		return nil, fmt.Errorf("nil claim")
	}
	if emb, ok := m.claimEmbeddings[claim.ClaimNumber]; ok {
		return emb, nil
	}
	emb := make([]float32, m.embedDim)
	for i := range emb {
		emb[i] = float32(claim.ClaimNumber) * 0.01 * float32(i+1)
	}
	return emb, nil
}

// registerIdenticalEmbedding registers the same embedding for two feature IDs
// so they will be matched as identical.
func (m *mockClaimEmbedder) registerIdenticalEmbedding(idA, idB string) {
	emb := make([]float32, m.embedDim)
	for i := range emb {
		emb[i] = float32(i+1) * 0.1
	}
	var norm float64
	for _, v := range emb {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	for i := range emb {
		emb[i] = float32(float64(emb[i]) / norm)
	}
	m.featureEmbeddings[idA] = emb
	m.featureEmbeddings[idB] = emb
}

// registerOrthogonalEmbeddings registers two embeddings that are orthogonal
// (cosine similarity ≈ 0).
func (m *mockClaimEmbedder) registerOrthogonalEmbeddings(idA, idB string) {
	embA := make([]float32, m.embedDim)
	embB := make([]float32, m.embedDim)
	// A = [1, 0, 0, ...], B = [0, 1, 0, ...]
	if m.embedDim >= 2 {
		embA[0] = 1.0
		embB[1] = 1.0
	}
	m.featureEmbeddings[idA] = embA
	m.featureEmbeddings[idB] = embB
}

// ---------------------------------------------------------------------------
// Factory functions for test claims
// ---------------------------------------------------------------------------

// makeFeature creates a TechnicalFeature with the given ID and optional embedding.
func makeFeature(id, description string, essential bool, embedding []float32) *TechnicalFeature {
	return &TechnicalFeature{
		ID:          id,
		Description: description,
		IsEssential: essential,
		Embedding:   embedding,
	}
}

// makeIndependentClaim creates an independent ParsedClaim with the specified
// number of features and transitional phrase.
func makeIndependentClaim(claimNum int, featureCount int, phrase TransitionalPhraseType) *ParsedClaim {
	features := make([]*TechnicalFeature, featureCount)
	for i := 0; i < featureCount; i++ {
		features[i] = makeFeature(
			fmt.Sprintf("c%d-f%d", claimNum, i),
			fmt.Sprintf("Feature %d of claim %d with some technical description text", i, claimNum),
			i == 0, // first feature is essential
			nil,
		)
	}
	return &ParsedClaim{
		ClaimNumber:        claimNum,
		ClaimType:          ClaimTypeIndependent,
		TransitionalPhrase: phrase,
		Features:           features,
		DependsOn:          nil,
		ScopeScore:         0.55, // neutral base from model
		Category:           "product",
		RawText:            fmt.Sprintf("An apparatus comprising %d elements...", featureCount),
	}
}

// makeDependentClaim creates a dependent ParsedClaim that depends on the given parent.
func makeDependentClaim(claimNum int, dependsOn int, additionalFeatures int, phrase TransitionalPhraseType) *ParsedClaim {
	features := make([]*TechnicalFeature, additionalFeatures)
	for i := 0; i < additionalFeatures; i++ {
		features[i] = makeFeature(
			fmt.Sprintf("c%d-f%d", claimNum, i),
			fmt.Sprintf("Additional feature %d of dependent claim %d", i, claimNum),
			false,
			nil,
		)
	}
	return &ParsedClaim{
		ClaimNumber:        claimNum,
		ClaimType:          ClaimTypeDependent,
		TransitionalPhrase: phrase,
		Features:           features,
		DependsOn:          []int{dependsOn},
		ScopeScore:         0.35,
		Category:           "product",
		RawText:            fmt.Sprintf("The apparatus of claim %d, further comprising...", dependsOn),
	}
}

// makeClaimSet creates a ParsedClaimSet with the specified number of independent
// claims and dependent claims per independent.
func makeClaimSet(independentCount, dependentPerIndependent int) *ParsedClaimSet {
	claims := make([]*ParsedClaim, 0)
	claimNum := 1

	for i := 0; i < independentCount; i++ {
		indep := makeIndependentClaim(claimNum, 4, PhraseComprising)
		// Vary categories across independent claims.
		categories := []string{"product", "method", "composition", "use"}
		indep.Category = categories[i%len(categories)]
		claims = append(claims, indep)
		parentNum := claimNum
		claimNum++

		for j := 0; j < dependentPerIndependent; j++ {
			dep := makeDependentClaim(claimNum, parentNum, 2, PhraseComprising)
			dep.Category = indep.Category
			claims = append(claims, dep)
			claimNum++
		}
	}

	return &ParsedClaimSet{
		PatentID: "US-TEST-001",
		Title:    "Test Patent for Scope Analysis",
		Claims:   claims,
	}
}

// newTestScopeAnalyzer creates a ScopeAnalyzer with a mock embedder for testing.
func newTestScopeAnalyzer(t *testing.T) (ScopeAnalyzer, *mockClaimEmbedder) {
	t.Helper()
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("NewScopeAnalyzer: %v", err)
	}
	return analyzer, embedder
}

// ============================================================================
// ComputeScopeBreadth tests
// ============================================================================

func TestComputeScopeBreadth_BroadClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 2, PhraseComprising)
	claim.ScopeScore = 0.65
	claim.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C", "D", "E", "F"}, IsOpen: true, Count: 6},
	}

	score, err := analyzer.ComputeScopeBreadth(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: base=0.65, phrase=+0.10, features(2<=3)=+0.05, markush(6>=5)=+0.08+0.05=+0.13
	// Total = 0.65 + 0.10 + 0.05 + 0.13 = 0.93
	if score < 0.75 {
		t.Errorf("expected BreadthScore >= 0.75 for broad claim, got %.4f", score)
	}
	level := ClassifyBreadth(score)
	if level != ScopeBroad {
		t.Errorf("expected ScopeBroad, got %s (score=%.4f)", level, score)
	}
}

func TestComputeScopeBreadth_NarrowClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 10, PhraseConsistingOf)
	claim.ScopeScore = 0.25

	score, err := analyzer.ComputeScopeBreadth(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: base=0.25, phrase=-0.15, features(10>=8)=-0.10
	// Total = 0.25 - 0.15 - 0.10 = 0.00
	if score >= 0.25 {
		t.Errorf("expected BreadthScore < 0.25 for narrow claim, got %.4f", score)
	}
	level := ClassifyBreadth(score)
	if level != ScopeVeryNarrow {
		t.Errorf("expected ScopeVeryNarrow, got %s (score=%.4f)", level, score)
	}
}

func TestComputeScopeBreadth_ModerateClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 5, PhraseComprising)
	claim.ScopeScore = 0.50

	score, err := analyzer.ComputeScopeBreadth(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected: base=0.50, phrase=+0.10, features(5 between 3-8)=interpolated ~+0.02
	// Total ≈ 0.62
	if score < 0.50 || score >= 0.75 {
		t.Errorf("expected 0.50 <= BreadthScore < 0.75 for moderate claim, got %.4f", score)
	}
	level := ClassifyBreadth(score)
	if level != ScopeModerate {
		t.Errorf("expected ScopeModerate, got %s (score=%.4f)", level, score)
	}
}

func TestComputeScopeBreadth_TransitionalPhraseImpact(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	phrases := []TransitionalPhraseType{
		PhraseComprising,
		PhraseConsistingEssentiallyOf,
		PhraseConsistingOf,
	}

	scores := make([]float64, len(phrases))
	for i, phrase := range phrases {
		claim := makeIndependentClaim(1, 5, phrase)
		claim.ScopeScore = 0.50
		score, err := analyzer.ComputeScopeBreadth(context.Background(), claim)
		if err != nil {
			t.Fatalf("unexpected error for phrase %s: %v", phrase, err)
		}
		scores[i] = score
	}

	// comprising > consisting essentially of > consisting of
	if scores[0] <= scores[1] {
		t.Errorf("comprising (%.4f) should be > consisting essentially of (%.4f)", scores[0], scores[1])
	}
	if scores[1] <= scores[2] {
		t.Errorf("consisting essentially of (%.4f) should be > consisting of (%.4f)", scores[1], scores[2])
	}
}

func TestComputeScopeBreadth_FeatureCountImpact(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	featureCounts := []int{2, 5, 10}
	scores := make([]float64, len(featureCounts))

	for i, fc := range featureCounts {
		claim := makeIndependentClaim(1, fc, PhraseComprising)
		claim.ScopeScore = 0.50
		score, err := analyzer.ComputeScopeBreadth(context.Background(), claim)
		if err != nil {
			t.Fatalf("unexpected error for %d features: %v", fc, err)
		}
		scores[i] = score
	}

	// Fewer features → higher score.
	if scores[0] <= scores[1] {
		t.Errorf("2 features (%.4f) should score higher than 5 features (%.4f)", scores[0], scores[1])
	}
	if scores[1] <= scores[2] {
		t.Errorf("5 features (%.4f) should score higher than 10 features (%.4f)", scores[1], scores[2])
	}
}

func TestComputeScopeBreadth_MarkushImpact(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Without Markush.
	claimNoMarkush := makeIndependentClaim(1, 4, PhraseComprising)
	claimNoMarkush.ScopeScore = 0.50

	// With Markush (6 members, open).
	claimWithMarkush := makeIndependentClaim(2, 4, PhraseComprising)
	claimWithMarkush.ScopeScore = 0.50
	claimWithMarkush.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C", "D", "E", "F"}, IsOpen: true, Count: 6},
	}

	scoreNo, err := analyzer.ComputeScopeBreadth(context.Background(), claimNoMarkush)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	scoreWith, err := analyzer.ComputeScopeBreadth(context.Background(), claimWithMarkush)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if scoreWith <= scoreNo {
		t.Errorf("claim with Markush (%.4f) should score higher than without (%.4f)", scoreWith, scoreNo)
	}
}

func TestComputeScopeBreadth_ClampToRange(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Extremely high base score + all positive adjustments.
	claimHigh := makeIndependentClaim(1, 1, PhraseComprising)
	claimHigh.ScopeScore = 0.99
	claimHigh.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C", "D", "E", "F", "G", "H"}, IsOpen: true, Count: 8},
	}
	claimHigh.NumericalRanges = []*NumericalRange{
		{Min: 0, Max: 10000, Width: 10000, Unit: "nm"},
	}

	scoreHigh, err := analyzer.ComputeScopeBreadth(context.Background(), claimHigh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scoreHigh > 1.0 {
		t.Errorf("score should be clamped to 1.0, got %.4f", scoreHigh)
	}
	if scoreHigh < 0.0 {
		t.Errorf("score should be >= 0.0, got %.4f", scoreHigh)
	}

	// Extremely low base score + all negative adjustments.
	claimLow := makeIndependentClaim(2, 15, PhraseConsistingOf)
	claimLow.ScopeScore = 0.05

	scoreLow, err := analyzer.ComputeScopeBreadth(context.Background(), claimLow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if scoreLow < 0.0 {
		t.Errorf("score should be clamped to 0.0, got %.4f", scoreLow)
	}
	if scoreLow > 1.0 {
		t.Errorf("score should be <= 1.0, got %.4f", scoreLow)
	}
}

func TestComputeScopeBreadth_NilClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)
	_, err := analyzer.ComputeScopeBreadth(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claim")
	}
}

func TestComputeScopeBreadth_ZeroScopeScore(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 4, PhraseComprising)
	claim.ScopeScore = 0 // model did not produce a score

	score, err := analyzer.ComputeScopeBreadth(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to 0.50 base.
	if score < 0.40 || score > 0.80 {
		t.Errorf("expected reasonable score with fallback base, got %.4f", score)
	}
}

// ============================================================================
// CompareScopes tests
// ============================================================================

func TestCompareScopes_Equivalent(t *testing.T) {
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("NewScopeAnalyzer: %v", err)
	}

	// Create two claims with identical features (same embeddings).
	claimA := makeIndependentClaim(1, 3, PhraseComprising)
	claimB := makeIndependentClaim(2, 3, PhraseComprising)

	// Register identical embeddings for corresponding features.
	for i := 0; i < 3; i++ {
		idA := claimA.Features[i].ID
		idB := claimB.Features[i].ID
		embedder.registerIdenticalEmbedding(idA, idB)
	}

	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if comp.Relationship != RelEquivalent {
		t.Errorf("expected Equivalent, got %s", comp.Relationship)
	}
	if comp.OverlapScore < 0.90 {
		t.Errorf("expected high overlap score for equivalent claims, got %.4f", comp.OverlapScore)
	}
	if len(comp.SharedFeatures) != 3 {
		t.Errorf("expected 3 shared features, got %d", len(comp.SharedFeatures))
	}
	if len(comp.UniqueToA) != 0 {
		t.Errorf("expected 0 unique to A, got %d", len(comp.UniqueToA))
	}
	if len(comp.UniqueToB) != 0 {
		t.Errorf("expected 0 unique to B, got %d", len(comp.UniqueToB))
	}
}

func TestCompareScopes_AContainsB(t *testing.T) {
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("NewScopeAnalyzer: %v", err)
	}

	// Claim A: 3 features (broader, fewer limitations).
	// Claim B: 5 features (narrower, includes all of A's features + 2 extra).
	claimA := makeIndependentClaim(1, 3, PhraseComprising)
	claimB := makeIndependentClaim(2, 5, PhraseComprising)

	// A's features all match B's first 3 features.
	for i := 0; i < 3; i++ {
		embedder.registerIdenticalEmbedding(claimA.Features[i].ID, claimB.Features[i].ID)
	}
	// B's features 3 and 4 are unique (orthogonal to everything in A).
	embedder.registerOrthogonalEmbeddings(claimB.Features[3].ID, "dummy-ortho-1")
	embedder.registerOrthogonalEmbeddings(claimB.Features[4].ID, "dummy-ortho-2")

	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A's features are all matched → A contains B (A is broader).
	if comp.Relationship != RelAContainsB {
		t.Errorf("expected AContainsB, got %s", comp.Relationship)
	}
	if comp.OverlapScore <= 0 {
		t.Errorf("expected positive overlap score, got %.4f", comp.OverlapScore)
	}
	if comp.OverlapScore >= 1.0 {
		t.Errorf("expected overlap < 1.0 (B has unique features), got %.4f", comp.OverlapScore)
	}
	if len(comp.UniqueToB) == 0 {
		t.Error("expected some features unique to B")
	}
}

func TestCompareScopes_Overlapping(t *testing.T) {
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("NewScopeAnalyzer: %v", err)
	}

	// Claim A: 4 features, Claim B: 4 features.
	// 2 features match, 2 are unique to each.
	claimA := makeIndependentClaim(1, 4, PhraseComprising)
	claimB := makeIndependentClaim(2, 4, PhraseComprising)

	// First 2 features match.
	embedder.registerIdenticalEmbedding(claimA.Features[0].ID, claimB.Features[0].ID)
	embedder.registerIdenticalEmbedding(claimA.Features[1].ID, claimB.Features[1].ID)
	// Last 2 features are orthogonal.
	embedder.registerOrthogonalEmbeddings(claimA.Features[2].ID, claimB.Features[2].ID)
	embedder.registerOrthogonalEmbeddings(claimA.Features[3].ID, claimB.Features[3].ID)

	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if comp.Relationship != RelOverlapping {
		t.Errorf("expected Overlapping, got %s", comp.Relationship)
	}
	if comp.OverlapScore <= 0 || comp.OverlapScore >= 1.0 {
		t.Errorf("expected 0 < overlap < 1, got %.4f", comp.OverlapScore)
	}
	if len(comp.SharedFeatures) != 2 {
		t.Errorf("expected 2 shared features, got %d", len(comp.SharedFeatures))
	}
	if len(comp.UniqueToA) != 2 {
		t.Errorf("expected 2 unique to A, got %d", len(comp.UniqueToA))
	}
	if len(comp.UniqueToB) != 2 {
		t.Errorf("expected 2 unique to B, got %d", len(comp.UniqueToB))
	}
}

func TestCompareScopes_Disjoint(t *testing.T) {
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("NewScopeAnalyzer: %v", err)
	}

	// Two claims with completely different features.
	claimA := makeIndependentClaim(1, 3, PhraseComprising)
	claimB := makeIndependentClaim(2, 3, PhraseComprising)

	// All features are orthogonal.
	for i := 0; i < 3; i++ {
		embedder.registerOrthogonalEmbeddings(claimA.Features[i].ID, claimB.Features[i].ID)
	}

	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if comp.Relationship != RelDisjoint {
		t.Errorf("expected Disjoint, got %s", comp.Relationship)
	}
	if comp.OverlapScore != 0 {
		t.Errorf("expected overlap score 0 for disjoint claims, got %.4f", comp.OverlapScore)
	}
	if len(comp.SharedFeatures) != 0 {
		t.Errorf("expected 0 shared features, got %d", len(comp.SharedFeatures))
	}
}

func TestCompareScopes_SharedAndUniqueFeatures(t *testing.T) {
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("NewScopeAnalyzer: %v", err)
	}

	claimA := makeIndependentClaim(1, 5, PhraseComprising)
	claimB := makeIndependentClaim(2, 4, PhraseComprising)

	// 3 features match.
	for i := 0; i < 3; i++ {
		embedder.registerIdenticalEmbedding(claimA.Features[i].ID, claimB.Features[i].ID)
	}
	// A has 2 unique, B has 1 unique.
	embedder.registerOrthogonalEmbeddings(claimA.Features[3].ID, "dummy-a3")
	embedder.registerOrthogonalEmbeddings(claimA.Features[4].ID, "dummy-a4")
	embedder.registerOrthogonalEmbeddings(claimB.Features[3].ID, "dummy-b3")

	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(comp.SharedFeatures) != 3 {
		t.Errorf("expected 3 shared features, got %d", len(comp.SharedFeatures))
	}
	if len(comp.UniqueToA) != 2 {
		t.Errorf("expected 2 unique to A, got %d", len(comp.UniqueToA))
	}
	if len(comp.UniqueToB) != 1 {
		t.Errorf("expected 1 unique to B, got %d", len(comp.UniqueToB))
	}

	// Total features accounted for.
	total := len(comp.SharedFeatures) + len(comp.UniqueToA) + len(comp.UniqueToB)
	// SharedFeatures counted once, but represents pairs. UniqueToA from A, UniqueToB from B.
	if len(comp.SharedFeatures)+len(comp.UniqueToA) != len(claimA.Features) {
		t.Errorf("shared(%d) + uniqueA(%d) should equal A's features(%d)",
			len(comp.SharedFeatures), len(comp.UniqueToA), len(claimA.Features))
	}
	if len(comp.SharedFeatures)+len(comp.UniqueToB) != len(claimB.Features) {
		t.Errorf("shared(%d) + uniqueB(%d) should equal B's features(%d)",
			len(comp.SharedFeatures), len(comp.UniqueToB), len(claimB.Features))
	}
	_ = total
}

func TestCompareScopes_NilClaims(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	_, err := analyzer.CompareScopes(context.Background(), nil, makeIndependentClaim(1, 3, PhraseComprising))
	if err == nil {
		t.Fatal("expected error for nil claimA")
	}

	_, err = analyzer.CompareScopes(context.Background(), makeIndependentClaim(1, 3, PhraseComprising), nil)
	if err == nil {
		t.Fatal("expected error for nil claimB")
	}
}

func TestCompareScopes_EmptyFeatures(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claimA := makeIndependentClaim(1, 0, PhraseComprising)
	claimB := makeIndependentClaim(2, 0, PhraseComprising)

	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.Relationship != RelEquivalent {
		t.Errorf("expected Equivalent for two empty claims, got %s", comp.Relationship)
	}
}

// ============================================================================
// AnalyzeClaimSetScope tests
// ============================================================================

func TestAnalyzeClaimSetScope_OverallCoverage(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claimSet := makeClaimSet(3, 2) // 3 independent

	// 3 independent, 2 dependent each = 9 total claims.
	result, err := analyzer.AnalyzeClaimSetScope(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalClaims != 9 {
		t.Errorf("expected 9 total claims, got %d", result.TotalClaims)
	}
	if result.IndependentCount != 3 {
		t.Errorf("expected 3 independent claims, got %d", result.IndependentCount)
	}
	if result.DependentCount != 6 {
		t.Errorf("expected 6 dependent claims, got %d", result.DependentCount)
	}

	// Overall coverage should be in a reasonable range (weighted average of independent breadths).
	if result.OverallCoverage < 0.0 || result.OverallCoverage > 1.0 {
		t.Errorf("overall coverage should be in [0, 1], got %.4f", result.OverallCoverage)
	}
	if result.OverallCoverage == 0 {
		t.Error("overall coverage should not be zero for a non-empty claim set")
	}

	// Verify claim analyses were generated.
	if len(result.ClaimAnalyses) == 0 {
		t.Error("expected non-empty claim analyses")
	}
	if len(result.ClaimAnalyses) != 9 {
		t.Errorf("expected 9 claim analyses, got %d", len(result.ClaimAnalyses))
	}

	// Verify category coverage.
	if result.CategoryCoverage == nil {
		t.Error("expected non-nil category coverage map")
	}
}

func TestAnalyzeClaimSetScope_WidestAndNarrowest(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Create a claim set with deliberately varied breadth scores.
	claims := make([]*ParsedClaim, 0)

	// Claim 1: very broad (comprising, 2 features, high base score, open Markush).
	broad := makeIndependentClaim(1, 2, PhraseComprising)
	broad.ScopeScore = 0.70
	broad.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C", "D", "E"}, IsOpen: true, Count: 5},
	}
	broad.Category = "product"
	claims = append(claims, broad)

	// Claim 2: moderate.
	moderate := makeIndependentClaim(2, 5, PhraseConsistingEssentiallyOf)
	moderate.ScopeScore = 0.50
	moderate.Category = "method"
	claims = append(claims, moderate)

	// Claim 3: very narrow (consisting of, 12 features, low base score).
	narrow := makeIndependentClaim(3, 12, PhraseConsistingOf)
	narrow.ScopeScore = 0.15
	narrow.Category = "composition"
	claims = append(claims, narrow)

	// Claim 4: dependent on claim 1.
	dep := makeDependentClaim(4, 1, 3, PhraseComprising)
	dep.Category = "product"
	claims = append(claims, dep)

	claimSet := &ParsedClaimSet{
		PatentID: "US-TEST-002",
		Title:    "Varied Breadth Test",
		Claims:   claims,
	}

	result, err := analyzer.AnalyzeClaimSetScope(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.WidestClaim == nil {
		t.Fatal("expected non-nil widest claim")
	}
	if result.NarrowestClaim == nil {
		t.Fatal("expected non-nil narrowest claim")
	}

	// The widest should be claim 1.
	if result.WidestClaim.ClaimNumber != 1 {
		t.Errorf("expected widest claim to be claim 1, got claim %d (score=%.4f)",
			result.WidestClaim.ClaimNumber, result.WidestClaim.BreadthScore)
	}

	// The narrowest should be claim 3.
	if result.NarrowestClaim.ClaimNumber != 3 {
		t.Errorf("expected narrowest claim to be claim 3, got claim %d (score=%.4f)",
			result.NarrowestClaim.ClaimNumber, result.NarrowestClaim.BreadthScore)
	}

	// Widest score > narrowest score.
	if result.WidestClaim.BreadthScore <= result.NarrowestClaim.BreadthScore {
		t.Errorf("widest (%.4f) should be > narrowest (%.4f)",
			result.WidestClaim.BreadthScore, result.NarrowestClaim.BreadthScore)
	}
}

func TestAnalyzeClaimSetScope_NilClaimSet(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	_, err := analyzer.AnalyzeClaimSetScope(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claim set")
	}
}

func TestAnalyzeClaimSetScope_EmptyClaimSet(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	_, err := analyzer.AnalyzeClaimSetScope(context.Background(), &ParsedClaimSet{
		PatentID: "US-EMPTY",
		Claims:   []*ParsedClaim{},
	})
	if err == nil {
		t.Fatal("expected error for empty claim set")
	}
}

func TestAnalyzeClaimSetScope_SingleClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claimSet := &ParsedClaimSet{
		PatentID: "US-SINGLE",
		Claims:   []*ParsedClaim{makeIndependentClaim(1, 4, PhraseComprising)},
	}

	result, err := analyzer.AnalyzeClaimSetScope(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalClaims != 1 {
		t.Errorf("expected 1 total claim, got %d", result.TotalClaims)
	}
	if result.WidestClaim == nil || result.NarrowestClaim == nil {
		t.Error("widest and narrowest should be set even for single claim")
	}
	if result.WidestClaim.ClaimNumber != result.NarrowestClaim.ClaimNumber {
		t.Error("widest and narrowest should be the same for single claim")
	}
}

// ============================================================================
// IdentifyScopeGaps tests
// ============================================================================

func TestIdentifyScopeGaps_NoGaps(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Create a well-rounded claim set covering all categories with dependents.
	claims := make([]*ParsedClaim, 0)
	categories := []string{"product", "method", "composition", "use"}
	claimNum := 1

	for _, cat := range categories {
		indep := makeIndependentClaim(claimNum, 4, PhraseComprising)
		indep.Category = cat
		indep.ScopeScore = 0.60 // moderate-to-broad
		claims = append(claims, indep)
		parentNum := claimNum
		claimNum++

		// Add 2 dependents per independent.
		for j := 0; j < 2; j++ {
			dep := makeDependentClaim(claimNum, parentNum, 2, PhraseComprising)
			dep.Category = cat
			claims = append(claims, dep)
			claimNum++
		}
	}

	claimSet := &ParsedClaimSet{
		PatentID: "US-COMPLETE",
		Claims:   claims,
	}

	gaps, err := analyzer.IdentifyScopeGaps(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have no category gaps (all 4 categories covered).
	for _, gap := range gaps {
		if containsAny(gap.Description, []string{"No independent product claim",
			"No independent method claim", "No independent composition claim",
			"No independent use claim"}) {
			t.Errorf("unexpected category gap: %s", gap.Description)
		}
	}

	// Should have no broken dependency gaps.
	for _, gap := range gaps {
		if containsAny(gap.Description, []string{"has no dependent claims", "does not exist"}) {
			t.Errorf("unexpected dependency gap: %s", gap.Description)
		}
	}
}

func TestIdentifyScopeGaps_MissingMethodClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Only product claims, no method claims.
	claims := []*ParsedClaim{
		func() *ParsedClaim {
			c := makeIndependentClaim(1, 4, PhraseComprising)
			c.Category = "product"
			c.ScopeScore = 0.60
			return c
		}(),
		makeDependentClaim(2, 1, 2, PhraseComprising),
	}
	claims[1].Category = "product"

	claimSet := &ParsedClaimSet{
		PatentID: "US-NO-METHOD",
		Claims:   claims,
	}

	gaps, err := analyzer.IdentifyScopeGaps(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find a critical gap for missing method claim.
	foundMethodGap := false
	for _, gap := range gaps {
		if strings.Contains(gap.Description, "method") && gap.Severity == GapCritical {
			foundMethodGap = true
			break
		}
	}
	if !foundMethodGap {
		t.Error("expected a Critical gap for missing method claim")
	}
}

func TestIdentifyScopeGaps_BrokenDependencyChain(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Independent claim with no dependents.
	indep := makeIndependentClaim(1, 4, PhraseComprising)
	indep.Category = "product"
	indep.ScopeScore = 0.60

	// A dependent that references a non-existent claim.
	broken := makeDependentClaim(3, 99, 2, PhraseComprising) // depends on claim 99 which doesn't exist
	broken.Category = "product"

	claimSet := &ParsedClaimSet{
		PatentID: "US-BROKEN",
		Claims:   []*ParsedClaim{indep, broken},
	}

	gaps, err := analyzer.IdentifyScopeGaps(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find a critical gap for broken reference.
	foundBrokenRef := false
	for _, gap := range gaps {
		if strings.Contains(gap.Description, "does not exist") && gap.Severity == GapCritical {
			foundBrokenRef = true
			break
		}
	}
	if !foundBrokenRef {
		t.Error("expected a Critical gap for broken dependency reference")
	}

	// Should also find a major gap for independent claim with no dependents.
	foundNoDeps := false
	for _, gap := range gaps {
		if strings.Contains(gap.Description, "has no dependent claims") && gap.Severity == GapMajor {
			foundNoDeps = true
			break
		}
	}
	if !foundNoDeps {
		t.Error("expected a Major gap for independent claim with no dependents")
	}
}

func TestIdentifyScopeGaps_Severity(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Claim set with multiple gap types.
	claims := []*ParsedClaim{
		func() *ParsedClaim {
			c := makeIndependentClaim(1, 4, PhraseComprising)
			c.Category = "product"
			c.ScopeScore = 0.60
			return c
		}(),
		// No dependents for claim 1 → Major
		// No method claim → Critical
		// Small Markush group → Minor
		func() *ParsedClaim {
			c := makeIndependentClaim(5, 4, PhraseComprising)
			c.Category = "composition"
			c.ScopeScore = 0.55
			c.MarkushGroups = []*MarkushGroup{
				{Members: []string{"X", "Y"}, Count: 2},
			}
			return c
		}(),
		makeDependentClaim(6, 5, 2, PhraseComprising),
	}
	claims[2].Category = "composition"

	claimSet := &ParsedClaimSet{
		PatentID: "US-SEVERITY",
		Claims:   claims,
	}

	gaps, err := analyzer.IdentifyScopeGaps(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(gaps) == 0 {
		t.Fatal("expected at least one gap")
	}

	// Verify gaps are sorted by severity (Critical first).
	severityOrder := map[GapSeverity]int{GapCritical: 3, GapMajor: 2, GapMinor: 1}
	for i := 1; i < len(gaps); i++ {
		if severityOrder[gaps[i].Severity] > severityOrder[gaps[i-1].Severity] {
			t.Errorf("gaps not sorted by severity: gap[%d]=%s > gap[%d]=%s",
				i, gaps[i].Severity, i-1, gaps[i-1].Severity)
		}
	}

	// Verify we have at least one of each expected severity.
	hasCritical := false
	hasMajor := false
	hasMinor := false
	for _, gap := range gaps {
		switch gap.Severity {
		case GapCritical:
			hasCritical = true
		case GapMajor:
			hasMajor = true
		case GapMinor:
			hasMinor = true
		}
	}
	if !hasCritical {
		t.Error("expected at least one Critical gap (missing method claim)")
	}
	if !hasMajor {
		t.Error("expected at least one Major gap (no dependents for claim 1)")
	}
	if !hasMinor {
		t.Error("expected at least one Minor gap (small Markush group)")
	}
}

func TestIdentifyScopeGaps_NilClaimSet(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)
	_, err := analyzer.IdentifyScopeGaps(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claim set")
	}
}

// ============================================================================
// GenerateScopeVisualization tests
// ============================================================================

func TestGenerateScopeVisualization_Nodes(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claimSet := makeClaimSet(2, 3) // 2 independent, 3 dependent each = 8 claims

	viz, err := analyzer.GenerateScopeVisualization(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(viz.Nodes) != len(claimSet.Claims) {
		t.Errorf("expected %d nodes, got %d", len(claimSet.Claims), len(viz.Nodes))
	}

	// Verify each node has valid data.
	for _, node := range viz.Nodes {
		if node.ClaimNumber <= 0 {
			t.Errorf("node has invalid claim number: %d", node.ClaimNumber)
		}
		if node.Label == "" {
			t.Errorf("node %d has empty label", node.ClaimNumber)
		}
		if node.BreadthScore < 0 || node.BreadthScore > 1 {
			t.Errorf("node %d has invalid breadth score: %.4f", node.ClaimNumber, node.BreadthScore)
		}
		if node.Size <= 0 {
			t.Errorf("node %d has invalid size: %.4f", node.ClaimNumber, node.Size)
		}
		if node.BreadthLevel == "" {
			t.Errorf("node %d has empty breadth level", node.ClaimNumber)
		}
	}
}

func TestGenerateScopeVisualization_Layers(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claimSet := makeClaimSet(2, 2) // 2 independent, 2 dependent each = 6 claims

	viz, err := analyzer.GenerateScopeVisualization(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(viz.Layers) == 0 {
		t.Fatal("expected at least one layer")
	}

	// Layer 0 should contain independent claims.
	layer0 := viz.Layers[0]
	if len(layer0) == 0 {
		t.Fatal("layer 0 should not be empty")
	}

	// Verify independent claims are in layer 0.
	independentNums := make(map[int]bool)
	for _, c := range claimSet.Claims {
		if c.ClaimType == ClaimTypeIndependent {
			independentNums[c.ClaimNumber] = true
		}
	}
	for _, cn := range layer0 {
		if !independentNums[cn] {
			t.Errorf("claim %d in layer 0 is not an independent claim", cn)
		}
	}

	// Verify all claims appear in exactly one layer.
	seen := make(map[int]bool)
	for layerIdx, layer := range viz.Layers {
		for _, cn := range layer {
			if seen[cn] {
				t.Errorf("claim %d appears in multiple layers (found again in layer %d)", cn, layerIdx)
			}
			seen[cn] = true
		}
	}
	for _, c := range claimSet.Claims {
		if !seen[c.ClaimNumber] {
			t.Errorf("claim %d not found in any layer", c.ClaimNumber)
		}
	}

	// Dependent claims should be in layer >= 1.
	if len(viz.Layers) >= 2 {
		for _, cn := range viz.Layers[1] {
			if independentNums[cn] {
				t.Errorf("independent claim %d should not be in layer 1", cn)
			}
		}
	}
}

func TestGenerateScopeVisualization_Heatmap(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	n := 5
	claims := make([]*ParsedClaim, n)
	for i := 0; i < n; i++ {
		claims[i] = makeIndependentClaim(i+1, 3, PhraseComprising)
		claims[i].Category = "product"
	}
	claimSet := &ParsedClaimSet{
		PatentID: "US-HEATMAP",
		Claims:   claims,
	}

	viz, err := analyzer.GenerateScopeVisualization(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Heatmap should be N×N.
	if len(viz.HeatmapData) != n {
		t.Fatalf("expected heatmap with %d rows, got %d", n, len(viz.HeatmapData))
	}
	for i, row := range viz.HeatmapData {
		if len(row) != n {
			t.Errorf("heatmap row %d has %d columns, expected %d", i, len(row), n)
		}
	}

	// Diagonal should be 1.0 (self-similarity).
	for i := 0; i < n; i++ {
		if math.Abs(viz.HeatmapData[i][i]-1.0) > 1e-9 {
			t.Errorf("heatmap diagonal [%d][%d] should be 1.0, got %.4f", i, i, viz.HeatmapData[i][i])
		}
	}

	// Heatmap should be symmetric: M[i][j] == M[j][i].
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if math.Abs(viz.HeatmapData[i][j]-viz.HeatmapData[j][i]) > 1e-9 {
				t.Errorf("heatmap not symmetric: [%d][%d]=%.4f != [%d][%d]=%.4f",
					i, j, viz.HeatmapData[i][j], j, i, viz.HeatmapData[j][i])
			}
		}
	}

	// All values should be in [0, 1].
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			v := viz.HeatmapData[i][j]
			if v < 0 || v > 1.0+1e-9 {
				t.Errorf("heatmap value [%d][%d]=%.4f out of [0, 1] range", i, j, v)
			}
		}
	}
}

func TestGenerateScopeVisualization_Edges(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claimSet := makeClaimSet(1, 2) // 1 independent + 2 dependents = 3 claims

	viz, err := analyzer.GenerateScopeVisualization(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least dependency edges.
	hasDependencyEdge := false
	for _, edge := range viz.Edges {
		if edge.EdgeType == EdgeDependency {
			hasDependencyEdge = true
			break
		}
	}
	if !hasDependencyEdge {
		t.Error("expected at least one dependency edge")
	}

	// Verify edge types are valid.
	validEdgeTypes := map[ScopeEdgeType]bool{
		EdgeDependency:  true,
		EdgeContainment: true,
		EdgeOverlap:     true,
	}
	for _, edge := range viz.Edges {
		if !validEdgeTypes[edge.EdgeType] {
			t.Errorf("invalid edge type: %s", edge.EdgeType)
		}
		if edge.Weight < 0 || edge.Weight > 1.0+1e-9 {
			t.Errorf("edge weight %.4f out of range", edge.Weight)
		}
	}
}

func TestGenerateScopeVisualization_NilClaimSet(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)
	_, err := analyzer.GenerateScopeVisualization(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claim set")
	}
}

// ============================================================================
// AnalyzeScope tests
// ============================================================================

func TestAnalyzeScope_BasicClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 4, PhraseComprising)
	claim.ScopeScore = 0.55

	sa, err := analyzer.AnalyzeScope(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sa.ClaimNumber != 1 {
		t.Errorf("expected claim number 1, got %d", sa.ClaimNumber)
	}
	if sa.BreadthScore < 0 || sa.BreadthScore > 1 {
		t.Errorf("breadth score out of range: %.4f", sa.BreadthScore)
	}
	if sa.BreadthLevel == "" {
		t.Error("breadth level should not be empty")
	}
	if sa.TransitionalPhraseImpact == "" {
		t.Error("transitional phrase impact should not be empty")
	}
	if sa.FeatureCount != 4 {
		t.Errorf("expected feature count 4, got %d", sa.FeatureCount)
	}
}

func TestAnalyzeScope_WithMarkush(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.ScopeScore = 0.50
	claim.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C"}, Count: 3},
		{Members: []string{"X", "Y"}, Count: 2},
	}

	sa, err := analyzer.AnalyzeScope(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 * 2 = 6 combinations.
	if sa.MarkushExpansion != 6 {
		t.Errorf("expected Markush expansion 6, got %d", sa.MarkushExpansion)
	}
}

func TestAnalyzeScope_WithNumericalRanges(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 4, PhraseComprising)
	claim.ScopeScore = 0.50
	claim.NumericalRanges = []*NumericalRange{
		{Min: 10, Max: 100, Width: 90, Unit: "nm"},
		{Min: 0.5, Max: 2.0, Width: 1.5, Unit: "mol"},
	}

	sa, err := analyzer.AnalyzeScope(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sa.NumericalRangeWidth <= 0 {
		t.Errorf("expected positive numerical range width, got %.4f", sa.NumericalRangeWidth)
	}
	if sa.NumericalRangeWidth > 1.0 {
		t.Errorf("numerical range width should be normalized to [0, 1], got %.4f", sa.NumericalRangeWidth)
	}
}

func TestAnalyzeScope_EmptyClaim(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	_, err := analyzer.AnalyzeScope(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claim")
	}
}

func TestAnalyzeScope_NilFeatures(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := &ParsedClaim{
		ClaimNumber:        1,
		ClaimType:          ClaimTypeIndependent,
		TransitionalPhrase: PhraseComprising,
		Features:           nil, // nil features
		ScopeScore:         0.50,
		Category:           "product",
	}

	sa, err := analyzer.AnalyzeScope(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sa.FeatureCount != 0 {
		t.Errorf("expected feature count 0 for nil features, got %d", sa.FeatureCount)
	}
	// Should still produce a valid breadth score.
	if sa.BreadthScore < 0 || sa.BreadthScore > 1 {
		t.Errorf("breadth score out of range: %.4f", sa.BreadthScore)
	}
}

func TestAnalyzeScope_BroadeningOpportunities(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Narrow claim should have broadening suggestions.
	claim := makeIndependentClaim(1, 8, PhraseConsistingOf)
	claim.ScopeScore = 0.30

	sa, err := analyzer.AnalyzeScope(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sa.BroadeningOpportunities) == 0 {
		t.Error("expected broadening opportunities for narrow claim")
	}

	// Should suggest replacing 'consisting of'.
	foundPhraseSuggestion := false
	for _, opp := range sa.BroadeningOpportunities {
		if strings.Contains(opp, "comprising") {
			foundPhraseSuggestion = true
			break
		}
	}
	if !foundPhraseSuggestion {
		t.Error("expected suggestion to use 'comprising'")
	}
}

func TestAnalyzeScope_NarrowingRisks(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	// Very broad claim should have narrowing risks.
	claim := makeIndependentClaim(1, 2, PhraseComprising)
	claim.ScopeScore = 0.75

	sa, err := analyzer.AnalyzeScope(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sa.NarrowingRisks) == 0 {
		t.Error("expected narrowing risks for very broad claim")
	}
}

func TestAnalyzeScope_KeyLimitations(t *testing.T) {
	analyzer, _ := newTestScopeAnalyzer(t)

	claim := makeIndependentClaim(1, 10, PhraseConsistingOf)
	claim.ScopeScore = 0.30
	claim.NumericalRanges = []*NumericalRange{
		{Min: 5.0, Max: 8.0, Width: 3.0, Unit: "mm"},
	}

	sa, err := analyzer.AnalyzeScope(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sa.KeyLimitations) == 0 {
		t.Error("expected key limitations for narrow claim with consisting of and narrow ranges")
	}

	// Should mention closed transitional phrase.
	foundPhraseLimitation := false
	for _, lim := range sa.KeyLimitations {
		if strings.Contains(lim, "consisting of") {
			foundPhraseLimitation = true
			break
		}
	}
	if !foundPhraseLimitation {
		t.Error("expected limitation mentioning 'consisting of'")
	}

	// Should mention narrow numerical range.
	foundRangeLimitation := false
	for _, lim := range sa.KeyLimitations {
		if strings.Contains(lim, "Narrow numerical range") {
			foundRangeLimitation = true
			break
		}
	}
	if !foundRangeLimitation {
		t.Error("expected limitation mentioning narrow numerical range")
	}

	// Should mention high feature count.
	foundFeatureCountLimitation := false
	for _, lim := range sa.KeyLimitations {
		if strings.Contains(lim, "High feature count") {
			foundFeatureCountLimitation = true
			break
		}
	}
	if !foundFeatureCountLimitation {
		t.Error("expected limitation mentioning high feature count")
	}
}

// ============================================================================
// ClassifyBreadth tests
// ============================================================================

func TestClassifyBreadth(t *testing.T) {
	tests := []struct {
		score    float64
		expected ScopeBreadthLevel
	}{
		{0.00, ScopeVeryNarrow},
		{0.10, ScopeVeryNarrow},
		{0.24, ScopeVeryNarrow},
		{0.25, ScopeNarrow},
		{0.35, ScopeNarrow},
		{0.49, ScopeNarrow},
		{0.50, ScopeModerate},
		{0.60, ScopeModerate},
		{0.74, ScopeModerate},
		{0.75, ScopeBroad},
		{0.85, ScopeBroad},
		{1.00, ScopeBroad},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%.2f", tt.score), func(t *testing.T) {
			got := ClassifyBreadth(tt.score)
			if got != tt.expected {
				t.Errorf("ClassifyBreadth(%.2f) = %s, want %s", tt.score, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// cosineSimilarity tests
// ============================================================================

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0, 3.0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("expected cosine similarity 1.0 for identical vectors, got %.6f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{0.0, 1.0, 0.0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim) > 1e-6 {
		t.Errorf("expected cosine similarity 0.0 for orthogonal vectors, got %.6f", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{-1.0, -2.0, -3.0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim-(-1.0)) > 1e-6 {
		t.Errorf("expected cosine similarity -1.0 for opposite vectors, got %.6f", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float32{0.0, 0.0, 0.0}
	b := []float32{1.0, 2.0, 3.0}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("expected 0 for zero vector, got %.6f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float32{1.0, 2.0}
	b := []float32{1.0, 2.0, 3.0}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("expected 0 for different length vectors, got %.6f", sim)
	}
}

func TestCosineSimilarity_Empty(t *testing.T) {
	sim := cosineSimilarity(nil, nil)
	if sim != 0 {
		t.Errorf("expected 0 for nil vectors, got %.6f", sim)
	}
}

// ============================================================================
// greedyBipartiteMatch tests
// ============================================================================

func TestGreedyBipartiteMatch_PerfectMatch(t *testing.T) {
	// 3x3 identity-like matrix (diagonal = 1.0, off-diagonal = 0.0).
	matrix := [][]float64{
		{1.0, 0.0, 0.0},
		{0.0, 1.0, 0.0},
		{0.0, 0.0, 1.0},
	}

	matchesA, matchesB := greedyBipartiteMatch(matrix, 0.80)

	// Each A[i] should match B[i].
	for i := 0; i < 3; i++ {
		if matchesA[i] != i {
			t.Errorf("expected A[%d] matched to B[%d], got B[%d]", i, i, matchesA[i])
		}
	}
	for j := 0; j < 3; j++ {
		if matchesB[j] != j {
			t.Errorf("expected B[%d] matched to A[%d], got A[%d]", j, j, matchesB[j])
		}
	}
}

func TestGreedyBipartiteMatch_NoMatch(t *testing.T) {
	// All similarities below threshold.
	matrix := [][]float64{
		{0.1, 0.2, 0.3},
		{0.2, 0.1, 0.4},
		{0.3, 0.4, 0.1},
	}

	matchesA, _ := greedyBipartiteMatch(matrix, 0.80)

	for i, m := range matchesA {
		if m != -1 {
			t.Errorf("expected A[%d] unmatched (-1), got %d", i, m)
		}
	}
}

func TestGreedyBipartiteMatch_PartialMatch(t *testing.T) {
	// Only some pairs above threshold.
	matrix := [][]float64{
		{0.95, 0.10, 0.10},
		{0.10, 0.50, 0.10},
		{0.10, 0.10, 0.85},
	}

	matchesA, _ := greedyBipartiteMatch(matrix, 0.80)

	// A[0] → B[0] (0.95 >= 0.80) ✓
	if matchesA[0] != 0 {
		t.Errorf("expected A[0] matched to B[0], got B[%d]", matchesA[0])
	}
	// A[1] → unmatched (0.50 < 0.80)
	if matchesA[1] != -1 {
		t.Errorf("expected A[1] unmatched, got B[%d]", matchesA[1])
	}
	// A[2] → B[2] (0.85 >= 0.80) ✓
	if matchesA[2] != 2 {
		t.Errorf("expected A[2] matched to B[2], got B[%d]", matchesA[2])
	}
}

func TestGreedyBipartiteMatch_AsymmetricMatrix(t *testing.T) {
	// 2x4 matrix: more B features than A features.
	matrix := [][]float64{
		{0.90, 0.10, 0.85, 0.10},
		{0.10, 0.95, 0.10, 0.10},
	}

	matchesA, matchesB := greedyBipartiteMatch(matrix, 0.80)

	// A[0] should match B[0] or B[2] (both above threshold, greedy picks highest first).
	// A[1] should match B[1].
	if matchesA[1] != 1 {
		t.Errorf("expected A[1] matched to B[1], got B[%d]", matchesA[1])
	}
	// A[0] should match B[0] (0.90 > 0.85, greedy picks 0.95 first for A[1]→B[1], then 0.90 for A[0]→B[0]).
	if matchesA[0] != 0 {
		t.Errorf("expected A[0] matched to B[0], got B[%d]", matchesA[0])
	}

	// B[3] should be unmatched.
	if matchesB[3] != -1 {
		t.Errorf("expected B[3] unmatched, got A[%d]", matchesB[3])
	}
}

func TestGreedyBipartiteMatch_EmptyMatrix(t *testing.T) {
	matchesA, matchesB := greedyBipartiteMatch(nil, 0.80)
	if matchesA != nil || matchesB != nil {
		t.Error("expected nil results for empty matrix")
	}
}

// ============================================================================
// determineRelationship tests
// ============================================================================

func TestDetermineRelationship_Equivalent(t *testing.T) {
	fa := make([]*TechnicalFeature, 3)
	fb := make([]*TechnicalFeature, 3)
	matchesA := []int{0, 1, 2}
	matchesB := []int{0, 1, 2}

	rel := determineRelationship(fa, fb, matchesA, matchesB, 0.96)
	if rel != RelEquivalent {
		t.Errorf("expected Equivalent, got %s", rel)
	}
}

func TestDetermineRelationship_AContainsB(t *testing.T) {
	fa := make([]*TechnicalFeature, 3)
	fb := make([]*TechnicalFeature, 5)
	// All of A's features matched.
	matchesA := []int{0, 1, 2}
	// Only 3 of B's 5 features matched.
	matchesB := []int{0, 1, 2, -1, -1}

	rel := determineRelationship(fa, fb, matchesA, matchesB, 0.60)
	if rel != RelAContainsB {
		t.Errorf("expected AContainsB, got %s", rel)
	}
}

func TestDetermineRelationship_BContainsA(t *testing.T) {
	fa := make([]*TechnicalFeature, 5)
	fb := make([]*TechnicalFeature, 3)
	// Only 3 of A's 5 features matched.
	matchesA := []int{0, 1, 2, -1, -1}
	// All of B's features matched.
	matchesB := []int{0, 1, 2}

	rel := determineRelationship(fa, fb, matchesA, matchesB, 0.60)
	if rel != RelBContainsA {
		t.Errorf("expected BContainsA, got %s", rel)
	}
}

func TestDetermineRelationship_Disjoint(t *testing.T) {
	fa := make([]*TechnicalFeature, 3)
	fb := make([]*TechnicalFeature, 3)
	matchesA := []int{-1, -1, -1}
	matchesB := []int{-1, -1, -1}

	rel := determineRelationship(fa, fb, matchesA, matchesB, 0.0)
	if rel != RelDisjoint {
		t.Errorf("expected Disjoint, got %s", rel)
	}
}

func TestDetermineRelationship_Overlapping(t *testing.T) {
	fa := make([]*TechnicalFeature, 4)
	fb := make([]*TechnicalFeature, 4)
	// 2 matched, 2 unmatched on each side.
	matchesA := []int{0, 1, -1, -1}
	matchesB := []int{0, 1, -1, -1}

	rel := determineRelationship(fa, fb, matchesA, matchesB, 0.50)
	if rel != RelOverlapping {
		t.Errorf("expected Overlapping, got %s", rel)
	}
}

// ============================================================================
// computeMarkushExpansion tests
// ============================================================================

func TestComputeMarkushExpansion_NoMarkush(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	expansion := computeMarkushExpansion(claim)
	if expansion != 0 {
		t.Errorf("expected 0 expansion for no Markush, got %d", expansion)
	}
}

func TestComputeMarkushExpansion_SingleGroup(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C"}, Count: 3},
	}
	expansion := computeMarkushExpansion(claim)
	if expansion != 3 {
		t.Errorf("expected expansion 3, got %d", expansion)
	}
}

func TestComputeMarkushExpansion_MultipleGroups(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C"}, Count: 3},
		{Members: []string{"X", "Y"}, Count: 2},
		{Members: []string{"P", "Q", "R", "S"}, Count: 4},
	}
	expansion := computeMarkushExpansion(claim)
	// 3 * 2 * 4 = 24
	if expansion != 24 {
		t.Errorf("expected expansion 24, got %d", expansion)
	}
}

func TestComputeMarkushExpansion_NilClaim(t *testing.T) {
	expansion := computeMarkushExpansion(nil)
	if expansion != 0 {
		t.Errorf("expected 0 for nil claim, got %d", expansion)
	}
}

func TestComputeMarkushExpansion_NilGroup(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.MarkushGroups = []*MarkushGroup{nil, nil}
	expansion := computeMarkushExpansion(claim)
	if expansion != 0 {
		t.Errorf("expected 0 for nil groups, got %d", expansion)
	}
}

func TestComputeMarkushExpansion_FallbackToMemberLen(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C"}, Count: 0}, // Count not set, fallback to len(Members)
	}
	expansion := computeMarkushExpansion(claim)
	if expansion != 3 {
		t.Errorf("expected expansion 3 (fallback to member count), got %d", expansion)
	}
}

// ============================================================================
// computeNumericalRangeWidth tests
// ============================================================================

func TestComputeNumericalRangeWidth_NoRanges(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	width := computeNumericalRangeWidth(claim)
	if width != 0 {
		t.Errorf("expected 0 for no ranges, got %.4f", width)
	}
}

func TestComputeNumericalRangeWidth_SingleRange(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.NumericalRanges = []*NumericalRange{
		{Min: 10, Max: 110, Width: 100, Unit: "nm"},
	}
	width := computeNumericalRangeWidth(claim)
	// tanh(100/100) = tanh(1) ≈ 0.7616
	expected := math.Tanh(1.0)
	if math.Abs(width-expected) > 0.01 {
		t.Errorf("expected width ≈ %.4f, got %.4f", expected, width)
	}
}

func TestComputeNumericalRangeWidth_MultipleRanges(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.NumericalRanges = []*NumericalRange{
		{Min: 0, Max: 100, Width: 100, Unit: "nm"},
		{Min: 0, Max: 200, Width: 200, Unit: "nm"},
	}
	width := computeNumericalRangeWidth(claim)
	// Average of tanh(100/100) and tanh(200/100) = (tanh(1) + tanh(2)) / 2
	expected := (math.Tanh(1.0) + math.Tanh(2.0)) / 2.0
	if math.Abs(width-expected) > 0.01 {
		t.Errorf("expected width ≈ %.4f, got %.4f", expected, width)
	}
}

func TestComputeNumericalRangeWidth_FallbackToMinMax(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.NumericalRanges = []*NumericalRange{
		{Min: 10, Max: 60, Width: 0, Unit: "nm"}, // Width not set, fallback to Max-Min=50
	}
	width := computeNumericalRangeWidth(claim)
	expected := math.Tanh(50.0 / 100.0) // tanh(0.5) ≈ 0.4621
	if math.Abs(width-expected) > 0.01 {
		t.Errorf("expected width ≈ %.4f, got %.4f", expected, width)
	}
}

func TestComputeNumericalRangeWidth_NilClaim(t *testing.T) {
	width := computeNumericalRangeWidth(nil)
	if width != 0 {
		t.Errorf("expected 0 for nil claim, got %.4f", width)
	}
}

func TestComputeNumericalRangeWidth_NormalizationBounds(t *testing.T) {
	claim := makeIndependentClaim(1, 3, PhraseComprising)
	claim.NumericalRanges = []*NumericalRange{
		{Min: 0, Max: 1000000, Width: 1000000, Unit: "nm"}, // Very large range
	}
	width := computeNumericalRangeWidth(claim)
	// tanh(10000) ≈ 1.0, so width should be close to 1.0.
	if width < 0.99 {
		t.Errorf("expected width close to 1.0 for very large range, got %.4f", width)
	}
	if width > 1.0 {
		t.Errorf("width should not exceed 1.0, got %.4f", width)
	}
}

// ============================================================================
// clamp01 tests
// ============================================================================

func TestClamp01(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{-0.5, 0.0},
		{0.0, 0.0},
		{0.5, 0.5},
		{1.0, 1.0},
		{1.5, 1.0},
		{-100.0, 0.0},
		{100.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.1f", tt.input), func(t *testing.T) {
			got := clamp01(tt.input)
			if got != tt.expected {
				t.Errorf("clamp01(%.1f) = %.1f, want %.1f", tt.input, got, tt.expected)
			}
		})
	}
}

// ============================================================================
// buildLayerLayout tests
// ============================================================================

func TestBuildLayerLayout_SimpleHierarchy(t *testing.T) {
	claimSet := &ParsedClaimSet{
		PatentID: "US-LAYERS",
		Claims: []*ParsedClaim{
			{ClaimNumber: 1, ClaimType: ClaimTypeIndependent, DependsOn: nil},
			{ClaimNumber: 2, ClaimType: ClaimTypeDependent, DependsOn: []int{1}},
			{ClaimNumber: 3, ClaimType: ClaimTypeDependent, DependsOn: []int{1}},
			{ClaimNumber: 4, ClaimType: ClaimTypeDependent, DependsOn: []int{2}},
		},
	}

	layers := buildLayerLayout(claimSet)

	if len(layers) < 3 {
		t.Fatalf("expected at least 3 layers, got %d", len(layers))
	}

	// Layer 0: claim 1 (independent).
	if !containsInt(layers[0], 1) {
		t.Error("layer 0 should contain claim 1")
	}

	// Layer 1: claims 2, 3 (depend on claim 1).
	if !containsInt(layers[1], 2) || !containsInt(layers[1], 3) {
		t.Errorf("layer 1 should contain claims 2 and 3, got %v", layers[1])
	}

	// Layer 2: claim 4 (depends on claim 2).
	if !containsInt(layers[2], 4) {
		t.Errorf("layer 2 should contain claim 4, got %v", layers[2])
	}
}

func TestBuildLayerLayout_MultipleIndependent(t *testing.T) {
	claimSet := &ParsedClaimSet{
		PatentID: "US-MULTI-INDEP",
		Claims: []*ParsedClaim{
			{ClaimNumber: 1, ClaimType: ClaimTypeIndependent, DependsOn: nil},
			{ClaimNumber: 5, ClaimType: ClaimTypeIndependent, DependsOn: nil},
			{ClaimNumber: 2, ClaimType: ClaimTypeDependent, DependsOn: []int{1}},
			{ClaimNumber: 6, ClaimType: ClaimTypeDependent, DependsOn: []int{5}},
		},
	}

	layers := buildLayerLayout(claimSet)

	// Layer 0 should contain both independent claims.
	if !containsInt(layers[0], 1) || !containsInt(layers[0], 5) {
		t.Errorf("layer 0 should contain claims 1 and 5, got %v", layers[0])
	}

	// Layer 1 should contain both dependent claims.
	if len(layers) < 2 {
		t.Fatal("expected at least 2 layers")
	}
	if !containsInt(layers[1], 2) || !containsInt(layers[1], 6) {
		t.Errorf("layer 1 should contain claims 2 and 6, got %v", layers[1])
	}
}

func TestBuildLayerLayout_NilClaimSet(t *testing.T) {
	layers := buildLayerLayout(nil)
	if layers != nil {
		t.Errorf("expected nil for nil claim set, got %v", layers)
	}
}

func TestBuildLayerLayout_SortedWithinLayers(t *testing.T) {
	claimSet := &ParsedClaimSet{
		PatentID: "US-SORTED",
		Claims: []*ParsedClaim{
			{ClaimNumber: 10, ClaimType: ClaimTypeIndependent, DependsOn: nil},
			{ClaimNumber: 3, ClaimType: ClaimTypeIndependent, DependsOn: nil},
			{ClaimNumber: 7, ClaimType: ClaimTypeIndependent, DependsOn: nil},
		},
	}

	layers := buildLayerLayout(claimSet)

	if len(layers) == 0 {
		t.Fatal("expected at least one layer")
	}

	// Layer 0 should be sorted.
	for i := 1; i < len(layers[0]); i++ {
		if layers[0][i] < layers[0][i-1] {
			t.Errorf("layer 0 not sorted: %v", layers[0])
			break
		}
	}
}

// ============================================================================
// NewScopeAnalyzer tests
// ============================================================================

func TestNewScopeAnalyzer_NilEmbedder(t *testing.T) {
	_, err := NewScopeAnalyzer(nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil embedder")
	}
}

func TestNewScopeAnalyzer_ValidCreation(t *testing.T) {
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if analyzer == nil {
		t.Fatal("expected non-nil analyzer")
	}
}

// ============================================================================
// gapSeverityRank tests
// ============================================================================

func TestGapSeverityRank(t *testing.T) {
	if gapSeverityRank(GapCritical) <= gapSeverityRank(GapMajor) {
		t.Error("Critical should rank higher than Major")
	}
	if gapSeverityRank(GapMajor) <= gapSeverityRank(GapMinor) {
		t.Error("Major should rank higher than Minor")
	}
	if gapSeverityRank(GapMinor) <= gapSeverityRank("UNKNOWN") {
		t.Error("Minor should rank higher than unknown")
	}
}

// ============================================================================
// Integration-style tests
// ============================================================================

func TestIntegration_FullClaimSetAnalysis(t *testing.T) {
	embedder := newMockClaimEmbedder(128)
	analyzer, err := NewScopeAnalyzer(embedder, nil, nil)
	if err != nil {
		t.Fatalf("NewScopeAnalyzer: %v", err)
	}

	// Build a realistic claim set.
	claims := make([]*ParsedClaim, 0)

	// Independent claim 1: broad product claim.
	c1 := makeIndependentClaim(1, 3, PhraseComprising)
	c1.ScopeScore = 0.65
	c1.Category = "product"
	c1.MarkushGroups = []*MarkushGroup{
		{Members: []string{"A", "B", "C", "D", "E"}, IsOpen: true, Count: 5},
	}
	claims = append(claims, c1)

	// Dependent claim 2: narrows claim 1.
	c2 := makeDependentClaim(2, 1, 3, PhraseComprising)
	c2.Category = "product"
	claims = append(claims, c2)

	// Dependent claim 3: further narrows claim 2.
	c3 := makeDependentClaim(3, 2, 2, PhraseComprising)
	c3.Category = "product"
	claims = append(claims, c3)

	// Independent claim 4: method claim.
	c4 := makeIndependentClaim(4, 5, PhraseComprising)
	c4.ScopeScore = 0.50
	c4.Category = "method"
	claims = append(claims, c4)

	// Dependent claim 5: narrows claim 4.
	c5 := makeDependentClaim(5, 4, 2, PhraseComprising)
	c5.Category = "method"
	claims = append(claims, c5)

	claimSet := &ParsedClaimSet{
		PatentID: "US-INTEGRATION-001",
		Title:    "Integration Test Patent",
		Claims:   claims,
	}

	// Run full analysis.
	result, err := analyzer.AnalyzeClaimSetScope(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("AnalyzeClaimSetScope: %v", err)
	}

	// Verify basic structure.
	if result.PatentID != "US-INTEGRATION-001" {
		t.Errorf("expected patent ID US-INTEGRATION-001, got %s", result.PatentID)
	}
	if result.TotalClaims != 5 {
		t.Errorf("expected 5 total claims, got %d", result.TotalClaims)
	}
	if result.IndependentCount != 2 {
		t.Errorf("expected 2 independent claims, got %d", result.IndependentCount)
	}
	if result.DependentCount != 3 {
		t.Errorf("expected 3 dependent claims, got %d", result.DependentCount)
	}

	// Overall coverage should be > 0.
	if result.OverallCoverage <= 0 {
		t.Errorf("expected overall coverage > 0, got %.4f", result.OverallCoverage)
	}

	// Widest claim should likely be claim 1 (broad product claim).
	if result.WidestClaim == nil {
		t.Fatal("expected non-nil widest claim")
	}
	if result.WidestClaim.ClaimNumber != 1 {
		t.Errorf("expected widest claim 1, got %d (score=%.4f)", result.WidestClaim.ClaimNumber, result.WidestClaim.BreadthScore)
	}

	// Narrowest claim should likely be claim 3 (deep dependent).
	if result.NarrowestClaim == nil {
		t.Fatal("expected non-nil narrowest claim")
	}
	if result.NarrowestClaim.ClaimNumber != 3 {
		t.Errorf("expected narrowest claim 3, got %d (score=%.4f)", result.NarrowestClaim.ClaimNumber, result.NarrowestClaim.BreadthScore)
	}

	// Identify gaps should not report missing method claim (we have one),
	// and should not report broken dependencies (all are consistent).
	gaps, err := analyzer.IdentifyScopeGaps(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("IdentifyScopeGaps: %v", err)
	}
	for _, g := range gaps {
		if strings.Contains(strings.ToLower(g.Description), "does not exist") {
			t.Errorf("unexpected broken dependency gap: %s", g.Description)
		}
		if strings.Contains(strings.ToLower(g.Description), "no independent method claim") {
			t.Errorf("unexpected missing method gap: %s", g.Description)
		}
	}

	// Generate visualization should succeed and have correct counts.
	viz, err := analyzer.GenerateScopeVisualization(context.Background(), claimSet)
	if err != nil {
		t.Fatalf("GenerateScopeVisualization: %v", err)
	}
	if viz == nil {
		t.Fatal("expected non-nil visualization")
	}
	if len(viz.Nodes) != 5 {
		t.Errorf("expected 5 nodes, got %d", len(viz.Nodes))
	}
	if len(viz.HeatmapData) != 5 {
		t.Errorf("expected 5x5 heatmap, got %dx?", len(viz.HeatmapData))
	}
}

// ============================================================================
// Local test utilities
// ============================================================================

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
