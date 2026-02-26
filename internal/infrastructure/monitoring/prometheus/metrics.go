package prometheus

import (
	"fmt"
	"time"
)

// Default bucket definitions
var (
	DefaultHTTPDurationBuckets     = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	DefaultAnalysisDurationBuckets = []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600}
	DefaultLLMDurationBuckets      = []float64{.5, 1, 2, 5, 10, 30, 60, 120}
	DefaultSizeBuckets             = []float64{100, 1000, 10000, 100000, 1000000, 10000000}
	DefaultDBDurationBuckets       = []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 5}
	DefaultGRPCDurationBuckets     = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
)

// AppMetrics holds all application metrics.
type AppMetrics struct {
	// HTTP
	HTTPRequestsTotal    CounterVec
	HTTPRequestDuration  HistogramVec
	HTTPRequestSize      HistogramVec
	HTTPResponseSize     HistogramVec
	HTTPActiveRequests   GaugeVec

	// Auth
	AuthAttemptsTotal       CounterVec
	AuthTokenVerifyDuration HistogramVec
	AuthActiveTokens        GaugeVec

	// Patent
	PatentIngestTotal       CounterVec
	PatentIngestDuration    HistogramVec
	PatentStorageSize       GaugeVec
	PatentSearchDuration    HistogramVec
	PatentSearchResultCount HistogramVec
	PatentTotalCount        GaugeVec

	// Analysis
	AnalysisTasksTotal     CounterVec
	AnalysisTaskDuration   HistogramVec
	AnalysisTaskQueueDepth GaugeVec
	AnalysisActiveWorkers  GaugeVec
	AnalysisTaskRetries    CounterVec

	// Risk Assessment (Infringement)
	RiskAssessmentRequestsTotal  CounterVec
	RiskAssessmentCacheHitsTotal CounterVec
	RiskAssessmentDuration       HistogramVec
	FTOAnalysisDuration          HistogramVec

	// Graph
	GraphNodesTotal    GaugeVec
	GraphEdgesTotal    GaugeVec
	GraphQueryDuration HistogramVec
	GraphBuildDuration HistogramVec

	// LLM
	LLMRequestsTotal   CounterVec
	LLMRequestDuration HistogramVec
	LLMTokensUsed      CounterVec
	LLMCostTotal       CounterVec
	LLMCacheHitRate    GaugeVec

	// Infra
	DBConnectionPoolSize   GaugeVec
	DBConnectionPoolActive GaugeVec
	DBQueryDuration        HistogramVec
	CacheHitsTotal         CounterVec
	CacheMissesTotal       CounterVec
	MessageQueueDepth      GaugeVec
	MessageProcessDuration HistogramVec

	// Health
	ServiceUptime     GaugeVec
	HealthCheckStatus GaugeVec
	ErrorsTotal       CounterVec
}

