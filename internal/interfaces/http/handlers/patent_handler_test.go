// Phase 11 - File 267: internal/interfaces/http/handlers/patent_handler_test.go
// Real tests for patent HTTP handler.

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/infringement"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// mockPatentService implements patent.Service for testing.
type mockPatentService struct {
	createFn        func(context.Context, *patent.CreateInput) (*patent.Patent, error)
	getByIDFn       func(context.Context, string) (*patent.Patent, error)
	listFn          func(context.Context, *patent.ListInput) (*patent.ListResult, error)
	updateFn        func(context.Context, *patent.UpdateInput) (*patent.Patent, error)
	deleteFn        func(context.Context, string, string) error
	searchFn        func(context.Context, *patent.SearchInput) (*patent.SearchResult, error)
	advancedSearchFn func(context.Context, *patent.AdvancedSearchInput) (*patent.SearchResult, error)
	getStatsFn      func(context.Context, *patent.StatsInput) (*patent.Stats, error)
}

func (m *mockPatentService) Create(ctx context.Context, in *patent.CreateInput) (*patent.Patent, error) {
	return m.createFn(ctx, in)
}
func (m *mockPatentService) GetByID(ctx context.Context, id string) (*patent.Patent, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockPatentService) List(ctx context.Context, in *patent.ListInput) (*patent.ListResult, error) {
	return m.listFn(ctx, in)
}
func (m *mockPatentService) Update(ctx context.Context, in *patent.UpdateInput) (*patent.Patent, error) {
	return m.updateFn(ctx, in)
}
func (m *mockPatentService) Delete(ctx context.Context, id, userID string) error {
	return m.deleteFn(ctx, id, userID)
}
func (m *mockPatentService) Search(ctx context.Context, in *patent.SearchInput) (*patent.SearchResult, error) {
	return m.searchFn(ctx, in)
}
func (m *mockPatentService) AdvancedSearch(ctx context.Context, in *patent.AdvancedSearchInput) (*patent.SearchResult, error) {
	return m.advancedSearchFn(ctx, in)
}
func (m *mockPatentService) GetStats(ctx context.Context, in *patent.StatsInput) (*patent.Stats, error) {
	return m.getStatsFn(ctx, in)
}

// mockInfringementSvc implements infringement.RiskAssessmentService minimally.
type mockInfringementSvc struct {
	assessFTOFn      func(context.Context, *infringement.FTORequest) (*infringement.FTOResponse, error)
	assessMoleculeFn func(context.Context, *infringement.MoleculeRiskRequest) (*infringement.MoleculeRiskResponse, error)
}

func (m *mockInfringementSvc) AssessMolecule(ctx context.Context, req *infringement.MoleculeRiskRequest) (*infringement.MoleculeRiskResponse, error) {
	return m.assessMoleculeFn(ctx, req)
}

func (m *mockInfringementSvc) AssessBatch(ctx context.Context, req *infringement.BatchRiskRequest) (*infringement.BatchRiskResponse, error) {
	return nil, nil
}

func (m *mockInfringementSvc) AssessFTO(ctx context.Context, req *infringement.FTORequest) (*infringement.FTOResponse, error) {
	return m.assessFTOFn(ctx, req)
}

func (m *mockInfringementSvc) GetRiskSummary(ctx context.Context, portfolioID string) (*infringement.RiskSummaryResponse, error) {
	return nil, nil
}

func (m *mockInfringementSvc) GetRiskHistory(ctx context.Context, moleculeID string, opts ...infringement.QueryOption) ([]*infringement.RiskRecord, error) {
	return nil, nil
}

