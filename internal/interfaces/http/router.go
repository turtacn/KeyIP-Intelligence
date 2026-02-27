// Phase 11 - File 282: internal/interfaces/http/router.go
// 实现 HTTP 路由层，替换 chi 为 net/http.ServeMux。
//
// 核心实现：
//   - 使用 Go 1.22+ `net/http.ServeMux` 作为路由器。
//   - 实现中间件链式调用机制 (`Chain` helper)。
//   - 注册全局中间件（RequestID, Logging, CORS, RateLimit）。
//   - 注册公共路由（/healthz, /readyz）。
//   - 注册 API v1 路由，并应用认证（Auth）和租户（Tenant）中间件。
//   - Metrics 路由注册。
//
// 强制约束：文件最后一行必须为 //Personal.AI order the ending

package http

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
)

// RouterConfig aggregates all handler and middleware dependencies required
// to construct the complete HTTP route tree.
type RouterConfig struct {
	// Handlers
	MoleculeHandler      *handlers.MoleculeHandler
	PatentHandler        *handlers.PatentHandler
	PortfolioHandler     *handlers.PortfolioHandler
	LifecycleHandler     *handlers.LifecycleHandler
	CollaborationHandler *handlers.CollaborationHandler
	ReportHandler        *handlers.ReportHandler
	HealthHandler        *handlers.HealthHandler

	// Middleware
	AuthMiddleware      *middleware.AuthMiddleware
	CORSMiddleware      *middleware.CORSMiddleware
	LoggingMiddleware   *middleware.LoggingMiddleware
	RateLimitMiddleware *middleware.RateLimitMiddleware
	TenantMiddleware    *middleware.TenantMiddleware

	// Infrastructure
	Logger           logging.Logger
	MetricsCollector prometheus.MetricsCollector
}

// MiddlewareFunc defines the standard middleware signature.
type MiddlewareFunc func(http.Handler) http.Handler

// Chain applies middlewares to a http.Handler.
// The first middleware in the list is the outermost one (executed first).
func Chain(h http.Handler, middlewares ...MiddlewareFunc) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// requestIDMiddleware generates a unique request ID if not present.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := middleware.WithRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// recoveryMiddleware recovers from panics and logs the error.
func recoveryMiddleware(logger logging.Logger) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					if logger != nil {
						logger.Error("panic recovered", logging.Any("panic", err))
					} else {
						// Fallback if logger is nil
						// In production this should not happen given proper setup
					}
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// NewRouter constructs the complete HTTP route tree from the given configuration
// using Go 1.22 net/http.ServeMux.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	// --- Public health endpoints (no auth) ---
	if cfg.HealthHandler != nil {
		cfg.HealthHandler.RegisterRoutes(mux)
	}

	// --- Metrics endpoint (protected by internal firewall usually, here public for simplicity) ---
	if cfg.MetricsCollector != nil {
		mux.Handle("GET /metrics", cfg.MetricsCollector.Handler())
	}

	// --- API v1 Routes ---
	// Create a separate mux for API routes if we wanted to isolate them,
	// but ServeMux is flat. We register them directly.
	// Handlers' RegisterRoutes methods use full paths (e.g. "POST /api/v1/molecules").

	if cfg.MoleculeHandler != nil {
		cfg.MoleculeHandler.RegisterRoutes(mux)
	}
	if cfg.PatentHandler != nil {
		cfg.PatentHandler.RegisterRoutes(mux)
	}
	if cfg.PortfolioHandler != nil {
		cfg.PortfolioHandler.RegisterRoutes(mux)
	}
	if cfg.LifecycleHandler != nil {
		cfg.LifecycleHandler.RegisterRoutes(mux)
	}
	if cfg.CollaborationHandler != nil {
		cfg.CollaborationHandler.RegisterRoutes(mux)
	}
	if cfg.ReportHandler != nil {
		cfg.ReportHandler.RegisterRoutes(mux)
	}

	// --- Global Middleware Chain ---
	// Applied to ALL requests.
	// Order: Recovery -> RequestID -> Logging -> CORS -> RateLimit -> [Conditional: Tenant -> Auth] -> Mux

	// Build the middleware stack.
	// Since ServeMux matches strictly, we wrap the entire mux.
	// However, Auth and Tenant middlewares should typically only apply to API routes, not health/metrics.
	// We implement a conditional middleware wrapper for API routes.

	var globalMiddlewares []MiddlewareFunc

	// 1. Recovery
	globalMiddlewares = append(globalMiddlewares, recoveryMiddleware(cfg.Logger))

	// 2. RequestID
	globalMiddlewares = append(globalMiddlewares, requestIDMiddleware)

	// 3. Logging
	if cfg.LoggingMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.LoggingMiddleware.Handler)
	}

	// 4. CORS
	if cfg.CORSMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.CORSMiddleware.Handler)
	}

	// 5. RateLimit
	if cfg.RateLimitMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.RateLimitMiddleware.Handler)
	}

	// 6. Tenant & Auth (Conditional)
	// We wrap these to only apply if path starts with /api/.
	if cfg.TenantMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, conditionalMiddleware("/api/", cfg.TenantMiddleware.Handler))
	}
	if cfg.AuthMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, conditionalMiddleware("/api/", cfg.AuthMiddleware.Handler))
	}

	return Chain(mux, globalMiddlewares...)
}

// conditionalMiddleware applies the middleware only if the request path starts with prefix.
func conditionalMiddleware(prefix string, mw MiddlewareFunc) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, prefix) {
				mw(next).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

//Personal.AI order the ending
