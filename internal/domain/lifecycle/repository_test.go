package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MockLifecycleRepository is a mock implementation of LifecycleRepository
type MockLifecycleRepository struct {
	mock.Mock
}

func (m *MockLifecycleRepository) CreateAnnuity(ctx context.Context, annuity *Annuity) error {
	args := m.Called(ctx, annuity)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetAnnuity(ctx context.Context, id string) (*Annuity, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Annuity), args.Error(1)
}

func (m *MockLifecycleRepository) GetAnnuitiesByPatent(ctx context.Context, patentID string) ([]*Annuity, error) {
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

func (m *MockLifecycleRepository) UpdateAnnuityStatus(ctx context.Context, id string, status AnnuityStatus, paidAmount int64, paidDate *time.Time, paymentRef string) error {
	args := m.Called(ctx, id, status, paidAmount, paidDate, paymentRef)
	return args.Error(0)
}

func (m *MockLifecycleRepository) BatchCreateAnnuities(ctx context.Context, annuities []*Annuity) error {
	args := m.Called(ctx, annuities)
	return args.Error(0)
}

func (m *MockLifecycleRepository) UpdateReminderSent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockLifecycleRepository) CreateDeadline(ctx context.Context, deadline *Deadline) error {
	args := m.Called(ctx, deadline)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetDeadline(ctx context.Context, id string) (*Deadline, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

func (m *MockLifecycleRepository) GetDeadlinesByPatent(ctx context.Context, patentID string, statusFilter []DeadlineStatus) ([]*Deadline, error) {
	args := m.Called(ctx, patentID, statusFilter)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockLifecycleRepository) GetActiveDeadlines(ctx context.Context, userID *string, daysAhead int, limit, offset int) ([]*Deadline, int64, error) {
	args := m.Called(ctx, userID, daysAhead, limit, offset)
	return args.Get(0).([]*Deadline), args.Get(1).(int64), args.Error(2)
}

func (m *MockLifecycleRepository) UpdateDeadlineStatus(ctx context.Context, id string, status DeadlineStatus, completedBy *string) error {
	args := m.Called(ctx, id, status, completedBy)
	return args.Error(0)
}

func (m *MockLifecycleRepository) ExtendDeadline(ctx context.Context, id string, newDueDate time.Time, reason string) error {
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

func (m *MockLifecycleRepository) GetEventsByPatent(ctx context.Context, patentID string, eventTypes []EventType, limit, offset int) ([]*LifecycleEvent, int64, error) {
	args := m.Called(ctx, patentID, eventTypes, limit, offset)
	return args.Get(0).([]*LifecycleEvent), args.Get(1).(int64), args.Error(2)
}

func (m *MockLifecycleRepository) GetEventTimeline(ctx context.Context, patentID string) ([]*LifecycleEvent, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*LifecycleEvent), args.Error(1)
}

func (m *MockLifecycleRepository) GetRecentEvents(ctx context.Context, orgID string, limit int) ([]*LifecycleEvent, error) {
	args := m.Called(ctx, orgID, limit)
	return args.Get(0).([]*LifecycleEvent), args.Error(1)
}

func (m *MockLifecycleRepository) CreateCostRecord(ctx context.Context, record *CostRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetCostsByPatent(ctx context.Context, patentID string) ([]*CostRecord, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*CostRecord), args.Error(1)
}

func (m *MockLifecycleRepository) GetCostSummary(ctx context.Context, patentID string) (*CostSummary, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CostSummary), args.Error(1)
}

func (m *MockLifecycleRepository) GetPortfolioCostSummary(ctx context.Context, portfolioID string, startDate, endDate time.Time) (*PortfolioCostSummary, error) {
	args := m.Called(ctx, portfolioID, startDate, endDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PortfolioCostSummary), args.Error(1)
}

func (m *MockLifecycleRepository) GetLifecycleDashboard(ctx context.Context, orgID string) (*DashboardStats, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DashboardStats), args.Error(1)
}

func (m *MockLifecycleRepository) SavePayment(ctx context.Context, payment *PaymentRecord) (*PaymentRecord, error) {
	args := m.Called(ctx, payment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PaymentRecord), args.Error(1)
}

func (m *MockLifecycleRepository) QueryPayments(ctx context.Context, query *PaymentQuery) ([]PaymentRecord, int64, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]PaymentRecord), args.Get(1).(int64), args.Error(2)
}

func (m *MockLifecycleRepository) GetByPatentID(ctx context.Context, patentID string) (*LegalStatusEntity, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LegalStatusEntity), args.Error(1)
}

func (m *MockLifecycleRepository) UpdateStatus(ctx context.Context, patentID string, status string, effectiveDate time.Time) error {
	args := m.Called(ctx, patentID, status, effectiveDate)
	return args.Error(0)
}

func (m *MockLifecycleRepository) SaveSubscription(ctx context.Context, sub *SubscriptionEntity) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockLifecycleRepository) DeactivateSubscription(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetStatusHistory(ctx context.Context, patentID string, pagination *commontypes.Pagination, from, to *time.Time) ([]*StatusHistoryEntity, error) {
	args := m.Called(ctx, patentID, pagination, from, to)
	return args.Get(0).([]*StatusHistoryEntity), args.Error(1)
}

func (m *MockLifecycleRepository) SaveCustomEvent(ctx context.Context, event *CustomEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetCustomEvents(ctx context.Context, patentIDs []string, start, end time.Time) ([]CustomEvent, error) {
	args := m.Called(ctx, patentIDs, start, end)
	return args.Get(0).([]CustomEvent), args.Error(1)
}

func (m *MockLifecycleRepository) UpdateEventStatus(ctx context.Context, eventID string, status string) error {
	args := m.Called(ctx, eventID, status)
	return args.Error(0)
}

func (m *MockLifecycleRepository) DeleteEvent(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func (m *MockLifecycleRepository) WithTx(ctx context.Context, fn func(LifecycleRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func TestApplyLifecycleOptions(t *testing.T) {
	tests := []struct {
		name     string
		opts     []LifecycleQueryOption
		expected LifecycleQueryOptions
	}{
		{
			name:     "Default Options",
			opts:     nil,
			expected: LifecycleQueryOptions{Limit: 20, Offset: 0},
		},
		{
			name:     "Custom Limit",
			opts:     []LifecycleQueryOption{WithLimit(50)},
			expected: LifecycleQueryOptions{Limit: 50, Offset: 0},
		},
		{
			name:     "Max Limit Enforced",
			opts:     []LifecycleQueryOption{WithLimit(150)},
			expected: LifecycleQueryOptions{Limit: 100, Offset: 0},
		},
		{
			name:     "Min Limit Enforced",
			opts:     []LifecycleQueryOption{WithLimit(0)},
			expected: LifecycleQueryOptions{Limit: 20, Offset: 0},
		},
		{
			name:     "Custom Offset",
			opts:     []LifecycleQueryOption{WithOffset(10)},
			expected: LifecycleQueryOptions{Limit: 20, Offset: 10},
		},
		{
			name:     "Negative Offset Corrected",
			opts:     []LifecycleQueryOption{WithOffset(-1)},
			expected: LifecycleQueryOptions{Limit: 20, Offset: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyLifecycleOptions(tt.opts...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

//Personal.AI order the ending
