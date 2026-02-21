package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func TestCache_GetSet(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, _ := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	cache := NewRedisCache(client, logging.NewNopLogger())

	ctx := context.Background()
	type Data struct { Name string }

	// Set
	err = cache.Set(ctx, "key", &Data{Name: "test"}, 0)
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

func TestCache_GetOrSet(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, _ := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	cache := NewRedisCache(client, logging.NewNopLogger())
	ctx := context.Background()

	// Loader called
	called := 0
	loader := func(ctx context.Context) (interface{}, error) {
		called++
		return &map[string]string{"foo": "bar"}, nil
	}

	var dest map[string]string
	err = cache.GetOrSet(ctx, "load_key", &dest, 0, loader)
	assert.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.Equal(t, "bar", dest["foo"])

	// Loader not called (cached)
	err = cache.GetOrSet(ctx, "load_key", &dest, 0, loader)
	assert.NoError(t, err)
	assert.Equal(t, 1, called)
}

func TestCache_DeleteByPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, _ := NewClient(&RedisConfig{Mode: "standalone", Addr: mr.Addr()}, logging.NewNopLogger())
	cache := NewRedisCache(client, logging.NewNopLogger())
	ctx := context.Background()

	cache.Set(ctx, "p:1", "v", 0)
	cache.Set(ctx, "p:2", "v", 0)
	cache.Set(ctx, "o:3", "v", 0)

	n, err := cache.DeleteByPrefix(ctx, "p:")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), n)

	exists, _ := cache.Exists(ctx, "p:1")
	assert.False(t, exists)
	exists, _ = cache.Exists(ctx, "o:3")
	assert.True(t, exists)
}

//Personal.AI order the ending
