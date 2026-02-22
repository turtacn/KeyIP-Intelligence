package common

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

func collectCounterValue(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		return 0
	}
	return m.GetCounter().GetValue()
}

func collectGaugeValue(g prometheus.Gauge) float64 {
	m := &dto.Metric{}
	if err := g.Write(m); err != nil {
		return 0
	}
	return m.GetGauge().GetValue()
}

func collectHistogramCount(h prometheus.Observer) uint64 {
	type writeable interface {
		Write(*dto.Metric) error
	}
	w, ok := h.(writeable)
	if !ok {
		return 0
	}
	m := &dto.Metric{}
	if err := w.Write(m); err != nil {
		return 0
	}
	return m.GetHistogram().GetSampleCount()
}

func generateLatencySamples(n int, minMs, maxMs float64) []float64 {
	r := rand.New(rand.NewSource(42))
	out := make([]float64, n)
	for i := range out {
		out[i] = minMs + r.Float64()*(maxMs-minMs)
	}
	return out
}

func inDelta(t *testing.T, expected, actual, delta float64, msg string) {
	t.Helper()
	if math.Abs(expected-actual) > delta {
		t.Errorf("%s: expected %f ± %f, got %f", msg, expected, delta, actual)
	}
}

// ---------------------------------------------------------------------------
// Prometheus implementation tests
// ---------------------------------------------------------------------------

func TestNewPrometheusIntelligenceMetrics_Success(t *testing.T) {
	reg := newTestRegistry()
	m, err := NewPrometheusIntelligenceMetrics(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil metrics")
	}
}

func TestNewPrometheusIntelligenceMetrics_DuplicateRegistration(t *testing.T) {
	reg := newTestRegistry()
	_, err := NewPrometheusIntelligenceMetrics(reg)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	_, err = NewPrometheusIntelligenceMetrics(reg)
	if err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestPrometheus_RecordInference_Success(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	ctx := context.Background()
	m.RecordInference(ctx, &InferenceMetricParams{
		ModelName:    "gnn-v1",
		ModelVersion: "1.0.0",
		TaskType:     "embed",
		DurationMs:   42.5,
		Success:      true,
		BatchSize:    1,
		DeviceType:   "gpu",
	})

	counter, err := m.inferenceTotal.GetMetricWithLabelValues("gnn-v1", "embed", "success")
	if err != nil {
		t.Fatalf("get counter: %v", err)
	}
	val := collectCounterValue(counter)
	if val != 1 {
		t.Errorf("expected counter 1, got %f", val)
	}

	hist := m.inferenceLatency.WithLabelValues("gnn-v1", "1.0.0", "embed", "gpu")
	histCount := collectHistogramCount(hist)
	if histCount != 1 {
		t.Errorf("expected histogram count 1, got %d", histCount)
	}
}

func TestPrometheus_RecordInference_Failure(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordInference(context.Background(), &InferenceMetricParams{
		ModelName:    "gnn-v1",
		ModelVersion: "1.0.0",
		TaskType:     "embed",
		DurationMs:   100.0,
		Success:      false,
		BatchSize:    1,
		DeviceType:   "cpu",
	})

	counter, err := m.inferenceTotal.GetMetricWithLabelValues("gnn-v1", "embed", "failure")
	if err != nil {
		t.Fatalf("get counter: %v", err)
	}
	val := collectCounterValue(counter)
	if val != 1 {
		t.Errorf("expected failure counter 1, got %f", val)
	}
}

func TestPrometheus_RecordInference_Labels(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	params := &InferenceMetricParams{
		ModelName:    "infringenet-v2",
		ModelVersion: "2.1.0",
		TaskType:     "structural_similarity",
		DurationMs:   55.0,
		Success:      true,
		BatchSize:    4,
		DeviceType:   "gpu",
	}
	m.RecordInference(context.Background(), params)

	// Verify the specific label combination exists and has count 1
	hist := m.inferenceLatency.WithLabelValues("infringenet-v2", "2.1.0", "structural_similarity", "gpu")
	count := collectHistogramCount(hist)
	if count != 1 {
		t.Errorf("expected histogram count 1 for specific labels, got %d", count)
	}

	counter, _ := m.inferenceTotal.GetMetricWithLabelValues("infringenet-v2", "structural_similarity", "success")
	val := collectCounterValue(counter)
	if val != 1 {
		t.Errorf("expected counter 1, got %f", val)
	}
}

func TestPrometheus_RecordInference_DefaultDeviceType(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordInference(context.Background(), &InferenceMetricParams{
		ModelName:    "gnn-v1",
		ModelVersion: "1.0.0",
		TaskType:     "embed",
		DurationMs:   10.0,
		Success:      true,
		BatchSize:    1,
		DeviceType:   "", // empty → should default to "cpu"
	})

	hist := m.inferenceLatency.WithLabelValues("gnn-v1", "1.0.0", "embed", "cpu")
	count := collectHistogramCount(hist)
	if count != 1 {
		t.Errorf("expected histogram count 1 with default device_type=cpu, got %d", count)
	}
}

func TestPrometheus_RecordInference_NilParams(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)
	// Should not panic
	m.RecordInference(context.Background(), nil)
}

