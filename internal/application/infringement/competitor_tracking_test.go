// Phase 10 - File 201 of 349
package infringement

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- Mock CompetitorRepository ---

type mockCompetitorRepository struct {
	mu          sync.Mutex
	competitors map[string]*TrackedCompetitor
	saveErr     error
	updateErr   error
	listErr     error
}

func newMockCompetitorRepository() *mockCompetitorRepository {
	return &mockCompetitorRepository{
		competitors: make(map[string]*TrackedCompetitor),
	}
}

func (m *mockCompetitorRepository) Save(ctx context.Context, c *TrackedCompetitor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.competitors[c.ID] = c
	return nil
}

func (m *mockCompetitorRepository) FindByID(ctx context.Context, id string) (*TrackedCompetitor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.competitors[id]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (m *mockCompetitorRepository) Update(ctx context.Context, c *TrackedCompetitor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	m.competitors[c.ID] = c
	return nil
}

func (m *mockCompetitorRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.competitors, id)
	return nil
}

func (m *mockCompetitorRepository) List(ctx context.Context, opts CompetitorListOptions) ([]*TrackedCompetitor, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var result []*TrackedCompetitor
	for _, c := range m.competitors {
		if opts.WatchlistID != "" && c.WatchlistID != opts.WatchlistID {
			continue
		}
		if opts.Status != nil && c.Status != *opts.Status {
			continue
		}
		if opts.TechnologyArea != "" {
			found := false
			for _, area := range c.TechnologyAreas {
				if area == opts.TechnologyArea {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, c)
	}
	return result, len(result), nil
}

func (m *mockCompetitorRepository) FindByName(ctx context.Context, name string, watchlistID string) (*TrackedCompetitor, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.competitors {
		if c.Name == name && c.WatchlistID == watchlistID {
			return c, nil
		}
	}
	return nil, nil
}

// --- Helper ---

func newTestCompetitorTrackingService(repo *mockCompetitorRepository) CompetitorTrackingService {
	return NewCompetitorTrackingService(
		repo,
		newMockAlertProducer(),
		newMockAlertCache(),
		&mockAlertLogger{},
	)
}

// --- Tests ---

func TestTrackCompetitor_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	req := &TrackCompetitorRequest{
		Name:            "PharmaCorp",
		Aliases:         []string{"Pharma Corp Inc", "PC"},
		WatchlistID:     "WL-001",
		TechnologyAreas: []string{"oncology", "immunology"},
	}

	competitor, err := svc.TrackCompetitor(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if competitor == nil {
		t.Fatal("expected competitor, got nil")
	}
	if competitor.Name != "PharmaCorp" {
		t.Errorf("expected name PharmaCorp, got %s", competitor.Name)
	}
	if competitor.Status != CompetitorStatusActive {
		t.Errorf("expected ACTIVE status, got %s", competitor.Status.String())
	}
}

func TestTrackCompetitor_NilRequest(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.TrackCompetitor(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestTrackCompetitor_ValidationFailure(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "", WatchlistID: "WL-001",
	})
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}

	_, err = svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "Test", WatchlistID: "",
	})
	if err == nil {
		t.Fatal("expected validation error for empty watchlist_id")
	}
}

func TestTrackCompetitor_Duplicate(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	req := &TrackCompetitorRequest{
		Name: "DupCorp", WatchlistID: "WL-001",
	}

	first, _ := svc.TrackCompetitor(context.Background(), req)
	second, err := svc.TrackCompetitor(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error on duplicate, got %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("expected same ID for duplicate, got %s vs %s", first.ID, second.ID)
	}
}

func TestTrackCompetitor_ReactivateArchived(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	req := &TrackCompetitorRequest{
		Name: "ArchivedCorp", WatchlistID: "WL-001",
	}

	competitor, _ := svc.TrackCompetitor(context.Background(), req)
	_ = svc.RemoveCompetitor(context.Background(), competitor.ID)

	// Verify archived.
	profile, _ := svc.GetCompetitorProfile(context.Background(), competitor.ID)
	if profile.Status != CompetitorStatusArchived {
		t.Fatalf("expected ARCHIVED, got %s", profile.Status.String())
	}

	// Re-track should reactivate.
	reactivated, err := svc.TrackCompetitor(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reactivated.Status != CompetitorStatusActive {
		t.Errorf("expected ACTIVE after reactivation, got %s", reactivated.Status.String())
	}
}

func TestRemoveCompetitor_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	competitor, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "RemoveCorp", WatchlistID: "WL-001",
	})

	err := svc.RemoveCompetitor(context.Background(), competitor.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	profile, _ := svc.GetCompetitorProfile(context.Background(), competitor.ID)
	if profile.Status != CompetitorStatusArchived {
		t.Errorf("expected ARCHIVED, got %s", profile.Status.String())
	}
}

