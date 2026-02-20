// Package lifecycle_test provides unit tests for the Deadline value object.
package lifecycle_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestNewDeadline
// ─────────────────────────────────────────────────────────────────────────────

func TestNewDeadline_ValidParams(t *testing.T) {
	t.Parallel()

	dueDate := time.Now().UTC().AddDate(0, 0, 30)
	d, err := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		dueDate,
		lifecycle.PriorityHigh,
		"Respond to office action",
	)

	require.NoError(t, err)
	require.NotNil(t, d)
	assert.NotEmpty(t, d.ID)
	assert.Equal(t, lifecycle.DeadlineOAResponse, d.Type)
	assert.Equal(t, dueDate, d.DueDate)
	assert.Equal(t, lifecycle.PriorityHigh, d.Priority)
	assert.Equal(t, "Respond to office action", d.Description)
	assert.False(t, d.Completed)
	assert.Nil(t, d.CompletedAt)
	assert.Equal(t, []int{30, 14, 7, 1}, d.ReminderDays)
	assert.False(t, d.ExtensionAvailable)
	assert.Equal(t, 0, d.MaxExtensionDays)
}

func TestNewDeadline_ZeroDueDate(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Time{},
		lifecycle.PriorityHigh,
		"Test",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "due_date")
}

func TestNewDeadline_EmptyDescription(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 30),
		lifecycle.PriorityHigh,
		"",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "description")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIsOverdue
// ─────────────────────────────────────────────────────────────────────────────

func TestIsOverdue_PastDateNotCompleted(t *testing.T) {
	t.Parallel()

	// Due yesterday
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, -1),
		lifecycle.PriorityHigh,
		"Test",
	)

	assert.True(t, d.IsOverdue(), "past due date should be overdue")
}

func TestIsOverdue_FutureDate(t *testing.T) {
	t.Parallel()

	// Due tomorrow
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 1),
		lifecycle.PriorityHigh,
		"Test",
	)

	assert.False(t, d.IsOverdue(), "future due date should not be overdue")
}

func TestIsOverdue_CompletedDeadline(t *testing.T) {
	t.Parallel()

	// Due yesterday but completed
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, -1),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.Complete()

	assert.False(t, d.IsOverdue(), "completed deadline should not be overdue")
}

func TestIsOverdue_WithExtension(t *testing.T) {
	t.Parallel()

	// Due yesterday
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, -1),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ExtensionAvailable = true
	d.MaxExtensionDays = 30

	// Extend by 10 days
	err := d.Extend(10)
	require.NoError(t, err)

	// Now should not be overdue (extended to 9 days from now)
	assert.False(t, d.IsOverdue())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestDaysUntilDue
// ─────────────────────────────────────────────────────────────────────────────

func TestDaysUntilDue_FutureDate(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 10),
		lifecycle.PriorityHigh,
		"Test",
	)

	days := d.DaysUntilDue()
	assert.InDelta(t, 10, days, 1, "should be approximately 10 days until due")
}

func TestDaysUntilDue_PastDate(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, -5),
		lifecycle.PriorityHigh,
		"Test",
	)

	days := d.DaysUntilDue()
	assert.Less(t, days, 0, "past due date should return negative days")
}

func TestDaysUntilDue_Today(t *testing.T) {
	t.Parallel()

	// Due in approximately 1 hour
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().Add(1*time.Hour),
		lifecycle.PriorityHigh,
		"Test",
	)

	days := d.DaysUntilDue()
	assert.Equal(t, 0, days, "deadline today should return 0 days")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestComplete
// ─────────────────────────────────────────────────────────────────────────────

func TestComplete_MarksAsCompleted(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 10),
		lifecycle.PriorityHigh,
		"Test",
	)

	assert.False(t, d.Completed)
	assert.Nil(t, d.CompletedAt)

	d.Complete()

	assert.True(t, d.Completed)
	assert.NotNil(t, d.CompletedAt)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestExtend
// ─────────────────────────────────────────────────────────────────────────────

func TestExtend_Success(t *testing.T) {
	t.Parallel()

	dueDate := time.Now().UTC().AddDate(0, 0, 30)
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		dueDate,
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ExtensionAvailable = true
	d.MaxExtensionDays = 60

	err := d.Extend(30)
	require.NoError(t, err)
	require.NotNil(t, d.ExtendedTo)
	assert.Equal(t, dueDate.AddDate(0, 0, 30), *d.ExtendedTo)
}

func TestExtend_NotAvailable(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 30),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ExtensionAvailable = false

	err := d.Extend(10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestExtend_ExceedsMaxDays(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 30),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ExtensionAvailable = true
	d.MaxExtensionDays = 30

	err := d.Extend(60)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceed maximum")
}

func TestExtend_CompletedDeadline(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 30),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ExtensionAvailable = true
	d.MaxExtensionDays = 30
	d.Complete()

	err := d.Extend(10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "completed")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestShouldRemind
// ─────────────────────────────────────────────────────────────────────────────

func TestShouldRemind_OnReminderDay(t *testing.T) {
	t.Parallel()

	// Due in exactly 7 days
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 7),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ReminderDays = []int{30, 14, 7, 1}

	assert.True(t, d.ShouldRemind(), "should remind on day 7")
}

func TestShouldRemind_NotOnReminderDay(t *testing.T) {
	t.Parallel()

	// Due in 15 days (not in reminder list)
	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 15),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ReminderDays = []int{30, 14, 7, 1}

	assert.False(t, d.ShouldRemind())
}

func TestShouldRemind_CompletedDeadline(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 7),
		lifecycle.PriorityHigh,
		"Test",
	)
	d.ReminderDays = []int{7}
	d.Complete()

	assert.False(t, d.ShouldRemind(), "completed deadline should not trigger reminder")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNeedsAttention
// ─────────────────────────────────────────────────────────────────────────────

func TestNeedsAttention_WithinSevenDays(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 5),
		lifecycle.PriorityCritical,
		"Test",
	)

	assert.True(t, d.NeedsAttention())
}

func TestNeedsAttention_MoreThanSevenDays(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 10),
		lifecycle.PriorityCritical,
		"Test",
	)

	assert.False(t, d.NeedsAttention())
}

func TestNeedsAttention_Completed(t *testing.T) {
	t.Parallel()

	d, _ := lifecycle.NewDeadline(
		lifecycle.DeadlineOAResponse,
		time.Now().UTC().AddDate(0, 0, 3),
		lifecycle.PriorityCritical,
		"Test",
	)
	d.Complete()

	assert.False(t, d.NeedsAttention())
}

//Personal.AI order the ending
