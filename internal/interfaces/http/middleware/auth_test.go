// Phase 11 - 接口层: HTTP Middleware - 认证中间件单元测试
// 序号: 273
// 文件: internal/interfaces/http/middleware/auth_test.go
// 测试用例:
//   - TestAuthenticate_BearerToken_Valid: 有效 JWT token 认证成功
//   - TestAuthenticate_BearerToken_Expired: 过期 token 返回 401
//   - TestAuthenticate_BearerToken_Invalid: 无效 token 返回 401
//   - TestAuthenticate_BearerToken_MalformedHeader: 格式错误的 Authorization header
//   - TestAuthenticate_APIKey_Valid: 有效 API key 认证成功
//   - TestAuthenticate_APIKey_Invalid: 无效 API key 返回 401
//   - TestAuthenticate_NoCredentials: 无凭证返回 401
//   - TestAuthenticate_SkipPaths: 白名单路径跳过认证
//   - TestAuthenticate_SkipPaths_SubPath: 白名单子路径跳过认证
//   - TestOptionalAuth_WithToken: 可选认证带 token
//   - TestOptionalAuth_WithoutToken: 可选认证无 token 继续匿名
//   - TestOptionalAuth_InvalidToken: 可选认证无效 token 继续匿名
//   - TestContextGetClaims_Present: context 中有 claims
//   - TestContextGetClaims_Absent: context 中无 claims
//   - TestContextGetAPIKeyInfo_Present: context 中有 API key info
//   - TestContextGetTenantID_FromClaims: 从 claims 获取 tenant ID
//   - TestContextGetTenantID_FromAPIKey: 从 API key 获取 tenant ID
//   - TestContextGetTenantID_Anonymous: 匿名无 tenant ID
//   - TestIsAuthenticated_True: 已认证
//   - TestIsAuthenticated_False: 未认证
//   - TestExtractBearerToken: 各种 Authorization header 格式
//   - TestExtractAPIKey_Header: 从 header 提取
//   - TestExtractAPIKey_QueryParam: 从 query 参数提取
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// --- Mock implementations ---

type mockTokenValidator struct {
	mock.Mock
}

func (m *mockTokenValidator) ValidateToken(token string) (*Claims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Claims), args.Error(1)
}

type mockAPIKeyValidator struct {
	mock.Mock
}

func (m *mockAPIKeyValidator) ValidateAPIKey(key string) (*APIKeyInfo, error) {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*APIKeyInfo), args.Error(1)
}

type mockMiddlewareLogger struct {
	mock.Mock
}

func (m *mockMiddlewareLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockMiddlewareLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockMiddlewareLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockMiddlewareLogger) Error(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}
func (m *mockMiddlewareLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockMiddlewareLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockMiddlewareLogger) WithError(err error) logging.Logger { return m }
func (m *mockMiddlewareLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockMiddlewareLogger) Sync() error { return nil }




// testHandler is a simple handler that records whether it was called.
func testHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})
}

func newTestAuthMiddleware() (*AuthMiddleware, *mockTokenValidator, *mockAPIKeyValidator) {
	tv := new(mockTokenValidator)
	akv := new(mockAPIKeyValidator)
	logger := new(mockMiddlewareLogger)
	logger.On("Error", mock.Anything, mock.Anything).Maybe()

	config := AuthConfig{
		SkipPaths:               []string{"/health", "/metrics"},
		AllowExpiredGracePeriod: 0,
	}

	m := NewAuthMiddleware(tv, akv, config, logger)
	return m, tv, akv
}

// --- Authenticate Tests ---

func TestAuthenticate_BearerToken_Valid(t *testing.T) {
	m, tv, _ := newTestAuthMiddleware()

	claims := &Claims{
		UserID:    "user-001",
		TenantID:  "tenant-001",
		Roles:     []string{"analyst"},
		ExpiresAt: time.Now().Add(1 * time.Hour),
		IssuedAt:  time.Now(),
	}
	tv.On("ValidateToken", "valid-token-123").Return(claims, nil)

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.Header.Set("Authorization", "Bearer valid-token-123")
	handler.ServeHTTP(w, r)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
	tv.AssertExpectations(t)
}

