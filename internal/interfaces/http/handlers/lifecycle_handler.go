package handlers

import (
	"encoding/json"
	"net/http"
)

type LifecycleHandler struct{}

func NewLifecycleHandler() *LifecycleHandler {
	return &LifecycleHandler{}
}

func (h *LifecycleHandler) GetDeadlines(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"deadlines": []string{"deadline1", "deadline2"}})
}

func (h *LifecycleHandler) CalculateAnnuity(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]float64{"amount": 12000.00})
}

//Personal.AI order the ending
