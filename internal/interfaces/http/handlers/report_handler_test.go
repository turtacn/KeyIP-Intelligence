package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestReportHandler_GenerateReport(t *testing.T) {
	handler := NewReportHandler()
	req := httptest.NewRequest("POST", "/api/v1/reports", nil)
	w := httptest.NewRecorder()

	handler.GenerateReport(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
