// Phase 10 - File 198 of 349
// Generation Plan:
// - Functionality: Infringement alert application service — orchestrates alert lifecycle
//   including creation, acknowledgement, dismissal, escalation, multi-channel dispatch,
//   deduplication, and statistics aggregation.
// - Core Implementations:
//   - Define AlertService interface with CreateAlert / AcknowledgeAlert / DismissAlert /
//     EscalateAlert / ListAlerts / GetAlert / UpdateAlertConfig / GetAlertStats
//   - Implement alertServiceImpl struct injecting domain services, repositories,
//     message producer, cache, and logger
//   - Alert trigger flow: risk result -> level determination -> dedup check -> persist ->
//     multi-channel dispatch -> stats update
//   - Dispatch strategy: route by severity to email/wechat/sms/in-app with quiet hours
// - Business Logic:
//   - Alert levels: Critical/High/Medium/Low with distinct channel routing and SLA
//   - Deduplication: same patent+molecule within 24h window
//   - Escalation: auto-escalate unacknowledged alerts past SLA
//   - Statistics: aggregate by time/level/status dimensions
// - Dependencies:
//   - Depends on: domain/patent, domain/molecule, kafka producer, redis cache, pkg/errors, pkg/types
//   - Depended by: http handlers, monitoring service
// - Testing: full lifecycle, dedup, escalation, channel routing, stats, error scenarios
// - Constraint: last line must be //Personal.AI order the ending

package infringement

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/redis"
	kafkainfra "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/messaging/kafka"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// AlertLevel enumerates the severity of an infringement alert.
type AlertLevel int

const (
	AlertLevelLow AlertLevel = iota + 1
	AlertLevelMedium
	AlertLevelHigh
	AlertLevelCritical
)

