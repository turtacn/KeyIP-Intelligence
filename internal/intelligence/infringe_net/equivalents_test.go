package infringe_net

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"testing"
)

// =========================================================================
// Mock: InfringeModel
// =========================================================================

type mockEquivalentsModel struct {
	funcScores map[string]float64 // key = "queryID|claimID"
	wayScores  map[string]float64
	resScores  map[string]float64

	funcCalls atomic.Int32
	wayCalls  atomic.Int32
	resCalls  atomic.Int32

	funcErr error
	wayErr  error
	resErr  error
}

func newMockModel() *mockEquivalentsModel {
	return &mockEquivalentsModel{
		funcScores: make(map[string]float64),
		wayScores:  make(map[string]float64),
		resScores:  make(map[string]float64),
	}
}

func pairKey(a, b *StructuralElement) string {
	return a.ElementID + "|" + b.ElementID
}

func (m *mockEquivalentsModel) ComputeFunctionSimilarity(_ context.Context, a, b *StructuralElement) (float64, error) {
	m.funcCalls.Add(1)
	if m.funcErr != nil {
		return 0, m.funcErr
	}
	if s, ok := m.funcScores[pairKey(a, b)]; ok {
		return s, nil
	}
	return 0.8, nil // default high
}

func (m *mockEquivalentsModel) ComputeWaySimilarity(_ context.Context, a, b *StructuralElement) (float64, error) {
	m.wayCalls.Add(1)
	if m.wayErr != nil {
		return 0, m.wayErr
	}
	if s, ok := m.wayScores[pairKey(a, b)]; ok {
		return s, nil
	}
	return 0.75, nil
}

func (m *mockEquivalentsModel) ComputeResultSimilarity(_ context.Context, a, b *StructuralElement) (float64, error) {
	m.resCalls.Add(1)
	if m.resErr != nil {
		return 0, m.resErr
	}
	if s, ok := m.resScores[pairKey(a, b)]; ok {
		return s, nil
	}
	return 0.70, nil
}

// =========================================================================
// Helpers
// =========================================================================

func inDelta(t *testing.T, expected, actual, delta float64, msg string) {
	t.Helper()
	if math.Abs(expected-actual) > delta {
		t.Errorf("%s: expected %.6f ± %.6f, got %.6f", msg, expected, delta, actual)
	}
}

func makeElement(id string, et ElementType, desc string) *StructuralElement {
	return &StructuralElement{
		ElementID:   id,
		ElementType: et,
		Description: desc,
	}
}

func makeElementWithSMILES(id string, et ElementType, desc, smiles string) *StructuralElement {
	return &StructuralElement{
		ElementID:      id,
		ElementType:    et,
		Description:    desc,
		SMILESFragment: smiles,
	}
}

// =========================================================================
// Constructor tests
// =========================================================================

func TestNewEquivalentsAnalyzer_Success(t *testing.T) {
	model := newMockModel()
	a, err := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil analyzer")
	}
}

func TestNewEquivalentsAnalyzer_NilModel(t *testing.T) {
	_, err := NewEquivalentsAnalyzer(nil, &mockEquivalentsLogger{})
	if err == nil {
		t.Fatal("expected error for nil model")
	}
}

func TestNewEquivalentsAnalyzer_NilLogger(t *testing.T) {
	model := newMockModel()
	a, err := NewEquivalentsAnalyzer(model, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil analyzer even with nil logger")
	}
}

// =========================================================================
// AnalyzeElement tests
// =========================================================================

