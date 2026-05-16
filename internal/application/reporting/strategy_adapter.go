// strategy_adapter.go — StrategyGPT ReportGenerator → FTOReportService bridge.
// Wires the RAG-augmented LLM reporting engine into the application-layer
// FTOReportService contract so the HTTP handler can serve AI-generated
// freedom-to-operate reports with patent-literature context retrieval.
package reporting

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	strategy "github.com/turtacn/KeyIP-Intelligence/internal/intelligence/strategy_gpt"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	pkgcommon "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// StrategyFTOReportService — concrete FTOReportService backed by StrategyGPT
// ---------------------------------------------------------------------------

// reportEntry holds the in-memory state for an in-progress or completed report.
type reportEntry struct {
	Status      ReportStatus
	ProgressPct int
	Message     string
	Content     []byte // generated report bytes
	ContentType string
	CreatedAt   time.Time
	Title       string
}

// StrategyFTOReportService implements FTOReportService by delegating to
// StrategyGPT's ReportGenerator with RAG-augmented retrieval.
//
// Architecture:
//
//	HTTP Handler → FTOReportService (this adapter)
//	                  → strategy_gpt.ReportGenerator
//	                       → common.ModelBackend (LLM inference)
//	                       → strategy_gpt.RAGEngine (vector retrieval)
//	                       → strategy_gpt.PromptManager (prompt assembly)
type StrategyFTOReportService struct {
	generator strategy.ReportGenerator
	cache     Cache

	// In-memory report store (no Kafka/Redis dependency).
	mu      sync.RWMutex
	reports map[string]*reportEntry
}

// NewStrategyFTOReportService creates an FTOReportService powered by StrategyGPT.
func NewStrategyFTOReportService(
	generator strategy.ReportGenerator,
	cache Cache,
) FTOReportService {
	if generator == nil {
		panic("StrategyGPT ReportGenerator is required")
	}
	return &StrategyFTOReportService{
		generator: generator,
		cache:     cache,
		reports:   make(map[string]*reportEntry),
	}
}

// Generate initiates FTO report generation via StrategyGPT.
func (s *StrategyFTOReportService) Generate(ctx context.Context, req *FTOReportRequest) (*FTOReportResponse, error) {
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	reportID := uuid.New().String()
	startTime := time.Now()

	s.mu.Lock()
	s.reports[reportID] = &reportEntry{
		Status:      StatusProcessing,
		ProgressPct: 0,
		CreatedAt:   startTime,
		Title:       s.buildTitle(req),
	}
	s.mu.Unlock()

	// Fire-and-forget async generation
	go s.generateAsync(context.Background(), reportID, req)

	return &FTOReportResponse{
		ReportID:          reportID,
		Status:            StatusProcessing,
		EstimatedDuration: s.estimateDuration(req),
		CreatedAt:         startTime,
	}, nil
}

// GetStatus returns the current generation status.
func (s *StrategyFTOReportService) GetStatus(ctx context.Context, reportID string) (*ReportStatusInfo, error) {
	s.mu.RLock()
	entry, ok := s.reports[reportID]
	s.mu.RUnlock()

	if !ok {
		return nil, errors.NewNotFound("report", reportID)
	}

	return &ReportStatusInfo{
		ReportID:    reportID,
		Status:      entry.Status,
		ProgressPct: entry.ProgressPct,
		Message:     entry.Message,
	}, nil
}

// GetReport returns the generated report file as an io.ReadCloser.
func (s *StrategyFTOReportService) GetReport(ctx context.Context, reportID string, format ReportFormat) (io.ReadCloser, error) {
	s.mu.RLock()
	entry, ok := s.reports[reportID]
	s.mu.RUnlock()

	if !ok {
		return nil, errors.NewNotFound("report", reportID)
	}
	if entry.Status != StatusCompleted {
		return nil, errors.Conflict(fmt.Sprintf("report not ready, current status: %s", entry.Status))
	}
	if entry.Content == nil {
		return nil, errors.NewNotFound("report content", reportID)
	}

	return io.NopCloser(bytes.NewReader(entry.Content)), nil
}

