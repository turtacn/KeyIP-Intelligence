package lifecycle

import (
	"context"
	"fmt"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// LifecycleStatus represents the comprehensive status of a patent.
type LifecycleStatus struct {
	PatentID         string         `json:"patent_id"`
	CurrentPhase     LifecyclePhase `json:"current_phase"`
	RemainingLifeYears float64        `json:"remaining_life_years"`
	NextDeadline     *Deadline      `json:"next_deadline"`
	NextAnnuity      *AnnuityRecord `json:"next_annuity"`
	OverdueItems     int            `json:"overdue_items"`
	HealthIndicator  string         `json:"health_indicator"` // healthy, warning, critical
	LastEventDate    time.Time      `json:"last_event_date"`
}

// LifecycleDashboard provides a high-level view.
type LifecycleDashboard struct {
	OwnerID           string                `json:"owner_id"`
	TotalPatents      int                   `json:"total_patents"`
	PhaseDistribution map[LifecyclePhase]int `json:"phase_distribution"`
	UpcomingDeadlines []*Deadline           `json:"upcoming_deadlines"`
	OverdueItems      []*Deadline           `json:"overdue_items"`
	UpcomingAnnuities []*AnnuityRecord      `json:"upcoming_annuities"`
	MonthlyExpenses   map[string]int64      `json:"monthly_expenses"`
	CriticalAlerts    []string              `json:"critical_alerts"`
	GeneratedAt       time.Time             `json:"generated_at"`
}

// MaintenanceReport summarizes the daily maintenance run.
type MaintenanceReport struct {
	ExecutedAt         time.Time `json:"executed_at"`
	OverdueDetected    int       `json:"overdue_detected"`
	UrgenciesRefreshed int       `json:"urgencies_refreshed"`
	RemindersGenerated int       `json:"reminders_generated"`
	PhasesAutoAdvanced int       `json:"phases_auto_advanced"`
	Errors             []string  `json:"errors"`
}

// CostAnalysis provides cost insights.
type CostAnalysis struct {
	PortfolioID            string            `json:"portfolio_id"`
	HistoricalCosts        map[int]int64     `json:"historical_costs"`
	ForecastCosts          map[int]int64     `json:"forecast_costs"`
	TotalHistorical        int64             `json:"total_historical"`
	TotalForecast          int64             `json:"total_forecast"`
	CostByJurisdiction     map[string]int64  `json:"cost_by_jurisdiction"`
	OptimizationSuggestions []string          `json:"optimization_suggestions"`
	Currency               string            `json:"currency"`
}

// TimelineView provides a chronological view of the lifecycle.
type TimelineView struct {
	PatentID         string            `json:"patent_id"`
	Events           []*LifecycleEvent `json:"events"`
	PhaseTransitions []PhaseTransition `json:"phase_transitions"`
	Deadlines        []*Deadline       `json:"deadlines"`
	AnnuityRecords   []*AnnuityRecord  `json:"annuity_records"`
	CurrentPhase     LifecyclePhase    `json:"current_phase"`
	FilingDate       time.Time         `json:"filing_date"`
	ExpirationDate   *time.Time        `json:"expiration_date"`
}

// LifecycleService defines the interface for lifecycle management.
type LifecycleService interface {
	InitializeLifecycle(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time) (*LifecycleRecord, error)
	AdvancePhase(ctx context.Context, patentID string, targetPhase LifecyclePhase, reason string) error
	RecordEvent(ctx context.Context, patentID, eventType, description string, metadata map[string]string) error
	GetLifecycleStatus(ctx context.Context, patentID string) (*LifecycleStatus, error)
	GetLifecycleDashboard(ctx context.Context, ownerID string) (*LifecycleDashboard, error)
	ProcessDailyMaintenance(ctx context.Context) (*MaintenanceReport, error)
	GetCostAnalysis(ctx context.Context, portfolioID string, forecastYears int) (*CostAnalysis, error)
	GetTimelineView(ctx context.Context, patentID string) (*TimelineView, error)
}

type lifecycleServiceImpl struct {
	repo            LifecycleRepository
	annuitySvc      AnnuityService
	deadlineSvc     DeadlineService
	jurisdictionReg JurisdictionRegistry
}

// NewLifecycleService creates a new LifecycleService.
func NewLifecycleService(repo LifecycleRepository, annuitySvc AnnuityService, deadlineSvc DeadlineService, reg JurisdictionRegistry) LifecycleService {
	return &lifecycleServiceImpl{
		repo:            repo,
		annuitySvc:      annuitySvc,
		deadlineSvc:     deadlineSvc,
		jurisdictionReg: reg,
	}
}

func (s *lifecycleServiceImpl) InitializeLifecycle(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time) (*LifecycleRecord, error) {
	if !s.jurisdictionReg.IsSupported(jurisdictionCode) {
		return nil, apperrors.NewValidation("unsupported jurisdiction: %s", jurisdictionCode)
	}
	if patentID == "" {
		return nil, apperrors.NewValidation("patentID required")
	}

	lr, err := NewLifecycleRecord(patentID, jurisdictionCode, filingDate)
	if err != nil {
		return nil, apperrors.NewValidation(err.Error())
	}

	// Assuming 20 years max for now, should get from jurisdiction
	term, err := s.jurisdictionReg.GetPatentTerm(jurisdictionCode, "invention")
	if err == nil {
		lr.TotalLifeYears = float64(term)
		exp := lr.FilingDate.AddDate(int(lr.TotalLifeYears), 0, 0)
		lr.ExpirationDate = &exp
	}

	if err := s.repo.Save(ctx, lr); err != nil {
		return nil, err
	}

	return lr, nil
}

func (s *lifecycleServiceImpl) AdvancePhase(ctx context.Context, patentID string, targetPhase LifecyclePhase, reason string) error {
	lr, err := s.repo.FindByPatentID(ctx, patentID)
	if err != nil {
		return err
	}
	if lr == nil {
		return apperrors.NewNotFound("patent lifecycle not found: %s", patentID)
	}

	if err := lr.TransitionTo(targetPhase, reason, "user"); err != nil {
		return apperrors.NewValidation(err.Error())
	}

	if targetPhase == PhaseGranted {
		if err := lr.MarkGranted(time.Now().UTC()); err != nil {
			return apperrors.NewValidation(err.Error())
		}
	} else if targetPhase == PhaseAbandoned {
		if err := lr.MarkAbandoned(reason); err != nil {
			return apperrors.NewValidation(err.Error())
		}
	}

	return s.repo.Save(ctx, lr)
}

func (s *lifecycleServiceImpl) RecordEvent(ctx context.Context, patentID, eventType, description string, metadata map[string]string) error {
	lr, err := s.repo.FindByPatentID(ctx, patentID)
	if err != nil {
		return err
	}
	if lr == nil {
		return apperrors.NewNotFound("patent lifecycle not found: %s", patentID)
	}

	lr.AddEvent(eventType, description, "user", metadata)
	return s.repo.Save(ctx, lr)
}

func (s *lifecycleServiceImpl) GetLifecycleStatus(ctx context.Context, patentID string) (*LifecycleStatus, error) {
	lr, err := s.repo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, err
	}
	if lr == nil {
		return nil, apperrors.NewNotFound("patent lifecycle not found: %s", patentID)
	}

	status := &LifecycleStatus{
		PatentID:           patentID,
		CurrentPhase:       lr.CurrentPhase,
		RemainingLifeYears: lr.CalculateRemainingLife(time.Now()),
		HealthIndicator:    "healthy",
	}
	if len(lr.Events) > 0 {
		status.LastEventDate = lr.Events[len(lr.Events)-1].EventDate
	}

	// Ideally fetch deadlines and annuities
	// For skeleton, returning nil for next items

	return status, nil
}

