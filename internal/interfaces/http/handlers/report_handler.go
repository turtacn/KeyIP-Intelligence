// Phase 11 - 接口层: HTTP Handler - 报告生成
// 序号: 270
// 文件: internal/interfaces/http/handlers/report_handler.go
// 功能定位: 实现报告生成与导出的 HTTP Handler，提供 FTO 报告、侵权分析报告、组合分析报告的生成与下载端点
// 核心实现:
//   - 定义 ReportHandler 结构体，注入 reporting 应用服务（FTO/侵权/组合）与 logger
//   - 定义 NewReportHandler(ftoSvc, infringeSvc, portfolioSvc, templateSvc, logger) *ReportHandler
//   - 实现 GenerateFTOReport: POST /api/v1/reports/fto，接收目标分子 SMILES + 法域 + 选项 → 调用 ftoSvc.Generate → 返回报告 ID 与状态
//   - 实现 GenerateInfringementReport: POST /api/v1/reports/infringement，接收专利号 + 目标分子 → 调用 infringeSvc.Generate → 返回报告 ID
//   - 实现 GeneratePortfolioReport: POST /api/v1/reports/portfolio，接收组合 ID + 报告类型 → 调用 portfolioSvc.Generate → 返回报告 ID
//   - 实现 GetReportStatus: GET /api/v1/reports/:report_id/status，查询报告生成进度
//   - 实现 DownloadReport: GET /api/v1/reports/:report_id/download，返回 PDF/DOCX 文件流，设置 Content-Disposition
//   - 实现 ListReports: GET /api/v1/reports，支持按类型/日期/状态过滤 + 分页
//   - 实现 DeleteReport: DELETE /api/v1/reports/:report_id，软删除报告
//   - 实现 RegisterRoutes(router)，注册所有报告相关路由
// 业务逻辑:
//   - 报告生成为异步操作，POST 返回 202 Accepted + 报告 ID
//   - 下载时检查报告状态，未完成返回 409 Conflict
//   - 支持 format 查询参数选择导出格式（pdf/docx/xlsx）
//   - 报告保留策略: 默认 90 天自动清理
// 依赖关系:
//   - 依赖: internal/application/reporting/fto_report.go, infringement_report.go, portfolio_report.go, template.go
//   - 被依赖: internal/interfaces/http/router.go
// 测试要求: 全部端点正常/异常路径、异步状态查询、文件下载 Content-Type、权限校验
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	pkghttp "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ReportHandler handles HTTP requests for report generation and management.
type ReportHandler struct {
	ftoSvc       reporting.FTOReportService
	infringeSvc  reporting.InfringementReportService
	portfolioSvc reporting.PortfolioReportService
	templateSvc  reporting.TemplateService
	logger       logging.Logger
}

// NewReportHandler creates a new ReportHandler with injected dependencies.
func NewReportHandler(
	ftoSvc reporting.FTOReportService,
	infringeSvc reporting.InfringementReportService,
	portfolioSvc reporting.PortfolioReportService,
	templateSvc reporting.TemplateService,
	logger logging.Logger,
) *ReportHandler {
	return &ReportHandler{
		ftoSvc:       ftoSvc,
		infringeSvc:  infringeSvc,
		portfolioSvc: portfolioSvc,
		templateSvc:  templateSvc,
		logger:       logger,
	}
}

// --- Request / Response DTOs ---

// GenerateFTOReportRequest represents the request body for FTO report generation.
type GenerateFTOReportRequest struct {
	TargetSMILES   string   `json:"target_smiles"`
	Jurisdiction   string   `json:"jurisdiction"`
	IncludeExpired bool     `json:"include_expired"`
	Depth          int      `json:"depth"`
	Format         string   `json:"format"`
	Languages      []string `json:"languages"`
}

// GenerateInfringementReportRequest represents the request body for infringement report generation.
type GenerateInfringementReportRequest struct {
	PatentNumber  string   `json:"patent_number"`
	TargetSMILES  []string `json:"target_smiles"`
	AnalysisDepth string   `json:"analysis_depth"`
	Format        string   `json:"format"`
}

