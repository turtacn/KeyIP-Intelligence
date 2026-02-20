package patent

import (
	"time"

	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// DomainEvent interface
// ─────────────────────────────────────────────────────────────────────────────

// DomainEvent is the marker interface implemented by every patent domain event.
// Events are immutable value objects that record something significant that
// happened within the patent aggregate.
//
// Events are:
//   - created inside aggregate methods when state changes occur
//   - collected in the aggregate's uncommitted event log
//   - published to Kafka by the infrastructure layer after a successful commit
//   - consumed by other bounded contexts (lifecycle, infringement, analytics)
type DomainEvent interface {
	// EventName returns the unique, stable name used as the Kafka topic key and
	// for event-store discrimination, e.g., "patent.created".
	EventName() string

	// OccurredAt returns the UTC wall-clock time at which the event was raised.
	OccurredAt() time.Time

	// AggregateID returns the ID of the Patent aggregate that raised the event.
	AggregateID() common.ID
}

// ─────────────────────────────────────────────────────────────────────────────
// baseEvent — shared implementation of DomainEvent
// ─────────────────────────────────────────────────────────────────────────────

// baseEvent provides the common DomainEvent fields and satisfies the interface.
// All concrete event structs embed baseEvent.
type baseEvent struct {
	name        string
	occurredAt  time.Time
	aggregateID common.ID
}

// EventName implements DomainEvent.
func (e *baseEvent) EventName() string { return e.name }

// OccurredAt implements DomainEvent.
func (e *baseEvent) OccurredAt() time.Time { return e.occurredAt }

// AggregateID implements DomainEvent.
func (e *baseEvent) AggregateID() common.ID { return e.aggregateID }

// newBaseEvent is the internal constructor for baseEvent.
func newBaseEvent(name string, aggregateID common.ID) baseEvent {
	return baseEvent{
		name:        name,
		occurredAt:  time.Now().UTC(),
		aggregateID: aggregateID,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PatentCreated event
// ─────────────────────────────────────────────────────────────────────────────

// PatentCreatedEventName is the stable topic/type identifier for PatentCreated.
const PatentCreatedEventName = "patent.created"

// PatentCreated is raised when a new patent aggregate is successfully persisted
// for the first time.  Consumers (e.g., OpenSearch indexer, analytics pipeline)
// use this event to initialise their own representations of the patent.
type PatentCreated struct {
	baseEvent

	// PatentNumber is the official publication number (e.g., "CN202310001234A").
	PatentNumber string

	// Title is the patent title as filed.
	Title string

	// Jurisdiction is the issuing authority code (CN, US, EP, …).
	Jurisdiction ptypes.JurisdictionCode
}

// NewPatentCreatedEvent constructs a PatentCreated event for the given aggregate.
func NewPatentCreatedEvent(
	aggregateID common.ID,
	patentNumber string,
	title string,
	jurisdiction ptypes.JurisdictionCode,
) *PatentCreated {
	return &PatentCreated{
		baseEvent:    newBaseEvent(PatentCreatedEventName, aggregateID),
		PatentNumber: patentNumber,
		Title:        title,
		Jurisdiction: jurisdiction,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PatentStatusChanged event
// ─────────────────────────────────────────────────────────────────────────────

// PatentStatusChangedEventName is the stable topic/type identifier for
// PatentStatusChanged.
const PatentStatusChangedEventName = "patent.status_changed"

// PatentStatusChanged is raised whenever the lifecycle status of a patent
// transitions from one state to another (e.g., Pending → Granted,
// Granted → Expired).  The lifecycle-management bounded context subscribes to
// this event to trigger notifications and maintenance workflows.
type PatentStatusChanged struct {
	baseEvent

	// PatentNumber is the official publication number.
	PatentNumber string

	// OldStatus is the status before the transition.
	OldStatus ptypes.PatentStatus

	// NewStatus is the status after the transition.
	NewStatus ptypes.PatentStatus
}

// NewPatentStatusChangedEvent constructs a PatentStatusChanged event.
func NewPatentStatusChangedEvent(
	aggregateID common.ID,
	patentNumber string,
	oldStatus ptypes.PatentStatus,
	newStatus ptypes.PatentStatus,
) *PatentStatusChanged {
	return &PatentStatusChanged{
		baseEvent:    newBaseEvent(PatentStatusChangedEventName, aggregateID),
		PatentNumber: patentNumber,
		OldStatus:    oldStatus,
		NewStatus:    newStatus,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ClaimAdded event
// ─────────────────────────────────────────────────────────────────────────────

// ClaimAddedEventName is the stable topic/type identifier for ClaimAdded.
const ClaimAddedEventName = "patent.claim_added"

// ClaimAdded is raised when a new Claim value object is appended to an existing
// patent aggregate.  The ClaimBERT service subscribes to this event to schedule
// NLP parsing of the new claim text.
type ClaimAdded struct {
	baseEvent

	// ClaimNumber is the sequential number of the newly added claim.
	ClaimNumber int

	// ClaimType identifies whether the claim is independent or dependent.
	ClaimType ptypes.ClaimType
}

// NewClaimAdded constructs a ClaimAdded event.
func NewClaimAdded(
	aggregateID common.ID,
	claimNumber int,
	claimType ptypes.ClaimType,
) *ClaimAdded {
	return &ClaimAdded{
		baseEvent:   newBaseEvent(ClaimAddedEventName, aggregateID),
		ClaimNumber: claimNumber,
		ClaimType:   claimType,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MarkushAdded event
// ─────────────────────────────────────────────────────────────────────────────

// MarkushAddedEventName is the stable topic/type identifier for MarkushAdded.
const MarkushAddedEventName = "patent.markush_added"

// MarkushAdded is raised when a Markush structure is extracted and associated
// with a patent aggregate.  The MolPatentGNN service subscribes to this event
// to enqueue Markush enumeration and vector embedding jobs.
type MarkushAdded struct {
	baseEvent

	// MarkushID is the platform-internal identifier of the newly added
	// Markush structure.
	MarkushID common.ID

	// EnumeratedCount is the pre-computed cardinality of the Markush virtual
	// library (product of all R-group alternative counts).
	EnumeratedCount int64
}

// NewMarkushAdded constructs a MarkushAdded event.
func NewMarkushAdded(
	aggregateID common.ID,
	markushID common.ID,
	enumeratedCount int64,
) *MarkushAdded {
	return &MarkushAdded{
		baseEvent:       newBaseEvent(MarkushAddedEventName, aggregateID),
		MarkushID:       markushID,
		EnumeratedCount: enumeratedCount,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PatentExpiring event
// ─────────────────────────────────────────────────────────────────────────────

// PatentExpiringEventName is the stable topic/type identifier for PatentExpiring.
const PatentExpiringEventName = "patent.expiring"

// PatentExpiring is raised by the lifecycle-management service when a patent
// is approaching its expiry date.  Portfolio managers and FTO analysts
// subscribe to this event to review and update their freedom-to-operate
// assessments before the patent lapses.
type PatentExpiring struct {
	baseEvent

	// ExpiryDate is the exact date on which the patent will expire.
	ExpiryDate time.Time

	// DaysRemaining is the number of calendar days until expiry at the time
	// the event was raised.
	DaysRemaining int
}

// NewPatentExpiring constructs a PatentExpiring event.
func NewPatentExpiring(
	aggregateID common.ID,
	expiryDate time.Time,
	daysRemaining int,
) *PatentExpiring {
	return &PatentExpiring{
		baseEvent:     newBaseEvent(PatentExpiringEventName, aggregateID),
		ExpiryDate:    expiryDate,
		DaysRemaining: daysRemaining,
	}
}

