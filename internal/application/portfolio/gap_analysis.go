// Phase 10 - File 224 of 349
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

// GapAnalysisRequest defines parameters for a comprehensive gap analysis.
type GapAnalysisRequest struct {
	PortfolioID      string   `json:"portfolio_id" validate:"required"`
	CompetitorNames  []string `json:"competitor_names,omitempty"`
	TechDomains      []string `json:"tech_domains,omitempty"`
	Jurisdictions    []string `json:"jurisdictions,omitempty"`
	ExpirationWindow int      `json:"expiration_window_years,omitempty"`
}

// GapAnalysisResponse contains the full gap analysis result.
type GapAnalysisResponse struct {
	PortfolioID      string              `json:"portfolio_id"`
	TechGaps         []TechnologyGap     `json:"technology_gaps"`
	ExpirationRisks  []ExpirationRisk    `json:"expiration_risks"`
	GeographicGaps   []GeographicGap     `json:"geographic_gaps"`
	Opportunities    []FilingOpportunity `json:"filing_opportunities"`
	OverallScore     float64             `json:"overall_health_score"`
	Summary          GapSummary          `json:"summary"`
	GeneratedAt      time.Time           `json:"generated_at"`
}

// TechnologyGap represents a gap in technology domain coverage.
type TechnologyGap struct {
	GapID            string   `json:"gap_id"`
	TechDomain       string   `json:"tech_domain"`
	DomainName       string   `json:"domain_name"`
	CompetitorCount  int      `json:"competitor_patent_count"`
	OwnCount         int      `json:"own_patent_count"`
	GapSeverity      float64  `json:"gap_severity"`
	CompetitorNames  []string `json:"competitors_present"`
	Recommendation   string   `json:"recommendation"`
}

// ExpirationRisk represents a patent nearing expiration that may create a coverage hole.
type ExpirationRisk struct {
	PatentID       string    `json:"patent_id"`
	PatentNumber   string    `json:"patent_number"`
	TechDomain     string    `json:"tech_domain"`
	ExpirationDate time.Time `json:"expiration_date"`
	DaysRemaining  int       `json:"days_remaining"`
	RiskLevel      RiskLevel `json:"risk_level"`
	CoverageImpact float64   `json:"coverage_impact"`
	HasReplacement bool      `json:"has_replacement"`
}

// RiskLevel enumerates expiration risk severity.
type RiskLevel string

const (
	RiskCritical RiskLevel = "critical"
	RiskHigh     RiskLevel = "high"
	RiskMedium   RiskLevel = "medium"
	RiskLow      RiskLevel = "low"
)

// GeographicGap represents a jurisdiction where patent protection is missing.
type GeographicGap struct {
	GapID          string  `json:"gap_id"`
	Jurisdiction   string  `json:"jurisdiction"`
	JurisdName     string  `json:"jurisdiction_name"`
	TechDomain     string  `json:"tech_domain,omitempty"`
	MarketSize     float64 `json:"market_size_score"`
	CompPresence   int     `json:"competitor_presence_count"`
	Priority       float64 `json:"priority_score"`
}

// FilingOpportunity represents a recommended patent filing action.
type FilingOpportunity struct {
	OpportunityID   string  `json:"opportunity_id"`
	Type            string  `json:"type"`
	TechDomain      string  `json:"tech_domain"`
	Jurisdiction    string  `json:"jurisdiction,omitempty"`
	Description     string  `json:"description"`
	StrategicValue  float64 `json:"strategic_value"`
	CompPressure    float64 `json:"competitive_pressure"`
	Feasibility     float64 `json:"feasibility"`
	OverallScore    float64 `json:"overall_score"`
	Urgency         string  `json:"urgency"`
}

