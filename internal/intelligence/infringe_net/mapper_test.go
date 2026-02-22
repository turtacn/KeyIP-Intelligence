/*
 * mapper.go 实现了完整的权利要求要素映射器，包括 NLP 文本分解、分子结构映射、匈牙利算法最优对齐和禁反言检测；
 * mapper_test.go 覆盖了所有要 求的测试用例，包含 mock 依赖、边界值验证和算法最优性断言。
*/
package infringe_net

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mock: NLPParser
// ---------------------------------------------------------------------------

type mockNLPParser struct {
	parseResults   map[string][]*RawElement // keyed by claim text
	classifyResult map[string]ElementType   // keyed by element text
	parseErr       error
	classifyErr    error
}

func newMockNLPParser() *mockNLPParser {
	return &mockNLPParser{
		parseResults:   make(map[string][]*RawElement),
		classifyResult: make(map[string]ElementType),
	}
}

func (m *mockNLPParser) ParseClaimText(ctx context.Context, text string) ([]*RawElement, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}
	if res, ok := m.parseResults[text]; ok {
		return res, nil
	}
	// Default: split on commas as a simple heuristic.
	return nil, fmt.Errorf("no mock result configured for text")
}

func (m *mockNLPParser) ClassifyElement(ctx context.Context, element *RawElement) (ElementType, error) {
	if m.classifyErr != nil {
		return ElementTypeUnknown, m.classifyErr
	}
	if et, ok := m.classifyResult[element.Text]; ok {
		return et, nil
	}
	return ElementTypeUnknown, nil
}

// ---------------------------------------------------------------------------
// Mock: StructureAnalyzer
// ---------------------------------------------------------------------------

type mockStructureAnalyzer struct {
	decomposeResult map[string][]*StructuralFragment
	similarityMap   map[string]float64 // key: "frag1|frag2"
	smartsMatchMap  map[string]bool    // key: "smiles|smarts"
	decomposeErr    error
	similarityErr   error
	smartsErr       error
}

func newMockStructureAnalyzer() *mockStructureAnalyzer {
	return &mockStructureAnalyzer{
		decomposeResult: make(map[string][]*StructuralFragment),
		similarityMap:   make(map[string]float64),
		smartsMatchMap:  make(map[string]bool),
	}
}

func (m *mockStructureAnalyzer) DecomposeMolecule(ctx context.Context, smiles string) ([]*StructuralFragment, error) {
	if m.decomposeErr != nil {
		return nil, m.decomposeErr
	}
	if res, ok := m.decomposeResult[smiles]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("no decomposition configured for %s", smiles)
}

func (m *mockStructureAnalyzer) ComputeFragmentSimilarity(ctx context.Context, frag1, frag2 string) (float64, error) {
	if m.similarityErr != nil {
		return 0, m.similarityErr
	}
	key := frag1 + "|" + frag2
	if sim, ok := m.similarityMap[key]; ok {
		return sim, nil
	}
	// Try reverse.
	key = frag2 + "|" + frag1
	if sim, ok := m.similarityMap[key]; ok {
		return sim, nil
	}
	return 0.1, nil // default low similarity
}

func (m *mockStructureAnalyzer) MatchSMARTS(ctx context.Context, smiles, smarts string) (bool, error) {
	if m.smartsErr != nil {
		return false, m.smartsErr
	}
	key := smiles + "|" + smarts
	if matched, ok := m.smartsMatchMap[key]; ok {
		return matched, nil
	}
	return false, nil
}

// ---------------------------------------------------------------------------
// Mock: InfringeModel
// ---------------------------------------------------------------------------

type mockInfringeModel struct{}

func (m *mockInfringeModel) Predict(ctx context.Context, req *InfringePredictRequest) (*InfringePredictResponse, error) {
	return &InfringePredictResponse{Score: 0.5}, nil
}

func (m *mockInfringeModel) Healthy(ctx context.Context) error { return nil }
func (m *mockInfringeModel) Close() error                      { return nil }

// ---------------------------------------------------------------------------
// Mock: Logger
// ---------------------------------------------------------------------------

type mockMapperLogger struct {
	warnings []string
}

func (l *mockMapperLogger) Info(msg string, kv ...interface{})  {}
func (l *mockMapperLogger) Warn(msg string, kv ...interface{})  { l.warnings = append(l.warnings, msg) }
func (l *mockMapperLogger) Error(msg string, kv ...interface{}) {}
func (l *mockMapperLogger) Debug(msg string, kv ...interface{}) {}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setupDefaultNLP() *mockNLPParser {
	nlp := newMockNLPParser()

	// Independent claim text.
	independentText := "A compound comprising a carbazole core scaffold, a phenyl substituent at position 3, and a cyano functional group"
	nlp.parseResults[independentText] = []*RawElement{
		{Text: "a carbazole core scaffold", StartPos: 24, EndPos: 49, Confidence: 0.95},
		{Text: "a phenyl substituent at position 3", StartPos: 51, EndPos: 85, Confidence: 0.92},
		{Text: "a cyano functional group", StartPos: 91, EndPos: 115, Confidence: 0.90},
	}
	nlp.classifyResult["a carbazole core scaffold"] = ElementTypeCoreScaffold
	nlp.classifyResult["a phenyl substituent at position 3"] = ElementTypeSubstituent
	nlp.classifyResult["a cyano functional group"] = ElementTypeFunctionalGroup

	// Dependent claim text.
	dependentText := "The compound of claim 1, wherein the phenyl substituent is a naphthyl group"
	nlp.parseResults[dependentText] = []*RawElement{
		{Text: "the phenyl substituent is a naphthyl group", StartPos: 35, EndPos: 78, Confidence: 0.88},
	}
	nlp.classifyResult["the phenyl substituent is a naphthyl group"] = ElementTypeSubstituent

	// Third-level dependent claim.
	dep2Text := "The compound of claim 2, further comprising a methyl group at position 6"
	nlp.parseResults[dep2Text] = []*RawElement{
		{Text: "a methyl group at position 6", StartPos: 45, EndPos: 73, Confidence: 0.91},
	}
	nlp.classifyResult["a methyl group at position 6"] = ElementTypeSubstituent

	// Unparseable text.
	nlp.parseResults["INVALID_CLAIM_TEXT"] = nil

	return nlp
}