// GeneratePortfolioReportRequest represents the request body for portfolio report generation.
type GeneratePortfolioReportRequest struct {
	PortfolioID string `json:"portfolio_id"`
	ReportType  string `json:"report_type"`
	Format      string `json:"format"`
	IncludeCharts bool `json:"include_charts"`
}

// ReportStatusResponse represents the status of a report generation task.
type ReportStatusResponse struct {
	ReportID    string  `json:"report_id"`
	Status      string  `json:"status"`
	Progress    float64 `json:"progress"`
	ReportType  string  `json:"report_type"`
	Format      string  `json:"format"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt string  `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
	DownloadURL string  `json:"download_url,omitempty"`
}

// ReportListItem represents a single report in a list response.
type ReportListItem struct {
	ReportID   string `json:"report_id"`
	ReportType string `json:"report_type"`
	Status     string `json:"status"`
	Format     string `json:"format"`
	Title      string `json:"title"`
	CreatedAt  string `json:"created_at"`
	FileSize   int64  `json:"file_size,omitempty"`
}

// RegisterRoutes registers all report-related routes on the given router.
func (h *ReportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/reports/fto", h.GenerateFTOReport)
	mux.HandleFunc("POST /api/v1/reports/infringement", h.GenerateInfringementReport)
	mux.HandleFunc("POST /api/v1/reports/portfolio", h.GeneratePortfolioReport)
	mux.HandleFunc("GET /api/v1/reports/{report_id}/status", h.GetReportStatus)
	mux.HandleFunc("GET /api/v1/reports/{report_id}/download", h.DownloadReport)
	mux.HandleFunc("GET /api/v1/reports", h.ListReports)
	mux.HandleFunc("DELETE /api/v1/reports/{report_id}", h.DeleteReport)
}

// GenerateFTOReport handles POST /api/v1/reports/fto.
// It initiates asynchronous FTO report generation and returns 202 Accepted with a report ID.
func (h *ReportHandler) GenerateFTOReport(w http.ResponseWriter, r *http.Request) {
	var req GenerateFTOReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode FTO report request", logging.Err(err))
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid request body")
		return
	}

	if req.TargetSMILES == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "target_smiles is required")
		return
	}
	if req.Jurisdiction == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "jurisdiction is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if !isValidFormat(req.Format) {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "format must be one of: pdf, docx, xlsx")
		return
	}

	// Map HTTP request to service request
	svcReq := &reporting.FTOReportRequest{
		TargetMolecules: []reporting.MoleculeInput{
			{Format: "smiles", Value: req.TargetSMILES},
		},
		Jurisdictions:       []string{req.Jurisdiction},
		AnalysisDepth:       mapDepth(req.Depth),
		IncludeDesignAround: req.IncludeExpired,
		Language:            mapLanguage(req.Languages),
	}

	result, err := h.ftoSvc.Generate(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("failed to initiate FTO report generation", logging.Err(err))
		writeReportAppError(w, err)
		return
	}

	writeReportJSON(w, http.StatusAccepted, map[string]interface{}{
		"report_id": result.ReportID,
		"status":    result.Status,
		"message":   "FTO report generation initiated",
	})
}

// GenerateInfringementReport handles POST /api/v1/reports/infringement.
func (h *ReportHandler) GenerateInfringementReport(w http.ResponseWriter, r *http.Request) {
	var req GenerateInfringementReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode infringement report request", logging.Err(err))
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid request body")
		return
	}

	if req.PatentNumber == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "patent_number is required")
		return
	}
	if len(req.TargetSMILES) == 0 {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "target_smiles is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if !isValidFormat(req.Format) {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "format must be one of: pdf, docx, xlsx")
		return
	}

	// Map HTTP request to service request
	moleculeInputs := make([]reporting.MoleculeInput, len(req.TargetSMILES))
	for i, smiles := range req.TargetSMILES {
		moleculeInputs[i] = reporting.MoleculeInput{Format: "smiles", Value: smiles}
	}

	svcReq := &reporting.InfringementReportRequest{
		OwnedPatentNumbers: []string{req.PatentNumber},
		SuspectedMolecules: moleculeInputs,
		AnalysisMode:       mapAnalysisMode(req.AnalysisDepth),
	}

	result, err := h.infringeSvc.Generate(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("failed to initiate infringement report generation", logging.Err(err))
		writeReportAppError(w, err)
		return
	}

	writeReportJSON(w, http.StatusAccepted, map[string]interface{}{
		"report_id": result.ReportID,
		"status":    result.Status,
		"message":   "Infringement report generation initiated",
	})
}

