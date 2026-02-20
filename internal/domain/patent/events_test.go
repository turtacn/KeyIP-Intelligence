package patent_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func newAggregateID() common.ID { return common.NewID() }

// assertBaseEvent verifies that any DomainEvent implementation carries a
// non-empty name, a recent timestamp, and the correct aggregate ID.
func assertBaseEvent(t *testing.T, evt patent.DomainEvent, wantName string, wantAggID common.ID) {
	t.Helper()
	assert.Equal(t, wantName, evt.EventName(), "EventName mismatch")
	assert.Equal(t, wantAggID, evt.AggregateID(), "AggregateID mismatch")
	assert.False(t, evt.OccurredAt().IsZero(), "OccurredAt must not be zero")
	assert.WithinDuration(t, time.Now().UTC(), evt.OccurredAt(), 5*time.Second,
		"OccurredAt must be recent (within 5 s)")
}

// ─────────────────────────────────────────────────────────────────────────────
// DomainEvent interface satisfaction
// ─────────────────────────────────────────────────────────────────────────────

// TestDomainEvent_InterfaceSatisfaction verifies that each concrete event type
// satisfies the DomainEvent interface at compile time via assignment.
func TestDomainEvent_InterfaceSatisfaction(t *testing.T) {
	t.Parallel()

	aggID := newAggregateID()
	expiry := time.Now().Add(30 * 24 * time.Hour)

	var _ patent.DomainEvent = patent.NewPatentCreated(aggID, "CN000001A", "Title", ptypes.JurisdictionCN)
	var _ patent.DomainEvent = patent.NewPatentStatusChanged(aggID, ptypes.PatentStatusPending, ptypes.PatentStatusGranted)
	var _ patent.DomainEvent = patent.NewClaimAdded(aggID, 1, ptypes.ClaimTypeIndependent)
	var _ patent.DomainEvent = patent.NewMarkushAdded(aggID, common.NewID(), 42)
	var _ patent.DomainEvent = patent.NewPatentExpiring(aggID, expiry, 30)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewPatentCreated
// ─────────────────────────────────────────────────────────────────────────────

func TestNewPatentCreated_Fields(t *testing.T) {
	t.Parallel()

	aggID := newAggregateID()
	evt := patent.NewPatentCreated(aggID, "CN202310001234A", "Organic LED Material", ptypes.JurisdictionCN)

	require.NotNil(t, evt)
	assertBaseEvent(t, evt, patent.PatentCreatedEventName, aggID)
	assert.Equal(t, "CN202310001234A", evt.PatentNumber)
	assert.Equal(t, "Organic LED Material", evt.Title)
	assert.Equal(t, ptypes.JurisdictionCN, evt.Jurisdiction)
}

func TestNewPatentCreated_EventName(t *testing.T) {
	t.Parallel()

	evt := patent.NewPatentCreated(newAggregateID(), "US10000001B2", "T", ptypes.JurisdictionUS)
	assert.Equal(t, "patent.created", evt.EventName())
}

func TestNewPatentCreated_OccurredAtIsUTC(t *testing.T) {
	t.Parallel()

	evt := patent.NewPatentCreated(newAggregateID(), "EP3000001A1", "T", ptypes.JurisdictionEP)
	assert.Equal(t, time.UTC, evt.OccurredAt().Location())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewPatentStatusChanged
// ─────────────────────────────────────────────────────────────────────────────

func TestNewPatentStatusChanged_Fields(t *testing.T) {
	t.Parallel()

	aggID := newAggregateID()
	evt := patent.NewPatentStatusChanged(aggID, ptypes.PatentStatusPending, ptypes.PatentStatusGranted)

	require.NotNil(t, evt)
	assertBaseEvent(t, evt, patent.PatentStatusChangedEventName, aggID)
	assert.Equal(t, ptypes.PatentStatusPending, evt.OldStatus)
	assert.Equal(t, ptypes.PatentStatusGranted, evt.NewStatus)
}

func TestNewPatentStatusChanged_EventName(t *testing.T) {
	t.Parallel()

	evt := patent.NewPatentStatusChanged(newAggregateID(), ptypes.PatentStatusGranted, ptypes.PatentStatusExpired)
	assert.Equal(t, "patent.status_changed", evt.EventName())
}

func TestNewPatentStatusChanged_DifferentTransitions(t *testing.T) {
	t.Parallel()

	transitions := []struct {
		from ptypes.PatentStatus
		to   ptypes.PatentStatus
	}{
		{ptypes.PatentStatusPending, ptypes.PatentStatusGranted},
		{ptypes.PatentStatusGranted, ptypes.PatentStatusExpired},
		{ptypes.PatentStatusGranted, ptypes.PatentStatusWithdrawn},
	}

	for _, tr := range transitions {
		tr := tr
		t.Run(string(tr.from)+"->"+string(tr.to), func(t *testing.T) {
			t.Parallel()
			evt := patent.NewPatentStatusChanged(newAggregateID(), tr.from, tr.to)
			assert.Equal(t, tr.from, evt.OldStatus)
			assert.Equal(t, tr.to, evt.NewStatus)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewClaimAdded
// ─────────────────────────────────────────────────────────────────────────────

func TestNewClaimAdded_Fields(t *testing.T) {
	t.Parallel()

	aggID := newAggregateID()
	evt := patent.NewClaimAdded(aggID, 3, ptypes.ClaimTypeDependent)

	require.NotNil(t, evt)
	assertBaseEvent(t, evt, patent.ClaimAddedEventName, aggID)
	assert.Equal(t, 3, evt.ClaimNumber)
	assert.Equal(t, ptypes.ClaimTypeDependent, evt.ClaimType)
}

func TestNewClaimAdded_EventName(t *testing.T) {
	t.Parallel()

	evt := patent.NewClaimAdded(newAggregateID(), 1, ptypes.ClaimTypeIndependent)
	assert.Equal(t, "patent.claim_added", evt.EventName())
}

func TestNewClaimAdded_IndependentClaim(t *testing.T) {
	t.Parallel()

	aggID := newAggregateID()
	evt := patent.NewClaimAdded(aggID, 1, ptypes.ClaimTypeIndependent)
	assert.Equal(t, ptypes.ClaimTypeIndependent, evt.ClaimType)
	assert.Equal(t, 1, evt.ClaimNumber)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewMarkushAdded
// ─────────────────────────────────────────────────────────────────────────────

func TestNewMarkushAdded_Fields(t *testing.T) {
	t.Parallel()

	aggID := newAggregateID()
	markushID := common.NewID()
	var count int64 = 1_000_000

	evt := patent.NewMarkushAdded(aggID, markushID, count)

	require.NotNil(t, evt)
	assertBaseEvent(t, evt, patent.MarkushAddedEventName, aggID)
	assert.Equal(t, markushID, evt.MarkushID)
	assert.Equal(t, count, evt.EnumeratedCount)
}

func TestNewMarkushAdded_EventName(t *testing.T) {
	t.Parallel()

	evt := patent.NewMarkushAdded(newAggregateID(), common.NewID(), 42)
	assert.Equal(t, "patent.markush_added", evt.EventName())
}

func TestNewMarkushAdded_ZeroCount(t *testing.T) {
	t.Parallel()

	// Zero is a valid value at the event level; business invariants are
	// enforced in the Markush value object, not in the event.
	evt := patent.NewMarkushAdded(newAggregateID(), common.NewID(), 0)
	assert.Equal(t, int64(0), evt.EnumeratedCount)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewPatentExpiring
// ─────────────────────────────────────────────────────────────────────────────

func TestNewPatentExpiring_Fields(t *testing.T) {
	t.Parallel()

	aggID := newAggregateID()
	expiry := time.Now().Add(90 * 24 * time.Hour).UTC()

	evt := patent.NewPatentExpiring(aggID, expiry, 90)

	require.NotNil(t, evt)
	assertBaseEvent(t, evt, patent.PatentExpiringEventName, aggID)
	assert.Equal(t, expiry, evt.ExpiryDate)
	assert.Equal(t, 90, evt.DaysRemaining)
}

func TestNewPatentExpiring_EventName(t *testing.T) {
	t.Parallel()

	evt := patent.NewPatentExpiring(newAggregateID(), time.Now(), 30)
	assert.Equal(t, "patent.expiring", evt.EventName())
}

func TestNewPatentExpiring_ZeroDaysRemaining(t *testing.T) {
	t.Parallel()

	// DaysRemaining == 0 means the patent expires today.
	evt := patent.NewPatentExpiring(newAggregateID(), time.Now().UTC(), 0)
	assert.Equal(t, 0, evt.DaysRemaining)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestEventName_Constants
// ─────────────────────────────────────────────────────────────────────────────

func TestEventNameConstants_AreDistinct(t *testing.T) {
	t.Parallel()

	names := []string{
		patent.PatentCreatedEventName,
		patent.PatentStatusChangedEventName,
		patent.ClaimAddedEventName,
		patent.MarkushAddedEventName,
		patent.PatentExpiringEventName,
	}

	seen := make(map[string]bool)
	for _, n := range names {
		assert.False(t, seen[n], "duplicate event name constant: %q", n)
		seen[n] = true
	}
}

func TestEventNameConstants_Values(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "patent.created", patent.PatentCreatedEventName)
	assert.Equal(t, "patent.status_changed", patent.PatentStatusChangedEventName)
	assert.Equal(t, "patent.claim_added", patent.ClaimAddedEventName)
	assert.Equal(t, "patent.markush_added", patent.MarkushAddedEventName)
	assert.Equal(t, "patent.expiring", patent.PatentExpiringEventName)
}

//Personal.AI order the ending
