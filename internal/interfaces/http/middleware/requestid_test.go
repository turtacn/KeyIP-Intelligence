// Phase 11 - 接口层: HTTP Middleware - 请求追踪 ID 中间件单元测试
// 序号: 275b
// 文件: internal/interfaces/http/middleware/requestid_test.go
// 测试用例:
//   - TestRequestID_GeneratesID: 无请求头时生成 UUID
//   - TestRequestID_ReusesHeader: 复用 X-Request-ID 请求头
//   - TestRequestID_SetsResponseHeader: 设置 X-Request-ID 响应头
//   - TestRequestID_InjectsContext: requestID 注入 context
//   - TestContextGetRequestID_FromMiddleware: ContextGetRequestID 正确提取
//   - TestContextGetRequestID_Empty: 无 requestID 时返回空字符串
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestRequestID_GeneratesID verifies that when no X-Request-ID header is present,
// the middleware generates a UUID v4.
func TestRequestID_GeneratesID(t *testing.T) {
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(w, r)

	reqID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, reqID, "X-Request-ID header should be set")
	// Verify it's a valid UUID
	_, err := uuid.Parse(reqID)
	assert.NoError(t, err, "generated ID should be a valid UUID v4")
}

// TestRequestID_ReusesHeader verifies that an existing X-Request-ID header is preserved.
func TestRequestID_ReusesHeader(t *testing.T) {
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Header.Set("X-Request-ID", "client-provided-id-123")
	handler.ServeHTTP(w, r)

	reqID := w.Header().Get("X-Request-ID")
	assert.Equal(t, "client-provided-id-123", reqID, "should reuse the client-provided request ID")
}

// TestRequestID_SetsResponseHeader verifies the response header is always set.
func TestRequestID_SetsResponseHeader(t *testing.T) {
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(w, r)

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"), "response must always have X-Request-ID")
}

// TestRequestID_InjectsContext verifies the request ID is injected into the context.
func TestRequestID_InjectsContext(t *testing.T) {
	var capturedID string
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = ContextGetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(w, r)

	responseID := w.Header().Get("X-Request-ID")
	assert.Equal(t, responseID, capturedID, "context request ID should match response header")
}

// TestContextGetRequestID_FromMiddleware verifies ContextGetRequestID retrieves
// the request ID set by the middleware.
func TestContextGetRequestID_FromMiddleware(t *testing.T) {
	var extractedID string
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedID = ContextGetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	r.Header.Set("X-Request-ID", "my-trace-id")
	handler.ServeHTTP(w, r)

	assert.Equal(t, "my-trace-id", extractedID, "ContextGetRequestID should return the correct ID")
}

// TestContextGetRequestID_Empty verifies ContextGetRequestID returns empty string
// when no request ID is in context.
func TestContextGetRequestID_Empty(t *testing.T) {
	// Test with a bare context that has no request ID
	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	id := ContextGetRequestID(ctx)
	assert.Empty(t, id, "should return empty string when no request ID in context")
}

// TestRequestID_FormatsHeader verifies X-Request-ID in response header uses exact value.
func TestRequestID_FormatsHeader(t *testing.T) {
	handler := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(w, r)

	header := w.Header().Get("X-Request-ID")
	// Should be a standard UUID v4 format: 8-4-4-4-12 hex digits
	parts := strings.Split(header, "-")
	assert.Len(t, parts, 5, "UUID v4 should have 5 parts separated by hyphens")
}

// TestNewRequestIDMiddleware_Handler verifies the struct-based wrapper works.
func TestNewRequestIDMiddleware_Handler(t *testing.T) {
	mw := NewRequestIDMiddleware()
	assert.NotNil(t, mw, "NewRequestIDMiddleware should return a non-nil instance")

	var capturedID string
	handler := mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = ContextGetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(w, r)

	responseID := w.Header().Get("X-Request-ID")
	assert.NotEmpty(t, responseID, "response should have X-Request-ID")
	assert.Equal(t, responseID, capturedID, "context request ID should match response header")
}

//Personal.AI order the ending
