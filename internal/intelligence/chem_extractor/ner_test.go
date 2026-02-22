package chem_extractor

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ---------------------------------------------------------------------------
// Mock backend
// ---------------------------------------------------------------------------

// mockNERBackend returns pre-configured emission matrices.
type mockNERBackend struct {
	emissionFn func(tokens []string) [][]float64
}

func (m *mockNERBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	tokens, _ := common.DecodeTokenList(req.InputData)
	var emission [][]float64
	if m.emissionFn != nil {
		emission = m.emissionFn(tokens)
	} else {
		emission = makeAllOEmission(len(tokens), len(DefaultLabelSet))
	}
	// In a real scenario, this would be unmarshaled JSON ([]interface{}),
	// but common.DecodeFloat64Matrix also accepts [][]float64 directly.
	return &common.PredictResponse{
		Outputs: map[string]interface{}{"emission": emission},
	}, nil
}

func (m *mockNERBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, nil
}

func (m *mockNERBackend) Healthy(ctx context.Context) error { return nil }
func (m *mockNERBackend) Close() error                      { return nil }

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// labelIdx returns the index of a label in DefaultLabelSet.
func labelIdx(label string) int {
	for i, l := range DefaultLabelSet {
		if l == label {
			return i
		}
	}
	return 0 // O
}

// makeAllOEmission creates an emission matrix where every token has high O probability.
func makeAllOEmission(seqLen, numLabels int) [][]float64 {
	m := make([][]float64, seqLen)
	for i := range m {
		row := make([]float64, numLabels)
		for j := range row {
			row[j] = 0.01
		}
		row[labelIdx(LabelO)] = 0.95
		m[i] = row
	}
	return m
}

// makeEmissionMatrix builds an emission matrix from a list of desired labels with high probs.
// labels[i] is the desired label for token i; that label gets highProb, others get lowProb.
func makeEmissionMatrix(labels []string, highProb, lowProb float64) [][]float64 {
	numLabels := len(DefaultLabelSet)
	m := make([][]float64, len(labels))
	for i, l := range labels {
		row := make([]float64, numLabels)
		for j := range row {
			row[j] = lowProb
		}
		idx := labelIdx(l)
		row[idx] = highProb
		m[i] = row
	}
	return m
}

// makeTransitionMatrix builds a BIO-legal transition matrix for testing.
func makeTransitionMatrix(labelSet []string) [][]float64 {
	idx := make(map[string]int, len(labelSet))
	for i, l := range labelSet {
		idx[l] = i
	}
	return buildBIOTransitionMatrix(labelSet, idx)
}

// newTestNERModel creates a NER model with a mock backend.
func newTestNERModel(t *testing.T, emissionFn func(tokens []string) [][]float64) NERModel {
	t.Helper()
	backend := &mockNERBackend{emissionFn: emissionFn}
	cfg := DefaultNERModelConfig()
	cfg.UseCRF = true
	model, err := NewNERModel(backend, cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewNERModel: %v", err)
	}
	return model
}

func newTestNERModelNoCRF(t *testing.T, emissionFn func(tokens []string) [][]float64) NERModel {
	t.Helper()
	backend := &mockNERBackend{emissionFn: emissionFn}
	cfg := DefaultNERModelConfig()
	cfg.UseCRF = false
	model, err := NewNERModel(backend, cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewNERModel: %v", err)
	}
	return model
}

// assertEntity checks that an entity matches expectations.
func assertEntity(t *testing.T, e *NEREntity, text, label string) {
	t.Helper()
	if e.Text != text {
		t.Errorf("entity text = %q, want %q", e.Text, text)
	}
	if e.Label != label {
		t.Errorf("entity label = %q, want %q", e.Label, label)
	}
}

// ---------------------------------------------------------------------------
// Tests: Predict simple entities
// ---------------------------------------------------------------------------

func TestPredict_SimpleEntity(t *testing.T) {
	// "aspirin is a drug" -> aspirin = B-COMMON, rest = O
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			if strings.EqualFold(tok, "aspirin") {
				labels[i] = LabelBCOMMON
			}
		}
		return makeEmissionMatrix(labels, 0.95, 0.02)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), "aspirin is a drug")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(pred.Entities))
	}
	assertEntity(t, pred.Entities[0], "aspirin", "COMMON")
}

func TestPredict_MultiTokenEntity(t *testing.T) {
	// "2-acetoxybenzoic acid" -> B-IUPAC, I-IUPAC
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			if strings.Contains(strings.ToLower(tok), "acetoxybenzoic") {
				labels[i] = LabelBIUPAC
			} else if strings.EqualFold(tok, "acid") {
				labels[i] = LabelIIUPAC
			}
		}
		return makeEmissionMatrix(labels, 0.92, 0.01)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), "2-acetoxybenzoic acid")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Entities) < 1 {
		t.Fatalf("expected at least 1 entity, got %d", len(pred.Entities))
	}
	found := false
	for _, e := range pred.Entities {
		if e.Label == "IUPAC" {
			found = true
			if e.EndToken-e.StartToken < 2 {
				t.Errorf("expected multi-token IUPAC entity, got span length %d", e.EndToken-e.StartToken)
			}
		}
	}
	if !found {
		t.Error("expected IUPAC entity not found")
	}
}

