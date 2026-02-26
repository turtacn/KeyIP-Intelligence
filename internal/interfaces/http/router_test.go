// ---
//
// 继续输出 283 `internal/interfaces/http/router_test.go` 要实现 HTTP 路由注册单元测试。
//
// 实现要求:
//
// * **功能定位**：验证 NewRouter 构建的路由树完整性、健康检查端点可用性
// * **测试用例**：
//   * `TestNewRouter_HealthEndpoints_NoAuth`：健康检查端点不经过认证中间件
//   * `TestNewRouter_NilHandlers_NoPanic`：Handler 为 nil 时不 panic
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
//
// ---
package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
)

// stubLogger implements logging.Logger for testing.
type stubLogger struct{}

func (s *stubLogger) Debug(msg string, fields ...logging.Field) {}
func (s *stubLogger) Info(msg string, fields ...logging.Field)  {}
func (s *stubLogger) Warn(msg string, fields ...logging.Field)  {}
func (s *stubLogger) Error(msg string, fields ...logging.Field) {}
func (s *stubLogger) Fatal(msg string, fields ...logging.Field) {}
func (s *stubLogger) With(fields ...logging.Field) logging.Logger { return s }
func (s *stubLogger) WithContext(ctx context.Context) logging.Logger { return s }
func (s *stubLogger) WithError(err error) logging.Logger { return s }
func (s *stubLogger) Sync() error { return nil }

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

func TestNewRouter_HealthEndpoints_NoAuth(t *testing.T) {
	cfg := RouterConfig{
		HealthHandler: newMinimalHealthHandler(),
		Logger:        &stubLogger{},
	}
	router := NewRouter(cfg)

	// /healthz should respond without auth header
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
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

func TestNewRouter_NilHandlers_NoPanic(t *testing.T) {
	// Should not panic even when handlers are nil
	cfg := RouterConfig{
		HealthHandler: newMinimalHealthHandler(),
		Logger:        &stubLogger{},
	}
	
	assert.NotPanics(t, func() {
		router := NewRouter(cfg)
		assert.NotNil(t, router)
	})
}

//Personal.AI order the ending
