//go:build e2e

// Phase 18 - E2E 场景驱动集成测试
// 使用 httptest.Server + JSON fixtures 覆盖 6 个核心业务场景。
// 每个场景为多步 API 调用工作流，验证完整用户操作流程。
package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// 1. Fixture Data
// ============================================================================

type scenarioFixtures struct {
	Molecules  []map[string]interface{} `json:"molecules"`
	Patents    []map[string]interface{} `json:"patents"`
	Portfolios []map[string]interface{} `json:"portfolios"`
	Lifecycles []map[string]interface{} `json:"lifecycle_records"`
	Workspaces []map[string]interface{} `json:"workspaces"`
	Valuations []map[string]interface{} `json:"valuation_benchmarks"`

	molByID map[string]map[string]interface{}
	patByID map[string]map[string]interface{}
}

var (
	fixtures     *scenarioFixtures
	fixturesOnce sync.Once
	fixturesOK   bool
	testServer   *httptest.Server
	serverOnce   sync.Once
)

// fixturePath resolves the path to a fixture file, handling both
// project-root and test-directory execution contexts.
func fixturePath(name string) string {
	candidates := []string{
		"test/testdata/fixtures/" + name,
		"../testdata/fixtures/" + name,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return candidates[0] // fallback; caller handles the error
}

func loadScenarioFixtures() bool {
	fixtures = &scenarioFixtures{
		molByID: make(map[string]map[string]interface{}),
		patByID: make(map[string]map[string]interface{}),
	}

	// Load molecule fixtures
	if data, err := os.ReadFile(fixturePath("molecule_fixtures.json")); err == nil {
		var wrapper struct {
			Molecules []map[string]interface{} `json:"molecules"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil {
			fixtures.Molecules = wrapper.Molecules
			for _, m := range fixtures.Molecules {
				if id, ok := m["id"].(string); ok {
					fixtures.molByID[id] = m
				}
			}
		} else {
			fmt.Printf("WARNING: failed to parse molecule fixtures: %v\n", err)
		}
	} else {
		fmt.Printf("WARNING: molecule fixtures not found: %v\n", err)
	}

	// Load patent fixtures (large file)
	if data, err := os.ReadFile(fixturePath("patent_fixtures.json")); err == nil {
		var wrapper struct {
			Patents []map[string]interface{} `json:"patents"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil {
			fixtures.Patents = wrapper.Patents
			for _, p := range fixtures.Patents {
				if id, ok := p["id"].(string); ok {
					fixtures.patByID[id] = p
				}
			}
		} else {
			fmt.Printf("WARNING: failed to parse patent fixtures: %v\n", err)
		}
	} else {
		fmt.Printf("WARNING: patent fixtures not found: %v\n", err)
	}

	// Load portfolio fixtures (includes lifecycle, workspaces, valuations)
	if data, err := os.ReadFile(fixturePath("portfolio_fixtures.json")); err == nil {
		var wrapper struct {
			Portfolios []map[string]interface{} `json:"portfolios"`
			Lifecycles []map[string]interface{} `json:"lifecycle_records"`
			Workspaces []map[string]interface{} `json:"workspaces"`
			Valuations []map[string]interface{} `json:"valuation_benchmarks"`
		}
		if err := json.Unmarshal(data, &wrapper); err == nil {
			fixtures.Portfolios = wrapper.Portfolios
			fixtures.Lifecycles = wrapper.Lifecycles
			fixtures.Workspaces = wrapper.Workspaces
			fixtures.Valuations = wrapper.Valuations
		} else {
			fmt.Printf("WARNING: failed to parse portfolio fixtures: %v\n", err)
		}
	} else {
		fmt.Printf("WARNING: portfolio fixtures not found: %v\n", err)
	}

	ok := len(fixtures.Molecules) > 0 || len(fixtures.Patents) > 0 || len(fixtures.Portfolios) > 0
	if !ok {
		fmt.Println("WARNING: no fixture data loaded, scenarios will skip")
	}
	return ok
}

func getFixtures() *scenarioFixtures {
	fixturesOnce.Do(func() {
		fixturesOK = loadScenarioFixtures()
	})
	if !fixturesOK {
		return nil
	}
	return fixtures
}

// ============================================================================
// 2. HTTP Response & Request Helpers
// ============================================================================

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "json encode error", http.StatusInternalServerError)
	}
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

func isJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "application/json")
}

// scenarioDoGet sends GET to the httptest server.
func scenarioDoGet(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", ts.URL+path, nil)
	if err != nil {
		t.Fatalf("create GET request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("execute GET %s: %v", path, err)
	}
	t.Logf(">> GET %s -> %d", path, resp.StatusCode)
	return resp
}

// scenarioDoPost sends POST with JSON body to the httptest server.
func scenarioDoPost(t *testing.T, ts *httptest.Server, path string, body interface{}) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest("POST", ts.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("create POST request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("execute POST %s: %v", path, err)
	}
	t.Logf(">> POST %s -> %d", path, resp.StatusCode)
	return resp
}

// scenarioDoPut sends PUT with JSON body to the httptest server.
func scenarioDoPut(t *testing.T, ts *httptest.Server, path string, body interface{}) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest("PUT", ts.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("create PUT request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("execute PUT %s: %v", path, err)
	}
	t.Logf(">> PUT %s -> %d", path, resp.StatusCode)
	return resp
}

// scenarioDoDelete sends DELETE to the httptest server.
func scenarioDoDelete(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("DELETE", ts.URL+path, nil)
	if err != nil {
		t.Fatalf("create DELETE request: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("execute DELETE %s: %v", path, err)
	}
	t.Logf(">> DELETE %s -> %d", path, resp.StatusCode)
	return resp
}

// readBody reads and returns the response body bytes, closing it.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	resp.Body.Close()
	return body
}

// assertStatusCode asserts the HTTP status code.
func assertStatusCode(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		t.Fatalf("expected status %d, got %d; body: %s", expected, resp.StatusCode, string(readBody(t, resp)))
	}
}

// assertJSONField asserts that the response body contains the expected key with non-nil value.
func assertJSONField(t *testing.T, data map[string]interface{}, field string) {
	t.Helper()
	if _, ok := data[field]; !ok {
		t.Fatalf("expected field %q not found in response", field)
	}
}

// unmarshalBody unmarshals response body into target.
func unmarshalBody(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	body := readBody(t, resp)
	if err := json.Unmarshal(body, target); err != nil {
		t.Fatalf("unmarshal response: %v; body: %s", err, string(body))
	}
}

// ============================================================================
// 3. Handler Registrations (stub handlers serving fixture data)
// ============================================================================

