package patent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventType is a string identifier for patent domain events.
type EventType string

const (
	EventPatentCreated            EventType = "patent.created"
	EventPatentPublished          EventType = "patent.published"
	EventPatentExaminationStarted EventType = "patent.examination_started"
	EventPatentGranted            EventType = "patent.granted"
	EventPatentRejected           EventType = "patent.rejected"
	EventPatentWithdrawn          EventType = "patent.withdrawn"
	EventPatentExpired            EventType = "patent.expired"
	EventPatentInvalidated        EventType = "patent.invalidated"
	EventPatentLapsed             EventType = "patent.lapsed"
	EventPatentClaimsUpdated      EventType = "patent.claims_updated"
	EventPatentMoleculeLinked     EventType = "patent.molecule_linked"
	EventPatentMoleculeUnlinked   EventType = "patent.molecule_unlinked"
	EventPatentCitationAdded      EventType = "patent.citation_added"
	EventPatentAnalysisCompleted  EventType = "patent.analysis_completed"
)

// DomainEvent is the interface for all domain events.
type DomainEvent interface {
	EventID() string
	EventType() EventType
	AggregateID() string
	AggregateType() string
	OccurredAt() time.Time
	Version() int
	Payload() interface{}
}

// BaseEvent provides common functionality for all domain events.
type BaseEvent struct {
	id          string
	eventType   EventType
	aggregateID string
	occurredAt  time.Time
	version     int
}

func (e BaseEvent) EventID() string      { return e.id }
func (e BaseEvent) EventType() EventType { return e.eventType }
func (e BaseEvent) AggregateID() string  { return e.aggregateID }
func (e BaseEvent) AggregateType() string { return "Patent" }
func (e BaseEvent) OccurredAt() time.Time { return e.occurredAt }
func (e BaseEvent) Version() int         { return e.version }
func (e BaseEvent) Payload() interface{} { return nil }

// NewBaseEvent creates a new BaseEvent.
func NewBaseEvent(eventType EventType, aggregateID string, version int) BaseEvent {
	return BaseEvent{
		id:          uuid.New().String(),
		eventType:   eventType,
		aggregateID: aggregateID,
		occurredAt:  time.Now().UTC(),
		version:     version,
	}
}

// Concrete Events

type PatentCreatedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber string       `json:"patent_number"`
		Title        string       `json:"title"`
		Office       PatentOffice `json:"office"`
		FilingDate   time.Time    `json:"filing_date"`
	} `json:"data"`
}

func (e PatentCreatedEvent) Payload() interface{} { return e.Data }

func NewPatentCreatedEvent(p *Patent) *PatentCreatedEvent {
	e := &PatentCreatedEvent{
		BaseEvent: NewBaseEvent(EventPatentCreated, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.Title = p.Title
	e.Data.Office = p.Office
	if p.Dates.FilingDate != nil {
		e.Data.FilingDate = *p.Dates.FilingDate
	}
	return e
}

type PatentPublishedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber    string    `json:"patent_number"`
		PublicationDate time.Time `json:"publication_date"`
	} `json:"data"`
}

func (e PatentPublishedEvent) Payload() interface{} { return e.Data }

func NewPatentPublishedEvent(p *Patent) *PatentPublishedEvent {
	e := &PatentPublishedEvent{
		BaseEvent: NewBaseEvent(EventPatentPublished, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	if p.Dates.PublicationDate != nil {
		e.Data.PublicationDate = *p.Dates.PublicationDate
	}
	return e
}

type PatentExaminationStartedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber string `json:"patent_number"`
	} `json:"data"`
}

func (e PatentExaminationStartedEvent) Payload() interface{} { return e.Data }

func NewPatentExaminationStartedEvent(p *Patent) *PatentExaminationStartedEvent {
	e := &PatentExaminationStartedEvent{
		BaseEvent: NewBaseEvent(EventPatentExaminationStarted, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	return e
}

type PatentGrantedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber string    `json:"patent_number"`
		GrantDate    time.Time `json:"grant_date"`
		ExpiryDate   time.Time `json:"expiry_date"`
		ClaimCount   int       `json:"claim_count"`
	} `json:"data"`
}

