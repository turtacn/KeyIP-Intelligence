package claim_bert

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ============================================================================
// Enumerations
// ============================================================================

// ClaimType enumerates the legal categories of patent claims.
type ClaimType string

const (
	ClaimIndependent ClaimType = "INDEPENDENT"
	ClaimDependent   ClaimType = "DEPENDENT"
	ClaimMethod      ClaimType = "METHOD"
	ClaimProduct     ClaimType = "PRODUCT"
	ClaimUse         ClaimType = "USE"
)

// claimTypeFromIndex maps model output index to ClaimType.
var claimTypeFromIndex = map[int]ClaimType{
	0: ClaimIndependent,
	1: ClaimDependent,
	2: ClaimMethod,
	3: ClaimProduct,
	4: ClaimUse,
}

// FeatureType enumerates the categories of technical features.
type FeatureType string

const (
	FeatureStructural  FeatureType = "STRUCTURAL"
	FeatureFunctional  FeatureType = "FUNCTIONAL"
	FeatureProcess     FeatureType = "PROCESS"
	FeatureComposition FeatureType = "COMPOSITION"
	FeatureParameter   FeatureType = "PARAMETER"
)

// featureTypeFromBIO maps the BIO suffix to FeatureType.
var featureTypeFromBIO = map[string]FeatureType{
	"Structural":  FeatureStructural,
	"Functional":  FeatureFunctional,
	"Process":     FeatureProcess,
	"Composition": FeatureComposition,
	"Parameter":   FeatureParameter,
}

// TransitionalPhraseType enumerates the legal categories of transitional phrases
// and their implications for claim scope.
type TransitionalPhraseType string

const (
	// PhraseComprising is open-ended: additional elements do not avoid infringement.
	PhraseComprising TransitionalPhraseType = "COMPRISING"
	// PhraseConsistingOf is closed: only the recited elements are covered.
	PhraseConsistingOf TransitionalPhraseType = "CONSISTING_OF"
	// PhraseConsistingEssentiallyOf is semi-open: additional elements that do not
	// materially affect the basic and novel characteristics are permitted.
	PhraseConsistingEssentiallyOf TransitionalPhraseType = "CONSISTING_ESSENTIALLY_OF"
)

// ============================================================================
// BIO Tag Vocabulary
// ============================================================================

const (
	bioO              = 0
	bioBStructural    = 1
	bioIStructural    = 2
	bioBFunctional    = 3
	bioIFunctional    = 4
	bioBProcess       = 5
	bioIProcess       = 6
	bioBComposition   = 7
	bioIComposition   = 8
	bioBParameter     = 9
	bioIParameter     = 10
)

// bioTagName maps tag index to human-readable name.
var bioTagName = map[int]string{
	bioO:            "O",
	bioBStructural:  "B-Structural",
	bioIStructural:  "I-Structural",
	bioBFunctional:  "B-Functional",
	bioIFunctional:  "I-Functional",
	bioBProcess:     "B-Process",
	bioIProcess:     "I-Process",
	bioBComposition: "B-Composition",
	bioIComposition: "I-Composition",
	bioBParameter:   "B-Parameter",
	bioIParameter:   "I-Parameter",
}

// bioTagCategory extracts the category suffix from a BIO tag index.
// Returns ("", false) for O tags.
func bioTagCategory(tagIdx int) (string, bool) {
	name, ok := bioTagName[tagIdx]
	if !ok || name == "O" {
		return "", false
	}
	parts := strings.SplitN(name, "-", 2)
	if len(parts) != 2 {
		return "", false
	}
	return parts[1], true
}

// bioTagIsB returns true if the tag is a B- (begin) tag.
func bioTagIsB(tagIdx int) bool {
	return tagIdx == bioBStructural || tagIdx == bioBFunctional ||
		tagIdx == bioBProcess || tagIdx == bioBComposition || tagIdx == bioBParameter
}

// bioTagIsI returns true if the tag is an I- (inside) tag.
func bioTagIsI(tagIdx int) bool {
	return tagIdx == bioIStructural || tagIdx == bioIFunctional ||
		tagIdx == bioIProcess || tagIdx == bioIComposition || tagIdx == bioIParameter
}

// bioCorrespondingB returns the B- tag for a given I- tag.
func bioCorrespondingB(iTag int) int {
	switch iTag {
	case bioIStructural:
		return bioBStructural
	case bioIFunctional:
		return bioBFunctional
	case bioIProcess:
		return bioBProcess
	case bioIComposition:
		return bioBComposition
	case bioIParameter:
		return bioBParameter
	default:
		return bioO
	}
}

// bioSameCategory returns true if two tags belong to the same feature category.
func bioSameCategory(a, b int) bool {
	catA, okA := bioTagCategory(a)
	catB, okB := bioTagCategory(b)
	return okA && okB && catA == catB
}

// ============================================================================
// Sentinel Errors
// ============================================================================

var (
	ErrEmptyClaimText       = errors.New("CLAIM_EMPTY", "claim text is empty")
	ErrClaimTextTooLong     = errors.New("CLAIM_TOO_LONG", "claim text exceeds maximum sequence length")
	ErrModelInferenceFailed = errors.New("CLAIM_INFERENCE_FAILED", "model inference failed")
	ErrInvalidBIOSequence   = errors.New("CLAIM_INVALID_BIO", "invalid BIO tag sequence")
)

// ============================================================================
// Data Structures
// ============================================================================

// NumericalRange represents a parsed numerical range from claim text.
type NumericalRange struct {
	Parameter     string   `json:"parameter"`
	LowerBound    *float64 `json:"lower_bound,omitempty"`
	UpperBound    *float64 `json:"upper_bound,omitempty"`
	Unit          string   `json:"unit,omitempty"`
	IsApproximate bool     `json:"is_approximate"`
}

// MarkushGroup represents a Markush-type chemical group in a claim.
type MarkushGroup struct {
	GroupID      string   `json:"group_id"`
	LeadPhrase   string   `json:"lead_phrase"`
	Members      []string `json:"members"`
	IsOpenEnded  bool     `json:"is_open_ended"`
	ChemicalType string   `json:"chemical_type,omitempty"`
}

