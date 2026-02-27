package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRemainingLifeYears(t *testing.T) {
	// Past expiry
	past := time.Now().AddDate(-1, 0, 0)
	assert.Equal(t, 0.0, RemainingLifeYears(&past))

	// Nil expiry
	assert.Equal(t, 0.0, RemainingLifeYears(nil))

	// Future expiry (approx 1 year)
	future := time.Now().AddDate(1, 0, 0)
	years := RemainingLifeYears(&future)
	assert.Greater(t, years, 0.99)
	assert.Less(t, years, 1.01)
}

func TestEventType_Constants(t *testing.T) {
	assert.Equal(t, EventType("filing"), EventTypeFiling)
	assert.Equal(t, EventType("grant"), EventTypeGrant)
}

//Personal.AI order the ending
