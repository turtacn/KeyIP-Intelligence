// Real tests for report HTTP handler.
// Tests request parsing, validation, error responses, and successful responses.

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

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/reporting"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	commontypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// mockFTOReportService implements reporting.FTOReportService for testing.
type mockFTOReportService struct {
	generateFn     func(context.Context, *reporting.FTOReportRequest) (*reporting.FTOReportResponse, error)
	getStatusFn    func(context.Context, string) (*reporting.ReportStatusInfo, error)
	getReportFn    func(context.Context, string, reporting.ReportFormat) (io.ReadCloser, error)
	listReportsFn  func(context.Context, *reporting.FTOReportFilter, *commontypes.Pagination) (*commontypes.PaginatedResult[reporting.FTOReportSummary], error)
	deleteReportFn func(context.Context, string) error
}

func (m *mockFTOReportService) Generate(ctx context.Context, req *reporting.FTOReportRequest) (*reporting.FTOReportResponse, error) {
	return m.generateFn(ctx, req)
}
func (m *mockFTOReportService) GetStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	return m.getStatusFn(ctx, reportID)
}
func (m *mockFTOReportService) GetReport(ctx context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
	return m.getReportFn(ctx, reportID, format)
}
func (m *mockFTOReportService) ListReports(ctx context.Context, filter *reporting.FTOReportFilter, page *commontypes.Pagination) (*commontypes.PaginatedResult[reporting.FTOReportSummary], error) {
	return m.listReportsFn(ctx, filter, page)
}
func (m *mockFTOReportService) DeleteReport(ctx context.Context, reportID string) error {
	return m.deleteReportFn(ctx, reportID)
}

// mockInfringementReportService implements reporting.InfringementReportService for testing.
type mockInfringementReportService struct {
	generateFn     func(context.Context, *reporting.InfringementReportRequest) (*reporting.InfringementReportResponse, error)
	getStatusFn    func(context.Context, string) (*reporting.ReportStatusInfo, error)
	getReportFn    func(context.Context, string, reporting.ReportFormat) (io.ReadCloser, error)
	listReportsFn  func(context.Context, *reporting.InfringementReportFilter, *commontypes.Pagination) (*commontypes.PaginatedResult[reporting.InfringementReportSummary], error)
	deleteReportFn func(context.Context, string) error
}

func (m *mockInfringementReportService) Generate(ctx context.Context, req *reporting.InfringementReportRequest) (*reporting.InfringementReportResponse, error) {
	return m.generateFn(ctx, req)
}
func (m *mockInfringementReportService) GetStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	return m.getStatusFn(ctx, reportID)
}
func (m *mockInfringementReportService) GetReport(ctx context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
	return m.getReportFn(ctx, reportID, format)
}
func (m *mockInfringementReportService) ListReports(ctx context.Context, filter *reporting.InfringementReportFilter, page *commontypes.Pagination) (*commontypes.PaginatedResult[reporting.InfringementReportSummary], error) {
	return m.listReportsFn(ctx, filter, page)
}
func (m *mockInfringementReportService) DeleteReport(ctx context.Context, reportID string) error {
	return m.deleteReportFn(ctx, reportID)
}

// mockPortfolioReportService implements reporting.PortfolioReportService for testing.
type mockPortfolioReportService struct {
	generateFullReportFn        func(context.Context, *reporting.PortfolioReportRequest) (*reporting.PortfolioReportResult, error)
	generateSummaryReportFn     func(context.Context, *reporting.PortfolioSummaryRequest) (*reporting.PortfolioReportResult, error)
	generateGapReportFn         func(context.Context, *reporting.GapReportRequest) (*reporting.PortfolioReportResult, error)
	generateCompetitiveReportFn func(context.Context, *reporting.CompetitiveReportRequest) (*reporting.PortfolioReportResult, error)
	getReportStatusFn           func(context.Context, string) (*reporting.ReportStatusInfo, error)
	listReportsFn               func(context.Context, string, *reporting.ListReportOptions) (*commontypes.PaginatedResult[reporting.ReportMeta], error)
	exportReportFn              func(context.Context, string, reporting.ExportFormat) ([]byte, error)
}