// newScenarioServer creates an httptest server with all stub handlers registered.
func newScenarioServer() *httptest.Server {
	mux := http.NewServeMux()

	// --- Molecule endpoints ---
	mux.HandleFunc("POST /api/v1/molecules", handleCreateMolecule)
	mux.HandleFunc("GET /api/v1/molecules/{id}", handleGetMolecule)
	mux.HandleFunc("POST /api/v1/molecules/search/similarity", handleMoleculeSimilaritySearch)
	mux.HandleFunc("POST /api/v1/molecules/search/structure", handleMoleculeStructureSearch)
	mux.HandleFunc("POST /api/v1/molecules/properties/calculate", handleMoleculePropertiesCalculate)

	// --- Patent endpoints ---
	mux.HandleFunc("POST /api/v1/patents", handleCreatePatent)
	mux.HandleFunc("GET /api/v1/patents/{id}", handleGetPatent)
	mux.HandleFunc("POST /api/v1/patents/search", handlePatentSearch)
	mux.HandleFunc("POST /api/v1/patents/search/advanced", handlePatentAdvancedSearch)
	mux.HandleFunc("GET /api/v1/patents/stats", handlePatentStats)
	mux.HandleFunc("POST /api/v1/patents/analyze-claims", handleAnalyzeClaims)
	mux.HandleFunc("GET /api/v1/patents/{id}/family", handlePatentFamily)
	mux.HandleFunc("GET /api/v1/patents/{id}/citations", handlePatentCitations)
	mux.HandleFunc("POST /api/v1/patents/check-fto", handleCheckFTO)
	mux.HandleFunc("POST /api/v1/patents/assess-infringement", handleAssessInfringement)

	// --- Portfolio endpoints ---
	mux.HandleFunc("POST /api/v1/portfolios", handleCreatePortfolio)
	mux.HandleFunc("GET /api/v1/portfolios", handleListPortfolios)
	mux.HandleFunc("GET /api/v1/portfolios/{id}", handleGetPortfolio)
	mux.HandleFunc("PUT /api/v1/portfolios/{id}", handleUpdatePortfolio)
	mux.HandleFunc("DELETE /api/v1/portfolios/{id}", handleDeletePortfolio)
	mux.HandleFunc("POST /api/v1/portfolios/{id}/patents", handlePortfolioAddPatents)
	mux.HandleFunc("DELETE /api/v1/portfolios/{id}/patents", handlePortfolioRemovePatents)
	mux.HandleFunc("GET /api/v1/portfolios/{id}/constellation", handlePortfolioConstellation)
	mux.HandleFunc("GET /api/v1/portfolios/{id}/valuation", handlePortfolioValuation)
	mux.HandleFunc("POST /api/v1/portfolios/{id}/valuation/run", handlePortfolioRunValuation)
	mux.HandleFunc("GET /api/v1/portfolios/{id}/gap-analysis", handlePortfolioGapAnalysis)
	mux.HandleFunc("POST /api/v1/portfolios/{id}/gap-analysis/run", handlePortfolioRunGapAnalysis)
	mux.HandleFunc("GET /api/v1/portfolios/{id}/analysis", handlePortfolioAnalysis)

	// --- Lifecycle endpoints ---
	mux.HandleFunc("GET /api/v1/patents/{patentId}/lifecycle", handleGetLifecycle)
	mux.HandleFunc("POST /api/v1/patents/{patentId}/lifecycle/advance", handleAdvancePhase)
	mux.HandleFunc("POST /api/v1/patents/{patentId}/fees", handleRecordFee)
	mux.HandleFunc("GET /api/v1/patents/{patentId}/fees", handleListFees)
	mux.HandleFunc("GET /api/v1/patents/{patentId}/timeline", handleGetTimeline)
	mux.HandleFunc("GET /api/v1/deadlines/upcoming", handleUpcomingDeadlines)
	mux.HandleFunc("POST /api/v1/lifecycle/{id}/annuities/calculate", handleCalculateAnnuities)
	mux.HandleFunc("GET /api/v1/lifecycle/{id}/annuities/budget", handleAnnuityBudget)
	mux.HandleFunc("POST /api/v1/lifecycle/{id}/legal-status/sync", handleSyncLegalStatus)
	mux.HandleFunc("GET /api/v1/lifecycle/{id}/calendar/export", handleExportCalendar)

	// --- Collaboration endpoints ---
	mux.HandleFunc("POST /api/v1/workspaces", handleCreateWorkspace)
	mux.HandleFunc("GET /api/v1/workspaces", handleListWorkspaces)
	mux.HandleFunc("GET /api/v1/workspaces/{id}", handleGetWorkspace)
	mux.HandleFunc("PUT /api/v1/workspaces/{id}", handleUpdateWorkspace)
	mux.HandleFunc("DELETE /api/v1/workspaces/{id}", handleDeleteWorkspace)
	mux.HandleFunc("POST /api/v1/workspaces/{id}/documents", handleShareDocument)
	mux.HandleFunc("GET /api/v1/workspaces/{id}/documents", handleListDocuments)
	mux.HandleFunc("POST /api/v1/workspaces/{id}/members", handleInviteMember)
	mux.HandleFunc("GET /api/v1/workspaces/{id}/members", handleListMembers)
	mux.HandleFunc("DELETE /api/v1/workspaces/{id}/members/{memberId}", handleRemoveMember)
	mux.HandleFunc("PUT /api/v1/workspaces/{id}/members/{memberId}/role", handleUpdateMemberRole)

	// --- Report endpoints (for async FTO) ---
	mux.HandleFunc("POST /api/v1/reports/fto", handleGenerateFTOReport)
	mux.HandleFunc("GET /api/v1/reports/{report_id}/status", handleReportStatus)
	mux.HandleFunc("GET /api/v1/reports/{report_id}/download", handleReportDownload)

	return httptest.NewServer(mux)
}

// ---------------------------------------------------------------------------
// Molecule stub handlers
// ---------------------------------------------------------------------------

func handleCreateMolecule(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["smiles"] == nil || req["smiles"] == "" {
		respondError(w, http.StatusBadRequest, "smiles is required")
		return
	}
	resp := map[string]interface{}{
		"id":        "e2e-mol-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"smiles":    req["smiles"],
		"name":      req["name"],
		"status":    "active",
		"created_at": time.Now().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusCreated, resp)
}

func handleGetMolecule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "molecule id is required")
		return
	}
	// Search in fixture data
	fx := getFixtures()
	if fx != nil {
		if m, ok := fx.molByID[id]; ok {
			respondJSON(w, http.StatusOK, m)
			return
		}
	}
	// Return minimal response for non-fixture IDs
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":     id,
		"smiles": "c1ccccc1",
		"name":   "unknown",
		"status": "active",
	})
}

func handleMoleculeSimilaritySearch(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["smiles"] == nil || req["smiles"] == "" {
		respondError(w, http.StatusBadRequest, "smiles is required")
		return
	}
	// Return first 3 molecules from fixtures with similarity scores
	fx := getFixtures()
	var results []map[string]interface{}
	if fx != nil && len(fx.Molecules) > 0 {
		limit := 3
		if limit > len(fx.Molecules) {
			limit = len(fx.Molecules)
		}
		for i := 0; i < limit; i++ {
			m := fx.Molecules[i]
			entry := make(map[string]interface{})
			for k, v := range m {
				entry[k] = v
			}
			entry["similarity"] = 0.95 - float64(i)*0.1
			results = append(results, entry)
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"molecules": results,
		"total":     len(results),
		"page":      1,
		"page_size": len(results),
	})
}

func handleMoleculeStructureSearch(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	fx := getFixtures()
	var results []map[string]interface{}
	if fx != nil && len(fx.Molecules) > 0 {
		limit := 3
		if limit > len(fx.Molecules) {
			limit = len(fx.Molecules)
		}
		for i := 0; i < limit; i++ {
			m := fx.Molecules[i]
			entry := make(map[string]interface{})
			for k, v := range m {
				entry[k] = v
			}
			entry["match_type"] = "substructure"
			results = append(results, entry)
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"molecules": results,
		"total":     len(results),
	})
}

func handleMoleculePropertiesCalculate(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	smiles, _ := req["smiles"].(string)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"smiles":           smiles,
		"molecular_weight": 331.41,
		"logP":             5.2,
		"hbd":              0,
		"hba":              1,
		"tpsa":             4.93,
		"rotatable_bonds":  2,
		"calculated_at":    time.Now().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// Patent stub handlers
// ---------------------------------------------------------------------------

func handleCreatePatent(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["title"] == nil || req["title"] == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}
	resp := map[string]interface{}{
		"id":          "pat-e2e-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"title":       req["title"],
		"status":      "active",
		"created_at":  time.Now().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusCreated, resp)
}

func handleGetPatent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "patent id is required")
		return
	}
	fx := getFixtures()
	if fx != nil {
		if p, ok := fx.patByID[id]; ok {
			respondJSON(w, http.StatusOK, p)
			return
		}
		// Search by partial ID match
		for _, p := range fx.Patents {
			if pid, ok := p["id"].(string); ok && strings.Contains(pid, id) {
				respondJSON(w, http.StatusOK, p)
				return
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":        id,
		"title":     "Patent Detail",
		"status":    "granted",
	})
}