// GeneratePortfolioReport handles POST /api/v1/reports/portfolio.
func (h *ReportHandler) GeneratePortfolioReport(w http.ResponseWriter, r *http.Request) {
	var req GeneratePortfolioReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode portfolio report request", logging.Err(err))
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid request body")
		return
	}

	if req.PortfolioID == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "portfolio_id is required")
		return
	}
	if req.ReportType == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_type is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if !isValidFormat(req.Format) {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "format must be one of: pdf, docx, xlsx")
		return
	}

	// Map HTTP request to service request
	svcReq := &reporting.PortfolioReportRequest{
		PortfolioID:     req.PortfolioID,
		IncludeSections: mapReportSections(req.ReportType),
		OutputFormat:    mapOutputFormat(req.Format),
	}

	result, err := h.portfolioSvc.GenerateFullReport(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("failed to initiate portfolio report generation", logging.Err(err))
		writeReportAppError(w, err)
		return
	}

	writeReportJSON(w, http.StatusAccepted, map[string]interface{}{
		"report_id": result.ReportID,
		"status":    result.Status,
		"message":   "Portfolio report generation initiated",
	})
}

// GetReportStatus handles GET /api/v1/reports/{report_id}/status.
func (h *ReportHandler) GetReportStatus(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	if reportID == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_id is required")
		return
	}

	status, err := h.ftoSvc.GetStatus(r.Context(), reportID)
	if err != nil {
		h.logger.Error("failed to get report status", logging.Err(err), logging.String("report_id", reportID))
		writeReportAppError(w, err)
		return
	}

	resp := ReportStatusResponse{
		ReportID: status.ReportID,
		Status:   string(status.Status),
		Progress: float64(status.ProgressPct),
	}
	if status.Message != "" {
		resp.Error = status.Message
	}
	if string(status.Status) == string(reporting.StatusCompleted) {
		resp.DownloadURL = fmt.Sprintf("/api/v1/reports/%s/download", reportID)
	}

	writeReportJSON(w, http.StatusOK, resp)
}

// DownloadReport handles GET /api/v1/reports/{report_id}/download.
// Returns the generated report file with appropriate Content-Type and Content-Disposition headers.
func (h *ReportHandler) DownloadReport(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	if reportID == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_id is required")
		return
	}

	// Check report status first
	status, err := h.ftoSvc.GetStatus(r.Context(), reportID)
	if err != nil {
		h.logger.Error("failed to get report status for download", logging.Err(err), logging.String("report_id", reportID))
		writeReportAppError(w, err)
		return
	}

	if status.Status != reporting.StatusCompleted {
		writeReportError(w, http.StatusConflict, errors.ErrCodeConflict,
			fmt.Sprintf("report is not ready for download, current status: %s", status.Status))
		return
	}

	// Get report file content - use PDF format by default
	formatParam := r.URL.Query().Get("format")
	reportFormat := reporting.FormatPDF
	if formatParam == "docx" {
		reportFormat = reporting.FormatDOCX
	}

	reader, err := h.ftoSvc.GetReport(r.Context(), reportID, reportFormat)
	if err != nil {
		h.logger.Error("failed to download report", logging.Err(err), logging.String("report_id", reportID))
		writeReportAppError(w, err)
		return
	}
	defer reader.Close()

	// Set response headers
	contentType := formatToContentType(string(reportFormat))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"report_%s.%s\"", reportID, strings.ToLower(string(reportFormat))))

	w.WriteHeader(http.StatusOK)

	// Stream file content
	if _, err := io.Copy(w, reader); err != nil {
		h.logger.Error("failed to write report content", logging.Err(err), logging.String("report_id", reportID))
	}
}

