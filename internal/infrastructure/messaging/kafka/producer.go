package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
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
	ErrProducerClosed = errors.New(errors.ErrCodeInternal, "producer closed")
	ErrPublishFailed  = errors.New(errors.ErrCodeInternal, "publish failed")
)

// ProducerConfig holds configuration for the Producer.
type ProducerConfig struct {
	Brokers          []string
	Acks             string
	MaxRetries       int
	RetryBackoff     time.Duration
	BatchSize        int
	BatchTimeout     time.Duration
	MaxMessageBytes  int
	CompressionCodec string
	Idempotent       bool
	WriteTimeout     time.Duration
	ReadTimeout      time.Duration
	SASLEnabled      bool
	SASLMechanism    string
	SASLUsername     string
	SASLPassword     string
	TLSEnabled       bool
	TLSCertPath      string
	AsyncErrorHandler func(err error, msg *common.ProducerMessage)
}

// ProducerMetrics holds producer metrics.
type ProducerMetrics struct {
	MessagesSent   atomic.Int64
	MessagesFailed atomic.Int64
	BytesSent      atomic.Int64
	LastSentAt     atomic.Value // time.Time
	AvgLatencyMs   atomic.Int64
}

// WriterInterface abstracts kafka.Writer for testing.
type WriterInterface interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
	Stats() kafka.WriterStats
}

// Producer manages message production.
type Producer struct {
	writer WriterInterface
	config ProducerConfig
	logger logging.Logger
	closed atomic.Bool
	metrics *ProducerMetrics
}

// NewProducer creates a new Producer.
func NewProducer(cfg ProducerConfig, logger logging.Logger) (*Producer, error) {
	if err := ValidateProducerConfig(cfg); err != nil {
		return nil, err
	}

	// Defaults
	if cfg.MaxRetries == 0 { cfg.MaxRetries = 3 }
	if cfg.RetryBackoff == 0 { cfg.RetryBackoff = 100 * time.Millisecond }
	if cfg.BatchSize == 0 { cfg.BatchSize = 100 }
	if cfg.BatchTimeout == 0 { cfg.BatchTimeout = 1 * time.Second }
	if cfg.MaxMessageBytes == 0 { cfg.MaxMessageBytes = 1024 * 1024 } // 1MB
	if cfg.WriteTimeout == 0 { cfg.WriteTimeout = 10 * time.Second }
	if cfg.ReadTimeout == 0 { cfg.ReadTimeout = 10 * time.Second }

	// Transport
	transport := &kafka.Transport{
		DialTimeout: 10 * time.Second,
	}
	if cfg.TLSEnabled {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
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
		transport.TLS = tlsConfig
	}
	if cfg.SASLEnabled {
		var mech sasl.Mechanism
		var err error
		switch cfg.SASLMechanism {
		case "PLAIN":
			mech = plain.Mechanism{Username: cfg.SASLUsername, Password: cfg.SASLPassword}
		case "SCRAM-SHA-256":
			mech, err = scram.Mechanism(scram.SHA256, cfg.SASLUsername, cfg.SASLPassword)
		case "SCRAM-SHA-512":
			mech, err = scram.Mechanism(scram.SHA512, cfg.SASLUsername, cfg.SASLPassword)
		}
		if err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to create SASL mechanism")
		}
		transport.SASL = mech
	}

	// Writer Config logic
	balancer := &kafka.Hash{}
	maxAttempts := cfg.MaxRetries + 1
	batchSize := cfg.BatchSize
	batchTimeout := cfg.BatchTimeout
	writeTimeout := cfg.WriteTimeout
	readTimeout := cfg.ReadTimeout

	// Acks
	var requiredAcks kafka.RequiredAcks
	switch cfg.Acks {
	case "none": requiredAcks = kafka.RequireNone
	case "all":  requiredAcks = kafka.RequireAll
	default:     requiredAcks = kafka.RequireOne
	}

	// Compression
	var compression kafka.Compression
	switch cfg.CompressionCodec {
	case "gzip":   compression = kafka.Gzip
	case "snappy": compression = kafka.Snappy
	case "lz4":    compression = kafka.Lz4
	case "zstd":   compression = kafka.Zstd
	default:       compression = kafka.Compression(0) // None
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     balancer,
		MaxAttempts:  maxAttempts,
		BatchSize:    batchSize,
		BatchTimeout: batchTimeout,
		WriteTimeout: writeTimeout,
		ReadTimeout:  readTimeout,
		RequiredAcks: requiredAcks,
		Compression:  compression,
		Transport:    transport,
	}

	return &Producer{
		writer:  writer,
		config:  cfg,
		logger:  logger,
		metrics: &ProducerMetrics{},
	}, nil
}

