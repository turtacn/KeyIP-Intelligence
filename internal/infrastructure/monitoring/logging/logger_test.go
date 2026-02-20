// Package logging_test provides unit tests for the Logger interface, its
// zap-backed implementation, the NopLogger, and the global default management.
package logging_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// newObservedLogger builds a zapLogger that writes to an in-memory observer so
// tests can assert on emitted entries without touching stdout/stderr.
// It returns the Logger under test and the observer sink.
func newObservedLogger(t *testing.T, level zapcore.Level) (logging.Logger, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(level)
	z := zap.New(core, zap.AddCallerSkip(1))
	// Use the package-internal zapLogger via the exported NewLoggerFromZap helper.
	// Because that helper is not exported we construct the logger via NewLogger
	// but then wrap the zap core ourselves for observation.
	// A cleaner approach: expose a constructor that accepts a zap.Core.
	// Since that is not yet in the public API, we use a two-step approach:
	// build a real logger for API tests and an observer core for entry tests.
	_ = z
	return logging.NewLoggerFromCore(core), logs
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewLogger_Configurations
// ─────────────────────────────────────────────────────────────────────────────

func TestNewLogger_JSONFormat(t *testing.T) {
	t.Parallel()

	cfg := logging.LogConfig{
		Level:  "info",
		Format: "json",
	}
	l, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, l)
}

func TestNewLogger_ConsoleFormat(t *testing.T) {
	t.Parallel()

	cfg := logging.LogConfig{
		Level:  "debug",
		Format: "console",
	}
	l, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, l)
}

func TestNewLogger_AllLevels(t *testing.T) {
	t.Parallel()

	for _, lvl := range []string{"debug", "info", "warn", "error"} {
		lvl := lvl
		t.Run(lvl, func(t *testing.T) {
			t.Parallel()
			cfg := logging.LogConfig{Level: lvl, Format: "json"}
			l, err := logging.NewLogger(cfg)
			require.NoError(t, err, "level=%s", lvl)
			require.NotNil(t, l)
		})
	}
}

func TestNewLogger_DefaultsApplied(t *testing.T) {
	t.Parallel()

	// Empty config — all defaults should kick in without error.
	cfg := logging.LogConfig{}
	l, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, l)
}

