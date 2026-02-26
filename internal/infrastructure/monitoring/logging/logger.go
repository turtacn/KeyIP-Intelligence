package logging

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger is the interface for structured logging.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	Fatal(msg string, fields ...Field)
	With(fields ...Field) Logger
	WithContext(ctx context.Context) Logger
	WithError(err error) Logger
	Sync() error
}

// Field is a wrapper around zap.Field.
type Field zap.Field

// Field constructors
func String(key, val string) Field      { return Field(zap.String(key, val)) }
func Int(key string, val int) Field        { return Field(zap.Int(key, val)) }
func Int64(key string, val int64) Field    { return Field(zap.Int64(key, val)) }
func Float64(key string, val float64) Field { return Field(zap.Float64(key, val)) }
func Bool(key string, val bool) Field      { return Field(zap.Bool(key, val)) }
func Duration(key string, val time.Duration) Field { return Field(zap.Duration(key, val)) }
func Time(key string, val time.Time) Field { return Field(zap.Time(key, val)) }
func Error(err error) Field                { return Field(zap.Error(err)) }
func Err(err error) Field                  { return Error(err) }
func Any(key string, val interface{}) Field { return Field(zap.Any(key, val)) }
func Stringer(key string, val fmt.Stringer) Field { return Field(zap.Stringer(key, val)) }

// LogLevel defines the severity of a log message.
type LogLevel int8

const (
	LevelDebug LogLevel = -1
	LevelInfo  LogLevel = 0
	LevelWarn  LogLevel = 1
	LevelError LogLevel = 2
	LevelFatal LogLevel = 5
)

func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	case LevelFatal:
		return "fatal"
	default:
		return fmt.Sprintf("level(%d)", l)
	}
}

// ParseLevel parses a string into a LogLevel.
func ParseLevel(s string) (LogLevel, error) {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return LevelInfo, err
	}
	return LogLevel(l), nil
}

// LogConfig defines the configuration for the logger.
type LogConfig struct {
	Level             LogLevel `json:"level"`
	Format            string   `json:"format"` // json or console
	OutputPaths       []string `json:"output_paths"`
	ErrorOutputPaths  []string `json:"error_output_paths"`
	EnableCaller      bool     `json:"enable_caller"`
	EnableStacktrace  bool     `json:"enable_stacktrace"`
	SamplingInitial   int      `json:"sampling_initial"`
	SamplingThereafter int      `json:"sampling_thereafter"`
	MaxSize           int      `json:"max_size"`
	MaxBackups        int      `json:"max_backups"`
	MaxAge            int      `json:"max_age"`
	Compress          bool     `json:"compress"`
	ServiceName       string   `json:"service_name"`
	Environment       string   `json:"environment"`
}

// Validate checks if the LogConfig is valid.
func (c LogConfig) Validate() error {
	if c.Format != "json" && c.Format != "console" && c.Format != "" {
		return fmt.Errorf("invalid log format: %s", c.Format)
	}
	if len(c.OutputPaths) == 0 {
		return fmt.Errorf("output_paths cannot be empty")
	}
	return nil
}

type zapLogger struct {
	z *zap.Logger
}

func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.z.Debug(msg, l.toZapFields(fields)...)
}

func (l *zapLogger) Info(msg string, fields ...Field) {
	l.z.Info(msg, l.toZapFields(fields)...)
}

func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.z.Warn(msg, l.toZapFields(fields)...)
}

func (l *zapLogger) Error(msg string, fields ...Field) {
	l.z.Error(msg, l.toZapFields(fields)...)
}

func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.z.Fatal(msg, l.toZapFields(fields)...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{z: l.z.With(l.toZapFields(fields)...)}
}

func (l *zapLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return l
	}
	var fields []Field
	if rid := RequestIDFromContext(ctx); rid != "" {
		fields = append(fields, String(FieldRequestID, rid))
	}
	if tid := TraceIDFromContext(ctx); tid != "" {
		fields = append(fields, String(FieldTraceID, tid))
	}
	if sid := SpanIDFromContext(ctx); sid != "" {
		fields = append(fields, String(FieldSpanID, sid))
	}
	if uid := UserIDFromContext(ctx); uid != "" {
		fields = append(fields, String(FieldUserID, uid))
	}
	if sid := SessionIDFromContext(ctx); sid != "" {
		fields = append(fields, String(FieldSessionID, sid))
	}
	if len(fields) > 0 {
		return l.With(fields...)
	}
	return l
}

