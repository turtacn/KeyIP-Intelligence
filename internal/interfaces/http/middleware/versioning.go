// Phase 11 - 接口层: HTTP Middleware - API 版本管理中间件
// 序号: 282
// 文件: internal/interfaces/http/middleware/versioning.go
// 功能定位: 实现 API 版本管理中间件，包括版本声明、版本协商、弃用警告
// 核心实现:
//   - 定义 VersioningConfig 结构体: CurrentVersion, SupportedVersions, DeprecatedVersions
//   - 定义 DefaultVersioningConfig() 从构建 ldflags 读取版本信息
//   - 实现 VersioningMiddleware 结构体:
//     - 所有响应添加 X-API-Version 头
//     - 检查 Accept-Version 请求头进行版本协商，不支持的版本返回 406
//     - 对已弃用版本添加 Deprecation 和 Sunset 响应头
//   - 版本集合使用 map O(1) 查找
//
// 依赖关系:
//   - 依赖: internal/config
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

// VersioningConfig holds configuration for the API versioning middleware.
type VersioningConfig struct {
	// CurrentVersion is the default API version returned in the X-API-Version response header.
	CurrentVersion string

	// SupportedVersions lists all API versions accepted via Accept-Version header negotiation.
	SupportedVersions []string

	// DeprecatedVersions maps deprecated version strings to their sunset dates (RFC1123 format).
	// When a deprecated version is requested via Accept-Version, Deprecation and Sunset headers
	// are added to the response to warn the client.
	DeprecatedVersions map[string]string
}

// DefaultVersioningConfig returns a VersioningConfig populated from build-time ldflags.
// CurrentVersion defaults to config.Version ("dev" when not set via ldflags).
// SupportedVersions defaults to ["1.0"].
func DefaultVersioningConfig() VersioningConfig {
	return VersioningConfig{
		CurrentVersion:     config.Version,
		SupportedVersions:  []string{"1.0"},
		DeprecatedVersions: make(map[string]string),
	}
}

// VersioningMiddleware provides API versioning via response headers and request negotiation.
type VersioningMiddleware struct {
	config VersioningConfig
}

// NewVersioningMiddleware creates a new VersioningMiddleware with the given configuration.
func NewVersioningMiddleware(config VersioningConfig) *VersioningMiddleware {
	return &VersioningMiddleware{config: config}
}

// Handler returns the versioning middleware handler function.
// It performs three functions:
//  1. Sets the X-API-Version response header on every response.
//  2. For /api/* routes, checks the Accept-Version request header and rejects
//     unsupported versions with HTTP 406 Not Acceptable.
//  3. For /api/* routes using a deprecated version, adds Deprecation and Sunset
//     response headers to warn the client.
func (m *VersioningMiddleware) Handler(next http.Handler) http.Handler {
	// Pre-compute supported versions set for O(1) lookup
	supported := make(map[string]bool, len(m.config.SupportedVersions))
	for _, v := range m.config.SupportedVersions {
		supported[v] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Always set X-API-Version on every response
		w.Header().Set("X-API-Version", m.config.CurrentVersion)

		// 2. Version negotiation via Accept-Version header (API routes only)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			if requested := r.Header.Get("Accept-Version"); requested != "" {
				if !supported[requested] {
					writeVersionNegotiationError(w, http.StatusNotAcceptable,
						"requested API version not supported: "+requested)
					return
				}

				// Echo the negotiated version
				w.Header().Set("X-API-Version", requested)

				// 3. Deprecation warning headers for old versions
				if sunset, ok := m.config.DeprecatedVersions[requested]; ok {
					w.Header().Set("Deprecation", "true")
					w.Header().Set("Sunset", sunset)
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// writeVersionNegotiationError writes a JSON error response for version negotiation failures.
func writeVersionNegotiationError(w http.ResponseWriter, statusCode int, message string) {
	resp := struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{}
	resp.Error.Code = http.StatusText(statusCode)
	resp.Error.Message = message

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}

//Personal.AI order the ending
