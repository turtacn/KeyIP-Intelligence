// Phase 13 - SDK Client Options (298/349)
// File: pkg/client/options.go
// Functional Options pattern for Client configuration.

package client

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Default constants
// ---------------------------------------------------------------------------

const (
	DefaultTimeout      = 30 * time.Second
	DefaultRetryMax     = 3
	DefaultRetryWaitMin = 500 * time.Millisecond
	DefaultRetryWaitMax = 5 * time.Second
	DefaultUserAgent    = "keyip-go-sdk/" + Version
	DefaultPageSize     = 20
	MaxRetryMax         = 10
	MaxPageSize         = 100
)

// ---------------------------------------------------------------------------
// Option type
// ---------------------------------------------------------------------------

// Option configures a Client. Options are applied in order during NewClient.
type Option func(*Client)

// ---------------------------------------------------------------------------
// Option constructors
// ---------------------------------------------------------------------------

// WithHTTPClient injects a custom *http.Client. Nil is ignored.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithLogger injects a custom Logger. Nil is ignored (keeps noopLogger).
func WithLogger(logger Logger) Option {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithUserAgent overrides the default User-Agent. Empty string is ignored.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if userAgent != "" {
			c.userAgent = userAgent
		}
	}
}

// WithRetryMax sets the maximum retry count. Clamped to [0, MaxRetryMax].
func WithRetryMax(retryMax int) Option {
	return func(c *Client) {
		if retryMax < 0 {
			retryMax = 0
		}
		if retryMax > MaxRetryMax {
			retryMax = MaxRetryMax
		}
		c.retryMax = retryMax
	}
}

// WithRetryWait sets the min/max backoff durations. Invalid values fall back
// to defaults; if min > max the two are swapped.
func WithRetryWait(min, max time.Duration) Option {
	return func(c *Client) {
		if min <= 0 {
			min = DefaultRetryWaitMin
		}
		if max <= 0 {
			max = DefaultRetryWaitMax
		}
		if min > max {
			min, max = max, min
		}
		c.retryWaitMin = min
		c.retryWaitMax = max
	}
}

// WithTimeout sets the HTTP client timeout. Values <= 0 are ignored.
// If combined with WithHTTPClient the last applied option wins.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout <= 0 {
			return
		}
		c.httpClient = &http.Client{
			Timeout: timeout,
		}
	}
}

// WithBaseHeaders sets extra headers sent with every request.
// Nil or empty maps are ignored. Protected headers (Authorization,
// Content-Type, Accept) are filtered out at request time, not here.
func WithBaseHeaders(headers map[string]string) Option {
	return func(c *Client) {
		if len(headers) == 0 {
			return
		}
		// Defensive copy so the caller cannot mutate after construction.
		cp := make(map[string]string, len(headers))
		for k, v := range headers {
			cp[k] = v
		}
		c.baseHeaders = cp
	}
}

// WithRateLimiter enables client-side rate limiting at the given requests per
// second. Values <= 0 are ignored (no limiting).
func WithRateLimiter(rps float64) Option {
	return func(c *Client) {
		if rps <= 0 {
			return
		}
		c.rateLimiter = newInternalRateLimiter(rps)
	}
}

// WithDebug enables verbose request/response logging.
func WithDebug(debug bool) Option {
	return func(c *Client) {
		c.debug = debug
	}
}

// ---------------------------------------------------------------------------
// internalRateLimiter — simple token-bucket without external deps
// ---------------------------------------------------------------------------

type internalRateLimiter struct {
	tokens chan struct{}
	quit   chan struct{}
	once   sync.Once
}

func newInternalRateLimiter(rps float64) *internalRateLimiter {
	if rps <= 0 {
		return nil
	}
	interval := time.Duration(float64(time.Second) / rps)
	if interval < time.Millisecond {
		interval = time.Millisecond
	}

	rl := &internalRateLimiter{
		tokens: make(chan struct{}, int(rps)+1),
		quit:   make(chan struct{}),
	}

	// Seed one token so the first call doesn't block.
	select {
	case rl.tokens <- struct{}{}:
	default:
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-rl.quit:
				return
			case <-ticker.C:
				select {
				case rl.tokens <- struct{}{}:
				default:
					// bucket full, discard
				}
			}
		}
	}()

	return rl
}

// Wait blocks until a token is available or ctx is done.
func (rl *internalRateLimiter) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case _, ok := <-rl.tokens:
		if !ok {
			return context.Canceled
		}
		return nil
	}
}

// Close stops the token-producing goroutine and drains the channel.
func (rl *internalRateLimiter) Close() {
	rl.once.Do(func() {
		close(rl.quit)
		// Drain remaining tokens so any blocked Wait returns.
		for {
			select {
			case <-rl.tokens:
			default:
				close(rl.tokens)
				return
			}
		}
	})
}

//Personal.AI order the ending
