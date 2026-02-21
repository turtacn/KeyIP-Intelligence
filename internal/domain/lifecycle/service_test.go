package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockLifecycleRepository struct {
	mock.Mock
}

func (m *mockLifecycleRepository) Save(ctx context.Context, record *LifecycleRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *mockLifecycleRepository) FindByID(ctx context.Context, id string) (*LifecycleRecord, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*LifecycleRecord), args.Error(1)
}

func (m *mockLifecycleRepository) FindByPatentID(ctx context.Context, patentID string) (*LifecycleRecord, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LifecycleRecord), args.Error(1)
}

func (m *mockLifecycleRepository) FindByPhase(ctx context.Context, phase LifecyclePhase, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error) {
	args := m.Called(ctx, phase)
	return args.Get(0).([]*LifecycleRecord), args.Error(1)
}

func (m *mockLifecycleRepository) FindExpiring(ctx context.Context, withinDays int) ([]*LifecycleRecord, error) {
	args := m.Called(ctx, withinDays)
	return args.Get(0).([]*LifecycleRecord), args.Error(1)
}

func (m *mockLifecycleRepository) FindByJurisdiction(ctx context.Context, jurisdictionCode string, opts ...LifecycleQueryOption) ([]*LifecycleRecord, error) {
	args := m.Called(ctx, jurisdictionCode)
	return args.Get(0).([]*LifecycleRecord), args.Error(1)
}

func (m *mockLifecycleRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockLifecycleRepository) Count(ctx context.Context, phase LifecyclePhase) (int64, error) {
	args := m.Called(ctx, phase)
	return args.Get(0).(int64), args.Error(1)
}

type mockAnnuityService struct {
	mock.Mock
}

func (m *mockAnnuityService) GenerateSchedule(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time, maxYears int) (*AnnuitySchedule, error) {
	args := m.Called(ctx, patentID, jurisdictionCode, filingDate, maxYears)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AnnuitySchedule), args.Error(1)
}

func (m *mockAnnuityService) CalculateAnnuityFee(ctx context.Context, jurisdictionCode string, yearNumber int) (Money, error) {
	args := m.Called(ctx, jurisdictionCode, yearNumber)
	return args.Get(0).(Money), args.Error(1)
}

func (m *mockAnnuityService) MarkAsPaid(ctx context.Context, recordID string, paidAmount Money, paidDate time.Time) error {
	args := m.Called(ctx, recordID, paidAmount, paidDate)
	return args.Error(0)
}

func (m *mockAnnuityService) MarkAsAbandoned(ctx context.Context, recordID string, reason string) error {
	args := m.Called(ctx, recordID, reason)
	return args.Error(0)
}

func (m *mockAnnuityService) CheckOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, asOfDate)
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *mockAnnuityService) ForecastCosts(ctx context.Context, portfolioID string, years int) (*AnnuityCostForecast, error) {
	args := m.Called(ctx, portfolioID, years)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AnnuityCostForecast), args.Error(1)
}

func (m *mockAnnuityService) GetUpcomingPayments(ctx context.Context, portfolioID string, withinDays int) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, portfolioID, withinDays)
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

type mockDeadlineService struct {
	mock.Mock
}

func (m *mockDeadlineService) CreateDeadline(ctx context.Context, patentID string, deadlineType DeadlineType, title string, dueDate time.Time) (*Deadline, error) {
	args := m.Called(ctx, patentID, deadlineType, title, dueDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

func (m *mockDeadlineService) CompleteDeadline(ctx context.Context, deadlineID, completedBy string) error {
	args := m.Called(ctx, deadlineID, completedBy)
	return args.Error(0)
}

func (m *mockDeadlineService) ExtendDeadline(ctx context.Context, deadlineID string, extensionDays int) error {
	args := m.Called(ctx, deadlineID, extensionDays)
	return args.Error(0)
}

func (m *mockDeadlineService) GetCalendar(ctx context.Context, ownerID string, from, to time.Time) (*DeadlineCalendar, error) {
	args := m.Called(ctx, ownerID, from, to)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*DeadlineCalendar), args.Error(1)
}

func (m *mockDeadlineService) GetOverdueDeadlines(ctx context.Context, ownerID string) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineService) GetUpcomingDeadlines(ctx context.Context, ownerID string, withinDays int) ([]*Deadline, error) {
	args := m.Called(ctx, ownerID, withinDays)
	return args.Get(0).([]*Deadline), args.Error(1)
}

func (m *mockDeadlineService) RefreshUrgencies(ctx context.Context, ownerID string) error {
	args := m.Called(ctx, ownerID)
	return args.Error(0)
}

