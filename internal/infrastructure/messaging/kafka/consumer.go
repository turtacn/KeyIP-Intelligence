package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

var (
	ErrAlreadyRunning = errors.New(errors.ErrCodeConflict, "consumer already running")
	ErrConsumerClosed = errors.New(errors.ErrCodeInternal, "consumer closed")
)

// RetryConfig defines retry behavior.
type RetryConfig struct {
	MaxRetries      int
	RetryBackoff    time.Duration
	MaxRetryBackoff time.Duration
	DeadLetterTopic string
}

// ConsumerConfig holds configuration for the Consumer.
type ConsumerConfig struct {
	Brokers           []string
	GroupID           string
	Topics            []string
	AutoOffsetReset   string
	EnableAutoCommit  bool
	AutoCommitInterval time.Duration
	SessionTimeout    time.Duration
	HeartbeatInterval time.Duration
	MaxPollInterval   time.Duration
	FetchMinBytes     int
	FetchMaxBytes     int
	MaxPollRecords    int
	IsolationLevel    string
	SASLEnabled       bool
	SASLMechanism     string
	SASLUsername      string
	SASLPassword      string
	TLSEnabled        bool
	TLSCertPath       string
	RetryConfig       RetryConfig
}

// ConsumerMetrics holds consumer metrics.
type ConsumerMetrics struct {
	MessagesConsumed     atomic.Int64
	MessagesProcessed    atomic.Int64
	MessagesFailed       atomic.Int64
	MessagesRetried      atomic.Int64
	MessagesDeadLettered atomic.Int64
	LastConsumedAt       atomic.Value // time.Time
	Lag                  atomic.Int64
}

// ReaderInterface abstracts kafka.Reader for testing.
type ReaderInterface interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
	Stats() kafka.ReaderStats
}

// Consumer manages message consumption.
type Consumer struct {
	reader ReaderInterface
	config ConsumerConfig
	logger logging.Logger

	handlers map[string]common.MessageHandler
	mu       sync.RWMutex

	running atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	deadLetterProducer *Producer
	metrics            *ConsumerMetrics
}

// NewConsumer creates a new Consumer.
func NewConsumer(cfg ConsumerConfig, logger logging.Logger) (*Consumer, error) {
	if err := ValidateConsumerConfig(cfg); err != nil {
		return nil, err
	}

	// Defaults
	if cfg.AutoOffsetReset == "" { cfg.AutoOffsetReset = "earliest" }
	if cfg.AutoCommitInterval == 0 { cfg.AutoCommitInterval = 5 * time.Second }
	if cfg.SessionTimeout == 0 { cfg.SessionTimeout = 30 * time.Second }
	if cfg.HeartbeatInterval == 0 { cfg.HeartbeatInterval = 3 * time.Second }
	if cfg.MaxPollInterval == 0 { cfg.MaxPollInterval = 300 * time.Second }
	if cfg.FetchMinBytes == 0 { cfg.FetchMinBytes = 1 }
	if cfg.FetchMaxBytes == 0 { cfg.FetchMaxBytes = 50 * 1024 * 1024 } // 50MB

	// Create Reader Config
	readerCfg := kafka.ReaderConfig{
		Brokers:           cfg.Brokers,
		GroupID:           cfg.GroupID,
		GroupTopics:       cfg.Topics,
		MinBytes:          cfg.FetchMinBytes,
		MaxBytes:          cfg.FetchMaxBytes,
		MaxWait:           cfg.MaxPollInterval, // MaxWait roughly equals poll interval
		CommitInterval:    cfg.AutoCommitInterval,
		SessionTimeout:    cfg.SessionTimeout,
		HeartbeatInterval: cfg.HeartbeatInterval,
		StartOffset:       kafka.FirstOffset,
	}
	if cfg.AutoOffsetReset == "latest" {
		readerCfg.StartOffset = kafka.LastOffset
	}

	// Dialer with TLS/SASL
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}
	if cfg.TLSEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true, // For now
		}
		if cfg.TLSCertPath != "" {
			caCert, err := os.ReadFile(cfg.TLSCertPath)
			if err == nil {
				caCertPool := x509.NewCertPool()
				caCertPool.AppendCertsFromPEM(caCert)
				tlsConfig.RootCAs = caCertPool
				tlsConfig.InsecureSkipVerify = false
			}
		}
		dialer.TLS = tlsConfig
	}

	if cfg.SASLEnabled {
		var mech sasl.Mechanism
		var err error
		switch cfg.SASLMechanism {
		case "PLAIN":
			mech = plain.Mechanism{
				Username: cfg.SASLUsername,
				Password: cfg.SASLPassword,
			}
		case "SCRAM-SHA-256":
			mech, err = scram.Mechanism(scram.SHA256, cfg.SASLUsername, cfg.SASLPassword)
		case "SCRAM-SHA-512":
			mech, err = scram.Mechanism(scram.SHA512, cfg.SASLUsername, cfg.SASLPassword)
		}
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to create SASL mechanism")
		}
		dialer.SASLMechanism = mech
	}
	readerCfg.Dialer = dialer

	if cfg.IsolationLevel == "read_committed" {
		readerCfg.IsolationLevel = kafka.ReadCommitted
	}

	reader := kafka.NewReader(readerCfg)

	var dlProducer *Producer
	if cfg.RetryConfig.DeadLetterTopic != "" {
		// Create dead letter producer
		// Reusing broker config
		dlCfg := ProducerConfig{
			Brokers:       cfg.Brokers,
			SASLEnabled:   cfg.SASLEnabled,
			SASLMechanism: cfg.SASLMechanism,
			SASLUsername:  cfg.SASLUsername,
			SASLPassword:  cfg.SASLPassword,
			TLSEnabled:    cfg.TLSEnabled,
			TLSCertPath:   cfg.TLSCertPath,
		}
		// Assuming NewProducer handles defaults
		p, err := NewProducer(dlCfg, logger)
		if err != nil {
			return nil, err
		}
		dlProducer = p
	}

	return &Consumer{
		reader:             reader,
		config:             cfg,
		logger:             logger,
		handlers:           make(map[string]common.MessageHandler),
		deadLetterProducer: dlProducer,
		metrics:            &ConsumerMetrics{},
	}, nil
}

