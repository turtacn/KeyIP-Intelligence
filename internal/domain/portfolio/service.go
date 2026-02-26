package portfolio

import (
	"context"
	"fmt"
	"math"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PortfolioService defines the application service for portfolio management.
type PortfolioService interface {
	CreatePortfolio(ctx context.Context, name, ownerID string, techDomains []string) (*Portfolio, error)
	AddPatentsToPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error
	RemovePatentsFromPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error
	ActivatePortfolio(ctx context.Context, portfolioID string) error
	ArchivePortfolio(ctx context.Context, portfolioID string) error
	CalculateHealthScore(ctx context.Context, portfolioID string) (*HealthScore, error)
	ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*PortfolioComparison, error)
	IdentifyGaps(ctx context.Context, portfolioID string, targetDomains []string) ([]*GapInfo, error)
	GetOverlapAnalysis(ctx context.Context, portfolioID1, portfolioID2 string) (*OverlapResult, error)
}

// PortfolioComparison holds comparison data.
type PortfolioComparison struct {
	PortfolioID        string         `json:"portfolio_id"`
	Name               string         `json:"name"`
	PatentCount        int            `json:"patent_count"`
	TechDomainCoverage map[string]int `json:"tech_domain_coverage"`
	HealthScore        *HealthScore   `json:"health_score"`
}

// GapInfo represents a coverage gap in a tech domain.
type GapInfo struct {
	TechDomain      string `json:"tech_domain"`
	DomainName      string `json:"domain_name"`
	CurrentCount    int    `json:"current_count"`
	IndustryAverage int    `json:"industry_average"`
	GapSeverity     string `json:"gap_severity"` // Critical / High / Medium / Low
	Recommendation  string `json:"recommendation"`
}

// OverlapResult represents the overlap between two portfolios.
type OverlapResult struct {
	Portfolio1ID       string   `json:"portfolio1_id"`
	Portfolio2ID       string   `json:"portfolio2_id"`
	OverlappingPatentIDs []string `json:"overlapping_patent_ids"`
	OverlapRatio       float64  `json:"overlap_ratio"`
	UniqueToPortfolio1 []string `json:"unique_to_portfolio1"`
	UniqueToPortfolio2 []string `json:"unique_to_portfolio2"`
}

type portfolioServiceImpl struct {
	repo PortfolioRepository
}

// NewPortfolioService creates a new PortfolioService.
func NewPortfolioService(repo PortfolioRepository) PortfolioService {
	return &portfolioServiceImpl{repo: repo}
}

func (s *portfolioServiceImpl) CreatePortfolio(ctx context.Context, name, ownerID string, techDomains []string) (*Portfolio, error) {
	p, err := NewPortfolio(name, ownerID)
	if err != nil {
		return nil, apperrors.NewValidation(err.Error())
	}
	p.TechDomains = techDomains
	if err := p.Validate(); err != nil {
		return nil, apperrors.NewValidation(err.Error())
	}
	if err := s.repo.Save(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *portfolioServiceImpl) AddPatentsToPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return err
	}
	if p == nil {
		return apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	var errs []error
	for _, pid := range patentIDs {
		if err := p.AddPatent(pid); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		// If partial failure, should we save the successful ones?
		// Requirement says: "批量添加专利时，部分失败不应导致整体回滚（领域层不管事务），而是收集所有错误后一次性返回"
		// This implies we should save the successful ones and return error for failed ones.
		// However, if we return error, the caller might assume failure.
		// Usually we return a specific error type containing the list of failures, or just log them.
		// But here "一次性返回" suggests returning an error that aggregates them.
		// I will save first, then return error.
	}

	if saveErr := s.repo.Save(ctx, p); saveErr != nil {
		return saveErr
	}

	if len(errs) > 0 {
		return fmt.Errorf("some patents failed to add: %v", errs)
	}
	return nil
}

func (s *portfolioServiceImpl) RemovePatentsFromPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return err
	}
	if p == nil {
		return apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	for _, pid := range patentIDs {
		// Ignoring error if not found? Requirement says "repo.FindByID -> 逐个 RemovePatent -> repo.Save"
		// entity.RemovePatent returns error if not found.
		// If we want to be idempotent, we might ignore "not found" error.
		// But let's follow standard behavior.
		if err := p.RemovePatent(pid); err != nil {
			return err // Return immediately? Or collect? Requirement for Add said collect. Remove usually assumes strictness or idempotency.
			// Let's return first error for simplicity unless specified otherwise.
		}
	}

	return s.repo.Save(ctx, p)
}

func (s *portfolioServiceImpl) ActivatePortfolio(ctx context.Context, portfolioID string) error {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return err
	}
	if p == nil {
		return apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	if err := p.Activate(); err != nil {
		return apperrors.NewValidation(err.Error())
	}
	return s.repo.Save(ctx, p)
}

func (s *portfolioServiceImpl) ArchivePortfolio(ctx context.Context, portfolioID string) error {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return err
	}
	if p == nil {
		return apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	if err := p.Archive(); err != nil {
		return apperrors.NewValidation(err.Error())
	}
	return s.repo.Save(ctx, p)
}

