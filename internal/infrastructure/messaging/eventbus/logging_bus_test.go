package eventbus

import (
	"context"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/events"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func TestLoggingBus_Publish(t *testing.T) {
	inner := events.NewInMemoryBus()
	logger := logging.NewNopLogger()
	bus := NewLoggingBus(inner, logger)

	ctx := context.Background()
	event := newTestEvent("agg-1")

	var handled bool
	_, _ = inner.Subscribe(events.HandlerFunc{
		Type: "test.event",
		Fn: func(ctx context.Context, event events.Event) error {
			handled = true
			return nil
		},
	})

	if err := bus.Publish(ctx, event); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if !handled {
		t.Error("inner handler was not called")
	}
}

func TestLoggingBus_Subscribe(t *testing.T) {
	inner := events.NewInMemoryBus()
	logger := logging.NewNopLogger()
	bus := NewLoggingBus(inner, logger)

	unsub, err := bus.Subscribe(events.HandlerFunc{
		Type: "test.event",
		Fn: func(ctx context.Context, event events.Event) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if unsub == nil {
		t.Fatal("unsubscribe function should not be nil")
	}

	// Unsubscribe should work without error
	unsub()
}

func TestLoggingBus_NilInner(t *testing.T) {
	logger := logging.NewNopLogger()
	bus := NewLoggingBus(nil, logger)

	ctx := context.Background()
	event := newTestEvent("agg-1")

	// Should panic with nil pointer access
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil inner bus")
		}
	}()

	_ = bus.Publish(ctx, event)
}
