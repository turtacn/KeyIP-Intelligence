// Phase 11 - 基础设施层: OpenTelemetry 分布式追踪
// 序号: 288
// 文件: internal/infrastructure/monitoring/tracing/tracer.go
// 功能定位: 提供 OpenTelemetry TracerProvider 的初始化和生命周期管理，
//           支持 stdout 导出器，配置服务名称、环境标签和采样率
// 核心实现:
//   - 定义 Config 结构体: ServiceName, Environment, SampleRate
//   - 实现 SetupTracerProvider(ctx, Config) (*trace.TracerProvider, error)
//   - 实现 Shutdown(ctx, *trace.TracerProvider) error
//   - 设置 W3C TraceContext 传播器用于上下文传递
//   - 设置全局 TracerProvider 以便其他组件通过 otel.Tracer() 获取
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/sdk,
//           go.opentelemetry.io/otel/exporters/stdout/stdouttrace
//   - 被依赖: cmd/apiserver/main.go, internal/interfaces/http/middleware/tracing.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package tracing

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config holds OpenTelemetry tracing configuration.
type Config struct {
	// ServiceName identifies the service in traces. Default: "keyip-intelligence".
	ServiceName string

	// Environment identifies the deployment environment (dev, staging, prod).
	// Default: "development".
	Environment string

	// SampleRate controls the tracing sampling rate (0.0 to 1.0). Default: 1.0.
	SampleRate float64
}

// DefaultConfig returns a sensible default tracing configuration.
func DefaultConfig() Config {
	return Config{
		ServiceName: "keyip-intelligence",
		Environment: "development",
		SampleRate:  1.0,
	}
}

// SetupTracerProvider creates and configures an OpenTelemetry TracerProvider
// with a stdout exporter. It sets the global TracerProvider and configures
// W3C TraceContext propagation for distributed context propagation.
//
// Call Shutdown when the application terminates to flush any remaining spans.
func SetupTracerProvider(ctx context.Context, cfg Config) (*sdktrace.TracerProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "keyip-intelligence"
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 1.0
	}

	// Create stdout exporter with pretty-print for development readability.
	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(os.Stdout),
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, err
	}

	// Build resource attributes that identify the service.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			attribute.String("service.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// Configure sampler based on sample rate.
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create the TracerProvider with batching exporter.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set the global TracerProvider so that otel.Tracer() works everywhere.
	otel.SetTracerProvider(tp)

	// Configure W3C TraceContext and Baggage propagators for distributed context propagation.
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp, nil
}

// Shutdown flushes remaining spans and shuts down the TracerProvider.
// It should be called during application graceful shutdown.
func Shutdown(ctx context.Context, tp *sdktrace.TracerProvider) error {
	if tp == nil {
		return nil
	}
	return tp.Shutdown(ctx)
}

//Personal.AI order the ending
