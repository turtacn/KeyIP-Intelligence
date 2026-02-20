// Package collaboration_test provides unit tests for the collaboration domain service.
package collaboration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock repository
// ─────────────────────────────────────────────────────────────────────────────

type mockRepository struct {
	workspaces map[common.ID]*collaboration.Workspace
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		workspaces: make(map[common.ID]*collaboration.Workspace),
	}
}

func (r *mockRepository) SaveWorkspace(ctx context.Context, ws *collaboration.Workspace) error {
	if _, exists := r.workspaces[ws.ID]; exists {
		return errors.Conflict("workspace already exists")
	}
	r.workspaces[ws.ID] = ws
	return nil
}

func (r *mockRepository) FindWorkspaceByID(ctx context.Context, id common.ID) (*collaboration.Workspace, error) {
	ws, ok := r.workspaces[id]
	if !ok {
		return nil, errors.NotFound("workspace not found")
	}
	return ws, nil
}

func (r *mockRepository) FindWorkspacesByUser(ctx context.Context, userID common.UserID, page common.PageRequest) (*common.PageResponse[*collaboration.Workspace], error) {
	var result []*collaboration.Workspace
	for _, ws := range r.workspaces {
		if ws.IsMember(userID) {
			result = append(result, ws)
		}
	}
	return &common.PageResponse[*collaboration.Workspace]{
	Items:    result,
		Total:    int64(len(result)),
			Page:     page.Page,
			PageSize: page.PageSize,
	}, nil
}

func (r *mockRepository) UpdateWorkspace(ctx context.Context, ws *collaboration.Workspace) error {
	if _, ok := r.workspaces[ws.ID]; !ok {
		return errors.NotFound("workspace not found")
	}
	r.workspaces[ws.ID] = ws
	return nil
}

func (r *mockRepository) DeleteWorkspace(ctx context.Context, id common.ID) error {
	delete(r.workspaces, id)
	return nil
}

func (r *mockRepository) FindWorkspacesByResource(ctx context.Context, resourceID common.ID) ([]*collaboration.Workspace, error) {
	var result []*collaboration.Workspace
	for _, ws := range r.workspaces {
		for _, res := range ws.SharedResources {
			if res.ResourceID == resourceID {
				result = append(result, ws)
				break
			}
		}
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Service tests
// ─────────────────────────────────────────────────────────────────────────────

func setupService() (*collaboration.Service, *mockRepository) {
	repo := newMockRepository()
	logger := logging.NewNopLogger()
	svc := collaboration.NewService(repo, logger)
	return svc, repo
}

func TestService_CreateWorkspace(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ws, err := svc.CreateWorkspace(ctx, "Test Workspace", "Description", "user-1")
	require.NoError(t, err)
	require.NotNil(t, ws)
	assert.Equal(t, "Test Workspace", ws.Name)
	assert.Equal(t, common.UserID("user-1"), ws.OwnerID)
}

func TestService_GetWorkspace(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	created, _ := svc.CreateWorkspace(ctx, "WS", "", "user-1")

	retrieved, err := svc.GetWorkspace(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
}

func TestService_InviteMember_WithPermission(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	newUser := common.UserID("new-user")
	err := svc.InviteMember(ctx, ws.ID, newUser, collaboration.RoleEditor, ownerID)
	require.NoError(t, err)

	updated, _ := svc.GetWorkspace(ctx, ws.ID)
	assert.Len(t, updated.Members, 2)
}

func TestService_InviteMember_WithoutPermission(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	viewerID := common.UserID("viewer")
	_ = svc.InviteMember(ctx, ws.ID, viewerID, collaboration.RoleViewer, ownerID)

	// Viewer tries to invite another user (should fail).
	err := svc.InviteMember(ctx, ws.ID, "user-3", collaboration.RoleEditor, viewerID)
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeForbidden))
}

func TestService_RemoveMember_Success(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	memberID := common.UserID("member")
	_ = svc.InviteMember(ctx, ws.ID, memberID, collaboration.RoleViewer, ownerID)

	err := svc.RemoveMember(ctx, ws.ID, memberID, ownerID)
	require.NoError(t, err)

	updated, _ := svc.GetWorkspace(ctx, ws.ID)
	assert.Len(t, updated.Members, 1, "only owner should remain")
}

func TestService_ShareResource(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	resource := collaboration.SharedResource{
		ResourceID:   common.ID("res-1"),
		ResourceType: "patent",
		AccessLevel:  "read",
	}

	err := svc.ShareResource(ctx, ws.ID, resource, ownerID)
	require.NoError(t, err)

	updated, _ := svc.GetWorkspace(ctx, ws.ID)
	require.Len(t, updated.SharedResources, 1)
	assert.Equal(t, common.ID("res-1"), updated.SharedResources[0].ResourceID)
}

func TestService_CheckAccess(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	resourceID := common.ID("res-1")
	resource := collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		AccessLevel:  "write",
	}
	_ = svc.ShareResource(ctx, ws.ID, resource, ownerID)

	hasAccess, err := svc.CheckAccess(ctx, ws.ID, ownerID, resourceID, "write")
	require.NoError(t, err)
	assert.True(t, hasAccess)
}

