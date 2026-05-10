package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

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

	// Report related
	PermReportRead   Permission = "report:read"
	PermReportCreate Permission = "report:create"
	PermReportExport Permission = "report:export"

	// Dashboard related
	PermDashboardRead Permission = "dashboard:read"
)

// Role represents a platform role.
type Role string

// Constants for roles
const (
	RoleSuperAdmin    Role = "super_admin"
	RoleTenantAdmin   Role = "tenant_admin"
	RoleAnalyst       Role = "analyst"
	RoleResearcher    Role = "researcher"
	RoleOperator      Role = "operator"
	RoleViewer        Role = "viewer"
	RoleAPIUser       Role = "api_user"
	RoleExecutive     Role = "executive"
	RolePartnerAgent  Role = "partner_agent"
	RoleIPManager     Role = "ip_manager"
)

// ResourceType represents a type of resource for fine-grained access control.
type ResourceType string

const (
	ResourcePatent    ResourceType = "patent"
	ResourcePortfolio ResourceType = "portfolio"
	ResourceReport    ResourceType = "report"
	ResourceDashboard ResourceType = "dashboard"
	ResourceAnalysis  ResourceType = "analysis"
	ResourceWorkspace ResourceType = "workspace"
	ResourceLifecycle ResourceType = "lifecycle"
)

// ResourcePermissionRequest defines a request to check access to a specific resource.
type ResourcePermissionRequest struct {
	ResourceType ResourceType
	ResourceID   string
	Action       Permission
	OwnerTenant  string
	OwnerUserID  string
}

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

	// Extended interface

	// HasResourcePermission checks if the caller has a specific action permission
	// on a specific resource, taking into account resource ownership and tenant isolation.
	HasResourcePermission(ctx context.Context, req ResourcePermissionRequest) bool

	// EnforceResourcePermission is the middleware-returning variant that returns
	// an error if the caller lacks the required resource-level access.
	EnforceResourcePermission(ctx context.Context, req ResourcePermissionRequest) error

	// InvalidatePermissionCache clears any cached permissions for the given user.
	InvalidatePermissionCache(ctx context.Context)
}

// rbacEnforcer implementation.
type rbacEnforcer struct {
	rolePermissions RolePermissionMapping
	logger          logging.Logger
	mu              sync.RWMutex
	permCache       *permissionCache
}

// NewRBACEnforcer creates a new RBACEnforcer.
func NewRBACEnforcer(mapping RolePermissionMapping, logger logging.Logger) RBACEnforcer {
	if mapping == nil {
		mapping = DefaultRolePermissionMapping()
	}
	return &rbacEnforcer{
		rolePermissions: mapping,
		logger:          logger,
		permCache:       newPermissionCache(5 * time.Minute),
	}
}

// NewRBACEnforcerWithCache creates a new RBACEnforcer with a custom cache TTL.
func NewRBACEnforcerWithCache(mapping RolePermissionMapping, logger logging.Logger, cacheTTL time.Duration) RBACEnforcer {
	if mapping == nil {
		mapping = DefaultRolePermissionMapping()
	}
	return &rbacEnforcer{
		rolePermissions: mapping,
		logger:          logger,
		permCache:       newPermissionCache(cacheTTL),
	}
}

