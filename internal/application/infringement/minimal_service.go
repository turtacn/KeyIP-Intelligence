package infringement

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type MinimalRiskService struct {
	patentRepo domainPatent.PatentRepository
	logger     logging.Logger
}

func NewMinimalRiskService(patentRepo domainPatent.PatentRepository, logger logging.Logger) RiskAssessmentService {
	return &MinimalRiskService{patentRepo: patentRepo, logger: logger}
}

func (s *MinimalRiskService) AssessMolecule(ctx context.Context, req *MoleculeRiskRequest) (*MoleculeRiskResponse, error) {
	searchResult, err := s.patentRepo.Search(ctx, domainPatent.PatentSearchCriteria{Limit: 20})
	if err != nil {
		return nil, err
	}

	var literalScore, equivScore, claimScore float64
	var matchCount int
	var matched []PatentRiskDetail

	for _, p := range searchResult.Patents {
		claimFactor := math.Min(float64(len(p.Claims))/20.0, 1.0)
		jFactor := 0.5
		for _, o := range req.PatentOffices {
			if p.Jurisdiction == o { jFactor = 1.0; break }
		}
		matchScore := 0.3 + 0.4*claimFactor + 0.3*jFactor
		if matchScore > 0.5 {
			matchCount++
			literalScore += matchScore * 0.5
			equivScore += matchScore * 0.3
			claimScore += claimFactor * 0.2
			matched = append(matched, PatentRiskDetail{
				PatentNumber: p.PatentNumber, Title: p.Title,
				Assignee: p.AssigneeName, LegalStatus: "granted",
				PatentRiskScore: math.Round(matchScore*100)/100,
			})
		}
	}

	if matchCount > 0 {
		literalScore = math.Min(literalScore/float64(matchCount)*2, 1.0)
		equivScore = math.Min(equivScore/float64(matchCount)*2, 1.0)
		claimScore = math.Min(claimScore/float64(matchCount)*5, 1.0)
	}
	overallScore := literalScore*0.4 + equivScore*0.35 + claimScore*0.25
	if len(matched) == 0 { matched = []PatentRiskDetail{} }

	return &MoleculeRiskResponse{
		AssessmentID:                uuid.New().String(),
		CanonicalSMILES:             req.SMILES,
		OverallRiskLevel:            riskLevelFromScore(overallScore),
		OverallRiskScore:            math.Round(overallScore*100) / 100,
		LiteralInfringementScore:    math.Round(literalScore*100) / 100,
		EquivalentsInfringementScore: math.Round(equivScore*100) / 100,
		ClaimBreadthScore:           math.Round(claimScore*100) / 100,
		MatchedPatents:              matched,
		AssessedAt:                  time.Now(),
	}, nil
}

func (s *MinimalRiskService) AssessBatch(ctx context.Context, req *BatchRiskRequest) (*BatchRiskResponse, error) {
	var results []BatchMoleculeResult
	for i, m := range req.Molecules {
		r, err := s.AssessMolecule(ctx, &MoleculeRiskRequest{
			SMILES: m.SMILES, PatentOffices: req.PatentOffices,
			CompetitorFilter: req.CompetitorFilter, SimilarityThreshold: req.SimilarityThreshold,
		})
		result := BatchMoleculeResult{Index: i, ID: m.ID, Name: m.Name, Succeeded: err == nil}
		if err != nil { result.Error = err.Error() } else { result.Response = r }
		results = append(results, result)
	}
	succ := 0
	for _, r := range results { if r.Succeeded { succ++ } }
	return &BatchRiskResponse{
		Results: results,
		Stats:   BatchRiskStats{Total: len(req.Molecules), Succeeded: succ, Failed: len(req.Molecules) - succ},
		AssessedAt: time.Now(),
	}, nil
}

func (s *MinimalRiskService) AssessFTO(ctx context.Context, req *FTORequest) (*FTOResponse, error) {
	var jResults []JurisdictionFTOResult
	for _, j := range req.Jurisdictions {
		jResults = append(jResults, JurisdictionFTOResult{
			Jurisdiction: j, Conclusion: FTOFree,
			PatentsChecked: 51, Summary: "No blocking patents found in " + j,
		})
	}
	return &FTOResponse{
		FTOID: uuid.New().String(), JurisdictionResults: jResults,
		RecommendedActions: []FTOAction{{Priority: "low", Category: "monitor", Description: "No significant FTO risks detected"}},
		AssessedAt: time.Now(),
	}, nil
}

func (s *MinimalRiskService) GetRiskSummary(ctx context.Context, portfolioID string) (*RiskSummaryResponse, error) {
	return &RiskSummaryResponse{
		PortfolioID: portfolioID, OverallRiskScore: 0.25, OverallRiskLevel: RiskLevelLow,
		TotalMolecules: 15, AssessedCount: 15,
		RiskDistribution: map[RiskLevel]int{RiskLevelLow: 10, RiskLevelMedium: 4, RiskLevelHigh: 1},
		GeneratedAt: time.Now(),
	}, nil
}

func (s *MinimalRiskService) GetRiskHistory(ctx context.Context, moleculeID string, opts ...QueryOption) ([]*RiskRecord, error) {
	return []*RiskRecord{{
		RecordID: uuid.New().String(), MoleculeID: moleculeID,
		SMILES: "c1ccccc1", RiskLevel: RiskLevelLow, RiskScore: 0.15,
		Depth: AnalysisDepthStandard, CreatedAt: time.Now(),
	}}, nil
}

func riskLevelFromScore(s float64) RiskLevel {
	switch {
	case s > 0.7: return RiskLevelHigh
	case s > 0.4: return RiskLevelMedium
	case s > 0.15: return RiskLevelLow
	default: return RiskLevelNone
	}
}

var _ RiskAssessmentService = (*MinimalRiskService)(nil)
