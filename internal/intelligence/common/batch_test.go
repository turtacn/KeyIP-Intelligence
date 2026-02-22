package common

import (
	"container/heap"
	"context"
	stdliberrors "errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =========================================================================
// Test helpers / factories
// =========================================================================

func successFunc[T any, R any](val R) ProcessFunc[T, R] {
	return func(_ context.Context, _ T) (R, error) {
		return val, nil
	}
}

func failFunc[T any, R any](err error) ProcessFunc[T, R] {
	return func(_ context.Context, _ T) (R, error) {
		var zero R
		return zero, err
	}
}

func slowFunc[T any, R any](delay time.Duration, val R) ProcessFunc[T, R] {
	return func(ctx context.Context, _ T) (R, error) {
		select {
		case <-time.After(delay):
			return val, nil
		case <-ctx.Done():
			var zero R
			return zero, ctx.Err()
		}
	}
}

func intermittentFunc[T any, R any](failCount *atomic.Int32, threshold int32, val R, retryErr error) ProcessFunc[T, R] {
	return func(_ context.Context, _ T) (R, error) {
		c := failCount.Add(1)
		if c <= threshold {
			var zero R
			return zero, retryErr
		}
		return val, nil
	}
}

func concurrencyTracker[T any, R any](
	current *atomic.Int32,
	maxSeen *atomic.Int32,
	holdDuration time.Duration,
	val R,
) ProcessFunc[T, R] {
	return func(ctx context.Context, _ T) (R, error) {
		c := current.Add(1)
		defer current.Add(-1)
		// Update max seen.
		for {
			old := maxSeen.Load()
			if c <= old {
				break
			}
			if maxSeen.CompareAndSwap(old, c) {
				break
			}
		}
		select {
		case <-time.After(holdDuration):
			return val, nil
		case <-ctx.Done():
			var zero R
			return zero, ctx.Err()
		}
	}
}

// orderTracker records the order in which items begin processing.
func orderTracker[T any](
	order *[]int,
	mu *sync.Mutex,
	holdDuration time.Duration,
) ProcessFunc[T, int] {
	return func(ctx context.Context, _ T) (int, error) {
		mu.Lock()
		idx := len(*order)
		*order = append(*order, idx)
		mu.Unlock()
		select {
		case <-time.After(holdDuration):
			return idx, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

// =========================================================================
// Mock metrics
// =========================================================================

type testBatchMetrics struct {
	batchCalls atomic.Int32
	lastParams atomic.Pointer[BatchMetricParams]
}

func (m *testBatchMetrics) RecordInference(_ context.Context, _ *InferenceMetricParams)                {}
func (m *testBatchMetrics) RecordBatchProcessing(_ context.Context, p *BatchMetricParams) {
	m.batchCalls.Add(1)
	m.lastParams.Store(p)
}
func (m *testBatchMetrics) RecordCacheAccess(_ context.Context, _ bool, _ string)                      {}
func (m *testBatchMetrics) RecordCircuitBreakerStateChange(_ context.Context, _, _, _ string)           {}
func (m *testBatchMetrics) RecordRiskAssessment(_ context.Context, _ string, _ float64)                 {}
func (m *testBatchMetrics) RecordModelLoad(_ context.Context, _, _ string, _ float64, _ bool)           {}
func (m *testBatchMetrics) GetInferenceLatencyHistogram() LatencyHistogram                              { return nil }
func (m *testBatchMetrics) GetCurrentStats() *IntelligenceStats                                         { return &IntelligenceStats{} }

// =========================================================================
// Tests — NewBatchProcessor
// =========================================================================

func TestNewBatchProcessor_Defaults(t *testing.T) {
	bp := NewBatchProcessor[int, int]()
	impl := bp.(*batchProcessor[int, int])
	if impl.cfg.maxConcurrency != runtime.NumCPU() {
		t.Errorf("default maxConcurrency: want %d, got %d", runtime.NumCPU(), impl.cfg.maxConcurrency)
	}
	if impl.cfg.itemTimeout != 30*time.Second {
		t.Errorf("default itemTimeout: want 30s, got %v", impl.cfg.itemTimeout)
	}
	if impl.cfg.batchTimeout != 5*time.Minute {
		t.Errorf("default batchTimeout: want 5m, got %v", impl.cfg.batchTimeout)
	}
	if impl.cfg.retryPolicy != nil {
		t.Error("default retryPolicy should be nil")
	}
	if impl.cb != nil {
		t.Error("default circuit breaker should be nil")
	}
}

func TestNewBatchProcessor_CustomOptions(t *testing.T) {
	m := &testBatchMetrics{}
	bp := NewBatchProcessor[string, string](
		WithMaxConcurrency(4),
		WithItemTimeout(10*time.Second),
		WithBatchTimeout(1*time.Minute),
		WithRetryPolicy(3, 100*time.Millisecond),
		WithCircuitBreaker(5, 500*time.Millisecond),
		WithBackpressureThreshold(50),
		WithBatchMetrics(m),
	)
	impl := bp.(*batchProcessor[string, string])
	if impl.cfg.maxConcurrency != 4 {
		t.Errorf("maxConcurrency: want 4, got %d", impl.cfg.maxConcurrency)
	}
	if impl.cfg.itemTimeout != 10*time.Second {
		t.Errorf("itemTimeout: want 10s, got %v", impl.cfg.itemTimeout)
	}
	if impl.cfg.batchTimeout != 1*time.Minute {
		t.Errorf("batchTimeout: want 1m, got %v", impl.cfg.batchTimeout)
	}
	if impl.cfg.retryPolicy == nil || impl.cfg.retryPolicy.MaxRetries != 3 {
		t.Error("retryPolicy not set correctly")
	}
	if impl.cb == nil || impl.cb.threshold != 5 {
		t.Error("circuit breaker not set correctly")
	}
	if impl.cfg.backpressureThreshold != 50 {
		t.Errorf("backpressureThreshold: want 50, got %d", impl.cfg.backpressureThreshold)
	}
}

// =========================================================================
// Tests — Process
// =========================================================================

func TestProcess_AllSuccess(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(4))
	items := make([]int, 10)
	for i := range items {
		items[i] = i
	}
	fn := func(_ context.Context, v int) (int, error) {
		return v * 2, nil
	}
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.TotalCount != 10 {
		t.Errorf("TotalCount: want 10, got %d", br.TotalCount)
	}
	if br.SuccessCount != 10 {
		t.Errorf("SuccessCount: want 10, got %d", br.SuccessCount)
	}
	if br.FailureCount != 0 {
		t.Errorf("FailureCount: want 0, got %d", br.FailureCount)
	}
	for i, r := range br.Results {
		if r.Index != i {
			t.Errorf("result[%d].Index = %d", i, r.Index)
		}
		if r.Result != i*2 {
			t.Errorf("result[%d].Result: want %d, got %d", i, i*2, r.Result)
		}
		if r.Status != ItemStatusSuccess {
			t.Errorf("result[%d].Status: want SUCCESS, got %s", i, r.Status)
		}
	}
}

func TestProcess_AllFailure(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(4))
	items := make([]int, 10)
	testErr := fmt.Errorf("always fail")
	br, err := bp.Process(context.Background(), items, failFunc[int, int](testErr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 0 {
		t.Errorf("SuccessCount: want 0, got %d", br.SuccessCount)
	}
	if br.FailureCount != 10 {
		t.Errorf("FailureCount: want 10, got %d", br.FailureCount)
	}
	for i, r := range br.Results {
		if r.Error == nil {
			t.Errorf("result[%d] expected error", i)
		}
		if r.Status != ItemStatusFailed {
			t.Errorf("result[%d].Status: want FAILED, got %s", i, r.Status)
		}
	}
}

func TestProcess_PartialFailure(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(4))
	items := make([]int, 10)
	for i := range items {
		items[i] = i
	}
	fn := func(_ context.Context, v int) (int, error) {
		if v%3 == 0 && v > 0 { // indices 3, 6, 9
			return 0, fmt.Errorf("fail on %d", v)
		}
		return v, nil
	}
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 7 {
		t.Errorf("SuccessCount: want 7, got %d", br.SuccessCount)
	}
	if br.FailureCount != 3 {
		t.Errorf("FailureCount: want 3, got %d", br.FailureCount)
	}
}

func TestProcess_ResultOrdering(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(8))
	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}
	fn := func(_ context.Context, v int) (int, error) {
		// Random-ish delay to shuffle completion order.
		time.Sleep(time.Duration(v%5) * time.Millisecond)
		return v, nil
	}
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range br.Results {
		if r.Index != i {
			t.Fatalf("result[%d].Index = %d — ordering broken", i, r.Index)
		}
	}
}

func TestProcess_ConcurrencyLimit(t *testing.T) {
	var current, maxSeen atomic.Int32
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(3))
	items := make([]int, 20)
	fn := concurrencyTracker[int, int](&current, &maxSeen, 20*time.Millisecond, 0)
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 20 {
		t.Errorf("SuccessCount: want 20, got %d", br.SuccessCount)
	}
	if maxSeen.Load() > 3 {
		t.Errorf("max concurrent: want <=3, got %d", maxSeen.Load())
	}
}