// ListReports returns a paginated list of FTO reports from in-memory store.
func (s *StrategyFTOReportService) ListReports(ctx context.Context, filter *FTOReportFilter, page *pkgcommon.Pagination) (*pkgcommon.PaginatedResult[FTOReportSummary], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []FTOReportSummary
	for id, entry := range s.reports {
		// Apply status filter
		if filter != nil && len(filter.Status) > 0 {
			match := false
			for _, st := range filter.Status {
				if entry.Status == st {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		// Apply date filter
		if filter != nil && filter.DateFrom != nil && entry.CreatedAt.Before(*filter.DateFrom) {
			continue
		}
		if filter != nil && filter.DateTo != nil && entry.CreatedAt.After(*filter.DateTo) {
			continue
		}
		all = append(all, FTOReportSummary{
			ReportID: id,
			Title:    entry.Title,
			Status:   entry.Status,
		})
	}

	total := int(len(all))
	pageNum := 1
	pageSize := 20
	if page != nil {
		if page.Page > 0 {
			pageNum = page.Page
		}
		if page.PageSize > 0 && page.PageSize <= 100 {
			pageSize = page.PageSize
		}
	}
	start := (pageNum - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}

	var items []FTOReportSummary
	if start < len(all) {
		items = all[start:end]
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}

	return &pkgcommon.PaginatedResult[FTOReportSummary]{
		Items: items,
		Pagination: pkgcommon.PaginationResult{
			Page:       pageNum,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}, nil
}

// DeleteReport removes a report from the in-memory store.
func (s *StrategyFTOReportService) DeleteReport(ctx context.Context, reportID string) error {
	s.mu.Lock()
	delete(s.reports, reportID)
	s.mu.Unlock()
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *StrategyFTOReportService) validateRequest(req *FTOReportRequest) error {
	if len(req.TargetMolecules) == 0 {
		return errors.NewValidation("target_molecules cannot be empty")
	}
	if len(req.Jurisdictions) == 0 {
		return errors.NewValidation("jurisdictions cannot be empty")
	}
	validDepths := map[AnalysisDepth]bool{DepthQuick: true, DepthStandard: true, DepthComprehensive: true}
	if !validDepths[req.AnalysisDepth] {
		return errors.NewValidation("invalid analysis_depth")
	}
	return nil
}

func (s *StrategyFTOReportService) buildTitle(req *FTOReportRequest) string {
	if req.TargetProduct != "" {
		return fmt.Sprintf("FTO Report: %s", req.TargetProduct)
	}
	if len(req.TargetMolecules) > 0 && req.TargetMolecules[0].Name != "" {
		return fmt.Sprintf("FTO Report: %s", req.TargetMolecules[0].Name)
	}
	return fmt.Sprintf("FTO Report - %s", time.Now().Format("2006-01-02"))
}

func (s *StrategyFTOReportService) estimateDuration(req *FTOReportRequest) time.Duration {
	complexity := len(req.TargetMolecules) * len(req.Jurisdictions)
	switch {
	case complexity <= 1:
		return 30 * time.Second
	case complexity <= 5:
		return 2 * time.Minute
	default:
		return 5 * time.Minute
	}
}

// generateAsync runs the StrategyGPT report generation in a goroutine.
func (s *StrategyFTOReportService) generateAsync(ctx context.Context, reportID string, req *FTOReportRequest) {
	startTime := time.Now()

	s.updateStatus(reportID, StatusProcessing, 10, "building RAG context")

	// Build StrategyGPT request
	sgReq := s.buildStrategyGPTRequest(req)

	// Generate report via StrategyGPT
	report, err := s.generator.GenerateReport(ctx, sgReq)
	if err != nil {
		s.updateStatus(reportID, StatusFailed, 0, err.Error())
		return
	}

	s.updateStatus(reportID, StatusProcessing, 50, "exporting report")

	// Export to Markdown
	markdownBytes, err := s.generator.ExportReport(report, strategy.ExportMarkdown)
	if err != nil {
		s.updateStatus(reportID, StatusFailed, 0, err.Error())
		return
	}

	s.updateStatusWithContent(reportID, StatusCompleted, 100, markdownBytes,
		"text/markdown", fmt.Sprintf("completed in %dms", time.Since(startTime).Milliseconds()))
}

// buildStrategyGPTRequest maps FTOReportRequest to StrategyGPT ReportRequest.
func (s *StrategyFTOReportService) buildStrategyGPTRequest(req *FTOReportRequest) *strategy.ReportRequest {
	var patentNumbers []string
	for _, mol := range req.TargetMolecules {
		if mol.Format == "patent_id" || mol.Format == "patent_number" {
			patentNumbers = append(patentNumbers, mol.Value)
		}
	}

	return &strategy.ReportRequest{
		Task: strategy.TaskFTO,
		Params: &strategy.ReportGenerationParams{
			PatentNumbers: patentNumbers,
			ProductDesc:   req.TargetProduct,
			Jurisdiction:  s.joinJurisdictions(req.Jurisdictions),
			CustomFields: map[string]interface{}{
				"analysis_depth":        string(req.AnalysisDepth),
				"include_design_around": req.IncludeDesignAround,
				"include_claim_chart":   req.IncludeClaimChart,
				"molecule_count":        len(req.TargetMolecules),
				"competitor_filter":     req.CompetitorFilter,
			},
		},
		OutputFormat:  strategy.OutputStructured,
		ExportFormats: []strategy.ExportFormat{strategy.ExportMarkdown},
		QualityCheck:  true,
		RequestID:     uuid.New().String(),
	}
}

func (s *StrategyFTOReportService) joinJurisdictions(jurisdictions []string) string {
	if len(jurisdictions) == 0 {
		return ""
	}
	return fmt.Sprintf("%v", jurisdictions)
}

func (s *StrategyFTOReportService) updateStatus(reportID string, status ReportStatus, progress int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.reports[reportID]
	if entry != nil {
		entry.Status = status
		entry.ProgressPct = progress
		entry.Message = message
	}
}

func (s *StrategyFTOReportService) updateStatusWithContent(reportID string, status ReportStatus, progress int, content []byte, contentType, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.reports[reportID]
	if entry != nil {
		entry.Status = status
		entry.ProgressPct = progress
		entry.Content = content
		entry.ContentType = contentType
		entry.Message = message
	}
}

// ---------------------------------------------------------------------------
// StrategyGPTServiceAdapter — implements reporting.StrategyGPTService
// ---------------------------------------------------------------------------

// StrategyGPTServiceAdapter bridges strategy_gpt.ReportGenerator into
// the reporting.StrategyGPTService interface for use by PortfolioReportService.
type StrategyGPTServiceAdapter struct {
	generator strategy.ReportGenerator
}

// NewStrategyGPTServiceAdapter creates a StrategyGPTService from a ReportGenerator.
func NewStrategyGPTServiceAdapter(generator strategy.ReportGenerator) StrategyGPTService {
	return &StrategyGPTServiceAdapter{generator: generator}
}

// GenerateSectionInsight generates an AI-powered insight for a report section.
func (a *StrategyGPTServiceAdapter) GenerateSectionInsight(ctx context.Context, section ReportSection, data interface{}, lang ReportLanguage) (string, error) {
	req := &strategy.ReportRequest{
		Task: strategy.TaskPortfolioStrategy,
		Params: &strategy.ReportGenerationParams{
			CustomFields: map[string]interface{}{
				"section":   string(section),
				"data":      data,
				"lang":      string(lang),
			},
		},
		OutputFormat: strategy.OutputStructured,
		QualityCheck: false,
	}

	report, err := a.generator.GenerateReport(ctx, req)
	if err != nil {
		return "", err
	}

	if report.Content != nil {
		return report.Content.ExecutiveSummary, nil
	}
	return "", fmt.Errorf("empty report generated")
}

// ---------------------------------------------------------------------------
// CommonLoggerAdapter — adapts application-layer Logger to common.Logger
// for StrategyGPT ReportGenerator consumption.
// ---------------------------------------------------------------------------

// CommonLoggerAdapter implements common.Logger, dropping the ctx parameter
// that the application-layer reporting.Logger requires.
type CommonLoggerAdapter struct {
	wrapped Logger // reporting.Logger with Info(ctx, msg, kvs...)
}

// NewCommonLoggerAdapter creates a common.Logger from a reporting.Logger.
func NewCommonLoggerAdapter(wrapped Logger) *CommonLoggerAdapter {
	return &CommonLoggerAdapter{wrapped: wrapped}
}

func (a *CommonLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	if a.wrapped != nil {
		a.wrapped.Info(context.Background(), msg, keysAndValues...)
	}
}
func (a *CommonLoggerAdapter) Warn(msg string, keysAndValues ...interface{}) {
	if a.wrapped != nil {
		a.wrapped.Warn(context.Background(), msg, keysAndValues...)
	}
}
func (a *CommonLoggerAdapter) Debug(msg string, keysAndValues ...interface{}) {
	if a.wrapped != nil {
		// reporting.Logger doesn't have Debug; use Info for now
		a.wrapped.Info(context.Background(), msg, keysAndValues...)
	}
}
func (a *CommonLoggerAdapter) Error(msg string, keysAndValues ...interface{}) {
	if a.wrapped != nil {
		a.wrapped.Error(context.Background(), msg, keysAndValues...)
	}
}

// ---------------------------------------------------------------------------
// Compile-time interface checks
// ---------------------------------------------------------------------------

var (
	_ FTOReportService = (*StrategyFTOReportService)(nil)
	_ StrategyGPTService = (*StrategyGPTServiceAdapter)(nil)
	_ common.Logger       = (*CommonLoggerAdapter)(nil)
)

//Personal.AI order the ending
