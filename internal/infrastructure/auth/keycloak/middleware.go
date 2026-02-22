package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type contextKey string

const (
	ContextKeyClaims   contextKey = "auth_claims"
	ContextKeyUserID   contextKey = "user_id"
	ContextKeyTenantID contextKey = "tenant_id"
	ContextKeyRoles    contextKey = "user_roles"
)

// AuthMiddlewareConfig configuration for AuthMiddleware.
type AuthMiddlewareConfig struct {
	SkipPaths          []string
	SkipPrefixes       []string
	RequireIntrospection bool
	TenantClaimKey       string
}

// AuthMiddleware is the authentication middleware.
type AuthMiddleware struct {
	authProvider    AuthProvider
	logger          logging.Logger
	skipPaths       map[string]bool
	skipPrefixes    []string
	requireIntrospection bool
	tenantExtractor func(*TokenClaims) string
	onAuthFailure   func(w http.ResponseWriter, r *http.Request, err error)
}

// MiddlewareOption is a function option for configuring AuthMiddleware.
type MiddlewareOption func(*AuthMiddleware)

// WithSkipPaths sets the skip paths.
func WithSkipPaths(paths ...string) MiddlewareOption {
	return func(m *AuthMiddleware) {
		for _, p := range paths {
			m.skipPaths[p] = true
		}
	}
}

// WithSkipPrefixes sets the skip prefixes.
func WithSkipPrefixes(prefixes ...string) MiddlewareOption {
	return func(m *AuthMiddleware) {
		m.skipPrefixes = append(m.skipPrefixes, prefixes...)
	}
}

// WithIntrospection enables token introspection.
func WithIntrospection(enabled bool) MiddlewareOption {
	return func(m *AuthMiddleware) {
		m.requireIntrospection = enabled
	}
}

// WithAuthFailureHandler sets the authentication failure handler.
func WithAuthFailureHandler(handler func(http.ResponseWriter, *http.Request, error)) MiddlewareOption {
	return func(m *AuthMiddleware) {
		m.onAuthFailure = handler
	}
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(provider AuthProvider, logger logging.Logger, cfg AuthMiddlewareConfig, opts ...MiddlewareOption) *AuthMiddleware {
	m := &AuthMiddleware{
		authProvider:    provider,
		logger:          logger,
		skipPaths:       make(map[string]bool),
		skipPrefixes:    cfg.SkipPrefixes,
		requireIntrospection: cfg.RequireIntrospection,
		tenantExtractor: func(claims *TokenClaims) string {
			return claims.TenantID
		},
		onAuthFailure: defaultAuthFailureHandler,
	}

	for _, p := range cfg.SkipPaths {
		m.skipPaths[p] = true
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check skip paths
		if m.skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		for _, prefix := range m.skipPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Extract token
		token, err := extractBearerToken(r)
		if err != nil {
			m.handleError(w, r, err)
			return
		}

		// Verify token
		ctx := r.Context()
		claims, err := m.authProvider.VerifyToken(ctx, token)
		if err != nil {
			m.handleError(w, r, err)
			return
		}

		// Introspection
		if m.requireIntrospection {
			result, err := m.authProvider.IntrospectToken(ctx, token)
			if err != nil {
				// Introspection failure usually means internal error or unavailable
				// But we treat it as auth failure for safety
				m.logger.Error("Token introspection failed", logging.Err(err))
				m.handleError(w, r, err)
				return
			}
			if !result.Active {
				m.handleError(w, r, ErrTokenExpired) // Treat as expired/invalid
				return
			}
		}

		// Inject into context
		ctx = context.WithValue(ctx, ContextKeyClaims, claims)
		ctx = context.WithValue(ctx, ContextKeyUserID, claims.Subject)

		tenantID := m.tenantExtractor(claims)
		if tenantID != "" {
			ctx = context.WithValue(ctx, ContextKeyTenantID, tenantID)
		}

		// Roles: combine realm roles and client roles?
		// Usually we care about realm roles or specific client roles.
		// Let's flatten all roles for convenience.
		var allRoles []string
		allRoles = append(allRoles, claims.RealmRoles...)
		for _, roles := range claims.ClientRoles {
			allRoles = append(allRoles, roles...)
		}
		ctx = context.WithValue(ctx, ContextKeyRoles, allRoles)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) HandlerFunc(next http.HandlerFunc) http.HandlerFunc {
	return m.Handler(next).ServeHTTP
}

func (m *AuthMiddleware) handleError(w http.ResponseWriter, r *http.Request, err error) {
	// Log error without token
	m.logger.Warn("Authentication failed",
		logging.String("path", r.URL.Path),
		logging.String("ip", r.RemoteAddr),
		logging.Err(err),
	)
	m.onAuthFailure(w, r, err)
}

func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrMissingAuthHeader
	}
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", ErrInvalidAuthFormat
	}
	return strings.TrimPrefix(authHeader, "Bearer "), nil
}

func defaultAuthFailureHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)

	resp := map[string]string{
		"code":    "UNAUTHORIZED",
		"message": "Authentication required",
	}

	if err == ErrTokenExpired {
		resp["code"] = "TOKEN_EXPIRED"
		resp["message"] = "Access token has expired"
	} else if err == ErrTokenInvalidSignature {
		resp["code"] = "TOKEN_INVALID"
		resp["message"] = "Invalid token signature"
	} else if err == ErrTokenMalformed || err == ErrInvalidAuthFormat {
		resp["code"] = "TOKEN_MALFORMED"
		resp["message"] = "Malformed authorization token"
	}

	json.NewEncoder(w).Encode(resp)
}

// Errors
// Note: Some errors are defined in client.go, we reuse them.
// We define middleware specific errors here if not exported from client.go?
// ErrMissingAuthHeader is new.
// ErrTokenExpired etc are exported from client.go (Wait, variables in client.go are exported).

var (
	ErrMissingAuthHeader = &authError{"missing authorization header"}
	ErrInvalidAuthFormat = &authError{"invalid authorization format"}
)

type authError struct {
	msg string
}

func (e *authError) Error() string {
	return e.msg
}

// Context helpers

func ClaimsFromContext(ctx context.Context) (*TokenClaims, bool) {
	claims, ok := ctx.Value(ContextKeyClaims).(*TokenClaims)
	return claims, ok
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	uid, ok := ctx.Value(ContextKeyUserID).(string)
	return uid, ok
}

func TenantIDFromContext(ctx context.Context) (string, bool) {
	tid, ok := ctx.Value(ContextKeyTenantID).(string)
	return tid, ok
}

func RolesFromContext(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(ContextKeyRoles).([]string)
	return roles, ok
}

func HasRole(ctx context.Context, role string) bool {
	roles, ok := RolesFromContext(ctx)
	if !ok {
		return false
	}
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

func HasAnyRole(ctx context.Context, roles ...string) bool {
	userRoles, ok := RolesFromContext(ctx)
	if !ok {
		return false
	}
	for _, r := range roles {
		for _, ur := range userRoles {
			if r == ur {
				return true
			}
		}
	}
	return false
}

//Personal.AI order the ending
