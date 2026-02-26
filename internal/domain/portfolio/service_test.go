package portfolio

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestCreatePortfolio_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	repo.On("Save", mock.Anything, mock.AnythingOfType("*portfolio.Portfolio")).Return(nil)

	p, err := svc.CreatePortfolio(context.Background(), "My Portfolio", "owner1", []string{"OLED"})
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "My Portfolio", p.Name)
	assert.Equal(t, "owner1", p.OwnerID)
}

func TestCreatePortfolio_EmptyName(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	_, err := svc.CreatePortfolio(context.Background(), "", "owner1", nil)
	assert.Error(t, err)
	// Check that it is a validation error
	assert.True(t, apperrors.IsValidation(err))
}

func TestAddPatentsToPortfolio_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p := &Portfolio{ID: "p1", PatentIDs: []string{}}
	repo.On("FindByID", mock.Anything, "p1").Return(p, nil)
	repo.On("Save", mock.Anything, p).Return(nil)

	err := svc.AddPatentsToPortfolio(context.Background(), "p1", []string{"pat1", "pat2"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(p.PatentIDs))
}

func TestAddPatentsToPortfolio_PortfolioNotFound(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	repo.On("FindByID", mock.Anything, "p1").Return(nil, nil)

	err := svc.AddPatentsToPortfolio(context.Background(), "p1", []string{"pat1"})
	assert.Error(t, err)
	assert.True(t, apperrors.IsNotFound(err))
}

func TestRemovePatentsFromPortfolio_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p := &Portfolio{ID: "p1", PatentIDs: []string{"pat1", "pat2"}}
	repo.On("FindByID", mock.Anything, "p1").Return(p, nil)
	repo.On("Save", mock.Anything, p).Return(nil)

	err := svc.RemovePatentsFromPortfolio(context.Background(), "p1", []string{"pat1"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(p.PatentIDs))
	assert.Equal(t, "pat2", p.PatentIDs[0])
}

func TestActivatePortfolio_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p := &Portfolio{ID: "p1", Status: PortfolioStatusDraft}
	repo.On("FindByID", mock.Anything, "p1").Return(p, nil)
	repo.On("Save", mock.Anything, p).Return(nil)

	err := svc.ActivatePortfolio(context.Background(), "p1")
	assert.NoError(t, err)
	assert.Equal(t, PortfolioStatusActive, p.Status)
}

func TestCalculateHealthScore_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p := &Portfolio{ID: "p1", PatentIDs: []string{"pat1", "pat2"}}
	repo.On("FindByID", mock.Anything, "p1").Return(p, nil)
	repo.On("Save", mock.Anything, p).Return(nil)

	hs, err := svc.CalculateHealthScore(context.Background(), "p1")
	assert.NoError(t, err)
	assert.NotNil(t, hs)
	assert.NotZero(t, hs.OverallScore)
}

func TestComparePortfolios_Success(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p1 := &Portfolio{ID: "p1", Name: "P1", PatentIDs: []string{"pat1"}}
	p2 := &Portfolio{ID: "p2", Name: "P2", PatentIDs: []string{"pat2"}}

	repo.On("FindByID", mock.Anything, "p1").Return(p1, nil)
	repo.On("FindByID", mock.Anything, "p2").Return(p2, nil)

	comps, err := svc.ComparePortfolios(context.Background(), []string{"p1", "p2"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(comps))
	assert.Equal(t, "P1", comps[0].Name)
}

func TestIdentifyGaps_WithGaps(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p := &Portfolio{ID: "p1"}
	repo.On("FindByID", mock.Anything, "p1").Return(p, nil)

	gaps, err := svc.IdentifyGaps(context.Background(), "p1", []string{"OLED"})
	assert.NoError(t, err)
	assert.NotEmpty(t, gaps)
	assert.Equal(t, "OLED", gaps[0].TechDomain)
}

func TestGetOverlapAnalysis_WithOverlap(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p1 := &Portfolio{ID: "p1", PatentIDs: []string{"pat1", "pat2"}}
	p2 := &Portfolio{ID: "p2", PatentIDs: []string{"pat2", "pat3"}}

	repo.On("FindByID", mock.Anything, "p1").Return(p1, nil)
	repo.On("FindByID", mock.Anything, "p2").Return(p2, nil)

	res, err := svc.GetOverlapAnalysis(context.Background(), "p1", "p2")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.OverlappingPatentIDs))
	assert.Equal(t, "pat2", res.OverlappingPatentIDs[0])
	assert.Equal(t, 1, len(res.UniqueToPortfolio1))
	assert.Equal(t, 1, len(res.UniqueToPortfolio2))
}

func TestGetOverlapAnalysis_SamePortfolio(t *testing.T) {
	repo := new(MockPortfolioRepository)
	svc := NewPortfolioService(repo)

	p1 := &Portfolio{ID: "p1", PatentIDs: []string{"pat1"}}

	repo.On("FindByID", mock.Anything, "p1").Return(p1, nil)

	res, err := svc.GetOverlapAnalysis(context.Background(), "p1", "p1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.OverlappingPatentIDs))
	assert.Equal(t, 0, len(res.UniqueToPortfolio1))
	// Wait, len(overlapping) / totalUnique. Total unique is 1. Overlapping is 1. Ratio 1.0.
	assert.Equal(t, 1.0, res.OverlapRatio)
}

//Personal.AI order the ending
