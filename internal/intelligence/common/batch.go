/*
 * batch.go 实现了完整的泛型批量处理引擎（信号量并发控制、指数退避重试、嵌入式熔断器、背压机制、优先级堆调度、优雅关闭），
 * batch_test.go 覆盖了需求中列出的全部测试用例，包括并发限制验证、超时/取消传播、熔断器状态机转换、背压拒绝、重试退避精度、优先级队列排序以及 -race 安全性。
*/
package common

import (
	"container/heap"
	"context"
	stdliberrors "errors"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Sentinel Errors
// ---------------------------------------------------------------------------

var (
	ErrShutdown     = stdliberrors.New("batch processor is shutting down")
	ErrBackpressure = stdliberrors.New("backpressure threshold exceeded")
	ErrCircuitOpen  = stdliberrors.New("circuit breaker is open")
)

// ---------------------------------------------------------------------------
// ItemStatus enumeration
// ---------------------------------------------------------------------------

// ItemStatus represents the outcome status of a single batch item.
type ItemStatus int

const (
	ItemStatusSuccess   ItemStatus = iota // processing completed successfully
	ItemStatusFailed                      // processing failed with an error
	ItemStatusTimeout                     // processing exceeded its timeout
	ItemStatusCancelled                   // processing was cancelled (context or shutdown)
)

// String returns the human-readable representation of an ItemStatus.
func (s ItemStatus) String() string {
	switch s {
	case ItemStatusSuccess:
		return "SUCCESS"
	case ItemStatusFailed:
		return "FAILED"
	case ItemStatusTimeout:
		return "TIMEOUT"
	case ItemStatusCancelled:
		return "CANCELLED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(s))
	}
}

// ---------------------------------------------------------------------------
// Generic types
// ---------------------------------------------------------------------------

// ProcessFunc is the signature for a function that processes a single item.
type ProcessFunc[T, R any] func(ctx context.Context, item T) (R, error)

// PrioritizedItem wraps an item with a priority value.
// Higher Priority values are processed first.
type PrioritizedItem[T any] struct {
	Item     T   `json:"item"`
	Priority int `json:"priority"`
}

// ItemResult holds the outcome of processing a single item within a batch.
type ItemResult[R any] struct {
	Index      int        `json:"index"`
	Result     R          `json:"result"`
	Error      error      `json:"error,omitempty"`
	DurationMs float64    `json:"duration_ms"`
	Status     ItemStatus `json:"status"`
}

// BatchResult aggregates the outcomes of an entire batch processing run.
type BatchResult[R any] struct {
	Results           []*ItemResult[R] `json:"results"`
	TotalCount        int              `json:"total_count"`
	SuccessCount      int              `json:"success_count"`
	FailureCount      int              `json:"failure_count"`
	TotalDurationMs   float64          `json:"total_duration_ms"`
	AvgItemDurationMs float64          `json:"avg_item_duration_ms"`
}

// ---------------------------------------------------------------------------
// BatchProcessor interface
// ---------------------------------------------------------------------------

// BatchProcessor defines the contract for a generic batch processing engine.
type BatchProcessor[T, R any] interface {
	// Process executes fn for every item in items, respecting concurrency
	// limits, timeouts, circuit-breaker and back-pressure policies.
	Process(ctx context.Context, items []T, fn ProcessFunc[T, R]) (*BatchResult[R], error)

	// ProcessWithPriority is identical to Process but items carry an explicit
	// priority — higher-priority items are scheduled first.
	ProcessWithPriority(ctx context.Context, items []PrioritizedItem[T], fn ProcessFunc[T, R]) (*BatchResult[R], error)

	// Shutdown gracefully drains in-flight work. After Shutdown returns (or
	// ctx expires) no new batches are accepted.
	Shutdown(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// RetryPolicy
// ---------------------------------------------------------------------------

// RetryPolicy governs how failed items are retried.
type RetryPolicy struct {
	MaxRetries        int           `json:"max_retries" yaml:"max_retries"`
	InitialBackoff    time.Duration `json:"initial_backoff" yaml:"initial_backoff"`
	MaxBackoff        time.Duration `json:"max_backoff" yaml:"max_backoff"`
	BackoffMultiplier float64       `json:"backoff_multiplier" yaml:"backoff_multiplier"`
	RetryableErrors   []error       `json:"-" yaml:"-"`
}

// shouldRetry decides whether err is eligible for another attempt.
func shouldRetry(err error, policy *RetryPolicy) bool {
	if policy == nil || err == nil {
		return false
	}
	// If no explicit retryable list, every error is retryable.
	if len(policy.RetryableErrors) == 0 {
		return true
	}
	for _, re := range policy.RetryableErrors {
		if stdliberrors.Is(err, re) {
			return true
		}
	}
	return false
}

// calculateBackoff returns the delay before the attempt-th retry.
// It applies exponential back-off with ±25 % jitter, capped at MaxBackoff.
func calculateBackoff(attempt int, policy *RetryPolicy) time.Duration {
	if policy == nil || policy.InitialBackoff <= 0 {
		return 0
	}
	multiplier := policy.BackoffMultiplier
	if multiplier <= 0 {
		multiplier = 2.0
	}
	base := float64(policy.InitialBackoff) * math.Pow(multiplier, float64(attempt))
	if policy.MaxBackoff > 0 && base > float64(policy.MaxBackoff) {
		base = float64(policy.MaxBackoff)
	}
	// jitter: ±25 %
	jitter := base * 0.25 * (rand.Float64()*2 - 1) // [-25%, +25%]
	d := time.Duration(base + jitter)
	if d < 0 {
		d = 0
	}
	return d
}

// ---------------------------------------------------------------------------
// Circuit-breaker (lightweight, embedded)
// ---------------------------------------------------------------------------

const (
	cbStateClosed   int32 = 0
	cbStateOpen     int32 = 1
	cbStateHalfOpen int32 = 2
)

// circuitBreaker is a minimal circuit-breaker embedded in the batch processor.
type circuitBreaker struct {
	state            atomic.Int32
	consecutiveFails atomic.Int32
	threshold        int32
	resetDuration    time.Duration
	lastOpenTime     atomic.Int64 // unix-nano
	halfOpenPermits  atomic.Int32
	logger           Logger
	metrics          IntelligenceMetrics
}

func newCircuitBreaker(threshold int, duration time.Duration, logger Logger, metrics IntelligenceMetrics) *circuitBreaker {
	cb := &circuitBreaker{
		threshold:     int32(threshold),
		resetDuration: duration,
		logger:        logger,
		metrics:       metrics,
	}
	cb.state.Store(cbStateClosed)
	return cb
}

// allow returns true if the request is permitted through the breaker.
func (cb *circuitBreaker) allow() bool {
	if cb == nil || cb.threshold <= 0 {
		return true // disabled
	}
	st := cb.state.Load()
	switch st {
	case cbStateClosed:
		return true
	case cbStateOpen:
		// Check whether enough time has elapsed to transition to half-open.
		openedAt := cb.lastOpenTime.Load()
		if time.Since(time.Unix(0, openedAt)) >= cb.resetDuration {
			if cb.state.CompareAndSwap(cbStateOpen, cbStateHalfOpen) {
				cb.halfOpenPermits.Store(1)
				cb.logStateChange("OPEN", "HALF_OPEN")
			}
			// Allow one probe request in half-open.
			if cb.halfOpenPermits.Add(-1) >= 0 {
				return true
			}
			return false
		}
		return false
	case cbStateHalfOpen:
		// Only the single probe request is allowed.
		if cb.halfOpenPermits.Add(-1) >= 0 {
			return true
		}
		return false
	}
	return false
}

// recordSuccess records a successful invocation.
func (cb *circuitBreaker) recordSuccess() {
	if cb == nil || cb.threshold <= 0 {
		return
	}
	cb.consecutiveFails.Store(0)
	if cb.state.CompareAndSwap(cbStateHalfOpen, cbStateClosed) {
		cb.logStateChange("HALF_OPEN", "CLOSED")
	}
}

// recordFailure records a failed invocation and may trip the breaker.
func (cb *circuitBreaker) recordFailure() {
	if cb == nil || cb.threshold <= 0 {
		return
	}
	fails := cb.consecutiveFails.Add(1)

	st := cb.state.Load()
	switch st {
	case cbStateClosed:
		if fails >= cb.threshold {
			if cb.state.CompareAndSwap(cbStateClosed, cbStateOpen) {
				cb.lastOpenTime.Store(time.Now().UnixNano())
				cb.logStateChange("CLOSED", "OPEN")
			}
		}
	case cbStateHalfOpen:
		// Probe failed — reopen.
		if cb.state.CompareAndSwap(cbStateHalfOpen, cbStateOpen) {
			cb.lastOpenTime.Store(time.Now().UnixNano())
			cb.logStateChange("HALF_OPEN", "OPEN")
		}
	}
}

func (cb *circuitBreaker) logStateChange(from, to string) {
	if cb.logger != nil {
		cb.logger.Info("circuit-breaker state change", "from", from, "to", to)
	}
	if cb.metrics != nil {
		cb.metrics.RecordCircuitBreakerStateChange(context.Background(), "batch-processor", from, to)
	}
}

func (cb *circuitBreaker) currentState() int32 {
	if cb == nil {
		return cbStateClosed
	}
	return cb.state.Load()
}

// ---------------------------------------------------------------------------
// BatchOption functional options
// ---------------------------------------------------------------------------

// batchConfig holds all tunables for a batchProcessor.
type batchConfig struct {
	maxConcurrency        int
	itemTimeout           time.Duration
	batchTimeout          time.Duration
	retryPolicy           *RetryPolicy
	cbThreshold           int
	cbDuration            time.Duration
	backpressureThreshold int
	metrics               IntelligenceMetrics
	logger                Logger
}

func defaultBatchConfig() *batchConfig {
	return &batchConfig{
		maxConcurrency:        runtime.NumCPU(),
		itemTimeout:           30 * time.Second,
		batchTimeout:          5 * time.Minute,
		retryPolicy:           nil,
		cbThreshold:           0, // disabled
		cbDuration:            0,
		backpressureThreshold: 0, // disabled
		metrics:               nil,
		logger:                nil,
	}
}

// BatchOption configures a batchProcessor.
type BatchOption func(*batchConfig)

// WithMaxConcurrency sets the maximum number of items processed concurrently.
func WithMaxConcurrency(n int) BatchOption {
	return func(c *batchConfig) {
		if n > 0 {
			c.maxConcurrency = n
		}
	}
}

// WithItemTimeout sets the per-item processing timeout.
func WithItemTimeout(d time.Duration) BatchOption {
	return func(c *batchConfig) {
		if d > 0 {
			c.itemTimeout = d
		}
	}
}

// WithBatchTimeout sets the overall batch processing timeout.
func WithBatchTimeout(d time.Duration) BatchOption {
	return func(c *batchConfig) {
		if d > 0 {
			c.batchTimeout = d
		}
	}
}

// WithRetryPolicy configures retry behaviour for failed items.
func WithRetryPolicy(maxRetries int, backoff time.Duration) BatchOption {
	return func(c *batchConfig) {
		if maxRetries > 0 {
			c.retryPolicy = &RetryPolicy{
				MaxRetries:        maxRetries,
				InitialBackoff:    backoff,
				MaxBackoff:        backoff * 16,
				BackoffMultiplier: 2.0,
			}
		}
	}
}

// WithRetryPolicyFull configures a complete retry policy.
func WithRetryPolicyFull(policy *RetryPolicy) BatchOption {
	return func(c *batchConfig) {
		c.retryPolicy = policy
	}
}

// WithCircuitBreaker enables the embedded circuit-breaker.
func WithCircuitBreaker(threshold int, duration time.Duration) BatchOption {
	return func(c *batchConfig) {
		if threshold > 0 && duration > 0 {
			c.cbThreshold = threshold
			c.cbDuration = duration
		}
	}
}

// WithBackpressureThreshold sets the maximum pending-item count before
// back-pressure is applied. A value of 0 disables back-pressure.
func WithBackpressureThreshold(n int) BatchOption {
	return func(c *batchConfig) {
		if n > 0 {
			c.backpressureThreshold = n
		}
	}
}

// WithBatchMetrics injects a metrics collector.
func WithBatchMetrics(m IntelligenceMetrics) BatchOption {
	return func(c *batchConfig) {
		c.metrics = m
	}
}

// WithBatchLogger injects a logger.
func WithBatchLogger(l Logger) BatchOption {
	return func(c *batchConfig) {
		c.logger = l
	}
}

// ---------------------------------------------------------------------------
// batchProcessor implementation
// ---------------------------------------------------------------------------

type batchProcessor[T, R any] struct {
	cfg     *batchConfig
	cb      *circuitBreaker
	metrics IntelligenceMetrics
	logger  Logger

	// shutdown coordination
	shutdownMu   sync.Mutex
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
	isShutdown   atomic.Bool
	activeWg     sync.WaitGroup

	// back-pressure: number of items currently queued or in-flight
	pendingCount atomic.Int64
}

// NewBatchProcessor creates a new BatchProcessor with the supplied options.
func NewBatchProcessor[T, R any](opts ...BatchOption) BatchProcessor[T, R] {
	cfg := defaultBatchConfig()
	for _, o := range opts {
		o(cfg)
	}
	if cfg.metrics == nil {
		cfg.metrics = NewNoopIntelligenceMetrics()
	}
	if cfg.logger == nil {
		cfg.logger = NewNoopLogger()
	}
	bp := &batchProcessor[T, R]{
		cfg:        cfg,
		metrics:    cfg.metrics,
		logger:     cfg.logger,
		shutdownCh: make(chan struct{}),
	}
	if cfg.cbThreshold > 0 && cfg.cbDuration > 0 {
		bp.cb = newCircuitBreaker(cfg.cbThreshold, cfg.cbDuration, cfg.logger, cfg.metrics)
	}
	return bp
}

// ---------------------------------------------------------------------------
// Process
// ---------------------------------------------------------------------------

func (bp *batchProcessor[T, R]) Process(
	ctx context.Context,
	items []T,
	fn ProcessFunc[T, R],
) (*BatchResult[R], error) {
	if fn == nil {
		return nil, errors.NewInvalidInputError("process function must not be nil")
	}
	if bp.isShutdown.Load() {
		return nil, ErrShutdown
	}
	n := len(items)
	if n == 0 {
		return &BatchResult[R]{
			Results:    []*ItemResult[R]{},
			TotalCount: 0,
		}, nil
	}

	// Back-pressure check.
	if bp.cfg.backpressureThreshold > 0 {
		current := bp.pendingCount.Load()
		if current+int64(n) > int64(bp.cfg.backpressureThreshold) {
			return nil, ErrBackpressure
		}
	}
	bp.pendingCount.Add(int64(n))
	defer bp.pendingCount.Add(-int64(n))

	bp.activeWg.Add(1)
	defer bp.activeWg.Done()

	batchStart := time.Now()

	// Batch-level timeout context.
	batchCtx, batchCancel := context.WithTimeout(ctx, bp.cfg.batchTimeout)
	defer batchCancel()

	resultCh := make(chan *ItemResult[R], n)

	// Semaphore via a buffered channel (avoids external dependency for
	// maximum compatibility with go 1.22.1 stdlib).
	sem := make(chan struct{}, bp.cfg.maxConcurrency)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int, item T) {
			defer wg.Done()

			// Acquire semaphore (or bail on context).
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-batchCtx.Done():
				resultCh <- &ItemResult[R]{
					Index:  idx,
					Error:  batchCtx.Err(),
					Status: classifyCtxError(batchCtx.Err()),
				}
				return
			}

			ir := bp.processOneItem(batchCtx, idx, item, fn)
			resultCh <- ir
		}(i, items[i])
	}

	// Close resultCh once all goroutines finish.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Collect results.
	results := make([]*ItemResult[R], 0, n)
	for ir := range resultCh {
		results = append(results, ir)
	}

	// Sort by original index.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})

	totalDuration := time.Since(batchStart)
	br := bp.buildBatchResult(results, totalDuration)

	// Record metrics.
	bp.metrics.RecordBatchProcessing(ctx, &BatchMetricParams{
		BatchName:       "batch-processor",
		TotalItems:      br.TotalCount,
		SuccessItems:    br.SuccessCount,
		FailedItems:     br.FailureCount,
		TotalDurationMs: br.TotalDurationMs,
	})

	return br, nil
}