// NewAppMetrics creates and registers all metrics.
func NewAppMetrics(collector MetricsCollector) *AppMetrics {
	m := &AppMetrics{}

	// HTTP
	m.HTTPRequestsTotal = collector.RegisterCounter("http_requests_total", "Total HTTP requests", "method", "path", "status_code")
	m.HTTPRequestDuration = collector.RegisterHistogram("http_request_duration_seconds", "HTTP request duration", DefaultHTTPDurationBuckets, "method", "path")
	m.HTTPRequestSize = collector.RegisterHistogram("http_request_size_bytes", "HTTP request size", DefaultSizeBuckets, "method", "path")
	m.HTTPResponseSize = collector.RegisterHistogram("http_response_size_bytes", "HTTP response size", DefaultSizeBuckets, "method", "path")
	m.HTTPActiveRequests = collector.RegisterGauge("http_active_requests", "Active HTTP requests", "method", "path")

	// Auth
	m.AuthAttemptsTotal = collector.RegisterCounter("auth_attempts_total", "Total authentication attempts", "result", "failure_reason")
	m.AuthTokenVerifyDuration = collector.RegisterHistogram("auth_token_verify_duration_seconds", "Token verification duration", DefaultDBDurationBuckets, "method")
	m.AuthActiveTokens = collector.RegisterGauge("auth_active_tokens", "Active tokens (introspection)", "token_type")

	// Patent
	m.PatentIngestTotal = collector.RegisterCounter("patent_ingest_total", "Total patents ingested", "source", "status")
	m.PatentIngestDuration = collector.RegisterHistogram("patent_ingest_duration_seconds", "Patent ingestion duration", DefaultAnalysisDurationBuckets, "source")
	m.PatentStorageSize = collector.RegisterGauge("patent_storage_bytes", "Patent storage size", "storage_type")
	m.PatentSearchDuration = collector.RegisterHistogram("patent_search_duration_seconds", "Patent search duration", DefaultHTTPDurationBuckets, "query_type")
	m.PatentSearchResultCount = collector.RegisterHistogram("patent_search_results", "Patent search results count", []float64{0, 10, 50, 100, 500, 1000}, "query_type")
	m.PatentTotalCount = collector.RegisterGauge("patent_total_count", "Total patents", "status")

	// Analysis
	m.AnalysisTasksTotal = collector.RegisterCounter("analysis_tasks_total", "Total analysis tasks", "type", "status")
	m.AnalysisTaskDuration = collector.RegisterHistogram("analysis_task_duration_seconds", "Analysis task duration", DefaultAnalysisDurationBuckets, "type")
	m.AnalysisTaskQueueDepth = collector.RegisterGauge("analysis_queue_depth", "Analysis task queue depth", "priority")
	m.AnalysisActiveWorkers = collector.RegisterGauge("analysis_active_workers", "Active analysis workers", "type")
	m.AnalysisTaskRetries = collector.RegisterCounter("analysis_task_retries_total", "Analysis task retries", "type", "reason")

	// Risk Assessment
	m.RiskAssessmentRequestsTotal = collector.RegisterCounter("risk_assessment_requests_total", "Total risk assessment requests", "method", "depth")
	m.RiskAssessmentCacheHitsTotal = collector.RegisterCounter("risk_assessment_cache_hits_total", "Risk assessment cache hits")
	m.RiskAssessmentDuration = collector.RegisterHistogram("risk_assessment_duration_seconds", "Risk assessment duration", DefaultAnalysisDurationBuckets, "depth")
	m.FTOAnalysisDuration = collector.RegisterHistogram("fto_analysis_duration_seconds", "FTO analysis duration", DefaultAnalysisDurationBuckets, "jurisdiction_count")

	// Graph
	m.GraphNodesTotal = collector.RegisterGauge("graph_nodes_total", "Total graph nodes", "node_type")
	m.GraphEdgesTotal = collector.RegisterGauge("graph_edges_total", "Total graph edges", "edge_type")
	m.GraphQueryDuration = collector.RegisterHistogram("graph_query_duration_seconds", "Graph query duration", DefaultDBDurationBuckets, "query_type")
	m.GraphBuildDuration = collector.RegisterHistogram("graph_build_duration_seconds", "Graph build duration", DefaultAnalysisDurationBuckets, "operation")

	// LLM
	m.LLMRequestsTotal = collector.RegisterCounter("llm_requests_total", "Total LLM requests", "model", "operation", "status")
	m.LLMRequestDuration = collector.RegisterHistogram("llm_request_duration_seconds", "LLM request duration", DefaultLLMDurationBuckets, "model", "operation")
	m.LLMTokensUsed = collector.RegisterCounter("llm_tokens_total", "Total LLM tokens used", "model", "direction")
	m.LLMCostTotal = collector.RegisterCounter("llm_cost_total", "Total LLM cost", "model")
	m.LLMCacheHitRate = collector.RegisterGauge("llm_cache_hit_rate", "LLM cache hit rate", "model")

	// Infra
	m.DBConnectionPoolSize = collector.RegisterGauge("db_pool_size", "DB connection pool size", "db")
	m.DBConnectionPoolActive = collector.RegisterGauge("db_pool_active", "Active DB connections", "db")
	m.DBQueryDuration = collector.RegisterHistogram("db_query_duration_seconds", "DB query duration", DefaultDBDurationBuckets, "db", "operation")
	m.CacheHitsTotal = collector.RegisterCounter("cache_hits_total", "Cache hits", "cache")
	m.CacheMissesTotal = collector.RegisterCounter("cache_misses_total", "Cache misses", "cache")
	m.MessageQueueDepth = collector.RegisterGauge("mq_depth", "Message queue depth", "queue")
	m.MessageProcessDuration = collector.RegisterHistogram("mq_process_duration_seconds", "Message processing duration", DefaultDBDurationBuckets, "queue", "message_type")

	// Health
	m.ServiceUptime = collector.RegisterGauge("uptime_seconds", "Service uptime", "service")
	m.HealthCheckStatus = collector.RegisterGauge("health_status", "Health check status (1=up, 0=down)", "component")
	m.ErrorsTotal = collector.RegisterCounter("errors_total", "Total errors", "component", "error_type", "severity")

	// Start uptime counter
	m.ServiceUptime.WithLabelValues("keyip").Set(0)

	return m
}

// RegisterAppMetrics is an alias for NewAppMetrics.
func RegisterAppMetrics(collector MetricsCollector) *AppMetrics {
	return NewAppMetrics(collector)
}

