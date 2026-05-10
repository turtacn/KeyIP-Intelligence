// Phase 11 - 接口层: HTTP Middleware - 安全响应头中间件单元测试
// 序号: 276
// 文件: internal/interfaces/http/middleware/security_test.go
// 测试用例:
//   - TestSecurityHeaders_DefaultHeaders: 默认配置设置所有安全头
//   - TestSecurityHeaders_CSP: CSP 头正确设置
//   - TestSecurityHeaders_XContentTypeOptions: X-Content-Type-Options 头正确设置
//   - TestSecurityHeaders_XFrameOptions: X-Frame-Options 头正确设置
//   - TestSecurityHeaders_XXSSProtection: X-XSS-Protection 头正确设置
//   - TestSecurityHeaders_HSTS: HSTS 头正确设置
//   - TestSecurityHeaders_ReferrerPolicy: Referrer-Policy 头正确设置
//   - TestSecurityHeaders_PermissionsPolicy: Permissions-Policy 头正确设置
//   - TestSecurityHeaders_CustomConfig: 自定义配置正确覆盖默认值
//   - TestSecurityHeaders_NonEmptyBody: 下游 handler 正常执行
//   - TestSecurityHeaders_ResponseWriterWrapper: 响应写入不受影响
//   - TestDefaultSecurityConfig: 默认配置值验证
//   - TestDefaultCSP: 默认 CSP 策略值验证
//   - TestSecurityHeadersMiddleware_WrapperNilConfig: 传 nil 使用默认配置
//   - TestSecurityHeadersMiddleware_WrapperCustomConfig: 传自定义配置
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders_DefaultHeaders(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
	assert.NotEmpty(t, w.Header().Get("Permissions-Policy"))
}

func TestSecurityHeaders_CSP(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	csp := w.Header().Get("Content-Security-Policy")
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "script-src 'self'")
	assert.Contains(t, csp, "style-src 'self'")
	assert.Contains(t, csp, "img-src 'self'")
	assert.Contains(t, csp, "font-src 'self'")
	assert.Contains(t, csp, "connect-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Contains(t, csp, "form-action 'self'")
	assert.Contains(t, csp, "base-uri 'self'")
}

func TestSecurityHeaders_XContentTypeOptions(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
}

func TestSecurityHeaders_XFrameOptions(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestSecurityHeaders_XXSSProtection(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
}

func TestSecurityHeaders_HSTS(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_ReferrerPolicy(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
}

func TestSecurityHeaders_PermissionsPolicy(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	pp := w.Header().Get("Permissions-Policy")
	assert.Contains(t, pp, "camera=()")
	assert.Contains(t, pp, "microphone=()")
	assert.Contains(t, pp, "geolocation=()")
}

func TestSecurityHeaders_CustomConfig(t *testing.T) {
	cfg := SecurityConfig{
		ContentSecurityPolicy:   "default-src 'none'",
		XContentTypeOptions:     "nosniff",
		XFrameOptions:           "SAMEORIGIN",
		XXSSProtection:          "0",
		StrictTransportSecurity: "max-age=63072000",
		ReferrerPolicy:          "no-referrer",
		PermissionsPolicy:       "camera=()",
	}

	handler := SecurityHeaders(cfg)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "default-src 'none'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "0", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "max-age=63072000", w.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=()", w.Header().Get("Permissions-Policy"))

	// Body should still be written by downstream handler
	assert.Equal(t, "ok", w.Body.String())
}

func TestSecurityHeaders_NonEmptyBody(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestSecurityHeaders_ResponseWriterWrapper(t *testing.T) {
	// Verify the ResponseWriter is not wrapped in a way that breaks standard usage
	handler := SecurityHeaders(DefaultSecurityConfig())(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("custom body"))
		},
	))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/molecules", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "custom body", w.Body.String())

	// Security headers should still be set
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