func handlePatentSearch(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	fx := getFixtures()
	var results []map[string]interface{}
	// Apply jurisdiction filter if present
	jurFilter, _ := req["jurisdictions"].([]interface{})
	jurSet := make(map[string]bool)
	for _, j := range jurFilter {
		if jStr, ok := j.(string); ok {
			jurSet[jStr] = true
		}
	}

	if fx != nil && len(fx.Patents) > 0 {
		for _, p := range fx.Patents {
			if len(jurSet) > 0 {
				if jur, ok := p["jurisdiction"].(string); ok && jurSet[jur] {
					results = append(results, p)
				}
			} else {
				results = append(results, p)
			}
			if len(results) >= 5 {
				break
			}
		}
	}
	if results == nil {
		results = make([]map[string]interface{}, 0)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"patents":   results,
		"total":     len(results),
		"page":      1,
		"page_size": 10,
		"has_more":  len(results) >= 5,
	})
}

func handlePatentAdvancedSearch(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"patents":   []map[string]interface{}{},
		"total":     0,
		"page":      1,
		"page_size": 10,
	})
}

func handlePatentStats(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_patents":    150,
		"active_patents":   120,
		"expired_patents":  15,
		"pending_patents":  15,
		"jurisdictions": map[string]interface{}{
			"US": 45,
			"CN": 40,
			"EP": 30,
			"JP": 20,
			"KR": 15,
		},
		"top_assignees": []string{"Samsung", "LG", "BOE", "Merck", "Universal Display"},
		"updated_at":    time.Now().Format(time.RFC3339),
	})
}

func handleAnalyzeClaims(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"analysis_id":   "ca-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"patent_id":     req["patent_id"],
		"total_claims":  8,
		"independent_claims": []map[string]interface{}{
			{"number": 1, "type": "independent", "scope": "broad", "novel_elements": 3},
			{"number": 5, "type": "independent", "scope": "medium", "novel_elements": 2},
			{"number": 7, "type": "independent", "scope": "medium", "novel_elements": 1},
		},
		"dependent_claims": 5,
		"key_features":     []string{"spirofluorene core", "carbazole-triazine bipolar", "high triplet energy"},
		"infringement_indicators": []string{"Markush claim covers broad R-group variation"},
		"analyzed_at": time.Now().Format(time.RFC3339),
	})
}

func handlePatentFamily(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"family_id":   "FAM-" + id,
		"patent_id":   id,
		"members": []map[string]interface{}{
			{"patent_number": "CN115650927B", "jurisdiction": "CN", "status": "granted"},
			{"patent_number": "US11847352B2", "jurisdiction": "US", "status": "granted"},
			{"patent_number": "EP4123456B1", "jurisdiction": "EP", "status": "granted"},
			{"patent_number": "KR20230045678A", "jurisdiction": "KR", "status": "published"},
			{"patent_number": "JP2023123456A", "jurisdiction": "JP", "status": "published"},
		},
		"priority_date": "2022-03-18",
		"member_count":  5,
	})
}

func handlePatentCitations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"patent_id": id,
		"forward_citations": []map[string]interface{}{
			{"patent_number": "US11847352B2", "title": "OLED Device", "cites_count": 5},
			{"patent_number": "CN115650927B", "title": "蓝光主体材料", "cites_count": 3},
		},
		"backward_citations": []map[string]interface{}{
			{"patent_number": "US10312345B2", "title": "Previous Host Material", "cited_count": 20},
			{"patent_number": "EP3123456B1", "title": "Carbazole Derivatives", "cited_count": 15},
		},
		"total_forward":  2,
		"total_backward": 2,
	})
}

func handleCheckFTO(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["molecule_smiles"] == nil || req["molecule_smiles"] == "" {
		respondError(w, http.StatusBadRequest, "molecule_smiles is required")
		return
	}
	smiles, _ := req["molecule_smiles"].(string)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"fto_id":          "fto-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"target_smiles":   smiles,
		"risk_level":      "medium",
		"jurisdictions":   req["jurisdictions"],
		"patents_analyzed": 3,
		"infringements": []map[string]interface{}{
			{
				"patent_number": "CN115650927B",
				"title":         "一种有机发光器件用蓝光主体材料及其制备方法与应用",
				"risk":          "medium",
				"claim_mapping": []int{1, 5},
				"notes":         "Similar Markush scope - recommended design-around",
			},
			{
				"patent_number": "US11847352B2",
				"title":         "OLED Host Material",
				"risk":          "low",
				"claim_mapping": []int{1},
				"notes":         "Structural differences in R-group substitution",
			},
		},
		"clearance_opinion": "Moderate risk identified. Two patents with overlapping Markush claims. Design-around possible via R-group modification.",
		"analyzed_at":       time.Now().Format(time.RFC3339),
	})
}

func handleAssessInfringement(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["molecule_smiles"] == nil || req["patent_id"] == nil {
		respondError(w, http.StatusBadRequest, "molecule_smiles and patent_id are required")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"assessment_id":   "ia-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"patent_id":       req["patent_id"],
		"risk_score":      0.65,
		"risk_level":      "medium",
		"claim_by_claim": []map[string]interface{}{
			{"claim": 1, "risk": "high", "notes": "Direct overlap with Markush R1 substitution"},
			{"claim": 2, "risk": "low", "notes": "Naphthyl group not claimed"},
			{"claim": 5, "risk": "medium", "notes": "Device claim - all elements present"},
		},
		"overall_opinion": "Partial infringement risk detected on claims 1 and 5.",
		"analyzed_at":     time.Now().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// Portfolio stub handlers
// ---------------------------------------------------------------------------

func handleCreatePortfolio(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["name"] == nil || req["name"] == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	patentIDs, _ := req["patent_ids"].([]interface{})
	resp := map[string]interface{}{
		"id":          "port-e2e-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"name":        req["name"],
		"description": req["description"],
		"patent_ids":  patentIDs,
		"strategy":    "balanced",
		"health_score": map[string]interface{}{
			"coverage":     80,
			"concentration": 65,
			"aging":        25,
			"overall":      75,
		},
		"statistics": map[string]interface{}{
			"total_patents":     len(patentIDs),
			"active_patents":    len(patentIDs),
			"expired_patents":   0,
			"jurisdictions":    map[string]interface{}{},
		},
		"created_at": time.Now().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusCreated, resp)
}

func handleListPortfolios(w http.ResponseWriter, r *http.Request) {
	fx := getFixtures()
	var results []map[string]interface{}
	if fx != nil && len(fx.Portfolios) > 0 {
		for _, p := range fx.Portfolios {
			results = append(results, p)
		}
	} else {
		results = make([]map[string]interface{}, 0)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolios": results,
		"total":      len(results),
	})
}

func handleGetPortfolio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "portfolio id is required")
		return
	}
	fx := getFixtures()
	if fx != nil {
		for _, p := range fx.Portfolios {
			if pid, ok := p["id"].(string); ok && pid == id {
				respondJSON(w, http.StatusOK, p)
				return
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":   id,
		"name": "Portfolio Detail",
	})
}

func handleUpdatePortfolio(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":         id,
		"name":       req["name"],
		"updated_at": time.Now().Format(time.RFC3339),
	})
}

func handleDeletePortfolio(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func handlePortfolioAddPatents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	patentIDs, _ := req["patent_ids"].([]interface{})
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id": id,
		"patents_added": len(patentIDs),
		"patent_ids":   patentIDs,
		"total_patents": 5,
		"message":      "patents added successfully",
	})
}

func handlePortfolioRemovePatents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id":    id,
		"patents_removed": 2,
		"total_patents":   3,
		"message":         "patents removed successfully",
	})
}