func (m *mockPortfolioReportService) GenerateFullReport(ctx context.Context, req *reporting.PortfolioReportRequest) (*reporting.PortfolioReportResult, error) {
	return m.generateFullReportFn(ctx, req)
}
func (m *mockPortfolioReportService) GenerateSummaryReport(ctx context.Context, req *reporting.PortfolioSummaryRequest) (*reporting.PortfolioReportResult, error) {
	return m.generateSummaryReportFn(ctx, req)
}
func (m *mockPortfolioReportService) GenerateGapReport(ctx context.Context, req *reporting.GapReportRequest) (*reporting.PortfolioReportResult, error) {
	return m.generateGapReportFn(ctx, req)
}
func (m *mockPortfolioReportService) GenerateCompetitiveReport(ctx context.Context, req *reporting.CompetitiveReportRequest) (*reporting.PortfolioReportResult, error) {
	return m.generateCompetitiveReportFn(ctx, req)
}
func (m *mockPortfolioReportService) GetReportStatus(ctx context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
	return m.getReportStatusFn(ctx, reportID)
}
func (m *mockPortfolioReportService) ListReports(ctx context.Context, portfolioID string, opts *reporting.ListReportOptions) (*commontypes.PaginatedResult[reporting.ReportMeta], error) {
	return m.listReportsFn(ctx, portfolioID, opts)
}
func (m *mockPortfolioReportService) ExportReport(ctx context.Context, reportID string, format reporting.ExportFormat) ([]byte, error) {
	return m.exportReportFn(ctx, reportID, format)
}

// mockTemplateEngine implements reporting.TemplateEngine for testing.
type mockTemplateEngine struct {
	renderFn           func(context.Context, *reporting.RenderRequest) (*reporting.RenderResult, error)
	renderToBytesFn    func(context.Context, *reporting.RenderRequest) ([]byte, error)
	listTemplatesFn    func(context.Context, *reporting.ListTemplateOptions) (*commontypes.PaginatedResult[reporting.TemplateMeta], error)
	getTemplateFn      func(context.Context, string) (*reporting.Template, error)
	registerTemplateFn func(context.Context, *reporting.Template) error
	updateTemplateFn   func(context.Context, *reporting.Template) error
	deleteTemplateFn   func(context.Context, string) error
	validateTemplateFn func(context.Context, *reporting.Template) (*reporting.ValidationResult, error)
	previewTemplateFn  func(context.Context, string, map[string]interface{}) (*reporting.RenderResult, error)
}

func (m *mockTemplateEngine) Render(ctx context.Context, req *reporting.RenderRequest) (*reporting.RenderResult, error) {
	return m.renderFn(ctx, req)
}
func (m *mockTemplateEngine) RenderToBytes(ctx context.Context, req *reporting.RenderRequest) ([]byte, error) {
	return m.renderToBytesFn(ctx, req)
}
func (m *mockTemplateEngine) ListTemplates(ctx context.Context, opts *reporting.ListTemplateOptions) (*commontypes.PaginatedResult[reporting.TemplateMeta], error) {
	return m.listTemplatesFn(ctx, opts)
}
func (m *mockTemplateEngine) GetTemplate(ctx context.Context, templateID string) (*reporting.Template, error) {
	return m.getTemplateFn(ctx, templateID)
}
func (m *mockTemplateEngine) RegisterTemplate(ctx context.Context, tmpl *reporting.Template) error {
	return m.registerTemplateFn(ctx, tmpl)
}
func (m *mockTemplateEngine) UpdateTemplate(ctx context.Context, tmpl *reporting.Template) error {
	return m.updateTemplateFn(ctx, tmpl)
}
func (m *mockTemplateEngine) DeleteTemplate(ctx context.Context, templateID string) error {
	return m.deleteTemplateFn(ctx, templateID)
}
func (m *mockTemplateEngine) ValidateTemplate(ctx context.Context, tmpl *reporting.Template) (*reporting.ValidationResult, error) {
	return m.validateTemplateFn(ctx, tmpl)
}
func (m *mockTemplateEngine) PreviewTemplate(ctx context.Context, templateID string, sampleData map[string]interface{}) (*reporting.RenderResult, error) {
	return m.previewTemplateFn(ctx, templateID, sampleData)
}

// nopCloser wraps a strings.Reader to implement io.ReadCloser.
type nopCloser struct {
	*strings.Reader
}

func (nopCloser) Close() error { return nil }

