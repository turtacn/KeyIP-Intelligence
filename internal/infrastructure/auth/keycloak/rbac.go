package keycloak

import (
	"context"
	"net/http"
	"sync"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Permission represents a fine-grained permission.
type Permission string

const (
	// Patent Data
	PermPatentRead       Permission = "patent:read"
	PermPatentWrite      Permission = "patent:write"
	PermPatentDelete     Permission = "patent:delete"
	PermPatentExport     Permission = "patent:export"
	PermPatentBulkImport Permission = "patent:bulk_import"

	// Analysis Tasks
	PermAnalysisCreate Permission = "analysis:create"
	PermAnalysisRead   Permission = "analysis:read"
	PermAnalysisCancel Permission = "analysis:cancel"
	PermAnalysisExport Permission = "analysis:export"

	// Graph
	PermGraphRead  Permission = "graph:read"
	PermGraphWrite Permission = "graph:write"
	PermGraphAdmin Permission = "graph:admin"

	// User Management
	PermUserRead       Permission = "user:read"
	PermUserWrite      Permission = "user:write"
	PermUserDelete     Permission = "user:delete"
	PermUserRoleAssign Permission = "user:role_assign"

	// System
	PermSystemConfig   Permission = "system:config"
	PermSystemMonitor  Permission = "system:monitor"
	PermSystemAuditLog Permission = "system:audit_log"

	// Tenant
	PermTenantCreate Permission = "tenant:create"
	PermTenantRead   Permission = "tenant:read"
	PermTenantUpdate Permission = "tenant:update"
	PermTenantDelete Permission = "tenant:delete"

	// API
	PermAPIKeyCreate  Permission = "api:key_create"
	PermAPIKeyRevoke  Permission = "api:key_revoke"
	PermAPIRateConfig Permission = "api:rate_config"
)

// Role represents a platform role.
type Role string

const (
	RoleSuperAdmin  Role = "super_admin"
	RoleTenantAdmin Role = "tenant_admin"
	RoleAnalyst     Role = "analyst"
	RoleResearcher  Role = "researcher"
	RoleOperator    Role = "operator"
	RoleViewer      Role = "viewer"
	RoleAPIUser     Role = "api_user"
)

// RolePermissionMapping maps roles to permissions.
type RolePermissionMapping map[Role][]Permission

// RBACEnforcer defines the interface for RBAC enforcement.
type RBACEnforcer interface {
	HasPermission(ctx context.Context, permission Permission) bool
	HasAllPermissions(ctx context.Context, permissions ...Permission) bool
	HasAnyPermission(ctx context.Context, permissions ...Permission) bool
	HasRole(ctx context.Context, role Role) bool
	GetPermissions(ctx context.Context) []Permission
	GetRoles(ctx context.Context) []Role
	EnforcePermission(ctx context.Context, permission Permission) error
	EnforceTenantAccess(ctx context.Context, targetTenantID string) error

	RequirePermission(permission Permission) func(http.Handler) http.Handler
	RequireAnyPermission(permissions ...Permission) func(http.Handler) http.Handler
	RequireRole(role Role) func(http.Handler) http.Handler
	RequireTenantAccess() func(http.Handler) http.Handler

	UpdateMapping(mapping RolePermissionMapping)
}

type rbacEnforcer struct {
	rolePermissions RolePermissionMapping
	logger          logging.Logger
	mu              sync.RWMutex
}

// NewRBACEnforcer creates a new RBAC enforcer.
func NewRBACEnforcer(mapping RolePermissionMapping, logger logging.Logger) RBACEnforcer {
	if mapping == nil {
		mapping = DefaultRolePermissionMapping()
	}
	return &rbacEnforcer{
		rolePermissions: mapping,
		logger:          logger,
	}
}

// DefaultRolePermissionMapping returns the default mapping.
func DefaultRolePermissionMapping() RolePermissionMapping {
	mapping := make(RolePermissionMapping)

	// Tenant Admin
	mapping[RoleTenantAdmin] = []Permission{
		PermPatentRead, PermPatentWrite, PermPatentDelete, PermPatentExport, PermPatentBulkImport,
		PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
		PermGraphRead, PermGraphWrite, PermGraphAdmin,
		PermUserRead, PermUserWrite, PermUserDelete, PermUserRoleAssign,
		PermTenantRead, PermTenantUpdate,
		PermAPIKeyCreate, PermAPIKeyRevoke, PermAPIRateConfig,
	}

	// Analyst
	mapping[RoleAnalyst] = []Permission{
		PermPatentRead, PermPatentExport,
		PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
		PermGraphRead,
	}

	// Researcher
	mapping[RoleResearcher] = []Permission{
		PermPatentRead,
		PermAnalysisRead,
		PermGraphRead,
	}

	// Operator
	mapping[RoleOperator] = []Permission{
		PermSystemMonitor, PermSystemAuditLog,
		PermUserRead,
	}

	// Viewer
	mapping[RoleViewer] = []Permission{
		PermPatentRead,
		PermAnalysisRead,
		PermGraphRead,
		PermTenantRead,
		PermUserRead,
	}

	// API User
	mapping[RoleAPIUser] = []Permission{
		PermPatentRead,
		PermAnalysisRead,
		PermGraphRead,
	}

	return mapping
}

func (e *rbacEnforcer) UpdateMapping(mapping RolePermissionMapping) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rolePermissions = mapping
}

