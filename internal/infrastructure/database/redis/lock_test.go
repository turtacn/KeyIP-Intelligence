package redis

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func setupLockTest(t *testing.T) (*miniredis.Miniredis, *Client, LockFactory) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, err := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	factory := NewLockFactory(client, logging.NewNopLogger())
	return mr, client, factory
}

func TestMutex_Lock_Unlock(t *testing.T) {
	mr, client, factory := setupLockTest(t)
	_ = mr

	ctx := context.Background()
	lock := factory.NewMutex("test-lock", WithLockTTL(1*time.Second))

	// Lock
	err := lock.Lock(ctx)
	assert.NoError(t, err)

	// Check Redis
	exists, _ := client.Exists(ctx, "keyip:lock:mutex:test-lock").Result()
	assert.Equal(t, int64(1), exists)

	// Unlock
	err = lock.Unlock(ctx)
	assert.NoError(t, err)

	exists, _ = client.Exists(ctx, "keyip:lock:mutex:test-lock").Result()
	assert.Equal(t, int64(0), exists)
}

func TestMutex_Lock_Contention(t *testing.T) {
	mr, client, factory := setupLockTest(t)
	_ = mr
	_ = client

	ctx := context.Background()
	lock1 := factory.NewMutex("test-lock", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))
	lock2 := factory.NewMutex("test-lock", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))

	// Lock 1
	err := lock1.Lock(ctx)
	assert.NoError(t, err)

	// Lock 2 should fail
	err = lock2.Lock(ctx)
	assert.Equal(t, ErrLockNotAcquired, err)

	// Unlock 1
	lock1.Unlock(ctx)

	// Lock 2 should succeed
	err = lock2.Lock(ctx)
	assert.NoError(t, err)
}

func TestMutex_TryLock(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock1 := factory.NewMutex("trylock-test")
	lock2 := factory.NewMutex("trylock-test")

	// First TryLock should succeed
	ok, err := lock1.TryLock(ctx)
	assert.NoError(t, err)
	assert.True(t, ok)

	// Second TryLock should fail
	ok, err = lock2.TryLock(ctx)
	assert.NoError(t, err)
	assert.False(t, ok)

	// Unlock first
	err = lock1.Unlock(ctx)
	assert.NoError(t, err)

	// Second TryLock should succeed now
	ok, err = lock2.TryLock(ctx)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestMutex_TTL(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewMutex("ttl-lock", WithLockTTL(5*time.Second))

	// TTL before lock - key does not exist, TTL should be negative
	ttl, err := lock.TTL(ctx)
	assert.NoError(t, err)
	assert.True(t, ttl < 0, "TTL should be negative for non-existent key")

	// Lock and check TTL
	err = lock.Lock(ctx)
	assert.NoError(t, err)

	ttl, err = lock.TTL(ctx)
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 5*time.Second)
}

func TestMutex_AutoExpiry(t *testing.T) {
	mr, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewMutex("expire-lock", WithLockTTL(100*time.Millisecond))

	err := lock.Lock(ctx)
	assert.NoError(t, err)

	// Fast forward past TTL
	mr.FastForward(200 * time.Millisecond)

	// Lock should have auto-expired, so a new lock can acquire it
	lock2 := factory.NewMutex("expire-lock", WithRetryCount(1), WithRetryDelay(10*time.Millisecond), WithLockTTL(100*time.Millisecond))
	err = lock2.Lock(ctx)
	assert.NoError(t, err, "second lock should acquire after auto-expiry")
}

