package collaboration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Mocks

type MockWorkspaceRepository struct {
	mock.Mock
}

func (m *MockWorkspaceRepository) Save(ctx context.Context, w *Workspace) error {
	args := m.Called(ctx, w)
	return args.Error(0)
}
func (m *MockWorkspaceRepository) FindByID(ctx context.Context, id string) (*Workspace, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*Workspace), args.Error(1)
}
func (m *MockWorkspaceRepository) FindByOwnerID(ctx context.Context, ownerID string) ([]*Workspace, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).([]*Workspace), args.Error(1)
}
func (m *MockWorkspaceRepository) FindByMemberID(ctx context.Context, userID string) ([]*Workspace, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).([]*Workspace), args.Error(1)
}
func (m *MockWorkspaceRepository) FindBySlug(ctx context.Context, slug string) (*Workspace, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*Workspace), args.Error(1)
}
func (m *MockWorkspaceRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockWorkspaceRepository) Count(ctx context.Context, ownerID string) (int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(int64), args.Error(1)
}

type MockMemberRepository struct {
	mock.Mock
}

func (m *MockMemberRepository) Save(ctx context.Context, mp *MemberPermission) error {
	args := m.Called(ctx, mp)
	return args.Error(0)
}
func (m *MockMemberRepository) FindByID(ctx context.Context, id string) (*MemberPermission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*MemberPermission), args.Error(1)
}
func (m *MockMemberRepository) FindByWorkspaceID(ctx context.Context, wsID string) ([]*MemberPermission, error) {
	args := m.Called(ctx, wsID)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).([]*MemberPermission), args.Error(1)
}
func (m *MockMemberRepository) FindByUserID(ctx context.Context, userID string) ([]*MemberPermission, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).([]*MemberPermission), args.Error(1)
}
func (m *MockMemberRepository) FindByWorkspaceAndUser(ctx context.Context, wsID, userID string) (*MemberPermission, error) {
	args := m.Called(ctx, wsID, userID)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*MemberPermission), args.Error(1)
}
func (m *MockMemberRepository) FindByRole(ctx context.Context, wsID string, role Role) ([]*MemberPermission, error) {
	args := m.Called(ctx, wsID, role)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).([]*MemberPermission), args.Error(1)
}
func (m *MockMemberRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockMemberRepository) CountByWorkspace(ctx context.Context, wsID string) (int64, error) {
	args := m.Called(ctx, wsID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockMemberRepository) CountByRole(ctx context.Context, wsID string) (map[Role]int64, error) {
	args := m.Called(ctx, wsID)
	return args.Get(0).(map[Role]int64), args.Error(1)
}

type MockActivityRepository struct {
	mock.Mock
}

func (m *MockActivityRepository) Save(ctx context.Context, ar *ActivityRecord) error {
	args := m.Called(ctx, ar)
	return args.Error(0)
}
func (m *MockActivityRepository) FindByWorkspaceID(ctx context.Context, wsID string, limit int) ([]*ActivityRecord, error) {
	args := m.Called(ctx, wsID, limit)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).([]*ActivityRecord), args.Error(1)
}

type MockPermissionPolicy struct {
	mock.Mock
}

func (m *MockPermissionPolicy) HasPermission(role Role, resource ResourceType, action Action) bool {
	args := m.Called(role, resource, action)
	return args.Bool(0)
}
func (m *MockPermissionPolicy) GetRolePermissions(role Role) (*RolePermissions, error) {
	args := m.Called(role)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*RolePermissions), args.Error(1)
}
func (m *MockPermissionPolicy) ListRoles() []*RolePermissions {
	args := m.Called()
	return args.Get(0).([]*RolePermissions)
}
func (m *MockPermissionPolicy) CheckAccess(member *MemberPermission, resource ResourceType, action Action) (bool, string) {
	args := m.Called(member, resource, action)
	return args.Bool(0), args.String(1)
}
func (m *MockPermissionPolicy) GetEffectivePermissions(member *MemberPermission) []*Permission {
	args := m.Called(member)
	return args.Get(0).([]*Permission)
}

// Tests

