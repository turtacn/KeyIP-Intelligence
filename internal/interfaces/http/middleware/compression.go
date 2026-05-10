// Phase 11 - 接口层: HTTP Middleware - 响应压缩中间件
// 序号: 283
// 文件: internal/interfaces/http/middleware/compression.go
// 功能定位: 实现 HTTP 响应压缩中间件，支持 gzip 和 brotli 压缩算法
// 核心实现:
//   - 定义 CompressionConfig 结构体: Level, MinSize, ContentTypes, DisableBrotli
//   - 定义 DefaultCompressionConfig() CompressionConfig，返回安全的默认配置
//   - 实现 Compression(config) func(http.Handler) http.Handler
//   - 根据 Accept-Encoding 自动选择压缩算法（brotli 优先）
//   - 仅压缩 text/* 和 application/json 响应
//   - 可配置压缩级别和最小响应大小阈值
//   - 使用 compressionWriter 包装 ResponseWriter，实现按需压缩
//
// 性能考量:
//   - 小于 MinSize 的响应不压缩，避免小对象的压缩开销
//   - Vary: Accept-Encoding 头确保缓存正确性
//   - 支持 Hijack 和 Flush 接口，兼容 WebSocket 和流式响应
//
// 依赖关系:
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

// CompressionConfig holds configuration for HTTP response compression middleware.
type CompressionConfig struct {
	// Level is the compression level for both gzip and brotli.
	// For gzip: -1 (default/6), 0 (no compression), 1 (best speed) to 9 (best compression).
	// For brotli: 0 (no compression) to 11 (best compression), -1 maps to brotli default (4).
	// Default: -1 (algorithm-specific default).
	Level int

	// MinSize is the minimum response body size in bytes to trigger compression.
	// Responses smaller than this threshold are not compressed.
	// Default: 1024 (1 KB).
	MinSize int

	// ContentTypes is the list of content type prefixes to compress.
	// A response is compressed if its Content-Type starts with any of these prefixes.
	// Default: ["text/", "application/json"].
	ContentTypes []string

	// DisableBrotli disables brotli compression even if the client supports it.
	// This is useful if the brotli dependency is not desired in certain deployments.
	DisableBrotli bool
}

// DefaultCompressionConfig returns a default compression configuration.
// By default it compresses text and JSON responses >= 1 KB using algorithm-specific
// default compression levels.
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Level:   -1, // algorithm-specific default
		MinSize: 1024,
		ContentTypes: []string{
			"text/",
			"application/json",
		},
		DisableBrotli: false,
	}
}

// Compression returns HTTP middleware that compresses response bodies based on
// the client's Accept-Encoding header. Brotli is preferred over gzip when both
// are supported. Only responses whose Content-Type matches the configured type
// prefixes and whose body exceeds MinSize are compressed.
func Compression(config CompressionConfig) func(http.Handler) http.Handler {
	// Normalize config
	if config.MinSize <= 0 {
		config.MinSize = 1024
	}
	if len(config.ContentTypes) == 0 {
		config.ContentTypes = []string{"text/", "application/json"}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip compression for HEAD requests (no body)
			if r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			// Check Accept-Encoding header
			ae := r.Header.Get("Accept-Encoding")
			if ae == "" {
				next.ServeHTTP(w, r)
				return
			}

			useBrotli := !config.DisableBrotli && strings.Contains(ae, "br")
			useGzip := strings.Contains(ae, "gzip")

			if !useGzip && !useBrotli {
				next.ServeHTTP(w, r)
				return
			}

			// Set Vary header to indicate response varies on Accept-Encoding
			w.Header().Add("Vary", "Accept-Encoding")

			// Wrap response writer with compression support
			cw := &compressionWriter{
				ResponseWriter: w,
				config:         config,
				useGzip:        useGzip && !useBrotli, // prefer brotli over gzip
				useBrotli:      useBrotli,
				statusCode:     http.StatusOK,
			}

			defer cw.close()

			next.ServeHTTP(cw, r)
		})
	}
}

// compressionWriter wraps http.ResponseWriter to provide on-the-fly response compression.
// It buffers the response body up to MinSize bytes, then transparently switches to
// compressed streaming if the content meets the criteria.
type compressionWriter struct {
	http.ResponseWriter
	config      CompressionConfig
	useGzip     bool
	useBrotli   bool
	buf         bytes.Buffer
	compressed  bool
	wroteHeader bool
	statusCode  int
	contentType string
	compressor  io.WriteCloser
}

// WriteHeader captures the status code and content type without forwarding to the
// underlying ResponseWriter. The actual header write is deferred until compression
// starts or close() is called.
func (w *compressionWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = code
	w.contentType = w.Header().Get("Content-Type")
}

