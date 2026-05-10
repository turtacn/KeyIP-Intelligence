// Phase 11 - 接口层: HTTP Middleware - 响应缓存中间件测试
// 序号: 286
// 文件: internal/interfaces/http/middleware/cache_test.go
// 功能定位: 验证 HTTP 响应缓存中间件的正确性，包括:
//   - Cache-Control 和 ETag 响应头设置
//   - If-None-Match / 304 Not Modified 条件请求
//   - 仅 GET 请求生效，非 GET 方法透传
//   - 缓存命中/未命中/过期行为
//   - 自定义 TTL 和可缓存状态码配置
//   - MemoryCache 存储实现单元测试
//   - 中间件 struct 模式遵循
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCacheHandler returns a simple HTTP handler that writes a 200 JSON response
// with the given body.
func testCacheHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
}

// testCacheHandlerWithStatus returns a handler that writes the given status code
// and body.
func testCacheHandlerWithStatus(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// --- Cache-Control Header Tests ---

// TestCache_SetsCacheControlHeader verifies that the middleware sets a default
// Cache-Control header with public and max-age directives matching the configured TTL.
func TestCache_SetsCacheControlHeader(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.DefaultTTL = 10 * time.Second

	handler := Cache(cfg)(testCacheHandler(`{"hello":"world"}`))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Contains(t, rec.Header().Get("Cache-Control"), "public",
		"Cache-Control should include public directive")
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=10",
		"Cache-Control should include max-age matching DefaultTTL")
}

// TestCache_CacheControlRespectsHandler verifies that when the downstream handler
// already sets a Cache-Control header, the middleware preserves it rather than
// overriding it.
func TestCache_CacheControlRespectsHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "private, max-age=30")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	cfg := DefaultCacheConfig()
	cfg.DefaultTTL = 5 * time.Minute
	mw := Cache(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	assert.Equal(t, "private, max-age=30", rec.Header().Get("Cache-Control"),
		"handler-set Cache-Control should be preserved")
}

// --- ETag Header Tests ---

// TestCache_SetsETagHeader verifies that the middleware computes an MD5-based
// ETag and sets it on the response.
func TestCache_SetsETagHeader(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"id":1,"name":"test"}`))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	etag := rec.Header().Get("ETag")
	assert.NotEmpty(t, etag, "ETag header should be set")
	assert.True(t, strings.HasPrefix(etag, `"`), "ETag should start with a double quote")
	assert.True(t, strings.HasSuffix(etag, `"`), "ETag should end with a double quote")
	// MD5 hex is 32 characters + 2 quotes = 34
	assert.Len(t, etag, 34, "ETag should be 34 characters (32 hex + 2 quotes)")

	// Verify ETag is deterministic for the same body
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, etag, rec2.Header().Get("ETag"),
		"ETag should be deterministic for the same response body")
}

// TestCache_ETagChangesWithBody verifies that different response bodies produce
// different ETag values.
func TestCache_ETagChangesWithBody(t *testing.T) {
	cfg := DefaultCacheConfig()

	handlerA := Cache(cfg)(testCacheHandler(`{"data":"A"}`))
	handlerB := Cache(cfg)(testCacheHandler(`{"data":"B"}`))

	recA := httptest.NewRecorder()
	handlerA.ServeHTTP(recA, httptest.NewRequest(http.MethodGet, "/a", nil))

	recB := httptest.NewRecorder()
	handlerB.ServeHTTP(recB, httptest.NewRequest(http.MethodGet, "/b", nil))

	assert.NotEqual(t, recA.Header().Get("ETag"), recB.Header().Get("ETag"),
		"different response bodies should produce different ETags")
}

// --- If-None-Match / 304 Tests ---

// TestCache_IfNoneMatchReturns304 verifies that sending If-None-Match with the
// cached ETag returns 304 Not Modified with an empty body.
func TestCache_IfNoneMatchReturns304(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	// First request to populate the cache
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")
	require.NotEmpty(t, etag, "first response must have an ETag")

	// Second request with matching If-None-Match
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotModified, rec2.Code,
		"matching If-None-Match should return 304")
	assert.Empty(t, rec2.Body.String(), "304 response should have an empty body")
	assert.Equal(t, etag, rec2.Header().Get("ETag"),
		"304 response should include the ETag header")
}

