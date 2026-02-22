// Phase 10 - File 203 of 349
package infringement

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// --- Mock WatchlistRepository ---

type mockWatchlistRepository struct {
	mu         sync.Mutex
	watchlists map[string]*Watchlist
	saveErr    error
	updateErr  error
}

func newMockWatchlistRepository() *mockWatchlistRepository {
	return &mockWatchlistRepository{
		watchlists: make(map[string]*Watchlist),
	}
}

func (m *mockWatchlistRepository) Save(ctx context.Context, wl *Watchlist) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.watchlists[wl.ID] = wl
	return nil
}

func (m *mockWatchlistRepository) FindByID(ctx context.Context, id string) (*Watchlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	wl, ok := m.watchlists[id]
	if !ok {
		return nil, nil
	}
	return wl, nil
}

func (m *mockWatchlistRepository) Update(ctx context.Context, wl *Watchlist) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	m.watchlists[wl.ID] = wl
	return nil
}

func (m *mockWatchlistRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.watchlists, id)
	return nil
}

func (m *mockWatchlistRepository) List(ctx context.Context, opts WatchlistListOptions) ([]*Watchlist, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Watchlist
	for _, wl := range m.watchlists {
		if opts.OwnerID != "" && wl.OwnerID != opts.OwnerID {
			continue
		}
		if opts.Status != nil && wl.Status != *opts.Status {
			continue
		}
		result = append(result, wl)
	}
	return result, len(result), nil
}

func (m *mockWatchlistRepository) FindDueForScan(ctx context.Context, before time.Time) ([]*Watchlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*Watchlist
	for _, wl := range m.watchlists {
		if wl.Status == WatchlistStatusActive && wl.NextScanAt != nil && wl.NextScanAt.Before(before) {
			result = append(result, wl)
		}
	}
	return result, nil
}

// --- Mock ScanResultRepository ---

type mockScanResultRepository struct {
	mu      sync.Mutex
	results map[string][]*ScanResult
}

func newMockScanResultRepository() *mockScanResultRepository {
	return &mockScanResultRepository{
		results: make(map[string][]*ScanResult),
	}
}

func (m *mockScanResultRepository) Save(ctx context.Context, result *ScanResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[result.WatchlistID] = append(m.results[result.WatchlistID], result)
	return nil
}

func (m *mockScanResultRepository) FindByWatchlistID(ctx context.Context, watchlistID string, limit int) ([]*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	results := m.results[watchlistID]
	if len(results) > limit {
		results = results[len(results)-limit:]
	}
	return results, nil
}

// --- Mock AlertService for monitoring tests ---

type mockAlertServiceForMonitoring struct {
	mu      sync.Mutex
	alerts  []*Alert
	created int
}

func newMockAlertServiceForMonitoring() *mockAlertServiceForMonitoring {
	return &mockAlertServiceForMonitoring{}
}

func (m *mockAlertServiceForMonitoring) CreateAlert(ctx context.Context, req *CreateAlertRequest) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.created++
	alert := &Alert{
		ID:           fmt.Sprintf("ALT-mock-%d", m.created),
		PatentNumber: req.PatentNumber,
		MoleculeID:   req.MoleculeID,
		WatchlistID:  req.WatchlistID,
		Level:        req.Level,
		Status:       AlertStatusOpen,
		Title:        req.Title,
		CreatedAt:    time.Now().UTC(),
	}
	m.alerts = append(m.alerts, alert)
	return alert, nil
}

