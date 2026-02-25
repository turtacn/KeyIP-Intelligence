// Phase 11 - File 259: internal/interfaces/http/handlers/collaboration_handler_test.go
// 实现协作空间 HTTP Handler 单元测试。
//
// 实现要求:
// * 功能定位：验证 CollaborationHandler 全部 HTTP 方法的行为正确性
// * 测试用例：
//   - TestCreateWorkspace_Success
//   - TestCreateWorkspace_MissingName
//   - TestCreateWorkspace_InvalidBody
//   - TestGetWorkspace_Success
//   - TestGetWorkspace_NotFound
//   - TestListWorkspaces_Success
//   - TestUpdateWorkspace_Success
//   - TestDeleteWorkspace_Success
//   - TestDeleteWorkspace_NotFound
//   - TestShareDocument_Success
//   - TestShareDocument_MissingDocumentID
//   - TestListSharedDocuments_Success
//   - TestInviteMember_Success
//   - TestInviteMember_InvalidRole
//   - TestInviteMember_MissingIdentifier
//   - TestRemoveMember_Success
//   - TestUpdateMemberRole_Success
//   - TestUpdateMemberRole_InvalidRole
// * Mock 依赖：mockWorkspaceService、mockSharingService、mockLogger
// * 断言验证：HTTP 状态码、响应 JSON 字段、Mock 调用参数与次数
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	pkgtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// --- Mock Services ---

type mockWorkspaceService struct {
	mock.Mock
}

func (m *mockWorkspaceService) Create(ctx context.Context, input *collaboration.CreateWorkspaceInput) (*collaboration.WorkspaceOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*collaboration.WorkspaceOutput), args.Error(1)
}

func (m *mockWorkspaceService) GetByID(ctx context.Context, id, userID string) (*collaboration.WorkspaceOutput, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*collaboration.WorkspaceOutput), args.Error(1)
}

func (m *mockWorkspaceService) List(ctx context.Context, input *collaboration.ListWorkspacesInput) (*collaboration.ListWorkspacesOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*collaboration.ListWorkspacesOutput), args.Error(1)
}

func (m *mockWorkspaceService) Update(ctx context.Context, input *collaboration.UpdateWorkspaceInput) (*collaboration.WorkspaceOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*collaboration.WorkspaceOutput), args.Error(1)
}

func (m *mockWorkspaceService) Delete(ctx context.Context, id, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *mockWorkspaceService) InviteMember(ctx context.Context, input *collaboration.InviteMemberInput) (*collaboration.MemberOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*collaboration.MemberOutput), args.Error(1)
}

func (m *mockWorkspaceService) RemoveMember(ctx context.Context, workspaceID, memberID, userID string) error {
	args := m.Called(ctx, workspaceID, memberID, userID)
	return args.Error(0)
}

func (m *mockWorkspaceService) UpdateMemberRole(ctx context.Context, workspaceID, memberID, role, userID string) error {
	args := m.Called(ctx, workspaceID, memberID, role, userID)
	return args.Error(0)
}

type mockSharingService struct {
	mock.Mock
}

func (m *mockSharingService) ShareDocument(ctx context.Context, input *collaboration.ShareDocumentInput) (*collaboration.SharedDocumentOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*collaboration.SharedDocumentOutput), args.Error(1)
}

func (m *mockSharingService) ListDocuments(ctx context.Context, input *collaboration.ListSharedDocumentsInput) (*collaboration.ListSharedDocumentsOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*collaboration.ListSharedDocumentsOutput), args.Error(1)
}

// --- Helper ---

func newTestCollabHandler() (*CollaborationHandler, *mockWorkspaceService, *mockSharingService) {
	wsSvc := new(mockWorkspaceService)
	shareSvc := new(mockSharingService)
	logger := new(mockTestLogger)
	h := NewCollaborationHandler(wsSvc, shareSvc, logger)
	return h, wsSvc, shareSvc
}

