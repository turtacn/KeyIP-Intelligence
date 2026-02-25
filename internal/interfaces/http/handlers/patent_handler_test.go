// Phase 11 - File 267: internal/interfaces/http/handlers/patent_handler_test.go
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

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type mockPatentService struct{ mock.Mock }

func (m *mockPatentService) Create(ctx context.Context, input *patent.CreateInput) (*patent.Output, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.Output), args.Error(1)
}
func (m *mockPatentService) GetByID(ctx context.Context, id string) (*patent.Output, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.Output), args.Error(1)
}
func (m *mockPatentService) List(ctx context.Context, input *patent.ListInput) (*patent.ListOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.ListOutput), args.Error(1)
}
func (m *mockPatentService) Update(ctx context.Context, input *patent.UpdateInput) (*patent.Output, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.Output), args.Error(1)
}
func (m *mockPatentService) Delete(ctx context.Context, id, userID string) error {
	return m.Called(ctx, id, userID).Error(0)
}
func (m *mockPatentService) Search(ctx context.Context, input *patent.SearchInput) (*patent.SearchOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.SearchOutput), args.Error(1)
}
func (m *mockPatentService) AdvancedSearch(ctx context.Context, input *patent.AdvancedSearchInput) (*patent.SearchOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.SearchOutput), args.Error(1)
}
func (m *mockPatentService) GetStats(ctx context.Context, input *patent.StatsInput) (*patent.StatsOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.StatsOutput), args.Error(1)
}

func newTestPatentHandler() (*PatentHandler, *mockPatentService) {
	svc := new(mockPatentService)
	logger := new(mockTestLogger)
	return NewPatentHandler(svc, logger), svc
}

func TestCreatePatent_Success(t *testing.T) {
	h, svc := newTestPatentHandler()
	expected := &patent.Output{ID: "pat-001", Title: "Test Patent"}
	svc.On("Create", mock.Anything, mock.AnythingOfType("*patent.CreateInput")).Return(expected, nil)

	body, _ := json.Marshal(CreatePatentRequest{Title: "Test Patent", ApplicationNo: "CN202401234567", Jurisdiction: "CN"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents", bytes.NewReader(body))
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()
	h.CreatePatent(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	svc.AssertExpectations(t)
}

func TestCreatePatent_MissingTitle(t *testing.T) {
	h, _ := newTestPatentHandler()
	body, _ := json.Marshal(CreatePatentRequest{ApplicationNo: "CN202401234567"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.CreatePatent(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetPatent_Success(t *testing.T) {
	h, svc := newTestPatentHandler()
	expected := &patent.Output{ID: "pat-001"}
	svc.On("GetByID", mock.Anything, "pat-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-001", nil)
	req.SetPathValue("id", "pat-001")
	rec := httptest.NewRecorder()
	h.GetPatent(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetPatent_NotFound(t *testing.T) {
	h, svc := newTestPatentHandler()
	svc.On("GetByID", mock.Anything, "pat-999").Return(nil, errors.NewNotFoundError("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-999", nil)
	req.SetPathValue("id", "pat-999")
	rec := httptest.NewRecorder()
	h.GetPatent(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListPatents_Success(t *testing.T) {
	h, svc := newTestPatentHandler()
	expected := &patent.ListOutput{Items: []*patent.Output{{ID: "pat-001"}}, Total: 1}
	svc.On("List", mock.Anything, mock.AnythingOfType("*patent.ListInput")).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents?page=1", nil)
	rec := httptest.NewRecorder()
	h.ListPatents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestDeletePatent_Success(t *testing.T) {
	h, svc := newTestPatentHandler()
	svc.On("Delete", mock.Anything, "pat-001", "user-001").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/patents/pat-001", nil)
	req.SetPathValue("id", "pat-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()
	h.DeletePatent(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestSearchPatents_Success(t *testing.T) {
	h, svc := newTestPatentHandler()
	expected := &patent.SearchOutput{Items: []*patent.Output{{ID: "pat-001"}}, Total: 1}
	svc.On("Search", mock.Anything, mock.AnythingOfType("*patent.SearchInput")).Return(expected, nil)

	body, _ := json.Marshal(SearchPatentsRequest{Query: "CRISPR", Page: 1, PageSize: 10})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.SearchPatents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestSearchPatents_MissingQuery(t *testing.T) {
	h, _ := newTestPatentHandler()
	body, _ := json.Marshal(SearchPatentsRequest{Page: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.SearchPatents(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdvancedSearch_Success(t *testing.T) {
	h, svc := newTestPatentHandler()
	expected := &patent.SearchOutput{Items: []*patent.Output{{ID: "pat-002"}}, Total: 1}
	svc.On("AdvancedSearch", mock.Anything, mock.AnythingOfType("*patent.AdvancedSearchInput")).Return(expected, nil)

	body, _ := json.Marshal(AdvancedSearchRequest{Applicant: "Pharma Corp", Jurisdiction: "CN"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/search/advanced", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.AdvancedSearch(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetPatentStats_Success(t *testing.T) {
	h, svc := newTestPatentHandler()
	expected := &patent.StatsOutput{TotalPatents: 100}
	svc.On("GetStats", mock.Anything, mock.AnythingOfType("*patent.StatsInput")).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/stats?jurisdiction=CN", nil)
	rec := httptest.NewRecorder()
	h.GetPatentStats(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

//Personal.AI order the ending
