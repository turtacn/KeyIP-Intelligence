// Phase 11 - 接口层: HTTP Middleware - 响应压缩中间件测试
// 序号: 284
// 文件: internal/interfaces/http/middleware/compression_test.go
// 功能定位: 验证 HTTP 响应压缩中间件的正确性，包括压缩算法选择、内容类型过滤、大小阈值等
// 核心测试覆盖:
//   - 无 Accept-Encoding 时不压缩
//   - gzip 压缩
//   - brotli 压缩（优先于 gzip）
//   - 小于 MinSize 的响应不压缩
//   - 非 text/ 和 application/json 内容不压缩
//   - Content-Encoding 和 Vary 头的正确设置
//   - 压缩级别配置
//   - Hijack 接口
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// compressTestHandler returns a simple handler that writes the given content type and body.
func compressTestHandler(contentType, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})
}

// TestCompression_NoAcceptEncoding verifies that responses are not compressed
// when the client does not send an Accept-Encoding header.
func TestCompression_NoAcceptEncoding(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1 // ensure any response would be compressed if it triggers

	handler := Compression(cfg)(compressTestHandler("text/plain", "hello world"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "hello world", rec.Body.String())
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
}

// TestCompression_Gzip verifies that responses are gzip-compressed when the
// client sends Accept-Encoding: gzip.
func TestCompression_Gzip(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	handler := Compression(cfg)(compressTestHandler("text/plain", "hello world gzip test content"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")

	// Decompress and verify
	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	defer gr.Close()

	decoded, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, "hello world gzip test content", string(decoded))

	// Verify compressed size is smaller than original
	assert.Less(t, rec.Body.Len(), 32, "compressed response should be smaller")
}

// TestCompression_Brotli verifies brotli compression when the client sends
// Accept-Encoding: br.
func TestCompression_Brotli(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	handler := Compression(cfg)(compressTestHandler("text/plain", "hello world brotli test content for compression"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "br", rec.Header().Get("Content-Encoding"))
	assert.Contains(t, rec.Header().Get("Vary"), "Accept-Encoding")

	// Decompress and verify
	decoded, err := io.ReadAll(brotli.NewReader(rec.Body))
	require.NoError(t, err)
	assert.Equal(t, "hello world brotli test content for compression", string(decoded))
}

// TestCompression_BrotliPreferred verifies that brotli is preferred over gzip
// when the client sends Accept-Encoding: gzip, br.
func TestCompression_BrotliPreferred(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	handler := Compression(cfg)(compressTestHandler("text/plain", "hello world brotli preferred test"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "br", rec.Header().Get("Content-Encoding"),
		"brotli should be preferred over gzip when both are supported")
}

// TestCompression_MinSizeThreshold verifies that responses smaller than MinSize
// are not compressed.
func TestCompression_MinSizeThreshold(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 100 // only compress responses >= 100 bytes

	handler := Compression(cfg)(compressTestHandler("text/plain", "small"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Response should be too small to compress
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Equal(t, "small", rec.Body.String())
}

// TestCompression_ContentTypeFiltering verifies that only configured content
// types are compressed.
func TestCompression_ContentTypeFiltering(t *testing.T) {
	tests := []struct {
		name           string
		contentType    string
		shouldCompress bool
	}{
		{"text/plain is compressed", "text/plain; charset=utf-8", true},
		{"text/html is compressed", "text/html", true},
		{"text/css is compressed", "text/css", true},
		{"application/json is compressed", "application/json", true},
		{"application/json with charset", "application/json; charset=utf-8", true},
		{"application/xml is not compressed", "application/xml", false},
		{"image/png is not compressed", "image/png", false},
		{"application/octet-stream is not compressed", "application/octet-stream", false},
	}

	body := "this is a response body that exceeds the minimum size threshold for compression testing purposes"
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := Compression(cfg)(compressTestHandler(tt.contentType, body))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if tt.shouldCompress {
				assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"),
					"expected compression for %s", tt.contentType)

				// Verify body decompresses correctly
				gr, err := gzip.NewReader(rec.Body)
				require.NoError(t, err)
				decoded, err := io.ReadAll(gr)
				require.NoError(t, err)
				assert.Equal(t, body, string(decoded))
			} else {
				assert.Empty(t, rec.Header().Get("Content-Encoding"),
					"expected no compression for %s", tt.contentType)
				assert.Equal(t, body, rec.Body.String())
			}
		})
	}
}

// TestCompression_NoBodyMethods verifies that methods with no body (HEAD, 204, 304)
// are not compressed.
func TestCompression_NoBodyMethods(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	t.Run("HEAD request is not compressed", func(t *testing.T) {
		handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodHead, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
	})

	t.Run("204 No Content is not compressed", func(t *testing.T) {
		handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})

	t.Run("304 Not Modified is not compressed", func(t *testing.T) {
		handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotModified)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Empty(t, rec.Header().Get("Content-Encoding"))
		assert.Equal(t, http.StatusNotModified, rec.Code)
	})
}

// TestCompression_DisableBrotli verifies that brotli is disabled when
// DisableBrotli is set to true.
func TestCompression_DisableBrotli(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1
	cfg.DisableBrotli = true

	handler := Compression(cfg)(compressTestHandler("text/plain", "hello world when brotli is disabled"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should use gzip, not brotli
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

// TestCompression_Levels verifies that compression levels are properly passed
// to the compressor and produce different results.
func TestCompression_Levels(t *testing.T) {
	body := "This is a test body that will be compressed at various levels to verify " +
		"that the compression level setting works correctly across gzip and brotli."

	t.Run("gzip level 1 (best speed) vs level 9 (best compression)", func(t *testing.T) {
		var size1, size9 int

		for _, level := range []int{1, 9} {
			cfg := DefaultCompressionConfig()
			cfg.MinSize = 1
			cfg.Level = level

			handler := Compression(cfg)(compressTestHandler("text/plain", body))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if level == 1 {
				size1 = rec.Body.Len()
			} else {
				size9 = rec.Body.Len()
			}
		}

		// Level 9 should produce smaller (or equal) output than level 1
		t.Logf("gzip level=1 size=%d, level=9 size=%d", size1, size9)
		assert.LessOrEqual(t, size9, size1, "level 9 should produce smaller output than level 1")
	})

	t.Run("brotli level 1 (best speed) vs level 11 (best compression)", func(t *testing.T) {
		var size1, size11 int

		for _, level := range []int{1, 11} {
			cfg := DefaultCompressionConfig()
			cfg.MinSize = 1
			cfg.Level = level

			handler := Compression(cfg)(compressTestHandler("text/plain", body))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "br")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if level == 1 {
				size1 = rec.Body.Len()
			} else {
				size11 = rec.Body.Len()
			}
		}

		// Level 11 should produce smaller (or equal) output than level 1
		t.Logf("brotli level=1 size=%d, level=11 size=%d", size1, size11)
		assert.LessOrEqual(t, size11, size1, "level 11 should produce smaller output than level 1")
	})
}

// TestCompression_CustomConfig verifies that custom compression configuration
// (custom content types, min size) is correctly applied.
func TestCompression_CustomConfig(t *testing.T) {
	cfg := CompressionConfig{
		Level:   6,
		MinSize: 50,
		ContentTypes: []string{
			"application/xml",
		},
		DisableBrotli: false,
	}

	handler := Compression(cfg)(compressTestHandler("application/xml", "<root><item>longer value that exceeds the minimum size threshold for compression testing</item></root>"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should compress application/xml since we configured it
	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	// Verify it actually decompresses correctly
	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	decoded, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Contains(t, string(decoded), "<root><item>")
}

// TestCompression_Hijack verifies that the Hijack interface works correctly
// for WebSocket-like scenarios.
func TestCompression_Hijack(t *testing.T) {
	// Create a ResponseRecorder that implements http.Hijacker
	hijackableRW := &hijackableResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
	}

	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		assert.True(t, ok, "ResponseWriter should implement http.Hijacker")
		if ok {
			conn, bufrw, err := hijacker.Hijack()
			assert.NoError(t, err)
			assert.NotNil(t, conn)
			assert.NotNil(t, bufrw)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	handler.ServeHTTP(hijackableRW, req)
}

// TestCompression_ResponseWriterInterface verifies basic ResponseWriter
// interface compliance.
func TestCompression_ResponseWriterInterface(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	t.Run("WriteHeader sets status code on compressed response", func(t *testing.T) {
		handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created resource response body for testing"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
	})

	t.Run("WriteHeader sets status code on uncompressed response", func(t *testing.T) {
		handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("not found"))
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Empty(t, rec.Header().Get("Content-Encoding"))
	})
}

// TestCompression_EmptyBody verifies that responses with an empty body are
// not compressed.
func TestCompression_EmptyBody(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Empty(t, rec.Body.String())
}

// TestCompression_MultipleWrites verifies that response streaming with
// multiple Write calls compresses correctly.
func TestCompression_MultipleWrites(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MinSize = 1

	handler := Compression(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 10; i++ {
			_, _ = w.Write([]byte(fmt.Sprintf("chunk %d ", i)))
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	gr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	decoded, err := io.ReadAll(gr)
	require.NoError(t, err)
	assert.Equal(t, "chunk 0 chunk 1 chunk 2 chunk 3 chunk 4 chunk 5 chunk 6 chunk 7 chunk 8 chunk 9 ", string(decoded))
}

// TestCompressionMiddlewareStruct verifies that the CompressionMiddleware
// struct follows the existing middleware pattern.
func TestCompressionMiddlewareStruct(t *testing.T) {
	cfg := DefaultCompressionConfig()
	mw := NewCompressionMiddleware(cfg)
	require.NotNil(t, mw)

	// Use a body larger than the default MinSize (1024) to ensure compression triggers
	bigBody := string(make([]byte, 1100))
	handler := mw.Handler(compressTestHandler("text/plain", bigBody))
	require.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
}

// --- Test Helpers ---

// hijackableResponseRecorder wraps httptest.ResponseRecorder to implement http.Hijacker
// for testing purposes.
type hijackableResponseRecorder struct {
	*httptest.ResponseRecorder
}

// Hijack implements http.Hijacker with a simple in-memory pipe.
func (h *hijackableResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	server, client := net.Pipe()
	brw := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
	return client, brw, nil
}

//Personal.AI order the ending
