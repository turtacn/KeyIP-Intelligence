// Phase 10 - File 223 of 349
// Tests for ConstellationService application service.

package portfolio

import (
	"context"
	"testing"
	"time"

	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
)

// -----------------------------------------------------------------------
// Test Helpers (using shared mocks from common_test.go)
// -----------------------------------------------------------------------

func buildConstellationTestConfig(overrides ...func(*ConstellationServiceConfig)) ConstellationServiceConfig {
	patentRepo := newMockPatentRepo()
	molRepo := newMockMoleculeRepo()
	portfolioRepo := newMockPortfolioRepo()

	// Seed test data: 3 patents, each with 1 molecule.
	p1 := createTestPatentWithMolecules("00000000-0000-0000-0000-000000000001", "US1234", "A61K", "OwnCorp", time.Date(2020, 3, 15, 0, 0, 0, 0, time.UTC), 8.5, []string{"mol-1"})
	p2 := createTestPatentWithMolecules("00000000-0000-0000-0000-000000000002", "US5678", "C07D", "OwnCorp", time.Date(2021, 7, 20, 0, 0, 0, 0, time.UTC), 7.2, []string{"mol-2"})
	p3 := createTestPatentWithMolecules("00000000-0000-0000-0000-000000000003", "US9012", "A61K", "OwnCorp", time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC), 9.1, []string{"mol-3"})

	portfolioID := "00000000-0000-0000-0000-000000000001"
	// Set up portfolio repo patents
	portfolioRepo.patents[portfolioID] = []*domainpatent.Patent{p1, p2, p3}

	// Molecules.
	molRepo.molecules["mol-1"] = (&mockMolecule{id: "mol-1", smiles: "CCO"}).toMolecule()
	molRepo.molecules["mol-2"] = (&mockMolecule{id: "mol-2", smiles: "c1ccccc1"}).toMolecule()
	molRepo.molecules["mol-3"] = (&mockMolecule{id: "mol-3", smiles: "CC(=O)O"}).toMolecule()

	portfolioForService := (&mockPortfolio{id: portfolioID, name: "Test Portfolio"}).toPortfolio()
	portfolioRepo.portfolios[portfolioID] = portfolioForService
	
	cfg := ConstellationServiceConfig{
		PortfolioService:    &mockPortfolioService{portfolio: portfolioForService},
		PortfolioRepository: portfolioRepo,
		MoleculeService:     &mockMoleculeService{},
		PatentRepository:    patentRepo,
		MoleculeRepository:  molRepo,
		GNNInference:        &mockGNNInference{},
		Logger:              &mockLogger{},
		Cache:               newMockConstellationCache(),
		CacheTTL:            5 * time.Minute,
	}

	for _, override := range overrides {
		override(&cfg)
	}

	return cfg
}

// -----------------------------------------------------------------------
// Tests: GenerateConstellation
// -----------------------------------------------------------------------

func TestGenerateConstellation_Success(t *testing.T) {
	cfg := buildConstellationTestConfig()
	svc, _ := NewConstellationService(cfg)

	resp, err := svc.GenerateConstellation(context.Background(), &ConstellationRequest{
		PortfolioID:        "00000000-0000-0000-0000-000000000001",
		IncludeWhiteSpaces: true,
		Reduction: DimensionReduction{
			Algorithm:  ReductionUMAP,
			Dimensions: 2,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.PortfolioID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("expected portfolio_id 'portfolio-1', got '%s'", resp.PortfolioID)
	}
	// We expect 3 points (one per patent-molecule pair)
	if len(resp.Points) != 3 {
		t.Errorf("expected 3 constellation points, got %d", len(resp.Points))
	}
}

func TestGenerateConstellation_NilRequest(t *testing.T) {
	cfg := buildConstellationTestConfig()
	svc, _ := NewConstellationService(cfg)

	_, err := svc.GenerateConstellation(context.Background(), nil)
	if err == nil {
		t.Fatal("expected validation error for nil request")
	}
}

func TestGetTechDomainDistribution_Success(t *testing.T) {
	cfg := buildConstellationTestConfig()
	svc, _ := NewConstellationService(cfg)

	dist, err := svc.GetTechDomainDistribution(context.Background(), "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dist.TotalCount != 3 {
		t.Errorf("expected total count 3, got %d", dist.TotalCount)
	}
}

//Personal.AI order the ending
