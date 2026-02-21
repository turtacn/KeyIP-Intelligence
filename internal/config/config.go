// Package config defines the global configuration structure and validation rules.
package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
)

// Config is the root configuration structure.
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Cache        CacheConfig        `mapstructure:"cache"`
	Search       SearchConfig       `mapstructure:"search"`
	Messaging    MessagingConfig    `mapstructure:"messaging"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Auth         AuthConfig         `mapstructure:"auth"`
	Intelligence IntelligenceConfig `mapstructure:"intelligence"`
	Monitoring   MonitoringConfig   `mapstructure:"monitoring"`
	Notification NotificationConfig `mapstructure:"notification"`
}

// ServerConfig holds server-related settings.
type ServerConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
	GRPC GRPCConfig `mapstructure:"grpc"`
}

type HTTPConfig struct {
	Host           string        `mapstructure:"host" validate:"required"`
	Port           int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxHeaderBytes int           `mapstructure:"max_header_bytes"`
}

type GRPCConfig struct {
	Port           int `mapstructure:"port" validate:"required,min=1,max=65535"`
	MaxRecvMsgSize int `mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize int `mapstructure:"max_send_msg_size"`
}

// DatabaseConfig holds database connection parameters.
type DatabaseConfig struct {
	Postgres PostgresConfig `mapstructure:"postgres"`
	Neo4j    Neo4jConfig    `mapstructure:"neo4j"`
}

type PostgresConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	User            string        `mapstructure:"user" validate:"required"`
	Password        string        `mapstructure:"password" validate:"required"`
	DBName          string        `mapstructure:"dbname" validate:"required"`
	SSLMode         string        `mapstructure:"sslmode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type Neo4jConfig struct {
	URI                          string        `mapstructure:"uri" validate:"required"`
	User                         string        `mapstructure:"user" validate:"required"`
	Password                     string        `mapstructure:"password" validate:"required"`
	MaxConnectionPoolSize        int           `mapstructure:"max_connection_pool_size"`
	ConnectionAcquisitionTimeout time.Duration `mapstructure:"connection_acquisition_timeout"`
}

// CacheConfig holds cache-related settings.
type CacheConfig struct {
	Redis RedisConfig `mapstructure:"redis"`
}

type RedisConfig struct {
	Addr         string        `mapstructure:"addr" validate:"required"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// SearchConfig holds search engine settings.
type SearchConfig struct {
	OpenSearch OpenSearchConfig `mapstructure:"opensearch"`
	Milvus     MilvusConfig     `mapstructure:"milvus"`
}

type OpenSearchConfig struct {
	Addresses             []string `mapstructure:"addresses" validate:"required,min=1"`
	Username              string   `mapstructure:"username"`
	Password              string   `mapstructure:"password"`
	MaxRetries            int      `mapstructure:"max_retries"`
	RetryOnStatus         []int    `mapstructure:"retry_on_status"`
	CompressRequestBody   bool     `mapstructure:"compress_request_body"`
}

type MilvusConfig struct {
	Address        string        `mapstructure:"address" validate:"required"`
	Port           int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	ConnectTimeout time.Duration `mapstructure:"connect_timeout"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
}

// MessagingConfig holds messaging settings.
type MessagingConfig struct {
	Kafka KafkaConfig `mapstructure:"kafka"`
}

type KafkaConfig struct {
	Brokers          []string      `mapstructure:"brokers" validate:"required,min=1"`
	ConsumerGroup    string        `mapstructure:"consumer_group" validate:"required"`
	AutoOffsetReset  string        `mapstructure:"auto_offset_reset"`
	MaxBytes         int           `mapstructure:"max_bytes"`
	SessionTimeout   time.Duration `mapstructure:"session_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	RebalanceTimeout time.Duration `mapstructure:"rebalance_timeout"`
}

// StorageConfig holds object storage settings.
type StorageConfig struct {
	MinIO MinIOConfig `mapstructure:"minio"`
}

