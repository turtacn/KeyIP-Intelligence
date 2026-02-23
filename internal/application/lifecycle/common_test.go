package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	importUUID "github.com/google/uuid"
	importCommon "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Mock implementations (Shared across tests)
// ---------------------------------------------------------------------------

type mockLifecycleService struct {
	calculateFn       func(ctx context.Context, patentID string, j domainLifecycle.Jurisdiction, asOf time.Time) (*domainLifecycle.AnnuityCalcResult, error)
	scheduleFn        func(ctx context.Context, patentID string, j domainLifecycle.Jurisdiction, start, end time.Time) ([]domainLifecycle.ScheduleEntry, error)
	fetchRemoteStatusFn func(ctx context.Context, patentID string) (*domainLifecycle.RemoteStatusResult, error)
}

func (m *mockLifecycleService) CalculateAnnuityFee(ctx context.Context, patentID string, j domainLifecycle.Jurisdiction, asOf time.Time) (*domainLifecycle.AnnuityCalcResult, error) {
	if m.calculateFn != nil {
		return m.calculateFn(ctx, patentID, j, asOf)
	}
	return &domainLifecycle.AnnuityCalcResult{
		YearNumber:     3,
		Fee:            900.0,
		DueDate:        asOf.AddDate(0, 3, 0),
		GracePeriodEnd: asOf.AddDate(0, 6, 0),
		Status:         "pending",
	}, nil
}

func (m *mockLifecycleService) GetAnnuitySchedule(ctx context.Context, patentID string, j domainLifecycle.Jurisdiction, start, end time.Time) ([]domainLifecycle.ScheduleEntry, error) {
	if m.scheduleFn != nil {
		return m.scheduleFn(ctx, patentID, j, start, end)
	}
	return []domainLifecycle.ScheduleEntry{
		{YearNumber: 3, Fee: 900.0, DueDate: start.AddDate(0, 3, 0), GracePeriodEnd: start.AddDate(0, 6, 0), Status: "pending"},
		{YearNumber: 4, Fee: 1200.0, DueDate: start.AddDate(1, 3, 0), GracePeriodEnd: start.AddDate(1, 6, 0), Status: "pending"},
	}, nil
}

func (m *mockLifecycleService) FetchRemoteStatus(ctx context.Context, patentID string) (*domainLifecycle.RemoteStatusResult, error) {
	if m.fetchRemoteStatusFn != nil {
		return m.fetchRemoteStatusFn(ctx, patentID)
	}
	return &domainLifecycle.RemoteStatusResult{
		Status:        "GRANTED",
		EffectiveDate: time.Now().AddDate(-1, 0, 0),
		Source:        "mock",
		Jurisdiction:  "CN",
	}, nil
}

type mockLifecycleRepo struct {
	savePaymentFn        func(ctx context.Context, p *domainLifecycle.PaymentRecord) (*domainLifecycle.PaymentRecord, error)
	queryPaymentFn       func(ctx context.Context, q *domainLifecycle.PaymentQuery) ([]domainLifecycle.PaymentRecord, int64, error)
	getByPatentIDFn      func(ctx context.Context, patentID string) (*domainLifecycle.LegalStatusEntity, error)
	updateStatusFn       func(ctx context.Context, patentID string, status string, effectiveDate time.Time) error
	saveSubscriptionFn   func(ctx context.Context, sub *domainLifecycle.SubscriptionEntity) error
	deactivateSubFn      func(ctx context.Context, id string) error
	getStatusHistoryFn   func(ctx context.Context, patentID string, pagination *importCommon.Pagination, from, to *time.Time) ([]*domainLifecycle.StatusHistoryEntity, error)
	saveCustomEventFn    func(ctx context.Context, event *domainLifecycle.CustomEvent) error
	getCustomEventsFn    func(ctx context.Context, patentIDs []string, start, end time.Time) ([]domainLifecycle.CustomEvent, error)
	updateEventStatusFn  func(ctx context.Context, eventID string, status string) error
	deleteEventFn        func(ctx context.Context, eventID string) error
}

func (m *mockLifecycleRepo) SavePayment(ctx context.Context, p *domainLifecycle.PaymentRecord) (*domainLifecycle.PaymentRecord, error) {
	if m.savePaymentFn != nil {
		return m.savePaymentFn(ctx, p)
	}
	p.ID = "pay-001"
	p.RecordedAt = time.Now()
	return p, nil
}

