package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks

type MockDeadlineRepository struct {
	mock.Mock
}

func (m *MockDeadlineRepository) SaveDeadline(ctx context.Context, deadline *Deadline) error {
	args := m.Called(ctx, deadline)
	return args.Error(0)
}

func (m *MockDeadlineRepository) GetDeadlineByID(ctx context.Context, id string) (*Deadline, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

func (m *MockDeadlineRepository) GetDeadlinesByPatentID(ctx context.Context, patentID string) ([]*Deadline, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineRepository) GetDeadlinesByOwnerID(ctx context.Context, ownerID string, from, to time.Time) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineRepository) GetOverdueDeadlines(ctx context.Context, ownerID string, asOf time.Time) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, asOf)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineRepository) GetUpcomingDeadlines(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, withinDays)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineRepository) GetDeadlinesByType(ctx context.Context, deadlineType DeadlineType) ([]*Deadline, error) {
	args := m.Called(ctx, deadlineType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineRepository) GetPendingDeadlineReminders(ctx context.Context, reminderDate time.Time) ([]*Deadline, error) {
	args := m.Called(ctx, reminderDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineRepository) DeleteDeadline(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDeadlineRepository) CountDeadlinesByUrgency(ctx context.Context, ownerID string) (map[DeadlineUrgency]int64, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[DeadlineUrgency]int64), args.Error(1)
}

// Tests

func TestNewDeadline_Success(t *testing.T) {
	dueDate := time.Now().AddDate(0, 0, 10)
	d, err := NewDeadline("p1", DeadlineTypeFilingResponse, "Title", dueDate)
	assert.NoError(t, err)
	assert.NotEmpty(t, d.ID)
	assert.Equal(t, UrgencyHigh, d.Urgency)
	assert.NotEmpty(t, d.ReminderDates)
}

func TestNewDeadline_PastDueDate(t *testing.T) {
	dueDate := time.Now().AddDate(0, 0, -1)
	_, err := NewDeadline("p1", DeadlineTypeCustom, "Title", dueDate)
	assert.Error(t, err)
}

func TestDeadline_CalculateUrgency(t *testing.T) {
	d := &Deadline{DueDate: time.Now().AddDate(0, 0, 5)}
	assert.Equal(t, UrgencyCritical, d.CalculateUrgency(time.Now()))

	d.DueDate = time.Now().AddDate(0, 0, 20)
	assert.Equal(t, UrgencyHigh, d.CalculateUrgency(time.Now()))

	d.DueDate = time.Now().AddDate(0, 0, 60)
	assert.Equal(t, UrgencyMedium, d.CalculateUrgency(time.Now()))

	d.DueDate = time.Now().AddDate(0, 0, 100)
	assert.Equal(t, UrgencyLow, d.CalculateUrgency(time.Now()))
}

func TestDeadline_Complete_Success(t *testing.T) {
	d, _ := NewDeadline("p1", DeadlineTypeCustom, "T", time.Now().AddDate(0, 0, 10))
	err := d.Complete("user1")
	assert.NoError(t, err)
	assert.True(t, d.IsCompleted)
	assert.NotNil(t, d.CompletedAt)
	assert.Equal(t, "user1", d.CompletedBy)
}

func TestDeadline_Extend_Success(t *testing.T) {
	d, _ := NewDeadline("p1", DeadlineTypeCustom, "T", time.Now().AddDate(0, 0, 10))
	d.ExtensionAvailable = true
	d.MaxExtensionDays = 30

	err := d.Extend(15)
	assert.NoError(t, err)
	assert.NotNil(t, d.ExtendedDueDate)
	// Original due + 15 days
	expected := d.DueDate.AddDate(0, 0, 15)
	assert.Equal(t, expected, *d.ExtendedDueDate)
}

func TestGetCalendar(t *testing.T) {
	repo := new(MockDeadlineRepository)
	svc := NewDeadlineService(repo)

	from := time.Now()
	to := from.AddDate(0, 1, 0)

	deadlines := []*Deadline{
		{ID: "d1", DueDate: from.AddDate(0, 0, 2), Urgency: UrgencyCritical}, // Week
		{ID: "d2", DueDate: from.AddDate(0, 0, 20), Urgency: UrgencyHigh}, // Month
	}
	repo.On("GetDeadlinesByOwnerID", mock.Anything, "owner1", mock.Anything, mock.Anything).Return(deadlines, nil)

	cal, err := svc.GetCalendar(context.Background(), "owner1", from, to)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cal.Deadlines))
	assert.Equal(t, 1, cal.CriticalCount)
	assert.Equal(t, 1, cal.HighCount)
	assert.Equal(t, 1, len(cal.UpcomingWeek)) // d1
	assert.Equal(t, 2, len(cal.UpcomingMonth)) // d2 + d1 (since d1 is also within month)
}

func TestGenerateReminderBatch(t *testing.T) {
	repo := new(MockDeadlineRepository)
	svc := NewDeadlineService(repo)

	today := time.Now().Truncate(24 * time.Hour)
	d1 := &Deadline{ID: "d1", ReminderDates: []time.Time{today}}

	repo.On("GetPendingDeadlineReminders", mock.Anything, mock.Anything).Return([]*Deadline{d1}, nil)

	rems, err := svc.GenerateReminderBatch(context.Background(), today)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(rems))
	assert.Equal(t, "d1", rems[0].DeadlineID)
}

//Personal.AI order the ending
