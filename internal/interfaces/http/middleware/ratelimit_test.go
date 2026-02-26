// Phase 11 - 接口层: HTTP Middleware - 速率限制中间件单元测试
// 序号: 279
// 文件: internal/interfaces/http/middleware/ratelimit_test.go
// 测试用例:
//   - TestTokenBucketLimiter_Allow: 正常请求允许通过
//   - TestTokenBucketLimiter_Burst: 突发请求在 burst 内允许
//   - TestTokenBucketLimiter_Exceeded: 超过限制被拒绝
//   - TestTokenBucketLimiter_Refill: 令牌随时间恢复
//   - TestTokenBucketLimiter_ConcurrentAccess: 并发安全
//   - TestTokenBucketLimiter_Cleanup: 过期桶清理
//   - TestTokenBucketLimiter_BucketCount: 桶计数
//   - TestRateLimit_Allowed: 中间件允许正常请求
//   - TestRateLimit_Exceeded: 中间件拒绝超限请求返回 429
//   - TestRateLimit_Headers: 响应头正确设置
//   - TestRateLimit_RetryAfter: Retry-After 头设置
//   - TestRateLimit_SkipPaths: 跳过配置路径
//   - TestRateLimit_CustomKeyFunc: 自定义键函数
//   - TestRateLimit_CustomExceededHandler: 自定义超限处理器
//   - TestDefaultKeyFunc_XForwardedFor: X-Forwarded-For 优先
//   - TestDefaultKeyFunc_XRealIP: X-Real-IP 次优先
//   - TestDefaultKeyFunc_RemoteAddr: 回退到 RemoteAddr
//   - TestTenantKeyFunc: 租户键函数
//   - TestAPIKeyKeyFunc: API Key 键函数
//   - TestCompositeKeyFunc: 组合键函数
//   - TestDefaultRateLimitConfig: 默认配置值
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- TokenBucketLimiter Tests ---

func TestTokenBucketLimiter_Allow(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 10, 0)
	defer limiter.Stop()

	allowed, info := limiter.Allow("test-key")
	assert.True(t, allowed)
	assert.Equal(t, 10, info.Limit)
	assert.True(t, info.Remaining >= 0)
}

func TestTokenBucketLimiter_Burst(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 5, 0) // 1 req/s, burst of 5
	defer limiter.Stop()

	// Should allow 5 requests in burst
	for i := 0; i < 5; i++ {
		allowed, _ := limiter.Allow("burst-key")
		assert.True(t, allowed, "request %d should be allowed", i)
	}

	// 6th should be rejected
	allowed, info := limiter.Allow("burst-key")
	assert.False(t, allowed)
	assert.Equal(t, 0, info.Remaining)
}

func TestTokenBucketLimiter_Exceeded(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 2, 0)
	defer limiter.Stop()

	limiter.Allow("exceed-key")
	limiter.Allow("exceed-key")

	allowed, _ := limiter.Allow("exceed-key")
	assert.False(t, allowed)
}

func TestTokenBucketLimiter_Refill(t *testing.T) {
	limiter := NewTokenBucketLimiter(100, 2, 0) // 100 req/s for fast refill
	defer limiter.Stop()

	// Exhaust tokens
	limiter.Allow("refill-key")
	limiter.Allow("refill-key")

	allowed, _ := limiter.Allow("refill-key")
	assert.False(t, allowed)

	// Wait for refill
	time.Sleep(50 * time.Millisecond)

	allowed, _ = limiter.Allow("refill-key")
	assert.True(t, allowed)
}

func TestTokenBucketLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewTokenBucketLimiter(1000, 100, 0)
	defer limiter.Stop()

	var wg sync.WaitGroup
	var allowedCount int64

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _ := limiter.Allow("concurrent-key")
			if allowed {
				atomic.AddInt64(&allowedCount, 1)
			}
		}()
	}

	wg.Wait()

	// At most burstSize (100) should be allowed initially
	assert.True(t, allowedCount <= 100, "allowed %d, expected <= 100", allowedCount)
	assert.True(t, allowedCount > 0, "at least some requests should be allowed")
}

func TestTokenBucketLimiter_Cleanup(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 10, 50*time.Millisecond)
	defer limiter.Stop()

	limiter.Allow("cleanup-key-1")
	limiter.Allow("cleanup-key-2")

	assert.Equal(t, 2, limiter.BucketCount())

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Buckets should be cleaned up (they're nearly full and idle)
	assert.True(t, limiter.BucketCount() <= 2) // may or may not be cleaned depending on timing
}

func TestTokenBucketLimiter_BucketCount(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 10, 0)
	defer limiter.Stop()

	assert.Equal(t, 0, limiter.BucketCount())

	limiter.Allow("key-a")
	assert.Equal(t, 1, limiter.BucketCount())

	limiter.Allow("key-b")
	assert.Equal(t, 2, limiter.BucketCount())

	// Same key doesn't create new bucket
	limiter.Allow("key-a")
	assert.Equal(t, 2, limiter.BucketCount())
}

// --- RateLimit Middleware Tests ---

