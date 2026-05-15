package collaboration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	collabdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// minimalWorkspaceService is an in-memory implementation of WorkspaceService.
type minimalWorkspaceService struct {
	logger     logging.Logger
	workspaces map[string]*WorkspaceResponse
	members    map[string][]*MemberResponse
}

func NewMinimalWorkspaceService(logger logging.Logger) WorkspaceService {
	return &minimalWorkspaceService{
		logger:     logger,
		workspaces: map[string]*WorkspaceResponse{},
		members:    map[string][]*MemberResponse{},
	}
}

func (s *minimalWorkspaceService) Create(ctx context.Context, req *CreateWorkspaceRequest) (*WorkspaceResponse, error) {
	id := uuid.New().String()
	ws := &WorkspaceResponse{
		ID: id, Name: req.Name, Description: req.Description,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.workspaces[id] = ws
	return ws, nil
}

func (s *minimalWorkspaceService) Update(ctx context.Context, req *UpdateWorkspaceRequest) (*WorkspaceResponse, error) {
	ws, ok := s.workspaces[req.WorkspaceID]
	if !ok { return nil, fmt.Errorf("workspace not found") }
	if req.Name != nil { ws.Name = *req.Name }
	if req.Description != nil { ws.Description = *req.Description }
	ws.UpdatedAt = time.Now()
	return ws, nil
}

func (s *minimalWorkspaceService) Delete(ctx context.Context, workspaceID, deletedBy string) error {
	delete(s.workspaces, workspaceID)
	return nil
}

func (s *minimalWorkspaceService) GetByID(ctx context.Context, workspaceID string) (*WorkspaceResponse, error) {
	ws, ok := s.workspaces[workspaceID]
	if !ok { return nil, fmt.Errorf("workspace not found") }
	return ws, nil
}

func (s *minimalWorkspaceService) List(ctx context.Context, input *ListWorkspacesInput) (*ListWorkspacesResult, error) {
	var items []*WorkspaceResponse
	for _, ws := range s.workspaces { items = append(items, ws) }
	if items == nil { items = []*WorkspaceResponse{} }
	return &ListWorkspacesResult{Workspaces: items, Total: len(items)}, nil
}

func (s *minimalWorkspaceService) InviteMember(ctx context.Context, input *InviteMemberInput) (*MemberResponse, error) {
	m := &MemberResponse{
		UserID:   input.InviteeID,
		Role:     collabdomain.Role(input.Role),
		JoinedAt: time.Now(),
	}
	s.members[input.WorkspaceID] = append(s.members[input.WorkspaceID], m)
	return m, nil
}

func (s *minimalWorkspaceService) RemoveMember(ctx context.Context, workspaceID, memberID, userID string) error {
	return nil
}

func (s *minimalWorkspaceService) UpdateMemberRole(ctx context.Context, workspaceID, memberID, role, userID string) error {
	return nil
}

func (s *minimalWorkspaceService) ListMembers(ctx context.Context, workspaceID string, pagination commontypes.Pagination) ([]*MemberResponse, int, error) {
	ms := s.members[workspaceID]
	if ms == nil { ms = []*MemberResponse{} }
	return ms, len(ms), nil
}

var _ WorkspaceService = (*minimalWorkspaceService)(nil)
