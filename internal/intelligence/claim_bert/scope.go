package claim_bert

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ============================================================================
// Enumerations
// ============================================================================

// ScopeBreadthLevel classifies the breadth of a claim's protection scope.
type ScopeBreadthLevel string

const (
	// ScopeBroad indicates a broad protection scope (score >= 0.75).
	ScopeBroad ScopeBreadthLevel = "BROAD"
	// ScopeModerate indicates a moderate protection scope (score >= 0.50).
	ScopeModerate ScopeBreadthLevel = "MODERATE"
	// ScopeNarrow indicates a narrow protection scope (score >= 0.25).
	ScopeNarrow ScopeBreadthLevel = "NARROW"
	// ScopeVeryNarrow indicates a very narrow protection scope (score < 0.25).
	ScopeVeryNarrow ScopeBreadthLevel = "VERY_NARROW"
)

// ClassifyBreadth maps a numeric breadth score to a ScopeBreadthLevel.
func ClassifyBreadth(score float64) ScopeBreadthLevel {
	switch {
	case score >= 0.75:
		return ScopeBroad
	case score >= 0.50:
		return ScopeModerate
	case score >= 0.25:
		return ScopeNarrow
	default:
		return ScopeVeryNarrow
	}
}

// ScopeRelationship describes the containment relationship between two claims.
type ScopeRelationship string

const (
	RelAContainsB ScopeRelationship = "A_CONTAINS_B"
	RelBContainsA ScopeRelationship = "B_CONTAINS_A"
	RelOverlapping ScopeRelationship = "OVERLAPPING"
	RelDisjoint    ScopeRelationship = "DISJOINT"
	RelEquivalent  ScopeRelationship = "EQUIVALENT"
)

// GapSeverity classifies the severity of a scope gap.
type GapSeverity string

const (
	GapCritical GapSeverity = "CRITICAL"
	GapMajor    GapSeverity = "MAJOR"
	GapMinor    GapSeverity = "MINOR"
)

// ============================================================================
// Core data structures
// ============================================================================

// ScopeAnalysis is the result of analyzing a single claim's protection scope.
type ScopeAnalysis struct {
	ClaimNumber              int               `json:"claim_number"`
	BreadthScore             float64           `json:"breadth_score"`
	BreadthLevel             ScopeBreadthLevel `json:"breadth_level"`
	TransitionalPhraseImpact string            `json:"transitional_phrase_impact"`
	FeatureCount             int               `json:"feature_count"`
	MarkushExpansion         int               `json:"markush_expansion"`
	NumericalRangeWidth      float64           `json:"numerical_range_width"`
	KeyLimitations           []string          `json:"key_limitations"`
	BroadeningOpportunities  []string          `json:"broadening_opportunities"`
	NarrowingRisks           []string          `json:"narrowing_risks"`
}

// ScopeComparison is the result of comparing two claims' scopes.
type ScopeComparison struct {
	ClaimA         int                `json:"claim_a"`
	ClaimB         int                `json:"claim_b"`
	Relationship   ScopeRelationship  `json:"relationship"`
	OverlapScore   float64            `json:"overlap_score"`
	SharedFeatures []*TechnicalFeature `json:"shared_features"`
	UniqueToA      []*TechnicalFeature `json:"unique_to_a"`
	UniqueToB      []*TechnicalFeature `json:"unique_to_b"`
	Analysis       string             `json:"analysis"`
}

// ClaimSetScopeAnalysis is the result of analyzing an entire claim set.
type ClaimSetScopeAnalysis struct {
	PatentID           string                   `json:"patent_id"`
	TotalClaims        int                      `json:"total_claims"`
	IndependentCount   int                      `json:"independent_count"`
	DependentCount     int                      `json:"dependent_count"`
	OverallCoverage    float64                  `json:"overall_coverage"`
	ClaimAnalyses      []*ScopeAnalysis         `json:"claim_analyses"`
	WidestClaim        *ScopeAnalysis           `json:"widest_claim"`
	NarrowestClaim     *ScopeAnalysis           `json:"narrowest_claim"`
	Gaps               []*ScopeGap              `json:"gaps"`
	Visualization      *ScopeVisualizationData  `json:"visualization"`
	CategoryCoverage   map[string]int           `json:"category_coverage"`
}

// ScopeGap represents a gap in the protection scope of a claim set.
type ScopeGap struct {
	Description    string      `json:"description"`
	AffectedClaims []int       `json:"affected_claims"`
	Severity       GapSeverity `json:"severity"`
	Recommendation string      `json:"recommendation"`
}

// ScopeVisualizationData provides data for rendering a claim scope graph.
type ScopeVisualizationData struct {
	Nodes       []ScopeNode     `json:"nodes"`
	Edges       []ScopeEdge     `json:"edges"`
	Layers      [][]int         `json:"layers"`
	HeatmapData [][]float64     `json:"heatmap_data"`
}

// ScopeNode represents a single claim in the visualization graph.
type ScopeNode struct {
	ClaimNumber  int               `json:"claim_number"`
	Label        string            `json:"label"`
	ClaimType    ClaimType         `json:"claim_type"`
	BreadthScore float64           `json:"breadth_score"`
	BreadthLevel ScopeBreadthLevel `json:"breadth_level"`
	Category     string            `json:"category"`
	Size         float64           `json:"size"`
}

// ScopeEdge represents a relationship between two claims.
type ScopeEdge struct {
	Source       int               `json:"source"`
	Target       int               `json:"target"`
	EdgeType     ScopeEdgeType     `json:"edge_type"`
	Weight       float64           `json:"weight"`
	Relationship ScopeRelationship `json:"relationship,omitempty"`
}

// ScopeEdgeType classifies the type of edge in the scope graph.
type ScopeEdgeType string