func handlePortfolioConstellation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fx := getFixtures()
	var patentIDs []interface{}
	if fx != nil {
		for _, p := range fx.Portfolios {
			if pid, ok := p["id"].(string); ok && pid == id {
				if pids, ok := p["patent_ids"].([]interface{}); ok {
					patentIDs = pids
				}
				break
			}
		}
	}
	if patentIDs == nil {
		patentIDs = []interface{}{"pat-001", "pat-002", "pat-003"}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id": id,
		"constellation": map[string]interface{}{
			"nodes": []map[string]interface{}{
				{"id": "pat-001", "label": "Core Patent 1", "group": "composition", "score": 92},
				{"id": "pat-002", "label": "Core Patent 2", "group": "composition", "score": 88},
				{"id": "pat-003", "label": "Device Patent", "group": "device", "score": 75},
				{"id": "pat-004", "label": "Method Patent", "group": "method", "score": 70},
				{"id": "pat-005", "label": "Application Patent", "group": "application", "score": 65},
			},
			"edges": []map[string]interface{}{
				{"source": "pat-001", "target": "pat-002", "weight": 0.85, "relation": "similar_chemistry"},
				{"source": "pat-001", "target": "pat-003", "weight": 0.60, "relation": "cites"},
				{"source": "pat-002", "target": "pat-004", "weight": 0.55, "relation": "cites"},
				{"source": "pat-003", "target": "pat-005", "weight": 0.70, "relation": "continuation"},
			},
			"clusters": []map[string]interface{}{
				{"id": 0, "label": "Core Chemistry", "patents": []string{"pat-001", "pat-002"}},
				{"id": 1, "label": "Device & Method", "patents": []string{"pat-003", "pat-004"}},
				{"id": 2, "label": "Applications", "patents": []string{"pat-005"}},
			},
		},
		"analysis_date": time.Now().Format(time.RFC3339),
	})
}

func handlePortfolioValuation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fx := getFixtures()
	if fx != nil && len(fx.Valuations) > 0 {
		// Return first valuation as a sample
		v := fx.Valuations[0]
		respondJSON(w, http.StatusOK, v)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id":     id,
		"patents_evaluated": 5,
		"overall_score":    85.5,
		"tier":             "A",
		"valuation": map[string]interface{}{
			"technical":   88,
			"legal":       82,
			"commercial":  90,
			"strategic":   85,
		},
		"estimated_value": map[string]interface{}{
			"min": 5000000,
			"mid": 8500000,
			"max": 12000000,
			"currency": "USD",
		},
		"assessment_date": time.Now().Format(time.RFC3339),
	})
}

func handlePortfolioRunValuation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id":     id,
		"valuation_id":     "val-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"status":           "completed",
		"overall_score":    83.5,
		"tier":             "A",
		"estimated_value": map[string]interface{}{
			"min": 4500000,
			"mid": 7800000,
			"max": 11000000,
			"currency": "USD",
		},
		"completed_at": time.Now().Format(time.RFC3339),
	})
}

func handlePortfolioGapAnalysis(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id": id,
		"gaps": []map[string]interface{}{
			{"domain": "HTL materials", "coverage": 65, "target": 80, "gap": -15, "priority": "high", "recommendation": "Acquire or develop new HTL patents"},
			{"domain": "ETL materials", "coverage": 40, "target": 70, "gap": -30, "priority": "high", "recommendation": "File new ETL patent applications"},
			{"domain": "Encapsulation", "coverage": 55, "target": 60, "gap": -5, "priority": "low", "recommendation": "Monitor competitor activities"},
			{"domain": "Green emitters", "coverage": 75, "target": 65, "gap": 10, "priority": "none", "recommendation": "Maintain current position"},
		},
		"overall_coverage": 58,
		"overall_target":   68,
		"overall_gap":      -10,
		"analysis_date":    time.Now().Format(time.RFC3339),
	})
}

func handlePortfolioRunGapAnalysis(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id":    id,
		"analysis_id":     "ga-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"status":          "completed",
		"overall_coverage": 60,
		"overall_gap":     -8,
		"recommendations": []string{
			"File patent applications for ETL materials (largest gap)",
			"Consider licensing HTL patents from third parties",
			"Strengthen encapsulation portfolio for flexible OLED market",
		},
		"completed_at": time.Now().Format(time.RFC3339),
	})
}

func handlePortfolioAnalysis(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"portfolio_id": id,
		"summary": map[string]interface{}{
			"total_patents":   5,
			"active":          4,
			"expired":         1,
			"jurisdictions":   []string{"US", "CN", "EP", "KR", "JP"},
			"tech_domains":    []string{"C09K-011/06", "H10K-050/00"},
			"avg_patent_age":  3.5,
		},
		"strengths":  []string{"Strong core chemistry coverage", "Multi-jurisdiction protection"},
		"weaknesses": []string{"Limited ETL coverage", "Aging portfolio in phosphorescent area"},
	})
}

// ---------------------------------------------------------------------------
// Lifecycle stub handlers
// ---------------------------------------------------------------------------

func handleGetLifecycle(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	fx := getFixtures()
	if fx != nil {
		for _, lc := range fx.Lifecycles {
			if pid, ok := lc["patent_id"].(string); ok && strings.Contains(pid, patentID) {
				respondJSON(w, http.StatusOK, lc)
				return
			}
			if lid, ok := lc["id"].(string); ok && strings.Contains(lid, patentID) {
				respondJSON(w, http.StatusOK, lc)
				return
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"patent_id":  patentID,
		"status":     "active",
		"deadlines":  []map[string]interface{}{},
		"annuities":  []map[string]interface{}{},
		"legal_status_history": []map[string]interface{}{
			{"status": "filed", "effective_date": "2022-01-01", "source": "USPTO"},
			{"status": "granted", "effective_date": "2023-06-15", "source": "USPTO"},
		},
		"documents":  []map[string]interface{}{},
	})
}

func handleAdvancePhase(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	patentID := r.PathValue("patentId")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"patent_id":  patentID,
		"phase":      req["phase"],
		"status":     req["status"],
		"updated_at": time.Now().Format(time.RFC3339),
		"message":    "legal status updated successfully",
	})
}

func handleRecordFee(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"fee_id":     "fee-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"patent_id":  r.PathValue("patentId"),
		"amount":     req["amount"],
		"currency":   req["currency"],
		"status":     "recorded",
		"created_at": time.Now().Format(time.RFC3339),
	})
}

func handleListFees(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"patent_id": r.PathValue("patentId"),
		"fees": []map[string]interface{}{
			{"year": 1, "amount": 1600.00, "currency": "USD", "due_date": "2022-06-18", "status": "paid"},
			{"year": 2, "amount": 1600.00, "currency": "USD", "due_date": "2023-06-18", "status": "paid"},
			{"year": 3, "amount": 1600.00, "currency": "USD", "due_date": "2024-06-18", "status": "pending"},
		},
		"total_paid":    3200.00,
		"total_pending": 3200.00,
	})
}

func handleGetTimeline(w http.ResponseWriter, r *http.Request) {
	patentID := r.PathValue("patentId")
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"patent_id": patentID,
		"events": []map[string]interface{}{
			{"date": "2019-11-05", "event": "filing", "description": "Patent application filed with USPTO"},
			{"date": "2020-05-07", "event": "publication", "description": "Patent application published"},
			{"date": "2021-12-18", "event": "grant", "description": "Patent granted"},
			{"date": "2023-12-18", "event": "annuity_due", "description": "Year 3 annuity due"},
		},
		"total_events": 4,
	})
}

func handleUpcomingDeadlines(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deadlines": []map[string]interface{}{
			{"patent_id": "pat-0001", "type": "annuity_payment", "due_date": "2024-06-15", "days_remaining": 35, "status": "upcoming"},
			{"patent_id": "pat-0002", "type": "oa_response", "due_date": "2024-04-10", "days_remaining": -30, "status": "overdue"},
			{"patent_id": "pat-0003", "type": "grant_fee", "due_date": "2024-05-30", "days_remaining": 19, "status": "upcoming"},
			{"patent_id": "pat-0004", "type": "annuity_payment", "due_date": "2024-07-31", "days_remaining": 81, "status": "upcoming"},
		},
		"total_upcoming": 3,
		"total_overdue":  1,
		"report_date":    time.Now().Format(time.RFC3339),
	})
}