// TestCache_IfNoneMatchStarReturns304 verifies that If-None-Match: * returns
// 304 Not Modified when there is a cached entry.
func TestCache_IfNoneMatchStarReturns304(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	// First request to populate the cache
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	require.NotEmpty(t, rec1.Header().Get("ETag"), "first response must have an ETag")

	// Second request with wildcard If-None-Match
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", "*")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotModified, rec2.Code,
		"If-None-Match: * should return 304 when cache entry exists")
}

// TestCache_IfNoneMatchNoMatchReturnsFullResponse verifies that sending
// If-None-Match with a non-matching ETag returns the full 200 response.
func TestCache_IfNoneMatchNoMatchReturnsFullResponse(t *testing.T) {
	cfg := DefaultCacheConfig()
	body := `{"status":"ok"}`
	handler := Cache(cfg)(testCacheHandler(body))

	// First request to populate the cache
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	require.NotEmpty(t, rec1.Header().Get("ETag"), "first response must have an ETag")

	// Second request with non-matching ETag
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", `"non-matching-etag-value"`)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code,
		"non-matching If-None-Match should return 200")
	assert.Equal(t, body, rec2.Body.String(),
		"non-matching If-None-Match should return the full body")
}

// TestCache_IfNoneMatchWeak verifies that a weak ETag (W/"etag") in the
// If-None-Match header is correctly matched against a strong ETag.
func TestCache_IfNoneMatchWeak(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	// First request to populate the cache
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")
	require.NotEmpty(t, etag, "first response must have an ETag")

	// Request with weak ETag comparator
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", "W/"+etag)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotModified, rec2.Code,
		"weak ETag W/"+etag+" should match strong ETag "+etag)
}

// TestCache_IfNoneMatchMultipleValues verifies that If-None-Match with multiple
// comma-separated ETags works correctly (matching the second value).
func TestCache_IfNoneMatchMultipleValues(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	// First request to populate the cache
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	etag := rec1.Header().Get("ETag")
	require.NotEmpty(t, etag, "first response must have an ETag")

	// Request with multiple ETags, one matching
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", `"etag1", `+etag+`, "etag3"`)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.Equal(t, http.StatusNotModified, rec2.Code,
		"should match when one of multiple If-None-Match values matches")
}

// --- Method Filtering Tests ---

// TestCache_OnlyAffectsGET verifies that POST requests pass through without
// any caching headers.
func TestCache_OnlyAffectsGET(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"),
		"POST should not have Cache-Control")
	assert.Empty(t, rec.Header().Get("ETag"),
		"POST should not have ETag")
}

// TestCache_PUTPassesThrough verifies that PUT requests pass through without
// any caching headers.
func TestCache_PUTPassesThrough(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"),
		"PUT should not have Cache-Control")
	assert.Empty(t, rec.Header().Get("ETag"),
		"PUT should not have ETag")
}

