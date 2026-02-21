package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// DeadlineType defines the type of a deadline.
type DeadlineType string

const (
	DeadlineTypeFilingResponse      DeadlineType = "FilingResponse"
	DeadlineTypeAnnuityPayment      DeadlineType = "AnnuityPayment"
	DeadlineTypePCTNationalPhase    DeadlineType = "PCTNationalPhase"
	DeadlineTypePriorityClaimFiling DeadlineType = "PriorityClaimFiling"
	DeadlineTypeOppositionFiling    DeadlineType = "OppositionFiling"
	DeadlineTypeRenewal             DeadlineType = "Renewal"
	DeadlineTypeCustom              DeadlineType = "Custom"
)

// DeadlineUrgency defines the urgency level of a deadline.
type DeadlineUrgency string

const (
	UrgencyCritical DeadlineUrgency = "Critical"
	UrgencyHigh     DeadlineUrgency = "High"
	UrgencyMedium   DeadlineUrgency = "Medium"
	UrgencyLow      DeadlineUrgency = "Low"
)

// Deadline represents a single deadline in the patent lifecycle.
type Deadline struct {
	ID                 string          `json:"id"`
	PatentID           string          `json:"patent_id"`
	Type               DeadlineType    `json:"type"`
	Title              string          `json:"title"`
	Description        string          `json:"description"`
	DueDate            time.Time       `json:"due_date"`
	ReminderDates      []time.Time     `json:"reminder_dates"`
	Urgency            DeadlineUrgency `json:"urgency"`
	IsCompleted        bool            `json:"is_completed"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty"`
	CompletedBy        string          `json:"completed_by,omitempty"`
	JurisdictionCode   string          `json:"jurisdiction_code"`
	ExtensionAvailable bool            `json:"extension_available"`
	MaxExtensionDays   int             `json:"max_extension_days"`
	ExtendedDueDate    *time.Time      `json:"extended_due_date,omitempty"`
	Notes              string          `json:"notes"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// NewDeadline creates a new Deadline.
func NewDeadline(patentID string, deadlineType DeadlineType, title string, dueDate time.Time) (*Deadline, error) {
	if patentID == "" {
		return nil, errors.InvalidParam("patent ID cannot be empty")
	}
	if title == "" {
		return nil, errors.InvalidParam("title cannot be empty")
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dueDay := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, time.UTC)

	if dueDay.Before(today) {
		return nil, errors.InvalidParam("due date cannot be in the past")
	}

	d := &Deadline{
		ID:            uuid.New().String(),
		PatentID:      patentID,
		Type:          deadlineType,
		Title:         title,
		DueDate:       dueDate,
		IsCompleted:   false,
		CreatedAt:     now,
		UpdatedAt:     now,
		ReminderDates: GenerateDefaultReminderDates(dueDate, now),
	}
	d.Urgency = d.CalculateUrgency(now)
	return d, nil
}

// CalculateUrgency computes the urgency based on the days remaining until the deadline.
func (d *Deadline) CalculateUrgency(asOf time.Time) DeadlineUrgency {
	days := d.DaysUntilDue(asOf)
	switch {
	case days <= 7:
		return UrgencyCritical
	case days <= 30:
		return UrgencyHigh
	case days <= 90:
		return UrgencyMedium
	default:
		return UrgencyLow
	}
}

// Complete marks the deadline as completed.
func (d *Deadline) Complete(completedBy string) error {
	if d.IsCompleted {
		return errors.InvalidState("deadline already completed")
	}
	now := time.Now().UTC()
	d.IsCompleted = true
	d.CompletedAt = &now
	d.CompletedBy = completedBy
	d.UpdatedAt = now
	return nil
}

// Extend extends the deadline if allowed.
func (d *Deadline) Extend(extensionDays int) error {
	if d.IsCompleted {
		return errors.InvalidState("cannot extend a completed deadline")
	}
	if !d.ExtensionAvailable {
		return errors.InvalidState("extension not available for this deadline")
	}
	if extensionDays > d.MaxExtensionDays {
		return errors.InvalidParam(fmt.Sprintf("extension days %d exceeds maximum %d", extensionDays, d.MaxExtensionDays))
	}

	newDue := d.DueDate.AddDate(0, 0, extensionDays)
	d.ExtendedDueDate = &newDue
	d.ReminderDates = GenerateDefaultReminderDates(newDue, time.Now().UTC())
	d.Urgency = d.CalculateUrgency(time.Now().UTC())
	d.UpdatedAt = time.Now().UTC()
	return nil
}

// IsOverdue checks if the deadline is overdue.
func (d *Deadline) IsOverdue(asOf time.Time) bool {
	if d.IsCompleted {
		return false
	}
	return d.EffectiveDueDate().Before(asOf)
}

// DaysUntilDue calculates the number of days until the deadline.
func (d *Deadline) DaysUntilDue(asOf time.Time) int {
	due := d.EffectiveDueDate()

	// Start of day normalization
	t1 := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	t2 := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.UTC)

	return int(t2.Sub(t1).Hours() / 24)
}

// EffectiveDueDate returns the current active due date.
func (d *Deadline) EffectiveDueDate() time.Time {
	if d.ExtendedDueDate != nil {
		return *d.ExtendedDueDate
	}
	return d.DueDate
}

// DeadlineCalendar represents a calendar view of deadlines.
type DeadlineCalendar struct {
	OwnerID       string      `json:"owner_id"`
	Deadlines     []*Deadline `json:"deadlines"`
	OverdueCount  int         `json:"overdue_count"`
	CriticalCount int         `json:"critical_count"`
	HighCount     int         `json:"high_count"`
	UpcomingWeek  []*Deadline `json:"upcoming_week"`
	UpcomingMonth []*Deadline `json:"upcoming_month"`
	GeneratedAt   time.Time   `json:"generated_at"`
}

// DeadlineService defines the domain service for managing deadlines.
type DeadlineService interface {
	CreateDeadline(ctx context.Context, patentID string, deadlineType DeadlineType, title string, dueDate time.Time) (*Deadline, error)
	CompleteDeadline(ctx context.Context, deadlineID, completedBy string) error
	ExtendDeadline(ctx context.Context, deadlineID string, extensionDays int) error
	GetCalendar(ctx context.Context, ownerID string, from, to time.Time) (*DeadlineCalendar, error)
	GetOverdueDeadlines(ctx context.Context, ownerID string) ([]*Deadline, error)
	GetUpcomingDeadlines(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error)
	RefreshUrgencies(ctx context.Context, ownerID string) error
	GenerateReminderBatch(ctx context.Context, asOf time.Time) ([]*DeadlineReminder, error)
	AddCustomDeadline(ctx context.Context, patentID, title, description string, dueDate time.Time) (*Deadline, error)
}

type deadlineServiceImpl struct {
	repo DeadlineRepository
}

// NewDeadlineService creates a new DeadlineService.
func NewDeadlineService(repo DeadlineRepository) DeadlineService {
	return &deadlineServiceImpl{repo: repo}
}

func (s *deadlineServiceImpl) CreateDeadline(ctx context.Context, patentID string, deadlineType DeadlineType, title string, dueDate time.Time) (*Deadline, error) {
	d, err := NewDeadline(patentID, deadlineType, title, dueDate)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *deadlineServiceImpl) CompleteDeadline(ctx context.Context, deadlineID, completedBy string) error {
	d, err := s.repo.FindByID(ctx, deadlineID)
	if err != nil {
		return err
	}
	if err := d.Complete(completedBy); err != nil {
		return err
	}
	return s.repo.Save(ctx, d)
}

func (s *deadlineServiceImpl) ExtendDeadline(ctx context.Context, deadlineID string, extensionDays int) error {
	d, err := s.repo.FindByID(ctx, deadlineID)
	if err != nil {
		return err
	}
	if err := d.Extend(extensionDays); err != nil {
		return err
	}
	return s.repo.Save(ctx, d)
}

func (s *deadlineServiceImpl) GetCalendar(ctx context.Context, ownerID string, from, to time.Time) (*DeadlineCalendar, error) {
	deadlines, err := s.repo.FindByOwnerID(ctx, ownerID, from, to)
	if err != nil {
		return nil, err
	}

	cal := &DeadlineCalendar{
		OwnerID:     ownerID,
		Deadlines:   deadlines,
		GeneratedAt: time.Now().UTC(),
	}

	now := time.Now().UTC()
	weekCutoff := now.AddDate(0, 0, 7)
	monthCutoff := now.AddDate(0, 0, 30)

	for _, d := range deadlines {
		if d.IsOverdue(now) {
			cal.OverdueCount++
		}
		switch d.Urgency {
		case UrgencyCritical:
			cal.CriticalCount++
		case UrgencyHigh:
			cal.HighCount++
		}

		if !d.IsCompleted {
			due := d.EffectiveDueDate()
			if (due.After(now) || due.Equal(now)) && due.Before(weekCutoff) {
				cal.UpcomingWeek = append(cal.UpcomingWeek, d)
			}
			if (due.After(now) || due.Equal(now)) && due.Before(monthCutoff) {
				cal.UpcomingMonth = append(cal.UpcomingMonth, d)
			}
		}
	}

	// Sort by DueDate ascending
	sortDeadlines := func(ds []*Deadline) {
		for i := 0; i < len(ds); i++ {
			for j := i + 1; j < len(ds); j++ {
				if ds[i].EffectiveDueDate().After(ds[j].EffectiveDueDate()) {
					ds[i], ds[j] = ds[j], ds[i]
				}
			}
		}
	}
	sortDeadlines(cal.Deadlines)
	sortDeadlines(cal.UpcomingWeek)
	sortDeadlines(cal.UpcomingMonth)

	return cal, nil
}

func (s *deadlineServiceImpl) GetOverdueDeadlines(ctx context.Context, ownerID string) ([]*Deadline, error) {
	return s.repo.FindOverdue(ctx, ownerID, time.Now().UTC())
}

func (s *deadlineServiceImpl) GetUpcomingDeadlines(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error) {
	return s.repo.FindUpcoming(ctx, ownerID, withinDays)
}

func (s *deadlineServiceImpl) RefreshUrgencies(ctx context.Context, ownerID string) error {
	deadlines, err := s.repo.FindUpcoming(ctx, ownerID, 365) // Refresh for next year
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, d := range deadlines {
		if !d.IsCompleted {
			newUrgency := d.CalculateUrgency(now)
			if newUrgency != d.Urgency {
				d.Urgency = newUrgency
				d.UpdatedAt = now
				if err := s.repo.Save(ctx, d); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *deadlineServiceImpl) GenerateReminderBatch(ctx context.Context, asOf time.Time) ([]*DeadlineReminder, error) {
	deadlines, err := s.repo.FindPendingReminders(ctx, asOf)
	if err != nil {
		return nil, err
	}

	var reminders []*DeadlineReminder
	for _, d := range deadlines {
		// Matching logic: check if asOf matches any of the ReminderDates
		match := false
		asOfDate := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
		for _, rd := range d.ReminderDates {
			rdDate := time.Date(rd.Year(), rd.Month(), rd.Day(), 0, 0, 0, 0, time.UTC)
			if asOfDate.Equal(rdDate) {
				match = true
				break
			}
		}

		if match && !d.IsCompleted {
			reminders = append(reminders, &DeadlineReminder{
				DeadlineID:   d.ID,
				PatentID:     d.PatentID,
				Title:        d.Title,
				DueDate:      d.EffectiveDueDate(),
				Urgency:      d.Urgency,
				DaysUntilDue: d.DaysUntilDue(asOf),
			})
		}
	}
	return reminders, nil
}

func (s *deadlineServiceImpl) AddCustomDeadline(ctx context.Context, patentID, title, description string, dueDate time.Time) (*Deadline, error) {
	d, err := NewDeadline(patentID, DeadlineTypeCustom, title, dueDate)
	if err != nil {
		return nil, err
	}
	d.Description = description
	if err := s.repo.Save(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// DeadlineReminder represents a reminder to be sent.
type DeadlineReminder struct {
	DeadlineID   string          `json:"deadline_id"`
	PatentID     string          `json:"patent_id"`
	Title        string          `json:"title"`
	DueDate      time.Time       `json:"due_date"`
	Urgency      DeadlineUrgency `json:"urgency"`
	DaysUntilDue int             `json:"days_until_due"`
	RecipientIDs []string        `json:"recipient_ids"`
	ReminderType string          `json:"reminder_type"`
}

// GenerateDefaultReminderDates creates standard reminder dates.
func GenerateDefaultReminderDates(dueDate time.Time, asOf time.Time) []time.Time {
	days := []int{7, 14, 30, 60}
	var dates []time.Time
	for _, d := range days {
		remDate := dueDate.AddDate(0, 0, -d)
		if remDate.After(asOf) {
			dates = append(dates, remDate)
		}
	}
	// Sort ascending? The requirement says按时间升序排列.
	// Since we go 7, 14, 30, 60, adding them results in 60 being earliest.
	// Wait, dueDate - 60 is earlier than dueDate - 7.
	// So let's do it in reverse.
	var sortedDates []time.Time
	for i := len(days) - 1; i >= 0; i-- {
		remDate := dueDate.AddDate(0, 0, -days[i])
		if remDate.After(asOf) || time.Date(remDate.Year(), remDate.Month(), remDate.Day(), 0, 0, 0, 0, time.UTC).Equal(time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)) {
			sortedDates = append(sortedDates, remDate)
		}
	}
	return sortedDates
}

// CalculateJurisdictionExtension returns the extension rules for a jurisdiction.
func CalculateJurisdictionExtension(jurisdictionCode string, deadlineType DeadlineType) (bool, int) {
	switch jurisdictionCode {
	case "CN":
		if deadlineType == DeadlineTypeFilingResponse {
			return true, 60
		}
	case "US":
		if deadlineType == DeadlineTypeFilingResponse {
			return true, 180
		}
	case "EP":
		if deadlineType == DeadlineTypeFilingResponse {
			return true, 60
		}
	}
	return false, 0
}

//Personal.AI order the ending