func TestAnalyzeElement_AllPass(t *testing.T) {
	model := newMockModel()
	q := makeElement("q1", ElementTypeSubstituent, "phenyl group")
	c := makeElement("c1", ElementTypeSubstituent, "naphthyl group")
	model.funcScores[pairKey(q, c)] = 0.85
	model.wayScores[pairKey(q, c)] = 0.78
	model.resScores[pairKey(q, c)] = 0.72

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	eq, err := a.AnalyzeElement(context.Background(), q, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eq.IsEquivalent {
		t.Errorf("expected IsEquivalent=true, got false. Reasoning: %s", eq.Reasoning)
	}
	inDelta(t, 0.85, eq.FunctionScore, 0.001, "FunctionScore")
	inDelta(t, 0.78, eq.WayScore, 0.001, "WayScore")
	inDelta(t, 0.72, eq.ResultScore, 0.001, "ResultScore")

	// OverallScore = (0.85*0.4 + 0.78*0.3 + 0.72*0.3) / (0.4+0.3+0.3)
	expectedOverall := (0.85*0.4 + 0.78*0.3 + 0.72*0.3) / 1.0
	inDelta(t, expectedOverall, eq.OverallScore, 0.001, "OverallScore")

	if eq.Reasoning == "" {
		t.Error("expected non-empty reasoning")
	}
	if model.funcCalls.Load() != 1 {
		t.Errorf("expected 1 function call, got %d", model.funcCalls.Load())
	}
	if model.wayCalls.Load() != 1 {
		t.Errorf("expected 1 way call, got %d", model.wayCalls.Load())
	}
	if model.resCalls.Load() != 1 {
		t.Errorf("expected 1 result call, got %d", model.resCalls.Load())
	}
}

func TestAnalyzeElement_FunctionFail_ShortCircuit(t *testing.T) {
	model := newMockModel()
	q := makeElement("q1", ElementTypeSubstituent, "alkyl chain")
	c := makeElement("c1", ElementTypeSubstituent, "aromatic ring")
	model.funcScores[pairKey(q, c)] = 0.40 // below default 0.7

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	eq, err := a.AnalyzeElement(context.Background(), q, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eq.IsEquivalent {
		t.Error("expected IsEquivalent=false when function fails")
	}
	inDelta(t, 0.40, eq.FunctionScore, 0.001, "FunctionScore")

	// Way and Result must NOT have been called.
	if model.wayCalls.Load() != 0 {
		t.Errorf("expected 0 way calls (short-circuit), got %d", model.wayCalls.Load())
	}
	if model.resCalls.Load() != 0 {
		t.Errorf("expected 0 result calls (short-circuit), got %d", model.resCalls.Load())
	}
	if eq.WayScore != 0 {
		t.Errorf("expected WayScore=0 (not evaluated), got %f", eq.WayScore)
	}
	if eq.ResultScore != 0 {
		t.Errorf("expected ResultScore=0 (not evaluated), got %f", eq.ResultScore)
	}
}

func TestAnalyzeElement_WayFail_ShortCircuit(t *testing.T) {
	model := newMockModel()
	q := makeElement("q1", ElementTypeFunctionalGroup, "electron donor")
	c := makeElement("c1", ElementTypeFunctionalGroup, "electron donor variant")
	model.funcScores[pairKey(q, c)] = 0.90 // passes
	model.wayScores[pairKey(q, c)] = 0.30  // fails (below 0.6)

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	eq, err := a.AnalyzeElement(context.Background(), q, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eq.IsEquivalent {
		t.Error("expected IsEquivalent=false when way fails")
	}
	if model.funcCalls.Load() != 1 {
		t.Errorf("expected 1 function call, got %d", model.funcCalls.Load())
	}
	if model.wayCalls.Load() != 1 {
		t.Errorf("expected 1 way call, got %d", model.wayCalls.Load())
	}
	if model.resCalls.Load() != 0 {
		t.Errorf("expected 0 result calls (short-circuit), got %d", model.resCalls.Load())
	}
	if eq.ResultScore != 0 {
		t.Errorf("expected ResultScore=0 (not evaluated), got %f", eq.ResultScore)
	}
}

func TestAnalyzeElement_ResultFail(t *testing.T) {
	model := newMockModel()
	q := makeElement("q1", ElementTypeCoreScaffold, "carbazole core")
	c := makeElement("c1", ElementTypeCoreScaffold, "fluorene core")
	model.funcScores[pairKey(q, c)] = 0.88
	model.wayScores[pairKey(q, c)] = 0.72
	model.resScores[pairKey(q, c)] = 0.50 // fails (below 0.65)

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	eq, err := a.AnalyzeElement(context.Background(), q, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eq.IsEquivalent {
		t.Error("expected IsEquivalent=false when result fails")
	}
	if model.funcCalls.Load() != 1 {
		t.Errorf("expected 1 function call, got %d", model.funcCalls.Load())
	}
	if model.wayCalls.Load() != 1 {
		t.Errorf("expected 1 way call, got %d", model.wayCalls.Load())
	}
	if model.resCalls.Load() != 1 {
		t.Errorf("expected 1 result call, got %d", model.resCalls.Load())
	}
	inDelta(t, 0.50, eq.ResultScore, 0.001, "ResultScore")
}

func TestAnalyzeElement_BoundaryScores(t *testing.T) {
	// Scores exactly at threshold should PASS.
	model := newMockModel()
	q := makeElement("q1", ElementTypeSubstituent, "methyl")
	c := makeElement("c1", ElementTypeSubstituent, "ethyl")
	model.funcScores[pairKey(q, c)] = 0.70 // == threshold
	model.wayScores[pairKey(q, c)] = 0.60  // == threshold
	model.resScores[pairKey(q, c)] = 0.65  // == threshold

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	eq, err := a.AnalyzeElement(context.Background(), q, c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eq.IsEquivalent {
		t.Errorf("expected IsEquivalent=true at exact boundary. Reasoning: %s", eq.Reasoning)
	}
}

func TestAnalyzeElement_BelowBoundary(t *testing.T) {
	q := makeElement("q1", ElementTypeSubstituent, "methyl")
	c := makeElement("c1", ElementTypeSubstituent, "ethyl")

	// Just below each threshold.
	tests := []struct {
		name string
		f, w, r float64
		failStep string
	}{
		{"function_below", 0.699, 0.80, 0.80, "function"},
		{"way_below", 0.80, 0.599, 0.80, "way"},
		{"result_below", 0.80, 0.70, 0.649, "result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newMockModel()
			m.funcScores[pairKey(q, c)] = tt.f
			m.wayScores[pairKey(q, c)] = tt.w
			m.resScores[pairKey(q, c)] = tt.r

			a, _ := NewEquivalentsAnalyzer(m, &mockEquivalentsLogger{})
			eq, err := a.AnalyzeElement(context.Background(), q, c)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if eq.IsEquivalent {
				t.Errorf("expected IsEquivalent=false for %s", tt.name)
			}
		})
	}
}

func TestAnalyzeElement_Reasoning(t *testing.T) {
	model := newMockModel()
	q := makeElement("q1", ElementTypeSubstituent, "tert-butyl group")
	c := makeElement("c1", ElementTypeSubstituent, "isopropyl group")
	model.funcScores[pairKey(q, c)] = 0.90
	model.wayScores[pairKey(q, c)] = 0.80
	model.resScores[pairKey(q, c)] = 0.75

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	eq, _ := a.AnalyzeElement(context.Background(), q, c)

	if eq.Reasoning == "" {
		t.Fatal("expected non-empty reasoning")
	}
	// Should mention both element descriptions.
	if !containsSubstring(eq.Reasoning, "tert-butyl") {
		t.Error("reasoning should mention query element description")
	}
	if !containsSubstring(eq.Reasoning, "isopropyl") {
		t.Error("reasoning should mention claim element description")
	}
	// Should mention FWR scores.
	if !containsSubstring(eq.Reasoning, "Function") && !containsSubstring(eq.Reasoning, "FWR") {
		t.Error("reasoning should reference the FWR test")
	}
}

func TestAnalyzeElement_NilElements(t *testing.T) {
	model := newMockModel()
	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})

	_, err := a.AnalyzeElement(context.Background(), nil, makeElement("c1", ElementTypeSubstituent, "x"))
	if err == nil {
		t.Error("expected error for nil query element")
	}
	_, err = a.AnalyzeElement(context.Background(), makeElement("q1", ElementTypeSubstituent, "x"), nil)
	if err == nil {
		t.Error("expected error for nil claim element")
	}
}

// =========================================================================
// Analyze (full) tests
// =========================================================================

func TestAnalyze_AllElementsEquivalent(t *testing.T) {
	model := newMockModel()
	// All pairs return high scores by default (0.8, 0.75, 0.70).
	query := []*StructuralElement{
		makeElement("q1", ElementTypeCoreScaffold, "carbazole scaffold"),
		makeElement("q2", ElementTypeSubstituent, "phenyl group"),
		makeElement("q3", ElementTypeFunctionalGroup, "amine donor"),
	}
	claim := []*StructuralElement{
		makeElement("c1", ElementTypeCoreScaffold, "carbazole scaffold variant"),
		makeElement("c2", ElementTypeSubstituent, "phenyl group variant"),
		makeElement("c3", ElementTypeFunctionalGroup, "amine donor variant"),
	}

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: query,
		ClaimElements: claim,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EquivalentElementCount != 3 {
		t.Errorf("expected 3 equivalent elements, got %d", res.EquivalentElementCount)
	}
	if res.TotalElementCount != 3 {
		t.Errorf("expected 3 total elements, got %d", res.TotalElementCount)
	}
	// Score should be close to 1.0.
	if res.OverallEquivalenceScore < 0.95 {
		t.Errorf("expected OverallEquivalenceScore ≈ 1.0, got %.4f", res.OverallEquivalenceScore)
	}
	if len(res.NonEquivalentElements) != 0 {
		t.Errorf("expected 0 non-equivalent elements, got %d", len(res.NonEquivalentElements))
	}
}

func TestAnalyze_NoElementsEquivalent(t *testing.T) {
	model := newMockModel()
	q1 := makeElement("q1", ElementTypeCoreScaffold, "anthracene")
	c1 := makeElement("c1", ElementTypeCoreScaffold, "pyrene")
	q2 := makeElement("q2", ElementTypeSubstituent, "methyl")
	c2 := makeElement("c2", ElementTypeSubstituent, "trifluoromethyl")

	// All function scores below threshold.
	model.funcScores[pairKey(q1, c1)] = 0.30
	model.funcScores[pairKey(q2, c2)] = 0.25

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{q1, q2},
		ClaimElements: []*StructuralElement{c1, c2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EquivalentElementCount != 0 {
		t.Errorf("expected 0 equivalent elements, got %d", res.EquivalentElementCount)
	}
	inDelta(t, 0.0, res.OverallEquivalenceScore, 0.001, "OverallEquivalenceScore")
	if len(res.NonEquivalentElements) != 2 {
		t.Errorf("expected 2 non-equivalent elements, got %d", len(res.NonEquivalentElements))
	}
}

func TestAnalyze_PartialEquivalence(t *testing.T) {
	model := newMockModel()
	q1 := makeElement("q1", ElementTypeSubstituent, "phenyl")
	c1 := makeElement("c1", ElementTypeSubstituent, "naphthyl")
	q2 := makeElement("q2", ElementTypeSubstituent, "methyl")
	c2 := makeElement("c2", ElementTypeSubstituent, "trifluoromethyl")

	// q1-c1 passes all three.
	model.funcScores[pairKey(q1, c1)] = 0.85
	model.wayScores[pairKey(q1, c1)] = 0.75
	model.resScores[pairKey(q1, c1)] = 0.70

	// q2-c2 fails function.
	model.funcScores[pairKey(q2, c2)] = 0.30

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{q1, q2},
		ClaimElements: []*StructuralElement{c1, c2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EquivalentElementCount != 1 {
		t.Errorf("expected 1 equivalent element, got %d", res.EquivalentElementCount)
	}
	if res.TotalElementCount != 2 {
		t.Errorf("expected 2 total elements, got %d", res.TotalElementCount)
	}
	// Both are ElementTypeSubstituent (weight 0.8 each), 1 of 2 equivalent => 0.5.
	inDelta(t, 0.5, res.OverallEquivalenceScore, 0.01, "OverallEquivalenceScore")
}

func TestAnalyze_ScaffoldWeightDominance(t *testing.T) {
	model := newMockModel()

	// Case A: scaffold equivalent, substituent not.
	qScaff := makeElement("qs", ElementTypeCoreScaffold, "carbazole")
	cScaff := makeElement("cs", ElementTypeCoreScaffold, "carbazole variant")
	qSub := makeElement("qsub", ElementTypeSubstituent, "methyl")
	cSub := makeElement("csub", ElementTypeSubstituent, "trifluoromethyl")

	model.funcScores[pairKey(qScaff, cScaff)] = 0.90
	model.wayScores[pairKey(qScaff, cScaff)] = 0.80
	model.resScores[pairKey(qScaff, cScaff)] = 0.75
	model.funcScores[pairKey(qSub, cSub)] = 0.30 // fails

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	resA, _ := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{qScaff, qSub},
		ClaimElements: []*StructuralElement{cScaff, cSub},
	})

	// Case B: scaffold NOT equivalent, substituent equivalent.
	model2 := newMockModel()
	model2.funcScores[pairKey(qScaff, cScaff)] = 0.30 // fails
	model2.funcScores[pairKey(qSub, cSub)] = 0.90
	model2.wayScores[pairKey(qSub, cSub)] = 0.80
	model2.resScores[pairKey(qSub, cSub)] = 0.75

	a2, _ := NewEquivalentsAnalyzer(model2, &mockEquivalentsLogger{})
	resB, _ := a2.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{qScaff, qSub},
		ClaimElements: []*StructuralElement{cScaff, cSub},
	})

	// Case A (scaffold equiv) should score significantly higher than Case B.
	if resA.OverallEquivalenceScore <= resB.OverallEquivalenceScore {
		t.Errorf(
			"scaffold-equivalent case (%.4f) should score higher than scaffold-non-equivalent case (%.4f)",
			resA.OverallEquivalenceScore, resB.OverallEquivalenceScore,
		)
	}
	// Scaffold weight=2.0, substituent weight=0.8.
	// Case A: 2.0 / (2.0+0.8) = 0.714
	// Case B: 0.8 / (2.0+0.8) = 0.286
	inDelta(t, 2.0/(2.0+0.8), resA.OverallEquivalenceScore, 0.01, "CaseA score")
	inDelta(t, 0.8/(2.0+0.8), resB.OverallEquivalenceScore, 0.01, "CaseB score")
}

