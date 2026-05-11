// Phase 12 - File: test/api_contract_test.go
// 验证 KeyIP-Intelligence 项目前后端 API 契约一致性。
//
// 该测试套件从三个来源收集 API 路径：
//   1. api/openapi/v1/keyip.yaml（OpenAPI 规范）
//   2. 各 handler 的 RegisterRoutes（实际 Go 路由注册）
//   3. web/src/services/*.service.ts（前端 HTTP 调用）
//
// 然后交叉验证，报告任何不匹配项。
//
// 强制约束：文件最后一行必须为 //Personal.AI order the ending

//go:build !integration

package clitest

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// =============================================================================
// 第一源：OpenAPI 规范中定义的路径
// 数据来源：api/openapi/v1/keyip.yaml → paths 节
// =============================================================================

// apiRoute 表示单个 API 路由 {METHOD} {path}
type apiRoute struct {
	Method string
	Path   string
}

func (r apiRoute) String() string { return fmt.Sprintf("%s %s", r.Method, r.Path) }

// openapiRoutes 从 OpenAPI 规范中提取的所有路径。
// 手动维护以保持与 api/openapi/v1/keyip.yaml 同步。
var openapiRoutes = []apiRoute{
	// Health
	{"GET", "/healthz"},
	{"GET", "/readyz"},
	{"GET", "/healthz/detail"},

	// Molecules
	{"GET", "/api/v1/molecules"},
	{"POST", "/api/v1/molecules"},
	{"GET", "/api/v1/molecules/{id}"},
	{"PUT", "/api/v1/molecules/{id}"},
	{"DELETE", "/api/v1/molecules/{id}"},
	{"POST", "/api/v1/molecules/search/structure"},
	{"POST", "/api/v1/molecules/search/similarity"},
	{"POST", "/api/v1/molecules/properties/calculate"},

	// Patents
	{"GET", "/api/v1/patents"},
	{"POST", "/api/v1/patents"},
	{"GET", "/api/v1/patents/{id}"},
	{"PUT", "/api/v1/patents/{id}"},
	{"DELETE", "/api/v1/patents/{id}"},
	{"POST", "/api/v1/patents/search"},
	{"POST", "/api/v1/patents/search/advanced"},
	{"GET", "/api/v1/patents/stats"},
	{"POST", "/api/v1/patents/analyze-claims"},
	{"GET", "/api/v1/patents/{id}/family"},
	{"GET", "/api/v1/patents/{id}/citations"},
	{"POST", "/api/v1/patents/check-fto"},
	{"POST", "/api/v1/patents/assess-infringement"},

	// Portfolios
	{"GET", "/api/v1/portfolios"},
	{"POST", "/api/v1/portfolios"},
	{"GET", "/api/v1/portfolios/{id}"},
	{"PUT", "/api/v1/portfolios/{id}"},
	{"DELETE", "/api/v1/portfolios/{id}"},
	{"POST", "/api/v1/portfolios/{id}/patents"},
	{"DELETE", "/api/v1/portfolios/{id}/patents"},
	{"GET", "/api/v1/portfolios/{id}/analysis"},
	{"GET", "/api/v1/portfolios/{id}/valuation"},
	{"POST", "/api/v1/portfolios/{id}/valuation/run"},
	{"GET", "/api/v1/portfolios/{id}/gap-analysis"},
	{"POST", "/api/v1/portfolios/{id}/gap-analysis/run"},
	{"POST", "/api/v1/portfolios/{id}/optimize"},

	// Lifecycle
	{"GET", "/api/v1/patents/{patentId}/lifecycle"},
	{"POST", "/api/v1/patents/{patentId}/lifecycle/advance"},
	{"GET", "/api/v1/patents/{patentId}/milestones"},
	{"POST", "/api/v1/patents/{patentId}/milestones"},
	{"GET", "/api/v1/patents/{patentId}/fees"},
	{"POST", "/api/v1/patents/{patentId}/fees"},
	{"GET", "/api/v1/patents/{patentId}/timeline"},
	{"GET", "/api/v1/deadlines/upcoming"},
	{"POST", "/api/v1/lifecycle/{id}/annuities/calculate"},
	{"GET", "/api/v1/lifecycle/{id}/annuities/budget"},
	{"POST", "/api/v1/lifecycle/{id}/legal-status/sync"},
	{"GET", "/api/v1/lifecycle/{id}/calendar/export"},

	// Collaboration
	{"GET", "/api/v1/workspaces"},
	{"POST", "/api/v1/workspaces"},
	{"GET", "/api/v1/workspaces/{id}"},
	{"PUT", "/api/v1/workspaces/{id}"},
	{"DELETE", "/api/v1/workspaces/{id}"},
	{"GET", "/api/v1/workspaces/{id}/documents"},
	{"POST", "/api/v1/workspaces/{id}/documents"},
	{"GET", "/api/v1/workspaces/{id}/members"},
	{"POST", "/api/v1/workspaces/{id}/members"},
	{"DELETE", "/api/v1/workspaces/{id}/members/{memberId}"},
	{"PUT", "/api/v1/workspaces/{id}/members/{memberId}/role"},
	{"GET", "/api/v1/workspaces/{id}/shared-resource"},
	{"DELETE", "/api/v1/workspaces/{id}/shares/{shareId}"},

	// Reporting
	{"POST", "/api/v1/reports/fto"},
	{"POST", "/api/v1/reports/infringement"},
	{"POST", "/api/v1/reports/portfolio"},
	{"GET", "/api/v1/reports"},
	{"GET", "/api/v1/reports/{report_id}/status"},
	{"GET", "/api/v1/reports/{report_id}/download"},
	{"DELETE", "/api/v1/reports/{report_id}"},
	{"GET", "/api/v1/reports/templates"},
	{"GET", "/api/v1/reports/templates/{id}"},
}

