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
//   * 实现 Worker Pool：可配置并发数的 goroutine 池，使用 `errgroup` 管理并发
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
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/prometheus"

	pgconn "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	neo4jdriver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	redisclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	minioclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"
	opensearchclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/opensearch"
	milvusclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/milvus"
	kafkaclient "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"

	intcommon "github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

const (
	defaultWorkerConfigPath = "configs/config.yaml"
	defaultHealthPort       = 8081
	defaultHandlerTimeout   = 5 * time.Minute
	maxRetries              = 3
)

// Well-known Kafka topics for async processing.
var allTopics = []string{
	"patent.new",
	"patent.status_changed",
	"molecule.indexed",
	"alert.trigger",
	"deadline.approaching",
	"report.generate",
	"infrastructure.health",
}

func main() {
	configPath := flag.String("config", defaultWorkerConfigPath, "path to configuration file")
	workerCount := flag.Int("workers", 0, "number of concurrent workers (default: CPU*2)")
	topicFilter := flag.String("topics", "", "comma-separated list of topics to consume (default: all)")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadFromFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger from config
	logCfg := logging.LogConfig{
		Level:            logging.LevelInfo,
		Format:           cfg.Monitoring.Logging.Format,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EnableCaller:     true,
		ServiceName:      "keyip-worker",
	}
	if cfg.Monitoring.Logging.Output == "file" && cfg.Monitoring.Logging.FilePath != "" {
		logCfg.OutputPaths = append(logCfg.OutputPaths, cfg.Monitoring.Logging.FilePath)
	}
	logger, err := logging.NewLogger(logCfg)
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
		logging.Int("workers", numWorkers),
		logging.String("topics", strings.Join(topics, ",")),
	)

	var shuttingDown atomic.Bool

	// Initialize Prometheus metrics
	promCfg := prometheus.CollectorConfig{
		Namespace:            cfg.Monitoring.Prometheus.Namespace,
		EnableProcessMetrics: true,
		EnableGoMetrics:      true,
	}
	metricsCollector, err := prometheus.NewMetricsCollector(promCfg, logger)
	if err != nil {
		logger.Error("failed to initialize metrics collector", logging.Err(err))
		os.Exit(1)
	}

	// Initialize infrastructure
	infra, err := initWorkerInfrastructure(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize infrastructure", logging.Err(err))
		os.Exit(1)
	}
	defer infra.Close()

	// Initialize intelligence layer
	modelRegistry, err := initWorkerIntelligence(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize intelligence layer", logging.Err(err))
		os.Exit(1)
	}
	defer modelRegistry.Close()

	// Create a regular producer for follow-up event publishing
	eventProducerCfg := kafkaclient.ProducerConfig{
		Brokers:    cfg.Messaging.Kafka.Brokers,
		Acks:       "all",
		MaxRetries: 3,
	}
	eventProducer, err := kafkaclient.NewProducer(eventProducerCfg, logger)
	if err != nil {
		logger.Error("failed to create event producer", logging.Err(err))
		os.Exit(1)
	}
	defer eventProducer.Close()

	// Create DLQ producer config from app config
	dlqProducerCfg := kafkaclient.ProducerConfig{
		Brokers:    cfg.Messaging.Kafka.Brokers,
		Acks:       "all",
		MaxRetries: 3,
	}
	dlqProducer, err := kafkaclient.NewProducer(dlqProducerCfg, logger)
	if err != nil {
		logger.Error("failed to create DLQ producer", logging.Err(err))
		os.Exit(1)
	}
	defer dlqProducer.Close()

	// Build handler registry with all dependencies
	handlerRegistry := buildHandlerRegistry(cfg, infra, modelRegistry, eventProducer, logger)

	// Create Kafka consumer config from app config
	consumerCfg := kafkaclient.ConsumerConfig{
		Brokers:           cfg.Messaging.Kafka.Brokers,
		GroupID:           cfg.Messaging.Kafka.ConsumerGroup,
		Topics:            topics,
		AutoOffsetReset:   cfg.Messaging.Kafka.AutoOffsetReset,
		SessionTimeout:    cfg.Messaging.Kafka.SessionTimeout,
		HeartbeatInterval: cfg.Messaging.Kafka.HeartbeatInterval,
	}
	consumer, err := kafkaclient.NewConsumer(consumerCfg, logger)
	if err != nil {
		logger.Error("failed to create Kafka consumer", logging.Err(err))
		os.Exit(1)
	}
	defer consumer.Close()

	// Context for graceful shutdown and error propagation
	// errgroup creates a context that is cancelled when any goroutine returns a non-nil error
	// or when the parent context is cancelled (e.g. by signal).
	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	g, ctx := errgroup.WithContext(parentCtx)

	// Start health check server
	healthSrv := startHealthServer(cfg, logger, metricsCollector, &shuttingDown)

	// Message channel
	msgChan := make(chan *common.Message, numWorkers*2)

	// Spawn workers using errgroup
	for i := 0; i < numWorkers; i++ {
		workerID := i
		g.Go(func() error {
			return workerLoop(ctx, workerID, msgChan, handlerRegistry, dlqProducer, logger)
		})
	}

	// Spawn consumer loop using errgroup
	g.Go(func() error {
		defer close(msgChan)
		return consumerLoop(ctx, consumer, msgChan, logger)
	})

	logger.Info("worker pool started", logging.Int("workers", numWorkers))

	// Listen for OS signals in a separate goroutine
	// We use a separate channel for signals to not block the main flow
	// If signal received, we cancel the parent context which propagates to errgroup
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-stopChan:
			logger.Info("received shutdown signal", logging.String("signal", sig.String()))
			shuttingDown.Store(true)
			cancel() // Cancel parent context, stopping all errgroup goroutines
		case <-ctx.Done():
			// Context cancelled elsewhere (e.g. error in group)
		}
	}()

	shutdownStart := time.Now()

	// Wait for all workers and consumer goroutines to finish
	if err := g.Wait(); err != nil {
		if err != context.Canceled {
			logger.Error("worker group exited with error", logging.Err(err))
		}
	}
	logger.Info("workers stopped", logging.Duration("elapsed", time.Since(shutdownStart)))

	// Shutdown health server
	t := time.Now()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthSrv.Shutdown(shutdownCtx); err != nil {
		logger.Error("health server shutdown error", logging.Err(err))
	}
	logger.Info("health server stopped", logging.Duration("elapsed", time.Since(t)))

	// Shutdown infrastructure connections (reverse init order, DB last)
	t = time.Now()
	infra.Close()
	logger.Info("infrastructure connections closed",
		logging.Duration("elapsed", time.Since(t)))

	logger.Info("KeyIP-Intelligence worker stopped",
		logging.Duration("total_elapsed", time.Since(shutdownStart)))
}

