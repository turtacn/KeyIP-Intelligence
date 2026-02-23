// Phase 10 - File 226 of 349
package portfolio

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// -----------------------------------------------------------------------
// Request / Response DTOs
// -----------------------------------------------------------------------

// OptimizationRequest defines parameters for portfolio optimization.
type OptimizationRequest struct {
	PortfolioID    string              `json:"portfolio_id" validate:"required"`
	Objective      OptimizationGoal    `json:"objective"`
	Budget         float64             `json:"budget,omitempty"`
	Constraints    OptConstraints      `json:"constraints,omitempty"`
	Preferences    OptPreferences      `json:"preferences,omitempty"`
}

// OptimizationGoal enumerates optimization objectives.
type OptimizationGoal string

const (
	GoalMaxCoverage    OptimizationGoal = "maximize_coverage"
	GoalMinCost        OptimizationGoal = "minimize_cost"
	GoalMaxROI         OptimizationGoal = "maximize_roi"
	GoalBalanced       OptimizationGoal = "balanced"
)

// OptConstraints defines constraints for the optimization.
type OptConstraints struct {
	MinPatentCount    int      `json:"min_patent_count,omitempty"`
	MaxPatentCount    int      `json:"max_patent_count,omitempty"`
	RequiredDomains   []string `json:"required_domains,omitempty"`
	RequiredJurisd    []string `json:"required_jurisdictions,omitempty"`
	MaxAnnualCost     float64  `json:"max_annual_cost,omitempty"`
}

// OptPreferences defines soft preferences for the optimization.
type OptPreferences struct {
	PreferRecent       bool    `json:"prefer_recent"`
	PreferHighValue    bool    `json:"prefer_high_value"`
	DiversityWeight    float64 `json:"diversity_weight,omitempty"`
	CostSensitivity    float64 `json:"cost_sensitivity,omitempty"`
}

// OptimizationResponse contains the optimization result.
type OptimizationResponse struct {
	PortfolioID       string                `json:"portfolio_id"`
	Objective         OptimizationGoal      `json:"objective"`
	Recommendations   []PatentRecommendation `json:"recommendations"`
	RetainList        []string              `json:"retain_list"`
	PruneList         []PruneCandidate      `json:"prune_list"`
	ProjectedSavings  float64               `json:"projected_savings"`
	ProjectedCoverage float64               `json:"projected_coverage"`
	HealthDelta       float64               `json:"health_delta"`
	Summary           OptSummary            `json:"summary"`
	GeneratedAt       time.Time             `json:"generated_at"`
}

// PatentRecommendation represents a recommendation for a specific patent.
type PatentRecommendation struct {
	PatentID      string  `json:"patent_id"`
	PatentNumber  string  `json:"patent_number"`
	TechDomain    string  `json:"tech_domain"`
	Action        string  `json:"action"`
	Reason        string  `json:"reason"`
	ValueScore    float64 `json:"value_score"`
	CostEstimate  float64 `json:"cost_estimate"`
	Priority      int     `json:"priority"`
}

// PruneCandidate represents a patent recommended for pruning (abandonment).
type PruneCandidate struct {
	PatentID       string  `json:"patent_id"`
	PatentNumber   string  `json:"patent_number"`
	TechDomain     string  `json:"tech_domain"`
	AnnualCost     float64 `json:"annual_cost"`
	ValueScore     float64 `json:"value_score"`
	Redundancy     float64 `json:"redundancy_score"`
	PruneScore     float64 `json:"prune_score"`
	Reason         string  `json:"reason"`
}

// OptSummary provides aggregate statistics about the optimization.
type OptSummary struct {
	TotalPatents       int     `json:"total_patents"`
	RetainCount        int     `json:"retain_count"`
	PruneCount         int     `json:"prune_count"`
	NewFilingCount     int     `json:"new_filing_count"`
	EstimatedSavings   float64 `json:"estimated_savings"`
	CoverageChange     float64 `json:"coverage_change_pct"`
	ROIImprovement     float64 `json:"roi_improvement_pct"`
}

// -----------------------------------------------------------------------
// Service Interface
// -----------------------------------------------------------------------