// TechnicalFeature represents a single technical feature extracted from a claim.
type TechnicalFeature struct {
	ID               string           `json:"id"`
	Text             string           `json:"text"`
	StartOffset      int              `json:"start_offset"`
	EndOffset        int              `json:"end_offset"`
	FeatureType      FeatureType      `json:"feature_type"`
	IsEssential      bool             `json:"is_essential"`
	ChemicalEntities []string         `json:"chemical_entities,omitempty"`
	NumericalRanges  []*NumericalRange `json:"numerical_ranges,omitempty"`
}

// ParsedClaim is the fully structured representation of a single patent claim.
type ParsedClaim struct {
	ClaimNumber        int                    `json:"claim_number"`
	ClaimType          ClaimType              `json:"claim_type"`
	Preamble           string                 `json:"preamble"`
	TransitionalPhrase string                 `json:"transitional_phrase"`
	TransitionalType   TransitionalPhraseType `json:"transitional_type"`
	Body               string                 `json:"body"`
	Features           []*TechnicalFeature    `json:"features"`
	DependsOn          []int                  `json:"depends_on,omitempty"`
	ScopeScore         float64                `json:"scope_score"`
	MarkushGroups      []*MarkushGroup        `json:"markush_groups,omitempty"`
	Confidence         float64                `json:"confidence"`
}

// ClaimClassification is the output of claim type classification.
type ClaimClassification struct {
	ClaimType     ClaimType              `json:"claim_type"`
	Confidence    float64                `json:"confidence"`
	Probabilities map[ClaimType]float64  `json:"probabilities"`
}

// DependencyTree represents the hierarchical dependency structure of a claim set.
type DependencyTree struct {
	Roots    []int          `json:"roots"`
	Children map[int][]int  `json:"children"`
	Depth    int            `json:"depth"`
}

// ParsedClaimSet is the structured representation of an entire set of claims.
type ParsedClaimSet struct {
	Claims            []*ParsedClaim  `json:"claims"`
	DependencyTree    *DependencyTree `json:"dependency_tree"`
	IndependentClaims []int           `json:"independent_claims"`
	ClaimCount        int             `json:"claim_count"`
}

// ============================================================================
// Internal model output structures (JSON decoded from backend response)
// ============================================================================

type classificationOutput struct {
	Probabilities []float64 `json:"probabilities"`
}

type bioTagsOutput struct {
	Tags []int `json:"tags"`
}

type scopeOutput struct {
	Score float64 `json:"score"`
}

type dependencyOutput struct {
	References []int `json:"references"`
}

// ============================================================================
// ClaimParser Interface
// ============================================================================

// ClaimParser defines the capabilities of the claim semantic parser.
type ClaimParser interface {
	// ParseClaim parses a single patent claim into a structured representation.
	ParseClaim(ctx context.Context, text string) (*ParsedClaim, error)

	// ParseClaimSet parses a complete set of claims including dependency analysis.
	ParseClaimSet(ctx context.Context, texts []string) (*ParsedClaimSet, error)

	// ExtractFeatures extracts technical features from a claim using BIO tagging.
	ExtractFeatures(ctx context.Context, text string) ([]*TechnicalFeature, error)

	// ClassifyClaim classifies a claim into its legal category.
	ClassifyClaim(ctx context.Context, text string) (*ClaimClassification, error)

	// AnalyzeDependency builds a dependency tree from a set of claims.
	AnalyzeDependency(ctx context.Context, claims []string) (*DependencyTree, error)
}

// ============================================================================
// Compiled Regex Patterns
// ============================================================================

var (
	// --- Transitional phrases (English) ---
	reConsistingEssentiallyOf = regexp.MustCompile(
		`(?i)\b(consisting\s+essentially\s+of)\b`)
	reConsistingOf = regexp.MustCompile(
		`(?i)\b(consisting\s+of)\b`)
	reComprising = regexp.MustCompile(
		`(?i)\b(comprising|which\s+comprises?|characterized\s+in\s+that|wherein)\b`)

	// --- Transitional phrases (Chinese) ---
	reChineseConsistingOf = regexp.MustCompile(
		`(由[^，,。.]+组成)`)
	reChineseConsistingEssentiallyOf = regexp.MustCompile(
		`(基本上由[^，,。.]+组成)`)
	reChineseComprising = regexp.MustCompile(
		`(包含|包括|含有|其特征在于|其中)`)

	// --- Dependency references (English) ---
	reDependencyEN = regexp.MustCompile(
		`(?i)(?:of|in|according\s+to|as\s+(?:claimed|defined|set\s+forth)\s+in)\s+claims?\s+` +
			`(\d+(?:\s*(?:,|and|or|to)\s*\d+)*)`)

	// --- Dependency references (Chinese) ---
	reDependencyCN = regexp.MustCompile(
		`(?:如)?权利要求\s*(\d+(?:\s*[、,，和或至到]\s*\d+)*)(?:\s*所述)?`)

	// --- Claim number extraction ---
	reClaimNumberEN = regexp.MustCompile(`(?i)^(?:claim\s+)?(\d+)\s*[.:\-)\]]\s*`)
	reClaimNumberCN = regexp.MustCompile(`^(\d+)\s*[、.．:：)\]]\s*`)

	// --- Markush group (closed) ---
	reMarkushClosed = regexp.MustCompile(
		`(?i)selected\s+from\s+the\s+group\s+consisting\s+of\s+(.+?)(?:\.|;|$)`)

	// --- Markush group (open-ended) ---
	reMarkushOpen = regexp.MustCompile(
		`(?i)(?:including\s+but\s+not\s+limited\s+to|such\s+as|for\s+example)\s+(.+?)(?:\.|;|$)`)

	// --- Numerical ranges ---
	reRangeFromTo = regexp.MustCompile(
		`(?i)(?:from\s+)(about\s+)?(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)\s*(?:to)\s+(about\s+)?(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)`)
	reRangeBetweenAnd = regexp.MustCompile(
		`(?i)(?:between\s+)(about\s+)?(\d+(?:\.\d+)?)\s*(?:and)\s+(about\s+)?(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)`)
	reAtLeast = regexp.MustCompile(
		`(?i)(?:at\s+least|no\s+less\s+than|not\s+less\s+than|≥|>=)\s*(about\s+)?(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)`)
	reAtMost = regexp.MustCompile(
		`(?i)(?:at\s+most|no\s+more\s+than|not\s+more\s+than|≤|<=)\s*(about\s+)?(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)`)
	reLessThan = regexp.MustCompile(
		`(?i)(?:less\s+than|below|under|<)\s*(about\s+)?(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)`)
	reGreaterThan = regexp.MustCompile(
		`(?i)(?:greater\s+than|above|over|more\s+than|exceeding|>)\s*(about\s+)?(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)`)
	reAboutSingle = regexp.MustCompile(
		`(?i)(about|approximately|circa|roughly|~)\s+(\d+(?:\.\d+)?)\s*([°%℃℉]?[a-zA-Z/µμ]*)`)

	// --- Chemical entity patterns ---
	reChemicalFormula = regexp.MustCompile(
		`(?:formula\s+)?\(([IVX]+|[A-Z])\)`)
	reChemicalSuffix = regexp.MustCompile(
		`(?i)\b[a-z]*(?:ine|ol|ase|ide|ate|ene|ane|one|yl|oyl|amide|amine|acid)\b`)
	reChemicalType = regexp.MustCompile(
		`(?i)\b(alkyl|aryl|heteroaryl|heterocyclic|cycloalkyl|alkenyl|alkynyl|alkoxy|halogen|amino|hydroxyl|carboxyl)\b`)

	// --- Parameter name extraction (preceding a numerical value) ---
	reParameterName = regexp.MustCompile(
		`(?i)(temperature|pressure|concentration|ratio|amount|weight|volume|` +
			`time|duration|pH|molecular\s+weight|viscosity|density|purity|yield|` +
			`thickness|diameter|length|width|height|dose|dosage|flow\s+rate)\s+(?:of\s+)?`)
)