func TestAnalyze_ElementTypeSubstituentLeniency(t *testing.T) {
	// ElementTypeSubstituent weight (0.8) is lower than ElementTypeFunctionalGroup (1.0), so
	// a failing substituent hurts the overall score less.
	model := newMockModel()
	qFG := makeElement("qfg", ElementTypeFunctionalGroup, "amine")
	cFG := makeElement("cfg", ElementTypeFunctionalGroup, "amine variant")
	qSub := makeElement("qsub", ElementTypeSubstituent, "methyl")
	cSub := makeElement("csub", ElementTypeSubstituent, "ethyl")

	// FG passes, Sub fails.
	model.funcScores[pairKey(qFG, cFG)] = 0.90
	model.wayScores[pairKey(qFG, cFG)] = 0.80
	model.resScores[pairKey(qFG, cFG)] = 0.75
	model.funcScores[pairKey(qSub, cSub)] = 0.30

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, _ := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{qFG, qSub},
		ClaimElements: []*StructuralElement{cFG, cSub},
	})

	// FG weight=1.0, Sub weight=0.8 => score = 1.0/(1.0+0.8) ≈ 0.556
	expected := 1.0 / (1.0 + 0.8)
	inDelta(t, expected, res.OverallEquivalenceScore, 0.01, "substituent leniency score")
}