// OptimizationService defines the application-level interface for portfolio optimization.
type OptimizationService interface {
	Optimize(ctx context.Context, req *OptimizationRequest) (*OptimizationResponse, error)
	GetPruneCandidates(ctx context.Context, portfolioID string, limit int) ([]PruneCandidate, error)
	EstimateCost(ctx context.Context, portfolioID string) (*CostEstimate, error)
}

// CostEstimate contains cost breakdown for a portfolio.
type CostEstimate struct {
	PortfolioID     string             `json:"portfolio_id"`
	TotalAnnualCost float64            `json:"total_annual_cost"`
	ByDomain        map[string]float64 `json:"by_domain"`
	ByJurisdiction  map[string]float64 `json:"by_jurisdiction"`
	TopCostPatents  []PatentCostEntry  `json:"top_cost_patents"`
	GeneratedAt     time.Time          `json:"generated_at"`
}

// PatentCostEntry represents cost information for a single patent.
type PatentCostEntry struct {
	PatentID     string  `json:"patent_id"`
	PatentNumber string  `json:"patent_number"`
	AnnualCost   float64 `json:"annual_cost"`
	TechDomain   string  `json:"tech_domain"`
}

// -----------------------------------------------------------------------
// Service Implementation
// -----------------------------------------------------------------------

type optimizationServiceImpl struct {
	portfolioSvc  domainportfolio.Service
	portfolioRepo domainportfolio.PortfolioRepository
	patentRepo    domainpatent.Repository
	logger        logging.Logger
}

// OptimizationServiceConfig holds configuration for constructing the optimization service.
type OptimizationServiceConfig struct {
	PortfolioService    domainportfolio.Service
	PortfolioRepository domainportfolio.PortfolioRepository
	PatentRepository    domainpatent.Repository
	Logger              logging.Logger
}

// NewOptimizationService constructs an OptimizationService.
func NewOptimizationService(cfg OptimizationServiceConfig) (OptimizationService, error) {
	if cfg.PortfolioService == nil {
		return nil, errors.NewValidation("OptimizationService requires PortfolioService")
	}
	if cfg.PatentRepository == nil {
		return nil, errors.NewValidation("OptimizationService requires PatentRepository")
	}
	if cfg.Logger == nil {
		return nil, errors.NewValidation("OptimizationService requires Logger")
	}
	return &optimizationServiceImpl{
		portfolioSvc:  cfg.PortfolioService,
		portfolioRepo: cfg.PortfolioRepository,
		patentRepo:    cfg.PatentRepository,
		logger:        cfg.Logger,
	}, nil
}

