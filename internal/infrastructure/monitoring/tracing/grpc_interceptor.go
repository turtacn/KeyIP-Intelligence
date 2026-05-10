// Phase 11 - 基础设施层: gRPC 追踪拦截器
// 序号: 289.5
// 文件: internal/infrastructure/monitoring/tracing/grpc_interceptor.go
// 功能定位: 提供 gRPC 服务器和客户端的 OpenTelemetry 追踪拦截器，
//           实现 W3C TraceContext 在 gRPC metadata 中的提取与注入
// 核心实现:
//   - metadataCarrier 适配器: 将 gRPC metadata.MD 转换为 OTel TextMapCarrier
//   - UnaryServerInterceptor: 从 gRPC metadata 提取 trace context → 创建服务端 span
//   - StreamServerInterceptor: 同上，针对流式 RPC
//   - UnaryClientInterceptor: 从 context 注入 trace context → gRPC metadata
//   - StreamClientInterceptor: 同上，针对流式 RPC
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/trace,
//           go.opentelemetry.io/otel/propagation, google.golang.org/grpc/metadata
//   - 被依赖: internal/interfaces/grpc/server.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package tracing

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ---------------------------------------------------------------------------
// metadataCarrier: adapts gRPC metadata.MD to propagation.TextMapCarrier
// ---------------------------------------------------------------------------

// metadataCarrier adapts gRPC metadata.MD to the OpenTelemetry TextMapCarrier
// interface, enabling W3C TraceContext (traceparent/tracestate) propagation
// through gRPC metadata.
type metadataCarrier struct {
	md *metadata.MD
}

// newMetadataCarrier creates a new metadataCarrier wrapping the given metadata.MD.
func newMetadataCarrier(md *metadata.MD) *metadataCarrier {
	return &metadataCarrier{md: md}
}

// Get returns the first value associated with the given key.
func (c *metadataCarrier) Get(key string) string {
	vals := c.md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// Set sets the key-value pair in the metadata. Replaces any existing values.
func (c *metadataCarrier) Set(key, value string) {
	c.md.Set(key, value)
}

// Keys returns a slice of all keys in the metadata.
func (c *metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.md))
	for k := range *c.md {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Server interceptors: extract trace context from incoming gRPC metadata
// ---------------------------------------------------------------------------

// UnaryServerInterceptor returns a gRPC unary server interceptor that
// extracts W3C TraceContext from incoming gRPC metadata and creates a
// server-side span for the RPC call.
//
// Usage:
//
//	grpc.UnaryInterceptor(tracing.UnaryServerInterceptor())
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract trace context from incoming gRPC metadata.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		propagator := otel.GetTextMapPropagator()
		ctx = propagator.Extract(ctx, newMetadataCarrier(&md))

		// Start a server-side span.
		tracer := otel.Tracer("grpc.server")
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Set RPC attributes on the span.
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
		)

		// Execute the handler with the trace context injected into the context.
		resp, err := handler(ctx, req)

		// Record error status if applicable.
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor that
// extracts W3C TraceContext from incoming gRPC metadata and creates a
// server-side span for the streaming RPC.
//
// Usage:
//
//	grpc.StreamInterceptor(tracing.StreamServerInterceptor())
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		// Extract trace context from incoming gRPC metadata.
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		propagator := otel.GetTextMapPropagator()
		ctx = propagator.Extract(ctx, newMetadataCarrier(&md))

		// Start a server-side span.
		tracer := otel.Tracer("grpc.server")
		ctx, span := tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Set RPC attributes.
		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
		)

		// Wrap the server stream so downstream code uses the trace context.
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		err := handler(srv, wrapped)

		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

// wrappedServerStream wraps grpc.ServerStream to override the Context() method
// with a context that carries the extracted trace context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the trace-aware context.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// ---------------------------------------------------------------------------
// Client interceptors: inject trace context into outgoing gRPC metadata
// ---------------------------------------------------------------------------

// UnaryClientInterceptor returns a gRPC unary client interceptor that injects
// W3C TraceContext from the current context into outgoing gRPC metadata,
// enabling distributed tracing across gRPC service boundaries.
//
// Usage:
//
//	grpc.WithUnaryClientInterceptor(tracing.UnaryClientInterceptor())
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		// Inject trace context into outgoing gRPC metadata.
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		propagator := otel.GetTextMapPropagator()
		propagator.Inject(ctx, newMetadataCarrier(&md))
		ctx = metadata.NewOutgoingContext(ctx, md)

		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor returns a gRPC stream client interceptor that injects
// W3C TraceContext from the current context into outgoing gRPC metadata,
// enabling distributed tracing across gRPC streaming service boundaries.
//
// Usage:
//
//	grpc.WithStreamClientInterceptor(tracing.StreamClientInterceptor())
func StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		// Inject trace context into outgoing gRPC metadata.
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		}

		propagator := otel.GetTextMapPropagator()
		propagator.Inject(ctx, newMetadataCarrier(&md))
		ctx = metadata.NewOutgoingContext(ctx, md)

		return streamer(ctx, desc, cc, method, opts...)
	}
}

//Personal.AI order the ending