// TestCache_DELETEPassesThrough verifies that DELETE requests pass through.
func TestCache_DELETEPassesThrough(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	req := httptest.NewRequest(http.MethodDelete, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
	assert.Empty(t, rec.Header().Get("ETag"))
}

// TestCache_PATCHPassesThrough verifies that PATCH requests pass through.
func TestCache_PATCHPassesThrough(t *testing.T) {
	cfg := DefaultCacheConfig()
	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	req := httptest.NewRequest(http.MethodPatch, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Cache-Control"))
	assert.Empty(t, rec.Header().Get("ETag"))
}

// TestCache_HEADGetsCacheHeaders verifies that HEAD requests receive caching
// headers when configured as cacheable.
func TestCache_HEADGetsCacheHeaders(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.CacheableMethods = []string{http.MethodGet, http.MethodHead}
	handler := Cache(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodHead, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("Cache-Control"),
		"HEAD should have Cache-Control when configured")
	assert.NotEmpty(t, rec.Header().Get("ETag"),
		"HEAD should have ETag when configured")
}

// --- Status Code Caching Tests ---

// TestCache_NonCacheableStatusStillSetsHeaders verifies that non-cacheable
// status codes still receive Cache-Control and ETag headers but are not stored
// server-side.
func TestCache_NonCacheableStatusStillSetsHeaders(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.CacheableStatuses = []int{http.StatusOK}

	handler := Cache(cfg)(testCacheHandlerWithStatus(http.StatusNotFound, `{"error":"not found"}`))

	// Use a MemoryCache directly to verify no server-side caching
	memCache := NewMemoryCache(cfg.DefaultTTL)
	cfg.CacheStore = memCache

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	Cache(cfg)(handler).ServeHTTP(rec, req)

	// Headers should still be set on non-cacheable status codes
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Cache-Control"),
		"non-cacheable status should still get Cache-Control")
	assert.NotEmpty(t, rec.Header().Get("ETag"),
		"non-cacheable status should still get ETag")

	// But the response should not be stored in the server-side cache
	_, found := memCache.Get("/test")
	assert.False(t, found, "non-cacheable status should not be stored in cache")
}

// TestCache_CacheableStatusIsCached verifies that cacheable status codes are
// stored in the server-side cache.
func TestCache_CacheableStatusIsCached(t *testing.T) {
	cfg := DefaultCacheConfig()
	memCache := NewMemoryCache(cfg.DefaultTTL)
	cfg.CacheStore = memCache

	handler := Cache(cfg)(testCacheHandler(`{"status":"ok"}`))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should be stored in server-side cache
	entry, found := memCache.Get("/test")
	assert.True(t, found, "200 response should be stored in cache")
	require.NotNil(t, entry)
	assert.Contains(t, string(entry.body), `"status":"ok"`)
}

// --- Cache Hit / Miss Tests ---

// TestCache_CacheHitServesCachedResponse verifies that a second request to the
// same URL returns the cached response without invoking the downstream handler.
func TestCache_CacheHitServesCachedResponse(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"count":%d}`, callCount)))
	})

	cfg := DefaultCacheConfig()
	mw := Cache(cfg)

	// First request: cache miss, handler called
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec1 := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec1, req1)
	assert.Equal(t, 1, callCount, "handler should be called on first request")
	assert.Equal(t, `{"count":1}`, rec1.Body.String())

	// Second request: cache hit, handler NOT called
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec2, req2)
	assert.Equal(t, 1, callCount, "handler should NOT be called on cache hit")
	assert.Equal(t, `{"count":1}`, rec2.Body.String(),
		"cached response should preserve the original body")
}

// TestCache_DifferentURLsHaveDifferentCache verifies that different URLs have
// separate cache entries and do not interfere.
func TestCache_DifferentURLsHaveDifferentCache(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"path":"%s"}`, r.URL.Path)))
	})

	cfg := DefaultCacheConfig()
	mw := Cache(cfg)
	wrapped := mw(handler)

	// Request to /a
	reqA := httptest.NewRequest(http.MethodGet, "/a", nil)
	recA := httptest.NewRecorder()
	wrapped.ServeHTTP(recA, reqA)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, `{"path":"/a"}`, recA.Body.String())

	// Request to /b (different URL)
	reqB := httptest.NewRequest(http.MethodGet, "/b", nil)
	recB := httptest.NewRecorder()
	wrapped.ServeHTTP(recB, reqB)
	assert.Equal(t, 2, callCount)

	// Repeat /a - should be served from cache
	reqA2 := httptest.NewRequest(http.MethodGet, "/a", nil)
	recA2 := httptest.NewRecorder()
	wrapped.ServeHTTP(recA2, reqA2)
	assert.Equal(t, 2, callCount, "should use cached response for /a")
	assert.Equal(t, `{"path":"/a"}`, recA2.Body.String())
}

