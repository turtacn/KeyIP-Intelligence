package collaboration

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Role defines the privilege level of a user within a workspace.
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

// ResourceType defines the types of entities that can be protected.
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

// Action defines the operations that can be performed on resources.
type Action string

const (
	ActionCreate        Action = "create"
	ActionRead          Action = "read"
	ActionUpdate        Action = "update"
	ActionDelete        Action = "delete"
	ActionExport        Action = "export"
	ActionShare         Action = "share"
	ActionManageMembers Action = "manage_members"
	ActionManageSettings Action = "manage_settings"
	ActionAnalyze       Action = "analyze"
)

// Permission represents a single authorization grant.
type Permission struct {
	Resource   ResourceType      `json:"resource"`
	Action     Action            `json:"action"`
	Allowed    bool              `json:"allowed"`
	Conditions map[string]string `json:"conditions,omitempty"`
}

// RolePermissions defines the default permissions for a role.
type RolePermissions struct {
	Role         Role          `json:"role"`
	Permissions  []*Permission `json:"permissions"`
	Description  string        `json:"description"`
	IsSystemRole bool          `json:"is_system_role"`
}

// MemberPermission represents a user's membership and specific permissions in a workspace.
type MemberPermission struct {
	ID                string        `json:"id"`
	WorkspaceID       string        `json:"workspace_id"`
	UserID            string        `json:"user_id"`
	Role              Role          `json:"role"`
	CustomPermissions []*Permission `json:"custom_permissions,omitempty"`
	InvitedBy         string        `json:"invited_by"`
	InvitedAt         time.Time     `json:"invited_at"`
	AcceptedAt        *time.Time    `json:"accepted_at,omitempty"`
	IsActive          bool          `json:"is_active"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

// PermissionPolicy defines the interface for checking permissions.
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

// NewPermissionPolicy initializes the permission matrix for all roles.
func NewPermissionPolicy() PermissionPolicy {
	p := &permissionPolicyImpl{
		rolePermissions: make(map[Role]*RolePermissions),
	}
	p.initRoles()
	return p
}

func (p *permissionPolicyImpl) initRoles() {
	perm := func(res ResourceType, act Action) *Permission {
		return &Permission{Resource: res, Action: act, Allowed: true}
	}
	permCond := func(res ResourceType, act Action, conditions map[string]string) *Permission {
		return &Permission{Resource: res, Action: act, Allowed: true, Conditions: conditions}
	}

	allResources := []ResourceType{
		ResourcePatent, ResourcePortfolio, ResourceLifecycle,
		ResourceAnnuity, ResourceDeadline, ResourceWorkspace,
		ResourceReport, ResourceSettings,
	}
	allActions := []Action{
		ActionCreate, ActionRead, ActionUpdate, ActionDelete,
		ActionExport, ActionShare, ActionManageMembers,
		ActionManageSettings, ActionAnalyze,
	}

	// Owner: 所有资源所有操作均允许
	ownerPerms := []*Permission{}
	for _, res := range allResources {
		for _, act := range allActions {
			ownerPerms = append(ownerPerms, perm(res, act))
		}
	}
	p.rolePermissions[RoleOwner] = &RolePermissions{
		Role:         RoleOwner,
		Permissions:  ownerPerms,
		Description:  "Owner - full access",
		IsSystemRole: true,
	}

	// Admin: 除 ManageSettings 外所有操作均允许
	adminPerms := []*Permission{}
	for _, res := range allResources {
		for _, act := range allActions {
			if act == ActionManageSettings {
				continue
			}
			adminPerms = append(adminPerms, perm(res, act))
		}
	}
	p.rolePermissions[RoleAdmin] = &RolePermissions{
		Role:         RoleAdmin,
		Permissions:  adminPerms,
		Description:  "Admin - full access except system settings",
		IsSystemRole: true,
	}

	// Manager: Patent/Portfolio/Lifecycle/Annuity/Deadline 的 CRUD + Analyze，不可 ManageMembers/ManageSettings
	managerResources := []ResourceType{ResourcePatent, ResourcePortfolio, ResourceLifecycle, ResourceAnnuity, ResourceDeadline}
	managerPerms := []*Permission{}
	for _, res := range managerResources {
		managerPerms = append(managerPerms, perm(res, ActionCreate))
		managerPerms = append(managerPerms, perm(res, ActionRead))
		managerPerms = append(managerPerms, perm(res, ActionUpdate))
		managerPerms = append(managerPerms, perm(res, ActionDelete))
		managerPerms = append(managerPerms, perm(res, ActionAnalyze))
	}
	managerPerms = append(managerPerms, perm(ResourceReport, ActionRead))
	managerPerms = append(managerPerms, perm(ResourceReport, ActionExport))
	managerPerms = append(managerPerms, perm(ResourceWorkspace, ActionRead))

	p.rolePermissions[RoleManager] = &RolePermissions{
		Role:         RoleManager,
		Permissions:  managerPerms,
		Description:  "Manager - manage IP assets",
		IsSystemRole: true,
	}

	// Attorney: Patent 的 CRUD + Analyze，Lifecycle/Deadline 的 Read/Update，不可 Delete Portfolio
	attorneyPerms := []*Permission{}
	for _, act := range []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionAnalyze} {
		attorneyPerms = append(attorneyPerms, perm(ResourcePatent, act))
	}
	for _, res := range []ResourceType{ResourceLifecycle, ResourceDeadline} {
		attorneyPerms = append(attorneyPerms, perm(res, ActionRead))
		attorneyPerms = append(attorneyPerms, perm(res, ActionUpdate))
	}
	attorneyPerms = append(attorneyPerms, perm(ResourcePortfolio, ActionRead))
	attorneyPerms = append(attorneyPerms, perm(ResourceAnnuity, ActionRead))
	attorneyPerms = append(attorneyPerms, perm(ResourceWorkspace, ActionRead))

	p.rolePermissions[RoleAttorney] = &RolePermissions{
		Role:         RoleAttorney,
		Permissions:  attorneyPerms,
		Description:  "Attorney - handle patent prosecution",
		IsSystemRole: true,
	}

	// Analyst：所有资源的 Read + Analyze + Export，不可 Create/Update/Delete
	analystPerms := []*Permission{}
	for _, res := range allResources {
		analystPerms = append(analystPerms, perm(res, ActionRead))
		analystPerms = append(analystPerms, perm(res, ActionAnalyze))
		analystPerms = append(analystPerms, perm(res, ActionExport))
	}
	p.rolePermissions[RoleAnalyst] = &RolePermissions{
		Role:         RoleAnalyst,
		Permissions:  analystPerms,
		Description:  "Analyst - read and analyze data",
		IsSystemRole: true,
	}

	// Viewer：所有资源的 Read，不可其他操作
	viewerPerms := []*Permission{}
	for _, res := range allResources {
		viewerPerms = append(viewerPerms, perm(res, ActionRead))
	}
	p.rolePermissions[RoleViewer] = &RolePermissions{
		Role:         RoleViewer,
		Permissions:  viewerPerms,
		Description:  "Viewer - read-only access",
		IsSystemRole: true,
	}

	// Inventor：Patent 的 Read（own_only）+ Create（发明披露），Deadline 的 Read（own_only）
	inventorPerms := []*Permission{
		permCond(ResourcePatent, ActionRead, map[string]string{"own_only": "true"}),
		perm(ResourcePatent, ActionCreate),
		permCond(ResourceDeadline, ActionRead, map[string]string{"own_only": "true"}),
		perm(ResourceWorkspace, ActionRead),
	}
	p.rolePermissions[RoleInventor] = &RolePermissions{
		Role:         RoleInventor,
		Permissions:  inventorPerms,
		Description:  "Inventor - view own patents and submit disclosures",
		IsSystemRole: true,
	}
}

func (p *permissionPolicyImpl) HasPermission(role Role, resource ResourceType, action Action) bool {
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
		return nil, errors.NotFound("role not found")
	}
	return rp, nil
}

func (p *permissionPolicyImpl) ListRoles() []*RolePermissions {
	roles := []*RolePermissions{
		p.rolePermissions[RoleOwner],
		p.rolePermissions[RoleAdmin],
		p.rolePermissions[RoleManager],
		p.rolePermissions[RoleAttorney],
		p.rolePermissions[RoleAnalyst],
		p.rolePermissions[RoleViewer],
		p.rolePermissions[RoleInventor],
	}
	return roles
}

func (p *permissionPolicyImpl) CheckAccess(member *MemberPermission, resource ResourceType, action Action) (bool, string) {
	if !member.IsActive {
		return false, "member is inactive"
	}
	if member.AcceptedAt == nil {
		return false, "pending invitation"
	}

	// Check custom permissions first (priority)
	for _, cp := range member.CustomPermissions {
		if cp.Resource == resource && cp.Action == action {
			if cp.Allowed {
				return true, ""
			}
			return false, "insufficient permission"
		}
	}

	// Check role defaults
	if p.HasPermission(member.Role, resource, action) {
		return true, ""
	}

	return false, "insufficient permission"
}

func (p *permissionPolicyImpl) GetEffectivePermissions(member *MemberPermission) []*Permission {
	rp, _ := p.GetRolePermissions(member.Role)
	effective := make(map[string]*Permission)

	// Add role permissions
	if rp != nil {
		for _, perm := range rp.Permissions {
			key := fmt.Sprintf("%s:%s", perm.Resource, perm.Action)
			effective[key] = perm
		}
	}

	// Override with custom permissions
	for _, perm := range member.CustomPermissions {
		key := fmt.Sprintf("%s:%s", perm.Resource, perm.Action)
		effective[key] = perm
	}

	result := make([]*Permission, 0, len(effective))
	for _, perm := range effective {
		result = append(result, perm)
	}
	return result
}

// IsRoleHigherOrEqual compares two roles.
func IsRoleHigherOrEqual(role1, role2 Role) bool {
	weights := map[Role]int{
		RoleOwner:    7,
		RoleAdmin:    6,
		RoleManager:  5,
		RoleAttorney: 4,
		RoleAnalyst:  3,
		RoleViewer:   2,
		RoleInventor: 1,
	}
	return weights[role1] >= weights[role2]
}

// ValidateRoleTransition ensures a role transition is valid.
func ValidateRoleTransition(currentRole, targetRole Role) error {
	if !IsRoleHigherOrEqual(currentRole, targetRole) {
		return errors.Forbidden("cannot escalate to role higher than yourself")
	}
	return nil
}

// NewMemberPermission creates a new member permission.
func NewMemberPermission(workspaceID, userID string, role Role, invitedBy string) (*MemberPermission, error) {
	if workspaceID == "" || userID == "" || invitedBy == "" {
		return nil, errors.InvalidParam("workspaceID, userID and invitedBy are required")
	}

	validRoles := map[Role]bool{
		RoleOwner: true, RoleAdmin: true, RoleManager: true,
		RoleAttorney: true, RoleAnalyst: true, RoleViewer: true, RoleInventor: true,
	}
	if !validRoles[role] {
		return nil, errors.InvalidParam("invalid role")
	}

	now := time.Now().UTC()
	return &MemberPermission{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
		InvitedBy:   invitedBy,
		InvitedAt:   now,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Accept marks the invitation as accepted.
func (mp *MemberPermission) Accept() error {
	if mp.AcceptedAt != nil {
		return errors.InvalidState("invitation already accepted")
	}
	now := time.Now().UTC()
	mp.AcceptedAt = &now
	mp.UpdatedAt = now
	return nil
}

// Deactivate deactivates the member.
func (mp *MemberPermission) Deactivate() error {
	mp.IsActive = false
	mp.UpdatedAt = time.Now().UTC()
	return nil
}

// ChangeRole changes the member's role.
func (mp *MemberPermission) ChangeRole(newRole Role, changerRole Role) error {
	if !IsRoleHigherOrEqual(changerRole, mp.Role) {
		return errors.Forbidden("insufficient authority to change this member's role")
	}
	if !IsRoleHigherOrEqual(changerRole, newRole) {
		return errors.Forbidden("cannot promote member to role higher than yourself")
	}
	if mp.Role == RoleOwner && changerRole != RoleOwner {
		return errors.Forbidden("only another Owner can demote an Owner")
	}

	mp.Role = newRole
	mp.UpdatedAt = time.Now().UTC()
	return nil
}

// AddCustomPermission adds a custom permission.
func (mp *MemberPermission) AddCustomPermission(perm *Permission) error {
	for _, cp := range mp.CustomPermissions {
		if cp.Resource == perm.Resource && cp.Action == perm.Action {
			return errors.Conflict(fmt.Sprintf("custom permission already exists: %s:%s", perm.Resource, perm.Action))
		}
	}
	mp.CustomPermissions = append(mp.CustomPermissions, perm)
	mp.UpdatedAt = time.Now().UTC()
	return nil
}

// RemoveCustomPermission removes a custom permission.
func (mp *MemberPermission) RemoveCustomPermission(resource ResourceType, action Action) error {
	for i, cp := range mp.CustomPermissions {
		if cp.Resource == resource && cp.Action == action {
			mp.CustomPermissions = append(mp.CustomPermissions[:i], mp.CustomPermissions[i+1:]...)
			mp.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return errors.NotFound("custom permission not found")
}

// Validate validates the member permission.
func (mp *MemberPermission) Validate() error {
	if mp.ID == "" {
		return errors.InvalidParam("ID is required")
	}
	if mp.WorkspaceID == "" {
		return errors.InvalidParam("WorkspaceID is required")
	}
	if mp.UserID == "" {
		return errors.InvalidParam("UserID is required")
	}
	validRoles := map[Role]bool{
		RoleOwner: true, RoleAdmin: true, RoleManager: true,
		RoleAttorney: true, RoleAnalyst: true, RoleViewer: true, RoleInventor: true,
	}
	if !validRoles[mp.Role] {
		return errors.InvalidParam("invalid role")
	}
	return nil
}

//Personal.AI order the ending
