// ---
//
// 继续输出 282 `internal/interfaces/http/router.go` 要实现 HTTP 路由注册与分组。
//
// 实现要求:
//
// * **功能定位**：HTTP 路由总线，将所有 Handler 和 Middleware 组装为完整的路由树，
//   作为 HTTP Server 的核心路由配置入口
// * **核心实现**：
//   * 定义 `RouterConfig` 结构体：聚合所有 Handler 和 Middleware 依赖
//     - MoleculeHandler, PatentHandler, PortfolioHandler, LifecycleHandler,
//       CollaborationHandler, ReportHandler, HealthHandler
//     - AuthMiddleware, CORSMiddleware, LoggingMiddleware, RateLimitMiddleware, TenantMiddleware
//     - Logger logging.Logger
//   * 定义 `NewRouter(cfg RouterConfig) http.Handler`：
//     - 创建 chi.Router 实例
//     - 注册全局中间件链：Recovery → CORS → Logging → RateLimit
//     - 注册健康检查路由组 `/healthz`、`/readyz`
//     - 注册 API v1 路由组 `/api/v1`，内部按资源分组：
//       · `/molecules` → MoleculeHandler 的 CRUD + 相似度搜索 + 属性预测
//       · `/patents` → PatentHandler 的 CRUD + 搜索 + 权利要求分析 + FTO + 家族 + 引用网络
//       · `/portfolios` → PortfolioHandler 的 CRUD + 估值 + 差距分析 + 优化
//       · `/lifecycle` → LifecycleHandler 的期限 + 年金 + 法律状态 + 日历
//       · `/collaboration` → CollaborationHandler 的工作空间 + 分享 + 权限
//       · `/reports` → ReportHandler 的生成 + 模板 + 下载
//     - 认证路由组内嵌 AuthMiddleware + TenantMiddleware
//   * 定义 `registerMoleculeRoutes(r chi.Router, h *handlers.MoleculeHandler)` 私有函数
//   * 定义 `registerPatentRoutes(r chi.Router, h *handlers.PatentHandler)` 私有函数
//   * 定义 `registerPortfolioRoutes(r chi.Router, h *handlers.PortfolioHandler)` 私有函数
//   * 定义 `registerLifecycleRoutes(r chi.Router, h *handlers.LifecycleHandler)` 私有函数
//   * 定义 `registerCollaborationRoutes(r chi.Router, h *handlers.CollaborationHandler)` 私有函数
//   * 定义 `registerReportRoutes(r chi.Router, h *handlers.ReportHandler)` 私有函数
// * **业务逻辑**：
//   - 健康检查端点不经过认证中间件
//   - API v1 路由组统一经过认证和租户中间件
//   - 每个资源路由组内部遵循 RESTful 约定：GET/POST/PUT/DELETE
//   - 搜索类端点使用 GET + Query Parameters
//   - 复杂分析类端点使用 POST + Request Body
// * **依赖关系**：
//   * 依赖：internal/interfaces/http/handlers/*、internal/interfaces/http/middleware/*、
//     internal/infrastructure/monitoring/logging/logger.go、github.com/go-chi/chi/v5
//   * 被依赖：internal/interfaces/http/server.go、cmd/apiserver/main.go
// * **测试要求**：路由注册完整性、中间件链顺序、路由分组隔离、健康检查无认证
// * **强制约束**：文件最后一行必须为 `//Personal.AI order the ending`
//
// ---
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/handlers"
	"github.com/turtacn/KeyIP-Intelligence/internal/interfaces/http/middleware"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// RouterConfig aggregates all handler and middleware dependencies required
// to construct the complete HTTP route tree.
type RouterConfig struct {
	// Handlers
	MoleculeHandler      *handlers.MoleculeHandler
	PatentHandler        *handlers.PatentHandler
	PortfolioHandler     *handlers.PortfolioHandler
	LifecycleHandler     *handlers.LifecycleHandler
	CollaborationHandler *handlers.CollaborationHandler
	ReportHandler        *handlers.ReportHandler
	HealthHandler        *handlers.HealthHandler

	// Middleware
	AuthMiddleware      *middleware.AuthMiddleware
	CORSMiddleware      *middleware.CORSMiddleware
	LoggingMiddleware   *middleware.LoggingMiddleware
	RateLimitMiddleware *middleware.RateLimitMiddleware
	TenantMiddleware    *middleware.TenantMiddleware

	// Infrastructure
	Logger logging.Logger
}

// NewRouter constructs the complete HTTP route tree from the given configuration.
// It wires global middleware, public health endpoints, and authenticated API v1
// resource groups into a single http.Handler suitable for use with http.Server.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	// --- Global middleware (applied to every request) ---
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)

	if cfg.CORSMiddleware != nil {
		r.Use(cfg.CORSMiddleware.Handler)
	}
	if cfg.LoggingMiddleware != nil {
		r.Use(cfg.LoggingMiddleware.Handler)
	}
	if cfg.RateLimitMiddleware != nil {
		r.Use(cfg.RateLimitMiddleware.Handler)
	}

	// --- Public health endpoints (no auth) ---
	r.Group(func(pub chi.Router) {
		if cfg.HealthHandler != nil {
			pub.Get("/healthz", cfg.HealthHandler.Liveness)
			pub.Get("/readyz", cfg.HealthHandler.Readiness)
		}
	})

	// --- API v1 (authenticated + tenant-scoped) ---
	r.Route("/api/v1", func(api chi.Router) {
		if cfg.AuthMiddleware != nil {
			api.Use(cfg.AuthMiddleware.Handler)
		}
		if cfg.TenantMiddleware != nil {
			api.Use(cfg.TenantMiddleware.Handler)
		}

		registerMoleculeRoutes(api, cfg.MoleculeHandler)
		registerPatentRoutes(api, cfg.PatentHandler)
		registerPortfolioRoutes(api, cfg.PortfolioHandler)
		registerLifecycleRoutes(api, cfg.LifecycleHandler)
		registerCollaborationRoutes(api, cfg.CollaborationHandler)
		registerReportRoutes(api, cfg.ReportHandler)
	})

	return r
}

