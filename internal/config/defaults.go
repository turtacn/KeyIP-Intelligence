// Package config provides configuration loading, defaults, and validation for
// the KeyIP-Intelligence platform.
package config

import "time"

// ─────────────────────────────────────────────────────────────────────────────
// Default value constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	DefaultServerPort = 8080
	DefaultServerMode = "debug"

	DefaultDBHost     = "localhost"
	DefaultDBPort     = 5432
	DefaultDBName     = "keyip"
	DefaultDBMaxConns = 25

	DefaultRedisAddr = "localhost:6379"
	DefaultRedisDB   = 0

	DefaultKafkaBroker  = "localhost:9092"
	DefaultKafkaGroupID = "keyip-group"

	DefaultMilvusAddr = "localhost:19530"

	DefaultMinIOEndpoint = "localhost:9000"

	DefaultLogLevel  = "info"
	DefaultLogFormat = "json"

	DefaultWorkerConcurrency = 10

	DefaultTritonAddr = "localhost:8001"
)

// ─────────────────────────────────────────────────────────────────────────────
// ApplyDefaults fills zero-value fields in cfg with well-known defaults.
// It must be called after unmarshalling raw config data and before Validate()
// so that optional-but-defaulted fields are never seen as missing.
// ─────────────────────────────────────────────────────────────────────────────

// ApplyDefaults fills every zero-value field in cfg with the platform default.
// Fields that have already been set by the caller (non-zero values) are left
// unchanged so that explicit configuration always wins.
func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	// ── Server ────────────────────────────────────────────────────────────────
	if cfg.Server.Port == 0 {
		cfg.Server.Port = DefaultServerPort
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = DefaultServerMode
	}

	// ── Database ──────────────────────────────────────────────────────────────
	if cfg.Database.Host == "" {
		cfg.Database.Host = DefaultDBHost
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = DefaultDBPort
	}
	if cfg.Database.DBName == "" {
		cfg.Database.DBName = DefaultDBName
	}
	if cfg.Database.MaxConns == 0 {
		cfg.Database.MaxConns = DefaultDBMaxConns
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = DefaultRedisAddr
	}
	// DB is an int; 0 is a valid explicit value so we cannot distinguish "not
	// set" from "set to 0".  We leave it as-is (0 is also the default).

	// ── Kafka ─────────────────────────────────────────────────────────────────
	if len(cfg.Kafka.Brokers) == 0 {
		cfg.Kafka.Brokers = []string{DefaultKafkaBroker}
	}
	if cfg.Kafka.GroupID == "" {
		cfg.Kafka.GroupID = DefaultKafkaGroupID
	}
	if cfg.Kafka.AutoOffsetReset == "" {
		cfg.Kafka.AutoOffsetReset = "earliest"
	}

	// ── Milvus ────────────────────────────────────────────────────────────────
	if cfg.Milvus.Addr == "" {
		cfg.Milvus.Addr = DefaultMilvusAddr
	}

	// ── Intelligence ──────────────────────────────────────────────────────────
	if cfg.Intelligence.TritonAddr == "" {
		cfg.Intelligence.TritonAddr = DefaultTritonAddr
	}
	if cfg.Intelligence.ModelTimeout == 0 {
		cfg.Intelligence.ModelTimeout = 30 * time.Second
	}
	if cfg.Intelligence.MaxBatchSize == 0 {
		cfg.Intelligence.MaxBatchSize = 64
	}

	// ── Multitenancy ──────────────────────────────────────────────────────────
	if cfg.Multitenancy.TenantHeader == "" {
		cfg.Multitenancy.TenantHeader = "X-Tenant-ID"
	}

	// ── MinIO ─────────────────────────────────────────────────────────────────
	if cfg.MinIO.Endpoint == "" {
		cfg.MinIO.Endpoint = DefaultMinIOEndpoint
	}

	// ── Worker ────────────────────────────────────────────────────────────────
	if cfg.Worker.Concurrency == 0 {
		cfg.Worker.Concurrency = DefaultWorkerConcurrency
	}
	if cfg.Worker.Mode == "" {
		cfg.Worker.Mode = "local"
	}
	if cfg.Worker.MaxRetries == 0 {
		cfg.Worker.MaxRetries = 3
	}

	// ── Log ───────────────────────────────────────────────────────────────────
	if cfg.Log.Level == "" {
		cfg.Log.Level = DefaultLogLevel
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = DefaultLogFormat
	}
}

//Personal.AI order the ending
