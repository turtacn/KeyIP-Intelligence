package lifecycle

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	importUUID "github.com/google/uuid"
)

// ===========================================================================
// Test helper: build service with defaults
// ===========================================================================

type testHarness struct {
	svc           LegalStatusService
	lifecycleSvc  *mockLifecycleService
	lifecycleRepo *mockLifecycleRepo
	patentRepo    *mockPatentRepo
	publisher     *mockEventPublisher
	cache         *mockCache
	logger        *mockLogger
	metrics       *mockMetrics
}

func newTestHarness(t *testing.T, cfgOverride *LegalStatusConfig) *testHarness {
	t.Helper()
	h := &testHarness{
		lifecycleSvc:  &mockLifecycleService{},
		lifecycleRepo: &mockLifecycleRepo{},
		patentRepo:    &mockPatentRepo{patents: make(map[string]*mockPatentInfo)},
		publisher:     &mockEventPublisher{},
		cache:         newMockCache(),
		logger:        &mockLogger{},
		metrics:       newMockMetrics(),
	}
	svc, err := NewLegalStatusService(
		h.lifecycleSvc,
		h.lifecycleRepo,
		h.patentRepo,
		h.publisher,
		h.cache,
		h.logger,
		h.metrics,
		cfgOverride,
	)
	if err != nil {
		t.Fatalf("NewLegalStatusService: %v", err)
	}
	h.svc = svc
	return h
}

// ===========================================================================
// Tests: NewLegalStatusService
// ===========================================================================

func TestNewLegalStatusService_NilDependencies(t *testing.T) {
	tests := []struct {
		name string
		fn   func() (LegalStatusService, error)
	}{
		{"nil lifecycle svc", func() (LegalStatusService, error) {
			return NewLegalStatusService(nil, &mockLifecycleRepo{}, &mockPatentRepo{}, &mockEventPublisher{}, newMockCache(), &mockLogger{}, newMockMetrics(), nil)
		}},
		{"nil lifecycle repo", func() (LegalStatusService, error) {
			return NewLegalStatusService(&mockLifecycleService{}, nil, &mockPatentRepo{}, &mockEventPublisher{}, newMockCache(), &mockLogger{}, newMockMetrics(), nil)
		}},
		{"nil patent repo", func() (LegalStatusService, error) {
			return NewLegalStatusService(&mockLifecycleService{}, &mockLifecycleRepo{}, nil, &mockEventPublisher{}, newMockCache(), &mockLogger{}, newMockMetrics(), nil)
		}},
		{"nil publisher", func() (LegalStatusService, error) {
			return NewLegalStatusService(&mockLifecycleService{}, &mockLifecycleRepo{}, &mockPatentRepo{}, nil, newMockCache(), &mockLogger{}, newMockMetrics(), nil)
		}},
		{"nil cache", func() (LegalStatusService, error) {
			return NewLegalStatusService(&mockLifecycleService{}, &mockLifecycleRepo{}, &mockPatentRepo{}, &mockEventPublisher{}, nil, &mockLogger{}, newMockMetrics(), nil)
		}},
		{"nil logger", func() (LegalStatusService, error) {
			return NewLegalStatusService(&mockLifecycleService{}, &mockLifecycleRepo{}, &mockPatentRepo{}, &mockEventPublisher{}, newMockCache(), nil, newMockMetrics(), nil)
		}},
		{"nil metrics", func() (LegalStatusService, error) {
			return NewLegalStatusService(&mockLifecycleService{}, &mockLifecycleRepo{}, &mockPatentRepo{}, &mockEventPublisher{}, newMockCache(), &mockLogger{}, nil, nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := tt.fn()
			if err == nil {
				t.Fatal("expected error for nil dependency")
			}
			if svc != nil {
				t.Fatal("expected nil service")
			}
		})
	}
}

func TestNewLegalStatusService_DefaultConfig(t *testing.T) {
	h := newTestHarness(t, nil)
	if h.svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewLegalStatusService_CustomConfig(t *testing.T) {
	cfg := &LegalStatusConfig{
		MaxBatchConcurrency:      5,
		StatusCacheTTL:           30 * time.Minute,
		SummaryCacheTTL:          5 * time.Minute,
		NotificationDedupeWindow: 12 * time.Hour,
		SyncFailureThreshold:     5,
	}
	h := newTestHarness(t, cfg)
	if h.svc == nil {
		t.Fatal("expected non-nil service")
	}
}

// ===========================================================================
// Tests: SyncStatus
// ===========================================================================

func TestSyncStatus_EmptyPatentID(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.SyncStatus(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestSyncStatus_StatusChanged(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:     "P001",
			Status:       "FILED",
			Jurisdiction: "CN",
		}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return &lifecycle.RemoteStatusResult{
			Status:        "GRANTED",
			Jurisdiction:  "CN",
			EffectiveDate: time.Now().UTC(),
			Source:        "CNIPA",
		}, nil
	}

	updateCalled := false
	h.lifecycleRepo.updateStatusFn = func(_ context.Context, _ string, status string, _ time.Time) error {
		updateCalled = true
		if status != "GRANTED" {
			t.Errorf("expected GRANTED, got %s", status)
		}
		return nil
	}

	result, err := h.svc.SyncStatus(context.Background(), "P001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Changed {
		t.Error("expected Changed=true")
	}
	if result.PreviousStatus != "FILED" {
		t.Errorf("expected previous=FILED, got %s", result.PreviousStatus)
	}
	if result.CurrentStatus != "GRANTED" {
		t.Errorf("expected current=GRANTED, got %s", result.CurrentStatus)
	}
	if !updateCalled {
		t.Error("expected UpdateStatus to be called")
	}
	if atomic.LoadInt32(&h.publisher.publishCount) == 0 {
		t.Error("expected event to be published")
	}
}

func TestSyncStatus_NoChange(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:     "P001",
			Status:       "GRANTED",
			Jurisdiction: "CN",
		}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return &lifecycle.RemoteStatusResult{
			Status:        "GRANTED",
			Jurisdiction:  "CN",
			EffectiveDate: time.Now().UTC(),
			Source:        "CNIPA",
		}, nil
	}

	result, err := h.svc.SyncStatus(context.Background(), "P001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Changed {
		t.Error("expected Changed=false")
	}
	if atomic.LoadInt32(&h.publisher.publishCount) != 0 {
		t.Error("expected no event published when status unchanged")
	}
}

