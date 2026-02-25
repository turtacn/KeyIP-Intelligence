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
	"fmt"
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
	if err := decodeJSON(r, &req); err != nil {
		h.logger.Error("failed to decode FTO report request", "error", err)
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid request body")
		return
	}

	if req.TargetSMILES == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "target_smiles is required")
		return
	}
	if req.Jurisdiction == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "jurisdiction is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if !isValidFormat(req.Format) {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "format must be one of: pdf, docx, xlsx")
		return
	}

	svcReq := &reporting.FTOReportRequest{
		TargetSMILES:   req.TargetSMILES,
		Jurisdiction:   req.Jurisdiction,
		IncludeExpired: req.IncludeExpired,
		Depth:          req.Depth,
		Format:         req.Format,
		Languages:      req.Languages,
	}

	result, err := h.ftoSvc.Generate(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("failed to initiate FTO report generation", "error", err)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"report_id": result.ReportID,
		"status":    result.Status,
		"message":   "FTO report generation initiated",
	})
}

// GenerateInfringementReport handles POST /api/v1/reports/infringement.
func (h *ReportHandler) GenerateInfringementReport(w http.ResponseWriter, r *http.Request) {
	var req GenerateInfringementReportRequest
	if err := decodeJSON(r, &req); err != nil {
		h.logger.Error("failed to decode infringement report request", "error", err)
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid request body")
		return
	}

	if req.PatentNumber == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "patent_number is required")
		return
	}
	if len(req.TargetSMILES) == 0 {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "target_smiles is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if !isValidFormat(req.Format) {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "format must be one of: pdf, docx, xlsx")
		return
	}

	svcReq := &reporting.InfringementReportRequest{
		PatentNumber:  req.PatentNumber,
		TargetSMILES:  req.TargetSMILES,
		AnalysisDepth: req.AnalysisDepth,
		Format:        req.Format,
	}

	result, err := h.infringeSvc.Generate(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("failed to initiate infringement report generation", "error", err)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"report_id": result.ReportID,
		"status":    result.Status,
		"message":   "Infringement report generation initiated",
	})
}

// GeneratePortfolioReport handles POST /api/v1/reports/portfolio.
func (h *ReportHandler) GeneratePortfolioReport(w http.ResponseWriter, r *http.Request) {
	var req GeneratePortfolioReportRequest
	if err := decodeJSON(r, &req); err != nil {
		h.logger.Error("failed to decode portfolio report request", "error", err)
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid request body")
		return
	}

	if req.PortfolioID == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "portfolio_id is required")
		return
	}
	if req.ReportType == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_type is required")
		return
	}
	if req.Format == "" {
		req.Format = "pdf"
	}
	if !isValidFormat(req.Format) {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "format must be one of: pdf, docx, xlsx")
		return
	}

	svcReq := &reporting.PortfolioReportRequest{
		PortfolioID:   req.PortfolioID,
		ReportType:    req.ReportType,
		Format:        req.Format,
		IncludeCharts: req.IncludeCharts,
	}

	result, err := h.portfolioSvc.Generate(r.Context(), svcReq)
	if err != nil {
		h.logger.Error("failed to initiate portfolio report generation", "error", err)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"report_id": result.ReportID,
		"status":    result.Status,
		"message":   "Portfolio report generation initiated",
	})
}

// GetReportStatus handles GET /api/v1/reports/{report_id}/status.
func (h *ReportHandler) GetReportStatus(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_id is required")
		return
	}

	status, err := h.ftoSvc.GetStatus(r.Context(), reportID)
	if err != nil {
		h.logger.Error("failed to get report status", "error", err, "report_id", reportID)
		writeAppError(w, err)
		return
	}

	resp := ReportStatusResponse{
		ReportID:   status.ReportID,
		Status:     status.Status,
		Progress:   status.Progress,
		ReportType: status.ReportType,
		Format:     status.Format,
		CreatedAt:  status.CreatedAt.Format(time.RFC3339),
	}
	if !status.CompletedAt.IsZero() {
		resp.CompletedAt = status.CompletedAt.Format(time.RFC3339)
	}
	if status.Error != "" {
		resp.Error = status.Error
	}
	if status.Status == "completed" {
		resp.DownloadURL = fmt.Sprintf("/api/v1/reports/%s/download", reportID)
	}

	writeJSON(w, http.StatusOK, resp)
}

// DownloadReport handles GET /api/v1/reports/{report_id}/download.
// Returns the generated report file with appropriate Content-Type and Content-Disposition headers.
func (h *ReportHandler) DownloadReport(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_id is required")
		return
	}

	// Check report status first
	status, err := h.ftoSvc.GetStatus(r.Context(), reportID)
	if err != nil {
		h.logger.Error("failed to get report status for download", "error", err, "report_id", reportID)
		writeAppError(w, err)
		return
	}

	if status.Status != "completed" {
		writeError(w, http.StatusConflict, errors.ErrCodeConflict,
			fmt.Sprintf("report is not ready for download, current status: %s", status.Status))
		return
	}

	// Get report file content
	file, err := h.ftoSvc.Download(r.Context(), reportID)
	if err != nil {
		h.logger.Error("failed to download report", "error", err, "report_id", reportID)
		writeAppError(w, err)
		return
	}
	defer file.Content.Close()

	// Set response headers
	contentType := formatToContentType(file.Format)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s\"", file.Filename))
	if file.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	}

	w.WriteHeader(http.StatusOK)

	// Stream file content
	buf := make([]byte, 32*1024)
	for {
		n, readErr := file.Content.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				h.logger.Error("failed to write report content", "error", writeErr, "report_id", reportID)
				return
			}
		}
		if readErr != nil {
			break
		}
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
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "page_size must be between 1 and 100")
		return
	}

	filter := &reporting.ReportListFilter{
		ReportType: query.Get("report_type"),
		Status:     query.Get("status"),
		PageToken:  query.Get("page_token"),
		PageSize:   pageSize,
	}

	if from := query.Get("created_from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid created_from format, use RFC3339")
			return
		}
		filter.CreatedFrom = &t
	}
	if to := query.Get("created_to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "invalid created_to format, use RFC3339")
			return
		}
		filter.CreatedTo = &t
	}

	result, err := h.ftoSvc.List(r.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list reports", "error", err)
		writeAppError(w, err)
		return
	}

	items := make([]ReportListItem, len(result.Reports))
	for i, rpt := range result.Reports {
		items[i] = ReportListItem{
			ReportID:   rpt.ReportID,
			ReportType: rpt.ReportType,
			Status:     rpt.Status,
			Format:     rpt.Format,
			Title:      rpt.Title,
			CreatedAt:  rpt.CreatedAt.Format(time.RFC3339),
			FileSize:   rpt.FileSize,
		}
	}

	writeJSON(w, http.StatusOK, pkghttp.PaginatedResponse{
		Items:         items,
		NextPageToken: result.NextPageToken,
		TotalCount:    result.TotalCount,
	})
}

// DeleteReport handles DELETE /api/v1/reports/{report_id}.
func (h *ReportHandler) DeleteReport(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	if reportID == "" {
		writeError(w, http.StatusBadRequest, errors.ErrCodeValidation, "report_id is required")
		return
	}

	if err := h.ftoSvc.Delete(r.Context(), reportID); err != nil {
		h.logger.Error("failed to delete report", "error", err, "report_id", reportID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
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

//Personal.AI order the ending
