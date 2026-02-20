// Package config defines all configuration structures for the KeyIP-Intelligence
// platform.  No I/O or parsing logic lives here — only plain data types and
// validation.
package config

import (
	"fmt"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Sub-configuration structs
// ─────────────────────────────────────────────────────────────────────────────

// ServerConfig holds HTTP server tunables.
type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	Mode            string        `mapstructure:"mode"` // "debug" | "release" | "test"
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	MaxBodySize     int64         `mapstructure:"max_body_size"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// DatabaseConfig holds PostgreSQL connection parameters.
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"db_name"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxConns        int           `mapstructure:"max_conns"`
	MinConns        int           `mapstructure:"min_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	MigrationPath   string        `mapstructure:"migration_path"`
}

// Neo4jConfig holds Neo4j / knowledge-graph connection parameters.
type Neo4jConfig struct {
	URI                   string        `mapstructure:"uri"`
	User                  string        `mapstructure:"user"`
	Password              string        `mapstructure:"password"`
	MaxConnectionPoolSize int           `mapstructure:"max_connection_pool_size"`
	ConnectionTimeout     time.Duration `mapstructure:"connection_timeout"`
	Database              string        `mapstructure:"database"`
}

// RedisConfig holds Redis connection parameters.
type RedisConfig struct {
	Addr         string        `mapstructure:"addr"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	DefaultTTL   time.Duration `mapstructure:"default_ttl"`
	KeyPrefix    string        `mapstructure:"key_prefix"`
}

// KafkaConfig holds Apache Kafka producer/consumer parameters.
type KafkaConfig struct {
	Brokers           []string `mapstructure:"brokers"`
	GroupID           string   `mapstructure:"group_id"`
	AutoOffsetReset   string   `mapstructure:"auto_offset_reset"` // "earliest" | "latest"
	TimeoutMS         int      `mapstructure:"timeout_ms"`
	ProducerRetries   int      `mapstructure:"producer_retries"`
	BatchSize         int      `mapstructure:"batch_size"`
	AutoCreateTopics  bool     `mapstructure:"auto_create_topics"`
	ReplicationFactor int      `mapstructure:"replication_factor"`
	NumPartitions     int      `mapstructure:"num_partitions"`
}

// OpenSearchConfig holds OpenSearch cluster connection parameters.
type OpenSearchConfig struct {
	Addresses          []string `mapstructure:"addresses"`
	User               string   `mapstructure:"user"`
	Password           string   `mapstructure:"password"`
	InsecureSkipVerify bool     `mapstructure:"insecure_skip_verify"`
	BulkBatchSize      int      `mapstructure:"bulk_batch_size"`
	ScrollSize         int      `mapstructure:"scroll_size"`
	IndexPrefix        string   `mapstructure:"index_prefix"`
}

// MilvusConfig holds Milvus vector-store connection parameters.
type MilvusConfig struct {
	Addr               string `mapstructure:"addr"`
	DBName             string `mapstructure:"db_name"`
	EmbeddingDim       int    `mapstructure:"embedding_dim"`
	IndexType          string `mapstructure:"index_type"`
	HNSWM              int    `mapstructure:"hnsw_m"`
	HNSWEfConstruction int    `mapstructure:"hnsw_ef_construction"`
	DefaultTopK        int    `mapstructure:"default_top_k"`
	CollectionPrefix   string `mapstructure:"collection_prefix"`
}

// MinIOConfig holds MinIO / S3-compatible object-storage parameters.
type MinIOConfig struct {
	Endpoint      string        `mapstructure:"endpoint"`
	AccessKey     string        `mapstructure:"access_key"`
	SecretKey     string        `mapstructure:"secret_key"`
	Bucket        string        `mapstructure:"bucket"`
	UseSSL        bool          `mapstructure:"use_ssl"`
	PresignExpiry time.Duration `mapstructure:"presign_expiry"`
}

// KeycloakConfig holds Keycloak OIDC / OAuth2 parameters.
type KeycloakConfig struct {
	BaseURL      string        `mapstructure:"base_url"`
	Realm        string        `mapstructure:"realm"`
	ClientID     string        `mapstructure:"client_id"`
	ClientSecret string        `mapstructure:"client_secret"`
	JWKSCacheTTL time.Duration `mapstructure:"jwks_cache_ttl"`
	Audience     string        `mapstructure:"audience"`
}

// WorkerConfig holds background-worker execution parameters.
type WorkerConfig struct {
	Mode              string        `mapstructure:"mode"` // "local" | "distributed"
	Concurrency       int           `mapstructure:"concurrency"`
	QueueDepth        int           `mapstructure:"queue_depth"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	MaxRetries        int           `mapstructure:"max_retries"`
	RetryBackoffMS    time.Duration `mapstructure:"retry_backoff_ms"`
}