func TestAnalyze_ElementAlignment(t *testing.T) {
	model := newMockModel()
	// Query has ElementTypeCoreScaffold + ElementTypeSubstituent; Claim has ElementTypeSubstituent + ElementTypeCoreScaffold (reversed order).
	qScaff := makeElement("qs", ElementTypeCoreScaffold, "fluorene core")
	qSub := makeElement("qsub", ElementTypeSubstituent, "phenyl arm")
	cSub := makeElement("csub", ElementTypeSubstituent, "phenyl arm variant")
	cScaff := makeElement("cs", ElementTypeCoreScaffold, "fluorene core variant")

	// Same-type pairs should be matched.
	model.funcScores[pairKey(qScaff, cScaff)] = 0.92
	model.wayScores[pairKey(qScaff, cScaff)] = 0.85
	model.resScores[pairKey(qScaff, cScaff)] = 0.80
	model.funcScores[pairKey(qSub, cSub)] = 0.88
	model.wayScores[pairKey(qSub, cSub)] = 0.76
	model.resScores[pairKey(qSub, cSub)] = 0.70

	// Cross-type pairs should NOT be used.
	model.funcScores[pairKey(qScaff, cSub)] = 0.10
	model.funcScores[pairKey(qSub, cScaff)] = 0.10

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{qScaff, qSub},
		ClaimElements: []*StructuralElement{cSub, cScaff}, // deliberately reversed
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EquivalentElementCount != 2 {
		t.Errorf("expected 2 equivalent (same-type aligned), got %d", res.EquivalentElementCount)
	}
}