// ============================================================================
// claimParserImpl
// ============================================================================

// claimParserImpl is the concrete implementation of ClaimParser.
type claimParserImpl struct {
	backend   common.ModelBackend
	config    *ClaimBERTConfig
	tokenizer Tokenizer
	logger    common.Logger
	metrics   common.IntelligenceMetrics
	mu        sync.RWMutex
}

// NewClaimParser creates a new ClaimParser with all required dependencies.
func NewClaimParser(
	backend common.ModelBackend,
	config *ClaimBERTConfig,
	tokenizer Tokenizer,
	logger common.Logger,
	metrics common.IntelligenceMetrics,
) (ClaimParser, error) {
	if backend == nil {
		return nil, errors.NewInvalidInputError("model backend is required")
	}
	if config == nil {
		return nil, errors.NewInvalidInputError("ClaimBERT config is required")
	}
	if tokenizer == nil {
		return nil, errors.NewInvalidInputError("tokenizer is required")
	}
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	if metrics == nil {
		metrics = common.NewNoopIntelligenceMetrics()
	}
	return &claimParserImpl{
		backend:   backend,
		config:    config,
		tokenizer: tokenizer,
		logger:    logger,
		metrics:   metrics,
	}, nil
}

// ============================================================================
// ParseClaim
// ============================================================================

func (p *claimParserImpl) ParseClaim(ctx context.Context, text string) (*ParsedClaim, error) {
	// 1. Validate input
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyClaimText
	}
	start := time.Now()

	// 2. Preprocess
	cleaned := preprocessClaimText(text)
	truncated := false
	if p.config.MaxSequenceLength > 0 && len([]rune(cleaned)) > p.config.MaxSequenceLength {
		runes := []rune(cleaned)
		cleaned = string(runes[:p.config.MaxSequenceLength])
		truncated = true
		p.logger.Warn("claim text truncated",
			"original_rune_len", len(runes),
			"max", p.config.MaxSequenceLength)
	}

	// 3. Tokenize
	tokenized, err := p.tokenizer.Tokenize(cleaned)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	// 4. Model inference — all task heads
	backendReq := p.buildPredictRequest(tokenized, []string{
		"classification", "bio_tags", "scope", "dependency",
	})
	backendResp, err := p.backend.Predict(ctx, backendReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModelInferenceFailed, err)
	}

	// 5. Decode classification
	claimType, confidence, probs, err := p.decodeClassification(backendResp)
	if err != nil {
		p.logger.Warn("classification decode failed, falling back to rule-based", "error", err)
		claimType, confidence = p.ruleBasedClassification(cleaned)
		probs = map[ClaimType]float64{claimType: confidence}
	}

	// 6. Decode BIO tags -> technical features
	features, err := p.decodeBIOTags(backendResp, tokenized, cleaned)
	if err != nil {
		p.logger.Warn("BIO decode failed, returning empty features", "error", err)
		features = []*TechnicalFeature{}
	}

	// 7. Decode scope score
	scopeScore := p.decodeScopeScore(backendResp)

	// 8. Decode dependency references (model-based)
	modelDeps := p.decodeModelDependency(backendResp)

	// 9. Rule-based post-processing
	claimNumber := extractClaimNumber(text)
	transitionalPhrase, transitionalType := detectTransitionalPhrase(cleaned)
	preamble, body := splitPreambleBody(cleaned, transitionalPhrase)
	markushGroups := extractMarkushGroups(cleaned)
	ruleDeps := extractDependencyReferences(cleaned)

	// Merge model and rule-based dependencies
	deps := mergeDependencies(modelDeps, ruleDeps)

	// Refine claim type based on dependencies
	if len(deps) > 0 && claimType == ClaimIndependent {
		claimType = ClaimDependent
	}

	// Enrich features with chemical entities and numerical ranges
	for _, feat := range features {
		feat.ChemicalEntities = extractChemicalEntities(feat.Text)
		feat.NumericalRanges = extractNumericalRanges(feat.Text)
	}

	// Also extract numerical ranges from the full body
	bodyRanges := extractNumericalRanges(body)
	_ = bodyRanges // available for future enrichment

	// Mark essential features for independent claims
	if claimType == ClaimIndependent || claimType == ClaimMethod ||
		claimType == ClaimProduct || claimType == ClaimUse {
		for _, f := range features {
			f.IsEssential = true
		}
	}

	elapsed := time.Since(start)
	p.metrics.RecordInference(ctx, &common.InferenceMetricParams{
		ModelName:  p.config.ModelID,
		TaskType:   "parse_claim",
		DurationMs: float64(elapsed.Milliseconds()),
		Success:    true,
		BatchSize:  1,
	})

	result := &ParsedClaim{
		ClaimNumber:        claimNumber,
		ClaimType:          claimType,
		Preamble:           preamble,
		TransitionalPhrase: transitionalPhrase,
		TransitionalType:   transitionalType,
		Body:               body,
		Features:           features,
		DependsOn:          deps,
		ScopeScore:         scopeScore,
		MarkushGroups:      markushGroups,
		Confidence:         confidence,
	}

	if truncated {
		result.Confidence *= 0.8 // penalize confidence for truncated text
	}

	return result, nil
}

