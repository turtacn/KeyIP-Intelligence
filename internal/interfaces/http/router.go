package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// RouterConfig holds configuration for the router.
type RouterConfig struct {
	APIPrefix       string
	EnableCORS      bool
	EnableRateLimit bool
	EnableAuth      bool
	EnableTenant    bool
	Logger          logging.Logger
}

// RouterDeps holds dependencies for the router.
type RouterDeps struct {
	MoleculeHandler      *handlers.MoleculeHandler
	PatentHandler        *handlers.PatentHandler
	PortfolioHandler     *handlers.PortfolioHandler
	LifecycleHandler     *handlers.LifecycleHandler
	CollaborationHandler *handlers.CollaborationHandler
	ReportHandler        *handlers.ReportHandler
	HealthHandler        *handlers.HealthHandler

	AuthMiddleware      func(http.Handler) http.Handler
	CORSMiddleware      func(http.Handler) http.Handler
	LoggingMiddleware   func(http.Handler) http.Handler
	RateLimitMiddleware func(http.Handler) http.Handler
	TenantMiddleware    func(http.Handler) http.Handler
}

// NewRouter constructs the complete HTTP route tree.
func NewRouter(cfg RouterConfig, deps RouterDeps) http.Handler {
	if cfg.APIPrefix == "" {
		cfg.APIPrefix = "/api/v1"
	}

	mux := http.NewServeMux()

	// Public routes
	if deps.HealthHandler != nil {
		mux.HandleFunc("GET /healthz", deps.HealthHandler.Liveness)
		mux.HandleFunc("GET /readyz", deps.HealthHandler.Readiness)
		mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotImplemented)
		})
	}

	// API routes
	apiMux := http.NewServeMux()
	registerMoleculeRoutes(apiMux, deps.MoleculeHandler)
	registerPatentRoutes(apiMux, deps.PatentHandler)
	registerPortfolioRoutes(apiMux, deps.PortfolioHandler)
	registerLifecycleRoutes(apiMux, deps.LifecycleHandler)
	registerCollaborationRoutes(apiMux, deps.CollaborationHandler)
	registerReportRoutes(apiMux, deps.ReportHandler)

	apiHandler := http.Handler(apiMux)

	if cfg.EnableAuth && deps.AuthMiddleware != nil {
		apiHandler = deps.AuthMiddleware(apiHandler)
	}
	if cfg.EnableTenant && deps.TenantMiddleware != nil {
		apiHandler = deps.TenantMiddleware(apiHandler)
	}

	mux.Handle(cfg.APIPrefix+"/", http.StripPrefix(cfg.APIPrefix, apiHandler))

	// Global Middleware Chain
	var rootHandler http.Handler = mux

	if cfg.EnableRateLimit && deps.RateLimitMiddleware != nil {
		rootHandler = deps.RateLimitMiddleware(rootHandler)
	}
	if cfg.EnableCORS && deps.CORSMiddleware != nil {
		rootHandler = deps.CORSMiddleware(rootHandler)
	}
	if deps.LoggingMiddleware != nil {
		rootHandler = deps.LoggingMiddleware(rootHandler)
	}

	rootHandler = requestIDHandler(rootHandler)
	rootHandler = recoveryHandler(rootHandler, cfg.Logger)

	return rootHandler
}

func registerMoleculeRoutes(mux *http.ServeMux, h *handlers.MoleculeHandler) {
	if h == nil {
		return
	}
	mux.HandleFunc("POST /molecules/similarity-search", h.SearchSimilar)
	// mux.HandleFunc("POST /molecules/parse", h.Parse)
	mux.HandleFunc("GET /molecules/{id}", h.Get)
	mux.HandleFunc("POST /molecules", h.Create)
	mux.HandleFunc("GET /molecules", h.List)
	mux.HandleFunc("PUT /molecules/{id}", h.Update)
	mux.HandleFunc("DELETE /molecules/{id}", h.Delete)
	mux.HandleFunc("POST /molecules/search/structure", h.SearchByStructure)
	mux.HandleFunc("POST /molecules/properties/calculate", h.CalculateProperties)
}

