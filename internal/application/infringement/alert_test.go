// Phase 10 - File 199 of 349
package infringement

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// --- Mock implementations ---

type mockAlertRepository struct {
	mu         sync.Mutex
	alerts     map[string]*Alert
	saveErr    error
	updateErr  error
	listErr    error
	statsErr   error
	overSLAErr error
}

func newMockAlertRepository() *mockAlertRepository {
	return &mockAlertRepository{
		alerts: make(map[string]*Alert),
	}
}

func (m *mockAlertRepository) Save(ctx context.Context, alert *Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr != nil {
		return m.saveErr
	}
	m.alerts[alert.ID] = alert
	return nil
}

func (m *mockAlertRepository) FindByID(ctx context.Context, id string) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.alerts[id]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (m *mockAlertRepository) Update(ctx context.Context, alert *Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateErr != nil {
		return m.updateErr
	}
	m.alerts[alert.ID] = alert
	return nil
}

func (m *mockAlertRepository) List(ctx context.Context, opts AlertListOptions) ([]*Alert, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	var result []*Alert
	for _, a := range m.alerts {
		if opts.WatchlistID != "" && a.WatchlistID != opts.WatchlistID {
			continue
		}
		if opts.Level != nil && a.Level != *opts.Level {
			continue
		}
		if opts.Status != nil && a.Status != *opts.Status {
			continue
		}
		result = append(result, a)
	}
	return result, len(result), nil
}

func (m *mockAlertRepository) FindDuplicate(ctx context.Context, patentNumber, moleculeID string, since time.Time) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.PatentNumber == patentNumber && a.MoleculeID == moleculeID && a.CreatedAt.After(since) {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockAlertRepository) GetStats(ctx context.Context, watchlistID string) (*AlertStats, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.statsErr != nil {
		return nil, m.statsErr
	}
	stats := &AlertStats{
		ByLevel: make(map[string]int),
	}
	for _, a := range m.alerts {
		if watchlistID != "" && a.WatchlistID != watchlistID {
			continue
		}
		switch a.Status {
		case AlertStatusOpen:
			stats.TotalOpen++
		case AlertStatusAcknowledged:
			stats.TotalAcknowledged++
		case AlertStatusDismissed:
			stats.TotalDismissed++
		case AlertStatusEscalated:
			stats.TotalEscalated++
		case AlertStatusResolved:
			stats.TotalResolved++
		}
		stats.ByLevel[a.Level.String()]++
	}
	return stats, nil
}

func (m *mockAlertRepository) FindOverSLA(ctx context.Context) ([]*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.overSLAErr != nil {
		return nil, m.overSLAErr
	}
	var result []*Alert
	for _, a := range m.alerts {
		if a.Status == AlertStatusOpen {
			elapsed := time.Since(a.CreatedAt)
			if elapsed > a.Level.SLADuration() {
				result = append(result, a)
			}
		}
	}
	return result, nil
}

type mockAlertProducer struct {
	mu       sync.Mutex
	messages []*commontypes.ProducerMessage
	publishErr error
}

func newMockAlertProducer() *mockAlertProducer {
	return &mockAlertProducer{}
}

func (m *mockAlertProducer) Publish(ctx context.Context, msg *commontypes.ProducerMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishErr != nil {
		return m.publishErr
	}
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockAlertProducer) PublishBatch(ctx context.Context, msgs []*commontypes.ProducerMessage) (*commontypes.BatchPublishResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishErr != nil {
		return nil, m.publishErr
	}
	m.messages = append(m.messages, msgs...)
	return &commontypes.BatchPublishResult{Succeeded: len(msgs)}, nil
}

func (m *mockAlertProducer) PublishAsync(ctx context.Context, msg *commontypes.ProducerMessage) {
	m.Publish(ctx, msg)
}

func (m *mockAlertProducer) Close() error { return nil }

type mockAlertCache struct {
	mu    sync.Mutex
	store map[string]any
}

func newMockAlertCache() *mockAlertCache {
	return &mockAlertCache{store: make(map[string]any)}
}

func (m *mockAlertCache) Get(ctx context.Context, key string, dest any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.store[key]; !ok {
		return fmt.Errorf("cache miss")
	}
	return nil
}

func (m *mockAlertCache) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	return nil
}

