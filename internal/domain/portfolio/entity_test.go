package portfolio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPortfolio(t *testing.T) {
	// Success
	p, err := NewPortfolio("My Portfolio", "user123")
	assert.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "My Portfolio", p.Name)
	assert.Equal(t, "user123", p.OwnerID)
	assert.Equal(t, PortfolioStatusDraft, p.Status)
	assert.WithinDuration(t, time.Now(), p.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), p.UpdatedAt, time.Second)

	// Empty name
	_, err = NewPortfolio("", "user123")
	assert.Error(t, err)

	// Name too long
	longName := make([]byte, 257)
	for i := range longName {
		longName[i] = 'a'
	}
	_, err = NewPortfolio(string(longName), "user123")
	assert.Error(t, err)

	// Empty owner ID
	_, err = NewPortfolio("My Portfolio", "")
	assert.Error(t, err)
}

func TestPortfolio_Validate(t *testing.T) {
	p := &Portfolio{
		ID:        "id1",
		Name:      "name1",
		OwnerID:   "owner1",
		Status:    PortfolioStatusActive,
		PatentIDs: []string{"p1", "p2"},
	}
	assert.NoError(t, p.Validate())

	// Empty ID
	p.ID = ""
	assert.Error(t, p.Validate())
	p.ID = "id1"

	// Invalid status
	p.Status = "invalid"
	assert.Error(t, p.Validate())
	p.Status = PortfolioStatusActive

	// Duplicate patent IDs
	p.PatentIDs = []string{"p1", "p1"}
	assert.Error(t, p.Validate())
}

func TestPortfolio_AddPatent(t *testing.T) {
	p, _ := NewPortfolio("test", "user")
	err := p.AddPatent("p1")
	assert.NoError(t, err)
	assert.Equal(t, 1, p.PatentCount())
	assert.True(t, p.ContainsPatent("p1"))

	// Add duplicate
	err = p.AddPatent("p1")
	assert.Error(t, err)
	assert.Equal(t, 1, p.PatentCount())

	// Add another
	err = p.AddPatent("p2")
	assert.NoError(t, err)
	assert.Equal(t, 2, p.PatentCount())
}

func TestPortfolio_RemovePatent(t *testing.T) {
	p, _ := NewPortfolio("test", "user")
	p.AddPatent("p1")
	p.AddPatent("p2")

	err := p.RemovePatent("p1")
	assert.NoError(t, err)
	assert.Equal(t, 1, p.PatentCount())
	assert.False(t, p.ContainsPatent("p1"))
	assert.True(t, p.ContainsPatent("p2"))

	// Remove non-existent
	err = p.RemovePatent("p3")
	assert.Error(t, err)
}

func TestPortfolio_StatusTransitions(t *testing.T) {
	p, _ := NewPortfolio("test", "user")

	// Activate from Draft
	err := p.Activate()
	assert.NoError(t, err)
	assert.Equal(t, PortfolioStatusActive, p.Status)

	// Activate from Active (should fail as it expects Draft)
	err = p.Activate()
	assert.Error(t, err)

	// Archive from Active
	err = p.Archive()
	assert.NoError(t, err)
	assert.Equal(t, PortfolioStatusArchived, p.Status)

	// Activate from Archived (should fail)
	err = p.Activate()
	assert.Error(t, err)
}

func TestPortfolio_HealthScore(t *testing.T) {
	p, _ := NewPortfolio("test", "user")
	hs := HealthScore{
		CoverageScore:      80,
		ConcentrationScore: 70,
		AgingScore:         60,
		QualityScore:       90,
		OverallScore:       75,
	}

	err := p.SetHealthScore(hs)
	assert.NoError(t, err)
	assert.NotNil(t, p.HealthScore)
	assert.Equal(t, 75.0, p.HealthScore.OverallScore)

	// Invalid score
	hsInvalid := hs
	hsInvalid.CoverageScore = 101
	err = p.SetHealthScore(hsInvalid)
	assert.Error(t, err)
}

func TestPortfolio_ToSummary(t *testing.T) {
	p, _ := NewPortfolio("test", "user")
	p.AddPatent("p1")
	p.SetHealthScore(HealthScore{OverallScore: 85})

	summary := p.ToSummary()
	assert.Equal(t, p.ID, summary.ID)
	assert.Equal(t, p.Name, summary.Name)
	assert.Equal(t, 1, summary.PatentCount)
	assert.Equal(t, 85.0, summary.OverallHealthScore)
}

//Personal.AI order the ending