const (
	EdgeDependency  ScopeEdgeType = "DEPENDENCY"
	EdgeContainment ScopeEdgeType = "CONTAINMENT"
	EdgeOverlap     ScopeEdgeType = "OVERLAP"
)

// ============================================================================
// ScopeAnalyzer interface
// ============================================================================

// ScopeAnalyzer provides deep analysis of patent claim protection scopes.
// It is the core strategic analysis capability for patent portfolios.
type ScopeAnalyzer interface {
	// AnalyzeScope performs scope analysis on a single parsed claim.
	AnalyzeScope(ctx context.Context, claim *ParsedClaim) (*ScopeAnalysis, error)

	// CompareScopes compares the protection scopes of two claims.
	CompareScopes(ctx context.Context, claimA, claimB *ParsedClaim) (*ScopeComparison, error)

	// AnalyzeClaimSetScope performs holistic scope analysis on an entire claim set.
	AnalyzeClaimSetScope(ctx context.Context, claimSet *ParsedClaimSet) (*ClaimSetScopeAnalysis, error)

	// ComputeScopeBreadth computes a normalized breadth score in [0, 1].
	ComputeScopeBreadth(ctx context.Context, claim *ParsedClaim) (float64, error)

	// IdentifyScopeGaps finds gaps in the protection coverage of a claim set.
	IdentifyScopeGaps(ctx context.Context, claimSet *ParsedClaimSet) ([]*ScopeGap, error)

	// GenerateScopeVisualization produces data for rendering a scope graph.
	GenerateScopeVisualization(ctx context.Context, claimSet *ParsedClaimSet) (*ScopeVisualizationData, error)
}

// ============================================================================
// Transitional phrase adjustment constants (based on MPEP 2111.03)
// ============================================================================

var transitionalPhraseAdjustments = map[TransitionalPhraseType]float64{
	PhraseComprising:              +0.10,
	PhraseConsistingEssentiallyOf: +0.00,
	PhraseConsistingOf:            -0.15,
}

var transitionalPhraseImpactDescriptions = map[TransitionalPhraseType]string{
	PhraseComprising: "Open-ended transitional phrase 'comprising' allows the claim to cover " +
		"embodiments with additional, unrecited elements. This is the broadest standard " +
		"transitional phrase under MPEP 2111.03, maximizing protection scope.",
	PhraseConsistingEssentiallyOf: "Semi-open transitional phrase 'consisting essentially of' limits " +
		"the scope to the recited elements and those that do not materially affect the basic " +
		"and novel characteristics. Moderate scope — narrower than 'comprising' but broader " +
		"than 'consisting of'.",
	PhraseConsistingOf: "Closed transitional phrase 'consisting of' restricts the claim to " +
		"only the recited elements, excluding any additional elements. This is the narrowest " +
		"standard transitional phrase, significantly limiting protection scope.",
}

// ============================================================================
// Similarity threshold constants
// ============================================================================

const (
	featureMatchThreshold     = 0.80 // cosine similarity threshold for feature matching
	overlapEdgeThreshold      = 0.30 // minimum overlap score to draw an overlap edge
	equivalenceThreshold      = 0.95 // overlap score above which claims are considered equivalent
	containmentThreshold      = 0.90 // fraction of features matched to consider containment
)

// ============================================================================
// scopeAnalyzerImpl
// ============================================================================

// scopeAnalyzerImpl is the production implementation of ScopeAnalyzer.
type scopeAnalyzerImpl struct {
	embedder ClaimEmbedder
	logger   common.Logger
	metrics  common.IntelligenceMetrics
	mu       sync.RWMutex
}

// ClaimEmbedder abstracts the embedding capability needed by scope analysis.
// It is typically backed by the ClaimBERT model.
type ClaimEmbedder interface {
	// EmbedFeature returns the embedding vector for a technical feature.
	EmbedFeature(ctx context.Context, feature *TechnicalFeature) ([]float32, error)
	// EmbedClaim returns the embedding vector for an entire claim.
	EmbedClaim(ctx context.Context, claim *ParsedClaim) ([]float32, error)
}

// NewScopeAnalyzer creates a new ScopeAnalyzer.
func NewScopeAnalyzer(
	embedder ClaimEmbedder,
	logger common.Logger,
	metrics common.IntelligenceMetrics,
) (ScopeAnalyzer, error) {
	if embedder == nil {
		return nil, errors.NewInvalidInputError("embedder is required for scope analysis")
	}
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	if metrics == nil {
		metrics = common.NewNoopIntelligenceMetrics()
	}
	return &scopeAnalyzerImpl{
		embedder: embedder,
		logger:   logger,
		metrics:  metrics,
	}, nil
}

// ---------------------------------------------------------------------------
// AnalyzeScope
// ---------------------------------------------------------------------------

func (s *scopeAnalyzerImpl) AnalyzeScope(ctx context.Context, claim *ParsedClaim) (*ScopeAnalysis, error) {
	if claim == nil {
		return nil, errors.NewInvalidInputError("claim must not be nil")
	}

	breadth, err := s.ComputeScopeBreadth(ctx, claim)
	if err != nil {
		return nil, fmt.Errorf("computing breadth: %w", err)
	}

	featureCount := len(claim.Features)
	markushExpansion := computeMarkushExpansion(claim)
	numericalWidth := computeNumericalRangeWidth(claim)
	phraseImpact := transitionalPhraseImpactDescriptions[TransitionalPhraseType(claim.TransitionalPhrase)]
	if phraseImpact == "" {
		phraseImpact = "Unknown transitional phrase; scope impact cannot be determined precisely."
	}

	limitations := extractKeyLimitations(claim)
	opportunities := suggestBroadeningOpportunities(claim, breadth)
	risks := identifyNarrowingRisks(claim, breadth)

	return &ScopeAnalysis{
		ClaimNumber:              claim.ClaimNumber,
		BreadthScore:             breadth,
		BreadthLevel:             ClassifyBreadth(breadth),
		TransitionalPhraseImpact: phraseImpact,
		FeatureCount:             featureCount,
		MarkushExpansion:         markushExpansion,
		NumericalRangeWidth:      numericalWidth,
		KeyLimitations:           limitations,
		BroadeningOpportunities:  opportunities,
		NarrowingRisks:           risks,
	}, nil
}