// Subscribe subscribes to a topic.
func (c *Consumer) Subscribe(topic string, handler common.MessageHandler) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[topic] = handler
	c.logger.Info("Subscribed to topic", logging.String("topic", topic))
	return nil
}

// Unsubscribe unsubscribes from a topic.
func (c *Consumer) Unsubscribe(topic string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.handlers, topic)
	c.logger.Info("Unsubscribed from topic", logging.String("topic", topic))
	return nil
}

// Start starts the consumer loop.
func (c *Consumer) Start(ctx context.Context) error {
	if c.running.Swap(true) {
		return ErrAlreadyRunning
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(1)

	go c.consumeLoop(ctx)

	c.logger.Info("Kafka consumer started", logging.String("group", c.config.GroupID))
	return nil
}

func (c *Consumer) consumeLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		if ctx.Err() != nil {
			return
		}

		m, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled
			}
			c.logger.Error("FetchMessage error", logging.Error(err))
			time.Sleep(time.Second) // prevent busy loop on error
			continue
		}

		c.metrics.MessagesConsumed.Add(1)
		c.metrics.LastConsumedAt.Store(time.Now())
		c.metrics.Lag.Store(m.HighWaterMark - m.Offset)

		// Convert to Message
		msg := &common.Message{
			Topic:     m.Topic,
			Partition: m.Partition,
			Offset:    m.Offset,
			Key:       m.Key,
			Value:     m.Value,
			Timestamp: m.Time,
			Headers:   make(map[string]string),
		}
		for _, h := range m.Headers {
			msg.Headers[h.Key] = string(h.Value)
		}

		c.mu.RLock()
		handler, ok := c.handlers[m.Topic]
		c.mu.RUnlock()

		if !ok {
			c.logger.Warn("No handler for topic", logging.String("topic", m.Topic))
			// Commit anyway to move forward? Usually yes.
			c.reader.CommitMessages(ctx, m)
			continue
		}

		// Process
		if err := c.processMessage(ctx, msg, handler); err == nil {
			// Success
			c.metrics.MessagesProcessed.Add(1)
			if !c.config.EnableAutoCommit {
				if err := c.reader.CommitMessages(ctx, m); err != nil {
					c.logger.Error("CommitMessages failed", logging.Error(err))
				}
			}
		} else {
			// Failed after retries and DLQ logic handled inside processMessage
			// Just log
			c.metrics.MessagesFailed.Add(1)
			// Should we commit? If sent to DLQ, yes. If dropped, yes.
			// Only if processMessage returns error do we NOT commit?
			// But processMessage implementation logic says "return nil (不阻塞消费进度)".
			// So we always commit if processMessage returns nil.
			if !c.config.EnableAutoCommit {
				c.reader.CommitMessages(ctx, m)
			}
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg *common.Message, handler common.MessageHandler) error {
	// First attempt
	err := handler(ctx, msg)
	if err == nil {
		return nil
	}

	// Retry loop
	maxRetries := c.config.RetryConfig.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // Default if not set
	}

	backoff := c.config.RetryConfig.RetryBackoff
	if backoff == 0 {
		backoff = 1 * time.Second
	}
	maxBackoff := c.config.RetryConfig.MaxRetryBackoff
	if maxBackoff == 0 {
		maxBackoff = 30 * time.Second
	}

	for i := 0; i < maxRetries; i++ {
		c.metrics.MessagesRetried.Add(1)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		err = handler(ctx, msg)
		if err == nil {
			return nil
		}

		// Exponential backoff
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}

	// Retries exhausted
	c.logger.Error("Message processing failed after retries",
		logging.String("topic", msg.Topic),
		logging.Int64("offset", msg.Offset),
		logging.Error(err))

	if c.deadLetterProducer != nil && c.config.RetryConfig.DeadLetterTopic != "" {
		// Send to DLQ
		dlMsg := &common.ProducerMessage{
			Topic: c.config.RetryConfig.DeadLetterTopic,
			Key:   msg.Key,
			Value: msg.Value,
			Headers: msg.Headers,
			// Add error info to headers?
		}
		dlMsg.Headers["original_topic"] = msg.Topic
		dlMsg.Headers["error_message"] = err.Error()

		if dlErr := c.deadLetterProducer.Publish(ctx, dlMsg); dlErr != nil {
			c.logger.Error("Failed to send to dead letter queue", logging.Error(dlErr))
			return nil
		}
		c.metrics.MessagesDeadLettered.Add(1)
	}

	return nil // Handled (dropped or DLQ'd)
}