func (l *zapLogger) WithError(err error) Logger {
	if err == nil {
		return l
	}
	if ae, ok := errors.GetAppError(err); ok {
		return l.With(
			String(FieldErrorCode, string(ae.Code)),
			String(FieldModule, ae.Module),
			String(FieldRequestID, ae.RequestID),
			String("internal_message", ae.InternalMessage),
			Error(err),
		)
	}
	return l.With(Error(err))
}

func (l *zapLogger) Sync() error {
	return l.z.Sync()
}

func (l *zapLogger) toZapFields(fields []Field) []zap.Field {
	zf := make([]zap.Field, len(fields))
	for i, f := range fields {
		zf[i] = zap.Field(f)
	}
	return zf
}

type contextKey string

const (
	ContextKeyRequestID contextKey = "request_id"
	ContextKeyTraceID   contextKey = "trace_id"
	ContextKeySpanID    contextKey = "span_id"
	ContextKeyUserID    contextKey = "user_id"
	ContextKeySessionID contextKey = "session_id"
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, id)
}
func WithTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextKeyTraceID, id)
}
func WithSpanID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextKeySpanID, id)
}
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, id)
}
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextKeySessionID, id)
}

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return v
	}
	return ""
}
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyTraceID).(string); ok {
		return v
	}
	return ""
}
func SpanIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeySpanID).(string); ok {
		return v
	}
	return ""
}
func UserIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyUserID).(string); ok {
		return v
	}
	return ""
}
func SessionIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeySessionID).(string); ok {
		return v
	}
	return ""
}

// Constructors