// ============================================================================
// ParseClaimSet
// ============================================================================

func (p *claimParserImpl) ParseClaimSet(ctx context.Context, texts []string) (*ParsedClaimSet, error) {
	if len(texts) == 0 {
		return &ParsedClaimSet{
			Claims:         []*ParsedClaim{},
			DependencyTree: &DependencyTree{Roots: []int{}, Children: map[int][]int{}, Depth: 0},
			ClaimCount:     0,
		}, nil
	}

	claims := make([]*ParsedClaim, 0, len(texts))
	var parseErrors []string

	for i, text := range texts {
		parsed, err := p.ParseClaim(ctx, text)
		if err != nil {
			p.logger.Warn("failed to parse claim",
				"index", i, "error", err)
			parseErrors = append(parseErrors, fmt.Sprintf("claim %d: %v", i+1, err))
			continue
		}
		// Assign claim number if not extracted from text
		if parsed.ClaimNumber == 0 {
			parsed.ClaimNumber = i + 1
		}
		claims = append(claims, parsed)
	}

	// Build dependency tree
	depTree, err := p.AnalyzeDependency(ctx, texts)
	if err != nil {
		p.logger.Warn("dependency analysis failed", "error", err)
		depTree = &DependencyTree{Roots: []int{}, Children: map[int][]int{}, Depth: 0}
	}

	// Identify independent claims
	independentClaims := make([]int, 0)
	for _, c := range claims {
		if len(c.DependsOn) == 0 {
			independentClaims = append(independentClaims, c.ClaimNumber)
		}
	}
	sort.Ints(independentClaims)

	return &ParsedClaimSet{
		Claims:            claims,
		DependencyTree:    depTree,
		IndependentClaims: independentClaims,
		ClaimCount:        len(claims),
	}, nil
}

// ============================================================================
// ExtractFeatures
// ============================================================================

func (p *claimParserImpl) ExtractFeatures(ctx context.Context, text string) ([]*TechnicalFeature, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyClaimText
	}

	cleaned := preprocessClaimText(text)
	if p.config.MaxSequenceLength > 0 && len([]rune(cleaned)) > p.config.MaxSequenceLength {
		runes := []rune(cleaned)
		cleaned = string(runes[:p.config.MaxSequenceLength])
	}

	tokenized, err := p.tokenizer.Tokenize(cleaned)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	backendReq := p.buildPredictRequest(tokenized, []string{"bio_tags"})
	backendResp, err := p.backend.Predict(ctx, backendReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModelInferenceFailed, err)
	}

	features, err := p.decodeBIOTags(backendResp, tokenized, cleaned)
	if err != nil {
		return nil, err
	}

	// Enrich
	for _, feat := range features {
		feat.ChemicalEntities = extractChemicalEntities(feat.Text)
		feat.NumericalRanges = extractNumericalRanges(feat.Text)
	}

	return features, nil
}

// ============================================================================
// ClassifyClaim
// ============================================================================

func (p *claimParserImpl) ClassifyClaim(ctx context.Context, text string) (*ClaimClassification, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyClaimText
	}

	cleaned := preprocessClaimText(text)
	if p.config.MaxSequenceLength > 0 && len([]rune(cleaned)) > p.config.MaxSequenceLength {
		runes := []rune(cleaned)
		cleaned = string(runes[:p.config.MaxSequenceLength])
	}

	tokenized, err := p.tokenizer.Tokenize(cleaned)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	backendReq := p.buildPredictRequest(tokenized, []string{"classification"})
	backendResp, err := p.backend.Predict(ctx, backendReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModelInferenceFailed, err)
	}

	claimType, confidence, probs, err := p.decodeClassification(backendResp)
	if err != nil {
		// Fallback to rule-based
		claimType, confidence = p.ruleBasedClassification(cleaned)
		probs = map[ClaimType]float64{claimType: confidence}
	}

	return &ClaimClassification{
		ClaimType:     claimType,
		Confidence:    confidence,
		Probabilities: probs,
	}, nil
}

// ============================================================================
// AnalyzeDependency
// ============================================================================

func (p *claimParserImpl) AnalyzeDependency(ctx context.Context, claims []string) (*DependencyTree, error) {
	if len(claims) == 0 {
		return &DependencyTree{
			Roots:    []int{},
			Children: map[int][]int{},
			Depth:    0,
		}, nil
	}

	// Parse dependency references from each claim
	claimDeps := make(map[int][]int) // claimNumber -> depends on
	allClaimNumbers := make(map[int]bool)

	for i, text := range claims {
		claimNum := i + 1
		// Try to extract claim number from text
		if n := extractClaimNumber(text); n > 0 {
			claimNum = n
		}
		allClaimNumbers[claimNum] = true
		deps := extractDependencyReferences(text)
		if len(deps) > 0 {
			claimDeps[claimNum] = deps
		}
	}

	// Build children map (parent -> children)
	children := make(map[int][]int)
	dependedOn := make(map[int]bool)
	for child, parents := range claimDeps {
		for _, parent := range parents {
			children[parent] = append(children[parent], child)
			dependedOn[child] = true
		}
	}

	// Sort children lists
	for k := range children {
		sort.Ints(children[k])
	}

	// Find roots (claims that don't depend on anything)
	roots := make([]int, 0)
	for num := range allClaimNumbers {
		if !dependedOn[num] {
			roots = append(roots, num)
		}
	}
	sort.Ints(roots)

	// Calculate max depth via BFS
	depth := calculateTreeDepth(roots, children)

	return &DependencyTree{
		Roots:    roots,
		Children: children,
		Depth:    depth,
	}, nil
}

// ============================================================================
// Internal: Model interaction helpers
// ============================================================================

