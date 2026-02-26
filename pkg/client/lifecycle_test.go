// Phase 13 - SDK Lifecycle Client Tests (295/349)
// File: pkg/client/lifecycle_test.go
// Unit tests for lifecycle sub-client.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	keyiperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func newTestLifecycleClient(t *testing.T, handler http.HandlerFunc) *LifecycleClient {
	c := newTestClient(t, handler)
	return c.Lifecycle()
}

func TestLifecycle_GetDeadlines(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/lifecycle/deadlines", r.URL.Path)
		assert.Equal(t, "90", r.URL.Query().Get("days_ahead"))
		assert.Equal(t, "CN1,US1", r.URL.Query().Get("patent_numbers"))
		assert.Equal(t, "CN,US", r.URL.Query().Get("jurisdictions"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]Deadline]{
			Data: []Deadline{{ID: "d1"}},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.GetDeadlines(context.Background(), &DeadlineQuery{
		PatentNumbers: []string{"CN1", "US1"},
		Jurisdictions: []string{"CN", "US"},
		DaysAhead:     90,
	})
	require.NoError(t, err)
	assert.Len(t, res, 1)
	assert.Equal(t, "d1", res[0].ID)
}

func TestLifecycle_GetDeadlines_NilQuery(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "90", r.URL.Query().Get("days_ahead")) // default
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]Deadline]{
			Data: []Deadline{},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.GetDeadlines(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestLifecycle_GetDeadline(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/lifecycle/deadlines/d1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[Deadline]{
			Data: Deadline{ID: "d1"},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.GetDeadline(context.Background(), "d1")
	require.NoError(t, err)
	assert.Equal(t, "d1", res.ID)
}

func TestLifecycle_GetAnnuities(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]AnnuityRecord]{
			Data: []AnnuityRecord{{ID: "a1"}},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.GetAnnuities(context.Background(), "US1")
	require.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestLifecycle_GetAnnuitySummary(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[AnnuitySummary]{
			Data: AnnuitySummary{TotalDue: 100},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.GetAnnuitySummary(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 100, res.TotalDue)
}

func TestLifecycle_RecordAnnuityPayment(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		var req AnnuityPayRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "a1", req.AnnuityID)
		assert.Equal(t, 100.0, req.Amount)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[AnnuityRecord]{
			Data: AnnuityRecord{ID: "a1", Status: "paid"},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{
		AnnuityID: "a1", PaidDate: "2024-01-01", Amount: 100.0,
	})
	require.NoError(t, err)
	assert.Equal(t, "paid", res.Status)

	// Validation
	_, err = lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{AnnuityID: ""})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestLifecycle_GetLegalStatus(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[LegalStatusRecord]{
			Data: LegalStatusRecord{CurrentStatus: "active"},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.GetLegalStatus(context.Background(), "US1")
	require.NoError(t, err)
	assert.Equal(t, "active", res.CurrentStatus)
}

func TestLifecycle_SyncLegalStatus(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusAccepted)
	}
	lc := newTestLifecycleClient(t, handler)
	err := lc.SyncLegalStatus(context.Background(), []string{"US1"})
	require.NoError(t, err)

	// Validation
	err = lc.SyncLegalStatus(context.Background(), nil)
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestLifecycle_GetReminders(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]ReminderConfig]{
			Data: []ReminderConfig{{ID: "r1"}},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.GetReminders(context.Background(), "US1")
	require.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestLifecycle_SetReminder(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[ReminderConfig]{
			Data: ReminderConfig{ID: "r1"},
		})
	}
	lc := newTestLifecycleClient(t, handler)
	res, err := lc.SetReminder(context.Background(), &ReminderConfigRequest{
		PatentNumber: "US1",
		ReminderDays: []int{7},
		Channels: []string{"email"},
	})
	require.NoError(t, err)
	assert.Equal(t, "r1", res.ID)

	// Validation
	_, err = lc.SetReminder(context.Background(), &ReminderConfigRequest{})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestLifecycle_DeleteReminder(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}
	lc := newTestLifecycleClient(t, handler)
	err := lc.DeleteReminder(context.Background(), "r1")
	require.NoError(t, err)

	// Validation
	err = lc.DeleteReminder(context.Background(), "")
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

//Personal.AI order the ending
