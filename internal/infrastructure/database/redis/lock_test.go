package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type LockTestSuite struct {
	suite.Suite
	mr      *miniredis.Miniredis
	client  *Client
	factory LockFactory
	log     logging.Logger
}

func (s *LockTestSuite) SetupTest() {
	var err error
	s.mr, err = miniredis.Run()
	require.NoError(s.T(), err)

	s.log = logging.NewNopLogger()
	cfg := &RedisConfig{
		Mode: "standalone",
		Addr: s.mr.Addr(),
	}
	s.client, err = NewClient(cfg, s.log)
	require.NoError(s.T(), err)

	s.factory = NewLockFactory(s.client, s.log)
}

func (s *LockTestSuite) TearDownTest() {
	s.client.Close()
	s.mr.Close()
}

func (s *LockTestSuite) TestMutex_Lock_Unlock_Success() {
	mutex := s.factory.NewMutex("test-lock")
	ctx := context.Background()

	err := mutex.Lock(ctx)
	assert.NoError(s.T(), err)

	// Check Redis
	assert.True(s.T(), s.mr.Exists("keyip:lock:mutex:test-lock"))

	err = mutex.Unlock(ctx)
	assert.NoError(s.T(), err)

	// Check Redis
	assert.False(s.T(), s.mr.Exists("keyip:lock:mutex:test-lock"))
}

func (s *LockTestSuite) TestMutex_Lock_AlreadyHeld() {
	mutex1 := s.factory.NewMutex("test-lock-conflict", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))
	mutex2 := s.factory.NewMutex("test-lock-conflict", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))
	ctx := context.Background()

	err := mutex1.Lock(ctx)
	assert.NoError(s.T(), err)

	err = mutex2.Lock(ctx)
	assert.Error(s.T(), err)
	// Check equality of Error type, not instance if it wraps/creates new error
	// ErrLockNotAcquired is a sentinel error defined in lock.go
	assert.Equal(s.T(), ErrLockNotAcquired, err)
}

func (s *LockTestSuite) TestMutex_TryLock() {
	mutex1 := s.factory.NewMutex("test-lock")
	mutex2 := s.factory.NewMutex("test-lock")
	ctx := context.Background()

	ok, err := mutex1.TryLock(ctx)
	assert.NoError(s.T(), err)
	assert.True(s.T(), ok)

	ok, err = mutex2.TryLock(ctx)
	assert.NoError(s.T(), err)
	assert.False(s.T(), ok)
}

func (s *LockTestSuite) TestMutex_Unlock_NotHeld() {
	mutex := s.factory.NewMutex("test-lock")
	ctx := context.Background()

	err := mutex.Unlock(ctx)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), ErrLockNotHeld, err)
}

func (s *LockTestSuite) TestMutex_Unlock_DifferentValue() {
	// Simulate stealing lock
	mutex1 := s.factory.NewMutex("test-lock")
	ctx := context.Background()

	err := mutex1.Lock(ctx)
	assert.NoError(s.T(), err)

	// Manually overwrite in redis
	s.mr.Set("keyip:lock:mutex:test-lock", "some-other-value")

	err = mutex1.Unlock(ctx)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), ErrLockNotHeld, err)
}

func (s *LockTestSuite) TestMutex_Watchdog() {
	// miniredis doesn't support PEXPIRE fully with Lua time advancement?
	// But it does support basic TTL.
	// Watchdog runs in background.
	// ttl = 100ms, watchdog = 20ms

	mutex := s.factory.NewMutex("test-lock", WithLockTTL(100*time.Millisecond), WithWatchdog(true), WithWatchdogInterval(20*time.Millisecond))
	ctx := context.Background()

	err := mutex.Lock(ctx)
	assert.NoError(s.T(), err)

	// Sleep longer than TTL
	time.Sleep(200 * time.Millisecond)

	// Lock should still exist
	assert.True(s.T(), s.mr.Exists("keyip:lock:mutex:test-lock"))

	err = mutex.Unlock(ctx)
	assert.NoError(s.T(), err)
}

func (s *LockTestSuite) TestReentrant_Lock_Unlock() {
	lock := s.factory.NewReentrantLock("test-rlock", "worker-1")
	ctx := context.Background()

	err := lock.Lock(ctx)
	assert.NoError(s.T(), err)

	// Verify Hash
	val := s.mr.HGet("keyip:lock:reentrant:test-rlock", "worker-1")
	assert.Equal(s.T(), "1", val)

	// Reenter
	err = lock.Lock(ctx)
	assert.NoError(s.T(), err)

	val = s.mr.HGet("keyip:lock:reentrant:test-rlock", "worker-1")
	assert.Equal(s.T(), "2", val)

	// Unlock once
	err = lock.Unlock(ctx)
	assert.NoError(s.T(), err)
	val = s.mr.HGet("keyip:lock:reentrant:test-rlock", "worker-1")
	assert.Equal(s.T(), "1", val)

	// Unlock twice
	err = lock.Unlock(ctx)
	assert.NoError(s.T(), err)
	assert.False(s.T(), s.mr.Exists("keyip:lock:reentrant:test-rlock"))
}

func (s *LockTestSuite) TestReentrant_DifferentOwner() {
	lock1 := s.factory.NewReentrantLock("test-rlock", "worker-1", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))
	lock2 := s.factory.NewReentrantLock("test-rlock", "worker-2", WithRetryCount(1), WithRetryDelay(10*time.Millisecond))
	ctx := context.Background()

	err := lock1.Lock(ctx)
	assert.NoError(s.T(), err)

	err = lock2.Lock(ctx)
	assert.Error(s.T(), err)
	assert.Equal(s.T(), ErrLockNotAcquired, err)
}

func TestLockSuite(t *testing.T) {
	suite.Run(t, new(LockTestSuite))
}
//Personal.AI order the ending