func (p *claimParserImpl) buildPredictRequest(tokenized *TokenizedInput, taskHeads []string) *common.PredictRequest {
	inputPayload := map[string]interface{}{
		"input_ids":      tokenized.InputIDs,
		"attention_mask": tokenized.AttentionMask,
		"token_type_ids": tokenized.TokenTypeIDs,
		"task_heads":     taskHeads,
	}
	data, _ := json.Marshal(inputPayload)
	return &common.PredictRequest{
		ModelName:   p.config.ModelID,
		InputData:   data,
		InputFormat: common.FormatJSON,
		Metadata: map[string]string{
			"task_heads": strings.Join(taskHeads, ","),
		},
	}
}

func (p *claimParserImpl) decodeClassification(resp *common.PredictResponse) (ClaimType, float64, map[ClaimType]float64, error) {
	raw, ok := resp.Outputs["classification"]
	if !ok {
		return "", 0, nil, fmt.Errorf("classification output not found")
	}
	var out classificationOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", 0, nil, fmt.Errorf("unmarshal classification: %w", err)
	}
	if len(out.Probabilities) == 0 {
		return "", 0, nil, fmt.Errorf("empty classification probabilities")
	}

	// Find argmax
	maxIdx := 0
	maxProb := out.Probabilities[0]
	probs := make(map[ClaimType]float64)
	for i, prob := range out.Probabilities {
		ct, exists := claimTypeFromIndex[i]
		if exists {
			probs[ct] = prob
		}
		if prob > maxProb {
			maxProb = prob
			maxIdx = i
		}
	}

	ct, exists := claimTypeFromIndex[maxIdx]
	if !exists {
		ct = ClaimIndependent
	}

	return ct, maxProb, probs, nil
}

func (p *claimParserImpl) decodeBIOTags(resp *common.PredictResponse, tokenized *TokenizedInput, originalText string) ([]*TechnicalFeature, error) {
	raw, ok := resp.Outputs["bio_tags"]
	if !ok {
		return nil, fmt.Errorf("bio_tags output not found")
	}
	var out bioTagsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshal bio_tags: %w", err)
	}

	// Align tags with tokens (skip special tokens [CLS], [SEP])
	tags := out.Tags
	if len(tags) > len(tokenized.Tokens) {
		tags = tags[:len(tokenized.Tokens)]
	}

	// Auto-correct inconsistent BIO sequences
	tags = correctBIOSequence(tags)

	// Convert BIO tags to spans
	spans := bioTagsToSpans(tags, tokenized, originalText)
	return spans, nil
}

func (p *claimParserImpl) decodeScopeScore(resp *common.PredictResponse) float64 {
	raw, ok := resp.Outputs["scope"]
	if !ok {
		return 0.5 // default mid-range
	}
	var out scopeOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return 0.5
	}
	// Clamp to [0, 1]
	if out.Score < 0 {
		return 0
	}
	if out.Score > 1 {
		return 1
	}
	return out.Score
}

func (p *claimParserImpl) decodeModelDependency(resp *common.PredictResponse) []int {
	raw, ok := resp.Outputs["dependency"]
	if !ok {
		return nil
	}
	var out dependencyOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out.References
}

// ============================================================================
// Internal: BIO tag processing
// ============================================================================

// correctBIOSequence fixes inconsistent BIO sequences.
// Rule: an I- tag without a matching preceding B- tag is promoted to B-.
func correctBIOSequence(tags []int) []int {
	if len(tags) == 0 {
		return tags
	}
	corrected := make([]int, len(tags))
	copy(corrected, tags)

	for i := 0; i < len(corrected); i++ {
		tag := corrected[i]
		if !bioTagIsI(tag) {
			continue
		}
		// Check if there's a matching preceding B or I of the same category
		if i == 0 {
			// No predecessor — promote to B
			corrected[i] = bioCorrespondingB(tag)
			continue
		}
		prev := corrected[i-1]
		if !bioSameCategory(prev, tag) {
			// Previous tag is a different category or O — promote to B
			corrected[i] = bioCorrespondingB(tag)
		}
	}
	return corrected
}

// bioSpan is an intermediate representation of a BIO span.
type bioSpan struct {
	startToken int
	endToken   int // exclusive
	category   string
}

// bioTagsToSpans converts a corrected BIO tag sequence into TechnicalFeature slices.
func bioTagsToSpans(tags []int, tokenized *TokenizedInput, originalText string) []*TechnicalFeature {
	if len(tags) == 0 {
		return nil
	}

	// Collect raw spans
	var spans []bioSpan
	var current *bioSpan

	for i, tag := range tags {
		if bioTagIsB(tag) {
			// Close previous span if open
			if current != nil {
				spans = append(spans, *current)
			}
			cat, _ := bioTagCategory(tag)
			current = &bioSpan{
				startToken: i,
				endToken:   i + 1,
				category:   cat,
			}
		} else if bioTagIsI(tag) {
			cat, _ := bioTagCategory(tag)
			if current != nil && current.category == cat {
				current.endToken = i + 1
			} else {
				// Orphan I tag (should have been corrected, but defensive)
				if current != nil {
					spans = append(spans, *current)
				}
				current = &bioSpan{
					startToken: i,
					endToken:   i + 1,
					category:   cat,
				}
			}
		} else {
			// O tag — close current span
			if current != nil {
				spans = append(spans, *current)
				current = nil
			}
		}
	}
	// Close trailing span
	if current != nil {
		spans = append(spans, *current)
	}

	// Convert spans to TechnicalFeature objects
	features := make([]*TechnicalFeature, 0, len(spans))
	for idx, sp := range spans {
		startChar, endChar := spanToCharOffsets(sp, tokenized, originalText)
		if startChar < 0 || endChar <= startChar {
			continue
		}

		// Clamp to text bounds
		textRunes := []rune(originalText)
		if endChar > len(textRunes) {
			endChar = len(textRunes)
		}
		if startChar >= len(textRunes) {
			continue
		}

		featureText := strings.TrimSpace(string(textRunes[startChar:endChar]))
		if featureText == "" {
			continue
		}

		ft, ok := featureTypeFromBIO[sp.category]
		if !ok {
			ft = FeatureStructural // default
		}

		features = append(features, &TechnicalFeature{
			ID:          fmt.Sprintf("feat-%d", idx+1),
			Text:        featureText,
			StartOffset: startChar,
			EndOffset:   endChar,
			FeatureType: ft,
			IsEssential: false, // caller sets this based on claim type
		})
	}

	return features
}

