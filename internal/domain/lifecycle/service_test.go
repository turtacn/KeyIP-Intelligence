package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks

type MockLifecycleRepository struct {
	mock.Mock
}

func (m *MockLifecycleRepository) SaveLifecycle(ctx context.Context, record *LifecycleRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockLifecycleRepository) GetLifecycleByID(ctx context.Context, id string) (*LifecycleRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LifecycleRecord), args.Error(1)
}

func (m *MockLifecycleRepository) GetLifecycleByPatentID(ctx context.Context, patentID string) (*LifecycleRecord, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LifecycleRecord), args.Error(1)
}

func (m *MockLifecycleRepository) GetLifecyclesByPhase(ctx context.Context, phase LifecyclePhase, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error) {
	args := m.Called(ctx, phase, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*LifecycleRecord), args.Error(1)
}

func (m *MockLifecycleRepository) GetExpiringLifecycles(ctx context.Context, withinDays int) ([]*LifecycleRecord, error) {
	args := m.Called(ctx, withinDays)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*LifecycleRecord), args.Error(1)
}

func (m *MockLifecycleRepository) GetLifecyclesByJurisdiction(ctx context.Context, jurisdictionCode string, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error) {
	args := m.Called(ctx, jurisdictionCode, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*LifecycleRecord), args.Error(1)
}

func (m *MockLifecycleRepository) DeleteLifecycle(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockLifecycleRepository) CountLifecycles(ctx context.Context, phase LifecyclePhase) (int64, error) {
	args := m.Called(ctx, phase)
	return args.Get(0).(int64), args.Error(1)
}

// Stub implementation for other methods
func (m *MockLifecycleRepository) SavePayment(ctx context.Context, payment *PaymentRecord) (*PaymentRecord, error) {
	return payment, nil
}
func (m *MockLifecycleRepository) QueryPayments(ctx context.Context, query *PaymentQuery) ([]PaymentRecord, int64, error) {
	return []PaymentRecord{}, 0, nil
}

type MockAnnuityService struct {
	mock.Mock
}

func (m *MockAnnuityService) GenerateSchedule(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time, maxYears int) (*AnnuitySchedule, error) {
	args := m.Called(ctx, patentID, jurisdictionCode, filingDate, maxYears)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AnnuitySchedule), args.Error(1)
}

func (m *MockAnnuityService) CalculateAnnuityFee(ctx context.Context, jurisdictionCode string, yearNumber int) (Money, error) {
	args := m.Called(ctx, jurisdictionCode, yearNumber)
	return args.Get(0).(Money), args.Error(1)
}

func (m *MockAnnuityService) MarkAsPaid(ctx context.Context, recordID string, paidAmount Money, paidDate time.Time) error {
	args := m.Called(ctx, recordID, paidAmount, paidDate)
	return args.Error(0)
}

func (m *MockAnnuityService) MarkAsAbandoned(ctx context.Context, recordID string, reason string) error {
	args := m.Called(ctx, recordID, reason)
	return args.Error(0)
}

func (m *MockAnnuityService) CheckOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, asOfDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *MockAnnuityService) ForecastCosts(ctx context.Context, portfolioID string, years int) (*AnnuityCostForecast, error) {
	args := m.Called(ctx, portfolioID, years)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AnnuityCostForecast), args.Error(1)
}

func (m *MockAnnuityService) GetUpcomingPayments(ctx context.Context, portfolioID string, withinDays int) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, portfolioID, withinDays)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

type MockDeadlineService struct {
	mock.Mock
}

func (m *MockDeadlineService) CreateDeadline(ctx context.Context, patentID string, deadlineType DeadlineType, title string, dueDate time.Time) (*Deadline, error) {
	args := m.Called(ctx, patentID, deadlineType, title, dueDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

func (m *MockDeadlineService) CompleteDeadline(ctx context.Context, deadlineID, completedBy string) error {
	args := m.Called(ctx, deadlineID, completedBy)
	return args.Error(0)
}

func (m *MockDeadlineService) ExtendDeadline(ctx context.Context, deadlineID string, extensionDays int) error {
	args := m.Called(ctx, deadlineID, extensionDays)
	return args.Error(0)
}

func (m *MockDeadlineService) GetCalendar(ctx context.Context, ownerID string, from, to time.Time) (*DeadlineCalendar, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DeadlineCalendar), args.Error(1)
}

func (m *MockDeadlineService) GetOverdueDeadlines(ctx context.Context, ownerID string) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineService) GetUpcomingDeadlines(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, withinDays)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *MockDeadlineService) RefreshUrgencies(ctx context.Context, ownerID string) error {
	args := m.Called(ctx, ownerID)
	return args.Error(0)
}

func (m *MockDeadlineService) GenerateReminderBatch(ctx context.Context, asOf time.Time) ([]*DeadlineReminder, error) {
	args := m.Called(ctx, asOf)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*DeadlineReminder), args.Error(1)
}