// ---------------------------------------------------------------------------
// ProcessWithPriority
// ---------------------------------------------------------------------------

func (bp *batchProcessor[T, R]) ProcessWithPriority(
	ctx context.Context,
	items []PrioritizedItem[T],
	fn ProcessFunc[T, R],
) (*BatchResult[R], error) {
	if fn == nil {
		return nil, errors.NewInvalidInputError("process function must not be nil")
	}
	if bp.isShutdown.Load() {
		return nil, ErrShutdown
	}
	n := len(items)
	if n == 0 {
		return &BatchResult[R]{
			Results:    []*ItemResult[R]{},
			TotalCount: 0,
		}, nil
	}

	// Back-pressure check.
	if bp.cfg.backpressureThreshold > 0 {
		current := bp.pendingCount.Load()
		if current+int64(n) > int64(bp.cfg.backpressureThreshold) {
			return nil, ErrBackpressure
		}
	}
	bp.pendingCount.Add(int64(n))
	defer bp.pendingCount.Add(-int64(n))

	bp.activeWg.Add(1)
	defer bp.activeWg.Done()

	batchStart := time.Now()
	batchCtx, batchCancel := context.WithTimeout(ctx, bp.cfg.batchTimeout)
	defer batchCancel()

	// Build a priority queue of work items.
	pq := make(priorityQueue[T], n)
	for i, pi := range items {
		pq[i] = &pqItem[T]{
			value:         pi.Item,
			originalIndex: i,
			priority:      pi.Priority,
			heapIndex:     i,
		}
	}
	heap.Init(&pq)

	resultCh := make(chan *ItemResult[R], n)
	sem := make(chan struct{}, bp.cfg.maxConcurrency)

	var wg sync.WaitGroup

	// Dispatch in priority order. We pop from the heap sequentially so that
	// higher-priority items acquire the semaphore first.
	for pq.Len() > 0 {
		it := heap.Pop(&pq).(*pqItem[T])
		wg.Add(1)

		// Acquire semaphore in the dispatching goroutine so that priority
		// ordering is respected (the highest-priority item blocks here first).
		select {
		case sem <- struct{}{}:
		case <-batchCtx.Done():
			// Push remaining items as cancelled.
			wg.Done()
			resultCh <- &ItemResult[R]{
				Index:  it.originalIndex,
				Error:  batchCtx.Err(),
				Status: classifyCtxError(batchCtx.Err()),
			}
			// Drain remaining.
			for pq.Len() > 0 {
				rem := heap.Pop(&pq).(*pqItem[T])
				resultCh <- &ItemResult[R]{
					Index:  rem.originalIndex,
					Error:  batchCtx.Err(),
					Status: classifyCtxError(batchCtx.Err()),
				}
			}
			goto collect
		}

		go func(idx int, item T) {
			defer wg.Done()
			defer func() { <-sem }()
			ir := bp.processOneItem(batchCtx, idx, item, fn)
			resultCh <- ir
		}(it.originalIndex, it.value)
	}

collect:
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]*ItemResult[R], 0, n)
	for ir := range resultCh {
		results = append(results, ir)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})

	totalDuration := time.Since(batchStart)
	br := bp.buildBatchResult(results, totalDuration)

	bp.metrics.RecordBatchProcessing(ctx, &BatchMetricParams{
		BatchName:       "batch-processor",
		TotalItems:      br.TotalCount,
		SuccessItems:    br.SuccessCount,
		FailedItems:     br.FailureCount,
		TotalDurationMs: br.TotalDurationMs,
	})

	return br, nil
}

