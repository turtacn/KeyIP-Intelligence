// Phase 12 - File #286: cmd/apiserver/main.go
// API server entry point for KeyIP-Intelligence.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	appauth "github.com/turtacn/KeyIP-Intelligence/internal/application/auth"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/molecule"
	app_patent "github.com/turtacn/KeyIP-Intelligence/internal/application/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	pg_repos "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"
	search_milvus "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/milvus"
	search_os "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/opensearch"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/grpc/services"
	csgrpc "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/grpc"
	httpserver "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http"
	h "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	httpmw "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/v1"
)

const (
	defaultConfigPath = "configs/config.yaml"
	shutdownTimeout   = 30 * time.Second
)

// shutdownStep represents a single resource to close during graceful shutdown.
type shutdownStep struct {
	name  string
	close func()
}

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logCfg := logging.LogConfig{
		Level:            logging.LevelInfo,
		Format:           cfg.Monitoring.Logging.Format,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EnableCaller:     true,
		ServiceName:      "keyip-apiserver",
	}
	if cfg.Monitoring.Logging.Level == "debug" {
		logCfg.Level = logging.LevelDebug
	}
	logger, err := logging.NewLogger(logCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	logger.Info("starting KeyIP-Intelligence API server", logging.String("version", config.Version))

	// shuttingDown is set atomically so health checks can read it concurrently.
	var shuttingDown atomic.Bool

	// --- Infrastructure Initialization ---

	// Collect resources to close in order (servers close first, DB last).
	var shutdownSteps []shutdownStep

	// Postgres
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
	pgConn, err := postgres.NewConnection(pgCfg, logger)
	if err != nil {
		logger.Fatal("failed to connect to postgres", logging.Err(err))
	}
	shutdownSteps = append(shutdownSteps, shutdownStep{name: "postgres", close: func() { pgConn.Close() }})

	// Neo4j (Optional)
	neo4jCfg := neo4j.Neo4jConfig{
		URI:                          cfg.Database.Neo4j.URI,
		Username:                     cfg.Database.Neo4j.User,
		Password:                     cfg.Database.Neo4j.Password,
		MaxConnectionPoolSize:        cfg.Database.Neo4j.MaxConnectionPoolSize,
		ConnectionAcquisitionTimeout: cfg.Database.Neo4j.ConnectionAcquisitionTimeout,
	}
	neo4jDriver, err := neo4j.NewDriver(neo4jCfg, logger)
	if err != nil {
		logger.Warn("failed to connect to neo4j (optional)", logging.Err(err))
	} else {
		shutdownSteps = append(shutdownSteps, shutdownStep{name: "neo4j", close: func() { neo4jDriver.Close() }})
	}

	// Redis (Required for some features, but we can make it optional for the minimal baseline)
	redisCfg := &redis.RedisConfig{
		Addr:         cfg.Cache.Redis.Addr,
		Password:     cfg.Cache.Redis.Password,
		DB:           cfg.Cache.Redis.DB,
		PoolSize:     cfg.Cache.Redis.PoolSize,
		MinIdleConns: cfg.Cache.Redis.MinIdleConns,
		DialTimeout:  cfg.Cache.Redis.DialTimeout,
		ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
		WriteTimeout: cfg.Cache.Redis.WriteTimeout,
	}
	redisClient, err := redis.NewClient(redisCfg, logger)
	if err != nil {
		logger.Warn("failed to connect to redis (optional)", logging.Err(err))
	} else {
		shutdownSteps = append(shutdownSteps, shutdownStep{name: "redis", close: func() { redisClient.Close() }})
	}

	// MinIO (Optional)
	minioCfg := &minio.MinIOConfig{
		Endpoint:        cfg.Storage.MinIO.Endpoint,
		AccessKeyID:     cfg.Storage.MinIO.AccessKey,
		SecretAccessKey: cfg.Storage.MinIO.SecretKey,
		UseSSL:          cfg.Storage.MinIO.UseSSL,
		DefaultBucket:   cfg.Storage.MinIO.BucketName,
		Region:          cfg.Storage.MinIO.Region,
	}
	minioClient, err := minio.NewMinIOClient(minioCfg, logger)
	if err != nil {
		logger.Warn("failed to connect to minio (optional)", logging.Err(err))
	} else {
		shutdownSteps = append(shutdownSteps, shutdownStep{name: "minio", close: func() { minioClient.Close() }})
	}

	// OpenSearch (Optional)
	osCfg := search_os.ClientConfig{
		Addresses: cfg.Search.OpenSearch.Addresses,
		Username:  cfg.Search.OpenSearch.Username,
		Password:  cfg.Search.OpenSearch.Password,
	}
	osClient, err := search_os.NewClient(osCfg, logger)
	if err != nil {
		logger.Warn("failed to connect to opensearch (optional)", logging.Err(err))
	} else {
		shutdownSteps = append(shutdownSteps, shutdownStep{name: "opensearch", close: func() { osClient.Close() }})
	}

	// Milvus (Optional)
	milvusCfg := search_milvus.ClientConfig{
		Address:  cfg.Search.Milvus.Address,
		Username: cfg.Search.Milvus.Username,
		Password: cfg.Search.Milvus.Password,
	}
	milvusClient, err := search_milvus.NewClient(milvusCfg, logger)
	if err != nil {
		logger.Warn("failed to connect to milvus (optional)", logging.Err(err))
	} else {
		shutdownSteps = append(shutdownSteps, shutdownStep{name: "milvus", close: func() { milvusClient.Close() }})
	}

	// Kafka Producer (for async tasks)
	kafkaCfg := kafka.ProducerConfig{
		Brokers:    cfg.Messaging.Kafka.Brokers,
		Acks:       "all",
		MaxRetries: 3,
	}
	kafkaProducer, err := kafka.NewProducer(kafkaCfg, logger)
	if err != nil {
		logger.Warn("failed to create kafka producer, async tasks will fail", logging.Err(err))
	} else {
		shutdownSteps = append(shutdownSteps, shutdownStep{name: "kafka", close: func() { kafkaProducer.Close() }})
	}

	// Metrics
	promCfg := prometheus.CollectorConfig{
		Namespace:            cfg.Monitoring.Prometheus.Namespace,
		EnableProcessMetrics: true,
		EnableGoMetrics:      true,
	}
	metrics, err := prometheus.NewMetricsCollector(promCfg, logger)
	if err != nil {
		logger.Error("failed to initialize metrics", logging.Err(err))
	}

	// --- Repositories ---
	moleculeRepo := pg_repos.NewPostgresMoleculeRepo(pgConn, logger)
	userRepo := pg_repos.NewPostgresUserRepo(pgConn, logger)
	patentRepo := pg_repos.NewPostgresPatentRepo(pgConn, logger)

	// --- Application Services ---
	moleculeSvc := molecule.NewService(moleculeRepo, logger)
	patentSvc := app_patent.NewService(patentRepo, logger)
	lifecycleSvc := lifecycle.NewStubTrackingService()
	portfolioSvc := portfolio.NewStubService()

	// Auth service (local JWT-based, no Keycloak required)
	jwtSecret := os.Getenv("KEYIP_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = appauth.GenerateRandomSecret()
		logger.Warn("KEYIP_JWT_SECRET not set, using random secret (tokens invalid on restart)")
	}
	authSvc := appauth.NewService(appauth.ServiceConfig{
		JWTSecret: jwtSecret,
		JWTTTL:    24 * time.Hour,
	}, userRepo, logger)

	// Minimal Similarity Search implementation
	similaritySvcDeps := patent_mining.SimilaritySearchDeps{}
	similaritySvc := patent_mining.NewSimilaritySearchService(similaritySvcDeps)

	// --- Handlers ---
	moleculeHandler := h.NewMoleculeHandler(moleculeSvc, logger)
	patentHandler := h.NewPatentHandler(patentSvc, nil, logger)
	lifecycleHandler := h.NewLifecycleHandler(lifecycleSvc, logger)
	portfolioHandler := h.NewPortfolioHandler(portfolioSvc, logger)

	healthHandler := h.NewHealthHandler(
		config.Version,
		&postgresHealthAdapter{pgConn},
		&redisHealthAdapter{redisClient},
	)

	authHandler := h.NewAuthHandler(authSvc, logger)

	// --- CORS Middleware (permissive for docker-machine dev) ---
	corsMw := httpmw.NewCORSMiddleware(httpmw.CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "Accept-Version", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           86400,
	})

	// --- Router ---
	pprofEnabled := cfg.Monitoring.Pprof.Enabled || os.Getenv("DEBUG") == "true"
	routerCfg := httpserver.RouterConfig{
		MoleculeHandler:     moleculeHandler,
		PatentHandler:       patentHandler,
		PortfolioHandler:    portfolioHandler,
		LifecycleHandler:    lifecycleHandler,
		AuthHandler:         authHandler,
		HealthHandler:       healthHandler,
		CORSMiddleware:      corsMw,
		Logger:              logger,
		MetricsCollector:    metrics,
		PprofEnabled:        pprofEnabled,
	}
	httpRouter := httpserver.NewRouter(routerCfg)

	// --- Servers ---

	// HTTP Server
	httpSrvCfg := httpserver.ServerConfig{
		Host:         cfg.Server.HTTP.Host,
		Port:         cfg.Server.HTTP.Port,
		ReadTimeout:  cfg.Server.HTTP.ReadTimeout,
		WriteTimeout: cfg.Server.HTTP.WriteTimeout,
	}
	httpServer := httpserver.NewServer(httpSrvCfg, httpRouter, logger)

	// gRPC Server
	// Build dependency health checkers for the gRPC health service.
	var healthCheckers []csgrpc.Checker
	healthCheckers = append(healthCheckers, csgrpc.NewChecker("postgres", func(ctx context.Context) error {
		return pgConn.HealthCheck(ctx)
	}))
	if redisClient != nil {
		healthCheckers = append(healthCheckers, csgrpc.NewChecker("redis", func(ctx context.Context) error {
			return redisClient.GetUnderlyingClient().Ping(ctx).Err()
		}))
	}

	grpcSrv, err := csgrpc.NewServer(&cfg.Server.GRPC,
		csgrpc.WithLogger(logger),
		csgrpc.WithHealthCheckers(healthCheckers...),
	)
	if err != nil {
		logger.Fatal("failed to create gRPC server", logging.Err(err))
	}

	// Register gRPC services - only the available vertical slices
	pb.RegisterMoleculeServiceServer(grpcSrv, services.NewMoleculeServiceServer(moleculeRepo, similaritySvc, logger))

	// Start HTTP Server
	go func() {
		if err := httpServer.Start(context.Background()); err != nil {
			logger.Fatal("HTTP server failed", logging.Err(err))
		}
	}()

	// Start gRPC Server
	go func() {
		logger.Info("gRPC server starting", logging.String("address", grpcSrv.Addr()))
		if err := grpcSrv.Start(); err != nil {
			logger.Fatal("gRPC server failed", logging.Err(err))
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shuttingDown.Store(true)
	shutdownStart := time.Now()
	logger.Info("initiating graceful shutdown with 30s timeout")

	// Mark shutting down so health checks immediately return 503.
	healthHandler.SetShuttingDown()

	// Graceful shutdown with global timeout.
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// 1. HTTP server first (stop accepting new requests)
	t := time.Now()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", logging.Err(err))
	}
	logger.Info("HTTP server stopped", logging.Duration("elapsed", time.Since(t)))

	// 2. gRPC server
	t = time.Now()
	if err := grpcSrv.Stop(ctx); err != nil {
		logger.Error("gRPC server shutdown error", logging.Err(err))
	}
	logger.Info("gRPC server stopped", logging.Duration("elapsed", time.Since(t)))

	// 3. Infrastructure resources in reverse init order (DB closed last)
	for i := len(shutdownSteps) - 1; i >= 0; i-- {
		t = time.Now()
		shutdownSteps[i].close()
		logger.Info(shutdownSteps[i].name+" connection closed",
			logging.Duration("elapsed", time.Since(t)))
	}

	logger.Info("graceful shutdown complete",
		logging.Duration("total_elapsed", time.Since(shutdownStart)))
}

// loadConfig attempts to load configuration from file.
func loadConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// return nil, fmt.Errorf("config file not found: %s", path)
		// Fallback to default for dev convenience if not found, or strict check
		return config.NewDefaultConfig(), nil
	}
	return config.LoadFromFile(path)
}

// Placeholder types to allow compilation where imports are missing/incomplete in this snippet context
// In a real scenario, these would be proper imports
// Ensuring unused imports are handled if placeholders are nil
var _ = collaboration.Service(nil)
var _ = lifecycle.Service(nil)
var _ = portfolio.Service(nil)

//Personal.AI order the ending
