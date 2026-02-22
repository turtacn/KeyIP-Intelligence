package common

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"golang.org/x/sync/semaphore"
)

// ItemStatus represents the processing status of a batch item.
type ItemStatus string

const (
	ItemStatusSuccess   ItemStatus = "success"
	ItemStatusFailed    ItemStatus = "failed"
	ItemStatusTimeout   ItemStatus = "timeout"
	ItemStatusCancelled ItemStatus = "cancelled"
)

// ProcessFunc defines the function to process a single item.
type ProcessFunc[T, R any] func(ctx context.Context, item T) (R, error)

// PrioritizedItem represents an item with priority.
type PrioritizedItem[T any] struct {
	Item     T
	Priority int
}

// ItemResult represents the result of processing a single item.
type ItemResult[R any] struct {
	Index      int
	Result     R
	Error      error
	DurationMs float64
	Status     ItemStatus
}

// BatchResult represents the result of a batch processing operation.
type BatchResult[R any] struct {
	Results           []*ItemResult[R]
	TotalCount        int
	SuccessCount      int
	FailureCount      int
	TotalDurationMs   float64
	AvgItemDurationMs float64
}

// RetryPolicy defines the retry strategy.
type RetryPolicy struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	RetryableErrors   []error // Simplified: explicit error matching or check type
}

// BatchProcessor defines the interface for batch processing.
type BatchProcessor[T, R any] interface {
	Process(ctx context.Context, items []T, fn ProcessFunc[T, R]) (*BatchResult[R], error)
	ProcessWithPriority(ctx context.Context, items []PrioritizedItem[T], fn ProcessFunc[T, R]) (*BatchResult[R], error)
	Shutdown(ctx context.Context) error
}

