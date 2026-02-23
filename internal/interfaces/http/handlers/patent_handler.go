package handlers

import (
	"encoding/json"
	"net/http"
)

type PatentHandler struct{}

func NewPatentHandler() *PatentHandler {
	return &PatentHandler{}
}

func (h *PatentHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}

//Personal.AI order the ending