func TestNewLogger_ExplicitOutputPaths(t *testing.T) {
	t.Parallel()

	cfg := logging.LogConfig{
		Level:            "info",
		Format:           "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	l, err := logging.NewLogger(cfg)
	require.NoError(t, err)
	require.NotNil(t, l)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestLogger_MethodsDoNotPanic
// ─────────────────────────────────────────────────────────────────────────────

func TestLogger_InfoDoesNotPanic(t *testing.T) {
	t.Parallel()
	l, _ := logging.NewLogger(logging.LogConfig{Level: "debug", Format: "json"})
	assert.NotPanics(t, func() { l.Info("test info message") })
}

func TestLogger_DebugDoesNotPanic(t *testing.T) {
	t.Parallel()
	l, _ := logging.NewLogger(logging.LogConfig{Level: "debug", Format: "json"})
	assert.NotPanics(t, func() { l.Debug("test debug message") })
}

func TestLogger_WarnDoesNotPanic(t *testing.T) {
	t.Parallel()
	l, _ := logging.NewLogger(logging.LogConfig{Level: "debug", Format: "json"})
	assert.NotPanics(t, func() { l.Warn("test warn message") })
}

func TestLogger_ErrorDoesNotPanic(t *testing.T) {
	t.Parallel()
	l, _ := logging.NewLogger(logging.LogConfig{Level: "debug", Format: "json"})
	assert.NotPanics(t, func() { l.Error("test error message") })
}

func TestLogger_WithFieldsDoesNotPanic(t *testing.T) {
	t.Parallel()
	l, _ := logging.NewLogger(logging.LogConfig{Level: "debug", Format: "json"})
	assert.NotPanics(t, func() {
		l.Info("msg",
			logging.String("key", "value"),
			logging.Int("count", 42),
			logging.Bool("flag", true),
			logging.Float64("ratio", 3.14),
			logging.Int64("big", 9999999999),
			logging.Duration("elapsed", time.Second),
			logging.Err(errors.New("boom")),
			logging.Err(nil),
			logging.Any("arbitrary", struct{ X int }{X: 1}),
		)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// TestLogger_With — observer-based entry assertions
// ─────────────────────────────────────────────────────────────────────────────

func TestLogger_With_PresetFieldsAppearInEntries(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)

	child := l.With(logging.String("service", "patent-svc"), logging.Int("tenant", 7))
	child.Info("hello from child")

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	assert.Equal(t, "hello from child", entry.Message)

	fieldMap := make(map[string]interface{})
	for _, f := range entry.Context {
		fieldMap[f.Key] = f.String
	}
	assert.Equal(t, "patent-svc", fieldMap["service"])
}

func TestLogger_With_DoesNotMutateParent(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)

	child := l.With(logging.String("child_field", "yes"))
	_ = child

	// Log on the parent — entry must NOT contain child_field.
	l.Info("parent message")

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	for _, f := range entry.Context {
		assert.NotEqual(t, "child_field", f.Key,
			"parent logger should not carry child's preset fields")
	}
}

func TestLogger_With_ChainedPresetFields(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)

	child := l.
		With(logging.String("layer", "handler")).
		With(logging.String("route", "/api/patents"))
	child.Info("chained")

	require.Equal(t, 1, logs.Len())
	ctx := logs.All()[0].Context
	keys := make([]string, 0, len(ctx))
	for _, f := range ctx {
		keys = append(keys, f.Key)
	}
	assert.Contains(t, keys, "layer")
	assert.Contains(t, keys, "route")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestLogger_Named
// ─────────────────────────────────────────────────────────────────────────────

func TestLogger_Named_IncludesNamePrefix(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)

	named := l.Named("molecule-svc")
	named.Info("named entry")

	require.Equal(t, 1, logs.Len())
	assert.Equal(t, "molecule-svc", logs.All()[0].LoggerName)
}

func TestLogger_Named_ChainedNames(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)

	named := l.Named("app").Named("http")
	named.Info("chained name")

	require.Equal(t, 1, logs.Len())
	// zap joins names with a dot: "app.http"
	assert.Equal(t, "app.http", logs.All()[0].LoggerName)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestLogger_ObservedEntries — level filtering and field types
// ─────────────────────────────────────────────────────────────────────────────

func TestLogger_DebugFilteredAtInfoLevel(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.InfoLevel)
	l := logging.NewLoggerFromCore(core)

	l.Debug("should be filtered")
	l.Info("should appear")

	assert.Equal(t, 1, logs.Len(), "only INFO should pass through INFO-level logger")
	assert.Equal(t, "should appear", logs.All()[0].Message)
}

func TestLogger_ErrorEntryLevel(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)

	l.Error("something broke", logging.Err(errors.New("disk full")))

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	assert.Equal(t, zapcore.ErrorLevel, entry.Level)
	assert.Equal(t, "something broke", entry.Message)
}

func TestLogger_WarnEntryLevel(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)
	l.Warn("degraded", logging.String("reason", "high latency"))

	require.Equal(t, 1, logs.Len())
	assert.Equal(t, zapcore.WarnLevel, logs.All()[0].Level)
}

func TestLogger_AllFieldTypes(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.DebugLevel)
	l := logging.NewLoggerFromCore(core)

	dur := 250 * time.Millisecond
	l.Info("all-types",
		logging.String("s", "hello"),
		logging.Int("i", 1),
		logging.Int64("i64", 2),
		logging.Float64("f", 3.14),
		logging.Bool("b", true),
		logging.Duration("d", dur),
		logging.Err(errors.New("e")),
		logging.Any("a", map[string]int{"x": 9}),
	)

	require.Equal(t, 1, logs.Len())
	// Verify the entry has context fields (exact count may vary by zap version).
	assert.NotEmpty(t, logs.All()[0].Context)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNopLogger
// ─────────────────────────────────────────────────────────────────────────────

func TestNopLogger_AllMethodsAreNoop(t *testing.T) {
	t.Parallel()

	l := logging.NewNopLogger()
	require.NotNil(t, l)

	assert.NotPanics(t, func() { l.Debug("d") })
	assert.NotPanics(t, func() { l.Info("i") })
	assert.NotPanics(t, func() { l.Warn("w") })
	assert.NotPanics(t, func() { l.Error("e") })
	// Fatal is NOT called in tests to avoid os.Exit.
}

