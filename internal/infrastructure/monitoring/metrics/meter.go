// Phase 11 - 基础设施层: OpenTelemetry 指标 (Metrics) 提供者
// 序号: 290
// 文件: internal/infrastructure/monitoring/metrics/meter.go
// 功能定位: 提供 OpenTelemetry MeterProvider 的初始化和生命周期管理，
//           支持 stdout 导出器和 Prometheus 导出器双通道输出，
//           配置服务名称和环境标签
// 核心实现:
//   - 定义 Config 结构体: ServiceName, Environment, EnableStdout, EnablePrometheus
//   - 实现 SetupMeterProvider(ctx, Config) (*metric.MeterProvider, error)
//   - 实现 Shutdown(ctx, *metric.MeterProvider) error
//   - 设置全局 MeterProvider 以便其他组件通过 otel.Meter() 获取
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/sdk/metric,
//           go.opentelemetry.io/otel/exporters/stdout/stdoutmetric,
//           go.opentelemetry.io/otel/exporters/prometheus
//   - 被依赖: cmd/apiserver/main.go, internal/infrastructure/monitoring/metrics/http_metrics.go,
//             internal/infrastructure/monitoring/metrics/business_metrics.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package metrics

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Config holds OpenTelemetry metrics configuration.
type Config struct {
	// ServiceName identifies the service in metrics. Default: "keyip-intelligence".
	ServiceName string

	// Environment identifies the deployment environment (dev, staging, prod).
	// Default: "development".
	Environment string

	// EnableStdout enables the stdout metrics exporter for development debugging.
	// Default: true.
	EnableStdout bool

	// EnablePrometheus enables the Prometheus exporter for production metrics
	// scraping. Default: true.
	EnablePrometheus bool

	// PrometheusPort is the HTTP port for the Prometheus metrics endpoint.
	// Default: 9090.
	PrometheusPort int
}

// DefaultConfig returns a sensible default metrics configuration.
func DefaultConfig() Config {
	return Config{
		ServiceName:      "keyip-intelligence",
		Environment:      "development",
		EnableStdout:     true,
		EnablePrometheus: true,
		PrometheusPort:   9090,
	}
}

// SetupMeterProvider creates and configures an OpenTelemetry MeterProvider
// with stdout and Prometheus exporters. It sets the global MeterProvider so
// that otel.Meter() works everywhere in the application.
//
// Call Shutdown when the application terminates to flush any remaining metrics.
func SetupMeterProvider(ctx context.Context, cfg Config) (*metric.MeterProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "keyip-intelligence"
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}
	if cfg.PrometheusPort <= 0 {
		cfg.PrometheusPort = 9090
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

	var readers []metric.Reader

	// Add stdout exporter for development debugging.
	if cfg.EnableStdout {
		stdoutExporter, err := stdoutmetric.New(
			stdoutmetric.WithWriter(os.Stdout),
			stdoutmetric.WithPrettyPrint(),
		)
		if err != nil {
			return nil, err
		}
		readers = append(readers, metric.NewPeriodicReader(stdoutExporter))
	}

	// Add Prometheus exporter for production metrics scraping.
	if cfg.EnablePrometheus {
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, err
		}
		readers = append(readers, promExporter)
	}

	// Build provider options.
	opts := []metric.Option{metric.WithResource(res)}
	for _, r := range readers {
		opts = append(opts, metric.WithReader(r))
	}

	// If no readers were configured, add a default stdout reader so the
	// MeterProvider is functional.
	if len(opts) == 1 {
		stdoutExporter, err := stdoutmetric.New(
			stdoutmetric.WithWriter(os.Stdout),
			stdoutmetric.WithPrettyPrint(),
		)
		if err != nil {
			return nil, err
		}
		opts = append(opts, metric.WithReader(metric.NewPeriodicReader(stdoutExporter)))
	}

	mp := metric.NewMeterProvider(opts...)

	// Set the global MeterProvider so that otel.Meter() works everywhere.
	otel.SetMeterProvider(mp)

	return mp, nil
}

// SetupMeterProviderWithReaders creates a MeterProvider with an explicit list
// of metric readers. This is useful for advanced setups where callers want
// full control over reader configuration (e.g., custom Prometheus registry).
func SetupMeterProviderWithReaders(ctx context.Context, cfg Config, readers ...metric.Reader) (*metric.MeterProvider, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "keyip-intelligence"
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			attribute.String("service.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, err
	}

	opts := []metric.Option{metric.WithResource(res)}
	for _, r := range readers {
		opts = append(opts, metric.WithReader(r))
	}

	mp := metric.NewMeterProvider(opts...)
	otel.SetMeterProvider(mp)
	return mp, nil
}

// Shutdown flushes remaining metrics and shuts down the MeterProvider.
// It should be called during application graceful shutdown.
func Shutdown(ctx context.Context, mp *metric.MeterProvider) error {
	if mp == nil {
		return nil
	}
	return mp.Shutdown(ctx)
}

// Personal.AI order the ending