func TestService_GetUserWorkspaces(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	userID := common.UserID("user-multi")
	_, _ = svc.CreateWorkspace(ctx, "WS1", "", userID)
	_, _ = svc.CreateWorkspace(ctx, "WS2", "", userID)

	page := common.PageRequest{Page: 1, PageSize: 10}
	resp, err := svc.GetUserWorkspaces(ctx, userID, page)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(resp.Items), 2)
}

func TestService_UpdateWorkspace(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "Old Name", "Old Desc", ownerID)

	err := svc.UpdateWorkspace(ctx, ws.ID, "New Name", "New Desc", ownerID)
	require.NoError(t, err)

	updated, _ := svc.GetWorkspace(ctx, ws.ID)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "New Desc", updated.Description)
}

func TestService_DeleteWorkspace(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	err := svc.DeleteWorkspace(ctx, ws.ID, ownerID)
	require.NoError(t, err)

	_, err = svc.GetWorkspace(ctx, ws.ID)
	require.Error(t, err)
	assert.True(t, errors.IsCode(err, errors.CodeNotFound))
}

func TestService_ChangeMemberRole(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	memberID := common.UserID("member")
	_ = svc.InviteMember(ctx, ws.ID, memberID, collaboration.RoleViewer, ownerID)

	err := svc.ChangeMemberRole(ctx, ws.ID, memberID, collaboration.RoleAdmin, ownerID)
	require.NoError(t, err)

	updated, _ := svc.GetWorkspace(ctx, ws.ID)
	role, _ := updated.GetMemberRole(memberID)
	assert.Equal(t, collaboration.RoleAdmin, role)
}

func TestService_UnshareResource(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	resource := collaboration.SharedResource{
		ResourceID:   common.ID("res-1"),
		ResourceType: "patent",
		AccessLevel:  "read",
	}
	_ = svc.ShareResource(ctx, ws.ID, resource, ownerID)

	err := svc.UnshareResource(ctx, ws.ID, common.ID("res-1"), ownerID)
	require.NoError(t, err)

	updated, _ := svc.GetWorkspace(ctx, ws.ID)
	assert.Empty(t, updated.SharedResources)
}

func TestService_GetWorkspacesByResource(t *testing.T) {
	t.Parallel()

	svc, _ := setupService()
	ctx := context.Background()

	ownerID := common.UserID("owner")
	ws, _ := svc.CreateWorkspace(ctx, "WS", "", ownerID)

	resourceID := common.ID("res-1")
	resource := collaboration.SharedResource{
		ResourceID:   resourceID,
		ResourceType: "patent",
		AccessLevel:  "read",
	}
	_ = svc.ShareResource(ctx, ws.ID, resource, ownerID)

	workspaces, err := svc.GetWorkspacesByResource(ctx, resourceID)
	require.NoError(t, err)
	assert.Len(t, workspaces, 1)
	assert.Equal(t, ws.ID, workspaces[0].ID)
}

//Personal.AI order the ending
