package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewLogger(t *testing.T) {
	cfg := LogConfig{
		Level:       LevelInfo,
		Format:      "json",
		OutputPaths: []string{"stdout"},
	}
	logger, err := NewLogger(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_Invalid(t *testing.T) {
	cfg := LogConfig{
		Format:      "invalid",
		OutputPaths: []string{"stdout"},
	}
	_, err := NewLogger(cfg)
	assert.Error(t, err)
}

func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()
	assert.NotNil(t, logger)
}

func TestNewDevelopmentLogger(t *testing.T) {
	logger := NewDevelopmentLogger()
	assert.NotNil(t, logger)
}

func TestNewNopLogger(t *testing.T) {
	logger := NewNopLogger()
	assert.NotNil(t, logger)
	logger.Info("test") // should not panic
}

func TestNopLogger_Fluent(t *testing.T) {
	logger := NewNopLogger()
	assert.Equal(t, logger, logger.With(String("k", "v")))
	assert.Equal(t, logger, logger.WithContext(context.Background()))
	assert.Equal(t, logger, logger.WithError(errors.New("err")))
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithTraceID(ctx, "trace-1")
	ctx = WithUserID(ctx, "user-1")

	assert.Equal(t, "req-1", RequestIDFromContext(ctx))
	assert.Equal(t, "trace-1", TraceIDFromContext(ctx))
	assert.Equal(t, "user-1", UserIDFromContext(ctx))
}

func TestGlobalLogger(t *testing.T) {
	original := GetGlobalLogger()
	newLogger := NewNopLogger()
	SetGlobalLogger(newLogger)
	assert.Equal(t, newLogger, GetGlobalLogger())
	assert.Equal(t, newLogger, L())
	SetGlobalLogger(original)
}

func TestLogLevel_String(t *testing.T) {
	assert.Equal(t, "info", LevelInfo.String())
	assert.Equal(t, "debug", LevelDebug.String())
}

func TestParseLevel(t *testing.T) {
	l, err := ParseLevel("debug")
	assert.NoError(t, err)
	assert.Equal(t, LevelDebug, l)

	_, err = ParseLevel("invalid")
	assert.Error(t, err)
}

func TestLogOperationDuration(t *testing.T) {
	logger := NewNopLogger()
	start := time.Now().Add(-2 * time.Second)
	LogOperationDuration(logger, "test-op", start)
}

func TestLogDatabaseQuery(t *testing.T) {
	logger := NewNopLogger()
	LogDatabaseQuery(logger, "SELECT * FROM patents", time.Millisecond*100, 1, nil)
	LogDatabaseQuery(logger, "SELECT * FROM patents", time.Millisecond*100, 0, errors.New("db error"))
}

func TestLogExternalCall(t *testing.T) {
	logger := NewNopLogger()
	LogExternalCall(logger, "service", "GET", "http://example.com", 200, time.Millisecond*100, nil)
}

func TestLogAIInference(t *testing.T) {
	logger := NewNopLogger()
	LogAIInference(logger, "model-v1", "predict", 1024, time.Second, nil)
}

func TestFieldConstructors(t *testing.T) {
	assert.Equal(t, "k", zap.Field(String("k", "v")).Key)
	assert.Equal(t, "k", zap.Field(Int("k", 1)).Key)
	assert.Equal(t, "k", zap.Field(Bool("k", true)).Key)
	assert.Equal(t, "error", zap.Field(Error(errors.New("e"))).Key)
}

func TestLogger_WithContext(t *testing.T) {
	logger := NewNopLogger()
	ctx := WithRequestID(context.Background(), "req-1")
	child := logger.WithContext(ctx)
	assert.NotNil(t, child)
}

func TestLogger_WithError(t *testing.T) {
	logger := NewNopLogger()
	child := logger.WithError(errors.New("err"))
	assert.NotNil(t, child)
}

func TestZapLogger_Sync(t *testing.T) {
	logger := NewDefaultLogger()
	err := logger.Sync()
	// Sync on stdout might return error in some environments (e.g. "sync /dev/stdout: invalid argument")
	// but it shouldn't panic.
	_ = err
}

//Personal.AI order the ending
