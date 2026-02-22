package milvus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// Reuse MockLogger from opensearch package? No, define here or move to shared test package.
// I'll define it here to be independent.
type MockLogger struct {
	logging.Logger
}
func (m *MockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *MockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logging.Field) {}
func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *MockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *MockLogger) WithError(err error) logging.Logger { return m }
func (m *MockLogger) Sync() error { return nil }
func newMockLogger() logging.Logger { return &MockLogger{} }

// Mock Milvus Client
type mockMilvusClient struct {
	client.Client // Embed interface

	// Mock function implementations
	checkHealthFunc func(ctx context.Context) (*entity.MilvusState, error)
	getVersionFunc  func(ctx context.Context) (string, error)
	closeFunc       func() error
}

func (m *mockMilvusClient) CheckHealth(ctx context.Context) (*entity.MilvusState, error) {
	if m.checkHealthFunc != nil {
		return m.checkHealthFunc(ctx)
	}
	return &entity.MilvusState{}, nil
}

func (m *mockMilvusClient) GetVersion(ctx context.Context) (string, error) {
	if m.getVersionFunc != nil {
		return m.getVersionFunc(ctx)
	}
	return "v2.3.0", nil
}

func (m *mockMilvusClient) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func newTestMilvusConfig() ClientConfig {
	return ClientConfig{
		Address:             "localhost:19530",
		ConnectTimeout:      1 * time.Second,
		HealthCheckInterval: 100 * time.Millisecond,
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := newTestMilvusConfig()
	err := ValidateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfig_EmptyAddress(t *testing.T) {
	cfg := newTestMilvusConfig()
	cfg.Address = ""
	err := ValidateConfig(cfg)
	assert.Error(t, err)
}

func TestNewClient_Success(t *testing.T) {
	// Restore original function
	originalNewClient := milvusNewClient
	defer func() { milvusNewClient = originalNewClient }()

	// Mock NewClient
	milvusNewClient = func(ctx context.Context, conf client.Config) (client.Client, error) {
		return &mockMilvusClientCheckHealth{}, nil
	}

	cfg := newTestMilvusConfig()
	c, err := NewClient(cfg, newMockLogger())

	assert.NoError(t, err)
	assert.NotNil(t, c)
	c.Close()
}

type mockMilvusClientCheckHealth struct {
	client.Client
}

func (m *mockMilvusClientCheckHealth) CheckHealth(ctx context.Context) (*entity.MilvusState, error) {
	return &entity.MilvusState{}, nil
}
func (m *mockMilvusClientCheckHealth) Close() error { return nil }

func TestNewClient_ConnectionFailed(t *testing.T) {
	originalNewClient := milvusNewClient
	defer func() { milvusNewClient = originalNewClient }()

	milvusNewClient = func(ctx context.Context, conf client.Config) (client.Client, error) {
		return nil, errors.New("dial failed")
	}

	cfg := newTestMilvusConfig()
	c, err := NewClient(cfg, newMockLogger())
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestClient_CheckHealth_Healthy(t *testing.T) {
	mock := &mockMilvusClient{
		checkHealthFunc: func(ctx context.Context) (*entity.MilvusState, error) {
			return &entity.MilvusState{}, nil
		},
	}
	c := &Client{milvusClient: mock, healthy: atomic.Bool{}}

	err := c.CheckHealth(context.Background())
	assert.NoError(t, err)
	assert.True(t, c.IsHealthy())
}

func TestClient_CheckHealth_Error(t *testing.T) {
	mock := &mockMilvusClient{
		checkHealthFunc: func(ctx context.Context) (*entity.MilvusState, error) {
			return nil, errors.New("error")
		},
	}
	c := &Client{milvusClient: mock, logger: newMockLogger()}

	err := c.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.False(t, c.IsHealthy())
}

func TestClient_Close_Idempotent(t *testing.T) {
	mock := &mockMilvusClient{
		closeFunc: func() error { return nil },
	}
	// Note: Client.Close calls cancel()
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{milvusClient: mock, logger: newMockLogger(), cancel: cancel}

	c.Close()
	c.Close() // Should not panic
	assert.Error(t, ctx.Err()) // context canceled
}

//Personal.AI order the ending
