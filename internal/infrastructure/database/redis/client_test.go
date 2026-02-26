package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func TestNewClient_Standalone_Success(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &RedisConfig{
		Mode: "standalone",
		Addr: mr.Addr(),
	}
	log := logging.NewNopLogger()

	client, err := NewClient(cfg, log)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	err = client.Ping(context.Background())
	assert.NoError(t, err)

	client.Close()
}

func TestNewClient_Standalone_DefaultConfig(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &RedisConfig{
		Addr: mr.Addr(),
	}
	log := logging.NewNopLogger()

	client, err := NewClient(cfg, log)
	assert.NoError(t, err)

	// Defaults check? Only via reflection or checking client behavior/config struct after applyDefaults (which modifies cfg)
	assert.Equal(t, 3 * time.Second, cfg.ReadTimeout)

	client.Close()
}

func TestNewClient_Standalone_ConnectionFailed(t *testing.T) {
	cfg := &RedisConfig{
		Mode: "standalone",
		Addr: "localhost:12345", // Assuming not running
	}
	log := logging.NewNopLogger()

	client, err := NewClient(cfg, log)
	assert.Error(t, err)
	assert.Equal(t, ErrConnectionFailed, err)
	assert.Nil(t, client)
}

func TestApplyDefaults_AllZeroValues(t *testing.T) {
	cfg := &RedisConfig{}
	applyDefaults(cfg)
	assert.Greater(t, cfg.PoolSize, 0)
	assert.Equal(t, 5, cfg.MinIdleConns)
	assert.Equal(t, 3 * time.Second, cfg.ReadTimeout)
}

func TestApplyDefaults_PartialConfig(t *testing.T) {
	cfg := &RedisConfig{
		MinIdleConns: 10,
	}
	applyDefaults(cfg)
	assert.Equal(t, 10, cfg.MinIdleConns)
	assert.Equal(t, 3 * time.Second, cfg.ReadTimeout)
}

func TestClient_Operations(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &RedisConfig{Addr: mr.Addr()}
	client, err := NewClient(cfg, logging.NewNopLogger())
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Set/Get
	err = client.Set(ctx, "key", "value", 0).Err()
	assert.NoError(t, err)

	val, err := client.Get(ctx, "key").Result()
	assert.NoError(t, err)
	assert.Equal(t, "value", val)

	// Del
	err = client.Del(ctx, "key").Err()
	assert.NoError(t, err)

	err = client.Get(ctx, "key").Err()
	assert.Equal(t, redis.Nil, err)

	// Exists
	client.Set(ctx, "k2", "v2", 0)
	exists, err := client.Exists(ctx, "k2").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(1), exists)

	// Incr
	client.Set(ctx, "counter", 10, 0)
	v, err := client.Incr(ctx, "counter").Result()
	assert.NoError(t, err)
	assert.Equal(t, int64(11), v)
}

func TestClient_Close(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &RedisConfig{Addr: mr.Addr()}
	client, err := NewClient(cfg, logging.NewNopLogger())
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)

	// Operations after close should fail
	err = client.Get(context.Background(), "key").Err()
	assert.Equal(t, ErrClientClosed, err)

	// Double close
	err = client.Close()
	assert.NoError(t, err)
}

func TestBuildTLSConfig(t *testing.T) {
	cfg := &RedisConfig{TLSEnabled: false}
	tls, err := buildTLSConfig(cfg)
	assert.NoError(t, err)
	assert.Nil(t, tls)

	cfg = &RedisConfig{TLSEnabled: true, TLSInsecure: true}
	tls, err = buildTLSConfig(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, tls)
	assert.True(t, tls.InsecureSkipVerify)
}
//Personal.AI order the ending
