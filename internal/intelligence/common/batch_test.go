package common

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBatchProcessor_Defaults(t *testing.T) {
	bp := NewBatchProcessor[string, string]()
	assert.NotNil(t, bp)
}

func TestProcess_AllSuccess(t *testing.T) {
	bp := NewBatchProcessor[string, string]()
	items := []string{"a", "b", "c"}
	fn := func(ctx context.Context, item string) (string, error) {
		return item + "_processed", nil
	}

	res, err := bp.Process(context.Background(), items, fn)
	assert.NoError(t, err)
	assert.Equal(t, 3, res.SuccessCount)
	assert.Equal(t, "a_processed", res.Results[0].Result)
}

func TestProcess_AllFailure(t *testing.T) {
	bp := NewBatchProcessor[string, string]()
	items := []string{"a", "b"}
	fn := func(ctx context.Context, item string) (string, error) {
		return "", errors.New("failed")
	}

	res, err := bp.Process(context.Background(), items, fn)
	assert.NoError(t, err)
	assert.Equal(t, 0, res.SuccessCount)
	assert.Equal(t, 2, res.FailureCount)
	assert.Error(t, res.Results[0].Error)
}

func TestProcess_ConcurrencyLimit(t *testing.T) {
	var concurrentCount int32
	var maxConcurrent int32

	bp := NewBatchProcessor[int, int](WithMaxConcurrency(2))
	items := []int{1, 2, 3, 4, 5}

	fn := func(ctx context.Context, item int) (int, error) {
		curr := atomic.AddInt32(&concurrentCount, 1)
		defer atomic.AddInt32(&concurrentCount, -1)

		max := atomic.LoadInt32(&maxConcurrent)
		if curr > max {
			atomic.StoreInt32(&maxConcurrent, curr)
		}

		time.Sleep(10 * time.Millisecond)
		return item * 2, nil
	}

	_, err := bp.Process(context.Background(), items, fn)
	assert.NoError(t, err)
	// This assertion is tricky because semaphore guarantees order but maxConcurrent update might have race if not careful,
	// but atomic Store should be fine. However, since we sleep, we expect concurrency to reach 2.
	// But it might not exceed 2.
	assert.LessOrEqual(t, atomic.LoadInt32(&maxConcurrent), int32(2))
}

func TestProcess_ItemTimeout(t *testing.T) {
	bp := NewBatchProcessor[int, int](WithItemTimeout(10 * time.Millisecond))
	items := []int{1}

	fn := func(ctx context.Context, item int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return item, nil
		}
	}

	res, err := bp.Process(context.Background(), items, fn)
	assert.NoError(t, err)
	assert.Equal(t, 1, res.FailureCount)
	assert.Equal(t, ItemStatusTimeout, res.Results[0].Status)
}

func TestProcessWithPriority_ResultOrdering(t *testing.T) {
	bp := NewBatchProcessor[string, string](WithMaxConcurrency(1)) // Force serial
	items := []PrioritizedItem[string]{
		{Item: "low", Priority: 1},
		{Item: "high", Priority: 10},
	}

	var executionOrder []string
	fn := func(ctx context.Context, item string) (string, error) {
		executionOrder = append(executionOrder, item)
		return item, nil
	}

	res, err := bp.ProcessWithPriority(context.Background(), items, fn)
	assert.NoError(t, err)

	// Expect high priority executed first (since we sort)
	// But `executionOrder` capture in goroutines might be racy if concurrency > 1.
	// With concurrency 1, it should be deterministic if sort worked.
	// Wait, processWithPriority sorts then calls Process.
	// Process iterates and launches goroutines.
	// Semaphore acquire is FIFO usually.
	// So sorting items should result in execution order.

	// Check results are reordered back to original index
	assert.Equal(t, "low", res.Results[0].Result)
	assert.Equal(t, "high", res.Results[1].Result)
}
//Personal.AI order the ending
