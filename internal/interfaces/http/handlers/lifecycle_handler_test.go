// Phase 11 - File 263: internal/interfaces/http/handlers/lifecycle_handler_test.go
// 实现专利生命周期 HTTP Handler 单元测试。
//
// 测试用例：
//   - TestGetLifecycle_Success / NotFound
//   - TestAdvancePhase_Success / MissingPhase / InvalidBody
//   - TestAddMilestone_Success / MissingTitle
//   - TestListMilestones_Success
//   - TestRecordFee_Success / InvalidAmount
//   - TestListFees_Success
//   - TestGetTimeline_Success
//   - TestGetUpcomingDeadlines_Success
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

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// --- Mock Lifecycle Service ---

type mockLifecycleService struct {
	mock.Mock
}

func (m *mockLifecycleService) GetLifecycle(ctx context.Context, patentID string) (*lifecycle.LifecycleOutput, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.LifecycleOutput), args.Error(1)
}

func (m *mockLifecycleService) AdvancePhase(ctx context.Context, input *lifecycle.AdvancePhaseInput) (*lifecycle.LifecycleOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.LifecycleOutput), args.Error(1)
}

func (m *mockLifecycleService) AddMilestone(ctx context.Context, input *lifecycle.AddMilestoneInput) (*lifecycle.MilestoneOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.MilestoneOutput), args.Error(1)
}

func (m *mockLifecycleService) ListMilestones(ctx context.Context, patentID string) ([]*lifecycle.MilestoneOutput, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lifecycle.MilestoneOutput), args.Error(1)
}

func (m *mockLifecycleService) RecordFee(ctx context.Context, input *lifecycle.RecordFeeInput) (*lifecycle.FeeOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.FeeOutput), args.Error(1)
}

func (m *mockLifecycleService) ListFees(ctx context.Context, patentID string) ([]*lifecycle.FeeOutput, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lifecycle.FeeOutput), args.Error(1)
}

func (m *mockLifecycleService) GetTimeline(ctx context.Context, patentID string) (*lifecycle.TimelineOutput, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.TimelineOutput), args.Error(1)
}

func (m *mockLifecycleService) GetUpcomingDeadlines(ctx context.Context, input *lifecycle.UpcomingDeadlinesInput) ([]*lifecycle.DeadlineOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*lifecycle.DeadlineOutput), args.Error(1)
}

func newTestLifecycleHandler() (*LifecycleHandler, *mockLifecycleService) {
	svc := new(mockLifecycleService)
	logger := new(mockTestLogger)
	h := NewLifecycleHandler(svc, logger)
	return h, svc
}

func TestGetLifecycle_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := &lifecycle.LifecycleOutput{PatentID: "p-001", CurrentPhase: "examination"}
	svc.On("GetLifecycle", mock.Anything, "p-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/p-001/lifecycle", nil)
	req.SetPathValue("patentId", "p-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.GetLifecycle(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetLifecycle_NotFound(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	svc.On("GetLifecycle", mock.Anything, "p-999").Return(nil, errors.NewNotFoundError("not found"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/p-999/lifecycle", nil)
	req.SetPathValue("patentId", "p-999")
	rec := httptest.NewRecorder()

	h.GetLifecycle(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	svc.AssertExpectations(t)
}

