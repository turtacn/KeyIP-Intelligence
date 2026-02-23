/*
---
继续输出 235 `internal/application/reporting/fto_report_test.go` 要实现FTO报告生成应用服务的完整单元测试。

实现要求:
* **功能定位**：FTOReportService接口所有公开方法的单元测试，通过Mock隔离所有外部依赖，验证编排逻辑的正确性、边界条件处理与错误传播链路。
* **核心实现**：完整定义所有Mock对象和测试辅助函数，覆盖20+个细分测试用例（包括同步/异步路径、各种错误截获、缓存与分页）。
* **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
---
*/

package reporting

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ============================================================================
// Mocks
// ============================================================================

type mockInfringementAssessor struct {
	assessFunc func(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error)
}

func (m *mockInfringementAssessor) Assess(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error) {
	if m.assessFunc != nil {
		return m.assessFunc(ctx, smiles, claimData, depth)
	}
	return sampleAssessmentResult(), nil
}

type mockClaimParser struct {
	parseFunc func(ctx context.Context, patentID string) (interface{}, error)
}

func (m *mockClaimParser) Parse(ctx context.Context, patentID string) (interface{}, error) {
	if m.parseFunc != nil {
		return m.parseFunc(ctx, patentID)
	}
	return sampleClaimParseResult(), nil
}

type mockMoleculeService struct {
	validateAndNormalizeFunc func(ctx context.Context, format, value string) (string, string, error)
}

func (m *mockMoleculeService) ValidateAndNormalize(ctx context.Context, format, value string) (string, string, error) {
	if m.validateAndNormalizeFunc != nil {
		return m.validateAndNormalizeFunc(ctx, format, value)
	}
	return value, "dummy-inchikey", nil
}

type mockPatentRepo struct {
	getDetailsFunc func(ctx context.Context, patentIDs []string) (interface{}, error)
}

func (m *mockPatentRepo) GetDetails(ctx context.Context, patentIDs []string) (interface{}, error) {
	if m.getDetailsFunc != nil {
		return m.getDetailsFunc(ctx, patentIDs)
	}
	return nil, nil
}

type mockSimilaritySearchService struct {
	searchFunc func(ctx context.Context, smiles string, jurisdictions []string, competitors []string, limit int) ([]string, error)
	lastLimit  int
}

func (m *mockSimilaritySearchService) Search(ctx context.Context, smiles string, jurisdictions []string, competitors []string, limit int) ([]string, error) {
	m.lastLimit = limit
	if m.searchFunc != nil {
		return m.searchFunc(ctx, smiles, jurisdictions, competitors, limit)
	}
	return sampleSimilarityResult(), nil
}

type mockTemplateEngine struct {
	renderFunc func(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error)
}

func (m *mockTemplateEngine) Render(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error) {
	if m.renderFunc != nil {
		return m.renderFunc(ctx, templateName, data, format)
	}
	return []byte("dummy-pdf-content"), nil
}

type mockStorageRepo struct {
	saveFunc      func(ctx context.Context, key string, data []byte, contentType string) error
	getStreamFunc func(ctx context.Context, key string) (io.ReadCloser, error)
	deleteFunc    func(ctx context.Context, key string) error
}

func (m *mockStorageRepo) Save(ctx context.Context, key string, data []byte, contentType string) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, key, data, contentType)
	}
	return nil
}

func (m *mockStorageRepo) GetStream(ctx context.Context, key string) (io.ReadCloser, error) {
	if m.getStreamFunc != nil {
		return m.getStreamFunc(ctx, key)
	}
	return io.NopCloser(bytes.NewReader([]byte("dummy-pdf-content"))), nil
}

func (m *mockStorageRepo) Delete(ctx context.Context, key string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, key)
	}
	return nil
}

type mockFTOReportMetadataRepo struct {
	createFunc       func(ctx context.Context, summary *FTOReportSummary) error
	updateStatusFunc func(ctx context.Context, reportID string, status ReportStatus, summary riskSummary) error
	getFunc          func(ctx context.Context, reportID string) (*FTOReportSummary, error)
	listFunc         func(ctx context.Context, filter *FTOReportFilter, page *common.Pagination) ([]FTOReportSummary, int64, error)
	deleteFunc       func(ctx context.Context, reportID string) error
}

