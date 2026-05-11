// Phase 11 - 接口层: HTTP Middleware - 速率限制中间件
// 序号: 278
// 文件: internal/interfaces/http/middleware/ratelimit.go
// 功能定位: 实现 HTTP 请求速率限制中间件，支持基于 IP、API Key、Tenant、User Tier 的多维度限流
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
//   - TenantRateLimiter: 租户级独立配额（跨所有用户聚合计数）
//   - 租户限流键格式: "tenant:<id>"，在用户/层级限流之前检查
//
// 依赖关系:
//   - 依赖: internal/infrastructure/monitoring/logging
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
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

	// TierLimits maps user tiers to their specific rate limits.
	// If nil or empty, per-tier limits are not enforced and RequestsPerSecond/BurstSize are used.
	TierLimits map[UserTier]TierLimits
	// TenantRequestsPerSecond is the per-tenant sustained request rate across all users in that tenant.
	// A value of 0 or less disables tenant-level rate limiting.
	TenantRequestsPerSecond float64
	// TenantBurstSize is the maximum burst for tenant-level rate limiting.
	// Ignored when TenantRequestsPerSecond <= 0.
	TenantBurstSize int
	// ExpensivePaths are request paths that consume extra tokens per request
	// (e.g., similarity search, report generation).
	ExpensivePaths []string
	// ExpensiveCost is the token cost for each expensive endpoint request.
	// A value of 0 or less defaults to 5.
	ExpensiveCost int
	// RedisAddr is the Redis address for distributed rate limiting.
	// If empty, only in-memory limiting is used.
	RedisAddr string
	// RedisDB is the Redis database number.
	RedisDB int
	// RedisPassword is the Redis password.
	RedisPassword string
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

// RateLimitMiddleware wraps rate limiting middleware for use with router configuration.
type RateLimitMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewRateLimitMiddleware creates a new rate limit middleware with the given limiter and config.
func NewRateLimitMiddleware(limiter RateLimiter, config RateLimitConfig) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		handler: RateLimit(limiter, config),
	}
}

// Handler returns the middleware handler function.
func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

// --- User Tier Types ---

// Default rate limits for expensive endpoints (used when config values are unavailable).
const (
	DefaultExpensiveRPS   = 0.5 // 30 requests/minute
	DefaultExpensiveBurst = 10
)

// UserTier represents a user's subscription/access tier for rate limiting.
type UserTier string

const (
	// TierUnset indicates no tier information is available (anonymous).
	TierUnset UserTier = ""
	// TierFree is the free tier: 60 requests per minute.
	TierFree UserTier = "free"
	// TierProfessional is the professional tier: 300 requests per minute.
	TierProfessional UserTier = "professional"
	// TierEnterprise is the enterprise tier: 1000 requests per minute.
	TierEnterprise UserTier = "enterprise"
)

// TierLimits defines rate limits for a specific user tier.
type TierLimits struct {
	// RequestsPerSecond is the sustained request rate for this tier.
	RequestsPerSecond float64
	// BurstSize is the maximum burst size for this tier.
	BurstSize int
}

// DefaultTierLimits returns the default rate limits for a given tier.
func DefaultTierLimits(tier UserTier) TierLimits {
	switch tier {
	case TierFree:
		return TierLimits{RequestsPerSecond: 1.0, BurstSize: 60} // 60 requests/minute
	case TierProfessional:
		return TierLimits{RequestsPerSecond: 5.0, BurstSize: 300} // ~300 requests/minute
	case TierEnterprise:
		return TierLimits{RequestsPerSecond: 17.0, BurstSize: 1000} // ~1000 requests/minute
	default:
		return TierLimits{RequestsPerSecond: 10.0, BurstSize: 20} // default (unset/anonymous)
	}
}

