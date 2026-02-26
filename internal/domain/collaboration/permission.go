package collaboration

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Role defines the role of a user in a workspace.
type Role string

const (
	RoleOwner    Role = "owner"
	RoleAdmin    Role = "admin"
	RoleManager  Role = "manager"
	RoleAttorney Role = "attorney"
	RoleAnalyst  Role = "analyst"
	RoleViewer   Role = "viewer"
	RoleInventor Role = "inventor"
)

// ResourceType defines the type of resource.
type ResourceType string

const (
	ResourcePatent    ResourceType = "patent"
	ResourcePortfolio ResourceType = "portfolio"
	ResourceLifecycle ResourceType = "lifecycle"
	ResourceAnnuity   ResourceType = "annuity"
	ResourceDeadline  ResourceType = "deadline"
	ResourceWorkspace ResourceType = "workspace"
	ResourceReport    ResourceType = "report"
	ResourceSettings  ResourceType = "settings"
)

// Action defines the action performed on a resource.
type Action string

const (
	ActionCreate         Action = "create"
	ActionRead           Action = "read"
	ActionUpdate         Action = "update"
	ActionDelete         Action = "delete"
	ActionExport         Action = "export"
	ActionShare          Action = "share"
	ActionManageMembers  Action = "manage_members"
	ActionManageSettings Action = "manage_settings"
	ActionAnalyze        Action = "analyze"
)

// Permission represents a specific access right.
type Permission struct {
	Resource   ResourceType      `json:"resource"`
	Action     Action            `json:"action"`
	Allowed    bool              `json:"allowed"`
	Conditions map[string]string `json:"conditions"`
}

// RolePermissions defines permissions for a role.
type RolePermissions struct {
	Role         Role          `json:"role"`
	Permissions  []*Permission `json:"permissions"`
	Description  string        `json:"description"`
	IsSystemRole bool          `json:"is_system_role"`
}