func TestPredict_MultipleEntities(t *testing.T) {
	// "aspirin and ibuprofen" -> two COMMON entities
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			lower := strings.ToLower(tok)
			if lower == "aspirin" || lower == "ibuprofen" {
				labels[i] = LabelBCOMMON
			}
		}
		return makeEmissionMatrix(labels, 0.93, 0.01)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), "aspirin and ibuprofen")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(pred.Entities))
	}
	assertEntity(t, pred.Entities[0], "aspirin", "COMMON")
	assertEntity(t, pred.Entities[1], "ibuprofen", "COMMON")
}

func TestPredict_CASNumber(t *testing.T) {
	// "CAS 50-78-2" -> B-CAS, I-CAS
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			if strings.EqualFold(tok, "CAS") {
				labels[i] = LabelBCAS
			} else if tok == "50-78-2" {
				labels[i] = LabelICAS
			}
		}
		return makeEmissionMatrix(labels, 0.90, 0.01)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), "CAS 50-78-2")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	found := false
	for _, e := range pred.Entities {
		if e.Label == "CAS" {
			found = true
		}
	}
	if !found {
		t.Error("expected CAS entity not found")
	}
}

func TestPredict_MolecularFormula(t *testing.T) {
	// "C9H8O4" -> B-FORMULA
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			if tok == "C9H8O4" {
				labels[i] = LabelBFORMULA
			}
		}
		return makeEmissionMatrix(labels, 0.94, 0.01)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), "C9H8O4")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(pred.Entities))
	}
	assertEntity(t, pred.Entities[0], "C9H8O4", "FORMULA")
}

func TestPredict_SMILES(t *testing.T) {
	// SMILES string as single token (no spaces/punctuation splitting it)
	smiles := "CC(=O)Oc1ccccc1C(=O)O"
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		// The tokenizer may split on parentheses; find the first token and mark B-SMILES
		if len(tokens) > 0 {
			labels[0] = LabelBSMILES
			for j := 1; j < len(tokens); j++ {
				labels[j] = LabelISMILES
			}
		}
		return makeEmissionMatrix(labels, 0.88, 0.01)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), smiles)
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	found := false
	for _, e := range pred.Entities {
		if e.Label == "SMILES" {
			found = true
		}
	}
	if !found {
		t.Error("expected SMILES entity not found")
	}
}

func TestPredict_MarkushVariable(t *testing.T) {
	// "R1 is alkyl" -> B-MARKUSH, I-MARKUSH
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			if tok == "R1" {
				labels[i] = LabelBMARKUSH
			} else if strings.EqualFold(tok, "is") && i > 0 && labels[i-1] == LabelBMARKUSH {
				labels[i] = LabelIMARKUSH
			} else if strings.EqualFold(tok, "alkyl") {
				labels[i] = LabelIMARKUSH
			}
		}
		return makeEmissionMatrix(labels, 0.85, 0.02)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), "R1 is alkyl")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	found := false
	for _, e := range pred.Entities {
		if e.Label == "MARKUSH" {
			found = true
		}
	}
	if !found {
		t.Error("expected MARKUSH entity not found")
	}
}

