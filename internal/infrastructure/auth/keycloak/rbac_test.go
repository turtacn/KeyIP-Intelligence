package keycloak

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func contextWithClaims(claims *TokenClaims) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyClaims, claims)
	ctx = context.WithValue(ctx, ContextKeyUserID, claims.Subject)
	if claims.TenantID != "" {
		ctx = context.WithValue(ctx, ContextKeyTenantID, claims.TenantID)
	}
	var roles []string
	roles = append(roles, claims.RealmRoles...)
	for _, r := range claims.ClientRoles {
		roles = append(roles, r...)
	}
	ctx = context.WithValue(ctx, ContextKeyRoles, roles)
	return ctx
}

func newTestEnforcer() RBACEnforcer {
	return NewRBACEnforcer(nil, logging.NewNopLogger())
}

func newClaimsWithRoles(roles ...string) *TokenClaims {
	return &TokenClaims{
		Subject:    "user-1",
		TenantID:   "tenant-1",
		RealmRoles: roles,
	}
}

func newClaimsWithTenant(tenantID string, roles ...string) *TokenClaims {
	return &TokenClaims{
		Subject:    "user-1",
		TenantID:   tenantID,
		RealmRoles: roles,
	}
}

func TestHasPermission_SuperAdmin_AllPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleSuperAdmin)))

	// Check random permissions
	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.True(t, e.HasPermission(ctx, PermSystemConfig))
	assert.True(t, e.HasPermission(ctx, "unknown:permission")) // SuperAdmin has everything by definition?
	// Wait, HasPermission logic: if HasRole(SuperAdmin) returns true.
	// Yes, I implemented it to return true for ANY permission.
}

func TestHasPermission_Analyst_AllowedPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.True(t, e.HasPermission(ctx, PermAnalysisCreate))
}

func TestHasPermission_Analyst_DeniedPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.False(t, e.HasPermission(ctx, PermUserDelete))
	assert.False(t, e.HasPermission(ctx, PermSystemConfig))
}

func TestHasPermission_Researcher_LimitedPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleResearcher)))

	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.False(t, e.HasPermission(ctx, PermPatentWrite))
}

func TestHasPermission_Viewer_ReadOnlyPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleViewer)))

	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.True(t, e.HasPermission(ctx, PermAnalysisRead))
	assert.False(t, e.HasPermission(ctx, PermAnalysisCreate))
}

func TestHasPermission_Operator_SystemPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleOperator)))

	assert.True(t, e.HasPermission(ctx, PermSystemMonitor))
	assert.True(t, e.HasPermission(ctx, PermSystemAuditLog))
	assert.False(t, e.HasPermission(ctx, PermPatentWrite))
}

func TestHasPermission_MultipleRoles_UnionPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst), string(RoleOperator)))

	// Analyst permissions
	assert.True(t, e.HasPermission(ctx, PermAnalysisCreate))
	// Operator permissions
	assert.True(t, e.HasPermission(ctx, PermSystemMonitor))
}

func TestHasPermission_NoRoles_NonePermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles())

	assert.False(t, e.HasPermission(ctx, PermPatentRead))
}

func TestHasPermission_UnknownRole_Ignored(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles("unknown-role"))

	assert.False(t, e.HasPermission(ctx, PermPatentRead))
}

func TestHasPermission_NoAuthContext(t *testing.T) {
	e := newTestEnforcer()
	ctx := context.Background()

	assert.False(t, e.HasPermission(ctx, PermPatentRead))
}

func TestHasAllPermissions_AllPresent(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.True(t, e.HasAllPermissions(ctx, PermPatentRead, PermAnalysisCreate))
}

func TestHasAllPermissions_OneMissing(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.False(t, e.HasAllPermissions(ctx, PermPatentRead, PermSystemConfig))
}

func TestHasAnyPermission_OnePresent(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.True(t, e.HasAnyPermission(ctx, PermSystemConfig, PermPatentRead))
}

func TestHasAnyPermission_NonePresent(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.False(t, e.HasAnyPermission(ctx, PermSystemConfig, PermUserDelete))
}

func TestHasRole_Exists(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.True(t, e.HasRole(ctx, RoleAnalyst))
}

