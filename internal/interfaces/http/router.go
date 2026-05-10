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
	"net/http/pprof"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/metrics"
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
	AuthMiddleware               *middleware.AuthMiddleware
	CORSMiddleware               *middleware.CORSMiddleware
	LoggingMiddleware            *middleware.LoggingMiddleware
	MetricsMiddleware            *metrics.MetricsMiddleware
	RateLimitMiddleware          *middleware.RateLimitMiddleware
	SecurityHeadersMiddleware    *middleware.SecurityHeadersMiddleware
	TenantMiddleware             *middleware.TenantMiddleware
	VersioningMiddleware         *middleware.VersioningMiddleware
	CompressionMiddleware        *middleware.CompressionMiddleware

	// Handlers
	VersionHandler    *handlers.VersionHandler
	DocsHandler       *handlers.DocsHandler
	CSPReportHandler  *handlers.CSPReportHandler

	// Infrastructure
	Logger           logging.Logger
	MetricsCollector prometheus.MetricsCollector

	// PprofEnabled enables pprof profiling endpoints at /debug/pprof/...
	// Default is false. Enable only in development/debug environments.
	PprofEnabled bool
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

	// --- Pprof profiling endpoints (only when explicitly enabled) ---
	if cfg.PprofEnabled {
		mux.Handle("GET /debug/pprof/", http.HandlerFunc(pprof.Index))
		mux.Handle("GET /debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		mux.Handle("GET /debug/pprof/trace", http.HandlerFunc(pprof.Trace))
		mux.Handle("GET /debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		mux.Handle("GET /debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	}

	// --- API version endpoint (no auth required) ---
	if cfg.VersionHandler != nil {
		cfg.VersionHandler.RegisterRoutes(mux)
	}

	// --- Documentation Routes ---
	if cfg.DocsHandler != nil {
		cfg.DocsHandler.RegisterRoutes(mux)
	}

	// --- CSP Report Endpoint (no auth required, accepts POST from browsers) ---
	if cfg.CSPReportHandler != nil {
		cfg.CSPReportHandler.RegisterRoutes(mux)
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
	// Order: Recovery -> RequestID -> Logging -> Metrics -> CORS -> SecurityHeaders -> Versioning -> RateLimit -> [Conditional: Tenant -> Auth] -> Mux

	// Build the middleware stack.
	// Since ServeMux matches strictly, we wrap the entire mux.
	// However, Auth and Tenant middlewares should typically only apply to API routes, not health/metrics.
	// We implement a conditional middleware wrapper for API routes.

	var globalMiddlewares []MiddlewareFunc

	// 1. Recovery
	globalMiddlewares = append(globalMiddlewares, recoveryMiddleware(cfg.Logger))

	// 2. RequestID
	globalMiddlewares = append(globalMiddlewares, middleware.RequestID())

	// 3. Logging
	if cfg.LoggingMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.LoggingMiddleware.Handler)
	}

	// 4. Metrics (applied to ALL requests including health and metrics endpoints)
	if cfg.MetricsMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.MetricsMiddleware.Handler)
	}

	// 5. CORS
	if cfg.CORSMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.CORSMiddleware.Handler)
	}

	// 6. SecurityHeaders (applied to ALL responses)
	if cfg.SecurityHeadersMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.SecurityHeadersMiddleware.Handler)
	}

	// 7. Compression (compress responses before versioning headers propagation)
	if cfg.CompressionMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.CompressionMiddleware.Handler)
	}

	// 7. Versioning (X-API-Version header, Accept-Version negotiation, deprecation warnings)
	if cfg.VersioningMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.VersioningMiddleware.Handler)
	}

	// 8. RateLimit
	if cfg.RateLimitMiddleware != nil {
		globalMiddlewares = append(globalMiddlewares, cfg.RateLimitMiddleware.Handler)
	}

	// 9. Tenant & Auth (Conditional)
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
