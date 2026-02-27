// Phase 11 - File 258: internal/interfaces/http/handlers/collaboration_handler.go
// 实现协作空间 HTTP Handler。
//
// 实现要求:
// * 功能定位：处理协作空间相关的 HTTP 请求，包括工作空间 CRUD、文件共享、权限管理、合作伙伴邀请
// * 核心实现：
//   - 定义 CollaborationHandler 结构体，注入 collaboration 应用服务与 logger
//   - 实现 CreateWorkspace：创建协作工作空间
//   - 实现 GetWorkspace：获取工作空间详情
//   - 实现 ListWorkspaces：列出用户可访问的工作空间
//   - 实现 UpdateWorkspace：更新工作空间配置
//   - 实现 DeleteWorkspace：删除工作空间（软删除）
//   - 实现 ShareDocument：在工作空间内共享文档（带水印选项）
//   - 实现 ListSharedDocuments：列出工作空间内的共享文档
//   - 实现 InviteMember：邀请成员加入工作空间
//   - 实现 RemoveMember：移除工作空间成员
//   - 实现 UpdateMemberRole：更新成员角色
//   - 实现 RegisterRoutes：注册所有协作相关路由到 router group
// * 业务逻辑：
//   - 工作空间名称唯一性校验
//   - 文件共享时可选数字水印嵌入
//   - 成员角色分为 owner/admin/editor/viewer 四级
//   - 删除工作空间需 owner 权限
// * 依赖关系：
//   - 依赖：internal/application/collaboration/workspace.go、internal/application/collaboration/sharing.go、pkg/errors、pkg/types/common
//   - 被依赖：internal/interfaces/http/router.go
// * 测试要求：全部 Handler 方法正常与异常路径、权限校验、参数验证
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// CollaborationHandler handles HTTP requests for collaboration workspace operations.
type CollaborationHandler struct {
	workspaceSvc collaboration.WorkspaceService
	sharingSvc   collaboration.SharingService
	logger       logging.Logger
}

// NewCollaborationHandler creates a new CollaborationHandler.
func NewCollaborationHandler(
	workspaceSvc collaboration.WorkspaceService,
	sharingSvc collaboration.SharingService,
	logger logging.Logger,
) *CollaborationHandler {
	return &CollaborationHandler{
		workspaceSvc: workspaceSvc,
		sharingSvc:   sharingSvc,
		logger:       logger,
	}
}

// CreateWorkspaceRequest is the request body for creating a workspace.
type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"` // private, internal, partner
}

// ShareDocumentRequest is the request body for sharing a document.
type ShareDocumentRequest struct {
	DocumentID     string `json:"document_id"`
	WorkspaceID    string `json:"workspace_id"`
	EnableWatermark bool  `json:"enable_watermark"`
	MaxDownloads   int    `json:"max_downloads"`
	ExpiresInHours int    `json:"expires_in_hours"`
}