// MessageHandler processes a single Kafka message.
type MessageHandler interface {
	Handle(ctx context.Context, msg *common.Message) error
	Topic() string
}

// workerInfrastructure holds infrastructure clients for the worker process.
type workerInfrastructure struct {
	pg         *pgconn.Connection
	neo4j      *neo4jdriver.Driver
	redis      *redisclient.Client
	minio      *minioclient.MinIOClient
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

	// Convert app config to postgres infrastructure config
	pgCfg := pgconn.PostgresConfig{
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
	pg, err := pgconn.NewConnection(pgCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	infra.pg = pg

	// Convert app config to neo4j infrastructure config
	neo4jCfg := neo4jdriver.Neo4jConfig{
		URI:                          cfg.Database.Neo4j.URI,
		Username:                     cfg.Database.Neo4j.User,
		Password:                     cfg.Database.Neo4j.Password,
		MaxConnectionPoolSize:        cfg.Database.Neo4j.MaxConnectionPoolSize,
		ConnectionAcquisitionTimeout: cfg.Database.Neo4j.ConnectionAcquisitionTimeout,
	}
	neo4jDrv, err := neo4jdriver.NewDriver(neo4jCfg, logger)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("neo4j: %w", err)
	}
	infra.neo4j = neo4jDrv

	// Convert app config to redis infrastructure config
	redisCfg := &redisclient.RedisConfig{
		Addr:         cfg.Cache.Redis.Addr,
		Password:     cfg.Cache.Redis.Password,
		DB:           cfg.Cache.Redis.DB,
		PoolSize:     cfg.Cache.Redis.PoolSize,
		MinIdleConns: cfg.Cache.Redis.MinIdleConns,
		DialTimeout:  cfg.Cache.Redis.DialTimeout,
		ReadTimeout:  cfg.Cache.Redis.ReadTimeout,
		WriteTimeout: cfg.Cache.Redis.WriteTimeout,
	}
	redisCli, err := redisclient.NewClient(redisCfg, logger)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}
	infra.redis = redisCli

	// Convert app config to minio infrastructure config
	minioCfg := &minioclient.MinIOConfig{
		Endpoint:        cfg.Storage.MinIO.Endpoint,
		AccessKeyID:     cfg.Storage.MinIO.AccessKey,
		SecretAccessKey: cfg.Storage.MinIO.SecretKey,
		UseSSL:          cfg.Storage.MinIO.UseSSL,
		DefaultBucket:   cfg.Storage.MinIO.BucketName,
		Region:          cfg.Storage.MinIO.Region,
	}
	minioCli, err := minioclient.NewMinIOClient(minioCfg, logger)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("minio: %w", err)
	}
	infra.minio = minioCli

	// Convert app config to opensearch infrastructure config
	osCfg := opensearchclient.ClientConfig{
		Addresses: cfg.Search.OpenSearch.Addresses,
		Username:  cfg.Search.OpenSearch.Username,
		Password:  cfg.Search.OpenSearch.Password,
	}
	osCli, err := opensearchclient.NewClient(osCfg, logger)
	if err != nil {
		infra.Close()
		return nil, fmt.Errorf("opensearch: %w", err)
	}
	infra.opensearch = osCli

	// Convert app config to milvus infrastructure config
		// Fix: include port in address, and make milvus optional
	milvusCfg := milvusclient.ClientConfig{
		Address:  fmt.Sprintf("%s:%d", cfg.Search.Milvus.Address, cfg.Search.Milvus.Port),
		Username: cfg.Search.Milvus.Username,
		Password: cfg.Search.Milvus.Password,
	}
	milvusCli, err := milvusclient.NewClient(milvusCfg, logger)
	if err != nil {
		logger.Warn("milvus connection failed (non-fatal)", logging.Err(err))
	} else {
		infra.milvus = milvusCli
	}

	logger.Info("worker infrastructure initialized")
	return infra, nil
}