func TestPrometheus_RecordBatchProcessing(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordBatchProcessing(context.Background(), &BatchMetricParams{
		BatchName:        "mol_embed_batch",
		TotalItems:       100,
		SuccessItems:     90,
		FailedItems:      5,
		TimeoutItems:     3,
		CancelledItems:   2,
		TotalDurationMs:  5000.0,
		AvgItemDurationMs: 50.0,
		MaxConcurrency:   8,
	})

	successCounter, _ := m.batchItemsTotal.GetMetricWithLabelValues("mol_embed_batch", "success")
	failedCounter, _ := m.batchItemsTotal.GetMetricWithLabelValues("mol_embed_batch", "failed")
	timeoutCounter, _ := m.batchItemsTotal.GetMetricWithLabelValues("mol_embed_batch", "timeout")
	cancelledCounter, _ := m.batchItemsTotal.GetMetricWithLabelValues("mol_embed_batch", "cancelled")

	if v := collectCounterValue(successCounter); v != 90 {
		t.Errorf("success items: expected 90, got %f", v)
	}
	if v := collectCounterValue(failedCounter); v != 5 {
		t.Errorf("failed items: expected 5, got %f", v)
	}
	if v := collectCounterValue(timeoutCounter); v != 3 {
		t.Errorf("timeout items: expected 3, got %f", v)
	}
	if v := collectCounterValue(cancelledCounter); v != 2 {
		t.Errorf("cancelled items: expected 2, got %f", v)
	}

	hist := m.batchProcessingDuration.WithLabelValues("mol_embed_batch")
	count := collectHistogramCount(hist)
	if count != 1 {
		t.Errorf("expected batch duration histogram count 1, got %d", count)
	}
}

func TestPrometheus_RecordBatchProcessing_NilParams(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)
	m.RecordBatchProcessing(context.Background(), nil)
}

func TestPrometheus_RecordCacheAccess_Hit(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordCacheAccess(context.Background(), true, "gnn-v1")

	hitCounter, _ := m.cacheAccessTotal.GetMetricWithLabelValues("gnn-v1", "hit")
	if v := collectCounterValue(hitCounter); v != 1 {
		t.Errorf("expected hit counter 1, got %f", v)
	}
}

func TestPrometheus_RecordCacheAccess_Miss(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordCacheAccess(context.Background(), false, "gnn-v1")

	missCounter, _ := m.cacheAccessTotal.GetMetricWithLabelValues("gnn-v1", "miss")
	if v := collectCounterValue(missCounter); v != 1 {
		t.Errorf("expected miss counter 1, got %f", v)
	}
}

func TestPrometheus_RecordCacheAccess_Multiple(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	for i := 0; i < 7; i++ {
		m.RecordCacheAccess(context.Background(), true, "model-a")
	}
	for i := 0; i < 3; i++ {
		m.RecordCacheAccess(context.Background(), false, "model-a")
	}

	hitCounter, _ := m.cacheAccessTotal.GetMetricWithLabelValues("model-a", "hit")
	missCounter, _ := m.cacheAccessTotal.GetMetricWithLabelValues("model-a", "miss")
	if v := collectCounterValue(hitCounter); v != 7 {
		t.Errorf("expected 7 hits, got %f", v)
	}
	if v := collectCounterValue(missCounter); v != 3 {
		t.Errorf("expected 3 misses, got %f", v)
	}
}

