package handlers

import (
	"encoding/json"
	"net/http"
)

type MoleculeHandler struct{}

func NewMoleculeHandler() *MoleculeHandler {
	return &MoleculeHandler{}
}

func (h *MoleculeHandler) GetMolecule(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"id": "mol-123", "smiles": "C1=CC=CC=C1"})
}

func (h *MoleculeHandler) SearchMolecules(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"results": []map[string]string{{"id": "mol-001"}}})
}

//Personal.AI order the ending
