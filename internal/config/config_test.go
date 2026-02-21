package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newValidConfig() *Config {
	cfg := NewDefaultConfig()
	cfg.Server.HTTP.Host = "localhost"
	cfg.Server.HTTP.Port = 8080
	cfg.Server.GRPC.Port = 9090
	cfg.Database.Postgres.Host = "localhost"
	cfg.Database.Postgres.Port = 5432
	cfg.Database.Postgres.User = "user"
	cfg.Database.Postgres.Password = "pass"
	cfg.Database.Postgres.DBName = "db"
	cfg.Database.Neo4j.URI = "bolt://localhost:7687"
	cfg.Database.Neo4j.User = "neo4j"
	cfg.Database.Neo4j.Password = "pass"
	cfg.Cache.Redis.Addr = "localhost:6379"
	cfg.Search.OpenSearch.Addresses = []string{"http://localhost:9200"}
	cfg.Search.Milvus.Address = "localhost"
	cfg.Search.Milvus.Port = 19530
	cfg.Messaging.Kafka.Brokers = []string{"localhost:9092"}
	cfg.Messaging.Kafka.ConsumerGroup = "group"
	cfg.Storage.MinIO.Endpoint = "localhost:9000"
	cfg.Storage.MinIO.AccessKey = "access"
	cfg.Storage.MinIO.SecretKey = "secret"
	cfg.Storage.MinIO.BucketName = "bucket"
	cfg.Auth.Keycloak.BaseURL = "http://localhost:8180"
	cfg.Auth.Keycloak.Realm = "realm"
	cfg.Auth.Keycloak.ClientID = "client"
	cfg.Auth.Keycloak.ClientSecret = "secret"
	cfg.Auth.JWT.Secret = "secret"
	cfg.Auth.JWT.Issuer = "issuer"
	cfg.Auth.JWT.Expiry = 24 * time.Hour
	cfg.Intelligence.ModelsDir = "./models"
	cfg.Intelligence.MolPatentGNN.ModelPath = "gnn.pt"
	cfg.Intelligence.MolPatentGNN.Device = "cpu"
	cfg.Intelligence.ClaimBERT.ModelPath = "bert.pt"
	cfg.Intelligence.ClaimBERT.Device = "cpu"
	cfg.Intelligence.StrategyGPT.Endpoint = "http://localhost:8080"
	cfg.Intelligence.StrategyGPT.APIKey = "key"
	cfg.Intelligence.StrategyGPT.ModelName = "gpt"
	cfg.Intelligence.ChemExtractor.OCREndpoint = "http://localhost:8000"
	cfg.Intelligence.ChemExtractor.NERModelPath = "ner.bin"
	cfg.Intelligence.InfringeNet.ModelPath = "infringe.pt"
	cfg.Intelligence.InfringeNet.Threshold = 0.8
	return cfg
}

func TestConfig_Validate(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := newValidConfig()
		assert.NoError(t, cfg.Validate())
	})

	t.Run("MissingPostgresHost", func(t *testing.T) {
		cfg := newValidConfig()
		cfg.Database.Postgres.Host = ""
		assert.Error(t, cfg.Validate())
	})
}

func TestConfig_PostgresDSN(t *testing.T) {
	cfg := newValidConfig()
	cfg.Database.Postgres.Host = "myhost"
	dsn := cfg.PostgresDSN()
	assert.Contains(t, dsn, "host=myhost")
}

// //Personal.AI order the ending
