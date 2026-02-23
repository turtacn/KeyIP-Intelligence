// internal/application/lifecycle/annuity.go
//
// Phase 10 - File #206
// Application service for patent annuity (maintenance fee) management.
// Orchestrates domain-layer annuity calculation, multi-jurisdiction fee rules,
// currency conversion, budget generation, and cost-optimization recommendations.
//
// Functional positioning:
//   Business orchestration layer for annuity fee management. Coordinates domain
//   annuity calculation, jurisdiction rules, and repository persistence. Provides
//   multi-jurisdiction fee calculation, budget compilation, payment reminders,
//   and cost-optimization suggestions.
//
// Core implementation:
//   - AnnuityService interface: CalculateAnnuity / BatchCalculate / GenerateBudget /
//     GetPaymentSchedule / OptimizeCosts / RecordPayment / GetPaymentHistory
//   - annuityServiceImpl struct injecting LifecycleDomainService, AnnuityDomainService,
//     LifecycleRepository, PatentRepository, Logger, Cache
//   - CalculateAnnuity: validate -> fetch patent -> resolve jurisdiction rules ->
//     delegate domain calculation -> currency conversion -> cache result
//   - BatchCalculate: concurrent multi-patent calculation with error aggregation
//   - GenerateBudget: aggregate fees by year/quarter, produce multi-currency budget
//   - OptimizeCosts: recommend abandoning low-value patents based on valuation threshold
//
// Business logic:
//   - Supports CN/US/EP/JP/KR five major jurisdiction fee schedules
//   - Exchange rates: real-time query + local cache (TTL 24h)
//   - Budget supports CNY/USD/EUR/JPY/KRW
//   - Cost optimization uses configurable patent-value score threshold
//
// Dependencies:
//   Depends on: domain/lifecycle, domain/patent, pkg/errors, pkg/types/common
//   Depended by: interfaces/http/handlers/lifecycle_handler, interfaces/cli/lifecycle

package lifecycle

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	domainLifecycle "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Request / Response DTOs
// ---------------------------------------------------------------------------

// CalculateAnnuityRequest holds parameters for a single-patent annuity calculation.
type CalculateAnnuityRequest struct {
	PatentID       string                  `json:"patent_id" validate:"required"`
	Jurisdiction   domainLifecycle.Jurisdiction `json:"jurisdiction" validate:"required"`
	TargetCurrency Currency                `json:"target_currency,omitempty"`
	AsOfDate       time.Time               `json:"as_of_date,omitempty"`
}

// AnnuityResult represents the computed annuity for one patent in one jurisdiction.
type AnnuityResult struct {
	PatentID       string                      `json:"patent_id"`
	PatentNumber   string                      `json:"patent_number"`
	Title          string                      `json:"title"`
	Jurisdiction   domainLifecycle.Jurisdiction `json:"jurisdiction"`
	YearNumber     int                         `json:"year_number"`
	BaseFee        MoneyAmount                 `json:"base_fee"`
	ConvertedFee   MoneyAmount                 `json:"converted_fee,omitempty"`
	DueDate        time.Time                   `json:"due_date"`
	GracePeriodEnd time.Time                   `json:"grace_period_end"`
	Status         AnnuityPaymentStatus        `json:"status"`
}

// BatchCalculateRequest holds parameters for multi-patent annuity calculation.
type BatchCalculateRequest struct {
	PatentIDs      []string                    `json:"patent_ids" validate:"required,min=1"`
	Jurisdiction   domainLifecycle.Jurisdiction `json:"jurisdiction,omitempty"`
	TargetCurrency Currency                    `json:"target_currency,omitempty"`
	AsOfDate       time.Time                   `json:"as_of_date,omitempty"`
}

// BatchCalculateResponse aggregates results and per-patent errors.
type BatchCalculateResponse struct {
	Results    []AnnuityResult       `json:"results"`
	Errors     []BatchItemError      `json:"errors,omitempty"`
	TotalFee   MoneyAmount           `json:"total_fee"`
	CalculatedAt time.Time           `json:"calculated_at"`
}

// BatchItemError captures a per-item failure inside a batch operation.
type BatchItemError struct {
	PatentID string `json:"patent_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// GenerateBudgetRequest defines the scope of a budget report.
type GenerateBudgetRequest struct {
	PortfolioID    string                      `json:"portfolio_id,omitempty"`
	PatentIDs      []string                    `json:"patent_ids,omitempty"`
	Jurisdictions  []domainLifecycle.Jurisdiction `json:"jurisdictions,omitempty"`
	StartDate      time.Time                   `json:"start_date" validate:"required"`
	EndDate        time.Time                   `json:"end_date" validate:"required"`
	GroupBy        BudgetGroupBy               `json:"group_by,omitempty"`
	TargetCurrency Currency                    `json:"target_currency,omitempty"`
}

// BudgetGroupBy enumerates grouping dimensions for budget reports.
type BudgetGroupBy string

const (
	BudgetGroupByYear         BudgetGroupBy = "year"
	BudgetGroupByQuarter      BudgetGroupBy = "quarter"
	BudgetGroupByJurisdiction BudgetGroupBy = "jurisdiction"
	BudgetGroupByPatent       BudgetGroupBy = "patent"
)

// BudgetReport is the output of GenerateBudget.
type BudgetReport struct {
	ID             string            `json:"id"`
	GeneratedAt    time.Time         `json:"generated_at"`
	Period         DateRange         `json:"period"`
	TargetCurrency Currency          `json:"target_currency"`
	TotalFee       MoneyAmount       `json:"total_fee"`
	Items          []BudgetLineItem  `json:"items"`
	Summary        BudgetSummary     `json:"summary"`
}

// BudgetLineItem is one row in the budget report.
type BudgetLineItem struct {
	GroupKey     string      `json:"group_key"`
	PatentID     string      `json:"patent_id,omitempty"`
	PatentNumber string      `json:"patent_number,omitempty"`
	Jurisdiction domainLifecycle.Jurisdiction `json:"jurisdiction"`
	YearNumber   int         `json:"year_number"`
	DueDate      time.Time   `json:"due_date"`
	Fee          MoneyAmount `json:"fee"`
}

// BudgetSummary provides aggregated statistics.
type BudgetSummary struct {
	TotalPatents       int                        `json:"total_patents"`
	TotalPayments      int                        `json:"total_payments"`
	ByJurisdiction     map[string]MoneyAmount      `json:"by_jurisdiction"`
	ByYear             map[int]MoneyAmount         `json:"by_year"`
}

// DateRange is a simple start/end pair.
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// PaymentScheduleRequest defines the query for upcoming payments.
type PaymentScheduleRequest struct {
	PatentID     string    `json:"patent_id,omitempty"`
	PortfolioID  string    `json:"portfolio_id,omitempty"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	TargetCurrency Currency `json:"target_currency,omitempty"`
}

