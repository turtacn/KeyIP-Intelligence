// Phase 11 - File 266: internal/interfaces/http/handlers/patent_handler.go
// 实现专利数据 HTTP Handler。
// * 依赖：internal/application/patent/service.go
// * 被依赖：internal/interfaces/http/router.go
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PatentHandler handles HTTP requests for patent operations.
type PatentHandler struct {
	patentSvc patent.Service
	logger    logging.Logger
}

// NewPatentHandler creates a new PatentHandler.
func NewPatentHandler(svc patent.Service, logger logging.Logger) *PatentHandler {
	return &PatentHandler{patentSvc: svc, logger: logger}
}

type CreatePatentRequest struct {
	Title           string   `json:"title"`
	Abstract        string   `json:"abstract"`
	ApplicationNo   string   `json:"application_no"`
	PublicationNo   string   `json:"publication_no,omitempty"`
	Applicant       string   `json:"applicant"`
	Inventors       []string `json:"inventors"`
	IPCCodes        []string `json:"ipc_codes,omitempty"`
	FilingDate      string   `json:"filing_date"`
	PublicationDate string   `json:"publication_date,omitempty"`
	Claims          string   `json:"claims,omitempty"`
	Description     string   `json:"description,omitempty"`
	Jurisdiction    string   `json:"jurisdiction"`
}

type UpdatePatentRequest struct {
	Title       *string  `json:"title,omitempty"`
	Abstract    *string  `json:"abstract,omitempty"`
	Claims      *string  `json:"claims,omitempty"`
	Description *string  `json:"description,omitempty"`
	IPCCodes    []string `json:"ipc_codes,omitempty"`
}

type SearchPatentsRequest struct {
	Query     string `json:"query"`
	QueryType string `json:"query_type,omitempty"` // Matches proto query_type and frontend searchType
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
	SortBy    string `json:"sort_by,omitempty"`
	SortOrder string `json:"sort_order,omitempty"`
}

