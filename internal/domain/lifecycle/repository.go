// Package lifecycle provides the repository interface for patent lifecycle
// aggregate persistence.
package lifecycle

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// Repository interface
// ─────────────────────────────────────────────────────────────────────────────

// Repository defines the persistence contract for PatentLifecycle aggregates.
// Implementations should be idempotent and thread-safe.
type Repository interface {
	// Save persists a PatentLifecycle aggregate (create or update).
	// If the lifecycle already exists, it is updated; otherwise, it is created.
	Save(ctx context.Context, lifecycle *PatentLifecycle) error

	// FindByID retrieves a PatentLifecycle by its ID.
	// Returns ErrNotFound if the lifecycle does not exist.
	FindByID(ctx context.Context, id common.ID) (*PatentLifecycle, error)

	// FindByPatentID retrieves the PatentLifecycle for a specific patent.
	// Returns ErrNotFound if no lifecycle exists for this patent.
	FindByPatentID(ctx context.Context, patentID common.ID) (*PatentLifecycle, error)

	// FindByPatentNumber retrieves the PatentLifecycle by patent number.
	// Returns ErrNotFound if the patent number is not found.
	FindByPatentNumber(ctx context.Context, patentNumber string) (*PatentLifecycle, error)

	// FindUpcomingDeadlines retrieves all lifecycles with deadlines due within
	// the specified number of days across all tenants (or scoped to tenant if
	// tenantID is provided).
	FindUpcomingDeadlines(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*PatentLifecycle, error)

	// FindOverdueDeadlines retrieves all lifecycles with overdue deadlines.
	FindOverdueDeadlines(ctx context.Context, tenantID *common.TenantID) ([]*PatentLifecycle, error)

	// FindUpcomingAnnuities retrieves all lifecycles with annuity payments due
	// within the specified number of days.
	FindUpcomingAnnuities(ctx context.Context, withinDays int, tenantID *common.TenantID) ([]*PatentLifecycle, error)

	// FindByJurisdiction retrieves all lifecycles for a specific jurisdiction.
	FindByJurisdiction(ctx context.Context, jurisdiction ptypes.JurisdictionCode, tenantID *common.TenantID) ([]*PatentLifecycle, error)

	// Delete removes a PatentLifecycle from the repository.
	// This is typically used only for test cleanup or administrative purposes.
	Delete(ctx context.Context, id common.ID) error

	// List retrieves lifecycles with pagination support.
	List(ctx context.Context, offset, limit int, tenantID *common.TenantID) ([]*PatentLifecycle, int64, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Query specifications (optional, for advanced filtering)
// ─────────────────────────────────────────────────────────────────────────────

// LifecycleQuery represents advanced filtering criteria for lifecycle searches.
// This can be used to implement complex queries in the repository.
type LifecycleQuery struct {
	// TenantID filters by tenant.
	TenantID *common.TenantID

	// PatentIDs filters by a list of patent IDs.
	PatentIDs []common.ID

	// Jurisdictions filters by a list of jurisdiction codes.
	Jurisdictions []ptypes.JurisdictionCode

	// LegalStatus filters by current legal status (e.g., "pending", "granted").
	LegalStatus []string

	// HasOverdueDeadlines filters lifecycles with at least one overdue deadline.
	HasOverdueDeadlines *bool

	// HasUnpaidAnnuities filters lifecycles with at least one unpaid annuity.
	HasUnpaidAnnuities *bool

	// ExpiresAfter filters lifecycles that expire after the specified date.
	ExpiresAfter *common.Time

	// ExpiresBefore filters lifecycles that expire before the specified date.
	ExpiresBefore *common.Time
}

// FindByQuery retrieves lifecycles matching the specified query criteria.
// This is an optional advanced feature that repositories may implement.
type AdvancedRepository interface {
	Repository
	FindByQuery(ctx context.Context, query *LifecycleQuery, offset, limit int) ([]*PatentLifecycle, int64, error)
}

//Personal.AI order the ending
