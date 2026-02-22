// Phase 10 - File 202 of 349
// Functionality: Infringement monitoring application service — orchestrates continuous
// patent infringement surveillance including watchlist management, scheduled scans,
// similarity-based detection, and integration with alert and competitor tracking services.

package infringement

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	kafkainfra "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// WatchlistStatus represents the state of a monitoring watchlist.
type WatchlistStatus int

const (
	WatchlistStatusActive WatchlistStatus = iota + 1
	WatchlistStatusPaused
	WatchlistStatusArchived
)

// String returns the human-readable representation.
func (s WatchlistStatus) String() string {
	switch s {
	case WatchlistStatusActive:
		return "ACTIVE"
	case WatchlistStatusPaused:
		return "PAUSED"
	case WatchlistStatusArchived:
		return "ARCHIVED"
	default:
		return "UNKNOWN"
	}
}

// ScanFrequency defines how often a watchlist is scanned.
type ScanFrequency int

const (
	ScanFrequencyDaily ScanFrequency = iota + 1
	ScanFrequencyWeekly
	ScanFrequencyBiWeekly
	ScanFrequencyMonthly
)

// String returns the human-readable representation.
func (f ScanFrequency) String() string {
	switch f {
	case ScanFrequencyDaily:
		return "DAILY"
	case ScanFrequencyWeekly:
		return "WEEKLY"
	case ScanFrequencyBiWeekly:
		return "BI_WEEKLY"
	case ScanFrequencyMonthly:
		return "MONTHLY"
	default:
		return "UNKNOWN"
	}
}

// Duration returns the time interval between scans.
func (f ScanFrequency) Duration() time.Duration {
	switch f {
	case ScanFrequencyDaily:
		return 24 * time.Hour
	case ScanFrequencyWeekly:
		return 7 * 24 * time.Hour
	case ScanFrequencyBiWeekly:
		return 14 * 24 * time.Hour
	case ScanFrequencyMonthly:
		return 30 * 24 * time.Hour
	default:
		return 7 * 24 * time.Hour
	}
}

