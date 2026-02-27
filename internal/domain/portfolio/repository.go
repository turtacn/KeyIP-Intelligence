package portfolio

import (
	"context"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

// PortfolioQueryOptions defines filtering and pagination for portfolio queries.
type PortfolioQueryOptions struct {
	Limit  int
	Offset int
	Status *Status
}

// PortfolioQueryOption defines a functional option for portfolio queries.
type PortfolioQueryOption func(*PortfolioQueryOptions)

// WithLimit sets the limit for the query.
func WithLimit(limit int) PortfolioQueryOption {
	return func(o *PortfolioQueryOptions) {
		o.Limit = limit
	}
}

// WithOffset sets the offset for the query.
func WithOffset(offset int) PortfolioQueryOption {
	return func(o *PortfolioQueryOptions) {
		o.Offset = offset
	}
}

// WithStatus filters by portfolio status.
func WithStatus(status Status) PortfolioQueryOption {
	return func(o *PortfolioQueryOptions) {
		o.Status = &status
	}
}

// ApplyPortfolioOptions applies the given options and returns the final configuration.
// It enforces default limit (20) and max limit (100).
func ApplyPortfolioOptions(opts ...PortfolioQueryOption) PortfolioQueryOptions {
	options := PortfolioQueryOptions{
		Limit:  20,
		Offset: 0,
	}
	for _, opt := range opts {
		opt(&options)
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

// PortfolioRepository defines the persistence contract for portfolio domain.
type PortfolioRepository interface {
	// Portfolio
	Create(ctx context.Context, p *Portfolio) error
	GetByID(ctx context.Context, id string) (*Portfolio, error)
	Update(ctx context.Context, p *Portfolio) error
	SoftDelete(ctx context.Context, id string) error
	List(ctx context.Context, ownerID string, opts ...PortfolioQueryOption) ([]*Portfolio, int64, error)
	GetByOwner(ctx context.Context, ownerID string) ([]*Portfolio, error)

	// Patents
	AddPatent(ctx context.Context, portfolioID, patentID string, role string, addedBy string) error
	RemovePatent(ctx context.Context, portfolioID, patentID string) error
	GetPatents(ctx context.Context, portfolioID string, role *string, limit, offset int) ([]*patent.Patent, int64, error)
	IsPatentInPortfolio(ctx context.Context, portfolioID, patentID string) (bool, error)
	BatchAddPatents(ctx context.Context, portfolioID string, patentIDs []string, role string, addedBy string) error
	GetPortfoliosByPatent(ctx context.Context, patentID string) ([]*Portfolio, error)

	// Valuation
	CreateValuation(ctx context.Context, v *Valuation) error
	GetLatestValuation(ctx context.Context, patentID string) (*Valuation, error)
	GetValuationHistory(ctx context.Context, patentID string, limit int) ([]*Valuation, error)
	GetValuationsByPortfolio(ctx context.Context, portfolioID string) ([]*Valuation, error)
	GetValuationDistribution(ctx context.Context, portfolioID string) (map[ValuationTier]int64, error)
	BatchCreateValuations(ctx context.Context, valuations []*Valuation) error

	// HealthScore
	CreateHealthScore(ctx context.Context, score *HealthScore) error
	GetLatestHealthScore(ctx context.Context, portfolioID string) (*HealthScore, error)
	GetHealthScoreHistory(ctx context.Context, portfolioID string, limit int) ([]*HealthScore, error)
	GetHealthScoreTrend(ctx context.Context, portfolioID string, startDate, endDate time.Time) ([]*HealthScore, error)

	// Suggestions
	CreateSuggestion(ctx context.Context, s *OptimizationSuggestion) error
	GetSuggestions(ctx context.Context, portfolioID string, status *string, limit, offset int) ([]*OptimizationSuggestion, int64, error)
	UpdateSuggestionStatus(ctx context.Context, id string, status string, resolvedBy string) error
	GetPendingSuggestionCount(ctx context.Context, portfolioID string) (int64, error)

	// Analytics
	GetPortfolioSummary(ctx context.Context, portfolioID string) (*Summary, error)
	GetJurisdictionCoverage(ctx context.Context, portfolioID string) (map[string]int64, error)
	GetTechDomainCoverage(ctx context.Context, portfolioID string) (map[string]int64, error)
	GetExpiryTimeline(ctx context.Context, portfolioID string) ([]*ExpiryTimelineEntry, error)
	ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*ComparisonResult, error)

	// Transaction
	WithTx(ctx context.Context, fn func(PortfolioRepository) error) error
}

// Repository is an alias for PortfolioRepository for backward compatibility with apiserver.
type Repository = PortfolioRepository

//Personal.AI order the ending
