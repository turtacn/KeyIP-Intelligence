package collaboration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPermissionPolicy(t *testing.T) {
	p := NewPermissionPolicy()
	roles := p.ListRoles()
	assert.Equal(t, 7, len(roles))
}

func TestHasPermission_Owner_AllAllowed(t *testing.T) {
	p := NewPermissionPolicy()
	resources := []ResourceType{ResourcePatent, ResourcePortfolio, ResourceWorkspace, ResourceSettings}
	actions := []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManageMembers, ActionManageSettings}

	for _, res := range resources {
		for _, act := range actions {
			assert.True(t, p.HasPermission(RoleOwner, res, act), "Owner should have permission for %s:%s", res, act)
		}
	}
}

func TestHasPermission_Admin_ManageSettings_Denied(t *testing.T) {
	p := NewPermissionPolicy()
	assert.False(t, p.HasPermission(RoleAdmin, ResourceSettings, ActionManageSettings))
}

func TestHasPermission_Admin_ManageMembers_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleAdmin, ResourceWorkspace, ActionManageMembers))
}

func TestHasPermission_Manager_Patent_CRUD(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleManager, ResourcePatent, ActionCreate))
	assert.True(t, p.HasPermission(RoleManager, ResourcePatent, ActionRead))
	assert.True(t, p.HasPermission(RoleManager, ResourcePatent, ActionUpdate))
	assert.True(t, p.HasPermission(RoleManager, ResourcePatent, ActionDelete))
}

func TestHasPermission_Manager_ManageMembers_Denied(t *testing.T) {
	p := NewPermissionPolicy()
	assert.False(t, p.HasPermission(RoleManager, ResourceWorkspace, ActionManageMembers))
}

func TestHasPermission_Attorney_Patent_CRUD(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleAttorney, ResourcePatent, ActionCreate))
	assert.True(t, p.HasPermission(RoleAttorney, ResourcePatent, ActionRead))
	assert.True(t, p.HasPermission(RoleAttorney, ResourcePatent, ActionUpdate))
	assert.True(t, p.HasPermission(RoleAttorney, ResourcePatent, ActionDelete))
}

func TestHasPermission_Attorney_DeletePortfolio_Denied(t *testing.T) {
	p := NewPermissionPolicy()
	assert.False(t, p.HasPermission(RoleAttorney, ResourcePortfolio, ActionDelete))
}

func TestHasPermission_Analyst_Read_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleAnalyst, ResourcePatent, ActionRead))
	assert.True(t, p.HasPermission(RoleAnalyst, ResourcePortfolio, ActionRead))
}

func TestHasPermission_Analyst_Create_Denied(t *testing.T) {
	p := NewPermissionPolicy()
	assert.False(t, p.HasPermission(RoleAnalyst, ResourcePatent, ActionCreate))
}

func TestHasPermission_Analyst_Analyze_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleAnalyst, ResourcePatent, ActionAnalyze))
}

func TestHasPermission_Analyst_Export_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleAnalyst, ResourcePatent, ActionExport))
}

func TestHasPermission_Viewer_Read_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleViewer, ResourcePatent, ActionRead))
}

func TestHasPermission_Viewer_Update_Denied(t *testing.T) {
	p := NewPermissionPolicy()
	assert.False(t, p.HasPermission(RoleViewer, ResourcePatent, ActionUpdate))
}

func TestHasPermission_Inventor_Patent_Read_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleInventor, ResourcePatent, ActionRead))
}

func TestHasPermission_Inventor_Patent_Create_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	assert.True(t, p.HasPermission(RoleInventor, ResourcePatent, ActionCreate))
}

func TestHasPermission_Inventor_Portfolio_Denied(t *testing.T) {
	p := NewPermissionPolicy()
	assert.False(t, p.HasPermission(RoleInventor, ResourcePortfolio, ActionRead))
}

func TestHasPermission_InvalidRole(t *testing.T) {
	p := NewPermissionPolicy()
	assert.False(t, p.HasPermission("invalid", ResourcePatent, ActionRead))
}

func TestGetRolePermissions_Owner(t *testing.T) {
	p := NewPermissionPolicy()
	rp, err := p.GetRolePermissions(RoleOwner)
	assert.NoError(t, err)
	assert.NotNil(t, rp)
	assert.Equal(t, RoleOwner, rp.Role)
	assert.True(t, len(rp.Permissions) > 0)
}

