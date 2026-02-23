package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCollaborationHandler_CreateWorkspace(t *testing.T) {
	handler := NewCollaborationHandler()
	req := httptest.NewRequest("POST", "/api/v1/workspaces", nil)
	w := httptest.NewRecorder()

	handler.CreateWorkspace(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