// GapSummary provides aggregate statistics about the gap analysis.
type GapSummary struct {
	TotalTechGaps       int     `json:"total_tech_gaps"`
	CriticalGaps        int     `json:"critical_gaps"`
	ExpiringPatents     int     `json:"expiring_patents"`
	GeographicGapCount  int     `json:"geographic_gap_count"`
	OpportunityCount    int     `json:"opportunity_count"`
	HealthScore         float64 `json:"health_score"`
	TopRecommendation   string  `json:"top_recommendation"`
}

// -----------------------------------------------------------------------
// Service Interface
// -----------------------------------------------------------------------

// GapAnalysisService defines the application-level interface for portfolio gap analysis.
type GapAnalysisService interface {
	// AnalyzeGaps performs a comprehensive gap analysis on a patent portfolio.
	AnalyzeGaps(ctx context.Context, req *GapAnalysisRequest) (*GapAnalysisResponse, error)

	// GetFilingOpportunities returns scored filing opportunities for a portfolio.
	GetFilingOpportunities(ctx context.Context, portfolioID string, limit int) ([]FilingOpportunity, error)

	// GetExpirationRisks returns patents at risk of expiration within a given window.
	GetExpirationRisks(ctx context.Context, portfolioID string, windowYears int) ([]ExpirationRisk, error)

	// GetGeographicGaps identifies jurisdictions where patent protection is missing.
	GetGeographicGaps(ctx context.Context, portfolioID string, targetJurisdictions []string) ([]GeographicGap, error)
}

// -----------------------------------------------------------------------
// Service Implementation
// -----------------------------------------------------------------------

type gapAnalysisServiceImpl struct {
	portfolioSvc    domainportfolio.Service
	portfolioRepo   domainportfolio.PortfolioRepository
	patentRepo      domainpatent.Repository
	logger          logging.Logger
	cache           ConstellationCache
	cacheTTL        time.Duration
}

// GapAnalysisServiceConfig holds configuration for constructing the gap analysis service.
type GapAnalysisServiceConfig struct {
	PortfolioService    domainportfolio.Service
	PortfolioRepository domainportfolio.PortfolioRepository
	PatentRepository    domainpatent.Repository
	Logger              logging.Logger
	Cache               ConstellationCache
	CacheTTL            time.Duration
}

// NewGapAnalysisService constructs a GapAnalysisService with all required dependencies.
func NewGapAnalysisService(cfg GapAnalysisServiceConfig) (GapAnalysisService, error) {
	if cfg.PortfolioService == nil {
		return nil, errors.NewValidation("GapAnalysisService requires PortfolioService")
	}
	if cfg.PatentRepository == nil {
		return nil, errors.NewValidation("GapAnalysisService requires PatentRepository")
	}
	if cfg.Logger == nil {
		return nil, errors.NewValidation("GapAnalysisService requires Logger")
	}

	ttl := cfg.CacheTTL
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	return &gapAnalysisServiceImpl{
		portfolioSvc:  cfg.PortfolioService,
		portfolioRepo: cfg.PortfolioRepository,
		patentRepo:    cfg.PatentRepository,
		logger:        cfg.Logger,
		cache:         cfg.Cache,
		cacheTTL:      ttl,
	}, nil
}

