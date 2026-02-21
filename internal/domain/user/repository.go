package user

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UserRepository defines the persistence contract for user domain.
type UserRepository interface {
	// User
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmailForAuth(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
	UpdateLoginInfo(ctx context.Context, id uuid.UUID, ip string) error
	IncrementFailedLogin(ctx context.Context, id uuid.UUID) error
	UpdateMFA(ctx context.Context, id uuid.UUID, enabled bool, secret string) error
	VerifyEmail(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*User, int64, error)

	// Organization
	CreateOrganization(ctx context.Context, org *Organization) error
	GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error)
	GetOrganizationBySlug(ctx context.Context, slug string) (*Organization, error)
	UpdateOrganization(ctx context.Context, org *Organization) error
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role string, invitedBy uuid.UUID) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error
	GetMembers(ctx context.Context, orgID uuid.UUID) ([]*OrganizationMember, error)
	GetUserOrganizations(ctx context.Context, userID uuid.UUID) ([]*Organization, error)
	IsMember(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
	GetMemberRole(ctx context.Context, orgID, userID uuid.UUID) (string, error)

	// Role & Permission
	GetRole(ctx context.Context, id uuid.UUID) (*Role, error)
	GetRoleByName(ctx context.Context, name string) (*Role, error)
	AssignRole(ctx context.Context, userID, roleID uuid.UUID, orgID *uuid.UUID, grantedBy uuid.UUID) error
	RevokeRole(ctx context.Context, userID, roleID uuid.UUID, orgID *uuid.UUID) error
	GetUserRoles(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID) ([]*Role, error)
	GetUserPermissions(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID) ([]string, error)
	HasPermission(ctx context.Context, userID uuid.UUID, orgID *uuid.UUID, permission string) (bool, error)

	// API Key
	CreateAPIKey(ctx context.Context, key *APIKey) error
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error)
	GetAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]*APIKey, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id uuid.UUID, ip string) error
	DeactivateAPIKey(ctx context.Context, id uuid.UUID) error
	DeleteAPIKey(ctx context.Context, id uuid.UUID) error

	// Audit Log
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	GetAuditLogs(ctx context.Context, filter AuditLogFilter) ([]*AuditLog, int64, error)
	GetAuditLogsByResource(ctx context.Context, resourceType string, resourceID string, limit int) ([]*AuditLog, error)
	GetUserActivitySummary(ctx context.Context, userID uuid.UUID, startDate, endDate time.Time) (*ActivitySummary, error)
	PurgeAuditLogs(ctx context.Context, olderThan time.Time) (int64, error)

	// Transaction
	WithTx(ctx context.Context, fn func(UserRepository) error) error
}

//Personal.AI order the ending
