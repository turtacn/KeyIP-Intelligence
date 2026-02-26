package collaboration

import (
	"context"
)

// WorkspaceRepository defines persistence for workspaces.
type WorkspaceRepository interface {
	Save(ctx context.Context, workspace *Workspace) error
	FindByID(ctx context.Context, id string) (*Workspace, error)
	FindByOwnerID(ctx context.Context, ownerID string) ([]*Workspace, error)
	FindByMemberID(ctx context.Context, userID string) ([]*Workspace, error)
	FindBySlug(ctx context.Context, slug string) (*Workspace, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, ownerID string) (int64, error)
}

// MemberRepository defines persistence for members.
type MemberRepository interface {
	Save(ctx context.Context, member *MemberPermission) error
	FindByID(ctx context.Context, id string) (*MemberPermission, error)
	FindByWorkspaceID(ctx context.Context, workspaceID string) ([]*MemberPermission, error)
	FindByUserID(ctx context.Context, userID string) ([]*MemberPermission, error)
	FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*MemberPermission, error)
	FindByRole(ctx context.Context, workspaceID string, role Role) ([]*MemberPermission, error)
	Delete(ctx context.Context, id string) error
	CountByWorkspace(ctx context.Context, workspaceID string) (int64, error)
	CountByRole(ctx context.Context, workspaceID string) (map[Role]int64, error)
}

// CollaborationQueryOption is a functional option.
type CollaborationQueryOption func(*CollaborationQueryOptions)

// CollaborationQueryOptions encapsulates query parameters.
type CollaborationQueryOptions struct {
	Offset       int
	Limit        int
	ActiveOnly   bool
	AcceptedOnly bool
	RoleFilter   Role
}

// WithCollabPagination sets pagination.
func WithCollabPagination(offset, limit int) CollaborationQueryOption {
	return func(o *CollaborationQueryOptions) {
		if offset < 0 {
			offset = 0
		}
		if limit < 1 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}
		o.Offset = offset
		o.Limit = limit
	}
}

// WithActiveOnly filters by active status.
func WithActiveOnly() CollaborationQueryOption {
	return func(o *CollaborationQueryOptions) {
		o.ActiveOnly = true
	}
}

// WithAcceptedOnly filters by accepted status.
func WithAcceptedOnly() CollaborationQueryOption {
	return func(o *CollaborationQueryOptions) {
		o.AcceptedOnly = true
	}
}

// WithRoleFilter filters by role.
func WithRoleFilter(role Role) CollaborationQueryOption {
	return func(o *CollaborationQueryOptions) {
		o.RoleFilter = role
	}
}

// ApplyCollabOptions applies options.
func ApplyCollabOptions(opts ...CollaborationQueryOption) CollaborationQueryOptions {
	o := CollaborationQueryOptions{
		Offset: 0,
		Limit:  20,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
