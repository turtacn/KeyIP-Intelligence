// Phase 10 - File 200 of 349
package infringement

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	kafkainfra "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// CompetitorStatus represents the tracking state of a competitor.
type CompetitorStatus int

const (
	CompetitorStatusActive CompetitorStatus = iota + 1
	CompetitorStatusPaused
	CompetitorStatusArchived
)

// String returns the human-readable representation.
func (s CompetitorStatus) String() string {
	switch s {
	case CompetitorStatusActive:
		return "ACTIVE"
	case CompetitorStatusPaused:
		return "PAUSED"
	case CompetitorStatusArchived:
		return "ARCHIVED"
	default:
		return "UNKNOWN"
	}
}

// TrackedCompetitor represents a competitor entity being monitored.
type TrackedCompetitor struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Aliases          []string         `json:"aliases,omitempty"`
	Status           CompetitorStatus `json:"status"`
	WatchlistID      string           `json:"watchlist_id"`
	TechnologyAreas  []string         `json:"technology_areas,omitempty"`
	PatentCount      int              `json:"patent_count"`
	RecentFilings    int              `json:"recent_filings"`
	LastScanAt       *time.Time       `json:"last_scan_at,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	Metadata         map[string]any   `json:"metadata,omitempty"`
}

// TrackCompetitorRequest carries the parameters for adding a new competitor to track.
type TrackCompetitorRequest struct {
	Name            string   `json:"name" validate:"required,max=200"`
	Aliases         []string `json:"aliases,omitempty"`
	WatchlistID     string   `json:"watchlist_id" validate:"required"`
	TechnologyAreas []string `json:"technology_areas,omitempty"`
}

// Validate checks the structural integrity of a TrackCompetitorRequest.
func (r *TrackCompetitorRequest) Validate() error {
	if r.Name == "" {
		return errors.NewValidation("competitor name is required")
	}
	if len(r.Name) > 200 {
		return errors.NewValidation("competitor name exceeds 200 characters")
	}
	if r.WatchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}
	return nil
}

// CompetitorPortfolioAnalysis holds the result of analyzing a competitor's patent portfolio.
type CompetitorPortfolioAnalysis struct {
	CompetitorID        string               `json:"competitor_id"`
	CompetitorName      string               `json:"competitor_name"`
	TotalPatents        int                  `json:"total_patents"`
	ActivePatents       int                  `json:"active_patents"`
	ExpiredPatents      int                  `json:"expired_patents"`
	PendingPatents      int                  `json:"pending_patents"`
	FilingVelocity      float64              `json:"filing_velocity"`
	TechnologyBreakdown map[string]int       `json:"technology_breakdown"`
	TopIPCClasses       []IPCClassCount      `json:"top_ipc_classes"`
	FilingTrend         []MonthlyFilingCount `json:"filing_trend"`
	AnalyzedAt          time.Time            `json:"analyzed_at"`
}

// IPCClassCount pairs an IPC classification code with its occurrence count.
type IPCClassCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// MonthlyFilingCount pairs a year-month with the number of filings.
type MonthlyFilingCount struct {
	YearMonth string `json:"year_month"`
	Count     int    `json:"count"`
}

// NewFilingDetection represents a newly detected patent filing by a tracked competitor.
type NewFilingDetection struct {
	CompetitorID   string    `json:"competitor_id"`
	CompetitorName string    `json:"competitor_name"`
	PatentNumber   string    `json:"patent_number"`
	Title          string    `json:"title"`
	FilingDate     time.Time `json:"filing_date"`
	IPCClasses     []string  `json:"ipc_classes,omitempty"`
	DetectedAt     time.Time `json:"detected_at"`
}

// CompetitiveLandscape represents the competitive intelligence overview for a technology area.
type CompetitiveLandscape struct {
	TechnologyArea   string                   `json:"technology_area"`
	TotalCompetitors int                      `json:"total_competitors"`
	TotalPatents     int                      `json:"total_patents"`
	TopFilers        []CompetitorFilingSummary `json:"top_filers"`
	TrendDirection   string                   `json:"trend_direction"`
	AnalyzedAt       time.Time                `json:"analyzed_at"`
}

// CompetitorFilingSummary provides a brief filing summary for a competitor.
type CompetitorFilingSummary struct {
	CompetitorID   string  `json:"competitor_id"`
	CompetitorName string  `json:"competitor_name"`
	PatentCount    int     `json:"patent_count"`
	MarketShare    float64 `json:"market_share"`
}

// PortfolioComparison holds the result of comparing two competitor portfolios.
type PortfolioComparison struct {
	CompetitorA      string   `json:"competitor_a"`
	CompetitorB      string   `json:"competitor_b"`
	OverlappingAreas []string `json:"overlapping_areas"`
	UniqueToA        []string `json:"unique_to_a"`
	UniqueToB        []string `json:"unique_to_b"`
	PatentCountA     int      `json:"patent_count_a"`
	PatentCountB     int      `json:"patent_count_b"`
	FilingVelocityA  float64  `json:"filing_velocity_a"`
	FilingVelocityB  float64  `json:"filing_velocity_b"`
	ComparedAt       time.Time `json:"compared_at"`
}

// CompetitorListOptions carries filtering parameters for listing tracked competitors.
type CompetitorListOptions struct {
	WatchlistID    string            `json:"watchlist_id,omitempty"`
	Status         *CompetitorStatus `json:"status,omitempty"`
	TechnologyArea string            `json:"technology_area,omitempty"`
	Page           int               `json:"page"`
	PageSize       int               `json:"page_size"`
}

// CompetitorRepository defines the persistence contract for tracked competitors.
type CompetitorRepository interface {
	Save(ctx context.Context, competitor *TrackedCompetitor) error
	FindByID(ctx context.Context, id string) (*TrackedCompetitor, error)
	Update(ctx context.Context, competitor *TrackedCompetitor) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, opts CompetitorListOptions) ([]*TrackedCompetitor, int, error)
	FindByName(ctx context.Context, name string, watchlistID string) (*TrackedCompetitor, error)
}

// CompetitorTrackingService defines the application-level contract for competitor tracking.
type CompetitorTrackingService interface {
	TrackCompetitor(ctx context.Context, req *TrackCompetitorRequest) (*TrackedCompetitor, error)
	RemoveCompetitor(ctx context.Context, competitorID string) error
	ListTrackedCompetitors(ctx context.Context, opts CompetitorListOptions) ([]*TrackedCompetitor, int, error)
	GetCompetitorProfile(ctx context.Context, competitorID string) (*TrackedCompetitor, error)
	AnalyzeCompetitorPortfolio(ctx context.Context, competitorID string) (*CompetitorPortfolioAnalysis, error)
	DetectNewFilings(ctx context.Context, competitorID string) ([]*NewFilingDetection, error)
	GetCompetitiveLandscape(ctx context.Context, technologyArea string) (*CompetitiveLandscape, error)
	ComparePortfolios(ctx context.Context, competitorAID, competitorBID string) (*PortfolioComparison, error)
}

// competitorTrackingServiceImpl is the concrete implementation.
type competitorTrackingServiceImpl struct {
	competitorRepo CompetitorRepository
	producer       kafkainfra.Producer
	cache          redis.Cache
	logger         logging.Logger
	mu             sync.RWMutex
}

// NewCompetitorTrackingService constructs a new CompetitorTrackingService.
func NewCompetitorTrackingService(
	competitorRepo CompetitorRepository,
	producer kafkainfra.Producer,
	cache redis.Cache,
	logger logging.Logger,
) CompetitorTrackingService {
	return &competitorTrackingServiceImpl{
		competitorRepo: competitorRepo,
		producer:       producer,
		cache:          cache,
		logger:         logger,
	}
}

// TrackCompetitor adds a new competitor to the tracking list.
func (s *competitorTrackingServiceImpl) TrackCompetitor(ctx context.Context, req *TrackCompetitorRequest) (*TrackedCompetitor, error) {
	if req == nil {
		return nil, errors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	existing, err := s.competitorRepo.FindByName(ctx, req.Name, req.WatchlistID)
	if err != nil {
		return nil, errors.NewInternal("failed to check existing competitor: %v", err)
	}
	if existing != nil {
		if existing.Status == CompetitorStatusArchived {
			existing.Status = CompetitorStatusActive
			existing.UpdatedAt = time.Now().UTC()
			if err := s.competitorRepo.Update(ctx, existing); err != nil {
				return nil, errors.NewInternal("failed to reactivate competitor: %v", err)
			}
			s.logger.Info("competitor reactivated", "id", existing.ID, "name", existing.Name)
			return existing, nil
		}
		return existing, nil
	}

	now := time.Now().UTC()
	competitor := &TrackedCompetitor{
		ID:              generateCompetitorID(req.Name, req.WatchlistID, now),
		Name:            req.Name,
		Aliases:         req.Aliases,
		Status:          CompetitorStatusActive,
		WatchlistID:     req.WatchlistID,
		TechnologyAreas: req.TechnologyAreas,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.competitorRepo.Save(ctx, competitor); err != nil {
		return nil, errors.NewInternal("failed to save competitor: %v", err)
	}

	_ = s.producer.Publish(ctx, "competitor.tracked", competitor.ID, map[string]any{
		"competitor_id": competitor.ID,
		"name":          competitor.Name,
		"watchlist_id":  competitor.WatchlistID,
		"timestamp":     now.Format(time.RFC3339),
	})

	s.logger.Info("competitor tracked", "id", competitor.ID, "name", competitor.Name)
	return competitor, nil
}

// RemoveCompetitor archives a tracked competitor.
func (s *competitorTrackingServiceImpl) RemoveCompetitor(ctx context.Context, competitorID string) error {
	if competitorID == "" {
		return errors.NewValidation("competitor_id is required")
	}

	competitor, err := s.competitorRepo.FindByID(ctx, competitorID)
	if err != nil {
		return errors.NewInternal("failed to find competitor: %v", err)
	}
	if competitor == nil {
		return errors.NewNotFound("competitor %s not found", competitorID)
	}

	competitor.Status = CompetitorStatusArchived
	competitor.UpdatedAt = time.Now().UTC()

	if err := s.competitorRepo.Update(ctx, competitor); err != nil {
		return errors.NewInternal("failed to archive competitor: %v", err)
	}

	s.logger.Info("competitor archived", "id", competitorID, "name", competitor.Name)
	return nil
}

// ListTrackedCompetitors returns a filtered list of tracked competitors.
func (s *competitorTrackingServiceImpl) ListTrackedCompetitors(ctx context.Context, opts CompetitorListOptions) ([]*TrackedCompetitor, int, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}

	return s.competitorRepo.List(ctx, opts)
}

// GetCompetitorProfile retrieves a single competitor's profile.
func (s *competitorTrackingServiceImpl) GetCompetitorProfile(ctx context.Context, competitorID string) (*TrackedCompetitor, error) {
	if competitorID == "" {
		return nil, errors.NewValidation("competitor_id is required")
	}

	competitor, err := s.competitorRepo.FindByID(ctx, competitorID)
	if err != nil {
		return nil, errors.NewInternal("failed to find competitor: %v", err)
	}
	if competitor == nil {
		return nil, errors.NewNotFound("competitor %s not found", competitorID)
	}

	return competitor, nil
}

// AnalyzeCompetitorPortfolio performs a comprehensive analysis of a competitor's patent portfolio.
func (s *competitorTrackingServiceImpl) AnalyzeCompetitorPortfolio(ctx context.Context, competitorID string) (*CompetitorPortfolioAnalysis, error) {
	if competitorID == "" {
		return nil, errors.NewValidation("competitor_id is required")
	}

	competitor, err := s.competitorRepo.FindByID(ctx, competitorID)
	if err != nil {
		return nil, errors.NewInternal("failed to find competitor: %v", err)
	}
	if competitor == nil {
		return nil, errors.NewNotFound("competitor %s not found", competitorID)
	}

	cacheKey := fmt.Sprintf("competitor_portfolio:%s", competitorID)
	var cached CompetitorPortfolioAnalysis
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	analysis := &CompetitorPortfolioAnalysis{
		CompetitorID:        competitorID,
		CompetitorName:      competitor.Name,
		TotalPatents:        competitor.PatentCount,
		TechnologyBreakdown: make(map[string]int),
		TopIPCClasses:       make([]IPCClassCount, 0),
		FilingTrend:         make([]MonthlyFilingCount, 0),
		AnalyzedAt:          time.Now().UTC(),
	}

	for _, area := range competitor.TechnologyAreas {
		analysis.TechnologyBreakdown[area] = 0
	}

	if competitor.PatentCount > 0 {
		monthsSinceCreation := time.Since(competitor.CreatedAt).Hours() / (24 * 30)
		if monthsSinceCreation > 0 {
			analysis.FilingVelocity = float64(competitor.PatentCount) / monthsSinceCreation
		}
	}

	_ = s.cache.Set(ctx, cacheKey, analysis, 30*time.Minute)

	s.logger.Info("competitor portfolio analyzed", "id", competitorID, "total_patents", analysis.TotalPatents)
	return analysis, nil
}

// DetectNewFilings scans for recently filed patents by a tracked competitor.
func (s *competitorTrackingServiceImpl) DetectNewFilings(ctx context.Context, competitorID string) ([]*NewFilingDetection, error) {
	if competitorID == "" {
		return nil, errors.NewValidation("competitor_id is required")
	}

	competitor, err := s.competitorRepo.FindByID(ctx, competitorID)
	if err != nil {
		return nil, errors.NewInternal("failed to find competitor: %v", err)
	}
	if competitor == nil {
		return nil, errors.NewNotFound("competitor %s not found", competitorID)
	}

	if competitor.Status != CompetitorStatusActive {
		return nil, errors.NewValidation("competitor %s is not active", competitorID)
	}

	scanSince := time.Now().UTC().Add(-30 * 24 * time.Hour)
	if competitor.LastScanAt != nil {
		scanSince = *competitor.LastScanAt
	}

	// In production, query external patent databases here.
	detections := make([]*NewFilingDetection, 0)

	now := time.Now().UTC()
	competitor.LastScanAt = &now
	competitor.UpdatedAt = now
	if err := s.competitorRepo.Update(ctx, competitor); err != nil {
		s.logger.Error("failed to update last scan time", "error", err, "competitor_id", competitorID)
	}

	for _, d := range detections {
		_ = s.producer.Publish(ctx, "competitor.new_filing", d.PatentNumber, map[string]any{
			"competitor_id": d.CompetitorID,
			"patent_number": d.PatentNumber,
			"detected_at":   d.DetectedAt.Format(time.RFC3339),
		})
	}

	s.logger.Info("new filing detection complete",
		"competitor_id", competitorID, "scan_since", scanSince.Format(time.RFC3339),
		"detections", len(detections))

	return detections, nil
}

// GetCompetitiveLandscape builds a competitive intelligence overview for a technology area.
func (s *competitorTrackingServiceImpl) GetCompetitiveLandscape(ctx context.Context, technologyArea string) (*CompetitiveLandscape, error) {
	if technologyArea == "" {
		return nil, errors.NewValidation("technology_area is required")
	}

	cacheKey := fmt.Sprintf("competitive_landscape:%s", technologyArea)
	var cached CompetitiveLandscape
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	activeStatus := CompetitorStatusActive
	competitors, _, err := s.competitorRepo.List(ctx, CompetitorListOptions{
		Status:         &activeStatus,
		TechnologyArea: technologyArea,
		Page:           1,
		PageSize:       100,
	})
	if err != nil {
		return nil, errors.NewInternal("failed to list competitors for landscape: %v", err)
	}

	totalPatents := 0
	summaries := make([]CompetitorFilingSummary, 0, len(competitors))
	for _, c := range competitors {
		totalPatents += c.PatentCount
		summaries = append(summaries, CompetitorFilingSummary{
			CompetitorID:   c.ID,
			CompetitorName: c.Name,
			PatentCount:    c.PatentCount,
		})
	}

	if totalPatents > 0 {
		for i := range summaries {
			summaries[i].MarketShare = float64(summaries[i].PatentCount) / float64(totalPatents) * 100
		}
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].PatentCount > summaries[j].PatentCount
	})

	if len(summaries) > 20 {
		summaries = summaries[:20]
	}

	trendDirection := "stable"
	recentActive := 0
	for _, c := range competitors {
		if c.RecentFilings > 0 {
			recentActive++
		}
	}
	if len(competitors) > 0 {
		activeRatio := float64(recentActive) / float64(len(competitors))
		if activeRatio > 0.6 {
			trendDirection = "increasing"
		} else if activeRatio < 0.3 {
			trendDirection = "decreasing"
		}
	}

	landscape := &CompetitiveLandscape{
		TechnologyArea:   technologyArea,
		TotalCompetitors: len(competitors),
		TotalPatents:     totalPatents,
		TopFilers:        summaries,
		TrendDirection:   trendDirection,
		AnalyzedAt:       time.Now().UTC(),
	}

	_ = s.cache.Set(ctx, cacheKey, landscape, 1*time.Hour)

	s.logger.Info("competitive landscape generated",
		"area", technologyArea, "competitors", len(competitors), "patents", totalPatents)

	return landscape, nil
}

// ComparePortfolios compares the patent portfolios of two tracked competitors.
func (s *competitorTrackingServiceImpl) ComparePortfolios(ctx context.Context, competitorAID, competitorBID string) (*PortfolioComparison, error) {
	if competitorAID == "" || competitorBID == "" {
		return nil, errors.NewValidation("both competitor IDs are required")
	}
	if competitorAID == competitorBID {
		return nil, errors.NewValidation("cannot compare a competitor with itself")
	}

	compA, err := s.competitorRepo.FindByID(ctx, competitorAID)
	if err != nil {
		return nil, errors.NewInternal("failed to find competitor A: %v", err)
	}
	if compA == nil {
		return nil, errors.NewNotFound("competitor %s not found", competitorAID)
	}

	compB, err := s.competitorRepo.FindByID(ctx, competitorBID)
	if err != nil {
		return nil, errors.NewInternal("failed to find competitor B: %v", err)
	}
	if compB == nil {
		return nil, errors.NewNotFound("competitor %s not found", competitorBID)
	}

	setA := make(map[string]bool)
	for _, area := range compA.TechnologyAreas {
		setA[area] = true
	}
	setB := make(map[string]bool)
	for _, area := range compB.TechnologyAreas {
		setB[area] = true
	}

	var overlapping, uniqueToA, uniqueToB []string
	for area := range setA {
		if setB[area] {
			overlapping = append(overlapping, area)
		} else {
			uniqueToA = append(uniqueToA, area)
		}
	}
	for area := range setB {
		if !setA[area] {
			uniqueToB = append(uniqueToB, area)
		}
	}

	sort.Strings(overlapping)
	sort.Strings(uniqueToA)
	sort.Strings(uniqueToB)

	calcVelocity := func(patentCount int, createdAt time.Time) float64 {
		if patentCount <= 0 {
			return 0
		}
		months := time.Since(createdAt).Hours() / (24 * 30)
		if months <= 0 {
			return 0
		}
		return float64(patentCount) / months
	}

	comparison := &PortfolioComparison{
		CompetitorA:      compA.Name,
		CompetitorB:      compB.Name,
		OverlappingAreas: overlapping,
		UniqueToA:        uniqueToA,
		UniqueToB:        uniqueToB,
		PatentCountA:     compA.PatentCount,
		PatentCountB:     compB.PatentCount,
		FilingVelocityA:  calcVelocity(compA.PatentCount, compA.CreatedAt),
		FilingVelocityB:  calcVelocity(compB.PatentCount, compB.CreatedAt),
		ComparedAt:       time.Now().UTC(),
	}

	s.logger.Info("portfolio comparison complete",
		"competitor_a", compA.Name, "competitor_b", compB.Name,
		"overlapping_areas", len(overlapping))

	return comparison, nil
}

// generateCompetitorID produces a unique competitor identifier.
func generateCompetitorID(name, watchlistID string, ts time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", name, watchlistID, ts.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("CMP-%x", hash[:6])
}

//Personal.AI order the ending

