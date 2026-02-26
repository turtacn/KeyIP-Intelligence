package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// Mock Logger
type mockLogger struct {
	logging.Logger
}

func (m *mockLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockLogger) WithError(err error) logging.Logger { return m }
func (m *mockLogger) Sync() error { return nil }

func newMockLogger() logging.Logger {
	return &mockLogger{}
}

func newTestCollector(t *testing.T) MetricsCollector {
	cfg := CollectorConfig{
		Namespace:            "test",
		Subsystem:            "unit",
		EnableProcessMetrics: false,
		EnableGoMetrics:      false,
	}
	c, err := NewMetricsCollector(cfg, newMockLogger())
	require.NoError(t, err)
	return c
}

func scrapeMetrics(t *testing.T, collector MetricsCollector) string {
	handler := collector.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	return rr.Body.String()
}

func assertMetricExists(t *testing.T, output, metricName string) {
	assert.Contains(t, output, metricName)
}

func assertMetricValue(t *testing.T, output, metricName string, expectedValue float64) {
	// Simple check: "metricName value"
	// Note: Prometheus output format varies, but usually "name value" or "name{labels} value"
	// This is a loose check.
	// For exact check, we might need a parser or stricter regex.
	// But for unit test, containing "name value" is often enough if unique.
	// Or use expfmt parser. But let's keep it simple string check first.
	// Format: metric_name{...} value
	// We check for substring.
	// E.g. test_unit_counter 1

	// Because labels might be present or not, we search for the metric name line.
	lines := strings.Split(output, "\n")
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, metricName) {
			// Extract value
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Last part is usually value
				valStr := parts[len(parts)-1]
				// Check if it matches expected
				if fmt.Sprintf("%v", expectedValue) == valStr || fmt.Sprintf("%.1f", expectedValue) == valStr {
					found = true
					break
				}
				// Also try int format if float is integer
				if fmt.Sprintf("%d", int(expectedValue)) == valStr {
					found = true
					break
				}
			}
		}
	}
	assert.True(t, found, "Metric %s with value %v not found", metricName, expectedValue)
}

func TestNewMetricsCollector_ValidConfig(t *testing.T) {
	c := newTestCollector(t)
	assert.NotNil(t, c)
}

func TestNewMetricsCollector_EmptyNamespace(t *testing.T) {
	_, err := NewMetricsCollector(CollectorConfig{}, newMockLogger())
	assert.Error(t, err)
}

func TestRegisterCounter_Success(t *testing.T) {
	c := newTestCollector(t)
	counter := c.RegisterCounter("requests_total", "Total requests")
	counter.WithLabelValues().Inc()
	counter.WithLabelValues().Add(5)

	output := scrapeMetrics(t, c)
	assertMetricExists(t, output, "test_unit_requests_total")
	assertMetricValue(t, output, "test_unit_requests_total", 6)
}

func TestRegisterCounter_WithLabels(t *testing.T) {
	c := newTestCollector(t)
	counter := c.RegisterCounter("labeled_total", "Labeled", "method")
	counter.WithLabelValues("GET").Inc()
	counter.With(map[string]string{"method": "POST"}).Add(2)

	output := scrapeMetrics(t, c)
	assert.Contains(t, output, `test_unit_labeled_total{method="GET"} 1`)
	assert.Contains(t, output, `test_unit_labeled_total{method="POST"} 2`)
}

func TestRegisterCounter_Duplicate(t *testing.T) {
	c := newTestCollector(t)
	c1 := c.RegisterCounter("dup_counter", "Duplicate")
	c2 := c.RegisterCounter("dup_counter", "Duplicate")

	c1.WithLabelValues().Inc()
	c2.WithLabelValues().Inc()

	output := scrapeMetrics(t, c)
	assertMetricValue(t, output, "test_unit_dup_counter", 2)
}

func TestRegisterGauge_Success(t *testing.T) {
	c := newTestCollector(t)
	gauge := c.RegisterGauge("active_users", "Active users")
	gauge.WithLabelValues().Set(10)
	gauge.WithLabelValues().Inc() // 11
	gauge.WithLabelValues().Dec() // 10
	gauge.WithLabelValues().Add(5) // 15
	gauge.WithLabelValues().Sub(2) // 13

	output := scrapeMetrics(t, c)
	assertMetricValue(t, output, "test_unit_active_users", 13)
}

func TestRegisterHistogram_Observe(t *testing.T) {
	c := newTestCollector(t)
	hist := c.RegisterHistogram("duration_seconds", "Duration", []float64{1, 2, 5})
	hist.WithLabelValues().Observe(0.5)
	hist.WithLabelValues().Observe(1.5)
	hist.WithLabelValues().Observe(3)

	output := scrapeMetrics(t, c)
	assertMetricExists(t, output, "test_unit_duration_seconds_bucket")
	assertMetricExists(t, output, "test_unit_duration_seconds_sum")
	assertMetricExists(t, output, "test_unit_duration_seconds_count")

	// Check count
	assertMetricValue(t, output, "test_unit_duration_seconds_count", 3)
}

func TestTimer_MeasuresDuration(t *testing.T) {
	c := newTestCollector(t)
	hist := c.RegisterHistogram("timer_seconds", "Timer", nil)

	timer := NewTimer(hist.WithLabelValues())
	time.Sleep(10 * time.Millisecond)
	timer.ObserveDuration()

	output := scrapeMetrics(t, c)
	assertMetricValue(t, output, "test_unit_timer_seconds_count", 1)
}

func TestConcurrentRegistration(t *testing.T) {
	c := newTestCollector(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("metric_%d", i)
			c.RegisterCounter(name, "help")
		}(i)
	}
	wg.Wait()
}

func TestConcurrentIncrement(t *testing.T) {
	c := newTestCollector(t)
	counter := c.RegisterCounter("concurrent_inc", "Concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.WithLabelValues().Inc()
		}()
	}
	wg.Wait()

	output := scrapeMetrics(t, c)
	assertMetricValue(t, output, "test_unit_concurrent_inc", 100)
}

func TestUnregister_Success(t *testing.T) {
	c := newTestCollector(t)
	// We need a prometheus.Collector to unregister.
	// But RegisterCounter returns CounterVec wrapper.
	// We can't easily unregister via wrapper unless we expose the underlying collector.
	// However, MetricsCollector interface has Unregister(collector prometheus.Collector).
	// This implies we need access to the underlying collector.
	// In the implementation, I didn't expose a way to get the underlying collector from wrapper.
	// So `Unregister` method in interface is useful if we used `MustRegister` with custom collector.

	custom := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "custom_metric",
		Help: "Custom",
	})
	c.MustRegister(custom)

	output := scrapeMetrics(t, c)
	assertMetricExists(t, output, "custom_metric")

	unreg := c.Unregister(custom)
	assert.True(t, unreg)

	output = scrapeMetrics(t, c)
	assert.NotContains(t, output, "custom_metric")
}

func TestNoopCounter_NoError(t *testing.T) {
	c := newTestCollector(t)

	// Register a counter first
	c.RegisterCounter("same_name", "help")

	// Try to register a gauge with the same name (should fail type check)
	gauge := c.RegisterGauge("same_name", "help")

	// Should return a no-op gauge that doesn't panic
	gauge.WithLabelValues().Inc()
	gauge.WithLabelValues().Set(10)
	gauge.With(map[string]string{"label": "val"}).Add(5)
}
//Personal.AI order the ending
