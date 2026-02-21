package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"golang.org/x/sync/singleflight"
)

var (
	ErrCacheMiss         = errors.New(errors.ErrCodeNotFound, "cache miss")
	ErrCacheUnavailable  = errors.New(errors.ErrCodeServiceUnavailable, "cache unavailable")
	ErrSerializationFailed = errors.New(errors.ErrCodeSerialization, "serialization failed")
)

type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	MGet(ctx context.Context, keys []string) (map[string][]byte, error)
	MSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error
	GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func(ctx context.Context) (interface{}, error)) error
	DeleteByPrefix(ctx context.Context, prefix string) (int64, error)
	HGet(ctx context.Context, key, field string) (string, error)
	HSet(ctx context.Context, key string, fields map[string]interface{}, ttl time.Duration) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) error
	Incr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	ZAdd(ctx context.Context, key string, members ...*ZMember) error
	ZRangeByScore(ctx context.Context, key string, min, max float64, offset, count int64) ([]string, error)
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]*ZMember, error)
	ZRem(ctx context.Context, key string, members ...string) error
	ZScore(ctx context.Context, key, member string) (float64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
	Ping(ctx context.Context) error
}

type ZMember struct {
	Score  float64
	Member string
}

type Serializer interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type jsonSerializer struct{}

func (s *jsonSerializer) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (s *jsonSerializer) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type redisCache struct {
	client       *Client
	logger       logging.Logger
	prefix       string
	defaultTTL   time.Duration
	serializer   Serializer
	nullCacheTTL time.Duration
	singleflight singleflight.Group
}

type CacheOption func(*redisCache)

func WithPrefix(prefix string) CacheOption {
	return func(c *redisCache) { c.prefix = prefix }
}

func WithDefaultTTL(ttl time.Duration) CacheOption {
	return func(c *redisCache) { c.defaultTTL = ttl }
}

func WithSerializer(s Serializer) CacheOption {
	return func(c *redisCache) { c.serializer = s }
}

func WithNullCacheTTL(ttl time.Duration) CacheOption {
	return func(c *redisCache) { c.nullCacheTTL = ttl }
}

func NewRedisCache(client *Client, log logging.Logger, opts ...CacheOption) Cache {
	c := &redisCache{
		client:       client,
		logger:       log,
		prefix:       "keyip:",
		defaultTTL:   15 * time.Minute,
		serializer:   &jsonSerializer{},
		nullCacheTTL: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *redisCache) fullKey(key string) string {
	return c.prefix + key
}

func (c *redisCache) jitterTTL(ttl time.Duration) time.Duration {
	if ttl == 0 {
		return 0
	}
	// +/- 10%
	jitter := float64(ttl) * 0.1 * (rand.Float64()*2 - 1)
	return ttl + time.Duration(jitter)
}

func (c *redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	fullKey := c.fullKey(key)
	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeCacheError, "failed to get from cache")
	}
	if string(data) == "__null__" {
		return ErrCacheMiss
	}
	return c.serializer.Unmarshal(data, dest)
}

func (c *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := c.fullKey(key)
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	ttl = c.jitterTTL(ttl)

	data, err := c.serializer.Marshal(value)
	if err != nil {
		return ErrSerializationFailed
	}
	return c.client.Set(ctx, fullKey, data, ttl).Err()
}

func (c *redisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.fullKey(k)
	}
	return c.client.Del(ctx, fullKeys...).Err()
}

func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	val, err := c.client.Exists(ctx, c.fullKey(key)).Result()
	return val > 0, err
}

func (c *redisCache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.fullKey(k)
	}
	// Using MGet command from client might be simpler than pipeline if implemented
	// Client doesn't implement MGet directly as delegate, but we can access underlying or use Pipeline
	// Using Pipeline for flexibility
	pipe := c.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, k := range fullKeys {
		cmds[i] = pipe.Get(ctx, k)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		// Pipeline errors are tricky, typically returns first error
		// But we want partial results? Redis Go client returns error if any command fails.
		// For cache MGet, usually we accept misses.
		// If implementation uses MGET command, it returns list.
		// Let's stick to MGET command via Underlying if available or implement logic.
		// Standard redis.UniversalClient has MGet.
		// But `Client` wraps it.
		// I'll assume I can use `c.client.GetUnderlyingClient().MGet`.
		// But `GetUnderlyingClient` returns `redis.UniversalClient`.
		// So:
		vals, err := c.client.GetUnderlyingClient().MGet(ctx, fullKeys...).Result()
		if err != nil {
			return nil, err
		}
		result := make(map[string][]byte)
		for i, v := range vals {
			if v != nil {
				if s, ok := v.(string); ok {
					result[keys[i]] = []byte(s)
				}
			}
		}
		return result, nil
	}
	return nil, err // Should typically use MGet above
}