// Watchlist represents a monitoring watchlist containing patents and molecules to monitor.
type Watchlist struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Description       string          `json:"description,omitempty"`
	OwnerID           string          `json:"owner_id"`
	Status            WatchlistStatus `json:"status"`
	ScanFrequency     ScanFrequency   `json:"scan_frequency"`
	SimilarityThreshold float64       `json:"similarity_threshold"`
	PatentNumbers     []string        `json:"patent_numbers"`
	MoleculeIDs       []string        `json:"molecule_ids"`
	LastScanAt        *time.Time      `json:"last_scan_at,omitempty"`
	NextScanAt        *time.Time      `json:"next_scan_at,omitempty"`
	TotalScans        int            `json:"total_scans"`
	TotalAlerts       int            `json:"total_alerts"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// CreateWatchlistRequest carries the parameters for creating a new watchlist.
type CreateWatchlistRequest struct {
	Name                string        `json:"name" validate:"required,max=200"`
	Description         string        `json:"description,omitempty"`
	OwnerID             string        `json:"owner_id" validate:"required"`
	ScanFrequency       ScanFrequency `json:"scan_frequency"`
	SimilarityThreshold float64       `json:"similarity_threshold"`
	PatentNumbers       []string      `json:"patent_numbers,omitempty"`
	MoleculeIDs         []string      `json:"molecule_ids,omitempty"`
}

// Validate checks the structural integrity of a CreateWatchlistRequest.
func (r *CreateWatchlistRequest) Validate() error {
	if r.Name == "" {
		return errors.NewValidation("watchlist name is required")
	}
	if len(r.Name) > 200 {
		return errors.NewValidation("watchlist name exceeds 200 characters")
	}
	if r.OwnerID == "" {
		return errors.NewValidation("owner_id is required")
	}
	if r.SimilarityThreshold < 0 || r.SimilarityThreshold > 1 {
		return errors.NewValidation("similarity_threshold must be between 0 and 1")
	}
	if r.ScanFrequency < ScanFrequencyDaily || r.ScanFrequency > ScanFrequencyMonthly {
		return errors.NewValidation("invalid scan_frequency")
	}
	if len(r.PatentNumbers) == 0 && len(r.MoleculeIDs) == 0 {
		return errors.NewValidation("at least one patent_number or molecule_id is required")
	}
	return nil
}

// UpdateWatchlistRequest carries the parameters for updating a watchlist.
type UpdateWatchlistRequest struct {
	WatchlistID         string         `json:"watchlist_id" validate:"required"`
	Name                *string        `json:"name,omitempty"`
	Description         *string        `json:"description,omitempty"`
	ScanFrequency       *ScanFrequency `json:"scan_frequency,omitempty"`
	SimilarityThreshold *float64       `json:"similarity_threshold,omitempty"`
	Status              *WatchlistStatus `json:"status,omitempty"`
}

// Validate checks the structural integrity of an UpdateWatchlistRequest.
func (r *UpdateWatchlistRequest) Validate() error {
	if r.WatchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}
	if r.SimilarityThreshold != nil && (*r.SimilarityThreshold < 0 || *r.SimilarityThreshold > 1) {
		return errors.NewValidation("similarity_threshold must be between 0 and 1")
	}
	if r.ScanFrequency != nil && (*r.ScanFrequency < ScanFrequencyDaily || *r.ScanFrequency > ScanFrequencyMonthly) {
		return errors.NewValidation("invalid scan_frequency")
	}
	return nil
}

// ScanResult represents the outcome of a single monitoring scan.
type ScanResult struct {
	ScanID        string          `json:"scan_id"`
	WatchlistID   string          `json:"watchlist_id"`
	StartedAt     time.Time       `json:"started_at"`
	CompletedAt   time.Time       `json:"completed_at"`
	Duration      time.Duration   `json:"duration"`
	PatentsScanned int            `json:"patents_scanned"`
	MoleculesScanned int          `json:"molecules_scanned"`
	MatchesFound  int             `json:"matches_found"`
	AlertsCreated int             `json:"alerts_created"`
	Matches       []ScanMatch     `json:"matches,omitempty"`
	Error         string          `json:"error,omitempty"`
}

// ScanMatch represents a single match found during a scan.
type ScanMatch struct {
	PatentNumber    string  `json:"patent_number"`
	MoleculeID      string  `json:"molecule_id"`
	SimilarityScore float64 `json:"similarity_score"`
	RiskScore       float64 `json:"risk_score"`
	MatchType       string  `json:"match_type"` // "structural", "functional", "sequence"
}

// WatchlistListOptions carries filtering parameters for listing watchlists.
type WatchlistListOptions struct {
	OwnerID  string           `json:"owner_id,omitempty"`
	Status   *WatchlistStatus `json:"status,omitempty"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// WatchlistRepository defines the persistence contract for watchlists.
type WatchlistRepository interface {
	Save(ctx context.Context, watchlist *Watchlist) error
	FindByID(ctx context.Context, id string) (*Watchlist, error)
	Update(ctx context.Context, watchlist *Watchlist) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, opts WatchlistListOptions) ([]*Watchlist, int, error)
	FindDueForScan(ctx context.Context, before time.Time) ([]*Watchlist, error)
}

// ScanResultRepository defines the persistence contract for scan results.
type ScanResultRepository interface {
	Save(ctx context.Context, result *ScanResult) error
	FindByWatchlistID(ctx context.Context, watchlistID string, limit int) ([]*ScanResult, error)
}

