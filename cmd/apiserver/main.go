// Phase 12 - File #286: cmd/apiserver/main.go
// 生成计划:
// ---
// 继续输出 286 `cmd/apiserver/main.go` 要实现 API 服务器入口程序。
//
// 实现要求:
//
// * **功能定位**：KeyIP-Intelligence HTTP/gRPC API 服务器的主入口，负责配置加载、依赖组装、服务器启动与优雅关闭
// * **核心实现**：
//   * 定义 main 函数：解析命令行参数（--config 路径、--port 端口）
//   * 实现配置加载：调用 internal/config.Load() 加载 YAML + 环境变量
//   * 实现日志初始化：基于配置创建 Logger 实例
//   * 实现基础设施初始化：按顺序初始化 PostgreSQL、Neo4j、Redis、OpenSearch、Milvus、Kafka、MinIO、Keycloak 客户端
//   * 实现仓储层组装：将基础设施客户端注入到各仓储实现
//   * 实现领域服务组装：将仓储注入到领域服务
//   * 实现智能层组装：初始化 AI 引擎（ModelRegistry、各模型加载）
//   * 实现应用层组装：将领域服务、智能层、仓储注入到应用服务
//   * 实现接口层组装：将应用服务注入到 HTTP Handler 和 gRPC Service
//   * 实现 HTTP 服务器启动：路由注册、中间件链、监听端口
//   * 实现 gRPC 服务器启动：服务注册、拦截器链、监听端口
//   * 实现 Prometheus metrics 端点暴露
//   * 实现优雅关闭：监听 SIGINT/SIGTERM，依次关闭 HTTP→gRPC→Kafka→数据库连接
//   * 实现健康检查端点注册
// * **业务逻辑**：
//   * 启动顺序：配置→日志→基础设施→仓储→领域→智能→应用→接口→服务器
//   * 关闭顺序：服务器→接口→应用→基础设施（反向依赖顺序）
//   * 启动失败任一组件应 fatal 退出并输出明确错误
//   * 支持 --config 指定配置文件路径，默认 configs/config.yaml
// * **依赖关系**：
//   * 依赖：internal/config、internal/infrastructure/*、internal/domain/*、internal/intelligence/*、internal/application/*、internal/interfaces/*
//   * 被依赖：Dockerfile.apiserver、Makefile
// * **测试要求**：入口程序不做单元测试，通过 E2E 测试覆盖
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
// ---
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"

	// Database clients
	pgconn "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	neo4jdriver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	redisclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"

	// Storage
	minioclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"

	// Search
	opensearchclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/opensearch"
	milvusclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/milvus"

	// Messaging
	kafkaproducer "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"

	// Auth
	keycloakclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/auth/keycloak"

	// Repositories
	pgrepo "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	neo4jrepo "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j/repositories"

	// Domain services
	moleculedomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	patentdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	portfoliodomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	lifecycledomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	collaborationdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"

	// Intelligence
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"

	// Application services
	appmining "github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	appinfringement "github.com/turtacn/KeyIP-Intelligence/internal/application/infringement"
	appportfolio "github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	applifecycle "github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	appcollaboration "github.com/turtacn/KeyIP-Intelligence/internal/application/collaboration"
	appquery "github.com/turtacn/KeyIP-Intelligence/internal/application/query"
	appreporting "github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"

	// Interfaces
	httpserver "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http"
	grpcserver "github.com/turtacn/KeyIP-Intelligence/internal/interfaces/grpc"
)

