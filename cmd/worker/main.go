package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/milvus"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/search/opensearch"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/storage/minio"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// TaskHandler interface for processing messages.
type TaskHandler interface {
	Handle(ctx context.Context, msg *kafka.Message) error
	Topic() string
}

// TaskRouter routes messages to handlers.
type TaskRouter struct {
	handlers map[string]TaskHandler
	mu       sync.RWMutex
}

func NewTaskRouter() *TaskRouter {
	return &TaskRouter{
		handlers: make(map[string]TaskHandler),
	}
}

func (r *TaskRouter) Register(handler TaskHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[handler.Topic()] = handler
}

func (r *TaskRouter) Dispatch(ctx context.Context, msg *kafka.Message) error {
	r.mu.RLock()
	handler, ok := r.handlers[msg.Topic]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler for topic: %s", msg.Topic)
	}
	return handler.Handle(ctx, msg)
}

// Metrics
var (
	taskTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "keyip_worker_tasks_total",
			Help: "Total number of tasks processed",
		},
		[]string{"topic", "status"},
	)
	taskDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "keyip_worker_task_duration_seconds",
			Help: "Duration of task processing",
		},
		[]string{"topic"},
	)
	activeTasks = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "keyip_worker_active_tasks",
			Help: "Current number of active tasks",
		},
	)
)

func init() {
	prometheus.MustRegister(taskTotal, taskDuration, activeTasks)
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

	if cfg.Worker.Concurrency == 0 {
		cfg.Worker.Concurrency = runtime.NumCPU()
	}
	if cfg.Worker.ConsumerGroup == "" {
		cfg.Worker.ConsumerGroup = "keyip-worker-group"
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

	logger.Info("Starting KeyIP-Intelligence Worker",
		logging.String("version", Version),
		logging.String("concurrency", fmt.Sprintf("%d", cfg.Worker.Concurrency)),
		logging.String("consumer_group", cfg.Worker.ConsumerGroup),
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
	db := dbConn.DB() // Use raw DB for ping

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
	_ = minioClient // suppress unused

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
	_ = opensearchClient // suppress unused

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
	_ = milvusClient // suppress unused

	// Kafka Consumer
	consumerCfg := kafka.ConsumerConfig{
		Brokers: cfg.Messaging.Kafka.Brokers,
		GroupID: cfg.Worker.ConsumerGroup,
		Topics:  cfg.Worker.Topics,
	}
	consumer, err := kafka.NewConsumer(consumerCfg, logger)
	if err != nil {
		logger.Fatal("Failed to create Kafka consumer", logging.Error(err))
	}
	// consumer.Close() called on shutdown

	// Task Router
	router := NewTaskRouter()

	// Register Handlers
	// For now, we are just instantiating infrastructure. Real handlers would be injected here.
	// Example: router.Register(NewPatentIngestHandler(patentService, logger))

	// Subscribe
	for _, topic := range cfg.Worker.Topics {
		err := consumer.Subscribe(topic, func(ctx context.Context, msg *kafka.Message) error {
			activeTasks.Inc()
			start := time.Now()

			// Process message
			err := router.Dispatch(ctx, msg)

			duration := time.Since(start).Seconds()
			taskDuration.WithLabelValues(msg.Topic).Observe(duration)
			activeTasks.Dec()

			if err != nil {
				taskTotal.WithLabelValues(msg.Topic, "failed").Inc()
				return err
			}
			taskTotal.WithLabelValues(msg.Topic, "success").Inc()
			return nil
		})
		if err != nil {
			logger.Fatal("Failed to subscribe to topic", logging.String("topic", topic), logging.Error(err))
		}
	}

	// Start Consumer
	if err := consumer.Start(context.Background()); err != nil {
		logger.Fatal("Failed to start consumer", logging.Error(err))
	}

	// Signal handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Health Check
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			http.Error(w, "Database not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
	})
	healthMux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:    ":9092",
		Handler: healthMux,
	}

	go func() {
		logger.Info("Starting health/metrics server", logging.String("addr", ":9092"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Health server failed", logging.Error(err))
		}
	}()

	// Wait for signal
	<-stop
	logger.Info("Shutting down worker...")
	
	srv.Shutdown(context.Background())
	consumer.Close()

	logger.Info("Shutdown complete")
}

//Personal.AI order the ending
