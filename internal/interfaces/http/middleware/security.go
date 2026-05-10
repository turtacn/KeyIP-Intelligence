// Phase 11 - 接口层: HTTP Middleware - 安全响应头中间件
// 序号: 275
// 文件: internal/interfaces/http/middleware/security.go
// 功能定位: 实现安全 HTTP 响应头中间件，为所有响应添加安全相关的 HTTP 头
// 核心实现:
//   - 定义 SecurityConfig 结构体: CSP策略、HSTS、各安全头的开关和配置
//   - 定义 DefaultSecurityConfig() SecurityConfig，返回安全的默认配置
//   - 实现 SecurityHeaders(config SecurityConfig) func(http.Handler) http.Handler
//   - 添加 Content-Security-Policy 头，限制脚本和样式来源
//   - 添加 X-Content-Type-Options: nosniff
//   - 添加 X-Frame-Options: DENY
//   - 添加 X-XSS-Protection: 1; mode=block
//   - 添加 Strict-Transport-Security: max-age=31536000; includeSubDomains
//   - 添加 Referrer-Policy: strict-origin-when-cross-origin
//   - 添加 Permissions-Policy: 限制 camera, microphone, geolocation
//   - 可配置是否启用 HSTS（仅 HTTPS 场景）
//
// 依赖关系:
//   - 被依赖: internal/interfaces/http/router.go
//
// 强制约束: 文件最后一行必须为 //Personal.AI order the ending
package middleware

import (
	"net/http"
	"strings"
)

// SecurityConfig holds configuration for security response headers.
type SecurityConfig struct {
	// ContentSecurityPolicy defines the Content-Security-Policy header value.
	// If empty, the default restrictive policy is used.
	ContentSecurityPolicy string

	// ContentSecurityPolicyReportOnly defines the Content-Security-Policy-Report-Only
	// header value. When set, browsers report CSP violations without blocking resources.
	// If empty, report-only mode is disabled.
	ContentSecurityPolicyReportOnly string

	// CSPReportURI defines the report-uri directive for CSP violation reporting.
	// When non-empty, it is appended to both the Content-Security-Policy and
	// Content-Security-Policy-Report-Only headers.
	// Default: "/api/v1/csp-report"
	CSPReportURI string

	// CSPReportTo defines the report-to directive for CSP violation reporting.
	// This is part of the Reporting API and replaces report-uri in modern browsers.
	// When non-empty, it is appended to both CSP headers.
	// Requires a matching Report-To header or endpoint configuration on the server.
	CSPReportTo string

	// XContentTypeOptions defines the X-Content-Type-Options header value.
	// Default: "nosniff"
	XContentTypeOptions string

	// XFrameOptions defines the X-Frame-Options header value.
	// Default: "DENY"
	XFrameOptions string

	// XXSSProtection defines the X-XSS-Protection header value.
	// Default: "1; mode=block"
	XXSSProtection string

	// StrictTransportSecurity defines the Strict-Transport-Security header value.
	// Only meaningful when served over HTTPS. Default: "max-age=31536000; includeSubDomains"
	// Set to empty string to disable HSTS.
	StrictTransportSecurity string

	// ReferrerPolicy defines the Referrer-Policy header value.
	// Default: "strict-origin-when-cross-origin"
	ReferrerPolicy string

	// PermissionsPolicy defines the Permissions-Policy header value.
	// Default restricts camera, microphone, geolocation.
	PermissionsPolicy string
}

// DefaultCSP returns a restrictive Content-Security-Policy string that allows
// the application's own resources and common CDNs.
func DefaultCSP() string {
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://unpkg.com",
		"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://fonts.googleapis.com",
		"img-src 'self' data: blob:",
		"font-src 'self' https://fonts.gstatic.com",
		"connect-src 'self'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"base-uri 'self'",
	}, "; ")
}

// DefaultCSPReportOnly returns the default Content-Security-Policy-Report-Only string.
// By default it uses the same policy as DefaultCSP so that violations are reported
// for the same set of rules currently being enforced. Users typically customize
// this to test a stricter policy before switching to enforcement mode.
func DefaultCSPReportOnly() string {
	return DefaultCSP()
}

// DefaultPermissionsPolicy returns the default Permissions-Policy string
// that restricts camera, microphone, and geolocation access.
func DefaultPermissionsPolicy() string {
	return strings.Join([]string{
		"camera=()",
		"microphone=()",
		"geolocation=()",
	}, ", ")
}