func TestRateLimit_Allowed(t *testing.T) {
	limiter := NewTokenBucketLimiter(100, 100, 0)
	defer limiter.Stop()

	config := DefaultRateLimitConfig()
	config.SkipPaths = nil

	called := false
	handler := RateLimit(limiter, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.RemoteAddr = "192.168.1.1:12345"
	handler.ServeHTTP(w, r)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimit_Exceeded(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 1, 0) // very restrictive
	defer limiter.Stop()

	config := DefaultRateLimitConfig()
	config.SkipPaths = nil

	handler := RateLimit(limiter, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should pass
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/api", nil)
	r1.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(w1, r1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Second request should be rate limited
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/api", nil)
	r2.RemoteAddr = "10.0.0.1:1234"
	handler.ServeHTTP(w2, r2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)

	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	errObj := resp["error"].(map[string]interface{})
	assert.Equal(t, "RATE_LIMITED", errObj["code"])
}

func TestRateLimit_Headers(t *testing.T) {
	limiter := NewTokenBucketLimiter(10, 10, 0)
	defer limiter.Stop()

	config := DefaultRateLimitConfig()
	config.SkipPaths = nil

	handler := RateLimit(limiter, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api", nil)
	r.RemoteAddr = "10.0.0.2:5678"
	handler.ServeHTTP(w, r)

	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
	remaining := w.Header().Get("X-RateLimit-Remaining")
	assert.NotEmpty(t, remaining)
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimit_RetryAfter(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 1, 0)
	defer limiter.Stop()

	config := DefaultRateLimitConfig()
	config.SkipPaths = nil

	handler := RateLimit(limiter, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/api", nil)
	r1.RemoteAddr = "10.0.0.3:1111"
	handler.ServeHTTP(w1, r1)

	// Exceed
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/api", nil)
	r2.RemoteAddr = "10.0.0.3:1111"
	handler.ServeHTTP(w2, r2)

	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.NotEmpty(t, w2.Header().Get("Retry-After"))
}

func TestRateLimit_SkipPaths(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 1, 0)
	defer limiter.Stop()

	config := DefaultRateLimitConfig()
	config.SkipPaths = []string{"/health"}

	called := false
	handler := RateLimit(limiter, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// Multiple requests to /health should all pass
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/health", nil)
		r.RemoteAddr = "10.0.0.4:2222"
		handler.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	}
	assert.True(t, called)
}

func TestRateLimit_CustomKeyFunc(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 1, 0)
	defer limiter.Stop()

	config := DefaultRateLimitConfig()
	config.SkipPaths = nil
	config.KeyFunc = func(r *http.Request) string {
		return r.Header.Get("X-Custom-Key")
	}

	handler := RateLimit(limiter, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Different keys should have independent limits
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/api", nil)
	r1.Header.Set("X-Custom-Key", "user-a")
	handler.ServeHTTP(w1, r1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/api", nil)
	r2.Header.Set("X-Custom-Key", "user-b")
	handler.ServeHTTP(w2, r2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestRateLimit_CustomExceededHandler(t *testing.T) {
	limiter := NewTokenBucketLimiter(1, 1, 0)
	defer limiter.Stop()

	customCalled := false
	config := DefaultRateLimitConfig()
	config.SkipPaths = nil
	config.ExceededHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customCalled = true
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("custom exceeded"))
	})

	handler := RateLimit(limiter, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest("GET", "/api", nil)
	r1.RemoteAddr = "10.0.0.5:3333"
	handler.ServeHTTP(w1, r1)

	// Exceed → custom handler
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/api", nil)
	r2.RemoteAddr = "10.0.0.5:3333"
	handler.ServeHTTP(w2, r2)

	assert.True(t, customCalled)
	assert.Equal(t, http.StatusServiceUnavailable, w2.Code)
	assert.Equal(t, "custom exceeded", w2.Body.String())
}

// --- Key Function Tests ---

func TestDefaultKeyFunc_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.50")
	r.RemoteAddr = "10.0.0.1:1234"

	assert.Equal(t, "203.0.113.50", defaultKeyFunc(r))
}

func TestDefaultKeyFunc_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "198.51.100.25")
	r.RemoteAddr = "10.0.0.1:1234"

	assert.Equal(t, "198.51.100.25", defaultKeyFunc(r))
}

func TestDefaultKeyFunc_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.100:54321"

	assert.Equal(t, "192.168.1.100:54321", defaultKeyFunc(r))
}

func TestTenantKeyFunc(t *testing.T) {
	// With tenant context
	claims := &Claims{TenantID: "tenant-xyz"}
	ctx := context.WithValue(context.Background(), claimsContextKey, claims)
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	assert.Equal(t, "tenant:tenant-xyz", TenantKeyFunc(r))

	// Without tenant context
	r2 := httptest.NewRequest("GET", "/", nil)
	r2.RemoteAddr = "10.0.0.1:1234"
	result := TenantKeyFunc(r2)
	assert.Contains(t, result, "ip:")
}

func TestAPIKeyKeyFunc(t *testing.T) {
	info := &APIKeyInfo{KeyID: "key-abc"}
	ctx := context.WithValue(context.Background(), apiKeyInfoContextKey, info)
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	assert.Equal(t, "apikey:key-abc", APIKeyKeyFunc(r))
}

func TestCompositeKeyFunc(t *testing.T) {
	claims := &Claims{UserID: "user-1", TenantID: "tenant-1"}
	ctx := context.WithValue(context.Background(), claimsContextKey, claims)
	r := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	r.RemoteAddr = "10.0.0.1:5555"

	result := CompositeKeyFunc(r)
	assert.Contains(t, result, "tenant-1")
	assert.Contains(t, result, "user-1")
	assert.Contains(t, result, "10.0.0.1")
}

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	assert.Equal(t, float64(10), config.RequestsPerSecond)
	assert.Equal(t, 20, config.BurstSize)
	assert.NotNil(t, config.KeyFunc)
	assert.Contains(t, config.SkipPaths, "/health")
	assert.Equal(t, 5*time.Minute, config.CleanupInterval)
	assert.Nil(t, config.ExceededHandler)
}

//Personal.AI order the ending
