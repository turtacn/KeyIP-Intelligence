package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDeadlineRepository struct {
	mock.Mock
}

func (m *mockDeadlineRepository) Save(ctx context.Context, deadline *Deadline) error {
	args := m.Called(ctx, deadline)
	return args.Error(0)
}

func (m *mockDeadlineRepository) FindByID(ctx context.Context, id string) (*Deadline, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

func (m *mockDeadlineRepository) FindByPatentID(ctx context.Context, patentID string) ([]*Deadline, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineRepository) FindByOwnerID(ctx context.Context, ownerID string, from, to time.Time) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, from, to)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineRepository) FindOverdue(ctx context.Context, ownerID string, asOf time.Time) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, asOf)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineRepository) FindUpcoming(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, withinDays)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineRepository) FindByType(ctx context.Context, deadlineType DeadlineType) ([]*Deadline, error) {
	args := m.Called(ctx, deadlineType)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineRepository) FindPendingReminders(ctx context.Context, reminderDate time.Time) ([]*Deadline, error) {
	args := m.Called(ctx, reminderDate)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockDeadlineRepository) CountByUrgency(ctx context.Context, ownerID string) (map[DeadlineUrgency]int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(map[DeadlineUrgency]int64), args.Error(1)
}

func TestNewDeadline_Success(t *testing.T) {
	now := time.Now().UTC()
	dueDate := now.AddDate(0, 0, 30)
	d, err := NewDeadline("pat1", DeadlineTypeFilingResponse, "OA Response", dueDate)
	assert.NoError(t, err)
	assert.NotEmpty(t, d.ID)
	assert.Equal(t, UrgencyHigh, d.Urgency)
	assert.NotEmpty(t, d.ReminderDates)
}

func TestNewDeadline_PastDueDate(t *testing.T) {
	now := time.Now().UTC()
	dueDate := now.AddDate(0, 0, -1)
	_, err := NewDeadline("pat1", DeadlineTypeFilingResponse, "OA Response", dueDate)
	assert.Error(t, err)
}

func TestDeadline_CalculateUrgency(t *testing.T) {
	now := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	d1 := &Deadline{DueDate: now.AddDate(0, 0, 5)}
	assert.Equal(t, UrgencyCritical, d1.CalculateUrgency(now))

	d2 := &Deadline{DueDate: now.AddDate(0, 0, 20)}
	assert.Equal(t, UrgencyHigh, d2.CalculateUrgency(now))

	d3 := &Deadline{DueDate: now.AddDate(0, 0, 60)}
	assert.Equal(t, UrgencyMedium, d3.CalculateUrgency(now))

	d4 := &Deadline{DueDate: now.AddDate(0, 0, 120)}
	assert.Equal(t, UrgencyLow, d4.CalculateUrgency(now))
}

func TestDeadline_Complete_Success(t *testing.T) {
	d := &Deadline{IsCompleted: false}
	err := d.Complete("user1")
	assert.NoError(t, err)
	assert.True(t, d.IsCompleted)
	assert.NotNil(t, d.CompletedAt)
	assert.Equal(t, "user1", d.CompletedBy)
}

func TestDeadline_Extend_Success(t *testing.T) {
	now := time.Now().UTC()
	d := &Deadline{
		DueDate:            now.AddDate(0, 0, 30),
		ExtensionAvailable: true,
		MaxExtensionDays:   60,
	}
	err := d.Extend(30)
	assert.NoError(t, err)
	assert.NotNil(t, d.ExtendedDueDate)
	assert.Equal(t, d.DueDate.AddDate(0, 0, 30), *d.ExtendedDueDate)
}

func TestDeadline_IsOverdue(t *testing.T) {
	now := time.Now().UTC()
	d := &Deadline{DueDate: now.AddDate(0, 0, -5)}
	assert.True(t, d.IsOverdue(now))

	d.IsCompleted = true
	assert.False(t, d.IsOverdue(now))
}

func TestGenerateDefaultReminderDates(t *testing.T) {
	now := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	dueDate := now.AddDate(0, 0, 45) // 45 days in future

	reminders := GenerateDefaultReminderDates(dueDate, now)
	// Reminders are at 7, 14, 30, 60 days before.
	// 45 - 60 = -15 (past)
	// 45 - 30 = 15 (future)
	// 45 - 14 = 31 (future)
	// 45 - 7 = 38 (future)
	// Should have 3 reminders.
	assert.Equal(t, 3, len(reminders))
}

func TestCalculateJurisdictionExtension(t *testing.T) {
	avail, days := CalculateJurisdictionExtension("CN", DeadlineTypeFilingResponse)
	assert.True(t, avail)
	assert.Equal(t, 60, days)
}

func TestGetCalendar_Success(t *testing.T) {
	repo := new(mockDeadlineRepository)
	s := NewDeadlineService(repo)

	now := time.Now().UTC()
	d1 := &Deadline{DueDate: now.AddDate(0, 0, 5), Urgency: UrgencyCritical}
	d2 := &Deadline{DueDate: now.AddDate(0, 0, 20), Urgency: UrgencyHigh}

	repo.On("FindByOwnerID", mock.Anything, "user1", mock.Anything, mock.Anything).Return([]*Deadline{d1, d2}, nil)

	cal, err := s.GetCalendar(context.Background(), "user1", now, now.AddDate(0, 0, 30))
	assert.NoError(t, err)
	assert.Equal(t, 1, cal.CriticalCount)
	assert.Equal(t, 1, cal.HighCount)
}

//Personal.AI order the ending
