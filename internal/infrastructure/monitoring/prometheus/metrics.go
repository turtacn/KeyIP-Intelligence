package prometheus

import (
	"fmt"
	"time"
)

// AppMetrics holds all application metrics.
type AppMetrics struct {
	// HTTP Layer
	HTTPRequestsTotal     CounterVec
	HTTPRequestDuration   HistogramVec
	HTTPRequestSize       HistogramVec
	HTTPResponseSize      HistogramVec
	HTTPActiveRequests    GaugeVec

	// Auth Layer
	AuthAttemptsTotal     CounterVec
	AuthTokenVerifyDuration HistogramVec
	AuthActiveTokens      GaugeVec

	// Patent Layer
	PatentIngestTotal     CounterVec
	PatentIngestDuration  HistogramVec
	PatentStorageSize     GaugeVec
	PatentSearchDuration  HistogramVec
	PatentSearchResultCount HistogramVec
	PatentTotalCount      GaugeVec

	// Infringement/Risk Layer
	RiskAssessmentRequestsTotal  CounterVec
	RiskAssessmentDuration       HistogramVec
	RiskAssessmentCacheHitsTotal CounterVec
	FTOAnalysisDuration          HistogramVec

	// Analysis Layer
	AnalysisTasksTotal    CounterVec
	AnalysisTaskDuration  HistogramVec
	AnalysisTaskQueueDepth GaugeVec
	AnalysisActiveWorkers GaugeVec
	AnalysisTaskRetries   CounterVec

	// Graph Layer
	GraphNodesTotal       GaugeVec
	GraphEdgesTotal       GaugeVec
	GraphQueryDuration    HistogramVec
	GraphBuildDuration    HistogramVec

	// AI/LLM Layer
	LLMRequestsTotal      CounterVec
	LLMRequestDuration    HistogramVec
	LLMTokensUsed         CounterVec
	LLMCostTotal          CounterVec
	LLMCacheHitRate       GaugeVec

	// Infrastructure Layer
	DBConnectionPoolSize  GaugeVec
	DBConnectionPoolActive GaugeVec
	DBQueryDuration       HistogramVec
	CacheHitsTotal        CounterVec
	CacheMissesTotal      CounterVec
	MessageQueueDepth     GaugeVec
	MessageProcessDuration HistogramVec

	// System Health
	ServiceUptime         GaugeVec
	HealthCheckStatus     GaugeVec
	ErrorsTotal           CounterVec
}

// Default Buckets
var (
	DefaultHTTPDurationBuckets     = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	DefaultAnalysisDurationBuckets = []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600}
	DefaultLLMDurationBuckets      = []float64{.5, 1, 2, 5, 10, 30, 60, 120}
	DefaultSizeBuckets             = []float64{100, 1000, 10000, 100000, 1000000, 10000000}
	DefaultDBDurationBuckets       = []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 5}
)

