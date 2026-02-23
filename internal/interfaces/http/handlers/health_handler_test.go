package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_Check(t *testing.T) {
	handler := NewHealthHandler()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.Check(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
