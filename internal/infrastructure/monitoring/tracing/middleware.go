// Phase 11 - 基础设施层: HTTP 追踪中间件
// 序号: 289
// 文件: internal/infrastructure/monitoring/tracing/middleware.go
// 功能定位: 实现 HTTP 请求的 OpenTelemetry 追踪中间件，为每个请求创建追踪跨度，
//           记录请求方法、路径、状态码等关键属性，并支持 W3C TraceContext 传播
// 核心实现:
//   - 定义 wrappedResponseWriter 包装器，捕获状态码
//   - 实现 Tracing() func(http.Handler) http.Handler 中间件函数
//   - 实现 TracingMiddleware 结构体，遵循现有中间件包装模式
//   - 提取 W3C TraceContext 请求头，维持分布式追踪链路
//   - 请求属性: http.method, http.path, http.status_code, http.host, http.user_agent
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/trace,
//           go.opentelemetry.io/otel/propagation, go.opentelemetry.io/otel/attribute
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package tracing

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// wrappedResponseWriter captures the HTTP status code for span attributes.
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

// Tracing returns HTTP middleware that creates an OpenTelemetry span for each
// incoming HTTP request. It extracts W3C TraceContext from request headers to
// maintain distributed trace continuity, and records request method, path,
// host, scheme, user-agent, and response status code as span attributes.
func Tracing() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract propagation context from incoming W3C TraceContext headers.
			ctx := otel.GetTextMapPropagator().Extract(
				r.Context(),
				propagation.HeaderCarrier(r.Header),
			)

			// Determine span name using method + path.
			spanName := r.Method + " " + r.URL.Path

			// Start a server-side span.
			tracer := otel.Tracer("http.server")
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			// Set request attributes on the span.
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
				attribute.String("http.host", r.Host),
				attribute.String("http.scheme", r.URL.Scheme),
			)
			if ua := r.UserAgent(); ua != "" {
				span.SetAttributes(attribute.String("http.user_agent", ua))
			}

			// Wrap response writer to capture the status code.
			wrapped := newWrappedResponseWriter(w)

			// Serve the request with the trace context injected into the request context.
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Record response status code on the span.
			span.SetAttributes(
				attribute.Int("http.status_code", wrapped.statusCode),
			)
		})
	}
}

// TracingMiddleware wraps tracing middleware for use with router configuration.
// It follows the same struct pattern as other middlewares in the project.
type TracingMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewTracingMiddleware creates a new TracingMiddleware.
func NewTracingMiddleware() *TracingMiddleware {
	return &TracingMiddleware{
		handler: Tracing(),
	}
}

// Handler returns the middleware handler function.
func (m *TracingMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

//Personal.AI order the ending
