// Auth HTTP handler: POST /api/v1/auth/signin, GET /api/v1/auth/me
// Supports local email+password sign-in (no Keycloak required).

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	appauth "github.com/turtacn/KeyIP-Intelligence/internal/application/auth"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authSvc *appauth.Service
	logger  logging.Logger
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc *appauth.Service, logger logging.Logger) *AuthHandler {
	return &AuthHandler{authSvc: svc, logger: logger}
}

// RegisterRoutes registers auth routes on the given ServeMux.
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/signin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, errors.New(errors.ErrCodeBadRequest, "method not allowed"))
			return
		}
		h.SignIn(w, r)
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New(errors.ErrCodeBadRequest, "method not allowed"))
			return
		}
		h.Me(w, r)
	})
}

// SignIn handles POST /api/v1/auth/signin.
func (h *AuthHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	if !isContentTypeJSON(r) {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("content-type", "Content-Type must be application/json"))
		return
	}

	var req appauth.SignInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "email and password are required"))
		return
	}

	resp, err := h.authSvc.SignIn(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Me handles GET /api/v1/auth/me — returns the current user profile from the Bearer token.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, errors.New(errors.ErrCodeUnauthorized, "authorization header required"))
		return
	}

	user, err := h.authSvc.GetUserProfile(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// extractBearerToken extracts the Bearer token from the Authorization header.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

//Personal.AI order the ending