// ListReports handles GET /api/v1/reports.
// Supports filtering by type, status, date range, and pagination.
func (h *ReportHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	pageSize, _ := strconv.Atoi(query.Get("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "page_size must be between 1 and 100")
		return
	}

	page, _ := strconv.Atoi(query.Get("page"))
	if page <= 0 {
		page = 1
	}

	// Build FTO report filter
	filter := &reporting.FTOReportFilter{}

	if statusStr := query.Get("status"); statusStr != "" {
		filter.Status = []reporting.ReportStatus{reporting.ReportStatus(statusStr)}
	}

	if from := query.Get("created_from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid created_from format, use RFC3339")
			return
		}
		filter.DateFrom = &t
	}
	if to := query.Get("created_to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid created_to format, use RFC3339")
			return
		}
		filter.DateTo = &t
	}

	pagination := &pkghttp.Pagination{
		Page:     page,
		PageSize: pageSize,
	}

	result, err := h.ftoSvc.ListReports(r.Context(), filter, pagination)
	if err != nil {
		h.logger.Error("failed to list reports", logging.Err(err))
		writeReportAppError(w, err)
		return
	}

	items := make([]ReportListItem, len(result.Items))
	for i, rpt := range result.Items {
		items[i] = ReportListItem{
			ReportID:   rpt.ReportID,
			ReportType: "fto",
			Status:     string(rpt.Status),
			Title:      rpt.Title,
			CreatedAt:  rpt.CreatedAt.Format(time.RFC3339),
		}
	}

	writeReportJSON(w, http.StatusOK, pkghttp.PageResponse[ReportListItem]{
		Items:    items,
		Total:    int64(result.Pagination.Total),
		Page:     result.Pagination.Page,
		PageSize: result.Pagination.PageSize,
	})
}

// DeleteReport handles DELETE /api/v1/reports/{report_id}.
func (h *ReportHandler) DeleteReport(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	if reportID == "" {
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_id is required")
		return
	}

	if err := h.ftoSvc.DeleteReport(r.Context(), reportID); err != nil {
		h.logger.Error("failed to delete report", logging.Err(err), logging.String("report_id", reportID))
		writeReportAppError(w, err)
		return
	}

	writeReportJSON(w, http.StatusOK, map[string]interface{}{
		"message": "report deleted successfully",
	})
}

// --- Helper functions ---

// isValidFormat checks if the given format string is a supported export format.
func isValidFormat(format string) bool {
	switch strings.ToLower(format) {
	case "pdf", "docx", "xlsx":
		return true
	default:
		return false
	}
}

// formatToContentType maps a report format to its HTTP Content-Type.
func formatToContentType(format string) string {
	switch strings.ToLower(format) {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	default:
		return "application/octet-stream"
	}
}

// mapDepth maps HTTP request depth to service AnalysisDepth
func mapDepth(depth int) reporting.AnalysisDepth {
	switch {
	case depth <= 1:
		return reporting.DepthQuick
	case depth == 2:
		return reporting.DepthStandard
	default:
		return reporting.DepthComprehensive
	}
}

// mapLanguage maps HTTP request languages to service ReportLanguage
func mapLanguage(languages []string) reporting.ReportLanguage {
	if len(languages) > 0 {
		switch strings.ToUpper(languages[0]) {
		case "ZH", "CN", "CHINESE":
			return reporting.LangZH
		case "EN", "ENGLISH":
			return reporting.LangEN
		}
	}
	return reporting.LangEN
}

// mapAnalysisMode maps HTTP request analysis depth to service InfringementAnalysisMode
func mapAnalysisMode(depth string) reporting.InfringementAnalysisMode {
	switch strings.ToLower(depth) {
	case "literal":
		return reporting.ModeLiteral
	case "equivalents":
		return reporting.ModeEquivalents
	default:
		return reporting.ModeComprehensive
	}
}

