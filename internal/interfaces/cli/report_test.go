package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MockFTOReportService is a mock implementation of FTOReportService
type MockFTOReportService struct {
	mock.Mock
}

func (m *MockFTOReportService) Generate(ctx context.Context, req *reporting.FTOReportRequest) (*reporting.FTOReportResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.FTOReportResponse), args.Error(1)
}

func (m *MockFTOReportService) GetStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportStatusInfo), args.Error(1)
}

func (m *MockFTOReportService) GetReport(ctx context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
	args := m.Called(ctx, reportID, format)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockFTOReportService) ListReports(ctx context.Context, filter *reporting.FTOReportFilter, page *common.Pagination) (*common.PaginatedResult[reporting.FTOReportSummary], error) {
	args := m.Called(ctx, filter, page)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PaginatedResult[reporting.FTOReportSummary]), args.Error(1)
}

func (m *MockFTOReportService) DeleteReport(ctx context.Context, reportID string) error {
	args := m.Called(ctx, reportID)
	return args.Error(0)
}

// MockInfringementReportService is a mock implementation of InfringementReportService
type MockInfringementReportService struct {
	mock.Mock
}

func (m *MockInfringementReportService) Generate(ctx context.Context, req *reporting.InfringementReportRequest) (*reporting.InfringementReportResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.InfringementReportResponse), args.Error(1)
}

func (m *MockInfringementReportService) GetStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportStatusInfo), args.Error(1)
}

func (m *MockInfringementReportService) GetReport(ctx context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
	args := m.Called(ctx, reportID, format)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockInfringementReportService) ListReports(ctx context.Context, filter *reporting.InfringementReportFilter, page *common.Pagination) (*common.PaginatedResult[reporting.InfringementReportSummary], error) {
	args := m.Called(ctx, filter, page)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PaginatedResult[reporting.InfringementReportSummary]), args.Error(1)
}

func (m *MockInfringementReportService) DeleteReport(ctx context.Context, reportID string) error {
	args := m.Called(ctx, reportID)
	return args.Error(0)
}

// MockPortfolioReportService is a mock implementation of PortfolioReportService
type MockPortfolioReportService struct {
	mock.Mock
}

func (m *MockPortfolioReportService) GenerateFullReport(ctx context.Context, req *reporting.PortfolioReportRequest) (*reporting.PortfolioReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.PortfolioReportResult), args.Error(1)
}

func (m *MockPortfolioReportService) GenerateSummaryReport(ctx context.Context, req *reporting.PortfolioSummaryRequest) (*reporting.PortfolioReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.PortfolioReportResult), args.Error(1)
}

func (m *MockPortfolioReportService) GenerateGapReport(ctx context.Context, req *reporting.GapReportRequest) (*reporting.PortfolioReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.PortfolioReportResult), args.Error(1)
}

func (m *MockPortfolioReportService) GenerateCompetitiveReport(ctx context.Context, req *reporting.CompetitiveReportRequest) (*reporting.PortfolioReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.PortfolioReportResult), args.Error(1)
}

func (m *MockPortfolioReportService) GetReportStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportStatusInfo), args.Error(1)
}

func (m *MockPortfolioReportService) ListReports(ctx context.Context, portfolioID string, opts *reporting.ListReportOptions) (*common.PaginatedResult[reporting.ReportMeta], error) {
	args := m.Called(ctx, portfolioID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PaginatedResult[reporting.ReportMeta]), args.Error(1)
}

func (m *MockPortfolioReportService) ExportReport(ctx context.Context, reportID string, format reporting.ExportFormat) ([]byte, error) {
	args := m.Called(ctx, reportID, format)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

// MockTemplateService is a mock implementation of TemplateService (alias for TemplateEngine)
type MockTemplateService struct {
	mock.Mock
}

func (m *MockTemplateService) Render(ctx context.Context, req *reporting.RenderRequest) (*reporting.RenderResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.RenderResult), args.Error(1)
}

func (m *MockTemplateService) RenderToBytes(ctx context.Context, req *reporting.RenderRequest) ([]byte, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockTemplateService) ListTemplates(ctx context.Context, opts *reporting.ListTemplateOptions) (*common.PaginatedResult[reporting.TemplateMeta], error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PaginatedResult[reporting.TemplateMeta]), args.Error(1)
}

func (m *MockTemplateService) GetTemplate(ctx context.Context, templateID string) (*reporting.Template, error) {
	args := m.Called(ctx, templateID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.Template), args.Error(1)
}

func (m *MockTemplateService) RegisterTemplate(ctx context.Context, tmpl *reporting.Template) error {
	args := m.Called(ctx, tmpl)
	return args.Error(0)
}

