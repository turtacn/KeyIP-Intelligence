// Phase 11 - File 269: internal/interfaces/http/handlers/portfolio_handler_test.go
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

	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type mockPortfolioService struct{ mock.Mock }

func (m *mockPortfolioService) Create(ctx context.Context, input *portfolio.CreateInput) (*portfolio.Output, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.Output), args.Error(1)
}
func (m *mockPortfolioService) GetByID(ctx context.Context, id string) (*portfolio.Output, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.Output), args.Error(1)
}
func (m *mockPortfolioService) List(ctx context.Context, input *portfolio.ListInput) (*portfolio.ListOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.ListOutput), args.Error(1)
}
func (m *mockPortfolioService) Update(ctx context.Context, input *portfolio.UpdateInput) (*portfolio.Output, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.Output), args.Error(1)
}
func (m *mockPortfolioService) Delete(ctx context.Context, id, userID string) error {
	return m.Called(ctx, id, userID).Error(0)
}
func (m *mockPortfolioService) AddPatents(ctx context.Context, id string, patentIDs []string, userID string) error {
	return m.Called(ctx, id, patentIDs, userID).Error(0)
}
func (m *mockPortfolioService) RemovePatents(ctx context.Context, id string, patentIDs []string, userID string) error {
	return m.Called(ctx, id, patentIDs, userID).Error(0)
}
func (m *mockPortfolioService) GetAnalysis(ctx context.Context, id string) (*portfolio.AnalysisOutput, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.AnalysisOutput), args.Error(1)
}

func newTestPortfolioHandler() (*PortfolioHandler, *mockPortfolioService) {
	svc := new(mockPortfolioService)
	logger := new(mockTestLogger)
	return NewPortfolioHandler(svc, logger), svc
}

func TestCreatePortfolio_Success(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	expected := &portfolio.Output{ID: "pf-001", Name: "My Portfolio"}
	svc.On("Create", mock.Anything, mock.AnythingOfType("*portfolio.CreateInput")).Return(expected, nil)

	body, _ := json.Marshal(CreatePortfolioRequest{Name: "My Portfolio"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios", bytes.NewReader(body))
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()
	h.CreatePortfolio(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	svc.AssertExpectations(t)
}

func TestCreatePortfolio_MissingName(t *testing.T) {
	h, _ := newTestPortfolioHandler()
	body, _ := json.Marshal(CreatePortfolioRequest{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.CreatePortfolio(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetPortfolio_Success(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	expected := &portfolio.Output{ID: "pf-001"}
	svc.On("GetByID", mock.Anything, "pf-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/pf-001", nil)
	req.SetPathValue("id", "pf-001")
	rec := httptest.NewRecorder()
	h.GetPortfolio(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetPortfolio_NotFound(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	svc.On("GetByID", mock.Anything, "pf-999").Return(nil, errors.NewNotFoundError("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/pf-999", nil)
	req.SetPathValue("id", "pf-999")
	rec := httptest.NewRecorder()
	h.GetPortfolio(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListPortfolios_Success(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	expected := &portfolio.ListOutput{Items: []*portfolio.Output{{ID: "pf-001"}}, Total: 1}
	svc.On("List", mock.Anything, mock.AnythingOfType("*portfolio.ListInput")).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios", nil)
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()
	h.ListPortfolios(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestDeletePortfolio_Success(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	svc.On("Delete", mock.Anything, "pf-001", "user-001").Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/portfolios/pf-001", nil)
	req.SetPathValue("id", "pf-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()
	h.DeletePortfolio(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestAddPatents_Success(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	svc.On("AddPatents", mock.Anything, "pf-001", []string{"pat-001", "pat-002"}, "user-001").Return(nil)

	body, _ := json.Marshal(AddPatentsRequest{PatentIDs: []string{"pat-001", "pat-002"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-001/patents", bytes.NewReader(body))
	req.SetPathValue("id", "pf-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()
	h.AddPatents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestAddPatents_EmptyIDs(t *testing.T) {
	h, _ := newTestPortfolioHandler()
	body, _ := json.Marshal(AddPatentsRequest{PatentIDs: []string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portfolios/pf-001/patents", bytes.NewReader(body))
	req.SetPathValue("id", "pf-001")
	rec := httptest.NewRecorder()
	h.AddPatents(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRemovePatents_Success(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	svc.On("RemovePatents", mock.Anything, "pf-001", []string{"pat-001"}, "user-001").Return(nil)

	body, _ := json.Marshal(RemovePatentsRequest{PatentIDs: []string{"pat-001"}})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/portfolios/pf-001/patents", bytes.NewReader(body))
	req.SetPathValue("id", "pf-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()
	h.RemovePatents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetAnalysis_Success(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	expected := &portfolio.AnalysisOutput{PortfolioID: "pf-001", TotalPatents: 10}
	svc.On("GetAnalysis", mock.Anything, "pf-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/pf-001/analysis", nil)
	req.SetPathValue("id", "pf-001")
	rec := httptest.NewRecorder()
	h.GetAnalysis(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetAnalysis_NotFound(t *testing.T) {
	h, svc := newTestPortfolioHandler()
	svc.On("GetAnalysis", mock.Anything, "pf-999").Return(nil, errors.NewNotFoundError("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolios/pf-999/analysis", nil)
	req.SetPathValue("id", "pf-999")
	rec := httptest.NewRecorder()
	h.GetAnalysis(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

//Personal.AI order the ending