// mapReportSections maps report type to sections
func mapReportSections(reportType string) []reporting.ReportSection {
	// Return all sections for now - can be customized based on reportType
	return []reporting.ReportSection{
		reporting.SectionOverview,
		reporting.SectionValueDistribution,
		reporting.SectionTechCoverage,
		reporting.SectionCompetitiveComparison,
		reporting.SectionLayoutRecommendation,
	}
}

// mapOutputFormat maps HTTP format string to service ExportFormat
func mapOutputFormat(format string) reporting.ExportFormat {
	switch strings.ToLower(format) {
	case "pdf":
		return reporting.FormatPortfolioPDF
	case "docx":
		return reporting.FormatPortfolioDOCX
	case "pptx":
		return reporting.FormatPortfolioPPTX
	default:
		return reporting.FormatPortfolioPDF
	}
}

// writeReportJSON writes a JSON response with the given status code.
func writeReportJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// ReportErrorResponse is the standard error response body for report handler.
type ReportErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeReportError writes a structured error response.
func writeReportError(w http.ResponseWriter, statusCode int, code errors.ErrorCode, message string) {
	resp := ReportErrorResponse{
		Code:    string(code),
		Message: message,
	}
	writeReportJSON(w, statusCode, resp)
}

// writeReportAppError maps application-level errors to HTTP status codes.
func writeReportAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.IsNotFound(err):
		writeReportError(w, http.StatusNotFound, errors.ErrCodeNotFound, err.Error())
	case errors.IsValidation(err):
		writeReportError(w, http.StatusBadRequest, errors.ErrCodeValidation, err.Error())
	case errors.IsConflict(err):
		writeReportError(w, http.StatusConflict, errors.ErrCodeConflict, err.Error())
	case errors.IsUnauthorized(err):
		writeReportError(w, http.StatusUnauthorized, errors.ErrCodeUnauthorized, err.Error())
	case errors.IsForbidden(err):
		writeReportError(w, http.StatusForbidden, errors.ErrCodeForbidden, err.Error())
	default:
		writeReportError(w, http.StatusInternalServerError, errors.ErrCodeInternal, "internal server error")
	}
}

// Router-compatible aliases for ReportHandler

// List is an alias for ListReports.
func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
	h.ListReports(w, r)
}

// Get is an alias for GetReportStatus.
func (h *ReportHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.GetReportStatus(w, r)
}

// Delete is an alias for DeleteReport.
func (h *ReportHandler) Delete(w http.ResponseWriter, r *http.Request) {
	h.DeleteReport(w, r)
}

// Download is an alias for DownloadReport.
func (h *ReportHandler) Download(w http.ResponseWriter, r *http.Request) {
	h.DownloadReport(w, r)
}

// Generate handles report generation (alias for GenerateFTOReport).
func (h *ReportHandler) Generate(w http.ResponseWriter, r *http.Request) {
	h.GenerateFTOReport(w, r)
}

// ListTemplates handles template listing (placeholder).
func (h *ReportHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	writeReportJSON(w, http.StatusNotImplemented, map[string]string{"message": "list templates not yet implemented"})
}

// GetTemplate handles template retrieval (placeholder).
func (h *ReportHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	writeReportJSON(w, http.StatusNotImplemented, map[string]string{"message": "get template not yet implemented"})
}

// GenerateReport handles unified report generation (placeholder).
func (h *ReportHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	writeReportJSON(w, http.StatusNotImplemented, map[string]string{"message": "unified report generation not yet implemented"})
}

// GetReport handles report metadata retrieval (alias for GetReportStatus).
func (h *ReportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	h.GetReportStatus(w, r)
}

// CreateTemplate handles template creation (placeholder).
func (h *ReportHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	writeReportJSON(w, http.StatusNotImplemented, map[string]string{"message": "create template not yet implemented"})
}

// UpdateTemplate handles template update (placeholder).
func (h *ReportHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	writeReportJSON(w, http.StatusNotImplemented, map[string]string{"message": "update template not yet implemented"})
}

// DeleteTemplate handles template deletion (placeholder).
func (h *ReportHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	writeReportJSON(w, http.StatusNotImplemented, map[string]string{"message": "delete template not yet implemented"})
}

//Personal.AI order the ending
