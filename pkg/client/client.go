package client

import (
"bytes"
"context"
"encoding/json"
"fmt"
"io"
"math/rand"
"net/http"
"net/url"
"strconv"
"strings"
"sync"
"time"

"github.com/google/uuid"
"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

const Version = "0.1.0"

// Logger defines the logging interface used by the Client
type Logger interface {
Debugf(format string, args ...interface{})
Infof(format string, args ...interface{})
Errorf(format string, args ...interface{})
}

// noopLogger is a no-op implementation of Logger
type noopLogger struct{}

func (noopLogger) Debugf(format string, args ...interface{}) {}
func (noopLogger) Infof(format string, args ...interface{})  {}
func (noopLogger) Errorf(format string, args ...interface{}) {}

// Client is the KeyIP-Intelligence SDK client
type Client struct {
baseURL       string
httpClient    *http.Client
apiKey        string
userAgent     string
logger        Logger
retryMax      int
retryWaitMin  time.Duration
retryWaitMax  time.Duration

molecules     *MoleculesClient
moleculesOnce sync.Once
patents       *PatentsClient
patentsOnce   sync.Once
lifecycle     *LifecycleClient
lifecycleOnce sync.Once
}

// APIError represents an error response from the API
type APIError struct {
StatusCode int    `json:"status_code"`
Code       string `json:"code"`
Message    string `json:"message"`
RequestID  string `json:"request_id"`
}

func (e *APIError) Error() string {
return fmt.Sprintf("keyip: %s (HTTP %d): %s [request_id=%s]", e.Code, e.StatusCode, e.Message, e.RequestID)
}

func (e *APIError) IsNotFound() bool {
return e.StatusCode == http.StatusNotFound
}

func (e *APIError) IsUnauthorized() bool {
return e.StatusCode == http.StatusUnauthorized
}

func (e *APIError) IsRateLimited() bool {
return e.StatusCode == http.StatusTooManyRequests
}

func (e *APIError) IsServerError() bool {
return e.StatusCode >= 500 && e.StatusCode < 600
}

// APIResponse is a generic response wrapper
type APIResponse[T any] struct {
Data T             `json:"data"`
Meta *ResponseMeta `json:"meta,omitempty"`
}

// ResponseMeta contains pagination metadata
type ResponseMeta struct {
Total    int64 `json:"total"`
Page     int   `json:"page"`
PageSize int   `json:"page_size"`
HasMore  bool  `json:"has_more"`
}

// NewClient creates a new KeyIP-Intelligence SDK client
func NewClient(baseURL string, apiKey string, opts ...Option) (*Client, error) {
if baseURL == "" {
return nil, errors.ErrInvalidConfig
}
if apiKey == "" {
return nil, errors.ErrInvalidConfig
}

// Validate baseURL
parsedURL, err := url.Parse(baseURL)
if err != nil {
return nil, fmt.Errorf("%w: invalid baseURL: %v", errors.ErrInvalidConfig, err)
}
if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
return nil, fmt.Errorf("%w: baseURL scheme must be http or https", errors.ErrInvalidConfig)
}

// Trim trailing slash
baseURL = strings.TrimSuffix(baseURL, "/")

c := &Client{
baseURL:      baseURL,
apiKey:       apiKey,
httpClient:   &http.Client{Timeout: 30 * time.Second},
userAgent:    fmt.Sprintf("keyip-go-sdk/%s", Version),
logger:       &noopLogger{},
retryMax:     3,
retryWaitMin: 500 * time.Millisecond,
retryWaitMax: 5 * time.Second,
}

// Apply options
for _, opt := range opts {
opt(c)
}

return c, nil
}

// Molecules returns the molecules sub-client (lazy initialization, thread-safe)
func (c *Client) Molecules() *MoleculesClient {
c.moleculesOnce.Do(func() {
c.molecules = &MoleculesClient{client: c}
})
return c.molecules
}

// Patents returns the patents sub-client (lazy initialization, thread-safe)
func (c *Client) Patents() *PatentsClient {
c.patentsOnce.Do(func() {
c.patents = &PatentsClient{client: c}
})
return c.patents
}