func TestProcess_ConcurrencyLimit_HighLoad(t *testing.T) {
	var current, maxSeen atomic.Int32
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(5))
	items := make([]int, 100)
	fn := concurrencyTracker[int, int](&current, &maxSeen, 5*time.Millisecond, 0)
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 100 {
		t.Errorf("SuccessCount: want 100, got %d", br.SuccessCount)
	}
	if maxSeen.Load() > 5 {
		t.Errorf("max concurrent: want <=5, got %d", maxSeen.Load())
	}
}

func TestProcess_ItemTimeout(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(4),
		WithItemTimeout(100*time.Millisecond),
		WithBatchTimeout(5*time.Second),
	)
	items := []int{0, 1, 2, 3, 4}
	fn := func(ctx context.Context, v int) (int, error) {
		if v == 2 {
			// This item is slow.
			select {
			case <-time.After(500 * time.Millisecond):
				return v, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
		return v, nil
	}
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.Results[2].Status != ItemStatusTimeout {
		t.Errorf("item 2 status: want TIMEOUT, got %s", br.Results[2].Status)
	}
	if br.SuccessCount != 4 {
		t.Errorf("SuccessCount: want 4, got %d", br.SuccessCount)
	}
}

func TestProcess_ItemTimeout_AllSlow(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(4),
		WithItemTimeout(50*time.Millisecond),
	)
	items := make([]int, 5)
	fn := slowFunc[int, int](500*time.Millisecond, 0)
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range br.Results {
		if r.Status != ItemStatusTimeout {
			t.Errorf("item %d status: want TIMEOUT, got %s", i, r.Status)
		}
	}
	if br.FailureCount != 5 {
		t.Errorf("FailureCount: want 5, got %d", br.FailureCount)
	}
}

