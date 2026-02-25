// Phase 11 - File 265: internal/interfaces/http/handlers/molecule_handler_test.go
// 实现分子 HTTP Handler 单元测试。
//
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/molecule"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type mockMoleculeService struct {
	mock.Mock
}

func (m *mockMoleculeService) Create(ctx context.Context, input *molecule.CreateInput) (*molecule.Output, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Output), args.Error(1)
}

func (m *mockMoleculeService) GetByID(ctx context.Context, id string) (*molecule.Output, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Output), args.Error(1)
}

func (m *mockMoleculeService) List(ctx context.Context, input *molecule.ListInput) (*molecule.ListOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.ListOutput), args.Error(1)
}

func (m *mockMoleculeService) Update(ctx context.Context, input *molecule.UpdateInput) (*molecule.Output, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.Output), args.Error(1)
}

func (m *mockMoleculeService) Delete(ctx context.Context, id, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *mockMoleculeService) SearchByStructure(ctx context.Context, input *molecule.StructureSearchInput) (*molecule.SearchOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.SearchOutput), args.Error(1)
}

func (m *mockMoleculeService) SearchBySimilarity(ctx context.Context, input *molecule.SimilaritySearchInput) (*molecule.SearchOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.SearchOutput), args.Error(1)
}

func (m *mockMoleculeService) CalculateProperties(ctx context.Context, input *molecule.CalculatePropertiesInput) (*molecule.PropertiesOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*molecule.PropertiesOutput), args.Error(1)
}

func newTestMoleculeHandler() (*MoleculeHandler, *mockMoleculeService) {
	svc := new(mockMoleculeService)
	logger := new(mockTestLogger)
	return NewMoleculeHandler(svc, logger), svc
}

func TestCreateMolecule_Success(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	expected := &molecule.Output{ID: "mol-001", SMILES: "CCO"}
	svc.On("Create", mock.Anything, mock.AnythingOfType("*molecule.CreateInput")).Return(expected, nil)

	body, _ := json.Marshal(CreateMoleculeRequest{Name: "Ethanol", SMILES: "CCO"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules", bytes.NewReader(body))
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.CreateMolecule(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	svc.AssertExpectations(t)
}

func TestCreateMolecule_MissingSMILES(t *testing.T) {
	h, _ := newTestMoleculeHandler()

	body, _ := json.Marshal(CreateMoleculeRequest{Name: "Ethanol"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.CreateMolecule(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetMolecule_Success(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	expected := &molecule.Output{ID: "mol-001", SMILES: "CCO"}
	svc.On("GetByID", mock.Anything, "mol-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules/mol-001", nil)
	req.SetPathValue("id", "mol-001")
	rec := httptest.NewRecorder()

	h.GetMolecule(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetMolecule_NotFound(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	svc.On("GetByID", mock.Anything, "mol-999").Return(nil, errors.NewNotFoundError("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules/mol-999", nil)
	req.SetPathValue("id", "mol-999")
	rec := httptest.NewRecorder()

	h.GetMolecule(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	svc.AssertExpectations(t)
}

func TestListMolecules_Success(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	expected := &molecule.ListOutput{Items: []*molecule.Output{{ID: "mol-001"}}, Total: 1}
	svc.On("List", mock.Anything, mock.AnythingOfType("*molecule.ListInput")).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/molecules?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()

	h.ListMolecules(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestDeleteMolecule_Success(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	svc.On("Delete", mock.Anything, "mol-001", "user-001").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/molecules/mol-001", nil)
	req.SetPathValue("id", "mol-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.DeleteMolecule(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestSearchByStructure_Success(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	expected := &molecule.SearchOutput{Items: []*molecule.Output{{ID: "mol-001"}}}
	svc.On("SearchByStructure", mock.Anything, mock.AnythingOfType("*molecule.StructureSearchInput")).Return(expected, nil)

	body, _ := json.Marshal(StructureSearchRequest{SMILES: "c1ccccc1", SearchType: "substructure"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/structure", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SearchByStructure(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestSearchByStructure_MissingSMILES(t *testing.T) {
	h, _ := newTestMoleculeHandler()

	body, _ := json.Marshal(StructureSearchRequest{SearchType: "substructure"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/structure", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SearchByStructure(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearchBySimilarity_Success(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	expected := &molecule.SearchOutput{Items: []*molecule.Output{{ID: "mol-002"}}}
	svc.On("SearchBySimilarity", mock.Anything, mock.AnythingOfType("*molecule.SimilaritySearchInput")).Return(expected, nil)

	body, _ := json.Marshal(SimilaritySearchRequest{SMILES: "CCO", Threshold: 0.8})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/search/similarity", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.SearchBySimilarity(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestCalculateProperties_Success(t *testing.T) {
	h, svc := newTestMoleculeHandler()

	expected := &molecule.PropertiesOutput{SMILES: "CCO", Properties: map[string]interface{}{"mw": 46.07}}
	svc.On("CalculateProperties", mock.Anything, mock.AnythingOfType("*molecule.CalculatePropertiesInput")).Return(expected, nil)

	body, _ := json.Marshal(CalculatePropertiesRequest{SMILES: "CCO", Properties: []string{"mw"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/properties/calculate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.CalculateProperties(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestCalculateProperties_MissingSMILES(t *testing.T) {
	h, _ := newTestMoleculeHandler()

	body, _ := json.Marshal(CalculatePropertiesRequest{Properties: []string{"mw"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/molecules/properties/calculate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.CalculateProperties(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

//Personal.AI order the ending
