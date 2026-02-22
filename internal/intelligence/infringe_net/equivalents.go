package infringe_net

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// ElementType enumeration
// ---------------------------------------------------------------------------

// ElementType classifies a structural element within a molecule or claim.
type ElementType int

const (
	// ElementTypeCoreScaffold is the central ring system or backbone that determines
	// the fundamental optoelectronic character of an OLED material.
	ElementTypeCoreScaffold ElementType = iota
	// ElementTypeSubstituent is a peripheral group attached to the scaffold that fine-
	// tunes solubility, morphology, or emission wavelength.
	ElementTypeSubstituent
	// ElementTypeFunctionalGroup is a chemically reactive moiety such as -OH, -NH2,
	// -COOH that participates in specific interactions.
	ElementTypeFunctionalGroup
	// ElementTypeLinker describes how fragments are connected (e.g. ortho vs
	// para substitution, spiro junction, fused ring).
	ElementTypeLinker
	// ElementTypeBackbone represents a polymeric or oligomeric backbone.
	ElementTypeBackbone
	// ElementTypeElectronicProperty captures an abstract electronic descriptor such as
	// HOMO/LUMO gap, charge-transfer character, or spin-orbit coupling.
	ElementTypeElectronicProperty
	// ElementTypeUnknown represents an unclassified element type.
	ElementTypeUnknown
	// Aliases for compatibility if needed (deprecated)
	LinkagePattern = ElementTypeLinker
)

// String returns a human-readable label for the ElementType.
func (et ElementType) String() string {
	switch et {
	case ElementTypeUnknown:
		return "Unknown"
	case ElementTypeCoreScaffold:
		return "CoreScaffold"
	case ElementTypeSubstituent:
		return "Substituent"
	case ElementTypeFunctionalGroup:
		return "FunctionalGroup"
	case ElementTypeLinker:
		return "Linker"
	case ElementTypeBackbone:
		return "Backbone"
	case ElementTypeElectronicProperty:
		return "ElectronicProperty"
	default:
		return fmt.Sprintf("ElementType(%d)", int(et))
	}
}

// allElementTypes is the canonical list used for iteration and validation.
var allElementTypes = []ElementType{
	ElementTypeCoreScaffold,
	ElementTypeSubstituent,
	ElementTypeFunctionalGroup,
	ElementTypeLinker,
	ElementTypeBackbone,
	ElementTypeElectronicProperty,
}

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// StructuralElement is the atomic unit of comparison in an equivalents
// analysis.  A claim is decomposed into a list of these elements, and the
// accused molecule is likewise decomposed so that element-wise FWR testing
// can proceed.
type StructuralElement struct {
	ElementID      string            `json:"element_id"`
	ElementType    ElementType       `json:"element_type"`
	Description    string            `json:"description"`
	SMILESFragment string            `json:"smiles_fragment,omitempty"`
	SMILES         string            `json:"smiles,omitempty"` // Alias for SMILESFragment or full SMILES
	Role           string            `json:"role,omitempty"`
	Position       string            `json:"position,omitempty"`
	Weight         float64           `json:"weight,omitempty"`
	Properties     map[string]string `json:"properties,omitempty"`
}

// ProsecutionHistoryEntry records a single event from the patent prosecution
// file wrapper that may trigger prosecution-history estoppel.
type ProsecutionHistoryEntry struct {
	EventType        string `json:"event_type"`          // e.g. "amendment", "argument", "restriction"
	AbandonedScope   string `json:"abandoned_scope"`     // human-readable description
	AbandonedSMILES  string `json:"abandoned_smiles,omitempty"`
	AbandonedType    ElementType `json:"abandoned_type"`
	Reason           string `json:"reason,omitempty"`
}

// EquivalentsRequest is the input to a full equivalents analysis.
type EquivalentsRequest struct {
	QueryMolecule      []*StructuralElement       `json:"query_molecule"`
	ClaimElements      []*StructuralElement       `json:"claim_elements"`
	ProsecutionHistory []*ProsecutionHistoryEntry  `json:"prosecution_history,omitempty"`
}