// DefaultRolePermissionMapping returns the default role-permission mapping.
// This mapping aligns with the architecture spec RBAC table at docs/architecture.md:
//
// | Role           | Patent Mining       | Infringement Watch | Portfolio      | Lifecycle       | Collaboration          | Admin |
// |----------------|--------------------|--------------------|---------------|----------------|------------------------|-------|
// | Researcher     | Read + Search      | Read alerts        | Read own      | --             | Read own workspace     | --    |
// | IP Manager     | Full               | Full               | Full          | Full           | Manage workspaces      | --    |
// | Executive      | Read reports       | Read dashboards    | Read reports  | Read summaries | --                     | --    |
// | Partner Agent  | Scoped read        | Scoped alerts      | --            | Scoped lifecycle| Own workspace only    | --    |
// | System Admin   | --                 | --                 | --            | --             | --                     | Full  |
func DefaultRolePermissionMapping() RolePermissionMapping {
	allPerms := []Permission{
		PermPatentRead, PermPatentWrite, PermPatentDelete, PermPatentExport, PermPatentBulkImport,
		PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
		PermGraphRead, PermGraphWrite, PermGraphAdmin,
		PermUserRead, PermUserWrite, PermUserDelete, PermUserRoleAssign,
		PermSystemConfig, PermSystemMonitor, PermSystemAuditLog,
		PermTenantCreate, PermTenantRead, PermTenantUpdate, PermTenantDelete,
		PermAPIKeyCreate, PermAPIKeyRevoke, PermAPIRateConfig,
		PermReportRead, PermReportCreate, PermReportExport, PermDashboardRead,
	}

	readPerms := []Permission{
		PermPatentRead, PermAnalysisRead, PermGraphRead, PermUserRead, PermTenantRead,
		PermReportRead, PermDashboardRead,
	}

	return RolePermissionMapping{
		// System Admin: Full access to everything (maps to architecture's System Admin)
		RoleSuperAdmin: allPerms,

		// IP Manager: Full access to patent/analysis/graph/user/tenant operations
		// (maps to architecture's IP Manager)
		RoleTenantAdmin: []Permission{
			PermPatentRead, PermPatentWrite, PermPatentDelete, PermPatentExport, PermPatentBulkImport,
			PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
			PermGraphRead, PermGraphWrite, PermGraphAdmin,
			PermUserRead, PermUserWrite, PermUserDelete, PermUserRoleAssign,
			PermSystemMonitor, PermSystemAuditLog,
			PermTenantRead, PermTenantUpdate,
			PermAPIKeyCreate, PermAPIKeyRevoke,
			PermReportRead, PermReportCreate, PermReportExport, PermDashboardRead,
		},

		// ip_manager alias for TenantAdmin (architecture spec explicitly calls this role "IP Manager")
		RoleIPManager: []Permission{
			PermPatentRead, PermPatentWrite, PermPatentDelete, PermPatentExport, PermPatentBulkImport,
			PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
			PermGraphRead, PermGraphWrite, PermGraphAdmin,
			PermUserRead, PermUserWrite, PermUserDelete, PermUserRoleAssign,
			PermSystemMonitor, PermSystemAuditLog,
			PermTenantRead, PermTenantUpdate,
			PermAPIKeyCreate, PermAPIKeyRevoke,
			PermReportRead, PermReportCreate, PermReportExport, PermDashboardRead,
		},

		// Analyst: Read + Analyze + Export on most resources
		RoleAnalyst: []Permission{
			PermPatentRead, PermPatentExport,
			PermAnalysisCreate, PermAnalysisRead, PermAnalysisCancel, PermAnalysisExport,
			PermGraphRead,
			PermReportRead, PermReportExport, PermDashboardRead,
		},

		// Researcher: Read + Search on patents, read alerts, read own workspace
		// (maps to architecture's Researcher)
		RoleResearcher: []Permission{
			PermPatentRead,
			PermAnalysisRead,
			PermGraphRead,
			PermReportRead,
		},

		// Operator: System monitoring and audit
		RoleOperator: []Permission{
			PermSystemMonitor, PermSystemAuditLog,
			PermUserRead,
		},

		// Viewer: Read-only access to most resources
		RoleViewer: readPerms,

		// API User: Programmatic read access
		RoleAPIUser: []Permission{
			PermPatentRead, PermAnalysisRead, PermGraphRead,
		},

		// Executive: Read reports + dashboards (maps to architecture's Executive)
		RoleExecutive: []Permission{
			PermReportRead, PermReportExport,
			PermDashboardRead,
			PermPatentRead,
			PermAnalysisRead,
		},

		// Partner Agent: Scoped read access (maps to architecture's Partner Agent)
		RolePartnerAgent: []Permission{
			PermPatentRead,
			PermAnalysisRead,
			PermGraphRead,
			PermReportRead,
		},
	}
}

// ---------------------------------------------------------------------------
// permissionCache -- in-memory TTL-based cache for resolved permissions
// ---------------------------------------------------------------------------

type cacheEntry struct {
	permissions []Permission
	expiresAt   time.Time
}

type permissionCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

func newPermissionCache(ttl time.Duration) *permissionCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &permissionCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// cacheKey builds a string key from the context so we can key off (tenantID + userID + roles).
func cacheKeyFromRoles(userID, tenantID string, roles []Role) string {
	// Use a stable concatenation; roles are sorted (they come from the token in order).
	return fmt.Sprintf("%s|%s|%v", tenantID, userID, roles)
}

func (pc *permissionCache) get(key string) ([]Permission, bool) {
	pc.mu.RLock()
	entry, ok := pc.entries[key]
	pc.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		pc.mu.Lock()
		delete(pc.entries, key)
		pc.mu.Unlock()
		return nil, false
	}
	result := make([]Permission, len(entry.permissions))
	copy(result, entry.permissions)
	return result, true
}

func (pc *permissionCache) set(key string, perms []Permission) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	entry := &cacheEntry{
		permissions: make([]Permission, len(perms)),
		expiresAt:   time.Now().Add(pc.ttl),
	}
	copy(entry.permissions, perms)
	pc.entries[key] = entry
}

func (pc *permissionCache) invalidate(key string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	delete(pc.entries, key)
}

func (pc *permissionCache) invalidateAll() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.entries = make(map[string]*cacheEntry)
}