func TestRemoveCompetitor_NotFound(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	err := svc.RemoveCompetitor(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestRemoveCompetitor_EmptyID(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	err := svc.RemoveCompetitor(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestListTrackedCompetitors_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	for i := 0; i < 5; i++ {
		_, _ = svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
			Name:        fmt.Sprintf("Corp-%d", i),
			WatchlistID: "WL-LIST",
		})
	}

	competitors, total, err := svc.ListTrackedCompetitors(context.Background(), CompetitorListOptions{
		WatchlistID: "WL-LIST",
		Page:        1,
		PageSize:    10,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(competitors) != 5 {
		t.Errorf("expected 5 competitors, got %d", len(competitors))
	}
}

func TestListTrackedCompetitors_DefaultPagination(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, _, err := svc.ListTrackedCompetitors(context.Background(), CompetitorListOptions{
		Page: 0, PageSize: 0,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestGetCompetitorProfile_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	created, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "ProfileCorp", WatchlistID: "WL-001",
		TechnologyAreas: []string{"biotech"},
	})

	profile, err := svc.GetCompetitorProfile(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if profile.Name != "ProfileCorp" {
		t.Errorf("expected ProfileCorp, got %s", profile.Name)
	}
}

func TestGetCompetitorProfile_NotFound(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.GetCompetitorProfile(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetCompetitorProfile_EmptyID(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.GetCompetitorProfile(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAnalyzeCompetitorPortfolio_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	created, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "AnalyzeCorp", WatchlistID: "WL-001",
		TechnologyAreas: []string{"oncology", "cardiology"},
	})

	// Set patent count manually.
	repo.mu.Lock()
	repo.competitors[created.ID].PatentCount = 120
	repo.mu.Unlock()

	analysis, err := svc.AnalyzeCompetitorPortfolio(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if analysis.TotalPatents != 120 {
		t.Errorf("expected 120 patents, got %d", analysis.TotalPatents)
	}
	if analysis.CompetitorName != "AnalyzeCorp" {
		t.Errorf("expected AnalyzeCorp, got %s", analysis.CompetitorName)
	}
	if len(analysis.TechnologyBreakdown) != 2 {
		t.Errorf("expected 2 technology areas, got %d", len(analysis.TechnologyBreakdown))
	}
}

func TestAnalyzeCompetitorPortfolio_NotFound(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.AnalyzeCompetitorPortfolio(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestDetectNewFilings_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	created, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "FilingCorp", WatchlistID: "WL-001",
	})

	detections, err := svc.DetectNewFilings(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if detections == nil {
		t.Fatal("expected non-nil detections slice")
	}

	// Verify last scan was updated.
	repo.mu.Lock()
	updated := repo.competitors[created.ID]
	repo.mu.Unlock()
	if updated.LastScanAt == nil {
		t.Error("expected LastScanAt to be set after detection")
	}
}

func TestDetectNewFilings_InactiveCompetitor(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	created, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "InactiveCorp", WatchlistID: "WL-001",
	})
	_ = svc.RemoveCompetitor(context.Background(), created.ID)

	_, err := svc.DetectNewFilings(context.Background(), created.ID)
	if err == nil {
		t.Fatal("expected error for inactive competitor")
	}
}

func TestGetCompetitiveLandscape_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	for i := 0; i < 3; i++ {
		c, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
			Name:            fmt.Sprintf("LandscapeCorp-%d", i),
			WatchlistID:     "WL-001",
			TechnologyAreas: []string{"oncology"},
		})
		repo.mu.Lock()
		repo.competitors[c.ID].PatentCount = (i + 1) * 10
		repo.competitors[c.ID].RecentFilings = i + 1
		repo.mu.Unlock()
	}

	landscape, err := svc.GetCompetitiveLandscape(context.Background(), "oncology")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if landscape.TotalCompetitors != 3 {
		t.Errorf("expected 3 competitors, got %d", landscape.TotalCompetitors)
	}
	if landscape.TotalPatents != 60 {
		t.Errorf("expected 60 total patents, got %d", landscape.TotalPatents)
	}
	if len(landscape.TopFilers) != 3 {
		t.Errorf("expected 3 top filers, got %d", len(landscape.TopFilers))
	}
	// First filer should have highest patent count.
	if landscape.TopFilers[0].PatentCount != 30 {
		t.Errorf("expected top filer with 30 patents, got %d", landscape.TopFilers[0].PatentCount)
	}
}

func TestGetCompetitiveLandscape_EmptyArea(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.GetCompetitiveLandscape(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestComparePortfolios_Success(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	compA, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "CompA", WatchlistID: "WL-001",
		TechnologyAreas: []string{"oncology", "immunology", "neurology"},
	})
	compB, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "CompB", WatchlistID: "WL-001",
		TechnologyAreas: []string{"oncology", "cardiology", "neurology"},
	})

	repo.mu.Lock()
	repo.competitors[compA.ID].PatentCount = 50
	repo.competitors[compB.ID].PatentCount = 80
	repo.mu.Unlock()

	comparison, err := svc.ComparePortfolios(context.Background(), compA.ID, compB.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(comparison.OverlappingAreas) != 2 {
		t.Errorf("expected 2 overlapping areas, got %d", len(comparison.OverlappingAreas))
	}
	if len(comparison.UniqueToA) != 1 {
		t.Errorf("expected 1 unique to A, got %d", len(comparison.UniqueToA))
	}
	if len(comparison.UniqueToB) != 1 {
		t.Errorf("expected 1 unique to B, got %d", len(comparison.UniqueToB))
	}
	if comparison.PatentCountA != 50 {
		t.Errorf("expected patent count A = 50, got %d", comparison.PatentCountA)
	}
	if comparison.PatentCountB != 80 {
		t.Errorf("expected patent count B = 80, got %d", comparison.PatentCountB)
	}
}

