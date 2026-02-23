package handlers

import (
	"encoding/json"
	"net/http"
)

type PortfolioHandler struct{}

func NewPortfolioHandler() *PortfolioHandler {
	return &PortfolioHandler{}
}

func (h *PortfolioHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}

//Personal.AI order the ending