func TestReportHandler_GenerateFTOReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			generateFn: func(_ context.Context, req *reporting.FTOReportRequest) (*reporting.FTOReportResponse, error) {
				assert.Equal(t, "CCO", req.TargetMolecules[0].Value)
				assert.Equal(t, "US", req.Jurisdictions[0])
				return &reporting.FTOReportResponse{
					ReportID: "rpt-1",
					Status:   reporting.StatusQueued,
				}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"target_smiles": "CCO",
			"jurisdiction":  "US",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateFTOReport(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "rpt-1", resp["report_id"])
		assert.Equal(t, "FTO report generation initiated", resp["message"])
	})

	t.Run("missing target_smiles", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"jurisdiction": "US"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateFTOReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing jurisdiction", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"target_smiles": "CCO"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateFTOReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid format", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"target_smiles": "CCO",
			"jurisdiction":  "US",
			"format":        "invalid",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateFTOReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json body", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/fto", bytes.NewReader([]byte("{bad")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateFTOReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestReportHandler_GenerateInfringementReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		infringeSvc := &mockInfringementReportService{
			generateFn: func(_ context.Context, req *reporting.InfringementReportRequest) (*reporting.InfringementReportResponse, error) {
				assert.Equal(t, "US12345", req.OwnedPatentNumbers[0])
				assert.Equal(t, "CCO", req.SuspectedMolecules[0].Value)
				return &reporting.InfringementReportResponse{
					ReportID: "rpt-2",
					Status:   reporting.StatusQueued,
				}, nil
			},
		}
		h := NewReportHandler(&mockFTOReportService{}, infringeSvc, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"patent_number": "US12345",
			"target_smiles": []string{"CCO"},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateInfringementReport(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "rpt-2", resp["report_id"])
	})

	t.Run("missing patent_number", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"target_smiles": []string{"CCO"},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateInfringementReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing target_smiles", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"patent_number": "US12345"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateInfringementReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid format", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"patent_number": "US12345",
			"target_smiles": []string{"CCO"},
			"format":        "bad",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateInfringementReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/infringement", bytes.NewReader([]byte("{bad")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GenerateInfringementReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestReportHandler_GeneratePortfolioReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		portfolioSvc := &mockPortfolioReportService{
			generateFullReportFn: func(_ context.Context, req *reporting.PortfolioReportRequest) (*reporting.PortfolioReportResult, error) {
				assert.Equal(t, "pf-1", req.PortfolioID)
				return &reporting.PortfolioReportResult{
					ReportID: "rpt-3",
					Status:   reporting.StatusQueued,
				}, nil
			},
		}
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, portfolioSvc, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"portfolio_id": "pf-1",
			"report_type":  "full",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/portfolio", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GeneratePortfolioReport(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "rpt-3", resp["report_id"])
	})

	t.Run("missing portfolio_id", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"report_type": "full"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/portfolio", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GeneratePortfolioReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing report_type", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"portfolio_id": "pf-1"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/portfolio", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GeneratePortfolioReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid format", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"portfolio_id": "pf-1",
			"report_type":  "full",
			"format":       "bad",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/portfolio", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GeneratePortfolioReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/portfolio", bytes.NewReader([]byte("{bad")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.GeneratePortfolioReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestReportHandler_GetReportStatus(t *testing.T) {
	t.Run("success queued", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			getStatusFn: func(_ context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
				assert.Equal(t, "rpt-1", reportID)
				return &reporting.ReportStatusInfo{
					ReportID:    "rpt-1",
					Status:      reporting.StatusQueued,
					ProgressPct: 0,
				}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/rpt-1/status", nil)
		req.SetPathValue("report_id", "rpt-1")
		rec := httptest.NewRecorder()

		h.GetReportStatus(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp ReportStatusResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "rpt-1", resp.ReportID)
		assert.Equal(t, string(reporting.StatusQueued), resp.Status)
	})

	t.Run("success completed", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			getStatusFn: func(_ context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
				return &reporting.ReportStatusInfo{
					ReportID:    "rpt-1",
					Status:      reporting.StatusCompleted,
					ProgressPct: 100,
				}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/rpt-1/status", nil)
		req.SetPathValue("report_id", "rpt-1")
		rec := httptest.NewRecorder()

		h.GetReportStatus(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp ReportStatusResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Contains(t, resp.DownloadURL, "/api/v1/reports/rpt-1/download")
	})

	t.Run("missing report_id", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports//status", nil)
		rec := httptest.NewRecorder()

		h.GetReportStatus(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			getStatusFn: func(_ context.Context, _ string) (*reporting.ReportStatusInfo, error) {
				return nil, errors.NewNotFound("report not found")
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/nonexistent/status", nil)
		req.SetPathValue("report_id", "nonexistent")
		rec := httptest.NewRecorder()

		h.GetReportStatus(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestReportHandler_DownloadReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			getStatusFn: func(_ context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
				return &reporting.ReportStatusInfo{
					ReportID:    reportID,
					Status:      reporting.StatusCompleted,
					ProgressPct: 100,
				}, nil
			},
			getReportFn: func(_ context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
				assert.Equal(t, reporting.FormatPDF, format)
				return nopCloser{strings.NewReader("pdf content")}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/rpt-1/download", nil)
		req.SetPathValue("report_id", "rpt-1")
		rec := httptest.NewRecorder()

		h.DownloadReport(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment; filename=")
		assert.Equal(t, "pdf content", rec.Body.String())
	})

	t.Run("missing report_id", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports//download", nil)
		rec := httptest.NewRecorder()

		h.DownloadReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("report not ready", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			getStatusFn: func(_ context.Context, _ string) (*reporting.ReportStatusInfo, error) {
				return &reporting.ReportStatusInfo{
					Status:      reporting.StatusProcessing,
					ProgressPct: 50,
				}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/rpt-1/download", nil)
		req.SetPathValue("report_id", "rpt-1")
		rec := httptest.NewRecorder()

		h.DownloadReport(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("service error on status", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			getStatusFn: func(_ context.Context, _ string) (*reporting.ReportStatusInfo, error) {
				return nil, errors.NewNotFound("report not found")
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/nonexistent/download", nil)
		req.SetPathValue("report_id", "nonexistent")
		rec := httptest.NewRecorder()

		h.DownloadReport(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("docx format parameter", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			getStatusFn: func(_ context.Context, reportID string) (*reporting.ReportStatusInfo, error) {
				return &reporting.ReportStatusInfo{
					ReportID:    reportID,
					Status:      reporting.StatusCompleted,
					ProgressPct: 100,
				}, nil
			},
			getReportFn: func(_ context.Context, reportID string, format reporting.ReportFormat) (io.ReadCloser, error) {
				assert.Equal(t, reporting.FormatDOCX, format)
				return nopCloser{strings.NewReader("docx content")}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/rpt-1/download?format=docx", nil)
		req.SetPathValue("report_id", "rpt-1")
		rec := httptest.NewRecorder()

		h.DownloadReport(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Header().Get("Content-Type"), "wordprocessing")
	})
}

func TestReportHandler_ListReports(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			listReportsFn: func(_ context.Context, filter *reporting.FTOReportFilter, page *commontypes.Pagination) (*commontypes.PaginatedResult[reporting.FTOReportSummary], error) {
				assert.Equal(t, 1, page.Page)
				assert.Equal(t, 20, page.PageSize)
				return &commontypes.PaginatedResult[reporting.FTOReportSummary]{
					Items: []reporting.FTOReportSummary{
						{ReportID: "rpt-1", Status: reporting.StatusCompleted},
					},
					Pagination: commontypes.PaginationResult{
						Page:     1,
						PageSize: 20,
						Total:    1,
					},
				}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports", nil)
		rec := httptest.NewRecorder()

		h.ListReports(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp commontypes.PageResponse[ReportListItem]
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, int64(1), resp.Total)
	})

	t.Run("with status filter", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			listReportsFn: func(_ context.Context, filter *reporting.FTOReportFilter, page *commontypes.Pagination) (*commontypes.PaginatedResult[reporting.FTOReportSummary], error) {
				assert.Len(t, filter.Status, 1)
				assert.Equal(t, reporting.StatusCompleted, filter.Status[0])
				return &commontypes.PaginatedResult[reporting.FTOReportSummary]{
					Items: []reporting.FTOReportSummary{},
					Pagination: commontypes.PaginationResult{
						Page:     1,
						PageSize: 20,
						Total:    0,
					},
				}, nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports?status=Completed", nil)
		rec := httptest.NewRecorder()

		h.ListReports(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("invalid page_size", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports?page_size=200", nil)
		rec := httptest.NewRecorder()

		h.ListReports(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid created_from format", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports?created_from=not-a-date", nil)
		rec := httptest.NewRecorder()

		h.ListReports(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			listReportsFn: func(_ context.Context, _ *reporting.FTOReportFilter, _ *commontypes.Pagination) (*commontypes.PaginatedResult[reporting.FTOReportSummary], error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports", nil)
		rec := httptest.NewRecorder()

		h.ListReports(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestReportHandler_DeleteReport(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			deleteReportFn: func(_ context.Context, reportID string) error {
				assert.Equal(t, "rpt-1", reportID)
				return nil
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/reports/rpt-1", nil)
		req.SetPathValue("report_id", "rpt-1")
		rec := httptest.NewRecorder()

		h.DeleteReport(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "report deleted successfully", resp["message"])
	})

	t.Run("missing report_id", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/reports/", nil)
		rec := httptest.NewRecorder()

		h.DeleteReport(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		ftoSvc := &mockFTOReportService{
			deleteReportFn: func(_ context.Context, _ string) error {
				return errors.NewNotFound("report not found")
			},
		}
		h := NewReportHandler(ftoSvc, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/reports/nonexistent", nil)
		req.SetPathValue("report_id", "nonexistent")
		rec := httptest.NewRecorder()

		h.DeleteReport(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestReportHandler_ListTemplates(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		tmplSvc := &mockTemplateEngine{
			listTemplatesFn: func(_ context.Context, opts *reporting.ListTemplateOptions) (*commontypes.PaginatedResult[reporting.TemplateMeta], error) {
				assert.Equal(t, 1, opts.Pagination.Page)
				assert.Equal(t, 20, opts.Pagination.PageSize)
				return &commontypes.PaginatedResult[reporting.TemplateMeta]{
					Items: []reporting.TemplateMeta{
						{ID: "tmpl-1", Name: "FTO Report"},
					},
					Pagination: commontypes.PaginationResult{
						Page:     1,
						PageSize: 20,
						Total:    1,
					},
				}, nil
			},
		}
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, tmplSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/templates", nil)
		rec := httptest.NewRecorder()

		h.ListTemplates(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("with query params", func(t *testing.T) {
		tmplSvc := &mockTemplateEngine{
			listTemplatesFn: func(_ context.Context, opts *reporting.ListTemplateOptions) (*commontypes.PaginatedResult[reporting.TemplateMeta], error) {
				assert.Equal(t, 2, opts.Pagination.Page)
				assert.Equal(t, 10, opts.Pagination.PageSize)
				assert.NotNil(t, opts.Type)
				assert.NotNil(t, opts.Format)
				assert.NotNil(t, opts.Keyword)
				return &commontypes.PaginatedResult[reporting.TemplateMeta]{
					Items: []reporting.TemplateMeta{},
					Pagination: commontypes.PaginationResult{
						Page:     2,
						PageSize: 10,
						Total:    0,
					},
				}, nil
			},
		}
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, tmplSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/templates?page=2&page_size=10&type=fto&format=PDF&q=test", nil)
		rec := httptest.NewRecorder()

		h.ListTemplates(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		tmplSvc := &mockTemplateEngine{
			listTemplatesFn: func(_ context.Context, _ *reporting.ListTemplateOptions) (*commontypes.PaginatedResult[reporting.TemplateMeta], error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, tmplSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/templates", nil)
		rec := httptest.NewRecorder()

		h.ListTemplates(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestReportHandler_GetTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tmplSvc := &mockTemplateEngine{
			getTemplateFn: func(_ context.Context, id string) (*reporting.Template, error) {
				assert.Equal(t, "tmpl-1", id)
				return &reporting.Template{ID: "tmpl-1", Name: "FTO Report"}, nil
			},
		}
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, tmplSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/templates/tmpl-1", nil)
		req.SetPathValue("id", "tmpl-1")
		rec := httptest.NewRecorder()

		h.GetTemplate(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, &mockTemplateEngine{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/templates/", nil)
		rec := httptest.NewRecorder()

		h.GetTemplate(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		tmplSvc := &mockTemplateEngine{
			getTemplateFn: func(_ context.Context, _ string) (*reporting.Template, error) {
				return nil, errors.NewNotFound("template not found")
			},
		}
		h := NewReportHandler(&mockFTOReportService{}, &mockInfringementReportService{}, &mockPortfolioReportService{}, tmplSvc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/templates/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.GetTemplate(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

//Personal.AI order the ending
