// Phase 11 - File 266: internal/interfaces/http/handlers/patent_handler.go
// 实现专利数据 HTTP Handler。
// * 依赖：internal/application/patent/service.go
// * 被依赖：internal/interfaces/http/router.go
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/infringement"
	"github.com/turtacn/KeyIP-Intelligence/internal/application/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PatentHandler handles HTTP requests for patent operations.
type PatentHandler struct {
	patentSvc      patent.Service
	infringementSvc infringement.RiskAssessmentService // optional; nil if not wired
	logger         logging.Logger
}

// NewPatentHandler creates a new PatentHandler.
// infringementSvc may be nil if infringement assessment is not wired yet.
func NewPatentHandler(svc patent.Service, infringementSvc infringement.RiskAssessmentService, logger logging.Logger) *PatentHandler {
	return &PatentHandler{patentSvc: svc, infringementSvc: infringementSvc, logger: logger}
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

// AnalyzeClaimsRequest is the request body for claim analysis.
type AnalyzeClaimsRequest struct {
	PatentID   string   `json:"patent_id,omitempty"`
	ClaimTexts []string `json:"claim_texts,omitempty"`
}

// ClaimSummary represents a single parsed claim.
type ClaimSummary struct {
	Number    int    `json:"number"`
	Type      string `json:"type"` // "independent" or "dependent"
	Text      string `json:"text"`
	DependsOn []int  `json:"depends_on"`
}

// ClaimAnalysisResponse is the response from claim analysis.
type ClaimAnalysisResponse struct {
	PatentID         string         `json:"patent_id,omitempty"`
	PatentTitle      string         `json:"patent_title,omitempty"`
	TotalClaims      int            `json:"total_claims"`
	IndependentCount int            `json:"independent_count"`
	DependentCount   int            `json:"dependent_count"`
	Claims           []ClaimSummary `json:"claims"`
	ClaimTree        interface{}    `json:"claim_tree"`
	AnalyzedAt       string         `json:"analyzed_at"`
}

// FamilyMember represents a related patent in a family.
type FamilyMember struct {
	ID           string `json:"id"`
	PatentNumber string `json:"patent_number"`
	Title        string `json:"title"`
	Jurisdiction string `json:"jurisdiction"`
	FilingDate   string `json:"filing_date"`
	Applicant    string `json:"applicant"`
	Relationship string `json:"relationship"`
}

// FamilyResponse is the response from family retrieval.
type FamilyResponse struct {
	PatentID     string         `json:"patent_id"`
	PatentNumber string         `json:"patent_number"`
	FamilyID     string         `json:"family_id,omitempty"`
	Members      []FamilyMember `json:"members"`
	TotalMembers int            `json:"total_members"`
}

// CitationRef represents a single citation reference.
type CitationRef struct {
	PatentNumber string `json:"patent_number"`
	Title        string `json:"title,omitempty"`
	Relation     string `json:"relation"` // "cites" or "cited_by"
}

// CitationNetworkResponse is the response from citation network retrieval.
type CitationNetworkResponse struct {
	PatentID          string        `json:"patent_id"`
	PatentNumber      string        `json:"patent_number"`
	Title             string        `json:"title"`
	ForwardCitations  []CitationRef `json:"forward_citations"`
	BackwardCitations []CitationRef `json:"backward_citations"`
	TotalCitations    int           `json:"total_citations"`
}

// CheckFTORequest is the request body for FTO checking.
type CheckFTORequest struct {
	MoleculeSMILES string   `json:"molecule_smiles"`
	Jurisdictions  []string `json:"jurisdictions"`
	ExcludePatents []string `json:"exclude_patents,omitempty"`
	Depth          string   `json:"depth,omitempty"`
}

// AssessInfringementRiskRequest describes an infringement risk request
type AssessInfringementRiskRequest struct {
	MoleculeSMILES string   `json:"molecule_smiles"`
	PatentID       string   `json:"patent_id"`
	ClaimNumbers   []uint32 `json:"claim_numbers,omitempty"`
	IncludePH      bool     `json:"include_prosecution_history_analysis,omitempty"`
}

// SearchPatentsFilter contains optional structured filters for patent search,
// matching the proto ListPatentsRequest filter fields in SearchPatentsRequest.filters.
type SearchPatentsFilter struct {
	Applicant      string `json:"applicant,omitempty"`
	IPCCode        string `json:"ipc_code,omitempty"`
	FilingDateFrom string `json:"filing_date_from,omitempty"`
	FilingDateTo   string `json:"filing_date_to,omitempty"`
}

type SearchPatentsRequest struct {
	Query     string              `json:"query"`
	QueryType string              `json:"query_type,omitempty"`
	Page      int                 `json:"page"`
	PageSize  int                 `json:"page_size"`
	SortBy    string              `json:"sort_by,omitempty"`
	SortOrder string              `json:"sort_order,omitempty"`
	Filters   *SearchPatentsFilter `json:"filters,omitempty"` // matches proto SearchPatentsRequest.filters
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
	if req.Filters != nil {
		input.Applicant = req.Filters.Applicant
		input.IPCCode = req.Filters.IPCCode
		input.FilingDateFrom = req.Filters.FilingDateFrom
		input.FilingDateTo = req.Filters.FilingDateTo
	}

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

// AnalyzeClaims parses and analyzes patent claims.
// Accepts either a patent_id to fetch from the service or raw claim_texts.
func (h *PatentHandler) AnalyzeClaims(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeClaimsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}

	var patentTitle string
	var claimsText string

	if req.PatentID != "" {
		p, err := h.patentSvc.GetByID(r.Context(), req.PatentID)
		if err != nil {
			h.logger.Error("failed to fetch patent for claims analysis", logging.Err(err), logging.String("id", req.PatentID))
			writeAppError(w, err)
			return
		}
		patentTitle = p.Title
		claimsText = p.Claims
	} else if len(req.ClaimTexts) > 0 {
		claimsText = strings.Join(req.ClaimTexts, "\n")
	} else {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "patent_id or claim_texts is required"))
		return
	}

	if claimsText == "" {
		writeJSON(w, http.StatusOK, ClaimAnalysisResponse{
			TotalClaims: 0,
			Claims:      []ClaimSummary{},
			AnalyzedAt:  time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	claims, err := parseClaimText(claimsText)
	if err != nil {
		h.logger.Error("failed to parse claims text", logging.Err(err))
		writeError(w, http.StatusInternalServerError, errors.NewInternal("claims parsing failed"))
		return
	}

	independentCount := 0
	dependentCount := 0
	for i := range claims {
		if claims[i].Type == "independent" {
			independentCount++
		} else {
			dependentCount++
		}
	}

	tree := buildClaimTree(claims)

	resp := ClaimAnalysisResponse{
		PatentID:         req.PatentID,
		PatentTitle:      patentTitle,
		TotalClaims:      len(claims),
		IndependentCount: independentCount,
		DependentCount:   dependentCount,
		Claims:           claims,
		ClaimTree:        tree,
		AnalyzedAt:       time.Now().UTC().Format(time.RFC3339),
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetFamily retrieves patent family members from the knowledge graph.
func (h *PatentHandler) GetFamily(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "patent id is required"))
		return
	}

	p, err := h.patentSvc.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get patent for family retrieval", logging.Err(err), logging.String("id", id))
		writeAppError(w, err)
		return
	}

	// Search for related patents by same applicant as family proxy
	var members []FamilyMember
	if p.Applicant != "" {
		searchResult, searchErr := h.patentSvc.Search(r.Context(), &patent.SearchInput{
			Query:    p.Applicant,
			Page:     1,
			PageSize: 50,
		})
		if searchErr == nil && searchResult != nil {
			for _, related := range searchResult.Patents {
				if related.ID == id {
					continue
				}
				rel := "family_member"
				if related.Jurisdiction == p.Jurisdiction {
					rel = "same_jurisdiction"
				}
				members = append(members, FamilyMember{
					ID:           related.ID,
					PatentNumber: related.PublicationNo,
					Title:        related.Title,
					Jurisdiction: related.Jurisdiction,
					FilingDate:   related.FilingDate,
					Applicant:    related.Applicant,
					Relationship: rel,
				})
			}
		}
	}

	if members == nil {
		members = []FamilyMember{}
	}

	resp := FamilyResponse{
		PatentID:     id,
		PatentNumber: p.PublicationNo,
		Members:      members,
		TotalMembers: len(members),
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetCitationNetwork retrieves the citation network for a patent.
func (h *PatentHandler) GetCitationNetwork(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "patent id is required"))
		return
	}

	p, err := h.patentSvc.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get patent for citation network", logging.Err(err), logging.String("id", id))
		writeAppError(w, err)
		return
	}

	var forward []CitationRef
	var backward []CitationRef

	// Search for potentially related patents to build a basic citation view
	if p.Applicant != "" {
		searchResult, searchErr := h.patentSvc.Search(r.Context(), &patent.SearchInput{
			Query:    p.Applicant,
			Page:     1,
			PageSize: 20,
		})
		if searchErr == nil && searchResult != nil {
			for _, related := range searchResult.Patents {
				if related.ID == id {
					continue
				}
				if related.FilingDate < p.FilingDate {
					backward = append(backward, CitationRef{
						PatentNumber: related.PublicationNo,
						Title:        related.Title,
						Relation:     "cited_by",
					})
				} else {
					forward = append(forward, CitationRef{
						PatentNumber: related.PublicationNo,
						Title:        related.Title,
						Relation:     "cites",
					})
				}
			}
		}
	}

	if forward == nil {
		forward = []CitationRef{}
	}
	if backward == nil {
		backward = []CitationRef{}
	}

	resp := CitationNetworkResponse{
		PatentID:          id,
		PatentNumber:      p.PublicationNo,
		Title:             p.Title,
		ForwardCitations:  forward,
		BackwardCitations: backward,
		TotalCitations:    len(forward) + len(backward),
	}

	writeJSON(w, http.StatusOK, resp)
}

