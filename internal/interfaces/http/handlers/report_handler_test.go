// Phase 11 - 接口层: HTTP Handler - 报告生成单元测试
// 序号: 271
// 文件: internal/interfaces/http/handlers/report_handler_test.go
// 功能定位: 验证 ReportHandler 全部端点的行为正确性
// 测试用例:
//   - TestGenerateFTOReport_Success: 正常发起 FTO 报告生成，返回 202
//   - TestGenerateFTOReport_MissingSMILES: 缺少 target_smiles，返回 400
//   - TestGenerateFTOReport_MissingJurisdiction: 缺少 jurisdiction，返回 400
//   - TestGenerateFTOReport_InvalidFormat: 非法导出格式，返回 400
//   - TestGenerateFTOReport_DefaultFormat: 未指定 format 时默认 pdf
//   - TestGenerateInfringementReport_Success: 正常发起侵权报告生成
//   - TestGenerateInfringementReport_MissingPatentNumber: 缺少专利号
//   - TestGenerateInfringementReport_EmptyTargetSMILES: 空目标分子列表
//   - TestGeneratePortfolioReport_Success: 正常发起组合报告生成
//   - TestGeneratePortfolioReport_MissingPortfolioID: 缺少组合 ID
//   - TestGeneratePortfolioReport_MissingReportType: 缺少报告类型
//   - TestGetReportStatus_Completed: 已完成报告状态查询
//   - TestGetReportStatus_InProgress: 进行中报告状态查询
//   - TestGetReportStatus_NotFound: 报告不存在
//   - TestDownloadReport_Success: 正常下载已完成报告
//   - TestDownloadReport_NotReady: 报告未完成时下载返回 409
//   - TestDownloadReport_ContentType: 验证不同格式的 Content-Type
//   - TestListReports_Default: 默认分页列表
//   - TestListReports_WithFilters: 带类型/状态/日期过滤
//   - TestListReports_InvalidPageSize: 非法分页大小
//   - TestDeleteReport_Success: 正常删除报告
//   - TestDeleteReport_NotFound: 删除不存在的报告
//   - TestIsValidFormat: 格式校验辅助函数
//   - TestFormatToContentType: Content-Type 映射辅助函数
// Mock 依赖: mockFTOReportService, mockInfringementReportService, mockPortfolioReportService, mockTemplateService, mockLogger
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// --- Mock Services ---

type mockFTOReportService struct {
	mock.Mock
}

func (m *mockFTOReportService) Generate(ctx context.Context, req *reporting.FTOReportRequest) (*reporting.ReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportResult), args.Error(1)
}

func (m *mockFTOReportService) QuickCheck(ctx context.Context, req *reporting.FTOQuickCheckRequest) (*reporting.FTOQuickCheckResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.FTOQuickCheckResult), args.Error(1)
}

func (m *mockFTOReportService) GetStatus(ctx context.Context, reportID string) (*reporting.ReportStatus, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportStatus), args.Error(1)
}

func (m *mockFTOReportService) Download(ctx context.Context, reportID string) (*reporting.ReportFile, error) {
	args := m.Called(ctx, reportID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportFile), args.Error(1)
}

func (m *mockFTOReportService) List(ctx context.Context, filter *reporting.ReportListFilter) (*reporting.ReportListResult, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportListResult), args.Error(1)
}

func (m *mockFTOReportService) Delete(ctx context.Context, reportID string) error {
	args := m.Called(ctx, reportID)
	return args.Error(0)
}

type mockInfringementReportService struct {
	mock.Mock
}

func (m *mockInfringementReportService) Generate(ctx context.Context, req *reporting.InfringementReportRequest) (*reporting.ReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportResult), args.Error(1)
}

type mockPortfolioReportService struct {
	mock.Mock
}

func (m *mockPortfolioReportService) Generate(ctx context.Context, req *reporting.PortfolioReportRequest) (*reporting.ReportResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reporting.ReportResult), args.Error(1)
}

type mockTemplateService struct {
	mock.Mock
}

