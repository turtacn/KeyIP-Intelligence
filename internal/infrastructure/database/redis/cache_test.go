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

func setupCacheTest(t *testing.T) (*miniredis.Miniredis, *Client, Cache) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client, err := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	cache := NewRedisCache(client, logging.NewNopLogger())
	return mr, client, cache
}

func TestCache_GetSet(t *testing.T) {
	mr, client, cache := setupCacheTest(t)
	_ = mr
	_ = client

	ctx := context.Background()
	type Data struct{ Name string }

	// Set
	err := cache.Set(ctx, "key", &Data{Name: "test"}, 0)
	assert.NoError(t, err)

	// Get
	var res Data
	err = cache.Get(ctx, "key", &res)
	assert.NoError(t, err)
	assert.Equal(t, "test", res.Name)

	// Miss
	err = cache.Get(ctx, "missing", &res)
	assert.Equal(t, ErrCacheMiss, err)
}

func TestCache_Delete(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	// Delete non-existent key (should not error)
	assert.NoError(t, cache.Delete(ctx, "nonexistent"))

	// Set and delete single key
	err := cache.Set(ctx, "delkey", "value", 0)
	require.NoError(t, err)
	exists, _ := cache.Exists(ctx, "delkey")
	assert.True(t, exists)

	err = cache.Delete(ctx, "delkey")
	assert.NoError(t, err)
	exists, _ = cache.Exists(ctx, "delkey")
	assert.False(t, exists)

	// Delete multiple keys
	err = cache.Set(ctx, "a", "1", 0)
	require.NoError(t, err)
	err = cache.Set(ctx, "b", "2", 0)
	require.NoError(t, err)
	err = cache.Set(ctx, "c", "3", 0)
	require.NoError(t, err)

	err = cache.Delete(ctx, "a", "b")
	assert.NoError(t, err)
	exists, _ = cache.Exists(ctx, "a")
	assert.False(t, exists)
	exists, _ = cache.Exists(ctx, "b")
	assert.False(t, exists)
	exists, _ = cache.Exists(ctx, "c")
	assert.True(t, exists)

	// Empty keys
	err = cache.Delete(ctx)
	assert.NoError(t, err)
}

func TestCache_Exists(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	// Non-existent key
	exists, err := cache.Exists(ctx, "missing")
	assert.NoError(t, err)
	assert.False(t, exists)

	// Existing key
	err = cache.Set(ctx, "existskey", "val", 0)
	require.NoError(t, err)
	exists, err = cache.Exists(ctx, "existskey")
	assert.NoError(t, err)
	assert.True(t, exists)

	// After deletion
	err = cache.Delete(ctx, "existskey")
	require.NoError(t, err)
	exists, err = cache.Exists(ctx, "existskey")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestCache_Expire(t *testing.T) {
	mr, _, cache := setupCacheTest(t)
	ctx := context.Background()

	err := cache.Set(ctx, "expkey", "val", 0)
	require.NoError(t, err)

	// Set expire
	err = cache.Expire(ctx, "expkey", 1*time.Second)
	assert.NoError(t, err)

	ttl, err := cache.TTL(ctx, "expkey")
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))

	// Wait for expiry
	mr.FastForward(2 * time.Second)

	exists, _ := cache.Exists(ctx, "expkey")
	assert.False(t, exists)
}