// AnalyzeGaps performs a comprehensive gap analysis on a patent portfolio.
func (s *gapAnalysisServiceImpl) AnalyzeGaps(ctx context.Context, req *GapAnalysisRequest) (*GapAnalysisResponse, error) {
	if req == nil {
		return nil, errors.NewValidation("gap analysis request must not be nil")
	}
	if req.PortfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}

	s.logger.Info("starting gap analysis", logging.String("portfolio_id", req.PortfolioID))

	// Validate portfolio exists.
	portfolioUUID, err := uuid.Parse(req.PortfolioID)
	if err != nil {
		return nil, errors.NewInvalidParameterError("invalid portfolio ID format")
	}
	portfolio, err := s.portfolioRepo.GetByID(ctx, portfolioUUID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeNotFound, "failed to load portfolio")
	}
	if portfolio == nil {
		return nil, errors.NewNotFound("portfolio", req.PortfolioID)
	}

	// Load own patents.
	ownPatentPtrs, err := s.patentRepo.ListByPortfolio(ctx, req.PortfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to load portfolio patents")
	}
	
	// Convert to value types
	ownPatents := make([]domainpatent.Patent, len(ownPatentPtrs))
	for i, p := range ownPatentPtrs {
		ownPatents[i] = *p
	}

	// Load competitor patents.
	// TODO: Implement competitor loading when repository methods are available
	competitorPatents := make(map[string][]domainpatent.Patent)

	// Step 1: Identify technology gaps.
	techGaps := s.identifyTechGaps(ownPatents, competitorPatents, req.TechDomains)

	// Step 2: Identify expiration risks.
	windowYears := req.ExpirationWindow
	if windowYears <= 0 {
		windowYears = 5
	}
	expirationRisks := s.identifyExpirationRisks(ownPatents, windowYears)

	// Step 3: Identify geographic gaps.
	geoGaps := s.identifyGeographicGaps(ownPatents, competitorPatents, req.Jurisdictions)

	// Step 4: Generate filing opportunities.
	opportunities := s.generateOpportunities(techGaps, geoGaps, expirationRisks)

	// Step 5: Compute overall health score.
	healthScore := s.computeHealthScore(ownPatents, techGaps, expirationRisks, geoGaps)

	// Build summary.
	criticalCount := 0
	for _, g := range techGaps {
		if g.GapSeverity >= 0.8 {
			criticalCount++
		}
	}

	topRec := "Portfolio is in good health."
	if len(opportunities) > 0 {
		topRec = opportunities[0].Description
	}

	summary := GapSummary{
		TotalTechGaps:      len(techGaps),
		CriticalGaps:       criticalCount,
		ExpiringPatents:    len(expirationRisks),
		GeographicGapCount: len(geoGaps),
		OpportunityCount:   len(opportunities),
		HealthScore:        healthScore,
		TopRecommendation:  topRec,
	}

	response := &GapAnalysisResponse{
		PortfolioID:     req.PortfolioID,
		TechGaps:        techGaps,
		ExpirationRisks: expirationRisks,
		GeographicGaps:  geoGaps,
		Opportunities:   opportunities,
		OverallScore:    healthScore,
		Summary:         summary,
		GeneratedAt:     time.Now().UTC(),
	}

	s.logger.Info("gap analysis completed",
		logging.String("portfolio_id", req.PortfolioID),
		logging.Int("tech_gaps", len(techGaps)),
		logging.Int("expiration_risks", len(expirationRisks)),
		logging.Int("geo_gaps", len(geoGaps)),
		logging.Int("opportunities", len(opportunities)),
		logging.Float64("health_score", healthScore),
	)

	return response, nil
}

// GetFilingOpportunities returns scored filing opportunities for a portfolio.
func (s *gapAnalysisServiceImpl) GetFilingOpportunities(ctx context.Context, portfolioID string, limit int) ([]FilingOpportunity, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}
	if limit <= 0 {
		limit = 10
	}

	resp, err := s.AnalyzeGaps(ctx, &GapAnalysisRequest{
		PortfolioID:      portfolioID,
		ExpirationWindow: 5,
	})
	if err != nil {
		return nil, err
	}

	opps := resp.Opportunities
	if len(opps) > limit {
		opps = opps[:limit]
	}

	return opps, nil
}

// GetExpirationRisks returns patents at risk of expiration within a given window.
func (s *gapAnalysisServiceImpl) GetExpirationRisks(ctx context.Context, portfolioID string, windowYears int) ([]ExpirationRisk, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}
	if windowYears <= 0 {
		windowYears = 5
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
		return nil, errors.NewNotFound("portfolio", portfolioID)
	}

	patents, err := s.patentRepo.ListByPortfolio(ctx, portfolioID)
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

	return s.identifyExpirationRisks(patentValues, windowYears), nil
}

