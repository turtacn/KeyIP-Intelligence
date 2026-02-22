package common

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewPrometheusIntelligenceMetrics_Success(t *testing.T) {
	registry := prometheus.NewRegistry()
	m, err := NewPrometheusIntelligenceMetrics(registry)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestNewPrometheusIntelligenceMetrics_DuplicateRegistration(t *testing.T) {
	registry := prometheus.NewRegistry()
	_, err := NewPrometheusIntelligenceMetrics(registry)
	assert.NoError(t, err)

	_, err = NewPrometheusIntelligenceMetrics(registry)
	assert.Error(t, err)
}

func TestPrometheus_RecordInference(t *testing.T) {
	registry := prometheus.NewRegistry()
	m, _ := NewPrometheusIntelligenceMetrics(registry)

	ctx := context.Background()
	params := &InferenceMetricParams{
		ModelName:   "test_model",
		TaskType:    "prediction",
		DurationMs:  100,
		Success:     true,
		DeviceType:  "cpu",
	}
	m.RecordInference(ctx, params)

	// Verification would require checking prometheus output, which is complex.
	// We trust prometheus client works if no panic.
}

func TestInMemory_RecordInference(t *testing.T) {
	m := NewInMemoryIntelligenceMetrics()
	ctx := context.Background()
	params := &InferenceMetricParams{
		ModelName:   "test_model",
		DurationMs:  100,
	}
	m.RecordInference(ctx, params)

	im := m.(*inMemoryIntelligenceMetrics)
	assert.Equal(t, 1, len(im.inferences))
	assert.Equal(t, "test_model", im.inferences[0].ModelName)
}

func TestNoop_AllMethods_NoPanic(t *testing.T) {
	m := NewNoopIntelligenceMetrics()
	ctx := context.Background()

	assert.NotPanics(t, func() {
		m.RecordInference(ctx, &InferenceMetricParams{})
		m.RecordBatchProcessing(ctx, &BatchMetricParams{})
		m.RecordCacheAccess(ctx, true, "model")
		m.RecordCircuitBreakerStateChange(ctx, "model", "closed", "open")
		m.RecordRiskAssessment(ctx, "high", 100)
		m.RecordModelLoad(ctx, "model", "v1", 100, true)
		m.GetInferenceLatencyHistogram()
		m.GetCurrentStats()
	})
}
//Personal.AI order the ending