func (e PatentGrantedEvent) Payload() interface{} { return e.Data }

func NewPatentGrantedEvent(p *Patent) *PatentGrantedEvent {
	e := &PatentGrantedEvent{
		BaseEvent: NewBaseEvent(EventPatentGranted, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	if p.Dates.GrantDate != nil {
		e.Data.GrantDate = *p.Dates.GrantDate
	}
	if p.Dates.ExpiryDate != nil {
		e.Data.ExpiryDate = *p.Dates.ExpiryDate
	}
	e.Data.ClaimCount = p.ClaimCount()
	return e
}

type PatentRejectedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber string `json:"patent_number"`
		Reason       string `json:"reason"`
	} `json:"data"`
}

func (e PatentRejectedEvent) Payload() interface{} { return e.Data }

func NewPatentRejectedEvent(p *Patent, reason string) *PatentRejectedEvent {
	e := &PatentRejectedEvent{
		BaseEvent: NewBaseEvent(EventPatentRejected, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.Reason = reason
	return e
}

type PatentWithdrawnEvent struct {
	BaseEvent
	Data struct {
		PatentNumber   string       `json:"patent_number"`
		PreviousStatus PatentStatus `json:"previous_status"`
	} `json:"data"`
}

func (e PatentWithdrawnEvent) Payload() interface{} { return e.Data }

func NewPatentWithdrawnEvent(p *Patent, previousStatus PatentStatus) *PatentWithdrawnEvent {
	e := &PatentWithdrawnEvent{
		BaseEvent: NewBaseEvent(EventPatentWithdrawn, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.PreviousStatus = previousStatus
	return e
}

type PatentExpiredEvent struct {
	BaseEvent
	Data struct {
		PatentNumber string    `json:"patent_number"`
		ExpiryDate   time.Time `json:"expiry_date"`
	} `json:"data"`
}

func (e PatentExpiredEvent) Payload() interface{} { return e.Data }

func NewPatentExpiredEvent(p *Patent) *PatentExpiredEvent {
	e := &PatentExpiredEvent{
		BaseEvent: NewBaseEvent(EventPatentExpired, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	if p.Dates.ExpiryDate != nil {
		e.Data.ExpiryDate = *p.Dates.ExpiryDate
	}
	return e
}

type PatentInvalidatedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber      string `json:"patent_number"`
		InvalidationReason string `json:"invalidation_reason"`
	} `json:"data"`
}

func (e PatentInvalidatedEvent) Payload() interface{} { return e.Data }

func NewPatentInvalidatedEvent(p *Patent, reason string) *PatentInvalidatedEvent {
	e := &PatentInvalidatedEvent{
		BaseEvent: NewBaseEvent(EventPatentInvalidated, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.InvalidationReason = reason
	return e
}

type PatentLapsedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber string `json:"patent_number"`
	} `json:"data"`
}

func (e PatentLapsedEvent) Payload() interface{} { return e.Data }

func NewPatentLapsedEvent(p *Patent) *PatentLapsedEvent {
	e := &PatentLapsedEvent{
		BaseEvent: NewBaseEvent(EventPatentLapsed, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	return e
}

type PatentClaimsUpdatedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber          string `json:"patent_number"`
		ClaimCount            int    `json:"claim_count"`
		IndependentClaimCount int    `json:"independent_claim_count"`
		HasMarkush            bool   `json:"has_markush"`
	} `json:"data"`
}

func (e PatentClaimsUpdatedEvent) Payload() interface{} { return e.Data }

func NewPatentClaimsUpdatedEvent(p *Patent) *PatentClaimsUpdatedEvent {
	e := &PatentClaimsUpdatedEvent{
		BaseEvent: NewBaseEvent(EventPatentClaimsUpdated, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.ClaimCount = p.ClaimCount()
	e.Data.IndependentClaimCount = p.IndependentClaimCount()
	e.Data.HasMarkush = p.Claims.HasMarkush() // Helper needed or check manually
	return e
}

// Helper to check for Markush in ClaimSet
func (cs ClaimSet) HasMarkush() bool {
	for _, c := range cs {
		if c.HasMarkushStructure() {
			return true
		}
	}
	return false
}

type PatentMoleculeLinkedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber         string `json:"patent_number"`
		MoleculeID           string `json:"molecule_id"`
		TotalLinkedMolecules int    `json:"total_linked_molecules"`
	} `json:"data"`
}

func (e PatentMoleculeLinkedEvent) Payload() interface{} { return e.Data }

func NewPatentMoleculeLinkedEvent(p *Patent, moleculeID string) *PatentMoleculeLinkedEvent {
	e := &PatentMoleculeLinkedEvent{
		BaseEvent: NewBaseEvent(EventPatentMoleculeLinked, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.MoleculeID = moleculeID
	e.Data.TotalLinkedMolecules = len(p.MoleculeIDs)
	return e
}

type PatentMoleculeUnlinkedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber         string `json:"patent_number"`
		MoleculeID           string `json:"molecule_id"`
		TotalLinkedMolecules int    `json:"total_linked_molecules"`
	} `json:"data"`
}

func (e PatentMoleculeUnlinkedEvent) Payload() interface{} { return e.Data }

func NewPatentMoleculeUnlinkedEvent(p *Patent, moleculeID string) *PatentMoleculeUnlinkedEvent {
	e := &PatentMoleculeUnlinkedEvent{
		BaseEvent: NewBaseEvent(EventPatentMoleculeUnlinked, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.MoleculeID = moleculeID
	e.Data.TotalLinkedMolecules = len(p.MoleculeIDs)
	return e
}

type PatentCitationAddedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber      string `json:"patent_number"`
		CitedPatentNumber string `json:"cited_patent_number"`
		Direction         string `json:"direction"` // forward/backward
	} `json:"data"`
}

func (e PatentCitationAddedEvent) Payload() interface{} { return e.Data }

func NewPatentCitationAddedEvent(p *Patent, citedNumber string, direction string) *PatentCitationAddedEvent {
	e := &PatentCitationAddedEvent{
		BaseEvent: NewBaseEvent(EventPatentCitationAdded, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.CitedPatentNumber = citedNumber
	e.Data.Direction = direction
	return e
}

type PatentAnalysisCompletedEvent struct {
	BaseEvent
	Data struct {
		PatentNumber  string `json:"patent_number"`
		AnalysisType  string `json:"analysis_type"`
		ResultSummary string `json:"result_summary"`
	} `json:"data"`
}

func (e PatentAnalysisCompletedEvent) Payload() interface{} { return e.Data }

func NewPatentAnalysisCompletedEvent(p *Patent, analysisType string, summary string) *PatentAnalysisCompletedEvent {
	e := &PatentAnalysisCompletedEvent{
		BaseEvent: NewBaseEvent(EventPatentAnalysisCompleted, p.ID, p.Version),
	}
	e.Data.PatentNumber = p.PatentNumber
	e.Data.AnalysisType = analysisType
	e.Data.ResultSummary = summary
	return e
}

// Interfaces

type EventHandler interface {
	Handle(ctx context.Context, event DomainEvent) error
	Supports() []EventType
}

type EventBus interface {
	Publish(ctx context.Context, events ...DomainEvent) error
	Subscribe(handler EventHandler) error
	Unsubscribe(handler EventHandler) error
}

type EventStore interface {
	Save(ctx context.Context, events ...DomainEvent) error
	Load(ctx context.Context, aggregateID string) ([]DomainEvent, error)
	LoadSince(ctx context.Context, aggregateID string, version int) ([]DomainEvent, error)
}

//Personal.AI order the ending
