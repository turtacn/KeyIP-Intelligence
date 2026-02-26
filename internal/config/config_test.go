package config

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newValidConfig() *Config {
	return &Config{
		Server: ServerConfig{
			HTTP: HTTPConfig{
				Host: "localhost",
				Port: 8080,
			},
			GRPC: GRPCConfig{
				Port: 9090,
			},
		},
		Database: DatabaseConfig{
			Postgres: PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "password",
				DBName:   "db",
			},
			Neo4j: Neo4jConfig{
				URI:      "bolt://localhost:7687",
				User:     "neo4j",
				Password: "password",
			},
		},
		Cache: CacheConfig{
			Redis: RedisConfig{
				Addr: "localhost:6379",
			},
		},
		Search: SearchConfig{
			OpenSearch: OpenSearchConfig{
				Addresses: []string{"http://localhost:9200"},
			},
			Milvus: MilvusConfig{
				Address: "localhost",
				Port:    19530,
			},
		},
		Messaging: MessagingConfig{
			Kafka: KafkaConfig{
				Brokers:       []string{"localhost:9092"},
				ConsumerGroup: "group",
			},
		},
		Storage: StorageConfig{
			MinIO: MinIOConfig{
				Endpoint:   "localhost:9000",
				AccessKey:  "key",
				SecretKey:  "secret",
				BucketName: "bucket",
			},
		},
		Auth: AuthConfig{
			Keycloak: KeycloakConfig{
				BaseURL:      "http://localhost:8180",
				Realm:        "realm",
				ClientID:     "client",
				ClientSecret: "secret",
			},
			JWT: JWTConfig{
				Secret: "secret",
				Issuer: "issuer",
				Expiry: time.Hour,
			},
		},
		Intelligence: IntelligenceConfig{
			ModelsDir: "./models",
			MolPatentGNN: MolPatentGNNConfig{
				ModelPath: "path",
				Device:    "cpu",
			},
			ClaimBERT: ClaimBERTConfig{
				ModelPath: "path",
				Device:    "cpu",
			},
			StrategyGPT: StrategyGPTConfig{
				Endpoint:  "http://api.openai.com",
				APIKey:    "key",
				ModelName: "gpt-4",
			},
			ChemExtractor: ChemExtractorConfig{
				OCREndpoint:  "http://ocr",
				NERModelPath: "path",
			},
			InfringeNet: InfringeNetConfig{
				ModelPath: "path",
			},
		},
		Monitoring: MonitoringConfig{
			Prometheus: PrometheusConfig{
				Enabled: true,
				Port:    9091,
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
		},
	}
}

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := newValidConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingPostgresHost(t *testing.T) {
	cfg := newValidConfig()
	cfg.Database.Postgres.Host = ""
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_MissingNeo4jURI(t *testing.T) {
	cfg := newValidConfig()
	cfg.Database.Neo4j.URI = ""
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_InvalidLogLevel(t *testing.T) {
	cfg := newValidConfig()
	cfg.Monitoring.Logging.Level = "invalid"
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_InvalidPort(t *testing.T) {
	cfg := newValidConfig()
	cfg.Server.HTTP.Port = 70000
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_Validate_EmptyKafkaBrokers(t *testing.T) {
	cfg := newValidConfig()
	cfg.Messaging.Kafka.Brokers = []string{}
	err := cfg.Validate()
	assert.Error(t, err)
}

func TestConfig_PostgresDSN(t *testing.T) {
	cfg := newValidConfig()
	dsn := cfg.PostgresDSN()
	expected := "host=localhost port=5432 user=user password=password dbname=db sslmode="
	assert.Equal(t, expected, dsn)
}

func TestConfig_PostgresDSN_WithSSL(t *testing.T) {
	cfg := newValidConfig()
	cfg.Database.Postgres.SSLMode = "require"
	dsn := cfg.PostgresDSN()
	expected := "host=localhost port=5432 user=user password=password dbname=db sslmode=require"
	assert.Equal(t, expected, dsn)
}

func TestConfig_Neo4jURI(t *testing.T) {
	cfg := newValidConfig()
	assert.Equal(t, "bolt://localhost:7687", cfg.Neo4jURI())
}

func TestConfig_RedisAddr(t *testing.T) {
	cfg := newValidConfig()
	assert.Equal(t, "localhost:6379", cfg.RedisAddr())
}

func TestConfig_KafkaBrokers(t *testing.T) {
	cfg := newValidConfig()
	assert.Equal(t, []string{"localhost:9092"}, cfg.KafkaBrokers())
}

func TestConfig_IsProduction_DebugLevel(t *testing.T) {
	cfg := newValidConfig()
	cfg.Monitoring.Logging.Level = "debug"
	assert.False(t, cfg.IsProduction())
}

func TestConfig_IsProduction_InfoLevel(t *testing.T) {
	cfg := newValidConfig()
	cfg.Monitoring.Logging.Level = "info"
	assert.True(t, cfg.IsProduction())
}

func TestConfig_GetSet(t *testing.T) {
	cfg := newValidConfig()
	Set(cfg)
	retrieved := Get()
	assert.Equal(t, cfg, retrieved)
}

func TestConfig_GetSet_ConcurrentAccess(t *testing.T) {
	cfg := newValidConfig()
	Set(cfg)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Get()
		}()
	}
	wg.Wait()
}

// //Personal.AI order the ending
