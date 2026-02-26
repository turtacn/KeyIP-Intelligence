package patent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// EventType identifies the type of a domain event.
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

// BaseEvent implements common behavior for domain events.
type BaseEvent struct {
	id            string
	eventType     EventType
	aggregateID   string
	occurredAt    time.Time
	version       int
}

func NewBaseEvent(eventType EventType, aggregateID string, version int) BaseEvent {
	return BaseEvent{
		id:          uuid.New().String(),
		eventType:   eventType,
		aggregateID: aggregateID,
		occurredAt:  time.Now().UTC(),
		version:     version,
	}
}

func (e BaseEvent) EventID() string {
	return e.id
}

func (e BaseEvent) EventType() EventType {
	return e.eventType
}

func (e BaseEvent) AggregateID() string {
	return e.aggregateID
}

func (e BaseEvent) AggregateType() string {
	return "Patent"
}

func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

func (e BaseEvent) Version() int {
	return e.version
}

// Concrete Events

type PatentCreatedEvent struct {
	BaseEvent
	PatentNumber string       `json:"patent_number"`
	Title        string       `json:"title"`
	Office       PatentOffice `json:"office"`
	FilingDate   time.Time    `json:"filing_date"`
}

func (e *PatentCreatedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number": e.PatentNumber,
		"title":         e.Title,
		"office":        e.Office,
		"filing_date":   e.FilingDate,
	}
}

func NewPatentCreatedEvent(patent *Patent) *PatentCreatedEvent {
	filingDate := time.Time{}
	if patent.Dates.FilingDate != nil {
		filingDate = *patent.Dates.FilingDate
	}
	return &PatentCreatedEvent{
		BaseEvent:    NewBaseEvent(EventPatentCreated, patent.ID, patent.Version),
		PatentNumber: patent.PatentNumber,
		Title:        patent.Title,
		Office:       patent.Office,
		FilingDate:   filingDate,
	}
}

type PatentPublishedEvent struct {
	BaseEvent
	PatentNumber    string    `json:"patent_number"`
	PublicationDate time.Time `json:"publication_date"`
}

func (e *PatentPublishedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":    e.PatentNumber,
		"publication_date": e.PublicationDate,
	}
}

func NewPatentPublishedEvent(patent *Patent) *PatentPublishedEvent {
	pubDate := time.Time{}
	if patent.Dates.PublicationDate != nil {
		pubDate = *patent.Dates.PublicationDate
	}
	return &PatentPublishedEvent{
		BaseEvent:       NewBaseEvent(EventPatentPublished, patent.ID, patent.Version),
		PatentNumber:    patent.PatentNumber,
		PublicationDate: pubDate,
	}
}

type PatentExaminationStartedEvent struct {
	BaseEvent
	PatentNumber string `json:"patent_number"`
}

func (e *PatentExaminationStartedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number": e.PatentNumber,
	}
}

func NewPatentExaminationStartedEvent(patent *Patent) *PatentExaminationStartedEvent {
	return &PatentExaminationStartedEvent{
		BaseEvent:    NewBaseEvent(EventPatentExaminationStarted, patent.ID, patent.Version),
		PatentNumber: patent.PatentNumber,
	}
}

type PatentGrantedEvent struct {
	BaseEvent
	PatentNumber string    `json:"patent_number"`
	GrantDate    time.Time `json:"grant_date"`
	ExpiryDate   time.Time `json:"expiry_date"`
	ClaimCount   int       `json:"claim_count"`
}

func (e *PatentGrantedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number": e.PatentNumber,
		"grant_date":    e.GrantDate,
		"expiry_date":   e.ExpiryDate,
		"claim_count":   e.ClaimCount,
	}
}

func NewPatentGrantedEvent(patent *Patent) *PatentGrantedEvent {
	grantDate := time.Time{}
	if patent.Dates.GrantDate != nil {
		grantDate = *patent.Dates.GrantDate
	}
	expiryDate := time.Time{}
	if patent.Dates.ExpiryDate != nil {
		expiryDate = *patent.Dates.ExpiryDate
	}
	return &PatentGrantedEvent{
		BaseEvent:    NewBaseEvent(EventPatentGranted, patent.ID, patent.Version),
		PatentNumber: patent.PatentNumber,
		GrantDate:    grantDate,
		ExpiryDate:   expiryDate,
		ClaimCount:   patent.ClaimCount(),
	}
}

