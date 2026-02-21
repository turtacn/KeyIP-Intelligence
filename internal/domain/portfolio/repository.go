package portfolio

import (
	"context"
)

// QueryOptions aggregate all optional query parameters.
type QueryOptions struct {
	Offset        int
	Limit         int
	SortField     string
	SortAscending bool
	NameKeyword   string
	TagFilters    map[string]string
}

// QueryOption defines a function signature for providing query options.
type QueryOption func(*QueryOptions)

// WithPagination sets the pagination parameters.
func WithPagination(offset, limit int) QueryOption {
	return func(o *QueryOptions) {
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

// WithSortBy sets the sorting parameters.
func WithSortBy(field string, ascending bool) QueryOption {
	return func(o *QueryOptions) {
		o.SortField = field
		o.SortAscending = ascending
	}
}

// WithNameFilter sets a name keyword filter.
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

// ApplyOptions applies the provided options to a default QueryOptions.
func ApplyOptions(opts ...QueryOption) QueryOptions {
	options := QueryOptions{
		Offset: 0,
		Limit:  20,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

// PortfolioRepository defines the persistence contract for Portfolio aggregates.
type PortfolioRepository interface {
	// Save creates or updates a portfolio.
	Save(ctx context.Context, portfolio *Portfolio) error

	// FindByID retrieves a portfolio by its ID.
	FindByID(ctx context.Context, id string) (*Portfolio, error)

	// FindByOwnerID retrieves portfolios belonging to a specific owner.
	FindByOwnerID(ctx context.Context, ownerID string, opts ...QueryOption) ([]*Portfolio, error)

	// FindByStatus retrieves portfolios with a specific status.
	FindByStatus(ctx context.Context, status PortfolioStatus, opts ...QueryOption) ([]*Portfolio, error)

	// FindByTechDomain retrieves portfolios associated with a specific tech domain.
	FindByTechDomain(ctx context.Context, techDomain string, opts ...QueryOption) ([]*Portfolio, error)

	// Delete performs a logical deletion of a portfolio.
	Delete(ctx context.Context, id string) error

	// Count returns the total number of portfolios for an owner.
	Count(ctx context.Context, ownerID string) (int64, error)

	// ListSummaries retrieves summary views of portfolios for an owner.
	ListSummaries(ctx context.Context, ownerID string, opts ...QueryOption) ([]*PortfolioSummary, error)

	// FindContainingPatent finds all portfolios that contain the specified patent.
	FindContainingPatent(ctx context.Context, patentID string) ([]*Portfolio, error)
}

//Personal.AI order the ending
