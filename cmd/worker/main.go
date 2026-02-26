// Phase 12 - File #288: cmd/worker/main.go
// 生成计划:
// ---
// 继续输出 288 `cmd/worker/main.go` 要实现后台工作进程入口程序。
//
// 实现要求:
//
// * **功能定位**：KeyIP-Intelligence 后台异步工作进程入口，负责消费 Kafka 消息队列中的异步任务，
//   包括专利文档解析、分子指纹计算、侵权批量分析、报告生成、知识图谱构建、向量索引更新等重计算任务
// * **核心实现**：
//   * 定义 main 函数：解析命令行参数（--config、--workers、--topics）
//   * 实现配置加载：调用 internal/config.Load()
//   * 实现日志初始化：基于配置创建 Logger
//   * 实现基础设施初始化：PostgreSQL、Neo4j、Redis、OpenSearch、Milvus、Kafka Consumer、MinIO
//   * 实现 Worker Pool：可配置并发数的 goroutine 池
//   * 实现 Topic 路由：根据 Kafka topic 将消息分发到对应的 Handler
//   * 实现 Handler 注册：
//     - patent.document.parse → 专利文档解析 Handler
//     - molecule.fingerprint.compute → 分子指纹计算 Handler
//     - infringement.batch.analyze → 侵权批量分析 Handler
//     - report.generate → 报告生成 Handler
//     - knowledge.graph.build → 知识图谱构建 Handler
//     - vector.index.update → 向量索引更新 Handler
//     - lifecycle.deadline.check → 生命周期截止日检查 Handler
//   * 实现消息确认：处理成功后 commit offset，失败后根据重试策略决定是否重试或发送到 DLQ
//   * 实现优雅关闭：监听 SIGINT/SIGTERM，等待当前处理中的消息完成后退出
//   * 实现健康检查：暴露 /healthz 端点供 K8s 探针使用
//   * 实现 Prometheus metrics：消息处理计数、延迟、错误率
// * **业务逻辑**：
//   * Worker 数量默认为 CPU 核数 * 2，可通过 --workers 覆盖
//   * 支持 --topics 过滤只消费特定 topic（用于分角色部署）
//   * 消息处理失败最多重试 3 次，间隔指数退避（1s, 2s, 4s）
//   * 超过重试次数的消息发送到 Dead Letter Queue（topic 后缀 .dlq）
//   * 每个 Handler 有独立的超时控制（默认 5 分钟）
// * **依赖关系**：
//   * 依赖：internal/config、internal/infrastructure/*、internal/application/*、internal/domain/*、internal/intelligence/*
//   * 被依赖：Dockerfile.worker、Makefile、docker-compose.yaml
// * **测试要求**：入口程序不做单元测试，通过 E2E 测试覆盖
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
// ---
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"

	pgconn "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	neo4jdriver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	redisclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	minioclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"
	opensearchclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/opensearch"
	milvusclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/milvus"
	kafkaconsumer "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	kafkaproducer "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"

	pgrepo "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	neo4jrepo "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j/repositories"

	moleculedomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	patentdomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	portfoliodomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	lifecycledomain "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

const (
	defaultWorkerConfigPath = "configs/config.yaml"
	defaultHealthPort       = 8081
	defaultHandlerTimeout   = 5 * time.Minute
	maxRetries              = 3
)

// Well-known Kafka topics for async processing.
var allTopics = []string{
	"patent.document.parse",
	"molecule.fingerprint.compute",
	"infringement.batch.analyze",
	"report.generate",
	"knowledge.graph.build",
	"vector.index.update",
	"lifecycle.deadline.check",
}

