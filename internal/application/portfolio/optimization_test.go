// Phase 10 - File 227 of 349
package portfolio

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

// -----------------------------------------------------------------------
// Tests: OptimizationService Construction
// -----------------------------------------------------------------------

func TestNewOptimizationService_Success(t *testing.T) {
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("p1")},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, err := NewOptimizationService(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewOptimizationService_MissingDeps(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*OptimizationServiceConfig)
	}{
		{"nil PortfolioService", func(c *OptimizationServiceConfig) { c.PortfolioService = nil }},
		{"nil PatentRepository", func(c *OptimizationServiceConfig) { c.PatentRepository = nil }},
		{"nil Logger", func(c *OptimizationServiceConfig) { c.Logger = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := OptimizationServiceConfig{
				PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("p1")},
				PatentRepository: newMockPatentRepo(),
				Logger:           &mockLogger{},
			}
			tt.mod(&cfg)
			_, err := NewOptimizationService(cfg)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// -----------------------------------------------------------------------
// Tests: Optimize
// -----------------------------------------------------------------------

func buildOptTestRepo() *mockPatentRepo {
	repo := newMockPatentRepo()
	now := time.Now()
	patentPtrs := []*domainpatent.Patent{
		createTestPatentWithMolecules("o1", "US001", "A61K", "Own", now.AddDate(-2, 0, 0), 9.0, []string{"m1"}),
		createTestPatentWithMolecules("o2", "US002", "A61K", "Own", now.AddDate(-15, 0, 0), 2.0, []string{"m2"}),
		createTestPatentWithMolecules("o3", "EP003", "A61K", "Own", now.AddDate(-10, 0, 0), 3.0, []string{"m3"}),
		createTestPatentWithMolecules("o4", "US004", "C07D", "Own", now.AddDate(-1, 0, 0), 8.5, []string{"m4"}),
		createTestPatentWithMolecules("o5", "CN005", "G16B", "Own", now.AddDate(-3, 0, 0), 7.0, []string{"m5"}),
	}
	repo.byPortfolio["opt-port"] = patentPtrs
	return repo
}

func TestOptimize_Success(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	resp, err := svc.Optimize(context.Background(), &OptimizationRequest{
		PortfolioID: "opt-port",
		Objective:   GoalBalanced,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != "opt-port" {
		t.Errorf("expected opt-port, got %s", resp.PortfolioID)
	}
	if resp.Summary.TotalPatents != 5 {
		t.Errorf("expected 5 total patents, got %d", resp.Summary.TotalPatents)
	}
	if resp.Summary.RetainCount+resp.Summary.PruneCount != 5 {
		t.Error("retain + prune should equal total")
	}
	if resp.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
}

func TestOptimize_NilRequest(t *testing.T) {
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("p1")},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)
	_, err := svc.Optimize(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOptimize_EmptyPortfolioID(t *testing.T) {
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("p1")},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)
	_, err := svc.Optimize(context.Background(), &OptimizationRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOptimize_DefaultObjective(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	resp, err := svc.Optimize(context.Background(), &OptimizationRequest{
		PortfolioID: "opt-port",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Objective != GoalBalanced {
		t.Errorf("expected balanced default, got %s", resp.Objective)
	}
}

func TestOptimize_MinCostObjective(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	resp, err := svc.Optimize(context.Background(), &OptimizationRequest{
		PortfolioID: "opt-port",
		Objective:   GoalMinCost,
		Constraints: OptConstraints{
			MinPatentCount:  4,
			RequiredDomains: []string{"A61K", "C07D"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Summary.RetainCount < 4 {
		t.Errorf("expected at least 4 retained with MinPatentCount=4, got %d", resp.Summary.RetainCount)
	}

	// Verify required domains are still covered.
	retainedDomains := make(map[string]bool)
	for _, rec := range resp.Recommendations {
		if rec.Action == "retain" {
			retainedDomains[rec.TechDomain] = true
		}
	}
	if !retainedDomains["A61K"] {
		t.Error("required domain A61K was not retained")
	}
	if !retainedDomains["C07D"] {
		t.Error("required domain C07D was not retained")
	}
}

func TestOptimize_MaxROIObjective(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	resp, err := svc.Optimize(context.Background(), &OptimizationRequest{
		PortfolioID: "opt-port",
		Objective:   GoalMaxROI,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Summary.TotalPatents != 5 {
		t.Errorf("expected 5 total, got %d", resp.Summary.TotalPatents)
	}
}

func TestOptimize_WithPreferences(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	resp, err := svc.Optimize(context.Background(), &OptimizationRequest{
		PortfolioID: "opt-port",
		Objective:   GoalBalanced,
		Preferences: OptPreferences{
			PreferRecent:    true,
			PreferHighValue: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Old, low-value patents should be more likely pruned.
	for _, pc := range resp.PruneList {
		if pc.PatentID == "o1" {
			t.Error("high-value recent patent o1 should not be pruned with these preferences")
		}
	}
}

func TestOptimize_PortfolioNotFound(t *testing.T) {
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: nil},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	_, err := svc.Optimize(context.Background(), &OptimizationRequest{
		PortfolioID: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestOptimize_EmptyPortfolio(t *testing.T) {
	repo := newMockPatentRepo()
	repo.byPortfolio["empty-port"] = []*domainpatent.Patent{}
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("empty-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	resp, err := svc.Optimize(context.Background(), &OptimizationRequest{
		PortfolioID: "empty-port",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Summary.TotalPatents != 0 {
		t.Errorf("expected 0 patents, got %d", resp.Summary.TotalPatents)
	}
	if len(resp.PruneList) != 0 {
		t.Error("expected no prune candidates for empty portfolio")
	}
}

// -----------------------------------------------------------------------
// Tests: GetPruneCandidates
// -----------------------------------------------------------------------

func TestGetPruneCandidates_Success(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	candidates, err := svc.GetPruneCandidates(context.Background(), "opt-port", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) > 3 {
		t.Errorf("expected at most 3 candidates, got %d", len(candidates))
	}
}

func TestGetPruneCandidates_EmptyPortfolioID(t *testing.T) {
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("p1")},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	_, err := svc.GetPruneCandidates(context.Background(), "", 5)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetPruneCandidates_DefaultLimit(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	candidates, err := svc.GetPruneCandidates(context.Background(), "opt-port", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) > 10 {
		t.Errorf("expected at most 10 with default limit, got %d", len(candidates))
	}
}

// -----------------------------------------------------------------------
// Tests: EstimateCost
// -----------------------------------------------------------------------

func TestEstimateCost_Success(t *testing.T) {
	repo := buildOptTestRepo()
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("opt-port")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	estimate, err := svc.EstimateCost(context.Background(), "opt-port")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if estimate == nil {
		t.Fatal("expected non-nil estimate")
	}
	if estimate.PortfolioID != "opt-port" {
		t.Errorf("expected opt-port, got %s", estimate.PortfolioID)
	}
	if estimate.TotalAnnualCost <= 0 {
		t.Error("expected positive total annual cost")
	}
	if len(estimate.ByDomain) == 0 {
		t.Error("expected non-empty domain breakdown")
	}
	if len(estimate.ByJurisdiction) == 0 {
		t.Error("expected non-empty jurisdiction breakdown")
	}
	if len(estimate.TopCostPatents) == 0 {
		t.Error("expected non-empty top cost patents")
	}
	if estimate.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}

	// Verify sorted by cost descending.
	for i := 1; i < len(estimate.TopCostPatents); i++ {
		if estimate.TopCostPatents[i].AnnualCost > estimate.TopCostPatents[i-1].AnnualCost {
			t.Error("top cost patents not sorted descending")
			break
		}
	}
}

func TestEstimateCost_EmptyPortfolioID(t *testing.T) {
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("p1")},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	_, err := svc.EstimateCost(context.Background(), "")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEstimateCost_PortfolioNotFound(t *testing.T) {
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: nil},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	_, err := svc.EstimateCost(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestEstimateCost_EmptyPortfolio(t *testing.T) {
	repo := newMockPatentRepo()
	repo.byPortfolio["empty"] = []*domainpatent.Patent{}
	cfg := OptimizationServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("empty")},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewOptimizationService(cfg)

	estimate, err := svc.EstimateCost(context.Background(), "empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if estimate.TotalAnnualCost != 0 {
		t.Errorf("expected 0 cost for empty portfolio, got %f", estimate.TotalAnnualCost)
	}
}

// -----------------------------------------------------------------------
// Tests: estimatePatentAnnualCost helper
// -----------------------------------------------------------------------

func TestEstimatePatentAnnualCost_USPatent(t *testing.T) {
	p := createTestPatentSimple("US1234", time.Now().AddDate(-2, 0, 0))
	cost := estimatePatentAnnualCost(*p) // Dereference pointer to value
	if cost <= 0 {
		t.Error("expected positive cost")
	}
	// US base cost is 2000 * 1.0, age < 4 so ageFactor = 1.0.
	if cost != 2000.0 {
		t.Errorf("expected 2000 for young US patent, got %f", cost)
	}
}

func TestEstimatePatentAnnualCost_EPPatent(t *testing.T) {
	p := createTestPatentSimple("EP5678", time.Now().AddDate(-2, 0, 0))
	cost := estimatePatentAnnualCost(*p) // Dereference pointer to value
	expected := 2000.0 * 1.8
	if cost != expected {
		t.Errorf("expected %f for young EP patent, got %f", expected, cost)
	}
}

func TestEstimatePatentAnnualCost_OldPatent(t *testing.T) {
	p := createTestPatentSimple("US9999", time.Now().AddDate(-14, 0, 0))
	cost := estimatePatentAnnualCost(*p) // Dereference pointer to value
	// Age ~14 years, ageFactor = 1.0 + (14-4)*0.05 = 1.5
	expected := 2000.0 * 1.0 * 1.5
	tolerance := 100.0
	if cost < expected-tolerance || cost > expected+tolerance {
		t.Errorf("expected ~%f for 14-year US patent, got %f", expected, cost)
	}
}

func TestEstimatePatentAnnualCost_MaxAgeFactor(t *testing.T) {
	p := createTestPatentSimple("US0001", time.Now().AddDate(-50, 0, 0))
	cost := estimatePatentAnnualCost(*p) // Dereference pointer to value
	// ageFactor capped at 3.0.
	expected := 2000.0 * 1.0 * 3.0
	if cost != expected {
		t.Errorf("expected %f with capped age factor, got %f", expected, cost)
	}
}

func TestEstimatePatentAnnualCost_UnknownJurisdiction(t *testing.T) {
	p := createTestPatentSimple("ZZ1234", time.Now().AddDate(-1, 0, 0))
	cost := estimatePatentAnnualCost(*p) // Dereference pointer to value
	// Unknown jurisdiction uses multiplier 1.0, age < 4 so ageFactor = 1.0.
	if cost != 2000.0 {
		t.Errorf("expected 2000 for unknown jurisdiction, got %f", cost)
	}
}

// -----------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------
// Note: createTestPortfolio and createTestPatent are defined in gap_analysis_test.go

// createTestPatentSimple creates a simple test patent with minimal fields
func createTestPatentSimple(number string, filingDate time.Time) *domainpatent.Patent {
	now := time.Now()
	expiryDate := filingDate.AddDate(20, 0, 0)
	return &domainpatent.Patent{
		ID:              uuid.New(),
		PatentNumber:    number,
		Title:           "Test Patent",
		FilingDate:      &filingDate,
		GrantDate:       &now,
		ExpiryDate:      &expiryDate,
		Status:          domainpatent.PatentStatusGranted,
		Office:          domainpatent.OfficeUSPTO,
		CreatedAt:       now,
		UpdatedAt:       now,
		Version:         1,
	}
}

//Personal.AI order the ending