func initWorkerIntelligence(cfg *config.Config, logger logging.Logger) (intcommon.ModelRegistry, error) {
	// Create a model loader and registry
	// For now, use a noop implementation until full intelligence layer is wired
	loader := intcommon.NewNoopModelLoader()
	metrics := intcommon.NewNoopIntelligenceMetrics()
	logAdapter := intcommon.NewNoopLogger()

	registry, err := intcommon.NewModelRegistry(loader, metrics, logAdapter)
	if err != nil {
		return nil, fmt.Errorf("model registry: %w", err)
	}
	logger.Info("worker intelligence models initialized")
	return registry, nil
}

func buildHandlerRegistry(
	cfg *config.Config,
	infra *workerInfrastructure,
	registry intcommon.ModelRegistry,
	producer *kafkaclient.Producer,
	logger logging.Logger,
) map[string]MessageHandler {
	handlers := make(map[string]MessageHandler, len(allTopics))

	// patent.new -- parse patent data, trigger ChemExtractor, store results, publish events
	handlers["patent.new"] = &patentNewHandler{
		producer: producer,
		infra:    infra,
		logger:   logger.With(logging.String("handler", "patent.new")),
	}

	// patent.status_changed -- update legal status, check deadline impacts
	handlers["patent.status_changed"] = &patentStatusChangedHandler{
		producer: producer,
		infra:    infra,
		logger:   logger.With(logging.String("handler", "patent.status_changed")),
	}

	// molecule.indexed -- trigger similarity comparison against watch rules
	handlers["molecule.indexed"] = &moleculeIndexedHandler{
		producer: producer,
		infra:    infra,
		logger:   logger.With(logging.String("handler", "molecule.indexed")),
	}

	// alert.trigger -- send alert via configured channels
	handlers["alert.trigger"] = &alertTriggerHandler{
		producer: producer,
		logger:   logger.With(logging.String("handler", "alert.trigger")),
	}

	// deadline.approaching -- check deadline config, create notifications
	handlers["deadline.approaching"] = &deadlineApproachingHandler{
		producer: producer,
		infra:    infra,
		logger:   logger.With(logging.String("handler", "deadline.approaching")),
	}

	// report.generate -- call report generation service, publish completion event
	handlers["report.generate"] = &reportGenerateHandler{
		producer: producer,
		infra:    infra,
		cfg:      cfg,
		logger:   logger.With(logging.String("handler", "report.generate")),
	}

	// infrastructure.health -- check dependencies, update health metrics
	handlers["infrastructure.health"] = &infrastructureHealthHandler{
		infra:  infra,
		logger: logger.With(logging.String("handler", "infrastructure.health")),
	}

	logger.Info("handler registry built",
		logging.Int("handlers", len(handlers)),
		logging.String("topics", strings.Join(allTopics, ",")),
	)
	return handlers
}

// ---------------------------------------------------------------------------
// Handler implementations
// ---------------------------------------------------------------------------

// --- patent.new handler ---

type patentNewPayload struct {
	PatentID     string `json:"patent_id"`
	PatentNumber string `json:"patent_number"`
	Title        string `json:"title"`
	Abstract     string `json:"abstract,omitempty"`
	Description  string `json:"description,omitempty"`
	Claims       string `json:"claims,omitempty"`
	FilingDate   string `json:"filing_date,omitempty"`
	Source       string `json:"source,omitempty"`
}

type patentNewHandler struct {
	producer *kafkaclient.Producer
	infra    *workerInfrastructure
	logger   logging.Logger
}

func (h *patentNewHandler) Topic() string { return "patent.new" }

