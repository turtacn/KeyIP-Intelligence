package portfolio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks
type MockPortfolioRepository struct {
	mock.Mock
}

func (m *MockPortfolioRepository) Save(ctx context.Context, portfolio *Portfolio) error {
	args := m.Called(ctx, portfolio)
	return args.Error(0)
}

func (m *MockPortfolioRepository) FindByID(ctx context.Context, id string) (*Portfolio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) FindByOwnerID(ctx context.Context, ownerID string, opts ...QueryOption) ([]*Portfolio, error) {
	args := m.Called(ctx, ownerID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) FindByStatus(ctx context.Context, status PortfolioStatus, opts ...QueryOption) ([]*Portfolio, error) {
	args := m.Called(ctx, status, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) FindByTechDomain(ctx context.Context, techDomain string, opts ...QueryOption) ([]*Portfolio, error) {
	args := m.Called(ctx, techDomain, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPortfolioRepository) Count(ctx context.Context, ownerID string) (int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPortfolioRepository) ListSummaries(ctx context.Context, ownerID string, opts ...QueryOption) ([]*PortfolioSummary, error) {
	args := m.Called(ctx, ownerID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*PortfolioSummary), args.Error(1)
}

func (m *MockPortfolioRepository) FindContainingPatent(ctx context.Context, patentID string) ([]*Portfolio, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Portfolio), args.Error(1)
}

type MockDimensionEvaluator struct {
	mock.Mock
	dim ValuationDimension
}

func (m *MockDimensionEvaluator) Evaluate(ctx context.Context, patentID string) (*DimensionScore, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DimensionScore), args.Error(1)
}

func (m *MockDimensionEvaluator) Dimension() ValuationDimension {
	return m.dim
}

// Tests

func TestWeightConfig_Validate(t *testing.T) {
	wc := DefaultWeightConfig()
	assert.NoError(t, wc.Validate())

	wc.TechnicalWeight = 0.5
	wc.LegalWeight = 0.5
	wc.CommercialWeight = 0
	wc.StrategicWeight = 0
	assert.NoError(t, wc.Validate())

	// Sum != 1
	wc.StrategicWeight = 0.1
	assert.Error(t, wc.Validate())

	// Negative
	wc.TechnicalWeight = -0.1
	assert.Error(t, wc.Validate())
}

func TestEvaluatePatent_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	evalTech := &MockDimensionEvaluator{dim: DimensionTechnical}
	evalLegal := &MockDimensionEvaluator{dim: DimensionLegal}

	engine := NewValuationEngine(repo, evalTech, evalLegal)

	evalTech.On("Evaluate", mock.Anything, "p1").Return(&DimensionScore{
		Dimension: DimensionTechnical,
		Score:     80,
	}, nil)
	evalLegal.On("Evaluate", mock.Anything, "p1").Return(&DimensionScore{
		Dimension: DimensionLegal,
		Score:     90,
	}, nil)

	config := &WeightConfig{
		TechnicalWeight:  0.5,
		LegalWeight:      0.5,
		CommercialWeight: 0,
		StrategicWeight:  0,
	}

	pv, err := engine.EvaluatePatent(context.Background(), "p1", config)
	assert.NoError(t, err)
	assert.Equal(t, "p1", pv.PatentID)
	assert.Equal(t, 85.0, pv.OverallScore)
	assert.Equal(t, TierA, pv.Tier)
}

func TestEvaluatePortfolio_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	evalTech := &MockDimensionEvaluator{dim: DimensionTechnical}

	engine := NewValuationEngine(repo, evalTech)

	p := &Portfolio{
		ID:        "port1",
		PatentIDs: []string{"p1", "p2"},
	}

	repo.On("FindByID", mock.Anything, "port1").Return(p, nil)

	evalTech.On("Evaluate", mock.Anything, "p1").Return(&DimensionScore{Score: 90}, nil)
	evalTech.On("Evaluate", mock.Anything, "p2").Return(&DimensionScore{Score: 50}, nil)

	config := &WeightConfig{
		TechnicalWeight: 1.0,
	}

	pv, err := engine.EvaluatePortfolio(context.Background(), "port1", config)
	assert.NoError(t, err)
	assert.Equal(t, 70.0, pv.AggregateScore)
	assert.Equal(t, "p1", pv.TopPerformers[0])
	assert.Equal(t, "p2", pv.UnderPerformers[0])
}

func TestDetermineValuationTier(t *testing.T) {
	assert.Equal(t, TierS, DetermineValuationTier(95))
	assert.Equal(t, TierS, DetermineValuationTier(90))
	assert.Equal(t, TierA, DetermineValuationTier(89.9))
	assert.Equal(t, TierA, DetermineValuationTier(75))
	assert.Equal(t, TierB, DetermineValuationTier(60))
	assert.Equal(t, TierC, DetermineValuationTier(40))
	assert.Equal(t, TierD, DetermineValuationTier(39.9))
}

func TestGenerateRecommendations(t *testing.T) {
	scores := map[ValuationDimension]*DimensionScore{
		DimensionTechnical: {Dimension: DimensionTechnical, Score: 29}, // Low Tech (<30)
		DimensionLegal:     {Dimension: DimensionLegal, Score: 20},     // Low Legal
	}
	recs := GenerateRecommendations(scores)
	assert.True(t, containsRec(recs, "Technical Strengthening"))
	assert.True(t, containsRec(recs, "Legal Enforcement Review"))
	assert.True(t, containsRec(recs, "Abandonment Consideration")) // All low
}

func containsRec(recs []ValuationRecommendation, action string) bool {
	for _, r := range recs {
		if r.Action == action {
			return true
		}
	}
	return false
}

//Personal.AI order the ending
