// Real tests for collaboration HTTP handler.
// Tests request parsing, validation, error responses, and successful responses.

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// mockWorkspaceService implements collaboration.WorkspaceService for testing.
type mockWorkspaceService struct {
	createFn           func(context.Context, *collaboration.CreateWorkspaceRequest) (*collaboration.WorkspaceResponse, error)
	getByIDFn          func(context.Context, string) (*collaboration.WorkspaceResponse, error)
	listFn             func(context.Context, *collaboration.ListWorkspacesInput) (*collaboration.ListWorkspacesResult, error)
	updateFn           func(context.Context, *collaboration.UpdateWorkspaceRequest) (*collaboration.WorkspaceResponse, error)
	deleteFn           func(context.Context, string, string) error
	inviteMemberFn     func(context.Context, *collaboration.InviteMemberInput) (*collaboration.MemberResponse, error)
	removeMemberFn     func(context.Context, string, string, string) error
	updateMemberRoleFn func(context.Context, string, string, string, string) error
	listMembersFn      func(context.Context, string, commontypes.Pagination) ([]*collaboration.MemberResponse, int, error)
}

func (m *mockWorkspaceService) Create(ctx context.Context, req *collaboration.CreateWorkspaceRequest) (*collaboration.WorkspaceResponse, error) {
	return m.createFn(ctx, req)
}
func (m *mockWorkspaceService) GetByID(ctx context.Context, id string) (*collaboration.WorkspaceResponse, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockWorkspaceService) List(ctx context.Context, in *collaboration.ListWorkspacesInput) (*collaboration.ListWorkspacesResult, error) {
	return m.listFn(ctx, in)
}
func (m *mockWorkspaceService) Update(ctx context.Context, req *collaboration.UpdateWorkspaceRequest) (*collaboration.WorkspaceResponse, error) {
	return m.updateFn(ctx, req)
}
func (m *mockWorkspaceService) Delete(ctx context.Context, workspaceID, deletedBy string) error {
	return m.deleteFn(ctx, workspaceID, deletedBy)
}
func (m *mockWorkspaceService) InviteMember(ctx context.Context, in *collaboration.InviteMemberInput) (*collaboration.MemberResponse, error) {
	return m.inviteMemberFn(ctx, in)
}
func (m *mockWorkspaceService) RemoveMember(ctx context.Context, workspaceID, memberID, userID string) error {
	return m.removeMemberFn(ctx, workspaceID, memberID, userID)
}
func (m *mockWorkspaceService) UpdateMemberRole(ctx context.Context, workspaceID, memberID, role, userID string) error {
	return m.updateMemberRoleFn(ctx, workspaceID, memberID, role, userID)
}
func (m *mockWorkspaceService) ListMembers(ctx context.Context, workspaceID string, pagination commontypes.Pagination) ([]*collaboration.MemberResponse, int, error) {
	return m.listMembersFn(ctx, workspaceID, pagination)
}

// mockSharingService implements collaboration.SharingService for testing.
type mockSharingService struct {
	shareDocumentFn func(context.Context, *collaboration.ShareDocumentInput) (*collaboration.SharedDocument, error)
	listDocumentsFn func(context.Context, *collaboration.ListSharedDocumentsInput) (*collaboration.ListSharedDocumentsResult, error)
	revokeFn        func(context.Context, string, string) error
	shareFn         func(context.Context, *collaboration.ShareRequest) (*collaboration.ShareResponse, error)
	listSharesFn    func(context.Context, string, ...collaboration.ListSharesOption) ([]*collaboration.ShareRecord, int, error)
	getShareLinkFn  func(context.Context, string) (string, error)
	validateFn      func(context.Context, string) (*collaboration.ShareInfo, error)
}

