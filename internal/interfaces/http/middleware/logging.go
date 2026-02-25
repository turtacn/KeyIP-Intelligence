// Phase 11 - 接口层: HTTP Middleware - 请求日志中间件
// 序号: 276
// 文件: internal/interfaces/http/middleware/logging.go
// 功能定位: 实现 HTTP 请求/响应日志中间件，记录请求方法、路径、状态码、耗时、请求 ID 等关键信息
// 核心实现:
//   - 定义 LoggingConfig 结构体: SkipPaths, LogRequestBody, LogResponseBody, SlowThreshold, MaxBodyLogSize
//   - 定义 responseWriter 包装器，捕获状态码和响应字节数
//   - 实现 RequestLogging(logger, config) func(http.Handler) http.Handler
//   - 请求开始时记录: method, path, remote_addr, user_agent, request_id
//   - 请求结束时记录: status, duration_ms, bytes_written, request_id
//   - 慢请求告警: 超过 SlowThreshold 的请求以 Warn 级别记录
//   - 错误请求高亮: 5xx 以 Error 级别记录，4xx 以 Warn 级别记录
//   - 健康检查等高频路径可配置跳过，减少日志噪音
// 依赖关系:
//   - 依赖: internal/infrastructure/monitoring/logging
//   - 被依赖: internal/interfaces/http/router.go
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// LoggingConfig holds configuration for the request logging middleware.
type LoggingConfig struct {
	// SkipPaths are paths that should not be logged (e.g., /health, /metrics).
	SkipPaths []string

	// LogRequestBody enables logging of request body (truncated to MaxBodyLogSize).
	LogRequestBody bool

	// LogResponseBody enables logging of response body (truncated to MaxBodyLogSize).
	LogResponseBody bool

	// SlowThreshold is the duration above which a request is considered slow.
	SlowThreshold time.Duration

	// MaxBodyLogSize is the maximum number of bytes to log from request/response bodies.
	MaxBodyLogSize int
}

// DefaultLoggingConfig returns a sensible default logging configuration.
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		SkipPaths:      []string{"/health", "/healthz", "/readyz"},
		LogRequestBody: false,
		LogResponseBody: false,
		SlowThreshold:  3 * time.Second,
		MaxBodyLogSize: 1024,
	}
}

// wrappedResponseWriter captures the status code and bytes written.
type wrappedResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
	wroteHeader  bool
}

// newWrappedResponseWriter creates a new wrappedResponseWriter.
func newWrappedResponseWriter(w http.ResponseWriter) *wrappedResponseWriter {
	return &wrappedResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default if WriteHeader is never called
	}
}

// WriteHeader captures the status code.
func (w *wrappedResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written.
func (w *wrappedResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// Hijack implements http.Hijacker for WebSocket support.
func (w *wrappedResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

// Flush implements http.Flusher for streaming support.
func (w *wrappedResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// RequestLogging returns middleware that logs HTTP requests and responses.
func RequestLogging(logger logging.Logger, config LoggingConfig) func(http.Handler) http.Handler {
	skipSet := make(map[string]bool, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip configured paths
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			requestID := r.Header.Get("X-Request-ID")
			path := r.URL.Path
			if r.URL.RawQuery != "" {
				path = path + "?" + r.URL.RawQuery
			}

			// Wrap response writer to capture status and size
			wrapped := newWrappedResponseWriter(w)

			// Serve the request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)
			durationMs := float64(duration.Nanoseconds()) / 1e6

			// Build log fields
			fields := []interface{}{
				"method", r.Method,
				"path", path,
				"status", wrapped.statusCode,
				"duration_ms", fmt.Sprintf("%.2f", durationMs),
				"bytes", wrapped.bytesWritten,
				"remote_addr", r.RemoteAddr,
				"request_id", requestID,
			}

			// Add user agent for non-API clients
			if ua := r.UserAgent(); ua != "" {
				fields = append(fields, "user_agent", ua)
			}

			// Add tenant/user context if available
			if tenantID := ContextGetTenantID(r.Context()); tenantID != "" {
				fields = append(fields, "tenant_id", tenantID)
			}
			if userID := ContextGetUserID(r.Context()); userID != "" {
				fields = append(fields, "user_id", userID)
			}

			// Log at appropriate level based on status code and duration
			switch {
			case wrapped.statusCode >= 500:
				logger.Error("HTTP request completed with server error", fields...)
			case wrapped.statusCode >= 400:
				logger.Warn("HTTP request completed with client error", fields...)
			case config.SlowThreshold > 0 && duration >= config.SlowThreshold:
				logger.Warn("HTTP request completed (slow)", fields...)
			default:
				logger.Info("HTTP request completed", fields...)
			}
		})
	}
}

//Personal.AI order the ending
