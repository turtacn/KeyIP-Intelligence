// Phase 11 - 基础设施层: HTTP 指标仪表化
// 序号: 291
// 文件: internal/infrastructure/monitoring/metrics/http_metrics.go
// 功能定位: 定义 HTTP 请求相关的 OpenTelemetry 指标，包括请求耗时直方图、
//           按状态码分类的请求计数、以及活跃请求数仪表盘
// 核心实现:
//   - 定义 HTTPMetrics 结构体: RequestDuration, RequestsTotal, ActiveRequests
//   - 实现 NewHTTPMetrics(meter) *HTTPMetrics
//   - 实现 RecordRequest(method, path, status, duration) 方法
//   - 实现 AddActiveRequests(method, path, delta) 方法
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/metric
//   - 被依赖: internal/infrastructure/monitoring/metrics/middleware.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package metrics

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics holds all HTTP-related OpenTelemetry instruments.
type HTTPMetrics struct {
	// RequestDuration is a histogram of HTTP request latencies in seconds.
	RequestDuration metric.Float64Histogram

	// RequestsTotal is a counter of total HTTP requests partitioned by
	// method, path, and status code.
	RequestsTotal metric.Int64Counter

	// ActiveRequests is an up-down counter tracking in-flight requests.
	ActiveRequests metric.Int64UpDownCounter

	// meterName is the name passed to otel.Meter().
	meterName string
}

// NewHTTPMetrics creates and registers all HTTP metric instruments using the
// global OTel meter provider. Returns an HTTPMetrics struct with ready-to-use
// instruments.
func NewHTTPMetrics() (*HTTPMetrics, error) {
	meter := otel.Meter("keyip.http")
	return NewHTTPMetricsWithMeter(meter)
}

// NewHTTPMetricsWithMeter creates HTTP metrics using a specific meter instance.
func NewHTTPMetricsWithMeter(meter metric.Meter) (*HTTPMetrics, error) {
	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Measures the duration of HTTP request handling"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		),
	)
	if err != nil {
		return nil, err
	}

	requestsTotal, err := meter.Int64Counter(
		"http.server.requests.total",
		metric.WithDescription("Total number of HTTP requests by method, path, and status code"),
	)
	if err != nil {
		return nil, err
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests currently being handled"),
	)
	if err != nil {
		return nil, err
	}

	return &HTTPMetrics{
		RequestDuration:  requestDuration,
		RequestsTotal:    requestsTotal,
		ActiveRequests:   activeRequests,
		meterName:        "keyip.http",
	}, nil
}

// RecordRequest records a completed HTTP request's duration and increments the
// request counter for the given method, path, and status code combination.
func (m *HTTPMetrics) RecordRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.path", path),
		attribute.Int("http.status_code", statusCode),
	}

	m.RequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.RequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// AddActiveRequests adjusts the active requests gauge by delta (+1 when a
// request starts, -1 when it finishes).
func (m *HTTPMetrics) AddActiveRequests(ctx context.Context, method, path string, delta int64) {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("http.path", path),
	}
	m.ActiveRequests.Add(ctx, delta, metric.WithAttributes(attrs...))
}

// Personal.AI order the ending