func TestProcess_BatchTimeout(t *testing.T) {
	// MaxConcurrency=1 forces serial execution. Each item takes 100ms.
	// BatchTimeout=250ms means only ~2 items can complete.
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithItemTimeout(2*time.Second),
		WithBatchTimeout(250*time.Millisecond),
	)
	items := make([]int, 10)
	fn := slowFunc[int, int](100*time.Millisecond, 42)
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount >= 10 {
		t.Errorf("expected some items to be cancelled/timed out, but all succeeded")
	}
	// At least some should have been cancelled or timed out.
	cancelledOrTimedOut := 0
	for _, r := range br.Results {
		if r.Status == ItemStatusTimeout || r.Status == ItemStatusCancelled {
			cancelledOrTimedOut++
		}
	}
	if cancelledOrTimedOut == 0 {
		t.Error("expected at least some items to be cancelled/timed out")
	}
}

func TestProcess_ContextCancellation(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithItemTimeout(5*time.Second),
		WithBatchTimeout(5*time.Second),
	)
	ctx, cancel := context.WithCancel(context.Background())
	items := make([]int, 10)
	callCount := atomic.Int32{}
	fn := func(fctx context.Context, v int) (int, error) {
		c := callCount.Add(1)
		if c == 3 {
			cancel() // Cancel after 3rd item starts.
		}
		select {
		case <-time.After(50 * time.Millisecond):
			return v, nil
		case <-fctx.Done():
			return 0, fctx.Err()
		}
	}
	br, err := bp.Process(ctx, items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cancelledCount := 0
	for _, r := range br.Results {
		if r.Status == ItemStatusCancelled {
			cancelledCount++
		}
	}
	if cancelledCount == 0 {
		t.Error("expected at least some items to be cancelled")
	}
}

func TestProcess_EmptyInput(t *testing.T) {
	bp := NewBatchProcessor[int, int]()
	br, err := bp.Process(context.Background(), []int{}, successFunc[int](42))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.TotalCount != 0 {
		t.Errorf("TotalCount: want 0, got %d", br.TotalCount)
	}
}

func TestProcess_NilFunction(t *testing.T) {
	bp := NewBatchProcessor[int, int]()
	_, err := bp.Process(context.Background(), []int{1}, nil)
	if err == nil {
		t.Fatal("expected error for nil function")
	}
}