// registerMoleculeRoutes mounts molecule resource endpoints under /molecules.
func registerMoleculeRoutes(r chi.Router, h *handlers.MoleculeHandler) {
	if h == nil {
		return
	}
	r.Route("/molecules", func(mr chi.Router) {
		mr.Get("/", h.List)
		mr.Post("/", h.Create)

		mr.Route("/{moleculeID}", func(item chi.Router) {
			item.Get("/", h.Get)
			item.Put("/", h.Update)
			item.Delete("/", h.Delete)
		})

		// Analytical endpoints
		mr.Post("/search/similar", h.SearchSimilar)
		mr.Post("/predict/properties", h.PredictProperties)
	})
}

// registerPatentRoutes mounts patent resource endpoints under /patents.
func registerPatentRoutes(r chi.Router, h *handlers.PatentHandler) {
	if h == nil {
		return
	}
	r.Route("/patents", func(pr chi.Router) {
		pr.Get("/", h.List)
		pr.Post("/", h.Create)
		pr.Get("/search", h.Search)

		pr.Route("/{patentNumber}", func(item chi.Router) {
			item.Get("/", h.Get)
			item.Put("/", h.Update)
			item.Delete("/", h.Delete)
			item.Get("/claims", h.AnalyzeClaims)
			item.Get("/family", h.GetFamily)
			item.Get("/citations", h.GetCitationNetwork)
		})

		// Complex analysis endpoints (POST with body)
		pr.Post("/fto/check", h.CheckFTO)
	})
}

// registerPortfolioRoutes mounts portfolio resource endpoints under /portfolios.
func registerPortfolioRoutes(r chi.Router, h *handlers.PortfolioHandler) {
	if h == nil {
		return
	}
	r.Route("/portfolios", func(pr chi.Router) {
		pr.Get("/", h.List)
		pr.Post("/", h.Create)

		pr.Route("/{portfolioID}", func(item chi.Router) {
			item.Get("/", h.Get)
			item.Put("/", h.Update)
			item.Delete("/", h.Delete)
			item.Get("/valuation", h.GetValuation)
			item.Post("/valuation", h.RunValuation)
			item.Get("/gaps", h.GetGapAnalysis)
			item.Post("/gaps", h.RunGapAnalysis)
			item.Post("/optimize", h.Optimize)
		})
	})
}

// registerLifecycleRoutes mounts lifecycle management endpoints under /lifecycle.
func registerLifecycleRoutes(r chi.Router, h *handlers.LifecycleHandler) {
	if h == nil {
		return
	}
	r.Route("/lifecycle", func(lr chi.Router) {
		// Deadline management
		lr.Get("/deadlines", h.ListDeadlines)
		lr.Get("/deadlines/upcoming", h.ListUpcomingDeadlines)

		// Annuity management
		lr.Get("/annuities", h.ListAnnuities)
		lr.Post("/annuities/calculate", h.CalculateAnnuities)
		lr.Get("/annuities/budget", h.GetAnnuityBudget)

		// Legal status
		lr.Get("/legal-status/{patentNumber}", h.GetLegalStatus)
		lr.Post("/legal-status/sync", h.SyncLegalStatus)

		// Calendar
		lr.Get("/calendar", h.GetCalendar)
		lr.Get("/calendar/export", h.ExportCalendar)
	})
}

// registerCollaborationRoutes mounts collaboration endpoints under /collaboration.
func registerCollaborationRoutes(r chi.Router, h *handlers.CollaborationHandler) {
	if h == nil {
		return
	}
	r.Route("/collaboration", func(cr chi.Router) {
		// Workspaces
		cr.Get("/workspaces", h.ListWorkspaces)
		cr.Post("/workspaces", h.CreateWorkspace)

		cr.Route("/workspaces/{workspaceID}", func(ws chi.Router) {
			ws.Get("/", h.GetWorkspace)
			ws.Put("/", h.UpdateWorkspace)
			ws.Delete("/", h.DeleteWorkspace)
			ws.Get("/members", h.ListMembers)
			ws.Post("/members", h.AddMember)
			ws.Delete("/members/{userID}", h.RemoveMember)
		})

		// Sharing
		cr.Post("/share", h.CreateShareLink)
		cr.Get("/share/{shareToken}", h.GetSharedResource)
		cr.Delete("/share/{shareToken}", h.RevokeShareLink)
	})
}

// registerReportRoutes mounts report generation endpoints under /reports.
func registerReportRoutes(r chi.Router, h *handlers.ReportHandler) {
	if h == nil {
		return
	}
	r.Route("/reports", func(rr chi.Router) {
		rr.Get("/", h.List)
		rr.Post("/generate", h.Generate)

		rr.Route("/{reportID}", func(item chi.Router) {
			item.Get("/", h.Get)
			item.Get("/download", h.Download)
			item.Delete("/", h.Delete)
		})

		// Templates
		rr.Get("/templates", h.ListTemplates)
		rr.Get("/templates/{templateID}", h.GetTemplate)
	})
}

//Personal.AI order the ending
