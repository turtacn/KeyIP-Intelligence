// internal/application/lifecycle/calendar_test.go
//
// Phase 10 - File #209
// Unit tests for CalendarService application service.

package lifecycle

import (
	"context"
	"strings"
	"testing"
	"time"

	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Mock implementations for calendar tests
// ---------------------------------------------------------------------------

// Mock implementations removed. Use shared mocks from common_test.go.

// ---------------------------------------------------------------------------
// Helper to build test calendar service
// ---------------------------------------------------------------------------

func newTestCalendarService(opts ...func(*testCalendarOpts)) CalendarService {
	o := &testCalendarOpts{
		lifecycleSvc:  &mockLifecycleService{},
		lifecycleRepo: &mockLifecycleRepo{},
		patentRepo:    newMockPatentRepo(),
		cache:         newMockCache(),
		logger:        &mockLogger{},
		cfg:           CalendarServiceConfig{DefaultTimezone: "Asia/Shanghai"},
	}

	// Set default patents
	fd := time.Now().AddDate(-3, 0, 0)
	o.patentRepo.patents["00000000-0000-0000-0000-000000000001"] = &mockPatentInfo{
		ID: "00000000-0000-0000-0000-000000000001", PatentNumber: "CN202310001234.5",
		Title: "Test Patent", Jurisdiction: "CN",
		FilingDate: fd,
	}

	for _, fn := range opts {
		fn(o)
	}
	return NewCalendarService(
		o.lifecycleSvc, o.lifecycleRepo, o.patentRepo,
		o.cache, o.logger, o.cfg,
	)
}

type testCalendarOpts struct {
	lifecycleSvc  domainLifecycle.Service
	lifecycleRepo domainLifecycle.LifecycleRepository
	patentRepo    *mockPatentRepo
	cache         common.CachePort
	logger        common.Logger
	cfg           CalendarServiceConfig
}

// ---------------------------------------------------------------------------
// Tests: GetCalendarView
// ---------------------------------------------------------------------------

func TestGetCalendarView_Success(t *testing.T) {
	svc := newTestCalendarService()
	ctx := context.Background()

	now := time.Now()
	view, err := svc.GetCalendarView(ctx, &CalendarViewRequest{
		PatentIDs: []string{"00000000-0000-0000-0000-000000000001"},
		StartDate: now.AddDate(-1, 0, 0),
		EndDate:   now.AddDate(5, 0, 0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view == nil {
		t.Fatal("expected non-nil view")
	}
	if view.TotalCount == 0 {
		t.Error("expected at least one event")
	}
	if len(view.ByType) == 0 {
		t.Error("expected ByType to be populated")
	}
}

func TestGetCalendarView_NilRequest(t *testing.T) {
	svc := newTestCalendarService()
	_, err := svc.GetCalendarView(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestGetCalendarView_InvalidDateRange(t *testing.T) {
	svc := newTestCalendarService()
	now := time.Now()
	_, err := svc.GetCalendarView(context.Background(), &CalendarViewRequest{
		PatentIDs: []string{"pat-001"},
		StartDate: now,
		EndDate:   now.AddDate(-1, 0, 0),
	})
	if err == nil {
		t.Fatal("expected error for end before start")
	}
}

func TestGetCalendarView_FilterByEventType(t *testing.T) {
	svc := newTestCalendarService()
	now := time.Now()

	view, err := svc.GetCalendarView(context.Background(), &CalendarViewRequest{
		PatentIDs:  []string{"pat-001"},
		StartDate:  now.AddDate(-1, 0, 0),
		EndDate:    now.AddDate(5, 0, 0),
		EventTypes: []CalendarEventType{EventTypeAnnuityDue},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, ev := range view.Events {
		if ev.EventType != EventTypeAnnuityDue {
			t.Errorf("expected only annuity_due events, got %s", ev.EventType)
		}
	}
}

func TestGetCalendarView_FilterByJurisdiction(t *testing.T) {
	svc := newTestCalendarService(func(o *testCalendarOpts) {
		fd := time.Now().AddDate(-2, 0, 0)
		o.patentRepo = newMockPatentRepo(
			&mockPatentInfo{ID: "00000000-0000-0000-0000-000000000001", PatentNumber: "CN001", Title: "CN Patent", Jurisdiction: "CN", FilingDate: fd},
			&mockPatentInfo{ID: "00000000-0000-0000-0000-000000000002", PatentNumber: "US001", Title: "US Patent", Jurisdiction: "US", FilingDate: fd},
		)
	})

	now := time.Now()
	view, err := svc.GetCalendarView(context.Background(), &CalendarViewRequest{
		PatentIDs:     []string{"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"},
		Jurisdictions: []domainLifecycle.Jurisdiction{domainLifecycle.JurisdictionCN},
		StartDate:     now.AddDate(-1, 0, 0),
		EndDate:       now.AddDate(5, 0, 0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, ev := range view.Events {
		if ev.Jurisdiction != domainLifecycle.JurisdictionCN {
			t.Errorf("expected only CN events, got %s", ev.Jurisdiction)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: AddEvent
// ---------------------------------------------------------------------------

func TestAddEvent_Success(t *testing.T) {
	var savedEvent *domainLifecycle.CustomEvent
	svc := newTestCalendarService(func(o *testCalendarOpts) {
		o.lifecycleRepo.(*mockLifecycleRepo).saveCustomEventFn = func(ctx context.Context, event *domainLifecycle.CustomEvent) error {
			savedEvent = event
			return nil
		}
	})

	event, err := svc.AddEvent(context.Background(), &AddEventRequest{
		PatentID: "00000000-0000-0000-0000-000000000001",
		Title:    "Custom Milestone",
		DueDate:  time.Now().AddDate(0, 3, 0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.EventType != EventTypeCustomMilestone {
		t.Errorf("expected custom_milestone, got %s", event.EventType)
	}
	if event.Priority != PriorityMedium {
		t.Errorf("expected medium priority, got %s", event.Priority)
	}
	if len(event.Reminders) == 0 {
		t.Error("expected default reminders to be set")
	}
	if savedEvent == nil {
		t.Error("expected event to be saved via repository")
	}
}

func TestAddEvent_NilRequest(t *testing.T) {
	svc := newTestCalendarService()
	_, err := svc.AddEvent(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAddEvent_MissingPatentID(t *testing.T) {
	svc := newTestCalendarService()
	_, err := svc.AddEvent(context.Background(), &AddEventRequest{
		Title:   "Test",
		DueDate: time.Now().AddDate(0, 1, 0),
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAddEvent_MissingTitle(t *testing.T) {
	svc := newTestCalendarService()
	_, err := svc.AddEvent(context.Background(), &AddEventRequest{
		PatentID: "pat-001",
		DueDate:  time.Now().AddDate(0, 1, 0),
	})
	if err == nil {
		t.Fatal("expected validation error for missing title")
	}
}

func TestAddEvent_PatentNotFound(t *testing.T) {
	svc := newTestCalendarService()
	_, err := svc.AddEvent(context.Background(), &AddEventRequest{
		PatentID: "nonexistent",
		Title:    "Test",
		DueDate:  time.Now().AddDate(0, 1, 0),
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

// ---------------------------------------------------------------------------
// Tests: UpdateEventStatus
// ---------------------------------------------------------------------------

func TestUpdateEventStatus_Success(t *testing.T) {
	svc := newTestCalendarService()
	err := svc.UpdateEventStatus(context.Background(), "evt-123", EventStatusCompleted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateEventStatus_EmptyID(t *testing.T) {
	svc := newTestCalendarService()
	err := svc.UpdateEventStatus(context.Background(), "", EventStatusCompleted)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestUpdateEventStatus_InvalidStatus(t *testing.T) {
	svc := newTestCalendarService()
	err := svc.UpdateEventStatus(context.Background(), "evt-123", EventStatus("bogus"))
	if err == nil {
		t.Fatal("expected validation error for invalid status")
	}
}

// ---------------------------------------------------------------------------
// Tests: DeleteEvent
// ---------------------------------------------------------------------------

func TestDeleteEvent_Success(t *testing.T) {
	svc := newTestCalendarService()
	err := svc.DeleteEvent(context.Background(), "evt-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteEvent_EmptyID(t *testing.T) {
	svc := newTestCalendarService()
	err := svc.DeleteEvent(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---------------------------------------------------------------------------
// Tests: ExportICal
// ---------------------------------------------------------------------------

func TestExportICal_Success(t *testing.T) {
	svc := newTestCalendarService()
	now := time.Now()

	data, err := svc.ExportICal(context.Background(), &ICalExportRequest{
		PatentIDs: []string{"00000000-0000-0000-0000-000000000001"},
		StartDate: now.AddDate(-1, 0, 0),
		EndDate:   now.AddDate(5, 0, 0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty iCal data")
	}
	content := string(data)
	if !strings.Contains(content, "BEGIN:VCALENDAR") {
		t.Error("expected VCALENDAR header")
	}
	if !strings.Contains(content, "END:VCALENDAR") {
		t.Error("expected VCALENDAR footer")
	}
	if !strings.Contains(content, "BEGIN:VEVENT") {
		t.Error("expected at least one VEVENT")
	}
}

func TestExportICal_NilRequest(t *testing.T) {
	svc := newTestCalendarService()
	_, err := svc.ExportICal(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetUpcomingDeadlines
// ---------------------------------------------------------------------------

func TestGetUpcomingDeadlines_Success(t *testing.T) {
	svc := newTestCalendarService()
	deadlines, err := svc.GetUpcomingDeadlines(context.Background(), "portfolio-001", 365)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return only non-completed events
	for _, d := range deadlines {
		if d.Status == EventStatusCompleted {
			t.Error("should not include completed events")
		}
	}
	_ = deadlines
}

func TestGetUpcomingDeadlines_EmptyPortfolio(t *testing.T) {
	svc := newTestCalendarService()
	_, err := svc.GetUpcomingDeadlines(context.Background(), "", 30)
	if err == nil {
		t.Fatal("expected validation error for empty portfolio")
	}
}

func TestGetUpcomingDeadlines_DefaultDays(t *testing.T) {
	svc := newTestCalendarService()
	// withinDays <= 0 should default to 30
	_, err := svc.GetUpcomingDeadlines(context.Background(), "portfolio-001", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: helper functions
// ---------------------------------------------------------------------------

func TestResolveEventStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		dueDate time.Time
		want    EventStatus
	}{
		{"future_60d", now.AddDate(0, 0, 60), EventStatusUpcoming},
		{"future_7d", now.AddDate(0, 0, 7), EventStatusDueSoon},
		{"past_5d", now.AddDate(0, 0, -5), EventStatusOverdue},
		{"past_60d", now.AddDate(0, 0, -60), EventStatusOverdue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEventStatus(tt.dueDate, now)
			if got != tt.want {
				t.Errorf("resolveEventStatus() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestClassifyDeadlinePriority(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		dueDate time.Time
		want    EventPriority
	}{
		{"overdue", now.AddDate(0, 0, -1), PriorityCritical},
		{"3_days", now.AddDate(0, 0, 3), PriorityCritical},
		{"20_days", now.AddDate(0, 0, 20), PriorityHigh},
		{"60_days", now.AddDate(0, 0, 60), PriorityMedium},
		{"180_days", now.AddDate(0, 0, 180), PriorityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyDeadlinePriority(tt.dueDate, now)
			if got != tt.want {
				t.Errorf("classifyDeadlinePriority() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestIsValidEventStatus(t *testing.T) {
	if !isValidEventStatus(EventStatusCompleted) {
		t.Error("completed should be valid")
	}
	if !isValidEventStatus(EventStatusCancelled) {
		t.Error("cancelled should be valid")
	}
	if isValidEventStatus(EventStatus("invalid")) {
		t.Error("invalid should not be valid")
	}
}

func TestDefaultReminders(t *testing.T) {
	reminders := defaultReminders()
	if len(reminders) != 5 {
		t.Errorf("expected 5 default reminders, got %d", len(reminders))
	}
	for _, r := range reminders {
		if !r.Enabled {
			t.Error("all default reminders should be enabled")
		}
		if r.DaysBefore <= 0 {
			t.Error("days_before should be positive")
		}
	}
}

func TestBuildICalData(t *testing.T) {
	events := []CalendarEvent{
		{
			ID:          "evt-1",
			Title:       "Test Event",
			Description: "Test Description",
			EventType:   EventTypeAnnuityDue,
			DueDate:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			Reminders: []ReminderConfig{
				{DaysBefore: 7, Channel: "email", Enabled: true},
			},
		},
	}
	data := buildICalData(events)
	content := string(data)

	if !strings.Contains(content, "BEGIN:VCALENDAR") {
		t.Error("missing VCALENDAR begin")
	}
	if !strings.Contains(content, "PRODID:-//KeyIP-Intelligence") {
		t.Error("missing PRODID")
	}
	if !strings.Contains(content, "Test Event") {
		t.Error("missing event summary")
	}
	if !strings.Contains(content, "BEGIN:VALARM") {
		t.Error("missing VALARM for reminder")
	}
	if !strings.Contains(content, "END:VCALENDAR") {
		t.Error("missing VCALENDAR end")
	}
}

func TestJurisdictionMaxLife(t *testing.T) {
	jurisdictions := []domainLifecycle.Jurisdiction{
		domainLifecycle.JurisdictionCN,
		domainLifecycle.JurisdictionUS,
		domainLifecycle.JurisdictionEP,
		domainLifecycle.JurisdictionJP,
		domainLifecycle.JurisdictionKR,
		domainLifecycle.Jurisdiction("XX"),
	}
	for _, j := range jurisdictions {
		life := jurisdictionMaxLife(j)
		if life != 20 {
			t.Errorf("jurisdictionMaxLife(%s) = %d, want 20", j, life)
		}
	}
}

//Personal.AI order the ending

