package patent

import (
	"context"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// EventType constants
const (
	EventPatentCreated          common.EventType = "patent.created"
	EventPatentPublished        common.EventType = "patent.published"
	EventPatentExaminationStarted common.EventType = "patent.examination_started"
	EventPatentGranted          common.EventType = "patent.granted"
	EventPatentRejected         common.EventType = "patent.rejected"
	EventPatentWithdrawn        common.EventType = "patent.withdrawn"
	EventPatentExpired          common.EventType = "patent.expired"
	EventPatentInvalidated      common.EventType = "patent.invalidated"
	EventPatentLapsed           common.EventType = "patent.lapsed"
	EventPatentClaimsUpdated    common.EventType = "patent.claims_updated"
	EventPatentMoleculeLinked   common.EventType = "patent.molecule_linked"
	EventPatentMoleculeUnlinked common.EventType = "patent.molecule_unlinked"
	EventPatentCitationAdded    common.EventType = "patent.citation_added"
	EventPatentAnalysisCompleted common.EventType = "patent.analysis_completed"
)

// EventHandler interface for handling domain events.
type EventHandler interface {
	Handle(ctx context.Context, event common.DomainEvent) error
	Supports() []common.EventType
}

// EventBus interface for publishing and subscribing to domain events.
type EventBus interface {
	Publish(ctx context.Context, events ...common.DomainEvent) error
	Subscribe(handler EventHandler) error
	Unsubscribe(handler EventHandler) error
}

// EventStore interface for persisting domain events.
type EventStore interface {
	Save(ctx context.Context, events ...common.DomainEvent) error
	Load(ctx context.Context, aggregateID string) ([]common.DomainEvent, error)
	LoadSince(ctx context.Context, aggregateID string, version int) ([]common.DomainEvent, error)
}

// PatentCreatedEvent (Using PatentFiledEvent name in previous code, renaming to match prompt or keeping consistent)
// Prompt asked for PatentCreatedEvent.
type PatentCreatedEvent struct {
	common.BaseEvent
	PatentNumber string       `json:"patent_number"`
	Title        string       `json:"title"`
	Office       PatentOffice `json:"office"`
	FilingDate   *time.Time   `json:"filing_date"`
}

func NewPatentCreatedEvent(p *Patent) *PatentCreatedEvent {
	return &PatentCreatedEvent{
		BaseEvent:    common.NewBaseEventWithVersion(EventPatentCreated, p.ID.String(), p.Version),
		PatentNumber: p.PatentNumber,
		Title:        p.Title,
		Office:       p.Office,
		FilingDate:   p.Dates.FilingDate,
	}
}

type PatentPublishedEvent struct {
	common.BaseEvent
	PatentNumber    string     `json:"patent_number"`
	PublicationDate *time.Time `json:"publication_date"`
}

func NewPatentPublishedEvent(p *Patent) *PatentPublishedEvent {
	return &PatentPublishedEvent{
		BaseEvent:       common.NewBaseEventWithVersion(EventPatentPublished, p.ID.String(), p.Version),
		PatentNumber:    p.PatentNumber,
		PublicationDate: p.Dates.PublicationDate,
	}
}

type PatentGrantedEvent struct {
	common.BaseEvent
	PatentNumber string     `json:"patent_number"`
	GrantDate    *time.Time `json:"grant_date"`
	ExpiryDate   *time.Time `json:"expiry_date"`
	ClaimCount   int        `json:"claim_count"`
}

func NewPatentGrantedEvent(p *Patent) *PatentGrantedEvent {
	return &PatentGrantedEvent{
		BaseEvent:    common.NewBaseEventWithVersion(EventPatentGranted, p.ID.String(), p.Version),
		PatentNumber: p.PatentNumber,
		GrantDate:    p.Dates.GrantDate,
		ExpiryDate:   p.Dates.ExpiryDate,
		ClaimCount:   p.ClaimCount(),
	}
}

type PatentRejectedEvent struct {
	common.BaseEvent
	PatentNumber string `json:"patent_number"`
	Reason       string `json:"reason"`
}

func NewPatentRejectedEvent(p *Patent, reason string) *PatentRejectedEvent {
	return &PatentRejectedEvent{
		BaseEvent:    common.NewBaseEventWithVersion(EventPatentRejected, p.ID.String(), p.Version),
		PatentNumber: p.PatentNumber,
		Reason:       reason,
	}
}

type PatentWithdrawnEvent struct {
	common.BaseEvent
	PatentNumber   string       `json:"patent_number"`
	PreviousStatus PatentStatus `json:"previous_status"`
}

func NewPatentWithdrawnEvent(p *Patent, previousStatus PatentStatus) *PatentWithdrawnEvent {
	return &PatentWithdrawnEvent{
		BaseEvent:      common.NewBaseEventWithVersion(EventPatentWithdrawn, p.ID.String(), p.Version),
		PatentNumber:   p.PatentNumber,
		PreviousStatus: previousStatus,
	}
}

type PatentExpiredEvent struct {
	common.BaseEvent
	PatentNumber string     `json:"patent_number"`
	ExpiryDate   *time.Time `json:"expiry_date"`
}

func NewPatentExpiredEvent(p *Patent) *PatentExpiredEvent {
	return &PatentExpiredEvent{
		BaseEvent:    common.NewBaseEventWithVersion(EventPatentExpired, p.ID.String(), p.Version),
		PatentNumber: p.PatentNumber,
		ExpiryDate:   p.Dates.ExpiryDate,
	}
}

type PatentInvalidatedEvent struct {
	common.BaseEvent
	PatentNumber       string `json:"patent_number"`
	InvalidationReason string `json:"invalidation_reason"`
}

func NewPatentInvalidatedEvent(p *Patent, reason string) *PatentInvalidatedEvent {
	return &PatentInvalidatedEvent{
		BaseEvent:          common.NewBaseEventWithVersion(EventPatentInvalidated, p.ID.String(), p.Version),
		PatentNumber:       p.PatentNumber,
		InvalidationReason: reason,
	}
}

type PatentLapsedEvent struct {
	common.BaseEvent
	PatentNumber string `json:"patent_number"`
}

func NewPatentLapsedEvent(p *Patent) *PatentLapsedEvent {
	return &PatentLapsedEvent{
		BaseEvent:    common.NewBaseEventWithVersion(EventPatentLapsed, p.ID.String(), p.Version),
		PatentNumber: p.PatentNumber,
	}
}

type PatentClaimsUpdatedEvent struct {
	common.BaseEvent
	PatentNumber          string `json:"patent_number"`
	ClaimCount            int    `json:"claim_count"`
	IndependentClaimCount int    `json:"independent_claim_count"`
	HasMarkush            bool   `json:"has_markush"`
}

func NewPatentClaimsUpdatedEvent(p *Patent) *PatentClaimsUpdatedEvent {
	independentCount := 0
	hasMarkush := false
	for _, c := range p.Claims {
		if c.Type == ClaimTypeIndependent {
			independentCount++
		}
		if c.HasMarkushStructure() {
			hasMarkush = true
		}
	}

	return &PatentClaimsUpdatedEvent{
		BaseEvent:             common.NewBaseEventWithVersion(EventPatentClaimsUpdated, p.ID.String(), p.Version),
		PatentNumber:          p.PatentNumber,
		ClaimCount:            p.ClaimCount(),
		IndependentClaimCount: independentCount,
		HasMarkush:            hasMarkush,
	}
}

type PatentMoleculeLinkedEvent struct {
	common.BaseEvent
	PatentNumber         string `json:"patent_number"`
	MoleculeID           string `json:"molecule_id"`
	TotalLinkedMolecules int    `json:"total_linked_molecules"`
}

func NewPatentMoleculeLinkedEvent(p *Patent, moleculeID string) *PatentMoleculeLinkedEvent {
	return &PatentMoleculeLinkedEvent{
		BaseEvent:            common.NewBaseEventWithVersion(EventPatentMoleculeLinked, p.ID.String(), p.Version),
		PatentNumber:         p.PatentNumber,
		MoleculeID:           moleculeID,
		TotalLinkedMolecules: len(p.MoleculeIDs),
	}
}

type PatentMoleculeUnlinkedEvent struct {
	common.BaseEvent
	PatentNumber         string `json:"patent_number"`
	MoleculeID           string `json:"molecule_id"`
	TotalLinkedMolecules int    `json:"total_linked_molecules"`
}

func NewPatentMoleculeUnlinkedEvent(p *Patent, moleculeID string) *PatentMoleculeUnlinkedEvent {
	return &PatentMoleculeUnlinkedEvent{
		BaseEvent:            common.NewBaseEventWithVersion(EventPatentMoleculeUnlinked, p.ID.String(), p.Version),
		PatentNumber:         p.PatentNumber,
		MoleculeID:           moleculeID,
		TotalLinkedMolecules: len(p.MoleculeIDs),
	}
}

type PatentCitationAddedEvent struct {
	common.BaseEvent
	PatentNumber      string `json:"patent_number"`
	CitedPatentNumber string `json:"cited_patent_number"`
	Direction         string `json:"direction"` // "forward" or "backward"
}

func NewPatentCitationAddedEvent(p *Patent, citedNumber string, direction string) *PatentCitationAddedEvent {
	return &PatentCitationAddedEvent{
		BaseEvent:         common.NewBaseEventWithVersion(EventPatentCitationAdded, p.ID.String(), p.Version),
		PatentNumber:      p.PatentNumber,
		CitedPatentNumber: citedNumber,
		Direction:         direction,
	}
}

type PatentAnalysisCompletedEvent struct {
	common.BaseEvent
	PatentNumber  string `json:"patent_number"`
	AnalysisType  string `json:"analysis_type"`
	ResultSummary string `json:"result_summary"`
}

func NewPatentAnalysisCompletedEvent(p *Patent, analysisType string, summary string) *PatentAnalysisCompletedEvent {
	return &PatentAnalysisCompletedEvent{
		BaseEvent:     common.NewBaseEventWithVersion(EventPatentAnalysisCompleted, p.ID.String(), p.Version),
		PatentNumber:  p.PatentNumber,
		AnalysisType:  analysisType,
		ResultSummary: summary,
	}
}

//Personal.AI order the ending
