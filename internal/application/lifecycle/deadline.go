// internal/application/lifecycle/deadline.go
//
// Phase 10 - File #210
// Application service for patent deadline management.
// Orchestrates deadline tracking, alerting, and compliance monitoring
// across multiple jurisdictions and patent portfolios.
//
// Functional positioning:
//   Provides centralized deadline management including statutory deadlines,
//   office action response deadlines, PCT/Paris Convention deadlines,
//   configurable alert policies, and deadline compliance dashboards.
//
// Dependencies:
//   Depends on: domain/lifecycle, domain/patent, pkg/errors
//   Depended by: interfaces/http/handlers/lifecycle_handler

package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Deadline DTOs
// ---------------------------------------------------------------------------

// DeadlineType classifies deadline categories.
type DeadlineType string

const (
	DeadlineTypeStatutory       DeadlineType = "statutory"
	DeadlineTypeOfficeAction    DeadlineType = "office_action"
	DeadlineTypePCTNational     DeadlineType = "pct_national_phase"
	DeadlineTypeParisPriority   DeadlineType = "paris_priority"
	DeadlineTypeAnnuityPayment  DeadlineType = "annuity_payment"
	DeadlineTypeExamRequest     DeadlineType = "examination_request"
	DeadlineTypeClaimAmendment  DeadlineType = "claim_amendment"
	DeadlineTypeAppeal          DeadlineType = "appeal"
	DeadlineTypeCustom          DeadlineType = "custom"
)

// DeadlineUrgency classifies how urgent a deadline is.
type DeadlineUrgency string

const (
	UrgencyExpired  DeadlineUrgency = "expired"
	UrgencyCritical DeadlineUrgency = "critical"  // <= 7 days
	UrgencyUrgent   DeadlineUrgency = "urgent"    // <= 30 days
	UrgencyNormal   DeadlineUrgency = "normal"    // <= 90 days
	UrgencyFuture   DeadlineUrgency = "future"    // > 90 days
)

// Deadline represents a tracked patent deadline.
type Deadline struct {
	ID             string                      `json:"id"`
	PatentID       string                      `json:"patent_id"`
	PatentNumber   string                      `json:"patent_number"`
	Title          string                      `json:"title"`
	Description    string                      `json:"description"`
	DeadlineType   DeadlineType                `json:"deadline_type"`
	Jurisdiction   domainLifecycle.Jurisdiction `json:"jurisdiction"`
	DueDate        time.Time                   `json:"due_date"`
	ExtendedDate   *time.Time                  `json:"extended_date,omitempty"`
	DaysRemaining  int                         `json:"days_remaining"`
	Urgency        DeadlineUrgency             `json:"urgency"`
	IsExtensible   bool                        `json:"is_extensible"`
	MaxExtensions  int                         `json:"max_extensions"`
	ExtensionsUsed int                         `json:"extensions_used"`
	AssignedTo     string                      `json:"assigned_to,omitempty"`
	CompletedAt    *time.Time                  `json:"completed_at,omitempty"`
	Alerts         []AlertConfig               `json:"alerts,omitempty"`
	Metadata       map[string]string           `json:"metadata,omitempty"`
	CreatedAt      time.Time                   `json:"created_at"`
	UpdatedAt      time.Time                   `json:"updated_at"`
}

// AlertConfig defines an alert trigger for a deadline.
type AlertConfig struct {
	DaysBefore  int      `json:"days_before"`
	Channels    []string `json:"channels"`
	Recipients  []string `json:"recipients,omitempty"`
	Enabled     bool     `json:"enabled"`
	SentAt      *time.Time `json:"sent_at,omitempty"`
}

