// Phase 11 - 基础设施层: 数据库连接池指标仪表化
// 序号: 294
// 文件: internal/infrastructure/monitoring/metrics/pool_metrics.go
// 功能定位: 定义 PostgreSQL 连接池的 OpenTelemetry 指标，包括活跃连接数、
//           空闲连接数、等待计数和等待耗时追踪
// 核心实现:
//   - 定义 PoolMetrics 结构体
//   - 实现 NewPoolMetrics(meter) *PoolMetrics
//   - 实现 RecordPoolStats(stats) 方法
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/metric
//   - 被依赖: internal/infrastructure/database/postgres
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package metrics

import (
	"context"
	"database/sql"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// PoolMetrics holds all PostgreSQL connection pool OpenTelemetry instruments.
type PoolMetrics struct {
	// OpenConnections tracks the current number of open connections in the pool.
	OpenConnections metric.Int64UpDownCounter

	// InUseConnections tracks the current number of connections in use.
	InUseConnections metric.Int64UpDownCounter

	// IdleConnections tracks the current number of idle connections.
	IdleConnections metric.Int64UpDownCounter

	// WaitCount is the cumulative number of connections that waited for
	// a connection from the pool.
	WaitCount metric.Int64Counter

	// WaitDurationTotal is the cumulative wait time for connection pool
	// in seconds.
	WaitDurationTotal metric.Float64Counter

	// PoolSaturation records the ratio of in-use connections to max open
	// connections, used for capacity planning and saturation monitoring.
	PoolSaturation metric.Float64Histogram

	// meterName identifies the meter used for these instruments.
	meterName string

	// mu protects the previous stats values used for delta computation.
	mu sync.Mutex

	prevOpen         int64
	prevInUse        int64
	prevIdle         int64
	prevWait         int64
	prevWaitDuration float64
}

// NewPoolMetrics creates and registers all pool metric instruments using the
// global OTel meter provider.
func NewPoolMetrics() (*PoolMetrics, error) {
	meter := otel.Meter("keyip.pool")
	return NewPoolMetricsWithMeter(meter)
}

// NewPoolMetricsWithMeter creates pool metrics using a specific meter instance.
func NewPoolMetricsWithMeter(meter metric.Meter) (*PoolMetrics, error) {
	openConns, err := meter.Int64UpDownCounter(
		"db.pool.open_connections",
		metric.WithDescription("Current number of open connections in the pool"),
	)
	if err != nil {
		return nil, err
	}

	inUseConns, err := meter.Int64UpDownCounter(
		"db.pool.in_use_connections",
		metric.WithDescription("Current number of connections in use"),
	)
	if err != nil {
		return nil, err
	}

	idleConns, err := meter.Int64UpDownCounter(
		"db.pool.idle_connections",
		metric.WithDescription("Current number of idle connections"),
	)
	if err != nil {
		return nil, err
	}

	waitCount, err := meter.Int64Counter(
		"db.pool.wait_total",
		metric.WithDescription("Cumulative number of connections that waited for a connection"),
	)
	if err != nil {
		return nil, err
	}

	waitDurationTotal, err := meter.Float64Counter(
		"db.pool.wait_duration_seconds_total",
		metric.WithDescription("Cumulative wait time for connection pool in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	poolSaturation, err := meter.Float64Histogram(
		"db.pool.saturation",
		metric.WithDescription("Ratio of in-use connections to max open connections"),
		metric.WithExplicitBucketBoundaries(
			0.1, 0.25, 0.5, 0.75, 0.8, 0.85, 0.9, 0.95, 1.0,
		),
	)
	if err != nil {
		return nil, err
	}

	return &PoolMetrics{
		OpenConnections:   openConns,
		InUseConnections:  inUseConns,
		IdleConnections:   idleConns,
		WaitCount:         waitCount,
		WaitDurationTotal: waitDurationTotal,
		PoolSaturation:    poolSaturation,
		meterName:         "keyip.pool",
	}, nil
}

// RecordPoolStats records a snapshot of connection pool statistics. It computes
// deltas from the previous recording to correctly update the cumulative and
// up-down counter instruments.
func (m *PoolMetrics) RecordPoolStats(ctx context.Context, stats sql.DBStats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	deltaOpen := int64(stats.OpenConnections) - m.prevOpen
	deltaInUse := int64(stats.InUse) - m.prevInUse
	deltaIdle := int64(stats.Idle) - m.prevIdle
	deltaWait := stats.WaitCount - m.prevWait
	deltaWaitDur := stats.WaitDuration.Seconds() - m.prevWaitDuration

	m.OpenConnections.Add(ctx, deltaOpen)
	m.InUseConnections.Add(ctx, deltaInUse)
	m.IdleConnections.Add(ctx, deltaIdle)

	// Only emit positive deltas for cumulative counters to avoid
	// negative values from counter resets (e.g., database restart).
	if deltaWait > 0 {
		m.WaitCount.Add(ctx, deltaWait)
	}
	if deltaWaitDur > 0 {
		m.WaitDurationTotal.Add(ctx, deltaWaitDur)
	}

	// Record pool saturation as a histogram value for percentile analysis.
	attrs := []attribute.KeyValue{
		attribute.Int("max_open_connections", stats.MaxOpenConnections),
	}
	saturation := float64(0)
	if stats.MaxOpenConnections > 0 {
		saturation = float64(stats.InUse) / float64(stats.MaxOpenConnections)
	}
	m.PoolSaturation.Record(ctx, saturation, metric.WithAttributes(attrs...))

	m.prevOpen = int64(stats.OpenConnections)
	m.prevInUse = int64(stats.InUse)
	m.prevIdle = int64(stats.Idle)
	m.prevWait = stats.WaitCount
	m.prevWaitDuration = stats.WaitDuration.Seconds()
}

// Personal.AI order the ending