// EquivalentsResult is the output of a full equivalents analysis.
type EquivalentsResult struct {
	OverallEquivalenceScore float64              `json:"overall_equivalence_score"`
	ElementResults          []*ElementEquivalence `json:"element_results"`
	EquivalentElementCount  int                   `json:"equivalent_element_count"`
	TotalElementCount       int                   `json:"total_element_count"`
	NonEquivalentElements   []*NonEquivalentInfo  `json:"non_equivalent_elements,omitempty"`
}

// NonEquivalentInfo records why a particular element pair failed the FWR
// test.
type NonEquivalentInfo struct {
	QueryElement *StructuralElement `json:"query_element"`
	ClaimElement *StructuralElement `json:"claim_element"`
	FailedStep   string             `json:"failed_step"` // "function", "way", "result", "estoppel"
	Reason       string             `json:"reason"`
}

// ElementEquivalence is the result of a single element-pair FWR test.
type ElementEquivalence struct {
	QueryElement *StructuralElement `json:"query_element"`
	ClaimElement *StructuralElement `json:"claim_element"`
	FunctionScore float64           `json:"function_score"`
	WayScore      float64           `json:"way_score"`
	ResultScore   float64           `json:"result_score"`
	OverallScore  float64           `json:"overall_score"`
	IsEquivalent  bool              `json:"is_equivalent"`
	Reasoning     string            `json:"reasoning"`
	// Internal flags indicating which steps were actually executed.
	functionEvaluated bool
	wayEvaluated      bool
	resultEvaluated   bool
}

// ---------------------------------------------------------------------------
// EquivalentsModel — dependency interface
// ---------------------------------------------------------------------------

// EquivalentsModel abstracts the neural model used for structural similarity
// scoring within the infringement analysis pipeline.
type EquivalentsModel interface {
	// ComputeFunctionSimilarity scores how similar two elements are in the
	// functional role they play inside their respective molecules.
	ComputeFunctionSimilarity(ctx context.Context, a, b *StructuralElement) (float64, error)
	// ComputeWaySimilarity scores how similar the chemical mechanisms are
	// by which two elements achieve their function.
	ComputeWaySimilarity(ctx context.Context, a, b *StructuralElement) (float64, error)
	// ComputeResultSimilarity scores how similar the downstream effects of
	// two elements are on overall molecular performance.
	ComputeResultSimilarity(ctx context.Context, a, b *StructuralElement) (float64, error)
}

// ---------------------------------------------------------------------------
// Logger — dependency interface (minimal)
// ---------------------------------------------------------------------------

// Logger is a minimal structured logger interface.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// ---------------------------------------------------------------------------
// EquivalentsAnalyzer interface
// ---------------------------------------------------------------------------

// EquivalentsAnalyzer performs Doctrine-of-Equivalents analysis on molecule /
// claim element pairs using the Function-Way-Result tripartite test.
type EquivalentsAnalyzer interface {
	// Analyze runs a full equivalents analysis for a query molecule against
	// a set of claim elements.
	Analyze(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error)
	// AnalyzeElement runs the FWR test on a single element pair.
	AnalyzeElement(ctx context.Context, queryElement, claimElement *StructuralElement) (*ElementEquivalence, error)
	ModelVersion() string
}

func (a *equivalentsAnalyzer) ModelVersion() string {
	return "equivalents-v1"
}

// ---------------------------------------------------------------------------
// EquivalentsOption — functional options
// ---------------------------------------------------------------------------

type equivalentsConfig struct {
	functionThreshold float64
	wayThreshold      float64
	resultThreshold   float64
	scaffoldWeight    float64
}

func defaultEquivalentsConfig() *equivalentsConfig {
	return &equivalentsConfig{
		functionThreshold: 0.7,
		wayThreshold:      0.6,
		resultThreshold:   0.65,
		scaffoldWeight:    2.0,
	}
}

// EquivalentsOption configures the equivalents analyzer.
type EquivalentsOption func(*equivalentsConfig)

// WithFunctionThreshold sets the minimum score for the Function step.
func WithFunctionThreshold(t float64) EquivalentsOption {
	return func(c *equivalentsConfig) {
		if t >= 0 && t <= 1 {
			c.functionThreshold = t
		}
	}
}

