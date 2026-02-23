package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRatelimit(t *testing.T) {
	handler := Ratelimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