func TestAnalyze_ProsecutionHistoryEstoppel(t *testing.T) {
	model := newMockModel()
	qSub := makeElementWithSMILES("qsub", ElementTypeSubstituent, "naphthyl substituent", "c1ccc2ccccc2c1")
	cSub := makeElement("csub", ElementTypeSubstituent, "aromatic substituent")

	// Would normally pass.
	model.funcScores[pairKey(qSub, cSub)] = 0.90
	model.wayScores[pairKey(qSub, cSub)] = 0.80
	model.resScores[pairKey(qSub, cSub)] = 0.75

	history := []*ProsecutionHistoryEntry{
		{
			EventType:      "amendment",
			AbandonedScope: "naphthyl",
			AbandonedType:  ElementTypeSubstituent,
			Reason:         "applicant narrowed claim to exclude naphthyl substituents",
		},
	}

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule:      []*StructuralElement{qSub},
		ClaimElements:      []*StructuralElement{cSub},
		ProsecutionHistory: history,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EquivalentElementCount != 0 {
		t.Errorf("expected 0 equivalent (estoppel), got %d", res.EquivalentElementCount)
	}
	if len(res.NonEquivalentElements) != 1 {
		t.Fatalf("expected 1 non-equivalent, got %d", len(res.NonEquivalentElements))
	}
	if res.NonEquivalentElements[0].FailedStep != "estoppel" {
		t.Errorf("expected failedStep=estoppel, got %s", res.NonEquivalentElements[0].FailedStep)
	}
}

func TestAnalyze_EmptyQueryElements(t *testing.T) {
	model := newMockModel()
	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	_, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{},
		ClaimElements: []*StructuralElement{makeElement("c1", ElementTypeSubstituent, "x")},
	})
	if err == nil {
		t.Fatal("expected error for empty query elements")
	}
}

func TestAnalyze_EmptyClaimElements(t *testing.T) {
	model := newMockModel()
	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	_, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{makeElement("q1", ElementTypeSubstituent, "x")},
		ClaimElements: []*StructuralElement{},
	})
	if err == nil {
		t.Fatal("expected error for empty claim elements")
	}
}

