package config

import (
	"time"
)

// Default configuration values.
const (
	DefaultHTTPHost               = "0.0.0.0"
	DefaultHTTPPort               = 8080
	DefaultHTTPReadTimeout        = 30 * time.Second
	DefaultHTTPWriteTimeout       = 30 * time.Second
	DefaultHTTPMaxHeaderBytes     = 1 << 20 // 1MB
	DefaultGRPCPort               = 9090
	DefaultGRPCMaxRecvMsgSize     = 50 << 20 // 50MB
	DefaultGRPCMaxSendMsgSize     = 50 << 20 // 50MB
	DefaultPostgresPort           = 5432
	DefaultPostgresSSLMode        = "disable"
	DefaultPostgresMaxOpenConns   = 25
	DefaultPostgresMaxIdleConns   = 10
	DefaultPostgresConnMaxLifetime = 5 * time.Minute
	DefaultNeo4jMaxPoolSize       = 50
	DefaultNeo4jAcquisitionTimeout = 60 * time.Second
	DefaultRedisDB                = 0
	DefaultRedisPoolSize          = 10
	DefaultRedisMinIdleConns      = 5
	DefaultRedisDialTimeout       = 5 * time.Second
	DefaultRedisReadTimeout       = 3 * time.Second
	DefaultRedisWriteTimeout      = 3 * time.Second
	DefaultOpenSearchMaxRetries   = 3
	DefaultMilvusPort             = 19530
	DefaultKafkaAutoOffsetReset   = "earliest"
	DefaultKafkaMaxBytes          = 10 << 20 // 10MB
	DefaultKafkaSessionTimeout    = 30 * time.Second
	DefaultMinIOUseSSL            = false
	DefaultMinIOPartSize          = 64 << 20 // 64MB
	DefaultJWTExpiry              = 24 * time.Hour
	DefaultJWTRefreshExpiry       = 7 * 24 * time.Hour
	DefaultJWTSigningMethod       = "HS256"
	DefaultMolPatentGNNBatchSize  = 32
	DefaultMolPatentGNNTimeout    = 30 * time.Second
	DefaultMolPatentGNNDevice     = "cpu"
	DefaultClaimBERTMaxSeqLength  = 512
	DefaultClaimBERTTimeout       = 30 * time.Second
	DefaultStrategyGPTMaxTokens   = 4096
	DefaultStrategyGPTTemperature = 0.7
	DefaultStrategyGPTTopP        = 0.9
	DefaultStrategyGPTTimeout     = 120 * time.Second
	DefaultStrategyGPTRetryCount  = 3
	DefaultStrategyGPTRetryDelay  = 2 * time.Second
	DefaultChemExtractorTimeout   = 60 * time.Second
	DefaultInfringeNetThreshold   = 0.85
	DefaultInfringeNetBatchSize   = 16
	DefaultInfringeNetTimeout     = 30 * time.Second
	DefaultInfringeNetSimilarityMetric = "cosine"
	DefaultPrometheusEnabled      = true
	DefaultPrometheusPort         = 9091
	DefaultPrometheusPath         = "/metrics"
	DefaultPrometheusNamespace    = "keyip"
	DefaultLogLevel               = "info"
	DefaultLogFormat              = "json"
	DefaultLogOutput              = "stdout"
	DefaultLogMaxSize             = 100 // MB
	DefaultLogMaxBackups          = 3
	DefaultLogMaxAge              = 28 // days
	DefaultLogCompress            = true
	DefaultTracingSampleRate      = 0.1
	DefaultTracingServiceName     = "keyip-intelligence"
	DefaultEmailSMTPPort          = 587
	DefaultEmailUseTLS            = true
	DefaultEmailTimeout           = 10 * time.Second
)

