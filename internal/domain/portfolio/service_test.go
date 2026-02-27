package portfolio

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestCreatePortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	ownerID := uuid.New().String()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	result, err := service.CreatePortfolio(context.Background(), "Test Portfolio", ownerID, []string{"OLED", "Emitter"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test Portfolio", result.Name)
	assert.Equal(t, ownerID, result.OwnerID)
	assert.Equal(t, StatusDraft, result.Status)
	assert.Equal(t, []string{"OLED", "Emitter"}, result.TechDomains)
	mockRepo.AssertExpectations(t)
}

func TestCreatePortfolio_InvalidInput(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	// Missing Name
	result, err := service.CreatePortfolio(context.Background(), "", "owner-1", []string{"OLED"})
	assert.Error(t, err)
	assert.Nil(t, result)

	// Missing OwnerID
	result, err = service.CreatePortfolio(context.Background(), "Name", "", []string{"OLED"})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCreatePortfolio_RepoError(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	ownerID := uuid.New().String()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(errors.New(errors.ErrCodeDatabaseError, "db error"))

	result, err := service.CreatePortfolio(context.Background(), "Test Portfolio", ownerID, []string{"OLED"})

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestAddPatentsToPortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New().String()
	patentID1 := uuid.New().String()
	patentID2 := uuid.New().String()

	mockRepo.On("AddPatent", mock.Anything, portfolioID, patentID1, "core", "").Return(nil)
	mockRepo.On("AddPatent", mock.Anything, portfolioID, patentID2, "core", "").Return(nil)

	portfolio := &Portfolio{
		ID:        portfolioID,
		UpdatedAt: time.Now(),
	}
	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	err := service.AddPatentsToPortfolio(context.Background(), portfolioID, []string{patentID1, patentID2})

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestAddPatentsToPortfolio_InvalidPortfolioID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	err := service.AddPatentsToPortfolio(context.Background(), "", []string{"patent-1"})
	assert.Error(t, err)
}

func TestRemovePatentsFromPortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New().String()
	patentID := uuid.New().String()

	mockRepo.On("RemovePatent", mock.Anything, portfolioID, patentID).Return(nil)

	err := service.RemovePatentsFromPortfolio(context.Background(), portfolioID, []string{patentID})

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestActivatePortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New().String()
	portfolio := &Portfolio{
		ID:     portfolioID,
		Status: StatusDraft,
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	err := service.ActivatePortfolio(context.Background(), portfolioID)

	assert.NoError(t, err)
	assert.Equal(t, StatusActive, portfolio.Status)
	mockRepo.AssertExpectations(t)
}

func TestCalculateHealthScore_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New().String()
	portfolio := &Portfolio{
		ID:          portfolioID,
		PatentCount: 20,
		TechDomains: []string{"OLED", "Emitter", "Transport"},
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("CreateHealthScore", mock.Anything, mock.AnythingOfType("*portfolio.HealthScore")).Return(nil)

	result, err := service.CalculateHealthScore(context.Background(), portfolioID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, portfolioID, result.PortfolioID)
	assert.Greater(t, result.OverallScore, 0.0)
	mockRepo.AssertExpectations(t)
}

func TestComparePortfolios_TooManyPortfolios(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioIDs := make([]string, 11)
	for i := 0; i < 11; i++ {
		portfolioIDs[i] = uuid.New().String()
	}

	results, err := service.ComparePortfolios(context.Background(), portfolioIDs)

	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "cannot compare more than 10 portfolios")
}

func TestGetOverlapAnalysis_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID1 := uuid.New().String()
	portfolioID2 := uuid.New().String()

	sharedPatentID := uuid.New()
	uniquePatent1 := uuid.New()
	uniquePatent2 := uuid.New()

	patents1 := []*patent.Patent{
		{ID: sharedPatentID},
		{ID: uniquePatent1},
	}
	patents2 := []*patent.Patent{
		{ID: sharedPatentID},
		{ID: uniquePatent2},
	}

	mockRepo.On("GetPatents", mock.Anything, portfolioID1, (*string)(nil), 10000, 0).Return(patents1, int64(2), nil)
	mockRepo.On("GetPatents", mock.Anything, portfolioID2, (*string)(nil), 10000, 0).Return(patents2, int64(2), nil)

	result, err := service.GetOverlapAnalysis(context.Background(), portfolioID1, portfolioID2)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, portfolioID1, result.Portfolio1ID)
	assert.Equal(t, portfolioID2, result.Portfolio2ID)
	assert.Len(t, result.OverlappingPatentIDs, 1)
	assert.Contains(t, result.OverlappingPatentIDs, sharedPatentID.String())
	mockRepo.AssertExpectations(t)
}

//Personal.AI order the ending