type PatentRejectedEvent struct {
	BaseEvent
	PatentNumber string `json:"patent_number"`
	Reason       string `json:"reason"`
}

func (e *PatentRejectedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number": e.PatentNumber,
		"reason":        e.Reason,
	}
}

func NewPatentRejectedEvent(patent *Patent, reason string) *PatentRejectedEvent {
	return &PatentRejectedEvent{
		BaseEvent:    NewBaseEvent(EventPatentRejected, patent.ID, patent.Version),
		PatentNumber: patent.PatentNumber,
		Reason:       reason,
	}
}

type PatentWithdrawnEvent struct {
	BaseEvent
	PatentNumber   string       `json:"patent_number"`
	PreviousStatus PatentStatus `json:"previous_status"`
}

func (e *PatentWithdrawnEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":   e.PatentNumber,
		"previous_status": e.PreviousStatus,
	}
}

func NewPatentWithdrawnEvent(patent *Patent, previousStatus PatentStatus) *PatentWithdrawnEvent {
	return &PatentWithdrawnEvent{
		BaseEvent:      NewBaseEvent(EventPatentWithdrawn, patent.ID, patent.Version),
		PatentNumber:   patent.PatentNumber,
		PreviousStatus: previousStatus,
	}
}

type PatentExpiredEvent struct {
	BaseEvent
	PatentNumber string    `json:"patent_number"`
	ExpiryDate   time.Time `json:"expiry_date"`
}

func (e *PatentExpiredEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number": e.PatentNumber,
		"expiry_date":   e.ExpiryDate,
	}
}

func NewPatentExpiredEvent(patent *Patent) *PatentExpiredEvent {
	expiryDate := time.Time{}
	if patent.Dates.ExpiryDate != nil {
		expiryDate = *patent.Dates.ExpiryDate
	}
	return &PatentExpiredEvent{
		BaseEvent:    NewBaseEvent(EventPatentExpired, patent.ID, patent.Version),
		PatentNumber: patent.PatentNumber,
		ExpiryDate:   expiryDate,
	}
}

type PatentInvalidatedEvent struct {
	BaseEvent
	PatentNumber       string `json:"patent_number"`
	InvalidationReason string `json:"invalidation_reason"`
}

func (e *PatentInvalidatedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":       e.PatentNumber,
		"invalidation_reason": e.InvalidationReason,
	}
}

func NewPatentInvalidatedEvent(patent *Patent, reason string) *PatentInvalidatedEvent {
	return &PatentInvalidatedEvent{
		BaseEvent:          NewBaseEvent(EventPatentInvalidated, patent.ID, patent.Version),
		PatentNumber:       patent.PatentNumber,
		InvalidationReason: reason,
	}
}

type PatentLapsedEvent struct {
	BaseEvent
	PatentNumber string `json:"patent_number"`
}

func (e *PatentLapsedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number": e.PatentNumber,
	}
}

func NewPatentLapsedEvent(patent *Patent) *PatentLapsedEvent {
	return &PatentLapsedEvent{
		BaseEvent:    NewBaseEvent(EventPatentLapsed, patent.ID, patent.Version),
		PatentNumber: patent.PatentNumber,
	}
}

type PatentClaimsUpdatedEvent struct {
	BaseEvent
	PatentNumber          string `json:"patent_number"`
	ClaimCount            int    `json:"claim_count"`
	IndependentClaimCount int    `json:"independent_claim_count"`
	HasMarkush            bool   `json:"has_markush"`
}

func (e *PatentClaimsUpdatedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":           e.PatentNumber,
		"claim_count":             e.ClaimCount,
		"independent_claim_count": e.IndependentClaimCount,
		"has_markush":             e.HasMarkush,
	}
}

func NewPatentClaimsUpdatedEvent(patent *Patent) *PatentClaimsUpdatedEvent {
	hasMarkush := false
	for _, c := range patent.Claims {
		if c.HasMarkushStructure() {
			hasMarkush = true
			break
		}
	}
	return &PatentClaimsUpdatedEvent{
		BaseEvent:             NewBaseEvent(EventPatentClaimsUpdated, patent.ID, patent.Version),
		PatentNumber:          patent.PatentNumber,
		ClaimCount:            patent.ClaimCount(),
		IndependentClaimCount: patent.IndependentClaimCount(),
		HasMarkush:            hasMarkush,
	}
}

