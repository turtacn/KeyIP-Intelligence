// Phase 11 - File: internal/interfaces/http/handlers/common.go
// Common helper functions for HTTP handlers.

package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// getUserIDFromContext extracts user ID from request context (set by auth middleware).
func getUserIDFromContext(r *http.Request) string {
	return middleware.ContextGetUserID(r.Context())
}

// parsePagination extracts page and page_size from query parameters.
func parsePagination(r *http.Request) (int, int) {
	page := 1
	pageSize := 20

	if v := r.URL.Query().Get("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			page = p
		}
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		if ps, err := strconv.Atoi(v); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}
	return page, pageSize
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// ErrorResponse is the standard error response body.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError writes a structured error response.
func writeError(w http.ResponseWriter, statusCode int, err error) {
	resp := ErrorResponse{
		Code:    http.StatusText(statusCode),
		Message: err.Error(),
	}
	writeJSON(w, statusCode, resp)
}

// writeAppError maps application-level errors to HTTP status codes.
func writeAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.IsNotFound(err):
		writeError(w, http.StatusNotFound, err)
	case errors.IsValidation(err):
		writeError(w, http.StatusBadRequest, err)
	case errors.IsConflict(err):
		writeError(w, http.StatusConflict, err)
	case errors.IsUnauthorized(err):
		writeError(w, http.StatusUnauthorized, err)
	case errors.IsForbidden(err):
		writeError(w, http.StatusForbidden, err)
	default:
		// Mask internal errors
		writeError(w, http.StatusInternalServerError, errors.New(errors.ErrCodeInternal, "internal server error"))
	}
}

//Personal.AI order the ending