// MonitoringService defines the application-level contract for infringement monitoring.
type MonitoringService interface {
	CreateWatchlist(ctx context.Context, req *CreateWatchlistRequest) (*Watchlist, error)
	UpdateWatchlist(ctx context.Context, req *UpdateWatchlistRequest) (*Watchlist, error)
	DeleteWatchlist(ctx context.Context, watchlistID string) error
	GetWatchlist(ctx context.Context, watchlistID string) (*Watchlist, error)
	ListWatchlists(ctx context.Context, opts WatchlistListOptions) ([]*Watchlist, int, error)
	AddPatentsToWatchlist(ctx context.Context, watchlistID string, patentNumbers []string) error
	AddMoleculesToWatchlist(ctx context.Context, watchlistID string, moleculeIDs []string) error
	RemovePatentsFromWatchlist(ctx context.Context, watchlistID string, patentNumbers []string) error
	RemoveMoleculesFromWatchlist(ctx context.Context, watchlistID string, moleculeIDs []string) error
	RunScan(ctx context.Context, watchlistID string) (*ScanResult, error)
	RunScheduledScans(ctx context.Context) (int, error)
	GetScanHistory(ctx context.Context, watchlistID string, limit int) ([]*ScanResult, error)
}

// monitoringServiceImpl is the concrete implementation.
type monitoringServiceImpl struct {
	watchlistRepo  WatchlistRepository
	scanResultRepo ScanResultRepository
	alertService   AlertService
	producer       kafkainfra.Producer
	cache          redis.Cache
	logger         logging.Logger
	mu             sync.RWMutex
}

// NewMonitoringService constructs a new MonitoringService.
func NewMonitoringService(
	watchlistRepo WatchlistRepository,
	scanResultRepo ScanResultRepository,
	alertService AlertService,
	producer kafkainfra.Producer,
	cache redis.Cache,
	logger logging.Logger,
) MonitoringService {
	return &monitoringServiceImpl{
		watchlistRepo:  watchlistRepo,
		scanResultRepo: scanResultRepo,
		alertService:   alertService,
		producer:       producer,
		cache:          cache,
		logger:         logger,
	}
}