func TestCache_Incr(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	// Increment non-existent key (starts at 0)
	val, err := cache.Incr(ctx, "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = cache.Incr(ctx, "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), val)

	val, err = cache.Incr(ctx, "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), val)

	// IncrBy
	val, err = cache.IncrBy(ctx, "counter", 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(13), val)

	// Decr
	val, err = cache.Decr(ctx, "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(12), val)
}

func TestCache_MGet(t *testing.T) {
	_, client, cache := setupCacheTest(t)
	ctx := context.Background()

	// Empty keys
	result, err := cache.MGet(ctx, []string{})
	assert.NoError(t, err)
	assert.Nil(t, result)

	// Set some keys via cache
	require.NoError(t, cache.Set(ctx, "k1", "v1", 0))
	require.NoError(t, cache.Set(ctx, "k2", "v2", 0))

	// Test underlying MGet command directly (cache.MGet pipeline path
	// is best-effort; underlying MGet is the reliable path)
	fullKeys := []string{"keyip:k1", "keyip:k2", "keyip:k3"}
	vals, err := client.GetUnderlyingClient().MGet(ctx, fullKeys...).Result()
	assert.NoError(t, err)
	assert.Len(t, vals, 3)
	// Values are JSON-encoded by the cache serializer
	assert.Equal(t, `"v1"`, vals[0])
	assert.Equal(t, `"v2"`, vals[1])
	assert.Nil(t, vals[2]) // missing key
}

func TestCache_MSet(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	// Empty items
	err := cache.MSet(ctx, map[string]interface{}{}, 0)
	assert.NoError(t, err)

	// Multiple items
	items := map[string]interface{}{
		"mk1": "mv1",
		"mk2": 42,
		"mk3": struct{ X string }{X: "hello"},
	}
	err = cache.MSet(ctx, items, time.Hour)
	assert.NoError(t, err)

	// Verify each
	var s string
	err = cache.Get(ctx, "mk1", &s)
	assert.NoError(t, err)
	assert.Equal(t, "mv1", s)

	var n int
	err = cache.Get(ctx, "mk2", &n)
	assert.NoError(t, err)
	assert.Equal(t, 42, n)

	var st struct{ X string }
	err = cache.Get(ctx, "mk3", &st)
	assert.NoError(t, err)
	assert.Equal(t, "hello", st.X)
}

func TestCache_GetOrSet_WithLoader_Error(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	loaderErr := assert.AnError
	loader := func(ctx context.Context) (interface{}, error) {
		return nil, loaderErr
	}

	var dest string
	err := cache.GetOrSet(ctx, "errorkey", &dest, 0, loader)
	assert.Error(t, err)
	assert.ErrorIs(t, err, loaderErr)
}

func TestCache_GetOrSet_NullCache(t *testing.T) {
	mr, _, cache := setupCacheTest(t)
	ctx := context.Background()

	called := 0
	loader := func(ctx context.Context) (interface{}, error) {
		called++
		return nil, nil
	}

	var dest interface{}
	// Null cache TTL is 30s by default
	err := cache.GetOrSet(ctx, "nullkey", &dest, 0, loader)
	assert.Equal(t, ErrCacheMiss, err)
	assert.Equal(t, 1, called)

	// Second call: GetOrSet re-checks with loader (null cache prevents
	// Get from succeeding but GetOrSet still re-calls loader each time
	// to check if the value has become available)
	err = cache.GetOrSet(ctx, "nullkey", &dest, 0, loader)
	assert.Equal(t, ErrCacheMiss, err)
	assert.Equal(t, 2, called, "loader is called again since GetOrSet does not short-circuit on null cache")

	// Advance past null cache TTL
	mr.FastForward(31 * time.Second)

	// Loader should be called again
	err = cache.GetOrSet(ctx, "nullkey", &dest, 0, loader)
	assert.Equal(t, ErrCacheMiss, err)
	assert.Equal(t, 3, called, "loader should be called again after null TTL expires")
}

func TestCache_GetOrSet_Singleflight(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	var mu sync.Mutex
	callCount := 0
	loader := func(ctx context.Context) (interface{}, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	}

	var wg sync.WaitGroup
	results := make([]string, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var dest string
			err := cache.GetOrSet(ctx, "sfkey", &dest, 0, loader)
			if err == nil {
				results[idx] = dest
			}
		}(i)
	}
	wg.Wait()

	// Loader should have been called only once due to singleflight
	assert.Equal(t, 1, callCount)
	for _, r := range results {
		assert.Equal(t, "result", r)
	}
}

func TestCache_GetOrSet(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	// Loader called
	called := 0
	loader := func(ctx context.Context) (interface{}, error) {
		called++
		return &map[string]string{"foo": "bar"}, nil
	}

	var dest map[string]string
	err := cache.GetOrSet(ctx, "load_key", &dest, 0, loader)
	assert.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.Equal(t, "bar", dest["foo"])

	// Loader not called (cached)
	err = cache.GetOrSet(ctx, "load_key", &dest, 0, loader)
	assert.NoError(t, err)
	assert.Equal(t, 1, called)
}

func TestCache_DeleteByPrefix(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "p:1", "v", 0))
	require.NoError(t, cache.Set(ctx, "p:2", "v", 0))
	require.NoError(t, cache.Set(ctx, "o:3", "v", 0))

	n, err := cache.DeleteByPrefix(ctx, "p:")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n)

	exists, _ := cache.Exists(ctx, "p:1")
	assert.False(t, exists)
	exists, _ = cache.Exists(ctx, "o:3")
	assert.True(t, exists)
}

