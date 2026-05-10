package events

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// testEvent is a simple domain event for testing.
type testEvent struct {
	common.BaseEvent
	Data string `json:"data"`
}

func newTestEvent(aggID string) testEvent {
	return testEvent{
		BaseEvent: common.NewBaseEventWithVersion("test.event", aggID, 1),
		Data:      "test-data",
	}
}

func TestInMemoryBus_PublishSubscribe(t *testing.T) {
	bus := NewInMemoryBus()
	ctx := context.Background()

	var handled atomic.Int32
	handler := HandlerFunc{
		Type: "test.event",
		Fn: func(ctx context.Context, event Event) error {
			handled.Add(1)
			return nil
		},
	}

	unsub, err := bus.Subscribe(handler)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	event := newTestEvent("agg-1")
	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if handled.Load() != 1 {
		t.Errorf("handler was called %d times, want 1", handled.Load())
	}

	// Unsubscribe and verify handler is not called again
	unsub()
	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if handled.Load() != 1 {
		t.Errorf("handler was called %d times after unsubscribe, want 1", handled.Load())
	}
}

func TestInMemoryBus_MultipleHandlers(t *testing.T) {
	bus := NewInMemoryBus()
	ctx := context.Background()

	var count1, count2 atomic.Int32

	h1 := HandlerFunc{Type: "test.event", Fn: func(ctx context.Context, event Event) error {
		count1.Add(1)
		return nil
	}}
	h2 := HandlerFunc{Type: "test.event", Fn: func(ctx context.Context, event Event) error {
		count2.Add(1)
		return nil
	}}

	_, _ = bus.Subscribe(h1)
	_, _ = bus.Subscribe(h2)

	event := newTestEvent("agg-1")
	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if count1.Load() != 1 || count2.Load() != 1 {
		t.Errorf("handler counts: h1=%d, h2=%d, want both 1", count1.Load(), count2.Load())
	}
}

func TestInMemoryBus_TypeFiltering(t *testing.T) {
	bus := NewInMemoryBus()
	ctx := context.Background()

	var typeA, typeB atomic.Int32

	hA := HandlerFunc{Type: "type.a", Fn: func(ctx context.Context, event Event) error {
		typeA.Add(1)
		return nil
	}}
	hB := HandlerFunc{Type: "type.b", Fn: func(ctx context.Context, event Event) error {
		typeB.Add(1)
		return nil
	}}

	_, _ = bus.Subscribe(hA)
	_, _ = bus.Subscribe(hB)

	// Publish type.a event - only handler A should fire
	evtA := testEvent{}
	evtA.Type = "type.a"
	evtA.Timestamp = time.Now().UTC()
	if err := bus.Publish(ctx, evtA); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if typeA.Load() != 1 {
		t.Errorf("type.a handler called %d times, want 1", typeA.Load())
	}
	if typeB.Load() != 0 {
		t.Errorf("type.b handler called %d times, want 0", typeB.Load())
	}
}

func TestInMemoryBus_NilEventBus(t *testing.T) {
	// Verify nil event bus is handled gracefully by calling
	// Publish without subscribing. Should simply do nothing.
	bus := NewInMemoryBus()
	ctx := context.Background()

	event := newTestEvent("agg-1")
	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish with no handlers failed: %v", err)
	}
}

func TestInMemoryBus_MultipleEvents(t *testing.T) {
	bus := NewInMemoryBus()
	ctx := context.Background()

	var handled atomic.Int32
	handler := HandlerFunc{
		Type: "test.event",
		Fn: func(ctx context.Context, event Event) error {
			handled.Add(1)
			return nil
		},
	}

	_, _ = bus.Subscribe(handler)

	events := []Event{
		newTestEvent("agg-1"),
		newTestEvent("agg-2"),
		newTestEvent("agg-3"),
	}

	if err := bus.Publish(ctx, events...); err != nil {
		t.Fatalf("Publish multiple events failed: %v", err)
	}

	if handled.Load() != 3 {
		t.Errorf("handler was called %d times, want 3", handled.Load())
	}
}

func TestNewEventEnvelope(t *testing.T) {
	event := newTestEvent("agg-1")
	event.Timestamp = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	env := NewEventEnvelope(event)

	if env.ID != event.EventID() {
		t.Errorf("envelope ID = %s, want %s", env.ID, event.EventID())
	}
	if env.Type != event.EventType() {
		t.Errorf("envelope Type = %s, want %s", env.Type, event.EventType())
	}
	if env.AggregateID != event.AggregateID() {
		t.Errorf("envelope AggregateID = %s, want %s", env.AggregateID, event.AggregateID())
	}
	if env.Timestamp != event.OccurredAt() {
		t.Errorf("envelope Timestamp = %v, want %v", env.Timestamp, event.OccurredAt())
	}
	if len(env.Data) == 0 {
		t.Error("envelope Data should not be empty")
	}
}

func TestEventEnvelope_WithMetadata(t *testing.T) {
	event := newTestEvent("agg-1")
	env := NewEventEnvelope(event)

	env2 := env.WithMetadata("key1", "value1").WithMetadata("key2", "value2")

	if env2.Metadata["key1"] != "value1" {
		t.Errorf("metadata key1 = %s, want value1", env2.Metadata["key1"])
	}
	if env2.Metadata["key2"] != "value2" {
		t.Errorf("metadata key2 = %s, want value2", env2.Metadata["key2"])
	}

	// Original envelope should not be modified (should still be empty)
	if len(env.Metadata) != 0 {
		t.Errorf("original envelope metadata length = %d, want 0", len(env.Metadata))
	}
}