// GetGeographicGaps identifies jurisdictions where patent protection is missing.
func (s *gapAnalysisServiceImpl) GetGeographicGaps(ctx context.Context, portfolioID string, targetJurisdictions []string) ([]GeographicGap, error) {
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
		return nil, errors.NewNotFound("portfolio", portfolioID)
	}

	patents, err := s.patentRepo.ListByPortfolio(ctx, portfolioID)
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

	return s.identifyGeographicGaps(patentValues, nil, targetJurisdictions), nil
}

// -----------------------------------------------------------------------
// Internal Analysis Methods
// -----------------------------------------------------------------------

// identifyTechGaps finds technology domains where competitors have coverage but own portfolio does not.
func (s *gapAnalysisServiceImpl) identifyTechGaps(
	ownPatents []domainpatent.Patent,
	competitorPatents map[string][]domainpatent.Patent,
	filterDomains []string,
) []TechnologyGap {
	// Build own domain coverage.
	ownDomains := make(map[string]int)
	for _, p := range ownPatents {
		domain := p.GetPrimaryTechDomain()
		if domain != "" {
			ownDomains[domain]++
		}
	}

	// Build competitor domain coverage.
	type compDomainInfo struct {
		totalCount int
		companies  []string
	}
	compDomains := make(map[string]*compDomainInfo)
	for compName, patents := range competitorPatents {
		for _, p := range patents {
			domain := p.GetPrimaryTechDomain()
			if domain == "" {
				continue
			}
			info, exists := compDomains[domain]
			if !exists {
				info = &compDomainInfo{}
				compDomains[domain] = info
			}
			info.totalCount++
			// Track unique competitor names.
			found := false
			for _, c := range info.companies {
				if c == compName {
					found = true
					break
				}
			}
			if !found {
				info.companies = append(info.companies, compName)
			}
		}
	}

	// Apply domain filter if specified.
	domainFilter := toStringSet(filterDomains)

	gaps := make([]TechnologyGap, 0)
	gapIdx := 0

	// Find domains where competitors are present but own coverage is weak or absent.
	for domain, compInfo := range compDomains {
		if domainFilter != nil {
			if _, ok := domainFilter[domain]; !ok {
				continue
			}
		}

		ownCount := ownDomains[domain]
		compCount := compInfo.totalCount

		// Gap severity: higher when competitor count is high and own count is low.
		severity := 0.0
		if compCount > 0 {
			if ownCount == 0 {
				severity = 1.0
			} else {
				ratio := float64(compCount) / float64(ownCount)
				severity = 1.0 - 1.0/(1.0+ratio)
			}
		}

		// Only report as a gap if severity is meaningful.
		if severity < 0.3 {
			continue
		}

		rec := "Consider filing patents in this domain."
		if severity >= 0.8 {
			rec = "Critical gap: competitors have strong presence. Immediate filing recommended."
		} else if severity >= 0.5 {
			rec = "Moderate gap: consider strategic filings to strengthen position."
		}

		gaps = append(gaps, TechnologyGap{
			GapID:           fmt.Sprintf("tg-%d", gapIdx),
			TechDomain:      domain,
			DomainName:      resolveDomainName(domain),
			CompetitorCount: compCount,
			OwnCount:        ownCount,
			GapSeverity:     severity,
			CompetitorNames: compInfo.companies,
			Recommendation:  rec,
		})
		gapIdx++
	}

	// Sort by severity descending.
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].GapSeverity > gaps[j].GapSeverity
	})

	return gaps
}

