package prometheus

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MetricsCollector defines the interface for metrics collection.
type MetricsCollector interface {
	RegisterCounter(name, help string, labels ...string) CounterVec
	RegisterGauge(name, help string, labels ...string) GaugeVec
	RegisterHistogram(name, help string, buckets []float64, labels ...string) HistogramVec
	RegisterSummary(name, help string, objectives map[float64]float64, labels ...string) SummaryVec
	Handler() http.Handler
	MustRegister(collectors ...prometheus.Collector)
	Unregister(collector prometheus.Collector) bool
}

// CounterVec wraps prometheus.CounterVec.
type CounterVec interface {
	WithLabelValues(lvs ...string) Counter
	With(labels map[string]string) Counter
}

// Counter wraps prometheus.Counter.
type Counter interface {
	Inc()
	Add(delta float64)
}

// GaugeVec wraps prometheus.GaugeVec.
type GaugeVec interface {
	WithLabelValues(lvs ...string) Gauge
	With(labels map[string]string) Gauge
}

// Gauge wraps prometheus.Gauge.
type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(delta float64)
	Sub(delta float64)
}

// HistogramVec wraps prometheus.HistogramVec.
type HistogramVec interface {
	WithLabelValues(lvs ...string) Histogram
	With(labels map[string]string) Histogram
}

// Histogram wraps prometheus.Histogram.
type Histogram interface {
	Observe(value float64)
}

// SummaryVec wraps prometheus.SummaryVec.
type SummaryVec interface {
	WithLabelValues(lvs ...string) Summary
	With(labels map[string]string) Summary
}

// Summary wraps prometheus.Summary.
type Summary interface {
	Observe(value float64)
}

// CollectorConfig holds configuration for the collector.
type CollectorConfig struct {
	Namespace             string
	Subsystem             string
	EnableProcessMetrics  bool
	EnableGoMetrics       bool
	DefaultHistogramBuckets []float64
	ConstLabels           map[string]string
}

// prometheusCollector implements MetricsCollector.
type prometheusCollector struct {
	registry          *prometheus.Registry
	config            CollectorConfig
	registeredMetrics map[string]prometheus.Collector
	mu                sync.RWMutex
	logger            logging.Logger
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector(cfg CollectorConfig, logger logging.Logger) (MetricsCollector, error) {
	if cfg.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	registry := prometheus.NewRegistry()

	if cfg.EnableProcessMetrics {
		registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
			Namespace: cfg.Namespace,
		}))
	}
	if cfg.EnableGoMetrics {
		registry.MustRegister(prometheus.NewGoCollector())
	}

	if cfg.DefaultHistogramBuckets == nil {
		cfg.DefaultHistogramBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	}

	return &prometheusCollector{
		registry:          registry,
		config:            cfg,
		registeredMetrics: make(map[string]prometheus.Collector),
		logger:            logger,
	}, nil
}

func (c *prometheusCollector) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

func (c *prometheusCollector) MustRegister(collectors ...prometheus.Collector) {
	c.registry.MustRegister(collectors...)
}

func (c *prometheusCollector) Unregister(collector prometheus.Collector) bool {
	return c.registry.Unregister(collector)
}

func (c *prometheusCollector) register(name string, newCollector prometheus.Collector) (prometheus.Collector, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fullName := prometheus.BuildFQName(c.config.Namespace, c.config.Subsystem, name)
	if existing, exists := c.registeredMetrics[fullName]; exists {
		return existing, nil
	}

	if err := c.registry.Register(newCollector); err != nil {
		return nil, err
	}
	c.registeredMetrics[fullName] = newCollector
	return newCollector, nil
}

func (c *prometheusCollector) RegisterCounter(name, help string, labels ...string) CounterVec {
	vec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   c.config.Namespace,
		Subsystem:   c.config.Subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: c.config.ConstLabels,
	}, labels)

	registered, err := c.register(name, vec)
	if err != nil {
		c.logger.Error("failed to register counter", logging.String("name", name), logging.Error(err))
		return &noopCounterVec{}
	}
	if v, ok := registered.(*prometheus.CounterVec); ok {
		return &promCounterVec{vec: v}
	}
	c.logger.Warn("metric type mismatch", logging.String("name", name), logging.String("type", "counter"))
	return &noopCounterVec{}
}

func (c *prometheusCollector) RegisterGauge(name, help string, labels ...string) GaugeVec {
	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   c.config.Namespace,
		Subsystem:   c.config.Subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: c.config.ConstLabels,
	}, labels)

	registered, err := c.register(name, vec)
	if err != nil {
		c.logger.Error("failed to register gauge", logging.String("name", name), logging.Error(err))
		return &noopGaugeVec{}
	}
	if v, ok := registered.(*prometheus.GaugeVec); ok {
		return &promGaugeVec{vec: v}
	}
	c.logger.Warn("metric type mismatch", logging.String("name", name), logging.String("type", "gauge"))
	return &noopGaugeVec{}
}

func (c *prometheusCollector) RegisterHistogram(name, help string, buckets []float64, labels ...string) HistogramVec {
	if buckets == nil {
		buckets = c.config.DefaultHistogramBuckets
	}
	vec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace:   c.config.Namespace,
		Subsystem:   c.config.Subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: c.config.ConstLabels,
		Buckets:     buckets,
	}, labels)

	registered, err := c.register(name, vec)
	if err != nil {
		c.logger.Error("failed to register histogram", logging.String("name", name), logging.Error(err))
		return &noopHistogramVec{}
	}
	if v, ok := registered.(*prometheus.HistogramVec); ok {
		return &promHistogramVec{vec: v}
	}
	c.logger.Warn("metric type mismatch", logging.String("name", name), logging.String("type", "histogram"))
	return &noopHistogramVec{}
}