func TestPredict_NoEntities(t *testing.T) {
	// "the method comprises" -> all O
	model := newTestNERModel(t, nil) // default: all O
	pred, err := model.Predict(context.Background(), "the method comprises")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(pred.Entities))
	}
	for i, l := range pred.Labels {
		if l != LabelO {
			t.Errorf("token %d label = %s, want O", i, l)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Entity span extraction
// ---------------------------------------------------------------------------

func TestPredict_EntitySpanExtraction(t *testing.T) {
	tokens := []string{"the", "compound", "2-acetoxybenzoic", "acid", "is", "used"}
	labels := []string{LabelO, LabelO, LabelBIUPAC, LabelIIUPAC, LabelO, LabelO}
	probs := makeEmissionMatrix(labels, 0.90, 0.01)
	spans := make([]tokenSpan, len(tokens))
	offset := 0
	for i, tok := range tokens {
		spans[i] = tokenSpan{Text: tok, StartChar: offset, EndChar: offset + len(tok)}
		offset += len(tok) + 1
	}

	entities := bioToEntities(tokens, labels, probs, spans)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	e := entities[0]
	if e.Text != "2-acetoxybenzoic acid" {
		t.Errorf("entity text = %q, want %q", e.Text, "2-acetoxybenzoic acid")
	}
	if e.Label != "IUPAC" {
		t.Errorf("entity label = %q, want IUPAC", e.Label)
	}
	if e.StartToken != 2 || e.EndToken != 4 {
		t.Errorf("entity span = [%d, %d), want [2, 4)", e.StartToken, e.EndToken)
	}
}

func TestPredict_EntityCharOffset(t *testing.T) {
	text := "use aspirin daily"
	spans := tokenize(normalizeText(text))
	tokens := make([]string, len(spans))
	for i, s := range spans {
		tokens[i] = s.Text
	}

	labels := make([]string, len(tokens))
	for i := range labels {
		labels[i] = LabelO
	}
	for i, tok := range tokens {
		if tok == "aspirin" {
			labels[i] = LabelBCOMMON
		}
	}
	probs := makeEmissionMatrix(labels, 0.90, 0.01)

	entities := bioToEntities(tokens, labels, probs, spans)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	e := entities[0]
	if e.StartChar != 4 {
		t.Errorf("StartChar = %d, want 4", e.StartChar)
	}
	if e.EndChar != 11 {
		t.Errorf("EndChar = %d, want 11", e.EndChar)
	}
	if text[e.StartChar:e.EndChar] != "aspirin" {
		t.Errorf("char slice = %q, want aspirin", text[e.StartChar:e.EndChar])
	}
}

// ---------------------------------------------------------------------------
// Tests: Viterbi decoding
// ---------------------------------------------------------------------------

func TestViterbi_SimpleSequence(t *testing.T) {
	// 3 tokens, 3 labels: O, B-COMMON, I-COMMON
	ls := []string{LabelO, LabelBCOMMON, LabelICOMMON}
	li := map[string]int{LabelO: 0, LabelBCOMMON: 1, LabelICOMMON: 2}
	trans := buildBIOTransitionMatrix(ls, li)

	// Token 0: high B-COMMON, Token 1: high I-COMMON, Token 2: high O
	emission := [][]float64{
		{0.1, 0.8, 0.1},
		{0.1, 0.1, 0.8},
		{0.8, 0.1, 0.1},
	}

	labels := viterbiDecode(emission, trans, ls, li)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	if labels[0] != LabelBCOMMON {
		t.Errorf("labels[0] = %s, want B-COMMON", labels[0])
	}
	if labels[1] != LabelICOMMON {
		t.Errorf("labels[1] = %s, want I-COMMON", labels[1])
	}
	if labels[2] != LabelO {
		t.Errorf("labels[2] = %s, want O", labels[2])
	}
}

func TestViterbi_BIOConstraint(t *testing.T) {
	// Emission strongly favors I-COMMON at position 0, but BIO forbids it
	ls := []string{LabelO, LabelBCOMMON, LabelICOMMON}
	li := map[string]int{LabelO: 0, LabelBCOMMON: 1, LabelICOMMON: 2}
	trans := buildBIOTransitionMatrix(ls, li)

	emission := [][]float64{
		{0.05, 0.05, 0.90}, // wants I-COMMON but illegal at start
		{0.80, 0.10, 0.10},
	}

	labels := viterbiDecode(emission, trans, ls, li)
	// Position 0 should NOT be I-COMMON
	if labels[0] == LabelICOMMON {
		t.Errorf("labels[0] = I-COMMON, should be forbidden at start")
	}
}

func TestViterbi_LongSequence(t *testing.T) {
	ls := DefaultLabelSet
	li := make(map[string]int, len(ls))
	for i, l := range ls {
		li[l] = i
	}
	trans := buildBIOTransitionMatrix(ls, li)

	seqLen := 50
	emission := make([][]float64, seqLen)
	for i := range emission {
		row := make([]float64, len(ls))
		for j := range row {
			row[j] = 0.02
		}
		row[0] = 0.80 // O
		emission[i] = row
	}

	labels := viterbiDecode(emission, trans, ls, li)
	if len(labels) != seqLen {
		t.Fatalf("expected %d labels, got %d", seqLen, len(labels))
	}
	// All should be O given the emission
	for i, l := range labels {
		if l != LabelO {
			t.Errorf("labels[%d] = %s, want O", i, l)
		}
	}
}

func TestViterbi_UniformProbabilities(t *testing.T) {
	ls := []string{LabelO, LabelBCOMMON, LabelICOMMON}
	li := map[string]int{LabelO: 0, LabelBCOMMON: 1, LabelICOMMON: 2}
	trans := buildBIOTransitionMatrix(ls, li)

	// All uniform
	emission := [][]float64{
		{0.33, 0.33, 0.34},
		{0.33, 0.33, 0.34},
		{0.33, 0.33, 0.34},
	}

	labels := viterbiDecode(emission, trans, ls, li)
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	// Should produce a legal BIO sequence
	for i, l := range labels {
		if strings.HasPrefix(l, "I-") && i == 0 {
			t.Errorf("I- label at position 0 is illegal")
		}
		if i > 0 && strings.HasPrefix(l, "I-") {
			prevType := ""
			if strings.HasPrefix(labels[i-1], "B-") {
				prevType = labels[i-1][2:]
			} else if strings.HasPrefix(labels[i-1], "I-") {
				prevType = labels[i-1][2:]
			}
			curType := l[2:]
			if prevType != curType {
				t.Errorf("illegal transition %s -> %s at position %d", labels[i-1], l, i)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Sliding window
// ---------------------------------------------------------------------------

func TestSlidingWindow_ShortText(t *testing.T) {
	windows := buildSlidingWindows(10, 256)
	if len(windows) != 1 {
		t.Fatalf("expected 1 window, got %d", len(windows))
	}
	if windows[0][0] != 0 || windows[0][1] != 10 {
		t.Errorf("window = %v, want [0, 10)", windows[0])
	}
}

func TestSlidingWindow_LongText(t *testing.T) {
	maxLen := 256
	numTokens := 2 * maxLen // 512
	windows := buildSlidingWindows(numTokens, maxLen)
	// step = 128, so windows: [0,256), [128,384), [256,512) = 3 windows
	if len(windows) < 3 {
		t.Errorf("expected at least 3 windows, got %d", len(windows))
	}
	// First window
	if windows[0][0] != 0 || windows[0][1] != maxLen {
		t.Errorf("window[0] = %v, want [0, %d)", windows[0], maxLen)
	}
	// Last window should end at numTokens
	last := windows[len(windows)-1]
	if last[1] != numTokens {
		t.Errorf("last window end = %d, want %d", last[1], numTokens)
	}
}

func TestSlidingWindow_OverlapMerge(t *testing.T) {
	numLabels := 3

	// Two overlapping windows: [0,4) and [2,6)
	// Token indices 2,3 are in the overlap
	wr1 := &windowResult{
		StartToken: 0, EndToken: 4,
		Labels: []string{"O", "O", "B-X", "I-X"}, // tokens 2,3 predicted as entity
		Probabilities: [][]float64{
			{0.9, 0.05, 0.05},
			{0.9, 0.05, 0.05},
			{0.1, 0.8, 0.1},  // token 2: max=0.8
			{0.1, 0.1, 0.8},  // token 3: max=0.8
		},
	}
	wr2 := &windowResult{
		StartToken: 2, EndToken: 6,
		Labels: []string{"O", "O", "O", "O"}, // tokens 2,3 predicted as O
		Probabilities: [][]float64{
			{0.7, 0.15, 0.15}, // token 2: max=0.7 (lower)
			{0.7, 0.15, 0.15}, // token 3: max=0.7 (lower)
			{0.9, 0.05, 0.05},
			{0.9, 0.05, 0.05},
		},
	}

	labels, _ := mergeWindows([]*windowResult{wr1, wr2}, 6, numLabels)

	// Tokens 2,3 should keep wr1's prediction (higher prob)
	if labels[2] != "B-X" {
		t.Errorf("labels[2] = %s, want B-X (higher prob from window 1)", labels[2])
	}
	if labels[3] != "I-X" {
		t.Errorf("labels[3] = %s, want I-X (higher prob from window 1)", labels[3])
	}
}

func TestSlidingWindow_CrossBoundaryEntity(t *testing.T) {
	// Entity spans tokens 3-5, window boundary at 4
	// Window 1: [0,4) sees tokens 3 as B-COMMON
	// Window 2: [2,6) sees tokens 3 as B-COMMON, 4 as I-COMMON, 5 as I-COMMON
	numLabels := 3 // O=0, B-COMMON=1, I-COMMON=2

	wr1 := &windowResult{
		StartToken: 0, EndToken: 4,
		Labels: []string{"O", "O", "O", "B-COMMON"},
		Probabilities: [][]float64{
			{0.9, 0.05, 0.05},
			{0.9, 0.05, 0.05},
			{0.9, 0.05, 0.05},
			{0.1, 0.85, 0.05}, // token 3: B-COMMON with 0.85
		},
	}
	wr2 := &windowResult{
		StartToken: 2, EndToken: 6,
		Labels: []string{"O", "B-COMMON", "I-COMMON", "I-COMMON"},
		Probabilities: [][]float64{
			{0.9, 0.05, 0.05},
			{0.1, 0.88, 0.02}, // token 3: B-COMMON with 0.88 (higher)
			{0.1, 0.05, 0.85}, // token 4: I-COMMON
			{0.1, 0.05, 0.85}, // token 5: I-COMMON
		},
	}

	labels, _ := mergeWindows([]*windowResult{wr1, wr2}, 6, numLabels)

	// Token 3 should be B-COMMON (wr2 has higher prob)
	if labels[3] != "B-COMMON" {
		t.Errorf("labels[3] = %s, want B-COMMON", labels[3])
	}
	if labels[4] != "I-COMMON" {
		t.Errorf("labels[4] = %s, want I-COMMON", labels[4])
	}
	if labels[5] != "I-COMMON" {
		t.Errorf("labels[5] = %s, want I-COMMON", labels[5])
	}
}

func TestSlidingWindow_BIOLegality(t *testing.T) {
	// After merge, ensure fixBIOLegality corrects orphan I- tags
	labels := []string{"O", "I-COMMON", "I-COMMON", "O"}
	fixed := fixBIOLegality(labels)
	// First I-COMMON should become B-COMMON
	if fixed[1] != "B-COMMON" {
		t.Errorf("fixed[1] = %s, want B-COMMON", fixed[1])
	}
	if fixed[2] != "I-COMMON" {
		t.Errorf("fixed[2] = %s, want I-COMMON", fixed[2])
	}
}

// ---------------------------------------------------------------------------
// Tests: Confidence score
// ---------------------------------------------------------------------------

func TestConfidenceScore_AllHighProb(t *testing.T) {
	probs := [][]float64{
		{0.02, 0.95, 0.03},
		{0.02, 0.03, 0.95},
	}
	score := computeEntityConfidence(probs, 0, 2)
	if math.Abs(score-0.95) > 0.01 {
		t.Errorf("score = %f, want ~0.95", score)
	}
}

func TestConfidenceScore_OneLowProb(t *testing.T) {
	probs := [][]float64{
		{0.02, 0.95, 0.03}, // max = 0.95
		{0.30, 0.35, 0.35}, // max = 0.35
	}
	score := computeEntityConfidence(probs, 0, 2)
	// Geometric mean of 0.95 and 0.35 = sqrt(0.95*0.35) ≈ 0.577
	expected := math.Sqrt(0.95 * 0.35)
	if math.Abs(score-expected) > 0.01 {
		t.Errorf("score = %f, want ~%f", score, expected)
	}
	// Should be significantly lower than 0.95
	if score > 0.80 {
		t.Errorf("score %f should be significantly lower than 0.95 due to geometric mean", score)
	}
}

func TestConfidenceScore_SingleToken(t *testing.T) {
	probs := [][]float64{
		{0.05, 0.88, 0.07},
	}
	score := computeEntityConfidence(probs, 0, 1)
	if math.Abs(score-0.88) > 0.001 {
		t.Errorf("score = %f, want 0.88", score)
	}
}

func TestConfidenceThreshold_AboveThreshold(t *testing.T) {
	tokens := []string{"aspirin"}
	labels := []string{LabelBCOMMON}
	probs := makeEmissionMatrix(labels, 0.80, 0.02)
	spans := []tokenSpan{{Text: "aspirin", StartChar: 0, EndChar: 7}}

	entities := bioToEntities(tokens, labels, probs, spans)
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	// Score should be ~0.80, above default threshold 0.60
	if entities[0].Score < 0.60 {
		t.Errorf("score %f should be above threshold 0.60", entities[0].Score)
	}
}

func TestConfidenceThreshold_BelowThreshold(t *testing.T) {
	// Create model with threshold 0.60, entity with score ~0.50
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		if len(tokens) > 0 {
			labels[0] = LabelBCOMMON
		}
		// Low probability for the entity token
		return makeEmissionMatrix(labels, 0.50, 0.05)
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), "aspirin")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	// Entity should be filtered out because score ~0.50 < threshold 0.60
	for _, e := range pred.Entities {
		if e.Label == "COMMON" && e.Score < 0.60 {
			t.Errorf("entity with score %f should have been filtered (threshold 0.60)", e.Score)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: PredictBatch
// ---------------------------------------------------------------------------

func TestPredictBatch_Success(t *testing.T) {
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			if strings.EqualFold(tok, "aspirin") || strings.EqualFold(tok, "ibuprofen") || strings.EqualFold(tok, "caffeine") {
				labels[i] = LabelBCOMMON
			}
		}
		return makeEmissionMatrix(labels, 0.92, 0.01)
	}
	model := newTestNERModel(t, emFn)
	texts := []string{
		"aspirin is used",
		"ibuprofen treats pain",
		"caffeine is a stimulant",
	}
	results, err := model.PredictBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("PredictBatch: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r == nil {
			t.Errorf("result[%d] is nil", i)
			continue
		}
		if len(r.Entities) < 1 {
			t.Errorf("result[%d] expected at least 1 entity, got %d", i, len(r.Entities))
		}
	}
}

func TestPredictBatch_EmptyBatch(t *testing.T) {
	model := newTestNERModel(t, nil)
	results, err := model.PredictBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("PredictBatch: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Tests: Edge cases
// ---------------------------------------------------------------------------

func TestPredict_EmptyText(t *testing.T) {
	model := newTestNERModel(t, nil)
	pred, err := model.Predict(context.Background(), "")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(pred.Tokens))
	}
	if len(pred.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(pred.Entities))
	}
}

func TestPredict_WhitespaceOnlyText(t *testing.T) {
	model := newTestNERModel(t, nil)
	pred, err := model.Predict(context.Background(), "   \t\n  ")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(pred.Tokens))
	}
}

func TestPredict_LongTextSlidingWindow(t *testing.T) {
	// Generate text longer than MaxSequenceLength (256 tokens)
	words := make([]string, 300)
	for i := range words {
		words[i] = fmt.Sprintf("word%d", i)
	}
	longText := strings.Join(words, " ")

	emFn := func(tokens []string) [][]float64 {
		return makeAllOEmission(len(tokens), len(DefaultLabelSet))
	}
	model := newTestNERModel(t, emFn)
	pred, err := model.Predict(context.Background(), longText)
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Tokens) != 300 {
		t.Errorf("expected 300 tokens, got %d", len(pred.Tokens))
	}
	if len(pred.Labels) != 300 {
		t.Errorf("expected 300 labels, got %d", len(pred.Labels))
	}
}

