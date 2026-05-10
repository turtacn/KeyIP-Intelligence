package eventbus

import (
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/events"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// testEvent is a simple domain event for testing.
type testEvent struct {
	common.BaseEvent
	Data string `json:"data"`
}

func newTestEvent(aggID string) events.Event {
	return testEvent{
		BaseEvent: common.NewBaseEventWithVersion("test.event", aggID, 1),
		Data:      "test-data",
	}
}

func init() {
	// Ensure timestamps are set for deterministic tests
	_ = time.Now()
}