func TestAnalyze_NilRequest(t *testing.T) {
	model := newMockModel()
	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	_, err := a.Analyze(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAnalyze_MismatchedElementCounts(t *testing.T) {
	model := newMockModel()
	// 3 query elements, 2 claim elements.
	query := []*StructuralElement{
		makeElement("q1", ElementTypeCoreScaffold, "scaffold A"),
		makeElement("q2", ElementTypeSubstituent, "sub A"),
		makeElement("q3", ElementTypeFunctionalGroup, "fg A"),
	}
	claim := []*StructuralElement{
		makeElement("c1", ElementTypeCoreScaffold, "scaffold B"),
		makeElement("c2", ElementTypeSubstituent, "sub B"),
	}

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: query,
		ClaimElements: claim,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should align 2 pairs (scaffold-scaffold, sub-sub). FG has no match.
	if res.TotalElementCount != 2 {
		t.Errorf("expected 2 aligned pairs, got %d", res.TotalElementCount)
	}
}

// =========================================================================
// Option tests
// =========================================================================

func TestEquivalentsOption_CustomThresholds(t *testing.T) {
	model := newMockModel()
	q := makeElement("q1", ElementTypeSubstituent, "phenyl")
	c := makeElement("c1", ElementTypeSubstituent, "naphthyl")

	// Scores that pass default thresholds but fail custom higher thresholds.
	model.funcScores[pairKey(q, c)] = 0.75
	model.wayScores[pairKey(q, c)] = 0.65
	model.resScores[pairKey(q, c)] = 0.68

	// Default thresholds: should pass.
	aDefault, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	eqDefault, _ := aDefault.AnalyzeElement(context.Background(), q, c)
	if !eqDefault.IsEquivalent {
		t.Error("expected pass with default thresholds")
	}

	// Custom higher thresholds: should fail.
	aCustom, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{},
		WithFunctionThreshold(0.80),
		WithWayThreshold(0.70),
		WithResultThreshold(0.75),
	)
	eqCustom, _ := aCustom.AnalyzeElement(context.Background(), q, c)
	if eqCustom.IsEquivalent {
		t.Error("expected fail with custom higher thresholds")
	}
}

func TestEquivalentsOption_CustomScaffoldWeight(t *testing.T) {
	model := newMockModel()
	qScaff := makeElement("qs", ElementTypeCoreScaffold, "carbazole")
	cScaff := makeElement("cs", ElementTypeCoreScaffold, "carbazole variant")
	qSub := makeElement("qsub", ElementTypeSubstituent, "phenyl")
	cSub := makeElement("csub", ElementTypeSubstituent, "trifluoromethyl")

	// Scaffold passes, substituent fails.
	model.funcScores[pairKey(qScaff, cScaff)] = 0.90
	model.wayScores[pairKey(qScaff, cScaff)] = 0.80
	model.resScores[pairKey(qScaff, cScaff)] = 0.75
	model.funcScores[pairKey(qSub, cSub)] = 0.30

	// Default scaffold weight = 2.0 => score = 2.0/(2.0+0.8) ≈ 0.714
	aDefault, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	resDefault, _ := aDefault.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{qScaff, qSub},
		ClaimElements: []*StructuralElement{cScaff, cSub},
	})

	// Custom scaffold weight = 5.0 => score = 5.0/(5.0+0.8) ≈ 0.862
	aHeavy, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{},
		WithScaffoldWeight(5.0),
	)
	resHeavy, _ := aHeavy.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule: []*StructuralElement{qScaff, qSub},
		ClaimElements: []*StructuralElement{cScaff, cSub},
	})

	inDelta(t, 2.0/(2.0+0.8), resDefault.OverallEquivalenceScore, 0.01, "default weight score")
	inDelta(t, 5.0/(5.0+0.8), resHeavy.OverallEquivalenceScore, 0.01, "heavy weight score")

	if resHeavy.OverallEquivalenceScore <= resDefault.OverallEquivalenceScore {
		t.Errorf(
			"heavier scaffold weight should yield higher score when scaffold is equivalent: %.4f vs %.4f",
			resHeavy.OverallEquivalenceScore, resDefault.OverallEquivalenceScore,
		)
	}
}

// =========================================================================
// ElementType tests
// =========================================================================

func TestElementType_String(t *testing.T) {
	tests := []struct {
		et   ElementType
		want string
	}{
		{ElementTypeCoreScaffold, "CoreScaffold"},
		{ElementTypeSubstituent, "Substituent"},
		{ElementTypeFunctionalGroup, "FunctionalGroup"},
		{ElementTypeLinker, "Linker"},
		{ElementTypeElectronicProperty, "ElectronicProperty"},
	}
	for _, tt := range tests {
		got := tt.et.String()
		if got != tt.want {
			t.Errorf("ElementType(%d).String() = %q, want %q", int(tt.et), got, tt.want)
		}
	}
}