// WithWayThreshold sets the minimum score for the Way step.
func WithWayThreshold(t float64) EquivalentsOption {
	return func(c *equivalentsConfig) {
		if t >= 0 && t <= 1 {
			c.wayThreshold = t
		}
	}
}

// WithResultThreshold sets the minimum score for the Result step.
func WithResultThreshold(t float64) EquivalentsOption {
	return func(c *equivalentsConfig) {
		if t >= 0 && t <= 1 {
			c.resultThreshold = t
		}
	}
}

// WithScaffoldWeight sets the weight multiplier for CoreScaffold elements
// when computing the overall equivalence score.
func WithScaffoldWeight(w float64) EquivalentsOption {
	return func(c *equivalentsConfig) {
		if w > 0 {
			c.scaffoldWeight = w
		}
	}
}

// ---------------------------------------------------------------------------
// equivalentsAnalyzer implementation
// ---------------------------------------------------------------------------

type equivalentsAnalyzer struct {
	model  EquivalentsModel
	logger Logger
	cfg    *equivalentsConfig
}

// NewEquivalentsAnalyzer constructs an EquivalentsAnalyzer.
func NewEquivalentsAnalyzer(model EquivalentsModel, logger Logger, opts ...EquivalentsOption) (EquivalentsAnalyzer, error) {
	if model == nil {
		return nil, errors.NewInvalidInputError("EquivalentsModel is required")
	}
	if logger == nil {
		logger = &noopLogger{} // Assuming noopLogger is defined in mapper.go or similar
	}
	cfg := defaultEquivalentsConfig()
	for _, o := range opts {
		o(cfg)
	}
	return &equivalentsAnalyzer{
		model:  model,
		logger: logger,
		cfg:    cfg,
	}, nil
}

// ---------- Analyze (full) --------------------------------------------------

func (a *equivalentsAnalyzer) Analyze(ctx context.Context, req *EquivalentsRequest) (*EquivalentsResult, error) {
	if req == nil {
		return nil, errors.NewInvalidInputError("request is nil")
	}
	if len(req.QueryMolecule) == 0 {
		return nil, errors.NewInvalidInputError("query molecule elements are empty")
	}
	if len(req.ClaimElements) == 0 {
		return nil, errors.NewInvalidInputError("claim elements are empty")
	}

	// 1. Build the estoppel index from prosecution history.
	estoppelIndex := a.buildEstoppelIndex(req.ProsecutionHistory)

	// 2. Align query elements to claim elements by type, then by greedy
	//    structural similarity within each type bucket.
	pairs := a.alignElements(ctx, req.QueryMolecule, req.ClaimElements)

	// 3. Run FWR on each aligned pair.
	var (
		elementResults []*ElementEquivalence
		nonEquiv       []*NonEquivalentInfo
		equivCount     int
	)

	for _, p := range pairs {
		// Prosecution-history estoppel check.
		if blocked, reason := a.checkEstoppel(p.query, estoppelIndex); blocked {
			eq := &ElementEquivalence{
				QueryElement: p.query,
				ClaimElement: p.claim,
				IsEquivalent: false,
				Reasoning:    fmt.Sprintf("Blocked by prosecution-history estoppel: %s", reason),
			}
			elementResults = append(elementResults, eq)
			nonEquiv = append(nonEquiv, &NonEquivalentInfo{
				QueryElement: p.query,
				ClaimElement: p.claim,
				FailedStep:   "estoppel",
				Reason:       reason,
			})
			continue
		}

		eq, err := a.AnalyzeElement(ctx, p.query, p.claim)
		if err != nil {
			a.logger.Error("AnalyzeElement failed", "query", p.query.ElementID, "claim", p.claim.ElementID, "error", err)
			eq = &ElementEquivalence{
				QueryElement: p.query,
				ClaimElement: p.claim,
				IsEquivalent: false,
				Reasoning:    fmt.Sprintf("analysis error: %v", err),
			}
		}
		elementResults = append(elementResults, eq)
		if eq.IsEquivalent {
			equivCount++
		} else {
			failedStep := determineFailedStep(eq)
			nonEquiv = append(nonEquiv, &NonEquivalentInfo{
				QueryElement: p.query,
				ClaimElement: p.claim,
				FailedStep:   failedStep,
				Reason:       eq.Reasoning,
			})
		}
	}

	totalCount := len(pairs)
	overallScore := a.computeOverallScore(elementResults, totalCount)

	return &EquivalentsResult{
		OverallEquivalenceScore: overallScore,
		ElementResults:          elementResults,
		EquivalentElementCount:  equivCount,
		TotalElementCount:       totalCount,
		NonEquivalentElements:   nonEquiv,
	}, nil
}

