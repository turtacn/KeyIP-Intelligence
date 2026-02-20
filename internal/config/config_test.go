package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

// validConfig returns a Config that passes Validate() with all required fields set.
func validConfig() *config.Config {
	cfg := &config.Config{}
	config.ApplyDefaults(cfg)
	// Fill required fields that have no default.
	cfg.Database.User = "keyip"
	cfg.Database.Password = "secret"
	return cfg
}

func TestConfig_Validate_ValidConfig(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validConfig().Validate())
}

func TestConfig_Validate_MissingDatabaseHost(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Database.Host = ""
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.host")
}

func TestConfig_Validate_MissingDatabaseUser(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Database.User = ""
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.user")
}

func TestConfig_Validate_MissingDatabaseName(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Database.DBName = ""
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.db_name")
}

func TestConfig_Validate_InvalidServerPort(t *testing.T) {
	t.Parallel()
	cases := []int{0, -1, 65536, 100000}
	for _, p := range cases {
		p := p
		t.Run("", func(t *testing.T) {
			t.Parallel()
			cfg := validConfig()
			cfg.Server.Port = p
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "server.port")
		})
	}
}

func TestConfig_Validate_InvalidServerMode(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Server.Mode = "production" // not an accepted value
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server.mode")
}

func TestConfig_Validate_InvalidDatabasePort(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Database.Port = 0
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.port")
}

func TestConfig_Validate_DatabaseMaxConnsLessThanOne(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Database.MaxConns = 0
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database.max_conns")
}

func TestConfig_Validate_MissingRedisAddr(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Redis.Addr = ""
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis.addr")
}

func TestConfig_Validate_EmptyKafkaBrokers(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Kafka.Brokers = nil
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kafka.brokers")
}

func TestConfig_Validate_MissingKafkaGroupID(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Kafka.GroupID = ""
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kafka.group_id")
}

func TestConfig_Validate_MissingMilvusAddr(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Milvus.Addr = ""
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "milvus.addr")
}

func TestConfig_Validate_WorkerConcurrencyLessThanOne(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Worker.Concurrency = 0
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worker.concurrency")
}

func TestConfig_Validate_InvalidLogLevel(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Log.Level = "verbose"
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log.level")
}

func TestConfig_Validate_InvalidLogFormat(t *testing.T) {
	t.Parallel()
	cfg := validConfig()
	cfg.Log.Format = "xml"
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "log.format")
}

func TestConfig_SubStructs_ZeroValues(t *testing.T) {
	t.Parallel()

	cfg := config.Config{}
	assert.Equal(t, 0, cfg.Server.Port)
	assert.Equal(t, "", cfg.Server.Mode)
	assert.Equal(t, "", cfg.Database.Host)
	assert.Equal(t, 0, cfg.Database.Port)
	assert.Equal(t, "", cfg.Redis.Addr)
	assert.Nil(t, cfg.Kafka.Brokers)
	assert.Equal(t, "", cfg.Milvus.Addr)
	assert.Equal(t, "", cfg.Log.Level)
	assert.Equal(t, 0, cfg.Worker.Concurrency)
}

//Personal.AI order the ending
