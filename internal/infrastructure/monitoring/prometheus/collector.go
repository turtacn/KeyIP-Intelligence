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

// MetricsCollector is the interface for metrics collection.
type MetricsCollector interface {
	RegisterCounter(name, help string, labels ...string) CounterVec
	RegisterGauge(name, help string, labels ...string) GaugeVec
	RegisterHistogram(name, help string, buckets []float64, labels ...string) HistogramVec
	RegisterSummary(name, help string, objectives map[float64]float64, labels ...string) SummaryVec
	Handler() http.Handler
	MustRegister(collectors ...prometheus.Collector)
	Unregister(collector prometheus.Collector) bool
}

// CounterVec interface
type CounterVec interface {
	WithLabelValues(lvs ...string) Counter
	With(labels map[string]string) Counter
}

// Counter interface
type Counter interface {
	Inc()
	Add(delta float64)
}

// GaugeVec interface
type GaugeVec interface {
	WithLabelValues(lvs ...string) Gauge
	With(labels map[string]string) Gauge
}

// Gauge interface
type Gauge interface {
	Set(value float64)
	Inc()
	Dec()
	Add(delta float64)
	Sub(delta float64)
}

// HistogramVec interface
type HistogramVec interface {
	WithLabelValues(lvs ...string) Histogram
	With(labels map[string]string) Histogram
}

// Histogram interface
type Histogram interface {
	Observe(value float64)
}

// SummaryVec interface
type SummaryVec interface {
	WithLabelValues(lvs ...string) Summary
	With(labels map[string]string) Summary
}

// Summary interface
type Summary interface {
	Observe(value float64)
}

// CollectorConfig configuration for metrics collector.
type CollectorConfig struct {
	Namespace             string            `json:"namespace"`
	Subsystem             string            `json:"subsystem"`
	EnableProcessMetrics  bool              `json:"enable_process_metrics"`
	EnableGoMetrics       bool              `json:"enable_go_metrics"`
	DefaultHistogramBuckets []float64         `json:"default_histogram_buckets"`
	ConstLabels           map[string]string `json:"const_labels"`
}

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

func (c *prometheusCollector) RegisterCounter(name, help string, labels ...string) CounterVec {
	fqName := prometheus.BuildFQName(c.config.Namespace, c.config.Subsystem, name)

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.registeredMetrics[fqName]; ok {
		if cv, ok := existing.(*prometheus.CounterVec); ok {
			return &promCounterVec{cv}
		}
		c.logger.Error("Metric name conflict", logging.String("name", fqName))
		return &noopCounterVec{}
	}

	vec := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:        fqName,
		Help:        help,
		ConstLabels: c.config.ConstLabels,
	}, labels)

	if err := c.registry.Register(vec); err != nil {
		c.logger.Error("Failed to register counter", logging.String("name", fqName), logging.Err(err))
		return &noopCounterVec{}
	}

	c.registeredMetrics[fqName] = vec
	return &promCounterVec{vec}
}

func (c *prometheusCollector) RegisterGauge(name, help string, labels ...string) GaugeVec {
	fqName := prometheus.BuildFQName(c.config.Namespace, c.config.Subsystem, name)

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.registeredMetrics[fqName]; ok {
		if gv, ok := existing.(*prometheus.GaugeVec); ok {
			return &promGaugeVec{gv}
		}
		c.logger.Error("Metric name conflict", logging.String("name", fqName))
		return &noopGaugeVec{}
	}

	vec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        fqName,
		Help:        help,
		ConstLabels: c.config.ConstLabels,
	}, labels)

	if err := c.registry.Register(vec); err != nil {
		c.logger.Error("Failed to register gauge", logging.String("name", fqName), logging.Err(err))
		return &noopGaugeVec{}
	}

	c.registeredMetrics[fqName] = vec
	return &promGaugeVec{vec}
}

func (c *prometheusCollector) RegisterHistogram(name, help string, buckets []float64, labels ...string) HistogramVec {
	fqName := prometheus.BuildFQName(c.config.Namespace, c.config.Subsystem, name)

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.registeredMetrics[fqName]; ok {
		if hv, ok := existing.(*prometheus.HistogramVec); ok {
			return &promHistogramVec{hv}
		}
		c.logger.Error("Metric name conflict", logging.String("name", fqName))
		return &noopHistogramVec{}
	}

	if buckets == nil {
		buckets = c.config.DefaultHistogramBuckets
	}

	vec := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        fqName,
		Help:        help,
		Buckets:     buckets,
		ConstLabels: c.config.ConstLabels,
	}, labels)

	if err := c.registry.Register(vec); err != nil {
		c.logger.Error("Failed to register histogram", logging.String("name", fqName), logging.Err(err))
		return &noopHistogramVec{}
	}

	c.registeredMetrics[fqName] = vec
	return &promHistogramVec{vec}
}

