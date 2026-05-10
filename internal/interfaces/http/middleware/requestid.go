// Phase 11 - 接口层: HTTP Middleware - 请求追踪 ID 中间件
// 序号: 275a
// 文件: internal/interfaces/http/middleware/requestid.go
// 功能定位: 为每个 HTTP 请求分配唯一追踪 ID，支持 X-Request-ID 请求头透传，
//           并将 requestID 注入 context 方便下游日志使用。
// 核心实现:
//   - RequestID() 中间件: 检查 X-Request-ID 请求头，存在则复用，否则生成 UUID v4
//   - 设置 X-Request-ID 响应头
//   - 将 requestID 注入 context（通过 logging.WithRequestID）
//   - ContextGetRequestID(ctx) 辅助函数供下游组件提取
//
// 依赖关系:
//   - 依赖: github.com/google/uuid, github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// RequestID returns middleware that ensures every request has a unique request ID.
// It checks the X-Request-ID request header first; if absent, it generates a UUID v4.
// The request ID is set on the X-Request-ID response header and injected into the
// request context for downstream logging and tracing.
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				reqID = uuid.New().String()
			}
			w.Header().Set("X-Request-ID", reqID)
			ctx := logging.WithRequestID(r.Context(), reqID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ContextGetRequestID retrieves the request ID from the context.
// Returns an empty string if no request ID is present.
func ContextGetRequestID(ctx context.Context) string {
	return logging.RequestIDFromContext(ctx)
}

// RequestIDMiddleware wraps the request ID middleware for use with router configuration.
type RequestIDMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewRequestIDMiddleware creates a new request ID middleware.
func NewRequestIDMiddleware() *RequestIDMiddleware {
	return &RequestIDMiddleware{
		handler: RequestID(),
	}
}

// Handler returns the middleware handler function.
func (m *RequestIDMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

//Personal.AI order the ending