// =============================================================================
// 第二源：后端 Handler 注册的路由
// 数据来源：internal/interfaces/http/handlers/*_handler.go → RegisterRoutes
// =============================================================================

var routerRoutes = []apiRoute{
	// HealthHandler
	{"GET", "/healthz"},
	{"GET", "/readyz"},
	{"GET", "/healthz/detail"},

	// MoleculeHandler
	{"POST", "/api/v1/molecules"},
	{"GET", "/api/v1/molecules"},
	{"GET", "/api/v1/molecules/{id}"},
	{"PUT", "/api/v1/molecules/{id}"},
	{"DELETE", "/api/v1/molecules/{id}"},
	{"POST", "/api/v1/molecules/search/structure"},
	{"POST", "/api/v1/molecules/search/similarity"},
	{"POST", "/api/v1/molecules/properties/calculate"},

	// PatentHandler
	{"POST", "/api/v1/patents"},
	{"GET", "/api/v1/patents"},
	{"GET", "/api/v1/patents/{id}"},
	{"PUT", "/api/v1/patents/{id}"},
	{"DELETE", "/api/v1/patents/{id}"},
	{"POST", "/api/v1/patents/search"},
	{"POST", "/api/v1/patents/search/advanced"},
	{"GET", "/api/v1/patents/stats"},
	{"POST", "/api/v1/patents/analyze-claims"},
	{"GET", "/api/v1/patents/{id}/family"},
	{"GET", "/api/v1/patents/{id}/citations"},
	{"POST", "/api/v1/patents/check-fto"},
	{"POST", "/api/v1/patents/assess-infringement"},

	// PortfolioHandler
	{"POST", "/api/v1/portfolios"},
	{"GET", "/api/v1/portfolios"},
	{"GET", "/api/v1/portfolios/{id}"},
	{"PUT", "/api/v1/portfolios/{id}"},
	{"DELETE", "/api/v1/portfolios/{id}"},
	{"POST", "/api/v1/portfolios/{id}/patents"},
	{"DELETE", "/api/v1/portfolios/{id}/patents"},
	{"GET", "/api/v1/portfolios/{id}/analysis"},
	{"GET", "/api/v1/portfolios/{id}/valuation"},
	{"POST", "/api/v1/portfolios/{id}/valuation/run"},
	{"GET", "/api/v1/portfolios/{id}/gap-analysis"},
	{"POST", "/api/v1/portfolios/{id}/gap-analysis/run"},
	{"GET", "/api/v1/portfolios/{id}/constellation"},
	{"POST", "/api/v1/portfolios/{id}/optimize"},

	// LifecycleHandler
	{"GET", "/api/v1/patents/{patentId}/lifecycle"},
	{"POST", "/api/v1/patents/{patentId}/lifecycle/advance"},
	{"POST", "/api/v1/patents/{patentId}/milestones"},
	{"GET", "/api/v1/patents/{patentId}/milestones"},
	{"POST", "/api/v1/patents/{patentId}/fees"},
	{"GET", "/api/v1/patents/{patentId}/fees"},
	{"GET", "/api/v1/patents/{patentId}/timeline"},
	{"GET", "/api/v1/deadlines/upcoming"},
	{"POST", "/api/v1/lifecycle/{id}/annuities/calculate"},
	{"GET", "/api/v1/lifecycle/{id}/annuities/budget"},
	{"POST", "/api/v1/lifecycle/{id}/legal-status/sync"},
	{"GET", "/api/v1/lifecycle/{id}/calendar/export"},

	// CollaborationHandler
	{"POST", "/api/v1/workspaces"},
	{"GET", "/api/v1/workspaces"},
	{"GET", "/api/v1/workspaces/{id}"},
	{"PUT", "/api/v1/workspaces/{id}"},
	{"DELETE", "/api/v1/workspaces/{id}"},
	{"POST", "/api/v1/workspaces/{id}/documents"},
	{"GET", "/api/v1/workspaces/{id}/documents"},
	{"POST", "/api/v1/workspaces/{id}/members"},
	{"DELETE", "/api/v1/workspaces/{id}/members/{memberId}"},
	{"PUT", "/api/v1/workspaces/{id}/members/{memberId}/role"},
	{"GET", "/api/v1/workspaces/{id}/members"},
	{"GET", "/api/v1/workspaces/{id}/shared-resource"},
	{"DELETE", "/api/v1/workspaces/{id}/shares/{shareId}"},

	// ReportHandler
	{"POST", "/api/v1/reports/fto"},
	{"POST", "/api/v1/reports/infringement"},
	{"POST", "/api/v1/reports/portfolio"},
	{"GET", "/api/v1/reports/{report_id}/status"},
	{"GET", "/api/v1/reports/{report_id}/download"},
	{"GET", "/api/v1/reports"},
	{"DELETE", "/api/v1/reports/{report_id}"},
	{"GET", "/api/v1/reports/templates"},
	{"GET", "/api/v1/reports/templates/{id}"},

	// Extra handlers registered in router.go (no OpenAPI counterpart)
	{"GET", "/api/version"},
	{"GET", "/api/docs"},
	{"GET", "/api/openapi.json"},
	{"GET", "/api/v1/runtime/info"},
	{"GET", "/api/v1/runtime/build"},
	{"GET", "/api/v1/ws/events"},
	{"POST", "/api/v1/csp-report"},
}