func (c *prometheusCollector) RegisterSummary(name, help string, objectives map[float64]float64, labels ...string) SummaryVec {
	if objectives == nil {
		objectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
	}
	vec := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:   c.config.Namespace,
		Subsystem:   c.config.Subsystem,
		Name:        name,
		Help:        help,
		ConstLabels: c.config.ConstLabels,
		Objectives:  objectives,
	}, labels)

	registered, err := c.register(name, vec)
	if err != nil {
		c.logger.Error("failed to register summary", logging.String("name", name), logging.Error(err))
		return &noopSummaryVec{}
	}
	if v, ok := registered.(*prometheus.SummaryVec); ok {
		return &promSummaryVec{vec: v}
	}
	c.logger.Warn("metric type mismatch", logging.String("name", name), logging.String("type", "summary"))
	return &noopSummaryVec{}
}

// Wrappers

type promCounterVec struct{ vec *prometheus.CounterVec }

func (v *promCounterVec) WithLabelValues(lvs ...string) Counter {
	return &promCounter{c: v.vec.WithLabelValues(lvs...)}
}
func (v *promCounterVec) With(labels map[string]string) Counter {
	return &promCounter{c: v.vec.With(labels)}
}

type promCounter struct{ c prometheus.Counter }

func (c *promCounter) Inc()              { c.c.Inc() }
func (c *promCounter) Add(delta float64) { c.c.Add(delta) }

type promGaugeVec struct{ vec *prometheus.GaugeVec }

func (v *promGaugeVec) WithLabelValues(lvs ...string) Gauge {
	return &promGauge{g: v.vec.WithLabelValues(lvs...)}
}
func (v *promGaugeVec) With(labels map[string]string) Gauge {
	return &promGauge{g: v.vec.With(labels)}
}

type promGauge struct{ g prometheus.Gauge }

func (g *promGauge) Set(value float64)   { g.g.Set(value) }
func (g *promGauge) Inc()                { g.g.Inc() }
func (g *promGauge) Dec()                { g.g.Dec() }
func (g *promGauge) Add(delta float64)   { g.g.Add(delta) }
func (g *promGauge) Sub(delta float64)   { g.g.Sub(delta) }

type promHistogramVec struct{ vec *prometheus.HistogramVec }

func (v *promHistogramVec) WithLabelValues(lvs ...string) Histogram {
	return &promHistogram{h: v.vec.WithLabelValues(lvs...)}
}
func (v *promHistogramVec) With(labels map[string]string) Histogram {
	return &promHistogram{h: v.vec.With(labels)}
}

type promHistogram struct{ h prometheus.Observer }

func (h *promHistogram) Observe(value float64) { h.h.Observe(value) }

type promSummaryVec struct{ vec *prometheus.SummaryVec }

func (v *promSummaryVec) WithLabelValues(lvs ...string) Summary {
	return &promSummary{s: v.vec.WithLabelValues(lvs...)}
}
func (v *promSummaryVec) With(labels map[string]string) Summary {
	return &promSummary{s: v.vec.With(labels)}
}

type promSummary struct{ s prometheus.Observer }

func (s *promSummary) Observe(value float64) { s.s.Observe(value) }

// No-op implementations

type noopCounterVec struct{}

func (v *noopCounterVec) WithLabelValues(lvs ...string) Counter { return &noopCounter{} }
func (v *noopCounterVec) With(labels map[string]string) Counter { return &noopCounter{} }

type noopCounter struct{}

func (c *noopCounter) Inc()              {}
func (c *noopCounter) Add(delta float64) {}

type noopGaugeVec struct{}

func (v *noopGaugeVec) WithLabelValues(lvs ...string) Gauge { return &noopGauge{} }
func (v *noopGaugeVec) With(labels map[string]string) Gauge { return &noopGauge{} }

type noopGauge struct{}

func (g *noopGauge) Set(value float64)   {}
func (g *noopGauge) Inc()                {}
func (g *noopGauge) Dec()                {}
func (g *noopGauge) Add(delta float64)   {}
func (g *noopGauge) Sub(delta float64)   {}

type noopHistogramVec struct{}

func (v *noopHistogramVec) WithLabelValues(lvs ...string) Histogram { return &noopHistogram{} }
func (v *noopHistogramVec) With(labels map[string]string) Histogram { return &noopHistogram{} }

type noopHistogram struct{}

func (h *noopHistogram) Observe(value float64) {}

type noopSummaryVec struct{}

func (v *noopSummaryVec) WithLabelValues(lvs ...string) Summary { return &noopSummary{} }
func (v *noopSummaryVec) With(labels map[string]string) Summary { return &noopSummary{} }

type noopSummary struct{}

func (s *noopSummary) Observe(value float64) {}

// Timer

type Timer struct {
	histogram Histogram
	start     time.Time
}

func NewTimer(histogram Histogram) *Timer {
	return &Timer{
		histogram: histogram,
		start:     time.Now(),
	}
}

func (t *Timer) ObserveDuration() {
	if t.histogram == nil {
		return
	}
	t.histogram.Observe(time.Since(t.start).Seconds())
}
//Personal.AI order the ending
