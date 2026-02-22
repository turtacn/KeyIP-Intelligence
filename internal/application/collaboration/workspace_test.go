// ---
// Phase 10 - File #197: internal/application/collaboration/workspace_test.go
//
// 测试用例:
//   - TestCreateWorkspaceRequest_Validate: 参数校验
//   - TestUpdateWorkspaceRequest_Validate: 参数校验
//   - TestAddMemberRequest_Validate / TestRemoveMemberRequest_Validate
//   - TestCreate_Success / NilRequest / PersistError
//   - TestUpdate_Success / NotFound / PermissionDenied / NilRequest
//   - TestDelete_Success / NotFound / PermissionDenied / OwnerCanDelete
//   - TestGetByID_Success / NotFound / EmptyID
//   - TestListByUser_Success / EmptyUserID / PaginationDefaults
//   - TestAddMember_Success / WorkspaceNotFound / PermissionDenied / NilRequest
//   - TestRemoveMember_Success / CannotRemoveOwner / NilRequest
//   - TestListMembers_Success / EmptyWorkspaceID
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
// ---

package collaboration

import (
	"context"
	"errors"
	"testing"
	"time"

	collabdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// --- Mock implementations for workspace tests ---

type mockWsDomainService struct {
	checkPermissionFn func(ctx context.Context, userID, workspaceID string, action collabdomain.Action) error
	createWorkspaceFn func(ctx context.Context, ws *collabdomain.Workspace) error
	addMemberFn       func(ctx context.Context, workspaceID, userID string, role collabdomain.Role) error
	removeMemberFn    func(ctx context.Context, workspaceID, userID string) error
}

func (m *mockWsDomainService) CheckPermission(ctx context.Context, userID, workspaceID string, action collabdomain.Action) error {
	if m.checkPermissionFn != nil {
		return m.checkPermissionFn(ctx, userID, workspaceID, action)
	}
	return nil
}

func (m *mockWsDomainService) CreateWorkspace(ctx context.Context, ws *collabdomain.Workspace) error {
	if m.createWorkspaceFn != nil {
		return m.createWorkspaceFn(ctx, ws)
	}
	return nil
}

func (m *mockWsDomainService) AddMember(ctx context.Context, workspaceID, userID string, role collabdomain.Role) error {
	if m.addMemberFn != nil {
		return m.addMemberFn(ctx, workspaceID, userID, role)
	}
	return nil
}

func (m *mockWsDomainService) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	if m.removeMemberFn != nil {
		return m.removeMemberFn(ctx, workspaceID, userID)
	}
	return nil
}

type mockWsRepo struct {
	createFn     func(ctx context.Context, ws *collabdomain.Workspace) error
	getByIDFn    func(ctx context.Context, id string) (*collabdomain.Workspace, error)
	updateFn     func(ctx context.Context, ws *collabdomain.Workspace) error
	deleteFn     func(ctx context.Context, id string) error
	listByUserFn func(ctx context.Context, userID string, p commontypes.Pagination) ([]*collabdomain.Workspace, int, error)
}

func (m *mockWsRepo) Create(ctx context.Context, ws *collabdomain.Workspace) error {
	if m.createFn != nil {
		return m.createFn(ctx, ws)
	}
	return nil
}

func (m *mockWsRepo) GetByID(ctx context.Context, id string) (*collabdomain.Workspace, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return &collabdomain.Workspace{
		ID:      id,
		Name:    "test-ws",
		OwnerID: "owner-1",
	}, nil
}

func (m *mockWsRepo) Update(ctx context.Context, ws *collabdomain.Workspace) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, ws)
	}
	return nil
}

func (m *mockWsRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockWsRepo) ListByUser(ctx context.Context, userID string, p commontypes.Pagination) ([]*collabdomain.Workspace, int, error) {
	if m.listByUserFn != nil {
		return m.listByUserFn(ctx, userID, p)
	}
	return nil, 0, nil
}

