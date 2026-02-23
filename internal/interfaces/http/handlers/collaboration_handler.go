package handlers

import (
	"encoding/json"
	"net/http"
)

type CollaborationHandler struct{}

func NewCollaborationHandler() *CollaborationHandler {
	return &CollaborationHandler{}
}

func (h *CollaborationHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}

//Personal.AI order the ending