func main() {
	configPath := flag.String("config", defaultWorkerConfigPath, "path to configuration file")
	workerCount := flag.Int("workers", 0, "number of concurrent workers (default: CPU*2)")
	topicFilter := flag.String("topics", "", "comma-separated list of topics to consume (default: all)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logging.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	// Determine worker count
	numWorkers := runtime.NumCPU() * 2
	if *workerCount > 0 {
		numWorkers = *workerCount
	}

	// Determine topics
	topics := allTopics
	if *topicFilter != "" {
		topics = strings.Split(*topicFilter, ",")
		for i := range topics {
			topics[i] = strings.TrimSpace(topics[i])
		}
	}

	logger.Info("starting KeyIP-Intelligence worker",
		"workers", numWorkers,
		"topics", strings.Join(topics, ","),
	)

	// Initialize Prometheus metrics
	metricsCollector := prometheus.NewCollector(cfg.Metrics)
	metricsCollector.Register()

	// Initialize infrastructure
	infra, err := initWorkerInfrastructure(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize infrastructure", "error", err)
		os.Exit(1)
	}
	defer infra.Close()

	// Initialize intelligence layer
	modelRegistry, err := initWorkerIntelligence(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize intelligence layer", "error", err)
		os.Exit(1)
	}

	// Build handler registry
	handlerRegistry := buildHandlerRegistry(cfg, infra, modelRegistry, logger)

	// Create Kafka consumer
	consumer, err := kafkaconsumer.NewConsumer(cfg.Messaging.Kafka, topics, logger)
	if err != nil {
		logger.Error("failed to create Kafka consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// Create DLQ producer for failed messages
	dlqProducer, err := kafkaproducer.NewProducer(cfg.Messaging.Kafka)
	if err != nil {
		logger.Error("failed to create DLQ producer", "error", err)
		os.Exit(1)
	}
	defer dlqProducer.Close()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start health check server
	healthSrv := startHealthServer(cfg, logger)

	// Start worker pool
	var wg sync.WaitGroup
	msgChan := make(chan *kafkaconsumer.Message, numWorkers*2)

	// Spawn workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerLoop(ctx, workerID, msgChan, handlerRegistry, dlqProducer, consumer, metricsCollector, logger)
		}(i)
	}

	// Spawn consumer loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumerLoop(ctx, consumer, msgChan, logger)
	}()

	logger.Info("worker pool started", "workers", numWorkers)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("received shutdown signal", "signal", sig.String())

	// Initiate graceful shutdown
	cancel()

	// Close message channel after consumer stops
	// Workers will drain remaining messages
	logger.Info("waiting for workers to finish current tasks")

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all workers finished")
	case <-time.After(defaultHandlerTimeout + 30*time.Second):
		logger.Warn("shutdown timeout exceeded, forcing exit")
	}

	// Shutdown health server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("health server shutdown error", "error", err)
	}

	logger.Info("KeyIP-Intelligence worker stopped")
}

// MessageHandler processes a single Kafka message.
type MessageHandler interface {
	Handle(ctx context.Context, msg *kafkaconsumer.Message) error
	Topic() string
}

// workerInfrastructure holds infrastructure clients for the worker process.
type workerInfrastructure struct {
	pg         *pgconn.Connection
	neo4j      *neo4jdriver.Driver
	redis      *redisclient.Client
	minio      *minioclient.Client
	opensearch *opensearchclient.Client
	milvus     *milvusclient.Client
}

func (w *workerInfrastructure) Close() {
	if w.milvus != nil {
		w.milvus.Close()
	}
	if w.opensearch != nil {
		w.opensearch.Close()
	}
	if w.redis != nil {
		w.redis.Close()
	}
	if w.neo4j != nil {
		w.neo4j.Close()
	}
	if w.pg != nil {
		w.pg.Close()
	}
	if w.minio != nil {
		w.minio.Close()
	}
}

func initWorkerInfrastructure(cfg *config.Config, logger logging.Logger) (*workerInfrastructure, error) {
	infra := &workerInfrastructure{}

	pg, err := pgconn.NewConnection(cfg.Database.Postgres)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	infra.pg = pg

	neo4jDrv, err := neo4jdriver.NewDriver(cfg.Database.Neo4j)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("neo4j: %w", err)
	}
	infra.neo4j = neo4jDrv

	redisCli, err := redisclient.NewClient(cfg.Database.Redis)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}
	infra.redis = redisCli

	minioCli, err := minioclient.NewClient(cfg.Storage.MinIO)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("minio: %w", err)
	}
	infra.minio = minioCli

	osCli, err := opensearchclient.NewClient(cfg.Search.OpenSearch)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("opensearch: %w", err)
	}
	infra.opensearch = osCli

	milvusCli, err := milvusclient.NewClient(cfg.Search.Milvus)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("milvus: %w", err)
	}
	infra.milvus = milvusCli

	logger.Info("worker infrastructure initialized")
	return infra, nil
}

