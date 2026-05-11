//go:build e2e

package testutil

import (
	"net/http"
	"net/http/httptest"
)

// TestServer wraps an httptest.Server pre-configured with the fixture
// middleware and all standard API routes registered. It is only compiled
// when the "e2e" build tag is active.
type TestServer struct {
	Server   *httptest.Server
	Fixtures FixtureSet
	Mux      *http.ServeMux
}

// TestServerConfig configures the test server.
type TestServerConfig struct {
	// Fixtures to serve via the fixture middleware.
	Fixtures FixtureSet

	// Optional handler registrations for paths not covered by fixtures.
	// If nil, only the fixture middleware is installed on the default mux.
	RegisterRoutes func(mux *http.ServeMux)

	// Optional extra middleware applied after the fixture middleware.
	ExtraMiddlewares []func(http.Handler) http.Handler
}

// NewTestServer creates an httptest.Server with the fixture middleware and
// optional real handler routes registered. Usage:
//
//	 fixtures := testutil.MustLoadAll("test/testdata/fixtures")
//	 ts := testutil.NewTestServer(testutil.TestServerConfig{
//	      Fixtures: fixtures,
//	      RegisterRoutes: func(mux *http.ServeMux) {
//	          // Register real handler routes if needed
//	      },
//	 })
//	 defer ts.Server.Close()
//
// The fixture middleware is installed OUTERMOST, so it intercepts requests
// before any real handler runs when X-Test-Fixture is set.
func NewTestServer(cfg TestServerConfig) *TestServer {
	mux := http.NewServeMux()

	// Register real API routes if provided.
	if cfg.RegisterRoutes != nil {
		cfg.RegisterRoutes(mux)
	}

	// Build middleware chain: extra middlewares -> fixture middleware -> mux
	var handler http.Handler = mux

	fixtureMW := NewFixtureMiddleware(cfg.Fixtures)
	handler = fixtureMW.Handler(handler)

	// Apply extra middlewares from outermost to innermost.
	for i := len(cfg.ExtraMiddlewares) - 1; i >= 0; i-- {
		handler = cfg.ExtraMiddlewares[i](handler)
	}

	server := httptest.NewServer(handler)

	return &TestServer{
		Server:   server,
		Fixtures: cfg.Fixtures,
		Mux:      mux,
	}
}

// URL returns the base URL of the test server (e.g., "http://127.0.0.1:PORT").
func (ts *TestServer) URL() string {
	return ts.Server.URL
}

// Close shuts down the test server.
func (ts *TestServer) Close() {
	ts.Server.Close()
}
