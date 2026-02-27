package portfolio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWeightConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  WeightConfig
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: WeightConfig{
				Technical: 0.3,
				Legal:     0.3,
				Market:    0.2,
				Strategic: 0.2,
			},
			wantErr: false,
		},
		{
			name: "Invalid Sum",
			config: WeightConfig{
				Technical: 0.3,
				Legal:     0.3,
				Market:    0.2,
				Strategic: 0.3, // Sum = 1.1
			},
			wantErr: true,
		},
		{
			name: "Zero Sum",
			config: WeightConfig{
				Technical: 0,
				Legal:     0,
				Market:    0,
				Strategic: 0,
			},
			wantErr: true,
		},
		{
			name: "Tolerance Check",
			config: WeightConfig{
				Technical: 0.333,
				Legal:     0.333,
				Market:    0.333,
				Strategic: 0.001, // Sum = 1.0
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluatePatent(t *testing.T) {
	config := DefaultWeightConfig()
	details := map[string]float64{
		"technical": 90.0,
		"legal":     80.0,
		"market":    70.0,
		"strategic": 60.0,
	}
	// Expected: (90*0.3) + (80*0.3) + (70*0.2) + (60*0.2) = 27 + 24 + 14 + 12 = 77

	valuation, err := EvaluatePatent("patent-1", details, config)
	assert.NoError(t, err)
	assert.NotNil(t, valuation)
	assert.Equal(t, "patent-1", valuation.PatentID)
	assert.InDelta(t, 77.0, valuation.CompositeScore, 0.001)
	assert.Equal(t, ValuationTierB, valuation.Tier)
}

func TestEvaluatePortfolio(t *testing.T) {
	val1 := &Valuation{CompositeScore: 80.0}
	val2 := &Valuation{CompositeScore: 60.0}
	mid := int64(1000)
	val1.MonetaryValueMid = &mid
	val2.MonetaryValueMid = &mid

	summary, err := EvaluatePortfolio("portfolio-1", []*Valuation{val1, val2})
	assert.NoError(t, err)
	assert.Equal(t, 2, summary.TotalPatents)
	assert.Equal(t, 70.0, summary.AverageScore)
	assert.Equal(t, int64(2000), summary.TotalValuation)
}

func TestEvaluatePortfolio_Empty(t *testing.T) {
	summary, err := EvaluatePortfolio("portfolio-1", []*Valuation{})
	assert.NoError(t, err)
	assert.Equal(t, 0, summary.TotalPatents)
	assert.Equal(t, 0.0, summary.AverageScore)
}

//Personal.AI order the ending
