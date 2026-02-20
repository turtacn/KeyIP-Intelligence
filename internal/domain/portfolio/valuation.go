// Package portfolio implements patent portfolio valuation algorithms and
// aggregation logic for computing portfolio-level financial metrics from
// individual patent factors.
package portfolio

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Valuator interface
// ─────────────────────────────────────────────────────────────────────────────

// Valuator computes the monetary value of a single patent given its
// multi-dimensional factors.  Implementations may use different weighting
// schemes, base values, or external market data sources.
type Valuator interface {
	// Valuate returns the estimated USD value of a patent or an error if
	// computation fails.
	Valuate(ctx context.Context, patentID common.ID, factors ValuationFactors) (float64, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// MultiFactorValuator — weighted linear combination implementation
// ─────────────────────────────────────────────────────────────────────────────

// MultiFactorValuator computes patent value as a weighted sum of technical,
// legal, and market scores, adjusted for remaining patent life.
//
// Formula:
//
//	DecayedBase = BaseValue × exp(-DecayRate × (20 - RemainingLife))
//	Value = DecayedBase × (TechWeight×TechScore + LegalWeight×LegalScore + MarketWeight×MarketScore)
//
// Default weights sum to 1.0.  BaseValue defaults to USD 100,000.
type MultiFactorValuator struct {
	// BaseValue is the starting valuation in USD before applying scores and decay.
	BaseValue float64

	// TechnicalWeight is the multiplier for TechnicalScore (default 0.4).
	TechnicalWeight float64

	// LegalWeight is the multiplier for LegalScore (default 0.3).
	LegalWeight float64

	// MarketWeight is the multiplier for MarketScore (default 0.3).
	MarketWeight float64

	// DecayRate controls how rapidly value decreases as RemainingLife shrinks.
	// Higher values cause steeper decay.  Default 0.05.
	DecayRate float64
}

// NewMultiFactorValuator constructs a MultiFactorValuator with sensible defaults.
func NewMultiFactorValuator() *MultiFactorValuator {
	return &MultiFactorValuator{
		BaseValue:       100000.0,
		TechnicalWeight: 0.4,
		LegalWeight:     0.3,
		MarketWeight:    0.3,
		DecayRate:       0.05,
	}
}

// Valuate implements the Valuator interface.
func (v *MultiFactorValuator) Valuate(ctx context.Context, patentID common.ID, factors ValuationFactors) (float64, error) {
	// Validate score ranges.
	if factors.TechnicalScore < 0 || factors.TechnicalScore > 1 {
		return 0, errors.InvalidParam(fmt.Sprintf("TechnicalScore must be in [0,1], got %.2f", factors.TechnicalScore))
	}
	if factors.LegalScore < 0 || factors.LegalScore > 1 {
		return 0, errors.InvalidParam(fmt.Sprintf("LegalScore must be in [0,1], got %.2f", factors.LegalScore))
	}
	if factors.MarketScore < 0 || factors.MarketScore > 1 {
		return 0, errors.InvalidParam(fmt.Sprintf("MarketScore must be in [0,1], got %.2f", factors.MarketScore))
	}

	// Apply time decay: patents nearing expiration lose value exponentially.
	// Remaining life typically ranges 0–20 years.
	yearsRemaining := math.Max(0, factors.RemainingLife)
	decayFactor := math.Exp(-v.DecayRate * (20.0 - yearsRemaining))
	decayedBase := v.BaseValue * decayFactor

	// Weighted sum of normalised scores.
	compositeScore := v.TechnicalWeight*factors.TechnicalScore +
		v.LegalWeight*factors.LegalScore +
		v.MarketWeight*factors.MarketScore

	value := decayedBase * compositeScore

	// Floor at zero (no negative valuations).
	if value < 0 {
		value = 0
	}

	return value, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Portfolio-level valuation aggregation
// ─────────────────────────────────────────────────────────────────────────────

// CalculatePortfolioValuation computes a ValuationResult for a collection of
// patents by applying the Valuator to each patent's factors, then aggregating
// total, average, median, highest, and lowest values.
//
// Returns an error if:
//   - patentFactors is empty
//   - any individual Valuate call fails
func CalculatePortfolioValuation(
	ctx context.Context,
	valuator Valuator,
	patentFactors map[common.ID]ValuationFactors,
) (*ValuationResult, error) {
	if len(patentFactors) == 0 {
		return nil, errors.InvalidParam("cannot valuate an empty portfolio")
	}

	breakdown := make([]PatentValuation, 0, len(patentFactors))
	values := make([]float64, 0, len(patentFactors))

	for patentID, factors := range patentFactors {
		value, err := valuator.Valuate(ctx, patentID, factors)
		if err != nil {
			return nil, errors.Wrap(err, errors.CodeInternal,
				fmt.Sprintf("failed to valuate patent %s", patentID))
		}

		breakdown = append(breakdown, PatentValuation{
			PatentID: patentID,
			Value:    value,
			Factors:  factors,
		})
		values = append(values, value)
	}

	// Compute aggregate statistics.
	total := sum(values)
	average := total / float64(len(values))
	median := computeMedian(values)
	highest := max(values)
	lowest := min(values)

	return &ValuationResult{
		TotalValue:    total,
		AverageValue:  average,
		MedianValue:   median,
		HighestValue:  highest,
		LowestValue:   lowest,
		ValuationDate: common.Timestamp(common.Timestamp{}.UTC()),
		Method:        "MultiFactorV1",
		Breakdown:     breakdown,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Claim breadth calculation
// ─────────────────────────────────────────────────────────────────────────────

// CalculateClaimBreadth estimates the scope coverage of a patent's claims
// based on the count and structure of independent and dependent claims.
//
// Formula (heuristic):
//
//	IndepRatio = independentClaimCount / claimCount
//	ElementFactor = 1.0 / (1.0 + avgElementCount/10.0)
//	Breadth = IndepRatio × ElementFactor
//
// Returns a score in [0.0, 1.0]:
//   - Higher scores: many independent claims with few limiting elements (broad protection).
//   - Lower scores: few independent claims or highly detailed elements (narrow protection).
func CalculateClaimBreadth(claimCount, independentClaimCount int, avgElementCount float64) float64 {
	if claimCount == 0 {
		return 0.0
	}

	indepRatio := float64(independentClaimCount) / float64(claimCount)

	// Penalise high element counts (more elements = narrower scope).
	elementFactor := 1.0 / (1.0 + avgElementCount/10.0)

	breadth := indepRatio * elementFactor

	// Clamp to [0, 1].
	if breadth < 0 {
		breadth = 0
	}
	if breadth > 1 {
		breadth = 1
	}

	return breadth
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper functions
// ─────────────────────────────────────────────────────────────────────────────

func sum(vals []float64) float64 {
	s := 0.0
	for _, v := range vals {
		s += v
	}
	return s
}

func max(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

func min(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func computeMedian(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2.0
}

//Personal.AI order the ending