// =============================================================================
// 第三源：前端 Service 调用路径
// 数据来源：web/src/services/*.service.ts
// =============================================================================

// frontendRoute 扩展了 apiRoute，用于记录前端特定的通路信息
type frontendRoute struct {
	Route    apiRoute
	Service  string // 来源 service 文件
	Params   string // 传递的参数（用于验证查询参数命名一致性）
}

// frontendRoutes 从前端所有 service 文件中提取的调用路径。
// baseUrl 固定为 /api/v1（见 web/src/utils/apiMode.ts），路径与 baseUrl 拼接。
var frontendRoutes = []frontendRoute{
	// health.service.ts
	{Route: apiRoute{"GET", "/healthz"}, Service: "health.service.ts", Params: ""},
	{Route: apiRoute{"GET", "/healthz/detail"}, Service: "health.service.ts", Params: ""},

	// molecule.service.ts
	{Route: apiRoute{"GET", "/api/v1/molecules"}, Service: "molecule.service.ts", Params: "page, pageSize"},
	{Route: apiRoute{"GET", "/api/v1/molecules/{id}"}, Service: "molecule.service.ts", Params: ""},
	// searchMolecules 复用了 GET /molecules，但使用 query 参数（注意：OpenAPI 中参数名为 q）

	// patent.service.ts
	{Route: apiRoute{"POST", "/api/v1/patents/search"}, Service: "patent.service.ts", Params: "page, page_size, query"},
	{Route: apiRoute{"GET", "/api/v1/patents/{id}"}, Service: "patent.service.ts", Params: ""},
	{Route: apiRoute{"GET", "/api/v1/patents/{id}/family"}, Service: "patent.service.ts", Params: ""},

	// portfolio.service.ts
	// 注意：路径使用 /portfolio（单数），而 OpenAPI/Router 使用 /portfolios（复数）
	{Route: apiRoute{"GET", "/api/v1/portfolio/summary"}, Service: "portfolio.service.ts", Params: ""},
	{Route: apiRoute{"GET", "/api/v1/portfolio/scores"}, Service: "portfolio.service.ts", Params: ""},
	{Route: apiRoute{"GET", "/api/v1/portfolio/coverage"}, Service: "portfolio.service.ts", Params: ""},
	{Route: apiRoute{"GET", "/api/v1/portfolios/{portfolioId}/constellation"}, Service: "portfolio.service.ts", Params: ""},

	// lifecycle.service.ts
	{Route: apiRoute{"GET", "/api/v1/lifecycle/events"}, Service: "lifecycle.service.ts", Params: "jurisdiction, status"},

	// dashboard.service.ts
	{Route: apiRoute{"GET", "/api/v1/dashboard/metrics"}, Service: "dashboard.service.ts", Params: ""},

	// infringement.service.ts
	{Route: apiRoute{"GET", "/api/v1/alerts"}, Service: "infringement.service.ts", Params: "riskLevel, page, pageSize"},

	// knowledgeGraph.service.ts
	{Route: apiRoute{"GET", "/api/v1/patents/{patentId}/citations"}, Service: "knowledgeGraph.service.ts", Params: ""},

	// partner.service.ts
	{Route: apiRoute{"GET", "/api/v1/partners"}, Service: "partner.service.ts", Params: ""},
}