func TestDefaultSecurityConfig(t *testing.T) {
	cfg := DefaultSecurityConfig()

	assert.NotEmpty(t, cfg.ContentSecurityPolicy)
	assert.Equal(t, "nosniff", cfg.XContentTypeOptions)
	assert.Equal(t, "DENY", cfg.XFrameOptions)
	assert.Equal(t, "1; mode=block", cfg.XXSSProtection)
	assert.Equal(t, "max-age=31536000; includeSubDomains", cfg.StrictTransportSecurity)
	assert.Equal(t, "strict-origin-when-cross-origin", cfg.ReferrerPolicy)
	assert.NotEmpty(t, cfg.PermissionsPolicy)
}

func TestDefaultCSP(t *testing.T) {
	csp := DefaultCSP()

	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "script-src 'self'")
	assert.Contains(t, csp, "style-src 'self'")
	assert.Contains(t, csp, "img-src 'self'")
	assert.Contains(t, csp, "font-src 'self'")
	assert.Contains(t, csp, "connect-src 'self'")
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Contains(t, csp, "form-action 'self'")
	assert.Contains(t, csp, "base-uri 'self'")

	// Verify CDN entries are included
	assert.Contains(t, csp, "cdn.jsdelivr.net")
	assert.Contains(t, csp, "fonts.googleapis.com")
	assert.Contains(t, csp, "fonts.gstatic.com")
}

func TestDefaultPermissionsPolicy(t *testing.T) {
	pp := DefaultPermissionsPolicy()

	assert.Contains(t, pp, "camera=()")
	assert.Contains(t, pp, "microphone=()")
	assert.Contains(t, pp, "geolocation=()")
}

func TestSecurityHeadersMiddleware_WrapperNilConfig(t *testing.T) {
	mw := NewSecurityHeadersMiddleware(nil)
	require.NotNil(t, mw)

	handler := mw.Handler(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	// Should use defaults
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
	assert.NotEmpty(t, w.Header().Get("Permissions-Policy"))
}

func TestSecurityHeadersMiddleware_WrapperCustomConfig(t *testing.T) {
	cfg := &SecurityConfig{
		ContentSecurityPolicy: "default-src 'none'",
		XFrameOptions:         "SAMEORIGIN",
	}
	mw := NewSecurityHeadersMiddleware(cfg)
	require.NotNil(t, mw)

	handler := mw.Handler(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "default-src 'none'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_EmptyConfigDisablesHeaders(t *testing.T) {
	cfg := SecurityConfig{}
	handler := SecurityHeaders(cfg)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	// All header values should be empty since zero-value SecurityConfig
	// has empty strings for all fields (default CSP, PP, etc. are computed).
	assert.Empty(t, w.Header().Get("Content-Security-Policy"))
	assert.Empty(t, w.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, w.Header().Get("X-Frame-Options"))
	assert.Empty(t, w.Header().Get("X-XSS-Protection"))
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
	assert.Empty(t, w.Header().Get("Referrer-Policy"))
	assert.Empty(t, w.Header().Get("Permissions-Policy"))

	// Downstream handler still works
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestSecurityHeaders_CSPinFrameAncestors(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	csp := w.Header().Get("Content-Security-Policy")
	// frame-ancestors 'none' combined with X-Frame-Options: DENY provides
	// defense in depth against clickjacking
	assert.Contains(t, csp, "frame-ancestors 'none'")
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
}

// Ensure all standard HTTP methods get the security headers.
func TestSecurityHeaders_AllMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodOptions,
		http.MethodHead,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

			w := httptest.NewRecorder()
			r := httptest.NewRequest(method, "/", nil)
			handler.ServeHTTP(w, r)

			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"),
				"X-Content-Type-Options should be set for %s requests", method)
			assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"),
				"X-Frame-Options should be set for %s requests", method)
		})
	}
}

// Verify CSP header is syntactically valid (semicolons separate directives).
func TestSecurityHeaders_CSPFormat(t *testing.T) {
	handler := SecurityHeaders(DefaultSecurityConfig())(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	csp := w.Header().Get("Content-Security-Policy")
	directives := strings.Split(csp, ";")
	require.GreaterOrEqual(t, len(directives), 5, "CSP should have multiple directives")

	for _, d := range directives {
		trimmed := strings.TrimSpace(d)
		assert.NotEmpty(t, trimmed, "CSP directive should not be empty")
	}
}

//Personal.AI order the ending
