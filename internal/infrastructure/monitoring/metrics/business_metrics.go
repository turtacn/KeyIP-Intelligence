// Phase 11 - 基础设施层: 业务指标仪表化
// 序号: 292
// 文件: internal/infrastructure/monitoring/metrics/business_metrics.go
// 功能定位: 定义关键业务操作的 OpenTelemetry 指标，包括分子搜索、
//           专利分析和侵权评估的计数与耗时追踪
// 核心实现:
//   - 定义 BusinessMetrics 结构体
//   - 实现 NewBusinessMetrics(meter) *BusinessMetrics
//   - 实现 RecordMoleculeSearch / RecordPatentAnalysis / RecordInfringement 方法
//
// 依赖关系:
//   - 依赖: go.opentelemetry.io/otel, go.opentelemetry.io/otel/metric
//   - 被依赖: internal/application 层各服务
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

// BusinessMetrics holds all business-level OpenTelemetry instruments.
type BusinessMetrics struct {
	// MoleculeSearchCount counts molecule search operations by search type.
	MoleculeSearchCount metric.Int64Counter

	// MoleculeSearchDuration measures molecule search latency in seconds.
	MoleculeSearchDuration metric.Float64Histogram

	// PatentAnalysisCount counts patent analysis operations by analysis type.
	PatentAnalysisCount metric.Int64Counter

	// PatentAnalysisDuration measures patent analysis latency in seconds.
	PatentAnalysisDuration metric.Float64Histogram

	// InfringementAssessmentCount counts infringement assessment operations
	// by assessment depth.
	InfringementAssessmentCount metric.Int64Counter

	// InfringementAssessmentDuration measures infringement assessment
	// latency in seconds.
	InfringementAssessmentDuration metric.Float64Histogram

	meterName string
}

// NewBusinessMetrics creates and registers all business metric instruments
// using the global OTel meter provider.
func NewBusinessMetrics() (*BusinessMetrics, error) {
	meter := otel.Meter("keyip.business")
	return NewBusinessMetricsWithMeter(meter)
}

// NewBusinessMetricsWithMeter creates business metrics using a specific meter.
func NewBusinessMetricsWithMeter(meter metric.Meter) (*BusinessMetrics, error) {
	moleculeSearchCount, err := meter.Int64Counter(
		"business.molecule.search.total",
		metric.WithDescription("Total number of molecule searches by search type"),
	)
	if err != nil {
		return nil, err
	}

	moleculeSearchDuration, err := meter.Float64Histogram(
		"business.molecule.search.duration",
		metric.WithDescription("Molecule search latency in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30,
		),
	)
	if err != nil {
		return nil, err
	}

	patentAnalysisCount, err := meter.Int64Counter(
		"business.patent.analysis.total",
		metric.WithDescription("Total number of patent analyses by analysis type"),
	)
	if err != nil {
		return nil, err
	}

	patentAnalysisDuration, err := meter.Float64Histogram(
		"business.patent.analysis.duration",
		metric.WithDescription("Patent analysis latency in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
		),
	)
	if err != nil {
		return nil, err
	}

	infringementAssessmentCount, err := meter.Int64Counter(
		"business.infringement.assessment.total",
		metric.WithDescription("Total number of infringement assessments by depth"),
	)
	if err != nil {
		return nil, err
	}

	infringementAssessmentDuration, err := meter.Float64Histogram(
		"business.infringement.assessment.duration",
		metric.WithDescription("Infringement assessment latency in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300,
		),
	)
	if err != nil {
		return nil, err
	}

	return &BusinessMetrics{
		MoleculeSearchCount:             moleculeSearchCount,
		MoleculeSearchDuration:          moleculeSearchDuration,
		PatentAnalysisCount:             patentAnalysisCount,
		PatentAnalysisDuration:          patentAnalysisDuration,
		InfringementAssessmentCount:     infringementAssessmentCount,
		InfringementAssessmentDuration:  infringementAssessmentDuration,
		meterName:                       "keyip.business",
	}, nil
}

// RecordMoleculeSearch records a molecule search operation with its search
// type (e.g., "structure", "similarity") and duration.
func (m *BusinessMetrics) RecordMoleculeSearch(ctx context.Context, searchType string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("search_type", searchType),
	}
	m.MoleculeSearchCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.MoleculeSearchDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordPatentAnalysis records a patent analysis operation with its analysis
// type (e.g., "search", "advanced_search", "stats") and duration.
func (m *BusinessMetrics) RecordPatentAnalysis(ctx context.Context, analysisType string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("analysis_type", analysisType),
	}
	m.PatentAnalysisCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.PatentAnalysisDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// RecordInfringementAssessment records an infringement assessment operation
// with its assessment depth (e.g., "quick", "standard", "deep") and duration.
func (m *BusinessMetrics) RecordInfringementAssessment(ctx context.Context, depth string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("depth", depth),
	}
	m.InfringementAssessmentCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.InfringementAssessmentDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
}

// Personal.AI order the ending
