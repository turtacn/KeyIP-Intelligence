package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestMoleculeHandler_GetMolecule(t *testing.T) {
	handler := NewMoleculeHandler()
	req := httptest.NewRequest("GET", "/api/v1/molecules/123", nil)
	w := httptest.NewRecorder()

	handler.GetMolecule(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
