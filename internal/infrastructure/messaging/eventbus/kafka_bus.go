// Package eventbus provides infrastructure implementations of the domain EventBus.
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/events"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MessagePublisher defines the interface for publishing messages to a message broker.
// This abstraction allows testing without a real Kafka cluster.
type MessagePublisher interface {
	Publish(ctx context.Context, msg *common.ProducerMessage) error
	Close() error
}

// topicMapper maps event types to Kafka topics.
type topicMapper func(eventType events.EventType) string

// defaultTopicMapper maps event types to topics by using the event type string
// as the topic name (e.g., "molecule.registered" -> topic "molecule.registered").
func defaultTopicMapper(eventType events.EventType) string {
	return string(eventType)
}

var kafkaSubIDCounter atomic.Int64

// kafkaSubscription wraps a handler with a unique ID for unsubscription.
type kafkaSubscription struct {
	id      int64
	handler events.Handler
}

// KafkaEventBus is a Kafka-backed implementation of events.EventBus.
// It publishes domain events as JSON messages to Kafka topics.
// Note: Subscribe is not supported directly through Kafka; handlers
// should use the kafka.Consumer directly for consumption.
type KafkaEventBus struct {
	producer    MessagePublisher
	topicMapper topicMapper
	logger      logging.Logger

	mu       sync.RWMutex
	handlers map[events.EventType][]kafkaSubscription
}

// NewKafkaEventBus creates a new KafkaEventBus backed by the given MessagePublisher.
// Events are serialized to JSON and published to Kafka topics determined
// by the topicMapper function.
func NewKafkaEventBus(producer MessagePublisher, logger logging.Logger) *KafkaEventBus {
	return &KafkaEventBus{
		producer:    producer,
		topicMapper: defaultTopicMapper,
		logger:      logger,
		handlers:    make(map[events.EventType][]kafkaSubscription),
	}
}

// WithTopicMapper sets a custom topic mapper function.
func (b *KafkaEventBus) WithTopicMapper(mapper topicMapper) *KafkaEventBus {
	b.topicMapper = mapper
	return b
}

// Publish serializes each domain event into a JSON EventEnvelope and publishes
// it to the Kafka topic determined by the topic mapper.
func (b *KafkaEventBus) Publish(ctx context.Context, domainEvents ...events.Event) error {
	for _, event := range domainEvents {
		envelope := events.NewEventEnvelope(event)
		payload, err := json.Marshal(envelope)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeInternal, "failed to serialize event envelope")
		}

		topic := b.topicMapper(event.EventType())
		key := []byte(event.AggregateID())

		msg := &common.ProducerMessage{
			Topic: topic,
			Key:   key,
			Value: payload,
			Headers: map[string]string{
				"event_type":   string(event.EventType()),
				"event_id":     event.EventID(),
				"aggregate_id": event.AggregateID(),
				"content_type": "application/x-domain-event",
			},
		}

		if err := b.producer.Publish(ctx, msg); err != nil {
			return errors.Wrap(err, errors.ErrCodeInternal,
				fmt.Sprintf("failed to publish event %s to topic %s", event.EventType(), topic))
		}

		b.logger.Debug("Event published to Kafka",
			logging.String("event_type", string(event.EventType())),
			logging.String("topic", topic),
			logging.String("aggregate_id", event.AggregateID()))
	}

	// Also deliver to local in-process handlers
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, event := range domainEvents {
		subs := b.handlers[event.EventType()]
		for _, sub := range subs {
			if err := sub.handler.HandleEvent(ctx, event); err != nil {
				b.logger.Error("Local handler failed for event",
					logging.String("event_type", string(event.EventType())),
					logging.Err(err))
			}
		}
	}

	return nil
}

// Subscribe registers an in-process handler. This is primarily for local
// side-effect handling (e.g., updating local caches). For distributed
// consumption, use kafka.Consumer directly with the appropriate topic.
func (b *KafkaEventBus) Subscribe(handler events.Handler) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := kafkaSubIDCounter.Add(1)
	sub := kafkaSubscription{id: id, handler: handler}
	eventType := handler.HandlesType()
	b.handlers[eventType] = append(b.handlers[eventType], sub)

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.handlers[eventType]
		for i, s := range subs {
			if s.id == id {
				b.handlers[eventType] = append(subs[:i], subs[i+1:]...)
				return
			}
		}
	}, nil
}

// Close closes the underlying message publisher.
func (b *KafkaEventBus) Close() error {
	return b.producer.Close()
}

// compile-time check
var _ events.EventBus = (*KafkaEventBus)(nil)