// CheckFTO performs freedom-to-operate analysis using the infringement risk
// assessment service.
func (h *PatentHandler) CheckFTO(w http.ResponseWriter, r *http.Request) {
	if h.infringementSvc == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New(errors.ErrCodeServiceUnavailable, "infringement assessment service not configured"))
		return
	}

	var req CheckFTORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}

	if req.MoleculeSMILES == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "molecule_smiles is required"))
		return
	}
	if len(req.Jurisdictions) == 0 {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "at least one jurisdiction is required"))
		return
	}

	// Map to the infringement service FTO request
	ftoReq := &infringement.FTORequest{
		Molecules: []infringement.BatchMoleculeInput{
			{
				SMILES: req.MoleculeSMILES,
			},
		},
		Jurisdictions:  req.Jurisdictions,
		ExcludePatents: req.ExcludePatents,
	}
	if req.Depth != "" {
		ftoReq.Depth = infringement.AnalysisDepth(req.Depth)
	}

	resp, err := h.infringementSvc.AssessFTO(r.Context(), ftoReq)
	if err != nil {
		h.logger.Error("FTO analysis failed", logging.Err(err))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// AssessInfringementRisk handles the infringement risk assessment.
func (h *PatentHandler) AssessInfringementRisk(w http.ResponseWriter, r *http.Request) {
	if h.infringementSvc == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New(errors.ErrCodeServiceUnavailable, "infringement assessment service not configured"))
		return
	}

	var req AssessInfringementRiskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "invalid request body"))
		return
	}

	if req.MoleculeSMILES == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "molecule_smiles is required"))
		return
	}
	if req.PatentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("field", "patent_id is required"))
		return
	}

	// Map to the infringement service risk request
	riskReq := &infringement.MoleculeRiskRequest{
		SMILES: req.MoleculeSMILES,
	}
	if req.IncludePH {
		riskReq.Depth = infringement.AnalysisDepthDeep
	}

	resp, err := h.infringementSvc.AssessMolecule(r.Context(), riskReq)
	if err != nil {
		h.logger.Error("infringement risk assessment failed", logging.Err(err))
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// parseClaimText splits a multi-claim text into individual ClaimSummary entries.
func parseClaimText(text string) ([]ClaimSummary, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return []ClaimSummary{}, nil
	}

	// Try to split by numbered claim patterns: "1. ", "2. ", etc.
	re := regexp.MustCompile(`(?m)^\s*(\d+)\s*[.:\)]\s*`)
	matches := re.FindAllStringSubmatchIndex(text, -1)

	if len(matches) == 0 {
		// Treat as a single unnumbered claim
		return []ClaimSummary{{
			Number: 1,
			Type:   "independent",
			Text:   text,
		}}, nil
	}

	// Extract each claim text
	var claimParts []string
	for i, match := range matches {
		start := match[1] // end of claim number group (start of claim text)
		var end int
		if i+1 < len(matches) {
			end = matches[i+1][0]
		} else {
			end = len(text)
		}
		part := strings.TrimSpace(text[start:end])
		if part != "" {
			claimParts = append(claimParts, part)
		}
	}

	if len(claimParts) == 0 {
		return []ClaimSummary{}, nil
	}

	claims := make([]ClaimSummary, 0, len(claimParts))
	for i, ct := range claimParts {
		num := i + 1
		lower := strings.ToLower(ct)
		claimType := "independent"
		dependsOn := []int{}

		if isDependentClaim(lower) {
			claimType = "dependent"
			dependsOn = extractDependencyRefs(lower)
		}

		claims = append(claims, ClaimSummary{
			Number:    num,
			Type:      claimType,
			Text:      ct,
			DependsOn: dependsOn,
		})
	}

	return claims, nil
}

