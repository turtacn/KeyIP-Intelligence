/*
 * metrics.go 实现了 IntelligenceMetrics 接口的三种变体（Prometheus、Noop、InMemory）以及基于排序切片+线性插值的 latencyHistogram，所有 Prometheus 指标遵循 patentcraft_intelligence_ 前缀命名规范，bucket 配置覆盖 1ms–5000ms 区间。
 * metrics_test.go 覆盖了全部要求的测试用例，包括 Prometheus 注册/重复注册、各 Record 方法的标签验证、Noop 零值安全、InMemory 查询回溯、百分位数精度（P50/P95/P99）、并发读写安全性以及参数结构体完整性校验。
*/

package common

import (
	"context"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// IntelligenceMetrics defines the unified metrics collection API for the
// intelligence layer. Every sub-module (MolPatent-GNN, InfringeNet, …)
// records its operational telemetry through this interface so that the
// underlying implementation (Prometheus, in-memory, noop) can be swapped
// without touching business code.
type IntelligenceMetrics interface {
	// RecordInference records a single model inference event.
	RecordInference(ctx context.Context, params *InferenceMetricParams)

	// RecordBatchProcessing records a batch processing event.
	RecordBatchProcessing(ctx context.Context, params *BatchMetricParams)

	// RecordCacheAccess records a cache hit or miss.
	RecordCacheAccess(ctx context.Context, hit bool, modelName string)

	// RecordCircuitBreakerStateChange records a circuit-breaker transition.
	RecordCircuitBreakerStateChange(ctx context.Context, modelName string, fromState, toState string)

	// RecordRiskAssessment records a risk assessment result.
	RecordRiskAssessment(ctx context.Context, riskLevel string, durationMs float64)

	// RecordModelLoad records a model load event.
	RecordModelLoad(ctx context.Context, modelName, version string, durationMs float64, success bool)

	// GetInferenceLatencyHistogram returns the latency histogram for SLO monitoring.
	GetInferenceLatencyHistogram() LatencyHistogram

	// GetCurrentStats returns a point-in-time statistics snapshot.
	GetCurrentStats() *IntelligenceStats
}

// LatencyHistogram provides percentile-based latency observation.
type LatencyHistogram interface {
	// Observe records a latency sample in milliseconds.
	Observe(durationMs float64)

	// Percentile returns the value at the given percentile (0–100).
	Percentile(p float64) float64

	// Count returns the total number of observed samples.
	Count() int64

	// Sum returns the sum of all observed values.
	Sum() float64
}

// ---------------------------------------------------------------------------
// Parameter structs
// ---------------------------------------------------------------------------

// InferenceMetricParams carries the data for a single inference event.
type InferenceMetricParams struct {
	ModelName    string  `json:"model_name"`
	ModelVersion string  `json:"model_version"`
	TaskType     string  `json:"task_type"`
	DurationMs   float64 `json:"duration_ms"`
	Success      bool    `json:"success"`
	BatchSize    int     `json:"batch_size"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	DeviceType   string  `json:"device_type,omitempty"`
}

// BatchMetricParams carries the data for a batch processing event.
type BatchMetricParams struct {
	BatchName        string  `json:"batch_name"`
	TotalItems       int     `json:"total_items"`
	SuccessItems     int     `json:"success_items"`
	FailedItems      int     `json:"failed_items"`
	TimeoutItems     int     `json:"timeout_items"`
	CancelledItems   int     `json:"cancelled_items"`
	TotalDurationMs  float64 `json:"total_duration_ms"`
	AvgItemDurationMs float64 `json:"avg_item_duration_ms"`
	MaxConcurrency   int     `json:"max_concurrency"`
}

// IntelligenceStats is a point-in-time snapshot of intelligence-layer metrics.
type IntelligenceStats struct {
	TotalInferences        int64              `json:"total_inferences"`
	SuccessfulInferences   int64              `json:"successful_inferences"`
	FailedInferences       int64              `json:"failed_inferences"`
	AvgInferenceLatencyMs  float64            `json:"avg_inference_latency_ms"`
	P50LatencyMs           float64            `json:"p50_latency_ms"`
	P95LatencyMs           float64            `json:"p95_latency_ms"`
	P99LatencyMs           float64            `json:"p99_latency_ms"`
	CacheHitRate           float64            `json:"cache_hit_rate"`
	ActiveModels           []string           `json:"active_models"`
	CircuitBreakerStates   map[string]string  `json:"circuit_breaker_states"`
}

// ---------------------------------------------------------------------------
// Prometheus implementation
// ---------------------------------------------------------------------------

const metricsPrefix = "patentcraft_intelligence_"

var defaultLatencyBuckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000}

type prometheusIntelligenceMetrics struct {
	inferenceLatency       *prometheus.HistogramVec
	inferenceTotal         *prometheus.CounterVec
	batchProcessingDuration *prometheus.HistogramVec
	batchItemsTotal        *prometheus.CounterVec
	cacheAccessTotal       *prometheus.CounterVec
	circuitBreakerState    *prometheus.GaugeVec
	riskAssessmentTotal    *prometheus.CounterVec
	riskAssessmentDuration *prometheus.HistogramVec
	modelLoadDuration      *prometheus.HistogramVec

	// in-memory tracking for GetCurrentStats / GetInferenceLatencyHistogram
	latencyHist *latencyHistogram
	totalInf    atomic.Int64
	successInf  atomic.Int64
	failedInf   atomic.Int64
	cacheHits   atomic.Int64
	cacheMisses atomic.Int64
	cbStates    sync.Map // model_name -> state string
}

// NewPrometheusIntelligenceMetrics creates a Prometheus-backed metrics collector
// and registers all metrics with the supplied Registerer.
func NewPrometheusIntelligenceMetrics(registerer prometheus.Registerer) (*prometheusIntelligenceMetrics, error) {
	if registerer == nil {
		registerer = prometheus.DefaultRegisterer
	}

	m := &prometheusIntelligenceMetrics{
		latencyHist: newLatencyHistogram(),
	}

	m.inferenceLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricsPrefix + "inference_duration_milliseconds",
		Help:    "Histogram of model inference latency in milliseconds.",
		Buckets: defaultLatencyBuckets,
	}, []string{"model_name", "model_version", "task_type", "device_type"})

	m.inferenceTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "inference_total",
		Help: "Total number of model inferences.",
	}, []string{"model_name", "task_type", "status"})

	m.batchProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricsPrefix + "batch_processing_duration_milliseconds",
		Help:    "Histogram of batch processing duration in milliseconds.",
		Buckets: defaultLatencyBuckets,
	}, []string{"batch_name"})

	m.batchItemsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "batch_items_total",
		Help: "Total number of items processed in batches.",
	}, []string{"batch_name", "status"})

	m.cacheAccessTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "cache_access_total",
		Help: "Total number of cache accesses.",
	}, []string{"model_name", "result"})

	m.circuitBreakerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricsPrefix + "circuit_breaker_state",
		Help: "Current circuit breaker state (0=closed, 1=half_open, 2=open).",
	}, []string{"model_name"})

	m.riskAssessmentTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: metricsPrefix + "risk_assessment_total",
		Help: "Total number of risk assessments by level.",
	}, []string{"risk_level"})

	m.riskAssessmentDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricsPrefix + "risk_assessment_duration_milliseconds",
		Help:    "Histogram of risk assessment duration in milliseconds.",
		Buckets: defaultLatencyBuckets,
	}, []string{"risk_level"})

	m.modelLoadDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    metricsPrefix + "model_load_duration_milliseconds",
		Help:    "Histogram of model load duration in milliseconds.",
		Buckets: defaultLatencyBuckets,
	}, []string{"model_name", "version", "status"})

	collectors := []prometheus.Collector{
		m.inferenceLatency,
		m.inferenceTotal,
		m.batchProcessingDuration,
		m.batchItemsTotal,
		m.cacheAccessTotal,
		m.circuitBreakerState,
		m.riskAssessmentTotal,
		m.riskAssessmentDuration,
		m.modelLoadDuration,
	}
	for _, c := range collectors {
		if err := registerer.Register(c); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (m *prometheusIntelligenceMetrics) RecordInference(_ context.Context, p *InferenceMetricParams) {
	if p == nil {
		return
	}
	status := "success"
	if !p.Success {
		status = "failure"
	}
	device := p.DeviceType
	if device == "" {
		device = "cpu"
	}

	m.inferenceLatency.WithLabelValues(p.ModelName, p.ModelVersion, p.TaskType, device).Observe(p.DurationMs)
	m.inferenceTotal.WithLabelValues(p.ModelName, p.TaskType, status).Inc()

	m.latencyHist.Observe(p.DurationMs)
	m.totalInf.Add(1)
	if p.Success {
		m.successInf.Add(1)
	} else {
		m.failedInf.Add(1)
	}
}

func (m *prometheusIntelligenceMetrics) RecordBatchProcessing(_ context.Context, p *BatchMetricParams) {
	if p == nil {
		return
	}
	m.batchProcessingDuration.WithLabelValues(p.BatchName).Observe(p.TotalDurationMs)
	m.batchItemsTotal.WithLabelValues(p.BatchName, "success").Add(float64(p.SuccessItems))
	m.batchItemsTotal.WithLabelValues(p.BatchName, "failed").Add(float64(p.FailedItems))
	m.batchItemsTotal.WithLabelValues(p.BatchName, "timeout").Add(float64(p.TimeoutItems))
	m.batchItemsTotal.WithLabelValues(p.BatchName, "cancelled").Add(float64(p.CancelledItems))
}

func (m *prometheusIntelligenceMetrics) RecordCacheAccess(_ context.Context, hit bool, modelName string) {
	result := "miss"
	if hit {
		result = "hit"
		m.cacheHits.Add(1)
	} else {
		m.cacheMisses.Add(1)
	}
	m.cacheAccessTotal.WithLabelValues(modelName, result).Inc()
}

func (m *prometheusIntelligenceMetrics) RecordCircuitBreakerStateChange(_ context.Context, modelName string, _, toState string) {
	m.cbStates.Store(modelName, toState)
	val := circuitBreakerStateToFloat(toState)
	m.circuitBreakerState.WithLabelValues(modelName).Set(val)
}

func (m *prometheusIntelligenceMetrics) RecordRiskAssessment(_ context.Context, riskLevel string, durationMs float64) {
	m.riskAssessmentTotal.WithLabelValues(riskLevel).Inc()
	m.riskAssessmentDuration.WithLabelValues(riskLevel).Observe(durationMs)
}

func (m *prometheusIntelligenceMetrics) RecordModelLoad(_ context.Context, modelName, version string, durationMs float64, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}
	m.modelLoadDuration.WithLabelValues(modelName, version, status).Observe(durationMs)
}

func (m *prometheusIntelligenceMetrics) GetInferenceLatencyHistogram() LatencyHistogram {
	return m.latencyHist
}

func (m *prometheusIntelligenceMetrics) GetCurrentStats() *IntelligenceStats {
	total := m.totalInf.Load()
	success := m.successInf.Load()
	failed := m.failedInf.Load()

	var avgLatency float64
	if total > 0 {
		avgLatency = m.latencyHist.Sum() / float64(total)
	}

	hits := m.cacheHits.Load()
	misses := m.cacheMisses.Load()
	var hitRate float64
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses)
	}

	cbStates := make(map[string]string)
	m.cbStates.Range(func(key, value any) bool {
		cbStates[key.(string)] = value.(string)
		return true
	})

	return &IntelligenceStats{
		TotalInferences:       total,
		SuccessfulInferences:  success,
		FailedInferences:      failed,
		AvgInferenceLatencyMs: avgLatency,
		P50LatencyMs:          m.latencyHist.Percentile(50),
		P95LatencyMs:          m.latencyHist.Percentile(95),
		P99LatencyMs:          m.latencyHist.Percentile(99),
		CacheHitRate:          hitRate,
		ActiveModels:          []string{},
		CircuitBreakerStates:  cbStates,
	}
}

// ---------------------------------------------------------------------------
// Noop implementation
// ---------------------------------------------------------------------------

type noopIntelligenceMetrics struct{}

// NewNoopIntelligenceMetrics returns a no-op metrics implementation.
func NewNoopIntelligenceMetrics() *noopIntelligenceMetrics {
	return &noopIntelligenceMetrics{}
}

func (n *noopIntelligenceMetrics) RecordInference(context.Context, *InferenceMetricParams)        {}
func (n *noopIntelligenceMetrics) RecordBatchProcessing(context.Context, *BatchMetricParams)       {}
func (n *noopIntelligenceMetrics) RecordCacheAccess(context.Context, bool, string)                 {}
func (n *noopIntelligenceMetrics) RecordCircuitBreakerStateChange(context.Context, string, string, string) {}
func (n *noopIntelligenceMetrics) RecordRiskAssessment(context.Context, string, float64)           {}
func (n *noopIntelligenceMetrics) RecordModelLoad(context.Context, string, string, float64, bool)  {}

func (n *noopIntelligenceMetrics) GetInferenceLatencyHistogram() LatencyHistogram {
	return newLatencyHistogram()
}

func (n *noopIntelligenceMetrics) GetCurrentStats() *IntelligenceStats {
	return &IntelligenceStats{
		ActiveModels:         []string{},
		CircuitBreakerStates: map[string]string{},
	}
}

// ---------------------------------------------------------------------------
// In-memory implementation (for testing)
// ---------------------------------------------------------------------------

type inMemoryIntelligenceMetrics struct {
	mu sync.Mutex

	inferences  []*InferenceMetricParams
	batches     []*BatchMetricParams
	cacheHits   int64
	cacheMisses int64
	riskCounts  map[string]int64
	modelLoads  []modelLoadRecord
	cbStates    map[string]string
	latencyHist *latencyHistogram
}

type modelLoadRecord struct {
	ModelName  string
	Version    string
	DurationMs float64
	Success    bool
	Timestamp  time.Time
}

// NewInMemoryIntelligenceMetrics returns an in-memory metrics implementation
// suitable for unit tests.
func NewInMemoryIntelligenceMetrics() *inMemoryIntelligenceMetrics {
	return &inMemoryIntelligenceMetrics{
		riskCounts:  make(map[string]int64),
		cbStates:    make(map[string]string),
		latencyHist: newLatencyHistogram(),
	}
}

func (m *inMemoryIntelligenceMetrics) RecordInference(_ context.Context, p *InferenceMetricParams) {
	if p == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *p
	m.inferences = append(m.inferences, &cp)
	m.latencyHist.observeUnlocked(p.DurationMs)
}

func (m *inMemoryIntelligenceMetrics) RecordBatchProcessing(_ context.Context, p *BatchMetricParams) {
	if p == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *p
	m.batches = append(m.batches, &cp)
}

func (m *inMemoryIntelligenceMetrics) RecordCacheAccess(_ context.Context, hit bool, _ string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if hit {
		m.cacheHits++
	} else {
		m.cacheMisses++
	}
}

func (m *inMemoryIntelligenceMetrics) RecordCircuitBreakerStateChange(_ context.Context, modelName, _, toState string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cbStates[modelName] = toState
}

func (m *inMemoryIntelligenceMetrics) RecordRiskAssessment(_ context.Context, riskLevel string, durationMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.riskCounts[riskLevel]++
	_ = durationMs
}

func (m *inMemoryIntelligenceMetrics) RecordModelLoad(_ context.Context, modelName, version string, durationMs float64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.modelLoads = append(m.modelLoads, modelLoadRecord{
		ModelName:  modelName,
		Version:    version,
		DurationMs: durationMs,
		Success:    success,
		Timestamp:  time.Now(),
	})
}

func (m *inMemoryIntelligenceMetrics) GetInferenceLatencyHistogram() LatencyHistogram {
	return m.latencyHist
}

func (m *inMemoryIntelligenceMetrics) GetCurrentStats() *IntelligenceStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := int64(len(m.inferences))
	var success, failed int64
	var sumLatency float64
	for _, inf := range m.inferences {
		if inf.Success {
			success++
		} else {
			failed++
		}
		sumLatency += inf.DurationMs
	}

	var avgLatency float64
	if total > 0 {
		avgLatency = sumLatency / float64(total)
	}

	hits := m.cacheHits
	misses := m.cacheMisses
	var hitRate float64
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses)
	}

	cbCopy := make(map[string]string, len(m.cbStates))
	for k, v := range m.cbStates {
		cbCopy[k] = v
	}

	return &IntelligenceStats{
		TotalInferences:       total,
		SuccessfulInferences:  success,
		FailedInferences:      failed,
		AvgInferenceLatencyMs: avgLatency,
		P50LatencyMs:          m.latencyHist.Percentile(50),
		P95LatencyMs:          m.latencyHist.Percentile(95),
		P99LatencyMs:          m.latencyHist.Percentile(99),
		CacheHitRate:          hitRate,
		ActiveModels:          []string{},
		CircuitBreakerStates:  cbCopy,
	}
}

// GetRecordedInferences returns a copy of all recorded inference params.
func (m *inMemoryIntelligenceMetrics) GetRecordedInferences() []*InferenceMetricParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*InferenceMetricParams, len(m.inferences))
	for i, p := range m.inferences {
		cp := *p
		out[i] = &cp
	}
	return out
}

// GetRecordedBatches returns a copy of all recorded batch params.
func (m *inMemoryIntelligenceMetrics) GetRecordedBatches() []*BatchMetricParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*BatchMetricParams, len(m.batches))
	for i, p := range m.batches {
		cp := *p
		out[i] = &cp
	}
	return out
}

// GetCacheHits returns the number of cache hits recorded.
func (m *inMemoryIntelligenceMetrics) GetCacheHits() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cacheHits
}

// GetCacheMisses returns the number of cache misses recorded.
func (m *inMemoryIntelligenceMetrics) GetCacheMisses() int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cacheMisses
}

// GetRiskCounts returns a copy of the risk-level counts.
func (m *inMemoryIntelligenceMetrics) GetRiskCounts() map[string]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]int64, len(m.riskCounts))
	for k, v := range m.riskCounts {
		out[k] = v
	}
	return out
}

// GetModelLoads returns a copy of all model load records.
func (m *inMemoryIntelligenceMetrics) GetModelLoads() []modelLoadRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]modelLoadRecord, len(m.modelLoads))
	copy(out, m.modelLoads)
	return out
}

// GetCircuitBreakerStates returns a copy of the circuit-breaker state map.
func (m *inMemoryIntelligenceMetrics) GetCircuitBreakerStates() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]string, len(m.cbStates))
	for k, v := range m.cbStates {
		out[k] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// latencyHistogram — in-memory, thread-safe, percentile-capable
// ---------------------------------------------------------------------------

type latencyHistogram struct {
	mu      sync.RWMutex
	samples []float64
	sum     float64
	sorted  bool
}

func newLatencyHistogram() *latencyHistogram {
	return &latencyHistogram{
		samples: make([]float64, 0, 1024),
	}
}

func (h *latencyHistogram) Observe(durationMs float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.observeUnlocked(durationMs)
}

// observeUnlocked is called when the caller already holds the lock (e.g.
// inMemoryIntelligenceMetrics which locks at a higher level).
func (h *latencyHistogram) observeUnlocked(durationMs float64) {
	h.samples = append(h.samples, durationMs)
	h.sum += durationMs
	h.sorted = false
}

// Percentile returns the value at percentile p (0–100) using linear
// interpolation between the two nearest ranks.
func (h *latencyHistogram) Percentile(p float64) float64 {
	h.mu.RLock()
	n := len(h.samples)
	if n == 0 {
		h.mu.RUnlock()
		return 0
	}

	// We need a sorted copy. If not sorted yet, upgrade to write lock.
	if !h.sorted {
		h.mu.RUnlock()
		h.mu.Lock()
		if !h.sorted {
			sort.Float64s(h.samples)
			h.sorted = true
		}
		h.mu.Unlock()
		h.mu.RLock()
	}

	defer h.mu.RUnlock()

	if p <= 0 {
		return h.samples[0]
	}
	if p >= 100 {
		return h.samples[n-1]
	}

	// Use the "C = 1" variant of the percentile formula (Excel PERCENTILE.INC).
	rank := (p / 100) * float64(n-1)
	lower := int(math.Floor(rank))
	upper := lower + 1
	if upper >= n {
		return h.samples[n-1]
	}
	frac := rank - float64(lower)
	return h.samples[lower] + frac*(h.samples[upper]-h.samples[lower])
}

func (h *latencyHistogram) Count() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return int64(len(h.samples))
}

func (h *latencyHistogram) Sum() float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sum
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func circuitBreakerStateToFloat(state string) float64 {
	switch state {
	case "closed":
		return 0
	case "half_open":
		return 1
	case "open":
		return 2
	default:
		return -1
	}
}

// compile-time interface checks
var (
	_ IntelligenceMetrics = (*prometheusIntelligenceMetrics)(nil)
	_ IntelligenceMetrics = (*noopIntelligenceMetrics)(nil)
	_ IntelligenceMetrics = (*inMemoryIntelligenceMetrics)(nil)
	_ LatencyHistogram    = (*latencyHistogram)(nil)
)

