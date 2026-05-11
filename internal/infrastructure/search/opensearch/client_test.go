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

func (m *MockLogger) Debug(msg string, fields ...logging.Field)      {}
func (m *MockLogger) Info(msg string, fields ...logging.Field)       {}
func (m *MockLogger) Warn(msg string, fields ...logging.Field)       {}
func (m *MockLogger) Error(msg string, fields ...logging.Field)      {}
func (m *MockLogger) Fatal(msg string, fields ...logging.Field)      {}
func (m *MockLogger) With(fields ...logging.Field) logging.Logger    { return m }
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *MockLogger) WithError(err error) logging.Logger             { return m }
func (m *MockLogger) Sync() error                                    { return nil }

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

// ConnectionTest

func TestConnectionTest_Success_OpenSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"name": "os-node-1",
			"cluster_name": "keyip-cluster",
			"cluster_uuid": "abc123",
			"version": {
				"number": "2.11.0",
				"distribution": "opensearch",
				"build_type": "tar"
			},
			"tagline": "The OpenSearch Project: https://opensearch.org/"
		}`))
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	info, err := client.ConnectionTest(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "os-node-1", info.Name)
	assert.Equal(t, "keyip-cluster", info.ClusterName)
	assert.Equal(t, "2.11.0", info.Version.Number)
	assert.True(t, info.IsOpenSearch())
	assert.True(t, client.IsHealthy())
}

func TestConnectionTest_Success_Elasticsearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// ES 8.x response: no "distribution" field, no "tagline"
		w.Write([]byte(`{
			"name": "es-node-1",
			"cluster_name": "docker-cluster",
			"cluster_uuid": "def456",
			"version": {
				"number": "8.12.0",
				"build_flavor": "default",
				"build_type": "docker"
			}
		}`))
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	info, err := client.ConnectionTest(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "es-node-1", info.Name)
	assert.Equal(t, "docker-cluster", info.ClusterName)
	assert.Equal(t, "8.12.0", info.Version.Number)
	assert.False(t, info.IsOpenSearch())
	assert.True(t, client.IsHealthy())
}

func TestConnectionTest_ErrorStatus(t *testing.T) {
	// Start with a working server, then simulate failure on the second request
	failures := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failures > 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"name":"ok","cluster_name":"ok","version":{"number":"1.0"}}`))
		}
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	failures = 1
	info, err := client.ConnectionTest(context.Background())
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.False(t, client.IsHealthy())
}

func TestConnectionTest_NonJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`OK`))
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	info, err := client.ConnectionTest(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, "unknown", info.Version.Number)
	assert.True(t, client.IsHealthy())
}

func TestConnectionTest_Unreachable(t *testing.T) {
	cfg := ClientConfig{
		Addresses:      []string{"http://invalid-host:9999"},
		MaxRetries:     0,
		RequestTimeout: 100 * time.Millisecond,
	}
	client, err := NewClient(cfg, newMockLogger())
	assert.Error(t, err)
	assert.Nil(t, client)
}

// Config validation for empty credentials (ES 8.x without security plugin)
func TestValidateConfig_EmptyCredentials(t *testing.T) {
	cfg := ClientConfig{
		Addresses:      []string{"http://localhost:9200"},
		Username:       "",
		Password:       "",
		RequestTimeout: 10 * time.Second,
	}
	err := ValidateConfig(cfg)
	assert.NoError(t, err)
}

// Ping with various response types (ES 8.x compatibility)
func TestClient_Ping_Elasticsearch8xFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ES 8.x root response with no-op security
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"name": "instance-1",
			"cluster_name": "es-cluster",
			"cluster_uuid": "xyz",
			"version": {"number": "8.12.0", "build_flavor": "default"}
		}`))
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	err = client.Ping(context.Background())
	assert.NoError(t, err)
	assert.True(t, client.IsHealthy())
}

func TestClient_Ping_EmptyBody_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// No body at all
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	client, err := NewClient(cfg, newMockLogger())
	assert.NoError(t, err)
	defer client.Close()

	err = client.Ping(context.Background())
	assert.NoError(t, err)
	assert.True(t, client.IsHealthy())
}

//Personal.AI order the ending
