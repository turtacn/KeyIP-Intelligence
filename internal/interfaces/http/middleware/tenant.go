// File: internal/interfaces/http/middleware/tenant.go
// Phase 11 - 序号 280
//
// 生成计划:
// 功能定位: HTTP 多租户中间件，从请求中提取租户标识并注入上下文，实现多租户数据隔离入口控制
// 核心实现:
//   - TenantConfig 配置结构体（HeaderName, QueryParam, DefaultTenantID, Required, AllowedTenants, TenantValidator）
//   - TenantValidatorFunc 函数类型
//   - TenantInfo 结构体（ID, Name, Plan, IsActive）
//   - NewTenantMiddleware: 三级提取（Header→QueryParam→Default）、格式校验、白名单、外部验证、上下文注入
//   - TenantFromContext / MustTenantFromContext 上下文读取
//   - DefaultTenantConfig 工厂函数
//   - validateTenantID 格式校验
// 业务逻辑:
//   - 默认 Header: X-Tenant-ID, QueryParam: tenant_id
//   - Required=true 时无租户返回 400; Required=false 时回退 DefaultTenantID 或跳过
//   - 白名单 O(1) 查找; 响应头回写 X-Tenant-ID
// 依赖: logging, pkg/errors
// 被依赖: router.go, handlers/*

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// tenantContextKey is the unexported key type for storing tenant info in context.
type tenantContextKey struct{}

// TenantValidatorFunc defines a function that validates a tenant ID against
// an external source (e.g., database, auth service). Returns true if valid.
type TenantValidatorFunc func(tenantID string) (bool, error)

// TenantInfo holds the resolved tenant information injected into the request context.
type TenantInfo struct {
	// ID is the unique tenant identifier extracted from the request.
	ID string `json:"id"`
	// Name is the human-readable tenant name, populated by the validator if available.
	Name string `json:"name,omitempty"`
	// Plan is the subscription plan of the tenant (e.g., "free", "pro", "enterprise").
	Plan string `json:"plan,omitempty"`
	// IsActive indicates whether the tenant account is currently active.
	IsActive bool `json:"is_active"`
}

// TenantConfig holds configuration for the tenant middleware.
type TenantConfig struct {
	// HeaderName is the HTTP header to extract the tenant ID from.
	// Default: "X-Tenant-ID".
	HeaderName string

	// QueryParam is the query parameter name used as a fallback for tenant ID extraction.
	// Default: "tenant_id".
	QueryParam string

	// DefaultTenantID is used when no tenant ID can be extracted and Required is false.
	DefaultTenantID string

	// Required, when true, causes the middleware to reject requests that lack a tenant ID
	// with HTTP 400 Bad Request.
	Required bool

	// AllowedTenants is an optional whitelist. When non-empty, only listed tenant IDs
	// are permitted. An empty slice disables whitelist checking.
	AllowedTenants []string

	// TenantValidator is an optional external validation function. When set, it is called
	// after format and whitelist checks pass.
	TenantValidator TenantValidatorFunc
}

// tenantIDPattern enforces: alphanumeric, underscore, hyphen, length 1-64.
var tenantIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// DefaultTenantConfig returns a TenantConfig with sensible defaults.
func DefaultTenantConfig() TenantConfig {
	return TenantConfig{
		HeaderName:      "X-Tenant-ID",
		QueryParam:      "tenant_id",
		DefaultTenantID: "",
		Required:        true,
		AllowedTenants:  nil,
		TenantValidator: nil,
	}
}

