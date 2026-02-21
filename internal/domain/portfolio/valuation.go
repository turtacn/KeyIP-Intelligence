package portfolio

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ValuationDimension represents a specific aspect of patent value.
type ValuationDimension string

const (
	DimensionTechnical  ValuationDimension = "Technical"
	DimensionLegal      ValuationDimension = "Legal"
	DimensionCommercial ValuationDimension = "Commercial"
	DimensionStrategic  ValuationDimension = "Strategic"
)

// ValuationTier represents the quality category of a patent.
type ValuationTier string

const (
	TierS ValuationTier = "S"
	TierA ValuationTier = "A"
	TierB ValuationTier = "B"
	TierC ValuationTier = "C"
	TierD ValuationTier = "D"
)

// DimensionScore holds the score and factors for a single dimension.
type DimensionScore struct {
	Dimension   ValuationDimension `json:"dimension"`
	Score       float64            `json:"score"`
	Factors     map[string]float64 `json:"factors"`
	Explanation string             `json:"explanation"`
}

func (ds *DimensionScore) Validate() error {
	if ds.Score < 0 || ds.Score > 100 {
		return errors.InvalidParam(fmt.Sprintf("%s score must be between 0 and 100", ds.Dimension))
	}
	return nil
}

// ValuationRecommendation provides actionable advice based on valuation.
type ValuationRecommendation struct {
	Type     string `json:"type"`     // maintain / strengthen / enforce / abandon
	Priority string `json:"priority"` // critical / high / medium / low
	Action   string `json:"action"`
	Reason   string `json:"reason"`
}

// WeightConfig defines the weights for each valuation dimension.
type WeightConfig struct {
	TechnicalWeight float64 `json:"technical_weight"`
	LegalWeight     float64 `json:"legal_weight"`
	CommercialWeight float64 `json:"commercial_weight"`
	StrategicWeight float64 `json:"strategic_weight"`
}

// DefaultWeightConfig returns the default weight distribution.
func DefaultWeightConfig() *WeightConfig {
	return &WeightConfig{
		TechnicalWeight:  0.20,
		LegalWeight:      0.25,
		CommercialWeight: 0.30,
		StrategicWeight:  0.25,
	}
}

// Validate ensures weights are valid and sum to 1.0.
func (wc *WeightConfig) Validate() error {
	weights := []float64{wc.TechnicalWeight, wc.LegalWeight, wc.CommercialWeight, wc.StrategicWeight}
	sum := 0.0
	for _, w := range weights {
		if w < 0 || w > 1 {
			return errors.InvalidParam("weights must be between 0 and 1")
		}
		sum += w
	}
	if math.Abs(sum-1.0) > 0.001 {
		return errors.InvalidParam(fmt.Sprintf("weights must sum to 1.0, got %f", sum))
	}
	return nil
}

// PatentValuation holds the full valuation data for a single patent.
type PatentValuation struct {
	PatentID        string                                  `json:"patent_id"`
	DimensionScores map[ValuationDimension]*DimensionScore `json:"dimension_scores"`
	OverallScore    float64                                 `json:"overall_score"`
	Tier            ValuationTier                            `json:"tier"`
	WeightConfig    *WeightConfig                            `json:"weight_config"`
	Recommendations []ValuationRecommendation                `json:"recommendations"`
	EvaluatedAt     time.Time                                `json:"evaluated_at"`
}

// PortfolioValuation holds the aggregated valuation data for a portfolio.
type PortfolioValuation struct {
	PortfolioID       string                      `json:"portfolio_id"`
	PatentValuations  map[string]*PatentValuation `json:"patent_valuations"`
	AggregateScore    float64                     `json:"aggregate_score"`
	TierDistribution  map[ValuationTier]int       `json:"tier_distribution"`
	DimensionAverages map[ValuationDimension]float64 `json:"dimension_averages"`
	TopPerformers     []string                    `json:"top_performers"`
	UnderPerformers   []string                    `json:"under_performers"`
	EvaluatedAt       time.Time                   `json:"evaluated_at"`
}

// DimensionEvaluator defines the interface for evaluating a single dimension.
type DimensionEvaluator interface {
	Evaluate(ctx context.Context, patentID string) (*DimensionScore, error)
	Dimension() ValuationDimension
}

// ValuationEngine defines the core domain logic for valuation.
type ValuationEngine interface {
	EvaluatePatent(ctx context.Context, patentID string, config *WeightConfig) (*PatentValuation, error)
	EvaluatePortfolio(ctx context.Context, portfolioID string, config *WeightConfig) (*PortfolioValuation, error)
	ComparativeValuation(ctx context.Context, portfolioIDs []string, config *WeightConfig) (map[string]*PortfolioValuation, error)
}

type valuationEngineImpl struct {
	repo       PortfolioRepository
	evaluators map[ValuationDimension]DimensionEvaluator
}