// spanToCharOffsets maps a token-level span to character-level offsets using
// the tokenizer's offset mapping. Falls back to heuristic if offsets are unavailable.
func spanToCharOffsets(sp bioSpan, tokenized *TokenizedInput, originalText string) (int, int) {
	if len(tokenized.Offsets) > 0 {
		// Use tokenizer-provided character offsets
		startIdx := sp.startToken
		endIdx := sp.endToken - 1 // inclusive

		if startIdx >= len(tokenized.Offsets) {
			return -1, -1
		}
		if endIdx >= len(tokenized.Offsets) {
			endIdx = len(tokenized.Offsets) - 1
		}

		startChar := tokenized.Offsets[startIdx][0]
		endChar := tokenized.Offsets[endIdx][1]
		return startChar, endChar
	}

	// Fallback: reconstruct from token strings
	if len(tokenized.Tokens) == 0 {
		return -1, -1
	}

	// Approximate by joining tokens and searching in original text
	var tokenTexts []string
	for i := sp.startToken; i < sp.endToken && i < len(tokenized.Tokens); i++ {
		tok := tokenized.Tokens[i]
		// Strip WordPiece prefix
		tok = strings.TrimPrefix(tok, "##")
		tok = strings.TrimPrefix(tok, "▁")
		tokenTexts = append(tokenTexts, tok)
	}
	joined := strings.Join(tokenTexts, "")

	idx := strings.Index(strings.ToLower(originalText), strings.ToLower(joined))
	if idx >= 0 {
		return idx, idx + len(joined)
	}

	return -1, -1
}

// ============================================================================
// Internal: Rule-based extraction helpers
// ============================================================================

// preprocessClaimText normalizes whitespace, punctuation, and encoding.
func preprocessClaimText(text string) string {
	// Replace various whitespace characters with single space
	var b strings.Builder
	b.Grow(len(text))
	prevSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	result := strings.TrimSpace(b.String())

	// Normalize common Unicode punctuation to ASCII equivalents
	replacer := strings.NewReplacer(
		"\u2018", "'", "\u2019", "'",
		"\u201C", "\"", "\u201D", "\"",
		"\u2013", "-", "\u2014", "-",
		"\u00B0", "°",
		"\u2264", "<=", "\u2265", ">=",
		"\uff0c", ",", "\u3001", ",",
		"\uff1b", ";", "\uff1a", ":",
		"\u3002", ".", "\uff0e", ".",
	)
	result = replacer.Replace(result)

	return result
}

// extractClaimNumber attempts to extract the claim number from the beginning of the text.
func extractClaimNumber(text string) int {
	trimmed := strings.TrimSpace(text)

	// Try English pattern: "Claim 1." or "1. A composition..."
	if m := reClaimNumberEN.FindStringSubmatch(trimmed); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}

	// Try Chinese pattern: "1、一种组合物..."
	if m := reClaimNumberCN.FindStringSubmatch(trimmed); len(m) > 1 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}

	return 0
}

// detectTransitionalPhrase identifies the transitional phrase and its legal type.
// Returns the matched phrase text and its classification.
func detectTransitionalPhrase(text string) (string, TransitionalPhraseType) {
	// Order matters: check more specific phrases first

	// English: "consisting essentially of" (must check before "consisting of")
	if m := reConsistingEssentiallyOf.FindString(text); m != "" {
		return m, PhraseConsistingEssentiallyOf
	}

	// English: "consisting of"
	if m := reConsistingOf.FindString(text); m != "" {
		return m, PhraseConsistingOf
	}

	// English: "comprising" and equivalents
	if m := reComprising.FindString(text); m != "" {
		return m, PhraseComprising
	}

	// Chinese: "基本上由...组成"
	if m := reChineseConsistingEssentiallyOf.FindString(text); m != "" {
		return m, PhraseConsistingEssentiallyOf
	}

	// Chinese: "由...组成"
	if m := reChineseConsistingOf.FindString(text); m != "" {
		return m, PhraseConsistingOf
	}

	// Chinese: "包含/包括/含有/其特征在于"
	if m := reChineseComprising.FindString(text); m != "" {
		return m, PhraseComprising
	}

	// Default: no transitional phrase found — assume comprising (most common)
	return "", PhraseComprising
}

// splitPreambleBody splits claim text into preamble and body around the transitional phrase.
func splitPreambleBody(text string, transitionalPhrase string) (string, string) {
	if transitionalPhrase == "" {
		return "", text
	}

	idx := strings.Index(strings.ToLower(text), strings.ToLower(transitionalPhrase))
	if idx < 0 {
		return "", text
	}

	preamble := strings.TrimSpace(text[:idx])
	bodyStart := idx + len(transitionalPhrase)
	body := ""
	if bodyStart < len(text) {
		body = strings.TrimSpace(text[bodyStart:])
	}

	// Remove leading claim number from preamble
	preamble = reClaimNumberEN.ReplaceAllString(preamble, "")
	preamble = reClaimNumberCN.ReplaceAllString(preamble, "")
	preamble = strings.TrimSpace(preamble)

	return preamble, body
}

// extractDependencyReferences parses claim dependency references from text.
// Returns a sorted, deduplicated list of referenced claim numbers.
func extractDependencyReferences(text string) []int {
	depSet := make(map[int]bool)

	// English patterns
	for _, m := range reDependencyEN.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			nums := parseClaimNumberList(m[1])
			for _, n := range nums {
				depSet[n] = true
			}
		}
	}

	// Chinese patterns
	for _, m := range reDependencyCN.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			nums := parseClaimNumberList(m[1])
			for _, n := range nums {
				depSet[n] = true
			}
		}
	}

	if len(depSet) == 0 {
		return nil
	}

	result := make([]int, 0, len(depSet))
	for n := range depSet {
		result = append(result, n)
	}
	sort.Ints(result)
	return result
}

// parseClaimNumberList parses a string like "1, 2, and 3" or "1至3" into individual numbers.
func parseClaimNumberList(s string) []int {
	// Normalize separators
	s = strings.NewReplacer(
		"and", ",", "or", ",", "to", "-",
		"和", ",", "或", ",", "至", "-", "到", "-",
		"、", ",", "，", ",",
	).Replace(s)

	var result []int
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for range: "1-3"
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			if len(rangeParts) == 2 {
				lo, errLo := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				hi, errHi := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if errLo == nil && errHi == nil && lo <= hi && hi-lo < 100 {
					for n := lo; n <= hi; n++ {
						result = append(result, n)
					}
					continue
				}
			}
		}

		// Single number
		if n, err := strconv.Atoi(part); err == nil && n > 0 {
			result = append(result, n)
		}
	}
	return result
}

