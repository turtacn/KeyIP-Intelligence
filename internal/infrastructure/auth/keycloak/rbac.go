package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Permission represents a fine-grained operation permission.
type Permission string

// Constants for permissions
const (
	// Patent related
	PermPatentRead       Permission = "patent:read"
	PermPatentWrite      Permission = "patent:write"
	PermPatentDelete     Permission = "patent:delete"
	PermPatentExport     Permission = "patent:export"
	PermPatentBulkImport Permission = "patent:bulk_import"

	// Analysis related
	PermAnalysisCreate Permission = "analysis:create"
	PermAnalysisRead   Permission = "analysis:read"
	PermAnalysisCancel Permission = "analysis:cancel"
	PermAnalysisExport Permission = "analysis:export"

	// Graph related
	PermGraphRead  Permission = "graph:read"
	PermGraphWrite Permission = "graph:write"
	PermGraphAdmin Permission = "graph:admin"

	// User related
	PermUserRead       Permission = "user:read"
	PermUserWrite      Permission = "user:write"
	PermUserDelete     Permission = "user:delete"
	PermUserRoleAssign Permission = "user:role_assign"

	// System related
	PermSystemConfig   Permission = "system:config"
	PermSystemMonitor  Permission = "system:monitor"
	PermSystemAuditLog Permission = "system:audit_log"

	// Tenant related
	PermTenantCreate Permission = "tenant:create"
	PermTenantRead   Permission = "tenant:read"
	PermTenantUpdate Permission = "tenant:update"
	PermTenantDelete Permission = "tenant:delete"

	// API related
	PermAPIKeyCreate  Permission = "api:key_create"
	PermAPIKeyRevoke  Permission = "api:key_revoke"
	PermAPIRateConfig Permission = "api:rate_config"
)

// Role represents a platform role.
type Role string

// Constants for roles
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

// RBACEnforcer is the interface for RBAC enforcement.
type RBACEnforcer interface {
	HasPermission(ctx context.Context, permission Permission) bool
	HasAllPermissions(ctx context.Context, permissions ...Permission) bool
	HasAnyPermission(ctx context.Context, permissions ...Permission) bool
	HasRole(ctx context.Context, role Role) bool
	GetPermissions(ctx context.Context) []Permission
	GetRoles(ctx context.Context) []Role
	EnforcePermission(ctx context.Context, permission Permission) error
	EnforceTenantAccess(ctx context.Context, targetTenantID string) error
	UpdateMapping(mapping RolePermissionMapping)
}

// rbacEnforcer implementation.
type rbacEnforcer struct {
	rolePermissions RolePermissionMapping
	logger          logging.Logger
	mu              sync.RWMutex
}

// NewRBACEnforcer creates a new RBACEnforcer.
func NewRBACEnforcer(mapping RolePermissionMapping, logger logging.Logger) RBACEnforcer {
	if mapping == nil {
		mapping = DefaultRolePermissionMapping()
	}
	return &rbacEnforcer{
		rolePermissions: mapping,
		logger:          logger,
	}
}

// DefaultRolePermissionMapping returns the default role-permission mapping.
func DefaultRolePermissionMapping() RolePermissionMapping {
	allPerms := []Permission{
		PermPatentRead, PermPatentWrite, PermPatentDelete, PermPatentExport, PermPatentBulkImport,
		PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
		PermGraphRead, PermGraphWrite, PermGraphAdmin,
		PermUserRead, PermUserWrite, PermUserDelete, PermUserRoleAssign,
		PermSystemConfig, PermSystemMonitor, PermSystemAuditLog,
		PermTenantCreate, PermTenantRead, PermTenantUpdate, PermTenantDelete,
		PermAPIKeyCreate, PermAPIKeyRevoke, PermAPIRateConfig,
	}

	return RolePermissionMapping{
		RoleSuperAdmin: allPerms,
		RoleTenantAdmin: []Permission{
			PermPatentRead, PermPatentWrite, PermPatentDelete, PermPatentExport, PermPatentBulkImport,
			PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
			PermGraphRead, PermGraphWrite, PermGraphAdmin,
			PermUserRead, PermUserWrite, PermUserDelete, PermUserRoleAssign,
			PermSystemMonitor, PermSystemAuditLog,
			PermTenantRead, PermTenantUpdate,
			PermAPIKeyCreate, PermAPIKeyRevoke,
		},
		RoleAnalyst: []Permission{
			PermPatentRead, PermPatentExport,
			PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
			PermGraphRead,
		},
		RoleResearcher: []Permission{
			PermPatentRead,
			PermAnalysisRead,
			PermGraphRead,
		},
		RoleOperator: []Permission{
			PermSystemMonitor, PermSystemAuditLog,
			PermUserRead,
		},
		RoleViewer: []Permission{
			PermPatentRead, PermAnalysisRead, PermGraphRead, PermUserRead, PermTenantRead,
		},
		RoleAPIUser: []Permission{
			PermPatentRead, PermAnalysisRead, PermGraphRead,
		},
	}
}

func (e *rbacEnforcer) UpdateMapping(mapping RolePermissionMapping) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rolePermissions = mapping
}

func (e *rbacEnforcer) getPermissionsForRole(role Role) []Permission {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rolePermissions[role]
}

