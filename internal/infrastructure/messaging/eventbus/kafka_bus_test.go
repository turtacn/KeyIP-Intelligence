package eventbus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/events"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

var errPublishFailed = errors.New("publish failed")

// mockProducer implements a lightweight mock for kafka.Producer.
type mockProducer struct {
	mu          sync.Mutex
	published   []*common.ProducerMessage
	publishFunc func(ctx context.Context, msg *common.ProducerMessage) error
	closed      atomic.Bool
}

func (m *mockProducer) Publish(ctx context.Context, msg *common.ProducerMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, msg)
	if m.publishFunc != nil {
		return m.publishFunc(ctx, msg)
	}
	return nil
}

func (m *mockProducer) PublishBatch(ctx context.Context, msgs []*common.ProducerMessage) (*common.BatchPublishResult, error) {
	return nil, nil
}

func (m *mockProducer) PublishAsync(ctx context.Context, msg *common.ProducerMessage) {}

func (m *mockProducer) Close() error {
	m.closed.Store(true)
	return nil
}

func (m *mockProducer) GetPublished() []*common.ProducerMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*common.ProducerMessage, len(m.published))
	copy(result, m.published)
	return result
}

func TestKafkaEventBus_Publish(t *testing.T) {
	mp := &mockProducer{}
	logger := logging.NewNopLogger()
	bus := NewKafkaEventBus(mp, logger)
	ctx := context.Background()

	event := newTestEvent("agg-1")

	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	published := mp.GetPublished()
	if len(published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(published))
	}

	msg := published[0]
	if msg.Topic != "test.event" {
		t.Errorf("topic = %s, want test.event", msg.Topic)
	}
	if string(msg.Key) != "agg-1" {
		t.Errorf("key = %s, want agg-1", string(msg.Key))
	}
	if msg.Headers["event_type"] != "test.event" {
		t.Errorf("event_type header = %s, want test.event", msg.Headers["event_type"])
	}
	if msg.Headers["aggregate_id"] != "agg-1" {
		t.Errorf("aggregate_id header = %s, want agg-1", msg.Headers["aggregate_id"])
	}
	if len(msg.Value) == 0 {
		t.Error("message value should not be empty")
	}
}

func TestKafkaEventBus_PublishMultiple(t *testing.T) {
	mp := &mockProducer{}
	logger := logging.NewNopLogger()
	bus := NewKafkaEventBus(mp, logger)
	ctx := context.Background()

	events := []events.Event{
		newTestEvent("agg-1"),
		newTestEvent("agg-2"),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	published := mp.GetPublished()
	if len(published) != 2 {
		t.Fatalf("expected 2 published messages, got %d", len(published))
	}
}

func TestKafkaEventBus_WithTopicMapper(t *testing.T) {
	mp := &mockProducer{}
	logger := logging.NewNopLogger()
	bus := NewKafkaEventBus(mp, logger)

	// Custom topic mapper that prefixes topics
	bus.WithTopicMapper(func(eventType events.EventType) string {
		return "domain." + string(eventType)
	})

	ctx := context.Background()
	event := newTestEvent("agg-1")

	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	published := mp.GetPublished()
	if len(published) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(published))
	}
	if published[0].Topic != "domain.test.event" {
		t.Errorf("topic = %s, want domain.test.event", published[0].Topic)
	}
}

func TestKafkaEventBus_Subscribe(t *testing.T) {
	mp := &mockProducer{}
	logger := logging.NewNopLogger()
	bus := NewKafkaEventBus(mp, logger)
	ctx := context.Background()

	var handled atomic.Int32
	unsub, err := bus.Subscribe(events.HandlerFunc{
		Type: "test.event",
		Fn: func(ctx context.Context, event events.Event) error {
			handled.Add(1)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	event := newTestEvent("agg-1")
	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Should have been published to Kafka AND handled locally
	published := mp.GetPublished()
	if len(published) != 1 {
		t.Errorf("expected 1 kafka message, got %d", len(published))
	}
	if handled.Load() != 1 {
		t.Errorf("local handler called %d times, want 1", handled.Load())
	}

	unsub()

	// After unsubscribe, local handler should not fire (but kafka publish still happens)
	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if handled.Load() != 1 {
		t.Errorf("local handler was called %d times after unsub, want 1", handled.Load())
	}
	if len(mp.GetPublished()) != 2 {
		t.Errorf("expected 2 kafka messages total, got %d", len(mp.GetPublished()))
	}
}

func TestKafkaEventBus_PublishError(t *testing.T) {
	mp := &mockProducer{
		publishFunc: func(ctx context.Context, msg *common.ProducerMessage) error {
			return errPublishFailed
		},
	}
	logger := logging.NewNopLogger()
	bus := NewKafkaEventBus(mp, logger)
	ctx := context.Background()

	event := newTestEvent("agg-1")
	if err := bus.Publish(ctx, event); err == nil {
		t.Error("expected publish error, got nil")
	}
}

func TestKafkaEventBus_Close(t *testing.T) {
	mp := &mockProducer{}
	logger := logging.NewNopLogger()
	bus := NewKafkaEventBus(mp, logger)

	if err := bus.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if !mp.closed.Load() {
		t.Error("producer was not closed")
	}
}
