package lifecycle

import (
	"time"

	"github.com/google/uuid"
)

// Jurisdiction represents a legal jurisdiction (e.g., CN, US).
type Jurisdiction string

const (
	JurisdictionCN Jurisdiction = "CN"
	JurisdictionUS Jurisdiction = "US"
	JurisdictionEP Jurisdiction = "EP"
	JurisdictionJP Jurisdiction = "JP"
	JurisdictionKR Jurisdiction = "KR"
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

// PaymentRecord represents a persisted payment entry.
type PaymentRecord struct {
	ID           string       `json:"id"`
	PatentID     string       `json:"patent_id"`
	Jurisdiction Jurisdiction `json:"jurisdiction"`
	YearNumber   int          `json:"year_number"`
	Amount       float64      `json:"amount"`
	Currency     string       `json:"currency"`
	PaidDate     time.Time    `json:"paid_date"`
	PaymentRef   string       `json:"payment_ref"`
	PaidBy       string       `json:"paid_by"`
	Notes        string       `json:"notes"`
	RecordedAt   time.Time    `json:"recorded_at"`
}

// PaymentQuery defines the criteria for querying payments.
type PaymentQuery struct {
	PatentID     string       `json:"patent_id,omitempty"`
	PortfolioID  string       `json:"portfolio_id,omitempty"`
	Jurisdiction Jurisdiction `json:"jurisdiction,omitempty"`
	StartDate    time.Time    `json:"start_date,omitempty"`
	EndDate      time.Time    `json:"end_date,omitempty"`
	Offset       int          `json:"offset,omitempty"`
	Limit        int          `json:"limit,omitempty"`
}

// SubscriptionEntity represents a subscription for status updates.
type SubscriptionEntity struct {
	ID            string       `json:"id"`
	PatentIDs     []string     `json:"patent_ids,omitempty"`
	PortfolioID   string       `json:"portfolio_id,omitempty"`
	StatusFilters []string     `json:"status_filters,omitempty"`
	Channels      []string     `json:"channels"`
	Recipient     string       `json:"recipient"`
	Active        bool         `json:"active"`
	CreatedAt     time.Time    `json:"created_at"`
}

// CustomEvent represents a user-defined calendar event.
type CustomEvent struct {
	ID           string            `json:"id"`
	PatentID     string            `json:"patent_id"`
	PatentNumber string            `json:"patent_number"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	EventType    string            `json:"event_type"`
	Jurisdiction Jurisdiction      `json:"jurisdiction"`
	EventDate    time.Time         `json:"event_date"`
	DueDate      time.Time         `json:"due_date"`
	Priority     string            `json:"priority"`
	Status       string            `json:"status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// LegalStatusEntity represents the persistent legal status of a patent.
type LegalStatusEntity struct {
	PatentID                string       `json:"patent_id"`
	Jurisdiction            string       `json:"jurisdiction"`
	Status                  string       `json:"status"`
	PreviousStatus          string       `json:"previous_status"`
	EffectiveDate           time.Time    `json:"effective_date"`
	NextAction              string       `json:"next_action,omitempty"`
	NextDeadline            *time.Time   `json:"next_deadline,omitempty"`
	RemoteStatus            string       `json:"remote_status"`
	LastSyncAt              *time.Time   `json:"last_sync_at"`
	ConsecutiveSyncFailures int          `json:"consecutive_sync_failures"`
	RawData                 map[string]any `json:"raw_data,omitempty"`
}

// StatusHistoryEntity represents a historical status change.
type StatusHistoryEntity struct {
	EventID     string    `json:"event_id"`
	PatentID    string    `json:"patent_id"`
	FromStatus  string    `json:"from_status"`
	ToStatus    string    `json:"to_status"`
	EventDate   time.Time `json:"event_date"`
	Source      string    `json:"source"`
	Description string    `json:"description"`
}

// AnnuityCalcResult holds the result of domain-level annuity calculation.
type AnnuityCalcResult struct {
	Fee            float64
	YearNumber     int
	DueDate        time.Time
	GracePeriodEnd time.Time
	Status         string
}

// ScheduleEntry is a single entry in an annuity schedule.
type ScheduleEntry struct {
	YearNumber     int
	Fee            float64
	DueDate        time.Time
	GracePeriodEnd time.Time
	Status         string
}

// RemoteStatusResult represents the status fetched from a remote source.
type RemoteStatusResult struct {
	Status        string
	EffectiveDate time.Time
	NextAction    string
	Source        string
	Jurisdiction  string
}


//DaysUntilDue calculates days remaining until due date
func (d *Deadline) DaysUntilDue() int {
	now := time.Now().UTC()
	duration := d.DueDate.Sub(now)
	return int(duration.Hours() / 24)
}

//Personal.AI order the ending
