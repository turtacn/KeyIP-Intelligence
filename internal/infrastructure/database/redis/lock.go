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
	ErrLockNotAcquired = errors.New(errors.ErrCodeValidation, "failed to acquire lock")
	ErrLockNotHeld     = errors.New(errors.ErrCodeValidation, "lock not held by this owner")
	ErrLockExtendFailed = errors.New(errors.ErrCodeValidation, "failed to extend lock")
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

type lockConfig struct {
	ttl              time.Duration
	retryDelay       time.Duration
	retryCount       int
	watchdogEnabled  bool
	watchdogInterval time.Duration
}

type redisLockFactory struct {
	client *Client
	log    logging.Logger
}

func NewLockFactory(client *Client, log logging.Logger) LockFactory {
	return &redisLockFactory{
		client: client,
		log:    log,
	}
}

func (f *redisLockFactory) NewMutex(name string, opts ...LockOption) DistributedLock {
	cfg := lockConfig{
		ttl:              30 * time.Second,
		retryDelay:       100 * time.Millisecond,
		retryCount:       30,
		watchdogInterval: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.watchdogInterval == 0 && cfg.watchdogEnabled {
		cfg.watchdogInterval = cfg.ttl / 3
	}

	return &redisMutex{
		client: f.client,
		name:   name,
		value:  generateLockValue(),
		config: cfg,
		logger: f.log,
	}
}

func (f *redisLockFactory) NewReentrantLock(name string, ownerID string, opts ...LockOption) DistributedLock {
	cfg := lockConfig{
		ttl:              30 * time.Second,
		retryDelay:       100 * time.Millisecond,
		retryCount:       30,
		watchdogInterval: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.watchdogInterval == 0 && cfg.watchdogEnabled {
		cfg.watchdogInterval = cfg.ttl / 3
	}

	return &redisReentrantLock{
		client:  f.client,
		name:    name,
		ownerID: ownerID,
		config:  cfg,
		logger:  f.log,
	}
}

// Mutex Implementation

type redisMutex struct {
	client         *Client
	name           string
	value          string
	config         lockConfig
	logger         logging.Logger
	watchdogCancel context.CancelFunc
	watchdogDone   chan struct{}
}

var mutexUnlockScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	else
		return 0
	end
`)

var mutexExtendScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("PEXPIRE", KEYS[1], ARGV[2])
	else
		return 0
	end
`)

func (m *redisMutex) Lock(ctx context.Context) error {
	key := buildLockKey("mutex", m.name)
	for i := 0; i < m.config.retryCount; i++ {
		success, err := m.client.GetUnderlyingClient().SetNX(ctx, key, m.value, m.config.ttl).Result()
		if err == nil && success {
			if m.config.watchdogEnabled {
				m.startWatchdog()
			}
			return nil
		}
		if err != nil && err != redis.Nil {
			return errors.Wrap(err, errors.ErrCodeCacheError, "failed to set lock")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.config.retryDelay):
			continue
		}
	}
	return ErrLockNotAcquired
}

func (m *redisMutex) TryLock(ctx context.Context) (bool, error) {
	key := buildLockKey("mutex", m.name)

	success, err := m.client.GetUnderlyingClient().SetNX(ctx, key, m.value, m.config.ttl).Result()
	if err != nil {
		return false, err
	}
	if success {
		if m.config.watchdogEnabled {
			m.startWatchdog()
		}
	}
	return success, nil
}

func (m *redisMutex) Unlock(ctx context.Context) error {
	m.stopWatchdog()
	key := buildLockKey("mutex", m.name)
	res, err := mutexUnlockScript.Run(ctx, m.client.GetUnderlyingClient(), []string{key}, m.value).Result()
	if err != nil {
		return err
	}
	if res.(int64) == 0 {
		return ErrLockNotHeld
	}
	return nil
}

func (m *redisMutex) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	key := buildLockKey("mutex", m.name)
	res, err := mutexExtendScript.Run(ctx, m.client.GetUnderlyingClient(), []string{key}, m.value, ttl.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	return res.(int64) == 1, nil
}

func (m *redisMutex) TTL(ctx context.Context) (time.Duration, error) {
	key := buildLockKey("mutex", m.name)
	return m.client.GetUnderlyingClient().PTTL(ctx, key).Result()
}

func (m *redisMutex) startWatchdog() {
	ctx, cancel := context.WithCancel(context.Background())
	m.watchdogCancel = cancel
	m.watchdogDone = make(chan struct{})

	go runWatchdog(ctx, m.Extend, m.config.watchdogInterval, m.config.ttl, m.logger, m.watchdogDone)
}

func (m *redisMutex) stopWatchdog() {
	if m.watchdogCancel != nil {
		m.watchdogCancel()
		<-m.watchdogDone
		m.watchdogCancel = nil
	}
}

