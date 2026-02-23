package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenant(t *testing.T) {
	handler := Tenant(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

//Personal.AI order the ending
