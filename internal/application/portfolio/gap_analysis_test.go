// Phase 10 - File 225 of 349
package portfolio

import (
	"context"
	"fmt"
	"testing"
	"time"

	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

// -----------------------------------------------------------------------
// Tests: GapAnalysisService Construction
// -----------------------------------------------------------------------

func TestNewGapAnalysisService_Success(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1", name: "Test"}},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, err := NewGapAnalysisService(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewGapAnalysisService_MissingDeps(t *testing.T) {
	tests := []struct {
		name     string
		modifier func(*GapAnalysisServiceConfig)
	}{
		{"nil PortfolioService", func(c *GapAnalysisServiceConfig) { c.PortfolioService = nil }},
		{"nil PatentRepository", func(c *GapAnalysisServiceConfig) { c.PatentRepository = nil }},
		{"nil Logger", func(c *GapAnalysisServiceConfig) { c.Logger = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GapAnalysisServiceConfig{
				PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1"}},
				PatentRepository: newMockPatentRepo(),
				Logger:           &mockLogger{},
			}
			tt.modifier(&cfg)
			svc, err := NewGapAnalysisService(cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if svc != nil {
				t.Fatal("expected nil service")
			}
		})
	}
}

func TestNewGapAnalysisService_DefaultTTL(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1"}},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
		CacheTTL:         0,
	}
	svc, err := NewGapAnalysisService(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	impl := svc.(*gapAnalysisServiceImpl)
	if impl.cacheTTL != 15*time.Minute {
		t.Errorf("expected 15m default TTL, got %v", impl.cacheTTL)
	}
}

// -----------------------------------------------------------------------
// Tests: AnalyzeGaps
// -----------------------------------------------------------------------

func buildGapTestPatentRepo() *mockPatentRepo {
	repo := newMockPatentRepo()
	ownPatents := []domainpatent.Patent{
		&mockPatent{
			id: "p1", number: "US1001", techDomain: "A61K", legalStatus: "granted",
			assignee: "OwnCorp", filingDate: time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 8.0, moleculeIDs: []string{"m1"},
		},
		&mockPatent{
			id: "p2", number: "US1002", techDomain: "A61K", legalStatus: "granted",
			assignee: "OwnCorp", filingDate: time.Date(2018, 6, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 7.5, moleculeIDs: []string{"m2"},
		},
		&mockPatent{
			id: "p3", number: "US1003", techDomain: "C07D", legalStatus: "granted",
			assignee: "OwnCorp", filingDate: time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 9.0, moleculeIDs: []string{"m3"},
		},
	}
	repo.byPortfolio["portfolio-gap"] = ownPatents

	compPatents := []domainpatent.Patent{
		&mockPatent{
			id: "c1", number: "EP2001", techDomain: "A61K", legalStatus: "granted",
			assignee: "RivalCo", filingDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 7.0,
		},
		&mockPatent{
			id: "c2", number: "EP2002", techDomain: "G16B", legalStatus: "granted",
			assignee: "RivalCo", filingDate: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 8.5,
		},
		&mockPatent{
			id: "c3", number: "CN3001", techDomain: "G16B", legalStatus: "granted",
			assignee: "RivalCo", filingDate: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 6.0,
		},
		&mockPatent{
			id: "c4", number: "JP4001", techDomain: "C12N", legalStatus: "granted",
			assignee: "RivalCo", filingDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			valueScore: 9.0,
		},
	}
	repo.byAssignee["RivalCo"] = compPatents

	return repo
}

func TestAnalyzeGaps_Success(t *testing.T) {
	repo := buildGapTestPatentRepo()
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "portfolio-gap", name: "Gap Test"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	resp, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID:      "portfolio-gap",
		CompetitorNames:  []string{"RivalCo"},
		ExpirationWindow: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != "portfolio-gap" {
		t.Errorf("expected portfolio-gap, got %s", resp.PortfolioID)
	}

	// Should detect G16B and C12N as tech gaps (competitor has them, we don't).
	foundG16B := false
	foundC12N := false
	for _, g := range resp.TechGaps {
		if g.TechDomain == "G16B" {
			foundG16B = true
		}
		if g.TechDomain == "C12N" {
			foundC12N = true
		}
	}
	if !foundG16B {
		t.Error("expected G16B technology gap")
	}
	if !foundC12N {
		t.Error("expected C12N technology gap")
	}

	if resp.OverallScore < 0 || resp.OverallScore > 100 {
		t.Errorf("health score out of range: %f", resp.OverallScore)
	}
	if resp.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
}

func TestAnalyzeGaps_NilRequest(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1"}},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	_, err := svc.AnalyzeGaps(context.Background(), nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAnalyzeGaps_EmptyPortfolioID(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1"}},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	_, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{})
	if err == nil {
		t.Fatal("expected validation error for empty portfolio_id")
	}
}

func TestAnalyzeGaps_PortfolioNotFound(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: nil},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	_, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{PortfolioID: "nonexistent"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestAnalyzeGaps_NoCompetitors(t *testing.T) {
	repo := buildGapTestPatentRepo()
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "portfolio-gap"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	resp, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID: "portfolio-gap",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.TechGaps) != 0 {
		t.Errorf("expected 0 tech gaps without competitors, got %d", len(resp.TechGaps))
	}
}

