package keycloak

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
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

func TestDefaultRolePermissionMapping_NewRoles(t *testing.T) {
	mapping := DefaultRolePermissionMapping()
	assert.Contains(t, mapping, RoleExecutive)
	assert.Contains(t, mapping, RolePartnerAgent)
	assert.Contains(t, mapping, RoleIPManager)
}

func TestHasPermission_Executive_ReportAndDashboard(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleExecutive)))

	// Should have report and dashboard permissions
	assert.True(t, e.HasPermission(ctx, PermReportRead))
	assert.True(t, e.HasPermission(ctx, PermReportExport))
	assert.True(t, e.HasPermission(ctx, PermDashboardRead))

	// Should NOT have write/delete/management permissions
	assert.False(t, e.HasPermission(ctx, PermPatentWrite))
	assert.False(t, e.HasPermission(ctx, PermPatentDelete))
	assert.False(t, e.HasPermission(ctx, PermUserWrite))
	assert.False(t, e.HasPermission(ctx, PermSystemConfig))
}

func TestHasPermission_PartnerAgent_ScopedRead(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RolePartnerAgent)))

	// Should have read permissions
	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.True(t, e.HasPermission(ctx, PermAnalysisRead))
	assert.True(t, e.HasPermission(ctx, PermGraphRead))

	// Should NOT have write/create/admin permissions
	assert.False(t, e.HasPermission(ctx, PermPatentWrite))
	assert.False(t, e.HasPermission(ctx, PermAnalysisCreate))
	assert.False(t, e.HasPermission(ctx, PermGraphAdmin))
	assert.False(t, e.HasPermission(ctx, PermUserDelete))
}

func TestHasPermission_IPManager_FullAccess(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleIPManager)))

	// Should have broad permissions
	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.True(t, e.HasPermission(ctx, PermPatentWrite))
	assert.True(t, e.HasPermission(ctx, PermPatentDelete))
	assert.True(t, e.HasPermission(ctx, PermAnalysisCreate))
	assert.True(t, e.HasPermission(ctx, PermUserRead))
	assert.True(t, e.HasPermission(ctx, PermReportRead))
	assert.True(t, e.HasPermission(ctx, PermDashboardRead))

	// Should NOT have system-config-level permissions
	assert.False(t, e.HasPermission(ctx, PermSystemConfig))
}

func TestHasPermission_Executive_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleExecutive)))

	assert.False(t, e.HasPermission(ctx, PermPatentWrite))
	assert.False(t, e.HasPermission(ctx, PermAnalysisCreate))
	assert.False(t, e.HasPermission(ctx, PermUserDelete))
	assert.False(t, e.HasPermission(ctx, PermSystemConfig))
}

func TestHasPermission_PartnerAgent_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RolePartnerAgent)))

	assert.False(t, e.HasPermission(ctx, PermPatentWrite))
	assert.False(t, e.HasPermission(ctx, PermUserWrite))
	assert.False(t, e.HasPermission(ctx, PermSystemMonitor))
}

// --- Permission Cache Tests ---

func TestPermissionCache_Hit(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	// First call populates cache
	perms1 := e.GetPermissions(ctx)
	assert.NotEmpty(t, perms1)

	// Second call should hit cache and return same set
	perms2 := e.GetPermissions(ctx)
	assert.Equal(t, len(perms1), len(perms2))
	for _, p := range perms1 {
		assert.Contains(t, perms2, p)
	}
}

func TestPermissionCache_Invalidate(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	// Populate cache
	_ = e.GetPermissions(ctx)

	// Invalidate
	e.InvalidatePermissionCache(ctx)

	// Ensure new permissions still resolve correctly after invalidation
	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.False(t, e.HasPermission(ctx, PermSystemConfig))
}

func TestPermissionCache_MappingUpdateInvalidates(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))

	// Before update - analyst does not have system:config
	assert.False(t, e.HasPermission(ctx, PermSystemConfig))

	// Update mapping
	newMapping := DefaultRolePermissionMapping()
	newMapping[RoleAnalyst] = append(newMapping[RoleAnalyst], PermSystemConfig)
	e.UpdateMapping(newMapping)

	// After update - should now have access (cache was invalidated by UpdateMapping)
	assert.True(t, e.HasPermission(ctx, PermSystemConfig))
}