func TestPrometheus_RecordCircuitBreakerStateChange(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	tests := []struct {
		toState  string
		expected float64
	}{
		{"closed", 0},
		{"half_open", 1},
		{"open", 2},
	}

	for _, tt := range tests {
		m.RecordCircuitBreakerStateChange(context.Background(), "gnn-v1", "", tt.toState)
		gauge, _ := m.circuitBreakerState.GetMetricWithLabelValues("gnn-v1")
		val := collectGaugeValue(gauge)
		if val != tt.expected {
			t.Errorf("state %s: expected gauge %f, got %f", tt.toState, tt.expected, val)
		}
	}
}

func TestPrometheus_RecordRiskAssessment(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordRiskAssessment(context.Background(), "high", 120.0)
	m.RecordRiskAssessment(context.Background(), "high", 130.0)
	m.RecordRiskAssessment(context.Background(), "medium", 80.0)
	m.RecordRiskAssessment(context.Background(), "low", 50.0)

	highCounter, _ := m.riskAssessmentTotal.GetMetricWithLabelValues("high")
	medCounter, _ := m.riskAssessmentTotal.GetMetricWithLabelValues("medium")
	lowCounter, _ := m.riskAssessmentTotal.GetMetricWithLabelValues("low")

	if v := collectCounterValue(highCounter); v != 2 {
		t.Errorf("high risk: expected 2, got %f", v)
	}
	if v := collectCounterValue(medCounter); v != 1 {
		t.Errorf("medium risk: expected 1, got %f", v)
	}
	if v := collectCounterValue(lowCounter); v != 1 {
		t.Errorf("low risk: expected 1, got %f", v)
	}

	hist := m.riskAssessmentDuration.WithLabelValues("high")
	count := collectHistogramCount(hist)
	if count != 2 {
		t.Errorf("high risk duration histogram: expected 2, got %d", count)
	}
}

func TestPrometheus_RecordModelLoad_Success(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordModelLoad(context.Background(), "gnn-v1", "1.0.0", 3500.0, true)

	hist := m.modelLoadDuration.WithLabelValues("gnn-v1", "1.0.0", "success")
	count := collectHistogramCount(hist)
	if count != 1 {
		t.Errorf("expected model load histogram count 1, got %d", count)
	}
}

func TestPrometheus_RecordModelLoad_Failure(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	m.RecordModelLoad(context.Background(), "gnn-v1", "1.0.0", 1000.0, false)

	hist := m.modelLoadDuration.WithLabelValues("gnn-v1", "1.0.0", "failure")
	count := collectHistogramCount(hist)
	if count != 1 {
		t.Errorf("expected model load histogram count 1, got %d", count)
	}
}

func TestPrometheus_GetInferenceLatencyHistogram(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)

	h := m.GetInferenceLatencyHistogram()
	if h == nil {
		t.Fatal("expected non-nil LatencyHistogram")
	}
}

func TestPrometheus_GetCurrentStats(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)
	ctx := context.Background()

	for i := 0; i < 8; i++ {
		m.RecordInference(ctx, &InferenceMetricParams{
			ModelName:    "gnn-v1",
			ModelVersion: "1.0.0",
			TaskType:     "embed",
			DurationMs:   float64(10 * (i + 1)),
			Success:      true,
			BatchSize:    1,
		})
	}
	m.RecordInference(ctx, &InferenceMetricParams{
		ModelName:    "gnn-v1",
		ModelVersion: "1.0.0",
		TaskType:     "embed",
		DurationMs:   200.0,
		Success:      false,
		BatchSize:    1,
	})

	m.RecordCacheAccess(ctx, true, "gnn-v1")
	m.RecordCacheAccess(ctx, true, "gnn-v1")
	m.RecordCacheAccess(ctx, false, "gnn-v1")

	m.RecordCircuitBreakerStateChange(ctx, "gnn-v1", "closed", "open")

	stats := m.GetCurrentStats()
	if stats.TotalInferences != 9 {
		t.Errorf("total inferences: expected 9, got %d", stats.TotalInferences)
	}
	if stats.SuccessfulInferences != 8 {
		t.Errorf("successful: expected 8, got %d", stats.SuccessfulInferences)
	}
	if stats.FailedInferences != 1 {
		t.Errorf("failed: expected 1, got %d", stats.FailedInferences)
	}
	if stats.AvgInferenceLatencyMs <= 0 {
		t.Error("expected positive avg latency")
	}
	// Cache hit rate: 2 / 3 ≈ 0.6667
	inDelta(t, 0.6667, stats.CacheHitRate, 0.01, "cache hit rate")

	if state, ok := stats.CircuitBreakerStates["gnn-v1"]; !ok || state != "open" {
		t.Errorf("expected circuit breaker state 'open', got %q", state)
	}
}