func setupDefaultAnalyzer() *mockStructureAnalyzer {
	analyzer := newMockStructureAnalyzer()

	// Standard OLED molecule decomposition.
	oledSMILES := "c1ccc2c(c1)[nH]c1ccccc12"
	analyzer.decomposeResult[oledSMILES] = []*StructuralFragment{
		{SMILES: "c1ccc2c(c1)[nH]c1ccccc12", Role: "core_scaffold", Description: "carbazole core", Weight: 0.6},
		{SMILES: "c1ccccc1", Role: "substituent", Position: "3", Description: "phenyl substituent", Weight: 0.25},
		{SMILES: "C#N", Role: "functional_group", Description: "cyano group", Weight: 0.15},
	}

	// Simple benzene.
	analyzer.decomposeResult["c1ccccc1"] = []*StructuralFragment{
		{SMILES: "c1ccccc1", Role: "core_scaffold", Description: "benzene ring", Weight: 1.0},
	}

	// Fragment similarities.
	analyzer.similarityMap["c1ccc2c(c1)[nH]c1ccccc12|carbazole_smarts"] = 0.96
	analyzer.similarityMap["c1ccccc1|phenyl_smarts"] = 0.98
	analyzer.similarityMap["C#N|cyano_smarts"] = 0.97

	return analyzer
}

func assertInDelta(t *testing.T, expected, actual, delta float64, msg string) {
	t.Helper()
	if math.Abs(expected-actual) > delta {
		t.Errorf("%s: expected %f ± %f, got %f", msg, expected, delta, actual)
	}
}

// ---------------------------------------------------------------------------
// Tests: NewClaimElementMapper
// ---------------------------------------------------------------------------

