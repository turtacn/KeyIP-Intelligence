// Package lifecycle implements the patent lifecycle management domain, which is
// the core subsystem responsible for tracking patent validity periods, managing
// renewal deadlines, calculating annuity payments, and monitoring legal status
// changes across multiple jurisdictions.
//
// Patent lifecycle management is mission-critical: missing a deadline can result
// in irreversible loss of patent rights, and incorrect annuity calculations can
// lead to financial penalties or inadvertent abandonment.
package lifecycle

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// PatentLifecycle — aggregate root for patent lifecycle management
// ─────────────────────────────────────────────────────────────────────────────

// PatentLifecycle is the aggregate root that encapsulates all lifecycle-related
// information for a single patent: deadlines, annuity schedule, legal status,
// and lifecycle events.  It enforces domain invariants and business rules that
// vary by jurisdiction.
type PatentLifecycle struct {
	// BaseEntity provides ID, tenant ID, and audit timestamps.
	common.BaseEntity

	// PatentID is the foreign key linking this lifecycle record to the patent entity.
	PatentID common.ID `json:"patent_id"`

	// PatentNumber is the official patent number in the jurisdiction's format
	// (e.g., "CN202310001234A", "US11123456B2").
	PatentNumber string `json:"patent_number"`

	// Jurisdiction is the patent office that issued this patent.
	Jurisdiction ptypes.JurisdictionCode `json:"jurisdiction"`

	// FilingDate is the original filing date (priority date if applicable).
	FilingDate time.Time `json:"filing_date"`

	// GrantDate is the date the patent was granted; nil for pending applications.
	GrantDate *time.Time `json:"grant_date,omitempty"`

	// ExpiryDate is the calculated statutory expiry date (typically FilingDate + 20 years).
	ExpiryDate time.Time `json:"expiry_date"`

	// Deadlines is the list of all administrative deadlines (OA responses,
	// examination requests, renewals, etc.) for this patent.
	Deadlines []Deadline `json:"deadlines"`

	// AnnuitySchedule contains the full payment schedule for all maintenance fees
	// over the patent's lifetime.
	AnnuitySchedule []AnnuityPayment `json:"annuity_schedule"`

	// LegalStatus tracks the current legal status and its history.
	LegalStatus LegalStatus `json:"legal_status"`

	// Events records all lifecycle events (status changes, deadline modifications,
	// payment confirmations) for audit purposes.
	Events []LifecycleEvent `json:"events"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Value objects
// ─────────────────────────────────────────────────────────────────────────────

// LegalStatus represents the current legal status of a patent and its full
// history of status changes.
type LegalStatus struct {
	// Current is the present legal status (e.g., "pending", "granted", "expired",
	// "abandoned", "lapsed").
	Current string `json:"current"`

	// History records all status transitions in chronological order.
	History []StatusChange `json:"history"`
}

// StatusChange records a single legal-status transition.
type StatusChange struct {
	// From is the previous status.
	From string `json:"from"`

	// To is the new status.
	To string `json:"to"`

	// Date is the effective date of the status change.
	Date time.Time `json:"date"`

	// Reason explains why the status changed (e.g., "granted by examiner",
	// "annuity not paid", "abandoned by applicant").
	Reason string `json:"reason"`
}

// LifecycleEvent is an audit log entry for significant lifecycle actions.
type LifecycleEvent struct {
	// Type categorizes the event (e.g., "deadline_added", "payment_recorded",
	// "status_changed", "deadline_extended").
	Type string `json:"type"`

	// Date is the timestamp when the event occurred.
	Date time.Time `json:"date"`

	// Description provides human-readable details about the event.
	Description string `json:"description"`

	// Handled indicates whether this event has been processed by downstream
	// systems (e.g., notification sent, calendar updated).
	Handled bool `json:"handled"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewPatentLifecycle creates a new PatentLifecycle aggregate with jurisdiction-
// specific deadlines and annuity schedule automatically generated.
//
// Business rules:
//   - PatentNumber must not be empty
//   - FilingDate must not be zero
//   - Jurisdiction must be a recognised code
//   - Initial legal status is "pending"
//   - Deadlines and annuity schedule are generated based on jurisdiction rules
func NewPatentLifecycle(
	patentID common.ID,
	patentNumber string,
	jurisdiction ptypes.JurisdictionCode,
	filingDate time.Time,
) (*PatentLifecycle, error) {
	if patentNumber == "" {
		return nil, errors.InvalidParam("patent_number must not be empty")
	}
	if filingDate.IsZero() {
		return nil, errors.InvalidParam("filing_date must not be zero")
	}

	// Fetch jurisdiction rules to generate initial lifecycle data.
	rules, err := GetJurisdictionRules(jurisdiction)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidParam, "unsupported jurisdiction")
	}

	// Calculate statutory expiry date.
	expiryDate := CalculateExpiryDate(jurisdiction, filingDate)

	// Generate initial deadlines (e.g., examination request deadline).
	deadlines, err := GenerateInitialDeadlines(jurisdiction, filingDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to generate initial deadlines")
	}

	// Generate annuity schedule (grant date is nil for pending applications).
	annuitySchedule, err := GenerateAnnuitySchedule(jurisdiction, filingDate, nil)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInternal, "failed to generate annuity schedule")
	}

	lc := &PatentLifecycle{
		BaseEntity: common.BaseEntity{
			ID:        common.NewID(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
		PatentID:        patentID,
		PatentNumber:    patentNumber,
		Jurisdiction:    jurisdiction,
		FilingDate:      filingDate,
		ExpiryDate:      expiryDate,
		Deadlines:       deadlines,
		AnnuitySchedule: annuitySchedule,
		LegalStatus: LegalStatus{
			Current: "pending",
			History: []StatusChange{
				{
					From:   "",
					To:     "pending",
					Date:   filingDate,
					Reason: "patent application filed",
				},
			},
		},
		Events: []LifecycleEvent{
			{
				Type:        "lifecycle_created",
				Date:        time.Now().UTC(),
				Description: fmt.Sprintf("lifecycle created for patent %s (%s)", patentNumber, rules.Code),
				Handled:     false,
			},
		},
	}

	return lc, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Deadline management methods
// ─────────────────────────────────────────────────────────────────────────────

// AddDeadline appends a new deadline to the lifecycle.
// Returns an error if the deadline is nil or has an ID conflict.
func (lc *PatentLifecycle) AddDeadline(deadline Deadline) error {
	if deadline.ID == "" {
		return errors.InvalidParam("deadline ID must not be empty")
	}

	// Check for ID collision.
	for _, d := range lc.Deadlines {
		if d.ID == deadline.ID {
			return errors.Conflict(fmt.Sprintf("deadline %s already exists", deadline.ID))
		}
	}

	lc.Deadlines = append(lc.Deadlines, deadline)
	lc.UpdatedAt = time.Now().UTC()
	lc.Events = append(lc.Events, LifecycleEvent{
		Type:        "deadline_added",
		Date:        time.Now().UTC(),
		Description: fmt.Sprintf("%s deadline added: %s", deadline.Type, deadline.Description),
		Handled:     false,
	})

	return nil
}

// GetUpcomingDeadlines returns all incomplete deadlines due within the specified
// number of days from today, sorted by due date (earliest first).
func (lc *PatentLifecycle) GetUpcomingDeadlines(withinDays int) []Deadline {
	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, withinDays)

	var upcoming []Deadline
	for _, d := range lc.Deadlines {
		if d.Completed {
			continue
		}
		effectiveDue := d.DueDate
		if d.ExtendedTo != nil {
			effectiveDue = *d.ExtendedTo
		}
		if effectiveDue.After(now) && effectiveDue.Before(cutoff) {
			upcoming = append(upcoming, d)
		}
	}

	// Sort by effective due date (simple bubble sort for small slices).
	for i := 0; i < len(upcoming); i++ {
		for j := i + 1; j < len(upcoming); j++ {
			di := upcoming[i].DueDate
			if upcoming[i].ExtendedTo != nil {
				di = *upcoming[i].ExtendedTo
			}
			dj := upcoming[j].DueDate
			if upcoming[j].ExtendedTo != nil {
				dj = *upcoming[j].ExtendedTo
			}
			if di.After(dj) {
				upcoming[i], upcoming[j] = upcoming[j], upcoming[i]
			}
		}
	}

	return upcoming
}

// GetOverdueDeadlines returns all incomplete deadlines whose effective due date
// has passed.
func (lc *PatentLifecycle) GetOverdueDeadlines() []Deadline {
	var overdue []Deadline
	for _, d := range lc.Deadlines {
		if d.IsOverdue() {
			overdue = append(overdue, d)
		}
	}
	return overdue
}

// MarkDeadlineCompleted marks the specified deadline as completed.
// Returns an error if the deadline is not found.
func (lc *PatentLifecycle) MarkDeadlineCompleted(deadlineID common.ID) error {
	for i := range lc.Deadlines {
		if lc.Deadlines[i].ID == deadlineID {
			lc.Deadlines[i].Complete()
			lc.UpdatedAt = time.Now().UTC()
			lc.Events = append(lc.Events, LifecycleEvent{
				Type:        "deadline_completed",
				Date:        time.Now().UTC(),
				Description: fmt.Sprintf("deadline %s completed: %s", deadlineID, lc.Deadlines[i].Description),
				Handled:     false,
			})
			return nil
		}
	}
	return errors.NotFound(fmt.Sprintf("deadline %s not found", deadlineID))
}

// ─────────────────────────────────────────────────────────────────────────────
// Annuity management methods
// ─────────────────────────────────────────────────────────────────────────────

// GetNextAnnuityPayment returns the next unpaid annuity payment, or nil if all
// are paid or none exist.
func (lc *PatentLifecycle) GetNextAnnuityPayment() *AnnuityPayment {
	now := time.Now().UTC()
	var next *AnnuityPayment

	for i := range lc.AnnuitySchedule {
		if lc.AnnuitySchedule[i].Paid {
			continue
		}
		if next == nil || lc.AnnuitySchedule[i].DueDate.Before(next.DueDate) {
			next = &lc.AnnuitySchedule[i]
		}
	}

	// If the next unpaid annuity is not yet due, return it anyway (it's the "next" one).
	// If it's overdue, definitely return it.
	_ = now
	return next
}

// RecordPayment records a payment for the specified annuity.
// Returns an error if the annuity is not found or the amount is insufficient.
func (lc *PatentLifecycle) RecordPayment(paymentID common.ID, amount float64, date time.Time) error {
	for i := range lc.AnnuitySchedule {
		if lc.AnnuitySchedule[i].ID == paymentID {
			if err := lc.AnnuitySchedule[i].Pay(amount); err != nil {
				return err
			}
			*lc.AnnuitySchedule[i].PaidAt = date
			lc.UpdatedAt = time.Now().UTC()
			lc.Events = append(lc.Events, LifecycleEvent{
				Type: "annuity_paid",
				Date: time.Now().UTC(),
				Description: fmt.Sprintf("year %d annuity paid: %.2f %s",
					lc.AnnuitySchedule[i].Year,
					amount,
					lc.AnnuitySchedule[i].Currency),
				Handled: false,
			})
			return nil
		}
	}
	return errors.NotFound(fmt.Sprintf("annuity payment %s not found", paymentID))
}

// ─────────────────────────────────────────────────────────────────────────────
// Legal status methods
// ─────────────────────────────────────────────────────────────────────────────

// UpdateLegalStatus transitions the patent to a new legal status and records
// the change in the history.
func (lc *PatentLifecycle) UpdateLegalStatus(newStatus, reason string) error {
	if newStatus == "" {
		return errors.InvalidParam("new_status must not be empty")
	}
	if reason == "" {
		return errors.InvalidParam("reason must not be empty")
	}

	oldStatus := lc.LegalStatus.Current
	lc.LegalStatus.Current = newStatus
	lc.LegalStatus.History = append(lc.LegalStatus.History, StatusChange{
		From:   oldStatus,
		To:     newStatus,
		Date:   time.Now().UTC(),
		Reason: reason,
	})

	lc.UpdatedAt = time.Now().UTC()
	lc.Events = append(lc.Events, LifecycleEvent{
		Type:        "status_changed",
		Date:        time.Now().UTC(),
		Description: fmt.Sprintf("status changed from %s to %s: %s", oldStatus, newStatus, reason),
		Handled:     false,
	})

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Utility methods
// ─────────────────────────────────────────────────────────────────────────────

// RemainingLifeYears calculates the number of years remaining until the patent
// expires.  Returns 0 if the patent has already expired.
func (lc *PatentLifecycle) RemainingLifeYears() float64 {
	now := time.Now().UTC()
	if now.After(lc.ExpiryDate) {
		return 0
	}
	duration := lc.ExpiryDate.Sub(now)
	years := duration.Hours() / 24 / 365.25
	return years
}

//Personal.AI order the ending