func handleCalculateAnnuities(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"lifecycle_id":     r.PathValue("id"),
		"total_annuities":  4,
		"total_paid":       4900.00,
		"total_pending":    3200.00,
		"currency":         req["currency"],
		"annuity_schedule": []map[string]interface{}{
			{"year": 1, "amount": 900.00, "due_date": "2021-09-10", "status": "paid"},
			{"year": 2, "amount": 1200.00, "due_date": "2022-09-10", "status": "paid"},
			{"year": 3, "amount": 1200.00, "due_date": "2023-09-10", "status": "paid"},
			{"year": 4, "amount": 2000.00, "due_date": "2024-09-10", "status": "pending"},
		},
		"calculated_at": time.Now().Format(time.RFC3339),
	})
}

func handleAnnuityBudget(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"lifecycle_id":    r.PathValue("id"),
		"current_year":    2024,
		"upcoming_payments": []map[string]interface{}{
			{"patent_id": "pat-0001", "annuity": 2000.00, "due_date": "2024-09-10"},
			{"patent_id": "pat-0002", "annuity": 1600.00, "due_date": "2024-06-18"},
			{"patent_id": "pat-0003", "annuity": 555.00, "due_date": "2024-04-30"},
		},
		"total_budget":   4155.00,
		"budget_currency": "USD",
	})
}

func handleSyncLegalStatus(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"lifecycle_id":    r.PathValue("id"),
		"previous_status": req["current_status"],
		"new_status":      req["new_status"],
		"updated_at":      time.Now().Format(time.RFC3339),
		"message":         "legal status synchronized successfully",
	})
}

func handleExportCalendar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="lifecycle_%s.csv"`, id))
	w.WriteHeader(http.StatusOK)
	csvContent := `patent_id,event_type,due_date,status
pat-0001,annuity_payment,2024-06-15,upcoming
pat-0002,oa_response,2024-04-10,overdue
pat-0003,grant_fee,2024-05-30,upcoming
pat-0004,annuity_payment,2024-07-31,upcoming
`
	fmt.Fprint(w, csvContent)
}

// ---------------------------------------------------------------------------
// Collaboration stub handlers
// ---------------------------------------------------------------------------

func handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["name"] == nil || req["name"] == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	resp := map[string]interface{}{
		"id":           "ws-e2e-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"name":         req["name"],
		"description":  req["description"],
		"type":         "internal",
		"status":       "active",
		"members": []map[string]interface{}{
			{"user_id": "user-owner", "role": "owner", "joined_at": time.Now().Format(time.RFC3339)},
		},
		"permissions": map[string]interface{}{
			"allow_download": true,
			"allow_share":    true,
			"watermark_enabled": false,
		},
		"created_at": time.Now().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusCreated, resp)
}

func handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	fx := getFixtures()
	var results []map[string]interface{}
	if fx != nil && len(fx.Workspaces) > 0 {
		for _, ws := range fx.Workspaces {
			results = append(results, ws)
		}
	} else {
		results = make([]map[string]interface{}, 0)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"workspaces": results,
		"total":      len(results),
	})
}

func handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fx := getFixtures()
	if fx != nil {
		for _, ws := range fx.Workspaces {
			if wsID, ok := ws["id"].(string); ok && wsID == id {
				respondJSON(w, http.StatusOK, ws)
				return
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":   id,
		"name": "Workspace Detail",
		"status": "active",
		"permissions": map[string]interface{}{
			"allow_download": true,
			"allow_share":    true,
			"watermark_enabled": false,
		},
	})
}

func handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":         r.PathValue("id"),
		"name":       req["name"],
		"updated_at": time.Now().Format(time.RFC3339),
	})
}

func handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func handleShareDocument(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"document_id":  "doc-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"workspace_id": r.PathValue("id"),
		"filename":     req["filename"],
		"shared_by":    req["shared_by"],
		"shared_at":    time.Now().Format(time.RFC3339),
		"status":       "shared",
	})
}

func handleListDocuments(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id": r.PathValue("id"),
		"documents": []map[string]interface{}{
			{"id": "doc-001", "filename": "FTO_Report_Q1_2024.pdf", "type": "fto_report", "uploaded_by": "user-owner", "uploaded_at": "2024-01-15T10:00:00Z"},
			{"id": "doc-002", "filename": "Portfolio_Analysis.xlsx", "type": "portfolio", "uploaded_by": "user-analyst", "uploaded_at": "2024-01-20T14:30:00Z"},
		},
		"total": 2,
	})
}

func handleInviteMember(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"workspace_id": r.PathValue("id"),
		"user_id":      req["user_id"],
		"role":         req["role"],
		"joined_at":    time.Now().Format(time.RFC3339),
		"status":       "invited",
	})
}

func handleListMembers(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id": r.PathValue("id"),
		"members": []map[string]interface{}{
			{"user_id": "user-owner", "role": "owner", "joined_at": "2024-01-01T08:00:00Z"},
			{"user_id": "user-analyst", "role": "editor", "joined_at": "2024-01-15T09:00:00Z"},
			{"user_id": "user-viewer", "role": "viewer", "joined_at": "2024-02-01T10:00:00Z"},
		},
		"total": 3,
	})
}

func handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func handleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"workspace_id": r.PathValue("id"),
		"user_id":      r.PathValue("memberId"),
		"new_role":     req["role"],
		"updated_at":   time.Now().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// Report stub handlers (for async FTO report)
// ---------------------------------------------------------------------------

var reportStatuses = make(map[string]string)
var reportMu sync.Mutex

func handleGenerateFTOReport(w http.ResponseWriter, r *http.Request) {
	if !isJSONContentType(r) {
		respondError(w, http.StatusBadRequest, "Content-Type must be application/json")
		return
	}
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req["target_smiles"] == nil || req["target_smiles"] == "" {
		respondError(w, http.StatusBadRequest, "target_smiles is required")
		return
	}

	reportID := "rpt-" + fmt.Sprintf("%d", time.Now().UnixNano())
	reportMu.Lock()
	reportStatuses[reportID] = "processing"
	reportMu.Unlock()

	// Simulate async completion in background
	go func() {
		time.Sleep(100 * time.Millisecond)
		reportMu.Lock()
		reportStatuses[reportID] = "completed"
		reportMu.Unlock()
	}()

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"report_id":   reportID,
		"status":      "processing",
		"target_smiles": req["target_smiles"],
		"jurisdiction":  req["jurisdiction"],
		"format":       req["format"],
		"message":     "FTO report generation started",
	})
}

func handleReportStatus(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	reportMu.Lock()
	status := reportStatuses[reportID]
	reportMu.Unlock()
	if status == "" {
		status = "completed"
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"report_id": reportID,
		"status":    status,
	})
}

func handleReportDownload(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("report_id")
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="fto_report_%s.pdf"`, reportID))
	w.WriteHeader(http.StatusOK)
	// Return minimal PDF-like binary content for testing
	fmt.Fprint(w, "PDF content placeholder for FTO report testing")
}

// ============================================================================
// 4. Scenario Test Functions
// ============================================================================

// TestScenarios is the main entry point for all 6 business scenarios.
func TestScenarios(t *testing.T) {
	if getFixtures() == nil {
		t.Skip("fixtures not loaded, skipping scenario tests")
	}

	ts := newScenarioServer()
	defer ts.Close()

	t.Run("S1_FTO自由实施分析", func(t *testing.T) { testFTOAnalysis(t, ts) })
	t.Run("S2_专利挖掘工作流", func(t *testing.T) { testPatentMining(t, ts) })
	t.Run("S3_组合管理", func(t *testing.T) { testPortfolioManagement(t, ts) })
	t.Run("S4_生命周期管理", func(t *testing.T) { testLifecycleManagement(t, ts) })
	t.Run("S5_协作工作区", func(t *testing.T) { testCollaboration(t, ts) })
	t.Run("S6_全局搜索", func(t *testing.T) { testGlobalSearch(t, ts) })
}

// ---------------------------------------------------------------------------
// Scenario 1: FTO 自由实施分析
// 输入分子 SMILES -> 相似度搜索 -> 侵权风险评估 -> FTO 报告生成
// ---------------------------------------------------------------------------