// PaymentScheduleEntry is one upcoming payment.
type PaymentScheduleEntry struct {
	PatentID     string                      `json:"patent_id"`
	PatentNumber string                      `json:"patent_number"`
	Jurisdiction domainLifecycle.Jurisdiction `json:"jurisdiction"`
	YearNumber   int                         `json:"year_number"`
	DueDate      time.Time                   `json:"due_date"`
	Fee          MoneyAmount                 `json:"fee"`
	Status       AnnuityPaymentStatus        `json:"status"`
	DaysUntilDue int                         `json:"days_until_due"`
}

// OptimizeCostsRequest defines parameters for cost-optimization analysis.
type OptimizeCostsRequest struct {
	PortfolioID        string  `json:"portfolio_id" validate:"required"`
	ValueScoreThreshold float64 `json:"value_score_threshold,omitempty"`
	ForecastYears      int     `json:"forecast_years,omitempty"`
	TargetCurrency     Currency `json:"target_currency,omitempty"`
}

// CostOptimizationReport is the output of OptimizeCosts.
type CostOptimizationReport struct {
	PortfolioID          string                    `json:"portfolio_id"`
	GeneratedAt          time.Time                 `json:"generated_at"`
	CurrentAnnualCost    MoneyAmount               `json:"current_annual_cost"`
	OptimizedAnnualCost  MoneyAmount               `json:"optimized_annual_cost"`
	PotentialSavings     MoneyAmount               `json:"potential_savings"`
	Recommendations      []AbandonmentRecommendation `json:"recommendations"`
	ForecastYears        int                       `json:"forecast_years"`
	CumulativeSavings    MoneyAmount               `json:"cumulative_savings"`
}

// AbandonmentRecommendation suggests dropping a low-value patent.
type AbandonmentRecommendation struct {
	PatentID       string      `json:"patent_id"`
	PatentNumber   string      `json:"patent_number"`
	Title          string      `json:"title"`
	ValueScore     float64     `json:"value_score"`
	AnnualCost     MoneyAmount `json:"annual_cost"`
	RemainingLife  int         `json:"remaining_life_years"`
	TotalSavings   MoneyAmount `json:"total_savings"`
	RiskLevel      string      `json:"risk_level"`
	Rationale      string      `json:"rationale"`
}

// RecordPaymentRequest captures a completed payment.
type RecordPaymentRequest struct {
	PatentID      string                      `json:"patent_id" validate:"required"`
	Jurisdiction  domainLifecycle.Jurisdiction `json:"jurisdiction" validate:"required"`
	YearNumber    int                         `json:"year_number" validate:"required,min=1"`
	Amount        MoneyAmount                 `json:"amount" validate:"required"`
	PaidDate      time.Time                   `json:"paid_date" validate:"required"`
	PaymentRef    string                      `json:"payment_ref,omitempty"`
	PaidBy        string                      `json:"paid_by,omitempty"`
	Notes         string                      `json:"notes,omitempty"`
}

// PaymentRecord is a persisted payment entry.
type PaymentRecord struct {
	ID            string                      `json:"id"`
	PatentID      string                      `json:"patent_id"`
	Jurisdiction  domainLifecycle.Jurisdiction `json:"jurisdiction"`
	YearNumber    int                         `json:"year_number"`
	Amount        MoneyAmount                 `json:"amount"`
	PaidDate      time.Time                   `json:"paid_date"`
	PaymentRef    string                      `json:"payment_ref"`
	PaidBy        string                      `json:"paid_by"`
	Notes         string                      `json:"notes"`
	RecordedAt    time.Time                   `json:"recorded_at"`
}

// PaymentHistoryRequest queries past payments.
type PaymentHistoryRequest struct {
	PatentID     string                      `json:"patent_id,omitempty"`
	PortfolioID  string                      `json:"portfolio_id,omitempty"`
	Jurisdiction domainLifecycle.Jurisdiction `json:"jurisdiction,omitempty"`
	StartDate    time.Time                   `json:"start_date,omitempty"`
	EndDate      time.Time                   `json:"end_date,omitempty"`
	Page         int                         `json:"page,omitempty"`
	PageSize     int                         `json:"page_size,omitempty"`
}

// Currency represents an ISO-4217 currency code.
type Currency string

const (
	CurrencyCNY Currency = "CNY"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyJPY Currency = "JPY"
	CurrencyKRW Currency = "KRW"
)

// MoneyAmount pairs a numeric value with its currency.
type MoneyAmount struct {
	Amount   float64  `json:"amount"`
	Currency Currency `json:"currency"`
}

// AnnuityPaymentStatus enumerates payment states.
type AnnuityPaymentStatus string

const (
	AnnuityStatusPending  AnnuityPaymentStatus = "pending"
	AnnuityStatusPaid     AnnuityPaymentStatus = "paid"
	AnnuityStatusOverdue  AnnuityPaymentStatus = "overdue"
	AnnuityStatusGrace    AnnuityPaymentStatus = "grace_period"
	AnnuityStatusWaived   AnnuityPaymentStatus = "waived"
	AnnuityStatusExpired  AnnuityPaymentStatus = "expired"
)