// ---------------------------------------------------------------------------
// ComputeScopeBreadth
// ---------------------------------------------------------------------------

func (s *scopeAnalyzerImpl) ComputeScopeBreadth(ctx context.Context, claim *ParsedClaim) (float64, error) {
	if claim == nil {
		return 0, errors.NewInvalidInputError("claim must not be nil")
	}

	// 1. Base score from ClaimBERT scope analysis task head.
	//    If the model has not produced a ScopeScore, we start from a neutral 0.50.
	base := claim.ScopeScore
	if base <= 0 {
		base = 0.50
	}

	// 2. Transitional phrase adjustment.
	phraseAdj, ok := transitionalPhraseAdjustments[TransitionalPhraseType(claim.TransitionalPhrase)]
	if !ok {
		phraseAdj = 0.0
	}

	// 3. Feature count adjustment.
	featureAdj := 0.0
	featureCount := len(claim.Features)
	switch {
	case featureCount <= 3:
		featureAdj = +0.05
	case featureCount >= 8:
		featureAdj = -0.10
	default:
		// Linear interpolation between 3 and 8 features: +0.05 → -0.10
		// At 3 features: +0.05, at 8 features: -0.10
		t := float64(featureCount-3) / float64(8-3) // 0..1
		featureAdj = 0.05 - t*0.15
	}

	// 4. Markush structure adjustment.
	markushAdj := 0.0
	for _, mg := range claim.MarkushGroups {
		if mg == nil {
			continue
		}
		memberCount := len(mg.Members)
		if memberCount >= 5 {
			markushAdj += 0.08
			if mg.IsOpenEnded {
				markushAdj += 0.05
			}
			break // Apply adjustment once for the most significant Markush group
		}
	}

	// 5. Numerical range adjustment: normalized width weighted +0.00 ~ +0.05.
	numericalAdj := 0.0
	normWidth := computeNumericalRangeWidth(claim)
	if normWidth > 0 {
		numericalAdj = normWidth * 0.05 // scale [0,1] → [0, 0.05]
	}

	// 6. Combine and clamp.
	score := base + phraseAdj + featureAdj + markushAdj + numericalAdj
	score = clamp01(score)

	return score, nil
}

// ---------------------------------------------------------------------------
// CompareScopes
// ---------------------------------------------------------------------------

func (s *scopeAnalyzerImpl) CompareScopes(ctx context.Context, claimA, claimB *ParsedClaim) (*ScopeComparison, error) {
	if claimA == nil || claimB == nil {
		return nil, errors.NewInvalidInputError("both claims must be non-nil")
	}

	fa := claimA.Features
	fb := claimB.Features

	if len(fa) == 0 && len(fb) == 0 {
		return &ScopeComparison{
			ClaimA:       claimA.ClaimNumber,
			ClaimB:       claimB.ClaimNumber,
			Relationship: RelEquivalent,
			OverlapScore: 1.0,
			Analysis:     "Both claims have no technical features; considered equivalent by default.",
		}, nil
	}

	// Step 1: Build semantic similarity matrix M[i][j] between features of A and B.
	simMatrix, err := s.buildSimilarityMatrix(ctx, fa, fb)
	if err != nil {
		return nil, fmt.Errorf("building similarity matrix: %w", err)
	}

	// Step 2: Greedy bipartite matching to find optimal feature alignment.
	matchesAtoB, matchesBtoA := greedyBipartiteMatch(simMatrix, featureMatchThreshold)

	// Step 3: Classify shared, unique-to-A, unique-to-B.
	var shared []*TechnicalFeature
	var uniqueA []*TechnicalFeature
	var uniqueB []*TechnicalFeature

	matchedBIndices := make(map[int]bool)
	for i, jIdx := range matchesAtoB {
		if jIdx >= 0 {
			shared = append(shared, fa[i])
			matchedBIndices[jIdx] = true
		} else {
			uniqueA = append(uniqueA, fa[i])
		}
	}
	for j := range fb {
		if !matchedBIndices[j] {
			uniqueB = append(uniqueB, fb[j])
		}
	}

	// Step 4: Determine relationship.
	sharedCount := len(shared)
	maxFeatures := max(len(fa), len(fb))
	overlapScore := 0.0
	if maxFeatures > 0 {
		overlapScore = float64(sharedCount) / float64(maxFeatures)
	}

	relationship := determineRelationship(fa, fb, matchesAtoB, matchesBtoA, overlapScore)

	// Step 5: Generate analysis text.
	analysis := generateComparisonAnalysis(claimA, claimB, relationship, overlapScore, sharedCount, len(uniqueA), len(uniqueB))

	return &ScopeComparison{
		ClaimA:         claimA.ClaimNumber,
		ClaimB:         claimB.ClaimNumber,
		Relationship:   relationship,
		OverlapScore:   overlapScore,
		SharedFeatures: shared,
		UniqueToA:      uniqueA,
		UniqueToB:      uniqueB,
		Analysis:       analysis,
	}, nil
}

// ---------------------------------------------------------------------------
// AnalyzeClaimSetScope
// ---------------------------------------------------------------------------