func TestHasRole_NotExists(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.False(t, e.HasRole(ctx, RoleSuperAdmin))
}

func TestGetPermissions_ReturnsAll(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	perms := e.GetPermissions(ctx)
	assert.Contains(t, perms, PermPatentRead)
	assert.Contains(t, perms, PermAnalysisCreate)
}

func TestGetRoles_ReturnsAll(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst), string(RoleOperator)))

	roles := e.GetRoles(ctx)
	assert.Contains(t, roles, RoleAnalyst)
	assert.Contains(t, roles, RoleOperator)
}

func TestEnforcePermission_Allowed(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	err := e.EnforcePermission(ctx, PermPatentRead)
	assert.NoError(t, err)
}

func TestEnforcePermission_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	err := e.EnforcePermission(ctx, PermSystemConfig)
	assert.Error(t, err)
	assert.Equal(t, ErrAccessDenied, err)
}

func TestEnforceTenantAccess_SameTenant(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	err := e.EnforceTenantAccess(ctx, "tenant-1")
	assert.NoError(t, err)
}

func TestEnforceTenantAccess_CrossTenant_NonAdmin(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	err := e.EnforceTenantAccess(ctx, "tenant-2")
	assert.Error(t, err)
	assert.Equal(t, ErrCrossTenantAccess, err)
}

func TestEnforceTenantAccess_CrossTenant_SuperAdmin(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleSuperAdmin)))

	err := e.EnforceTenantAccess(ctx, "tenant-2")
	assert.NoError(t, err)
}

func TestEnforceTenantAccess_EmptyTenantID(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	err := e.EnforceTenantAccess(ctx, "")
	assert.Error(t, err)
}

func TestRequirePermission_Middleware_Allowed(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	mw := RequirePermission(e, PermPatentRead)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequirePermission_Middleware_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	mw := RequirePermission(e, PermSystemConfig)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequirePermission_Middleware_NoAuth(t *testing.T) {
	e := newTestEnforcer()
	ctx := context.Background()

	mw := RequirePermission(e, PermPatentRead)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code) // ErrAccessDenied defaults to Forbidden in handleRBACError
	// Actually, EnforcePermission calls HasPermission which calls GetPermissions which returns nil.
	// HasPermission returns false.
	// EnforcePermission returns ErrAccessDenied.
	// handleRBACError checks ErrNoAuthContext? No.
	// ErrAccessDenied is mapped to 403.
}

func TestRequireAnyPermission_Middleware(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	mw := RequireAnyPermission(e, PermSystemConfig, PermPatentRead)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Middleware_Allowed(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	mw := RequireRole(e, RoleAnalyst)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireRole_Middleware_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	mw := RequireRole(e, RoleSuperAdmin)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestUpdateMapping_DynamicUpdate(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	assert.False(t, e.HasPermission(ctx, PermSystemConfig))

	newMapping := DefaultRolePermissionMapping()
	newMapping[RoleAnalyst] = append(newMapping[RoleAnalyst], PermSystemConfig)

	e.UpdateMapping(newMapping)

	assert.True(t, e.HasPermission(ctx, PermSystemConfig))
}

func TestDefaultRolePermissionMapping_Completeness(t *testing.T) {
	mapping := DefaultRolePermissionMapping()
	assert.Contains(t, mapping, RoleSuperAdmin)
	assert.Contains(t, mapping, RoleTenantAdmin)
	assert.Contains(t, mapping, RoleAnalyst)
	assert.Contains(t, mapping, RoleResearcher)
	assert.Contains(t, mapping, RoleOperator)
	assert.Contains(t, mapping, RoleViewer)
	assert.Contains(t, mapping, RoleAPIUser)
}

func TestDefaultRolePermissionMapping_SuperAdminHasAll(t *testing.T) {
	// Although we shortcut SuperAdmin in HasPermission, verifying mapping is good practice
	mapping := DefaultRolePermissionMapping()
	superPerms := mapping[RoleSuperAdmin]
	// Should have at least one of each category
	assert.Contains(t, superPerms, PermPatentRead)
	assert.Contains(t, superPerms, PermSystemConfig)
}

//Personal.AI order the ending