// ---------------------------------------------------------------------------
// Cache key helpers
// ---------------------------------------------------------------------------

const (
	annuityCachePrefix   = "annuity:"
	exchangeRateCacheKey = "exchange_rate:"
	exchangeRateTTL      = 24 * time.Hour
	annuityCacheTTL      = 1 * time.Hour
)

func annuityCacheKey(patentID string, jurisdiction domainLifecycle.Jurisdiction) string {
	return fmt.Sprintf("%s%s:%s", annuityCachePrefix, patentID, jurisdiction)
}

func exchangeRatePairKey(from, to Currency) string {
	return fmt.Sprintf("%s%s_%s", exchangeRateCacheKey, from, to)
}

// ---------------------------------------------------------------------------
// Service interface
// ---------------------------------------------------------------------------

// AnnuityService defines the application-level contract for annuity management.
type AnnuityService interface {
	// CalculateAnnuity computes the next annuity fee for a single patent.
	CalculateAnnuity(ctx context.Context, req *CalculateAnnuityRequest) (*AnnuityResult, error)

	// BatchCalculate computes annuity fees for multiple patents concurrently.
	BatchCalculate(ctx context.Context, req *BatchCalculateRequest) (*BatchCalculateResponse, error)

	// GenerateBudget produces a multi-currency budget report for a date range.
	GenerateBudget(ctx context.Context, req *GenerateBudgetRequest) (*BudgetReport, error)

	// GetPaymentSchedule returns upcoming payment entries.
	GetPaymentSchedule(ctx context.Context, req *PaymentScheduleRequest) ([]PaymentScheduleEntry, error)

	// OptimizeCosts analyses the portfolio and recommends cost-saving abandonments.
	OptimizeCosts(ctx context.Context, req *OptimizeCostsRequest) (*CostOptimizationReport, error)

	// RecordPayment persists a completed annuity payment.
	RecordPayment(ctx context.Context, req *RecordPaymentRequest) (*PaymentRecord, error)

	// GetPaymentHistory retrieves historical payment records.
	GetPaymentHistory(ctx context.Context, req *PaymentHistoryRequest) ([]PaymentRecord, int64, error)
}

// ---------------------------------------------------------------------------
// Adapter interfaces (ports for infrastructure)
// ---------------------------------------------------------------------------

// ExchangeRateProvider abstracts currency conversion.
type ExchangeRateProvider interface {
	GetRate(ctx context.Context, from, to Currency) (float64, error)
}

// PatentValueProvider abstracts patent value scoring (used by OptimizeCosts).
type PatentValueProvider interface {
	GetValueScore(ctx context.Context, patentID string) (float64, error)
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

// annuityServiceImpl is the concrete implementation of AnnuityService.
type annuityServiceImpl struct {
	lifecycleSvc   domainLifecycle.Service
	lifecycleRepo  domainLifecycle.LifecycleRepository
	patentRepo     domainPatent.PatentRepository
	exchangeRate   ExchangeRateProvider
	valueProvider  PatentValueProvider
	cache          CachePort
	logger         Logger

	// Configuration
	defaultCurrency        Currency
	valueScoreThreshold    float64
	defaultForecastYears   int
	batchConcurrencyLimit  int
}

// AnnuityServiceConfig holds tunables for the annuity service.
type AnnuityServiceConfig struct {
	DefaultCurrency       Currency `yaml:"default_currency"`
	ValueScoreThreshold   float64  `yaml:"value_score_threshold"`
	DefaultForecastYears  int      `yaml:"default_forecast_years"`
	BatchConcurrencyLimit int      `yaml:"batch_concurrency_limit"`
}

// NewAnnuityService constructs an AnnuityService with all required dependencies.
func NewAnnuityService(
	lifecycleSvc domainLifecycle.Service,
	lifecycleRepo domainLifecycle.LifecycleRepository,
	patentRepo domainPatent.PatentRepository,
	exchangeRate ExchangeRateProvider,
	valueProvider PatentValueProvider,
	cache CachePort,
	logger Logger,
	cfg AnnuityServiceConfig,
) AnnuityService {
	if cfg.DefaultCurrency == "" {
		cfg.DefaultCurrency = CurrencyCNY
	}
	if cfg.ValueScoreThreshold <= 0 {
		cfg.ValueScoreThreshold = 40.0
	}
	if cfg.DefaultForecastYears <= 0 {
		cfg.DefaultForecastYears = 5
	}
	if cfg.BatchConcurrencyLimit <= 0 {
		cfg.BatchConcurrencyLimit = 20
	}
	return &annuityServiceImpl{
		lifecycleSvc:          lifecycleSvc,
		lifecycleRepo:         lifecycleRepo,
		patentRepo:            patentRepo,
		exchangeRate:          exchangeRate,
		valueProvider:         valueProvider,
		cache:                 cache,
		logger:                logger,
		defaultCurrency:       cfg.DefaultCurrency,
		valueScoreThreshold:   cfg.ValueScoreThreshold,
		defaultForecastYears:  cfg.DefaultForecastYears,
		batchConcurrencyLimit: cfg.BatchConcurrencyLimit,
	}
}

// CalculateAnnuity computes the next annuity fee for a single patent.
func (s *annuityServiceImpl) CalculateAnnuity(ctx context.Context, req *CalculateAnnuityRequest) (*AnnuityResult, error) {
	if req == nil {
		return nil, errors.NewValidationOp("annuity.calculate", "request must not be nil")
	}
	if req.PatentID == "" {
		return nil, errors.NewValidationOp("annuity.calculate", "patent_id is required")
	}
	if req.Jurisdiction == "" {
		return nil, errors.NewValidationOp("annuity.calculate", "jurisdiction is required")
	}

	targetCurrency := req.TargetCurrency
	if targetCurrency == "" {
		targetCurrency = s.defaultCurrency
	}
	asOf := req.AsOfDate
	if asOf.IsZero() {
		asOf = time.Now()
	}

	// Check cache
	cacheKey := annuityCacheKey(req.PatentID, req.Jurisdiction)
	var cached AnnuityResult
	if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
		s.logger.Info("annuity cache hit", "patent_id", req.PatentID)
		if targetCurrency != cached.ConvertedFee.Currency {
			converted, convErr := s.convertCurrency(ctx, cached.BaseFee, targetCurrency)
			if convErr != nil {
				return nil, convErr
			}
			cached.ConvertedFee = converted
		}
		return &cached, nil
	}

	// Fetch patent
	patentID, err := uuid.Parse(req.PatentID)
	if err != nil {
		return nil, errors.NewValidationOp("annuity.calculate", fmt.Sprintf("invalid patent_id: %s", req.PatentID))
	}

	patent, err := s.patentRepo.GetByID(ctx, patentID)
	if err != nil {
		s.logger.Error("failed to fetch patent", "patent_id", req.PatentID, "error", err)
		return nil, errors.NewNotFoundOp("annuity.calculate", fmt.Sprintf("patent %s not found", req.PatentID))
	}

	// Delegate to domain service for fee calculation
	domainAnnuity, err := s.lifecycleSvc.CalculateAnnuityFee(ctx, patent.ID.String(), req.Jurisdiction, asOf)
	if err != nil {
		s.logger.Error("domain annuity calculation failed", "patent_id", req.PatentID, "error", err)
		return nil, errors.NewInternalOp("annuity.calculate", fmt.Sprintf("fee calculation failed: %v", err))
	}

	baseFee := MoneyAmount{
		Amount:   domainAnnuity.Fee,
		Currency: jurisdictionBaseCurrency(req.Jurisdiction),
	}

	convertedFee, err := s.convertCurrency(ctx, baseFee, targetCurrency)
	if err != nil {
		return nil, err
	}

	result := &AnnuityResult{
		PatentID:       patent.ID.String(),
		PatentNumber:   patent.PatentNumber,
		Title:          patent.Title,
		Jurisdiction:   req.Jurisdiction,
		YearNumber:     domainAnnuity.YearNumber,
		BaseFee:        baseFee,
		ConvertedFee:   convertedFee,
		DueDate:        domainAnnuity.DueDate,
		GracePeriodEnd: domainAnnuity.GracePeriodEnd,
		Status:         mapDomainPaymentStatus(domainAnnuity.Status, asOf, domainAnnuity.DueDate, domainAnnuity.GracePeriodEnd),
	}

	// Populate cache (ignore error, non-critical)
	_ = s.cache.Set(ctx, cacheKey, result, annuityCacheTTL)

	s.logger.Info("annuity calculated",
		"patent_id", req.PatentID,
		"jurisdiction", req.Jurisdiction,
		"year", domainAnnuity.YearNumber,
		"fee", convertedFee.Amount,
		"currency", convertedFee.Currency,
	)

	return result, nil
}

