// Phase 11 - File 263: internal/interfaces/http/handlers/docs_handler_test.go
// 实现 Swagger UI 文档和 OpenAPI 规范端点测试。
//
// 实现要求:
// * 测试用例：
//   - TestDocsHandler_SwaggerUI: 验证 Swagger UI 页面返回 HTML 且包含关键元素
//   - TestDocsHandler_OpenAPISpec: 验证 OpenAPI 规范返回 JSON/YAML 且包含 API 信息
//   - TestDocsHandler_RegisterRoutes: 验证路由正确注册和响应
//
// 强制约束：文件最后一行必须为 //Personal.AI order the ending

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDocsHandler_SwaggerUI(t *testing.T) {
	h := NewDocsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
	rec := httptest.NewRecorder()

	h.SwaggerUI(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, rec.Body.String(), "swagger-ui")
	assert.Contains(t, rec.Body.String(), "/api/openapi.json")
	assert.Contains(t, rec.Body.String(), "KeyIP-Intelligence")
}

func TestDocsHandler_OpenAPISpec(t *testing.T) {
	h := NewDocsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	rec := httptest.NewRecorder()

	h.OpenAPISpec(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "openapi")
	assert.Contains(t, body, "KeyIP-Intelligence API")
	assert.Contains(t, body, "/api/v1/molecules")
	assert.Contains(t, body, "/api/v1/patents")
	assert.Contains(t, body, "/api/v1/portfolios")
}

func TestDocsHandler_RegisterRoutes(t *testing.T) {
	h := NewDocsHandler()
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	t.Run("openapi spec route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "openapi")
	})

	t.Run("swagger ui route", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/docs", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "swagger-ui")
	})

	t.Run("openapi spec not found on wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/openapi.json", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

func TestDocsHandler_OpenAPISpec_ContainsAllSections(t *testing.T) {
	h := NewDocsHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	rec := httptest.NewRecorder()

	h.OpenAPISpec(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	// Verify major sections are present
	assert.Contains(t, body, "/healthz")
	assert.Contains(t, body, "/readyz")
	assert.Contains(t, body, "/api/v1/molecules")
	assert.Contains(t, body, "/api/v1/patents")
	assert.Contains(t, body, "/api/v1/portfolios")
	assert.Contains(t, body, "/api/v1/workspaces")
	assert.Contains(t, body, "/api/v1/reports")
	assert.Contains(t, body, "BearerAuth")
	assert.Contains(t, body, "ErrorResponse")
}

//Personal.AI order the ending