// NewAppMetrics registers all metrics and returns AppMetrics struct.
func NewAppMetrics(collector MetricsCollector) *AppMetrics {
	m := &AppMetrics{}

	// HTTP
	m.HTTPRequestsTotal = collector.RegisterCounter("http_requests_total", "Total HTTP requests", "method", "path", "status_code")
	m.HTTPRequestDuration = collector.RegisterHistogram("http_request_duration_seconds", "HTTP request duration", DefaultHTTPDurationBuckets, "method", "path")
	m.HTTPRequestSize = collector.RegisterHistogram("http_request_size_bytes", "HTTP request size", DefaultSizeBuckets, "method", "path")
	m.HTTPResponseSize = collector.RegisterHistogram("http_response_size_bytes", "HTTP response size", DefaultSizeBuckets, "method", "path")
	m.HTTPActiveRequests = collector.RegisterGauge("http_active_requests", "Active HTTP requests", "method", "path")

	// Auth
	m.AuthAttemptsTotal = collector.RegisterCounter("auth_attempts_total", "Authentication attempts", "result", "failure_reason")
	m.AuthTokenVerifyDuration = collector.RegisterHistogram("auth_token_verify_duration_seconds", "Token verification duration", DefaultHTTPDurationBuckets, "method")
	m.AuthActiveTokens = collector.RegisterGauge("auth_active_tokens", "Active tokens (introspected)", "token_type")

	// Patent
	m.PatentIngestTotal = collector.RegisterCounter("patent_ingest_total", "Patent ingestion count", "source", "status")
	m.PatentIngestDuration = collector.RegisterHistogram("patent_ingest_duration_seconds", "Patent ingestion duration", DefaultAnalysisDurationBuckets, "source")
	m.PatentStorageSize = collector.RegisterGauge("patent_storage_bytes", "Patent storage size", "storage_type")
	m.PatentSearchDuration = collector.RegisterHistogram("patent_search_duration_seconds", "Patent search duration", DefaultHTTPDurationBuckets, "query_type")
	m.PatentSearchResultCount = collector.RegisterHistogram("patent_search_result_count", "Patent search result count", []float64{0, 10, 50, 100, 500, 1000, 5000, 10000}, "query_type")
	m.PatentTotalCount = collector.RegisterGauge("patent_total_count", "Total patents", "status")

	// Infringement/Risk
	m.RiskAssessmentRequestsTotal = collector.RegisterCounter("risk_assessment_requests_total", "Risk assessment requests", "method", "depth")
	m.RiskAssessmentDuration = collector.RegisterHistogram("risk_assessment_duration_seconds", "Risk assessment duration", DefaultAnalysisDurationBuckets, "depth")
	m.RiskAssessmentCacheHitsTotal = collector.RegisterCounter("risk_assessment_cache_hits_total", "Risk assessment cache hits")
	m.FTOAnalysisDuration = collector.RegisterHistogram("fto_analysis_duration_seconds", "FTO analysis duration", DefaultAnalysisDurationBuckets, "jurisdictions")

	// Analysis
	m.AnalysisTasksTotal = collector.RegisterCounter("analysis_tasks_total", "Analysis tasks total", "type", "status")
	m.AnalysisTaskDuration = collector.RegisterHistogram("analysis_task_duration_seconds", "Analysis task duration", DefaultAnalysisDurationBuckets, "type")
	m.AnalysisTaskQueueDepth = collector.RegisterGauge("analysis_task_queue_depth", "Analysis task queue depth", "priority")
	m.AnalysisActiveWorkers = collector.RegisterGauge("analysis_active_workers", "Active analysis workers", "type")
	m.AnalysisTaskRetries = collector.RegisterCounter("analysis_task_retries_total", "Analysis task retries", "type", "reason")

	// Graph
	m.GraphNodesTotal = collector.RegisterGauge("graph_nodes_total", "Graph nodes total", "node_type")
	m.GraphEdgesTotal = collector.RegisterGauge("graph_edges_total", "Graph edges total", "edge_type")
	m.GraphQueryDuration = collector.RegisterHistogram("graph_query_duration_seconds", "Graph query duration", DefaultDBDurationBuckets, "query_type")
	m.GraphBuildDuration = collector.RegisterHistogram("graph_build_duration_seconds", "Graph build duration", DefaultAnalysisDurationBuckets, "operation")

	// AI/LLM
	m.LLMRequestsTotal = collector.RegisterCounter("llm_requests_total", "LLM requests total", "model", "operation", "status")
	m.LLMRequestDuration = collector.RegisterHistogram("llm_request_duration_seconds", "LLM request duration", DefaultLLMDurationBuckets, "model", "operation")
	m.LLMTokensUsed = collector.RegisterCounter("llm_tokens_total", "LLM tokens used", "model", "direction")
	m.LLMCostTotal = collector.RegisterCounter("llm_cost_total", "LLM cost total", "model")
	m.LLMCacheHitRate = collector.RegisterGauge("llm_cache_hit_rate", "LLM cache hit rate", "model")

	// Infrastructure
	m.DBConnectionPoolSize = collector.RegisterGauge("db_pool_size", "Database connection pool size", "db")
	m.DBConnectionPoolActive = collector.RegisterGauge("db_pool_active", "Database active connections", "db")
	m.DBQueryDuration = collector.RegisterHistogram("db_query_duration_seconds", "Database query duration", DefaultDBDurationBuckets, "db", "operation")
	m.CacheHitsTotal = collector.RegisterCounter("cache_hits_total", "Cache hits", "cache")
	m.CacheMissesTotal = collector.RegisterCounter("cache_misses_total", "Cache misses", "cache")
	m.MessageQueueDepth = collector.RegisterGauge("mq_depth", "Message queue depth", "queue")
	m.MessageProcessDuration = collector.RegisterHistogram("mq_process_duration_seconds", "Message processing duration", DefaultHTTPDurationBuckets, "queue", "message_type")

	// System Health
	m.ServiceUptime = collector.RegisterGauge("service_uptime_seconds", "Service uptime", "service")
	m.HealthCheckStatus = collector.RegisterGauge("health_check_status", "Health check status (1=up, 0=down)", "component")
	m.ErrorsTotal = collector.RegisterCounter("errors_total", "Total errors", "component", "error_type", "severity")

	return m
}