// Reentrant Lock Implementation

type redisReentrantLock struct {
	client         *Client
	name           string
	ownerID        string
	config         lockConfig
	logger         logging.Logger
	watchdogCancel context.CancelFunc
	watchdogDone   chan struct{}
}

var reentrantLockScript = redis.NewScript(`
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

var reentrantUnlockScript = redis.NewScript(`
	if redis.call("HEXISTS", KEYS[1], ARGV[1]) == 0 then
		return -1
	end
	local count = redis.call("HINCRBY", KEYS[1], ARGV[1], -1)
	if count <= 0 then
		redis.call("DEL", KEYS[1])
		return 0
	else
		redis.call("PEXPIRE", KEYS[1], ARGV[2])
		return count
	end
`)

var reentrantExtendScript = redis.NewScript(`
	if redis.call("HEXISTS", KEYS[1], ARGV[1]) == 1 then
		return redis.call("PEXPIRE", KEYS[1], ARGV[2])
	else
		return 0
	end
`)

func (l *redisReentrantLock) Lock(ctx context.Context) error {
	key := buildLockKey("reentrant", l.name)
	for i := 0; i < l.config.retryCount; i++ {
		res, err := reentrantLockScript.Run(ctx, l.client.GetUnderlyingClient(), []string{key}, l.ownerID, l.config.ttl.Milliseconds()).Result()
		if err == nil && res.(int64) == 1 {
			if l.config.watchdogEnabled {
				// Only start if not already running? Reentrant...
				// If re-entering, watchdog is already running.
				// We need tracking? Or simply ensure it's running.
				// But stopWatchdog() cancels it.
				// If we have count > 1, we shouldn't stop watchdog on Unlock unless count reaches 0.
				// My Unlock script returns count.
				// Lock script returns 1 always on success.
				// I should track local count or check if watchdog is nil.
				if l.watchdogCancel == nil {
					l.startWatchdog()
				}
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(l.config.retryDelay):
			continue
		}
	}
	return ErrLockNotAcquired
}

func (l *redisReentrantLock) TryLock(ctx context.Context) (bool, error) {
	key := buildLockKey("reentrant", l.name)
	res, err := reentrantLockScript.Run(ctx, l.client.GetUnderlyingClient(), []string{key}, l.ownerID, l.config.ttl.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	success := res.(int64) == 1
	if success && l.config.watchdogEnabled && l.watchdogCancel == nil {
		l.startWatchdog()
	}
	return success, nil
}

func (l *redisReentrantLock) Unlock(ctx context.Context) error {
	key := buildLockKey("reentrant", l.name)
	res, err := reentrantUnlockScript.Run(ctx, l.client.GetUnderlyingClient(), []string{key}, l.ownerID, l.config.ttl.Milliseconds()).Result()
	if err != nil {
		return err
	}
	ret := res.(int64)
	if ret == -1 {
		return ErrLockNotHeld
	}
	if ret == 0 {
		l.stopWatchdog()
	}
	return nil
}

func (l *redisReentrantLock) Extend(ctx context.Context, ttl time.Duration) (bool, error) {
	key := buildLockKey("reentrant", l.name)
	res, err := reentrantExtendScript.Run(ctx, l.client.GetUnderlyingClient(), []string{key}, l.ownerID, ttl.Milliseconds()).Result()
	if err != nil {
		return false, err
	}
	return res.(int64) == 1, nil
}

func (l *redisReentrantLock) TTL(ctx context.Context) (time.Duration, error) {
	key := buildLockKey("reentrant", l.name)
	return l.client.GetUnderlyingClient().PTTL(ctx, key).Result()
}

func (l *redisReentrantLock) startWatchdog() {
	ctx, cancel := context.WithCancel(context.Background())
	l.watchdogCancel = cancel
	l.watchdogDone = make(chan struct{})
	go runWatchdog(ctx, l.Extend, l.config.watchdogInterval, l.config.ttl, l.logger, l.watchdogDone)
}

func (l *redisReentrantLock) stopWatchdog() {
	if l.watchdogCancel != nil {
		l.watchdogCancel()
		<-l.watchdogDone
		l.watchdogCancel = nil
	}
}

// Helpers

func generateLockValue() string {
	return uuid.New().String()
}

func buildLockKey(lockType, name string) string {
	return "keyip:lock:" + lockType + ":" + name
}

func runWatchdog(ctx context.Context, extendFn func(context.Context, time.Duration) (bool, error), interval time.Duration, ttl time.Duration, log logging.Logger, done chan struct{}) {
	defer close(done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := extendFn(ctx, ttl)
			if err != nil {
				log.Error("Watchdog failed to extend lock", logging.Err(err))
				return
			}
			if !ok {
				log.Warn("Watchdog lost lock")
				return
			}
		}
	}
}
//Personal.AI order the ending