func (m *mockLifecycleRepo) QueryPayments(ctx context.Context, q *domainLifecycle.PaymentQuery) ([]domainLifecycle.PaymentRecord, int64, error) {
	if m.queryPaymentFn != nil {
		return m.queryPaymentFn(ctx, q)
	}
	return []domainLifecycle.PaymentRecord{
		{ID: "pay-001", PatentID: q.PatentID, YearNumber: 2, Amount: 600, Currency: "CNY", PaidDate: time.Now().AddDate(0, -1, 0), RecordedAt: time.Now()},
	}, 1, nil
}

// Implement other methods as needed, returning nil/empty by default or using fn hooks
func (m *mockLifecycleRepo) CreateAnnuity(ctx context.Context, annuity *domainLifecycle.Annuity) error { return nil }
func (m *mockLifecycleRepo) GetAnnuity(ctx context.Context, id importUUID.UUID) (*domainLifecycle.Annuity, error) { return nil, nil }
func (m *mockLifecycleRepo) GetAnnuitiesByPatent(ctx context.Context, patentID importUUID.UUID) ([]*domainLifecycle.Annuity, error) { return nil, nil }
func (m *mockLifecycleRepo) GetUpcomingAnnuities(ctx context.Context, daysAhead int, limit, offset int) ([]*domainLifecycle.Annuity, int64, error) { return nil, 0, nil }
func (m *mockLifecycleRepo) GetOverdueAnnuities(ctx context.Context, limit, offset int) ([]*domainLifecycle.Annuity, int64, error) { return nil, 0, nil }
func (m *mockLifecycleRepo) UpdateAnnuityStatus(ctx context.Context, id importUUID.UUID, status domainLifecycle.AnnuityStatus, paidAmount int64, paidDate *time.Time, paymentRef string) error { return nil }
func (m *mockLifecycleRepo) BatchCreateAnnuities(ctx context.Context, annuities []*domainLifecycle.Annuity) error { return nil }
func (m *mockLifecycleRepo) UpdateReminderSent(ctx context.Context, id importUUID.UUID) error { return nil }

func (m *mockLifecycleRepo) CreateDeadline(ctx context.Context, deadline *domainLifecycle.Deadline) error { return nil }
func (m *mockLifecycleRepo) GetDeadline(ctx context.Context, id importUUID.UUID) (*domainLifecycle.Deadline, error) { return nil, nil }
func (m *mockLifecycleRepo) GetDeadlinesByPatent(ctx context.Context, patentID importUUID.UUID, statusFilter []domainLifecycle.DeadlineStatus) ([]*domainLifecycle.Deadline, error) { return nil, nil }
func (m *mockLifecycleRepo) GetActiveDeadlines(ctx context.Context, userID *importUUID.UUID, daysAhead int, limit, offset int) ([]*domainLifecycle.Deadline, int64, error) { return nil, 0, nil }
func (m *mockLifecycleRepo) UpdateDeadlineStatus(ctx context.Context, id importUUID.UUID, status domainLifecycle.DeadlineStatus, completedBy *importUUID.UUID) error { return nil }
func (m *mockLifecycleRepo) ExtendDeadline(ctx context.Context, id importUUID.UUID, newDueDate time.Time, reason string) error { return nil }
func (m *mockLifecycleRepo) GetCriticalDeadlines(ctx context.Context, limit int) ([]*domainLifecycle.Deadline, error) { return nil, nil }

func (m *mockLifecycleRepo) CreateEvent(ctx context.Context, event *domainLifecycle.LifecycleEvent) error { return nil }
func (m *mockLifecycleRepo) GetEventsByPatent(ctx context.Context, patentID importUUID.UUID, eventTypes []domainLifecycle.EventType, limit, offset int) ([]*domainLifecycle.LifecycleEvent, int64, error) { return nil, 0, nil }
func (m *mockLifecycleRepo) GetEventTimeline(ctx context.Context, patentID importUUID.UUID) ([]*domainLifecycle.LifecycleEvent, error) { return nil, nil }
func (m *mockLifecycleRepo) GetRecentEvents(ctx context.Context, orgID importUUID.UUID, limit int) ([]*domainLifecycle.LifecycleEvent, error) { return nil, nil }

func (m *mockLifecycleRepo) CreateCostRecord(ctx context.Context, record *domainLifecycle.CostRecord) error { return nil }
func (m *mockLifecycleRepo) GetCostsByPatent(ctx context.Context, patentID importUUID.UUID) ([]*domainLifecycle.CostRecord, error) { return nil, nil }
func (m *mockLifecycleRepo) GetCostSummary(ctx context.Context, patentID importUUID.UUID) (*domainLifecycle.CostSummary, error) { return nil, nil }
func (m *mockLifecycleRepo) GetPortfolioCostSummary(ctx context.Context, portfolioID importUUID.UUID, startDate, endDate time.Time) (*domainLifecycle.PortfolioCostSummary, error) { return nil, nil }

