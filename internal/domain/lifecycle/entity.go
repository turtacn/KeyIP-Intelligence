package lifecycle

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// LifecyclePhase defines the phase of a patent lifecycle.
type LifecyclePhase string

const (
	PhaseApplication LifecyclePhase = "application"
	PhaseExamination LifecyclePhase = "examination"
	PhaseGranted     LifecyclePhase = "granted"
	PhaseMaintenance LifecyclePhase = "maintenance"
	PhaseExpired     LifecyclePhase = "expired"
	PhaseAbandoned   LifecyclePhase = "abandoned"
	PhaseRevoked     LifecyclePhase = "revoked"
	PhaseLapsed      LifecyclePhase = "lapsed"
)

// LifecycleEvent represents an event in the lifecycle.
type LifecycleEvent struct {
	ID          string            `json:"id"`
	EventType   string            `json:"event_type"`
	EventDate   time.Time         `json:"event_date"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata"`
	TriggeredBy string            `json:"triggered_by"`
	CreatedAt   time.Time         `json:"created_at"`
}

// PhaseTransition records a transition between phases.
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
	GrantDate          *time.Time        `json:"grant_date"`
	ExpirationDate     *time.Time        `json:"expiration_date"`
	AbandonmentDate    *time.Time        `json:"abandonment_date"`
	JurisdictionCode   string            `json:"jurisdiction_code"`
	RemainingLifeYears float64           `json:"remaining_life_years"` // Computed
	TotalLifeYears     float64           `json:"total_life_years"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// NewLifecycleRecord creates a new lifecycle record.
func NewLifecycleRecord(patentID, jurisdictionCode string, filingDate time.Time) (*LifecycleRecord, error) {
	if patentID == "" {
		return nil, errors.New("patent ID cannot be empty")
	}
	if jurisdictionCode == "" {
		return nil, errors.New("jurisdiction code cannot be empty")
	}
	if filingDate.IsZero() {
		return nil, errors.New("filing date cannot be zero")
	}

	now := time.Now().UTC()

	// Default TotalLifeYears to 20. Can be adjusted by service based on jurisdiction rules.
	totalLife := 20.0
	expDate := filingDate.AddDate(int(totalLife), 0, 0)

	lr := &LifecycleRecord{
		ID:               uuid.New().String(),
		PatentID:         patentID,
		CurrentPhase:     PhaseApplication,
		PhaseHistory:     []PhaseTransition{},
		Events:           []*LifecycleEvent{},
		FilingDate:       filingDate,
		ExpirationDate:   &expDate,
		JurisdictionCode: jurisdictionCode,
		TotalLifeYears:   totalLife,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	lr.AddEvent("filed", "Patent application filed", "system", nil)
	return lr, nil
}

// TransitionTo transitions the lifecycle to a new phase.
func (lr *LifecycleRecord) TransitionTo(phase LifecyclePhase, reason, triggeredBy string) error {
	if !lr.isValidTransition(lr.CurrentPhase, phase) {
		return fmt.Errorf("invalid transition from %s to %s", lr.CurrentPhase, phase)
	}

	now := time.Now().UTC()
	transition := PhaseTransition{
		FromPhase:      lr.CurrentPhase,
		ToPhase:        phase,
		TransitionDate: now,
		Reason:         reason,
		TriggeredBy:    triggeredBy,
	}

	lr.PhaseHistory = append(lr.PhaseHistory, transition)
	lr.CurrentPhase = phase
	lr.UpdatedAt = now

	lr.AddEvent("phase_change", fmt.Sprintf("Phase changed to %s: %s", phase, reason), triggeredBy, nil)
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

// CalculateRemainingLife calculates remaining life years.
func (lr *LifecycleRecord) CalculateRemainingLife(asOf time.Time) float64 {
	if !lr.IsActive() {
		return 0
	}
	if lr.ExpirationDate == nil {
		return 0
	}
	if asOf.After(*lr.ExpirationDate) {
		return 0
	}
	diff := lr.ExpirationDate.Sub(asOf)
	return diff.Hours() / 24 / 365.25
}

// IsActive checks if the patent is in an active phase.
func (lr *LifecycleRecord) IsActive() bool {
	switch lr.CurrentPhase {
	case PhaseApplication, PhaseExamination, PhaseGranted, PhaseMaintenance:
		return true
	default:
		return false
	}
}

// MarkGranted marks the patent as granted.
func (lr *LifecycleRecord) MarkGranted(grantDate time.Time) error {
	if lr.CurrentPhase != PhaseExamination {
		return fmt.Errorf("cannot mark granted from phase: %s", lr.CurrentPhase)
	}
	lr.GrantDate = &grantDate
	return lr.TransitionTo(PhaseGranted, "Patent granted", "system")
}

// MarkAbandoned marks the patent as abandoned.
func (lr *LifecycleRecord) MarkAbandoned(reason string) error {
	if !lr.IsActive() {
		return fmt.Errorf("cannot abandon inactive patent in phase: %s", lr.CurrentPhase)
	}
	now := time.Now().UTC()
	lr.AbandonmentDate = &now
	return lr.TransitionTo(PhaseAbandoned, reason, "system")
}

// Validate checks the integrity of the record.
func (lr *LifecycleRecord) Validate() error {
	if lr.ID == "" {
		return errors.New("ID cannot be empty")
	}
	if lr.PatentID == "" {
		return errors.New("PatentID cannot be empty")
	}
	if lr.JurisdictionCode == "" {
		return errors.New("JurisdictionCode cannot be empty")
	}
	if lr.FilingDate.IsZero() {
		return errors.New("FilingDate cannot be zero")
	}

	// Validate PhaseHistory consistency
	if len(lr.PhaseHistory) > 0 {
		last := lr.PhaseHistory[len(lr.PhaseHistory)-1]
		if last.ToPhase != lr.CurrentPhase {
			return errors.New("PhaseHistory inconsistent with CurrentPhase")
		}
	}
	return nil
}

func (lr *LifecycleRecord) isValidTransition(from, to LifecyclePhase) bool {
	validTransitions := map[LifecyclePhase][]LifecyclePhase{
		PhaseApplication: {PhaseExamination, PhaseAbandoned},
		PhaseExamination: {PhaseGranted, PhaseAbandoned},
		PhaseGranted:     {PhaseMaintenance, PhaseRevoked, PhaseAbandoned},
		PhaseMaintenance: {PhaseExpired, PhaseLapsed, PhaseAbandoned},
		// Terminal states have no transitions
	}

	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

//Personal.AI order the ending
