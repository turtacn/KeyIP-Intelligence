package opensearch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockLogger to capture logs
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

func newMockLogger() logging.Logger {
	return &MockLogger{}
}

func newTestServer(statusCode int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
}

func newTestConfig(addr string) ClientConfig {
	return ClientConfig{
		Addresses:      []string{addr},
		MaxRetries:     0,
		RequestTimeout: 1 * time.Second,
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := ClientConfig{
		Addresses:      []string{"http://localhost:9200"},
		RequestTimeout: 10 * time.Second,
	}
	err := ValidateConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConfig_EmptyAddresses(t *testing.T) {
	cfg := ClientConfig{
		Addresses:      []string{},
		RequestTimeout: 10 * time.Second,
	}
	err := ValidateConfig(cfg)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidConfig, err)
}

func TestValidateConfig_NegativeMaxRetries(t *testing.T) {
	cfg := ClientConfig{
		Addresses:      []string{"http://localhost:9200"},
		MaxRetries:     -1,
		RequestTimeout: 10 * time.Second,
	}
	err := ValidateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MaxRetries must be >= 0")
}

func TestValidateConfig_ZeroTimeout(t *testing.T) {
	cfg := ClientConfig{
		Addresses:      []string{"http://localhost:9200"},
		RequestTimeout: 0,
	}
	err := ValidateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RequestTimeout must be > 0")
}

func TestValidateConfig_TLSWithoutCert(t *testing.T) {
	cfg := ClientConfig{
		Addresses:      []string{"https://localhost:9200"},
		TLSEnabled:     true,
		TLSCertPath:    "",
		RequestTimeout: 10 * time.Second,
	}
	err := ValidateConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TLSCertPath required")
}

func TestNewClient_Success(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, client.IsHealthy())
	client.Close()
}

func TestNewClient_ConnectionFailed(t *testing.T) {
	server := newTestServer(http.StatusServiceUnavailable)
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.True(t, errors.Is(err, ErrConnectionFailed))
}

func TestNewClient_UnreachableAddress(t *testing.T) {
	cfg := newTestConfig("http://invalid-address:9999")
	client, err := NewClient(cfg, newMockLogger())
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestClient_Ping_Success(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	err = client.Ping(context.Background())
	assert.NoError(t, err)
	assert.True(t, client.IsHealthy())
}

func TestClient_Ping_Failure(t *testing.T) {
	// Create a server that works initially then fails
	failures := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failures > 0 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	failures = 1
	err = client.Ping(context.Background())
	assert.Error(t, err)
	assert.False(t, client.IsHealthy())
}

func TestClient_GetClient_NotNil(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	assert.NotNil(t, client.GetClient())
}

func TestClient_Close_Idempotent(t *testing.T) {
	server := newTestServer(http.StatusOK)
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)

	client.Close()
	client.Close() // Should not panic
}

//Personal.AI order the ending
