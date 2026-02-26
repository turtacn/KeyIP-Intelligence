// Phase 13 - SDK Lifecycle Management Sub-Client Test (295/349)
// File: pkg/client/lifecycle_test.go
// Comprehensive unit tests for LifecycleClient.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestLifecycleClient(t *testing.T, handler http.HandlerFunc) *LifecycleClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "test-key",
		WithHTTPClient(srv.Client()),
		WithRetryMax(0),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c.Lifecycle()
}

func lcReadBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal body: %v (raw: %s)", err, string(b))
	}
	return m
}

func lcWriteJSON(t *testing.T, w http.ResponseWriter, status int, v interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// GetDeadlines
// ---------------------------------------------------------------------------

func TestGetDeadlines_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		lcWriteJSON(t, w, 200, deadlineListResp{
			Data: []Deadline{
				{ID: "dl-1", PatentNumber: "CN115000001B", Jurisdiction: "CN", DeadlineType: "annuity_payment", DueDate: "2025-03-15", DaysRemaining: 30, Status: "pending", Priority: "high", Description: "Year 5 annuity"},
				{ID: "dl-2", PatentNumber: "US11000001B2", Jurisdiction: "US", DeadlineType: "oa_response", DueDate: "2025-04-01", DaysRemaining: 47, Status: "pending", Priority: "critical", Description: "OA response due"},
			},
		})
	})
	deadlines, err := lc.GetDeadlines(context.Background(), &DeadlineQuery{})
	if err != nil {
		t.Fatalf("GetDeadlines: %v", err)
	}
	if len(deadlines) != 2 {
		t.Fatalf("want 2 deadlines, got %d", len(deadlines))
	}
	if deadlines[0].ID != "dl-1" {
		t.Errorf("ID: want dl-1, got %s", deadlines[0].ID)
	}
	if deadlines[1].Priority != "critical" {
		t.Errorf("Priority: want critical, got %s", deadlines[1].Priority)
	}
}

func TestGetDeadlines_DefaultDaysAhead(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		da := r.URL.Query().Get("days_ahead")
		if da != "90" {
			t.Errorf("days_ahead: want 90, got %s", da)
		}
		lcWriteJSON(t, w, 200, deadlineListResp{Data: []Deadline{}})
	})
	_, _ = lc.GetDeadlines(context.Background(), &DeadlineQuery{DaysAhead: 0})
}

func TestGetDeadlines_WithFilters(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		pn := q.Get("patent_numbers")
		if !strings.Contains(pn, "CN115xxx") || !strings.Contains(pn, "US11xxx") {
			t.Errorf("patent_numbers: want CN115xxx,US11xxx, got %s", pn)
		}
		jur := q.Get("jurisdictions")
		if !strings.Contains(jur, "CN") || !strings.Contains(jur, "US") {
			t.Errorf("jurisdictions: want CN,US, got %s", jur)
		}
		lcWriteJSON(t, w, 200, deadlineListResp{Data: []Deadline{}})
	})
	_, _ = lc.GetDeadlines(context.Background(), &DeadlineQuery{
		PatentNumbers: []string{"CN115xxx", "US11xxx"},
		Jurisdictions: []string{"CN", "US"},
		DaysAhead:     60,
	})
}

func TestGetDeadlines_EmptyResult(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		lcWriteJSON(t, w, 200, deadlineListResp{Data: []Deadline{}})
	})
	deadlines, err := lc.GetDeadlines(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetDeadlines: %v", err)
	}
	if deadlines == nil {
		t.Fatal("want non-nil empty slice, got nil")
	}
	if len(deadlines) != 0 {
		t.Errorf("want 0 deadlines, got %d", len(deadlines))
	}
}

