// Package collaboration implements the collaboration bounded context for the
// KeyIP-Intelligence platform, enabling multi-user workspaces with fine-grained
// access control over shared patents, portfolios, and reports.
package collaboration

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// MemberRole — enumeration of workspace membership roles
// ─────────────────────────────────────────────────────────────────────────────

// MemberRole defines the privilege level a user has within a workspace.
// Roles form a strict hierarchy: Owner > Admin > Editor > Viewer.
type MemberRole string

const (
	// RoleOwner has full control: can delete workspace, manage all members,
	// and perform all resource operations.
	RoleOwner MemberRole = "owner"

	// RoleAdmin can manage members (invite, remove, change roles except Owner),
	// share/unshare resources, and perform all resource operations.
	RoleAdmin MemberRole = "admin"

	// RoleEditor can read and modify shared resources but cannot manage
	// workspace membership or sharing settings.
	RoleEditor MemberRole = "editor"

	// RoleViewer has read-only access to all shared resources in the workspace.
	RoleViewer MemberRole = "viewer"
)

// ─────────────────────────────────────────────────────────────────────────────
// Value objects
// ─────────────────────────────────────────────────────────────────────────────

// Member represents a user's membership in a workspace.
// It is a value object embedded in the Workspace aggregate.
type Member struct {
	// UserID uniquely identifies the user across the platform.
	UserID common.UserID `json:"user_id"`

	// Role defines the user's privilege level within this workspace.
	Role MemberRole `json:"role"`

	// JoinedAt records when the user became a member (accepted invitation).
	JoinedAt time.Time `json:"joined_at"`

	// InvitedBy records which user sent the invitation that led to this membership.
	InvitedBy common.UserID `json:"invited_by"`
}

