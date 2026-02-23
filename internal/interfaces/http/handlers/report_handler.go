package handlers

import (
	"encoding/json"
	"net/http"
)

type ReportHandler struct{}

func NewReportHandler() *ReportHandler {
	return &ReportHandler{}
}

func (h *ReportHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"report_id": "rpt-123", "status": "generating"})
}

func (h *ReportHandler) GetReportStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "completed", "url": "/reports/123.pdf"})
}

//Personal.AI order the ending
