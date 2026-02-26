package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
)

// MockDeadlineService is a mock implementation of DeadlineService
// matching the actual lifecycle.DeadlineService interface
type MockDeadlineService struct {
	mock.Mock
}

func (m *MockDeadlineService) ListDeadlines(ctx context.Context, query *lifecycle.DeadlineQuery) (*lifecycle.DeadlineListResponse, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.DeadlineListResponse), args.Error(1)
}

func (m *MockDeadlineService) CreateDeadline(ctx context.Context, req *lifecycle.CreateDeadlineRequest) (*lifecycle.Deadline, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.Deadline), args.Error(1)
}

func (m *MockDeadlineService) CompleteDeadline(ctx context.Context, deadlineID string) error {
	args := m.Called(ctx, deadlineID)
	return args.Error(0)
}

func (m *MockDeadlineService) ExtendDeadline(ctx context.Context, req *lifecycle.ExtendDeadlineRequest) (*lifecycle.Deadline, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.Deadline), args.Error(1)
}

func (m *MockDeadlineService) DeleteDeadline(ctx context.Context, deadlineID string) error {
	args := m.Called(ctx, deadlineID)
	return args.Error(0)
}

func (m *MockDeadlineService) GetComplianceDashboard(ctx context.Context, portfolioID string) (*lifecycle.ComplianceDashboard, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.ComplianceDashboard), args.Error(1)
}

func (m *MockDeadlineService) GetOverdueDeadlines(ctx context.Context, portfolioID string) ([]lifecycle.Deadline, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]lifecycle.Deadline), args.Error(1)
}

func (m *MockDeadlineService) SyncStatutoryDeadlines(ctx context.Context, patentID string) (int, error) {
	args := m.Called(ctx, patentID)
	return args.Int(0), args.Error(1)
}

// MockAnnuityService is a mock implementation of AnnuityService
// matching the actual lifecycle.AnnuityService interface
type MockAnnuityService struct {
	mock.Mock
}

func (m *MockAnnuityService) CalculateAnnuity(ctx context.Context, req *lifecycle.CalculateAnnuityRequest) (*lifecycle.AnnuityResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.AnnuityResult), args.Error(1)
}

func (m *MockAnnuityService) BatchCalculate(ctx context.Context, req *lifecycle.BatchCalculateRequest) (*lifecycle.BatchCalculateResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.BatchCalculateResponse), args.Error(1)
}

func (m *MockAnnuityService) GenerateBudgetReport(ctx context.Context, req *lifecycle.GenerateBudgetRequest) (*lifecycle.BudgetReport, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.BudgetReport), args.Error(1)
}

func (m *MockAnnuityService) GetPaymentSchedule(ctx context.Context, req *lifecycle.PaymentScheduleRequest) ([]lifecycle.PaymentScheduleEntry, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]lifecycle.PaymentScheduleEntry), args.Error(1)
}

func (m *MockAnnuityService) OptimizeCosts(ctx context.Context, req *lifecycle.OptimizeCostsRequest) (*lifecycle.CostOptimizationReport, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.CostOptimizationReport), args.Error(1)
}

func (m *MockAnnuityService) RecordPayment(ctx context.Context, req *lifecycle.RecordPaymentRequest) (*lifecycle.PaymentRecord, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.PaymentRecord), args.Error(1)
}

func (m *MockAnnuityService) GetPaymentHistory(ctx context.Context, req *lifecycle.PaymentHistoryRequest) ([]lifecycle.PaymentRecord, int64, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]lifecycle.PaymentRecord), int64(args.Int(1)), args.Error(2)
}

func (m *MockAnnuityService) GenerateBudget(ctx context.Context, req *lifecycle.GenerateBudgetRequest) (*lifecycle.BudgetReport, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.BudgetReport), args.Error(1)
}

// MockLegalStatusService is a mock implementation of LegalStatusService
// matching the actual lifecycle.LegalStatusService interface
type MockLegalStatusService struct {
	mock.Mock
}

func (m *MockLegalStatusService) SyncStatus(ctx context.Context, patentID string) (*lifecycle.SyncResult, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.SyncResult), args.Error(1)
}

func (m *MockLegalStatusService) BatchSync(ctx context.Context, req *lifecycle.BatchSyncRequest) (*lifecycle.BatchSyncResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.BatchSyncResult), args.Error(1)
}

func (m *MockLegalStatusService) GetCurrentStatus(ctx context.Context, patentID string) (*lifecycle.LegalStatusDetail, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.LegalStatusDetail), args.Error(1)
}

func (m *MockLegalStatusService) GetStatusHistory(ctx context.Context, patentID string, opts ...lifecycle.QueryOption) ([]*lifecycle.LegalStatusEvent, error) {
	args := m.Called(ctx, patentID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lifecycle.LegalStatusEvent), args.Error(1)
}

func (m *MockLegalStatusService) DetectAnomalies(ctx context.Context, portfolioID string) ([]*lifecycle.StatusAnomaly, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lifecycle.StatusAnomaly), args.Error(1)
}

func (m *MockLegalStatusService) SubscribeStatusChange(ctx context.Context, req *lifecycle.SubscriptionRequest) (*lifecycle.Subscription, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.Subscription), args.Error(1)
}