// DefaultSecurityConfig returns a secure default security headers configuration.
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		ContentSecurityPolicy:     DefaultCSP(),
		XContentTypeOptions:       "nosniff",
		XFrameOptions:             "DENY",
		XXSSProtection:            "1; mode=block",
		StrictTransportSecurity:   "max-age=31536000; includeSubDomains",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		PermissionsPolicy:         DefaultPermissionsPolicy(),
		CSPReportURI:              "/api/v1/csp-report",
	}
}

// buildCSPWithReports appends report-uri and/or report-to directives to a base
// CSP policy string. If both reportURI and reportTo are empty, the base string
// is returned unmodified. If baseCSP is empty, an empty string is returned so
// the header is not set.
func buildCSPWithReports(baseCSP, reportURI, reportTo string) string {
	if baseCSP == "" {
		return ""
	}
	if reportURI == "" && reportTo == "" {
		return baseCSP
	}
	parts := []string{baseCSP}
	if reportURI != "" {
		parts = append(parts, "report-uri "+reportURI)
	}
	if reportTo != "" {
		parts = append(parts, "report-to "+reportTo)
	}
	return strings.Join(parts, "; ")
}

// SecurityHeaders returns middleware that adds security-related HTTP response headers.
// It applies headers in a deterministic order and always applies them before
// delegating to the next handler.
func SecurityHeaders(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Content-Security-Policy (enforcement mode)
			if config.ContentSecurityPolicy != "" {
				csp := buildCSPWithReports(config.ContentSecurityPolicy, config.CSPReportURI, config.CSPReportTo)
				w.Header().Set("Content-Security-Policy", csp)
			}

			// Content-Security-Policy-Report-Only (report-only mode)
			if config.ContentSecurityPolicyReportOnly != "" {
				cspRO := buildCSPWithReports(config.ContentSecurityPolicyReportOnly, config.CSPReportURI, config.CSPReportTo)
				w.Header().Set("Content-Security-Policy-Report-Only", cspRO)
			}

			// X-Content-Type-Options
			if config.XContentTypeOptions != "" {
				w.Header().Set("X-Content-Type-Options", config.XContentTypeOptions)
			}

			// X-Frame-Options
			if config.XFrameOptions != "" {
				w.Header().Set("X-Frame-Options", config.XFrameOptions)
			}

			// X-XSS-Protection
			if config.XXSSProtection != "" {
				w.Header().Set("X-XSS-Protection", config.XXSSProtection)
			}

			// Strict-Transport-Security (only meaningful over HTTPS; applied here
			// so the header is present when the request arrives over TLS)
			if config.StrictTransportSecurity != "" {
				w.Header().Set("Strict-Transport-Security", config.StrictTransportSecurity)
			}

			// Referrer-Policy
			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}

			// Permissions-Policy
			if config.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeadersMiddleware wraps the security headers middleware for use with
// router configuration.
type SecurityHeadersMiddleware struct {
	handler func(http.Handler) http.Handler
}

// NewSecurityHeadersMiddleware creates a new SecurityHeadersMiddleware.
// If config is nil, DefaultSecurityConfig is used.
func NewSecurityHeadersMiddleware(config *SecurityConfig) *SecurityHeadersMiddleware {
	cfg := DefaultSecurityConfig()
	if config != nil {
		// Merge: copy non-zero fields from provided config into defaults.
		if config.ContentSecurityPolicy != "" {
			cfg.ContentSecurityPolicy = config.ContentSecurityPolicy
		}
		if config.ContentSecurityPolicyReportOnly != "" {
			cfg.ContentSecurityPolicyReportOnly = config.ContentSecurityPolicyReportOnly
		}
		if config.CSPReportURI != "" {
			cfg.CSPReportURI = config.CSPReportURI
		}
		if config.CSPReportTo != "" {
			cfg.CSPReportTo = config.CSPReportTo
		}
		if config.XContentTypeOptions != "" {
			cfg.XContentTypeOptions = config.XContentTypeOptions
		}
		if config.XFrameOptions != "" {
			cfg.XFrameOptions = config.XFrameOptions
		}
		if config.XXSSProtection != "" {
			cfg.XXSSProtection = config.XXSSProtection
		}
		if config.StrictTransportSecurity != "" {
			cfg.StrictTransportSecurity = config.StrictTransportSecurity
		}
		if config.ReferrerPolicy != "" {
			cfg.ReferrerPolicy = config.ReferrerPolicy
		}
		if config.PermissionsPolicy != "" {
			cfg.PermissionsPolicy = config.PermissionsPolicy
		}
	}
	return &SecurityHeadersMiddleware{
		handler: SecurityHeaders(cfg),
	}
}

// Handler returns the middleware handler function.
func (m *SecurityHeadersMiddleware) Handler(next http.Handler) http.Handler {
	return m.handler(next)
}

//Personal.AI order the ending
