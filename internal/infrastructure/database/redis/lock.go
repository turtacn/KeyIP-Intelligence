package redis

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	ErrLockNotAcquired = errors.New(errors.ErrCodeConflict, "failed to acquire lock")
	ErrLockNotHeld     = errors.New(errors.ErrCodeConflict, "lock not held by this owner")
	ErrLockExtendFailed = errors.New(errors.ErrCodeInternal, "failed to extend lock")
)

type DistributedLock interface {
	Lock(ctx context.Context) error
	TryLock(ctx context.Context) (bool, error)
	Unlock(ctx context.Context) error
	Extend(ctx context.Context, ttl time.Duration) (bool, error)
	TTL(ctx context.Context) (time.Duration, error)
}

type LockFactory interface {
	NewMutex(name string, opts ...LockOption) DistributedLock
	NewReentrantLock(name string, ownerID string, opts ...LockOption) DistributedLock
}

type lockConfig struct {
	ttl              time.Duration
	retryDelay       time.Duration
	retryCount       int
	watchdogEnabled  bool
	watchdogInterval time.Duration
}

type LockOption func(*lockConfig)

func WithLockTTL(ttl time.Duration) LockOption {
	return func(c *lockConfig) { c.ttl = ttl }
}

func WithRetryDelay(delay time.Duration) LockOption {
	return func(c *lockConfig) { c.retryDelay = delay }
}

func WithRetryCount(count int) LockOption {
	return func(c *lockConfig) { c.retryCount = count }
}

func WithWatchdog(enabled bool) LockOption {
	return func(c *lockConfig) { c.watchdogEnabled = enabled }
}

func WithWatchdogInterval(interval time.Duration) LockOption {
	return func(c *lockConfig) { c.watchdogInterval = interval }
}

type redisLockFactory struct {
	client *Client
	logger logging.Logger
}

func NewLockFactory(client *Client, log logging.Logger) LockFactory {
	return &redisLockFactory{
		client: client,
		logger: log,
	}
}

func (f *redisLockFactory) NewMutex(name string, opts ...LockOption) DistributedLock {
	cfg := lockConfig{
		ttl:        30 * time.Second,
		retryDelay: 100 * time.Millisecond,
		retryCount: 30,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.watchdogEnabled && cfg.watchdogInterval == 0 {
		cfg.watchdogInterval = cfg.ttl / 3
	}
	return &redisMutex{
		client: f.client,
		name:   "keyip:lock:mutex:" + name,
		value:  uuid.New().String(),
		config: cfg,
		logger: f.logger,
	}
}

func (f *redisLockFactory) NewReentrantLock(name string, ownerID string, opts ...LockOption) DistributedLock {
	cfg := lockConfig{
		ttl:        30 * time.Second,
		retryDelay: 100 * time.Millisecond,
		retryCount: 30,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.watchdogEnabled && cfg.watchdogInterval <= 0 {
		if cfg.ttl > 0 {
			cfg.watchdogInterval = cfg.ttl / 3
		} else {
			cfg.watchdogInterval = 10 * time.Second // Default fallback
		}
	}
	if cfg.watchdogInterval <= 0 {
		cfg.watchdogInterval = 10 * time.Second
	}
	return &redisReentrantLock{
		client:  f.client,
		name:    "keyip:lock:reentrant:" + name,
		ownerID: ownerID,
		config:  cfg,
		logger:  f.logger,
	}
}

type redisMutex struct {
	client         *Client
	name           string
	value          string
	config         lockConfig
	logger         logging.Logger
	watchdogCancel context.CancelFunc
	watchdogDone   chan struct{}
}

func (m *redisMutex) Lock(ctx context.Context) error {
	for i := 0; i <= m.config.retryCount; i++ {
		ok, err := m.TryLock(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if i < m.config.retryCount {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.config.retryDelay):
				continue
			}
		}
	}
	return ErrLockNotAcquired
}

func (m *redisMutex) TryLock(ctx context.Context) (bool, error) {
	ok, err := m.client.GetUnderlyingClient().SetNX(ctx, m.name, m.value, m.config.ttl).Result()
	if err != nil {
		return false, err
	}
	if ok {
		if m.config.watchdogEnabled {
			m.startWatchdog()
		}
		return true, nil
	}
	return false, nil
}

func (m *redisMutex) Unlock(ctx context.Context) error {
	m.stopWatchdog()
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)
	res, err := script.Run(ctx, m.client.GetUnderlyingClient(), []string{m.name}, m.value).Result()
	if err != nil {
		return err
	}
	if res.(int64) == 0 {
		return ErrLockNotHeld
	}
	return nil
}