func TestGetDeadlines_ServerError(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		lcWriteJSON(t, w, 500, map[string]string{"code": "internal_error", "message": "boom"})
	})
	_, err := lc.GetDeadlines(context.Background(), &DeadlineQuery{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsServerError() {
		t.Error("IsServerError should be true")
	}
}

// ---------------------------------------------------------------------------
// GetDeadline
// ---------------------------------------------------------------------------

func TestGetDeadline_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/dl-abc") {
			t.Errorf("path: want suffix /dl-abc, got %s", r.URL.Path)
		}
		lcWriteJSON(t, w, 200, deadlineResp{Data: Deadline{
			ID: "dl-abc", PatentNumber: "CN115000001B", DeadlineType: "annuity_payment",
			DueDate: "2025-06-01", DaysRemaining: 120, Status: "pending", Priority: "medium",
		}})
	})
	dl, err := lc.GetDeadline(context.Background(), "dl-abc")
	if err != nil {
		t.Fatalf("GetDeadline: %v", err)
	}
	if dl.ID != "dl-abc" {
		t.Errorf("ID: want dl-abc, got %s", dl.ID)
	}
	if dl.DaysRemaining != 120 {
		t.Errorf("DaysRemaining: want 120, got %d", dl.DaysRemaining)
	}
}

func TestGetDeadline_EmptyID(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.GetDeadline(context.Background(), "")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestGetDeadline_NotFound(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		lcWriteJSON(t, w, 404, map[string]string{"code": "not_found", "message": "not found"})
	})
	_, err := lc.GetDeadline(context.Background(), "dl-999")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Error("IsNotFound should be true")
	}
}

// ---------------------------------------------------------------------------
// GetAnnuities
// ---------------------------------------------------------------------------

func TestGetAnnuities_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/annuities") {
			t.Errorf("path: want contains /annuities, got %s", r.URL.Path)
		}
		lcWriteJSON(t, w, 200, annuityListResp{Data: []AnnuityRecord{
			{ID: "an-1", PatentNumber: "CN115000001B", Year: 3, Amount: 900, Currency: "CNY", Status: "paid", PaidDate: "2024-12-01"},
			{ID: "an-2", PatentNumber: "CN115000001B", Year: 4, Amount: 1200, Currency: "CNY", Status: "due"},
			{ID: "an-3", PatentNumber: "CN115000001B", Year: 5, Amount: 2000, Currency: "CNY", Status: "due"},
		}})
	})
	records, err := lc.GetAnnuities(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetAnnuities: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 records, got %d", len(records))
	}
	if records[0].Status != "paid" {
		t.Errorf("Status: want paid, got %s", records[0].Status)
	}
	if records[1].Amount != 1200 {
		t.Errorf("Amount: want 1200, got %f", records[1].Amount)
	}
}

func TestGetAnnuities_EmptyPatentNumber(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.GetAnnuities(context.Background(), "")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetAnnuitySummary
// ---------------------------------------------------------------------------

func TestGetAnnuitySummary_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/lifecycle/annuities/summary" {
			t.Errorf("path: want /api/v1/lifecycle/annuities/summary, got %s", r.URL.Path)
		}
		lcWriteJSON(t, w, 200, annuitySummaryResp{Data: AnnuitySummary{
			TotalDue: 10, TotalPaid: 5, TotalOverdue: 2,
			TotalAmountDue: 50000, TotalAmountPaid: 25000, Currency: "CNY",
			ByJurisdiction: map[string]AnnuityJurisdictionSummary{
				"CN": {Jurisdiction: "CN", Due: 6, Paid: 3, Overdue: 1, AmountDue: 30000, AmountPaid: 15000, Currency: "CNY"},
				"US": {Jurisdiction: "US", Due: 4, Paid: 2, Overdue: 1, AmountDue: 20000, AmountPaid: 10000, Currency: "USD"},
			},
		}})
	})
	summary, err := lc.GetAnnuitySummary(context.Background())
	if err != nil {
		t.Fatalf("GetAnnuitySummary: %v", err)
	}
	if summary.TotalDue != 10 {
		t.Errorf("TotalDue: want 10, got %d", summary.TotalDue)
	}
	cn, ok := summary.ByJurisdiction["CN"]
	if !ok {
		t.Fatal("ByJurisdiction missing CN")
	}
	if cn.Due != 6 {
		t.Errorf("CN.Due: want 6, got %d", cn.Due)
	}
	us, ok := summary.ByJurisdiction["US"]
	if !ok {
		t.Fatal("ByJurisdiction missing US")
	}
	if us.Currency != "USD" {
		t.Errorf("US.Currency: want USD, got %s", us.Currency)
	}
}

