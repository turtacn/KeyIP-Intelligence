// internal/application/lifecycle/annuity_test.go
//
// Phase 10 - File #207
// Unit tests for AnnuityService application service.

package lifecycle

import (
	"context"
	"testing"
	"time"

	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Mock implementations (Moved to common_test.go)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Helper to build a fully-wired service for tests
// ---------------------------------------------------------------------------

func newTestAnnuityService(opts ...func(*testServiceOpts)) AnnuityService {
	o := &testServiceOpts{
		lifecycleSvc:  &mockLifecycleService{},
		lifecycleRepo: &mockLifecycleRepo{},
		patentRepo: newMockPatentRepo(&mockPatentInfo{
			ID: "00000000-0000-0000-0000-000000000001", PatentNumber: "CN202310001234.5",
			Title: "Test Patent", Jurisdiction: "CN",
			FilingDate: time.Now().AddDate(-3, 0, 0),
		}),
		exchangeRate:  newMockExchangeRate(),
		valueProvider: newMockValueProvider(map[string]float64{"00000000-0000-0000-0000-000000000001": 30.0}),
		cache:         newMockCache(),
		logger:        &mockLogger{},
		cfg: AnnuityServiceConfig{
			DefaultCurrency:       CurrencyCNY,
			ValueScoreThreshold:   40.0,
			DefaultForecastYears:  5,
			BatchConcurrencyLimit: 5,
		},
	}
	for _, fn := range opts {
		fn(o)
	}
	return NewAnnuityService(
		o.lifecycleSvc, o.lifecycleRepo, o.patentRepo,
		o.exchangeRate, o.valueProvider, o.cache, o.logger, o.cfg,
	)
}

type testServiceOpts struct {
	lifecycleSvc  domainLifecycle.Service
	lifecycleRepo domainLifecycle.LifecycleRepository
	patentRepo    domainPatent.PatentRepository
	exchangeRate  ExchangeRateProvider
	valueProvider PatentValueProvider
	cache         common.CachePort
	logger        common.Logger
	cfg           AnnuityServiceConfig
}

// ---------------------------------------------------------------------------
// Tests: CalculateAnnuity
// ---------------------------------------------------------------------------

func TestCalculateAnnuity_Success(t *testing.T) {
	svc := newTestAnnuityService()
	ctx := context.Background()

	result, err := svc.CalculateAnnuity(ctx, &CalculateAnnuityRequest{
		PatentID:       "00000000-0000-0000-0000-000000000001",
		Jurisdiction:   domainLifecycle.JurisdictionCN,
		TargetCurrency: CurrencyCNY,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.PatentID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("expected patent_id 00000000-0000-0000-0000-000000000001, got %s", result.PatentID)
	}
	if result.YearNumber != 3 {
		t.Errorf("expected year 3, got %d", result.YearNumber)
	}
	if result.BaseFee.Amount != 900.0 {
		t.Errorf("expected base fee 900, got %.2f", result.BaseFee.Amount)
	}
	if result.BaseFee.Currency != CurrencyCNY {
		t.Errorf("expected CNY, got %s", result.BaseFee.Currency)
	}
	// Same currency -> converted == base
	if result.ConvertedFee.Amount != 900.0 {
		t.Errorf("expected converted fee 900 (same currency), got %.2f", result.ConvertedFee.Amount)
	}
}

func TestCalculateAnnuity_CurrencyConversion(t *testing.T) {
	svc := newTestAnnuityService()
	ctx := context.Background()

	result, err := svc.CalculateAnnuity(ctx, &CalculateAnnuityRequest{
		PatentID:       "00000000-0000-0000-0000-000000000001",
		Jurisdiction:   domainLifecycle.JurisdictionCN,
		TargetCurrency: CurrencyUSD,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 900 CNY * 0.14 = 126 USD
	expected := 900.0 * 0.14
	if diff := result.ConvertedFee.Amount - expected; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("expected %.2f USD, got %.2f", expected, result.ConvertedFee.Amount)
	}
	if result.ConvertedFee.Currency != CurrencyUSD {
		t.Errorf("expected USD, got %s", result.ConvertedFee.Currency)
	}
}

func TestCalculateAnnuity_NilRequest(t *testing.T) {
	svc := newTestAnnuityService()
	_, err := svc.CalculateAnnuity(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestCalculateAnnuity_MissingPatentID(t *testing.T) {
	svc := newTestAnnuityService()
	_, err := svc.CalculateAnnuity(context.Background(), &CalculateAnnuityRequest{
		Jurisdiction: domainLifecycle.JurisdictionCN,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCalculateAnnuity_PatentNotFound(t *testing.T) {
	svc := newTestAnnuityService()
	_, err := svc.CalculateAnnuity(context.Background(), &CalculateAnnuityRequest{
		PatentID:     "00000000-0000-0000-0000-000000000000",
		Jurisdiction: domainLifecycle.JurisdictionCN,
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestCalculateAnnuity_CacheHit(t *testing.T) {
	cache := newMockCache()
	cached := &AnnuityResult{
		PatentID:     "00000000-0000-0000-0000-000000000001",
		YearNumber:   3,
		BaseFee:      MoneyAmount{Amount: 900, Currency: CurrencyCNY},
		ConvertedFee: MoneyAmount{Amount: 900, Currency: CurrencyCNY},
	}
	cKey := annuityCacheKey("00000000-0000-0000-0000-000000000001", domainLifecycle.JurisdictionCN)
	cache.store[cKey] = cached

	svc := newTestAnnuityService(func(o *testServiceOpts) {
		o.cache = cache
	})

	result, err := svc.CalculateAnnuity(context.Background(), &CalculateAnnuityRequest{
		PatentID:       "00000000-0000-0000-0000-000000000001",
		Jurisdiction:   domainLifecycle.JurisdictionCN,
		TargetCurrency: CurrencyCNY,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BaseFee.Amount != 900 {
		t.Errorf("expected cached fee 900, got %.2f", result.BaseFee.Amount)
	}
}

// ---------------------------------------------------------------------------
// Tests: BatchCalculate
// ---------------------------------------------------------------------------

func TestBatchCalculate_Success(t *testing.T) {
	svc := newTestAnnuityService(func(o *testServiceOpts) {
		o.patentRepo = newMockPatentRepo(
			&mockPatentInfo{ID: "00000000-0000-0000-0000-000000000001", PatentNumber: "CN001", Title: "P1", Jurisdiction: "CN", FilingDate: time.Now().AddDate(-3, 0, 0)},
			&mockPatentInfo{ID: "00000000-0000-0000-0000-000000000002", PatentNumber: "CN002", Title: "P2", Jurisdiction: "CN", FilingDate: time.Now().AddDate(-5, 0, 0)},
		)
	})

	resp, err := svc.BatchCalculate(context.Background(), &BatchCalculateRequest{
		PatentIDs:      []string{"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002"},
		Jurisdiction:   domainLifecycle.JurisdictionCN,
		TargetCurrency: CurrencyCNY,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
	if len(resp.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(resp.Errors))
	}
	if resp.TotalFee.Amount != 1800.0 {
		t.Errorf("expected total 1800, got %.2f", resp.TotalFee.Amount)
	}
}

func TestBatchCalculate_PartialFailure(t *testing.T) {
	svc := newTestAnnuityService()

	resp, err := svc.BatchCalculate(context.Background(), &BatchCalculateRequest{
		PatentIDs:    []string{"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000000"},
		Jurisdiction: domainLifecycle.JurisdictionCN,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Errorf("expected 1 success, got %d", len(resp.Results))
	}
	if len(resp.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(resp.Errors))
	}
}

func TestBatchCalculate_EmptyIDs(t *testing.T) {
	svc := newTestAnnuityService()
	_, err := svc.BatchCalculate(context.Background(), &BatchCalculateRequest{
		PatentIDs: []string{},
	})
	if err == nil {
		t.Fatal("expected validation error for empty IDs")
	}
}

// ---------------------------------------------------------------------------
// Tests: RecordPayment
// ---------------------------------------------------------------------------

func TestRecordPayment_Success(t *testing.T) {
	svc := newTestAnnuityService()

	record, err := svc.RecordPayment(context.Background(), &RecordPaymentRequest{
		PatentID:     "00000000-0000-0000-0000-000000000001",
		Jurisdiction: domainLifecycle.JurisdictionCN,
		YearNumber:   3,
		Amount:       MoneyAmount{Amount: 900, Currency: CurrencyCNY},
		PaidDate:     time.Now(),
		PaymentRef:   "REF-001",
		PaidBy:       "admin",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.ID != "pay-001" {
		t.Errorf("expected ID pay-001, got %s", record.ID)
	}
	if record.Amount.Amount != 900 {
		t.Errorf("expected amount 900, got %.2f", record.Amount.Amount)
	}
}

func TestRecordPayment_InvalidAmount(t *testing.T) {
	svc := newTestAnnuityService()
	_, err := svc.RecordPayment(context.Background(), &RecordPaymentRequest{
		PatentID:     "00000000-0000-0000-0000-000000000001",
		Jurisdiction: domainLifecycle.JurisdictionCN,
		YearNumber:   3,
		Amount:       MoneyAmount{Amount: -10, Currency: CurrencyCNY},
		PaidDate:     time.Now(),
	})
	if err == nil {
		t.Fatal("expected validation error for negative amount")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetPaymentHistory
// ---------------------------------------------------------------------------

func TestGetPaymentHistory_Success(t *testing.T) {
	svc := newTestAnnuityService()

	records, total, err := svc.GetPaymentHistory(context.Background(), &PaymentHistoryRequest{
		PatentID: "00000000-0000-0000-0000-000000000001",
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestGetPaymentHistory_NilRequest(t *testing.T) {
	svc := newTestAnnuityService()
	_, _, err := svc.GetPaymentHistory(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

// ---------------------------------------------------------------------------
// Tests: helper functions
// ---------------------------------------------------------------------------

func TestJurisdictionBaseCurrency(t *testing.T) {
	tests := []struct {
		j    domainLifecycle.Jurisdiction
		want Currency
	}{
		{domainLifecycle.JurisdictionCN, CurrencyCNY},
		{domainLifecycle.JurisdictionUS, CurrencyUSD},
		{domainLifecycle.JurisdictionEP, CurrencyEUR},
		{domainLifecycle.JurisdictionJP, CurrencyJPY},
		{domainLifecycle.JurisdictionKR, CurrencyKRW},
		{"XX", CurrencyUSD},
	}
	for _, tt := range tests {
		got := jurisdictionBaseCurrency(tt.j)
		if got != tt.want {
			t.Errorf("jurisdictionBaseCurrency(%s) = %s, want %s", tt.j, got, tt.want)
		}
	}
}

func TestMapDomainPaymentStatus(t *testing.T) {
	now := time.Now()
	due := now.AddDate(0, 0, -5)
	grace := now.AddDate(0, 0, 25)

	if s := mapDomainPaymentStatus("paid", now, due, grace); s != AnnuityStatusPaid {
		t.Errorf("expected paid, got %s", s)
	}
	if s := mapDomainPaymentStatus("", now, due, grace); s != AnnuityStatusGrace {
		t.Errorf("expected grace, got %s", s)
	}

	pastGrace := now.AddDate(0, 0, -1)
	if s := mapDomainPaymentStatus("", now, due, pastGrace); s != AnnuityStatusOverdue {
		t.Errorf("expected overdue, got %s", s)
	}

	futureDue := now.AddDate(0, 1, 0)
	futureGrace := now.AddDate(0, 4, 0)
	if s := mapDomainPaymentStatus("", now, futureDue, futureGrace); s != AnnuityStatusPending {
		t.Errorf("expected pending, got %s", s)
	}
}

func TestBuildGroupKey(t *testing.T) {
	d := time.Date(2024, 7, 15, 0, 0, 0, 0, time.UTC)
	if k := buildGroupKey(BudgetGroupByYear, d, "CN", "p1"); k != "2024" {
		t.Errorf("year key: got %s", k)
	}
	if k := buildGroupKey(BudgetGroupByQuarter, d, "CN", "p1"); k != "2024-Q3" {
		t.Errorf("quarter key: got %s", k)
	}
	if k := buildGroupKey(BudgetGroupByJurisdiction, d, "CN", "p1"); k != "CN" {
		t.Errorf("jurisdiction key: got %s", k)
	}
	if k := buildGroupKey(BudgetGroupByPatent, d, "CN", "p1"); k != "p1" {
		t.Errorf("patent key: got %s", k)
	}
}

func TestClassifyAbandonmentRisk(t *testing.T) {
	if r := classifyAbandonmentRisk(10, 40); r != "low" {
		t.Errorf("expected low, got %s", r)
	}
	if r := classifyAbandonmentRisk(20, 40); r != "medium" {
		t.Errorf("expected medium, got %s", r)
	}
	if r := classifyAbandonmentRisk(35, 40); r != "high" {
		t.Errorf("expected high, got %s", r)
	}
}

func TestEstimateRemainingLife(t *testing.T) {
	filing := time.Now().AddDate(-10, 0, 0)
	rem := estimateRemainingLife(filing, domainLifecycle.JurisdictionCN)
	if rem < 9 || rem > 11 {
		t.Errorf("expected ~10 years remaining, got %d", rem)
	}

	expired := time.Now().AddDate(-25, 0, 0)
	rem = estimateRemainingLife(expired, domainLifecycle.JurisdictionCN)
	if rem != 0 {
		t.Errorf("expected 0 for expired, got %d", rem)
	}

	rem = estimateRemainingLife(time.Time{}, domainLifecycle.JurisdictionUS)
	if rem != 20 {
		t.Errorf("expected 20 for zero filing date, got %d", rem)
	}
}

//Personal.AI order the ending