// ---------------------------------------------------------------------------
// rbacEnforcer method implementations
// ---------------------------------------------------------------------------

func (e *rbacEnforcer) UpdateMapping(mapping RolePermissionMapping) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rolePermissions = mapping
	e.permCache.invalidateAll()
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

func (e *rbacEnforcer) resolvePermissions(ctx context.Context) []Permission {
	roles := e.GetRoles(ctx)
	if len(roles) == 0 {
		return nil
	}

	// -- Try cache first --
	uid, _ := UserIDFromContext(ctx)
	tid, _ := TenantIDFromContext(ctx)
	ckey := cacheKeyFromRoles(uid, tid, roles)

	if cached, ok := e.permCache.get(ckey); ok {
		return cached
	}

	// -- Resolve from mapping --
	permMap := make(map[Permission]bool)
	for _, role := range roles {
		perms := e.getPermissionsForRole(role)
		for _, p := range perms {
			permMap[p] = true
		}
	}

	result := make([]Permission, 0, len(permMap))
	for p := range permMap {
		result = append(result, p)
	}

	// Cache for future calls
	e.permCache.set(ckey, result)
	return result
}

func (e *rbacEnforcer) GetPermissions(ctx context.Context) []Permission {
	return e.resolvePermissions(ctx)
}

func (e *rbacEnforcer) HasPermission(ctx context.Context, permission Permission) bool {
	if e.HasRole(ctx, RoleSuperAdmin) {
		return true
	}
	userPerms := e.resolvePermissions(ctx)
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
	userPerms := e.resolvePermissions(ctx)
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
	userPerms := e.resolvePermissions(ctx)
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

// InvalidatePermissionCache clears cached permissions for the current user in context.
func (e *rbacEnforcer) InvalidatePermissionCache(ctx context.Context) {
	roles := e.GetRoles(ctx)
	uid, _ := UserIDFromContext(ctx)
	tid, _ := TenantIDFromContext(ctx)
	ckey := cacheKeyFromRoles(uid, tid, roles)
	e.permCache.invalidate(ckey)
}

// ---------------------------------------------------------------------------
// Resource-level permission checking
// ---------------------------------------------------------------------------

// HasResourcePermission checks if the caller has the requested action on a
// specific resource.  The check layers:
//
//  1. Does the caller hold the base permission for the action?
//  2. Does tenant isolation allow access (SuperAdmin bypasses)?
//  3. If the resource has an owner_user_id and the caller is not SuperAdmin,
//     does the caller own the resource?  (Only enforced when OwnerUserID != "")
//
// When req.OwnerTenant is set, tenant isolation is enforced (unless the
// caller is SuperAdmin).  When req.OwnerUserID is set, ownership is checked.
func (e *rbacEnforcer) HasResourcePermission(ctx context.Context, req ResourcePermissionRequest) bool {
	// 1. Base permission check
	if !e.HasPermission(ctx, req.Action) {
		return false
	}

	// 2. Tenant isolation
	if req.OwnerTenant != "" {
		if err := e.EnforceTenantAccess(ctx, req.OwnerTenant); err != nil {
			return false
		}
	}

	// 3. Resource ownership -- only enforced when OwnerUserID is provided
	//    and the caller is NOT a SuperAdmin.
	if req.OwnerUserID != "" {
		if !e.HasRole(ctx, RoleSuperAdmin) {
			callerUID, ok := UserIDFromContext(ctx)
			if !ok || callerUID != req.OwnerUserID {
				return false
			}
		}
	}

	return true
}

// EnforceResourcePermission wraps HasResourcePermission and returns an error
// when access is denied.
func (e *rbacEnforcer) EnforceResourcePermission(ctx context.Context, req ResourcePermissionRequest) error {
	if !e.HasResourcePermission(ctx, req) {
		if req.ResourceID != "" {
			return errors.New(errors.ErrCodeForbidden,
				fmt.Sprintf("access denied to resource %s/%s for action %s",
					req.ResourceType, req.ResourceID, req.Action))
		}
		return ErrAccessDenied
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
			// Extract tenant_id from: X-Tenant-ID header (preferred) -> query param -> skip
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				tenantID = r.URL.Query().Get("tenant_id")
			}
			if tenantID == "" {
				// No explicit target tenant in request -- skip enforcement.
				// The handler may use the user's own tenant from the JWT context.
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

// RequireResourcePermission returns middleware that enforces resource-level access.
// It extracts the resource configuration from the provided function, which receives
// the HTTP request and returns the resource parameters to check.
func RequireResourcePermission(enforcer RBACEnforcer, fn func(r *http.Request) ResourcePermissionRequest) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := fn(r)
			if err := enforcer.EnforceResourcePermission(r.Context(), req); err != nil {
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
