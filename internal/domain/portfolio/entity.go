package portfolio

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// PortfolioStatus defines the lifecycle state of a portfolio.
type PortfolioStatus string

const (
	PortfolioStatusActive   PortfolioStatus = "Active"
	PortfolioStatusArchived PortfolioStatus = "Archived"
	PortfolioStatusDraft    PortfolioStatus = "Draft"
)

// HealthScore is a value object representing the quality snapshot of a portfolio.
type HealthScore struct {
	CoverageScore      float64   `json:"coverage_score"`
	ConcentrationScore float64   `json:"concentration_score"`
	AgingScore         float64   `json:"aging_score"`
	QualityScore       float64   `json:"quality_score"`
	OverallScore       float64   `json:"overall_score"`
	EvaluatedAt        time.Time `json:"evaluated_at"`
}

// Portfolio is an aggregate root representing a collection of patents.
type Portfolio struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	OwnerID     string            `json:"owner_id"`
	TechDomains []string          `json:"tech_domains"`
	PatentIDs   []string          `json:"patent_ids"`
	Tags        map[string]string `json:"tags"`
	Status      PortfolioStatus   `json:"status"`
	HealthScore *HealthScore      `json:"health_score"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PortfolioSummary provides a lightweight view of a portfolio.
type PortfolioSummary struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Status             PortfolioStatus `json:"status"`
	PatentCount        int             `json:"patent_count"`
	OverallHealthScore float64         `json:"overall_health_score"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// NewPortfolio constructs a new Portfolio in Draft status.
func NewPortfolio(name, ownerID string) (*Portfolio, error) {
	if name == "" {
		return nil, errors.InvalidParam("portfolio name cannot be empty")
	}
	if len(name) > 256 {
		return nil, errors.InvalidParam("portfolio name exceeds maximum length of 256 characters")
	}
	if ownerID == "" {
		return nil, errors.InvalidParam("owner ID cannot be empty")
	}

	now := time.Now().UTC()
	return &Portfolio{
		ID:          string(common.NewID()),
		Name:        name,
		OwnerID:     ownerID,
		TechDomains: make([]string, 0),
		PatentIDs:   make([]string, 0),
		Tags:        make(map[string]string),
		Status:      PortfolioStatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Validate ensures the portfolio entity is in a valid state.
func (p *Portfolio) Validate() error {
	if p.ID == "" {
		return errors.InvalidParam("portfolio ID cannot be empty")
	}
	if p.Name == "" || len(p.Name) > 256 {
		return errors.InvalidParam("invalid portfolio name")
	}
	if p.OwnerID == "" {
		return errors.InvalidParam("owner ID cannot be empty")
	}

	switch p.Status {
	case PortfolioStatusActive, PortfolioStatusArchived, PortfolioStatusDraft:
		// Valid
	default:
		return errors.InvalidParam(fmt.Sprintf("invalid portfolio status: %s", p.Status))
	}

	// Check for duplicate patent IDs
	ids := make(map[string]bool)
	for _, id := range p.PatentIDs {
		if ids[id] {
			return errors.InvalidParam(fmt.Sprintf("duplicate patent ID in portfolio: %s", id))
		}
		ids[id] = true
	}

	return nil
}

// AddPatent adds a patent to the portfolio.
func (p *Portfolio) AddPatent(patentID string) error {
	if patentID == "" {
		return errors.InvalidParam("patent ID cannot be empty")
	}

	if p.ContainsPatent(patentID) {
		return errors.Conflict(fmt.Sprintf("patent %s is already in portfolio %s", patentID, p.ID))
	}

	p.PatentIDs = append(p.PatentIDs, patentID)
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// RemovePatent removes a patent from the portfolio.
func (p *Portfolio) RemovePatent(patentID string) error {
	if patentID == "" {
		return errors.InvalidParam("patent ID cannot be empty")
	}

	idx := -1
	for i, id := range p.PatentIDs {
		if id == patentID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return errors.NotFound(fmt.Sprintf("patent %s not found in portfolio %s", patentID, p.ID))
	}

	p.PatentIDs = append(p.PatentIDs[:idx], p.PatentIDs[idx+1:]...)
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// ContainsPatent checks if a patent is in the portfolio.
func (p *Portfolio) ContainsPatent(patentID string) bool {
	for _, id := range p.PatentIDs {
		if id == patentID {
			return true
		}
	}
	return false
}

// PatentCount returns the number of patents in the portfolio.
func (p *Portfolio) PatentCount() int {
	return len(p.PatentIDs)
}

// Activate transitions the portfolio from Draft to Active.
func (p *Portfolio) Activate() error {
	if p.Status != PortfolioStatusDraft {
		return errors.InvalidState(fmt.Sprintf("cannot activate portfolio in %s status", p.Status))
	}
	p.Status = PortfolioStatusActive
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// Archive transitions the portfolio from Active to Archived.
func (p *Portfolio) Archive() error {
	if p.Status != PortfolioStatusActive {
		return errors.InvalidState(fmt.Sprintf("cannot archive portfolio in %s status", p.Status))
	}
	p.Status = PortfolioStatusArchived
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// SetHealthScore sets the health score for the portfolio.
func (p *Portfolio) SetHealthScore(score HealthScore) error {
	if err := score.Validate(); err != nil {
		return err
	}
	p.HealthScore = &score
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// ToSummary converts the portfolio to a summary view.
func (p *Portfolio) ToSummary() PortfolioSummary {
	var overallScore float64
	if p.HealthScore != nil {
		overallScore = p.HealthScore.OverallScore
	}

	return PortfolioSummary{
		ID:                 p.ID,
		Name:               p.Name,
		Status:             p.Status,
		PatentCount:        len(p.PatentIDs),
		OverallHealthScore: overallScore,
		UpdatedAt:          p.UpdatedAt,
	}
}

// Validate ensures the health score values are within range [0, 100].
func (hs HealthScore) Validate() error {
	scores := []struct {
		val  float64
		name string
	}{
		{hs.CoverageScore, "CoverageScore"},
		{hs.ConcentrationScore, "ConcentrationScore"},
		{hs.AgingScore, "AgingScore"},
		{hs.QualityScore, "QualityScore"},
		{hs.OverallScore, "OverallScore"},
	}

	for _, s := range scores {
		if s.val < 0 || s.val > 100 {
			return errors.InvalidParam(fmt.Sprintf("%s must be between 0 and 100", s.name))
		}
	}
	return nil
}

//Personal.AI order the ending