// identifyExpirationRisks finds patents nearing expiration.
func (s *gapAnalysisServiceImpl) identifyExpirationRisks(patents []domainpatent.Patent, windowYears int) []ExpirationRisk {
	now := time.Now()
	windowEnd := now.AddDate(windowYears, 0, 0)

	// Standard patent term is 20 years from filing date.
	const patentTermYears = 20

	// Count patents per domain for coverage impact calculation.
	domainCounts := make(map[string]int)
	for _, p := range patents {
		domain := p.GetPrimaryTechDomain()
		if domain != "" {
			domainCounts[domain]++
		}
	}

	risks := make([]ExpirationRisk, 0)

	for _, p := range patents {
		filingDate := p.GetFilingDate()
		if filingDate == nil || filingDate.IsZero() {
			continue
		}

		expirationDate := filingDate.AddDate(patentTermYears, 0, 0)
		if expirationDate.Before(now) || expirationDate.After(windowEnd) {
			continue
		}

		daysRemaining := int(expirationDate.Sub(now).Hours() / 24)

		// Determine risk level.
		var riskLevel RiskLevel
		switch {
		case daysRemaining <= 365:
			riskLevel = RiskCritical
		case daysRemaining <= 730:
			riskLevel = RiskHigh
		case daysRemaining <= 1095:
			riskLevel = RiskMedium
		default:
			riskLevel = RiskLow
		}

		// Coverage impact: how much of the domain's coverage is lost.
		domain := p.GetPrimaryTechDomain()
		coverageImpact := 0.0
		if count, ok := domainCounts[domain]; ok && count > 0 {
			coverageImpact = 1.0 / float64(count)
		}

		// Check if there's a replacement (newer patent in same domain).
		hasReplacement := false
		for _, other := range patents {
			if other.GetID() == p.GetID() {
				continue
			}
			if other.GetPrimaryTechDomain() == domain {
				otherFiling := other.GetFilingDate()
				if otherFiling != nil && !otherFiling.IsZero() && filingDate != nil && otherFiling.After(*filingDate) {
					otherExpiration := otherFiling.AddDate(patentTermYears, 0, 0)
					if otherExpiration.After(expirationDate) {
						hasReplacement = true
						break
					}
				}
			}
		}

		risks = append(risks, ExpirationRisk{
			PatentID:       p.GetID(),
			PatentNumber:   p.GetPatentNumber(),
			TechDomain:     domain,
			ExpirationDate: expirationDate,
			DaysRemaining:  daysRemaining,
			RiskLevel:      riskLevel,
			CoverageImpact: coverageImpact,
			HasReplacement: hasReplacement,
		})
	}

	// Sort by days remaining ascending (most urgent first).
	sort.Slice(risks, func(i, j int) bool {
		return risks[i].DaysRemaining < risks[j].DaysRemaining
	})

	return risks
}

// identifyGeographicGaps finds jurisdictions where patent protection is missing.
func (s *gapAnalysisServiceImpl) identifyGeographicGaps(
	ownPatents []domainpatent.Patent,
	competitorPatents map[string][]domainpatent.Patent,
	targetJurisdictions []string,
) []GeographicGap {
	// Build own jurisdiction coverage.
	ownJurisdictions := make(map[string]bool)
	for _, p := range ownPatents {
		juris := extractJurisdiction(p.GetPatentNumber())
		if juris != "" {
			ownJurisdictions[juris] = true
		}
	}

	// Build competitor jurisdiction presence.
	compJurisCount := make(map[string]int)
	for _, patents := range competitorPatents {
		seen := make(map[string]bool)
		for _, p := range patents {
			juris := extractJurisdiction(p.GetPatentNumber())
			if juris != "" && !seen[juris] {
				seen[juris] = true
				compJurisCount[juris]++
			}
		}
	}

	// Default target jurisdictions if none specified.
	targets := targetJurisdictions
	if len(targets) == 0 {
		targets = defaultTargetJurisdictions()
	}

	gaps := make([]GeographicGap, 0)
	gapIdx := 0

	for _, juris := range targets {
		if ownJurisdictions[juris] {
			continue
		}

		marketSize := jurisdictionMarketScore(juris)
		compPresence := compJurisCount[juris]

		// Priority: weighted combination of market size and competitive presence.
		priority := 0.6*marketSize + 0.4*math.Min(float64(compPresence)/3.0, 1.0)

		gaps = append(gaps, GeographicGap{
			GapID:        fmt.Sprintf("gg-%d", gapIdx),
			Jurisdiction: juris,
			JurisdName:   jurisdictionName(juris),
			MarketSize:   marketSize,
			CompPresence: compPresence,
			Priority:     priority,
		})
		gapIdx++
	}

	// Sort by priority descending.
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].Priority > gaps[j].Priority
	})

	return gaps
}