func TestAuthenticate_BearerToken_Expired(t *testing.T) {
	m, tv, _ := newTestAuthMiddleware()

	claims := &Claims{
		UserID:    "user-001",
		TenantID:  "tenant-001",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // expired
		IssuedAt:  time.Now().Add(-2 * time.Hour),
	}
	tv.On("ValidateToken", "expired-token").Return(claims, nil)

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.Header.Set("Authorization", "Bearer expired-token")
	handler.ServeHTTP(w, r)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthenticate_BearerToken_Invalid(t *testing.T) {
	m, tv, _ := newTestAuthMiddleware()

	tv.On("ValidateToken", "bad-token").Return(nil, fmt.Errorf("invalid signature"))

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.Header.Set("Authorization", "Bearer bad-token")
	handler.ServeHTTP(w, r)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthenticate_BearerToken_MalformedHeader(t *testing.T) {
	m, _, akv := newTestAuthMiddleware()

	// Malformed: no "Bearer" prefix, no API key either → 401
	akv.On("ValidateAPIKey", mock.Anything).Maybe()

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	handler.ServeHTTP(w, r)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthenticate_APIKey_Valid(t *testing.T) {
	m, _, akv := newTestAuthMiddleware()

	info := &APIKeyInfo{
		KeyID:     "key-001",
		TenantID:  "tenant-002",
		Scopes:    []string{"read", "write"},
		RateLimit: 1000,
	}
	akv.On("ValidateAPIKey", "ak_test_valid_key").Return(info, nil)

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.Header.Set("X-API-Key", "ak_test_valid_key")
	handler.ServeHTTP(w, r)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
	akv.AssertExpectations(t)
}

func TestAuthenticate_APIKey_Invalid(t *testing.T) {
	m, _, akv := newTestAuthMiddleware()

	akv.On("ValidateAPIKey", "bad-key").Return(nil, fmt.Errorf("key not found"))

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.Header.Set("X-API-Key", "bad-key")
	handler.ServeHTTP(w, r)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthenticate_NoCredentials(t *testing.T) {
	m, _, _ := newTestAuthMiddleware()

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	handler.ServeHTTP(w, r)

	assert.False(t, called)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "UNAUTHORIZED", errObj["code"])
}

func TestAuthenticate_SkipPaths(t *testing.T) {
	m, _, _ := newTestAuthMiddleware()

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health", nil)
	handler.ServeHTTP(w, r)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuthenticate_SkipPaths_SubPath(t *testing.T) {
	m, _, _ := newTestAuthMiddleware()

	called := false
	handler := m.Authenticate()(testHandler(&called))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/metrics/prometheus", nil)
	handler.ServeHTTP(w, r)

	assert.True(t, called)
}

// --- OptionalAuth Tests ---

func TestOptionalAuth_WithToken(t *testing.T) {
	m, tv, _ := newTestAuthMiddleware()

	claims := &Claims{
		UserID:    "user-opt",
		TenantID:  "tenant-opt",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	tv.On("ValidateToken", "opt-token").Return(claims, nil)

	var capturedCtx context.Context
	handler := m.OptionalAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/search", nil)
	r.Header.Set("Authorization", "Bearer opt-token")
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, ContextGetClaims(capturedCtx))
	assert.Equal(t, "user-opt", ContextGetClaims(capturedCtx).UserID)
}

func TestOptionalAuth_WithoutToken(t *testing.T) {
	m, _, _ := newTestAuthMiddleware()

	var capturedCtx context.Context
	handler := m.OptionalAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/search", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, ContextGetClaims(capturedCtx))
	assert.Nil(t, ContextGetAPIKeyInfo(capturedCtx))
}

func TestOptionalAuth_InvalidToken(t *testing.T) {
	m, tv, _ := newTestAuthMiddleware()

	tv.On("ValidateToken", "invalid").Return(nil, fmt.Errorf("bad"))

	var capturedCtx context.Context
	handler := m.OptionalAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/search", nil)
	r.Header.Set("Authorization", "Bearer invalid")
	handler.ServeHTTP(w, r)

	// Should still succeed as anonymous
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, ContextGetClaims(capturedCtx))
}

// --- Context Helper Tests ---

func TestContextGetClaims_Present(t *testing.T) {
	claims := &Claims{UserID: "u1", TenantID: "t1"}
	ctx := context.WithValue(context.Background(), claimsContextKey, claims)
	result := ContextGetClaims(ctx)
	assert.NotNil(t, result)
	assert.Equal(t, "u1", result.UserID)
}

func TestContextGetClaims_Absent(t *testing.T) {
	ctx := context.Background()
	assert.Nil(t, ContextGetClaims(ctx))
}

func TestContextGetAPIKeyInfo_Present(t *testing.T) {
	info := &APIKeyInfo{KeyID: "k1", TenantID: "t2"}
	ctx := context.WithValue(context.Background(), apiKeyInfoContextKey, info)
	result := ContextGetAPIKeyInfo(ctx)
	assert.NotNil(t, result)
	assert.Equal(t, "k1", result.KeyID)
}

func TestContextGetTenantID_FromClaims(t *testing.T) {
	claims := &Claims{TenantID: "tenant-from-jwt"}
	ctx := context.WithValue(context.Background(), claimsContextKey, claims)
	assert.Equal(t, "tenant-from-jwt", ContextGetTenantID(ctx))
}

func TestContextGetTenantID_FromAPIKey(t *testing.T) {
	info := &APIKeyInfo{TenantID: "tenant-from-key"}
	ctx := context.WithValue(context.Background(), apiKeyInfoContextKey, info)
	assert.Equal(t, "tenant-from-key", ContextGetTenantID(ctx))
}

func TestContextGetTenantID_Anonymous(t *testing.T) {
	ctx := context.Background()
	assert.Equal(t, "", ContextGetTenantID(ctx))
}

func TestIsAuthenticated_True(t *testing.T) {
	claims := &Claims{UserID: "u1"}
	ctx := context.WithValue(context.Background(), claimsContextKey, claims)
	assert.True(t, IsAuthenticated(ctx))
}

func TestIsAuthenticated_False(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsAuthenticated(ctx))
}

// --- extractBearerToken Tests ---

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected string
	}{
		{"valid bearer", "Bearer abc123", "abc123"},
		{"bearer lowercase", "bearer abc123", "abc123"},
		{"BEARER uppercase", "BEARER abc123", "abc123"},
		{"with spaces", "Bearer  abc123 ", "abc123"},
		{"empty", "", ""},
		{"basic auth", "Basic dXNlcjpwYXNz", ""},
		{"no space", "Bearerabc123", ""},
		{"only bearer", "Bearer ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			assert.Equal(t, tt.expected, extractBearerToken(r))
		})
	}
}

// --- extractAPIKey Tests ---

func TestExtractAPIKey_Header(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "ak_test_key_123")
	assert.Equal(t, "ak_test_key_123", extractAPIKey(r))
}

func TestExtractAPIKey_QueryParam(t *testing.T) {
	r := httptest.NewRequest("GET", "/?api_key=ak_query_key", nil)
	assert.Equal(t, "ak_query_key", extractAPIKey(r))
}

func TestExtractAPIKey_HeaderPriority(t *testing.T) {
	r := httptest.NewRequest("GET", "/?api_key=query_key", nil)
	r.Header.Set("X-API-Key", "header_key")
	// Header should take priority
	assert.Equal(t, "header_key", extractAPIKey(r))
}

func TestExtractAPIKey_None(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	assert.Equal(t, "", extractAPIKey(r))
}

//Personal.AI order the ending
