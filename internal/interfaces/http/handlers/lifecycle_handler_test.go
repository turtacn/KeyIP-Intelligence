package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestLifecycleHandler_GetDeadlines(t *testing.T) {
	handler := NewLifecycleHandler()
	req := httptest.NewRequest("GET", "/api/v1/lifecycle/deadlines", nil)
	w := httptest.NewRecorder()

	handler.GetDeadlines(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
