// Package portfolio provides portfolio management application services.
package portfolio

import (
	"context"
	"time"
)

// Service provides the main portfolio management service interface.
// This interface is used by HTTP handlers for portfolio operations.
type Service interface {
	Create(ctx context.Context, input *CreateInput) (*Portfolio, error)
	GetByID(ctx context.Context, id string) (*Portfolio, error)
	List(ctx context.Context, input *ListInput) (*ListResult, error)
	Update(ctx context.Context, input *UpdateInput) (*Portfolio, error)
	Delete(ctx context.Context, id string, userID string) error
	AddPatents(ctx context.Context, id string, patentIDs []string, userID string) error
	RemovePatents(ctx context.Context, id string, patentIDs []string, userID string) error
	GetAnalysis(ctx context.Context, id string) (*PortfolioAnalysis, error)
}

// CreateInput contains input for creating a portfolio.
type CreateInput struct {
	Name        string
	Description string
	PatentIDs   []string
	Tags        []string
	UserID      string
}

// UpdateInput contains input for updating a portfolio.
type UpdateInput struct {
	ID          string
	Name        *string
	Description *string
	Tags        []string
	UserID      string
}

// ListInput contains input for listing portfolios.
type ListInput struct {
	Page     int
	PageSize int
	UserID   string
}

// Portfolio represents a portfolio DTO.
type Portfolio struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	PatentIDs   []string  `json:"patent_ids,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	PatentCount int       `json:"patent_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListResult represents a paginated list of portfolios.
type ListResult struct {
	Portfolios []*Portfolio `json:"portfolios"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
}

// PortfolioAnalysis represents portfolio analysis results.
type PortfolioAnalysis struct {
	PortfolioID     string         `json:"portfolio_id"`
	TotalPatents    int            `json:"total_patents"`
	ByJurisdiction  map[string]int `json:"by_jurisdiction"`
	ByStatus        map[string]int `json:"by_status"`
	ByYear          map[string]int `json:"by_year"`
	TopIPCCodes     []IPCCount     `json:"top_ipc_codes"`
	TotalValue      float64        `json:"total_value"`
	Recommendations []string       `json:"recommendations,omitempty"`
}

// IPCCount represents IPC code count.
type IPCCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// ValuationResult represents the result of portfolio valuation.
type ValuationResult struct {
	PortfolioID   string             `json:"portfolio_id"`
	TotalValue    float64            `json:"total_value"`
	PatentValues  []PatentValue      `json:"patent_values"`
	Methodology   string             `json:"methodology"`
	Confidence    float64            `json:"confidence"`
	CalculatedAt  time.Time          `json:"calculated_at"`
}

// PatentValue represents the value of a single patent.
type PatentValue struct {
	PatentID    string  `json:"patent_id"`
	PatentNo    string  `json:"patent_no"`
	Value       float64 `json:"value"`
	Factors     map[string]float64 `json:"factors"`
}

// PortfolioAssessRequest is the input for portfolio assessment.
type PortfolioAssessRequest struct {
	PortfolioID string `json:"portfolio_id"`
	Method      string `json:"method,omitempty"`
}

// PortfolioAssessResult is the output from portfolio assessment.
type PortfolioAssessResult struct {
	PortfolioID   string              `json:"portfolio_id"`
	TotalValue    float64             `json:"total_value"`
	RiskScore     float64             `json:"risk_score"`
	Items         []*ValuationItem    `json:"items"`
	Summary       string              `json:"summary"`
	CalculatedAt  time.Time           `json:"calculated_at"`
}

// ValuationItem represents a single patent valuation item.
type ValuationItem struct {
	PatentID       string             `json:"patent_id"`
	PatentNumber   string             `json:"patent_number"`
	Title          string             `json:"title"`
	Value          float64            `json:"value"`
	RiskScore      float64            `json:"risk_score"`
	LegalStatus    string             `json:"legal_status"`
	ExpirationDate time.Time          `json:"expiration_date"`
	Factors        map[string]float64 `json:"factors"`
}

//Personal.AI order the ending