// Optimize performs portfolio optimization based on the given objective.
func (s *optimizationServiceImpl) Optimize(ctx context.Context, req *OptimizationRequest) (*OptimizationResponse, error) {
	if req == nil {
		return nil, errors.NewValidation("optimization request must not be nil")
	}
	if req.PortfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}

	objective := req.Objective
	if objective == "" {
		objective = GoalBalanced
	}

	s.logger.Info("starting portfolio optimization",
		logging.String("portfolio_id", req.PortfolioID),
		logging.String("objective", string(objective)))

	portfolioUUID, err := uuid.Parse(req.PortfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeValidation, "invalid portfolio ID")
	}

	portfolio, err := s.portfolioRepo.GetByID(ctx, portfolioUUID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to load portfolio")
	}
	if portfolio == nil {
		return nil, errors.ErrNotFound("portfolio", req.PortfolioID)
	}

	patents, err := s.patentRepo.ListByPortfolio(ctx, req.PortfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to load patents")
	}

	// Convert []*Patent to []Patent
	patentValues := make([]domainpatent.Patent, len(patents))
	for i, p := range patents {
		if p != nil {
			patentValues[i] = *p
		}
	}

	// Score each patent.
	scored := s.scorePatents(patentValues, objective, req.Preferences)

	// Identify prune candidates.
	pruneCandidates := s.identifyPruneCandidates(scored, req.Constraints)

	// Build retain list.
	pruneSet := make(map[string]bool)
	for _, pc := range pruneCandidates {
		pruneSet[pc.PatentID] = true
	}

	retainList := make([]string, 0, len(patents))
	recommendations := make([]PatentRecommendation, 0, len(patents))
	priority := 1

	for _, sp := range scored {
		if pruneSet[sp.patent.GetID()] {
			recommendations = append(recommendations, PatentRecommendation{
				PatentID:     sp.patent.GetID(),
				PatentNumber: sp.patent.GetPatentNumber(),
				TechDomain:   sp.patent.GetPrimaryTechDomain(),
				Action:       "prune",
				Reason:       sp.pruneReason,
				ValueScore:   sp.valueScore,
				CostEstimate: sp.annualCost,
				Priority:     priority,
			})
			priority++
		} else {
			retainList = append(retainList, sp.patent.GetID())
			recommendations = append(recommendations, PatentRecommendation{
				PatentID:     sp.patent.GetID(),
				PatentNumber: sp.patent.GetPatentNumber(),
				TechDomain:   sp.patent.GetPrimaryTechDomain(),
				Action:       "retain",
				Reason:       "Meets portfolio objectives",
				ValueScore:   sp.valueScore,
				CostEstimate: sp.annualCost,
				Priority:     priority,
			})
			priority++
		}
	}

	// Calculate projected savings.
	totalSavings := 0.0
	for _, pc := range pruneCandidates {
		totalSavings += pc.AnnualCost
	}

	// Calculate projected coverage.
	totalDomains := s.countUniqueDomains(patentValues)
	retainDomains := 0
	for _, sp := range scored {
		if !pruneSet[sp.patent.GetID()] {
			retainDomains++
		}
	}
	projectedCoverage := 1.0
	if totalDomains > 0 {
		retainDomainCount := s.countUniqueDomainsFromScored(scored, pruneSet)
		projectedCoverage = float64(retainDomainCount) / float64(totalDomains)
	}

	summary := OptSummary{
		TotalPatents:     len(patents),
		RetainCount:      len(retainList),
		PruneCount:       len(pruneCandidates),
		EstimatedSavings: totalSavings,
		CoverageChange:   (projectedCoverage - 1.0) * 100,
	}

	response := &OptimizationResponse{
		PortfolioID:       req.PortfolioID,
		Objective:         objective,
		Recommendations:   recommendations,
		RetainList:        retainList,
		PruneList:         pruneCandidates,
		ProjectedSavings:  totalSavings,
		ProjectedCoverage: projectedCoverage,
		Summary:           summary,
		GeneratedAt:       time.Now().UTC(),
	}

	s.logger.Info("optimization completed",
		logging.String("portfolio_id", req.PortfolioID),
		logging.Int("retain", len(retainList)),
		logging.Int("prune", len(pruneCandidates)),
		logging.Float64("savings", totalSavings))

	return response, nil
}

// GetPruneCandidates returns patents recommended for pruning.
func (s *optimizationServiceImpl) GetPruneCandidates(ctx context.Context, portfolioID string, limit int) ([]PruneCandidate, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}
	if limit <= 0 {
		limit = 10
	}

	resp, err := s.Optimize(ctx, &OptimizationRequest{
		PortfolioID: portfolioID,
		Objective:   GoalMinCost,
	})
	if err != nil {
		return nil, err
	}

	candidates := resp.PruneList
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates, nil
}

// EstimateCost calculates the cost breakdown for a portfolio.
func (s *optimizationServiceImpl) EstimateCost(ctx context.Context, portfolioID string) (*CostEstimate, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}

	portfolioUUID, err := uuid.Parse(portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeValidation, "invalid portfolio ID")
	}

	portfolio, err := s.portfolioRepo.GetByID(ctx, portfolioUUID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to load portfolio")
	}
	if portfolio == nil {
		return nil, errors.ErrNotFound("portfolio", portfolioID)
	}

	patents, err := s.patentRepo.ListByPortfolio(ctx, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to load patents")
	}

	totalCost := 0.0
	byDomain := make(map[string]float64)
	byJurisd := make(map[string]float64)
	entries := make([]PatentCostEntry, 0, len(patents))

	for _, p := range patents {
		cost := estimatePatentAnnualCost(*p)
		totalCost += cost

		domain := p.GetPrimaryTechDomain()
		if domain == "" {
			domain = "unclassified"
		}
		byDomain[domain] += cost

		juris := extractJurisdiction(p.GetPatentNumber())
		if juris == "" {
			juris = "unknown"
		}
		byJurisd[juris] += cost

		entries = append(entries, PatentCostEntry{
			PatentID:     p.GetID(),
			PatentNumber: p.GetPatentNumber(),
			AnnualCost:   cost,
			TechDomain:   domain,
		})
	}

	// Sort entries by cost descending.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AnnualCost > entries[j].AnnualCost
	})

	topN := 10
	if len(entries) < topN {
		topN = len(entries)
	}

	return &CostEstimate{
		PortfolioID:     portfolioID,
		TotalAnnualCost: totalCost,
		ByDomain:        byDomain,
		ByJurisdiction:  byJurisd,
		TopCostPatents:  entries[:topN],
		GeneratedAt:     time.Now().UTC(),
	}, nil
}