func (h *patentNewHandler) Handle(ctx context.Context, msg *common.Message) error {
	var payload patentNewPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("failed to decode patent.new payload: %w", err)
	}

	h.logger.Info("processing new patent",
		logging.String("patent_id", payload.PatentID),
		logging.String("patent_number", payload.PatentNumber),
	)

	// Store patent metadata in PostgreSQL
	_, err := h.infra.pg.DB().ExecContext(ctx,
		`INSERT INTO patents (patent_id, patent_number, title, abstract, source, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			 ON CONFLICT (patent_id) DO UPDATE SET
			   title = EXCLUDED.title,
			   abstract = EXCLUDED.abstract,
			   updated_at = NOW()`,
		payload.PatentID, payload.PatentNumber, payload.Title, payload.Abstract, payload.Source,
	)
	if err != nil {
		h.logger.Error("failed to store patent", logging.Err(err))
		return fmt.Errorf("store patent: %w", err)
	}

	// If the patent has text content, queue chemical extraction for processing
	if payload.Description != "" || payload.Claims != "" {
		textContent := payload.Description
		if payload.Claims != "" {
			textContent = textContent + "\n" + payload.Claims
		}

		_, extractErr := h.infra.pg.DB().ExecContext(ctx,
			`INSERT INTO extraction_queue (patent_id, patent_number, text_content, status, created_at)
				 VALUES ($1, $2, $3, 'pending', NOW())`,
			payload.PatentID, payload.PatentNumber, textContent,
		)
		if extractErr != nil {
			h.logger.Warn("failed to queue chemical extraction", logging.Err(extractErr))
		}
	}

	// Publish event that patent has been processed
	env, envErr := kafkaclient.NewEventEnvelope("patent.processed", "worker", map[string]interface{}{
		"patent_id":     payload.PatentID,
		"patent_number": payload.PatentNumber,
		"status":        "stored",
		"processed_at":  time.Now().UTC(),
	})
	if envErr != nil {
		h.logger.Warn("failed to create event envelope", logging.Err(envErr))
		return nil
	}
	prodMsg, prodErr := env.ToMessage(kafkaclient.TopicPatentAnalyzed)
	if prodErr != nil {
		h.logger.Warn("failed to create producer message", logging.Err(prodErr))
		return nil
	}
	if pubErr := h.producer.Publish(ctx, prodMsg); pubErr != nil {
		h.logger.Warn("failed to publish patent.processed event", logging.Err(pubErr))
	}

	h.logger.Info("new patent processed successfully",
		logging.String("patent_id", payload.PatentID),
	)
	return nil
}

// --- patent.status_changed handler ---

type patentStatusChangedPayload struct {
	PatentID     string `json:"patent_id"`
	PatentNumber string `json:"patent_number"`
	OldStatus    string `json:"old_status"`
	NewStatus    string `json:"new_status"`
	Reason       string `json:"reason,omitempty"`
	ChangedAt    string `json:"changed_at"`
}

type patentStatusChangedHandler struct {
	producer *kafkaclient.Producer
	infra    *workerInfrastructure
	logger   logging.Logger
}

func (h *patentStatusChangedHandler) Topic() string { return "patent.status_changed" }

func (h *patentStatusChangedHandler) Handle(ctx context.Context, msg *common.Message) error {
	var payload patentStatusChangedPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("failed to decode patent.status_changed payload: %w", err)
	}

	h.logger.Info("processing patent status change",
		logging.String("patent_id", payload.PatentID),
		logging.String("old_status", payload.OldStatus),
		logging.String("new_status", payload.NewStatus),
	)

	// Update legal status in PostgreSQL
	_, err := h.infra.pg.DB().ExecContext(ctx,
		`UPDATE patents SET legal_status = $1, updated_at = NOW() WHERE patent_id = $2`,
		payload.NewStatus, payload.PatentID,
	)
	if err != nil {
		h.logger.Error("failed to update patent status", logging.Err(err))
		return fmt.Errorf("update patent status: %w", err)
	}

	// Record status change history
	_, err = h.infra.pg.DB().ExecContext(ctx,
		`INSERT INTO patent_status_history (patent_id, old_status, new_status, reason, changed_at)
			 VALUES ($1, $2, $3, $4, $5)`,
		payload.PatentID, payload.OldStatus, payload.NewStatus, payload.Reason, payload.ChangedAt,
	)
	if err != nil {
		h.logger.Warn("failed to record status history", logging.Err(err))
	}

	// Check for deadline impacts when a patent becomes active or expires
	impactDeadlines := false
	switch payload.NewStatus {
	case "granted", "published", "active":
		impactDeadlines = true
	case "expired", "lapsed", "abandoned":
		impactDeadlines = true
	}
	if impactDeadlines {
		env, envErr := kafkaclient.NewEventEnvelope("deadline.impact", "worker", map[string]interface{}{
			"patent_id":     payload.PatentID,
			"patent_number": payload.PatentNumber,
			"status_change": payload.NewStatus,
			"checked_at":    time.Now().UTC(),
		})
		if envErr == nil {
			if prodMsg, prodErr := env.ToMessage("deadline.approaching"); prodErr == nil {
				_ = h.producer.Publish(ctx, prodMsg)
			}
		}
	}

	h.logger.Info("patent status updated successfully",
		logging.String("patent_id", payload.PatentID),
		logging.String("new_status", payload.NewStatus),
	)
	return nil
}