// batchProcessor implements BatchProcessor.
type batchProcessor[T, R any] struct {
	maxConcurrency int
	itemTimeout    time.Duration
	batchTimeout   time.Duration
	retryPolicy    *RetryPolicy
	circuitBreaker *circuitBreaker
	backpressure   int32
	backpressureThreshold int32

	metrics IntelligenceMetrics
	logger  logging.Logger
	sem     *semaphore.Weighted

	wg sync.WaitGroup
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

// BatchOption defines configuration options for BatchProcessor.
type BatchOption func(*batchProcessorOptions)

type batchProcessorOptions struct {
	MaxConcurrency        int
	ItemTimeout           time.Duration
	BatchTimeout          time.Duration
	RetryPolicy           *RetryPolicy
	BackpressureThreshold int
	Metrics               IntelligenceMetrics
	Logger                logging.Logger
}

func WithMaxConcurrency(n int) BatchOption {
	return func(o *batchProcessorOptions) { o.MaxConcurrency = n }
}

func WithItemTimeout(d time.Duration) BatchOption {
	return func(o *batchProcessorOptions) { o.ItemTimeout = d }
}

func WithBatchTimeout(d time.Duration) BatchOption {
	return func(o *batchProcessorOptions) { o.BatchTimeout = d }
}

func WithRetryPolicy(policy *RetryPolicy) BatchOption {
	return func(o *batchProcessorOptions) { o.RetryPolicy = policy }
}

func WithBackpressureThreshold(n int) BatchOption {
	return func(o *batchProcessorOptions) { o.BackpressureThreshold = n }
}

func WithMetrics(m IntelligenceMetrics) BatchOption {
	return func(o *batchProcessorOptions) { o.Metrics = m }
}

func WithLogger(l logging.Logger) BatchOption {
	return func(o *batchProcessorOptions) { o.Logger = l }
}

var (
	ErrCircuitOpen   = errors.New("circuit breaker open")
	ErrBackpressure  = errors.New("backpressure limit exceeded")
	ErrShutdown      = errors.New("processor is shutting down")
	ErrBatchTimeout  = errors.New("batch timeout")
)

// NewBatchProcessor creates a new BatchProcessor.
func NewBatchProcessor[T, R any](opts ...BatchOption) BatchProcessor[T, R] {
	options := &batchProcessorOptions{
		MaxConcurrency:        10,
		ItemTimeout:           30 * time.Second,
		BatchTimeout:          5 * time.Minute,
		BackpressureThreshold: 100,
		Metrics:               NewNoopIntelligenceMetrics(),
		// Logger can be nil, handled in methods
	}
	for _, opt := range opts {
		opt(options)
	}

	ctx, cancel := context.WithCancel(context.Background())

	bp := &batchProcessor[T, R]{
		maxConcurrency:        options.MaxConcurrency,
		itemTimeout:           options.ItemTimeout,
		batchTimeout:          options.BatchTimeout,
		retryPolicy:           options.RetryPolicy,
		backpressureThreshold: int32(options.BackpressureThreshold),
		metrics:               options.Metrics,
		logger:                options.Logger,
		sem:                   semaphore.NewWeighted(int64(options.MaxConcurrency)),
		shutdownCtx:           ctx,
		shutdownCancel:        cancel,
		// Circuit breaker simplified
		circuitBreaker: &circuitBreaker{state: 0},
	}
	return bp
}

func (p *batchProcessor[T, R]) Process(ctx context.Context, items []T, fn ProcessFunc[T, R]) (*BatchResult[R], error) {
	if len(items) == 0 {
		return &BatchResult[R]{}, nil
	}
	if fn == nil {
		return nil, ErrInvalidInput
	}

	// Check shutdown
	if p.shutdownCtx.Err() != nil {
		return nil, ErrShutdown
	}

	// Backpressure check
	currentLoad := atomic.AddInt32(&p.backpressure, int32(len(items)))
	defer atomic.AddInt32(&p.backpressure, -int32(len(items)))

	if p.backpressureThreshold > 0 && currentLoad > p.backpressureThreshold {
		return nil, ErrBackpressure
	}

	// Check circuit breaker
	if p.circuitBreaker.isOpen() {
		return nil, ErrCircuitOpen
	}

	startTime := time.Now()

	// Batch context
	batchCtx, cancel := context.WithTimeout(ctx, p.batchTimeout)
	defer cancel()

	results := make([]*ItemResult[R], len(items))
	resultChan := make(chan *ItemResult[R], len(items))

	for i, item := range items {
		p.wg.Add(1)
		go func(idx int, it T) {
			defer p.wg.Done()

			// Acquire semaphore
			if err := p.sem.Acquire(batchCtx, 1); err != nil {
				// Failed to acquire, likely context cancelled/timeout
				resultChan <- &ItemResult[R]{
					Index:  idx,
					Error:  err,
					Status: ItemStatusCancelled,
				}
				return
			}
			defer p.sem.Release(1)

			itemStart := time.Now()
			itemCtx, itemCancel := context.WithTimeout(batchCtx, p.itemTimeout)
			defer itemCancel()

			res, err := p.processItemWithRetry(itemCtx, it, fn)
			duration := time.Since(itemStart).Seconds() * 1000

			status := ItemStatusSuccess
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					status = ItemStatusTimeout
				} else if errors.Is(err, context.Canceled) {
					status = ItemStatusCancelled
				} else {
					status = ItemStatusFailed
				}
				// Update circuit breaker on failure
				p.circuitBreaker.recordFailure()
			} else {
				p.circuitBreaker.recordSuccess()
			}

			resultChan <- &ItemResult[R]{
				Index:      idx,
				Result:     res,
				Error:      err,
				DurationMs: duration,
				Status:     status,
			}
		}(i, item)
	}

	// Wait for all goroutines
	p.wg.Wait()
	close(resultChan)

	// Collect results
	successCount := 0
	failureCount := 0
	totalItemDuration := 0.0

	for res := range resultChan {
		results[res.Index] = res
		if res.Status == ItemStatusSuccess {
			successCount++
		} else {
			failureCount++
		}
		totalItemDuration += res.DurationMs
	}

	totalDuration := time.Since(startTime).Seconds() * 1000
	avgDuration := 0.0
	if len(items) > 0 {
		avgDuration = totalItemDuration / float64(len(items))
	}

	batchRes := &BatchResult[R]{
		Results:           results,
		TotalCount:        len(items),
		SuccessCount:      successCount,
		FailureCount:      failureCount,
		TotalDurationMs:   totalDuration,
		AvgItemDurationMs: avgDuration,
	}

	// Record metrics
	p.metrics.RecordBatchProcessing(ctx, &BatchMetricParams{
		BatchName:         "default",
		TotalItems:        len(items),
		SuccessItems:      successCount,
		FailedItems:       failureCount,
		TotalDurationMs:   totalDuration,
		AvgItemDurationMs: avgDuration,
		MaxConcurrency:    p.maxConcurrency,
	})

	return batchRes, nil
}

