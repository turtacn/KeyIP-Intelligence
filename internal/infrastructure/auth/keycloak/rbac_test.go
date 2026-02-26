package keycloak

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"sync"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func contextWithRoles(roles ...string) context.Context {
	return context.WithValue(context.Background(), ContextKeyRoles, roles)
}

func contextWithTenant(tenantID string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyTenantID, tenantID)
	return ctx
}

func contextWithAuth(tenantID string, roles ...string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, ContextKeyRoles, roles)
	return ctx
}

func newTestEnforcer() RBACEnforcer {
	return NewRBACEnforcer(nil, newMockLogger())
}

func TestHasPermission_SuperAdmin_AllPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithRoles("super_admin")

	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.True(t, e.HasPermission(ctx, PermSystemConfig))
	assert.True(t, e.HasPermission(ctx, "unknown:permission")) // SuperAdmin has all? Implementation says yes
}

func TestHasPermission_Analyst_AllowedPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithRoles("analyst")

	assert.True(t, e.HasPermission(ctx, PermPatentRead))
	assert.True(t, e.HasPermission(ctx, PermAnalysisCreate))
	assert.False(t, e.HasPermission(ctx, PermUserDelete))
}

func TestHasPermission_MultipleRoles_UnionPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithRoles("analyst", "operator")

	assert.True(t, e.HasPermission(ctx, PermAnalysisCreate)) // From Analyst
	assert.True(t, e.HasPermission(ctx, PermSystemMonitor))  // From Operator
}

func TestHasPermission_NoRoles_NonePermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := context.Background()

	assert.False(t, e.HasPermission(ctx, PermPatentRead))
}

func TestHasAllPermissions(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithRoles("analyst")

	assert.True(t, e.HasAllPermissions(ctx, PermPatentRead, PermAnalysisCreate))
	assert.False(t, e.HasAllPermissions(ctx, PermPatentRead, PermUserDelete))
}

func TestHasAnyPermission(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithRoles("analyst")

	assert.True(t, e.HasAnyPermission(ctx, PermUserDelete, PermPatentRead))
	assert.False(t, e.HasAnyPermission(ctx, PermUserDelete, PermSystemConfig))
}

func TestEnforceTenantAccess_SameTenant(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithAuth("tenant-1", "analyst")

	err := e.EnforceTenantAccess(ctx, "tenant-1")
	assert.NoError(t, err)
}

func TestEnforceTenantAccess_CrossTenant_NonAdmin(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithAuth("tenant-1", "analyst")

	err := e.EnforceTenantAccess(ctx, "tenant-2")
	assert.Error(t, err)
	assert.True(t, errors.IsForbidden(err))
}

func TestEnforceTenantAccess_CrossTenant_SuperAdmin(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithAuth("tenant-1", "super_admin")

	err := e.EnforceTenantAccess(ctx, "tenant-2")
	assert.NoError(t, err)
}

func TestRequirePermission_Middleware_Allowed(t *testing.T) {
	e := newTestEnforcer()

	req := httptest.NewRequest("GET", "/", nil)
	// Inject context manually as middleware is not running previous steps
	ctx := contextWithRoles("analyst")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler := e.RequirePermission(PermPatentRead)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRequirePermission_Middleware_Denied(t *testing.T) {
	e := newTestEnforcer()

	req := httptest.NewRequest("GET", "/", nil)
	ctx := contextWithRoles("analyst")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler := e.RequirePermission(PermUserDelete)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestUpdateMapping(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithRoles("viewer")

	assert.False(t, e.HasPermission(ctx, PermPatentWrite))

	newMapping := make(RolePermissionMapping)
	newMapping[RoleViewer] = []Permission{PermPatentWrite}
	e.UpdateMapping(newMapping)

	assert.True(t, e.HasPermission(ctx, PermPatentWrite))
}

func TestConcurrentMappingUpdate(t *testing.T) {
	e := newTestEnforcer()
	ctx := contextWithRoles("viewer")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.HasPermission(ctx, PermPatentRead)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.UpdateMapping(DefaultRolePermissionMapping())
		}()
	}
	wg.Wait()
}
//Personal.AI order the ending
