// Phase 11 - File 260: internal/interfaces/http/handlers/health_handler.go
// 实现健康检查 HTTP Handler。
//
// 实现要求:
// * 功能定位：提供系统健康检查端点，包括 liveness 和 readiness 探针
// * 核心实现：
//   - 定义 HealthHandler 结构体，注入各基础设施组件的健康检查器
//   - 实现 Liveness：返回服务存活状态（始终 200，除非进程异常）
//   - 实现 Readiness：检查所有依赖组件（DB、Redis、ES、MQ）的连通性
//   - 实现 Detailed：返回各组件详细健康状态（仅内部访问）
//   - 实现 RegisterRoutes：注册健康检查路由
// * 业务逻辑：
//   - Liveness 不检查外部依赖，仅确认进程存活
//   - Readiness 检查所有关键依赖，任一失败返回 503
//   - Detailed 返回每个组件的延迟和状态
// * 依赖关系：
//   - 被依赖：internal/interfaces/http/router.go、Kubernetes 探针配置
// * 测试要求：全部探针正常与异常路径
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// HealthChecker is an interface for components that can report their health.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) error
}

// HealthHandler handles health check HTTP requests.
type HealthHandler struct {
	checkers []HealthChecker
	version  string
	startAt  time.Time
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(version string, checkers ...HealthChecker) *HealthHandler {
	return &HealthHandler{
		checkers: checkers,
		version:  version,
		startAt:  time.Now(),
	}
}

// RegisterRoutes registers health check routes.
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.Liveness)
	mux.HandleFunc("GET /readyz", h.Readiness)
	mux.HandleFunc("GET /healthz/detail", h.Detailed)
}

// LivenessResponse is the response for liveness probe.
type LivenessResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

// ReadinessResponse is the response for readiness probe.
type ReadinessResponse struct {
	Status     string                    `json:"status"`
	Components map[string]ComponentCheck `json:"components,omitempty"`
}

// ComponentCheck represents the health status of a single component.
type ComponentCheck struct {
	Status   string `json:"status"`
	Latency  string `json:"latency,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Liveness handles GET /healthz - Kubernetes liveness probe.
// Always returns 200 if the process is running.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	resp := LivenessResponse{
		Status:  "alive",
		Version: h.version,
		Uptime:  time.Since(h.startAt).Truncate(time.Second).String(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// Readiness handles GET /readyz - Kubernetes readiness probe.
// Returns 200 if all dependencies are healthy, 503 otherwise.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if len(h.checkers) == 0 {
		writeJSON(w, http.StatusOK, ReadinessResponse{Status: "ready"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	components := h.checkAll(ctx)

	allHealthy := true
	for _, c := range components {
		if c.Status != "healthy" {
			allHealthy = false
			break
		}
	}

	resp := ReadinessResponse{
		Components: components,
	}

	if allHealthy {
		resp.Status = "ready"
		writeJSON(w, http.StatusOK, resp)
	} else {
		resp.Status = "not_ready"
		writeJSON(w, http.StatusServiceUnavailable, resp)
	}
}

// Detailed handles GET /healthz/detail - detailed health status.
func (h *HealthHandler) Detailed(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	components := h.checkAll(ctx)

	allHealthy := true
	for _, c := range components {
		if c.Status != "healthy" {
			allHealthy = false
			break
		}
	}

	type DetailedResponse struct {
		Status     string                    `json:"status"`
		Version    string                    `json:"version"`
		Uptime     string                    `json:"uptime"`
		Components map[string]ComponentCheck `json:"components"`
	}

	status := "healthy"
	if !allHealthy {
		status = "degraded"
	}

	resp := DetailedResponse{
		Status:     status,
		Version:    h.version,
		Uptime:     time.Since(h.startAt).Truncate(time.Second).String(),
		Components: components,
	}

	code := http.StatusOK
	if !allHealthy {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, resp)
}

// checkAll runs all health checkers concurrently and returns results.
func (h *HealthHandler) checkAll(ctx context.Context) map[string]ComponentCheck {
	results := make(map[string]ComponentCheck, len(h.checkers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, checker := range h.checkers {
		wg.Add(1)
		go func(c HealthChecker) {
			defer wg.Done()

			start := time.Now()
			err := c.Check(ctx)
			latency := time.Since(start)

			cc := ComponentCheck{
				Status:  "healthy",
				Latency: latency.Truncate(time.Microsecond).String(),
			}
			if err != nil {
				cc.Status = "unhealthy"
				cc.Error = err.Error()
			}

			mu.Lock()
			results[c.Name()] = cc
			mu.Unlock()
		}(checker)
	}

	wg.Wait()
	return results
}

// --- Common Helper Functions (Moved from common.go to avoid extra file) ---

// getUserIDFromContext extracts user ID from request context (set by auth middleware).
func getUserIDFromContext(r *http.Request) string {
	return middleware.ContextGetUserID(r.Context())
}

// parsePagination extracts page and page_size from query parameters.
func parsePagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 20

	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}
	return page, pageSize
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError writes a structured error response.
func writeError(w http.ResponseWriter, statusCode int, err error) {
	resp := ErrorResponse{
		Code:    http.StatusText(statusCode),
		Message: err.Error(),
	}
	writeJSON(w, statusCode, resp)
}

// writeAppError maps application-level errors to HTTP status codes.
func writeAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.IsNotFound(err):
		writeError(w, http.StatusNotFound, err)
	case errors.IsValidation(err):
		writeError(w, http.StatusBadRequest, err)
	case errors.IsConflict(err):
		writeError(w, http.StatusConflict, err)
	case errors.IsUnauthorized(err):
		writeError(w, http.StatusUnauthorized, err)
	case errors.IsForbidden(err):
		writeError(w, http.StatusForbidden, err)
	default:
		// Mask internal errors
		writeError(w, http.StatusInternalServerError, errors.New(errors.ErrCodeInternal, "internal server error"))
	}
}

//Personal.AI order the ending