// -----------------------------------------------------------------------
// Tests: GetExpirationRisks
// -----------------------------------------------------------------------

func TestGetExpirationRisks_Success(t *testing.T) {
	repo := newMockPatentRepo()
	// Patent filed 16 years ago — expires in 4 years.
	repo.byPortfolio["port-exp"] = []domainpatent.Patent{
		&mockPatent{
			id: "exp1", number: "US9999", techDomain: "A61K",
			filingDate: time.Now().AddDate(-16, 0, 0), valueScore: 8.0,
		},
		&mockPatent{
			id: "exp2", number: "US8888", techDomain: "C07D",
			filingDate: time.Now().AddDate(-5, 0, 0), valueScore: 7.0,
		},
	}

	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "port-exp"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	risks, err := svc.GetExpirationRisks(context.Background(), "port-exp", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only exp1 should be at risk (expires in ~4 years, within 5-year window).
	if len(risks) != 1 {
		t.Fatalf("expected 1 risk, got %d", len(risks))
	}
	if risks[0].PatentID != "exp1" {
		t.Errorf("expected exp1, got %s", risks[0].PatentID)
	}
	if risks[0].DaysRemaining <= 0 {
		t.Error("expected positive days remaining")
	}
}

func TestGetExpirationRisks_EmptyPortfolioID(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1"}},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	_, err := svc.GetExpirationRisks(context.Background(), "", 5)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetExpirationRisks_DefaultWindow(t *testing.T) {
	repo := newMockPatentRepo()
	repo.byPortfolio["port-def"] = []domainpatent.Patent{}
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "port-def"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	risks, err := svc.GetExpirationRisks(context.Background(), "port-def", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risks == nil {
		t.Fatal("expected non-nil result")
	}
}

// -----------------------------------------------------------------------
// Tests: GetGeographicGaps
// -----------------------------------------------------------------------

func TestGetGeographicGaps_Success(t *testing.T) {
	repo := newMockPatentRepo()
	repo.byPortfolio["port-geo"] = []domainpatent.Patent{
		&mockPatent{id: "g1", number: "US1111", techDomain: "A61K"},
		&mockPatent{id: "g2", number: "EP2222", techDomain: "A61K"},
	}
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "port-geo"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	gaps, err := svc.GetGeographicGaps(context.Background(), "port-geo", []string{"US", "EP", "CN", "JP"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// US and EP are covered, CN and JP should be gaps.
	for _, g := range gaps {
		if g.Jurisdiction == "US" || g.Jurisdiction == "EP" {
			t.Errorf("jurisdiction %s should not be a gap", g.Jurisdiction)
		}
	}

	foundCN := false
	foundJP := false
	for _, g := range gaps {
		if g.Jurisdiction == "CN" {
			foundCN = true
		}
		if g.Jurisdiction == "JP" {
			foundJP = true
		}
	}
	if !foundCN {
		t.Error("expected CN geographic gap")
	}
	if !foundJP {
		t.Error("expected JP geographic gap")
	}
}

func TestGetGeographicGaps_EmptyPortfolioID(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1"}},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	_, err := svc.GetGeographicGaps(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetGeographicGaps_DefaultJurisdictions(t *testing.T) {
	repo := newMockPatentRepo()
	repo.byPortfolio["port-def-geo"] = []domainpatent.Patent{
		&mockPatent{id: "dg1", number: "US5555", techDomain: "A61K"},
	}
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "port-def-geo"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	gaps, err := svc.GetGeographicGaps(context.Background(), "port-def-geo", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should use default jurisdictions and exclude US.
	for _, g := range gaps {
		if g.Jurisdiction == "US" {
			t.Error("US should not be a gap")
		}
	}
	if len(gaps) == 0 {
		t.Error("expected at least some geographic gaps with default jurisdictions")
	}
}

// -----------------------------------------------------------------------
// Tests: GetFilingOpportunities
// -----------------------------------------------------------------------

func TestGetFilingOpportunities_Success(t *testing.T) {
	repo := buildGapTestPatentRepo()
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "portfolio-gap"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	opps, err := svc.GetFilingOpportunities(context.Background(), "portfolio-gap", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opps) > 5 {
		t.Errorf("expected at most 5 opportunities, got %d", len(opps))
	}
	// Verify sorted by overall score descending.
	for i := 1; i < len(opps); i++ {
		if opps[i].OverallScore > opps[i-1].OverallScore {
			t.Error("opportunities not sorted by overall score descending")
			break
		}
	}
}

func TestGetFilingOpportunities_EmptyPortfolioID(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "p1"}},
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	_, err := svc.GetFilingOpportunities(context.Background(), "", 10)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestGetFilingOpportunities_DefaultLimit(t *testing.T) {
	repo := buildGapTestPatentRepo()
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: &mockPortfolio{id: "portfolio-gap"}},
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	opps, err := svc.GetFilingOpportunities(context.Background(), "portfolio-gap", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opps) > 10 {
		t.Errorf("expected at most 10 with default limit, got %d", len(opps))
	}
}

// -----------------------------------------------------------------------
// Tests: Helper Functions
// -----------------------------------------------------------------------

func TestExtractJurisdiction(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"US1234567", "US"},
		{"EP1234567", "EP"},
		{"CN1234567", "CN"},
		{"JP1234567", "JP"},
		{"WO2023001234", "WO"},
		{"1234567", ""},
		{"", ""},
		{"A", "A"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractJurisdiction(tt.input)
			if result != tt.expected {
				t.Errorf("extractJurisdiction(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestJurisdictionMarketScore(t *testing.T) {
	usScore := jurisdictionMarketScore("US")
	if usScore != 1.0 {
		t.Errorf("expected US score 1.0, got %f", usScore)
	}
	unknownScore := jurisdictionMarketScore("ZZ")
	if unknownScore != 0.3 {
		t.Errorf("expected unknown score 0.3, got %f", unknownScore)
	}
}

func TestJurisdictionName(t *testing.T) {
	if jurisdictionName("US") != "United States" {
		t.Error("expected United States for US")
	}
	if jurisdictionName("ZZ") != "ZZ" {
		t.Error("expected ZZ passthrough for unknown")
	}
}

func TestDefaultTargetJurisdictions(t *testing.T) {
	targets := defaultTargetJurisdictions()
	if len(targets) == 0 {
		t.Fatal("expected non-empty default jurisdictions")
	}
	found := false
	for _, j := range targets {
		if j == "US" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected US in default jurisdictions")
	}
}

// -----------------------------------------------------------------------
// Tests: identifyExpirationRisks risk levels
// -----------------------------------------------------------------------

func TestExpirationRiskLevels(t *testing.T) {
	svc := &gapAnalysisServiceImpl{logger: &mockLogger{}}

	now := time.Now()
	patents := []domainpatent.Patent{
		// Expires in ~6 months -> critical
		&mockPatent{id: "r1", number: "US-R1", techDomain: "A61K",
			filingDate: now.AddDate(-19, -6, 0)},
		// Expires in ~1.5 years -> high
		&mockPatent{id: "r2", number: "US-R2", techDomain: "A61K",
			filingDate: now.AddDate(-18, -6, 0)},
		// Expires in ~2.5 years -> medium
		&mockPatent{id: "r3", number: "US-R3", techDomain: "C07D",
			filingDate: now.AddDate(-17, -6, 0)},
		// Expires in ~4 years -> low
		&mockPatent{id: "r4", number: "US-R4", techDomain: "C07D",
			filingDate: now.AddDate(-16, 0, 0)},
	}

	risks := svc.identifyExpirationRisks(patents, 5)

	expectedLevels := map[string]RiskLevel{
		"r1": RiskCritical,
		"r2": RiskHigh,
		"r3": RiskMedium,
		"r4": RiskLow,
	}

	for _, r := range risks {
		expected, ok := expectedLevels[r.PatentID]
		if !ok {
			continue
		}
		if r.RiskLevel != expected {
			t.Errorf("patent %s: expected risk %s, got %s (days remaining: %d)",
				r.PatentID, expected, r.RiskLevel, r.DaysRemaining)
		}
	}
}

// -----------------------------------------------------------------------
// Tests: computeHealthScore
// -----------------------------------------------------------------------

func TestComputeHealthScore_EmptyPortfolio(t *testing.T) {
	svc := &gapAnalysisServiceImpl{logger: &mockLogger{}}
	score := svc.computeHealthScore(nil, nil, nil, nil)
	if score != 0.0 {
		t.Errorf("expected 0 for empty portfolio, got %f", score)
	}
}

func TestComputeHealthScore_HealthyPortfolio(t *testing.T) {
	svc := &gapAnalysisServiceImpl{logger: &mockLogger{}}
	patents := make([]domainpatent.Patent, 10)
	for i := range patents {
		patents[i] = &mockPatent{id: fmt.Sprintf("h%d", i)}
	}
	score := svc.computeHealthScore(patents, nil, nil, nil)
	if score <= 0 || score > 100 {
		t.Errorf("expected positive score <= 100, got %f", score)
	}
}

func TestComputeHealthScore_WithGaps(t *testing.T) {
	svc := &gapAnalysisServiceImpl{logger: &mockLogger{}}
	patents := make([]domainpatent.Patent, 5)
	for i := range patents {
		patents[i] = &mockPatent{id: fmt.Sprintf("g%d", i)}
	}

	gaps := []TechnologyGap{
		{GapSeverity: 0.9},
		{GapSeverity: 0.7},
	}
	risks := []ExpirationRisk{
		{RiskLevel: RiskCritical, HasReplacement: false},
	}

	scoreWithGaps := svc.computeHealthScore(patents, gaps, risks, nil)
	scoreWithout := svc.computeHealthScore(patents, nil, nil, nil)

	if scoreWithGaps >= scoreWithout {
		t.Errorf("score with gaps (%f) should be less than without (%f)", scoreWithGaps, scoreWithout)
	}
}

//Personal.AI order the ending
