package patent

import (
	"context"
	"time"

	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// Repository is the domain-layer port (interface) for patent persistence.
// Following the Dependency Inversion Principle, this interface is defined in
// the domain package and implemented by the infrastructure layer
// (internal/infrastructure/database/postgres/repositories/patent_repo.go).
//
// The domain layer depends only on this abstraction; it never imports any
// database driver, ORM, or storage SDK directly.  This keeps the domain model
// portable across storage backends (Postgres, in-memory, mock) and makes the
// full test suite runnable without a live database.
//
// All methods accept a context.Context so that deadlines, cancellations, and
// distributed trace spans propagate through the call stack.  Implementations
// must honour context cancellation and return ctx.Err() when appropriate.
//
// Error conventions:
//   - Record not found → errors.NotFound or errors.IsCode(err, errors.CodeNotFound)
//   - Optimistic lock conflict (version mismatch) → errors.IsCode(err, errors.CodeConflict)
//   - Infrastructure failure → errors.IsCode(err, errors.CodeDBConnectionError)
type Repository interface {
	// Save persists a newly created Patent aggregate.  Implementations must
	// store all nested value objects (Claims, Markush structures) atomically.
	// Returns an error if a patent with the same Number and Jurisdiction
	// already exists (CodeConflict).
	Save(ctx context.Context, patent *Patent) error

	// FindByID retrieves a Patent aggregate by its platform-internal UUID.
	// Returns CodeNotFound when no matching record exists.
	FindByID(ctx context.Context, id common.ID) (*Patent, error)

	// FindByNumber retrieves a Patent by its official publication number
	// (e.g., "CN202310001234A", "US10000001B2").
	// Returns CodeNotFound when no matching record exists.
	FindByNumber(ctx context.Context, number string) (*Patent, error)

	// FindByFamilyID returns all patents that belong to the specified patent
	// family.  An empty slice (not an error) is returned when no patents match.
	FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error)

	// Search executes a structured search across the patent corpus using the
	// criteria encoded in req.  It returns a paginated response containing
	// matching Patent DTOs and total-count metadata.
	Search(ctx context.Context, req ptypes.PatentSearchRequest) (*ptypes.PatentSearchResponse, error)

	// Update persists mutations to an existing Patent aggregate.
	// Implementations should perform an optimistic-lock check (Version field)
	// and return CodeConflict when the persisted version differs from
	// patent.Version.
	Update(ctx context.Context, patent *Patent) error

	// Delete soft-deletes the patent identified by id.  Soft-deleted patents
	// remain in the database for audit purposes but are excluded from all
	// queries unless explicitly requested.
	// Returns CodeNotFound when no matching record exists.
	Delete(ctx context.Context, id common.ID) error

	// FindByApplicant returns a paginated list of patents filed by the specified
	// applicant name (exact or fuzzy match depending on implementation).
	FindByApplicant(
		ctx context.Context,
		applicant string,
		page common.PageRequest,
	) (*common.PageResponse[*Patent], error)

	// FindByJurisdiction returns a paginated list of patents issued under the
	// specified jurisdiction code (e.g., CN, US, EP).
	FindByJurisdiction(
		ctx context.Context,
		jurisdiction ptypes.JurisdictionCode,
		page common.PageRequest,
	) (*common.PageResponse[*Patent], error)

	// FindByIPCCode returns a paginated list of patents classified under the
	// given IPC (International Patent Classification) code prefix
	// (e.g., "C07D", "C07D 401/04").  Implementations should support
	// hierarchical prefix matching.
	FindByIPCCode(
		ctx context.Context,
		ipcCode string,
		page common.PageRequest,
	) (*common.PageResponse[*Patent], error)

	// CountByStatus aggregates the total number of patents in each lifecycle
	// status (pending, granted, expired, etc.).  Useful for dashboard metrics.
	CountByStatus(ctx context.Context) (map[ptypes.PatentStatus]int64, error)

	// FindExpiring returns all granted patents whose expiry date falls on or
	// before the given time.Time threshold.  Used by the lifecycle-management
	// service to proactively notify portfolio managers.
	FindExpiring(ctx context.Context, before time.Time) ([]*Patent, error)
}

