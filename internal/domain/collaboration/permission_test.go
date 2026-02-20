// Package collaboration_test provides unit tests for the permission model.
package collaboration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestHasPermission
// ─────────────────────────────────────────────────────────────────────────────

func TestHasPermission_OwnerHasAllPermissions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action   string
		resource string
	}{
		{"read", "patent"},
		{"write", "patent"},
		{"delete", "patent"},
		{"read", "portfolio"},
		{"write", "portfolio"},
		{"generate", "report"},
		{"manage", "workspace"},
		{"invite", "member"},
		{"remove", "member"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.action+"_"+tc.resource, func(t *testing.T) {
			t.Parallel()
			assert.True(t, collaboration.HasPermission(collaboration.RoleOwner, tc.action, tc.resource))
		})
	}
}

func TestHasPermission_AdminCannotDelete(t *testing.T) {
	t.Parallel()

	assert.False(t, collaboration.HasPermission(collaboration.RoleAdmin, "delete", "patent"),
		"Admin should not have delete permission")
}

func TestHasPermission_AdminHasOtherPermissions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		action   string
		resource string
	}{
		{"read", "patent"},
		{"write", "patent"},
		{"manage", "workspace"},
		{"invite", "member"},
		{"remove", "member"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.action+"_"+tc.resource, func(t *testing.T) {
			t.Parallel()
			assert.True(t, collaboration.HasPermission(collaboration.RoleAdmin, tc.action, tc.resource))
		})
	}
}

func TestHasPermission_EditorCanReadAndWrite(t *testing.T) {
	t.Parallel()

	assert.True(t, collaboration.HasPermission(collaboration.RoleEditor, "read", "patent"))
	assert.True(t, collaboration.HasPermission(collaboration.RoleEditor, "write", "patent"))
	assert.True(t, collaboration.HasPermission(collaboration.RoleEditor, "read", "portfolio"))
	assert.True(t, collaboration.HasPermission(collaboration.RoleEditor, "write", "portfolio"))
}

func TestHasPermission_EditorCannotDelete(t *testing.T) {
	t.Parallel()

	assert.False(t, collaboration.HasPermission(collaboration.RoleEditor, "delete", "patent"))
}

func TestHasPermission_EditorCannotManageWorkspace(t *testing.T) {
	t.Parallel()

	assert.False(t, collaboration.HasPermission(collaboration.RoleEditor, "manage", "workspace"))
	assert.False(t, collaboration.HasPermission(collaboration.RoleEditor, "invite", "member"))
}

func TestHasPermission_ViewerCanOnlyRead(t *testing.T) {
	t.Parallel()

	assert.True(t, collaboration.HasPermission(collaboration.RoleViewer, "read", "patent"))
	assert.True(t, collaboration.HasPermission(collaboration.RoleViewer, "read", "portfolio"))
}

func TestHasPermission_ViewerCannotWrite(t *testing.T) {
	t.Parallel()

	assert.False(t, collaboration.HasPermission(collaboration.RoleViewer, "write", "patent"))
	assert.False(t, collaboration.HasPermission(collaboration.RoleViewer, "write", "portfolio"))
	assert.False(t, collaboration.HasPermission(collaboration.RoleViewer, "delete", "patent"))
	assert.False(t, collaboration.HasPermission(collaboration.RoleViewer, "generate", "report"))
}

func TestHasPermission_UnknownRoleReturnsFalse(t *testing.T) {
	t.Parallel()

	unknownRole := collaboration.MemberRole("unknown")
	assert.False(t, collaboration.HasPermission(unknownRole, "read", "patent"))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPermissionSet_Contains
// ─────────────────────────────────────────────────────────────────────────────

func TestPermissionSet_Contains_True(t *testing.T) {
	t.Parallel()

	ps := collaboration.PermissionSet{
		Permissions: []collaboration.Permission{
			{Action: "read", Resource: "patent"},
			{Action: "write", Resource: "patent"},
		},
	}

	assert.True(t, ps.Contains("read", "patent"))
	assert.True(t, ps.Contains("write", "patent"))
}

func TestPermissionSet_Contains_False(t *testing.T) {
	t.Parallel()

	ps := collaboration.PermissionSet{
		Permissions: []collaboration.Permission{
			{Action: "read", Resource: "patent"},
		},
	}

	assert.False(t, ps.Contains("write", "patent"))
	assert.False(t, ps.Contains("read", "portfolio"))
}

func TestPermissionSet_Contains_EmptySet(t *testing.T) {
	t.Parallel()

	ps := collaboration.PermissionSet{}
	assert.False(t, ps.Contains("read", "patent"))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPermissionSet_Merge
// ─────────────────────────────────────────────────────────────────────────────

func TestPermissionSet_Merge_CombinesBothSets(t *testing.T) {
	t.Parallel()

	ps1 := collaboration.PermissionSet{
		Permissions: []collaboration.Permission{
			{Action: "read", Resource: "patent"},
		},
	}
	ps2 := collaboration.PermissionSet{
		Permissions: []collaboration.Permission{
			{Action: "write", Resource: "patent"},
		},
	}

	merged := ps1.Merge(ps2)

	assert.Len(t, merged.Permissions, 2)
	assert.True(t, merged.Contains("read", "patent"))
	assert.True(t, merged.Contains("write", "patent"))
}

func TestPermissionSet_Merge_OriginalUnchanged(t *testing.T) {
	t.Parallel()

	ps1 := collaboration.PermissionSet{
		Permissions: []collaboration.Permission{
			{Action: "read", Resource: "patent"},
		},
	}
	ps2 := collaboration.PermissionSet{
		Permissions: []collaboration.Permission{
			{Action: "write", Resource: "patent"},
		},
	}

	_ = ps1.Merge(ps2)

	assert.Len(t, ps1.Permissions, 1, "original should be unchanged")
	assert.Len(t, ps2.Permissions, 1, "other should be unchanged")
}

func TestPermissionSet_Merge_EmptySets(t *testing.T) {
	t.Parallel()

	ps1 := collaboration.PermissionSet{}
	ps2 := collaboration.PermissionSet{}

	merged := ps1.Merge(ps2)
	assert.Len(t, merged.Permissions, 0)
}

func TestPermissionSet_Merge_PreservesDuplicates(t *testing.T) {
	t.Parallel()

	perm := collaboration.Permission{Action: "read", Resource: "patent"}
	ps1 := collaboration.PermissionSet{Permissions: []collaboration.Permission{perm}}
	ps2 := collaboration.PermissionSet{Permissions: []collaboration.Permission{perm}}

	merged := ps1.Merge(ps2)
	assert.Len(t, merged.Permissions, 2, "Merge does not deduplicate")
}