// ---------------------------------------------------------------------------
// RecordAnnuityPayment
// ---------------------------------------------------------------------------

func TestRecordAnnuityPayment_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		lcWriteJSON(t, w, 200, annuityRecordResp{Data: AnnuityRecord{
			ID: "an-2", PatentNumber: "CN115000001B", Year: 4,
			Amount: 1200, Currency: "CNY", Status: "paid", PaidDate: "2025-01-15",
		}})
	})
	rec, err := lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{
		AnnuityID: "an-2", PaidDate: "2025-01-15", Amount: 1200, Currency: "CNY",
	})
	if err != nil {
		t.Fatalf("RecordAnnuityPayment: %v", err)
	}
	if rec.Status != "paid" {
		t.Errorf("Status: want paid, got %s", rec.Status)
	}
}

func TestRecordAnnuityPayment_EmptyAnnuityID(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{
		AnnuityID: "", PaidDate: "2025-01-15", Amount: 100,
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestRecordAnnuityPayment_EmptyPaidDate(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{
		AnnuityID: "an-1", PaidDate: "", Amount: 100,
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestRecordAnnuityPayment_ZeroAmount(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{
		AnnuityID: "an-1", PaidDate: "2025-01-15", Amount: 0,
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestRecordAnnuityPayment_NegativeAmount(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{
		AnnuityID: "an-1", PaidDate: "2025-01-15", Amount: -100,
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestRecordAnnuityPayment_NilRequest(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.RecordAnnuityPayment(context.Background(), nil)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestRecordAnnuityPayment_RequestBody(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := lcReadBody(t, r)
		if body["annuity_id"] != "an-99" {
			t.Errorf("annuity_id: want an-99, got %v", body["annuity_id"])
		}
		if body["paid_date"] != "2025-02-20" {
			t.Errorf("paid_date: want 2025-02-20, got %v", body["paid_date"])
		}
		if body["amount"] != float64(5500) {
			t.Errorf("amount: want 5500, got %v", body["amount"])
		}
		if body["currency"] != "USD" {
			t.Errorf("currency: want USD, got %v", body["currency"])
		}
		if body["receipt"] != "https://example.com/receipt.pdf" {
			t.Errorf("receipt: want URL, got %v", body["receipt"])
		}
		if body["notes"] != "Paid via wire transfer" {
			t.Errorf("notes: want 'Paid via wire transfer', got %v", body["notes"])
		}
		lcWriteJSON(t, w, 200, annuityRecordResp{Data: AnnuityRecord{ID: "an-99", Status: "paid"}})
	})
	_, _ = lc.RecordAnnuityPayment(context.Background(), &AnnuityPayRequest{
		AnnuityID: "an-99",
		PaidDate:  "2025-02-20",
		Amount:    5500,
		Currency:  "USD",
		Receipt:   "https://example.com/receipt.pdf",
		Notes:     "Paid via wire transfer",
	})
}

// ---------------------------------------------------------------------------
// GetLegalStatus
// ---------------------------------------------------------------------------

func TestGetLegalStatus_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/legal-status") {
			t.Errorf("path: want contains /legal-status, got %s", r.URL.Path)
		}
		lcWriteJSON(t, w, 200, legalStatusResp{Data: LegalStatusRecord{
			PatentNumber:  "CN115000001B",
			Jurisdiction:  "CN",
			CurrentStatus: "granted",
			StatusDate:    "2024-06-15",
			History: []LegalStatusChange{
				{FromStatus: "filed", ToStatus: "published", ChangeDate: "2022-01-10", Reason: "18-month publication", Source: "CNIPA_API"},
				{FromStatus: "published", ToStatus: "under_examination", ChangeDate: "2022-07-20", Reason: "Substantive examination requested", Source: "CNIPA_API"},
				{FromStatus: "under_examination", ToStatus: "granted", ChangeDate: "2024-06-15", Reason: "Patent granted", Source: "CNIPA_API"},
			},
		}})
	})
	ls, err := lc.GetLegalStatus(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetLegalStatus: %v", err)
	}
	if ls.CurrentStatus != "granted" {
		t.Errorf("CurrentStatus: want granted, got %s", ls.CurrentStatus)
	}
	if len(ls.History) != 3 {
		t.Fatalf("History: want 3, got %d", len(ls.History))
	}
	if ls.History[0].FromStatus != "filed" {
		t.Errorf("History[0].FromStatus: want filed, got %s", ls.History[0].FromStatus)
	}
	if ls.History[0].Source != "CNIPA_API" {
		t.Errorf("History[0].Source: want CNIPA_API, got %s", ls.History[0].Source)
	}
	if ls.History[2].ToStatus != "granted" {
		t.Errorf("History[2].ToStatus: want granted, got %s", ls.History[2].ToStatus)
	}
}

func TestGetLegalStatus_EmptyPatentNumber(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.GetLegalStatus(context.Background(), "")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestGetLegalStatus_NotFound(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		lcWriteJSON(t, w, 404, map[string]string{"code": "not_found", "message": "patent not found"})
	})
	_, err := lc.GetLegalStatus(context.Background(), "CN000000000X")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Error("IsNotFound should be true")
	}
}

// ---------------------------------------------------------------------------
// SyncLegalStatus
// ---------------------------------------------------------------------------

func TestSyncLegalStatus_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/lifecycle/legal-status/sync" {
			t.Errorf("path: want /api/v1/lifecycle/legal-status/sync, got %s", r.URL.Path)
		}
		w.WriteHeader(202)
	})
	err := lc.SyncLegalStatus(context.Background(), []string{"CN115000001B", "US11000001B2"})
	if err != nil {
		t.Fatalf("SyncLegalStatus: %v", err)
	}
}

