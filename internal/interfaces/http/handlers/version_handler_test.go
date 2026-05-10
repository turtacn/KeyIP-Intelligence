// Phase 11 - 接口层: HTTP Handler - API 版本信息端点测试
// 序号: 262
// 文件: internal/interfaces/http/handlers/version_handler_test.go
// 测试目标:
//   - GET /api/version 返回 200
//   - JSON 响应体包含 version, commit_sha, build_time, go_version 字段
//   - 自定义字段值通过 JSON 正确返回
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionHandler_GetVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		commitSHA string
		buildTime string
		goVersion string
	}{
		{
			name:      "returns build info",
			version:   "1.2.3",
			commitSHA: "abc123def456",
			buildTime: "2024-01-15T10:00:00Z",
			goVersion: "go1.22.1",
		},
		{
			name:      "handles dev defaults",
			version:   "dev",
			commitSHA: "unknown",
			buildTime: "unknown",
			goVersion: "unknown",
		},
		{
			name:      "handles empty values",
			version:   "",
			commitSHA: "",
			buildTime: "",
			goVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewVersionHandler(tt.version, tt.commitSHA, tt.buildTime, tt.goVersion)

			mux := http.NewServeMux()
			handler.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var resp VersionResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.NoError(t, err, "response body must be valid JSON")
			assert.Equal(t, tt.version, resp.Version)
			assert.Equal(t, tt.commitSHA, resp.CommitSHA)
			assert.Equal(t, tt.buildTime, resp.BuildTime)
			assert.Equal(t, tt.goVersion, resp.GoVersion)
		})
	}
}

func TestVersionHandler_RegisterRoutes(t *testing.T) {
	handler := NewVersionHandler("1.0.0", "abc", "now", "go1.22")

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test registered route
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// POST should 405
	req = httptest.NewRequest(http.MethodPost, "/api/version", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

//Personal.AI order the ending
