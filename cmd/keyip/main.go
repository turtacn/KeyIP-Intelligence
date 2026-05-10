// CLI client entry point for KeyIP-Intelligence.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent_mining"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/cli"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Build-time variables injected via ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func init() {
	cli.Version = version
	cli.GitCommit = commit
	cli.BuildDate = buildDate
}

func main() {
	rootCmd := cli.NewRootCommand()

	logger := logging.NewDefaultLogger()
	cfg := config.NewDefaultConfig()

	logger.Info("KeyIP-Intelligence CLI starting",
		logging.String("version", version),
		logging.String("commit", commit))

	// Build all service dependencies -- all use noop/remote-only implementations.
	// These commands will work when connecting to the KeyIP API server via --server.
	deps := buildDependencies(logger, cfg)

	cli.RegisterCommands(rootCmd, deps)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// buildDependencies constructs CLI command dependencies.
// All services require the KeyIP API server; the CLI is a thin client.
func buildDependencies(logger logging.Logger, cfg *config.Config) cli.CommandDependencies {
	return cli.CommandDependencies{
		Logger:                  logger,
		SimilaritySearchService: buildSearchService(logger, cfg),
		ValuationService:        &noopValuationService{},
		DeadlineService:         &noopDeadlineService{},
		AnnuityService:          &noopAnnuityService{},
		LegalStatusService:      &noopLegalStatusService{},
		CalendarService:         &noopCalendarService{},
		FTOReportService:        &noopFTOReportService{},
		InfringementReportService: &noopInfringementReportService{},
		PortfolioReportService:  &noopPortfolioReportService{},
		TemplateService:         &noopTemplateService{},
	}
}

func buildSearchService(logger logging.Logger, cfg *config.Config) patent_mining.SimilaritySearchService {
	return patent_mining.NewSimilaritySearchService(patent_mining.SimilaritySearchDeps{
		FPEngine:     newLocalFingerprintEngine(nil), // no local chemistry service; fingerprints require --server
		VectorStore:  newMilvusVectorStore(cfg, logger),
		PatentIndex:  newLocalPatentIndex(logger),
		HistoryStore: &inMemorySearchHistoryStore{},
		Logger:       &searchLoggerAdapter{logger: logger},
	})
}

// ============================================================================
// SearchLogger adapter -- converts key-value pairs to logging.Field
// ============================================================================

type searchLoggerAdapter struct {
	logger logging.Logger
}

func (a *searchLoggerAdapter) keyvalsToFields(keyvals ...interface{}) []logging.Field {
	fields := make([]logging.Field, 0, len(keyvals)/2)
	for i := 0; i+1 < len(keyvals); i += 2 {
		key, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, logging.Any(key, keyvals[i+1]))
	}
	if len(keyvals)%2 == 1 {
		fields = append(fields, logging.Any("extra", keyvals[len(keyvals)-1]))
	}
	return fields
}

func (a *searchLoggerAdapter) Debug(msg string, keyvals ...interface{}) {
	a.logger.Debug(msg, a.keyvalsToFields(keyvals...)...)
}
func (a *searchLoggerAdapter) Info(msg string, keyvals ...interface{}) {
	a.logger.Info(msg, a.keyvalsToFields(keyvals...)...)
}
func (a *searchLoggerAdapter) Warn(msg string, keyvals ...interface{}) {
	a.logger.Warn(msg, a.keyvalsToFields(keyvals...)...)
}
func (a *searchLoggerAdapter) Error(msg string, keyvals ...interface{}) {
	a.logger.Error(msg, a.keyvalsToFields(keyvals...)...)
}

var errNeedsServer = errors.NewMsg("this command requires the KeyIP API server; use --server <addr> to connect")

type inMemorySearchHistoryStore struct {
	entries []patent_mining.SearchHistoryEntry
}