func (m *mockAlertServiceForMonitoring) GetAlert(ctx context.Context, alertID string) (*Alert, error) {
	return nil, nil
}
func (m *mockAlertServiceForMonitoring) AcknowledgeAlert(ctx context.Context, alertID string) error {
	return nil
}
func (m *mockAlertServiceForMonitoring) DismissAlert(ctx context.Context, req *DismissAlertRequest) error {
	return nil
}
func (m *mockAlertServiceForMonitoring) EscalateAlert(ctx context.Context, alertID string) error {
	return nil
}
func (m *mockAlertServiceForMonitoring) ResolveAlert(ctx context.Context, alertID, resolution string) error {
	return nil
}
func (m *mockAlertServiceForMonitoring) ListAlerts(ctx context.Context, opts AlertListOptions) ([]*Alert, *AlertPaginationResult, error) {
	return nil, &AlertPaginationResult{}, nil
}
func (m *mockAlertServiceForMonitoring) GetAlertStats(ctx context.Context, watchlistID string) (*AlertStats, error) {
	return &AlertStats{}, nil
}
func (m *mockAlertServiceForMonitoring) ProcessOverSLAAlerts(ctx context.Context) (int, error) {
	return 0, nil
}
func (m *mockAlertServiceForMonitoring) UpdateAlertConfig(ctx context.Context, req *AlertConfigRequest) error {
	return nil
}

// --- Helper ---

func newTestMonitoringService(
	wlRepo *mockWatchlistRepository,
	srRepo *mockScanResultRepository,
	alertSvc AlertService,
) MonitoringService {
	return NewMonitoringService(
		wlRepo,
		srRepo,
		alertSvc,
		newMockAlertProducer(),
		newMockAlertCache(),
		&mockAlertLogger{},
	)
}

// --- Tests ---

func TestCreateWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	srRepo := newMockScanResultRepository()
	alertSvc := newMockAlertServiceForMonitoring()
	svc := newTestMonitoringService(wlRepo, srRepo, alertSvc)

	req := &CreateWatchlistRequest{
		Name:                "Test Watchlist",
		OwnerID:             "user-001",
		ScanFrequency:       ScanFrequencyDaily,
		SimilarityThreshold: 0.85,
		PatentNumbers:       []string{"US-001", "US-002"},
		MoleculeIDs:         []string{"MOL-001"},
	}

	wl, err := svc.CreateWatchlist(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if wl == nil {
		t.Fatal("expected watchlist, got nil")
	}
	if wl.Name != "Test Watchlist" {
		t.Errorf("expected name 'Test Watchlist', got %s", wl.Name)
	}
	if wl.Status != WatchlistStatusActive {
		t.Errorf("expected ACTIVE, got %s", wl.Status.String())
	}
	if wl.SimilarityThreshold != 0.85 {
		t.Errorf("expected threshold 0.85, got %f", wl.SimilarityThreshold)
	}
	if wl.NextScanAt == nil {
		t.Error("expected NextScanAt to be set")
	}
}

