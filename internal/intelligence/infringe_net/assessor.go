package infringe_net

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// RiskLevel enumeration
// ---------------------------------------------------------------------------

// RiskLevel classifies the overall infringement risk.
type RiskLevel int

const (
	RiskNone     RiskLevel = iota // < 0.30
	RiskLow                       // >= 0.30
	RiskMedium                    // >= 0.50
	RiskHigh                      // >= 0.70
	RiskCritical                  // >= 0.85
)

var riskLevelNames = map[RiskLevel]string{
	RiskNone:     "NONE",
	RiskLow:      "LOW",
	RiskMedium:   "MEDIUM",
	RiskHigh:     "HIGH",
	RiskCritical: "CRITICAL",
}

func (r RiskLevel) String() string {
	if s, ok := riskLevelNames[r]; ok {
		return s
	}
	return "UNKNOWN"
}

// MarshalJSON serialises RiskLevel as a JSON string.
func (r RiskLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

// UnmarshalJSON deserialises a JSON string into RiskLevel.
func (r *RiskLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for k, v := range riskLevelNames {
		if v == s {
			*r = k
			return nil
		}
	}
	return fmt.Errorf("unknown risk level: %s", s)
}

// ClassifyRisk maps a score in [0,1] to a RiskLevel.
func ClassifyRisk(score float64) RiskLevel {
	switch {
	case score >= 0.85:
		return RiskCritical
	case score >= 0.70:
		return RiskHigh
	case score >= 0.50:
		return RiskMedium
	case score >= 0.30:
		return RiskLow
	default:
		return RiskNone
	}
}

// ---------------------------------------------------------------------------
// Scoring weights (reflecting patent-law practice)
// ---------------------------------------------------------------------------

const (
	weightLiteral     = 0.50
	weightEquivalents = 0.35
	weightEstoppel    = 0.15 // penalty coefficient

	literalShortCircuitThreshold = 0.90
)

// ---------------------------------------------------------------------------
// Molecule / Claim input types
// ---------------------------------------------------------------------------

// MoleculeInput describes the target molecule under analysis.
type MoleculeInput struct {
	SMILES      string `json:"smiles,omitempty"`
	InChI       string `json:"inchi,omitempty"`
	Fingerprint []byte `json:"fingerprint,omitempty"`
	Name        string `json:"name,omitempty"`
}

// Validate checks that at least one molecular representation is present.
func (m *MoleculeInput) Validate() error {
	if m == nil {
		return errors.NewInvalidInputError("molecule input is nil")
	}
	if m.SMILES == "" && m.InChI == "" && len(m.Fingerprint) == 0 {
		return errors.NewInvalidInputError("at least one molecular representation (SMILES, InChI, or fingerprint) is required")
	}
	return nil
}

// ClaimInput represents a single patent claim to compare against.
type ClaimInput struct {
	ClaimID     string   `json:"claim_id"`
	ClaimText   string   `json:"claim_text"`
	ClaimType   string   `json:"claim_type"` // "independent" or "dependent"
	ParentID    string   `json:"parent_id,omitempty"`
	Elements    []string `json:"elements,omitempty"`
	PatentID    string   `json:"patent_id,omitempty"`
}

// ---------------------------------------------------------------------------
// Assessment options (functional option pattern)
// ---------------------------------------------------------------------------

// AssessmentOptions holds configurable parameters for an assessment run.
type AssessmentOptions struct {
	EnableEquivalents    bool
	EnableEstoppelCheck  bool
	ConfidenceThreshold  float64
	MaxConcurrency       int
	Timeout              time.Duration
}

// DefaultAssessmentOptions returns production defaults.
func DefaultAssessmentOptions() *AssessmentOptions {
	return &AssessmentOptions{
		EnableEquivalents:   true,
		EnableEstoppelCheck: true,
		ConfidenceThreshold: 0.50,
		MaxConcurrency:      8,
		Timeout:             30 * time.Second,
	}
}

// AssessmentOption is a functional option that mutates AssessmentOptions.
type AssessmentOption func(*AssessmentOptions)

// WithEquivalentsAnalysis toggles the doctrine-of-equivalents path.
func WithEquivalentsAnalysis(enabled bool) AssessmentOption {
	return func(o *AssessmentOptions) { o.EnableEquivalents = enabled }
}

// WithEstoppelCheck toggles prosecution-history estoppel detection.
func WithEstoppelCheck(enabled bool) AssessmentOption {
	return func(o *AssessmentOptions) { o.EnableEstoppelCheck = enabled }
}

// WithConfidenceThreshold sets the minimum confidence for a result to be
// considered meaningful.
func WithConfidenceThreshold(threshold float64) AssessmentOption {
	return func(o *AssessmentOptions) {
		if threshold > 0 && threshold <= 1 {
			o.ConfidenceThreshold = threshold
		}
	}
}

// WithMaxConcurrency caps the number of parallel assessments in batch mode.
func WithMaxConcurrency(n int) AssessmentOption {
	return func(o *AssessmentOptions) {
		if n > 0 {
			o.MaxConcurrency = n
		}
	}
}

// WithTimeout sets the per-assessment timeout.
func WithTimeout(d time.Duration) AssessmentOption {
	return func(o *AssessmentOptions) {
		if d > 0 {
			o.Timeout = d
		}
	}
}

func applyOptions(opts []AssessmentOption) *AssessmentOptions {
	o := DefaultAssessmentOptions()
	for _, fn := range opts {
		fn(o)
	}
	return o
}

// ---------------------------------------------------------------------------
// Request / Response structures
// ---------------------------------------------------------------------------

// AssessmentRequest is the input for a single infringement assessment.
type AssessmentRequest struct {
	RequestID string          `json:"request_id,omitempty"`
	Molecule  *MoleculeInput  `json:"molecule"`
	Claims    []*ClaimInput   `json:"claims"`
	Options   []AssessmentOption `json:"-"`
}

// ClaimMatchResult captures the per-claim analysis outcome.
type ClaimMatchResult struct {
	ClaimID          string    `json:"claim_id"`
	LiteralScore     float64   `json:"literal_score"`
	EquivalentsScore float64   `json:"equivalents_score"`
	EstoppelPenalty  float64   `json:"estoppel_penalty"`
	CombinedScore    float64   `json:"combined_score"`
	RiskLevel        RiskLevel `json:"risk_level"`
	MatchedElements  []string  `json:"matched_elements,omitempty"`
	MissedElements   []string  `json:"missed_elements,omitempty"`
}

// LiteralAnalysisResult is the sub-result from literal infringement analysis.
type LiteralAnalysisResult struct {
	Score           float64            `json:"score"`
	ElementScores   map[string]float64 `json:"element_scores,omitempty"`
	AllElementsMet  bool               `json:"all_elements_met"`
	Confidence      float64            `json:"confidence"`
	ModelVersion    string             `json:"model_version,omitempty"`
}

// EquivalentsAnalysisResult is the sub-result from doctrine-of-equivalents.
type EquivalentsAnalysisResult struct {
	Score        float64            `json:"score"`
	FWRScores    map[string]float64 `json:"fwr_scores,omitempty"` // function, way, result
	Confidence   float64            `json:"confidence"`
	ModelVersion string             `json:"model_version,omitempty"`
	Skipped      bool               `json:"skipped,omitempty"`
	SkipReason   string             `json:"skip_reason,omitempty"`
}

// EstoppelCheckResult is the sub-result from prosecution-history estoppel.
type EstoppelCheckResult struct {
	HasEstoppel     bool               `json:"has_estoppel"`
	PenaltyScore    float64            `json:"penalty_score"`
	Amendments      []string           `json:"amendments,omitempty"`
	Confidence      float64            `json:"confidence"`
	Skipped         bool               `json:"skipped,omitempty"`
	SkipReason      string             `json:"skip_reason,omitempty"`
}

// AssessmentResult is the comprehensive output of a single assessment.
type AssessmentResult struct {
	RequestID          string                     `json:"request_id"`
	OverallRiskLevel   RiskLevel                  `json:"overall_risk_level"`
	OverallScore       float64                    `json:"overall_score"`
	LiteralAnalysis    *LiteralAnalysisResult     `json:"literal_analysis"`
	EquivalentsAnalysis *EquivalentsAnalysisResult `json:"equivalents_analysis"`
	EstoppelCheck      *EstoppelCheckResult        `json:"estoppel_check"`
	MatchedClaims      []*ClaimMatchResult         `json:"matched_claims"`
	Confidence         float64                    `json:"confidence"`
	Degraded           bool                       `json:"degraded"`
	DegradedReason     string                     `json:"degraded_reason,omitempty"`
	ProcessingTimeMs   int64                      `json:"processing_time_ms"`
	ModelVersions      map[string]string          `json:"model_versions"`
	Error              string                     `json:"error,omitempty"`
}

// PortfolioAssessmentResult aggregates results across a patent portfolio.
type PortfolioAssessmentResult struct {
	Molecule            *MoleculeInput             `json:"molecule"`
	PortfolioID         string                     `json:"portfolio_id"`
	TotalPatentsScanned int                        `json:"total_patents_scanned"`
	RiskDistribution    map[RiskLevel]int          `json:"risk_distribution"`
	TopRisks            []*AssessmentResult         `json:"top_risks"`
	ScanDurationMs      int64                      `json:"scan_duration_ms"`
}

// AssessmentExplanation provides human-readable interpretation.
type AssessmentExplanation struct {
	ResultID                string   `json:"result_id"`
	NaturalLanguageSummary  string   `json:"natural_language_summary"`
	ElementByElementBreakdown []ElementBreakdownItem `json:"element_by_element_breakdown"`
	KeyFactors              []string `json:"key_factors"`
	SuggestedActions        []string `json:"suggested_actions"`
}

// ElementBreakdownItem is one row in the element-by-element comparison.
type ElementBreakdownItem struct {
	Element       string  `json:"element"`
	ClaimText     string  `json:"claim_text"`
	MoleculeMatch string  `json:"molecule_match"`
	Score         float64 `json:"score"`
	Method        string  `json:"method"` // "literal" | "equivalents"
}

// ---------------------------------------------------------------------------
// Dependency interfaces (defined in sibling files)
// ---------------------------------------------------------------------------

// InfringeModel is the neural model that predicts literal infringement.
type InfringeModel interface {
	PredictLiteralInfringement(ctx context.Context, mol *MoleculeInput, elements []string) (*LiteralAnalysisResult, error)
	ModelVersion() string
}

// EquivalentsAnalyzer performs doctrine-of-equivalents analysis.
type EquivalentsAnalyzer interface {
	Analyze(ctx context.Context, mol *MoleculeInput, elements []string) (*EquivalentsAnalysisResult, error)
	ModelVersion() string
}

// ClaimElementMapper decomposes claims into structural elements and checks estoppel.
type ClaimElementMapper interface {
	MapElements(ctx context.Context, claims []*ClaimInput) (map[string][]string, error)
	CheckEstoppel(ctx context.Context, claims []*ClaimInput) (*EstoppelCheckResult, error)
	LoadIndependentClaims(ctx context.Context, dependentClaims []*ClaimInput) ([]*ClaimInput, error)
}

// PortfolioLoader retrieves all claims belonging to a portfolio.
type PortfolioLoader interface {
	LoadPortfolioClaims(ctx context.Context, portfolioID string) ([]*ClaimInput, error)
}

// ExplanationStore persists and retrieves assessment results for later explanation.
type ExplanationStore interface {
	Store(ctx context.Context, result *AssessmentResult) error
	Load(ctx context.Context, resultID string) (*AssessmentResult, error)
}

// ExplanationGenerator produces natural-language explanations.
type ExplanationGenerator interface {
	Generate(ctx context.Context, result *AssessmentResult) (*AssessmentExplanation, error)
}

// ---------------------------------------------------------------------------
// InfringementAssessor interface
// ---------------------------------------------------------------------------

// InfringementAssessor is the top-level orchestrator for infringement analysis.
type InfringementAssessor interface {
	Assess(ctx context.Context, req *AssessmentRequest) (*AssessmentResult, error)
	BatchAssess(ctx context.Context, reqs []*AssessmentRequest) ([]*AssessmentResult, error)
	AssessAgainstPortfolio(ctx context.Context, molecule *MoleculeInput, portfolioID string) (*PortfolioAssessmentResult, error)
	ExplainAssessment(ctx context.Context, resultID string) (*AssessmentExplanation, error)
}

// ---------------------------------------------------------------------------
// infringementAssessor implementation
// ---------------------------------------------------------------------------

type infringementAssessor struct {
	model          InfringeModel
	equivalents    EquivalentsAnalyzer
	mapper         ClaimElementMapper
	portfolio      PortfolioLoader
	explStore      ExplanationStore
	explGenerator  ExplanationGenerator
	metrics        common.IntelligenceMetrics
	logger         common.Logger
	defaultOpts    *AssessmentOptions
}

// NewInfringementAssessor constructs the assessor with all required dependencies.
func NewInfringementAssessor(
	model InfringeModel,
	equivalents EquivalentsAnalyzer,
	mapper ClaimElementMapper,
	portfolio PortfolioLoader,
	explStore ExplanationStore,
	explGenerator ExplanationGenerator,
	metrics common.IntelligenceMetrics,
	logger common.Logger,
	opts ...AssessmentOption,
) (InfringementAssessor, error) {
	if model == nil {
		return nil, errors.NewInvalidInputError("InfringeModel is required")
	}
	if equivalents == nil {
		return nil, errors.NewInvalidInputError("EquivalentsAnalyzer is required")
	}
	if mapper == nil {
		return nil, errors.NewInvalidInputError("ClaimElementMapper is required")
	}
	if metrics == nil {
		metrics = common.NewNoopIntelligenceMetrics()
	}
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	return &infringementAssessor{
		model:         model,
		equivalents:   equivalents,
		mapper:        mapper,
		portfolio:     portfolio,
		explStore:     explStore,
		explGenerator: explGenerator,
		metrics:       metrics,
		logger:        logger,
		defaultOpts:   applyOptions(opts),
	}, nil
}

// -------------------------------------------------------------------------
// Assess — single infringement assessment
// -------------------------------------------------------------------------

func (a *infringementAssessor) Assess(ctx context.Context, req *AssessmentRequest) (*AssessmentResult, error) {
	if req == nil {
		return nil, errors.NewInvalidInputError("assessment request is nil")
	}
	if err := req.Molecule.Validate(); err != nil {
		return nil, err
	}
	if len(req.Claims) == 0 {
		return nil, errors.NewInvalidInputError("at least one claim is required")
	}

	opts := applyOptions(req.Options)
	start := time.Now()

	// Apply per-assessment timeout.
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Ensure independent claims are present when only dependents are supplied.
	claims, err := a.ensureIndependentClaims(ctx, req.Claims)
	if err != nil {
		a.logger.Warn("failed to load independent claims, proceeding with originals", "error", err)
		claims = req.Claims
	}

	// Decompose claims into elements.
	elementMap, err := a.mapper.MapElements(ctx, claims)
	if err != nil {
		return nil, fmt.Errorf("claim element mapping failed: %w", err)
	}

	// Flatten all elements for model input.
	allElements := flattenElements(elementMap)

	// ---- Parallel evaluation paths ----
	var (
		literalResult     *LiteralAnalysisResult
		equivalentsResult *EquivalentsAnalysisResult
		estoppelResult    *EstoppelCheckResult
		literalErr        error
		equivalentsErr    error
		estoppelErr       error
		wg                sync.WaitGroup
	)

	// Path A: Literal infringement (always runs).
	wg.Add(1)
	go func() {
		defer wg.Done()
		literalResult, literalErr = a.model.PredictLiteralInfringement(ctx, req.Molecule, allElements)
	}()

	// Path C: Estoppel check (conditional).
	if opts.EnableEstoppelCheck {
		wg.Add(1)
		go func() {
			defer wg.Done()
			estoppelResult, estoppelErr = a.mapper.CheckEstoppel(ctx, claims)
		}()
	}

	// Wait for literal before deciding on equivalents.
	wg.Wait()

	if literalErr != nil {
		return nil, fmt.Errorf("literal infringement analysis failed: %w", literalErr)
	}

	// Short-circuit: if literal score >= 0.9, skip equivalents.
	shortCircuited := false
	if literalResult.Score >= literalShortCircuitThreshold {
		shortCircuited = true
		equivalentsResult = &EquivalentsAnalysisResult{
			Score:      0,
			Skipped:    true,
			SkipReason: fmt.Sprintf("literal score %.3f >= %.2f, equivalents analysis unnecessary", literalResult.Score, literalShortCircuitThreshold),
			Confidence: 1.0,
		}
	} else if opts.EnableEquivalents {
		equivalentsResult, equivalentsErr = a.equivalents.Analyze(ctx, req.Molecule, allElements)
	}

	// Build default sub-results for disabled / skipped paths.
	if equivalentsResult == nil && equivalentsErr == nil {
		equivalentsResult = &EquivalentsAnalysisResult{Skipped: true, SkipReason: "equivalents analysis disabled"}
	}
	if estoppelResult == nil && estoppelErr == nil {
		estoppelResult = &EstoppelCheckResult{Skipped: true, SkipReason: "estoppel check disabled or not run"}
	}

	// Handle degraded mode: equivalents failure.
	degraded := false
	degradedReason := ""
	if equivalentsErr != nil {
		degraded = true
		degradedReason = fmt.Sprintf("equivalents analysis failed: %v", equivalentsErr)
		equivalentsResult = &EquivalentsAnalysisResult{Score: 0, Skipped: true, SkipReason: degradedReason, Confidence: 0}
		a.logger.Warn("equivalents analysis failed, degrading to literal-only", "error", equivalentsErr)
	}
	if estoppelErr != nil {
		if !degraded {
			degraded = true
			degradedReason = fmt.Sprintf("estoppel check failed: %v", estoppelErr)
		} else {
			degradedReason += fmt.Sprintf("; estoppel check failed: %v", estoppelErr)
		}
		estoppelResult = &EstoppelCheckResult{Skipped: true, SkipReason: fmt.Sprintf("estoppel check failed: %v", estoppelErr)}
		a.logger.Warn("estoppel check failed, proceeding without penalty", "error", estoppelErr)
	}

	// ---- Compute overall score ----
	eqScore := 0.0
	if equivalentsResult != nil && !equivalentsResult.Skipped {
		eqScore = equivalentsResult.Score
	}
	estPenalty := 0.0
	if estoppelResult != nil && estoppelResult.HasEstoppel {
		estPenalty = estoppelResult.PenaltyScore
	}

	overall := weightLiteral*literalResult.Score + weightEquivalents*eqScore - weightEstoppel*estPenalty
	overall = clamp01(overall)

	// Short-circuit override: literal >= 0.9 forces Critical.
	riskLevel := ClassifyRisk(overall)
	if shortCircuited && literalResult.Score >= literalShortCircuitThreshold {
		riskLevel = RiskCritical
		if overall < 0.85 {
			overall = math.Max(overall, 0.85)
		}
	}

	// ---- Per-claim match results ----
	matchedClaims := a.buildClaimMatches(elementMap, literalResult, equivalentsResult, estoppelResult)

	// ---- Confidence ----
	confidence := literalResult.Confidence
	if equivalentsResult != nil && !equivalentsResult.Skipped {
		confidence = (confidence + equivalentsResult.Confidence) / 2.0
	}

	// ---- Model versions ----
	modelVersions := map[string]string{
		"literal_model": a.model.ModelVersion(),
	}
	if !equivalentsResult.Skipped {
		modelVersions["equivalents_model"] = a.equivalents.ModelVersion()
	}

	elapsed := time.Since(start).Milliseconds()

	result := &AssessmentResult{
		RequestID:           req.RequestID,
		OverallRiskLevel:    riskLevel,
		OverallScore:        roundTo4(overall),
		LiteralAnalysis:     literalResult,
		EquivalentsAnalysis: equivalentsResult,
		EstoppelCheck:       estoppelResult,
		MatchedClaims:       matchedClaims,
		Confidence:          roundTo4(confidence),
		Degraded:            degraded,
		DegradedReason:      degradedReason,
		ProcessingTimeMs:    elapsed,
		ModelVersions:       modelVersions,
	}

	// Persist for later explanation.
	if a.explStore != nil && req.RequestID != "" {
		if storeErr := a.explStore.Store(ctx, result); storeErr != nil {
			a.logger.Warn("failed to store assessment result", "error", storeErr)
		}
	}

	// Record metrics.
	a.metrics.RecordRiskAssessment(ctx, riskLevel.String(), float64(elapsed))
	a.metrics.RecordInference(ctx, &common.InferenceMetricParams{
		ModelName:  "infringe-net",
		TaskType:   "assess",
		DurationMs: float64(elapsed),
		Success:    true,
		BatchSize:  1,
	})

	return result, nil
}

// -------------------------------------------------------------------------
// BatchAssess
// -------------------------------------------------------------------------

func (a *infringementAssessor) BatchAssess(ctx context.Context, reqs []*AssessmentRequest) ([]*AssessmentResult, error) {
	if len(reqs) == 0 {
		return []*AssessmentResult{}, nil
	}

	concurrency := a.defaultOpts.MaxConcurrency
	if concurrency <= 0 {
		concurrency = 8
	}

	results := make([]*AssessmentResult, len(reqs))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, req := range reqs {
		wg.Add(1)
		go func(idx int, r *AssessmentRequest) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := a.Assess(ctx, r)
			if err != nil {
				results[idx] = &AssessmentResult{
					RequestID: r.RequestID,
					Error:     err.Error(),
				}
			} else {
				results[idx] = res
			}
		}(i, req)
	}
	wg.Wait()

	return results, nil
}

