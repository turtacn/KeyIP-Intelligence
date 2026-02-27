package lifecycle

import (
	"context"
	"time"

	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// LifecycleQueryOptions defines filtering and pagination for lifecycle queries.
type LifecycleQueryOptions struct {
	Limit  int
	Offset int
}

// LifecycleQueryOption defines a functional option for lifecycle queries.
type LifecycleQueryOption func(*LifecycleQueryOptions)

// WithLimit sets the limit for the query.
func WithLimit(limit int) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.Limit = limit
	}
}

// WithOffset sets the offset for the query.
func WithOffset(offset int) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.Offset = offset
	}
}

// ApplyLifecycleOptions applies the given options and returns the final configuration.
func ApplyLifecycleOptions(opts ...LifecycleQueryOption) LifecycleQueryOptions {
	options := LifecycleQueryOptions{
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

// LifecycleRepository defines the persistence contract for lifecycle domain.
type LifecycleRepository interface {
	// Annuity
	CreateAnnuity(ctx context.Context, annuity *Annuity) error
	GetAnnuity(ctx context.Context, id string) (*Annuity, error)
	GetAnnuitiesByPatent(ctx context.Context, patentID string) ([]*Annuity, error)
	GetUpcomingAnnuities(ctx context.Context, daysAhead int, limit, offset int) ([]*Annuity, int64, error)
	GetOverdueAnnuities(ctx context.Context, limit, offset int) ([]*Annuity, int64, error)
	UpdateAnnuityStatus(ctx context.Context, id string, status AnnuityStatus, paidAmount int64, paidDate *time.Time, paymentRef string) error
	BatchCreateAnnuities(ctx context.Context, annuities []*Annuity) error
	UpdateReminderSent(ctx context.Context, id string) error

	// Deadline
	CreateDeadline(ctx context.Context, deadline *Deadline) error
	GetDeadline(ctx context.Context, id string) (*Deadline, error)
	GetDeadlinesByPatent(ctx context.Context, patentID string, statusFilter []DeadlineStatus) ([]*Deadline, error)
	GetActiveDeadlines(ctx context.Context, userID *string, daysAhead int, limit, offset int) ([]*Deadline, int64, error)
	UpdateDeadlineStatus(ctx context.Context, id string, status DeadlineStatus, completedBy *string) error
	ExtendDeadline(ctx context.Context, id string, newDueDate time.Time, reason string) error
	GetCriticalDeadlines(ctx context.Context, limit int) ([]*Deadline, error)

	// Event
	CreateEvent(ctx context.Context, event *LifecycleEvent) error
	GetEventsByPatent(ctx context.Context, patentID string, eventTypes []EventType, limit, offset int) ([]*LifecycleEvent, int64, error)
	GetEventTimeline(ctx context.Context, patentID string) ([]*LifecycleEvent, error)
	GetRecentEvents(ctx context.Context, orgID string, limit int) ([]*LifecycleEvent, error)

	// Cost
	CreateCostRecord(ctx context.Context, record *CostRecord) error
	GetCostsByPatent(ctx context.Context, patentID string) ([]*CostRecord, error)
	GetCostSummary(ctx context.Context, patentID string) (*CostSummary, error)
	GetPortfolioCostSummary(ctx context.Context, portfolioID string, startDate, endDate time.Time) (*PortfolioCostSummary, error)

	// Dashboard
	GetLifecycleDashboard(ctx context.Context, orgID string) (*DashboardStats, error)

	// Payment (Added for AnnuityService)
	SavePayment(ctx context.Context, payment *PaymentRecord) (*PaymentRecord, error)
	QueryPayments(ctx context.Context, query *PaymentQuery) ([]PaymentRecord, int64, error)

	// Legal Status (Added for LegalStatusService)
	GetByPatentID(ctx context.Context, patentID string) (*LegalStatusEntity, error)
	UpdateStatus(ctx context.Context, patentID string, status string, effectiveDate time.Time) error
	SaveSubscription(ctx context.Context, sub *SubscriptionEntity) error
	DeactivateSubscription(ctx context.Context, id string) error
	GetStatusHistory(ctx context.Context, patentID string, pagination *commontypes.Pagination, from, to *time.Time) ([]*StatusHistoryEntity, error)

	// Custom Event (Added for CalendarService)
	SaveCustomEvent(ctx context.Context, event *CustomEvent) error
	GetCustomEvents(ctx context.Context, patentIDs []string, start, end time.Time) ([]CustomEvent, error)
	UpdateEventStatus(ctx context.Context, eventID string, status string) error
	DeleteEvent(ctx context.Context, eventID string) error

	// Transaction
	WithTx(ctx context.Context, fn func(LifecycleRepository) error) error
}

// Repository is an alias for LifecycleRepository for backward compatibility with apiserver.
type Repository = LifecycleRepository

//Personal.AI order the ending
