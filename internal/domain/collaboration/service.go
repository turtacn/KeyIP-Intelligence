package collaboration

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// WorkspaceSummary provides a summary of a workspace for user listing.
type WorkspaceSummary struct {
	WorkspaceID    string    `json:"workspace_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	MyRole         Role      `json:"my_role"`
	MemberCount    int       `json:"member_count"`
	PatentCount    int       `json:"patent_count"`
	LastActivityAt time.Time `json:"last_activity_at"`
}

// ActivityRecord represents an event in a workspace.
type ActivityRecord struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspace_id"`
	ActorID     string            `json:"actor_id"`
	ActorName   string            `json:"actor_name"`
	ActionType  string            `json:"action_type"`
	Description string            `json:"description"`
	TargetType  string            `json:"target_type"`
	TargetID    string            `json:"target_id"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// CollaborationService defines the service interface for collaboration.
type CollaborationService interface {
	CreateWorkspace(ctx context.Context, name, ownerID string) (*Workspace, error)
	InviteMember(ctx context.Context, workspaceID, inviterID, inviteeUserID string, role Role) (*MemberPermission, error)
	AcceptInvitation(ctx context.Context, memberID, userID string) error
	RemoveMember(ctx context.Context, workspaceID, removerID, targetUserID string) error
	ChangeMemberRole(ctx context.Context, workspaceID, changerID, targetUserID string, newRole Role) error
	CheckMemberAccess(ctx context.Context, workspaceID, userID string, resource ResourceType, action Action) (bool, string, error)
	GetWorkspaceMembers(ctx context.Context, workspaceID, requesterID string) ([]*MemberPermission, error)
	GetUserWorkspaces(ctx context.Context, userID string) ([]*WorkspaceSummary, error)
	TransferOwnership(ctx context.Context, workspaceID, currentOwnerID, newOwnerID string) error
	UpdateWorkspace(ctx context.Context, workspaceID, requesterID, name, description string) error
	DeleteWorkspace(ctx context.Context, workspaceID, requesterID string) error
	GrantCustomPermission(ctx context.Context, workspaceID, granterID, targetUserID string, perm *Permission) error
	RevokeCustomPermission(ctx context.Context, workspaceID, revokerID, targetUserID string, resource ResourceType, action Action) error
	GetWorkspaceActivity(ctx context.Context, workspaceID string, limit int) ([]*ActivityRecord, error)
}

type collaborationServiceImpl struct {
	workspaceRepo WorkspaceRepository
	memberRepo    MemberRepository
	policy        PermissionPolicy
}

// NewCollaborationService creates a new collaboration service implementation.
func NewCollaborationService(workspaceRepo WorkspaceRepository, memberRepo MemberRepository, policy PermissionPolicy) CollaborationService {
	return &collaborationServiceImpl{
		workspaceRepo: workspaceRepo,
		memberRepo:    memberRepo,
		policy:        policy,
	}
}

func (s *collaborationServiceImpl) CreateWorkspace(ctx context.Context, name, ownerID string) (*Workspace, error) {
	if name == "" || len(name) > 100 {
		return nil, errors.InvalidParam("workspace name must be between 1 and 100 characters")
	}

	slug := GenerateSlug(name)
	existing, _ := s.workspaceRepo.FindBySlug(ctx, slug)
	if existing != nil {
		slug = GenerateSlugWithSuffix(slug)
	}

	ws, err := NewWorkspace(name, ownerID)
	if err != nil {
		return nil, err
	}
	ws.Slug = slug

	mp, err := NewMemberPermission(ws.ID, ownerID, RoleOwner, ownerID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	mp.AcceptedAt = &now

	if err := s.workspaceRepo.Save(ctx, ws); err != nil {
		return nil, err
	}
	if err := s.memberRepo.Save(ctx, mp); err != nil {
		return nil, err
	}

	// Activity record would be created here if repository existed
	return ws, nil
}

func (s *collaborationServiceImpl) InviteMember(ctx context.Context, workspaceID, inviterID, inviteeUserID string, role Role) (*MemberPermission, error) {
	inviter, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, inviterID)
	if err != nil {
		return nil, errors.Forbidden("inviter is not a member of the workspace")
	}

	allowed, _ := s.policy.CheckAccess(inviter, ResourceWorkspace, ActionManageMembers)
	if !allowed {
		return nil, errors.Forbidden("insufficient permission to invite members")
	}

	if !IsRoleHigherOrEqual(inviter.Role, role) {
		return nil, errors.Forbidden("cannot invite member with role higher than yourself")
	}

	existing, _ := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, inviteeUserID)
	if existing != nil && existing.IsActive {
		return nil, errors.Conflict("user is already an active member of the workspace")
	}

	mp, err := NewMemberPermission(workspaceID, inviteeUserID, role, inviterID)
	if err != nil {
		return nil, err
	}

	if err := s.memberRepo.Save(ctx, mp); err != nil {
		return nil, err
	}

	return mp, nil
}

func (s *collaborationServiceImpl) AcceptInvitation(ctx context.Context, memberID, userID string) error {
	mp, err := s.memberRepo.FindByID(ctx, memberID)
	if err != nil {
		return err
	}
	if mp.UserID != userID {
		return errors.Forbidden("only the invited user can accept the invitation")
	}

	if err := mp.Accept(); err != nil {
		return err
	}

	ws, err := s.workspaceRepo.FindByID(ctx, mp.WorkspaceID)
	if err == nil {
		ws.IncrementMemberCount()
		_ = s.workspaceRepo.Save(ctx, ws)
	}

	return s.memberRepo.Save(ctx, mp)
}

func (s *collaborationServiceImpl) RemoveMember(ctx context.Context, workspaceID, removerID, targetUserID string) error {
	remover, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, removerID)
	if err != nil {
		return errors.Forbidden("remover is not a member of the workspace")
	}

	target, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, targetUserID)
	if err != nil {
		return errors.NotFound("target user is not a member of the workspace")
	}

	if removerID == targetUserID {
		return errors.Forbidden("cannot remove yourself from workspace")
	}

	allowed, _ := s.policy.CheckAccess(remover, ResourceWorkspace, ActionManageMembers)
	if !allowed {
		return errors.Forbidden("insufficient permission to remove members")
	}

	if !IsRoleHigherOrEqual(remover.Role, target.Role) {
		return errors.Forbidden("insufficient authority to remove this member")
	}

	if target.Role == RoleOwner {
		return errors.Forbidden("cannot remove the owner of the workspace")
	}

	if err := target.Deactivate(); err != nil {
		return err
	}

	ws, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err == nil {
		ws.DecrementMemberCount()
		_ = s.workspaceRepo.Save(ctx, ws)
	}

	return s.memberRepo.Save(ctx, target)
}

func (s *collaborationServiceImpl) ChangeMemberRole(ctx context.Context, workspaceID, changerID, targetUserID string, newRole Role) error {
	changer, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, changerID)
	if err != nil {
		return errors.Forbidden("changer is not a member of the workspace")
	}

	target, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, targetUserID)
	if err != nil {
		return errors.NotFound("target user is not a member of the workspace")
	}

	allowed, _ := s.policy.CheckAccess(changer, ResourceWorkspace, ActionManageMembers)
	if !allowed {
		return errors.Forbidden("insufficient permission to change member roles")
	}

	if err := target.ChangeRole(newRole, changer.Role); err != nil {
		return err
	}

	return s.memberRepo.Save(ctx, target)
}

func (s *collaborationServiceImpl) CheckMemberAccess(ctx context.Context, workspaceID, userID string, resource ResourceType, action Action) (bool, string, error) {
	mp, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		return false, "not a member", nil
	}

	allowed, reason := s.policy.CheckAccess(mp, resource, action)
	return allowed, reason, nil
}

func (s *collaborationServiceImpl) GetWorkspaceMembers(ctx context.Context, workspaceID, requesterID string) ([]*MemberPermission, error) {
	_, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, requesterID)
	if err != nil {
		return nil, errors.Forbidden("requester is not a member of the workspace")
	}

	return s.memberRepo.FindByWorkspaceID(ctx, workspaceID)
}

func (s *collaborationServiceImpl) GetUserWorkspaces(ctx context.Context, userID string) ([]*WorkspaceSummary, error) {
	workspaces, err := s.workspaceRepo.FindByMemberID(ctx, userID)
	if err != nil {
		return nil, err
	}

	summaries := make([]*WorkspaceSummary, 0, len(workspaces))
	for _, ws := range workspaces {
		mp, _ := s.memberRepo.FindByWorkspaceAndUser(ctx, ws.ID, userID)
		role := RoleViewer
		if mp != nil {
			role = mp.Role
		}

		summaries = append(summaries, &WorkspaceSummary{
			WorkspaceID:    ws.ID,
			Name:           ws.Name,
			Slug:           ws.Slug,
			MyRole:         role,
			MemberCount:    ws.MemberCount,
			PatentCount:    len(ws.PortfolioIDs),
			LastActivityAt: ws.UpdatedAt,
		})
	}
	return summaries, nil
}

func (s *collaborationServiceImpl) TransferOwnership(ctx context.Context, workspaceID, currentOwnerID, newOwnerID string) error {
	if currentOwnerID == newOwnerID {
		return errors.InvalidParam("cannot transfer ownership to current owner")
	}

	ws, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.OwnerID != currentOwnerID {
		return errors.Forbidden("only the current owner can transfer ownership")
	}

	newOwner, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, newOwnerID)
	if err != nil || !newOwner.IsActive {
		return errors.Forbidden("new owner must be an active member of the workspace")
	}

	oldOwner, _ := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, currentOwnerID)

	ws.TransferOwnership(newOwnerID)
	newOwner.Role = RoleOwner
	oldOwner.Role = RoleAdmin

	if err := s.workspaceRepo.Save(ctx, ws); err != nil {
		return err
	}
	if err := s.memberRepo.Save(ctx, newOwner); err != nil {
		return err
	}
	if err := s.memberRepo.Save(ctx, oldOwner); err != nil {
		return err
	}

	return nil
}

func (s *collaborationServiceImpl) UpdateWorkspace(ctx context.Context, workspaceID, requesterID, name, description string) error {
	requester, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, requesterID)
	if err != nil {
		return errors.Forbidden("requester is not a member of the workspace")
	}

	allowed, _ := s.policy.CheckAccess(requester, ResourceWorkspace, ActionUpdate)
	if !allowed {
		return errors.Forbidden("insufficient permission to update workspace")
	}

	ws, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err != nil {
		return err
	}

	if name != "" {
		if err := ws.UpdateName(name); err != nil {
			return err
		}
	}
	if description != "" {
		if err := ws.UpdateDescription(description); err != nil {
			return err
		}
	}

	return s.workspaceRepo.Save(ctx, ws)
}

func (s *collaborationServiceImpl) DeleteWorkspace(ctx context.Context, workspaceID, requesterID string) error {
	ws, err := s.workspaceRepo.FindByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.OwnerID != requesterID {
		return errors.Forbidden("only the owner can delete the workspace")
	}

	members, _ := s.memberRepo.FindByWorkspaceID(ctx, workspaceID)
	for _, m := range members {
		_ = m.Deactivate()
		_ = s.memberRepo.Save(ctx, m)
	}

	ws.MarkDeleted()
	return s.workspaceRepo.Save(ctx, ws)
}

func (s *collaborationServiceImpl) GrantCustomPermission(ctx context.Context, workspaceID, granterID, targetUserID string, perm *Permission) error {
	granter, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, granterID)
	if err != nil {
		return errors.Forbidden("granter is not a member of the workspace")
	}

	target, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, targetUserID)
	if err != nil {
		return errors.NotFound("target user is not a member of the workspace")
	}

	allowed, _ := s.policy.CheckAccess(granter, ResourceWorkspace, ActionManageMembers)
	if !allowed {
		return errors.Forbidden("insufficient permission to grant custom permissions")
	}

	// Cannot grant permission you do not possess
	granterHasPerm, _ := s.policy.CheckAccess(granter, perm.Resource, perm.Action)
	if !granterHasPerm {
		return errors.Forbidden("cannot grant permission you do not possess")
	}

	if err := target.AddCustomPermission(perm); err != nil {
		return err
	}

	return s.memberRepo.Save(ctx, target)
}

func (s *collaborationServiceImpl) RevokeCustomPermission(ctx context.Context, workspaceID, revokerID, targetUserID string, resource ResourceType, action Action) error {
	revoker, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, revokerID)
	if err != nil {
		return errors.Forbidden("revoker is not a member of the workspace")
	}

	target, err := s.memberRepo.FindByWorkspaceAndUser(ctx, workspaceID, targetUserID)
	if err != nil {
		return errors.NotFound("target user is not a member of the workspace")
	}

	allowed, _ := s.policy.CheckAccess(revoker, ResourceWorkspace, ActionManageMembers)
	if !allowed {
		return errors.Forbidden("insufficient permission to revoke custom permissions")
	}

	if err := target.RemoveCustomPermission(resource, action); err != nil {
		return err
	}

	return s.memberRepo.Save(ctx, target)
}

func (s *collaborationServiceImpl) GetWorkspaceActivity(ctx context.Context, workspaceID string, limit int) ([]*ActivityRecord, error) {
	if limit > 100 {
		limit = 100
	}
	// Placeholder since ActivityRepository is not defined in injection list
	return []*ActivityRecord{}, nil
}

// GenerateSlug converts a name to a URL-friendly slug.
func GenerateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	reg := regexp.MustCompile("[^a-z0-9-]")
	slug = reg.ReplaceAllString(slug, "")

	reg2 := regexp.MustCompile("-+")
	slug = reg2.ReplaceAllString(slug, "-")

	slug = strings.Trim(slug, "-")

	if len(slug) > 50 {
		slug = slug[:50]
		slug = strings.TrimSuffix(slug, "-")
	}
	return slug
}

// GenerateSlugWithSuffix adds a random 4-character suffix to a slug.
func GenerateSlugWithSuffix(slug string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 4)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return fmt.Sprintf("%s-%s", slug, string(b))
}

//Personal.AI order the ending