// ---------- AnalyzeElement (FWR) --------------------------------------------

func (a *equivalentsAnalyzer) AnalyzeElement(
	ctx context.Context,
	queryElement, claimElement *StructuralElement,
) (*ElementEquivalence, error) {
	if queryElement == nil || claimElement == nil {
		return nil, errors.NewInvalidInputError("both query and claim elements are required")
	}

	eq := &ElementEquivalence{
		QueryElement: queryElement,
		ClaimElement: claimElement,
	}

	// --- Step 1: Function ---------------------------------------------------
	funcScore, err := a.model.ComputeFunctionSimilarity(ctx, queryElement, claimElement)
	if err != nil {
		return nil, fmt.Errorf("function similarity: %w", err)
	}
	eq.FunctionScore = clampScore(funcScore)
	eq.functionEvaluated = true

	if eq.FunctionScore < a.cfg.functionThreshold {
		eq.IsEquivalent = false
		eq.OverallScore = eq.FunctionScore * fwrWeight("function")
		eq.Reasoning = fmt.Sprintf(
			"Function test failed: score %.4f < threshold %.4f. "+
				"The query element [%s] and claim element [%s] serve substantially different functional roles.",
			eq.FunctionScore, a.cfg.functionThreshold,
			queryElement.Description, claimElement.Description,
		)
		return eq, nil
	}

	// --- Step 2: Way --------------------------------------------------------
	wayScore, err := a.model.ComputeWaySimilarity(ctx, queryElement, claimElement)
	if err != nil {
		return nil, fmt.Errorf("way similarity: %w", err)
	}
	eq.WayScore = clampScore(wayScore)
	eq.wayEvaluated = true

	if eq.WayScore < a.cfg.wayThreshold {
		eq.IsEquivalent = false
		eq.OverallScore = (eq.FunctionScore*fwrWeight("function") + eq.WayScore*fwrWeight("way")) /
			(fwrWeight("function") + fwrWeight("way"))
		eq.Reasoning = fmt.Sprintf(
			"Way test failed: score %.4f < threshold %.4f. "+
				"Although the elements share a similar function (%.4f), "+
				"the chemical mechanism differs: query [%s] vs claim [%s].",
			eq.WayScore, a.cfg.wayThreshold,
			eq.FunctionScore,
			queryElement.Description, claimElement.Description,
		)
		return eq, nil
	}

	// --- Step 3: Result -----------------------------------------------------
	resScore, err := a.model.ComputeResultSimilarity(ctx, queryElement, claimElement)
	if err != nil {
		return nil, fmt.Errorf("result similarity: %w", err)
	}
	eq.ResultScore = clampScore(resScore)
	eq.resultEvaluated = true

	if eq.ResultScore < a.cfg.resultThreshold {
		eq.IsEquivalent = false
		eq.OverallScore = (eq.FunctionScore*fwrWeight("function") +
			eq.WayScore*fwrWeight("way") +
			eq.ResultScore*fwrWeight("result")) /
			(fwrWeight("function") + fwrWeight("way") + fwrWeight("result"))
		eq.Reasoning = fmt.Sprintf(
			"Result test failed: score %.4f < threshold %.4f. "+
				"Function (%.4f) and Way (%.4f) are substantially similar, "+
				"but the downstream effect on molecular performance differs: "+
				"query [%s] vs claim [%s].",
			eq.ResultScore, a.cfg.resultThreshold,
			eq.FunctionScore, eq.WayScore,
			queryElement.Description, claimElement.Description,
		)
		return eq, nil
	}

	// --- All three steps passed ---------------------------------------------
	eq.IsEquivalent = true
	eq.OverallScore = (eq.FunctionScore*fwrWeight("function") +
		eq.WayScore*fwrWeight("way") +
		eq.ResultScore*fwrWeight("result")) /
		(fwrWeight("function") + fwrWeight("way") + fwrWeight("result"))
	eq.Reasoning = fmt.Sprintf(
		"All three FWR steps passed. Function=%.4f (≥%.4f), Way=%.4f (≥%.4f), Result=%.4f (≥%.4f). "+
			"The query element [%s] is equivalent to claim element [%s] under the Doctrine of Equivalents.",
		eq.FunctionScore, a.cfg.functionThreshold,
		eq.WayScore, a.cfg.wayThreshold,
		eq.ResultScore, a.cfg.resultThreshold,
		queryElement.Description, claimElement.Description,
	)
	return eq, nil
}

