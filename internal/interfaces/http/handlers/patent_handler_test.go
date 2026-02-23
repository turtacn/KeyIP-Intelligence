package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestPatentHandler_GetPatent(t *testing.T) {
	handler := NewPatentHandler()
	req := httptest.NewRequest("GET", "/api/v1/patents/CN123", nil)
	w := httptest.NewRecorder()

	handler.GetPatent(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