func (m *MockDeadlineService) AddCustomDeadline(ctx context.Context, patentID, title, description string, dueDate time.Time) (*Deadline, error) {
	args := m.Called(ctx, patentID, title, description, dueDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

type MockJurisdictionRegistry struct {
	mock.Mock
}

func (m *MockJurisdictionRegistry) Get(code string) (*JurisdictionInfo, error) {
	args := m.Called(code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*JurisdictionInfo), args.Error(1)
}

func (m *MockJurisdictionRegistry) List() []*JurisdictionInfo {
	args := m.Called()
	return args.Get(0).([]*JurisdictionInfo)
}

func (m *MockJurisdictionRegistry) IsSupported(code string) bool {
	args := m.Called(code)
	return args.Bool(0)
}

func (m *MockJurisdictionRegistry) GetPatentTerm(code string, patentType string) (int, error) {
	args := m.Called(code, patentType)
	return args.Int(0), args.Error(1)
}

func (m *MockJurisdictionRegistry) GetAnnuityRules(code string) (*AnnuityRules, error) {
	args := m.Called(code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AnnuityRules), args.Error(1)
}

func (m *MockJurisdictionRegistry) GetOAResponseRules(code string) (*OAResponseRules, error) {
	args := m.Called(code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*OAResponseRules), args.Error(1)
}

// Tests

func TestInitializeLifecycle_Success(t *testing.T) {
	repo := new(MockLifecycleRepository)
	annuitySvc := new(MockAnnuityService)
	deadlineSvc := new(MockDeadlineService)
	reg := new(MockJurisdictionRegistry)
	svc := NewLifecycleService(repo, annuitySvc, deadlineSvc, reg)

	reg.On("IsSupported", "CN").Return(true)
	reg.On("GetPatentTerm", "CN", "invention").Return(20, nil)

	repo.On("SaveLifecycle", mock.Anything, mock.AnythingOfType("*lifecycle.LifecycleRecord")).Return(nil)

	lr, err := svc.InitializeLifecycle(context.Background(), "p1", "CN", time.Now())
	assert.NoError(t, err)
	assert.NotNil(t, lr)
	assert.Equal(t, "p1", lr.PatentID)
}

func TestInitializeLifecycle_UnsupportedJurisdiction(t *testing.T) {
	repo := new(MockLifecycleRepository)
	annuitySvc := new(MockAnnuityService)
	deadlineSvc := new(MockDeadlineService)
	reg := new(MockJurisdictionRegistry)
	svc := NewLifecycleService(repo, annuitySvc, deadlineSvc, reg)

	reg.On("IsSupported", "XX").Return(false)

	_, err := svc.InitializeLifecycle(context.Background(), "p1", "XX", time.Now())
	assert.Error(t, err)
}

func TestAdvancePhase_Success(t *testing.T) {
	repo := new(MockLifecycleRepository)
	annuitySvc := new(MockAnnuityService)
	deadlineSvc := new(MockDeadlineService)
	reg := new(MockJurisdictionRegistry)
	svc := NewLifecycleService(repo, annuitySvc, deadlineSvc, reg)

	lr, _ := NewLifecycleRecord("p1", "CN", time.Now())
	repo.On("GetLifecycleByPatentID", mock.Anything, "p1").Return(lr, nil)
	repo.On("SaveLifecycle", mock.Anything, lr).Return(nil)

	err := svc.AdvancePhase(context.Background(), "p1", PhaseExamination, "Reason")
	assert.NoError(t, err)
	assert.Equal(t, PhaseExamination, lr.CurrentPhase)
}

func TestProcessDailyMaintenance_Success(t *testing.T) {
	repo := new(MockLifecycleRepository)
	annuitySvc := new(MockAnnuityService)
	deadlineSvc := new(MockDeadlineService)
	reg := new(MockJurisdictionRegistry)
	svc := NewLifecycleService(repo, annuitySvc, deadlineSvc, reg)

	annuitySvc.On("CheckOverdue", mock.Anything, mock.Anything).Return([]*AnnuityRecord{}, nil)
	deadlineSvc.On("GenerateReminderBatch", mock.Anything, mock.Anything).Return([]*DeadlineReminder{}, nil)
	repo.On("GetExpiringLifecycles", mock.Anything, 0).Return([]*LifecycleRecord{}, nil)

	report, err := svc.ProcessDailyMaintenance(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Empty(t, report.Errors)
}

func TestGetCostAnalysis_Success(t *testing.T) {
	repo := new(MockLifecycleRepository)
	annuitySvc := new(MockAnnuityService)
	deadlineSvc := new(MockDeadlineService)
	reg := new(MockJurisdictionRegistry)
	svc := NewLifecycleService(repo, annuitySvc, deadlineSvc, reg)

	forecast := &AnnuityCostForecast{
		TotalForecastCost: NewMoney(100, "USD"),
		YearlyCosts: map[int]Money{2025: NewMoney(100, "USD")},
	}
	annuitySvc.On("ForecastCosts", mock.Anything, "port1", 5).Return(forecast, nil)

	analysis, err := svc.GetCostAnalysis(context.Background(), "port1", 5)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), analysis.TotalForecast)
	assert.Equal(t, int64(100), analysis.ForecastCosts[2025])
}

//Personal.AI order the ending
