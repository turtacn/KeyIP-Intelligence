package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewPatentHandler(t *testing.T) {
	handler := NewPatentHandler()
	if handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestPatentHandler_Handle(t *testing.T) {
	handler := NewPatentHandler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.Handle(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
