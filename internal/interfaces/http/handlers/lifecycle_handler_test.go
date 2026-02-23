package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewLifecycleHandler(t *testing.T) {
	handler := NewLifecycleHandler()
	if handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestLifecycleHandler_Handle(t *testing.T) {
	handler := NewLifecycleHandler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