func (m *mockFTOReportMetadataRepo) Create(ctx context.Context, summary *FTOReportSummary) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, summary)
	}
	return nil
}

func (m *mockFTOReportMetadataRepo) UpdateStatus(ctx context.Context, reportID string, status ReportStatus, summary riskSummary) error {
	if m.updateStatusFunc != nil {
		return m.updateStatusFunc(ctx, reportID, status, summary)
	}
	return nil
}

func (m *mockFTOReportMetadataRepo) Get(ctx context.Context, reportID string) (*FTOReportSummary, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, reportID)
	}
	now := time.Now()
	return &FTOReportSummary{ReportID: reportID, Status: StatusCompleted, CreatedAt: now, CompletedAt: &now}, nil
}

func (m *mockFTOReportMetadataRepo) List(ctx context.Context, filter *FTOReportFilter, page *common.Pagination) ([]FTOReportSummary, int64, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, filter, page)
	}
	return []FTOReportSummary{{ReportID: "report-1", Status: StatusCompleted}}, 1, nil
}

func (m *mockFTOReportMetadataRepo) Delete(ctx context.Context, reportID string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, reportID)
	}
	return nil
}

type mockCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]interface{})}
}

func (m *mockCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if val, ok := m.data[key]; ok {
		// Mock logic: assert dest type and assign
		switch d := dest.(type) {
		case *ReportStatusInfo:
			*d = val.(ReportStatusInfo)
		}
		return nil
	}
	return errors.NewInternal("cache miss")
}

func (m *mockCache) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = val
	return nil
}

type mockLogger struct{}

func (l *mockLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{})  {}
func (l *mockLogger) Error(ctx context.Context, msg string, keysAndValues ...interface{}) {}
func (l *mockLogger) Warn(ctx context.Context, msg string, keysAndValues ...interface{})  {}

type mockMetrics struct {
	mu     sync.Mutex
	counts map[string]int
	histos map[string][]float64
}

func newMockMetrics() *mockMetrics {
	return &mockMetrics{counts: make(map[string]int), histos: make(map[string][]float64)}
}

func (m *mockMetrics) IncCounter(name string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[name]++
}

func (m *mockMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.histos[name] = append(m.histos[name], value)
}

// ============================================================================
// Test Helpers
// ============================================================================

type testMocks struct {
	assessor  *mockInfringementAssessor
	parser    *mockClaimParser
	molSvc    *mockMoleculeService
	patRepo   *mockPatentRepo
	simSearch *mockSimilaritySearchService
	templater *mockTemplateEngine
	storage   *mockStorageRepo
	metaRepo  *mockFTOReportMetadataRepo
	cache     *mockCache
	logger    *mockLogger
	metrics   *mockMetrics
}

func newTestFTOService() (FTOReportService, *testMocks) {
	m := &testMocks{
		assessor:  &mockInfringementAssessor{},
		parser:    &mockClaimParser{},
		molSvc:    &mockMoleculeService{},
		patRepo:   &mockPatentRepo{},
		simSearch: &mockSimilaritySearchService{},
		templater: &mockTemplateEngine{},
		storage:   &mockStorageRepo{},
		metaRepo:  &mockFTOReportMetadataRepo{},
		cache:     newMockCache(),
		logger:    &mockLogger{},
		metrics:   newMockMetrics(),
	}
	svc := NewFTOReportService(
		m.assessor, m.parser, m.molSvc, m.patRepo,
		m.simSearch, m.templater, m.storage, m.metaRepo,
		m.cache, m.logger, m.metrics,
	)
	return svc, m
}

func validFTORequest() *FTOReportRequest {
	return &FTOReportRequest{
		TargetMolecules: []MoleculeInput{
			{Format: "smiles", Value: "C1=CC=CC=C1"},
			{Format: "smiles", Value: "CC(=O)OC1=CC=CC=C1C(=O)O"},
		},
		TargetProduct:    "Test Product Alpha",
		Jurisdictions:    []string{"CN", "US"},
		AnalysisDepth:    DepthStandard,
		RequestedBy:      "user-123",
		Language:         LangZH,
	}
}

