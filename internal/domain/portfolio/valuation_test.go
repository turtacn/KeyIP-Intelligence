// Package portfolio_test provides unit tests for the patent valuation algorithms.
package portfolio_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// MultiFactorValuator tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMultiFactorValuator_HighScoreFactors(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	factors := portfolio.ValuationFactors{
		TechnicalScore: 0.9,
		LegalScore:     0.9,
		MarketScore:    0.9,
		RemainingLife:  15.0,
	}

	value, err := v.Valuate(context.Background(), common.NewID(), factors)

	require.NoError(t, err)
	// With default BaseValue=100k and high scores (0.9) across all dimensions,
	// expect a value well above 50k (exact value depends on decay formula).
	assert.Greater(t, value, 50000.0)
}

func TestMultiFactorValuator_LowScoreFactors(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	factors := portfolio.ValuationFactors{
		TechnicalScore: 0.1,
		LegalScore:     0.1,
		MarketScore:    0.1,
		RemainingLife:  15.0,
	}

	value, err := v.Valuate(context.Background(), common.NewID(), factors)

	require.NoError(t, err)
	// Low scores should yield a low valuation.
	assert.Less(t, value, 20000.0)
}

func TestMultiFactorValuator_ZeroRemainingLife(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	v.DecayRate = 0.2 // Explicitly set high decay to test end-of-life valuation
	factors := portfolio.ValuationFactors{
		TechnicalScore: 0.8,
		LegalScore:     0.8,
		MarketScore:    0.8,
		RemainingLife:  0.0, // patent expired
	}

	value, err := v.Valuate(context.Background(), common.NewID(), factors)

	require.NoError(t, err)
	// With zero remaining life, decay should push value close to zero.
	assert.Less(t, value, 5000.0)
}

func TestMultiFactorValuator_InvalidTechnicalScore(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	factors := portfolio.ValuationFactors{
		TechnicalScore: 1.5, // out of range
		LegalScore:     0.5,
		MarketScore:    0.5,
		RemainingLife:  10.0,
	}

	_, err := v.Valuate(context.Background(), common.NewID(), factors)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TechnicalScore")
}

func TestMultiFactorValuator_InvalidLegalScore(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	factors := portfolio.ValuationFactors{
		TechnicalScore: 0.5,
		LegalScore:     -0.1, // negative
		MarketScore:    0.5,
		RemainingLife:  10.0,
	}

	_, err := v.Valuate(context.Background(), common.NewID(), factors)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LegalScore")
}

func TestMultiFactorValuator_InvalidMarketScore(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	factors := portfolio.ValuationFactors{
		TechnicalScore: 0.5,
		LegalScore:     0.5,
		MarketScore:    2.0, // out of range
		RemainingLife:  10.0,
	}

	_, err := v.Valuate(context.Background(), common.NewID(), factors)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MarketScore")
}

// ─────────────────────────────────────────────────────────────────────────────
// CalculatePortfolioValuation tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculatePortfolioValuation_MultiplePatents(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	patentFactors := map[common.ID]portfolio.ValuationFactors{
		common.NewID(): {
			TechnicalScore: 0.8,
			LegalScore:     0.7,
			MarketScore:    0.9,
			RemainingLife:  15.0,
		},
		common.NewID(): {
			TechnicalScore: 0.6,
			LegalScore:     0.6,
			MarketScore:    0.6,
			RemainingLife:  10.0,
		},
		common.NewID(): {
			TechnicalScore: 0.9,
			LegalScore:     0.8,
			MarketScore:    0.7,
			RemainingLife:  12.0,
		},
	}

	result, err := portfolio.CalculatePortfolioValuation(context.Background(), v, patentFactors)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.TotalValue, 0.0)
	assert.Greater(t, result.AverageValue, 0.0)
	assert.Greater(t, result.MedianValue, 0.0)
	assert.Greater(t, result.HighestValue, 0.0)
	assert.Greater(t, result.LowestValue, 0.0)
	assert.Equal(t, 3, len(result.Breakdown))
	assert.Equal(t, "MultiFactorV1", result.Method)

	// Sanity checks on aggregates.
	assert.InDelta(t, result.TotalValue/3, result.AverageValue, 0.01)
	assert.GreaterOrEqual(t, result.HighestValue, result.MedianValue)
	assert.GreaterOrEqual(t, result.MedianValue, result.LowestValue)
}

func TestCalculatePortfolioValuation_EmptyPortfolio(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	patentFactors := map[common.ID]portfolio.ValuationFactors{}

	result, err := portfolio.CalculatePortfolioValuation(context.Background(), v, patentFactors)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty")
}

func TestCalculatePortfolioValuation_SinglePatent(t *testing.T) {
	t.Parallel()

	v := portfolio.NewMultiFactorValuator()
	patentFactors := map[common.ID]portfolio.ValuationFactors{
		common.NewID(): {
			TechnicalScore: 0.7,
			LegalScore:     0.7,
			MarketScore:    0.7,
			RemainingLife:  15.0,
		},
	}

	result, err := portfolio.CalculatePortfolioValuation(context.Background(), v, patentFactors)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, result.TotalValue, result.AverageValue)
	assert.Equal(t, result.MedianValue, result.HighestValue)
	assert.Equal(t, result.LowestValue, result.HighestValue)
	assert.Len(t, result.Breakdown, 1)

	// Verify ValuationDate is set and not zero.
	assert.False(t, result.ValuationDate.IsZero())
}

// ─────────────────────────────────────────────────────────────────────────────
// CalculateClaimBreadth tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateClaimBreadth_ManyIndependentFewElements(t *testing.T) {
	t.Parallel()

	// Many independent claims with few elements → broad scope.
	breadth := portfolio.CalculateClaimBreadth(10, 5, 3.0)

	assert.Greater(t, breadth, 0.3, "High independent claim ratio should yield high breadth")
}

func TestCalculateClaimBreadth_FewIndependentManyElements(t *testing.T) {
	t.Parallel()

	// Few independent claims with many elements → narrow scope.
	breadth := portfolio.CalculateClaimBreadth(20, 2, 15.0)

	assert.Less(t, breadth, 0.2, "Low independent claim ratio and high element count should yield low breadth")
}

func TestCalculateClaimBreadth_ZeroClaims(t *testing.T) {
	t.Parallel()

	breadth := portfolio.CalculateClaimBreadth(0, 0, 0)

	assert.Equal(t, 0.0, breadth)
}

func TestCalculateClaimBreadth_AllIndependent(t *testing.T) {
	t.Parallel()

	// All claims are independent with minimal elements.
	breadth := portfolio.CalculateClaimBreadth(5, 5, 2.0)

	assert.Greater(t, breadth, 0.7, "All independent claims with low element count should yield very high breadth")
}

func TestCalculateClaimBreadth_BoundedOutputRange(t *testing.T) {
	t.Parallel()

	cases := []struct {
		claims      int
		independent int
		avgElements float64
	}{
		{10, 5, 5.0},
		{20, 2, 10.0},
		{15, 8, 3.0},
		{5, 1, 20.0},
		{50, 25, 7.0},
	}

	for _, tc := range cases {
		breadth := portfolio.CalculateClaimBreadth(tc.claims, tc.independent, tc.avgElements)
		assert.GreaterOrEqual(t, breadth, 0.0)
		assert.LessOrEqual(t, breadth, 1.0)
	}
}

//Personal.AI order the ending
