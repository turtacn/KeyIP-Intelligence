// Phase 13 - SDK Client (292/349)
// File: pkg/client/client.go
// Core SDK entry point for the KeyIP-Intelligence platform.
// Encapsulates all HTTP communication: construction, signing, retry, timeout, error decoding.
// Upper-layer sub-clients (Molecules, Patents, Lifecycle) only assemble request/response DTOs.

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Version is the SDK semantic version.
const Version = "0.1.0"

// ErrInvalidConfig is returned when client configuration is invalid.
var ErrInvalidConfig = errors.New("keyip: invalid client configuration")

// ---------------------------------------------------------------------------
// Logger
// ---------------------------------------------------------------------------

// Logger defines the minimal logging interface consumed by the SDK.
type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// noopLogger silently discards all log output.
type noopLogger struct{}

func (noopLogger) Debugf(string, ...interface{}) {}
func (noopLogger) Infof(string, ...interface{})  {}
func (noopLogger) Errorf(string, ...interface{}) {}

// ---------------------------------------------------------------------------
// APIError
// ---------------------------------------------------------------------------

// APIError represents a structured error response from the KeyIP API.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
}

// Error implements the error interface.
// Format: keyip: <Code> (HTTP <StatusCode>): <Message> [request_id=<RequestID>]
func (e *APIError) Error() string {
	return fmt.Sprintf("keyip: %s (HTTP %d): %s [request_id=%s]",
		e.Code, e.StatusCode, e.Message, e.RequestID)
}

// IsNotFound returns true when the status code is 404.
func (e *APIError) IsNotFound() bool { return e.StatusCode == http.StatusNotFound }

// IsUnauthorized returns true when the status code is 401.
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == http.StatusUnauthorized }

// IsRateLimited returns true when the status code is 429.
func (e *APIError) IsRateLimited() bool { return e.StatusCode == http.StatusTooManyRequests }

// IsServerError returns true for 500, 502, 503, 504.
func (e *APIError) IsServerError() bool {
	switch e.StatusCode {
	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// APIResponse / ResponseMeta
// ---------------------------------------------------------------------------

// APIResponse is a generic envelope for successful API responses.
type APIResponse[T any] struct {
Data T             `json:"data"`
Meta *ResponseMeta `json:"meta,omitempty"`
}

// ResponseMeta carries pagination metadata.
type ResponseMeta struct {
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	HasMore  bool  `json:"has_more"`
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client is the top-level SDK client. It is safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	userAgent  string
	logger     Logger

	retryMax     int
	retryWaitMin time.Duration
	retryWaitMax time.Duration

	moleculesOnce sync.Once
	molecules     *MoleculesClient

	patentsOnce sync.Once
	patents     *PatentsClient

	lifecycleOnce sync.Once
	lifecycle     *LifecycleClient
}

// NewClient creates a new SDK client.
//
// baseURL must be a valid http/https URL. apiKey must be non-empty.
// Optional behaviour can be customised via Option functions.
func NewClient(baseURL string, apiKey string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("%w: baseURL is required", ErrInvalidConfig)
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("%w: baseURL must use http or https scheme, got %q", ErrInvalidConfig, baseURL)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("%w: apiKey is required", ErrInvalidConfig)
	}

	c := &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       apiKey,
		userAgent:    "keyip-go-sdk/" + Version,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       noopLogger{},
		retryMax:     3,
		retryWaitMin: 500 * time.Millisecond,
		retryWaitMax: 5 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// ---------------------------------------------------------------------------
// Sub-client accessors (lazy, thread-safe via sync.Once)
// ---------------------------------------------------------------------------

// Molecules returns the molecule resource sub-client.
func (c *Client) Molecules() *MoleculesClient {
	c.moleculesOnce.Do(func() {
		c.molecules = &MoleculesClient{client: c}
	})
	return c.molecules
}

// Patents returns the patent resource sub-client.
func (c *Client) Patents() *PatentsClient {
	c.patentsOnce.Do(func() {
		c.patents = &PatentsClient{client: c}
	})
	return c.patents
}

// Lifecycle returns the lifecycle resource sub-client.
func (c *Client) Lifecycle() *LifecycleClient {
	c.lifecycleOnce.Do(func() {
		c.lifecycle = &LifecycleClient{client: c}
	})
	return c.lifecycle
}

// ---------------------------------------------------------------------------
// Convenience HTTP verbs (unexported, used by sub-clients)
// ---------------------------------------------------------------------------

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, result)
}

func (c *Client) post(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, result)
}

func (c *Client) put(ctx context.Context, path string, body interface{}, result interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, result)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// ---------------------------------------------------------------------------
// Core HTTP execution with retry
// ---------------------------------------------------------------------------