// BatchCalculate computes annuity fees for multiple patents concurrently.
func (s *annuityServiceImpl) BatchCalculate(ctx context.Context, req *BatchCalculateRequest) (*BatchCalculateResponse, error) {
	if req == nil {
		return nil, errors.NewValidationOp("annuity.batch", "request must not be nil")
	}
	if len(req.PatentIDs) == 0 {
		return nil, errors.NewValidationOp("annuity.batch", "patent_ids must not be empty")
	}

	asOf := req.AsOfDate
	if asOf.IsZero() {
		asOf = time.Now()
	}
	targetCurrency := req.TargetCurrency
	if targetCurrency == "" {
		targetCurrency = s.defaultCurrency
	}

	type itemResult struct {
		result *AnnuityResult
		err    *BatchItemError
	}

	results := make([]itemResult, len(req.PatentIDs))
	sem := make(chan struct{}, s.batchConcurrencyLimit)
	var wg sync.WaitGroup

	for i, pid := range req.PatentIDs {
		wg.Add(1)
		go func(idx int, patentID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			jurisdiction := req.Jurisdiction
			if jurisdiction == "" {
				// Resolve primary jurisdiction from patent record
			uid, _ := uuid.Parse(patentID)
			pat, fetchErr := s.patentRepo.GetByID(ctx, uid)
				if fetchErr != nil {
					results[idx] = itemResult{err: &BatchItemError{
						PatentID: patentID,
						Code:     "NOT_FOUND",
						Message:  fmt.Sprintf("patent not found: %v", fetchErr),
					}}
					return
				}
				jurisdiction = domainLifecycle.Jurisdiction(pat.Jurisdiction)
			}

			singleReq := &CalculateAnnuityRequest{
				PatentID:       patentID,
				Jurisdiction:   jurisdiction,
				TargetCurrency: targetCurrency,
				AsOfDate:       asOf,
			}
			res, calcErr := s.CalculateAnnuity(ctx, singleReq)
			if calcErr != nil {
				results[idx] = itemResult{err: &BatchItemError{
					PatentID: patentID,
					Code:     "CALC_FAILED",
					Message:  calcErr.Error(),
				}}
				return
			}
			results[idx] = itemResult{result: res}
		}(i, pid)
	}
	wg.Wait()

	resp := &BatchCalculateResponse{
		Results:      make([]AnnuityResult, 0, len(req.PatentIDs)),
		Errors:       make([]BatchItemError, 0),
		CalculatedAt: time.Now(),
	}
	var totalAmount float64
	for _, ir := range results {
		if ir.err != nil {
			resp.Errors = append(resp.Errors, *ir.err)
			continue
		}
		if ir.result != nil {
			resp.Results = append(resp.Results, *ir.result)
			totalAmount += ir.result.ConvertedFee.Amount
		}
	}
	resp.TotalFee = MoneyAmount{Amount: totalAmount, Currency: targetCurrency}

	s.logger.Info("batch annuity calculated",
		"total", len(req.PatentIDs),
		"success", len(resp.Results),
		"errors", len(resp.Errors),
	)

	return resp, nil
}

