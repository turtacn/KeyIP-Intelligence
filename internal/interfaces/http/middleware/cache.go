// Phase 11 - 接口层: HTTP Middleware - 响应缓存中间件
// 序号: 285
// 文件: internal/interfaces/http/middleware/cache.go
// 功能定位: 实现 HTTP 响应缓存中间件，支持 Cache-Control/ETag 响应头设置，
//           If-None-Match / 304 Not Modified 条件请求，以及可配置的内存缓存存储。
// 核心实现:
//   - CacheConfig 结构体: DefaultTTL, CacheableMethods, CacheableStatuses, CacheStore
//   - DefaultCacheConfig() CacheConfig 返回安全的默认配置 (5min TTL, GET only, 200 only)
//   - Cache(config) func(http.Handler) http.Handler 中间件工厂
//   - CacheStore 接口 + MemoryCache 内存实现，支持 TTL 过期
//   - ETag 生成: 基于响应体 MD5 哈希，格式为 "hex-encoded-hash"
//   - If-None-Match 支持: 强 ETag 匹配 + 弱 ETag(W/) + 通配符(*)匹配
//   - Cache-Control: 默认 public, max-age=<ttl>，尊重 handler 已设置值
//   - 缓存写入器 cacheResponseWriter 缓冲响应体用于 ETag 计算
//   - 遵循现有中间件 struct (CacheMiddleware) + Handler() 模式
//   - 仅对 GET 请求生效，仅缓存配置的状态码
//
// 依赖关系:
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CacheConfig holds configuration for the HTTP response caching middleware.
type CacheConfig struct {
	// DefaultTTL is the default cache duration applied to Cache-Control: max-age
	// and used as the server-side cache entry expiration.
	// Default: 5 minutes.
	DefaultTTL time.Duration

	// CacheableMethods lists HTTP methods eligible for caching.
	// Only responses to these methods are inspected for caching.
	// Default: ["GET"].
	CacheableMethods []string

	// CacheableStatuses lists HTTP status codes eligible for server-side caching.
	// Responses with status codes not in this set still receive Cache-Control and
	// ETag headers but are not stored in the server-side cache store.
	// Default: [200].
	CacheableStatuses []int

	// CacheStore is the storage backend for cached responses.
	// If nil, a default in-memory store with the configured DefaultTTL is used.
	CacheStore CacheStore
}

// DefaultCacheConfig returns a safe default cache configuration.
// Caches GET responses with status 200 for 5 minutes using an in-memory store.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		DefaultTTL:        5 * time.Minute,
		CacheableMethods:  []string{http.MethodGet},
		CacheableStatuses: []int{http.StatusOK},
	}
}

// cacheEntry stores a cached HTTP response along with metadata.
type cacheEntry struct {
	// etag is the strong ETag value computed from the response body.
	etag string
	// statusCode is the HTTP status code of the cached response.
	statusCode int
	// header contains the response headers from the original response.
	header http.Header
	// body is the full response body.
	body []byte
	// createdAt is the time when this entry was stored.
	createdAt time.Time
}

// CacheStore defines the interface for cache storage backends.
// Implementations must be safe for concurrent use.
type CacheStore interface {
	// Get retrieves a cached entry by key.
	// Returns nil, false if the key does not exist or the entry has expired.
	Get(key string) (*cacheEntry, bool)

	// Set stores a cache entry under the given key.
	Set(key string, entry *cacheEntry)

	// Delete removes a cache entry by key.
	Delete(key string)

	// Clear removes all cached entries.
	Clear()
}

// MemoryCache implements CacheStore using an in-memory map with TTL-based expiration.
// Each entry's TTL is checked on Get; expired entries are not returned but remain
// in the map until overwritten or cleared. For high-throughput deployments with
// many unique cache keys, consider a periodic cleanup or an LRU-based store.
type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	ttl     time.Duration
}

// NewMemoryCache creates a new in-memory cache with the given default TTL.
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves an entry from the cache. Returns nil, false if the key is not
// found or the entry has exceeded its TTL.
func (c *MemoryCache) Get(key string) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if time.Since(entry.createdAt) > c.ttl {
		return nil, false
	}
	return entry, true
}

// Set stores an entry in the cache under the given key.
func (c *MemoryCache) Set(key string, entry *cacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = entry
}

// Delete removes an entry from the cache by key.
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Clear removes all cached entries.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*cacheEntry)
}

// Len returns the number of entries currently in the cache (including expired
// entries that have not been cleaned up).
func (c *MemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// cacheResponseWriter wraps http.ResponseWriter to buffer the response body
// and status code. The actual response is not sent to the client until flush()
// is called, allowing the middleware to compute the ETag and set caching headers
// before the response is transmitted.
type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	body        bytes.Buffer
	header      http.Header
	wroteHeader bool
}

// Header returns the buffered header map. Headers set on this map are not
// forwarded to the underlying ResponseWriter until flush() is called.
func (w *cacheResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

// WriteHeader captures the status code without sending it to the client.
// The actual status code is forwarded during flush().
func (w *cacheResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = code
}

// Write buffers the data for ETag computation and later transmission.
// All writes are accumulated in an internal buffer; nothing is sent to the
// client until flush() is called.
func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(b)
}

