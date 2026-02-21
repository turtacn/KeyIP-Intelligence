package user

import (
	"time"

	"github.com/google/uuid"
)

// User represents a system user.
type User struct {
	ID               uuid.UUID         `json:"id"`
	Email            string            `json:"email"`
	Username         string            `json:"username"`
	DisplayName      string            `json:"display_name"`
	PasswordHash     string            `json:"-"`
	Status           string            `json:"status"`
	AvatarURL        string            `json:"avatar_url,omitempty"`
	Locale           string            `json:"locale"`
	Timezone         string            `json:"timezone"`
	LastLoginAt      *time.Time        `json:"last_login_at,omitempty"`
	LastLoginIP      string            `json:"last_login_ip,omitempty"`
	LoginCount       int               `json:"login_count"`
	FailedLoginCount int               `json:"failed_login_count"`
	LockedUntil      *time.Time        `json:"locked_until,omitempty"`
	EmailVerifiedAt  *time.Time        `json:"email_verified_at,omitempty"`
	MFAEnabled       bool              `json:"mfa_enabled"`
	MFASecret        string            `json:"-"`
	Preferences      map[string]any    `json:"preferences,omitempty"`
	Metadata         map[string]any    `json:"metadata,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	DeletedAt        *time.Time        `json:"deleted_at,omitempty"`
}

// Organization represents a tenant or team.
type Organization struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Description string         `json:"description,omitempty"`
	LogoURL     string         `json:"logo_url,omitempty"`
	Plan        string         `json:"plan"`
	MaxMembers  int            `json:"max_members"`
	MaxPatents  int            `json:"max_patents"`
	Settings    map[string]any `json:"settings,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   *time.Time     `json:"deleted_at,omitempty"`
}

// OrganizationMember represents a user's membership in an organization.
type OrganizationMember struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	Role           string    `json:"role"`
	JoinedAt       time.Time `json:"joined_at"`
	InvitedBy      *uuid.UUID `json:"invited_by,omitempty"`
}

// Role represents a role definition.
type Role struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description,omitempty"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// APIKey represents an API access key.
type APIKey struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	OrganizationID *uuid.UUID `json:"organization_id,omitempty"`
	Name           string    `json:"name"`
	KeyHash        string    `json:"-"`
	KeyPrefix      string    `json:"key_prefix"`
	Scopes         []string  `json:"scopes"`
	RateLimit      int       `json:"rate_limit"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP     string    `json:"last_used_ip,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AuditLog represents a system audit record.
type AuditLog struct {
	ID             uuid.UUID      `json:"id"`
	UserID         *uuid.UUID     `json:"user_id,omitempty"`
	OrganizationID *uuid.UUID     `json:"organization_id,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resource_type"`
	ResourceID     string         `json:"resource_id,omitempty"`
	IPAddress      string         `json:"ip_address,omitempty"`
	UserAgent      string         `json:"user_agent,omitempty"`
	RequestID      string         `json:"request_id,omitempty"`
	BeforeState    map[string]any `json:"before_state,omitempty"`
	AfterState     map[string]any `json:"after_state,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// ListFilter defines filtering options for listing users.
type ListFilter struct {
	Status string
	Email  string
	Offset int
	Limit  int
}

// AuditLogFilter defines filtering options for audit logs.
type AuditLogFilter struct {
	UserID         *uuid.UUID
	OrganizationID *uuid.UUID
	Action         string
	ResourceType   string
	ResourceID     string
	StartDate      *time.Time
	EndDate        *time.Time
	Offset         int
	Limit          int
}

// ActivitySummary aggregates user activity.
type ActivitySummary struct {
	UserID     uuid.UUID      `json:"user_id"`
	ActionCounts map[string]int64 `json:"action_counts"`
}

//Personal.AI order the ending