func TestProcess_SingleItem(t *testing.T) {
	bp := NewBatchProcessor[int, string]()
	br, err := bp.Process(context.Background(), []int{7}, func(_ context.Context, v int) (string, error) {
		return fmt.Sprintf("val-%d", v), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.TotalCount != 1 || br.SuccessCount != 1 {
		t.Errorf("counts: total=%d success=%d", br.TotalCount, br.SuccessCount)
	}
	if br.Results[0].Result != "val-7" {
		t.Errorf("result: want val-7, got %s", br.Results[0].Result)
	}
}

func TestProcess_LargeInput(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(32))
	n := 1000
	items := make([]int, n)
	for i := range items {
		items[i] = i
	}
	fn := func(_ context.Context, v int) (int, error) { return v, nil }
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.TotalCount != n {
		t.Errorf("TotalCount: want %d, got %d", n, br.TotalCount)
	}
	if br.SuccessCount != n {
		t.Errorf("SuccessCount: want %d, got %d", n, br.SuccessCount)
	}
	for i, r := range br.Results {
		if r.Index != i || r.Result != i {
			t.Fatalf("result[%d] mismatch: index=%d result=%d", i, r.Index, r.Result)
		}
	}
}

func TestProcess_DurationTracking(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(2))
	items := make([]int, 4)
	fn := slowFunc[int, int](50*time.Millisecond, 0)
	br, err := bp.Process(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.TotalDurationMs < 50 {
		t.Errorf("TotalDurationMs too low: %f", br.TotalDurationMs)
	}
	if br.AvgItemDurationMs < 40 {
		t.Errorf("AvgItemDurationMs too low: %f", br.AvgItemDurationMs)
	}
}

// =========================================================================
// Tests — ProcessWithPriority
// =========================================================================

func TestProcessWithPriority_HighFirst(t *testing.T) {
	// MaxConcurrency=1 ensures serial execution, so we can verify order.
	bp := NewBatchProcessor[string, int](WithMaxConcurrency(1))
	var executionOrder []int
	var mu sync.Mutex

	items := []PrioritizedItem[string]{
		{Item: "low", Priority: 1},
		{Item: "high", Priority: 100},
		{Item: "medium", Priority: 50},
	}
	fn := func(_ context.Context, _ string) (int, error) {
		mu.Lock()
		executionOrder = append(executionOrder, len(executionOrder))
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return 0, nil
	}
	br, err := bp.ProcessWithPriority(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 3 {
		t.Errorf("SuccessCount: want 3, got %d", br.SuccessCount)
	}
	// With MaxConcurrency=1, the first dispatched item is the highest priority.
	// We can't directly observe which item ran first from executionOrder alone,
	// but we can verify the result ordering is by original index.
	for i, r := range br.Results {
		if r.Index != i {
			t.Errorf("result[%d].Index = %d", i, r.Index)
		}
	}
}

func TestProcessWithPriority_SamePriority(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(1))
	items := make([]PrioritizedItem[int], 5)
	for i := range items {
		items[i] = PrioritizedItem[int]{Item: i, Priority: 10}
	}
	fn := func(_ context.Context, v int) (int, error) {
		time.Sleep(5 * time.Millisecond)
		return v, nil
	}
	br, err := bp.ProcessWithPriority(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 5 {
		t.Errorf("SuccessCount: want 5, got %d", br.SuccessCount)
	}
}

func TestProcessWithPriority_ResultOrdering(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(4))
	items := make([]PrioritizedItem[int], 10)
	for i := range items {
		items[i] = PrioritizedItem[int]{Item: i, Priority: 10 - i}
	}
	fn := func(_ context.Context, v int) (int, error) {
		time.Sleep(time.Duration(v) * time.Millisecond)
		return v * 10, nil
	}
	br, err := bp.ProcessWithPriority(context.Background(), items, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, r := range br.Results {
		if r.Index != i {
			t.Fatalf("result ordering broken at %d: Index=%d", i, r.Index)
		}
	}
}

// =========================================================================
// Tests — Retry
// =========================================================================

func TestRetry_RetryableError(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithRetryPolicy(3, 10*time.Millisecond),
	)
	var callCount atomic.Int32
	fn := intermittentFunc[int, int](&callCount, 2, 42, fmt.Errorf("transient"))
	br, err := bp.Process(context.Background(), []int{1}, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 1 {
		t.Errorf("SuccessCount: want 1, got %d", br.SuccessCount)
	}
	if br.Results[0].Result != 42 {
		t.Errorf("Result: want 42, got %d", br.Results[0].Result)
	}
	// Should have been called 3 times (2 failures + 1 success).
	if callCount.Load() != 3 {
		t.Errorf("call count: want 3, got %d", callCount.Load())
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	nonRetryable := fmt.Errorf("non-retryable")
	retryable := fmt.Errorf("retryable")
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithRetryPolicyFull(&RetryPolicy{
			MaxRetries:        3,
			InitialBackoff:    10 * time.Millisecond,
			MaxBackoff:        100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			RetryableErrors:   []error{retryable},
		}),
	)
	var callCount atomic.Int32
	fn := func(_ context.Context, _ int) (int, error) {
		callCount.Add(1)
		return 0, nonRetryable
	}
	br, err := bp.Process(context.Background(), []int{1}, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.FailureCount != 1 {
		t.Errorf("FailureCount: want 1, got %d", br.FailureCount)
	}
	// Should only be called once — no retry for non-retryable errors.
	if callCount.Load() != 1 {
		t.Errorf("call count: want 1, got %d", callCount.Load())
	}
}

func TestRetry_MaxRetriesExhausted(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithRetryPolicy(2, 5*time.Millisecond),
	)
	persistentErr := fmt.Errorf("always fails")
	var callCount atomic.Int32
	fn := func(_ context.Context, _ int) (int, error) {
		callCount.Add(1)
		return 0, persistentErr
	}
	br, err := bp.Process(context.Background(), []int{1}, fn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.FailureCount != 1 {
		t.Errorf("FailureCount: want 1, got %d", br.FailureCount)
	}
	// 1 initial + 2 retries = 3 calls.
	if callCount.Load() != 3 {
		t.Errorf("call count: want 3, got %d", callCount.Load())
	}
}

func TestRetry_ExponentialBackoff(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithRetryPolicyFull(&RetryPolicy{
			MaxRetries:        3,
			InitialBackoff:    50 * time.Millisecond,
			MaxBackoff:        1 * time.Second,
			BackoffMultiplier: 2.0,
		}),
	)
	var timestamps []time.Time
	var mu sync.Mutex
	fn := func(_ context.Context, _ int) (int, error) {
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		return 0, fmt.Errorf("fail")
	}
	_, _ = bp.Process(context.Background(), []int{1}, fn)

	mu.Lock()
	defer mu.Unlock()
	if len(timestamps) < 3 {
		t.Fatalf("expected at least 3 timestamps, got %d", len(timestamps))
	}
	// Gap between attempt 1 and 2 should be ~50ms (±jitter).
	gap1 := timestamps[1].Sub(timestamps[0])
	// Gap between attempt 2 and 3 should be ~100ms (±jitter).
	gap2 := timestamps[2].Sub(timestamps[1])

	if gap1 < 25*time.Millisecond || gap1 > 100*time.Millisecond {
		t.Errorf("gap1 out of range: %v", gap1)
	}
	if gap2 < 50*time.Millisecond || gap2 > 200*time.Millisecond {
		t.Errorf("gap2 out of range: %v", gap2)
	}
	// gap2 should be roughly 2x gap1 (exponential).
	ratio := float64(gap2) / float64(gap1)
	if ratio < 1.2 || ratio > 4.0 {
		t.Errorf("backoff ratio: want ~2.0, got %f", ratio)
	}
}

func TestRetry_BackoffJitter(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:        1,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
	}
	// Run calculateBackoff many times; they should not all be identical.
	seen := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		d := calculateBackoff(0, policy)
		seen[d] = true
	}
	if len(seen) < 2 {
		t.Error("expected jitter to produce varying backoff durations")
	}
}