func (m *mockAlertCache) Delete(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

func (m *mockAlertCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.store[key]
	return ok, nil
}

func (m *mockAlertCache) Decr(ctx context.Context, key string) (int64, error) {
	return 0, nil // Dummy implementation
}

func (m *mockAlertCache) DeleteByPrefix(ctx context.Context, prefix string) (int64, error) {
	return 0, nil // Dummy implementation
}

func (m *mockAlertCache) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return nil
}

func (m *mockAlertCache) GetOrSet(ctx context.Context, key string, dest interface{}, expiration time.Duration, fetch func(context.Context) (interface{}, error)) error {
	// Simple implementation: try Get, if fail, call fetch and Set
	if err := m.Get(ctx, key, dest); err == nil {
		return nil
	}
	val, err := fetch(ctx)
	if err != nil {
		return err
	}
	// We need to set the value. But Set takes interface{}.
	// And Get unmarshals. This mock stores []byte.
	// This is getting complicated for a mock.
	// Assuming Set handles marshalling.
	if err := m.Set(ctx, key, val, expiration); err != nil {
		return err
	}
	// And now we need to put it into dest.
	// Since we just Set it, we can Get it again?
	// Or just assign? We can't assign to interface{}.
	// Let's rely on Get.
	return m.Get(ctx, key, dest)
}

// Implement remaining redis.Cache interface methods with dummy implementations
func (m *mockAlertCache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) { return nil, nil }
func (m *mockAlertCache) MSet(ctx context.Context, items map[string]interface{}, ttl time.Duration) error { return nil }
func (m *mockAlertCache) HGet(ctx context.Context, key, field string) (string, error) { return "", nil }
func (m *mockAlertCache) HSet(ctx context.Context, key string, fields map[string]interface{}, ttl time.Duration) error { return nil }
func (m *mockAlertCache) HGetAll(ctx context.Context, key string) (map[string]string, error) { return nil, nil }
func (m *mockAlertCache) HDel(ctx context.Context, key string, fields ...string) error { return nil }
func (m *mockAlertCache) Incr(ctx context.Context, key string) (int64, error) { return 0, nil }
func (m *mockAlertCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) { return 0, nil }
// Decr is already implemented
func (m *mockAlertCache) ZAdd(ctx context.Context, key string, members ...*redis.ZMember) error { return nil }
func (m *mockAlertCache) ZRangeByScore(ctx context.Context, key string, min, max float64, offset, count int64) ([]string, error) { return nil, nil }
func (m *mockAlertCache) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]*redis.ZMember, error) { return nil, nil }
func (m *mockAlertCache) ZRem(ctx context.Context, key string, members ...string) error { return nil }
func (m *mockAlertCache) ZScore(ctx context.Context, key, member string) (float64, error) { return 0, nil }
func (m *mockAlertCache) TTL(ctx context.Context, key string) (time.Duration, error) { return 0, nil }
func (m *mockAlertCache) Ping(ctx context.Context) error { return nil }

type mockAlertLogger struct{}

func (m *mockAlertLogger) Sync() error { return nil }

func (m *mockAlertLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockAlertLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockAlertLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockAlertLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockAlertLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockAlertLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockAlertLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockAlertLogger) WithError(err error) logging.Logger { return m }

// --- Helper ---

func newTestAlertService(repo *mockAlertRepository, producer *mockAlertProducer, cache *mockAlertCache) AlertService {
	return NewAlertService(
		repo,
		nil, // patent service not used directly in alert creation
		producer,
		cache,
		&mockAlertLogger{},
		AlertServiceConfig{DefaultDedupWindow: 24 * time.Hour},
	)
}

// --- Tests ---

func TestCreateAlert_Success(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-001",
		MoleculeID:      "MOL-001",
		WatchlistID:     "WL-001",
		Level:           AlertLevelHigh,
		Title:           "Potential infringement detected",
		Description:     "Structural similarity exceeds threshold",
		RiskScore:       0.85,
		SimilarityScore: 0.92,
		AssigneeID:      "user-001",
	}

	alert, err := svc.CreateAlert(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if alert == nil {
		t.Fatal("expected alert, got nil")
	}
	if alert.Status != AlertStatusOpen {
		t.Errorf("expected status OPEN, got %s", alert.Status.String())
	}
	if alert.Level != AlertLevelHigh {
		t.Errorf("expected level HIGH, got %s", alert.Level.String())
	}
	if len(producer.messages) == 0 {
		t.Error("expected dispatch messages, got none")
	}
}

