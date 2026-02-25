// Phase 11 - 接口层: HTTP Middleware - 速率限制中间件
// 序号: 278
// 文件: internal/interfaces/http/middleware/ratelimit.go
// 功能定位: 实现 HTTP 请求速率限制中间件，支持基于 IP、API Key、Tenant 的多维度限流
// 核心实现:
//   - 定义 RateLimitConfig 结构体: RequestsPerSecond, BurstSize, KeyFunc, SkipPaths, ExceededHandler
//   - 定义 RateLimiter 接口: Allow(key string) (bool, RateLimitInfo)
//   - 定义 RateLimitInfo 结构体: Limit, Remaining, ResetAt
//   - 定义 TokenBucketLimiter 实现: 基于令牌桶算法的内存限流器
//   - 实现 RateLimit(limiter, config) func(http.Handler) http.Handler
//   - 限流键提取策略: IP → API Key → Tenant ID → 组合键
//   - 响应头设置: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
//   - 超限时返回 429 Too Many Requests + Retry-After 头
//   - 支持过期清理，防止内存泄漏
// 依赖关系:
//   - 依赖: internal/infrastructure/monitoring/logging
//   - 被依赖: internal/interfaces/http/router.go
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)


// RateLimiter defines the interface for rate limiting implementations.
type RateLimiter interface {
	// Allow checks if a request with the given key is allowed.
	// Returns whether the request is allowed and current rate limit info.
	Allow(key string) (bool, RateLimitInfo)
}

// RateLimitInfo contains current rate limit state for a given key.
type RateLimitInfo struct {
	// Limit is the maximum number of requests allowed per window.
	Limit int
	// Remaining is the number of requests remaining in the current window.
	Remaining int
	// ResetAt is the time when the rate limit window resets.
	ResetAt time.Time
}

// RateLimitConfig holds configuration for the rate limit middleware.
type RateLimitConfig struct {
	// RequestsPerSecond is the sustained request rate.
	RequestsPerSecond float64
	// BurstSize is the maximum burst size above the sustained rate.
	BurstSize int
	// KeyFunc extracts the rate limit key from a request.
	// If nil, defaults to client IP extraction.
	KeyFunc func(r *http.Request) string
	// SkipPaths are paths that bypass rate limiting.
	SkipPaths []string
	// ExceededHandler is called when rate limit is exceeded.
	// If nil, a default 429 response is sent.
	ExceededHandler http.Handler
	// CleanupInterval is how often expired entries are cleaned up.
	CleanupInterval time.Duration
}

// DefaultRateLimitConfig returns a sensible default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 10,
		BurstSize:         20,
		KeyFunc:           defaultKeyFunc,
		SkipPaths:         []string{"/health", "/healthz", "/readyz"},
		CleanupInterval:   5 * time.Minute,
	}
}

// defaultKeyFunc extracts the client IP as the rate limit key.
func defaultKeyFunc(r *http.Request) string {
	// Prefer X-Forwarded-For for proxied requests
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Prefer X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

// --- Token Bucket Limiter ---

// tokenBucket represents a single token bucket for one key.
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// TokenBucketLimiter implements RateLimiter using the token bucket algorithm.
type TokenBucketLimiter struct {
	rate            float64
	burstSize       int
	buckets         map[string]*tokenBucket
	mu              sync.RWMutex
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// NewTokenBucketLimiter creates a new token bucket rate limiter.
func NewTokenBucketLimiter(rate float64, burstSize int, cleanupInterval time.Duration) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		rate:            rate,
		burstSize:       burstSize,
		buckets:         make(map[string]*tokenBucket),
		cleanupInterval: cleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start background cleanup goroutine
	if cleanupInterval > 0 {
		go l.cleanupLoop()
	}

	return l
}

// Allow checks if a request with the given key is allowed under the rate limit.
func (l *TokenBucketLimiter) Allow(key string) (bool, RateLimitInfo) {
	now := time.Now()

	l.mu.RLock()
	bucket, exists := l.buckets[key]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()
		// Double-check after acquiring write lock
		bucket, exists = l.buckets[key]
		if !exists {
			bucket = &tokenBucket{
				tokens:     float64(l.burstSize),
				lastRefill: now,
			}
			l.buckets[key] = bucket
		}
		l.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * l.rate
	if bucket.tokens > float64(l.burstSize) {
		bucket.tokens = float64(l.burstSize)
	}
	bucket.lastRefill = now

	// Calculate reset time
	resetAt := now.Add(time.Duration(float64(time.Second) / l.rate))

	info := RateLimitInfo{
		Limit:     l.burstSize,
		Remaining: int(bucket.tokens),
		ResetAt:   resetAt,
	}

	// Check if we have tokens available
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		info.Remaining = int(bucket.tokens)
		return true, info
	}

	info.Remaining = 0
	return false, info
}