func (s *inMemorySearchHistoryStore) Save(ctx context.Context, entry *patent_mining.SearchHistoryEntry) error {
	s.entries = append(s.entries, *entry)
	return nil
}
func (s *inMemorySearchHistoryStore) ListByUser(ctx context.Context, userID string, limit int) ([]patent_mining.SearchHistoryEntry, error) {
	if len(s.entries) == 0 {
		return []patent_mining.SearchHistoryEntry{}, nil
	}
	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	result := make([]patent_mining.SearchHistoryEntry, limit)
	copy(result, s.entries[:limit])
	return result, nil
}

// ============================================================================
// Noop service implementations -- return "requires API server" error
// ============================================================================

type noopValuationService struct{}

func (s *noopValuationService) AssessPatent(ctx context.Context, req *portfolio.SinglePatentAssessmentRequest) (*portfolio.SinglePatentAssessmentResponse, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) Assess(ctx context.Context, req *portfolio.CLIValuationRequest) (*portfolio.CLIValuationResult, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) AssessPortfolioCLI(ctx context.Context, req *portfolio.CLIPortfolioAssessRequest) (*portfolio.CLIPortfolioAssessResult, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) AssessPortfolio(ctx context.Context, req *portfolio.PortfolioAssessmentRequest) (*portfolio.PortfolioAssessmentResponse, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) AssessPortfolioFull(ctx context.Context, req *portfolio.PortfolioAssessmentRequest) (*portfolio.PortfolioAssessmentResponse, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) GetAssessmentHistory(ctx context.Context, patentID string, opts ...portfolio.QueryOption) ([]*portfolio.AssessmentRecord, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) CompareAssessments(ctx context.Context, req *portfolio.CompareAssessmentsRequest) (*portfolio.CompareAssessmentsResponse, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) ExportAssessment(ctx context.Context, assessmentID string, format portfolio.ExportFormat) ([]byte, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) GetTierDistribution(ctx context.Context, portfolioID string) (*portfolio.TierDistribution, error) {
	return nil, errNeedsServer
}
func (s *noopValuationService) RecommendActions(ctx context.Context, assessmentID string) ([]*portfolio.ActionRecommendation, error) {
	return nil, errNeedsServer
}

type noopDeadlineService struct{}

func (s *noopDeadlineService) ListDeadlines(ctx context.Context, query *lifecycle.DeadlineQuery) (*lifecycle.DeadlineListResponse, error) {
	return nil, errNeedsServer
}
func (s *noopDeadlineService) CreateDeadline(ctx context.Context, req *lifecycle.CreateDeadlineRequest) (*lifecycle.Deadline, error) {
	return nil, errNeedsServer
}
func (s *noopDeadlineService) CompleteDeadline(ctx context.Context, deadlineID string) error {
	return errNeedsServer
}
func (s *noopDeadlineService) ExtendDeadline(ctx context.Context, req *lifecycle.ExtendDeadlineRequest) (*lifecycle.Deadline, error) {
	return nil, errNeedsServer
}
func (s *noopDeadlineService) DeleteDeadline(ctx context.Context, deadlineID string) error {
	return errNeedsServer
}
func (s *noopDeadlineService) GetComplianceDashboard(ctx context.Context, portfolioID string) (*lifecycle.ComplianceDashboard, error) {
	return nil, errNeedsServer
}
func (s *noopDeadlineService) GetOverdueDeadlines(ctx context.Context, portfolioID string) ([]lifecycle.Deadline, error) {
	return nil, errNeedsServer
}
func (s *noopDeadlineService) SyncStatutoryDeadlines(ctx context.Context, patentID string) (int, error) {
	return 0, errNeedsServer
}

type noopAnnuityService struct{}