func TestElementType_String_Unknown(t *testing.T) {
	unknown := ElementType(999)
	got := unknown.String()
	if got == "" {
		t.Error("expected non-empty string for unknown ElementType")
	}
	if !containsSubstring(got, "999") {
		t.Errorf("expected string to contain '999', got %q", got)
	}
}

func TestElementType_AllValues(t *testing.T) {
	expected := []ElementType{
		ElementTypeCoreScaffold,
		ElementTypeSubstituent,
		ElementTypeFunctionalGroup,
		ElementTypeLinker,
		ElementTypeBackbone,
		ElementTypeElectronicProperty,
	}
	if len(allElementTypes) != len(expected) {
		t.Fatalf("expected %d element types, got %d", len(expected), len(allElementTypes))
	}
	for i, et := range expected {
		if allElementTypes[i] != et {
			t.Errorf("allElementTypes[%d] = %v, want %v", i, allElementTypes[i], et)
		}
		// Each must have a non-fallback String().
		s := et.String()
		if containsSubstring(s, "ElementType(") {
			t.Errorf("ElementType %d should have a named string, got %q", int(et), s)
		}
	}
}

// =========================================================================
// Edge case: model returns error
// =========================================================================

func TestAnalyzeElement_ModelFunctionError(t *testing.T) {
	model := newMockModel()
	model.funcErr = fmt.Errorf("GPU OOM")
	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	_, err := a.AnalyzeElement(context.Background(),
		makeElement("q1", ElementTypeSubstituent, "x"),
		makeElement("c1", ElementTypeSubstituent, "y"),
	)
	if err == nil {
		t.Fatal("expected error when model returns error")
	}
	if !containsSubstring(err.Error(), "function similarity") {
		t.Errorf("error should mention function similarity, got: %v", err)
	}
}

func TestAnalyzeElement_ModelWayError(t *testing.T) {
	model := newMockModel()
	model.wayErr = fmt.Errorf("timeout")
	q := makeElement("q1", ElementTypeSubstituent, "x")
	c := makeElement("c1", ElementTypeSubstituent, "y")
	model.funcScores[pairKey(q, c)] = 0.90

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	_, err := a.AnalyzeElement(context.Background(), q, c)
	if err == nil {
		t.Fatal("expected error when way model returns error")
	}
	if !containsSubstring(err.Error(), "way similarity") {
		t.Errorf("error should mention way similarity, got: %v", err)
	}
}

func TestAnalyzeElement_ModelResultError(t *testing.T) {
	model := newMockModel()
	model.resErr = fmt.Errorf("connection refused")
	q := makeElement("q1", ElementTypeSubstituent, "x")
	c := makeElement("c1", ElementTypeSubstituent, "y")
	model.funcScores[pairKey(q, c)] = 0.90
	model.wayScores[pairKey(q, c)] = 0.80

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	_, err := a.AnalyzeElement(context.Background(), q, c)
	if err == nil {
		t.Fatal("expected error when result model returns error")
	}
	if !containsSubstring(err.Error(), "result similarity") {
		t.Errorf("error should mention result similarity, got: %v", err)
	}
}

// =========================================================================
// Prosecution history: SMILES-based estoppel
// =========================================================================

func TestAnalyze_ProsecutionHistoryEstoppel_SMILESMatch(t *testing.T) {
	model := newMockModel()
	qSub := makeElementWithSMILES("qsub", ElementTypeSubstituent, "some aromatic", "c1ccc2ccccc2c1")
	cSub := makeElement("csub", ElementTypeSubstituent, "aromatic group")

	model.funcScores[pairKey(qSub, cSub)] = 0.95
	model.wayScores[pairKey(qSub, cSub)] = 0.90
	model.resScores[pairKey(qSub, cSub)] = 0.85

	history := []*ProsecutionHistoryEntry{
		{
			EventType:       "restriction",
			AbandonedScope:  "fused bicyclic",
			AbandonedSMILES: "c1ccc2ccccc2c1",
			AbandonedType:   ElementTypeSubstituent,
			Reason:          "restricted to monocyclic aromatics only",
		},
	}

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule:      []*StructuralElement{qSub},
		ClaimElements:      []*StructuralElement{cSub},
		ProsecutionHistory: history,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EquivalentElementCount != 0 {
		t.Errorf("expected 0 equivalent (SMILES estoppel), got %d", res.EquivalentElementCount)
	}
	if len(res.NonEquivalentElements) == 0 {
		t.Fatal("expected non-equivalent entry")
	}
	if res.NonEquivalentElements[0].FailedStep != "estoppel" {
		t.Errorf("expected estoppel, got %s", res.NonEquivalentElements[0].FailedStep)
	}
}

// =========================================================================
// Prosecution history: type mismatch does not trigger estoppel
// =========================================================================

