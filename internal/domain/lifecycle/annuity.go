package lifecycle

import (
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
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

// Annuity represents a yearly fee.
type Annuity struct {
	ID               string         `json:"id"`
	PatentID         string         `json:"patent_id"`
	YearNumber       int            `json:"year_number"`
	DueDate          time.Time      `json:"due_date"`
	GraceDeadline    *time.Time     `json:"grace_deadline,omitempty"`
	Status           AnnuityStatus  `json:"status"`
	Amount           *int64         `json:"amount,omitempty"` // Money in minor units
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

// NewAnnuity creates a new annuity record.
func NewAnnuity(patentID string, year int, dueDate time.Time, currency string, amount int64) (*Annuity, error) {
	if patentID == "" {
		return nil, errors.NewValidation("patentID is required")
	}
	if year < 1 {
		return nil, errors.NewValidation("year must be >= 1")
	}
	if currency == "" {
		return nil, errors.NewValidation("currency is required")
	}
	if amount < 0 {
		return nil, errors.NewValidation("amount cannot be negative")
	}

	now := time.Time(common.NewTimestamp())
	grace := dueDate.AddDate(0, 6, 0) // Default 6 months grace

	return &Annuity{
		ID:            string(common.NewID()),
		PatentID:      patentID,
		YearNumber:    year,
		DueDate:       dueDate,
		GraceDeadline: &grace,
		Status:        AnnuityStatusUpcoming,
		Amount:        &amount,
		Currency:      currency,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// Pay marks the annuity as paid.
func (a *Annuity) Pay(amount int64, currency string, paidDate time.Time, ref string) error {
	if currency != a.Currency {
		return errors.NewValidation("payment currency mismatch")
	}
	if amount < 0 {
		return errors.NewValidation("payment amount cannot be negative")
	}

	a.Status = AnnuityStatusPaid
	a.PaidAmount = &amount
	a.PaidDate = &paidDate
	a.PaymentReference = ref
	a.UpdatedAt = time.Time(common.NewTimestamp())
	return nil
}

// CheckStatus updates the status based on current date.
func (a *Annuity) CheckStatus(now time.Time) {
	if a.Status == AnnuityStatusPaid || a.Status == AnnuityStatusWaived || a.Status == AnnuityStatusAbandoned {
		return
	}

	if now.After(a.DueDate) {
		if a.GraceDeadline != nil && now.Before(*a.GraceDeadline) {
			a.Status = AnnuityStatusGracePeriod
		} else {
			a.Status = AnnuityStatusOverdue
		}
	} else if now.AddDate(0, 3, 0).After(a.DueDate) { // Within 3 months
		a.Status = AnnuityStatusDue
	} else {
		a.Status = AnnuityStatusUpcoming
	}
	a.UpdatedAt = time.Time(common.NewTimestamp())
}

//Personal.AI order the ending
