// Package portfolio defines the repository interface for persisting and
// retrieving Portfolio aggregates.
package portfolio

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Repository defines persistence operations for Portfolio aggregates.
// Implementations must enforce multi-tenancy by scoping all queries to the
// TenantID in the context.
type Repository interface {
	// Save persists a new Portfolio to the data store.
	// Returns an error if a portfolio with the same ID already exists.
	Save(ctx context.Context, portfolio *Portfolio) error

	// FindByID retrieves a Portfolio by its unique ID.
	// Returns errors.CodePortfolioNotFound if not found.
	FindByID(ctx context.Context, id common.ID) (*Portfolio, error)

	// FindByOwner returns all portfolios owned by the given user, paginated.
	FindByOwner(ctx context.Context, ownerID common.UserID, page common.PageRequest) (*common.PageResponse[*Portfolio], error)

	// Update persists modifications to an existing Portfolio.
	// Uses optimistic locking: returns errors.CodeConflict if the Version
	// does not match the persisted value.
	Update(ctx context.Context, portfolio *Portfolio) error

	// Delete removes a Portfolio by ID.
	// Typically implements soft-delete by setting Status=StatusDeleted.
	Delete(ctx context.Context, id common.ID) error

	// FindByPatentID returns all portfolios that contain the specified patent.
	FindByPatentID(ctx context.Context, patentID common.ID) ([]*Portfolio, error)

	// FindByTag returns paginated portfolios that have the specified tag.
	FindByTag(ctx context.Context, tag string, page common.PageRequest) (*common.PageResponse[*Portfolio], error)
}

//Personal.AI order the ending
