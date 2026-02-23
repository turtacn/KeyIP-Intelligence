// Phase 10 - File 225 of 349
package portfolio

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
)

// -----------------------------------------------------------------------
// Tests: GapAnalysisService Construction
// -----------------------------------------------------------------------

func TestNewGapAnalysisService_Success(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("Test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
				PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
	ownPatents := []*domainpatent.Patent{
		createTestPatent("US1001", "A61K", "OwnCorp", time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC), 8.0),
		createTestPatent("US1002", "A61K", "OwnCorp", time.Date(2018, 6, 1, 0, 0, 0, 0, time.UTC), 7.5),
		createTestPatent("US1003", "C07D", "OwnCorp", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), 9.0),
	}
	repo.byPortfolio["40000000-0000-0000-0000-000000000002"] = ownPatents

	compPatents := []*domainpatent.Patent{
		createTestPatent("EP2001", "A61K", "RivalCo", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), 7.0),
		createTestPatent("EP2002", "G16B", "RivalCo", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), 8.5),
		createTestPatent("CN3001", "G16B", "RivalCo", time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), 6.0),
		createTestPatent("JP4001", "C12N", "RivalCo", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), 9.0),
	}
	repo.byAssignee["RivalCo"] = compPatents

	return repo
}

func TestAnalyzeGaps_Success(t *testing.T) {
	repo := buildGapTestPatentRepo()
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("Gap Test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	resp, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID:      "40000000-0000-0000-0000-000000000002",
		CompetitorNames:  []string{"RivalCo"},
		ExpirationWindow: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != "40000000-0000-0000-0000-000000000002" {
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository: newMockPatentRepo(),
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	_, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{PortfolioID: "40000000-0000-0000-0000-000000000001"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestAnalyzeGaps_NoCompetitors(t *testing.T) {
	repo := buildGapTestPatentRepo()
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	resp, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID: "40000000-0000-0000-0000-000000000002",
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
	repo.byPortfolio["port-exp"] = []*domainpatent.Patent{
		createTestPatent("US9999", "A61K", "OwnCorp", time.Now().AddDate(-16, 0, 0), 8.0),
		createTestPatent("US8888", "C07D", "OwnCorp", time.Now().AddDate(-5, 0, 0), 7.0),
	}

	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
	repo.byPortfolio["port-def"] = []*domainpatent.Patent{}
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
	repo.byPortfolio["port-geo"] = []*domainpatent.Patent{
		createTestPatent("US1111", "A61K", "OwnCorp", time.Now(), 5.0),
		createTestPatent("EP2222", "A61K", "OwnCorp", time.Now(), 5.0),
	}
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
	repo.byPortfolio["port-def-geo"] = []*domainpatent.Patent{
		createTestPatent("US5555", "A61K", "OwnCorp", time.Now(), 5.0),
	}
	cfg := GapAnalysisServiceConfig{
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	opps, err := svc.GetFilingOpportunities(context.Background(), "40000000-0000-0000-0000-000000000002", 5)
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
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
		PortfolioService: &mockPortfolioService{portfolio: createTestPortfolio("test")},
PortfolioRepository: newMockPortfolioRepoConstellation(),
		PatentRepository: repo,
		Logger:           &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	opps, err := svc.GetFilingOpportunities(context.Background(), "40000000-0000-0000-0000-000000000002", 0)
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
	patentPtrs := []*domainpatent.Patent{
		// Expires in ~6 months -> critical
		createTestPatent("US-R1", "A61K", "OwnCorp", now.AddDate(-19, -6, 0), 5.0),
		// Expires in ~1.5 years -> high
		createTestPatent("US-R2", "A61K", "OwnCorp", now.AddDate(-18, -6, 0), 5.0),
		// Expires in ~2.5 years -> medium
		createTestPatent("US-R3", "C07D", "OwnCorp", now.AddDate(-17, -6, 0), 5.0),
		// Expires in ~4 years -> low
		createTestPatent("US-R4", "C07D", "OwnCorp", now.AddDate(-16, 0, 0), 5.0),
	}
	
	// Convert pointers to values as the function expects
	patents := make([]domainpatent.Patent, len(patentPtrs))
	for i, p := range patentPtrs {
		patents[i] = *p
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
	patentPtrs := make([]*domainpatent.Patent, 10)
	for i := range patentPtrs {
		patentPtrs[i] = createTestPatent(fmt.Sprintf("US-%d", i), "A61K", "OwnCorp", time.Now(), 5.0)
	}
	// Convert pointers to values
	patents := make([]domainpatent.Patent, len(patentPtrs))
	for i, p := range patentPtrs {
		patents[i] = *p
	}
	score := svc.computeHealthScore(patents, nil, nil, nil)
	if score <= 0 || score > 100 {
		t.Errorf("expected positive score <= 100, got %f", score)
	}
}

func TestComputeHealthScore_WithGaps(t *testing.T) {
	svc := &gapAnalysisServiceImpl{logger: &mockLogger{}}
	patentPtrs := make([]*domainpatent.Patent, 5)
	for i := range patentPtrs {
		patentPtrs[i] = createTestPatent(fmt.Sprintf("US-%d", i), "A61K", "OwnCorp", time.Now(), 5.0)
	}
	// Convert pointers to values
	patents := make([]domainpatent.Patent, len(patentPtrs))
	for i, p := range patentPtrs {
		patents[i] = *p
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

// Helper to create a test portfolio
func createTestPortfolio(name string) *domainportfolio.Portfolio {
	now := time.Now()
	p := &domainportfolio.Portfolio{
		ID:           uuid.New(),
		Name:         name,
		OwnerID:      uuid.New(),
		TechDomains:  []string{"C07D"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return p
}

// Helper to create test patent from mock data (simple version without ID and moleculeIDs)
func createTestPatent(number, techDomain, assignee string, filingDate time.Time, valueScore float64) *domainpatent.Patent {
	return createTestPatentWithMolecules("", number, techDomain, assignee, filingDate, valueScore, nil)
}

// Helper to create test patent with full fields including ID and moleculeIDs
func createTestPatentWithMolecules(id, number, techDomain, assignee string, filingDate time.Time, valueScore float64, moleculeIDs []string) *domainpatent.Patent {
	var patentID uuid.UUID
	if id != "" {
		if parsedID, err := uuid.Parse(id); err == nil {
			patentID = parsedID
		} else {
			patentID = uuid.New()
		}
	} else {
		patentID = uuid.New()
	}
	
	now := time.Now()
	expiryDate := filingDate.AddDate(20, 0, 0) // Standard 20-year term
	return &domainpatent.Patent{
		ID:              patentID,
		PatentNumber:    number,
		Title:           "Test Patent",
		Abstract:        "Test abstract",
		AssigneeName:    assignee,
		FilingDate:      &filingDate,
		GrantDate:       &now,
		ExpiryDate:      &expiryDate,
		Status:          domainpatent.PatentStatusGranted,
		Office:          domainpatent.OfficeUSPTO,
		IPCCodes:        []string{techDomain},
		KeyIPTechCodes:  []string{techDomain},
		MoleculeIDs:     moleculeIDs,
		Metadata:        map[string]any{"value_score": valueScore},
		CreatedAt:       now,
		UpdatedAt:       now,
		Version:         1,
	}
}
