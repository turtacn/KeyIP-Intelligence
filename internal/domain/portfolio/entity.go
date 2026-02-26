package portfolio

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PortfolioStatus defines the status of a portfolio.
type PortfolioStatus string

const (
	PortfolioStatusActive   PortfolioStatus = "active"
	PortfolioStatusArchived PortfolioStatus = "archived"
	PortfolioStatusDraft    PortfolioStatus = "draft"
)

// HealthScore represents the health snapshot of a portfolio.
type HealthScore struct {
	CoverageScore      float64   `json:"coverage_score"`
	ConcentrationScore float64   `json:"concentration_score"`
	AgingScore         float64   `json:"aging_score"`
	QualityScore       float64   `json:"quality_score"`
	OverallScore       float64   `json:"overall_score"`
	EvaluatedAt        time.Time `json:"evaluated_at"`
}

// Portfolio is the aggregate root for patent portfolio management.
type Portfolio struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	OwnerID     string            `json:"owner_id"`
	TechDomains []string          `json:"tech_domains"`
	PatentIDs   []string          `json:"patent_ids"`
	Tags                map[string]string `json:"tags"`
	Status              PortfolioStatus   `json:"status"`
	HealthScore         *HealthScore      `json:"health_score"`
	Metadata            map[string]interface{} `json:"metadata"` // Added for legacy compat
	TargetJurisdictions []string          `json:"target_jurisdictions"` // Added for legacy compat
	DeletedAt           *time.Time        `json:"deleted_at"` // Added for legacy compat
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// PortfolioSummary is a lightweight view of a portfolio for lists.
type PortfolioSummary struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Status             PortfolioStatus `json:"status"`
	PatentCount        int             `json:"patent_count"`
	OverallHealthScore float64         `json:"overall_health_score"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// NewPortfolio creates a new portfolio in Draft status.
func NewPortfolio(name, ownerID string) (*Portfolio, error) {
	if name == "" {
		return nil, errors.New("portfolio name cannot be empty")
	}
	if len(name) > 256 {
		return nil, errors.New("portfolio name cannot exceed 256 characters")
	}
	if ownerID == "" {
		return nil, errors.New("owner ID cannot be empty")
	}

	now := time.Now().UTC()
	return &Portfolio{
		ID:        uuid.New().String(),
		Name:      name,
		OwnerID:   ownerID,
		Status:    PortfolioStatusDraft,
		PatentIDs: []string{},
		Tags:      make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Validate checks the integrity of the portfolio entity.
func (p *Portfolio) Validate() error {
	if p.ID == "" {
		return errors.New("portfolio ID cannot be empty")
	}
	if p.Name == "" {
		return errors.New("portfolio name cannot be empty")
	}
	if len(p.Name) > 256 {
		return errors.New("portfolio name cannot exceed 256 characters")
	}
	if p.OwnerID == "" {
		return errors.New("owner ID cannot be empty")
	}
	switch p.Status {
	case PortfolioStatusActive, PortfolioStatusArchived, PortfolioStatusDraft:
		// valid
	default:
		return fmt.Errorf("invalid portfolio status: %s", p.Status)
	}

	// Check for duplicate patent IDs
	seen := make(map[string]bool)
	for _, id := range p.PatentIDs {
		if seen[id] {
			return fmt.Errorf("duplicate patent ID found: %s", id)
		}
		seen[id] = true
	}

	return nil
}

// AddPatent adds a patent to the portfolio.
func (p *Portfolio) AddPatent(patentID string) error {
	if patentID == "" {
		return errors.New("patent ID cannot be empty")
	}
	if p.ContainsPatent(patentID) {
		return fmt.Errorf("patent already exists in portfolio: %s", patentID)
	}
	p.PatentIDs = append(p.PatentIDs, patentID)
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// RemovePatent removes a patent from the portfolio.
func (p *Portfolio) RemovePatent(patentID string) error {
	for i, id := range p.PatentIDs {
		if id == patentID {
			p.PatentIDs = append(p.PatentIDs[:i], p.PatentIDs[i+1:]...)
			p.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return apperrors.NewNotFound("patent not found in portfolio: %s", patentID)
}

// ContainsPatent checks if a patent exists in the portfolio.
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
		return fmt.Errorf("cannot activate portfolio from status: %s", p.Status)
	}
	p.Status = PortfolioStatusActive
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// Archive transitions the portfolio from Active to Archived.
func (p *Portfolio) Archive() error {
	if p.Status != PortfolioStatusActive {
		return fmt.Errorf("cannot archive portfolio from status: %s", p.Status)
	}
	p.Status = PortfolioStatusArchived
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// SetHealthScore updates the health score snapshot.
func (p *Portfolio) SetHealthScore(score HealthScore) error {
	if err := score.Validate(); err != nil {
		return err
	}
	p.HealthScore = &score
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// ToSummary creates a summary view of the portfolio.
func (p *Portfolio) ToSummary() PortfolioSummary {
	overallScore := 0.0
	if p.HealthScore != nil {
		overallScore = p.HealthScore.OverallScore
	}
	return PortfolioSummary{
		ID:                 p.ID,
		Name:               p.Name,
		Status:             p.Status,
		PatentCount:        p.PatentCount(),
		OverallHealthScore: overallScore,
		UpdatedAt:          p.UpdatedAt,
	}
}

// Validate checks if the health score values are within range [0, 100].
func (hs HealthScore) Validate() error {
	if hs.CoverageScore < 0 || hs.CoverageScore > 100 {
		return fmt.Errorf("coverage score out of range [0, 100]: %f", hs.CoverageScore)
	}
	if hs.ConcentrationScore < 0 || hs.ConcentrationScore > 100 {
		return fmt.Errorf("concentration score out of range [0, 100]: %f", hs.ConcentrationScore)
	}
	if hs.AgingScore < 0 || hs.AgingScore > 100 {
		return fmt.Errorf("aging score out of range [0, 100]: %f", hs.AgingScore)
	}
	if hs.QualityScore < 0 || hs.QualityScore > 100 {
		return fmt.Errorf("quality score out of range [0, 100]: %f", hs.QualityScore)
	}
	if hs.OverallScore < 0 || hs.OverallScore > 100 {
		return fmt.Errorf("overall score out of range [0, 100]: %f", hs.OverallScore)
	}
	return nil
}

//Personal.AI order the ending