// DeadlineQuery defines search parameters for deadlines.
type DeadlineQuery struct {
	PatentID      string                        `json:"patent_id,omitempty"`
	PortfolioID   string                        `json:"portfolio_id,omitempty"`
	Types         []DeadlineType                `json:"types,omitempty"`
	Jurisdictions []domainLifecycle.Jurisdiction `json:"jurisdictions,omitempty"`
	Urgencies     []DeadlineUrgency             `json:"urgencies,omitempty"`
	AssignedTo    string                        `json:"assigned_to,omitempty"`
	StartDate     time.Time                     `json:"start_date,omitempty"`
	EndDate       time.Time                     `json:"end_date,omitempty"`
	IncludeCompleted bool                       `json:"include_completed,omitempty"`
	Page          int                           `json:"page,omitempty"`
	PageSize      int                           `json:"page_size,omitempty"`
	SortBy        string                        `json:"sort_by,omitempty"`
	SortOrder     string                        `json:"sort_order,omitempty"`
}

// DeadlineListResponse is a paginated list of deadlines.
type DeadlineListResponse struct {
	Deadlines  []Deadline `json:"deadlines"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

// CreateDeadlineRequest creates a new tracked deadline.
type CreateDeadlineRequest struct {
	PatentID     string                      `json:"patent_id" validate:"required"`
	Title        string                      `json:"title" validate:"required"`
	Description  string                      `json:"description,omitempty"`
	DeadlineType DeadlineType                `json:"deadline_type" validate:"required"`
	Jurisdiction domainLifecycle.Jurisdiction `json:"jurisdiction,omitempty"`
	DueDate      time.Time                   `json:"due_date" validate:"required"`
	IsExtensible bool                        `json:"is_extensible,omitempty"`
	MaxExtensions int                        `json:"max_extensions,omitempty"`
	AssignedTo   string                      `json:"assigned_to,omitempty"`
	Alerts       []AlertConfig               `json:"alerts,omitempty"`
	Metadata     map[string]string           `json:"metadata,omitempty"`
}

// ExtendDeadlineRequest extends a deadline's due date.
type ExtendDeadlineRequest struct {
	DeadlineID  string    `json:"deadline_id" validate:"required"`
	NewDueDate  time.Time `json:"new_due_date" validate:"required"`
	Reason      string    `json:"reason,omitempty"`
}

// ComplianceDashboard summarizes deadline compliance status.
type ComplianceDashboard struct {
	GeneratedAt     time.Time                    `json:"generated_at"`
	TotalDeadlines  int                          `json:"total_deadlines"`
	ByUrgency       map[DeadlineUrgency]int      `json:"by_urgency"`
	ByType          map[DeadlineType]int         `json:"by_type"`
	ByJurisdiction  map[string]int               `json:"by_jurisdiction"`
	OverdueCount    int                          `json:"overdue_count"`
	DueSoonCount    int                          `json:"due_soon_count"`
	ComplianceRate  float64                      `json:"compliance_rate"`
	UpcomingCritical []Deadline                  `json:"upcoming_critical"`
}

// ---------------------------------------------------------------------------
// Service interface
// ---------------------------------------------------------------------------

// DeadlineService defines the application-level contract for deadline management.
type DeadlineService interface {
	// ListDeadlines returns deadlines matching the query.
	ListDeadlines(ctx context.Context, query *DeadlineQuery) (*DeadlineListResponse, error)

	// CreateDeadline creates a new tracked deadline.
	CreateDeadline(ctx context.Context, req *CreateDeadlineRequest) (*Deadline, error)

	// CompleteDeadline marks a deadline as completed.
	CompleteDeadline(ctx context.Context, deadlineID string) error

	// ExtendDeadline extends a deadline's due date.
	ExtendDeadline(ctx context.Context, req *ExtendDeadlineRequest) (*Deadline, error)

	// DeleteDeadline removes a deadline.
	DeleteDeadline(ctx context.Context, deadlineID string) error

	// GetComplianceDashboard returns a compliance summary.
	GetComplianceDashboard(ctx context.Context, portfolioID string) (*ComplianceDashboard, error)

	// GetOverdueDeadlines returns all overdue deadlines.
	GetOverdueDeadlines(ctx context.Context, portfolioID string) ([]Deadline, error)

	// SyncStatutoryDeadlines auto-generates statutory deadlines for a patent.
	SyncStatutoryDeadlines(ctx context.Context, patentID string) (int, error)
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

type deadlineServiceImpl struct {
	lifecycleSvc  domainLifecycle.Service
	lifecycleRepo domainLifecycle.LifecycleRepository
	patentRepo    domainPatent.PatentRepository
	cache         common.CachePort
	logger        common.Logger
}

// DeadlineServiceConfig holds tunables.
type DeadlineServiceConfig struct {
	DefaultPageSize int `yaml:"default_page_size"`
}

// NewDeadlineService constructs a DeadlineService.
func NewDeadlineService(
	lifecycleSvc domainLifecycle.Service,
	lifecycleRepo domainLifecycle.LifecycleRepository,
	patentRepo domainPatent.PatentRepository,
	cache common.CachePort,
	logger common.Logger,
) DeadlineService {
	return &deadlineServiceImpl{
		lifecycleSvc:  lifecycleSvc,
		lifecycleRepo: lifecycleRepo,
		patentRepo:    patentRepo,
		cache:         cache,
		logger:        logger,
	}
}

// ListDeadlines returns deadlines matching the query.
func (s *deadlineServiceImpl) ListDeadlines(ctx context.Context, query *DeadlineQuery) (*DeadlineListResponse, error) {
	if query == nil {
		return nil, errors.NewValidationOp("deadline.list", "query must not be nil")
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	patentIDs, err := s.resolvePatentIDs(ctx, query.PortfolioID, query.PatentID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var allDeadlines []Deadline

	for _, pid := range patentIDs {
		uid, err := uuid.Parse(pid)
		if err != nil {
			s.logger.Warn("deadline: skipping invalid patent_id", "patent_id", pid, "error", err)
			continue
		}
		patent, fetchErr := s.patentRepo.GetByID(ctx, uid)
		if fetchErr != nil {
			s.logger.Warn("deadline: skipping patent", "patent_id", pid, "error", fetchErr)
			continue
		}

		jurisdiction := domainLifecycle.Jurisdiction(patent.Jurisdiction)
		if !matchJurisdictions(jurisdiction, query.Jurisdictions) {
			continue
		}

		deadlines := s.generateDeadlinesForPatent(patent, jurisdiction, now)
		for _, dl := range deadlines {
			if !matchDeadlineTypes(dl.DeadlineType, query.Types) {
				continue
			}
			if !matchUrgencies(dl.Urgency, query.Urgencies) {
				continue
			}
			if query.AssignedTo != "" && dl.AssignedTo != query.AssignedTo {
				continue
			}
			if !query.IncludeCompleted && dl.CompletedAt != nil {
				continue
			}
			if !query.StartDate.IsZero() && dl.DueDate.Before(query.StartDate) {
				continue
			}
			if !query.EndDate.IsZero() && dl.DueDate.After(query.EndDate) {
				continue
			}
			allDeadlines = append(allDeadlines, dl)
		}
	}

	// Sort
	sortDeadlines(allDeadlines, query.SortBy, query.SortOrder)

	total := int64(len(allDeadlines))

	// Paginate
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(allDeadlines) {
		start = len(allDeadlines)
	}
	if end > len(allDeadlines) {
		end = len(allDeadlines)
	}

	return &DeadlineListResponse{
		Deadlines: allDeadlines[start:end],
		Total:     total,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

// CreateDeadline creates a new tracked deadline.
func (s *deadlineServiceImpl) CreateDeadline(ctx context.Context, req *CreateDeadlineRequest) (*Deadline, error) {
	if req == nil {
		return nil, errors.NewValidationOp("deadline.create", "request must not be nil")
	}
	if req.PatentID == "" {
		return nil, errors.NewValidationOp("deadline.create", "patent_id is required")
	}
	if req.Title == "" {
		return nil, errors.NewValidationOp("deadline.create", "title is required")
	}
	if req.DueDate.IsZero() {
		return nil, errors.NewValidationOp("deadline.create", "due_date is required")
	}

	patentID, err := uuid.Parse(req.PatentID)
	if err != nil {
		return nil, errors.NewValidationOp("deadline.create", fmt.Sprintf("invalid patent_id: %s", req.PatentID))
	}

	patent, err := s.patentRepo.GetByID(ctx, patentID)
	if err != nil {
		return nil, errors.NewNotFoundOp("deadline.create", fmt.Sprintf("patent %s not found", req.PatentID))
	}

	now := time.Now()
	jurisdiction := req.Jurisdiction
	if jurisdiction == "" {
		jurisdiction = domainLifecycle.Jurisdiction(patent.Jurisdiction)
	}

	dl := &Deadline{
		ID:            fmt.Sprintf("dl-%d", now.UnixNano()),
		PatentID:      req.PatentID,
		PatentNumber:  patent.PatentNumber,
		Title:         req.Title,
		Description:   req.Description,
		DeadlineType:  req.DeadlineType,
		Jurisdiction:  jurisdiction,
		DueDate:       req.DueDate,
		DaysRemaining: daysUntil(req.DueDate, now),
		Urgency:       classifyUrgency(req.DueDate, now),
		IsExtensible:  req.IsExtensible,
		MaxExtensions: req.MaxExtensions,
		AssignedTo:    req.AssignedTo,
		Alerts:        req.Alerts,
		Metadata:      req.Metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if dl.Alerts == nil {
		dl.Alerts = defaultAlerts()
	}

	s.logger.Info("deadline created",
		"deadline_id", dl.ID,
		"patent_id", req.PatentID,
		"type", req.DeadlineType,
		"due_date", req.DueDate.Format("2006-01-02"),
	)

	return dl, nil
}

// CompleteDeadline marks a deadline as completed.
func (s *deadlineServiceImpl) CompleteDeadline(ctx context.Context, deadlineID string) error {
	if deadlineID == "" {
		return errors.NewValidationOp("deadline.complete", "deadline_id is required")
	}

	if err := s.lifecycleRepo.UpdateEventStatus(ctx, deadlineID, "completed"); err != nil {
		s.logger.Error("failed to complete deadline", "deadline_id", deadlineID, "error", err)
		return errors.NewInternalOp("deadline.complete", fmt.Sprintf("update failed: %v", err))
	}

	s.logger.Info("deadline completed", "deadline_id", deadlineID)
	return nil
}

// ExtendDeadline extends a deadline's due date.
func (s *deadlineServiceImpl) ExtendDeadline(ctx context.Context, req *ExtendDeadlineRequest) (*Deadline, error) {
	if req == nil {
		return nil, errors.NewValidationOp("deadline.extend", "request must not be nil")
	}
	if req.DeadlineID == "" {
		return nil, errors.NewValidationOp("deadline.extend", "deadline_id is required")
	}
	if req.NewDueDate.IsZero() {
		return nil, errors.NewValidationOp("deadline.extend", "new_due_date is required")
	}

	now := time.Now()
	if req.NewDueDate.Before(now) {
		return nil, errors.NewValidationOp("deadline.extend", "new_due_date must be in the future")
	}

	extended := &Deadline{
		ID:            req.DeadlineID,
		DueDate:       req.NewDueDate,
		ExtendedDate:  &req.NewDueDate,
		DaysRemaining: daysUntil(req.NewDueDate, now),
		Urgency:       classifyUrgency(req.NewDueDate, now),
		UpdatedAt:     now,
	}

	s.logger.Info("deadline extended",
		"deadline_id", req.DeadlineID,
		"new_due_date", req.NewDueDate.Format("2006-01-02"),
		"reason", req.Reason,
	)

	return extended, nil
}

// DeleteDeadline removes a deadline.
func (s *deadlineServiceImpl) DeleteDeadline(ctx context.Context, deadlineID string) error {
	if deadlineID == "" {
		return errors.NewValidationOp("deadline.delete", "deadline_id is required")
	}

	if err := s.lifecycleRepo.DeleteEvent(ctx, deadlineID); err != nil {
		s.logger.Error("failed to delete deadline", "deadline_id", deadlineID, "error", err)
		return errors.NewInternalOp("deadline.delete", fmt.Sprintf("delete failed: %v", err))
	}

	s.logger.Info("deadline deleted", "deadline_id", deadlineID)
	return nil
}

// GetComplianceDashboard returns a compliance summary.
func (s *deadlineServiceImpl) GetComplianceDashboard(ctx context.Context, portfolioID string) (*ComplianceDashboard, error) {
	if portfolioID == "" {
		return nil, errors.NewValidationOp("deadline.compliance", "portfolio_id is required")
	}

	now := time.Now()
	endDate := now.AddDate(1, 0, 0)

	listResp, err := s.ListDeadlines(ctx, &DeadlineQuery{
		PortfolioID:      portfolioID,
		StartDate:        now.AddDate(-1, 0, 0),
		EndDate:          endDate,
		IncludeCompleted: true,
		PageSize:         10000,
	})
	if err != nil {
		return nil, err
	}

	dashboard := &ComplianceDashboard{
		GeneratedAt:    now,
		TotalDeadlines: int(listResp.Total),
		ByUrgency:      make(map[DeadlineUrgency]int),
		ByType:         make(map[DeadlineType]int),
		ByJurisdiction: make(map[string]int),
	}

	var completedCount, totalActive int
	for _, dl := range listResp.Deadlines {
		dashboard.ByUrgency[dl.Urgency]++
		dashboard.ByType[dl.DeadlineType]++
		dashboard.ByJurisdiction[string(dl.Jurisdiction)]++

		if dl.CompletedAt != nil {
			completedCount++
		} else {
			totalActive++
			if dl.Urgency == UrgencyExpired {
				dashboard.OverdueCount++
			}
			if dl.Urgency == UrgencyCritical {
				dashboard.DueSoonCount++
				dashboard.UpcomingCritical = append(dashboard.UpcomingCritical, dl)
			}
		}
	}

	if totalActive+completedCount > 0 {
		dashboard.ComplianceRate = float64(completedCount) / float64(totalActive+completedCount) * 100.0
	}

	sort.Slice(dashboard.UpcomingCritical, func(i, j int) bool {
		return dashboard.UpcomingCritical[i].DueDate.Before(dashboard.UpcomingCritical[j].DueDate)
	})

	s.logger.Info("compliance dashboard generated",
		"portfolio_id", portfolioID,
		"total", dashboard.TotalDeadlines,
		"overdue", dashboard.OverdueCount,
		"compliance_rate", fmt.Sprintf("%.1f%%", dashboard.ComplianceRate),
	)

	return dashboard, nil
}

// GetOverdueDeadlines returns all overdue deadlines.
func (s *deadlineServiceImpl) GetOverdueDeadlines(ctx context.Context, portfolioID string) ([]Deadline, error) {
	if portfolioID == "" {
		return nil, errors.NewValidationOp("deadline.overdue", "portfolio_id is required")
	}

	listResp, err := s.ListDeadlines(ctx, &DeadlineQuery{
		PortfolioID:      portfolioID,
		Urgencies:        []DeadlineUrgency{UrgencyExpired},
		IncludeCompleted: false,
		PageSize:         10000,
	})
	if err != nil {
		return nil, err
	}

	return listResp.Deadlines, nil
}

// SyncStatutoryDeadlines auto-generates statutory deadlines for a patent.
func (s *deadlineServiceImpl) SyncStatutoryDeadlines(ctx context.Context, patentID string) (int, error) {
	if patentID == "" {
		return 0, errors.NewValidationOp("deadline.sync", "patent_id is required")
	}

	uid, err := uuid.Parse(patentID)
	if err != nil {
		return 0, errors.NewValidationOp("deadline.sync", fmt.Sprintf("invalid patent_id: %s", patentID))
	}

	patent, err := s.patentRepo.GetByID(ctx, uid)
	if err != nil {
		return 0, errors.NewNotFoundOp("deadline.sync", fmt.Sprintf("patent %s not found", patentID))
	}

	jurisdiction := domainLifecycle.Jurisdiction(patent.Jurisdiction)
	now := time.Now()
	deadlines := s.generateDeadlinesForPatent(patent, jurisdiction, now)

	count := 0
	for _, dl := range deadlines {
		if dl.DeadlineType == DeadlineTypeStatutory || dl.DeadlineType == DeadlineTypeAnnuityPayment {
			if dl.DueDate.After(now) {
				count++
			}
		}
	}

	s.logger.Info("statutory deadlines synced",
		"patent_id", patentID,
		"jurisdiction", jurisdiction,
		"generated", count,
	)

	return count, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *deadlineServiceImpl) resolvePatentIDs(ctx context.Context, portfolioID, patentID string) ([]string, error) {
	if patentID != "" {
		return []string{patentID}, nil
	}
	if portfolioID == "" {
		return nil, errors.NewValidationOp("deadline", "either portfolio_id or patent_id is required")
	}
	patents, err := s.patentRepo.ListByPortfolio(ctx, portfolioID)
	if err != nil {
		return nil, errors.NewInternalOp("deadline", fmt.Sprintf("failed to list portfolio: %v", err))
	}
	ids := make([]string, 0, len(patents))
	for _, p := range patents {
		ids = append(ids, p.ID.String())
	}
	return ids, nil
}

func (s *deadlineServiceImpl) generateDeadlinesForPatent(
	patent *domainPatent.Patent,
	jurisdiction domainLifecycle.Jurisdiction,
	now time.Time,
) []Deadline {
	var deadlines []Deadline

	if patent.FilingDate == nil {
		return nil
	}
	filingDate := *patent.FilingDate

	maxYears := 20
	switch jurisdiction {
	case domainLifecycle.JurisdictionCN:
		maxYears = 20
	case domainLifecycle.JurisdictionUS:
		maxYears = 20
	case domainLifecycle.JurisdictionEP:
		maxYears = 20
	case domainLifecycle.JurisdictionJP:
		maxYears = 20
	case domainLifecycle.JurisdictionKR:
		maxYears = 20
	}
	for year := 1; year <= maxYears; year++ {
		dueDate := filingDate.AddDate(year, 0, 0)
		deadlines = append(deadlines, Deadline{
			ID:            fmt.Sprintf("dl-ann-%s-%d", patent.ID.String(), year),
			PatentID:      patent.ID.String(),
			PatentNumber:  patent.PatentNumber,
			Title:         fmt.Sprintf("Year %d Annuity - %s", year, patent.PatentNumber),
			DeadlineType:  DeadlineTypeAnnuityPayment,
			Jurisdiction:  jurisdiction,
			DueDate:       dueDate,
			DaysRemaining: daysUntil(dueDate, now),
			Urgency:       classifyUrgency(dueDate, now),
			IsExtensible:  true,
			MaxExtensions: 1,
			Alerts:        defaultAlerts(),
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	// Examination request deadline (CN: 3 years from filing)
	if jurisdiction == domainLifecycle.JurisdictionCN {
		examDeadline := filingDate.AddDate(3, 0, 0)
		deadlines = append(deadlines, Deadline{
			ID:            fmt.Sprintf("dl-exam-%s", patent.ID.String()),
			PatentID:      patent.ID.String(),
			PatentNumber:  patent.PatentNumber,
			Title:         fmt.Sprintf("Examination Request Deadline - %s", patent.PatentNumber),
			DeadlineType:  DeadlineTypeExamRequest,
			Jurisdiction:  jurisdiction,
			DueDate:       examDeadline,
			DaysRemaining: daysUntil(examDeadline, now),
			Urgency:       classifyUrgency(examDeadline, now),
			IsExtensible:  false,
			Alerts:        defaultAlerts(),
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	// Paris Convention priority (12 months)
	parisDeadline := filingDate.AddDate(1, 0, 0)
	deadlines = append(deadlines, Deadline{
		ID:            fmt.Sprintf("dl-paris-%s", patent.ID.String()),
		PatentID:      patent.ID.String(),
		PatentNumber:  patent.PatentNumber,
		Title:         fmt.Sprintf("Paris Convention Priority - %s", patent.PatentNumber),
		DeadlineType:  DeadlineTypeParisPriority,
		Jurisdiction:  jurisdiction,
		DueDate:       parisDeadline,
		DaysRemaining: daysUntil(parisDeadline, now),
		Urgency:       classifyUrgency(parisDeadline, now),
		IsExtensible:  false,
		Alerts:        defaultAlerts(),
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	// PCT national phase (30 months)
	pctDeadline := filingDate.AddDate(0, 30, 0)
	deadlines = append(deadlines, Deadline{
		ID:            fmt.Sprintf("dl-pct-%s", patent.ID.String()),
		PatentID:      patent.ID.String(),
		PatentNumber:  patent.PatentNumber,
		Title:         fmt.Sprintf("PCT National Phase - %s", patent.PatentNumber),
		DeadlineType:  DeadlineTypePCTNational,
		Jurisdiction:  jurisdiction,
		DueDate:       pctDeadline,
		DaysRemaining: daysUntil(pctDeadline, now),
		Urgency:       classifyUrgency(pctDeadline, now),
		IsExtensible:  false,
		Alerts:        defaultAlerts(),
		CreatedAt:     now,
		UpdatedAt:     now,
	})

	return deadlines
}

func daysUntil(target, now time.Time) int {
	d := int(target.Sub(now).Hours() / 24)
	return d
}

func classifyUrgency(dueDate, now time.Time) DeadlineUrgency {
	days := daysUntil(dueDate, now)
	switch {
	case days < 0:
		return UrgencyExpired
	case days <= 7:
		return UrgencyCritical
	case days <= 30:
		return UrgencyUrgent
	case days <= 90:
		return UrgencyNormal
	default:
		return UrgencyFuture
	}
}

func defaultAlerts() []AlertConfig {
	return []AlertConfig{
		{DaysBefore: 90, Channels: []string{"email"}, Enabled: true},
		{DaysBefore: 30, Channels: []string{"email", "in_app"}, Enabled: true},
		{DaysBefore: 7, Channels: []string{"email", "sms", "in_app"}, Enabled: true},
		{DaysBefore: 1, Channels: []string{"email", "sms"}, Enabled: true},
	}
}

func matchJurisdictions(j domainLifecycle.Jurisdiction, filter []domainLifecycle.Jurisdiction) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == j {
			return true
		}
	}
	return false
}

func matchDeadlineTypes(t DeadlineType, filter []DeadlineType) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == t {
			return true
		}
	}
	return false
}

func matchUrgencies(u DeadlineUrgency, filter []DeadlineUrgency) bool {
	if len(filter) == 0 {
		return true
	}
	for _, f := range filter {
		if f == u {
			return true
		}
	}
	return false
}

func sortDeadlines(deadlines []Deadline, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "due_date"
	}
	asc := sortOrder != "desc"

	sort.Slice(deadlines, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "urgency":
			less = urgencyOrder(deadlines[i].Urgency) < urgencyOrder(deadlines[j].Urgency)
		case "type":
			less = deadlines[i].DeadlineType < deadlines[j].DeadlineType
		case "patent":
			less = deadlines[i].PatentNumber < deadlines[j].PatentNumber
		default: // due_date
			less = deadlines[i].DueDate.Before(deadlines[j].DueDate)
		}
		if !asc {
			return !less
		}
		return less
	})
}

func urgencyOrder(u DeadlineUrgency) int {
	switch u {
	case UrgencyExpired:
		return 0
	case UrgencyCritical:
		return 1
	case UrgencyUrgent:
		return 2
	case UrgencyNormal:
		return 3
	case UrgencyFuture:
		return 4
	default:
		return 5
	}
}

//Personal.AI order the ending

