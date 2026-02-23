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

//Personal.AI order the ending