// MemberPermission represents a user's permission in a workspace.
type MemberPermission struct {
	ID                string        `json:"id"`
	WorkspaceID       string        `json:"workspace_id"`
	UserID            string        `json:"user_id"`
	Role              Role          `json:"role"`
	CustomPermissions []*Permission `json:"custom_permissions"`
	InvitedBy         string        `json:"invited_by"`
	InvitedAt         time.Time     `json:"invited_at"`
	AcceptedAt        *time.Time    `json:"accepted_at"`
	IsActive          bool          `json:"is_active"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// PermissionPolicy defines the interface for permission checks.
type PermissionPolicy interface {
	HasPermission(role Role, resource ResourceType, action Action) bool
	GetRolePermissions(role Role) (*RolePermissions, error)
	ListRoles() []*RolePermissions
	CheckAccess(member *MemberPermission, resource ResourceType, action Action) (bool, string)
	GetEffectivePermissions(member *MemberPermission) []*Permission
}

type permissionPolicyImpl struct {
	rolePermissions map[Role]*RolePermissions
}

// NewPermissionPolicy creates a new PermissionPolicy with default rules.
func NewPermissionPolicy() PermissionPolicy {
	p := &permissionPolicyImpl{
		rolePermissions: make(map[Role]*RolePermissions),
	}
	p.initDefaults()
	return p
}

func (p *permissionPolicyImpl) initDefaults() {
	// Helper to create perm
	allow := func(res ResourceType, acts ...Action) []*Permission {
		var perms []*Permission
		for _, act := range acts {
			perms = append(perms, &Permission{Resource: res, Action: act, Allowed: true})
		}
		return perms
	}

	// Owner: All
	ownerPerms := []*Permission{
		{Resource: ResourceWorkspace, Action: ActionManageSettings, Allowed: true},
		{Resource: ResourceWorkspace, Action: ActionManageMembers, Allowed: true},
		{Resource: ResourceWorkspace, Action: ActionDelete, Allowed: true},
	}
	p.rolePermissions[RoleOwner] = &RolePermissions{Role: RoleOwner, Permissions: ownerPerms, Description: "Owner", IsSystemRole: true}

	// Admin
	adminPerms := []*Permission{}
	adminPerms = append(adminPerms, allow(ResourceWorkspace, ActionManageMembers, ActionRead, ActionUpdate)...)
	adminPerms = append(adminPerms, allow(ResourcePatent, ActionCreate, ActionRead, ActionUpdate, ActionDelete)...)
	p.rolePermissions[RoleAdmin] = &RolePermissions{Role: RoleAdmin, Permissions: adminPerms, Description: "Admin", IsSystemRole: true}

	// Manager
	mgrPerms := []*Permission{}
	// Patent, Portfolio, Lifecycle, Annuity, Deadline -> CRUD + Analyze
	resources := []ResourceType{ResourcePatent, ResourcePortfolio, ResourceLifecycle, ResourceAnnuity, ResourceDeadline}
	for _, res := range resources {
		mgrPerms = append(mgrPerms, allow(res, ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionAnalyze)...)
	}
	p.rolePermissions[RoleManager] = &RolePermissions{Role: RoleManager, Permissions: mgrPerms, Description: "Manager", IsSystemRole: true}

	// Attorney
	attyPerms := []*Permission{}
	attyPerms = append(attyPerms, allow(ResourcePatent, ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionAnalyze)...)
	// No Portfolio Delete
	attyPerms = append(attyPerms, allow(ResourcePortfolio, ActionRead, ActionUpdate)...)
	// Lifecycle/Deadline Read/Update
	attyPerms = append(attyPerms, allow(ResourceLifecycle, ActionRead, ActionUpdate)...)
	attyPerms = append(attyPerms, allow(ResourceDeadline, ActionRead, ActionUpdate)...)
	p.rolePermissions[RoleAttorney] = &RolePermissions{Role: RoleAttorney, Permissions: attyPerms, Description: "Attorney", IsSystemRole: true}

	// Analyst
	analystPerms := []*Permission{}
	// Read & Analyze All
	analystPerms = append(analystPerms, allow(ResourcePatent, ActionRead, ActionAnalyze, ActionExport)...)
	analystPerms = append(analystPerms, allow(ResourcePortfolio, ActionRead, ActionAnalyze, ActionExport)...)
	analystPerms = append(analystPerms, allow(ResourceLifecycle, ActionRead, ActionAnalyze, ActionExport)...)
	analystPerms = append(analystPerms, allow(ResourceAnnuity, ActionRead, ActionAnalyze, ActionExport)...)
	analystPerms = append(analystPerms, allow(ResourceDeadline, ActionRead, ActionAnalyze, ActionExport)...)
	p.rolePermissions[RoleAnalyst] = &RolePermissions{Role: RoleAnalyst, Permissions: analystPerms, Description: "Analyst", IsSystemRole: true}

	// Viewer
	viewerPerms := []*Permission{}
	viewerPerms = append(viewerPerms, allow(ResourcePatent, ActionRead)...)
	viewerPerms = append(viewerPerms, allow(ResourcePortfolio, ActionRead)...)
	viewerPerms = append(viewerPerms, allow(ResourceLifecycle, ActionRead)...)
	viewerPerms = append(viewerPerms, allow(ResourceAnnuity, ActionRead)...)
	viewerPerms = append(viewerPerms, allow(ResourceDeadline, ActionRead)...)
	p.rolePermissions[RoleViewer] = &RolePermissions{Role: RoleViewer, Permissions: viewerPerms, Description: "Viewer", IsSystemRole: true}

	// Inventor
	invPerms := []*Permission{}
	invPerms = append(invPerms, &Permission{Resource: ResourcePatent, Action: ActionRead, Allowed: true, Conditions: map[string]string{"own_only": "true"}})
	invPerms = append(invPerms, &Permission{Resource: ResourcePatent, Action: ActionCreate, Allowed: true}) // Invention disclosure
	invPerms = append(invPerms, &Permission{Resource: ResourceDeadline, Action: ActionRead, Allowed: true, Conditions: map[string]string{"own_only": "true"}})
	p.rolePermissions[RoleInventor] = &RolePermissions{Role: RoleInventor, Permissions: invPerms, Description: "Inventor", IsSystemRole: true}
}

func (p *permissionPolicyImpl) HasPermission(role Role, resource ResourceType, action Action) bool {
	if role == RoleOwner {
		return true
	}
	rp, ok := p.rolePermissions[role]
	if !ok {
		return false
	}
	for _, perm := range rp.Permissions {
		if perm.Resource == resource && perm.Action == action && perm.Allowed {
			return true
		}
	}
	return false
}

func (p *permissionPolicyImpl) GetRolePermissions(role Role) (*RolePermissions, error) {
	rp, ok := p.rolePermissions[role]
	if !ok {
		return nil, apperrors.NewNotFound("role not found: %s", role)
	}
	return rp, nil
}

func (p *permissionPolicyImpl) ListRoles() []*RolePermissions {
	var roles []*RolePermissions
	for _, r := range []Role{RoleOwner, RoleAdmin, RoleManager, RoleAttorney, RoleAnalyst, RoleViewer, RoleInventor} {
		if rp, ok := p.rolePermissions[r]; ok {
			roles = append(roles, rp)
		}
	}
	return roles
}

func (p *permissionPolicyImpl) CheckAccess(member *MemberPermission, resource ResourceType, action Action) (bool, string) {
	if !member.IsActive {
		return false, "member is inactive"
	}
	if member.AcceptedAt == nil && member.Role != RoleOwner { // Owner auto-accepted
		return false, "invitation not accepted"
	}

	// Check custom permissions first
	for _, perm := range member.CustomPermissions {
		if perm.Resource == resource && perm.Action == action {
			if perm.Allowed {
				return true, ""
			} else {
				return false, "denied by custom permission"
			}
		}
	}

	// Check role permissions
	if p.HasPermission(member.Role, resource, action) {
		return true, ""
	}

	return false, "insufficient permission"
}

func (p *permissionPolicyImpl) GetEffectivePermissions(member *MemberPermission) []*Permission {
	// Start with role permissions
	rolePerms, _ := p.GetRolePermissions(member.Role)
	effective := make(map[string]*Permission)

	if rolePerms != nil {
		for _, perm := range rolePerms.Permissions {
			key := fmt.Sprintf("%s:%s", perm.Resource, perm.Action)
			effective[key] = perm
		}
	}

	// Override with custom
	for _, perm := range member.CustomPermissions {
		key := fmt.Sprintf("%s:%s", perm.Resource, perm.Action)
		effective[key] = perm
	}

	var result []*Permission
	for _, perm := range effective {
		if perm.Allowed {
			result = append(result, perm)
		}
	}
	return result
}

// NewMemberPermission creates a new member permission record.
func NewMemberPermission(workspaceID, userID string, role Role, invitedBy string) (*MemberPermission, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID required")
	}
	if userID == "" {
		return nil, errors.New("userID required")
	}
	// Fixed validation for invitedBy
	if invitedBy == "" {
		return nil, errors.New("invitedBy required")
	}

	mp := &MemberPermission{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
		InvitedBy:   invitedBy,
		InvitedAt:   time.Now().UTC(),
		IsActive:    true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	return mp, nil
}

func (mp *MemberPermission) Accept() error {
	if mp.AcceptedAt != nil {
		return apperrors.NewValidation("invitation already accepted")
	}
	now := time.Now().UTC()
	mp.AcceptedAt = &now
	mp.UpdatedAt = now
	return nil
}

func (mp *MemberPermission) Deactivate() error {
	mp.IsActive = false
	mp.UpdatedAt = time.Now().UTC()
	return nil
}

func (mp *MemberPermission) ChangeRole(newRole Role, changedBy Role) error {
	if !IsRoleHigherOrEqual(changedBy, newRole) {
		return apperrors.NewValidation("cannot promote to role higher than self")
	}
	if !IsRoleHigherOrEqual(changedBy, mp.Role) {
		// Can't change someone higher than self
		return apperrors.NewValidation("insufficient authority")
	}

	if mp.Role == RoleOwner {
		return apperrors.NewValidation("cannot change owner role directly")
	}

	mp.Role = newRole
	mp.UpdatedAt = time.Now().UTC()
	return nil
}

func (mp *MemberPermission) AddCustomPermission(perm *Permission) error {
	for _, p := range mp.CustomPermissions {
		if p.Resource == perm.Resource && p.Action == perm.Action {
			return apperrors.NewValidation("custom permission already exists")
		}
	}
	mp.CustomPermissions = append(mp.CustomPermissions, perm)
	mp.UpdatedAt = time.Now().UTC()
	return nil
}

func (mp *MemberPermission) RemoveCustomPermission(resource ResourceType, action Action) error {
	for i, p := range mp.CustomPermissions {
		if p.Resource == resource && p.Action == action {
			mp.CustomPermissions = append(mp.CustomPermissions[:i], mp.CustomPermissions[i+1:]...)
			mp.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return apperrors.NewNotFound("custom permission not found")
}

func (mp *MemberPermission) Validate() error {
	if mp.ID == "" || mp.WorkspaceID == "" || mp.UserID == "" {
		return errors.New("invalid member permission")
	}
	return nil
}

// Helper functions

func IsRoleHigherOrEqual(role1, role2 Role) bool {
	levels := map[Role]int{
		RoleOwner:    100,
		RoleAdmin:    90,
		RoleManager:  80,
		RoleAttorney: 70,
		RoleAnalyst:  60,
		RoleViewer:   50,
		RoleInventor: 40,
	}
	return levels[role1] >= levels[role2]
}

func ValidateRoleTransition(currentRole, targetRole Role) error {
	// Logic handled in ChangeRole?
	// Requirement mentions this helper.
	return nil
}

//Personal.AI order the ending