// CreateWatchlist creates a new monitoring watchlist.
func (s *monitoringServiceImpl) CreateWatchlist(ctx context.Context, req *CreateWatchlistRequest) (*Watchlist, error) {
	if req == nil {
		return nil, errors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if req.SimilarityThreshold == 0 {
		req.SimilarityThreshold = 0.8
	}
	if req.ScanFrequency == 0 {
		req.ScanFrequency = ScanFrequencyWeekly
	}

	now := time.Now().UTC()
	nextScan := now.Add(req.ScanFrequency.Duration())

	watchlist := &Watchlist{
		ID:                  generateWatchlistID(req.Name, req.OwnerID, now),
		Name:                req.Name,
		Description:         req.Description,
		OwnerID:             req.OwnerID,
		Status:              WatchlistStatusActive,
		ScanFrequency:       req.ScanFrequency,
		SimilarityThreshold: req.SimilarityThreshold,
		PatentNumbers:       req.PatentNumbers,
		MoleculeIDs:         req.MoleculeIDs,
		NextScanAt:          &nextScan,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := s.watchlistRepo.Save(ctx, watchlist); err != nil {
		return nil, errors.NewInternal("failed to save watchlist: %v", err)
	}

	_ = s.producer.Publish(ctx, "monitoring.watchlist.created", watchlist.ID, map[string]any{
		"watchlist_id": watchlist.ID,
		"name":         watchlist.Name,
		"owner_id":     watchlist.OwnerID,
		"timestamp":    now.Format(time.RFC3339),
	})

	s.logger.Info("watchlist created", "id", watchlist.ID, "name", watchlist.Name)
	return watchlist, nil
}

// UpdateWatchlist updates an existing watchlist.
func (s *monitoringServiceImpl) UpdateWatchlist(ctx context.Context, req *UpdateWatchlistRequest) (*Watchlist, error) {
	if req == nil {
		return nil, errors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, req.WatchlistID)
	if err != nil {
		return nil, errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return nil, errors.NewNotFound("watchlist %s not found", req.WatchlistID)
	}

	if req.Name != nil {
		watchlist.Name = *req.Name
	}
	if req.Description != nil {
		watchlist.Description = *req.Description
	}
	if req.ScanFrequency != nil {
		watchlist.ScanFrequency = *req.ScanFrequency
		nextScan := time.Now().UTC().Add(watchlist.ScanFrequency.Duration())
		watchlist.NextScanAt = &nextScan
	}
	if req.SimilarityThreshold != nil {
		watchlist.SimilarityThreshold = *req.SimilarityThreshold
	}
	if req.Status != nil {
		watchlist.Status = *req.Status
	}

	watchlist.UpdatedAt = time.Now().UTC()

	if err := s.watchlistRepo.Update(ctx, watchlist); err != nil {
		return nil, errors.NewInternal("failed to update watchlist: %v", err)
	}

	s.logger.Info("watchlist updated", "id", watchlist.ID)
	return watchlist, nil
}

// DeleteWatchlist archives a watchlist.
func (s *monitoringServiceImpl) DeleteWatchlist(ctx context.Context, watchlistID string) error {
	if watchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return errors.NewNotFound("watchlist %s not found", watchlistID)
	}

	watchlist.Status = WatchlistStatusArchived
	watchlist.UpdatedAt = time.Now().UTC()

	if err := s.watchlistRepo.Update(ctx, watchlist); err != nil {
		return errors.NewInternal("failed to archive watchlist: %v", err)
	}

	s.logger.Info("watchlist archived", "id", watchlistID)
	return nil
}

// GetWatchlist retrieves a single watchlist by ID.
func (s *monitoringServiceImpl) GetWatchlist(ctx context.Context, watchlistID string) (*Watchlist, error) {
	if watchlistID == "" {
		return nil, errors.NewValidation("watchlist_id is required")
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return nil, errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return nil, errors.NewNotFound("watchlist %s not found", watchlistID)
	}

	return watchlist, nil
}

// ListWatchlists returns a filtered list of watchlists.
func (s *monitoringServiceImpl) ListWatchlists(ctx context.Context, opts WatchlistListOptions) ([]*Watchlist, int, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}

	return s.watchlistRepo.List(ctx, opts)
}

// AddPatentsToWatchlist adds patent numbers to a watchlist.
func (s *monitoringServiceImpl) AddPatentsToWatchlist(ctx context.Context, watchlistID string, patentNumbers []string) error {
	if watchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}
	if len(patentNumbers) == 0 {
		return errors.NewValidation("at least one patent_number is required")
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return errors.NewNotFound("watchlist %s not found", watchlistID)
	}

	existing := make(map[string]bool)
	for _, pn := range watchlist.PatentNumbers {
		existing[pn] = true
	}
	for _, pn := range patentNumbers {
		if !existing[pn] {
			watchlist.PatentNumbers = append(watchlist.PatentNumbers, pn)
			existing[pn] = true
		}
	}

	watchlist.UpdatedAt = time.Now().UTC()
	return s.watchlistRepo.Update(ctx, watchlist)
}

// AddMoleculesToWatchlist adds molecule IDs to a watchlist.
func (s *monitoringServiceImpl) AddMoleculesToWatchlist(ctx context.Context, watchlistID string, moleculeIDs []string) error {
	if watchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}
	if len(moleculeIDs) == 0 {
		return errors.NewValidation("at least one molecule_id is required")
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return errors.NewNotFound("watchlist %s not found", watchlistID)
	}

	existing := make(map[string]bool)
	for _, mid := range watchlist.MoleculeIDs {
		existing[mid] = true
	}
	for _, mid := range moleculeIDs {
		if !existing[mid] {
			watchlist.MoleculeIDs = append(watchlist.MoleculeIDs, mid)
			existing[mid] = true
		}
	}

	watchlist.UpdatedAt = time.Now().UTC()
	return s.watchlistRepo.Update(ctx, watchlist)
}