type PatentMoleculeLinkedEvent struct {
	BaseEvent
	PatentNumber         string `json:"patent_number"`
	MoleculeID           string `json:"molecule_id"`
	TotalLinkedMolecules int    `json:"total_linked_molecules"`
}

func (e *PatentMoleculeLinkedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":          e.PatentNumber,
		"molecule_id":            e.MoleculeID,
		"total_linked_molecules": e.TotalLinkedMolecules,
	}
}

func NewPatentMoleculeLinkedEvent(patent *Patent, moleculeID string) *PatentMoleculeLinkedEvent {
	return &PatentMoleculeLinkedEvent{
		BaseEvent:            NewBaseEvent(EventPatentMoleculeLinked, patent.ID, patent.Version),
		PatentNumber:         patent.PatentNumber,
		MoleculeID:           moleculeID,
		TotalLinkedMolecules: len(patent.MoleculeIDs),
	}
}

type PatentMoleculeUnlinkedEvent struct {
	BaseEvent
	PatentNumber         string `json:"patent_number"`
	MoleculeID           string `json:"molecule_id"`
	TotalLinkedMolecules int    `json:"total_linked_molecules"`
}

func (e *PatentMoleculeUnlinkedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":          e.PatentNumber,
		"molecule_id":            e.MoleculeID,
		"total_linked_molecules": e.TotalLinkedMolecules,
	}
}

func NewPatentMoleculeUnlinkedEvent(patent *Patent, moleculeID string) *PatentMoleculeUnlinkedEvent {
	return &PatentMoleculeUnlinkedEvent{
		BaseEvent:            NewBaseEvent(EventPatentMoleculeUnlinked, patent.ID, patent.Version),
		PatentNumber:         patent.PatentNumber,
		MoleculeID:           moleculeID,
		TotalLinkedMolecules: len(patent.MoleculeIDs),
	}
}

type PatentCitationAddedEvent struct {
	BaseEvent
	PatentNumber      string `json:"patent_number"`
	CitedPatentNumber string `json:"cited_patent_number"`
	Direction         string `json:"direction"` // forward or backward
}

func (e *PatentCitationAddedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":       e.PatentNumber,
		"cited_patent_number": e.CitedPatentNumber,
		"direction":           e.Direction,
	}
}

func NewPatentCitationAddedEvent(patent *Patent, citedNumber string, direction string) *PatentCitationAddedEvent {
	return &PatentCitationAddedEvent{
		BaseEvent:         NewBaseEvent(EventPatentCitationAdded, patent.ID, patent.Version),
		PatentNumber:      patent.PatentNumber,
		CitedPatentNumber: citedNumber,
		Direction:         direction,
	}
}

type PatentAnalysisCompletedEvent struct {
	BaseEvent
	PatentNumber  string `json:"patent_number"`
	AnalysisType  string `json:"analysis_type"`
	ResultSummary string `json:"result_summary"`
}

func (e *PatentAnalysisCompletedEvent) Payload() interface{} {
	return map[string]interface{}{
		"patent_number":  e.PatentNumber,
		"analysis_type":  e.AnalysisType,
		"result_summary": e.ResultSummary,
	}
}

func NewPatentAnalysisCompletedEvent(patent *Patent, analysisType string, summary string) *PatentAnalysisCompletedEvent {
	return &PatentAnalysisCompletedEvent{
		BaseEvent:     NewBaseEvent(EventPatentAnalysisCompleted, patent.ID, patent.Version),
		PatentNumber:  patent.PatentNumber,
		AnalysisType:  analysisType,
		ResultSummary: summary,
	}
}

// EventHandler handles domain events.
type EventHandler interface {
	Handle(ctx context.Context, event DomainEvent) error
	Supports() []EventType
}

// EventBus publishes domain events.
type EventBus interface {
	Publish(ctx context.Context, events ...DomainEvent) error
	Subscribe(handler EventHandler) error
	Unsubscribe(handler EventHandler) error
}

// EventStore persists domain events.
type EventStore interface {
	Save(ctx context.Context, events ...DomainEvent) error
	Load(ctx context.Context, aggregateID string) ([]DomainEvent, error)
	LoadSince(ctx context.Context, aggregateID string, version int) ([]DomainEvent, error)
}

//Personal.AI order the ending
