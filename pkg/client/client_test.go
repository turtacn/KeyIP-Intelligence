package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewClient_Success(t *testing.T) {
	client, err := NewClient("https://api.example.com", "test-api-key")

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "https://api.example.com", client.baseURL)
	assert.Equal(t, "test-api-key", client.apiKey)
}

func TestNewClient_EmptyBaseURL(t *testing.T) {
	client, err := NewClient("", "test-api-key")

	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_EmptyAPIKey(t *testing.T) {
	client, err := NewClient("https://api.example.com", "")

	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_InvalidScheme(t *testing.T) {
	client, err := NewClient("ftp://api.example.com", "test-api-key")

	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	client, err := NewClient("https://api.example.com/", "test-api-key")

	assert.NoError(t, err)
	assert.Equal(t, "https://api.example.com", client.baseURL)
}

func TestNewClient_InvalidURL(t *testing.T) {
	client, err := NewClient("://invalid-url", "test-api-key")

	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestNewClient_WithOptions(t *testing.T) {
	customHTTP := &http.Client{Timeout: 60 * time.Second}
	logger := &testLogger{}

	client, err := NewClient(
		"https://api.example.com",
		"test-api-key",
		WithHTTPClient(customHTTP),
		WithLogger(logger),
		WithRetryMax(5),
		WithUserAgent("custom-agent/1.0"),
	)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, customHTTP, client.httpClient)
	assert.Equal(t, logger, client.logger)
	assert.Equal(t, 5, client.retryMax)
	assert.Equal(t, "custom-agent/1.0", client.userAgent)
}

func TestClient_Molecules(t *testing.T) {
	client, _ := NewClient("https://api.example.com", "test-api-key")

	molecules := client.Molecules()
	assert.NotNil(t, molecules)

	// Should return the same instance on multiple calls (thread-safe singleton)
	molecules2 := client.Molecules()
	assert.Same(t, molecules, molecules2)
}

func TestClient_Patents(t *testing.T) {
	client, _ := NewClient("https://api.example.com", "test-api-key")

	patents := client.Patents()
	assert.NotNil(t, patents)

	// Should return the same instance on multiple calls
	patents2 := client.Patents()
	assert.Same(t, patents, patents2)
}

func TestClient_Lifecycle(t *testing.T) {
	client, _ := NewClient("https://api.example.com", "test-api-key")

	lifecycle := client.Lifecycle()
	assert.NotNil(t, lifecycle)

	// Should return the same instance on multiple calls
	lifecycle2 := client.Lifecycle()
	assert.Same(t, lifecycle, lifecycle2)
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		StatusCode: 404,
		Code:       "NOT_FOUND",
		Message:    "Resource not found",
		RequestID:  "req-123",
	}

	errorStr := err.Error()
	assert.Contains(t, errorStr, "NOT_FOUND")
	assert.Contains(t, errorStr, "404")
	assert.Contains(t, errorStr, "Resource not found")
	assert.Contains(t, errorStr, "req-123")
}

func TestAPIError_IsNotFound(t *testing.T) {
	err := &APIError{StatusCode: 404}
	assert.True(t, err.IsNotFound())

	err2 := &APIError{StatusCode: 500}
	assert.False(t, err2.IsNotFound())
}

func TestAPIError_IsUnauthorized(t *testing.T) {
	err := &APIError{StatusCode: 401}
	assert.True(t, err.IsUnauthorized())

	err2 := &APIError{StatusCode: 403}
	assert.False(t, err2.IsUnauthorized())
}

func TestAPIError_IsRateLimited(t *testing.T) {
	err := &APIError{StatusCode: 429}
	assert.True(t, err.IsRateLimited())

	err2 := &APIError{StatusCode: 400}
	assert.False(t, err2.IsRateLimited())
}

func TestAPIError_IsServerError(t *testing.T) {
	testCases := []struct {
		code     int
		expected bool
	}{
		{500, true},
		{502, true},
		{503, true},
		{599, true},
		{400, false},
		{404, false},
		{600, false},
	}

	for _, tc := range testCases {
		err := &APIError{StatusCode: tc.code}
		assert.Equal(t, tc.expected, err.IsServerError(), "status code %d", tc.code)
	}
}

func TestClient_do_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("X-Request-ID"))

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key")

	var result map[string]string
	err := client.get(context.Background(), "/test", &result)

	assert.NoError(t, err)
	assert.Equal(t, "ok", result["status"])
}