// Publish publishes a single message.
func (p *Producer) Publish(ctx context.Context, msg *common.ProducerMessage) error {
	if p.closed.Load() {
		return ErrProducerClosed
	}
	if msg.Topic == "" {
		return errors.New(errors.ErrCodeValidation, "Topic required")
	}
	if len(msg.Value) == 0 {
		return errors.New(errors.ErrCodeValidation, "Value required")
	}
	if len(msg.Value) > p.config.MaxMessageBytes {
		return errors.New(errors.ErrCodeValidation, "Message too large")
	}

	kMsg := p.toKafkaMessage(msg)

	start := time.Now()
	err := p.writer.WriteMessages(ctx, kMsg)
	if err != nil {
		p.metrics.MessagesFailed.Add(1)
		return errors.Wrap(err, errors.ErrCodeInternal, "publish failed")
	}

	p.metrics.MessagesSent.Add(1)
	p.metrics.BytesSent.Add(int64(len(msg.Value)))
	p.metrics.LastSentAt.Store(time.Now())

	latency := time.Since(start).Milliseconds()
	p.metrics.AvgLatencyMs.Store(latency)

	p.logger.Debug("Message published",
		logging.String("topic", msg.Topic),
		logging.Int64("latency_ms", latency))
	return nil
}

// PublishBatch publishes multiple messages.
func (p *Producer) PublishBatch(ctx context.Context, msgs []*common.ProducerMessage) (*common.BatchPublishResult, error) {
	if p.closed.Load() {
		return nil, ErrProducerClosed
	}
	if len(msgs) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "Messages empty")
	}

	kMsgs := make([]kafka.Message, len(msgs))
	for i, msg := range msgs {
		kMsgs[i] = p.toKafkaMessage(msg)
	}

	result := &common.BatchPublishResult{}

	err := p.writer.WriteMessages(ctx, kMsgs...)
	if err != nil {
		if writeErrs, ok := err.(kafka.WriteErrors); ok {
			for i, we := range writeErrs {
				if we != nil {
					result.Failed++
					result.Errors = append(result.Errors, common.BatchItemError{
						Index: i,
						Topic: msgs[i].Topic,
						Error: we,
					})
				} else {
					result.Succeeded++
				}
			}
		} else {
			// Generic error (all failed)
			result.Failed = len(msgs)
			result.Errors = append(result.Errors, common.BatchItemError{
				Index: -1,
				Error: err,
			})
		}
	} else {
		result.Succeeded = len(msgs)
	}

	p.metrics.MessagesSent.Add(int64(result.Succeeded))
	p.metrics.MessagesFailed.Add(int64(result.Failed))

	p.logger.Info("Batch published",
		logging.Int("succeeded", result.Succeeded),
		logging.Int("failed", result.Failed))

	return result, nil
}

// PublishAsync publishes asynchronously.
func (p *Producer) PublishAsync(ctx context.Context, msg *common.ProducerMessage) {
	go func() {
		err := p.Publish(ctx, msg)
		if err != nil && p.config.AsyncErrorHandler != nil {
			p.config.AsyncErrorHandler(err, msg)
		}
	}()
}

// GetMetrics returns metrics snapshot.
func (p *Producer) GetMetrics() ProducerMetrics {
	m := ProducerMetrics{}
	m.MessagesSent.Store(p.metrics.MessagesSent.Load())
	m.MessagesFailed.Store(p.metrics.MessagesFailed.Load())
	m.BytesSent.Store(p.metrics.BytesSent.Load())
	m.AvgLatencyMs.Store(p.metrics.AvgLatencyMs.Load())
	m.LastSentAt.Store(p.metrics.LastSentAt.Load())
	return m
}

// Close closes the producer.
func (p *Producer) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	err := p.writer.Close()
	p.logger.Info("Kafka producer closed", logging.Int64("sent", p.metrics.MessagesSent.Load()))
	return err
}

func (p *Producer) toKafkaMessage(msg *common.ProducerMessage) kafka.Message {
	headers := make([]kafka.Header, 0, len(msg.Headers))
	for k, v := range msg.Headers {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}

	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	return kafka.Message{
		Topic:     msg.Topic,
		Key:       msg.Key,
		Value:     msg.Value,
		Headers:   headers,
		Time:      ts,
		Partition: msg.Partition,
	}
}

func ValidateProducerConfig(cfg ProducerConfig) error {
	if len(cfg.Brokers) == 0 {
		return errors.New(errors.ErrCodeValidation, "Brokers required")
	}
	if cfg.MaxRetries < 0 {
		return errors.New(errors.ErrCodeValidation, "MaxRetries must be >= 0")
	}
	return nil
}