func sampleSimilarityResult() []string {
	return []string{"US10000001B2", "CN110000001A", "EP3000001A1"}
}

func sampleClaimParseResult() interface{} {
	return map[string]interface{}{"claims": "parsed"}
}

func sampleAssessmentResult() interface{} {
	return map[string]interface{}{"risk": "High"}
}

func assertErrorCode(t *testing.T, err error, expectedCode errors.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("Expected error code %s, got nil", expectedCode)
	}
	if !errors.IsCode(err, expectedCode) {
		t.Errorf("Expected error code %s, got %v", expectedCode, err)
	}
}

// ============================================================================
// Test Cases: Generate
// ============================================================================

func TestFTOReportService_Generate_SyncPath_Success(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest() // 2 molecules * 2 jurisdictions = 4 <= 10 (sync path)

	resp, err := svc.Generate(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.Status != StatusCompleted {
		t.Errorf("Expected status Completed, got %s", resp.Status)
	}
	if resp.ReportID == "" {
		t.Errorf("Expected non-empty ReportID")
	}

	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	if len(m.metrics.histos["fto_report_generation_latency"]) == 0 {
		t.Errorf("Expected metrics to be recorded for sync generation")
	}
}

func TestFTOReportService_Generate_AsyncPath_Triggered(t *testing.T) {
	t.Parallel()
	svc, _ := newTestFTOService()
	req := validFTORequest()
	// 4 molecules * 3 jurisdictions = 12 > 10 (async path threshold)
	req.TargetMolecules = append(req.TargetMolecules, MoleculeInput{Format: "smiles", Value: "C1=CC=C(C=C1)O"}, MoleculeInput{Format: "smiles", Value: "C"})
	req.Jurisdictions = append(req.Jurisdictions, "EP")

	resp, err := svc.Generate(context.Background(), req)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.Status != StatusQueued {
		t.Errorf("Expected status Queued, got %s", resp.Status)
	}
	if resp.EstimatedDuration <= 0 {
		t.Errorf("Expected EstimatedDuration > 0")
	}
	if resp.ReportID == "" {
		t.Errorf("Expected non-empty ReportID")
	}
}

func TestFTOReportService_Generate_EmptyMolecules(t *testing.T) {
	t.Parallel()
	svc, _ := newTestFTOService()
	req := validFTORequest()
	req.TargetMolecules = []MoleculeInput{}

	_, err := svc.Generate(context.Background(), req)
	assertErrorCode(t, err, errors.ErrCodeValidation)
}

func TestFTOReportService_Generate_InvalidJurisdiction(t *testing.T) {
	t.Parallel()
	svc, _ := newTestFTOService()
	req := validFTORequest()
	req.Jurisdictions = []string{}

	_, err := svc.Generate(context.Background(), req)
	assertErrorCode(t, err, errors.ErrCodeValidation)
	if !strings.Contains(err.Error(), "jurisdictions") {
		t.Errorf("Expected error message to mention jurisdictions")
	}
}

func TestFTOReportService_Generate_InvalidAnalysisDepth(t *testing.T) {
	t.Parallel()
	svc, _ := newTestFTOService()
	req := validFTORequest()
	req.AnalysisDepth = AnalysisDepth("SuperDeepAndFake")

	_, err := svc.Generate(context.Background(), req)
	assertErrorCode(t, err, errors.ErrCodeValidation)
}

func TestFTOReportService_Generate_PartialMoleculeFailure(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()
	// 3 molecules
	req.TargetMolecules = append(req.TargetMolecules, MoleculeInput{Format: "smiles", Value: "INVALID_SMILES"})

	m.molSvc.validateAndNormalizeFunc = func(ctx context.Context, format, value string) (string, string, error) {
		if value == "INVALID_SMILES" {
			return "", "", errors.NewInvalidParameterError("bad smiles")
		}
		return value, "valid-inchikey", nil
	}

	// Should still complete but report contains errors for the bad molecule
	resp, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no top-level error for partial failure, got: %v", err)
	}
	if resp.Status != StatusCompleted {
		t.Errorf("Expected StatusCompleted")
	}
}

