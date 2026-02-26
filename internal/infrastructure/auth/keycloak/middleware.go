package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Context keys
type contextKey string

const (
	ContextKeyClaims   contextKey = "auth_claims"
	ContextKeyUserID   contextKey = "user_id"
	ContextKeyTenantID contextKey = "tenant_id"
	ContextKeyRoles    contextKey = "user_roles"
)

var (
	ErrMissingAuthHeader = errors.ErrUnauthorized("missing authorization header")
	ErrInvalidAuthFormat = errors.ErrUnauthorized("invalid authorization format")
)

// AuthMiddleware handles request authentication.
type AuthMiddleware struct {
	authProvider         AuthProvider
	logger               logging.Logger
	skipPaths            map[string]bool
	skipPrefixes         []string
	requireIntrospection bool
	tenantExtractor      func(*TokenClaims) string
	onAuthFailure        func(w http.ResponseWriter, r *http.Request, err error)
}

// AuthMiddlewareConfig holds configuration for the middleware.
type AuthMiddlewareConfig struct {
	SkipPaths            []string
	SkipPrefixes         []string
	RequireIntrospection bool
	TenantClaimKey       string
}

// NewAuthMiddleware creates a new instance of AuthMiddleware.
func NewAuthMiddleware(provider AuthProvider, logger logging.Logger, cfg AuthMiddlewareConfig) *AuthMiddleware {
	skipPaths := make(map[string]bool)
	for _, p := range cfg.SkipPaths {
		skipPaths[p] = true
	}

	mw := &AuthMiddleware{
		authProvider:         provider,
		logger:               logger,
		skipPaths:            skipPaths,
		skipPrefixes:         cfg.SkipPrefixes,
		requireIntrospection: cfg.RequireIntrospection,
		tenantExtractor: func(c *TokenClaims) string {
			if c.TenantID != "" {
				return c.TenantID
			}
			return "default"
		},
		onAuthFailure: defaultAuthFailureHandler,
	}

	return mw
}

// MiddlewareOption defines functional options for the middleware.
type MiddlewareOption func(*AuthMiddleware)

func WithSkipPaths(paths ...string) MiddlewareOption {
	return func(mw *AuthMiddleware) {
		for _, p := range paths {
			mw.skipPaths[p] = true
		}
	}
}

func WithSkipPrefixes(prefixes ...string) MiddlewareOption {
	return func(mw *AuthMiddleware) {
		mw.skipPrefixes = append(mw.skipPrefixes, prefixes...)
	}
}

func WithIntrospection(enabled bool) MiddlewareOption {
	return func(mw *AuthMiddleware) {
		mw.requireIntrospection = enabled
	}
}

func WithAuthFailureHandler(handler func(http.ResponseWriter, *http.Request, error)) MiddlewareOption {
	return func(mw *AuthMiddleware) {
		mw.onAuthFailure = handler
	}
}

// Handler returns the HTTP handler for the middleware.
func (mw *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check skip paths
		if mw.skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		for _, prefix := range mw.skipPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}
		}

		// 2. Extract Token
		token, err := extractBearerToken(r)
		if err != nil {
			mw.handleError(w, r, err)
			return
		}

		// 3. Verify Token
		ctx := r.Context()
		claims, err := mw.authProvider.VerifyToken(ctx, token)
		if err != nil {
			mw.handleError(w, r, err)
			return
		}

		// 4. Introspection (Optional)
		if mw.requireIntrospection {
			res, err := mw.authProvider.IntrospectToken(ctx, token)
			if err != nil {
				mw.handleError(w, r, ErrTokenIntrospectionFailed.WithCause(err))
				return
			}
			if !res.Active {
				mw.handleError(w, r, ErrTokenExpired)
				return
			}
		}

		// 5. Inject Context
		ctx = context.WithValue(ctx, ContextKeyClaims, claims)
		ctx = context.WithValue(ctx, ContextKeyUserID, claims.Subject)
		ctx = context.WithValue(ctx, ContextKeyTenantID, mw.tenantExtractor(claims))
		ctx = context.WithValue(ctx, ContextKeyRoles, claims.RealmRoles)

		// 6. Next
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// HandlerFunc is a convenience wrapper for http.HandlerFunc.
func (mw *AuthMiddleware) HandlerFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mw.Handler(next).ServeHTTP(w, r)
	}
}

func (mw *AuthMiddleware) handleError(w http.ResponseWriter, r *http.Request, err error) {
	mw.logger.Warn("authentication failed",
		logging.String("path", r.URL.Path),
		logging.String("ip", r.RemoteAddr),
		logging.Error(err),
	)
	mw.onAuthFailure(w, r, err)
}

func extractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", ErrMissingAuthHeader
	}
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", ErrInvalidAuthFormat
	}
	return strings.TrimPrefix(auth, "Bearer "), nil
}

func defaultAuthFailureHandler(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")

	code := "UNAUTHORIZED"
	msg := "Authentication required"
	status := http.StatusUnauthorized

	if errors.Is(err, ErrTokenExpired) {
		code = "TOKEN_EXPIRED"
		msg = "Access token has expired"
	} else if errors.Is(err, ErrTokenInvalidSignature) {
		code = "TOKEN_INVALID"
		msg = "Invalid token signature"
	} else if errors.Is(err, ErrTokenMalformed) {
		code = "TOKEN_MALFORMED"
		msg = "Malformed authorization token"
	} else if errors.Is(err, ErrMissingAuthHeader) {
		code = "MISSING_AUTH_HEADER"
		msg = "Missing authorization header"
	} else if errors.Is(err, ErrTokenIntrospectionFailed) {
		code = "INTROSPECTION_FAILED"
		msg = "Token introspection failed"
		status = http.StatusInternalServerError
	} else if errors.IsForbidden(err) {
		code = "ACCESS_DENIED"
		msg = "Access denied"
		status = http.StatusForbidden
	}

	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": msg,
	})
}

// Context Helpers

func ClaimsFromContext(ctx context.Context) (*TokenClaims, bool) {
	c, ok := ctx.Value(ContextKeyClaims).(*TokenClaims)
	return c, ok
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(string)
	return v, ok
}

func TenantIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyTenantID).(string)
	return v, ok
}

func RolesFromContext(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(ContextKeyRoles).([]string)
	return v, ok
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
