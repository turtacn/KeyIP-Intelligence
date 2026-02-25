// Phase 11 - File 268: internal/interfaces/http/handlers/portfolio_handler.go
// 实现专利组合管理 HTTP Handler。
// * 依赖：internal/application/portfolio/service.go
// * 被依赖：internal/interfaces/http/router.go
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PortfolioHandler handles HTTP requests for portfolio operations.
type PortfolioHandler struct {
	portfolioSvc portfolio.Service
	logger       logging.Logger
}

// NewPortfolioHandler creates a new PortfolioHandler.
func NewPortfolioHandler(svc portfolio.Service, logger logging.Logger) *PortfolioHandler {
	return &PortfolioHandler{portfolioSvc: svc, logger: logger}
}

type CreatePortfolioRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	PatentIDs   []string `json:"patent_ids,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type UpdatePortfolioRequest struct {
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type AddPatentsRequest struct {
	PatentIDs []string `json:"patent_ids"`
}

type RemovePatentsRequest struct {
	PatentIDs []string `json:"patent_ids"`
}

func (h *PortfolioHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/portfolios", h.CreatePortfolio)
	mux.HandleFunc("GET /api/v1/portfolios", h.ListPortfolios)
	mux.HandleFunc("GET /api/v1/portfolios/{id}", h.GetPortfolio)
	mux.HandleFunc("PUT /api/v1/portfolios/{id}", h.UpdatePortfolio)
	mux.HandleFunc("DELETE /api/v1/portfolios/{id}", h.DeletePortfolio)
	mux.HandleFunc("POST /api/v1/portfolios/{id}/patents", h.AddPatents)
	mux.HandleFunc("DELETE /api/v1/portfolios/{id}/patents", h.RemovePatents)
	mux.HandleFunc("GET /api/v1/portfolios/{id}/analysis", h.GetAnalysis)
}

func (h *PortfolioHandler) CreatePortfolio(w http.ResponseWriter, r *http.Request) {
	var req CreatePortfolioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("name is required"))
		return
	}

	userID := getUserIDFromContext(r)
	input := &portfolio.CreateInput{
		Name:        req.Name,
		Description: req.Description,
		PatentIDs:   req.PatentIDs,
		Tags:        req.Tags,
		UserID:      userID,
	}

	p, err := h.portfolioSvc.Create(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to create portfolio", "error", err)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *PortfolioHandler) GetPortfolio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("portfolio id is required"))
		return
	}

	p, err := h.portfolioSvc.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get portfolio", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *PortfolioHandler) ListPortfolios(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	userID := getUserIDFromContext(r)

	input := &portfolio.ListInput{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
	}

	result, err := h.portfolioSvc.List(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to list portfolios", "error", err)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PortfolioHandler) UpdatePortfolio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("portfolio id is required"))
		return
	}

	var req UpdatePortfolioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}

	userID := getUserIDFromContext(r)
	input := &portfolio.UpdateInput{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
		UserID:      userID,
	}

	p, err := h.portfolioSvc.Update(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to update portfolio", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *PortfolioHandler) DeletePortfolio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("portfolio id is required"))
		return
	}

	userID := getUserIDFromContext(r)
	if err := h.portfolioSvc.Delete(r.Context(), id, userID); err != nil {
		h.logger.Error("failed to delete portfolio", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *PortfolioHandler) AddPatents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("portfolio id is required"))
		return
	}

	var req AddPatentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}
	if len(req.PatentIDs) == 0 {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent_ids is required"))
		return
	}

	userID := getUserIDFromContext(r)
	if err := h.portfolioSvc.AddPatents(r.Context(), id, req.PatentIDs, userID); err != nil {
		h.logger.Error("failed to add patents to portfolio", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *PortfolioHandler) RemovePatents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("portfolio id is required"))
		return
	}

	var req RemovePatentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}
	if len(req.PatentIDs) == 0 {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent_ids is required"))
		return
	}

	userID := getUserIDFromContext(r)
	if err := h.portfolioSvc.RemovePatents(r.Context(), id, req.PatentIDs, userID); err != nil {
		h.logger.Error("failed to remove patents from portfolio", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *PortfolioHandler) GetAnalysis(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("portfolio id is required"))
		return
	}

	analysis, err := h.portfolioSvc.GetAnalysis(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get portfolio analysis", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, analysis)
}

//Personal.AI order the ending
