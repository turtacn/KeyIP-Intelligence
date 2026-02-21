package portfolio

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestNewPortfolio_Success(t *testing.T) {
	name := "Blue OLED Materials"
	ownerID := "user-123"
	p, err := NewPortfolio(name, ownerID)

	assert.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, name, p.Name)
	assert.Equal(t, ownerID, p.OwnerID)
	assert.Equal(t, PortfolioStatusDraft, p.Status)
	assert.NotZero(t, p.CreatedAt)
	assert.NotZero(t, p.UpdatedAt)
}

func TestNewPortfolio_EmptyName(t *testing.T) {
	_, err := NewPortfolio("", "owner")
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeBadRequest))
}

func TestNewPortfolio_NameTooLong(t *testing.T) {
	name := strings.Repeat("a", 257)
	_, err := NewPortfolio(name, "owner")
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeBadRequest))
}

func TestNewPortfolio_EmptyOwnerID(t *testing.T) {
	_, err := NewPortfolio("Portfolio", "")
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeBadRequest))
}

func TestPortfolio_Validate_Success(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	err := p.Validate()
	assert.NoError(t, err)
}

func TestPortfolio_Validate_EmptyID(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.ID = ""
	err := p.Validate()
	assert.Error(t, err)
}

func TestPortfolio_Validate_InvalidStatus(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.Status = "Invalid"
	err := p.Validate()
	assert.Error(t, err)
}

func TestPortfolio_Validate_DuplicatePatentIDs(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.PatentIDs = []string{"P1", "P1"}
	err := p.Validate()
	assert.Error(t, err)
}

func TestPortfolio_AddPatent_Success(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	initialUpdate := p.UpdatedAt
	time.Sleep(1 * time.Millisecond)

	err := p.AddPatent("P1")
	assert.NoError(t, err)
	assert.Equal(t, 1, p.PatentCount())
	assert.True(t, p.ContainsPatent("P1"))
	assert.True(t, p.UpdatedAt.After(initialUpdate))
}

func TestPortfolio_AddPatent_EmptyID(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	err := p.AddPatent("")
	assert.Error(t, err)
}

func TestPortfolio_AddPatent_Duplicate(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.AddPatent("P1")
	err := p.AddPatent("P1")
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeConflict))
}

func TestPortfolio_RemovePatent_Success(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.AddPatent("P1")
	p.AddPatent("P2")
	initialUpdate := p.UpdatedAt
	time.Sleep(1 * time.Millisecond)

	err := p.RemovePatent("P1")
	assert.NoError(t, err)
	assert.Equal(t, 1, p.PatentCount())
	assert.False(t, p.ContainsPatent("P1"))
	assert.True(t, p.UpdatedAt.After(initialUpdate))
}

func TestPortfolio_RemovePatent_NotFound(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	err := p.RemovePatent("P1")
	assert.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.ErrCodeNotFound))
}

func TestPortfolio_ContainsPatent(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.AddPatent("P1")
	assert.True(t, p.ContainsPatent("P1"))
	assert.False(t, p.ContainsPatent("P2"))
}

func TestPortfolio_Activate_FromDraft(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	err := p.Activate()
	assert.NoError(t, err)
	assert.Equal(t, PortfolioStatusActive, p.Status)
}

func TestPortfolio_Activate_FromArchived(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.Activate()
	p.Archive()
	err := p.Activate()
	assert.Error(t, err)
}

func TestPortfolio_Activate_AlreadyActive(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.Activate()
	err := p.Activate()
	assert.Error(t, err)
}

func TestPortfolio_Archive_FromActive(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.Activate()
	err := p.Archive()
	assert.NoError(t, err)
	assert.Equal(t, PortfolioStatusArchived, p.Status)
}

func TestPortfolio_Archive_FromDraft(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	err := p.Archive()
	assert.Error(t, err)
}

func TestPortfolio_SetHealthScore_Valid(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	score := HealthScore{
		CoverageScore:      80,
		ConcentrationScore: 70,
		AgingScore:         60,
		QualityScore:       90,
		OverallScore:       75,
		EvaluatedAt:        time.Now().UTC(),
	}
	err := p.SetHealthScore(score)
	assert.NoError(t, err)
	assert.NotNil(t, p.HealthScore)
	assert.Equal(t, 75.0, p.HealthScore.OverallScore)
}

func TestPortfolio_SetHealthScore_OutOfRange(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	score := HealthScore{CoverageScore: 110}
	err := p.SetHealthScore(score)
	assert.Error(t, err)
}

func TestPortfolio_ToSummary(t *testing.T) {
	p, _ := NewPortfolio("Portfolio", "owner")
	p.AddPatent("P1")
	score := HealthScore{OverallScore: 85}
	p.SetHealthScore(score)

	summary := p.ToSummary()
	assert.Equal(t, p.ID, summary.ID)
	assert.Equal(t, p.Name, summary.Name)
	assert.Equal(t, p.Status, summary.Status)
	assert.Equal(t, 1, summary.PatentCount)
	assert.Equal(t, 85.0, summary.OverallHealthScore)
	assert.Equal(t, p.UpdatedAt, summary.UpdatedAt)
}

func TestHealthScore_Validate_AllValid(t *testing.T) {
	hs := HealthScore{
		CoverageScore:      100,
		ConcentrationScore: 0,
		AgingScore:         50,
		QualityScore:       75,
		OverallScore:       80,
	}
	assert.NoError(t, hs.Validate())
}

func TestHealthScore_Validate_NegativeScore(t *testing.T) {
	hs := HealthScore{CoverageScore: -1}
	assert.Error(t, hs.Validate())
}

func TestHealthScore_Validate_ExceedsMax(t *testing.T) {
	hs := HealthScore{OverallScore: 100.1}
	assert.Error(t, hs.Validate())
}

//Personal.AI order the ending
