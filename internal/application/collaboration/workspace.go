// ---
// Phase 10 - File #196: internal/application/collaboration/workspace.go
//
// 功能定位: 工作空间应用服务，编排工作空间的创建、更新、删除、查询、成员管理等业务流程。
//   协调接口层与领域层交互，不包含核心业务规则。
//
// 核心实现:
//   - WorkspaceAppService 接口: Create / Update / Delete / GetByID / ListByUser / AddMember / RemoveMember / ListMembers
//   - workspaceAppServiceImpl: 注入领域服务、仓储、日志
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
// ---

package collaboration

import (
	"context"
	"fmt"
	"strings"
	"time"

	collabdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// CreateWorkspaceRequest is the input DTO for workspace creation.
type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OwnerID     string `json:"owner_id"`
}

func (r *CreateWorkspaceRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return pkgerrors.NewValidation("name is required")
	}
	if len(r.Name) > 256 {
		return pkgerrors.NewValidation("name must not exceed 256 characters")
	}
	if strings.TrimSpace(r.OwnerID) == "" {
		return pkgerrors.NewValidation("owner_id is required")
	}
	return nil
}

// UpdateWorkspaceRequest is the input DTO for workspace update.
type UpdateWorkspaceRequest struct {
	WorkspaceID string  `json:"workspace_id"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	UpdatedBy   string  `json:"updated_by"`
}

func (r *UpdateWorkspaceRequest) Validate() error {
	if strings.TrimSpace(r.WorkspaceID) == "" {
		return pkgerrors.NewValidation("workspace_id is required")
	}
	if strings.TrimSpace(r.UpdatedBy) == "" {
		return pkgerrors.NewValidation("updated_by is required")
	}
	if r.Name != nil && strings.TrimSpace(*r.Name) == "" {
		return pkgerrors.NewValidation("name must not be empty when provided")
	}
	if r.Name != nil && len(*r.Name) > 256 {
		return pkgerrors.NewValidation("name must not exceed 256 characters")
	}
	return nil
}

// AddMemberRequest is the input DTO for adding a member.
type AddMemberRequest struct {
	WorkspaceID string             `json:"workspace_id"`
	UserID      string             `json:"user_id"`
	Role        collabdomain.Role  `json:"role"`
	AddedBy     string             `json:"added_by"`
}

func (r *AddMemberRequest) Validate() error {
	if strings.TrimSpace(r.WorkspaceID) == "" {
		return pkgerrors.NewValidation("workspace_id is required")
	}
	if strings.TrimSpace(r.UserID) == "" {
		return pkgerrors.NewValidation("user_id is required")
	}
	if strings.TrimSpace(r.AddedBy) == "" {
		return pkgerrors.NewValidation("added_by is required")
	}
	if strings.TrimSpace(string(r.Role)) == "" {
		return pkgerrors.NewValidation("role is required")
	}
	return nil
}

// RemoveMemberRequest is the input DTO for removing a member.
type RemoveMemberRequest struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	RemovedBy   string `json:"removed_by"`
}

func (r *RemoveMemberRequest) Validate() error {
	if strings.TrimSpace(r.WorkspaceID) == "" {
		return pkgerrors.NewValidation("workspace_id is required")
	}
	if strings.TrimSpace(r.UserID) == "" {
		return pkgerrors.NewValidation("user_id is required")
	}
	if strings.TrimSpace(r.RemovedBy) == "" {
		return pkgerrors.NewValidation("removed_by is required")
	}
	return nil
}

// WorkspaceResponse is the output DTO for workspace queries.
type WorkspaceResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	OwnerID     string    `json:"owner_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MemberResponse is the output DTO for member queries.
type MemberResponse struct {
	UserID    string            `json:"user_id"`
	Role      collabdomain.Role `json:"role"`
	JoinedAt  time.Time         `json:"joined_at"`
}

// WorkspaceAppService defines the application-level workspace operations.
type WorkspaceAppService interface {
	Create(ctx context.Context, req *CreateWorkspaceRequest) (*WorkspaceResponse, error)
	Update(ctx context.Context, req *UpdateWorkspaceRequest) (*WorkspaceResponse, error)
	Delete(ctx context.Context, workspaceID string, deletedBy string) error
	GetByID(ctx context.Context, workspaceID string) (*WorkspaceResponse, error)
	ListByUser(ctx context.Context, userID string, pagination commontypes.Pagination) ([]*WorkspaceResponse, int, error)
	AddMember(ctx context.Context, req *AddMemberRequest) error
	RemoveMember(ctx context.Context, req *RemoveMemberRequest) error
	ListMembers(ctx context.Context, workspaceID string, pagination commontypes.Pagination) ([]*MemberResponse, int, error)
}

// MemberRepository abstracts persistence for workspace members.
type MemberRepository interface {
	Add(ctx context.Context, workspaceID, userID string, role collabdomain.Role) error
	Remove(ctx context.Context, workspaceID, userID string) error
	List(ctx context.Context, workspaceID string, pagination commontypes.Pagination) ([]*MemberResponse, int, error)
	GetMember(ctx context.Context, workspaceID, userID string) (*MemberResponse, error)
}

type workspaceAppServiceImpl struct {
	domainService collabdomain.Service
	workspaceRepo collabdomain.WorkspaceRepository
	memberRepo    MemberRepository
	logger        logging.Logger
}

// NewWorkspaceAppService constructs a WorkspaceAppService.
func NewWorkspaceAppService(
	domainService collabdomain.Service,
	workspaceRepo collabdomain.WorkspaceRepository,
	memberRepo MemberRepository,
	logger logging.Logger,
) WorkspaceAppService {
	return &workspaceAppServiceImpl{
		domainService: domainService,
		workspaceRepo: workspaceRepo,
		memberRepo:    memberRepo,
		logger:        logger,
	}
}

func (s *workspaceAppServiceImpl) Create(ctx context.Context, req *CreateWorkspaceRequest) (*WorkspaceResponse, error) {
	if req == nil {
		return nil, pkgerrors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	ws := &collabdomain.Workspace{
		ID:          commontypes.NewID(),
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		OwnerID:     req.OwnerID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.domainService.CreateWorkspace(ctx, ws); err != nil {
		s.logger.Error("domain service failed to create workspace", "error", err)
		return nil, pkgerrors.NewInternal("failed to create workspace")
	}

	if err := s.workspaceRepo.Create(ctx, ws); err != nil {
		s.logger.Error("failed to persist workspace", "error", err)
		return nil, pkgerrors.NewInternal("failed to create workspace")
	}

	// Add owner as admin member
	if err := s.memberRepo.Add(ctx, ws.ID, req.OwnerID, collabdomain.RoleAdmin); err != nil {
		s.logger.Warn("failed to add owner as member", "workspace_id", ws.ID, "owner_id", req.OwnerID, "error", err)
	}

	s.logger.Info("workspace created", "workspace_id", ws.ID, "owner_id", req.OwnerID)

	return &WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}, nil
}

func (s *workspaceAppServiceImpl) Update(ctx context.Context, req *UpdateWorkspaceRequest) (*WorkspaceResponse, error) {
	if req == nil {
		return nil, pkgerrors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	ws, err := s.workspaceRepo.GetByID(ctx, req.WorkspaceID)
	if err != nil {
		return nil, pkgerrors.NewNotFound(fmt.Sprintf("workspace %s not found", req.WorkspaceID))
	}

	// Permission check
	if err := s.domainService.CheckPermission(ctx, req.UpdatedBy, ws.ID, collabdomain.ActionEdit); err != nil {
		return nil, pkgerrors.NewPermissionDenied("insufficient permission to update workspace")
	}

	if req.Name != nil {
		ws.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		ws.Description = *req.Description
	}
	ws.UpdatedAt = time.Now().UTC()

	if err := s.workspaceRepo.Update(ctx, ws); err != nil {
		s.logger.Error("failed to update workspace", "workspace_id", ws.ID, "error", err)
		return nil, pkgerrors.NewInternal("failed to update workspace")
	}

	s.logger.Info("workspace updated", "workspace_id", ws.ID, "updated_by", req.UpdatedBy)

	return &WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}, nil
}

func (s *workspaceAppServiceImpl) Delete(ctx context.Context, workspaceID string, deletedBy string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return pkgerrors.NewValidation("workspace_id is required")
	}
	if strings.TrimSpace(deletedBy) == "" {
		return pkgerrors.NewValidation("deleted_by is required")
	}

	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return pkgerrors.NewNotFound(fmt.Sprintf("workspace %s not found", workspaceID))
	}

	// Only owner can delete
	if ws.OwnerID != deletedBy {
		if err := s.domainService.CheckPermission(ctx, deletedBy, ws.ID, collabdomain.ActionDelete); err != nil {
			return pkgerrors.NewPermissionDenied("only the owner or admin can delete a workspace")
		}
	}

	if err := s.workspaceRepo.Delete(ctx, workspaceID); err != nil {
		s.logger.Error("failed to delete workspace", "workspace_id", workspaceID, "error", err)
		return pkgerrors.NewInternal("failed to delete workspace")
	}

	s.logger.Info("workspace deleted", "workspace_id", workspaceID, "deleted_by", deletedBy)
	return nil
}