func (m *redisMutex) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	res, err := script.Run(ctx, m.client.GetUnderlyingClient(), []string{m.name}, m.value, ttl.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	return res.(int64) == 1, nil
}

func (m *redisMutex) TTL(ctx context.Context) (time.Duration, error) {
	return m.client.TTL(ctx, m.name).Result()
}

func (m *redisMutex) startWatchdog() {
	ctx, cancel := context.WithCancel(context.Background())
	m.watchdogCancel = cancel
	m.watchdogDone = make(chan struct{})
	go func() {
		defer close(m.watchdogDone)
		ticker := time.NewTicker(m.config.watchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ok, err := m.Extend(ctx, m.config.ttl)
				if err != nil || !ok {
					// Lock lost
					return
				}
			}
		}
	}()
}

func (m *redisMutex) stopWatchdog() {
	if m.watchdogCancel != nil {
		m.watchdogCancel()
		<-m.watchdogDone
		m.watchdogCancel = nil
	}
}

type redisReentrantLock struct {
	client         *Client
	name           string
	ownerID        string
	config         lockConfig
	logger         logging.Logger
	watchdogCancel context.CancelFunc
	watchdogDone   chan struct{}
}

// Reentrant Lock Implementation

func (m *redisReentrantLock) Lock(ctx context.Context) error {
	for i := 0; i <= m.config.retryCount; i++ {
		ok, err := m.TryLock(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if i < m.config.retryCount {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.config.retryDelay):
				continue
			}
		}
	}
	return ErrLockNotAcquired
}

func (m *redisReentrantLock) TryLock(ctx context.Context) (bool, error) {
	script := redis.NewScript(`
		if redis.call("EXISTS", KEYS[1]) == 0 then
			redis.call("HSET", KEYS[1], ARGV[1], 1)
			redis.call("PEXPIRE", KEYS[1], ARGV[2])
			return 1
		elseif redis.call("HEXISTS", KEYS[1], ARGV[1]) == 1 then
			redis.call("HINCRBY", KEYS[1], ARGV[1], 1)
			redis.call("PEXPIRE", KEYS[1], ARGV[2])
			return 1
		else
			return 0
		end
	`)
	res, err := script.Run(ctx, m.client.GetUnderlyingClient(), []string{m.name}, m.ownerID, m.config.ttl.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	if res.(int64) == 1 {
		if m.config.watchdogEnabled && m.watchdogCancel == nil {
			m.startWatchdog()
		}
		return true, nil
	}
	return false, nil
}

func (m *redisReentrantLock) Unlock(ctx context.Context) error {
	script := redis.NewScript(`
		if redis.call("HEXISTS", KEYS[1], ARGV[1]) == 0 then
			return -1
		end
		local count = redis.call("HINCRBY", KEYS[1], ARGV[1], -1)
		if count <= 0 then
			redis.call("DEL", KEYS[1])
			return 0
		else
			redis.call("PEXPIRE", KEYS[1], ARGV[2]) -- Reset expiration on partial unlock to prevent expiry? Or keep original? Usually keep or extend.
			return count
		end
	`)
	// Note: We might want to extend TTL on partial unlock to keep it alive for the remaining nesting levels.
	// Adding ARGV[2] for TTL.
	res, err := script.Run(ctx, m.client.GetUnderlyingClient(), []string{m.name}, m.ownerID, m.config.ttl.Milliseconds()).Result()
	if err != nil {
		return err
	}
	val := res.(int64)
	if val == -1 {
		return ErrLockNotHeld
	}
	if val == 0 {
		m.stopWatchdog()
	}
	return nil
}

func (m *redisReentrantLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	script := redis.NewScript(`
		if redis.call("HEXISTS", KEYS[1], ARGV[1]) == 1 then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	res, err := script.Run(ctx, m.client.GetUnderlyingClient(), []string{m.name}, m.ownerID, ttl.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	return res.(int64) == 1, nil
}

func (m *redisReentrantLock) TTL(ctx context.Context) (time.Duration, error) {
	return m.client.TTL(ctx, m.name).Result()
}

func (m *redisReentrantLock) startWatchdog() {
	// ... Same as mutex
	ctx, cancel := context.WithCancel(context.Background())
	m.watchdogCancel = cancel
	m.watchdogDone = make(chan struct{})
	go func() {
		defer close(m.watchdogDone)
		ticker := time.NewTicker(m.config.watchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done(): return
			case <-ticker.C:
				ok, err := m.Extend(ctx, m.config.ttl)
				if err != nil || !ok { return }
			}
		}
	}()
}

func (m *redisReentrantLock) stopWatchdog() {
	if m.watchdogCancel != nil {
		m.watchdogCancel()
		<-m.watchdogDone
		m.watchdogCancel = nil
	}
}

//Personal.AI order the ending
