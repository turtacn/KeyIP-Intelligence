package lifecycle

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// DeadlineType defines the type of deadline.
type DeadlineType string

const (
	DeadlineTypeFilingResponse      DeadlineType = "filing_response"
	DeadlineTypeAnnuityPayment      DeadlineType = "annuity_payment"
	DeadlineTypePCTNationalPhase    DeadlineType = "pct_national_phase"
	DeadlineTypePriorityClaimFiling DeadlineType = "priority_claim_filing"
	DeadlineTypeOppositionFiling    DeadlineType = "opposition_filing"
	DeadlineTypeRenewal             DeadlineType = "renewal"
	DeadlineTypeCustom              DeadlineType = "custom"
)

// DeadlineUrgency defines the urgency level.
type DeadlineUrgency string

const (
	UrgencyCritical DeadlineUrgency = "critical" // < 7 days
	UrgencyHigh     DeadlineUrgency = "high"     // < 30 days
	UrgencyMedium   DeadlineUrgency = "medium"   // < 90 days
	UrgencyLow      DeadlineUrgency = "low"      // > 90 days
)

// DeadlineStatus defines status for legacy compatibility.
type DeadlineStatus string

const (
	DeadlineStatusActive    DeadlineStatus = "active"
	DeadlineStatusCompleted DeadlineStatus = "completed"
	DeadlineStatusMissed    DeadlineStatus = "missed"
	DeadlineStatusExtended  DeadlineStatus = "extended"
	DeadlineStatusWaived    DeadlineStatus = "waived"
)

