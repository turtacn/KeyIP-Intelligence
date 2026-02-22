package common

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// IntelligenceMetrics defines the interface for intelligence layer metrics.
type IntelligenceMetrics interface {
	RecordInference(ctx context.Context, params *InferenceMetricParams)
	RecordBatchProcessing(ctx context.Context, params *BatchMetricParams)
	RecordCacheAccess(ctx context.Context, hit bool, modelName string)
	RecordCircuitBreakerStateChange(ctx context.Context, modelName string, fromState, toState string)
	RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64)
	RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool)
	GetInferenceLatencyHistogram() LatencyHistogram
	GetCurrentStats() *IntelligenceStats
}

// InferenceMetricParams contains parameters for inference metrics.
type InferenceMetricParams struct {
	ModelName   string
	ModelVersion string
	TaskType    string
	DurationMs  float64
	Success     bool
	BatchSize   int
	InputTokens int
	DeviceType  string
}

// BatchMetricParams contains parameters for batch processing metrics.
type BatchMetricParams struct {
	BatchName        string
	TotalItems       int
	SuccessItems     int
	FailedItems      int
	TimeoutItems     int
	CancelledItems   int
	TotalDurationMs  float64
	AvgItemDurationMs float64
	MaxConcurrency   int
}

// IntelligenceStats contains current statistics snapshot.
type IntelligenceStats struct {
	TotalInferences      int64
	SuccessfulInferences int64
	FailedInferences     int64
	AvgInferenceLatencyMs float64
	P50LatencyMs         float64
	P95LatencyMs         float64
	P99LatencyMs         float64
	CacheHitRate         float64
	ActiveModels         []string
	CircuitBreakerStates map[string]string
}

// LatencyHistogram interface for latency metrics.
type LatencyHistogram interface {
	Observe(durationMs float64)
	Percentile(p float64) float64
	Count() int64
	Sum() float64
}

// prometheusIntelligenceMetrics implements IntelligenceMetrics using Prometheus.
type prometheusIntelligenceMetrics struct {
	inferenceLatency        *prometheus.HistogramVec
	inferenceTotal          *prometheus.CounterVec
	batchProcessingDuration *prometheus.HistogramVec
	batchItemsTotal         *prometheus.CounterVec
	cacheAccessTotal        *prometheus.CounterVec
	circuitBreakerState     *prometheus.GaugeVec
	riskAssessmentTotal     *prometheus.CounterVec
	riskAssessmentDuration  *prometheus.HistogramVec
	modelLoadDuration       *prometheus.HistogramVec
}

// NewPrometheusIntelligenceMetrics creates a new Prometheus metrics collector.
func NewPrometheusIntelligenceMetrics(registerer prometheus.Registerer) (IntelligenceMetrics, error) {
	m := &prometheusIntelligenceMetrics{
		inferenceLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "patentcraft_intelligence_inference_latency_seconds",
			Help: "Inference latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"model_name", "model_version", "task_type", "device_type"}),
		inferenceTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "patentcraft_intelligence_inference_total",
			Help: "Total number of inferences",
		}, []string{"model_name", "task_type", "status"}),
		batchProcessingDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "patentcraft_intelligence_batch_processing_duration_seconds",
			Help: "Batch processing duration in seconds",
		}, []string{"batch_name"}),
		batchItemsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "patentcraft_intelligence_batch_items_total",
			Help: "Total number of items processed in batches",
		}, []string{"batch_name", "status"}),
		cacheAccessTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "patentcraft_intelligence_cache_access_total",
			Help: "Total cache accesses",
		}, []string{"model_name", "result"}),
		circuitBreakerState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "patentcraft_intelligence_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half_open, 2=open)",
		}, []string{"model_name"}),
		riskAssessmentTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "patentcraft_intelligence_risk_assessment_total",
			Help: "Total risk assessments",
		}, []string{"risk_level"}),
		riskAssessmentDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "patentcraft_intelligence_risk_assessment_duration_seconds",
			Help: "Risk assessment duration in seconds",
		}, []string{"risk_level"}),
		modelLoadDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "patentcraft_intelligence_model_load_duration_seconds",
			Help: "Model load duration in seconds",
		}, []string{"model_name", "version", "status"}),
	}

	if err := registerer.Register(m.inferenceLatency); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.inferenceTotal); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.batchProcessingDuration); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.batchItemsTotal); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.cacheAccessTotal); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.circuitBreakerState); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.riskAssessmentTotal); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.riskAssessmentDuration); err != nil {
		return nil, err
	}
	if err := registerer.Register(m.modelLoadDuration); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *prometheusIntelligenceMetrics) RecordInference(ctx context.Context, params *InferenceMetricParams) {
	status := "success"
	if !params.Success {
		status = "failure"
	}
	m.inferenceTotal.WithLabelValues(params.ModelName, params.TaskType, status).Inc()
	m.inferenceLatency.WithLabelValues(params.ModelName, params.ModelVersion, params.TaskType, params.DeviceType).Observe(params.DurationMs / 1000.0)
}

