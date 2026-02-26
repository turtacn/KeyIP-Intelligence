package collaboration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPermissionPolicy(t *testing.T) {
	p := NewPermissionPolicy()
	assert.NotNil(t, p)
	assert.Equal(t, 7, len(p.ListRoles()))
}

func TestHasPermission(t *testing.T) {
	p := NewPermissionPolicy()

	// Owner: All
	assert.True(t, p.HasPermission(RoleOwner, ResourcePatent, ActionDelete))

	// Admin: Manage Members
	assert.True(t, p.HasPermission(RoleAdmin, ResourceWorkspace, ActionManageMembers))
	// Admin: Manage Settings? NO (Owner only in default)
	assert.False(t, p.HasPermission(RoleAdmin, ResourceWorkspace, ActionManageSettings))

	// Viewer: Read
	assert.True(t, p.HasPermission(RoleViewer, ResourcePatent, ActionRead))
	// Viewer: Update? NO
	assert.False(t, p.HasPermission(RoleViewer, ResourcePatent, ActionUpdate))
}

func TestCheckAccess(t *testing.T) {
	p := NewPermissionPolicy()
	now := time.Now()

	mp, _ := NewMemberPermission("ws1", "u1", RoleViewer, "admin")
	mp.AcceptedAt = &now // Accepted

	// Viewer Read -> OK
	allowed, _ := p.CheckAccess(mp, ResourcePatent, ActionRead)
	assert.True(t, allowed)

	// Viewer Update -> Denied
	allowed, reason := p.CheckAccess(mp, ResourcePatent, ActionUpdate)
	assert.False(t, allowed)
	assert.Contains(t, reason, "insufficient permission")

	// Inactive
	mp.IsActive = false
	allowed, reason = p.CheckAccess(mp, ResourcePatent, ActionRead)
	assert.False(t, allowed)
	assert.Contains(t, reason, "inactive")
	mp.IsActive = true

	// Not Accepted
	mp.AcceptedAt = nil
	allowed, reason = p.CheckAccess(mp, ResourcePatent, ActionRead)
	assert.False(t, allowed)
	assert.Contains(t, reason, "invitation not accepted")
	mp.AcceptedAt = &now

	// Custom Permission Override (Allow Update)
	mp.AddCustomPermission(&Permission{Resource: ResourcePatent, Action: ActionUpdate, Allowed: true})
	allowed, _ = p.CheckAccess(mp, ResourcePatent, ActionUpdate)
	assert.True(t, allowed)

	// Custom Permission Deny (Deny Read)
	mp.AddCustomPermission(&Permission{Resource: ResourcePatent, Action: ActionRead, Allowed: false})
	allowed, reason = p.CheckAccess(mp, ResourcePatent, ActionRead)
	assert.False(t, allowed)
	assert.Contains(t, reason, "denied by custom permission")
}

func TestIsRoleHigherOrEqual(t *testing.T) {
	assert.True(t, IsRoleHigherOrEqual(RoleOwner, RoleAdmin))
	assert.False(t, IsRoleHigherOrEqual(RoleAdmin, RoleOwner))
	assert.True(t, IsRoleHigherOrEqual(RoleAdmin, RoleAdmin))
}

func TestMemberPermission_ChangeRole(t *testing.T) {
	mp, _ := NewMemberPermission("ws1", "u1", RoleViewer, "admin")

	// Admin promotes Viewer to Manager (Admin > Manager) -> OK
	err := mp.ChangeRole(RoleManager, RoleAdmin)
	assert.NoError(t, err)
	assert.Equal(t, RoleManager, mp.Role)

	// Manager tries to promote to Owner (Manager < Owner) -> Fail
	err = mp.ChangeRole(RoleOwner, RoleManager)
	assert.Error(t, err)

	// Manager tries to change someone else (Viewer) -> OK if Manager > Viewer
	// But here we invoke on `mp` which is `RoleManager` (from previous step).
	// "Can't change someone higher than self".
	// If `mp` is Manager, and `changedBy` is Admin. Admin > Manager. OK.
}

//Personal.AI order the ending