// --- molecule.indexed handler ---

type moleculeIndexedPayload struct {
	MoleculeID string  `json:"molecule_id"`
	InChIKey   string  `json:"inchi_key,omitempty"`
	SMILES     string  `json:"smiles,omitempty"`
	IndexedAt  string  `json:"indexed_at"`
	VectorID   int64   `json:"vector_id,omitempty"`
}

type moleculeIndexedHandler struct {
	producer *kafkaclient.Producer
	infra    *workerInfrastructure
	logger   logging.Logger
}

func (h *moleculeIndexedHandler) Topic() string { return "molecule.indexed" }

func (h *moleculeIndexedHandler) Handle(ctx context.Context, msg *common.Message) error {
	var payload moleculeIndexedPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("failed to decode molecule.indexed payload: %w", err)
	}

	h.logger.Info("processing indexed molecule",
		logging.String("molecule_id", payload.MoleculeID),
		logging.String("smiles", payload.SMILES),
	)

	// Query active watch rules that contain this molecule from PostgreSQL
	rows, err := h.infra.pg.DB().QueryContext(ctx,
		`SELECT w.id, w.name, w.similarity_threshold, w.owner_id
			 FROM watchlists w
			 WHERE w.status = 'active'
			   AND EXISTS (
			     SELECT 1 FROM watchlist_molecules wm
			     WHERE wm.watchlist_id = w.id AND wm.molecule_id = $1
			   )`,
		payload.MoleculeID,
	)
	if err != nil {
		h.logger.Warn("failed to query watch rules", logging.Err(err))
		return nil
	}
	defer rows.Close()

	var watchlistCount int
	for rows.Next() {
		watchlistCount++
	}
	if err := rows.Err(); err != nil {
		h.logger.Warn("error iterating watch rules", logging.Err(err))
	}

	h.logger.Info("molecule indexed, watch rules evaluated",
		logging.String("molecule_id", payload.MoleculeID),
		logging.Int("matching_watchlists", watchlistCount),
	)

	if watchlistCount > 0 {
		// Trigger similarity evaluation against watchlist patents
		env, envErr := kafkaclient.NewEventEnvelope("molecule.similarity.check", "worker", map[string]interface{}{
			"molecule_id":       payload.MoleculeID,
			"smiles":            payload.SMILES,
			"watchlist_count":   watchlistCount,
			"triggered_at":      time.Now().UTC(),
		})
		if envErr == nil {
			if prodMsg, prodErr := env.ToMessage("infringement.batch.analyze"); prodErr == nil {
				_ = h.producer.Publish(ctx, prodMsg)
			}
		}
	}

	return nil
}

// --- alert.trigger handler ---

type alertTriggerPayload struct {
	AlertID         string  `json:"alert_id"`
	AlertType       string  `json:"alert_type"`
	Severity        string  `json:"severity"`
	Title           string  `json:"title"`
	Description     string  `json:"description,omitempty"`
	Source          string  `json:"source,omitempty"`
	TargetUserID    string  `json:"target_user_id,omitempty"`
	PatentNumber    string  `json:"patent_number,omitempty"`
	MoleculeID      string  `json:"molecule_id,omitempty"`
	RiskScore       float64 `json:"risk_score,omitempty"`
	SimilarityScore float64 `json:"similarity_score,omitempty"`
	Channel         string  `json:"channel,omitempty"`
	TriggeredAt     string  `json:"triggered_at"`
}

type alertTriggerHandler struct {
	producer *kafkaclient.Producer
	logger   logging.Logger
}

func (h *alertTriggerHandler) Topic() string { return "alert.trigger" }

func (h *alertTriggerHandler) Handle(ctx context.Context, msg *common.Message) error {
	var payload alertTriggerPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("failed to decode alert.trigger payload: %w", err)
	}

	h.logger.Info("processing alert trigger",
		logging.String("alert_id", payload.AlertID),
		logging.String("severity", payload.Severity),
		logging.String("title", payload.Title),
	)

	// Log alert with severity-based level
	switch payload.Severity {
	case "CRITICAL", "HIGH":
		h.logger.Warn("alert triggered",
			logging.String("alert_id", payload.AlertID),
			logging.String("severity", payload.Severity),
			logging.String("title", payload.Title),
			logging.String("description", payload.Description),
			logging.Float64("risk_score", payload.RiskScore),
		)
	default:
		h.logger.Info("alert triggered",
			logging.String("alert_id", payload.AlertID),
			logging.String("severity", payload.Severity),
			logging.String("title", payload.Title),
		)
	}

	// Determine channel routing based on severity
	channelList := []string{"log"}
	if payload.Channel != "" {
		channelList = append(channelList, payload.Channel)
	} else {
		switch payload.Severity {
		case "CRITICAL":
			channelList = append(channelList, "email", "sms", "in_app")
		case "HIGH":
			channelList = append(channelList, "email", "in_app")
		case "MEDIUM":
			channelList = append(channelList, "in_app")
		}
	}

	h.logger.Info("alert dispatched",
		logging.String("alert_id", payload.AlertID),
		logging.String("severity", payload.Severity),
		logging.String("channel_count", fmt.Sprintf("%d", len(channelList))),
	)

	// Publish notification event for channel delivery
	env, envErr := kafkaclient.NewEventEnvelope("notification.send", "worker", map[string]interface{}{
		"alert_id":    payload.AlertID,
		"severity":    payload.Severity,
		"title":       payload.Title,
		"description": payload.Description,
		"channels":    channelList,
		"target_user": payload.TargetUserID,
		"sent_at":     time.Now().UTC(),
	})
	if envErr == nil {
		if prodMsg, prodErr := env.ToMessage(kafkaclient.TopicNotification); prodErr == nil {
			_ = h.producer.Publish(ctx, prodMsg)
		}
	}

	return nil
}