// SharedResource represents a patent, portfolio, or report that has been
// shared with all members of a workspace.
type SharedResource struct {
	// ResourceID is the platform-wide unique identifier of the shared entity.
	ResourceID common.ID `json:"resource_id"`

	// ResourceType classifies the entity: "patent", "portfolio", "report".
	ResourceType string `json:"resource_type"`

	// SharedBy records the user who added this resource to the workspace.
	SharedBy common.UserID `json:"shared_by"`

	// SharedAt is the UTC timestamp when the resource was shared.
	SharedAt time.Time `json:"shared_at"`

	// AccessLevel defines the maximum permission any workspace member has:
	// "read" or "write".  Individual member roles may further restrict this.
	AccessLevel string `json:"access_level"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Workspace — aggregate root
// ─────────────────────────────────────────────────────────────────────────────

// Workspace is the aggregate root for the collaboration bounded context.
// It groups users and shared resources together under a common namespace with
// role-based access control.
type Workspace struct {
	common.BaseEntity

	// Name is the human-readable workspace identifier shown in the UI.
	Name string `json:"name"`

	// Description explains the purpose of this workspace (optional).
	Description string `json:"description"`

	// OwnerID identifies the user who created the workspace and holds ultimate
	// control.  The owner cannot be removed and has all permissions.
	OwnerID common.UserID `json:"owner_id"`

	// Members lists all users with access to this workspace, including the owner.
	Members []Member `json:"members"`

	// SharedResources lists all patents, portfolios, and reports accessible to
	// workspace members.
	SharedResources []SharedResource `json:"shared_resources"`

	// Status tracks the workspace lifecycle (active, archived, deleted).
	Status common.Status `json:"status"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewWorkspace constructs a new workspace with the given owner.
// The owner is automatically added as a member with RoleOwner.
// Returns an error if name is empty.
func NewWorkspace(name, description string, ownerID common.UserID) (*Workspace, error) {
	if name == "" {
		return nil, errors.InvalidParam("workspace name must not be empty")
	}
	if ownerID == "" {
		return nil, errors.InvalidParam("owner ID must not be empty")
	}

	now := time.Now().UTC()
	ws := &Workspace{
		BaseEntity: common.BaseEntity{
			ID:        common.NewID(),
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		},
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		Status:      common.StatusActive,
		Members: []Member{
			{
				UserID:    ownerID,
				Role:      RoleOwner,
				JoinedAt:  now,
				InvitedBy: ownerID, // owner invites themselves
			},
		},
		SharedResources: []SharedResource{},
	}

	return ws, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Member management
// ─────────────────────────────────────────────────────────────────────────────

// AddMember adds a new member to the workspace.
// Returns an error if the user is already a member.
func (w *Workspace) AddMember(userID common.UserID, role MemberRole, invitedBy common.UserID) error {
	if userID == "" {
		return errors.InvalidParam("user ID must not be empty")
	}

	// Check for duplicate membership.
	for _, m := range w.Members {
		if m.UserID == userID {
			return errors.Conflict(fmt.Sprintf("user %s is already a member", userID))
		}
	}

	w.Members = append(w.Members, Member{
		UserID:    userID,
		Role:      role,
		JoinedAt:  time.Now().UTC(),
		InvitedBy: invitedBy,
	})
	w.UpdatedAt = time.Now().UTC()
	w.Version++

	return nil
}

// RemoveMember removes a member from the workspace.
// The owner cannot be removed; attempting to do so returns an error.
func (w *Workspace) RemoveMember(userID common.UserID) error {
	if userID == w.OwnerID {
		return errors.Forbidden("cannot remove workspace owner")
	}

	idx := -1
	for i, m := range w.Members {
		if m.UserID == userID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return errors.NotFound(fmt.Sprintf("user %s is not a member", userID))
	}

	// Remove by replacing with last element and truncating.
	w.Members[idx] = w.Members[len(w.Members)-1]
	w.Members = w.Members[:len(w.Members)-1]

	w.UpdatedAt = time.Now().UTC()
	w.Version++

	return nil
}

// ChangeMemberRole updates a member's role.
// The owner's role cannot be changed.
func (w *Workspace) ChangeMemberRole(userID common.UserID, newRole MemberRole) error {
	if userID == w.OwnerID {
		return errors.Forbidden("cannot change owner role")
	}

	found := false
	for i := range w.Members {
		if w.Members[i].UserID == userID {
			w.Members[i].Role = newRole
			found = true
			break
		}
	}

	if !found {
		return errors.NotFound(fmt.Sprintf("user %s is not a member", userID))
	}

	w.UpdatedAt = time.Now().UTC()
	w.Version++

	return nil
}

// IsMember returns true if the given user is a member of this workspace.
func (w *Workspace) IsMember(userID common.UserID) bool {
	for _, m := range w.Members {
		if m.UserID == userID {
			return true
		}
	}
	return false
}

// GetMemberRole returns the role of a member.
// Returns an error if the user is not a member.
func (w *Workspace) GetMemberRole(userID common.UserID) (MemberRole, error) {
	for _, m := range w.Members {
		if m.UserID == userID {
			return m.Role, nil
		}
	}
	return "", errors.NotFound(fmt.Sprintf("user %s is not a member", userID))
}

// ─────────────────────────────────────────────────────────────────────────────
// Resource sharing
// ─────────────────────────────────────────────────────────────────────────────

// ShareResource adds a resource to the workspace's shared collection.
// Returns an error if the resource is already shared.
func (w *Workspace) ShareResource(resource SharedResource) error {
	if resource.ResourceID == "" {
		return errors.InvalidParam("resource ID must not be empty")
	}
	if resource.ResourceType == "" {
		return errors.InvalidParam("resource type must not be empty")
	}
	if resource.AccessLevel != "read" && resource.AccessLevel != "write" {
		return errors.InvalidParam("access level must be 'read' or 'write'")
	}

	// Check for duplicate.
	for _, r := range w.SharedResources {
		if r.ResourceID == resource.ResourceID {
			return errors.Conflict(fmt.Sprintf("resource %s is already shared", resource.ResourceID))
		}
	}

	resource.SharedAt = time.Now().UTC()
	w.SharedResources = append(w.SharedResources, resource)
	w.UpdatedAt = time.Now().UTC()
	w.Version++

	return nil
}

// UnshareResource removes a resource from the workspace's shared collection.
func (w *Workspace) UnshareResource(resourceID common.ID) error {
	idx := -1
	for i, r := range w.SharedResources {
		if r.ResourceID == resourceID {
			idx = i
			break
		}
	}

	if idx == -1 {
		return errors.NotFound(fmt.Sprintf("resource %s is not shared in this workspace", resourceID))
	}

	w.SharedResources[idx] = w.SharedResources[len(w.SharedResources)-1]
	w.SharedResources = w.SharedResources[:len(w.SharedResources)-1]

	w.UpdatedAt = time.Now().UTC()
	w.Version++

	return nil
}

// HasAccess checks if a user has the required access level to a resource.
// Returns true if:
// 1. The user is a member of the workspace
// 2. The resource is shared in this workspace
// 3. The user's role grants sufficient permissions
// 4. The resource's access level permits the required operation
func (w *Workspace) HasAccess(userID common.UserID, resourceID common.ID, requiredLevel string) bool {
	// Check membership.
	role, err := w.GetMemberRole(userID)
	if err != nil {
		return false
	}

	// Find the resource.
	var resource *SharedResource
	for i := range w.SharedResources {
		if w.SharedResources[i].ResourceID == resourceID {
			resource = &w.SharedResources[i]
			break
		}
	}
	if resource == nil {
		return false
	}

	// Owner and Admin always have full access.
	if role == RoleOwner || role == RoleAdmin {
		return true
	}

	// For Editor and Viewer, check the resource's access level.
	if requiredLevel == "write" {
		// Write requires both role Editor+ and resource access_level "write".
		return role == RoleEditor && resource.AccessLevel == "write"
	}

	// Read access: Editor and Viewer both have read on any shared resource.
	return role == RoleEditor || role == RoleViewer
}

//Personal.AI order the ending