// Deadline represents a critical date or task.
// Updated to include fields for legacy repo compatibility.
type Deadline struct {
	ID                string                 `json:"id"`
	PatentID          string                 `json:"patent_id"`
	Type              DeadlineType           `json:"type"`
	Title             string                 `json:"title"`
	Description       string                 `json:"description"`
	DueDate           time.Time              `json:"due_date"`
	OriginalDueDate   time.Time              `json:"original_due_date"` // Added
	ReminderDates     []time.Time            `json:"reminder_dates"`
	Urgency           DeadlineUrgency        `json:"urgency"`
	IsCompleted       bool                   `json:"is_completed"`
	Status            DeadlineStatus         `json:"status"` // Added for compatibility
	Priority          string                 `json:"priority"` // Added: e.g. "critical"
	AssigneeID        *string                `json:"assignee_id"` // Added
	CompletedAt       *time.Time             `json:"completed_at"`
	CompletedBy       string                 `json:"completed_by"`
	JurisdictionCode  string                 `json:"jurisdiction_code"`
	ExtensionAvailable bool                  `json:"extension_available"`
	ExtensionCount    int                    `json:"extension_count"` // Added
	ExtensionHistory  []map[string]interface{} `json:"extension_history"` // Added
	MaxExtensionDays  int                    `json:"max_extension_days"`
	ExtendedDueDate   *time.Time             `json:"extended_due_date"`
	ReminderConfig    map[string]interface{} `json:"reminder_config"` // Added
	LastReminderAt    *time.Time             `json:"last_reminder_at"` // Added
	Notes             string                 `json:"notes"`
	Metadata          map[string]interface{} `json:"metadata"` // Added
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
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

// DeadlineReminder represents a notification for a deadline.
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

// DeadlineService defines the interface for deadline management.
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

// NewDeadline creates a new Deadline entity.
func NewDeadline(patentID string, deadlineType DeadlineType, title string, dueDate time.Time) (*Deadline, error) {
	if patentID == "" {
		return nil, errors.New("patent ID cannot be empty")
	}
	if title == "" {
		return nil, errors.New("title cannot be empty")
	}

	today := time.Now().Truncate(24 * time.Hour)
	if dueDate.Before(today) {
		return nil, errors.New("due date cannot be in the past")
	}

	d := &Deadline{
		ID:              uuid.New().String(),
		PatentID:        patentID,
		Type:            deadlineType,
		Title:           title,
		DueDate:         dueDate,
		OriginalDueDate: dueDate,
		Status:          DeadlineStatusActive,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	d.ReminderDates = GenerateDefaultReminderDates(dueDate, time.Now())
	d.Urgency = d.CalculateUrgency(time.Now())
	d.Priority = string(d.Urgency)

	return d, nil
}

func (d *Deadline) CalculateUrgency(asOf time.Time) DeadlineUrgency {
	effectiveDue := d.EffectiveDueDate()
	diff := effectiveDue.Sub(asOf)
	days := int(diff.Hours() / 24)

	if days < 7 {
		return UrgencyCritical
	}
	if days < 30 {
		return UrgencyHigh
	}
	if days < 90 {
		return UrgencyMedium
	}
	return UrgencyLow
}

func (d *Deadline) Complete(completedBy string) error {
	if d.IsCompleted {
		return apperrors.NewValidation("deadline already completed")
	}
	d.IsCompleted = true
	d.Status = DeadlineStatusCompleted
	now := time.Now().UTC()
	d.CompletedAt = &now
	d.CompletedBy = completedBy
	d.UpdatedAt = now
	return nil
}

func (d *Deadline) Extend(extensionDays int) error {
	if !d.ExtensionAvailable {
		return apperrors.NewValidation("deadline cannot be extended")
	}
	if extensionDays > d.MaxExtensionDays {
		return apperrors.NewValidation("extension days exceed maximum allowed: %d", d.MaxExtensionDays)
	}

	newDue := d.DueDate.AddDate(0, 0, extensionDays)
	d.ExtendedDueDate = &newDue

	// Regenerate reminders based on new date
	d.ReminderDates = GenerateDefaultReminderDates(newDue, time.Now())
	d.Status = DeadlineStatusExtended
	d.ExtensionCount++
	d.UpdatedAt = time.Now().UTC()
	return nil
}

func (d *Deadline) IsOverdue(asOf time.Time) bool {
	if d.IsCompleted {
		return false
	}
	return d.EffectiveDueDate().Before(asOf)
}

func (d *Deadline) DaysUntilDue(asOf time.Time) int {
	diff := d.EffectiveDueDate().Sub(asOf)
	return int(diff.Hours() / 24)
}

func (d *Deadline) EffectiveDueDate() time.Time {
	if d.ExtendedDueDate != nil {
		return *d.ExtendedDueDate
	}
	return d.DueDate
}

func GenerateDefaultReminderDates(dueDate time.Time, asOf time.Time) []time.Time {
	offsets := []int{60, 30, 14, 7} // Days before due
	var dates []time.Time

	for _, days := range offsets {
		remDate := dueDate.AddDate(0, 0, -days)
		if remDate.After(asOf) {
			dates = append(dates, remDate)
		}
	}
	// Sort ascending
	sort.Slice(dates, func(i, j int) bool {
		return dates[i].Before(dates[j])
	})
	return dates
}

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

// Service Implementation methods

func (s *deadlineServiceImpl) CreateDeadline(ctx context.Context, patentID string, deadlineType DeadlineType, title string, dueDate time.Time) (*Deadline, error) {
	d, err := NewDeadline(patentID, deadlineType, title, dueDate)
	if err != nil {
		return nil, apperrors.NewValidation(err.Error())
	}
	if err := s.repo.SaveDeadline(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *deadlineServiceImpl) CompleteDeadline(ctx context.Context, deadlineID, completedBy string) error {
	d, err := s.repo.GetDeadlineByID(ctx, deadlineID)
	if err != nil {
		return err
	}
	if d == nil {
		return apperrors.NewNotFound("deadline not found: %s", deadlineID)
	}
	if err := d.Complete(completedBy); err != nil {
		return err
	}
	return s.repo.SaveDeadline(ctx, d)
}

func (s *deadlineServiceImpl) ExtendDeadline(ctx context.Context, deadlineID string, extensionDays int) error {
	d, err := s.repo.GetDeadlineByID(ctx, deadlineID)
	if err != nil {
		return err
	}
	if d == nil {
		return apperrors.NewNotFound("deadline not found: %s", deadlineID)
	}
	if err := d.Extend(extensionDays); err != nil {
		return err
	}
	return s.repo.SaveDeadline(ctx, d)
}

func (s *deadlineServiceImpl) GetCalendar(ctx context.Context, ownerID string, from, to time.Time) (*DeadlineCalendar, error) {
	if to.Sub(from) > 365*24*time.Hour {
		to = from.AddDate(1, 0, 0) // Cap at 1 year
	}

	deadlines, err := s.repo.GetDeadlinesByOwnerID(ctx, ownerID, from, to)
	if err != nil {
		return nil, err
	}

	cal := &DeadlineCalendar{
		OwnerID:     ownerID,
		Deadlines:   deadlines,
		GeneratedAt: time.Now().UTC(),
	}

	now := time.Now().UTC()
	week := now.AddDate(0, 0, 7)
	month := now.AddDate(0, 1, 0)

	for _, d := range deadlines {
		if d.IsOverdue(now) {
			cal.OverdueCount++
		}
		if d.Urgency == UrgencyCritical {
			cal.CriticalCount++
		} else if d.Urgency == UrgencyHigh {
			cal.HighCount++
		}

		eff := d.EffectiveDueDate()
		if eff.After(now) && eff.Before(week) {
			cal.UpcomingWeek = append(cal.UpcomingWeek, d)
		}
		if eff.After(now) && eff.Before(month) {
			cal.UpcomingMonth = append(cal.UpcomingMonth, d)
		}
	}

	// Sort lists
	sortDeadlines := func(ds []*Deadline) {
		sort.Slice(ds, func(i, j int) bool {
			return ds[i].EffectiveDueDate().Before(ds[j].EffectiveDueDate())
		})
	}
	sortDeadlines(cal.Deadlines)
	sortDeadlines(cal.UpcomingWeek)
	sortDeadlines(cal.UpcomingMonth)

	return cal, nil
}

func (s *deadlineServiceImpl) GetOverdueDeadlines(ctx context.Context, ownerID string) ([]*Deadline, error) {
	return s.repo.GetOverdueDeadlines(ctx, ownerID, time.Now().UTC())
}

func (s *deadlineServiceImpl) GetUpcomingDeadlines(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error) {
	return s.repo.GetUpcomingDeadlines(ctx, ownerID, withinDays)
}

func (s *deadlineServiceImpl) RefreshUrgencies(ctx context.Context, ownerID string) error {
	deadlines, err := s.repo.GetUpcomingDeadlines(ctx, ownerID, 3650)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, d := range deadlines {
		newUrgency := d.CalculateUrgency(now)
		if newUrgency != d.Urgency {
			d.Urgency = newUrgency
			d.UpdatedAt = now
			if err := s.repo.SaveDeadline(ctx, d); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *deadlineServiceImpl) GenerateReminderBatch(ctx context.Context, asOf time.Time) ([]*DeadlineReminder, error) {
	deadlines, err := s.repo.GetPendingDeadlineReminders(ctx, asOf)
	if err != nil {
		return nil, err
	}

	var reminders []*DeadlineReminder
	for _, d := range deadlines {
		if d.IsCompleted {
			continue
		}

		match := false
		for _, rd := range d.ReminderDates {
			// Check if same day
			if rd.Year() == asOf.Year() && rd.Month() == asOf.Month() && rd.Day() == asOf.Day() {
				match = true
				break
			}
		}

		if match {
			reminders = append(reminders, &DeadlineReminder{
				DeadlineID:   d.ID,
				PatentID:     d.PatentID,
				Title:        d.Title,
				DueDate:      d.EffectiveDueDate(),
				Urgency:      d.Urgency,
				DaysUntilDue: d.DaysUntilDue(asOf),
				ReminderType: "email", // Default
			})
		}
	}
	return reminders, nil
}

func (s *deadlineServiceImpl) AddCustomDeadline(ctx context.Context, patentID, title, description string, dueDate time.Time) (*Deadline, error) {
	d, err := NewDeadline(patentID, DeadlineTypeCustom, title, dueDate)
	if err != nil {
		return nil, apperrors.NewValidation(err.Error())
	}
	d.Description = description
	if err := s.repo.SaveDeadline(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

//Personal.AI order the ending
