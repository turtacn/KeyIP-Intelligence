// Phase 11 - File 265: internal/interfaces/http/handlers/molecule_handler_test.go
// Real tests for molecule HTTP handler.
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
	"github.com/turtacn/KeyIP-Intelligence/internal/application/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// mockMoleculeService implements molecule.Service for testing.
type mockMoleculeService struct {
	createFn             func(context.Context, *molecule.CreateInput) (*molecule.Molecule, error)
	getByIDFn            func(context.Context, string) (*molecule.Molecule, error)
	listFn               func(context.Context, *molecule.ListInput) (*molecule.ListResult, error)
	updateFn             func(context.Context, *molecule.UpdateInput) (*molecule.Molecule, error)
	deleteFn             func(context.Context, string, string) error
	searchByStructureFn  func(context.Context, *molecule.StructureSearchInput) (*molecule.SearchResult, error)
	searchBySimilarityFn func(context.Context, *molecule.SimilaritySearchInput) (*molecule.SearchResult, error)
	calcPropsFn          func(context.Context, *molecule.CalculatePropertiesInput) (*molecule.PropertiesResult, error)
}

func (m *mockMoleculeService) Create(ctx context.Context, in *molecule.CreateInput) (*molecule.Molecule, error) {
	return m.createFn(ctx, in)
}
func (m *mockMoleculeService) GetByID(ctx context.Context, id string) (*molecule.Molecule, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockMoleculeService) List(ctx context.Context, in *molecule.ListInput) (*molecule.ListResult, error) {
	return m.listFn(ctx, in)
}
func (m *mockMoleculeService) Update(ctx context.Context, in *molecule.UpdateInput) (*molecule.Molecule, error) {
	return m.updateFn(ctx, in)
}
func (m *mockMoleculeService) Delete(ctx context.Context, id, userID string) error {
	return m.deleteFn(ctx, id, userID)
}
func (m *mockMoleculeService) SearchByStructure(ctx context.Context, in *molecule.StructureSearchInput) (*molecule.SearchResult, error) {
	return m.searchByStructureFn(ctx, in)
}
func (m *mockMoleculeService) SearchBySimilarity(ctx context.Context, in *molecule.SimilaritySearchInput) (*molecule.SearchResult, error) {
	return m.searchBySimilarityFn(ctx, in)
}
func (m *mockMoleculeService) CalculateProperties(ctx context.Context, in *molecule.CalculatePropertiesInput) (*molecule.PropertiesResult, error) {
	return m.calcPropsFn(ctx, in)
}

