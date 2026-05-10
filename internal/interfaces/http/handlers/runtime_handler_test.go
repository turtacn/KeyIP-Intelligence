// Phase 11 - File 284: internal/interfaces/http/handlers/runtime_handler_test.go
// 测试运行时信息 API 端点。
//
// 测试目标:
//   - GET /api/v1/runtime/info 返回 200，JSON 包含运行时字段
//   - GET /api/v1/runtime/build 返回 200，JSON 包含构建字段
//   - POST 方法返回 405
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

func TestRuntimeHandler_GetInfo(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		goVersion string
	}{
		{
			name:      "returns runtime info with version",
			version:   "1.2.3",
			goVersion: "go1.22.1",
		},
		{
			name:      "handles dev defaults",
			version:   "dev",
			goVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewRuntimeHandler(tt.version, "abc123", "2024-01-15T10:00:00Z", tt.goVersion)

			mux := http.NewServeMux()
			handler.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/info", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var resp RuntimeInfoResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.NoError(t, err, "response body must be valid JSON")
			assert.Equal(t, tt.version, resp.Version)
			if tt.goVersion != "" {
				assert.Equal(t, tt.goVersion, resp.GoVersion)
			} else {
				assert.NotEmpty(t, resp.GoVersion, "Go version should fallback to runtime.Version()")
			}
			assert.NotEmpty(t, resp.OS)
			assert.NotEmpty(t, resp.Arch)
			assert.NotEmpty(t, resp.Uptime)
			assert.Greater(t, resp.Goroutines, 0)
			assert.Greater(t, resp.MemoryAlloc, uint64(0))
		})
	}
}

func TestRuntimeHandler_GetInfo_HasExpectedFields(t *testing.T) {
	handler := NewRuntimeHandler("2.0.0", "def456", "now", "go1.22.0")

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/info", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var raw map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &raw)
	assert.NoError(t, err)

	assert.Contains(t, raw, "version")
	assert.Contains(t, raw, "go_version")
	assert.Contains(t, raw, "os")
	assert.Contains(t, raw, "arch")
	assert.Contains(t, raw, "uptime")
	assert.Contains(t, raw, "goroutines")
	assert.Contains(t, raw, "memory_alloc_bytes")
	assert.Contains(t, raw, "memory_total_bytes")
	assert.Contains(t, raw, "memory_sys_bytes")
	assert.Contains(t, raw, "num_gc")
}

func TestRuntimeHandler_GetBuild(t *testing.T) {
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
			goVersion: "",
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
			handler := NewRuntimeHandler(tt.version, tt.commitSHA, tt.buildTime, tt.goVersion)

			mux := http.NewServeMux()
			handler.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/build", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

			var resp BuildInfoResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			assert.NoError(t, err, "response body must be valid JSON")
			assert.Equal(t, tt.version, resp.Version)
			assert.Equal(t, tt.commitSHA, resp.CommitSHA)
			assert.Equal(t, tt.buildTime, resp.BuildTime)
			if tt.goVersion != "" {
				assert.Equal(t, tt.goVersion, resp.GoVersion)
			} else {
				assert.NotEmpty(t, resp.GoVersion, "Go version should fallback to runtime.Version()")
			}
		})
	}
}

func TestRuntimeHandler_RegisterRoutes(t *testing.T) {
	handler := NewRuntimeHandler("1.0.0", "abc", "now", "go1.22")

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Test registered GET /api/v1/runtime/info
	req := httptest.NewRequest(http.MethodGet, "/api/v1/runtime/info", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// POST /api/v1/runtime/info should 405
	req = httptest.NewRequest(http.MethodPost, "/api/v1/runtime/info", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	// Test registered GET /api/v1/runtime/build
	req = httptest.NewRequest(http.MethodGet, "/api/v1/runtime/build", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// POST /api/v1/runtime/build should 405
	req = httptest.NewRequest(http.MethodPost, "/api/v1/runtime/build", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

//Personal.AI order the ending
