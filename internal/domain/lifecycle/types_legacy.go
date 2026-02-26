package lifecycle

import (
	"time"

	"github.com/google/uuid"
)

// Legacy types for infrastructure compatibility

type EventType string

type CostRecord struct {
	ID               uuid.UUID              `json:"id"`
	PatentID         uuid.UUID              `json:"patent_id"`
	CostType         string                 `json:"cost_type"`
	Amount           int64                  `json:"amount"`
	Currency         string                 `json:"currency"`
	AmountUSD        *int64                 `json:"amount_usd"`
	ExchangeRate     *float64               `json:"exchange_rate"`
	IncurredDate     time.Time              `json:"incurred_date"`
	Description      string                 `json:"description"`
	InvoiceReference string                 `json:"invoice_reference"`
	RelatedAnnuityID *uuid.UUID             `json:"related_annuity_id"`
	RelatedEventID   *uuid.UUID             `json:"related_event_id"`
	Metadata         map[string]interface{} `json:"metadata"`
	CreatedAt        time.Time              `json:"created_at"`
}

type CostSummary struct {
	TotalCosts map[string]int64
}

type PortfolioCostSummary struct {
	TotalCosts map[string]int64
}

type DashboardStats struct {
	UpcomingAnnuities int64
	OverdueAnnuities  int64
	ActiveDeadlines   int64
	RecentEvents      int64
	TotalCostYTD      int64
}

type PaymentRecord struct {
	ID           string
	PatentID     string
	Jurisdiction string
	YearNumber   int
	Amount       float64
	Currency     string
	PaidDate     time.Time
	PaymentRef   string
	PaidBy       string
	Notes        string
	RecordedAt   time.Time
}

type PaymentQuery struct {
	PatentID     string
	PortfolioID  string
	Jurisdiction string
	StartDate    time.Time
	EndDate      time.Time
	Offset       int
	Limit        int
}

type SubscriptionEntity struct {
	ID            string
	PatentIDs     []string
	PortfolioID   string
	StatusFilters []string
	Channels      []string
	Recipient     string
	Active        bool
	CreatedAt     time.Time
}

type LegalStatusEntity struct {
	PatentID                string
	Jurisdiction            string
	Status                  string
	PreviousStatus          string
	EffectiveDate           time.Time
	NextAction              string
	NextDeadline            *time.Time
	RemoteStatus            string
	LastSyncAt              *time.Time
	ConsecutiveSyncFailures int
	RawData                 map[string]interface{}
}

type StatusHistoryEntity struct {
	EventID     string
	PatentID    string
	FromStatus  string
	ToStatus    string
	EventDate   time.Time
	Source      string
	Description string
}

type CustomEvent struct {
	ID           string
	PatentID     string
	PatentNumber string
	Title        string
	Description  string
	EventType    string
	Jurisdiction string
	EventDate    time.Time
	DueDate      time.Time
	Priority     string
	Status       string
	Metadata     map[string]string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AnnuityCalcResult holds result of calculation (Legacy DTO)
type AnnuityCalcResult struct {
	Fee            float64
	YearNumber     int
	DueDate        time.Time
	GracePeriodEnd time.Time
	Status         string
}

// ScheduleEntry is a single entry in schedule (Legacy DTO)
type ScheduleEntry struct {
	YearNumber     int
	Fee            float64
	DueDate        time.Time
	GracePeriodEnd time.Time
	Status         string
}

// RemoteStatusResult represents external status (Legacy)
type RemoteStatusResult struct {
	Status        string
	EffectiveDate time.Time
	NextAction    string
	Source        string
	Jurisdiction  string
}

// Ensure compatibility types are available
type Jurisdiction = string
type DeadlineID = string
type AnnuityID = string