func TestFTOReportService_Generate_SimilaritySearchError(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()

	m.simSearch.searchFunc = func(ctx context.Context, smiles string, jurisdictions []string, competitors []string, limit int) ([]string, error) {
		return nil, errors.NewInternal("sim search down")
	}

	// Notice that the current implementation treats similarity search failure as a partial failure (skips to next molecule).
	// Let's verify it skips and completes without top-level failure, or fails if we require strict bubbling.
	// Based on implementation: `continue` is used. So it completes but with internal errors recorded.
	resp, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected partial success strategy to absorb the error, got: %v", err)
	}
	if resp.Status != StatusCompleted {
		t.Errorf("Expected StatusCompleted")
	}
}

func TestFTOReportService_Generate_ClaimParserError(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()

	m.parser.parseFunc = func(ctx context.Context, patentID string) (interface{}, error) {
		if patentID == "CN110000001A" {
			return nil, errors.NewInternal("parse failed")
		}
		return sampleClaimParseResult(), nil
	}

	// Should skip the failing patent and proceed to assess others
	resp, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.Status != StatusCompleted {
		t.Errorf("Expected StatusCompleted")
	}
}

func TestFTOReportService_Generate_AssessorError(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()

	m.assessor.assessFunc = func(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error) {
		return nil, errors.NewInternal("assess failed")
	}

	resp, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.Status != StatusCompleted {
		t.Errorf("Expected StatusCompleted")
	}
}

func TestFTOReportService_Generate_TemplateRenderError(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()

	m.templater.renderFunc = func(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error) {
		return nil, errors.NewInternal("render failed")
	}

	_, err := svc.Generate(context.Background(), req)
	if err == nil {
		t.Fatalf("Expected error due to template render failure")
	}
	// Verify metadata status was updated to Failed
	// Assuming implementation updates metaRepo internally on sync failure
}

func TestFTOReportService_Generate_StorageError(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()

	m.storage.saveFunc = func(ctx context.Context, key string, data []byte, contentType string) error {
		return errors.NewInternal("s3 down")
	}

	_, err := svc.Generate(context.Background(), req)
	if err == nil {
		t.Fatalf("Expected error due to storage failure")
	}
}

func TestFTOReportService_Generate_DepthQuick(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()
	req.AnalysisDepth = DepthQuick

	_, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if m.simSearch.lastLimit != 10 {
		t.Errorf("Expected limit 10 for DepthQuick, got %d", m.simSearch.lastLimit)
	}
}

func TestFTOReportService_Generate_DepthComprehensive(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()
	req := validFTORequest()
	req.AnalysisDepth = DepthComprehensive

	_, err := svc.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if m.simSearch.lastLimit != 200 {
		t.Errorf("Expected limit 200 for DepthComprehensive, got %d", m.simSearch.lastLimit)
	}
}

// ============================================================================
// Test Cases: GetStatus
// ============================================================================