// -----------------------------------------------------------------------
// Internal Scoring and Analysis
// -----------------------------------------------------------------------

type scoredPatent struct {
	patent      domainpatent.Patent
	valueScore  float64
	annualCost  float64
	redundancy  float64
	recency     float64
	pruneScore  float64
	pruneReason string
}

// scorePatents evaluates each patent against the optimization objective.
func (s *optimizationServiceImpl) scorePatents(
	patents []domainpatent.Patent,
	objective OptimizationGoal,
	prefs OptPreferences,
) []scoredPatent {
	now := time.Now()

	// Build domain frequency map for redundancy calculation.
	domainFreq := make(map[string]int)
	for _, p := range patents {
		d := p.GetPrimaryTechDomain()
		if d != "" {
			domainFreq[d]++
		}
	}

	scored := make([]scoredPatent, 0, len(patents))

	for _, p := range patents {
		sp := scoredPatent{
			patent:     p,
			valueScore: p.GetValueScore(),
			annualCost: estimatePatentAnnualCost(p),
		}

		// Recency score [0, 1]: newer patents score higher.
		filingDate := p.GetFilingDate()
		if filingDate != nil && !filingDate.IsZero() {
			ageYears := now.Sub(*filingDate).Hours() / (24 * 365.25)
			sp.recency = math.Max(0, 1.0-ageYears/20.0)
		}

		// Redundancy score [0, 1]: higher if many patents in same domain.
		domain := p.GetPrimaryTechDomain()
		if freq, ok := domainFreq[domain]; ok && freq > 1 {
			sp.redundancy = 1.0 - 1.0/float64(freq)
		}

		// Compute prune score based on objective.
		switch objective {
		case GoalMinCost:
			// High cost + low value + high redundancy = good prune candidate.
			costNorm := math.Min(sp.annualCost/10000.0, 1.0)
			valueNorm := sp.valueScore / 10.0
			sp.pruneScore = 0.4*costNorm + 0.3*(1.0-valueNorm) + 0.3*sp.redundancy
		case GoalMaxCoverage:
			// Only prune highly redundant, low-value patents.
			valueNorm := sp.valueScore / 10.0
			sp.pruneScore = 0.6*sp.redundancy + 0.4*(1.0-valueNorm)
		case GoalMaxROI:
			// Prune low ROI patents (high cost, low value).
			roi := 0.0
			if sp.annualCost > 0 {
				roi = sp.valueScore / sp.annualCost
			}
			roiNorm := math.Min(roi, 1.0)
			sp.pruneScore = 1.0 - roiNorm
		default: // GoalBalanced
			costNorm := math.Min(sp.annualCost/10000.0, 1.0)
			valueNorm := sp.valueScore / 10.0
			sp.pruneScore = 0.3*costNorm + 0.3*(1.0-valueNorm) + 0.2*sp.redundancy + 0.2*(1.0-sp.recency)
		}

		// Apply preferences.
		if prefs.PreferRecent && sp.recency < 0.3 {
			sp.pruneScore += 0.1
		}
		if prefs.PreferHighValue && sp.valueScore < 3.0 {
			sp.pruneScore += 0.1
		}

		// Clamp.
		if sp.pruneScore > 1.0 {
			sp.pruneScore = 1.0
		}

		scored = append(scored, sp)
	}

	// Sort by prune score descending (best prune candidates first).
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].pruneScore > scored[j].pruneScore
	})

	return scored
}

