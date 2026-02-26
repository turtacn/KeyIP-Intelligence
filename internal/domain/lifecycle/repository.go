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
	Save(ctx context.Context, record *LifecycleRecord) error
	FindByID(ctx context.Context, id string) (*LifecycleRecord, error)
	FindByPatentID(ctx context.Context, patentID string) (*LifecycleRecord, error)
	FindByPhase(ctx context.Context, phase LifecyclePhase, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error)
	FindExpiring(ctx context.Context, withinDays int) ([]*LifecycleRecord, error)
	FindByJurisdiction(ctx context.Context, jurisdictionCode string, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, phase LifecyclePhase) (int64, error)
}

// AnnuityRepository defines persistence for AnnuityRecord.
type AnnuityRepository interface {
	Save(ctx context.Context, record *AnnuityRecord) error
	SaveBatch(ctx context.Context, records []*AnnuityRecord) error
	FindByID(ctx context.Context, id string) (*AnnuityRecord, error)
	FindByPatentID(ctx context.Context, patentID string) ([]*AnnuityRecord, error)
	FindByStatus(ctx context.Context, status AnnuityStatus) ([]*AnnuityRecord, error)
	FindPending(ctx context.Context, beforeDate time.Time) ([]*AnnuityRecord, error)
	FindOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error)
	SumByPortfolio(ctx context.Context, portfolioID string, fromDate, toDate time.Time) (int64, string, error)
	Delete(ctx context.Context, id string) error
}

// DeadlineRepository defines persistence for Deadline.
type DeadlineRepository interface {
	Save(ctx context.Context, deadline *Deadline) error
	FindByID(ctx context.Context, id string) (*Deadline, error)
	FindByPatentID(ctx context.Context, patentID string) ([]*Deadline, error)
	FindByOwnerID(ctx context.Context, ownerID string, from, to time.Time) ([]*Deadline, error)
	FindOverdue(ctx context.Context, ownerID string, asOf time.Time) ([]*Deadline, error)
	FindUpcoming(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error)
	FindByType(ctx context.Context, deadlineType DeadlineType) ([]*Deadline, error)
	FindPendingReminders(ctx context.Context, reminderDate time.Time) ([]*Deadline, error)
	Delete(ctx context.Context, id string) error
	CountByUrgency(ctx context.Context, ownerID string) (map[DeadlineUrgency]int64, error)
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
