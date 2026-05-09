// Real tests for portfolio HTTP handler.
// Tests request parsing, validation, error responses, and successful responses.

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// mockPortfolioService implements portfolio.Service for testing.
type mockPortfolioService struct {
	createFn        func(context.Context, *portfolio.CreateInput) (*portfolio.Portfolio, error)
	getByIDFn       func(context.Context, string) (*portfolio.Portfolio, error)
	listFn          func(context.Context, *portfolio.ListInput) (*portfolio.ListResult, error)
	updateFn        func(context.Context, *portfolio.UpdateInput) (*portfolio.Portfolio, error)
	deleteFn        func(context.Context, string, string) error
	addPatentsFn    func(context.Context, string, []string, string) error
	removePatentsFn func(context.Context, string, []string, string) error
	getAnalysisFn   func(context.Context, string) (*portfolio.PortfolioAnalysis, error)
}

func (m *mockPortfolioService) Create(ctx context.Context, in *portfolio.CreateInput) (*portfolio.Portfolio, error) {
	return m.createFn(ctx, in)
}
func (m *mockPortfolioService) GetByID(ctx context.Context, id string) (*portfolio.Portfolio, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockPortfolioService) List(ctx context.Context, in *portfolio.ListInput) (*portfolio.ListResult, error) {
	return m.listFn(ctx, in)
}
func (m *mockPortfolioService) Update(ctx context.Context, in *portfolio.UpdateInput) (*portfolio.Portfolio, error) {
	return m.updateFn(ctx, in)
}
func (m *mockPortfolioService) Delete(ctx context.Context, id, userID string) error {
	return m.deleteFn(ctx, id, userID)
}
func (m *mockPortfolioService) AddPatents(ctx context.Context, id string, patentIDs []string, userID string) error {
	return m.addPatentsFn(ctx, id, patentIDs, userID)
}
func (m *mockPortfolioService) RemovePatents(ctx context.Context, id string, patentIDs []string, userID string) error {
	return m.removePatentsFn(ctx, id, patentIDs, userID)
}
func (m *mockPortfolioService) GetAnalysis(ctx context.Context, id string) (*portfolio.PortfolioAnalysis, error) {
	return m.getAnalysisFn(ctx, id)
}