// -------------------------------------------------------------------------
// AssessAgainstPortfolio
// -------------------------------------------------------------------------

func (a *infringementAssessor) AssessAgainstPortfolio(ctx context.Context, molecule *MoleculeInput, portfolioID string) (*PortfolioAssessmentResult, error) {
	if err := molecule.Validate(); err != nil {
		return nil, err
	}
	if portfolioID == "" {
		return nil, errors.NewInvalidInputError("portfolio_id is required")
	}
	if a.portfolio == nil {
		return nil, errors.NewInvalidInputError("portfolio loader not configured")
	}

	start := time.Now()

	claims, err := a.portfolio.LoadPortfolioClaims(ctx, portfolioID)
	if err != nil {
		return nil, fmt.Errorf("loading portfolio claims: %w", err)
	}

	if len(claims) == 0 {
		return &PortfolioAssessmentResult{
			Molecule:            molecule,
			PortfolioID:         portfolioID,
			TotalPatentsScanned: 0,
			RiskDistribution:    map[RiskLevel]int{RiskNone: 0, RiskLow: 0, RiskMedium: 0, RiskHigh: 0, RiskCritical: 0},
			TopRisks:            []*AssessmentResult{},
			ScanDurationMs:      time.Since(start).Milliseconds(),
		}, nil
	}

	// Group claims by patent.
	patentClaims := groupClaimsByPatent(claims)

	// Build batch requests.
	var reqs []*AssessmentRequest
	for patentID, pClaims := range patentClaims {
		reqs = append(reqs, &AssessmentRequest{
			RequestID: fmt.Sprintf("portfolio-%s-%s", portfolioID, patentID),
			Molecule:  molecule,
			Claims:    pClaims,
		})
	}

	results, err := a.BatchAssess(ctx, reqs)
	if err != nil {
		return nil, fmt.Errorf("batch assessment failed: %w", err)
	}

	// Aggregate.
	dist := map[RiskLevel]int{RiskNone: 0, RiskLow: 0, RiskMedium: 0, RiskHigh: 0, RiskCritical: 0}
	for _, r := range results {
		if r.Error == "" {
			dist[r.OverallRiskLevel]++
		}
	}

	// Sort by score descending for top risks.
	sort.Slice(results, func(i, j int) bool {
		return results[i].OverallScore > results[j].OverallScore
	})
	topN := 10
	if len(results) < topN {
		topN = len(results)
	}

	return &PortfolioAssessmentResult{
		Molecule:            molecule,
		PortfolioID:         portfolioID,
		TotalPatentsScanned: len(patentClaims),
		RiskDistribution:    dist,
		TopRisks:            results[:topN],
		ScanDurationMs:      time.Since(start).Milliseconds(),
	}, nil
}

