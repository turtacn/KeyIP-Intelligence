package portfolio

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
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

func (m *MockPortfolioRepository) GetByID(ctx context.Context, id uuid.UUID) (*Portfolio, error) {
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

func (m *MockPortfolioRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPortfolioRepository) List(ctx context.Context, ownerID uuid.UUID, status *Status, limit, offset int) ([]*Portfolio, int64, error) {
	args := m.Called(ctx, ownerID, status, limit, offset)
	return args.Get(0).([]*Portfolio), args.Get(1).(int64), args.Error(2)
}

func (m *MockPortfolioRepository) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*Portfolio, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) AddPatent(ctx context.Context, portfolioID, patentID uuid.UUID, role string, addedBy uuid.UUID) error {
	args := m.Called(ctx, portfolioID, patentID, role, addedBy)
	return args.Error(0)
}

func (m *MockPortfolioRepository) RemovePatent(ctx context.Context, portfolioID, patentID uuid.UUID) error {
	args := m.Called(ctx, portfolioID, patentID)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetPatents(ctx context.Context, portfolioID uuid.UUID, role *string, limit, offset int) ([]*patent.Patent, int64, error) {
	args := m.Called(ctx, portfolioID, role, limit, offset)
	return args.Get(0).([]*patent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPortfolioRepository) IsPatentInPortfolio(ctx context.Context, portfolioID, patentID uuid.UUID) (bool, error) {
	args := m.Called(ctx, portfolioID, patentID)
	return args.Bool(0), args.Error(1)
}

func (m *MockPortfolioRepository) BatchAddPatents(ctx context.Context, portfolioID uuid.UUID, patentIDs []uuid.UUID, role string, addedBy uuid.UUID) error {
	args := m.Called(ctx, portfolioID, patentIDs, role, addedBy)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetPortfoliosByPatent(ctx context.Context, patentID uuid.UUID) ([]*Portfolio, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) CreateValuation(ctx context.Context, v *Valuation) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetLatestValuation(ctx context.Context, patentID uuid.UUID) (*Valuation, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Valuation), args.Error(1)
}

func (m *MockPortfolioRepository) GetValuationHistory(ctx context.Context, patentID uuid.UUID, limit int) ([]*Valuation, error) {
	args := m.Called(ctx, patentID, limit)
	return args.Get(0).([]*Valuation), args.Error(1)
}

func (m *MockPortfolioRepository) GetValuationsByPortfolio(ctx context.Context, portfolioID uuid.UUID) ([]*Valuation, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).([]*Valuation), args.Error(1)
}

func (m *MockPortfolioRepository) GetValuationDistribution(ctx context.Context, portfolioID uuid.UUID) (map[ValuationTier]int64, error) {
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

func (m *MockPortfolioRepository) GetLatestHealthScore(ctx context.Context, portfolioID uuid.UUID) (*HealthScore, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*HealthScore), args.Error(1)
}

func (m *MockPortfolioRepository) GetHealthScoreHistory(ctx context.Context, portfolioID uuid.UUID, limit int) ([]*HealthScore, error) {
	args := m.Called(ctx, portfolioID, limit)
	return args.Get(0).([]*HealthScore), args.Error(1)
}

func (m *MockPortfolioRepository) GetHealthScoreTrend(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) ([]*HealthScore, error) {
	args := m.Called(ctx, portfolioID, startDate, endDate)
	return args.Get(0).([]*HealthScore), args.Error(1)
}

func (m *MockPortfolioRepository) CreateSuggestion(ctx context.Context, s *OptimizationSuggestion) error {
	args := m.Called(ctx, s)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetSuggestions(ctx context.Context, portfolioID uuid.UUID, status *string, limit, offset int) ([]*OptimizationSuggestion, int64, error) {
	args := m.Called(ctx, portfolioID, status, limit, offset)
	return args.Get(0).([]*OptimizationSuggestion), args.Get(1).(int64), args.Error(2)
}

func (m *MockPortfolioRepository) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status string, resolvedBy uuid.UUID) error {
	args := m.Called(ctx, id, status, resolvedBy)
	return args.Error(0)
}

func (m *MockPortfolioRepository) GetPendingSuggestionCount(ctx context.Context, portfolioID uuid.UUID) (int64, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPortfolioRepository) GetPortfolioSummary(ctx context.Context, portfolioID uuid.UUID) (*Summary, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Summary), args.Error(1)
}

func (m *MockPortfolioRepository) GetJurisdictionCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockPortfolioRepository) GetTechDomainCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockPortfolioRepository) GetExpiryTimeline(ctx context.Context, portfolioID uuid.UUID) ([]*ExpiryTimelineEntry, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).([]*ExpiryTimelineEntry), args.Error(1)
}

func (m *MockPortfolioRepository) ComparePortfolios(ctx context.Context, portfolioIDs []uuid.UUID) ([]*ComparisonResult, error) {
	args := m.Called(ctx, portfolioIDs)
	return args.Get(0).([]*ComparisonResult), args.Error(1)
}