func (s *scopeAnalyzerImpl) AnalyzeClaimSetScope(ctx context.Context, claimSet *ParsedClaimSet) (*ClaimSetScopeAnalysis, error) {
	if claimSet == nil || len(claimSet.Claims) == 0 {
		return nil, errors.NewInvalidInputError("claim set must contain at least one claim")
	}

	// 1. Analyze each claim individually.
	analyses := make([]*ScopeAnalysis, 0, len(claimSet.Claims))
	var widest, narrowest *ScopeAnalysis
	independentCount := 0
	dependentCount := 0
	categoryCoverage := make(map[string]int)
	independentBreadthSum := 0.0
	independentBreadthCount := 0

	for _, claim := range claimSet.Claims {
		sa, err := s.AnalyzeScope(ctx, claim)
		if err != nil {
			s.logger.Warn("scope analysis failed for claim", "claim_number", claim.ClaimNumber, "error", err)
			continue
		}
		analyses = append(analyses, sa)

		if claim.ClaimType == ClaimIndependent {
			independentCount++
			independentBreadthSum += sa.BreadthScore
			independentBreadthCount++
		} else {
			dependentCount++
		}

		cat := claim.Category
		if cat == "" {
			cat = "unclassified"
		}
		categoryCoverage[cat]++

		if widest == nil || sa.BreadthScore > widest.BreadthScore {
			widest = sa
		}
		if narrowest == nil || sa.BreadthScore < narrowest.BreadthScore {
			narrowest = sa
		}
	}

	// 2. Overall coverage = weighted average of independent claim breadth scores.
	overallCoverage := 0.0
	if independentBreadthCount > 0 {
		overallCoverage = independentBreadthSum / float64(independentBreadthCount)
	}

	// 3. Identify gaps.
	gaps, err := s.IdentifyScopeGaps(ctx, claimSet)
	if err != nil {
		s.logger.Warn("gap identification failed", "error", err)
		gaps = []*ScopeGap{}
	}

	// 4. Generate visualization.
	viz, err := s.GenerateScopeVisualization(ctx, claimSet)
	if err != nil {
		s.logger.Warn("visualization generation failed", "error", err)
		viz = &ScopeVisualizationData{}
	}

	return &ClaimSetScopeAnalysis{
		PatentID:         claimSet.PatentID,
		TotalClaims:      len(claimSet.Claims),
		IndependentCount: independentCount,
		DependentCount:   dependentCount,
		OverallCoverage:  overallCoverage,
		ClaimAnalyses:    analyses,
		WidestClaim:      widest,
		NarrowestClaim:   narrowest,
		Gaps:             gaps,
		Visualization:    viz,
		CategoryCoverage: categoryCoverage,
	}, nil
}

// ---------------------------------------------------------------------------
// IdentifyScopeGaps
// ---------------------------------------------------------------------------

func (s *scopeAnalyzerImpl) IdentifyScopeGaps(ctx context.Context, claimSet *ParsedClaimSet) ([]*ScopeGap, error) {
	if claimSet == nil || len(claimSet.Claims) == 0 {
		return nil, errors.NewInvalidInputError("claim set must contain at least one claim")
	}

	var gaps []*ScopeGap

	// --- Gap type 1: Missing claim categories ---
	// A well-drafted patent typically covers product, method, composition, and use.
	expectedCategories := map[string]bool{
		"product":     false,
		"method":      false,
		"composition": false,
		"use":         false,
	}
	independentClaimNumbers := make([]int, 0)
	for _, claim := range claimSet.Claims {
		cat := strings.ToLower(claim.Category)
		if _, ok := expectedCategories[cat]; ok {
			expectedCategories[cat] = true
		}
		if claim.ClaimType == ClaimIndependent {
			independentClaimNumbers = append(independentClaimNumbers, claim.ClaimNumber)
		}
	}

	for cat, covered := range expectedCategories {
		if !covered {
			severity := GapMajor
			recommendation := fmt.Sprintf("Consider adding an independent %s claim to strengthen protection.", cat)
			if cat == "method" || cat == "product" {
				severity = GapCritical
				recommendation = fmt.Sprintf("CRITICAL: No independent %s claim found. "+
					"This leaves a significant gap in protection. A competitor could potentially "+
					"practice the invention through a %s approach without infringement. "+
					"Strongly recommend adding an independent %s claim.", cat, cat, cat)
			}
			gaps = append(gaps, &ScopeGap{
				Description:    fmt.Sprintf("No independent %s claim found in the claim set.", cat),
				AffectedClaims: independentClaimNumbers,
				Severity:       severity,
				Recommendation: recommendation,
			})
		}
	}

	// --- Gap type 2: Broken dependency chains ---
	// Check that each independent claim has at least one dependent claim.
	dependentMap := buildDependencyMap(claimSet)
	for _, claim := range claimSet.Claims {
		if claim.ClaimType != ClaimIndependent {
			continue
		}
		deps := dependentMap[claim.ClaimNumber]
		if len(deps) == 0 {
			gaps = append(gaps, &ScopeGap{
				Description: fmt.Sprintf("Independent claim %d has no dependent claims. "+
					"Dependent claims provide fallback positions during prosecution.", claim.ClaimNumber),
				AffectedClaims: []int{claim.ClaimNumber},
				Severity:       GapMajor,
				Recommendation: fmt.Sprintf("Add dependent claims narrowing independent claim %d "+
					"to provide fallback positions. Consider adding at least 2-3 dependent claims "+
					"covering key embodiments and preferred ranges.", claim.ClaimNumber),
			})
		}
	}

	// --- Gap type 3: Shallow dependency chains ---
	// If a dependent claim depends on another dependent (chain depth >= 2),
	// check that intermediate links exist.
	for _, claim := range claimSet.Claims {
		if claim.ClaimType != ClaimDependent {
			continue
		}
		for _, depOn := range claim.DependsOn {
			parent := findClaimByNumber(claimSet, depOn)
			if parent == nil {
				gaps = append(gaps, &ScopeGap{
					Description: fmt.Sprintf("Claim %d depends on claim %d, which does not exist "+
						"in the claim set. This is a broken dependency reference.", claim.ClaimNumber, depOn),
					AffectedClaims: []int{claim.ClaimNumber, depOn},
					Severity:       GapCritical,
					Recommendation: fmt.Sprintf("Fix the dependency reference for claim %d. "+
						"Either add the missing claim %d or update the dependency to reference "+
						"an existing claim.", claim.ClaimNumber, depOn),
				})
			}
		}
	}

	// --- Gap type 4: Markush group coverage gaps ---
	// If a claim has a Markush group with very few members, it may be under-inclusive.
	for _, claim := range claimSet.Claims {
		for _, mg := range claim.MarkushGroups {
			if mg == nil {
				continue
			}
			memberCount := len(mg.Members)
			if memberCount > 0 && memberCount < 3 {
				gaps = append(gaps, &ScopeGap{
					Description: fmt.Sprintf("Claim %d contains a Markush group with only %d member(s). "+
						"This may leave out important alternatives.", claim.ClaimNumber, memberCount),
					AffectedClaims: []int{claim.ClaimNumber},
					Severity:       GapMinor,
					Recommendation: fmt.Sprintf("Consider expanding the Markush group in claim %d "+
						"to include additional structurally related alternatives. Review the chemical "+
						"space around the existing members for commonly used substituents.", claim.ClaimNumber),
				})
			}
		}
	}

	// --- Gap type 5: No independent claim with broad scope ---
	// If all independent claims are narrow, the overall protection is weak.
	allNarrow := true
	for _, claim := range claimSet.Claims {
		if claim.ClaimType != ClaimIndependent {
			continue
		}
		breadth, err := s.ComputeScopeBreadth(ctx, claim)
		if err != nil {
			continue
		}
		if breadth >= 0.50 {
			allNarrow = false
			break
		}
	}
	if allNarrow && len(independentClaimNumbers) > 0 {
		gaps = append(gaps, &ScopeGap{
			Description: "All independent claims have narrow scope (BreadthScore < 0.50). " +
				"The patent may be easy to design around.",
			AffectedClaims: independentClaimNumbers,
			Severity:       GapCritical,
			Recommendation: "Consider broadening at least one independent claim by removing " +
				"non-essential limitations, using open-ended transitional phrases ('comprising'), " +
				"or generalizing specific structural features to functional language.",
		})
	}

	// Sort gaps by severity: Critical > Major > Minor.
	sort.SliceStable(gaps, func(i, j int) bool {
		return gapSeverityRank(gaps[i].Severity) > gapSeverityRank(gaps[j].Severity)
	})

	return gaps, nil
}

