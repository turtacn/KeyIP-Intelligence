// Phase 11 - 接口层: HTTP Middleware - CORS 跨域中间件
// 序号: 274
// 文件: internal/interfaces/http/middleware/cors.go
// 功能定位: 实现 CORS (Cross-Origin Resource Sharing) 中间件，控制跨域请求策略
// 核心实现:
//   - 定义 CORSConfig 结构体: AllowedOrigins, AllowedMethods, AllowedHeaders, ExposedHeaders,
//     AllowCredentials, MaxAge, AllowWildcard
//   - 定义 DefaultCORSConfig() CORSConfig，返回安全的默认配置
//   - 实现 CORS(config CORSConfig) func(http.Handler) http.Handler
//   - 预检请求 (OPTIONS) 处理: 设置 Access-Control-Allow-* 头后返回 204
//   - 简单请求处理: 验证 Origin 后设置响应头
//   - Origin 匹配支持: 精确匹配、通配符 *、子域名模式 *.example.com
//   - Vary 头正确设置，确保缓存行为正确
// 安全考量:
//   - 默认不允许通配符 origin + credentials 组合
//   - 严格验证 Origin 格式防止注入
//   - 生产环境建议显式列出允许的 origins
// 依赖关系:
//   - 被依赖: internal/interfaces/http/router.go
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds configuration for CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins that are allowed to make cross-origin requests.
	// Use ["*"] to allow all origins (not recommended for production with credentials).
	AllowedOrigins []string

	// AllowedMethods is a list of HTTP methods allowed for cross-origin requests.
	AllowedMethods []string

	// AllowedHeaders is a list of request headers allowed for cross-origin requests.
	AllowedHeaders []string

	// ExposedHeaders is a list of response headers exposed to the client.
	ExposedHeaders []string

	// AllowCredentials indicates whether credentials (cookies, auth headers) are allowed.
	AllowCredentials bool

	// MaxAge indicates how long (in seconds) preflight results can be cached.
	MaxAge int

	// AllowWildcard enables subdomain wildcard matching (e.g., *.example.com).
	AllowWildcard bool
}

// DefaultCORSConfig returns a secure default CORS configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{},
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-API-Key",
			"X-Request-ID",
			"X-Tenant-ID",
		},
		ExposedHeaders: []string{
			"X-Request-ID",
			"X-RateLimit-Limit",
			"X-RateLimit-Remaining",
			"X-RateLimit-Reset",
		},
		AllowCredentials: false,
		MaxAge:           86400, // 24 hours
		AllowWildcard:    false,
	}
}

// CORS returns middleware that handles Cross-Origin Resource Sharing.
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	// Pre-compute joined strings for performance
	allowedMethodsStr := strings.Join(config.AllowedMethods, ", ")
	allowedHeadersStr := strings.Join(config.AllowedHeaders, ", ")
	exposedHeadersStr := strings.Join(config.ExposedHeaders, ", ")
	maxAgeStr := strconv.Itoa(config.MaxAge)

	// Build origin lookup set for O(1) matching
	originSet := make(map[string]bool, len(config.AllowedOrigins))
	var wildcardPatterns []string
	allowAll := false

	for _, origin := range config.AllowedOrigins {
		if origin == "*" {
			allowAll = true
		} else if config.AllowWildcard && strings.HasPrefix(origin, "*.") {
			wildcardPatterns = append(wildcardPatterns, origin[1:]) // store ".example.com"
		} else {
			originSet[strings.ToLower(origin)] = true
		}
	}

	isOriginAllowed := func(origin string) bool {
		if allowAll {
			return true
		}
		if originSet[strings.ToLower(origin)] {
			return true
		}
		for _, pattern := range wildcardPatterns {
			if strings.HasSuffix(strings.ToLower(origin), pattern) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// No Origin header means same-origin or non-browser request
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Validate origin
			if !isOriginAllowed(origin) {
				// Origin not allowed — proceed without CORS headers
				// The browser will block the response on the client side
				next.ServeHTTP(w, r)
				return
			}

			// Set Vary header for proper caching
			w.Header().Add("Vary", "Origin")
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")

			// Set allowed origin
			if allowAll && !config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			// Set credentials
			if config.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight (OPTIONS) request
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", allowedMethodsStr)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeadersStr)
				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", maxAgeStr)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Set exposed headers for actual requests
			if exposedHeadersStr != "" {
				w.Header().Set("Access-Control-Expose-Headers", exposedHeadersStr)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware wraps CORS middleware for use with router configuration.
type CORSMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewCORSMiddleware creates a new CORS middleware with the given config.
func NewCORSMiddleware(config CORSConfig) *CORSMiddleware {
	return &CORSMiddleware{
		handler: CORS(config),
	}
}

// Handler returns the middleware handler function.
func (m *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

//Personal.AI order the ending