func (e *rbacEnforcer) HasPermission(ctx context.Context, permission Permission) bool {
	roles := e.GetRoles(ctx)
	if len(roles) == 0 {
		return false
	}

	// Check for Super Admin
	for _, r := range roles {
		if r == RoleSuperAdmin {
			return true
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, r := range roles {
		perms, ok := e.rolePermissions[r]
		if !ok {
			continue
		}
		for _, p := range perms {
			if p == permission {
				return true
			}
		}
	}
	return false
}

func (e *rbacEnforcer) HasAllPermissions(ctx context.Context, permissions ...Permission) bool {
	for _, p := range permissions {
		if !e.HasPermission(ctx, p) {
			return false
		}
	}
	return true
}

func (e *rbacEnforcer) HasAnyPermission(ctx context.Context, permissions ...Permission) bool {
	for _, p := range permissions {
		if e.HasPermission(ctx, p) {
			return true
		}
	}
	return false
}

func (e *rbacEnforcer) HasRole(ctx context.Context, role Role) bool {
	userRoles := e.GetRoles(ctx)
	for _, r := range userRoles {
		if r == role {
			return true
		}
	}
	return false
}

func (e *rbacEnforcer) GetPermissions(ctx context.Context) []Permission {
	roles := e.GetRoles(ctx)
	if len(roles) == 0 {
		return nil
	}

	permMap := make(map[Permission]bool)

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, r := range roles {
		// SuperAdmin gets everything in the map + extras?
		// For simplicity, we only return explicitly mapped permissions.
		// If SuperAdmin needs to show all, UI usually handles it by role check.

		if perms, ok := e.rolePermissions[r]; ok {
			for _, p := range perms {
				permMap[p] = true
			}
		}
	}

	var result []Permission
	for p := range permMap {
		result = append(result, p)
	}
	return result
}

func (e *rbacEnforcer) GetRoles(ctx context.Context) []Role {
	rawRoles, ok := RolesFromContext(ctx)
	if !ok {
		return nil
	}

	var roles []Role
	for _, r := range rawRoles {
		roles = append(roles, Role(r))
	}
	return roles
}

func (e *rbacEnforcer) EnforcePermission(ctx context.Context, permission Permission) error {
	if !e.HasPermission(ctx, permission) {
		return errors.ErrForbidden("access denied").WithDetail(string(permission))
	}
	return nil
}

func (e *rbacEnforcer) EnforceTenantAccess(ctx context.Context, targetTenantID string) error {
	if targetTenantID == "" {
		return ErrInvalidConfig.WithInternalMessage("target tenant ID empty")
	}

	// Super Admin can access all tenants
	if e.HasRole(ctx, RoleSuperAdmin) {
		return nil
	}

	currentTenantID, ok := TenantIDFromContext(ctx)
	if !ok {
		return ErrNoAuthContext
	}

	if currentTenantID != targetTenantID {
		return errors.ErrForbidden("cross-tenant access denied").
			WithDetails("current", currentTenantID).
			WithDetails("target", targetTenantID)
	}

	return nil
}

// Middleware Factories

func (e *rbacEnforcer) RequirePermission(permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := e.EnforcePermission(r.Context(), permission); err != nil {
				defaultAuthFailureHandler(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (e *rbacEnforcer) RequireAnyPermission(permissions ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !e.HasAnyPermission(r.Context(), permissions...) {
				defaultAuthFailureHandler(w, r, ErrAccessDenied)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (e *rbacEnforcer) RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !e.HasRole(r.Context(), role) {
				defaultAuthFailureHandler(w, r, ErrAccessDenied)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (e *rbacEnforcer) RequireTenantAccess() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			targetTenantID := r.URL.Query().Get("tenant_id")
			if targetTenantID != "" {
				if err := e.EnforceTenantAccess(r.Context(), targetTenantID); err != nil {
					defaultAuthFailureHandler(w, r, err)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

var (
	ErrAccessDenied      = errors.ErrForbidden("access denied")
	ErrCrossTenantAccess = errors.ErrForbidden("cross-tenant access denied")
	ErrNoAuthContext     = errors.ErrUnauthorized("no authentication context")
)
//Personal.AI order the ending
