package portfolio

import (
	"time"

	"github.com/google/uuid"
)

// PortfolioStatus defines the lifecycle state of a portfolio.
type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
	StatusDraft    Status = "draft"
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
	ID                 uuid.UUID      `json:"id"`
	Name               string         `json:"name"`
	Description        string         `json:"description,omitempty"`
	OwnerID            uuid.UUID      `json:"owner_id"`
	Status             Status         `json:"status"`
	TechDomains        []string       `json:"tech_domains,omitempty"`
	TargetJurisdictions []string      `json:"target_jurisdictions,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	PatentCount        int            `json:"patent_count,omitempty"` // Aggregated field
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          *time.Time     `json:"deleted_at,omitempty"`
}

// Valuation represents a patent valuation.
type Valuation struct {
	ID                uuid.UUID      `json:"id"`
	PatentID          uuid.UUID      `json:"patent_id"`
	PortfolioID       *uuid.UUID     `json:"portfolio_id,omitempty"`
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
	EvaluatedBy       *uuid.UUID     `json:"evaluated_by,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// HealthScore represents the health of a portfolio.
type HealthScore struct {
	ID                       uuid.UUID      `json:"id"`
	PortfolioID              uuid.UUID      `json:"portfolio_id"`
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
	ID               uuid.UUID      `json:"id"`
	PortfolioID      uuid.UUID      `json:"portfolio_id"`
	HealthScoreID    *uuid.UUID     `json:"health_score_id,omitempty"`
	SuggestionType   string         `json:"suggestion_type"`
	Priority         string         `json:"priority"`
	Title            string         `json:"title"`
	Description      string         `json:"description"`
	TargetPatentID   *uuid.UUID     `json:"target_patent_id,omitempty"`
	TargetTechDomain string         `json:"target_tech_domain,omitempty"`
	TargetJurisdiction string       `json:"target_jurisdiction,omitempty"`
	EstimatedImpact  *float64       `json:"estimated_impact,omitempty"`
	EstimatedCost    *int64         `json:"estimated_cost,omitempty"`
	Rationale        map[string]any `json:"rationale"`
	Status           string         `json:"status"`
	ResolvedBy       *uuid.UUID     `json:"resolved_by,omitempty"`
	ResolvedAt       *time.Time     `json:"resolved_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
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
	PortfolioID uuid.UUID `json:"portfolio_id"`
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
}

//Personal.AI order the ending