// ---------------------------------------------------------------------------
// Element alignment
// ---------------------------------------------------------------------------

type elementPair struct {
	query *StructuralElement
	claim *StructuralElement
}

// alignElements pairs query elements with claim elements.  The strategy is:
//  1. Group both sides by ElementType.
//  2. Within each type bucket, greedily match by description similarity
//     (simple Jaccard on whitespace-tokenised descriptions).
//  3. Unmatched claim elements are paired with the closest remaining query
//     element regardless of type (fallback).
func (a *equivalentsAnalyzer) alignElements(
	ctx context.Context,
	query []*StructuralElement,
	claim []*StructuralElement,
) []elementPair {
	queryByType := groupByType(query)
	claimByType := groupByType(claim)

	usedQuery := make(map[string]bool)
	usedClaim := make(map[string]bool)
	var pairs []elementPair

	// Phase 1: same-type greedy matching.
	for _, et := range allElementTypes {
		qs := queryByType[et]
		cs := claimByType[et]
		matched := greedyMatch(qs, cs, usedQuery, usedClaim)
		pairs = append(pairs, matched...)
	}

	// Phase 2: unmatched claim elements get the best remaining query element.
	for _, c := range claim {
		if usedClaim[c.ElementID] {
			continue
		}
		bestQ := bestUnusedQuery(query, c, usedQuery)
		if bestQ != nil {
			usedQuery[bestQ.ElementID] = true
			usedClaim[c.ElementID] = true
			pairs = append(pairs, elementPair{query: bestQ, claim: c})
		}
	}

	return pairs
}

func groupByType(elems []*StructuralElement) map[ElementType][]*StructuralElement {
	m := make(map[ElementType][]*StructuralElement)
	for _, e := range elems {
		m[e.ElementType] = append(m[e.ElementType], e)
	}
	return m
}

func greedyMatch(
	qs, cs []*StructuralElement,
	usedQ, usedC map[string]bool,
) []elementPair {
	type scored struct {
		q   *StructuralElement
		c   *StructuralElement
		sim float64
	}
	var candidates []scored
	for _, q := range qs {
		if usedQ[q.ElementID] {
			continue
		}
		for _, c := range cs {
			if usedC[c.ElementID] {
				continue
			}
			sim := descriptionSimilarity(q.Description, c.Description)
			candidates = append(candidates, scored{q: q, c: c, sim: sim})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].sim > candidates[j].sim
	})

	var pairs []elementPair
	for _, cand := range candidates {
		if usedQ[cand.q.ElementID] || usedC[cand.c.ElementID] {
			continue
		}
		usedQ[cand.q.ElementID] = true
		usedC[cand.c.ElementID] = true
		pairs = append(pairs, elementPair{query: cand.q, claim: cand.c})
	}
	return pairs
}

func bestUnusedQuery(all []*StructuralElement, target *StructuralElement, used map[string]bool) *StructuralElement {
	var best *StructuralElement
	bestSim := -1.0
	for _, q := range all {
		if used[q.ElementID] {
			continue
		}
		sim := descriptionSimilarity(q.Description, target.Description)
		if sim > bestSim {
			bestSim = sim
			best = q
		}
	}
	return best
}