type mockMemberRepo struct {
	addFn       func(ctx context.Context, workspaceID, userID string, role collabdomain.Role) error
	removeFn    func(ctx context.Context, workspaceID, userID string) error
	listFn      func(ctx context.Context, workspaceID string, p commontypes.Pagination) ([]*MemberResponse, int, error)
	getMemberFn func(ctx context.Context, workspaceID, userID string) (*MemberResponse, error)
}

func (m *mockMemberRepo) Add(ctx context.Context, workspaceID, userID string, role collabdomain.Role) error {
	if m.addFn != nil {
		return m.addFn(ctx, workspaceID, userID, role)
	}
	return nil
}

func (m *mockMemberRepo) Remove(ctx context.Context, workspaceID, userID string) error {
	if m.removeFn != nil {
		return m.removeFn(ctx, workspaceID, userID)
	}
	return nil
}

func (m *mockMemberRepo) List(ctx context.Context, workspaceID string, p commontypes.Pagination) ([]*MemberResponse, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, workspaceID, p)
	}
	return nil, 0, nil
}

func (m *mockMemberRepo) GetMember(ctx context.Context, workspaceID, userID string) (*MemberResponse, error) {
	if m.getMemberFn != nil {
		return m.getMemberFn(ctx, workspaceID, userID)
	}
	return nil, errors.New("not found")
}

type mockWsLogger struct{}

func (m *mockWsLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockWsLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockWsLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (m *mockWsLogger) Error(msg string, keysAndValues ...interface{}) {}
func (m *mockWsLogger) With(keysAndValues ...interface{}) interface{}  { return m }

func newTestWorkspaceAppService(
	domainSvc *mockWsDomainService,
	wsRepo *mockWsRepo,
	memberRepo *mockMemberRepo,
) WorkspaceAppService {
	if domainSvc == nil {
		domainSvc = &mockWsDomainService{}
	}
	if wsRepo == nil {
		wsRepo = &mockWsRepo{}
	}
	if memberRepo == nil {
		memberRepo = &mockMemberRepo{}
	}
	return NewWorkspaceAppService(domainSvc, wsRepo, memberRepo, &mockWsLogger{})
}

// --- CreateWorkspaceRequest.Validate ---

func TestCreateWorkspaceRequest_Validate_Success(t *testing.T) {
	req := &CreateWorkspaceRequest{Name: "My WS", OwnerID: "user-1"}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCreateWorkspaceRequest_Validate_MissingName(t *testing.T) {
	req := &CreateWorkspaceRequest{OwnerID: "user-1"}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestCreateWorkspaceRequest_Validate_MissingOwner(t *testing.T) {
	req := &CreateWorkspaceRequest{Name: "WS"}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for missing owner_id")
	}
}

func TestCreateWorkspaceRequest_Validate_NameTooLong(t *testing.T) {
	longName := make([]byte, 257)
	for i := range longName {
		longName[i] = 'a'
	}
	req := &CreateWorkspaceRequest{Name: string(longName), OwnerID: "user-1"}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for name too long")
	}
}

// --- UpdateWorkspaceRequest.Validate ---

func TestUpdateWorkspaceRequest_Validate_Success(t *testing.T) {
	name := "Updated"
	req := &UpdateWorkspaceRequest{WorkspaceID: "ws-1", Name: &name, UpdatedBy: "user-1"}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpdateWorkspaceRequest_Validate_MissingWorkspaceID(t *testing.T) {
	req := &UpdateWorkspaceRequest{UpdatedBy: "user-1"}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestUpdateWorkspaceRequest_Validate_EmptyName(t *testing.T) {
	empty := "  "
	req := &UpdateWorkspaceRequest{WorkspaceID: "ws-1", Name: &empty, UpdatedBy: "user-1"}
	if err := req.Validate(); err == nil {
		t.Fatal("expected error for empty name")
	}
}

// --- AddMemberRequest.Validate ---

func TestAddMemberRequest_Validate_Success(t *testing.T) {
	req := &AddMemberRequest{WorkspaceID: "ws-1", UserID: "u-1", Role: collabdomain.RoleEditor, AddedBy: "admin-1"}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddMemberRequest_Validate_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		req  AddMemberRequest
	}{
		{"missing workspace_id", AddMemberRequest{UserID: "u", Role: collabdomain.RoleViewer, AddedBy: "a"}},
		{"missing user_id", AddMemberRequest{WorkspaceID: "ws", Role: collabdomain.RoleViewer, AddedBy: "a"}},
		{"missing role", AddMemberRequest{WorkspaceID: "ws", UserID: "u", AddedBy: "a"}},
		{"missing added_by", AddMemberRequest{WorkspaceID: "ws", UserID: "u", Role: collabdomain.RoleViewer}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
		})
	}
}

// --- RemoveMemberRequest.Validate ---

func TestRemoveMemberRequest_Validate_Success(t *testing.T) {
	req := &RemoveMemberRequest{WorkspaceID: "ws-1", UserID: "u-1", RemovedBy: "admin-1"}
	if err := req.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRemoveMemberRequest_Validate_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		req  RemoveMemberRequest
	}{
		{"missing workspace_id", RemoveMemberRequest{UserID: "u", RemovedBy: "a"}},
		{"missing user_id", RemoveMemberRequest{WorkspaceID: "ws", RemovedBy: "a"}},
		{"missing removed_by", RemoveMemberRequest{WorkspaceID: "ws", UserID: "u"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
		})
	}
}