func newTestReportHandler() (*ReportHandler, *mockFTOReportService, *mockInfringementReportService, *mockPortfolioReportService) {
	ftoSvc := new(mockFTOReportService)
	infSvc := new(mockInfringementReportService)
	portSvc := new(mockPortfolioReportService)
	tmplSvc := new(mockTemplateService)
	logger := new(mockHandlerLogger)
	logger.On("Error", mock.Anything, mock.Anything).Maybe()
	logger.On("Info", mock.Anything, mock.Anything).Maybe()

	h := NewReportHandler(ftoSvc, infSvc, portSvc, tmplSvc, logger)
	return h, ftoSvc, infSvc, portSvc
}

func makeReportRequest(method, path string, body interface{}) *http.Request {
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// --- FTO Report Tests ---

func TestGenerateFTOReport_Success(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("Generate", mock.Anything, mock.AnythingOfType("*reporting.FTOReportRequest")).Return(
		&reporting.ReportResult{ReportID: "rpt-001", Status: "pending"}, nil)

	body := GenerateFTOReportRequest{
		TargetSMILES: "c1ccccc1",
		Jurisdiction: "CN",
		Format:       "pdf",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/fto", body)
	h.GenerateFTOReport(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "rpt-001", resp["report_id"])
	assert.Equal(t, "pending", resp["status"])
	ftoSvc.AssertExpectations(t)
}

func TestGenerateFTOReport_MissingSMILES(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	body := GenerateFTOReportRequest{
		Jurisdiction: "CN",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/fto", body)
	h.GenerateFTOReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "target_smiles")
}

func TestGenerateFTOReport_MissingJurisdiction(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	body := GenerateFTOReportRequest{
		TargetSMILES: "c1ccccc1",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/fto", body)
	h.GenerateFTOReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "jurisdiction")
}

func TestGenerateFTOReport_InvalidFormat(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	body := GenerateFTOReportRequest{
		TargetSMILES: "c1ccccc1",
		Jurisdiction: "CN",
		Format:       "html",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/fto", body)
	h.GenerateFTOReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "format")
}

func TestGenerateFTOReport_DefaultFormat(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("Generate", mock.Anything, mock.MatchedBy(func(req *reporting.FTOReportRequest) bool {
		return req.Format == "pdf"
	})).Return(&reporting.ReportResult{ReportID: "rpt-002", Status: "pending"}, nil)

	body := GenerateFTOReportRequest{
		TargetSMILES: "c1ccccc1",
		Jurisdiction: "US",
		// Format intentionally omitted
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/fto", body)
	h.GenerateFTOReport(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)
	ftoSvc.AssertExpectations(t)
}

func TestGenerateFTOReport_ServiceError(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("Generate", mock.Anything, mock.Anything).Return(
		nil, errors.NewAppError(errors.ErrCodeInternal, "generation failed"))

	body := GenerateFTOReportRequest{
		TargetSMILES: "c1ccccc1",
		Jurisdiction: "CN",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/fto", body)
	h.GenerateFTOReport(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGenerateFTOReport_InvalidJSON(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/reports/fto", strings.NewReader("{invalid"))
	r.Header.Set("Content-Type", "application/json")
	h.GenerateFTOReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Infringement Report Tests ---

func TestGenerateInfringementReport_Success(t *testing.T) {
	h, _, infSvc, _ := newTestReportHandler()

	infSvc.On("Generate", mock.Anything, mock.AnythingOfType("*reporting.InfringementReportRequest")).Return(
		&reporting.ReportResult{ReportID: "rpt-inf-001", Status: "pending"}, nil)

	body := GenerateInfringementReportRequest{
		PatentNumber: "CN202310001234.5",
		TargetSMILES: []string{"c1ccccc1", "CC(=O)O"},
		Format:       "docx",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/infringement", body)
	h.GenerateInfringementReport(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "rpt-inf-001", resp["report_id"])
	infSvc.AssertExpectations(t)
}

func TestGenerateInfringementReport_MissingPatentNumber(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	body := GenerateInfringementReportRequest{
		TargetSMILES: []string{"c1ccccc1"},
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/infringement", body)
	h.GenerateInfringementReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "patent_number")
}

func TestGenerateInfringementReport_EmptyTargetSMILES(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	body := GenerateInfringementReportRequest{
		PatentNumber: "CN202310001234.5",
		TargetSMILES: []string{},
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/infringement", body)
	h.GenerateInfringementReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "target_smiles")
}

// --- Portfolio Report Tests ---

func TestGeneratePortfolioReport_Success(t *testing.T) {
	h, _, _, portSvc := newTestReportHandler()

	portSvc.On("Generate", mock.Anything, mock.AnythingOfType("*reporting.PortfolioReportRequest")).Return(
		&reporting.ReportResult{ReportID: "rpt-port-001", Status: "pending"}, nil)

	body := GeneratePortfolioReportRequest{
		PortfolioID:   "portfolio-abc",
		ReportType:    "landscape",
		Format:        "xlsx",
		IncludeCharts: true,
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/portfolio", body)
	h.GeneratePortfolioReport(w, r)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "rpt-port-001", resp["report_id"])
	portSvc.AssertExpectations(t)
}

func TestGeneratePortfolioReport_MissingPortfolioID(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	body := GeneratePortfolioReportRequest{
		ReportType: "landscape",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/portfolio", body)
	h.GeneratePortfolioReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "portfolio_id")
}

func TestGeneratePortfolioReport_MissingReportType(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	body := GeneratePortfolioReportRequest{
		PortfolioID: "portfolio-abc",
	}

	w := httptest.NewRecorder()
	r := makeReportRequest("POST", "/api/v1/reports/portfolio", body)
	h.GeneratePortfolioReport(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "report_type")
}

// --- GetReportStatus Tests ---

func TestGetReportStatus_Completed(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	now := time.Now()
	ftoSvc.On("GetStatus", mock.Anything, "rpt-001").Return(&reporting.ReportStatus{
		ReportID:    "rpt-001",
		Status:      "completed",
		Progress:    1.0,
		ReportType:  "fto",
		Format:      "pdf",
		CreatedAt:   now.Add(-10 * time.Minute),
		CompletedAt: now,
	}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports/rpt-001/status", nil)
	r.SetPathValue("report_id", "rpt-001")
	h.GetReportStatus(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ReportStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, float64(1.0), resp.Progress)
	assert.Contains(t, resp.DownloadURL, "rpt-001")
	assert.NotEmpty(t, resp.CompletedAt)
}

func TestGetReportStatus_InProgress(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("GetStatus", mock.Anything, "rpt-002").Return(&reporting.ReportStatus{
		ReportID:   "rpt-002",
		Status:     "processing",
		Progress:   0.45,
		ReportType: "infringement",
		Format:     "docx",
		CreatedAt:  time.Now(),
	}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports/rpt-002/status", nil)
	r.SetPathValue("report_id", "rpt-002")
	h.GetReportStatus(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp ReportStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "processing", resp.Status)
	assert.Equal(t, 0.45, resp.Progress)
	assert.Empty(t, resp.DownloadURL)
}

func TestGetReportStatus_NotFound(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("GetStatus", mock.Anything, "nonexistent").Return(
		nil, errors.NewAppError(errors.ErrCodeNotFound, "report not found"))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports/nonexistent/status", nil)
	r.SetPathValue("report_id", "nonexistent")
	h.GetReportStatus(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetReportStatus_EmptyID(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports//status", nil)
	r.SetPathValue("report_id", "")
	h.GetReportStatus(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- DownloadReport Tests ---

func TestDownloadReport_Success(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("GetStatus", mock.Anything, "rpt-dl-001").Return(&reporting.ReportStatus{
		ReportID: "rpt-dl-001",
		Status:   "completed",
	}, nil)

	fileContent := "fake-pdf-content-bytes"
	ftoSvc.On("Download", mock.Anything, "rpt-dl-001").Return(&reporting.ReportFile{
		Filename: "fto_report_2024.pdf",
		Format:   "pdf",
		Size:     int64(len(fileContent)),
		Content:  io.NopCloser(strings.NewReader(fileContent)),
	}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports/rpt-dl-001/download", nil)
	r.SetPathValue("report_id", "rpt-dl-001")
	h.DownloadReport(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "fto_report_2024.pdf")
	assert.Equal(t, fileContent, w.Body.String())
}

func TestDownloadReport_NotReady(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("GetStatus", mock.Anything, "rpt-pending").Return(&reporting.ReportStatus{
		ReportID: "rpt-pending",
		Status:   "processing",
	}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports/rpt-pending/download", nil)
	r.SetPathValue("report_id", "rpt-pending")
	h.DownloadReport(w, r)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "not ready")
}

func TestDownloadReport_ContentType_DOCX(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("GetStatus", mock.Anything, "rpt-docx").Return(&reporting.ReportStatus{
		ReportID: "rpt-docx",
		Status:   "completed",
	}, nil)

	ftoSvc.On("Download", mock.Anything, "rpt-docx").Return(&reporting.ReportFile{
		Filename: "report.docx",
		Format:   "docx",
		Size:     10,
		Content:  io.NopCloser(strings.NewReader("docx-bytes")),
	}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports/rpt-docx/download", nil)
	r.SetPathValue("report_id", "rpt-docx")
	h.DownloadReport(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "wordprocessingml")
}

// --- ListReports Tests ---

func TestListReports_Default(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("List", mock.Anything, mock.MatchedBy(func(f *reporting.ReportListFilter) bool {
		return f.PageSize == 20 && f.ReportType == "" && f.Status == ""
	})).Return(&reporting.ReportListResult{
		Reports: []reporting.ReportSummary{
			{ReportID: "rpt-1", ReportType: "fto", Status: "completed", Format: "pdf", Title: "FTO Report", CreatedAt: time.Now()},
		},
		TotalCount:    1,
		NextPageToken: "",
	}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports", nil)
	h.ListReports(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListReports_WithFilters(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("List", mock.Anything, mock.MatchedBy(func(f *reporting.ReportListFilter) bool {
		return f.ReportType == "fto" && f.Status == "completed" && f.PageSize == 10
	})).Return(&reporting.ReportListResult{
		Reports:    []reporting.ReportSummary{},
		TotalCount: 0,
	}, nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports?report_type=fto&status=completed&page_size=10", nil)
	h.ListReports(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListReports_InvalidPageSize(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports?page_size=200", nil)
	h.ListReports(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "page_size")
}

func TestListReports_InvalidDateFormat(t *testing.T) {
	h, _, _, _ := newTestReportHandler()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/reports?created_from=not-a-date", nil)
	h.ListReports(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "created_from")
}

// --- DeleteReport Tests ---

func TestDeleteReport_Success(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("Delete", mock.Anything, "rpt-del-001").Return(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/reports/rpt-del-001", nil)
	r.SetPathValue("report_id", "rpt-del-001")
	h.DeleteReport(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "deleted")
}

func TestDeleteReport_NotFound(t *testing.T) {
	h, ftoSvc, _, _ := newTestReportHandler()

	ftoSvc.On("Delete", mock.Anything, "nonexistent").Return(
		errors.NewAppError(errors.ErrCodeNotFound, "report not found"))

	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/reports/nonexistent", nil)
	r.SetPathValue("report_id", "nonexistent")
	h.DeleteReport(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Helper Function Tests ---

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"pdf", true},
		{"PDF", true},
		{"docx", true},
		{"DOCX", true},
		{"xlsx", true},
		{"html", false},
		{"csv", false},
		{"", false},
		{"json", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidFormat(tt.format))
		})
	}
}

func TestFormatToContentType(t *testing.T) {
	tests := []struct {
		format      string
		contentType string
	}{
		{"pdf", "application/pdf"},
		{"PDF", "application/pdf"},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{"xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"unknown", "application/octet-stream"},
		{"", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			assert.Equal(t, tt.contentType, formatToContentType(tt.format))
		})
	}
}

//Personal.AI order the ending