func (m *mockLifecycleRepo) GetLifecycleDashboard(ctx context.Context, orgID importUUID.UUID) (*domainLifecycle.DashboardStats, error) { return nil, nil }

func (m *mockLifecycleRepo) WithTx(ctx context.Context, fn func(domainLifecycle.LifecycleRepository) error) error { return fn(m) }

// LegalStatus specific methods
func (m *mockLifecycleRepo) GetByPatentID(ctx context.Context, patentID string) (*domainLifecycle.LegalStatusEntity, error) {
	if m.getByPatentIDFn != nil {
		return m.getByPatentIDFn(ctx, patentID)
	}
	return &domainLifecycle.LegalStatusEntity{
		PatentID: patentID,
		Jurisdiction: "CN",
		Status: "GRANTED",
		EffectiveDate: time.Now().AddDate(-1, 0, 0),
	}, nil
}

func (m *mockLifecycleRepo) UpdateStatus(ctx context.Context, patentID string, status string, effectiveDate time.Time) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, patentID, status, effectiveDate)
	}
	return nil
}

func (m *mockLifecycleRepo) SaveSubscription(ctx context.Context, sub *domainLifecycle.SubscriptionEntity) error {
	if m.saveSubscriptionFn != nil {
		return m.saveSubscriptionFn(ctx, sub)
	}
	return nil
}

func (m *mockLifecycleRepo) DeactivateSubscription(ctx context.Context, id string) error {
	if m.deactivateSubFn != nil {
		return m.deactivateSubFn(ctx, id)
	}
	return nil
}

func (m *mockLifecycleRepo) GetStatusHistory(ctx context.Context, patentID string, pagination *importCommon.Pagination, from, to *time.Time) ([]*domainLifecycle.StatusHistoryEntity, error) {
	if m.getStatusHistoryFn != nil {
		return m.getStatusHistoryFn(ctx, patentID, pagination, from, to)
	}
	return nil, nil
}

// Calendar specific methods
func (m *mockLifecycleRepo) SaveCustomEvent(ctx context.Context, event *domainLifecycle.CustomEvent) error {
	if m.saveCustomEventFn != nil {
		return m.saveCustomEventFn(ctx, event)
	}
	return nil
}

func (m *mockLifecycleRepo) GetCustomEvents(ctx context.Context, patentIDs []string, start, end time.Time) ([]domainLifecycle.CustomEvent, error) {
	if m.getCustomEventsFn != nil {
		return m.getCustomEventsFn(ctx, patentIDs, start, end)
	}
	return nil, nil
}

func (m *mockLifecycleRepo) UpdateEventStatus(ctx context.Context, eventID string, status string) error {
	if m.updateEventStatusFn != nil {
		return m.updateEventStatusFn(ctx, eventID, status)
	}
	return nil
}

func (m *mockLifecycleRepo) DeleteEvent(ctx context.Context, eventID string) error {
	if m.deleteEventFn != nil {
		return m.deleteEventFn(ctx, eventID)
	}
	return nil
}

type mockPatentInfo struct {
	ID           string
	PatentNumber string
	Title        string
	Jurisdiction string
	FilingDate   time.Time
}

type mockPatentRepo struct {
	patents map[string]*mockPatentInfo
	listByPortfolioFn func(ctx context.Context, portfolioID string) ([]*domainPatent.Patent, error)
}

func newMockPatentRepo(patents ...*mockPatentInfo) *mockPatentRepo {
	m := &mockPatentRepo{patents: make(map[string]*mockPatentInfo)}
	for _, p := range patents {
		m.patents[p.ID] = p
	}
	return m
}

func (m *mockPatentRepo) GetByID(ctx context.Context, id importUUID.UUID) (*domainPatent.Patent, error) {
	p, ok := m.patents[id.String()]
	if !ok {
		return nil, fmt.Errorf("not found: %s", id)
	}
	fd := p.FilingDate
	return &domainPatent.Patent{
		ID:           id,
		PatentNumber: p.PatentNumber,
		Title:        p.Title,
		Jurisdiction: p.Jurisdiction,
		Dates:        domainPatent.PatentDate{FilingDate: &fd},
		FilingDate:   &fd,
	}, nil
}

