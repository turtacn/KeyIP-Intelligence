// Phase 11 - 接口层: HTTP Middleware - API 使用统计中间件
// 序号: 294
// 文件: internal/interfaces/http/middleware/usage_stats.go
// 功能定位: 收集 API 使用统计数据，包括按端点和方法的请求计数、
//           按用户/租户的请求统计、响应大小直方图，并定期记录汇总日志
// 核心实现:
//   - 定义 UsageStatsConfig 结构体: SkipPaths, FlushInterval, EnableUserTracking
//   - 使用 sync.Map 线程安全存储统计数据
//   - 按 endpoint + method 统计请求数
//   - 按用户/租户统计（从 JWT claims 或 API key 提取）
//   - 响应大小统计（五级直方图: small/medium/large/xlarge/xxlarge）
//   - 定期写入日志（每 5 分钟汇总并重置计数器）
//   - 实现 Stop() 方法用于优雅关闭
//
// 依赖关系:
//   - 依赖: internal/infrastructure/monitoring/logging
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// UsageStatsConfig holds configuration for the API usage statistics middleware.
type UsageStatsConfig struct {
	// SkipPaths are paths that should not be tracked.
	// Default: /health, /healthz, /readyz, /metrics.
	SkipPaths []string

	// FlushInterval is how often aggregated stats are logged.
	// Default: 5 minutes.
	FlushInterval time.Duration

	// EnableUserTracking enables tracking by user/tenant from auth context.
	// Default: true.
	EnableUserTracking bool
}

// DefaultUsageStatsConfig returns a sensible default configuration.
func DefaultUsageStatsConfig() UsageStatsConfig {
	return UsageStatsConfig{
		SkipPaths:          []string{"/health", "/healthz", "/readyz", "/metrics"},
		FlushInterval:      5 * time.Minute,
		EnableUserTracking: true,
	}
}

// endpointKey uniquely identifies a (method, endpoint) combination for stats.
type endpointKey struct {
	Method   string
	Endpoint string
}

// String returns a human-readable representation of the endpoint key.
func (k endpointKey) String() string {
	return fmt.Sprintf("%s %s", k.Method, k.Endpoint)
}

// userKey uniquely identifies a (userID, tenantID) combination for stats.
type userKey struct {
	UserID   string
	TenantID string
}

// String returns a human-readable representation of the user key.
func (k userKey) String() string {
	switch {
	case k.UserID != "" && k.TenantID != "":
		return fmt.Sprintf("user=%s/tenant=%s", k.UserID, k.TenantID)
	case k.UserID != "":
		return fmt.Sprintf("user=%s", k.UserID)
	default:
		return fmt.Sprintf("tenant=%s", k.TenantID)
	}
}

// SizeBucket represents a response size category for histogram tracking.
type SizeBucket int

const (
	// SizeSmall represents responses < 1 KB.
	SizeSmall SizeBucket = iota
	// SizeMedium represents responses 1 KB - 10 KB.
	SizeMedium
	// SizeLarge represents responses 10 KB - 100 KB.
	SizeLarge
	// SizeXLarge represents responses 100 KB - 1 MB.
	SizeXLarge
	// SizeXXLarge represents responses > 1 MB.
	SizeXXLarge
)

// sizeBucketNames maps each SizeBucket to its human-readable name.
var sizeBucketNames = map[SizeBucket]string{
	SizeSmall:  "small",
	SizeMedium: "medium",
	SizeLarge:  "large",
	SizeXLarge: "xlarge",
	SizeXXLarge: "xxlarge",
}

// sizeBucketThresholds defines the minimum byte count for each bucket.
// A response falls into the highest bucket whose threshold it meets or exceeds.
var sizeBucketThresholds = []int64{0, 1024, 10240, 102400, 1048576}

// responseBucketFor returns the SizeBucket for a given byte count.
func responseBucketFor(bytes int64) SizeBucket {
	for i := len(sizeBucketThresholds) - 1; i >= 0; i-- {
		if bytes >= sizeBucketThresholds[i] {
			return SizeBucket(i)
		}
	}
	return SizeSmall
}

// usageStatsResponseWriter wraps http.ResponseWriter to capture bytes written.
type usageStatsResponseWriter struct {
	http.ResponseWriter
	bytesWritten int64
}