// NewValuationEngine creates a new ValuationEngine.
func NewValuationEngine(repo PortfolioRepository, evaluators ...DimensionEvaluator) ValuationEngine {
	if len(evaluators) == 0 {
		panic("at least one dimension evaluator must be provided")
	}
	evalMap := make(map[ValuationDimension]DimensionEvaluator)
	for _, e := range evaluators {
		evalMap[e.Dimension()] = e
	}
	return &valuationEngineImpl{
		repo:       repo,
		evaluators: evalMap,
	}
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

	for dim, evaluator := range e.evaluators {
		score, err := evaluator.Evaluate(ctx, patentID)
		if err != nil {
			return nil, err
		}
		scores[dim] = score

		switch dim {
		case DimensionTechnical:
			overallScore += score.Score * config.TechnicalWeight
		case DimensionLegal:
			overallScore += score.Score * config.LegalWeight
		case DimensionCommercial:
			overallScore += score.Score * config.CommercialWeight
		case DimensionStrategic:
			overallScore += score.Score * config.StrategicWeight
		}
	}

	return &PatentValuation{
		PatentID:        patentID,
		DimensionScores: scores,
		OverallScore:    overallScore,
		Tier:            DetermineValuationTier(overallScore),
		WeightConfig:    config,
		Recommendations: GenerateRecommendations(scores),
		EvaluatedAt:     time.Now().UTC(),
	}, nil
}

func (e *valuationEngineImpl) EvaluatePortfolio(ctx context.Context, portfolioID string, config *WeightConfig) (*PortfolioValuation, error) {
	p, err := e.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	valuations := make(map[string]*PatentValuation)
	tierDist := make(map[ValuationTier]int)
	dimSums := make(map[ValuationDimension]float64)
	totalScore := 0.0

	for _, pid := range p.PatentIDs {
		val, err := e.EvaluatePatent(ctx, pid, config)
		if err != nil {
			return nil, err
		}
		valuations[pid] = val
		tierDist[val.Tier]++
		totalScore += val.OverallScore
		for dim, ds := range val.DimensionScores {
			dimSums[dim] += ds.Score
		}
	}

	patentCount := float64(len(p.PatentIDs))
	aggregateScore := 0.0
	dimAverages := make(map[ValuationDimension]float64)
	if patentCount > 0 {
		aggregateScore = totalScore / patentCount
		for dim, sum := range dimSums {
			dimAverages[dim] = sum / patentCount
		}
	}

	// Sort for top/under performers
	type pidScore struct {
		id    string
		score float64
	}
	sorted := make([]pidScore, 0, len(valuations))
	for id, val := range valuations {
		sorted = append(sorted, pidScore{id, val.OverallScore})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	top := make([]string, 0)
	for i := 0; i < len(sorted) && i < 10; i++ {
		top = append(top, sorted[i].id)
	}

	under := make([]string, 0)
	for i := len(sorted) - 1; i >= 0 && len(under) < 10; i-- {
		under = append(under, sorted[i].id)
	}

	return &PortfolioValuation{
		PortfolioID:       portfolioID,
		PatentValuations:  valuations,
		AggregateScore:    aggregateScore,
		TierDistribution:  tierDist,
		DimensionAverages: dimAverages,
		TopPerformers:     top,
		UnderPerformers:   under,
		EvaluatedAt:       time.Now().UTC(),
	}, nil
}

func (e *valuationEngineImpl) ComparativeValuation(ctx context.Context, portfolioIDs []string, config *WeightConfig) (map[string]*PortfolioValuation, error) {
	results := make(map[string]*PortfolioValuation)
	for _, id := range portfolioIDs {
		val, err := e.EvaluatePortfolio(ctx, id, config)
		if err != nil {
			return nil, err
		}
		results[id] = val
	}
	return results, nil
}

// DetermineValuationTier maps a score to a tier.
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

// GenerateRecommendations generates advice based on dimension scores.
func GenerateRecommendations(scores map[ValuationDimension]*DimensionScore) []ValuationRecommendation {
	recs := make([]ValuationRecommendation, 0)

	tech, okT := scores[DimensionTechnical]
	legal, okL := scores[DimensionLegal]
	comm, okC := scores[DimensionCommercial]
	strat, okS := scores[DimensionStrategic]

	if okT && tech.Score < 40 {
		recs = append(recs, ValuationRecommendation{
			Type:     "strengthen",
			Priority: "high",
			Action:   "Strengthen technical documentation and innovation depth.",
			Reason:   "Technical score is below threshold.",
		})
	}

	if okL && legal.Score < 30 {
		recs = append(recs, ValuationRecommendation{
			Type:     "enforce",
			Priority: "critical",
			Action:   "Review and enforce legal claims or file continuations.",
			Reason:   "Legal strength is critically low.",
		})
	}

	if okC && okS && comm.Score > 80 && strat.Score < 50 {
		recs = append(recs, ValuationRecommendation{
			Type:     "maintain",
			Priority: "medium",
			Action:   "Maintain commercial value while improving strategic alignment.",
			Reason:   "High commercial value but weak strategic positioning.",
		})
	}

	allHigh := true
	allLow := true
	for _, s := range scores {
		if s.Score <= 75 {
			allHigh = false
		}
		if s.Score >= 30 {
			allLow = false
		}
	}

	if len(scores) > 0 {
		if allHigh {
			recs = append(recs, ValuationRecommendation{
				Type:     "maintain",
				Priority: "low",
				Action:   "Keep current maintenance strategy.",
				Reason:   "Excellent scores across all dimensions.",
			})
		}
		if allLow {
			recs = append(recs, ValuationRecommendation{
				Type:     "abandon",
				Priority: "medium",
				Action:   "Consider abandoning or licensing out this patent.",
				Reason:   "Poor performance across all value dimensions.",
			})
		}
	}

	return recs
}

//Personal.AI order the ending