// ContextGetUserTier extracts the user's rate limit tier from the request context.
// It checks API key RateLimit field first, then JWT claims roles.
// Returns TierUnset for anonymous/unauthenticated requests.
func ContextGetUserTier(ctx context.Context) UserTier {
	// Check API key info first (APIKeyInfo has an explicit RateLimit field)
	if info := ContextGetAPIKeyInfo(ctx); info != nil {
		switch {
		case info.RateLimit >= 1000:
			return TierEnterprise
		case info.RateLimit >= 300:
			return TierProfessional
		case info.RateLimit > 0:
			return TierFree
		}
		return TierFree
	}

	// Check JWT claims roles
	if claims := ContextGetClaims(ctx); claims != nil {
		for _, role := range claims.Roles {
			switch strings.ToLower(role) {
			case "enterprise", "admin":
				return TierEnterprise
			case "professional":
				return TierProfessional
			}
		}
		// Authenticated but no matching role = free tier
		return TierFree
	}

	return TierUnset
}

// TierKeyFunc returns a key function that encodes the user's rate limit tier
// into the rate limit key. Falls back to CompositeKeyFunc when no tier is detected.
func TierKeyFunc(r *http.Request) string {
	tier := ContextGetUserTier(r.Context())
	if tier == TierUnset {
		return CompositeKeyFunc(r)
	}

	if claims := ContextGetClaims(r.Context()); claims != nil {
		return "tier:" + string(tier) + ":user:" + claims.UserID
	}
	if info := ContextGetAPIKeyInfo(r.Context()); info != nil {
		return "tier:" + string(tier) + ":apikey:" + info.KeyID
	}
	return "tier:" + string(tier) + ":ip:" + defaultKeyFunc(r)
}

// ExpensiveKeyFunc wraps another key function with an "expensive:" prefix
// so the rate limiter can apply higher token costs.
func ExpensiveKeyFunc(base func(r *http.Request) string) func(r *http.Request) string {
	return func(r *http.Request) string {
		return "expensive:" + base(r)
	}
}

// --- Tier Rate Limiter ---

// TierRateLimiter implements RateLimiter with per-tier token buckets.
// It manages separate TokenBucketLimiter instances for each tier, plus
// an optional expensive-endpoint limiter.
type TierRateLimiter struct {
	defaultLimiter   *TokenBucketLimiter
	tierLimiters     map[UserTier]*TokenBucketLimiter
	expensiveLimiter *TokenBucketLimiter
	tierLimits       map[UserTier]TierLimits
}

// NewTierRateLimiter creates a new TierRateLimiter from the given config.
// It creates per-tier limiters based on config.TierLimits or defaults.
// If config.ExpensivePaths is non-empty, an additional limiter is created.
func NewTierRateLimiter(config RateLimitConfig) *TierRateLimiter {
	// Build tier limiter map
	tierLimiters := make(map[UserTier]*TokenBucketLimiter)
	tierLimits := make(map[UserTier]TierLimits)

	tiers := []UserTier{TierFree, TierProfessional, TierEnterprise}
	for _, tier := range tiers {
		limits, ok := config.TierLimits[tier]
		if !ok {
			limits = DefaultTierLimits(tier)
		}
		tierLimits[tier] = limits
		tierLimiters[tier] = NewTokenBucketLimiter(
			limits.RequestsPerSecond,
			limits.BurstSize,
			config.CleanupInterval,
		)
	}

	// Default limiter (for non-tiered keys, e.g., IP-based)
	defaultLimiter := NewTokenBucketLimiter(
		config.RequestsPerSecond,
		config.BurstSize,
		config.CleanupInterval,
	)

	var expensiveLimiter *TokenBucketLimiter
	if len(config.ExpensivePaths) > 0 {
		expensiveLimiter = NewTokenBucketLimiter(
			DefaultExpensiveRPS,
			DefaultExpensiveBurst,
			config.CleanupInterval,
		)
	}

	return &TierRateLimiter{
		defaultLimiter:   defaultLimiter,
		tierLimiters:     tierLimiters,
		expensiveLimiter: expensiveLimiter,
		tierLimits:       tierLimits,
	}
}

// Stop stops all managed limiters' background cleanup goroutines.
func (l *TierRateLimiter) Stop() {
	l.defaultLimiter.Stop()
	for _, limiter := range l.tierLimiters {
		limiter.Stop()
	}
	if l.expensiveLimiter != nil {
		l.expensiveLimiter.Stop()
	}
}