// do executes an HTTP request with automatic retry for transient failures.
//
// Retry policy:
//   - 5xx (500/502/503/504): exponential back-off + jitter
//   - 429: honour Retry-After header, then retry
//   - net.Error (timeout, connection refused): exponential back-off + jitter
//   - 4xx (except 429): no retry, return immediately
//   - context cancellation / deadline: return immediately
func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	// Normalise path — ensure leading slash.
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	fullURL := c.baseURL + path

	// Pre-marshal body so it can be replayed across retries.
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("keyip: failed to marshal request body: %w", err)
		}
		bodyBytes = b
	}

	requestID := uuid.New().String()
	maxAttempts := 1 + c.retryMax
	var lastErr error
	skipNextBackoff := false

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// ---- back-off (skip on first attempt and after Retry-After wait) ----
		if attempt > 0 && !skipNextBackoff {
			if err := ctx.Err(); err != nil {
				return err
			}
			wait := c.backoff(attempt)
			c.logger.Debugf("retry %d/%d backoff=%v path=%s", attempt, c.retryMax, wait, path)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		}
		skipNextBackoff = false

		// ---- build request ----
		var reqBody io.Reader
		if bodyBytes != nil {
			reqBody = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
		if err != nil {
			return fmt.Errorf("keyip: failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("X-Request-ID", requestID)

		// ---- execute ----
		start := time.Now()
		resp, err := c.httpClient.Do(req)
		elapsed := time.Since(start)

		// ---- transport / network error ----
		if err != nil {
			lastErr = err
			c.logger.Errorf("%s %s error=%v elapsed=%v", method, path, err, elapsed)

			// Context errors are terminal — never retry.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			// Network errors are retryable.
			continue
		}

		// ---- read body & close immediately (safe for retry loops) ----
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()

		c.logger.Infof("%s %s status=%d elapsed=%v request_id=%s",
			method, path, resp.StatusCode, elapsed, requestID)

		if readErr != nil {
			lastErr = fmt.Errorf("keyip: failed to read response body: %w", readErr)
			continue
		}

		// ---- 429 Rate Limited ----
		if resp.StatusCode == http.StatusTooManyRequests {
			apiErr := c.buildAPIError(resp.StatusCode, respBody, requestID)
			lastErr = apiErr

			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if seconds, parseErr := strconv.Atoi(ra); parseErr == nil && seconds > 0 {
					c.logger.Debugf("429 Retry-After=%ds path=%s", seconds, path)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Duration(seconds) * time.Second):
					}
					skipNextBackoff = true
				}
			}
			continue
		}

		// ---- 5xx Server Error — retryable ----
		if resp.StatusCode >= 500 {
			lastErr = c.buildAPIError(resp.StatusCode, respBody, requestID)
			continue
		}

		// ---- 4xx Client Error — terminal, no retry ----
		if resp.StatusCode >= 400 {
			return c.buildAPIError(resp.StatusCode, respBody, requestID)
		}

		// ---- 2xx / 3xx Success ----
		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("keyip: failed to unmarshal response body: %w", err)
			}
		}
		return nil
	}

	// All attempts exhausted.
	return lastErr
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// buildAPIError parses a JSON error body into an APIError.
func (c *Client) buildAPIError(statusCode int, body []byte, requestID string) *APIError {
	apiErr := &APIError{
		StatusCode: statusCode,
		RequestID:  requestID,
	}
	if len(body) > 0 {
		// Best-effort parse; fields that don't match are silently ignored.
		_ = json.Unmarshal(body, apiErr)
	}
	// Ensure status code survives a potential overwrite from the JSON body.
	apiErr.StatusCode = statusCode
	if apiErr.RequestID == "" {
		apiErr.RequestID = requestID
	}
	if apiErr.Code == "" {
		apiErr.Code = http.StatusText(statusCode)
	}
	if apiErr.Message == "" {
		apiErr.Message = fmt.Sprintf("HTTP %d", statusCode)
	}
	return apiErr
}

// backoff computes the wait duration for the given retry attempt using
// exponential back-off with full jitter.
//
//	base = retryWaitMin * 2^(attempt-1)
//	capped = min(base, retryWaitMax)
//	sleep  = random in [0, capped)
func (c *Client) backoff(attempt int) time.Duration {
	base := c.retryWaitMin
	for i := 1; i < attempt; i++ {
		base *= 2
		if base >= c.retryWaitMax {
			base = c.retryWaitMax
			break
		}
	}
	if base <= 0 {
		return c.retryWaitMin
	}
	// Full jitter: uniform random in [retryWaitMin, base].
	jitter := time.Duration(rand.Int63n(int64(base)-int64(c.retryWaitMin)+1)) + c.retryWaitMin
	return jitter
}

// isRetryableStatus returns true for HTTP status codes that warrant a retry.
func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusTooManyRequests:
		return true
	}
	return false
}

// isConnError performs a best-effort check for connection-level errors
// that are not already covered by net.Error.
func isConnError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	return false
}

//Personal.AI order the ending
