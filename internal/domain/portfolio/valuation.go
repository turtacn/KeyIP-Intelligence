package portfolio

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ValuationDimension represents a dimension of patent valuation.
type ValuationDimension string

const (
	DimensionTechnical  ValuationDimension = "technical"
	DimensionLegal      ValuationDimension = "legal"
	DimensionCommercial ValuationDimension = "commercial"
	DimensionStrategic  ValuationDimension = "strategic"
)

// ValuationTier represents the classification of a patent based on value.
type ValuationTier string

const (
	TierS ValuationTier = "S" // Strategic (90-100)
	TierA ValuationTier = "A" // Core (75-89)
	TierB ValuationTier = "B" // Important (60-74)
	TierC ValuationTier = "C" // General (40-59)
	TierD ValuationTier = "D" // Low Value (0-39)
)

// DimensionScore holds the score for a specific dimension.
type DimensionScore struct {
	Dimension   ValuationDimension `json:"dimension"`
	Score       float64            `json:"score"`
	Factors     map[string]float64 `json:"factors"`
	Explanation string             `json:"explanation"`
}

// ValuationRecommendation represents an actionable suggestion.
type ValuationRecommendation struct {
	Type     string `json:"type"`     // maintain, strengthen, enforce, abandon
	Priority string `json:"priority"` // critical, high, medium, low
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// WeightConfig defines the weights for valuation dimensions.
type WeightConfig struct {
	TechnicalWeight  float64 `json:"technical_weight"`
	LegalWeight      float64 `json:"legal_weight"`
	CommercialWeight float64 `json:"commercial_weight"`
	StrategicWeight  float64 `json:"strategic_weight"`
}

// PatentValuation represents the valuation result for a single patent.
type PatentValuation struct {
	PatentID        string                            `json:"patent_id"`
	DimensionScores map[ValuationDimension]*DimensionScore `json:"dimension_scores"`
	OverallScore    float64                           `json:"overall_score"`
	Tier            ValuationTier                     `json:"tier"`
	WeightConfig    *WeightConfig                     `json:"weight_config"`
	Recommendations []ValuationRecommendation         `json:"recommendations"`
	EvaluatedAt     time.Time                         `json:"evaluated_at"`
}

// PortfolioValuation represents the valuation result for a portfolio.
type PortfolioValuation struct {
	PortfolioID       string                          `json:"portfolio_id"`
	PatentValuations  map[string]*PatentValuation     `json:"patent_valuations"`
	AggregateScore    float64                         `json:"aggregate_score"`
	TierDistribution  map[ValuationTier]int           `json:"tier_distribution"`
	DimensionAverages map[ValuationDimension]float64  `json:"dimension_averages"`
	TopPerformers     []string                        `json:"top_performers"`
	UnderPerformers   []string                        `json:"under_performers"`
	EvaluatedAt       time.Time                       `json:"evaluated_at"`
}

// ValuationEngine defines the interface for valuation operations.
type ValuationEngine interface {
	EvaluatePatent(ctx context.Context, patentID string, config *WeightConfig) (*PatentValuation, error)
	EvaluatePortfolio(ctx context.Context, portfolioID string, config *WeightConfig) (*PortfolioValuation, error)
	ComparativeValuation(ctx context.Context, portfolioIDs []string, config *WeightConfig) (map[string]*PortfolioValuation, error)
}

// DimensionEvaluator defines the interface for evaluating a specific dimension.
type DimensionEvaluator interface {
	Evaluate(ctx context.Context, patentID string) (*DimensionScore, error)
	Dimension() ValuationDimension
}

// valuationEngineImpl implements ValuationEngine.
type valuationEngineImpl struct {
	repo       PortfolioRepository
	evaluators map[ValuationDimension]DimensionEvaluator
}

// NewValuationEngine creates a new ValuationEngine.
func NewValuationEngine(repo PortfolioRepository, evaluators ...DimensionEvaluator) ValuationEngine {
	engine := &valuationEngineImpl{
		repo:       repo,
		evaluators: make(map[ValuationDimension]DimensionEvaluator),
	}
	for _, e := range evaluators {
		engine.evaluators[e.Dimension()] = e
	}
	return engine
}

// DefaultWeightConfig returns the default weights.
func DefaultWeightConfig() *WeightConfig {
	return &WeightConfig{
		TechnicalWeight:  0.20,
		LegalWeight:      0.25,
		CommercialWeight: 0.30,
		StrategicWeight:  0.25,
	}
}

// Validate checks if the weight config is valid.
func (wc *WeightConfig) Validate() error {
	if wc.TechnicalWeight < 0 || wc.TechnicalWeight > 1 ||
		wc.LegalWeight < 0 || wc.LegalWeight > 1 ||
		wc.CommercialWeight < 0 || wc.CommercialWeight > 1 ||
		wc.StrategicWeight < 0 || wc.StrategicWeight > 1 {
		return apperrors.NewValidation("weights must be between 0 and 1")
	}
	sum := wc.TechnicalWeight + wc.LegalWeight + wc.CommercialWeight + wc.StrategicWeight
	if math.Abs(sum-1.0) > 0.001 {
		return apperrors.NewValidation(fmt.Sprintf("weights must sum to 1.0, got %f", sum))
	}
	return nil
}

func (e *valuationEngineImpl) EvaluatePatent(ctx context.Context, patentID string, config *WeightConfig) (*PatentValuation, error) {
	if config == nil {
		config = DefaultWeightConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	scores := make(map[ValuationDimension]*DimensionScore)
	overallScore := 0.0

	// Helper to evaluate a dimension
	evaluateDim := func(dim ValuationDimension, weight float64) error {
		if ev, ok := e.evaluators[dim]; ok {
			score, err := ev.Evaluate(ctx, patentID)
			if err != nil {
				return err
			}
			scores[dim] = score
			overallScore += score.Score * weight
		}
		return nil
	}

	if err := evaluateDim(DimensionTechnical, config.TechnicalWeight); err != nil {
		return nil, err
	}
	if err := evaluateDim(DimensionLegal, config.LegalWeight); err != nil {
		return nil, err
	}
	if err := evaluateDim(DimensionCommercial, config.CommercialWeight); err != nil {
		return nil, err
	}
	if err := evaluateDim(DimensionStrategic, config.StrategicWeight); err != nil {
		return nil, err
	}

	pv := &PatentValuation{
		PatentID:        patentID,
		DimensionScores: scores,
		OverallScore:    overallScore,
		Tier:            DetermineValuationTier(overallScore),
		WeightConfig:    config,
		Recommendations: GenerateRecommendations(scores),
		EvaluatedAt:     time.Now().UTC(),
	}
	return pv, nil
}

func (e *valuationEngineImpl) EvaluatePortfolio(ctx context.Context, portfolioID string, config *WeightConfig) (*PortfolioValuation, error) {
	p, err := e.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	patentValuations := make(map[string]*PatentValuation)
	totalScore := 0.0
	tierDist := make(map[ValuationTier]int)
	dimSums := make(map[ValuationDimension]float64)

	type scoredPatent struct {
		ID    string
		Score float64
	}
	patentScores := make([]scoredPatent, 0, len(p.PatentIDs))

	for _, pid := range p.PatentIDs {
		pv, err := e.EvaluatePatent(ctx, pid, config)
		if err != nil {
			return nil, err
		}
		patentValuations[pid] = pv
		totalScore += pv.OverallScore
		tierDist[pv.Tier]++
		for dim, score := range pv.DimensionScores {
			dimSums[dim] += score.Score
		}
		patentScores = append(patentScores, scoredPatent{ID: pid, Score: pv.OverallScore})
	}

	count := float64(len(p.PatentIDs))
	avgScore := 0.0
	dimAvgs := make(map[ValuationDimension]float64)
	if count > 0 {
		avgScore = totalScore / count
		for dim, sum := range dimSums {
			dimAvgs[dim] = sum / count
		}
	}

	// Sort for Top/Under performers
	sort.Slice(patentScores, func(i, j int) bool {
		return patentScores[i].Score > patentScores[j].Score
	})

	limit := 10
	if len(patentScores) < 10 {
		limit = len(patentScores)
	}

	top := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		top = append(top, patentScores[i].ID)
	}

	under := make([]string, 0, limit)
	// Bottom 10 (lowest scores)
	// patentScores is sorted DESC (high to low). So bottom are at the end.
	// We want the absolute lowest.
	start := len(patentScores) - 1
	end := len(patentScores) - limit
	if end < 0 {
		end = 0
	}
	for i := start; i >= end; i-- {
		under = append(under, patentScores[i].ID)
	}

	return &PortfolioValuation{
		PortfolioID:       portfolioID,
		PatentValuations:  patentValuations,
		AggregateScore:    avgScore,
		TierDistribution:  tierDist,
		DimensionAverages: dimAvgs,
		TopPerformers:     top,
		UnderPerformers:   under,
		EvaluatedAt:       time.Now().UTC(),
	}, nil
}

func (e *valuationEngineImpl) ComparativeValuation(ctx context.Context, portfolioIDs []string, config *WeightConfig) (map[string]*PortfolioValuation, error) {
	results := make(map[string]*PortfolioValuation)
	for _, pid := range portfolioIDs {
		pv, err := e.EvaluatePortfolio(ctx, pid, config)
		if err != nil {
			return nil, err
		}
		results[pid] = pv
	}
	return results, nil
}

// DetermineValuationTier determines the tier based on score.
func DetermineValuationTier(score float64) ValuationTier {
	if score >= 90 {
		return TierS
	}
	if score >= 75 {
		return TierA
	}
	if score >= 60 {
		return TierB
	}
	if score >= 40 {
		return TierC
	}
	return TierD
}

// GenerateRecommendations generates actionable advice.
func GenerateRecommendations(scores map[ValuationDimension]*DimensionScore) []ValuationRecommendation {
	recs := []ValuationRecommendation{}

	tech, hasTech := scores[DimensionTechnical]
	legal, hasLegal := scores[DimensionLegal]
	comm, hasComm := scores[DimensionCommercial]
	strat, hasStrat := scores[DimensionStrategic]

	if hasTech && tech.Score < 40 {
		recs = append(recs, ValuationRecommendation{
			Type:     "strengthen",
			Priority: "high",
			Action:   "Technical Strengthening",
			Reason:   "Technical score is low (<40)",
		})
	}
	if hasLegal && legal.Score < 30 {
		recs = append(recs, ValuationRecommendation{
			Type:     "enforce",
			Priority: "critical",
			Action:   "Legal Enforcement Review",
			Reason:   "Legal score is critically low (<30)",
		})
	}
	if hasComm && hasStrat && comm.Score > 80 && strat.Score < 50 {
		recs = append(recs, ValuationRecommendation{
			Type:     "maintain",
			Priority: "medium",
			Action:   "Strategic Alignment",
			Reason:   "High commercial value but low strategic fit",
		})
	}

	allHigh := true
	allLow := true
	for _, s := range scores {
		if s.Score < 75 {
			allHigh = false
		}
		if s.Score >= 30 {
			allLow = false
		}
	}

	if allHigh && len(scores) > 0 {
		recs = append(recs, ValuationRecommendation{
			Type:     "maintain",
			Priority: "low",
			Action:   "Maintenance",
			Reason:   "High value across all dimensions",
		})
	}
	if allLow && len(scores) > 0 {
		recs = append(recs, ValuationRecommendation{
			Type:     "abandon",
			Priority: "medium",
			Action:   "Abandonment Consideration",
			Reason:   "Low value across all dimensions",
		})
	}

	return recs
}

//Personal.AI order the ending
