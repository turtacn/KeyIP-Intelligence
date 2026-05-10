// Phase 11 - 接口层: HTTP Handler - API 版本信息端点
// 序号: 261
// 文件: internal/interfaces/http/handlers/version_handler.go
// 功能定位: 提供 API 版本信息端点 GET /api/version，返回构建时版本元数据
// 核心实现:
//   - 定义 VersionHandler 结构体，注入构建版本信息
//   - 实现 GetVersion: 返回 JSON 格式的版本、提交 SHA、构建时间、Go 版本
//   - 实现 RegisterRoutes: 注册 GET /api/version 路由
//
// 依赖关系:
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package handlers

import (
	"net/http"
)

// VersionHandler handles the GET /api/version endpoint.
// It returns build-time version metadata for the running server.
type VersionHandler struct {
	version   string
	commitSHA string
	buildTime string
	goVersion string
}

// NewVersionHandler creates a new VersionHandler with the given build info.
func NewVersionHandler(version, commitSHA, buildTime, goVersion string) *VersionHandler {
	return &VersionHandler{
		version:   version,
		commitSHA: commitSHA,
		buildTime: buildTime,
		goVersion: goVersion,
	}
}

// RegisterRoutes registers the version endpoint on the given mux.
func (h *VersionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/version", h.GetVersion)
}

// VersionResponse is the JSON response body for the version endpoint.
type VersionResponse struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// GetVersion handles GET /api/version.
// Returns build-time version metadata as JSON.
func (h *VersionHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	resp := VersionResponse{
		Version:   h.version,
		CommitSHA: h.commitSHA,
		BuildTime: h.buildTime,
		GoVersion: h.goVersion,
	}
	writeJSON(w, http.StatusOK, resp)
}

//Personal.AI order the ending