// ---------------------------------------------------------------------------
// GenerateScopeVisualization
// ---------------------------------------------------------------------------

func (s *scopeAnalyzerImpl) GenerateScopeVisualization(ctx context.Context, claimSet *ParsedClaimSet) (*ScopeVisualizationData, error) {
	if claimSet == nil || len(claimSet.Claims) == 0 {
		return nil, errors.NewInvalidInputError("claim set must contain at least one claim")
	}

	n := len(claimSet.Claims)

	// --- 1. Build nodes ---
	nodes := make([]ScopeNode, 0, n)
	breadthScores := make(map[int]float64) // claimNumber -> breadthScore
	for _, claim := range claimSet.Claims {
		breadth, err := s.ComputeScopeBreadth(ctx, claim)
		if err != nil {
			breadth = 0.50 // fallback
		}
		breadthScores[claim.ClaimNumber] = breadth

		cat := claim.Category
		if cat == "" {
			cat = "unclassified"
		}

		nodes = append(nodes, ScopeNode{
			ClaimNumber:  claim.ClaimNumber,
			Label:        fmt.Sprintf("Claim %d", claim.ClaimNumber),
			ClaimType:    claim.ClaimType,
			BreadthScore: breadth,
			BreadthLevel: ClassifyBreadth(breadth),
			Category:     cat,
			Size:         20.0 + breadth*80.0, // size range [20, 100]
		})
	}

	// --- 2. Build edges ---
	edges := make([]ScopeEdge, 0)

	// 2a. Dependency edges (from dependent to its parent).
	for _, claim := range claimSet.Claims {
		for _, depOn := range claim.DependsOn {
			edges = append(edges, ScopeEdge{
				Source:   claim.ClaimNumber,
				Target:   depOn,
				EdgeType: EdgeDependency,
				Weight:   1.0,
			})
		}
	}

	// 2b. Containment and overlap edges via pairwise comparison.
	// We compute a full N×N heatmap at the same time.
	heatmap := make([][]float64, n)
	for i := range heatmap {
		heatmap[i] = make([]float64, n)
		heatmap[i][i] = 1.0 // self-similarity
	}

	claimIndex := make(map[int]int) // claimNumber -> index in claimSet.Claims
	for i, c := range claimSet.Claims {
		claimIndex[c.ClaimNumber] = i
	}

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			cA := claimSet.Claims[i]
			cB := claimSet.Claims[j]

			comp, err := s.CompareScopes(ctx, cA, cB)
			if err != nil {
				s.logger.Warn("pairwise comparison failed",
					"claim_a", cA.ClaimNumber, "claim_b", cB.ClaimNumber, "error", err)
				continue
			}

			heatmap[i][j] = comp.OverlapScore
			heatmap[j][i] = comp.OverlapScore // symmetric

			// Add containment or overlap edges if significant.
			switch comp.Relationship {
			case RelAContainsB, RelBContainsA:
				edges = append(edges, ScopeEdge{
					Source:       cA.ClaimNumber,
					Target:       cB.ClaimNumber,
					EdgeType:     EdgeContainment,
					Weight:       comp.OverlapScore,
					Relationship: comp.Relationship,
				})
			case RelOverlapping:
				if comp.OverlapScore >= overlapEdgeThreshold {
					edges = append(edges, ScopeEdge{
						Source:       cA.ClaimNumber,
						Target:       cB.ClaimNumber,
						EdgeType:     EdgeOverlap,
						Weight:       comp.OverlapScore,
						Relationship: comp.Relationship,
					})
				}
			case RelEquivalent:
				edges = append(edges, ScopeEdge{
					Source:       cA.ClaimNumber,
					Target:       cB.ClaimNumber,
					EdgeType:     EdgeContainment,
					Weight:       1.0,
					Relationship: RelEquivalent,
				})
			}
		}
	}

	// --- 3. Layer layout ---
	layers := buildLayerLayout(claimSet)

	return &ScopeVisualizationData{
		Nodes:       nodes,
		Edges:       edges,
		Layers:      layers,
		HeatmapData: heatmap,
	}, nil
}

