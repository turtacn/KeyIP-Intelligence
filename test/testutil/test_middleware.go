//go:build e2e

// Package testutil provides shared test utilities for unit, integration, and E2E tests.
//
// This file provides the FixtureMiddleware HTTP middleware for injecting fixture
// data into E2E test HTTP responses. It is only compiled when the "e2e" build
// tag is active, preventing accidental use in production builds.
package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// FixtureType enumerates the fixture data types that the middleware can serve.
type FixtureType string

const (
	FixtureTypePatents    FixtureType = "patents"
	FixtureTypeMolecules  FixtureType = "molecules"
	FixtureTypePortfolios FixtureType = "portfolios"
	FixtureTypeUsers      FixtureType = "users"
	FixtureTypeAll        FixtureType = "all"
)

// XTestFixtureHeader is the HTTP header that activates fixture injection.
const XTestFixtureHeader = "X-Test-Fixture"

// FixtureMiddleware is an HTTP middleware that intercepts API requests and
// returns JSON fixture data instead of calling the real handler. It activates
// only when the X-Test-Fixture request header is set.
//
// The header value selects which fixture set to serve:
//   - patents    → serve patent fixture data
//   - molecules  → serve molecule fixture data
//   - portfolios → serve portfolio fixture data
//   - users      → serve user fixture data
//   - all        → serve any fixture type based on the request path
//
// Path prefix matching maps requests to fixture types:
//   - /api/v1/patents/...    → patents
//   - /api/v1/molecules/...  → molecules
//   - /api/v1/portfolios/... → portfolios
//   - /api/v1/users/...      → users
type FixtureMiddleware struct {
	fixtures FixtureSet
}

// NewFixtureMiddleware creates a FixtureMiddleware that serves the given fixtures.
func NewFixtureMiddleware(fixtures FixtureSet) *FixtureMiddleware {
	return &FixtureMiddleware{fixtures: fixtures}
}

// Handler returns an http.Handler middleware. It checks the X-Test-Fixture
// header and, if present and recognized, intercepts matching API paths to
// return fixture data in the appropriate response format.
func (m *FixtureMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fixtureHeader := r.Header.Get(XTestFixtureHeader)
		if fixtureHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		ft := FixtureType(strings.ToLower(strings.TrimSpace(fixtureHeader)))
		path := r.URL.Path

		// Determine which fixture data to serve based on the header + path.
		data, err := m.selectFixtureData(ft, path)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Fixture-Mode", string(ft))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
	})
}

// selectFixtureData chooses the appropriate fixture response based on the
// requested fixture type and the request path. Returns the data to serialize,
// or an error if the fixture type is unknown.
func (m *FixtureMiddleware) selectFixtureData(ft FixtureType, path string) (any, error) {
	switch ft {
	case FixtureTypePatents, FixtureTypeMolecules, FixtureTypePortfolios, FixtureTypeUsers:
		return m.dataForFixtureType(ft, path)
	case FixtureTypeAll:
		return m.dataForAll(path)
	default:
		return nil, fmt.Errorf("unknown X-Test-Fixture value: %q; valid: patents, molecules, portfolios, users, all", string(ft))
	}
}

// dataForFixtureType returns fixture data for a specific type, checking the
// path to determine the response format.
func (m *FixtureMiddleware) dataForFixtureType(ft FixtureType, path string) (any, error) {
	switch ft {
	case FixtureTypePatents:
		if isSingleItemPath(path, "/api/v1/patents/") {
			return firstOrEmpty(m.fixtures.Patents), nil
		}
		return wrapListResult("patents", m.fixtures.Patents, len(m.fixtures.Patents)), nil

	case FixtureTypeMolecules:
		if isSingleItemPath(path, "/api/v1/molecules/") {
			return firstOrEmpty(m.fixtures.Molecules), nil
		}
		return wrapListResult("molecules", m.fixtures.Molecules, len(m.fixtures.Molecules)), nil

	case FixtureTypePortfolios:
		if isSingleItemPath(path, "/api/v1/portfolios/") {
			return firstOrEmpty(m.fixtures.Portfolios), nil
		}
		return wrapListResult("portfolios", m.fixtures.Portfolios, len(m.fixtures.Portfolios)), nil

	case FixtureTypeUsers:
		return wrapListResult("users", m.fixtures.Users, len(m.fixtures.Users)), nil

	default:
		return nil, fmt.Errorf("unhandled fixture type: %s", ft)
	}
}

// dataForAll returns fixture data for the "all" mode, selecting by path prefix.
func (m *FixtureMiddleware) dataForAll(path string) (any, error) {
	switch {
	case strings.HasPrefix(path, "/api/v1/patents/"):
		return m.dataForFixtureType(FixtureTypePatents, path)
	case strings.HasPrefix(path, "/api/v1/molecules/"):
		return m.dataForFixtureType(FixtureTypeMolecules, path)
	case strings.HasPrefix(path, "/api/v1/portfolios/"):
		return m.dataForFixtureType(FixtureTypePortfolios, path)
	case strings.HasPrefix(path, "/api/v1/users/"):
		return m.dataForFixtureType(FixtureTypeUsers, path)
	default:
		// For unrecognized paths in "all" mode, return the full FixtureSet.
		return m.fixtures, nil
	}
}

// isSingleItemPath returns true if the path appears to target a single resource
// (e.g., /api/v1/patents/{id}) rather than a collection or search endpoint.
// It checks that the path after the prefix is non-empty and is not a known
// collection-level action.
func isSingleItemPath(path, prefix string) bool {
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" {
		return false
	}
	// A slash after the ID or a known sub-path indicates a sub-resource,
	// not a single-item path. Single-item: "/api/v1/patents/some-uuid"
	// Sub-resource: "/api/v1/patents/some-uuid/family"
	// Collection action: "/api/v1/patents/search"
	if strings.Contains(rest, "/") {
		return false
	}
	return true
}

// firstOrEmpty returns the first element of a slice, or nil if empty.
func firstOrEmpty(items any) any {
	switch v := items.(type) {
	case []PatentFixture:
		if len(v) == 0 {
			return nil
		}
		return v[0]
	case []MoleculeFixture:
		if len(v) == 0 {
			return nil
		}
		return v[0]
	case []PortfolioFixture:
		if len(v) == 0 {
			return nil
		}
		return v[0]
	case []UserFixture:
		if len(v) == 0 {
			return nil
		}
		return v[0]
	default:
		return nil
	}
}

// wrapListResult wraps a list of items in a paginated response envelope,
// mimicking the real API list/search response format.
func wrapListResult(key string, items any, total int) map[string]any {
	return map[string]any{
		key:          items,
		"total":      total,
		"page":       1,
		"page_size":  total,
		"total_pages": 1,
	}
}
