package lifecycle

import (
	"context"
	"time"
)

// LifecycleQueryOptions aggregate all optional query parameters.
type LifecycleQueryOptions struct {
	Offset        int
	Limit         int
	SortField     string
	SortAscending bool
	OwnerID       string
	FromDate      time.Time
	ToDate        time.Time
}

// LifecycleQueryOption defines a function signature for providing query options.
type LifecycleQueryOption func(*LifecycleQueryOptions)

// WithLifecyclePagination sets the pagination parameters.
func WithLifecyclePagination(offset, limit int) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		if offset < 0 {
			offset = 0
		}
		if limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		o.Offset = offset
		o.Limit = limit
	}
}

// WithLifecycleSortBy sets the sorting parameters.
func WithLifecycleSortBy(field string, ascending bool) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.SortField = field
		o.SortAscending = ascending
	}
}

// WithOwnerFilter sets an owner filter.
func WithOwnerFilter(ownerID string) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.OwnerID = ownerID
	}
}

// WithDateRange sets a date range filter.
func WithDateRange(from, to time.Time) LifecycleQueryOption {
	return func(o *LifecycleQueryOptions) {
		o.FromDate = from
		o.ToDate = to
	}
}

// ApplyLifecycleOptions applies the provided options to a default LifecycleQueryOptions.
func ApplyLifecycleOptions(opts ...LifecycleQueryOption) LifecycleQueryOptions {
	options := LifecycleQueryOptions{
		Offset: 0,
		Limit:  20,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// LifecycleRepository defines the persistence contract for LifecycleRecord aggregates.
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

// AnnuityRepository defines the persistence contract for AnnuityRecord entities.
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

// DeadlineRepository defines the persistence contract for Deadline entities.
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

//Personal.AI order the ending
