// Phase 11 - 接口层: HTTP Middleware - 认证中间件
// 序号: 272
// 文件: internal/interfaces/http/middleware/auth.go
// 功能定位: 实现 HTTP 请求认证中间件，支持 JWT Bearer Token 和 API Key 两种认证方式
// 核心实现:
//   - 定义 AuthMiddleware 结构体，注入 TokenValidator、APIKeyValidator、Logger
//   - 定义 TokenValidator 接口: ValidateToken(token string) (*Claims, error)
//   - 定义 APIKeyValidator 接口: ValidateAPIKey(key string) (*APIKeyInfo, error)
//   - 定义 Claims 结构体: UserID, TenantID, Roles, ExpiresAt, IssuedAt
//   - 定义 APIKeyInfo 结构体: KeyID, TenantID, Scopes, RateLimit
//   - 实现 Authenticate() func(http.Handler) http.Handler，核心认证逻辑:
//     1. 从 Authorization header 提取 Bearer token 或 X-API-Key header 提取 API key
//     2. 优先尝试 Bearer token 认证，失败则尝试 API key
//     3. 认证成功后将身份信息注入 context
//     4. 认证失败返回 401 Unauthorized
//   - 实现 OptionalAuth() 中间件: 认证可选，未提供凭证时以匿名身份继续
//   - 实现 contextKey 类型和 ContextGetClaims/ContextGetAPIKeyInfo 辅助函数
//   - 支持路径白名单配置，跳过特定路径的认证（如 /health, /metrics）
// 安全考量:
//   - Token 过期自动拒绝
//   - 不在错误响应中泄露认证细节
//   - 支持 token 黑名单检查
// 依赖关系:
//   - 依赖: internal/infrastructure/monitoring/logging
//   - 被依赖: internal/interfaces/http/router.go, middleware chain
// 测试要求: Bearer token 正常/过期/格式错误、API key 正常/无效、白名单跳过、context 注入
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// contextKey is an unexported type for context keys to prevent collisions.
type contextKey int

const (
	// claimsContextKey is the context key for JWT claims.
	claimsContextKey contextKey = iota
	// apiKeyInfoContextKey is the context key for API key info.
	apiKeyInfoContextKey
)