func TestGetLabelSet(t *testing.T) {
	model := newTestNERModel(t, nil)
	ls := model.GetLabelSet()
	if len(ls) != 15 {
		t.Errorf("expected 15 labels, got %d", len(ls))
	}
	// Verify all expected labels present
	expected := map[string]bool{
		LabelO:        true,
		LabelBIUPAC:   true, LabelIIUPAC: true,
		LabelBCAS:     true, LabelICAS: true,
		LabelBFORMULA: true, LabelIFORMULA: true,
		LabelBSMILES:  true, LabelISMILES: true,
		LabelBCOMMON:  true, LabelICOMMON: true,
		LabelBGENERIC: true, LabelIGENERIC: true,
		LabelBMARKUSH: true, LabelIMARKUSH: true,
	}
	for _, l := range ls {
		if !expected[l] {
			t.Errorf("unexpected label %q in label set", l)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: fixBIOLegality
// ---------------------------------------------------------------------------

func TestFixBIOLegality_OrphanIAtStart(t *testing.T) {
	labels := []string{"I-IUPAC", "I-IUPAC", "O"}
	fixed := fixBIOLegality(labels)
	if fixed[0] != "B-IUPAC" {
		t.Errorf("fixed[0] = %s, want B-IUPAC", fixed[0])
	}
	if fixed[1] != "I-IUPAC" {
		t.Errorf("fixed[1] = %s, want I-IUPAC", fixed[1])
	}
}

func TestFixBIOLegality_OrphanIAfterO(t *testing.T) {
	labels := []string{"O", "I-CAS", "I-CAS"}
	fixed := fixBIOLegality(labels)
	if fixed[1] != "B-CAS" {
		t.Errorf("fixed[1] = %s, want B-CAS", fixed[1])
	}
	if fixed[2] != "I-CAS" {
		t.Errorf("fixed[2] = %s, want I-CAS", fixed[2])
	}
}

func TestFixBIOLegality_TypeMismatch(t *testing.T) {
	labels := []string{"B-IUPAC", "I-CAS", "O"}
	fixed := fixBIOLegality(labels)
	// I-CAS after B-IUPAC -> should become B-CAS
	if fixed[1] != "B-CAS" {
		t.Errorf("fixed[1] = %s, want B-CAS", fixed[1])
	}
}

func TestFixBIOLegality_AlreadyLegal(t *testing.T) {
	labels := []string{"O", "B-COMMON", "I-COMMON", "O", "B-IUPAC", "O"}
	fixed := fixBIOLegality(labels)
	for i, l := range labels {
		if fixed[i] != l {
			t.Errorf("fixed[%d] = %s, want %s (should be unchanged)", i, fixed[i], l)
		}
	}
}

func TestFixBIOLegality_AllO(t *testing.T) {
	labels := []string{"O", "O", "O"}
	fixed := fixBIOLegality(labels)
	for i, l := range fixed {
		if l != "O" {
			t.Errorf("fixed[%d] = %s, want O", i, l)
		}
	}
}

func TestFixBIOLegality_Empty(t *testing.T) {
	fixed := fixBIOLegality([]string{})
	if len(fixed) != 0 {
		t.Errorf("expected empty, got %d", len(fixed))
	}
}

// ---------------------------------------------------------------------------
// Tests: BIO transition matrix
// ---------------------------------------------------------------------------

func TestBIOTransitionMatrix_OToI_Illegal(t *testing.T) {
	trans := makeTransitionMatrix(DefaultLabelSet)
	oIdx := labelIdx(LabelO)
	for _, l := range DefaultLabelSet {
		if strings.HasPrefix(l, "I-") {
			iIdx := labelIdx(l)
			if trans[oIdx][iIdx] != 0.0 {
				t.Errorf("O -> %s should be 0.0, got %f", l, trans[oIdx][iIdx])
			}
		}
	}
}

func TestBIOTransitionMatrix_OToB_Legal(t *testing.T) {
	trans := makeTransitionMatrix(DefaultLabelSet)
	oIdx := labelIdx(LabelO)
	for _, l := range DefaultLabelSet {
		if strings.HasPrefix(l, "B-") {
			bIdx := labelIdx(l)
			if trans[oIdx][bIdx] != 1.0 {
				t.Errorf("O -> %s should be 1.0, got %f", l, trans[oIdx][bIdx])
			}
		}
	}
}

func TestBIOTransitionMatrix_BToMatchingI_Legal(t *testing.T) {
	trans := makeTransitionMatrix(DefaultLabelSet)
	bIdx := labelIdx(LabelBIUPAC)
	iIdx := labelIdx(LabelIIUPAC)
	if trans[bIdx][iIdx] != 1.0 {
		t.Errorf("B-IUPAC -> I-IUPAC should be 1.0, got %f", trans[bIdx][iIdx])
	}
}

func TestBIOTransitionMatrix_BToMismatchI_Illegal(t *testing.T) {
	trans := makeTransitionMatrix(DefaultLabelSet)
	bIdx := labelIdx(LabelBIUPAC)
	iIdx := labelIdx(LabelICAS)
	if trans[bIdx][iIdx] != 0.0 {
		t.Errorf("B-IUPAC -> I-CAS should be 0.0, got %f", trans[bIdx][iIdx])
	}
}

// ---------------------------------------------------------------------------
// Tests: Tokenizer
// ---------------------------------------------------------------------------

func TestTokenize_Simple(t *testing.T) {
	spans := tokenize("hello world")
	if len(spans) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(spans))
	}
	if spans[0].Text != "hello" || spans[1].Text != "world" {
		t.Errorf("tokens = [%s, %s], want [hello, world]", spans[0].Text, spans[1].Text)
	}
}

func TestTokenize_Punctuation(t *testing.T) {
	spans := tokenize("hello, world!")
	// Should split: "hello", ",", "world", "!"
	if len(spans) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(spans), spansToStrings(spans))
	}
}

