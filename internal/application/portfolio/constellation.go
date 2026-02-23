// Phase 10 - File 222 of 349
// Generation Plan:
// - Functionality: Patent portfolio constellation (panoramic view) application service
// - Responsibility: Orchestrate molecular space dimensionality reduction, patent coverage overlay,
//   competitor comparison, and coverage heatmap generation into consumable application services
// - Core Implementations:
//   - Define ConstellationService interface with GenerateConstellation, GetTechDomainDistribution,
//     CompareWithCompetitor, GetCoverageHeatmap methods
//   - Implement constellationServiceImpl with dependency injection of domain services, repositories,
//     GNN inference engine, logger, and cache
//   - Orchestrate: validate -> load portfolio -> extract molecules -> GNN embedding -> overlay coverage
//     -> annotate white spaces -> cache -> return
// - Business Logic:
//   - Each point in constellation represents a molecule/patent, positioned by GNN embedding reduction
//   - Coverage represented by convex hull or density contour
//   - White spaces are low-density regions without patent coverage
//   - Competitor comparison marks overlap zones (competition) and exclusive zones (differentiation)
//   - Supports filtering by tech domain, filing year, legal status
// - Dependencies: domain/portfolio, domain/molecule, domain/patent, intelligence/molpatent_gnn,
//   pkg/errors, pkg/types/common
// - Depended by: interfaces/http/handlers/portfolio_handler
// - Test Requirements: Mock domain services and GNN inference, verify orchestration, caching, error handling
// - Constraint: Last line must be //Personal.AI order the ending

package portfolio

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	domainmol "github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	domainpatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	domainportfolio "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/molpatent_gnn"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// -----------------------------------------------------------------------
// Request / Response DTOs
// -----------------------------------------------------------------------

// ConstellationRequest defines parameters for generating a portfolio constellation view.
type ConstellationRequest struct {
	PortfolioID   string                `json:"portfolio_id" validate:"required"`
	Filters       ConstellationFilters  `json:"filters,omitempty"`
	Reduction     DimensionReduction    `json:"reduction,omitempty"`
	IncludeWhiteSpaces bool             `json:"include_white_spaces"`
}

// ConstellationFilters provides optional filtering criteria for constellation generation.
type ConstellationFilters struct {
	TechDomains   []string              `json:"tech_domains,omitempty"`
	FilingYearMin int                   `json:"filing_year_min,omitempty"`
	FilingYearMax int                   `json:"filing_year_max,omitempty"`
	LegalStatuses []string              `json:"legal_statuses,omitempty"`
	Assignees     []string              `json:"assignees,omitempty"`
}

// DimensionReduction specifies the algorithm and target dimensions for embedding reduction.
type DimensionReduction struct {
	Algorithm  ReductionAlgorithm `json:"algorithm,omitempty"`
	Dimensions int                `json:"dimensions,omitempty"`
	Perplexity float64            `json:"perplexity,omitempty"`
	Neighbors  int                `json:"neighbors,omitempty"`
}

// ReductionAlgorithm enumerates supported dimensionality reduction algorithms.
type ReductionAlgorithm string

const (
	ReductionTSNE ReductionAlgorithm = "tsne"
	ReductionUMAP ReductionAlgorithm = "umap"
	ReductionPCA  ReductionAlgorithm = "pca"
)

// ConstellationResponse contains the full constellation view data.
type ConstellationResponse struct {
	PortfolioID   string                `json:"portfolio_id"`
	Points        []ConstellationPoint  `json:"points"`
	Clusters      []ConstellationCluster `json:"clusters,omitempty"`
	WhiteSpaces   []WhiteSpaceRegion    `json:"white_spaces,omitempty"`
	CoverageStats CoverageStatistics    `json:"coverage_stats"`
	GeneratedAt   time.Time             `json:"generated_at"`
	CacheKey      string                `json:"cache_key,omitempty"`
}

// ConstellationPoint represents a single point (molecule/patent) in the constellation.
type ConstellationPoint struct {
	ID            string    `json:"id"`
	PatentNumber  string    `json:"patent_number,omitempty"`
	MoleculeID    string    `json:"molecule_id,omitempty"`
	SMILES        string    `json:"smiles,omitempty"`
	X             float64   `json:"x"`
	Y             float64   `json:"y"`
	Z             float64   `json:"z,omitempty"`
	TechDomain    string    `json:"tech_domain,omitempty"`
	LegalStatus   string    `json:"legal_status,omitempty"`
	Assignee      string    `json:"assignee,omitempty"`
	FilingYear    int       `json:"filing_year,omitempty"`
	ValueScore    float64   `json:"value_score,omitempty"`
	PointType     PointType `json:"point_type"`
}

// PointType enumerates the type of a constellation point.
type PointType string

const (
	PointTypeOwnPatent       PointType = "own_patent"
	PointTypeCompetitorPatent PointType = "competitor_patent"
	PointTypePublicPatent    PointType = "public_patent"
	PointTypeMolecule        PointType = "molecule"
)

// ConstellationCluster represents a cluster of related points in the constellation.
type ConstellationCluster struct {
	ClusterID   string    `json:"cluster_id"`
	Label       string    `json:"label"`
	CenterX     float64   `json:"center_x"`
	CenterY     float64   `json:"center_y"`
	Radius      float64   `json:"radius"`
	PointCount  int       `json:"point_count"`
	TechDomains []string  `json:"tech_domains,omitempty"`
	Density     float64   `json:"density"`
}

// WhiteSpaceRegion represents an identified gap in the patent landscape.
type WhiteSpaceRegion struct {
	RegionID    string    `json:"region_id"`
	CenterX     float64   `json:"center_x"`
	CenterY     float64   `json:"center_y"`
	Area        float64   `json:"area"`
	NearestTech []string  `json:"nearest_tech_domains,omitempty"`
	Opportunity string    `json:"opportunity_description,omitempty"`
	Score       float64   `json:"score"`
}

// CoverageStatistics provides aggregate statistics about the constellation.
type CoverageStatistics struct {
	TotalPoints       int     `json:"total_points"`
	OwnPatentCount    int     `json:"own_patent_count"`
	CompetitorCount   int     `json:"competitor_count"`
	CoverageRatio     float64 `json:"coverage_ratio"`
	WhiteSpaceCount   int     `json:"white_space_count"`
	ClusterCount      int     `json:"cluster_count"`
	DensityMean       float64 `json:"density_mean"`
	DensityStdDev     float64 `json:"density_std_dev"`
}