// TestCache_QueryParametersAreCachedSeparately verifies that different query
// parameters produce different cache keys.
func TestCache_QueryParametersAreCachedSeparately(t *testing.T) {
	cfg := DefaultCacheConfig()
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"q":"%s"}`, r.URL.Query().Get("q"))))
	})

	mw := Cache(cfg)
	wrapped := mw(handler)

	// Request with q=hello
	req1 := httptest.NewRequest(http.MethodGet, "/search?q=hello", nil)
	rec1 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec1, req1)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, `{"q":"hello"}`, rec1.Body.String())

	// Same query - should be cached
	req2 := httptest.NewRequest(http.MethodGet, "/search?q=hello", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)
	assert.Equal(t, 1, callCount, "same query should be cached")

	// Different query - should NOT be cached
	req3 := httptest.NewRequest(http.MethodGet, "/search?q=world", nil)
	rec3 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec3, req3)
	assert.Equal(t, 2, callCount)
	assert.Equal(t, `{"q":"world"}`, rec3.Body.String())
}

// TestCache_CacheMissForDifferentURL verifies that a request to a URL that has
// not been cached yet results in a cache miss and handler invocation.
func TestCache_CacheMissForDifferentURL(t *testing.T) {
	cfg := DefaultCacheConfig()
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mw := Cache(cfg)
	wrapped := mw(handler)

	// Cache /a
	reqA := httptest.NewRequest(http.MethodGet, "/a", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), reqA)
	assert.Equal(t, 1, callCount)

	// Request /b - different URL, cache miss
	reqB := httptest.NewRequest(http.MethodGet, "/b", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), reqB)
	assert.Equal(t, 2, callCount, "different URL should cause cache miss")
}

// --- Cache Expiration Tests ---

// TestCache_CacheExpiration verifies that after the TTL expires, the next
// request results in a cache miss and the handler is called again.
func TestCache_CacheExpiration(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.DefaultTTL = 50 * time.Millisecond

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"count":%d}`, callCount)))
	})

	mw := Cache(cfg)
	wrapped := mw(handler)

	// First request
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), req1)
	assert.Equal(t, 1, callCount)

	// Wait for cache to expire
	time.Sleep(80 * time.Millisecond)

	// Request after expiration - handler should be called again
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), req2)
	assert.Equal(t, 2, callCount, "handler should be called again after cache expiry")
}

// --- Vary Header Tests ---

// TestCache_VaryHeaderPreserved verifies that Vary headers set by the handler
// are preserved in the cached response.
func TestCache_VaryHeaderPreserved(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	cfg := DefaultCacheConfig()
	mw := Cache(cfg)
	wrapped := mw(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Vary should be preserved from handler
	vary := rec.Header().Get("Vary")
	assert.Contains(t, vary, "Accept-Encoding",
		"Vary header from handler should be preserved")

	// On cache hit, Vary should also be present
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)
	assert.Contains(t, rec2.Header().Get("Vary"), "Accept-Encoding",
		"Vary header should be present on cache hit")
}

// --- Content-Type Preservation Tests ---

// TestCache_ContentTypePreserved verifies that the Content-Type header set by
// the handler is correctly preserved in the cached response.
func TestCache_ContentTypePreserved(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<root><item/></root>"))
	})

	cfg := DefaultCacheConfig()
	mw := Cache(cfg)
	wrapped := mw(handler)

	// First request (cache miss)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	contentType := rec.Header().Get("Content-Type")
	assert.Equal(t, "application/xml", contentType,
		"Content-Type should be preserved from handler")

	// On cache hit, Content-Type should also match
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)
	assert.Equal(t, "application/xml", rec2.Header().Get("Content-Type"),
		"Content-Type should be preserved on cache hit")
}

// --- Custom Cacheable Methods Tests ---

// TestCache_CustomCacheableMethods verifies that the middleware caches methods
// beyond GET when configured.
func TestCache_CustomCacheableMethods(t *testing.T) {
	cfg := DefaultCacheConfig()
	cfg.CacheableMethods = []string{http.MethodGet, http.MethodPost}

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mw := Cache(cfg)
	wrapped := mw(handler)

	// First POST
	req1 := httptest.NewRequest(http.MethodPost, "/test", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), req1)
	assert.Equal(t, 1, callCount)

	// Second POST should be cached
	req2 := httptest.NewRequest(http.MethodPost, "/test", nil)
	wrapped.ServeHTTP(httptest.NewRecorder(), req2)
	assert.Equal(t, 1, callCount, "POST should be cached when configured")
}

