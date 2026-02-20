// Package common provides foundational types shared across every layer of the
// KeyIP-Intelligence platform: domain entities, DTOs, request/response wrappers,
// and pagination primitives.  No business logic lives here.
package common

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────────────────────────────────────
// Primitive type aliases
// ─────────────────────────────────────────────────────────────────────────────

// ID is the platform-wide primary-key type, represented as a UUID string.
// Using a named type prevents accidental mixing of different ID domains at
// compile time.
type ID string

// TenantID identifies a tenant in the multi-tenant deployment model.
type TenantID string

// UserID identifies an individual user (human or service account).
type UserID string

// Timestamp is a named alias for time.Time.  It serialises to / from RFC 3339
// in JSON by default (standard library behaviour).
type Timestamp = time.Time

// Time is an alias for time.Time, used in repository queries and domain entities.
type Time = time.Time

// Metadata is an open-ended key-value bag attached to entities to carry
// domain-specific attributes without schema migrations.
type Metadata map[string]interface{}

// ─────────────────────────────────────────────────────────────────────────────
// Status — generic lifecycle status for platform entities
// ─────────────────────────────────────────────────────────────────────────────

// Status represents the lifecycle state of a platform entity.
type Status string

const (
	// StatusActive indicates the entity is live and fully operational.
	StatusActive Status = "active"

	// StatusInactive indicates the entity has been administratively disabled
	// but not permanently removed.
	StatusInactive Status = "inactive"

	// StatusPending indicates the entity has been created but is awaiting
	// a workflow step (e.g., ingestion, review, enrichment) before becoming active.
	StatusPending Status = "pending"

	// StatusArchived indicates the entity has been moved to long-term storage
	// and is no longer visible in default queries.
	StatusArchived Status = "archived"

	// StatusDeleted indicates a soft-deleted entity that is retained for audit
	// purposes but excluded from all normal business operations.
	StatusDeleted Status = "deleted"
)

// ─────────────────────────────────────────────────────────────────────────────
// ID generation
// ─────────────────────────────────────────────────────────────────────────────

// NewID generates a new random UUID v4 and returns it as an ID.
// It panics only if the underlying entropy source is broken, which is
// exceedingly rare on modern operating systems.
func NewID() ID {
	return ID(uuid.New().String())
}

// ─────────────────────────────────────────────────────────────────────────────
// BaseEntity — common audit fields embedded by all domain entities and DTOs
// ─────────────────────────────────────────────────────────────────────────────

// BaseEntity carries audit and concurrency-control metadata that every
// persistent domain entity and DTO in the platform must include.  Structs
// that need these fields should embed BaseEntity rather than redeclaring them.
//
// JSON tags use snake_case to match the PostgreSQL column naming convention
// and the OpenAPI specification generated from these types.
type BaseEntity struct {
	// ID is the globally unique identifier (UUID v4) for this entity.
	ID ID `json:"id"`

	// TenantID scopes the entity to a specific tenant in the multi-tenant model.
	TenantID TenantID `json:"tenant_id"`

	// CreatedAt is the UTC timestamp at which the entity was first persisted.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the UTC timestamp of the most recent mutation to the entity.
	UpdatedAt time.Time `json:"updated_at"`

	// CreatedBy records the UserID (human or service account) that created
	// the entity, used for audit logging.
	CreatedBy UserID `json:"created_by,omitempty"`

	// Version is an integer optimistic-lock counter incremented on every
	// successful write.  Conflicts are detected when a writer's Version does
	// not match the persisted value.
	Version int `json:"version"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Pagination primitives
// ─────────────────────────────────────────────────────────────────────────────

const (
	// defaultPageSize is applied by service layers when PageSize is zero.
	defaultPageSize = 20

	// maxPageSize is the hard upper bound enforced by Validate.
	maxPageSize = 1000
)

// PageRequest carries pagination and sorting parameters for list/search APIs.
// Page is 1-indexed (first page = 1).
type PageRequest struct {
	// Page is the 1-based page number to retrieve.
	Page int `json:"page" form:"page"`

	// PageSize is the maximum number of items per page (1–1000).
	PageSize int `json:"page_size" form:"page_size"`

	// SortBy is the field name on which results should be ordered.
	// An empty string means the repository uses its default ordering.
	SortBy string `json:"sort_by,omitempty" form:"sort_by"`

	// SortOrder controls the direction of sorting.
	// Accepted values: "asc", "desc".  Defaults to "asc" in the service layer.
	SortOrder string `json:"sort_order,omitempty" form:"sort_order"`
}

// Validate checks that the pagination parameters are within accepted bounds.
// Returns a descriptive error for the first violation found.
//
//   - Page must be ≥ 1.
//   - PageSize must be between 1 and maxPageSize (1 000) inclusive.
//   - SortOrder, when non-empty, must be "asc" or "desc".
func (r *PageRequest) Validate() error {
	if r.Page < 1 {
		return fmt.Errorf("page must be ≥ 1, got %d", r.Page)
	}
	if r.PageSize < 1 {
		return fmt.Errorf("page_size must be ≥ 1, got %d", r.PageSize)
	}
	if r.PageSize > maxPageSize {
		return fmt.Errorf("page_size must be ≤ %d, got %d", maxPageSize, r.PageSize)
	}
	if r.SortOrder != "" && r.SortOrder != "asc" && r.SortOrder != "desc" {
		return fmt.Errorf("sort_order must be \"asc\" or \"desc\", got %q", r.SortOrder)
	}
	return nil
}

// Offset returns the zero-based record offset corresponding to this page,
// useful for SQL OFFSET clauses.
func (r *PageRequest) Offset() int {
	if r.Page < 1 {
		return 0
	}
	return (r.Page - 1) * r.PageSize
}

// PageResponse is the generic paginated response wrapper used by all list and
// search APIs in the platform.  T is the element type (e.g., PatentDTO,
// MoleculeDTO).
type PageResponse[T any] struct {
	// Items holds the current page of results.
	Items []T `json:"items"`

	// Total is the total number of matching records across all pages.
	Total int64 `json:"total"`

	// Page is the 1-based index of the current page.
	Page int `json:"page"`

	// PageSize is the maximum number of items returned per page.
	PageSize int `json:"page_size"`

	// TotalPages is the computed ceiling of Total / PageSize.
	TotalPages int `json:"total_pages"`
}

// NewPageResponse constructs a PageResponse from the full result set,
// computing TotalPages automatically.
func NewPageResponse[T any](items []T, total int64, req PageRequest) PageResponse[T] {
	ps := req.PageSize
	if ps <= 0 {
		ps = defaultPageSize
	}
	totalPages := 0
	if ps > 0 && total > 0 {
		totalPages = int((total + int64(ps) - 1) / int64(ps))
	}
	return PageResponse[T]{
		Items:      items,
		Total:      total,
		Page:       req.Page,
		PageSize:   ps,
		TotalPages: totalPages,
	}
}

//Personal.AI order the ending
