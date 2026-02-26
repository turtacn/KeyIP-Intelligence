// Phase 13 - SDK Client Tests (293/349)
// File: pkg/client/client_test.go
// Unit tests for the core SDK client.

package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	keyiperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Test Helpers
// ---------------------------------------------------------------------------

func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, "test-api-key", opts...)
	require.NoError(t, err)
	return client
}

type testLogger struct {
	lastMsg string
	count   int32
}

func (l *testLogger) Debugf(format string, args ...interface{}) {
	l.log(format, args...)
}
func (l *testLogger) Infof(format string, args ...interface{}) {
	l.log(format, args...)
}
func (l *testLogger) Errorf(format string, args ...interface{}) {
	l.log(format, args...)
}
func (l *testLogger) log(format string, args ...interface{}) {
	atomic.AddInt32(&l.count, 1)
	l.lastMsg = fmt.Sprintf(format, args...)
}

// ---------------------------------------------------------------------------
// Constructor Tests
// ---------------------------------------------------------------------------

func TestNewClient_Success(t *testing.T) {
	c, err := NewClient("http://api.example.com", "key")
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, "http://api.example.com", c.baseURL) // Trailing slash trimmed? No, input didn't have it.
	assert.Equal(t, 3, c.retryMax)
	assert.Contains(t, c.userAgent, "keyip-go-sdk/")
}

func TestNewClient_EmptyBaseURL(t *testing.T) {
	_, err := NewClient("", "key")
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidConfig)
}

func TestNewClient_InvalidBaseURL(t *testing.T) {
	_, err := NewClient("ftp://invalid", "key")
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidConfig)
	_, err = NewClient("invalid-url", "key")
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidConfig)
}

func TestNewClient_EmptyAPIKey(t *testing.T) {
	_, err := NewClient("http://api.example.com", "")
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidConfig)
}

func TestNewClient_BaseURLTrailingSlash(t *testing.T) {
	c, err := NewClient("http://api.example.com/", "key")
	assert.NoError(t, err)
	assert.Equal(t, "http://api.example.com", c.baseURL)
}

func TestNewClient_WithOptions(t *testing.T) {
	customClient := &http.Client{Timeout: 10 * time.Second}
	logger := &testLogger{}
	c, err := NewClient("http://api.example.com", "key",
		WithHTTPClient(customClient),
		WithLogger(logger),
		WithRetryMax(5),
	)
	assert.NoError(t, err)
	assert.Equal(t, customClient, c.httpClient)
	assert.Equal(t, logger, c.logger)
	assert.Equal(t, 5, c.retryMax)
}

// ---------------------------------------------------------------------------
// Lazy Init Tests
// ---------------------------------------------------------------------------

func TestClient_Molecules_LazyInit(t *testing.T) {
	c, _ := NewClient("http://api.example.com", "key")
	assert.Nil(t, c.molecules)
	m1 := c.Molecules()
	assert.NotNil(t, m1)
	m2 := c.Molecules()
	assert.Same(t, m1, m2)
}

func TestClient_Patents_LazyInit(t *testing.T) {
	c, _ := NewClient("http://api.example.com", "key")
	assert.Nil(t, c.patents)
	p1 := c.Patents()
	assert.NotNil(t, p1)
	p2 := c.Patents()
	assert.Same(t, p1, p2)
}

func TestClient_Lifecycle_LazyInit(t *testing.T) {
	c, _ := NewClient("http://api.example.com", "key")
	assert.Nil(t, c.lifecycle)
	l1 := c.Lifecycle()
	assert.NotNil(t, l1)
	l2 := c.Lifecycle()
	assert.Same(t, l1, l2)
}

func TestClient_SubClients_ConcurrentAccess(t *testing.T) {
	c, _ := NewClient("http://api.example.com", "key")
	var wg sync.WaitGroup
	molecules := make([]*MoleculesClient, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			molecules[idx] = c.Molecules()
		}(i)
	}
	wg.Wait()

	first := molecules[0]
	for i := 1; i < 100; i++ {
		assert.Same(t, first, molecules[i])
	}
}

// ---------------------------------------------------------------------------
// HTTP Execution Tests (do)
// ---------------------------------------------------------------------------

func TestClient_Do_Success(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": "success"}`))
	}
	c := newTestClient(t, handler)
	var resp APIResponse[string]
	err := c.get(context.Background(), "/test", &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Data)
}

func TestClient_Do_NilBody(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, int64(0), r.ContentLength)
		w.WriteHeader(http.StatusOK)
	}
	c := newTestClient(t, handler)
	err := c.get(context.Background(), "/test", nil)
	assert.NoError(t, err)
}

func TestClient_Do_NilResult(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ignored": true}`))
	}
	c := newTestClient(t, handler)
	err := c.get(context.Background(), "/test", nil)
	assert.NoError(t, err)
}