// =============================================================================
// 归一化工具函数
// =============================================================================

// routeMap 构建 map[string]apiRoute，key 为 "METHOD path"
func routeMap(routes []apiRoute) map[string]apiRoute {
	m := make(map[string]apiRoute, len(routes))
	for _, r := range routes {
		m[r.String()] = r
	}
	return m
}

// routeSet 构建 set，key 为 "METHOD path"
func routeSet(routes []apiRoute) map[string]bool {
	s := make(map[string]bool, len(routes))
	for _, r := range routes {
		s[r.String()] = true
	}
	return s
}

// normalizePath 去除路径参数名差异以便比较。
// 例如：{id}、{patentId}、{portfolioId}、{report_id}、{shareId}、{memberId} 都归一化为 {param}
func normalizePath(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			segments[i] = "{param}"
		}
	}
	return strings.Join(segments, "/")
}

// collectMissing 返回在 expected 中但不在 actual 中的路由。
func collectMissing(expected []apiRoute, actual map[string]bool) []apiRoute {
	var missing []apiRoute
	for _, r := range expected {
		if !actual[r.String()] {
			missing = append(missing, r)
		}
	}
	return missing
}

// collectExtra 返回在 actual 中但不在 expected 中的路由。
func collectExtra(actual []apiRoute, expected map[string]bool) []apiRoute {
	var extra []apiRoute
	for _, r := range actual {
		if !expected[r.String()] {
			extra = append(extra, r)
		}
	}
	return extra
}

// pathParams 提取路径中的参数名。
// 例："/api/v1/patents/{patentId}/lifecycle" → ["patentId"]
func pathParams(path string) []string {
	var params []string
	segments := strings.Split(path, "/")
	for _, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			params = append(params, seg[1:len(seg)-1])
		}
	}
	return params
}

