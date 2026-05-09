package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

func TestLoadConfig_NonExistentPathReturnsDefault(t *testing.T) {
	cfg, err := loadConfig("/tmp/nonexistent_config_file.yaml")
	require.NoError(t, err, "should return default config without error when file not found")
	require.NotNil(t, cfg, "should return non-nil config")
	// Verify defaults are applied
	assert.Equal(t, config.DefaultHTTPHost, cfg.Server.HTTP.Host)
	assert.Equal(t, config.DefaultHTTPPort, cfg.Server.HTTP.Port)
}

func TestLoadConfig_WithValidConfigFile(t *testing.T) {
	// Create a minimal valid config YAML file in a temp directory
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "test_config.yaml")

	cfgContent := `
server:
  http:
    host: "0.0.0.0"
    port: 8080
  grpc:
    port: 9090
database:
  postgres:
    host: "localhost"
    port: 5432
    user: "test_user"
    password: "test_pass"
    dbname: "test_db"
  neo4j:
    uri: "bolt://localhost:7687"
    user: "neo4j"
    password: "test_pass"
cache:
  redis:
    addr: "localhost:6379"
search:
  opensearch:
    addresses: ["http://localhost:9200"]
  milvus:
    address: "localhost:19530"
    port: 19530
messaging:
  kafka:
    brokers: ["localhost:9092"]
    consumer_group: "test-group"
storage:
  minio:
    endpoint: "localhost:9000"
    access_key: "minioadmin"
    secret_key: "minioadmin"
    bucket_name: "test-bucket"
auth:
  keycloak:
    base_url: "http://localhost:8080"
    realm: "keyip"
    client_id: "keyip-backend"
    client_secret: "test-secret"
  jwt:
    secret: "test-secret-key-change-in-production"
    issuer: "keyip"
    expiry: 1h
intelligence:
  models_dir: "/tmp/models"
  molpatent_gnn:
    model_path: "/tmp/models/molpatent_gnn.pt"
    device: "cpu"
  claim_bert:
    model_path: "/tmp/models/claim_bert.pt"
    device: "cpu"
  strategy_gpt:
    endpoint: "http://localhost:11434/v1"
    api_key: "test-key"
    model_name: "llama3"
  chem_extractor:
    ocr_endpoint: "http://localhost:5000/ocr"
    ner_model_path: "/tmp/models/chem_ner.pt"
  infringe_net:
    model_path: "/tmp/models/infringe_net.pt"
    threshold: 0.5
monitoring:
  logging:
    level: "info"
    format: "json"
    output: "stdout"
  prometheus:
    namespace: "keyip"
`

	err := os.WriteFile(cfgPath, []byte(cfgContent), 0644)
	require.NoError(t, err)

	cfg, err := loadConfig(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, 8080, cfg.Server.HTTP.Port)
	assert.Equal(t, "localhost:6379", cfg.Cache.Redis.Addr)
	assert.Equal(t, "test-group", cfg.Messaging.Kafka.ConsumerGroup)
}

func TestLoadConfig_EmptyPathReturnsDefault(t *testing.T) {
	cfg, err := loadConfig("")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "dev", config.Version) // Verify config package is accessible
}

// --- Health Adapter Tests ---

func TestPostgresHealthAdapter_Name(t *testing.T) {
	// We can't easily create a *postgres.Connection without a real database,
	// but we can verify the Name() method returns the expected value
	adapter := &postgresHealthAdapter{}
	assert.Equal(t, "postgres", adapter.Name())
}

func TestRedisHealthAdapter_Name(t *testing.T) {
	adapter := &redisHealthAdapter{}
	assert.Equal(t, "redis", adapter.Name())
}

func TestPostgresHealthAdapter_CheckWithNilConnection(t *testing.T) {
	adapter := &postgresHealthAdapter{}
	// Should panic or return error depending on implementation
	// The Check method accesses a.conn.HealthCheck(ctx), so with nil conn it panics.
	// This test is here for documentation purposes; in practice,
	// these adapters are only created with valid connections.
	assert.Panics(t, func() {
		adapter.Check(context.Background())
	}, "nil connection should cause panic")
}

func TestRedisHealthAdapter_CheckWithNilClient(t *testing.T) {
	adapter := &redisHealthAdapter{}
	// The Check method accesses a.client.GetUnderlyingClient() which panics on nil
	// Only called with valid clients in practice
	assert.Panics(t, func() {
		adapter.Check(context.Background())
	}, "nil client should cause panic")
}

// --- Global Placeholders Test ---

func TestGlobalPlaceholderVarsCompile(t *testing.T) {
	// This test verifies that the package-level placeholder variables
	// exist and can be referenced (ensuring they compile)
	assert.NotNil(t, loadConfig)    // function exists
	assert.NotNil(t, main)          // main exists
}