// --- Resource-Level Permission Checking Tests ---

func TestHasResourcePermission_SameTenant(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-123",
		Action:       PermPatentRead,
		OwnerTenant:  "tenant-1",
	}
	assert.True(t, e.HasResourcePermission(ctx, req))
}

func TestHasResourcePermission_CrossTenant_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-456",
		Action:       PermPatentRead,
		OwnerTenant:  "tenant-2",
	}
	assert.False(t, e.HasResourcePermission(ctx, req))
}

func TestHasResourcePermission_CrossTenant_SuperAdminBypasses(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleSuperAdmin)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-789",
		Action:       PermPatentRead,
		OwnerTenant:  "tenant-2",
	}
	assert.True(t, e.HasResourcePermission(ctx, req))
}

func TestHasResourcePermission_OwnershipCheck(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleResearcher)))

	// Own resource
	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-own",
		Action:       PermPatentRead,
		OwnerUserID:  "user-1",
	}
	assert.True(t, e.HasResourcePermission(ctx, req))

	// Someone else's resource
	req2 := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-other",
		Action:       PermPatentRead,
		OwnerUserID:  "user-2",
	}
	assert.False(t, e.HasResourcePermission(ctx, req2))
}

func TestHasResourcePermission_OwnershipBypassedBySuperAdmin(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleSuperAdmin)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-any",
		Action:       PermPatentRead,
		OwnerUserID:  "some-other-user",
	}
	assert.True(t, e.HasResourcePermission(ctx, req))
}

func TestHasResourcePermission_NoTenantNoOwner(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleResearcher)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-open",
		Action:       PermPatentRead,
		// No OwnerTenant, no OwnerUserID -- only base permission check
	}
	assert.True(t, e.HasResourcePermission(ctx, req))
}

func TestHasResourcePermission_MissingBasePermission(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleResearcher)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-123",
		Action:       PermPatentWrite, // Researcher does not have write
		OwnerTenant:  "tenant-1",
	}
	assert.False(t, e.HasResourcePermission(ctx, req))
}

func TestHasResourcePermission_NoAuthContext(t *testing.T) {
	e := newTestEnforcer()
	ctx := context.Background()

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-123",
		Action:       PermPatentRead,
	}
	assert.False(t, e.HasResourcePermission(ctx, req))
}

func TestHasResourcePermission_TenantCheckCombinedWithOwnership(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleResearcher)))

	// Same tenant, own resource
	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-xyz",
		Action:       PermPatentRead,
		OwnerTenant:  "tenant-1",
		OwnerUserID:  "user-1",
	}
	assert.True(t, e.HasResourcePermission(ctx, req))

	// Same tenant, not own resource
	req2 := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-xyz",
		Action:       PermPatentRead,
		OwnerTenant:  "tenant-1",
		OwnerUserID:  "user-other",
	}
	assert.False(t, e.HasResourcePermission(ctx, req2))
}

func TestEnforceResourcePermission_Allowed(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-123",
		Action:       PermPatentRead,
		OwnerTenant:  "tenant-1",
	}
	err := e.EnforceResourcePermission(ctx, req)
	assert.NoError(t, err)
}

func TestEnforceResourcePermission_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleResearcher)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-123",
		Action:       PermPatentWrite, // Researcher cannot write
		OwnerTenant:  "tenant-1",
	}
	err := e.EnforceResourcePermission(ctx, req)
	assert.Error(t, err)
	assert.Equal(t, errors.ErrCodeForbidden, err.(*errors.AppError).Code)
}

func TestEnforceResourcePermission_CrossTenant(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	req := ResourcePermissionRequest{
		ResourceType: ResourcePatent,
		ResourceID:   "patent-123",
		Action:       PermPatentRead,
		OwnerTenant:  "tenant-other",
	}
	err := e.EnforceResourcePermission(ctx, req)
	assert.Error(t, err)
}