func (m *mockSharingService) ShareDocument(ctx context.Context, in *collaboration.ShareDocumentInput) (*collaboration.SharedDocument, error) {
	return m.shareDocumentFn(ctx, in)
}
func (m *mockSharingService) ListDocuments(ctx context.Context, in *collaboration.ListSharedDocumentsInput) (*collaboration.ListSharedDocumentsResult, error) {
	return m.listDocumentsFn(ctx, in)
}
func (m *mockSharingService) Revoke(ctx context.Context, shareID, revokedBy string) error {
	return m.revokeFn(ctx, shareID, revokedBy)
}
func (m *mockSharingService) Share(ctx context.Context, req *collaboration.ShareRequest) (*collaboration.ShareResponse, error) {
	return m.shareFn(ctx, req)
}
func (m *mockSharingService) ListShares(ctx context.Context, workspaceID string, opts ...collaboration.ListSharesOption) ([]*collaboration.ShareRecord, int, error) {
	return m.listSharesFn(ctx, workspaceID, opts...)
}
func (m *mockSharingService) GetShareLink(ctx context.Context, shareID string) (string, error) {
	return m.getShareLinkFn(ctx, shareID)
}
func (m *mockSharingService) ValidateShareToken(ctx context.Context, token string) (*collaboration.ShareInfo, error) {
	return m.validateFn(ctx, token)
}