// GenerateBudget produces a multi-currency budget report for a date range.
func (s *annuityServiceImpl) GenerateBudget(ctx context.Context, req *GenerateBudgetRequest) (*BudgetReport, error) {
	if req == nil {
		return nil, errors.NewValidationOp("annuity.budget", "request must not be nil")
	}
	if req.StartDate.IsZero() || req.EndDate.IsZero() {
		return nil, errors.NewValidationOp("annuity.budget", "start_date and end_date are required")
	}
	if req.EndDate.Before(req.StartDate) {
		return nil, errors.NewValidationOp("annuity.budget", "end_date must be after start_date")
	}

	targetCurrency := req.TargetCurrency
	if targetCurrency == "" {
		targetCurrency = s.defaultCurrency
	}
	groupBy := req.GroupBy
	if groupBy == "" {
		groupBy = BudgetGroupByYear
	}

	// Resolve patent IDs
	patentIDs := req.PatentIDs
	if len(patentIDs) == 0 && req.PortfolioID != "" {
		patents, err := s.patentRepo.ListByPortfolio(ctx, req.PortfolioID)
		if err != nil {
			return nil, errors.NewInternalOp("annuity.budget", fmt.Sprintf("failed to list portfolio patents: %v", err))
		}
		for _, p := range patents {
			patentIDs = append(patentIDs, p.ID.String())
		}
	}
	if len(patentIDs) == 0 {
		return nil, errors.NewValidationOp("annuity.budget", "no patents specified or found in portfolio")
	}

	jurisdictions := req.Jurisdictions
	if len(jurisdictions) == 0 {
		jurisdictions = []domainLifecycle.Jurisdiction{
			domainLifecycle.JurisdictionCN,
			domainLifecycle.JurisdictionUS,
			domainLifecycle.JurisdictionEP,
			domainLifecycle.JurisdictionJP,
			domainLifecycle.JurisdictionKR,
		}
	}

	var items []BudgetLineItem
	byJurisdiction := make(map[string]float64)
	byYear := make(map[int]float64)
	var totalAmount float64

	for _, pid := range patentIDs {
		uid, _ := uuid.Parse(pid)
		patent, err := s.patentRepo.GetByID(ctx, uid)
		if err != nil {
			s.logger.Warn("budget: skipping patent", "patent_id", pid, "error", err)
			continue
		}

		patentJurisdiction := domainLifecycle.Jurisdiction(patent.Jurisdiction)
		matched := false
		for _, j := range jurisdictions {
			if j == patentJurisdiction {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		schedule, err := s.lifecycleSvc.GetAnnuitySchedule(ctx, pid, patentJurisdiction, req.StartDate, req.EndDate)
		if err != nil {
			s.logger.Warn("budget: schedule fetch failed", "patent_id", pid, "error", err)
			continue
		}

		for _, entry := range schedule {
			if entry.DueDate.Before(req.StartDate) || entry.DueDate.After(req.EndDate) {
				continue
			}

			baseFee := MoneyAmount{
				Amount:   entry.Fee,
				Currency: jurisdictionBaseCurrency(patentJurisdiction),
			}
			converted, convErr := s.convertCurrency(ctx, baseFee, targetCurrency)
			if convErr != nil {
				s.logger.Warn("budget: currency conversion failed", "patent_id", pid, "error", convErr)
				converted = baseFee
			}

			groupKey := buildGroupKey(groupBy, entry.DueDate, patentJurisdiction, pid)

			item := BudgetLineItem{
				GroupKey:     groupKey,
				PatentID:     pid,
				PatentNumber: patent.PatentNumber,
				Jurisdiction: patentJurisdiction,
				YearNumber:   entry.YearNumber,
				DueDate:      entry.DueDate,
				Fee:          converted,
			}
			items = append(items, item)

			totalAmount += converted.Amount
			byJurisdiction[string(patentJurisdiction)] += converted.Amount
			byYear[entry.DueDate.Year()] += converted.Amount
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].DueDate.Before(items[j].DueDate)
	})

	summaryByJurisdiction := make(map[string]MoneyAmount, len(byJurisdiction))
	for k, v := range byJurisdiction {
		summaryByJurisdiction[k] = MoneyAmount{Amount: v, Currency: targetCurrency}
	}
	summaryByYear := make(map[int]MoneyAmount, len(byYear))
	for k, v := range byYear {
		summaryByYear[k] = MoneyAmount{Amount: v, Currency: targetCurrency}
	}

	uniquePatents := make(map[string]struct{})
	for _, item := range items {
		uniquePatents[item.PatentID] = struct{}{}
	}

	report := &BudgetReport{
		ID:             fmt.Sprintf("budget-%d", time.Now().UnixNano()),
		GeneratedAt:    time.Now(),
		Period:         DateRange{Start: req.StartDate, End: req.EndDate},
		TargetCurrency: targetCurrency,
		TotalFee:       MoneyAmount{Amount: totalAmount, Currency: targetCurrency},
		Items:          items,
		Summary: BudgetSummary{
			TotalPatents:   len(uniquePatents),
			TotalPayments:  len(items),
			ByJurisdiction: summaryByJurisdiction,
			ByYear:         summaryByYear,
		},
	}

	s.logger.Info("budget generated",
		"patents", len(uniquePatents),
		"payments", len(items),
		"total", totalAmount,
		"currency", targetCurrency,
	)

	return report, nil
}

// GetPaymentSchedule returns upcoming payment entries.
func (s *annuityServiceImpl) GetPaymentSchedule(ctx context.Context, req *PaymentScheduleRequest) ([]PaymentScheduleEntry, error) {
	if req == nil {
		return nil, errors.NewValidationOp("annuity.schedule", "request must not be nil")
	}

	startDate := req.StartDate
	if startDate.IsZero() {
		startDate = time.Now()
	}
	endDate := req.EndDate
	if endDate.IsZero() {
		endDate = startDate.AddDate(1, 0, 0)
	}
	targetCurrency := req.TargetCurrency
	if targetCurrency == "" {
		targetCurrency = s.defaultCurrency
	}

	var patentIDs []string
	if req.PatentID != "" {
		patentIDs = []string{req.PatentID}
	} else if req.PortfolioID != "" {
		patents, err := s.patentRepo.ListByPortfolio(ctx, req.PortfolioID)
		if err != nil {
			return nil, errors.NewInternalOp("annuity.schedule", fmt.Sprintf("failed to list portfolio: %v", err))
		}
		for _, p := range patents {
			patentIDs = append(patentIDs, p.ID.String())
		}
	} else {
		return nil, errors.NewValidationOp("annuity.schedule", "patent_id or portfolio_id is required")
	}

	now := time.Now()
	var entries []PaymentScheduleEntry

	for _, pid := range patentIDs {
		uid, _ := uuid.Parse(pid)
		patent, err := s.patentRepo.GetByID(ctx, uid)
		if err != nil {
			s.logger.Warn("schedule: skipping patent", "patent_id", pid, "error", err)
			continue
		}

		jurisdiction := domainLifecycle.Jurisdiction(patent.Jurisdiction)
		schedule, err := s.lifecycleSvc.GetAnnuitySchedule(ctx, pid, jurisdiction, startDate, endDate)
		if err != nil {
			s.logger.Warn("schedule: fetch failed", "patent_id", pid, "error", err)
			continue
		}

		for _, se := range schedule {
			if se.DueDate.Before(startDate) || se.DueDate.After(endDate) {
				continue
			}

			baseFee := MoneyAmount{
				Amount:   se.Fee,
				Currency: jurisdictionBaseCurrency(jurisdiction),
			}
			converted, convErr := s.convertCurrency(ctx, baseFee, targetCurrency)
			if convErr != nil {
				converted = baseFee
			}

			daysUntil := int(se.DueDate.Sub(now).Hours() / 24)
			status := mapDomainPaymentStatus(se.Status, now, se.DueDate, se.GracePeriodEnd)

			entries = append(entries, PaymentScheduleEntry{
				PatentID:     pid,
				PatentNumber: patent.PatentNumber,
				Jurisdiction: jurisdiction,
				YearNumber:   se.YearNumber,
				DueDate:      se.DueDate,
				Fee:          converted,
				Status:       status,
				DaysUntilDue: daysUntil,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].DueDate.Before(entries[j].DueDate)
	})

	return entries, nil
}

// OptimizeCosts analyses the portfolio and recommends cost-saving abandonments.
func (s *annuityServiceImpl) OptimizeCosts(ctx context.Context, req *OptimizeCostsRequest) (*CostOptimizationReport, error) {
	if req == nil {
		return nil, errors.NewValidationOp("annuity.optimize", "request must not be nil")
	}
	if req.PortfolioID == "" {
		return nil, errors.NewValidationOp("annuity.optimize", "portfolio_id is required")
	}

	threshold := req.ValueScoreThreshold
	if threshold <= 0 {
		threshold = s.valueScoreThreshold
	}
	forecastYears := req.ForecastYears
	if forecastYears <= 0 {
		forecastYears = s.defaultForecastYears
	}
	targetCurrency := req.TargetCurrency
	if targetCurrency == "" {
		targetCurrency = s.defaultCurrency
	}

	patents, err := s.patentRepo.ListByPortfolio(ctx, req.PortfolioID)
	if err != nil {
		return nil, errors.NewInternalOp("annuity.optimize", fmt.Sprintf("failed to list portfolio: %v", err))
	}
	if len(patents) == 0 {
		return nil, errors.NewNotFoundOp("annuity.optimize", "no patents found in portfolio")
	}

	now := time.Now()
	forecastEnd := now.AddDate(forecastYears, 0, 0)

	var currentTotal float64
	var recommendations []AbandonmentRecommendation

	for _, patent := range patents {
		jurisdiction := domainLifecycle.Jurisdiction(patent.Jurisdiction)

		// Get annual cost for this patent
		schedule, schedErr := s.lifecycleSvc.GetAnnuitySchedule(ctx, patent.ID.String(), jurisdiction, now, forecastEnd)
		if schedErr != nil {
			s.logger.Warn("optimize: schedule fetch failed", "patent_id", patent.ID.String(), "error", schedErr)
			continue
		}

		var annualCost float64
		var totalForecastCost float64
		for _, entry := range schedule {
			baseFee := MoneyAmount{Amount: entry.Fee, Currency: jurisdictionBaseCurrency(jurisdiction)}
			converted, convErr := s.convertCurrency(ctx, baseFee, targetCurrency)
			if convErr != nil {
				converted = baseFee
			}
			totalForecastCost += converted.Amount
		}
		if len(schedule) > 0 {
			annualCost = totalForecastCost / float64(forecastYears)
		}
		currentTotal += annualCost

		// Get value score
		valueScore, valErr := s.valueProvider.GetValueScore(ctx, patent.ID.String())
		if valErr != nil {
			s.logger.Warn("optimize: value score unavailable", "patent_id", patent.ID.String(), "error", valErr)
			valueScore = 50.0 // default mid-range
		}

		if valueScore < threshold {
			filingDate := time.Time{}
			if patent.FilingDate != nil {
				filingDate = *patent.FilingDate
			}
			remainingLife := estimateRemainingLife(filingDate, jurisdiction)
			riskLevel := classifyAbandonmentRisk(valueScore, threshold)
			rationale := buildAbandonmentRationale(valueScore, threshold, annualCost, remainingLife, targetCurrency)

			recommendations = append(recommendations, AbandonmentRecommendation{
				PatentID:      patent.ID.String(),
				PatentNumber:  patent.PatentNumber,
				Title:         patent.Title,
				ValueScore:    valueScore,
				AnnualCost:    MoneyAmount{Amount: annualCost, Currency: targetCurrency},
				RemainingLife: remainingLife,
				TotalSavings:  MoneyAmount{Amount: annualCost * float64(remainingLife), Currency: targetCurrency},
				RiskLevel:     riskLevel,
				Rationale:     rationale,
			})
		}
	}

	// Sort recommendations by total savings descending
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].TotalSavings.Amount > recommendations[j].TotalSavings.Amount
	})

	var savingsAnnual float64
	for _, rec := range recommendations {
		savingsAnnual += rec.AnnualCost.Amount
	}

	var cumulativeSavings float64
	for _, rec := range recommendations {
		cumulativeSavings += rec.TotalSavings.Amount
	}

	report := &CostOptimizationReport{
		PortfolioID:         req.PortfolioID,
		GeneratedAt:         time.Now(),
		CurrentAnnualCost:   MoneyAmount{Amount: currentTotal, Currency: targetCurrency},
		OptimizedAnnualCost: MoneyAmount{Amount: currentTotal - savingsAnnual, Currency: targetCurrency},
		PotentialSavings:    MoneyAmount{Amount: savingsAnnual, Currency: targetCurrency},
		Recommendations:     recommendations,
		ForecastYears:       forecastYears,
		CumulativeSavings:   MoneyAmount{Amount: cumulativeSavings, Currency: targetCurrency},
	}

	s.logger.Info("cost optimization completed",
		"portfolio", req.PortfolioID,
		"patents", len(patents),
		"recommendations", len(recommendations),
		"annual_savings", savingsAnnual,
		"currency", targetCurrency,
	)

	return report, nil
}

