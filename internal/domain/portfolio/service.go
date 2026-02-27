package portfolio

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// PortfolioComparison provides a comparison view of multiple portfolios.
type PortfolioComparison struct {
	PortfolioID        string         `json:"portfolio_id"`
	Name               string         `json:"name"`
	PatentCount        int            `json:"patent_count"`
	TechDomainCoverage map[string]int `json:"tech_domain_coverage"`
	HealthScore        *HealthScore   `json:"health_score"`
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
	Portfolio1ID         string   `json:"portfolio1_id"`
	Portfolio2ID         string   `json:"portfolio2_id"`
	OverlappingPatentIDs []string `json:"overlapping_patent_ids"`
	OverlapRatio         float64  `json:"overlap_ratio"`
	UniqueToPortfolio1   []string `json:"unique_to_portfolio1"`
	UniqueToPortfolio2   []string `json:"unique_to_portfolio2"`
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

// Service is an alias for PortfolioService for backward compatibility.
type Service = PortfolioService

// PortfolioDomainService is an alias for PortfolioService.
type PortfolioDomainService = PortfolioService

// ValuationDomainService defines the valuation domain service interface.
type ValuationDomainService interface {
	CalculateHealthScore(ctx context.Context, portfolioID string) (*HealthScore, error)
}

type portfolioServiceImpl struct {
	repo PortfolioRepository
}

// NewPortfolioService creates a new PortfolioService.
func NewPortfolioService(repo PortfolioRepository) PortfolioService {
	return &portfolioServiceImpl{repo: repo}
}

func (s *portfolioServiceImpl) CreatePortfolio(ctx context.Context, name, ownerID string, techDomains []string) (*Portfolio, error) {
	p, err := NewPortfolio(name, ownerID, techDomains)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *portfolioServiceImpl) AddPatentsToPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error {
	if portfolioID == "" {
		return errors.NewValidation("invalid portfolio id")
	}

	var errs error
	for _, pidStr := range patentIDs {
		if pidStr == "" {
			continue
		}

		// Role defaults to core, addedBy needs context user, assuming empty or system for now
		if err := s.repo.AddPatent(ctx, portfolioID, pidStr, "core", ""); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to add patent %s: %w", pidStr, err))
		}
	}

	// Update timestamp
	p, err := s.repo.GetByID(ctx, portfolioID)
	if err == nil {
		p.UpdatedAt = time.Time(common.NewTimestamp())
		_ = s.repo.Update(ctx, p)
	}

	return errs
}

func (s *portfolioServiceImpl) RemovePatentsFromPortfolio(ctx context.Context, portfolioID string, patentIDs []string) error {
	if portfolioID == "" {
		return errors.NewValidation("invalid portfolio id")
	}

	for _, pidStr := range patentIDs {
		if pidStr != "" {
			_ = s.repo.RemovePatent(ctx, portfolioID, pidStr)
		}
	}
	return nil
}

func (s *portfolioServiceImpl) ActivatePortfolio(ctx context.Context, portfolioID string) error {
	if portfolioID == "" {
		return errors.NewValidation("invalid portfolio id")
	}
	p, err := s.repo.GetByID(ctx, portfolioID)
	if err != nil {
		return err
	}

	if err := p.Activate(); err != nil {
		return err
	}
	return s.repo.Update(ctx, p)
}

func (s *portfolioServiceImpl) ArchivePortfolio(ctx context.Context, portfolioID string) error {
	if portfolioID == "" {
		return errors.NewValidation("invalid portfolio id")
	}
	p, err := s.repo.GetByID(ctx, portfolioID)
	if err != nil {
		return err
	}

	if err := p.Archive(); err != nil {
		return err
	}
	return s.repo.Update(ctx, p)
}

