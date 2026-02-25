// Phase 11 - File 262: internal/interfaces/http/handlers/lifecycle_handler.go
// 实现专利生命周期管理 HTTP Handler。
//
// 实现要求:
// * 功能定位：处理专利生命周期相关的 HTTP 请求
// * 核心实现：
//   - GetLifecycle / AdvancePhase / AddMilestone / ListMilestones
//   - RecordFee / ListFees / GetTimeline / GetUpcomingDeadlines
//   - RegisterRoutes
// * 依赖：internal/application/lifecycle/tracking.go
// * 被依赖：internal/interfaces/http/router.go
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/turtacn/KeyIP-Intelligence/internal/application/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// LifecycleHandler handles HTTP requests for patent lifecycle management.
type LifecycleHandler struct {
	lifecycleSvc lifecycle.TrackingService
	logger       logging.Logger
}

// NewLifecycleHandler creates a new LifecycleHandler.
func NewLifecycleHandler(
	lifecycleSvc lifecycle.TrackingService,
	logger logging.Logger,
) *LifecycleHandler {
	return &LifecycleHandler{
		lifecycleSvc: lifecycleSvc,
		logger:       logger,
	}
}

// AdvancePhaseRequest is the request body for advancing a patent phase.
type AdvancePhaseRequest struct {
	TargetPhase string `json:"target_phase"`
	Notes       string `json:"notes"`
}

// AddMilestoneRequest is the request body for adding a milestone.
type AddMilestoneRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Type        string `json:"type"`
}

// RecordFeeRequest is the request body for recording a fee payment.
type RecordFeeRequest struct {
	FeeType     string  `json:"fee_type"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	PaidDate    string  `json:"paid_date"`
	DueDate     string  `json:"due_date"`
	Description string  `json:"description"`
}

// RegisterRoutes registers lifecycle routes.
func (h *LifecycleHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/patents/{patentId}/lifecycle", h.GetLifecycle)
	mux.HandleFunc("POST /api/v1/patents/{patentId}/lifecycle/advance", h.AdvancePhase)
	mux.HandleFunc("POST /api/v1/patents/{patentId}/milestones", h.AddMilestone)
	mux.HandleFunc("GET /api/v1/patents/{patentId}/milestones", h.ListMilestones)
	mux.HandleFunc("POST /api/v1/patents/{patentId}/fees", h.RecordFee)
	mux.HandleFunc("GET /api/v1/patents/{patentId}/fees", h.ListFees)
	mux.HandleFunc("GET /api/v1/patents/{patentId}/timeline", h.GetTimeline)
	mux.HandleFunc("GET /api/v1/deadlines/upcoming", h.GetUpcomingDeadlines)
}

// GetLifecycle handles GET /api/v1/patents/{patentId}/lifecycle
func (h *LifecycleHandler) GetLifecycle(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	if patentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent id is required"))
		return
	}

	lc, err := h.lifecycleSvc.GetLifecycle(r.Context(), patentID)
	if err != nil {
		h.logger.Error("failed to get lifecycle", "error", err, "patent_id", patentID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, lc)
}

// AdvancePhase handles POST /api/v1/patents/{patentId}/lifecycle/advance
func (h *LifecycleHandler) AdvancePhase(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	if patentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent id is required"))
		return
	}

	var req AdvancePhaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}

	if req.TargetPhase == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("target_phase is required"))
		return
	}

	userID := getUserIDFromContext(r)

	input := &lifecycle.AdvancePhaseInput{
		PatentID:    patentID,
		TargetPhase: req.TargetPhase,
		Notes:       req.Notes,
		UserID:      userID,
	}

	result, err := h.lifecycleSvc.AdvancePhase(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to advance phase", "error", err, "patent_id", patentID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// AddMilestone handles POST /api/v1/patents/{patentId}/milestones
func (h *LifecycleHandler) AddMilestone(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	if patentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent id is required"))
		return
	}

	var req AddMilestoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("title is required"))
		return
	}

	userID := getUserIDFromContext(r)

	input := &lifecycle.AddMilestoneInput{
		PatentID:    patentID,
		Title:       req.Title,
		Description: req.Description,
		Date:        req.Date,
		Type:        req.Type,
		UserID:      userID,
	}

	ms, err := h.lifecycleSvc.AddMilestone(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to add milestone", "error", err, "patent_id", patentID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, ms)
}

// ListMilestones handles GET /api/v1/patents/{patentId}/milestones
func (h *LifecycleHandler) ListMilestones(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	if patentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent id is required"))
		return
	}

	result, err := h.lifecycleSvc.ListMilestones(r.Context(), patentID)
	if err != nil {
		h.logger.Error("failed to list milestones", "error", err, "patent_id", patentID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// RecordFee handles POST /api/v1/patents/{patentId}/fees
func (h *LifecycleHandler) RecordFee(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	if patentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent id is required"))
		return
	}

	var req RecordFeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("invalid request body"))
		return
	}

	if req.FeeType == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("fee_type and positive amount are required"))
		return
	}

	userID := getUserIDFromContext(r)

	input := &lifecycle.RecordFeeInput{
		PatentID:    patentID,
		FeeType:     req.FeeType,
		Amount:      req.Amount,
		Currency:    req.Currency,
		PaidDate:    req.PaidDate,
		DueDate:     req.DueDate,
		Description: req.Description,
		UserID:      userID,
	}

	fee, err := h.lifecycleSvc.RecordFee(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to record fee", "error", err, "patent_id", patentID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, fee)
}

// ListFees handles GET /api/v1/patents/{patentId}/fees
func (h *LifecycleHandler) ListFees(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	if patentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent id is required"))
		return
	}

	result, err := h.lifecycleSvc.ListFees(r.Context(), patentID)
	if err != nil {
		h.logger.Error("failed to list fees", "error", err, "patent_id", patentID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GetTimeline handles GET /api/v1/patents/{patentId}/timeline
func (h *LifecycleHandler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	if patentID == "" {
		writeError(w, http.StatusBadRequest, errors.NewValidationError("patent id is required"))
		return
	}

	timeline, err := h.lifecycleSvc.GetTimeline(r.Context(), patentID)
	if err != nil {
		h.logger.Error("failed to get timeline", "error", err, "patent_id", patentID)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, timeline)
}

// GetUpcomingDeadlines handles GET /api/v1/deadlines/upcoming
func (h *LifecycleHandler) GetUpcomingDeadlines(w http.ResponseWriter, r *http.Request) {
	userID := getUserIDFromContext(r)
	daysAhead := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if d, err := parseInt(v); err == nil && d > 0 && d <= 365 {
			daysAhead = d
		}
	}

	input := &lifecycle.UpcomingDeadlinesInput{
		UserID:    userID,
		DaysAhead: daysAhead,
	}

	deadlines, err := h.lifecycleSvc.GetUpcomingDeadlines(r.Context(), input)
	if err != nil {
		h.logger.Error("failed to get upcoming deadlines", "error", err)
		writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, deadlines)
}

// parseInt is a small helper to parse an integer from string.
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.NewValidationError("invalid integer")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

//Personal.AI order the ending