func TestTokenize_ChemicalHyphen(t *testing.T) {
	// Hyphens inside words should NOT be split (chemical names)
	spans := tokenize("2-acetoxybenzoic acid")
	if len(spans) != 2 {
		t.Fatalf("expected 2 tokens, got %d: %v", len(spans), spansToStrings(spans))
	}
	if spans[0].Text != "2-acetoxybenzoic" {
		t.Errorf("token[0] = %q, want 2-acetoxybenzoic", spans[0].Text)
	}
}

func TestTokenize_Empty(t *testing.T) {
	spans := tokenize("")
	if len(spans) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(spans))
	}
}

func TestTokenize_Offsets(t *testing.T) {
	text := "ab cd"
	spans := tokenize(text)
	if len(spans) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(spans))
	}
	if spans[0].StartChar != 0 || spans[0].EndChar != 2 {
		t.Errorf("token[0] offsets = [%d, %d), want [0, 2)", spans[0].StartChar, spans[0].EndChar)
	}
	if spans[1].StartChar != 3 || spans[1].EndChar != 5 {
		t.Errorf("token[1] offsets = [%d, %d), want [3, 5)", spans[1].StartChar, spans[1].EndChar)
	}
}

func spansToStrings(spans []tokenSpan) []string {
	out := make([]string, len(spans))
	for i, s := range spans {
		out[i] = s.Text
	}
	return out
}