func (s *workspaceAppServiceImpl) GetByID(ctx context.Context, workspaceID string) (*WorkspaceResponse, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, pkgerrors.NewValidation("workspace_id is required")
	}

	ws, err := s.workspaceRepo.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, pkgerrors.NewNotFound(fmt.Sprintf("workspace %s not found", workspaceID))
	}

	return &WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		OwnerID:     ws.OwnerID,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}, nil
}

func (s *workspaceAppServiceImpl) ListByUser(ctx context.Context, userID string, pagination commontypes.Pagination) ([]*WorkspaceResponse, int, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, 0, pkgerrors.NewValidation("user_id is required")
	}
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize < 1 || pagination.PageSize > 100 {
		pagination.PageSize = 20
	}

	workspaces, total, err := s.workspaceRepo.ListByUser(ctx, userID, pagination)
	if err != nil {
		s.logger.Error("failed to list workspaces", "user_id", userID, "error", err)
		return nil, 0, pkgerrors.NewInternal("failed to list workspaces")
	}

	results := make([]*WorkspaceResponse, 0, len(workspaces))
	for _, ws := range workspaces {
		results = append(results, &WorkspaceResponse{
			ID:          ws.ID,
			Name:        ws.Name,
			Description: ws.Description,
			OwnerID:     ws.OwnerID,
			CreatedAt:   ws.CreatedAt,
			UpdatedAt:   ws.UpdatedAt,
		})
	}

	return results, total, nil
}

