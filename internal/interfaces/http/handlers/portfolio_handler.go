package handlers

import (
	"encoding/json"
	"net/http"
)

type PortfolioHandler struct{}

func NewPortfolioHandler() *PortfolioHandler {
	return &PortfolioHandler{}
}

func (h *PortfolioHandler) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"id": "pf-123", "patents": 90})
}

func (h *PortfolioHandler) Analyze(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"health_score": "85"})
}

//Personal.AI order the ending