// ============================================================================
// Internal helper functions
// ============================================================================

// buildSimilarityMatrix computes the cosine similarity between all pairs of
// features from two claims. Returns M[i][j] where i indexes fa and j indexes fb.
func (s *scopeAnalyzerImpl) buildSimilarityMatrix(ctx context.Context, fa, fb []*TechnicalFeature) ([][]float64, error) {
	m := len(fa)
	n := len(fb)
	matrix := make([][]float64, m)

	// Collect embeddings for all features.
	embA := make([][]float32, m)
	embB := make([][]float32, n)

	var mu sync.Mutex
	var firstErr error

	var wg sync.WaitGroup
	for i, f := range fa {
		wg.Add(1)
		go func(idx int, feat *TechnicalFeature) {
			defer wg.Done()
			emb, err := s.resolveFeatureEmbedding(ctx, feat)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			embA[idx] = emb
			mu.Unlock()
		}(i, f)
	}
	for j, f := range fb {
		wg.Add(1)
		go func(idx int, feat *TechnicalFeature) {
			defer wg.Done()
			emb, err := s.resolveFeatureEmbedding(ctx, feat)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			embB[idx] = emb
			mu.Unlock()
		}(j, f)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// Compute pairwise cosine similarities.
	for i := 0; i < m; i++ {
		matrix[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if embA[i] == nil || embB[j] == nil {
				matrix[i][j] = 0
				continue
			}
			matrix[i][j] = cosineSimilarity(embA[i], embB[j])
		}
	}

	return matrix, nil
}

// resolveFeatureEmbedding returns the embedding for a feature, using the
// pre-computed embedding if available, otherwise calling the embedder.
func (s *scopeAnalyzerImpl) resolveFeatureEmbedding(ctx context.Context, feat *TechnicalFeature) ([]float32, error) {
	if feat == nil {
		return nil, errors.NewInvalidInputError("feature must not be nil")
	}
	if len(feat.Embedding) > 0 {
		return feat.Embedding, nil
	}
	return s.embedder.EmbedFeature(ctx, feat)
}

// greedyBipartiteMatch performs greedy matching on a similarity matrix.
// Returns matchesAtoB[i] = index in B matched to A[i] (-1 if unmatched),
// and matchesBtoA[j] = index in A matched to B[j] (-1 if unmatched).
func greedyBipartiteMatch(simMatrix [][]float64, threshold float64) ([]int, []int) {
	m := len(simMatrix)
	if m == 0 {
		return nil, nil
	}
	n := len(simMatrix[0])

	matchesAtoB := make([]int, m)
	matchesBtoA := make([]int, n)
	for i := range matchesAtoB {
		matchesAtoB[i] = -1
	}
	for j := range matchesBtoA {
		matchesBtoA[j] = -1
	}

	// Collect all (i, j, sim) pairs above threshold, sort descending by sim.
	type pair struct {
		i, j int
		sim  float64
	}
	var pairs []pair
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if simMatrix[i][j] >= threshold {
				pairs = append(pairs, pair{i, j, simMatrix[i][j]})
			}
		}
	}
	sort.Slice(pairs, func(a, b int) bool {
		return pairs[a].sim > pairs[b].sim
	})

	usedA := make(map[int]bool)
	usedB := make(map[int]bool)
	for _, p := range pairs {
		if usedA[p.i] || usedB[p.j] {
			continue
		}
		matchesAtoB[p.i] = p.j
		matchesBtoA[p.j] = p.i
		usedA[p.i] = true
		usedB[p.j] = true
	}

	return matchesAtoB, matchesBtoA
}

// determineRelationship classifies the scope relationship between two claims
// based on their feature matching results.
func determineRelationship(fa, fb []*TechnicalFeature, matchesAtoB, matchesBtoA []int, overlapScore float64) ScopeRelationship {
	if len(fa) == 0 && len(fb) == 0 {
		return RelEquivalent
	}

	// Count matched features on each side.
	matchedA := 0
	for _, j := range matchesAtoB {
		if j >= 0 {
			matchedA++
		}
	}
	matchedB := 0
	for _, i := range matchesBtoA {
		if i >= 0 {
			matchedB++
		}
	}

	fracA := 0.0
	if len(fa) > 0 {
		fracA = float64(matchedA) / float64(len(fa))
	}
	fracB := 0.0
	if len(fb) > 0 {
		fracB = float64(matchedB) / float64(len(fb))
	}

	// Equivalence: both sides fully matched.
	if overlapScore >= equivalenceThreshold || (fracA >= containmentThreshold && fracB >= containmentThreshold) {
		return RelEquivalent
	}

	// A contains B: all of A's features are in B, but B has extras.
	// In patent law, if A has fewer features and all are matched in B,
	// then A's scope is broader (A contains B).
	if fracA >= containmentThreshold && fracB < containmentThreshold {
		return RelAContainsB
	}

	// B contains A: all of B's features are in A, but A has extras.
	if fracB >= containmentThreshold && fracA < containmentThreshold {
		return RelBContainsA
	}

	// Disjoint: no meaningful overlap.
	if matchedA == 0 && matchedB == 0 {
		return RelDisjoint
	}

	return RelOverlapping
}

