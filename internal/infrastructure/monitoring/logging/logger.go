// Package logging provides the platform-wide structured logging interface and
// its zap-backed implementation.  Every component that requires logging must
// depend on the Logger interface defined here; direct use of go.uber.org/zap
// is forbidden outside this package so that the underlying library can be
// swapped without touching business logic.
//
// Initialisation order in cmd/*/main.go:
//
//  1. Parse configuration.
//  2. Call NewLogger(cfg.Log) → store result in logging.SetDefault.
//  3. Initialise all other components, injecting the Logger instance.
package logging

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ─────────────────────────────────────────────────────────────────────────────
// Field — structured log field carrier
// ─────────────────────────────────────────────────────────────────────────────

// Field is a typed key-value pair attached to a log entry.  Using a concrete
// struct rather than variadic interface{} arguments keeps the API explicit and
// allows zero-allocation fast paths in the zapLogger implementation.
type Field struct {
	Key   string
	Value interface{}
}

// ── Convenience constructors ──────────────────────────────────────────────────

// String constructs a Field with a string value.
func String(key, val string) Field { return Field{Key: key, Value: val} }

// Int constructs a Field with an int value.
func Int(key string, val int) Field { return Field{Key: key, Value: val} }

// Int64 constructs a Field with an int64 value.
func Int64(key string, val int64) Field { return Field{Key: key, Value: val} }

// Float64 constructs a Field with a float64 value.
func Float64(key string, val float64) Field { return Field{Key: key, Value: val} }

// Bool constructs a Field with a bool value.
func Bool(key string, val bool) Field { return Field{Key: key, Value: val} }

// Err constructs a Field that captures an error under the canonical key "error".
// If err is nil the field value is the string "<nil>".
func Err(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: "<nil>"}
	}
	return Field{Key: "error", Value: err.Error()}
}

// Any constructs a Field with an arbitrary value.  Use this only when none of
// the typed constructors apply; the logger will fall back to fmt.Sprintf.
func Any(key string, val interface{}) Field { return Field{Key: key, Value: val} }

// Duration constructs a Field with a time.Duration value.
func Duration(key string, val time.Duration) Field { return Field{Key: key, Value: val} }

// ─────────────────────────────────────────────────────────────────────────────
// Logger interface
// ─────────────────────────────────────────────────────────────────────────────

// Logger is the platform-wide structured logging contract.  All components
// receive a Logger via constructor injection so that implementations can be
// swapped (e.g., NopLogger in tests) without code changes.
type Logger interface {
	// Debug logs a message at DEBUG level.  High-cardinality, high-frequency
	// entries that are disabled in production by setting level to INFO or above.
	Debug(msg string, fields ...Field)

	// Info logs a message at INFO level.  Routine operational events.
	Info(msg string, fields ...Field)

	// Warn logs a message at WARN level.  Recoverable abnormal conditions that
	// do not immediately affect correctness but deserve attention.
	Warn(msg string, fields ...Field)

	// Error logs a message at ERROR level.  Failures that affect a single
	// request or operation but from which the process can continue.
	Error(msg string, fields ...Field)

	// Fatal logs a message at FATAL level and then calls os.Exit(1).
	// Reserve for catastrophic startup failures; never call in request paths.
	Fatal(msg string, fields ...Field)

	// With returns a child Logger that includes the supplied fields in every
	// subsequent log entry.  The parent Logger is not mutated.
	With(fields ...Field) Logger

	// Named returns a child Logger whose name is appended to the parent's
	// name with a period separator (e.g., "app" → "app.http").
	Named(name string) Logger
}

// ─────────────────────────────────────────────────────────────────────────────
// LogConfig — logger construction parameters
// ─────────────────────────────────────────────────────────────────────────────

// LogConfig carries all parameters required to construct a Logger instance.
// It is typically populated from the application's configuration file via
// internal/config/loader.go.
type LogConfig struct {
	// Level controls the minimum severity that will be emitted.
	// Accepted values (case-insensitive): "debug", "info", "warn", "error".
	// Defaults to "info" when empty or unrecognised.
	Level string `yaml:"level" json:"level"`

	// Format selects the output encoding.
	// "json"    — structured JSON, suitable for log aggregation pipelines.
	// "console" — human-readable, coloured output for local development.
	// Defaults to "json" when empty or unrecognised.
	Format string `yaml:"format" json:"format"`

	// OutputPaths is the list of URLs or file paths to write log entries to.
	// "stdout" and "stderr" are special values; file paths are created if absent.
	// Defaults to ["stdout"] when nil.
	OutputPaths []string `yaml:"output_paths" json:"output_paths"`

	// ErrorOutputPaths is the list of URLs or file paths for internal zap errors
	// (e.g., write failures).  Defaults to ["stderr"] when nil.
	ErrorOutputPaths []string `yaml:"error_output_paths" json:"error_output_paths"`
}

// ─────────────────────────────────────────────────────────────────────────────
// zapLogger — zap-backed Logger implementation
// ─────────────────────────────────────────────────────────────────────────────

// zapLogger wraps a *zap.Logger and satisfies the Logger interface.  The inner
// zap.Logger is always synchronous (no sugar); we translate our Field slice to
// zap.Field values on every call, which lets zap's internal allocator pool
// them efficiently.
type zapLogger struct {
	z *zap.Logger
}