func TestSyncLegalStatus_EmptyPatentNumbers(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	err := lc.SyncLegalStatus(context.Background(), []string{})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSyncLegalStatus_NilPatentNumbers(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	err := lc.SyncLegalStatus(context.Background(), nil)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSyncLegalStatus_RequestBody(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := lcReadBody(t, r)
		pns, ok := body["patent_numbers"].([]interface{})
		if !ok {
			t.Fatalf("patent_numbers: want array, got %T", body["patent_numbers"])
		}
		if len(pns) != 3 {
			t.Errorf("patent_numbers: want 3 elements, got %d", len(pns))
		}
		if pns[0] != "P1" || pns[1] != "P2" || pns[2] != "P3" {
			t.Errorf("patent_numbers: want [P1,P2,P3], got %v", pns)
		}
		w.WriteHeader(202)
	})
	_ = lc.SyncLegalStatus(context.Background(), []string{"P1", "P2", "P3"})
}

func TestSyncLegalStatus_ServerError(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		lcWriteJSON(t, w, 503, map[string]string{"code": "service_unavailable", "message": "overloaded"})
	})
	err := lc.SyncLegalStatus(context.Background(), []string{"P1"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 503 {
		t.Errorf("StatusCode: want 503, got %d", apiErr.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GetReminders
// ---------------------------------------------------------------------------

func TestGetReminders_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/reminders") {
			t.Errorf("path: want contains /reminders, got %s", r.URL.Path)
		}
		lcWriteJSON(t, w, 200, reminderListResp{Data: []ReminderConfig{
			{ID: "rm-1", PatentNumber: "CN115000001B", DeadlineType: "annuity_payment", ReminderDays: []int{90, 60, 30, 7}, Channels: []string{"email", "in_app"}, Recipients: []string{"user-1"}, Enabled: true},
			{ID: "rm-2", PatentNumber: "CN115000001B", DeadlineType: "oa_response", ReminderDays: []int{30, 7}, Channels: []string{"wechat_work"}, Recipients: []string{"user-1", "user-2"}, Enabled: false},
		}})
	})
	reminders, err := lc.GetReminders(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetReminders: %v", err)
	}
	if len(reminders) != 2 {
		t.Fatalf("want 2 reminders, got %d", len(reminders))
	}
	if len(reminders[0].ReminderDays) != 4 {
		t.Errorf("ReminderDays: want 4 elements, got %d", len(reminders[0].ReminderDays))
	}
	if reminders[0].ReminderDays[0] != 90 {
		t.Errorf("ReminderDays[0]: want 90, got %d", reminders[0].ReminderDays[0])
	}
	if len(reminders[0].Channels) != 2 {
		t.Errorf("Channels: want 2, got %d", len(reminders[0].Channels))
	}
	if !reminders[0].Enabled {
		t.Error("Enabled: want true")
	}
	if reminders[1].Enabled {
		t.Error("Enabled: want false")
	}
}

