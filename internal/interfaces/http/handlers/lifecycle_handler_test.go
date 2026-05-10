// Real tests for lifecycle HTTP handler.
// Tests request parsing, validation, error responses, and successful responses.

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// mockLifecycleService implements lifecycle.TrackingService for testing.
type mockLifecycleService struct {
	getLifecycleFn         func(context.Context, string) (*lifecycle.Lifecycle, error)
	advancePhaseFn         func(context.Context, *lifecycle.AdvancePhaseInput) (*lifecycle.Lifecycle, error)
	addMilestoneFn         func(context.Context, *lifecycle.AddMilestoneInput) (*lifecycle.Milestone, error)
	listMilestonesFn       func(context.Context, string) (*lifecycle.MilestoneList, error)
	recordFeeFn            func(context.Context, *lifecycle.RecordFeeInput) (*lifecycle.Fee, error)
	listFeesFn             func(context.Context, string) (*lifecycle.FeeList, error)
	getTimelineFn          func(context.Context, string) (*lifecycle.Timeline, error)
	getUpcomingDeadlinesFn func(context.Context, *lifecycle.UpcomingDeadlinesInput) ([]*lifecycle.DeadlineInfo, error)
}

func (m *mockLifecycleService) GetLifecycle(ctx context.Context, patentID string) (*lifecycle.Lifecycle, error) {
	return m.getLifecycleFn(ctx, patentID)
}
func (m *mockLifecycleService) AdvancePhase(ctx context.Context, in *lifecycle.AdvancePhaseInput) (*lifecycle.Lifecycle, error) {
	return m.advancePhaseFn(ctx, in)
}
func (m *mockLifecycleService) AddMilestone(ctx context.Context, in *lifecycle.AddMilestoneInput) (*lifecycle.Milestone, error) {
	return m.addMilestoneFn(ctx, in)
}
func (m *mockLifecycleService) ListMilestones(ctx context.Context, patentID string) (*lifecycle.MilestoneList, error) {
	return m.listMilestonesFn(ctx, patentID)
}
func (m *mockLifecycleService) RecordFee(ctx context.Context, in *lifecycle.RecordFeeInput) (*lifecycle.Fee, error) {
	return m.recordFeeFn(ctx, in)
}
func (m *mockLifecycleService) ListFees(ctx context.Context, patentID string) (*lifecycle.FeeList, error) {
	return m.listFeesFn(ctx, patentID)
}
func (m *mockLifecycleService) GetTimeline(ctx context.Context, patentID string) (*lifecycle.Timeline, error) {
	return m.getTimelineFn(ctx, patentID)
}
func (m *mockLifecycleService) GetUpcomingDeadlines(ctx context.Context, in *lifecycle.UpcomingDeadlinesInput) ([]*lifecycle.DeadlineInfo, error) {
	return m.getUpcomingDeadlinesFn(ctx, in)
}