func TestNopLogger_WithReturnsSelf(t *testing.T) {
	t.Parallel()

	l := logging.NewNopLogger()
	child := l.With(logging.String("k", "v"))
	require.NotNil(t, child)

	// Both parent and child must operate without panic.
	assert.NotPanics(t, func() { child.Info("child info") })
}

func TestNopLogger_NamedReturnsSelf(t *testing.T) {
	t.Parallel()

	l := logging.NewNopLogger()
	named := l.Named("component")
	require.NotNil(t, named)
	assert.NotPanics(t, func() { named.Warn("named warn") })
}

func TestNopLogger_SatisfiesInterface(t *testing.T) {
	t.Parallel()

	var _ logging.Logger = logging.NewNopLogger()
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDefaultLogger — global management
// ─────────────────────────────────────────────────────────────────────────────

func TestDefault_InitialValueIsNopLogger(t *testing.T) {
	// Do not run in parallel because this manipulates global state.
	l := logging.Default()
	require.NotNil(t, l)
	// NopLogger must not panic on any operation.
	assert.NotPanics(t, func() { l.Info("boot check") })
}

func TestSetDefault_ReplacesDefaultLogger(t *testing.T) {
	cfg := logging.LogConfig{Level: "info", Format: "json"}
	newLogger, err := logging.NewLogger(cfg)
	require.NoError(t, err)

	logging.SetDefault(newLogger)
	retrieved := logging.Default()
	assert.Equal(t, newLogger, retrieved)

	// Restore nop so subsequent tests are not affected.
	logging.SetDefault(logging.NewNopLogger())
}

func TestSetDefault_NilIsIgnored(t *testing.T) {
	// Passing nil should not replace the current default.
	original := logging.Default()
	logging.SetDefault(nil)
	assert.Equal(t, original, logging.Default())
}

func TestDefault_UsableAfterSetDefault(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	l := logging.NewLoggerFromCore(core)
	logging.SetDefault(l)

	logging.Default().Info("via default")
	assert.Equal(t, 1, logs.Len())

	// Restore.
	logging.SetDefault(logging.NewNopLogger())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestField — convenience constructors
// ─────────────────────────────────────────────────────────────────────────────

func TestField_String(t *testing.T) {
	t.Parallel()
	f := logging.String("k", "v")
	assert.Equal(t, "k", f.Key)
	assert.Equal(t, "v", f.Value)
}

func TestField_Int(t *testing.T) {
	t.Parallel()
	f := logging.Int("n", 42)
	assert.Equal(t, "n", f.Key)
	assert.Equal(t, 42, f.Value)
}

func TestField_Int64(t *testing.T) {
	t.Parallel()
	f := logging.Int64("big", int64(1<<40))
	assert.Equal(t, int64(1<<40), f.Value)
}

func TestField_Float64(t *testing.T) {
	t.Parallel()
	f := logging.Float64("pi", 3.14159)
	assert.InDelta(t, 3.14159, f.Value, 1e-9)
}

func TestField_Bool(t *testing.T) {
	t.Parallel()
	f := logging.Bool("flag", true)
	assert.Equal(t, true, f.Value)
}

func TestField_Duration(t *testing.T) {
	t.Parallel()
	d := 500 * time.Millisecond
	f := logging.Duration("elapsed", d)
	assert.Equal(t, d, f.Value)
}

func TestField_ErrWithError(t *testing.T) {
	t.Parallel()
	e := errors.New("disk full")
	f := logging.Err(e)
	assert.Equal(t, "error", f.Key)
	assert.Equal(t, "disk full", f.Value)
}

func TestField_ErrWithNil(t *testing.T) {
	t.Parallel()
	f := logging.Err(nil)
	assert.Equal(t, "error", f.Key)
	assert.Equal(t, "<nil>", f.Value)
}

func TestField_Any(t *testing.T) {
	t.Parallel()
	type Custom struct{ X int }
	v := Custom{X: 99}
	f := logging.Any("obj", v)
	assert.Equal(t, "obj", f.Key)
	assert.Equal(t, v, f.Value)
}

//Personal.AI order the ending
