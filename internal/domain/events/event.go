// Package events provides domain event infrastructure for DDD event-driven communication.
package events

import (
	"encoding/json"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Event is a domain event marker interface, aliased from common.DomainEvent.
type Event = common.DomainEvent

// EventType represents the type of a domain event.
type EventType = common.EventType

// EventEnvelope wraps a domain event with serialization and routing metadata
// for transport over messaging infrastructure (e.g., Kafka).
type EventEnvelope struct {
	ID          string            `json:"id"`
	Type        EventType         `json:"type"`
	AggregateID string            `json:"aggregate_id"`
	Version     int               `json:"version"`
	Timestamp   time.Time         `json:"timestamp"`
	Data        json.RawMessage   `json:"data"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NewEventEnvelope creates an EventEnvelope from a domain event.
func NewEventEnvelope(event Event) EventEnvelope {
	data, _ := json.Marshal(event)
	return EventEnvelope{
		ID:          event.EventID(),
		Type:        event.EventType(),
		AggregateID: event.AggregateID(),
		Version:     event.Version(),
		Timestamp:   event.OccurredAt(),
		Data:        data,
		Metadata:    make(map[string]string),
	}
}

// WithMetadata adds a key-value pair to the envelope's metadata.
// Returns a new envelope with the metadata added; the original is unchanged.
func (e EventEnvelope) WithMetadata(key, value string) EventEnvelope {
	// Copy the metadata map to avoid mutating shared state.
	meta := make(map[string]string, len(e.Metadata)+1)
	for k, v := range e.Metadata {
		meta[k] = v
	}
	meta[key] = value
	e.Metadata = meta
	return e
}