// RemovePatentsFromWatchlist removes patent numbers from a watchlist.
func (s *monitoringServiceImpl) RemovePatentsFromWatchlist(ctx context.Context, watchlistID string, patentNumbers []string) error {
	if watchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return errors.NewNotFound("watchlist %s not found", watchlistID)
	}

	toRemove := make(map[string]bool)
	for _, pn := range patentNumbers {
		toRemove[pn] = true
	}

	filtered := make([]string, 0, len(watchlist.PatentNumbers))
	for _, pn := range watchlist.PatentNumbers {
		if !toRemove[pn] {
			filtered = append(filtered, pn)
		}
	}
	watchlist.PatentNumbers = filtered
	watchlist.UpdatedAt = time.Now().UTC()

	return s.watchlistRepo.Update(ctx, watchlist)
}

// RemoveMoleculesFromWatchlist removes molecule IDs from a watchlist.
func (s *monitoringServiceImpl) RemoveMoleculesFromWatchlist(ctx context.Context, watchlistID string, moleculeIDs []string) error {
	if watchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return errors.NewNotFound("watchlist %s not found", watchlistID)
	}

	toRemove := make(map[string]bool)
	for _, mid := range moleculeIDs {
		toRemove[mid] = true
	}

	filtered := make([]string, 0, len(watchlist.MoleculeIDs))
	for _, mid := range watchlist.MoleculeIDs {
		if !toRemove[mid] {
			filtered = append(filtered, mid)
		}
	}
	watchlist.MoleculeIDs = filtered
	watchlist.UpdatedAt = time.Now().UTC()

	return s.watchlistRepo.Update(ctx, watchlist)
}

// RunScan executes an infringement scan for a specific watchlist.
func (s *monitoringServiceImpl) RunScan(ctx context.Context, watchlistID string) (*ScanResult, error) {
	if watchlistID == "" {
		return nil, errors.NewValidation("watchlist_id is required")
	}

	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return nil, errors.NewInternal("failed to find watchlist: %v", err)
	}
	if watchlist == nil {
		return nil, errors.NewNotFound("watchlist %s not found", watchlistID)
	}
	if watchlist.Status != WatchlistStatusActive {
		return nil, errors.NewValidation("watchlist %s is not active", watchlistID)
	}

	startedAt := time.Now().UTC()
	scanID := generateScanID(watchlistID, startedAt)

	s.logger.Info("scan started", "scan_id", scanID, "watchlist_id", watchlistID)

	// Execute pairwise comparison between patents and molecules.
	var matches []ScanMatch
	alertsCreated := 0

	for _, patentNum := range watchlist.PatentNumbers {
		for _, molID := range watchlist.MoleculeIDs {
			// In production, this calls the similarity computation engine.
			// Here we simulate the framework.
			score := s.computeSimilarity(ctx, patentNum, molID)
			if score >= watchlist.SimilarityThreshold {
				match := ScanMatch{
					PatentNumber:    patentNum,
					MoleculeID:      molID,
					SimilarityScore: score,
					RiskScore:       score * 0.95,
					MatchType:       "structural",
				}
				matches = append(matches, match)

				// Create alert for the match.
				level := s.scoreToAlertLevel(score)
				_, alertErr := s.alertService.CreateAlert(ctx, &CreateAlertRequest{
					PatentNumber:    patentNum,
					MoleculeID:      molID,
					WatchlistID:     watchlistID,
					Level:           level,
					Title:           fmt.Sprintf("Infringement match: %s vs %s", patentNum, molID),
					Description:     fmt.Sprintf("Similarity score %.2f exceeds threshold %.2f", score, watchlist.SimilarityThreshold),
					RiskScore:       match.RiskScore,
					SimilarityScore: score,
				})
				if alertErr == nil {
					alertsCreated++
				}
			}
		}
	}

	completedAt := time.Now().UTC()
	result := &ScanResult{
		ScanID:           scanID,
		WatchlistID:      watchlistID,
		StartedAt:        startedAt,
		CompletedAt:      completedAt,
		Duration:         completedAt.Sub(startedAt),
		PatentsScanned:   len(watchlist.PatentNumbers),
		MoleculesScanned: len(watchlist.MoleculeIDs),
		MatchesFound:     len(matches),
		AlertsCreated:    alertsCreated,
		Matches:          matches,
	}

	if err := s.scanResultRepo.Save(ctx, result); err != nil {
		s.logger.Error("failed to save scan result", "error", err, "scan_id", scanID)
	}

	// Update watchlist scan metadata.
	now := time.Now().UTC()
	nextScan := now.Add(watchlist.ScanFrequency.Duration())
	watchlist.LastScanAt = &now
	watchlist.NextScanAt = &nextScan
	watchlist.TotalScans++
	watchlist.TotalAlerts += alertsCreated
	watchlist.UpdatedAt = now
	if err := s.watchlistRepo.Update(ctx, watchlist); err != nil {
		s.logger.Error("failed to update watchlist after scan", "error", err)
	}

	_ = s.producer.Publish(ctx, "monitoring.scan.completed", scanID, map[string]any{
		"scan_id":        scanID,
		"watchlist_id":   watchlistID,
		"matches_found":  len(matches),
		"alerts_created": alertsCreated,
		"duration_ms":    result.Duration.Milliseconds(),
	})

	s.logger.Info("scan completed", "scan_id", scanID, "matches", len(matches), "alerts", alertsCreated)
	return result, nil
}

