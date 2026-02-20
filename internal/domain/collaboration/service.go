// Package collaboration implements the domain service for workspace management.
package collaboration

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Service encapsulates business logic for workspace operations that spans
// multiple aggregates or requires coordination with external systems.
type Service struct {
	repo   Repository
	logger logging.Logger
}

// NewService constructs a new collaboration domain service.
func NewService(repo Repository, logger logging.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Workspace lifecycle operations
// ─────────────────────────────────────────────────────────────────────────────

// CreateWorkspace creates a new workspace and persists it.
func (s *Service) CreateWorkspace(ctx context.Context, name, description string, ownerID common.UserID) (*Workspace, error) {
	ws, err := NewWorkspace(name, description, ownerID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidParam, "failed to create workspace")
	}

	if err := s.repo.SaveWorkspace(ctx, ws); err != nil {
		s.logger.Error("failed to save workspace", logging.Err(err))
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to persist workspace")
	}

	s.logger.Info("created workspace",
		logging.String("workspace_id", string(ws.ID)),
		logging.String("owner_id", string(ownerID)))
	return ws, nil
}

// UpdateWorkspace updates workspace metadata.
func (s *Service) UpdateWorkspace(ctx context.Context, id common.ID, name, description string, updaterID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	// Check permission.
	updaterRole, err := ws.GetMemberRole(updaterID)
	if err != nil {
		return errors.Forbidden("updater is not a member")
	}
	if !HasPermission(updaterRole, "manage", "workspace") {
		return errors.Forbidden(fmt.Sprintf("role %s cannot manage workspace", updaterRole))
	}

	if name != "" {
		ws.Name = name
	}
	ws.Description = description
	ws.UpdatedAt = time.Now().UTC()
	ws.Version++

	if err := s.repo.UpdateWorkspace(ctx, ws); err != nil {
		s.logger.Error("failed to update workspace", logging.Err(err))
		return errors.Wrap(err, errors.CodeInternal, "failed to persist workspace update")
	}

	s.logger.Info("workspace updated",
		logging.String("workspace_id", string(id)),
		logging.String("updater_id", string(updaterID)))
	return nil
}

// DeleteWorkspace deletes a workspace. Only the owner can perform this action.
func (s *Service) DeleteWorkspace(ctx context.Context, id common.ID, deleterID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, id)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	if ws.OwnerID != deleterID {
		return errors.Forbidden("only the owner can delete a workspace")
	}

	if err := s.repo.DeleteWorkspace(ctx, id); err != nil {
		s.logger.Error("failed to delete workspace", logging.Err(err))
		return errors.Wrap(err, errors.CodeInternal, "failed to delete workspace")
	}

	s.logger.Info("workspace deleted",
		logging.String("workspace_id", string(id)),
		logging.String("deleter_id", string(deleterID)))
	return nil
}

// GetWorkspace retrieves a workspace by ID.
func (s *Service) GetWorkspace(ctx context.Context, id common.ID) (*Workspace, error) {
	ws, err := s.repo.FindWorkspaceByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}
	return ws, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Member management operations
// ─────────────────────────────────────────────────────────────────────────────

// InviteMember adds a new member to the workspace.
// The inviter must have the PermMemberInvite permission.
func (s *Service) InviteMember(ctx context.Context, workspaceID common.ID, userID common.UserID, role MemberRole, inviterID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	// Check inviter permission.
	inviterRole, err := ws.GetMemberRole(inviterID)
	if err != nil {
		return errors.Forbidden("inviter is not a member of this workspace")
	}
	if !HasPermission(inviterRole, "invite", "member") {
		return errors.Forbidden(fmt.Sprintf("role %s cannot invite members", inviterRole))
	}

	if err := ws.AddMember(userID, role, inviterID); err != nil {
		return errors.Wrap(err, errors.CodeConflict, "failed to add member")
	}

	if err := s.repo.UpdateWorkspace(ctx, ws); err != nil {
		s.logger.Error("failed to update workspace after adding member", logging.Err(err))
		return errors.Wrap(err, errors.CodeInternal, "failed to persist member addition")
	}

	s.logger.Info("user invited to workspace",
		logging.String("inviter_id", string(inviterID)),
		logging.String("user_id", string(userID)),
		logging.String("workspace_id", string(workspaceID)),
		logging.String("role", string(role)))
	return nil
}

// RemoveMember removes a member from the workspace.
// The remover must be an Admin or Owner.
func (s *Service) RemoveMember(ctx context.Context, workspaceID common.ID, userID common.UserID, removerID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	// Check remover permission.
	removerRole, err := ws.GetMemberRole(removerID)
	if err != nil {
		return errors.Forbidden("remover is not a member")
	}
	if !HasPermission(removerRole, "remove", "member") {
		return errors.Forbidden(fmt.Sprintf("role %s cannot remove members", removerRole))
	}

	if err := ws.RemoveMember(userID); err != nil {
		return errors.Wrap(err, errors.CodeForbidden, "failed to remove member")
	}

	if err := s.repo.UpdateWorkspace(ctx, ws); err != nil {
		s.logger.Error("failed to update workspace after removing member", logging.Err(err))
		return errors.Wrap(err, errors.CodeInternal, "failed to persist member removal")
	}

	s.logger.Info("user removed from workspace",
		logging.String("remover_id", string(removerID)),
		logging.String("user_id", string(userID)),
		logging.String("workspace_id", string(workspaceID)))
	return nil
}