// GRPCMetrics holds metrics for gRPC servers.
type GRPCMetrics struct {
	UnaryRequestsTotal   CounterVec
	UnaryRequestDuration HistogramVec
	StreamRequestsTotal  CounterVec
	StreamRequestDuration HistogramVec
}

// NewGRPCMetrics creates and registers gRPC metrics.
func NewGRPCMetrics(collector MetricsCollector) *GRPCMetrics {
	return &GRPCMetrics{
		UnaryRequestsTotal: collector.RegisterCounter(
			"grpc_unary_requests_total",
			"Total gRPC unary requests",
			"service", "method", "code",
		),
		UnaryRequestDuration: collector.RegisterHistogram(
			"grpc_unary_request_duration_seconds",
			"gRPC unary request duration",
			DefaultGRPCDurationBuckets,
			"service", "method",
		),
		StreamRequestsTotal: collector.RegisterCounter(
			"grpc_stream_requests_total",
			"Total gRPC stream requests",
			"service", "method", "code",
		),
		StreamRequestDuration: collector.RegisterHistogram(
			"grpc_stream_request_duration_seconds",
			"gRPC stream request duration",
			DefaultGRPCDurationBuckets,
			"service", "method",
		),
	}
}

func (m *GRPCMetrics) RecordUnaryRequest(service, method, code string, duration time.Duration) {
	if m == nil {
		return
	}
	m.UnaryRequestsTotal.WithLabelValues(service, method, code).Inc()
	m.UnaryRequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

func (m *GRPCMetrics) RecordStreamRequest(service, method, code string, duration time.Duration) {
	if m == nil {
		return
	}
	m.StreamRequestsTotal.WithLabelValues(service, method, code).Inc()
	m.StreamRequestDuration.WithLabelValues(service, method).Observe(duration.Seconds())
}

// Helpers

func RecordHTTPRequest(metrics *AppMetrics, method, path string, statusCode int, duration time.Duration, reqSize, respSize int64) {
	if metrics == nil {
		return
	}
	codeStr := IntToString(statusCode)
	metrics.HTTPRequestsTotal.WithLabelValues(method, path, codeStr).Inc()
	metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

	// Sizes
	// For histogram, we observe the size
	if reqSize > 0 {
		metrics.HTTPRequestSize.WithLabelValues(method, path).Observe(float64(reqSize))
	}
	if respSize > 0 {
		metrics.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(respSize))
	}
}

func RecordAuthAttempt(metrics *AppMetrics, success bool, failureReason string, duration time.Duration) {
	if metrics == nil {
		return
	}
	result := "success"
	if !success {
		result = "failure"
	}
	metrics.AuthAttemptsTotal.WithLabelValues(result, failureReason).Inc()

	// If this is considered a token verification or similar auth op
	metrics.AuthTokenVerifyDuration.WithLabelValues("verify").Observe(duration.Seconds())
}

func RecordLLMCall(metrics *AppMetrics, model, operation string, success bool, duration time.Duration, inputTokens, outputTokens int, cost float64) {
	if metrics == nil {
		return
	}
	status := "success"
	if !success {
		status = "failure"
	}
	metrics.LLMRequestsTotal.WithLabelValues(model, operation, status).Inc()
	metrics.LLMRequestDuration.WithLabelValues(model, operation).Observe(duration.Seconds())
	if success {
		metrics.LLMTokensUsed.WithLabelValues(model, "input").Add(float64(inputTokens))
		metrics.LLMTokensUsed.WithLabelValues(model, "output").Add(float64(outputTokens))
		metrics.LLMCostTotal.WithLabelValues(model).Add(cost)
	}
}

func RecordDBQuery(metrics *AppMetrics, db, operation string, duration time.Duration, err error) {
	if metrics == nil {
		return
	}
	metrics.DBQueryDuration.WithLabelValues(db, operation).Observe(duration.Seconds())
	if err != nil {
		RecordError(metrics, db, "query_error", "error")
	}
}

func RecordCacheAccess(metrics *AppMetrics, cache string, hit bool) {
	if metrics == nil {
		return
	}
	if hit {
		metrics.CacheHitsTotal.WithLabelValues(cache).Inc()
	} else {
		metrics.CacheMissesTotal.WithLabelValues(cache).Inc()
	}
}

func RecordError(metrics *AppMetrics, component, errorType, severity string) {
	if metrics == nil {
		return
	}
	metrics.ErrorsTotal.WithLabelValues(component, errorType, severity).Inc()
}

// IntToString converts int to string efficiently
func IntToString(i int) string {
	return fmt.Sprintf("%d", i)
}
//Personal.AI order the ending
