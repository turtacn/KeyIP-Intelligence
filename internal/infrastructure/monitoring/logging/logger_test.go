package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	errs "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

// Helper to create a logger that writes to a buffer for verification
func newTestLogger(t *testing.T) (Logger, *zaptest.Buffer) {
	buf := &zaptest.Buffer{}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)
	core := zapcore.NewCore(encoder, buf, zapcore.DebugLevel)
	z := zap.New(core)
	return &zapLogger{z: z}, buf
}

func TestNewLogger_JSONFormat(t *testing.T) {
	cfg := LogConfig{
		Level:       LevelInfo,
		Format:      "json",
		OutputPaths: []string{"stdout"},
	}
	l, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.NotNil(t, l)
}

func TestNewLogger_ConsoleFormat(t *testing.T) {
	cfg := LogConfig{
		Level:       LevelDebug,
		Format:      "console",
		OutputPaths: []string{"stdout"},
	}
	l, err := NewLogger(cfg)
	require.NoError(t, err)
	assert.NotNil(t, l)
}

func TestNewLogger_InvalidConfig(t *testing.T) {
	cfg := LogConfig{
		OutputPaths: []string{}, // Invalid
	}
	l, err := NewLogger(cfg)
	assert.Error(t, err)
	assert.Nil(t, l)
}

func TestNewDefaultLogger_NotNil(t *testing.T) {
	l := NewDefaultLogger()
	assert.NotNil(t, l)
}

func TestNewDevelopmentLogger_NotNil(t *testing.T) {
	l := NewDevelopmentLogger()
	assert.NotNil(t, l)
}

func TestNewNopLogger_NotNil(t *testing.T) {
	l := NewNopLogger()
	assert.NotNil(t, l)
}

func TestNopLogger_AllMethodsNoOp(t *testing.T) {
	l := NewNopLogger()
	// Should not panic
	l.Debug("msg")
	l.Info("msg")
	l.Warn("msg")
	l.Error("msg")
	l.Fatal("msg")
}

func TestNopLogger_With_ReturnsSelf(t *testing.T) {
	l := NewNopLogger()
	l2 := l.With(String("k", "v"))
	assert.Equal(t, l, l2)
}

func TestNopLogger_WithContext_ReturnsSelf(t *testing.T) {
	l := NewNopLogger()
	l2 := l.WithContext(context.Background())
	assert.Equal(t, l, l2)
}

func TestNopLogger_WithError_ReturnsSelf(t *testing.T) {
	l := NewNopLogger()
	l2 := l.WithError(errors.New("err"))
	assert.Equal(t, l, l2)
}

func TestNopLogger_Sync_ReturnsNil(t *testing.T) {
	l := NewNopLogger()
	assert.NoError(t, l.Sync())
}

func TestZapLogger_Debug_WritesLog(t *testing.T) {
	l, buf := newTestLogger(t)
	l.Debug("debug msg")
	assert.Contains(t, buf.String(), "debug msg")
	assert.Contains(t, buf.String(), "\"level\":\"debug\"")
}

func TestZapLogger_Info_WritesLog(t *testing.T) {
	l, buf := newTestLogger(t)
	l.Info("info msg")
	assert.Contains(t, buf.String(), "info msg")
	assert.Contains(t, buf.String(), "\"level\":\"info\"")
}

func TestZapLogger_Warn_WritesLog(t *testing.T) {
	l, buf := newTestLogger(t)
	l.Warn("warn msg")
	assert.Contains(t, buf.String(), "warn msg")
	assert.Contains(t, buf.String(), "\"level\":\"warn\"")
}

func TestZapLogger_Error_WritesLog(t *testing.T) {
	l, buf := newTestLogger(t)
	l.Error("error msg")
	assert.Contains(t, buf.String(), "error msg")
	assert.Contains(t, buf.String(), "\"level\":\"error\"")
}

func TestZapLogger_With_AddsFields(t *testing.T) {
	l, buf := newTestLogger(t)
	l.With(String("foo", "bar")).Info("msg")
	assert.Contains(t, buf.String(), "\"foo\":\"bar\"")
}

func TestZapLogger_WithContext_ExtractsRequestID(t *testing.T) {
	l, buf := newTestLogger(t)
	ctx := WithRequestID(context.Background(), "req-123")
	l.WithContext(ctx).Info("msg")
	assert.Contains(t, buf.String(), "\"request_id\":\"req-123\"")
}

func TestZapLogger_WithError_AppError(t *testing.T) {
	l, buf := newTestLogger(t)
	appErr := errs.New(errs.ErrCodeInternal, "app error").WithRequestID("req-1")
	l.WithError(appErr).Error("msg")
	assert.Contains(t, buf.String(), "\"error_code\":\"COMMON_001\"")
	assert.Contains(t, buf.String(), "\"request_id\":\"req-1\"")
	assert.Contains(t, buf.String(), "\"error\":\"[COMMON_001] app error\"")
}

func TestZapLogger_WithError_StandardError(t *testing.T) {
	l, buf := newTestLogger(t)
	err := errors.New("std error")
	l.WithError(err).Error("msg")
	assert.Contains(t, buf.String(), "\"error\":\"std error\"")
}

func TestZapLogger_WithError_NilError(t *testing.T) {
	l, buf := newTestLogger(t)
	l.WithError(nil).Info("msg")
	// Should not add error field
	assert.NotContains(t, buf.String(), "\"error\"")
}

func TestSetGlobalLogger_UpdatesGlobal(t *testing.T) {
	orig := GetGlobalLogger()
	defer SetGlobalLogger(orig)

	l := NewNopLogger()
	SetGlobalLogger(l)
	assert.Equal(t, l, GetGlobalLogger())
}

func TestLogLevel_String(t *testing.T) {
	assert.Equal(t, "info", LevelInfo.String())
	assert.Equal(t, "debug", LevelDebug.String())
}

func TestParseLevel_Valid(t *testing.T) {
	l, err := ParseLevel("debug")
	assert.NoError(t, err)
	assert.Equal(t, LevelDebug, l)
}

func TestParseLevel_Invalid(t *testing.T) {
	_, err := ParseLevel("invalid")
	assert.Error(t, err)
}

func TestLogOperationDuration_FastOperation(t *testing.T) {
	l, buf := newTestLogger(t)
	start := time.Now()
	LogOperationDuration(l, "op", start)
	assert.Contains(t, buf.String(), "operation completed")
	assert.Contains(t, buf.String(), "\"level\":\"info\"")
	assert.Contains(t, buf.String(), "duration_ms")
}

func TestLogDatabaseQuery_Success(t *testing.T) {
	l, buf := newTestLogger(t)
	LogDatabaseQuery(l, "SELECT *", time.Millisecond, 10, nil)
	assert.Contains(t, buf.String(), "database query completed")
	assert.Contains(t, buf.String(), "\"query\":\"SELECT *\"")
}

func TestLogDatabaseQuery_Error(t *testing.T) {
	l, buf := newTestLogger(t)
	LogDatabaseQuery(l, "SELECT *", time.Millisecond, 0, errors.New("db error"))
	assert.Contains(t, buf.String(), "database query failed")
	assert.Contains(t, buf.String(), "\"level\":\"error\"")
}

func TestField_String(t *testing.T) {
	f := String("k", "v")
	assert.Equal(t, zapcore.FieldType(zapcore.StringType), zap.Field(f).Type)
}

func TestFieldConstants_Values(t *testing.T) {
	assert.Equal(t, "request_id", FieldRequestID)
}

//Personal.AI order the ending
