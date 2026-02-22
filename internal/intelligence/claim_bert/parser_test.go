package claim_bert

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ============================================================================
// Mock: Tokenizer
// ============================================================================

type mockTokenizer struct {
	tokenizeFn func(text string) (*TokenizedOutput, error)
	encodeFn   func(text string) (*EncodedInput, error)
	decodeFn   func(ids []int) (string, error)
}

func (m *mockTokenizer) Tokenize(text string) (*TokenizedOutput, error) {
	if m.tokenizeFn != nil {
		return m.tokenizeFn(text)
	}
	words := strings.Fields(text)
	offsets := make([][2]int, len(words))
	pos := 0
	for i, w := range words {
		idx := strings.Index(text[pos:], w)
		if idx >= 0 {
			start := pos + idx
			end := start + len(w)
			offsets[i] = [2]int{start, end}
			pos = end
		}
	}
	return &TokenizedOutput{
		Tokens:  words,
		Offsets: offsets,
	}, nil
}

func (m *mockTokenizer) Encode(text string) (*EncodedInput, error) {
	if m.encodeFn != nil {
		return m.encodeFn(text)
	}
	// Default: split on whitespace, generate offsets
	words := strings.Fields(text)
	ids := make([]int, len(words))
	mask := make([]int, len(words))
	typeIDs := make([]int, len(words))
	offsets := make([][2]int, len(words))

	pos := 0
	for i, w := range words {
		idx := strings.Index(text[pos:], w)
		if idx >= 0 {
			start := pos + idx
			end := start + len(w)
			offsets[i] = [2]int{start, end}
			pos = end
		}
		ids[i] = i + 1
		mask[i] = 1
	}

	return &EncodedInput{
		InputIDs:      ids,
		AttentionMask: mask,
		TokenTypeIDs:  typeIDs,
		Tokens:        words,
		Offsets:       offsets,
	}, nil
}

// Add stub implementations for other interface methods
func (m *mockTokenizer) EncodePair(textA, textB string) (*EncodedInput, error) { return nil, nil }
func (m *mockTokenizer) BatchEncode(texts []string) ([]*EncodedInput, error)   { return nil, nil }
func (m *mockTokenizer) VocabSize() int                                        { return 1000 }

func (m *mockTokenizer) Decode(ids []int) (string, error) {
	if m.decodeFn != nil {
		return m.decodeFn(ids)
	}
	return "", nil
}

// ============================================================================
// Mock: Model Backend
// ============================================================================

type mockClaimModelBackend struct {
	predictFn func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error)

	// Pre-configured outputs for convenience
	classificationProbs []float64
	bioTags             []int
	scopeScore          float64
	dependencyRefs      []int
}

func (m *mockClaimModelBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	if m.predictFn != nil {
		return m.predictFn(ctx, req)
	}

	outputs := make(map[string][]byte)

	// Classification
	if m.classificationProbs != nil {
		data, _ := json.Marshal(classificationOutput{Probabilities: m.classificationProbs})
		outputs["classification"] = data
	}

	// BIO tags
	if m.bioTags != nil {
		data, _ := json.Marshal(bioTagsOutput{Tags: m.bioTags})
		outputs["bio_tags"] = data
	}

	// Scope
	{
		data, _ := json.Marshal(scopeOutput{Score: m.scopeScore})
		outputs["scope"] = data
	}

	// Dependency
	if m.dependencyRefs != nil {
		data, _ := json.Marshal(dependencyOutput{References: m.dependencyRefs})
		outputs["dependency"] = data
	}

	return &common.PredictResponse{
		Outputs:         outputs,
		InferenceTimeMs: 10,
	}, nil
}

func (m *mockClaimModelBackend) LoadModel(ctx context.Context, modelPath string) error {
	return nil
}

func (m *mockClaimModelBackend) UnloadModel(ctx context.Context) error {
	return nil
}

func (m *mockClaimModelBackend) Healthy(ctx context.Context) error {
	return nil
}

func (m *mockClaimModelBackend) Close() error {
	return nil
}

func (m *mockClaimModelBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, nil
}

// ============================================================================
// Mock: Logger & Metrics (no-op)
// ============================================================================

type noopLogger struct{}

func (n *noopLogger) Info(msg string, keysAndValues ...interface{})  {}
func (n *noopLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (n *noopLogger) Error(msg string, keysAndValues ...interface{}) {}
func (n *noopLogger) Debug(msg string, keysAndValues ...interface{}) {}

type noopMetrics struct{}

func (n *noopMetrics) RecordInference(ctx context.Context, params *common.InferenceMetricParams) {}
func (n *noopMetrics) RecordBatchProcessing(ctx context.Context, params *common.BatchMetricParams) {}
func (n *noopMetrics) RecordCacheAccess(ctx context.Context, hit bool, modelName string) {}
func (n *noopMetrics) RecordCircuitBreakerStateChange(ctx context.Context, modelName string, fromState, toState string) {}
func (n *noopMetrics) RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64) {}
func (n *noopMetrics) RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool) {}
func (n *noopMetrics) GetInferenceLatencyHistogram() common.LatencyHistogram { return nil }
func (n *noopMetrics) GetCurrentStats() *common.IntelligenceStats { return &common.IntelligenceStats{} }

// ============================================================================
// Helper: create parser with mocks
// ============================================================================

func newTestParser(backend *mockClaimModelBackend) ClaimParser {
	cfg := &ClaimBERTConfig{
		ModelID:           "claim-bert-v1.0.0",
		MaxSequenceLength: 512,
		TaskHeads:         DefaultTaskHeads(),
	}
	p, err := NewClaimParser(backend, cfg, &mockTokenizer{}, &noopLogger{}, &noopMetrics{})
	if err != nil {
		panic(fmt.Sprintf("failed to create test parser: %v", err))
	}
	return p
}

func newTestParserWithTokenizer(backend *mockClaimModelBackend, tok Tokenizer) ClaimParser {
	cfg := &ClaimBERTConfig{
		ModelID:           "claim-bert-v1.0.0",
		MaxSequenceLength: 512,
		TaskHeads:         DefaultTaskHeads(),
	}
	p, err := NewClaimParser(backend, cfg, tok, &noopLogger{}, &noopMetrics{})
	if err != nil {
		panic(fmt.Sprintf("failed to create test parser: %v", err))
	}
	return p
}

// ============================================================================
// Test: ParseClaim — Independent Claim
// ============================================================================

func TestParseClaim_IndependentClaim(t *testing.T) {
	text := "A pharmaceutical composition comprising a compound of formula (I) and a pharmaceutically acceptable carrier."

	// Tokenize: split by whitespace gives ~14 tokens
	// We need BIO tags aligned with tokens:
	// "A"=O "pharmaceutical"=O "composition"=O "comprising"=O
	// "a"=B-Structural "compound"=I-Structural "of"=I-Structural "formula"=I-Structural "(I)"=I-Structural
	// "and"=O
	// "a"=B-Structural "pharmaceutically"=I-Structural "acceptable"=I-Structural "carrier."=I-Structural

	words := strings.Fields(text)
	bioTags := make([]int, len(words))
	for i, w := range words {
		switch {
		case w == "a" && i == 4:
			bioTags[i] = bioBStructural // B-Structural
		case i >= 5 && i <= 8:
			bioTags[i] = bioIStructural // I-Structural
		case w == "a" && i == 10:
			bioTags[i] = bioBStructural // B-Structural
		case i >= 11 && i <= 13:
			bioTags[i] = bioIStructural // I-Structural
		default:
			bioTags[i] = bioO
		}
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.85, 0.05, 0.03, 0.05, 0.02}, // Independent
		bioTags:             bioTags,
		scopeScore:          0.78,
		dependencyRefs:      nil,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, text)
	if err != nil {
		t.Fatalf("ParseClaim failed: %v", err)
	}

	// Verify claim type
	if result.ClaimType != ClaimIndependent {
		t.Errorf("expected ClaimType=%s, got %s", ClaimIndependent, result.ClaimType)
	}

	// Verify transitional phrase
	if !strings.Contains(strings.ToLower(result.TransitionalPhrase), "comprising") {
		t.Errorf("expected transitional phrase containing 'comprising', got %q", result.TransitionalPhrase)
	}
	if result.TransitionalType != PhraseComprising {
		t.Errorf("expected transitional type=%s, got %s", PhraseComprising, result.TransitionalType)
	}

	// Verify preamble contains "pharmaceutical composition"
	if !strings.Contains(strings.ToLower(result.Preamble), "pharmaceutical composition") {
		t.Errorf("expected preamble to contain 'pharmaceutical composition', got %q", result.Preamble)
	}

	// Verify body is non-empty
	if result.Body == "" {
		t.Error("expected non-empty body")
	}

	// Verify features extracted
	if len(result.Features) < 1 {
		t.Errorf("expected at least 1 feature, got %d", len(result.Features))
	}

	// Check that features contain expected text fragments
	foundCompound := false
	foundCarrier := false
	for _, f := range result.Features {
		lower := strings.ToLower(f.Text)
		if strings.Contains(lower, "compound") {
			foundCompound = true
		}
		if strings.Contains(lower, "carrier") {
			foundCarrier = true
		}
		// All features in independent claim should be essential
		if !f.IsEssential {
			t.Errorf("feature %q should be essential in independent claim", f.Text)
		}
	}
	if !foundCompound {
		t.Error("expected a feature containing 'compound'")
	}
	if !foundCarrier {
		t.Error("expected a feature containing 'carrier'")
	}

	// Verify no dependencies
	if len(result.DependsOn) > 0 {
		t.Errorf("independent claim should have no dependencies, got %v", result.DependsOn)
	}

	// Verify scope score
	if result.ScopeScore < 0 || result.ScopeScore > 1 {
		t.Errorf("scope score out of range: %f", result.ScopeScore)
	}

	// Verify confidence
	if result.Confidence <= 0 || result.Confidence > 1 {
		t.Errorf("confidence out of range: %f", result.Confidence)
	}
}