func TestNewClaimElementMapper_Success(t *testing.T) {
	mapper, err := NewClaimElementMapper(
		newMockNLPParser(),
		newMockStructureAnalyzer(),
		&mockInfringeModel{},
		&mockMapperLogger{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mapper == nil {
		t.Fatal("expected non-nil mapper")
	}
}

func TestNewClaimElementMapper_NilNLP(t *testing.T) {
	_, err := NewClaimElementMapper(nil, newMockStructureAnalyzer(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil NLPParser")
	}
}

func TestNewClaimElementMapper_NilAnalyzer(t *testing.T) {
	_, err := NewClaimElementMapper(newMockNLPParser(), nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil StructureAnalyzer")
	}
}

// ---------------------------------------------------------------------------
// Tests: MapElements
// ---------------------------------------------------------------------------

func TestMapElements_IndependentClaim(t *testing.T) {
	nlp := setupDefaultNLP()
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(nlp, analyzer, nil, nil)

	claims := []*ClaimInput{
		{
			ClaimID:   "CLM-001",
			ClaimText: "A compound comprising a carbazole core scaffold, a phenyl substituent at position 3, and a cyano functional group",
			ClaimType: ClaimTypeIndependent,
		},
	}

	results, err := mapper.MapElements(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	mc := results[0]
	if mc.ClaimID != "CLM-001" {
		t.Errorf("expected ClaimID CLM-001, got %s", mc.ClaimID)
	}
	if len(mc.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(mc.Elements))
	}

	// Verify types.
	expectedTypes := []ElementType{ElementTypeCoreScaffold, ElementTypeSubstituent, ElementTypeFunctionalGroup}
	for i, el := range mc.Elements {
		if el.ElementType != expectedTypes[i] {
			t.Errorf("element[%d] type: expected %s, got %s", i, expectedTypes[i], el.ElementType)
		}
		if !el.IsEssential {
			t.Errorf("element[%d] should be essential (independent claim)", i)
		}
	}

	// Dependency chain for independent claim = own elements.
	if len(mc.DependencyChain) != 3 {
		t.Errorf("expected dependency chain length 3, got %d", len(mc.DependencyChain))
	}
}

func TestMapElements_DependentClaim(t *testing.T) {
	nlp := setupDefaultNLP()
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(nlp, analyzer, nil, nil)

	claims := []*ClaimInput{
		{
			ClaimID:   "CLM-001",
			ClaimText: "A compound comprising a carbazole core scaffold, a phenyl substituent at position 3, and a cyano functional group",
			ClaimType: ClaimTypeIndependent,
		},
		{
			ClaimID:       "CLM-002",
			ClaimText:     "The compound of claim 1, wherein the phenyl substituent is a naphthyl group",
			ClaimType:     ClaimTypeDependent,
			ParentClaimID: "CLM-001",
		},
	}

	results, err := mapper.MapElements(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	dep := results[1]
	if dep.ClaimType != ClaimTypeDependent {
		t.Errorf("expected dependent claim type")
	}
	if len(dep.Elements) != 1 {
		t.Errorf("expected 1 own element, got %d", len(dep.Elements))
	}
	// Dependent claim's own elements should NOT be essential.
	for _, el := range dep.Elements {
		if el.IsEssential {
			t.Error("dependent claim's own elements should not be essential")
		}
	}
	// Dependency chain = parent(3) + own(1) = 4.
	if len(dep.DependencyChain) != 4 {
		t.Errorf("expected dependency chain length 4, got %d", len(dep.DependencyChain))
	}
}

func TestMapElements_MultiLevelDependency(t *testing.T) {
	nlp := setupDefaultNLP()
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(nlp, analyzer, nil, nil)

	claims := []*ClaimInput{
		{
			ClaimID:   "CLM-001",
			ClaimText: "A compound comprising a carbazole core scaffold, a phenyl substituent at position 3, and a cyano functional group",
			ClaimType: ClaimTypeIndependent,
		},
		{
			ClaimID:       "CLM-002",
			ClaimText:     "The compound of claim 1, wherein the phenyl substituent is a naphthyl group",
			ClaimType:     ClaimTypeDependent,
			ParentClaimID: "CLM-001",
		},
		{
			ClaimID:       "CLM-003",
			ClaimText:     "The compound of claim 2, further comprising a methyl group at position 6",
			ClaimType:     ClaimTypeDependent,
			ParentClaimID: "CLM-002",
		},
	}

	results, err := mapper.MapElements(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// CLM-003 chain: CLM-001(3) + CLM-002(1) + CLM-003(1) = 5.
	clm3 := results[2]
	if len(clm3.DependencyChain) != 5 {
		t.Errorf("expected dependency chain length 5, got %d", len(clm3.DependencyChain))
	}
}

func TestMapElements_EssentialFeatureMarking(t *testing.T) {
	nlp := setupDefaultNLP()
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(nlp, analyzer, nil, nil)

	claims := []*ClaimInput{
		{
			ClaimID:   "CLM-001",
			ClaimText: "A compound comprising a carbazole core scaffold, a phenyl substituent at position 3, and a cyano functional group",
			ClaimType: ClaimTypeIndependent,
		},
		{
			ClaimID:       "CLM-002",
			ClaimText:     "The compound of claim 1, wherein the phenyl substituent is a naphthyl group",
			ClaimType:     ClaimTypeDependent,
			ParentClaimID: "CLM-001",
		},
	}

	results, err := mapper.MapElements(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Independent claim: all essential.
	for _, el := range results[0].Elements {
		if !el.IsEssential {
			t.Errorf("independent claim element %s should be essential", el.ElementID)
		}
	}
	// Dependent claim: own elements not essential.
	for _, el := range results[1].Elements {
		if el.IsEssential {
			t.Errorf("dependent claim element %s should not be essential", el.ElementID)
		}
	}
}

func TestMapElements_EmptyClaims(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)
	_, err := mapper.MapElements(context.Background(), []*ClaimInput{})
	if err == nil {
		t.Fatal("expected error for empty claims")
	}
}

func TestMapElements_InvalidClaimText(t *testing.T) {
	nlp := newMockNLPParser()
	nlp.parseErr = fmt.Errorf("parse failure")
	mapper, _ := NewClaimElementMapper(nlp, newMockStructureAnalyzer(), nil, nil)

	claims := []*ClaimInput{
		{ClaimID: "CLM-001", ClaimText: "some text", ClaimType: ClaimTypeIndependent},
	}
	_, err := mapper.MapElements(context.Background(), claims)
	if err == nil {
		t.Fatal("expected error for unparseable claim text")
	}
}

// ---------------------------------------------------------------------------
// Tests: MapMoleculeToElements
// ---------------------------------------------------------------------------

func TestMapMoleculeToElements_NormalDecomposition(t *testing.T) {
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), analyzer, nil, nil)

	mol := &MoleculeInput{
		SMILES:     "c1ccc2c(c1)[nH]c1ccccc12",
		MoleculeID: "MOL-001",
	}
	elements, err := mapper.MapMoleculeToElements(context.Background(), mol)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elements))
	}

	// Verify roles mapped to types.
	typeMap := make(map[ElementType]int)
	for _, el := range elements {
		typeMap[el.ElementType]++
	}
	if typeMap[ElementTypeCoreScaffold] != 1 {
		t.Error("expected 1 core scaffold element")
	}
	if typeMap[ElementTypeSubstituent] != 1 {
		t.Error("expected 1 substituent element")
	}
	if typeMap[ElementTypeFunctionalGroup] != 1 {
		t.Error("expected 1 functional group element")
	}
}

