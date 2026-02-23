// internal/application/lifecycle/deadline_test.go
//
// Phase 10 - File #211
// Unit tests for DeadlineService application service.

package lifecycle

import (
	"context"
	"testing"
	"time"

	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
)

// ---------------------------------------------------------------------------
// Mock implementations for deadline tests
// ---------------------------------------------------------------------------

// Mock implementations removed. Use shared mocks from common_test.go.

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestDeadlineService(opts ...func(*testDeadlineOpts)) DeadlineService {
	o := &testDeadlineOpts{
		lifecycleSvc:  &mockLifecycleService{},
		lifecycleRepo: &mockLifecycleRepo{},
		patentRepo:    newMockPatentRepo(),
		cache:         newMockCache(),
		logger:        &mockLogger{},
	}

	// Default patent for tests
	fd := time.Now().AddDate(-3, 0, 0)
	o.patentRepo.patents["00000000-0000-0000-0000-000000000001"] = &mockPatentInfo{
		ID: "00000000-0000-0000-0000-000000000001", PatentNumber: "CN202310001234.5",
		Title: "Test Patent", Jurisdiction: "CN",
		FilingDate: fd,
	}

	for _, fn := range opts {
		fn(o)
	}
	return NewDeadlineService(
		o.lifecycleSvc, o.lifecycleRepo, o.patentRepo,
		o.cache, o.logger,
	)
}

type testDeadlineOpts struct {
	lifecycleSvc  domainLifecycle.Service
	lifecycleRepo domainLifecycle.LifecycleRepository
	patentRepo    *mockPatentRepo
	cache         CachePort
	logger        Logger
}

// ---------------------------------------------------------------------------
// Tests: ListDeadlines
// ---------------------------------------------------------------------------