func TestClient_Do_RequestHeaders(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Contains(t, r.Header.Get("User-Agent"), "keyip-go-sdk/")
		assert.NotEmpty(t, r.Header.Get("X-Request-ID"))
		w.WriteHeader(http.StatusOK)
	}
	c := newTestClient(t, handler)
	c.get(context.Background(), "/test", nil)
}

func TestClient_Do_RequestID_Unique(t *testing.T) {
	ids := make(chan string, 2)
	handler := func(w http.ResponseWriter, r *http.Request) {
		ids <- r.Header.Get("X-Request-ID")
		w.WriteHeader(http.StatusOK)
	}
	c := newTestClient(t, handler)
	c.get(context.Background(), "/test", nil)
	c.get(context.Background(), "/test", nil)
	close(ids)

	id1 := <-ids
	id2 := <-ids
	assert.NotEqual(t, id1, id2)
}

func TestClient_Do_4xxError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code": "NOT_FOUND", "message": "Missing"}`))
	}
	c := newTestClient(t, handler)
	err := c.get(context.Background(), "/test", nil)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 404, apiErr.StatusCode)
	assert.Equal(t, "NOT_FOUND", apiErr.Code)
	assert.Equal(t, "Missing", apiErr.Message)
	assert.NotEmpty(t, apiErr.RequestID)
}

func TestClient_Do_4xxNoRetry(t *testing.T) {
	var calls int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}
	c := newTestClient(t, handler)
	err := c.get(context.Background(), "/test", nil)
	assert.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestClient_Do_5xxRetry(t *testing.T) {
	var calls int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&calls, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	// Use explicit retry wait to speed up test
	c := newTestClient(t, handler, WithRetryWait(1*time.Millisecond, 2*time.Millisecond))
	err := c.get(context.Background(), "/test", nil)
	assert.NoError(t, err)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestClient_Do_5xxRetryExhausted(t *testing.T) {
	var calls int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}
	c := newTestClient(t, handler, WithRetryMax(2), WithRetryWait(1*time.Millisecond, 2*time.Millisecond))
	err := c.get(context.Background(), "/test", nil)
	assert.Error(t, err)
	// 1 initial + 2 retries = 3 calls
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestClient_Do_429RetryAfter(t *testing.T) {
	var calls int32
	handler := func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&calls, 1)
		if count == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
	// Note: sleep 1s in test might be slow, but it verifies logic
	c := newTestClient(t, handler)
	start := time.Now()
	err := c.get(context.Background(), "/test", nil)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
	assert.True(t, elapsed >= 1*time.Second, "Should wait at least 1s for Retry-After")
}

func TestClient_Do_NetworkError(t *testing.T) {
	// Create server then close it to force connection error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	c, _ := NewClient(server.URL, "key", WithRetryMax(1), WithRetryWait(1*time.Millisecond, 2*time.Millisecond))
	err := c.get(context.Background(), "/test", nil)
	assert.Error(t, err)
	// Can't easily count calls on closed server, but err should be returned
}

func TestClient_Do_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	err := c.get(ctx, "/test", nil)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestClient_Do_ContextTimeout(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}
	c := newTestClient(t, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := c.get(ctx, "/test", nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// ---------------------------------------------------------------------------
// APIError Tests
// ---------------------------------------------------------------------------

func TestAPIError_Methods(t *testing.T) {
	e404 := &APIError{StatusCode: 404}
	assert.True(t, e404.IsNotFound())

	e401 := &APIError{StatusCode: 401}
	assert.True(t, e401.IsUnauthorized())

	e429 := &APIError{StatusCode: 429}
	assert.True(t, e429.IsRateLimited())

	e500 := &APIError{StatusCode: 500}
	assert.True(t, e500.IsServerError())
	e503 := &APIError{StatusCode: 503}
	assert.True(t, e503.IsServerError())

	e400 := &APIError{StatusCode: 400}
	assert.False(t, e400.IsServerError())

	eStr := (&APIError{Code: "ERR", StatusCode: 400, Message: "Msg", RequestID: "ID"}).Error()
	assert.Equal(t, "keyip: ERR (HTTP 400): Msg [request_id=ID]", eStr)
}

// ---------------------------------------------------------------------------
// Convenience Methods Tests
// ---------------------------------------------------------------------------

func TestClient_Methods(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			w.Write(body) // Echo body
		}
	}
	c := newTestClient(t, handler)
	ctx := context.Background()

	assert.NoError(t, c.get(ctx, "/get", nil))

	type Payload struct { Val string `json:"val"` }
	var res Payload
	assert.NoError(t, c.post(ctx, "/post", Payload{Val: "A"}, &res))
	assert.Equal(t, "A", res.Val)

	assert.NoError(t, c.put(ctx, "/put", Payload{Val: "B"}, &res))
	assert.Equal(t, "B", res.Val)

	assert.NoError(t, c.delete(ctx, "/delete"))
}

//Personal.AI order the ending
