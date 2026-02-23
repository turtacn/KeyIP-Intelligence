package handlers

import (
	"encoding/json"
	"net/http"
)

type MoleculeHandler struct{}

func NewMoleculeHandler() *MoleculeHandler {
	return &MoleculeHandler{}
}

func (h *MoleculeHandler) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}

//Personal.AI order the ending
