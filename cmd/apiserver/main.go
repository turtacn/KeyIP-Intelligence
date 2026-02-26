package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	domain_collaboration "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	domain_lifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	domain_molecule "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	domain_patent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domain_portfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/auth/keycloak"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	neo4j_repos "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	postgres_repos "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/milvus"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/opensearch"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	interface_grpc "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/grpc"
	grpc_services "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/grpc/services"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// Wrapper logger to adapt infrastructure logger to intelligence common logger
type intelligenceLoggerAdapter struct {
	logger logging.Logger
}

func (l *intelligenceLoggerAdapter) Debug(msg string, fields ...interface{}) {
	l.logger.Debug(msg)
}
func (l *intelligenceLoggerAdapter) Info(msg string, fields ...interface{}) {
	l.logger.Info(msg)
}
func (l *intelligenceLoggerAdapter) Warn(msg string, fields ...interface{}) {
	l.logger.Warn(msg)
}
func (l *intelligenceLoggerAdapter) Error(msg string, fields ...interface{}) {
	l.logger.Error(msg)
}

func main() {
	configFile := flag.String("config", "configs/config.yaml", "Path to configuration file")
	flag.Parse()

	if envConfig := os.Getenv("KEYIP_CONFIG"); envConfig != "" {
		*configFile = envConfig
	}

	cfg, err := config.LoadFromFile(*configFile)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logLevel, err := logging.ParseLevel(cfg.Monitoring.Logging.Level)
	if err != nil {
		fmt.Printf("Invalid log level: %v\n", err)
		os.Exit(1)
	}

	logger, err := logging.NewLogger(logging.LogConfig{
		Level:       logLevel,
		Format:      cfg.Monitoring.Logging.Format,
		OutputPaths: []string{cfg.Monitoring.Logging.Output},
	})
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting KeyIP-Intelligence API Server",
		logging.String("version", Version),
		logging.String("build_time", BuildTime),
		logging.String("git_commit", GitCommit),
		logging.String("config_file", *configFile),
		logging.String("mode", "production"),
	)

	// Infrastructure
	pgCfg := postgres.PostgresConfig{
		Host:            cfg.Database.Postgres.Host,
		Port:            cfg.Database.Postgres.Port,
		Database:        cfg.Database.Postgres.DBName,
		Username:        cfg.Database.Postgres.User,
		Password:        cfg.Database.Postgres.Password,
		SSLMode:         cfg.Database.Postgres.SSLMode,
		MaxOpenConns:    cfg.Database.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Database.Postgres.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.Postgres.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.Postgres.ConnMaxIdleTime,
	}
	dbConn, err := postgres.NewConnection(pgCfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to Postgres", logging.Error(err))
	}
	defer dbConn.Close()

	neoCfg := neo4j.Neo4jConfig{
		URI:                          cfg.Database.Neo4j.URI,
		Username:                     cfg.Database.Neo4j.User,
		Password:                     cfg.Database.Neo4j.Password,
		MaxConnectionPoolSize:        cfg.Database.Neo4j.MaxConnectionPoolSize,
		ConnectionAcquisitionTimeout: cfg.Database.Neo4j.ConnectionAcquisitionTimeout,
	}
	neo4jDriver, err := neo4j.NewDriver(neoCfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to Neo4j", logging.Error(err))
	}
	defer neo4jDriver.Close()

	redisCfg := &redis.RedisConfig{
		Addr:         cfg.Cache.Redis.Addr,
		Password:     cfg.Cache.Redis.Password,
		DB:           cfg.Cache.Redis.DB,
		PoolSize:     cfg.Cache.Redis.PoolSize,
		MinIdleConns: cfg.Cache.Redis.MinIdleConns,
		DialTimeout:  cfg.Cache.Redis.DialTimeout,
		ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
		WriteTimeout: cfg.Cache.Redis.WriteTimeout,
		Mode:         "standalone",
	}
	redisClient, err := redis.NewClient(redisCfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", logging.Error(err))
	}
	defer redisClient.Close()

	minioCfg := &minio.MinIOConfig{
		Endpoint:        cfg.Storage.MinIO.Endpoint,
		AccessKeyID:     cfg.Storage.MinIO.AccessKey,
		SecretAccessKey: cfg.Storage.MinIO.SecretKey,
		UseSSL:          cfg.Storage.MinIO.UseSSL,
		Region:          cfg.Storage.MinIO.Region,
		DefaultBucket:   cfg.Storage.MinIO.BucketName,
		PartSize:        cfg.Storage.MinIO.PartSize,
	}
	minioClient, err := minio.NewMinIOClient(minioCfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to MinIO", logging.Error(err))
	}
	defer minioClient.Close()

	osCfg := opensearch.ClientConfig{
		Addresses:     cfg.Search.OpenSearch.Addresses,
		Username:      cfg.Search.OpenSearch.Username,
		Password:      cfg.Search.OpenSearch.Password,
		MaxRetries:    cfg.Search.OpenSearch.MaxRetries,
	}
	opensearchClient, err := opensearch.NewClient(osCfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to OpenSearch", logging.Error(err))
	}
	defer opensearchClient.Close()

	milvusCfg := milvus.ClientConfig{
		Address:        fmt.Sprintf("%s:%d", cfg.Search.Milvus.Address, cfg.Search.Milvus.Port),
		Username:       cfg.Search.Milvus.Username,
		Password:       cfg.Search.Milvus.Password,
		ConnectTimeout: cfg.Search.Milvus.ConnectTimeout,
		RequestTimeout: cfg.Search.Milvus.ReadTimeout,
	}
	milvusClient, err := milvus.NewClient(milvusCfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to Milvus", logging.Error(err))
	}
	defer milvusClient.Close()

	kafkaCfg := kafka.ProducerConfig{
		Brokers: cfg.Messaging.Kafka.Brokers,
	}
	kafkaProducer, err := kafka.NewProducer(kafkaCfg, logger)
	if err != nil {
		logger.Fatal("Failed to create Kafka producer", logging.Error(err))
	}
	defer kafkaProducer.Close()

	keycloakCfg := &keycloak.KeycloakConfig{
		BaseURL:          cfg.Auth.Keycloak.BaseURL,
		Realm:            cfg.Auth.Keycloak.Realm,
		ClientID:         cfg.Auth.Keycloak.ClientID,
		ClientSecret:     cfg.Auth.Keycloak.ClientSecret,
	}
	keycloakClient, err := keycloak.NewClient(keycloakCfg)
	if err != nil {
		logger.Fatal("Failed to create Keycloak client", logging.Error(err))
	}

	var metricsCollector prometheus.MetricsCollector
	if cfg.Monitoring.Prometheus.Enabled {
		promCfg := prometheus.CollectorConfig{
			Namespace: cfg.Monitoring.Prometheus.Namespace,
			Subsystem: "apiserver",
			EnableProcessMetrics: true,
			EnableGoMetrics: true,
		}
		var err error
		metricsCollector, err = prometheus.NewMetricsCollector(promCfg, logger)
		if err != nil {
			logger.Fatal("Failed to initialize Prometheus collector", logging.Error(err))
		}
	}

	// Repositories
	patentRepo := postgres_repos.NewPostgresPatentRepo(dbConn, logger)
	moleculeRepo := postgres_repos.NewPostgresMoleculeRepo(dbConn, logger)
	portfolioRepo := postgres_repos.NewPostgresPortfolioRepo(dbConn, logger)
	lifecycleRepo := postgres_repos.NewPostgresLifecycleRepo(dbConn, logger)
	userRepo := postgres_repos.NewPostgresUserRepo(dbConn, logger)

	citationRepo := neo4j_repos.NewNeo4jCitationRepo(neo4jDriver, logger)
	familyRepo := neo4j_repos.NewNeo4jFamilyRepo(neo4jDriver, logger)
	kgRepo := neo4j_repos.NewNeo4jKnowledgeGraphRepo(neo4jDriver, logger)

	redisCache := redis.NewRedisCache(redisClient, logger)
	storageRepo := minio.NewMinIORepository(minioClient, logger)

	// Domain Services
	moleculeService, _ := domain_molecule.NewMoleculeService(moleculeRepo, nil, nil, logger)
	patentService := domain_patent.NewService(patentRepo, logger)
	portfolioService := domain_portfolio.NewService(portfolioRepo, logger)
	lifecycleService := domain_lifecycle.NewService(lifecycleRepo, logger)
	collaborationService := domain_collaboration.NewService(nil, logger)

	// Intelligence Models
	loader := common.NewNoopModelLoader()
	intelMetrics := common.NewNoopIntelligenceMetrics()
	intelLogger := &intelligenceLoggerAdapter{logger: logger}
	modelRegistry, err := common.NewModelRegistry(loader, intelMetrics, intelLogger)
	if err != nil {
		logger.Fatal("Failed to create model registry", logging.Error(err))
	}

	// Application Services
	miningLogger := &miningLoggerAdapter{logger: logger}
	miningDeps := patent_mining.SimilaritySearchDeps{
		Logger: miningLogger,
	}
	miningService := patent_mining.NewSimilaritySearchService(miningDeps)

	// Using stub services for complex initializations to pass compilation
	_ = miningService

    // Using dummy variables to avoid unused variable errors if services aren't used yet
	_ = citationRepo
	_ = familyRepo
	_ = kgRepo
	_ = redisCache
	_ = storageRepo
	_ = keycloakClient
	_ = modelRegistry
	_ = metricsCollector
	_ = moleculeService
	_ = patentService
	_ = portfolioService
	_ = lifecycleService
	_ = collaborationService
	_ = userRepo

	// Handlers & Router (Placeholder)
    router := http.NewServeMux()

	// Run Servers
	go func() {
		logger.Info("Starting HTTP server", logging.String("addr", fmt.Sprintf(":%d", cfg.Server.HTTP.Port)))
        server := &http.Server{
            Addr: fmt.Sprintf(":%d", cfg.Server.HTTP.Port),
            Handler: router,
        }
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", logging.Error(err))
		}
	}()

	go func() {
		logger.Info("Starting gRPC server", logging.String("addr", fmt.Sprintf(":%d", cfg.Server.GRPC.Port)))

        lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPC.Port))
        if err != nil {
             logger.Fatal("Failed to listen for gRPC", logging.Error(err))
        }
        // grpcServer.Serve(lis)
        _ = lis

        // Placeholder for gRPC services
        _ = interface_grpc.NewServer

        // Use underscore to ignore unused imports if specific services aren't ready
        _ = grpc_services.NewPatentServiceServer
        _ = grpc_services.NewMoleculeServiceServer
	}()

	// Metrics
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		logger.Info("Starting metrics server", logging.String("addr", ":9091"))
		if err := http.ListenAndServe(":9091", nil); err != nil {
			logger.Error("Metrics server failed", logging.Error(err))
		}
	}()

	// Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("Shutting down...")

	// Ensure variables are used to avoid errors
	_ = opensearchClient
	_ = milvusClient
	_ = kafkaProducer
	_ = keycloakClient
	_ = redisClient

	logger.Info("Shutdown complete")
}

// Adapters
type miningLoggerAdapter struct {
	logger logging.Logger
}
func (l *miningLoggerAdapter) Debug(msg string, fields ...interface{}) { l.logger.Debug(msg) }
func (l *miningLoggerAdapter) Info(msg string, fields ...interface{}) { l.logger.Info(msg) }
func (l *miningLoggerAdapter) Warn(msg string, fields ...interface{}) { l.logger.Warn(msg) }
func (l *miningLoggerAdapter) Error(msg string, fields ...interface{}) { l.logger.Error(msg) }

//Personal.AI order the ending
