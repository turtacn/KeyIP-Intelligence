package collaboration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Mock objects
type mockWorkspaceRepository struct {
	mock.Mock
}

func (m *mockWorkspaceRepository) Save(ctx context.Context, workspace *Workspace) error {
	args := m.Called(ctx, workspace)
	return args.Error(0)
}

func (m *mockWorkspaceRepository) FindByID(ctx context.Context, id string) (*Workspace, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Workspace), args.Error(1)
}

func (m *mockWorkspaceRepository) FindByOwnerID(ctx context.Context, ownerID string) ([]*Workspace, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]*Workspace), args.Error(1)
}

func (m *mockWorkspaceRepository) FindByMemberID(ctx context.Context, userID string) ([]*Workspace, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*Workspace), args.Error(1)
}

func (m *mockWorkspaceRepository) FindBySlug(ctx context.Context, slug string) (*Workspace, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Workspace), args.Error(1)
}

func (m *mockWorkspaceRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockWorkspaceRepository) Count(ctx context.Context, ownerID string) (int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(int64), args.Error(1)
}

type mockMemberRepository struct {
	mock.Mock
}

func (m *mockMemberRepository) Save(ctx context.Context, member *MemberPermission) error {
	args := m.Called(ctx, member)
	return args.Error(0)
}

func (m *mockMemberRepository) FindByID(ctx context.Context, id string) (*MemberPermission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MemberPermission), args.Error(1)
}

func (m *mockMemberRepository) FindByWorkspaceID(ctx context.Context, workspaceID string) ([]*MemberPermission, error) {
	args := m.Called(ctx, workspaceID)
	return args.Get(0).([]*MemberPermission), args.Error(1)
}

func (m *mockMemberRepository) FindByUserID(ctx context.Context, userID string) ([]*MemberPermission, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*MemberPermission), args.Error(1)
}

func (m *mockMemberRepository) FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*MemberPermission, error) {
	args := m.Called(ctx, workspaceID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MemberPermission), args.Error(1)
}

func (m *mockMemberRepository) FindByRole(ctx context.Context, workspaceID string, role Role) ([]*MemberPermission, error) {
	args := m.Called(ctx, workspaceID, role)
	return args.Get(0).([]*MemberPermission), args.Error(1)
}

func (m *mockMemberRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockMemberRepository) CountByWorkspace(ctx context.Context, workspaceID string) (int64, error) {
	args := m.Called(ctx, workspaceID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockMemberRepository) CountByRole(ctx context.Context, workspaceID string) (map[Role]int64, error) {
	args := m.Called(ctx, workspaceID)
	return args.Get(0).(map[Role]int64), args.Error(1)
}

type mockPermissionPolicy struct {
	mock.Mock
}

func (m *mockPermissionPolicy) HasPermission(role Role, resource ResourceType, action Action) bool {
	args := m.Called(role, resource, action)
	return args.Bool(0)
}

func (m *mockPermissionPolicy) GetRolePermissions(role Role) (*RolePermissions, error) {
	args := m.Called(role)
	return args.Get(0).(*RolePermissions), args.Error(1)
}

func (m *mockPermissionPolicy) ListRoles() []*RolePermissions {
	args := m.Called()
	return args.Get(0).([]*RolePermissions)
}

func (m *mockPermissionPolicy) CheckAccess(member *MemberPermission, resource ResourceType, action Action) (bool, string) {
	args := m.Called(member, resource, action)
	return args.Bool(0), args.String(1)
}

func (m *mockPermissionPolicy) GetEffectivePermissions(member *MemberPermission) []*Permission {
	args := m.Called(member)
	return args.Get(0).([]*Permission)
}

// Tests
func TestCreateWorkspace_Success(t *testing.T) {
	wsRepo := new(mockWorkspaceRepository)
	memRepo := new(mockMemberRepository)
	policy := new(mockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, policy)

	wsRepo.On("FindBySlug", mock.Anything, "my-workspace").Return(nil, nil)
	wsRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	memRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	ws, err := svc.CreateWorkspace(context.Background(), "My Workspace", "owner1")
	assert.NoError(t, err)
	assert.Equal(t, "My Workspace", ws.Name)
	assert.Equal(t, "my-workspace", ws.Slug)
	wsRepo.AssertExpectations(t)
	memRepo.AssertExpectations(t)
}

func TestInviteMember_Success(t *testing.T) {
	wsRepo := new(mockWorkspaceRepository)
	memRepo := new(mockMemberRepository)
	policy := new(mockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, policy)

	inviter := &MemberPermission{Role: RoleAdmin, IsActive: true, AcceptedAt: new(time.Time)}
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "inviter1").Return(inviter, nil)
	policy.On("CheckAccess", inviter, ResourceWorkspace, ActionManageMembers).Return(true, "")
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "invitee1").Return(nil, errors.NotFound("not found"))
	memRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	mp, err := svc.InviteMember(context.Background(), "ws1", "inviter1", "invitee1", RoleManager)
	assert.NoError(t, err)
	assert.Equal(t, RoleManager, mp.Role)
	assert.Nil(t, mp.AcceptedAt)
}