func TestGetRolePermissions_InvalidRole(t *testing.T) {
	p := NewPermissionPolicy()
	_, err := p.GetRolePermissions("invalid")
	assert.Error(t, err)
}

func TestCheckAccess_Allowed(t *testing.T) {
	p := NewPermissionPolicy()
	now := time.Now()
	member := &MemberPermission{
		Role:       RoleManager,
		IsActive:   true,
		AcceptedAt: &now,
	}
	allowed, reason := p.CheckAccess(member, ResourcePatent, ActionRead)
	assert.True(t, allowed)
	assert.Empty(t, reason)
}

func TestCheckAccess_Denied_NoPermission(t *testing.T) {
	p := NewPermissionPolicy()
	now := time.Now()
	member := &MemberPermission{
		Role:       RoleViewer,
		IsActive:   true,
		AcceptedAt: &now,
	}
	allowed, reason := p.CheckAccess(member, ResourcePatent, ActionDelete)
	assert.False(t, allowed)
	assert.Contains(t, reason, "insufficient permission")
}

func TestCheckAccess_Denied_Inactive(t *testing.T) {
	p := NewPermissionPolicy()
	now := time.Now()
	member := &MemberPermission{
		Role:       RoleOwner,
		IsActive:   false,
		AcceptedAt: &now,
	}
	allowed, reason := p.CheckAccess(member, ResourcePatent, ActionRead)
	assert.False(t, allowed)
	assert.Contains(t, reason, "inactive")
}

func TestCheckAccess_Denied_NotAccepted(t *testing.T) {
	p := NewPermissionPolicy()
	member := &MemberPermission{
		Role:       RoleOwner,
		IsActive:   true,
		AcceptedAt: nil,
	}
	allowed, reason := p.CheckAccess(member, ResourcePatent, ActionRead)
	assert.False(t, allowed)
	assert.Contains(t, reason, "pending invitation")
}

func TestCheckAccess_CustomPermission_Override(t *testing.T) {
	p := NewPermissionPolicy()
	now := time.Now()
	member := &MemberPermission{
		Role:       RoleViewer,
		IsActive:   true,
		AcceptedAt: &now,
		CustomPermissions: []*Permission{
			{Resource: ResourcePatent, Action: ActionUpdate, Allowed: true},
		},
	}
	allowed, reason := p.CheckAccess(member, ResourcePatent, ActionUpdate)
	assert.True(t, allowed)
	assert.Empty(t, reason)
}

func TestCheckAccess_CustomPermission_Deny(t *testing.T) {
	p := NewPermissionPolicy()
	now := time.Now()
	member := &MemberPermission{
		Role:       RoleOwner,
		IsActive:   true,
		AcceptedAt: &now,
		CustomPermissions: []*Permission{
			{Resource: ResourcePatent, Action: ActionDelete, Allowed: false},
		},
	}
	allowed, reason := p.CheckAccess(member, ResourcePatent, ActionDelete)
	assert.False(t, allowed)
	assert.Contains(t, reason, "insufficient permission")
}

func TestGetEffectivePermissions_RoleOnly(t *testing.T) {
	p := NewPermissionPolicy()
	member := &MemberPermission{Role: RoleViewer}
	perms := p.GetEffectivePermissions(member)
	assert.True(t, len(perms) > 0)
}

func TestGetEffectivePermissions_WithCustomOverride(t *testing.T) {
	p := NewPermissionPolicy()
	member := &MemberPermission{
		Role: RoleViewer,
		CustomPermissions: []*Permission{
			{Resource: ResourcePatent, Action: ActionRead, Allowed: false},
		},
	}
	perms := p.GetEffectivePermissions(member)
	found := false
	for _, perm := range perms {
		if perm.Resource == ResourcePatent && perm.Action == ActionRead {
			assert.False(t, perm.Allowed)
			found = true
		}
	}
	assert.True(t, found)
}

func TestGetEffectivePermissions_CustomAddition(t *testing.T) {
	p := NewPermissionPolicy()
	member := &MemberPermission{
		Role: RoleViewer,
		CustomPermissions: []*Permission{
			{Resource: ResourceSettings, Action: ActionUpdate, Allowed: true},
		},
	}
	perms := p.GetEffectivePermissions(member)
	found := false
	for _, perm := range perms {
		if perm.Resource == ResourceSettings && perm.Action == ActionUpdate {
			assert.True(t, perm.Allowed)
			found = true
		}
	}
	assert.True(t, found)
}