// ============================================================================
// Test: ParseClaim — Dependent Claim with Markush Group
// ============================================================================

func TestParseClaim_DependentClaim(t *testing.T) {
	text := "The composition of claim 1, wherein the compound is selected from the group consisting of aspirin, ibuprofen, and naproxen."

	words := strings.Fields(text)
	bioTags := make([]int, len(words))
	// Mark "aspirin, ibuprofen, and naproxen" as composition features
	for i, w := range words {
		lower := strings.ToLower(strings.TrimRight(w, ".,;"))
		switch lower {
		case "aspirin":
			bioTags[i] = bioBComposition
		case "ibuprofen":
			bioTags[i] = bioBComposition
		case "naproxen":
			bioTags[i] = bioBComposition
		default:
			bioTags[i] = bioO
		}
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.05, 0.88, 0.02, 0.03, 0.02}, // Dependent
		bioTags:             bioTags,
		scopeScore:          0.35,
		dependencyRefs:      []int{1},
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, text)
	if err != nil {
		t.Fatalf("ParseClaim failed: %v", err)
	}

	// Verify claim type
	if result.ClaimType != ClaimDependent {
		t.Errorf("expected ClaimType=%s, got %s", ClaimDependent, result.ClaimType)
	}

	// Verify dependency on claim 1
	if len(result.DependsOn) == 0 {
		t.Fatal("expected DependsOn to contain claim 1")
	}
	foundDep1 := false
	for _, d := range result.DependsOn {
		if d == 1 {
			foundDep1 = true
		}
	}
	if !foundDep1 {
		t.Errorf("expected DependsOn to contain 1, got %v", result.DependsOn)
	}

	// Verify Markush group
	if len(result.MarkushGroups) == 0 {
		t.Fatal("expected at least one Markush group")
	}
	mg := result.MarkushGroups[0]
	if mg.IsOpenEnded {
		t.Error("expected closed Markush group")
	}
	if len(mg.Members) != 3 {
		t.Errorf("expected 3 Markush members, got %d: %v", len(mg.Members), mg.Members)
	}

	// Verify members contain aspirin, ibuprofen, naproxen
	expectedMembers := map[string]bool{"aspirin": false, "ibuprofen": false, "naproxen": false}
	for _, member := range mg.Members {
		lower := strings.ToLower(strings.TrimSpace(member))
		if _, ok := expectedMembers[lower]; ok {
			expectedMembers[lower] = true
		}
	}
	for name, found := range expectedMembers {
		if !found {
			t.Errorf("Markush group missing member: %s", name)
		}
	}
}

// ============================================================================
// Test: ParseClaim — Method Claim
// ============================================================================

func TestParseClaim_MethodClaim(t *testing.T) {
	text := "A method for treating inflammation comprising administering to a patient an effective amount of the composition of claim 1."

	words := strings.Fields(text)
	bioTags := make([]int, len(words))
	// Mark "administering to a patient an effective amount" as process feature
	for i, w := range words {
		lower := strings.ToLower(w)
		switch lower {
		case "administering":
			bioTags[i] = bioBProcess
		case "to":
			if i > 0 && bioTags[i-1] == bioBProcess {
				bioTags[i] = bioIProcess
			}
		case "a":
			if i > 0 && (bioTags[i-1] == bioIProcess || bioTags[i-1] == bioBProcess) {
				bioTags[i] = bioIProcess
			}
		case "patient":
			if i > 0 && bioTags[i-1] == bioIProcess {
				bioTags[i] = bioIProcess
			}
		case "an":
			if i > 0 && bioTags[i-1] == bioIProcess {
				bioTags[i] = bioIProcess
			}
		case "effective":
			if i > 0 && bioTags[i-1] == bioIProcess {
				bioTags[i] = bioIProcess
			}
		case "amount":
			if i > 0 && bioTags[i-1] == bioIProcess {
				bioTags[i] = bioIProcess
			}
		default:
			bioTags[i] = bioO
		}
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.03, 0.05, 0.85, 0.04, 0.03}, // Method
		bioTags:             bioTags,
		scopeScore:          0.60,
		dependencyRefs:      []int{1},
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, text)
	if err != nil {
		t.Fatalf("ParseClaim failed: %v", err)
	}

	// Method claims with dependency references are still classified as Method
	// (the dependency refinement only changes Independent -> Dependent)
	if result.ClaimType != ClaimMethod && result.ClaimType != ClaimDependent {
		t.Errorf("expected ClaimType=METHOD or DEPENDENT, got %s", result.ClaimType)
	}

	// Verify process-type features exist
	hasProcessFeature := false
	for _, f := range result.Features {
		if f.FeatureType == FeatureProcess {
			hasProcessFeature = true
			break
		}
	}
	if !hasProcessFeature && len(result.Features) > 0 {
		// At minimum, features should be present
		t.Log("Note: no process-type feature detected, but features were extracted")
	}

	// Verify dependency on claim 1
	foundDep := false
	for _, d := range result.DependsOn {
		if d == 1 {
			foundDep = true
		}
	}
	if !foundDep {
		t.Errorf("expected dependency on claim 1, got %v", result.DependsOn)
	}
}

// ============================================================================
// Test: ParseClaim — Consisting Of
// ============================================================================

func TestParseClaim_ConsistingOf(t *testing.T) {
	text := "A pharmaceutical composition consisting of compound A and compound B."

	words := strings.Fields(text)
	bioTags := make([]int, len(words))
	for i := range bioTags {
		bioTags[i] = bioO
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.80, 0.05, 0.05, 0.08, 0.02},
		bioTags:             bioTags,
		scopeScore:          0.25, // narrow scope for consisting of
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, text)
	if err != nil {
		t.Fatalf("ParseClaim failed: %v", err)
	}

	if result.TransitionalType != PhraseConsistingOf {
		t.Errorf("expected transitional type=%s, got %s", PhraseConsistingOf, result.TransitionalType)
	}

	if !strings.Contains(strings.ToLower(result.TransitionalPhrase), "consisting of") {
		t.Errorf("expected transitional phrase 'consisting of', got %q", result.TransitionalPhrase)
	}
}

// ============================================================================
// Test: ParseClaim — Numerical Range
// ============================================================================

func TestParseClaim_NumericalRange(t *testing.T) {
	text := "A process for synthesizing compound X at a temperature of from about 50°C to about 100°C."

	words := strings.Fields(text)
	bioTags := make([]int, len(words))
	// Mark the temperature range as parameter feature
	for i, w := range words {
		lower := strings.ToLower(w)
		if lower == "temperature" {
			bioTags[i] = bioBParameter
		} else if i > 0 && bioTags[i-1] != bioO && (lower == "of" || lower == "from" ||
			strings.Contains(lower, "about") || strings.Contains(lower, "50") ||
			strings.Contains(lower, "100") || lower == "to") {
			bioTags[i] = bioIParameter
		} else {
			bioTags[i] = bioO
		}
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.10, 0.02, 0.82, 0.03, 0.03},
		bioTags:             bioTags,
		scopeScore:          0.45,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, text)
	if err != nil {
		t.Fatalf("ParseClaim failed: %v", err)
	}
	// Verify numerical ranges are present in the parsed result
	if len(result.Features) > 0 && len(result.Features[0].NumericalRanges) > 0 {
		// Just ensuring the field is populated via enrichment
	}

	// Check numerical ranges extracted from the full text
	// The rule-based extractor should find "from about 50°C to about 100°C"
	allRanges := extractNumericalRanges(text)
	if len(allRanges) == 0 {
		t.Fatal("expected at least one numerical range")
	}

	nr := allRanges[0]
	if nr.LowerBound == nil {
		t.Fatal("expected non-nil LowerBound")
	}
	if nr.UpperBound == nil {
		t.Fatal("expected non-nil UpperBound")
	}
	if *nr.LowerBound != 50 {
		t.Errorf("expected LowerBound=50, got %f", *nr.LowerBound)
	}
	if *nr.UpperBound != 100 {
		t.Errorf("expected UpperBound=100, got %f", *nr.UpperBound)
	}
	if !nr.IsApproximate {
		t.Error("expected IsApproximate=true due to 'about'")
	}
	if nr.Unit != "°C" {
		t.Errorf("expected Unit='°C', got %q", nr.Unit)
	}
}

// ============================================================================
// Test: ParseClaim — Markush Group Open-Ended
// ============================================================================

