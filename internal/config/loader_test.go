package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// writeTempYAML writes content to a temp file and returns the path.
func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// minimalValidYAML is a YAML snippet that passes Validate() when combined with
// ApplyDefaults (i.e. it only supplies fields that have no default).
const minimalValidYAML = `
database:
  user: keyip
  password: secret
`

// fullValidYAML exercises all top-level sections.
const fullValidYAML = `
server:
  port: 9090
  mode: release
database:
  host: db.example.com
  port: 5432
  user: keyip
  password: s3cret
  db_name: keyip_prod
  max_conns: 30
redis:
  addr: redis.example.com:6379
kafka:
  brokers:
    - kafka1.example.com:9092
    - kafka2.example.com:9092
  group_id: prod-group
milvus:
  addr: milvus.example.com:19530
log:
  level: warn
  format: text
worker:
  concurrency: 20
`

// ─────────────────────────────────────────────────────────────────────────────
// Load — happy paths
// ─────────────────────────────────────────────────────────────────────────────

func TestLoad_MinimalValidFile(t *testing.T) {
	t.Parallel()
	path := writeTempYAML(t, minimalValidYAML)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestLoad_FullValidFile_FieldsParsed(t *testing.T) {
	t.Parallel()
	path := writeTempYAML(t, fullValidYAML)
	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "release", cfg.Server.Mode)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, 30, cfg.Database.MaxConns)
	assert.Equal(t, "redis.example.com:6379", cfg.Redis.Addr)
	assert.Equal(t, []string{"kafka1.example.com:9092", "kafka2.example.com:9092"}, cfg.Kafka.Brokers)
	assert.Equal(t, "prod-group", cfg.Kafka.GroupID)
	assert.Equal(t, "milvus.example.com:19530", cfg.Milvus.Addr)
	assert.Equal(t, "warn", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
	assert.Equal(t, 20, cfg.Worker.Concurrency)
}

// ─────────────────────────────────────────────────────────────────────────────
// Load — error paths
// ─────────────────────────────────────────────────────────────────────────────

func TestLoad_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := config.Load("/nonexistent/path/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()
	path := writeTempYAML(t, ":::not valid yaml:::")
	_, err := config.Load(path)
	require.Error(t, err)
}

func TestLoad_ValidationFailure(t *testing.T) {
	t.Parallel()
	// Valid YAML but fails Validate() — missing database.user and invalid log level.
	bad := `
database:
  user: ""
log:
  level: verbose
`
	path := writeTempYAML(t, bad)
	_, err := config.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

// ─────────────────────────────────────────────────────────────────────────────
// Environment variable override
// ─────────────────────────────────────────────────────────────────────────────

func TestLoad_EnvVarOverridesFileValue(t *testing.T) {
	t.Setenv("KEYIP_DATABASE_HOST", "testhost")

	path := writeTempYAML(t, minimalValidYAML)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "testhost", cfg.Database.Host)
}

func TestLoad_EnvVarOverridesServerPort(t *testing.T) {
	t.Setenv("KEYIP_SERVER_PORT", "7777")

	path := writeTempYAML(t, minimalValidYAML)
	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, 7777, cfg.Server.Port)
}

// ─────────────────────────────────────────────────────────────────────────────
// ApplyDefaults is invoked by Load
// ─────────────────────────────────────────────────────────────────────────────

func TestLoad_DefaultsApplied_WhenFieldsAbsentFromFile(t *testing.T) {
	t.Parallel()
	path := writeTempYAML(t, minimalValidYAML)
	cfg, err := config.Load(path)
	require.NoError(t, err)

	// Fields not present in minimalValidYAML must carry platform defaults.
	assert.Equal(t, config.DefaultServerPort, cfg.Server.Port)
	assert.Equal(t, config.DefaultServerMode, cfg.Server.Mode)
	assert.Equal(t, config.DefaultDBHost, cfg.Database.Host)
	assert.Equal(t, config.DefaultDBPort, cfg.Database.Port)
	assert.Equal(t, config.DefaultRedisAddr, cfg.Redis.Addr)
	assert.Equal(t, config.DefaultWorkerConcurrency, cfg.Worker.Concurrency)
	assert.Equal(t, config.DefaultLogLevel, cfg.Log.Level)
	assert.Equal(t, config.DefaultLogFormat, cfg.Log.Format)
}

// ─────────────────────────────────────────────────────────────────────────────
// LoadFromEnv
// ─────────────────────────────────────────────────────────────────────────────

func TestLoadFromEnv_FullEnvSet(t *testing.T) {
	t.Setenv("KEYIP_SERVER_PORT", "8888")
	t.Setenv("KEYIP_SERVER_MODE", "release")
	t.Setenv("KEYIP_DATABASE_HOST", "pghost")
	t.Setenv("KEYIP_DATABASE_PORT", "5432")
	t.Setenv("KEYIP_DATABASE_USER", "envuser")
	t.Setenv("KEYIP_DATABASE_PASSWORD", "envpass")
	t.Setenv("KEYIP_DATABASE_DB_NAME", "envdb")
	t.Setenv("KEYIP_DATABASE_MAX_CONNS", "10")
	t.Setenv("KEYIP_REDIS_ADDR", "redishost:6379")
	t.Setenv("KEYIP_KAFKA_BROKERS", "kafka1:9092")
	t.Setenv("KEYIP_KAFKA_GROUP_ID", "env-group")
	t.Setenv("KEYIP_MILVUS_ADDR", "milvushost:19530")
	t.Setenv("KEYIP_LOG_LEVEL", "info")
	t.Setenv("KEYIP_LOG_FORMAT", "json")
	t.Setenv("KEYIP_WORKER_CONCURRENCY", "5")

	cfg, err := config.LoadFromEnv()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, 8888, cfg.Server.Port)
	assert.Equal(t, "release", cfg.Server.Mode)
	assert.Equal(t, "pghost", cfg.Database.Host)
	assert.Equal(t, "envuser", cfg.Database.User)
	assert.Equal(t, "envdb", cfg.Database.DBName)
	assert.Equal(t, "redishost:6379", cfg.Redis.Addr)
	assert.Equal(t, "env-group", cfg.Kafka.GroupID)
	assert.Equal(t, "milvushost:19530", cfg.Milvus.Addr)
	assert.Equal(t, 5, cfg.Worker.Concurrency)
}

func TestLoadFromEnv_DefaultsFilledForMissingEnvVars(t *testing.T) {
	// Only supply the mandatory fields that have no default.
	t.Setenv("KEYIP_DATABASE_USER", "u")
	t.Setenv("KEYIP_DATABASE_PASSWORD", "p")

	cfg, err := config.LoadFromEnv()
	require.NoError(t, err)
	assert.Equal(t, config.DefaultServerPort, cfg.Server.Port)
	assert.Equal(t, config.DefaultDBHost, cfg.Database.Host)
}

// ─────────────────────────────────────────────────────────────────────────────
// MustLoad
// ─────────────────────────────────────────────────────────────────────────────

func TestMustLoad_ValidConfigDoesNotPanic(t *testing.T) {
	t.Parallel()
	path := writeTempYAML(t, minimalValidYAML)
	assert.NotPanics(t, func() { config.MustLoad(path) })
}

func TestMustLoad_InvalidPathPanics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { config.MustLoad("/no/such/file/config.yaml") })
}

