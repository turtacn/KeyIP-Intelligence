// Package lifecycle defines the deadline management value objects and their
// business logic for patent lifecycle tracking.
package lifecycle

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// DeadlineType enumeration
// ─────────────────────────────────────────────────────────────────────────────

// DeadlineType categorizes the nature of a patent-related deadline.
type DeadlineType string

const (
	// DeadlineFilingResponse is the deadline to respond to an initial filing
	// office action or formality examination report.
	DeadlineFilingResponse DeadlineType = "filing_response"

	// DeadlineExamination is the deadline to request substantive examination
	// (applicable in jurisdictions that have a two-stage examination process).
	DeadlineExamination DeadlineType = "examination"

	// DeadlineOAResponse is the deadline to respond to an office action during
	// substantive examination (e.g., objections, rejections, clarity issues).
	DeadlineOAResponse DeadlineType = "oa_response"

	// DeadlineAnnuity is the deadline to pay an annual maintenance fee.
	DeadlineAnnuity DeadlineType = "annuity"

	// DeadlineRenewal is the deadline to renew a trademark, design, or other
	// renewable IP right (not typically used for utility patents).
	DeadlineRenewal DeadlineType = "renewal"

	// DeadlinePCTNationalPhase is the deadline to enter the national/regional
	// phase of a PCT application (typically 30 or 31 months from priority date).
	DeadlinePCTNationalPhase DeadlineType = "pct_national_phase"

	// DeadlineOpposition is the deadline to file or respond to an opposition
	// against a granted patent or published application.
	DeadlineOpposition DeadlineType = "opposition"

	// DeadlineAppeal is the deadline to file an appeal against an examiner's
	// final rejection or adverse decision.
	DeadlineAppeal DeadlineType = "appeal"

	// DeadlineCustom is a user-defined deadline that doesn't fit standard categories.
	DeadlineCustom DeadlineType = "custom"
)

// ─────────────────────────────────────────────────────────────────────────────
// DeadlinePriority enumeration
// ─────────────────────────────────────────────────────────────────────────────

// DeadlinePriority indicates the urgency level of a deadline.
type DeadlinePriority string

const (
	// PriorityCritical: missing this deadline results in irreversible loss
	// (e.g., patent rights abandoned, application deemed withdrawn).
	PriorityCritical DeadlinePriority = "critical"

	// PriorityHigh: missing this deadline has severe consequences but may be
	// recoverable with extraordinary measures (e.g., petition for revival).
	PriorityHigh DeadlinePriority = "high"

	// PriorityMedium: important deadline with moderate consequences (e.g.,
	// delay in prosecution, additional fees).
	PriorityMedium DeadlinePriority = "medium"

	// PriorityLow: informational or internal deadline with minimal external impact.
	PriorityLow DeadlinePriority = "low"
)

// ─────────────────────────────────────────────────────────────────────────────
// Deadline value object
// ─────────────────────────────────────────────────────────────────────────────

