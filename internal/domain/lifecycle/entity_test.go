// Package lifecycle_test provides unit tests for the PatentLifecycle aggregate root.
package lifecycle_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestNewPatentLifecycle
// ─────────────────────────────────────────────────────────────────────────────

func TestNewPatentLifecycle_CN_GeneratesCorrectSchedule(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)

	require.NoError(t, err)
	require.NotNil(t, lc)

	// CN patents: 20-year term, annuities start year 3.
	assert.Equal(t, ptypes.JurisdictionCN, lc.Jurisdiction)
	assert.Equal(t, filingDate, lc.FilingDate)
	assert.Equal(t, filingDate.AddDate(20, 0, 0), lc.ExpiryDate)

	// Should have initial deadlines (e.g., examination request).
	assert.NotEmpty(t, lc.Deadlines, "CN patent should have initial deadlines")

	// Should have annuity schedule for years 3-20 (18 payments).
	assert.Len(t, lc.AnnuitySchedule, 18, "CN patent should have 18 annuity payments (years 3-20)")

	// Legal status should be "pending".
	assert.Equal(t, "pending", lc.LegalStatus.Current)
	assert.Len(t, lc.LegalStatus.History, 1)

	// Should have a lifecycle_created event.
	assert.Len(t, lc.Events, 1)
	assert.Equal(t, "lifecycle_created", lc.Events[0].Type)
}

func TestNewPatentLifecycle_EmptyPatentNumber(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"",
		ptypes.JurisdictionCN,
		time.Now(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "patent_number")
}

func TestNewPatentLifecycle_ZeroFilingDate(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		time.Time{},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "filing_date")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetUpcomingDeadlines
// ─────────────────────────────────────────────────────────────────────────────

func TestGetUpcomingDeadlines_ReturnsDeadlinesWithinWindow(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	// Add a deadline due in 5 days.
	futureDeadline, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 5),
		lifecycle.PriorityHigh,
		"OA response due",
	)
	err = lc.AddDeadline(*futureDeadline)
	require.NoError(t, err)

	// Add a deadline due in 20 days.
	farFutureDeadline, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineExamination,
		time.Now().UTC().AddDate(0, 0, 20),
		lifecycle.PriorityMedium,
		"Examination request",
	)
	err = lc.AddDeadline(*farFutureDeadline)
	require.NoError(t, err)

	// Get deadlines within 10 days.
	upcoming := lc.GetUpcomingDeadlines(10)

	// Should return only the first deadline.
	assert.Len(t, upcoming, 1)
	assert.Equal(t, lifecycle.DeadlineOAResponse, upcoming[0].Type)
}

func TestGetUpcomingDeadlines_ExcludesCompletedDeadlines(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	deadline, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 5),
		lifecycle.PriorityHigh,
		"OA response",
	)
	err = lc.AddDeadline(*deadline)
	require.NoError(t, err)

	// Mark it completed.
	err = lc.MarkDeadlineCompleted(deadline.ID)
	require.NoError(t, err)

	// Should return empty.
	upcoming := lc.GetUpcomingDeadlines(10)
	assert.Empty(t, upcoming)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMarkDeadlineCompleted
// ─────────────────────────────────────────────────────────────────────────────

func TestMarkDeadlineCompleted_NoLongerInOverdue(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	// Add an overdue deadline (due yesterday).
	overdueDeadline, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, -1),
		lifecycle.PriorityCritical,
		"Overdue OA",
	)
	err = lc.AddDeadline(*overdueDeadline)
	require.NoError(t, err)

	// Verify it's overdue.
	overdue := lc.GetOverdueDeadlines()
	assert.Len(t, overdue, 1)

	// Mark it completed.
	err = lc.MarkDeadlineCompleted(overdueDeadline.ID)
	require.NoError(t, err)

	// Should no longer be overdue.
	overdue = lc.GetOverdueDeadlines()
	assert.Empty(t, overdue)
}

func TestMarkDeadlineCompleted_NotFound(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	err = lc.MarkDeadlineCompleted(common.ID("nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetNextAnnuityPayment
// ─────────────────────────────────────────────────────────────────────────────

func TestGetNextAnnuityPayment_ReturnsEarliestUnpaid(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	next := lc.GetNextAnnuityPayment()
	require.NotNil(t, next)

	// Should be year 3 (first annuity for CN patents).
	assert.Equal(t, 3, next.Year)
	assert.False(t, next.Paid)
}

func TestGetNextAnnuityPayment_NilWhenAllPaid(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	// Mark all annuities as paid.
	for i := range lc.AnnuitySchedule {
		lc.AnnuitySchedule[i].Paid = true
	}

	next := lc.GetNextAnnuityPayment()
	assert.Nil(t, next)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRecordPayment
// ─────────────────────────────────────────────────────────────────────────────

func TestRecordPayment_MarksAnnuityAsPaid(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	next := lc.GetNextAnnuityPayment()
	require.NotNil(t, next)

	// Record payment.
	err = lc.RecordPayment(next.ID, next.Amount, time.Now().UTC())
	require.NoError(t, err)

	// Should be marked as paid.
	for _, a := range lc.AnnuitySchedule {
		if a.ID == next.ID {
			assert.True(t, a.Paid)
			assert.NotNil(t, a.PaidAt)
		}
	}
}

func TestRecordPayment_InsufficientAmount(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	next := lc.GetNextAnnuityPayment()
	require.NotNil(t, next)

	// Try to pay less than required.
	err = lc.RecordPayment(next.ID, next.Amount-10, time.Now().UTC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestUpdateLegalStatus
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateLegalStatus_RecordsInHistory(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	// Initial status is "pending" with 1 history entry.
	assert.Equal(t, "pending", lc.LegalStatus.Current)
	assert.Len(t, lc.LegalStatus.History, 1)

	// Update to "granted".
	err = lc.UpdateLegalStatus("granted", "examiner approved")
	require.NoError(t, err)

	assert.Equal(t, "granted", lc.LegalStatus.Current)
	assert.Len(t, lc.LegalStatus.History, 2)
	assert.Equal(t, "pending", lc.LegalStatus.History[1].From)
	assert.Equal(t, "granted", lc.LegalStatus.History[1].To)
	assert.Equal(t, "examiner approved", lc.LegalStatus.History[1].Reason)
}

func TestUpdateLegalStatus_EmptyNewStatus(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	err = lc.UpdateLegalStatus("", "some reason")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "new_status")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRemainingLifeYears
// ─────────────────────────────────────────────────────────────────────────────

func TestRemainingLifeYears_CorrectCalculation(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	lc, err := lifecycle.NewPatentLifecycle(
		common.NewID(),
		"CN202010000001A",
		ptypes.JurisdictionCN,
		filingDate,
	)
	require.NoError(t, err)

	// Expiry date is 2040-01-01.
	// If today is 2025-01-01, remaining years ≈ 15.
	// We can't control "now" easily in tests, but we can at least check it's > 0.
	remaining := lc.RemainingLifeYears()
	assert.Greater(t, remaining, 0.0, "patent should not be expired yet")

	// If we artificially set expiry to the past:
	lc.ExpiryDate = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	remaining = lc.RemainingLifeYears()
	assert.Equal(t, 0.0, remaining, "expired patent should have 0 remaining years")
}

//Personal.AI order the ending