// =============================================================================
// 测试 1：OpenAPI → Router（OpenAPI 中定义的所有路径在 router 中已注册）
// =============================================================================

func TestOpenAPIRoutesRegisteredInRouter(t *testing.T) {
	routerSet := routeSet(routerRoutes)

	missing := collectMissing(openapiRoutes, routerSet)
	if len(missing) > 0 {
		sort.Slice(missing, func(i, j int) bool { return missing[i].String() < missing[j].String() })
		for _, r := range missing {
			t.Errorf("[OpenAPI→Router 缺失] OpenAPI 定义了 %s，但 router 中未注册", r)
		}
	}

	// 验证路径参数命名一致性：将路径归一化后比较
	openapiNorm := make(map[string]apiRoute)
	for _, r := range openapiRoutes {
		key := r.Method + " " + normalizePath(r.Path)
		openapiNorm[key] = r
	}
	routerNorm := make(map[string]apiRoute)
	for _, r := range routerRoutes {
		key := r.Method + " " + normalizePath(r.Path)
		routerNorm[key] = r
	}

	for key, oaRoute := range openapiNorm {
		rtRoute, found := routerNorm[key]
		if !found {
			continue // 已在 missing 中报告
		}
		oaParams := pathParams(oaRoute.Path)
		rtParams := pathParams(rtRoute.Path)
		if len(oaParams) != len(rtParams) {
			t.Errorf("[路径参数数量不匹配] OpenAPI %s (params=%v) vs Router %s (params=%v)",
				oaRoute, oaParams, rtRoute, rtParams)
			continue
		}
		for i := range oaParams {
			if oaParams[i] != rtParams[i] {
				t.Errorf("[路径参数命名不一致] OpenAPI 使用 `{%s}` 但 Router 使用 `{%s}` → 路径 %s",
					oaParams[i], rtParams[i], oaRoute.Path)
			}
		}
	}
}

// =============================================================================
// 测试 2：Router → OpenAPI（router 中注册的路径在 OpenAPI 中有定义）
// =============================================================================

func TestRouterRoutesDefinedInOpenAPI(t *testing.T) {
	openapiSet := routeSet(openapiRoutes)

	// 将 router 路由中本属于基础设施/运营类的路径排除
	// （这些属于运维端点，不需要在业务 OpenAPI 中定义）
	infraPrefixes := []string{
		"/api/version",
		"/api/docs",
		"/api/openapi.json",
		"/api/v1/runtime/",
		"/api/v1/ws/",
		"/api/v1/csp-report",
	}

	isInfra := func(r apiRoute) bool {
		for _, p := range infraPrefixes {
			if strings.HasPrefix(r.Path, p) {
				return true
			}
		}
		return false
	}

	var bizRoutes []apiRoute
	for _, r := range routerRoutes {
		if !isInfra(r) {
			bizRoutes = append(bizRoutes, r)
		}
	}

	extra := collectExtra(bizRoutes, openapiSet)
	if len(extra) > 0 {
		sort.Slice(extra, func(i, j int) bool { return extra[i].String() < extra[j].String() })
		for _, r := range extra {
			t.Errorf("[Router→OpenAPI 未定义] Router 注册了 %s，但在 OpenAPI 中未定义", r)
		}
	}
}

// =============================================================================
// 测试 3：前端 → OpenAPI（前端调用的路径在 OpenAPI 中有定义）
// 使用归一化路径匹配（忽略参数名差异），参数名不一致单独报告。
// =============================================================================

func TestFrontendRoutesDefinedInOpenAPI(t *testing.T) {
	// 构建 OpenAPI 归一化路由 map {METHOD normalized_path} → original
	openapiNorm := make(map[string]apiRoute)
	for _, r := range openapiRoutes {
		key := r.Method + " " + normalizePath(r.Path)
		openapiNorm[key] = r
	}

	for _, fr := range frontendRoutes {
		feKey := fr.Route.Method + " " + normalizePath(fr.Route.Path)
		_, found := openapiNorm[feKey]
		if !found {
			// 即使归一化后也没匹配到 → 真正缺失
			t.Errorf("[前端→OpenAPI 未定义] %s 调用了 %s，但在 OpenAPI 中未定义（来源: %s, 参数: %s）",
				fr.Service, fr.Route, fr.Service, fr.Params)
		}
	}
}

