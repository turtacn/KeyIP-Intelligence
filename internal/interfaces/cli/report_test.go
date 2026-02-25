package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
)

// MockFTOReportService is a mock implementation of FTOReportService
type MockFTOReportService struct {
	mock.Mock
}

func (m *MockFTOReportService) Generate(ctx context.Context, req *reporting.FTOReportRequest) (*reporting.ReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportResult), args.Error(1)
}

func (m *MockFTOReportService) GetJobStatus(ctx context.Context, jobID string) (*reporting.JobStatus, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.JobStatus), args.Error(1)
}

// MockInfringementReportService is a mock implementation of InfringementReportService
type MockInfringementReportService struct {
	mock.Mock
}

func (m *MockInfringementReportService) Generate(ctx context.Context, req *reporting.InfringementReportRequest) (*reporting.ReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportResult), args.Error(1)
}

// MockPortfolioReportService is a mock implementation of PortfolioReportService
type MockPortfolioReportService struct {
	mock.Mock
}

func (m *MockPortfolioReportService) Generate(ctx context.Context, req *reporting.PortfolioReportRequest) (*reporting.ReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportResult), args.Error(1)
}

func (m *MockPortfolioReportService) GenerateAnnual(ctx context.Context, req *reporting.AnnualReportRequest) (*reporting.ReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportResult), args.Error(1)
}

// MockTemplateService is a mock implementation of TemplateService
type MockTemplateService struct {
	mock.Mock
}

func (m *MockTemplateService) ListTemplates(ctx context.Context, req *reporting.ListTemplatesRequest) ([]*reporting.Template, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*reporting.Template), args.Error(1)
}

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
		PageCount:       25,
		SectionCount:    8,
		DataSourceCount: 12,
		Warnings:        []string{"Missing patent citations", "Incomplete competitor data"},
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

	reportResult := &reporting.ReportResult{
		Content:         []byte("PDF content"),
		PageCount:       10,
		SectionCount:    5,
		DataSourceCount: 3,
		Warnings:        []string{},
		Async:           false,
	}

	mockFTO.On("Generate", mock.Anything, mock.MatchedBy(func(req *reporting.FTOReportRequest) bool {
		return req.Target == "CN115123456" && req.Format == "pdf"
	})).Return(reportResult, nil)

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
	reportResult := &reporting.ReportResult{
		Async:               true,
		JobID:               "job-12345",
		EstimatedPages:      150,
		EstimatedCompletion: now.Add(10 * time.Minute),
	}

	mockFTO.On("Generate", mock.Anything, mock.Anything).Return(reportResult, nil)
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

	templates := []*reporting.Template{
		{
			ID:          "tmpl-001",
			Name:        "Standard FTO Report",
			Type:        "fto",
			Language:    "zh",
			Version:     "1.0",
			Description: "Standard FTO analysis template",
		},
		{
			ID:          "tmpl-002",
			Name:        "Detailed Infringement Report",
			Type:        "infringement",
			Language:    "en",
			Version:     "2.1",
			Description: "Comprehensive infringement analysis",
		},
	}

	mockTemplate.On("ListTemplates", mock.Anything, mock.MatchedBy(func(req *reporting.ListTemplatesRequest) bool {
		return req.Type == ""
	})).Return(templates, nil)

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

	now := time.Now()
	jobStatus := &reporting.JobStatus{
		JobID:       "job-12345",
		Status:      "completed",
		Progress:    100,
		OutputPath:  "/reports/fto_report.pdf",
		PageCount:   25,
		CompletedAt: now,
	}

	mockFTO.On("GetJobStatus", mock.Anything, "job-12345").Return(jobStatus, nil)
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

	jobStatus := &reporting.JobStatus{
		JobID:        "job-12345",
		Status:       "failed",
		Progress:     45,
		ErrorMessage: "Database connection timeout",
	}

	mockFTO.On("GetJobStatus", mock.Anything, "job-12345").Return(jobStatus, nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	ctx := context.Background()
	reportJobID = "job-12345"

	err := runReportStatus(ctx, mockFTO, mockLogger)
	assert.NoError(t, err)

	mockFTO.AssertExpectations(t)
}

//Personal.AI order the ending