func TestMoleculeHandler_CreateMolecule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMoleculeService{
			createFn: func(_ context.Context, in *molecule.CreateInput) (*molecule.Molecule, error) {
				assert.Equal(t, "test-mol", in.Name)
				assert.Equal(t, "CCO", in.SMILES)
				return &molecule.Molecule{ID: "mol-1", Name: "test-mol", SMILES: "CCO"}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"name": "test-mol", "smiles": "CCO"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateMolecule(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp molecule.Molecule
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "mol-1", resp.ID)
	})

	t.Run("missing smiles", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test-mol"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateMolecule(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var errResp ErrorResponse
		json.NewDecoder(rec.Body).Decode(&errResp)
		assert.Contains(t, errResp.Message, "smiles is required")
	})

	t.Run("invalid json body", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules", bytes.NewReader([]byte("{invalid")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateMolecule(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockMoleculeService{
			createFn: func(_ context.Context, _ *molecule.CreateInput) (*molecule.Molecule, error) {
				return nil, errors.NewNotFound("molecule conflict")
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test", "smiles": "CCO"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CreateMolecule(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestMoleculeHandler_GetMolecule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMoleculeService{
			getByIDFn: func(_ context.Context, id string) (*molecule.Molecule, error) {
				assert.Equal(t, "mol-1", id)
				return &molecule.Molecule{ID: "mol-1", Name: "test", SMILES: "CCO"}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules/mol-1", nil)
		req.SetPathValue("id", "mol-1")
		rec := httptest.NewRecorder()

		h.GetMolecule(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp molecule.Molecule
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "mol-1", resp.ID)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules/", nil)
		rec := httptest.NewRecorder()

		h.GetMolecule(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		svc := &mockMoleculeService{
			getByIDFn: func(_ context.Context, _ string) (*molecule.Molecule, error) {
				return nil, errors.NewNotFound("molecule not found")
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.GetMolecule(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestMoleculeHandler_ListMolecules(t *testing.T) {
	t.Run("success with defaults", func(t *testing.T) {
		svc := &mockMoleculeService{
			listFn: func(_ context.Context, in *molecule.ListInput) (*molecule.ListResult, error) {
				assert.Equal(t, 1, in.Page)
				assert.Equal(t, 20, in.PageSize)
				return &molecule.ListResult{
					Molecules: []*molecule.Molecule{{ID: "mol-1", Name: "test"}},
					Total:     1,
					Page:      1,
					PageSize:  20,
				}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules", nil)
		rec := httptest.NewRecorder()

		h.ListMolecules(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp molecule.ListResult
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp.Molecules, 1)
		assert.Equal(t, int64(1), resp.Total)
	})

	t.Run("with query params", func(t *testing.T) {
		svc := &mockMoleculeService{
			listFn: func(_ context.Context, in *molecule.ListInput) (*molecule.ListResult, error) {
				assert.Equal(t, 2, in.Page)
				assert.Equal(t, 10, in.PageSize)
				assert.Equal(t, "CCO", in.Query)
				assert.Equal(t, "organic", in.Tag)
				return &molecule.ListResult{Molecules: []*molecule.Molecule{}, Total: 0, Page: 2, PageSize: 10}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules?page=2&page_size=10&q=CCO&tag=organic", nil)
		rec := httptest.NewRecorder()

		h.ListMolecules(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockMoleculeService{
			listFn: func(_ context.Context, _ *molecule.ListInput) (*molecule.ListResult, error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules", nil)
		rec := httptest.NewRecorder()

		h.ListMolecules(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestMoleculeHandler_UpdateMolecule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMoleculeService{
			updateFn: func(_ context.Context, in *molecule.UpdateInput) (*molecule.Molecule, error) {
				assert.Equal(t, "mol-1", in.ID)
				return &molecule.Molecule{ID: "mol-1", Name: "updated"}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "updated"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/molecules/mol-1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("id", "mol-1")
		rec := httptest.NewRecorder()

		h.UpdateMolecule(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"name": "test"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/molecules/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdateMolecule(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		svc := &mockMoleculeService{
			updateFn: func(_ context.Context, in *molecule.UpdateInput) (*molecule.Molecule, error) {
				return nil, errors.NewNotFound("molecule not found")
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPut, "/api/v1/molecules/mol-1", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("id", "mol-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.UpdateMolecule(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestMoleculeHandler_DeleteMolecule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMoleculeService{
			deleteFn: func(_ context.Context, id, _ string) error {
				assert.Equal(t, "mol-1", id)
				return nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/molecules/mol-1", nil)
		req.SetPathValue("id", "mol-1")
		rec := httptest.NewRecorder()

		h.DeleteMolecule(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/molecules/", nil)
		rec := httptest.NewRecorder()

		h.DeleteMolecule(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockMoleculeService{
			deleteFn: func(_ context.Context, _, _ string) error { return errors.NewNotFound("molecule not found") },
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/molecules/nonexistent", nil)
		req.SetPathValue("id", "nonexistent")
		rec := httptest.NewRecorder()

		h.DeleteMolecule(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestMoleculeHandler_SearchByStructure(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMoleculeService{
			searchByStructureFn: func(_ context.Context, in *molecule.StructureSearchInput) (*molecule.SearchResult, error) {
				assert.Equal(t, "CCO", in.SMILES)
				assert.Equal(t, "substructure", in.SearchType)
				return &molecule.SearchResult{Molecules: []*molecule.MoleculeMatch{}, Total: 0}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"smiles": "CCO", "search_type": "substructure", "max_results": 50})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/structure", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchByStructure(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing smiles", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"search_type": "substructure"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/structure", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchByStructure(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("defaults when zero values", func(t *testing.T) {
		svc := &mockMoleculeService{
			searchByStructureFn: func(_ context.Context, in *molecule.StructureSearchInput) (*molecule.SearchResult, error) {
				assert.Equal(t, "CCO", in.SMILES)
				assert.Equal(t, "substructure", in.SearchType)
				assert.Equal(t, 100, in.MaxResults)
				return &molecule.SearchResult{}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"smiles": "CCO"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/structure", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchByStructure(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestMoleculeHandler_SearchBySimilarity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMoleculeService{
			searchBySimilarityFn: func(_ context.Context, in *molecule.SimilaritySearchInput) (*molecule.SearchResult, error) {
				assert.Equal(t, "CCO", in.SMILES)
				assert.InDelta(t, 0.8, in.Threshold, 0.01)
				return &molecule.SearchResult{}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"smiles": "CCO", "similarity_threshold": 0.8, "max_results": 50})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/similarity", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchBySimilarity(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing smiles", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]float64{"similarity_threshold": 0.8})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/similarity", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchBySimilarity(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("defaults", func(t *testing.T) {
		svc := &mockMoleculeService{
			searchBySimilarityFn: func(_ context.Context, in *molecule.SimilaritySearchInput) (*molecule.SearchResult, error) {
				assert.Equal(t, "CCO", in.SMILES)
				assert.InDelta(t, 0.7, in.Threshold, 0.01)
				assert.Equal(t, 100, in.MaxResults)
				return &molecule.SearchResult{}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"smiles": "CCO"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/similarity", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.SearchBySimilarity(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestMoleculeHandler_CalculateProperties(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMoleculeService{
			calcPropsFn: func(_ context.Context, in *molecule.CalculatePropertiesInput) (*molecule.PropertiesResult, error) {
				assert.Equal(t, "CCO", in.SMILES)
				assert.Equal(t, []string{"mw", "logp"}, in.Properties)
				return &molecule.PropertiesResult{SMILES: "CCO", Properties: map[string]interface{}{"mw": 46.07}}, nil
			},
		}
		h := NewMoleculeHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"smiles": "CCO", "properties": []string{"mw", "logp"}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/properties/calculate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CalculateProperties(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp molecule.PropertiesResult
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "CCO", resp.SMILES)
	})

	t.Run("missing smiles", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"properties": []string{"mw"}})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/properties/calculate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CalculateProperties(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		h := NewMoleculeHandler(&mockMoleculeService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/properties/calculate", bytes.NewReader([]byte("")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.CalculateProperties(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

//Personal.AI order the ending