func (m *MockPortfolioRepository) WithTx(ctx context.Context, fn func(PortfolioRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

// Test cases for PortfolioService

func TestNewPortfolioService(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)
	assert.NotNil(t, service)
}

func TestCreatePortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	ownerID := uuid.New()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	result, err := service.CreatePortfolio(context.Background(), "Test Portfolio", ownerID.String(), []string{"OLED", "Emitter"})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Test Portfolio", result.Name)
	assert.Equal(t, ownerID, result.OwnerID)
	assert.Equal(t, StatusDraft, result.Status)
	assert.Equal(t, []string{"OLED", "Emitter"}, result.TechDomains)
	mockRepo.AssertExpectations(t)
}

func TestCreatePortfolio_InvalidOwnerID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	result, err := service.CreatePortfolio(context.Background(), "Test Portfolio", "invalid-uuid", []string{"OLED"})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid owner id")
}

func TestCreatePortfolio_RepoError(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	ownerID := uuid.New()
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(errors.New("db error"))

	result, err := service.CreatePortfolio(context.Background(), "Test Portfolio", ownerID.String(), []string{"OLED"})

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestAddPatentsToPortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	patentID1 := uuid.New()
	patentID2 := uuid.New()

	mockRepo.On("AddPatent", mock.Anything, portfolioID, patentID1, "core", uuid.Nil).Return(nil)
	mockRepo.On("AddPatent", mock.Anything, portfolioID, patentID2, "core", uuid.Nil).Return(nil)

	portfolio := &Portfolio{
		ID:        portfolioID,
		UpdatedAt: time.Now(),
	}
	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	err := service.AddPatentsToPortfolio(context.Background(), portfolioID.String(), []string{patentID1.String(), patentID2.String()})

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestAddPatentsToPortfolio_InvalidPortfolioID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	err := service.AddPatentsToPortfolio(context.Background(), "invalid-uuid", []string{uuid.New().String()})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid portfolio id")
}

func TestAddPatentsToPortfolio_SkipsInvalidPatentIDs(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	validPatentID := uuid.New()

	mockRepo.On("AddPatent", mock.Anything, portfolioID, validPatentID, "core", uuid.Nil).Return(nil)

	portfolio := &Portfolio{
		ID:        portfolioID,
		UpdatedAt: time.Now(),
	}
	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	// Mix of valid and invalid patent IDs - invalid ones should be skipped
	err := service.AddPatentsToPortfolio(context.Background(), portfolioID.String(), []string{"invalid-uuid", validPatentID.String()})

	assert.NoError(t, err)
}

func TestRemovePatentsFromPortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	patentID := uuid.New()

	mockRepo.On("RemovePatent", mock.Anything, portfolioID, patentID).Return(nil)

	err := service.RemovePatentsFromPortfolio(context.Background(), portfolioID.String(), []string{patentID.String()})

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestRemovePatentsFromPortfolio_InvalidPortfolioID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	err := service.RemovePatentsFromPortfolio(context.Background(), "invalid-uuid", []string{uuid.New().String()})

	assert.Error(t, err)
}

func TestActivatePortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	portfolio := &Portfolio{
		ID:     portfolioID,
		Status: StatusDraft,
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	err := service.ActivatePortfolio(context.Background(), portfolioID.String())

	assert.NoError(t, err)
	assert.Equal(t, StatusActive, portfolio.Status)
	mockRepo.AssertExpectations(t)
}

func TestActivatePortfolio_InvalidID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	err := service.ActivatePortfolio(context.Background(), "invalid-uuid")

	assert.Error(t, err)
}

func TestActivatePortfolio_NotFound(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(nil, errors.New("not found"))

	err := service.ActivatePortfolio(context.Background(), portfolioID.String())

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

func TestArchivePortfolio_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	portfolio := &Portfolio{
		ID:     portfolioID,
		Status: StatusActive,
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	err := service.ArchivePortfolio(context.Background(), portfolioID.String())

	assert.NoError(t, err)
	assert.Equal(t, StatusArchived, portfolio.Status)
	mockRepo.AssertExpectations(t)
}

func TestArchivePortfolio_InvalidID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	err := service.ArchivePortfolio(context.Background(), "invalid-uuid")

	assert.Error(t, err)
}

func TestCalculateHealthScore_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	portfolio := &Portfolio{
		ID:          portfolioID,
		PatentCount: 20,
		TechDomains: []string{"OLED", "Emitter", "Transport"},
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("CreateHealthScore", mock.Anything, mock.AnythingOfType("*portfolio.HealthScore")).Return(nil)

	result, err := service.CalculateHealthScore(context.Background(), portfolioID.String())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, portfolioID, result.PortfolioID)
	assert.Greater(t, result.OverallScore, 0.0)
	mockRepo.AssertExpectations(t)
}