// mergeDependencies merges model-based and rule-based dependency lists, deduplicating.
func mergeDependencies(modelDeps, ruleDeps []int) []int {
	seen := make(map[int]bool)
	for _, d := range modelDeps {
		if d > 0 {
			seen[d] = true
		}
	}
	for _, d := range ruleDeps {
		if d > 0 {
			seen[d] = true
		}
	}
	if len(seen) == 0 {
		return nil
	}
	result := make([]int, 0, len(seen))
	for d := range seen {
		result = append(result, d)
	}
	sort.Ints(result)
	return result
}

// ============================================================================
// Internal: Markush group extraction
// ============================================================================

// extractMarkushGroups identifies Markush-type group structures in claim text.
func extractMarkushGroups(text string) []*MarkushGroup {
	var groups []*MarkushGroup
	groupIdx := 0

	// Closed Markush: "selected from the group consisting of A, B, and C"
	for _, m := range reMarkushClosed.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			groupIdx++
			members := parseMarkushMembers(m[1])
			chemType := inferChemicalType(m[1])
			groups = append(groups, &MarkushGroup{
				GroupID:      fmt.Sprintf("markush-%d", groupIdx),
				LeadPhrase:   "selected from the group consisting of",
				Members:      members,
				IsOpenEnded:  false,
				ChemicalType: chemType,
			})
		}
	}

	// Open-ended Markush: "including but not limited to A, B, C"
	for _, m := range reMarkushOpen.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			groupIdx++
			members := parseMarkushMembers(m[1])
			chemType := inferChemicalType(m[1])
			leadPhrase := "including but not limited to"
			if strings.Contains(strings.ToLower(text), "such as") {
				leadPhrase = "such as"
			}
			groups = append(groups, &MarkushGroup{
				GroupID:      fmt.Sprintf("markush-%d", groupIdx),
				LeadPhrase:   leadPhrase,
				Members:      members,
				IsOpenEnded:  true,
				ChemicalType: chemType,
			})
		}
	}

	return groups
}

// parseMarkushMembers splits a Markush member list like "aspirin, ibuprofen, and naproxen"
// into individual members.
func parseMarkushMembers(s string) []string {
	// Remove trailing punctuation
	s = strings.TrimRight(s, ".;,")
	s = strings.TrimSpace(s)

	// Normalize "and"/"or" to comma
	s = regexp.MustCompile(`(?i)\s*,?\s+and\s+`).ReplaceAllString(s, ", ")
	s = regexp.MustCompile(`(?i)\s*,?\s+or\s+`).ReplaceAllString(s, ", ")

	parts := strings.Split(s, ",")
	var members []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			members = append(members, p)
		}
	}
	return members
}

// inferChemicalType attempts to identify the chemical type of a Markush group.
func inferChemicalType(text string) string {
	matches := reChemicalType.FindAllString(text, -1)
	if len(matches) > 0 {
		return strings.ToLower(matches[0])
	}
	return ""
}

// ============================================================================
// Internal: Numerical range extraction
// ============================================================================

// extractNumericalRanges parses numerical ranges from text.
func extractNumericalRanges(text string) []*NumericalRange {
	var ranges []*NumericalRange

	// "from [about] X to [about] Y [unit]"
	for _, m := range reRangeFromTo.FindAllStringSubmatch(text, -1) {
		if len(m) >= 6 {
			isApprox := strings.TrimSpace(m[1]) != "" || strings.TrimSpace(m[4]) != ""
			lo := parseFloat(m[2])
			hi := parseFloat(m[5])
			unit := bestUnit(m[3], m[6])
			param := findPrecedingParameter(text, m[0])
			ranges = append(ranges, &NumericalRange{
				Parameter:     param,
				LowerBound:    lo,
				UpperBound:    hi,
				Unit:          unit,
				IsApproximate: isApprox,
			})
		}
	}

	// "between [about] X and [about] Y [unit]"
	for _, m := range reRangeBetweenAnd.FindAllStringSubmatch(text, -1) {
		if len(m) >= 5 {
			isApprox := strings.TrimSpace(m[1]) != "" || strings.TrimSpace(m[3]) != ""
			lo := parseFloat(m[2])
			hi := parseFloat(m[4])
			unit := strings.TrimSpace(m[5])
			param := findPrecedingParameter(text, m[0])
			ranges = append(ranges, &NumericalRange{
				Parameter:     param,
				LowerBound:    lo,
				UpperBound:    hi,
				Unit:          unit,
				IsApproximate: isApprox,
			})
		}
	}

	// "at least [about] X [unit]"
	for _, m := range reAtLeast.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			isApprox := strings.TrimSpace(m[1]) != ""
			lo := parseFloat(m[2])
			unit := strings.TrimSpace(m[3])
			param := findPrecedingParameter(text, m[0])
			ranges = append(ranges, &NumericalRange{
				Parameter:     param,
				LowerBound:    lo,
				UpperBound:    nil,
				Unit:          unit,
				IsApproximate: isApprox,
			})
		}
	}

	// "at most [about] X [unit]"
	for _, m := range reAtMost.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			isApprox := strings.TrimSpace(m[1]) != ""
			hi := parseFloat(m[2])
			unit := strings.TrimSpace(m[3])
			param := findPrecedingParameter(text, m[0])
			ranges = append(ranges, &NumericalRange{
				Parameter:     param,
				LowerBound:    nil,
				UpperBound:    hi,
				Unit:          unit,
				IsApproximate: isApprox,
			})
		}
	}

	// "less than [about] X [unit]"
	for _, m := range reLessThan.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			isApprox := strings.TrimSpace(m[1]) != ""
			hi := parseFloat(m[2])
			unit := strings.TrimSpace(m[3])
			param := findPrecedingParameter(text, m[0])
			ranges = append(ranges, &NumericalRange{
				Parameter:     param,
				LowerBound:    nil,
				UpperBound:    hi,
				Unit:          unit,
				IsApproximate: isApprox,
			})
		}
	}

	// "greater than [about] X [unit]"
	for _, m := range reGreaterThan.FindAllStringSubmatch(text, -1) {
		if len(m) >= 3 {
			isApprox := strings.TrimSpace(m[1]) != ""
			lo := parseFloat(m[2])
			unit := strings.TrimSpace(m[3])
			param := findPrecedingParameter(text, m[0])
			ranges = append(ranges, &NumericalRange{
				Parameter:     param,
				LowerBound:    lo,
				UpperBound:    nil,
				Unit:          unit,
				IsApproximate: isApprox,
			})
		}
	}

	// Standalone "about X [unit]" (only if not already captured above)
	if len(ranges) == 0 {
		for _, m := range reAboutSingle.FindAllStringSubmatch(text, -1) {
			if len(m) >= 3 {
				val := parseFloat(m[2])
				unit := strings.TrimSpace(m[3])
				param := findPrecedingParameter(text, m[0])
				// "about" in patent law typically means ±10%
				var lo, hi *float64
				if val != nil {
					loVal := *val * 0.9
					hiVal := *val * 1.1
					lo = &loVal
					hi = &hiVal
				}
				ranges = append(ranges, &NumericalRange{
					Parameter:     param,
					LowerBound:    lo,
					UpperBound:    hi,
					Unit:          unit,
					IsApproximate: true,
				})
			}
		}
	}

	return ranges
}

