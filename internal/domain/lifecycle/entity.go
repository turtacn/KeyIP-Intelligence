package lifecycle

import (
	"time"

	"github.com/google/uuid"
)

// AnnuityStatus represents the status of an annuity payment.
type AnnuityStatus string

const (
	AnnuityStatusUpcoming    AnnuityStatus = "upcoming"
	AnnuityStatusDue         AnnuityStatus = "due"
	AnnuityStatusOverdue     AnnuityStatus = "overdue"
	AnnuityStatusPaid        AnnuityStatus = "paid"
	AnnuityStatusGracePeriod AnnuityStatus = "grace_period"
	AnnuityStatusWaived      AnnuityStatus = "waived"
	AnnuityStatusAbandoned   AnnuityStatus = "abandoned"
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

// EventType represents the type of lifecycle event.
type EventType string

const (
	EventTypeFiling             EventType = "filing"
	EventTypePublication        EventType = "publication"
	EventTypeExaminationRequest EventType = "examination_request"
	EventTypeOfficeAction       EventType = "office_action"
	EventTypeResponseFiled      EventType = "response_filed"
	EventTypeGrant              EventType = "grant"
	EventTypeAnnuityPayment     EventType = "annuity_payment"
	EventTypeAnnuityMissed      EventType = "annuity_missed"
	EventTypeRenewal            EventType = "renewal"
	EventTypeAssignment         EventType = "assignment"
	EventTypeLicense            EventType = "license"
	EventTypeOpposition         EventType = "opposition"
	EventTypeInvalidation       EventType = "invalidation"
	EventTypeExpiry             EventType = "expiry"
	EventTypeRestoration        EventType = "restoration"
	EventTypeAbandonment        EventType = "abandonment"
	EventTypeStatusChange       EventType = "status_change"
	EventTypeCustom             EventType = "custom"
)

// Annuity represents a yearly fee.
type Annuity struct {
	ID               uuid.UUID      `json:"id"`
	PatentID         uuid.UUID      `json:"patent_id"`
	YearNumber       int            `json:"year_number"`
	DueDate          time.Time      `json:"due_date"`
	GraceDeadline    *time.Time     `json:"grace_deadline,omitempty"`
	Status           AnnuityStatus  `json:"status"`
	Amount           *int64         `json:"amount,omitempty"`
	Currency         string         `json:"currency"`
	PaidAmount       *int64         `json:"paid_amount,omitempty"`
	PaidDate         *time.Time     `json:"paid_date,omitempty"`
	PaymentReference string         `json:"payment_reference,omitempty"`
	AgentName        string         `json:"agent_name,omitempty"`
	AgentReference   string         `json:"agent_reference,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	ReminderSentAt   *time.Time     `json:"reminder_sent_at,omitempty"`
	ReminderCount    int            `json:"reminder_count"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// Deadline represents a critical date.
type Deadline struct {
	ID              uuid.UUID      `json:"id"`
	PatentID        uuid.UUID      `json:"patent_id"`
	DeadlineType    string         `json:"deadline_type"`
	Title           string         `json:"title"`
	Description     string         `json:"description,omitempty"`
	DueDate         time.Time      `json:"due_date"`
	OriginalDueDate time.Time      `json:"original_due_date"`
	Status          DeadlineStatus `json:"status"`
	Priority        string         `json:"priority"`
	AssigneeID      *uuid.UUID     `json:"assignee_id,omitempty"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	CompletedBy     *uuid.UUID     `json:"completed_by,omitempty"`
	ExtensionCount  int            `json:"extension_count"`
	ExtensionHistory []map[string]any `json:"extension_history,omitempty"`
	ReminderConfig  map[string]any `json:"reminder_config,omitempty"`
	LastReminderAt  *time.Time     `json:"last_reminder_at,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// LifecycleEvent represents a historical event.
type LifecycleEvent struct {
	ID                uuid.UUID      `json:"id"`
	PatentID          uuid.UUID      `json:"patent_id"`
	EventType         EventType      `json:"event_type"`
	EventDate         time.Time      `json:"event_date"`
	Title             string         `json:"title"`
	Description       string         `json:"description,omitempty"`
	ActorID           *uuid.UUID     `json:"actor_id,omitempty"`
	ActorName         string         `json:"actor_name,omitempty"`
	RelatedDeadlineID *uuid.UUID     `json:"related_deadline_id,omitempty"`
	RelatedAnnuityID  *uuid.UUID     `json:"related_annuity_id,omitempty"`
	BeforeState       map[string]any `json:"before_state,omitempty"`
	AfterState        map[string]any `json:"after_state,omitempty"`
	Attachments       []map[string]string `json:"attachments,omitempty"`
	Source            string         `json:"source"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

// CostRecord represents a financial transaction.
type CostRecord struct {
	ID               uuid.UUID      `json:"id"`
	PatentID         uuid.UUID      `json:"patent_id"`
	CostType         string         `json:"cost_type"`
	Amount           int64          `json:"amount"`
	Currency         string         `json:"currency"`
	AmountUSD        *int64         `json:"amount_usd,omitempty"`
	ExchangeRate     *float64       `json:"exchange_rate,omitempty"`
	IncurredDate     time.Time      `json:"incurred_date"`
	Description      string         `json:"description,omitempty"`
	InvoiceReference string         `json:"invoice_reference,omitempty"`
	RelatedAnnuityID *uuid.UUID     `json:"related_annuity_id,omitempty"`
	RelatedEventID   *uuid.UUID     `json:"related_event_id,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

// CostSummary aggregates costs by type.
type CostSummary struct {
	TotalCosts map[string]int64 `json:"total_costs"`
}

// PortfolioCostSummary aggregates costs by portfolio.
type PortfolioCostSummary struct {
	TotalCosts map[string]int64 `json:"total_costs"`
}

// DashboardStats contains lifecycle metrics.
type DashboardStats struct {
	UpcomingAnnuities int64 `json:"upcoming_annuities"`
	OverdueAnnuities  int64 `json:"overdue_annuities"`
	ActiveDeadlines   int64 `json:"active_deadlines"`
	RecentEvents      int64 `json:"recent_events"`
	TotalCostYTD      int64 `json:"total_cost_ytd"`
}

//DaysUntilDue calculates days remaining until due date
func (d *Deadline) DaysUntilDue() int {
	now := time.Now().UTC()
	duration := d.DueDate.Sub(now)
	return int(duration.Hours() / 24)
}

//Personal.AI order the ending