func registerPatentRoutes(mux *http.ServeMux, h *handlers.PatentHandler) {
	if h == nil {
		return
	}
	mux.HandleFunc("GET /patents/{id}", h.Get)
	mux.HandleFunc("POST /patents/search", h.Search)
	mux.HandleFunc("GET /patents/{id}/claims/analyze", h.AnalyzeClaims)
	// mux.HandleFunc("POST /patents/patentability", h.AssessPatentability)
	// mux.HandleFunc("POST /patents/white-space", h.WhiteSpaceAnalysis)
	mux.HandleFunc("GET /patents", h.List)
	mux.HandleFunc("POST /patents", h.Create)
	mux.HandleFunc("PUT /patents/{id}", h.Update)
	mux.HandleFunc("DELETE /patents/{id}", h.Delete)
	mux.HandleFunc("GET /patents/{id}/family", h.GetFamily)
	mux.HandleFunc("GET /patents/{id}/citations", h.GetCitationNetwork)
	mux.HandleFunc("POST /patents/fto/check", h.CheckFTO)
}

func registerPortfolioRoutes(mux *http.ServeMux, h *handlers.PortfolioHandler) {
	if h == nil {
		return
	}
	mux.HandleFunc("GET /portfolios/{id}", h.Get)
	// mux.HandleFunc("POST /portfolios/{id}/valuation", h.Valuate)
	// mux.HandleFunc("POST /portfolios/{id}/gaps", h.GapAnalysis)
	// mux.HandleFunc("POST /portfolios/{id}/optimize", h.Optimize)
	// mux.HandleFunc("GET /portfolios/{id}/constellation", h.Constellation)
	mux.HandleFunc("GET /portfolios", h.List)
	mux.HandleFunc("POST /portfolios", h.Create)
	mux.HandleFunc("PUT /portfolios/{id}", h.Update)
	mux.HandleFunc("DELETE /portfolios/{id}", h.Delete)
	mux.HandleFunc("GET /portfolios/{id}/patents", h.ListPortfolioPatents)
	mux.HandleFunc("POST /portfolios/{id}/patents", h.AddPatents)
	mux.HandleFunc("DELETE /portfolios/{id}/patents/{patentNumber}", h.RemovePatent)
	mux.HandleFunc("POST /portfolios/{id}/assess", h.AssessPortfolio)
	mux.HandleFunc("GET /portfolios/{id}/analytics", h.GetAnalytics)
	mux.HandleFunc("GET /portfolios/{id}/recommendations", h.GetRecommendations)
	mux.HandleFunc("POST /portfolios/{id}/optimize", h.OptimizePortfolio)
	mux.HandleFunc("GET /portfolios/{id}/timeline", h.GetTimeline)
	mux.HandleFunc("GET /portfolios/{id}/heatmap", h.GetJurisdictionHeatmap)
}

func registerLifecycleRoutes(mux *http.ServeMux, h *handlers.LifecycleHandler) {
	if h == nil {
		return
	}
	// mux.HandleFunc("GET /lifecycle/patents/{id}", h.GetPatentLifecycle)
	mux.HandleFunc("GET /lifecycle/deadlines", h.ListDeadlines)
	mux.HandleFunc("GET /lifecycle/deadlines/upcoming", h.ListUpcomingDeadlines)
	// mux.HandleFunc("GET /lifecycle/annuities", h.ListAnnuities)
	mux.HandleFunc("POST /lifecycle/legal-status/sync", h.SyncLegalStatus)
	// mux.HandleFunc("GET /lifecycle/calendar", h.GetCalendar)

	mux.HandleFunc("POST /lifecycle/deadlines", h.CreateDeadline)
	mux.HandleFunc("PUT /lifecycle/deadlines/{deadlineId}", h.UpdateDeadline)
	mux.HandleFunc("PUT /lifecycle/deadlines/{deadlineId}/complete", h.CompleteDeadline)
	mux.HandleFunc("GET /lifecycle/patents/{patentNumber}/annuities", h.GetAnnuities)
	mux.HandleFunc("POST /lifecycle/patents/{patentNumber}/annuities/calculate", h.CalculateAnnuity)
	mux.HandleFunc("GET /lifecycle/patents/{patentNumber}/annuities/forecast", h.ForecastAnnuities)
	mux.HandleFunc("GET /lifecycle/patents/{patentNumber}/legal-status", h.GetLegalStatus)
	mux.HandleFunc("GET /lifecycle/patents/{patentNumber}/legal-status/history", h.GetLegalStatusHistory)
	mux.HandleFunc("GET /lifecycle/reminders", h.ListReminders)
	mux.HandleFunc("POST /lifecycle/reminders", h.CreateReminder)
	mux.HandleFunc("PUT /lifecycle/reminders/{reminderId}", h.UpdateReminder)
	mux.HandleFunc("DELETE /lifecycle/reminders/{reminderId}", h.DeleteReminder)
}