// InviteMemberRequest is the request body for inviting a member.
type InviteMemberRequest struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` // owner, admin, editor, viewer
}

// UpdateMemberRoleRequest is the request body for updating a member role.
type UpdateMemberRoleRequest struct {
	Role string `json:"role"`
}

// UpdateWorkspaceRequest is the request body for updating a workspace.
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Visibility  *string `json:"visibility,omitempty"`
}

// RegisterRoutes registers all collaboration routes on the given router group.
// Expected to be called with a group prefixed at /api/v1/collaboration.
func (h *CollaborationHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/workspaces", h.CreateWorkspace)
	mux.HandleFunc("GET /api/v1/workspaces", h.ListWorkspaces)
	mux.HandleFunc("GET /api/v1/workspaces/{id}", h.GetWorkspace)
	mux.HandleFunc("PUT /api/v1/workspaces/{id}", h.UpdateWorkspace)
	mux.HandleFunc("DELETE /api/v1/workspaces/{id}", h.DeleteWorkspace)
	mux.HandleFunc("POST /api/v1/workspaces/{id}/documents", h.ShareDocument)
	mux.HandleFunc("GET /api/v1/workspaces/{id}/documents", h.ListSharedDocuments)
	mux.HandleFunc("POST /api/v1/workspaces/{id}/members", h.InviteMember)
	mux.HandleFunc("DELETE /api/v1/workspaces/{id}/members/{memberId}", h.RemoveMember)
	mux.HandleFunc("PUT /api/v1/workspaces/{id}/members/{memberId}/role", h.UpdateMemberRole)
}

// CreateWorkspace handles POST /api/v1/workspaces
func (h *CollaborationHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("body", "invalid request body"))
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("name", "name is required"))
		return
	}

	userID := getUserIDFromContext(r)

	input := &collaboration.CreateWorkspaceRequest{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     userID,
	}

	ws, err := h.workspaceSvc.Create(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to create workspace", logging.Err(err))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, ws)
}

// GetWorkspace handles GET /api/v1/workspaces/{id}
func (h *CollaborationHandler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id is required"))
		return
	}

	ws, err := h.workspaceSvc.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get workspace", logging.Err(err), logging.String("workspace_id", id))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// ListWorkspaces handles GET /api/v1/workspaces
func (h *CollaborationHandler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r)
	page, pageSize := parsePagination(r)

	input := &collaboration.ListWorkspacesInput{
		UserID:   userID,
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.workspaceSvc.List(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to list workspaces", logging.Err(err))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateWorkspace handles PUT /api/v1/workspaces/{id}
func (h *CollaborationHandler) UpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id is required"))
		return
	}

	var req UpdateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("body", "invalid request body"))
		return
	}

	userID := getUserIDFromContext(r)

	input := &collaboration.UpdateWorkspaceRequest{
		WorkspaceID: id,
		Name:        req.Name,
		Description: req.Description,
		UpdatedBy:   userID,
	}

	ws, err := h.workspaceSvc.Update(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to update workspace", logging.Err(err), logging.String("workspace_id", id))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ws)
}

// DeleteWorkspace handles DELETE /api/v1/workspaces/{id}
func (h *CollaborationHandler) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id is required"))
		return
	}

	userID := getUserIDFromContext(r)

	err := h.workspaceSvc.Delete(r.Context(), id, userID)
	if err != nil {
		h.logger.Error("failed to delete workspace", logging.Err(err), logging.String("workspace_id", id))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ShareDocument handles POST /api/v1/workspaces/{id}/documents
func (h *CollaborationHandler) ShareDocument(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id is required"))
		return
	}

	var req ShareDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("body", "invalid request body"))
		return
	}

	if req.DocumentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("document_id", "document_id is required"))
		return
	}

	userID := getUserIDFromContext(r)

	input := &collaboration.ShareDocumentInput{
		WorkspaceID:     workspaceID,
		DocumentID:      req.DocumentID,
		SharedByUserID:  userID,
		EnableWatermark: req.EnableWatermark,
		MaxDownloads:    req.MaxDownloads,
		ExpiresInHours:  req.ExpiresInHours,
	}

	shared, err := h.sharingSvc.ShareDocument(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to share document", logging.Err(err), logging.String("workspace_id", workspaceID))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, shared)
}

// ListSharedDocuments handles GET /api/v1/workspaces/{id}/documents
func (h *CollaborationHandler) ListSharedDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id is required"))
		return
	}

	userID := getUserIDFromContext(r)
	page, pageSize := parsePagination(r)

	input := &collaboration.ListSharedDocumentsInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Page:        page,
		PageSize:    pageSize,
	}

	result, err := h.sharingSvc.ListDocuments(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to list shared documents", logging.Err(err), logging.String("workspace_id", workspaceID))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// InviteMember handles POST /api/v1/workspaces/{id}/members
func (h *CollaborationHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id is required"))
		return
	}

	var req InviteMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("body", "invalid request body"))
		return
	}

	if req.UserID == "" && req.Email == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("user_id", "user_id or email is required"))
		return
	}

	if !isValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("role", "role must be one of: owner, admin, editor, viewer"))
		return
	}

	userID := getUserIDFromContext(r)

	input := &collaboration.InviteMemberInput{
		WorkspaceID:  workspaceID,
		InviterID:    userID,
		InviteeID:    req.UserID,
		InviteeEmail: req.Email,
		Role:         req.Role,
	}

	member, err := h.workspaceSvc.InviteMember(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to invite member", logging.Err(err), logging.String("workspace_id", workspaceID))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, member)
}

// RemoveMember handles DELETE /api/v1/workspaces/{id}/members/{memberId}
func (h *CollaborationHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")
	memberID := r.PathValue("memberId")

	if workspaceID == "" || memberID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id and member id are required"))
		return
	}

	userID := getUserIDFromContext(r)

	err := h.workspaceSvc.RemoveMember(r.Context(), workspaceID, memberID, userID)
	if err != nil {
		h.logger.Error("failed to remove member", logging.Err(err), logging.String("workspace_id", workspaceID), logging.String("member_id", memberID))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// UpdateMemberRole handles PUT /api/v1/workspaces/{id}/members/{memberId}/role
func (h *CollaborationHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("id")
	memberID := r.PathValue("memberId")

	if workspaceID == "" || memberID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("id", "workspace id and member id are required"))
		return
	}

	var req UpdateMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("body", "invalid request body"))
		return
	}

	if !isValidRole(req.Role) {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("role", "role must be one of: owner, admin, editor, viewer"))
		return
	}

	userID := getUserIDFromContext(r)

	err := h.workspaceSvc.UpdateMemberRole(r.Context(), workspaceID, memberID, req.Role, userID)
	if err != nil {
		h.logger.Error("failed to update member role", logging.Err(err), logging.String("workspace_id", workspaceID), logging.String("member_id", memberID))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// isValidRole checks if the given role is one of the allowed workspace roles.
func isValidRole(role string) bool {
	switch role {
	case "owner", "admin", "editor", "viewer":
		return true
	default:
		return false
	}
}

// Router-compatible aliases for CollaborationHandler

// ListMembers handles member listing (placeholder).
func (h *CollaborationHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "list members not yet implemented"})
}

// AddMember is an alias for InviteMember.
func (h *CollaborationHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	h.InviteMember(w, r)
}

// CreateShareLink handles share link creation (alias for ShareDocument).
func (h *CollaborationHandler) CreateShareLink(w http.ResponseWriter, r *http.Request) {
	h.ShareDocument(w, r)
}

// GetSharedResource handles shared resource retrieval (placeholder).
func (h *CollaborationHandler) GetSharedResource(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "get shared resource not yet implemented"})
}

// RevokeShareLink handles share link revocation (placeholder).
func (h *CollaborationHandler) RevokeShareLink(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "revoke share link not yet implemented"})
}

//Personal.AI order the ending
