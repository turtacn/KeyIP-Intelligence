// response.go — API response envelope helpers.
// Wraps domain-layer responses in the frontend-compatible ApiResponse format.
package handlers

import (
	"encoding/json"
	"net/http"
)

// ApiResponse is the standard success envelope expected by the frontend.
type ApiResponse struct {
	Code       int          `json:"code"`
	Message    string       `json:"message"`
	Data       interface{}  `json:"data"`
	Pagination *Pagination  `json:"pagination,omitempty"`
}

// Pagination carries page metadata for list endpoints.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

// writeAPISuccess writes a successful API response with the ApiResponse envelope.
// data is placed inside the "data" field.
func writeAPISuccess(w http.ResponseWriter, statusCode int, data interface{}) {
	resp := ApiResponse{
		Code:    0,
		Message: "ok",
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeAPIList writes a paginated list response.
func writeAPIList(w http.ResponseWriter, data interface{}, page, pageSize, total int) {
	resp := ApiResponse{
		Code:    0,
		Message: "ok",
		Data:    data,
		Pagination: &Pagination{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// writeAPIError writes an error response in the frontend-compatible format.
func writeAPIError(w http.ResponseWriter, statusCode int, message string) {
	resp := map[string]interface{}{
		"code":    statusCode,
		"message": message,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}