// GetMetrics returns a snapshot of metrics.
func (c *Consumer) GetMetrics() ConsumerMetrics {
	m := ConsumerMetrics{
		MessagesConsumed:     atomic.Int64{},
		MessagesProcessed:    atomic.Int64{},
		MessagesFailed:       atomic.Int64{},
		MessagesRetried:      atomic.Int64{},
		MessagesDeadLettered: atomic.Int64{},
		Lag:                  atomic.Int64{},
	}
	m.MessagesConsumed.Store(c.metrics.MessagesConsumed.Load())
	m.MessagesProcessed.Store(c.metrics.MessagesProcessed.Load())
	m.MessagesFailed.Store(c.metrics.MessagesFailed.Load())
	m.MessagesRetried.Store(c.metrics.MessagesRetried.Load())
	m.MessagesDeadLettered.Store(c.metrics.MessagesDeadLettered.Load())
	m.Lag.Store(c.metrics.Lag.Load())
	m.LastConsumedAt.Store(c.metrics.LastConsumedAt.Load())
	return m
}

// Close closes the consumer.
func (c *Consumer) Close() error {
	if !c.running.CompareAndSwap(true, false) {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	if c.reader != nil {
		c.reader.Close()
	}
	if c.deadLetterProducer != nil {
		c.deadLetterProducer.Close()
	}

	c.logger.Info("Kafka consumer closed",
		logging.Int64("consumed", c.metrics.MessagesConsumed.Load()))
	return nil
}

// ValidateConsumerConfig validates configuration.
func ValidateConsumerConfig(cfg ConsumerConfig) error {
	if len(cfg.Brokers) == 0 {
		return errors.New(errors.ErrCodeValidation, "Brokers required")
	}
	if cfg.GroupID == "" {
		return errors.New(errors.ErrCodeValidation, "GroupID required")
	}
	if cfg.AutoOffsetReset != "" && cfg.AutoOffsetReset != "earliest" && cfg.AutoOffsetReset != "latest" {
		return errors.New(errors.ErrCodeValidation, "Invalid AutoOffsetReset")
	}
	if cfg.SASLEnabled {
		if cfg.SASLMechanism == "" {
			return errors.New(errors.ErrCodeValidation, "SASLMechanism required")
		}
		if cfg.SASLUsername == "" || cfg.SASLPassword == "" {
			return errors.New(errors.ErrCodeValidation, "SASL credentials required")
		}
	}
	if cfg.TLSEnabled && cfg.TLSCertPath == "" {
		return errors.New(errors.ErrCodeValidation, "TLSCertPath required")
	}
	if cfg.RetryConfig.MaxRetries < 0 {
		return errors.New(errors.ErrCodeValidation, "MaxRetries must be >= 0")
	}
	return nil
}