func TestPortfolioHandler_CreatePortfolio(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			createFn: func(_ context.Context, in *portfolio.CreateInput) (*portfolio.Portfolio, error) {
				assert.Equal(t, "test-portfolio", in.Name)
				return &portfolio.Portfolio{ID: "pf-1", Name: "test-portfolio"}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test-portfolio"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreatePortfolio(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp portfolio.Portfolio
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pf-1", resp.ID)
	})

	t.Run("missing name", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"description": "no name"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreatePortfolio(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp ErrorResponse
		json.NewDecoder(rec.Body).Decode(&errResp)
		assert.Contains(t, errResp.Message, "name is required")
	})

	t.Run("invalid json body", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios", bytes.NewReader([]byte("{bad")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreatePortfolio(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockPortfolioService{
			createFn: func(_ context.Context, _ *portfolio.CreateInput) (*portfolio.Portfolio, error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreatePortfolio(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPortfolioHandler_GetPortfolio(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			getByIDFn: func(_ context.Context, id string) (*portfolio.Portfolio, error) {
				assert.Equal(t, "pf-1", id)
				return &portfolio.Portfolio{ID: "pf-1", Name: "test"}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/pf-1", nil)
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.GetPortfolio(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp portfolio.Portfolio
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pf-1", resp.ID)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/", nil)
		rec := httptest.NewRecorder()

		h.GetPortfolio(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		svc := &mockPortfolioService{
			getByIDFn: func(_ context.Context, _ string) (*portfolio.Portfolio, error) {
				return nil, errors.NewNotFound("portfolio not found")
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.GetPortfolio(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestPortfolioHandler_ListPortfolios(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		svc := &mockPortfolioService{
			listFn: func(_ context.Context, in *portfolio.ListInput) (*portfolio.ListResult, error) {
				assert.Equal(t, 1, in.Page)
				assert.Equal(t, 20, in.PageSize)
				return &portfolio.ListResult{
					Portfolios: []*portfolio.Portfolio{{ID: "pf-1", Name: "test"}},
					Total:     1,
					Page:      1,
					PageSize:  20,
				}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios", nil)
		rec := httptest.NewRecorder()

		h.ListPortfolios(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp portfolio.ListResult
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp.Portfolios, 1)
		assert.Equal(t, int64(1), resp.Total)
	})

	t.Run("with query params", func(t *testing.T) {
		svc := &mockPortfolioService{
			listFn: func(_ context.Context, in *portfolio.ListInput) (*portfolio.ListResult, error) {
				assert.Equal(t, 2, in.Page)
				assert.Equal(t, 10, in.PageSize)
				return &portfolio.ListResult{Portfolios: []*portfolio.Portfolio{}, Total: 0, Page: 2, PageSize: 10}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios?page=2&page_size=10", nil)
		rec := httptest.NewRecorder()

		h.ListPortfolios(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockPortfolioService{
			listFn: func(_ context.Context, _ *portfolio.ListInput) (*portfolio.ListResult, error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios", nil)
		rec := httptest.NewRecorder()

		h.ListPortfolios(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPortfolioHandler_UpdatePortfolio(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			updateFn: func(_ context.Context, in *portfolio.UpdateInput) (*portfolio.Portfolio, error) {
				assert.Equal(t, "pf-1", in.ID)
				return &portfolio.Portfolio{ID: "pf-1", Name: "updated"}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "updated"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/portfolios/pf-1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.UpdatePortfolio(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/portfolios/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdatePortfolio(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPut, "/api/v1/portfolios/pf-1", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("id", "pf-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdatePortfolio(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockPortfolioService{
			updateFn: func(_ context.Context, _ *portfolio.UpdateInput) (*portfolio.Portfolio, error) {
				return nil, errors.NewNotFound("portfolio not found")
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "updated"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/portfolios/nonexistent", bytes.NewReader(body))
		req.SetPathValue("id", "nonexistent")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdatePortfolio(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestPortfolioHandler_DeletePortfolio(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			deleteFn: func(_ context.Context, id, _ string) error {
				assert.Equal(t, "pf-1", id)
				return nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/portfolios/pf-1", nil)
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.DeletePortfolio(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/portfolios/", nil)
		rec := httptest.NewRecorder()

		h.DeletePortfolio(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockPortfolioService{
			deleteFn: func(_ context.Context, _, _ string) error { return errors.NewNotFound("portfolio not found") },
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/portfolios/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.DeletePortfolio(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestPortfolioHandler_AddPatents(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			addPatentsFn: func(_ context.Context, id string, patentIDs []string, _ string) error {
				assert.Equal(t, "pf-1", id)
				assert.Equal(t, []string{"pat-1", "pat-2"}, patentIDs)
				return nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"patent_ids": []string{"pat-1", "pat-2"}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-1/patents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.AddPatents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing patent_ids", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"patent_ids": []string{}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-1/patents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.AddPatents(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-1/patents", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("id", "pf-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AddPatents(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPortfolioHandler_RemovePatents(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			removePatentsFn: func(_ context.Context, id string, patentIDs []string, _ string) error {
				assert.Equal(t, "pf-1", id)
				assert.Equal(t, []string{"pat-1"}, patentIDs)
				return nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"patent_ids": []string{"pat-1"}})
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/portfolios/pf-1/patents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.RemovePatents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing patent_ids", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"patent_ids": []string{}})
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/portfolios/pf-1/patents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.RemovePatents(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPortfolioHandler_GetAnalysis(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			getAnalysisFn: func(_ context.Context, id string) (*portfolio.PortfolioAnalysis, error) {
				assert.Equal(t, "pf-1", id)
				return &portfolio.PortfolioAnalysis{
					PortfolioID:  "pf-1",
					TotalPatents: 5,
					TotalValue:   1000000,
				}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/pf-1/analysis", nil)
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.GetAnalysis(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp portfolio.PortfolioAnalysis
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pf-1", resp.PortfolioID)
		assert.Equal(t, 5, resp.TotalPatents)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios//analysis", nil)
		rec := httptest.NewRecorder()

		h.GetAnalysis(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockPortfolioService{
			getAnalysisFn: func(_ context.Context, _ string) (*portfolio.PortfolioAnalysis, error) {
				return nil, errors.NewNotFound("portfolio not found")
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/nonexistent/analysis", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.GetAnalysis(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestPortfolioHandler_RunValuation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			getAnalysisFn: func(_ context.Context, id string) (*portfolio.PortfolioAnalysis, error) {
				assert.Equal(t, "pf-1", id)
				return &portfolio.PortfolioAnalysis{
					PortfolioID:     "pf-1",
					TotalValue:      500000,
					ByJurisdiction:  map[string]int{"US": 3},
					ByStatus:        map[string]int{"granted": 3},
					Recommendations: []string{"maintain"},
				}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-1/valuation/run", nil)
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.RunValuation(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pf-1", resp["portfolio_id"])
		assert.Equal(t, "valuation completed", resp["message"])
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios//valuation/run", nil)
		rec := httptest.NewRecorder()

		h.RunValuation(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPortfolioHandler_GetGapAnalysis(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			getAnalysisFn: func(_ context.Context, id string) (*portfolio.PortfolioAnalysis, error) {
				return &portfolio.PortfolioAnalysis{
					PortfolioID:  "pf-1",
					TotalPatents: 10,
				}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/pf-1/gap-analysis", nil)
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.GetGapAnalysis(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pf-1", resp["portfolio_id"])
		assert.Equal(t, float64(10), resp["total_patents"])
	})
}

func TestPortfolioHandler_RunGapAnalysis(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			getAnalysisFn: func(_ context.Context, id string) (*portfolio.PortfolioAnalysis, error) {
				return &portfolio.PortfolioAnalysis{
					PortfolioID:     "pf-1",
					TotalPatents:    10,
					Recommendations: []string{"expand in US"},
				}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-1/gap-analysis/run", nil)
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.RunGapAnalysis(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pf-1", resp["portfolio_id"])
		assert.Equal(t, "gap analysis completed", resp["message"])
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios//gap-analysis/run", nil)
		rec := httptest.NewRecorder()

		h.RunGapAnalysis(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPortfolioHandler_Optimize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPortfolioService{
			getAnalysisFn: func(_ context.Context, id string) (*portfolio.PortfolioAnalysis, error) {
				return &portfolio.PortfolioAnalysis{
					PortfolioID:     "pf-1",
					TotalPatents:    10,
					TotalValue:      2000000,
					Recommendations: []string{"divest non-core"},
				}, nil
			},
		}
		h := NewPortfolioHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-1/optimize", nil)
		req.SetPathValue("id", "pf-1")
		rec := httptest.NewRecorder()

		h.Optimize(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pf-1", resp["portfolio_id"])
		assert.Equal(t, "optimization completed", resp["message"])
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPortfolioHandler(&mockPortfolioService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios//optimize", nil)
		rec := httptest.NewRecorder()

		h.Optimize(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

//Personal.AI order the ending
