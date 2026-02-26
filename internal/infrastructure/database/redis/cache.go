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
	ErrCacheMiss         = errors.New(errors.ErrCodeCacheError, "cache miss")
	ErrCacheUnavailable  = errors.New(errors.ErrCodeCacheError, "cache unavailable")
	ErrSerializationFailed = errors.New(errors.ErrCodeSerialization, "serialization failed")
)

type Serializer interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

type JSONSerializer struct{}

func (s JSONSerializer) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (s JSONSerializer) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type ZMember struct {
	Score  float64
	Member string
}

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

type redisCache struct {
	client       *Client
	log          logging.Logger
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
		log:          log,
		prefix:       "keyip:",
		defaultTTL:   15 * time.Minute,
		serializer:   JSONSerializer{},
		nullCacheTTL: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *redisCache) buildKey(key string) string {
	return c.prefix + key
}

func (c *redisCache) jitterTTL(ttl time.Duration) time.Duration {
	if ttl == 0 {
		return 0
	}
	// +/- 10%
	jitter := time.Duration(float64(ttl) * 0.1 * (rand.Float64()*2 - 1))
	return ttl + jitter
}

func (c *redisCache) Get(ctx context.Context, key string, dest interface{}) error {
	fullKey := c.buildKey(key)
	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return ErrCacheMiss
		}
		return errors.Wrap(err, errors.ErrCodeCacheError, "redis get failed")
	}

	if string(data) == "__null__" {
		return ErrCacheMiss // Or distinct ErrNullCache? Treating as miss but cached miss
	}

	if err := c.serializer.Unmarshal(data, dest); err != nil {
		return errors.Wrap(err, errors.ErrCodeSerialization, "unmarshal failed")
	}
	return nil
}

func (c *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	fullKey := c.buildKey(key)
	if ttl == 0 {
		ttl = c.defaultTTL
	}
	ttl = c.jitterTTL(ttl)

	data, err := c.serializer.Marshal(value)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeSerialization, "marshal failed")
	}

	if err := c.client.Set(ctx, fullKey, data, ttl).Err(); err != nil {
		return errors.Wrap(err, errors.ErrCodeCacheError, "redis set failed")
	}
	return nil
}

func (c *redisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	fullKeys := make([]string, len(keys))
	for i, k := range keys {
		fullKeys[i] = c.buildKey(k)
	}
	return c.client.Del(ctx, fullKeys...).Err()
}

func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	val, err := c.client.Exists(ctx, c.buildKey(key)).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

func (c *redisCache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	pipe := c.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd)
	for _, k := range keys {
		cmds[k] = pipe.Get(ctx, c.buildKey(k))
	}
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for k, cmd := range cmds {
		data, err := cmd.Bytes()
		if err == nil {
			result[k] = data
		}
	}
	return result, nil
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
			return errors.Wrap(err, errors.ErrCodeSerialization, "marshal failed")
		}
		pipe.Set(ctx, c.buildKey(k), data, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redisCache) GetOrSet(ctx context.Context, key string, dest interface{}, ttl time.Duration, loader func(ctx context.Context) (interface{}, error)) error {
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrCacheMiss) {
		return err // Redis error
	}

	// Singleflight
	val, err, _ := c.singleflight.Do(key, func() (interface{}, error) {
		// Double check cache? singleflight handles concurrent calls, but if first one fails/misses, all fail/miss.
		// Loader
		v, loadErr := loader(ctx)
		if loadErr != nil {
			return nil, loadErr
		}

		// If nil, cache null
		if v == nil {
			c.client.Set(ctx, c.buildKey(key), "__null__", c.nullCacheTTL)
			return nil, nil // Return nil as success but dest remains empty
		}

		// Cache value
		if setErr := c.Set(ctx, key, v, ttl); setErr != nil {
			c.log.Warn("Failed to set cache in GetOrSet", logging.Err(setErr))
		}
		return v, nil
	})

	if err != nil {
		return err
	}
	if val == nil {
		return ErrCacheMiss // Loader returned nil or null cached
	}

	// Copy value to dest?
	// val is interface{}. dest is pointer.
	// We need to marshal/unmarshal to copy to dest structure if val type matches?
	// Loader returns interface{}. `v` might be *Struct.
	// If `dest` is *Struct, we can try to assign or marshal/unmarshal.
	// To be safe and consistent with Get behavior (which unmarshals), we can marshal val and unmarshal to dest.
	// Or use reflection. Marshal/Unmarshal is safer.
	data, _ := c.serializer.Marshal(val)
	return c.serializer.Unmarshal(data, dest)
}

func (c *redisCache) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	fullPrefix := c.buildKey(prefix) + "*"
	var deleted int64
	var cursor uint64

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, fullPrefix, 100).Result()
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

func (c *redisCache) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := c.client.HGet(ctx, c.buildKey(key), field).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return val, err
}

func (c *redisCache) HSet(ctx context.Context, key string, fields map[string]interface{}, ttl time.Duration) error {
	pipe := c.client.Pipeline()
	fullKey := c.buildKey(key)

	// Flatten map to slice
	values := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		values = append(values, k, v)
	}

	pipe.HSet(ctx, fullKey, values...)
	if ttl > 0 {
		pipe.Expire(ctx, fullKey, ttl)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *redisCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, c.buildKey(key)).Result()
}

func (c *redisCache) HDel(ctx context.Context, key string, fields ...string) error {
	return c.client.HDel(ctx, c.buildKey(key), fields...).Err()
}

func (c *redisCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, c.buildKey(key)).Result()
}

func (c *redisCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, c.buildKey(key), value).Result()
}

func (c *redisCache) Decr(ctx context.Context, key string) (int64, error) {
	return c.client.Decr(ctx, c.buildKey(key)).Result()
}

func (c *redisCache) ZAdd(ctx context.Context, key string, members ...*ZMember) error {
	zMembers := make([]redis.Z, len(members))
	for i, m := range members {
		zMembers[i] = redis.Z{Score: m.Score, Member: m.Member}
	}
	return c.client.ZAdd(ctx, c.buildKey(key), zMembers...).Err()
}

func (c *redisCache) ZRangeByScore(ctx context.Context, key string, min, max float64, offset, count int64) ([]string, error) {
	opt := &redis.ZRangeBy{
		Min:    fmt.Sprintf("%f", min),
		Max:    fmt.Sprintf("%f", max),
		Offset: offset,
		Count:  count,
	}
	return c.client.ZRangeByScore(ctx, c.buildKey(key), opt).Result()
}

func (c *redisCache) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]*ZMember, error) {
	res, err := c.client.ZRevRangeWithScores(ctx, c.buildKey(key), start, stop).Result()
	if err != nil {
		return nil, err
	}
	members := make([]*ZMember, len(res))
	for i, z := range res {
		members[i] = &ZMember{Score: z.Score, Member: z.Member.(string)}
	}
	return members, nil
}

func (c *redisCache) ZRem(ctx context.Context, key string, members ...string) error {
	// Convert []string to []interface{}
	args := make([]interface{}, len(members))
	for i, m := range members {
		args[i] = m
	}
	return c.client.ZRem(ctx, c.buildKey(key), args...).Err()
}

func (c *redisCache) ZScore(ctx context.Context, key, member string) (float64, error) {
	return c.client.ZScore(ctx, c.buildKey(key), member).Result()
}

func (c *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.client.Expire(ctx, c.buildKey(key), ttl).Err()
}

func (c *redisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, c.buildKey(key)).Result()
}

func (c *redisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx)
}
//Personal.AI order the ending