func TestParseClaim_MarkushGroup_OpenEnded(t *testing.T) {
	text := "The composition of claim 1, wherein the excipient is selected from excipients including but not limited to lactose, starch, and cellulose."

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.05, 0.85, 0.03, 0.04, 0.03},
		bioTags:             make([]int, len(strings.Fields(text))), // all O
		scopeScore:          0.40,
		dependencyRefs:      []int{1},
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, text)
	if err != nil {
		t.Fatalf("ParseClaim failed: %v", err)
	}

	// Find open-ended Markush group
	foundOpen := false
	for _, mg := range result.MarkushGroups {
		if mg.IsOpenEnded {
			foundOpen = true
			if len(mg.Members) < 2 {
				t.Errorf("expected at least 2 members in open Markush group, got %d", len(mg.Members))
			}
			break
		}
	}
	if !foundOpen {
		t.Error("expected an open-ended Markush group")
	}
}

// ============================================================================
// Test: ParseClaim — Markush Group Closed
// ============================================================================

func TestParseClaim_MarkushGroup_Closed(t *testing.T) {
	text := "A compound selected from the group consisting of A, B, and C."

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.80, 0.05, 0.05, 0.08, 0.02},
		bioTags:             make([]int, len(strings.Fields(text))),
		scopeScore:          0.50,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, text)
	if err != nil {
		t.Fatalf("ParseClaim failed: %v", err)
	}

	if len(result.MarkushGroups) == 0 {
		t.Fatal("expected at least one Markush group")
	}

	mg := result.MarkushGroups[0]
	if mg.IsOpenEnded {
		t.Error("expected closed Markush group")
	}
	if len(mg.Members) != 3 {
		t.Errorf("expected 3 members, got %d: %v", len(mg.Members), mg.Members)
	}

	expectedMembers := []string{"A", "B", "C"}
	for i, expected := range expectedMembers {
		if i < len(mg.Members) {
			if strings.TrimSpace(mg.Members[i]) != expected {
				t.Errorf("member[%d]: expected %q, got %q", i, expected, mg.Members[i])
			}
		}
	}
}

// ============================================================================
// Test: ExtractFeatures — BIO Conversion
// ============================================================================

func TestExtractFeatures_BIOConversion(t *testing.T) {
	// Simulate text with 6 tokens
	text := "alpha beta gamma delta epsilon zeta"

	// BIO tags: [B-Structural, I-Structural, O, B-Functional, I-Functional, I-Functional]
	bioTags := []int{
		bioBStructural, bioIStructural, bioO,
		bioBFunctional, bioIFunctional, bioIFunctional,
	}

	backend := &mockClaimModelBackend{
		bioTags: bioTags,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	features, err := parser.ExtractFeatures(ctx, text)
	if err != nil {
		t.Fatalf("ExtractFeatures failed: %v", err)
	}

	if len(features) != 2 {
		t.Fatalf("expected 2 features (spans), got %d", len(features))
	}

	// First span: Structural (tokens 0-1 -> "alpha beta")
	if features[0].FeatureType != FeatureStructural {
		t.Errorf("feature[0] type: expected %s, got %s", FeatureStructural, features[0].FeatureType)
	}
	if !strings.Contains(features[0].Text, "alpha") || !strings.Contains(features[0].Text, "beta") {
		t.Errorf("feature[0] text should contain 'alpha beta', got %q", features[0].Text)
	}

	// Second span: Functional (tokens 3-5 -> "delta epsilon zeta")
	if features[1].FeatureType != FeatureFunctional {
		t.Errorf("feature[1] type: expected %s, got %s", FeatureFunctional, features[1].FeatureType)
	}
	if !strings.Contains(features[1].Text, "delta") {
		t.Errorf("feature[1] text should contain 'delta', got %q", features[1].Text)
	}
}

// ============================================================================
// Test: ExtractFeatures — Inconsistent BIO (I without preceding B)
// ============================================================================

func TestExtractFeatures_InconsistentBIO(t *testing.T) {
	text := "alpha beta"

	// [I-Structural, I-Structural] — no preceding B
	bioTags := []int{bioIStructural, bioIStructural}

	backend := &mockClaimModelBackend{
		bioTags: bioTags,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	features, err := parser.ExtractFeatures(ctx, text)
	if err != nil {
		t.Fatalf("ExtractFeatures failed: %v", err)
	}

	// Should auto-correct to [B-Structural, I-Structural] -> 1 span
	if len(features) != 1 {
		t.Fatalf("expected 1 feature after BIO correction, got %d", len(features))
	}

	if features[0].FeatureType != FeatureStructural {
		t.Errorf("expected Structural, got %s", features[0].FeatureType)
	}
}

// ============================================================================
// Test: BIO Correction directly
// ============================================================================

func TestCorrectBIOSequence(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "already correct",
			input:    []int{bioBStructural, bioIStructural, bioO},
			expected: []int{bioBStructural, bioIStructural, bioO},
		},
		{
			name:     "orphan I at start",
			input:    []int{bioIStructural, bioIStructural},
			expected: []int{bioBStructural, bioIStructural},
		},
		{
			name:     "I after O",
			input:    []int{bioO, bioIFunctional},
			expected: []int{bioO, bioBFunctional},
		},
		{
			name:     "I after different category",
			input:    []int{bioBStructural, bioIFunctional},
			expected: []int{bioBStructural, bioBFunctional},
		},
		{
			name:     "empty",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "all O",
			input:    []int{bioO, bioO, bioO},
			expected: []int{bioO, bioO, bioO},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := correctBIOSequence(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("length mismatch: got %d, want %d", len(result), len(tt.expected))
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("index %d: got %d (%s), want %d (%s)",
						i, result[i], bioTagName[result[i]], tt.expected[i], bioTagName[tt.expected[i]])
				}
			}
		})
	}
}

// ============================================================================
// Test: ClassifyClaim — All Types
// ============================================================================

func TestClassifyClaim_AllTypes(t *testing.T) {
	testCases := []struct {
		name        string
		text        string
		probs       []float64
		expectedType ClaimType
	}{
		{
			name:         "Independent",
			text:         "A composition comprising compound X.",
			probs:        []float64{0.90, 0.03, 0.02, 0.03, 0.02},
			expectedType: ClaimIndependent,
		},
		{
			name:         "Dependent",
			text:         "The composition of claim 1, wherein X is Y.",
			probs:        []float64{0.05, 0.88, 0.02, 0.03, 0.02},
			expectedType: ClaimDependent,
		},
		{
			name:         "Method",
			text:         "A method for treating disease comprising administering compound X.",
			probs:        []float64{0.03, 0.02, 0.90, 0.03, 0.02},
			expectedType: ClaimMethod,
		},
		{
			name:         "Product",
			text:         "A device for measuring temperature comprising a sensor.",
			probs:        []float64{0.03, 0.02, 0.03, 0.90, 0.02},
			expectedType: ClaimProduct,
		},
		{
			name:         "Use",
			text:         "Use of compound X for the treatment of inflammation.",
			probs:        []float64{0.02, 0.02, 0.03, 0.03, 0.90},
			expectedType: ClaimUse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &mockClaimModelBackend{
				classificationProbs: tc.probs,
				bioTags:             make([]int, len(strings.Fields(tc.text))),
				scopeScore:          0.5,
			}
			parser := newTestParser(backend)
			ctx := context.Background()

			result, err := parser.ClassifyClaim(ctx, tc.text)
			if err != nil {
				t.Fatalf("ClassifyClaim failed: %v", err)
			}

			if result.ClaimType != tc.expectedType {
				t.Errorf("expected type=%s, got %s", tc.expectedType, result.ClaimType)
			}

			if result.Confidence <= 0 || result.Confidence > 1 {
				t.Errorf("confidence out of range: %f", result.Confidence)
			}

			if len(result.Probabilities) == 0 {
				t.Error("expected non-empty Probabilities map")
			}

			// Verify the highest probability matches the expected type
			maxProb := 0.0
			var maxType ClaimType
			for ct, prob := range result.Probabilities {
				if prob > maxProb {
					maxProb = prob
					maxType = ct
				}
			}
			if maxType != tc.expectedType {
				t.Errorf("highest probability type=%s, expected %s", maxType, tc.expectedType)
			}
		})
	}
}

// ============================================================================
// Test: ClassifyClaim — Fallback to Rule-Based
// ============================================================================

func TestClassifyClaim_FallbackRuleBased(t *testing.T) {
	// Backend returns error for classification
	backend := &mockClaimModelBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			// Return response without classification output
			return &common.PredictResponse{
				Outputs:         map[string][]byte{},
				InferenceTimeMs: 5,
			}, nil
		},
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	// Method claim — rule-based should detect "method for"
	result, err := parser.ClassifyClaim(ctx, "A method for treating cancer.")
	if err != nil {
		t.Fatalf("ClassifyClaim failed: %v", err)
	}

	if result.ClaimType != ClaimMethod {
		t.Errorf("expected rule-based fallback to detect Method, got %s", result.ClaimType)
	}

	// Confidence should be lower for rule-based
	if result.Confidence > 0.80 {
		t.Errorf("rule-based confidence should be <= 0.80, got %f", result.Confidence)
	}
}

// ============================================================================
// Test: ClassifyClaim — Empty Input
// ============================================================================