// =============================================================================
// 测试 4：前端路径参数命名与 OpenAPI 一致性
// =============================================================================

func TestFrontendPathParamNaming(t *testing.T) {
	// 构建 OpenAPI 归一化路由 map
	openapiNorm := make(map[string]apiRoute)
	for _, r := range openapiRoutes {
		key := r.Method + " " + normalizePath(r.Path)
		openapiNorm[key] = r
	}

	for _, fr := range frontendRoutes {
		feKey := fr.Route.Method + " " + normalizePath(fr.Route.Path)
		oaRoute, found := openapiNorm[feKey]
		if !found {
			continue // 已在 TestFrontendRoutesDefinedInOpenAPI 中报告
		}

		frParams := pathParams(fr.Route.Path)
		oaParams := pathParams(oaRoute.Path)
		if len(frParams) != len(oaParams) {
			t.Errorf("[前端路径参数数量不匹配] %s 的 %s 有参数 %v，但 OpenAPI 对应路径 %s 有 %v",
				fr.Service, fr.Route, frParams, oaRoute, oaParams)
			continue
		}
		for i := range frParams {
			if frParams[i] != oaParams[i] {
				t.Errorf("[前端路径参数命名不一致] %s 使用 `{%s}`，但 OpenAPI 使用 `{%s}` → 路径 %s",
					fr.Service, frParams[i], oaParams[i], fr.Route.Path)
			}
		}
	}
}

// =============================================================================
// 测试 5：前端查询参数命名与 OpenAPI 一致性
// =============================================================================

func TestFrontendQueryParamNaming(t *testing.T) {
	type paramCheck struct {
		Service   string
		Path      string
		UsedParam string // 前端实际使用的参数名
		SpecParam string // OpenAPI 中定义的参数名
		Note      string
	}

	checks := []paramCheck{
		// molecule.service.ts: getMolecules(page, pageSize) → OpenAPI 使用 page 和 page_size
		{
			Service:   "molecule.service.ts",
			Path:      "GET /api/v1/molecules",
			UsedParam: "pageSize",
			SpecParam: "page_size",
			Note:      "前端使用驼峰 pageSize，OpenAPI 使用下划线 page_size",
		},
		// patent.service.ts: getPatents() 使用 POST body，body 中 page_size → 正确
		{
			Service:   "patent.service.ts",
			Path:      "POST /api/v1/patents/search",
			UsedParam: "page_size",
			SpecParam: "page_size",
			Note:      "一致（使用 POST body）",
		},
		// infringement.service.ts: getAlerts(riskLevel, page, pageSize)
		{
			Service:   "infringement.service.ts",
			Path:      "GET /api/v1/alerts",
			UsedParam: "riskLevel",
			SpecParam: "risk_level (推测)",
			Note:      "前端使用驼峰 riskLevel，但该路径未在 OpenAPI 中定义，无法验证",
		},
		// infringement.service.ts: pageSize → page_size
		{
			Service:   "infringement.service.ts",
			Path:      "GET /api/v1/alerts",
			UsedParam: "pageSize",
			SpecParam: "page_size (推测)",
			Note:      "前端使用驼峰 pageSize",
		},
		// molecule.service.ts: searchMolecules(query) → OpenAPI 中参数名为 q
		{
			Service:   "molecule.service.ts",
			Path:      "GET /api/v1/molecules",
			UsedParam: "query",
			SpecParam: "q",
			Note:      "前端使用 query，OpenAPI 中定义为 q",
		},
	}

	for _, c := range checks {
		if c.UsedParam != c.SpecParam {
			t.Logf("[查询参数命名不一致] %s 在 %s 中使用 `%s`，但 OpenAPI 期望 `%s` → %s",
				c.Service, c.Path, c.UsedParam, c.SpecParam, c.Note)
		}
	}
}