func registerCollaborationRoutes(mux *http.ServeMux, h *handlers.CollaborationHandler) {
	if h == nil {
		return
	}
	// mux.HandleFunc("POST /workspaces", h.CreateWorkspace)
	// mux.HandleFunc("GET /workspaces/{id}", h.GetWorkspace)
	// mux.HandleFunc("POST /workspaces/{id}/share", h.Share)
	// mux.HandleFunc("DELETE /workspaces/{id}/shares/{shareId}", h.RevokeShare)

	mux.HandleFunc("GET /collaboration/teams", h.ListTeams)
	mux.HandleFunc("POST /collaboration/teams", h.CreateTeam)
	mux.HandleFunc("GET /collaboration/teams/{teamId}", h.GetTeam)
	mux.HandleFunc("PUT /collaboration/teams/{teamId}", h.UpdateTeam)
	mux.HandleFunc("DELETE /collaboration/teams/{teamId}", h.DeleteTeam)
	mux.HandleFunc("POST /collaboration/teams/{teamId}/members", h.AddMember)
	mux.HandleFunc("DELETE /collaboration/teams/{teamId}/members/{userId}", h.RemoveMember)
	mux.HandleFunc("PUT /collaboration/teams/{teamId}/members/{userId}/role", h.UpdateMemberRole)
	mux.HandleFunc("GET /collaboration/approvals", h.ListApprovals)
	mux.HandleFunc("POST /collaboration/approvals", h.CreateApproval)
	mux.HandleFunc("PUT /collaboration/approvals/{approvalId}/approve", h.ApproveRequest)
	mux.HandleFunc("PUT /collaboration/approvals/{approvalId}/reject", h.RejectRequest)
	mux.HandleFunc("GET /collaboration/activities", h.ListActivities)
}

func registerReportRoutes(mux *http.ServeMux, h *handlers.ReportHandler) {
	if h == nil {
		return
	}
	// mux.HandleFunc("POST /reports/fto", h.GenerateFTO)
	// mux.HandleFunc("POST /reports/infringement", h.GenerateInfringement)
	// mux.HandleFunc("POST /reports/portfolio", h.GeneratePortfolio)
	// mux.HandleFunc("GET /reports/{id}", h.GetReport)
	// mux.HandleFunc("GET /reports/{id}/download", h.Download)

	mux.HandleFunc("POST /reports/generate", h.GenerateReport)
	mux.HandleFunc("GET /reports", h.ListReports)
	mux.HandleFunc("GET /reports/{reportId}", h.GetReport)
	mux.HandleFunc("GET /reports/{reportId}/download", h.DownloadReport)
	mux.HandleFunc("DELETE /reports/{reportId}", h.DeleteReport)
	mux.HandleFunc("GET /reports/{reportId}/status", h.GetReportStatus)
	mux.HandleFunc("GET /reports/templates", h.ListTemplates)
	mux.HandleFunc("GET /reports/templates/{templateId}", h.GetTemplate)
	mux.HandleFunc("POST /reports/templates", h.CreateTemplate)
	mux.HandleFunc("PUT /reports/templates/{templateId}", h.UpdateTemplate)
	mux.HandleFunc("DELETE /reports/templates/{templateId}", h.DeleteTemplate)
}

func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func recoveryHandler(next http.Handler, logger logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				if logger != nil {
					logger.Error("panic recovered", logging.String("error", fmt.Sprintf("%v", err)))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				resp := common.ErrorResponse{
					Error: common.ErrorDetail{
						Code:    "INTERNAL_SERVER_ERROR",
						Message: "internal server error",
					},
				}
				json.NewEncoder(w).Encode(resp)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestIDHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), common.ContextKeyRequestID, reqID)
		w.Header().Set("X-Request-ID", reqID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

//Personal.AI order the ending
