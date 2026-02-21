package portfolio

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

// PortfolioRepository defines the persistence contract for portfolio domain.
type PortfolioRepository interface {
	// Portfolio
	Create(ctx context.Context, p *Portfolio) error
	GetByID(ctx context.Context, id uuid.UUID) (*Portfolio, error)
	Update(ctx context.Context, p *Portfolio) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, ownerID uuid.UUID, status *Status, limit, offset int) ([]*Portfolio, int64, error)
	GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*Portfolio, error)

	// Patents
	AddPatent(ctx context.Context, portfolioID, patentID uuid.UUID, role string, addedBy uuid.UUID) error
	RemovePatent(ctx context.Context, portfolioID, patentID uuid.UUID) error
	GetPatents(ctx context.Context, portfolioID uuid.UUID, role *string, limit, offset int) ([]*patent.Patent, int64, error)
	IsPatentInPortfolio(ctx context.Context, portfolioID, patentID uuid.UUID) (bool, error)
	BatchAddPatents(ctx context.Context, portfolioID uuid.UUID, patentIDs []uuid.UUID, role string, addedBy uuid.UUID) error
	GetPortfoliosByPatent(ctx context.Context, patentID uuid.UUID) ([]*Portfolio, error)

	// Valuation
	CreateValuation(ctx context.Context, v *Valuation) error
	GetLatestValuation(ctx context.Context, patentID uuid.UUID) (*Valuation, error)
	GetValuationHistory(ctx context.Context, patentID uuid.UUID, limit int) ([]*Valuation, error)
	GetValuationsByPortfolio(ctx context.Context, portfolioID uuid.UUID) ([]*Valuation, error)
	GetValuationDistribution(ctx context.Context, portfolioID uuid.UUID) (map[ValuationTier]int64, error)
	BatchCreateValuations(ctx context.Context, valuations []*Valuation) error

	// HealthScore
	CreateHealthScore(ctx context.Context, score *HealthScore) error
	GetLatestHealthScore(ctx context.Context, portfolioID uuid.UUID) (*HealthScore, error)
	GetHealthScoreHistory(ctx context.Context, portfolioID uuid.UUID, limit int) ([]*HealthScore, error)
	GetHealthScoreTrend(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) ([]*HealthScore, error)

	// Suggestions
	CreateSuggestion(ctx context.Context, s *OptimizationSuggestion) error
	GetSuggestions(ctx context.Context, portfolioID uuid.UUID, status *string, limit, offset int) ([]*OptimizationSuggestion, int64, error)
	UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status string, resolvedBy uuid.UUID) error
	GetPendingSuggestionCount(ctx context.Context, portfolioID uuid.UUID) (int64, error)

	// Analytics
	GetPortfolioSummary(ctx context.Context, portfolioID uuid.UUID) (*Summary, error)
	GetJurisdictionCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error)
	GetTechDomainCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error)
	GetExpiryTimeline(ctx context.Context, portfolioID uuid.UUID) ([]*ExpiryTimelineEntry, error)
	ComparePortfolios(ctx context.Context, portfolioIDs []uuid.UUID) ([]*ComparisonResult, error)

	// Transaction
	WithTx(ctx context.Context, fn func(PortfolioRepository) error) error
}

//Personal.AI order the ending