// =============================================================================
// 测试 6：OpenAPI 路径参数命名与 Router 一致性（归一化后）
// =============================================================================

func TestOpenAPIRouterParamNamingConsistency(t *testing.T) {
	// 按归一化路径分组，检查同一路由的参数名是否一致
	type groupKey struct {
		Method string
		NPath  string // normalized path
	}

	openapiByGroup := make(map[groupKey][]apiRoute)
	for _, r := range openapiRoutes {
		key := groupKey{r.Method, normalizePath(r.Path)}
		openapiByGroup[key] = append(openapiByGroup[key], r)
	}

	routerByGroup := make(map[groupKey][]apiRoute)
	for _, r := range routerRoutes {
		key := groupKey{r.Method, normalizePath(r.Path)}
		routerByGroup[key] = append(routerByGroup[key], r)
	}

	for key, oaRoutes := range openapiByGroup {
		rtRoutes, found := routerByGroup[key]
		if !found {
			continue
		}
		for _, oa := range oaRoutes {
			for _, rt := range rtRoutes {
				oaP := pathParams(oa.Path)
				rtP := pathParams(rt.Path)
				if len(oaP) != len(rtP) {
					continue
				}
				for i := range oaP {
					if oaP[i] != rtP[i] {
						t.Errorf("[参数命名不一致] 路径 %s: OpenAPI 参数 `{%s}` 但 Router 参数 `{%s}`",
							key.NPath, oaP[i], rtP[i])
					}
				}
			}
		}
	}
}

// =============================================================================
// 测试 7：OpenAPI servers 配置一致性
// =============================================================================

func TestOpenAPIServerConfig(t *testing.T) {
	// OpenAPI spec 中 servers 列表：
	//   - https://api.keyip.example.com/api/v1
	//   - https://staging-api.keyip.example.com/api/v1
	//   - http://localhost:8080
	//
	// 注意到前两个 server 以 /api/v1 结尾，但第三个没有。
	// 这意味着：
	//   - 生产环境：/healthz → https://api.keyip.example.com/api/v1/healthz
	//   - 本地环境：/healthz → http://localhost:8080/healthz
	//
	// 而前端 proxy mode（web/src/utils/apiMode.ts）的 baseUrl 固定为
	// http://localhost:8080/api/v1，会调用 http://localhost:8080/api/v1/healthz，
	// 与本地 OpenAPI server 基础的 /healthz 不一致。

	t.Log("[OpenAPI servers 不一致] 前两个 server URL 以 /api/v1 结尾，第三个 http://localhost:8080 没有 /api/v1 后缀")
	t.Log("[OpenAPI servers 不一致] 这意味着 /healthz 的完整 URL 在本地是 http://localhost:8080/healthz")
	t.Log("[OpenAPI servers 不一致] 但前端 proxy baseUrl 是 http://localhost:8080/api/v1，调用 healthz 得到 http://localhost:8080/api/v1/healthz → 不匹配")
}

// =============================================================================
// 测试 8：前端特有路径总览（不在 OpenAPI/Router 中的前端调用）
// =============================================================================

func TestFrontendOnlyPaths(t *testing.T) {
	// 构建归一化路由 set
	openapiNorm := make(map[string]bool)
	for _, r := range openapiRoutes {
		openapiNorm[r.Method+" "+normalizePath(r.Path)] = true
	}
	routerNorm := make(map[string]bool)
	for _, r := range routerRoutes {
		routerNorm[r.Method+" "+normalizePath(r.Path)] = true
	}

	type feOnly struct {
		Route     apiRoute
		InOpenAPI bool
		InRouter  bool
		Service   string
	}

	var only []feOnly
	for _, fr := range frontendRoutes {
		normKey := fr.Route.Method + " " + normalizePath(fr.Route.Path)
		_, inOA := openapiNorm[normKey]
		_, inRT := routerNorm[normKey]
		if !inOA || !inRT {
			only = append(only, feOnly{
				Route:     fr.Route,
				InOpenAPI: inOA,
				InRouter:  inRT,
				Service:   fr.Service,
			})
		}
	}

	if len(only) > 0 {
		t.Logf("=== 前端特有路径（共 %d 条，未在 OpenAPI 和/或 Router 中定义）===", len(only))
		for _, o := range only {
			oaStatus := "已定义"
			if !o.InOpenAPI {
				oaStatus = "未定义"
			}
			rtStatus := "已注册"
			if !o.InRouter {
				rtStatus = "未注册"
			}
			t.Logf("  %-30s OpenAPI: %s | Router: %s | 来源: %s",
				o.Route.String(), oaStatus, rtStatus, o.Service)
		}
	}
}

