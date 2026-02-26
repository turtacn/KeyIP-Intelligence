// Phase 13 - SDK Client Options Tests (299/349)
// File: pkg/client/options_test.go
// Unit tests for client options and rate limiter.

package client

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOption_WithHTTPClient(t *testing.T) {
	custom := &http.Client{Timeout: 99 * time.Second}
	c := &Client{}
	WithHTTPClient(custom)(c)
	assert.Equal(t, custom, c.httpClient)

	WithHTTPClient(nil)(c)
	assert.Equal(t, custom, c.httpClient) // Should not overwrite with nil
}

func TestOption_WithLogger(t *testing.T) {
	l := &testLogger{}
	c := &Client{}
	WithLogger(l)(c)
	assert.Equal(t, l, c.logger)

	WithLogger(nil)(c)
	assert.Equal(t, l, c.logger)
}

func TestOption_WithUserAgent(t *testing.T) {
	c := &Client{}
	WithUserAgent("my-agent")(c)
	assert.Equal(t, "my-agent", c.userAgent)

	WithUserAgent("")(c)
	assert.Equal(t, "my-agent", c.userAgent)
}

func TestOption_WithRetryMax(t *testing.T) {
	c := &Client{}
	WithRetryMax(5)(c)
	assert.Equal(t, 5, c.retryMax)

	WithRetryMax(-1)(c)
	assert.Equal(t, 0, c.retryMax)

	WithRetryMax(100)(c)
	assert.Equal(t, 10, c.retryMax)
}

func TestOption_WithRetryWait(t *testing.T) {
	c := &Client{}
	WithRetryWait(1*time.Second, 2*time.Second)(c)
	assert.Equal(t, 1*time.Second, c.retryWaitMin)
	assert.Equal(t, 2*time.Second, c.retryWaitMax)

	WithRetryWait(0, 0)(c)
	assert.Equal(t, DefaultRetryWaitMin, c.retryWaitMin)
	assert.Equal(t, DefaultRetryWaitMax, c.retryWaitMax)

	WithRetryWait(5*time.Second, 1*time.Second)(c) // Swap
	assert.Equal(t, 1*time.Second, c.retryWaitMin)
	assert.Equal(t, 5*time.Second, c.retryWaitMax)
}

func TestOption_WithTimeout(t *testing.T) {
	c := &Client{}
	WithTimeout(5 * time.Second)(c)
	assert.NotNil(t, c.httpClient)
	assert.Equal(t, 5*time.Second, c.httpClient.Timeout)

	WithTimeout(0)(c)
	assert.Equal(t, 5*time.Second, c.httpClient.Timeout)
}

func TestOption_WithBaseHeaders(t *testing.T) {
	c := &Client{}
	h := map[string]string{"X-Test": "Val"}
	WithBaseHeaders(h)(c)
	assert.Equal(t, h, c.baseHeaders)

	h2 := map[string]string{"X-Test-2": "Val2"}
	WithBaseHeaders(h2)(c) // Overwrite
	assert.Equal(t, h2, c.baseHeaders)

	WithBaseHeaders(nil)(c)
	assert.Equal(t, h2, c.baseHeaders)
}

func TestOption_WithRateLimiter(t *testing.T) {
	c := &Client{}
	WithRateLimiter(10)(c)
	assert.NotNil(t, c.rateLimiter)

	c.rateLimiter.Close()

	c2 := &Client{}
	WithRateLimiter(0)(c2)
	assert.Nil(t, c2.rateLimiter)
}

func TestOption_WithDebug(t *testing.T) {
	c := &Client{}
	WithDebug(true)(c)
	assert.True(t, c.debug)
}

// ---------------------------------------------------------------------------
// Internal Rate Limiter Tests
// ---------------------------------------------------------------------------

func TestInternalRateLimiter_Lifecycle(t *testing.T) {
	rl := newInternalRateLimiter(100) // 100 rps -> 10ms interval
	require.NotNil(t, rl)
	defer rl.Close()

	ctx := context.Background()
	// Should allow immediately (seeded token)
	err := rl.Wait(ctx)
	assert.NoError(t, err)

	// Should allow again quickly (ticker)
	time.Sleep(20 * time.Millisecond)
	err = rl.Wait(ctx)
	assert.NoError(t, err)
}

func TestInternalRateLimiter_Wait(t *testing.T) {
	rl := newInternalRateLimiter(1) // 1 rps
	defer rl.Close()

	ctx := context.Background()

	start := time.Now()
	// 1st token (immediate)
	assert.NoError(t, rl.Wait(ctx))
	// 2nd token (wait ~1s)
	assert.NoError(t, rl.Wait(ctx))
	elapsed := time.Since(start)

	assert.True(t, elapsed >= 900*time.Millisecond, "Should wait ~1s for 2nd token")
}

func TestInternalRateLimiter_ContextCanceled(t *testing.T) {
	rl := newInternalRateLimiter(0.1) // 10s per token
	defer rl.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Consume initial token
	rl.Wait(context.Background())

	cancel()
	err := rl.Wait(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestInternalRateLimiter_Close(t *testing.T) {
	rl := newInternalRateLimiter(10)
	// Consume initial token
	rl.Wait(context.Background())

	// Start a waiter
	done := make(chan error)
	go func() {
		done <- rl.Wait(context.Background())
	}()

	// Close limiter
	rl.Close()

	// Waiter should unblock with error (context canceled or closed channel behavior)
	err := <-done
	assert.Error(t, err) // Expect error when closed
}

func TestInternalRateLimiter_Concurrent(t *testing.T) {
	rl := newInternalRateLimiter(1000)
	defer rl.Close()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Wait(context.Background())
		}()
	}
	wg.Wait()
}

//Personal.AI order the ending
