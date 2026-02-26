package portfolio

import (
	"context"
)

// PortfolioRepository defines the persistence operations for portfolios.
type PortfolioRepository interface {
	Save(ctx context.Context, portfolio *Portfolio) error
	FindByID(ctx context.Context, id string) (*Portfolio, error)
	FindByOwnerID(ctx context.Context, ownerID string, opts ...QueryOption) ([]*Portfolio, error)
	FindByStatus(ctx context.Context, status PortfolioStatus, opts ...QueryOption) ([]*Portfolio, error)
	FindByTechDomain(ctx context.Context, techDomain string, opts ...QueryOption) ([]*Portfolio, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, ownerID string) (int64, error)
	ListSummaries(ctx context.Context, ownerID string, opts ...QueryOption) ([]*PortfolioSummary, error)
	FindContainingPatent(ctx context.Context, patentID string) ([]*Portfolio, error)
}

// QueryOptions encapsulates query parameters.
type QueryOptions struct {
	Offset        int
	Limit         int
	SortField     string
	SortAscending bool
	NameKeyword   string
	TagFilters    map[string]string
}

// QueryOption is a functional option for QueryOptions.
type QueryOption func(*QueryOptions)

// WithPagination sets pagination options.
func WithPagination(offset, limit int) QueryOption {
	return func(o *QueryOptions) {
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

// WithSortBy sets sorting options.
func WithSortBy(field string, ascending bool) QueryOption {
	return func(o *QueryOptions) {
		o.SortField = field
		o.SortAscending = ascending
	}
}

// WithNameFilter sets a name filter.
func WithNameFilter(keyword string) QueryOption {
	return func(o *QueryOptions) {
		o.NameKeyword = keyword
	}
}

// WithTagFilter adds a tag filter.
func WithTagFilter(key, value string) QueryOption {
	return func(o *QueryOptions) {
		if o.TagFilters == nil {
			o.TagFilters = make(map[string]string)
		}
		o.TagFilters[key] = value
	}
}

// ApplyOptions applies the functional options to create QueryOptions.
func ApplyOptions(opts ...QueryOption) QueryOptions {
	o := QueryOptions{
		Offset:     0,
		Limit:      20,
		TagFilters: make(map[string]string),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

//Personal.AI order the ending