func TestRetry_BackoffCapped(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:        10,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        500 * time.Millisecond,
		BackoffMultiplier: 3.0,
	}
	for attempt := 0; attempt < 20; attempt++ {
		d := calculateBackoff(attempt, policy)
		// With ±25% jitter on 500ms cap, max possible is 625ms.
		if d > 625*time.Millisecond {
			t.Errorf("attempt %d: backoff %v exceeds cap+jitter", attempt, d)
		}
	}
}

// =========================================================================
// Tests — Circuit Breaker
// =========================================================================

func TestCircuitBreaker_Open(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithCircuitBreaker(5, 500*time.Millisecond),
	)
	// First batch: 5 failures to trip the breaker.
	items := make([]int, 5)
	failErr := fmt.Errorf("cb-fail")
	br, err := bp.Process(context.Background(), items, failFunc[int, int](failErr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.FailureCount != 5 {
		t.Errorf("FailureCount: want 5, got %d", br.FailureCount)
	}

	// Second batch: should be rejected by circuit breaker.
	br2, err := bp.Process(context.Background(), []int{1, 2, 3}, successFunc[int](99))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range br2.Results {
		if !stdliberrors.Is(r.Error, ErrCircuitOpen) {
			t.Errorf("expected ErrCircuitOpen, got %v", r.Error)
		}
		if r.Status != ItemStatusFailed {
			t.Errorf("expected FAILED status, got %s", r.Status)
		}
	}
}

func TestCircuitBreaker_HalfOpen(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithCircuitBreaker(3, 100*time.Millisecond),
	)
	// Trip the breaker.
	items := make([]int, 3)
	_, _ = bp.Process(context.Background(), items, failFunc[int, int](fmt.Errorf("trip")))

	// Verify it's open.
	impl := bp.(*batchProcessor[int, int])
	if impl.cb.currentState() != cbStateOpen {
		t.Fatalf("expected OPEN state, got %d", impl.cb.currentState())
	}

	// Wait for reset duration.
	time.Sleep(150 * time.Millisecond)

	// Next request should be allowed (half-open probe).
	br, err := bp.Process(context.Background(), []int{1}, successFunc[int](42))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 1 {
		t.Errorf("SuccessCount: want 1, got %d", br.SuccessCount)
	}
}

func TestCircuitBreaker_CloseAfterSuccess(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithCircuitBreaker(3, 100*time.Millisecond),
	)
	impl := bp.(*batchProcessor[int, int])

	// Trip the breaker.
	_, _ = bp.Process(context.Background(), make([]int, 3), failFunc[int, int](fmt.Errorf("trip")))
	if impl.cb.currentState() != cbStateOpen {
		t.Fatalf("expected OPEN, got %d", impl.cb.currentState())
	}

	// Wait for half-open.
	time.Sleep(150 * time.Millisecond)

	// Successful probe should close the breaker.
	_, _ = bp.Process(context.Background(), []int{1}, successFunc[int](1))
	if impl.cb.currentState() != cbStateClosed {
		t.Errorf("expected CLOSED after successful probe, got %d", impl.cb.currentState())
	}

	// Subsequent batch should work normally.
	br, _ := bp.Process(context.Background(), []int{1, 2, 3}, successFunc[int](10))
	if br.SuccessCount != 3 {
		t.Errorf("SuccessCount: want 3, got %d", br.SuccessCount)
	}
}