func TestCreateAlert_NilRequest(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	_, err := svc.CreateAlert(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestCreateAlert_ValidationFailure(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber: "",
		MoleculeID:   "MOL-001",
		Level:        AlertLevelHigh,
		Title:        "Test",
	}

	_, err := svc.CreateAlert(context.Background(), req)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCreateAlert_Deduplication(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-002",
		MoleculeID:      "MOL-002",
		WatchlistID:     "WL-001",
		Level:           AlertLevelMedium,
		Title:           "Duplicate test",
		RiskScore:       0.7,
		SimilarityScore: 0.8,
	}

	first, err := svc.CreateAlert(context.Background(), req)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	second, err := svc.CreateAlert(context.Background(), req)
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("expected dedup to return same alert, got %s vs %s", first.ID, second.ID)
	}

	// Only one alert should exist in the repo.
	repo.mu.Lock()
	count := len(repo.alerts)
	repo.mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 alert in repo after dedup, got %d", count)
	}
}

func TestAcknowledgeAlert_Success(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-003",
		MoleculeID:      "MOL-003",
		WatchlistID:     "WL-001",
		Level:           AlertLevelHigh,
		Title:           "Ack test",
		RiskScore:       0.9,
		SimilarityScore: 0.95,
	}

	alert, _ := svc.CreateAlert(context.Background(), req)

	err := svc.AcknowledgeAlert(context.Background(), alert.ID, "user-002")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updated, _ := svc.GetAlert(context.Background(), alert.ID)
	if updated.Status != AlertStatusAcknowledged {
		t.Errorf("expected ACKNOWLEDGED, got %s", updated.Status.String())
	}
	if updated.AcknowledgedAt == nil {
		t.Error("expected AcknowledgedAt to be set")
	}
}

func TestAcknowledgeAlert_NotFound(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	err := svc.AcknowledgeAlert(context.Background(), "nonexistent", "user-001")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestAcknowledgeAlert_InvalidStatus(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-004",
		MoleculeID:      "MOL-004",
		WatchlistID:     "WL-001",
		Level:           AlertLevelLow,
		Title:           "Status test",
		RiskScore:       0.3,
		SimilarityScore: 0.4,
	}

	alert, _ := svc.CreateAlert(context.Background(), req)
	_ = svc.DismissAlert(context.Background(), &DismissAlertRequest{
		AlertID: alert.ID,
		Reason:  "false positive",
	}, "user-001")

	err := svc.AcknowledgeAlert(context.Background(), alert.ID, "user-002")
	if err == nil {
		t.Fatal("expected error for dismissed alert")
	}
}

func TestDismissAlert_Success(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-005",
		MoleculeID:      "MOL-005",
		WatchlistID:     "WL-001",
		Level:           AlertLevelMedium,
		Title:           "Dismiss test",
		RiskScore:       0.5,
		SimilarityScore: 0.6,
	}

	alert, _ := svc.CreateAlert(context.Background(), req)

	err := svc.DismissAlert(context.Background(), &DismissAlertRequest{
		AlertID: alert.ID,
		Reason:  "confirmed false positive after manual review",
	}, "user-003")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updated, _ := svc.GetAlert(context.Background(), alert.ID)
	if updated.Status != AlertStatusDismissed {
		t.Errorf("expected DISMISSED, got %s", updated.Status.String())
	}
	if updated.DismissReason == "" {
		t.Error("expected dismiss reason to be set")
	}
}

func TestEscalateAlert_Success(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-006",
		MoleculeID:      "MOL-006",
		WatchlistID:     "WL-001",
		Level:           AlertLevelMedium,
		Title:           "Escalate test",
		RiskScore:       0.75,
		SimilarityScore: 0.88,
	}

	alert, _ := svc.CreateAlert(context.Background(), req)
	initialMsgCount := len(producer.messages)

	err := svc.EscalateAlert(context.Background(), alert.ID, "requires immediate attention")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updated, _ := svc.GetAlert(context.Background(), alert.ID)
	if updated.Status != AlertStatusEscalated {
		t.Errorf("expected ESCALATED, got %s", updated.Status.String())
	}
	if updated.EscalatedAt == nil {
		t.Error("expected EscalatedAt to be set")
	}

	// Escalation should trigger re-dispatch with all channels.
	if len(producer.messages) <= initialMsgCount {
		t.Error("expected additional dispatch messages after escalation")
	}
}