// Write implements the standard Write method. It buffers data until MinSize is
// reached (if content type matches), then transparently switches to compressed
// streaming for the remainder of the response.
func (w *compressionWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	if w.compressed {
		return w.compressor.Write(b)
	}

	// Check if adding this data pushes us over MinSize
	if w.shouldCompress() && w.buf.Len()+len(b) >= w.config.MinSize {
		w.startCompression()

		// Flush any previously buffered data through the compressor
		if w.buf.Len() > 0 {
			if _, err := w.compressor.Write(w.buf.Bytes()); err != nil {
				return 0, err
			}
			w.buf.Reset()
		}

		return w.compressor.Write(b)
	}

	// Still buffering: accumulate data
	return w.buf.Write(b)
}

// close finalizes the response. If compression was activated, it closes the
// compressor to flush remaining compressed data. Otherwise, it writes the
// buffered response directly to the underlying ResponseWriter.
func (w *compressionWriter) close() {
	if w.compressed {
		// Close compressor to flush any remaining compressed data
		_ = w.compressor.Close()
		return
	}

	// Never compressed — write buffered data directly
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	// Skip body for responses that must not have one
	if w.statusCode == http.StatusNoContent ||
		w.statusCode == http.StatusNotModified ||
		w.statusCode/100 == 1 { // 1xx informational
		w.ResponseWriter.WriteHeader(w.statusCode)
		return
	}

	w.ResponseWriter.WriteHeader(w.statusCode)
	if w.buf.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.buf.Bytes())
	}
}

// shouldCompress checks whether the response content type matches the configured
// compressible types.
func (w *compressionWriter) shouldCompress() bool {
	if w.contentType == "" {
		return false
	}
	// Strip optional charset and parameters for prefix matching
	baseType := w.contentType
	if idx := strings.IndexByte(baseType, ';'); idx >= 0 {
		baseType = strings.TrimSpace(baseType[:idx])
	}
	for _, ct := range w.config.ContentTypes {
		if strings.HasPrefix(baseType, ct) {
			return true
		}
	}
	return false
}

// startCompression activates compressed streaming. It sets Content-Encoding,
// removes Content-Length, and initializes the chosen compressor.
func (w *compressionWriter) startCompression() {
	w.compressed = true

	// Remove Content-Length since it will change post-compression
	w.Header().Del("Content-Length")

	if w.useBrotli {
		w.Header().Set("Content-Encoding", "br")
		w.compressor = brotli.NewWriterLevel(w.ResponseWriter, w.getBrotliLevel())
	} else {
		w.Header().Set("Content-Encoding", "gzip")
		var err error
		w.compressor, err = gzip.NewWriterLevel(w.ResponseWriter, w.getGzipLevel())
		if err != nil {
			// If level is invalid, fall back to default level
			w.compressor = gzip.NewWriter(w.ResponseWriter)
		}
	}

	// Forward the captured status code
	w.ResponseWriter.WriteHeader(w.statusCode)
}

// getGzipLevel maps the config compression level to a valid gzip level.
// Returns gzip.DefaultCompression (6) when config level is -1.
func (w *compressionWriter) getGzipLevel() int {
	if w.config.Level == -1 {
		return gzip.DefaultCompression // 6
	}
	// Clamp to valid gzip range: gzip.HuffmanOnly (-2) to gzip.BestCompression (9)
	if w.config.Level < gzip.HuffmanOnly {
		return gzip.HuffmanOnly
	}
	if w.config.Level > gzip.BestCompression {
		return gzip.BestCompression
	}
	return w.config.Level
}

// getBrotliLevel maps the config compression level to a valid brotli level.
// Returns brotli.DefaultCompression (4) when config level is -1 or 0.
func (w *compressionWriter) getBrotliLevel() int {
	if w.config.Level <= 0 {
		return brotli.DefaultCompression // 4
	}
	// Clamp to valid brotli range: 1 to 11
	if w.config.Level > 11 {
		return 11
	}
	return w.config.Level
}

// Hijack implements http.Hijacker to support WebSocket upgrades through the
// compression wrapper. It finalizes any pending compression and passes through
// to the underlying ResponseWriter.
func (w *compressionWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		// Flush any buffered data before hijacking
		if !w.compressed && w.buf.Len() > 0 {
			w.ResponseWriter.WriteHeader(w.statusCode)
			_, _ = w.ResponseWriter.Write(w.buf.Bytes())
			w.buf.Reset()
		}
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("compressionWriter: underlying ResponseWriter does not implement http.Hijacker")
}

// Flush implements http.Flusher to support streaming responses through the
// compression wrapper.
func (w *compressionWriter) Flush() {
	if w.compressed {
		if flusher, ok := w.compressor.(interface{ Flush() error }); ok {
			_ = flusher.Flush()
		}
	} else if w.buf.Len() > 0 {
		// If not compressed yet but we have buffered data, we can't flush it
		// because we might still decide to compress later. Do nothing.
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// CompressionMiddleware wraps compression middleware for use with router configuration.
type CompressionMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewCompressionMiddleware creates a new CompressionMiddleware with the given config.
func NewCompressionMiddleware(config CompressionConfig) *CompressionMiddleware {
	return &CompressionMiddleware{
		handler: Compression(config),
	}
}

// Handler returns the middleware handler function.
func (m *CompressionMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

//Personal.AI order the ending
