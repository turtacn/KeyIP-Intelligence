package handlers

import (
	"encoding/json"
	"net/http"
)

type PatentHandler struct{}

func NewPatentHandler() *PatentHandler {
	return &PatentHandler{}
}

func (h *PatentHandler) GetPatent(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"number": "CN202110123456", "title": "OLED Material"})
}

func (h *PatentHandler) SearchPatents(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{"results": []map[string]string{{"number": "CN001"}}})
}

//Personal.AI order the ending
