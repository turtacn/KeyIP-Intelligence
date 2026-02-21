package portfolio

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PortfolioComparison provides a comparison view of multiple portfolios.
type PortfolioComparison struct {
	PortfolioID        string            `json:"portfolio_id"`
	Name               string            `json:"name"`
	PatentCount        int               `json:"patent_count"`
	TechDomainCoverage map[string]int    `json:"tech_domain_coverage"`
	HealthScore        *HealthScore      `json:"health_score"`
}

// GapInfo describes a missing or weak technical area in a portfolio.
type GapInfo struct {
	TechDomain      string `json:"tech_domain"`
	DomainName      string `json:"domain_name"`
	CurrentCount    int    `json:"current_count"`
	IndustryAverage int    `json:"industry_average"`
	GapSeverity     string `json:"gap_severity"`
	Recommendation  string `json:"recommendation"`
}

// OverlapResult describes the intersection of two portfolios.
type OverlapResult struct {
	Portfolio1ID        string   `json:"portfolio1_id"`
	Portfolio2ID        string   `json:"portfolio2_id"`
	OverlappingPatentIDs []string `json:"overlapping_patent_ids"`
	OverlapRatio        float64  `json:"overlap_ratio"`
	UniqueToPortfolio1  []string `json:"unique_to_portfolio1"`
	UniqueToPortfolio2  []string `json:"unique_to_portfolio2"`
}

// PortfolioService defines the domain service for portfolio management.
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
		return nil, err
	}
	p.TechDomains = techDomains
	if err := p.Validate(); err != nil {
		return nil, err
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

	var errs []string
	for _, id := range patentIDs {
		if err := p.AddPatent(id); err != nil {
			errs = append(errs, err.Error())
		}
	}

	saveErr := s.repo.Save(ctx, p)
	if saveErr != nil {
		return saveErr
	}

	if len(errs) > 0 {
		return errors.New(errors.ErrCodeConflict, fmt.Sprintf("failed to add some patents: %v", errs))
	}

	return nil
}

func (s *portfolioServiceImpl) RemovePatentsFromPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return err
	}

	for _, id := range patentIDs {
		if err := p.RemovePatent(id); err != nil {
			return err
		}
	}

	return s.repo.Save(ctx, p)
}

func (s *portfolioServiceImpl) ActivatePortfolio(ctx context.Context, portfolioID string) error {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return err
	}
	if err := p.Activate(); err != nil {
		return err
	}
	return s.repo.Save(ctx, p)
}

func (s *portfolioServiceImpl) ArchivePortfolio(ctx context.Context, portfolioID string) error {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return err
	}
	if err := p.Archive(); err != nil {
		return err
	}
	return s.repo.Save(ctx, p)
}

func (s *portfolioServiceImpl) CalculateHealthScore(ctx context.Context, portfolioID string) (*HealthScore, error) {
	p, err := s.repo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	patentCount := float64(p.PatentCount())
	coverageScore := math.Min(patentCount/10.0*100.0, 100.0)

	// ConcentrationScore based on TechDomains distribution (Shannon entropy)
	// Skeleton: if no domains, 0. If domains exist, assume equal distribution for skeleton
	concentrationScore := 0.0
	numDomains := len(p.TechDomains)
	if numDomains > 0 && patentCount > 0 {
		// H = -sum(pi * log2(pi)), pi = 1/numDomains
		p_i := 1.0 / float64(numDomains)
		entropy := -float64(numDomains) * (p_i * math.Log2(p_i))
		maxEntropy := math.Log2(float64(numDomains))
		if maxEntropy > 0 {
			concentrationScore = (entropy / maxEntropy) * 100.0
		} else {
			concentrationScore = 0.0 // Only one domain, highly concentrated
		}
	}

	score := HealthScore{
		CoverageScore:      coverageScore,
		ConcentrationScore: concentrationScore,
		AgingScore:         50.0, // placeholder
		QualityScore:       50.0, // placeholder
		OverallScore:       (coverageScore*0.4 + concentrationScore*0.3 + 50.0*0.2 + 50.0*0.1),
		EvaluatedAt:        time.Now().UTC(),
	}

	if err := p.SetHealthScore(score); err != nil {
		return nil, err
	}

	if err := s.repo.Save(ctx, p); err != nil {
		return nil, err
	}

	return &score, nil
}

func (s *portfolioServiceImpl) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*PortfolioComparison, error) {
	if len(portfolioIDs) > 10 {
		return nil, errors.InvalidParam("cannot compare more than 10 portfolios")
	}

	results := make([]*PortfolioComparison, 0, len(portfolioIDs))
	for _, id := range portfolioIDs {
		p, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}

		// Mock tech domain coverage: distribute patents round-robin
		coverage := make(map[string]int)
		for i, domain := range p.TechDomains {
			count := p.PatentCount() / len(p.TechDomains)
			if i < p.PatentCount()%len(p.TechDomains) {
				count++
			}
			coverage[domain] = count
		}

		results = append(results, &PortfolioComparison{
			PortfolioID:        p.ID,
			Name:               p.Name,
			PatentCount:        p.PatentCount(),
			TechDomainCoverage: coverage,
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

	industryAverage := 10
	gaps := make([]*GapInfo, 0)

	// Build current coverage map
	currentCoverage := make(map[string]int)
	for _, domain := range p.TechDomains {
		// Mock: distribute patents
		currentCoverage[domain] = p.PatentCount() / len(p.TechDomains)
	}

	for _, target := range targetDomains {
		count := currentCoverage[target]
		if count < industryAverage {
			severity := "Low"
			if count == 0 {
				severity = "Critical"
			} else if count < industryAverage/2 {
				severity = "High"
			} else if count < industryAverage {
				severity = "Medium"
			}

			gaps = append(gaps, &GapInfo{
				TechDomain:      target,
				DomainName:      target, // Simplification
				CurrentCount:    count,
				IndustryAverage: industryAverage,
				GapSeverity:     severity,
				Recommendation:  fmt.Sprintf("Consider acquiring or developing more patents in %s area", target),
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
	p2, err := s.repo.FindByID(ctx, portfolioID2)
	if err != nil {
		return nil, err
	}

	map1 := make(map[string]bool)
	for _, id := range p1.PatentIDs {
		map1[id] = true
	}

	overlapping := make([]string, 0)
	uniqueToP1 := make([]string, 0)
	uniqueToP2 := make([]string, 0)

	for _, id := range p1.PatentIDs {
		found := false
		for _, id2 := range p2.PatentIDs {
			if id == id2 {
				found = true
				break
			}
		}
		if !found {
			uniqueToP1 = append(uniqueToP1, id)
		}
	}

	for _, id := range p2.PatentIDs {
		if map1[id] {
			overlapping = append(overlapping, id)
		} else {
			uniqueToP2 = append(uniqueToP2, id)
		}
	}

	var ratio float64
	totalUnique := len(uniqueToP1) + len(uniqueToP2) + len(overlapping)
	if totalUnique > 0 {
		ratio = float64(len(overlapping)) / float64(totalUnique)
	}

	return &OverlapResult{
		Portfolio1ID:        portfolioID1,
		Portfolio2ID:        portfolioID2,
		OverlappingPatentIDs: overlapping,
		OverlapRatio:        ratio,
		UniqueToPortfolio1:  uniqueToP1,
		UniqueToPortfolio2:  uniqueToP2,
	}, nil
}

//Personal.AI order the ending