// identifyPruneCandidates selects patents to prune based on scores and constraints.
func (s *optimizationServiceImpl) identifyPruneCandidates(scored []scoredPatent, constraints OptConstraints) []PruneCandidate {
	pruneThreshold := 0.5
	candidates := make([]PruneCandidate, 0)

	requiredDomains := toStringSet(constraints.RequiredDomains)

	// Track how many patents remain per domain.
	domainRemaining := make(map[string]int)
	for _, sp := range scored {
		d := sp.patent.GetPrimaryTechDomain()
		if d != "" {
			domainRemaining[d]++
		}
	}

	retainCount := len(scored)

	for _, sp := range scored {
		if sp.pruneScore < pruneThreshold {
			break
		}

		// Respect min patent count constraint.
		if constraints.MinPatentCount > 0 && retainCount <= constraints.MinPatentCount {
			break
		}

		domain := sp.patent.GetPrimaryTechDomain()

		// Don't prune if it would eliminate a required domain.
		if requiredDomains != nil {
			if _, required := requiredDomains[domain]; required {
				if domainRemaining[domain] <= 1 {
					continue
				}
			}
		}

		// Don't prune if it would leave a domain with zero coverage.
		if domainRemaining[domain] <= 1 {
			sp.pruneReason = "Low value but sole domain representative — retained"
			continue
		}

		reason := "Low strategic value relative to cost"
		if sp.redundancy > 0.5 {
			reason = fmt.Sprintf("High redundancy (%.0f%%) in domain %s", sp.redundancy*100, domain)
		}

		candidates = append(candidates, PruneCandidate{
			PatentID:     sp.patent.GetID(),
			PatentNumber: sp.patent.GetPatentNumber(),
			TechDomain:   domain,
			AnnualCost:   sp.annualCost,
			ValueScore:   sp.valueScore,
			Redundancy:   sp.redundancy,
			PruneScore:   sp.pruneScore,
			Reason:       reason,
		})

		domainRemaining[domain]--
		retainCount--
	}

	return candidates
}

// countUniqueDomains counts unique tech domains across patents.
func (s *optimizationServiceImpl) countUniqueDomains(patents []domainpatent.Patent) int {
	seen := make(map[string]struct{})
	for _, p := range patents {
		d := p.GetPrimaryTechDomain()
		if d != "" {
			seen[d] = struct{}{}
		}
	}
	return len(seen)
}

// countUniqueDomainsFromScored counts unique domains from retained patents.
func (s *optimizationServiceImpl) countUniqueDomainsFromScored(scored []scoredPatent, pruneSet map[string]bool) int {
	seen := make(map[string]struct{})
	for _, sp := range scored {
		if pruneSet[sp.patent.GetID()] {
			continue
		}
		d := sp.patent.GetPrimaryTechDomain()
		if d != "" {
			seen[d] = struct{}{}
		}
	}
	return len(seen)
}

// estimatePatentAnnualCost estimates the annual maintenance cost for a patent.
func estimatePatentAnnualCost(p domainpatent.Patent) float64 {
	baseCost := 2000.0

	juris := extractJurisdiction(p.GetPatentNumber())
	multipliers := map[string]float64{
		"US": 1.0, "EP": 1.8, "CN": 0.6, "JP": 1.5,
		"KR": 0.8, "IN": 0.4, "CA": 0.9, "AU": 0.7,
	}
	mult := 1.0
	if m, ok := multipliers[juris]; ok {
		mult = m
	}

	// Age-based escalation: maintenance fees increase over time.
	ageFactor := 1.0
	filingDate := p.GetFilingDate()
	if filingDate != nil && !filingDate.IsZero() {
		ageYears := time.Since(*filingDate).Hours() / (24 * 365.25)
		if ageYears > 4 {
			ageFactor = 1.0 + (ageYears-4)*0.05
		}
		if ageFactor > 3.0 {
			ageFactor = 3.0
		}
	}

	return baseCost * mult * ageFactor
}

//Personal.AI order the ending
