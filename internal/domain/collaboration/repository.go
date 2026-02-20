// Package collaboration defines the repository interface for workspace persistence.
package collaboration

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Repository defines the persistence contract for the Workspace aggregate.
// Implementations must handle multi-tenancy isolation and optimistic locking.
type Repository interface {
	// SaveWorkspace persists a new workspace to the data store.
	// Returns an error if a workspace with the same ID already exists.
	SaveWorkspace(ctx context.Context, ws *Workspace) error

	// FindWorkspaceByID retrieves a workspace by its unique identifier.
	// Returns errors.CodeNotFound if no such workspace exists.
	FindWorkspaceByID(ctx context.Context, id common.ID) (*Workspace, error)

	// FindWorkspacesByUser returns all workspaces where the given user is a member,
	// paginated according to the PageRequest.
	FindWorkspacesByUser(ctx context.Context, userID common.UserID, page common.PageRequest) (*common.PageResponse[*Workspace], error)

	// UpdateWorkspace persists changes to an existing workspace.
	// Must enforce optimistic locking via the Version field: if the persisted
	// Version differs from ws.Version, return errors.CodeConflict.
	UpdateWorkspace(ctx context.Context, ws *Workspace) error

	// DeleteWorkspace removes a workspace from the data store.
	// Implementations may choose soft-delete (set Status = StatusDeleted) or
	// hard-delete depending on audit requirements.
	DeleteWorkspace(ctx context.Context, id common.ID) error

	// FindWorkspacesByResource returns all workspaces that have shared the
	// given resource (patent, portfolio, or report).
	// Used by the application layer to determine which users can access a resource.
	FindWorkspacesByResource(ctx context.Context, resourceID common.ID) ([]*Workspace, error)
}

//Personal.AI order the ending
