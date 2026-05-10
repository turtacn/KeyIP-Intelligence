// Phase 11 - 基础设施层: gRPC 指标拦截器
// 序号: 295
// 文件: internal/infrastructure/monitoring/metrics/grpc_interceptor.go
// 功能定位: 定义 gRPC 服务端和客户端的 OpenTelemetry 指标拦截器，
//           包括请求数（按方法+状态码）、延迟直方图、活跃请求数仪表盘
// 核心实现:
//   - 定义 GRPCMetrics 结构体: 服务端和客户端两侧的 instruments
//   - 实现 NewGRPCMetrics / NewGRPCMetricsWithMeter 构造函数
//   - UnaryServerInterceptor: 记录服务端一元 RPC 指标
//   - StreamServerInterceptor: 记录服务端流式 RPC 指标
//   - UnaryClientInterceptor: 记录客户端一元 RPC 指标
//   - StreamClientInterceptor: 记录客户端流式 RPC 指标
//   - splitMethodName 辅助函数解析 "/package.Service/Method"
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/metric,
//           google.golang.org/grpc, google.golang.org/grpc/status
//   - 被依赖: internal/interfaces/grpc/server.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package metrics

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// GRPCMetrics holds all gRPC-related OpenTelemetry instruments for both
// server-side and client-side monitoring.
type GRPCMetrics struct {
	// ServerRequestsTotal counts total gRPC server requests by service,
	// method, and status code.
	ServerRequestsTotal metric.Int64Counter

	// ServerRequestDuration is a histogram of gRPC server request latencies
	// in seconds.
	ServerRequestDuration metric.Float64Histogram

	// ServerActiveRequests is an up-down counter tracking in-flight server
	// requests.
	ServerActiveRequests metric.Int64UpDownCounter

	// ClientRequestsTotal counts total gRPC client requests by service,
	// method, and status code.
	ClientRequestsTotal metric.Int64Counter

	// ClientRequestDuration is a histogram of gRPC client request latencies
	// in seconds.
	ClientRequestDuration metric.Float64Histogram

	// ClientActiveRequests is an up-down counter tracking in-flight client
	// requests.
	ClientActiveRequests metric.Int64UpDownCounter

	// meterName identifies the meter used for these instruments.
	meterName string
}

// NewGRPCMetrics creates and registers all gRPC metric instruments using the
// global OTel meter provider. Returns a GRPCMetrics struct with ready-to-use
// instruments.
func NewGRPCMetrics() (*GRPCMetrics, error) {
	meter := otel.Meter("keyip.grpc")
	return NewGRPCMetricsWithMeter(meter)
}

// NewGRPCMetricsWithMeter creates gRPC metrics using a specific meter instance.
func NewGRPCMetricsWithMeter(meter metric.Meter) (*GRPCMetrics, error) {
	serverRequestsTotal, err := meter.Int64Counter(
		"rpc.server.requests.total",
		metric.WithDescription("Total number of gRPC server requests by service, method, and status code"),
	)
	if err != nil {
		return nil, err
	}

	serverRequestDuration, err := meter.Float64Histogram(
		"rpc.server.request.duration",
		metric.WithDescription("Measures the duration of gRPC server request handling"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		),
	)
	if err != nil {
		return nil, err
	}

	serverActiveRequests, err := meter.Int64UpDownCounter(
		"rpc.server.active_requests",
		metric.WithDescription("Number of active gRPC server requests currently being handled"),
	)
	if err != nil {
		return nil, err
	}

	clientRequestsTotal, err := meter.Int64Counter(
		"rpc.client.requests.total",
		metric.WithDescription("Total number of gRPC client requests by service, method, and status code"),
	)
	if err != nil {
		return nil, err
	}

	clientRequestDuration, err := meter.Float64Histogram(
		"rpc.client.request.duration",
		metric.WithDescription("Measures the duration of gRPC client request handling"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		),
	)
	if err != nil {
		return nil, err
	}

	clientActiveRequests, err := meter.Int64UpDownCounter(
		"rpc.client.active_requests",
		metric.WithDescription("Number of active gRPC client requests currently in flight"),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCMetrics{
		ServerRequestsTotal:    serverRequestsTotal,
		ServerRequestDuration:  serverRequestDuration,
		ServerActiveRequests:   serverActiveRequests,
		ClientRequestsTotal:    clientRequestsTotal,
		ClientRequestDuration:  clientRequestDuration,
		ClientActiveRequests:   clientActiveRequests,
		meterName:              "keyip.grpc",
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// splitMethodName splits "/package.Service/Method" into ("package.Service", "Method").
func splitMethodName(fullMethod string) (string, string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	idx := strings.LastIndex(fullMethod, "/")
	if idx < 0 {
		return "unknown", fullMethod
	}
	return fullMethod[:idx], fullMethod[idx+1:]
}

// ---------------------------------------------------------------------------
// Server interceptors
// ---------------------------------------------------------------------------

// UnaryServerInterceptor returns a gRPC unary server interceptor that records
// OpenTelemetry metrics for each request: request count (by service, method,
// and status code), request duration histogram, and active requests gauge.
//
// Usage:
//
//	grpc.UnaryInterceptor(metrics.UnaryServerInterceptor(myMetrics))
func UnaryServerInterceptor(m *GRPCMetrics) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if m == nil {
			return handler(ctx, req)
		}

		service, method := splitMethodName(info.FullMethod)
		baseAttrs := []attribute.KeyValue{
			attribute.String("rpc.service", service),
			attribute.String("rpc.method", method),
		}

		// Track active requests.
		m.ServerActiveRequests.Add(ctx, 1, metric.WithAttributes(baseAttrs...))
		defer m.ServerActiveRequests.Add(ctx, -1, metric.WithAttributes(baseAttrs...))

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		code := status.Code(err)
		attrs := append(baseAttrs, attribute.String("rpc.status_code", code.String()))

		m.ServerRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.ServerRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor that records
// OpenTelemetry metrics for each streaming RPC: request count, request duration
// histogram, and active requests gauge.
//
// Usage:
//
//	grpc.StreamInterceptor(metrics.StreamServerInterceptor(myMetrics))
func StreamServerInterceptor(m *GRPCMetrics) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if m == nil {
			return handler(srv, ss)
		}

		ctx := ss.Context()
		service, method := splitMethodName(info.FullMethod)
		baseAttrs := []attribute.KeyValue{
			attribute.String("rpc.service", service),
			attribute.String("rpc.method", method),
		}

		// Track active requests.
		m.ServerActiveRequests.Add(ctx, 1, metric.WithAttributes(baseAttrs...))
		defer m.ServerActiveRequests.Add(ctx, -1, metric.WithAttributes(baseAttrs...))

		start := time.Now()
		err := handler(srv, ss)
		duration := time.Since(start)

		code := status.Code(err)
		attrs := append(baseAttrs, attribute.String("rpc.status_code", code.String()))

		m.ServerRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.ServerRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

		return err
	}
}

// ---------------------------------------------------------------------------
// Client interceptors
// ---------------------------------------------------------------------------

// UnaryClientInterceptor returns a gRPC unary client interceptor that records
// OpenTelemetry metrics for each outgoing request: request count, request
// duration histogram, and active requests gauge.
//
// Usage:
//
//	grpc.WithUnaryClientInterceptor(metrics.UnaryClientInterceptor(myMetrics))
func UnaryClientInterceptor(m *GRPCMetrics) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if m == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		service, methodName := splitMethodName(method)
		baseAttrs := []attribute.KeyValue{
			attribute.String("rpc.service", service),
			attribute.String("rpc.method", methodName),
		}

		// Track active requests.
		m.ClientActiveRequests.Add(ctx, 1, metric.WithAttributes(baseAttrs...))
		defer m.ClientActiveRequests.Add(ctx, -1, metric.WithAttributes(baseAttrs...))

		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		code := status.Code(err)
		attrs := append(baseAttrs, attribute.String("rpc.status_code", code.String()))

		m.ClientRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.ClientRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

		return err
	}
}