func (c *prometheusCollector) RegisterSummary(name, help string, objectives map[float64]float64, labels ...string) SummaryVec {
	fqName := prometheus.BuildFQName(c.config.Namespace, c.config.Subsystem, name)

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.registeredMetrics[fqName]; ok {
		if sv, ok := existing.(*prometheus.SummaryVec); ok {
			return &promSummaryVec{sv}
		}
		c.logger.Error("Metric name conflict", logging.String("name", fqName))
		return &noopSummaryVec{}
	}

	if objectives == nil {
		objectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}
	}

	vec := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:        fqName,
		Help:        help,
		Objectives:  objectives,
		ConstLabels: c.config.ConstLabels,
	}, labels)

	if err := c.registry.Register(vec); err != nil {
		c.logger.Error("Failed to register summary", logging.String("name", fqName), logging.Err(err))
		return &noopSummaryVec{}
	}

	c.registeredMetrics[fqName] = vec
	return &promSummaryVec{vec}
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

// Wrappers

type promCounterVec struct {
	vec *prometheus.CounterVec
}

func (v *promCounterVec) WithLabelValues(lvs ...string) Counter {
	return &promCounter{v.vec.WithLabelValues(lvs...)}
}

func (v *promCounterVec) With(labels map[string]string) Counter {
	return &promCounter{v.vec.With(prometheus.Labels(labels))}
}

type promCounter struct {
	c prometheus.Counter
}

func (c *promCounter) Inc() {
	c.c.Inc()
}

func (c *promCounter) Add(delta float64) {
	c.c.Add(delta)
}

type promGaugeVec struct {
	vec *prometheus.GaugeVec
}

func (v *promGaugeVec) WithLabelValues(lvs ...string) Gauge {
	return &promGauge{v.vec.WithLabelValues(lvs...)}
}

func (v *promGaugeVec) With(labels map[string]string) Gauge {
	return &promGauge{v.vec.With(prometheus.Labels(labels))}
}

type promGauge struct {
	g prometheus.Gauge
}

func (g *promGauge) Set(value float64) {
	g.g.Set(value)
}

func (g *promGauge) Inc() {
	g.g.Inc()
}

func (g *promGauge) Dec() {
	g.g.Dec()
}

func (g *promGauge) Add(delta float64) {
	g.g.Add(delta)
}

func (g *promGauge) Sub(delta float64) {
	g.g.Sub(delta)
}

type promHistogramVec struct {
	vec *prometheus.HistogramVec
}

func (v *promHistogramVec) WithLabelValues(lvs ...string) Histogram {
	return &promHistogram{v.vec.WithLabelValues(lvs...)}
}

func (v *promHistogramVec) With(labels map[string]string) Histogram {
	return &promHistogram{v.vec.With(prometheus.Labels(labels))}
}

type promHistogram struct {
	h prometheus.Observer
}

func (h *promHistogram) Observe(value float64) {
	h.h.Observe(value)
}

type promSummaryVec struct {
	vec *prometheus.SummaryVec
}

func (v *promSummaryVec) WithLabelValues(lvs ...string) Summary {
	return &promSummary{v.vec.WithLabelValues(lvs...)}
}

func (v *promSummaryVec) With(labels map[string]string) Summary {
	return &promSummary{v.vec.With(prometheus.Labels(labels))}
}

type promSummary struct {
	s prometheus.Observer
}

func (s *promSummary) Observe(value float64) {
	s.s.Observe(value)
}

// No-ops

type noopCounterVec struct{}
type noopCounter struct{}

func (v *noopCounterVec) WithLabelValues(lvs ...string) Counter { return &noopCounter{} }
func (v *noopCounterVec) With(labels map[string]string) Counter { return &noopCounter{} }
func (c *noopCounter) Inc()                                     {}
func (c *noopCounter) Add(delta float64)                        {}

type noopGaugeVec struct{}
type noopGauge struct{}

func (v *noopGaugeVec) WithLabelValues(lvs ...string) Gauge { return &noopGauge{} }
func (v *noopGaugeVec) With(labels map[string]string) Gauge { return &noopGauge{} }
func (g *noopGauge) Set(value float64)                      {}
func (g *noopGauge) Inc()                                   {}
func (g *noopGauge) Dec()                                   {}
func (g *noopGauge) Add(delta float64)                      {}
func (g *noopGauge) Sub(delta float64)                      {}

type noopHistogramVec struct{}
type noopHistogram struct{}

func (v *noopHistogramVec) WithLabelValues(lvs ...string) Histogram { return &noopHistogram{} }
func (v *noopHistogramVec) With(labels map[string]string) Histogram { return &noopHistogram{} }
func (h *noopHistogram) Observe(value float64)                      {}

type noopSummaryVec struct{}
type noopSummary struct{}

func (v *noopSummaryVec) WithLabelValues(lvs ...string) Summary { return &noopSummary{} }
func (v *noopSummaryVec) With(labels map[string]string) Summary { return &noopSummary{} }
func (s *noopSummary) Observe(value float64)                    {}

// Timer helper

type Timer struct {
	begin     time.Time
	histogram Histogram
}

func NewTimer(histogram Histogram) *Timer {
	return &Timer{
		begin:     time.Now(),
		histogram: histogram,
	}
}

func (t *Timer) ObserveDuration() {
	d := time.Since(t.begin).Seconds()
	t.histogram.Observe(d)
}

//Personal.AI order the ending