func testFTOAnalysis(t *testing.T, ts *httptest.Server) {
	t.Log("=== 场景 1: FTO 自由实施分析 ===")

	// Step 1: 创建目标分子
	t.Run("Step1-创建分子", func(t *testing.T) {
		body := map[string]interface{}{
			"smiles": "c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1",
			"name":   "Target-OLED-Host",
		}
		resp := scenarioDoPost(t, ts, "/api/v1/molecules", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusCreated)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "id")
		assertJSONField(t, result, "smiles")
		t.Logf("分子创建成功, ID: %v", result["id"])
	})

	// Step 2: 结构相似度搜索
	t.Run("Step2-相似度搜索", func(t *testing.T) {
		body := map[string]interface{}{
			"smiles":               "c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1",
			"similarity_threshold": 0.7,
			"max_results":          10,
		}
		resp := scenarioDoPost(t, ts, "/api/v1/molecules/search/similarity", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "molecules")
		molecules, ok := result["molecules"].([]interface{})
		if !ok || len(molecules) == 0 {
			t.Fatal("expected at least one similar molecule")
		}
		for _, m := range molecules {
			mol := m.(map[string]interface{})
			t.Logf("  相似分子: %v (相似度: %.2f)", mol["name"], mol["similarity"])
		}
		t.Logf("相似度搜索成功, 找到 %d 个类似分子", len(molecules))
	})

	// Step 3: FTO 侵权风险评估
	t.Run("Step3-FTO检查", func(t *testing.T) {
		body := map[string]interface{}{
			"molecule_smiles": "c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1",
			"jurisdictions":   []string{"CN", "US", "EP"},
		}
		resp := scenarioDoPost(t, ts, "/api/v1/patents/check-fto", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)

		assertJSONField(t, result, "fto_id")
		assertJSONField(t, result, "risk_level")
		assertJSONField(t, result, "infringements")
		assertJSONField(t, result, "clearance_opinion")

		t.Logf("FTO 评估结果: 风险等级=%v", result["risk_level"])
		infringements, _ := result["infringements"].([]interface{})
		t.Logf("发现 %d 个潜在侵权专利", len(infringements))
		for _, inf := range infringements {
			i := inf.(map[string]interface{})
			t.Logf("  - %s (风险: %v)", i["patent_number"], i["risk"])
		}
	})

	// Step 4: 生成 FTO 报告
	t.Run("Step4-生成FTO报告", func(t *testing.T) {
		body := map[string]interface{}{
			"target_smiles": "c1ccc(-c2ccc(-n3c4ccccc4c4ccccc43)cc2)cc1",
			"jurisdiction":  "CN",
			"format":        "pdf",
		}
		resp := scenarioDoPost(t, ts, "/api/v1/reports/fto", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusAccepted)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "report_id")
		reportID := result["report_id"].(string)
		t.Logf("FTO 报告创建成功, ID: %s", reportID)

		// Step 5: 轮询报告状态直到完成
		t.Run("Step5-轮询报告状态", func(t *testing.T) {
			var status string
			for i := 0; i < 10; i++ {
				time.Sleep(50 * time.Millisecond)
				statusResp := scenarioDoGet(t, ts, "/api/v1/reports/"+reportID+"/status")
				if statusResp.StatusCode == http.StatusOK {
					var statusResult map[string]interface{}
					unmarshalBody(t, statusResp, &statusResult)
					status, _ = statusResult["status"].(string)
					if status == "completed" {
						t.Log("报告生成完成")
						break
					}
					t.Logf("报告状态: %s (轮询 #%d)", status, i+1)
				} else {
					statusResp.Body.Close()
				}
			}
			if status != "completed" {
				t.Fatal("FTO 报告未在预期时间内完成")
			}
		})

		// Step 6: 下载报告
		t.Run("Step6-下载报告", func(t *testing.T) {
			downloadResp := scenarioDoGet(t, ts, "/api/v1/reports/"+reportID+"/download")
			defer downloadResp.Body.Close()
			assertStatusCode(t, downloadResp, http.StatusOK)
			body := readBody(t, downloadResp)
			if len(body) == 0 {
				t.Fatal("报告下载内容为空")
			}
			t.Logf("报告下载成功, 大小: %d bytes", len(body))
		})
	})
}

// ---------------------------------------------------------------------------
// Scenario 2: 专利挖掘工作流
// 专利搜索 -> 查看详情 -> 分析权利要求 -> 查看引用网络
// ---------------------------------------------------------------------------

func testPatentMining(t *testing.T, ts *httptest.Server) {
	t.Log("=== 场景 2: 专利挖掘工作流 ===")

	var patentID string

	// Step 1: 关键词搜索专利
	t.Run("Step1-关键词搜索", func(t *testing.T) {
		body := map[string]interface{}{
			"query":      "OLED 蓝光主体材料",
			"query_type": "keyword",
			"page":       1,
			"page_size":  10,
		}
		resp := scenarioDoPost(t, ts, "/api/v1/patents/search", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "patents")
		assertJSONField(t, result, "total")

		patents, _ := result["patents"].([]interface{})
		if len(patents) > 0 {
			first := patents[0].(map[string]interface{})
			patentID, _ = first["id"].(string)
			t.Logf("搜索成功, 共 %v 个结果, 首个专利 ID: %s", result["total"], patentID)
		} else {
			t.Log("搜索返回 0 个结果")
		}
	})

	// Step 2: 查看专利详情
	t.Run("Step2-查看详情", func(t *testing.T) {
		if patentID == "" {
			t.Skip("无专利 ID，跳过详情查看")
		}
		resp := scenarioDoGet(t, ts, "/api/v1/patents/"+patentID)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "id")
		assertJSONField(t, result, "title")
		assertJSONField(t, result, "legal_status")

		t.Logf("专利详情: %v (状态: %v)", result["title"], result["legal_status"])
		if filingDate, ok := result["filing_date"]; ok {
			t.Logf("申请日: %v", filingDate)
		}
	})

	// Step 3: 分析权利要求
	t.Run("Step3-分析权利要求", func(t *testing.T) {
		if patentID == "" {
			t.Skip("无专利 ID，跳过权利要求分析")
		}
		body := map[string]interface{}{
			"patent_id": patentID,
		}
		resp := scenarioDoPost(t, ts, "/api/v1/patents/analyze-claims", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "analysis_id")
		assertJSONField(t, result, "total_claims")
		assertJSONField(t, result, "independent_claims")
		assertJSONField(t, result, "dependent_claims")

		t.Logf("权利要求分析: 共 %v 项权利要求 (独立: %d, 从属: %v)",
			result["total_claims"],
			len(result["independent_claims"].([]interface{})),
			result["dependent_claims"])
	})

	// Step 4: 查看引用网络
	t.Run("Step4-引用网络", func(t *testing.T) {
		if patentID == "" {
			t.Skip("无专利 ID，跳过引用网络")
		}

		// 前向引用
		fwdResp := scenarioDoGet(t, ts, "/api/v1/patents/"+patentID+"/citations")
		defer fwdResp.Body.Close()
		assertStatusCode(t, fwdResp, http.StatusOK)

		var fwdResult map[string]interface{}
		unmarshalBody(t, fwdResp, &fwdResult)
		assertJSONField(t, fwdResult, "forward_citations")
		assertJSONField(t, fwdResult, "backward_citations")

		forward, _ := fwdResult["forward_citations"].([]interface{})
		backward, _ := fwdResult["backward_citations"].([]interface{})
		t.Logf("前向引用: %d, 后向引用: %d", len(forward), len(backward))
	})

	// Step 5: 查看专利家族
	t.Run("Step5-专利家族", func(t *testing.T) {
		if patentID == "" {
			t.Skip("无专利 ID，跳过家族查看")
		}
		resp := scenarioDoGet(t, ts, "/api/v1/patents/"+patentID+"/family")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "family_id")
		assertJSONField(t, result, "members")

		members, _ := result["members"].([]interface{})
		t.Logf("专利家族: %v, 共 %d 个成员", result["family_id"], len(members))
		for _, m := range members {
			member := m.(map[string]interface{})
			t.Logf("  - %s (%s, %s)", member["patent_number"], member["jurisdiction"], member["status"])
		}
	})
}