// Allow checks if a request with the given key is allowed.
// The key can encode tier and expensive endpoint information via prefixes:
//   - "tier:<tier>:<key>" routes to the per-tier limiter
//   - "expensive:<inner_key>" routes to the expensive-endpoint limiter
//   - "expensive:tier:<tier>:<key>" routes to the tiered expensive limiter
//   - any other key uses the default (non-tiered) limiter
func (l *TierRateLimiter) Allow(key string) (bool, RateLimitInfo) {
	// Handle expensive endpoint keys
	if strings.HasPrefix(key, "expensive:") {
		innerKey := strings.TrimPrefix(key, "expensive:")

		// Nested tier prefix inside expensive
		if strings.HasPrefix(innerKey, "tier:") {
			tier := extractTierFromKey(innerKey)
			innerKey = stripTierFromKey(innerKey)
			if limiter, ok := l.tierLimiters[tier]; ok {
				return l.allowWithCost(limiter, innerKey, 5) // expensive = 5x token cost
			}
			return l.allowWithCost(l.defaultLimiter, innerKey, 5)
		}

		// Expensive but not tiered: apply default limiter with higher cost
		return l.allowWithCost(l.defaultLimiter, innerKey, 5)
	}

	// Handle tier-keyed requests
	if strings.HasPrefix(key, "tier:") {
		tier := extractTierFromKey(key)
		innerKey := stripTierFromKey(key)
		if limiter, ok := l.tierLimiters[tier]; ok {
			return limiter.Allow(innerKey)
		}
		return l.defaultLimiter.Allow(key)
	}

	// Default: use the default limiter (IP-based, anonymous, etc.)
	return l.defaultLimiter.Allow(key)
}

// allowWithCost allows a request consuming the given number of tokens.
func (l *TierRateLimiter) allowWithCost(limiter *TokenBucketLimiter, key string, cost int) (bool, RateLimitInfo) {
	var info RateLimitInfo
	for i := 0; i < cost; i++ {
		ok, currentInfo := limiter.Allow(key)
		if !ok {
			return false, currentInfo
		}
		if i == cost-1 {
			info = currentInfo
		}
	}
	return true, info
}

// extractTierFromKey extracts the UserTier from a tier-prefixed key.
// Key format: "tier:<tier>:<rest>"
// Returns TierUnset if the key does not start with "tier:".
func extractTierFromKey(key string) UserTier {
	if !strings.HasPrefix(key, "tier:") {
		return TierUnset
	}
	parts := strings.SplitN(key, ":", 3)
	if len(parts) >= 2 {
		return UserTier(parts[1])
	}
	return TierUnset
}

