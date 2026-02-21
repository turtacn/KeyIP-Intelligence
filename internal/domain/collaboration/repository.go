package collaboration

import (
	"context"
)

// WorkspaceRepository defines the persistence interface for workspaces.
type WorkspaceRepository interface {
	Save(ctx context.Context, workspace *Workspace) error
	FindByID(ctx context.Context, id string) (*Workspace, error)
	FindByOwnerID(ctx context.Context, ownerID string) ([]*Workspace, error)
	FindByMemberID(ctx context.Context, userID string) ([]*Workspace, error)
	FindBySlug(ctx context.Context, slug string) (*Workspace, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, ownerID string) (int64, error)
}

// MemberRepository defines the persistence interface for member permissions.
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

// CollaborationQueryOptions defines filtering and pagination for collaboration queries.
type CollaborationQueryOptions struct {
	Offset       int
	Limit        int
	ActiveOnly   bool
	AcceptedOnly bool
	RoleFilter   Role
}

// CollaborationQueryOption defines a functional option for collaboration queries.
type CollaborationQueryOption func(*CollaborationQueryOptions)

// WithCollabPagination sets pagination options.
func WithCollabPagination(offset, limit int) CollaborationQueryOption {
	return func(o *CollaborationQueryOptions) {
		o.Offset = offset
		o.Limit = limit
	}
}

// WithActiveOnly filters for active members only.
func WithActiveOnly() CollaborationQueryOption {
	return func(o *CollaborationQueryOptions) {
		o.ActiveOnly = true
	}
}

// WithAcceptedOnly filters for accepted members only.
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

// ApplyCollabOptions applies the given options and returns the final configuration.
func ApplyCollabOptions(opts ...CollaborationQueryOption) CollaborationQueryOptions {
	options := CollaborationQueryOptions{
		Offset: 0,
		Limit:  20,
	}
	for _, opt := range opts {
		opt(&options)
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	if options.Limit <= 0 {
		options.Limit = 20
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	return options
}

//Personal.AI order the ending
