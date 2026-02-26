// Phase 11 - File 261: internal/interfaces/http/handlers/health_handler_test.go
// 实现健康检查 HTTP Handler 单元测试。
//
// 实现要求:
// * 测试用例：
//   - TestLiveness_AlwaysOK
//   - TestReadiness_AllHealthy
//   - TestReadiness_OneUnhealthy
//   - TestReadiness_NoCheckers
//   - TestDetailed_AllHealthy
//   - TestDetailed_Degraded
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Mock HealthChecker ---

type mockHealthChecker struct {
	name string
	err  error
}

func (m *mockHealthChecker) Name() string                    { return m.name }
func (m *mockHealthChecker) Check(_ context.Context) error   { return m.err }

func TestLiveness_AlwaysOK(t *testing.T) {
	h := NewHealthHandler("v1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.Liveness(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp LivenessResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	assert.Equal(t, "alive", resp.Status)
	assert.Equal(t, "v1.0.0", resp.Version)
	assert.NotEmpty(t, resp.Uptime)
}

func TestReadiness_AllHealthy(t *testing.T) {
	checkers := []HealthChecker{
		&mockHealthChecker{name: "postgres", err: nil},
		&mockHealthChecker{name: "redis", err: nil},
	}
	h := NewHealthHandler("v1.0.0", checkers...)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readiness(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ReadinessResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	assert.Equal(t, "ready", resp.Status)
	assert.Equal(t, "healthy", resp.Components["postgres"].Status)
	assert.Equal(t, "healthy", resp.Components["redis"].Status)
}

func TestReadiness_OneUnhealthy(t *testing.T) {
	checkers := []HealthChecker{
		&mockHealthChecker{name: "postgres", err: nil},
		&mockHealthChecker{name: "redis", err: fmt.Errorf("connection refused")},
	}
	h := NewHealthHandler("v1.0.0", checkers...)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readiness(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)

	var resp ReadinessResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	assert.Equal(t, "not_ready", resp.Status)
	assert.Equal(t, "unhealthy", resp.Components["redis"].Status)
	assert.Contains(t, resp.Components["redis"].Error, "connection refused")
}

func TestReadiness_NoCheckers(t *testing.T) {
	h := NewHealthHandler("v1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.Readiness(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp ReadinessResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	assert.Equal(t, "ready", resp.Status)
}

func TestDetailed_AllHealthy(t *testing.T) {
	checkers := []HealthChecker{
		&mockHealthChecker{name: "postgres", err: nil},
		&mockHealthChecker{name: "elasticsearch", err: nil},
	}
	h := NewHealthHandler("v1.0.0", checkers...)

	req := httptest.NewRequest(http.MethodGet, "/healthz/detail", nil)
	rec := httptest.NewRecorder()

	h.Detailed(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDetailed_Degraded(t *testing.T) {
	checkers := []HealthChecker{
		&mockHealthChecker{name: "postgres", err: nil},
		&mockHealthChecker{name: "elasticsearch", err: fmt.Errorf("cluster red")},
	}
	h := NewHealthHandler("v1.0.0", checkers...)

	req := httptest.NewRequest(http.MethodGet, "/healthz/detail", nil)
	rec := httptest.NewRecorder()

	h.Detailed(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

//Personal.AI order the ending