// ChangeMemberRole updates a member's role within the workspace.
func (s *Service) ChangeMemberRole(ctx context.Context, workspaceID common.ID, userID common.UserID, newRole MemberRole, updaterID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	// Only Owner and Admin can change roles.
	updaterRole, err := ws.GetMemberRole(updaterID)
	if err != nil {
		return errors.Forbidden("updater is not a member")
	}
	if updaterRole != RoleOwner && updaterRole != RoleAdmin {
		return errors.Forbidden(fmt.Sprintf("role %s cannot change member roles", updaterRole))
	}

	if err := ws.ChangeMemberRole(userID, newRole); err != nil {
		return errors.Wrap(err, errors.CodeConflict, "failed to change member role")
	}

	if err := s.repo.UpdateWorkspace(ctx, ws); err != nil {
		s.logger.Error("failed to update workspace after role change", logging.Err(err))
		return errors.Wrap(err, errors.CodeInternal, "failed to persist role change")
	}

	s.logger.Info("member role changed",
		logging.String("workspace_id", string(workspaceID)),
		logging.String("user_id", string(userID)),
		logging.String("new_role", string(newRole)),
		logging.String("updater_id", string(updaterID)))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Resource sharing operations
// ─────────────────────────────────────────────────────────────────────────────

// ShareResource shares a patent, portfolio, or report with all workspace members.
// The sharer must be a member with write permissions.
func (s *Service) ShareResource(ctx context.Context, workspaceID common.ID, resource SharedResource, sharerID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	// Check sharer permission.
	sharerRole, err := ws.GetMemberRole(sharerID)
	if err != nil {
		return errors.Forbidden("sharer is not a member")
	}
	// Editor+ can share resources.
	if sharerRole != RoleOwner && sharerRole != RoleAdmin && sharerRole != RoleEditor {
		return errors.Forbidden(fmt.Sprintf("role %s cannot share resources", sharerRole))
	}

	resource.SharedBy = sharerID
	if err := ws.ShareResource(resource); err != nil {
		return errors.Wrap(err, errors.CodeConflict, "failed to share resource")
	}

	if err := s.repo.UpdateWorkspace(ctx, ws); err != nil {
		s.logger.Error("failed to update workspace after sharing resource", logging.Err(err))
		return errors.Wrap(err, errors.CodeInternal, "failed to persist resource sharing")
	}

	s.logger.Info("resource shared in workspace",
		logging.String("sharer_id", string(sharerID)),
		logging.String("resource_type", resource.ResourceType),
		logging.String("resource_id", string(resource.ResourceID)),
		logging.String("workspace_id", string(workspaceID)))
	return nil
}

// UnshareResource removes a resource from the workspace.
func (s *Service) UnshareResource(ctx context.Context, workspaceID common.ID, resourceID common.ID, removerID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	removerRole, err := ws.GetMemberRole(removerID)
	if err != nil {
		return errors.Forbidden("remover is not a member")
	}

	// Owner and Admin can unshare anything.
	if removerRole != RoleOwner && removerRole != RoleAdmin {
		// Editors can unshare resources they shared themselves.
		isSharer := false
		for _, r := range ws.SharedResources {
			if r.ResourceID == resourceID && r.SharedBy == removerID {
				isSharer = true
				break
			}
		}
		if !isSharer {
			return errors.Forbidden("only admins or the original sharer can unshare a resource")
		}
	}

	if err := ws.UnshareResource(resourceID); err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "failed to unshare resource")
	}

	if err := s.repo.UpdateWorkspace(ctx, ws); err != nil {
		s.logger.Error("failed to update workspace after unsharing resource", logging.Err(err))
		return errors.Wrap(err, errors.CodeInternal, "failed to persist resource unsharing")
	}

	s.logger.Info("resource unshared",
		logging.String("workspace_id", string(workspaceID)),
		logging.String("resource_id", string(resourceID)),
		logging.String("remover_id", string(removerID)))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Access control queries
// ─────────────────────────────────────────────────────────────────────────────

// CheckAccess verifies whether a user has the required access level to a resource
// within a workspace.  Returns true if access is granted, false otherwise.
func (s *Service) CheckAccess(ctx context.Context, workspaceID common.ID, userID common.UserID, resourceID common.ID, requiredLevel string) (bool, error) {
	ws, err := s.repo.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return false, errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	hasAccess := ws.HasAccess(userID, resourceID, requiredLevel)
	return hasAccess, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Query operations
// ─────────────────────────────────────────────────────────────────────────────

// GetUserWorkspaces retrieves all workspaces where the user is a member,
// paginated according to the page request.
func (s *Service) GetUserWorkspaces(ctx context.Context, userID common.UserID, page common.PageRequest) (*common.PageResponse[*Workspace], error) {
	if err := page.Validate(); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidParam, "invalid pagination parameters")
	}

	resp, err := s.repo.FindWorkspacesByUser(ctx, userID, page)
	if err != nil {
		s.logger.Error("failed to find workspaces for user",
			logging.String("user_id", string(userID)),
			logging.Err(err))
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to query workspaces")
	}

	return resp, nil
}

// GetWorkspacesByResource retrieves all workspaces that have shared the given resource.
func (s *Service) GetWorkspacesByResource(ctx context.Context, resourceID common.ID) ([]*Workspace, error) {
	workspaces, err := s.repo.FindWorkspacesByResource(ctx, resourceID)
	if err != nil {
		s.logger.Error("failed to find workspaces by resource",
			logging.String("resource_id", string(resourceID)),
			logging.Err(err))
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to query workspaces by resource")
	}
	return workspaces, nil
}

//Personal.AI order the ending
