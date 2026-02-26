// Phase 11 - 接口层: HTTP Middleware - CORS 中间件单元测试
// 序号: 275
// 文件: internal/interfaces/http/middleware/cors_test.go
// 测试用例:
//   - TestCORS_PreflightRequest: OPTIONS 预检请求返回 204 + 正确头
//   - TestCORS_SimpleRequest: 简单请求设置 CORS 头
//   - TestCORS_AllowedOrigin: 允许的 origin 设置 Allow-Origin
//   - TestCORS_DisallowedOrigin: 不允许的 origin 不设置 CORS 头
//   - TestCORS_WildcardOrigin: 通配符 * 允许所有 origin
//   - TestCORS_SubdomainWildcard: *.example.com 匹配子域名
//   - TestCORS_Credentials: AllowCredentials 设置正确头
//   - TestCORS_NoOriginHeader: 无 Origin 头直接通过
//   - TestCORS_ExposedHeaders: 暴露头正确设置
//   - TestCORS_MaxAge: 预检缓存时间正确
//   - TestCORS_VaryHeader: Vary 头正确设置
//   - TestCORS_WildcardWithCredentials: 通配符 + credentials 使用具体 origin
//   - TestDefaultCORSConfig: 默认配置值验证
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestCORS_PreflightRequest(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://app.example.com"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/api/v1/patents", nil)
	r.Header.Set("Origin", "https://app.example.com")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Max-Age"))
	// Body should be empty for 204
	assert.Empty(t, w.Body.String())
}

func TestCORS_SimpleRequest(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://app.example.com"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
	r.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "ok", w.Body.String())
}

func TestCORS_AllowedOrigin(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://a.com", "https://b.com"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://b.com")
	handler.ServeHTTP(w, r)

	assert.Equal(t, "https://b.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://allowed.com"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://evil.com")
	handler.ServeHTTP(w, r)

	// Should still serve the response (browser enforces blocking)
	assert.Equal(t, http.StatusOK, w.Code)
	// But no CORS headers
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardOrigin(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"*"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://any-origin.com")
	handler.ServeHTTP(w, r)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_SubdomainWildcard(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"*.example.com"}
	config.AllowWildcard = true

	handler := CORS(config)(okHandler())

	// Matching subdomain
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(w, r)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))

	// Non-matching domain
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Origin", "https://other.com")
	handler.ServeHTTP(w2, r2)
	assert.Empty(t, w2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_Credentials(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://app.example.com"}
	config.AllowCredentials = true

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(w, r)

	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	// With credentials, origin must be specific, not *
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_NoOriginHeader(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://app.example.com"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Origin header
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_ExposedHeaders(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://app.example.com"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(w, r)

	exposed := w.Header().Get("Access-Control-Expose-Headers")
	assert.Contains(t, exposed, "X-Request-ID")
	assert.Contains(t, exposed, "X-RateLimit-Limit")
}

func TestCORS_MaxAge(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://app.example.com"}
	config.MaxAge = 3600

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/", nil)
	r.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(w, r)

	assert.Equal(t, "3600", w.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_VaryHeader(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"https://app.example.com"}

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://app.example.com")
	handler.ServeHTTP(w, r)

	vary := w.Header().Values("Vary")
	assert.Contains(t, vary, "Origin")
	assert.Contains(t, vary, "Access-Control-Request-Method")
	assert.Contains(t, vary, "Access-Control-Request-Headers")
}

func TestCORS_WildcardWithCredentials(t *testing.T) {
	config := DefaultCORSConfig()
	config.AllowedOrigins = []string{"*"}
	config.AllowCredentials = true

	handler := CORS(config)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://specific.com")
	handler.ServeHTTP(w, r)

	// When credentials + wildcard, should echo specific origin, not *
	assert.Equal(t, "https://specific.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestDefaultCORSConfig(t *testing.T) {
	config := DefaultCORSConfig()

	assert.Empty(t, config.AllowedOrigins)
	assert.Contains(t, config.AllowedMethods, http.MethodGet)
	assert.Contains(t, config.AllowedMethods, http.MethodPost)
	assert.Contains(t, config.AllowedMethods, http.MethodDelete)
	assert.Contains(t, config.AllowedHeaders, "Authorization")
	assert.Contains(t, config.AllowedHeaders, "X-API-Key")
	assert.Contains(t, config.ExposedHeaders, "X-Request-ID")
	assert.False(t, config.AllowCredentials)
	assert.Equal(t, 86400, config.MaxAge)
	assert.False(t, config.AllowWildcard)
}

//Personal.AI order the ending