// generateOpportunities synthesizes filing opportunities from identified gaps.
func (s *gapAnalysisServiceImpl) generateOpportunities(
	techGaps []TechnologyGap,
	geoGaps []GeographicGap,
	expirationRisks []ExpirationRisk,
) []FilingOpportunity {
	opps := make([]FilingOpportunity, 0)
	oppIdx := 0

	// Technology gap opportunities.
	for _, gap := range techGaps {
		strategicValue := gap.GapSeverity
		compPressure := math.Min(float64(gap.CompetitorCount)/10.0, 1.0)
		feasibility := 0.7 // Default moderate feasibility.
		if gap.OwnCount > 0 {
			feasibility = 0.85 // Easier if we already have some presence.
		}

		overall := 0.4*strategicValue + 0.35*compPressure + 0.25*feasibility

		urgency := "normal"
		if gap.GapSeverity >= 0.8 {
			urgency = "high"
		} else if gap.GapSeverity >= 0.6 {
			urgency = "medium"
		}

		opps = append(opps, FilingOpportunity{
			OpportunityID:  fmt.Sprintf("opp-tech-%d", oppIdx),
			Type:           "technology_gap",
			TechDomain:     gap.TechDomain,
			Description:    fmt.Sprintf("File patents in %s (%s) to close technology gap", gap.TechDomain, gap.DomainName),
			StrategicValue: strategicValue,
			CompPressure:   compPressure,
			Feasibility:    feasibility,
			OverallScore:   overall,
			Urgency:        urgency,
		})
		oppIdx++
	}

	// Geographic gap opportunities.
	for _, gap := range geoGaps {
		if gap.Priority < 0.3 {
			continue
		}

		strategicValue := gap.MarketSize
		compPressure := math.Min(float64(gap.CompPresence)/3.0, 1.0)
		feasibility := 0.6

		overall := 0.4*strategicValue + 0.35*compPressure + 0.25*feasibility

		urgency := "normal"
		if gap.Priority >= 0.7 {
			urgency = "high"
		}

		opps = append(opps, FilingOpportunity{
			OpportunityID:  fmt.Sprintf("opp-geo-%d", oppIdx),
			Type:           "geographic_gap",
			Jurisdiction:   gap.Jurisdiction,
			Description:    fmt.Sprintf("Extend patent protection to %s (%s)", gap.Jurisdiction, gap.JurisdName),
			StrategicValue: strategicValue,
			CompPressure:   compPressure,
			Feasibility:    feasibility,
			OverallScore:   overall,
			Urgency:        urgency,
		})
		oppIdx++
	}

	// Expiration replacement opportunities.
	for _, risk := range expirationRisks {
		if risk.HasReplacement || risk.RiskLevel == RiskLow {
			continue
		}

		strategicValue := risk.CoverageImpact
		compPressure := 0.5
		feasibility := 0.8

		overall := 0.4*strategicValue + 0.35*compPressure + 0.25*feasibility

		urgency := "normal"
		if risk.RiskLevel == RiskCritical {
			urgency = "critical"
		} else if risk.RiskLevel == RiskHigh {
			urgency = "high"
		}

		opps = append(opps, FilingOpportunity{
			OpportunityID:  fmt.Sprintf("opp-exp-%d", oppIdx),
			Type:           "expiration_replacement",
			TechDomain:     risk.TechDomain,
			Description:    fmt.Sprintf("File replacement for expiring patent %s in %s", risk.PatentNumber, risk.TechDomain),
			StrategicValue: strategicValue,
			CompPressure:   compPressure,
			Feasibility:    feasibility,
			OverallScore:   overall,
			Urgency:        urgency,
		})
		oppIdx++
	}

	// Sort by overall score descending.
	sort.Slice(opps, func(i, j int) bool {
		return opps[i].OverallScore > opps[j].OverallScore
	})

	return opps
}