// stripTierFromKey removes the "tier:<tier>:" prefix from a key.
// Returns the original key if it does not start with "tier:".
func stripTierFromKey(key string) string {
	if !strings.HasPrefix(key, "tier:") {
		return key
	}
	parts := strings.SplitN(key, ":", 3)
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// IsExpensivePath checks whether the given request path matches an expensive endpoint.
func IsExpensivePath(path string, expensivePaths []string) bool {
	for _, p := range expensivePaths {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// --- Tenant Rate Limiter ---

// TenantRateLimiter enforces per-tenant aggregate rate limits across all users.
// It provides an additional rate limiting dimension beyond user/tier/IP-level limits.
// Each tenant gets a separate token bucket keyed by "tenant:<tenantID>".
type TenantRateLimiter struct {
	limiter *TokenBucketLimiter
	enabled bool
	rate    float64
	burst   int
}

// NewTenantRateLimiter creates a new TenantRateLimiter.
// Tenant-level rate limiting is disabled when rate <= 0 or burst <= 0.
func NewTenantRateLimiter(rate float64, burstSize int, cleanupInterval time.Duration) *TenantRateLimiter {
	enabled := rate > 0 && burstSize > 0
	var limiter *TokenBucketLimiter
	if enabled {
		limiter = NewTokenBucketLimiter(rate, burstSize, cleanupInterval)
	}
	return &TenantRateLimiter{
		limiter: limiter,
		enabled: enabled,
		rate:    rate,
		burst:   burstSize,
	}
}

// Allow checks if a request for the given tenant ID is within the tenant's aggregate quota.
// Returns (true, empty RateLimitInfo) when tenant limiting is disabled or tenantID is empty.
func (l *TenantRateLimiter) Allow(tenantID string) (bool, RateLimitInfo) {
	if !l.enabled || tenantID == "" {
		return true, RateLimitInfo{}
	}
	return l.limiter.Allow("tenant:" + tenantID)
}

// Stop stops the underlying limiter's cleanup goroutine.
func (l *TenantRateLimiter) Stop() {
	if l.limiter != nil {
		l.limiter.Stop()
	}
}

// IsEnabled returns whether tenant-level rate limiting is enabled.
func (l *TenantRateLimiter) IsEnabled() bool {
	return l.enabled
}

// --- Redis Rate Limiter ---

// redisTokenBucketScript is a Lua script implementing the token bucket algorithm in Redis.
// KEYS[1]: rate_limit:{key} (tokens count)
// KEYS[2]: rate_limit:{key}:last (last refill timestamp)
// ARGV[1]: rate (tokens per second)
// ARGV[2]: burst (maximum tokens)
// ARGV[3]: now (current Unix timestamp in seconds)
// ARGV[4]: cost (token cost per request, default 1)
// Returns: {allowed (1 or 0), remaining (int), reset_in_seconds (int)}
const redisTokenBucketScript = `
local key = KEYS[1]
local last_key = KEYS[2]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local t = redis.call('get', key)
local tokens
if t then
	tokens = tonumber(t)
else
	tokens = burst
end

local lt = redis.call('get', last_key)
local last
if lt then
	last = tonumber(lt)
else
	last = now
end

local elapsed = now - last
tokens = math.min(tokens + elapsed * rate, burst)

if tokens >= cost then
	tokens = tokens - cost
	redis.call('set', key, tokens)
	redis.call('set', last_key, now)
	redis.call('expire', key, 60)
	redis.call('expire', last_key, 60)
	local remaining = math.floor(tokens)
	local reset_in = 1
	if rate > 0 then
		reset_in = math.ceil((burst - tokens) / rate)
		if reset_in < 1 then reset_in = 1 end
	end
	return {1, remaining, reset_in}
else
	redis.call('set', key, tokens)
	redis.call('set', last_key, last)
	redis.call('expire', key, 60)
	redis.call('expire', last_key, 60)
	local reset_in = 1
	if rate > 0 then
		reset_in = math.ceil((burst - tokens) / rate)
		if reset_in < 1 then reset_in = 1 end
	end
	return {0, 0, reset_in}
end
`

// RedisRateLimiter implements RateLimiter backed by Redis.
// When Redis is unavailable, it gracefully falls back to an in-memory TokenBucketLimiter.
type RedisRateLimiter struct {
	client          *goredis.Client
	prefix          string
	fallbackLimiter *TokenBucketLimiter
	mu              sync.RWMutex
	fallbackMode    bool
	rate            float64
	burstSize       int
	logger          logging.Logger
	scriptSHA       string
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter.
// If redisAddr is empty or connection fails, it operates in fallback mode using
// an in-memory TokenBucketLimiter.
func NewRedisRateLimiter(redisAddr string, config RateLimitConfig, logger logging.Logger) *RedisRateLimiter {
	l := &RedisRateLimiter{
		prefix:          "rate_limit:",
		rate:            config.RequestsPerSecond,
		burstSize:       config.BurstSize,
		fallbackLimiter: NewTokenBucketLimiter(config.RequestsPerSecond, config.BurstSize, config.CleanupInterval),
		logger:          logger,
	}

	if redisAddr == "" {
		l.fallbackMode = true
		if logger != nil {
			logger.Warn("Redis address empty, rate limiter running in fallback (in-memory) mode")
		}
		return l
	}

	// Attempt Redis connection
	l.client = goredis.NewClient(&goredis.Options{
		Addr:         redisAddr,
		DB:           config.RedisDB,
		Password:     config.RedisPassword,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  1 * time.Second,
		WriteTimeout: 1 * time.Second,
		PoolSize:     5,
		MinIdleConns: 2,
	})

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := l.client.Ping(ctx).Err(); err != nil {
		l.fallbackMode = true
		l.client = nil
		if logger != nil {
			logger.Warn("Redis connection failed, rate limiter running in fallback (in-memory) mode",
				logging.Err(err))
		}
		return l
	}

	// Load the Lua script
	sha, err := l.client.ScriptLoad(ctx, redisTokenBucketScript).Result()
	if err != nil {
		l.fallbackMode = true
		l.client = nil
		if logger != nil {
			logger.Warn("Redis script load failed, rate limiter running in fallback (in-memory) mode",
				logging.Err(err))
		}
		return l
	}
	l.scriptSHA = sha

	if logger != nil {
		logger.Info("Redis rate limiter initialized",
			logging.String("addr", redisAddr))
	}

	return l
}

// Allow checks rate limit via Redis. Falls back to in-memory limiter on Redis errors.
func (l *RedisRateLimiter) Allow(key string) (bool, RateLimitInfo) {
	l.mu.RLock()
	fallbackMode := l.fallbackMode
	client := l.client
	l.mu.RUnlock()

	if fallbackMode || client == nil {
		return l.fallbackLimiter.Allow(key)
	}

	now := time.Now().Unix()
	redisKey := l.prefix + key
	lastKey := redisKey + ":last"

	result, err := client.EvalSha(context.Background(), l.scriptSHA, []string{redisKey, lastKey},
		l.rate, l.burstSize, now, 1,
	).Result()

	if err != nil {
		// Redis error: switch to fallback mode and retry with in-memory
		l.mu.Lock()
		l.fallbackMode = true
		l.mu.Unlock()

		if l.logger != nil {
			l.logger.Warn("Redis rate limiter error, switching to fallback mode",
				logging.Err(err))
		}

		return l.fallbackLimiter.Allow(key)
	}

	// Parse result from Redis Lua script: [allowed, remaining, reset_in]
	vals, ok := result.([]any)
	if !ok || len(vals) < 3 {
		return l.fallbackLimiter.Allow(key)
	}

	allowed, _ := vals[0].(int64)
	remaining, _ := vals[1].(int64)
	resetIn, _ := vals[2].(int64)

	resetAt := time.Now().Add(time.Duration(resetIn) * time.Second)

	return allowed == 1, RateLimitInfo{
		Limit:     l.burstSize,
		Remaining: int(remaining),
		ResetAt:   resetAt,
	}
}

// Stop stops the fallback limiter's background goroutine.
func (l *RedisRateLimiter) Stop() {
	l.fallbackLimiter.Stop()
	if l.client != nil {
		l.client.Close()
	}
}

// IsFallbackMode returns true if the limiter is currently running in fallback (in-memory) mode.
func (l *RedisRateLimiter) IsFallbackMode() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.fallbackMode
}

// --- Enhanced Middleware Factory ---

// NewTieredRateLimitMiddleware creates a tier-aware rate limit middleware.
// It detects the user's tier from context, applies appropriate rate limits,
// and handles expensive endpoints separately.
//
// The returned middleware:
//   - Extracts user tier from JWT claims or API key info
//   - Applies per-tier rate limits (Free: 60/min, Professional: 300/min, Enterprise: 1000/min)
//   - Applies stricter limits for expensive endpoints (similarity search, report generation)
//   - Sets X-RateLimit-* headers on every response
//   - Returns 429 with Retry-After when rate limit is exceeded
//   - Uses Redis when available, falls back to in-memory
func NewTieredRateLimitMiddleware(config RateLimitConfig) *RateLimitMiddleware {
	// Apply defaults for expensive cost
	if config.ExpensiveCost <= 0 {
		config.ExpensiveCost = 5
	}

	// Use the tier-aware key function by default in the tiered middleware
	config.KeyFunc = TierKeyFunc

	// Create the limiter: Redis-backed or in-memory tiered
	var limiter RateLimiter

	if config.RedisAddr != "" {
		// Use Redis with a fallback tiered limiter
		redisLimiter := NewRedisRateLimiter(config.RedisAddr, config, nil)
		limiter = redisLimiter
	} else {
		limiter = NewTierRateLimiter(config)
	}

	skipSet := make(map[string]bool, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		skipSet[p] = true
	}

	keyFunc := config.KeyFunc
	expensivePaths := config.ExpensivePaths
	expensiveCost := config.ExpensiveCost

	// Create tenant-level rate limiter for per-tenant aggregate quotas
	tenantLimiter := NewTenantRateLimiter(
		config.TenantRequestsPerSecond,
		config.TenantBurstSize,
		config.CleanupInterval,
	)

	return &RateLimitMiddleware{
		handler: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Skip configured paths
				if skipSet[r.URL.Path] {
					next.ServeHTTP(w, r)
					return
				}

				// --- Tenant-level rate limit check (applies before user/tier check) ---
				// Tenant quota is shared across all users in the same tenant.
				// This ensures a single tenant cannot overwhelm the system through
				// many concurrent users, even if each user is within their personal limit.
				tenantID := ContextGetTenantID(r.Context())
				if tenantLimiter.IsEnabled() && tenantID != "" {
					tenantAllowed, tenantInfo := tenantLimiter.Allow(tenantID)
					if !tenantAllowed {
						w.Header().Set("X-RateLimit-Tenant-Limit", strconv.Itoa(tenantInfo.Limit))
						w.Header().Set("X-RateLimit-Tenant-Remaining", "0")
						w.Header().Set("X-RateLimit-Tenant-Reset", strconv.FormatInt(tenantInfo.ResetAt.Unix(), 10))
						writeRateLimitExceeded(w, config, tenantInfo)
						return
					}
					w.Header().Set("X-RateLimit-Tenant-Limit", strconv.Itoa(tenantInfo.Limit))
					w.Header().Set("X-RateLimit-Tenant-Remaining", strconv.Itoa(tenantInfo.Remaining))
					w.Header().Set("X-RateLimit-Tenant-Reset", strconv.FormatInt(tenantInfo.ResetAt.Unix(), 10))
				}

				// Determine key: use expensive key for expensive paths
				var key string
				if IsExpensivePath(r.URL.Path, expensivePaths) {
					// Apply cost multiplier by calling Allow multiple times
					key = keyFunc(r)
					// For expensive endpoints, try to consume expensiveCost tokens
					allowed := true
					var info RateLimitInfo
					for i := 0; i < expensiveCost; i++ {
						var ok bool
						ok, info = limiter.Allow(key)
						if !ok {
							allowed = false
							break
						}
					}
					if !allowed {
						writeRateLimitExceeded(w, config, info)
						return
					}
					// Get fresh info after consumption
					setRateLimitHeaders(w, info)
					next.ServeHTTP(w, r)
					return
				}

				// Standard endpoint
				key = keyFunc(r)
				allowed, info := limiter.Allow(key)

				// Always set rate limit headers
				setRateLimitHeaders(w, info)

				if !allowed {
					writeRateLimitExceeded(w, config, info)
					return
				}

				next.ServeHTTP(w, r)
			})
		},
	}
}

// setRateLimitHeaders sets standard rate limit response headers.
func setRateLimitHeaders(w http.ResponseWriter, info RateLimitInfo) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(info.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(info.ResetAt.Unix(), 10))
}

// writeRateLimitExceeded writes a 429 Too Many Requests response.
func writeRateLimitExceeded(w http.ResponseWriter, config RateLimitConfig, info RateLimitInfo) {
	retryAfter := time.Until(info.ResetAt).Seconds()
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter))))

	if config.ExceededHandler != nil {
		config.ExceededHandler.ServeHTTP(w, &http.Request{})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"rate limit exceeded, please retry later"}}`))
}

//Personal.AI order the ending
