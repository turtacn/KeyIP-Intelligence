// File: internal/interfaces/http/middleware/tenant_test.go
// Phase 11 - 序号 281
//
// 生成计划:
// 功能定位: 验证 TenantMiddleware 全部提取路径、校验逻辑、上下文注入行为
// 测试用例: Header提取、QueryParam提取、默认值回退、Required模式、格式校验、
//           白名单、自定义验证器、响应头回写、上下文读取、panic恢复
// Mock依赖: mockLogger
// 断言: HTTP状态码、JSON响应体、上下文值、响应头

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// --- Mock Logger ---

type tenantMockLogger struct{}

func (l *tenantMockLogger) Debug(msg string, fields ...logging.Field) {}
func (l *tenantMockLogger) Info(msg string, fields ...logging.Field)  {}
func (l *tenantMockLogger) Warn(msg string, fields ...logging.Field)  {}
func (l *tenantMockLogger) Error(msg string, fields ...logging.Field) {}
func (l *tenantMockLogger) Fatal(msg string, fields ...logging.Field) {}
func (l *tenantMockLogger) With(fields ...logging.Field) logging.Logger { return l }
func (l *tenantMockLogger) WithContext(ctx context.Context) logging.Logger { return l }
func (l *tenantMockLogger) WithError(err error) logging.Logger { return l }
func (l *tenantMockLogger) Sync() error { return nil }

func newTenantMockLogger() *tenantMockLogger {
	return &tenantMockLogger{}
}

// --- Helper: execute middleware and capture result ---

func executeTenantMiddleware(cfg TenantConfig, r *http.Request) (*httptest.ResponseRecorder, *TenantInfo) {
	logger := newTenantMockLogger()
	mw := NewTenantMiddleware(cfg, logger)

	var captured *TenantInfo
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := TenantFromContext(r.Context())
		if ok {
			captured = info
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, r)
	return rr, captured
}

// --- Tests ---

func TestTenantMiddleware_ExtractFromHeader(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "tenant-alpha")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "tenant-alpha", info.ID)
	assert.True(t, info.IsActive)
}

func TestTenantMiddleware_ExtractFromQueryParam(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents?tenant_id=tenant-beta", nil)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "tenant-beta", info.ID)
}

func TestTenantMiddleware_FallbackToDefault(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.Required = false
	cfg.DefaultTenantID = "default-tenant"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "default-tenant", info.ID)
}

func TestTenantMiddleware_RequiredMode_Missing(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.Required = true
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Nil(t, info)

	var body tenantErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body.Error.Message, "tenant ID is required")
}

func TestTenantMiddleware_NotRequired_NoDefault_PassThrough(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.Required = false
	cfg.DefaultTenantID = ""
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	// No tenant info injected — handler still called.
	assert.Nil(t, info)
}

func TestTenantMiddleware_InvalidFormat_TooLong(t *testing.T) {
	cfg := DefaultTenantConfig()
	longID := strings.Repeat("a", 65)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", longID)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Nil(t, info)

	var body tenantErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body.Error.Message, "invalid tenant ID format")
}

func TestTenantMiddleware_InvalidFormat_SpecialChars(t *testing.T) {
	cfg := DefaultTenantConfig()

	invalidIDs := []string{
		"tenant@evil",
		"tenant/path",
		"tenant id",
		"tenant;drop",
		"<script>",
		"tenant.dot",
		"",
	}

	for _, id := range invalidIDs {
		if id == "" {
			continue // empty is handled by Required logic, not format
		}
		t.Run(fmt.Sprintf("id=%s", id), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
			req.Header.Set("X-Tenant-ID", id)

			rr, info := executeTenantMiddleware(cfg, req)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Nil(t, info)
		})
	}
}

func TestTenantMiddleware_AllowedTenants_Permitted(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.AllowedTenants = []string{"tenant-a", "tenant-b", "tenant-c"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "tenant-b")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "tenant-b", info.ID)
}

func TestTenantMiddleware_AllowedTenants_Rejected(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.AllowedTenants = []string{"tenant-a", "tenant-b"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "tenant-x")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Nil(t, info)

	var body tenantErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body.Error.Message, "not permitted")
}

func TestTenantMiddleware_CustomValidator_Valid(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.TenantValidator = func(tenantID string) (bool, error) {
		return tenantID == "validated-tenant", nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "validated-tenant")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "validated-tenant", info.ID)
}