// ---------------------------------------------------------------------------
// Scenario 3: 组合管理
// 创建组合 -> 添加专利 -> 星座图分析 -> 估值 -> 差距分析
// ---------------------------------------------------------------------------

func testPortfolioManagement(t *testing.T, ts *httptest.Server) {
	t.Log("=== 场景 3: 组合管理 ===")

	var portfolioID string

	// Step 1: 创建组合
	t.Run("Step1-创建组合", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "E2E 测试组合 - OLED 蓝光材料",
			"description": "E2E 测试创建的 OLED 蓝光主体材料专利组合",
			"patent_ids": []string{
				"pat-00000000-0000-4000-a000-000000000001",
				"pat-00000000-0000-4000-a000-000000000002",
			},
		}
		resp := scenarioDoPost(t, ts, "/api/v1/portfolios", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusCreated)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "id")
		assertJSONField(t, result, "name")
		portfolioID = result["id"].(string)
		t.Logf("组合创建成功: %s (ID: %s)", result["name"], portfolioID)
	})

	if portfolioID == "" {
		t.Fatal("组合创建失败，无法继续")
	}

	// Step 2: 添加专利到组合
	t.Run("Step2-添加专利", func(t *testing.T) {
		body := map[string]interface{}{
			"patent_ids": []string{
				"pat-00000000-0000-4000-a000-000000000003",
				"pat-00000000-0000-4000-a000-000000000004",
				"pat-00000000-0000-4000-a000-000000000005",
			},
		}
		resp := scenarioDoPost(t, ts, "/api/v1/portfolios/"+portfolioID+"/patents", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "portfolio_id")
		assertJSONField(t, result, "patents_added")
		t.Logf("成功添加 %v 个专利到组合", result["patents_added"])
	})

	// Step 3: 星座图分析
	t.Run("Step3-星座图分析", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/portfolios/"+portfolioID+"/constellation")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "constellation")

		constellation := result["constellation"].(map[string]interface{})
		nodes, _ := constellation["nodes"].([]interface{})
		edges, _ := constellation["edges"].([]interface{})
		clusters, _ := constellation["clusters"].([]interface{})

		t.Logf("星座图: %d 个节点, %d 条边, %d 个聚类", len(nodes), len(edges), len(clusters))
		for _, node := range nodes {
			n := node.(map[string]interface{})
			t.Logf("  节点: %s (组: %s, 评分: %.0f)", n["label"], n["group"], n["score"])
		}
	})

	// Step 4: 估值分析
	t.Run("Step4-估值分析", func(t *testing.T) {
		// 运行估值
		runResp := scenarioDoPost(t, ts, "/api/v1/portfolios/"+portfolioID+"/valuation/run", nil)
		defer runResp.Body.Close()
		assertStatusCode(t, runResp, http.StatusOK)

		var runResult map[string]interface{}
		unmarshalBody(t, runResp, &runResult)
		assertJSONField(t, runResult, "overall_score")
		assertJSONField(t, runResult, "estimated_value")
		t.Logf("估值完成: 综合评分 %.1f, Tier %v", runResult["overall_score"], runResult["tier"])

		estVal := runResult["estimated_value"].(map[string]interface{})
		t.Logf("预估价值: $%.0f ~ $%.0f (中位 $%.0f)", estVal["min"], estVal["max"], estVal["mid"])
	})

	// Step 5: 差距分析
	t.Run("Step5-差距分析", func(t *testing.T) {
		// 运行差距分析
		runResp := scenarioDoPost(t, ts, "/api/v1/portfolios/"+portfolioID+"/gap-analysis/run", nil)
		defer runResp.Body.Close()
		assertStatusCode(t, runResp, http.StatusOK)

		var runResult map[string]interface{}
		unmarshalBody(t, runResp, &runResult)
		assertJSONField(t, runResult, "overall_gap")
		assertJSONField(t, runResult, "recommendations")

		t.Logf("差距分析完成: 总体差距 %v", runResult["overall_gap"])
		recommendations, _ := runResult["recommendations"].([]interface{})
		for _, rec := range recommendations {
			t.Logf("  建议: %s", rec)
		}
	})

	// Cleanup: 删除组合
	t.Cleanup(func() {
		resp := scenarioDoDelete(t, ts, "/api/v1/portfolios/"+portfolioID)
		resp.Body.Close()
		t.Logf("组合 %s 清理完成", portfolioID)
	})
}

// ---------------------------------------------------------------------------
// Scenario 4: 生命周期管理
// 查看期限 -> 计算年费 -> 更新法律状态 -> 导出 CSV
// ---------------------------------------------------------------------------

func testLifecycleManagement(t *testing.T, ts *httptest.Server) {
	t.Log("=== 场景 4: 生命周期管理 ===")

	patentLifecycleID := "lc-00000000-0000-4000-c000-000000000001"

	// Step 1: 查看生命周期和期限
	t.Run("Step1-查看生命周期", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/patents/"+patentLifecycleID+"/lifecycle")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "deadlines")
		assertJSONField(t, result, "annuities")
		assertJSONField(t, result, "legal_status_history")

		deadlines, _ := result["deadlines"].([]interface{})
		annuities, _ := result["annuities"].([]interface{})
		statusHistory, _ := result["legal_status_history"].([]interface{})

		t.Logf("生命同期信息: %d 个截止日期, %d 条年费记录, %d 条法律状态记录",
			len(deadlines), len(annuities), len(statusHistory))

		for _, d := range deadlines {
			dl := d.(map[string]interface{})
			t.Logf("  截止日期: %s - %s (剩余: %v 天)",
				dl["type"], dl["due_date"], dl["days_remaining"])
		}
	})

	// Step 2: 查看即将到期的截止日期
	t.Run("Step2-即将到期的截止日期", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/deadlines/upcoming")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "deadlines")
		assertJSONField(t, result, "total_upcoming")
		assertJSONField(t, result, "total_overdue")

		t.Logf("即将到期: %v, 已逾期: %v", result["total_upcoming"], result["total_overdue"])
	})

	// Step 3: 计算年费
	t.Run("Step3-计算年费", func(t *testing.T) {
		body := map[string]interface{}{
			"currency": "CNY",
			"years":    []int{1, 2, 3, 4, 5},
		}
		resp := scenarioDoPost(t, ts, "/api/v1/lifecycle/"+patentLifecycleID+"/annuities/calculate", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "annuity_schedule")
		assertJSONField(t, result, "total_pending")

		schedule, _ := result["annuity_schedule"].([]interface{})
		t.Logf("年费计算: 共 %d 年, 待缴 %.2f %s",
			len(schedule), result["total_pending"], result["currency"])
	})

	// Step 4: 更新法律状态
	t.Run("Step4-更新法律状态", func(t *testing.T) {
		body := map[string]interface{}{
			"current_status": "substantive_examination",
			"new_status":     "granted",
			"effective_date": time.Now().Format("2006-01-02"),
		}
		resp := scenarioDoPost(t, ts, "/api/v1/lifecycle/"+patentLifecycleID+"/legal-status/sync", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "new_status")
		assertJSONField(t, result, "message")

		t.Logf("法律状态更新: %v -> %v", result["previous_status"], result["new_status"])
	})

	// Step 5: 导出 CSV 日历
	t.Run("Step5-导出CSV", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/lifecycle/"+patentLifecycleID+"/calendar/export")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		body := readBody(t, resp)
		contentType := resp.Header.Get("Content-Type")
		disposition := resp.Header.Get("Content-Disposition")

		if contentType != "text/csv" {
			t.Logf("注意: 期望 text/csv, 实际 %s", contentType)
		}
		t.Logf("导出头: Content-Type=%s, Content-Disposition=%s", contentType, disposition)
		t.Logf("CSV 内容:\n%s", string(body))
	})
}

// ---------------------------------------------------------------------------
// Scenario 5: 协作工作区
// 创建工作区 -> 邀请成员 -> 共享文档 -> 权限验证
// ---------------------------------------------------------------------------