func TestAcceptInvitation_Success(t *testing.T) {
	wsRepo := new(mockWorkspaceRepository)
	memRepo := new(mockMemberRepository)
	policy := new(mockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, policy)

	mp := &MemberPermission{ID: "m1", WorkspaceID: "ws1", UserID: "user1"}
	ws := &Workspace{ID: "ws1", MemberCount: 1}
	memRepo.On("FindByID", mock.Anything, "m1").Return(mp, nil)
	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)
	wsRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	memRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.AcceptInvitation(context.Background(), "m1", "user1")
	assert.NoError(t, err)
	assert.NotNil(t, mp.AcceptedAt)
	assert.Equal(t, 2, ws.MemberCount)
}

func TestRemoveMember_Success(t *testing.T) {
	wsRepo := new(mockWorkspaceRepository)
	memRepo := new(mockMemberRepository)
	policy := new(mockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, policy)

	remover := &MemberPermission{Role: RoleAdmin, IsActive: true, AcceptedAt: new(time.Time)}
	target := &MemberPermission{Role: RoleManager, IsActive: true, AcceptedAt: new(time.Time)}
	ws := &Workspace{ID: "ws1", MemberCount: 2}

	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "remover1").Return(remover, nil)
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "target1").Return(target, nil)
	policy.On("CheckAccess", remover, ResourceWorkspace, ActionManageMembers).Return(true, "")
	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)
	wsRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	memRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.RemoveMember(context.Background(), "ws1", "remover1", "target1")
	assert.NoError(t, err)
	assert.False(t, target.IsActive)
	assert.Equal(t, 1, ws.MemberCount)
}

func TestTransferOwnership_Success(t *testing.T) {
	wsRepo := new(mockWorkspaceRepository)
	memRepo := new(mockMemberRepository)
	policy := new(mockPermissionPolicy)
	svc := NewCollaborationService(wsRepo, memRepo, policy)

	ws := &Workspace{ID: "ws1", OwnerID: "owner1"}
	newOwner := &MemberPermission{UserID: "owner2", Role: RoleAdmin, IsActive: true}
	oldOwner := &MemberPermission{UserID: "owner1", Role: RoleOwner, IsActive: true}

	wsRepo.On("FindByID", mock.Anything, "ws1").Return(ws, nil)
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "owner2").Return(newOwner, nil)
	memRepo.On("FindByWorkspaceAndUser", mock.Anything, "ws1", "owner1").Return(oldOwner, nil)
	wsRepo.On("Save", mock.Anything, mock.Anything).Return(nil)
	memRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := svc.TransferOwnership(context.Background(), "ws1", "owner1", "owner2")
	assert.NoError(t, err)
	assert.Equal(t, "owner2", ws.OwnerID)
	assert.Equal(t, RoleOwner, newOwner.Role)
	assert.Equal(t, RoleAdmin, oldOwner.Role)
}

func TestGenerateSlug(t *testing.T) {
	assert.Equal(t, "my-patent-portfolio", GenerateSlug("My Patent Portfolio"))
	assert.Equal(t, "hello-world2025", GenerateSlug("Hello! World@2025"))
	assert.Equal(t, "a-b-c", GenerateSlug("a--b---c"))
	assert.Equal(t, "hello", GenerateSlug("-hello-"))
}

//Personal.AI order the ending