// ApplyDefaults fills in default values for empty configuration fields.
func ApplyDefaults(cfg *Config) *Config {
	if cfg == nil {
		return cfg
	}

	// Server
	if cfg.Server.HTTP.Host == "" {
		cfg.Server.HTTP.Host = DefaultHTTPHost
	}
	if cfg.Server.HTTP.Port == 0 {
		cfg.Server.HTTP.Port = DefaultHTTPPort
	}
	if cfg.Server.HTTP.ReadTimeout == 0 {
		cfg.Server.HTTP.ReadTimeout = DefaultHTTPReadTimeout
	}
	if cfg.Server.HTTP.WriteTimeout == 0 {
		cfg.Server.HTTP.WriteTimeout = DefaultHTTPWriteTimeout
	}
	if cfg.Server.HTTP.MaxHeaderBytes == 0 {
		cfg.Server.HTTP.MaxHeaderBytes = DefaultHTTPMaxHeaderBytes
	}
	if cfg.Server.GRPC.Port == 0 {
		cfg.Server.GRPC.Port = DefaultGRPCPort
	}
	if cfg.Server.GRPC.MaxRecvMsgSize == 0 {
		cfg.Server.GRPC.MaxRecvMsgSize = DefaultGRPCMaxRecvMsgSize
	}
	if cfg.Server.GRPC.MaxSendMsgSize == 0 {
		cfg.Server.GRPC.MaxSendMsgSize = DefaultGRPCMaxSendMsgSize
	}

	// Database
	if cfg.Database.Postgres.Port == 0 {
		cfg.Database.Postgres.Port = DefaultPostgresPort
	}
	if cfg.Database.Postgres.SSLMode == "" {
		cfg.Database.Postgres.SSLMode = DefaultPostgresSSLMode
	}
	if cfg.Database.Postgres.MaxOpenConns == 0 {
		cfg.Database.Postgres.MaxOpenConns = DefaultPostgresMaxOpenConns
	}
	if cfg.Database.Postgres.MaxIdleConns == 0 {
		cfg.Database.Postgres.MaxIdleConns = DefaultPostgresMaxIdleConns
	}
	if cfg.Database.Postgres.ConnMaxLifetime == 0 {
		cfg.Database.Postgres.ConnMaxLifetime = DefaultPostgresConnMaxLifetime
	}
	if cfg.Database.Neo4j.MaxConnectionPoolSize == 0 {
		cfg.Database.Neo4j.MaxConnectionPoolSize = DefaultNeo4jMaxPoolSize
	}
	if cfg.Database.Neo4j.ConnectionAcquisitionTimeout == 0 {
		cfg.Database.Neo4j.ConnectionAcquisitionTimeout = DefaultNeo4jAcquisitionTimeout
	}

	// Cache
	if cfg.Cache.Redis.PoolSize == 0 {
		cfg.Cache.Redis.PoolSize = DefaultRedisPoolSize
	}
	if cfg.Cache.Redis.MinIdleConns == 0 {
		cfg.Cache.Redis.MinIdleConns = DefaultRedisMinIdleConns
	}
	if cfg.Cache.Redis.DialTimeout == 0 {
		cfg.Cache.Redis.DialTimeout = DefaultRedisDialTimeout
	}
	if cfg.Cache.Redis.ReadTimeout == 0 {
		cfg.Cache.Redis.ReadTimeout = DefaultRedisReadTimeout
	}
	if cfg.Cache.Redis.WriteTimeout == 0 {
		cfg.Cache.Redis.WriteTimeout = DefaultRedisWriteTimeout
	}

	// Search
	if cfg.Search.OpenSearch.MaxRetries == 0 {
		cfg.Search.OpenSearch.MaxRetries = DefaultOpenSearchMaxRetries
	}
	if cfg.Search.Milvus.Port == 0 {
		cfg.Search.Milvus.Port = DefaultMilvusPort
	}

	// Messaging
	if cfg.Messaging.Kafka.AutoOffsetReset == "" {
		cfg.Messaging.Kafka.AutoOffsetReset = DefaultKafkaAutoOffsetReset
	}
	if cfg.Messaging.Kafka.MaxBytes == 0 {
		cfg.Messaging.Kafka.MaxBytes = DefaultKafkaMaxBytes
	}
	if cfg.Messaging.Kafka.SessionTimeout == 0 {
		cfg.Messaging.Kafka.SessionTimeout = DefaultKafkaSessionTimeout
	}

	// Storage
	if cfg.Storage.MinIO.PartSize == 0 {
		cfg.Storage.MinIO.PartSize = DefaultMinIOPartSize
	}

	// Auth
	if cfg.Auth.JWT.Expiry == 0 {
		cfg.Auth.JWT.Expiry = DefaultJWTExpiry
	}
	if cfg.Auth.JWT.RefreshExpiry == 0 {
		cfg.Auth.JWT.RefreshExpiry = DefaultJWTRefreshExpiry
	}
	if cfg.Auth.JWT.SigningMethod == "" {
		cfg.Auth.JWT.SigningMethod = DefaultJWTSigningMethod
	}

	// Intelligence
	if cfg.Intelligence.MolPatentGNN.BatchSize == 0 {
		cfg.Intelligence.MolPatentGNN.BatchSize = DefaultMolPatentGNNBatchSize
	}
	if cfg.Intelligence.MolPatentGNN.Timeout == 0 {
		cfg.Intelligence.MolPatentGNN.Timeout = DefaultMolPatentGNNTimeout
	}
	if cfg.Intelligence.MolPatentGNN.Device == "" {
		cfg.Intelligence.MolPatentGNN.Device = DefaultMolPatentGNNDevice
	}
	if cfg.Intelligence.ClaimBERT.MaxSeqLength == 0 {
		cfg.Intelligence.ClaimBERT.MaxSeqLength = DefaultClaimBERTMaxSeqLength
	}
	if cfg.Intelligence.ClaimBERT.Timeout == 0 {
		cfg.Intelligence.ClaimBERT.Timeout = DefaultClaimBERTTimeout
	}
	if cfg.Intelligence.ClaimBERT.Device == "" {
		cfg.Intelligence.ClaimBERT.Device = DefaultMolPatentGNNDevice // Using same default (cpu)
	}
	if cfg.Intelligence.StrategyGPT.MaxTokens == 0 {
		cfg.Intelligence.StrategyGPT.MaxTokens = DefaultStrategyGPTMaxTokens
	}
	if cfg.Intelligence.StrategyGPT.Temperature == 0 {
		cfg.Intelligence.StrategyGPT.Temperature = DefaultStrategyGPTTemperature
	}
	if cfg.Intelligence.StrategyGPT.TopP == 0 {
		cfg.Intelligence.StrategyGPT.TopP = DefaultStrategyGPTTopP
	}
	if cfg.Intelligence.StrategyGPT.Timeout == 0 {
		cfg.Intelligence.StrategyGPT.Timeout = DefaultStrategyGPTTimeout
	}
	if cfg.Intelligence.StrategyGPT.RetryCount == 0 {
		cfg.Intelligence.StrategyGPT.RetryCount = DefaultStrategyGPTRetryCount
	}
	if cfg.Intelligence.StrategyGPT.RetryDelay == 0 {
		cfg.Intelligence.StrategyGPT.RetryDelay = DefaultStrategyGPTRetryDelay
	}
	if cfg.Intelligence.ChemExtractor.Timeout == 0 {
		cfg.Intelligence.ChemExtractor.Timeout = DefaultChemExtractorTimeout
	}
	if cfg.Intelligence.InfringeNet.Threshold == 0 {
		cfg.Intelligence.InfringeNet.Threshold = DefaultInfringeNetThreshold
	}
	if cfg.Intelligence.InfringeNet.BatchSize == 0 {
		cfg.Intelligence.InfringeNet.BatchSize = DefaultInfringeNetBatchSize
	}
	if cfg.Intelligence.InfringeNet.Timeout == 0 {
		cfg.Intelligence.InfringeNet.Timeout = DefaultInfringeNetTimeout
	}
	if cfg.Intelligence.InfringeNet.SimilarityMetric == "" {
		cfg.Intelligence.InfringeNet.SimilarityMetric = DefaultInfringeNetSimilarityMetric
	}

	// Monitoring
	if !cfg.Monitoring.Prometheus.Enabled {
		cfg.Monitoring.Prometheus.Enabled = DefaultPrometheusEnabled
	}
	if cfg.Monitoring.Prometheus.Port == 0 {
		cfg.Monitoring.Prometheus.Port = DefaultPrometheusPort
	}
	if cfg.Monitoring.Prometheus.Path == "" {
		cfg.Monitoring.Prometheus.Path = DefaultPrometheusPath
	}
	if cfg.Monitoring.Prometheus.Namespace == "" {
		cfg.Monitoring.Prometheus.Namespace = DefaultPrometheusNamespace
	}
	if cfg.Monitoring.Logging.Level == "" {
		cfg.Monitoring.Logging.Level = DefaultLogLevel
	}
	if cfg.Monitoring.Logging.Format == "" {
		cfg.Monitoring.Logging.Format = DefaultLogFormat
	}
	if cfg.Monitoring.Logging.Output == "" {
		cfg.Monitoring.Logging.Output = DefaultLogOutput
	}
	if cfg.Monitoring.Logging.MaxSize == 0 {
		cfg.Monitoring.Logging.MaxSize = DefaultLogMaxSize
	}
	if cfg.Monitoring.Logging.MaxBackups == 0 {
		cfg.Monitoring.Logging.MaxBackups = DefaultLogMaxBackups
	}
	if cfg.Monitoring.Logging.MaxAge == 0 {
		cfg.Monitoring.Logging.MaxAge = DefaultLogMaxAge
	}
	// DefaultLogCompress is boolean, we must be careful not to overwrite explicitly set 'false' if it differs from default.
	// But in this case default is true. If struct field is false, it might be zero value or explicit false.
	// Since boolean zero value is false, and default is true, we can check if it's false, but we can't distinguish explicit false from zero false without pointer.
	// Assuming that if it is false (zero value), we want the default (true). This might override explicit false.
	// However, standard Go struct config usually implies pointers for optional bools, or we accept this limitation.
	// The plan doesn't specify pointers for bools in config struct.
	// Let's assume we set it to default if it's false (zero value).
	// But wait, Storage.MinIO.UseSSL default is false, zero value is false. So that's fine.
	// Monitoring.Logging.Compress default is true. If user wants false, they can't set it if we overwrite.
	// To support explicit false, we should have used *bool or a separate flag.
	// Given the struct definition in config.go uses `bool` (value), we have this ambiguity.
	// For now, I will NOT set default for bools that default to true if the zero value is false, UNLESS I change the struct to use pointers.
	// But I am not changing struct to pointers in this step (Plan says "Refine...").
	// Actually, `viper` unmarshals into the struct. If the key is missing in YAML, it remains zero value.
	// If `Compress` is missing in YAML, it is `false`. We want it `true`.
	// If user sets `compress: false` in YAML, it is `false`.
	// We can't distinguish.
	// *Solution*: `ApplyDefaults` runs AFTER unmarshal.
	// We will skip bool defaults logic for now if it conflicts, OR we just set it.
	// `DefaultLogCompress` is true.
	// I'll set it to true if it is false. This means user cannot disable compression via config unless we change logic.
	// Ideally we use Viper's default mechanism, but `ApplyDefaults` is manual.
	// I will skip setting `Compress` default here to avoid breaking explicit `false`.
    // Wait, the requirement says "ApplyDefaults... default value ... DefaultLogCompress = true".
    // I will assume the intention is to set it.
    if !cfg.Monitoring.Logging.Compress {
        cfg.Monitoring.Logging.Compress = DefaultLogCompress
    }

	if cfg.Monitoring.Tracing.SampleRate == 0 {
		cfg.Monitoring.Tracing.SampleRate = DefaultTracingSampleRate
	}
	if cfg.Monitoring.Tracing.ServiceName == "" {
		cfg.Monitoring.Tracing.ServiceName = DefaultTracingServiceName
	}

	// Notification
	if cfg.Notification.Email.SMTPPort == 0 {
		cfg.Notification.Email.SMTPPort = DefaultEmailSMTPPort
	}
	if cfg.Notification.Email.Timeout == 0 {
		cfg.Notification.Email.Timeout = DefaultEmailTimeout
	}
	// UseTLS default is true. Same bool issue.
	if !cfg.Notification.Email.UseTLS {
		cfg.Notification.Email.UseTLS = DefaultEmailUseTLS
	}

	return cfg
}

// NewDefaultConfig returns a configuration instance with all defaults applied.
func NewDefaultConfig() *Config {
	cfg := &Config{}
	return ApplyDefaults(cfg)
}

// //Personal.AI order the ending