// Deadline represents a single time-bound obligation in the patent lifecycle.
// Deadlines are immutable once created, except for completion and extension.
type Deadline struct {
	// ID uniquely identifies this deadline.
	ID common.ID `json:"id"`

	// Type categorizes the deadline.
	Type DeadlineType `json:"type"`

	// DueDate is the statutory or contractual deadline (not considering extensions).
	DueDate time.Time `json:"due_date"`

	// Priority indicates the urgency and impact of missing this deadline.
	Priority DeadlinePriority `json:"priority"`

	// Description provides human-readable context about what action is required.
	Description string `json:"description"`

	// Completed indicates whether the required action has been taken.
	Completed bool `json:"completed"`

	// CompletedAt is the timestamp when the deadline was marked as completed.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// ReminderDays lists the number of days before the due date when reminders
	// should be sent (e.g., [30, 14, 7, 1] means send reminders 30 days, 14 days,
	// 7 days, and 1 day before the deadline).
	ReminderDays []int `json:"reminder_days"`

	// ExtensionAvailable indicates whether this deadline can be extended by
	// filing a request with the patent office.
	ExtensionAvailable bool `json:"extension_available"`

	// MaxExtensionDays is the maximum number of days by which the deadline can
	// be extended (0 if extension is not available).
	MaxExtensionDays int `json:"max_extension_days"`

	// ExtendedTo is the new due date if an extension has been granted.
	ExtendedTo *time.Time `json:"extended_to,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewDeadline creates a new Deadline value object with validation.
//
// Business rules:
//   - DueDate must not be zero
//   - Description must not be empty
//   - Default reminder schedule is [30, 14, 7, 1] days before due date
func NewDeadline(
	deadlineType DeadlineType,
	dueDate time.Time,
	priority DeadlinePriority,
	description string,
) (*Deadline, error) {
	if dueDate.IsZero() {
		return nil, errors.InvalidParam("due_date must not be zero")
	}
	if description == "" {
		return nil, errors.InvalidParam("description must not be empty")
	}

	return &Deadline{
		ID:                 common.NewID(),
		Type:               deadlineType,
		DueDate:            dueDate,
		Priority:           priority,
		Description:        description,
		Completed:          false,
		ReminderDays:       []int{30, 14, 7, 1}, // Default reminder schedule
		ExtensionAvailable: false,
		MaxExtensionDays:   0,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Query methods
// ─────────────────────────────────────────────────────────────────────────────

// IsOverdue returns true if the deadline has passed and has not been completed.
// Takes into account any granted extension.
func (d *Deadline) IsOverdue() bool {
	if d.Completed {
		return false
	}
	now := time.Now().UTC()
	effectiveDue := d.DueDate
	if d.ExtendedTo != nil {
		effectiveDue = *d.ExtendedTo
	}
	return now.After(effectiveDue)
}

// DaysUntilDue returns the number of days until the deadline is due.
// Negative values indicate the deadline is overdue.
// Takes into account any granted extension.
func (d *Deadline) DaysUntilDue() int {
	now := time.Now().UTC()
	effectiveDue := d.DueDate
	if d.ExtendedTo != nil {
		effectiveDue = *d.ExtendedTo
	}

	// Normalize both to the start of the day for date-only comparison.
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	due := time.Date(effectiveDue.Year(), effectiveDue.Month(), effectiveDue.Day(), 0, 0, 0, 0, time.UTC)

	days := int(due.Sub(today).Hours() / 24)
	return days
}

// NeedsAttention returns true if the deadline is incomplete and due within 7 days.
func (d *Deadline) NeedsAttention() bool {
	if d.Completed {
		return false
	}
	daysUntil := d.DaysUntilDue()
	return daysUntil >= 0 && daysUntil <= 7
}

// ShouldRemind returns true if today is one of the configured reminder days.
// For example, if ReminderDays = [30, 14, 7, 1] and the deadline is 7 days away,
// this returns true.
func (d *Deadline) ShouldRemind() bool {
	if d.Completed {
		return false
	}
	daysUntil := d.DaysUntilDue()
	for _, reminderDay := range d.ReminderDays {
		if daysUntil == reminderDay {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// Command methods
// ─────────────────────────────────────────────────────────────────────────────

// Complete marks the deadline as completed and records the completion timestamp.
func (d *Deadline) Complete() {
	now := time.Now().UTC()
	d.Completed = true
	d.CompletedAt = &now
}

// Extend grants an extension to the deadline.
//
// Business rules:
//   - Extension must be available (ExtensionAvailable = true)
//   - Extension days must not exceed MaxExtensionDays
//   - Cannot extend an already completed deadline
func (d *Deadline) Extend(days int) error {
	if d.Completed {
		return errors.InvalidState("cannot extend a completed deadline")
	}
	if !d.ExtensionAvailable {
		return errors.InvalidState("extension is not available for this deadline")
	}
	if days <= 0 {
		return errors.InvalidParam("extension days must be positive")
	}
	if days > d.MaxExtensionDays {
		return errors.InvalidParam(
			fmt.Sprintf("extension days (%d) exceed maximum allowed (%d)",
				days, d.MaxExtensionDays),
		)
	}

	newDueDate := d.DueDate.AddDate(0, 0, days)
	d.ExtendedTo = &newDueDate
	return nil
}

