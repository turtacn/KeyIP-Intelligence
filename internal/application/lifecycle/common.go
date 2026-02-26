package lifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

// Logger abstracts structured logging.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// CachePort abstracts cache get/set for this service.
// Shared by AnnuityService, CalendarService, LegalStatusService
type CachePort interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

// patentRepoPort is the minimal patent repository interface needed by lifecycle services.
type patentRepoPort interface {
	GetByID(ctx context.Context, id uuid.UUID) (*domainPatent.Patent, error)
	ListByPortfolio(ctx context.Context, portfolioID string) ([]*domainPatent.Patent, error)
}

// ---------------------------------------------------------------------------
// Additional DTO types for API handlers
// ---------------------------------------------------------------------------

// LifecycleOutput represents the output for lifecycle operations.
type LifecycleOutput struct {
	PatentID    string `json:"patent_id"`
	Phase       string `json:"phase"`
	Status      string `json:"status"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// MilestoneOutput represents the output for milestone operations.
type MilestoneOutput struct {
	ID          string `json:"id"`
	PatentID    string `json:"patent_id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// FeeOutput represents the output for fee operations.
type FeeOutput struct {
	ID       string  `json:"id"`
	PatentID string  `json:"patent_id"`
	FeeType  string  `json:"fee_type"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	DueDate  string  `json:"due_date,omitempty"`
	PaidAt   string  `json:"paid_at,omitempty"`
}

//Personal.AI order the ending