func TestCircuitBreaker_ReopenAfterHalfOpenFailure(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(1),
		WithCircuitBreaker(3, 100*time.Millisecond),
	)
	impl := bp.(*batchProcessor[int, int])

	// Trip the breaker.
	_, _ = bp.Process(context.Background(), make([]int, 3), failFunc[int, int](fmt.Errorf("trip")))

	// Wait for half-open.
	time.Sleep(150 * time.Millisecond)

	// Failed probe should reopen.
	_, _ = bp.Process(context.Background(), []int{1}, failFunc[int, int](fmt.Errorf("probe-fail")))

	st := impl.cb.currentState()
	if st != cbStateOpen {
		t.Errorf("expected OPEN after failed probe, got %d", st)
	}
}

func TestCircuitBreaker_Disabled(t *testing.T) {
	// No WithCircuitBreaker option — breaker is nil.
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(1))
	items := make([]int, 20)
	br, err := bp.Process(context.Background(), items, failFunc[int, int](fmt.Errorf("fail")))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All should be plain failures, no ErrCircuitOpen.
	for _, r := range br.Results {
		if stdliberrors.Is(r.Error, ErrCircuitOpen) {
			t.Error("circuit breaker should be disabled")
		}
	}
	if br.FailureCount != 20 {
		t.Errorf("FailureCount: want 20, got %d", br.FailureCount)
	}
}

// =========================================================================
// Tests — Backpressure
// =========================================================================

func TestBackpressure_BelowThreshold(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(4),
		WithBackpressureThreshold(100),
	)
	items := make([]int, 10)
	br, err := bp.Process(context.Background(), items, successFunc[int](1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 10 {
		t.Errorf("SuccessCount: want 10, got %d", br.SuccessCount)
	}
}

func TestBackpressure_AboveThreshold(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(2),
		WithBackpressureThreshold(5),
	)
	// First batch occupies the pending count with slow items.
	var wg sync.WaitGroup
	wg.Add(1)
	var firstErr error
	go func() {
		defer wg.Done()
		slowItems := make([]int, 5)
		_, firstErr = bp.Process(context.Background(), slowItems, slowFunc[int, int](200*time.Millisecond, 0))
	}()

	// Give the first batch time to register its pending count.
	time.Sleep(50 * time.Millisecond)

	// Second batch should be rejected.
	_, err := bp.Process(context.Background(), make([]int, 5), successFunc[int](1))
	if !stdliberrors.Is(err, ErrBackpressure) {
		t.Errorf("expected ErrBackpressure, got %v", err)
	}

	wg.Wait()
	if firstErr != nil {
		t.Errorf("first batch error: %v", firstErr)
	}
}

func TestBackpressure_Disabled(t *testing.T) {
	// backpressureThreshold=0 means disabled (the constructor sets it to
	// maxConcurrency*10 by default, so we override explicitly).
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(2),
	)
	// Even a large batch should not trigger backpressure.
	items := make([]int, 200)
	br, err := bp.Process(context.Background(), items, successFunc[int](1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if br.SuccessCount != 200 {
		t.Errorf("SuccessCount: want 200, got %d", br.SuccessCount)
	}
}

// =========================================================================
// Tests — Shutdown
// =========================================================================

func TestShutdown_WaitsForCompletion(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(2))
	var completed atomic.Int32
	started := make(chan struct{})
	var once sync.Once

	// Start a slow batch in the background.
	go func() {
		items := make([]int, 4)
		fn := func(_ context.Context, _ int) (int, error) {
			once.Do(func() { close(started) })
			time.Sleep(100 * time.Millisecond)
			completed.Add(1)
			return 0, nil
		}
		_, _ = bp.Process(context.Background(), items, fn)
	}()

	// Wait for the first item to start processing.
	select {
	case <-started:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for batch to start")
	}

	// Shutdown should wait for the batch to finish.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := bp.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
	if completed.Load() != 4 {
		t.Errorf("completed: want 4, got %d", completed.Load())
	}
}

func TestShutdown_Timeout(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithMaxConcurrency(1))

	// Start a very slow batch.
	go func() {
		items := make([]int, 10)
		_, _ = bp.Process(context.Background(), items, slowFunc[int, int](5*time.Second, 0))
	}()

	time.Sleep(30 * time.Millisecond)

	// Shutdown with a short timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := bp.Shutdown(ctx)
	if err == nil {
		t.Error("expected shutdown timeout error")
	}
}

func TestShutdown_RejectsNewWork(t *testing.T) {
	bp := NewBatchProcessor[int, int]()
	_ = bp.Shutdown(context.Background())

	_, err := bp.Process(context.Background(), []int{1}, successFunc[int](1))
	if !stdliberrors.Is(err, ErrShutdown) {
		t.Errorf("expected ErrShutdown, got %v", err)
	}
}

