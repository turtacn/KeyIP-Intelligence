// Package collaboration_test provides unit tests for the Workspace aggregate root.
package collaboration_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestNewWorkspace
// ─────────────────────────────────────────────────────────────────────────────

func TestNewWorkspace_ValidParams(t *testing.T) {
	t.Parallel()

	ownerID := common.UserID("user-123")
	ws, err := collaboration.NewWorkspace("My Workspace", "Description", ownerID)

	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, "My Workspace", ws.Name)
	assert.Equal(t, "Description", ws.Description)
	assert.Equal(t, ownerID, ws.OwnerID)
	assert.Equal(t, common.StatusActive, ws.Status)
	assert.NotEmpty(t, ws.ID)
}

func TestNewWorkspace_OwnerAutomaticallyAddedAsMember(t *testing.T) {
	t.Parallel()

	ownerID := common.UserID("user-owner")
	ws, err := collaboration.NewWorkspace("Test", "", ownerID)

	require.NoError(t, err)
	require.Len(t, ws.Members, 1, "owner should be auto-added")
	assert.Equal(t, ownerID, ws.Members[0].UserID)
	assert.Equal(t, collaboration.RoleOwner, ws.Members[0].Role)
	assert.Equal(t, ownerID, ws.Members[0].InvitedBy)
}

func TestNewWorkspace_EmptyNameReturnsError(t *testing.T) {
	t.Parallel()

	_, err := collaboration.NewWorkspace("", "desc", "user-123")
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeInvalidParam))
}

func TestNewWorkspace_EmptyOwnerIDReturnsError(t *testing.T) {
	t.Parallel()

	_, err := collaboration.NewWorkspace("name", "desc", "")
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeInvalidParam))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAddMember
// ─────────────────────────────────────────────────────────────────────────────

func TestAddMember_Success(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	newUser := common.UserID("user-new")

	err := ws.AddMember(newUser, collaboration.RoleEditor, "owner")
	require.NoError(t, err)

	assert.Len(t, ws.Members, 2)
	assert.Equal(t, newUser, ws.Members[1].UserID)
	assert.Equal(t, collaboration.RoleEditor, ws.Members[1].Role)
}

func TestAddMember_DuplicateReturnsError(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	userID := common.UserID("user-dup")

	err := ws.AddMember(userID, collaboration.RoleViewer, "owner")
	require.NoError(t, err)

	err = ws.AddMember(userID, collaboration.RoleEditor, "owner")
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeConflict))
}

func TestAddMember_EmptyUserIDReturnsError(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	err := ws.AddMember("", collaboration.RoleViewer, "owner")
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeInvalidParam))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRemoveMember
// ─────────────────────────────────────────────────────────────────────────────

func TestRemoveMember_Success(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	userID := common.UserID("user-remove")
	_ = ws.AddMember(userID, collaboration.RoleViewer, "owner")

	err := ws.RemoveMember(userID)
	require.NoError(t, err)
	assert.Len(t, ws.Members, 1, "only owner should remain")
}

func TestRemoveMember_CannotRemoveOwner(t *testing.T) {
	t.Parallel()

	ownerID := common.UserID("owner")
	ws, _ := collaboration.NewWorkspace("ws", "", ownerID)

	err := ws.RemoveMember(ownerID)
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeForbidden))
}

func TestRemoveMember_NonExistentUserReturnsError(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	err := ws.RemoveMember("nonexistent")
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeNotFound))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestChangeMemberRole
// ─────────────────────────────────────────────────────────────────────────────

func TestChangeMemberRole_Success(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	userID := common.UserID("user-change")
	_ = ws.AddMember(userID, collaboration.RoleViewer, "owner")

	err := ws.ChangeMemberRole(userID, collaboration.RoleEditor)
	require.NoError(t, err)

	role, _ := ws.GetMemberRole(userID)
	assert.Equal(t, collaboration.RoleEditor, role)
}

func TestChangeMemberRole_CannotChangeOwnerRole(t *testing.T) {
	t.Parallel()

	ownerID := common.UserID("owner")
	ws, _ := collaboration.NewWorkspace("ws", "", ownerID)

	err := ws.ChangeMemberRole(ownerID, collaboration.RoleAdmin)
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeForbidden))
}

func TestChangeMemberRole_NonMemberReturnsError(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	err := ws.ChangeMemberRole("nonexistent", collaboration.RoleAdmin)
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeNotFound))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestHasAccess
// ─────────────────────────────────────────────────────────────────────────────

func TestHasAccess_MemberWithSufficientPermissions(t *testing.T) {
	t.Parallel()

	ownerID := common.UserID("owner")
	ws, _ := collaboration.NewWorkspace("ws", "", ownerID)

	resourceID := common.ID("res-1")
	_ = ws.ShareResource(collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		SharedBy:     ownerID,
		AccessLevel:  "write",
	})

	// Owner has access to everything.
	assert.True(t, ws.HasAccess(ownerID, resourceID, "write"))
	assert.True(t, ws.HasAccess(ownerID, resourceID, "read"))
}

func TestHasAccess_NonMemberReturnsFalse(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	resourceID := common.ID("res-1")
	_ = ws.ShareResource(collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		SharedBy:     "owner",
		AccessLevel:  "read",
	})

	nonMember := common.UserID("nonmember")
	assert.False(t, ws.HasAccess(nonMember, resourceID, "read"))
}

func TestHasAccess_ViewerCannotWrite(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	viewerID := common.UserID("viewer")
	_ = ws.AddMember(viewerID, collaboration.RoleViewer, "owner")

	resourceID := common.ID("res-1")
	_ = ws.ShareResource(collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		SharedBy:     "owner",
		AccessLevel:  "write",
	})

	assert.True(t, ws.HasAccess(viewerID, resourceID, "read"))
	assert.False(t, ws.HasAccess(viewerID, resourceID, "write"))
}

func TestHasAccess_EditorCanWriteIfResourcePermits(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	editorID := common.UserID("editor")
	_ = ws.AddMember(editorID, collaboration.RoleEditor, "owner")

	resourceID := common.ID("res-1")
	_ = ws.ShareResource(collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		SharedBy:     "owner",
		AccessLevel:  "write",
	})

	assert.True(t, ws.HasAccess(editorID, resourceID, "write"))
	assert.True(t, ws.HasAccess(editorID, resourceID, "read"))
}

func TestHasAccess_EditorCannotWriteIfResourceReadOnly(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	editorID := common.UserID("editor")
	_ = ws.AddMember(editorID, collaboration.RoleEditor, "owner")

	resourceID := common.ID("res-1")
	_ = ws.ShareResource(collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		SharedBy:     "owner",
		AccessLevel:  "read", // read-only
	})

	assert.True(t, ws.HasAccess(editorID, resourceID, "read"))
	assert.False(t, ws.HasAccess(editorID, resourceID, "write"))
}

func TestHasAccess_AdminHasFullAccess(t *testing.T) {
	t.Parallel()

	ws, _ := collaboration.NewWorkspace("ws", "", "owner")
	adminID := common.UserID("admin")
	_ = ws.AddMember(adminID, collaboration.RoleAdmin, "owner")

	resourceID := common.ID("res-1")
	_ = ws.ShareResource(collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		SharedBy:     "owner",
		AccessLevel:  "read",
	})

	// Admin bypasses resource access level restrictions.
	assert.True(t, ws.HasAccess(adminID, resourceID, "write"))
	assert.True(t, ws.HasAccess(adminID, resourceID, "read"))
}