// RecordPayment persists a completed annuity payment.
func (s *annuityServiceImpl) RecordPayment(ctx context.Context, req *RecordPaymentRequest) (*PaymentRecord, error) {
	if req == nil {
		return nil, errors.NewValidationOp("annuity.record", "request must not be nil")
	}
	if req.PatentID == "" {
		return nil, errors.NewValidationOp("annuity.record", "patent_id is required")
	}
	if req.Jurisdiction == "" {
		return nil, errors.NewValidationOp("annuity.record", "jurisdiction is required")
	}
	if req.YearNumber < 1 {
		return nil, errors.NewValidationOp("annuity.record", "year_number must be >= 1")
	}
	if req.Amount.Amount <= 0 {
		return nil, errors.NewValidationOp("annuity.record", "amount must be positive")
	}
	if req.PaidDate.IsZero() {
		return nil, errors.NewValidationOp("annuity.record", "paid_date is required")
	}

	// Verify patent exists
	uid, err := uuid.Parse(req.PatentID)
	if err != nil {
		return nil, errors.NewValidationOp("annuity.record", fmt.Sprintf("invalid patent_id: %s", req.PatentID))
	}
	_, err = s.patentRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, errors.NewNotFoundOp("annuity.record", fmt.Sprintf("patent %s not found", req.PatentID))
	}

	// Persist via domain/repository
	domainPayment := &domainLifecycle.PaymentRecord{
		PatentID:     req.PatentID,
		Jurisdiction: req.Jurisdiction,
		YearNumber:   req.YearNumber,
		Amount:       req.Amount.Amount,
		Currency:     string(req.Amount.Currency),
		PaidDate:     req.PaidDate,
		PaymentRef:   req.PaymentRef,
		PaidBy:       req.PaidBy,
		Notes:        req.Notes,
	}

	saved, err := s.lifecycleRepo.SavePayment(ctx, domainPayment)
	if err != nil {
		s.logger.Error("failed to save payment", "patent_id", req.PatentID, "error", err)
		return nil, errors.NewInternalOp("annuity.record", fmt.Sprintf("failed to save payment: %v", err))
	}

	// Invalidate annuity cache for this patent
	cacheKey := annuityCacheKey(req.PatentID, req.Jurisdiction)
	_ = s.cache.Delete(ctx, cacheKey)

	record := &PaymentRecord{
		ID:           saved.ID,
		PatentID:     saved.PatentID,
		Jurisdiction: saved.Jurisdiction,
		YearNumber:   saved.YearNumber,
		Amount:       MoneyAmount{Amount: saved.Amount, Currency: Currency(saved.Currency)},
		PaidDate:     saved.PaidDate,
		PaymentRef:   saved.PaymentRef,
		PaidBy:       saved.PaidBy,
		Notes:        saved.Notes,
		RecordedAt:   saved.RecordedAt,
	}

	s.logger.Info("payment recorded",
		"patent_id", req.PatentID,
		"jurisdiction", req.Jurisdiction,
		"year", req.YearNumber,
		"amount", req.Amount.Amount,
	)

	return record, nil
}

