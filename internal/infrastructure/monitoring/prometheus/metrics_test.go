package prometheus

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func newTestAppMetrics(t *testing.T) (*AppMetrics, MetricsCollector) {
	cfg := CollectorConfig{
		Namespace: "test",
		Subsystem: "metrics",
	}
	c, err := NewMetricsCollector(cfg, logging.NewNopLogger())
	require.NoError(t, err)
	m := NewAppMetrics(c)
	return m, c
}

func getMetricOutput(t *testing.T, collector MetricsCollector) string {
	handler := collector.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	return w.Body.String()
}

func TestNewAppMetrics_AllMetricsRegistered(t *testing.T) {
	m, _ := newTestAppMetrics(t)
	assert.NotNil(t, m)

	// Verify fields are not nil
	assert.NotNil(t, m.HTTPRequestsTotal)
	assert.NotNil(t, m.HTTPRequestDuration)
	assert.NotNil(t, m.AuthAttemptsTotal)
	assert.NotNil(t, m.PatentIngestTotal)
	assert.NotNil(t, m.AnalysisTasksTotal)
	assert.NotNil(t, m.GraphNodesTotal)
	assert.NotNil(t, m.LLMRequestsTotal)
	assert.NotNil(t, m.DBConnectionPoolSize)
	assert.NotNil(t, m.ServiceUptime)
}

func TestRecordHTTPRequest_AllMetricsUpdated(t *testing.T) {
	m, c := newTestAppMetrics(t)

	// Simulate middleware behavior for ActiveRequests
	m.HTTPActiveRequests.WithLabelValues("GET", "/test").Inc()
	RecordHTTPRequest(m, "GET", "/test", 200, 100*time.Millisecond, 123, 456)
	m.HTTPActiveRequests.WithLabelValues("GET", "/test").Dec()

	output := getMetricOutput(t, c)

	// RequestsTotal increased
	assert.Contains(t, output, "http_requests_total")
	// Duration observed (bucket count increased)
	assert.Contains(t, output, "http_request_duration_seconds_bucket")
	// Size observed
	assert.Contains(t, output, "http_request_size_bytes_bucket")
	// ActiveRequests should be 0 (inc then dec)
	// But it existed, so it should be present.
	assert.Contains(t, output, "http_active_requests")
}

func TestRecordHTTPRequest_DifferentStatusCodes(t *testing.T) {
	m, c := newTestAppMetrics(t)

	RecordHTTPRequest(m, "GET", "/test", 200, 10*time.Millisecond, 0, 0)
	RecordHTTPRequest(m, "GET", "/test", 500, 10*time.Millisecond, 0, 0)

	output := getMetricOutput(t, c)
	assert.Contains(t, output, "status_code=\"200\"")
	assert.Contains(t, output, "status_code=\"500\"")
}

func TestRecordAuthAttempt_Success(t *testing.T) {
	m, c := newTestAppMetrics(t)

	RecordAuthAttempt(m, true, "", 50*time.Millisecond)

	output := getMetricOutput(t, c)
	assert.Contains(t, output, "result=\"success\"")
}

func TestRecordAuthAttempt_Failure(t *testing.T) {
	m, c := newTestAppMetrics(t)

	RecordAuthAttempt(m, false, "invalid_password", 50*time.Millisecond)

	output := getMetricOutput(t, c)
	assert.Contains(t, output, "result=\"failure\"")
	assert.Contains(t, output, "failure_reason=\"invalid_password\"")
}

func TestRecordLLMCall_Success(t *testing.T) {
	m, c := newTestAppMetrics(t)

	RecordLLMCall(m, "gpt-4", "chat", true, 1*time.Second, 100, 50, 0.03)

	output := getMetricOutput(t, c)
	assert.Contains(t, output, "model=\"gpt-4\"")
	assert.Contains(t, output, "operation=\"chat\"")
	assert.Contains(t, output, "status=\"success\"")
	// Check tokens
	assert.Contains(t, output, "direction=\"input\"")
	assert.Contains(t, output, "direction=\"output\"")
	// Check cost
	assert.Contains(t, output, "llm_cost_total")
}

func TestRecordDBQuery_Success(t *testing.T) {
	m, c := newTestAppMetrics(t)

	RecordDBQuery(m, "postgres", "select", 10*time.Millisecond, nil)

	output := getMetricOutput(t, c)
	assert.Contains(t, output, "db=\"postgres\"")
	assert.Contains(t, output, "operation=\"select\"")
}

func TestRecordDBQuery_Error(t *testing.T) {
	m, c := newTestAppMetrics(t)

	RecordDBQuery(m, "postgres", "select", 10*time.Millisecond, fmt.Errorf("db error"))

	output := getMetricOutput(t, c)
	assert.Contains(t, output, "errors_total")
	assert.Contains(t, output, "error_type=\"query_error\"")
}

func TestRecordCacheAccess_Hit(t *testing.T) {
	m, c := newTestAppMetrics(t)
	RecordCacheAccess(m, "redis", true)
	output := getMetricOutput(t, c)
	assert.Contains(t, output, "cache_hits_total")
}

func TestRecordCacheAccess_Miss(t *testing.T) {
	m, c := newTestAppMetrics(t)
	RecordCacheAccess(m, "redis", false)
	output := getMetricOutput(t, c)
	assert.Contains(t, output, "cache_misses_total")
}

func TestMetricNaming_FollowsConvention(t *testing.T) {
	_, c := newTestAppMetrics(t)
	output := getMetricOutput(t, c)
	// All metrics should start with test_metrics_ (namespace_subsystem_)
	// We scan lines and check
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# TYPE") {
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				metricName := parts[2]
				assert.True(t, strings.HasPrefix(metricName, "test_metrics_"), "Metric %s does not follow convention", metricName)
			}
		}
	}
}

func TestDefaultBuckets_HTTPDuration(t *testing.T) {
	m, c := newTestAppMetrics(t)
	RecordHTTPRequest(m, "GET", "/", 200, 5*time.Millisecond, 0, 0) // exact boundary
	output := getMetricOutput(t, c)
	// Check buckets exist
	assert.Contains(t, output, "le=\"0.005\"")
	assert.Contains(t, output, "le=\"10\"")
}
//Personal.AI order the ending