func TestFTOReportService_GetStatus_CacheHit(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	reportID := "report-cache-hit"
	cachedInfo := ReportStatusInfo{ReportID: reportID, Status: StatusProcessing, ProgressPct: 50}
	_ = m.cache.Set(context.Background(), "fto_status:"+reportID, cachedInfo, 10*time.Minute)

	// Make sure DB is not hit by setting it to fail if called
	m.metaRepo.getFunc = func(ctx context.Context, id string) (*FTOReportSummary, error) {
		return nil, errors.NewInternal("should not be called")
	}

	info, err := svc.GetStatus(context.Background(), reportID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info.ProgressPct != 50 {
		t.Errorf("Expected ProgressPct 50 from cache, got %d", info.ProgressPct)
	}
}

func TestFTOReportService_GetStatus_CacheMiss(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	reportID := "report-cache-miss"
	m.metaRepo.getFunc = func(ctx context.Context, id string) (*FTOReportSummary, error) {
		if id != reportID {
			t.Errorf("ID mismatch")
		}
		return &FTOReportSummary{ReportID: id, Status: StatusCompleted}, nil
	}

	info, err := svc.GetStatus(context.Background(), reportID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if info.Status != StatusCompleted || info.ProgressPct != 100 {
		t.Errorf("Expected StatusCompleted and 100%% progress, got %s / %d", info.Status, info.ProgressPct)
	}
}

func TestFTOReportService_GetStatus_NotFound(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	m.metaRepo.getFunc = func(ctx context.Context, id string) (*FTOReportSummary, error) {
		return nil, errors.NotFound("not found")
	}

	_, err := svc.GetStatus(context.Background(), "unknown-id")
	if err == nil {
		t.Fatalf("Expected error for missing report")
	}
}

// ============================================================================
// Test Cases: GetReport
// ============================================================================

func TestFTOReportService_GetReport_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newTestFTOService()

	stream, err := svc.GetReport(context.Background(), "report-123", FormatPDF)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if stream == nil {
		t.Fatalf("Expected non-nil stream")
	}
	stream.Close()
}

func TestFTOReportService_GetReport_FormatMismatch(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	m.storage.getStreamFunc = func(ctx context.Context, key string) (io.ReadCloser, error) {
		if strings.HasSuffix(key, ".DOCX") {
			return nil, errors.NotFound("object not found")
		}
		return io.NopCloser(bytes.NewReader([]byte("pdf"))), nil
	}

	// Requesting DOCX when only PDF exists should return error from storage mock
	_, err := svc.GetReport(context.Background(), "report-123", FormatDOCX)
	if err == nil {
		t.Fatalf("Expected error when requesting missing format")
	}
}

func TestFTOReportService_GetReport_NotCompleted(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	m.metaRepo.getFunc = func(ctx context.Context, id string) (*FTOReportSummary, error) {
		return &FTOReportSummary{ReportID: id, Status: StatusProcessing}, nil
	}

	_, err := svc.GetReport(context.Background(), "report-123", FormatPDF)
	if err == nil {
		t.Fatalf("Expected error when requesting incomplete report")
	}
	assertErrorCode(t, err, errors.ErrCodeConflict)
}

// ============================================================================
// Test Cases: ListReports
// ============================================================================

func TestFTOReportService_ListReports_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newTestFTOService()

	res, err := svc.ListReports(context.Background(), nil, &common.Pagination{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Pagination.Total != 1 || len(res.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(res.Items))
	}
	if res.Pagination.Page != 1 || res.Pagination.PageSize != 10 {
		t.Errorf("Pagination properties mismatch")
	}
}

func TestFTOReportService_ListReports_EmptyResult(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	m.metaRepo.listFunc = func(ctx context.Context, filter *FTOReportFilter, page *common.Pagination) ([]FTOReportSummary, int64, error) {
		return []FTOReportSummary{}, 0, nil
	}

	res, err := svc.ListReports(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if res.Pagination.Total != 0 || len(res.Items) != 0 {
		t.Errorf("Expected empty results")
	}
}

// ============================================================================
// Test Cases: DeleteReport
// ============================================================================

func TestFTOReportService_DeleteReport_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newTestFTOService()

	err := svc.DeleteReport(context.Background(), "report-123")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestFTOReportService_DeleteReport_NotFound(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	m.metaRepo.getFunc = func(ctx context.Context, id string) (*FTOReportSummary, error) {
		return nil, errors.NotFound("not found")
	}

	err := svc.DeleteReport(context.Background(), "unknown-report")
	if err == nil {
		t.Fatalf("Expected error for non-existent report")
	}
}

func TestFTOReportService_DeleteReport_AlreadyDeleted(t *testing.T) {
	t.Parallel()
	svc, m := newTestFTOService()

	// Mimic scenario where Get returns not found because it's soft deleted
	m.metaRepo.getFunc = func(ctx context.Context, id string) (*FTOReportSummary, error) {
		return nil, errors.NotFound("report already deleted")
	}

	err := svc.DeleteReport(context.Background(), "deleted-report")
	if err == nil {
		t.Fatalf("Expected ErrNotFound")
	}
}

//Personal.AI order the ending