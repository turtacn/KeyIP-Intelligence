// Package lifecycle implements annuity payment management for patent maintenance fees.
package lifecycle

import (
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// AnnuityPayment value object
// ─────────────────────────────────────────────────────────────────────────────

// AnnuityPayment represents a single annual maintenance fee payment obligation.
// Different jurisdictions have vastly different annuity structures, payment
// schedules, and penalty regimes.
type AnnuityPayment struct {
	// ID uniquely identifies this payment record.
	ID common.ID `json:"id"`

	// Year indicates which year of the patent term this payment covers.
	// For CN patents, year 1 is counted from filing date; for US patents,
	// maintenance fees are due at years 3.5, 7.5, and 11.5.
	Year int `json:"year"`

	// DueDate is the statutory deadline for payment (before grace period).
	DueDate time.Time `json:"due_date"`

	// Amount is the base payment amount in the specified currency.
	Amount float64 `json:"amount"`

	// Currency is the ISO 4217 currency code (CNY, USD, EUR, JPY, etc.).
	Currency string `json:"currency"`

	// Paid indicates whether the payment has been made.
	Paid bool `json:"paid"`

	// PaidAt is the timestamp when the payment was recorded.
	PaidAt *time.Time `json:"paid_at,omitempty"`

	// PaidAmount is the actual amount paid (may include surcharge if late).
	PaidAmount *float64 `json:"paid_amount,omitempty"`

	// GracePeriodEnd is the last date to pay with a surcharge (if applicable).
	// After this date, the patent may lapse or require petition for revival.
	GracePeriodEnd *time.Time `json:"grace_period_end,omitempty"`

	// SurchargeRate is the penalty rate applied for late payment during grace period.
	// For example, 0.25 means 25% surcharge.
	SurchargeRate float64 `json:"surcharge_rate"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewAnnuityPayment creates a new AnnuityPayment with validation.
//
// Business rules:
//   - Year must be positive
//   - DueDate must not be zero
//   - Amount must be non-negative
//   - Currency must not be empty
func NewAnnuityPayment(year int, dueDate time.Time, amount float64, currency string) (*AnnuityPayment, error) {
	if year < 1 {
		return nil, errors.InvalidParam("year must be positive")
	}
	if dueDate.IsZero() {
		return nil, errors.InvalidParam("due_date must not be zero")
	}
	if amount < 0 {
		return nil, errors.InvalidParam("amount must be non-negative")
	}
	if currency == "" {
		return nil, errors.InvalidParam("currency must not be empty")
	}

	return &AnnuityPayment{
		ID:            common.NewID(),
		Year:          year,
		DueDate:       dueDate,
		Amount:        amount,
		Currency:      currency,
		Paid:          false,
		SurchargeRate: 0,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Query methods
// ─────────────────────────────────────────────────────────────────────────────

// IsOverdue returns true if the due date has passed and payment has not been made.
func (a *AnnuityPayment) IsOverdue() bool {
	if a.Paid {
		return false
	}
	now := time.Now().UTC()
	return now.After(a.DueDate)
}

// IsInGracePeriod returns true if the payment is overdue but still within the
// grace period (if one exists).
func (a *AnnuityPayment) IsInGracePeriod() bool {
	if a.Paid || a.GracePeriodEnd == nil {
		return false
	}
	now := time.Now().UTC()
	return now.After(a.DueDate) && now.Before(*a.GracePeriodEnd)
}

// CalculateSurcharge computes the late payment penalty if the payment is made
// during the grace period.
func (a *AnnuityPayment) CalculateSurcharge() float64 {
	if !a.IsInGracePeriod() {
		return 0
	}
	return a.Amount * a.SurchargeRate
}

// TotalDue returns the total amount due including surcharge if applicable.
func (a *AnnuityPayment) TotalDue() float64 {
	return a.Amount + a.CalculateSurcharge()
}

// ─────────────────────────────────────────────────────────────────────────────
// Command methods
// ─────────────────────────────────────────────────────────────────────────────

// Pay records a payment for this annuity.
//
// Business rules:
//   - Amount must be >= TotalDue()
//   - Cannot pay an already paid annuity
func (a *AnnuityPayment) Pay(amount float64) error {
	if a.Paid {
		return errors.InvalidState("annuity payment has already been recorded")
	}

	totalDue := a.TotalDue()
	if amount < totalDue {
		return errors.InvalidParam(
			fmt.Sprintf("insufficient payment: %.2f %s required, %.2f %s provided",
				totalDue, a.Currency, amount, a.Currency),
		)
	}

	now := time.Now().UTC()
	a.Paid = true
	a.PaidAt = &now
	a.PaidAmount = &amount

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Schedule generation
// ─────────────────────────────────────────────────────────────────────────────

// GenerateAnnuitySchedule creates the complete annuity payment schedule for a
// patent based on its jurisdiction.
//
// Jurisdiction-specific rules:
//   - CN: Years 3-20 (18 payments), due on anniversary of filing, 6-month grace period
//   - US: Years 3.5, 7.5, 11.5 (3 "maintenance fees"), 6-month grace period
//   - EP: Years 3-20 (18 payments), due on anniversary of filing, 6-month grace period
//   - JP: Years 1-20 (20 payments), due on anniversary of filing
//   - KR: Years 1-20 (20 payments), due on anniversary of filing
//
// Amount calculations are simplified here; in production, these should come
// from an official fee schedule database.
func GenerateAnnuitySchedule(
	jurisdiction ptypes.JurisdictionCode,
	filingDate time.Time,
	grantDate *time.Time,
) ([]AnnuityPayment, error) {
	rules, err := GetJurisdictionRules(jurisdiction)
	if err != nil {
		return nil, err
	}

	var schedule []AnnuityPayment

	switch jurisdiction {
	case ptypes.JurisdictionCN:
		schedule = generateCNSchedule(filingDate, rules)
	case ptypes.JurisdictionUS:
		schedule = generateUSSchedule(filingDate, grantDate, rules)
	case ptypes.JurisdictionEP:
		schedule = generateEPSchedule(filingDate, rules)
	case ptypes.JurisdictionJP:
		schedule = generateJPSchedule(filingDate, rules)
	case ptypes.JurisdictionKR:
		schedule = generateKRSchedule(filingDate, rules)
	default:
		return nil, errors.InvalidParam(fmt.Sprintf("unsupported jurisdiction: %s", jurisdiction))
	}

	return schedule, nil
}

// generateCNSchedule creates annuity schedule for Chinese patents (years 3-20).
func generateCNSchedule(filingDate time.Time, rules *JurisdictionRules) []AnnuityPayment {
	var schedule []AnnuityPayment

	for year := rules.AnnuityStartYear; year <= rules.PatentTermYears; year++ {
		dueDate := filingDate.AddDate(year, 0, 0)
		gracePeriodEnd := dueDate.AddDate(0, rules.GracePeriodMonths, 0)

		// Simplified fee structure (actual fees increase with year).
		amount := float64(900 + (year-3)*200)

		ap := AnnuityPayment{
			ID:              common.NewID(),
			Year:            year,
			DueDate:         dueDate,
			Amount:          amount,
			Currency:        "CNY",
			Paid:            false,
			GracePeriodEnd:  &gracePeriodEnd,
			SurchargeRate:   rules.SurchargeRate,
		}
		schedule = append(schedule, ap)
	}

	return schedule
}

// generateUSSchedule creates maintenance fee schedule for US patents (milestone-based).
func generateUSSchedule(filingDate time.Time, grantDate *time.Time, rules *JurisdictionRules) []AnnuityPayment {
	var schedule []AnnuityPayment

	// US maintenance fees are due at 3.5, 7.5, and 11.5 years from grant date.
	// If grant date is nil, use filing date as fallback (should not happen in practice).
	baseDate := filingDate
	if grantDate != nil {
		baseDate = *grantDate
	}

	milestones := []struct {
		year   float64
		amount float64
	}{
		{3.5, 1600},
		{7.5, 3600},
		{11.5, 7400},
	}

	for _, m := range milestones {
		years := int(m.year)
		months := int((m.year - float64(years)) * 12)
		dueDate := baseDate.AddDate(years, months, 0)
		gracePeriodEnd := dueDate.AddDate(0, rules.GracePeriodMonths, 0)

		ap := AnnuityPayment{
			ID:             common.NewID(),
			Year:           int(m.year * 2), // Store as integer (7, 15, 23 for display convenience)
			DueDate:        dueDate,
			Amount:         m.amount,
			Currency:       "USD",
			Paid:           false,
			GracePeriodEnd: &gracePeriodEnd,
			SurchargeRate:  rules.SurchargeRate,
		}
		schedule = append(schedule, ap)
	}

	return schedule
}

// generateEPSchedule creates annuity schedule for European patents (years 3-20).
func generateEPSchedule(filingDate time.Time, rules *JurisdictionRules) []AnnuityPayment {
	var schedule []AnnuityPayment

	for year := rules.AnnuityStartYear; year <= rules.PatentTermYears; year++ {
		dueDate := filingDate.AddDate(year, 0, 0)
		gracePeriodEnd := dueDate.AddDate(0, rules.GracePeriodMonths, 0)

		// Simplified fee structure (actual fees vary and increase with year).
		amount := float64(465 + (year-3)*50)

		ap := AnnuityPayment{
			ID:             common.NewID(),
			Year:           year,
			DueDate:        dueDate,
			Amount:         amount,
			Currency:       "EUR",
			Paid:           false,
			GracePeriodEnd: &gracePeriodEnd,
			SurchargeRate:  rules.SurchargeRate,
		}
		schedule = append(schedule, ap)
	}

	return schedule
}

// generateJPSchedule creates annuity schedule for Japanese patents (years 1-20).
func generateJPSchedule(filingDate time.Time, rules *JurisdictionRules) []AnnuityPayment {
	var schedule []AnnuityPayment

	for year := 1; year <= rules.PatentTermYears; year++ {
		dueDate := filingDate.AddDate(year, 0, 0)
		amount := float64(4300 + year*300)

		ap := AnnuityPayment{
			ID:       common.NewID(),
			Year:     year,
			DueDate:  dueDate,
			Amount:   amount,
			Currency: "JPY",
			Paid:     false,
		}
		schedule = append(schedule, ap)
	}

	return schedule
}

// generateKRSchedule creates annuity schedule for Korean patents (years 1-20).
func generateKRSchedule(filingDate time.Time, rules *JurisdictionRules) []AnnuityPayment {
	var schedule []AnnuityPayment

	for year := 1; year <= rules.PatentTermYears; year++ {
		dueDate := filingDate.AddDate(year, 0, 0)
		amount := float64(45000 + year*5000)

		ap := AnnuityPayment{
			ID:       common.NewID(),
			Year:     year,
			DueDate:  dueDate,
			Amount:   amount,
			Currency: "KRW",
			Paid:     false,
		}
		schedule = append(schedule, ap)
	}

	return schedule
}

//Personal.AI order the ending