func TestCreateWatchlist_NilRequest(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())
	_, err := svc.CreateWatchlist(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestCreateWatchlist_ValidationErrors(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	tests := []struct {
		name string
		req  CreateWatchlistRequest
	}{
		{"empty name", CreateWatchlistRequest{OwnerID: "u1", ScanFrequency: ScanFrequencyDaily, PatentNumbers: []string{"P1"}}},
		{"empty owner", CreateWatchlistRequest{Name: "WL", ScanFrequency: ScanFrequencyDaily, PatentNumbers: []string{"P1"}}},
		{"no items", CreateWatchlistRequest{Name: "WL", OwnerID: "u1", ScanFrequency: ScanFrequencyDaily}},
		{"bad threshold", CreateWatchlistRequest{Name: "WL", OwnerID: "u1", ScanFrequency: ScanFrequencyDaily, PatentNumbers: []string{"P1"}, SimilarityThreshold: 1.5}},
		{"bad frequency", CreateWatchlistRequest{Name: "WL", OwnerID: "u1", ScanFrequency: ScanFrequency(99), PatentNumbers: []string{"P1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateWatchlist(context.Background(), &tt.req)
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestCreateWatchlist_Defaults(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	req := &CreateWatchlistRequest{
		Name:          "Default WL",
		OwnerID:       "user-001",
		PatentNumbers: []string{"US-001"},
	}

	wl, err := svc.CreateWatchlist(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if wl.SimilarityThreshold != 0.8 {
		t.Errorf("expected default threshold 0.8, got %f", wl.SimilarityThreshold)
	}
	if wl.ScanFrequency != ScanFrequencyWeekly {
		t.Errorf("expected default frequency WEEKLY, got %s", wl.ScanFrequency.String())
	}
}

func TestUpdateWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "Original", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly, PatentNumbers: []string{"P1"},
	})

	newName := "Updated"
	newThreshold := 0.9
	updated, err := svc.UpdateWatchlist(context.Background(), &UpdateWatchlistRequest{
		WatchlistID:         created.ID,
		Name:                &newName,
		SimilarityThreshold: &newThreshold,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("expected Updated, got %s", updated.Name)
	}
	if updated.SimilarityThreshold != 0.9 {
		t.Errorf("expected 0.9, got %f", updated.SimilarityThreshold)
	}
}

func TestUpdateWatchlist_NotFound(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())
	name := "X"
	_, err := svc.UpdateWatchlist(context.Background(), &UpdateWatchlistRequest{
		WatchlistID: "nonexistent", Name: &name,
	})
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestDeleteWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "ToDelete", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly, PatentNumbers: []string{"P1"},
	})

	err := svc.DeleteWatchlist(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wl, _ := svc.GetWatchlist(context.Background(), created.ID)
	if wl.Status != WatchlistStatusArchived {
		t.Errorf("expected ARCHIVED, got %s", wl.Status.String())
	}
}

func TestDeleteWatchlist_NotFound(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())
	err := svc.DeleteWatchlist(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "GetMe", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly, PatentNumbers: []string{"P1"},
	})

	wl, err := svc.GetWatchlist(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if wl.Name != "GetMe" {
		t.Errorf("expected GetMe, got %s", wl.Name)
	}
}

func TestGetWatchlist_EmptyID(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())
	_, err := svc.GetWatchlist(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestListWatchlists_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	for i := 0; i < 3; i++ {
		_, _ = svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
			Name: fmt.Sprintf("WL-%d", i), OwnerID: "user-001",
			ScanFrequency: ScanFrequencyWeekly, PatentNumbers: []string{"P1"},
		})
	}

	wls, total, err := svc.ListWatchlists(context.Background(), WatchlistListOptions{
		OwnerID: "user-001", Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if total != 3 {
		t.Errorf("expected 3, got %d", total)
	}
	if len(wls) != 3 {
		t.Errorf("expected 3 watchlists, got %d", len(wls))
	}
}

func TestAddPatentsToWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "AddPatents", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly,
		PatentNumbers: []string{"P1"},
	})

	err := svc.AddPatentsToWatchlist(context.Background(), created.ID, []string{"P2", "P3"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wl, _ := svc.GetWatchlist(context.Background(), created.ID)
	if len(wl.PatentNumbers) != 3 {
		t.Errorf("expected 3 patents, got %d", len(wl.PatentNumbers))
	}
}

func TestAddPatentsToWatchlist_NoDuplicates(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "NoDup", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly,
		PatentNumbers: []string{"P1", "P2"},
	})

	_ = svc.AddPatentsToWatchlist(context.Background(), created.ID, []string{"P2", "P3"})

	wl, _ := svc.GetWatchlist(context.Background(), created.ID)
	if len(wl.PatentNumbers) != 3 {
		t.Errorf("expected 3 patents (no dup), got %d", len(wl.PatentNumbers))
	}
}

func TestAddMoleculesToWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "AddMols", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly,
		PatentNumbers: []string{"P1"}, MoleculeIDs: []string{"M1"},
	})

	err := svc.AddMoleculesToWatchlist(context.Background(), created.ID, []string{"M2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wl, _ := svc.GetWatchlist(context.Background(), created.ID)
	if len(wl.MoleculeIDs) != 2 {
		t.Errorf("expected 2 molecules, got %d", len(wl.MoleculeIDs))
	}
}

func TestRemovePatentsFromWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "RemoveP", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly,
		PatentNumbers: []string{"P1", "P2", "P3"},
	})

	err := svc.RemovePatentsFromWatchlist(context.Background(), created.ID, []string{"P2"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wl, _ := svc.GetWatchlist(context.Background(), created.ID)
	if len(wl.PatentNumbers) != 2 {
		t.Errorf("expected 2 patents, got %d", len(wl.PatentNumbers))
	}
}

func TestRemoveMoleculesFromWatchlist_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "RemoveM", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly,
		PatentNumbers: []string{"P1"}, MoleculeIDs: []string{"M1", "M2", "M3"},
	})

	err := svc.RemoveMoleculesFromWatchlist(context.Background(), created.ID, []string{"M1", "M3"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wl, _ := svc.GetWatchlist(context.Background(), created.ID)
	if len(wl.MoleculeIDs) != 1 {
		t.Errorf("expected 1 molecule, got %d", len(wl.MoleculeIDs))
	}
}

func TestRunScan_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	srRepo := newMockScanResultRepository()
	alertSvc := newMockAlertServiceForMonitoring()
	svc := newTestMonitoringService(wlRepo, srRepo, alertSvc)

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "ScanWL", OwnerID: "user-001", ScanFrequency: ScanFrequencyDaily,
		SimilarityThreshold: 0.8,
		PatentNumbers:       []string{"P1", "P2"},
		MoleculeIDs:         []string{"M1"},
	})

	result, err := svc.RunScan(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected scan result, got nil")
	}
	if result.PatentsScanned != 2 {
		t.Errorf("expected 2 patents scanned, got %d", result.PatentsScanned)
	}
	if result.MoleculesScanned != 1 {
		t.Errorf("expected 1 molecule scanned, got %d", result.MoleculesScanned)
	}

	// Verify watchlist was updated.
	wl, _ := svc.GetWatchlist(context.Background(), created.ID)
	if wl.TotalScans != 1 {
		t.Errorf("expected 1 total scan, got %d", wl.TotalScans)
	}
	if wl.LastScanAt == nil {
		t.Error("expected LastScanAt to be set")
	}
}

func TestRunScan_InactiveWatchlist(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "InactiveWL", OwnerID: "user-001", ScanFrequency: ScanFrequencyWeekly,
		PatentNumbers: []string{"P1"},
	})
	_ = svc.DeleteWatchlist(context.Background(), created.ID)

	_, err := svc.RunScan(context.Background(), created.ID)
	if err == nil {
		t.Fatal("expected error for inactive watchlist")
	}
}

func TestRunScan_NotFound(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())
	_, err := svc.RunScan(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestRunScheduledScans_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	svc := newTestMonitoringService(wlRepo, newMockScanResultRepository(), newMockAlertServiceForMonitoring())

	// Create watchlist with NextScanAt in the past.
	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "ScheduledWL", OwnerID: "user-001", ScanFrequency: ScanFrequencyDaily,
		PatentNumbers: []string{"P1"},
	})

	// Force NextScanAt to the past.
	wlRepo.mu.Lock()
	past := time.Now().UTC().Add(-1 * time.Hour)
	wlRepo.watchlists[created.ID].NextScanAt = &past
	wlRepo.mu.Unlock()

	count, err := svc.RunScheduledScans(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 scan run, got %d", count)
	}
}

