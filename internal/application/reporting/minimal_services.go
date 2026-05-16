// minimal_services.go — minimal implementations of InfringementReportService,
// PortfolioReportService, and TemplateService for the ReportHandler wiring.
package reporting

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ============================================================================
// MinimalInfringementReportService
// ============================================================================

// MinimalInfringementReportService provides a minimal InfringementReportService
// implementation using in-memory storage. It returns placeholder reports.
type MinimalInfringementReportService struct {
	mu      sync.RWMutex
	reports map[string]*infringementEntry
}

type infringementEntry struct {
	Status      ReportStatus
	ProgressPct int
	Message     string
	Content     []byte
	Summary     InfringementReportSummary
	CreatedAt   time.Time
}

// NewMinimalInfringementReportService creates a minimal InfringementReportService.
func NewMinimalInfringementReportService() InfringementReportService {
	return &MinimalInfringementReportService{
		reports: make(map[string]*infringementEntry),
	}
}

func (s *MinimalInfringementReportService) Generate(ctx context.Context, req *InfringementReportRequest) (*InfringementReportResponse, error) {
	if len(req.OwnedPatentNumbers) == 0 {
		return nil, errors.NewValidation("owned_patent_numbers cannot be empty")
	}

	reportID := uuid.New().String()
	now := time.Now()

	s.mu.Lock()
	s.reports[reportID] = &infringementEntry{
		Status:      StatusCompleted,
		ProgressPct: 100,
		Content:     []byte("# Infringement Report\n\nPlaceholder report."),
		Summary: InfringementReportSummary{
			ReportID: reportID,
			Title:    fmt.Sprintf("Infringement Report - %s", now.Format("2006-01-02")),
			Status:   StatusCompleted,
		},
		CreatedAt: now,
	}
	s.mu.Unlock()

	return &InfringementReportResponse{
		ReportID:          reportID,
		Status:            StatusCompleted,
		EstimatedDuration: 0,
		CreatedAt:         now,
	}, nil
}