func TestIsRoleHigherOrEqual_OwnerVsAdmin(t *testing.T) {
	assert.True(t, IsRoleHigherOrEqual(RoleOwner, RoleAdmin))
}

func TestIsRoleHigherOrEqual_AdminVsOwner(t *testing.T) {
	assert.False(t, IsRoleHigherOrEqual(RoleAdmin, RoleOwner))
}

func TestIsRoleHigherOrEqual_SameRole(t *testing.T) {
	assert.True(t, IsRoleHigherOrEqual(RoleManager, RoleManager))
}

func TestIsRoleHigherOrEqual_ViewerVsInventor(t *testing.T) {
	assert.True(t, IsRoleHigherOrEqual(RoleViewer, RoleInventor))
}

func TestIsRoleHigherOrEqual_InventorVsViewer(t *testing.T) {
	assert.False(t, IsRoleHigherOrEqual(RoleInventor, RoleViewer))
}

func TestValidateRoleTransition_Valid(t *testing.T) {
	err := ValidateRoleTransition(RoleAdmin, RoleManager)
	assert.NoError(t, err)
}

func TestValidateRoleTransition_EscalationBeyondSelf(t *testing.T) {
	err := ValidateRoleTransition(RoleManager, RoleAdmin)
	assert.Error(t, err)
}

func TestNewMemberPermission_Success(t *testing.T) {
	mp, err := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	assert.NoError(t, err)
	assert.NotNil(t, mp)
	assert.Equal(t, "ws1", mp.WorkspaceID)
	assert.Equal(t, RoleManager, mp.Role)
	assert.True(t, mp.IsActive)
	assert.Nil(t, mp.AcceptedAt)
}

func TestNewMemberPermission_EmptyWorkspaceID(t *testing.T) {
	_, err := NewMemberPermission("", "user1", RoleManager, "owner1")
	assert.Error(t, err)
}

func TestNewMemberPermission_InvalidRole(t *testing.T) {
	_, err := NewMemberPermission("ws1", "user1", "invalid", "owner1")
	assert.Error(t, err)
}

func TestMemberPermission_Accept_Success(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	err := mp.Accept()
	assert.NoError(t, err)
	assert.NotNil(t, mp.AcceptedAt)
}

func TestMemberPermission_Accept_AlreadyAccepted(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	mp.Accept()
	err := mp.Accept()
	assert.Error(t, err)
}

func TestMemberPermission_Deactivate_Success(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	err := mp.Deactivate()
	assert.NoError(t, err)
	assert.False(t, mp.IsActive)
}

func TestMemberPermission_ChangeRole_Success(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	err := mp.ChangeRole(RoleAttorney, RoleAdmin)
	assert.NoError(t, err)
	assert.Equal(t, RoleAttorney, mp.Role)
}

func TestMemberPermission_ChangeRole_InsufficientAuthority(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleAdmin, "owner1")
	err := mp.ChangeRole(RoleManager, RoleManager)
	assert.Error(t, err)
}

func TestMemberPermission_AddCustomPermission_Success(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	err := mp.AddCustomPermission(&Permission{Resource: ResourcePatent, Action: ActionDelete, Allowed: true})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(mp.CustomPermissions))
}

func TestMemberPermission_AddCustomPermission_Duplicate(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	perm := &Permission{Resource: ResourcePatent, Action: ActionDelete, Allowed: true}
	mp.AddCustomPermission(perm)
	err := mp.AddCustomPermission(perm)
	assert.Error(t, err)
}

func TestMemberPermission_RemoveCustomPermission_Success(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	mp.AddCustomPermission(&Permission{Resource: ResourcePatent, Action: ActionDelete, Allowed: true})
	err := mp.RemoveCustomPermission(ResourcePatent, ActionDelete)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(mp.CustomPermissions))
}

func TestMemberPermission_RemoveCustomPermission_NotFound(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	err := mp.RemoveCustomPermission(ResourcePatent, ActionDelete)
	assert.Error(t, err)
}

func TestMemberPermission_Validate_Success(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	err := mp.Validate()
	assert.NoError(t, err)
}

func TestMemberPermission_Validate_EmptyID(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "user1", RoleManager, "owner1")
	mp.ID = ""
	err := mp.Validate()
	assert.Error(t, err)
}

//Personal.AI order the ending
