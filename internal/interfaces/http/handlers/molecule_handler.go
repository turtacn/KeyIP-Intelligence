// Phase 11 - File 264: internal/interfaces/http/handlers/molecule_handler.go
// 实现分子数据 HTTP Handler。
//
// * 依赖：internal/application/molecule/service.go
// * 被依赖：internal/interfaces/http/router.go
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MoleculeHandler handles HTTP requests for molecule operations.
type MoleculeHandler struct {
	moleculeSvc molecule.Service
	logger      logging.Logger
}

// NewMoleculeHandler creates a new MoleculeHandler.
func NewMoleculeHandler(svc molecule.Service, logger logging.Logger) *MoleculeHandler {
	return &MoleculeHandler{moleculeSvc: svc, logger: logger}
}

// CreateMoleculeRequest is the request body for creating a molecule.
type CreateMoleculeRequest struct {
	Name       string                 `json:"name"`
	SMILES     string                 `json:"smiles"`
	InChI      string                 `json:"inchi,omitempty"`
	MolFormula string                 `json:"mol_formula,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
}

// UpdateMoleculeRequest is the request body for updating a molecule.
type UpdateMoleculeRequest struct {
	Name       *string                `json:"name,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
}

// StructureSearchRequest is the request body for structure search.
type StructureSearchRequest struct {
	SMILES     string `json:"smiles"`
	SearchType string `json:"search_type"` // substructure, exact
	MaxResults int    `json:"max_results"`
}

// SimilaritySearchRequest is the request body for similarity search.
type SimilaritySearchRequest struct {
	SMILES    string  `json:"smiles"`
	Threshold float64 `json:"threshold"` // 0.0 - 1.0
	MaxResults int    `json:"max_results"`
}

// CalculatePropertiesRequest is the request body for property calculation.
type CalculatePropertiesRequest struct {
	SMILES     string   `json:"smiles"`
	Properties []string `json:"properties"` // mw, logp, tpsa, hbd, hba, rotatable_bonds
}

// RegisterRoutes registers molecule routes.
func (h *MoleculeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/molecules", h.CreateMolecule)
	mux.HandleFunc("GET /api/v1/molecules", h.ListMolecules)
	mux.HandleFunc("GET /api/v1/molecules/{id}", h.GetMolecule)
	mux.HandleFunc("PUT /api/v1/molecules/{id}", h.UpdateMolecule)
	mux.HandleFunc("DELETE /api/v1/molecules/{id}", h.DeleteMolecule)
	mux.HandleFunc("POST /api/v1/molecules/search/structure", h.SearchByStructure)
	mux.HandleFunc("POST /api/v1/molecules/search/similarity", h.SearchBySimilarity)
	mux.HandleFunc("POST /api/v1/molecules/properties/calculate", h.CalculateProperties)
}

// CreateMolecule handles POST /api/v1/molecules
func (h *MoleculeHandler) CreateMolecule(w http.ResponseWriter, r *http.Request) {
	var req CreateMoleculeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}
	if req.SMILES == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("smiles is required"))
		return
	}

	userID := getUserIDFromContext(r)
	input := &molecule.CreateInput{
		Name:       req.Name,
		SMILES:     req.SMILES,
		InChI:      req.InChI,
		MolFormula: req.MolFormula,
		Properties: req.Properties,
		Tags:       req.Tags,
		UserID:     userID,
	}

	mol, err := h.moleculeSvc.Create(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to create molecule", "error", err)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mol)
}

// GetMolecule handles GET /api/v1/molecules/{id}
func (h *MoleculeHandler) GetMolecule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("molecule id is required"))
		return
	}

	mol, err := h.moleculeSvc.GetByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get molecule", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mol)
}

// ListMolecules handles GET /api/v1/molecules
func (h *MoleculeHandler) ListMolecules(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	query := r.URL.Query().Get("q")
	tag := r.URL.Query().Get("tag")

	input := &molecule.ListInput{
		Page:     page,
		PageSize: pageSize,
		Query:    query,
		Tag:      tag,
	}

	result, err := h.moleculeSvc.List(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to list molecules", "error", err)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// UpdateMolecule handles PUT /api/v1/molecules/{id}
func (h *MoleculeHandler) UpdateMolecule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("molecule id is required"))
		return
	}

	var req UpdateMoleculeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}

	userID := getUserIDFromContext(r)
	input := &molecule.UpdateInput{
		ID:         id,
		Name:       req.Name,
		Properties: req.Properties,
		Tags:       req.Tags,
		UserID:     userID,
	}

	mol, err := h.moleculeSvc.Update(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to update molecule", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mol)
}

// DeleteMolecule handles DELETE /api/v1/molecules/{id}
func (h *MoleculeHandler) DeleteMolecule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("molecule id is required"))
		return
	}

	userID := getUserIDFromContext(r)
	if err := h.moleculeSvc.Delete(r.Context(), id, userID); err != nil {
		h.logger.Error("failed to delete molecule", "error", err, "id", id)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// SearchByStructure handles POST /api/v1/molecules/search/structure
func (h *MoleculeHandler) SearchByStructure(w http.ResponseWriter, r *http.Request) {
	var req StructureSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}
	if req.SMILES == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("smiles is required"))
		return
	}
	if req.SearchType == "" {
		req.SearchType = "substructure"
	}
	if req.MaxResults <= 0 || req.MaxResults > 1000 {
		req.MaxResults = 100
	}

	input := &molecule.StructureSearchInput{
		SMILES:     req.SMILES,
		SearchType: req.SearchType,
		MaxResults: req.MaxResults,
	}

	result, err := h.moleculeSvc.SearchByStructure(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to search by structure", "error", err)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SearchBySimilarity handles POST /api/v1/molecules/search/similarity
func (h *MoleculeHandler) SearchBySimilarity(w http.ResponseWriter, r *http.Request) {
	var req SimilaritySearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}
	if req.SMILES == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("smiles is required"))
		return
	}
	if req.Threshold <= 0 || req.Threshold > 1.0 {
		req.Threshold = 0.7
	}
	if req.MaxResults <= 0 || req.MaxResults > 1000 {
		req.MaxResults = 100
	}

	input := &molecule.SimilaritySearchInput{
		SMILES:     req.SMILES,
		Threshold:  req.Threshold,
		MaxResults: req.MaxResults,
	}

	result, err := h.moleculeSvc.SearchBySimilarity(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to search by similarity", "error", err)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// CalculateProperties handles POST /api/v1/molecules/properties/calculate
func (h *MoleculeHandler) CalculateProperties(w http.ResponseWriter, r *http.Request) {
	var req CalculatePropertiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}
	if req.SMILES == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("smiles is required"))
		return
	}

	input := &molecule.CalculatePropertiesInput{
		SMILES:     req.SMILES,
		Properties: req.Properties,
	}

	result, err := h.moleculeSvc.CalculateProperties(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to calculate properties", "error", err)
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

//Personal.AI order the ending