func initWorkerIntelligence(cfg *config.Config, logger logging.Logger) (*common.ModelRegistry, error) {
	registry := common.NewModelRegistry(cfg.Intelligence, logger)
	if err := registry.LoadAll(); err != nil {
		return nil, fmt.Errorf("model registry: %w", err)
	}
	logger.Info("worker intelligence models loaded", "count", registry.Count())
	return registry, nil
}

func buildHandlerRegistry(
	cfg *config.Config,
	infra *workerInfrastructure,
	registry *common.ModelRegistry,
	logger logging.Logger,
) map[string]MessageHandler {
	// Build repositories
	moleculeRepo := pgrepo.NewMoleculeRepository(infra.pg.DB(), logger)
	patentRepo := pgrepo.NewPatentRepository(infra.pg.DB(), logger)
	portfolioRepo := pgrepo.NewPortfolioRepository(infra.pg.DB(), logger)
	lifecycleRepo := pgrepo.NewLifecycleRepository(infra.pg.DB(), logger)
	kgRepo := neo4jrepo.NewKnowledgeGraphRepository(infra.neo4j, logger)
	citationRepo := neo4jrepo.NewCitationRepository(infra.neo4j, logger)

	// Build domain services
	moleculeSvc := moleculedomain.NewService(moleculeRepo, logger)
	patentSvc := patentdomain.NewService(patentRepo, logger)
	portfolioSvc := portfoliodomain.NewService(portfolioRepo, logger)
	lifecycleSvc := lifecycledomain.NewService(lifecycleRepo, logger)

	// Build search clients
	vectorSearcher := milvusclient.NewSearcher(infra.milvus, logger)
	textSearcher := opensearchclient.NewSearcher(infra.opensearch, logger)
	cache := redisclient.NewCache(infra.redis, logger)
	docRepo := minioclient.NewDocumentRepository(infra.minio, logger)

	handlers := map[string]MessageHandler{
		"patent.document.parse": NewPatentDocumentParseHandler(
			patentSvc, patentRepo, docRepo, textSearcher, logger,
		),
		"molecule.fingerprint.compute": NewMoleculeFingerprintHandler(
			moleculeSvc, moleculeRepo, vectorSearcher, registry, logger,
		),
		"infringement.batch.analyze": NewInfringementBatchHandler(
			patentSvc, moleculeSvc, patentRepo, moleculeRepo,
			citationRepo, registry, cache, logger,
		),
		"report.generate": NewReportGenerateHandler(
			cfg.Reporting, patentSvc, moleculeSvc, portfolioSvc,
			patentRepo, moleculeRepo, registry, docRepo, logger,
		),
		"knowledge.graph.build": NewKnowledgeGraphBuildHandler(
			kgRepo, patentRepo, moleculeRepo, citationRepo, registry, logger,
		),
		"vector.index.update": NewVectorIndexUpdateHandler(
			moleculeRepo, patentRepo, vectorSearcher, registry, logger,
		),
		"lifecycle.deadline.check": NewLifecycleDeadlineHandler(
			lifecycleSvc, lifecycleRepo, patentRepo, cache, logger,
		),
	}

	return handlers
}

func consumerLoop(ctx context.Context, consumer *kafkaconsumer.Consumer, msgChan chan<- *kafkaconsumer.Message, logger logging.Logger) {
	defer close(msgChan)

	for {
		select {
		case <-ctx.Done():
			logger.Info("consumer loop stopping")
			return
		default:
			msg, err := consumer.Poll(ctx, 1*time.Second)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.Warn("consumer poll error", "error", err)
				continue
			}
			if msg == nil {
				continue
			}

			select {
			case msgChan <- msg:
			case <-ctx.Done():
				return
			}
		}
	}
}