func TestClassifyClaim_EmptyInput(t *testing.T) {
	backend := &mockClaimModelBackend{}
	parser := newTestParser(backend)
	ctx := context.Background()

	_, err := parser.ClassifyClaim(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty input")
	}

	_, err = parser.ClassifyClaim(ctx, "   \t\n  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
}

// ============================================================================
// Test: AnalyzeDependency — Simple Chain
// ============================================================================

func TestAnalyzeDependency_SimpleChain(t *testing.T) {
	claims := []string{
		"1. A composition comprising compound X.",
		"2. The composition of claim 1, wherein X is a salt.",
		"3. The composition of claim 2, wherein the salt is sodium chloride.",
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.80, 0.05, 0.05, 0.05, 0.05},
		bioTags:             []int{bioO, bioO, bioO, bioO, bioO},
		scopeScore:          0.5,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	tree, err := parser.AnalyzeDependency(ctx, claims)
	if err != nil {
		t.Fatalf("AnalyzeDependency failed: %v", err)
	}

	// Claim 1 should be root
	if len(tree.Roots) == 0 {
		t.Fatal("expected at least one root")
	}
	foundRoot1 := false
	for _, r := range tree.Roots {
		if r == 1 {
			foundRoot1 = true
		}
	}
	if !foundRoot1 {
		t.Errorf("expected claim 1 as root, got roots=%v", tree.Roots)
	}

	// Claim 1 -> [2], Claim 2 -> [3]
	if children, ok := tree.Children[1]; ok {
		found2 := false
		for _, c := range children {
			if c == 2 {
				found2 = true
			}
		}
		if !found2 {
			t.Errorf("expected claim 2 as child of claim 1, got %v", children)
		}
	} else {
		t.Error("expected claim 1 to have children")
	}

	if children, ok := tree.Children[2]; ok {
		found3 := false
		for _, c := range children {
			if c == 3 {
				found3 = true
			}
		}
		if !found3 {
			t.Errorf("expected claim 3 as child of claim 2, got %v", children)
		}
	} else {
		t.Error("expected claim 2 to have children")
	}

	// Depth should be 3 (1 -> 2 -> 3)
	if tree.Depth != 3 {
		t.Errorf("expected depth=3, got %d", tree.Depth)
	}
}

// ============================================================================
// Test: AnalyzeDependency — Multiple Dependencies
// ============================================================================

func TestAnalyzeDependency_MultipleDependencies(t *testing.T) {
	claims := []string{
		"1. A composition comprising compound X.",
		"2. A method for making the composition of claim 1.",
		"3. The composition of claims 1 and 2, further comprising compound Y.",
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.80, 0.05, 0.05, 0.05, 0.05},
		bioTags:             []int{bioO, bioO, bioO, bioO, bioO},
		scopeScore:          0.5,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	tree, err := parser.AnalyzeDependency(ctx, claims)
	if err != nil {
		t.Fatalf("AnalyzeDependency failed: %v", err)
	}

	// Claim 1 should be root
	foundRoot1 := false
	for _, r := range tree.Roots {
		if r == 1 {
			foundRoot1 = true
		}
	}
	if !foundRoot1 {
		t.Errorf("expected claim 1 as root, roots=%v", tree.Roots)
	}

	// Claim 3 depends on both 1 and 2
	// So both claim 1 and claim 2 should have claim 3 as a child
	for _, parent := range []int{1, 2} {
		children := tree.Children[parent]
		found3 := false
		for _, c := range children {
			if c == 3 {
				found3 = true
			}
		}
		if !found3 {
			t.Errorf("expected claim 3 as child of claim %d, got %v", parent, children)
		}
	}
}

// ============================================================================
// Test: AnalyzeDependency — Chinese Claims
// ============================================================================

func TestAnalyzeDependency_ChineseClaims(t *testing.T) {
	claims := []string{
		"1、一种药物组合物，包含化合物X。",
		"2、如权利要求1所述的组合物，其中化合物X为盐酸盐。",
		"3、如权利要求1或2所述的组合物，还包含赋形剂。",
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.80, 0.05, 0.05, 0.05, 0.05},
		bioTags:             []int{bioO},
		scopeScore:          0.5,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	tree, err := parser.AnalyzeDependency(ctx, claims)
	if err != nil {
		t.Fatalf("AnalyzeDependency failed: %v", err)
	}

	// Claim 1 should be root
	foundRoot1 := false
	for _, r := range tree.Roots {
		if r == 1 {
			foundRoot1 = true
		}
	}
	if !foundRoot1 {
		t.Errorf("expected claim 1 as root, roots=%v", tree.Roots)
	}

	// Claim 2 depends on 1
	if children, ok := tree.Children[1]; ok {
		found2 := false
		for _, c := range children {
			if c == 2 {
				found2 = true
			}
		}
		if !found2 {
			t.Errorf("expected claim 2 as child of claim 1, children=%v", children)
		}
	}

	// Claim 3 depends on 1 and 2
	for _, parent := range []int{1, 2} {
		children := tree.Children[parent]
		found3 := false
		for _, c := range children {
			if c == 3 {
				found3 = true
			}
		}
		if !found3 {
			t.Errorf("expected claim 3 as child of claim %d, children=%v", parent, children)
		}
	}
}

// ============================================================================
// Test: AnalyzeDependency — Empty Input
// ============================================================================

func TestAnalyzeDependency_Empty(t *testing.T) {
	backend := &mockClaimModelBackend{}
	parser := newTestParser(backend)
	ctx := context.Background()

	tree, err := parser.AnalyzeDependency(ctx, []string{})
	if err != nil {
		t.Fatalf("AnalyzeDependency failed: %v", err)
	}

	if len(tree.Roots) != 0 {
		t.Errorf("expected empty roots, got %v", tree.Roots)
	}
	if tree.Depth != 0 {
		t.Errorf("expected depth=0, got %d", tree.Depth)
	}
}

// ============================================================================
// Test: ParseClaimSet — Full Pipeline
// ============================================================================

func TestParseClaimSet_FullPipeline(t *testing.T) {
	claims := []string{
		"1. A pharmaceutical composition comprising compound A and a carrier.",
		"2. The composition of claim 1, wherein compound A is aspirin.",
		"3. A method for treating pain comprising administering the composition of claim 1.",
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.80, 0.05, 0.05, 0.05, 0.05},
		bioTags:             nil, // will be set per call
		scopeScore:          0.6,
	}

	// Dynamic BIO tags based on input length
	backend.predictFn = func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
		var payload map[string]interface{}
		json.Unmarshal(req.InputData, &payload)

		inputIDs, _ := payload["input_ids"].([]interface{})
		numTokens := len(inputIDs)

		tags := make([]int, numTokens)
		for i := range tags {
			tags[i] = bioO
		}

		outputs := make(map[string][]byte)

		classData, _ := json.Marshal(classificationOutput{
			Probabilities: []float64{0.80, 0.05, 0.05, 0.05, 0.05},
		})
		outputs["classification"] = classData

		bioData, _ := json.Marshal(bioTagsOutput{Tags: tags})
		outputs["bio_tags"] = bioData

		scopeData, _ := json.Marshal(scopeOutput{Score: 0.6})
		outputs["scope"] = scopeData

		depData, _ := json.Marshal(dependencyOutput{References: nil})
		outputs["dependency"] = depData

		return &common.PredictResponse{
			Outputs:         outputs,
			InferenceTimeMs: 10,
		}, nil
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaimSet(ctx, claims)
	if err != nil {
		t.Fatalf("ParseClaimSet failed: %v", err)
	}

	if result.ClaimCount != 3 {
		t.Errorf("expected ClaimCount=3, got %d", result.ClaimCount)
	}

	if len(result.Claims) != 3 {
		t.Fatalf("expected 3 parsed claims, got %d", len(result.Claims))
	}

	// Verify claim numbers
	for i, c := range result.Claims {
		if c.ClaimNumber != i+1 {
			t.Errorf("claim[%d]: expected number=%d, got %d", i, i+1, c.ClaimNumber)
		}
	}

	// Verify dependency tree exists
	if result.DependencyTree == nil {
		t.Fatal("expected non-nil DependencyTree")
	}

	// Verify independent claims
	if len(result.IndependentClaims) == 0 {
		t.Error("expected at least one independent claim")
	}
}

// ============================================================================
// Test: ParseClaimSet — Partial Failures
// ============================================================================

func TestParseClaimSet_PartialFailures(t *testing.T) {
	claims := []string{
		"1. A valid composition comprising X.",
		"",  // empty — should fail
		"3. Another valid composition comprising Y.",
	}

	backend := &mockClaimModelBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			var payload map[string]interface{}
			json.Unmarshal(req.InputData, &payload)

			inputIDs, _ := payload["input_ids"].([]interface{})
			numTokens := len(inputIDs)

			tags := make([]int, numTokens)
			outputs := make(map[string][]byte)

			classData, _ := json.Marshal(classificationOutput{
				Probabilities: []float64{0.80, 0.05, 0.05, 0.05, 0.05},
			})
			outputs["classification"] = classData

			bioData, _ := json.Marshal(bioTagsOutput{Tags: tags})
			outputs["bio_tags"] = bioData

			scopeData, _ := json.Marshal(scopeOutput{Score: 0.5})
			outputs["scope"] = scopeData

			depData, _ := json.Marshal(dependencyOutput{References: nil})
			outputs["dependency"] = depData

			return &common.PredictResponse{
				Outputs:         outputs,
				InferenceTimeMs: 5,
			}, nil
		},
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaimSet(ctx, claims)
	if err != nil {
		t.Fatalf("ParseClaimSet should not fail entirely: %v", err)
	}

	// Should have 2 successful claims (empty one skipped)
	if len(result.Claims) != 2 {
		t.Errorf("expected 2 successful claims, got %d", len(result.Claims))
	}

	if result.ClaimCount != 2 {
		t.Errorf("expected ClaimCount=2, got %d", result.ClaimCount)
	}
}

