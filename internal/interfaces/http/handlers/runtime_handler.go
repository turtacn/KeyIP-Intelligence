// Phase 11 - File 283: internal/interfaces/http/handlers/runtime_handler.go
// 实现运行时信息 API 端点。
//
// 核心实现:
//   - 定义 RuntimeHandler 结构体，注入构建版本信息和启动时间
//   - GET /api/v1/runtime/info  → 返回 Go 运行时状态（版本、OS/ARCH、启动时间、goroutine 数、内存使用）
//   - GET /api/v1/runtime/build → 返回构建元数据（版本、提交 SHA、构建时间、Go 版本）
//   - 实现 RegisterRoutes: 注册运行时信息路由
//
// 依赖关系:
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package handlers

import (
	"net/http"
	"runtime"
	"time"
)

// RuntimeHandler handles runtime and build information endpoints.
type RuntimeHandler struct {
	version   string
	commitSHA string
	buildTime string
	goVersion string
	startAt   time.Time
}

// NewRuntimeHandler creates a new RuntimeHandler with the given build info.
func NewRuntimeHandler(version, commitSHA, buildTime, goVersion string) *RuntimeHandler {
	return &RuntimeHandler{
		version:   version,
		commitSHA: commitSHA,
		buildTime: buildTime,
		goVersion: goVersion,
		startAt:   time.Now(),
	}
}

// RegisterRoutes registers runtime endpoints on the given mux.
func (h *RuntimeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/runtime/info", h.GetInfo)
	mux.HandleFunc("GET /api/v1/runtime/build", h.GetBuild)
}

// --- Runtime Info ---

// RuntimeInfoResponse is the JSON response for the runtime info endpoint.
type RuntimeInfoResponse struct {
	Version       string `json:"version"`
	GoVersion     string `json:"go_version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	Uptime        string `json:"uptime"`
	Goroutines    int    `json:"goroutines"`
	MemoryAlloc   uint64 `json:"memory_alloc_bytes"`
	MemoryTotal   uint64 `json:"memory_total_bytes"`
	MemorySys     uint64 `json:"memory_sys_bytes"`
	NumGC         uint32 `json:"num_gc"`
}

// GetInfo handles GET /api/v1/runtime/info.
// Returns live runtime information including Go version, OS/ARCH, uptime,
// number of goroutines, and memory statistics.
func (h *RuntimeHandler) GetInfo(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	goVersion := h.goVersion
	if goVersion == "" {
		goVersion = runtime.Version()
	}

	resp := RuntimeInfoResponse{
		Version:       h.version,
		GoVersion:     goVersion,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Uptime:        time.Since(h.startAt).Truncate(time.Second).String(),
		Goroutines:    runtime.NumGoroutine(),
		MemoryAlloc:   mem.Alloc,
		MemoryTotal:   mem.TotalAlloc,
		MemorySys:     mem.Sys,
		NumGC:         mem.NumGC,
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- Build Info ---

// BuildInfoResponse is the JSON response for the build info endpoint.
type BuildInfoResponse struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commit_sha"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

// GetBuild handles GET /api/v1/runtime/build.
// Returns build-time metadata as JSON (values injected via ldflags).
func (h *RuntimeHandler) GetBuild(w http.ResponseWriter, r *http.Request) {
	goVersion := h.goVersion
	if goVersion == "" {
		goVersion = runtime.Version()
	}

	resp := BuildInfoResponse{
		Version:   h.version,
		CommitSHA: h.commitSHA,
		BuildTime: h.buildTime,
		GoVersion: goVersion,
	}
	writeJSON(w, http.StatusOK, resp)
}

//Personal.AI order the ending