func TestGetReminders_EmptyPatentNumber(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.GetReminders(context.Background(), "")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SetReminder
// ---------------------------------------------------------------------------

func TestSetReminder_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: want POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/lifecycle/reminders" {
			t.Errorf("path: want /api/v1/lifecycle/reminders, got %s", r.URL.Path)
		}
		lcWriteJSON(t, w, 200, reminderResp{Data: ReminderConfig{
			ID: "rm-new", PatentNumber: "CN115000001B", DeadlineType: "annuity_payment",
			ReminderDays: []int{60, 30}, Channels: []string{"email"}, Recipients: []string{"user-1"}, Enabled: true,
		}})
	})
	rc, err := lc.SetReminder(context.Background(), &ReminderConfigRequest{
		PatentNumber: "CN115000001B", DeadlineType: "annuity_payment",
		ReminderDays: []int{60, 30}, Channels: []string{"email"}, Recipients: []string{"user-1"},
	})
	if err != nil {
		t.Fatalf("SetReminder: %v", err)
	}
	if rc.ID != "rm-new" {
		t.Errorf("ID: want rm-new, got %s", rc.ID)
	}
	if !rc.Enabled {
		t.Error("Enabled: want true")
	}
}

func TestSetReminder_EmptyPatentNumber(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.SetReminder(context.Background(), &ReminderConfigRequest{
		PatentNumber: "", ReminderDays: []int{30}, Channels: []string{"email"},
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSetReminder_EmptyReminderDays(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.SetReminder(context.Background(), &ReminderConfigRequest{
		PatentNumber: "CN115000001B", ReminderDays: []int{}, Channels: []string{"email"},
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSetReminder_EmptyChannels(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.SetReminder(context.Background(), &ReminderConfigRequest{
		PatentNumber: "CN115000001B", ReminderDays: []int{30}, Channels: []string{},
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSetReminder_NilRequest(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := lc.SetReminder(context.Background(), nil)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSetReminder_RequestBody(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := lcReadBody(t, r)
		if body["patent_number"] != "CN115000001B" {
			t.Errorf("patent_number: want CN115000001B, got %v", body["patent_number"])
		}
		if body["deadline_type"] != "oa_response" {
			t.Errorf("deadline_type: want oa_response, got %v", body["deadline_type"])
		}
		days, ok := body["reminder_days"].([]interface{})
		if !ok {
			t.Fatalf("reminder_days: want array, got %T", body["reminder_days"])
		}
		if len(days) != 3 {
			t.Errorf("reminder_days: want 3 elements, got %d", len(days))
		}
		if days[0] != float64(90) || days[1] != float64(30) || days[2] != float64(7) {
			t.Errorf("reminder_days: want [90,30,7], got %v", days)
		}
		channels, ok := body["channels"].([]interface{})
		if !ok {
			t.Fatalf("channels: want array, got %T", body["channels"])
		}
		if len(channels) != 2 {
			t.Errorf("channels: want 2 elements, got %d", len(channels))
		}
		if channels[0] != "email" || channels[1] != "wechat_work" {
			t.Errorf("channels: want [email,wechat_work], got %v", channels)
		}
		lcWriteJSON(t, w, 200, reminderResp{Data: ReminderConfig{ID: "rm-x"}})
	})
	_, _ = lc.SetReminder(context.Background(), &ReminderConfigRequest{
		PatentNumber: "CN115000001B",
		DeadlineType: "oa_response",
		ReminderDays: []int{90, 30, 7},
		Channels:     []string{"email", "wechat_work"},
		Recipients:   []string{"user-1"},
	})
}

// ---------------------------------------------------------------------------
// DeleteReminder
// ---------------------------------------------------------------------------

func TestDeleteReminder_Success(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method: want DELETE, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rm-abc") {
			t.Errorf("path: want suffix /rm-abc, got %s", r.URL.Path)
		}
		w.WriteHeader(204)
	})
	err := lc.DeleteReminder(context.Background(), "rm-abc")
	if err != nil {
		t.Fatalf("DeleteReminder: %v", err)
	}
}

func TestDeleteReminder_EmptyID(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	err := lc.DeleteReminder(context.Background(), "")
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestDeleteReminder_NotFound(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		lcWriteJSON(t, w, 404, map[string]string{"code": "not_found", "message": "reminder not found"})
	})
	err := lc.DeleteReminder(context.Background(), "rm-999")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Error("IsNotFound should be true")
	}
}

// ---------------------------------------------------------------------------
// buildDeadlineQueryParams
// ---------------------------------------------------------------------------

func TestBuildDeadlineQueryParams_AllFields(t *testing.T) {
	lc := &LifecycleClient{}
	qs := lc.buildDeadlineQueryParams(&DeadlineQuery{
		PatentNumbers: []string{"CN115000001B", "US11000001B2"},
		Jurisdictions: []string{"CN", "US", "EP"},
		DaysAhead:     60,
	})
	if !strings.Contains(qs, "days_ahead=60") {
		t.Errorf("want days_ahead=60 in %s", qs)
	}
	if !strings.Contains(qs, "patent_numbers=CN115000001B") {
		t.Errorf("want patent_numbers containing CN115000001B in %s", qs)
	}
	if !strings.Contains(qs, "US11000001B2") {
		t.Errorf("want patent_numbers containing US11000001B2 in %s", qs)
	}
	if !strings.Contains(qs, "jurisdictions=CN") {
		t.Errorf("want jurisdictions containing CN in %s", qs)
	}
}

func TestBuildDeadlineQueryParams_EmptyQuery(t *testing.T) {
	lc := &LifecycleClient{}
	qs := lc.buildDeadlineQueryParams(&DeadlineQuery{})
	if !strings.Contains(qs, "days_ahead=90") {
		t.Errorf("want days_ahead=90 (default) in %s", qs)
	}
	if strings.Contains(qs, "patent_numbers") {
		t.Errorf("want no patent_numbers in %s", qs)
	}
	if strings.Contains(qs, "jurisdictions") {
		t.Errorf("want no jurisdictions in %s", qs)
	}
}

func TestBuildDeadlineQueryParams_SpecialCharacters(t *testing.T) {
	lc := &LifecycleClient{}
	qs := lc.buildDeadlineQueryParams(&DeadlineQuery{
		PatentNumbers: []string{"CN115/123", "US 11&000"},
		DaysAhead:     30,
	})
	// The comma-joined value should be URL-encoded by url.Values.Encode()
	if !strings.Contains(qs, "patent_numbers=") {
		t.Errorf("want patent_numbers param in %s", qs)
	}
	// Verify the raw query string is properly encoded
	decoded, err := url.ParseQuery(qs)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	pn := decoded.Get("patent_numbers")
	if !strings.Contains(pn, "CN115/123") {
		t.Errorf("decoded patent_numbers should contain CN115/123, got %s", pn)
	}
	if !strings.Contains(pn, "US 11&000") {
		t.Errorf("decoded patent_numbers should contain 'US 11&000', got %s", pn)
	}
}

// ---------------------------------------------------------------------------
// URL path construction
// ---------------------------------------------------------------------------

func TestLifecycleClient_URLPathConstruction(t *testing.T) {
	paths := make(map[string]bool)

	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		paths[key] = true

		switch {
		case r.URL.Path == "/api/v1/lifecycle/deadlines" && r.Method == http.MethodGet:
			lcWriteJSON(t, w, 200, deadlineListResp{Data: []Deadline{}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/lifecycle/deadlines/") && r.Method == http.MethodGet:
			lcWriteJSON(t, w, 200, deadlineResp{Data: Deadline{ID: "dl-1"}})
		case strings.HasSuffix(r.URL.Path, "/annuities") && r.Method == http.MethodGet:
			lcWriteJSON(t, w, 200, annuityListResp{Data: []AnnuityRecord{}})
		case r.URL.Path == "/api/v1/lifecycle/annuities/summary" && r.Method == http.MethodGet:
			lcWriteJSON(t, w, 200, annuitySummaryResp{Data: AnnuitySummary{}})
		case r.URL.Path == "/api/v1/lifecycle/annuities/payments" && r.Method == http.MethodPost:
			lcWriteJSON(t, w, 200, annuityRecordResp{Data: AnnuityRecord{ID: "an-1"}})
		case strings.HasSuffix(r.URL.Path, "/legal-status") && r.Method == http.MethodGet:
			lcWriteJSON(t, w, 200, legalStatusResp{Data: LegalStatusRecord{}})
		case r.URL.Path == "/api/v1/lifecycle/legal-status/sync" && r.Method == http.MethodPost:
			w.WriteHeader(202)
		case strings.HasSuffix(r.URL.Path, "/reminders") && r.Method == http.MethodGet:
			lcWriteJSON(t, w, 200, reminderListResp{Data: []ReminderConfig{}})
		case r.URL.Path == "/api/v1/lifecycle/reminders" && r.Method == http.MethodPost:
			lcWriteJSON(t, w, 200, reminderResp{Data: ReminderConfig{ID: "rm-1"}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/lifecycle/reminders/") && r.Method == http.MethodDelete:
			w.WriteHeader(204)
		default:
			w.WriteHeader(404)
		}
	})

	ctx := context.Background()

	_, _ = lc.GetDeadlines(ctx, &DeadlineQuery{DaysAhead: 30})
	_, _ = lc.GetDeadline(ctx, "dl-1")
	_, _ = lc.GetAnnuities(ctx, "CN115000001B")
	_, _ = lc.GetAnnuitySummary(ctx)
	_, _ = lc.RecordAnnuityPayment(ctx, &AnnuityPayRequest{AnnuityID: "an-1", PaidDate: "2025-01-01", Amount: 100})
	_, _ = lc.GetLegalStatus(ctx, "CN115000001B")
	_ = lc.SyncLegalStatus(ctx, []string{"P1"})
	_, _ = lc.GetReminders(ctx, "CN115000001B")
	_, _ = lc.SetReminder(ctx, &ReminderConfigRequest{PatentNumber: "CN115000001B", ReminderDays: []int{30}, Channels: []string{"email"}})
	_ = lc.DeleteReminder(ctx, "rm-1")

	expected := []string{
		"GET /api/v1/lifecycle/deadlines",
		"GET /api/v1/lifecycle/deadlines/dl-1",
		"GET /api/v1/lifecycle/patents/CN115000001B/annuities",
		"GET /api/v1/lifecycle/annuities/summary",
		"POST /api/v1/lifecycle/annuities/payments",
		"GET /api/v1/lifecycle/patents/CN115000001B/legal-status",
		"POST /api/v1/lifecycle/legal-status/sync",
		"GET /api/v1/lifecycle/patents/CN115000001B/reminders",
		"POST /api/v1/lifecycle/reminders",
		"DELETE /api/v1/lifecycle/reminders/rm-1",
	}
	for _, e := range expected {
		if !paths[e] {
			t.Errorf("missing path: %s", e)
		}
	}
}

// ---------------------------------------------------------------------------
// Context propagation
// ---------------------------------------------------------------------------

func TestLifecycleClient_ContextPropagation(t *testing.T) {
	lc := newTestLifecycleClient(t, func(w http.ResponseWriter, r *http.Request) {
		lcWriteJSON(t, w, 200, deadlineListResp{Data: []Deadline{}})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := lc.GetDeadlines(ctx, &DeadlineQuery{})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	}
}

//Personal.AI order the ending