const (
	defaultConfigPath  = "configs/config.yaml"
	defaultHTTPPort    = 8080
	defaultGRPCPort    = 9090
	defaultMetricsPort = 9100
	shutdownTimeout    = 30 * time.Second
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", defaultConfigPath, "path to configuration file")
	httpPort := flag.Int("http-port", 0, "HTTP server port (overrides config)")
	grpcPort := flag.Int("grpc-port", 0, "gRPC server port (overrides config)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Apply command-line overrides
	if *httpPort > 0 {
		cfg.Server.HTTPPort = *httpPort
	}
	if cfg.Server.HTTPPort == 0 {
		cfg.Server.HTTPPort = defaultHTTPPort
	}
	if *grpcPort > 0 {
		cfg.Server.GRPCPort = *grpcPort
	}
	if cfg.Server.GRPCPort == 0 {
		cfg.Server.GRPCPort = defaultGRPCPort
	}
	if cfg.Server.MetricsPort == 0 {
		cfg.Server.MetricsPort = defaultMetricsPort
	}

	// Initialize logger
	logger, err := logging.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	logger.Info("starting KeyIP-Intelligence API server",
		"version", cfg.Version,
		"http_port", cfg.Server.HTTPPort,
		"grpc_port", cfg.Server.GRPCPort,
	)

	// Initialize Prometheus metrics
	metricsCollector := prometheus.NewCollector(cfg.Metrics)
	metricsCollector.Register()

	// Initialize infrastructure components
	infra, err := initInfrastructure(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize infrastructure", "error", err)
		os.Exit(1)
	}
	defer infra.Close()

	// Assemble repositories
	repos := assembleRepositories(infra, logger)

	// Assemble domain services
	domainServices := assembleDomainServices(repos, logger)

	// Initialize intelligence layer
	modelRegistry, err := initIntelligence(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize intelligence layer", "error", err)
		os.Exit(1)
	}

	// Assemble application services
	appServices := assembleApplicationServices(cfg, domainServices, repos, modelRegistry, infra, logger)

	// Create HTTP server
	httpRouter := httpserver.NewRouter(cfg, appServices, logger, metricsCollector)
	httpSrv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler:      httpRouter,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Create gRPC server
	grpcSrv := grpcserver.NewServer(cfg, appServices, logger, metricsCollector)
	if cfg.Server.GRPCReflection {
		reflection.Register(grpcSrv.Server())
	}

	// Start metrics server
	metricsSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.MetricsPort),
		Handler: metricsCollector.Handler(),
	}
	go func() {
		logger.Info("metrics server listening", "port", cfg.Server.MetricsPort)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	// Start HTTP server
	go func() {
		logger.Info("HTTP server listening", "port", cfg.Server.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		logger.Error("failed to listen for gRPC", "error", err)
		os.Exit(1)
	}
	go func() {
		logger.Info("gRPC server listening", "port", cfg.Server.GRPCPort)
		if err := grpcSrv.Server().Serve(grpcListener); err != nil {
			logger.Error("gRPC server error", "error", err)
		}
	}()

	logger.Info("KeyIP-Intelligence API server started successfully")

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("received shutdown signal", "signal", sig.String())

	// Graceful shutdown
	shutdown(httpSrv, grpcSrv.Server(), metricsSrv, infra, logger)
}

// infrastructure holds all initialized infrastructure clients.
type infrastructure struct {
	pg        *pgconn.Connection
	neo4j     *neo4jdriver.Driver
	redis     *redisclient.Client
	minio     *minioclient.Client
	opensearch *opensearchclient.Client
	milvus    *milvusclient.Client
	kafka     *kafkaproducer.Producer
	keycloak  *keycloakclient.Client
}

func (i *infrastructure) Close() {
	if i.kafka != nil {
		i.kafka.Close()
	}
	if i.milvus != nil {
		i.milvus.Close()
	}
	if i.opensearch != nil {
		i.opensearch.Close()
	}
	if i.redis != nil {
		i.redis.Close()
	}
	if i.neo4j != nil {
		i.neo4j.Close()
	}
	if i.pg != nil {
		i.pg.Close()
	}
	if i.minio != nil {
		i.minio.Close()
	}
}

func initInfrastructure(cfg *config.Config, logger logging.Logger) (*infrastructure, error) {
	infra := &infrastructure{}

	// PostgreSQL
	pg, err := pgconn.NewConnection(cfg.Database.Postgres)
	if err != nil {
		return nil, fmt.Errorf("postgres connection: %w", err)
	}
	infra.pg = pg
	logger.Info("PostgreSQL connected")

	// Neo4j
	neo4jDrv, err := neo4jdriver.NewDriver(cfg.Database.Neo4j)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("neo4j connection: %w", err)
	}
	infra.neo4j = neo4jDrv
	logger.Info("Neo4j connected")

	// Redis
	redisCli, err := redisclient.NewClient(cfg.Database.Redis)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("redis connection: %w", err)
	}
	infra.redis = redisCli
	logger.Info("Redis connected")

	// MinIO
	minioCli, err := minioclient.NewClient(cfg.Storage.MinIO)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("minio connection: %w", err)
	}
	infra.minio = minioCli
	logger.Info("MinIO connected")

	// OpenSearch
	osCli, err := opensearchclient.NewClient(cfg.Search.OpenSearch)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("opensearch connection: %w", err)
	}
	infra.opensearch = osCli
	logger.Info("OpenSearch connected")

	// Milvus
	milvusCli, err := milvusclient.NewClient(cfg.Search.Milvus)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("milvus connection: %w", err)
	}
	infra.milvus = milvusCli
	logger.Info("Milvus connected")

	// Kafka producer
	kafkaProd, err := kafkaproducer.NewProducer(cfg.Messaging.Kafka)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("kafka producer: %w", err)
	}
	infra.kafka = kafkaProd
	logger.Info("Kafka producer connected")

	// Keycloak
	kcCli, err := keycloakclient.NewClient(cfg.Auth.Keycloak)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("keycloak connection: %w", err)
	}
	infra.keycloak = kcCli
	logger.Info("Keycloak connected")

	return infra, nil
}

// repositories holds all repository implementations.
type repositories struct {
	molecule      moleculedomain.Repository
	patent        patentdomain.Repository
	portfolio     portfoliodomain.Repository
	lifecycle     lifecycledomain.Repository
	collaboration collaborationdomain.Repository
	citation      patentdomain.CitationRepository
	family        patentdomain.FamilyRepository
	knowledgeGraph patentdomain.KnowledgeGraphRepository
	document      minioclient.DocumentRepository
}