// NewTenantMiddleware creates an HTTP middleware that extracts and validates a tenant ID
// from incoming requests, then injects TenantInfo into the request context.
//
// Extraction order: Header (HeaderName) → Query parameter (QueryParam) → DefaultTenantID.
func NewTenantMiddleware(cfg TenantConfig, logger logging.Logger) func(http.Handler) http.Handler {
	// Normalize config defaults.
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Tenant-ID"
	}
	if cfg.QueryParam == "" {
		cfg.QueryParam = "tenant_id"
	}

	// Pre-build allowed tenants lookup map for O(1) checks.
	var allowedSet map[string]struct{}
	if len(cfg.AllowedTenants) > 0 {
		allowedSet = make(map[string]struct{}, len(cfg.AllowedTenants))
		for _, t := range cfg.AllowedTenants {
			allowedSet[strings.TrimSpace(t)] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := extractTenantID(r, cfg.HeaderName, cfg.QueryParam)

			// Fallback to default if extraction yields nothing.
			if tenantID == "" {
				tenantID = cfg.DefaultTenantID
			}

			// If still empty, decide based on Required flag.
			if tenantID == "" {
				if cfg.Required {
					logger.Warn("tenant ID missing in required mode",
						logging.String("method", r.Method),
						logging.String("path", r.URL.Path),
						logging.String("remote", r.RemoteAddr),
					)
					writeTenantError(w, http.StatusBadRequest,
						errors.ErrCodeValidation,
						"tenant ID is required: provide via header or query parameter")
					return
				}
				// Not required and no default — pass through without tenant context.
				next.ServeHTTP(w, r)
				return
			}

			// Format validation.
			if !validateTenantID(tenantID) {
				logger.Warn("invalid tenant ID format",
					logging.String("tenant_id", tenantID),
					logging.String("method", r.Method),
					logging.String("path", r.URL.Path),
				)
				writeTenantError(w, http.StatusBadRequest,
					errors.ErrCodeValidation,
					fmt.Sprintf("invalid tenant ID format: must match [a-zA-Z0-9_-]{1,64}, got %q", tenantID))
				return
			}

			// Whitelist check.
			if allowedSet != nil {
				if _, ok := allowedSet[tenantID]; !ok {
					logger.Warn("tenant ID not in allowed list",
						logging.String("tenant_id", tenantID),
						logging.String("method", r.Method),
						logging.String("path", r.URL.Path),
					)
					writeTenantError(w, http.StatusForbidden,
						errors.ErrCodeUnauthorized,
						fmt.Sprintf("tenant %q is not permitted", tenantID))
					return
				}
			}

			// External validator.
			if cfg.TenantValidator != nil {
				valid, err := cfg.TenantValidator(tenantID)
				if err != nil {
					logger.Error("tenant validation failed",
						logging.String("tenant_id", tenantID),
						logging.Err(err),
					)
					writeTenantError(w, http.StatusInternalServerError,
						errors.ErrCodeInternal,
						"tenant validation error")
					return
				}
				if !valid {
					logger.Warn("tenant rejected by validator",
						logging.String("tenant_id", tenantID),
						logging.String("method", r.Method),
						logging.String("path", r.URL.Path),
					)
					writeTenantError(w, http.StatusForbidden,
						errors.ErrCodeUnauthorized,
						fmt.Sprintf("tenant %q is not authorized", tenantID))
					return
				}
			}

			// Build TenantInfo and inject into context.
			info := &TenantInfo{
				ID:       tenantID,
				IsActive: true,
			}

			ctx := context.WithValue(r.Context(), tenantContextKey{}, info)
			r = r.WithContext(ctx)

			// Echo tenant ID in response header for client confirmation.
			w.Header().Set("X-Tenant-ID", tenantID)

			logger.Debug("tenant resolved",
				logging.String("tenant_id", tenantID),
				logging.String("method", r.Method),
				logging.String("path", r.URL.Path),
			)

			next.ServeHTTP(w, r)
		})
	}
}

// TenantFromContext retrieves TenantInfo from the request context.
// Returns nil and false if no tenant info is present.
func TenantFromContext(ctx context.Context) (*TenantInfo, bool) {
	info, ok := ctx.Value(tenantContextKey{}).(*TenantInfo)
	return info, ok
}

// MustTenantFromContext retrieves TenantInfo from the context or panics.
// Use only in code paths where tenant presence is guaranteed (e.g., after middleware).
func MustTenantFromContext(ctx context.Context) *TenantInfo {
	info, ok := TenantFromContext(ctx)
	if !ok || info == nil {
		panic("middleware/tenant: TenantInfo not found in context; ensure TenantMiddleware is applied")
	}
	return info
}

// extractTenantID attempts to extract the tenant ID from the request using
// the configured header name and query parameter, in that order.
func extractTenantID(r *http.Request, headerName, queryParam string) string {
	// Priority 1: HTTP header.
	if v := strings.TrimSpace(r.Header.Get(headerName)); v != "" {
		return v
	}

	// Priority 2: Query parameter.
	if v := strings.TrimSpace(r.URL.Query().Get(queryParam)); v != "" {
		return v
	}

	return ""
}

// validateTenantID checks that the tenant ID matches the allowed pattern:
// alphanumeric characters, underscores, and hyphens, length 1-64.
func validateTenantID(id string) bool {
	return tenantIDPattern.MatchString(id)
}

// tenantErrorResponse is the JSON body returned on tenant validation failures.
type tenantErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// writeTenantError writes a JSON error response for tenant-related failures.
func writeTenantError(w http.ResponseWriter, statusCode int, code errors.ErrorCode, message string) {
	resp := tenantErrorResponse{}
	resp.Error.Code = string(code)
	resp.Error.Message = message

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

//Personal.AI order the ending