func (m *mockPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*domainPatent.Patent, error) {
	if m.listByPortfolioFn != nil {
		return m.listByPortfolioFn(ctx, portfolioID)
	}
	var result []*domainPatent.Patent
	for _, p := range m.patents {
		uid, _ := importUUID.Parse(p.ID)
		fd := p.FilingDate
		result = append(result, &domainPatent.Patent{
			ID:           uid,
			PatentNumber: p.PatentNumber,
			Title:        p.Title,
			Jurisdiction: p.Jurisdiction,
			Dates:        domainPatent.PatentDate{FilingDate: &fd},
			FilingDate:   &fd,
		})
	}
	return result, nil
}

// implement other interface methods for PatentRepository if needed, returning defaults
func (m *mockPatentRepo) Create(ctx context.Context, p *domainPatent.Patent) error { return nil }
func (m *mockPatentRepo) GetByPatentNumber(ctx context.Context, number string) (*domainPatent.Patent, error) { return nil, nil }
func (m *mockPatentRepo) Update(ctx context.Context, p *domainPatent.Patent) error { return nil }
func (m *mockPatentRepo) SoftDelete(ctx context.Context, id importUUID.UUID) error { return nil }
func (m *mockPatentRepo) Restore(ctx context.Context, id importUUID.UUID) error { return nil }
func (m *mockPatentRepo) HardDelete(ctx context.Context, id importUUID.UUID) error { return nil }
func (m *mockPatentRepo) Search(ctx context.Context, query domainPatent.SearchQuery) (*domainPatent.SearchResult, error) { return nil, nil }
func (m *mockPatentRepo) GetByFamilyID(ctx context.Context, familyID string) ([]*domainPatent.Patent, error) { return nil, nil }
func (m *mockPatentRepo) GetByAssignee(ctx context.Context, assigneeID importUUID.UUID, limit, offset int) ([]*domainPatent.Patent, int64, error) { return nil, 0, nil }
func (m *mockPatentRepo) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*domainPatent.Patent, int64, error) { return nil, 0, nil }
func (m *mockPatentRepo) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*domainPatent.Patent, int64, error) { return nil, 0, nil }
func (m *mockPatentRepo) FindDuplicates(ctx context.Context, fullTextHash string) ([]*domainPatent.Patent, error) { return nil, nil }
func (m *mockPatentRepo) CreateClaim(ctx context.Context, claim *domainPatent.Claim) error { return nil }
func (m *mockPatentRepo) GetClaimsByPatent(ctx context.Context, patentID importUUID.UUID) ([]*domainPatent.Claim, error) { return nil, nil }
func (m *mockPatentRepo) UpdateClaim(ctx context.Context, claim *domainPatent.Claim) error { return nil }
func (m *mockPatentRepo) DeleteClaimsByPatent(ctx context.Context, patentID importUUID.UUID) error { return nil }
func (m *mockPatentRepo) BatchCreateClaims(ctx context.Context, claims []*domainPatent.Claim) error { return nil }
func (m *mockPatentRepo) GetIndependentClaims(ctx context.Context, patentID importUUID.UUID) ([]*domainPatent.Claim, error) { return nil, nil }
func (m *mockPatentRepo) SetInventors(ctx context.Context, patentID importUUID.UUID, inventors []*domainPatent.Inventor) error { return nil }
func (m *mockPatentRepo) GetInventors(ctx context.Context, patentID importUUID.UUID) ([]*domainPatent.Inventor, error) { return nil, nil }
func (m *mockPatentRepo) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*domainPatent.Patent, int64, error) { return nil, 0, nil }
func (m *mockPatentRepo) SetPriorityClaims(ctx context.Context, patentID importUUID.UUID, claims []*domainPatent.PriorityClaim) error { return nil }
func (m *mockPatentRepo) GetPriorityClaims(ctx context.Context, patentID importUUID.UUID) ([]*domainPatent.PriorityClaim, error) { return nil, nil }
func (m *mockPatentRepo) BatchCreate(ctx context.Context, patents []*domainPatent.Patent) (int, error) { return 0, nil }
func (m *mockPatentRepo) BatchUpdateStatus(ctx context.Context, ids []importUUID.UUID, status domainPatent.PatentStatus) (int64, error) { return 0, nil }
func (m *mockPatentRepo) CountByStatus(ctx context.Context) (map[domainPatent.PatentStatus]int64, error) { return nil, nil }
func (m *mockPatentRepo) CountByJurisdiction(ctx context.Context) (map[string]int64, error) { return nil, nil }
func (m *mockPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return nil, nil }
func (m *mockPatentRepo) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) { return nil, nil }
func (m *mockPatentRepo) WithTx(ctx context.Context, fn func(domainPatent.PatentRepository) error) error { return fn(m) }
func (m *mockPatentRepo) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error { return nil }
func (m *mockPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*domainPatent.Patent, error) { return nil, nil }