// parseFloat parses a string to *float64, returning nil on failure.
func parseFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

// bestUnit returns the non-empty unit string, preferring the first.
func bestUnit(a, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a != "" {
		return a
	}
	return b
}

// findPrecedingParameter looks for a parameter name immediately before the matched range text.
func findPrecedingParameter(fullText, matchedText string) string {
	idx := strings.Index(fullText, matchedText)
	if idx <= 0 {
		return ""
	}
	// Look at the 80 characters preceding the match
	lookback := 80
	start := idx - lookback
	if start < 0 {
		start = 0
	}
	preceding := fullText[start:idx]

	m := reParameterName.FindStringSubmatch(preceding)
	if len(m) > 1 {
		return strings.TrimSpace(strings.ToLower(m[1]))
	}
	return ""
}

// ============================================================================
// Internal: Chemical entity extraction
// ============================================================================

// extractChemicalEntities identifies chemical entities in a text fragment.
func extractChemicalEntities(text string) []string {
	seen := make(map[string]bool)
	var entities []string

	// Chemical formulas: (I), (II), etc.
	for _, m := range reChemicalFormula.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			entity := "formula (" + m[1] + ")"
			if !seen[entity] {
				seen[entity] = true
				entities = append(entities, entity)
			}
		}
	}

	// Chemical name suffixes
	for _, m := range reChemicalSuffix.FindAllString(text, -1) {
		lower := strings.ToLower(m)
		// Filter out common English words that happen to match
		if isCommonWord(lower) {
			continue
		}
		if !seen[lower] {
			seen[lower] = true
			entities = append(entities, m)
		}
	}

	return entities
}

// isCommonWord filters out false positives from chemical suffix matching.
func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"the": true, "one": true, "done": true, "gone": true,
		"none": true, "alone": true, "bone": true, "tone": true,
		"zone": true, "stone": true, "phone": true, "define": true,
		"combine": true, "determine": true, "examine": true,
		"machine": true, "medicine": true, "online": true,
		"routine": true, "decline": true, "discipline": true,
		"imagine": true, "genuine": true, "mine": true, "line": true,
		"fine": true, "wine": true, "nine": true, "pine": true,
		"dine": true, "vine": true, "shine": true, "spine": true,
		"whole": true, "role": true, "sole": true, "hole": true,
		"pole": true, "control": true, "protocol": true,
	}
	return commonWords[word]
}

// ============================================================================
// Internal: Rule-based classification fallback
// ============================================================================

func (p *claimParserImpl) ruleBasedClassification(text string) (ClaimType, float64) {
	lower := strings.ToLower(text)

	// Check for dependency references first
	deps := extractDependencyReferences(text)
	if len(deps) > 0 {
		return ClaimDependent, 0.85
	}

	// Method claim indicators
	methodPatterns := []string{
		"a method", "a process", "method for", "process for",
		"method of", "process of", "the method", "the process",
		"步骤", "方法", "工艺",
	}
	for _, p := range methodPatterns {
		if strings.Contains(lower, p) {
			return ClaimMethod, 0.75
		}
	}

	// Use claim indicators
	usePatterns := []string{
		"use of", "the use of", "a use of",
		"用途", "应用",
	}
	for _, p := range usePatterns {
		if strings.Contains(lower, p) {
			return ClaimUse, 0.70
		}
	}

	// Product claim indicators
	productPatterns := []string{
		"a composition", "a compound", "a formulation", "a device",
		"a system", "a kit", "an apparatus", "a pharmaceutical",
		"组合物", "化合物", "制剂", "装置", "系统", "试剂盒",
	}
	for _, p := range productPatterns {
		if strings.Contains(lower, p) {
			return ClaimProduct, 0.70
		}
	}

	// Default: independent
	return ClaimIndependent, 0.60
}

// ============================================================================
// Internal: Dependency tree helpers
// ============================================================================

// calculateTreeDepth computes the maximum depth of the dependency tree via BFS.
func calculateTreeDepth(roots []int, children map[int][]int) int {
	if len(roots) == 0 {
		return 0
	}

	maxDepth := 0
	type queueItem struct {
		node  int
		depth int
	}

	visited := make(map[int]bool)
	queue := make([]queueItem, 0, len(roots))
	for _, r := range roots {
		queue = append(queue, queueItem{node: r, depth: 1})
		visited[r] = true
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth > maxDepth {
			maxDepth = item.depth
		}

		for _, child := range children[item.node] {
			if !visited[child] {
				visited[child] = true
				queue = append(queue, queueItem{node: child, depth: item.depth + 1})
			}
		}
	}

	return maxDepth
}

// ============================================================================
// Utility: Cosine similarity (used by scope analysis and other callers)
// ============================================================================

// CosineSimilarity computes the cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector dimension mismatch: %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return 0, fmt.Errorf("empty vectors")
	}

	var dot, normA, normB float64
	for i := range a {
		fa := float64(a[i])
		fb := float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0 || normB == 0 {
		return 0, nil
	}

	return dot / (normA * normB), nil
}

//Personal.AI order the ending

