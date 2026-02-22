package claim_bert

import (
	"context"
	"errors"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ScopeBreadthLevel defines the breadth of scope.
type ScopeBreadthLevel string

const (
	ScopeBroad      ScopeBreadthLevel = "Broad"
	ScopeModerate   ScopeBreadthLevel = "Moderate"
	ScopeNarrow     ScopeBreadthLevel = "Narrow"
	ScopeVeryNarrow ScopeBreadthLevel = "VeryNarrow"
)

// ScopeRelationship defines the relationship between scopes.
type ScopeRelationship string

const (
	RelAContainsB  ScopeRelationship = "AContainsB"
	RelBContainsA  ScopeRelationship = "BContainsA"
	RelOverlapping ScopeRelationship = "Overlapping"
	RelDisjoint    ScopeRelationship = "Disjoint"
	RelEquivalent  ScopeRelationship = "Equivalent"
)

// GapSeverity defines the severity of a scope gap.
type GapSeverity string

const (
	SeverityCritical GapSeverity = "Critical"
	SeverityMajor    GapSeverity = "Major"
	SeverityMinor    GapSeverity = "Minor"
)

// ScopeAnalysis represents the result of scope analysis.
type ScopeAnalysis struct {
	ClaimNumber             int
	BreadthScore            float64
	BreadthLevel            ScopeBreadthLevel
	TransitionalPhraseImpact string
	FeatureCount            int
	MarkushExpansion        int
	NumericalRangeWidth     float64
	KeyLimitations          []string
	BroadeningOpportunities []string
	NarrowingRisks          []string
}

// ScopeComparison represents the comparison between two scopes.
type ScopeComparison struct {
	ClaimA         int
	ClaimB         int
	Relationship   ScopeRelationship
	OverlapScore   float64
	SharedFeatures []*TechnicalFeature
	UniqueToA      []*TechnicalFeature
	UniqueToB      []*TechnicalFeature
	Analysis       string
}

// ScopeGap represents a identified gap in scope.
type ScopeGap struct {
	Description    string
	AffectedClaims []int
	Severity       GapSeverity
	Recommendation string
}

// ClaimSetScopeAnalysis represents analysis of a claim set scope.
type ClaimSetScopeAnalysis struct {
	OverallCoverage float64
	WidestClaim     int
	NarrowestClaim  int
	Gaps            []*ScopeGap
	Visualization   *ScopeVisualizationData
}

// ScopeVisualizationData represents visualization data.
type ScopeVisualizationData struct {
	Nodes       []ScopeNode
	Edges       []ScopeEdge
	Layers      [][]int
	HeatmapData [][]float64
}

type ScopeNode struct {
	ID    int
	Label string
	Size  float64
}

type ScopeEdge struct {
	Source int
	Target int
	Type   string
}

// ScopeAnalyzer defines the interface for scope analysis.
type ScopeAnalyzer interface {
	AnalyzeScope(ctx context.Context, claim *ParsedClaim) (*ScopeAnalysis, error)
	CompareScopes(ctx context.Context, claimA, claimB *ParsedClaim) (*ScopeComparison, error)
	AnalyzeClaimSetScope(ctx context.Context, claimSet *ParsedClaimSet) (*ClaimSetScopeAnalysis, error)
	ComputeScopeBreadth(ctx context.Context, claim *ParsedClaim) (float64, error)
	IdentifyScopeGaps(ctx context.Context, claimSet *ParsedClaimSet) ([]*ScopeGap, error)
	GenerateScopeVisualization(ctx context.Context, claimSet *ParsedClaimSet) (*ScopeVisualizationData, error)
}

// scopeAnalyzerImpl implements ScopeAnalyzer.
type scopeAnalyzerImpl struct {
	backend common.ModelBackend // Used for semantic similarity of features
	logger  logging.Logger
}

// NewScopeAnalyzer creates a new ScopeAnalyzer.
func NewScopeAnalyzer(backend common.ModelBackend, logger logging.Logger) ScopeAnalyzer {
	return &scopeAnalyzerImpl{
		backend: backend,
		logger:  logger,
	}
}

func (s *scopeAnalyzerImpl) AnalyzeScope(ctx context.Context, claim *ParsedClaim) (*ScopeAnalysis, error) {
	if claim == nil {
		return nil, errors.New("claim is nil")
	}

	score, err := s.ComputeScopeBreadth(ctx, claim)
	if err != nil {
		return nil, err
	}

	level := ScopeModerate
	if score >= 0.75 {
		level = ScopeBroad
	} else if score < 0.25 {
		level = ScopeVeryNarrow
	} else if score < 0.50 {
		level = ScopeNarrow
	}

	return &ScopeAnalysis{
		ClaimNumber:  claim.ClaimNumber,
		BreadthScore: score,
		BreadthLevel: level,
		FeatureCount: len(claim.Features),
	}, nil
}

func (s *scopeAnalyzerImpl) ComputeScopeBreadth(ctx context.Context, claim *ParsedClaim) (float64, error) {
	// Base score from model (assumed passed in ParsedClaim or calculated here)
	baseScore := claim.ScopeScore
	if baseScore == 0 {
		baseScore = 0.5 // Default
	}

	// Adjustments
	if strings.Contains(strings.ToLower(claim.TransitionalPhrase), "comprising") {
		baseScore += 0.1
	} else if strings.Contains(strings.ToLower(claim.TransitionalPhrase), "consisting of") {
		baseScore -= 0.15
	}

	if len(claim.Features) <= 3 {
		baseScore += 0.05
	} else if len(claim.Features) >= 8 {
		baseScore -= 0.10
	}

	if len(claim.MarkushGroups) > 0 {
		baseScore += 0.08
	}

	// Clamp
	if baseScore > 1.0 { baseScore = 1.0 }
	if baseScore < 0.0 { baseScore = 0.0 }

	return baseScore, nil
}

func (s *scopeAnalyzerImpl) CompareScopes(ctx context.Context, claimA, claimB *ParsedClaim) (*ScopeComparison, error) {
	// Simplified feature comparison
	// Ideally use semantic similarity on feature text

	// Assume Exact match if ID matches (if mapped) or text match
	shared := []*TechnicalFeature{}
	uniqueA := []*TechnicalFeature{}
	uniqueB := []*TechnicalFeature{}

	// Map texts
	mapA := make(map[string]*TechnicalFeature)
	for _, f := range claimA.Features {
		mapA[f.Text] = f
	}

	for _, f := range claimB.Features {
		if _, ok := mapA[f.Text]; ok {
			shared = append(shared, f)
			delete(mapA, f.Text)
		} else {
			uniqueB = append(uniqueB, f)
		}
	}
	for _, f := range mapA {
		uniqueA = append(uniqueA, f)
	}

	overlap := 0.0
	maxLen := float64(max(len(claimA.Features), len(claimB.Features)))
	if maxLen > 0 {
		overlap = float64(len(shared)) / maxLen
	}

	rel := RelOverlapping
	if len(uniqueA) == 0 && len(uniqueB) == 0 {
		rel = RelEquivalent
	} else if len(uniqueA) == 0 {
		// A has no unique features, so A is subset of features of B?
		// Wait, Scope: Fewer features = Broader scope.
		// If A features <= B features, A scope >= B scope.
		// If A features is subset of B features, A contains B.
		rel = RelAContainsB
	} else if len(uniqueB) == 0 {
		rel = RelBContainsA
	} else if overlap == 0 {
		rel = RelDisjoint
	}

	return &ScopeComparison{
		ClaimA:         claimA.ClaimNumber,
		ClaimB:         claimB.ClaimNumber,
		Relationship:   rel,
		OverlapScore:   overlap,
		SharedFeatures: shared,
		UniqueToA:      uniqueA,
		UniqueToB:      uniqueB,
	}, nil
}

func (s *scopeAnalyzerImpl) AnalyzeClaimSetScope(ctx context.Context, claimSet *ParsedClaimSet) (*ClaimSetScopeAnalysis, error) {
	// ... logic to aggregate ...
	return &ClaimSetScopeAnalysis{}, nil
}

func (s *scopeAnalyzerImpl) IdentifyScopeGaps(ctx context.Context, claimSet *ParsedClaimSet) ([]*ScopeGap, error) {
	return []*ScopeGap{}, nil
}

func (s *scopeAnalyzerImpl) GenerateScopeVisualization(ctx context.Context, claimSet *ParsedClaimSet) (*ScopeVisualizationData, error) {
	return &ScopeVisualizationData{}, nil
}

func max(a, b int) int {
	if a > b { return a }
	return b
}

//Personal.AI order the ending