// cleanupLoop periodically removes stale buckets to prevent memory leaks.
func (l *TokenBucketLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stopCleanup:
			return
		}
	}
}

// cleanup removes buckets that have been full (idle) for longer than the cleanup interval.
func (l *TokenBucketLimiter) cleanup() {
	now := time.Now()
	threshold := now.Add(-l.cleanupInterval)

	l.mu.Lock()
	defer l.mu.Unlock()

	for key, bucket := range l.buckets {
		bucket.mu.Lock()
		if bucket.lastRefill.Before(threshold) && bucket.tokens >= float64(l.burstSize)-1 {
			delete(l.buckets, key)
		}
		bucket.mu.Unlock()
	}
}

// Stop stops the background cleanup goroutine.
func (l *TokenBucketLimiter) Stop() {
	close(l.stopCleanup)
}

// BucketCount returns the number of active buckets (for monitoring).
func (l *TokenBucketLimiter) BucketCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.buckets)
}

// --- Middleware ---

// RateLimit returns middleware that enforces rate limiting.
func RateLimit(limiter RateLimiter, config RateLimitConfig) func(http.Handler) http.Handler {
	skipSet := make(map[string]bool, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		skipSet[p] = true
	}

	keyFunc := config.KeyFunc
	if keyFunc == nil {
		keyFunc = defaultKeyFunc
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip configured paths
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			key := keyFunc(r)
			allowed, info := limiter.Allow(key)

			// Always set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(info.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(info.ResetAt.Unix(), 10))

			if !allowed {
				retryAfter := time.Until(info.ResetAt).Seconds()
				if retryAfter < 1 {
					retryAfter = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter)))

				if config.ExceededHandler != nil {
					config.ExceededHandler.ServeHTTP(w, r)
					return
				}

				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"rate limit exceeded, please retry later"}}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TenantKeyFunc returns a key function that uses the tenant ID for rate limiting.
// Falls back to IP if no tenant context is available.
func TenantKeyFunc(r *http.Request) string {
	if tenantID := ContextGetTenantID(r.Context()); tenantID != "" {
		return "tenant:" + tenantID
	}
	return "ip:" + defaultKeyFunc(r)
}

// APIKeyKeyFunc returns a key function that uses the API key ID for rate limiting.
func APIKeyKeyFunc(r *http.Request) string {
	if info := ContextGetAPIKeyInfo(r.Context()); info != nil {
		return "apikey:" + info.KeyID
	}
	return "ip:" + defaultKeyFunc(r)
}

// CompositeKeyFunc returns a key function that combines tenant + user + IP.
func CompositeKeyFunc(r *http.Request) string {
	parts := make([]byte, 0, 64)
	if tenantID := ContextGetTenantID(r.Context()); tenantID != "" {
		parts = append(parts, tenantID...)
		parts = append(parts, ':')
	}
	if userID := ContextGetUserID(r.Context()); userID != "" {
		parts = append(parts, userID...)
		parts = append(parts, ':')
	}
	parts = append(parts, defaultKeyFunc(r)...)
	return string(parts)
}

//Personal.AI order the ending
