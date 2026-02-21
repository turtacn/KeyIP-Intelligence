package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
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
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	cfg := &RedisConfig{Mode: "standalone", Addr: mr.Addr()}
	client, err := NewClient(cfg, logging.NewNopLogger())
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()

	// Get/Set
	err = client.Set(ctx, "foo", "bar", 0).Err()
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
}

//Personal.AI order the ending