func (s *lifecycleServiceImpl) GetLifecycleDashboard(ctx context.Context, ownerID string) (*LifecycleDashboard, error) {
	return &LifecycleDashboard{
		OwnerID:     ownerID,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

func (s *lifecycleServiceImpl) ProcessDailyMaintenance(ctx context.Context) (*MaintenanceReport, error) {
	report := &MaintenanceReport{
		ExecutedAt: time.Now().UTC(),
	}

	// 1. Check Overdue Annuities
	overdue, err := s.annuitySvc.CheckOverdue(ctx, time.Now().UTC())
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("Annuity CheckOverdue: %v", err))
	} else {
		report.OverdueDetected = len(overdue)
	}

	// 2. Refresh Urgencies

	// 3. Generate Reminders
	rems, err := s.deadlineSvc.GenerateReminderBatch(ctx, time.Now().UTC())
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("GenerateReminderBatch: %v", err))
	} else {
		report.RemindersGenerated = len(rems)
	}

	// 4. Auto-expire patents
	expiring, err := s.repo.FindExpiring(ctx, 0)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("FindExpiring: %v", err))
	} else {
		for _, lr := range expiring {
			if lr.CurrentPhase != PhaseExpired {
				lr.TransitionTo(PhaseExpired, "Term expired", "system")
				s.repo.Save(ctx, lr)
				report.PhasesAutoAdvanced++
			}
		}
	}

	return report, nil
}

func (s *lifecycleServiceImpl) GetCostAnalysis(ctx context.Context, portfolioID string, forecastYears int) (*CostAnalysis, error) {
	forecast, err := s.annuitySvc.ForecastCosts(ctx, portfolioID, forecastYears)
	if err != nil {
		return nil, err
	}

	analysis := &CostAnalysis{
		PortfolioID:        portfolioID,
		ForecastCosts:      make(map[int]int64),
		CostByJurisdiction: make(map[string]int64),
		TotalForecast:      forecast.TotalForecastCost.Amount,
		Currency:           forecast.TotalForecastCost.Currency,
	}

	for year, money := range forecast.YearlyCosts {
		analysis.ForecastCosts[year] = money.Amount
	}
	for jur, money := range forecast.CostByJurisdiction {
		analysis.CostByJurisdiction[jur] = money.Amount
	}

	return analysis, nil
}

func (s *lifecycleServiceImpl) GetTimelineView(ctx context.Context, patentID string) (*TimelineView, error) {
	lr, err := s.repo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, err
	}
	if lr == nil {
		return nil, apperrors.NewNotFound("patent lifecycle not found: %s", patentID)
	}

	view := &TimelineView{
		PatentID:         patentID,
		Events:           lr.Events,
		PhaseTransitions: lr.PhaseHistory,
		CurrentPhase:     lr.CurrentPhase,
		FilingDate:       lr.FilingDate,
		ExpirationDate:   lr.ExpirationDate,
	}

	return view, nil
}

//Personal.AI order the ending