func (m *MockTemplateService) UpdateTemplate(ctx context.Context, tmpl *reporting.Template) error {
	args := m.Called(ctx, tmpl)
	return args.Error(0)
}

func (m *MockTemplateService) DeleteTemplate(ctx context.Context, templateID string) error {
	args := m.Called(ctx, templateID)
	return args.Error(0)
}

func (m *MockTemplateService) ValidateTemplate(ctx context.Context, tmpl *reporting.Template) (*reporting.ValidationResult, error) {
	args := m.Called(ctx, tmpl)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ValidationResult), args.Error(1)
}

func (m *MockTemplateService) PreviewTemplate(ctx context.Context, templateID string, sampleData map[string]interface{}) (*reporting.RenderResult, error) {
	args := m.Called(ctx, templateID, sampleData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.RenderResult), args.Error(1)
}

// suppress unused import warning
var _ = strings.Contains

func TestResolveOutputPath_UniqueTimestamp(t *testing.T) {
	tmpDir := t.TempDir()

	path1 := resolveOutputPath(tmpDir, "fto", "pdf")
	time.Sleep(time.Second) // Ensure different timestamp
	path2 := resolveOutputPath(tmpDir, "fto", "pdf")

	assert.NotEqual(t, path1, path2)
	assert.Contains(t, path1, "fto_report_")
	assert.Contains(t, path1, ".pdf")
}

func TestResolveOutputPath_FileNameConflict(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	firstPath := resolveOutputPath(tmpDir, "fto", "pdf")
	file, err := os.Create(firstPath)
	require.NoError(t, err)
	file.Close()

	// Next call should append sequence number
	secondPath := resolveOutputPath(tmpDir, "fto", "pdf")
	assert.NotEqual(t, firstPath, secondPath)
	assert.Contains(t, secondPath, "_1.pdf")
}

func TestEnsureOutputDir_CreateNew(t *testing.T) {
	tmpDir := t.TempDir()
	newDir := filepath.Join(tmpDir, "reports", "2024")

	err := ensureOutputDir(newDir)
	require.NoError(t, err)

	// Verify directory exists
	info, err := os.Stat(newDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestEnsureOutputDir_ExistingDir(t *testing.T) {
	tmpDir := t.TempDir()

	err := ensureOutputDir(tmpDir)
	require.NoError(t, err)
}

func TestWriteReportToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_report.pdf")

	content := []byte("Test report content")
	err := writeReportToFile(content, outputPath)
	require.NoError(t, err)

	// Verify file content
	readContent, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, content, readContent)
}

func TestPrintReportSummary(t *testing.T) {
	result := &reporting.ReportResult{
		ReportID:    "report-001",
		ReportType:  "fto",
		Title:       "FTO Analysis Report",
		Format:      "pdf",
		GeneratedAt: time.Now(),
		Size:        1024,
		Status:      "completed",
	}

	// This function prints to stdout, we're just testing it doesn't panic
	printReportSummary(result)
}

func TestColorizeStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"completed", "\033[32mcompleted\033[0m"},
		{"in_progress", "\033[33min_progress\033[0m"},
		{"failed", "\033[31mfailed\033[0m"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := colorizeStatus(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30 seconds"},
		{90 * time.Second, "2 minutes"},
		{5 * time.Minute, "5 minutes"},
		{90 * time.Minute, "1.5 hours"},
		{3 * time.Hour, "3.0 hours"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"Short string", "Hello", 10, "Hello"},
		{"Exact length", "Hello", 5, "Hello"},
		{"Truncate", "Hello World", 8, "Hello..."},
		{"Long truncate", "This is a very long description", 20, "This is a very lo..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReportGenerateCmd_InvalidType(t *testing.T) {
	mockFTO := new(MockFTOReportService)
	mockInfringement := new(MockInfringementReportService)
	mockPortfolio := new(MockPortfolioReportService)
	mockLogger := new(MockLogger)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportType = "invalid"
	reportTarget = "CN115123456"
	reportFormat = "pdf"
	reportLanguage = "zh"
	reportOutputDir = t.TempDir()

	err := runReportGenerate(ctx, mockFTO, mockInfringement, mockPortfolio, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid report type")
}

func TestReportGenerateCmd_InvalidFormat(t *testing.T) {
	mockFTO := new(MockFTOReportService)
	mockInfringement := new(MockInfringementReportService)
	mockPortfolio := new(MockPortfolioReportService)
	mockLogger := new(MockLogger)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportType = "fto"
	reportTarget = "CN115123456"
	reportFormat = "html"
	reportLanguage = "zh"
	reportOutputDir = t.TempDir()

	err := runReportGenerate(ctx, mockFTO, mockInfringement, mockPortfolio, mockLogger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid output format")
}

func TestReportGenerateCmd_FTOReport_PDF(t *testing.T) {
	mockFTO := new(MockFTOReportService)
	mockInfringement := new(MockInfringementReportService)
	mockPortfolio := new(MockPortfolioReportService)
	mockLogger := new(MockLogger)

	// FTOReportResponse matches the actual interface
	ftoResponse := &reporting.FTOReportResponse{
		ReportID:          "report-12345",
		Status:            reporting.StatusProcessing,
		EstimatedDuration: 5 * time.Minute,
		CreatedAt:         time.Now(),
	}

	mockFTO.On("Generate", mock.Anything, mock.MatchedBy(func(req *reporting.FTOReportRequest) bool {
		return len(req.TargetMolecules) > 0
	})).Return(ftoResponse, nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportType = "fto"
	reportTarget = "CN115123456"
	reportFormat = "pdf"
	reportLanguage = "zh"
	reportOutputDir = t.TempDir()
	reportIncludeAppendix = true

	err := runReportGenerate(ctx, mockFTO, mockInfringement, mockPortfolio, mockLogger)
	assert.NoError(t, err)

	mockFTO.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestReportGenerateCmd_AsyncMode(t *testing.T) {
	mockFTO := new(MockFTOReportService)
	mockInfringement := new(MockInfringementReportService)
	mockPortfolio := new(MockPortfolioReportService)
	mockLogger := new(MockLogger)

	now := time.Now()
	// FTOReportResponse with estimated duration for async processing
	ftoResponse := &reporting.FTOReportResponse{
		ReportID:          "job-12345",
		Status:            reporting.StatusProcessing,
		EstimatedDuration: 10 * time.Minute,
		CreatedAt:         now,
	}

	mockFTO.On("Generate", mock.Anything, mock.Anything).Return(ftoResponse, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportType = "fto"
	reportTarget = "CN115123456"
	reportFormat = "pdf"
	reportLanguage = "zh"
	reportOutputDir = t.TempDir()

	err := runReportGenerate(ctx, mockFTO, mockInfringement, mockPortfolio, mockLogger)
	assert.NoError(t, err)

	mockFTO.AssertExpectations(t)
}

func TestReportListTemplatesCmd(t *testing.T) {
	mockTemplate := new(MockTemplateService)
	mockLogger := new(MockLogger)

	// TemplateMeta is what ListTemplates returns in PaginatedResult
	templateMetas := []reporting.TemplateMeta{
		{
			ID:      "tmpl-001",
			Name:    "Standard FTO Report",
			Type:    "fto",
			Version: "1.0",
			Format:  reporting.HTMLTemplate,
		},
		{
			ID:      "tmpl-002",
			Name:    "Detailed Infringement Report",
			Type:    "infringement",
			Version: "2.1",
			Format:  reporting.DOCXTemplate,
		},
	}

	paginatedResult := &common.PaginatedResult[reporting.TemplateMeta]{
		Items: templateMetas,
		Pagination: common.PaginationResult{
			Page:       1,
			PageSize:   100,
			Total:      2,
			TotalPages: 1,
		},
	}

	mockTemplate.On("ListTemplates", mock.Anything, mock.MatchedBy(func(opts *reporting.ListTemplateOptions) bool {
		return opts.Type == nil || *opts.Type == ""
	})).Return(paginatedResult, nil)

	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportType = ""

	err := runReportListTemplates(ctx, mockTemplate, mockLogger)
	assert.NoError(t, err)

	mockTemplate.AssertExpectations(t)
}

func TestReportStatusCmd_Completed(t *testing.T) {
	mockFTO := new(MockFTOReportService)
	mockLogger := new(MockLogger)

	// ReportStatusInfo matches the actual interface
	statusInfo := &reporting.ReportStatusInfo{
		ReportID:    "job-12345",
		Status:      reporting.StatusCompleted,
		ProgressPct: 100,
		Message:     "Report generation completed successfully",
	}

	mockFTO.On("GetStatus", mock.Anything, "job-12345").Return(statusInfo, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportJobID = "job-12345"

	err := runReportStatus(ctx, mockFTO, mockLogger)
	assert.NoError(t, err)

	mockFTO.AssertExpectations(t)
}

func TestReportStatusCmd_Failed(t *testing.T) {
	mockFTO := new(MockFTOReportService)
	mockLogger := new(MockLogger)

	statusInfo := &reporting.ReportStatusInfo{
		ReportID:    "job-12345",
		Status:      reporting.StatusFailed,
		ProgressPct: 45,
		Message:     "Database connection timeout",
	}

	mockFTO.On("GetStatus", mock.Anything, "job-12345").Return(statusInfo, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportJobID = "job-12345"

	err := runReportStatus(ctx, mockFTO, mockLogger)
	assert.NoError(t, err)

	mockFTO.AssertExpectations(t)
}

//Personal.AI order the ending