func workerLoop(
	ctx context.Context,
	workerID int,
	msgChan <-chan *kafkaconsumer.Message,
	handlers map[string]MessageHandler,
	dlqProducer *kafkaproducer.Producer,
	consumer *kafkaconsumer.Consumer,
	metrics *prometheus.Collector,
	logger logging.Logger,
) {
	logger.Info("worker started", "worker_id", workerID)

	for msg := range msgChan {
		handler, ok := handlers[msg.Topic]
		if !ok {
			logger.Warn("no handler for topic", "topic", msg.Topic, "worker_id", workerID)
			consumer.Commit(msg)
			continue
		}

		start := time.Now()
		err := processWithRetry(ctx, handler, msg, logger, workerID)
		duration := time.Since(start)

		if err != nil {
			logger.Error("message processing failed after retries",
				"topic", msg.Topic,
				"worker_id", workerID,
				"error", err,
				"duration", duration,
			)
			metrics.RecordMessageError(msg.Topic)

			// Send to Dead Letter Queue
			dlqTopic := msg.Topic + ".dlq"
			if dlqErr := dlqProducer.Send(ctx, dlqTopic, msg.Key, msg.Value); dlqErr != nil {
				logger.Error("failed to send to DLQ",
					"topic", dlqTopic,
					"error", dlqErr,
				)
			}
		} else {
			logger.Info("message processed",
				"topic", msg.Topic,
				"worker_id", workerID,
				"duration", duration,
			)
			metrics.RecordMessageProcessed(msg.Topic, duration)
		}

		// Commit offset regardless (failed messages go to DLQ)
		consumer.Commit(msg)
	}

	logger.Info("worker stopped", "worker_id", workerID)
}

func processWithRetry(
	ctx context.Context,
	handler MessageHandler,
	msg *kafkaconsumer.Message,
	logger logging.Logger,
	workerID int,
) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			logger.Warn("retrying message",
				"topic", msg.Topic,
				"attempt", attempt,
				"backoff", backoff,
				"worker_id", workerID,
			)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Create handler context with timeout
		handlerCtx, cancel := context.WithTimeout(ctx, defaultHandlerTimeout)
		lastErr = handler.Handle(handlerCtx, msg)
		cancel()

		if lastErr == nil {
			return nil
		}

		logger.Warn("message processing attempt failed",
			"topic", msg.Topic,
			"attempt", attempt,
			"error", lastErr,
			"worker_id", workerID,
		)
	}

	return fmt.Errorf("exhausted %d retries: %w", maxRetries, lastErr)
}

func startHealthServer(cfg *config.Config, logger logging.Logger) *http.Server {
	port := defaultHealthPort
	if cfg.Worker.HealthPort > 0 {
		port = cfg.Worker.HealthPort
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		logger.Info("health server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", "error", err)
		}
	}()

	return srv
}

// --- Handler stubs (each would be in separate files in production) ---

// PatentDocumentParseHandler handles patent document parsing tasks.
type PatentDocumentParseHandler struct {
	patentSvc  *patentdomain.Service
	patentRepo patentdomain.Repository
	docRepo    minioclient.DocumentRepository
	searcher   *opensearchclient.Searcher
	logger     logging.Logger
}

func NewPatentDocumentParseHandler(
	patentSvc *patentdomain.Service,
	patentRepo patentdomain.Repository,
	docRepo minioclient.DocumentRepository,
	searcher *opensearchclient.Searcher,
	logger logging.Logger,
) *PatentDocumentParseHandler {
	return &PatentDocumentParseHandler{
		patentSvc:  patentSvc,
		patentRepo: patentRepo,
		docRepo:    docRepo,
		searcher:   searcher,
		logger:     logger,
	}
}

func (h *PatentDocumentParseHandler) Topic() string { return "patent.document.parse" }

func (h *PatentDocumentParseHandler) Handle(ctx context.Context, msg *kafkaconsumer.Message) error {
	h.logger.Info("parsing patent document", "key", string(msg.Key))
	// Implementation: download from MinIO → parse PDF/XML → extract claims → index in OpenSearch
	return nil
}

// MoleculeFingerprintHandler handles molecular fingerprint computation.
type MoleculeFingerprintHandler struct {
	moleculeSvc  *moleculedomain.Service
	moleculeRepo moleculedomain.Repository
	vectorSearch *milvusclient.Searcher
	registry     *common.ModelRegistry
	logger       logging.Logger
}

