package portfolio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDimensionEvaluator struct {
	mock.Mock
}

func (m *mockDimensionEvaluator) Evaluate(ctx context.Context, patentID string) (*DimensionScore, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DimensionScore), args.Error(1)
}

func (m *mockDimensionEvaluator) Dimension() ValuationDimension {
	args := m.Called()
	return args.Get(0).(ValuationDimension)
}

func TestDefaultWeightConfig(t *testing.T) {
	wc := DefaultWeightConfig()
	assert.NoError(t, wc.Validate())
	assert.Equal(t, 0.20, wc.TechnicalWeight)
	assert.Equal(t, 0.25, wc.LegalWeight)
	assert.Equal(t, 0.30, wc.CommercialWeight)
	assert.Equal(t, 0.25, wc.StrategicWeight)
}

func TestWeightConfig_Validate_Success(t *testing.T) {
	wc := &WeightConfig{
		TechnicalWeight:  0.25,
		LegalWeight:      0.25,
		CommercialWeight: 0.25,
		StrategicWeight:  0.25,
	}
	assert.NoError(t, wc.Validate())
}

func TestWeightConfig_Validate_SumNotOne(t *testing.T) {
	wc := &WeightConfig{TechnicalWeight: 0.1, LegalWeight: 0.1, CommercialWeight: 0.1, StrategicWeight: 0.1}
	assert.Error(t, wc.Validate())
}

func TestWeightConfig_Validate_FloatingPointTolerance(t *testing.T) {
	wc := &WeightConfig{
		TechnicalWeight:  0.25,
		LegalWeight:      0.25,
		CommercialWeight: 0.25,
		StrategicWeight:  0.2500001,
	}
	assert.NoError(t, wc.Validate())
}

func TestDetermineValuationTier(t *testing.T) {
	tests := []struct {
		score float64
		want  ValuationTier
	}{
		{95, TierS},
		{90, TierS},
		{89.99, TierA},
		{80, TierA},
		{75, TierA},
		{74.99, TierB},
		{65, TierB},
		{60, TierB},
		{59.99, TierC},
		{50, TierC},
		{40, TierC},
		{39.99, TierD},
		{20, TierD},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, DetermineValuationTier(tt.score), "score: %v", tt.score)
	}
}

func TestGenerateRecommendations(t *testing.T) {
	t.Run("LowTechnical", func(t *testing.T) {
		scores := map[ValuationDimension]*DimensionScore{
			DimensionTechnical: {Dimension: DimensionTechnical, Score: 35},
		}
		recs := GenerateRecommendations(scores)
		assert.NotEmpty(t, recs)
		found := false
		for _, r := range recs {
			if r.Type == "strengthen" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("LowLegal", func(t *testing.T) {
		scores := map[ValuationDimension]*DimensionScore{
			DimensionLegal: {Dimension: DimensionLegal, Score: 25},
		}
		recs := GenerateRecommendations(scores)
		found := false
		for _, r := range recs {
			if r.Type == "enforce" && r.Priority == "critical" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("HighCommercialLowStrategic", func(t *testing.T) {
		scores := map[ValuationDimension]*DimensionScore{
			DimensionCommercial: {Dimension: DimensionCommercial, Score: 85},
			DimensionStrategic:  {Dimension: DimensionStrategic, Score: 40},
		}
		recs := GenerateRecommendations(scores)
		found := false
		for _, r := range recs {
			if r.Type == "maintain" && r.Reason == "High commercial value but weak strategic positioning." {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("AllHigh", func(t *testing.T) {
		scores := map[ValuationDimension]*DimensionScore{
			DimensionTechnical:  {Dimension: DimensionTechnical, Score: 80},
			DimensionLegal:      {Dimension: DimensionLegal, Score: 80},
			DimensionCommercial: {Dimension: DimensionCommercial, Score: 80},
			DimensionStrategic:  {Dimension: DimensionStrategic, Score: 80},
		}
		recs := GenerateRecommendations(scores)
		found := false
		for _, r := range recs {
			if r.Type == "maintain" && r.Priority == "low" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("AllLow", func(t *testing.T) {
		scores := map[ValuationDimension]*DimensionScore{
			DimensionTechnical:  {Dimension: DimensionTechnical, Score: 25},
			DimensionLegal:      {Dimension: DimensionLegal, Score: 25},
			DimensionCommercial: {Dimension: DimensionCommercial, Score: 25},
			DimensionStrategic:  {Dimension: DimensionStrategic, Score: 25},
		}
		recs := GenerateRecommendations(scores)
		found := false
		for _, r := range recs {
			if r.Type == "abandon" {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

func TestEvaluatePatent_Success(t *testing.T) {
	repo := new(mockPortfolioRepository)
	eval := new(mockDimensionEvaluator)
	eval.On("Dimension").Return(DimensionTechnical)
	eval.On("Evaluate", mock.Anything, "P1").Return(&DimensionScore{Score: 80}, nil)

	engine := NewValuationEngine(repo, eval)
	config := &WeightConfig{TechnicalWeight: 1.0}

	val, err := engine.EvaluatePatent(context.Background(), "P1", config)
	assert.NoError(t, err)
	assert.Equal(t, 80.0, val.OverallScore)
	assert.Equal(t, TierA, val.Tier)
}

func TestEvaluatePortfolio_Success(t *testing.T) {
	repo := new(mockPortfolioRepository)
	eval := new(mockDimensionEvaluator)
	eval.On("Dimension").Return(DimensionTechnical)
	eval.On("Evaluate", mock.Anything, "P1").Return(&DimensionScore{Score: 80}, nil)
	eval.On("Evaluate", mock.Anything, "P2").Return(&DimensionScore{Score: 60}, nil)

	p, _ := NewPortfolio("Test", "owner")
	p.AddPatent("P1")
	p.AddPatent("P2")
	repo.On("FindByID", mock.Anything, p.ID).Return(p, nil)

	engine := NewValuationEngine(repo, eval)
	config := &WeightConfig{TechnicalWeight: 1.0}

	val, err := engine.EvaluatePortfolio(context.Background(), p.ID, config)
	assert.NoError(t, err)
	assert.Equal(t, 70.0, val.AggregateScore)
	assert.Len(t, val.PatentValuations, 2)
	assert.Equal(t, "P1", val.TopPerformers[0])
	assert.Equal(t, "P2", val.UnderPerformers[0])
}

func TestDimensionScore_Validate(t *testing.T) {
	ds := &DimensionScore{Score: 105}
	assert.Error(t, ds.Validate())
	ds.Score = -5
	assert.Error(t, ds.Validate())
	ds.Score = 50
	assert.NoError(t, ds.Validate())
}

//Personal.AI order the ending