func TestRequireResourcePermission_Middleware_Allowed(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleResearcher)))

	mw := RequireResourcePermission(e, func(r *http.Request) ResourcePermissionRequest {
		return ResourcePermissionRequest{
			ResourceType: ResourcePatent,
			ResourceID:   "patent-123",
			Action:       PermPatentRead,
			OwnerTenant:  "tenant-1",
		}
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/patents/patent-123", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireResourcePermission_Middleware_Denied(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleResearcher)))

	mw := RequireResourcePermission(e, func(r *http.Request) ResourcePermissionRequest {
		return ResourcePermissionRequest{
			ResourceType: ResourcePatent,
			ResourceID:   "patent-123",
			Action:       PermPatentWrite, // Researcher cannot write
			OwnerTenant:  "tenant-1",
		}
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/patents/patent-123", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireResourcePermission_Middleware_CrossTenant(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleResearcher)))

	mw := RequireResourcePermission(e, func(r *http.Request) ResourcePermissionRequest {
		return ResourcePermissionRequest{
			ResourceType: ResourcePatent,
			ResourceID:   "patent-123",
			Action:       PermPatentRead,
			OwnerTenant:  "tenant-other",
		}
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/patents/patent-123", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestNewRBACEnforcerWithCache_CustomTTL(t *testing.T) {
	logger := logging.NewNopLogger()
	mapping := DefaultRolePermissionMapping()
	e := NewRBACEnforcerWithCache(mapping, logger, 1*time.Second)
	assert.NotNil(t, e)

	ctx := contextWithClaims(newClaimsWithRoles(string(RoleAnalyst)))
	assert.True(t, e.HasPermission(ctx, PermPatentRead))
}

func TestPermissionCache_TTLExpiry(t *testing.T) {
	cache := newPermissionCache(50 * time.Millisecond)
	perms := []Permission{PermPatentRead, PermAnalysisRead}
	cache.set("test-key|user-1|tenant-1|", perms)

	// Immediate hit
	got, ok := cache.get("test-key|user-1|tenant-1|")
	assert.True(t, ok)
	assert.Equal(t, perms, got)

	// Wait for TTL to expire
	time.Sleep(100 * time.Millisecond)

	// Should miss
	got, ok = cache.get("test-key|user-1|tenant-1|")
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestPermissionCache_InvalidateAll(t *testing.T) {
	cache := newPermissionCache(5 * time.Minute)
	cache.set("key1|user-1|t1|", []Permission{PermPatentRead})
	cache.set("key2|user-2|t2|", []Permission{PermAnalysisRead})

	cache.invalidateAll()

	_, ok := cache.get("key1|user-1|t1|")
	assert.False(t, ok)
	_, ok = cache.get("key2|user-2|t2|")
	assert.False(t, ok)
}

// --- Role-Based Route Guard Tests (Architecture Spec Alignment) ---

func TestRouteGuard_Researcher(t *testing.T) {
	e := newTestEnforcer()
	// Architecture: Researcher = Read + Search on patents, read alerts, read own workspace
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleResearcher)))

	assert.True(t, e.HasPermission(ctx, PermPatentRead), "Researcher should read patents")
	assert.True(t, e.HasPermission(ctx, PermAnalysisRead), "Researcher should read analysis/alerts")
	assert.True(t, e.HasPermission(ctx, PermGraphRead), "Researcher should read graphs")

	assert.False(t, e.HasPermission(ctx, PermPatentWrite), "Researcher should NOT write patents")
	assert.False(t, e.HasPermission(ctx, PermPatentDelete), "Researcher should NOT delete patents")
	assert.False(t, e.HasPermission(ctx, PermUserWrite), "Researcher should NOT manage users")
}

func TestRouteGuard_IPManager(t *testing.T) {
	e := newTestEnforcer()
	// Architecture: IP Manager = Full access on patent mining/infringement/portfolio/lifecycle
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleIPManager)))

	assert.True(t, e.HasPermission(ctx, PermPatentRead), "IP Manager should read patents")
	assert.True(t, e.HasPermission(ctx, PermPatentWrite), "IP Manager should write patents")
	assert.True(t, e.HasPermission(ctx, PermPatentDelete), "IP Manager should delete patents")
	assert.True(t, e.HasPermission(ctx, PermAnalysisCreate), "IP Manager should create analyses")
	assert.True(t, e.HasPermission(ctx, PermAnalysisCancel), "IP Manager should cancel analyses")
	assert.True(t, e.HasPermission(ctx, PermReportRead), "IP Manager should read reports")
	assert.True(t, e.HasPermission(ctx, PermDashboardRead), "IP Manager should read dashboards")

	assert.False(t, e.HasPermission(ctx, PermSystemConfig), "IP Manager should NOT configure system")
}

