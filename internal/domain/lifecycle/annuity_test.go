package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockLifecycleRepository struct {
	mock.Mock
}

func (m *MockLifecycleRepository) CreateAnnuity(ctx context.Context, annuity *Annuity) error {
	args := m.Called(ctx, annuity)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetAnnuity(ctx context.Context, id uuid.UUID) (*Annuity, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Annuity), args.Error(1)
}

func (m *MockLifecycleRepository) GetAnnuitiesByPatent(ctx context.Context, patentID uuid.UUID) ([]*Annuity, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*Annuity), args.Error(1)
}

func (m *MockLifecycleRepository) GetUpcomingAnnuities(ctx context.Context, daysAhead int, limit, offset int) ([]*Annuity, int64, error) {
	args := m.Called(ctx, daysAhead, limit, offset)
	return args.Get(0).([]*Annuity), args.Get(1).(int64), args.Error(2)
}

func (m *MockLifecycleRepository) GetOverdueAnnuities(ctx context.Context, limit, offset int) ([]*Annuity, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*Annuity), args.Get(1).(int64), args.Error(2)
}

func (m *MockLifecycleRepository) UpdateAnnuityStatus(ctx context.Context, id uuid.UUID, status AnnuityStatus, paidAmount int64, paidDate *time.Time, paymentRef string) error {
	args := m.Called(ctx, id, status, paidAmount, paidDate, paymentRef)
	return args.Error(0)
}

func (m *MockLifecycleRepository) BatchCreateAnnuities(ctx context.Context, annuities []*Annuity) error {
	args := m.Called(ctx, annuities)
	return args.Error(0)
}

func (m *MockLifecycleRepository) UpdateReminderSent(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockLifecycleRepository) CreateDeadline(ctx context.Context, deadline *Deadline) error {
	args := m.Called(ctx, deadline)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetDeadline(ctx context.Context, id uuid.UUID) (*Deadline, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

func (m *MockLifecycleRepository) GetDeadlinesByPatent(ctx context.Context, patentID uuid.UUID, statusFilter []DeadlineStatus) ([]*Deadline, error) {
	args := m.Called(ctx, patentID, statusFilter)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockLifecycleRepository) GetActiveDeadlines(ctx context.Context, userID *uuid.UUID, daysAhead int, limit, offset int) ([]*Deadline, int64, error) {
	args := m.Called(ctx, userID, daysAhead, limit, offset)
	return args.Get(0).([]*Deadline), args.Get(1).(int64), args.Error(2)
}

func (m *MockLifecycleRepository) UpdateDeadlineStatus(ctx context.Context, id uuid.UUID, status DeadlineStatus, completedBy *uuid.UUID) error {
	args := m.Called(ctx, id, status, completedBy)
	return args.Error(0)
}

func (m *MockLifecycleRepository) ExtendDeadline(ctx context.Context, id uuid.UUID, newDueDate time.Time, reason string) error {
	args := m.Called(ctx, id, newDueDate, reason)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetCriticalDeadlines(ctx context.Context, limit int) ([]*Deadline, error) {
	args := m.Called(ctx, limit)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockLifecycleRepository) CreateEvent(ctx context.Context, event *LifecycleEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetEventsByPatent(ctx context.Context, patentID uuid.UUID, eventTypes []EventType, limit, offset int) ([]*LifecycleEvent, int64, error) {
	args := m.Called(ctx, patentID, eventTypes, limit, offset)
	return args.Get(0).([]*LifecycleEvent), args.Get(1).(int64), args.Error(2)
}

func (m *MockLifecycleRepository) GetEventTimeline(ctx context.Context, patentID uuid.UUID) ([]*LifecycleEvent, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*LifecycleEvent), args.Error(1)
}

func (m *MockLifecycleRepository) GetRecentEvents(ctx context.Context, orgID uuid.UUID, limit int) ([]*LifecycleEvent, error) {
	args := m.Called(ctx, orgID, limit)
	return args.Get(0).([]*LifecycleEvent), args.Error(1)
}

func (m *MockLifecycleRepository) CreateCostRecord(ctx context.Context, record *CostRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetCostsByPatent(ctx context.Context, patentID uuid.UUID) ([]*CostRecord, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*CostRecord), args.Error(1)
}

func (m *MockLifecycleRepository) GetCostSummary(ctx context.Context, patentID uuid.UUID) (*CostSummary, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).(*CostSummary), args.Error(1)
}

func (m *MockLifecycleRepository) GetPortfolioCostSummary(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) (*PortfolioCostSummary, error) {
	args := m.Called(ctx, portfolioID, startDate, endDate)
	return args.Get(0).(*PortfolioCostSummary), args.Error(1)
}

func (m *MockLifecycleRepository) GetLifecycleDashboard(ctx context.Context, orgID uuid.UUID) (*DashboardStats, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).(*DashboardStats), args.Error(1)
}

func (m *MockLifecycleRepository) WithTx(ctx context.Context, fn func(LifecycleRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

// TestAnnuityStatusIsValid tests the validity of annuity status enum
func TestAnnuityStatusIsValid(t *testing.T) {
	assert.Equal(t, AnnuityStatus("upcoming"), AnnuityStatusUpcoming)
}

func TestDeadlineDaysUntilDue(t *testing.T) {
	now := time.Now().UTC()
	// Using a fixed point in time relative to "now" might be tricky if not careful with truncation.
	// But the logic is int(duration.Hours() / 24).
	// If due tomorrow exactly: 24h -> 1 day.

	deadline := Deadline{
		DueDate: now.Add(25 * time.Hour), // 25 hours from now is > 1 day
	}
	assert.Equal(t, 1, deadline.DaysUntilDue())
}

//Personal.AI order the ending
