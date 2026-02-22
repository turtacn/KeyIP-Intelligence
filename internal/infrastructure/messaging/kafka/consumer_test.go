package kafka

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockLogger (reused)
type MockLogger struct {
	logging.Logger
}
func (m *MockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *MockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logging.Field) {}
func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *MockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *MockLogger) WithError(err error) logging.Logger { return m }
func (m *MockLogger) Sync() error { return nil }
func newMockLogger() logging.Logger { return &MockLogger{} }

// mockKafkaReader
type mockKafkaReader struct {
	fetchFunc  func(ctx context.Context) (kafka.Message, error)
	commitFunc func(ctx context.Context, msgs ...kafka.Message) error
	closeFunc  func() error
}

func (m *mockKafkaReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(ctx)
	}
	// Block forever
	<-ctx.Done()
	return kafka.Message{}, ctx.Err()
}

func (m *mockKafkaReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	if m.commitFunc != nil {
		return m.commitFunc(ctx, msgs...)
	}
	return nil
}

func (m *mockKafkaReader) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockKafkaReader) Stats() kafka.ReaderStats {
	return kafka.ReaderStats{}
}

func newTestConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		Brokers: []string{"localhost:9092"},
		GroupID: "test-group",
		Topics:  []string{"test-topic"},
	}
}

func TestValidateConsumerConfig_Valid(t *testing.T) {
	cfg := newTestConsumerConfig()
	err := ValidateConsumerConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateConsumerConfig_EmptyBrokers(t *testing.T) {
	cfg := newTestConsumerConfig()
	cfg.Brokers = nil
	err := ValidateConsumerConfig(cfg)
	assert.Error(t, err)
}

func TestSubscribe_Success(t *testing.T) {
	c := &Consumer{
		handlers: make(map[string]MessageHandler),
		logger:   newMockLogger(),
	}
	c.Subscribe("topic", func(ctx context.Context, msg *Message) error { return nil })
	assert.Len(t, c.handlers, 1)
}

func TestStart_AlreadyRunning(t *testing.T) {
	c := &Consumer{
		handlers: make(map[string]MessageHandler),
		logger:   newMockLogger(),
	}
	// Manually set running
	c.running.Store(true)
	err := c.Start(context.Background())
	assert.Equal(t, ErrAlreadyRunning, err)
}

func TestConsumeLoop_SingleMessage(t *testing.T) {
	msgProcessed := false
	mockReader := &mockKafkaReader{
		fetchFunc: func(ctx context.Context) (kafka.Message, error) {
			if msgProcessed {
				<-ctx.Done()
				return kafka.Message{}, ctx.Err()
			}
			msgProcessed = true // only fetch once
			return kafka.Message{
				Topic: "test-topic",
				Value: []byte("value"),
			}, nil
		},
		commitFunc: func(ctx context.Context, msgs ...kafka.Message) error {
			assert.Len(t, msgs, 1)
			return nil
		},
	}

	c := &Consumer{
		reader:   mockReader,
		config:   newTestConsumerConfig(),
		logger:   newMockLogger(),
		handlers: make(map[string]MessageHandler),
		metrics:  &ConsumerMetrics{},
	}

	handlerCalled := make(chan struct{})
	c.Subscribe("test-topic", func(ctx context.Context, msg *Message) error {
		assert.Equal(t, "value", string(msg.Value))
		close(handlerCalled)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	c.Start(ctx) // Starts goroutine

	select {
	case <-handlerCalled:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for handler")
	}

	c.Close()
	cancel()
}

func TestProcessMessage_RetrySuccess(t *testing.T) {
	c := &Consumer{
		config: ConsumerConfig{
			RetryConfig: RetryConfig{
				MaxRetries:   2,
				RetryBackoff: 1 * time.Millisecond,
			},
		},
		metrics: &ConsumerMetrics{},
		logger:  newMockLogger(),
	}

	attempts := 0
	handler := func(ctx context.Context, msg *Message) error {
		attempts++
		if attempts < 2 {
			return errors.New("fail")
		}
		return nil
	}

	err := c.processMessage(context.Background(), &Message{}, handler)
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
	assert.Equal(t, int64(1), c.metrics.MessagesRetried.Load())
}

func TestProcessMessage_RetryExhausted(t *testing.T) {
	c := &Consumer{
		config: ConsumerConfig{
			RetryConfig: RetryConfig{
				MaxRetries:   1,
				RetryBackoff: 1 * time.Millisecond,
			},
		},
		metrics: &ConsumerMetrics{},
		logger:  newMockLogger(),
	}

	handler := func(ctx context.Context, msg *Message) error {
		return errors.New("fail")
	}

	err := c.processMessage(context.Background(), &Message{}, handler)
	// Should return nil (handled/dropped)
	assert.NoError(t, err)
	// But logged error
}

//Personal.AI order the ending
