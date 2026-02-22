package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =========================================================================
// Helpers
// =========================================================================

func newTestPredictRequest(modelName string) *PredictRequest {
	return &PredictRequest{
		ModelName:   modelName,
		InputData:   []byte(`{"input":[1,2,3]}`),
		InputFormat: FormatJSON,
		Metadata:    map[string]string{"request_id": "test-001"},
	}
}

func newTestPredictResponse(modelName string) *PredictResponse {
	return &PredictResponse{
		ModelName:       modelName,
		ModelVersion:    "1",
		Outputs:         map[string][]byte{"embedding": EncodeFloat32Vector([]float32{0.1, 0.2, 0.3})},
		OutputFormat:    FormatJSON,
		InferenceTimeMs: 5,
	}
}

func newTestNodes(n int) []*ServingNode {
	nodes := make([]*ServingNode, n)
	for i := 0; i < n; i++ {
		nodes[i] = &ServingNode{
			Address: fmt.Sprintf("node-%d:8500", i),
			Healthy: true,
			Weight:  1.0,
		}
	}
	return nodes
}

func countSelections(balancer LoadBalancer, nodes []*ServingNode, n int) map[string]int {
	counts := make(map[string]int)
	ctx := context.Background()
	for i := 0; i < n; i++ {
		node, err := balancer.Select(ctx, nodes)
		if err != nil {
			continue
		}
		counts[node.Address]++
	}
	return counts
}

// startTestHTTPServer creates a test HTTP server with the given handler.
func startTestHTTPServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

// predictHandler returns an http.HandlerFunc that responds to predict requests.
func predictHandler(resp *PredictResponse, statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if resp != nil {
			json.NewEncoder(w).Encode(resp)
		}
	}
}

// multiRouteHandler dispatches based on URL path patterns.
func multiRouteHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case r.Method == http.MethodGet && path == "/v1/health":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))

		case r.Method == http.MethodGet && strings.HasPrefix(path, "/v1/models/") && !strings.Contains(path, ":"):
			parts := strings.Split(strings.TrimPrefix(path, "/v1/models/"), "/")
			modelName := parts[0]
			if modelName == "nonexistent" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			status := ServingModelStatus{
				ModelName:      modelName,
				DefaultVersion: "1",
				Versions: []*ServingVersionStatus{
					{Version: "1", Status: VersionReady, InferenceCount: 100, AvgLatencyMs: 5.0},
				},
			}
			json.NewEncoder(w).Encode(status)

		case r.Method == http.MethodGet && path == "/v1/models":
			list := struct {
				Models []*ServingModelStatus `json:"models"`
			}{
				Models: []*ServingModelStatus{
					{ModelName: "model-a", DefaultVersion: "1"},
					{ModelName: "model-b", DefaultVersion: "2"},
				},
			}
			json.NewEncoder(w).Encode(list)

		case r.Method == http.MethodPost && strings.Contains(path, ":predict"):
			body, _ := io.ReadAll(r.Body)
			var req PredictRequest
			json.Unmarshal(body, &req)
			resp := PredictResponse{
				ModelName:       req.ModelName,
				ModelVersion:    "1",
				Outputs:         map[string][]byte{"default": req.InputData},
				OutputFormat:    FormatJSON,
				InferenceTimeMs: 3,
			}
			json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && strings.Contains(path, ":batchPredict"):
			body, _ := io.ReadAll(r.Body)
			var batch struct {
				Requests []*PredictRequest `json:"requests"`
			}
			json.Unmarshal(body, &batch)
			responses := make([]*PredictResponse, len(batch.Requests))
			for i, req := range batch.Requests {
				responses[i] = &PredictResponse{
					ModelName:       req.ModelName,
					ModelVersion:    "1",
					Outputs:         map[string][]byte{"default": req.InputData},
					OutputFormat:    FormatJSON,
					InferenceTimeMs: 2,
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"responses": responses})

		case r.Method == http.MethodPost && strings.Contains(path, ":streamPredict"):
			body, _ := io.ReadAll(r.Body)
			var req PredictRequest
			json.Unmarshal(body, &req)
			flusher, ok := w.(http.Flusher)
			resp := PredictResponse{
				ModelName:       req.ModelName,
				ModelVersion:    "1",
				Outputs:         map[string][]byte{"chunk": []byte("data")},
				OutputFormat:    FormatJSON,
				InferenceTimeMs: 1,
			}
			json.NewEncoder(w).Encode(resp)
			if ok {
				flusher.Flush()
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// =========================================================================
// Mock interceptor
// =========================================================================

type mockInterceptor struct {
	id       string
	mu       sync.Mutex
	before   []string
	after    []string
	beforeFn func(ctx context.Context, req *PredictRequest) (context.Context, *PredictRequest, error)
	afterFn  func(ctx context.Context, resp *PredictResponse, err error) (*PredictResponse, error)
}

func newMockInterceptor(id string) *mockInterceptor {
	return &mockInterceptor{id: id}
}

func (m *mockInterceptor) BeforeRequest(ctx context.Context, req *PredictRequest) (context.Context, *PredictRequest, error) {
	m.mu.Lock()
	m.before = append(m.before, m.id)
	m.mu.Unlock()
	if m.beforeFn != nil {
		return m.beforeFn(ctx, req)
	}
	return ctx, req, nil
}

func (m *mockInterceptor) AfterResponse(ctx context.Context, resp *PredictResponse, err error) (*PredictResponse, error) {
	m.mu.Lock()
	m.after = append(m.after, m.id)
	m.mu.Unlock()
	if m.afterFn != nil {
		return m.afterFn(ctx, resp, err)
	}
	return resp, err
}

// =========================================================================
// Mock metrics / logger for serving tests
// =========================================================================

type servingMockMetrics struct {
	inferenceCount atomic.Int32
}

func (m *servingMockMetrics) RecordInference(_ context.Context, _ *InferenceMetricParams)       { m.inferenceCount.Add(1) }
func (m *servingMockMetrics) RecordBatchProcessing(_ context.Context, _ *BatchMetricParams)      {}
func (m *servingMockMetrics) RecordCacheAccess(_ context.Context, _ bool, _ string)              {}
func (m *servingMockMetrics) RecordCircuitBreakerStateChange(_ context.Context, _, _, _ string)   {}
func (m *servingMockMetrics) RecordRiskAssessment(_ context.Context, _ string, _ float64)         {}
func (m *servingMockMetrics) RecordModelLoad(_ context.Context, _, _ string, _ float64, _ bool)   {}
func (m *servingMockMetrics) GetInferenceLatencyHistogram() LatencyHistogram                      { return nil }
func (m *servingMockMetrics) GetCurrentStats() *IntelligenceStats                                 { return &IntelligenceStats{} }

type servingMockLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *servingMockLogger) Info(msg string, kv ...interface{})  { l.log("INFO", msg, kv...) }
func (l *servingMockLogger) Warn(msg string, kv ...interface{})  { l.log("WARN", msg, kv...) }
func (l *servingMockLogger) Error(msg string, kv ...interface{}) { l.log("ERROR", msg, kv...) }
func (l *servingMockLogger) Debug(msg string, kv ...interface{}) { l.log("DEBUG", msg, kv...) }
func (l *servingMockLogger) log(level, msg string, kv ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, fmt.Sprintf("[%s] %s %v", level, msg, kv))
}
func (l *servingMockLogger) Entries() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	cp := make([]string, len(l.entries))
	copy(cp, l.entries)
	return cp
}

// =========================================================================
// gRPC Client Tests
// =========================================================================

