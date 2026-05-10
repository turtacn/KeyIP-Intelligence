package eventbus

import (
	"context"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/events"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// LoggingBus is a decorator that wraps an events.EventBus and logs
// all published events and subscriptions for observability.
type LoggingBus struct {
	inner  events.EventBus
	logger logging.Logger
}

// NewLoggingBus creates a new LoggingBus wrapping the given EventBus.
func NewLoggingBus(inner events.EventBus, logger logging.Logger) *LoggingBus {
	return &LoggingBus{
		inner:  inner,
		logger: logger,
	}
}

// Publish logs each event and delegates to the wrapped bus.
func (b *LoggingBus) Publish(ctx context.Context, domainEvents ...events.Event) error {
	for _, event := range domainEvents {
		b.logger.Info("Domain event published",
			logging.String("event_type", string(event.EventType())),
			logging.String("event_id", event.EventID()),
			logging.String("aggregate_id", event.AggregateID()),
			logging.Int("version", event.Version()),
		)
	}

	return b.inner.Publish(ctx, domainEvents...)
}

// Subscribe logs the subscription and delegates to the wrapped bus.
func (b *LoggingBus) Subscribe(handler events.Handler) (func(), error) {
	b.logger.Info("Event handler subscribed",
		logging.String("event_type", string(handler.HandlesType())),
	)

	unsubscribe, err := b.inner.Subscribe(handler)
	if err != nil {
		return nil, err
	}

	return func() {
		b.logger.Info("Event handler unsubscribed",
			logging.String("event_type", string(handler.HandlesType())),
		)
		unsubscribe()
	}, nil
}

// compile-time check
var _ events.EventBus = (*LoggingBus)(nil)
