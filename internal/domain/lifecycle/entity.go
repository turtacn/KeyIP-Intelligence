package lifecycle

import (
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
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

// LifecycleEvent represents a historical event.
type LifecycleEvent struct {
	ID                string              `json:"id"`
	PatentID          string              `json:"patent_id"`
	EventType         EventType           `json:"event_type"`
	EventDate         time.Time           `json:"event_date"`
	Title             string              `json:"title"`
	Description       string              `json:"description,omitempty"`
	ActorID           *string             `json:"actor_id,omitempty"`
	ActorName         string              `json:"actor_name,omitempty"`
	RelatedDeadlineID *string             `json:"related_deadline_id,omitempty"`
	RelatedAnnuityID  *string             `json:"related_annuity_id,omitempty"`
	BeforeState       map[string]any      `json:"before_state,omitempty"`
	AfterState        map[string]any      `json:"after_state,omitempty"`
	Attachments       []map[string]string `json:"attachments,omitempty"`
	Source            string              `json:"source"`
	Metadata          map[string]any      `json:"metadata,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
}

// CostRecord represents a financial transaction.
type CostRecord struct {
	ID               string         `json:"id"`
	PatentID         string         `json:"patent_id"`
	CostType         string         `json:"cost_type"`
	Amount           int64          `json:"amount"` // Money in minor units
	Currency         string         `json:"currency"`
	AmountUSD        *int64         `json:"amount_usd,omitempty"`
	ExchangeRate     *float64       `json:"exchange_rate,omitempty"`
	IncurredDate     time.Time      `json:"incurred_date"`
	Description      string         `json:"description,omitempty"`
	InvoiceReference string         `json:"invoice_reference,omitempty"`
	RelatedAnnuityID *string        `json:"related_annuity_id,omitempty"`
	RelatedEventID   *string        `json:"related_event_id,omitempty"`
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
	ID            string    `json:"id"`
	PatentIDs     []string  `json:"patent_ids,omitempty"`
	PortfolioID   string    `json:"portfolio_id,omitempty"`
	StatusFilters []string  `json:"status_filters,omitempty"`
	Channels      []string  `json:"channels"`
	Recipient     string    `json:"recipient"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
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
	PatentID                string         `json:"patent_id"`
	Jurisdiction            string         `json:"jurisdiction"`
	Status                  string         `json:"status"`
	PreviousStatus          string         `json:"previous_status"`
	EffectiveDate           time.Time      `json:"effective_date"`
	NextAction              string         `json:"next_action,omitempty"`
	NextDeadline            *time.Time     `json:"next_deadline,omitempty"`
	RemoteStatus            string         `json:"remote_status"`
	LastSyncAt              *time.Time     `json:"last_sync_at"`
	ConsecutiveSyncFailures int            `json:"consecutive_sync_failures"`
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
	Fee            int64 // Money in minor units
	Currency       string
	YearNumber     int
	DueDate        time.Time
	GracePeriodEnd time.Time
	Status         string
}

// ScheduleEntry is a single entry in an annuity schedule.
type ScheduleEntry struct {
	YearNumber     int
	Fee            int64 // Money in minor units
	Currency       string
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

// RemainingLifeYears calculates remaining life in years.
func RemainingLifeYears(expiryDate *time.Time) float64 {
	if expiryDate == nil {
		return 0
	}
	now := time.Time(common.NewTimestamp())
	if now.After(*expiryDate) {
		return 0
	}
	duration := expiryDate.Sub(now)
	return duration.Hours() / (24 * 365.25)
}

//Personal.AI order the ending