func TestRouteGuard_Executive(t *testing.T) {
	e := newTestEnforcer()
	// Architecture: Executive = Read reports + dashboards
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleExecutive)))

	assert.True(t, e.HasPermission(ctx, PermReportRead), "Executive should read reports")
	assert.True(t, e.HasPermission(ctx, PermReportExport), "Executive should export reports")
	assert.True(t, e.HasPermission(ctx, PermDashboardRead), "Executive should read dashboards")
	assert.True(t, e.HasPermission(ctx, PermPatentRead), "Executive should read patent summaries")
	assert.True(t, e.HasPermission(ctx, PermAnalysisRead), "Executive should read analysis summaries")

	assert.False(t, e.HasPermission(ctx, PermPatentWrite), "Executive should NOT write patents")
	assert.False(t, e.HasPermission(ctx, PermPatentDelete), "Executive should NOT delete patents")
	assert.False(t, e.HasPermission(ctx, PermAnalysisCreate), "Executive should NOT create analyses")
}

func TestRouteGuard_PartnerAgent(t *testing.T) {
	e := newTestEnforcer()
	// Architecture: Partner Agent = Scoped read
	ctx := contextWithClaims(newClaimsWithRoles(string(RolePartnerAgent)))

	assert.True(t, e.HasPermission(ctx, PermPatentRead), "Partner Agent should read patents")
	assert.True(t, e.HasPermission(ctx, PermAnalysisRead), "Partner Agent should read alerts")
	assert.True(t, e.HasPermission(ctx, PermGraphRead), "Partner Agent should read graphs")

	assert.False(t, e.HasPermission(ctx, PermPatentWrite), "Partner Agent should NOT write patents")
	assert.False(t, e.HasPermission(ctx, PermPatentExport), "Partner Agent should NOT export patents")
	assert.False(t, e.HasPermission(ctx, PermAnalysisCreate), "Partner Agent should NOT create analyses")
}

func TestRouteGuard_SystemAdmin(t *testing.T) {
	e := newTestEnforcer()
	// Architecture: System Admin = Full on everything
	ctx := contextWithClaims(newClaimsWithRoles(string(RoleSuperAdmin)))

	assert.True(t, e.HasPermission(ctx, PermSystemConfig), "System Admin should configure system")
	assert.True(t, e.HasPermission(ctx, PermSystemMonitor), "System Admin should monitor")
	assert.True(t, e.HasPermission(ctx, PermUserRoleAssign), "System Admin should assign roles")
	assert.True(t, e.HasPermission(ctx, PermTenantCreate), "System Admin should create tenants")
	assert.True(t, e.HasPermission(ctx, PermTenantDelete), "System Admin should delete tenants")
	assert.True(t, e.HasPermission(ctx, PermAPIKeyCreate), "System Admin should create API keys")
}

func TestRequireTenantAccess_XTenantIDHeader(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	mw := RequireTenantAccess(e)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test with X-Tenant-ID header matching the user's tenant
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	req.Header.Set("X-Tenant-ID", "tenant-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test with X-Tenant-ID header for a different tenant
	req2 := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	req2.Header.Set("X-Tenant-ID", "tenant-2")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}

func TestRequireTenantAccess_QueryParamFallback(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithClaims(newClaimsWithTenant("tenant-1", string(RoleAnalyst)))

	mw := RequireTenantAccess(e)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test with tenant_id query param matching the user's tenant
	req := httptest.NewRequest("GET", "/?tenant_id=tenant-1", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewRBACEnforcer_NilMapping_UsesDefault(t *testing.T) {
	e := NewRBACEnforcer(nil, logging.NewNopLogger())
	assert.NotNil(t, e)

	ctx := contextWithClaims(newClaimsWithRoles(string(RoleSuperAdmin)))
	assert.True(t, e.HasPermission(ctx, PermPatentRead))
}

//Personal.AI order the ending
