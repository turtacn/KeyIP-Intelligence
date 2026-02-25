// ---
//
// 继续输出 283 `internal/interfaces/http/router_test.go` 要实现 HTTP 路由注册单元测试。
//
// 实现要求:
//
// * **功能定位**：验证 NewRouter 构建的路由树完整性、中间件链顺序、路由分组隔离
// * **测试用例**：
//   * `TestNewRouter_HealthEndpoints_NoAuth`：健康检查端点不经过认证中间件
//   * `TestNewRouter_APIv1_RequiresAuth`：API v1 路由组经过认证中间件
//   * `TestNewRouter_MoleculeRoutes_Registered`：分子资源路由完整注册
//   * `TestNewRouter_PatentRoutes_Registered`：专利资源路由完整注册
//   * `TestNewRouter_PortfolioRoutes_Registered`：组合资源路由完整注册
//   * `TestNewRouter_LifecycleRoutes_Registered`：生命周期路由完整注册
//   * `TestNewRouter_CollaborationRoutes_Registered`：协作路由完整注册
//   * `TestNewRouter_ReportRoutes_Registered`：报告路由完整注册
//   * `TestNewRouter_NilHandlers_NoPanic`：Handler 为 nil 时不 panic
//   * `TestNewRouter_MiddlewareOrder`：中间件按 Recovery→CORS→Logging→RateLimit 顺序
//   * `TestNewRouter_GlobalMiddleware_Applied`：全局中间件对所有路由生效
// * **Mock 依赖**：stubHandler（返回固定状态码的 http.HandlerFunc）、stubMiddleware
// * **断言验证**：httptest 请求路由匹配、状态码、中间件执行标记 Header
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
//
// ---
package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
)

// stubLogger implements logging.Logger for testing.
type stubLogger struct{}

func (s *stubLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (s *stubLogger) Info(msg string, keysAndValues ...interface{})  {}
func (s *stubLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (s *stubLogger) Error(msg string, keysAndValues ...interface{}) {}

// newMinimalHealthHandler creates a HealthHandler with stub dependencies.
func newMinimalHealthHandler() *handlers.HealthHandler {
	return handlers.NewHealthHandler(nil, &stubLogger{})
}

// newTrackingAuthMiddleware creates an AuthMiddleware that sets a header
// to prove it was invoked, then passes through.
func newTrackingAuthMiddleware() *middleware.AuthMiddleware {
	return middleware.NewTrackingAuthMiddleware("X-Auth-Applied")
}

// newTrackingTenantMiddleware creates a TenantMiddleware that sets a header.
func newTrackingTenantMiddleware() *middleware.TenantMiddleware {
	return middleware.NewTrackingTenantMiddleware("X-Tenant-Applied")
}

func TestNewRouter_HealthEndpoints_NoAuth(t *testing.T) {
	cfg := RouterConfig{
		HealthHandler:  newMinimalHealthHandler(),
		AuthMiddleware: newTrackingAuthMiddleware(),
		Logger:         &stubLogger{},
	}
	router := NewRouter(cfg)

	// /healthz should respond without auth header
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("X-Auth-Applied"),
		"health endpoint must not pass through auth middleware")
}

func TestNewRouter_HealthEndpoints_Readiness(t *testing.T) {
	cfg := RouterConfig{
		HealthHandler: newMinimalHealthHandler(),
		Logger:        &stubLogger{},
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNewRouter_APIv1_RequiresAuth(t *testing.T) {
	cfg := RouterConfig{
		HealthHandler:  newMinimalHealthHandler(),
		AuthMiddleware: newTrackingAuthMiddleware(),
		MoleculeHandler: handlers.NewStubMoleculeHandler(),
		Logger:         &stubLogger{},
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, "true", rec.Header().Get("X-Auth-Applied"),
		"API v1 routes must pass through auth middleware")
}

func TestNewRouter_MoleculeRoutes_Registered(t *testing.T) {
	cfg := RouterConfig{
		MoleculeHandler: handlers.NewStubMoleculeHandler(),
		Logger:          &stubLogger{},
	}
	router := NewRouter(cfg)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/molecules"},
		{http.MethodPost, "/api/v1/molecules"},
		{http.MethodGet, "/api/v1/molecules/mol-123"},
		{http.MethodPut, "/api/v1/molecules/mol-123"},
		{http.MethodDelete, "/api/v1/molecules/mol-123"},
		{http.MethodPost, "/api/v1/molecules/search/similar"},
		{http.MethodPost, "/api/v1/molecules/predict/properties"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusNotFound, rec.Code,
				"route %s %s should be registered", rt.method, rt.path)
		})
	}
}

