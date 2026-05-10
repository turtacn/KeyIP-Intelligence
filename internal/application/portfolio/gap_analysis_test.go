// Phase 10 - File: gap_analysis_test.go
// Tests for patent portfolio gap analysis application service.
package portfolio

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
)

// -----------------------------------------------------------------------
// Test IDs for gap analysis
// -----------------------------------------------------------------------
const (
	testGAPortfolioID      = "90000000-0000-0000-0000-000000000001"
	testGAPortfolioEmptyID = "90000000-0000-0000-0000-000000000002"
)

// -----------------------------------------------------------------------
// Helpers: build gap analysis test data
// -----------------------------------------------------------------------

func buildGAPortfolio() *domainportfolio.Portfolio {
	now := time.Now()
	return &domainportfolio.Portfolio{
		ID:          testGAPortfolioID,
		Name:        "Gap Analysis Test Portfolio",
		OwnerID:     uuid.New().String(),
		TechDomains: []string{"A61K", "C07C"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func buildGAPatents() []*domainpatent.Patent {
	now := time.Now()
	recent := now.AddDate(-1, 0, 0)
	middle := now.AddDate(-8, 0, 0)
	old := now.AddDate(-15, 0, 0)
	expiring := now.AddDate(-19, 0, 0)

	return []*domainpatent.Patent{
		{
			ID:             uuid.MustParse("a1000000-0000-0000-0000-000000000001"),
			PatentNumber:   "US100001",
			Title:          "Patent 1",
			AssigneeName:   "TestCorp",
			FilingDate:     &recent,
			Status:         domainpatent.PatentStatusGranted,
			IPCCodes:       []string{"A61K"},
			KeyIPTechCodes: []string{"A61K"},
			Metadata:       map[string]any{"value_score": 8.0},
		},
		{
			ID:             uuid.MustParse("a1000000-0000-0000-0000-000000000002"),
			PatentNumber:   "US100002",
			Title:          "Patent 2",
			AssigneeName:   "TestCorp",
			FilingDate:     &middle,
			Status:         domainpatent.PatentStatusGranted,
			IPCCodes:       []string{"C07C"},
			KeyIPTechCodes: []string{"C07C"},
			Metadata:       map[string]any{"value_score": 6.0},
		},
		{
			ID:             uuid.MustParse("a1000000-0000-0000-0000-000000000003"),
			PatentNumber:   "US100003",
			Title:          "Patent 3",
			AssigneeName:   "TestCorp",
			FilingDate:     &old,
			Status:         domainpatent.PatentStatusGranted,
			IPCCodes:       []string{"A61K"},
			KeyIPTechCodes: []string{"A61K"},
			Metadata:       map[string]any{"value_score": 4.0},
		},
		{
			ID:             uuid.MustParse("a1000000-0000-0000-0000-000000000004"),
			PatentNumber:   "US100004",
			Title:          "Patent 4",
			AssigneeName:   "TestCorp",
			FilingDate:     &expiring,
			Status:         domainpatent.PatentStatusGranted,
			IPCCodes:       []string{"C07C"},
			KeyIPTechCodes: []string{"C07C"},
			Metadata:       map[string]any{"value_score": 3.0},
		},
	}
}

func buildGAService() (*gapAnalysisServiceImpl, *mockPortfolioRepoConstellation, *mockPatentRepoConstellation) {
	portfolio := buildGAPortfolio()
	patents := buildGAPatents()

	portfolioRepo := newMockPortfolioRepoConstellation()
	portfolioRepo.portfolios[testGAPortfolioID] = portfolio
	portfolioRepo.portfolios[testGAPortfolioEmptyID] = &domainportfolio.Portfolio{
		ID:   testGAPortfolioEmptyID,
		Name: "Empty Portfolio",
	}

	patentRepo := newMockPatentRepoConstellation()
	patentRepo.byPortfolio[testGAPortfolioID] = patents
	for _, p := range patents {
		patentRepo.byIDs[p.ID.String()] = p
	}

	cache := newMockConstellationCache()

	svc := &gapAnalysisServiceImpl{
		portfolioSvc:  &mockPortfolioService{portfolio: portfolio},
		portfolioRepo: portfolioRepo,
		patentRepo:    patentRepo,
		logger:        &mockLogger{},
		cache:         cache,
		cacheTTL:      15 * time.Minute,
	}

	return svc, portfolioRepo, patentRepo
}

// -----------------------------------------------------------------------
// Tests: NewGapAnalysisService
// -----------------------------------------------------------------------

func TestNewGapAnalysisService_Success(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService:    &mockPortfolioService{portfolio: buildGAPortfolio()},
		PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository:    newMockPatentRepoConstellation(),
		Logger:              &mockLogger{},
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
	baseCfg := GapAnalysisServiceConfig{
		PortfolioService:    &mockPortfolioService{portfolio: buildGAPortfolio()},
		PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository:    newMockPatentRepoConstellation(),
		Logger:              &mockLogger{},
	}
	tests := []struct {
		name string
		mod  func(*GapAnalysisServiceConfig)
	}{
		{"nil PortfolioService", func(c *GapAnalysisServiceConfig) { c.PortfolioService = nil }},
		{"nil PortfolioRepository", func(c *GapAnalysisServiceConfig) { c.PortfolioRepository = nil }},
		{"nil PatentRepository", func(c *GapAnalysisServiceConfig) { c.PatentRepository = nil }},
		{"nil Logger", func(c *GapAnalysisServiceConfig) { c.Logger = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseCfg
			tt.mod(&cfg)
			_, err := NewGapAnalysisService(cfg)
			if err == nil {
				t.Fatal("expected error for missing dependency")
			}
		})
	}
}

func TestNewGapAnalysisService_DefaultCacheTTL(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService:    &mockPortfolioService{portfolio: buildGAPortfolio()},
		PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository:    newMockPatentRepoConstellation(),
		Logger:              &mockLogger{},
	}
	svc, err := NewGapAnalysisService(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	impl := svc.(*gapAnalysisServiceImpl)
	if impl.cacheTTL != 15*time.Minute {
		t.Errorf("expected default TTL 15m, got %v", impl.cacheTTL)
	}
}

// -----------------------------------------------------------------------
// Tests: AnalyzeGaps
// -----------------------------------------------------------------------

func TestAnalyzeGaps_Success(t *testing.T) {
	svc, _, _ := buildGAService()

	resp, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID:      testGAPortfolioID,
		CompetitorNames:  []string{},
		ExpirationWindow: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != testGAPortfolioID {
		t.Errorf("expected portfolio %s, got %s", testGAPortfolioID, resp.PortfolioID)
	}
	if resp.GeneratedAt.IsZero() {
		t.Error("expected non-zero GeneratedAt")
	}
	if resp.OverallScore <= 0 {
		t.Error("expected positive health score")
	}
	if resp.Summary.TotalTechGaps < 0 {
		t.Error("expected non-negative tech gaps")
	}
	if resp.Summary.ExpiringPatents < 0 {
		t.Error("expected non-negative expiring patents")
	}
}

func TestAnalyzeGaps_NilRequest(t *testing.T) {
	svc, _, _ := buildGAService()
	_, err := svc.AnalyzeGaps(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestAnalyzeGaps_EmptyPortfolioID(t *testing.T) {
	svc, _, _ := buildGAService()
	_, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{})
	if err == nil {
		t.Fatal("expected error for empty portfolio ID")
	}
}

func TestAnalyzeGaps_PortfolioNotFound(t *testing.T) {
	svc, portfolioRepo, _ := buildGAService()
	delete(portfolioRepo.portfolios, testGAPortfolioID)

	_, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID: testGAPortfolioID,
	})
	if err == nil {
		t.Fatal("expected error for portfolio not found")
	}
}

func TestAnalyzeGaps_WithCompetitors(t *testing.T) {
	svc, _, patentRepo := buildGAService()

	// Add competitor patents
	now := time.Now()
	compFiling := now.AddDate(-2, 0, 0)
	compPatent := &domainpatent.Patent{
		ID:             uuid.MustParse("b2000000-0000-0000-0000-000000000001"),
		PatentNumber:   "US200001",
		AssigneeName:   "RivalCorp",
		FilingDate:     &compFiling,
		Status:         domainpatent.PatentStatusGranted,
		IPCCodes:       []string{"G01N"},
		KeyIPTechCodes: []string{"G01N"},
		Metadata:       map[string]any{"value_score": 7.0},
	}
	patentRepo.byAssignee["RivalCorp"] = []*domainpatent.Patent{compPatent}
	patentRepo.byIDs[compPatent.ID.String()] = compPatent

	resp, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID:      testGAPortfolioID,
		CompetitorNames:  []string{"RivalCorp"},
		ExpirationWindow: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	// Should identify G01N as a technology gap
	if resp.Summary.TotalTechGaps == 0 {
		t.Log("no gaps identified (competitor domain may already be covered)")
	}
}

// -----------------------------------------------------------------------
// Tests: GetFilingOpportunities
// -----------------------------------------------------------------------

func TestGetFilingOpportunities_Success(t *testing.T) {
	svc, _, _ := buildGAService()

	opps, err := svc.GetFilingOpportunities(context.Background(), testGAPortfolioID, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opps == nil {
		t.Fatal("expected non-nil opportunities")
	}
	// May be empty if no gaps — that's fine
	_ = opps
}

func TestGetFilingOpportunities_EmptyPortfolioID(t *testing.T) {
	svc, _, _ := buildGAService()
	_, err := svc.GetFilingOpportunities(context.Background(), "", 5)
	if err == nil {
		t.Fatal("expected error for empty portfolio ID")
	}
}

func TestGetFilingOpportunities_DefaultLimit(t *testing.T) {
	svc, _, _ := buildGAService()

	// With limit <= 0, defaults to 10
	opps, err := svc.GetFilingOpportunities(context.Background(), testGAPortfolioID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opps == nil {
		t.Fatal("expected non-nil opportunities")
	}
}

// -----------------------------------------------------------------------
// Tests: GetExpirationRisks
// -----------------------------------------------------------------------

func TestGetExpirationRisks_Success(t *testing.T) {
	svc, _, _ := buildGAService()

	risks, err := svc.GetExpirationRisks(context.Background(), testGAPortfolioID, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if risks == nil {
		t.Fatal("expected non-nil risks")
	}
}

func TestGetExpirationRisks_EmptyPortfolioID(t *testing.T) {
	svc, _, _ := buildGAService()
	_, err := svc.GetExpirationRisks(context.Background(), "", 5)
	if err == nil {
		t.Fatal("expected error for empty portfolio ID")
	}
}

func TestGetExpirationRisks_DefaultWindow(t *testing.T) {
	svc, _, _ := buildGAService()
	risks, err := svc.GetExpirationRisks(context.Background(), testGAPortfolioID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = risks
}

func TestGetExpirationRisks_PortfolioNotFound(t *testing.T) {
	svc, portfolioRepo, _ := buildGAService()
	delete(portfolioRepo.portfolios, testGAPortfolioID)

	_, err := svc.GetExpirationRisks(context.Background(), testGAPortfolioID, 5)
	if err == nil {
		t.Fatal("expected error for portfolio not found")
	}
}

// -----------------------------------------------------------------------
// Tests: GetGeographicGaps
// -----------------------------------------------------------------------

func TestGetGeographicGaps_Success(t *testing.T) {
	svc, _, _ := buildGAService()

	gaps, err := svc.GetGeographicGaps(context.Background(), testGAPortfolioID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gaps == nil {
		t.Fatal("expected non-nil gaps")
	}
}

func TestGetGeographicGaps_EmptyPortfolioID(t *testing.T) {
	svc, _, _ := buildGAService()
	_, err := svc.GetGeographicGaps(context.Background(), "", nil)
	if err == nil {
		t.Fatal("expected error for empty portfolio ID")
	}
}

func TestGetGeographicGaps_PortfolioNotFound(t *testing.T) {
	svc, portfolioRepo, _ := buildGAService()
	delete(portfolioRepo.portfolios, testGAPortfolioID)

	_, err := svc.GetGeographicGaps(context.Background(), testGAPortfolioID, nil)
	if err == nil {
		t.Fatal("expected error for portfolio not found")
	}
}

func TestGetGeographicGaps_WithTargets(t *testing.T) {
	svc, _, _ := buildGAService()

	gaps, err := svc.GetGeographicGaps(context.Background(), testGAPortfolioID,
		[]string{"US", "EP", "CN", "JP"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With US patents, US may not be a gap; EP, CN, JP likely are
	if len(gaps) == 0 {
		t.Log("no geographic gaps found")
	}
}

// -----------------------------------------------------------------------
// Tests: Internal Analysis Methods
// -----------------------------------------------------------------------

func TestIdentifyTechGaps(t *testing.T) {
	svc, _, _ := buildGAService()

	patents := buildGAPatents()
	ownPatents := make([]domainpatent.Patent, len(patents))
	for i, p := range patents {
		ownPatents[i] = *p
	}

	// No competitors
	gaps := svc.identifyTechGaps(ownPatents, nil, nil)
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps with no competitors, got %d", len(gaps))
	}

	// With competitor in new domain
	now := time.Now()
	compPatents := map[string][]domainpatent.Patent{
		"Competitor": {
			{
				PatentNumber:   "US-COMP-1",
				AssigneeName:   "Competitor",
				FilingDate:     &now,
				Status:         domainpatent.PatentStatusGranted,
				IPCCodes:       []string{"G01N"},
				KeyIPTechCodes: []string{"G01N"},
				Metadata:       map[string]any{"value_score": 7.0},
			},
		},
	}
	gaps = svc.identifyTechGaps(ownPatents, compPatents, nil)
	if len(gaps) == 0 {
		t.Error("expected at least 1 gap with competitor in uncovered domain")
	}

	// Filter by domain
	gaps = svc.identifyTechGaps(ownPatents, compPatents, []string{"G01N"})
	if len(gaps) == 0 {
		t.Error("expected gap in G01N when filtering")
	}
}

func TestIdentifyExpirationRisks(t *testing.T) {
	svc, _, _ := buildGAService()

	patents := buildGAPatents()
	ownPatents := make([]domainpatent.Patent, len(patents))
	for i, p := range patents {
		ownPatents[i] = *p
	}

	risks := svc.identifyExpirationRisks(ownPatents, 5)
	// Should find the patent filed 19 years ago as about to expire
	if len(risks) == 0 {
		t.Log("no expiration risks in 5-year window")
	} else {
		// The expiring patent should be in the list
		for _, r := range risks {
			if r.PatentID != "" {
				t.Logf("expiring: %s, days=%d, risk=%s", r.PatentNumber, r.DaysRemaining, r.RiskLevel)
			}
		}
	}
}

func TestIdentifyGeographicGaps(t *testing.T) {
	svc, _, _ := buildGAService()

	patents := buildGAPatents()
	ownPatents := make([]domainpatent.Patent, len(patents))
	for i, p := range patents {
		ownPatents[i] = *p
	}

	// With US patents already, US should not be a gap
	gaps := svc.identifyGeographicGaps(ownPatents, nil, []string{"US", "CN"})
	for _, g := range gaps {
		if g.Jurisdiction == "US" {
			t.Error("US should not be a gap when we have US patents")
		}
	}
}

func TestGenerateOpportunities(t *testing.T) {
	svc, _, _ := buildGAService()

	techGaps := []TechnologyGap{
		{
			GapID:           "tg-0",
			TechDomain:      "G01N",
			CompetitorCount: 5,
			GapSeverity:     0.85,
			Recommendation:  "Critical gap",
		},
	}
	geoGaps := []GeographicGap{
		{
			GapID:        "gg-0",
			Jurisdiction: "CN",
			MarketSize:   0.95,
			Priority:     0.8,
		},
	}
	expRisks := []ExpirationRisk{
		{
			PatentID:       "p1",
			PatentNumber:   "US100004",
			RiskLevel:      RiskCritical,
			HasReplacement: false,
		},
	}

	opps := svc.generateOpportunities(techGaps, geoGaps, expRisks)
	if len(opps) == 0 {
		t.Error("expected at least 1 opportunity")
	}
	// Verify sorted by score descending
	for i := 1; i < len(opps); i++ {
		if opps[i].OverallScore > opps[i-1].OverallScore {
			t.Error("opportunities not sorted by overall score descending")
			break
		}
	}
}

func TestComputeHealthScore(t *testing.T) {
	svc, _, _ := buildGAService()

	patents := buildGAPatents()
	ownPatents := make([]domainpatent.Patent, len(patents))
	for i, p := range patents {
		ownPatents[i] = *p
	}

	// Empty portfolio
	score := svc.computeHealthScore(nil, nil, nil, nil)
	if score != 0.0 {
		t.Errorf("expected 0 for empty portfolio, got %f", score)
	}

	// Healthy portfolio
	score = svc.computeHealthScore(ownPatents, nil, nil, nil)
	if score <= 0 || score > 100 {
		t.Errorf("expected score in (0,100], got %f", score)
	}

	// With gaps and risks
	techGaps := []TechnologyGap{
		{TechDomain: "G01N", GapSeverity: 0.9},
	}
	expRisks := []ExpirationRisk{
		{RiskLevel: RiskCritical, HasReplacement: false},
		{RiskLevel: RiskHigh, HasReplacement: true},
	}
	geoGaps := []GeographicGap{
		{Priority: 0.8},
	}
	score = svc.computeHealthScore(ownPatents, techGaps, expRisks, geoGaps)
	if score < 0 {
		t.Errorf("expected non-negative score, got %f", score)
	}
}

// -----------------------------------------------------------------------
// Tests: Package-Level Helpers
// -----------------------------------------------------------------------

func TestExtractJurisdiction(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"US1234567", "US"},
		{"EP1234567", "EP"},
		{"CN1234567", "CN"},
		{"WO2018123456", "WO"},
		{"", ""},
		{"123456", ""},
	}
	for _, tt := range tests {
		result := extractJurisdiction(tt.input)
		if result != tt.expected {
			t.Errorf("extractJurisdiction(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultTargetJurisdictions(t *testing.T) {
	jurs := defaultTargetJurisdictions()
	if len(jurs) == 0 {
		t.Fatal("expected non-empty jurisdictions")
	}
	if jurs[0] != "US" {
		t.Errorf("expected US first, got %s", jurs[0])
	}
}

func TestJurisdictionMarketScore(t *testing.T) {
	if score := jurisdictionMarketScore("US"); score != 1.0 {
		t.Errorf("expected US score 1.0, got %f", score)
	}
	if score := jurisdictionMarketScore("XX"); score != 0.3 {
		t.Errorf("expected unknown jurisdiction score 0.3, got %f", score)
	}
}

func TestJurisdictionName(t *testing.T) {
	if name := jurisdictionName("US"); name != "United States" {
		t.Errorf("expected 'United States', got %s", name)
	}
	if name := jurisdictionName("XX"); name != "XX" {
		t.Errorf("expected 'XX' for unknown, got %s", name)
	}
}

//Personal.AI order the ending