// --- Empty Body Tests ---

// TestCache_EmptyBody verifies that responses with an empty body still get
// caching headers.
func TestCache_EmptyBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	cfg := DefaultCacheConfig()
	cfg.CacheableStatuses = []int{http.StatusNoContent}
	mw := Cache(cfg)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	mw(handler).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Cache-Control"),
		"empty body response should still get Cache-Control")
	assert.NotEmpty(t, rec.Header().Get("ETag"),
		"empty body response should still get ETag")
}

// --- CacheMiddleware Struct Pattern Tests ---

// TestCacheMiddlewareStruct verifies that the CacheMiddleware struct follows
// the established middleware pattern with New*Middleware and Handler().
func TestCacheMiddlewareStruct(t *testing.T) {
	cfg := DefaultCacheConfig()
	mw := NewCacheMiddleware(cfg)
	require.NotNil(t, mw, "NewCacheMiddleware should return a non-nil instance")

	handler := mw.Handler(testCacheHandler(`{"status":"ok"}`))
	require.NotNil(t, handler, "Handler() should return a non-nil handler")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("ETag"),
		"CacheMiddleware.Handler should set ETag")
	assert.NotEmpty(t, rec.Header().Get("Cache-Control"),
		"CacheMiddleware.Handler should set Cache-Control")
}

// TestNewCacheMiddleware_DefaultConfig verifies that NewCacheMiddleware works
// with the default configuration.
func TestNewCacheMiddleware_DefaultConfig(t *testing.T) {
	mw := NewCacheMiddleware(DefaultCacheConfig())
	require.NotNil(t, mw)

	handler := mw.Handler(testCacheHandler(`{"data":"value"}`))
	require.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Cache-Control"), "public")
	assert.Contains(t, rec.Header().Get("Cache-Control"), "max-age=300")
	assert.Len(t, rec.Header().Get("ETag"), 34)
}

// --- MemoryCache Unit Tests ---

// TestMemoryCache_GetSet verifies basic Get and Set operations.
func TestMemoryCache_GetSet(t *testing.T) {
	cache := NewMemoryCache(time.Minute)
	assert.Equal(t, 0, cache.Len(), "new cache should be empty")

	entry := &cacheEntry{
		etag:       `"abc123"`,
		statusCode: http.StatusOK,
		body:       []byte(`{"status":"ok"}`),
		createdAt:  time.Now(),
	}

	cache.Set("/key", entry)
	assert.Equal(t, 1, cache.Len())

	// Get should return the entry
	got, found := cache.Get("/key")
	assert.True(t, found)
	require.NotNil(t, got)
	assert.Equal(t, entry.etag, got.etag)
	assert.Equal(t, entry.statusCode, got.statusCode)
	assert.Equal(t, entry.body, got.body)

	// Get non-existent key
	_, found = cache.Get("/nonexistent")
	assert.False(t, found)
}

// TestMemoryCache_Delete verifies Delete removes entries.
func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(time.Minute)
	cache.Set("/key", &cacheEntry{etag: `"abc"`, createdAt: time.Now()})
	assert.Equal(t, 1, cache.Len())

	cache.Delete("/key")
	assert.Equal(t, 0, cache.Len())

	_, found := cache.Get("/key")
	assert.False(t, found)
}

// TestMemoryCache_Clear verifies Clear removes all entries.
func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(time.Minute)
	cache.Set("/a", &cacheEntry{etag: `"a"`, createdAt: time.Now()})
	cache.Set("/b", &cacheEntry{etag: `"b"`, createdAt: time.Now()})
	cache.Set("/c", &cacheEntry{etag: `"c"`, createdAt: time.Now()})
	assert.Equal(t, 3, cache.Len())

	cache.Clear()
	assert.Equal(t, 0, cache.Len())

	_, found := cache.Get("/a")
	assert.False(t, found)
}