func (s *portfolioServiceImpl) CalculateHealthScore(ctx context.Context, portfolioID string) (*HealthScore, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("invalid portfolio id")
	}

	p, err := s.repo.GetByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	patentCount := float64(p.PatentCount)
	coverageScore := math.Min(patentCount/10.0*100.0, 100.0)

	concentrationScore := 0.0
	numDomains := len(p.TechDomains)
	if numDomains > 0 && patentCount > 0 {
		p_i := 1.0 / float64(numDomains)
		entropy := -float64(numDomains) * (p_i * math.Log2(p_i))
		maxEntropy := math.Log2(float64(numDomains))
		if maxEntropy > 0 {
			concentrationScore = (entropy / maxEntropy) * 100.0
		} else {
			concentrationScore = 0.0
		}
	}

	score := &HealthScore{
		ID:             string(common.NewID()),
		PortfolioID:    p.ID,
		CoverageScore:  coverageScore,
		DiversityScore: concentrationScore,
		FreshnessScore: 50.0,
		StrengthScore:  50.0,
		RiskScore:      20.0,
		OverallScore:   (coverageScore*0.4 + concentrationScore*0.3 + 50.0*0.2 + 50.0*0.1),
		EvaluatedAt:    time.Time(common.NewTimestamp()),
		CreatedAt:      time.Time(common.NewTimestamp()),
	}

	if err := s.repo.CreateHealthScore(ctx, score); err != nil {
		return nil, err
	}

	return score, nil
}

func (s *portfolioServiceImpl) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*PortfolioComparison, error) {
	if len(portfolioIDs) > 10 {
		return nil, errors.NewValidation("cannot compare more than 10 portfolios")
	}

	results := make([]*PortfolioComparison, 0, len(portfolioIDs))
	for _, idStr := range portfolioIDs {
		if idStr == "" {
			continue
		}

		p, err := s.repo.GetByID(ctx, idStr)
		if err != nil {
			return nil, err
		}

		hs, _ := s.repo.GetLatestHealthScore(ctx, idStr)

		// Mock tech domain coverage
		coverage := make(map[string]int)
		for _, domain := range p.TechDomains {
			coverage[domain] = p.PatentCount / len(p.TechDomains) // Simplified
		}

		results = append(results, &PortfolioComparison{
			PortfolioID:        p.ID,
			Name:               p.Name,
			PatentCount:        p.PatentCount,
			TechDomainCoverage: coverage,
			HealthScore:        hs,
		})
	}

	return results, nil
}

func (s *portfolioServiceImpl) IdentifyGaps(ctx context.Context, portfolioID string, targetDomains []string) ([]*GapInfo, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("invalid portfolio id")
	}

	p, err := s.repo.GetByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	industryAverage := 10
	gaps := make([]*GapInfo, 0)

	// Build current coverage map
	currentCoverage := make(map[string]int)
	if len(p.TechDomains) > 0 {
		countPerDomain := p.PatentCount / len(p.TechDomains)
		for _, domain := range p.TechDomains {
			currentCoverage[domain] = countPerDomain
		}
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
				DomainName:      target,
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
	if portfolioID1 == "" || portfolioID2 == "" {
		return nil, errors.NewValidation("invalid portfolio ids")
	}

	// This requires fetching all patent IDs for both portfolios.
	// Repository method GetPatents returns []*patent.Patent
	patents1, _, err := s.repo.GetPatents(ctx, portfolioID1, nil, 10000, 0)
	if err != nil {
		return nil, err
	}
	patents2, _, err := s.repo.GetPatents(ctx, portfolioID2, nil, 10000, 0)
	if err != nil {
		return nil, err
	}

	map1 := make(map[string]bool)
	for _, p := range patents1 {
		map1[p.GetID()] = true
	}

	overlapping := make([]string, 0)
	uniqueToP1 := make([]string, 0)
	uniqueToP2 := make([]string, 0)

	for _, p := range patents1 {
		found := false
		for _, p2 := range patents2 {
			if p.GetID() == p2.GetID() {
				found = true
				break
			}
		}
		if !found {
			uniqueToP1 = append(uniqueToP1, p.GetID())
		}
	}

	for _, p := range patents2 {
		if map1[p.GetID()] {
			overlapping = append(overlapping, p.GetID())
		} else {
			uniqueToP2 = append(uniqueToP2, p.GetID())
		}
	}

	var ratio float64
	totalUnique := len(uniqueToP1) + len(uniqueToP2) + len(overlapping)
	if totalUnique > 0 {
		ratio = float64(len(overlapping)) / float64(totalUnique)
	}

	return &OverlapResult{
		Portfolio1ID:         portfolioID1,
		Portfolio2ID:         portfolioID2,
		OverlappingPatentIDs: overlapping,
		OverlapRatio:         ratio,
		UniqueToPortfolio1:   uniqueToP1,
		UniqueToPortfolio2:   uniqueToP2,
	}, nil
}

// NewService creates a new PortfolioService with a logger (ignores logger for backward compatibility).
func NewService(repo PortfolioRepository, logger interface{}) PortfolioService {
	return NewPortfolioService(repo)
}

//Personal.AI order the ending