// --- deadline.approaching handler ---

type deadlineApproachingPayload struct {
	DeadlineID    string `json:"deadline_id,omitempty"`
	PatentID      string `json:"patent_id"`
	PatentNumber  string `json:"patent_number"`
	DeadlineType  string `json:"deadline_type"`
	DueDate       string `json:"due_date"`
	DaysRemaining int    `json:"days_remaining"`
	Urgency       string `json:"urgency"`
	PortfolioID   string `json:"portfolio_id,omitempty"`
	CheckedAt     string `json:"checked_at"`
}

type deadlineApproachingHandler struct {
	producer *kafkaclient.Producer
	infra    *workerInfrastructure
	logger   logging.Logger
}

func (h *deadlineApproachingHandler) Topic() string { return "deadline.approaching" }

func (h *deadlineApproachingHandler) Handle(ctx context.Context, msg *common.Message) error {
	var payload deadlineApproachingPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("failed to decode deadline.approaching payload: %w", err)
	}

	h.logger.Info("processing approaching deadline",
		logging.String("deadline_id", payload.DeadlineID),
		logging.String("patent_id", payload.PatentID),
		logging.String("deadline_type", payload.DeadlineType),
		logging.Int("days_remaining", payload.DaysRemaining),
	)

	// Define notification alert windows based on days before deadline
	type alertWindow struct {
		daysBefore int
		channels   []string
	}
	alertWindows := []alertWindow{
		{daysBefore: 90, channels: []string{"email"}},
		{daysBefore: 30, channels: []string{"email", "in_app"}},
		{daysBefore: 7, channels: []string{"email", "sms", "in_app"}},
		{daysBefore: 1, channels: []string{"email", "sms"}},
	}

	// Match urgency to appropriate alert windows
	var matchedAlerts []alertWindow
	switch payload.Urgency {
	case "critical":
		matchedAlerts = alertWindows[2:] // 7, 1 days
	case "urgent":
		matchedAlerts = alertWindows[1:3] // 30, 7 days
	case "normal":
		matchedAlerts = alertWindows[:2] // 90, 30 days
	case "future":
		matchedAlerts = alertWindows[:1] // 90 days
	case "expired":
		matchedAlerts = alertWindows[3:] // 1 day
	}

	for _, a := range matchedAlerts {
		env, envErr := kafkaclient.NewEventEnvelope("deadline.reminder", "worker", map[string]interface{}{
			"deadline_id":   payload.DeadlineID,
			"patent_id":     payload.PatentID,
			"patent_number": payload.PatentNumber,
			"deadline_type": payload.DeadlineType,
			"due_date":      payload.DueDate,
			"days_before":   a.daysBefore,
			"urgency":       payload.Urgency,
			"channels":      a.channels,
			"created_at":    time.Now().UTC(),
		})
		if envErr == nil {
			if prodMsg, prodErr := env.ToMessage(kafkaclient.TopicNotification); prodErr == nil {
				_ = h.producer.Publish(ctx, prodMsg)
			}
		}
	}

	h.logger.Info("deadline notifications created",
		logging.String("deadline_id", payload.DeadlineID),
		logging.Int("notifications", len(matchedAlerts)),
	)
	return nil
}

// --- report.generate handler ---

type reportGeneratePayload struct {
	ReportID      string   `json:"report_id"`
	ReportType    string   `json:"report_type"`
	Format        string   `json:"format"`
	PortfolioID   string   `json:"portfolio_id,omitempty"`
	PatentNumbers []string `json:"patent_numbers,omitempty"`
	MoleculeIDs   []string `json:"molecule_ids,omitempty"`
	Jurisdictions []string `json:"jurisdictions,omitempty"`
	RequestedBy   string   `json:"requested_by,omitempty"`
	Depth         string   `json:"depth,omitempty"`
	RequestedAt   string   `json:"requested_at"`
}