func (p *batchProcessor[T, R]) ProcessWithPriority(ctx context.Context, items []PrioritizedItem[T], fn ProcessFunc[T, R]) (*BatchResult[R], error) {
	// Simple sorting implementation for now
	// Ideally should use priority queue for execution order, but sorting beforehand works if we submit in order.
	// Since we spawn goroutines that wait on semaphore, if we spawn in priority order, they will likely acquire semaphore in that order (FIFO queue in semaphore).

	// Sort by priority desc
	// Make a copy with index to preserve original order
	type indexedItem struct {
		Item     T
		Priority int
		Index    int
	}

	sortedItems := make([]indexedItem, len(items))
	for i, it := range items {
		sortedItems[i] = indexedItem{it.Item, it.Priority, i}
	}

	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].Priority > sortedItems[j].Priority
	})

	// Unpack to T slice for Process, but Process needs mapping back.
	// We cannot reuse Process directly because Process returns results by index of passed slice.
	// We need custom logic or map results back.

	// Custom implementation reusing logic:

	// ... (Skipping full duplication for brevity, would be similar to Process but iterating sortedItems)
	// For this phase, I'll map to T and map results back.

	tItems := make([]T, len(items))
	for i, it := range sortedItems {
		tItems[i] = it.Item
	}

	res, err := p.Process(ctx, tItems, fn)
	if err != nil {
		return nil, err
	}

	// Remap results to original order
	remappedResults := make([]*ItemResult[R], len(items))
	for i, r := range res.Results {
		originalIndex := sortedItems[i].Index // i is index in sorted list
		r.Index = originalIndex // Restore original index
		remappedResults[originalIndex] = r
	}
	res.Results = remappedResults

	return res, nil
}

func (p *batchProcessor[T, R]) processItemWithRetry(ctx context.Context, item T, fn ProcessFunc[T, R]) (R, error) {
	var result R
	var err error

	attempts := 1
	if p.retryPolicy != nil {
		attempts = p.retryPolicy.MaxRetries + 1
	}

	for i := 0; i < attempts; i++ {
		result, err = fn(ctx, item)
		if err == nil {
			return result, nil
		}

		// Check retry policy
		if p.retryPolicy == nil {
			return result, err
		}

		// Calculate backoff
		backoff := p.retryPolicy.InitialBackoff * time.Duration(math.Pow(p.retryPolicy.BackoffMultiplier, float64(i)))
		if backoff > p.retryPolicy.MaxBackoff {
			backoff = p.retryPolicy.MaxBackoff
		}

		// Wait
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(backoff):
			// Retry
		}
	}
	return result, err
}

func (p *batchProcessor[T, R]) Shutdown(ctx context.Context) error {
	p.shutdownCancel()

	// Wait for active tasks with context
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// circuitBreaker (simplified)
type circuitBreaker struct {
	state int32 // 0: closed, 1: open
	failures int32
	lastFailure time.Time
}

func (cb *circuitBreaker) isOpen() bool {
	return atomic.LoadInt32(&cb.state) == 1
}

func (cb *circuitBreaker) recordSuccess() {
	atomic.StoreInt32(&cb.failures, 0)
	atomic.StoreInt32(&cb.state, 0)
}

func (cb *circuitBreaker) recordFailure() {
	f := atomic.AddInt32(&cb.failures, 1)
	if f > 5 { // Threshold hardcoded for simplicity
		atomic.StoreInt32(&cb.state, 1)
		// Should spawn goroutine to reset after timeout
	}
}

//Personal.AI order the ending
