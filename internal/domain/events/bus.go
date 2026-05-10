package events

import "context"

// Handler handles domain events of a specific type.
type Handler interface {
	// HandleEvent processes a single domain event.
	HandleEvent(ctx context.Context, event Event) error
	// HandlesType returns the event type this handler is registered for.
	HandlesType() EventType
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc struct {
	Type EventType
	Fn   func(ctx context.Context, event Event) error
}

// HandleEvent implements Handler.
func (h HandlerFunc) HandleEvent(ctx context.Context, event Event) error {
	return h.Fn(ctx, event)
}

// HandlesType implements Handler.
func (h HandlerFunc) HandlesType() EventType {
	return h.Type
}

// EventBus defines the interface for publishing and subscribing to domain events.
type EventBus interface {
	// Publish publishes one or more domain events to all registered handlers.
	Publish(ctx context.Context, events ...Event) error

	// Subscribe registers a handler for a specific event type.
	// Returns an unsubscribe function that removes the handler when called.
	Subscribe(handler Handler) (func(), error)
}