// generateComparisonAnalysis produces a human-readable analysis of the comparison.
func generateComparisonAnalysis(
	claimA, claimB *ParsedClaim,
	rel ScopeRelationship,
	overlapScore float64,
	sharedCount, uniqueACount, uniqueBCount int,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Comparison of Claim %d and Claim %d: ", claimA.ClaimNumber, claimB.ClaimNumber))

	switch rel {
	case RelEquivalent:
		sb.WriteString("The two claims are substantially equivalent in scope. ")
		sb.WriteString("All technical features in one claim have corresponding features in the other. ")
		sb.WriteString("This may indicate redundancy — consider whether both claims are necessary, ")
		sb.WriteString("or whether one could be modified to cover a different embodiment.")

	case RelAContainsB:
		sb.WriteString(fmt.Sprintf("Claim %d has a broader scope that encompasses Claim %d. ", claimA.ClaimNumber, claimB.ClaimNumber))
		sb.WriteString(fmt.Sprintf("Claim %d adds %d additional limitation(s) that narrow its scope. ", claimB.ClaimNumber, uniqueBCount))
		sb.WriteString("This is a typical independent-dependent relationship pattern.")

	case RelBContainsA:
		sb.WriteString(fmt.Sprintf("Claim %d has a broader scope that encompasses Claim %d. ", claimB.ClaimNumber, claimA.ClaimNumber))
		sb.WriteString(fmt.Sprintf("Claim %d adds %d additional limitation(s) that narrow its scope. ", claimA.ClaimNumber, uniqueACount))

	case RelOverlapping:
		sb.WriteString(fmt.Sprintf("The claims partially overlap with an overlap score of %.2f. ", overlapScore))
		sb.WriteString(fmt.Sprintf("They share %d feature(s), while Claim %d has %d unique feature(s) and Claim %d has %d unique feature(s). ",
			sharedCount, claimA.ClaimNumber, uniqueACount, claimB.ClaimNumber, uniqueBCount))
		sb.WriteString("The overlapping but distinct scopes provide complementary protection.")

	case RelDisjoint:
		sb.WriteString("The claims cover entirely different technical subject matter with no meaningful overlap. ")
		sb.WriteString("This provides breadth of coverage across different aspects of the invention.")
	}

	return sb.String()
}

// computeMarkushExpansion calculates the total combinatorial expansion of
// all Markush groups in a claim.
func computeMarkushExpansion(claim *ParsedClaim) int {
	if claim == nil || len(claim.MarkushGroups) == 0 {
		return 0
	}
	total := 1
	hasMarkush := false
	for _, mg := range claim.MarkushGroups {
		if mg == nil {
			continue
		}
		count := len(mg.Members)
		if count > 0 {
			hasMarkush = true
			total *= count
		}
	}
	if !hasMarkush {
		return 0
	}
	return total
}