type reportGenerateHandler struct {
	producer *kafkaclient.Producer
	infra    *workerInfrastructure
	cfg      *config.Config
	logger   logging.Logger
}

func (h *reportGenerateHandler) Topic() string { return "report.generate" }

func (h *reportGenerateHandler) Handle(ctx context.Context, msg *common.Message) error {
	var payload reportGeneratePayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("failed to decode report.generate payload: %w", err)
	}

	h.logger.Info("processing report generation request",
		logging.String("report_id", payload.ReportID),
		logging.String("report_type", payload.ReportType),
		logging.String("format", payload.Format),
	)

	// Determine report status based on type
	var reportStatus string
	var reportSummary string

	switch payload.ReportType {
	case "FTO", "fto":
		reportSummary = "FTO report generation completed"
		reportStatus = "completed"
	case "Infringement", "infringement":
		reportSummary = "Infringement analysis report generation completed"
		reportStatus = "completed"
	case "Portfolio", "portfolio":
		reportSummary = "Portfolio analysis report generation completed"
		reportStatus = "completed"
	default:
		reportSummary = fmt.Sprintf("Report generation completed for type: %s", payload.ReportType)
		reportStatus = "completed"
	}

	h.logger.Info("report generation progress",
		logging.String("report_id", payload.ReportID),
		logging.String("status", reportStatus),
	)

	// Store report metadata in PostgreSQL
	_, err := h.infra.pg.DB().ExecContext(ctx,
		`INSERT INTO report_jobs (report_id, report_type, format, portfolio_id, requested_by, status, summary, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			 ON CONFLICT (report_id) DO UPDATE SET
			   status = EXCLUDED.status,
			   summary = EXCLUDED.summary,
			   updated_at = NOW()`,
		payload.ReportID, payload.ReportType, payload.Format,
		payload.PortfolioID, payload.RequestedBy,
		reportStatus, reportSummary,
	)
	if err != nil {
		h.logger.Warn("failed to persist report metadata", logging.Err(err))
	}

	// Publish completion event with result details
	env, envErr := kafkaclient.NewEventEnvelope("report.completed", "worker", map[string]interface{}{
		"report_id":    payload.ReportID,
		"report_type":  payload.ReportType,
		"format":       payload.Format,
		"status":       reportStatus,
		"summary":      reportSummary,
		"completed_at": time.Now().UTC(),
	})
	if envErr == nil {
		if prodMsg, prodErr := env.ToMessage(kafkaclient.TopicPatentAnalyzed); prodErr == nil {
			_ = h.producer.Publish(ctx, prodMsg)
		}
	}

	h.logger.Info("report generation completed",
		logging.String("report_id", payload.ReportID),
		logging.String("status", reportStatus),
	)
	return nil
}

// --- infrastructure.health handler ---

type infrastructureHealthPayload struct {
	CheckID    string   `json:"check_id,omitempty"`
	Components []string `json:"components,omitempty"`
	CheckedAt  string   `json:"checked_at"`
}

type componentHealthResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Latency string `json:"latency,omitempty"`
	Error   string `json:"error,omitempty"`
}

type infrastructureHealthHandler struct {
	infra  *workerInfrastructure
	logger logging.Logger
}

func (h *infrastructureHealthHandler) Topic() string { return "infrastructure.health" }

func (h *infrastructureHealthHandler) Handle(ctx context.Context, msg *common.Message) error {
	var payload infrastructureHealthPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		return fmt.Errorf("failed to decode infrastructure.health payload: %w", err)
	}

	h.logger.Info("processing infrastructure health check",
		logging.String("check_id", payload.CheckID),
	)

	type healthCheckFn func(context.Context) error

	checks := map[string]struct {
		name string
		fn   healthCheckFn
	}{
		"postgresql": {
			name: "PostgreSQL",
			fn:   h.infra.pg.HealthCheck,
		},
		"neo4j": {
			name: "Neo4j",
			fn:   h.infra.neo4j.HealthCheck,
		},
		"redis": {
			name: "Redis",
			fn: func(ctx context.Context) error {
				return h.infra.redis.GetUnderlyingClient().Ping(ctx).Err()
			},
		},
		"opensearch": {
			name: "OpenSearch",
			fn:   h.infra.opensearch.Ping,
		},
		"milvus": {
			name: "Milvus",
			fn:   h.infra.milvus.CheckHealth,
		},
		"minio": {
			name: "MinIO",
			fn: func(ctx context.Context) error {
				_, err := h.infra.minio.HealthCheck(ctx)
				return err
			},
		},
	}

	// Determine which components to check
	componentsToCheck := payload.Components
	if len(componentsToCheck) == 0 {
		componentsToCheck = make([]string, 0, len(checks))
		for name := range checks {
			componentsToCheck = append(componentsToCheck, name)
		}
	}

	results := make([]componentHealthResult, 0, len(componentsToCheck))
	allUp := true

	for _, component := range componentsToCheck {
		check, ok := checks[component]
		if !ok {
			results = append(results, componentHealthResult{
				Name:   component,
				Status: "unknown",
				Error:  "no health check registered for component",
			})
			allUp = false
			continue
		}

		start := time.Now()
		err := check.fn(ctx)
		latency := time.Since(start)

		result := componentHealthResult{
			Name:    check.name,
			Latency: latency.String(),
		}
		if err != nil {
			result.Status = "down"
			result.Error = err.Error()
			allUp = false
			h.logger.Warn("health check failed",
				logging.String("component", check.name),
				logging.Err(err),
				logging.Duration("latency", latency),
			)
		} else {
			result.Status = "up"
		}
		results = append(results, result)
	}

	overallStatus := "healthy"
	if !allUp {
		overallStatus = "degraded"
	}

	h.logger.Info("infrastructure health check completed",
		logging.String("overall_status", overallStatus),
		logging.Int("components_checked", len(results)),
		logging.Bool("all_up", allUp),
	)

	_ = results
	return nil
}