// descriptionSimilarity computes a simple Jaccard similarity over
// whitespace-tokenised, lowercased descriptions.
func descriptionSimilarity(a, b string) float64 {
	tokA := tokenize(a)
	tokB := tokenize(b)
	if len(tokA) == 0 && len(tokB) == 0 {
		return 1.0
	}
	inter := 0
	setB := make(map[string]bool, len(tokB))
	for _, t := range tokB {
		setB[t] = true
	}
	for _, t := range tokA {
		if setB[t] {
			inter++
		}
	}
	union := len(tokA) + len(tokB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenize(s string) []string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

// ---------------------------------------------------------------------------
// Prosecution-history estoppel
// ---------------------------------------------------------------------------

type estoppelEntry struct {
	elementType ElementType
	scope       string
	smiles      string
	reason      string
}

func (a *equivalentsAnalyzer) buildEstoppelIndex(history []*ProsecutionHistoryEntry) []estoppelEntry {
	if len(history) == 0 {
		return nil
	}
	idx := make([]estoppelEntry, 0, len(history))
	for _, h := range history {
		idx = append(idx, estoppelEntry{
			elementType: h.AbandonedType,
			scope:       strings.ToLower(h.AbandonedScope),
			smiles:      h.AbandonedSMILES,
			reason:      h.Reason,
		})
	}
	return idx
}

func (a *equivalentsAnalyzer) checkEstoppel(elem *StructuralElement, index []estoppelEntry) (bool, string) {
	if len(index) == 0 || elem == nil {
		return false, ""
	}
	descLower := strings.ToLower(elem.Description)
	smilesLower := strings.ToLower(elem.SMILESFragment)

	for _, e := range index {
		// Type must match.
		if e.elementType != elem.ElementType {
			continue
		}
		// Check scope overlap via substring or SMILES match.
		scopeMatch := e.scope != "" && strings.Contains(descLower, e.scope)
		smilesMatch := e.smiles != "" && smilesLower != "" && strings.Contains(smilesLower, strings.ToLower(e.smiles))
		if scopeMatch || smilesMatch {
			reason := fmt.Sprintf(
				"Applicant surrendered scope covering '%s' during prosecution",
				e.scope,
			)
			if e.reason != "" {
				reason += ": " + e.reason
			}
			return true, reason
		}
	}
	return false, ""
}

// ---------------------------------------------------------------------------
// Overall score computation
// ---------------------------------------------------------------------------

func (a *equivalentsAnalyzer) computeOverallScore(results []*ElementEquivalence, total int) float64 {
	if total == 0 {
		return 0
	}
	weightedSum := 0.0
	weightTotal := 0.0

	for _, r := range results {
		w := a.elementWeight(r.QueryElement.ElementType)
		if r.IsEquivalent {
			weightedSum += w * 1.0
		}
		weightTotal += w
	}
	if weightTotal == 0 {
		return 0
	}
	return clampScore(weightedSum / weightTotal)
}

func (a *equivalentsAnalyzer) elementWeight(et ElementType) float64 {
	switch et {
	case ElementTypeCoreScaffold:
		return a.cfg.scaffoldWeight
	case ElementTypeSubstituent:
		return 0.8
	case ElementTypeFunctionalGroup:
		return 1.0
	case ElementTypeLinker: // Was LinkagePattern
		return 1.0
	case ElementTypeElectronicProperty:
		return 1.0
	default:
		return 1.0
	}
}

// ---------------------------------------------------------------------------
// FWR step weights (for computing OverallScore within a single element pair)
// ---------------------------------------------------------------------------

func fwrWeight(step string) float64 {
	switch step {
	case "function":
		return 0.40
	case "way":
		return 0.30
	case "result":
		return 0.30
	default:
		return 1.0
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func clampScore(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func determineFailedStep(eq *ElementEquivalence) string {
	if eq.functionEvaluated && eq.FunctionScore < 0.5 {
		return "function"
	}
	if eq.wayEvaluated && eq.WayScore < 0.5 {
		return "way"
	}
	if eq.resultEvaluated {
		return "result"
	}
	if !eq.wayEvaluated {
		return "function"
	}
	if !eq.resultEvaluated {
		return "way"
	}
	return "unknown"
}

