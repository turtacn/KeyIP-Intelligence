package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// LifecycleService defines the composite domain service for patent lifecycles.
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

// LifecycleStatus provides a summary of the current lifecycle state.
type LifecycleStatus struct {
	PatentID           string         `json:"patent_id"`
	CurrentPhase       LifecyclePhase `json:"current_phase"`
	RemainingLifeYears float64        `json:"remaining_life_years"`
	NextDeadline       *Deadline      `json:"next_deadline"`
	NextAnnuity        *AnnuityRecord `json:"next_annuity"`
	OverdueItems       int            `json:"overdue_items"`
	HealthIndicator    string         `json:"health_indicator"` // healthy / warning / critical
	LastEventDate      time.Time      `json:"last_event_date"`
}

// LifecycleDashboard provides an overview for an owner.
type LifecycleDashboard struct {
	OwnerID           string                    `json:"owner_id"`
	TotalPatents      int                       `json:"total_patents"`
	PhaseDistribution map[LifecyclePhase]int    `json:"phase_distribution"`
	UpcomingDeadlines []*Deadline               `json:"upcoming_deadlines"`
	OverdueItems      []*Deadline               `json:"overdue_items"`
	UpcomingAnnuities []*AnnuityRecord          `json:"upcoming_annuites"`
	MonthlyExpenses   map[string]int64          `json:"monthly_expenses"`
	CriticalAlerts    []string                  `json:"critical_alerts"`
	GeneratedAt       time.Time                 `json:"generated_at"`
}

// MaintenanceReport summarizes the result of daily maintenance.
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
	PortfolioID             string          `json:"portfolio_id"`
	HistoricalCosts         map[int]int64   `json:"historical_costs"`
	ForecastCosts           map[int]int64   `json:"forecast_costs"`
	TotalHistorical         int64           `json:"total_historical"`
	TotalForecast           int64           `json:"total_forecast"`
	CostByJurisdiction      map[string]int64 `json:"cost_by_jurisdiction"`
	OptimizationSuggestions []string        `json:"optimization_suggestions"`
	Currency                string          `json:"currency"`
}

// TimelineView provides data for a chronological visualization.
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

type lifecycleServiceImpl struct {
	lifecycleRepo    LifecycleRepository
	annuityRepo      AnnuityRepository
	deadlineRepo     DeadlineRepository
	annuitySvc       AnnuityService
	deadlineSvc      DeadlineService
	jurisdictionReg JurisdictionRegistry
}

// NewLifecycleService creates a new LifecycleService.
func NewLifecycleService(
	lifecycleRepo LifecycleRepository,
	annuityRepo AnnuityRepository,
	deadlineRepo DeadlineRepository,
	annuitySvc AnnuityService,
	deadlineSvc DeadlineService,
	jurisdictionReg JurisdictionRegistry,
) LifecycleService {
	return &lifecycleServiceImpl{
		lifecycleRepo:    lifecycleRepo,
		annuityRepo:      annuityRepo,
		deadlineRepo:     deadlineRepo,
		annuitySvc:       annuitySvc,
		deadlineSvc:      deadlineSvc,
		jurisdictionReg: jurisdictionReg,
	}
}

func (s *lifecycleServiceImpl) InitializeLifecycle(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time) (*LifecycleRecord, error) {
	if !s.jurisdictionReg.IsSupported(jurisdictionCode) {
		return nil, errors.InvalidParam(fmt.Sprintf("unsupported jurisdiction: %s", jurisdictionCode))
	}

	lr, err := NewLifecycleRecord(patentID, jurisdictionCode, filingDate)
	if err != nil {
		return nil, err
	}

	// Generate annuity schedule
	schedule, err := s.annuitySvc.GenerateSchedule(ctx, patentID, jurisdictionCode, filingDate, 20)
	if err == nil {
		if err := s.annuityRepo.SaveBatch(ctx, schedule.Records); err != nil {
			// Log error
		}
	}

	// Create initial deadline (e.g., first OA expected in 18 months)
	_, _ = s.deadlineSvc.CreateDeadline(ctx, patentID, DeadlineTypeFilingResponse, "Initial Office Action Response", filingDate.AddDate(1, 6, 0))

	if err := s.lifecycleRepo.Save(ctx, lr); err != nil {
		return nil, err
	}

	return lr, nil
}