func TestNewGRPCServingClient_Success(t *testing.T) {
	c, err := NewGRPCServingClient([]string{"localhost:8500"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Close()
	if len(c.addresses) != 1 {
		t.Errorf("expected 1 address, got %d", len(c.addresses))
	}
}

func TestNewGRPCServingClient_EmptyAddresses(t *testing.T) {
	_, err := NewGRPCServingClient([]string{})
	if err == nil {
		t.Fatal("expected error for empty addresses")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewGRPCServingClient_MultipleAddresses(t *testing.T) {
	c, err := NewGRPCServingClient([]string{"node1:8500", "node2:8500", "node3:8500"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Close()
	if len(c.nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(c.nodes))
	}
}

func TestGRPC_Predict_Success(t *testing.T) {
	c, err := NewGRPCServingClient([]string{"localhost:8500"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Close()

	req := newTestPredictRequest("test-model")
	resp, err := c.Predict(context.Background(), req)
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if resp.ModelName != "test-model" {
		t.Errorf("expected model name test-model, got %s", resp.ModelName)
	}
}

func TestGRPC_Predict_Validation(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	_, err := c.Predict(context.Background(), &PredictRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGRPC_Predict_AllNodesUnhealthy(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	for _, n := range c.nodes {
		n.Healthy = false
	}
	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrAllNodesUnhealthy) {
		t.Errorf("expected ErrAllNodesUnhealthy, got %v", err)
	}
}

func TestGRPC_Predict_ClientClosed(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestGRPC_BatchPredict_Success(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	reqs := []*PredictRequest{
		newTestPredictRequest("model-a"),
		newTestPredictRequest("model-b"),
		newTestPredictRequest("model-c"),
	}
	results, err := c.BatchPredict(context.Background(), reqs)
	if err != nil {
		t.Fatalf("BatchPredict error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestGRPC_BatchPredict_ClientClosed(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	c.Close()

	_, err := c.BatchPredict(context.Background(), []*PredictRequest{newTestPredictRequest("m")})
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestGRPC_StreamPredict_Complete(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	ch, err := c.StreamPredict(context.Background(), newTestPredictRequest("model-a"))
	if err != nil {
		t.Fatalf("StreamPredict error: %v", err)
	}

	count := 0
	for resp := range ch {
		if resp == nil {
			t.Error("nil response in stream")
		}
		count++
	}
	if count == 0 {
		t.Error("expected at least one response from stream")
	}
}

func TestGRPC_StreamPredict_ContextCancel(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	ch, err := c.StreamPredict(ctx, newTestPredictRequest("model-a"))
	if err != nil {
		t.Fatalf("StreamPredict error: %v", err)
	}

	// Channel should close quickly
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case _, ok := <-ch:
		if ok {
			// Got a response before cancel took effect, that's acceptable
		}
	case <-timer.C:
		t.Error("stream did not close after context cancel")
	}
}

func TestGRPC_StreamPredict_Validation(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	_, err := c.StreamPredict(context.Background(), &PredictRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGRPC_StreamPredict_ClientClosed(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	c.Close()

	_, err := c.StreamPredict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestGRPC_GetModelStatus_Success(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	status, err := c.GetModelStatus(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("GetModelStatus error: %v", err)
	}
	if status.ModelName != "test-model" {
		t.Errorf("expected model name test-model, got %s", status.ModelName)
	}
}

func TestGRPC_GetModelStatus_ClientClosed(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	c.Close()

	_, err := c.GetModelStatus(context.Background(), "m")
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestGRPC_ListServingModels_Success(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	list, err := c.ListServingModels(context.Background())
	if err != nil {
		t.Fatalf("ListServingModels error: %v", err)
	}
	if list == nil {
		t.Error("expected non-nil list")
	}
}

func TestGRPC_Healthy_AllHealthy(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500", "localhost:8501"})
	defer c.Close()

	if err := c.Healthy(context.Background()); err != nil {
		t.Errorf("expected healthy, got %v", err)
	}
}

func TestGRPC_Healthy_AllUnhealthy(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	defer c.Close()

	for _, n := range c.nodes {
		n.Healthy = false
	}
	err := c.Healthy(context.Background())
	if !errors.Is(err, ErrAllNodesUnhealthy) {
		t.Errorf("expected ErrAllNodesUnhealthy, got %v", err)
	}
}

func TestGRPC_Healthy_ClientClosed(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	c.Close()

	err := c.Healthy(context.Background())
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestGRPC_Close_Idempotent(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	if err := c.Close(); err != nil {
		t.Fatalf("first Close error: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}
}

// =========================================================================
// HTTP Client Tests
// =========================================================================

func TestNewHTTPServingClient_Success(t *testing.T) {
	c, err := NewHTTPServingClient("http://localhost:8501")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Close()
	if c.baseURL != "http://localhost:8501" {
		t.Errorf("unexpected baseURL: %s", c.baseURL)
	}
}

func TestNewHTTPServingClient_InvalidURL(t *testing.T) {
	_, err := NewHTTPServingClient("not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestNewHTTPServingClient_EmptyURL(t *testing.T) {
	_, err := NewHTTPServingClient("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestHTTP_Predict_Success(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, err := NewHTTPServingClient(srv.URL)
	if err != nil {
		t.Fatalf("NewHTTPServingClient: %v", err)
	}
	defer c.Close()

	req := newTestPredictRequest("test-model")
	resp, err := c.Predict(context.Background(), req)
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if resp.ModelName != "test-model" {
		t.Errorf("expected model name test-model, got %s", resp.ModelName)
	}
	if resp.InferenceTimeMs <= 0 {
		t.Errorf("expected positive inference time, got %d", resp.InferenceTimeMs)
	}
}

func TestHTTP_Predict_ServerError(t *testing.T) {
	srv := startTestHTTPServer(t, predictHandler(nil, http.StatusServiceUnavailable))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrServingUnavailable) {
		t.Errorf("expected ErrServingUnavailable, got %v", err)
	}
}

func TestHTTP_Predict_NotFound(t *testing.T) {
	srv := startTestHTTPServer(t, predictHandler(nil, http.StatusNotFound))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrModelNotDeployed) {
		t.Errorf("expected ErrModelNotDeployed, got %v", err)
	}
}

func TestHTTP_Predict_BadRequest(t *testing.T) {
	srv := startTestHTTPServer(t, predictHandler(nil, http.StatusBadRequest))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestHTTP_Predict_Timeout(t *testing.T) {
	srv := startTestHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL, WithRequestTimeout(100*time.Millisecond))
	defer c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestHTTP_Predict_ConnectionRefused(t *testing.T) {
	c, _ := NewHTTPServingClient("http://127.0.0.1:1") // port 1 should be refused
	defer c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestHTTP_Predict_Validation(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	_, err := c.Predict(context.Background(), &PredictRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestHTTP_Predict_ClientClosed(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestHTTP_BatchPredict_Success(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	reqs := []*PredictRequest{
		newTestPredictRequest("model-a"),
		newTestPredictRequest("model-a"),
	}
	results, err := c.BatchPredict(context.Background(), reqs)
	if err != nil {
		t.Fatalf("BatchPredict error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestHTTP_BatchPredict_Empty(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	results, err := c.BatchPredict(context.Background(), []*PredictRequest{})
	if err != nil {
		t.Fatalf("BatchPredict error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestHTTP_GetModelStatus_Success(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	status, err := c.GetModelStatus(context.Background(), "test-model")
	if err != nil {
		t.Fatalf("GetModelStatus error: %v", err)
	}
	if status.ModelName != "test-model" {
		t.Errorf("expected model name test-model, got %s", status.ModelName)
	}
	if len(status.Versions) == 0 {
		t.Error("expected at least one version")
	}
}

func TestHTTP_GetModelStatus_NotFound(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	_, err := c.GetModelStatus(context.Background(), "nonexistent")
	if !errors.Is(err, ErrModelNotDeployed) {
		t.Errorf("expected ErrModelNotDeployed, got %v", err)
	}
}

func TestHTTP_ListServingModels_Success(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	list, err := c.ListServingModels(context.Background())
	if err != nil {
		t.Fatalf("ListServingModels error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 models, got %d", len(list))
	}
}

func TestHTTP_Healthy_Up(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	if err := c.Healthy(context.Background()); err != nil {
		t.Errorf("expected healthy, got %v", err)
	}
}

func TestHTTP_Healthy_Down(t *testing.T) {
	srv := startTestHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	err := c.Healthy(context.Background())
	if !errors.Is(err, ErrServingUnavailable) {
		t.Errorf("expected ErrServingUnavailable, got %v", err)
	}
}

func TestHTTP_Close_Idempotent(t *testing.T) {
	c, _ := NewHTTPServingClient("http://localhost:8501")
	if err := c.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// =========================================================================
// Mock Client Tests
// =========================================================================

func TestNewMockServingClient(t *testing.T) {
	m := NewMockServingClient()
	if m == nil {
		t.Fatal("expected non-nil mock client")
	}
}

func TestMockServingClient_ConfigurableResponse(t *testing.T) {
	m := NewMockServingClient()
	expected := newTestPredictResponse("mock-model")
	m.SetPredictResponse(expected)

	resp, err := m.Predict(context.Background(), newTestPredictRequest("mock-model"))
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if resp.ModelName != expected.ModelName {
		t.Errorf("expected model %s, got %s", expected.ModelName, resp.ModelName)
	}
	if resp.InferenceTimeMs != expected.InferenceTimeMs {
		t.Errorf("expected latency %d, got %d", expected.InferenceTimeMs, resp.InferenceTimeMs)
	}
}

func TestMockServingClient_ConfigurableError(t *testing.T) {
	m := NewMockServingClient()
	m.SetPredictError(ErrServingUnavailable)

	_, err := m.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrServingUnavailable) {
		t.Errorf("expected ErrServingUnavailable, got %v", err)
	}
}

func TestMockServingClient_CallHistory(t *testing.T) {
	m := NewMockServingClient()
	m.Predict(context.Background(), newTestPredictRequest("a"))
	m.Predict(context.Background(), newTestPredictRequest("b"))
	m.Healthy(context.Background())

	history := m.CallHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(history))
	}
	if history[0].Method != "Predict" {
		t.Errorf("call[0] expected Predict, got %s", history[0].Method)
	}
	if history[1].Method != "Predict" {
		t.Errorf("call[1] expected Predict, got %s", history[1].Method)
	}
	if history[2].Method != "Healthy" {
		t.Errorf("call[2] expected Healthy, got %s", history[2].Method)
	}
}

func TestMockServingClient_ConfigurableDelay(t *testing.T) {
	m := NewMockServingClient()
	m.SetDelay(200 * time.Millisecond)

	start := time.Now()
	_, err := m.Predict(context.Background(), newTestPredictRequest("m"))
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Errorf("expected at least 150ms delay, got %v", elapsed)
	}
}

func TestMockServingClient_ErrorSequence(t *testing.T) {
	m := NewMockServingClient()
	m.SetErrorSequence(map[int]error{
		2: fmt.Errorf("transient failure"),
	})

	// Call 1: success
	_, err := m.Predict(context.Background(), newTestPredictRequest("m"))
	if err != nil {
		t.Errorf("call 1 expected success, got %v", err)
	}

	// Call 2: error
	_, err = m.Predict(context.Background(), newTestPredictRequest("m"))
	if err == nil {
		t.Error("call 2 expected error")
	}

	// Call 3: success
	_, err = m.Predict(context.Background(), newTestPredictRequest("m"))
	if err != nil {
		t.Errorf("call 3 expected success, got %v", err)
	}
}

func TestMockServingClient_GetModelStatus_Configured(t *testing.T) {
	m := NewMockServingClient()
	expected := &ServingModelStatus{
		ModelName:      "my-model",
		DefaultVersion: "2",
	}
	m.SetModelStatus(expected)

	status, err := m.GetModelStatus(context.Background(), "my-model")
	if err != nil {
		t.Fatalf("GetModelStatus error: %v", err)
	}
	if status.ModelName != "my-model" {
		t.Errorf("expected my-model, got %s", status.ModelName)
	}
}

func TestMockServingClient_GetModelStatus_NotConfigured(t *testing.T) {
	m := NewMockServingClient()
	_, err := m.GetModelStatus(context.Background(), "m")
	if !errors.Is(err, ErrModelNotDeployed) {
		t.Errorf("expected ErrModelNotDeployed, got %v", err)
	}
}

func TestMockServingClient_ListServingModels(t *testing.T) {
	m := NewMockServingClient()
	list, err := m.ListServingModels(context.Background())
	if err != nil {
		t.Fatalf("ListServingModels error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestMockServingClient_Healthy_Default(t *testing.T) {
	m := NewMockServingClient()
	if err := m.Healthy(context.Background()); err != nil {
		t.Errorf("expected healthy by default, got %v", err)
	}
}

func TestMockServingClient_Healthy_Error(t *testing.T) {
	m := NewMockServingClient()
	m.SetHealthError(ErrServingUnavailable)
	err := m.Healthy(context.Background())
	if !errors.Is(err, ErrServingUnavailable) {
		t.Errorf("expected ErrServingUnavailable, got %v", err)
	}
}

func TestMockServingClient_StreamPredict(t *testing.T) {
	m := NewMockServingClient()
	ch, err := m.StreamPredict(context.Background(), newTestPredictRequest("m"))
	if err != nil {
		t.Fatalf("StreamPredict error: %v", err)
	}
	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("expected at least one response")
	}
}

func TestMockServingClient_BatchPredict(t *testing.T) {
	m := NewMockServingClient()
	reqs := []*PredictRequest{
		newTestPredictRequest("a"),
		newTestPredictRequest("b"),
	}
	results, err := m.BatchPredict(context.Background(), reqs)
	if err != nil {
		t.Fatalf("BatchPredict error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestMockServingClient_Close(t *testing.T) {
	m := NewMockServingClient()
	if err := m.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	history := m.CallHistory()
	found := false
	for _, h := range history {
		if h.Method == "Close" {
			found = true
		}
	}
	if !found {
		t.Error("Close not recorded in call history")
	}
}

// =========================================================================
// Round-Robin Balancer Tests
// =========================================================================

func TestRoundRobinBalancer_Cycling(t *testing.T) {
	b := NewRoundRobinBalancer()
	nodes := newTestNodes(3)
	ctx := context.Background()

	expected := []string{"node-0:8500", "node-1:8500", "node-2:8500", "node-0:8500", "node-1:8500", "node-2:8500"}
	for i, want := range expected {
		node, err := b.Select(ctx, nodes)
		if err != nil {
			t.Fatalf("Select[%d] error: %v", i, err)
		}
		if node.Address != want {
			t.Errorf("Select[%d] = %s, want %s", i, node.Address, want)
		}
	}
}

func TestRoundRobinBalancer_SkipUnhealthy(t *testing.T) {
	b := NewRoundRobinBalancer()
	nodes := newTestNodes(3)
	nodes[1].Healthy = false
	ctx := context.Background()

	expected := []string{"node-0:8500", "node-2:8500", "node-0:8500", "node-2:8500"}
	for i, want := range expected {
		node, err := b.Select(ctx, nodes)
		if err != nil {
			t.Fatalf("Select[%d] error: %v", i, err)
		}
		if node.Address != want {
			t.Errorf("Select[%d] = %s, want %s", i, node.Address, want)
		}
	}
}

func TestRoundRobinBalancer_AllUnhealthy(t *testing.T) {
	b := NewRoundRobinBalancer()
	nodes := newTestNodes(3)
	for _, n := range nodes {
		n.Healthy = false
	}

	_, err := b.Select(context.Background(), nodes)
	if !errors.Is(err, ErrAllNodesUnhealthy) {
		t.Errorf("expected ErrAllNodesUnhealthy, got %v", err)
	}
}

func TestRoundRobinBalancer_SingleNode(t *testing.T) {
	b := NewRoundRobinBalancer()
	nodes := newTestNodes(1)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		node, err := b.Select(ctx, nodes)
		if err != nil {
			t.Fatalf("Select[%d] error: %v", i, err)
		}
		if node.Address != "node-0:8500" {
			t.Errorf("Select[%d] = %s, want node-0:8500", i, node.Address)
		}
	}
}

func TestRoundRobinBalancer_ReportResult(t *testing.T) {
	b := NewRoundRobinBalancer()
	node := &ServingNode{Address: "n", Healthy: true, Weight: 1.0}

	b.ReportResult(node, true, 10.0)
	if node.AvgLatencyMs != 10.0 {
		t.Errorf("expected avg latency 10, got %f", node.AvgLatencyMs)
	}
	if node.ErrorRate != 0 {
		t.Errorf("expected error rate 0, got %f", node.ErrorRate)
	}

	b.ReportResult(node, false, 20.0)
	if node.ErrorRate == 0 {
		t.Error("expected non-zero error rate after failure")
	}
}

// =========================================================================
// Least-Connections Balancer Tests
// =========================================================================

func TestLeastConnectionsBalancer_SelectMinimum(t *testing.T) {
	b := NewLeastConnectionsBalancer()
	nodes := newTestNodes(3)
	atomic.StoreInt64(&nodes[0].ActiveRequests, 5)
	atomic.StoreInt64(&nodes[1].ActiveRequests, 2)
	atomic.StoreInt64(&nodes[2].ActiveRequests, 8)

	node, err := b.Select(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if node.Address != "node-1:8500" {
		t.Errorf("expected node-1:8500 (least connections), got %s", node.Address)
	}
}

func TestLeastConnectionsBalancer_TieBreaking(t *testing.T) {
	b := NewLeastConnectionsBalancer()
	nodes := newTestNodes(3)
	atomic.StoreInt64(&nodes[0].ActiveRequests, 2)
	atomic.StoreInt64(&nodes[1].ActiveRequests, 2)
	atomic.StoreInt64(&nodes[2].ActiveRequests, 5)

	node, err := b.Select(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	// Should pick first with minimum
	if node.Address != "node-0:8500" {
		t.Errorf("expected node-0:8500 (first tie), got %s", node.Address)
	}
}

func TestLeastConnectionsBalancer_SkipUnhealthy(t *testing.T) {
	b := NewLeastConnectionsBalancer()
	nodes := newTestNodes(3)
	atomic.StoreInt64(&nodes[0].ActiveRequests, 10)
	atomic.StoreInt64(&nodes[1].ActiveRequests, 1) // least but unhealthy
	atomic.StoreInt64(&nodes[2].ActiveRequests, 5)
	nodes[1].Healthy = false

	node, err := b.Select(context.Background(), nodes)
	if err != nil {
		t.Fatalf("Select error: %v", err)
	}
	if node.Address != "node-2:8500" {
		t.Errorf("expected node-2:8500, got %s", node.Address)
	}
}

func TestLeastConnectionsBalancer_AllUnhealthy(t *testing.T) {
	b := NewLeastConnectionsBalancer()
	nodes := newTestNodes(2)
	nodes[0].Healthy = false
	nodes[1].Healthy = false

	_, err := b.Select(context.Background(), nodes)
	if !errors.Is(err, ErrAllNodesUnhealthy) {
		t.Errorf("expected ErrAllNodesUnhealthy, got %v", err)
	}
}

func TestLeastConnectionsBalancer_ReportResult(t *testing.T) {
	b := NewLeastConnectionsBalancer()
	node := &ServingNode{Address: "n", Healthy: true, Weight: 1.0}

	b.ReportResult(node, true, 15.0)
	if node.AvgLatencyMs != 15.0 {
		t.Errorf("expected avg latency 15, got %f", node.AvgLatencyMs)
	}
}

// =========================================================================
// Weighted Balancer Tests
// =========================================================================

func TestWeightedBalancer_Distribution(t *testing.T) {
	b := NewWeightedBalancer()
	nodes := newTestNodes(3)
	nodes[0].Weight = 70
	nodes[1].Weight = 20
	nodes[2].Weight = 10

	counts := countSelections(b, nodes, 10000)

	total := 0
	for _, c := range counts {
		total += c
	}

	for _, tc := range []struct {
		addr     string
		expected float64
	}{
		{"node-0:8500", 0.70},
		{"node-1:8500", 0.20},
		{"node-2:8500", 0.10},
	} {
		actual := float64(counts[tc.addr]) / float64(total)
		if math.Abs(actual-tc.expected) > 0.05 {
			t.Errorf("node %s: expected ~%.0f%%, got %.1f%%", tc.addr, tc.expected*100, actual*100)
		}
	}
}

func TestWeightedBalancer_DynamicAdjustment(t *testing.T) {
	b := NewWeightedBalancer()
	nodes := newTestNodes(2)
	nodes[0].Weight = 50
	nodes[1].Weight = 50

	// Simulate high latency on node 0
	nodes[0].AvgLatencyMs = 5000 // 5 seconds
	nodes[1].AvgLatencyMs = 10   // 10ms

	counts := countSelections(b, nodes, 5000)

	// Node 1 should be selected much more often due to lower latency
	if counts["node-1:8500"] <= counts["node-0:8500"] {
		t.Errorf("expected node-1 to be selected more often: node-0=%d, node-1=%d",
			counts["node-0:8500"], counts["node-1:8500"])
	}
}

func TestWeightedBalancer_ErrorRateAdjustment(t *testing.T) {
	b := NewWeightedBalancer()
	nodes := newTestNodes(2)
	nodes[0].Weight = 50
	nodes[1].Weight = 50

	// Simulate high error rate on node 0
	nodes[0].ErrorRate = 0.9
	nodes[1].ErrorRate = 0.0

	counts := countSelections(b, nodes, 5000)

	// Node 1 should be selected much more often
	if counts["node-1:8500"] <= counts["node-0:8500"] {
		t.Errorf("expected node-1 to be selected more often: node-0=%d, node-1=%d",
			counts["node-0:8500"], counts["node-1:8500"])
	}
}

func TestWeightedBalancer_Recovery(t *testing.T) {
	b := NewWeightedBalancer()
	nodes := newTestNodes(2)
	nodes[0].Weight = 50
	nodes[1].Weight = 50

	// Initially high error rate
	nodes[0].ErrorRate = 0.9
	counts1 := countSelections(b, nodes, 1000)

	// Recover
	nodes[0].ErrorRate = 0.0
	counts2 := countSelections(b, nodes, 1000)

	// After recovery, node-0 should get more traffic than before
	ratio1 := float64(counts1["node-0:8500"]) / float64(counts1["node-0:8500"]+counts1["node-1:8500"])
	ratio2 := float64(counts2["node-0:8500"]) / float64(counts2["node-0:8500"]+counts2["node-1:8500"])

	if ratio2 <= ratio1 {
		t.Errorf("expected node-0 ratio to increase after recovery: before=%.2f, after=%.2f", ratio1, ratio2)
	}
}

func TestWeightedBalancer_AllUnhealthy(t *testing.T) {
	b := NewWeightedBalancer()
	nodes := newTestNodes(2)
	nodes[0].Healthy = false
	nodes[1].Healthy = false

	_, err := b.Select(context.Background(), nodes)
	if !errors.Is(err, ErrAllNodesUnhealthy) {
		t.Errorf("expected ErrAllNodesUnhealthy, got %v", err)
	}
}

func TestWeightedBalancer_SkipUnhealthy(t *testing.T) {
	b := NewWeightedBalancer()
	nodes := newTestNodes(2)
	nodes[0].Healthy = false

	for i := 0; i < 10; i++ {
		node, err := b.Select(context.Background(), nodes)
		if err != nil {
			t.Fatalf("Select error: %v", err)
		}
		if node.Address != "node-1:8500" {
			t.Errorf("expected node-1:8500, got %s", node.Address)
		}
	}
}

// =========================================================================
// NewLoadBalancer factory
// =========================================================================

func TestNewLoadBalancer_RoundRobin(t *testing.T) {
	b := NewLoadBalancer("round_robin")
	if _, ok := b.(*roundRobinBalancer); !ok {
		t.Error("expected roundRobinBalancer")
	}
}

func TestNewLoadBalancer_LeastConnections(t *testing.T) {
	b := NewLoadBalancer("least_connections")
	if _, ok := b.(*leastConnectionsBalancer); !ok {
		t.Error("expected leastConnectionsBalancer")
	}
}

func TestNewLoadBalancer_Weighted(t *testing.T) {
	b := NewLoadBalancer("weighted")
	if _, ok := b.(*weightedBalancer); !ok {
		t.Error("expected weightedBalancer")
	}
}

func TestNewLoadBalancer_Default(t *testing.T) {
	b := NewLoadBalancer("unknown")
	if _, ok := b.(*roundRobinBalancer); !ok {
		t.Error("expected roundRobinBalancer as default")
	}
}

// =========================================================================
// Interceptor Chain Tests
// =========================================================================

func TestInterceptorChain_Order(t *testing.T) {
	var orderBefore []string
	var orderAfter []string
	var mu sync.Mutex

	makeInterceptor := func(id string) RequestInterceptor {
		m := newMockInterceptor(id)
		m.beforeFn = func(ctx context.Context, req *PredictRequest) (context.Context, *PredictRequest, error) {
			mu.Lock()
			orderBefore = append(orderBefore, id)
			mu.Unlock()
			return ctx, req, nil
		}
		m.afterFn = func(ctx context.Context, resp *PredictResponse, err error) (*PredictResponse, error) {
			mu.Lock()
			orderAfter = append(orderAfter, id)
			mu.Unlock()
			return resp, err
		}
		return m
	}

	chain := []RequestInterceptor{
		makeInterceptor("A"),
		makeInterceptor("B"),
		makeInterceptor("C"),
	}

	req := newTestPredictRequest("m")
	ctx := context.Background()

	ctx, req, err := runBeforeInterceptors(ctx, req, chain)
	if err != nil {
		t.Fatalf("runBeforeInterceptors error: %v", err)
	}

	resp := newTestPredictResponse("m")
	_, err = runAfterInterceptors(ctx, resp, nil, chain)
	if err != nil {
		t.Fatalf("runAfterInterceptors error: %v", err)
	}

	// Before: A, B, C
	expectedBefore := []string{"A", "B", "C"}
	if len(orderBefore) != len(expectedBefore) {
		t.Fatalf("before order length: got %d, want %d", len(orderBefore), len(expectedBefore))
	}
	for i, v := range expectedBefore {
		if orderBefore[i] != v {
			t.Errorf("before[%d] = %s, want %s", i, orderBefore[i], v)
		}
	}

	// After: C, B, A (reverse)
	expectedAfter := []string{"C", "B", "A"}
	if len(orderAfter) != len(expectedAfter) {
		t.Fatalf("after order length: got %d, want %d", len(orderAfter), len(expectedAfter))
	}
	for i, v := range expectedAfter {
		if orderAfter[i] != v {
			t.Errorf("after[%d] = %s, want %s", i, orderAfter[i], v)
		}
	}
}

func TestInterceptorChain_BeforeRequestModification(t *testing.T) {
	ic := newMockInterceptor("modifier")
	ic.beforeFn = func(ctx context.Context, req *PredictRequest) (context.Context, *PredictRequest, error) {
		if req.Metadata == nil {
			req.Metadata = make(map[string]string)
		}
		req.Metadata["injected"] = "true"
		return ctx, req, nil
	}

	chain := []RequestInterceptor{ic}
	req := newTestPredictRequest("m")
	_, req, err := runBeforeInterceptors(context.Background(), req, chain)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if req.Metadata["injected"] != "true" {
		t.Error("expected metadata injection")
	}
}

func TestInterceptorChain_AfterResponseModification(t *testing.T) {
	ic := newMockInterceptor("modifier")
	ic.afterFn = func(ctx context.Context, resp *PredictResponse, err error) (*PredictResponse, error) {
		if resp != nil {
			if resp.Metadata == nil {
				resp.Metadata = make(map[string]string)
			}
			resp.Metadata["modified"] = "yes"
		}
		return resp, err
	}

	chain := []RequestInterceptor{ic}
	resp := newTestPredictResponse("m")
	resp, err := runAfterInterceptors(context.Background(), resp, nil, chain)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resp.Metadata["modified"] != "yes" {
		t.Error("expected response modification")
	}
}

func TestInterceptorChain_BeforeRequestError(t *testing.T) {
	errInterceptor := newMockInterceptor("err")
	errInterceptor.beforeFn = func(ctx context.Context, req *PredictRequest) (context.Context, *PredictRequest, error) {
		return ctx, req, fmt.Errorf("interceptor error")
	}

	neverCalled := newMockInterceptor("never")
	called := false
	neverCalled.beforeFn = func(ctx context.Context, req *PredictRequest) (context.Context, *PredictRequest, error) {
		called = true
		return ctx, req, nil
	}

	chain := []RequestInterceptor{errInterceptor, neverCalled}
	_, _, err := runBeforeInterceptors(context.Background(), newTestPredictRequest("m"), chain)
	if err == nil {
		t.Fatal("expected error from interceptor")
	}
	if called {
		t.Error("second interceptor should not have been called")
	}
}

// =========================================================================
// Tracing Interceptor Tests
// =========================================================================

func TestTracingInterceptor_InjectsTraceID(t *testing.T) {
	ic := NewTracingInterceptor()
	req := newTestPredictRequest("m")
	req.Metadata = nil

	_, req, err := ic.BeforeRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if req.Metadata["trace_id"] == "" {
		t.Error("expected trace_id in metadata")
	}
}

func TestTracingInterceptor_InjectsSpanID(t *testing.T) {
	ic := NewTracingInterceptor()
	req := newTestPredictRequest("m")
	req.Metadata = nil

	_, req, err := ic.BeforeRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if req.Metadata["span_id"] == "" {
		t.Error("expected span_id in metadata")
	}
}

func TestTracingInterceptor_PreservesExistingTraceID(t *testing.T) {
	ic := NewTracingInterceptor()
	req := newTestPredictRequest("m")
	req.Metadata["trace_id"] = "existing-trace"

	_, req, err := ic.BeforeRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if req.Metadata["trace_id"] != "existing-trace" {
		t.Errorf("expected existing-trace, got %s", req.Metadata["trace_id"])
	}
}

// =========================================================================
// Logging Interceptor Tests
// =========================================================================

func TestLoggingInterceptor_LogsRequest(t *testing.T) {
	logger := &servingMockLogger{}
	ic := NewLoggingInterceptor(logger, LoggingInterceptorConfig{})

	req := newTestPredictRequest("m")
	_, _, err := ic.BeforeRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	entries := logger.Entries()
	if len(entries) == 0 {
		t.Error("expected log entry for request")
	}
	found := false
	for _, e := range entries {
		if strings.Contains(e, "serving request") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'serving request' log entry")
	}
}

func TestLoggingInterceptor_LogsResponse(t *testing.T) {
	logger := &servingMockLogger{}
	ic := NewLoggingInterceptor(logger, LoggingInterceptorConfig{})

	resp := newTestPredictResponse("m")
	_, err := ic.AfterResponse(context.Background(), resp, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	entries := logger.Entries()
	found := false
	for _, e := range entries {
		if strings.Contains(e, "serving response") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'serving response' log entry")
	}
}

func TestLoggingInterceptor_LogsError(t *testing.T) {
	logger := &servingMockLogger{}
	ic := NewLoggingInterceptor(logger, LoggingInterceptorConfig{})

	_, _ = ic.AfterResponse(context.Background(), nil, fmt.Errorf("test error"))

	entries := logger.Entries()
	found := false
	for _, e := range entries {
		if strings.Contains(e, "ERROR") && strings.Contains(e, "error") {
			found = true
		}
	}
	if !found {
		t.Error("expected ERROR level log entry")
	}
}

// =========================================================================
// Auth Interceptor Tests
// =========================================================================

func TestAuthInterceptor_APIKey(t *testing.T) {
	ic := NewAuthInterceptor(AuthModeAPIKey, "my-api-key")
	req := newTestPredictRequest("m")

	_, req, err := ic.BeforeRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if req.Metadata["X-API-Key"] != "my-api-key" {
		t.Errorf("expected X-API-Key=my-api-key, got %s", req.Metadata["X-API-Key"])
	}
}

func TestAuthInterceptor_BearerToken(t *testing.T) {
	ic := NewAuthInterceptor(AuthModeBearer, "my-token")
	req := newTestPredictRequest("m")

	_, req, err := ic.BeforeRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	expected := "Bearer my-token"
	if req.Metadata["Authorization"] != expected {
		t.Errorf("expected Authorization=%s, got %s", expected, req.Metadata["Authorization"])
	}
}

// =========================================================================
// Metrics Interceptor Tests
// =========================================================================

func TestMetricsInterceptor_RecordsLatency(t *testing.T) {
	metrics := &servingMockMetrics{}
	ic := NewMetricsInterceptor(metrics)

	ctx := context.Background()
	req := newTestPredictRequest("m")
	ctx, req, _ = ic.BeforeRequest(ctx, req)

	time.Sleep(10 * time.Millisecond)

	resp := newTestPredictResponse("m")
	_, _ = ic.AfterResponse(ctx, resp, nil)

	if metrics.inferenceCount.Load() != 1 {
		t.Errorf("expected 1 inference recorded, got %d", metrics.inferenceCount.Load())
	}
}

func TestMetricsInterceptor_RecordsSuccess(t *testing.T) {
	metrics := &servingMockMetrics{}
	ic := NewMetricsInterceptor(metrics)

	ctx := context.Background()
	ctx, _, _ = ic.BeforeRequest(ctx, newTestPredictRequest("m"))
	_, _ = ic.AfterResponse(ctx, newTestPredictResponse("m"), nil)

	if metrics.inferenceCount.Load() != 1 {
		t.Errorf("expected 1 inference, got %d", metrics.inferenceCount.Load())
	}
}

func TestMetricsInterceptor_RecordsFailure(t *testing.T) {
	metrics := &servingMockMetrics{}
	ic := NewMetricsInterceptor(metrics)

	ctx := context.Background()
	ctx, _, _ = ic.BeforeRequest(ctx, newTestPredictRequest("m"))
	_, _ = ic.AfterResponse(ctx, nil, fmt.Errorf("fail"))

	if metrics.inferenceCount.Load() != 1 {
		t.Errorf("expected 1 inference, got %d", metrics.inferenceCount.Load())
	}
}

// =========================================================================
// ServingOption Tests
// =========================================================================

func TestServingOption_TLS(t *testing.T) {
	o := defaultServingOptions()
	WithTLS("cert.pem", "key.pem", "ca.pem")(o)
	if o.tlsCertFile != "cert.pem" {
		t.Errorf("expected cert.pem, got %s", o.tlsCertFile)
	}
	if o.tlsKeyFile != "key.pem" {
		t.Errorf("expected key.pem, got %s", o.tlsKeyFile)
	}
	if o.tlsCAFile != "ca.pem" {
		t.Errorf("expected ca.pem, got %s", o.tlsCAFile)
	}
}

func TestServingOption_ConnectionTimeout(t *testing.T) {
	o := defaultServingOptions()
	WithConnectionTimeout(10 * time.Second)(o)
	if o.connectionTimeout != 10*time.Second {
		t.Errorf("expected 10s, got %v", o.connectionTimeout)
	}
}

func TestServingOption_RequestTimeout(t *testing.T) {
	o := defaultServingOptions()
	WithRequestTimeout(60 * time.Second)(o)
	if o.requestTimeout != 60*time.Second {
		t.Errorf("expected 60s, got %v", o.requestTimeout)
	}
}

func TestServingOption_MaxRetries(t *testing.T) {
	o := defaultServingOptions()
	WithMaxRetries(5)(o)
	if o.maxRetries != 5 {
		t.Errorf("expected 5, got %d", o.maxRetries)
	}
}

func TestServingOption_LoadBalancer(t *testing.T) {
	o := defaultServingOptions()
	WithLoadBalancerStrategy("weighted")(o)
	if o.loadBalancerStrategy != "weighted" {
		t.Errorf("expected weighted, got %s", o.loadBalancerStrategy)
	}
}

func TestServingOption_ConnectionPoolSize(t *testing.T) {
	o := defaultServingOptions()
	WithConnectionPoolSize(20)(o)
	if o.connectionPoolSize != 20 {
		t.Errorf("expected 20, got %d", o.connectionPoolSize)
	}
}

func TestServingOption_HealthCheckInterval(t *testing.T) {
	o := defaultServingOptions()
	WithServingHealthCheckInterval(30 * time.Second)(o)
	if o.healthCheckInterval != 30*time.Second {
		t.Errorf("expected 30s, got %v", o.healthCheckInterval)
	}
}

func TestServingOption_Interceptors(t *testing.T) {
	o := defaultServingOptions()
	ic1 := newMockInterceptor("a")
	ic2 := newMockInterceptor("b")
	WithInterceptors(ic1, ic2)(o)
	if len(o.interceptors) != 2 {
		t.Errorf("expected 2 interceptors, got %d", len(o.interceptors))
	}
}

func TestServingOption_Metrics(t *testing.T) {
	o := defaultServingOptions()
	m := &servingMockMetrics{}
	WithServingMetrics(m)(o)
	if o.metrics == nil {
		t.Error("expected non-nil metrics")
	}
}

func TestServingOption_Logger(t *testing.T) {
	o := defaultServingOptions()
	l := &servingMockLogger{}
	WithServingLogger(l)(o)
	if o.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestServingOption_Defaults(t *testing.T) {
	o := defaultServingOptions()
	if o.connectionTimeout != 5*time.Second {
		t.Errorf("default connectionTimeout: got %v, want 5s", o.connectionTimeout)
	}
	if o.requestTimeout != 30*time.Second {
		t.Errorf("default requestTimeout: got %v, want 30s", o.requestTimeout)
	}
	if o.maxRetries != 2 {
		t.Errorf("default maxRetries: got %d, want 2", o.maxRetries)
	}
	if o.loadBalancerStrategy != "round_robin" {
		t.Errorf("default loadBalancerStrategy: got %s, want round_robin", o.loadBalancerStrategy)
	}
	if o.connectionPoolSize != 10 {
		t.Errorf("default connectionPoolSize: got %d, want 10", o.connectionPoolSize)
	}
	if o.healthCheckInterval != 10*time.Second {
		t.Errorf("default healthCheckInterval: got %v, want 10s", o.healthCheckInterval)
	}
}

// =========================================================================
// Close / Graceful Shutdown Tests
// =========================================================================

func TestClose_GracefulShutdown(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"},
		WithServingHealthCheckInterval(50*time.Millisecond),
	)

	// Start a predict in background
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Predict(context.Background(), newTestPredictRequest("m"))
	}()

	// Wait for predict to start
	time.Sleep(10 * time.Millisecond)

	// Close should wait for in-flight
	err := c.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	<-done
}

func TestClose_RejectsNewRequests(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"})
	c.Close()

	_, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}

	_, err = c.BatchPredict(context.Background(), []*PredictRequest{newTestPredictRequest("m")})
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed for BatchPredict, got %v", err)
	}

	_, err = c.StreamPredict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed for StreamPredict, got %v", err)
	}

	_, err = c.GetModelStatus(context.Background(), "m")
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed for GetModelStatus, got %v", err)
	}

	_, err = c.ListServingModels(context.Background())
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed for ListServingModels, got %v", err)
	}

	err = c.Healthy(context.Background())
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed for Healthy, got %v", err)
	}
}

// =========================================================================
// Concurrent Safety Tests
// =========================================================================

func TestConcurrent_MultiplePredict(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500", "localhost:8501"})
	defer c.Close()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := newTestPredictRequest(fmt.Sprintf("model-%d", idx%5))
			_, err := c.Predict(context.Background(), req)
			if err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent Predict error: %v", err)
	}
}

func TestConcurrent_PredictDuringHealthCheck(t *testing.T) {
	c, _ := NewGRPCServingClient([]string{"localhost:8500"},
		WithServingHealthCheckInterval(10*time.Millisecond),
	)
	defer c.Close()

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					c.Predict(context.Background(), newTestPredictRequest("m"))
				}
			}
		}()
	}

	wg.Wait()
}

func TestConcurrent_MockClient(t *testing.T) {
	m := NewMockServingClient()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Predict(context.Background(), newTestPredictRequest("m"))
			m.Healthy(context.Background())
		}()
	}

	wg.Wait()

	history := m.CallHistory()
	if len(history) != 100 {
		t.Errorf("expected 100 calls, got %d", len(history))
	}
}

func TestConcurrent_LoadBalancer(t *testing.T) {
	b := NewRoundRobinBalancer()
	nodes := newTestNodes(3)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.Select(context.Background(), nodes)
			if err != nil {
				t.Errorf("Select error: %v", err)
			}
		}()
	}

	wg.Wait()
}

// =========================================================================
// InputFormat Tests
// =========================================================================

func TestInputFormat_String(t *testing.T) {
	tests := []struct {
		f    InputFormat
		want string
	}{
		{FormatJSON, "JSON"},
		{FormatProtobuf, "Protobuf"},
		{FormatNumpy, "Numpy"},
		{InputFormat(99), "Unknown"},
	}
	for _, tc := range tests {
		if got := tc.f.String(); got != tc.want {
			t.Errorf("InputFormat(%d).String() = %s, want %s", tc.f, got, tc.want)
		}
	}
}

// =========================================================================
// PredictRequest Validation Tests
// =========================================================================

func TestPredictRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		req     *PredictRequest
		wantErr bool
	}{
		{"nil request", nil, true},
		{"empty model name", &PredictRequest{InputData: []byte("x")}, true},
		{"empty input data", &PredictRequest{ModelName: "m"}, true},
		{"valid", &PredictRequest{ModelName: "m", InputData: []byte("x")}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error= %v, wantErr = %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
		})
	}
}

// =========================================================================
// ServingNode Tests
// =========================================================================

func TestServingNode_ReportSuccess(t *testing.T) {
	n := &ServingNode{Address: "n", Healthy: true, Weight: 1.0}

	n.ReportSuccess(10.0)
	if n.AvgLatencyMs != 10.0 {
		t.Errorf("expected avg latency 10, got %f", n.AvgLatencyMs)
	}
	if n.ErrorRate != 0 {
		t.Errorf("expected error rate 0, got %f", n.ErrorRate)
	}

	n.ReportSuccess(20.0)
	expected := 15.0 // (10+20)/2
	if math.Abs(n.AvgLatencyMs-expected) > 0.01 {
		t.Errorf("expected avg latency %.1f, got %f", expected, n.AvgLatencyMs)
	}
}

func TestServingNode_ReportFailure(t *testing.T) {
	n := &ServingNode{Address: "n", Healthy: true, Weight: 1.0}

	n.ReportFailure(10.0)
	if n.ErrorRate != 1.0 {
		t.Errorf("expected error rate 1.0, got %f", n.ErrorRate)
	}

	n.ReportSuccess(10.0)
	if n.ErrorRate != 0.5 {
		t.Errorf("expected error rate 0.5, got %f", n.ErrorRate)
	}
}

func TestServingNode_MixedReports(t *testing.T) {
	n := &ServingNode{Address: "n", Healthy: true, Weight: 1.0}

	// 3 successes, 1 failure
	n.ReportSuccess(10.0)
	n.ReportSuccess(20.0)
	n.ReportSuccess(30.0)
	n.ReportFailure(40.0)

	expectedAvg := (10.0 + 20.0 + 30.0 + 40.0) / 4.0
	if math.Abs(n.AvgLatencyMs-expectedAvg) > 0.01 {
		t.Errorf("expected avg latency %.1f, got %f", expectedAvg, n.AvgLatencyMs)
	}

	expectedErrRate := 1.0 / 4.0
	if math.Abs(n.ErrorRate-expectedErrRate) > 0.01 {
		t.Errorf("expected error rate %.2f, got %f", expectedErrRate, n.ErrorRate)
	}
}

func TestServingNode_ConcurrentReports(t *testing.T) {
	n := &ServingNode{Address: "n", Healthy: true, Weight: 1.0}
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%3 == 0 {
				n.ReportFailure(float64(idx))
			} else {
				n.ReportSuccess(float64(idx))
			}
		}(i)
	}

	wg.Wait()

	n.mu.Lock()
	defer n.mu.Unlock()
	if n.totalRequests != 100 {
		t.Errorf("expected 100 total requests, got %d", n.totalRequests)
	}
}

// =========================================================================
// Codec Tests
// =========================================================================

func TestEncodeDecodeFloat32Vector(t *testing.T) {
	original := []float32{1.0, 2.5, -3.14, 0.0, math.MaxFloat32, math.SmallestNonzeroFloat32}
	encoded := EncodeFloat32Vector(original)

	decoded, err := DecodeFloat32Vector(encoded)
	if err != nil {
		t.Fatalf("DecodeFloat32Vector error: %v", err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(original))
	}

	for i, v := range original {
		if decoded[i] != v {
			t.Errorf("index %d: got %f, want %f", i, decoded[i], v)
		}
	}
}

func TestDecodeFloat32Vector_InvalidLength(t *testing.T) {
	_, err := DecodeFloat32Vector([]byte{1, 2, 3}) // 3 bytes, not divisible by 4
	if err == nil {
		t.Fatal("expected error for invalid length")
	}
}

func TestDecodeFloat32Vector_Empty(t *testing.T) {
	decoded, err := DecodeFloat32Vector([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decoded) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(decoded))
	}
}

func TestEncodeFloat32Vector_Empty(t *testing.T) {
	encoded := EncodeFloat32Vector([]float32{})
	if len(encoded) != 0 {
		t.Errorf("expected empty bytes, got %d bytes", len(encoded))
	}
}

func TestEncodeFloat32Vector_SingleValue(t *testing.T) {
	encoded := EncodeFloat32Vector([]float32{42.0})
	decoded, err := DecodeFloat32Vector(encoded)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(decoded) != 1 || decoded[0] != 42.0 {
		t.Errorf("expected [42.0], got %v", decoded)
	}
}

func TestEncodeMolecularGraph(t *testing.T) {
	nodeFeatures := [][]float32{{1.0, 2.0}, {3.0, 4.0}}
	edgeIndex := [][2]int{{0, 1}, {1, 0}}
	edgeFeatures := [][]float32{{0.5}, {0.5}}
	globalFeatures := []float32{1.0, 0.0}

	data := EncodeMolecularGraph(nodeFeatures, edgeIndex, edgeFeatures, globalFeatures)
	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := parsed["node_features"]; !ok {
		t.Error("missing node_features key")
	}
	if _, ok := parsed["edge_index"]; !ok {
		t.Error("missing edge_index key")
	}
	if _, ok := parsed["edge_features"]; !ok {
		t.Error("missing edge_features key")
	}
	if _, ok := parsed["global_features"]; !ok {
		t.Error("missing global_features key")
	}
}

func TestEncodeMolecularGraph_Empty(t *testing.T) {
	data := EncodeMolecularGraph(nil, nil, nil, nil)
	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data even with nil inputs")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

// =========================================================================
// ID Generator Tests
// =========================================================================

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := generateID()
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestGenerateID_NonEmpty(t *testing.T) {
	id := generateID()
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestGenerateID_ContainsHyphen(t *testing.T) {
	id := generateID()
	if !strings.Contains(id, "-") {
		t.Errorf("expected ID to contain hyphen: %s", id)
	}
}

func TestGenerateID_Concurrent(t *testing.T) {
	var wg sync.WaitGroup
	ids := make([]string, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ids[idx] = generateID()
		}(i)
	}

	wg.Wait()

	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate concurrent ID: %s", id)
		}
		seen[id] = true
	}
}

// =========================================================================
// mapHTTPStatus Tests
// =========================================================================

func TestMapHTTPStatus(t *testing.T) {
	tests := []struct {
		code    int
		wantErr error
		wantNil bool
	}{
		{200, nil, true},
		{201, nil, true},
		{204, nil, true},
		{400, ErrInvalidInput, false},
		{404, ErrModelNotDeployed, false},
		{408, ErrInferenceTimeout, false},
		{504, ErrInferenceTimeout, false},
		{503, ErrServingUnavailable, false},
		{500, ErrServingUnavailable, false},
		{502, ErrServingUnavailable, false},
		{301, nil, false}, // unexpected
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.code), func(t *testing.T) {
			err := mapHTTPStatus(tc.code)
			if tc.wantNil {
				if err != nil {
					t.Errorf("expected nil error for %d, got %v", tc.code, err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error for %d", tc.code)
				return
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Errorf("status %d: expected %v, got %v", tc.code, tc.wantErr, err)
			}
		})
	}
}

// =========================================================================
// isRetryableGRPCError Tests
// =========================================================================

func TestIsRetryableGRPCError(t *testing.T) {
	tests := []struct {
		err       error
		retryable bool
	}{
		{nil, false},
		{fmt.Errorf("unavailable"), true},
		{fmt.Errorf("deadline exceeded"), true},
		{fmt.Errorf("connection refused"), true},
		{fmt.Errorf("permission denied"), false},
		{fmt.Errorf("not found"), false},
		{fmt.Errorf("rpc error: code = Unavailable"), true},
	}

	for _, tc := range tests {
		name := "nil"
		if tc.err != nil {
			name = tc.err.Error()
		}
		t.Run(name, func(t *testing.T) {
			got := isRetryableGRPCError(tc.err)
			if got != tc.retryable {
				t.Errorf("isRetryableGRPCError(%v) = %v, want %v", tc.err, got, tc.retryable)
			}
		})
	}
}

// =========================================================================
// filterHealthy Tests
// =========================================================================

func TestFilterHealthy_AllHealthy(t *testing.T) {
	nodes := newTestNodes(3)
	healthy := filterHealthy(nodes)
	if len(healthy) != 3 {
		t.Errorf("expected 3 healthy, got %d", len(healthy))
	}
}

func TestFilterHealthy_SomeUnhealthy(t *testing.T) {
	nodes := newTestNodes(4)
	nodes[1].Healthy = false
	nodes[3].Healthy = false

	healthy := filterHealthy(nodes)
	if len(healthy) != 2 {
		t.Errorf("expected 2 healthy, got %d", len(healthy))
	}
}

func TestFilterHealthy_NoneHealthy(t *testing.T) {
	nodes := newTestNodes(3)
	for _, n := range nodes {
		n.Healthy = false
	}

	healthy := filterHealthy(nodes)
	if len(healthy) != 0 {
		t.Errorf("expected 0 healthy, got %d", len(healthy))
	}
}

func TestFilterHealthy_Empty(t *testing.T) {
	healthy := filterHealthy([]*ServingNode{})
	if len(healthy) != 0 {
		t.Errorf("expected 0 healthy, got %d", len(healthy))
	}
}

// =========================================================================
// ServingVersionStatus Tests
// =========================================================================

func TestServingVersionStatusState_Values(t *testing.T) {
	if VersionReady != "Ready" {
		t.Errorf("VersionReady = %s, want Ready", VersionReady)
	}
	if VersionLoading != "Loading" {
		t.Errorf("VersionLoading = %s, want Loading", VersionLoading)
	}
	if VersionUnloading != "Unloading" {
		t.Errorf("VersionUnloading = %s, want Unloading", VersionUnloading)
	}
	if VersionFailed != "Failed" {
		t.Errorf("VersionFailed = %s, want Failed", VersionFailed)
	}
}

func TestServingVersionStatus_JSON(t *testing.T) {
	status := ServingVersionStatus{
		Version:         "1",
		Status:          VersionReady,
		LastInferenceAt: time.Now(),
		InferenceCount:  42,
		AvgLatencyMs:    5.5,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ServingVersionStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Version != "1" {
		t.Errorf("expected version 1, got %s", decoded.Version)
	}
	if decoded.Status != VersionReady {
		t.Errorf("expected Ready, got %s", decoded.Status)
	}
	if decoded.InferenceCount != 42 {
		t.Errorf("expected 42, got %d", decoded.InferenceCount)
	}
}

func TestServingModelStatus_JSON(t *testing.T) {
	status := ServingModelStatus{
		ModelName:      "test-model",
		DefaultVersion: "2",
		Versions: []*ServingVersionStatus{
			{Version: "1", Status: VersionReady},
			{Version: "2", Status: VersionLoading},
		},
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ServingModelStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ModelName != "test-model" {
		t.Errorf("expected test-model, got %s", decoded.ModelName)
	}
	if len(decoded.Versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(decoded.Versions))
	}
}

// =========================================================================
// PredictRequest / PredictResponse JSON Tests
// =========================================================================

func TestPredictRequest_JSON(t *testing.T) {
	req := &PredictRequest{
		ModelName:    "my-model",
		ModelVersion: "3",
		InputName:    "input_0",
		InputData:    []byte(`{"features":[1,2,3]}`),
		InputFormat:  FormatJSON,
		OutputNames:  []string{"output_0", "output_1"},
		Metadata:     map[string]string{"key": "value"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PredictRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ModelName != "my-model" {
		t.Errorf("expected my-model, got %s", decoded.ModelName)
	}
	if decoded.ModelVersion != "3" {
		t.Errorf("expected version 3, got %s", decoded.ModelVersion)
	}
	if len(decoded.OutputNames) != 2 {
		t.Errorf("expected 2 output names, got %d", len(decoded.OutputNames))
	}
	if decoded.Metadata["key"] != "value" {
		t.Errorf("expected metadata key=value, got %s", decoded.Metadata["key"])
	}
}

func TestPredictResponse_JSON(t *testing.T) {
	resp := &PredictResponse{
		ModelName:       "my-model",
		ModelVersion:    "1",
		Outputs:         map[string][]byte{"embedding": {1, 2, 3, 4}},
		OutputFormat:    FormatJSON,
		InferenceTimeMs: 42,
		Metadata:        map[string]string{"trace_id": "abc"},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded PredictResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.ModelName != "my-model" {
		t.Errorf("expected my-model, got %s", decoded.ModelName)
	}
	if decoded.InferenceTimeMs != 42 {
		t.Errorf("expected 42ms, got %d", decoded.InferenceTimeMs)
	}
}

// =========================================================================
// HTTP Client with Interceptors Integration Test
// =========================================================================

func TestHTTP_Predict_WithInterceptors(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	logger := &servingMockLogger{}
	metrics := &servingMockMetrics{}

	c, err := NewHTTPServingClient(srv.URL,
		WithInterceptors(
			NewTracingInterceptor(),
			NewLoggingInterceptor(logger, LoggingInterceptorConfig{}),
			NewMetricsInterceptor(metrics),
			NewAuthInterceptor(AuthModeAPIKey, "test-key"),
		),
	)
	if err != nil {
		t.Fatalf("NewHTTPServingClient: %v", err)
	}
	defer c.Close()

	req := newTestPredictRequest("test-model")
	resp, err := c.Predict(context.Background(), req)
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}

	if resp.ModelName != "test-model" {
		t.Errorf("expected test-model, got %s", resp.ModelName)
	}

	// Verify tracing
	if req.Metadata["trace_id"] == "" {
		t.Error("expected trace_id to be set by tracing interceptor")
	}

	// Verify auth
	if req.Metadata["X-API-Key"] != "test-key" {
		t.Error("expected X-API-Key to be set by auth interceptor")
	}

	// Verify logging
	entries := logger.Entries()
	if len(entries) == 0 {
		t.Error("expected log entries from logging interceptor")
	}

	// Verify metrics
	if metrics.inferenceCount.Load() == 0 {
		t.Error("expected metrics to be recorded")
	}
}

// =========================================================================
// gRPC Client with Options Integration Test
// =========================================================================

func TestGRPC_WithAllOptions(t *testing.T) {
	logger := &servingMockLogger{}
	metrics := &servingMockMetrics{}

	c, err := NewGRPCServingClient(
		[]string{"localhost:8500", "localhost:8501"},
		WithConnectionTimeout(10*time.Second),
		WithRequestTimeout(60*time.Second),
		WithMaxRetries(3),
		WithLoadBalancerStrategy("weighted"),
		WithConnectionPoolSize(20),
		WithServingHealthCheckInterval(30*time.Second),
		WithServingLogger(logger),
		WithServingMetrics(metrics),
		WithInterceptors(
			NewTracingInterceptor(),
			NewLoggingInterceptor(logger, LoggingInterceptorConfig{}),
		),
	)
	if err != nil {
		t.Fatalf("NewGRPCServingClient: %v", err)
	}
	defer c.Close()

	if c.opts.connectionTimeout != 10*time.Second {
		t.Errorf("connectionTimeout: got %v, want 10s", c.opts.connectionTimeout)
	}
	if c.opts.requestTimeout != 60*time.Second {
		t.Errorf("requestTimeout: got %v, want 60s", c.opts.requestTimeout)
	}
	if c.opts.maxRetries != 3 {
		t.Errorf("maxRetries: got %d, want 3", c.opts.maxRetries)
	}
	if c.opts.connectionPoolSize != 20 {
		t.Errorf("connectionPoolSize: got %d, want 20", c.opts.connectionPoolSize)
	}
	if len(c.opts.interceptors) != 2 {
		t.Errorf("interceptors: got %d, want 2", len(c.opts.interceptors))
	}

	// Verify predict still works
	resp, err := c.Predict(context.Background(), newTestPredictRequest("m"))
	if err != nil {
		t.Fatalf("Predict error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

// =========================================================================
// HTTP StreamPredict Integration Test
// =========================================================================

func TestHTTP_StreamPredict_Success(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	ch, err := c.StreamPredict(context.Background(), newTestPredictRequest("test-model"))
	if err != nil {
		t.Fatalf("StreamPredict error: %v", err)
	}

	count := 0
	for resp := range ch {
		if resp == nil {
			t.Error("nil response in stream")
		}
		count++
	}
	if count == 0 {
		t.Error("expected at least one response from stream")
	}
}

func TestHTTP_StreamPredict_ClientClosed(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	c.Close()

	_, err := c.StreamPredict(context.Background(), newTestPredictRequest("m"))
	if !errors.Is(err, ErrClientClosed) {
		t.Errorf("expected ErrClientClosed, got %v", err)
	}
}

func TestHTTP_StreamPredict_Validation(t *testing.T) {
	srv := startTestHTTPServer(t, multiRouteHandler(t))
	defer srv.Close()

	c, _ := NewHTTPServingClient(srv.URL)
	defer c.Close()

	_, err := c.StreamPredict(context.Background(), &PredictRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// =========================================================================
// Sentinel Error Tests
// =========================================================================

func TestSentinelErrors_Distinct(t *testing.T) {
	errs := []error{
		ErrServingUnavailable,
		ErrModelNotDeployed,
		ErrInvalidInput,
		ErrInferenceTimeout,
		ErrAllNodesUnhealthy,
		ErrClientClosed,
	}

	for i := 0; i < len(errs); i++ {
		for j := i + 1; j < len(errs); j++ {
			if errors.Is(errs[i], errs[j]) {
				t.Errorf("sentinel errors should be distinct: %v == %v", errs[i], errs[j])
			}
		}
	}
}

func TestSentinelErrors_Wrapping(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", ErrServingUnavailable)
	if !errors.Is(wrapped, ErrServingUnavailable) {
		t.Error("wrapped error should match ErrServingUnavailable")
	}
}

// =========================================================================
// Compile-time interface satisfaction (redundant with var block, but explicit)
// =========================================================================

func TestInterfaceSatisfaction(t *testing.T) {
	var _ ServingClient = (*grpcServingClient)(nil)
	var _ ServingClient = (*httpServingClient)(nil)
	var _ ServingClient = (*mockServingClient)(nil)
	var _ LoadBalancer = (*roundRobinBalancer)(nil)
	var _ LoadBalancer = (*leastConnectionsBalancer)(nil)
	var _ LoadBalancer = (*weightedBalancer)(nil)
	var _ RequestInterceptor = (*tracingInterceptor)(nil)
	var _ RequestInterceptor = (*loggingInterceptor)(nil)
	var _ RequestInterceptor = (*authInterceptor)(nil)
	var _ RequestInterceptor = (*metricsInterceptor)(nil)
}

// Suppress unused import warnings
var (
	_ = io.EOF
	_ = math.MaxFloat64
)