// Claims represents the decoded JWT token claims.
type Claims struct {
	UserID    string    `json:"user_id"`
	TenantID  string    `json:"tenant_id"`
	Roles     []string  `json:"roles"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
}

// APIKeyInfo represents validated API key information.
type APIKeyInfo struct {
	KeyID     string   `json:"key_id"`
	TenantID  string   `json:"tenant_id"`
	Scopes    []string `json:"scopes"`
	RateLimit int      `json:"rate_limit"`
}

// TokenValidator validates JWT bearer tokens.
type TokenValidator interface {
	ValidateToken(token string) (*Claims, error)
}

// APIKeyValidator validates API keys.
type APIKeyValidator interface {
	ValidateAPIKey(key string) (*APIKeyInfo, error)
}

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	// SkipPaths are paths that bypass authentication entirely.
	SkipPaths []string
	// AllowExpiredGracePeriod allows tokens expired within this duration.
	AllowExpiredGracePeriod time.Duration
}

// AuthMiddleware provides HTTP authentication middleware.
type AuthMiddleware struct {
	tokenValidator  TokenValidator
	apiKeyValidator APIKeyValidator
	config          AuthConfig
	logger          logging.Logger
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(
	tokenValidator TokenValidator,
	apiKeyValidator APIKeyValidator,
	config AuthConfig,
	logger logging.Logger,
) *AuthMiddleware {
	return &AuthMiddleware{
		tokenValidator:  tokenValidator,
		apiKeyValidator: apiKeyValidator,
		config:          config,
		logger:          logger,
	}
}

// Authenticate returns middleware that enforces authentication.
// Requests without valid credentials receive 401 Unauthorized.
func (m *AuthMiddleware) Authenticate() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check skip paths
			if m.shouldSkip(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Try Bearer token first
			if token := extractBearerToken(r); token != "" {
				claims, err := m.tokenValidator.ValidateToken(token)
				if err != nil {
					m.logger.Error("token validation failed", "error", err, "path", r.URL.Path)
					writeUnauthorized(w, "invalid or expired token")
					return
				}

				// Check expiration
				if time.Now().After(claims.ExpiresAt.Add(m.config.AllowExpiredGracePeriod)) {
					writeUnauthorized(w, "token expired")
					return
				}

				ctx := context.WithValue(r.Context(), claimsContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try API key
			if apiKey := extractAPIKey(r); apiKey != "" {
				info, err := m.apiKeyValidator.ValidateAPIKey(apiKey)
				if err != nil {
					m.logger.Error("API key validation failed", "error", err, "path", r.URL.Path)
					writeUnauthorized(w, "invalid API key")
					return
				}

				ctx := context.WithValue(r.Context(), apiKeyInfoContextKey, info)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No credentials provided
			writeUnauthorized(w, "authentication required")
		})
	}
}

// OptionalAuth returns middleware that attempts authentication but allows anonymous access.
func (m *AuthMiddleware) OptionalAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Bearer token
			if token := extractBearerToken(r); token != "" {
				claims, err := m.tokenValidator.ValidateToken(token)
				if err == nil && time.Now().Before(claims.ExpiresAt.Add(m.config.AllowExpiredGracePeriod)) {
					ctx := context.WithValue(r.Context(), claimsContextKey, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Try API key
			if apiKey := extractAPIKey(r); apiKey != "" {
				info, err := m.apiKeyValidator.ValidateAPIKey(apiKey)
				if err == nil {
					ctx := context.WithValue(r.Context(), apiKeyInfoContextKey, info)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Continue as anonymous
			next.ServeHTTP(w, r)
		})
	}
}

// shouldSkip checks if the given path should bypass authentication.
func (m *AuthMiddleware) shouldSkip(path string) bool {
	for _, skip := range m.config.SkipPaths {
		if path == skip || strings.HasPrefix(path, skip+"/") {
			return true
		}
	}
	return false
}

// extractBearerToken extracts the Bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// extractAPIKey extracts the API key from the X-API-Key header.
func extractAPIKey(r *http.Request) string {
	key := r.Header.Get("X-API-Key")
	if key != "" {
		return strings.TrimSpace(key)
	}
	// Fallback: check query parameter (less secure, for webhook callbacks)
	return r.URL.Query().Get("api_key")
}

// ContextGetClaims retrieves JWT claims from the request context.
// Returns nil if no claims are present (anonymous or API key auth).
func ContextGetClaims(ctx context.Context) *Claims {
	claims, ok := ctx.Value(claimsContextKey).(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// ContextGetAPIKeyInfo retrieves API key info from the request context.
// Returns nil if no API key info is present (anonymous or JWT auth).
func ContextGetAPIKeyInfo(ctx context.Context) *APIKeyInfo {
	info, ok := ctx.Value(apiKeyInfoContextKey).(*APIKeyInfo)
	if !ok {
		return nil
	}
	return info
}

// ContextGetTenantID extracts the tenant ID from either JWT claims or API key info.
// Returns empty string if no authentication context is present.
func ContextGetTenantID(ctx context.Context) string {
	if claims := ContextGetClaims(ctx); claims != nil {
		return claims.TenantID
	}
	if info := ContextGetAPIKeyInfo(ctx); info != nil {
		return info.TenantID
	}
	return ""
}

// ContextGetUserID extracts the user ID from JWT claims.
// Returns empty string if not authenticated via JWT.
func ContextGetUserID(ctx context.Context) string {
	if claims := ContextGetClaims(ctx); claims != nil {
		return claims.UserID
	}
	return ""
}

// IsAuthenticated checks whether the request context contains valid authentication.
func IsAuthenticated(ctx context.Context) bool {
	return ContextGetClaims(ctx) != nil || ContextGetAPIKeyInfo(ctx) != nil
}

// writeUnauthorized writes a 401 Unauthorized JSON response.
// Intentionally vague to avoid leaking authentication details.
func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="keyip"`)
	w.WriteHeader(http.StatusUnauthorized)
	// Use generic message in production to avoid information leakage
	w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"` + message + `"}}`))
}

//Personal.AI order the ending