func (c *redisCache) MSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	ttl = c.jitterTTL(ttl)

	pipe := c.client.Pipeline()
	for k, v := range items {
		data, err := c.serializer.Marshal(v)
		if err != nil {
			return ErrSerializationFailed
		}
		pipe.Set(ctx, c.fullKey(k), data, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redisCache) GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func(ctx context.Context) (interface{}, error)) error {
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}
	if err != ErrCacheMiss {
		return err // Real error
	}

	// Singleflight
	val, err, _ := c.singleflight.Do(key, func() (interface{}, error) {
		v, loadErr := loader(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		if v == nil {
			// Cache null
			c.client.Set(ctx, c.fullKey(key), "__null__", c.nullCacheTTL)
			return nil, nil
		}
		// Set cache
		if setErr := c.Set(ctx, key, v, ttl); setErr != nil {
			c.logger.Warn("Failed to set cache in GetOrSet", logging.Err(setErr))
		}
		return v, nil
	})

	if err != nil {
		return err
	}
	if val == nil {
		return ErrCacheMiss // Was null
	}

	// Copy value to dest if possible.
	// Since val is interface{}, and dest is pointer.
	// We need to re-marshal/unmarshal or use reflection assignment.
	// Simplest: Marshal val to bytes, then Unmarshal to dest.
	data, _ := c.serializer.Marshal(val)
	return c.serializer.Unmarshal(data, dest)
}

func (c *redisCache) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	var deleted int64
	var cursor uint64
	match := c.fullKey(prefix) + "*"
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, match, 100).Result()
		if err != nil {
			return deleted, err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return deleted, err
			}
			deleted += int64(len(keys))
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return deleted, nil
}

// Hash operations
func (c *redisCache) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := c.client.HGet(ctx, c.fullKey(key), field).Result()
	if err == redis.Nil { return "", ErrCacheMiss }
	return val, err
}

func (c *redisCache) HSet(ctx context.Context, key string, fields map[string]interface{}, ttl time.Duration) error {
	fullKey := c.fullKey(key)
	pipe := c.client.Pipeline()
	pipe.HSet(ctx, fullKey, fields) // redis client HSet accepts map
	if ttl > 0 {
		pipe.Expire(ctx, fullKey, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, c.fullKey(key)).Result()
}

func (c *redisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, c.fullKey(key), fields...).Err()
}

// Counters
func (c *redisCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, c.fullKey(key)).Result()
}

func (c *redisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, c.fullKey(key), value).Result()
}

func (c *redisCache) Decr(ctx context.Context, key string) (int64, error) {
	return c.client.Decr(ctx, c.fullKey(key)).Result()
}

// ZSet
func (c *redisCache) ZAdd(ctx context.Context, key string, members ...*ZMember) error {
	zMembers := make([]redis.Z, len(members))
	for i, m := range members {
		zMembers[i] = redis.Z{Score: m.Score, Member: m.Member}
	}
	return c.client.ZAdd(ctx, c.fullKey(key), zMembers...).Err()
}

func (c *redisCache) ZRangeByScore(ctx context.Context, key string, min, max float64, offset, count int64) ([]string, error) {
	opt := &redis.ZRangeBy{Min: fmt.Sprintf("%f", min), Max: fmt.Sprintf("%f", max), Offset: offset, Count: count}
	return c.client.ZRangeByScore(ctx, c.fullKey(key), opt).Result()
}

func (c *redisCache) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]*ZMember, error) {
	vals, err := c.client.ZRevRangeWithScores(ctx, c.fullKey(key), start, stop).Result()
	if err != nil { return nil, err }
	result := make([]*ZMember, len(vals))
	for i, v := range vals {
		result[i] = &ZMember{Score: v.Score, Member: v.Member.(string)}
	}
	return result, nil
}

func (c *redisCache) ZRem(ctx context.Context, key string, members ...string) error {
	// Need to cast members to interface{}
	ifaces := make([]interface{}, len(members))
	for i, m := range members { ifaces[i] = m }
	return c.client.ZRem(ctx, c.fullKey(key), ifaces...).Err()
}

func (c *redisCache) ZScore(ctx context.Context, key, member string) (float64, error) {
	val, err := c.client.ZScore(ctx, c.fullKey(key), member).Result()
	if err == redis.Nil { return 0, ErrCacheMiss }
	return val, err
}

func (c *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.client.Expire(ctx, c.fullKey(key), ttl).Err()
}

func (c *redisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, c.fullKey(key)).Result()
}

func (c *redisCache) Ping(ctx context.Context) error {
	return c.client.GetUnderlyingClient().Ping(ctx).Err()
}

//Personal.AI order the ending