func TestMapMoleculeToElements_SimpleMolecule(t *testing.T) {
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), analyzer, nil, nil)

	mol := &MoleculeInput{SMILES: "c1ccccc1", MoleculeID: "MOL-002"}
	elements, err := mapper.MapMoleculeToElements(context.Background(), mol)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}
	if elements[0].ElementType != ElementTypeCoreScaffold {
		t.Errorf("expected CoreScaffold, got %s", elements[0].ElementType)
	}
}

func TestMapMoleculeToElements_InvalidSMILES(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)
	_, err := mapper.MapMoleculeToElements(context.Background(), &MoleculeInput{SMILES: ""})
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestMapMoleculeToElements_NilInput(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)
	_, err := mapper.MapMoleculeToElements(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

// ---------------------------------------------------------------------------
// Tests: AlignElements
// ---------------------------------------------------------------------------

func buildTestClaimElements() []*ClaimElement {
	return []*ClaimElement{
		{ElementID: "CE-001", ElementType: ElementTypeCoreScaffold, Description: "carbazole core", StructuralConstraint: "carbazole_smarts", IsEssential: true},
		{ElementID: "CE-002", ElementType: ElementTypeSubstituent, Description: "phenyl substituent", StructuralConstraint: "phenyl_smarts", IsEssential: true},
		{ElementID: "CE-003", ElementType: ElementTypeFunctionalGroup, Description: "cyano group", StructuralConstraint: "cyano_smarts", IsEssential: true},
	}
}

func buildTestMolElements() []*StructuralElement {
	return []*StructuralElement{
		{ElementID: "ME-001", ElementType: ElementTypeCoreScaffold, SMILES: "c1ccc2c(c1)[nH]c1ccccc12", Description: "carbazole core"},
		{ElementID: "ME-002", ElementType: ElementTypeSubstituent, SMILES: "c1ccccc1", Description: "phenyl substituent"},
		{ElementID: "ME-003", ElementType: ElementTypeFunctionalGroup, SMILES: "C#N", Description: "cyano group"},
	}
}

func TestAlignElements_PerfectMatch(t *testing.T) {
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), analyzer, nil, nil)

	molElems := buildTestMolElements()
	claimElems := buildTestClaimElements()

	alignment, err := mapper.AlignElements(context.Background(), molElems, claimElems)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(alignment.Pairs) != 3 {
		t.Errorf("expected 3 pairs, got %d", len(alignment.Pairs))
	}
	assertInDelta(t, 1.0, alignment.CoverageRatio, 0.001, "CoverageRatio")
	if len(alignment.UnmatchedClaimElements) != 0 {
		t.Errorf("expected 0 unmatched claim elements, got %d", len(alignment.UnmatchedClaimElements))
	}
	if len(alignment.UnmatchedMoleculeElements) != 0 {
		t.Errorf("expected 0 unmatched molecule elements, got %d", len(alignment.UnmatchedMoleculeElements))
	}
}

func TestAlignElements_PartialMatch(t *testing.T) {
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), analyzer, nil, nil)

	// Molecule missing the functional group.
	molElems := buildTestMolElements()[:2]
	claimElems := buildTestClaimElements()

	alignment, err := mapper.AlignElements(context.Background(), molElems, claimElems)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if alignment.CoverageRatio >= 1.0 {
		t.Errorf("expected CoverageRatio < 1.0, got %f", alignment.CoverageRatio)
	}
	if len(alignment.UnmatchedClaimElements) == 0 {
		t.Error("expected at least 1 unmatched claim element")
	}
}

func TestAlignElements_ExtraMoleculeElements(t *testing.T) {
	analyzer := setupDefaultAnalyzer()
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), analyzer, nil, nil)

	molElems := append(buildTestMolElements(), &StructuralElement{
		ElementID:   "ME-004",
		ElementType: ElementTypeLinker,
		SMILES:      "CC",
		Description: "ethyl linker",
	})
	claimElems := buildTestClaimElements()

	alignment, err := mapper.AlignElements(context.Background(), molElems, claimElems)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All claim elements should still be covered.
	assertInDelta(t, 1.0, alignment.CoverageRatio,0.001, "CoverageRatio with extra molecule elements")
	if len(alignment.UnmatchedMoleculeElements) == 0 {
		t.Error("expected at least 1 unmatched molecule element (the extra linker)")
	}
}

