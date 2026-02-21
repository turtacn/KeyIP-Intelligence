package portfolio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// mockPortfolioRepository is a mock implementation of PortfolioRepository.
type mockPortfolioRepository struct {
	mock.Mock
}

func (m *mockPortfolioRepository) Save(ctx context.Context, portfolio *Portfolio) error {
	args := m.Called(ctx, portfolio)
	return args.Error(0)
}

func (m *mockPortfolioRepository) FindByID(ctx context.Context, id string) (*Portfolio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) FindByOwnerID(ctx context.Context, ownerID string, opts ...QueryOption) ([]*Portfolio, error) {
	args := m.Called(ctx, ownerID, opts)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) FindByStatus(ctx context.Context, status PortfolioStatus, opts ...QueryOption) ([]*Portfolio, error) {
	args := m.Called(ctx, status, opts)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) FindByTechDomain(ctx context.Context, techDomain string, opts ...QueryOption) ([]*Portfolio, error) {
	args := m.Called(ctx, techDomain, opts)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockPortfolioRepository) Count(ctx context.Context, ownerID string) (int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockPortfolioRepository) ListSummaries(ctx context.Context, ownerID string, opts ...QueryOption) ([]*PortfolioSummary, error) {
	args := m.Called(ctx, ownerID, opts)
	return args.Get(0).([]*PortfolioSummary), args.Error(1)
}

func (m *mockPortfolioRepository) FindContainingPatent(ctx context.Context, patentID string) ([]*Portfolio, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*Portfolio), args.Error(1)
}

func TestCreatePortfolio_Success(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	repo.On("Save", ctx, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	p, err := svc.CreatePortfolio(ctx, "Test Portfolio", "owner-1", []string{"OLED"})

	assert.NoError(t, err)
	assert.Equal(t, "Test Portfolio", p.Name)
	assert.Equal(t, []string{"OLED"}, p.TechDomains)
	repo.AssertExpectations(t)
}

func TestCreatePortfolio_EmptyName(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	_, err := svc.CreatePortfolio(ctx, "", "owner-1", nil)
	assert.Error(t, err)
	repo.AssertNotCalled(t, "Save", ctx, mock.Anything)
}

func TestAddPatentsToPortfolio_Success(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	p, _ := NewPortfolio("Test", "owner")
	repo.On("FindByID", ctx, p.ID).Return(p, nil)
	repo.On("Save", ctx, p).Return(nil)

	err := svc.AddPatentsToPortfolio(ctx, p.ID, []string{"P1", "P2"})
	assert.NoError(t, err)
	assert.Equal(t, 2, p.PatentCount())
}

func TestAddPatentsToPortfolio_PortfolioNotFound(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	repo.On("FindByID", ctx, "invalid").Return(nil, errors.New(errors.ErrCodeNotFound, "not found"))

	err := svc.AddPatentsToPortfolio(ctx, "invalid", []string{"P1"})
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeNotFound))
}

func TestAddPatentsToPortfolio_PartialDuplicate(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	p, _ := NewPortfolio("Test", "owner")
	p.AddPatent("P1")
	repo.On("FindByID", ctx, p.ID).Return(p, nil)
	repo.On("Save", ctx, p).Return(nil)

	err := svc.AddPatentsToPortfolio(ctx, p.ID, []string{"P1", "P2"})
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeConflict))
}

func TestActivatePortfolio_Success(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	p, _ := NewPortfolio("Test", "owner")
	repo.On("FindByID", ctx, p.ID).Return(p, nil)
	repo.On("Save", ctx, p).Return(nil)

	err := svc.ActivatePortfolio(ctx, p.ID)
	assert.NoError(t, err)
	assert.Equal(t, PortfolioStatusActive, p.Status)
}

func TestCalculateHealthScore_Success(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	p, _ := NewPortfolio("Test", "owner")
	p.TechDomains = []string{"D1", "D2"}
	for i := 0; i < 5; i++ {
		p.AddPatent(string(rune('A' + i)))
	}
	repo.On("FindByID", ctx, p.ID).Return(p, nil)
	repo.On("Save", ctx, p).Return(nil)

	score, err := svc.CalculateHealthScore(ctx, p.ID)
	assert.NoError(t, err)
	assert.Equal(t, 50.0, score.CoverageScore) // 5/10 * 100
	assert.Equal(t, 100.0, score.ConcentrationScore) // Round-robin uniform distribution assumed in score calc?
	// Wait, in my implementation of CalculateHealthScore, numDomains=2, patentCount=5.
	// It assumes uniform distribution for skeleton score.
	// H = -2 * (0.5 * log2(0.5)) = -2 * (0.5 * -1) = 1.
	// maxEntropy = log2(2) = 1.
	// concentrationScore = 1/1 * 100 = 100.
	assert.NotNil(t, p.HealthScore)
}

func TestComparePortfolios_Success(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	p1, _ := NewPortfolio("P1", "owner")
	p1.TechDomains = []string{"D1"}
	p1.AddPatent("Pat1")
	p2, _ := NewPortfolio("P2", "owner")
	p2.TechDomains = []string{"D1", "D2"}
	p2.AddPatent("Pat2")

	repo.On("FindByID", ctx, p1.ID).Return(p1, nil)
	repo.On("FindByID", ctx, p2.ID).Return(p2, nil)

	results, err := svc.ComparePortfolios(ctx, []string{p1.ID, p2.ID})
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, 1, results[0].PatentCount)
	assert.Equal(t, 1, results[1].PatentCount)
}

func TestIdentifyGaps_WithGaps(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	p, _ := NewPortfolio("Test", "owner")
	p.TechDomains = []string{"D1"} // Only D1
	repo.On("FindByID", ctx, p.ID).Return(p, nil)

	gaps, err := svc.IdentifyGaps(ctx, p.ID, []string{"D1", "D2"})
	assert.NoError(t, err)
	// D1 might have a gap if count < 10. D2 will definitely have a gap because it's not in p.TechDomains.
	// My implementation distributes patents across p.TechDomains.
	// D1 count will be p.PatentCount(). If p.PatentCount() is 0, D1 is a gap.
	assert.NotEmpty(t, gaps)
}

func TestGetOverlapAnalysis_WithOverlap(t *testing.T) {
	repo := new(mockPortfolioRepository)
	svc := NewPortfolioService(repo)
	ctx := context.Background()

	p1, _ := NewPortfolio("P1", "owner")
	p1.AddPatent("Shared")
	p1.AddPatent("Unique1")

	p2, _ := NewPortfolio("P2", "owner")
	p2.AddPatent("Shared")
	p2.AddPatent("Unique2")

	repo.On("FindByID", ctx, p1.ID).Return(p1, nil)
	repo.On("FindByID", ctx, p2.ID).Return(p2, nil)

	res, err := svc.GetOverlapAnalysis(ctx, p1.ID, p2.ID)
	assert.NoError(t, err)
	assert.Contains(t, res.OverlappingPatentIDs, "Shared")
	assert.Equal(t, 1, len(res.OverlappingPatentIDs))
	assert.Equal(t, 1/3.0, res.OverlapRatio) // Shared / (Unique1 + Unique2 + Shared) = 1 / 3
}

//Personal.AI order the ending