func (m *MockLegalStatusService) UnsubscribeStatusChange(ctx context.Context, subscriptionID string) error {
	args := m.Called(ctx, subscriptionID)
	return args.Error(0)
}

func (m *MockLegalStatusService) GetStatusSummary(ctx context.Context, portfolioID string) (*lifecycle.StatusSummary, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.StatusSummary), args.Error(1)
}

func (m *MockLegalStatusService) ReconcileStatus(ctx context.Context, patentID string) (*lifecycle.ReconcileResult, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.ReconcileResult), args.Error(1)
}

// MockCalendarService is a mock implementation of CalendarService
// matching the actual lifecycle.CalendarService interface
type MockCalendarService struct {
	mock.Mock
}

func (m *MockCalendarService) GetCalendarView(ctx context.Context, req *lifecycle.CalendarViewRequest) (*lifecycle.CalendarView, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.CalendarView), args.Error(1)
}

func (m *MockCalendarService) AddEvent(ctx context.Context, req *lifecycle.AddEventRequest) (*lifecycle.CalendarEvent, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.CalendarEvent), args.Error(1)
}

func (m *MockCalendarService) UpdateEventStatus(ctx context.Context, eventID string, status lifecycle.EventStatus) error {
	args := m.Called(ctx, eventID, status)
	return args.Error(0)
}

func (m *MockCalendarService) DeleteEvent(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func (m *MockCalendarService) ExportICal(ctx context.Context, req *lifecycle.ICalExportRequest) ([]byte, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCalendarService) GetUpcomingDeadlines(ctx context.Context, portfolioID string, withinDays int) ([]lifecycle.CalendarEvent, error) {
	args := m.Called(ctx, portfolioID, withinDays)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]lifecycle.CalendarEvent), args.Error(1)
}

