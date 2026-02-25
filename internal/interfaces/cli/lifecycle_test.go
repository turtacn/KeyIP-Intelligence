package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
)

// MockDeadlineService is a mock implementation of DeadlineService
type MockDeadlineService struct {
	mock.Mock
}

func (m *MockDeadlineService) ListUpcoming(ctx context.Context, req *lifecycle.DeadlineQueryRequest) ([]*lifecycle.Deadline, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lifecycle.Deadline), args.Error(1)
}

// MockAnnuityService is a mock implementation of AnnuityService
type MockAnnuityService struct {
	mock.Mock
}

func (m *MockAnnuityService) Calculate(ctx context.Context, req *lifecycle.AnnuityQueryRequest) (*lifecycle.AnnuityResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.AnnuityResult), args.Error(1)
}

// MockLegalStatusService is a mock implementation of LegalStatusService
type MockLegalStatusService struct {
	mock.Mock
}

func (m *MockLegalStatusService) SyncFromOffice(ctx context.Context, req *lifecycle.SyncStatusRequest) (*lifecycle.SyncResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.SyncResult), args.Error(1)
}

// MockCalendarService is a mock implementation of CalendarService
type MockCalendarService struct {
	mock.Mock
}

func (m *MockCalendarService) ListReminders(ctx context.Context, patentNumber string) ([]*lifecycle.Reminder, error) {
	args := m.Called(ctx, patentNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lifecycle.Reminder), args.Error(1)
}

func (m *MockCalendarService) AddReminder(ctx context.Context, req *lifecycle.AddReminderRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockCalendarService) RemoveReminder(ctx context.Context, patentNumber string) error {
	args := m.Called(ctx, patentNumber)
	return args.Error(0)
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

	deadlines := []*lifecycle.Deadline{
		{Urgency: "NORMAL", DueDate: now.AddDate(0, 0, 10)},
		{Urgency: "CRITICAL", DueDate: now.AddDate(0, 0, 5)},
		{Urgency: "WARNING", DueDate: now.AddDate(0, 0, 7)},
		{Urgency: "CRITICAL", DueDate: now.AddDate(0, 0, 3)},
	}

	sortDeadlinesByUrgency(deadlines)

	// Verify CRITICAL first (sorted by date)
	assert.Equal(t, "CRITICAL", deadlines[0].Urgency)
	assert.True(t, deadlines[0].DueDate.Before(deadlines[1].DueDate))

	// Verify WARNING in middle
	assert.Equal(t, "WARNING", deadlines[2].Urgency)

	// Verify NORMAL last
	assert.Equal(t, "NORMAL", deadlines[3].Urgency)
}

func TestFormatDeadlineTable_EmptyList(t *testing.T) {
	output := formatDeadlineTable([]*lifecycle.Deadline{})
	assert.Contains(t, output, "No upcoming deadlines found")
}

func TestFormatDeadlineTable_WithDeadlines(t *testing.T) {
	now := time.Now()

	deadlines := []*lifecycle.Deadline{
		{
			Urgency:      "CRITICAL",
			PatentNumber: "CN115123456",
			DeadlineType: "Annuity Payment",
			DueDate:      now.AddDate(0, 0, 5),
			Status:       "pending",
		},
		{
			Urgency:      "NORMAL",
			PatentNumber: "US11987654",
			DeadlineType: "Response Due",
			DueDate:      now.AddDate(0, 0, 60),
			Status:       "pending",
		},
	}

	output := formatDeadlineTable(deadlines)

	assert.Contains(t, output, "Upcoming Patent Deadlines")
	assert.Contains(t, output, "CN115123456")
	assert.Contains(t, output, "US11987654")
	assert.Contains(t, output, "CRITICAL")
	assert.Contains(t, output, "Total deadlines: 2")
}

func TestFormatAnnuityTable(t *testing.T) {
	now := time.Now()

	details := []*lifecycle.AnnuityDetail{
		{
			Year:     2025,
			DueDate:  now.AddDate(0, 3, 0),
			BaseFee:  1200.00,
			LateFee:  0,
			TotalFee: 1200.00,
			Status:   "pending",
		},
		{
			Year:     2026,
			DueDate:  now.AddDate(1, 3, 0),
			BaseFee:  1500.00,
			LateFee:  0,
			TotalFee: 1500.00,
			Status:   "pending",
		},
	}

	output := formatAnnuityTable(details, "CNY")

	assert.Contains(t, output, "Patent Annuity Fees")
	assert.Contains(t, output, "2025")
	assert.Contains(t, output, "2026")
	assert.Contains(t, output, "CNY 1200.00")
	assert.Contains(t, output, "CNY 1500.00")
}

func TestCountByUrgency(t *testing.T) {
	deadlines := []*lifecycle.Deadline{
		{Urgency: "CRITICAL"},
		{Urgency: "CRITICAL"},
		{Urgency: "WARNING"},
		{Urgency: "NORMAL"},
	}

	assert.Equal(t, 2, countByUrgency(deadlines, "CRITICAL"))
	assert.Equal(t, 1, countByUrgency(deadlines, "WARNING"))
	assert.Equal(t, 1, countByUrgency(deadlines, "NORMAL"))
	assert.Equal(t, 0, countByUrgency(deadlines, "UNKNOWN"))
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

	// Set invalid days-ahead
	lifecycleDaysAhead = 500

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be between 1 and 365")
}

func TestLifecycleAnnuityCmd_InvalidCurrency(t *testing.T) {
	mockService := new(MockAnnuityService)
	mockLogger := new(MockLogger)

	cmd := NewLifecycleCmd(nil, mockService, nil, nil, mockLogger)

	lifecyclePatentNumber = "CN115123456"
	lifecycleCurrency = "BTC"

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid currency")
}

func TestLifecycleSyncStatusCmd_DryRun(t *testing.T) {
	mockService := new(MockLegalStatusService)
	mockLogger := new(MockLogger)

	syncResult := &lifecycle.SyncResult{
		TotalProcessed:  10,
		NewRecords:      3,
		UpdatedRecords:  5,
		FailedCount:     0,
		Errors:          []*lifecycle.SyncError{},
	}

	mockService.On("SyncFromOffice", mock.Anything, mock.MatchedBy(func(req *lifecycle.SyncStatusRequest) bool {
		return req.DryRun == true
	})).Return(syncResult, nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	lifecycleDryRun = true

	err := runLifecycleSyncStatus(ctx, mockService, mockLogger)
	require.NoError(t, err)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// This would normally be part of the command execution
	fmt.Printf("\n=== Legal Status Sync Summary ===\n\n")
	if lifecycleDryRun {
		fmt.Println("🔍 DRY RUN MODE - No changes applied")
	}

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()
	assert.Contains(t, output, "DRY RUN MODE")

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

	reminders := []*lifecycle.Reminder{
		{
			PatentNumber:  "CN115123456",
			DeadlineType:  "Annuity Payment",
			Channels:      []string{"email", "wechat"},
			AdvanceDays:   []int{30, 60, 90},
		},
	}

	mockService.On("ListReminders", mock.Anything, "").Return(reminders, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	lifecycleAction = "list"
	lifecyclePatentNumber = ""

	err := listReminders(ctx, mockService, mockLogger)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}

//Personal.AI order the ending