// Lifecycle returns the lifecycle sub-client (lazy initialization, thread-safe)
func (c *Client) Lifecycle() *LifecycleClient {
c.lifecycleOnce.Do(func() {
c.lifecycle = &LifecycleClient{client: c}
})
return c.lifecycle
}

// do performs an HTTP request with retry logic
func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
// Ensure path starts with /
if !strings.HasPrefix(path, "/") {
path = "/" + path
}

fullURL := c.baseURL + path

var bodyReader io.Reader
if body != nil {
bodyBytes, err := json.Marshal(body)
if err != nil {
return fmt.Errorf("failed to marshal request body: %w", err)
}
bodyReader = bytes.NewReader(bodyBytes)
}

var lastErr error
for attempt := 0; attempt <= c.retryMax; attempt++ {
if attempt > 0 {
// Calculate backoff with jitter
backoff := c.calculateBackoff(attempt)
c.logger.Debugf("Retry attempt %d after %v", attempt, backoff)

select {
case <-time.After(backoff):
case <-ctx.Done():
return ctx.Err()
}

// Reset body reader for retry
if body != nil {
bodyBytes, _ := json.Marshal(body)
bodyReader = bytes.NewReader(bodyBytes)
}
}

req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
if err != nil {
return fmt.Errorf("failed to create request: %w", err)
}

// Set headers
requestID := uuid.New().String()
req.Header.Set("Authorization", "Bearer "+c.apiKey)
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json")
req.Header.Set("User-Agent", c.userAgent)
req.Header.Set("X-Request-ID", requestID)

start := time.Now()
resp, err := c.httpClient.Do(req)
duration := time.Since(start)

if err != nil {
c.logger.Errorf("Request failed: %v", err)
lastErr = err
if c.shouldRetry(nil, err) {
continue
}
return err
}

c.logger.Debugf("%s %s %d (%v)", method, path, resp.StatusCode, duration)

respBody, err := io.ReadAll(resp.Body)
resp.Body.Close()
if err != nil {
return fmt.Errorf("failed to read response body: %w", err)
}

// Handle rate limiting
if resp.StatusCode == http.StatusTooManyRequests {
retryAfter := resp.Header.Get("Retry-After")
if retryAfter != "" {
if seconds, err := strconv.Atoi(retryAfter); err == nil && attempt < c.retryMax {
c.logger.Infof("Rate limited, retrying after %d seconds", seconds)
select {
case <-time.After(time.Duration(seconds) * time.Second):
continue
case <-ctx.Done():
return ctx.Err()
}
}
}
}

// Handle HTTP errors
if resp.StatusCode >= 400 {
apiErr := &APIError{
StatusCode: resp.StatusCode,
RequestID:  requestID,
}

if len(respBody) > 0 {
var errResp struct {
Code    string `json:"code"`
Message string `json:"message"`
}
if err := json.Unmarshal(respBody, &errResp); err == nil {
apiErr.Code = errResp.Code
apiErr.Message = errResp.Message
} else {
apiErr.Message = string(respBody)
}
}

lastErr = apiErr
if c.shouldRetry(resp, nil) {
continue
}
return apiErr
}

// Success - parse response
if result != nil && len(respBody) > 0 {
if err := json.Unmarshal(respBody, result); err != nil {
return fmt.Errorf("failed to unmarshal response: %w", err)
}
}

return nil
}

return lastErr
}

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

func (c *Client) shouldRetry(resp *http.Response, err error) bool {
// Retry on network errors
if err != nil {
return true
}

// Retry on 5xx errors
if resp != nil && resp.StatusCode >= 500 && resp.StatusCode < 600 {
return true
}

// Do not retry 4xx (except 429 which is handled separately)
return false
}

func (c *Client) calculateBackoff(attempt int) time.Duration {
// Exponential backoff with jitter
backoff := c.retryWaitMin * time.Duration(1<<uint(attempt-1))
if backoff > c.retryWaitMax {
backoff = c.retryWaitMax
}

// Add jitter (0-25% of backoff)
jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
return backoff + jitter
}

//Personal.AI order the ending