// =============================================================================
// 测试总结：运行 go test ./test/... -short 汇总
// =============================================================================

func TestAPIContractSummary(t *testing.T) {
	if !testing.Short() {
		t.Skip("跳过摘要，仅在 -short 模式运行")
	}

	t.Log("===== API 契约一致性验证摘要 =====")
	t.Logf("OpenAPI 定义路径: %d 条", len(openapiRoutes))
	t.Logf("Router 注册路径:  %d 条（含 %d 条基础设施路径）", len(routerRoutes), 7)
	t.Logf("前端 Service 调用: %d 条", len(frontendRoutes))

	t.Log("")
	t.Log("--- 路径不匹配报告 ---")

	// 1. Router 中注册但 OpenAPI 未定义的业务路径（归一化匹配）
	openapiNorm := make(map[string]bool)
	for _, r := range openapiRoutes {
		openapiNorm[r.Method+" "+normalizePath(r.Path)] = true
	}
	var bizRouterExtra []apiRoute
	infraPrefixes := []string{
		"/api/version", "/api/docs", "/api/openapi.json",
		"/api/v1/runtime/", "/api/v1/ws/", "/api/v1/csp-report",
	}
	for _, r := range routerRoutes {
		isInfra := false
		for _, p := range infraPrefixes {
			if strings.HasPrefix(r.Path, p) {
				isInfra = true
				break
			}
		}
		if !isInfra && !openapiNorm[r.Method+" "+normalizePath(r.Path)] {
			bizRouterExtra = append(bizRouterExtra, r)
		}
	}
	for _, r := range bizRouterExtra {
		t.Logf("  [Router 有/OpenAPI 无] %s", r)
	}

	// 2. 前端调用但 OpenAPI 未定义的路径（归一化匹配）
	for _, fr := range frontendRoutes {
		if !openapiNorm[fr.Route.Method+" "+normalizePath(fr.Route.Path)] {
			t.Logf("  [前端有/OpenAPI 无] %s (来源: %s)", fr.Route, fr.Service)
		}
	}

	// 3. 前端调用但 Router 未注册的路径（归一化匹配）
	routerNorm := make(map[string]bool)
	for _, r := range routerRoutes {
		routerNorm[r.Method+" "+normalizePath(r.Path)] = true
	}
	for _, fr := range frontendRoutes {
		if !routerNorm[fr.Route.Method+" "+normalizePath(fr.Route.Path)] {
			t.Logf("  [前端有/Router 无] %s (来源: %s)", fr.Route, fr.Service)
		}
	}

	t.Log("")
	t.Log("--- 参数命名差异 ---")
	t.Log("  molecule.service.ts: 使用 query 参数, OpenAPI 定义为 q")
	t.Log("  molecule.service.ts: 使用 pageSize 参数, OpenAPI 定义为 page_size")
	t.Log("  portfolio.service.ts: 使用 /portfolio (单数), OpenAPI/Router 使用 /portfolios (复数)")
	t.Log("  knowledgeGraph.service.ts: 使用 {patentId}, OpenAPI/Router 使用 {id}")
	t.Log("  OpenAPI server URLs: 前两个以 /api/v1 结尾, 第三个没有 (localhost:8080)")
	t.Log("  前端 health 调用: 使用 baseUrl /api/v1 导致 /api/v1/healthz, 但本地 server 只有 /healthz")
}

//Personal.AI order the ending
