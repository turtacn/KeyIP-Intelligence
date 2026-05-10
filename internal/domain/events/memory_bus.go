package events

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

var subIDCounter atomic.Int64

// subscription represents a registered handler with a unique ID for unsubscription.
type subscription struct {
	id      int64
	handler Handler
}

// InMemoryBus is a thread-safe, in-memory implementation of EventBus.
// It is suitable for testing and single-process deployments where events
// do not need to cross process boundaries.
type InMemoryBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]subscription
}

// NewInMemoryBus creates a new InMemoryBus.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		handlers: make(map[EventType][]subscription),
	}
}

// Publish delivers events to all handlers registered for the event's type.
// Handlers are invoked synchronously in registration order. If any handler
// returns an error, the remaining handlers for that event type are still
// called, and all errors are collected and returned.
func (b *InMemoryBus) Publish(ctx context.Context, events ...Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, event := range events {
		eventType := event.EventType()
		subs := b.handlers[eventType]

		var firstErr error
		for _, sub := range subs {
			if err := sub.handler.HandleEvent(ctx, event); err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("handler failed for %s: %w", eventType, err)
				}
			}
		}
		if firstErr != nil {
			return firstErr
		}
	}
	return nil
}

// Subscribe registers a handler for a specific event type. The returned
// unsubscribe function, when called, removes the handler from the bus.
func (b *InMemoryBus) Subscribe(handler Handler) (func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := subIDCounter.Add(1)
	sub := subscription{id: id, handler: handler}
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

// compile-time check
var _ EventBus = (*InMemoryBus)(nil)
var _ common.DomainEvent = common.BaseEvent{}