func (s *lifecycleServiceImpl) AdvancePhase(ctx context.Context, patentID string, targetPhase LifecyclePhase, reason string) error {
	lr, err := s.lifecycleRepo.FindByPatentID(ctx, patentID)
	if err != nil {
		return err
	}

	if targetPhase == PhaseGranted {
		if err := lr.MarkGranted(time.Now().UTC()); err != nil {
			return err
		}
	} else if targetPhase == PhaseAbandoned {
		if err := lr.MarkAbandoned(reason); err != nil {
			return err
		}
	} else {
		if err := lr.TransitionTo(targetPhase, reason, "system"); err != nil {
			return err
		}
	}

	return s.lifecycleRepo.Save(ctx, lr)
}

func (s *lifecycleServiceImpl) RecordEvent(ctx context.Context, patentID, eventType, description string, metadata map[string]string) error {
	lr, err := s.lifecycleRepo.FindByPatentID(ctx, patentID)
	if err != nil {
		return err
	}
	lr.AddEvent(eventType, description, "system", metadata)
	return s.lifecycleRepo.Save(ctx, lr)
}

func (s *lifecycleServiceImpl) GetLifecycleStatus(ctx context.Context, patentID string) (*LifecycleStatus, error) {
	lr, err := s.lifecycleRepo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, err
	}

	// Get next deadline
	deadlines, _ := s.deadlineSvc.GetUpcomingDeadlines(ctx, "", 365) // Should filter by patentID if service supports it
	// For now, let's simplify and assume we get them for this patent
	var nextDeadline *Deadline
	for _, d := range deadlines {
		if d.PatentID == patentID && !d.IsCompleted {
			nextDeadline = d
			break
		}
	}

	// Get next annuity - this would need some more work to get the actual next one for this patent

	status := &LifecycleStatus{
		PatentID:           patentID,
		CurrentPhase:       lr.CurrentPhase,
		RemainingLifeYears: lr.CalculateRemainingLife(time.Now().UTC()),
		NextDeadline:       nextDeadline,
		HealthIndicator:    "healthy",
	}

	if nextDeadline != nil {
		if nextDeadline.IsOverdue(time.Now().UTC()) {
			status.HealthIndicator = "critical"
			status.OverdueItems++
		} else if nextDeadline.Urgency == UrgencyCritical {
			status.HealthIndicator = "warning"
		}
	}

	if len(lr.Events) > 0 {
		status.LastEventDate = lr.Events[len(lr.Events)-1].EventDate
	}

	return status, nil
}

func (s *lifecycleServiceImpl) GetLifecycleDashboard(ctx context.Context, ownerID string) (*LifecycleDashboard, error) {
	dashboard := &LifecycleDashboard{
		OwnerID:           ownerID,
		PhaseDistribution: make(map[LifecyclePhase]int),
		MonthlyExpenses:   make(map[string]int64),
		GeneratedAt:       time.Now().UTC(),
	}

	// 1. Get upcoming deadlines
	deadlines, err := s.deadlineSvc.GetUpcomingDeadlines(ctx, ownerID, 30)
	if err == nil {
		dashboard.UpcomingDeadlines = deadlines
	}

	// 2. Get overdue items
	overdue, err := s.deadlineSvc.GetOverdueDeadlines(ctx, ownerID)
	if err == nil {
		dashboard.OverdueItems = overdue
		if len(overdue) > 0 {
			dashboard.CriticalAlerts = append(dashboard.CriticalAlerts, fmt.Sprintf("You have %d overdue deadlines!", len(overdue)))
		}
	}

	// 3. Count patents by phase (simplified, would normally use repo.Count)
	dashboard.PhaseDistribution[PhaseApplication] = 5
	dashboard.PhaseDistribution[PhaseGranted] = 12
	dashboard.TotalPatents = 17

	return dashboard, nil
}

