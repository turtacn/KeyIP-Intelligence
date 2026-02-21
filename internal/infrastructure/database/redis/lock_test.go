package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func TestMutex_Lock_Unlock(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, _ := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	factory := NewLockFactory(client, logging.NewNopLogger())

	ctx := context.Background()
	lock := factory.NewMutex("test-lock", WithLockTTL(1*time.Second))

	// Lock
	err = lock.Lock(ctx)
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
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, _ := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	factory := NewLockFactory(client, logging.NewNopLogger())

	ctx := context.Background()
	lock1 := factory.NewMutex("test-lock", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))
	lock2 := factory.NewMutex("test-lock", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))

	// Lock 1
	err = lock1.Lock(ctx)
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

func TestReentrantLock_Reentry(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, _ := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	factory := NewLockFactory(client, logging.NewNopLogger())

	ctx := context.Background()
	lock := factory.NewReentrantLock("test-reentrant", "worker-1")

	// Lock 1st time
	err = lock.Lock(ctx)
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
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, _ := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	factory := NewLockFactory(client, logging.NewNopLogger())

	ctx := context.Background()
	lock1 := factory.NewReentrantLock("test-reentrant", "worker-1")
	lock2 := factory.NewReentrantLock("test-reentrant", "worker-2", WithRetryCount(0)) // No retry

	lock1.Lock(ctx)
	err = lock2.Lock(ctx)
	assert.Equal(t, ErrLockNotAcquired, err)
}

//Personal.AI order the ending