func TestPrometheus_ConcurrentRecording(t *testing.T) {
	reg := newTestRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(reg)
	ctx := context.Background()

	const goroutines = 100
	const recordsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < recordsPerGoroutine; i++ {
				m.RecordInference(ctx, &InferenceMetricParams{
					ModelName:    "gnn-v1",
					ModelVersion: "1.0.0",
					TaskType:     "embed",
					DurationMs:   float64(i + 1),
					Success:      i%5 != 0,
					BatchSize:    1,
					DeviceType:   "gpu",
				})
				m.RecordCacheAccess(ctx, i%2 == 0, "gnn-v1")
			}
		}(g)
	}
	wg.Wait()

	stats := m.GetCurrentStats()
	expectedTotal := int64(goroutines * recordsPerGoroutine)
	if stats.TotalInferences != expectedTotal {
		t.Errorf("total inferences: expected %d, got %d", expectedTotal, stats.TotalInferences)
	}

	counter, _ := m.inferenceTotal.GetMetricWithLabelValues("gnn-v1", "embed", "success")
	successVal := collectCounterValue(counter)
	failCounter, _ := m.inferenceTotal.GetMetricWithLabelValues("gnn-v1", "embed", "failure")
	failVal := collectCounterValue(failCounter)

	totalCounter := int64(successVal + failVal)
	if totalCounter != expectedTotal {
		t.Errorf("prometheus counter total: expected %d, got %d", expectedTotal, totalCounter)
	}
}

// ---------------------------------------------------------------------------
// Noop implementation tests
// ---------------------------------------------------------------------------

func TestNoop_AllMethods_NoPanic(t *testing.T) {
	n := NewNoopIntelligenceMetrics()
	ctx := context.Background()

	n.RecordInference(ctx, &InferenceMetricParams{ModelName: "test"})
	n.RecordInference(ctx, nil)
	n.RecordBatchProcessing(ctx, &BatchMetricParams{BatchName: "test"})
	n.RecordBatchProcessing(ctx, nil)
	n.RecordCacheAccess(ctx, true, "test")
	n.RecordCacheAccess(ctx, false, "test")
	n.RecordCircuitBreakerStateChange(ctx, "test", "closed", "open")
	n.RecordRiskAssessment(ctx, "high", 100.0)
	n.RecordModelLoad(ctx, "test", "1.0", 500.0, true)
	n.RecordModelLoad(ctx, "test", "1.0", 500.0, false)

	h := n.GetInferenceLatencyHistogram()
	if h == nil {
		t.Fatal("expected non-nil histogram from noop")
	}

	stats := n.GetCurrentStats()
	if stats == nil {
		t.Fatal("expected non-nil stats from noop")
	}
}

func TestNoop_GetCurrentStats_ZeroValues(t *testing.T) {
	n := NewNoopIntelligenceMetrics()
	stats := n.GetCurrentStats()

	if stats.TotalInferences != 0 {
		t.Errorf("expected 0 total inferences, got %d", stats.TotalInferences)
	}
	if stats.SuccessfulInferences != 0 {
		t.Errorf("expected 0 successful, got %d", stats.SuccessfulInferences)
	}
	if stats.FailedInferences != 0 {
		t.Errorf("expected 0 failed, got %d", stats.FailedInferences)
	}
	if stats.AvgInferenceLatencyMs != 0 {
		t.Errorf("expected 0 avg latency, got %f", stats.AvgInferenceLatencyMs)
	}
	if stats.CacheHitRate != 0 {
		t.Errorf("expected 0 cache hit rate, got %f", stats.CacheHitRate)
	}
}

// ---------------------------------------------------------------------------
// InMemory implementation tests
// ---------------------------------------------------------------------------

func TestInMemory_RecordInference(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	m.RecordInference(context.Background(), &InferenceMetricParams{
		ModelName:  "gnn-v1",
		TaskType:   "embed",
		DurationMs: 42.0,
		Success:    true,
		BatchSize:  1,
	})

	records := m.GetRecordedInferences()
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ModelName != "gnn-v1" {
		t.Errorf("expected model_name gnn-v1, got %s", records[0].ModelName)
	}
	if records[0].DurationMs != 42.0 {
		t.Errorf("expected duration 42.0, got %f", records[0].DurationMs)
	}
}

