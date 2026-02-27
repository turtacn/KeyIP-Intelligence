package portfolio

import (
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Status defines the lifecycle state of a portfolio.
type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

// ValuationTier defines the valuation grade.
type ValuationTier string

const (
	ValuationTierS ValuationTier = "S"
	ValuationTierA ValuationTier = "A"
	ValuationTierB ValuationTier = "B"
	ValuationTierC ValuationTier = "C"
	ValuationTierD ValuationTier = "D"
)

// Portfolio represents a collection of patents.
type Portfolio struct {
	ID                  string         `json:"id"`
	Name                string         `json:"name"`
	Description         string         `json:"description,omitempty"`
	OwnerID             string         `json:"owner_id"`
	Status              Status         `json:"status"`
	TechDomains         []string       `json:"tech_domains,omitempty"`
	TargetJurisdictions []string       `json:"target_jurisdictions,omitempty"`
	Metadata            map[string]any `json:"metadata,omitempty"`
	PatentCount         int            `json:"patent_count,omitempty"` // Aggregated field
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           *time.Time     `json:"deleted_at,omitempty"`
}

// NewPortfolio creates a new portfolio in Draft status.
func NewPortfolio(name, ownerID string, techDomains []string) (*Portfolio, error) {
	if name == "" {
		return nil, errors.NewValidation("name cannot be empty")
	}
	if ownerID == "" {
		return nil, errors.NewValidation("ownerID cannot be empty")
	}
	if len(techDomains) == 0 {
		techDomains = []string{}
	}

	now := time.Time(common.NewTimestamp())
	return &Portfolio{
		ID:          string(common.NewID()),
		Name:        name,
		OwnerID:     ownerID,
		Status:      StatusDraft,
		TechDomains: techDomains,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Validate validates the portfolio entity.
func (p *Portfolio) Validate() error {
	if p.ID == "" {
		return errors.NewValidation("ID cannot be empty")
	}
	if p.Name == "" {
		return errors.NewValidation("Name cannot be empty")
	}
	if len(p.Name) > 256 {
		return errors.NewValidation("Name cannot be longer than 256 characters")
	}
	if p.OwnerID == "" {
		return errors.NewValidation("OwnerID cannot be empty")
	}
	switch p.Status {
	case StatusDraft, StatusActive, StatusArchived:
		// valid
	default:
		return errors.NewValidation("invalid status: " + string(p.Status))
	}
	return nil
}

// Activate transitions the portfolio from Draft to Active.
func (p *Portfolio) Activate() error {
	if p.Status != StatusDraft {
		return errors.NewValidation("can only activate from draft status")
	}
	p.Status = StatusActive
	p.UpdatedAt = time.Time(common.NewTimestamp())
	return nil
}

// Archive transitions the portfolio from Active to Archived.
func (p *Portfolio) Archive() error {
	if p.Status != StatusActive {
		return errors.NewValidation("can only archive from active status")
	}
	p.Status = StatusArchived
	p.UpdatedAt = time.Time(common.NewTimestamp())
	return nil
}

// Valuation represents a patent valuation.
type Valuation struct {
	ID                string         `json:"id"`
	PatentID          string         `json:"patent_id"`
	PortfolioID       *string        `json:"portfolio_id,omitempty"`
	TechnicalScore    float64        `json:"technical_score"`
	LegalScore        float64        `json:"legal_score"`
	MarketScore       float64        `json:"market_score"`
	StrategicScore    float64        `json:"strategic_score"`
	CompositeScore    float64        `json:"composite_score"`
	Tier              ValuationTier  `json:"tier"`
	MonetaryValueLow  *int64         `json:"monetary_value_low,omitempty"`
	MonetaryValueMid  *int64         `json:"monetary_value_mid,omitempty"`
	MonetaryValueHigh *int64         `json:"monetary_value_high,omitempty"`
	Currency          string         `json:"currency"`
	ValuationMethod   string         `json:"valuation_method"`
	ModelVersion      string         `json:"model_version,omitempty"`
	ScoringDetails    map[string]any `json:"scoring_details"`
	ComparablePatents []string       `json:"comparable_patents,omitempty"` // IDs or Numbers
	Assumptions       map[string]any `json:"assumptions,omitempty"`
	ValidFrom         time.Time      `json:"valid_from"`
	ValidUntil        *time.Time     `json:"valid_until,omitempty"`
	EvaluatedBy       *string        `json:"evaluated_by,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// HealthScore represents the health of a portfolio.
type HealthScore struct {
	ID                       string         `json:"id"`
	PortfolioID              string         `json:"portfolio_id"`
	OverallScore             float64        `json:"overall_score"`
	CoverageScore            float64        `json:"coverage_score"`
	DiversityScore           float64        `json:"diversity_score"`
	FreshnessScore           float64        `json:"freshness_score"`
	StrengthScore            float64        `json:"strength_score"`
	RiskScore                float64        `json:"risk_score"`
	TotalPatents             int            `json:"total_patents"`
	ActivePatents            int            `json:"active_patents"`
	ExpiringWithinYear       int            `json:"expiring_within_year"`
	ExpiringWithin3Years     int            `json:"expiring_within_3years"`
	JurisdictionDistribution map[string]int `json:"jurisdiction_distribution"`
	TechDomainDistribution   map[string]int `json:"tech_domain_distribution"`
	TierDistribution         map[string]int `json:"tier_distribution"`
	Recommendations          []string       `json:"recommendations,omitempty"`
	ModelVersion             string         `json:"model_version,omitempty"`
	EvaluatedAt              time.Time      `json:"evaluated_at"`
	CreatedAt                time.Time      `json:"created_at"`
}

// OptimizationSuggestion represents a suggestion for portfolio optimization.
type OptimizationSuggestion struct {
	ID                 string         `json:"id"`
	PortfolioID        string         `json:"portfolio_id"`
	HealthScoreID      *string        `json:"health_score_id,omitempty"`
	SuggestionType     string         `json:"suggestion_type"`
	Priority           string         `json:"priority"`
	Title              string         `json:"title"`
	Description        string         `json:"description"`
	TargetPatentID     *string        `json:"target_patent_id,omitempty"`
	TargetTechDomain   string         `json:"target_tech_domain,omitempty"`
	TargetJurisdiction string         `json:"target_jurisdiction,omitempty"`
	EstimatedImpact    *float64       `json:"estimated_impact,omitempty"`
	EstimatedCost      *int64         `json:"estimated_cost,omitempty"`
	Rationale          map[string]any `json:"rationale"`
	Status             string         `json:"status"`
	ResolvedBy         *string        `json:"resolved_by,omitempty"`
	ResolvedAt         *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

// Summary represents a high-level summary of a portfolio.
type Summary struct {
	TotalPatents   int            `json:"total_patents"`
	ActivePatents  int            `json:"active_patents"`
	StatusCounts   map[string]int `json:"status_counts"`
	AverageScore   float64        `json:"average_score"`
	TotalValuation int64          `json:"total_valuation"`
	HealthScore    float64        `json:"health_score"`
}

// ExpiryTimelineEntry represents an entry in the expiry timeline.
type ExpiryTimelineEntry struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// ComparisonResult represents a comparison between portfolios.
type ComparisonResult struct {
	PortfolioID string  `json:"portfolio_id"`
	Metric      string  `json:"metric"`
	Value       float64 `json:"value"`
}

//Personal.AI order the ending