func TestClient_do_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"code":    "INVALID_REQUEST",
			"message": "Bad request",
		})
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key")

	var result map[string]string
	err := client.get(context.Background(), "/test", &result)

	assert.Error(t, err)
	apiErr, ok := err.(*APIError)
	assert.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "INVALID_REQUEST", apiErr.Code)
}

func TestClient_do_WithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "test-value", body["key"])

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"received": "ok"})
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key")

	var result map[string]string
	err := client.post(context.Background(), "/test", map[string]string{"key": "test-value"}, &result)

	assert.NoError(t, err)
	assert.Equal(t, "ok", result["received"])
}

func TestClient_do_PathPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/test", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key")

	// Path without leading slash should still work
	err := client.get(context.Background(), "test", nil)
	assert.NoError(t, err)
}

func TestClient_do_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := client.get(ctx, "/test", nil)
	assert.Error(t, err)
}

func TestClient_shouldRetry(t *testing.T) {
	client, _ := NewClient("https://api.example.com", "test-api-key")

	// Should retry on network error
	assert.True(t, client.shouldRetry(nil, assert.AnError))

	// Should retry on 5xx
	resp500 := &http.Response{StatusCode: 500}
	assert.True(t, client.shouldRetry(resp500, nil))

	resp503 := &http.Response{StatusCode: 503}
	assert.True(t, client.shouldRetry(resp503, nil))

	// Should NOT retry on 4xx (except 429)
	resp400 := &http.Response{StatusCode: 400}
	assert.False(t, client.shouldRetry(resp400, nil))

	resp404 := &http.Response{StatusCode: 404}
	assert.False(t, client.shouldRetry(resp404, nil))
}

func TestClient_calculateBackoff(t *testing.T) {
	client, _ := NewClient("https://api.example.com", "test-api-key", WithRetryWait(100*time.Millisecond, 1*time.Second))

	// First attempt backoff
	backoff1 := client.calculateBackoff(1)
	assert.GreaterOrEqual(t, backoff1, 100*time.Millisecond)
	assert.LessOrEqual(t, backoff1, 125*time.Millisecond) // With 25% jitter

	// Second attempt backoff (should be larger)
	backoff2 := client.calculateBackoff(2)
	assert.GreaterOrEqual(t, backoff2, 200*time.Millisecond)

	// Large attempt should be capped at max
	backoffLarge := client.calculateBackoff(10)
	assert.LessOrEqual(t, backoffLarge, 1250*time.Millisecond) // Max + 25% jitter
}

func TestClient_delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key")

	err := client.delete(context.Background(), "/resource/123")
	assert.NoError(t, err)
}

func TestClient_put(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"updated": "true"})
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key")

	var result map[string]string
	err := client.put(context.Background(), "/resource/123", map[string]string{"name": "updated"}, &result)

	assert.NoError(t, err)
	assert.Equal(t, "true", result["updated"])
}

func TestNoopLogger(t *testing.T) {
	// Should not panic or error
	logger := &noopLogger{}
	logger.Debugf("test %s", "debug")
	logger.Infof("test %s", "info")
	logger.Errorf("test %s", "error")
}

func TestResponseMeta(t *testing.T) {
	meta := &ResponseMeta{
		Total:    100,
		Page:     2,
		PageSize: 20,
		HasMore:  true,
	}

	assert.Equal(t, int64(100), meta.Total)
	assert.Equal(t, 2, meta.Page)
	assert.Equal(t, 20, meta.PageSize)
	assert.True(t, meta.HasMore)
}

func TestVersion(t *testing.T) {
	assert.NotEmpty(t, Version)
	assert.Equal(t, "0.1.0", Version)
}

func TestClient_do_RetryOnServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client, _ := NewClient(server.URL, "test-api-key", WithRetryMax(3), WithRetryWait(10*time.Millisecond, 50*time.Millisecond))

	var result map[string]string
	err := client.get(context.Background(), "/test", &result)

	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, "ok", result["status"])
}
