package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

func TestApplyDefaults_EmptyConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	config.ApplyDefaults(cfg)

	assert.Equal(t, config.DefaultServerPort, cfg.Server.Port)
	assert.Equal(t, config.DefaultServerMode, cfg.Server.Mode)

	assert.Equal(t, config.DefaultDBHost, cfg.Database.Host)
	assert.Equal(t, config.DefaultDBPort, cfg.Database.Port)
	assert.Equal(t, config.DefaultDBName, cfg.Database.DBName)
	assert.Equal(t, config.DefaultDBMaxConns, cfg.Database.MaxConns)
	assert.Equal(t, "disable", cfg.Database.SSLMode)

	assert.Equal(t, config.DefaultRedisAddr, cfg.Redis.Addr)
	assert.Equal(t, config.DefaultRedisDB, cfg.Redis.DB)

	assert.Equal(t, []string{config.DefaultKafkaBroker}, cfg.Kafka.Brokers)
	assert.Equal(t, config.DefaultKafkaGroupID, cfg.Kafka.GroupID)
	assert.Equal(t, "earliest", cfg.Kafka.AutoOffsetReset)

	assert.Equal(t, config.DefaultMilvusAddr, cfg.Milvus.Addr)
	assert.Equal(t, config.DefaultMinIOEndpoint, cfg.MinIO.Endpoint)

	assert.Equal(t, config.DefaultWorkerConcurrency, cfg.Worker.Concurrency)
	assert.Equal(t, "local", cfg.Worker.Mode)
	assert.Equal(t, 3, cfg.Worker.MaxRetries)

	assert.Equal(t, config.DefaultLogLevel, cfg.Log.Level)
	assert.Equal(t, config.DefaultLogFormat, cfg.Log.Format)
}

func TestApplyDefaults_DoesNotOverrideNonZeroValues(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Port = 9090
	cfg.Server.Mode = "release"
	cfg.Database.Host = "db.prod.internal"
	cfg.Database.Port = 5433
	cfg.Database.DBName = "keyip_prod"
	cfg.Database.MaxConns = 50
	cfg.Database.SSLMode = "require"
	cfg.Redis.Addr = "redis.prod.internal:6379"
	cfg.Kafka.Brokers = []string{"kafka1:9092", "kafka2:9092"}
	cfg.Kafka.GroupID = "prod-group"
	cfg.Kafka.AutoOffsetReset = "latest"
	cfg.Milvus.Addr = "milvus.prod.internal:19530"
	cfg.MinIO.Endpoint = "minio.prod.internal:9000"
	cfg.Worker.Concurrency = 20
	cfg.Worker.Mode = "distributed"
	cfg.Worker.MaxRetries = 5
	cfg.Log.Level = "warn"
	cfg.Log.Format = "text"

	config.ApplyDefaults(cfg)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "release", cfg.Server.Mode)
	assert.Equal(t, "db.prod.internal", cfg.Database.Host)
	assert.Equal(t, 5433, cfg.Database.Port)
	assert.Equal(t, "keyip_prod", cfg.Database.DBName)
	assert.Equal(t, 50, cfg.Database.MaxConns)
	assert.Equal(t, "require", cfg.Database.SSLMode)
	assert.Equal(t, "redis.prod.internal:6379", cfg.Redis.Addr)
	assert.Equal(t, []string{"kafka1:9092", "kafka2:9092"}, cfg.Kafka.Brokers)
	assert.Equal(t, "prod-group", cfg.Kafka.GroupID)
	assert.Equal(t, "latest", cfg.Kafka.AutoOffsetReset)
	assert.Equal(t, "milvus.prod.internal:19530", cfg.Milvus.Addr)
	assert.Equal(t, "minio.prod.internal:9000", cfg.MinIO.Endpoint)
	assert.Equal(t, 20, cfg.Worker.Concurrency)
	assert.Equal(t, "distributed", cfg.Worker.Mode)
	assert.Equal(t, 5, cfg.Worker.MaxRetries)
	assert.Equal(t, "warn", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
}

func TestApplyDefaults_NilConfigDoesNotPanic(t *testing.T) {
	t.Parallel()
	assert.NotPanics(t, func() { config.ApplyDefaults(nil) })
}

func TestApplyDefaults_PartialConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Server.Port = 3000      // set explicitly
	cfg.Database.User = "alice" // set explicitly; other DB fields are zero

	config.ApplyDefaults(cfg)

	// Explicitly set fields must be preserved.
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "alice", cfg.Database.User)

	// Unset fields must receive defaults.
	assert.Equal(t, config.DefaultServerMode, cfg.Server.Mode)
	assert.Equal(t, config.DefaultDBHost, cfg.Database.Host)
	assert.Equal(t, config.DefaultDBPort, cfg.Database.Port)
}

//Personal.AI order the ending
