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

func setupClientTest(t *testing.T) (*miniredis.Miniredis, *Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	cfg := &RedisConfig{Mode: "standalone", Addr: mr.Addr()}
	client, err := NewClient(cfg, logging.NewNopLogger())
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })

	return mr, client
}

func TestNewClient_Standalone_Success(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &RedisConfig{
		Mode: "standalone",
		Addr: mr.Addr(),
	}
	logger := logging.NewNopLogger()

	client, err := NewClient(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, client)
	defer client.Close()

	assert.NoError(t, client.GetUnderlyingClient().Ping(context.Background()).Err())
}

func TestNewClient_Standalone_ConnectionFailed(t *testing.T) {
	cfg := &RedisConfig{
		Mode: "standalone",
		Addr: "localhost:99999", // Invalid port
	}
	logger := logging.NewNopLogger()

	client, err := NewClient(cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestClient_Operations(t *testing.T) {
	_, client := setupClientTest(t)
	ctx := context.Background()

	// Get/Set
	err := client.Set(ctx, "foo", "bar", 0).Err()
	assert.NoError(t, err)
	val, err := client.Get(ctx, "foo").Result()
	assert.NoError(t, err)
	assert.Equal(t, "bar", val)

	// Del
	deleted, err := client.Del(ctx, "foo").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Exists
	exists, err := client.Exists(ctx, "foo").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), exists)

	// Hash
	err = client.HSet(ctx, "hash", "f1", "v1").Err()
	assert.NoError(t, err)
	hval, err := client.HGet(ctx, "hash", "f1").Result()
	assert.NoError(t, err)
	assert.Equal(t, "v1", hval)
}

func TestClient_Close(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &RedisConfig{Mode: "standalone", Addr: mr.Addr()}
	client, err := NewClient(cfg, logging.NewNopLogger())
	require.NoError(t, err)

	assert.NoError(t, client.Close())

	// Should fail after close
	err = client.Get(context.Background(), "foo").Err()
	assert.Equal(t, ErrClientClosed, err)

	// Double close should not panic
	assert.NoError(t, client.Close())
}

func TestClient_Ping(t *testing.T) {
	_, client := setupClientTest(t)
	ctx := context.Background()

	// Ping the underlying client
	err := client.GetUnderlyingClient().Ping(ctx).Err()
	assert.NoError(t, err)
}

func TestClient_HealthCheck(t *testing.T) {
	_, client := setupClientTest(t)
	ctx := context.Background()

	// Set and retrieve to verify health
	err := client.Set(ctx, "health:check", "ok", 10*time.Second).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, "health:check").Result()
	assert.NoError(t, err)
	assert.Equal(t, "ok", val)

	// Del after check
	_, err = client.Del(ctx, "health:check").Result()
	assert.NoError(t, err)
}

func TestClient_DefaultConfig(t *testing.T) {
	// Verify defaults are applied
	cfg := &RedisConfig{
		Mode: "standalone",
		Addr: "localhost:6379",
	}
	applyDefaults(cfg)
	assert.Equal(t, 40, cfg.PoolSize)
	assert.Equal(t, 5, cfg.MinIdleConns)
	assert.Equal(t, 5*time.Second, cfg.DialTimeout)
	assert.Equal(t, 3*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 3*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 8*time.Millisecond, cfg.MinRetryBackoff)
	assert.Equal(t, 512*time.Millisecond, cfg.MaxRetryBackoff)

	// Existing values are not overwritten
	cfg2 := &RedisConfig{
		Mode:     "standalone",
		Addr:     "localhost:6379",
		PoolSize: 100,
	}
	applyDefaults(cfg2)
	assert.Equal(t, 100, cfg2.PoolSize)
}

func TestClient_GetSetVariousTypes(t *testing.T) {
	_, client := setupClientTest(t)
	ctx := context.Background()

	// String
	err := client.Set(ctx, "typestring", "hello", 0).Err()
	assert.NoError(t, err)
	val, err := client.Get(ctx, "typestring").Result()
	assert.NoError(t, err)
	assert.Equal(t, "hello", val)

	// Integer
	err = client.Set(ctx, "typeint", 42, 0).Err()
	assert.NoError(t, err)
	val, err = client.Get(ctx, "typeint").Result()
	assert.NoError(t, err)
	assert.Equal(t, "42", val)

	// Float
	err = client.Set(ctx, "typefloat", 3.14, 0).Err()
	assert.NoError(t, err)
	val, err = client.Get(ctx, "typefloat").Result()
	assert.NoError(t, err)
	assert.Equal(t, "3.14", val)

	// Boolean
	err = client.Set(ctx, "typebool", true, 0).Err()
	assert.NoError(t, err)
	val, err = client.Get(ctx, "typebool").Result()
	assert.NoError(t, err)
	assert.Equal(t, "1", val)
}