func (s *noopAnnuityService) CalculateAnnuity(ctx context.Context, req *lifecycle.CalculateAnnuityRequest) (*lifecycle.AnnuityResult, error) {
	return nil, errNeedsServer
}
func (s *noopAnnuityService) BatchCalculate(ctx context.Context, req *lifecycle.BatchCalculateRequest) (*lifecycle.BatchCalculateResponse, error) {
	return nil, errNeedsServer
}
func (s *noopAnnuityService) GenerateBudget(ctx context.Context, req *lifecycle.GenerateBudgetRequest) (*lifecycle.BudgetReport, error) {
	return nil, errNeedsServer
}
func (s *noopAnnuityService) GetPaymentSchedule(ctx context.Context, req *lifecycle.PaymentScheduleRequest) ([]lifecycle.PaymentScheduleEntry, error) {
	return nil, errNeedsServer
}
func (s *noopAnnuityService) OptimizeCosts(ctx context.Context, req *lifecycle.OptimizeCostsRequest) (*lifecycle.CostOptimizationReport, error) {
	return nil, errNeedsServer
}
func (s *noopAnnuityService) RecordPayment(ctx context.Context, req *lifecycle.RecordPaymentRequest) (*lifecycle.PaymentRecord, error) {
	return nil, errNeedsServer
}
func (s *noopAnnuityService) GetPaymentHistory(ctx context.Context, req *lifecycle.PaymentHistoryRequest) ([]lifecycle.PaymentRecord, int64, error) {
	return nil, 0, errNeedsServer
}

type noopLegalStatusService struct{}

func (s *noopLegalStatusService) SyncStatus(ctx context.Context, patentID string) (*lifecycle.SyncResult, error) {
	return nil, errNeedsServer
}
func (s *noopLegalStatusService) BatchSync(ctx context.Context, req *lifecycle.BatchSyncRequest) (*lifecycle.BatchSyncResult, error) {
	return nil, errNeedsServer
}
func (s *noopLegalStatusService) GetCurrentStatus(ctx context.Context, patentID string) (*lifecycle.LegalStatusDetail, error) {
	return nil, errNeedsServer
}
func (s *noopLegalStatusService) GetStatusHistory(ctx context.Context, patentID string, opts ...lifecycle.QueryOption) ([]*lifecycle.LegalStatusEvent, error) {
	return nil, errNeedsServer
}
func (s *noopLegalStatusService) DetectAnomalies(ctx context.Context, portfolioID string) ([]*lifecycle.StatusAnomaly, error) {
	return nil, errNeedsServer
}
func (s *noopLegalStatusService) SubscribeStatusChange(ctx context.Context, req *lifecycle.SubscriptionRequest) (*lifecycle.Subscription, error) {
	return nil, errNeedsServer
}
func (s *noopLegalStatusService) UnsubscribeStatusChange(ctx context.Context, subscriptionID string) error {
	return errNeedsServer
}
func (s *noopLegalStatusService) GetStatusSummary(ctx context.Context, portfolioID string) (*lifecycle.StatusSummary, error) {
	return nil, errNeedsServer
}
func (s *noopLegalStatusService) ReconcileStatus(ctx context.Context, patentID string) (*lifecycle.ReconcileResult, error) {
	return nil, errNeedsServer
}

type noopCalendarService struct{}

func (s *noopCalendarService) GetCalendarView(ctx context.Context, req *lifecycle.CalendarViewRequest) (*lifecycle.CalendarView, error) {
	return nil, errNeedsServer
}
func (s *noopCalendarService) AddEvent(ctx context.Context, req *lifecycle.AddEventRequest) (*lifecycle.CalendarEvent, error) {
	return nil, errNeedsServer
}
func (s *noopCalendarService) UpdateEventStatus(ctx context.Context, eventID string, status lifecycle.EventStatus) error {
	return errNeedsServer
}
func (s *noopCalendarService) DeleteEvent(ctx context.Context, eventID string) error {
	return errNeedsServer
}
func (s *noopCalendarService) ExportICal(ctx context.Context, req *lifecycle.ICalExportRequest) ([]byte, error) {
	return nil, errNeedsServer
}
func (s *noopCalendarService) GetUpcomingDeadlines(ctx context.Context, portfolioID string, withinDays int) ([]lifecycle.CalendarEvent, error) {
	return nil, errNeedsServer
}

type noopFTOReportService struct{}

func (s *noopFTOReportService) Generate(ctx context.Context, req *reporting.FTOReportRequest) (*reporting.FTOReportResponse, error) {
	return nil, errNeedsServer
}
func (s *noopFTOReportService) GetStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	return nil, errNeedsServer
}
func (s *noopFTOReportService) GetReport(ctx context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
	return nil, errNeedsServer
}
func (s *noopFTOReportService) ListReports(ctx context.Context, filter *reporting.FTOReportFilter, page *common.Pagination) (*common.PaginatedResult[reporting.FTOReportSummary], error) {
	return nil, errNeedsServer
}
func (s *noopFTOReportService) DeleteReport(ctx context.Context, reportID string) error {
	return errNeedsServer
}