// TestMemoryCache_Expiration verifies that Get returns false for expired entries.
func TestMemoryCache_Expiration(t *testing.T) {
	ttl := 50 * time.Millisecond
	cache := NewMemoryCache(ttl)

	cache.Set("/key", &cacheEntry{
		etag:      `"expired"`,
		createdAt: time.Now(),
	})

	// Should be found immediately
	_, found := cache.Get("/key")
	assert.True(t, found)

	// Wait for expiration
	time.Sleep(80 * time.Millisecond)

	_, found = cache.Get("/key")
	assert.False(t, found, "entry should be expired")
}

// TestMemoryCache_ConcurrentAccess verifies the cache handles concurrent
// read and write operations without race conditions.
func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	cache := NewMemoryCache(time.Minute)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := fmt.Sprintf("/key/%d", idx)
			cache.Set(key, &cacheEntry{
				etag:      fmt.Sprintf(`"etag-%d"`, idx),
				createdAt: time.Now(),
			})
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := fmt.Sprintf("/key/%d", idx)
			cache.Get(key)
		}(i)
	}

	// Wait for goroutines to finish
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 10, cache.Len(), "all 10 entries should be stored")
}

// TestCache_RevalidateAfterETagChange verifies that if the handler produces
// different content (e.g., due to a data change), the old cache entry is
// invalidated and a new one is created.
func TestCache_RevalidateAfterETagChange(t *testing.T) {
	cfg := DefaultCacheConfig()

	// Use a custom cache store with a very short TTL so old entries expire
	cache := NewMemoryCache(500 * time.Millisecond)
	cfg.CacheStore = cache

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"version":%d}`, callCount)))
	})

	mw := Cache(cfg)
	wrapped := mw(handler)

	// First request - stores version 1
	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, 1, callCount)

	// Second request - should be cached (version 1 served)
	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, 1, callCount)

	// Wait for cache expiry
	time.Sleep(600 * time.Millisecond)

	// Third request - cache expired, handler called again with version 2
	rec3 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/test", nil))
	assert.Equal(t, 2, callCount, "handler should be called after cache expiry")
	assert.Equal(t, `{"version":2}`, rec3.Body.String(),
		"new version should be served after cache expiry")
}

// --- ETag Key Tests ---

// TestGenerateETag verifies the ETag generation utility.
func TestGenerateETag(t *testing.T) {
	// Known MD5 hash for "hello" = 5d41402abc4b2a76b9719d911017c592
	etag := generateETag([]byte("hello"))
	assert.Equal(t, `"5d41402abc4b2a76b9719d911017c592"`, etag)

	// Consistency
	assert.Equal(t, generateETag([]byte("hello")), generateETag([]byte("hello")))

	// Different inputs produce different ETags
	assert.NotEqual(t, generateETag([]byte("hello")), generateETag([]byte("world")))

	// Empty input
	assert.NotEmpty(t, generateETag(nil))
	assert.NotEmpty(t, generateETag([]byte{}))
}

// TestMatchETag verifies the ETag matching utility.
func TestMatchETag(t *testing.T) {
	etag := `"abc123"`

	// Exact match
	assert.True(t, matchETag(etag, etag))

	// Wildcard
	assert.True(t, matchETag("*", etag))

	// Weak ETag match
	assert.True(t, matchETag("W/"+etag, etag))

	// No match
	assert.False(t, matchETag(`"xyz789"`, etag))

	// Multiple values - match
	assert.True(t, matchETag(`"xyz789", `+etag+`, "def456"`, etag))

	// Multiple values - no match
	assert.False(t, matchETag(`"xyz789", "def456"`, etag))

	// Empty header
	assert.False(t, matchETag("", etag))

	// Whitespace handling
	assert.True(t, matchETag("  "+etag+"  ", etag))

	// Multiple weak ETags
	assert.True(t, matchETag("W/\"weak1\", W/\"weak2\"", `"weak2"`))
}

//Personal.AI order the ending