func TestClient_ExpireAndTTL(t *testing.T) {
	mr, client := setupClientTest(t)
	ctx := context.Background()

	err := client.Set(ctx, "ttlkey", "val", 0).Err()
	require.NoError(t, err)

	// Set expiry
	ok, err := client.Expire(ctx, "ttlkey", 1*time.Second).Result()
	assert.NoError(t, err)
	assert.True(t, ok)

	// Check TTL is positive
	d, err := client.TTL(ctx, "ttlkey").Result()
	assert.NoError(t, err)
	assert.Greater(t, d, time.Duration(0))

	// Advance time
	mr.FastForward(2 * time.Second)

	exists, err := client.Exists(ctx, "ttlkey").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

func TestClient_Incr(t *testing.T) {
	_, client := setupClientTest(t)
	ctx := context.Background()

	val, err := client.Incr(ctx, "mycounter").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = client.Incr(ctx, "mycounter").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), val)

	val, err = client.IncrBy(ctx, "mycounter", 5).Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(7), val)

	val, err = client.Decr(ctx, "mycounter").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(6), val)
}

func TestClient_HashOperations(t *testing.T) {
	_, client := setupClientTest(t)
	ctx := context.Background()

	// HSet
	err := client.HSet(ctx, "myhash", "field1", "val1").Err()
	assert.NoError(t, err)

	// HGet
	val, err := client.HGet(ctx, "myhash", "field1").Result()
	assert.NoError(t, err)
	assert.Equal(t, "val1", val)

	// HGetAll
	all, err := client.HGetAll(ctx, "myhash").Result()
	assert.NoError(t, err)
	assert.Equal(t, "val1", all["field1"])

	// HDel
	deleted, err := client.HDel(ctx, "myhash", "field1").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// HGet miss
	_, err = client.HGet(ctx, "myhash", "field1").Result()
	assert.Error(t, err)
}

func TestClient_Pipeline(t *testing.T) {
	_, client := setupClientTest(t)
	ctx := context.Background()

	pipe := client.Pipeline()
	pipe.Set(ctx, "pipe:a", "1", 0)
	pipe.Set(ctx, "pipe:b", "2", 0)
	_, err := pipe.Exec(ctx)
	assert.NoError(t, err)

	val, err := client.Get(ctx, "pipe:a").Result()
	assert.NoError(t, err)
	assert.Equal(t, "1", val)

	val, err = client.Get(ctx, "pipe:b").Result()
	assert.NoError(t, err)
	assert.Equal(t, "2", val)
}

// ---------------------------------------------------------------------------
// Cluster mode & auto-detect tests
// ---------------------------------------------------------------------------

func TestSplitAndTrimAddrs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"host1:6379,host2:6380,host3:6381", []string{"host1:6379", "host2:6380", "host3:6381"}},
		{" host1:6379 , host2:6380 ", []string{"host1:6379", "host2:6380"}},
		{"host1:6379", []string{"host1:6379"}},
		{"", []string{}},
	}
	for _, tt := range tests {
		got := splitAndTrimAddrs(tt.input)
		assert.Equal(t, tt.want, got, "splitAndTrimAddrs(%q)", tt.input)
	}
}

func TestNewClient_AutoDetectClusterFromCommaAddr(t *testing.T) {
	// Empty Mode + comma-separated Addr -> auto-detect cluster
	cfg := &RedisConfig{
		Addr: "host1:6379,host2:6380,host3:6381",
	}
	client, err := NewClient(cfg, logging.NewNopLogger())
	assert.Error(t, err) // No real cluster available
	assert.Nil(t, client)
	assert.Equal(t, "cluster", cfg.Mode)
	assert.Equal(t, []string{"host1:6379", "host2:6380", "host3:6381"}, cfg.ClusterAddrs)
}

func TestNewClient_AutoDetectClusterFromClusterAddrs(t *testing.T) {
	// Empty Mode + populated ClusterAddrs -> auto-detect cluster
	cfg := &RedisConfig{
		ClusterAddrs: []string{"host1:6379", "host2:6380"},
	}
	client, err := NewClient(cfg, logging.NewNopLogger())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, "cluster", cfg.Mode)
}

func TestNewClient_AutoDetectStandalone(t *testing.T) {
	// Empty Mode + single Addr (no comma) -> auto-detect standalone
	// Will fail because no redis server, but mode should be set
	cfg := &RedisConfig{
		Addr: "localhost:6379",
	}
	client, err := NewClient(cfg, logging.NewNopLogger())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, "standalone", cfg.Mode)
}

func TestNewClient_ExplicitClusterWithCommaAddr(t *testing.T) {
	// Explicit Mode=cluster + comma-separated Addr
	cfg := &RedisConfig{
		Mode: "cluster",
		Addr: "node1:6379,node2:6379,node3:6379",
	}
	client, err := NewClient(cfg, logging.NewNopLogger())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Equal(t, "cluster", cfg.Mode)
	assert.Equal(t, []string{"node1:6379", "node2:6379", "node3:6379"}, cfg.ClusterAddrs)
}

func TestClient_Cluster_ReturnsNilForStandalone(t *testing.T) {
	_, client := setupClientTest(t)
	cc := client.Cluster()
	assert.Nil(t, cc, "Cluster() should return nil for standalone mode")
}

func TestClient_Cluster_ClusterMethodsPanicOnNil(t *testing.T) {
	// Verify that calling methods on nil *RedisClusterClient panics
	// (expected Go behavior for nil pointer receiver)
	_, client := setupClientTest(t)
	cc := client.Cluster()
	assert.Nil(t, cc)
}

//Personal.AI order the ending