func (m *mockDeadlineService) GenerateReminderBatch(ctx context.Context, asOf time.Time) ([]*DeadlineReminder, error) {
	args := m.Called(ctx, asOf)
	return args.Get(0).([]*DeadlineReminder), args.Error(1)
}

func (m *mockDeadlineService) AddCustomDeadline(ctx context.Context, patentID, title, description string, dueDate time.Time) (*Deadline, error) {
	args := m.Called(ctx, patentID, title, description, dueDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Deadline), args.Error(1)
}

type mockJurisdictionRegistry struct {
	mock.Mock
}

func (m *mockJurisdictionRegistry) Get(code string) (*Jurisdiction, error) {
	args := m.Called(code)
	return args.Get(0).(*Jurisdiction), args.Error(1)
}

func (m *mockJurisdictionRegistry) List() []*Jurisdiction {
	args := m.Called()
	return args.Get(0).([]*Jurisdiction)
}

func (m *mockJurisdictionRegistry) IsSupported(code string) bool {
	args := m.Called(code)
	return args.Bool(0)
}

func (m *mockJurisdictionRegistry) GetPatentTerm(code string, patentType string) (int, error) {
	args := m.Called(code, patentType)
	return args.Int(0), args.Error(1)
}

func (m *mockJurisdictionRegistry) GetAnnuityRules(code string) (*AnnuityRules, error) {
	args := m.Called(code)
	return args.Get(0).(*AnnuityRules), args.Error(1)
}

func (m *mockJurisdictionRegistry) GetOAResponseRules(code string) (*OAResponseRules, error) {
	args := m.Called(code)
	return args.Get(0).(*OAResponseRules), args.Error(1)
}

func TestInitializeLifecycle_Success(t *testing.T) {
	lRepo := new(mockLifecycleRepository)
	aRepo := new(mockAnnuityRepository)
	dRepo := new(mockDeadlineRepository)
	aSvc := new(mockAnnuityService)
	dSvc := new(mockDeadlineService)
	jReg := new(mockJurisdictionRegistry)
	s := NewLifecycleService(lRepo, aRepo, dRepo, aSvc, dSvc, jReg)

	patentID := "pat1"
	jurisdiction := "CN"
	filingDate := time.Now().UTC()

	jReg.On("IsSupported", jurisdiction).Return(true)
	aSvc.On("GenerateSchedule", mock.Anything, patentID, jurisdiction, filingDate, 20).Return(&AnnuitySchedule{Records: []*AnnuityRecord{}}, nil)
	aRepo.On("SaveBatch", mock.Anything, mock.Anything).Return(nil)
	dSvc.On("CreateDeadline", mock.Anything, patentID, mock.Anything, mock.Anything, mock.Anything).Return(&Deadline{}, nil)
	lRepo.On("Save", mock.Anything, mock.Anything).Return(nil)

	lr, err := s.InitializeLifecycle(context.Background(), patentID, jurisdiction, filingDate)
	assert.NoError(t, err)
	assert.NotNil(t, lr)
	jReg.AssertExpectations(t)
	lRepo.AssertExpectations(t)
}

func TestAdvancePhase_ToGranted(t *testing.T) {
	lRepo := new(mockLifecycleRepository)
	s := NewLifecycleService(lRepo, nil, nil, nil, nil, nil)

	lr, _ := NewLifecycleRecord("pat1", "CN", time.Now())
	_ = lr.TransitionTo(PhaseExamination, "Exam", "user1")
	lRepo.On("FindByPatentID", mock.Anything, "pat1").Return(lr, nil)
	lRepo.On("Save", mock.Anything, lr).Return(nil)

	err := s.AdvancePhase(context.Background(), "pat1", PhaseGranted, "Granted by examiner")
	assert.NoError(t, err)
	assert.Equal(t, PhaseGranted, lr.CurrentPhase)
	assert.NotNil(t, lr.GrantDate)
}

func TestProcessDailyMaintenance_Success(t *testing.T) {
	lRepo := new(mockLifecycleRepository)
	aSvc := new(mockAnnuityService)
	dSvc := new(mockDeadlineService)
	s := NewLifecycleService(lRepo, nil, nil, aSvc, dSvc, nil)

	aSvc.On("CheckOverdue", mock.Anything, mock.Anything).Return([]*AnnuityRecord{}, nil)
	dSvc.On("RefreshUrgencies", mock.Anything, "").Return(nil)
	dSvc.On("GenerateReminderBatch", mock.Anything, mock.Anything).Return([]*DeadlineReminder{}, nil)
	lRepo.On("FindExpiring", mock.Anything, 0).Return([]*LifecycleRecord{}, nil)

	report, err := s.ProcessDailyMaintenance(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 0, len(report.Errors))
}

//Personal.AI order the ending