func TestGetScanHistory_Success(t *testing.T) {
	wlRepo := newMockWatchlistRepository()
	srRepo := newMockScanResultRepository()
	svc := newTestMonitoringService(wlRepo, srRepo, newMockAlertServiceForMonitoring())

	created, _ := svc.CreateWatchlist(context.Background(), &CreateWatchlistRequest{
		Name: "HistoryWL", OwnerID: "user-001", ScanFrequency: ScanFrequencyDaily,
		PatentNumbers: []string{"P1"},
	})

	_, _ = svc.RunScan(context.Background(), created.ID)
	_, _ = svc.RunScan(context.Background(), created.ID)

	history, err := svc.GetScanHistory(context.Background(), created.ID, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(history) != 2 {
		t.Errorf("expected 2 scan results, got %d", len(history))
	}
}

func TestGetScanHistory_EmptyID(t *testing.T) {
	svc := newTestMonitoringService(newMockWatchlistRepository(), newMockScanResultRepository(), newMockAlertServiceForMonitoring())
	_, err := svc.GetScanHistory(context.Background(), "", 10)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWatchlistStatus_String(t *testing.T) {
	tests := []struct {
		s    WatchlistStatus
		want string
	}{
		{WatchlistStatusActive, "ACTIVE"},
		{WatchlistStatusPaused, "PAUSED"},
		{WatchlistStatusArchived, "ARCHIVED"},
		{WatchlistStatus(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("WatchlistStatus(%d).String() = %s, want %s", tt.s, got, tt.want)
		}
	}
}

func TestScanFrequency_String(t *testing.T) {
	tests := []struct {
		f    ScanFrequency
		want string
	}{
		{ScanFrequencyDaily, "DAILY"},
		{ScanFrequencyWeekly, "WEEKLY"},
		{ScanFrequencyBiWeekly, "BI_WEEKLY"},
		{ScanFrequencyMonthly, "MONTHLY"},
		{ScanFrequency(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.f.String(); got != tt.want {
			t.Errorf("ScanFrequency(%d).String() = %s, want %s", tt.f, got, tt.want)
		}
	}
}

func TestScanFrequency_Duration(t *testing.T) {
	if ScanFrequencyDaily.Duration() != 24*time.Hour {
		t.Errorf("daily expected 24h, got %v", ScanFrequencyDaily.Duration())
	}
	if ScanFrequencyWeekly.Duration() != 7*24*time.Hour {
		t.Errorf("weekly expected 168h, got %v", ScanFrequencyWeekly.Duration())
	}
}

func TestScoreToAlertLevel(t *testing.T) {
	svc := &monitoringServiceImpl{}
	tests := []struct {
		score float64
		want  AlertLevel
	}{
		{0.96, AlertLevelCritical},
		{0.95, AlertLevelCritical},
		{0.90, AlertLevelHigh},
		{0.85, AlertLevelHigh},
		{0.75, AlertLevelMedium},
		{0.70, AlertLevelMedium},
		{0.50, AlertLevelLow},
	}
	for _, tt := range tests {
		got := svc.scoreToAlertLevel(tt.score)
		if got != tt.want {
			t.Errorf("scoreToAlertLevel(%f) = %s, want %s", tt.score, got.String(), tt.want.String())
		}
	}
}

func TestCreateWatchlistRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateWatchlistRequest
		wantErr bool
	}{
		{"valid", CreateWatchlistRequest{Name: "WL", OwnerID: "u1", ScanFrequency: ScanFrequencyDaily, PatentNumbers: []string{"P1"}}, false},
		{"empty name", CreateWatchlistRequest{OwnerID: "u1", ScanFrequency: ScanFrequencyDaily, PatentNumbers: []string{"P1"}}, true},
		{"empty owner", CreateWatchlistRequest{Name: "WL", ScanFrequency: ScanFrequencyDaily, PatentNumbers: []string{"P1"}}, true},
		{"no items", CreateWatchlistRequest{Name: "WL", OwnerID: "u1", ScanFrequency: ScanFrequencyDaily}, true},
		{"bad threshold", CreateWatchlistRequest{Name: "WL", OwnerID: "u1", ScanFrequency: ScanFrequencyDaily, PatentNumbers: []string{"P1"}, SimilarityThreshold: -0.1}, true},
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

func TestUpdateWatchlistRequest_Validate(t *testing.T) {
	badThreshold := 2.0
	tests := []struct {
		name    string
		req     UpdateWatchlistRequest
		wantErr bool
	}{
		{"valid", UpdateWatchlistRequest{WatchlistID: "WL-001"}, false},
		{"empty id", UpdateWatchlistRequest{}, true},
		{"bad threshold", UpdateWatchlistRequest{WatchlistID: "WL-001", SimilarityThreshold: &badThreshold}, true},
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

func TestGenerateWatchlistID_Deterministic(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	id1 := generateWatchlistID("WL", "user-001", ts)
	id2 := generateWatchlistID("WL", "user-001", ts)
	if id1 != id2 {
		t.Errorf("expected deterministic, got %s vs %s", id1, id2)
	}
}

func TestGenerateScanID_Deterministic(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	id1 := generateScanID("WL-001", ts)
	id2 := generateScanID("WL-001", ts)
	if id1 != id2 {
		t.Errorf("expected deterministic, got %s vs %s", id1, id2)
	}
}

//Personal.AI order the ending

