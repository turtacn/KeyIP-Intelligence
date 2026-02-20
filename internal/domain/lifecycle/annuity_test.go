// Package lifecycle_test provides unit tests for annuity payment management.
package lifecycle_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestNewAnnuityPayment
// ─────────────────────────────────────────────────────────────────────────────

func TestNewAnnuityPayment_ValidParams(t *testing.T) {
	t.Parallel()

	dueDate := time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)
	ap, err := lifecycle.NewAnnuityPayment(3, dueDate, 900.0, "CNY")

	require.NoError(t, err)
	require.NotNil(t, ap)
	assert.NotEmpty(t, ap.ID)
	assert.Equal(t, 3, ap.Year)
	assert.Equal(t, dueDate, ap.DueDate)
	assert.Equal(t, 900.0, ap.Amount)
	assert.Equal(t, "CNY", ap.Currency)
	assert.False(t, ap.Paid)
	assert.Nil(t, ap.PaidAt)
}

func TestNewAnnuityPayment_InvalidYear(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.NewAnnuityPayment(0, time.Now(), 900.0, "CNY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "year")
}

func TestNewAnnuityPayment_ZeroDueDate(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.NewAnnuityPayment(3, time.Time{}, 900.0, "CNY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "due_date")
}

func TestNewAnnuityPayment_NegativeAmount(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.NewAnnuityPayment(3, time.Now(), -100.0, "CNY")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIsOverdue and IsInGracePeriod
// ─────────────────────────────────────────────────────────────────────────────

func TestIsOverdue_PastDueNotPaid(t *testing.T) {
	t.Parallel()

	ap, _ := lifecycle.NewAnnuityPayment(3, time.Now().UTC().AddDate(0, 0, -5), 900.0, "CNY")
	assert.True(t, ap.IsOverdue())
}

func TestIsOverdue_PastDuePaid(t *testing.T) {
	t.Parallel()

	ap, _ := lifecycle.NewAnnuityPayment(3, time.Now().UTC().AddDate(0, 0, -5), 900.0, "CNY")
	_ = ap.Pay(900.0)
	assert.False(t, ap.IsOverdue())
}

func TestIsInGracePeriod_WithinGracePeriod(t *testing.T) {
	t.Parallel()

	dueDate := time.Now().UTC().AddDate(0, 0, -10)
	gracePeriodEnd := time.Now().UTC().AddDate(0, 1, 0)
	ap, _ := lifecycle.NewAnnuityPayment(3, dueDate, 900.0, "CNY")
	ap.GracePeriodEnd = &gracePeriodEnd
	ap.SurchargeRate = 0.25

	assert.True(t, ap.IsInGracePeriod())
}

func TestIsInGracePeriod_AfterGracePeriod(t *testing.T) {
	t.Parallel()

	dueDate := time.Now().UTC().AddDate(0, -7, 0)
	gracePeriodEnd := time.Now().UTC().AddDate(0, -1, 0)
	ap, _ := lifecycle.NewAnnuityPayment(3, dueDate, 900.0, "CNY")
	ap.GracePeriodEnd = &gracePeriodEnd

	assert.False(t, ap.IsInGracePeriod())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCalculateSurcharge
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateSurcharge_InGracePeriod(t *testing.T) {
	t.Parallel()

	dueDate := time.Now().UTC().AddDate(0, 0, -10)
	gracePeriodEnd := time.Now().UTC().AddDate(0, 1, 0)
	ap, _ := lifecycle.NewAnnuityPayment(3, dueDate, 1000.0, "CNY")
	ap.GracePeriodEnd = &gracePeriodEnd
	ap.SurchargeRate = 0.25

	surcharge := ap.CalculateSurcharge()
	assert.Equal(t, 250.0, surcharge, "25% surcharge on 1000 should be 250")
}

func TestCalculateSurcharge_NotInGracePeriod(t *testing.T) {
	t.Parallel()

	dueDate := time.Now().UTC().AddDate(0, 0, 10)
	ap, _ := lifecycle.NewAnnuityPayment(3, dueDate, 1000.0, "CNY")
	ap.SurchargeRate = 0.25

	surcharge := ap.CalculateSurcharge()
	assert.Equal(t, 0.0, surcharge)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPay
// ─────────────────────────────────────────────────────────────────────────────

func TestPay_SufficientAmount(t *testing.T) {
	t.Parallel()

	ap, _ := lifecycle.NewAnnuityPayment(3, time.Now().UTC().AddDate(0, 0, 10), 900.0, "CNY")

	err := ap.Pay(900.0)
	require.NoError(t, err)
	assert.True(t, ap.Paid)
	assert.NotNil(t, ap.PaidAt)
	assert.Equal(t, 900.0, *ap.PaidAmount)
}

func TestPay_InsufficientAmount(t *testing.T) {
	t.Parallel()

	ap, _ := lifecycle.NewAnnuityPayment(3, time.Now().UTC().AddDate(0, 0, 10), 900.0, "CNY")

	err := ap.Pay(800.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
	assert.False(t, ap.Paid)
}

func TestPay_WithSurcharge(t *testing.T) {
	t.Parallel()

	dueDate := time.Now().UTC().AddDate(0, 0, -10)
	gracePeriodEnd := time.Now().UTC().AddDate(0, 1, 0)
	ap, _ := lifecycle.NewAnnuityPayment(3, dueDate, 1000.0, "CNY")
	ap.GracePeriodEnd = &gracePeriodEnd
	ap.SurchargeRate = 0.25

	// Total due = 1000 + 250 = 1250
	err := ap.Pay(1250.0)
	require.NoError(t, err)
	assert.True(t, ap.Paid)
}

func TestPay_AlreadyPaid(t *testing.T) {
	t.Parallel()

	ap, _ := lifecycle.NewAnnuityPayment(3, time.Now().UTC().AddDate(0, 0, 10), 900.0, "CNY")
	_ = ap.Pay(900.0)

	err := ap.Pay(900.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGenerateAnnuitySchedule
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateAnnuitySchedule_CN_18Payments(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule, err := lifecycle.GenerateAnnuitySchedule(ptypes.JurisdictionCN, filingDate, nil)

	require.NoError(t, err)
	assert.Len(t, schedule, 18, "CN patents should have 18 annuity payments (years 3-20)")

	// First payment should be for year 3.
	assert.Equal(t, 3, schedule[0].Year)
	assert.Equal(t, "CNY", schedule[0].Currency)

	// Last payment should be for year 20.
	assert.Equal(t, 20, schedule[len(schedule)-1].Year)
}

func TestGenerateAnnuitySchedule_US_3Payments(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	grantDate := time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)
	schedule, err := lifecycle.GenerateAnnuitySchedule(ptypes.JurisdictionUS, filingDate, &grantDate)

	require.NoError(t, err)
	assert.Len(t, schedule, 3, "US patents should have 3 maintenance fee payments")

	assert.Equal(t, "USD", schedule[0].Currency)

	// First maintenance fee is 3.5 years from grant = 2025-12-01 (approximately).
	expectedFirst := grantDate.AddDate(3, 6, 0)
	assert.Equal(t, expectedFirst.Year(), schedule[0].DueDate.Year())
}

func TestGenerateAnnuitySchedule_EP_18Payments(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule, err := lifecycle.GenerateAnnuitySchedule(ptypes.JurisdictionEP, filingDate, nil)

	require.NoError(t, err)
	assert.Len(t, schedule, 18, "EP patents should have 18 annuity payments (years 3-20)")
	assert.Equal(t, "EUR", schedule[0].Currency)
}

func TestGenerateAnnuitySchedule_UnsupportedJurisdiction(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := lifecycle.GenerateAnnuitySchedule("XX", filingDate, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