func TestValidateJurisdictions(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{"Valid single", "CN", false},
		{"Valid multiple", "CN,US,EP", false},
		{"Valid lowercase", "cn,us", false},
		{"Valid with spaces", " CN , US ", false},
		{"Invalid jurisdiction", "XX", true},
		{"Mixed valid/invalid", "CN,XX", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJurisdictions(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseJurisdictions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Empty", "", nil},
		{"Single", "CN", []string{"CN"}},
		{"Multiple", "CN,US,EP", []string{"CN", "US", "EP"}},
		{"With spaces", " cn , us ", []string{"CN", "US"}},
		{"Trailing comma", "CN,US,", []string{"CN", "US"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseJurisdictions(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseChannels(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"Single", "email", []string{"email"}},
		{"Multiple", "email,wechat,sms", []string{"email", "wechat", "sms"}},
		{"With spaces", " EMAIL , WeChat ", []string{"email", "wechat"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseChannels(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAdvanceDays(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{"Single", "30", []int{30}},
		{"Multiple", "30,60,90", []int{30, 60, 90}},
		{"With spaces", " 30 , 60 ", []int{30, 60}},
		{"Invalid mixed", "30,abc,60", []int{30, 60}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAdvanceDays(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSortDeadlinesByUrgency(t *testing.T) {
	now := time.Now()

	// Use actual DeadlineUrgency constants from lifecycle package
	deadlines := []lifecycle.Deadline{
		{Urgency: lifecycle.UrgencyNormal, DueDate: now.AddDate(0, 0, 10)},
		{Urgency: lifecycle.UrgencyCritical, DueDate: now.AddDate(0, 0, 5)},
		{Urgency: lifecycle.UrgencyUrgent, DueDate: now.AddDate(0, 0, 7)},
		{Urgency: lifecycle.UrgencyCritical, DueDate: now.AddDate(0, 0, 3)},
	}

	sortDeadlinesByUrgency(deadlines)

	// Verify critical first (sorted by date)
	assert.Equal(t, lifecycle.UrgencyCritical, deadlines[0].Urgency)
	assert.True(t, deadlines[0].DueDate.Before(deadlines[1].DueDate))

	// Verify urgent in middle
	assert.Equal(t, lifecycle.UrgencyUrgent, deadlines[2].Urgency)

	// Verify normal last
	assert.Equal(t, lifecycle.UrgencyNormal, deadlines[3].Urgency)
}

func TestFormatDeadlineTable_EmptyList(t *testing.T) {
	output := formatDeadlineTable([]lifecycle.Deadline{})
	assert.Contains(t, output, "No upcoming deadlines found")
}

func TestFormatDeadlineTable_WithDeadlines(t *testing.T) {
	now := time.Now()

	deadlines := []lifecycle.Deadline{
		{
			Urgency:      lifecycle.UrgencyCritical,
			PatentNumber: "CN115123456",
			DeadlineType: lifecycle.DeadlineTypeAnnuityPayment,
			DueDate:      now.AddDate(0, 0, 5),
		},
		{
			Urgency:      lifecycle.UrgencyNormal,
			PatentNumber: "US11987654",
			DeadlineType: lifecycle.DeadlineTypeOfficeAction,
			DueDate:      now.AddDate(0, 0, 60),
		},
	}

	output := formatDeadlineTable(deadlines)

	assert.Contains(t, output, "Upcoming Patent Deadlines")
	assert.Contains(t, output, "CN115123456")
	assert.Contains(t, output, "US11987654")
	assert.Contains(t, output, "critical")
	assert.Contains(t, output, "Total deadlines: 2")
}

func TestFormatAnnuityTable(t *testing.T) {
	now := time.Now()

	details := []*lifecycle.AnnuityDetail{
		{
			YearNumber:   2025,
			PatentNumber: "CN115123456",
			DueDate:      now.AddDate(0, 3, 0),
			BaseFee:      lifecycle.MoneyAmount{Amount: 1200.00, Currency: "CNY"},
			Status:       lifecycle.AnnuityStatusPending,
		},
		{
			YearNumber:   2026,
			PatentNumber: "CN115123456",
			DueDate:      now.AddDate(1, 3, 0),
			BaseFee:      lifecycle.MoneyAmount{Amount: 1500.00, Currency: "CNY"},
			Status:       lifecycle.AnnuityStatusPending,
		},
	}

	output := formatAnnuityTable(details, "CNY")

	assert.Contains(t, output, "Patent Annuity Fees")
	assert.Contains(t, output, "2025")
	assert.Contains(t, output, "2026")
	assert.Contains(t, output, "CNY 1200.00")
	assert.Contains(t, output, "CNY 1500.00")
}

func TestCountByUrgencyValue(t *testing.T) {
	deadlines := []lifecycle.Deadline{
		{Urgency: lifecycle.UrgencyCritical},
		{Urgency: lifecycle.UrgencyCritical},
		{Urgency: lifecycle.UrgencyUrgent},
		{Urgency: lifecycle.UrgencyNormal},
	}

	assert.Equal(t, 2, countByUrgencyValue(deadlines, lifecycle.UrgencyCritical))
	assert.Equal(t, 1, countByUrgencyValue(deadlines, lifecycle.UrgencyUrgent))
	assert.Equal(t, 1, countByUrgencyValue(deadlines, lifecycle.UrgencyNormal))
	assert.Equal(t, 0, countByUrgencyValue(deadlines, lifecycle.UrgencyExpired))
}

func TestContains(t *testing.T) {
	slice := []string{"email", "wechat", "sms"}

	assert.True(t, contains(slice, "email"))
	assert.True(t, contains(slice, "wechat"))
	assert.False(t, contains(slice, "telegram"))
}

func TestLifecycleDeadlinesCmd_InvalidDaysAhead(t *testing.T) {
	mockService := new(MockDeadlineService)
	mockLogger := new(MockLogger)

	cmd := NewLifecycleCmd(mockService, nil, nil, nil, mockLogger)

	// Reset and set invalid days-ahead via flag
	cmd.SetArgs([]string{"deadlines", "--days-ahead", "500"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be between 1 and 365")
}

func TestLifecycleAnnuityCmd_InvalidCurrency(t *testing.T) {
	mockService := new(MockAnnuityService)
	mockLogger := new(MockLogger)

	cmd := NewLifecycleCmd(nil, mockService, nil, nil, mockLogger)

	// Execute annuity subcommand with invalid currency
	cmd.SetArgs([]string{"annuity", "--patent-number", "CN115123456", "--currency", "BTC"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid currency")
}

func TestLifecycleSyncStatusCmd_DryRun(t *testing.T) {
	mockService := new(MockLegalStatusService)
	mockLogger := new(MockLogger)

	syncResult := &lifecycle.SyncResult{
		PatentID:       "CN115123456",
		PreviousStatus: "pending",
		CurrentStatus:  "granted",
		Changed:        true,
		SyncedAt:       time.Now(),
		Source:         "CNIPA",
	}

	// SyncStatus takes patentID string, not a request struct
	mockService.On("SyncStatus", mock.Anything, "CN115123456").Return(syncResult, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	lifecycleDryRun = true
	lifecyclePatentNumber = "CN115123456"

	err := runLifecycleSyncStatus(ctx, mockService, mockLogger)
	require.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestLifecycleRemindersCmd_InvalidAction(t *testing.T) {
	mockService := new(MockCalendarService)
	mockLogger := new(MockLogger)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	lifecycleAction = "invalid"

	err := runLifecycleReminders(ctx, mockService, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid action")
}

func TestLifecycleRemindersCmd_List(t *testing.T) {
	mockService := new(MockCalendarService)
	mockLogger := new(MockLogger)

	events := []lifecycle.CalendarEvent{
		{
			ID:        "event-001",
			Title:     "Annuity Payment Due",
			DueDate:   time.Now().AddDate(0, 0, 30),
			EventType: lifecycle.EventTypeAnnuityDue,
			Status:    lifecycle.EventStatusUpcoming,
		},
	}

	// listReminders uses GetUpcomingDeadlines, not ListReminders
	mockService.On("GetUpcomingDeadlines", mock.Anything, "", 30).Return(events, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	lifecycleAction = "list"
	lifecyclePatentNumber = ""
	lifecycleDaysAhead = 30

	err := listReminders(ctx, mockService, mockLogger)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}