func TestCalculateHealthScore_InvalidID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	result, err := service.CalculateHealthScore(context.Background(), "invalid-uuid")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCalculateHealthScore_ZeroPatents(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	portfolio := &Portfolio{
		ID:          portfolioID,
		PatentCount: 0,
		TechDomains: []string{},
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)
	mockRepo.On("CreateHealthScore", mock.Anything, mock.AnythingOfType("*portfolio.HealthScore")).Return(nil)

	result, err := service.CalculateHealthScore(context.Background(), portfolioID.String())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.CoverageScore)
	mockRepo.AssertExpectations(t)
}

func TestComparePortfolios_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID1 := uuid.New()
	portfolioID2 := uuid.New()

	portfolio1 := &Portfolio{
		ID:          portfolioID1,
		Name:        "Portfolio 1",
		PatentCount: 10,
		TechDomains: []string{"OLED", "Emitter"},
	}
	portfolio2 := &Portfolio{
		ID:          portfolioID2,
		Name:        "Portfolio 2",
		PatentCount: 20,
		TechDomains: []string{"Transport", "Host"},
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID1).Return(portfolio1, nil)
	mockRepo.On("GetByID", mock.Anything, portfolioID2).Return(portfolio2, nil)
	mockRepo.On("GetLatestHealthScore", mock.Anything, portfolioID1).Return(nil, errors.New("not found"))
	mockRepo.On("GetLatestHealthScore", mock.Anything, portfolioID2).Return(nil, errors.New("not found"))

	results, err := service.ComparePortfolios(context.Background(), []string{portfolioID1.String(), portfolioID2.String()})

	assert.NoError(t, err)
	assert.Len(t, results, 2)
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

func TestIdentifyGaps_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID := uuid.New()
	portfolio := &Portfolio{
		ID:          portfolioID,
		PatentCount: 6,
		TechDomains: []string{"OLED", "Emitter"},
	}

	mockRepo.On("GetByID", mock.Anything, portfolioID).Return(portfolio, nil)

	gaps, err := service.IdentifyGaps(context.Background(), portfolioID.String(), []string{"OLED", "Emitter", "Transport", "NewDomain"})

	assert.NoError(t, err)
	assert.NotNil(t, gaps)
	// Transport and NewDomain should be identified as gaps (not in portfolio tech domains)
	assert.GreaterOrEqual(t, len(gaps), 2)
	mockRepo.AssertExpectations(t)
}

func TestIdentifyGaps_InvalidID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	gaps, err := service.IdentifyGaps(context.Background(), "invalid-uuid", []string{"OLED"})

	assert.Error(t, err)
	assert.Nil(t, gaps)
}

func TestGetOverlapAnalysis_Success(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID1 := uuid.New()
	portfolioID2 := uuid.New()

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

	result, err := service.GetOverlapAnalysis(context.Background(), portfolioID1.String(), portfolioID2.String())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, portfolioID1, result.Portfolio1ID)
	assert.Equal(t, portfolioID2, result.Portfolio2ID)
	assert.Len(t, result.OverlappingPatentIDs, 1)
	assert.Contains(t, result.OverlappingPatentIDs, sharedPatentID.String())
	mockRepo.AssertExpectations(t)
}

func TestGetOverlapAnalysis_InvalidID(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	// Invalid first ID
	result, err := service.GetOverlapAnalysis(context.Background(), "invalid-uuid", uuid.New().String())
	assert.Error(t, err)
	assert.Nil(t, result)

	// Invalid second ID
	result, err = service.GetOverlapAnalysis(context.Background(), uuid.New().String(), "invalid-uuid")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetOverlapAnalysis_EmptyPortfolios(t *testing.T) {
	mockRepo := new(MockPortfolioRepository)
	service := NewPortfolioService(mockRepo)

	portfolioID1 := uuid.New()
	portfolioID2 := uuid.New()

	mockRepo.On("GetPatents", mock.Anything, portfolioID1, (*string)(nil), 10000, 0).Return([]*patent.Patent{}, int64(0), nil)
	mockRepo.On("GetPatents", mock.Anything, portfolioID2, (*string)(nil), 10000, 0).Return([]*patent.Patent{}, int64(0), nil)

	result, err := service.GetOverlapAnalysis(context.Background(), portfolioID1.String(), portfolioID2.String())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.OverlapRatio)
	assert.Len(t, result.OverlappingPatentIDs, 0)
	mockRepo.AssertExpectations(t)
}

// Test Status constants
func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("active"), StatusActive)
	assert.Equal(t, Status("archived"), StatusArchived)
	assert.Equal(t, Status("draft"), StatusDraft)
}

// Test ValuationTier constants
func TestValuationTier_Constants(t *testing.T) {
	assert.Equal(t, ValuationTier("S"), ValuationTierS)
	assert.Equal(t, ValuationTier("A"), ValuationTierA)
	assert.Equal(t, ValuationTier("B"), ValuationTierB)
	assert.Equal(t, ValuationTier("C"), ValuationTierC)
	assert.Equal(t, ValuationTier("D"), ValuationTierD)
}