// computeNumericalRangeWidth computes a normalized [0,1] score representing
// the aggregate width of numerical ranges in a claim.
func computeNumericalRangeWidth(claim *ParsedClaim) float64 {
	if claim == nil || len(claim.NumericalRanges) == 0 {
		return 0
	}
	totalWidth := 0.0
	count := 0
	for _, nr := range claim.NumericalRanges {
		if nr == nil {
			continue
		}
		w := nr.Width
		if w <= 0 && nr.Max > nr.Min {
			w = nr.Max - nr.Min
		}
		if w > 0 {
			// Normalize: use sigmoid-like mapping so very large ranges
			// don't dominate. tanh(w/100) maps [0, ∞) → [0, 1).
			totalWidth += math.Tanh(w / 100.0)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return totalWidth / float64(count)
}

// extractKeyLimitations identifies the most restrictive features in a claim.
func extractKeyLimitations(claim *ParsedClaim) []string {
	if claim == nil {
		return nil
	}
	var limitations []string

	// Closed transitional phrase is itself a limitation.
	if claim.TransitionalType == PhraseConsistingOf {
		limitations = append(limitations, "Closed transitional phrase 'consisting of' excludes additional elements.")
	}

	// Specific numerical values are limiting.
	for _, nr := range claim.NumericalRanges {
		if nr == nil {
			continue
		}
		if nr.Max > 0 && nr.Min > 0 && (nr.Max-nr.Min) < 10 {
			limitations = append(limitations, fmt.Sprintf("Narrow numerical range: %.1f-%.1f %s", nr.Min, nr.Max, nr.Unit))
		}
	}

	// Essential features that are very specific.
	for _, f := range claim.Features {
		if f == nil {
			continue
		}
		if f.IsEssential && len(f.Text) > 50 {
			desc := f.Text
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			limitations = append(limitations, fmt.Sprintf("Detailed essential feature: %s", desc))
		}
	}

	// Many features = many limitations.
	if len(claim.Features) >= 8 {
		limitations = append(limitations, fmt.Sprintf("High feature count (%d) creates numerous limitations.", len(claim.Features)))
	}

	return limitations
}

// suggestBroadeningOpportunities generates suggestions for broadening a claim.
func suggestBroadeningOpportunities(claim *ParsedClaim, breadth float64) []string {
	if claim == nil {
		return nil
	}
	var suggestions []string

	if claim.TransitionalType == PhraseConsistingOf {
		suggestions = append(suggestions, "Replace 'consisting of' with 'comprising' to allow additional unrecited elements.")
	} else if claim.TransitionalType == PhraseConsistingEssentiallyOf {
		suggestions = append(suggestions, "Consider using 'comprising' instead of 'consisting essentially of' for maximum breadth.")
	}

	if len(claim.Features) >= 6 {
		suggestions = append(suggestions, "Reduce the number of technical features by removing non-essential limitations.")
	}

	if len(claim.MarkushGroups) == 0 && breadth < 0.60 {
		suggestions = append(suggestions, "Consider introducing Markush groups to cover structural alternatives.")
	}

	for _, mg := range claim.MarkushGroups {
		if mg != nil && !mg.IsOpenEnded {
			suggestions = append(suggestions, "Convert closed Markush groups to open-ended format (e.g., 'selected from the group including').")
			break
		}
	}

	for _, nr := range claim.NumericalRanges {
		if nr != nil && nr.Max > 0 && nr.Min > 0 && (nr.Max-nr.Min) < 20 {
			suggestions = append(suggestions, fmt.Sprintf("Widen numerical range %.1f-%.1f %s to cover broader operating conditions.", nr.Min, nr.Max, nr.Unit))
		}
	}

	return suggestions
}

// identifyNarrowingRisks identifies risks that could narrow the claim during prosecution.
func identifyNarrowingRisks(claim *ParsedClaim, breadth float64) []string {
	if claim == nil {
		return nil
	}
	var risks []string

	if breadth >= 0.75 {
		risks = append(risks, "Broad claims face higher risk of prior art rejections during prosecution, "+
			"which may require narrowing amendments.")
	}

	if claim.TransitionalType == PhraseComprising && len(claim.Features) <= 2 {
		risks = append(risks, "Very few features with open-ended language may trigger enablement "+
			"or written description challenges (35 U.S.C. §112).")
	}

	for _, mg := range claim.MarkushGroups {
		if mg == nil {
			continue
		}
		count := len(mg.Members)
		if count > 20 {
			risks = append(risks, fmt.Sprintf("Large Markush group (%d members) may face unity of invention "+
				"objections or restriction requirements.", count))
		}
	}

	if len(claim.Features) <= 2 {
		risks = append(risks, "Minimal feature count may be challenged as lacking sufficient "+
			"structural or functional specificity.")
	}

	return risks
}

// buildDependencyMap returns a map from claim number to its direct dependents.
func buildDependencyMap(claimSet *ParsedClaimSet) map[int][]int {
	depMap := make(map[int][]int)
	for _, claim := range claimSet.Claims {
		for _, depOn := range claim.DependsOn {
			depMap[depOn] = append(depMap[depOn], claim.ClaimNumber)
		}
	}
	return depMap
}

// findClaimByNumber finds a claim in the set by its number.
func findClaimByNumber(claimSet *ParsedClaimSet, number int) *ParsedClaim {
	for _, c := range claimSet.Claims {
		if c.ClaimNumber == number {
			return c
		}
	}
	return nil
}

// buildLayerLayout assigns claims to layers for hierarchical visualization.
// Layer 0 = independent claims, Layer 1 = direct dependents, etc.
func buildLayerLayout(claimSet *ParsedClaimSet) [][]int {
	if claimSet == nil || len(claimSet.Claims) == 0 {
		return nil
	}

	// Build parent map: claimNumber -> set of parent claim numbers.
	parentMap := make(map[int][]int)
	for _, c := range claimSet.Claims {
		parentMap[c.ClaimNumber] = c.DependsOn
	}

	// Assign layers via BFS from independent claims.
	layerOf := make(map[int]int)
	queue := make([]int, 0)

	// Independent claims are at layer 0.
	for _, c := range claimSet.Claims {
		if c.ClaimType == ClaimIndependent || len(c.DependsOn) == 0 {
			layerOf[c.ClaimNumber] = 0
			queue = append(queue, c.ClaimNumber)
		}
	}

	// Build children map.
	childrenMap := buildDependencyMap(claimSet)

	// BFS.
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		currentLayer := layerOf[current]
		for _, child := range childrenMap[current] {
			newLayer := currentLayer + 1
			if existingLayer, ok := layerOf[child]; ok {
				if newLayer > existingLayer {
					layerOf[child] = newLayer
				}
			} else {
				layerOf[child] = newLayer
			}
			queue = append(queue, child)
		}
	}

	// Handle orphan claims (not reachable from any independent claim).
	for _, c := range claimSet.Claims {
		if _, ok := layerOf[c.ClaimNumber]; !ok {
			layerOf[c.ClaimNumber] = 0
		}
	}

	// Group by layer.
	maxLayer := 0
	for _, l := range layerOf {
		if l > maxLayer {
			maxLayer = l
		}
	}

	layers := make([][]int, maxLayer+1)
	for i := range layers {
		layers[i] = []int{}
	}
	for _, c := range claimSet.Claims {
		l := layerOf[c.ClaimNumber]
		layers[l] = append(layers[l], c.ClaimNumber)
	}

	// Sort each layer by claim number.
	for i := range layers {
		sort.Ints(layers[i])
	}

	return layers
}

// cosineSimilarity computes the cosine similarity between two float32 vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// clamp01 clamps a value to the [0, 1] range.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// gapSeverityRank returns a numeric rank for sorting (higher = more severe).
func gapSeverityRank(s GapSeverity) int {
	switch s {
	case GapCritical:
		return 3
	case GapMajor:
		return 2
	case GapMinor:
		return 1
	default:
		return 0
	}
}

//Personal.AI order the ending
