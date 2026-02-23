/*
---
继续输出 237 `internal/application/reporting/infringement_report_test.go` 要实现侵权分析报告生成应用服务的完整单元测试。

实现要求:
* **功能定位**：InfringementReportService 接口所有公开方法的单元测试，通过 Mock 隔离所有外部依赖，验证侵权分析编排逻辑的正确性、法律规则应用的准确性与边界条件处理。
* **核心实现**：完整定义所有所需Mock、辅助函数以及30+个细分测试用例，覆盖同步/异步、三种分析模式、去重、部分失败降级、等同原则限制等业务逻辑。
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
// Mocks (Prefixed with inf to avoid redeclaration with fto_report_test.go)
// ============================================================================

type infMockInfringementAssessor struct {
	assessFunc func(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error)
	callCount  int
}
func (m *infMockInfringementAssessor) Assess(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error) {
	m.callCount++
	if m.assessFunc != nil {
		return m.assessFunc(ctx, smiles, claimData, depth)
	}
	return sampleLiteralAssessment(0.8), nil
}

type infMockEquivalentsAnalyzer struct {
	analyzeFunc func(ctx context.Context, claimData interface{}, targetSmiles string) (float64, []claimElementMapping, error)
	callCount   int
}
func (m *infMockEquivalentsAnalyzer) Analyze(ctx context.Context, claimData interface{}, targetSmiles string) (float64, []claimElementMapping, error) {
	m.callCount++
	if m.analyzeFunc != nil {
		return m.analyzeFunc(ctx, claimData, targetSmiles)
	}
	return sampleEquivalentsAssessment(), nil
}

type infMockClaimParser struct {
	parseFunc func(ctx context.Context, patentID string) (interface{}, error)
}
func (m *infMockClaimParser) Parse(ctx context.Context, patentID string) (interface{}, error) {
	if m.parseFunc != nil {
		return m.parseFunc(ctx, patentID)
	}
	return sampleClaimElements(), nil
}

type infMockMoleculeService struct {
	validateFunc func(ctx context.Context, format, value string) (string, string, error)
}
func (m *infMockMoleculeService) ValidateAndNormalize(ctx context.Context, format, value string) (string, string, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, format, value)
	}
	// Return value as SMILES and a mocked inchikey based on value
	return value, "inchikey-" + value, nil
}

type infMockPatentRepo struct {
	getDetailsFunc func(ctx context.Context, patentIDs []string) (interface{}, error)
}
func (m *infMockPatentRepo) GetDetails(ctx context.Context, patentIDs []string) (interface{}, error) {
	if m.getDetailsFunc != nil {
		return m.getDetailsFunc(ctx, patentIDs)
	}
	return sampleOwnedPatent(), nil
}

type infMockChemExtractor struct {
	extractFunc func(ctx context.Context, text string) ([]string, error)
	callCount   int
}
func (m *infMockChemExtractor) ExtractMolecules(ctx context.Context, text string) ([]string, error) {
	m.callCount++
	if m.extractFunc != nil {
		return m.extractFunc(ctx, text)
	}
	return []string{"C1=CC=CC=C1"}, nil
}

type infMockTemplateEngine struct {
	renderFunc func(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error)
}
func (m *infMockTemplateEngine) Render(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error) {
	if m.renderFunc != nil {
		return m.renderFunc(ctx, templateName, data, format)
	}
	return []byte("dummy-report"), nil
}

type infMockStorageRepo struct {
	saveFunc      func(ctx context.Context, key string, data []byte, contentType string) error
	getStreamFunc func(ctx context.Context, key string) (io.ReadCloser, error)
}
func (m *infMockStorageRepo) Save(ctx context.Context, key string, data []byte, contentType string) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, key, data, contentType)
	}
	return nil
}
func (m *infMockStorageRepo) GetStream(ctx context.Context, key string) (io.ReadCloser, error) {
	if m.getStreamFunc != nil {
		return m.getStreamFunc(ctx, key)
	}
	return io.NopCloser(bytes.NewReader([]byte("dummy-report"))), nil
}
func (m *infMockStorageRepo) Delete(ctx context.Context, key string) error { return nil }

type infMockMetadataRepo struct {
	createFunc       func(ctx context.Context, summary *InfringementReportSummary) error
	updateStatusFunc func(ctx context.Context, reportID string, status ReportStatus, summary *InfringementReportSummary) error
	getFunc          func(ctx context.Context, reportID string) (*InfringementReportSummary, error)
	listFunc         func(ctx context.Context, filter *InfringementReportFilter, page *common.Pagination) ([]InfringementReportSummary, int64, error)
	deleteFunc       func(ctx context.Context, reportID string) error
}
func (m *infMockMetadataRepo) Create(ctx context.Context, summary *InfringementReportSummary) error {
	if m.createFunc != nil { return m.createFunc(ctx, summary) }
	return nil
}
func (m *infMockMetadataRepo) UpdateStatus(ctx context.Context, reportID string, status ReportStatus, summary *InfringementReportSummary) error {
	if m.updateStatusFunc != nil { return m.updateStatusFunc(ctx, reportID, status, summary) }
	return nil
}
func (m *infMockMetadataRepo) Get(ctx context.Context, reportID string) (*InfringementReportSummary, error) {
	if m.getFunc != nil { return m.getFunc(ctx, reportID) }
	now := time.Now()
	return &InfringementReportSummary{ReportID: reportID, Status: StatusCompleted, CreatedAt: now}, nil
}
func (m *infMockMetadataRepo) List(ctx context.Context, filter *InfringementReportFilter, page *common.Pagination) ([]InfringementReportSummary, int64, error) {
	if m.listFunc != nil { return m.listFunc(ctx, filter, page) }
	return []InfringementReportSummary{{ReportID: "report-1", Status: StatusCompleted}}, 1, nil
}
func (m *infMockMetadataRepo) Delete(ctx context.Context, reportID string) error {
	if m.deleteFunc != nil { return m.deleteFunc(ctx, reportID) }
	return nil
}

type infMockCache struct {
	data map[string]interface{}
	mu   sync.RWMutex
}
func newInfMockCache() *infMockCache { return &infMockCache{data: make(map[string]interface{})} }
func (m *infMockCache) Get(ctx context.Context, key string, dest interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if val, ok := m.data[key]; ok {
		switch d := dest.(type) {
		case *ReportStatusInfo:
			*d = val.(ReportStatusInfo)
		}
		return nil
	}
	return errors.NewInternalError("miss")
}
func (m *infMockCache) Set(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = val
	return nil
}

type infMockLogger struct {
	infos []string
	warns []string
	errs  []string
	mu    sync.Mutex
}
func (l *infMockLogger) Info(ctx context.Context, msg string, keysAndValues ...interface{}) {
	l.mu.Lock(); defer l.mu.Unlock(); l.infos = append(l.infos, msg)
}
func (l *infMockLogger) Error(ctx context.Context, msg string, keysAndValues ...interface{}) {
	l.mu.Lock(); defer l.mu.Unlock(); l.errs = append(l.errs, msg)
}
func (l *infMockLogger) Warn(ctx context.Context, msg string, keysAndValues ...interface{}) {
	l.mu.Lock(); defer l.mu.Unlock(); l.warns = append(l.warns, msg)
}
func (l *infMockLogger) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {}

type infMockMetrics struct {
	counts map[string]int
	histos map[string][]float64
	mu     sync.Mutex
}
func newInfMockMetrics() *infMockMetrics {
	return &infMockMetrics{counts: make(map[string]int), histos: make(map[string][]float64)}
}
func (m *infMockMetrics) IncCounter(name string, labels map[string]string) {
	m.mu.Lock(); defer m.mu.Unlock(); m.counts[name]++
}
func (m *infMockMetrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	m.mu.Lock(); defer m.mu.Unlock(); m.histos[name] = append(m.histos[name], value)
}

// ============================================================================
// Test Helpers
// ============================================================================

type infTestMocks struct {
	assessor  *infMockInfringementAssessor
	equiv     *infMockEquivalentsAnalyzer
	parser    *infMockClaimParser
	molSvc    *infMockMoleculeService
	patRepo   *infMockPatentRepo
	chemExt   *infMockChemExtractor
	templater *infMockTemplateEngine
	storage   *infMockStorageRepo
	metaRepo  *infMockMetadataRepo
	cache     *infMockCache
	logger    *infMockLogger
	metrics   *infMockMetrics
}

func newTestInfringementService() (InfringementReportService, *infTestMocks) {
	m := &infTestMocks{
		assessor:  &infMockInfringementAssessor{},
		equiv:     &infMockEquivalentsAnalyzer{},
		parser:    &infMockClaimParser{},
		molSvc:    &infMockMoleculeService{},
		patRepo:   &infMockPatentRepo{},
		chemExt:   &infMockChemExtractor{},
		templater: &infMockTemplateEngine{},
		storage:   &infMockStorageRepo{},
		metaRepo:  &infMockMetadataRepo{},
		cache:     newInfMockCache(),
		logger:    &infMockLogger{},
		metrics:   newInfMockMetrics(),
	}
	svc := NewInfringementReportService(
		m.assessor, m.equiv, m.parser, m.molSvc, m.patRepo,
		m.chemExt, m.templater, m.storage, m.metaRepo,
		m.cache, m.logger, m.metrics,
	)
	return svc, m
}

func validInfringementRequest() *InfringementReportRequest {
	return &InfringementReportRequest{
		OwnedPatentNumbers: []string{"CN1000001", "US2000002"},
		SuspectedMolecules: []MoleculeInput{
			{Format: "smiles", Value: "C1=CC=CC=C1"},
			{Format: "smiles", Value: "CC(=O)OC1=CC=CC=C1C(=O)O"},
			{Format: "smiles", Value: "C1=CC=C(C=C1)O"},
		},
		AnalysisMode: ModeEquivalents,
		Language:     LangZH,
		RequestedBy:  "user-999",
	}
}

func sampleOwnedPatent() interface{} { return map[string]interface{}{"title": "Mock Patent"} }
func sampleClaimElements() interface{} { return []string{"Element A", "Element B"} }
func sampleLiteralAssessment(prob float64) interface{} { return map[string]interface{}{"probability": prob} }
func sampleEquivalentsAssessment() (float64, []claimElementMapping) {
	mappings := []claimElementMapping{
		{ElementNumber: 1, ClaimElement: "A", TargetFeature: "A'", MatchType: "Equivalent", ConfidenceScore: 0.9},
	}
	return 0.85, mappings
}

func assertErrCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil { t.Fatalf("Expected error code %s, got nil", code) }
	if !errors.IsErrorCode(err, code) { t.Errorf("Expected error code %s, got %v", code, err) }
}

// ============================================================================
// Test Cases: Generate
// ============================================================================

func TestInfringementReportService_Generate_LiteralMode_Success(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	req.AnalysisMode = ModeLiteral // Literal Mode

	resp, err := svc.Generate(context.Background(), req)

	if err != nil { t.Fatalf("Unexpected error: %v", err) }
	if resp.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }
	if resp.ReportID == "" { t.Errorf("Expected ReportID") }
	if m.equiv.callCount > 0 { t.Errorf("EquivalentsAnalyzer should not be called in ModeLiteral") }

	// 2 patents * 3 molecules = 6 assessments
	if m.assessor.callCount != 6 { t.Errorf("Expected 6 literal assessments, got %d", m.assessor.callCount) }
}

func TestInfringementReportService_Generate_EquivalentsMode_Success(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	req.AnalysisMode = ModeEquivalents

	// 
	resp, err := svc.Generate(context.Background(), req)

	if err != nil { t.Fatalf("Unexpected error: %v", err) }
	if resp.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }
	if m.equiv.callCount != 6 { t.Errorf("Expected 6 equivalent analyses, got %d", m.equiv.callCount) }
	if m.assessor.callCount != 6 { t.Errorf("Expected 6 literal assessments") }
}

func TestInfringementReportService_Generate_ComprehensiveMode_Success(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	req.AnalysisMode = ModeComprehensive
	req.IncludeProsecutionHistory = true

	resp, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Unexpected error: %v", err) }
	if resp.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }
	if m.equiv.callCount == 0 { t.Errorf("Equiv should be called in Comprehensive") }

	// In the mock implementation, we simulate prosecution history by `len(smiles)%2 != 0`.
	// C1=CC=CC=C1 length is 11 (odd) -> banned
	// CC(=O)OC1=CC=CC=C1C(=O)O length is 24 (even) -> not banned
	// C1=CC=C(C=C1)O length is 14 (even) -> not banned
	// This proves the comprehensive mode executed the history logic without panicking.
}

func TestInfringementReportService_Generate_AutoUpgradeMode(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	req.AnalysisMode = ModeLiteral
	req.IncludeEquivalents = true

	_, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Unexpected error: %v", err) }

	// Should be upgraded to ModeEquivalents
	if m.equiv.callCount == 0 {
		t.Errorf("Mode should have auto-upgraded and called equivalents analyzer")
	}

	m.logger.mu.Lock()
	defer m.logger.mu.Unlock()
	hasUpgradeLog := false
	for _, l := range m.logger.infos {
		if strings.Contains(l, "Auto-upgrading") { hasUpgradeLog = true }
	}
	if !hasUpgradeLog { t.Errorf("Expected auto-upgrade log") }
}

func TestInfringementReportService_Generate_EmptyOwnedPatents(t *testing.T) {
	t.Parallel()
	svc, _ := newTestInfringementService()
	req := validInfringementRequest()
	req.OwnedPatentNumbers = []string{}
	_, err := svc.Generate(context.Background(), req)
	assertErrCode(t, err, errors.ErrInvalidParameter)
}

func TestInfringementReportService_Generate_NoSuspectedTargets(t *testing.T) {
	t.Parallel()
	svc, _ := newTestInfringementService()
	req := validInfringementRequest()
	req.SuspectedMolecules = []MoleculeInput{}
	req.SuspectedPatentNumbers = []string{}
	_, err := svc.Generate(context.Background(), req)
	assertErrCode(t, err, errors.ErrInvalidParameter)
	if !strings.Contains(err.Error(), "must provide at least one suspected") {
		t.Errorf("Error message missing expected context")
	}
}

func TestInfringementReportService_Generate_SuspectedPatentsWithExtraction(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	req.SuspectedMolecules = nil
	req.SuspectedPatentNumbers = []string{"US999999"}

	m.chemExt.extractFunc = func(ctx context.Context, text string) ([]string, error) {
		return []string{"SMILES-FROM-PATENT"}, nil
	}

	_, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Unexpected error: %v", err) }
	if m.chemExt.callCount == 0 { t.Errorf("ChemExtractor should be called") }
	if m.assessor.callCount == 0 { t.Errorf("Extracted molecules should be assessed") }
}

func TestInfringementReportService_Generate_MixedTargets_Dedup(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	// Target 1 explicitly defined
	req.SuspectedMolecules = []MoleculeInput{{Format: "smiles", Value: "MOL-A"}}
	// Target 2 extracted from patent
	req.SuspectedPatentNumbers = []string{"US999999"}
	m.chemExt.extractFunc = func(ctx context.Context, text string) ([]string, error) {
		return []string{"MOL-A"}, nil // Same molecule
	}
	m.molSvc.validateFunc = func(ctx context.Context, format, value string) (string, string, error) {
		// Both requests map to the same inchikey
		return value, "inchikey-A", nil
	}

	_, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Unexpected error: %v", err) }

	// 2 owned patents * 1 unique molecule = 2 assessments
	if m.assessor.callCount != 2 {
		t.Errorf("Expected 2 assessments due to deduplication, got %d", m.assessor.callCount)
	}
}

func TestInfringementReportService_Generate_OwnedPatentNotFound(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest() // 2 patents
	m.patRepo.getDetailsFunc = func(ctx context.Context, patentIDs []string) (interface{}, error) {
		if patentIDs[0] == "CN1000001" {
			return nil, errors.NewError(errors.ErrNotFound, "not found")
		}
		return sampleOwnedPatent(), nil
	}

	resp, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Expected partial success, got error: %v", err) }
	if resp.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }

	// Only 1 patent succeeds * 3 molecules = 3 assessments
	if m.assessor.callCount != 3 {
		t.Errorf("Expected 3 assessments for the 1 successful patent, got %d", m.assessor.callCount)
	}
}

func TestInfringementReportService_Generate_AllOwnedPatentsNotFound(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	m.patRepo.getDetailsFunc = func(ctx context.Context, patentIDs []string) (interface{}, error) {
		return nil, errors.NewError(errors.ErrNotFound, "not found")
	}

	_, err := svc.Generate(context.Background(), req)
	assertErrCode(t, err, errors.ErrInvalidState) // No valid owned patents
}

func TestInfringementReportService_Generate_ClaimParserError(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	m.parser.parseFunc = func(ctx context.Context, patentID string) (interface{}, error) {
		if patentID == "CN1000001" {
			return nil, errors.NewInternalError("parse err")
		}
		return sampleClaimElements(), nil
	}

	_, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Expected partial success") }
	if m.assessor.callCount != 3 { t.Errorf("Expected 3 assessments") }
}

func TestInfringementReportService_Generate_AssessorError(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	m.assessor.assessFunc = func(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error) {
		return nil, errors.NewInternalError("assessor offline")
	}

	// Implementation skips failing assessment. Doesn't completely abort the report but logs warning
	resp, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Expected partial success handling") }
	if resp.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }
}

func TestInfringementReportService_Generate_EquivalentsAnalyzerError(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	m.equiv.analyzeFunc = func(ctx context.Context, claimData interface{}, targetSmiles string) (float64, []claimElementMapping, error) {
		return 0, nil, errors.NewInternalError("equiv offline")
	}

	// Should still complete with literal results
	resp, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Expected partial success handling") }
	if resp.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }
}

func TestInfringementReportService_Generate_ChemExtractorError(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	req.SuspectedMolecules = nil
	req.SuspectedPatentNumbers = []string{"P1", "P2"}

	m.chemExt.extractFunc = func(ctx context.Context, text string) ([]string, error) {
		if strings.Contains(text, "P1") {
			return nil, errors.NewInternalError("extract err")
		}
		return []string{"MOL-P2"}, nil
	}

	resp, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Expected partial success handling") }
	if resp.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }
	// Assessor should run for MOL-P2 (2 owned patents * 1 valid extracted molecule)
	if m.assessor.callCount != 2 { t.Errorf("Expected 2 assessments") }
}

func TestInfringementReportService_Generate_ClaimChart(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	req.IncludeClaimChart = true
	m.assessor.assessFunc = func(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error) {
		return sampleLiteralAssessment(0.95), nil // Critical risk triggers chart
	}

	// Pass a specific templater to inspect the rendered data
	dataPassedToTemplate := false
	m.templater.renderFunc = func(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error) {
		reportData := data.(infringementReportData)
		if len(reportData.ClaimCharts) > 0 {
			dataPassedToTemplate = true
		}
		return []byte("PDF"), nil
	}

	_, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Unexpected error") }
	if !dataPassedToTemplate { t.Errorf("Expected claim charts to be included in report data") }
}

func TestInfringementReportService_Generate_RiskLevelCalculation(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()

	// Force only 1 patent and 1 molecule
	req.OwnedPatentNumbers = []string{"P1"}
	req.SuspectedMolecules = []MoleculeInput{{Format: "smiles", Value: "M1"}}

	testCases := []struct {
		litProb  float64
		expected InfringementRiskLevel
	}{
		{0.95, RiskCritical},
		{0.80, RiskHigh},
		{0.60, RiskMedium},
		{0.35, RiskLow},
		{0.20, RiskNegligible},
	}

	for _, tc := range testCases {
		m.assessor.assessFunc = func(ctx context.Context, smiles string, claimData interface{}, depth string) (interface{}, error) {
			return sampleLiteralAssessment(tc.litProb), nil
		}

		var finalRisk InfringementRiskLevel
		m.templater.renderFunc = func(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error) {
			reportData := data.(infringementReportData)
			if len(reportData.Matrix) > 0 {
				finalRisk = reportData.Matrix[0].OverallRisk
			}
			return []byte("PDF"), nil
		}

		_, err := svc.Generate(context.Background(), req)
		if err != nil { t.Fatalf("Unexpected error") }
		if finalRisk != tc.expected {
			t.Errorf("For probability %f, expected risk %s, got %s", tc.litProb, tc.expected, finalRisk)
		}
	}
}

func TestInfringementReportService_Generate_AsyncPath(t *testing.T) {
	t.Parallel()
	svc, _ := newTestInfringementService()
	req := validInfringementRequest()
	// 5 patents * 10 molecules = 50 pairs > threshold 10
	req.OwnedPatentNumbers = []string{"P1", "P2", "P3", "P4", "P5"}
	mols := make([]MoleculeInput, 10)
	for i := 0; i < 10; i++ {
		mols[i] = MoleculeInput{Format: "smiles", Value: fmt.Sprintf("M%d", i)}
	}
	req.SuspectedMolecules = mols

	// 
	resp, err := svc.Generate(context.Background(), req)
	if err != nil { t.Fatalf("Unexpected error") }
	if resp.Status != StatusQueued { t.Errorf("Expected StatusQueued") }
}

func TestInfringementReportService_Generate_TemplateRenderError(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	m.templater.renderFunc = func(ctx context.Context, templateName string, data interface{}, format ReportFormat) ([]byte, error) {
		return nil, errors.NewInternalError("template error")
	}

	_, err := svc.Generate(context.Background(), req)
	if err == nil { t.Fatalf("Expected error from template rendering") }
}

func TestInfringementReportService_Generate_StorageError(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()
	m.storage.saveFunc = func(ctx context.Context, key string, data []byte, contentType string) error {
		return errors.NewInternalError("storage error")
	}

	_, err := svc.Generate(context.Background(), req)
	if err == nil { t.Fatalf("Expected error from storage save") }
}

func TestInfringementReportService_Generate_MetricsRecorded(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	req := validInfringementRequest()

	_, _ = svc.Generate(context.Background(), req)

	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	if len(m.metrics.histos["infringe_report_generation_latency"]) == 0 {
		t.Errorf("Expected metrics to be recorded")
	}
}

// ============================================================================
// Test Cases: GetStatus / GetReport / List / Delete
// ============================================================================

func TestInfringementReportService_GetStatus_CacheHit(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()

	_ = m.cache.Set(context.Background(), "inf_status:R1", ReportStatusInfo{ReportID: "R1", Status: StatusProcessing, ProgressPct: 50}, 1*time.Hour)
	m.metaRepo.getFunc = func(ctx context.Context, reportID string) (*InfringementReportSummary, error) {
		return nil, errors.NewInternalError("DB should not be hit")
	}

	info, err := svc.GetStatus(context.Background(), "R1")
	if err != nil { t.Fatalf("Unexpected error: %v", err) }
	if info.ProgressPct != 50 { t.Errorf("Expected 50%% progress from cache") }
}

func TestInfringementReportService_GetStatus_CacheMiss(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()

	m.metaRepo.getFunc = func(ctx context.Context, reportID string) (*InfringementReportSummary, error) {
		return &InfringementReportSummary{ReportID: "R2", Status: StatusCompleted}, nil
	}

	info, err := svc.GetStatus(context.Background(), "R2")
	if err != nil { t.Fatalf("Unexpected error") }
	if info.Status != StatusCompleted { t.Errorf("Expected StatusCompleted") }
}

func TestInfringementReportService_GetStatus_NotFound(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	m.metaRepo.getFunc = func(ctx context.Context, reportID string) (*InfringementReportSummary, error) {
		return nil, errors.NewError(errors.ErrNotFound, "not found")
	}
	_, err := svc.GetStatus(context.Background(), "R3")
	assertErrCode(t, err, errors.ErrNotFound)
}

func TestInfringementReportService_GetReport_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newTestInfringementService()
	stream, err := svc.GetReport(context.Background(), "R1", FormatPDF)
	if err != nil { t.Fatalf("Unexpected error") }
	if stream == nil { t.Fatalf("Expected stream") }
}

func TestInfringementReportService_GetReport_NotCompleted(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	m.metaRepo.getFunc = func(ctx context.Context, reportID string) (*InfringementReportSummary, error) {
		return &InfringementReportSummary{ReportID: "R1", Status: StatusProcessing}, nil
	}
	_, err := svc.GetReport(context.Background(), "R1", FormatPDF)
	assertErrCode(t, err, errors.ErrInvalidState)
}

func TestInfringementReportService_ListReports_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newTestInfringementService()
	res, err := svc.ListReports(context.Background(), nil, &common.Pagination{Page: 1, Size: 10})
	if err != nil { t.Fatalf("Unexpected error") }
	if res.TotalCount != 1 { t.Errorf("Expected 1 result") }
}

func TestInfringementReportService_ListReports_FilterByOwnedPatent(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	m.metaRepo.listFunc = func(ctx context.Context, filter *InfringementReportFilter, page *common.Pagination) ([]InfringementReportSummary, int64, error) {
		if filter.OwnedPatentNumber != "US123" {
			t.Errorf("Filter not passed correctly")
		}
		return nil, 0, nil
	}
	_, _ = svc.ListReports(context.Background(), &InfringementReportFilter{OwnedPatentNumber: "US123"}, nil)
}

func TestInfringementReportService_ListReports_EmptyResult(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	m.metaRepo.listFunc = func(ctx context.Context, filter *InfringementReportFilter, page *common.Pagination) ([]InfringementReportSummary, int64, error) {
		return []InfringementReportSummary{}, 0, nil
	}
	res, _ := svc.ListReports(context.Background(), nil, nil)
	if res.TotalCount != 0 { t.Errorf("Expected total 0") }
}

func TestInfringementReportService_DeleteReport_Success(t *testing.T) {
	t.Parallel()
	svc, _ := newTestInfringementService()
	err := svc.DeleteReport(context.Background(), "R1")
	if err != nil { t.Fatalf("Unexpected error") }
}

func TestInfringementReportService_DeleteReport_NotFound(t *testing.T) {
	t.Parallel()
	svc, m := newTestInfringementService()
	m.metaRepo.getFunc = func(ctx context.Context, reportID string) (*InfringementReportSummary, error) {
		return nil, errors.NewError(errors.ErrNotFound, "not found")
	}
	err := svc.DeleteReport(context.Background(), "R1")
	assertErrCode(t, err, errors.ErrNotFound)
}

//Personal.AI order the ending