package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromFile_ValidConfig(t *testing.T) {
	content := `
server:
  http:
    host: "localhost"
    port: 8080
database:
  postgres:
    host: "localhost"
    port: 5432
    user: "user"
    password: "pass"
    dbname: "db"
  neo4j:
    uri: "bolt://localhost:7687"
    user: "neo4j"
    password: "pass"
cache:
  redis:
    addr: "localhost:6379"
search:
  opensearch:
    addresses: ["http://localhost:9200"]
  milvus:
    address: "localhost"
    port: 19530
messaging:
  kafka:
    brokers: ["localhost:9092"]
    consumer_group: "group"
storage:
  minio:
    endpoint: "localhost:9000"
    access_key: "access"
    secret_key: "secret"
    bucket_name: "bucket"
auth:
  keycloak:
    base_url: "http://localhost:8180"
    realm: "realm"
    client_id: "client"
    client_secret: "secret"
  jwt:
    secret: "secret"
    issuer: "issuer"
    expiry: 24h
intelligence:
  models_dir: "./models"
  molpatent_gnn:
    model_path: "gnn.pt"
    device: "cpu"
  claim_bert:
    model_path: "bert.pt"
    device: "cpu"
  strategy_gpt:
    endpoint: "http://localhost:8080"
    api_key: "key"
    model_name: "gpt"
  chem_extractor:
    ocr_endpoint: "http://localhost:8000"
    ner_model_path: "ner.bin"
  infringe_net:
    model_path: "infringe.pt"
    threshold: 0.8
monitoring:
  logging:
    level: "info"
    format: "json"
    output: "stdout"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(WithConfigPath(configPath))
	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Server.HTTP.Host)
	assert.Equal(t, 8080, cfg.Server.HTTP.Port)
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("KEYIP_SERVER_HTTP_PORT", "9999")
	defer os.Unsetenv("KEYIP_SERVER_HTTP_PORT")

	content := `
server:
  http:
    host: "localhost"
    port: 8080
database:
  postgres:
    host: "localhost"
    port: 5432
    user: "user"
    password: "pass"
    dbname: "db"
  neo4j:
    uri: "bolt://localhost:7687"
    user: "neo4j"
    password: "pass"
cache:
  redis:
    addr: "localhost:6379"
search:
  opensearch:
    addresses: ["http://localhost:9200"]
  milvus:
    address: "localhost"
    port: 19530
messaging:
  kafka:
    brokers: ["localhost:9092"]
    consumer_group: "group"
storage:
  minio:
    endpoint: "localhost:9000"
    access_key: "access"
    secret_key: "secret"
    bucket_name: "bucket"
auth:
  keycloak:
    base_url: "http://localhost:8180"
    realm: "realm"
    client_id: "client"
    client_secret: "secret"
  jwt:
    secret: "secret"
    issuer: "issuer"
    expiry: 24h
intelligence:
  models_dir: "./models"
  molpatent_gnn:
    model_path: "gnn.pt"
    device: "cpu"
  claim_bert:
    model_path: "bert.pt"
    device: "cpu"
  strategy_gpt:
    endpoint: "http://localhost:8080"
    api_key: "key"
    model_name: "gpt"
  chem_extractor:
    ocr_endpoint: "http://localhost:8000"
    ner_model_path: "ner.bin"
  infringe_net:
    model_path: "infringe.pt"
    threshold: 0.8
monitoring:
  logging:
    level: "info"
    format: "json"
    output: "stdout"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	loadedCfg, err := Load(WithConfigPath(configPath))
	require.NoError(t, err)
	assert.Equal(t, 9999, loadedCfg.Server.HTTP.Port)
}

// //Personal.AI order the ending
