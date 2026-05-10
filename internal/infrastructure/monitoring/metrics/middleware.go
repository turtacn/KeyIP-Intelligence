// Phase 11 - 基础设施层: HTTP 指标中间件
// 序号: 293
// 文件: internal/infrastructure/monitoring/metrics/middleware.go
// 功能定位: 实现 HTTP 请求的 OpenTelemetry 指标采集中间件，记录请求耗时、
//           按状态码分类的请求计数、以及活跃请求数
// 核心实现:
//   - 定义 MetricsMiddleware 结构体，遵循现有中间件包装模式
//   - 实现 Metrics() func(http.Handler) http.Handler 中间件函数
//   - 在请求开始前增加活跃请求计数
//   - 在请求结束后记录持续时间和请求计数
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, internal/infrastructure/monitoring/metrics
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package metrics

import (
	"net/http"
	"time"
)

// wrappedResponseWriter captures the HTTP status code for metrics recording.
type wrappedResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

// newWrappedResponseWriter creates a new wrappedResponseWriter.
func newWrappedResponseWriter(w http.ResponseWriter) *wrappedResponseWriter {
	return &wrappedResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default if WriteHeader is never called
	}
}

// WriteHeader captures the status code before delegating.
func (w *wrappedResponseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.statusCode = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write ensures the default status code is set if WriteHeader was never called.
func (w *wrappedResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Metrics returns HTTP middleware that records OpenTelemetry metrics for each
// incoming HTTP request: request duration histogram, request count by method/
// path/status, and active requests gauge.
func Metrics(metrics *HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Increment active requests before processing.
			metrics.AddActiveRequests(r.Context(), r.Method, r.URL.Path, 1)

			start := time.Now()

			// Wrap response writer to capture the status code.
			wrapped := newWrappedResponseWriter(w)

			// Serve the request.
			next.ServeHTTP(wrapped, r)

			// Record request duration and count.
			duration := time.Since(start)
			metrics.RecordRequest(r.Context(), r.Method, r.URL.Path, wrapped.statusCode, duration)

			// Decrement active requests after processing.
			metrics.AddActiveRequests(r.Context(), r.Method, r.URL.Path, -1)
		})
	}
}

// MetricsMiddleware wraps the metrics middleware for use with router
// configuration. It follows the same struct pattern as other middlewares
// in the project.
type MetricsMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewMetricsMiddleware creates a new MetricsMiddleware.
func NewMetricsMiddleware(httpMetrics *HTTPMetrics) *MetricsMiddleware {
	return &MetricsMiddleware{
		handler: Metrics(httpMetrics),
	}
}

// Handler returns the middleware handler function.
func (m *MetricsMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

// Personal.AI order the ending