func TestInMemory_RecordInference_Multiple(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		m.RecordInference(ctx, &InferenceMetricParams{
			ModelName:  "gnn-v1",
			TaskType:   "embed",
			DurationMs: float64(i * 10),
			Success:    i%2 == 0,
			BatchSize:  1,
		})
	}

	records := m.GetRecordedInferences()
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}
}

func TestInMemory_RecordInference_NilParams(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	m.RecordInference(context.Background(), nil)
	records := m.GetRecordedInferences()
	if len(records) != 0 {
		t.Errorf("expected 0 records for nil params, got %d", len(records))
	}
}

func TestInMemory_RecordBatchProcessing(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	m.RecordBatchProcessing(context.Background(), &BatchMetricParams{
		BatchName:    "test_batch",
		TotalItems:   50,
		SuccessItems: 45,
		FailedItems:  5,
	})

	batches := m.GetRecordedBatches()
	if len(batches) != 1 {
		t.Fatalf("expected 1 batch record, got %d", len(batches))
	}
	if batches[0].TotalItems != 50 {
		t.Errorf("expected 50 total items, got %d", batches[0].TotalItems)
	}
}

func TestInMemory_RecordCacheAccess_Counting(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	for i := 0; i < 7; i++ {
		m.RecordCacheAccess(ctx, true, "model-a")
	}
	for i := 0; i < 3; i++ {
		m.RecordCacheAccess(ctx, false, "model-a")
	}

	if m.GetCacheHits() != 7 {
		t.Errorf("expected 7 hits, got %d", m.GetCacheHits())
	}
	if m.GetCacheMisses() != 3 {
		t.Errorf("expected 3 misses, got %d", m.GetCacheMisses())
	}
}

func TestInMemory_RecordRiskAssessment(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	m.RecordRiskAssessment(ctx, "high", 100.0)
	m.RecordRiskAssessment(ctx, "high", 110.0)
	m.RecordRiskAssessment(ctx, "low", 30.0)

	counts := m.GetRiskCounts()
	if counts["high"] != 2 {
		t.Errorf("expected 2 high, got %d", counts["high"])
	}
	if counts["low"] != 1 {
		t.Errorf("expected 1 low, got %d", counts["low"])
	}
}

func TestInMemory_RecordModelLoad(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	m.RecordModelLoad(ctx, "gnn-v1", "1.0.0", 3000.0, true)
	m.RecordModelLoad(ctx, "gnn-v1", "1.0.0", 500.0, false)

	loads := m.GetModelLoads()
	if len(loads) != 2 {
		t.Fatalf("expected 2 load records, got %d", len(loads))
	}
	if !loads[0].Success {
		t.Error("first load should be success")
	}
	if loads[1].Success {
		t.Error("second load should be failure")
	}
}

func TestInMemory_RecordCircuitBreakerStateChange(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	m.RecordCircuitBreakerStateChange(ctx, "gnn-v1", "closed", "open")
	m.RecordCircuitBreakerStateChange(ctx, "infringenet", "closed", "half_open")

	states := m.GetCircuitBreakerStates()
	if states["gnn-v1"] != "open" {
		t.Errorf("expected gnn-v1 state 'open', got %q", states["gnn-v1"])
	}
	if states["infringenet"] != "half_open" {
		t.Errorf("expected infringenet state 'half_open', got %q", states["infringenet"])
	}
}

func TestInMemory_GetCurrentStats_Aggregation(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	latencies := []float64{10, 20, 30, 40, 50}
	for i, lat := range latencies {
		m.RecordInference(ctx, &InferenceMetricParams{
			ModelName:  "gnn-v1",
			TaskType:   "embed",
			DurationMs: lat,
			Success:    i < 4, // 4 success, 1 failure
			BatchSize:  1,
		})
	}

	m.RecordCacheAccess(ctx, true, "gnn-v1")
	m.RecordCacheAccess(ctx, true, "gnn-v1")
	m.RecordCacheAccess(ctx, false, "gnn-v1")

	stats := m.GetCurrentStats()

	if stats.TotalInferences != 5 {
		t.Errorf("total: expected 5, got %d", stats.TotalInferences)
	}
	if stats.SuccessfulInferences != 4 {
		t.Errorf("success: expected 4, got %d", stats.SuccessfulInferences)
	}
	if stats.FailedInferences != 1 {
		t.Errorf("failed: expected 1, got %d", stats.FailedInferences)
	}
	// avg = (10+20+30+40+50)/5 = 30
	inDelta(t, 30.0, stats.AvgInferenceLatencyMs, 0.01, "avg latency")
	// cache hit rate = 2/3
	inDelta(t, 0.6667, stats.CacheHitRate, 0.01, "cache hit rate")
}

