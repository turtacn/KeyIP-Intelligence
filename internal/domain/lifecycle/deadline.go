package lifecycle

import (
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// DeadlineStatus represents the status of a deadline.
type DeadlineStatus string

const (
	DeadlineStatusActive    DeadlineStatus = "active"
	DeadlineStatusCompleted DeadlineStatus = "completed"
	DeadlineStatusMissed    DeadlineStatus = "missed"
	DeadlineStatusExtended  DeadlineStatus = "extended"
	DeadlineStatusWaived    DeadlineStatus = "waived"
)

// UrgencyLevel indicates how critical a deadline is.
type UrgencyLevel string

const (
	UrgencyNormal   UrgencyLevel = "normal"
	UrgencyHigh     UrgencyLevel = "high"
	UrgencyCritical UrgencyLevel = "critical"
	UrgencyOverdue  UrgencyLevel = "overdue"
)

// Deadline represents a critical date.
type Deadline struct {
	ID              string         `json:"id"`
	PatentID        string         `json:"patent_id"`
	DeadlineType    string         `json:"deadline_type"`
	Title           string         `json:"title"`
	Description     string         `json:"description,omitempty"`
	DueDate         time.Time      `json:"due_date"`
	OriginalDueDate time.Time      `json:"original_due_date"`
	Status          DeadlineStatus `json:"status"`
	Priority        string         `json:"priority"`
	AssigneeID      *string        `json:"assignee_id,omitempty"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	CompletedBy     *string        `json:"completed_by,omitempty"`
	ExtensionCount  int            `json:"extension_count"`
	ExtensionHistory []map[string]any `json:"extension_history,omitempty"`
	ReminderConfig  map[string]any `json:"reminder_config,omitempty"`
	LastReminderAt  *time.Time     `json:"last_reminder_at,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// NewDeadline creates a new deadline.
func NewDeadline(patentID, title string, dueDate time.Time) (*Deadline, error) {
	if patentID == "" {
		return nil, errors.NewValidation("patentID is required")
	}
	if title == "" {
		return nil, errors.NewValidation("title is required")
	}

	now := time.Time(common.NewTimestamp())
	return &Deadline{
		ID:              string(common.NewID()),
		PatentID:        patentID,
		Title:           title,
		DueDate:         dueDate,
		OriginalDueDate: dueDate,
		Status:          DeadlineStatusActive,
		Priority:        "medium",
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// CheckUrgency calculates the urgency level based on due date.
func (d *Deadline) CheckUrgency() UrgencyLevel {
	if d.Status != DeadlineStatusActive && d.Status != DeadlineStatusExtended {
		return UrgencyNormal
	}

	now := time.Time(common.NewTimestamp())
	if now.After(d.DueDate) {
		return UrgencyOverdue
	}

	daysUntil := int(d.DueDate.Sub(now).Hours() / 24)
	if daysUntil <= 7 {
		return UrgencyCritical
	}
	if daysUntil <= 30 {
		return UrgencyHigh
	}
	return UrgencyNormal
}

// Complete marks the deadline as completed.
func (d *Deadline) Complete(completedBy string) error {
	if d.Status == DeadlineStatusCompleted {
		return errors.NewValidation("deadline already completed")
	}
	d.Status = DeadlineStatusCompleted
	now := time.Time(common.NewTimestamp())
	d.CompletedAt = &now
	d.CompletedBy = &completedBy
	d.UpdatedAt = now
	return nil
}

// Extend extends the deadline.
func (d *Deadline) Extend(newDate time.Time, reason string) error {
	if newDate.Before(d.DueDate) {
		return errors.NewValidation("new due date must be after current due date")
	}
	d.DueDate = newDate
	d.Status = DeadlineStatusExtended
	d.ExtensionCount++

	entry := map[string]any{
		"date":   time.Time(common.NewTimestamp()),
		"reason": reason,
		"to":     newDate,
	}
	d.ExtensionHistory = append(d.ExtensionHistory, entry)
	d.UpdatedAt = time.Time(common.NewTimestamp())
	return nil
}

// DaysUntilDue calculates days remaining until due date.
func (d *Deadline) DaysUntilDue() int {
	now := time.Time(common.NewTimestamp())
	duration := d.DueDate.Sub(now)
	return int(duration.Hours() / 24)
}

//Personal.AI order the ending
