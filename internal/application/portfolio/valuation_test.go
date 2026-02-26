// File: internal/application/portfolio/valuation_test.go
// Tests for patent portfolio valuation application service.

package portfolio

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Test Helpers (using shared mocks from common_test.go)
// ---------------------------------------------------------------------------

func buildValuationTestService(
	patentRepo *mockPatentRepo,
	portfolioRepo *mockPortfolioRepo,
	assessmentRepo *mockAssessmentRepo,
	aiScorer *mockAIScorer,
	citationRepo *mockCitationRepo,
	cache *mockCache,
) ValuationService {
	if patentRepo == nil { patentRepo = newMockPatentRepo() }
	if portfolioRepo == nil { portfolioRepo = newMockPortfolioRepo() }
	if assessmentRepo == nil { assessmentRepo = newMockAssessmentRepo() }
	if cache == nil { cache = newMockCache() }

	var citationRepoInterface CitationRepository
	if citationRepo != nil { citationRepoInterface = citationRepo }
	
	var aiScorerInterface IntelligenceValueScorer
	if aiScorer != nil { aiScorerInterface = aiScorer }

	return NewValuationService(
		&mockPortfolioDomainSvc{},
		mockValuationDomainSvc{},
		patentRepo,
		portfolioRepo,
		assessmentRepo,
		aiScorerInterface,
		citationRepoInterface,
		mockLogger{},
		cache,
		nil,
		&ValuationServiceConfig{Concurrency: 5, CacheTTL: time.Minute},
	)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestTierFromScore(t *testing.T) {
	tests := []struct {
		score    float64
		expected PatentTier
	}{
		{100, TierS}, {90, TierS},
		{89.99, TierA}, {80, TierA},
		{79.99, TierB}, {65, TierB},
		{64.99, TierC}, {50, TierC},
		{49.99, TierD}, {0, TierD},
	}
	for _, tt := range tests {
		got := TierFromScore(tt.score)
		if got != tt.expected {
			t.Errorf("TierFromScore(%.2f) = %s, want %s", tt.score, got, tt.expected)
		}
	}
}

func TestAssessPatent_Success(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := createTestPatentWithMolecules("10000000-0000-0000-0000-000000000001", "Patent One", "A61K", "OwnCorp", time.Now(), 2.0, nil)
	patentRepo.patents[pat.ID] = pat
	patentRepo.byIDs[pat.ID] = pat

	svc := buildValuationTestService(patentRepo, nil, nil, nil, nil, nil)

	resp, err := svc.AssessPatent(context.Background(), &SinglePatentAssessmentRequest{PatentID: pat.ID})
	if err != nil {
		t.Fatalf("AssessPatent failed: %v", err)
	}
	if resp.PatentID != pat.ID {
		t.Errorf("expected ID %s, got %s", pat.ID, resp.PatentID)
	}
}

func TestAssessPatent_PatentNotFound(t *testing.T) {
	svc := buildValuationTestService(nil, nil, nil, nil, nil, nil)
	_, err := svc.AssessPatent(context.Background(), &SinglePatentAssessmentRequest{PatentID: "missing"})
	if err == nil {
		t.Fatal("expected error for missing patent")
	}
}

func TestAssessPatent_WithAIScorer(t *testing.T) {
	patentRepo := newMockPatentRepo()
	pat := createTestPatentWithMolecules("10000000-0000-0000-0000-000000000002", "AI Patent", "A61K", "OwnCorp", time.Now(), 2.0, nil)
	patentRepo.patents[pat.ID] = pat
	patentRepo.byIDs[pat.ID] = pat

	aiScorer := newMockAIScorer()
	aiScorer.scores[DimensionTechnicalValue] = map[string]float64{"novelty": 95}

	svc := buildValuationTestService(patentRepo, nil, nil, aiScorer, nil, nil)

	resp, err := svc.AssessPatent(context.Background(), &SinglePatentAssessmentRequest{PatentID: pat.ID})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if atomic.LoadInt32(&aiScorer.calls) == 0 {
		t.Error("expected AI scorer call")
	}
	if resp.AssessorType != AssessorHybrid {
		t.Errorf("expected hybrid, got %s", resp.AssessorType)
	}
}

func TestExportAssessment_JSON(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	rec := &AssessmentRecord{
		ID: "EXP_JSON", PatentID: "P1", OverallScore: 75, Tier: TierB,
		AssessedAt: time.Now(), AssessorType: AssessorHybrid,
	}
	_ = assessmentRepo.Save(context.Background(), rec)

	svc := buildValuationTestService(nil, nil, assessmentRepo, nil, nil, nil)
	data, err := svc.ExportAssessment(context.Background(), "EXP_JSON", ExportJSON)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !strings.Contains(string(data), "EXP_JSON") {
		t.Error("json missing ID")
	}
}

func TestRecommendActions_TierS(t *testing.T) {
	assessmentRepo := newMockAssessmentRepo()
	_ = assessmentRepo.Save(context.Background(), &AssessmentRecord{
		ID: "RA_S", PatentID: "P_S", OverallScore: 95, Tier: TierS,
		DimensionScores: map[AssessmentDimension]float64{DimensionTechnicalValue: 95},
		AssessedAt: time.Now(), AssessorType: AssessorHybrid,
	})

	svc := buildValuationTestService(nil, nil, assessmentRepo, nil, nil, nil)
	recs, err := svc.RecommendActions(context.Background(), "RA_S")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	found := false
	for _, r := range recs {
		if r.Type == ActionMaintain { found = true }
	}
	if !found {
		t.Error("expected maintain action for Tier S")
	}
}

func TestErrorTypes(t *testing.T) {
	// Coverage for error types if needed
	_ = pkgerrors.ErrNotFound
}

//Personal.AI order the ending
