package handlers

import (
	"net/http/httptest"
	"testing"
)

func TestPortfolioHandler_GetPortfolio(t *testing.T) {
	handler := NewPortfolioHandler()
	req := httptest.NewRequest("GET", "/api/v1/portfolios/123", nil)
	w := httptest.NewRecorder()

	handler.GetPortfolio(w, req)

	if w.Code != 200 {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

//Personal.AI order the ending