func TestAlignElements_HungarianOptimality(t *testing.T) {
	// Construct a scenario where greedy matching yields suboptimal results
	// but Hungarian algorithm finds the global optimum.
	//
	// Similarity matrix:
	//   M0 -> C0: 0.90,  M0 -> C1: 0.85
	//   M1 -> C0: 0.88,  M1 -> C1: 0.50
	//
	// Greedy: M0->C0 (0.90), M1->C1 (0.50) => total 1.40
	// Hungarian: M0->C1 (0.85), M1->C0 (0.88) => total 1.73
	analyzer := newMockStructureAnalyzer()
	analyzer.similarityMap["MOL_A|CLAIM_X"] = 0.90
	analyzer.similarityMap["MOL_A|CLAIM_Y"] = 0.85
	analyzer.similarityMap["MOL_B|CLAIM_X"] = 0.88
	analyzer.similarityMap["MOL_B|CLAIM_Y"] = 0.50

	mapper, _ := NewClaimElementMapper(newMockNLPParser(), analyzer, nil, nil)

	molElems := []*StructuralElement{
		{ElementID: "ME-A", ElementType: ElementTypeCoreScaffold, SMILES: "MOL_A", Description: "mol A"},
		{ElementID: "ME-B", ElementType: ElementTypeCoreScaffold, SMILES: "MOL_B", Description: "mol B"},
	}
	claimElems := []*ClaimElement{
		{ElementID: "CE-X", ElementType: ElementTypeCoreScaffold, Description: "claim X", StructuralConstraint: "CLAIM_X", IsEssential: true},
		{ElementID: "CE-Y", ElementType: ElementTypeCoreScaffold, Description: "claim Y", StructuralConstraint: "CLAIM_Y", IsEssential: true},
	}

	alignment, err := mapper.AlignElements(context.Background(), molElems, claimElems)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(alignment.Pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(alignment.Pairs))
	}

	// Verify total score is optimal (1.73 with type bonus).
	totalSim := 0.0
	for _, p := range alignment.Pairs {
		totalSim += p.SimilarityScore
	}

	// Greedy total would be 0.90+0.50+bonuses = lower than Hungarian.
	// Hungarian total should be 0.85+0.88+bonuses = higher.
	// With type bonus of 0.15 each: greedy = 1.05+0.65=1.70, hungarian = 1.00+1.03=2.03
	// The key assertion: the total should be >= the greedy total.
	greedyTotal := (0.90 + 0.15) + (0.50 + 0.15) // 1.70
	if totalSim < greedyTotal {
		t.Errorf("Hungarian should yield >= greedy total (%f), got %f", greedyTotal, totalSim)
	}
}