func TestInMemory_GetCurrentStats_Percentiles(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	// Insert 100 samples: 1, 2, 3, ..., 100
	for i := 1; i <= 100; i++ {
		m.RecordInference(ctx, &InferenceMetricParams{
			ModelName:  "gnn-v1",
			TaskType:   "embed",
			DurationMs: float64(i),
			Success:    true,
			BatchSize:  1,
		})
	}

	stats := m.GetCurrentStats()

	// P50 of [1..100]: rank = 0.50 * 99 = 49.5 → lerp(50, 51, 0.5) = 50.5
	inDelta(t, 50.5, stats.P50LatencyMs, 0.5, "P50")
	// P95: rank = 0.95 * 99 = 94.05 → lerp(95, 96, 0.05) = 95.05
	inDelta(t, 95.05, stats.P95LatencyMs, 0.5, "P95")
	// P99: rank = 0.99 * 99 = 98.01 → lerp(99, 100, 0.01) = 99.01
	inDelta(t, 99.01, stats.P99LatencyMs, 0.5, "P99")
}

func TestInMemory_ConcurrentRecording(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()

	const goroutines = 100
	const recordsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < recordsPerGoroutine; i++ {
				m.RecordInference(ctx, &InferenceMetricParams{
					ModelName:  "gnn-v1",
					TaskType:   "embed",
					DurationMs: float64(i),
					Success:    true,
					BatchSize:  1,
				})
				m.RecordCacheAccess(ctx, i%2 == 0, "gnn-v1")
				m.RecordRiskAssessment(ctx, "medium", float64(i))
			}
		}()
	}
	wg.Wait()

	records := m.GetRecordedInferences()
	expected := goroutines * recordsPerGoroutine
	if len(records) != expected {
		t.Errorf("expected %d records, got %d", expected, len(records))
	}

	stats := m.GetCurrentStats()
	if stats.TotalInferences != int64(expected) {
		t.Errorf("total inferences: expected %d, got %d", expected, stats.TotalInferences)
	}
}

// ---------------------------------------------------------------------------
// LatencyHistogram tests
// ---------------------------------------------------------------------------

func TestLatencyHistogram_SingleSample(t *testing.T) {
	h := newLatencyHistogram()
	h.Observe(42.0)

	if h.Count() != 1 {
		t.Errorf("count: expected 1, got %d", h.Count())
	}
	inDelta(t, 42.0, h.Sum(), 0.001, "sum")

	// All percentiles should return 42.0 for a single sample
	for _, p := range []float64{0, 25, 50, 75, 95, 99, 100} {
		val := h.Percentile(p)
		inDelta(t, 42.0, val, 0.001, "percentile")
	}
}

func TestLatencyHistogram_MultipleSamples(t *testing.T) {
	h := newLatencyHistogram()
	samples := []float64{5, 15, 25, 35, 45, 55, 65, 75, 85, 95}
	for _, s := range samples {
		h.Observe(s)
	}

	if h.Count() != 10 {
		t.Errorf("count: expected 10, got %d", h.Count())
	}
	expectedSum := 0.0
	for _, s := range samples {
		expectedSum += s
	}
	inDelta(t, expectedSum, h.Sum(), 0.001, "sum")

	// P0 → min = 5
	inDelta(t, 5.0, h.Percentile(0), 0.001, "P0")
	// P100 → max = 95
	inDelta(t, 95.0, h.Percentile(100), 0.001, "P100")
}

func TestLatencyHistogram_P50(t *testing.T) {
	h := newLatencyHistogram()
	// [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
	for i := 1; i <= 10; i++ {
		h.Observe(float64(i))
	}

	// P50: rank = 0.50 * 9 = 4.5 → lerp(5, 6, 0.5) = 5.5
	p50 := h.Percentile(50)
	inDelta(t, 5.5, p50, 0.5, "P50 of [1..10]")
}