type MinIOConfig struct {
	Endpoint   string `mapstructure:"endpoint" validate:"required"`
	AccessKey  string `mapstructure:"access_key" validate:"required"`
	SecretKey  string `mapstructure:"secret_key" validate:"required"`
	UseSSL     bool   `mapstructure:"use_ssl"`
	BucketName string `mapstructure:"bucket_name" validate:"required"`
	Region     string `mapstructure:"region"`
	PartSize   int64  `mapstructure:"part_size"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Keycloak KeycloakConfig `mapstructure:"keycloak"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

type KeycloakConfig struct {
	BaseURL          string `mapstructure:"base_url" validate:"required,url"`
	Realm            string `mapstructure:"realm" validate:"required"`
	ClientID         string `mapstructure:"client_id" validate:"required"`
	ClientSecret     string `mapstructure:"client_secret" validate:"required"`
	AdminUser        string `mapstructure:"admin_user"`
	AdminPassword    string `mapstructure:"admin_password"`
	TokenEndpoint    string `mapstructure:"token_endpoint"`
	UserInfoEndpoint string `mapstructure:"userinfo_endpoint"`
}

type JWTConfig struct {
	Secret         string        `mapstructure:"secret" validate:"required"`
	Issuer         string        `mapstructure:"issuer" validate:"required"`
	Expiry         time.Duration `mapstructure:"expiry" validate:"required"`
	RefreshExpiry  time.Duration `mapstructure:"refresh_expiry"`
	SigningMethod  string        `mapstructure:"signing_method"`
}

// IntelligenceConfig holds AI engine settings.
type IntelligenceConfig struct {
	ModelsDir     string             `mapstructure:"models_dir" validate:"required"`
	MolPatentGNN  MolPatentGNNConfig `mapstructure:"molpatent_gnn"`
	ClaimBERT     ClaimBERTConfig    `mapstructure:"claim_bert"`
	StrategyGPT   StrategyGPTConfig  `mapstructure:"strategy_gpt"`
	ChemExtractor ChemExtractorConfig `mapstructure:"chem_extractor"`
	InfringeNet   InfringeNetConfig  `mapstructure:"infringe_net"`
}

type MolPatentGNNConfig struct {
	ModelPath string        `mapstructure:"model_path" validate:"required"`
	BatchSize int           `mapstructure:"batch_size"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Device    string        `mapstructure:"device" validate:"oneof=cpu cuda"`
	NumWorkers int          `mapstructure:"num_workers"`
}

type ClaimBERTConfig struct {
	ModelPath    string        `mapstructure:"model_path" validate:"required"`
	MaxSeqLength int           `mapstructure:"max_seq_length"`
	Timeout      time.Duration `mapstructure:"timeout"`
	Device       string        `mapstructure:"device" validate:"oneof=cpu cuda"`
	VocabPath    string        `mapstructure:"vocab_path"`
}

type StrategyGPTConfig struct {
	Endpoint    string        `mapstructure:"endpoint" validate:"required,url"`
	APIKey      string        `mapstructure:"api_key" validate:"required"`
	ModelName   string        `mapstructure:"model_name" validate:"required"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float64       `mapstructure:"temperature"`
	TopP        float64       `mapstructure:"top_p"`
	Timeout     time.Duration `mapstructure:"timeout"`
	RetryCount  int           `mapstructure:"retry_count"`
	RetryDelay  time.Duration `mapstructure:"retry_delay"`
}

type ChemExtractorConfig struct {
	OCREndpoint         string        `mapstructure:"ocr_endpoint" validate:"required,url"`
	NERModelPath        string        `mapstructure:"ner_model_path" validate:"required"`
	ImagePreprocessing  bool          `mapstructure:"image_preprocessing"`
	SupportedFormats    []string      `mapstructure:"supported_formats"`
	Timeout             time.Duration `mapstructure:"timeout"`
}

type InfringeNetConfig struct {
	ModelPath        string        `mapstructure:"model_path" validate:"required"`
	Threshold        float64       `mapstructure:"threshold" validate:"min=0,max=1"`
	BatchSize        int           `mapstructure:"batch_size"`
	Timeout          time.Duration `mapstructure:"timeout"`
	SimilarityMetric string        `mapstructure:"similarity_metric"`
}

// MonitoringConfig holds monitoring and logging settings.
type MonitoringConfig struct {
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Tracing    TracingConfig    `mapstructure:"tracing"`
}

type PrometheusConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	Port      int    `mapstructure:"port" validate:"required_if=Enabled true"`
	Path      string `mapstructure:"path"`
	Namespace string `mapstructure:"namespace"`
}