func TestLifecycleHandler_GetLifecycle(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			getLifecycleFn: func(_ context.Context, patentID string) (*lifecycle.Lifecycle, error) {
				assert.Equal(t, "pat-1", patentID)
				return &lifecycle.Lifecycle{PatentID: "pat-1", Phase: "filing", Status: "active"}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-1/lifecycle", nil)
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.GetLifecycle(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp lifecycle.Lifecycle
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pat-1", resp.PatentID)
		assert.Equal(t, "filing", resp.Phase)
	})

	t.Run("missing patentId", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents//lifecycle", nil)
		rec := httptest.NewRecorder()

		h.GetLifecycle(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		svc := &mockLifecycleService{
			getLifecycleFn: func(_ context.Context, _ string) (*lifecycle.Lifecycle, error) {
				return nil, errors.NewNotFound("patent not found")
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/nonexistent/lifecycle", nil)
		req.SetPathValue("patentId", "nonexistent")
		rec := httptest.NewRecorder()

		h.GetLifecycle(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestLifecycleHandler_AdvancePhase(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			advancePhaseFn: func(_ context.Context, in *lifecycle.AdvancePhaseInput) (*lifecycle.Lifecycle, error) {
				assert.Equal(t, "pat-1", in.PatentID)
				assert.Equal(t, "examination", in.NewPhase)
				return &lifecycle.Lifecycle{PatentID: "pat-1", Phase: "examination"}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"target_phase": "examination", "notes": "filed"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/lifecycle/advance", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.AdvancePhase(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp lifecycle.Lifecycle
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "examination", resp.Phase)
	})

	t.Run("missing patentId", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"target_phase": "examination"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents//lifecycle/advance", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AdvancePhase(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing target_phase", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"notes": "test"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/lifecycle/advance", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.AdvancePhase(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/lifecycle/advance", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("patentId", "pat-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AdvancePhase(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_AddMilestone(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			addMilestoneFn: func(_ context.Context, in *lifecycle.AddMilestoneInput) (*lifecycle.Milestone, error) {
				assert.Equal(t, "pat-1", in.PatentID)
				assert.Equal(t, "filing", in.Type)
				return &lifecycle.Milestone{ID: "ms-1", PatentID: "pat-1", Type: "filing"}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"title": "Filed", "type": "filing", "description": "done"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/milestones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.AddMilestone(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp lifecycle.Milestone
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "ms-1", resp.ID)
	})

	t.Run("missing title", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"type": "filing"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/milestones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.AddMilestone(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing patentId", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]string{"title": "Filed", "type": "filing"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents//milestones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.AddMilestone(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_ListMilestones(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			listMilestonesFn: func(_ context.Context, patentID string) (*lifecycle.MilestoneList, error) {
				assert.Equal(t, "pat-1", patentID)
				return &lifecycle.MilestoneList{
					Milestones: []*lifecycle.Milestone{{ID: "ms-1", Type: "filing"}},
					Total:      1,
				}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-1/milestones", nil)
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.ListMilestones(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp lifecycle.MilestoneList
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp.Milestones, 1)
	})

	t.Run("missing patentId", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents//milestones", nil)
		rec := httptest.NewRecorder()

		h.ListMilestones(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_RecordFee(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			recordFeeFn: func(_ context.Context, in *lifecycle.RecordFeeInput) (*lifecycle.Fee, error) {
				assert.Equal(t, "pat-1", in.PatentID)
				assert.Equal(t, "filing", in.Type)
				assert.Equal(t, 250.0, in.Amount)
				return &lifecycle.Fee{ID: "fee-1", PatentID: "pat-1", Type: "filing", Amount: 250}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"fee_type": "filing", "amount": 250.0})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/fees", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.RecordFee(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		var resp lifecycle.Fee
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "fee-1", resp.ID)
	})

	t.Run("missing fee_type", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"amount": 250.0})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/fees", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.RecordFee(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("zero amount", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"fee_type": "filing", "amount": 0})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/fees", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.RecordFee(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("invalid body", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents/pat-1/fees", bytes.NewReader([]byte("{bad")))
		req.SetPathValue("patentId", "pat-1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.RecordFee(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("missing patentId", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		body, _ := json.Marshal(map[string]interface{}{"fee_type": "filing", "amount": 250.0})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/patents//fees", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		h.RecordFee(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_ListFees(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			listFeesFn: func(_ context.Context, patentID string) (*lifecycle.FeeList, error) {
				assert.Equal(t, "pat-1", patentID)
				return &lifecycle.FeeList{
					Fees:  []*lifecycle.Fee{{ID: "fee-1", Type: "filing", Amount: 250}},
					Total: 1,
				}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-1/fees", nil)
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.ListFees(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp lifecycle.FeeList
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp.Fees, 1)
	})

	t.Run("missing patentId", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents//fees", nil)
		rec := httptest.NewRecorder()

		h.ListFees(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_GetTimeline(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			getTimelineFn: func(_ context.Context, patentID string) (*lifecycle.Timeline, error) {
				assert.Equal(t, "pat-1", patentID)
				return &lifecycle.Timeline{PatentID: "pat-1", Events: []*lifecycle.TimelineEvent{}}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents/pat-1/timeline", nil)
		req.SetPathValue("patentId", "pat-1")
		rec := httptest.NewRecorder()

		h.GetTimeline(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("missing patentId", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patents//timeline", nil)
		rec := httptest.NewRecorder()

		h.GetTimeline(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_GetUpcomingDeadlines(t *testing.T) {
	t.Run("success with default days", func(t *testing.T) {
		svc := &mockLifecycleService{
			getUpcomingDeadlinesFn: func(_ context.Context, in *lifecycle.UpcomingDeadlinesInput) ([]*lifecycle.DeadlineInfo, error) {
				assert.Equal(t, 30, in.Days)
				return []*lifecycle.DeadlineInfo{
					{PatentID: "pat-1", Type: "maintenance", DueDate: mustParseTime("2026-06-01")},
				}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/deadlines/upcoming", nil)
		rec := httptest.NewRecorder()

		h.GetUpcomingDeadlines(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp []*lifecycle.DeadlineInfo
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Len(t, resp, 1)
	})

	t.Run("with custom days", func(t *testing.T) {
		svc := &mockLifecycleService{
			getUpcomingDeadlinesFn: func(_ context.Context, in *lifecycle.UpcomingDeadlinesInput) ([]*lifecycle.DeadlineInfo, error) {
				assert.Equal(t, 60, in.Days)
				return []*lifecycle.DeadlineInfo{}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/deadlines/upcoming?days=60", nil)
		rec := httptest.NewRecorder()

		h.GetUpcomingDeadlines(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("invalid days param uses default", func(t *testing.T) {
		svc := &mockLifecycleService{
			getUpcomingDeadlinesFn: func(_ context.Context, in *lifecycle.UpcomingDeadlinesInput) ([]*lifecycle.DeadlineInfo, error) {
				assert.Equal(t, 30, in.Days)
				return []*lifecycle.DeadlineInfo{}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/deadlines/upcoming?days=-1", nil)
		rec := httptest.NewRecorder()

		h.GetUpcomingDeadlines(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("service error", func(t *testing.T) {
		svc := &mockLifecycleService{
			getUpcomingDeadlinesFn: func(_ context.Context, _ *lifecycle.UpcomingDeadlinesInput) ([]*lifecycle.DeadlineInfo, error) {
				return nil, errors.NewInternal("db error")
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/deadlines/upcoming", nil)
		rec := httptest.NewRecorder()

		h.GetUpcomingDeadlines(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestLifecycleHandler_CalculateAnnuities(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			listFeesFn: func(_ context.Context, id string) (*lifecycle.FeeList, error) {
				assert.Equal(t, "pat-1", id)
				return &lifecycle.FeeList{
					Fees:  []*lifecycle.Fee{{ID: "fee-1", Type: "annuity", Amount: 500}},
					Total: 1,
				}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle/pat-1/annuities/calculate", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.CalculateAnnuities(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pat-1", resp["patent_id"])
		assert.Equal(t, "annuity calculation completed", resp["message"])
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle//annuities/calculate", nil)
		rec := httptest.NewRecorder()

		h.CalculateAnnuities(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_GetAnnuityBudget(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			listFeesFn: func(_ context.Context, id string) (*lifecycle.FeeList, error) {
				return &lifecycle.FeeList{
					Fees:  []*lifecycle.Fee{{ID: "fee-1", Type: "annuity", Amount: 500}},
					Total: 1,
				}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lifecycle/pat-1/annuities/budget", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.GetAnnuityBudget(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pat-1", resp["patent_id"])
		assert.Equal(t, float64(500), resp["total_amount"])
	})
}

func TestLifecycleHandler_SyncLegalStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			getLifecycleFn: func(_ context.Context, id string) (*lifecycle.Lifecycle, error) {
				assert.Equal(t, "pat-1", id)
				return &lifecycle.Lifecycle{PatentID: "pat-1", Phase: "granted", Status: "active"}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle/pat-1/legal-status/sync", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.SyncLegalStatus(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pat-1", resp["patent_id"])
		assert.Equal(t, "granted", resp["phase"])
		assert.Equal(t, "active", resp["status"])
		assert.NotEmpty(t, resp["synced_at"])
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lifecycle//legal-status/sync", nil)
		rec := httptest.NewRecorder()

		h.SyncLegalStatus(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestLifecycleHandler_ExportCalendar(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockLifecycleService{
			getTimelineFn: func(_ context.Context, id string) (*lifecycle.Timeline, error) {
				assert.Equal(t, "pat-1", id)
				return &lifecycle.Timeline{
					PatentID: "pat-1",
					Events:   []*lifecycle.TimelineEvent{{Type: "filing", Description: "filed"}},
				}, nil
			},
		}
		h := NewLifecycleHandler(svc, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lifecycle/pat-1/calendar/export", nil)
		req.SetPathValue("id", "pat-1")
		rec := httptest.NewRecorder()

		h.ExportCalendar(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		json.NewDecoder(rec.Body).Decode(&resp)
		assert.Equal(t, "pat-1", resp["patent_id"])
		assert.Equal(t, "calendar export completed", resp["message"])
	})

	t.Run("missing id", func(t *testing.T) {
		h := NewLifecycleHandler(&mockLifecycleService{}, testutil.NewNopLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lifecycle//calendar/export", nil)
		rec := httptest.NewRecorder()

		h.ExportCalendar(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

// mustParseTime is a helper for creating time.Time in tests.
func mustParseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
