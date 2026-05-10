// Phase 11 - 基础设施层: HTTP 客户端追踪传输层
// 序号: 289.6
// 文件: internal/infrastructure/monitoring/tracing/http_transport.go
// 功能定位: 为出站 HTTP 请求提供 W3C TraceContext 自动注入，实现跨 HTTP 服务的分布式追踪传播
// 核心实现:
//   - Transport 结构体包装 http.RoundTripper
//   - RoundTrip 方法在发送请求前注入 traceparent/tracestate 头
//   - 支持自定义底层 RoundTripper（如 TLS、连接池配置）
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/propagation
//   - 被依赖: 任何需要出站 HTTP 追踪的客户端
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package tracing

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Transport wraps an http.RoundTripper to automatically inject W3C TraceContext
// headers (traceparent, tracestate) into outgoing HTTP requests, enabling
// distributed trace propagation across HTTP service boundaries.
//
// Usage:
//
//	client := &http.Client{
//	    Transport: tracing.NewTransport(http.DefaultTransport),
//	}
type Transport struct {
	rt http.RoundTripper
}

// NewTransport creates a new Transport wrapping the provided http.RoundTripper.
// If rt is nil, http.DefaultTransport is used.
func NewTransport(rt http.RoundTripper) *Transport {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &Transport{rt: rt}
}

// RoundTrip implements http.RoundTripper. It injects W3C TraceContext headers
// from the request context into the outgoing request headers before delegating
// to the wrapped RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Inject W3C TraceContext (traceparent, tracestate) into the outgoing
	// HTTP request headers using the global propagator.
	otel.GetTextMapPropagator().Inject(
		req.Context(),
		propagation.HeaderCarrier(req.Header),
	)
	return t.rt.RoundTrip(req)
}

//Personal.AI order the ending
