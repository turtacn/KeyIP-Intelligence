package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Mock AuthProvider
type mockAuthProvider struct {
	verifyTokenFunc     func(ctx context.Context, token string) (*TokenClaims, error)
	introspectTokenFunc func(ctx context.Context, token string) (*IntrospectionResult, error)
}

func (m *mockAuthProvider) VerifyToken(ctx context.Context, rawToken string) (*TokenClaims, error) {
	if m.verifyTokenFunc != nil {
		return m.verifyTokenFunc(ctx, rawToken)
	}
	return nil, nil
}

func (m *mockAuthProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	return nil, nil
}

func (m *mockAuthProvider) IntrospectToken(ctx context.Context, token string) (*IntrospectionResult, error) {
	if m.introspectTokenFunc != nil {
		return m.introspectTokenFunc(ctx, token)
	}
	return nil, nil
}

func (m *mockAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	return nil, nil
}

func (m *mockAuthProvider) GetServiceToken(ctx context.Context) (string, error) {
	return "", nil
}

func (m *mockAuthProvider) Logout(ctx context.Context, refreshToken string) error {
	return nil
}

func (m *mockAuthProvider) Health(ctx context.Context) error {
	return nil
}

func newTestMiddleware(provider AuthProvider, opts ...MiddlewareOption) *AuthMiddleware {
	cfg := AuthMiddlewareConfig{
		SkipPaths:    []string{"/health"},
		SkipPrefixes: []string{"/public"},
	}
	mw := NewAuthMiddleware(provider, newMockLogger(), cfg)
	for _, opt := range opts {
		opt(mw)
	}
	return mw
}

func executeMiddleware(mw *AuthMiddleware, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	handler.ServeHTTP(rr, req)
	return rr
}

func TestMiddleware_ValidToken_PassesThrough(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			return &TokenClaims{Subject: "user-123"}, nil
		},
	}
	mw := newTestMiddleware(provider)

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	rr := executeMiddleware(mw, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())
}

func TestMiddleware_ValidToken_ContextInjection(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			return &TokenClaims{
				Subject:    "user-123",
				TenantID:   "tenant-abc",
				RealmRoles: []string{"admin"},
			}, nil
		},
	}
	mw := newTestMiddleware(provider)

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	var claims *TokenClaims
	var userID, tenantID string
	var roles []string

	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ = ClaimsFromContext(r.Context())
		userID, _ = UserIDFromContext(r.Context())
		tenantID, _ = TenantIDFromContext(r.Context())
		roles, _ = RolesFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "user-123", claims.Subject)
	assert.Equal(t, "user-123", userID)
	assert.Equal(t, "tenant-abc", tenantID)
	assert.Contains(t, roles, "admin")
}

func TestMiddleware_MissingAuthHeader(t *testing.T) {
	provider := &mockAuthProvider{}
	mw := newTestMiddleware(provider)

	req := httptest.NewRequest("GET", "/api/protected", nil)
	rr := executeMiddleware(mw, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var body map[string]string
	json.Unmarshal(rr.Body.Bytes(), &body)
	assert.Equal(t, "MISSING_AUTH_HEADER", body["code"])
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			return nil, ErrTokenExpired
		},
	}
	mw := newTestMiddleware(provider)

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer expired")
	rr := executeMiddleware(mw, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var body map[string]string
	json.Unmarshal(rr.Body.Bytes(), &body)
	assert.Equal(t, "TOKEN_EXPIRED", body["code"])
}

func TestMiddleware_SkipPath(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			t.Fatal("Should not call VerifyToken")
			return nil, nil
		},
	}
	mw := newTestMiddleware(provider)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := executeMiddleware(mw, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMiddleware_SkipPrefix(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			t.Fatal("Should not call VerifyToken")
			return nil, nil
		},
	}
	mw := newTestMiddleware(provider)

	req := httptest.NewRequest("GET", "/public/image.png", nil)
	rr := executeMiddleware(mw, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMiddleware_WithIntrospection_Active(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			return &TokenClaims{Subject: "user-123"}, nil
		},
		introspectTokenFunc: func(ctx context.Context, token string) (*IntrospectionResult, error) {
			return &IntrospectionResult{Active: true}, nil
		},
	}
	mw := newTestMiddleware(provider, WithIntrospection(true))

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := executeMiddleware(mw, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestMiddleware_WithIntrospection_Inactive(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			return &TokenClaims{Subject: "user-123"}, nil
		},
		introspectTokenFunc: func(ctx context.Context, token string) (*IntrospectionResult, error) {
			return &IntrospectionResult{Active: false}, nil
		},
	}
	mw := newTestMiddleware(provider, WithIntrospection(true))

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := executeMiddleware(mw, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	var body map[string]string
	json.Unmarshal(rr.Body.Bytes(), &body)
	assert.Equal(t, "TOKEN_EXPIRED", body["code"])
}

func TestMiddleware_WithIntrospection_Error(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			return &TokenClaims{Subject: "user-123"}, nil
		},
		introspectTokenFunc: func(ctx context.Context, token string) (*IntrospectionResult, error) {
			return nil, errors.ErrInternal("introspection failed")
		},
	}
	mw := newTestMiddleware(provider, WithIntrospection(true))

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer token")
	rr := executeMiddleware(mw, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHasRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKeyRoles, []string{"admin", "editor"})
	assert.True(t, HasRole(ctx, "admin"))
	assert.False(t, HasRole(ctx, "viewer"))
}

func TestHasAnyRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKeyRoles, []string{"admin", "editor"})
	assert.True(t, HasAnyRole(ctx, "viewer", "admin"))
	assert.False(t, HasAnyRole(ctx, "viewer", "guest"))
}

func TestHandlerFunc_Adapter(t *testing.T) {
	provider := &mockAuthProvider{
		verifyTokenFunc: func(ctx context.Context, token string) (*TokenClaims, error) {
			return &TokenClaims{Subject: "user-123"}, nil
		},
	}
	mw := newTestMiddleware(provider)

	req := httptest.NewRequest("GET", "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	mw.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
//Personal.AI order the ending