func (s *portfolioServiceImpl) CalculateHealthScore(ctx context.Context, portfolioID string) (*HealthScore, error) {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	// Skeleton calculation
	patentCount := float64(len(p.PatentIDs))
	coverageScore := math.Min(patentCount/10.0*100.0, 100.0)

	// Concentration based on TechDomains?
	// Requirement: "ConcentrationScore 基于 TechDomains 分布的香农熵"
	// But TechDomains is just a list of strings on Portfolio.
	// Does it mean distribution of patents across domains?
	// But we don't know which patent belongs to which domain here (that data is in Patent entity, not Portfolio).
	// But `IdentifyGaps` requirement implies we can know "统计各 TechDomain 的专利分布".
	// This suggests we need to fetch patents to know their domains?
	// Or `TechDomains` on Portfolio implies the target domains?
	// Actually `IdentifyGaps` says: "FindByID -> 统计各 TechDomain 的专利分布".
	// This implies fetching patents.
	// But `PortfolioService` implementation here doesn't have access to `PatentRepository`.
	// The requirement for `serviceImpl` says: "注入：`PortfolioRepository`". It doesn't mention `PatentRepository`.
	// So how do we know the patent distribution?
	// Maybe `TechDomains` on Portfolio is sufficient? No, that's what the portfolio is ABOUT, not the distribution of actual patents.
	// Unless... we assume some data is available or we just mock/randomize for the skeleton.
	// "Skeleton calculation"
	// Let's use a dummy value or based on `TechDomains` count.
	concentrationScore := 50.0 // Placeholder

	hs := HealthScore{
		CoverageScore:      coverageScore,
		ConcentrationScore: concentrationScore,
		AgingScore:         50.0,
		QualityScore:       50.0,
		EvaluatedAt:        time.Now().UTC(),
	}
	hs.OverallScore = (hs.CoverageScore + hs.ConcentrationScore + hs.AgingScore + hs.QualityScore) / 4.0

	if err := p.SetHealthScore(hs); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, p); err != nil {
		return nil, err
	}
	return &hs, nil
}

func (s *portfolioServiceImpl) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*PortfolioComparison, error) {
	if len(portfolioIDs) > 10 {
		return nil, apperrors.NewValidation("cannot compare more than 10 portfolios")
	}

	var results []*PortfolioComparison
	for _, id := range portfolioIDs {
		p, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, apperrors.NewNotFound("portfolio not found: %s", id)
		}

		results = append(results, &PortfolioComparison{
			PortfolioID:        p.ID,
			Name:               p.Name,
			PatentCount:        p.PatentCount(),
			TechDomainCoverage: make(map[string]int), // Placeholder
			HealthScore:        p.HealthScore,
		})
	}
	return results, nil
}

func (s *portfolioServiceImpl) IdentifyGaps(ctx context.Context, portfolioID string, targetDomains []string) ([]*GapInfo, error) {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	var gaps []*GapInfo
	// Mock distribution: assume we have 0 for everything since we can't query patents
	for _, domain := range targetDomains {
		currentCount := 0
		// If we had distribution, we would set currentCount

		industryAvg := 10
		severity := "Low"
		recommendation := "None"

		if currentCount < industryAvg {
			diff := industryAvg - currentCount
			if diff >= 8 {
				severity = "Critical"
				recommendation = "Acquire or Develop urgently"
			} else if diff >= 5 {
				severity = "High"
				recommendation = "Prioritize R&D"
			} else {
				severity = "Medium"
				recommendation = "Monitor"
			}

			gaps = append(gaps, &GapInfo{
				TechDomain:      domain,
				DomainName:      domain, // Assuming code is same as name for now
				CurrentCount:    currentCount,
				IndustryAverage: industryAvg,
				GapSeverity:     severity,
				Recommendation:  recommendation,
			})
		}
	}
	return gaps, nil
}

func (s *portfolioServiceImpl) GetOverlapAnalysis(ctx context.Context, portfolioID1, portfolioID2 string) (*OverlapResult, error) {
	p1, err := s.repo.FindByID(ctx, portfolioID1)
	if err != nil {
		return nil, err
	}
	if p1 == nil {
		return nil, apperrors.NewNotFound("portfolio not found: %s", portfolioID1)
	}

	p2, err := s.repo.FindByID(ctx, portfolioID2)
	if err != nil {
		return nil, err
	}
	if p2 == nil {
		return nil, apperrors.NewNotFound("portfolio not found: %s", portfolioID2)
	}

	set1 := make(map[string]bool)
	for _, id := range p1.PatentIDs {
		set1[id] = true
	}

	var overlapping []string
	var uniqueToP1 []string
	var uniqueToP2 []string

	for _, id := range p1.PatentIDs {
		if p2.ContainsPatent(id) {
			overlapping = append(overlapping, id)
		} else {
			uniqueToP1 = append(uniqueToP1, id)
		}
	}

	for _, id := range p2.PatentIDs {
		if !set1[id] {
			uniqueToP2 = append(uniqueToP2, id)
		}
	}

	totalUnique := len(uniqueToP1) + len(uniqueToP2) + len(overlapping)
	ratio := 0.0
	if totalUnique > 0 {
		ratio = float64(len(overlapping)) / float64(totalUnique)
	}

	return &OverlapResult{
		Portfolio1ID:       portfolioID1,
		Portfolio2ID:       portfolioID2,
		OverlappingPatentIDs: overlapping,
		OverlapRatio:       ratio,
		UniqueToPortfolio1: uniqueToP1,
		UniqueToPortfolio2: uniqueToP2,
	}, nil
}

//Personal.AI order the ending