func TestAnalyze_ProsecutionHistoryEstoppel_TypeMismatch(t *testing.T) {
	model := newMockModel()
	qScaff := makeElementWithSMILES("qs", ElementTypeCoreScaffold, "naphthyl scaffold", "c1ccc2ccccc2c1")
	cScaff := makeElement("cs", ElementTypeCoreScaffold, "aromatic scaffold")

	model.funcScores[pairKey(qScaff, cScaff)] = 0.90
	model.wayScores[pairKey(qScaff, cScaff)] = 0.80
	model.resScores[pairKey(qScaff, cScaff)] = 0.75

	// Estoppel targets ElementTypeSubstituent, not ElementTypeCoreScaffold.
	history := []*ProsecutionHistoryEntry{
		{
			EventType:      "amendment",
			AbandonedScope: "naphthyl",
			AbandonedType:  ElementTypeSubstituent, // different type
			Reason:         "narrowed substituent scope",
		},
	}

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	res, err := a.Analyze(context.Background(), &EquivalentsRequest{
		QueryMolecule:      []*StructuralElement{qScaff},
		ClaimElements:      []*StructuralElement{cScaff},
		ProsecutionHistory: history,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should NOT be blocked because estoppel type doesn't match.
	if res.EquivalentElementCount != 1 {
		t.Errorf("expected 1 equivalent (type mismatch, no estoppel), got %d", res.EquivalentElementCount)
	}
}

// =========================================================================
// Helper: descriptionSimilarity unit test
// =========================================================================

func TestDescriptionSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"phenyl group", "phenyl group", 1.0, 1.0},
		{"phenyl group", "naphthyl group", 0.3, 0.6},
		{"", "", 1.0, 1.0},
		{"alpha", "beta", 0.0, 0.0},
		{"electron donor amine", "electron donor variant", 0.3, 0.7},
	}
	for _, tt := range tests {
		sim := descriptionSimilarity(tt.a, tt.b)
		if sim < tt.min || sim > tt.max {
			t.Errorf("descriptionSimilarity(%q, %q) = %.4f, expected in [%.2f, %.2f]",
				tt.a, tt.b, sim, tt.min, tt.max)
		}
	}
}

// =========================================================================
// Helper: clampScore
// =========================================================================

func TestClampScore(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{0.5, 0.5},
		{0.0, 0.0},
		{1.0, 1.0},
		{-0.1, 0.0},
		{1.5, 1.0},
		{math.NaN(), 0.0},
		{math.Inf(1), 0.0},
		{math.Inf(-1), 0.0},
	}
	for _, tt := range tests {
		got := clampScore(tt.in)
		inDelta(t, tt.want, got, 0.001, fmt.Sprintf("clampScore(%v)", tt.in))
	}
}

// =========================================================================
// Helper: containsSubstring
// =========================================================================

func containsSubstring(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// =========================================================================
// Benchmark
// =========================================================================

func BenchmarkAnalyzeElement_AllPass(b *testing.B) {
	model := newMockModel()
	q := makeElement("q1", ElementTypeSubstituent, "phenyl group")
	c := makeElement("c1", ElementTypeSubstituent, "naphthyl group")
	model.funcScores[pairKey(q, c)] = 0.85
	model.wayScores[pairKey(q, c)] = 0.78
	model.resScores[pairKey(q, c)] = 0.72

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = a.AnalyzeElement(ctx, q, c)
	}
}

func BenchmarkAnalyze_TenElements(b *testing.B) {
	model := newMockModel()
	query := make([]*StructuralElement, 10)
	claim := make([]*StructuralElement, 10)
	types := []ElementType{ElementTypeCoreScaffold, ElementTypeSubstituent, ElementTypeSubstituent, ElementTypeFunctionalGroup, ElementTypeFunctionalGroup,
		ElementTypeLinker, ElementTypeElectronicProperty, ElementTypeSubstituent, ElementTypeSubstituent, ElementTypeFunctionalGroup}
	for i := 0; i < 10; i++ {
		query[i] = makeElement(fmt.Sprintf("q%d", i), types[i], fmt.Sprintf("query element %d", i))
		claim[i] = makeElement(fmt.Sprintf("c%d", i), types[i], fmt.Sprintf("claim element %d", i))
	}

	a, _ := NewEquivalentsAnalyzer(model, &mockEquivalentsLogger{})
	ctx := context.Background()
	req := &EquivalentsRequest{QueryMolecule: query, ClaimElements: claim}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = a.Analyze(ctx, req)
	}
}

// =========================================================================
// Mock: Logger
// =========================================================================

type mockEquivalentsLogger struct{}

func (l *mockEquivalentsLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *mockEquivalentsLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *mockEquivalentsLogger) Error(msg string, keysAndValues ...interface{}) {}
func (l *mockEquivalentsLogger) Debug(msg string, keysAndValues ...interface{}) {}