type mockTestLogger struct{ mock.Mock }

func (m *mockTestLogger) Error(msg string, keysAndValues ...interface{}) {}
func (m *mockTestLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockTestLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockTestLogger) Warn(msg string, keysAndValues ...interface{})  {}

func withUserID(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), pkgtypes.ContextKeyUserID, userID)
	return r.WithContext(ctx)
}

func TestCreateWorkspace_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	expected := &collaboration.WorkspaceOutput{
		ID:   "ws-001",
		Name: "Test Workspace",
	}
	wsSvc.On("Create", mock.Anything, mock.AnythingOfType("*collaboration.CreateWorkspaceInput")).Return(expected, nil)

	body, _ := json.Marshal(CreateWorkspaceRequest{Name: "Test Workspace", Visibility: "private"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader(body))
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.CreateWorkspace(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp collaboration.WorkspaceOutput
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	assert.Equal(t, "ws-001", resp.ID)
	wsSvc.AssertExpectations(t)
}

func TestCreateWorkspace_MissingName(t *testing.T) {
	h, _, _ := newTestCollabHandler()

	body, _ := json.Marshal(CreateWorkspaceRequest{Visibility: "private"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader(body))
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.CreateWorkspace(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreateWorkspace_InvalidBody(t *testing.T) {
	h, _, _ := newTestCollabHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader([]byte("not-json")))
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.CreateWorkspace(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetWorkspace_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	expected := &collaboration.WorkspaceOutput{ID: "ws-001", Name: "My WS"}
	wsSvc.On("GetByID", mock.Anything, "ws-001", "user-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-001", nil)
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.GetWorkspace(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp collaboration.WorkspaceOutput
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	assert.Equal(t, "ws-001", resp.ID)
	wsSvc.AssertExpectations(t)
}

func TestGetWorkspace_NotFound(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	wsSvc.On("GetByID", mock.Anything, "ws-999", "user-001").Return(nil, errors.NewNotFoundError("workspace not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-999", nil)
	req.SetPathValue("id", "ws-999")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.GetWorkspace(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestListWorkspaces_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	expected := &collaboration.ListWorkspacesOutput{
		Items:    []*collaboration.WorkspaceOutput{{ID: "ws-001"}},
		Total:    1,
		Page:     1,
		PageSize: 20,
	}
	wsSvc.On("List", mock.Anything, mock.AnythingOfType("*collaboration.ListWorkspacesInput")).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces?page=1&page_size=20", nil)
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.ListWorkspaces(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestUpdateWorkspace_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	name := "Updated Name"
	expected := &collaboration.WorkspaceOutput{ID: "ws-001", Name: name}
	wsSvc.On("Update", mock.Anything, mock.AnythingOfType("*collaboration.UpdateWorkspaceInput")).Return(expected, nil)

	body, _ := json.Marshal(UpdateWorkspaceRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-001", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.UpdateWorkspace(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestDeleteWorkspace_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	wsSvc.On("Delete", mock.Anything, "ws-001", "user-001").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-001", nil)
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.DeleteWorkspace(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestDeleteWorkspace_NotFound(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	wsSvc.On("Delete", mock.Anything, "ws-999", "user-001").Return(errors.NewNotFoundError("workspace not found"))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-999", nil)
	req.SetPathValue("id", "ws-999")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.DeleteWorkspace(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestShareDocument_Success(t *testing.T) {
	h, _, shareSvc := newTestCollabHandler()

	expected := &collaboration.SharedDocumentOutput{
		ID:         "sd-001",
		DocumentID: "doc-001",
	}
	shareSvc.On("ShareDocument", mock.Anything, mock.AnythingOfType("*collaboration.ShareDocumentInput")).Return(expected, nil)

	body, _ := json.Marshal(ShareDocumentRequest{
		DocumentID:      "doc-001",
		EnableWatermark: true,
		MaxDownloads:    5,
		ExpiresInHours:  72,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-001/documents", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.ShareDocument(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	shareSvc.AssertExpectations(t)
}

func TestShareDocument_MissingDocumentID(t *testing.T) {
	h, _, _ := newTestCollabHandler()

	body, _ := json.Marshal(ShareDocumentRequest{EnableWatermark: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-001/documents", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.ShareDocument(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListSharedDocuments_Success(t *testing.T) {
	h, _, shareSvc := newTestCollabHandler()

	expected := &collaboration.ListSharedDocumentsOutput{
		Items: []*collaboration.SharedDocumentOutput{{ID: "sd-001"}},
		Total: 1,
	}
	shareSvc.On("ListDocuments", mock.Anything, mock.AnythingOfType("*collaboration.ListSharedDocumentsInput")).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-001/documents", nil)
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.ListSharedDocuments(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	shareSvc.AssertExpectations(t)
}

func TestInviteMember_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	expected := &collaboration.MemberOutput{
		UserID: "user-002",
		Role:   "editor",
	}
	wsSvc.On("InviteMember", mock.Anything, mock.AnythingOfType("*collaboration.InviteMemberInput")).Return(expected, nil)

	body, _ := json.Marshal(InviteMemberRequest{UserID: "user-002", Role: "editor"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-001/members", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.InviteMember(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestInviteMember_InvalidRole(t *testing.T) {
	h, _, _ := newTestCollabHandler()

	body, _ := json.Marshal(InviteMemberRequest{UserID: "user-002", Role: "superadmin"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-001/members", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.InviteMember(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInviteMember_MissingIdentifier(t *testing.T) {
	h, _, _ := newTestCollabHandler()

	body, _ := json.Marshal(InviteMemberRequest{Role: "editor"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-001/members", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.InviteMember(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRemoveMember_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	wsSvc.On("RemoveMember", mock.Anything, "ws-001", "user-002", "user-001").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-001/members/user-002", nil)
	req.SetPathValue("id", "ws-001")
	req.SetPathValue("memberId", "user-002")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.RemoveMember(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestUpdateMemberRole_Success(t *testing.T) {
	h, wsSvc, _ := newTestCollabHandler()

	wsSvc.On("UpdateMemberRole", mock.Anything, "ws-001", "user-002", "admin", "user-001").Return(nil)

	body, _ := json.Marshal(UpdateMemberRoleRequest{Role: "admin"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-001/members/user-002/role", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req.SetPathValue("memberId", "user-002")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.UpdateMemberRole(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	wsSvc.AssertExpectations(t)
}

func TestUpdateMemberRole_InvalidRole(t *testing.T) {
	h, _, _ := newTestCollabHandler()

	body, _ := json.Marshal(UpdateMemberRoleRequest{Role: "god"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-001/members/user-002/role", bytes.NewReader(body))
	req.SetPathValue("id", "ws-001")
	req.SetPathValue("memberId", "user-002")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.UpdateMemberRole(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	page, pageSize := parsePagination(req)
	assert.Equal(t, 1, page)
	assert.Equal(t, 20, pageSize)
}

func TestParsePagination_Custom(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?page=3&page_size=50", nil)
	page, pageSize := parsePagination(req)
	assert.Equal(t, 3, page)
	assert.Equal(t, 50, pageSize)
}

func TestParsePagination_ExceedsMax(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?page_size=200", nil)
	_, pageSize := parsePagination(req)
	assert.Equal(t, 20, pageSize) // exceeds 100, falls back to default
}

func TestIsValidRole(t *testing.T) {
	assert.True(t, isValidRole("owner"))
	assert.True(t, isValidRole("admin"))
	assert.True(t, isValidRole("editor"))
	assert.True(t, isValidRole("viewer"))
	assert.False(t, isValidRole("superadmin"))
	assert.False(t, isValidRole(""))
}

//Personal.AI order the ending