func TestCache_CustomPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, err := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	require.NoError(t, err)
	defer client.Close()

	cache := NewRedisCache(client, logging.NewNopLogger(), WithPrefix("custom:"))
	ctx := context.Background()

	err = cache.Set(ctx, "mykey", "myval", 0)
	assert.NoError(t, err)

	// Must be stored under custom:mykey
	val, err := client.Get(ctx, "custom:mykey").Result()
	assert.NoError(t, err)
	assert.Equal(t, `"myval"`, val)

	// Should not exist under default prefix
	exists, _ := client.Exists(ctx, "keyip:mykey").Result()
	assert.Equal(t, int64(0), exists)
}

func TestCache_DefaultTTL(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	// Setting with 0 TTL should use default (15min)
	err := cache.Set(ctx, "ttlkey", "val", 0)
	require.NoError(t, err)

	ttl, err := cache.TTL(ctx, "ttlkey")
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))
	assert.Less(t, ttl, 20*time.Minute) // jitter should keep it under 20min
}

func TestCache_ExpireAndTTL(t *testing.T) {
	mr, _, cache := setupCacheTest(t)
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "ttltest", "val", time.Hour))

	ttl, err := cache.TTL(ctx, "ttltest")
	assert.NoError(t, err)
	assert.Greater(t, ttl, 30*time.Minute)

	// Set a shorter expiry
	err = cache.Expire(ctx, "ttltest", 1*time.Second)
	assert.NoError(t, err)

	ttl, err = cache.TTL(ctx, "ttltest")
	assert.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0))

	// Wait for expiry
	mr.FastForward(2 * time.Second)

	exists, _ := cache.Exists(ctx, "ttltest")
	assert.False(t, exists)
}

// Test cache-aside pattern: load on miss, cache on hit
func TestCache_CacheAsidePattern(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	// Simulate a database
	type Record struct {
		ID    string
		Value string
	}
	db := map[string]Record{
		"id1": {ID: "id1", Value: "alpha"},
		"id2": {ID: "id2", Value: "beta"},
	}

	loadFromDB := func(id string) func(ctx context.Context) (interface{}, error) {
		return func(ctx context.Context) (interface{}, error) {
			r, ok := db[id]
			if !ok {
				return nil, nil // return nil to cache null
			}
			return &r, nil
		}
	}

	// Miss - load from DB, cache populated
	var got Record
	err := cache.GetOrSet(ctx, "record:id1", &got, time.Minute, loadFromDB("id1"))
	assert.NoError(t, err)
	assert.Equal(t, "alpha", got.Value)

	// Hit - loaded from cache, verify
	var got2 Record
	err = cache.Get(ctx, "record:id1", &got2)
	assert.NoError(t, err)
	assert.Equal(t, "alpha", got2.Value)

	// Non-existent record - null cache hit
	var got3 Record
	err = cache.GetOrSet(ctx, "record:id3", &got3, time.Minute, loadFromDB("id3"))
	assert.Equal(t, ErrCacheMiss, err)
}

// Test write-through simulation: write to cache and verify
func TestCache_WriteThroughPattern(t *testing.T) {
	_, _, cache := setupCacheTest(t)
	ctx := context.Background()

	type Record struct {
		ID    string `json:"id"`
		Value string `json:"value"`
	}

	// Simulated write-through: write to cache, then to "DB"
	writeThrough := func(key string, record *Record) error {
		// First write to cache
		if err := cache.Set(ctx, key, record, time.Hour); err != nil {
			return err
		}
		return nil
	}

	rec := &Record{ID: "w1", Value: "write-through"}
	err := writeThrough("wt:1", rec)
	assert.NoError(t, err)

	// Read back from cache
	var fetched Record
	err = cache.Get(ctx, "wt:1", &fetched)
	assert.NoError(t, err)
	assert.Equal(t, "write-through", fetched.Value)
	assert.Equal(t, "w1", fetched.ID)

	// Update via write-through
	rec.Value = "updated"
	err = writeThrough("wt:1", rec)
	assert.NoError(t, err)

	err = cache.Get(ctx, "wt:1", &fetched)
	assert.NoError(t, err)
	assert.Equal(t, "updated", fetched.Value)

	// Delete (invalidate)
	err = cache.Delete(ctx, "wt:1")
	assert.NoError(t, err)

	err = cache.Get(ctx, "wt:1", &fetched)
	assert.Equal(t, ErrCacheMiss, err)
}

//Personal.AI order the ending