// RunScheduledScans finds all watchlists due for scanning and runs them.
func (s *monitoringServiceImpl) RunScheduledScans(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	dueWatchlists, err := s.watchlistRepo.FindDueForScan(ctx, now)
	if err != nil {
		return 0, errors.NewInternal("failed to find due watchlists: %v", err)
	}

	scansRun := 0
	for _, wl := range dueWatchlists {
		if wl.Status != WatchlistStatusActive {
			continue
		}
		if _, scanErr := s.RunScan(ctx, wl.ID); scanErr != nil {
			s.logger.Error("scheduled scan failed", "watchlist_id", wl.ID, "error", scanErr)
			continue
		}
		scansRun++
	}

	s.logger.Info("scheduled scans completed", "total_due", len(dueWatchlists), "scans_run", scansRun)
	return scansRun, nil
}

// GetScanHistory retrieves recent scan results for a watchlist.
func (s *monitoringServiceImpl) GetScanHistory(ctx context.Context, watchlistID string, limit int) ([]*ScanResult, error) {
	if watchlistID == "" {
		return nil, errors.NewValidation("watchlist_id is required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	return s.scanResultRepo.FindByWatchlistID(ctx, watchlistID, limit)
}

// computeSimilarity is a placeholder for the actual similarity computation engine.
func (s *monitoringServiceImpl) computeSimilarity(ctx context.Context, patentNumber, moleculeID string) float64 {
	// In production, this delegates to the molecular similarity service.
	// Returns 0 as a safe default for the framework.
	return 0
}

// scoreToAlertLevel maps a similarity score to an alert level.
func (s *monitoringServiceImpl) scoreToAlertLevel(score float64) AlertLevel {
	switch {
	case score >= 0.95:
		return AlertLevelCritical
	case score >= 0.85:
		return AlertLevelHigh
	case score >= 0.70:
		return AlertLevelMedium
	default:
		return AlertLevelLow
	}
}

// generateWatchlistID produces a unique watchlist identifier.
func generateWatchlistID(name, ownerID string, ts time.Time) string {
	data := fmt.Sprintf("wl:%s:%s:%d", name, ownerID, ts.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("WL-%x", hash[:6])
}

// generateScanID produces a unique scan identifier.
func generateScanID(watchlistID string, ts time.Time) string {
	data := fmt.Sprintf("scan:%s:%d", watchlistID, ts.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("SCN-%x", hash[:8])
}

//Personal.AI order the ending