func TestListDeadlines_ByPatentID(t *testing.T) {
	svc := newTestDeadlineService()
	ctx := context.Background()

	resp, err := svc.ListDeadlines(ctx, &DeadlineQuery{
		PatentID: "00000000-0000-0000-0000-000000000001",
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total == 0 {
		t.Error("expected at least one deadline")
	}
	for _, dl := range resp.Deadlines {
		if dl.PatentID != "00000000-0000-0000-0000-000000000001" {
			t.Errorf("expected patent_id 00000000-0000-0000-0000-000000000001, got %s", dl.PatentID)
		}
	}
}

func TestListDeadlines_ByPortfolio(t *testing.T) {
	svc := newTestDeadlineService()

	resp, err := svc.ListDeadlines(context.Background(), &DeadlineQuery{
		PortfolioID: "portfolio-001",
		PageSize:    100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total == 0 {
		t.Error("expected deadlines from portfolio")
	}
}

func TestListDeadlines_NilQuery(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.ListDeadlines(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil query")
	}
}

func TestListDeadlines_FilterByType(t *testing.T) {
	svc := newTestDeadlineService()

	resp, err := svc.ListDeadlines(context.Background(), &DeadlineQuery{
		PatentID: "00000000-0000-0000-0000-000000000001",
		Types:    []DeadlineType{DeadlineTypeExamRequest},
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, dl := range resp.Deadlines {
		if dl.DeadlineType != DeadlineTypeExamRequest {
			t.Errorf("expected exam_request type, got %s", dl.DeadlineType)
		}
	}
}

func TestListDeadlines_FilterByUrgency(t *testing.T) {
	svc := newTestDeadlineService()

	resp, err := svc.ListDeadlines(context.Background(), &DeadlineQuery{
		PatentID:  "00000000-0000-0000-0000-000000000001",
		Urgencies: []DeadlineUrgency{UrgencyFuture},
		PageSize:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, dl := range resp.Deadlines {
		if dl.Urgency != UrgencyFuture {
			t.Errorf("expected future urgency, got %s", dl.Urgency)
		}
	}
}

func TestListDeadlines_Pagination(t *testing.T) {
	svc := newTestDeadlineService()

	resp, err := svc.ListDeadlines(context.Background(), &DeadlineQuery{
		PatentID: "00000000-0000-0000-0000-000000000001",
		Page:     1,
		PageSize: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Deadlines) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(resp.Deadlines))
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
}

func TestListDeadlines_SortByUrgency(t *testing.T) {
	svc := newTestDeadlineService()

	resp, err := svc.ListDeadlines(context.Background(), &DeadlineQuery{
		PatentID: "00000000-0000-0000-0000-000000000001",
		SortBy:   "urgency",
		PageSize: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 1; i < len(resp.Deadlines); i++ {
		prev := urgencyOrder(resp.Deadlines[i-1].Urgency)
		curr := urgencyOrder(resp.Deadlines[i].Urgency)
		if prev > curr {
			t.Errorf("deadlines not sorted by urgency at index %d", i)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: CreateDeadline
// ---------------------------------------------------------------------------

func TestCreateDeadline_Success(t *testing.T) {
	svc := newTestDeadlineService()

	dl, err := svc.CreateDeadline(context.Background(), &CreateDeadlineRequest{
		PatentID:     "00000000-0000-0000-0000-000000000001",
		Title:        "Custom Deadline",
		DeadlineType: DeadlineTypeCustom,
		DueDate:      time.Now().AddDate(0, 6, 0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dl.PatentID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("expected 00000000-0000-0000-0000-000000000001, got %s", dl.PatentID)
	}
	if dl.DeadlineType != DeadlineTypeCustom {
		t.Errorf("expected custom, got %s", dl.DeadlineType)
	}
	if len(dl.Alerts) == 0 {
		t.Error("expected default alerts")
	}
}

func TestCreateDeadline_NilRequest(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.CreateDeadline(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateDeadline_MissingFields(t *testing.T) {
	svc := newTestDeadlineService()

	_, err := svc.CreateDeadline(context.Background(), &CreateDeadlineRequest{
		Title:        "Test",
		DeadlineType: DeadlineTypeCustom,
		DueDate:      time.Now().AddDate(0, 1, 0),
	})
	if err == nil {
		t.Fatal("expected error for missing patent_id")
	}

	_, err = svc.CreateDeadline(context.Background(), &CreateDeadlineRequest{
		PatentID:     "00000000-0000-0000-0000-000000000001",
		DeadlineType: DeadlineTypeCustom,
		DueDate:      time.Now().AddDate(0, 1, 0),
	})
	if err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestCreateDeadline_PatentNotFound(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.CreateDeadline(context.Background(), &CreateDeadlineRequest{
		PatentID:     "00000000-0000-0000-0000-000000000000",
		Title:        "Test",
		DeadlineType: DeadlineTypeCustom,
		DueDate:      time.Now().AddDate(0, 1, 0),
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

// ---------------------------------------------------------------------------
// Tests: CompleteDeadline
// ---------------------------------------------------------------------------

func TestCompleteDeadline_Success(t *testing.T) {
	svc := newTestDeadlineService()
	err := svc.CompleteDeadline(context.Background(), "dl-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCompleteDeadline_EmptyID(t *testing.T) {
	svc := newTestDeadlineService()
	err := svc.CompleteDeadline(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---------------------------------------------------------------------------
// Tests: ExtendDeadline
// ---------------------------------------------------------------------------

func TestExtendDeadline_Success(t *testing.T) {
	svc := newTestDeadlineService()
	newDate := time.Now().AddDate(0, 6, 0)

	dl, err := svc.ExtendDeadline(context.Background(), &ExtendDeadlineRequest{
		DeadlineID: "dl-123",
		NewDueDate: newDate,
		Reason:     "Extension granted",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dl.ExtendedDate == nil {
		t.Fatal("expected extended_date to be set")
	}
}

func TestExtendDeadline_PastDate(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.ExtendDeadline(context.Background(), &ExtendDeadlineRequest{
		DeadlineID: "dl-123",
		NewDueDate: time.Now().AddDate(0, 0, -1),
	})
	if err == nil {
		t.Fatal("expected error for past date")
	}
}

func TestExtendDeadline_NilRequest(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.ExtendDeadline(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Tests: DeleteDeadline
// ---------------------------------------------------------------------------

func TestDeleteDeadline_Success(t *testing.T) {
	svc := newTestDeadlineService()
	err := svc.DeleteDeadline(context.Background(), "dl-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteDeadline_EmptyID(t *testing.T) {
	svc := newTestDeadlineService()
	err := svc.DeleteDeadline(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetComplianceDashboard
// ---------------------------------------------------------------------------

func TestGetComplianceDashboard_Success(t *testing.T) {
	svc := newTestDeadlineService()

	dashboard, err := svc.GetComplianceDashboard(context.Background(), "portfolio-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dashboard == nil {
		t.Fatal("expected non-nil dashboard")
	}
	if dashboard.TotalDeadlines == 0 {
		t.Error("expected at least one deadline")
	}
	if len(dashboard.ByUrgency) == 0 {
		t.Error("expected ByUrgency to be populated")
	}
	if len(dashboard.ByType) == 0 {
		t.Error("expected ByType to be populated")
	}
}

func TestGetComplianceDashboard_EmptyPortfolio(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.GetComplianceDashboard(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetOverdueDeadlines
// ---------------------------------------------------------------------------

func TestGetOverdueDeadlines_Success(t *testing.T) {
	svc := newTestDeadlineService()

	deadlines, err := svc.GetOverdueDeadlines(context.Background(), "portfolio-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, dl := range deadlines {
		if dl.Urgency != UrgencyExpired {
			t.Errorf("expected expired urgency, got %s", dl.Urgency)
		}
	}
}

func TestGetOverdueDeadlines_EmptyPortfolio(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.GetOverdueDeadlines(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---------------------------------------------------------------------------
// Tests: SyncStatutoryDeadlines
// ---------------------------------------------------------------------------

func TestSyncStatutoryDeadlines_Success(t *testing.T) {
	svc := newTestDeadlineService()

	count, err := svc.SyncStatutoryDeadlines(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count == 0 {
		t.Error("expected at least one statutory deadline generated")
	}
}

func TestSyncStatutoryDeadlines_EmptyID(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.SyncStatutoryDeadlines(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSyncStatutoryDeadlines_NotFound(t *testing.T) {
	svc := newTestDeadlineService()
	_, err := svc.SyncStatutoryDeadlines(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

// ---------------------------------------------------------------------------
// Tests: helper functions
// ---------------------------------------------------------------------------

func TestDaysUntil(t *testing.T) {
	now := time.Now()
	future := now.AddDate(0, 0, 10)
	d := daysUntil(future, now)
	if d < 9 || d > 11 {
		t.Errorf("expected ~10, got %d", d)
	}

	past := now.AddDate(0, 0, -5)
	d = daysUntil(past, now)
	if d > -4 || d < -6 {
		t.Errorf("expected ~-5, got %d", d)
	}
}

func TestClassifyUrgency(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		due  time.Time
		want DeadlineUrgency
	}{
		{"expired", now.AddDate(0, 0, -1), UrgencyExpired},
		{"critical", now.AddDate(0, 0, 3), UrgencyCritical},
		{"urgent", now.AddDate(0, 0, 15), UrgencyUrgent},
		{"normal", now.AddDate(0, 0, 60), UrgencyNormal},
		{"future", now.AddDate(0, 0, 120), UrgencyFuture},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyUrgency(tt.due, now)
			if got != tt.want {
				t.Errorf("classifyUrgency() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestUrgencyOrder(t *testing.T) {
	tests := []struct {
		u    DeadlineUrgency
		want int
	}{
		{UrgencyExpired, 0},
		{UrgencyCritical, 1},
		{UrgencyUrgent, 2},
		{UrgencyNormal, 3},
		{UrgencyFuture, 4},
		{DeadlineUrgency("unknown"), 5},
	}
	for _, tt := range tests {
		got := urgencyOrder(tt.u)
		if got != tt.want {
			t.Errorf("urgencyOrder(%s) = %d, want %d", tt.u, got, tt.want)
		}
	}
}

func TestDefaultAlerts(t *testing.T) {
	alerts := defaultAlerts()
	if len(alerts) != 4 {
		t.Errorf("expected 4 default alerts, got %d", len(alerts))
	}
	for _, a := range alerts {
		if !a.Enabled {
			t.Error("all default alerts should be enabled")
		}
		if len(a.Channels) == 0 {
			t.Error("each alert should have at least one channel")
		}
		if a.DaysBefore <= 0 {
			t.Error("days_before should be positive")
		}
	}
}

func TestMatchJurisdictions(t *testing.T) {
	if !matchJurisdictions("CN", nil) {
		t.Error("nil filter should match all")
	}
	if !matchJurisdictions("CN", []domainLifecycle.Jurisdiction{"CN", "US"}) {
		t.Error("CN should match [CN, US]")
	}
	if matchJurisdictions("JP", []domainLifecycle.Jurisdiction{"CN", "US"}) {
		t.Error("JP should not match [CN, US]")
	}
}

func TestMatchDeadlineTypes(t *testing.T) {
	if !matchDeadlineTypes(DeadlineTypeCustom, nil) {
		t.Error("nil filter should match all")
	}
	if !matchDeadlineTypes(DeadlineTypeCustom, []DeadlineType{DeadlineTypeCustom}) {
		t.Error("custom should match [custom]")
	}
	if matchDeadlineTypes(DeadlineTypeAppeal, []DeadlineType{DeadlineTypeCustom}) {
		t.Error("appeal should not match [custom]")
	}
}

func TestMatchUrgencies(t *testing.T) {
	if !matchUrgencies(UrgencyCritical, nil) {
		t.Error("nil filter should match all")
	}
	if !matchUrgencies(UrgencyCritical, []DeadlineUrgency{UrgencyCritical, UrgencyExpired}) {
		t.Error("critical should match [critical, expired]")
	}
	if matchUrgencies(UrgencyFuture, []DeadlineUrgency{UrgencyCritical}) {
		t.Error("future should not match [critical]")
	}
}

func TestSortDeadlines(t *testing.T) {
	now := time.Now()
	deadlines := []Deadline{
		{DueDate: now.AddDate(0, 0, 30), Urgency: UrgencyNormal},
		{DueDate: now.AddDate(0, 0, 3), Urgency: UrgencyCritical},
		{DueDate: now.AddDate(0, 0, 90), Urgency: UrgencyFuture},
	}

	// Sort by due_date asc (default)
	sortDeadlines(deadlines, "", "asc")
	if !deadlines[0].DueDate.Before(deadlines[1].DueDate) {
		t.Error("expected ascending due_date sort")
	}

	// Sort by due_date desc
	sortDeadlines(deadlines, "due_date", "desc")
	if !deadlines[0].DueDate.After(deadlines[1].DueDate) {
		t.Error("expected descending due_date sort")
	}

	// Sort by urgency asc
	sortDeadlines(deadlines, "urgency", "asc")
	if urgencyOrder(deadlines[0].Urgency) > urgencyOrder(deadlines[1].Urgency) {
		t.Error("expected ascending urgency sort")
	}
}

//Personal.AI order the ending

