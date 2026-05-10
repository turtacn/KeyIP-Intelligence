// Phase 11 - 接口层: HTTP Handler - CSP 违规报告收集端点
// 序号: 285
// 文件: internal/interfaces/http/handlers/csp_report_handler.go
// 功能定位: 接收浏览器发送的 Content-Security-Policy 违规报告
// 核心实现:
//   - 定义 CSPReportHandler 结构体，注入日志记录器
//   - 实现 PostCSPReport: 接收 POST /api/v1/csp-report，解码 JSON 报告
//   - 支持 CSP Level 1/2 格式 ({"csp-report": {...}})
//   - 支持 CSP Level 3 格式 ({"type": "csp-violation", "body": {...}})
//   - 结构化日志记录违规详情
//   - 返回 204 No Content
//   - 实现 RegisterRoutes: 注册 POST /api/v1/csp-report 路由
//
// 依赖关系:
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// CSPReportHandler handles Content-Security-Policy violation reports sent by
// browsers. It receives the reports via POST, logs the violation details in a
// structured format, and returns 204 No Content per the CSP specification.
type CSPReportHandler struct {
	logger logging.Logger
}

// NewCSPReportHandler creates a new CSPReportHandler.
// The logger is used for structured logging of CSP violation details.
// If nil, violations are silently discarded (still returns 204).
func NewCSPReportHandler(logger logging.Logger) *CSPReportHandler {
	return &CSPReportHandler{logger: logger}
}

// RegisterRoutes registers the CSP report endpoint on the given mux.
func (h *CSPReportHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/csp-report", h.PostCSPReport)
}

// CSPViolationReport represents a single CSP violation report sent by the
// browser. The fields follow the CSP Level 2+ specification.
type CSPViolationReport struct {
	DocumentURI        string `json:"document-uri"`
	Referrer           string `json:"referrer"`
	BlockedURI         string `json:"blocked-uri"`
	ViolatedDirective  string `json:"violated-directive"`
	EffectiveDirective string `json:"effective-directive"`
	OriginalPolicy     string `json:"original-policy"`
	Disposition        string `json:"disposition"`
	ScriptSample       string `json:"script-sample"`
	StatusCode         int    `json:"status-code"`
	SourceFile         string `json:"source-file"`
	LineNumber         int    `json:"line-number"`
	ColumnNumber       int    `json:"column-number"`
}

// cspReportBody is the outer envelope that browsers send. Two formats exist:
//
// CSP Level 1/2 (deprecated but still sent by some browsers):
//
//	{"csp-report": {"document-uri": "...", ...}}
//
// CSP Level 3 (Reporting API, modern browsers):
//
//	{"type": "csp-violation", "body": {"document-uri": "...", ...}}
type cspReportBody struct {
	CSPReport *CSPViolationReport `json:"csp-report"`
	Type      string              `json:"type"`
	Body      *CSPViolationReport `json:"body"`
}

// PostCSPReport handles POST /api/v1/csp-report.
// It decodes the CSP violation report, logs the violation details, and
// returns 204 No Content. Invalid or empty bodies are silently accepted
// and also return 204.
func (h *CSPReportHandler) PostCSPReport(w http.ResponseWriter, r *http.Request) {
	// Limit request body size to 10KB to prevent abuse.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<10)

	var body cspReportBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if h.logger != nil {
			h.logger.Warn("csp-report: invalid request body",
				logging.String("source_ip", r.RemoteAddr),
				logging.String("user_agent", r.UserAgent()),
				logging.Error(err),
			)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Extract the violation from whichever format the browser sent.
	var violation *CSPViolationReport
	switch {
	case body.CSPReport != nil:
		violation = body.CSPReport
	case body.Type == "csp-violation" && body.Body != nil:
		violation = body.Body
	}

	if violation == nil {
		if h.logger != nil {
			h.logger.Warn("csp-report: empty or unrecognized report format",
				logging.String("source_ip", r.RemoteAddr),
				logging.String("user_agent", r.UserAgent()),
			)
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Log the violation details in a structured format.
	if h.logger != nil {
		h.logger.Warn("csp-report: violation received",
			logging.String("blocked_uri", violation.BlockedURI),
			logging.String("document_uri", violation.DocumentURI),
			logging.String("violated_directive", violation.ViolatedDirective),
			logging.String("effective_directive", violation.EffectiveDirective),
			logging.String("original_policy", violation.OriginalPolicy),
			logging.String("disposition", violation.Disposition),
			logging.String("source_file", violation.SourceFile),
			logging.Int("line_number", violation.LineNumber),
			logging.Int("column_number", violation.ColumnNumber),
			logging.String("script_sample", violation.ScriptSample),
			logging.String("referrer", violation.Referrer),
			logging.Int("status_code", violation.StatusCode),
			logging.String("source_ip", r.RemoteAddr),
			logging.String("user_agent", r.UserAgent()),
		)
	}

	w.WriteHeader(http.StatusNoContent)
}

//Personal.AI order the ending
