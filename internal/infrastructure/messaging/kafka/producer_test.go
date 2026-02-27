package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// mockKafkaWriter
type mockKafkaWriter struct {
	writeFunc func(ctx context.Context, msgs ...kafka.Message) error
	closeFunc func() error
	statsFunc func() kafka.WriterStats
}

func (m *mockKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.writeFunc != nil {
		return m.writeFunc(ctx, msgs...)
	}
	return nil
}

func (m *mockKafkaWriter) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockKafkaWriter) Stats() kafka.WriterStats {
	if m.statsFunc != nil {
		return m.statsFunc()
	}
	return kafka.WriterStats{}
}

func newTestProducerConfig() ProducerConfig {
	return ProducerConfig{
		Brokers:         []string{"localhost:9092"},
		MaxMessageBytes: 1024 * 1024,
	}
}

func newTestProducerMessage(topic, key, value string) *common.ProducerMessage {
	return &common.ProducerMessage{
		Topic: topic,
		Key:   []byte(key),
		Value: []byte(value),
	}
}

func newTestProducer(mockWriter WriterInterface) *Producer {
	return &Producer{
		writer:  mockWriter,
		config:  newTestProducerConfig(),
		logger:  newMockLogger(),
		metrics: &ProducerMetrics{},
	}
}

func TestValidateProducerConfig_Valid(t *testing.T) {
	cfg := newTestProducerConfig()
	err := ValidateProducerConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateProducerConfig_EmptyBrokers(t *testing.T) {
	cfg := newTestProducerConfig()
	cfg.Brokers = nil
	err := ValidateProducerConfig(cfg)
	assert.Error(t, err)
}

func TestPublish_Success(t *testing.T) {
	var capturedMsgs []kafka.Message
	mock := &mockKafkaWriter{
		writeFunc: func(ctx context.Context, msgs ...kafka.Message) error {
			capturedMsgs = msgs
			return nil
		},
	}
	p := newTestProducer(mock)
	msg := newTestProducerMessage("test", "k", "v")
	err := p.Publish(context.Background(), msg)
	assert.NoError(t, err)
	assert.Len(t, capturedMsgs, 1)
	assert.Equal(t, "test", capturedMsgs[0].Topic)
	assert.Equal(t, "k", string(capturedMsgs[0].Key))
	assert.Equal(t, "v", string(capturedMsgs[0].Value))
	assert.Equal(t, int64(1), p.metrics.MessagesSent.Load())
}

func TestPublish_Failure(t *testing.T) {
	mock := &mockKafkaWriter{
		writeFunc: func(ctx context.Context, msgs ...kafka.Message) error {
			return errors.New("write failed")
		},
	}
	p := newTestProducer(mock)
	msg := newTestProducerMessage("test", "k", "v")
	err := p.Publish(context.Background(), msg)
	assert.Error(t, err)
	assert.Equal(t, int64(1), p.metrics.MessagesFailed.Load())
}

func TestPublishBatch_PartialFailure(t *testing.T) {
	mock := &mockKafkaWriter{
		writeFunc: func(ctx context.Context, msgs ...kafka.Message) error {
			// Simulate WriteErrors
			// kafka.WriteErrors is []error
			errs := make(kafka.WriteErrors, len(msgs))
			errs[0] = nil
			errs[1] = errors.New("fail")
			return errs
		},
	}
	p := newTestProducer(mock)
	msgs := []*common.ProducerMessage{
		newTestProducerMessage("test", "1", "1"),
		newTestProducerMessage("test", "2", "2"),
	}
	res, err := p.PublishBatch(context.Background(), msgs)
	// PublishBatch returns nil error if we handle partials?
	// Implementation: returns res, nil.
	assert.NoError(t, err)
	assert.Equal(t, 1, res.Succeeded)
	assert.Equal(t, 1, res.Failed)
	assert.Len(t, res.Errors, 1)
	assert.Equal(t, 1, res.Errors[0].Index)
}

func TestPublishAsync_Success(t *testing.T) {
	done := make(chan struct{})
	mock := &mockKafkaWriter{
		writeFunc: func(ctx context.Context, msgs ...kafka.Message) error {
			close(done)
			return nil
		},
	}
	p := newTestProducer(mock)
	msg := newTestProducerMessage("test", "k", "v")
	p.PublishAsync(context.Background(), msg)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

func TestClose_Success(t *testing.T) {
	closed := false
	mock := &mockKafkaWriter{
		closeFunc: func() error {
			closed = true
			return nil
		},
	}
	p := newTestProducer(mock)
	err := p.Close()
	assert.NoError(t, err)
	assert.True(t, closed)
}
