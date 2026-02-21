package lifecycle

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// LifecyclePhase defines the phases of a patent lifecycle.
type LifecyclePhase string

const (
	PhaseApplication LifecyclePhase = "Application"
	PhaseExamination LifecyclePhase = "Examination"
	PhaseGranted     LifecyclePhase = "Granted"
	PhaseMaintenance LifecyclePhase = "Maintenance"
	PhaseExpired     LifecyclePhase = "Expired"
	PhaseAbandoned   LifecyclePhase = "Abandoned"
	PhaseRevoked     LifecyclePhase = "Revoked"
	PhaseLapsed      LifecyclePhase = "Lapsed"
)

// LifecycleEvent represents an event in the patent lifecycle.
type LifecycleEvent struct {
	ID          string            `json:"id"`
	EventType   string            `json:"event_type"`
	EventDate   time.Time         `json:"event_date"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	TriggeredBy string            `json:"triggered_by"`
	CreatedAt   time.Time         `json:"created_at"`
}

// PhaseTransition represents a transition between lifecycle phases.
type PhaseTransition struct {
	FromPhase      LifecyclePhase `json:"from_phase"`
	ToPhase        LifecyclePhase `json:"to_phase"`
	TransitionDate time.Time      `json:"transition_date"`
	Reason         string         `json:"reason"`
	TriggeredBy    string         `json:"triggered_by"`
}

// LifecycleRecord is the aggregate root for patent lifecycle.
type LifecycleRecord struct {
	ID                 string            `json:"id"`
	PatentID           string            `json:"patent_id"`
	CurrentPhase       LifecyclePhase    `json:"current_phase"`
	PhaseHistory       []PhaseTransition `json:"phase_history"`
	Events             []*LifecycleEvent `json:"events"`
	FilingDate         time.Time         `json:"filing_date"`
	GrantDate          *time.Time        `json:"grant_date,omitempty"`
	ExpirationDate     *time.Time        `json:"expiration_date,omitempty"`
	AbandonmentDate    *time.Time        `json:"abandonment_date,omitempty"`
	JurisdictionCode   string            `json:"jurisdiction_code"`
	RemainingLifeYears float64           `json:"remaining_life_years"`
	TotalLifeYears     float64           `json:"total_life_years"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

var validTransitions = map[LifecyclePhase][]LifecyclePhase{
	PhaseApplication: {PhaseExamination, PhaseAbandoned},
	PhaseExamination: {PhaseGranted, PhaseAbandoned},
	PhaseGranted:     {PhaseMaintenance, PhaseRevoked, PhaseAbandoned},
	PhaseMaintenance: {PhaseExpired, PhaseLapsed, PhaseAbandoned},
}

// NewLifecycleRecord creates a new LifecycleRecord.
func NewLifecycleRecord(patentID, jurisdictionCode string, filingDate time.Time) (*LifecycleRecord, error) {
	if patentID == "" {
		return nil, errors.InvalidParam("patent ID cannot be empty")
	}
	if jurisdictionCode == "" {
		return nil, errors.InvalidParam("jurisdiction code cannot be empty")
	}
	if filingDate.IsZero() {
		return nil, errors.InvalidParam("filing date cannot be zero")
	}

	totalLifeYears := 20.0
	// Simplified: utility models might have different terms, but for now default to 20
	expirationDate := filingDate.AddDate(int(totalLifeYears), 0, 0)

	lr := &LifecycleRecord{
		ID:               uuid.New().String(),
		PatentID:         patentID,
		CurrentPhase:     PhaseApplication,
		FilingDate:       filingDate,
		ExpirationDate:   &expirationDate,
		JurisdictionCode: jurisdictionCode,
		TotalLifeYears:   totalLifeYears,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	lr.AddEvent("filed", "Patent application filed", "system", nil)

	return lr, nil
}

// TransitionTo changes the lifecycle phase.
func (lr *LifecycleRecord) TransitionTo(phase LifecyclePhase, reason, triggeredBy string) error {
	allowed, ok := validTransitions[lr.CurrentPhase]
	if !ok {
		return errors.InvalidState(fmt.Sprintf("cannot transition from terminal phase %s", lr.CurrentPhase))
	}

	isValid := false
	for _, p := range allowed {
		if p == phase {
			isValid = true
			break
		}
	}

	if !isValid {
		return errors.InvalidState(fmt.Sprintf("invalid transition from %s to %s", lr.CurrentPhase, phase))
	}

	transition := PhaseTransition{
		FromPhase:      lr.CurrentPhase,
		ToPhase:        phase,
		TransitionDate: time.Now().UTC(),
		Reason:         reason,
		TriggeredBy:    triggeredBy,
	}

	lr.PhaseHistory = append(lr.PhaseHistory, transition)
	lr.CurrentPhase = phase
	lr.UpdatedAt = time.Now().UTC()

	lr.AddEvent(string(phase), fmt.Sprintf("Phase transitioned to %s", phase), triggeredBy, map[string]string{"reason": reason})

	return nil
}

// AddEvent adds a new event to the record.
func (lr *LifecycleRecord) AddEvent(eventType, description, triggeredBy string, metadata map[string]string) *LifecycleEvent {
	event := &LifecycleEvent{
		ID:          uuid.New().String(),
		EventType:   eventType,
		EventDate:   time.Now().UTC(),
		Description: description,
		Metadata:    metadata,
		TriggeredBy: triggeredBy,
		CreatedAt:   time.Now().UTC(),
	}
	lr.Events = append(lr.Events, event)
	return event
}

// CalculateRemainingLife computes the remaining life in years.
func (lr *LifecycleRecord) CalculateRemainingLife(asOf time.Time) float64 {
	if !lr.IsActive() {
		return 0
	}
	if lr.ExpirationDate == nil || asOf.After(*lr.ExpirationDate) {
		return 0
	}
	return lr.ExpirationDate.Sub(asOf).Hours() / 24 / 365.25
}

// IsActive checks if the patent is in an active phase.
func (lr *LifecycleRecord) IsActive() bool {
	switch lr.CurrentPhase {
	case PhaseApplication, PhaseExamination, PhaseGranted, PhaseMaintenance:
		return true
	}
	return false
}

// MarkGranted marks the patent as granted.
func (lr *LifecycleRecord) MarkGranted(grantDate time.Time) error {
	if lr.CurrentPhase != PhaseExamination {
		return errors.InvalidState("can only mark as granted from Examination phase")
	}
	lr.GrantDate = &grantDate
	return lr.TransitionTo(PhaseGranted, "Patent granted", "system")
}

// MarkAbandoned marks the patent as abandoned.
func (lr *LifecycleRecord) MarkAbandoned(reason string) error {
	if !lr.IsActive() {
		return errors.InvalidState("can only abandon active patent")
	}
	now := time.Now().UTC()
	lr.AbandonmentDate = &now
	return lr.TransitionTo(PhaseAbandoned, reason, "system")
}

// Validate checks the integrity of the lifecycle record.
func (lr *LifecycleRecord) Validate() error {
	if lr.ID == "" {
		return errors.InvalidParam("ID cannot be empty")
	}
	if lr.PatentID == "" {
		return errors.InvalidParam("PatentID cannot be empty")
	}
	if lr.JurisdictionCode == "" {
		return errors.InvalidParam("JurisdictionCode cannot be empty")
	}
	if lr.FilingDate.IsZero() {
		return errors.InvalidParam("FilingDate cannot be zero")
	}
	// Check phase history consistency
	if len(lr.PhaseHistory) > 0 {
		if lr.PhaseHistory[len(lr.PhaseHistory)-1].ToPhase != lr.CurrentPhase {
			return errors.InvalidState("current phase inconsistent with history")
		}
	}
	return nil
}

//Personal.AI order the ending