func (s *MinimalInfringementReportService) GetStatus(ctx context.Context, reportID string) (*ReportStatusInfo, error) {
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

func (s *MinimalInfringementReportService) GetReport(ctx context.Context, reportID string, format ReportFormat) (io.ReadCloser, error) {
	s.mu.RLock()
	entry, ok := s.reports[reportID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.NewNotFound("report", reportID)
	}
	if entry.Content == nil {
		return nil, errors.NewNotFound("report content", reportID)
	}
	return io.NopCloser(bytes.NewReader(entry.Content)), nil
}

func (s *MinimalInfringementReportService) ListReports(ctx context.Context, filter *InfringementReportFilter, page *common.Pagination) (*common.PaginatedResult[InfringementReportSummary], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []InfringementReportSummary
	for _, entry := range s.reports {
		all = append(all, entry.Summary)
	}

	total := len(all)
	pageNum, pageSize := paginationParams(page)
	start, end := sliceRange(pageNum, pageSize, total)

	var items []InfringementReportSummary
	if start < total {
		items = all[start:end]
	}

	return &common.PaginatedResult[InfringementReportSummary]{
		Items: items,
		Pagination: common.PaginationResult{
			Page:       pageNum,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: paginationTotalPages(total, pageSize),
		},
	}, nil
}

func (s *MinimalInfringementReportService) DeleteReport(ctx context.Context, reportID string) error {
	s.mu.Lock()
	delete(s.reports, reportID)
	s.mu.Unlock()
	return nil
}

// ============================================================================
// MinimalPortfolioReportService
// ============================================================================

// MinimalPortfolioReportService provides a minimal PortfolioReportService
// implementation using in-memory storage.
type MinimalPortfolioReportService struct {
	mu      sync.RWMutex
	reports map[string]*portfolioEntry
}

type portfolioEntry struct {
	ReportID  string
	Status    ReportStatus
	Content   []byte
	CreatedAt time.Time
}

// NewMinimalPortfolioReportService creates a minimal PortfolioReportService.
func NewMinimalPortfolioReportService() PortfolioReportService {
	return &MinimalPortfolioReportService{
		reports: make(map[string]*portfolioEntry),
	}
}

func (s *MinimalPortfolioReportService) GenerateFullReport(ctx context.Context, req *PortfolioReportRequest) (*PortfolioReportResult, error) {
	reportID := uuid.New().String()
	s.mu.Lock()
	s.reports[reportID] = &portfolioEntry{
		ReportID:  reportID,
		Status:    StatusCompleted,
		CreatedAt: time.Now(),
	}
	s.mu.Unlock()
	return &PortfolioReportResult{
		ReportID: reportID,
		Status:   StatusCompleted,
	}, nil
}

func (s *MinimalPortfolioReportService) GenerateSummaryReport(ctx context.Context, req *PortfolioSummaryRequest) (*PortfolioReportResult, error) {
	return &PortfolioReportResult{}, nil
}

func (s *MinimalPortfolioReportService) GenerateGapReport(ctx context.Context, req *GapReportRequest) (*PortfolioReportResult, error) {
	return &PortfolioReportResult{}, nil
}

func (s *MinimalPortfolioReportService) GenerateCompetitiveReport(ctx context.Context, req *CompetitiveReportRequest) (*PortfolioReportResult, error) {
	return &PortfolioReportResult{}, nil
}

func (s *MinimalPortfolioReportService) GetReportStatus(ctx context.Context, reportID string) (*ReportStatusInfo, error) {
	s.mu.RLock()
	entry, ok := s.reports[reportID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.NewNotFound("report", reportID)
	}
	return &ReportStatusInfo{
		ReportID:    reportID,
		Status:      entry.Status,
		ProgressPct: 100,
	}, nil
}

func (s *MinimalPortfolioReportService) ListReports(ctx context.Context, portfolioID string, opts *ListReportOptions) (*common.PaginatedResult[ReportMeta], error) {
	return &common.PaginatedResult[ReportMeta]{
		Items:      []ReportMeta{},
		Pagination: common.PaginationResult{Page: 1, PageSize: 20, Total: 0, TotalPages: 0},
	}, nil
}

func (s *MinimalPortfolioReportService) ExportReport(ctx context.Context, reportID string, format ExportFormat) ([]byte, error) {
	s.mu.RLock()
	entry, ok := s.reports[reportID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.NewNotFound("report", reportID)
	}
	return entry.Content, nil
}

// ============================================================================
// MinimalTemplateService
// ============================================================================

// MinimalTemplateService provides an in-memory TemplateEngine implementation.
type MinimalTemplateService struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewMinimalTemplateService creates an in-memory TemplateEngine.
func NewMinimalTemplateService() TemplateEngine {
	return &MinimalTemplateService{
		templates: make(map[string]*Template),
	}
}

func (s *MinimalTemplateService) Render(ctx context.Context, req *RenderRequest) (*RenderResult, error) {
	return &RenderResult{}, nil
}

func (s *MinimalTemplateService) RenderToBytes(ctx context.Context, req *RenderRequest) ([]byte, error) {
	return nil, nil
}

func (s *MinimalTemplateService) ListTemplates(ctx context.Context, opts *ListTemplateOptions) (*common.PaginatedResult[TemplateMeta], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []TemplateMeta
	for id, tmpl := range s.templates {
		all = append(all, TemplateMeta{
			ID:   id,
			Name: tmpl.Name,
		})
	}

	total := len(all)
	pageNum, pageSize := 1, 20
	if opts != nil {
		if opts.Pagination.Page > 0 {
			pageNum = opts.Pagination.Page
		}
		if opts.Pagination.PageSize > 0 {
			pageSize = opts.Pagination.PageSize
		}
	}
	start, end := sliceRange(pageNum, pageSize, total)
	var items []TemplateMeta
	if start < total {
		items = all[start:end]
	}

	return &common.PaginatedResult[TemplateMeta]{
		Items: items,
		Pagination: common.PaginationResult{
			Page:       pageNum,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: paginationTotalPages(total, pageSize),
		},
	}, nil
}

func (s *MinimalTemplateService) GetTemplate(ctx context.Context, templateID string) (*Template, error) {
	s.mu.RLock()
	tmpl, ok := s.templates[templateID]
	s.mu.RUnlock()
	if !ok {
		return nil, errors.NewNotFound("template", templateID)
	}
	return tmpl, nil
}

func (s *MinimalTemplateService) RegisterTemplate(ctx context.Context, tmpl *Template) error {
	s.mu.Lock()
	s.templates[tmpl.ID] = tmpl
	s.mu.Unlock()
	return nil
}

func (s *MinimalTemplateService) UpdateTemplate(ctx context.Context, tmpl *Template) error {
	s.mu.Lock()
	s.templates[tmpl.ID] = tmpl
	s.mu.Unlock()
	return nil
}

func (s *MinimalTemplateService) DeleteTemplate(ctx context.Context, templateID string) error {
	s.mu.Lock()
	delete(s.templates, templateID)
	s.mu.Unlock()
	return nil
}

func (s *MinimalTemplateService) ValidateTemplate(ctx context.Context, tmpl *Template) (*ValidationResult, error) {
	return &ValidationResult{Valid: true}, nil
}

func (s *MinimalTemplateService) PreviewTemplate(ctx context.Context, templateID string, sampleData map[string]interface{}) (*RenderResult, error) {
	return &RenderResult{}, nil
}

// ============================================================================
// Shared helpers
// ============================================================================

func paginationParams(page *common.Pagination) (pageNum, pageSize int) {
	pageNum, pageSize = 1, 20
	if page != nil {
		if page.Page > 0 {
			pageNum = page.Page
		}
		if page.PageSize > 0 && page.PageSize <= 100 {
			pageSize = page.PageSize
		}
	}
	return
}

func sliceRange(pageNum, pageSize, total int) (start, end int) {
	start = (pageNum - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end = start + pageSize
	if end > total {
		end = total
	}
	return
}

func paginationTotalPages(total, pageSize int) int {
	if total == 0 || pageSize == 0 {
		return 0
	}
	tp := (total + pageSize - 1) / pageSize
	if tp < 1 {
		tp = 1
	}
	return tp
}

// Compile-time interface checks
var (
	_ InfringementReportService = (*MinimalInfringementReportService)(nil)
	_ PortfolioReportService     = (*MinimalPortfolioReportService)(nil)
	_ TemplateEngine             = (*MinimalTemplateService)(nil)
)

//Personal.AI order the ending