// toZapFields converts a slice of our Field values into zap.Field values.
// It handles the common concrete types without reflection; for everything else
// it falls back to zap.Any which uses fmt.Sprintf internally.
func toZapFields(fields []Field) []zap.Field {
	out := make([]zap.Field, 0, len(fields))
	for _, f := range fields {
		switch v := f.Value.(type) {
		case string:
			out = append(out, zap.String(f.Key, v))
		case int:
			out = append(out, zap.Int(f.Key, v))
		case int64:
			out = append(out, zap.Int64(f.Key, v))
		case float64:
			out = append(out, zap.Float64(f.Key, v))
		case bool:
			out = append(out, zap.Bool(f.Key, v))
		case time.Duration:
			out = append(out, zap.Duration(f.Key, v))
		case error:
			out = append(out, zap.NamedError(f.Key, v))
		default:
			out = append(out, zap.Any(f.Key, v))
		}
	}
	return out
}

func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.z.Debug(msg, toZapFields(fields)...)
}

func (l *zapLogger) Info(msg string, fields ...Field) {
	l.z.Info(msg, toZapFields(fields)...)
}

func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.z.Warn(msg, toZapFields(fields)...)
}

func (l *zapLogger) Error(msg string, fields ...Field) {
	l.z.Error(msg, toZapFields(fields)...)
}

func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.z.Fatal(msg, toZapFields(fields)...)
}

func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{z: l.z.With(toZapFields(fields)...)}
}

func (l *zapLogger) Named(name string) Logger {
	return &zapLogger{z: l.z.Named(name)}
}

// ─────────────────────────────────────────────────────────────────────────────
// NewLogger — factory
// ─────────────────────────────────────────────────────────────────────────────

// parseLevel converts a string level to a zapcore.Level.  Unknown values
// default to InfoLevel so the application remains operational.
func parseLevel(s string) zapcore.Level {
	switch s {
	case "debug", "DEBUG":
		return zapcore.DebugLevel
	case "warn", "WARN":
		return zapcore.WarnLevel
	case "error", "ERROR":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// NewLogger constructs and returns a Logger backed by zap according to cfg.
// Sensible defaults are applied for any unset configuration field:
//   - Level:            "info"
//   - Format:           "json"
//   - OutputPaths:      ["stdout"]
//   - ErrorOutputPaths: ["stderr"]
//
// Returns an error if zap fails to build the underlying logger (e.g., an
// invalid output path that cannot be opened).
func NewLogger(cfg LogConfig) (Logger, error) {
	// Apply defaults.
	if len(cfg.OutputPaths) == 0 {
		cfg.OutputPaths = []string{"stdout"}
	}
	if len(cfg.ErrorOutputPaths) == 0 {
		cfg.ErrorOutputPaths = []string{"stderr"}
	}

	level := parseLevel(cfg.Level)

	// Choose encoder config.
	var encCfg zapcore.EncoderConfig
	switch cfg.Format {
	case "console":
		encCfg = zap.NewDevelopmentEncoderConfig()
	default:
		encCfg = zap.NewProductionEncoderConfig()
	}
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// Choose encoder.
	var encoding string
	switch cfg.Format {
	case "console":
		encoding = "console"
	default:
		encoding = "json"
	}

	zapCfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(level),
		Development:      cfg.Format == "console",
		Encoding:         encoding,
		EncoderConfig:    encCfg,
		OutputPaths:      cfg.OutputPaths,
		ErrorOutputPaths: cfg.ErrorOutputPaths,
	}

	z, err := zapCfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, fmt.Errorf("logging: failed to build zap logger: %w", err)
	}
	return &zapLogger{z: z}, nil
}

// NewLoggerFromCore constructs a Logger from an existing zapcore.Core.
// This is primarily used for testing with observed logs.
func NewLoggerFromCore(core zapcore.Core) Logger {
	return &zapLogger{z: zap.New(core, zap.AddCallerSkip(1))}
}

// ─────────────────────────────────────────────────────────────────────────────
// nopLogger — no-op implementation for tests and disabled components
// ─────────────────────────────────────────────────────────────────────────────

type nopLogger struct{}

func (nopLogger) Debug(_ string, _ ...Field) {}
func (nopLogger) Info(_ string, _ ...Field)  {}
func (nopLogger) Warn(_ string, _ ...Field)  {}
func (nopLogger) Error(_ string, _ ...Field) {}
func (nopLogger) Fatal(_ string, _ ...Field) {}
func (n nopLogger) With(_ ...Field) Logger   { return n }
func (n nopLogger) Named(_ string) Logger    { return n }

// NewNopLogger returns a Logger that discards all log entries.  It is safe
// for concurrent use and intended exclusively for unit tests and benchmarks
// where log output would add noise without value.
func NewNopLogger() Logger { return nopLogger{} }

// ─────────────────────────────────────────────────────────────────────────────
// Global default Logger
// ─────────────────────────────────────────────────────────────────────────────

var (
	defaultMu     sync.RWMutex
	defaultLogger Logger = nopLogger{} // safe zero value; replaced during init
)

// SetDefault replaces the process-wide default Logger.  It is safe to call
// concurrently, though in practice it should only be called once during
// application startup before any goroutines that use Default() are started.
func SetDefault(l Logger) {
	if l == nil {
		return
	}
	defaultMu.Lock()
	defaultLogger = l
	defaultMu.Unlock()
}

// Default returns the process-wide default Logger.  Components that cannot
// receive an injected Logger (e.g., init functions, package-level variables)
// may fall back to Default(), but constructor injection is always preferred.
func Default() Logger {
	defaultMu.RLock()
	l := defaultLogger
	defaultMu.RUnlock()
	return l
}