func TestTenantMiddleware_CustomValidator_Invalid(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.TenantValidator = func(tenantID string) (bool, error) {
		return false, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "unknown-tenant")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Nil(t, info)

	var body tenantErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body.Error.Message, "not authorized")
}

func TestTenantMiddleware_CustomValidator_Error(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.TenantValidator = func(tenantID string) (bool, error) {
		return false, fmt.Errorf("database connection failed")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "some-tenant")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Nil(t, info)

	var body tenantErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)
	assert.Contains(t, body.Error.Message, "validation error")
}

func TestTenantMiddleware_ResponseHeader(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "echo-tenant")

	rr, _ := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "echo-tenant", rr.Header().Get("X-Tenant-ID"))
}

func TestTenantMiddleware_HeaderPriorityOverQuery(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents?tenant_id=query-tenant", nil)
	req.Header.Set("X-Tenant-ID", "header-tenant")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "header-tenant", info.ID, "header should take priority over query param")
}

func TestTenantFromContext_Present(t *testing.T) {
	expected := &TenantInfo{
		ID:       "ctx-tenant",
		Name:     "Context Tenant",
		Plan:     "enterprise",
		IsActive: true,
	}
	ctx := context.WithValue(context.Background(), tenantContextKey{}, expected)

	info, ok := TenantFromContext(ctx)

	assert.True(t, ok)
	require.NotNil(t, info)
	assert.Equal(t, "ctx-tenant", info.ID)
	assert.Equal(t, "Context Tenant", info.Name)
	assert.Equal(t, "enterprise", info.Plan)
	assert.True(t, info.IsActive)
}

func TestTenantFromContext_Absent(t *testing.T) {
	ctx := context.Background()

	info, ok := TenantFromContext(ctx)

	assert.False(t, ok)
	assert.Nil(t, info)
}

func TestMustTenantFromContext_Panic(t *testing.T) {
	ctx := context.Background()

	assert.Panics(t, func() {
		MustTenantFromContext(ctx)
	}, "MustTenantFromContext should panic when no tenant info in context")
}

func TestMustTenantFromContext_Success(t *testing.T) {
	expected := &TenantInfo{
		ID:       "must-tenant",
		IsActive: true,
	}
	ctx := context.WithValue(context.Background(), tenantContextKey{}, expected)

	assert.NotPanics(t, func() {
		info := MustTenantFromContext(ctx)
		assert.Equal(t, "must-tenant", info.ID)
		assert.True(t, info.IsActive)
	})
}

func TestDefaultTenantConfig(t *testing.T) {
	cfg := DefaultTenantConfig()

	assert.Equal(t, "X-Tenant-ID", cfg.HeaderName)
	assert.Equal(t, "tenant_id", cfg.QueryParam)
	assert.Equal(t, "", cfg.DefaultTenantID)
	assert.True(t, cfg.Required)
	assert.Nil(t, cfg.AllowedTenants)
	assert.Nil(t, cfg.TenantValidator)
}

func TestValidateTenantID_ValidCases(t *testing.T) {
	validIDs := []string{
		"a",
		"tenant-1",
		"tenant_2",
		"TENANT-UPPER",
		"MixedCase_123",
		"a-b-c-d-e",
		"0123456789",
		strings.Repeat("x", 64), // exactly 64 chars — max allowed
		"my_org-prod-01",
		"T",
		"_leading-underscore",
		"-leading-hyphen",
	}

	for _, id := range validIDs {
		t.Run(fmt.Sprintf("valid=%s", id), func(t *testing.T) {
			assert.True(t, validateTenantID(id), "expected %q to be valid", id)
		})
	}
}