// RegisterAppMetrics is an alias for NewAppMetrics.
func RegisterAppMetrics(collector MetricsCollector) *AppMetrics {
	return NewAppMetrics(collector)
}

// Helpers

func RecordHTTPRequest(metrics *AppMetrics, method, path string, statusCode int, duration time.Duration, reqSize, respSize int64) {
	status := fmt.Sprintf("%d", statusCode)
	metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
	metrics.HTTPRequestSize.WithLabelValues(method, path).Observe(float64(reqSize))
	metrics.HTTPResponseSize.WithLabelValues(method, path).Observe(float64(respSize))
}

func RecordAuthAttempt(metrics *AppMetrics, success bool, failureReason string, duration time.Duration) {
	result := "success"
	if !success {
		result = "failure"
	}
	metrics.AuthAttemptsTotal.WithLabelValues(result, failureReason).Inc()
	metrics.AuthTokenVerifyDuration.WithLabelValues("local").Observe(duration.Seconds()) // Assuming local verify
}

func RecordLLMCall(metrics *AppMetrics, model, operation string, success bool, duration time.Duration, inputTokens, outputTokens int, cost float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	metrics.LLMRequestsTotal.WithLabelValues(model, operation, status).Inc()
	metrics.LLMRequestDuration.WithLabelValues(model, operation).Observe(duration.Seconds())
	metrics.LLMTokensUsed.WithLabelValues(model, "input").Add(float64(inputTokens))
	metrics.LLMTokensUsed.WithLabelValues(model, "output").Add(float64(outputTokens))
	metrics.LLMCostTotal.WithLabelValues(model).Add(cost)
}

func RecordDBQuery(metrics *AppMetrics, db, operation string, duration time.Duration, err error) {
	metrics.DBQueryDuration.WithLabelValues(db, operation).Observe(duration.Seconds())
	if err != nil {
		metrics.ErrorsTotal.WithLabelValues(db, "query_error", "error").Inc()
	}
}

func RecordCacheAccess(metrics *AppMetrics, cache string, hit bool) {
	if hit {
		metrics.CacheHitsTotal.WithLabelValues(cache).Inc()
	} else {
		metrics.CacheMissesTotal.WithLabelValues(cache).Inc()
	}
}

func RecordError(metrics *AppMetrics, component, errorType, severity string) {
	metrics.ErrorsTotal.WithLabelValues(component, errorType, severity).Inc()
}

//Personal.AI order the ending