func NewMoleculeFingerprintHandler(
	moleculeSvc *moleculedomain.Service,
	moleculeRepo moleculedomain.Repository,
	vectorSearch *milvusclient.Searcher,
	registry *common.ModelRegistry,
	logger logging.Logger,
) *MoleculeFingerprintHandler {
	return &MoleculeFingerprintHandler{
		moleculeSvc:  moleculeSvc,
		moleculeRepo: moleculeRepo,
		vectorSearch: vectorSearch,
		registry:     registry,
		logger:       logger,
	}
}

func (h *MoleculeFingerprintHandler) Topic() string { return "molecule.fingerprint.compute" }

func (h *MoleculeFingerprintHandler) Handle(ctx context.Context, msg *kafkaconsumer.Message) error {
	h.logger.Info("computing molecule fingerprint", "key", string(msg.Key))
	// Implementation: load SMILES → compute fingerprint via AI model → store in Milvus
	return nil
}

// InfringementBatchHandler handles batch infringement analysis.
type InfringementBatchHandler struct {
	patentSvc    *patentdomain.Service
	moleculeSvc  *moleculedomain.Service
	patentRepo   patentdomain.Repository
	moleculeRepo moleculedomain.Repository
	citationRepo patentdomain.CitationRepository
	registry     *common.ModelRegistry
	cache        *redisclient.Cache
	logger       logging.Logger
}

func NewInfringementBatchHandler(
	patentSvc *patentdomain.Service,
	moleculeSvc *moleculedomain.Service,
	patentRepo patentdomain.Repository,
	moleculeRepo moleculedomain.Repository,
	citationRepo patentdomain.CitationRepository,
	registry *common.ModelRegistry,
	cache *redisclient.Cache,
	logger logging.Logger,
) *InfringementBatchHandler {
	return &InfringementBatchHandler{
		patentSvc:    patentSvc,
		moleculeSvc:  moleculeSvc,
		patentRepo:   patentRepo,
		moleculeRepo: moleculeRepo,
		citationRepo: citationRepo,
		registry:     registry,
		cache:        cache,
		logger:       logger,
	}
}

func (h *InfringementBatchHandler) Topic() string { return "infringement.batch.analyze" }

func (h *InfringementBatchHandler) Handle(ctx context.Context, msg *kafkaconsumer.Message) error {
	h.logger.Info("running batch infringement analysis", "key", string(msg.Key))
	// Implementation: load patent claims + molecule structures → run similarity → generate report
	return nil
}

// ReportGenerateHandler handles report generation tasks.
type ReportGenerateHandler struct {
	reportingCfg interface{}
	patentSvc    *patentdomain.Service
	moleculeSvc  *moleculedomain.Service
	portfolioSvc *portfoliodomain.Service
	patentRepo   patentdomain.Repository
	moleculeRepo moleculedomain.Repository
	registry     *common.ModelRegistry
	docRepo      minioclient.DocumentRepository
	logger       logging.Logger
}

func NewReportGenerateHandler(
	reportingCfg interface{},
	patentSvc *patentdomain.Service,
	moleculeSvc *moleculedomain.Service,
	portfolioSvc *portfoliodomain.Service,
	patentRepo patentdomain.Repository,
	moleculeRepo moleculedomain.Repository,
	registry *common.ModelRegistry,
	docRepo minioclient.DocumentRepository,
	logger logging.Logger,
) *ReportGenerateHandler {
	return &ReportGenerateHandler{
		reportingCfg: reportingCfg,
		patentSvc:    patentSvc,
		moleculeSvc:  moleculeSvc,
		portfolioSvc: portfolioSvc,
		patentRepo:   patentRepo,
		moleculeRepo: moleculeRepo,
		registry:     registry,
		docRepo:      docRepo,
		logger:       logger,
	}
}

func (h *ReportGenerateHandler) Topic() string { return "report.generate" }

func (h *ReportGenerateHandler) Handle(ctx context.Context, msg *kafkaconsumer.Message) error {
	h.logger.Info("generating report", "key", string(msg.Key))
	// Implementation: gather data → render template → generate PDF → upload to MinIO
	return nil
}