// ---------------------------------------------------------------------------
// Tests: normalizeText
// ---------------------------------------------------------------------------

func TestNormalizeText_CollapseWhitespace(t *testing.T) {
	got := normalizeText("hello   world\t\nfoo")
	if got != "hello world foo" {
		t.Errorf("normalizeText = %q, want %q", got, "hello world foo")
	}
}

func TestNormalizeText_Trim(t *testing.T) {
	got := normalizeText("  hello  ")
	if got != "hello" {
		t.Errorf("normalizeText = %q, want %q", got, "hello")
	}
}

// ---------------------------------------------------------------------------
// Tests: NERModelConfig validation
// ---------------------------------------------------------------------------

func TestNERModelConfig_Validate_Valid(t *testing.T) {
	cfg := DefaultNERModelConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNERModelConfig_Validate_EmptyModelID(t *testing.T) {
	cfg := DefaultNERModelConfig()
	cfg.ModelID = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty model_id")
	}
}

func TestNERModelConfig_Validate_InvalidThreshold(t *testing.T) {
	cfg := DefaultNERModelConfig()
	cfg.ConfidenceThreshold = 1.5
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for threshold > 1")
	}
	cfg.ConfidenceThreshold = -0.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative threshold")
	}
}

func TestNERModelConfig_Validate_EmptyLabelSet(t *testing.T) {
	cfg := DefaultNERModelConfig()
	cfg.LabelSet = []string{}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty label_set")
	}
}