func TestMutex_Extend(t *testing.T) {
	mr, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewMutex("extend-lock", WithLockTTL(1*time.Second))

	err := lock.Lock(ctx)
	assert.NoError(t, err)

	// Extend TTL
	ok, err := lock.Extend(ctx, 5*time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	// TTL should now be closer to 5s
	ttl, err := lock.TTL(ctx)
	assert.NoError(t, err)
	assert.Greater(t, ttl, 2*time.Second)

	// Advance time past original TTL but within extended TTL
	mr.FastForward(2 * time.Second)

	// Lock should still be held
	exists := mr.Exists("keyip:lock:mutex:extend-lock")
	assert.True(t, exists)
}

func TestMutex_Extend_NotHeld(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewMutex("extend-not-held", WithLockTTL(1*time.Second))

	// Extending a lock that was never acquired should fail
	ok, err := lock.Extend(ctx, 5*time.Second)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestMutex_Unlock_WrongValue(t *testing.T) {
	mr, client, factory := setupLockTest(t)
	_ = client
	ctx := context.Background()

	lock := factory.NewMutex("wrong-unlock", WithLockTTL(1*time.Second))

	err := lock.Lock(ctx)
	assert.NoError(t, err)

	// Manually set a different value in Redis (simulating another owner)
	mr.Set("keyip:lock:mutex:wrong-unlock", "different-value")

	// Unlock should fail because our value doesn't match
	err = lock.Unlock(ctx)
	assert.Equal(t, ErrLockNotHeld, err)
}

func TestMutex_Lock_CtxCancelled(t *testing.T) {
	_, _, factory := setupLockTest(t)

	lock1 := factory.NewMutex("ctx-cancel", WithRetryCount(10), WithRetryDelay(50*time.Millisecond))
	lock2 := factory.NewMutex("ctx-cancel", WithRetryCount(10), WithRetryDelay(50*time.Millisecond))

	ctx := context.Background()
	err := lock1.Lock(ctx)
	require.NoError(t, err)

	// Create cancelled context
	ctxCancelled, cancel := context.WithCancel(ctx)
	cancel() // cancel immediately

	err = lock2.Lock(ctxCancelled)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestMutex_ConcurrentLockAttempts(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	// Multiple goroutines competing for the same lock
	const numWorkers = 10
	var wg sync.WaitGroup
	winner := make(chan int, 1)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lock := factory.NewMutex("concurrent-lock",
				WithRetryCount(3),
				WithRetryDelay(5*time.Millisecond),
				WithLockTTL(2*time.Second),
			)
			err := lock.Lock(ctx)
			if err == nil {
				select {
				case winner <- id:
				default:
				}
				lock.Unlock(ctx)
			}
		}(i)
	}
	wg.Wait()

	select {
	case <-winner:
		// At least one goroutine acquired the lock
	default:
		t.Fatal("no goroutine could acquire the lock")
	}
}

func TestMutex_LockReleaseReacquire(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewMutex("rera-lock", WithLockTTL(5*time.Second))

	// Acquire
	err := lock.Lock(ctx)
	assert.NoError(t, err)

	// Release
	err = lock.Unlock(ctx)
	assert.NoError(t, err)

	// Re-acquire
	err = lock.Lock(ctx)
	assert.NoError(t, err)

	// Release again
	err = lock.Unlock(ctx)
	assert.NoError(t, err)
}

func TestReentrantLock_Reentry(t *testing.T) {
	mr, client, factory := setupLockTest(t)
	_ = mr

	ctx := context.Background()
	lock := factory.NewReentrantLock("test-reentrant", "worker-1")

	// Lock 1st time
	err := lock.Lock(ctx)
	assert.NoError(t, err)

	// Lock 2nd time (reentry)
	err = lock.Lock(ctx)
	assert.NoError(t, err)

	// Unlock 1st time (still held)
	err = lock.Unlock(ctx)
	assert.NoError(t, err)
	exists, _ := client.Exists(ctx, "keyip:lock:reentrant:test-reentrant").Result()
	assert.Equal(t, int64(1), exists)

	// Unlock 2nd time (released)
	err = lock.Unlock(ctx)
	assert.NoError(t, err)
	exists, _ = client.Exists(ctx, "keyip:lock:reentrant:test-reentrant").Result()
	assert.Equal(t, int64(0), exists)
}

func TestReentrantLock_DifferentOwner(t *testing.T) {
	mr, client, factory := setupLockTest(t)
	_ = mr
	_ = client

	ctx := context.Background()
	lock1 := factory.NewReentrantLock("test-reentrant", "worker-1")
	lock2 := factory.NewReentrantLock("test-reentrant", "worker-2", WithRetryCount(0)) // No retry

	lock1.Lock(ctx)
	err := lock2.Lock(ctx)
	assert.Equal(t, ErrLockNotAcquired, err)
}

func TestReentrantLock_TTL(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewReentrantLock("re-ttl", "owner-1", WithLockTTL(5*time.Second))

	// TTL before lock - key does not exist, TTL should be negative
	ttl, err := lock.TTL(ctx)
	assert.NoError(t, err)
	assert.True(t, ttl < 0, "TTL should be negative for non-existent key")

	// Lock and check TTL
	err = lock.Lock(ctx)
	assert.NoError(t, err)

	ttl, err = lock.TTL(ctx)
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.LessOrEqual(t, ttl, 5*time.Second)
}

func TestReentrantLock_AutoExpiry(t *testing.T) {
	mr, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock1 := factory.NewReentrantLock("re-expire", "worker-a", WithLockTTL(100*time.Millisecond))

	err := lock1.Lock(ctx)
	assert.NoError(t, err)

	// Fast forward past TTL
	mr.FastForward(200 * time.Millisecond)

	// Different worker should be able to acquire after expiry
	lock2 := factory.NewReentrantLock("re-expire", "worker-b", WithRetryCount(1), WithRetryDelay(10*time.Millisecond), WithLockTTL(100*time.Millisecond))
	err = lock2.Lock(ctx)
	assert.NoError(t, err, "second worker should acquire after auto-expiry")
}

func TestReentrantLock_Extend(t *testing.T) {
	mr, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewReentrantLock("re-extend", "owner-x", WithLockTTL(1*time.Second))

	err := lock.Lock(ctx)
	assert.NoError(t, err)

	// Extend TTL
	ok, err := lock.Extend(ctx, 5*time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	ttl, err := lock.TTL(ctx)
	assert.NoError(t, err)
	assert.Greater(t, ttl, 2*time.Second)

	// Advance past original TTL
	mr.FastForward(2 * time.Second)

	exists := mr.Exists("keyip:lock:reentrant:re-extend")
	assert.True(t, exists)
}

func TestReentrantLock_Unlock_WrongOwner(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock1 := factory.NewReentrantLock("re-wrong", "owner-a")
	lock2 := factory.NewReentrantLock("re-wrong", "owner-b")

	err := lock1.Lock(ctx)
	assert.NoError(t, err)

	// Owner B trying to unlock owner A's lock should fail
	err = lock2.Unlock(ctx)
	assert.Equal(t, ErrLockNotHeld, err)

	// Owner A can still unlock
	err = lock1.Unlock(ctx)
	assert.NoError(t, err)
}

func TestReentrantLock_DeepReentry(t *testing.T) {
	_, _, factory := setupLockTest(t)
	ctx := context.Background()

	lock := factory.NewReentrantLock("deep-re", "worker-d", WithLockTTL(10*time.Second))

	// Lock 5 times
	for i := 0; i < 5; i++ {
		err := lock.Lock(ctx)
		assert.NoError(t, err, "reentry attempt %d should succeed", i+1)
	}

	// Unlock 4 times - still held
	for i := 0; i < 4; i++ {
		err := lock.Unlock(ctx)
		assert.NoError(t, err)
	}

	exists, err := mrExists(factory, "deep-re")
	assert.NoError(t, err)
	assert.True(t, exists, "lock should still be held after 4 unlocks")

	// 5th unlock releases
	err = lock.Unlock(ctx)
	assert.NoError(t, err)

	exists, err = mrExists(factory, "deep-re")
	assert.NoError(t, err)
	assert.False(t, exists, "lock should be released after 5th unlock")
}

func TestReentrantLock_CtxCancelled(t *testing.T) {
	_, _, factory := setupLockTest(t)

	lock1 := factory.NewReentrantLock("re-ctx-cancel", "owner-p", WithRetryCount(10), WithRetryDelay(50*time.Millisecond))
	lock2 := factory.NewReentrantLock("re-ctx-cancel", "owner-q", WithRetryCount(10), WithRetryDelay(50*time.Millisecond))

	ctx := context.Background()
	err := lock1.Lock(ctx)
	require.NoError(t, err)

	ctxCancelled, cancel := context.WithCancel(ctx)
	cancel()

	err = lock2.Lock(ctxCancelled)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// Helper to check if a reentrant lock key exists in miniredis
func mrExists(factory LockFactory, name string) (bool, error) {
	// Access the underlying factory's client
	rf := factory.(*redisLockFactory)
	n, err := rf.client.GetUnderlyingClient().Exists(context.Background(), "keyip:lock:reentrant:"+name).Result()
	return n > 0, err
}

//Personal.AI order the ending