// String returns the human-readable representation of an AlertLevel.
func (l AlertLevel) String() string {
	switch l {
	case AlertLevelLow:
		return "LOW"
	case AlertLevelMedium:
		return "MEDIUM"
	case AlertLevelHigh:
		return "HIGH"
	case AlertLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// SLADuration returns the maximum response time before automatic escalation.
func (l AlertLevel) SLADuration() time.Duration {
	switch l {
	case AlertLevelCritical:
		return 2 * time.Hour
	case AlertLevelHigh:
		return 8 * time.Hour
	case AlertLevelMedium:
		return 24 * time.Hour
	case AlertLevelLow:
		return 72 * time.Hour
	default:
		return 72 * time.Hour
	}
}

// AlertStatus enumerates the lifecycle states of an alert.
type AlertStatus int

const (
	AlertStatusOpen AlertStatus = iota + 1
	AlertStatusAcknowledged
	AlertStatusDismissed
	AlertStatusEscalated
	AlertStatusResolved
)

// String returns the human-readable representation of an AlertStatus.
func (s AlertStatus) String() string {
	switch s {
	case AlertStatusOpen:
		return "OPEN"
	case AlertStatusAcknowledged:
		return "ACKNOWLEDGED"
	case AlertStatusDismissed:
		return "DISMISSED"
	case AlertStatusEscalated:
		return "ESCALATED"
	case AlertStatusResolved:
		return "RESOLVED"
	default:
		return "UNKNOWN"
	}
}

// DispatchChannel enumerates the notification channels for alert delivery.
type DispatchChannel int

const (
	DispatchChannelInApp DispatchChannel = 1 << iota
	DispatchChannelEmail
	DispatchChannelWeChat
	DispatchChannelSMS
)

// Alert represents a single infringement alert record.
type Alert struct {
	ID              string          `json:"id"`
	PatentNumber    string          `json:"patent_number"`
	MoleculeID      string          `json:"molecule_id"`
	WatchlistID     string          `json:"watchlist_id"`
	Level           AlertLevel      `json:"level"`
	Status          AlertStatus     `json:"status"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	RiskScore       float64         `json:"risk_score"`
	SimilarityScore float64         `json:"similarity_score"`
	Channels        DispatchChannel `json:"channels"`
	AssigneeID      string          `json:"assignee_id"`
	CreatedAt       time.Time       `json:"created_at"`
	AcknowledgedAt  *time.Time      `json:"acknowledged_at,omitempty"`
	ResolvedAt      *time.Time      `json:"resolved_at,omitempty"`
	EscalatedAt     *time.Time      `json:"escalated_at,omitempty"`
	DismissedAt     *time.Time      `json:"dismissed_at,omitempty"`
	DismissReason   string          `json:"dismiss_reason,omitempty"`
	Metadata        map[string]any  `json:"metadata,omitempty"`
}

// CreateAlertRequest carries the parameters for creating a new alert.
type CreateAlertRequest struct {
	PatentNumber    string         `json:"patent_number" validate:"required"`
	MoleculeID      string         `json:"molecule_id" validate:"required"`
	WatchlistID     string         `json:"watchlist_id"`
	Level           AlertLevel     `json:"level" validate:"required,min=1,max=4"`
	Title           string         `json:"title" validate:"required,max=500"`
	Description     string         `json:"description" validate:"max=5000"`
	RiskScore       float64        `json:"risk_score" validate:"min=0,max=1"`
	SimilarityScore float64        `json:"similarity_score" validate:"min=0,max=1"`
	AssigneeID      string         `json:"assignee_id"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// Validate checks the structural integrity of a CreateAlertRequest.
func (r *CreateAlertRequest) Validate() error {
	if r.PatentNumber == "" {
		return errors.NewValidation("patent_number is required")
	}
	if r.MoleculeID == "" {
		return errors.NewValidation("molecule_id is required")
	}
	if r.Level < AlertLevelLow || r.Level > AlertLevelCritical {
		return errors.NewValidation("level must be between 1 and 4")
	}
	if r.Title == "" {
		return errors.NewValidation("title is required")
	}
	if len(r.Title) > 500 {
		return errors.NewValidation("title exceeds 500 characters")
	}
	if r.RiskScore < 0 || r.RiskScore > 1 {
		return errors.NewValidation("risk_score must be between 0 and 1")
	}
	if r.SimilarityScore < 0 || r.SimilarityScore > 1 {
		return errors.NewValidation("similarity_score must be between 0 and 1")
	}
	return nil
}

// DismissAlertRequest carries the parameters for dismissing an alert.
type DismissAlertRequest struct {
	AlertID string `json:"alert_id" validate:"required"`
	Reason  string `json:"reason" validate:"required,max=2000"`
}

// Validate checks the structural integrity of a DismissAlertRequest.
func (r *DismissAlertRequest) Validate() error {
	if r.AlertID == "" {
		return errors.NewValidation("alert_id is required")
	}
	if r.Reason == "" {
		return errors.NewValidation("dismiss reason is required")
	}
	return nil
}

// AlertConfigRequest carries the parameters for updating alert configuration.
type AlertConfigRequest struct {
	WatchlistID    string                    `json:"watchlist_id" validate:"required"`
	ChannelMapping map[AlertLevel]DispatchChannel `json:"channel_mapping"`
	QuietHours     *QuietHoursConfig         `json:"quiet_hours,omitempty"`
	DedupWindowMin int                       `json:"dedup_window_min"`
}

// QuietHoursConfig defines the time window during which non-critical alerts are suppressed.
type QuietHoursConfig struct {
	Enabled  bool   `json:"enabled"`
	Start    string `json:"start"`    // HH:MM format
	End      string `json:"end"`      // HH:MM format
	Timezone string `json:"timezone"` // IANA timezone
}

// AlertListOptions carries filtering and pagination parameters for listing alerts.
type AlertListOptions struct {
	WatchlistID string       `json:"watchlist_id,omitempty"`
	Level       *AlertLevel  `json:"level,omitempty"`
	Status      *AlertStatus `json:"status,omitempty"`
	PatentNumber string      `json:"patent_number,omitempty"`
	MoleculeID  string       `json:"molecule_id,omitempty"`
	Since       *time.Time   `json:"since,omitempty"`
	Until       *time.Time   `json:"until,omitempty"`
	Page        int          `json:"page"`
	PageSize    int          `json:"page_size"`
}

// AlertStats holds aggregated alert statistics.
type AlertStats struct {
	TotalOpen         int            `json:"total_open"`
	TotalAcknowledged int            `json:"total_acknowledged"`
	TotalDismissed    int            `json:"total_dismissed"`
	TotalEscalated    int            `json:"total_escalated"`
	TotalResolved     int            `json:"total_resolved"`
	ByLevel           map[string]int `json:"by_level"`
	AvgResponseTimeMs int64          `json:"avg_response_time_ms"`
	OverSLACount      int            `json:"over_sla_count"`
}

// AlertRepository defines the persistence contract for alert records.
type AlertRepository interface {
	Save(ctx context.Context, alert *Alert) error
	FindByID(ctx context.Context, id string) (*Alert, error)
	Update(ctx context.Context, alert *Alert) error
	List(ctx context.Context, opts AlertListOptions) ([]*Alert, int, error)
	FindDuplicate(ctx context.Context, patentNumber, moleculeID string, since time.Time) (*Alert, error)
	GetStats(ctx context.Context, watchlistID string) (*AlertStats, error)
	FindOverSLA(ctx context.Context) ([]*Alert, error)
}

// MessageProducer abstracts the messaging system.
type MessageProducer interface {
	Publish(ctx context.Context, msg *kafkainfra.ProducerMessage) error
}

// AlertService defines the application-level contract for infringement alert management.
type AlertService interface {
	// CreateAlert creates a new infringement alert after deduplication and dispatches notifications.
	CreateAlert(ctx context.Context, req *CreateAlertRequest) (*Alert, error)

	// AcknowledgeAlert marks an alert as acknowledged by the assignee.
	AcknowledgeAlert(ctx context.Context, alertID string, userID string) error

	// DismissAlert dismisses an alert with a documented reason.
	DismissAlert(ctx context.Context, req *DismissAlertRequest, userID string) error

	// EscalateAlert manually escalates an alert to a higher severity.
	EscalateAlert(ctx context.Context, alertID string, reason string) error

	// ListAlerts returns a paginated list of alerts matching the given filters.
	ListAlerts(ctx context.Context, opts AlertListOptions) ([]*Alert, *commontypes.PaginationResult, error)

	// GetAlert retrieves a single alert by its identifier.
	GetAlert(ctx context.Context, alertID string) (*Alert, error)

	// UpdateAlertConfig updates the alert dispatch configuration for a watchlist.
	UpdateAlertConfig(ctx context.Context, req *AlertConfigRequest) error

	// GetAlertStats returns aggregated alert statistics for a watchlist.
	GetAlertStats(ctx context.Context, watchlistID string) (*AlertStats, error)

	// ProcessOverSLAAlerts scans for alerts that have exceeded their SLA and escalates them.
	ProcessOverSLAAlerts(ctx context.Context) (int, error)
}

// alertServiceImpl is the concrete implementation of AlertService.
type alertServiceImpl struct {
	alertRepo      AlertRepository
	patentService  *patent.PatentService
	producer       MessageProducer
	cache          redis.Cache
	logger         logging.Logger
	mu             sync.RWMutex
	channelConfigs map[string]map[AlertLevel]DispatchChannel // watchlistID -> level -> channels
	quietConfigs   map[string]*QuietHoursConfig              // watchlistID -> quiet hours
	dedupWindow    time.Duration
}

// AlertServiceConfig holds the initialization parameters for the alert service.
type AlertServiceConfig struct {
	DefaultDedupWindow time.Duration
}

// NewAlertService constructs a new AlertService with all required dependencies.
func NewAlertService(
	alertRepo AlertRepository,
	patentService *patent.PatentService,
	producer MessageProducer,
	cache redis.Cache,
	logger logging.Logger,
	cfg AlertServiceConfig,
) AlertService {
	dedupWindow := cfg.DefaultDedupWindow
	if dedupWindow == 0 {
		dedupWindow = 24 * time.Hour
	}

	svc := &alertServiceImpl{
		alertRepo:      alertRepo,
		patentService:  patentService,
		producer:       producer,
		cache:          cache,
		logger:         logger,
		channelConfigs: make(map[string]map[AlertLevel]DispatchChannel),
		quietConfigs:   make(map[string]*QuietHoursConfig),
		dedupWindow:    dedupWindow,
	}

	return svc
}

// CreateAlert validates the request, checks for duplicates, persists the alert,
// dispatches notifications through configured channels, and updates statistics cache.
func (s *alertServiceImpl) CreateAlert(ctx context.Context, req *CreateAlertRequest) (*Alert, error) {
	if req == nil {
		return nil, errors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// Deduplication: check if an identical alert exists within the dedup window.
	dedupSince := time.Now().UTC().Add(-s.dedupWindow)
	existing, err := s.alertRepo.FindDuplicate(ctx, req.PatentNumber, req.MoleculeID, dedupSince)
	if err != nil {
		s.logger.Error("failed to check alert deduplication", logging.Err(err),
			logging.String("patent", req.PatentNumber), logging.String("molecule", req.MoleculeID))
		return nil, errors.NewInternal("deduplication check failed: %v", err)
	}
	if existing != nil {
		s.logger.Info("duplicate alert suppressed",
			logging.String("existing_id", existing.ID), logging.String("patent", req.PatentNumber), logging.String("molecule", req.MoleculeID))
		return existing, nil
	}

	// Determine dispatch channels based on alert level and watchlist configuration.
	channels := s.resolveChannels(req.WatchlistID, req.Level)

	// Check quiet hours — suppress non-critical alerts during quiet periods.
	if req.Level < AlertLevelCritical && s.isQuietHours(req.WatchlistID) {
		// During quiet hours, only deliver via in-app channel.
		channels = DispatchChannelInApp
		s.logger.Info("quiet hours active, restricting to in-app only",
			logging.String("watchlist", req.WatchlistID), logging.String("level", req.Level.String()))
	}

	now := time.Now().UTC()
	alert := &Alert{
		ID:              generateAlertID(req.PatentNumber, req.MoleculeID, now),
		PatentNumber:    req.PatentNumber,
		MoleculeID:      req.MoleculeID,
		WatchlistID:     req.WatchlistID,
		Level:           req.Level,
		Status:          AlertStatusOpen,
		Title:           req.Title,
		Description:     req.Description,
		RiskScore:       req.RiskScore,
		SimilarityScore: req.SimilarityScore,
		Channels:        channels,
		AssigneeID:      req.AssigneeID,
		CreatedAt:       now,
		Metadata:        req.Metadata,
	}

	if err := s.alertRepo.Save(ctx, alert); err != nil {
		s.logger.Error("failed to persist alert", logging.Err(err), logging.String("alert_id", alert.ID))
		return nil, errors.NewInternal("failed to save alert: %v", err)
	}

	// Dispatch notifications asynchronously via message queue.
	if err := s.dispatchAlert(ctx, alert); err != nil {
		// Dispatch failure is non-fatal — the alert is already persisted.
		s.logger.Error("failed to dispatch alert notification", logging.Err(err), logging.String("alert_id", alert.ID))
	}

	// Invalidate stats cache for the watchlist.
	s.invalidateStatsCache(ctx, alert.WatchlistID)

	s.logger.Info("alert created",
		logging.String("alert_id", alert.ID), logging.String("level", alert.Level.String()),
		logging.String("patent", alert.PatentNumber), logging.String("molecule", alert.MoleculeID))

	return alert, nil
}

// AcknowledgeAlert transitions an alert from Open/Escalated to Acknowledged.
func (s *alertServiceImpl) AcknowledgeAlert(ctx context.Context, alertID string, userID string) error {
	if alertID == "" {
		return errors.NewValidation("alert_id is required")
	}
	if userID == "" {
		return errors.NewValidation("user_id is required")
	}

	alert, err := s.alertRepo.FindByID(ctx, alertID)
	if err != nil {
		return errors.NewInternal("failed to find alert: %v", err)
	}
	if alert == nil {
		return errors.NewNotFound("alert %s not found", alertID)
	}

	if alert.Status != AlertStatusOpen && alert.Status != AlertStatusEscalated {
		return errors.NewValidation("alert %s cannot be acknowledged in status %s", alertID, alert.Status.String())
	}

	now := time.Now().UTC()
	alert.Status = AlertStatusAcknowledged
	alert.AcknowledgedAt = &now
	alert.AssigneeID = userID

	if err := s.alertRepo.Update(ctx, alert); err != nil {
		return errors.NewInternal("failed to update alert: %v", err)
	}

	s.invalidateStatsCache(ctx, alert.WatchlistID)

	s.logger.Info("alert acknowledged", logging.String("alert_id", alertID), logging.String("user_id", userID))
	return nil
}

// DismissAlert marks an alert as dismissed with a documented reason.
func (s *alertServiceImpl) DismissAlert(ctx context.Context, req *DismissAlertRequest, userID string) error {
	if req == nil {
		return errors.NewValidation("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return err
	}
	if userID == "" {
		return errors.NewValidation("user_id is required")
	}

	alert, err := s.alertRepo.FindByID(ctx, req.AlertID)
	if err != nil {
		return errors.NewInternal("failed to find alert: %v", err)
	}
	if alert == nil {
		return errors.NewNotFound("alert %s not found", req.AlertID)
	}

	if alert.Status == AlertStatusDismissed {
		return errors.NewValidation("alert %s is already dismissed", req.AlertID)
	}
	if alert.Status == AlertStatusResolved {
		return errors.NewValidation("alert %s is already resolved", req.AlertID)
	}

	now := time.Now().UTC()
	alert.Status = AlertStatusDismissed
	alert.DismissedAt = &now
	alert.DismissReason = req.Reason

	if err := s.alertRepo.Update(ctx, alert); err != nil {
		return errors.NewInternal("failed to update alert: %v", err)
	}

	s.invalidateStatsCache(ctx, alert.WatchlistID)

	s.logger.Info("alert dismissed", logging.String("alert_id", req.AlertID), logging.String("reason", req.Reason), logging.String("user_id", userID))
	return nil
}

// EscalateAlert manually escalates an alert, raising its visibility and re-dispatching.
func (s *alertServiceImpl) EscalateAlert(ctx context.Context, alertID string, reason string) error {
	if alertID == "" {
		return errors.NewValidation("alert_id is required")
	}

	alert, err := s.alertRepo.FindByID(ctx, alertID)
	if err != nil {
		return errors.NewInternal("failed to find alert: %v", err)
	}
	if alert == nil {
		return errors.NewNotFound("alert %s not found", alertID)
	}

	if alert.Status == AlertStatusDismissed || alert.Status == AlertStatusResolved {
		return errors.NewValidation("cannot escalate alert in status %s", alert.Status.String())
	}

	now := time.Now().UTC()
	alert.Status = AlertStatusEscalated
	alert.EscalatedAt = &now

	// Upgrade channels to include all available channels on escalation.
	alert.Channels = DispatchChannelInApp | DispatchChannelEmail | DispatchChannelWeChat | DispatchChannelSMS

	if alert.Metadata == nil {
		alert.Metadata = make(map[string]any)
	}
	alert.Metadata["escalation_reason"] = reason
	alert.Metadata["escalated_at"] = now.Format(time.RFC3339)

	if err := s.alertRepo.Update(ctx, alert); err != nil {
		return errors.NewInternal("failed to update alert: %v", err)
	}

	// Re-dispatch with escalated channels.
	if err := s.dispatchAlert(ctx, alert); err != nil {
		s.logger.Error("failed to dispatch escalated alert", logging.Err(err), logging.String("alert_id", alertID))
	}

	s.invalidateStatsCache(ctx, alert.WatchlistID)

	s.logger.Info("alert escalated", logging.String("alert_id", alertID), logging.String("reason", reason))
	return nil
}

// ListAlerts returns a paginated, filtered list of alerts.
func (s *alertServiceImpl) ListAlerts(ctx context.Context, opts AlertListOptions) ([]*Alert, *commontypes.PaginationResult, error) {
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = 20
	}
	if opts.PageSize > 100 {
		opts.PageSize = 100
	}

	alerts, total, err := s.alertRepo.List(ctx, opts)
	if err != nil {
		return nil, nil, errors.NewInternal("failed to list alerts: %v", err)
	}

	pagination := &commontypes.PaginationResult{
		Page:       opts.Page,
		PageSize:   opts.PageSize,
		Total:      total,
		TotalPages: (total + opts.PageSize - 1) / opts.PageSize,
	}

	return alerts, pagination, nil
}

// GetAlert retrieves a single alert by its identifier.
func (s *alertServiceImpl) GetAlert(ctx context.Context, alertID string) (*Alert, error) {
	if alertID == "" {
		return nil, errors.NewValidation("alert_id is required")
	}

	alert, err := s.alertRepo.FindByID(ctx, alertID)
	if err != nil {
		return nil, errors.NewInternal("failed to find alert: %v", err)
	}
	if alert == nil {
		return nil, errors.NewNotFound("alert %s not found", alertID)
	}

	return alert, nil
}

// UpdateAlertConfig updates the dispatch channel mapping and quiet hours for a watchlist.
func (s *alertServiceImpl) UpdateAlertConfig(ctx context.Context, req *AlertConfigRequest) error {
	if req == nil {
		return errors.NewValidation("request must not be nil")
	}
	if req.WatchlistID == "" {
		return errors.NewValidation("watchlist_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if req.ChannelMapping != nil {
		s.channelConfigs[req.WatchlistID] = req.ChannelMapping
	}

	if req.QuietHours != nil {
		s.quietConfigs[req.WatchlistID] = req.QuietHours
	}

	if req.DedupWindowMin > 0 {
		s.dedupWindow = time.Duration(req.DedupWindowMin) * time.Minute
	}

	s.logger.Info("alert config updated", logging.String("watchlist_id", req.WatchlistID))
	return nil
}

// GetAlertStats returns aggregated statistics for a watchlist, with cache support.
func (s *alertServiceImpl) GetAlertStats(ctx context.Context, watchlistID string) (*AlertStats, error) {
	if watchlistID == "" {
		return nil, errors.NewValidation("watchlist_id is required")
	}

	cacheKey := fmt.Sprintf("alert_stats:%s", watchlistID)
	var cached AlertStats
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	stats, err := s.alertRepo.GetStats(ctx, watchlistID)
	if err != nil {
		return nil, errors.NewInternal("failed to get alert stats: %v", err)
	}

	// Cache for 5 minutes.
	_ = s.cache.Set(ctx, cacheKey, stats, 5*time.Minute)

	return stats, nil
}

// ProcessOverSLAAlerts scans for alerts that have exceeded their SLA and auto-escalates them.
func (s *alertServiceImpl) ProcessOverSLAAlerts(ctx context.Context) (int, error) {
	overdue, err := s.alertRepo.FindOverSLA(ctx)
	if err != nil {
		return 0, errors.NewInternal("failed to find over-SLA alerts: %v", err)
	}

	escalated := 0
	for _, alert := range overdue {
		elapsed := time.Since(alert.CreatedAt)
		sla := alert.Level.SLADuration()

		if elapsed > sla && alert.Status == AlertStatusOpen {
			reason := fmt.Sprintf("auto-escalated: exceeded SLA of %s (elapsed: %s)", sla, elapsed.Round(time.Minute))
			if err := s.EscalateAlert(ctx, alert.ID, reason); err != nil {
				s.logger.Error("failed to auto-escalate alert", logging.Err(err), logging.String("alert_id", alert.ID))
				continue
			}
			escalated++
		}
	}

	s.logger.Info("over-SLA alert processing complete", logging.Int("total_overdue", len(overdue)), logging.Int("escalated", escalated))
	return escalated, nil
}

// resolveChannels determines the dispatch channels for a given watchlist and alert level.
func (s *alertServiceImpl) resolveChannels(watchlistID string, level AlertLevel) DispatchChannel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if mapping, ok := s.channelConfigs[watchlistID]; ok {
		if ch, found := mapping[level]; found {
			return ch
		}
	}

	// Default channel mapping by severity.
	switch level {
	case AlertLevelCritical:
		return DispatchChannelInApp | DispatchChannelEmail | DispatchChannelWeChat | DispatchChannelSMS
	case AlertLevelHigh:
		return DispatchChannelInApp | DispatchChannelEmail | DispatchChannelWeChat
	case AlertLevelMedium:
		return DispatchChannelInApp | DispatchChannelEmail
	case AlertLevelLow:
		return DispatchChannelInApp
	default:
		return DispatchChannelInApp
	}
}

// isQuietHours checks whether the current time falls within the quiet hours window for a watchlist.
func (s *alertServiceImpl) isQuietHours(watchlistID string) bool {
	s.mu.RLock()
	qh, ok := s.quietConfigs[watchlistID]
	s.mu.RUnlock()

	if !ok || qh == nil || !qh.Enabled {
		return false
	}

	loc, err := time.LoadLocation(qh.Timezone)
	if err != nil {
		s.logger.Error("invalid timezone in quiet hours config", logging.String("timezone", qh.Timezone), logging.Err(err))
		return false
	}

	now := time.Now().In(loc)
	startH, startM, err1 := parseHHMM(qh.Start)
	endH, endM, err2 := parseHHMM(qh.End)
	if err1 != nil || err2 != nil {
		s.logger.Error("invalid quiet hours time format", logging.String("start", qh.Start), logging.String("end", qh.End))
		return false
	}

	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startH*60 + startM
	endMinutes := endH*60 + endM

	if startMinutes <= endMinutes {
		// Same-day window: e.g., 22:00 - 23:00
		return currentMinutes >= startMinutes && currentMinutes < endMinutes
	}
	// Cross-midnight window: e.g., 22:00 - 07:00
	return currentMinutes >= startMinutes || currentMinutes < endMinutes
}

// parseHHMM parses a "HH:MM" string into hour and minute integers.
func parseHHMM(s string) (int, int, error) {
	var h, m int
	n, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil || n != 2 {
		return 0, 0, fmt.Errorf("invalid HH:MM format: %s", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("out of range HH:MM: %s", s)
	}
	return h, m, nil
}

// dispatchAlert publishes alert notification messages to the configured channels via Kafka.
func (s *alertServiceImpl) dispatchAlert(ctx context.Context, alert *Alert) error {
	type alertNotification struct {
		AlertID      string `json:"alert_id"`
		Channel      string `json:"channel"`
		Level        string `json:"level"`
		Title        string `json:"title"`
		PatentNumber string `json:"patent_number"`
		MoleculeID   string `json:"molecule_id"`
		AssigneeID   string `json:"assignee_id"`
		Timestamp    string `json:"timestamp"`
	}

	channels := []struct {
		flag DispatchChannel
		name string
	}{
		{DispatchChannelInApp, "in_app"},
		{DispatchChannelEmail, "email"},
		{DispatchChannelWeChat, "wechat"},
		{DispatchChannelSMS, "sms"},
	}

	var firstErr error
	for _, ch := range channels {
		if alert.Channels&ch.flag == 0 {
			continue
		}

		msg := alertNotification{
			AlertID:      alert.ID,
			Channel:      ch.name,
			Level:        alert.Level.String(),
			Title:        alert.Title,
			PatentNumber: alert.PatentNumber,
			MoleculeID:   alert.MoleculeID,
			AssigneeID:   alert.AssigneeID,
			Timestamp:    alert.CreatedAt.Format(time.RFC3339),
		}

		payload, err := json.Marshal(msg)
		if err != nil {
			s.logger.Error("failed to marshal alert notification", logging.Err(err))
			continue
		}

		pm := &kafkainfra.ProducerMessage{
			Topic: fmt.Sprintf("alert.dispatch.%s", ch.name),
			Key:   []byte(alert.ID),
			Value: payload,
		}

		if err := s.producer.Publish(ctx, pm); err != nil {
			s.logger.Error("failed to publish alert to channel",
				logging.String("channel", ch.name), logging.String("alert_id", alert.ID), logging.Err(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

// invalidateStatsCache removes the cached statistics for a watchlist.
func (s *alertServiceImpl) invalidateStatsCache(ctx context.Context, watchlistID string) {
	if watchlistID == "" {
		return
	}
	cacheKey := fmt.Sprintf("alert_stats:%s", watchlistID)
	if err := s.cache.Delete(ctx, cacheKey); err != nil {
		s.logger.Error("failed to invalidate stats cache", logging.String("key", cacheKey), logging.Err(err))
	}
}

// generateAlertID produces a deterministic, unique alert identifier.
func generateAlertID(patentNumber, moleculeID string, ts time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", patentNumber, moleculeID, ts.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("ALT-%x", hash[:8])
}

//Personal.AI order the ending