func TestNERModelConfig_Validate_InvalidMaxBatchSize(t *testing.T) {
	cfg := DefaultNERModelConfig()
	cfg.MaxBatchSize = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for zero max_batch_size")
	}
}

// ---------------------------------------------------------------------------
// Tests: NewNERModel validation
// ---------------------------------------------------------------------------

func TestNewNERModel_NilBackend(t *testing.T) {
	_, err := NewNERModel(nil, DefaultNERModelConfig(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil backend")
	}
}

func TestNewNERModel_NilConfig_UsesDefault(t *testing.T) {
	backend := &mockNERBackend{}
	model, err := NewNERModel(backend, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewNERModel: %v", err)
	}
	ls := model.GetLabelSet()
	if len(ls) != 15 {
		t.Errorf("expected 15 labels from default config, got %d", len(ls))
	}
}

// ---------------------------------------------------------------------------
// Tests: argmax decode (no CRF)
// ---------------------------------------------------------------------------

func TestArgmaxDecode_Simple(t *testing.T) {
	indexLabel := map[int]string{0: LabelO, 1: LabelBCOMMON, 2: LabelICOMMON}
	emission := [][]float64{
		{0.1, 0.8, 0.1},
		{0.1, 0.1, 0.8},
		{0.9, 0.05, 0.05},
	}
	labels := argmaxDecode(emission, indexLabel)
	if labels[0] != LabelBCOMMON {
		t.Errorf("labels[0] = %s, want B-COMMON", labels[0])
	}
	if labels[1] != LabelICOMMON {
		t.Errorf("labels[1] = %s, want I-COMMON", labels[1])
	}
	if labels[2] != LabelO {
		t.Errorf("labels[2] = %s, want O", labels[2])
	}
}

func TestArgmaxDecode_NoCRF_Integration(t *testing.T) {
	emFn := func(tokens []string) [][]float64 {
		labels := make([]string, len(tokens))
		for i := range labels {
			labels[i] = LabelO
		}
		for i, tok := range tokens {
			if strings.EqualFold(tok, "aspirin") {
				labels[i] = LabelBCOMMON
			}
		}
		return makeEmissionMatrix(labels, 0.90, 0.01)
	}
	model := newTestNERModelNoCRF(t, emFn)
	pred, err := model.Predict(context.Background(), "aspirin is used")
	if err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(pred.Entities) < 1 {
		t.Fatalf("expected at least 1 entity, got %d", len(pred.Entities))
	}
	assertEntity(t, pred.Entities[0], "aspirin", "COMMON")
}

// ---------------------------------------------------------------------------
// Tests: bioToEntities edge cases
// ---------------------------------------------------------------------------

func TestBioToEntities_ConsecutiveBTags(t *testing.T) {
	tokens := []string{"aspirin", "ibuprofen"}
	labels := []string{LabelBCOMMON, LabelBCOMMON}
	probs := makeEmissionMatrix(labels, 0.90, 0.01)
	spans := []tokenSpan{
		{Text: "aspirin", StartChar: 0, EndChar: 7},
		{Text: "ibuprofen", StartChar: 8, EndChar: 17},
	}
	entities := bioToEntities(tokens, labels, probs, spans)
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	assertEntity(t, entities[0], "aspirin", "COMMON")
	assertEntity(t, entities[1], "ibuprofen", "COMMON")
}

func TestBioToEntities_MixedTypes(t *testing.T) {
	tokens := []string{"aspirin", "CAS", "50-78-2"}
	labels := []string{LabelBCOMMON, LabelBCAS, LabelICAS}
	probs := makeEmissionMatrix(labels, 0.90, 0.01)
	spans := []tokenSpan{
		{Text: "aspirin", StartChar: 0, EndChar: 7},
		{Text: "CAS", StartChar: 8, EndChar: 11},
		{Text: "50-78-2", StartChar: 12, EndChar: 19},
	}
	entities := bioToEntities(tokens, labels, probs, spans)
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	assertEntity(t, entities[0], "aspirin", "COMMON")
	assertEntity(t, entities[1], "CAS 50-78-2", "CAS")
}

func TestBioToEntities_Empty(t *testing.T) {
	entities := bioToEntities([]string{}, []string{}, [][]float64{}, []tokenSpan{})
	if len(entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(entities))
	}
}