func TestNewRouter_PatentRoutes_Registered(t *testing.T) {
	cfg := RouterConfig{
		PatentHandler: handlers.NewStubPatentHandler(),
		Logger:        &stubLogger{},
	}
	router := NewRouter(cfg)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/patents"},
		{http.MethodPost, "/api/v1/patents"},
		{http.MethodGet, "/api/v1/patents/search"},
		{http.MethodGet, "/api/v1/patents/CN115123456A"},
		{http.MethodPut, "/api/v1/patents/CN115123456A"},
		{http.MethodDelete, "/api/v1/patents/CN115123456A"},
		{http.MethodGet, "/api/v1/patents/CN115123456A/claims"},
		{http.MethodGet, "/api/v1/patents/CN115123456A/family"},
		{http.MethodGet, "/api/v1/patents/CN115123456A/citations"},
		{http.MethodPost, "/api/v1/patents/fto/check"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestNewRouter_PortfolioRoutes_Registered(t *testing.T) {
	cfg := RouterConfig{
		PortfolioHandler: handlers.NewStubPortfolioHandler(),
		Logger:           &stubLogger{},
	}
	router := NewRouter(cfg)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/portfolios"},
		{http.MethodPost, "/api/v1/portfolios"},
		{http.MethodGet, "/api/v1/portfolios/pf-1"},
		{http.MethodPut, "/api/v1/portfolios/pf-1"},
		{http.MethodDelete, "/api/v1/portfolios/pf-1"},
		{http.MethodGet, "/api/v1/portfolios/pf-1/valuation"},
		{http.MethodPost, "/api/v1/portfolios/pf-1/valuation"},
		{http.MethodGet, "/api/v1/portfolios/pf-1/gaps"},
		{http.MethodPost, "/api/v1/portfolios/pf-1/gaps"},
		{http.MethodPost, "/api/v1/portfolios/pf-1/optimize"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestNewRouter_LifecycleRoutes_Registered(t *testing.T) {
	cfg := RouterConfig{
		LifecycleHandler: handlers.NewStubLifecycleHandler(),
		Logger:           &stubLogger{},
	}
	router := NewRouter(cfg)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/lifecycle/deadlines"},
		{http.MethodGet, "/api/v1/lifecycle/deadlines/upcoming"},
		{http.MethodGet, "/api/v1/lifecycle/annuities"},
		{http.MethodPost, "/api/v1/lifecycle/annuities/calculate"},
		{http.MethodGet, "/api/v1/lifecycle/annuities/budget"},
		{http.MethodGet, "/api/v1/lifecycle/legal-status/CN115123456A"},
		{http.MethodPost, "/api/v1/lifecycle/legal-status/sync"},
		{http.MethodGet, "/api/v1/lifecycle/calendar"},
		{http.MethodGet, "/api/v1/lifecycle/calendar/export"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestNewRouter_CollaborationRoutes_Registered(t *testing.T) {
	cfg := RouterConfig{
		CollaborationHandler: handlers.NewStubCollaborationHandler(),
		Logger:               &stubLogger{},
	}
	router := NewRouter(cfg)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/collaboration/workspaces"},
		{http.MethodPost, "/api/v1/collaboration/workspaces"},
		{http.MethodGet, "/api/v1/collaboration/workspaces/ws-1"},
		{http.MethodPut, "/api/v1/collaboration/workspaces/ws-1"},
		{http.MethodDelete, "/api/v1/collaboration/workspaces/ws-1"},
		{http.MethodGet, "/api/v1/collaboration/workspaces/ws-1/members"},
		{http.MethodPost, "/api/v1/collaboration/workspaces/ws-1/members"},
		{http.MethodDelete, "/api/v1/collaboration/workspaces/ws-1/members/user-1"},
		{http.MethodPost, "/api/v1/collaboration/share"},
		{http.MethodGet, "/api/v1/collaboration/share/tok-abc"},
		{http.MethodDelete, "/api/v1/collaboration/share/tok-abc"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestNewRouter_ReportRoutes_Registered(t *testing.T) {
	cfg := RouterConfig{
		ReportHandler: handlers.NewStubReportHandler(),
		Logger:        &stubLogger{},
	}
	router := NewRouter(cfg)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/reports"},
		{http.MethodPost, "/api/v1/reports/generate"},
		{http.MethodGet, "/api/v1/reports/rpt-1"},
		{http.MethodGet, "/api/v1/reports/rpt-1/download"},
		{http.MethodDelete, "/api/v1/reports/rpt-1"},
		{http.MethodGet, "/api/v1/reports/templates"},
		{http.MethodGet, "/api/v1/reports/templates/tpl-1"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			req := httptest.NewRequest(rt.method, rt.path, nil)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			assert.NotEqual(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestNewRouter_NilHandlers_NoPanic(t *testing.T) {
	cfg := RouterConfig{
		Logger: &stubLogger{},
	}

	assert.NotPanics(t, func() {
		router := NewRouter(cfg)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	})
}

func TestNewRouter_MiddlewareOrder(t *testing.T) {
	// Track middleware execution order via response headers
	orderTracker := make([]string, 0, 4)

	cfg := RouterConfig{
		CORSMiddleware:      middleware.NewOrderTrackingCORS(&orderTracker, "cors"),
		LoggingMiddleware:   middleware.NewOrderTrackingLogging(&orderTracker, "logging"),
		RateLimitMiddleware: middleware.NewOrderTrackingRateLimit(&orderTracker, "ratelimit"),
		HealthHandler:       newMinimalHealthHandler(),
		Logger:              &stubLogger{},
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify order: CORS → Logging → RateLimit
	assert.Equal(t, []string{"cors", "logging", "ratelimit"}, orderTracker)
}

func TestNewRouter_GlobalMiddleware_Applied(t *testing.T) {
	cfg := RouterConfig{
		LoggingMiddleware: middleware.NewHeaderSettingLogging("X-Logging", "applied"),
		HealthHandler:     newMinimalHealthHandler(),
		MoleculeHandler:   handlers.NewStubMoleculeHandler(),
		Logger:            &stubLogger{},
	}
	router := NewRouter(cfg)

	// Health endpoint
	req1 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	assert.Equal(t, "applied", rec1.Header().Get("X-Logging"))

	// API endpoint
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/molecules", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	assert.Equal(t, "applied", rec2.Header().Get("X-Logging"))
}

//Personal.AI order the ending