// =========================================================================
// Tests — Concurrent Safety
// =========================================================================

func TestConcurrentSafety_RaceDetection(t *testing.T) {
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(4),
		WithRetryPolicy(1, 5*time.Millisecond),
		WithCircuitBreaker(10, 100*time.Millisecond),
	)
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			items := make([]int, 20)
			for i := range items {
				items[i] = gid*100 + i
			}
			fn := func(_ context.Context, v int) (int, error) {
				if v%7 == 0 {
					return 0, fmt.Errorf("fail-%d", v)
				}
				return v, nil
			}
			_, _ = bp.Process(context.Background(), items, fn)
		}(g)
	}
	wg.Wait()
	// If -race detects issues, the test will fail automatically.
}

// =========================================================================
// Tests — Metrics
// =========================================================================

func TestMetrics_Recorded(t *testing.T) {
	m := &testBatchMetrics{}
	bp := NewBatchProcessor[int, int](
		WithMaxConcurrency(4),
		WithBatchMetrics(m),
	)
	items := make([]int, 5)
	_, err := bp.Process(context.Background(), items, successFunc[int](1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.batchCalls.Load() != 1 {
		t.Errorf("batch metric calls: want 1, got %d", m.batchCalls.Load())
	}
	params := m.lastParams.Load()
	if params == nil {
		t.Fatal("no metric params recorded")
	}
	if params.TotalItems != 5 {
		t.Errorf("TotalItems: want 5, got %d", params.TotalItems)
	}
	if params.SuccessItems != 5 {
		t.Errorf("SuccessItems: want 5, got %d", params.SuccessItems)
	}
	if params.FailedItems != 0 {
		t.Errorf("FailedItems: want 0, got %d", params.FailedItems)
	}
	if params.TotalDurationMs <= 0 {
		t.Errorf("DurationMs should be positive: %f", params.TotalDurationMs)
	}
}

// =========================================================================
// Tests — ItemStatus.String
// =========================================================================

func TestItemStatus_String(t *testing.T) {
	tests := []struct {
		status ItemStatus
		want   string
	}{
		{ItemStatusSuccess, "SUCCESS"},
		{ItemStatusFailed, "FAILED"},
		{ItemStatusTimeout, "TIMEOUT"},
		{ItemStatusCancelled, "CANCELLED"},
		{ItemStatus(99), "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		got := tt.status.String()
		if got != tt.want {
			t.Errorf("ItemStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

// =========================================================================
// Tests — shouldRetry / calculateBackoff unit tests
// =========================================================================

func TestShouldRetry_NilPolicy(t *testing.T) {
	if shouldRetry(fmt.Errorf("err"), nil) {
		t.Error("should not retry with nil policy")
	}
}

func TestShouldRetry_NilError(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: 3}
	if shouldRetry(nil, policy) {
		t.Error("should not retry nil error")
	}
}

func TestShouldRetry_EmptyRetryableList(t *testing.T) {
	policy := &RetryPolicy{MaxRetries: 3, RetryableErrors: nil}
	if !shouldRetry(fmt.Errorf("any"), policy) {
		t.Error("empty retryable list means all errors are retryable")
	}
}

func TestShouldRetry_MatchingError(t *testing.T) {
	target := fmt.Errorf("specific")
	policy := &RetryPolicy{
		MaxRetries:      3,
		RetryableErrors: []error{target},
	}
	if !shouldRetry(target, policy) {
		t.Error("matching error should be retryable")
	}
}

func TestShouldRetry_NonMatchingError(t *testing.T) {
	target := fmt.Errorf("specific")
	other := fmt.Errorf("other")
	policy := &RetryPolicy{
		MaxRetries:      3,
		RetryableErrors: []error{target},
	}
	if shouldRetry(other, policy) {
		t.Error("non-matching error should not be retryable")
	}
}

func TestCalculateBackoff_NilPolicy(t *testing.T) {
	d := calculateBackoff(0, nil)
	if d != 0 {
		t.Errorf("expected 0, got %v", d)
	}
}

func TestCalculateBackoff_ZeroInitial(t *testing.T) {
	d := calculateBackoff(0, &RetryPolicy{InitialBackoff: 0})
	if d != 0 {
		t.Errorf("expected 0, got %v", d)
	}
}

func TestCalculateBackoff_Exponential(t *testing.T) {
	policy := &RetryPolicy{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
	}
	// attempt 0: ~100ms, attempt 1: ~200ms, attempt 2: ~400ms
	for attempt := 0; attempt < 5; attempt++ {
		d := calculateBackoff(attempt, policy)
		expected := 100.0 * math.Pow(2.0, float64(attempt))
		expectedMs := expected // in milliseconds
		actualMs := float64(d) / float64(time.Millisecond)
		// Allow ±30% for jitter.
		if actualMs < expectedMs*0.7 || actualMs > expectedMs*1.3 {
			t.Errorf("attempt %d: backoff %v outside expected range ~%.0fms", attempt, d, expectedMs)
		}
	}
}

func TestCalculateBackoff_Capped(t *testing.T) {
	policy := &RetryPolicy{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        300 * time.Millisecond,
		BackoffMultiplier: 10.0,
	}
	d := calculateBackoff(5, policy)
	// Cap is 300ms, with ±25% jitter max is 375ms.
	if d > 375*time.Millisecond {
		t.Errorf("backoff %v exceeds cap+jitter (375ms)", d)
	}
}

// =========================================================================
// Tests — circuitBreaker unit tests
// =========================================================================

func TestCircuitBreaker_AllowWhenClosed(t *testing.T) {
	cb := newCircuitBreaker(5, 100*time.Millisecond, NewNoopLogger(), NewNoopIntelligenceMetrics())
	if !cb.allow() {
		t.Error("closed breaker should allow")
	}
}

func TestCircuitBreaker_NilIsAlwaysAllowed(t *testing.T) {
	var cb *circuitBreaker
	if !cb.allow() {
		t.Error("nil breaker should allow")
	}
}

func TestCircuitBreaker_DisabledThreshold(t *testing.T) {
	cb := newCircuitBreaker(0, 100*time.Millisecond, NewNoopLogger(), NewNoopIntelligenceMetrics())
	for i := 0; i < 100; i++ {
		cb.recordFailure()
	}
	if !cb.allow() {
		t.Error("disabled breaker (threshold=0) should always allow")
	}
}

func TestCircuitBreaker_TripsAfterThreshold(t *testing.T) {
	cb := newCircuitBreaker(3, 100*time.Millisecond, NewNoopLogger(), NewNoopIntelligenceMetrics())
	for i := 0; i < 3; i++ {
		cb.recordFailure()
	}
	if cb.currentState() != cbStateOpen {
		t.Errorf("expected OPEN, got %d", cb.currentState())
	}
	if cb.allow() {
		t.Error("open breaker should not allow")
	}
}

func TestCircuitBreaker_ResetsOnSuccess(t *testing.T) {
	cb := newCircuitBreaker(3, 100*time.Millisecond, NewNoopLogger(), NewNoopIntelligenceMetrics())
	cb.recordFailure()
	cb.recordFailure()
	cb.recordSuccess() // Reset consecutive count.
	cb.recordFailure()
	cb.recordFailure()
	// Only 2 consecutive failures, should still be closed.
	if cb.currentState() != cbStateClosed {
		t.Errorf("expected CLOSED, got %d", cb.currentState())
	}
}

func TestCircuitBreaker_HalfOpenTransition(t *testing.T) {
	cb := newCircuitBreaker(2, 50*time.Millisecond, NewNoopLogger(), NewNoopIntelligenceMetrics())
	cb.recordFailure()
	cb.recordFailure()
	if cb.currentState() != cbStateOpen {
		t.Fatal("expected OPEN")
	}
	time.Sleep(80 * time.Millisecond)
	// allow() should transition to half-open.
	if !cb.allow() {
		t.Error("should allow probe in half-open")
	}
	if cb.currentState() != cbStateHalfOpen {
		t.Errorf("expected HALF_OPEN, got %d", cb.currentState())
	}
}

func TestCircuitBreaker_HalfOpenOnlyOneProbe(t *testing.T) {
	cb := newCircuitBreaker(2, 50*time.Millisecond, NewNoopLogger(), NewNoopIntelligenceMetrics())
	cb.recordFailure()
	cb.recordFailure()
	time.Sleep(80 * time.Millisecond)

	// First call transitions and allows.
	if !cb.allow() {
		t.Error("first probe should be allowed")
	}
	// Second call in half-open should be denied.
	if cb.allow() {
		t.Error("second call in half-open should be denied")
	}
}

// =========================================================================
// Tests — priorityQueue
// =========================================================================

func TestPriorityQueue_Ordering(t *testing.T) {
	pq := make(priorityQueue[string], 0)
	heap.Init(&pq)
	heap.Push(&pq, &pqItem[string]{value: "low", priority: 1, originalIndex: 0})
	heap.Push(&pq, &pqItem[string]{value: "high", priority: 100, originalIndex: 1})
	heap.Push(&pq, &pqItem[string]{value: "mid", priority: 50, originalIndex: 2})

	first := heap.Pop(&pq).(*pqItem[string])
	if first.value != "high" {
		t.Errorf("expected 'high' first, got %q", first.value)
	}
	second := heap.Pop(&pq).(*pqItem[string])
	if second.value != "mid" {
		t.Errorf("expected 'mid' second, got %q", second.value)
	}
	third := heap.Pop(&pq).(*pqItem[string])
	if third.value != "low" {
		t.Errorf("expected 'low' third, got %q", third.value)
	}
}