func TestBioToEntities_AllO(t *testing.T) {
	tokens := []string{"the", "method"}
	labels := []string{LabelO, LabelO}
	probs := makeEmissionMatrix(labels, 0.90, 0.01)
	spans := []tokenSpan{
		{Text: "the", StartChar: 0, EndChar: 3},
		{Text: "method", StartChar: 4, EndChar: 10},
	}
	entities := bioToEntities(tokens, labels, probs, spans)
	if len(entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(entities))
	}
}

// ---------------------------------------------------------------------------
// Tests: computeEntityConfidence edge cases
// ---------------------------------------------------------------------------

func TestComputeEntityConfidence_ZeroProb(t *testing.T) {
	probs := [][]float64{
		{0.0, 0.0, 0.0},
	}
	score := computeEntityConfidence(probs, 0, 1)
	if score != 0 {
		t.Errorf("score = %f, want 0 for zero probabilities", score)
	}
}

func TestComputeEntityConfidence_EmptyRange(t *testing.T) {
	probs := [][]float64{{0.5, 0.5}}
	score := computeEntityConfidence(probs, 0, 0)
	if score != 0 {
		t.Errorf("score = %f, want 0 for empty range", score)
	}
}

func TestComputeEntityConfidence_ThreeTokens(t *testing.T) {
	probs := [][]float64{
		{0.05, 0.90, 0.05},
		{0.05, 0.05, 0.90},
		{0.05, 0.05, 0.90},
	}
	score := computeEntityConfidence(probs, 0, 3)
	// Geometric mean of 0.90, 0.90, 0.90 = 0.90
	expected := math.Pow(0.90*0.90*0.90, 1.0/3.0)
	if math.Abs(score-expected) > 0.001 {
		t.Errorf("score = %f, want ~%f", score, expected)
	}
}

// ---------------------------------------------------------------------------
// Tests: isLegalBIOTransition
// ---------------------------------------------------------------------------

func TestIsLegalBIOTransition(t *testing.T) {
	tests := []struct {
		from, to string
		want     bool
	}{
		{"O", "O", true},
		{"O", "B-IUPAC", true},
		{"O", "I-IUPAC", false},
		{"B-IUPAC", "I-IUPAC", true},
		{"B-IUPAC", "I-CAS", false},
		{"B-IUPAC", "O", true},
		{"B-IUPAC", "B-CAS", true},
		{"I-IUPAC", "I-IUPAC", true},
		{"I-IUPAC", "I-CAS", false},
		{"I-IUPAC", "O", true},
		{"I-IUPAC", "B-CAS", true},
	}
	for _, tt := range tests {
		got := isLegalBIOTransition(tt.from, tt.to)
		if got != tt.want {
			t.Errorf("isLegalBIOTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: buildSlidingWindows
// ---------------------------------------------------------------------------

func TestBuildSlidingWindows_ExactFit(t *testing.T) {
	windows := buildSlidingWindows(256, 256)
	if len(windows) != 1 {
		t.Errorf("expected 1 window, got %d", len(windows))
	}
}

func TestBuildSlidingWindows_OneExtra(t *testing.T) {
	windows := buildSlidingWindows(257, 256)
	if len(windows) < 2 {
		t.Errorf("expected at least 2 windows, got %d", len(windows))
	}
	last := windows[len(windows)-1]
	if last[1] != 257 {
		t.Errorf("last window end = %d, want 257", last[1])
	}
}

func TestBuildSlidingWindows_VeryShort(t *testing.T) {
	windows := buildSlidingWindows(1, 256)
	if len(windows) != 1 {
		t.Errorf("expected 1 window, got %d", len(windows))
	}
	if windows[0][0] != 0 || windows[0][1] != 1 {
		t.Errorf("window = %v, want [0, 1)", windows[0])
	}
}

func TestBuildSlidingWindows_StepSize(t *testing.T) {
	// maxLen=10, step=5, numTokens=25
	windows := buildSlidingWindows(25, 10)
	// [0,10), [5,15), [10,20), [15,25) = 4 windows
	if len(windows) < 3 {
		t.Errorf("expected at least 3 windows for 25 tokens with maxLen 10, got %d", len(windows))
	}
	// All windows should be within bounds
	for i, w := range windows {
		if w[0] < 0 || w[1] > 25 {
			t.Errorf("window[%d] = %v out of bounds", i, w)
		}
	}
}

//Personal.AI order the ending

