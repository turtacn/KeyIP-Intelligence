package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// tenantContextKey is the unexported key type for storing tenant info in context.
type tenantContextKey struct{}

// TenantInfo holds the resolved tenant information injected into the request context.
type TenantInfo struct {
	TenantID   string            `json:"tenant_id"`
	TenantName string            `json:"tenant_name,omitempty"`
	Plan       string            `json:"plan,omitempty"` // free/pro/enterprise
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// TenantResolver resolves tenant information from a tenant ID.
type TenantResolver interface {
	Resolve(ctx context.Context, tenantID string) (*TenantInfo, error)
}

// TenantCache caches resolved tenant information.
type TenantCache interface {
	Get(ctx context.Context, tenantID string) (*TenantInfo, bool)
	Set(ctx context.Context, tenantID string, info *TenantInfo, ttl time.Duration)
}

// TenantMiddlewareConfig holds configuration for the tenant middleware.
type TenantMiddlewareConfig struct {
	HeaderName      string
	QueryParam      string
	Required        bool
	DefaultTenantID string
	TenantResolver  TenantResolver
	Cache           TenantCache
	Logger          logging.Logger
}

// tenantIDPattern enforces: alphanumeric, underscore, hyphen, length 1-128.
var tenantIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// NewTenantMiddleware creates a new tenant middleware.
func NewTenantMiddleware(cfg TenantMiddlewareConfig) func(http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Tenant-ID"
	}
	if cfg.QueryParam == "" {
		cfg.QueryParam = "tenant_id"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			tenantID := r.Header.Get(cfg.HeaderName)
			if tenantID == "" {
				tenantID = r.URL.Query().Get(cfg.QueryParam)
			}

			if tenantID == "" {
				if cfg.Required {
					writeTenantError(w, http.StatusUnauthorized, errors.ErrTenantRequired, "tenant ID is required")
					return
				}
				tenantID = cfg.DefaultTenantID
			}

			// If tenantID is still empty (not required and no default), proceed without tenant info
			if tenantID != "" {
				if !tenantIDPattern.MatchString(tenantID) {
					writeTenantError(w, http.StatusBadRequest, errors.ErrInvalidTenantID, "invalid tenant ID format")
					return
				}

				var info *TenantInfo
				var found bool

				// Check cache
				if cfg.Cache != nil {
					info, found = cfg.Cache.Get(ctx, tenantID)
				}

				if !found {
					if cfg.TenantResolver != nil {
						resolvedInfo, err := cfg.TenantResolver.Resolve(ctx, tenantID)
						if err != nil {
							cfg.Logger.Error("failed to resolve tenant", logging.Err(err), logging.String("tenant_id", tenantID))
							writeTenantError(w, http.StatusInternalServerError, errors.ErrTenantResolveFailed, "failed to resolve tenant")
							return
						}
						if resolvedInfo == nil {
							writeTenantError(w, http.StatusForbidden, errors.ErrTenantNotFound, "tenant not found")
							return
						}
						info = resolvedInfo
						if cfg.Cache != nil {
							cfg.Cache.Set(ctx, tenantID, info, 5*time.Minute)
						}
					} else {
						// Minimal info if no resolver
						info = &TenantInfo{TenantID: tenantID}
					}
				}

				ctx = ContextWithTenant(ctx, info)
				w.Header().Set("X-Tenant-ID", tenantID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TenantFromContext retrieves TenantInfo from the request context.
func TenantFromContext(ctx context.Context) (*TenantInfo, bool) {
	info, ok := ctx.Value(tenantContextKey{}).(*TenantInfo)
	return info, ok
}

// ContextWithTenant injects TenantInfo into the context.
func ContextWithTenant(ctx context.Context, info *TenantInfo) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, info)
}

// DefaultTenantResolver is a simple in-memory resolver implementation.
type DefaultTenantResolver struct {
	tenants map[string]*TenantInfo
}

func NewDefaultTenantResolver() *DefaultTenantResolver {
	return &DefaultTenantResolver{
		tenants: make(map[string]*TenantInfo),
	}
}

func (r *DefaultTenantResolver) Register(info *TenantInfo) {
	r.tenants[info.TenantID] = info
}

func (r *DefaultTenantResolver) Resolve(ctx context.Context, tenantID string) (*TenantInfo, error) {
	if info, ok := r.tenants[tenantID]; ok {
		return info, nil
	}
	return nil, nil
}

func writeTenantError(w http.ResponseWriter, statusCode int, code errors.ErrorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := common.ErrorResponse{
		Error: common.ErrorDetail{
			Code:    string(code),
			Message: message,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// DefaultTenantConfig returns a TenantMiddlewareConfig with default values.
func DefaultTenantConfig() TenantMiddlewareConfig {
	return TenantMiddlewareConfig{
		HeaderName:      "X-Tenant-ID",
		QueryParam:      "tenant_id",
		Required:        true,
		DefaultTenantID: "",
	}
}

// TenantMiddleware wraps the tenant middleware function.
type TenantMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewTenantMiddlewareWrapper creates a new TenantMiddleware wrapper.
func NewTenantMiddlewareWrapper(cfg TenantMiddlewareConfig, logger logging.Logger) *TenantMiddleware {
	cfg.Logger = logger
	return &TenantMiddleware{
		handler: NewTenantMiddleware(cfg),
	}
}

// Handler returns the middleware handler.
func (m *TenantMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

//Personal.AI order the ending