type AdvancedSearchRequest struct {
	Title          string   `json:"title,omitempty"`
	Abstract       string   `json:"abstract,omitempty"`
	Applicant      string   `json:"applicant,omitempty"`
	Inventor       string   `json:"inventor,omitempty"`
	IPCCode        string   `json:"ipc_code,omitempty"`
	Jurisdiction   string   `json:"jurisdiction,omitempty"`
	FilingDateFrom string   `json:"filing_date_from,omitempty"`
	FilingDateTo   string   `json:"filing_date_to,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	Page           int      `json:"page"`
	PageSize       int      `json:"page_size"`
}

func (h *PatentHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/patents", h.CreatePatent)
	mux.HandleFunc("GET /api/v1/patents", h.ListPatents)
	mux.HandleFunc("GET /api/v1/patents/{id}", h.GetPatent)
	mux.HandleFunc("PUT /api/v1/patents/{id}", h.UpdatePatent)
	mux.HandleFunc("DELETE /api/v1/patents/{id}", h.DeletePatent)
	mux.HandleFunc("POST /api/v1/patents/search", h.SearchPatents)
	mux.HandleFunc("POST /api/v1/patents/search/advanced", h.AdvancedSearch)
	mux.HandleFunc("GET /api/v1/patents/stats", h.GetPatentStats)

	// Add routes for previously unimplemented placeholders matching proto
	mux.HandleFunc("POST /api/v1/patents/analyze-claims", h.AnalyzeClaims)
	mux.HandleFunc("GET /api/v1/patents/{id}/family", h.GetFamily)
	mux.HandleFunc("GET /api/v1/patents/{id}/citations", h.GetCitationNetwork)
	mux.HandleFunc("POST /api/v1/patents/check-fto", h.CheckFTO)
	mux.HandleFunc("POST /api/v1/patents/assess-infringement", h.AssessInfringementRisk)
}

func (h *PatentHandler) CreatePatent(w http.ResponseWriter, r *http.Request) {
	var req CreatePatentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}
	if req.Title == "" || req.ApplicationNo == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "title and application_no are required"))
		return
	}

	userID := getUserIDFromContext(r)
	input := &patent.CreateInput{
		Title:           req.Title,
		Abstract:        req.Abstract,
		ApplicationNo:   req.ApplicationNo,
		PublicationNo:   req.PublicationNo,
		Applicant:       req.Applicant,
		Inventors:       req.Inventors,
		IPCCodes:        req.IPCCodes,
		FilingDate:      req.FilingDate,
		PublicationDate: req.PublicationDate,
		Claims:          req.Claims,
		Description:     req.Description,
		Jurisdiction:    req.Jurisdiction,
		UserID:          userID,
	}

	p, err := h.patentSvc.Create(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to create patent", logging.Err(err))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *PatentHandler) GetPatent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "patent id is required"))
		return
	}

	p, err := h.patentSvc.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get patent", logging.Err(err), logging.String("id", id))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *PatentHandler) ListPatents(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	jurisdiction := r.URL.Query().Get("jurisdiction")
	applicant := r.URL.Query().Get("applicant")

	input := &patent.ListInput{
		Page:         page,
		PageSize:     pageSize,
		Jurisdiction: jurisdiction,
		Applicant:    applicant,
	}

	result, err := h.patentSvc.List(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to list patents", logging.Err(err))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PatentHandler) UpdatePatent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "patent id is required"))
		return
	}

	var req UpdatePatentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}

	userID := getUserIDFromContext(r)
	input := &patent.UpdateInput{
		ID:          id,
		Title:       req.Title,
		Abstract:    req.Abstract,
		Claims:      req.Claims,
		Description: req.Description,
		IPCCodes:    req.IPCCodes,
		UserID:      userID,
	}

	p, err := h.patentSvc.Update(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to update patent", logging.Err(err), logging.String("id", id))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *PatentHandler) DeletePatent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "patent id is required"))
		return
	}

	userID := getUserIDFromContext(r)
	if err := h.patentSvc.Delete(r.Context(), id, userID); err != nil {
		h.logger.Error("failed to delete patent", logging.Err(err), logging.String("id", id))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *PatentHandler) SearchPatents(w http.ResponseWriter, r *http.Request) {
	var req SearchPatentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "query is required"))
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}
	if req.QueryType == "" {
		req.QueryType = "keyword"
	}

	input := &patent.SearchInput{
		Query:     req.Query,
		Page:      req.Page,
		PageSize:  req.PageSize,
		SortBy:    req.SortBy,
		SortOrder: req.SortOrder,
	}
	// Note: patent.SearchInput may need updating in internal/application/patent to accept QueryType if semantic search logic is implemented there.

	result, err := h.patentSvc.Search(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to search patents", logging.Err(err))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PatentHandler) AdvancedSearch(w http.ResponseWriter, r *http.Request) {
	var req AdvancedSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	input := &patent.AdvancedSearchInput{
		Title:          req.Title,
		Abstract:       req.Abstract,
		Applicant:      req.Applicant,
		Inventor:       req.Inventor,
		IPCCode:        req.IPCCode,
		Jurisdiction:   req.Jurisdiction,
		FilingDateFrom: req.FilingDateFrom,
		FilingDateTo:   req.FilingDateTo,
		Keywords:       req.Keywords,
		Page:           req.Page,
		PageSize:       req.PageSize,
	}

	result, err := h.patentSvc.AdvancedSearch(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to advanced search patents", logging.Err(err))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PatentHandler) GetPatentStats(w http.ResponseWriter, r *http.Request) {
	jurisdiction := r.URL.Query().Get("jurisdiction")
	applicant := r.URL.Query().Get("applicant")

	input := &patent.StatsInput{
		Jurisdiction: jurisdiction,
		Applicant:    applicant,
	}

	stats, err := h.patentSvc.GetStats(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to get patent stats", logging.Err(err))
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// Router-compatible aliases for standard CRUD operations

// List is an alias for ListPatents.
func (h *PatentHandler) List(w http.ResponseWriter, r *http.Request) {
	h.ListPatents(w, r)
}

// Create is an alias for CreatePatent.
func (h *PatentHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.CreatePatent(w, r)
}

// Get is an alias for GetPatent.
func (h *PatentHandler) Get(w http.ResponseWriter, r *http.Request) {
	h.GetPatent(w, r)
}

// Update is an alias for UpdatePatent.
func (h *PatentHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.UpdatePatent(w, r)
}

// Delete is an alias for DeletePatent.
func (h *PatentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	h.DeletePatent(w, r)
}

// Search is an alias for SearchPatents.
func (h *PatentHandler) Search(w http.ResponseWriter, r *http.Request) {
	h.SearchPatents(w, r)
}

// AnalyzeClaims handles claims analysis (placeholder for router).
func (h *PatentHandler) AnalyzeClaims(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "claims analysis not yet implemented"})
}

// GetFamily handles patent family retrieval (placeholder for router).
func (h *PatentHandler) GetFamily(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "patent family not yet implemented"})
}

// GetCitationNetwork handles citation network retrieval (placeholder for router).
func (h *PatentHandler) GetCitationNetwork(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "citation network not yet implemented"})
}

// CheckFTO handles FTO check (placeholder for router).
func (h *PatentHandler) CheckFTO(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "FTO check not yet implemented"})
}

// AssessInfringementRiskRequest describes an infringement risk request
type AssessInfringementRiskRequest struct {
	MoleculeSMILES string   `json:"molecule_smiles"`
	PatentID       string   `json:"patent_id"`
	ClaimNumbers   []uint32 `json:"claim_numbers,omitempty"`
	IncludePH      bool     `json:"include_prosecution_history_analysis,omitempty"`
}

// AssessInfringementRisk handles the infringement risk assessment (placeholder for router, aligning with proto).
func (h *PatentHandler) AssessInfringementRisk(w http.ResponseWriter, r *http.Request) {
	var req AssessInfringementRiskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}

	// TODO: Wire up to actual infringement assessment service when implemented
	writeJSON(w, http.StatusNotImplemented, map[string]string{"message": "Infringement assessment not yet implemented in HTTP handler, see gRPC PatentService.AssessInfringementRisk"})
}

//Personal.AI order the ending