func TestValidateTenantID_InvalidCases(t *testing.T) {
	invalidIDs := []string{
		"",                        // empty
		strings.Repeat("a", 65),   // too long
		"tenant id",               // space
		"tenant@org",              // @
		"tenant.org",              // dot
		"tenant/path",             // slash
		"tenant\\back",            // backslash
		"tenant:colon",            // colon
		"tenant;semi",             // semicolon
		"<script>alert</script>",  // HTML injection
		"tenant\ttab",             // tab
		"tenant\nnewline",         // newline
		"日本語テナント",              // unicode
		"tenant=value",            // equals
		"tenant&other",            // ampersand
		"tenant#hash",             // hash
		"tenant%20encoded",        // percent
		"tenant+plus",             // plus
		"tenant!bang",             // exclamation
		"tenant$dollar",           // dollar
		"tenant(paren)",           // parentheses
		"tenant{brace}",           // braces
		"tenant[bracket]",         // brackets
		"tenant|pipe",             // pipe
		"tenant~tilde",            // tilde
		"tenant`backtick",         // backtick
		"tenant'quote",            // single quote
		"tenant\"dquote",          // double quote
		"tenant,comma",            // comma
		"tenant?question",         // question mark
		"tenant*star",             // asterisk
	}

	for _, id := range invalidIDs {
		t.Run(fmt.Sprintf("invalid=%s", id), func(t *testing.T) {
			assert.False(t, validateTenantID(id), "expected %q to be invalid", id)
		})
	}
}

func TestTenantMiddleware_CustomHeaderName(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.HeaderName = "X-Organization-ID"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Organization-ID", "custom-header-tenant")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "custom-header-tenant", info.ID)
}

func TestTenantMiddleware_CustomQueryParam(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.QueryParam = "org_id"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents?org_id=custom-query-tenant", nil)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "custom-query-tenant", info.ID)
}

func TestTenantMiddleware_WhitespaceTrimming(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "  trimmed-tenant  ")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "trimmed-tenant", info.ID)
}

func TestTenantMiddleware_EmptyHeaderFallsToQuery(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents?tenant_id=fallback-query", nil)
	req.Header.Set("X-Tenant-ID", "   ") // whitespace-only header

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "fallback-query", info.ID)
}

func TestTenantMiddleware_AllowedTenants_EmptyList_NoRestriction(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.AllowedTenants = []string{} // empty — no restriction
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "any-tenant")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "any-tenant", info.ID)
}

func TestTenantMiddleware_ValidatorCalledAfterWhitelist(t *testing.T) {
	validatorCalled := false
	cfg := DefaultTenantConfig()
	cfg.AllowedTenants = []string{"allowed-tenant"}
	cfg.TenantValidator = func(tenantID string) (bool, error) {
		validatorCalled = true
		return true, nil
	}

	// Tenant NOT in whitelist — validator should NOT be called.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "blocked-tenant")

	rr, _ := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.False(t, validatorCalled, "validator should not be called when whitelist rejects")

	// Tenant IN whitelist — validator should be called.
	validatorCalled = false
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req2.Header.Set("X-Tenant-ID", "allowed-tenant")

	rr2, info := executeTenantMiddleware(cfg, req2)

	assert.Equal(t, http.StatusOK, rr2.Code)
	assert.True(t, validatorCalled, "validator should be called after whitelist passes")
	require.NotNil(t, info)
	assert.Equal(t, "allowed-tenant", info.ID)
}

func TestTenantMiddleware_ErrorResponseFormat(t *testing.T) {
	cfg := DefaultTenantConfig()
	cfg.Required = true
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)

	rr, _ := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")

	var body map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&body)
	require.NoError(t, err)

	errorObj, ok := body["error"].(map[string]interface{})
	require.True(t, ok, "response should have 'error' object")
	assert.NotEmpty(t, errorObj["code"])
	assert.NotEmpty(t, errorObj["message"])
}

func TestTenantMiddleware_POSTRequest(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents", strings.NewReader(`{"title":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "post-tenant")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "post-tenant", info.ID)
}

func TestTenantMiddleware_DefaultTenantID_WithRequired(t *testing.T) {
	// When Required=true but DefaultTenantID is set and no header/query,
	// the default should be used (default is applied before Required check).
	cfg := DefaultTenantConfig()
	cfg.Required = true
	cfg.DefaultTenantID = "fallback-required"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "fallback-required", info.ID)
}

func TestTenantMiddleware_BoundaryLength_Exactly64(t *testing.T) {
	cfg := DefaultTenantConfig()
	id64 := strings.Repeat("A", 64)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", id64)

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, id64, info.ID)
}

func TestTenantMiddleware_BoundaryLength_SingleChar(t *testing.T) {
	cfg := DefaultTenantConfig()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	req.Header.Set("X-Tenant-ID", "X")

	rr, info := executeTenantMiddleware(cfg, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, info)
	assert.Equal(t, "X", info.ID)
}

//Personal.AI order the ending
