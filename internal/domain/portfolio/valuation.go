package portfolio

import (
	"math"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// WeightConfig defines the weight configuration for valuation.
type WeightConfig struct {
	Technical float64 `json:"technical"`
	Legal     float64 `json:"legal"`
	Market    float64 `json:"market"`
	Strategic float64 `json:"strategic"`
}

// DefaultWeightConfig returns the default weight configuration.
func DefaultWeightConfig() WeightConfig {
	return WeightConfig{
		Technical: 0.3,
		Legal:     0.3,
		Market:    0.2,
		Strategic: 0.2,
	}
}

// Validate validates the weight configuration.
func (c *WeightConfig) Validate() error {
	sum := c.Technical + c.Legal + c.Market + c.Strategic
	if math.Abs(sum-1.0) > 0.001 {
		return errors.NewValidation("weights must sum to 1.0 (tolerance 0.001)")
	}
	return nil
}

// EvaluatePatent calculates the valuation score for a patent.
// This is a pure domain function.
// In a real implementation, this would likely take a more complex input struct or
// dependency injection for scoring models. For now, we simulate scoring based on input details.
func EvaluatePatent(patentID string, details map[string]float64, config WeightConfig) (*Valuation, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Assuming details contains raw scores normalized to 0-100
	techScore := details["technical"]
	legalScore := details["legal"]
	marketScore := details["market"]
	strategicScore := details["strategic"]

	compositeScore := (techScore * config.Technical) +
		(legalScore * config.Legal) +
		(marketScore * config.Market) +
		(strategicScore * config.Strategic)

	tier := calculateTier(compositeScore)
	now := time.Time(common.NewTimestamp())

	return &Valuation{
		ID:             string(common.NewID()),
		PatentID:       patentID,
		TechnicalScore: techScore,
		LegalScore:     legalScore,
		MarketScore:    marketScore,
		StrategicScore: strategicScore,
		CompositeScore: compositeScore,
		Tier:           tier,
		Currency:       "USD",
		CreatedAt:      now,
		ValidFrom:      now,
		ScoringDetails: map[string]any{
			"weights": config,
		},
	}, nil
}

// EvaluatePortfolio calculates the aggregated valuation for a portfolio.
func EvaluatePortfolio(portfolioID string, valuations []*Valuation) (*Summary, error) {
	if len(valuations) == 0 {
		return &Summary{
			TotalPatents: 0,
			AverageScore: 0,
		}, nil
	}

	var totalScore float64
	var totalValue int64

	for _, v := range valuations {
		totalScore += v.CompositeScore
		if v.MonetaryValueMid != nil {
			totalValue += *v.MonetaryValueMid
		}
	}

	avgScore := totalScore / float64(len(valuations))

	return &Summary{
		TotalPatents:   len(valuations),
		AverageScore:   avgScore,
		TotalValuation: totalValue,
	}, nil
}

func calculateTier(score float64) ValuationTier {
	switch {
	case score >= 90:
		return ValuationTierS
	case score >= 80:
		return ValuationTierA
	case score >= 70:
		return ValuationTierB
	case score >= 60:
		return ValuationTierC
	default:
		return ValuationTierD
	}
}

//Personal.AI order the ending