// computeHealthScore calculates an overall portfolio health score [0, 100].
func (s *gapAnalysisServiceImpl) computeHealthScore(
	patents []domainpatent.Patent,
	techGaps []TechnologyGap,
	expirationRisks []ExpirationRisk,
	geoGaps []GeographicGap,
) float64 {
	if len(patents) == 0 {
		return 0.0
	}

	score := 100.0

	// Deduct for technology gaps.
	for _, g := range techGaps {
		deduction := g.GapSeverity * 5.0
		score -= deduction
	}

	// Deduct for expiration risks.
	for _, r := range expirationRisks {
		switch r.RiskLevel {
		case RiskCritical:
			if !r.HasReplacement {
				score -= 8.0
			} else {
				score -= 2.0
			}
		case RiskHigh:
			if !r.HasReplacement {
				score -= 5.0
			} else {
				score -= 1.0
			}
		case RiskMedium:
			score -= 2.0
		case RiskLow:
			score -= 0.5
		}
	}

	// Deduct for geographic gaps.
	for _, g := range geoGaps {
		score -= g.Priority * 3.0
	}

	// Bonus for portfolio size (diminishing returns).
	sizeBonus := math.Log1p(float64(len(patents))) * 2.0
	score += sizeBonus

	// Clamp to [0, 100].
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return math.Round(score*100) / 100
}

// -----------------------------------------------------------------------
// Package-Level Helper Functions for Gap Analysis
// -----------------------------------------------------------------------

// extractJurisdiction extracts the jurisdiction code from a patent number.
// e.g., "US1234567" -> "US", "EP1234567" -> "EP", "CN1234567" -> "CN"
func extractJurisdiction(patentNumber string) string {
	if len(patentNumber) < 2 {
		return ""
	}
	// Extract leading alphabetic characters.
	i := 0
	for i < len(patentNumber) && i < 3 {
		c := patentNumber[i]
		if c < 'A' || c > 'Z' {
			break
		}
		i++
	}
	if i == 0 {
		return ""
	}
	return patentNumber[:i]
}

// defaultTargetJurisdictions returns the default set of major patent jurisdictions.
func defaultTargetJurisdictions() []string {
	return []string{"US", "EP", "CN", "JP", "KR", "IN", "AU", "CA", "BR", "WO"}
}

// jurisdictionMarketScore returns a normalized market size score for a jurisdiction.
func jurisdictionMarketScore(juris string) float64 {
	scores := map[string]float64{
		"US": 1.0, "CN": 0.95, "EP": 0.9, "JP": 0.8, "KR": 0.7,
		"IN": 0.65, "CA": 0.55, "AU": 0.5, "BR": 0.45, "WO": 0.85,
	}
	if s, ok := scores[juris]; ok {
		return s
	}
	return 0.3
}

// jurisdictionName returns a human-readable name for a jurisdiction code.
func jurisdictionName(juris string) string {
	names := map[string]string{
		"US": "United States", "EP": "European Patent Office", "CN": "China",
		"JP": "Japan", "KR": "South Korea", "IN": "India",
		"CA": "Canada", "AU": "Australia", "BR": "Brazil",
		"WO": "WIPO (PCT)", "GB": "United Kingdom", "DE": "Germany",
		"FR": "France", "TW": "Taiwan", "RU": "Russia",
	}
	if n, ok := names[juris]; ok {
		return n
	}
	return juris
}

//Personal.AI order the ending