func TestLatencyHistogram_P95(t *testing.T) {
	h := newLatencyHistogram()
	// 100 samples: 1, 2, 3, ..., 100
	for i := 1; i <= 100; i++ {
		h.Observe(float64(i))
	}

	// P95: rank = 0.95 * 99 = 94.05 → lerp(95, 96, 0.05) = 95.05
	p95 := h.Percentile(95)
	inDelta(t, 95.05, p95, 0.5, "P95 of [1..100]")
}

func TestLatencyHistogram_P99(t *testing.T) {
	h := newLatencyHistogram()
	// 1000 samples: 1, 2, 3, ..., 1000
	for i := 1; i <= 1000; i++ {
		h.Observe(float64(i))
	}

	// P99: rank = 0.99 * 999 = 989.01 → lerp(990, 991, 0.01) = 990.01
	p99 := h.Percentile(99)
	inDelta(t, 990.01, p99, 0.5, "P99 of [1..1000]")
}

func TestLatencyHistogram_Count(t *testing.T) {
	h := newLatencyHistogram()
	for i := 0; i < 42; i++ {
		h.Observe(float64(i))
	}
	if h.Count() != 42 {
		t.Errorf("count: expected 42, got %d", h.Count())
	}
}

func TestLatencyHistogram_Sum(t *testing.T) {
	h := newLatencyHistogram()
	var expected float64
	for i := 1; i <= 50; i++ {
		v := float64(i) * 1.5
		h.Observe(v)
		expected += v
	}
	inDelta(t, expected, h.Sum(), 0.001, "sum")
}

func TestLatencyHistogram_Empty(t *testing.T) {
	h := newLatencyHistogram()

	if h.Count() != 0 {
		t.Errorf("count: expected 0, got %d", h.Count())
	}
	if h.Sum() != 0 {
		t.Errorf("sum: expected 0, got %f", h.Sum())
	}
	if h.Percentile(50) != 0 {
		t.Errorf("P50 of empty: expected 0, got %f", h.Percentile(50))
	}
	if h.Percentile(99) != 0 {
		t.Errorf("P99 of empty: expected 0, got %f", h.Percentile(99))
	}
}

func TestLatencyHistogram_ConcurrentObserve(t *testing.T) {
	h := newLatencyHistogram()

	const goroutines = 100
	const samplesPerGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < samplesPerGoroutine; i++ {
				h.Observe(float64(id*samplesPerGoroutine + i))
			}
		}(g)
	}
	wg.Wait()

	expectedCount := int64(goroutines * samplesPerGoroutine)
	if h.Count() != expectedCount {
		t.Errorf("count: expected %d, got %d", expectedCount, h.Count())
	}

	// Percentile should not panic after concurrent writes
	_ = h.Percentile(50)
	_ = h.Percentile(95)
	_ = h.Percentile(99)
}

func TestLatencyHistogram_ConcurrentObserveAndRead(t *testing.T) {
	h := newLatencyHistogram()

	const writers = 50
	const readers = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(writers + readers)

	for w := 0; w < writers; w++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				h.Observe(float64(id*iterations + i))
			}
		}(w)
	}

	for r := 0; r < readers; r++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = h.Count()
				_ = h.Sum()
				_ = h.Percentile(50)
				_ = h.Percentile(95)
			}
		}()
	}

	wg.Wait()

	expectedCount := int64(writers * iterations)
	if h.Count() != expectedCount {
		t.Errorf("count: expected %d, got %d", expectedCount, h.Count())
	}
}

func TestLatencyHistogram_PercentileBoundary(t *testing.T) {
	h := newLatencyHistogram()
	h.Observe(10.0)
	h.Observe(20.0)

	// P0 → first element
	inDelta(t, 10.0, h.Percentile(0), 0.001, "P0")
	// P100 → last element
	inDelta(t, 20.0, h.Percentile(100), 0.001, "P100")
	// Negative → clamp to first
	inDelta(t, 10.0, h.Percentile(-10), 0.001, "P-10")
	// >100 → clamp to last
	inDelta(t, 20.0, h.Percentile(200), 0.001, "P200")
}

func TestLatencyHistogram_UnsortedInput(t *testing.T) {
	h := newLatencyHistogram()
	// Insert in reverse order
	for i := 100; i >= 1; i-- {
		h.Observe(float64(i))
	}

	// P50 should still be correct after internal sort
	p50 := h.Percentile(50)
	inDelta(t, 50.5, p50, 0.5, "P50 of reverse [100..1]")
}