// StreamClientInterceptor returns a gRPC stream client interceptor that records
// OpenTelemetry metrics for each outgoing streaming RPC. Metrics are recorded
// when the stream closes.
//
// Usage:
//
//	grpc.WithStreamClientInterceptor(metrics.StreamClientInterceptor(myMetrics))
func StreamClientInterceptor(m *GRPCMetrics) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		if m == nil {
			return streamer(ctx, desc, cc, method, opts...)
		}

		service, methodName := splitMethodName(method)
		baseAttrs := []attribute.KeyValue{
			attribute.String("rpc.service", service),
			attribute.String("rpc.method", methodName),
		}

		// Increment active requests before creating the stream.
		m.ClientActiveRequests.Add(ctx, 1, metric.WithAttributes(baseAttrs...))

		start := time.Now()
		clientStream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			// Stream creation failed; revert the active request increment
			// and record the error.
			m.ClientActiveRequests.Add(ctx, -1, metric.WithAttributes(baseAttrs...))
			code := status.Code(err)
			attrs := append(baseAttrs, attribute.String("rpc.status_code", code.String()))
			m.ClientRequestsTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
			m.ClientRequestDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
			return nil, err
		}

		// Wrap the stream to record metrics when it closes.
		wrapped := &metricsClientStream{
			ClientStream: clientStream,
			m:            m,
			attrs:        baseAttrs,
			start:        start,
		}

		return wrapped, nil
	}
}

// metricsClientStream wraps grpc.ClientStream to record OpenTelemetry metrics
// exactly once when the client-side stream finishes.
type metricsClientStream struct {
	grpc.ClientStream
	m     *GRPCMetrics
	attrs []attribute.KeyValue
	start time.Time
	once  sync.Once
}

// CloseSend signals the end of client-side sending and records stream metrics.
func (w *metricsClientStream) CloseSend() error {
	err := w.ClientStream.CloseSend()
	w.recordOnce(err)
	return err
}

// RecvMsg receives a message from the stream. When it returns an error
// (including io.EOF), the stream lifecycle metrics are recorded.
func (w *metricsClientStream) RecvMsg(m interface{}) error {
	err := w.ClientStream.RecvMsg(m)
	if err != nil {
		w.recordOnce(err)
	}
	return err
}

// recordOnce ensures metrics are recorded exactly once per stream lifecycle,
// regardless of whether CloseSend or RecvMsg triggers it first.
func (w *metricsClientStream) recordOnce(err error) {
	w.once.Do(func() {
		w.m.ClientActiveRequests.Add(w.Context(), -1, metric.WithAttributes(w.attrs...))

		code := status.Code(err)
		attrs := append(w.attrs, attribute.String("rpc.status_code", code.String()))
		w.m.ClientRequestsTotal.Add(w.Context(), 1, metric.WithAttributes(attrs...))
		w.m.ClientRequestDuration.Record(w.Context(), time.Since(w.start).Seconds(), metric.WithAttributes(attrs...))
	})
}

// Personal.AI order the ending