func (m *prometheusIntelligenceMetrics) RecordBatchProcessing(ctx context.Context, params *BatchMetricParams) {
	m.batchProcessingDuration.WithLabelValues(params.BatchName).Observe(params.TotalDurationMs / 1000.0)
	m.batchItemsTotal.WithLabelValues(params.BatchName, "success").Add(float64(params.SuccessItems))
	m.batchItemsTotal.WithLabelValues(params.BatchName, "failed").Add(float64(params.FailedItems))
	m.batchItemsTotal.WithLabelValues(params.BatchName, "timeout").Add(float64(params.TimeoutItems))
	m.batchItemsTotal.WithLabelValues(params.BatchName, "cancelled").Add(float64(params.CancelledItems))
}

func (m *prometheusIntelligenceMetrics) RecordCacheAccess(ctx context.Context, hit bool, modelName string) {
	result := "miss"
	if hit {
		result = "hit"
	}
	m.cacheAccessTotal.WithLabelValues(modelName, result).Inc()
}

func (m *prometheusIntelligenceMetrics) RecordCircuitBreakerStateChange(ctx context.Context, modelName string, fromState, toState string) {
	var val float64
	switch toState {
	case "closed":
		val = 0
	case "half_open":
		val = 1
	case "open":
		val = 2
	}
	m.circuitBreakerState.WithLabelValues(modelName).Set(val)
}

func (m *prometheusIntelligenceMetrics) RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64) {
	m.riskAssessmentTotal.WithLabelValues(riskLevel).Inc()
	m.riskAssessmentDuration.WithLabelValues(riskLevel).Observe(durationMs / 1000.0)
}

func (m *prometheusIntelligenceMetrics) RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.modelLoadDuration.WithLabelValues(modelName, version, status).Observe(durationMs / 1000.0)
}

func (m *prometheusIntelligenceMetrics) GetInferenceLatencyHistogram() LatencyHistogram {
	// Not implemented for Prometheus wrapper as it's complex to extract
	return &noopLatencyHistogram{}
}

func (m *prometheusIntelligenceMetrics) GetCurrentStats() *IntelligenceStats {
	// Not implemented for Prometheus wrapper
	return &IntelligenceStats{}
}

// noopIntelligenceMetrics implements IntelligenceMetrics with no-op.
type noopIntelligenceMetrics struct{}

func NewNoopIntelligenceMetrics() IntelligenceMetrics {
	return &noopIntelligenceMetrics{}
}

func (m *noopIntelligenceMetrics) RecordInference(ctx context.Context, params *InferenceMetricParams) {}
func (m *noopIntelligenceMetrics) RecordBatchProcessing(ctx context.Context, params *BatchMetricParams) {}
func (m *noopIntelligenceMetrics) RecordCacheAccess(ctx context.Context, hit bool, modelName string) {}
func (m *noopIntelligenceMetrics) RecordCircuitBreakerStateChange(ctx context.Context, modelName string, fromState, toState string) {}
func (m *noopIntelligenceMetrics) RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64) {}
func (m *noopIntelligenceMetrics) RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool) {}
func (m *noopIntelligenceMetrics) GetInferenceLatencyHistogram() LatencyHistogram { return &noopLatencyHistogram{} }
func (m *noopIntelligenceMetrics) GetCurrentStats() *IntelligenceStats { return &IntelligenceStats{} }

type noopLatencyHistogram struct{}
func (h *noopLatencyHistogram) Observe(durationMs float64) {}
func (h *noopLatencyHistogram) Percentile(p float64) float64 { return 0 }
func (h *noopLatencyHistogram) Count() int64 { return 0 }
func (h *noopLatencyHistogram) Sum() float64 { return 0 }

// InMemoryIntelligenceMetrics implements IntelligenceMetrics in memory.
type inMemoryIntelligenceMetrics struct {
	mu           sync.RWMutex
	inferences   []*InferenceMetricParams
	batchMetrics []*BatchMetricParams
}

func NewInMemoryIntelligenceMetrics() IntelligenceMetrics {
	return &inMemoryIntelligenceMetrics{}
}

func (m *inMemoryIntelligenceMetrics) RecordInference(ctx context.Context, params *InferenceMetricParams) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inferences = append(m.inferences, params)
}

func (m *inMemoryIntelligenceMetrics) RecordBatchProcessing(ctx context.Context, params *BatchMetricParams) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchMetrics = append(m.batchMetrics, params)
}

func (m *inMemoryIntelligenceMetrics) RecordCacheAccess(ctx context.Context, hit bool, modelName string) {}
func (m *inMemoryIntelligenceMetrics) RecordCircuitBreakerStateChange(ctx context.Context, modelName string, fromState, toState string) {}
func (m *inMemoryIntelligenceMetrics) RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64) {}
func (m *inMemoryIntelligenceMetrics) RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool) {}
func (m *inMemoryIntelligenceMetrics) GetInferenceLatencyHistogram() LatencyHistogram { return &noopLatencyHistogram{} }
func (m *inMemoryIntelligenceMetrics) GetCurrentStats() *IntelligenceStats { return &IntelligenceStats{} }

//Personal.AI order the ending