func TestLatencyHistogram_RepeatedValues(t *testing.T) {
	h := newLatencyHistogram()
	for i := 0; i < 100; i++ {
		h.Observe(42.0)
	}

	if h.Count() != 100 {
		t.Errorf("count: expected 100, got %d", h.Count())
	}
	inDelta(t, 4200.0, h.Sum(), 0.001, "sum of 100 × 42")
	inDelta(t, 42.0, h.Percentile(50), 0.001, "P50 of constant")
	inDelta(t, 42.0, h.Percentile(99), 0.001, "P99 of constant")
}

// ---------------------------------------------------------------------------
// Parameter struct tests
// ---------------------------------------------------------------------------

func TestInferenceMetricParams_Validation(t *testing.T) {
	p := &InferenceMetricParams{
		ModelName:    "gnn-v1",
		ModelVersion: "1.0.0",
		TaskType:     "literal_prediction",
		DurationMs:   42.5,
		Success:      true,
		BatchSize:    1,
		InputTokens:  128,
		DeviceType:   "gpu",
	}

	if p.ModelName == "" {
		t.Error("ModelName should not be empty")
	}
	if p.ModelVersion == "" {
		t.Error("ModelVersion should not be empty")
	}
	if p.TaskType == "" {
		t.Error("TaskType should not be empty")
	}
	if p.DurationMs <= 0 {
		t.Error("DurationMs should be positive")
	}
	if p.BatchSize <= 0 {
		t.Error("BatchSize should be positive")
	}
	if p.InputTokens <= 0 {
		t.Error("InputTokens should be positive")
	}
	if p.DeviceType == "" {
		t.Error("DeviceType should not be empty")
	}
}

func TestBatchMetricParams_Validation(t *testing.T) {
	p := &BatchMetricParams{
		BatchName:         "mol_embed_batch",
		TotalItems:        100,
		SuccessItems:      90,
		FailedItems:       5,
		TimeoutItems:      3,
		CancelledItems:    2,
		TotalDurationMs:   5000.0,
		AvgItemDurationMs: 50.0,
		MaxConcurrency:    8,
	}

	if p.BatchName == "" {
		t.Error("BatchName should not be empty")
	}
	if p.TotalItems <= 0 {
		t.Error("TotalItems should be positive")
	}
	sum := p.SuccessItems + p.FailedItems + p.TimeoutItems + p.CancelledItems
	if sum != p.TotalItems {
		t.Errorf("item counts should sum to TotalItems: %d + %d + %d + %d = %d, want %d",
			p.SuccessItems, p.FailedItems, p.TimeoutItems, p.CancelledItems, sum, p.TotalItems)
	}
	if p.TotalDurationMs <= 0 {
		t.Error("TotalDurationMs should be positive")
	}
	if p.AvgItemDurationMs <= 0 {
		t.Error("AvgItemDurationMs should be positive")
	}
	if p.MaxConcurrency <= 0 {
		t.Error("MaxConcurrency should be positive")
	}
}

func TestIntelligenceStats_ZeroValue(t *testing.T) {
	s := &IntelligenceStats{}
	if s.TotalInferences != 0 {
		t.Error("zero value TotalInferences should be 0")
	}
	if s.CacheHitRate != 0 {
		t.Error("zero value CacheHitRate should be 0")
	}
}

// ---------------------------------------------------------------------------
// circuitBreakerStateToFloat helper test
// ---------------------------------------------------------------------------

func TestCircuitBreakerStateToFloat(t *testing.T) {
	tests := []struct {
		state    string
		expected float64
	}{
		{"closed", 0},
		{"half_open", 1},
		{"open", 2},
		{"unknown", -1},
		{"", -1},
	}
	for _, tt := range tests {
		got := circuitBreakerStateToFloat(tt.state)
		if got != tt.expected {
			t.Errorf("circuitBreakerStateToFloat(%q) = %f, want %f", tt.state, got, tt.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Interface compliance compile-time checks (redundant with metrics.go but
// validates that the test file sees the same types)
// ---------------------------------------------------------------------------

var (
	_ IntelligenceMetrics = (*prometheusIntelligenceMetrics)(nil)
	_ IntelligenceMetrics = (*noopIntelligenceMetrics)(nil)
	_ IntelligenceMetrics = (*inMemoryIntelligenceMetrics)(nil)
	_ LatencyHistogram    = (*latencyHistogram)(nil)
)

//Personal.AI order the ending


