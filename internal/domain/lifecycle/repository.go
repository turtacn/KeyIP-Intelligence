package lifecycle

import (
	"context"
	"time"
)

// LifecycleQueryOption is a functional option for lifecycle queries.
type LifecycleQueryOption func(*LifecycleQueryOptions)

// LifecycleQueryOptions encapsulates query parameters.
type LifecycleQueryOptions struct {
	Offset        int
	Limit         int
	SortField     string
	SortAscending bool
	OwnerID       string
	FromDate      time.Time
	ToDate        time.Time
}

// LifecycleRepository defines persistence for LifecycleRecord.
type LifecycleRepository interface {
	SaveLifecycle(ctx context.Context, record *LifecycleRecord) error
	GetLifecycleByID(ctx context.Context, id string) (*LifecycleRecord, error)
	GetLifecycleByPatentID(ctx context.Context, patentID string) (*LifecycleRecord, error)
	GetLifecyclesByPhase(ctx context.Context, phase LifecyclePhase, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error)
	GetExpiringLifecycles(ctx context.Context, withinDays int) ([]*LifecycleRecord, error)
	GetLifecyclesByJurisdiction(ctx context.Context, jurisdictionCode string, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error)
	DeleteLifecycle(ctx context.Context, id string) error
	CountLifecycles(ctx context.Context, phase LifecyclePhase) (int64, error)

	// Legacy methods if needed by infra/app
	SavePayment(ctx context.Context, payment *PaymentRecord) (*PaymentRecord, error)
	QueryPayments(ctx context.Context, query *PaymentQuery) ([]PaymentRecord, int64, error)
}

// AnnuityRepository defines persistence for AnnuityRecord.
type AnnuityRepository interface {
	SaveAnnuity(ctx context.Context, record *AnnuityRecord) error
	SaveAnnuitiesBatch(ctx context.Context, records []*AnnuityRecord) error
	GetAnnuityByID(ctx context.Context, id string) (*AnnuityRecord, error)
	GetAnnuitiesByPatentID(ctx context.Context, patentID string) ([]*AnnuityRecord, error)
	GetAnnuitiesByStatus(ctx context.Context, status AnnuityStatus) ([]*AnnuityRecord, error)
	GetPendingAnnuities(ctx context.Context, beforeDate time.Time) ([]*AnnuityRecord, error)
	GetOverdueAnnuities(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error)
	SumAnnuitiesByPortfolio(ctx context.Context, portfolioID string, fromDate, toDate time.Time) (int64, string, error)
	DeleteAnnuity(ctx context.Context, id string) error
}

// DeadlineRepository defines persistence for Deadline.
type DeadlineRepository interface {
	SaveDeadline(ctx context.Context, deadline *Deadline) error
	GetDeadlineByID(ctx context.Context, id string) (*Deadline, error)
	GetDeadlinesByPatentID(ctx context.Context, patentID string) ([]*Deadline, error)
	GetDeadlinesByOwnerID(ctx context.Context, ownerID string, from, to time.Time) ([]*Deadline, error)
	GetOverdueDeadlines(ctx context.Context, ownerID string, asOf time.Time) ([]*Deadline, error)
	GetUpcomingDeadlines(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error)
	GetDeadlinesByType(ctx context.Context, deadlineType DeadlineType) ([]*Deadline, error)
	GetPendingDeadlineReminders(ctx context.Context, reminderDate time.Time) ([]*Deadline, error)
	DeleteDeadline(ctx context.Context, id string) error
	CountDeadlinesByUrgency(ctx context.Context, ownerID string) (map[DeadlineUrgency]int64, error)
}

// WithLifecyclePagination sets pagination options.
func WithLifecyclePagination(offset, limit int) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		if offset < 0 {
			offset = 0
		}
		if limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		o.Offset = offset
		o.Limit = limit
	}
}

// WithLifecycleSortBy sets sorting options.
func WithLifecycleSortBy(field string, ascending bool) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.SortField = field
		o.SortAscending = ascending
	}
}

// WithOwnerFilter sets owner filter.
func WithOwnerFilter(ownerID string) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.OwnerID = ownerID
	}
}

// WithDateRange sets date range filter.
func WithDateRange(from, to time.Time) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.FromDate = from
		o.ToDate = to
	}
}

// ApplyLifecycleOptions applies options.
func ApplyLifecycleOptions(opts ...LifecycleQueryOption) LifecycleQueryOptions {
	o := LifecycleQueryOptions{
		Offset: 0,
		Limit:  20,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