// Write captures the number of bytes written to the response.
func (w *usageStatsResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// UsageStatsMiddleware collects API usage statistics including:
//   - request counts by endpoint + method
//   - request counts by user/tenant (from JWT claims or API key)
//   - response size distribution (histogram with 5 buckets)
//   - periodic aggregated logging with configurable interval
//
// All counters use sync.Map with int64 values for thread-safe concurrent access.
type UsageStatsMiddleware struct {
	config UsageStatsConfig
	logger logging.Logger

	// endpointCounts tracks requests per (method, endpoint): map[endpointKey]*int64
	endpointCounts sync.Map
	// endpointBytes tracks bytes written per (method, endpoint): map[endpointKey]*int64
	endpointBytes sync.Map
	// userCounts tracks requests per (userID, tenantID): map[userKey]*int64
	userCounts sync.Map
	// sizeBuckets tracks response size distribution: map[SizeBucket]*int64
	sizeBuckets sync.Map

	startOnce sync.Once
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// NewUsageStatsMiddleware creates a new UsageStatsMiddleware with the given
// configuration. It initializes size bucket counters and prepares the periodic
// flush goroutine which starts on the first handled request.
func NewUsageStatsMiddleware(logger logging.Logger, config UsageStatsConfig) *UsageStatsMiddleware {
	if config.FlushInterval <= 0 {
		config.FlushInterval = 5 * time.Minute
	}
	if config.SkipPaths == nil {
		config.SkipPaths = []string{"/health", "/healthz", "/readyz", "/metrics"}
	}

	m := &UsageStatsMiddleware{
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
	}

	// Pre-initialize size bucket counters so they always appear in reports.
	for i := SizeSmall; i <= SizeXXLarge; i++ {
		m.sizeBuckets.Store(i, new(int64))
	}

	return m
}

// ensureFlushLoop starts the periodic flush goroutine on first call.
// Uses sync.Once to guarantee single startup.
func (m *UsageStatsMiddleware) ensureFlushLoop() {
	m.startOnce.Do(func() {
		go m.flushLoop()
	})
}

// flushLoop runs on a ticker, periodically flushing and resetting stats.
// It performs a final flush when stopCh is closed.
func (m *UsageStatsMiddleware) flushLoop() {
	ticker := time.NewTicker(m.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.flush()
		case <-m.stopCh:
			// Final flush before stopping.
			m.flush()
			return
		}
	}
}

// flush logs all aggregated statistics and resets counters to zero.
func (m *UsageStatsMiddleware) flush() {
	m.flushEndpointStats()
	m.flushUserStats()
	m.flushSizeHistogram()
}

// flushEndpointStats collects, logs, and resets endpoint request counters.
func (m *UsageStatsMiddleware) flushEndpointStats() {
	type endpointEntry struct {
		key   endpointKey
		count int64
		bytes int64
	}

	// Collect endpoint bytes first.
	bytesByEndpoint := make(map[endpointKey]int64)
	m.endpointBytes.Range(func(k, v interface{}) bool {
		key := k.(endpointKey)
		bytes := atomic.SwapInt64(v.(*int64), 0)
		if bytes > 0 {
			bytesByEndpoint[key] = bytes
		}
		return true
	})

	// Collect endpoint counts.
	var entries []endpointEntry
	m.endpointCounts.Range(func(k, v interface{}) bool {
		key := k.(endpointKey)
		count := atomic.SwapInt64(v.(*int64), 0)
		if count > 0 {
			entries = append(entries, endpointEntry{
				key:   key,
				count: count,
				bytes: bytesByEndpoint[key],
			})
		}
		return true
	})

	if len(entries) == 0 {
		return
	}

	// Sort by count descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	fields := []logging.Field{
		logging.Int("total_endpoints", len(entries)),
	}
	for _, e := range entries {
		fields = append(fields,
			logging.String("ep", e.key.String()),
			logging.Int64("count", e.count),
			logging.Int64("bytes", e.bytes),
		)
	}
	m.logger.Info("API usage stats - endpoints", fields...)
}

// flushUserStats collects, logs, and resets user/tenant request counters.
func (m *UsageStatsMiddleware) flushUserStats() {
	if !m.config.EnableUserTracking {
		return
	}

	type userEntry struct {
		key   userKey
		count int64
	}

	var entries []userEntry
	m.userCounts.Range(func(k, v interface{}) bool {
		key := k.(userKey)
		count := atomic.SwapInt64(v.(*int64), 0)
		if count > 0 {
			entries = append(entries, userEntry{key: key, count: count})
		}
		return true
	})

	if len(entries) == 0 {
		return
	}

	// Sort by count descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	fields := []logging.Field{
		logging.Int("total_users", len(entries)),
	}
	for _, e := range entries {
		fields = append(fields,
			logging.String("identity", e.key.String()),
			logging.Int64("count", e.count),
		)
	}
	m.logger.Info("API usage stats - users", fields...)
}

