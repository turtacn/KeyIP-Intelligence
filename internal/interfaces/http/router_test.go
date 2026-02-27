// Phase 11 - File 283: internal/interfaces/http/router_test.go
// 路由注册与中间件集成测试。
//
// 核心测试点：
// 1. 公共端点（/healthz）可访问且不受 Auth 中间件影响（通过 Mock 验证）。
// 2. API 端点（/api/v1/molecules）受全局和条件中间件（Auth/Tenant）保护。
// 3. 中间件链式调用顺序正确。
//
// 强制约束：文件最后一行必须为 //Personal.AI order the ending

package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
)

// stubLogger implements logging.Logger for testing.
type stubLogger struct{}

func (s *stubLogger) Debug(msg string, fields ...logging.Field) {}
func (s *stubLogger) Info(msg string, fields ...logging.Field)  {}
func (s *stubLogger) Warn(msg string, fields ...logging.Field)  {}
func (s *stubLogger) Error(msg string, fields ...logging.Field) {}
func (s *stubLogger) Fatal(msg string, fields ...logging.Field) {}
func (s *stubLogger) With(fields ...logging.Field) logging.Logger    { return s }
func (s *stubLogger) WithContext(ctx context.Context) logging.Logger { return s }
func (s *stubLogger) WithError(err error) logging.Logger             { return s }
func (s *stubLogger) Sync() error                                    { return nil }

// stubHealthChecker implements handlers.HealthChecker for testing.
type stubHealthChecker struct {
	name string
}

func (s *stubHealthChecker) Name() string { return s.name }
func (s *stubHealthChecker) Check(ctx context.Context) error { return nil }

// newMinimalHealthHandler creates a HealthHandler with stub dependencies.
func newMinimalHealthHandler() *handlers.HealthHandler {
	checker := &stubHealthChecker{name: "stub"}
	return handlers.NewHealthHandler("test", checker)
}

// Mock TokenValidator
type mockTokenValidator struct{}

func (m *mockTokenValidator) ValidateToken(token string) (*middleware.Claims, error) {
	return &middleware.Claims{UserID: "user1", TenantID: "tenant1"}, nil
}

// Mock APIKeyValidator
type mockAPIKeyValidator struct{}

func (m *mockAPIKeyValidator) ValidateAPIKey(key string) (*middleware.APIKeyInfo, error) {
	return &middleware.APIKeyInfo{KeyID: "key1", TenantID: "tenant1"}, nil
}

func TestNewRouter_HealthEndpoints_NoAuth(t *testing.T) {
	// Setup RouterConfig with just HealthHandler and a Logger
	cfg := RouterConfig{
		HealthHandler: newMinimalHealthHandler(),
		Logger:        &stubLogger{},
		// Note: No AuthMiddleware provided in this test config,
		// but even if it were, we expect public endpoints to be accessible.
		// However, with our new conditional middleware logic, Auth is ONLY applied to /api/
		// so /healthz is naturally skipped.
	}
	router := NewRouter(cfg)

	// /healthz should respond without auth header
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestNewRouter_APIEndpoints_WithAuthMiddleware(t *testing.T) {
	// Setup AuthMiddleware
	authConfig := middleware.AuthConfig{}
	authMw := middleware.NewAuthMiddleware(&mockTokenValidator{}, &mockAPIKeyValidator{}, authConfig, &stubLogger{})

	// Setup a dummy handler for API route (we need at least one to register the route)
	// Since we don't want to mock full services for handlers, we'll rely on the fact that
	// 404 means the router worked but no handler matched, OR we can mock a handler.
	// But `NewRouter` takes struct pointers. Let's just create a dummy handler or use nil and expect 404
	// BUT Auth middleware should run BEFORE 404 if the path matches prefix.
	// Actually, if we don't register any API routes, the router might not even match /api/...
	// Let's create a partial RouterConfig.

	cfg := RouterConfig{
		Logger:         &stubLogger{},
		AuthMiddleware: authMw,
	}
	router := NewRouter(cfg)

	// Request to /api/v1/something
	// Should fail with 401 Unauthorized because no token provided
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Since we wrap AuthMiddleware with conditionalMiddleware("/api/"), it should execute.
	// AuthMiddleware returns 401 if no credentials.
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestNewRouter_GlobalMiddleware_RequestID(t *testing.T) {
	cfg := RouterConfig{
		HealthHandler: newMinimalHealthHandler(),
		Logger:        &stubLogger{},
	}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Check X-Request-ID header
	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func TestNewRouter_ConditionalMiddleware_Logic(t *testing.T) {
	// Verify that conditional middleware applies only to matching prefix
	executed := false
	testMw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executed = true
			next.ServeHTTP(w, r)
		})
	}

	conditional := conditionalMiddleware("/api/", testMw)
	
	// Case 1: Match
	handler := conditional(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/api/v1/resource", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.True(t, executed, "Middleware should execute for matching path")

	// Case 2: No Match
	executed = false
	req = httptest.NewRequest("GET", "/healthz", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	assert.False(t, executed, "Middleware should NOT execute for non-matching path")
}

func TestChain(t *testing.T) {
	order := []string{}

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1 start")
			next.ServeHTTP(w, r)
			order = append(order, "mw1 end")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2 start")
			next.ServeHTTP(w, r)
			order = append(order, "mw2 end")
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	chained := Chain(finalHandler, mw1, mw2)
	chained.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	expected := []string{
		"mw1 start",
		"mw2 start",
		"handler",
		"mw2 end",
		"mw1 end",
	}

	assert.Equal(t, expected, order)
}

//Personal.AI order the ending