// --- Create ---

func TestCreate_Success(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	resp, err := svc.Create(context.Background(), &CreateWorkspaceRequest{
		Name:    "Test Workspace",
		OwnerID: "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if resp.Name != "Test Workspace" {
		t.Fatalf("expected 'Test Workspace', got %s", resp.Name)
	}
	if resp.OwnerID != "user-1" {
		t.Fatalf("expected owner user-1, got %s", resp.OwnerID)
	}
}

func TestCreate_NilRequest(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	_, err := svc.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestCreate_PersistError(t *testing.T) {
	wsRepo := &mockWsRepo{
		createFn: func(ctx context.Context, ws *collabdomain.Workspace) error {
			return errors.New("db error")
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	_, err := svc.Create(context.Background(), &CreateWorkspaceRequest{
		Name:    "WS",
		OwnerID: "user-1",
	})
	if err == nil {
		t.Fatal("expected error for persist failure")
	}
}

func TestCreate_DomainServiceError(t *testing.T) {
	domainSvc := &mockWsDomainService{
		createWorkspaceFn: func(ctx context.Context, ws *collabdomain.Workspace) error {
			return errors.New("domain error")
		},
	}
	svc := newTestWorkspaceAppService(domainSvc, nil, nil)
	_, err := svc.Create(context.Background(), &CreateWorkspaceRequest{
		Name:    "WS",
		OwnerID: "user-1",
	})
	if err == nil {
		t.Fatal("expected error for domain service failure")
	}
}

// --- Update ---

func TestUpdate_Success(t *testing.T) {
	name := "Updated Name"
	svc := newTestWorkspaceAppService(nil, nil, nil)
	resp, err := svc.Update(context.Background(), &UpdateWorkspaceRequest{
		WorkspaceID: "ws-1",
		Name:        &name,
		UpdatedBy:   "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Name != "Updated Name" {
		t.Fatalf("expected 'Updated Name', got %s", resp.Name)
	}
}

func TestUpdate_NilRequest(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	_, err := svc.Update(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestUpdate_NotFound(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return nil, errors.New("not found")
		},
	}
	name := "X"
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	_, err := svc.Update(context.Background(), &UpdateWorkspaceRequest{
		WorkspaceID: "ws-missing",
		Name:        &name,
		UpdatedBy:   "user-1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestUpdate_PermissionDenied(t *testing.T) {
	domainSvc := &mockWsDomainService{
		checkPermissionFn: func(ctx context.Context, userID, workspaceID string, action collabdomain.Action) error {
			return errors.New("denied")
		},
	}
	name := "X"
	svc := newTestWorkspaceAppService(domainSvc, nil, nil)
	_, err := svc.Update(context.Background(), &UpdateWorkspaceRequest{
		WorkspaceID: "ws-1",
		Name:        &name,
		UpdatedBy:   "user-no-perm",
	})
	if err == nil {
		t.Fatal("expected permission denied error")
	}
}

// --- Delete ---

func TestDelete_Success_Owner(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return &collabdomain.Workspace{ID: id, OwnerID: "owner-1"}, nil
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	err := svc.Delete(context.Background(), "ws-1", "owner-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return nil, errors.New("not found")
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	err := svc.Delete(context.Background(), "ws-missing", "user-1")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestDelete_PermissionDenied(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return &collabdomain.Workspace{ID: id, OwnerID: "owner-1"}, nil
		},
	}
	domainSvc := &mockWsDomainService{
		checkPermissionFn: func(ctx context.Context, userID, workspaceID string, action collabdomain.Action) error {
			return errors.New("denied")
		},
	}
	svc := newTestWorkspaceAppService(domainSvc, wsRepo, nil)
	err := svc.Delete(context.Background(), "ws-1", "user-not-owner")
	if err == nil {
		t.Fatal("expected permission denied error")
	}
}

func TestDelete_EmptyWorkspaceID(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	err := svc.Delete(context.Background(), "", "user-1")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDelete_EmptyDeletedBy(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	err := svc.Delete(context.Background(), "ws-1", "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// --- GetByID ---

func TestGetByID_Success(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	resp, err := svc.GetByID(context.Background(), "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "ws-1" {
		t.Fatalf("expected ws-1, got %s", resp.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return nil, errors.New("not found")
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	_, err := svc.GetByID(context.Background(), "ws-missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetByID_EmptyID(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	_, err := svc.GetByID(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// --- ListByUser ---

func TestListByUser_Success(t *testing.T) {
	now := time.Now()
	wsRepo := &mockWsRepo{
		listByUserFn: func(ctx context.Context, userID string, p commontypes.Pagination) ([]*collabdomain.Workspace, int, error) {
			return []*collabdomain.Workspace{
				{ID: "ws-1", Name: "WS1", OwnerID: userID, CreatedAt: now, UpdatedAt: now},
			}, 1, nil
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	results, total, err := svc.ListByUser(context.Background(), "user-1", commontypes.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestListByUser_EmptyUserID(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	_, _, err := svc.ListByUser(context.Background(), "", commontypes.Pagination{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestListByUser_PaginationDefaults(t *testing.T) {
	var capturedPagination commontypes.Pagination
	wsRepo := &mockWsRepo{
		listByUserFn: func(ctx context.Context, userID string, p commontypes.Pagination) ([]*collabdomain.Workspace, int, error) {
			capturedPagination = p
			return nil, 0, nil
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	_, _, _ = svc.ListByUser(context.Background(), "user-1", commontypes.Pagination{Page: 0, PageSize: 0})
	if capturedPagination.Page != 1 {
		t.Fatalf("expected page 1, got %d", capturedPagination.Page)
	}
	if capturedPagination.PageSize != 20 {
		t.Fatalf("expected pageSize 20, got %d", capturedPagination.PageSize)
	}
}

// --- AddMember ---

func TestAddMember_Success(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	err := svc.AddMember(context.Background(), &AddMemberRequest{
		WorkspaceID: "ws-1",
		UserID:      "user-new",
		Role:        collabdomain.RoleEditor,
		AddedBy:     "admin-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddMember_NilRequest(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	err := svc.AddMember(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAddMember_WorkspaceNotFound(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return nil, errors.New("not found")
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	err := svc.AddMember(context.Background(), &AddMemberRequest{
		WorkspaceID: "ws-missing",
		UserID:      "user-1",
		Role:        collabdomain.RoleViewer,
		AddedBy:     "admin-1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestAddMember_PermissionDenied(t *testing.T) {
	domainSvc := &mockWsDomainService{
		checkPermissionFn: func(ctx context.Context, userID, workspaceID string, action collabdomain.Action) error {
			return errors.New("denied")
		},
	}
	svc := newTestWorkspaceAppService(domainSvc, nil, nil)
	err := svc.AddMember(context.Background(), &AddMemberRequest{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Role:        collabdomain.RoleViewer,
		AddedBy:     "user-no-perm",
	})
	if err == nil {
		t.Fatal("expected permission denied error")
	}
}

// --- RemoveMember ---

func TestRemoveMember_Success(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return &collabdomain.Workspace{ID: id, OwnerID: "owner-1"}, nil
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	err := svc.RemoveMember(context.Background(), &RemoveMemberRequest{
		WorkspaceID: "ws-1",
		UserID:      "user-to-remove",
		RemovedBy:   "admin-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveMember_CannotRemoveOwner(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return &collabdomain.Workspace{ID: id, OwnerID: "owner-1"}, nil
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	err := svc.RemoveMember(context.Background(), &RemoveMemberRequest{
		WorkspaceID: "ws-1",
		UserID:      "owner-1",
		RemovedBy:   "admin-1",
	})
	if err == nil {
		t.Fatal("expected error when removing owner")
	}
}

func TestRemoveMember_NilRequest(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	err := svc.RemoveMember(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestRemoveMember_WorkspaceNotFound(t *testing.T) {
	wsRepo := &mockWsRepo{
		getByIDFn: func(ctx context.Context, id string) (*collabdomain.Workspace, error) {
			return nil, errors.New("not found")
		},
	}
	svc := newTestWorkspaceAppService(nil, wsRepo, nil)
	err := svc.RemoveMember(context.Background(), &RemoveMemberRequest{
		WorkspaceID: "ws-missing",
		UserID:      "user-1",
		RemovedBy:   "admin-1",
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

// --- ListMembers ---

func TestListMembers_Success(t *testing.T) {
	now := time.Now()
	memberRepo := &mockMemberRepo{
		listFn: func(ctx context.Context, workspaceID string, p commontypes.Pagination) ([]*MemberResponse, int, error) {
			return []*MemberResponse{
				{UserID: "u-1", Role: collabdomain.RoleAdmin, JoinedAt: now},
				{UserID: "u-2", Role: collabdomain.RoleEditor, JoinedAt: now},
			}, 2, nil
		},
	}
	svc := newTestWorkspaceAppService(nil, nil, memberRepo)
	members, total, err := svc.ListMembers(context.Background(), "ws-1", commontypes.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestListMembers_EmptyWorkspaceID(t *testing.T) {
	svc := newTestWorkspaceAppService(nil, nil, nil)
	_, _, err := svc.ListMembers(context.Background(), "", commontypes.Pagination{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected validation error for empty workspace_id")
	}
}

func TestListMembers_PaginationDefaults(t *testing.T) {
	var capturedPagination commontypes.Pagination
	memberRepo := &mockMemberRepo{
		listFn: func(ctx context.Context, workspaceID string, p commontypes.Pagination) ([]*MemberResponse, int, error) {
			capturedPagination = p
			return nil, 0, nil
		},
	}
	svc := newTestWorkspaceAppService(nil, nil, memberRepo)
	_, _, _ = svc.ListMembers(context.Background(), "ws-1", commontypes.Pagination{Page: -1, PageSize: 999})
	if capturedPagination.Page != 1 {
		t.Fatalf("expected page 1, got %d", capturedPagination.Page)
	}
	if capturedPagination.PageSize != 20 {
		t.Fatalf("expected pageSize 20, got %d", capturedPagination.PageSize)
	}
}

func TestListMembers_RepoError(t *testing.T) {
	memberRepo := &mockMemberRepo{
		listFn: func(ctx context.Context, workspaceID string, p commontypes.Pagination) ([]*MemberResponse, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	svc := newTestWorkspaceAppService(nil, nil, memberRepo)
	_, _, err := svc.ListMembers(context.Background(), "ws-1", commontypes.Pagination{Page: 1, PageSize: 10})
	if err == nil {
		t.Fatal("expected error for repo failure")
	}
}

//Personal.AI order the ending