// GetPaymentHistory retrieves historical payment records.
func (s *annuityServiceImpl) GetPaymentHistory(ctx context.Context, req *PaymentHistoryRequest) ([]PaymentRecord, int64, error) {
	if req == nil {
		return nil, 0, errors.NewValidationOp("annuity.history", "request must not be nil")
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	query := &domainLifecycle.PaymentQuery{
		PatentID:     req.PatentID,
		PortfolioID:  req.PortfolioID,
		Jurisdiction: req.Jurisdiction,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		Offset:       (page - 1) * pageSize,
		Limit:        pageSize,
	}

	domainRecords, total, err := s.lifecycleRepo.QueryPayments(ctx, query)
	if err != nil {
		s.logger.Error("failed to query payment history", "error", err)
		return nil, 0, errors.NewInternalOp("annuity.history", fmt.Sprintf("query failed: %v", err))
	}

	records := make([]PaymentRecord, 0, len(domainRecords))
	for _, dr := range domainRecords {
		records = append(records, PaymentRecord{
			ID:           dr.ID,
			PatentID:     dr.PatentID,
			Jurisdiction: dr.Jurisdiction,
			YearNumber:   dr.YearNumber,
			Amount:       MoneyAmount{Amount: dr.Amount, Currency: Currency(dr.Currency)},
			PaidDate:     dr.PaidDate,
			PaymentRef:   dr.PaymentRef,
			PaidBy:       dr.PaidBy,
			Notes:        dr.Notes,
			RecordedAt:   dr.RecordedAt,
		})
	}

	return records, total, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// convertCurrency converts a MoneyAmount to the target currency using the
// exchange rate provider with a 24-hour cache layer.
func (s *annuityServiceImpl) convertCurrency(ctx context.Context, from MoneyAmount, to Currency) (MoneyAmount, error) {
	if from.Currency == to {
		return MoneyAmount{Amount: from.Amount, Currency: to}, nil
	}

	cacheKey := exchangeRatePairKey(from.Currency, to)
	var rate float64

	if err := s.cache.Get(ctx, cacheKey, &rate); err == nil && rate > 0 {
		return MoneyAmount{Amount: from.Amount * rate, Currency: to}, nil
	}

	var fetchErr error
	rate, fetchErr = s.exchangeRate.GetRate(ctx, from.Currency, to)
	if fetchErr != nil {
		s.logger.Error("exchange rate fetch failed",
			"from", from.Currency, "to", to, "error", fetchErr)
		return MoneyAmount{}, errors.NewInternalOp("annuity.currency",
			fmt.Sprintf("exchange rate %s->%s unavailable: %v", from.Currency, to, fetchErr))
	}

	_ = s.cache.Set(ctx, cacheKey, rate, exchangeRateTTL)

	return MoneyAmount{Amount: from.Amount * rate, Currency: to}, nil
}

// jurisdictionBaseCurrency returns the native currency for a jurisdiction.
func jurisdictionBaseCurrency(j domainLifecycle.Jurisdiction) Currency {
	switch j {
	case domainLifecycle.JurisdictionCN:
		return CurrencyCNY
	case domainLifecycle.JurisdictionUS:
		return CurrencyUSD
	case domainLifecycle.JurisdictionEP:
		return CurrencyEUR
	case domainLifecycle.JurisdictionJP:
		return CurrencyJPY
	case domainLifecycle.JurisdictionKR:
		return CurrencyKRW
	default:
		return CurrencyUSD
	}
}

// mapDomainPaymentStatus translates domain status + temporal context into
// the application-layer AnnuityPaymentStatus.
func mapDomainPaymentStatus(domainStatus string, now, dueDate, gracePeriodEnd time.Time) AnnuityPaymentStatus {
	switch domainStatus {
	case "paid":
		return AnnuityStatusPaid
	case "waived":
		return AnnuityStatusWaived
	case "expired":
		return AnnuityStatusExpired
	default:
		if now.After(gracePeriodEnd) {
			return AnnuityStatusOverdue
		}
		if now.After(dueDate) {
			return AnnuityStatusGrace
		}
		return AnnuityStatusPending
	}
}

// buildGroupKey creates the grouping key for budget line items.
func buildGroupKey(groupBy BudgetGroupBy, dueDate time.Time, jurisdiction domainLifecycle.Jurisdiction, patentID string) string {
	switch groupBy {
	case BudgetGroupByQuarter:
		q := (dueDate.Month()-1)/3 + 1
		return fmt.Sprintf("%d-Q%d", dueDate.Year(), q)
	case BudgetGroupByJurisdiction:
		return string(jurisdiction)
	case BudgetGroupByPatent:
		return patentID
	default: // BudgetGroupByYear
		return fmt.Sprintf("%d", dueDate.Year())
	}
}

// estimateRemainingLife estimates remaining patent life in years.
// Standard patent term is 20 years from filing for most jurisdictions.
func estimateRemainingLife(filingDate time.Time, jurisdiction domainLifecycle.Jurisdiction) int {
	maxTerm := 20
	switch jurisdiction {
	case domainLifecycle.JurisdictionUS:
		maxTerm = 20
	case domainLifecycle.JurisdictionCN:
		maxTerm = 20
	case domainLifecycle.JurisdictionEP:
		maxTerm = 20
	case domainLifecycle.JurisdictionJP:
		maxTerm = 20
	case domainLifecycle.JurisdictionKR:
		maxTerm = 20
	}

	if filingDate.IsZero() {
		return maxTerm
	}

	expiry := filingDate.AddDate(maxTerm, 0, 0)
	remaining := int(expiry.Sub(time.Now()).Hours() / (24 * 365))
	if remaining < 0 {
		return 0
	}
	return remaining
}

// classifyAbandonmentRisk categorizes the risk of abandoning a patent.
func classifyAbandonmentRisk(valueScore, threshold float64) string {
	ratio := valueScore / threshold
	switch {
	case ratio < 0.3:
		return "low"
	case ratio < 0.6:
		return "medium"
	default:
		return "high"
	}
}

// buildAbandonmentRationale generates a human-readable explanation.
func buildAbandonmentRationale(valueScore, threshold, annualCost float64, remainingLife int, currency Currency) string {
	return fmt.Sprintf(
		"Patent value score (%.1f) is below the threshold (%.1f). "+
			"Annual maintenance cost is %.2f %s with %d years remaining. "+
			"Abandoning would save approximately %.2f %s over the remaining patent life.",
		valueScore, threshold,
		annualCost, currency,
		remainingLife,
		annualCost*float64(remainingLife), currency,
	)
}

// Ensure interface compliance at compile time.
var _ AnnuityService = (*annuityServiceImpl)(nil)

//Personal.AI order the ending