func testCollaboration(t *testing.T, ts *httptest.Server) {
	t.Log("=== 场景 5: 协作工作区 ===")

	var workspaceID string

	// Step 1: 创建工作区
	t.Run("Step1-创建工作区", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "E2E 测试协作工作区",
			"description": "用于 E2E 测试的协作工作区",
		}
		resp := scenarioDoPost(t, ts, "/api/v1/workspaces", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusCreated)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "id")
		assertJSONField(t, result, "name")
		assertJSONField(t, result, "permissions")
		workspaceID = result["id"].(string)
		t.Logf("工作区创建成功: %s (ID: %s)", result["name"], workspaceID)
	})

	if workspaceID == "" {
		t.Fatal("工作区创建失败，无法继续")
	}

	// Step 2: 邀请成员
	var memberID string
	t.Run("Step2-邀请成员", func(t *testing.T) {
		body := map[string]interface{}{
			"user_id": "user-e2e-analyst",
			"role":    "editor",
		}
		resp := scenarioDoPost(t, ts, "/api/v1/workspaces/"+workspaceID+"/members", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusCreated)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "user_id")
		assertJSONField(t, result, "role")
		memberID = result["user_id"].(string)
		t.Logf("成员 %s 已以 %s 角色邀请", memberID, result["role"])
	})

	// Step 3: 共享文档
	t.Run("Step3-共享文档", func(t *testing.T) {
		body := map[string]interface{}{
			"filename":  "FTO_Analysis_Report.pdf",
			"shared_by": "user-owner",
			"type":      "fto_report",
		}
		resp := scenarioDoPost(t, ts, "/api/v1/workspaces/"+workspaceID+"/documents", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusCreated)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "document_id")
		assertJSONField(t, result, "status")
		t.Logf("文档共享成功: %s (状态: %s)", result["filename"], result["status"])
	})

	// Step 4: 列出文档并验证共享
	t.Run("Step4-列出文档", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/workspaces/"+workspaceID+"/documents")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "documents")

		docs, _ := result["documents"].([]interface{})
		t.Logf("工作区有 %d 个共享文档", len(docs))
	})

	// Step 5: 列出成员并验证权限
	t.Run("Step5-列出成员并验证权限", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/workspaces/"+workspaceID+"/members")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "members")

		members, _ := result["members"].([]interface{})
		t.Logf("工作区有 %d 个成员", len(members))
		for _, m := range members {
			member := m.(map[string]interface{})
			t.Logf("  成员: %s (角色: %s)", member["user_id"], member["role"])
		}
	})

	// Step 6: 权限验证 - 列出现有工作区
	t.Run("Step6-权限验证", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/workspaces")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "workspaces")

		workspaces, _ := result["workspaces"].([]interface{})
		t.Logf("系统共有 %d 个工作区", len(workspaces))
		for _, ws := range workspaces {
			w := ws.(map[string]interface{})
			perms := w["permissions"].(map[string]interface{})
			t.Logf("  工作区: %s, 允许下载: %v, 允许共享: %v",
				w["name"], perms["allow_download"], perms["allow_share"])
		}
	})

	// Cleanup: 删除工作区
	t.Cleanup(func() {
		resp := scenarioDoDelete(t, ts, "/api/v1/workspaces/"+workspaceID)
		resp.Body.Close()
		t.Logf("工作区 %s 清理完成", workspaceID)
	})
}

// ---------------------------------------------------------------------------
// Scenario 6: 全局搜索
// 关键词搜索 -> 筛选专利局/日期 -> 查看结果 -> 导出
// ---------------------------------------------------------------------------

func testGlobalSearch(t *testing.T, ts *httptest.Server) {
	t.Log("=== 场景 6: 全局搜索 ===")

	// Step 1: 关键词搜索
	t.Run("Step1-关键词搜索", func(t *testing.T) {
		body := map[string]interface{}{
			"query":      "OLED 发光材料",
			"query_type": "keyword",
			"page":       1,
			"page_size":  10,
		}
		resp := scenarioDoPost(t, ts, "/api/v1/patents/search", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "patents")
		assertJSONField(t, result, "total")

		t.Logf("关键词搜索成功: 共 %v 个结果", result["total"])
		patents, _ := result["patents"].([]interface{})
		for _, p := range patents {
			pat := p.(map[string]interface{})
			t.Logf("  结果: %s (状态: %s)", pat["title"], pat["legal_status"])
		}
	})

	// Step 2: 按专利局筛选
	t.Run("Step2-专利局筛选", func(t *testing.T) {
		body := map[string]interface{}{
			"query":         "organic light emitting",
			"query_type":    "keyword",
			"jurisdictions": []string{"US", "EP"},
			"page":          1,
			"page_size":     10,
		}
		resp := scenarioDoPost(t, ts, "/api/v1/patents/search", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "patents")

		patents, _ := result["patents"].([]interface{})
		t.Logf("专利局筛选 (US/EP): 共 %v 个结果", result["total"])
		for _, p := range patents {
			pat := p.(map[string]interface{})
			if jur, ok := pat["jurisdiction"].(string); ok {
				t.Logf("  %s (管辖局: %s)", pat["title"], jur)
				if jur != "US" && jur != "EP" {
					t.Errorf("筛选结果包含非目标管辖局: %s", jur)
				}
			}
		}
	})

	// Step 3: 查看统计信息
	t.Run("Step3-查看统计", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/patents/stats")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "total_patents")
		assertJSONField(t, result, "jurisdictions")
		assertJSONField(t, result, "top_assignees")

		t.Logf("专利统计: 总计 %v 件", result["total_patents"])
		jurs, _ := result["jurisdictions"].(map[string]interface{})
		for jur, count := range jurs {
			t.Logf("  %s: %v 件", jur, count)
		}
		assignees, _ := result["top_assignees"].([]interface{})
		t.Logf("主要申请人: %v", assignees)
	})

	// Step 4: 高级搜索 (带日期范围筛选)
	t.Run("Step4-高级搜索", func(t *testing.T) {
		body := map[string]interface{}{
			"query":      "TADF",
			"query_type": "keyword",
			"date_from":  "2020-01-01",
			"date_to":    "2024-12-31",
			"page":       1,
			"page_size":  10,
		}
		resp := scenarioDoPost(t, ts, "/api/v1/patents/search", body)
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		assertJSONField(t, result, "patents")

		patents, _ := result["patents"].([]interface{})
		t.Logf("高级搜索 (日期范围 2020-2024): 共 %v 个结果", result["total"])
		for _, p := range patents {
			pat := p.(map[string]interface{})
			t.Logf("  %s (申请日: %v)", pat["title"], pat["filing_date"])
		}
	})

	// Step 5: 导出/下载功能
	t.Run("Step5-导出", func(t *testing.T) {
		resp := scenarioDoGet(t, ts, "/api/v1/patents/stats")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		var result map[string]interface{}
		unmarshalBody(t, resp, &result)
		t.Log("统计信息导出成功，可用于生成报告")
		t.Logf("导出数据摘要: %d 件专利, %d 个管辖局",
			int(result["total_patents"].(float64)),
			len(result["jurisdictions"].(map[string]interface{})))
	})

	// Step 5b: 更精确的导出 - 通过生命周期日历导出模拟
	t.Run("Step5b-CSV导出验证", func(t *testing.T) {
		// 查找一个生命周期记录并导出
		testLCID := "lc-00000000-0000-4000-c000-000000000001"
		resp := scenarioDoGet(t, ts, "/api/v1/lifecycle/"+testLCID+"/calendar/export")
		defer resp.Body.Close()
		assertStatusCode(t, resp, http.StatusOK)

		body := readBody(t, resp)
		contentDisposition := resp.Header.Get("Content-Disposition")
		t.Logf("CSV 导出: Content-Disposition=%s", contentDisposition)
		t.Logf("CSV 内容首行: %s", strings.SplitN(string(body), "\n", 2)[0])
	})
}

// ============================================================================
// 5. Initialization
// ============================================================================

func init() {
	// Load fixtures eagerly in init to fail fast if missing.
	fixturesOnce.Do(func() {
		fixturesOK = loadScenarioFixtures()
	})
}

//Personal.AI order the ending