// -------------------------------------------------------------------------
// ExplainAssessment
// -------------------------------------------------------------------------

func (a *infringementAssessor) ExplainAssessment(ctx context.Context, resultID string) (*AssessmentExplanation, error) {
	if resultID == "" {
		return nil, errors.NewInvalidInputError("result_id is required")
	}
	if a.explStore == nil {
		return nil, errors.NewInvalidInputError("explanation store not configured")
	}
	if a.explGenerator == nil {
		return nil, errors.NewInvalidInputError("explanation generator not configured")
	}

	result, err := a.explStore.Load(ctx, resultID)
	if err != nil {
		return nil, fmt.Errorf("loading assessment result: %w", err)
	}
	if result == nil {
		return nil, errors.NewNotFoundError(fmt.Sprintf("assessment result %s not found", resultID))
	}

	explanation, err := a.explGenerator.Generate(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("generating explanation: %w", err)
	}
	explanation.ResultID = resultID
	return explanation, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// ensureIndependentClaims checks whether all claims are dependent; if so,
// loads the corresponding independent claims via the mapper.
func (a *infringementAssessor) ensureIndependentClaims(ctx context.Context, claims []*ClaimInput) ([]*ClaimInput, error) {
	allDependent := true
	for _, c := range claims {
		if c.ClaimType == "independent" || c.ClaimType == "" {
			allDependent = false
			break
		}
	}
	if !allDependent {
		return claims, nil
	}
	// All claims are dependent — load their independent parents.
	independents, err := a.mapper.LoadIndependentClaims(ctx, claims)
	if err != nil {
		return nil, err
	}
	merged := make([]*ClaimInput, 0, len(claims)+len(independents))
	merged = append(merged, independents...)
	merged = append(merged, claims...)
	return merged, nil
}

// buildClaimMatches constructs per-claim match results from the sub-analyses.
func (a *infringementAssessor) buildClaimMatches(
	elementMap map[string][]string,
	literal *LiteralAnalysisResult,
	equivalents *EquivalentsAnalysisResult,
	estoppel *EstoppelCheckResult,
) []*ClaimMatchResult {
	var results []*ClaimMatchResult
	for claimID, elements := range elementMap {
		litScore := literal.Score
		eqScore := 0.0
		if equivalents != nil && !equivalents.Skipped {
			eqScore = equivalents.Score
		}
		estPenalty := 0.0
		if estoppel != nil && estoppel.HasEstoppel {
			estPenalty = estoppel.PenaltyScore
		}
		combined := clamp01(weightLiteral*litScore + weightEquivalents*eqScore -	weightEstoppel*estPenalty)

		matched := []string{}
		missed := []string{}
		if literal.ElementScores != nil {
			for _, elem := range elements {
				if s, ok := literal.ElementScores[elem]; ok && s >= 0.5 {
					matched = append(matched, elem)
				} else {
					missed = append(missed, elem)
				}
			}
		}

		results = append(results, &ClaimMatchResult{
			ClaimID:          claimID,
			LiteralScore:     roundTo4(litScore),
			EquivalentsScore: roundTo4(eqScore),
			EstoppelPenalty:  roundTo4(estPenalty),
			CombinedScore:    roundTo4(combined),
			RiskLevel:        ClassifyRisk(combined),
			MatchedElements:  matched,
			MissedElements:   missed,
		})
	}

	// Sort by combined score descending.
	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})
	return results
}

// flattenElements merges all element lists from the claim-element map.
func flattenElements(m map[string][]string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, elems := range m {
		for _, e := range elems {
			if _, ok := seen[e]; !ok {
				seen[e] = struct{}{}
				out = append(out, e)
			}
		}
	}
	return out
}

// groupClaimsByPatent groups claims by their PatentID.
func groupClaimsByPatent(claims []*ClaimInput) map[string][]*ClaimInput {
	m := make(map[string][]*ClaimInput)
	for _, c := range claims {
		pid := c.PatentID
		if pid == "" {
			pid = "_unknown_"
		}
		m[pid] = append(m[pid], c)
	}
	return m
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func roundTo4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

//Personal.AI order the ending

