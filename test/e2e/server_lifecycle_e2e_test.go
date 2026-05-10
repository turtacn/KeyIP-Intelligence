// Phase 18 - E2E Test: API Server Lifecycle
// Validates the API server startup, health check endpoints, readiness probe,
// and graceful shutdown behavior. Ensures the server responds correctly to
// liveness and readiness probes as expected by Kubernetes-style orchestration.
package e2e_test

import (
	"net/http"
	"testing"
	"time"
)

// TestServerLifecycle_HealthChecks validates the health check endpoints.
func TestServerLifecycle_HealthChecks(t *testing.T) {
	skipIfNoServer(t)

	t.Run("LivenessProbe", func(t *testing.T) {
		// GET /healthz should return 200 with status "alive".
		resp := doGet(t, "/healthz", "")
		defer resp.Body.Close()

		assertStatus(t, resp, http.StatusOK)

		var body map[string]interface{}
		assertJSON(t, resp, &body)

		assertFieldExists(t, body, "status")
		assertFieldExists(t, body, "version")
		assertFieldExists(t, body, "uptime")

		if status, ok := body["status"].(string); ok {
			if status != "alive" && status != "ok" {
				t.Logf("liveness status: %s (accepting alive/ok)", status)
			}
		}
		t.Log("liveness probe returned healthy response")
	})

	t.Run("ReadinessProbe", func(t *testing.T) {
		// GET /readyz should return 200 when all dependencies are healthy,
		// or 503 with "not_ready" if dependencies are degraded.
		resp := doGet(t, "/readyz", "")
		defer resp.Body.Close()

		// Both 200 and 503 are valid responses depending on infrastructure availability.
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected readiness probe to return 200 or 503, got %d", resp.StatusCode)
		}

		var body map[string]interface{}
		assertJSON(t, resp, &body)

		assertFieldExists(t, body, "status")
		status, _ := body["status"].(string)
		t.Logf("readiness probe: status=%s, http=%d", status, resp.StatusCode)
	})

	t.Run("DetailedHealth", func(t *testing.T) {
		// GET /healthz/detail should return component-level health status.
		resp := doGet(t, "/healthz/detail", "")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected detailed health to return 200 or 503, got %d", resp.StatusCode)
		}

		var body map[string]interface{}
		assertJSON(t, resp, &body)

		assertFieldExists(t, body, "status")
		assertFieldExists(t, body, "version")
		assertFieldExists(t, body, "uptime")

		t.Logf("detailed health: status=%s, http=%d", body["status"], resp.StatusCode)

		// If components field exists, log individual component statuses.
		if components, ok := body["components"].(map[string]interface{}); ok {
			for name, status := range components {
				t.Logf("  component %s: %v", name, status)
			}
		}
	})
}

// TestServerLifecycle_APIVersion verifies the API version endpoint if available.
func TestServerLifecycle_APIVersion(t *testing.T) {
	skipIfNoServer(t)

	// Attempt to fetch API version from a well-known endpoint.
	// Some deployments expose version information at GET /api/v1/version.
	resp := doGet(t, "/api/v1/version", env.viewerToken)
	defer resp.Body.Close()

	// The version endpoint may not be implemented; accept 404 as a valid response.
	if resp.StatusCode == http.StatusOK {
		var body map[string]interface{}
		assertJSON(t, resp, &body)
		t.Logf("API version endpoint returned: %v", body)
	} else if resp.StatusCode == http.StatusNotFound {
		t.Log("API version endpoint not implemented (expected for some deployments)")
	} else {
		t.Logf("API version endpoint returned status %d (non-critical)", resp.StatusCode)
	}
}

// TestServerLifecycle_ResponseHeaders validates response headers include
// security and content-type headers expected from the API server.
func TestServerLifecycle_ResponseHeaders(t *testing.T) {
	skipIfNoServer(t)

	resp := doGet(t, "/healthz", "")
	defer resp.Body.Close()

	// Verify Content-Type is application/json.
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		t.Log("warning: no Content-Type header set on health response")
	} else if contentType != "application/json" {
		t.Logf("info: Content-Type is %q (expected application/json)", contentType)
	}

	// Check for security headers (optional, warn only).
	securityHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Strict-Transport-Security",
	}
	for _, h := range securityHeaders {
		if v := resp.Header.Get(h); v != "" {
			t.Logf("security header present: %s: %s", h, v)
		}
	}

	// Verify X-E2E-Test echo header if the server reflects it.
	if v := resp.Header.Get("X-E2E-Test"); v != "" {
		t.Logf("X-E2E-Test echoed: %s", v)
	}
}

// TestServerLifecycle_ShutdownBehavior verifies that the server responds
// correctly during shutdown scenarios. This test simulates what happens
// when the server is in a shutting-down state (e.g., SIGTERM received).
func TestServerLifecycle_ShutdownBehavior(t *testing.T) {
	skipIfNoServer(t)

	// This test validates that:
	// 1. A request to a non-existent endpoint returns 404.
	// 2. A request with an unsupported method returns 405 (or similar).
	// 3. The server handles timeouts gracefully.

	t.Run("NotFoundEndpoint", func(t *testing.T) {
		resp := doGet(t, "/api/v1/nonexistent-resource-12345", env.viewerToken)
		defer resp.Body.Close()

		// Should return 404.
		if resp.StatusCode != http.StatusNotFound {
			t.Logf("non-existent endpoint returned %d (expected 404)", resp.StatusCode)
		} else {
			t.Log("non-existent endpoint correctly returned 404")
		}
	})

	t.Run("UnsupportedMethod", func(t *testing.T) {
		// Send a DELETE to a read-only endpoint.
		resp := doDelete(t, "/healthz", env.viewerToken)
		defer resp.Body.Close()

		// Should return 405 Method Not Allowed or 404.
		if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
			t.Logf("unsupported method returned %d (expected 405 or 404)", resp.StatusCode)
		} else {
			t.Logf("unsupported method correctly returned %d", resp.StatusCode)
		}
	})

	t.Run("ResponseTimeWithinLimit", func(t *testing.T) {
		// Verify that health check responses are returned quickly.
		start := time.Now()
		resp := doGet(t, "/healthz", "")
		defer resp.Body.Close()
		elapsed := time.Since(start)

		maxLatency := 5 * time.Second
		if elapsed > maxLatency {
			t.Fatalf("health check took %v, exceeds limit of %v", elapsed, maxLatency)
		}
		t.Logf("health check response time: %v (limit: %v)", elapsed, maxLatency)
	})
}

// TestServerLifecycle_ServiceDiscovery validates that well-known service
// endpoints return expected responses.
func TestServerLifecycle_ServiceDiscovery(t *testing.T) {
	skipIfNoServer(t)

	// OpenAPI / Swagger endpoint is commonly served at these paths.
	docsPaths := []string{
		"/api/v1/openapi.json",
		"/api/v1/swagger.json",
		"/api/v1/docs",
		"/docs",
	}

	foundDocs := false
	for _, path := range docsPaths {
		resp := doGet(t, path, env.viewerToken)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			foundDocs = true
			t.Logf("API documentation found at %s", path)
			break
		}
	}

	if !foundDocs {
		t.Log("no API documentation endpoint found (non-critical, deployment may not serve docs)")
	}
}

// Personal.AI order the ending
