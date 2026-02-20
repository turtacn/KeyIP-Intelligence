// Package collaboration implements the domain service for workspace management.
package collaboration

import (
	"context"
	"fmt"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/logging"
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
		s.logger.Errorf("failed to save workspace: %v", err)
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to persist workspace")
	}

	s.logger.Infof("created workspace %s for owner %s", ws.ID, ownerID)
	return ws, nil
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
		s.logger.Errorf("failed to update workspace after adding member: %v", err)
		return errors.Wrap(err, errors.CodeInternal, "failed to persist member addition")
	}

	s.logger.Infof("user %s invited user %s to workspace %s with role %s", inviterID, userID, workspaceID, role)
	return nil
}

// RemoveMember removes a member from the workspace.
// The remover must be an Admin or Owner.
func (s *Service) RemoveMember(ctx context.Context, workspaceID common.ID, userID common.UserID, removerID common.UserID) error {
	ws, err := s.repo.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return errors.Wrap(err, errors.CodeNotFound, "workspace not found")
	}

	// Check remover permission (Admin+ can remove members).
	removerRole, err := ws.GetMemberRole(removerID)
	if err != nil {
		return errors.Forbidden("remover is not a member")
	}
	if removerRole != RoleOwner && removerRole != RoleAdmin {
		return errors.Forbidden(fmt.Sprintf("role %s cannot remove members", removerRole))
	}

	if err := ws.RemoveMember(userID); err != nil {
		return errors.Wrap(err, errors.CodeForbidden, "failed to remove member")
	}

	if err := s.repo.UpdateWorkspace(ctx, ws); err != nil {
		s.logger.Errorf("failed to update workspace after removing member: %v", err)
		return errors.Wrap(err, errors.CodeInternal, "failed to persist member removal")
	}

	s.logger.Infof("user %s removed user %s from workspace %s", removerID, userID, workspaceID)
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
		s.logger.Errorf("failed to update workspace after sharing resource: %v", err)
		return errors.Wrap(err, errors.CodeInternal, "failed to persist resource sharing")
	}

	s.logger.Infof("user %s shared %s %s in workspace %s", sharerID, resource.ResourceType, resource.ResourceID, workspaceID)
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
		s.logger.Errorf("failed to find workspaces for user %s: %v", userID, err)
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to query workspaces")
	}

	return resp, nil
}

//Personal.AI order the ending