type noopInfringementReportService struct{}

func (s *noopInfringementReportService) Generate(ctx context.Context, req *reporting.InfringementReportRequest) (*reporting.InfringementReportResponse, error) {
	return nil, errNeedsServer
}
func (s *noopInfringementReportService) GetStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	return nil, errNeedsServer
}
func (s *noopInfringementReportService) GetReport(ctx context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
	return nil, errNeedsServer
}
func (s *noopInfringementReportService) ListReports(ctx context.Context, filter *reporting.InfringementReportFilter, page *common.Pagination) (*common.PaginatedResult[reporting.InfringementReportSummary], error) {
	return nil, errNeedsServer
}
func (s *noopInfringementReportService) DeleteReport(ctx context.Context, reportID string) error {
	return errNeedsServer
}

type noopPortfolioReportService struct{}

func (s *noopPortfolioReportService) GenerateFullReport(ctx context.Context, req *reporting.PortfolioReportRequest) (*reporting.PortfolioReportResult, error) {
	return nil, errNeedsServer
}
func (s *noopPortfolioReportService) GenerateSummaryReport(ctx context.Context, req *reporting.PortfolioSummaryRequest) (*reporting.PortfolioReportResult, error) {
	return nil, errNeedsServer
}
func (s *noopPortfolioReportService) GenerateGapReport(ctx context.Context, req *reporting.GapReportRequest) (*reporting.PortfolioReportResult, error) {
	return nil, errNeedsServer
}
func (s *noopPortfolioReportService) GenerateCompetitiveReport(ctx context.Context, req *reporting.CompetitiveReportRequest) (*reporting.PortfolioReportResult, error) {
	return nil, errNeedsServer
}
func (s *noopPortfolioReportService) GetReportStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	return nil, errNeedsServer
}
func (s *noopPortfolioReportService) ListReports(ctx context.Context, portfolioID string, opts *reporting.ListReportOptions) (*common.PaginatedResult[reporting.ReportMeta], error) {
	return nil, errNeedsServer
}
func (s *noopPortfolioReportService) ExportReport(ctx context.Context, reportID string, format reporting.ExportFormat) ([]byte, error) {
	return nil, errNeedsServer
}

type noopTemplateService struct{}

func (s *noopTemplateService) Render(ctx context.Context, req *reporting.RenderRequest) (*reporting.RenderResult, error) {
	return nil, errNeedsServer
}
func (s *noopTemplateService) RenderToBytes(ctx context.Context, req *reporting.RenderRequest) ([]byte, error) {
	return nil, errNeedsServer
}
func (s *noopTemplateService) ListTemplates(ctx context.Context, opts *reporting.ListTemplateOptions) (*common.PaginatedResult[reporting.TemplateMeta], error) {
	return nil, errNeedsServer
}
func (s *noopTemplateService) GetTemplate(ctx context.Context, templateID string) (*reporting.Template, error) {
	return nil, errNeedsServer
}
func (s *noopTemplateService) RegisterTemplate(ctx context.Context, tmpl *reporting.Template) error {
	return errNeedsServer
}
func (s *noopTemplateService) UpdateTemplate(ctx context.Context, tmpl *reporting.Template) error {
	return errNeedsServer
}
func (s *noopTemplateService) DeleteTemplate(ctx context.Context, templateID string) error {
	return errNeedsServer
}
func (s *noopTemplateService) ValidateTemplate(ctx context.Context, tmpl *reporting.Template) (*reporting.ValidationResult, error) {
	return nil, errNeedsServer
}
func (s *noopTemplateService) PreviewTemplate(ctx context.Context, templateID string, sampleData map[string]interface{}) (*reporting.RenderResult, error) {
	return nil, errNeedsServer
}

//Personal.AI order the ending