func TestCreateWorkspace_Success(t *testing.T) {
	wsRepo := new(MockWorkspaceRepository)
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	policy := new(MockPermissionPolicy)

	svc := NewCollaborationService(wsRepo, memRepo, actRepo, policy)

	wsRepo.On("FindBySlug", mock.Anything, "my-workspace").Return(nil, nil)
	wsRepo.On("Save", mock.Anything, mock.AnythingOfType("*collaboration.Workspace")).Return(nil)
	memRepo.On("Save", mock.Anything, mock.AnythingOfType("*collaboration.MemberPermission")).Return(nil)
	actRepo.On("Save", mock.Anything, mock.AnythingOfType("*collaboration.ActivityRecord")).Return(nil)

	ws, err := svc.CreateWorkspace(context.Background(), "My Workspace", "owner1")
	assert.NoError(t, err)
	assert.NotNil(t, ws)
	assert.Equal(t, "my-workspace", ws.Slug)
}

func TestInviteMember_Success(t *testing.T) {
	wsRepo := new(MockWorkspaceRepository)
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	policy := new(MockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, actRepo, policy)

	inviter := &MemberPermission{Role: RoleAdmin, IsActive: true}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "inviter1").Return(inviter, nil)

	policy.On("CheckAccess", inviter, ResourceWorkspace, ActionManageMembers).Return(true, "")

	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "invitee1").Return(nil, nil)

	ws := &Workspace{Settings: WorkspaceSettings{MaxMembers: 10}, MemberCount: 2}
	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)

	memRepo.On("Save", mock.Anything, mock.AnythingOfType("*collaboration.MemberPermission")).Return(nil)
	wsRepo.On("Save", mock.Anything, ws).Return(nil)
	actRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	mp, err := svc.InviteMember(context.Background(), "ws1", "inviter1", "invitee1", RoleViewer)
	assert.NoError(t, err)
	assert.NotNil(t, mp)
}

func TestInviteMember_InviterNoPermission(t *testing.T) {
	wsRepo := new(MockWorkspaceRepository)
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	policy := new(MockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, actRepo, policy)

	inviter := &MemberPermission{Role: RoleViewer, IsActive: true}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "inviter1").Return(inviter, nil)

	policy.On("CheckAccess", inviter, ResourceWorkspace, ActionManageMembers).Return(false, "denied")

	_, err := svc.InviteMember(context.Background(), "ws1", "inviter1", "invitee1", RoleViewer)
	assert.Error(t, err)
	assert.True(t, apperrors.IsForbidden(err))
}

func TestAcceptInvitation_Success(t *testing.T) {
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	svc := NewCollaborationService(nil, memRepo, actRepo, nil)

	mp := &MemberPermission{ID: "mp1", UserID: "u1"}
	memRepo.On("FindByID", mock.Anything, "mp1").Return(mp, nil)
	memRepo.On("Save", mock.Anything, mp).Return(nil)
	actRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.AcceptInvitation(context.Background(), "mp1", "u1")
	assert.NoError(t, err)
	assert.NotNil(t, mp.AcceptedAt)
}

func TestRemoveMember_Success(t *testing.T) {
	wsRepo := new(MockWorkspaceRepository)
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	policy := new(MockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, actRepo, policy)

	remover := &MemberPermission{Role: RoleAdmin, IsActive: true}
	target := &MemberPermission{Role: RoleViewer, IsActive: true}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "remover").Return(remover, nil)
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "target").Return(target, nil)

	policy.On("CheckAccess", remover, ResourceWorkspace, ActionManageMembers).Return(true, "")

	memRepo.On("Save", mock.Anything, target).Return(nil)
	ws := &Workspace{MemberCount: 2}
	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)
	wsRepo.On("Save", mock.Anything, ws).Return(nil)
	actRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.RemoveMember(context.Background(), "ws1", "remover", "target")
	assert.NoError(t, err)
	assert.False(t, target.IsActive)
}

func TestChangeMemberRole_Success(t *testing.T) {
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	policy := new(MockPermissionPolicy)
	svc := NewCollaborationService(nil, memRepo, actRepo, policy)

	changer := &MemberPermission{Role: RoleAdmin, IsActive: true}
	target := &MemberPermission{Role: RoleViewer, IsActive: true}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "changer").Return(changer, nil)
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "target").Return(target, nil)

	policy.On("CheckAccess", changer, ResourceWorkspace, ActionManageMembers).Return(true, "")

	memRepo.On("Save", mock.Anything, target).Return(nil)
	actRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.ChangeMemberRole(context.Background(), "ws1", "changer", "target", RoleManager)
	assert.NoError(t, err)
	assert.Equal(t, RoleManager, target.Role)
}

