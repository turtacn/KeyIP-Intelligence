// Package portfolio_test provides unit tests for the portfolio domain service.
package portfolio_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock implementations
// ─────────────────────────────────────────────────────────────────────────────

type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Save(ctx context.Context, p *portfolio.Portfolio) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockRepository) FindByID(ctx context.Context, id common.ID) (*portfolio.Portfolio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.Portfolio), args.Error(1)
}

func (m *MockRepository) FindByOwner(ctx context.Context, ownerID common.UserID, page common.PageRequest) (*common.PageResponse[*portfolio.Portfolio], error) {
	args := m.Called(ctx, ownerID, page)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PageResponse[*portfolio.Portfolio]), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, p *portfolio.Portfolio) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockRepository) Delete(ctx context.Context, id common.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) FindByPatentID(ctx context.Context, patentID common.ID) ([]*portfolio.Portfolio, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*portfolio.Portfolio), args.Error(1)
}

func (m *MockRepository) FindByTag(ctx context.Context, tag string, page common.PageRequest) (*common.PageResponse[*portfolio.Portfolio], error) {
	args := m.Called(ctx, tag, page)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PageResponse[*portfolio.Portfolio]), args.Error(1)
}

type MockValuator struct {
	mock.Mock
}

func (m *MockValuator) Valuate(ctx context.Context, patentID common.ID, factors portfolio.ValuationFactors) (float64, error) {
	args := m.Called(ctx, patentID, factors)
	return args.Get(0).(float64), args.Error(1)
}

// ─────────────────────────────────────────────────────────────────────────────
// Service tests
// ─────────────────────────────────────────────────────────────────────────────

func TestService_CreatePortfolio_Success(t *testing.T) {
	t.Parallel()

	mockRepo := new(MockRepository)
	mockValuator := new(MockValuator)
	logger := logging.NewNopLogger()
	svc := portfolio.NewService(mockRepo, mockValuator, logger)

	mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).
		Return(nil)

	p, err := svc.CreatePortfolio(context.Background(), "Test Portfolio", "desc", "user-1")

	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, "Test Portfolio", p.Name)
	mockRepo.AssertExpectations(t)
}

func TestService_CreatePortfolio_EmptyName(t *testing.T) {
	t.Parallel()

	mockRepo := new(MockRepository)
	mockValuator := new(MockValuator)
	logger := logging.NewNopLogger()
	svc := portfolio.NewService(mockRepo, mockValuator, logger)

	p, err := svc.CreatePortfolio(context.Background(), "", "desc", "user-1")

	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestService_AddPatentToPortfolio_Success(t *testing.T) {
	t.Parallel()

	mockRepo := new(MockRepository)
	mockValuator := new(MockValuator)
	logger := logging.NewNopLogger()
	svc := portfolio.NewService(mockRepo, mockValuator, logger)

	existingPortfolio, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	mockRepo.On("FindByID", mock.Anything, existingPortfolio.ID).
		Return(existingPortfolio, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).
		Return(nil)

	patentID := common.NewID()
	err = svc.AddPatentToPortfolio(context.Background(), existingPortfolio.ID, patentID)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_AddPatentToPortfolio_PortfolioNotFound(t *testing.T) {
	t.Parallel()

	mockRepo := new(MockRepository)
	mockValuator := new(MockValuator)
	logger := logging.NewNopLogger()
	svc := portfolio.NewService(mockRepo, mockValuator, logger)

	nonExistentID := common.NewID()
	mockRepo.On("FindByID", mock.Anything, nonExistentID).
		Return(nil, assert.AnError)

	err := svc.AddPatentToPortfolio(context.Background(), nonExistentID, common.NewID())

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_ValuatePortfolio_Success(t *testing.T) {
	t.Parallel()

	mockRepo := new(MockRepository)
	mockValuator := new(MockValuator)
	logger := logging.NewNopLogger()
	svc := portfolio.NewService(mockRepo, mockValuator, logger)

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)
	patentID := common.NewID()
	require.NoError(t, p.AddPatent(patentID))

	mockRepo.On("FindByID", mock.Anything, p.ID).Return(p, nil)
	mockValuator.On("Valuate", mock.Anything, patentID, mock.AnythingOfType("portfolio.ValuationFactors")).
		Return(100000.0, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).
		Return(nil)

	factors := map[common.ID]portfolio.ValuationFactors{
		patentID: {
			TechnicalScore: 0.8,
			LegalScore:     0.7,
			MarketScore:    0.9,
			RemainingLife:  15.0,
		},
	}

	result, err := svc.ValuatePortfolio(context.Background(), p.ID, factors)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Greater(t, result.TotalValue, 0.0)
	mockRepo.AssertExpectations(t)
	mockValuator.AssertExpectations(t)
}

func TestService_ValuatePortfolio_FactorCountMismatch(t *testing.T) {
	t.Parallel()

	mockRepo := new(MockRepository)
	mockValuator := new(MockValuator)
	logger := logging.NewNopLogger()
	svc := portfolio.NewService(mockRepo, mockValuator, logger)

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)
	require.NoError(t, p.AddPatent(common.NewID()))
	require.NoError(t, p.AddPatent(common.NewID()))

	mockRepo.On("FindByID", mock.Anything, p.ID).Return(p, nil)

	// Provide factors for only one patent (mismatch).
	factors := map[common.ID]portfolio.ValuationFactors{
		common.NewID(): {
			TechnicalScore: 0.8,
			LegalScore:     0.7,
			MarketScore:    0.9,
			RemainingLife:  15.0,
		},
	}

	result, err := svc.ValuatePortfolio(context.Background(), p.ID, factors)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "factor count")
	mockRepo.AssertExpectations(t)
}