// ---------------------------------------------------------------------------
// Shutdown
// ---------------------------------------------------------------------------

func (bp *batchProcessor[T, R]) Shutdown(ctx context.Context) error {
	bp.shutdownOnce.Do(func() {
		bp.isShutdown.Store(true)
		close(bp.shutdownCh)
	})

	// Wait for in-flight work or context expiry.
	done := make(chan struct{})
	go func() {
		bp.activeWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("shutdown timed out: %w", ctx.Err())
	}
}

// ---------------------------------------------------------------------------
// processOneItem — core per-item logic with retry + circuit-breaker
// ---------------------------------------------------------------------------

func (bp *batchProcessor[T, R]) processOneItem(
	batchCtx context.Context,
	idx int,
	item T,
	fn ProcessFunc[T, R],
) *ItemResult[R] {
	itemStart := time.Now()

	// Circuit-breaker gate.
	if bp.cb != nil && !bp.cb.allow() {
		return &ItemResult[R]{
			Index:      idx,
			Error:      ErrCircuitOpen,
			Status:     ItemStatusFailed,
			DurationMs: float64(time.Since(itemStart).Microseconds()) / 1000.0,
		}
	}

	maxAttempts := 1
	if bp.cfg.retryPolicy != nil && bp.cfg.retryPolicy.MaxRetries > 0 {
		maxAttempts = 1 + bp.cfg.retryPolicy.MaxRetries
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Retry back-off (skip on first attempt).
		if attempt > 0 {
			delay := calculateBackoff(attempt-1, bp.cfg.retryPolicy)
			if delay > 0 {
				select {
				case <-batchCtx.Done():
					return &ItemResult[R]{
						Index:      idx,
						Error:      batchCtx.Err(),
						Status:     classifyCtxError(batchCtx.Err()),
						DurationMs: msSince(itemStart),
					}
				case <-time.After(delay):
				}
			}
		}

		// Per-item timeout context derived from the batch context.
		itemCtx, itemCancel := context.WithTimeout(batchCtx, bp.cfg.itemTimeout)
		result, err := fn(itemCtx, item)
		itemCancel()

		if err == nil {
			if bp.cb != nil {
				bp.cb.recordSuccess()
			}
			return &ItemResult[R]{
				Index:      idx,
				Result:     result,
					Status:     ItemStatusSuccess,
					DurationMs: msSince(itemStart),
			}
		}

		lastErr = err
		if bp.cb != nil {
			bp.cb.recordFailure()
		}

		// Decide whether to retry.
		if attempt < maxAttempts-1 && shouldRetry(err, bp.cfg.retryPolicy) {
			continue
		}
		break
	}

	// Determine final status.
	status := ItemStatusFailed
	if lastErr != nil {
		status = classifyError(batchCtx, lastErr)
	}

	return &ItemResult[R]{
		Index:      idx,
		Error:      lastErr,
		Status:     status,
		DurationMs: msSince(itemStart),
	}
}

