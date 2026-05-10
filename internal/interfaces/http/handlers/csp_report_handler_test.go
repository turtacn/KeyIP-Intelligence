// Phase 11 - 接口层: HTTP Handler - CSP 违规报告收集端点单元测试
// 序号: 286
// 文件: internal/interfaces/http/handlers/csp_report_handler_test.go
// 测试用例:
//   - TestCSPReportHandler_Level1Format: CSP Level 1 格式 (csp-report)
//   - TestCSPReportHandler_Level3Format: CSP Level 3 格式 (type/body)
//   - TestCSPReportHandler_InvalidBody: 无效 JSON 返回 204
//   - TestCSPReportHandler_EmptyBody: 空 body 返回 204
//   - TestCSPReportHandler_Returns204: 始终返回 204 No Content
//   - TestCSPReportHandler_NilLogger: logger 为 nil 不 panic
//   - TestCSPReportHandler_WithLogger: 有 logger 正常记录
//   - TestCSPReportHandler_TooLargeBody: 超大 body 被截断返回 204
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// nopLoggerForTest is a minimal no-op logger for testing.
type testLogger struct {
	logging.Logger
	lastMsg  string
	lastFields []logging.Field
}

func (l *testLogger) Warn(msg string, fields ...logging.Field) {
	l.lastMsg = msg
	l.lastFields = fields
}

func TestCSPReportHandler_Level1Format(t *testing.T) {
	logger := &testLogger{}
	h := NewCSPReportHandler(logger)

	report := CSPViolationReport{
		DocumentURI:        "https://example.com/page",
		Referrer:           "https://example.com/other",
		BlockedURI:         "https://evil.com/script.js",
		ViolatedDirective:  "script-src 'self'",
		EffectiveDirective: "script-src",
		OriginalPolicy:     "default-src 'self'; script-src 'self'",
		Disposition:        "enforce",
		ScriptSample:       "alert('xss')",
		StatusCode:         200,
		SourceFile:         "https://example.com/page",
		LineNumber:         42,
		ColumnNumber:       10,
	}

	body := map[string]interface{}{
		"csp-report": report,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/csp-report")
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	h.PostCSPReport(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())

	// Verify structured logging
	assert.Contains(t, logger.lastMsg, "csp-report")
	assert.NotEmpty(t, logger.lastFields)
}

func TestCSPReportHandler_Level3Format(t *testing.T) {
	logger := &testLogger{}
	h := NewCSPReportHandler(logger)

	report := CSPViolationReport{
		DocumentURI:        "https://example.com/app",
		BlockedURI:         "https://evil.com/beacon",
		ViolatedDirective:  "connect-src 'self'",
		EffectiveDirective: "connect-src",
		OriginalPolicy:     "default-src 'self'; connect-src 'self'",
		Disposition:        "report",
		StatusCode:         0,
	}

	body := map[string]interface{}{
		"type": "csp-violation",
		"body": report,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/reports+json")
	rec := httptest.NewRecorder()

	h.PostCSPReport(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Contains(t, logger.lastMsg, "csp-report")
}

func TestCSPReportHandler_InvalidBody(t *testing.T) {
	logger := &testLogger{}
	h := NewCSPReportHandler(logger)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader("not-json-at-all"))
	req.RemoteAddr = "10.0.0.1:54321"
	rec := httptest.NewRecorder()

	h.PostCSPReport(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Contains(t, logger.lastMsg, "invalid request body")
}

func TestCSPReportHandler_EmptyBody(t *testing.T) {
	logger := &testLogger{}
	h := NewCSPReportHandler(logger)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(""))
	rec := httptest.NewRecorder()

	h.PostCSPReport(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCSPReportHandler_Returns204(t *testing.T) {
	h := NewCSPReportHandler(nil)

	// Test with valid CSP report
	report := CSPViolationReport{
		DocumentURI:       "https://example.com/",
		BlockedURI:        "https://evil.com/",
		ViolatedDirective: "script-src 'self'",
	}
	body := map[string]interface{}{"csp-report": report}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(string(bodyJSON)))
	rec := httptest.NewRecorder()

	h.PostCSPReport(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Test with invalid body
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader("{{{"))
	rec2 := httptest.NewRecorder()

	h.PostCSPReport(rec2, req2)
	assert.Equal(t, http.StatusNoContent, rec2.Code)

	// Test with empty body
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", nil)
	rec3 := httptest.NewRecorder()

	h.PostCSPReport(rec3, req3)
	assert.Equal(t, http.StatusNoContent, rec3.Code)
}

func TestCSPReportHandler_NilLogger(t *testing.T) {
	h := NewCSPReportHandler(nil)

	report := CSPViolationReport{
		DocumentURI: "https://example.com/",
		BlockedURI:  "https://evil.com/",
	}
	body := map[string]interface{}{"csp-report": report}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(string(bodyJSON)))
	rec := httptest.NewRecorder()

	// Should not panic
	assert.NotPanics(t, func() {
		h.PostCSPReport(rec, req)
	})
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCSPReportHandler_WithLogger(t *testing.T) {
	logger := &testLogger{}
	h := NewCSPReportHandler(logger)

	report := CSPViolationReport{
		DocumentURI:        "https://example.com/page",
		BlockedURI:         "https://evil.com/script.js",
		ViolatedDirective:  "script-src 'self'",
		EffectiveDirective: "script-src",
		OriginalPolicy:     "default-src 'self'",
		Disposition:        "enforce",
		SourceFile:         "https://example.com/page",
		LineNumber:         15,
		ColumnNumber:       3,
		StatusCode:         200,
	}

	body := map[string]interface{}{"csp-report": report}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(string(bodyJSON)))
	req.RemoteAddr = "10.0.0.1:8080"
	req.Header.Set("User-Agent", "TestBrowser/1.0")
	rec := httptest.NewRecorder()

	h.PostCSPReport(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Contains(t, logger.lastMsg, "violation received")
}

func TestCSPReportHandler_TooLargeBody(t *testing.T) {
	logger := &testLogger{}
	h := NewCSPReportHandler(logger)

	// Create a body larger than 10KB limit
	largeBody := strings.Repeat("a", 11<<10)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(largeBody))
	rec := httptest.NewRecorder()

	h.PostCSPReport(rec, req)

	// Should be handled gracefully (body too large, decode fails, still 204)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestCSPReportHandler_RegisterRoutes(t *testing.T) {
	h := NewCSPReportHandler(nil)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// Verify the route is registered by sending a valid request
	report := CSPViolationReport{
		DocumentURI: "https://example.com/",
		BlockedURI:  "https://evil.com/",
	}
	body := map[string]interface{}{"csp-report": report}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/csp-report", strings.NewReader(string(bodyJSON)))
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

// Personal.AI order the ending