func TestComparePortfolios_SameCompetitor(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.ComparePortfolios(context.Background(), "same-id", "same-id")
	if err == nil {
		t.Fatal("expected error when comparing same competitor")
	}
}

func TestComparePortfolios_EmptyIDs(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	_, err := svc.ComparePortfolios(context.Background(), "", "id-b")
	if err == nil {
		t.Fatal("expected validation error for empty ID")
	}
}

func TestComparePortfolios_NotFound(t *testing.T) {
	repo := newMockCompetitorRepository()
	svc := newTestCompetitorTrackingService(repo)

	compA, _ := svc.TrackCompetitor(context.Background(), &TrackCompetitorRequest{
		Name: "ExistsCorp", WatchlistID: "WL-001",
	})

	_, err := svc.ComparePortfolios(context.Background(), compA.ID, "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestCompetitorStatus_String(t *testing.T) {
	tests := []struct {
		status   CompetitorStatus
		expected string
	}{
		{CompetitorStatusActive, "ACTIVE"},
		{CompetitorStatusPaused, "PAUSED"},
		{CompetitorStatusArchived, "ARCHIVED"},
		{CompetitorStatus(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("CompetitorStatus(%d).String() = %s, want %s", tt.status, got, tt.expected)
		}
	}
}

func TestGenerateCompetitorID_Deterministic(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	id1 := generateCompetitorID("Corp", "WL-001", ts)
	id2 := generateCompetitorID("Corp", "WL-001", ts)
	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %s vs %s", id1, id2)
	}

	id3 := generateCompetitorID("OtherCorp", "WL-001", ts)
	if id1 == id3 {
		t.Error("expected different IDs for different names")
	}
}

func TestTrackCompetitorRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     TrackCompetitorRequest
		wantErr bool
	}{
		{"valid", TrackCompetitorRequest{Name: "Corp", WatchlistID: "WL-001"}, false},
		{"empty name", TrackCompetitorRequest{Name: "", WatchlistID: "WL-001"}, true},
		{"empty watchlist", TrackCompetitorRequest{Name: "Corp", WatchlistID: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

//Personal.AI order the ending