func (s *workspaceAppServiceImpl) AddMember(ctx context.Context, req *AddMemberRequest) error {
	if req == nil {
		return pkgerrors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return err
	}

	// Verify workspace exists
	if _, err := s.workspaceRepo.GetByID(ctx, req.WorkspaceID); err != nil {
		return pkgerrors.NewNotFound(fmt.Sprintf("workspace %s not found", req.WorkspaceID))
	}

	// Permission check
	if err := s.domainService.CheckPermission(ctx, req.AddedBy, req.WorkspaceID, collabdomain.ActionManageMembers); err != nil {
		return pkgerrors.NewPermissionDenied("insufficient permission to add members")
	}

	if err := s.domainService.AddMember(ctx, req.WorkspaceID, req.UserID, req.Role); err != nil {
		s.logger.Error("domain service failed to add member", "error", err)
		return pkgerrors.NewInternal("failed to add member")
	}

	if err := s.memberRepo.Add(ctx, req.WorkspaceID, req.UserID, req.Role); err != nil {
		s.logger.Error("failed to persist member", "error", err)
		return pkgerrors.NewInternal("failed to add member")
	}

	s.logger.Info("member added", "workspace_id", req.WorkspaceID, "user_id", req.UserID, "role", req.Role)
	return nil
}

func (s *workspaceAppServiceImpl) RemoveMember(ctx context.Context, req *RemoveMemberRequest) error {
	if req == nil {
		return pkgerrors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return err
	}

	// Verify workspace exists
	ws, err := s.workspaceRepo.GetByID(ctx, req.WorkspaceID)
	if err != nil {
		return pkgerrors.NewNotFound(fmt.Sprintf("workspace %s not found", req.WorkspaceID))
	}

	// Cannot remove the owner
	if ws.OwnerID == req.UserID {
		return pkgerrors.NewValidation("cannot remove the workspace owner")
	}

	// Permission check
	if err := s.domainService.CheckPermission(ctx, req.RemovedBy, req.WorkspaceID, collabdomain.ActionManageMembers); err != nil {
		return pkgerrors.NewPermissionDenied("insufficient permission to remove members")
	}

	if err := s.domainService.RemoveMember(ctx, req.WorkspaceID, req.UserID); err != nil {
		s.logger.Error("domain service failed to remove member", "error", err)
		return pkgerrors.NewInternal("failed to remove member")
	}

	if err := s.memberRepo.Remove(ctx, req.WorkspaceID, req.UserID); err != nil {
		s.logger.Error("failed to remove member from repo", "error", err)
		return pkgerrors.NewInternal("failed to remove member")
	}

	s.logger.Info("member removed", "workspace_id", req.WorkspaceID, "user_id", req.UserID)
	return nil
}

func (s *workspaceAppServiceImpl) ListMembers(ctx context.Context, workspaceID string, pagination commontypes.Pagination) ([]*MemberResponse, int, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, 0, pkgerrors.NewValidation("workspace_id is required")
	}
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize < 1 || pagination.PageSize > 100 {
		pagination.PageSize = 20
	}

	members, total, err := s.memberRepo.List(ctx, workspaceID, pagination)
	if err != nil {
		s.logger.Error("failed to list members", "workspace_id", workspaceID, "error", err)
		return nil, 0, pkgerrors.NewInternal("failed to list members")
	}

	return members, total, nil
}

//Personal.AI order the ending