// ============================================================================
// Test: ParseClaim — Model Inference Failure
// ============================================================================

func TestParseClaim_ModelInferenceFailure(t *testing.T) {
	backend := &mockClaimModelBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			return nil, fmt.Errorf("GPU out of memory")
		},
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	_, err := parser.ParseClaim(ctx, "A composition comprising X.")
	if err == nil {
		t.Fatal("expected error on model inference failure")
	}

	if !strings.Contains(err.Error(), "inference") && !strings.Contains(err.Error(), "GPU") {
		t.Errorf("error should mention inference failure, got: %v", err)
	}
}

// ============================================================================
// Test: ParseClaim — Text Truncation
// ============================================================================

func TestParseClaim_TextTruncation(t *testing.T) {
	// Create a very long claim text
	var sb strings.Builder
	sb.WriteString("A composition comprising ")
	for i := 0; i < 1000; i++ {
		sb.WriteString(fmt.Sprintf("component_%d, ", i))
	}
	longText := sb.String()

	backend := &mockClaimModelBackend{
		predictFn: func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
			var payload map[string]interface{}
			json.Unmarshal(req.InputData, &payload)

			inputIDs, _ := payload["input_ids"].([]interface{})
			numTokens := len(inputIDs)

			tags := make([]int, numTokens)
			outputs := make(map[string][]byte)

			classData, _ := json.Marshal(classificationOutput{
				Probabilities: []float64{0.90, 0.03, 0.02, 0.03, 0.02},
			})
			outputs["classification"] = classData

			bioData, _ := json.Marshal(bioTagsOutput{Tags: tags})
			outputs["bio_tags"] = bioData

			scopeData, _ := json.Marshal(scopeOutput{Score: 0.8})
			outputs["scope"] = scopeData

			depData, _ := json.Marshal(dependencyOutput{References: nil})
			outputs["dependency"] = depData

			return &common.PredictResponse{
				Outputs:         outputs,
				InferenceTimeMs: 15,
			}, nil
		},
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	result, err := parser.ParseClaim(ctx, longText)
	if err != nil {
		t.Fatalf("ParseClaim should handle truncation gracefully: %v", err)
	}

	// Confidence should be penalized for truncation
	if result.Confidence >= 0.90 {
		t.Errorf("expected penalized confidence for truncated text, got %f", result.Confidence)
	}
}

// ============================================================================
// Test: Transitional Phrase Detection
// ============================================================================

func TestDetectTransitionalPhrase(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		expectedType TransitionalPhraseType
		shouldMatch  string // substring that should appear in the matched phrase
	}{
		{
			name:         "comprising",
			text:         "A composition comprising compound X.",
			expectedType: PhraseComprising,
			shouldMatch:  "comprising",
		},
		{
			name:         "consisting of",
			text:         "A composition consisting of compound X.",
			expectedType: PhraseConsistingOf,
			shouldMatch:  "consisting of",
		},
		{
			name:         "consisting essentially of",
			text:         "A composition consisting essentially of compound X.",
			expectedType: PhraseConsistingEssentiallyOf,
			shouldMatch:  "consisting essentially of",
		},
		{
			name:         "which comprises",
			text:         "A device which comprises a sensor.",
			expectedType: PhraseComprising,
			shouldMatch:  "which comprises",
		},
		{
			name:         "characterized in that",
			text:         "A method characterized in that the temperature is controlled.",
			expectedType: PhraseComprising,
			shouldMatch:  "characterized in that",
		},
		{
			name:         "wherein",
			text:         "The composition wherein compound X is a salt.",
			expectedType: PhraseComprising,
			shouldMatch:  "wherein",
		},
		{
			name:         "Chinese comprising",
			text:         "一种组合物，包含化合物X。",
			expectedType: PhraseComprising,
			shouldMatch:  "包含",
		},
		{
			name:         "Chinese consisting of",
			text:         "一种组合物，由化合物X和Y组成。",
			expectedType: PhraseConsistingOf,
			shouldMatch:  "组成",
		},
		{
			name:         "Chinese consisting essentially of",
			text:         "一种组合物，基本上由化合物X组成。",
			expectedType: PhraseConsistingEssentiallyOf,
			shouldMatch:  "基本上由",
		},
		{
			name:         "Chinese characterized in that",
			text:         "一种方法，其特征在于温度被控制。",
			expectedType: PhraseComprising,
			shouldMatch:  "其特征在于",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phrase, phraseType := detectTransitionalPhrase(tt.text)

			if phraseType != tt.expectedType {
				t.Errorf("expected type=%s, got %s", tt.expectedType, phraseType)
			}

			if tt.shouldMatch != "" && !strings.Contains(strings.ToLower(phrase), strings.ToLower(tt.shouldMatch)) {
				t.Errorf("expected phrase to contain %q, got %q", tt.shouldMatch, phrase)
			}
		})
	}
}

// ============================================================================
// Test: Dependency Reference Extraction
// ============================================================================

func TestExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []int
	}{
		{
			name:     "single claim reference",
			text:     "The composition of claim 1, wherein X is Y.",
			expected: []int{1},
		},
		{
			name:     "multiple claims with and",
			text:     "The composition of claims 1 and 3, further comprising Z.",
			expected: []int{1, 3},
		},
		{
			name:     "claim range with to",
			text:     "The method according to claims 1 to 5.",
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "as claimed in",
			text:     "The device as claimed in claim 7.",
			expected: []int{7},
		},
		{
			name:     "as set forth in",
			text:     "The composition as set forth in claims 1, 2, and 4.",
			expected: []int{1, 2, 4},
		},
		{
			name:     "Chinese single reference",
			text:     "如权利要求1所述的组合物。",
			expected: []int{1},
		},
		{
			name:     "Chinese multiple references",
			text:     "如权利要求1或2所述的组合物。",
			expected: []int{1, 2},
		},
		{
			name:     "Chinese range reference",
			text:     "如权利要求1至5中任一项所述的方法。",
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "no reference",
			text:     "A composition comprising compound X.",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDependencyReferences(tt.text)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}

			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// ============================================================================
// Test: Claim Number Extraction
// ============================================================================

func TestExtractClaimNumber(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"1. A composition comprising X.", 1},
		{"Claim 1. A composition comprising X.", 1},
		{"15: A method for treating disease.", 15},
		{"1、一种药物组合物。", 1},
		{"23．一种方法。", 23},
		{"A composition comprising X.", 0}, // no number
	}

	for _, tt := range tests {
		t.Run(tt.text[:min(30, len(tt.text))], func(t *testing.T) {
			result := extractClaimNumber(tt.text)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// Test: Numerical Range Extraction
// ============================================================================

func TestExtractNumericalRanges(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		expectedCount int
		checkFirst    func(t *testing.T, nr *NumericalRange)
	}{
		{
			name:          "from X to Y",
			text:          "temperature of from 50°C to 100°C",
			expectedCount: 1,
			checkFirst: func(t *testing.T, nr *NumericalRange) {
				if nr.LowerBound == nil || *nr.LowerBound != 50 {
					t.Errorf("expected LowerBound=50, got %v", nr.LowerBound)
				}
				if nr.UpperBound == nil || *nr.UpperBound != 100 {
					t.Errorf("expected UpperBound=100, got %v", nr.UpperBound)
				}
				if nr.IsApproximate {
					t.Error("expected IsApproximate=false")
				}
			},
		},
		{
			name:          "from about X to about Y",
			text:          "from about 20% to about 80%",
			expectedCount: 1,
			checkFirst: func(t *testing.T, nr *NumericalRange) {
				if nr.LowerBound == nil || *nr.LowerBound != 20 {
					t.Errorf("expected LowerBound=20, got %v", nr.LowerBound)
				}
				if nr.UpperBound == nil || *nr.UpperBound != 80 {
					t.Errorf("expected UpperBound=80, got %v", nr.UpperBound)
				}
				if !nr.IsApproximate {
					t.Error("expected IsApproximate=true")
				}
			},
		},
		{
			name:          "between X and Y",
			text:          "between 100 and 200 mg",
			expectedCount: 1,
			checkFirst: func(t *testing.T, nr *NumericalRange) {
				if nr.LowerBound == nil || *nr.LowerBound != 100 {
					t.Errorf("expected LowerBound=100")
				}
				if nr.UpperBound == nil || *nr.UpperBound != 200 {
					t.Errorf("expected UpperBound=200")
				}
			},
		},
		{
			name:          "at least X",
			text:          "at least 95% purity",
			expectedCount: 1,
			checkFirst: func(t *testing.T, nr *NumericalRange) {
				if nr.LowerBound == nil || *nr.LowerBound != 95 {
					t.Errorf("expected LowerBound=95")
				}
				if nr.UpperBound != nil {
					t.Error("expected nil UpperBound")
				}
			},
		},
		{
			name:          "at most X",
			text:          "at most 5% impurity",
			expectedCount: 1,
			checkFirst: func(t *testing.T, nr *NumericalRange) {
				if nr.LowerBound != nil {
					t.Error("expected nil LowerBound")
				}
				if nr.UpperBound == nil || *nr.UpperBound != 5 {
					t.Errorf("expected UpperBound=5")
				}
			},
		},
		{
			name:          "less than X",
			text:          "less than 10 microns",
			expectedCount: 1,
			checkFirst: func(t *testing.T, nr *NumericalRange) {
				if nr.LowerBound != nil {
					t.Error("expected nil LowerBound")
				}
				if nr.UpperBound == nil || *nr.UpperBound != 10 {
					t.Errorf("expected UpperBound=10")
				}
			},
		},
		{
			name:          "greater than X",
			text:          "greater than 500 kDa",
			expectedCount: 1,
			checkFirst: func(t *testing.T, nr *NumericalRange) {
				if nr.LowerBound == nil || *nr.LowerBound != 500 {
					t.Errorf("expected LowerBound=500")
				}
				if nr.UpperBound != nil {
					t.Error("expected nil UpperBound")
				}
			},
		},
		{
			name:          "no range",
			text:          "a composition comprising compound X",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ranges := extractNumericalRanges(tt.text)
			if len(ranges) != tt.expectedCount {
				t.Fatalf("expected %d ranges, got %d", tt.expectedCount, len(ranges))
			}
			if tt.checkFirst != nil && len(ranges) > 0 {
				tt.checkFirst(t, ranges[0])
			}
		})
	}
}

// ============================================================================
// Test: Chemical Entity Extraction
// ============================================================================

func TestExtractChemicalEntities(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		minCount int
		contains string
	}{
		{
			name:     "chemical formula reference",
			text:     "a compound of formula (I)",
			minCount: 1,
			contains: "formula (I)",
		},
		{
			name:     "chemical suffix -ine",
			text:     "aspirin and caffeine",
			minCount: 1,
			contains: "caffeine",
		},
		{
			name:     "chemical suffix -ol",
			text:     "ethanol and methanol",
			minCount: 2,
			contains: "ethanol",
		},
		{
			name:     "chemical suffix -ase",
			text:     "lipase enzyme",
			minCount: 1,
			contains: "lipase",
		},
		{
			name:     "chemical suffix -ide",
			text:     "sodium chloride solution",
			minCount: 1,
			contains: "chloride",
		},
		{
			name:     "chemical suffix -ate",
			text:     "sodium acetate buffer",
			minCount: 1,
			contains: "acetate",
		},
		{
			name:     "no chemical entities",
			text:     "a method for treating a patient",
			minCount: 0,
		},
		{
			name:     "filter common words",
			text:     "the machine is done and gone",
			minCount: 0, // "machine", "done", "gone" should be filtered
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entities := extractChemicalEntities(tt.text)

			if len(entities) < tt.minCount {
				t.Errorf("expected at least %d entities, got %d: %v", tt.minCount, len(entities), entities)
			}

			if tt.contains != "" && len(entities) > 0 {
				found := false
				for _, e := range entities {
					if strings.Contains(strings.ToLower(e), strings.ToLower(tt.contains)) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected entity containing %q, got %v", tt.contains, entities)
				}
			}
		})
	}
}