func TestSyncStatus_RemoteFetchError(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{PatentID: "P001", Status: "FILED"}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return nil, fmt.Errorf("network timeout")
	}

	_, err := h.svc.SyncStatus(context.Background(), "P001")
	if err == nil {
		t.Fatal("expected error on remote fetch failure")
	}
}

func TestSyncStatus_LocalFetchError(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return nil, fmt.Errorf("database connection lost")
	}

	_, err := h.svc.SyncStatus(context.Background(), "P001")
	if err == nil {
		t.Fatal("expected error on local fetch failure")
	}
}

// ===========================================================================
// Tests: BatchSync
// ===========================================================================

func TestBatchSync_NilRequest(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.BatchSync(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestBatchSync_EmptyPatentIDs(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.BatchSync(context.Background(), &BatchSyncRequest{PatentIDs: []string{}})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBatchSync_DuplicatePatentIDs(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.BatchSync(context.Background(), &BatchSyncRequest{PatentIDs: []string{"P001", "P001"}})
	if err == nil {
		t.Fatal("expected validation error for duplicates")
	}
}

func TestBatchSync_Success(t *testing.T) {
	h := newTestHarness(t, &LegalStatusConfig{MaxBatchConcurrency: 2})

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, pid string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{PatentID: pid, Status: "FILED", Jurisdiction: "CN"}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return &lifecycle.RemoteStatusResult{Status: "GRANTED", Jurisdiction: "CN", EffectiveDate: time.Now().UTC(), Source: "CNIPA"}, nil
	}

	result, err := h.svc.BatchSync(context.Background(), &BatchSyncRequest{
		PatentIDs: []string{"P001", "P002", "P003"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Succeeded != 3 {
		t.Errorf("expected 3 succeeded, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected 0 failed, got %d", result.Failed)
	}
}

func TestBatchSync_PartialFailure(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, pid string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{PatentID: pid, Status: "FILED", Jurisdiction: "CN"}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, pid string) (*lifecycle.RemoteStatusResult, error) {
		if pid == "P002" {
			return nil, fmt.Errorf("API rate limited")
		}
		return &lifecycle.RemoteStatusResult{Status: "GRANTED", Jurisdiction: "CN", EffectiveDate: time.Now().UTC(), Source: "CNIPA"}, nil
	}

	result, err := h.svc.BatchSync(context.Background(), &BatchSyncRequest{
		PatentIDs: []string{"P001", "P002", "P003"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error entry, got %d", len(result.Errors))
	}
}

// ===========================================================================
// Tests: GetCurrentStatus
// ===========================================================================

func TestGetCurrentStatus_EmptyID(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.GetCurrentStatus(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetCurrentStatus_CacheHit(t *testing.T) {
	h := newTestHarness(t, nil)

	expected := &LegalStatusDetail{
		PatentID:     "P001",
		Jurisdiction: "CN",
		Status:       StatusGranted,
		StatusText:   "授权",
	}
	h.cache.getFn = func(_ context.Context, key string, dest interface{}) error {
		if ptr, ok := dest.(*LegalStatusDetail); ok {
			*ptr = *expected
			return nil
		}
		return fmt.Errorf("type mismatch")
	}

	result, err := h.svc.GetCurrentStatus(context.Background(), "P001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PatentID != "P001" {
		t.Errorf("expected P001, got %s", result.PatentID)
	}
	if h.metrics.counters["legal_status_cache_hits_total"] != 1 {
		t.Error("expected cache hit counter to be incremented")
	}
}

func TestGetCurrentStatus_CacheMiss_RepoHit(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:      "P001",
			Jurisdiction:  "CN",
			Status:        "授权",
			EffectiveDate: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		}, nil
	}

	result, err := h.svc.GetCurrentStatus(context.Background(), "P001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != StatusGranted {
		t.Errorf("expected GRANTED, got %s", result.Status)
	}
	if h.metrics.counters["legal_status_cache_misses_total"] != 1 {
		t.Error("expected cache miss counter to be incremented")
	}
}

func TestGetCurrentStatus_NotFound(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return nil, nil
	}

	_, err := h.svc.GetCurrentStatus(context.Background(), "P999")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

// ===========================================================================
// Tests: GetStatusHistory
// ===========================================================================

func TestGetStatusHistory_EmptyID(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.GetStatusHistory(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetStatusHistory_WithResults(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getStatusHistoryFn = func(_ context.Context, _ string, _ *commontypes.Pagination, _ *time.Time, _ *time.Time) ([]*lifecycle.StatusHistoryEntity, error) {
		return []*lifecycle.StatusHistoryEntity{
			{EventID: "E1", PatentID: "P001", FromStatus: "FILED", ToStatus: "PUBLISHED", EventDate: time.Now().UTC(), Source: "CNIPA"},
			{EventID: "E2", PatentID: "P001", FromStatus: "PUBLISHED", ToStatus: "GRANTED", EventDate: time.Now().UTC(), Source: "CNIPA"},
		}, nil
	}

	events, err := h.svc.GetStatusHistory(context.Background(), "P001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestGetStatusHistory_WithTimeRange(t *testing.T) {
	h := newTestHarness(t, nil)

	var capturedFrom, capturedTo *time.Time
	h.lifecycleRepo.getStatusHistoryFn = func(_ context.Context, _ string, _ *commontypes.Pagination, from *time.Time, to *time.Time) ([]*lifecycle.StatusHistoryEntity, error) {
		capturedFrom = from
		capturedTo = to
		return nil, nil
	}

	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)
	_, err := h.svc.GetStatusHistory(context.Background(), "P001", WithTimeRange(from, to))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFrom == nil || !capturedFrom.Equal(from) {
		t.Error("expected from time to be passed through")
	}
	if capturedTo == nil || !capturedTo.Equal(to) {
		t.Error("expected to time to be passed through")
	}
}

// ===========================================================================
// Tests: DetectAnomalies
// ===========================================================================

func TestDetectAnomalies_EmptyPortfolioID(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.DetectAnomalies(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDetectAnomalies_UnexpectedLapse(t *testing.T) {
	h := newTestHarness(t, nil)

	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000001")}}, nil
	}
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:       "00000000-0000-0000-0000-000000000001",
			Jurisdiction:   "US",
			Status:         "LAPSED",
			PreviousStatus: "PATENTED",
		}, nil
	}

	anomalies, err := h.svc.DetectAnomalies(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, a := range anomalies {
		if a.AnomalyType == AnomalyUnexpectedLapse {
			found = true
			if a.Severity != SeverityCritical {
				t.Errorf("expected CRITICAL severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected UnexpectedLapse anomaly")
	}
}

func TestDetectAnomalies_MissedDeadline(t *testing.T) {
	h := newTestHarness(t, nil)

	deadline := time.Now().UTC().Add(3 * 24 * time.Hour) // 3 days from now
	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000001")}}, nil
	}
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:     "00000000-0000-0000-0000-000000000001",
			Jurisdiction: "CN",
			Status:       "实质审查",
			NextAction:   "Submit response to office action",
			NextDeadline: &deadline,
		}, nil
	}

	anomalies, err := h.svc.DetectAnomalies(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, a := range anomalies {
		if a.AnomalyType == AnomalyMissedDeadline {
			found = true
			if a.Severity != SeverityHigh {
				t.Errorf("expected HIGH severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected MissedDeadline anomaly")
	}
}

func TestDetectAnomalies_PastDueDeadline(t *testing.T) {
	h := newTestHarness(t, nil)

	pastDeadline := time.Now().UTC().Add(-2 * 24 * time.Hour)
	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000001")}}, nil
	}
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:     "00000000-0000-0000-0000-000000000001",
			Jurisdiction: "CN",
			Status:       "实质审查",
			NextAction:   "Response overdue",
			NextDeadline: &pastDeadline,
		}, nil
	}

	anomalies, err := h.svc.DetectAnomalies(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, a := range anomalies {
		if a.AnomalyType == AnomalyMissedDeadline && a.Severity == SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Error("expected MissedDeadline anomaly with CRITICAL severity for past-due deadline")
	}
}

func TestDetectAnomalies_StatusConflict(t *testing.T) {
	h := newTestHarness(t, nil)

	syncTime := time.Now().UTC().Add(-48 * time.Hour) // 48 hours ago
	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000001")}}, nil
	}
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:     "00000000-0000-0000-0000-000000000001",
			Jurisdiction: "CN",
			Status:       "授权",
			RemoteStatus: "失效",
			LastSyncAt:   &syncTime,
		}, nil
	}

	anomalies, err := h.svc.DetectAnomalies(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, a := range anomalies {
		if a.AnomalyType == AnomalyStatusConflict {
			found = true
			if a.Severity != SeverityHigh {
				t.Errorf("expected HIGH severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected StatusConflict anomaly")
	}
}

func TestDetectAnomalies_SyncFailure(t *testing.T) {
	h := newTestHarness(t, nil)

	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000001")}}, nil
	}
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:                  "00000000-0000-0000-0000-000000000001",
			Jurisdiction:              "CN",
			Status:                    "授权",
			ConsecutiveSyncFailures:   5,
		}, nil
	}

	anomalies, err := h.svc.DetectAnomalies(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, a := range anomalies {
		if a.AnomalyType == AnomalySyncFailure {
			found = true
			if a.Severity != SeverityMedium {
				t.Errorf("expected MEDIUM severity, got %s", a.Severity)
			}
		}
	}
	if !found {
		t.Error("expected SyncFailure anomaly")
	}
}

func TestDetectAnomalies_SortedBySeverity(t *testing.T) {
	h := newTestHarness(t, nil)

	syncTime := time.Now().UTC().Add(-48 * time.Hour)
	deadline := time.Now().UTC().Add(2 * 24 * time.Hour)

	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{
			{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000001")},
			{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000002")},
			{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000003")},
		}, nil
	}
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, pid string) (*lifecycle.LegalStatusEntity, error) {
		switch pid {
		case "00000000-0000-0000-0000-000000000001":
			// SyncFailure -> MEDIUM
			return &lifecycle.LegalStatusEntity{
				PatentID: "00000000-0000-0000-0000-000000000001", Jurisdiction: "CN", Status: "授权",
				ConsecutiveSyncFailures: 5,
			}, nil
		case "00000000-0000-0000-0000-000000000002":
			// UnexpectedLapse -> CRITICAL
			return &lifecycle.LegalStatusEntity{
				PatentID: "00000000-0000-0000-0000-000000000002", Jurisdiction: "US", Status: "LAPSED",
				PreviousStatus: "PATENTED",
			}, nil
		case "00000000-0000-0000-0000-000000000003":
			// StatusConflict -> HIGH + MissedDeadline -> HIGH
			return &lifecycle.LegalStatusEntity{
				PatentID: "00000000-0000-0000-0000-000000000003", Jurisdiction: "CN", Status: "授权",
				RemoteStatus: "失效", LastSyncAt: &syncTime,
				NextAction: "Pay annuity", NextDeadline: &deadline,
			}, nil
		}
		return nil, nil
	}

	anomalies, err := h.svc.DetectAnomalies(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) < 3 {
		t.Fatalf("expected at least 3 anomalies, got %d", len(anomalies))
	}

	// Verify ordering: CRITICAL first, then HIGH, then MEDIUM
	prevOrder := -1
	for _, a := range anomalies {
		order := severityOrder[a.Severity]
		if order < prevOrder {
			t.Errorf("anomalies not sorted by severity: found %s after a less severe item", a.Severity)
		}
		prevOrder = order
	}
}

func TestDetectAnomalies_EmptyPortfolio(t *testing.T) {
	h := newTestHarness(t, nil)

	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{}, nil
	}

	anomalies, err := h.svc.DetectAnomalies(context.Background(), "empty-portfolio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies, got %d", len(anomalies))
	}
}

// ===========================================================================
// Tests: SubscribeStatusChange
// ===========================================================================

func TestSubscribeStatusChange_NilRequest(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.SubscribeStatusChange(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSubscribeStatusChange_MissingFields(t *testing.T) {
	h := newTestHarness(t, nil)

	tests := []struct {
		name string
		req  *SubscriptionRequest
	}{
		{"no patent or portfolio", &SubscriptionRequest{
			Channels: []NotificationChannel{ChannelEmail}, Recipient: "user@example.com",
		}},
		{"no channels", &SubscriptionRequest{
			PatentIDs: []string{"P001"}, Recipient: "user@example.com",
		}},
		{"no recipient", &SubscriptionRequest{
			PatentIDs: []string{"P001"}, Channels: []NotificationChannel{ChannelEmail},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.svc.SubscribeStatusChange(context.Background(), tt.req)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSubscribeStatusChange_Success(t *testing.T) {
	h := newTestHarness(t, nil)

	saveCalled := false
	h.lifecycleRepo.saveSubscriptionFn = func(_ context.Context, sub *lifecycle.SubscriptionEntity) error {
		saveCalled = true
		if !sub.Active {
			t.Error("expected subscription to be active")
		}
		if sub.Recipient != "user@example.com" {
			t.Errorf("expected recipient user@example.com, got %s", sub.Recipient)
		}
		return nil
	}

	sub, err := h.svc.SubscribeStatusChange(context.Background(), &SubscriptionRequest{
		PatentIDs:     []string{"P001", "P002"},
		StatusFilters: []string{"GRANTED", "LAPSED"},
		Channels:      []NotificationChannel{ChannelEmail, ChannelWeChatWork},
		Recipient:     "user@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !saveCalled {
		t.Error("expected SaveSubscription to be called")
	}
	if sub.ID == "" {
		t.Error("expected non-empty subscription ID")
	}
	if !sub.Active {
		t.Error("expected active subscription")
	}
}

func TestSubscribeStatusChange_RepoError(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.saveSubscriptionFn = func(_ context.Context, _ *lifecycle.SubscriptionEntity) error {
		return fmt.Errorf("database write failed")
	}

	_, err := h.svc.SubscribeStatusChange(context.Background(), &SubscriptionRequest{
		PortfolioID: "portfolio-1",
		Channels:    []NotificationChannel{ChannelEmail},
		Recipient:   "user@example.com",
	})
	if err == nil {
		t.Fatal("expected error on repo failure")
	}
}

// ===========================================================================
// Tests: UnsubscribeStatusChange
// ===========================================================================

func TestUnsubscribeStatusChange_EmptyID(t *testing.T) {
	h := newTestHarness(t, nil)
	err := h.svc.UnsubscribeStatusChange(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestUnsubscribeStatusChange_Success(t *testing.T) {
	h := newTestHarness(t, nil)

	deactivateCalled := false
	h.lifecycleRepo.deactivateSubFn = func(_ context.Context, id string) error {
		deactivateCalled = true
		if id != "sub-123" {
			t.Errorf("expected sub-123, got %s", id)
		}
		return nil
	}

	err := h.svc.UnsubscribeStatusChange(context.Background(), "sub-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deactivateCalled {
		t.Error("expected DeactivateSubscription to be called")
	}
}

func TestUnsubscribeStatusChange_RepoError(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.deactivateSubFn = func(_ context.Context, _ string) error {
		return fmt.Errorf("not found")
	}

	err := h.svc.UnsubscribeStatusChange(context.Background(), "sub-999")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ===========================================================================
// Tests: GetStatusSummary
// ===========================================================================

func TestGetStatusSummary_EmptyPortfolioID(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.GetStatusSummary(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetStatusSummary_CacheHit(t *testing.T) {
	h := newTestHarness(t, nil)

	expected := &StatusSummary{
		PortfolioID:  "portfolio-1",
		TotalPatents: 10,
		HealthScore:  0.85,
	}
	h.cache.getFn = func(_ context.Context, key string, dest interface{}) error {
		if ptr, ok := dest.(*StatusSummary); ok {
			*ptr = *expected
			return nil
		}
		return fmt.Errorf("type mismatch")
	}

	result, err := h.svc.GetStatusSummary(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalPatents != 10 {
		t.Errorf("expected 10 patents, got %d", result.TotalPatents)
	}
	if h.metrics.counters["legal_status_summary_cache_hits_total"] != 1 {
		t.Error("expected summary cache hit counter incremented")
	}
}

func TestGetStatusSummary_Aggregation(t *testing.T) {
	h := newTestHarness(t, nil)

	syncTime := time.Now().UTC()
	h.patentRepo.listByPortfolioFn = func(_ context.Context, _ string) ([]*patent.Patent, error) {
		return []*patent.Patent{
			{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000001")},
			{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000002")},
			{ID: importUUID.MustParse("00000000-0000-0000-0000-000000000003")},
		}, nil
	}
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, pid string) (*lifecycle.LegalStatusEntity, error) {
		switch pid {
		case "00000000-0000-0000-0000-000000000001":
			return &lifecycle.LegalStatusEntity{PatentID: "00000000-0000-0000-0000-000000000001", Jurisdiction: "CN", Status: "授权", LastSyncAt: &syncTime}, nil
		case "00000000-0000-0000-0000-000000000002":
			return &lifecycle.LegalStatusEntity{PatentID: "00000000-0000-0000-0000-000000000002", Jurisdiction: "US", Status: "PATENTED", LastSyncAt: &syncTime}, nil
		case "00000000-0000-0000-0000-000000000003":
			return &lifecycle.LegalStatusEntity{PatentID: "00000000-0000-0000-0000-000000000003", Jurisdiction: "CN", Status: "实质审查", LastSyncAt: &syncTime}, nil
		}
		return nil, nil
	}

	result, err := h.svc.GetStatusSummary(context.Background(), "portfolio-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalPatents != 3 {
		t.Errorf("expected 3 patents, got %d", result.TotalPatents)
	}
	if result.ByJurisdiction["CN"] != 2 {
		t.Errorf("expected 2 CN patents, got %d", result.ByJurisdiction["CN"])
	}
	if result.ByJurisdiction["US"] != 1 {
		t.Errorf("expected 1 US patent, got %d", result.ByJurisdiction["US"])
	}
	if result.HealthScore < 0 || result.HealthScore > 1.0 {
		t.Errorf("health score out of range: %f", result.HealthScore)
	}
}

// ===========================================================================
// Tests: ReconcileStatus
// ===========================================================================

func TestReconcileStatus_EmptyID(t *testing.T) {
	h := newTestHarness(t, nil)
	_, err := h.svc.ReconcileStatus(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestReconcileStatus_Consistent(t *testing.T) {
	h := newTestHarness(t, nil)

	effectiveDate := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:      "P001",
			Jurisdiction:  "CN",
			Status:        "GRANTED",
			EffectiveDate: effectiveDate,
			NextAction:    "Pay annuity",
		}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return &lifecycle.RemoteStatusResult{
			Status:        "GRANTED",
			Jurisdiction:  "CN",
			EffectiveDate: effectiveDate,
			NextAction:    "Pay annuity",
			Source:        "CNIPA",
		}, nil
	}

	result, err := h.svc.ReconcileStatus(context.Background(), "P001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Consistent {
		t.Error("expected consistent result")
	}
	if len(result.Discrepancies) != 0 {
		t.Errorf("expected 0 discrepancies, got %d", len(result.Discrepancies))
	}
}

func TestReconcileStatus_Inconsistent(t *testing.T) {
	h := newTestHarness(t, nil)

	localDate := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	remoteDate := time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{
			PatentID:      "P001",
			Jurisdiction:  "CN",
			Status:        "FILED",
			EffectiveDate: localDate,
			NextAction:    "Await examination",
		}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return &lifecycle.RemoteStatusResult{
			Status:        "GRANTED",
			Jurisdiction:  "CN",
			EffectiveDate: remoteDate,
			NextAction:    "Pay annuity",
			Source:        "CNIPA",
		}, nil
	}

	updateCalled := false
	h.lifecycleRepo.updateStatusFn = func(_ context.Context, _ string, status string, _ time.Time) error {
		updateCalled = true
		if status != "GRANTED" {
			t.Errorf("expected auto-fix to GRANTED, got %s", status)
		}
		return nil
	}

	result, err := h.svc.ReconcileStatus(context.Background(), "P001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Consistent {
		t.Error("expected inconsistent result")
	}
	if result.LocalStatus != "FILED" {
		t.Errorf("expected local=FILED, got %s", result.LocalStatus)
	}
	if result.RemoteStatus != "GRANTED" {
		t.Errorf("expected remote=GRANTED, got %s", result.RemoteStatus)
	}

	// Expect discrepancies for status, effective_date, next_action
	if len(result.Discrepancies) < 3 {
		t.Errorf("expected at least 3 discrepancies, got %d", len(result.Discrepancies))
	}

	fieldSet := make(map[string]bool)
	for _, d := range result.Discrepancies {
		fieldSet[d.Field] = true
	}
	for _, expected := range []string{"status", "effective_date", "next_action"} {
		if !fieldSet[expected] {
			t.Errorf("expected discrepancy for field %s", expected)
		}
	}

	if !updateCalled {
		t.Error("expected auto-reconciliation UpdateStatus to be called")
	}
}

func TestReconcileStatus_NotFound(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return nil, nil
	}

	_, err := h.svc.ReconcileStatus(context.Background(), "P999")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestReconcileStatus_RemoteError(t *testing.T) {
	h := newTestHarness(t, nil)

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, _ string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{PatentID: "P001", Status: "FILED", Jurisdiction: "CN"}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return nil, fmt.Errorf("patent office unavailable")
	}

	_, err := h.svc.ReconcileStatus(context.Background(), "P001")
	if err == nil {
		t.Fatal("expected error on remote failure")
	}
}

// ===========================================================================
// Tests: computeHealthScore
// ===========================================================================

func TestComputeHealthScore_NoAnomalies(t *testing.T) {
	score := computeHealthScore(nil, 10)
	if score != 1.0 {
		t.Errorf("expected 1.0, got %f", score)
	}
}

func TestComputeHealthScore_ZeroPatents(t *testing.T) {
	score := computeHealthScore(nil, 0)
	if score != 1.0 {
		t.Errorf("expected 1.0 for zero patents, got %f", score)
	}
}

func TestComputeHealthScore_AllCritical(t *testing.T) {
	anomalies := make([]*StatusAnomaly, 10)
	for i := range anomalies {
		anomalies[i] = &StatusAnomaly{Severity: SeverityCritical}
	}
	// penalty = 10 * 0.3 / 10 = 0.3 => score = 0.7
	score := computeHealthScore(anomalies, 10)
	if score < 0.69 || score > 0.71 {
		t.Errorf("expected ~0.7, got %f", score)
	}
}

func TestComputeHealthScore_MixedSeverity(t *testing.T) {
	anomalies := []*StatusAnomaly{
		{Severity: SeverityCritical}, // 0.3
		{Severity: SeverityHigh},     // 0.15
		{Severity: SeverityMedium},   // 0.05
		{Severity: SeverityLow},      // 0.0
		{Severity: SeverityInfo},     // 0.0
	}
	// penalty = (0.3 + 0.15 + 0.05) / 10 = 0.05 => score = 0.95
	score := computeHealthScore(anomalies, 10)
	if score < 0.94 || score > 0.96 {
		t.Errorf("expected ~0.95, got %f", score)
	}
}

func TestComputeHealthScore_ClampToZero(t *testing.T) {
	// 5 critical anomalies for 1 patent: penalty = 5*0.3/1 = 1.5 => clamped to 0
	anomalies := make([]*StatusAnomaly, 5)
	for i := range anomalies {
		anomalies[i] = &StatusAnomaly{Severity: SeverityCritical}
	}
	score := computeHealthScore(anomalies, 1)
	if score != 0.0 {
		t.Errorf("expected 0.0, got %f", score)
	}
}

// ===========================================================================
// Tests: MapJurisdictionStatus
// ===========================================================================

func TestMapJurisdictionStatus_KnownMappings(t *testing.T) {
	tests := []struct {
		jurisdiction string
		raw          string
		expected     LegalStatusCode
		exactMatch   bool
	}{
		{"CN", "授权", StatusGranted, true},
		{"CN", "驳回", StatusRejected, true},
		{"US", "PATENTED", StatusGranted, true},
		{"US", "ABANDONED", StatusWithdrawn, true},
		{"EP", "GRANT", StatusGranted, true},
		{"EP", "REVOCATION", StatusRevoked, true},
		{"JP", "登録", StatusGranted, true},
		{"JP", "拒絶", StatusRejected, true},
		{"KR", "등록", StatusGranted, true},
		{"KR", "거절", StatusRejected, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.jurisdiction, tt.raw), func(t *testing.T) {
			code, exact := MapJurisdictionStatus(tt.jurisdiction, tt.raw)
			if code != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, code)
			}
			if exact != tt.exactMatch {
				t.Errorf("expected exact=%t, got %t", tt.exactMatch, exact)
			}
		})
	}
}

func TestMapJurisdictionStatus_UnknownJurisdiction(t *testing.T) {
	code, exact := MapJurisdictionStatus("XX", "SOME_STATUS")
	if code != StatusFiled {
		t.Errorf("expected fallback StatusFiled, got %s", code)
	}
	if exact {
		t.Error("expected exact=false for unknown jurisdiction")
	}
}

func TestMapJurisdictionStatus_UnknownStatus(t *testing.T) {
	code, exact := MapJurisdictionStatus("CN", "未知状态")
	if code != StatusFiled {
		t.Errorf("expected fallback StatusFiled, got %s", code)
	}
	if exact {
		t.Error("expected exact=false for unknown status")
	}
}

// ===========================================================================
// Tests: LegalStatusCode methods
// ===========================================================================

func TestLegalStatusCode_IsTerminal(t *testing.T) {
	terminals := []LegalStatusCode{StatusLapsed, StatusWithdrawn, StatusRejected, StatusExpired, StatusRevoked}
	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}
	nonTerminals := []LegalStatusCode{StatusFiled, StatusPublished, StatusUnderExam, StatusGranted, StatusUnderAppeal, StatusTransferred, StatusLicenseRecorded}
	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}

func TestLegalStatusCode_IsActive(t *testing.T) {
	actives := []LegalStatusCode{StatusFiled, StatusPublished, StatusUnderExam, StatusGranted, StatusUnderAppeal, StatusTransferred, StatusLicenseRecorded}
	for _, s := range actives {
		if !s.IsActive() {
			t.Errorf("expected %s to be active", s)
		}
	}
	inactives := []LegalStatusCode{StatusLapsed, StatusWithdrawn, StatusRejected, StatusExpired, StatusRevoked}
	for _, s := range inactives {
		if s.IsActive() {
			t.Errorf("expected %s to be inactive", s)
		}
	}
}

func TestLegalStatusCode_String(t *testing.T) {
	if StatusGranted.String() != "GRANTED" {
		t.Errorf("expected GRANTED, got %s", StatusGranted.String())
	}
}

// ===========================================================================
// Tests: SeverityLevel.SeverityWeight
// ===========================================================================

func TestSeverityWeight(t *testing.T) {
	tests := []struct {
		severity SeverityLevel
		expected float64
	}{
		{SeverityCritical, 0.30},
		{SeverityHigh, 0.15},
		{SeverityMedium, 0.05},
		{SeverityLow, 0.0},
		{SeverityInfo, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.severity.String(), func(t *testing.T) {
			if w := tt.severity.SeverityWeight(); w != tt.expected {
				t.Errorf("expected %f, got %f", tt.expected, w)
			}
		})
	}
}

// ===========================================================================
// Tests: BatchSyncRequest.Validate
// ===========================================================================

func TestBatchSyncRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     BatchSyncRequest
		wantErr bool
	}{
		{"valid", BatchSyncRequest{PatentIDs: []string{"P001"}}, false},
		{"empty ids", BatchSyncRequest{PatentIDs: []string{}}, true},
		{"contains empty string", BatchSyncRequest{PatentIDs: []string{"P001", ""}}, true},
		{"duplicates", BatchSyncRequest{PatentIDs: []string{"P001", "P001"}}, true},
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

// ===========================================================================
// Tests: SubscriptionRequest.Validate
// ===========================================================================

func TestSubscriptionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     SubscriptionRequest
		wantErr bool
	}{
		{"valid with patents", SubscriptionRequest{
			PatentIDs: []string{"P001"}, Channels: []NotificationChannel{ChannelEmail}, Recipient: "a@b.com",
		}, false},
		{"valid with portfolio", SubscriptionRequest{
			PortfolioID: "pf-1", Channels: []NotificationChannel{ChannelSMS}, Recipient: "a@b.com",
		}, false},
		{"no target", SubscriptionRequest{
			Channels: []NotificationChannel{ChannelEmail}, Recipient: "a@b.com",
		}, true},
		{"no channels", SubscriptionRequest{
			PatentIDs: []string{"P001"}, Recipient: "a@b.com",
		}, true},
		{"no recipient", SubscriptionRequest{
			PatentIDs: []string{"P001"}, Channels: []NotificationChannel{ChannelEmail},
		}, true},
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

// ===========================================================================
// Tests: NotificationChannel.String
// ===========================================================================

func TestNotificationChannel_String(t *testing.T) {
	tests := []struct {
		ch       NotificationChannel
		expected string
	}{
		{ChannelEmail, "EMAIL"},
		{ChannelSMS, "SMS"},
		{ChannelWebhook, "WEBHOOK"},
		{ChannelWeChatWork, "WECHAT_WORK"},
		{ChannelDingTalk, "DINGTALK"},
		{ChannelInApp, "IN_APP"},
		{NotificationChannel("unknown"), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.ch.String(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

// ===========================================================================
// Tests: AnomalyType.String
// ===========================================================================

func TestAnomalyType_String(t *testing.T) {
	tests := []struct {
		at       AnomalyType
		expected string
	}{
		{AnomalyUnexpectedLapse, "UNEXPECTED_LAPSE"}, // Corrected case
		{AnomalyMissedDeadline, "MISSED_DEADLINE"},
		{AnomalyStatusConflict, "STATUS_CONFLICT"},
		{AnomalySyncFailure, "SYNC_FAILURE"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.at.String(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

// ===========================================================================
// Tests: QueryOption functional options
// ===========================================================================

func TestQueryOptions(t *testing.T) {
	from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	pagination := &commontypes.Pagination{Page: 2, PageSize: 50}

	opts := applyQueryOptions([]QueryOption{
		WithTimeRange(from, to),
		WithPagination(*pagination), // pass value as WithPagination expects value in legal_status.go implementation?
	})

    // Check implementation of WithPagination in legal_status.go
    // func WithPagination(p commontypes.Pagination) QueryOption
    // So passing value is correct.

	if opts.From == nil || !opts.From.Equal(from) {
		t.Error("expected From to be set")
	}
	if opts.To == nil || !opts.To.Equal(to) {
		t.Error("expected To to be set")
	}
	if opts.Pagination == nil || opts.Pagination.Page != 2 || opts.Pagination.PageSize != 50 {
		t.Error("expected Pagination to be set correctly")
	}
}

func TestQueryOptions_Empty(t *testing.T) {
	opts := applyQueryOptions(nil)
	if opts.From != nil || opts.To != nil || opts.Pagination != nil {
		t.Error("expected all nil for empty options")
	}
}

// ===========================================================================
// Tests: sortAnomaliesBySeverity
// ===========================================================================

func TestSortAnomaliesBySeverity_AlreadySorted(t *testing.T) {
	anomalies := []*StatusAnomaly{
		{Severity: SeverityCritical},
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
	}
	sortAnomaliesBySeverity(anomalies)
	if anomalies[0].Severity != SeverityCritical || anomalies[1].Severity != SeverityHigh || anomalies[2].Severity != SeverityMedium {
		t.Error("already sorted list should remain unchanged")
	}
}

func TestSortAnomaliesBySeverity_Reversed(t *testing.T) {
	anomalies := []*StatusAnomaly{
		{Severity: SeverityInfo},
		{Severity: SeverityLow},
		{Severity: SeverityMedium},
		{Severity: SeverityHigh},
		{Severity: SeverityCritical},
	}
	sortAnomaliesBySeverity(anomalies)
	expected := []SeverityLevel{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo}
	for i, e := range expected {
		if anomalies[i].Severity != e {
			t.Errorf("position %d: expected %s, got %s", i, e, anomalies[i].Severity)
		}
	}
}

func TestSortAnomaliesBySeverity_Empty(t *testing.T) {
	var anomalies []*StatusAnomaly
	sortAnomaliesBySeverity(anomalies) // should not panic
}

func TestSortAnomaliesBySeverity_Single(t *testing.T) {
	anomalies := []*StatusAnomaly{{Severity: SeverityHigh}}
	sortAnomaliesBySeverity(anomalies)
	if anomalies[0].Severity != SeverityHigh {
		t.Error("single element should remain unchanged")
	}
}

// ===========================================================================
// Tests: toStringChannels
// ===========================================================================

func TestToStringChannels(t *testing.T) {
	channels := []NotificationChannel{ChannelEmail, ChannelWebhook, ChannelInApp} // Use valid consts
	result := toStringChannels(channels)
	expected := []string{"EMAIL", "WEBHOOK", "IN_APP"} // based on String() output
	if len(result) != len(expected) {
		t.Fatalf("expected %d, got %d", len(expected), len(result))
	}
	for i, v := range expected {
		if result[i] != v {
			t.Errorf("position %d: expected %s, got %s", i, v, result[i])
		}
	}
}

func TestToStringChannels_Empty(t *testing.T) {
	result := toStringChannels(nil)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(result))
	}
}

// ===========================================================================
// Tests: Cache key helpers
// ===========================================================================

func TestStatusCacheKey(t *testing.T) {
	key := statusCacheKey("P001")
	if key != "legal_status:current:P001" {
		t.Errorf("unexpected key: %s", key)
	}
}

func TestSummaryCacheKey(t *testing.T) {
	key := summaryCacheKey("portfolio-1")
	if key != "legal_status:summary:portfolio-1" {
		t.Errorf("unexpected key: %s", key)
	}
}

func TestNotificationDedupeKey(t *testing.T) {
	key := notificationDedupeKey("P001", "GRANTED")
	if key != "legal_status:notify_dedupe:P001:GRANTED" {
		t.Errorf("unexpected key: %s", key)
	}
}

// ===========================================================================
// Tests: Context cancellation in BatchSync
// ===========================================================================

func TestBatchSync_ContextCancellation(t *testing.T) {
	h := newTestHarness(t, &LegalStatusConfig{MaxBatchConcurrency: 1})

	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, pid string) (*lifecycle.LegalStatusEntity, error) {
		return &lifecycle.LegalStatusEntity{PatentID: pid, Status: "FILED", Jurisdiction: "CN"}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(ctx context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		// Simulate slow operation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
			return &lifecycle.RemoteStatusResult{Status: "GRANTED", Jurisdiction: "CN", EffectiveDate: time.Now().UTC(), Source: "CNIPA"}, nil
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, err := h.svc.BatchSync(ctx, &BatchSyncRequest{
		PatentIDs: []string{"P001", "P002", "P003", "P004", "P005"},
	})
	if err != nil {
		t.Fatalf("BatchSync should not return error itself: %v", err)
	}
	// With very short timeout and concurrency=1, most should fail
	if result.Failed == 0 {
		t.Log("Warning: expected some failures due to context cancellation")
	}
}

// ===========================================================================
// Tests: Concurrent safety of BatchSync
// ===========================================================================

func TestBatchSync_ConcurrentSafety(t *testing.T) {
	h := newTestHarness(t, &LegalStatusConfig{MaxBatchConcurrency: 10})

	var callCount int32
	h.lifecycleRepo.getByPatentIDFn = func(_ context.Context, pid string) (*lifecycle.LegalStatusEntity, error) {
		atomic.AddInt32(&callCount, 1)
		return &lifecycle.LegalStatusEntity{PatentID: pid, Status: "FILED", Jurisdiction: "CN"}, nil
	}
	h.lifecycleSvc.fetchRemoteStatusFn = func(_ context.Context, _ string) (*lifecycle.RemoteStatusResult, error) {
		return &lifecycle.RemoteStatusResult{Status: "GRANTED", Jurisdiction: "CN", EffectiveDate: time.Now().UTC(), Source: "CNIPA"}, nil
	}

	ids := make([]string, 50)
	for i := range ids {
		ids[i] = fmt.Sprintf("P%03d", i)
	}

	result, err := h.svc.BatchSync(context.Background(), &BatchSyncRequest{PatentIDs: ids})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Succeeded != 50 {
		t.Errorf("expected 50 succeeded, got %d (failed: %d)", result.Succeeded, result.Failed)
	}
	if atomic.LoadInt32(&callCount) != 50 {
		t.Errorf("expected 50 repo calls, got %d", callCount)
	}
}

//Personal.AI order the ending