// flushSizeHistogram collects, logs, and resets response size bucket counters.
func (m *UsageStatsMiddleware) flushSizeHistogram() {
	type sizeEntry struct {
		bucket SizeBucket
		count  int64
	}

	var entries []sizeEntry
	m.sizeBuckets.Range(func(k, v interface{}) bool {
		bucket := k.(SizeBucket)
		count := atomic.SwapInt64(v.(*int64), 0)
		if count > 0 {
			entries = append(entries, sizeEntry{bucket: bucket, count: count})
		}
		return true
	})

	if len(entries) == 0 {
		return
	}

	// Sort by bucket ascending (small -> xxlarge).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].bucket < entries[j].bucket
	})

	fields := []logging.Field{}
	totalCount := int64(0)
	for _, e := range entries {
		fields = append(fields,
			logging.String("bucket", sizeBucketNames[e.bucket]),
			logging.Int64("count", e.count),
		)
		totalCount += e.count
	}
	fields = append(fields, logging.Int64("total", totalCount))
	m.logger.Info("API usage stats - response sizes", fields...)
}

// skipPath checks if the given path should be excluded from tracking.
func (m *UsageStatsMiddleware) skipPath(path string) bool {
	for _, p := range m.config.SkipPaths {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// incrementCounter atomically increments a counter stored in sync.Map.
// The counter must be a *int64 value.
func incrementCounter(m *sync.Map, key interface{}) {
	actual, _ := m.LoadOrStore(key, new(int64))
	atomic.AddInt64(actual.(*int64), 1)
}

// addCounter atomically adds delta to a counter stored in sync.Map.
// The counter must be a *int64 value.
func addCounter(m *sync.Map, key interface{}, delta int64) {
	actual, _ := m.LoadOrStore(key, new(int64))
	atomic.AddInt64(actual.(*int64), delta)
}

// Handler returns the middleware handler function that collects API usage
// statistics for each incoming HTTP request.
//
// It wraps the response writer to capture response size, extracts user/tenant
// identity from the request context, and atomically updates counters.
func (m *UsageStatsMiddleware) Handler(next http.Handler) http.Handler {
	m.ensureFlushLoop()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip configured paths (health checks, metrics, etc.).
		if m.skipPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Track request by endpoint + method.
		epKey := endpointKey{Method: r.Method, Endpoint: r.URL.Path}
		incrementCounter(&m.endpointCounts, epKey)

		// Track request by user/tenant from auth context.
		if m.config.EnableUserTracking {
			uid := ContextGetUserID(r.Context())
			tid := ContextGetTenantID(r.Context())
			if uid != "" || tid != "" {
				uKey := userKey{UserID: uid, TenantID: tid}
				incrementCounter(&m.userCounts, uKey)
			}
		}

		// Wrap response writer to capture bytes written.
		wrapped := &usageStatsResponseWriter{ResponseWriter: w}

		// Serve the request.
		next.ServeHTTP(wrapped, r)

		// Record response size for the endpoint and histogram.
		addCounter(&m.endpointBytes, epKey, wrapped.bytesWritten)
		bucket := responseBucketFor(wrapped.bytesWritten)
		incrementCounter(&m.sizeBuckets, bucket)
	})
}

// Stop stops the periodic flush goroutine and performs a final flush of all
// collected statistics. It should be called during server graceful shutdown.
// Calling Stop multiple times is safe (only the first call triggers shutdown).
func (m *UsageStatsMiddleware) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

// FlushNow triggers an immediate flush and reset of all collected statistics
// without waiting for the next periodic interval.
func (m *UsageStatsMiddleware) FlushNow() {
	m.flush()
}

// UsageStatsMiddlewareWrapper wraps usage stats middleware for use with router
// configuration following the same struct pattern as other middlewares.
type UsageStatsMiddlewareWrapper struct {
	*UsageStatsMiddleware
}

// NewUsageStatsMiddlewareWrapper creates a new wrapped usage stats middleware.
func NewUsageStatsMiddlewareWrapper(logger logging.Logger, config UsageStatsConfig) *UsageStatsMiddlewareWrapper {
	return &UsageStatsMiddlewareWrapper{
		UsageStatsMiddleware: NewUsageStatsMiddleware(logger, config),
	}
}

//Personal.AI order the ending