type LoggingConfig struct {
	Level      string `mapstructure:"level" validate:"oneof=debug info warn error"`
	Format     string `mapstructure:"format" validate:"oneof=json text"`
	Output     string `mapstructure:"output" validate:"oneof=stdout file"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	Endpoint    string  `mapstructure:"endpoint"`
	SampleRate  float64 `mapstructure:"sample_rate"`
	ServiceName string  `mapstructure:"service_name"`
	Environment string  `mapstructure:"environment"`
}

// NotificationConfig holds notification settings.
type NotificationConfig struct {
	Email      EmailConfig      `mapstructure:"email"`
	WeChatWork WeChatWorkConfig `mapstructure:"wechat_work"`
}

type EmailConfig struct {
	SMTPHost string        `mapstructure:"smtp_host"`
	SMTPPort int           `mapstructure:"smtp_port"`
	Username string        `mapstructure:"username"`
	Password string        `mapstructure:"password"`
	From     string        `mapstructure:"from"`
	UseTLS   bool          `mapstructure:"use_tls"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

type WeChatWorkConfig struct {
	CorpID         string `mapstructure:"corp_id"`
	AgentID        string `mapstructure:"agent_id"`
	Secret         string `mapstructure:"secret"`
	Token          string `mapstructure:"token"`
	EncodingAESKey string `mapstructure:"encoding_aes_key"`
}

// Versioning information injected at build time.
var (
	Version   = "dev"
	CommitSHA = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

// Global configuration access.
var (
	globalConfig *Config
	configOnce   sync.Once
	configMu     sync.RWMutex
	validate     = validator.New()
)

// Get returns the global configuration instance.
func Get() *Config {
	configOnce.Do(func() {
		configMu.Lock()
		defer configMu.Unlock()
		if globalConfig == nil {
			globalConfig = NewDefaultConfig()
		}
	})

	configMu.RLock()
	defer configMu.RUnlock()
	return globalConfig
}

// Set sets the global configuration instance.
func Set(cfg *Config) {
	configMu.Lock()
	defer configMu.Unlock()
	globalConfig = cfg
}

// Validate performs validation on the configuration.
func (c *Config) Validate() error {
	return validate.Struct(c)
}

// PostgresDSN returns the PostgreSQL connection string.
func (c *Config) PostgresDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Postgres.Host,
		c.Database.Postgres.Port,
		c.Database.Postgres.User,
		c.Database.Postgres.Password,
		c.Database.Postgres.DBName,
		c.Database.Postgres.SSLMode,
	)
}

// Neo4jURI returns the Neo4j connection URI.
func (c *Config) Neo4jURI() string {
	return c.Database.Neo4j.URI
}

// RedisAddr returns the Redis address.
func (c *Config) RedisAddr() string {
	return c.Cache.Redis.Addr
}

// KafkaBrokers returns the Kafka brokers list.
func (c *Config) KafkaBrokers() []string {
	return c.Messaging.Kafka.Brokers
}

// IsProduction returns true if the environment is production (log level info/warn/error).
func (c *Config) IsProduction() bool {
	return c.Monitoring.Logging.Level != "debug"
}

// //Personal.AI order the ending