// LogConfig holds structured-logging parameters.
type LogConfig struct {
	Level            string `mapstructure:"level"`  // "debug" | "info" | "warn" | "error"
	Format           string `mapstructure:"format"` // "json" | "text"
	Output           string `mapstructure:"output"`
	EnableCaller     bool   `mapstructure:"enable_caller"`
	EnableStacktrace bool   `mapstructure:"enable_stacktrace"`
	SamplingRate     int    `mapstructure:"sampling_rate"`
}

// IntelligenceConfig holds AI model and inference parameters.
type IntelligenceConfig struct {
	TritonAddr         string        `mapstructure:"triton_addr"`
	ModelTimeout       time.Duration `mapstructure:"model_timeout"`
	MaxBatchSize       int           `mapstructure:"max_batch_size"`
	MolPatentGNNModel  string        `mapstructure:"molpatent_gnn_model"`
	ClaimBERTModel     string        `mapstructure:"claim_bert_model"`
	InfringeNetModel   string        `mapstructure:"infringe_net_model"`
	StrategyGPTBaseURL string        `mapstructure:"strategy_gpt_base_url"`
	StrategyGPTAPIKey  string        `mapstructure:"strategy_gpt_api_key"`
	StrategyGPTModel   string        `mapstructure:"strategy_gpt_model"`
	RagTopK            int           `mapstructure:"rag_top_k"`
}

// MultitenancyConfig holds multi-tenancy isolation parameters.
type MultitenancyConfig struct {
	EnableRLS    bool   `mapstructure:"enable_rls"`
	TenantHeader string `mapstructure:"tenant_header"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Root Config
// ─────────────────────────────────────────────────────────────────────────────

// Config is the root configuration structure for the entire platform.
// Every infrastructure component and application service reads its settings
// from the relevant sub-struct.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Neo4j        Neo4jConfig        `mapstructure:"neo4j"`
	Redis        RedisConfig        `mapstructure:"redis"`
	Kafka        KafkaConfig        `mapstructure:"kafka"`
	OpenSearch   OpenSearchConfig   `mapstructure:"opensearch"`
	Milvus       MilvusConfig       `mapstructure:"milvus"`
	MinIO        MinIOConfig        `mapstructure:"minio"`
	Keycloak     KeycloakConfig     `mapstructure:"keycloak"`
	Worker       WorkerConfig       `mapstructure:"worker"`
	Log          LogConfig          `mapstructure:"log"`
	Intelligence IntelligenceConfig `mapstructure:"intelligence"`
	Multitenancy MultitenancyConfig `mapstructure:"multitenancy"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Validation
// ─────────────────────────────────────────────────────────────────────────────

// Validate performs semantic validation of the fully-populated Config.
// It returns the first error encountered; callers should treat any error as
// fatal and refuse to start the application.
func (c *Config) Validate() error {
	// Server
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("config: server.port %d is out of range [1, 65535]", c.Server.Port)
	}
	switch c.Server.Mode {
	case "debug", "release", "test":
	default:
		return fmt.Errorf("config: server.mode %q is invalid; expected debug|release|test", c.Server.Mode)
	}

	// Database
	if c.Database.Host == "" {
		return fmt.Errorf("config: database.host is required")
	}
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("config: database.port %d is out of range [1, 65535]", c.Database.Port)
	}
	if c.Database.User == "" {
		return fmt.Errorf("config: database.user is required")
	}
	if c.Database.DBName == "" {
		return fmt.Errorf("config: database.db_name is required")
	}
	if c.Database.MaxConns < 1 {
		return fmt.Errorf("config: database.max_conns must be ≥ 1, got %d", c.Database.MaxConns)
	}

	// Redis
	if c.Redis.Addr == "" {
		return fmt.Errorf("config: redis.addr is required")
	}
	if c.Redis.DB < 0 {
		return fmt.Errorf("config: redis.db must be ≥ 0, got %d", c.Redis.DB)
	}

	// Kafka
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("config: kafka.brokers must contain at least one broker address")
	}
	if c.Kafka.GroupID == "" {
		return fmt.Errorf("config: kafka.group_id is required")
	}

	// Intelligence
	if c.Intelligence.TritonAddr == "" {
		return fmt.Errorf("config: intelligence.triton_addr is required")
	}

	// Milvus
	if c.Milvus.Addr == "" {
		return fmt.Errorf("config: milvus.addr is required")
	}

	// Worker
	if c.Worker.Concurrency < 1 {
		return fmt.Errorf("config: worker.concurrency must be ≥ 1, got %d", c.Worker.Concurrency)
	}

	// Log
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("config: log.level %q is invalid; expected debug|info|warn|error", c.Log.Level)
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		return fmt.Errorf("config: log.format %q is invalid; expected json|text", c.Log.Format)
	}

	return nil
}

//Personal.AI order the ending