// KnowledgeGraphBuildHandler handles knowledge graph construction.
type KnowledgeGraphBuildHandler struct {
	kgRepo       patentdomain.KnowledgeGraphRepository
	patentRepo   patentdomain.Repository
	moleculeRepo moleculedomain.Repository
	citationRepo patentdomain.CitationRepository
	registry     *common.ModelRegistry
	logger       logging.Logger
}

func NewKnowledgeGraphBuildHandler(
	kgRepo patentdomain.KnowledgeGraphRepository,
	patentRepo patentdomain.Repository,
	moleculeRepo moleculedomain.Repository,
	citationRepo patentdomain.CitationRepository,
	registry *common.ModelRegistry,
	logger logging.Logger,
) *KnowledgeGraphBuildHandler {
	return &KnowledgeGraphBuildHandler{
		kgRepo:       kgRepo,
		patentRepo:   patentRepo,
		moleculeRepo: moleculeRepo,
		citationRepo: citationRepo,
		registry:     registry,
		logger:       logger,
	}
}

func (h *KnowledgeGraphBuildHandler) Topic() string { return "knowledge.graph.build" }

func (h *KnowledgeGraphBuildHandler) Handle(ctx context.Context, msg *kafkaconsumer.Message) error {
	h.logger.Info("building knowledge graph", "key", string(msg.Key))
	// Implementation: extract entities → build relationships → write to Neo4j
	return nil
}

// VectorIndexUpdateHandler handles vector index updates.
type VectorIndexUpdateHandler struct {
	moleculeRepo moleculedomain.Repository
	patentRepo   patentdomain.Repository
	vectorSearch *milvusclient.Searcher
	registry     *common.ModelRegistry
	logger       logging.Logger
}

func NewVectorIndexUpdateHandler(
	moleculeRepo moleculedomain.Repository,
	patentRepo patentdomain.Repository,
	vectorSearch *milvusclient.Searcher,
	registry *common.ModelRegistry,
	logger logging.Logger,
) *VectorIndexUpdateHandler {
	return &VectorIndexUpdateHandler{
		moleculeRepo: moleculeRepo,
		patentRepo:   patentRepo,
		vectorSearch: vectorSearch,
		registry:     registry,
		logger:       logger,
	}
}

func (h *VectorIndexUpdateHandler) Topic() string { return "vector.index.update" }

func (h *VectorIndexUpdateHandler) Handle(ctx context.Context, msg *kafkaconsumer.Message) error {
	h.logger.Info("updating vector index", "key", string(msg.Key))
	// Implementation: load new/updated molecules → compute embeddings → upsert into Milvus
	return nil
}

// LifecycleDeadlineHandler handles lifecycle deadline checking.
type LifecycleDeadlineHandler struct {
	lifecycleSvc  *lifecycledomain.Service
	lifecycleRepo lifecycledomain.Repository
	patentRepo    patentdomain.Repository
	cache         *redisclient.Cache
	logger        logging.Logger
}

func NewLifecycleDeadlineHandler(
	lifecycleSvc *lifecycledomain.Service,
	lifecycleRepo lifecycledomain.Repository,
	patentRepo patentdomain.Repository,
	cache *redisclient.Cache,
	logger logging.Logger,
) *LifecycleDeadlineHandler {
	return &LifecycleDeadlineHandler{
		lifecycleSvc:  lifecycleSvc,
		lifecycleRepo: lifecycleRepo,
		patentRepo:    patentRepo,
		cache:         cache,
		logger:        logger,
	}
}

func (h *LifecycleDeadlineHandler) Topic() string { return "lifecycle.deadline.check" }

func (h *LifecycleDeadlineHandler) Handle(ctx context.Context, msg *kafkaconsumer.Message) error {
	h.logger.Info("checking lifecycle deadlines", "key", string(msg.Key))
	// Implementation:
	// 1. Query patents approaching deadline (renewal, response, abandonment)
	// 2. For each patent nearing deadline:
	//    a. Calculate days remaining
	//    b. Determine urgency level (critical < 7d, warning < 30d, notice < 90d)
	//    c. Check if notification already sent (via cache)
	//    d. If not sent, create notification event
	// 3. Update lifecycle status for expired patents
	// 4. Invalidate relevant cache entries
	return nil
}

//Personal.AI order the ending

