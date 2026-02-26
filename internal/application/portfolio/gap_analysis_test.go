// Phase 10 - File 225 of 349
package portfolio

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
)

// Standard test UUIDs for gap analysis tests
const (
	testPortfolioGapID    = "40000000-0000-0000-0000-000000000002"
	testPortfolioExpID    = "40000000-0000-0000-0000-000000000011"
	testPortfolioGeoID    = "40000000-0000-0000-0000-000000000012"
	testPortfolioDefID    = "40000000-0000-0000-0000-000000000013"
	testPortfolioOppsID   = "40000000-0000-0000-0000-000000000014"
)

// -----------------------------------------------------------------------
// Tests: GapAnalysisService Construction
// -----------------------------------------------------------------------

func TestNewGapAnalysisService_Success(t *testing.T) {
	cfg := GapAnalysisServiceConfig{
		PortfolioService:    &mockPortfolioService{portfolio: createTestPortfolio("Test")},
		PortfolioRepository: newMockPortfolioRepoWithData(createTestPortfolio("Test")),
		PatentRepository:    newMockPatentRepo(),
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
				PortfolioService:    &mockPortfolioService{portfolio: createTestPortfolio("test")},
				PortfolioRepository: newMockPortfolioRepoWithData(createTestPortfolio("test")),
				PatentRepository:    newMockPatentRepo(),
				Logger:              &mockLogger{},
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
		PortfolioService:    &mockPortfolioService{portfolio: createTestPortfolio("test")},
		PortfolioRepository: newMockPortfolioRepoWithData(createTestPortfolio("test")),
		PatentRepository:    newMockPatentRepo(),
		Logger:              &mockLogger{},
		CacheTTL:            0,
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
	// Add to byIDs for FindByID used by PortfolioRepo.GetPatents mock implementation
	for _, p := range ownPatents {
		repo.byIDs[p.ID] = p
	}

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
	patentRepo := buildGapTestPatentRepo()
	testPortfolio := createTestPortfolioWithID(testPortfolioGapID, "Gap Test")

	portfolioRepo := newMockPortfolioRepoWithData(testPortfolio)
	// Populate patents in portfolioRepo
	ownPatents := []*domainpatent.Patent{
		createTestPatent("US1001", "A61K", "OwnCorp", time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC), 8.0),
		createTestPatent("US1002", "A61K", "OwnCorp", time.Date(2018, 6, 1, 0, 0, 0, 0, time.UTC), 7.5),
		createTestPatent("US1003", "C07D", "OwnCorp", time.Date(2022, 3, 1, 0, 0, 0, 0, time.UTC), 9.0),
	}
	portfolioRepo.patents[testPortfolioGapID] = ownPatents

	cfg := GapAnalysisServiceConfig{
		PortfolioService:    &mockPortfolioService{portfolio: testPortfolio},
		PortfolioRepository: portfolioRepo,
		PatentRepository:    patentRepo,
		Logger:              &mockLogger{},
	}
	svc, _ := NewGapAnalysisService(cfg)

	resp, err := svc.AnalyzeGaps(context.Background(), &GapAnalysisRequest{
		PortfolioID:      testPortfolioGapID,
		CompetitorNames:  []string{"RivalCo"},
		ExpirationWindow: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != testPortfolioGapID {
		t.Errorf("expected %s, got %s", testPortfolioGapID, resp.PortfolioID)
	}
}

// Helper to create a test portfolio with random UUID
func createTestPortfolio(name string) *domainportfolio.Portfolio {
	return createTestPortfolioWithID(uuid.New().String(), name)
}

// Helper to create a test portfolio with specific ID
func createTestPortfolioWithID(id, name string) *domainportfolio.Portfolio {
	now := time.Now()
	portfolioID, err := uuid.Parse(id)
	if err != nil {
		portfolioID = uuid.New()
	}
	p := &domainportfolio.Portfolio{
		ID:           portfolioID,
		Name:         name,
		OwnerID:      uuid.New(),
		TechDomains:  []string{"C07D"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return p
}

// Helper to create a mock portfolio repository with predefined portfolios
func newMockPortfolioRepoWithData(portfolios ...*domainportfolio.Portfolio) *mockPortfolioRepo {
	repo := newMockPortfolioRepo()
	for _, p := range portfolios {
		repo.portfolios[p.ID.String()] = p
	}
	return repo
}

//Personal.AI order the ending