func TestEscalateAlert_DismissedCannotEscalate(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-007",
		MoleculeID:      "MOL-007",
		WatchlistID:     "WL-001",
		Level:           AlertLevelLow,
		Title:           "Cannot escalate dismissed",
		RiskScore:       0.2,
		SimilarityScore: 0.3,
	}

	alert, _ := svc.CreateAlert(context.Background(), req)
	_ = svc.DismissAlert(context.Background(), &DismissAlertRequest{
		AlertID: alert.ID,
		Reason:  "not relevant",
	}, "user-001")

	err := svc.EscalateAlert(context.Background(), alert.ID, "try escalate")
	if err == nil {
		t.Fatal("expected error when escalating dismissed alert")
	}
}

func TestListAlerts_Pagination(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	for i := 0; i < 5; i++ {
		req := &CreateAlertRequest{
			PatentNumber:    fmt.Sprintf("US-2024-LIST-%d", i),
			MoleculeID:      fmt.Sprintf("MOL-LIST-%d", i),
			WatchlistID:     "WL-LIST",
			Level:           AlertLevelMedium,
			Title:           fmt.Sprintf("List test %d", i),
			RiskScore:       0.5,
			SimilarityScore: 0.6,
		}
		_, _ = svc.CreateAlert(context.Background(), req)
	}

	alerts, pagination, err := svc.ListAlerts(context.Background(), AlertListOptions{
		WatchlistID: "WL-LIST",
		Page:        1,
		PageSize:    10,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(alerts) != 5 {
		t.Errorf("expected 5 alerts, got %d", len(alerts))
	}
	if pagination == nil {
		t.Fatal("expected pagination result")
	}
	if pagination.Total != 5 {
		t.Errorf("expected total 5, got %d", pagination.Total)
	}
}

func TestListAlerts_DefaultPagination(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	_, pagination, err := svc.ListAlerts(context.Background(), AlertListOptions{
		Page:     0,
		PageSize: 0,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pagination.Page != 1 {
		t.Errorf("expected default page 1, got %d", pagination.Page)
	}
	if pagination.PageSize != 20 {
		t.Errorf("expected default page size 20, got %d", pagination.PageSize)
	}
}

func TestGetAlert_Success(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	req := &CreateAlertRequest{
		PatentNumber:    "US-2024-GET",
		MoleculeID:      "MOL-GET",
		WatchlistID:     "WL-001",
		Level:           AlertLevelCritical,
		Title:           "Get test",
		RiskScore:       0.99,
		SimilarityScore: 0.99,
	}

	created, _ := svc.CreateAlert(context.Background(), req)

	fetched, err := svc.GetAlert(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("expected ID %s, got %s", created.ID, fetched.ID)
	}
}

func TestGetAlert_NotFound(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	_, err := svc.GetAlert(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestGetAlert_EmptyID(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	_, err := svc.GetAlert(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty ID")
	}
}

func TestUpdateAlertConfig_Success(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	err := svc.UpdateAlertConfig(context.Background(), &AlertConfigRequest{
		WatchlistID: "WL-CFG",
		ChannelMapping: map[AlertLevel]DispatchChannel{
			AlertLevelCritical: DispatchChannelSMS | DispatchChannelEmail,
			AlertLevelHigh:     DispatchChannelEmail,
			AlertLevelMedium:   DispatchChannelInApp,
			AlertLevelLow:      DispatchChannelInApp,
		},
		QuietHours: &QuietHoursConfig{
			Enabled:  true,
			Start:    "22:00",
			End:      "07:00",
			Timezone: "Asia/Shanghai",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpdateAlertConfig_NilRequest(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	err := svc.UpdateAlertConfig(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestGetAlertStats_Success(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	for i := 0; i < 3; i++ {
		req := &CreateAlertRequest{
			PatentNumber:    fmt.Sprintf("US-STAT-%d", i),
			MoleculeID:      fmt.Sprintf("MOL-STAT-%d", i),
			WatchlistID:     "WL-STATS",
			Level:           AlertLevel(i%4 + 1),
			Title:           fmt.Sprintf("Stats test %d", i),
			RiskScore:       0.5,
			SimilarityScore: 0.6,
		}
		_, _ = svc.CreateAlert(context.Background(), req)
	}

	stats, err := svc.GetAlertStats(context.Background(), "WL-STATS")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stats.TotalOpen != 3 {
		t.Errorf("expected 3 open alerts, got %d", stats.TotalOpen)
	}
}

func TestGetAlertStats_EmptyWatchlist(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	_, err := svc.GetAlertStats(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error for empty watchlist_id")
	}
}

func TestProcessOverSLAAlerts_NoOverdue(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	count, err := svc.ProcessOverSLAAlerts(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 escalated, got %d", count)
	}
}

func TestProcessOverSLAAlerts_WithOverdue(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	// Manually insert an alert with a creation time far in the past to exceed SLA.
	oldAlert := &Alert{
		ID:           "ALT-overdue-001",
		PatentNumber: "US-OVERDUE-001",
		MoleculeID:   "MOL-OVERDUE-001",
		WatchlistID:  "WL-SLA",
		Level:        AlertLevelCritical, // SLA = 2 hours
		Status:       AlertStatusOpen,
		Title:        "Overdue alert",
		CreatedAt:    time.Now().UTC().Add(-5 * time.Hour), // 5 hours ago, exceeds 2h SLA
		Channels:     DispatchChannelInApp,
	}
	repo.mu.Lock()
	repo.alerts[oldAlert.ID] = oldAlert
	repo.mu.Unlock()

	count, err := svc.ProcessOverSLAAlerts(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 escalated, got %d", count)
	}

	// Verify the alert was escalated.
	repo.mu.Lock()
	updated := repo.alerts[oldAlert.ID]
	repo.mu.Unlock()
	if updated.Status != AlertStatusEscalated {
		t.Errorf("expected ESCALATED, got %s", updated.Status.String())
	}
}

func TestAlertLevel_String(t *testing.T) {
	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{AlertLevelLow, "LOW"},
		{AlertLevelMedium, "MEDIUM"},
		{AlertLevelHigh, "HIGH"},
		{AlertLevelCritical, "CRITICAL"},
		{AlertLevel(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.expected {
			t.Errorf("AlertLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
		}
	}
}

func TestAlertStatus_String(t *testing.T) {
	tests := []struct {
		status   AlertStatus
		expected string
	}{
		{AlertStatusOpen, "OPEN"},
		{AlertStatusAcknowledged, "ACKNOWLEDGED"},
		{AlertStatusDismissed, "DISMISSED"},
		{AlertStatusEscalated, "ESCALATED"},
		{AlertStatusResolved, "RESOLVED"},
		{AlertStatus(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.expected {
			t.Errorf("AlertStatus(%d).String() = %s, want %s", tt.status, got, tt.expected)
		}
	}
}

func TestAlertLevel_SLADuration(t *testing.T) {
	if AlertLevelCritical.SLADuration() != 2*time.Hour {
		t.Errorf("critical SLA expected 2h, got %v", AlertLevelCritical.SLADuration())
	}
	if AlertLevelHigh.SLADuration() != 8*time.Hour {
		t.Errorf("high SLA expected 8h, got %v", AlertLevelHigh.SLADuration())
	}
	if AlertLevelMedium.SLADuration() != 24*time.Hour {
		t.Errorf("medium SLA expected 24h, got %v", AlertLevelMedium.SLADuration())
	}
	if AlertLevelLow.SLADuration() != 72*time.Hour {
		t.Errorf("low SLA expected 72h, got %v", AlertLevelLow.SLADuration())
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input   string
		h, m    int
		wantErr bool
	}{
		{"22:00", 22, 0, false},
		{"07:30", 7, 30, false},
		{"00:00", 0, 0, false},
		{"23:59", 23, 59, false},
		{"24:00", 0, 0, true},
		{"12:60", 0, 0, true},
		{"invalid", 0, 0, true},
		{"", 0, 0, true},
	}
	for _, tt := range tests {
		h, m, err := parseHHMM(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseHHMM(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseHHMM(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if h != tt.h || m != tt.m {
			t.Errorf("parseHHMM(%q) = (%d,%d), want (%d,%d)", tt.input, h, m, tt.h, tt.m)
		}
	}
}

func TestGenerateAlertID_Deterministic(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	id1 := generateAlertID("US-001", "MOL-001", ts)
	id2 := generateAlertID("US-001", "MOL-001", ts)
	if id1 != id2 {
		t.Errorf("expected deterministic IDs, got %s vs %s", id1, id2)
	}

	id3 := generateAlertID("US-002", "MOL-001", ts)
	if id1 == id3 {
		t.Error("expected different IDs for different patents")
	}
}

func TestCreateAlertRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateAlertRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: CreateAlertRequest{
				PatentNumber: "US-001", MoleculeID: "MOL-001",
				Level: AlertLevelHigh, Title: "Test", RiskScore: 0.5, SimilarityScore: 0.5,
			},
			wantErr: false,
		},
		{
			name: "missing patent",
			req: CreateAlertRequest{
				MoleculeID: "MOL-001", Level: AlertLevelHigh, Title: "Test",
			},
			wantErr: true,
		},
		{
			name: "missing molecule",
			req: CreateAlertRequest{
				PatentNumber: "US-001", Level: AlertLevelHigh, Title: "Test",
			},
			wantErr: true,
		},
		{
			name: "invalid level low bound",
			req: CreateAlertRequest{
				PatentNumber: "US-001", MoleculeID: "MOL-001",
				Level: 0, Title: "Test",
			},
			wantErr: true,
		},
		{
			name: "invalid level high bound",
			req: CreateAlertRequest{
				PatentNumber: "US-001", MoleculeID: "MOL-001",
				Level: 5, Title: "Test",
			},
			wantErr: true,
		},
		{
			name: "missing title",
			req: CreateAlertRequest{
				PatentNumber: "US-001", MoleculeID: "MOL-001", Level: AlertLevelHigh,
			},
			wantErr: true,
		},
		{
			name: "risk score out of range",
			req: CreateAlertRequest{
				PatentNumber: "US-001", MoleculeID: "MOL-001",
				Level: AlertLevelHigh, Title: "Test", RiskScore: 1.5,
			},
			wantErr: true,
		},
		{
			name: "similarity score negative",
			req: CreateAlertRequest{
				PatentNumber: "US-001", MoleculeID: "MOL-001",
				Level: AlertLevelHigh, Title: "Test", SimilarityScore: -0.1,
			},
			wantErr: true,
		},
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

func TestDismissAlertRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     DismissAlertRequest
		wantErr bool
	}{
		{"valid", DismissAlertRequest{AlertID: "ALT-001", Reason: "false positive"}, false},
		{"missing id", DismissAlertRequest{Reason: "reason"}, true},
		{"missing reason", DismissAlertRequest{AlertID: "ALT-001"}, true},
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

func TestChannelRouting_DefaultByLevel(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache).(*alertServiceImpl)

	// Critical: all channels
	ch := svc.resolveChannels("unknown-wl", AlertLevelCritical)
	if ch&DispatchChannelSMS == 0 {
		t.Error("critical should include SMS")
	}
	if ch&DispatchChannelWeChat == 0 {
		t.Error("critical should include WeChat")
	}

	// Low: in-app only
	ch = svc.resolveChannels("unknown-wl", AlertLevelLow)
	if ch != DispatchChannelInApp {
		t.Errorf("low should be in-app only, got %d", ch)
	}
}

func TestChannelRouting_CustomConfig(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	_ = svc.UpdateAlertConfig(context.Background(), &AlertConfigRequest{
		WatchlistID: "WL-CUSTOM",
		ChannelMapping: map[AlertLevel]DispatchChannel{
			AlertLevelLow: DispatchChannelEmail | DispatchChannelInApp,
		},
	})

	impl := svc.(*alertServiceImpl)
	ch := impl.resolveChannels("WL-CUSTOM", AlertLevelLow)
	if ch&DispatchChannelEmail == 0 {
		t.Error("custom config should include email for low level")
	}
}

func TestDispatchAlert_ProducerError(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	producer.publishErr = fmt.Errorf("kafka unavailable")
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	// Alert creation should still succeed even if dispatch fails.
	req := &CreateAlertRequest{
		PatentNumber:    "US-DISPATCH-ERR",
		MoleculeID:      "MOL-DISPATCH-ERR",
		WatchlistID:     "WL-001",
		Level:           AlertLevelHigh,
		Title:           "Dispatch error test",
		RiskScore:       0.8,
		SimilarityScore: 0.9,
	}

	alert, err := svc.CreateAlert(context.Background(), req)
	if err != nil {
		t.Fatalf("alert creation should succeed despite dispatch error, got %v", err)
	}
	if alert == nil {
		t.Fatal("expected alert to be created")
	}
}

// Ensure PaginationResult is used correctly.
func TestListAlerts_PaginationResult(t *testing.T) {
	repo := newMockAlertRepository()
	producer := newMockAlertProducer()
	cache := newMockAlertCache()
	svc := newTestAlertService(repo, producer, cache)

	_, pagination, err := svc.ListAlerts(context.Background(), AlertListOptions{
		PageSize: 200, // exceeds max
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pagination.PageSize != 100 {
		t.Errorf("expected capped page size 100, got %d", pagination.PageSize)
	}
}

// Verify unused import guard — commontypes.PaginationResult is used.
var _ *commontypes.PaginationResult