func TestPatentHandler_CreatePatent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			createFn: func(_ context.Context, in *patent.CreateInput) (*patent.Patent, error) {
				assert.Equal(t, "Test Patent", in.Title)
				assert.Equal(t, "APP001", in.ApplicationNo)
				return &patent.Patent{ID: "pat-1", Title: "Test Patent"}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"title":          "Test Patent",
			"abstract":       "An invention",
			"application_no": "APP001",
			"applicant":      "Acme",
			"inventors":      []string{"Alice"},
			"filing_date":    "2024-01-15",
			"jurisdiction":   "US",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreatePatent(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp patent.Patent
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pat-1", resp.ID)
	})

	t.Run("missing required fields", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"title": "Test"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreatePatent(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents", bytes.NewReader([]byte("{")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreatePatent(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatentHandler_GetPatent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			getByIDFn: func(_ context.Context, id string) (*patent.Patent, error) {
				assert.Equal(t, "pat-1", id)
				return &patent.Patent{ID: "pat-1", Title: "Test"}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-1", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.GetPatent(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/", nil)
		rec := httptest.NewRecorder()

		h.GetPatent(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		svc := &mockPatentService{
			getByIDFn: func(_ context.Context, _ string) (*patent.Patent, error) {
				return nil, errors.NewNotFound("patent not found")
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/nope", nil)
		req.SetPathValue("id", "nope")
		rec := httptest.NewRecorder()

		h.GetPatent(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestPatentHandler_ListPatents(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			listFn: func(_ context.Context, in *patent.ListInput) (*patent.ListResult, error) {
				assert.Equal(t, 1, in.Page)
				assert.Equal(t, 20, in.PageSize)
				return &patent.ListResult{
					Patents:  []*patent.Patent{{ID: "pat-1"}},
					Total:    1,
					Page:     1,
					PageSize: 20,
				}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
		rec := httptest.NewRecorder()

		h.ListPatents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp patent.ListResult
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp.Patents, 1)
	})

	t.Run("with filters", func(t *testing.T) {
		svc := &mockPatentService{
			listFn: func(_ context.Context, in *patent.ListInput) (*patent.ListResult, error) {
				assert.Equal(t, "US", in.Jurisdiction)
				assert.Equal(t, "Acme", in.Applicant)
				return &patent.ListResult{Patents: []*patent.Patent{}, Total: 0}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents?jurisdiction=US&applicant=Acme", nil)
		rec := httptest.NewRecorder()

		h.ListPatents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockPatentService{
			listFn: func(_ context.Context, _ *patent.ListInput) (*patent.ListResult, error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents", nil)
		rec := httptest.NewRecorder()

		h.ListPatents(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPatentHandler_UpdatePatent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			updateFn: func(_ context.Context, in *patent.UpdateInput) (*patent.Patent, error) {
				assert.Equal(t, "pat-1", in.ID)
				return &patent.Patent{ID: "pat-1", Title: "Updated"}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"title": "Updated"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/patents/pat-1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.UpdatePatent(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"title": "Test"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/patents/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdatePatent(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatentHandler_DeletePatent(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			deleteFn: func(_ context.Context, id, _ string) error {
				assert.Equal(t, "pat-1", id)
				return nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/patents/pat-1", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.DeletePatent(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/patents/", nil)
		rec := httptest.NewRecorder()

		h.DeletePatent(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatentHandler_SearchPatents(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			searchFn: func(_ context.Context, in *patent.SearchInput) (*patent.SearchResult, error) {
				assert.Equal(t, "OLED", in.Query)
				return &patent.SearchResult{
					Patents: []*patent.Patent{{ID: "pat-1"}},
					Total:   1,
				}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"query": "OLED", "page": 1, "page_size": 20})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchPatents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing query", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]int{"page": 1})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchPatents(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("defaults with filters", func(t *testing.T) {
		svc := &mockPatentService{
			searchFn: func(_ context.Context, in *patent.SearchInput) (*patent.SearchResult, error) {
				assert.Equal(t, "OLED", in.Query)
				assert.Equal(t, 1, in.Page)
				assert.Equal(t, 20, in.PageSize)
				assert.Equal(t, "", in.SortBy)
				return &patent.SearchResult{}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"query": "OLED",
			"filters": map[string]string{
				"applicant": "Acme",
			},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchPatents(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestPatentHandler_AdvancedSearch(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			advancedSearchFn: func(_ context.Context, in *patent.AdvancedSearchInput) (*patent.SearchResult, error) {
				assert.Equal(t, "Acme", in.Applicant)
				assert.Equal(t, "H01L", in.IPCCode)
				return &patent.SearchResult{Patents: []*patent.Patent{{ID: "pat-1"}}, Total: 1}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"applicant": "Acme",
			"ipc_code":  "H01L",
			"page":      1,
			"page_size": 20,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search/advanced", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AdvancedSearch(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search/advanced", bytes.NewReader([]byte("")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AdvancedSearch(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatentHandler_GetPatentStats(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			getStatsFn: func(_ context.Context, in *patent.StatsInput) (*patent.Stats, error) {
				return &patent.Stats{
					TotalPatents:   100,
					ByJurisdiction: map[string]int64{"US": 50},
				}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/stats", nil)
		rec := httptest.NewRecorder()

		h.GetPatentStats(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp patent.Stats
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, int64(100), resp.TotalPatents)
	})
}

func TestPatentHandler_AnalyzeClaims(t *testing.T) {
	t.Run("with claim texts", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"claim_texts": []string{"1. A compound of formula X.", "2. The compound of claim 1."},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/analyze-claims", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AnalyzeClaims(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp ClaimAnalysisResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, 2, resp.TotalClaims)
		assert.Equal(t, 1, resp.IndependentCount)
		assert.Equal(t, 1, resp.DependentCount)
	})

	t.Run("with patent id", func(t *testing.T) {
		svc := &mockPatentService{
			getByIDFn: func(_ context.Context, id string) (*patent.Patent, error) {
				assert.Equal(t, "pat-1", id)
				return &patent.Patent{
					ID:     "pat-1",
					Title:  "Test Patent",
					Claims: "1. A compound.\n2. The compound of claim 1.",
				}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"patent_id": "pat-1"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/analyze-claims", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AnalyzeClaims(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp ClaimAnalysisResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "Test Patent", resp.PatentTitle)
		assert.Equal(t, 2, resp.TotalClaims)
	})

	t.Run("missing both patent_id and claim_texts", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/analyze-claims", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AnalyzeClaims(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/analyze-claims", bytes.NewReader([]byte("bad")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AnalyzeClaims(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("empty claims text returns empty result", func(t *testing.T) {
		svc := &mockPatentService{
			getByIDFn: func(_ context.Context, _ string) (*patent.Patent, error) {
				return &patent.Patent{ID: "pat-1", Title: "Empty", Claims: ""}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"patent_id": "pat-1"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/analyze-claims", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AnalyzeClaims(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp ClaimAnalysisResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, 0, resp.TotalClaims)
	})
}

func TestPatentHandler_GetFamily(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			getByIDFn: func(_ context.Context, id string) (*patent.Patent, error) {
				return &patent.Patent{
					ID:            id,
					Title:         "Test",
					PublicationNo: "US20240000001",
					Applicant:     "Acme",
					Jurisdiction:  "US",
				}, nil
			},
			searchFn: func(_ context.Context, in *patent.SearchInput) (*patent.SearchResult, error) {
				return &patent.SearchResult{
					Patents: []*patent.Patent{
						{ID: "pat-2", Title: "Related", PublicationNo: "EP40000001", Jurisdiction: "EP", FilingDate: "2024-01-01", Applicant: "Acme"},
					},
				}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-1/family", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.GetFamily(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp FamilyResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pat-1", resp.PatentID)
		assert.Equal(t, 1, resp.TotalMembers)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents//family", nil)
		rec := httptest.NewRecorder()

		h.GetFamily(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatentHandler_GetCitationNetwork(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockPatentService{
			getByIDFn: func(_ context.Context, id string) (*patent.Patent, error) {
				return &patent.Patent{
					ID: id, Title: "Test", PublicationNo: "US20240000001",
					Applicant: "Acme", FilingDate: "2024-06-01",
				}, nil
			},
			searchFn: func(_ context.Context, in *patent.SearchInput) (*patent.SearchResult, error) {
				return &patent.SearchResult{
					Patents: []*patent.Patent{
						{ID: "pat-2", PublicationNo: "US20230000001", FilingDate: "2023-01-01", Applicant: "Acme", Title: "Earlier"},
						{ID: "pat-3", PublicationNo: "US20250000001", FilingDate: "2025-01-01", Applicant: "Acme", Title: "Later"},
					},
				}, nil
			},
		}
		h := NewPatentHandler(svc, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-1/citations", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.GetCitationNetwork(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp CitationNetworkResponse
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, 2, resp.TotalCitations)
		assert.Len(t, resp.ForwardCitations, 1)
		assert.Len(t, resp.BackwardCitations, 1)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents//citations", nil)
		rec := httptest.NewRecorder()

		h.GetCitationNetwork(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatentHandler_CheckFTO(t *testing.T) {
	t.Run("success with infringement service", func(t *testing.T) {
		infSvc := &mockInfringementSvc{
			assessFTOFn: func(_ context.Context, req *infringement.FTORequest) (*infringement.FTOResponse, error) {
				assert.Equal(t, "CCO", req.Molecules[0].SMILES)
				assert.Equal(t, []string{"US"}, req.Jurisdictions)
				return &infringement.FTOResponse{FTOID: "fto-1"}, nil
			},
		}
		h := NewPatentHandler(&mockPatentService{}, infSvc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"molecule_smiles": "CCO",
			"jurisdictions":   []string{"US"},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/check-fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CheckFTO(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing infringement service", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"molecule_smiles": "CCO",
			"jurisdictions":   []string{"US"},
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/check-fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CheckFTO(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("missing molecule_smiles", func(t *testing.T) {
		infSvc := &mockInfringementSvc{}
		h := NewPatentHandler(&mockPatentService{}, infSvc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"jurisdictions": []string{"US"}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/check-fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CheckFTO(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing jurisdictions", func(t *testing.T) {
		infSvc := &mockInfringementSvc{}
		h := NewPatentHandler(&mockPatentService{}, infSvc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"molecule_smiles": "CCO"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/check-fto", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CheckFTO(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestPatentHandler_AssessInfringementRisk(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		infSvc := &mockInfringementSvc{
			assessMoleculeFn: func(_ context.Context, req *infringement.MoleculeRiskRequest) (*infringement.MoleculeRiskResponse, error) {
				assert.Equal(t, "CCO", req.SMILES)
				return &infringement.MoleculeRiskResponse{AssessmentID: "ra-1", OverallRiskLevel: infringement.RiskLevelLow}, nil
			},
		}
		h := NewPatentHandler(&mockPatentService{}, infSvc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"molecule_smiles": "CCO",
			"patent_id":       "pat-1",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/assess-infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AssessInfringementRisk(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing infringement service", func(t *testing.T) {
		h := NewPatentHandler(&mockPatentService{}, nil, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{
			"molecule_smiles": "CCO",
			"patent_id":       "pat-1",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/assess-infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AssessInfringementRisk(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("missing molecule_smiles", func(t *testing.T) {
		infSvc := &mockInfringementSvc{}
		h := NewPatentHandler(&mockPatentService{}, infSvc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"patent_id": "pat-1"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/assess-infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AssessInfringementRisk(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing patent_id", func(t *testing.T) {
		infSvc := &mockInfringementSvc{}
		h := NewPatentHandler(&mockPatentService{}, infSvc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"molecule_smiles": "CCO"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/assess-infringement", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AssessInfringementRisk(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

//Personal.AI order the ending
