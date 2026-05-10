// Phase 11 - File 262: internal/interfaces/http/handlers/docs_handler.go
// 实现 Swagger UI 文档和 OpenAPI 规范端点。
//
// 实现要求:
// * 功能定位：提供 API 文档浏览和 OpenAPI 规范访问端点
// * 核心实现：
//   - 使用 go:embed 嵌入 swagger.html 和 OpenAPI YAML 规范
//   - SwaggerUI: 提供交互式 API 文档界面 (GET /api/docs)
//   - OpenAPISpec: 提供原始 OpenAPI 规范 (GET /api/openapi.json)
//   - 实现 RegisterRoutes: 注册文档路由
// * 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	_ "embed"
	"net/http"
)

//go:embed swagger.html
var swaggerHTML string

//go:embed keyip.yaml
// Note: keyip.yaml is a copy of api/openapi/v1/keyip.yaml kept in sync manually.
// When updating the canonical OpenAPI spec, update this copy as well.
var openapiSpec []byte

// DocsHandler serves the Swagger UI documentation page and raw OpenAPI spec.
type DocsHandler struct{}

// NewDocsHandler creates a new DocsHandler.
func NewDocsHandler() *DocsHandler {
	return &DocsHandler{}
}

// RegisterRoutes registers documentation routes on the given mux.
func (h *DocsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/docs", h.SwaggerUI)
	mux.HandleFunc("GET /api/openapi.json", h.OpenAPISpec)
}

// SwaggerUI handles GET /api/docs - serves the interactive Swagger UI documentation.
func (h *DocsHandler) SwaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerHTML))
}

// OpenAPISpec handles GET /api/openapi.json - serves the raw OpenAPI specification.
func (h *DocsHandler) OpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openapiSpec)
}

//Personal.AI order the ending