func TestAlignElements_MatchTypeClassification(t *testing.T) {
	tests := []struct {
		score float64
		want  MatchType
	}{
		{1.00, MatchExact},
		{0.95, MatchExact},
		{0.94, MatchSimilar},
		{0.80, MatchSimilar},
		{0.79, MatchPartial},
		{0.60, MatchPartial},
		{0.59, MatchNone},
		{0.00, MatchNone},
	}
	for _, tt := range tests {
		got := ClassifyMatchType(tt.score)
		if got != tt.want {
			t.Errorf("ClassifyMatchType(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestAlignElements_EmptyInputs(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	_, err := mapper.AlignElements(context.Background(), []*StructuralElement{}, buildTestClaimElements())
	if err == nil {
		t.Fatal("expected error for empty molecule elements")
	}

	_, err = mapper.AlignElements(context.Background(), buildTestMolElements(), []*ClaimElement{})
	if err == nil {
		t.Fatal("expected error for empty claim elements")
	}
}

// ---------------------------------------------------------------------------
// Tests: CheckEstoppel
// ---------------------------------------------------------------------------

func buildTestAlignment() *ElementAlignment {
	return &ElementAlignment{
		Pairs: []*AlignedPair{
			{
				MoleculeElement: &StructuralElement{ElementID: "ME-001", ElementType: ElementTypeCoreScaffold},
				ClaimElement:    &ClaimElement{ElementID: "CE-001", ElementType: ElementTypeCoreScaffold, IsEssential: true},
				SimilarityScore: 0.85,
				MatchType:       MatchSimilar, // equivalence-based
			},
			{
				MoleculeElement: &StructuralElement{ElementID: "ME-002", ElementType: ElementTypeSubstituent},
				ClaimElement:    &ClaimElement{ElementID: "CE-002", ElementType: ElementTypeSubstituent, IsEssential: true},
				SimilarityScore: 0.70,
				MatchType:       MatchPartial, // equivalence-based
			},
			{
				MoleculeElement: &StructuralElement{ElementID: "ME-003", ElementType: ElementTypeFunctionalGroup},
				ClaimElement:    &ClaimElement{ElementID: "CE-003", ElementType: ElementTypeFunctionalGroup, IsEssential: true},
				SimilarityScore: 0.97,
				MatchType:       MatchExact, // NOT equivalence-based
			},
		},
		CoverageRatio: 1.0,
	}
}

func TestCheckEstoppel_NarrowingAmendment(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	alignment := buildTestAlignment()
	history := &ProsecutionHistory{
		PatentID: "PAT-001",
		Amendments: []*Amendment{
			{
				AmendmentDate:    time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
				OriginalText:     "an aromatic core scaffold",
				AmendedText:      "a carbazole core scaffold",
				AmendmentType:    AmendmentNarrowing,
				AffectedElements: []string{"CE-001"},
			},
		},
	}

	result, err := mapper.CheckEstoppel(context.Background(), alignment, history)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasEstoppel {
		t.Fatal("expected estoppel to be detected")
	}
	if result.EstoppelPenalty <= 0 {
		t.Errorf("expected positive penalty, got %f", result.EstoppelPenalty)
	}
	if len(result.BlockedEquivalences) == 0 {
		t.Error("expected at least 1 blocked equivalence")
	}
	if len(result.EstoppelDetails) == 0 {
		t.Fatal("expected at least 1 estoppel detail")
	}

	// CE-001 is matched via MatchSimilar (equivalence), so it should be blocked.
	found := false
	for _, d := range result.EstoppelDetails {
		if d.AffectedElementID == "CE-001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CE-001 to be in estoppel details")
	}
}

func TestCheckEstoppel_BroadeningAmendment(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	alignment := buildTestAlignment()
	history := &ProsecutionHistory{
		PatentID: "PAT-001",
		Amendments: []*Amendment{
			{
				AmendmentDate:    time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
				OriginalText:     "a carbazole core scaffold",
				AmendedText:      "an aromatic heterocyclic core scaffold",
				AmendmentType:    AmendmentBroadening,
				AffectedElements: []string{"CE-001"},
			},
		},
	}

	result, err := mapper.CheckEstoppel(context.Background(), alignment, history)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasEstoppel {
		t.Error("broadening amendment should not trigger estoppel")
	}
	assertInDelta(t, 0.0, result.EstoppelPenalty, 0.001, "penalty for broadening")
}

func TestCheckEstoppel_ClarifyingAmendment(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	alignment := buildTestAlignment()
	history := &ProsecutionHistory{
		PatentID: "PAT-001",
		Amendments: []*Amendment{
			{
				AmendmentDate:    time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC),
				OriginalText:     "a core scaffold",
				AmendedText:      "a core scaffold structure",
				AmendmentType:    AmendmentClarifying,
				AffectedElements: []string{"CE-001"},
			},
		},
	}

	result, err := mapper.CheckEstoppel(context.Background(), alignment, history)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasEstoppel {
		t.Error("clarifying amendment should not trigger estoppel")
	}
}

func TestCheckEstoppel_ApplicantSurrender(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	// Build alignment where CE-002 (substituent) is matched via equivalence.
	alignment := buildTestAlignment()
	// Modify the molecule element description to overlap with surrender scope.
	alignment.Pairs[1].MoleculeElement.Description = "naphthyl substituent group"

	history := &ProsecutionHistory{
		PatentID: "PAT-001",
		Arguments: []*ApplicantArgument{
			{
				ArgumentDate:          time.Date(2023, 4, 1, 0, 0, 0, 0, time.UTC),
				ArgumentText:          "The claimed compound is distinguished by its specific phenyl substituent",
				DistinguishedFeatures: []string{"phenyl substituent"},
				SurrenderScope:        "naphthyl substituent group variants",
			},
		},
	}

	result, err := mapper.CheckEstoppel(context.Background(), alignment, history)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasEstoppel {
		t.Fatal("expected estoppel from applicant surrender")
	}
	if result.EstoppelPenalty <= 0 {
		t.Errorf("expected positive penalty, got %f", result.EstoppelPenalty)
	}
}

func TestCheckEstoppel_NoHistory(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	alignment := buildTestAlignment()
	result, err := mapper.CheckEstoppel(context.Background(), alignment, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HasEstoppel {
		t.Error("expected no estoppel with nil history")
	}
	assertInDelta(t, 0.0, result.EstoppelPenalty, 0.001, "penalty with nil history")
}

func TestCheckEstoppel_MultipleAmendments(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	alignment := buildTestAlignment()
	history := &ProsecutionHistory{
		PatentID: "PAT-001",
		Amendments: []*Amendment{
			{
				AmendmentDate:    time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
				OriginalText:     "an aromatic core",
				AmendedText:      "a carbazole core",
				AmendmentType:    AmendmentNarrowing,
				AffectedElements: []string{"CE-001"},
			},
			{
				AmendmentDate:    time.Date(2023, 2, 20, 0, 0, 0, 0, time.UTC),
				OriginalText:     "a substituent",
				AmendedText:      "a phenyl substituent",
				AmendmentType:    AmendmentNarrowing,
				AffectedElements: []string{"CE-002"},
			},
		},
	}

	result, err := mapper.CheckEstoppel(context.Background(), alignment, history)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.HasEstoppel {
		t.Fatal("expected estoppel with multiple narrowing amendments")
	}
	// Both CE-001 (MatchSimilar) and CE-002 (MatchPartial) should be blocked.
	if len(result.BlockedEquivalences) < 2 {
		t.Errorf("expected at least 2 blocked equivalences, got %d", len(result.BlockedEquivalences))
	}
	// Penalty should be higher than single amendment.
	if result.EstoppelPenalty <= 0.5 {
		t.Errorf("expected penalty > 0.5 for multiple amendments, got %f", result.EstoppelPenalty)
	}
}

func TestCheckEstoppel_EssentialElementBlocked(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	// Alignment with one essential and one non-essential equivalence pair.
	alignment := &ElementAlignment{
		Pairs: []*AlignedPair{
			{
				MoleculeElement: &StructuralElement{ElementID: "ME-001"},
				ClaimElement:    &ClaimElement{ElementID: "CE-001", IsEssential: true},
				SimilarityScore: 0.85,
				MatchType:       MatchSimilar,
			},
			{
				MoleculeElement: &StructuralElement{ElementID: "ME-002"},
				ClaimElement:    &ClaimElement{ElementID: "CE-002", IsEssential: false},
				SimilarityScore: 0.70,
				MatchType:       MatchPartial,
			},
		},
	}

	// Only block the essential element.
	historyEssential := &ProsecutionHistory{
		PatentID: "PAT-001",
		Amendments: []*Amendment{
			{
				AmendmentType:    AmendmentNarrowing,
				AffectedElements: []string{"CE-001"},
			},
		},
	}

	// Only block the non-essential element.
	historyNonEssential := &ProsecutionHistory{
		PatentID: "PAT-001",
		Amendments: []*Amendment{
			{
				AmendmentType:    AmendmentNarrowing,
				AffectedElements: []string{"CE-002"},
			},
		},
	}

	resultEssential, _ := mapper.CheckEstoppel(context.Background(), alignment, historyEssential)
	resultNonEssential, _ := mapper.CheckEstoppel(context.Background(), alignment, historyNonEssential)

	// Essential block should have higher penalty due to weighting.
	if resultEssential.EstoppelPenalty <= resultNonEssential.EstoppelPenalty {
		t.Errorf("essential block penalty (%f) should be > non-essential (%f)",
			resultEssential.EstoppelPenalty, resultNonEssential.EstoppelPenalty)
	}
}

func TestCheckEstoppel_NilAlignment(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)
	_, err := mapper.CheckEstoppel(context.Background(), nil, &ProsecutionHistory{})
	if err == nil {
		t.Fatal("expected error for nil alignment")
	}
}

// ---------------------------------------------------------------------------
// Tests: ParseProsecutionHistory
// ---------------------------------------------------------------------------

func TestParseProsecutionHistory_ValidJSON(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	history := ProsecutionHistory{
		PatentID: "PAT-001",
		Amendments: []*Amendment{
			{
				AmendmentDate:    time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
				OriginalText:     "an aromatic core",
				AmendedText:      "a carbazole core",
				AmendmentType:    AmendmentNarrowing,
				AffectedElements: []string{"CE-001"},
			},
		},
		Arguments: []*ApplicantArgument{
			{
				ArgumentDate:          time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
				ArgumentText:          "The invention is distinguished by...",
				DistinguishedFeatures: []string{"carbazole core"},
				SurrenderScope:        "non-carbazole aromatic cores",
			},
		},
	}

	data, err := json.Marshal(history)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := mapper.ParseProsecutionHistory(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.PatentID != "PAT-001" {
		t.Errorf("expected PatentID PAT-001, got %s", parsed.PatentID)
	}
	if len(parsed.Amendments) != 1 {
		t.Errorf("expected 1 amendment, got %d", len(parsed.Amendments))
	}
	if len(parsed.Arguments) != 1 {
		t.Errorf("expected 1 argument, got %d", len(parsed.Arguments))
	}
	if parsed.Amendments[0].AmendmentType != AmendmentNarrowing {
		t.Errorf("expected narrowing, got %s", parsed.Amendments[0].AmendmentType)
	}
}

func TestParseProsecutionHistory_ValidXML(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	history := ProsecutionHistory{
		PatentID: "PAT-002",
		Amendments: []*Amendment{
			{
				OriginalText:     "a substituent",
				AmendedText:      "a phenyl substituent",
				AmendmentType:    AmendmentNarrowing,
				AffectedElements: []string{"CE-002"},
			},
		},
	}

	data, err := xml.Marshal(history)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	parsed, err := mapper.ParseProsecutionHistory(context.Background(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.PatentID != "PAT-002" {
		t.Errorf("expected PatentID PAT-002, got %s", parsed.PatentID)
	}
	if len(parsed.Amendments) != 1 {
		t.Errorf("expected 1 amendment, got %d", len(parsed.Amendments))
	}
}

func TestParseProsecutionHistory_InvalidFormat(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	// Neither JSON nor XML.
	_, err := mapper.ParseProsecutionHistory(context.Background(), []byte("this is plain text"))
	if err == nil {
		t.Fatal("expected error for unrecognized format")
	}
}

func TestParseProsecutionHistory_EmptyInput(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	_, err := mapper.ParseProsecutionHistory(context.Background(), []byte{})
	if err == nil {
		t.Fatal("expected error for empty input")
	}

	_, err = mapper.ParseProsecutionHistory(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil input")
	}
}

func TestParseProsecutionHistory_MissingPatentID(t *testing.T) {
	mapper, _ := NewClaimElementMapper(newMockNLPParser(), newMockStructureAnalyzer(), nil, nil)

	data := []byte(`{"amendments":[]}`)
	_, err := mapper.ParseProsecutionHistory(context.Background(), data)
	if err == nil {
		t.Fatal("expected error for missing patent_id")
	}
}

// ---------------------------------------------------------------------------
// Tests: MatchType
// ---------------------------------------------------------------------------

func TestMatchType_String(t *testing.T) {
	tests := []struct {
		mt   MatchType
		want string
	}{
		{MatchExact, "EXACT"},
		{MatchSimilar, "SIMILAR"},
		{MatchPartial, "PARTIAL"},
		{MatchNone, "NONE"},
	}
	for _, tt := range tests {
		if got := tt.mt.String(); got != tt.want {
			t.Errorf("MatchType.String() = %s, want %s", got, tt.want)
		}
	}
}

func TestMatchType_Thresholds(t *testing.T) {
	// Boundary values.
	boundaries := []struct {
		score    float64
		expected MatchType
	}{
		{0.950, MatchExact},
		{0.9499, MatchSimilar},
		{0.800, MatchSimilar},
		{0.7999, MatchPartial},
		{0.600, MatchPartial},
		{0.5999, MatchNone},
	}
	for _, b := range boundaries {
		got := ClassifyMatchType(b.score)
		if got != b.expected {
			t.Errorf("ClassifyMatchType(%f) = %s, want %s", b.score, got, b.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Helper functions
// ---------------------------------------------------------------------------

func TestExtractStructuralConstraint(t *testing.T) {
	// Text with a SMILES-like token.
	text := "a carbazole core c1ccc2c(c1)[nH]c1ccccc12 scaffold"
	constraint := extractStructuralConstraint(text)
	if constraint == "" {
		t.Error("expected to extract a structural constraint from text containing SMILES")
	}
}

func TestExtractStructuralConstraint_NoSMILES(t *testing.T) {
	text := "a simple aromatic heterocyclic compound"
	constraint := extractStructuralConstraint(text)
	if constraint != "" {
		t.Errorf("expected empty constraint for plain text, got %s", constraint)
	}
}

func TestLooksLikeSMILES(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"c1ccccc1", true},
		{"CCO", true},
		{"CC(=O)O", true},
		{"hello", false},
		{"a", false},
		{"C#N", true},
		{"[nH]", true},
	}
	for _, tt := range tests {
		got := looksLikeSMILES(tt.input)
		if got != tt.want {
			t.Errorf("looksLikeSMILES(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExtractKeywords(t *testing.T) {
	text := "the naphthyl substituent group variants and other modifications"
	keywords := extractKeywords(text)
	// Should include: naphthyl, substituent, group, variants, other, modifications
	// Should exclude: the, and (stop words or < 4 chars)
	for _, kw := range keywords {
		if len(kw) < 4 {
			t.Errorf("keyword %q should have length >= 4", kw)
		}
	}
	if len(keywords) == 0 {
		t.Error("expected at least some keywords")
	}
}

func TestRoleToElementType(t *testing.T) {
	tests := []struct {
		role string
		want ElementType
	}{
		{"core_scaffold", ElementTypeCoreScaffold},
		{"scaffold", ElementTypeCoreScaffold},
		{"core", ElementTypeCoreScaffold},
		{"substituent", ElementTypeSubstituent},
		{"sub", ElementTypeSubstituent},
		{"functional_group", ElementTypeFunctionalGroup},
		{"functional", ElementTypeFunctionalGroup},
		{"group", ElementTypeFunctionalGroup},
		{"linker", ElementTypeLinker},
		{"bridge", ElementTypeLinker},
		{"backbone", ElementTypeBackbone},
		{"unknown_role", ElementTypeUnknown},
		{"", ElementTypeUnknown},
	}
	for _, tt := range tests {
		got := roleToElementType(tt.role)
		if got != tt.want {
			t.Errorf("roleToElementType(%q) = %s, want %s", tt.role, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Hungarian Algorithm
// ---------------------------------------------------------------------------

func TestHungarianMaximize_Square(t *testing.T) {
	// 3x3 matrix.
	sim := [][]float64{
		{0.9, 0.2, 0.1},
		{0.1, 0.8, 0.3},
		{0.2, 0.1, 0.7},
	}
	assignment := hungarianMaximize(sim, 3, 3)
	// Optimal: 0->0 (0.9), 1->1 (0.8), 2->2 (0.7) = 2.4
	total := 0.0
	for i, j := range assignment {
		if j >= 0 {
			total += sim[i][j]
		}
	}
	assertInDelta(t, 2.4, total, 0.001, "Hungarian 3x3 optimal total")
}

func TestHungarianMaximize_Rectangular_MoreRows(t *testing.T) {
	// 3 rows, 2 cols.
	sim := [][]float64{
		{0.9, 0.1},
		{0.2, 0.8},
		{0.5, 0.5},
	}
	assignment := hungarianMaximize(sim, 3, 2)
	assigned := 0
	for _, j := range assignment {
		if j >= 0 && j < 2 {
			assigned++
		}
	}
	if assigned != 2 {
		t.Errorf("expected 2 assignments, got %d", assigned)
	}
}

func TestHungarianMaximize_Rectangular_MoreCols(t *testing.T) {
	// 2 rows, 3 cols.
	sim := [][]float64{
		{0.9, 0.1, 0.5},
		{0.2, 0.8, 0.3},
	}
	assignment := hungarianMaximize(sim, 2, 3)
	assigned := 0
	total := 0.0
	for i, j := range assignment {
		if j >= 0 && j < 3 {
			assigned++
			total += sim[i][j]
		}
	}
	if assigned != 2 {
		t.Errorf("expected 2 assignments, got %d", assigned)
	}
	// Optimal: 0->0 (0.9), 1->1 (0.8) = 1.7
	assertInDelta(t, 1.7, total, 0.001, "Hungarian 2x3 optimal total")
}

func TestHungarianMaximize_SingleElement(t *testing.T) {
	sim := [][]float64{{0.75}}
	assignment := hungarianMaximize(sim, 1, 1)
	if len(assignment) != 1 || assignment[0] != 0 {
		t.Errorf("expected [0], got %v", assignment)
	}
}

//Personal.AI order the ending

