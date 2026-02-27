package portfolio

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

// MockPortfolioRepository is a mock implementation of PortfolioRepository
type MockPortfolioRepository struct {
	mock.Mock
}

func (m *MockPortfolioRepository) Create(ctx context.Context, p *Portfolio) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetByID(ctx context.Context, id string) (*Portfolio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) Update(ctx context.Context, p *Portfolio) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPortfolioRepository) SoftDelete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPortfolioRepository) List(ctx context.Context, ownerID string, opts ...PortfolioQueryOption) ([]*Portfolio, int64, error) {
	// Apply options to make them comparable or just ignore them in mock matching if complex
	// For simplicity, we pass them through or use mock.Anything
	args := m.Called(ctx, ownerID, opts)
	return args.Get(0).([]*Portfolio), args.Get(1).(int64), args.Error(2)
}

func (m *MockPortfolioRepository) GetByOwner(ctx context.Context, ownerID string) ([]*Portfolio, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) AddPatent(ctx context.Context, portfolioID, patentID string, role string, addedBy string) error {
	args := m.Called(ctx, portfolioID, patentID, role, addedBy)
	return args.Error(0)
}

func (m *MockPortfolioRepository) RemovePatent(ctx context.Context, portfolioID, patentID string) error {
	args := m.Called(ctx, portfolioID, patentID)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetPatents(ctx context.Context, portfolioID string, role *string, limit, offset int) ([]*patent.Patent, int64, error) {
	args := m.Called(ctx, portfolioID, role, limit, offset)
	return args.Get(0).([]*patent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPortfolioRepository) IsPatentInPortfolio(ctx context.Context, portfolioID, patentID string) (bool, error) {
	args := m.Called(ctx, portfolioID, patentID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPortfolioRepository) BatchAddPatents(ctx context.Context, portfolioID string, patentIDs []string, role string, addedBy string) error {
	args := m.Called(ctx, portfolioID, patentIDs, role, addedBy)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetPortfoliosByPatent(ctx context.Context, patentID string) ([]*Portfolio, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) CreateValuation(ctx context.Context, v *Valuation) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetLatestValuation(ctx context.Context, patentID string) (*Valuation, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Valuation), args.Error(1)
}

func (m *MockPortfolioRepository) GetValuationHistory(ctx context.Context, patentID string, limit int) ([]*Valuation, error) {
	args := m.Called(ctx, patentID, limit)
	return args.Get(0).([]*Valuation), args.Error(1)
}

func (m *MockPortfolioRepository) GetValuationsByPortfolio(ctx context.Context, portfolioID string) ([]*Valuation, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).([]*Valuation), args.Error(1)
}

func (m *MockPortfolioRepository) GetValuationDistribution(ctx context.Context, portfolioID string) (map[ValuationTier]int64, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).(map[ValuationTier]int64), args.Error(1)
}

func (m *MockPortfolioRepository) BatchCreateValuations(ctx context.Context, valuations []*Valuation) error {
	args := m.Called(ctx, valuations)
	return args.Error(0)
}

func (m *MockPortfolioRepository) CreateHealthScore(ctx context.Context, score *HealthScore) error {
	args := m.Called(ctx, score)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetLatestHealthScore(ctx context.Context, portfolioID string) (*HealthScore, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*HealthScore), args.Error(1)
}

func (m *MockPortfolioRepository) GetHealthScoreHistory(ctx context.Context, portfolioID string, limit int) ([]*HealthScore, error) {
	args := m.Called(ctx, portfolioID, limit)
	return args.Get(0).([]*HealthScore), args.Error(1)
}

func (m *MockPortfolioRepository) GetHealthScoreTrend(ctx context.Context, portfolioID string, startDate, endDate time.Time) ([]*HealthScore, error) {
	args := m.Called(ctx, portfolioID, startDate, endDate)
	return args.Get(0).([]*HealthScore), args.Error(1)
}

func (m *MockPortfolioRepository) CreateSuggestion(ctx context.Context, s *OptimizationSuggestion) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetSuggestions(ctx context.Context, portfolioID string, status *string, limit, offset int) ([]*OptimizationSuggestion, int64, error) {
	args := m.Called(ctx, portfolioID, status, limit, offset)
	return args.Get(0).([]*OptimizationSuggestion), args.Get(1).(int64), args.Error(2)
}

func (m *MockPortfolioRepository) UpdateSuggestionStatus(ctx context.Context, id string, status string, resolvedBy string) error {
	args := m.Called(ctx, id, status, resolvedBy)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetPendingSuggestionCount(ctx context.Context, portfolioID string) (int64, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPortfolioRepository) GetPortfolioSummary(ctx context.Context, portfolioID string) (*Summary, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Summary), args.Error(1)
}

func (m *MockPortfolioRepository) GetJurisdictionCoverage(ctx context.Context, portfolioID string) (map[string]int64, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockPortfolioRepository) GetTechDomainCoverage(ctx context.Context, portfolioID string) (map[string]int64, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockPortfolioRepository) GetExpiryTimeline(ctx context.Context, portfolioID string) ([]*ExpiryTimelineEntry, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).([]*ExpiryTimelineEntry), args.Error(1)
}

func (m *MockPortfolioRepository) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*ComparisonResult, error) {
	args := m.Called(ctx, portfolioIDs)
	return args.Get(0).([]*ComparisonResult), args.Error(1)
}

func (m *MockPortfolioRepository) WithTx(ctx context.Context, fn func(PortfolioRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func TestApplyPortfolioOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []PortfolioQueryOption
		expected PortfolioQueryOptions
	}{
		{
			name:     "Default Options",
			opts:     nil,
			expected: PortfolioQueryOptions{Limit: 20, Offset: 0},
		},
		{
			name: "Custom Limit",
			opts: []PortfolioQueryOption{WithLimit(50)},
			expected: PortfolioQueryOptions{Limit: 50, Offset: 0},
		},
		{
			name: "Max Limit Enforced",
			opts: []PortfolioQueryOption{WithLimit(150)},
			expected: PortfolioQueryOptions{Limit: 100, Offset: 0},
		},
		{
			name: "Min Limit Enforced",
			opts: []PortfolioQueryOption{WithLimit(0)},
			expected: PortfolioQueryOptions{Limit: 20, Offset: 0},
		},
		{
			name: "Custom Offset",
			opts: []PortfolioQueryOption{WithOffset(10)},
			expected: PortfolioQueryOptions{Limit: 20, Offset: 10},
		},
		{
			name: "Negative Offset Corrected",
			opts: []PortfolioQueryOption{WithOffset(-1)},
			expected: PortfolioQueryOptions{Limit: 20, Offset: 0},
		},
		{
			name: "Status Filter",
			opts: []PortfolioQueryOption{WithStatus(StatusActive)},
			expected: func() PortfolioQueryOptions {
				s := StatusActive
				return PortfolioQueryOptions{Limit: 20, Offset: 0, Status: &s}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyPortfolioOptions(tt.opts...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

//Personal.AI order the ending