// ---------------------------------------------------------------------------
// Priority queue (max-heap by Priority)
// ---------------------------------------------------------------------------

type pqItem[T any] struct {
	value         T
	originalIndex int
	priority      int
	heapIndex     int
}

type priorityQueue[T any] []*pqItem[T]

func (pq priorityQueue[T]) Len() int { return len(pq) }

func (pq priorityQueue[T]) Less(i, j int) bool {
	// Higher priority first (max-heap).
	return pq[i].priority > pq[j].priority
}

func (pq priorityQueue[T]) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].heapIndex = i
	pq[j].heapIndex = j
}

func (pq *priorityQueue[T]) Push(x any) {
	item := x.(*pqItem[T])
	item.heapIndex = len(*pq)
	*pq = append(*pq, item)
}

func (pq *priorityQueue[T]) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.heapIndex = -1
	*pq = old[:n-1]
	return item
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (bp *batchProcessor[T, R]) buildBatchResult(
	results []*ItemResult[R],
	totalDuration time.Duration,
) *BatchResult[R] {
	br := &BatchResult[R]{
		Results:         results,
		TotalCount:      len(results),
		TotalDurationMs: float64(totalDuration.Microseconds()) / 1000.0,
	}
	var sumItemMs float64
	for _, r := range results {
		switch r.Status {
		case ItemStatusSuccess:
			br.SuccessCount++
		default:
			br.FailureCount++
		}
		sumItemMs += r.DurationMs
	}
	if br.TotalCount > 0 {
		br.AvgItemDurationMs = sumItemMs / float64(br.TotalCount)
	}
	return br
}

func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}

func classifyCtxError(err error) ItemStatus {
	if err == nil {
		return ItemStatusSuccess
	}
	if err == context.DeadlineExceeded {
		return ItemStatusTimeout
	}
	return ItemStatusCancelled
}

func classifyError(batchCtx context.Context, err error) ItemStatus {
	if err == nil {
		return ItemStatusSuccess
	}
	if err == context.DeadlineExceeded || stdliberrors.Is(err, context.DeadlineExceeded) {
		return ItemStatusTimeout
	}
	if err == context.Canceled || stdliberrors.Is(err, context.Canceled) {
		return ItemStatusCancelled
	}
	// Check if the batch context itself expired.
	if batchCtx.Err() == context.DeadlineExceeded {
		return ItemStatusTimeout
	}
	if batchCtx.Err() == context.Canceled {
		return ItemStatusCancelled
	}
	return ItemStatusFailed
}