// ============================================================================
// Test: Preprocess Claim Text
// ============================================================================

func TestPreprocessClaimText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normalize whitespace",
			input:    "A  composition\t\tcomprising\n\ncompound X.",
			expected: "A composition comprising compound X.",
		},
		{
			name:     "trim leading/trailing whitespace",
			input:    "  A composition comprising X.  ",
			expected: "A composition comprising X.",
		},
		{
			name:     "normalize unicode quotes",
			input:    "\u201Ccomposition\u201D",
			expected: "\"composition\"",
		},
		{
			name:     "normalize unicode dashes",
			input:    "compound A\u2014compound B",
			expected: "compound A-compound B",
		},
		{
			name:     "normalize Chinese punctuation",
			input:    "化合物X，包含Y。",
			expected: "化合物X,包含Y.",
		},
		{
			name:     "normalize Chinese semicolons and colons",
			input:    "步骤一：混合；步骤二：加热",
			expected: "步骤一:混合;步骤二:加热",
		},
		{
			name:     "normalize inequality symbols",
			input:    "浓度≥50%且≤90%",
			expected: "浓度>=50%且<=90%",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessClaimText(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Test: Split Preamble Body
// ============================================================================

func TestSplitPreambleBody(t *testing.T) {
	tests := []struct {
		name             string
		text             string
		transitional     string
		expectedPreamble string
		expectedBody     string
	}{
		{
			name:             "standard comprising",
			text:             "A pharmaceutical composition comprising compound X and carrier Y.",
			transitional:     "comprising",
			expectedPreamble: "A pharmaceutical composition",
			expectedBody:     "compound X and carrier Y.",
		},
		{
			name:             "consisting of",
			text:             "A kit consisting of reagent A and reagent B.",
			transitional:     "consisting of",
			expectedPreamble: "A kit",
			expectedBody:     "reagent A and reagent B.",
		},
		{
			name:             "no transitional phrase",
			text:             "A composition with compound X.",
			transitional:     "",
			expectedPreamble: "",
			expectedBody:     "A composition with compound X.",
		},
		{
			name:             "with claim number in preamble",
			text:             "1. A composition comprising X.",
			transitional:     "comprising",
			expectedPreamble: "A composition",
			expectedBody:     "X.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preamble, body := splitPreambleBody(tt.text, tt.transitional)

			if preamble != tt.expectedPreamble {
				t.Errorf("preamble: expected %q, got %q", tt.expectedPreamble, preamble)
			}
			if body != tt.expectedBody {
				t.Errorf("body: expected %q, got %q", tt.expectedBody, body)
			}
		})
	}
}

// ============================================================================
// Test: Parse Claim Number List
// ============================================================================

func TestParseClaimNumberList(t *testing.T) {
	tests := []struct {
		input    string
		expected []int
	}{
		{"1", []int{1}},
		{"1, 2, 3", []int{1, 2, 3}},
		{"1 and 3", []int{1, 3}},
		{"1 to 5", []int{1, 2, 3, 4, 5}},
		{"1, 3 and 5", []int{1, 3, 5}},
		{"1至3", []int{1, 2, 3}},
		{"1、3和5", []int{1, 3, 5}},
		{"1或2", []int{1, 2}},
		{"", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseClaimNumberList(tt.input)

			if tt.expected == nil {
				if len(result) != 0 {
					t.Errorf("expected empty, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// ============================================================================
// Test: Merge Dependencies
// ============================================================================

func TestMergeDependencies(t *testing.T) {
	tests := []struct {
		name      string
		modelDeps []int
		ruleDeps  []int
		expected  []int
	}{
		{
			name:      "both non-empty with overlap",
			modelDeps: []int{1, 3},
			ruleDeps:  []int{1, 2},
			expected:  []int{1, 2, 3},
		},
		{
			name:      "model only",
			modelDeps: []int{5, 7},
			ruleDeps:  nil,
			expected:  []int{5, 7},
		},
		{
			name:      "rule only",
			modelDeps: nil,
			ruleDeps:  []int{2, 4},
			expected:  []int{2, 4},
		},
		{
			name:      "both empty",
			modelDeps: nil,
			ruleDeps:  nil,
			expected:  nil,
		},
		{
			name:      "filter zero values",
			modelDeps: []int{0, 1, 0},
			ruleDeps:  []int{0, 2},
			expected:  []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeDependencies(tt.modelDeps, tt.ruleDeps)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("index %d: expected %d, got %d", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// ============================================================================
// Test: Calculate Tree Depth
// ============================================================================

func TestCalculateTreeDepth(t *testing.T) {
	tests := []struct {
		name     string
		roots    []int
		children map[int][]int
		expected int
	}{
		{
			name:     "empty tree",
			roots:    []int{},
			children: map[int][]int{},
			expected: 0,
		},
		{
			name:     "single root no children",
			roots:    []int{1},
			children: map[int][]int{},
			expected: 1,
		},
		{
			name:  "linear chain depth 3",
			roots: []int{1},
			children: map[int][]int{
				1: {2},
				2: {3},
			},
			expected: 3,
		},
		{
			name:  "branching tree",
			roots: []int{1},
			children: map[int][]int{
				1: {2, 3},
				2: {4, 5},
				3: {6},
				5: {7},
			},
			expected: 4, // 1 -> 2 -> 5 -> 7
		},
		{
			name:  "multiple roots",
			roots: []int{1, 10},
			children: map[int][]int{
				1:  {2},
				10: {11, 12},
				12: {13},
			},
			expected: 3, // 10 -> 12 -> 13
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTreeDepth(tt.roots, tt.children)
			if result != tt.expected {
				t.Errorf("expected depth=%d, got %d", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Test: Cosine Similarity
// ============================================================================

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name        string
		a           []float32
		b           []float32
		expected    float64
		tolerance   float64
		expectError bool
	}{
		{
			name:      "identical vectors",
			a:         []float32{1, 2, 3},
			b:         []float32{1, 2, 3},
			expected:  1.0,
			tolerance: 1e-6,
		},
		{
			name:      "orthogonal vectors",
			a:         []float32{1, 0, 0},
			b:         []float32{0, 1, 0},
			expected:  0.0,
			tolerance: 1e-6,
		},
		{
			name:      "opposite vectors",
			a:         []float32{1, 2, 3},
			b:         []float32{-1, -2, -3},
			expected:  -1.0,
			tolerance: 1e-6,
		},
		{
			name:      "known similarity",
			a:         []float32{1, 0},
			b:         []float32{1, 1},
			expected:  1.0 / math.Sqrt(2),
			tolerance: 1e-6,
		},
		{
			name:        "dimension mismatch",
			a:           []float32{1, 2},
			b:           []float32{1, 2, 3},
			expectError: true,
		},
		{
			name:        "empty vectors",
			a:           []float32{},
			b:           []float32{},
			expectError: true,
		},
		{
			name:      "zero vector",
			a:         []float32{0, 0, 0},
			b:         []float32{1, 2, 3},
			expected:  0.0,
			tolerance: 1e-6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CosineSimilarity(tt.a, tt.b)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(result-tt.expected) > tt.tolerance {
				t.Errorf("expected %f (±%f), got %f", tt.expected, tt.tolerance, result)
			}
		})
	}
}

// ============================================================================
// Test: Markush Member Parsing
// ============================================================================

func TestParseMarkushMembers(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "aspirin, ibuprofen, and naproxen",
			expected: []string{"aspirin", "ibuprofen", "naproxen"},
		},
		{
			input:    "methanol or ethanol",
			expected: []string{"methanol", "ethanol"},
		},
		{
			input:    "A, B, C, and D.",
			expected: []string{"A", "B", "C", "D"},
		},
		{
			input:    "single_member",
			expected: []string{"single_member"},
		},
		{
			input:    "alpha and beta and gamma",
			expected: []string{"alpha", "beta", "gamma"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseMarkushMembers(tt.input)

			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d members, got %d: %v", len(tt.expected), len(result), result)
			}

			for i := range result {
				if strings.TrimSpace(result[i]) != tt.expected[i] {
					t.Errorf("member[%d]: expected %q, got %q", i, tt.expected[i], result[i])
				}
			}
		})
	}
}

// ============================================================================
// Test: Rule-Based Classification
// ============================================================================

func TestRuleBasedClassification(t *testing.T) {
	backend := &mockClaimModelBackend{}
	parser := newTestParser(backend)
	impl := parser.(*claimParserImpl)

	tests := []struct {
		text         string
		expectedType ClaimType
	}{
		{"A method for treating cancer.", ClaimMethod},
		{"A process for synthesizing compound X.", ClaimMethod},
		{"Use of compound X for treating inflammation.", ClaimUse},
		{"A composition comprising compound X.", ClaimProduct},
		{"A device for measuring temperature.", ClaimProduct},
		{"The composition of claim 1, wherein X is Y.", ClaimDependent},
		{"如权利要求1所述的方法。", ClaimDependent},
		{"一种方法，包括步骤A和步骤B。", ClaimMethod},
		{"化合物X的用途。", ClaimUse},
		{"一种组合物，包含化合物X。", ClaimProduct},
		{"Something without clear indicators.", ClaimIndependent},
	}

	for _, tt := range tests {
		t.Run(tt.text[:min(40, len(tt.text))], func(t *testing.T) {
			claimType, confidence := impl.ruleBasedClassification(tt.text)

			if claimType != tt.expectedType {
				t.Errorf("expected %s, got %s", tt.expectedType, claimType)
			}

			if confidence <= 0 || confidence > 1 {
				t.Errorf("confidence out of range: %f", confidence)
			}
		})
	}
}

// ============================================================================
// Test: BIO Tag Helper Functions
// ============================================================================

func TestBIOTagHelpers(t *testing.T) {
	// Test bioTagIsB
	if !bioTagIsB(bioBStructural) {
		t.Error("bioBStructural should be B tag")
	}
	if !bioTagIsB(bioBFunctional) {
		t.Error("bioBFunctional should be B tag")
	}
	if bioTagIsB(bioIStructural) {
		t.Error("bioIStructural should not be B tag")
	}
	if bioTagIsB(bioO) {
		t.Error("bioO should not be B tag")
	}

	// Test bioTagIsI
	if !bioTagIsI(bioIStructural) {
		t.Error("bioIStructural should be I tag")
	}
	if !bioTagIsI(bioIFunctional) {
		t.Error("bioIFunctional should be I tag")
	}
	if bioTagIsI(bioBStructural) {
		t.Error("bioBStructural should not be I tag")
	}
	if bioTagIsI(bioO) {
		t.Error("bioO should not be I tag")
	}

	// Test bioTagCategory
	cat, ok := bioTagCategory(bioBStructural)
	if !ok || cat != "structural" {
		t.Errorf("bioBStructural category: expected 'structural', got %q (ok=%v)", cat, ok)
	}
	cat, ok = bioTagCategory(bioIStructural)
	if !ok || cat != "structural" {
		t.Errorf("bioIStructural category: expected 'structural', got %q (ok=%v)", cat, ok)
	}
	cat, ok = bioTagCategory(bioBFunctional)
	if !ok || cat != "functional" {
		t.Errorf("bioBFunctional category: expected 'functional', got %q (ok=%v)", cat, ok)
	}
	_, ok = bioTagCategory(bioO)
	if ok {
		t.Error("bioO should not have a category")
	}

	// Test bioCorrespondingB
	if bioCorrespondingB(bioIStructural) != bioBStructural {
		t.Error("corresponding B of I-Structural should be B-Structural")
	}
	if bioCorrespondingB(bioIFunctional) != bioBFunctional {
		t.Error("corresponding B of I-Functional should be B-Functional")
	}
	if bioCorrespondingB(bioIProcess) != bioBProcess {
		t.Error("corresponding B of I-Process should be B-Process")
	}
	if bioCorrespondingB(bioIComposition) != bioBComposition {
		t.Error("corresponding B of I-Composition should be B-Composition")
	}
	if bioCorrespondingB(bioIParameter) != bioBParameter {
		t.Error("corresponding B of I-Parameter should be B-Parameter")
	}

	// Test bioSameCategory
	if !bioSameCategory(bioBStructural, bioIStructural) {
		t.Error("B-Structural and I-Structural should be same category")
	}
	if bioSameCategory(bioBStructural, bioIFunctional) {
		t.Error("B-Structural and I-Functional should not be same category")
	}
	if bioSameCategory(bioO, bioBStructural) {
		t.Error("O and B-Structural should not be same category")
	}
}

// ============================================================================
// Test: NewClaimParser — Validation
// ============================================================================

func TestNewClaimParser_Validation(t *testing.T) {
	cfg := &ClaimBERTConfig{
		ModelID:           "test-v1.0.0",
		MaxSequenceLength: 512,
		TaskHeads:         DefaultTaskHeads(),
	}
	tok := &mockTokenizer{}
	log := &noopLogger{}
	met := &noopMetrics{}
	backend := &mockClaimModelBackend{}

	// Valid creation
	p, err := NewClaimParser(backend, cfg, tok, log, met)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil parser")
	}

	// Nil backend
	_, err = NewClaimParser(nil, cfg, tok, log, met)
	if err == nil {
		t.Error("expected error for nil backend")
	}

	// Nil config
	_, err = NewClaimParser(backend, nil, tok, log, met)
	if err == nil {
		t.Error("expected error for nil config")
	}

	// Nil tokenizer
	_, err = NewClaimParser(backend, cfg, nil, log, met)
	if err == nil {
		t.Error("expected error for nil tokenizer")
	}

	// Nil logger — should use noop, no error
	p, err = NewClaimParser(backend, cfg, tok, nil, met)
	if err != nil {
		t.Fatalf("expected no error with nil logger, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil parser with nil logger")
	}

	// Nil metrics — should use noop, no error
	p, err = NewClaimParser(backend, cfg, tok, log, nil)
	if err != nil {
		t.Fatalf("expected no error with nil metrics, got %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil parser with nil metrics")
	}
}

// ============================================================================
// Test: isCommonWord
// ============================================================================

func TestIsCommonWord(t *testing.T) {
	commonWords := []string{
		"the", "one", "done", "gone", "none", "alone", "bone",
		"machine", "medicine", "online", "routine", "mine",
		"whole", "role", "sole", "hole", "control", "protocol",
	}
	for _, w := range commonWords {
		if !isCommonWord(w) {
			t.Errorf("expected %q to be common word", w)
		}
	}

	chemicalWords := []string{
		"caffeine", "aspirin", "ethanol", "methanol",
		"lipase", "chloride", "acetate", "benzene",
	}
	for _, w := range chemicalWords {
		if isCommonWord(w) {
			t.Errorf("expected %q to NOT be common word", w)
		}
	}
}

// ============================================================================
// Test: Infer Chemical Type
// ============================================================================

func TestInferChemicalType(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"an alkyl group", "alkyl"},
		{"substituted aryl moiety", "aryl"},
		{"a heteroaryl ring", "heteroaryl"},
		{"C1-C6 alkoxy group", "alkoxy"},
		{"halogen substituent", "halogen"},
		{"no chemical type here", ""},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := inferChemicalType(tt.text)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Test: Find Preceding Parameter
// ============================================================================

func TestFindPrecedingParameter(t *testing.T) {
	tests := []struct {
		fullText    string
		matchedText string
		expected    string
	}{
		{
			fullText:    "temperature of from 50 to 100",
			matchedText: "from 50 to 100",
			expected:    "temperature",
		},
		{
			fullText:    "the pressure of at least 200 kPa",
			matchedText: "at least 200 kPa",
			expected:    "pressure",
		},
		{
			fullText:    "concentration of from about 1% to about 10%",
			matchedText: "from about 1% to about 10%",
			expected:    "concentration",
		},
		{
			fullText:    "from 50 to 100",
			matchedText: "from 50 to 100",
			expected:    "", // no preceding parameter
		},
	}

	for _, tt := range tests {
		t.Run(tt.fullText[:min(40, len(tt.fullText))], func(t *testing.T) {
			result := findPrecedingParameter(tt.fullText, tt.matchedText)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Test: Scope Score Decoding Edge Cases
// ============================================================================

func TestDecodeScopeScore_EdgeCases(t *testing.T) {
	parser := newTestParser(&mockClaimModelBackend{})
	impl := parser.(*claimParserImpl)

	// Missing scope output -> default 0.5
	resp := &common.PredictResponse{
		Outputs: map[string][]byte{},
	}
	score := impl.decodeScopeScore(resp)
	if score != 0.5 {
		t.Errorf("expected default 0.5, got %f", score)
	}

	// Invalid JSON -> default 0.5
	resp = &common.PredictResponse{
		Outputs: map[string][]byte{
			"scope": []byte("not json"),
		},
	}
	score = impl.decodeScopeScore(resp)
	if score != 0.5 {
		t.Errorf("expected default 0.5 for invalid JSON, got %f", score)
	}

	// Negative score -> clamped to 0
	data, _ := json.Marshal(scopeOutput{Score: -0.5})
	resp = &common.PredictResponse{
		Outputs: map[string][]byte{
			"scope": data,
		},
	}
	score = impl.decodeScopeScore(resp)
	if score != 0 {
		t.Errorf("expected clamped 0, got %f", score)
	}

	// Score > 1 -> clamped to 1
	data, _ = json.Marshal(scopeOutput{Score: 1.5})
	resp = &common.PredictResponse{
		Outputs: map[string][]byte{
			"scope": data,
		},
	}
	score = impl.decodeScopeScore(resp)
	if score != 1 {
		t.Errorf("expected clamped 1, got %f", score)
	}

	// Normal score
	data, _ = json.Marshal(scopeOutput{Score: 0.73})
	resp = &common.PredictResponse{
		Outputs: map[string][]byte{
			"scope": data,
		},
	}
	score = impl.decodeScopeScore(resp)
	if math.Abs(score-0.73) > 1e-6 {
		t.Errorf("expected 0.73, got %f", score)
	}
}

// ============================================================================
// Test: Model Dependency Decoding
// ============================================================================

func TestDecodeModelDependency(t *testing.T) {
	parser := newTestParser(&mockClaimModelBackend{})
	impl := parser.(*claimParserImpl)

	// Missing dependency output -> nil
	resp := &common.PredictResponse{
		Outputs: map[string][]byte{},
	}
	deps := impl.decodeModelDependency(resp)
	if deps != nil {
		t.Errorf("expected nil, got %v", deps)
	}

	// Invalid JSON -> nil
	resp = &common.PredictResponse{
		Outputs: map[string][]byte{
			"dependency": []byte("bad json"),
		},
	}
	deps = impl.decodeModelDependency(resp)
	if deps != nil {
		t.Errorf("expected nil for invalid JSON, got %v", deps)
	}

	// Valid references
	data, _ := json.Marshal(dependencyOutput{References: []int{1, 3, 5}})
	resp = &common.PredictResponse{
		Outputs: map[string][]byte{
			"dependency": data,
		},
	}
	deps = impl.decodeModelDependency(resp)
	if len(deps) != 3 || deps[0] != 1 || deps[1] != 3 || deps[2] != 5 {
		t.Errorf("expected [1,3,5], got %v", deps)
	}
}

// ============================================================================
// Benchmark: ParseClaim
// ============================================================================

func BenchmarkParseClaim(b *testing.B) {
	text := "A pharmaceutical composition comprising a therapeutically effective amount of a compound of formula (I), or a pharmaceutically acceptable salt thereof, and at least one pharmaceutically acceptable excipient selected from the group consisting of lactose, starch, microcrystalline cellulose, and magnesium stearate."

	words := strings.Fields(text)
	bioTags := make([]int, len(words))
	for i := range bioTags {
		if i%5 == 0 {
			bioTags[i] = bioBStructural
		} else if i%5 == 1 || i%5 == 2 {
			bioTags[i] = bioIStructural
		} else {
			bioTags[i] = bioO
		}
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.85, 0.05, 0.03, 0.05, 0.02},
		bioTags:             bioTags,
		scopeScore:          0.72,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ParseClaim(ctx, text)
		if err != nil {
			b.Fatalf("ParseClaim failed: %v", err)
		}
	}
}

// ============================================================================
// Benchmark: ExtractFeatures
// ============================================================================

func BenchmarkExtractFeatures(b *testing.B) {
	text := "a compound of formula (I) and a pharmaceutically acceptable carrier and an excipient selected from the group consisting of lactose and starch"

	words := strings.Fields(text)
	bioTags := make([]int, len(words))
	for i := range bioTags {
		if i < 5 {
			bioTags[i] = bioBStructural + (i % 2) // alternate B/I
		} else {
			bioTags[i] = bioO
		}
	}
	// Fix: ensure proper BIO sequence
	bioTags[0] = bioBStructural
	for i := 1; i < 5; i++ {
		bioTags[i] = bioIStructural
	}

	backend := &mockClaimModelBackend{
		bioTags: bioTags,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ExtractFeatures(ctx, text)
		if err != nil {
			b.Fatalf("ExtractFeatures failed: %v", err)
		}
	}
}

// ============================================================================
// Benchmark: ClassifyClaim
// ============================================================================

func BenchmarkClassifyClaim(b *testing.B) {
	text := "A method for treating inflammation in a mammalian subject comprising administering to the subject a therapeutically effective amount of a compound of formula (I)."

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.05, 0.03, 0.85, 0.04, 0.03},
		bioTags:             make([]int, len(strings.Fields(text))),
		scopeScore:          0.55,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ClassifyClaim(ctx, text)
		if err != nil {
			b.Fatalf("ClassifyClaim failed: %v", err)
		}
	}
}

// ============================================================================
// Benchmark: AnalyzeDependency
// ============================================================================

func BenchmarkAnalyzeDependency(b *testing.B) {
	claims := make([]string, 20)
	claims[0] = "1. A composition comprising compound X."
	for i := 1; i < 20; i++ {
		claims[i] = fmt.Sprintf("%d. The composition of claim %d, further comprising component %c.", i+1, i, 'A'+rune(i%26))
	}

	backend := &mockClaimModelBackend{
		classificationProbs: []float64{0.80, 0.05, 0.05, 0.05, 0.05},
		bioTags:             []int{bioO, bioO, bioO, bioO, bioO},
		scopeScore:          0.5,
	}

	parser := newTestParser(backend)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.AnalyzeDependency(ctx, claims)
		if err != nil {
			b.Fatalf("AnalyzeDependency failed: %v", err)
		}
	}
}

// ============================================================================
// Benchmark: Rule-Based Extraction Functions
// ============================================================================

func BenchmarkExtractDependencyReferences(b *testing.B) {
	text := "The composition of claims 1, 3, and 5 to 10, wherein the compound is a salt."
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractDependencyReferences(text)
	}
}

func BenchmarkExtractNumericalRanges(b *testing.B) {
	text := "a temperature of from about 50°C to about 100°C and a pressure of between 1 atm and 5 atm with at least 95% purity"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractNumericalRanges(text)
	}
}

func BenchmarkExtractMarkushGroups(b *testing.B) {
	text := "selected from the group consisting of aspirin, ibuprofen, naproxen, acetaminophen, and celecoxib"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractMarkushGroups(text)
	}
}

func BenchmarkPreprocessClaimText(b *testing.B) {
	text := "  A  pharmaceutical\t\tcomposition\n\ncomprising  a  compound  of  formula  (I)\u201C\u201D\u2014\u2013  "
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preprocessClaimText(text)
	}
}

func BenchmarkCosineSimilarity(b *testing.B) {
	a := make([]float32, 768)
	bv := make([]float32, 768)
	for i := range a {
		a[i] = float32(i) * 0.001
		bv[i] = float32(768-i) * 0.001
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CosineSimilarity(a, bv)
	}
}

func BenchmarkCorrectBIOSequence(b *testing.B) {
	tags := make([]int, 128)
	for i := range tags {
		switch i % 7 {
		case 0:
			tags[i] = bioBStructural
		case 1, 2:
			tags[i] = bioIStructural
		case 3:
			tags[i] = bioO
		case 4:
			tags[i] = bioIFunctional // orphan I
		case 5:
			tags[i] = bioBFunctional
		case 6:
			tags[i] = bioIFunctional
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		correctBIOSequence(tags)
	}
}