func TestCollaborationHandler_CreateWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			createFn: func(_ context.Context, req *collaboration.CreateWorkspaceRequest) (*collaboration.WorkspaceResponse, error) {
				assert.Equal(t, "My Workspace", req.Name)
				return &collaboration.WorkspaceResponse{ID: "ws-1", Name: "My Workspace"}, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "My Workspace", "description": "desc"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateWorkspace(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp collaboration.WorkspaceResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "ws-1", resp.ID)
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"description": "desc"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateWorkspace(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json body", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader([]byte("{bad")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateWorkspace(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			createFn: func(_ context.Context, _ *collaboration.CreateWorkspaceRequest) (*collaboration.WorkspaceResponse, error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateWorkspace(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestCollaborationHandler_GetWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			getByIDFn: func(_ context.Context, id string) (*collaboration.WorkspaceResponse, error) {
				assert.Equal(t, "ws-1", id)
				return &collaboration.WorkspaceResponse{ID: "ws-1", Name: "test"}, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1", nil)
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.GetWorkspace(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp collaboration.WorkspaceResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "ws-1", resp.ID)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/", nil)
		rec := httptest.NewRecorder()

		h.GetWorkspace(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			getByIDFn: func(_ context.Context, _ string) (*collaboration.WorkspaceResponse, error) {
				return nil, errors.NewNotFound("workspace not found")
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.GetWorkspace(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestCollaborationHandler_ListWorkspaces(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			listFn: func(_ context.Context, in *collaboration.ListWorkspacesInput) (*collaboration.ListWorkspacesResult, error) {
				assert.Equal(t, 1, in.Page)
				assert.Equal(t, 20, in.PageSize)
				return &collaboration.ListWorkspacesResult{
					Workspaces: []*collaboration.WorkspaceResponse{{ID: "ws-1", Name: "test"}},
					Total:      1,
					Page:       1,
					PageSize:   20,
				}, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
		rec := httptest.NewRecorder()

		h.ListWorkspaces(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp collaboration.ListWorkspacesResult
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp.Workspaces, 1)
		assert.Equal(t, 1, resp.Total)
	})

	t.Run("with query params", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			listFn: func(_ context.Context, in *collaboration.ListWorkspacesInput) (*collaboration.ListWorkspacesResult, error) {
				assert.Equal(t, 2, in.Page)
				assert.Equal(t, 10, in.PageSize)
				return &collaboration.ListWorkspacesResult{Workspaces: []*collaboration.WorkspaceResponse{}, Total: 0, Page: 2, PageSize: 10}, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces?page=2&page_size=10", nil)
		rec := httptest.NewRecorder()

		h.ListWorkspaces(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			listFn: func(_ context.Context, _ *collaboration.ListWorkspacesInput) (*collaboration.ListWorkspacesResult, error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
		rec := httptest.NewRecorder()

		h.ListWorkspaces(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestCollaborationHandler_UpdateWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			updateFn: func(_ context.Context, req *collaboration.UpdateWorkspaceRequest) (*collaboration.WorkspaceResponse, error) {
				assert.Equal(t, "ws-1", req.WorkspaceID)
				return &collaboration.WorkspaceResponse{ID: "ws-1", Name: "updated"}, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "updated"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.UpdateWorkspace(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdateWorkspace(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-1", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("id", "ws-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdateWorkspace(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_DeleteWorkspace(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			deleteFn: func(_ context.Context, id, _ string) error {
				assert.Equal(t, "ws-1", id)
				return nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1", nil)
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.DeleteWorkspace(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/", nil)
		rec := httptest.NewRecorder()

		h.DeleteWorkspace(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			deleteFn: func(_ context.Context, _, _ string) error {
				return errors.NewNotFound("workspace not found")
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.DeleteWorkspace(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestCollaborationHandler_ShareDocument(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		shSvc := &mockSharingService{
			shareDocumentFn: func(_ context.Context, in *collaboration.ShareDocumentInput) (*collaboration.SharedDocument, error) {
				assert.Equal(t, "ws-1", in.WorkspaceID)
				assert.Equal(t, "doc-1", in.DocumentID)
				assert.True(t, in.EnableWatermark)
				return &collaboration.SharedDocument{ID: "share-1", DocumentID: "doc-1"}, nil
			},
		}
		h := NewCollaborationHandler(&mockWorkspaceService{}, shSvc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"document_id":      "doc-1",
			"enable_watermark": true,
			"max_downloads":    5,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/documents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.ShareDocument(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp collaboration.SharedDocument
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "share-1", resp.ID)
	})

	t.Run("missing document_id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"enable_watermark": true})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/documents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.ShareDocument(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing workspace id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"document_id": "doc-1"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces//documents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ShareDocument(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/documents", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("id", "ws-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.ShareDocument(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_ListSharedDocuments(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		shSvc := &mockSharingService{
			listDocumentsFn: func(_ context.Context, in *collaboration.ListSharedDocumentsInput) (*collaboration.ListSharedDocumentsResult, error) {
				assert.Equal(t, "ws-1", in.WorkspaceID)
				assert.Equal(t, 1, in.Page)
				assert.Equal(t, 20, in.PageSize)
				return &collaboration.ListSharedDocumentsResult{
					Documents: []*collaboration.SharedDocument{{ID: "share-1"}},
					Total:     1,
					Page:      1,
					PageSize:  20,
				}, nil
			},
		}
		h := NewCollaborationHandler(&mockWorkspaceService{}, shSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/documents", nil)
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.ListSharedDocuments(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp collaboration.ListSharedDocumentsResult
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp.Documents, 1)
	})

	t.Run("missing workspace id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces//documents", nil)
		rec := httptest.NewRecorder()

		h.ListSharedDocuments(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_InviteMember(t *testing.T) {
	t.Run("success with user_id", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			inviteMemberFn: func(_ context.Context, in *collaboration.InviteMemberInput) (*collaboration.MemberResponse, error) {
				assert.Equal(t, "ws-1", in.WorkspaceID)
				assert.Equal(t, "user-2", in.InviteeID)
				assert.Equal(t, "editor", in.Role)
				return &collaboration.MemberResponse{UserID: "user-2"}, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"user_id": "user-2", "role": "editor"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/members", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.InviteMember(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("success with email", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			inviteMemberFn: func(_ context.Context, in *collaboration.InviteMemberInput) (*collaboration.MemberResponse, error) {
				assert.Equal(t, "user@example.com", in.InviteeEmail)
				return &collaboration.MemberResponse{UserID: "user-3"}, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"email": "user@example.com", "role": "viewer"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/members", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.InviteMember(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("missing user_id and email", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"role": "editor"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/members", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.InviteMember(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid role", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"user_id": "user-2", "role": "superadmin"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/members", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.InviteMember(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing workspace id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"user_id": "user-2", "role": "editor"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces//members", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.InviteMember(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws-1/members", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("id", "ws-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.InviteMember(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_RemoveMember(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			removeMemberFn: func(_ context.Context, workspaceID, memberID, _ string) error {
				assert.Equal(t, "ws-1", workspaceID)
				assert.Equal(t, "member-1", memberID)
				return nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/members/member-1", nil)
		req.SetPathValue("id", "ws-1")
		req.SetPathValue("memberId", "member-1")
		rec := httptest.NewRecorder()

		h.RemoveMember(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing ids", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces//members/", nil)
		rec := httptest.NewRecorder()

		h.RemoveMember(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_UpdateMemberRole(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			updateMemberRoleFn: func(_ context.Context, workspaceID, memberID, role, _ string) error {
				assert.Equal(t, "ws-1", workspaceID)
				assert.Equal(t, "member-1", memberID)
				assert.Equal(t, "admin", role)
				return nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"role": "admin"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-1/members/member-1/role", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		req.SetPathValue("memberId", "member-1")
		rec := httptest.NewRecorder()

		h.UpdateMemberRole(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("invalid role", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"role": "superadmin"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-1/members/member-1/role", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "ws-1")
		req.SetPathValue("memberId", "member-1")
		rec := httptest.NewRecorder()

		h.UpdateMemberRole(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing ids", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"role": "admin"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces//members//role", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdateMemberRole(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws-1/members/member-1/role", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("id", "ws-1")
		req.SetPathValue("memberId", "member-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdateMemberRole(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_ListMembers(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		wsSvc := &mockWorkspaceService{
			listMembersFn: func(_ context.Context, workspaceID string, pagination commontypes.Pagination) ([]*collaboration.MemberResponse, int, error) {
				assert.Equal(t, "ws-1", workspaceID)
				assert.Equal(t, 1, pagination.Page)
				assert.Equal(t, 20, pagination.PageSize)
				return []*collaboration.MemberResponse{{UserID: "user-1"}}, 1, nil
			},
		}
		h := NewCollaborationHandler(wsSvc, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/members", nil)
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.ListMembers(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, float64(1), resp["total"])
	})

	t.Run("missing workspace id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces//members", nil)
		rec := httptest.NewRecorder()

		h.ListMembers(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_GetSharedResource(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		shSvc := &mockSharingService{
			listDocumentsFn: func(_ context.Context, in *collaboration.ListSharedDocumentsInput) (*collaboration.ListSharedDocumentsResult, error) {
				assert.Equal(t, "ws-1", in.WorkspaceID)
				return &collaboration.ListSharedDocumentsResult{
					Documents: []*collaboration.SharedDocument{{ID: "share-1"}},
					Total:     1,
					Page:      1,
					PageSize:  20,
				}, nil
			},
		}
		h := NewCollaborationHandler(&mockWorkspaceService{}, shSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/shared-resource", nil)
		req.SetPathValue("id", "ws-1")
		rec := httptest.NewRecorder()

		h.GetSharedResource(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces//shared-resource", nil)
		rec := httptest.NewRecorder()

		h.GetSharedResource(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestCollaborationHandler_RevokeShareLink(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		shSvc := &mockSharingService{
			revokeFn: func(_ context.Context, shareID, _ string) error {
				assert.Equal(t, "share-1", shareID)
				return nil
			},
		}
		h := NewCollaborationHandler(&mockWorkspaceService{}, shSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/shares/share-1", nil)
		req.SetPathValue("shareId", "share-1")
		rec := httptest.NewRecorder()

		h.RevokeShareLink(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing shareId", func(t *testing.T) {
		h := NewCollaborationHandler(&mockWorkspaceService{}, &mockSharingService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/shares/", nil)
		rec := httptest.NewRecorder()

		h.RevokeShareLink(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		shSvc := &mockSharingService{
			revokeFn: func(_ context.Context, _ string, _ string) error {
				return errors.NewNotFound("share not found")
			},
		}
		h := NewCollaborationHandler(&mockWorkspaceService{}, shSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws-1/shares/nonexistent", nil)
		req.SetPathValue("shareId", "nonexistent")
		rec := httptest.NewRecorder()

		h.RevokeShareLink(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

//Personal.AI order the ending