func TestTransferOwnership_Success(t *testing.T) {
	wsRepo := new(MockWorkspaceRepository)
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	svc := NewCollaborationService(wsRepo, memRepo, actRepo, nil)

	ws := &Workspace{ID: "ws1", OwnerID: "old"}
	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)

	newOwnerMP := &MemberPermission{Role: RoleAdmin, IsActive: true}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "new").Return(newOwnerMP, nil)

	oldOwnerMP := &MemberPermission{Role: RoleOwner, IsActive: true}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "old").Return(oldOwnerMP, nil)

	memRepo.On("Save", mock.Anything, mock.Anything).Return(nil) // Save both
	wsRepo.On("Save", mock.Anything, ws).Return(nil)
	actRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.TransferOwnership(context.Background(), "ws1", "old", "new")
	assert.NoError(t, err)
	assert.Equal(t, "new", ws.OwnerID)
}

func TestUpdateWorkspace_Success(t *testing.T) {
	wsRepo := new(MockWorkspaceRepository)
	memRepo := new(MockMemberRepository)
	policy := new(MockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, nil, policy)

	mp := &MemberPermission{Role: RoleAdmin}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "u1").Return(mp, nil)

	policy.On("CheckAccess", mp, ResourceWorkspace, ActionUpdate).Return(true, "")

	ws := &Workspace{ID: "ws1", Name: "Old"}
	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)
	wsRepo.On("Save", mock.Anything, ws).Return(nil)

	err := svc.UpdateWorkspace(context.Background(), "ws1", "u1", "New", "Desc")
	assert.NoError(t, err)
	assert.Equal(t, "New", ws.Name)
}

func TestDeleteWorkspace_Success(t *testing.T) {
	wsRepo := new(MockWorkspaceRepository)
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	svc := NewCollaborationService(wsRepo, memRepo, actRepo, nil)

	ws := &Workspace{ID: "ws1", OwnerID: "u1"}
	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)

	memRepo.On("FindByWorkspaceID", mock.Anything, "ws1").Return([]*MemberPermission{}, nil)
	wsRepo.On("Save", mock.Anything, ws).Return(nil)
	actRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.DeleteWorkspace(context.Background(), "ws1", "u1")
	assert.NoError(t, err)
	assert.Equal(t, WorkspaceStatusDeleted, ws.Status)
}

func TestGrantCustomPermission_Success(t *testing.T) {
	memRepo := new(MockMemberRepository)
	actRepo := new(MockActivityRepository)
	policy := new(MockPermissionPolicy)
	svc := NewCollaborationService(nil, memRepo, actRepo, policy)

	granter := &MemberPermission{Role: RoleAdmin}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "granter").Return(granter, nil)

	policy.On("CheckAccess", granter, ResourceWorkspace, ActionManageMembers).Return(true, "")
	policy.On("CheckAccess", granter, ResourcePatent, ActionDelete).Return(true, "")

	target := &MemberPermission{Role: RoleViewer}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "target").Return(target, nil)
	memRepo.On("Save", mock.Anything, target).Return(nil)
	actRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	perm := &Permission{Resource: ResourcePatent, Action: ActionDelete, Allowed: true}
	err := svc.GrantCustomPermission(context.Background(), "ws1", "granter", "target", perm)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(target.CustomPermissions))
}

func TestGetWorkspaceActivity_Success(t *testing.T) {
	actRepo := new(MockActivityRepository)
	svc := NewCollaborationService(nil, nil, actRepo, nil)

	recs := []*ActivityRecord{{ID: "act1"}}
	actRepo.On("FindByWorkspaceID", mock.Anything, "ws1", 10).Return(recs, nil)

	res, err := svc.GetWorkspaceActivity(context.Background(), "ws1", 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res))
}

func TestGenerateSlug(t *testing.T) {
	assert.Equal(t, "hello-world", GenerateSlug("Hello World"))
	assert.Equal(t, "a-b", GenerateSlug("a_b"))
	assert.Equal(t, "a-b", GenerateSlug("a--b"))
}

//Personal.AI order the ending