type mockExchangeRate struct {
	rates map[string]float64
}

func newMockExchangeRate() *mockExchangeRate {
	return &mockExchangeRate{rates: map[string]float64{
		"CNY_USD": 0.14,
		"USD_CNY": 7.15,
		"EUR_CNY": 7.80,
		"JPY_CNY": 0.048,
		"KRW_CNY": 0.0054,
		"CNY_EUR": 0.128,
		"CNY_JPY": 20.83,
		"CNY_KRW": 185.19,
		"USD_EUR": 0.92,
		"EUR_USD": 1.09,
	}}
}

func (m *mockExchangeRate) GetRate(ctx context.Context, from, to Currency) (float64, error) {
	key := fmt.Sprintf("%s_%s", from, to)
	r, ok := m.rates[key]
	if !ok {
		return 0, fmt.Errorf("no rate for %s->%s", from, to)
	}
	return r, nil
}

type mockCache struct {
	mu       sync.Mutex
	store    map[string]interface{}
	getFn    func(ctx context.Context, key string, dest interface{}) error
	setFn    func(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	deleteFn func(ctx context.Context, keys ...string) error
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]interface{})}
}

func (m *mockCache) Get(ctx context.Context, key string, dest interface{}) error {
	if m.getFn != nil {
		return m.getFn(ctx, key, dest)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.store[key]
	if !ok {
		return fmt.Errorf("cache miss: %s", key)
	}
	switch d := dest.(type) {
	case *float64:
		if f, ok := v.(float64); ok {
			*d = f
			return nil
		}
	case *AnnuityResult:
		if r, ok := v.(*AnnuityResult); ok {
			*d = *r
			return nil
		}
	case *LegalStatusDetail:
		if r, ok := v.(*LegalStatusDetail); ok {
			*d = *r
			return nil
		}
	case *StatusSummary:
		if r, ok := v.(*StatusSummary); ok {
			*d = *r
			return nil
		}
	}
	return fmt.Errorf("type mismatch or unknown type for key %s", key)
}

func (m *mockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.setFn != nil {
		return m.setFn(ctx, key, value, ttl)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, keys ...string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, keys...)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.store, k)
	}
	return nil
}

type mockLogger struct{
	mu       sync.Mutex
	messages []string
}

func (m *mockLogger) Info(msg string, _ ...interface{})  { m.mu.Lock(); defer m.mu.Unlock(); m.messages = append(m.messages, msg) }
func (m *mockLogger) Warn(msg string, _ ...interface{})  { m.mu.Lock(); defer m.mu.Unlock(); m.messages = append(m.messages, msg) }
func (m *mockLogger) Error(msg string, _ ...interface{}) { m.mu.Lock(); defer m.mu.Unlock(); m.messages = append(m.messages, msg) }
func (m *mockLogger) Debug(msg string, _ ...interface{}) { m.mu.Lock(); defer m.mu.Unlock(); m.messages = append(m.messages, msg) }

type mockValueProvider struct {
	scores map[string]float64
}

func newMockValueProvider(scores map[string]float64) *mockValueProvider {
	return &mockValueProvider{scores: scores}
}

func (m *mockValueProvider) GetValueScore(_ context.Context, patentID string) (float64, error) {
	s, ok := m.scores[patentID]
	if !ok {
		return 0, fmt.Errorf("no score for %s", patentID)
	}
	return s, nil
}

type mockEventPublisher struct {
	publishFn    func(ctx context.Context, topic string, key string, payload interface{}) error
	publishCount int32
}

func (m *mockEventPublisher) Publish(ctx context.Context, topic string, key string, payload interface{}) error {
	atomic.AddInt32(&m.publishCount, 1)
	if m.publishFn != nil {
		return m.publishFn(ctx, topic, key, payload)
	}
	return nil
}

type mockMetrics struct {
	mu         sync.Mutex
	counters   map[string]int
	histograms map[string][]float64
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{
		counters:   make(map[string]int),
		histograms: make(map[string][]float64),
	}
}

func (m *mockMetrics) IncCounter(name string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name]++
}

func (m *mockMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.histograms[name] = append(m.histograms[name], value)
}

//Personal.AI order the ending
