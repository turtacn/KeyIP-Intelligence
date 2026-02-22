package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockAuthProvider
type MockAuthProvider struct {
	mock.Mock
}

func (m *MockAuthProvider) VerifyToken(ctx context.Context, rawToken string) (*TokenClaims, error) {
	args := m.Called(ctx, rawToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TokenClaims), args.Error(1)
}

func (m *MockAuthProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	args := m.Called(ctx, accessToken)
	return args.Get(0).(*UserInfo), args.Error(1)
}

func (m *MockAuthProvider) IntrospectToken(ctx context.Context, token string) (*IntrospectionResult, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*IntrospectionResult), args.Error(1)
}

func (m *MockAuthProvider) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	args := m.Called(ctx, refreshToken)
	return args.Get(0).(*TokenPair), args.Error(1)
}

func (m *MockAuthProvider) GetServiceToken(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockAuthProvider) Logout(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

func (m *MockAuthProvider) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	mockAuth := new(MockAuthProvider)
	logger := logging.NewNopLogger()

	claims := &TokenClaims{
		Subject:  "user-1",
		TenantID: "tenant-1",
		RealmRoles: []string{"admin"},
	}
	mockAuth.On("VerifyToken", mock.Anything, "valid-token").Return(claims, nil)

	mw := NewAuthMiddleware(mockAuth, logger, AuthMiddlewareConfig{})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := ClaimsFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "user-1", c.Subject)

		uid, ok := UserIDFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "user-1", uid)

		tid, ok := TenantIDFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "tenant-1", tid)

		assert.True(t, HasRole(r.Context(), "admin"))

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	mockAuth := new(MockAuthProvider)
	logger := logging.NewNopLogger()
	mw := NewAuthMiddleware(mockAuth, logger, AuthMiddlewareConfig{})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	mockAuth := new(MockAuthProvider)
	logger := logging.NewNopLogger()
	mockAuth.On("VerifyToken", mock.Anything, "invalid-token").Return(nil, ErrTokenInvalidSignature)

	mw := NewAuthMiddleware(mockAuth, logger, AuthMiddlewareConfig{})
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "TOKEN_INVALID", resp["code"])
}

func TestAuthMiddleware_SkipPath(t *testing.T) {
	mockAuth := new(MockAuthProvider)
	logger := logging.NewNopLogger()

	cfg := AuthMiddlewareConfig{
		SkipPaths: []string{"/health"},
	}
	mw := NewAuthMiddleware(mockAuth, logger, cfg)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	mockAuth.AssertNotCalled(t, "VerifyToken")
}

func TestAuthMiddleware_Introspection(t *testing.T) {
	mockAuth := new(MockAuthProvider)
	logger := logging.NewNopLogger()

	cfg := AuthMiddlewareConfig{
		RequireIntrospection: true,
	}

	claims := &TokenClaims{Subject: "user-1"}
	mockAuth.On("VerifyToken", mock.Anything, "token").Return(claims, nil)
	mockAuth.On("IntrospectToken", mock.Anything, "token").Return(&IntrospectionResult{Active: false}, nil)

	mw := NewAuthMiddleware(mockAuth, logger, cfg)
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "TOKEN_EXPIRED", resp["code"])
}
//Personal.AI order the ending