// isDependentClaim checks if claim text indicates a dependent claim by matching
// common patent dependency phrasing.
func isDependentClaim(lowerText string) bool {
	keywords := []string{
		"according to claim",
		"as claimed in",
		"as defined in",
		"according to any",
		"of claim",
		"of any of claims",
		"according to claims",
		"according to the preceding",
		"as set forth in",
		"according to any preceding",
		"the method of claim",
		"the compound of claim",
		"the composition of claim",
		"the device of claim",
		"the use of claim",
		"a process according to claim",
	}
	for _, kw := range keywords {
		if strings.Contains(lowerText, kw) {
			return true
		}
	}
	return false
}

// extractDependencyRefs extracts claim dependency references from claim text.
func extractDependencyRefs(lowerText string) []int {
	re := regexp.MustCompile(`claims?\s+(\d+)`)
	matches := re.FindAllStringSubmatch(lowerText, -1)
	seen := make(map[int]bool)
	refs := make([]int, 0, len(matches))
	for _, m := range matches {
		if len(m) >= 2 {
			num, err := strconv.Atoi(m[1])
			if err == nil && !seen[num] {
				seen[num] = true
				refs = append(refs, num)
			}
		}
	}
	return refs
}

// claimTreeNode represents a node in the claim dependency tree.
type claimTreeNode struct {
	Claim    ClaimSummary    `json:"claim"`
	Children []claimTreeNode `json:"children,omitempty"`
}

// buildClaimTree builds a dependency tree from parsed claims.
func buildClaimTree(claims []ClaimSummary) []claimTreeNode {
	nodeMap := make(map[int]*claimTreeNode, len(claims))
	for i := range claims {
		node := &claimTreeNode{Claim: claims[i]}
		nodeMap[claims[i].Number] = node
	}

	var roots []claimTreeNode
	seen := make(map[int]bool)

	for _, c := range claims {
		if len(c.DependsOn) == 0 {
			if node, ok := nodeMap[c.Number]; ok {
				roots = append(roots, *node)
				seen[c.Number] = true
			}
		} else {
			for _, depNum := range c.DependsOn {
				if parent, ok := nodeMap[depNum]; ok {
					child := nodeMap[c.Number]
					parent.Children = append(parent.Children, *child)
					seen[c.Number] = true
				}
			}
		}
	}

	// Any claim not attached as a child becomes a root
	for _, c := range claims {
		if !seen[c.Number] {
			if node, ok := nodeMap[c.Number]; ok {
				roots = append(roots, *node)
			}
		}
	}

	return roots
}

//Personal.AI order the ending