// CompetitorCompareRequest defines parameters for comparing portfolios with a competitor.
type CompetitorCompareRequest struct {
	PortfolioID    string   `json:"portfolio_id" validate:"required"`
	CompetitorName string   `json:"competitor_name" validate:"required"`
	CompetitorIDs  []string `json:"competitor_patent_ids,omitempty"`
	TechDomains    []string `json:"tech_domains,omitempty"`
}

// CompetitorCompareResponse contains the comparison result.
type CompetitorCompareResponse struct {
	PortfolioID     string              `json:"portfolio_id"`
	CompetitorName  string              `json:"competitor_name"`
	OverlapZones    []OverlapZone       `json:"overlap_zones"`
	OwnExclusive    []ExclusiveZone     `json:"own_exclusive_zones"`
	CompExclusive   []ExclusiveZone     `json:"competitor_exclusive_zones"`
	StrengthIndex   float64             `json:"strength_index"`
	Summary         ComparisonSummary   `json:"summary"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

// OverlapZone represents a region where both portfolios have coverage.
type OverlapZone struct {
	ZoneID      string   `json:"zone_id"`
	TechDomain  string   `json:"tech_domain"`
	OwnCount    int      `json:"own_patent_count"`
	CompCount   int      `json:"competitor_patent_count"`
	Intensity   float64  `json:"competition_intensity"`
}

// ExclusiveZone represents a region where only one party has coverage.
type ExclusiveZone struct {
	ZoneID      string   `json:"zone_id"`
	TechDomain  string   `json:"tech_domain"`
	PatentCount int      `json:"patent_count"`
	Strength    float64  `json:"strength_score"`
}

// ComparisonSummary provides a high-level summary of the comparison.
type ComparisonSummary struct {
	TotalOwnPatents      int     `json:"total_own_patents"`
	TotalCompPatents     int     `json:"total_competitor_patents"`
	OverlapDomainCount   int     `json:"overlap_domain_count"`
	OwnExclusiveCount    int     `json:"own_exclusive_domain_count"`
	CompExclusiveCount   int     `json:"competitor_exclusive_domain_count"`
	OverallAdvantage     string  `json:"overall_advantage"`
	AdvantageScore       float64 `json:"advantage_score"`
}

// DomainDistribution contains the distribution of patents across technology domains.
type DomainDistribution struct {
	PortfolioID string                `json:"portfolio_id"`
	Domains     []DomainEntry         `json:"domains"`
	TotalCount  int                   `json:"total_count"`
	GeneratedAt time.Time             `json:"generated_at"`
}

// DomainEntry represents a single technology domain's statistics.
type DomainEntry struct {
	DomainCode   string  `json:"domain_code"`
	DomainName   string  `json:"domain_name"`
	PatentCount  int     `json:"patent_count"`
	Percentage   float64 `json:"percentage"`
	ValueSum     float64 `json:"value_sum"`
	ValuePercent float64 `json:"value_percent"`
	AvgAge       float64 `json:"avg_age_years"`
}

// HeatmapOption configures heatmap generation behavior.
type HeatmapOption func(*heatmapConfig)

type heatmapConfig struct {
	Resolution int
	MinDensity float64
	MaxDensity float64
	ColorScale string
}

// CoverageHeatmap contains the heatmap data for patent coverage visualization.
type CoverageHeatmap struct {
	PortfolioID string          `json:"portfolio_id"`
	Grid        [][]float64     `json:"grid"`
	XRange      [2]float64      `json:"x_range"`
	YRange      [2]float64      `json:"y_range"`
	Resolution  int             `json:"resolution"`
	MaxDensity  float64         `json:"max_density"`
	GeneratedAt time.Time       `json:"generated_at"`
}

// WithResolution sets the heatmap grid resolution.
func WithResolution(res int) HeatmapOption {
	return func(c *heatmapConfig) {
		if res > 0 && res <= 500 {
			c.Resolution = res
		}
	}
}

// WithDensityRange sets the min/max density for heatmap normalization.
func WithDensityRange(min, max float64) HeatmapOption {
	return func(c *heatmapConfig) {
		if min >= 0 && max > min {
			c.MinDensity = min
			c.MaxDensity = max
		}
	}
}

// -----------------------------------------------------------------------
// Cache Adapter Interface
// -----------------------------------------------------------------------

// ConstellationCache abstracts caching operations for constellation data.
type ConstellationCache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// -----------------------------------------------------------------------
// Service Interface
// -----------------------------------------------------------------------

// ConstellationService defines the application-level interface for portfolio constellation operations.
type ConstellationService interface {
	// GenerateConstellation produces a full constellation view of a patent portfolio,
	// including molecular space embedding, patent coverage overlay, and optional white space detection.
	GenerateConstellation(ctx context.Context, req *ConstellationRequest) (*ConstellationResponse, error)

	// GetTechDomainDistribution returns the distribution of patents across technology domains
	// within a given portfolio.
	GetTechDomainDistribution(ctx context.Context, portfolioID string) (*DomainDistribution, error)

	// CompareWithCompetitor generates a comparative analysis between the user's portfolio
	// and a specified competitor's patent holdings.
	CompareWithCompetitor(ctx context.Context, req *CompetitorCompareRequest) (*CompetitorCompareResponse, error)

	// GetCoverageHeatmap produces a density heatmap of patent coverage in molecular space.
	GetCoverageHeatmap(ctx context.Context, portfolioID string, opts ...HeatmapOption) (*CoverageHeatmap, error)
}

// -----------------------------------------------------------------------
// Service Implementation
// -----------------------------------------------------------------------

type constellationServiceImpl struct {
	portfolioSvc   domainportfolio.Service
	portfolioRepo  domainportfolio.PortfolioRepository
	moleculeSvc    domainmol.Service
	patentRepo     domainpatent.Repository
	moleculeRepo   domainmol.Repository
	gnnInference   molpatent_gnn.InferenceEngine
	logger         logging.Logger
	cache          ConstellationCache
	cacheTTL       time.Duration
}

// ConstellationServiceConfig holds configuration for constructing the service.
type ConstellationServiceConfig struct {
	PortfolioService    domainportfolio.Service
	PortfolioRepository domainportfolio.PortfolioRepository
	MoleculeService     domainmol.Service
	PatentRepository    domainpatent.Repository
	MoleculeRepository  domainmol.Repository
	GNNInference        molpatent_gnn.InferenceEngine
	Logger              logging.Logger
	Cache               ConstellationCache
	CacheTTL            time.Duration
}

// NewConstellationService constructs a ConstellationService with all required dependencies.
// Returns an error if any mandatory dependency is nil.
func NewConstellationService(cfg ConstellationServiceConfig) (ConstellationService, error) {
	if cfg.PortfolioService == nil {
		return nil, errors.NewValidation("ConstellationService requires PortfolioService")
	}
	if cfg.MoleculeService == nil {
		return nil, errors.NewValidation("ConstellationService requires MoleculeService")
	}
	if cfg.PatentRepository == nil {
		return nil, errors.NewValidation("ConstellationService requires PatentRepository")
	}
	if cfg.MoleculeRepository == nil {
		return nil, errors.NewValidation("ConstellationService requires MoleculeRepository")
	}
	if cfg.GNNInference == nil {
		return nil, errors.NewValidation("ConstellationService requires GNNInference")
	}
	if cfg.Logger == nil {
		return nil, errors.NewValidation("ConstellationService requires Logger")
	}

	ttl := cfg.CacheTTL
	if ttl == 0 {
		ttl = 30 * time.Minute
	}

	return &constellationServiceImpl{
		portfolioSvc:   cfg.PortfolioService,
		portfolioRepo:  cfg.PortfolioRepository,
		moleculeSvc:    cfg.MoleculeService,
		patentRepo:     cfg.PatentRepository,
		moleculeRepo:   cfg.MoleculeRepository,
		gnnInference:   cfg.GNNInference,
		logger:         cfg.Logger,
		cache:          cfg.Cache,
		cacheTTL:       ttl,
	}, nil
}

// GenerateConstellation orchestrates the full constellation generation pipeline.
func (s *constellationServiceImpl) GenerateConstellation(ctx context.Context, req *ConstellationRequest) (*ConstellationResponse, error) {
	if req == nil {
		return nil, errors.NewValidation("constellation request must not be nil")
	}
	if req.PortfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}

	// Apply defaults for reduction parameters.
	reduction := applyReductionDefaults(req.Reduction)

	// Attempt cache lookup.
	cacheKey := s.buildCacheKey("constellation", req)
	if s.cache != nil {
		var cached ConstellationResponse
		if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
			s.logger.Debug(ctx, "constellation cache hit", "portfolio_id", req.PortfolioID)
			return &cached, nil
		}
	}

	s.logger.Info(ctx, "generating constellation", "portfolio_id", req.PortfolioID)

	// Step 1: Load portfolio and its patents.
	portfolioID, err := uuid.Parse(req.PortfolioID)
	if err != nil {
		return nil, errors.NewInvalidParameterError("invalid portfolio ID format")
	}
	portfolio, err := s.portfolioRepo.GetByID(ctx, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeNotFound, "failed to load portfolio")
	}
	if portfolio == nil {
		return nil, errors.NewNotFound("portfolio", req.PortfolioID)
	}

	patents, err := s.patentRepo.FindByPortfolioID(ctx, req.PortfolioID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load portfolio patents")
	}

	// Step 2: Apply filters.
	patents = s.applyPatentFilters(patents, req.Filters)
	if len(patents) == 0 {
		return &ConstellationResponse{
			PortfolioID:   req.PortfolioID,
			Points:        []ConstellationPoint{},
			CoverageStats: CoverageStatistics{},
			GeneratedAt:   time.Now().UTC(),
		}, nil
	}

	// Step 3: Extract molecules associated with patents.
	moleculeIDs := s.extractMoleculeIDs(patents)
	molecules, err := s.moleculeRepo.FindByIDs(ctx, moleculeIDs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load molecules for constellation")
	}

	// Step 4: Generate GNN embeddings for all molecules.
	embeddings, err := s.generateEmbeddings(ctx, molecules)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate GNN embeddings")
	}

	// Step 5: Perform dimensionality reduction.
	reduced, err := s.reduceEmbeddings(ctx, embeddings, reduction)
	if err != nil {
		return nil, errors.Wrap(err, "failed to reduce embeddings")
	}

	// Step 6: Build constellation points.
	points := s.buildPoints(patents, molecules, reduced)

	// Step 7: Detect clusters.
	clusters := s.detectClusters(points)

	// Step 8: Identify white spaces if requested.
	var whiteSpaces []WhiteSpaceRegion
	if req.IncludeWhiteSpaces {
		whiteSpaces = s.identifyWhiteSpaces(points, clusters)
	}

	// Step 9: Compute coverage statistics.
	stats := s.computeStats(points, clusters, whiteSpaces)

	response := &ConstellationResponse{
		PortfolioID:   req.PortfolioID,
		Points:        points,
		Clusters:      clusters,
		WhiteSpaces:   whiteSpaces,
		CoverageStats: stats,
		GeneratedAt:   time.Now().UTC(),
		CacheKey:      cacheKey,
	}

	// Cache the result.
	if s.cache != nil {
		if cacheErr := s.cache.Set(ctx, cacheKey, response, s.cacheTTL); cacheErr != nil {
			s.logger.Warn(ctx, "failed to cache constellation result", "error", cacheErr)
		}
	}

	s.logger.Info(ctx, "constellation generated",
		"portfolio_id", req.PortfolioID,
		"points", len(points),
		"clusters", len(clusters),
		"white_spaces", len(whiteSpaces),
	)

	return response, nil
}

// GetTechDomainDistribution returns the technology domain distribution for a portfolio.
func (s *constellationServiceImpl) GetTechDomainDistribution(ctx context.Context, portfolioID string) (*DomainDistribution, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}

	cacheKey := fmt.Sprintf("domain_dist:%s", portfolioID)
	if s.cache != nil {
		var cached DomainDistribution
		if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
	}

	portfolioUUID, err := uuid.Parse(portfolioID)
	if err != nil {
		return nil, errors.NewInvalidParameterError("invalid portfolio ID format")
	}
	portfolio, err := s.portfolioRepo.GetByID(ctx, portfolioUUID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeNotFound, "failed to load portfolio")
	}
	if portfolio == nil {
		return nil, errors.NewNotFound("portfolio", portfolioID)
	}

	patents, err := s.patentRepo.FindByPortfolioID(ctx, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to load patents for domain distribution")
	}

	// Aggregate by technology domain.
	domainMap := make(map[string]*DomainEntry)
	totalCount := 0
	now := time.Now()

	for _, p := range patents {
		domain := p.GetPrimaryTechDomain()
		if domain == "" {
			domain = "unclassified"
		}

		entry, exists := domainMap[domain]
		if !exists {
			entry = &DomainEntry{
				DomainCode: domain,
				DomainName: resolveDomainName(domain),
			}
			domainMap[domain] = entry
		}

		entry.PatentCount++
		entry.ValueSum += p.GetValueScore()
		if !p.GetFilingDate().IsZero() {
			ageYears := now.Sub(p.GetFilingDate()).Hours() / (24 * 365.25)
			entry.AvgAge = entry.AvgAge + ageYears
		}
		totalCount++
	}

	// Compute percentages and finalize averages.
	domains := make([]DomainEntry, 0, len(domainMap))
	totalValue := 0.0
	for _, e := range domainMap {
		totalValue += e.ValueSum
	}

	for _, e := range domainMap {
		if e.PatentCount > 0 {
			e.Percentage = float64(e.PatentCount) / float64(totalCount) * 100.0
			e.AvgAge = e.AvgAge / float64(e.PatentCount)
		}
		if totalValue > 0 {
			e.ValuePercent = e.ValueSum / totalValue * 100.0
		}
		domains = append(domains, *e)
	}

	// Sort by patent count descending.
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].PatentCount > domains[j].PatentCount
	})

	result := &DomainDistribution{
		PortfolioID: portfolioID,
		Domains:     domains,
		TotalCount:  totalCount,
		GeneratedAt: time.Now().UTC(),
	}

	if s.cache != nil {
		if cacheErr := s.cache.Set(ctx, cacheKey, result, s.cacheTTL); cacheErr != nil {
			s.logger.Warn(ctx, "failed to cache domain distribution", "error", cacheErr)
		}
	}

	return result, nil
}

// CompareWithCompetitor generates a comparative analysis between the user's portfolio and a competitor.
func (s *constellationServiceImpl) CompareWithCompetitor(ctx context.Context, req *CompetitorCompareRequest) (*CompetitorCompareResponse, error) {
	if req == nil {
		return nil, errors.NewValidation("competitor compare request must not be nil")
	}
	if req.PortfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}
	if req.CompetitorName == "" {
		return nil, errors.NewValidation("competitor_name is required")
	}

	s.logger.Info(ctx, "comparing portfolio with competitor",
		"portfolio_id", req.PortfolioID,
		"competitor", req.CompetitorName,
	)

	// Load own patents.
	ownPatents, err := s.patentRepo.FindByPortfolioID(ctx, req.PortfolioID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load own patents")
	}

	// Load competitor patents.
	var compPatents []domainpatent.Patent
	if len(req.CompetitorIDs) > 0 {
		compPatents, err = s.patentRepo.FindByIDs(ctx, req.CompetitorIDs)
	} else {
		compPatents, err = s.patentRepo.FindByAssignee(ctx, req.CompetitorName)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to load competitor patents")
	}

	// Filter by tech domains if specified.
	if len(req.TechDomains) > 0 {
		ownPatents = filterByTechDomains(ownPatents, req.TechDomains)
		compPatents = filterByTechDomains(compPatents, req.TechDomains)
	}

	// Build domain-level comparison.
	ownDomains := groupByDomain(ownPatents)
	compDomains := groupByDomain(compPatents)

	allDomains := mergeKeys(ownDomains, compDomains)

	var overlapZones []OverlapZone
	var ownExclusive []ExclusiveZone
	var compExclusive []ExclusiveZone

	for _, domain := range allDomains {
		ownCount := len(ownDomains[domain])
		compCount := len(compDomains[domain])

		if ownCount > 0 && compCount > 0 {
			// Both parties have patents in this domain — overlap (competition) zone.
			total := float64(ownCount + compCount)
			intensity := 1.0 - math.Abs(float64(ownCount)-float64(compCount))/total
			overlapZones = append(overlapZones, OverlapZone{
				ZoneID:     fmt.Sprintf("overlap-%s", domain),
				TechDomain: domain,
				OwnCount:   ownCount,
				CompCount:  compCount,
				Intensity:  intensity,
			})
		} else if ownCount > 0 {
			// Only own portfolio covers this domain — exclusive advantage.
			ownExclusive = append(ownExclusive, ExclusiveZone{
				ZoneID:      fmt.Sprintf("own-excl-%s", domain),
				TechDomain:  domain,
				PatentCount: ownCount,
				Strength:    computeZoneStrength(ownDomains[domain]),
			})
		} else if compCount > 0 {
			// Only competitor covers this domain — competitor advantage / own gap.
			compExclusive = append(compExclusive, ExclusiveZone{
				ZoneID:      fmt.Sprintf("comp-excl-%s", domain),
				TechDomain:  domain,
				PatentCount: compCount,
				Strength:    computeZoneStrength(compDomains[domain]),
			})
		}
	}

	// Compute overall strength index: positive means own advantage, negative means competitor advantage.
	strengthIndex := computeStrengthIndex(ownPatents, compPatents, overlapZones)

	advantage := "neutral"
	if strengthIndex > 0.1 {
		advantage = "own"
	} else if strengthIndex < -0.1 {
		advantage = "competitor"
	}

	summary := ComparisonSummary{
		TotalOwnPatents:    len(ownPatents),
		TotalCompPatents:   len(compPatents),
		OverlapDomainCount: len(overlapZones),
		OwnExclusiveCount:  len(ownExclusive),
		CompExclusiveCount: len(compExclusive),
		OverallAdvantage:   advantage,
		AdvantageScore:     strengthIndex,
	}

	response := &CompetitorCompareResponse{
		PortfolioID:    req.PortfolioID,
		CompetitorName: req.CompetitorName,
		OverlapZones:   overlapZones,
		OwnExclusive:   ownExclusive,
		CompExclusive:  compExclusive,
		StrengthIndex:  strengthIndex,
		Summary:        summary,
		GeneratedAt:    time.Now().UTC(),
	}

	s.logger.Info(ctx, "competitor comparison completed",
		"portfolio_id", req.PortfolioID,
		"competitor", req.CompetitorName,
		"overlap_zones", len(overlapZones),
		"own_exclusive", len(ownExclusive),
		"comp_exclusive", len(compExclusive),
		"strength_index", strengthIndex,
	)

	return response, nil
}

// GetCoverageHeatmap produces a density heatmap of patent coverage in molecular space.
func (s *constellationServiceImpl) GetCoverageHeatmap(ctx context.Context, portfolioID string, opts ...HeatmapOption) (*CoverageHeatmap, error) {
	if portfolioID == "" {
		return nil, errors.NewValidation("portfolio_id is required")
	}

	// Apply option defaults.
	cfg := &heatmapConfig{
		Resolution: 100,
		MinDensity: 0.0,
		MaxDensity: 0.0,
		ColorScale: "viridis",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	cacheKey := fmt.Sprintf("heatmap:%s:res%d", portfolioID, cfg.Resolution)
	if s.cache != nil {
		var cached CoverageHeatmap
		if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
			s.logger.Debug(ctx, "heatmap cache hit", "portfolio_id", portfolioID)
			return &cached, nil
		}
	}

	s.logger.Info(ctx, "generating coverage heatmap", "portfolio_id", portfolioID, "resolution", cfg.Resolution)

	// Load patents and their molecules.
	patents, err := s.patentRepo.FindByPortfolioID(ctx, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load patents for heatmap")
	}
	if len(patents) == 0 {
		return &CoverageHeatmap{
			PortfolioID: portfolioID,
			Grid:        make([][]float64, 0),
			Resolution:  cfg.Resolution,
			GeneratedAt: time.Now().UTC(),
		}, nil
	}

	moleculeIDs := s.extractMoleculeIDs(patents)
	molecules, err := s.moleculeRepo.FindByIDs(ctx, moleculeIDs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load molecules for heatmap")
	}

	// Generate embeddings and reduce to 2D.
	embeddings, err := s.generateEmbeddings(ctx, molecules)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate embeddings for heatmap")
	}

	reduction := DimensionReduction{
		Algorithm:  ReductionUMAP,
		Dimensions: 2,
		Neighbors:  15,
	}
	reduced, err := s.reduceEmbeddings(ctx, embeddings, reduction)
	if err != nil {
		return nil, errors.Wrap(err, "failed to reduce embeddings for heatmap")
	}

	// Compute bounding box of all reduced points.
	xMin, xMax, yMin, yMax := computeBoundingBox(reduced)

	// Add padding (10% on each side).
	xPad := (xMax - xMin) * 0.1
	yPad := (yMax - yMin) * 0.1
	xMin -= xPad
	xMax += xPad
	yMin -= yPad
	yMax += yPad

	// Prevent degenerate ranges.
	if xMax-xMin < 1e-6 {
		xMin -= 1.0
		xMax += 1.0
	}
	if yMax-yMin < 1e-6 {
		yMin -= 1.0
		yMax += 1.0
	}

	// Build density grid using kernel density estimation (Gaussian kernel).
	res := cfg.Resolution
	grid := make([][]float64, res)
	for i := range grid {
		grid[i] = make([]float64, res)
	}

	xStep := (xMax - xMin) / float64(res)
	yStep := (yMax - yMin) / float64(res)

	// Bandwidth selection: Silverman's rule of thumb.
	n := float64(len(reduced))
	bandwidth := 1.06 * math.Pow(n, -0.2)
	if bandwidth < 0.01 {
		bandwidth = 0.01
	}

	maxDensity := 0.0
	for i := 0; i < res; i++ {
		gx := xMin + (float64(i)+0.5)*xStep
		for j := 0; j < res; j++ {
			gy := yMin + (float64(j)+0.5)*yStep
			density := 0.0
			for _, pt := range reduced {
				if len(pt) < 2 {
					continue
				}
				dx := (gx - pt[0]) / bandwidth
				dy := (gy - pt[1]) / bandwidth
				density += math.Exp(-0.5 * (dx*dx + dy*dy))
			}
			density /= (n * 2.0 * math.Pi * bandwidth * bandwidth)
			grid[i][j] = density
			if density > maxDensity {
				maxDensity = density
			}
		}
	}

	// Normalize grid if maxDensity override is specified.
	if cfg.MaxDensity > 0 {
		maxDensity = cfg.MaxDensity
	}

	result := &CoverageHeatmap{
		PortfolioID: portfolioID,
		Grid:        grid,
		XRange:      [2]float64{xMin, xMax},
		YRange:      [2]float64{yMin, yMax},
		Resolution:  res,
		MaxDensity:  maxDensity,
		GeneratedAt: time.Now().UTC(),
	}

	if s.cache != nil {
		if cacheErr := s.cache.Set(ctx, cacheKey, result, s.cacheTTL); cacheErr != nil {
			s.logger.Warn(ctx, "failed to cache heatmap", "error", cacheErr)
		}
	}

	s.logger.Info(ctx, "heatmap generated",
		"portfolio_id", portfolioID,
		"resolution", res,
		"max_density", maxDensity,
		"point_count", len(reduced),
	)

	return result, nil
}

// -----------------------------------------------------------------------
// Internal Helper Methods
// -----------------------------------------------------------------------

// applyReductionDefaults fills in default values for dimension reduction parameters.
func applyReductionDefaults(r DimensionReduction) DimensionReduction {
	if r.Algorithm == "" {
		r.Algorithm = ReductionUMAP
	}
	if r.Dimensions == 0 {
		r.Dimensions = 2
	}
	if r.Dimensions < 2 {
		r.Dimensions = 2
	}
	if r.Dimensions > 3 {
		r.Dimensions = 3
	}
	if r.Perplexity == 0 && r.Algorithm == ReductionTSNE {
		r.Perplexity = 30.0
	}
	if r.Neighbors == 0 && r.Algorithm == ReductionUMAP {
		r.Neighbors = 15
	}
	return r
}

// buildCacheKey generates a deterministic cache key from the request parameters.
func (s *constellationServiceImpl) buildCacheKey(prefix string, req *ConstellationRequest) string {
	raw := fmt.Sprintf("%s:%s:%v:%v:%v",
		prefix,
		req.PortfolioID,
		req.Filters,
		req.Reduction,
		req.IncludeWhiteSpaces,
	)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%s:%s:%x", prefix, req.PortfolioID, hash[:8])
}

// applyPatentFilters filters patents based on the provided criteria.
func (s *constellationServiceImpl) applyPatentFilters(patents []domainpatent.Patent, filters ConstellationFilters) []domainpatent.Patent {
	if len(filters.TechDomains) == 0 &&
		filters.FilingYearMin == 0 &&
		filters.FilingYearMax == 0 &&
		len(filters.LegalStatuses) == 0 &&
		len(filters.Assignees) == 0 {
		return patents
	}

	techSet := toStringSet(filters.TechDomains)
	statusSet := toStringSet(filters.LegalStatuses)
	assigneeSet := toStringSet(filters.Assignees)

	filtered := make([]domainpatent.Patent, 0, len(patents))
	for _, p := range patents {
		// Filter by tech domain.
		if len(techSet) > 0 {
			if _, ok := techSet[p.GetPrimaryTechDomain()]; !ok {
				continue
			}
		}

		// Filter by filing year range.
		if filters.FilingYearMin > 0 || filters.FilingYearMax > 0 {
			filingDate := p.GetFilingDate()
			if filingDate.IsZero() {
				continue
			}
			year := filingDate.Year()
			if filters.FilingYearMin > 0 && year < filters.FilingYearMin {
				continue
			}
			if filters.FilingYearMax > 0 && year > filters.FilingYearMax {
				continue
			}
		}

		// Filter by legal status.
		if len(statusSet) > 0 {
			if _, ok := statusSet[p.GetLegalStatus()]; !ok {
				continue
			}
		}

		// Filter by assignee.
		if len(assigneeSet) > 0 {
			if _, ok := assigneeSet[p.GetAssignee()]; !ok {
				continue
			}
		}

		filtered = append(filtered, p)
	}

	return filtered
}

// extractMoleculeIDs collects unique molecule IDs from a set of patents.
func (s *constellationServiceImpl) extractMoleculeIDs(patents []domainpatent.Patent) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, p := range patents {
		for _, mid := range p.GetMoleculeIDs() {
			if _, exists := seen[mid]; !exists {
				seen[mid] = struct{}{}
				ids = append(ids, mid)
			}
		}
	}
	return ids
}

// generateEmbeddings invokes the GNN inference engine to produce embeddings for molecules.
func (s *constellationServiceImpl) generateEmbeddings(ctx context.Context, molecules []domainmol.Molecule) (map[string][]float64, error) {
	embeddings := make(map[string][]float64, len(molecules))

	for _, mol := range molecules {
		smiles := mol.GetSMILES()
		if smiles == "" {
			continue
		}

		embedding, err := s.gnnInference.GenerateEmbedding(ctx, &molpatent_gnn.EmbeddingRequest{
			SMILES:    smiles,
			ModelType: molpatent_gnn.ModelTypeMolecular,
		})
		if err != nil {
			s.logger.Warn(ctx, "failed to generate embedding for molecule",
				"molecule_id", mol.GetID(),
				"error", err,
			)
			continue
		}

		embeddings[mol.GetID()] = embedding.Vector
	}

	if len(embeddings) == 0 {
		return nil, errors.NewInternal("no embeddings could be generated for portfolio molecules")
	}

	return embeddings, nil
}

// reduceEmbeddings performs dimensionality reduction on the embedding vectors.
// This delegates to the GNN inference engine's built-in reduction capability.
func (s *constellationServiceImpl) reduceEmbeddings(ctx context.Context, embeddings map[string][]float64, reduction DimensionReduction) ([][]float64, error) {
	// Collect all vectors in deterministic order.
	keys := make([]string, 0, len(embeddings))
	for k := range embeddings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	vectors := make([][]float64, 0, len(keys))
	for _, k := range keys {
		vectors = append(vectors, embeddings[k])
	}

	reduced, err := s.gnnInference.ReduceDimensions(ctx, &molpatent_gnn.ReductionRequest{
		Vectors:    vectors,
		Algorithm:  string(reduction.Algorithm),
		Dimensions: reduction.Dimensions,
		Perplexity: reduction.Perplexity,
		Neighbors:  reduction.Neighbors,
	})
	if err != nil {
		return nil, err
	}

	return reduced.Reduced, nil
}

// buildPoints constructs ConstellationPoint entries from patents, molecules, and reduced coordinates.
func (s *constellationServiceImpl) buildPoints(patents []domainpatent.Patent, molecules []domainmol.Molecule, reduced [][]float64) []ConstellationPoint {
	// Build molecule lookup.
	molMap := make(map[string]domainmol.Molecule, len(molecules))
	for _, m := range molecules {
		molMap[m.GetID()] = m
	}

	points := make([]ConstellationPoint, 0, len(reduced))
	coordIdx := 0

	for _, p := range patents {
		molIDs := p.GetMoleculeIDs()
		for _, mid := range molIDs {
			if coordIdx >= len(reduced) {
				break
			}

			coords := reduced[coordIdx]
			coordIdx++

			pt := ConstellationPoint{
				ID:           fmt.Sprintf("%s-%s", p.GetID(), mid),
				PatentNumber: p.GetPatentNumber(),
				MoleculeID:   mid,
				TechDomain:   p.GetPrimaryTechDomain(),
				LegalStatus:  p.GetLegalStatus(),
				Assignee:     p.GetAssignee(),
				ValueScore:   p.GetValueScore(),
				PointType:    PointTypeOwnPatent,
			}

			if len(coords) >= 1 {
				pt.X = coords[0]
			}
			if len(coords) >= 2 {
				pt.Y = coords[1]
			}
			if len(coords) >= 3 {
				pt.Z = coords[2]
			}

			if mol, ok := molMap[mid]; ok {
				pt.SMILES = mol.GetSMILES()
			}

			if !p.GetFilingDate().IsZero() {
				pt.FilingYear = p.GetFilingDate().Year()
			}

			points = append(points, pt)
		}
	}

	return points
}

// detectClusters performs simple grid-based density clustering on constellation points.
func (s *constellationServiceImpl) detectClusters(points []ConstellationPoint) []ConstellationCluster {
	if len(points) < 3 {
		return nil
	}

	// Simple approach: grid-based clustering.
	// Determine bounding box.
	xMin, xMax := points[0].X, points[0].X
	yMin, yMax := points[0].Y, points[0].Y
	for _, p := range points[1:] {
		if p.X < xMin {
			xMin = p.X
		}
		if p.X > xMax {
			xMax = p.X
		}
		if p.Y < yMin {
			yMin = p.Y
		}
		if p.Y > yMax {
			yMax = p.Y
		}
	}

	// Use adaptive grid size based on point count.
	gridSize := int(math.Ceil(math.Sqrt(float64(len(points)) / 3.0)))
	if gridSize < 2 {
		gridSize = 2
	}
	if gridSize > 20 {
		gridSize = 20
	}

	xStep := (xMax - xMin) / float64(gridSize)
	yStep := (yMax - yMin) / float64(gridSize)
	if xStep < 1e-9 {
		xStep = 1.0
	}
	if yStep < 1e-9 {
		yStep = 1.0
	}

	// Assign points to grid cells.
	type cellKey struct{ r, c int }
	cellPoints := make(map[cellKey][]ConstellationPoint)

	for _, p := range points {
		r := int((p.X - xMin) / xStep)
		c := int((p.Y - yMin) / yStep)
		if r >= gridSize {
			r = gridSize - 1
		}
		if c >= gridSize {
			c = gridSize - 1
		}
		key := cellKey{r, c}
		cellPoints[key] = append(cellPoints[key], p)
	}

	// Identify clusters: cells with density above threshold.
	meanDensity := float64(len(points)) / float64(gridSize*gridSize)
	threshold := meanDensity * 1.5
	if threshold < 2 {
		threshold = 2
	}

	clusters := make([]ConstellationCluster, 0)
	clusterIdx := 0

	for key, pts := range cellPoints {
		if float64(len(pts)) < threshold {
			continue
		}

		// Compute cluster center and radius.
		cx, cy := 0.0, 0.0
		for _, p := range pts {
			cx += p.X
			cy += p.Y
		}
		cx /= float64(len(pts))
		cy /= float64(len(pts))

		maxDist := 0.0
		techDomainSet := make(map[string]struct{})
		for _, p := range pts {
			dist := math.Sqrt((p.X-cx)*(p.X-cx) + (p.Y-cy)*(p.Y-cy))
			if dist > maxDist {
				maxDist = dist
			}
			if p.TechDomain != "" {
				techDomainSet[p.TechDomain] = struct{}{}
			}
		}

		techDomains := make([]string, 0, len(techDomainSet))
		for td := range techDomainSet {
			techDomains = append(techDomains, td)
		}
		sort.Strings(techDomains)

		label := "cluster"
		if len(techDomains) > 0 {
			label = techDomains[0]
		}

		area := math.Pi * maxDist * maxDist
		density := 0.0
		if area > 0 {
			density = float64(len(pts)) / area
		}

		clusters = append(clusters, ConstellationCluster{
			ClusterID:   fmt.Sprintf("cluster-%d-%d-%d", clusterIdx, key.r, key.c),
			Label:       label,
			CenterX:     cx,
			CenterY:     cy,
			Radius:      maxDist,
			PointCount:  len(pts),
			TechDomains: techDomains,
			Density:     density,
		})
		clusterIdx++
	}

	// Sort clusters by point count descending.
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].PointCount > clusters[j].PointCount
	})

	return clusters
}

// identifyWhiteSpaces finds regions in the constellation with low patent density.
func (s *constellationServiceImpl) identifyWhiteSpaces(points []ConstellationPoint, clusters []ConstellationCluster) []WhiteSpaceRegion {
	if len(points) < 5 {
		return nil
	}

	// Determine bounding box.
	xMin, xMax := points[0].X, points[0].X
	yMin, yMax := points[0].Y, points[0].Y
	for _, p := range points[1:] {
		if p.X < xMin {
			xMin = p.X
		}
		if p.X > xMax {
			xMax = p.X
		}
		if p.Y < yMin {
			yMin = p.Y
		}
		if p.Y > yMax {
			yMax = p.Y
		}
	}

	// Scan a grid for low-density regions.
	scanRes := 20
	xStep := (xMax - xMin) / float64(scanRes)
	yStep := (yMax - yMin) / float64(scanRes)
	if xStep < 1e-9 || yStep < 1e-9 {
		return nil
	}

	// Compute search radius based on average inter-point distance.
	avgDist := math.Sqrt((xMax-xMin)*(yMax-yMin)/float64(len(points))) * 0.5
	if avgDist < 1e-6 {
		avgDist = 1.0
	}

	// Density threshold: below this is considered a white space.
	densityThreshold := float64(len(points)) / float64(scanRes*scanRes) * 0.3

	whiteSpaces := make([]WhiteSpaceRegion, 0)
	wsIdx := 0

	for i := 0; i < scanRes; i++ {
		gx := xMin + (float64(i)+0.5)*xStep
		for j := 0; j < scanRes; j++ {
			gy := yMin + (float64(j)+0.5)*yStep

			// Count nearby points.
			nearby := 0
			nearestDomains := make(map[string]struct{})
			for _, p := range points {
				dist := math.Sqrt((gx-p.X)*(gx-p.X) + (gy-p.Y)*(gy-p.Y))
				if dist <= avgDist {
					nearby++
				}
				if dist <= avgDist*2 && p.TechDomain != "" {
					nearestDomains[p.TechDomain] = struct{}{}
				}
			}

			if float64(nearby) < densityThreshold {
				// Check it's not too far from any cluster (we want actionable white spaces).
				minClusterDist := math.MaxFloat64
				for _, c := range clusters {
					dist := math.Sqrt((gx-c.CenterX)*(gx-c.CenterX) + (gy-c.CenterY)*(gy-c.CenterY))
					if dist < minClusterDist {
						minClusterDist = dist
					}
				}

				// Only include white spaces that are reasonably close to existing clusters.
				maxRelevantDist := avgDist * 5.0
				if len(clusters) > 0 && minClusterDist > maxRelevantDist {
					continue
				}

				domains := make([]string, 0, len(nearestDomains))
				for d := range nearestDomains {
					domains = append(domains, d)
				}
				sort.Strings(domains)

				// Score: higher for regions closer to clusters but with lower density.
				score := 0.0
				if minClusterDist > 0 && minClusterDist < math.MaxFloat64 {
					score = 1.0 / (1.0 + minClusterDist/avgDist)
				}

				area := xStep * yStep
				whiteSpaces = append(whiteSpaces, WhiteSpaceRegion{
					RegionID:    fmt.Sprintf("ws-%d", wsIdx),
					CenterX:     gx,
					CenterY:     gy,
					Area:        area,
					NearestTech: domains,
					Score:       score,
				})
				wsIdx++
			}
		}
	}

	// Sort by score descending and limit to top 20.
	sort.Slice(whiteSpaces, func(i, j int) bool {
		return whiteSpaces[i].Score > whiteSpaces[j].Score
	})
	if len(whiteSpaces) > 20 {
		whiteSpaces = whiteSpaces[:20]
	}

	return whiteSpaces
}

// computeStats aggregates coverage statistics from constellation data.
func (s *constellationServiceImpl) computeStats(points []ConstellationPoint, clusters []ConstellationCluster, whiteSpaces []WhiteSpaceRegion) CoverageStatistics {
	stats := CoverageStatistics{
		TotalPoints:     len(points),
		ClusterCount:    len(clusters),
		WhiteSpaceCount: len(whiteSpaces),
	}

	for _, p := range points {
		switch p.PointType {
		case PointTypeOwnPatent:
			stats.OwnPatentCount++
		case PointTypeCompetitorPatent:
			stats.CompetitorCount++
		}
	}

	// Compute density statistics from clusters.
	if len(clusters) > 0 {
		sum := 0.0
		for _, c := range clusters {
			sum += c.Density
		}
		stats.DensityMean = sum / float64(len(clusters))

		variance := 0.0
		for _, c := range clusters {
			diff := c.Density - stats.DensityMean
			variance += diff * diff
		}
		stats.DensityStdDev = math.Sqrt(variance / float64(len(clusters)))
	}

	// Coverage ratio: fraction of scanned area covered by clusters vs total.
	if len(clusters) > 0 && len(whiteSpaces) > 0 {
		clusterArea := 0.0
		for _, c := range clusters {
			clusterArea += math.Pi * c.Radius * c.Radius
		}
		wsArea := 0.0
		for _, ws := range whiteSpaces {
			wsArea += ws.Area
		}
		totalArea := clusterArea + wsArea
		if totalArea > 0 {
			stats.CoverageRatio = clusterArea / totalArea
		}
	} else if len(clusters) > 0 {
		stats.CoverageRatio = 1.0
	}

	return stats
}

// -----------------------------------------------------------------------
// Package-Level Helper Functions
// -----------------------------------------------------------------------

// toStringSet converts a string slice to a set for O(1) lookups.
func toStringSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}

// resolveDomainName maps a domain code to a human-readable name.
// In production this would consult a classification database; here we provide a basic mapping.
func resolveDomainName(code string) string {
	knownDomains := map[string]string{
		"A61K": "Preparations for Medical Purposes",
		"A61P": "Therapeutic Activity of Chemical Compounds",
		"C07D": "Heterocyclic Compounds",
		"C07C": "Acyclic or Carbocyclic Compounds",
		"C07K": "Peptides",
		"C12N": "Microorganisms or Enzymes",
		"G01N": "Investigating or Analysing Materials",
		"G16B": "Bioinformatics",
		"unclassified": "Unclassified",
	}
	if name, ok := knownDomains[code]; ok {
		return name
	}
	return code
}

// filterByTechDomains filters patents to only those matching specified tech domains.
func filterByTechDomains(patents []domainpatent.Patent, domains []string) []domainpatent.Patent {
	domainSet := toStringSet(domains)
	filtered := make([]domainpatent.Patent, 0, len(patents))
	for _, p := range patents {
		if _, ok := domainSet[p.GetPrimaryTechDomain()]; ok {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// groupByDomain groups patents by their primary technology domain.
func groupByDomain(patents []domainpatent.Patent) map[string][]domainpatent.Patent {
	result := make(map[string][]domainpatent.Patent)
	for _, p := range patents {
		domain := p.GetPrimaryTechDomain()
		if domain == "" {
			domain = "unclassified"
		}
		result[domain] = append(result[domain], p)
	}
	return result
}

// mergeKeys returns a sorted, deduplicated list of all keys from two maps.
func mergeKeys(a, b map[string][]domainpatent.Patent) []string {
	seen := make(map[string]struct{})
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// computeZoneStrength calculates a strength score for a set of patents in a zone.
// The score considers patent count, recency, and value scores.
func computeZoneStrength(patents []domainpatent.Patent) float64 {
	if len(patents) == 0 {
		return 0.0
	}

	now := time.Now()
	totalScore := 0.0

	for _, p := range patents {
		// Base contribution from value score.
		value := p.GetValueScore()
		if value <= 0 {
			value = 1.0
		}

		// Recency bonus: patents filed more recently get a higher weight.
		recencyFactor := 1.0
		if !p.GetFilingDate().IsZero() {
			ageYears := now.Sub(p.GetFilingDate()).Hours() / (24 * 365.25)
			if ageYears < 20 {
				recencyFactor = 1.0 + (20.0-ageYears)/20.0 // Range [1.0, 2.0]
			}
		}

		totalScore += value * recencyFactor
	}

	// Normalize by count to get per-patent strength, then scale by log(count) for portfolio breadth.
	perPatent := totalScore / float64(len(patents))
	breadthFactor := 1.0 + math.Log1p(float64(len(patents)))

	return perPatent * breadthFactor
}

// computeStrengthIndex calculates an overall competitive strength index.
// Positive values indicate own advantage, negative values indicate competitor advantage.
// Range is approximately [-1.0, 1.0].
func computeStrengthIndex(ownPatents, compPatents []domainpatent.Patent, overlapZones []OverlapZone) float64 {
	ownCount := float64(len(ownPatents))
	compCount := float64(len(compPatents))

	if ownCount == 0 && compCount == 0 {
		return 0.0
	}

	// Component 1: Volume ratio.
	total := ownCount + compCount
	volumeRatio := (ownCount - compCount) / total // Range [-1, 1]

	// Component 2: Overlap dominance — in overlap zones, who has more patents?
	overlapDominance := 0.0
	if len(overlapZones) > 0 {
		ownOverlapTotal := 0
		compOverlapTotal := 0
		for _, oz := range overlapZones {
			ownOverlapTotal += oz.OwnCount
			compOverlapTotal += oz.CompCount
		}
		overlapTotal := float64(ownOverlapTotal + compOverlapTotal)
		if overlapTotal > 0 {
			overlapDominance = (float64(ownOverlapTotal) - float64(compOverlapTotal)) / overlapTotal
		}
	}

	// Component 3: Value-weighted strength.
	ownValueSum := 0.0
	for _, p := range ownPatents {
		v := p.GetValueScore()
		if v > 0 {
			ownValueSum += v
		}
	}
	compValueSum := 0.0
	for _, p := range compPatents {
		v := p.GetValueScore()
		if v > 0 {
			compValueSum += v
		}
	}
	valueRatio := 0.0
	totalValue := ownValueSum + compValueSum
	if totalValue > 0 {
		valueRatio = (ownValueSum - compValueSum) / totalValue
	}

	// Weighted combination.
	index := 0.4*volumeRatio + 0.3*overlapDominance + 0.3*valueRatio

	// Clamp to [-1, 1].
	if index > 1.0 {
		index = 1.0
	}
	if index < -1.0 {
		index = -1.0
	}

	return index
}

// computeBoundingBox returns the min/max X and Y values from a set of 2D+ coordinate vectors.
func computeBoundingBox(points [][]float64) (xMin, xMax, yMin, yMax float64) {
	if len(points) == 0 {
		return 0, 0, 0, 0
	}

	first := points[0]
	if len(first) >= 2 {
		xMin, xMax = first[0], first[0]
		yMin, yMax = first[1], first[1]
	}

	for _, pt := range points[1:] {
		if len(pt) < 2 {
			continue
		}
		if pt[0] < xMin {
			xMin = pt[0]
		}
		if pt[0] > xMax {
			xMax = pt[0]
		}
		if pt[1] < yMin {
			yMin = pt[1]
		}
		if pt[1] > yMax {
			yMax = pt[1]
		}
	}

	return xMin, xMax, yMin, yMax
}

// Ensure unused imports are referenced (compile guard).
var (
	_ commontypes.ID = ""
)

//Personal.AI order the ending