// flush sends the buffered status code, headers, and body to the underlying
// ResponseWriter. Must be called exactly once after the handler chain completes.
func (w *cacheResponseWriter) flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	// Copy buffered headers to the underlying writer
	dest := w.ResponseWriter.Header()
	for k, vv := range w.header {
		for _, v := range vv {
			dest.Add(k, v)
		}
	}

	w.ResponseWriter.WriteHeader(w.statusCode)

	if w.body.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.body.Bytes())
	}
}

// Cache returns HTTP middleware that inspects and caches responses for
// cacheable HTTP methods (GET by default). On a cache hit, the response is
// served directly from the cache. On a cache miss, the response is captured,
// an ETag is computed from the body, Cache-Control and ETag headers are set,
// and the response is optionally stored in the server-side cache.
//
// The middleware also handles If-None-Match conditional requests, returning
// 304 Not Modified when the client's ETag matches the cached value.
func Cache(config CacheConfig) func(http.Handler) http.Handler {
	// Normalize configuration
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 5 * time.Minute
	}
	if len(config.CacheableMethods) == 0 {
		config.CacheableMethods = []string{http.MethodGet}
	}
	if len(config.CacheableStatuses) == 0 {
		config.CacheableStatuses = []int{http.StatusOK}
	}

	// Build lookup sets for O(1) access
	cacheableMethods := make(map[string]bool, len(config.CacheableMethods))
	for _, m := range config.CacheableMethods {
		cacheableMethods[strings.ToUpper(m)] = true
	}

	cacheableStatuses := make(map[int]bool, len(config.CacheableStatuses))
	for _, s := range config.CacheableStatuses {
		cacheableStatuses[s] = true
	}

	// Use provided cache store or create a default in-memory store
	store := config.CacheStore
	if store == nil {
		store = NewMemoryCache(config.DefaultTTL)
	}

	maxAge := int(config.DefaultTTL.Seconds())
	cacheControlValue := fmt.Sprintf("public, max-age=%d", maxAge)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only process cacheable methods (GET by default)
			if !cacheableMethods[r.Method] {
				next.ServeHTTP(w, r)
				return
			}

			cacheKey := r.URL.RequestURI()

			// --- Cache Hit ---
			if entry, ok := store.Get(cacheKey); ok {
				// Handle If-None-Match conditional request
				if noneMatch := r.Header.Get("If-None-Match"); noneMatch != "" {
					if matchETag(noneMatch, entry.etag) {
						// Copy Vary header from cached entry for cache correctness
						if vary := entry.header.Get("Vary"); vary != "" {
							w.Header().Set("Vary", vary)
						}
						w.Header().Set("ETag", entry.etag)
						w.WriteHeader(http.StatusNotModified)
						return
					}
				}

				// Serve the full cached response
				for k, vv := range entry.header {
					for _, v := range vv {
						w.Header().Add(k, v)
					}
				}
				w.Header().Set("ETag", entry.etag)
				w.Header().Set("Cache-Control", cacheControlValue)
				w.WriteHeader(entry.statusCode)
				if len(entry.body) > 0 {
					_, _ = w.Write(entry.body)
				}
				return
			}

			// --- Cache Miss: buffer the response for ETag computation ---
			crw := &cacheResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(crw, r)

			// Generate ETag from the captured response body
			bodyBytes := crw.body.Bytes()
			etag := generateETag(bodyBytes)

			// Set ETag header unconditionally
			crw.Header().Set("ETag", etag)

			// Set Cache-Control only if the handler did not already set one
			if crw.Header().Get("Cache-Control") == "" {
				crw.Header().Set("Cache-Control", cacheControlValue)
			}

			// Store in server-side cache if the status code is cacheable
			if cacheableStatuses[crw.statusCode] {
				entry := &cacheEntry{
					etag:       etag,
					statusCode: crw.statusCode,
					header:     crw.Header().Clone(),
					body:       bodyBytes,
					createdAt:  time.Now(),
				}
				store.Set(cacheKey, entry)
			}

			// Send the buffered response to the client
			crw.flush()
		})
	}
}

// generateETag computes a strong ETag value from data using MD5.
// The returned string is a quoted hexadecimal string, e.g. "d41d8cd98f00b204e9800998ecf8427e".
func generateETag(data []byte) string {
	hash := md5.Sum(data)
	return `"` + hex.EncodeToString(hash[:]) + `"`
}

// matchETag checks whether any ETag in the comma-separated If-None-Match header
// value matches the given ETag. It supports:
//   - Strong ETag comparison (e.g. "abc123")
//   - Weak ETag comparison (W/"abc123" matches "abc123")
//   - Wildcard (* matches any ETag)
func matchETag(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "*" {
		return true
	}

	for _, token := range strings.Split(header, ",") {
		token = strings.TrimSpace(token)

		// Strip weak comparator prefix if present
		if strings.HasPrefix(token, "W/") {
			token = token[2:]
		}

		if token == etag {
			return true
		}
	}
	return false
}

// CacheMiddleware wraps the response cache middleware for use with the router
// configuration struct, following the established middleware pattern.
type CacheMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewCacheMiddleware creates a new CacheMiddleware with the given configuration.
func NewCacheMiddleware(config CacheConfig) *CacheMiddleware {
	return &CacheMiddleware{
		handler: Cache(config),
	}
}

// Handler returns the middleware handler function, allowing this middleware to
// be used with the router's Chain helper and RouterConfig struct.
func (m *CacheMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

//Personal.AI order the ending