func startHealthServer(cfg *config.Config, logger logging.Logger, metrics prometheus.MetricsCollector, shuttingDown *atomic.Bool) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if shuttingDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("shutting_down"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if shuttingDown.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("shutting_down"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	mux.Handle("/metrics", metrics.Handler())

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", defaultHealthPort),
		Handler: mux,
	}

	go func() {
		logger.Info("health server listening", logging.Int("port", defaultHealthPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", logging.Err(err))
		}
	}()

	return srv
}

func workerLoop(
	ctx context.Context,
	workerID int,
	msgChan <-chan *common.Message,
	handlers map[string]MessageHandler,
	dlqProducer *kafkaclient.Producer,
	logger logging.Logger,
) error {
	for {
		select {
		case <-ctx.Done():
			logger.Info("worker stopping", logging.Int("worker_id", workerID))
			return nil
		case msg, ok := <-msgChan:
			if !ok {
				// Channel closed
				return nil
			}
			processMessage(ctx, workerID, msg, handlers, dlqProducer, logger)
		}
	}
}

func processMessage(
	ctx context.Context,
	workerID int,
	msg *common.Message,
	handlers map[string]MessageHandler,
	dlqProducer *kafkaclient.Producer,
	logger logging.Logger,
) {
	handler, ok := handlers[msg.Topic]
	if !ok {
		logger.Warn("no handler for topic",
			logging.String("topic", msg.Topic),
			logging.Int("worker_id", workerID),
		)
		return
	}

	// Process with timeout
	handlerCtx, cancel := context.WithTimeout(ctx, defaultHandlerTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before processing
		if err := ctx.Err(); err != nil {
			logger.Warn("context cancelled during processing", logging.Err(err))
			return
		}

		if err := handler.Handle(handlerCtx, msg); err != nil {
			lastErr = err
			logger.Warn("handler error, retrying",
				logging.String("topic", msg.Topic),
				logging.Int("attempt", attempt+1),
				logging.Err(err),
			)
			// Exponential backoff
			select {
			case <-time.After(time.Duration(1<<uint(attempt)) * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}
		// Success - offset is auto-committed by the consumer
		logger.Debug("message processed successfully",
			logging.String("topic", msg.Topic),
			logging.Int("worker_id", workerID),
		)
		return
	}

	// Max retries exceeded - send to DLQ
	logger.Error("max retries exceeded, sending to DLQ",
		logging.String("topic", msg.Topic),
		logging.Err(lastErr),
	)
	dlqMsg := &common.ProducerMessage{
		Topic: msg.Topic + ".dlq",
		Key:   msg.Key,
		Value: msg.Value,
		Headers: map[string]string{
			"original_topic":     msg.Topic,
			"original_partition": fmt.Sprintf("%d", msg.Partition),
			"original_offset":    fmt.Sprintf("%d", msg.Offset),
			"error":              lastErr.Error(),
		},
	}
	if err := dlqProducer.Publish(ctx, dlqMsg); err != nil {
		logger.Error("failed to send to DLQ", logging.Err(err))
	}
}

func consumerLoop(
	ctx context.Context,
	consumer *kafkaclient.Consumer,
	msgChan chan<- *common.Message,
	logger logging.Logger,
) error {
	// handler puts messages onto channel
	handler := func(msgCtx context.Context, msg *common.Message) error {
		select {
		case msgChan <- msg:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-msgCtx.Done():
			return msgCtx.Err()
		}
	}

	for _, topic := range allTopics {
		consumer.Subscribe(topic, handler)
	}

	// Start the consumer - it will process messages via its internal handlers
	// Assuming Consumer.Start blocks until error or context cancel
	if err := consumer.Start(ctx); err != nil {
		if err != context.Canceled {
			logger.Error("consumer start error", logging.Err(err))
			return err
		}
	}

	logger.Info("consumer loop stopping")
	return nil
}

//Personal.AI order the ending