func assembleRepositories(infra *infrastructure, logger logging.Logger) *repositories {
	return &repositories{
		molecule:       pgrepo.NewMoleculeRepository(infra.pg.DB(), logger),
		patent:         pgrepo.NewPatentRepository(infra.pg.DB(), logger),
		portfolio:      pgrepo.NewPortfolioRepository(infra.pg.DB(), logger),
		lifecycle:      pgrepo.NewLifecycleRepository(infra.pg.DB(), logger),
		collaboration:  pgrepo.NewUserRepository(infra.pg.DB(), logger),
		citation:       neo4jrepo.NewCitationRepository(infra.neo4j, logger),
		family:         neo4jrepo.NewFamilyRepository(infra.neo4j, logger),
		knowledgeGraph: neo4jrepo.NewKnowledgeGraphRepository(infra.neo4j, logger),
		document:       minioclient.NewDocumentRepository(infra.minio, logger),
	}
}

// domainSvcs holds all domain service instances.
type domainSvcs struct {
	molecule      *moleculedomain.Service
	patent        *patentdomain.Service
	portfolio     *portfoliodomain.Service
	lifecycle     *lifecycledomain.Service
	collaboration *collaborationdomain.Service
}

func assembleDomainServices(repos *repositories, logger logging.Logger) *domainSvcs {
	return &domainSvcs{
		molecule:      moleculedomain.NewService(repos.molecule, logger),
		patent:        patentdomain.NewService(repos.patent, logger),
		portfolio:     portfoliodomain.NewService(repos.portfolio, logger),
		lifecycle:     lifecycledomain.NewService(repos.lifecycle, logger),
		collaboration: collaborationdomain.NewService(repos.collaboration, logger),
	}
}

func initIntelligence(cfg *config.Config, logger logging.Logger) (*common.ModelRegistry, error) {
	registry := common.NewModelRegistry(cfg.Intelligence, logger)
	if err := registry.LoadAll(); err != nil {
		return nil, fmt.Errorf("model registry load: %w", err)
	}
	logger.Info("intelligence models loaded", "count", registry.Count())
	return registry, nil
}

// applicationServices holds all application service instances.
type applicationServices struct {
	PatentMining  *appmining.Service
	Infringement  *appinfringement.Service
	Portfolio     *appportfolio.Service
	Lifecycle     *applifecycle.Service
	Collaboration *appcollaboration.Service
	Query         *appquery.Service
	Reporting     *appreporting.Service
}

func assembleApplicationServices(
	cfg *config.Config,
	ds *domainSvcs,
	repos *repositories,
	registry *common.ModelRegistry,
	infra *infrastructure,
	logger logging.Logger,
) *applicationServices {
	cache := redisclient.NewCache(infra.redis, logger)
	searcher := opensearchclient.NewSearcher(infra.opensearch, logger)
	vectorSearcher := milvusclient.NewSearcher(infra.milvus, logger)

	miningService := appmining.NewService(
		ds.molecule, ds.patent, repos.molecule, repos.patent,
		searcher, vectorSearcher, registry, cache, logger,
	)

	infringementService := appinfringement.NewService(
		ds.patent, ds.molecule, repos.patent, repos.molecule,
		repos.citation, registry, infra.kafka, cache, logger,
	)

	portfolioService := appportfolio.NewService(
		ds.portfolio, ds.patent, repos.portfolio, repos.patent,
		repos.citation, registry, cache, logger,
	)

	lifecycleService := applifecycle.NewService(
		ds.lifecycle, repos.lifecycle, repos.patent,
		infra.kafka, cache, logger,
	)

	collaborationService := appcollaboration.NewService(
		ds.collaboration, repos.collaboration, repos.document,
		infra.keycloak, cache, logger,
	)

	queryService := appquery.NewService(
		repos.knowledgeGraph, searcher, vectorSearcher,
		registry, cache, logger,
	)

	reportingService := appreporting.NewService(
		cfg.Reporting, ds.patent, ds.molecule, ds.portfolio,
		repos.patent, repos.molecule, registry, logger,
	)

	return &applicationServices{
		PatentMining:  miningService,
		Infringement:  infringementService,
		Portfolio:     portfolioService,
		Lifecycle:     lifecycleService,
		Collaboration: collaborationService,
		Query:         queryService,
		Reporting:     reportingService,
	}
}

func shutdown(httpSrv *http.Server, grpcSrv *grpc.Server, metricsSrv *http.Server, infra *infrastructure, logger logging.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	logger.Info("shutting down HTTP server")
	if err := httpSrv.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	logger.Info("shutting down gRPC server")
	grpcSrv.GracefulStop()

	logger.Info("shutting down metrics server")
	if err := metricsSrv.Shutdown(ctx); err != nil {
		logger.Error("metrics server shutdown error", "error", err)
	}

	logger.Info("closing infrastructure connections")
	infra.Close()

	logger.Info("KeyIP-Intelligence API server stopped")
}

//Personal.AI order the ending
