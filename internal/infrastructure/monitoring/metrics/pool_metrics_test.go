package metrics

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewPoolMetrics(t *testing.T) {
	pm, err := NewPoolMetrics()
	require.NoError(t, err)
	require.NotNil(t, pm)

	assert.NotNil(t, pm.OpenConnections)
	assert.NotNil(t, pm.InUseConnections)
	assert.NotNil(t, pm.IdleConnections)
	assert.NotNil(t, pm.WaitCount)
	assert.NotNil(t, pm.WaitDurationTotal)
	assert.NotNil(t, pm.PoolSaturation)
	assert.Equal(t, "keyip.pool", pm.meterName)
}

func TestNewPoolMetricsWithMeter(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	pm, err := NewPoolMetricsWithMeter(meter)
	require.NoError(t, err)
	require.NotNil(t, pm)

	assert.NotNil(t, pm.OpenConnections)
	assert.NotNil(t, pm.InUseConnections)
	assert.NotNil(t, pm.IdleConnections)
	assert.NotNil(t, pm.WaitCount)
	assert.NotNil(t, pm.WaitDurationTotal)
	assert.NotNil(t, pm.PoolSaturation)
	assert.Equal(t, "keyip.pool", pm.meterName)
}

func TestRecordPoolStats_DeltaTracking(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	pm, err := NewPoolMetricsWithMeter(meter)
	require.NoError(t, err)

	ctx := context.Background()

	// First recording: delta = stats (prev values are 0)
	stats1 := sql.DBStats{
		MaxOpenConnections: 25,
		OpenConnections:    5,
		InUse:              3,
		Idle:               2,
		WaitCount:          10,
		WaitDuration:       5 * time.Second,
	}
	pm.RecordPoolStats(ctx, stats1)

	// Verify internal prev values were updated.
	assert.Equal(t, int64(5), pm.prevOpen)
	assert.Equal(t, int64(3), pm.prevInUse)
	assert.Equal(t, int64(2), pm.prevIdle)
	assert.Equal(t, int64(10), pm.prevWait)
	assert.Equal(t, 5.0, pm.prevWaitDuration)

	// Collect and verify metrics were recorded.
	var data metricdata.ResourceMetrics
	err = reader.Collect(ctx, &data)
	require.NoError(t, err)
	require.Len(t, data.ScopeMetrics, 1)

	// Second recording: compute actual deltas.
	stats2 := sql.DBStats{
		MaxOpenConnections: 25,
		OpenConnections:    8,
		InUse:              5,
		Idle:               3,
		WaitCount:          15,
		WaitDuration:       8 * time.Second,
	}
	pm.RecordPoolStats(ctx, stats2)

	assert.Equal(t, int64(8), pm.prevOpen)
	assert.Equal(t, int64(5), pm.prevInUse)
	assert.Equal(t, int64(3), pm.prevIdle)
	assert.Equal(t, int64(15), pm.prevWait)
	assert.Equal(t, 8.0, pm.prevWaitDuration)
}

func TestRecordPoolStats_NegativeDeltaDoesNotPanic(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	pm, err := NewPoolMetricsWithMeter(meter)
	require.NoError(t, err)

	ctx := context.Background()

	// First recording with some stats.
	pm.RecordPoolStats(ctx, sql.DBStats{
		MaxOpenConnections: 25,
		OpenConnections:    10,
		InUse:              5,
		Idle:               5,
		WaitCount:          100,
		WaitDuration:       30 * time.Second,
	})

	// Second recording with fewer connections (negative deltas).
	stats2 := sql.DBStats{
		MaxOpenConnections: 25,
		OpenConnections:    6,
		InUse:              2,
		Idle:               4,
		WaitCount:          100,
		WaitDuration:       30 * time.Second,
	}
	pm.RecordPoolStats(ctx, stats2)

	// WaitCount should not decrease (delta filtered to positive only).
	assert.Equal(t, int64(100), pm.prevWait)
}

func TestRecordPoolStats_ZeroMaxOpenConns(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	pm, err := NewPoolMetricsWithMeter(meter)
	require.NoError(t, err)

	ctx := context.Background()

	// When MaxOpenConnections is 0, saturation should be 0.
	stats := sql.DBStats{
		MaxOpenConnections: 0,
		OpenConnections:    5,
		InUse:              3,
		Idle:               2,
	}
	pm.RecordPoolStats(ctx, stats)

	assert.Equal(t, int64(5), pm.prevOpen)
	assert.Equal(t, int64(3), pm.prevInUse)
	assert.Equal(t, int64(2), pm.prevIdle)
}

func TestRecordPoolStats_FullPoolSaturation(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	pm, err := NewPoolMetricsWithMeter(meter)
	require.NoError(t, err)

	ctx := context.Background()

	// All max connections in use.
	stats := sql.DBStats{
		MaxOpenConnections: 10,
		OpenConnections:    10,
		InUse:              10,
		Idle:               0,
		WaitCount:          50,
		WaitDuration:       60 * time.Second,
	}
	pm.RecordPoolStats(ctx, stats)

	// Verify prev state.
	assert.Equal(t, int64(10), pm.prevOpen)
	assert.Equal(t, int64(10), pm.prevInUse)
	assert.Equal(t, int64(0), pm.prevIdle)
	assert.Equal(t, int64(50), pm.prevWait)
	assert.Equal(t, 60.0, pm.prevWaitDuration)
}

// Personal.AI order the ending
