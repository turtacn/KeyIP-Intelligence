// Phase 11 - 接口层: HTTP Middleware - 请求日志中间件单元测试
// 序号: 277
// 文件: internal/interfaces/http/middleware/logging_test.go
// 测试用例:
//   - TestRequestLogging_BasicRequest: 基本请求日志记录
//   - TestRequestLogging_StatusCapture: 状态码正确捕获
//   - TestRequestLogging_BytesCapture: 响应字节数正确捕获
//   - TestRequestLogging_Duration: 耗时记录
//   - TestRequestLogging_SkipPaths: 跳过配置路径
//   - TestRequestLogging_SlowRequest: 慢请求 Warn 级别
//   - TestRequestLogging_ServerError: 5xx Error 级别
//   - TestRequestLogging_ClientError: 4xx Warn 级别
//   - TestRequestLogging_RequestID: 请求 ID 传递
//   - TestWrappedResponseWriter_DefaultStatus: 默认 200
//   - TestWrappedResponseWriter_WriteHeader: 状态码捕获
//   - TestWrappedResponseWriter_DoubleWriteHeader: 只记录第一次
//   - TestDefaultLoggingConfig: 默认配置值
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"context"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// captureLogger captures log calls for assertion.
type captureLogger struct {
	mock.Mock
	lastLevel string
	lastMsg   string
	lastFields []logging.Field
}

func (l *captureLogger) Debug(msg string, fields ...logging.Field) {
	l.lastLevel = "debug"
	l.lastMsg = msg
	l.lastFields = fields
}
func (l *captureLogger) Info(msg string, fields ...logging.Field) {
	l.lastLevel = "info"
	l.lastMsg = msg
	l.lastFields = fields
}
func (l *captureLogger) Warn(msg string, fields ...logging.Field) {
	l.lastLevel = "warn"
	l.lastMsg = msg
	l.lastFields = fields
}
func (l *captureLogger) Error(msg string, fields ...logging.Field) {
	l.lastLevel = "error"
	l.lastMsg = msg
	l.lastFields = fields
}
func (l *captureLogger) With(fields ...logging.Field) logging.Logger { return l }
func (l *captureLogger) WithContext(ctx context.Context) logging.Logger { return l }
func (l *captureLogger) WithError(err error) logging.Logger { return l }
func (l *captureLogger) Fatal(msg string, fields ...logging.Field) { l.lastLevel = "fatal"; l.lastMsg = msg; l.lastFields = fields }
func (l *captureLogger) Sync() error { return nil }

func TestRequestLogging_BasicRequest(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = nil // don't skip anything

	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/patents", nil)
	r.Header.Set("X-Request-ID", "req-123")
	handler.ServeHTTP(w, r)

	assert.Equal(t, "info", logger.lastLevel)
	assert.Contains(t, logger.lastMsg, "HTTP request completed")

	// Verify fields contain expected keys
	args := logger.lastFields
	argMap := argsToMap(args)
	assert.Equal(t, "GET", argMap["method"])
	assert.Equal(t, "/api/v1/patents", argMap["path"])
	assert.Equal(t, int64(200), argMap["status"])
	assert.Equal(t, "req-123", argMap["request_id"])
}

func TestRequestLogging_StatusCapture(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = nil

	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/patents", nil)
	handler.ServeHTTP(w, r)

	argMap := argsToMap(logger.lastFields)
	assert.Equal(t, int64(201), argMap["status"])
}

func TestRequestLogging_BytesCapture(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = nil

	body := "response-body-content"
	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, r)

	argMap := argsToMap(logger.lastFields)
	assert.Equal(t, int64(len(body)), argMap["bytes"])
}

func TestRequestLogging_SkipPaths(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = []string{"/health"}

	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health", nil)
	handler.ServeHTTP(w, r)

	// Logger should not have been called
	assert.Empty(t, logger.lastLevel)
}

func TestRequestLogging_SlowRequest(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = nil
	config.SlowThreshold = 10 * time.Millisecond

	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/slow", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "warn", logger.lastLevel)
	assert.Contains(t, logger.lastMsg, "slow")
}

func TestRequestLogging_ServerError(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = nil

	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/error", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "error", logger.lastLevel)
	assert.Contains(t, logger.lastMsg, "server error")
}

func TestRequestLogging_ClientError(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = nil

	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/missing", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, "warn", logger.lastLevel)
	assert.Contains(t, logger.lastMsg, "client error")
}

func TestRequestLogging_RequestID(t *testing.T) {
	logger := &captureLogger{}
	config := DefaultLoggingConfig()
	config.SkipPaths = nil

	handler := RequestLogging(logger, config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("X-Request-ID", "unique-req-456")
	handler.ServeHTTP(w, r)

	argMap := argsToMap(logger.lastFields)
	assert.Equal(t, "unique-req-456", argMap["request_id"])
}

// --- wrappedResponseWriter Tests ---

func TestWrappedResponseWriter_DefaultStatus(t *testing.T) {
	w := httptest.NewRecorder()
	wrapped := newWrappedResponseWriter(w)

	// Write without calling WriteHeader
	wrapped.Write([]byte("data"))

	assert.Equal(t, http.StatusOK, wrapped.statusCode)
	assert.True(t, wrapped.wroteHeader)
}

func TestWrappedResponseWriter_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	wrapped := newWrappedResponseWriter(w)

	wrapped.WriteHeader(http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, wrapped.statusCode)
	assert.True(t, wrapped.wroteHeader)
}

func TestWrappedResponseWriter_DoubleWriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	wrapped := newWrappedResponseWriter(w)

	wrapped.WriteHeader(http.StatusCreated)
	wrapped.WriteHeader(http.StatusInternalServerError) // should be ignored

	assert.Equal(t, http.StatusCreated, wrapped.statusCode)
}

func TestDefaultLoggingConfig(t *testing.T) {
	config := DefaultLoggingConfig()

	assert.Contains(t, config.SkipPaths, "/health")
	assert.Contains(t, config.SkipPaths, "/healthz")
	assert.False(t, config.LogRequestBody)
	assert.False(t, config.LogResponseBody)
	assert.Equal(t, 3*time.Second, config.SlowThreshold)
	assert.Equal(t, 1024, config.MaxBodyLogSize)
}

// argsToMap converts logging.Field slice to a map for easy assertion.
// It extracts values from zap.Field based on their Type.
func argsToMap(fields []logging.Field) map[string]interface{} {
	m := make(map[string]interface{})
	for _, f := range fields {
		// zapcore.Field stores values differently based on Type
		// String types use String field, integers use Integer field, others use Interface
		switch {
		case f.String != "":
			m[f.Key] = f.String
		case f.Integer != 0:
			m[f.Key] = f.Integer // keep as int64
		case f.Interface != nil:
			m[f.Key] = f.Interface
		default:
			m[f.Key] = f.String // fallback for empty strings
		}
	}
	return m
}

//Personal.AI order the ending