func TestAdvancePhase_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := &lifecycle.LifecycleOutput{PatentID: "p-001", CurrentPhase: "grant"}
	svc.On("AdvancePhase", mock.Anything, mock.AnythingOfType("*lifecycle.AdvancePhaseInput")).Return(expected, nil)

	body, _ := json.Marshal(AdvancePhaseRequest{TargetPhase: "grant", Notes: "approved"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/p-001/lifecycle/advance", bytes.NewReader(body))
	req.SetPathValue("patentId", "p-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.AdvancePhase(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestAdvancePhase_MissingPhase(t *testing.T) {
	h, _ := newTestLifecycleHandler()

	body, _ := json.Marshal(AdvancePhaseRequest{Notes: "no phase"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/p-001/lifecycle/advance", bytes.NewReader(body))
	req.SetPathValue("patentId", "p-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.AdvancePhase(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdvancePhase_InvalidBody(t *testing.T) {
	h, _ := newTestLifecycleHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/p-001/lifecycle/advance", bytes.NewReader([]byte("bad")))
	req.SetPathValue("patentId", "p-001")
	rec := httptest.NewRecorder()

	h.AdvancePhase(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAddMilestone_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := &lifecycle.MilestoneOutput{ID: "ms-001", Title: "Filed"}
	svc.On("AddMilestone", mock.Anything, mock.AnythingOfType("*lifecycle.AddMilestoneInput")).Return(expected, nil)

	body, _ := json.Marshal(AddMilestoneRequest{Title: "Filed", Type: "filing", Date: "2024-01-15T00:00:00Z"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/p-001/milestones", bytes.NewReader(body))
	req.SetPathValue("patentId", "p-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.AddMilestone(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	svc.AssertExpectations(t)
}

func TestAddMilestone_MissingTitle(t *testing.T) {
	h, _ := newTestLifecycleHandler()

	body, _ := json.Marshal(AddMilestoneRequest{Type: "filing"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/p-001/milestones", bytes.NewReader(body))
	req.SetPathValue("patentId", "p-001")
	rec := httptest.NewRecorder()

	h.AddMilestone(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListMilestones_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := []*lifecycle.MilestoneOutput{{ID: "ms-001"}}
	svc.On("ListMilestones", mock.Anything, "p-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/p-001/milestones", nil)
	req.SetPathValue("patentId", "p-001")
	rec := httptest.NewRecorder()

	h.ListMilestones(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestRecordFee_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := &lifecycle.FeeOutput{ID: "fee-001"}
	svc.On("RecordFee", mock.Anything, mock.AnythingOfType("*lifecycle.RecordFeeInput")).Return(expected, nil)

	body, _ := json.Marshal(RecordFeeRequest{FeeType: "annual", Amount: 500.0, Currency: "CNY"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/p-001/fees", bytes.NewReader(body))
	req.SetPathValue("patentId", "p-001")
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.RecordFee(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	svc.AssertExpectations(t)
}

func TestRecordFee_InvalidAmount(t *testing.T) {
	h, _ := newTestLifecycleHandler()

	body, _ := json.Marshal(RecordFeeRequest{FeeType: "annual", Amount: -10})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/p-001/fees", bytes.NewReader(body))
	req.SetPathValue("patentId", "p-001")
	rec := httptest.NewRecorder()

	h.RecordFee(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListFees_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := []*lifecycle.FeeOutput{{ID: "fee-001"}}
	svc.On("ListFees", mock.Anything, "p-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/p-001/fees", nil)
	req.SetPathValue("patentId", "p-001")
	rec := httptest.NewRecorder()

	h.ListFees(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetTimeline_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := &lifecycle.TimelineOutput{PatentID: "p-001"}
	svc.On("GetTimeline", mock.Anything, "p-001").Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/p-001/timeline", nil)
	req.SetPathValue("patentId", "p-001")
	rec := httptest.NewRecorder()

	h.GetTimeline(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestGetUpcomingDeadlines_Success(t *testing.T) {
	h, svc := newTestLifecycleHandler()

	expected := []*lifecycle.DeadlineOutput{{PatentID: "p-001", Description: "Annual fee due"}}
	svc.On("GetUpcomingDeadlines", mock.Anything, mock.AnythingOfType("*lifecycle.UpcomingDeadlinesInput")).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/deadlines/upcoming?days=60", nil)
	req = withUserID(req, "user-001")
	rec := httptest.NewRecorder()

	h.GetUpcomingDeadlines(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	svc.AssertExpectations(t)
}

func TestParseInt_Valid(t *testing.T) {
	n, err := parseInt("42")
	assert.NoError(t, err)
	assert.Equal(t, 42, n)
}

func TestParseInt_Invalid(t *testing.T) {
	_, err := parseInt("abc")
	assert.Error(t, err)
}

//Personal.AI order the ending