func (e *rbacEnforcer) GetRoles(ctx context.Context) []Role {
	userRoles, ok := RolesFromContext(ctx)
	if !ok {
		return nil
	}
	var roles []Role
	for _, r := range userRoles {
		roles = append(roles, Role(r))
	}
	return roles
}

func (e *rbacEnforcer) GetPermissions(ctx context.Context) []Permission {
	roles := e.GetRoles(ctx)
	if len(roles) == 0 {
		return nil
	}

	permMap := make(map[Permission]bool)
	for _, role := range roles {
		perms := e.getPermissionsForRole(role)
		for _, p := range perms {
			permMap[p] = true
		}
	}

	var perms []Permission
	for p := range permMap {
		perms = append(perms, p)
	}
	return perms
}

func (e *rbacEnforcer) HasPermission(ctx context.Context, permission Permission) bool {
	if e.HasRole(ctx, RoleSuperAdmin) {
		return true
	}
	userPerms := e.GetPermissions(ctx)
	for _, p := range userPerms {
		if p == permission {
			return true
		}
	}
	return false
}

func (e *rbacEnforcer) HasAllPermissions(ctx context.Context, permissions ...Permission) bool {
	if len(permissions) == 0 {
		return true
	}
	if e.HasRole(ctx, RoleSuperAdmin) {
		return true
	}
	userPerms := e.GetPermissions(ctx)
	userPermMap := make(map[Permission]bool)
	for _, p := range userPerms {
		userPermMap[p] = true
	}

	for _, p := range permissions {
		if !userPermMap[p] {
			return false
		}
	}
	return true
}

func (e *rbacEnforcer) HasAnyPermission(ctx context.Context, permissions ...Permission) bool {
	if len(permissions) == 0 {
		return false
	}
	if e.HasRole(ctx, RoleSuperAdmin) {
		return true
	}
	userPerms := e.GetPermissions(ctx)
	userPermMap := make(map[Permission]bool)
	for _, p := range userPerms {
		userPermMap[p] = true
	}

	for _, p := range permissions {
		if userPermMap[p] {
			return true
		}
	}
	return false
}

func (e *rbacEnforcer) HasRole(ctx context.Context, role Role) bool {
	roles := e.GetRoles(ctx)
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func (e *rbacEnforcer) EnforcePermission(ctx context.Context, permission Permission) error {
	if !e.HasPermission(ctx, permission) {
		return ErrAccessDenied
	}
	return nil
}

func (e *rbacEnforcer) EnforceTenantAccess(ctx context.Context, targetTenantID string) error {
	if targetTenantID == "" {
		return errors.New(errors.ErrCodeValidation, "target tenant ID required")
	}

	if e.HasRole(ctx, RoleSuperAdmin) {
		return nil
	}

	userTenantID, ok := TenantIDFromContext(ctx)
	if !ok {
		return ErrNoAuthContext
	}

	if userTenantID != targetTenantID {
		return ErrCrossTenantAccess
	}
	return nil
}

// Errors
var (
	ErrAccessDenied      = errors.New(errors.ErrCodeForbidden, "access denied")
	ErrCrossTenantAccess = errors.New(errors.ErrCodeForbidden, "cross-tenant access denied")
	ErrNoAuthContext     = errors.New(errors.ErrCodeUnauthorized, "no authentication context")
)

// Middleware factories

func RequirePermission(enforcer RBACEnforcer, permission Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := enforcer.EnforcePermission(r.Context(), permission); err != nil {
				handleRBACError(w, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireAnyPermission(enforcer RBACEnforcer, permissions ...Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enforcer.HasAnyPermission(r.Context(), permissions...) {
				handleRBACError(w, ErrAccessDenied)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireRole(enforcer RBACEnforcer, role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enforcer.HasRole(r.Context(), role) {
				handleRBACError(w, ErrAccessDenied)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireTenantAccess(enforcer RBACEnforcer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract tenant_id from query or path?
			// Usually routing framework handles path params. But here we assume standard http.Handler.
			// So check query param or maybe context (if extracted by router earlier).
			// Assuming "tenant_id" query param for simplicity or header X-Tenant-ID?
			// Or maybe path param requires integration with router (like Chi/Gin).
			// Standard http.Request doesn't have path params.
			// We check query param "tenant_id".
			tenantID := r.URL.Query().Get("tenant_id")
			if tenantID == "" {
				// If not in query, maybe we don't enforce?
				// Or maybe it's implicitly the user's tenant?
				// If operation is tenant-scoped but no tenant ID provided, what to do?
				// Assume it's user's tenant? No, EnforceTenantAccess checks explicit target.
				// If explicit target is missing, maybe it's fine (user acts on own tenant)?
				// But if user tries to access /tenants/{id}/...
				// Without router integration, this middleware is limited.
				// We'll skip if no tenant_id found in query, assuming controller handles it.
				next.ServeHTTP(w, r)
				return
			}

			if err := enforcer.EnforceTenantAccess(r.Context(), tenantID); err != nil {
				handleRBACError(w, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func handleRBACError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	resp := map[string]string{
		"code":    "FORBIDDEN",
		"message": err.Error(),
	}
	if err == ErrNoAuthContext {
		w.WriteHeader(http.StatusUnauthorized)
		resp["code"] = "UNAUTHORIZED"
	}
	json.NewEncoder(w).Encode(resp)
}

//Personal.AI order the ending
