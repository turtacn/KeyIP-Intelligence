package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MockAnnuityService
type MockAnnuityService struct {
	mock.Mock
}

func (m *MockAnnuityService) CalculateFee(ctx context.Context, jurisdiction Jurisdiction, year int, filingDate, grantDate *time.Time) (*AnnuityCalcResult, error) {
	args := m.Called(ctx, jurisdiction, year, filingDate, grantDate)
	return args.Get(0).(*AnnuityCalcResult), args.Error(1)
}

func (m *MockAnnuityService) GetSchedule(ctx context.Context, jurisdiction Jurisdiction, filingDate, grantDate *time.Time) ([]ScheduleEntry, error) {
	args := m.Called(ctx, jurisdiction, filingDate, grantDate)
	return args.Get(0).([]ScheduleEntry), args.Error(1)
}

// MockDeadlineService
type MockDeadlineService struct {
	mock.Mock
}

func (m *MockDeadlineService) CalculateDeadlines(ctx context.Context, eventType EventType, eventDate time.Time, jurisdiction Jurisdiction) ([]*Deadline, error) {
	args := m.Called(ctx, eventType, eventDate, jurisdiction)
	return args.Get(0).([]*Deadline), args.Error(1)
}

// MockJurisdictionRegistry
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

func (m *MockJurisdictionRegistry) Normalize(code string) (Jurisdiction, error) {
	args := m.Called(code)
	return args.Get(0).(Jurisdiction), args.Error(1)
}

func (m *MockJurisdictionRegistry) List() []*JurisdictionInfo {
	args := m.Called()
	return args.Get(0).([]*JurisdictionInfo)
}

func TestProcessDailyMaintenance(t *testing.T) {
	mockRepo := new(MockLifecycleRepository)
	mockAnnuity := new(MockAnnuityService)
	mockDeadline := new(MockDeadlineService)
	mockJurisdiction := new(MockJurisdictionRegistry)

	service := NewService(mockRepo, mockAnnuity, mockDeadline, mockJurisdiction)

	// Mock GetUpcomingAnnuities
	annuity := &Annuity{
		ID:      "ann-1",
		Status:  AnnuityStatusUpcoming,
		DueDate: time.Now().AddDate(0, 0, 30),
	}
	mockRepo.On("GetUpcomingAnnuities", mock.Anything, 90, 100, 0).Return([]*Annuity{annuity}, int64(1), nil)

	// Mock UpdateAnnuityStatus
	mockRepo.On("UpdateAnnuityStatus", mock.Anything, "ann-1", mock.Anything, int64(0), (*time.Time)(nil), "").Return(nil)

	// Mock GetActiveDeadlines
	deadline := &Deadline{
		ID:      "dl-1",
		Status:  DeadlineStatusActive,
		DueDate: time.Now().AddDate(0, 0, 5), // Critical
	}
	mockRepo.On("GetActiveDeadlines", mock.Anything, (*string)(nil), 30, 100, 0).Return([]*Deadline{deadline}, int64(1), nil)

	err := service.ProcessDailyMaintenance(context.Background())
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestProcessDailyMaintenance_WithErrors(t *testing.T) {
	mockRepo := new(MockLifecycleRepository)
	mockAnnuity := new(MockAnnuityService)
	mockDeadline := new(MockDeadlineService)
	mockJurisdiction := new(MockJurisdictionRegistry)

	service := NewService(mockRepo, mockAnnuity, mockDeadline, mockJurisdiction)

	// Mock error in fetching annuities
	mockRepo.On("GetUpcomingAnnuities", mock.Anything, 90, 100, 0).Return([]*Annuity{}, int64(0), errors.New(errors.ErrCodeDatabaseError, "db error"))

	// Mock error in fetching deadlines
	mockRepo.On("GetActiveDeadlines", mock.Anything, (*string)(nil), 30, 100, 0).Return([]*Deadline{}, int64(0), errors.New(errors.ErrCodeDatabaseError, "db error"))

	err := service.ProcessDailyMaintenance(context.Background())
	assert.Error(t, err)
	// Should contain both errors (implied by Join)
}

func TestCheckHealth(t *testing.T) {
	mockRepo := new(MockLifecycleRepository)
	mockAnnuity := new(MockAnnuityService)
	mockDeadline := new(MockDeadlineService)
	mockJurisdiction := new(MockJurisdictionRegistry)

	service := NewService(mockRepo, mockAnnuity, mockDeadline, mockJurisdiction)

	// Case 1: Healthy
	mockRepo.On("GetAnnuitiesByPatent", mock.Anything, "pat-1").Return([]*Annuity{}, nil).Once()
	mockRepo.On("GetDeadlinesByPatent", mock.Anything, "pat-1", []DeadlineStatus{DeadlineStatusActive}).Return([]*Deadline{}, nil).Once()

	health, err := service.CheckHealth(context.Background(), "pat-1")
	assert.NoError(t, err)
	assert.Equal(t, "healthy", health)

	// Case 2: Critical (Overdue Annuity)
	overdueAnnuity := &Annuity{Status: AnnuityStatusOverdue}
	mockRepo.On("GetAnnuitiesByPatent", mock.Anything, "pat-2").Return([]*Annuity{overdueAnnuity}, nil).Once()

	health, err = service.CheckHealth(context.Background(), "pat-2")
	assert.NoError(t, err)
	assert.Equal(t, "critical", health)

	// Case 3: Warning (Critical Deadline)
	criticalDeadline := &Deadline{DueDate: time.Now().Add(24 * time.Hour), Status: DeadlineStatusActive}
	mockRepo.On("GetAnnuitiesByPatent", mock.Anything, "pat-3").Return([]*Annuity{}, nil).Once()
	mockRepo.On("GetDeadlinesByPatent", mock.Anything, "pat-3", []DeadlineStatus{DeadlineStatusActive}).Return([]*Deadline{criticalDeadline}, nil).Once()

	health, err = service.CheckHealth(context.Background(), "pat-3")
	assert.NoError(t, err)
	assert.Equal(t, "warning", health)
}

//Personal.AI order the ending