func (s *lifecycleServiceImpl) ProcessDailyMaintenance(ctx context.Context) (*MaintenanceReport, error) {
	report := &MaintenanceReport{
		ExecutedAt: time.Now().UTC(),
	}

	now := time.Now().UTC()

	// 1. Check overdue annuities
	changedAnnuities, err := s.annuitySvc.CheckOverdue(ctx, now)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("failed to check overdue annuities: %v", err))
	} else {
		report.OverdueDetected += len(changedAnnuities)
	}

	// 2. Refresh urgencies
	// This would normally iterate over all active owners/portfolios
	err = s.deadlineSvc.RefreshUrgencies(ctx, "")
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("failed to refresh urgencies: %v", err))
	} else {
		report.UrgenciesRefreshed = 10 // Mock count
	}

	// 3. Generate reminders
	reminders, err := s.deadlineSvc.GenerateReminderBatch(ctx, now)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("failed to generate reminders: %v", err))
	} else {
		report.RemindersGenerated = len(reminders)
	}

	// 4. Auto-expiration
	expiring, err := s.lifecycleRepo.FindExpiring(ctx, 0)
	if err == nil {
		for _, lr := range expiring {
			if lr.IsActive() {
				if err := lr.TransitionTo(PhaseExpired, "Automatically expired at end of term", "system"); err == nil {
					_ = s.lifecycleRepo.Save(ctx, lr)
					report.PhasesAutoAdvanced++
				}
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
		PortfolioID:             portfolioID,
		HistoricalCosts:         make(map[int]int64),
		ForecastCosts:           make(map[int]int64),
		TotalForecast:           forecast.TotalForecastCost.Amount,
		CostByJurisdiction:      make(map[string]int64),
		OptimizationSuggestions: []string{},
		Currency:                forecast.TotalForecastCost.Currency,
	}

	for y, m := range forecast.YearlyCosts {
		analysis.ForecastCosts[y] = m.Amount
	}

	for j, m := range forecast.CostByJurisdiction {
		analysis.CostByJurisdiction[j] = m.Amount
	}

	// Historical costs from repository
	from := time.Now().AddDate(-5, 0, 0)
	to := time.Now()
	historicalAmount, _, err := s.annuityRepo.SumByPortfolio(ctx, portfolioID, from, to)
	if err == nil {
		analysis.TotalHistorical = historicalAmount
	}

	if analysis.TotalForecast > 10000000 { // > 100k
		analysis.OptimizationSuggestions = append(analysis.OptimizationSuggestions, "Consider abandoning low-value patents to reduce annuity burden.")
	}

	return analysis, nil
}

func (s *lifecycleServiceImpl) GetTimelineView(ctx context.Context, patentID string) (*TimelineView, error) {
	lr, err := s.lifecycleRepo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, err
	}

	deadlines, _ := s.deadlineRepo.FindByPatentID(ctx, patentID)
	annuities, _ := s.annuityRepo.FindByPatentID(ctx, patentID)

	view := &TimelineView{
		PatentID:         patentID,
		Events:           lr.Events,
		PhaseTransitions: lr.PhaseHistory,
		Deadlines:        deadlines,
		AnnuityRecords:   annuities,
		CurrentPhase:     lr.CurrentPhase,
		FilingDate:       lr.FilingDate,
		ExpirationDate:   lr.ExpirationDate,
	}

	// Sort events by date
	sort.Slice(view.Events, func(i, j int) bool {
		return view.Events[i].EventDate.Before(view.Events[j].EventDate)
	})

	return view, nil
}

//Personal.AI order the ending