func NewLogger(cfg LogConfig) (Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	var encoder zapcore.Encoder
	var encoderConfig zapcore.EncoderConfig
	if cfg.Format == "console" {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var writer zapcore.WriteSyncer
	var syncers []zapcore.WriteSyncer

	for _, path := range cfg.OutputPaths {
		switch path {
		case "stdout":
			syncers = append(syncers, zapcore.AddSync(os.Stdout))
		case "stderr":
			syncers = append(syncers, zapcore.AddSync(os.Stderr))
		default:
			// Treat as file path with rotation
			lumberjackLogger := &lumberjack.Logger{
				Filename:   path,
				MaxSize:    cfg.MaxSize,
				MaxBackups: cfg.MaxBackups,
				MaxAge:     cfg.MaxAge,
				Compress:   cfg.Compress,
			}
			syncers = append(syncers, zapcore.AddSync(lumberjackLogger))
		}
	}

	// Re-using zap's internal logic for stdout/stderr to be safe
	if len(cfg.OutputPaths) == 1 && (cfg.OutputPaths[0] == "stdout" || cfg.OutputPaths[0] == "stderr") {
		// Just use zap.Config for simplicity if it's just std streams
		zCfg := zap.Config{
			Level:       zap.NewAtomicLevelAt(zapcore.Level(cfg.Level)),
			Development: cfg.Format == "console",
			Sampling: &zap.SamplingConfig{
				Initial:    cfg.SamplingInitial,
				Thereafter: cfg.SamplingThereafter,
			},
			Encoding:      cfg.Format,
			EncoderConfig: encoderConfig,
			OutputPaths:   cfg.OutputPaths,
			ErrorOutputPaths: cfg.ErrorOutputPaths,
		}
		if zCfg.Encoding == "" {
			zCfg.Encoding = "json"
		}
		z, err := zCfg.Build()
		if err != nil {
			return nil, err
		}
		if cfg.ServiceName != "" || cfg.Environment != "" {
			z = z.With(zap.String("service_name", cfg.ServiceName), zap.String("environment", cfg.Environment))
		}
		return &zapLogger{z: z}, nil
	}

	writer = zapcore.NewMultiWriteSyncer(syncers...)
	core := zapcore.NewCore(encoder, writer, zap.NewAtomicLevelAt(zapcore.Level(cfg.Level)))

	opts := []zap.Option{}
	if cfg.EnableCaller {
		opts = append(opts, zap.AddCaller())
	}
	if cfg.EnableStacktrace {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	z := zap.New(core, opts...)
	if cfg.ServiceName != "" || cfg.Environment != "" {
		z = z.With(zap.String("service_name", cfg.ServiceName), zap.String("environment", cfg.Environment))
	}

	return &zapLogger{z: z}, nil
}

func NewDefaultLogger() Logger {
	l, _ := NewLogger(LogConfig{
		Level:       LevelInfo,
		Format:      "json",
		OutputPaths: []string{"stdout"},
		Environment: "prod",
	})
	return l
}

func NewDevelopmentLogger() Logger {
	l, _ := NewLogger(LogConfig{
		Level:       LevelDebug,
		Format:      "console",
		OutputPaths: []string{"stdout"},
		Environment: "dev",
	})
	return l
}

type nopLogger struct{}

func (n *nopLogger) Debug(msg string, fields ...Field) {}
func (n *nopLogger) Info(msg string, fields ...Field)  {}
func (n *nopLogger) Warn(msg string, fields ...Field)  {}
func (n *nopLogger) Error(msg string, fields ...Field) {}
func (n *nopLogger) Fatal(msg string, fields ...Field) {}
func (n *nopLogger) With(fields ...Field) Logger      { return n }
func (n *nopLogger) WithContext(ctx context.Context) Logger { return n }
func (n *nopLogger) WithError(err error) Logger        { return n }
func (n *nopLogger) Sync() error                      { return nil }

func NewNopLogger() Logger {
	return &nopLogger{}
}

// NewNoop is an alias for NewNopLogger for convenience.
func NewNoop() Logger {
	return &nopLogger{}
}

var (
	globalLogger   Logger
	globalLoggerMu sync.RWMutex
)

func init() {
	globalLogger = NewDefaultLogger()
}

func SetGlobalLogger(l Logger) {
	globalLoggerMu.Lock()
	defer globalLoggerMu.Unlock()
	globalLogger = l
}

func GetGlobalLogger() Logger {
	globalLoggerMu.RLock()
	defer globalLoggerMu.RUnlock()
	return globalLogger
}

func L() Logger {
	return GetGlobalLogger()
}

// Performance logging helpers

func LogOperationDuration(logger Logger, operation string, start time.Time, fields ...Field) {
	duration := time.Since(start)
	allFields := append(fields, String(FieldOperation, operation), Duration(FieldDurationMs, duration))

	if duration > 5*time.Second {
		allFields = append(allFields, Bool("very_slow_operation", true))
		logger.Error("operation very slow", allFields...)
	} else if duration > time.Second {
		allFields = append(allFields, Bool("slow_operation", true))
		logger.Warn("operation slow", allFields...)
	} else {
		logger.Info("operation completed", allFields...)
	}
}

func LogDatabaseQuery(logger Logger, query string, duration time.Duration, rowsAffected int64, err error) {
	shortQuery := query
	if len(shortQuery) > 200 {
		shortQuery = shortQuery[:197] + "..."
	}
	fields := []Field{
		String(FieldQuery, shortQuery),
		Duration(FieldDurationMs, duration),
		Int64(FieldRowsAffected, rowsAffected),
	}
	if err != nil {
		logger.WithError(err).Error("database query failed", fields...)
	} else {
		logger.Info("database query completed", fields...)
	}
}

func LogExternalCall(logger Logger, service string, method string, url string, statusCode int, duration time.Duration, err error) {
	fields := []Field{
		String(FieldService, service),
		String("method", method),
		String("url", url),
		Int("status_code", statusCode),
		Duration(FieldDurationMs, duration),
	}
	if err != nil || (statusCode >= 400) {
		logger.WithError(err).Error("external call failed", fields...)
	} else {
		logger.Info("external call completed", fields...)
	}
}

func LogAIInference(logger Logger, model string, operation string, inputSize int, duration time.Duration, err error) {
	fields := []Field{
		String(FieldModel, model),
		String(FieldOperation, operation),
		Int(FieldInputSize, inputSize),
		Duration(FieldDurationMs, duration),
	}
	if err != nil {
		logger.WithError(err).Error("AI inference failed", fields...)
	} else {
		logger.Info("AI inference completed", fields...)
	}
}

// Field constants
const (
	FieldRequestID      = "request_id"
	FieldTraceID        = "trace_id"
	FieldSpanID         = "span_id"
	FieldUserID         = "user_id"
	FieldSessionID      = "session_id"
	FieldModule         = "module"
	FieldOperation      = "operation"
	FieldDurationMs     = "duration_ms"
	FieldErrorCode      = "error_code"
	FieldHTTPMethod     = "http_method"
	FieldHTTPPath       = "http_path"
	FieldHTTPStatus     = "http_status"
	FieldMoleculeID     = "molecule_id"
	FieldPatentNumber   = "patent_number"
	FieldSMILES         = "smiles"
	FieldFingerprintType = "fingerprint_type"
	FieldSimilarityScore = "similarity_score"
	FieldRiskLevel      = "risk_level"
	FieldQuery          = "query"
	FieldRowsAffected   = "rows_affected"
	FieldService        = "service"
	FieldModel          = "model"
	FieldInputSize      = "input_size"
)

//Personal.AI order the ending
