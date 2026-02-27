// Phase 13 - SDK Patents Sub-Client Test (301/349)
// File: pkg/client/patents_test.go
// Comprehensive unit tests for PatentsClient.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	kerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestPatentsClient(t *testing.T, handler http.HandlerFunc) *PatentsClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL, "test-key",
		WithHTTPClient(srv.Client()),
		WithRetryMax(0),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c.Patents()
}

func ptReadBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal body: %v (raw: %s)", err, string(b))
	}
	return m
}

func ptWriteJSON(t *testing.T, w http.ResponseWriter, status int, v interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

// captureRequest returns a handler that captures the last request and body,
// plus two getters to retrieve them. The handler responds with respBody.
func captureRequest(t *testing.T, status int, respBody interface{}) (http.HandlerFunc, func() *http.Request, func() []byte) {
	t.Helper()
	var mu sync.Mutex
	var captured *http.Request
	var capturedBody []byte

	handler := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		captured = r
		capturedBody = b
		mu.Unlock()
		ptWriteJSON(t, w, status, respBody)
	}
	getReq := func() *http.Request {
		mu.Lock()
		defer mu.Unlock()
		return captured
	}
	getBody := func() []byte {
		mu.Lock()
		defer mu.Unlock()
		return capturedBody
	}
	return handler, getReq, getBody
}

func samplePatent() Patent {
	return Patent{
		ID: "pat-001", PatentNumber: "CN115000001B",
		Title: "一种化合物及其制备方法", Abstract: "本发明涉及...",
		Applicants: []string{"某药业"}, Inventors: []string{"张三"},
		Assignees: []string{"某药业"}, Jurisdiction: "CN",
		FilingDate: "2021-06-15", PublicationDate: "2022-01-01",
		GrantDate: "2024-03-01", PriorityDate: "2021-06-15",
		IPCCodes: []string{"C07D401/04", "A61K31/506"},
		CPCCodes: []string{"C07D401/04"}, Status: "granted",
		MoleculeCount: 3, CitationCount: 12,
		RelevanceScore: 0.95, PatentValue: 78.5,
	}
}

func samplePatentDetail() PatentDetail {
	return PatentDetail{
		Patent:    samplePatent(),
		FamilyID: "fam-001",
		Claims: []Claim{
			{Number: 1, Text: "A compound of formula I...", Type: "independent", DependsOn: []int{}},
			{Number: 2, Text: "The compound of claim 1 wherein...", Type: "dependent", DependsOn: []int{1}},
			{Number: 3, Text: "A pharmaceutical composition...", Type: "independent", DependsOn: []int{}},
		},
		Description: "DETAILED DESCRIPTION...",
		Drawings: []Drawing{
			{Number: 1, URL: "https://example.com/d1.png", Description: "Figure 1"},
		},
		FamilyMembers: []FamilyMember{
			{PatentNumber: "US11000001B2", Jurisdiction: "US", Status: "granted", FilingDate: "2021-12-01", PublicationDate: "2022-06-01"},
			{PatentNumber: "EP4000001A1", Jurisdiction: "EP", Status: "pending", FilingDate: "2021-12-15", PublicationDate: "2022-07-01"},
		},
		Citations: []Citation{
			{PatentNumber: "US9000001B2", Title: "Prior art A", CitationType: "examiner", Relevance: "X"},
			{PatentNumber: "CN110000001A", Title: "Prior art B", CitationType: "applicant", Relevance: "Y"},
		},
		CitedBy: []Citation{
			{PatentNumber: "CN116000001A", Title: "Citing patent", CitationType: "examiner", Relevance: "A"},
		},
		Molecules: []MoleculeBrief{
			{ID: "mol-001", SMILES: "CCO", MolecularFormula: "C2H6O", MolecularWeight: 46.07, Relevance: 0.9},
		},
		LegalEvents: []LegalEvent{
			{Date: "2021-06-15", Code: "FILING", Description: "Application filed", Source: "CNIPA_API"},
			{Date: "2024-03-01", Code: "GRANT", Description: "Patent granted", Source: "CNIPA_API"},
		},
		FullTextURL: "https://example.com/CN115000001B.pdf",
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestPatentSearch_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, PatentSearchResult{
			Patents:    []Patent{samplePatent(), samplePatent(), samplePatent()},
			Total:      150, Page: 1, PageSize: 20, HasMore: true, SearchTime: 0.123,
			Facets: &SearchFacets{
				Jurisdictions: []FacetItem{{Value: "CN", Count: 100}, {Value: "US", Count: 50}},
				Years:         []FacetItem{{Value: "2024", Count: 30}},
				Applicants:    []FacetItem{{Value: "某药业", Count: 20}},
				IPCCodes:      []FacetItem{{Value: "C07D", Count: 80}},
				Status:        []FacetItem{{Value: "granted", Count: 90}},
			},
		})
	})
	res, err := pc.Search(context.Background(), &PatentSearchRequest{Query: "kinase inhibitor"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Patents) != 3 {
		t.Errorf("Patents: want 3, got %d", len(res.Patents))
	}
	if res.Total != 150 {
		t.Errorf("Total: want 150, got %d", res.Total)
	}
	if res.Facets == nil {
		t.Fatal("Facets should not be nil")
	}
	if len(res.Facets.Jurisdictions) != 2 {
		t.Errorf("Facets.Jurisdictions: want 2, got %d", len(res.Facets.Jurisdictions))
	}
	if res.Facets.Jurisdictions[0].Value != "CN" || res.Facets.Jurisdictions[0].Count != 100 {
		t.Errorf("Facets.Jurisdictions[0]: want CN/100, got %s/%d", res.Facets.Jurisdictions[0].Value, res.Facets.Jurisdictions[0].Count)
	}
}

func TestPatentSearch_EmptyQuery(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.Search(context.Background(), &PatentSearchRequest{Query: ""})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentSearch_NilRequest(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.Search(context.Background(), nil)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentSearch_DefaultValues(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.Search(context.Background(), &PatentSearchRequest{Query: "test"})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["query_type"] != "keyword" {
		t.Errorf("query_type: want keyword, got %v", body["query_type"])
	}
	if body["page"] != float64(1) {
		t.Errorf("page: want 1, got %v", body["page"])
	}
	// clampPageSize(0) returns 1 (since 0 < 1)
	if body["page_size"] != float64(1) {
		t.Errorf("page_size: want 1, got %v", body["page_size"])
	}
	if body["sort_order"] != "desc" {
		t.Errorf("sort_order: want desc, got %v", body["sort_order"])
	}
}

func TestPatentSearch_PageSizeClampToMin(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.Search(context.Background(), &PatentSearchRequest{Query: "test", PageSize: 0})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["page_size"] != float64(1) {
		t.Errorf("page_size: want 1, got %v", body["page_size"])
	}
}

func TestPatentSearch_PageSizeClampToMax(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.Search(context.Background(), &PatentSearchRequest{Query: "test", PageSize: 500})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["page_size"] != float64(100) {
		t.Errorf("page_size: want 100, got %v", body["page_size"])
	}
}

func TestPatentSearch_WithAllFilters(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.Search(context.Background(), &PatentSearchRequest{
		Query:         "CDK4/6 inhibitor",
		QueryType:     "boolean",
		Jurisdictions: []string{"CN", "US", "EP"},
		DateRange:     &DateRange{Field: "filing_date", From: "2020-01-01", To: "2024-12-31"},
		Applicants:    []string{"Pfizer", "Novartis"},
		Inventors:     []string{"John Doe"},
		IPCCodes:      []string{"C07D", "A61K"},
		CPCCodes:      []string{"C07D401/04"},
		Status:        []string{"granted", "pending"},
		HasMolecule:   true,
		Page:          2,
		PageSize:      50,
		SortBy:        "filing_date",
		SortOrder:     "asc",
	})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)

	if body["query"] != "CDK4/6 inhibitor" {
		t.Errorf("query: want 'CDK4/6 inhibitor', got %v", body["query"])
	}
	if body["query_type"] != "boolean" {
		t.Errorf("query_type: want boolean, got %v", body["query_type"])
	}
	jurs, _ := body["jurisdictions"].([]interface{})
	if len(jurs) != 3 {
		t.Errorf("jurisdictions: want 3, got %d", len(jurs))
	}
	dr, _ := body["date_range"].(map[string]interface{})
	if dr["field"] != "filing_date" {
		t.Errorf("date_range.field: want filing_date, got %v", dr["field"])
	}
	if dr["from"] != "2020-01-01" {
		t.Errorf("date_range.from: want 2020-01-01, got %v", dr["from"])
	}
	if dr["to"] != "2024-12-31" {
		t.Errorf("date_range.to: want 2024-12-31, got %v", dr["to"])
	}
	apps, _ := body["applicants"].([]interface{})
	if len(apps) != 2 {
		t.Errorf("applicants: want 2, got %d", len(apps))
	}
	invs, _ := body["inventors"].([]interface{})
	if len(invs) != 1 {
		t.Errorf("inventors: want 1, got %d", len(invs))
	}
	ipcs, _ := body["ipc_codes"].([]interface{})
	if len(ipcs) != 2 {
		t.Errorf("ipc_codes: want 2, got %d", len(ipcs))
	}
	cpcs, _ := body["cpc_codes"].([]interface{})
	if len(cpcs) != 1 {
		t.Errorf("cpc_codes: want 1, got %d", len(cpcs))
	}
	sts, _ := body["status"].([]interface{})
	if len(sts) != 2 {
		t.Errorf("status: want 2, got %d", len(sts))
	}
	if body["has_molecule"] != true {
		t.Errorf("has_molecule: want true, got %v", body["has_molecule"])
	}
	if body["page"] != float64(2) {
		t.Errorf("page: want 2, got %v", body["page"])
	}
	if body["page_size"] != float64(50) {
		t.Errorf("page_size: want 50, got %v", body["page_size"])
	}
	if body["sort_by"] != "filing_date" {
		t.Errorf("sort_by: want filing_date, got %v", body["sort_by"])
	}
	if body["sort_order"] != "asc" {
		t.Errorf("sort_order: want asc, got %v", body["sort_order"])
	}
}

func TestPatentSearch_SemanticQueryType(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.Search(context.Background(), &PatentSearchRequest{Query: "EGFR mutation", QueryType: "semantic"})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["query_type"] != "semantic" {
		t.Errorf("query_type: want semantic, got %v", body["query_type"])
	}
}

func TestPatentSearch_BooleanQueryType(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.Search(context.Background(), &PatentSearchRequest{Query: "kinase AND inhibitor", QueryType: "boolean"})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["query_type"] != "boolean" {
		t.Errorf("query_type: want boolean, got %v", body["query_type"])
	}
}

func TestPatentSearch_ServerError(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 500, map[string]string{"code": "internal_error", "message": "boom"})
	})
	_, err := pc.Search(context.Background(), &PatentSearchRequest{Query: "test"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsServerError() {
		t.Error("IsServerError should be true")
	}
}

func TestPatentSearch_Unauthorized(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 401, map[string]string{"code": "unauthorized", "message": "bad key"})
	})
	_, err := pc.Search(context.Background(), &PatentSearchRequest{Query: "test"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsUnauthorized() {
		t.Error("IsUnauthorized should be true")
	}
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestPatentGet_Success(t *testing.T) {
	detail := samplePatentDetail()
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, patentDetailResp{Data: detail})
	})
	pd, err := pc.Get(context.Background(), "pat-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if pd.ID != "pat-001" {
		t.Errorf("ID: want pat-001, got %s", pd.ID)
	}
	if pd.PatentNumber != "CN115000001B" {
		t.Errorf("PatentNumber: want CN115000001B, got %s", pd.PatentNumber)
	}
	if len(pd.Claims) != 3 {
		t.Fatalf("Claims: want 3, got %d", len(pd.Claims))
	}
	if pd.Claims[0].Type != "independent" {
		t.Errorf("Claims[0].Type: want independent, got %s", pd.Claims[0].Type)
	}
	if pd.Claims[1].Type != "dependent" {
		t.Errorf("Claims[1].Type: want dependent, got %s", pd.Claims[1].Type)
	}
	if len(pd.Claims[1].DependsOn) != 1 || pd.Claims[1].DependsOn[0] != 1 {
		t.Errorf("Claims[1].DependsOn: want [1], got %v", pd.Claims[1].DependsOn)
	}
	if len(pd.Citations) != 2 {
		t.Errorf("Citations: want 2, got %d", len(pd.Citations))
	}
	if pd.Citations[0].Relevance != "X" {
		t.Errorf("Citations[0].Relevance: want X, got %s", pd.Citations[0].Relevance)
	}
	if len(pd.CitedBy) != 1 {
		t.Errorf("CitedBy: want 1, got %d", len(pd.CitedBy))
	}
	if len(pd.FamilyMembers) != 2 {
		t.Errorf("FamilyMembers: want 2, got %d", len(pd.FamilyMembers))
	}
	if pd.FamilyMembers[0].Jurisdiction != "US" {
		t.Errorf("FamilyMembers[0].Jurisdiction: want US, got %s", pd.FamilyMembers[0].Jurisdiction)
	}
	if len(pd.Molecules) != 1 {
		t.Errorf("Molecules: want 1, got %d", len(pd.Molecules))
	}
	if pd.Molecules[0].SMILES != "CCO" {
		t.Errorf("Molecules[0].SMILES: want CCO, got %s", pd.Molecules[0].SMILES)
	}
	if len(pd.LegalEvents) != 2 {
		t.Errorf("LegalEvents: want 2, got %d", len(pd.LegalEvents))
	}
	if pd.LegalEvents[1].Code != "GRANT" {
		t.Errorf("LegalEvents[1].Code: want GRANT, got %s", pd.LegalEvents[1].Code)
	}
	if len(pd.Drawings) != 1 {
		t.Errorf("Drawings: want 1, got %d", len(pd.Drawings))
	}
	if pd.FullTextURL != "https://example.com/CN115000001B.pdf" {
		t.Errorf("FullTextURL mismatch")
	}
	if pd.FamilyID != "fam-001" {
		t.Errorf("FamilyID: want fam-001, got %s", pd.FamilyID)
	}
	if pd.Description != "DETAILED DESCRIPTION..." {
		t.Errorf("Description mismatch")
	}
}

func TestPatentGet_EmptyID(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.Get(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGet_NotFound(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 404, map[string]string{"code": "not_found", "message": "not found"})
	})
	_, err := pc.Get(context.Background(), "pat-999")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Error("IsNotFound should be true")
	}
}

func TestPatentGet_URLPath(t *testing.T) {
	handler, getReq, _ := captureRequest(t, 200, patentDetailResp{Data: PatentDetail{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.Get(context.Background(), "pat-abc-123")
	req := getReq()
	if req.URL.Path != "/api/v1/patents/pat-abc-123" {
		t.Errorf("path: want /api/v1/patents/pat-abc-123, got %s", req.URL.Path)
	}
	if req.Method != http.MethodGet {
		t.Errorf("method: want GET, got %s", req.Method)
	}
}

// ---------------------------------------------------------------------------
// GetByNumber
// ---------------------------------------------------------------------------

func TestPatentGetByNumber_Success(t *testing.T) {
	handler, getReq, _ := captureRequest(t, 200, patentDetailResp{Data: samplePatentDetail()})
	pc := newTestPatentsClient(t, handler)
	pd, err := pc.GetByNumber(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetByNumber: %v", err)
	}
	if pd.PatentNumber != "CN115000001B" {
		t.Errorf("PatentNumber: want CN115000001B, got %s", pd.PatentNumber)
	}
	req := getReq()
	num := req.URL.Query().Get("number")
	if num != "CN115000001B" {
		t.Errorf("query param number: want CN115000001B, got %s", num)
	}
}

func TestPatentGetByNumber_EmptyNumber(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetByNumber(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGetByNumber_SpecialCharacters(t *testing.T) {
	handler, getReq, _ := captureRequest(t, 200, patentDetailResp{Data: PatentDetail{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.GetByNumber(context.Background(), "EP1234/B1")
	req := getReq()
	num := req.URL.Query().Get("number")
	if num != "EP1234/B1" {
		t.Errorf("query param number: want EP1234/B1, got %s", num)
	}
}

// ---------------------------------------------------------------------------
// GetFamily
// ---------------------------------------------------------------------------

func TestPatentGetFamily_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, familyMemberListResp{Data: []FamilyMember{
			{PatentNumber: "CN115000001B", Jurisdiction: "CN", Status: "granted", FilingDate: "2021-06-15", PublicationDate: "2022-01-01"},
			{PatentNumber: "US11000001B2", Jurisdiction: "US", Status: "granted", FilingDate: "2021-12-01", PublicationDate: "2022-06-01"},
			{PatentNumber: "EP4000001A1", Jurisdiction: "EP", Status: "pending", FilingDate: "2021-12-15", PublicationDate: "2022-07-01"},
			{PatentNumber: "JP2022-100001A", Jurisdiction: "JP", Status: "pending", FilingDate: "2022-01-10", PublicationDate: "2022-08-01"},
		}})
	})
	members, err := pc.GetFamily(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetFamily: %v", err)
	}
	if len(members) != 4 {
		t.Fatalf("want 4 members, got %d", len(members))
	}
	if members[0].Jurisdiction != "CN" {
		t.Errorf("members[0].Jurisdiction: want CN, got %s", members[0].Jurisdiction)
	}
	if members[3].Jurisdiction != "JP" {
		t.Errorf("members[3].Jurisdiction: want JP, got %s", members[3].Jurisdiction)
	}
}

func TestPatentGetFamily_EmptyNumber(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetFamily(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGetFamily_URLPath(t *testing.T) {
	handler, getReq, _ := captureRequest(t, 200, familyMemberListResp{Data: []FamilyMember{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.GetFamily(context.Background(), "CN115000001B")
	req := getReq()
	if req.URL.Path != "/api/v1/patents/CN115000001B/family" {
		t.Errorf("path: want /api/v1/patents/CN115000001B/family, got %s", req.URL.Path)
	}
}

// ---------------------------------------------------------------------------
// GetCitations
// ---------------------------------------------------------------------------

func TestPatentGetCitations_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, citationListResp{Data: []Citation{
			{PatentNumber: "US9000001B2", Title: "Prior art A", CitationType: "examiner", Relevance: "X"},
			{PatentNumber: "CN110000001A", Title: "Prior art B", CitationType: "applicant", Relevance: "Y"},
			{PatentNumber: "EP3000001B1", Title: "Prior art C", CitationType: "examiner", Relevance: "A"},
			{PatentNumber: "WO2019100001A1", Title: "Prior art D", CitationType: "third_party", Relevance: "D"},
			{PatentNumber: "JP6000001B2", Title: "Prior art E", CitationType: "examiner", Relevance: "X"},
		}})
	})
	citations, err := pc.GetCitations(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetCitations: %v", err)
	}
	if len(citations) != 5 {
		t.Fatalf("want 5 citations, got %d", len(citations))
	}
	if citations[0].CitationType != "examiner" {
		t.Errorf("citations[0].CitationType: want examiner, got %s", citations[0].CitationType)
	}
	if citations[0].Relevance != "X" {
		t.Errorf("citations[0].Relevance: want X, got %s", citations[0].Relevance)
	}
	if citations[3].CitationType != "third_party" {
		t.Errorf("citations[3].CitationType: want third_party, got %s", citations[3].CitationType)
	}
	if citations[3].Relevance != "D" {
		t.Errorf("citations[3].Relevance: want D, got %s", citations[3].Relevance)
	}
}

func TestPatentGetCitations_EmptyNumber(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetCitations(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGetCitations_EmptyResult(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, citationListResp{Data: []Citation{}})
	})
	citations, err := pc.GetCitations(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetCitations: %v", err)
	}
	if len(citations) != 0 {
		t.Errorf("want 0 citations, got %d", len(citations))
	}
}

// ---------------------------------------------------------------------------
// GetCitedBy
// ---------------------------------------------------------------------------

func TestPatentGetCitedBy_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, citationListResp{Data: []Citation{
			{PatentNumber: "CN116000001A", Title: "Citing A", CitationType: "examiner", Relevance: "A"},
			{PatentNumber: "US12000001A1", Title: "Citing B", CitationType: "applicant", Relevance: "Y"},
			{PatentNumber: "EP5000001A1", Title: "Citing C", CitationType: "examiner", Relevance: "X"},
		}})
	})
	cited, err := pc.GetCitedBy(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetCitedBy: %v", err)
	}
	if len(cited) != 3 {
		t.Fatalf("want 3, got %d", len(cited))
	}
	if cited[2].Relevance != "X" {
		t.Errorf("cited[2].Relevance: want X, got %s", cited[2].Relevance)
	}
}

func TestPatentGetCitedBy_EmptyNumber(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetCitedBy(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetMolecules
// ---------------------------------------------------------------------------

func TestPatentGetMolecules_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, moleculeBriefListResp{
			Data: []MoleculeBrief{
				{ID: "mol-001", SMILES: "CCO", MolecularFormula: "C2H6O", MolecularWeight: 46.07, Relevance: 0.9},
				{ID: "mol-002", SMILES: "CC(=O)O", MolecularFormula: "C2H4O2", MolecularWeight: 60.05, Relevance: 0.7},
			},
			Meta: &ResponseMeta{Total: 15},
		})
	})
	mols, total, err := pc.GetMolecules(context.Background(), "CN115000001B", 1, 20)
	if err != nil {
		t.Fatalf("GetMolecules: %v", err)
	}
	if len(mols) != 2 {
		t.Fatalf("want 2 molecules, got %d", len(mols))
	}
	if total != 15 {
		t.Errorf("total: want 15, got %d", total)
	}
	if mols[0].SMILES != "CCO" {
		t.Errorf("mols[0].SMILES: want CCO, got %s", mols[0].SMILES)
	}
	if mols[1].MolecularWeight != 60.05 {
		t.Errorf("mols[1].MolecularWeight: want 60.05, got %f", mols[1].MolecularWeight)
	}
}

func TestPatentGetMolecules_EmptyNumber(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, _, err := pc.GetMolecules(context.Background(), "", 1, 20)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGetMolecules_URLParams(t *testing.T) {
	handler, getReq, _ := captureRequest(t, 200, moleculeBriefListResp{Data: []MoleculeBrief{}})
	pc := newTestPatentsClient(t, handler)
	_, _, _ = pc.GetMolecules(context.Background(), "CN115000001B", 3, 50)
	req := getReq()
	if req.URL.Query().Get("page") != "3" {
		t.Errorf("page: want 3, got %s", req.URL.Query().Get("page"))
	}
	if req.URL.Query().Get("page_size") != "50" {
		t.Errorf("page_size: want 50, got %s", req.URL.Query().Get("page_size"))
	}
}

// ---------------------------------------------------------------------------
// EvaluateValue
// ---------------------------------------------------------------------------

func TestPatentEvaluateValue_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, patentValueResp{Data: PatentValueResult{
			Evaluations: []PatentValuation{
				{
					PatentNumber: "CN115000001B", OverallScore: 78.5,
					TechnicalScore: 82, LegalScore: 75, MarketScore: 70,
					CitationScore: 85, FamilyScore: 72, RemainingLife: 12.5,
					Factors: []ValuationFactor{
						{Name: "claim_breadth", Score: 80, Weight: 0.2, Description: "Broad independent claims"},
						{Name: "citation_impact", Score: 85, Weight: 0.15, Description: "High citation count"},
					},
				},
				{
					PatentNumber: "US11000001B2", OverallScore: 65.0,
					TechnicalScore: 70, LegalScore: 60, MarketScore: 65,
					CitationScore: 55, FamilyScore: 80, RemainingLife: 14.2,
					Factors: []ValuationFactor{
						{Name: "family_coverage", Score: 80, Weight: 0.1, Description: "Good geographic coverage"},
					},
				},
			},
		}})
	})
	res, err := pc.EvaluateValue(context.Background(), &PatentValueRequest{
		PatentNumbers: []string{"CN115000001B", "US11000001B2"},
	})
	if err != nil {
		t.Fatalf("EvaluateValue: %v", err)
	}
	if len(res.Evaluations) != 2 {
		t.Fatalf("Evaluations: want 2, got %d", len(res.Evaluations))
	}
	ev0 := res.Evaluations[0]
	if ev0.OverallScore != 78.5 {
		t.Errorf("OverallScore: want 78.5, got %f", ev0.OverallScore)
	}
	if ev0.TechnicalScore != 82 {
		t.Errorf("TechnicalScore: want 82, got %f", ev0.TechnicalScore)
	}
	if len(ev0.Factors) != 2 {
		t.Fatalf("Factors: want 2, got %d", len(ev0.Factors))
	}
	if ev0.Factors[0].Name != "claim_breadth" {
		t.Errorf("Factors[0].Name: want claim_breadth, got %s", ev0.Factors[0].Name)
	}
	if ev0.Factors[0].Weight != 0.2 {
		t.Errorf("Factors[0].Weight: want 0.2, got %f", ev0.Factors[0].Weight)
	}
	ev1 := res.Evaluations[1]
	if ev1.RemainingLife != 14.2 {
		t.Errorf("RemainingLife: want 14.2, got %f", ev1.RemainingLife)
	}
}

func TestPatentEvaluateValue_EmptyPatentNumbers(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.EvaluateValue(context.Background(), &PatentValueRequest{PatentNumbers: []string{}})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentEvaluateValue_NilPatentNumbers(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.EvaluateValue(context.Background(), &PatentValueRequest{PatentNumbers: nil})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentEvaluateValue_NilRequest(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.EvaluateValue(context.Background(), nil)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentEvaluateValue_ExceedLimit(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	nums := make([]string, 101)
	for i := range nums {
		nums[i] = fmt.Sprintf("P%d", i)
	}
	_, err := pc.EvaluateValue(context.Background(), &PatentValueRequest{PatentNumbers: nums})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentEvaluateValue_ExactLimit(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, patentValueResp{Data: PatentValueResult{Evaluations: []PatentValuation{}}})
	})
	nums := make([]string, 100)
	for i := range nums {
		nums[i] = fmt.Sprintf("P%d", i)
	}
	_, err := pc.EvaluateValue(context.Background(), &PatentValueRequest{PatentNumbers: nums})
	if err != nil {
		t.Fatalf("EvaluateValue with 100 patents should not error: %v", err)
	}
}

func TestPatentEvaluateValue_RequestBody(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, patentValueResp{Data: PatentValueResult{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.EvaluateValue(context.Background(), &PatentValueRequest{
		PatentNumbers: []string{"CN115000001B", "US11000001B2", "EP4000001A1"},
	})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	pns, ok := body["patent_numbers"].([]interface{})
	if !ok {
		t.Fatalf("patent_numbers: want array, got %T", body["patent_numbers"])
	}
	if len(pns) != 3 {
		t.Errorf("patent_numbers: want 3, got %d", len(pns))
	}
	if pns[0] != "CN115000001B" {
		t.Errorf("patent_numbers[0]: want CN115000001B, got %v", pns[0])
	}
	if pns[2] != "EP4000001A1" {
		t.Errorf("patent_numbers[2]: want EP4000001A1, got %v", pns[2])
	}
}

// ---------------------------------------------------------------------------
// GetLandscape
// ---------------------------------------------------------------------------

func TestPatentGetLandscape_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, patentLandscapeResp{Data: PatentLandscape{
			TotalPatents: 5000,
			YearlyTrend: []YearCount{
				{Year: 2020, Count: 800}, {Year: 2021, Count: 1000},
				{Year: 2022, Count: 1200}, {Year: 2023, Count: 1100}, {Year: 2024, Count: 900},
			},
			TopApplicants: []ApplicantStats{
				{Name: "Pfizer", PatentCount: 320, FirstFiling: "2015-03-01", LastFiling: "2024-06-01", TopIPCCodes: []string{"C07D", "A61K"}},
				{Name: "Novartis", PatentCount: 280, FirstFiling: "2016-01-15", LastFiling: "2024-05-20", TopIPCCodes: []string{"A61K", "A61P"}},
			},
			TopIPCCodes: []IPCStats{
				{Code: "C07D", Description: "Heterocyclic compounds", Count: 2500},
				{Code: "A61K", Description: "Preparations for medical purposes", Count: 1800},
			},
			JurisdictionDistribution: []FacetItem{{Value: "CN", Count: 2000}, {Value: "US", Count: 1500}},
			StatusDistribution:       []FacetItem{{Value: "granted", Count: 3000}, {Value: "pending", Count: 2000}},
			TechnologyClusters: []TechnologyCluster{
				{ID: "tc-1", Label: "CDK4/6 Inhibitors", PatentCount: 450, Keywords: []string{"CDK4", "CDK6", "palbociclib"}, CentralPatent: "US9000001B2"},
				{ID: "tc-2", Label: "EGFR Inhibitors", PatentCount: 380, Keywords: []string{"EGFR", "erlotinib", "gefitinib"}, CentralPatent: "US8000001B2"},
			},
		}})
	})
	ls, err := pc.GetLandscape(context.Background(), &PatentLandscapeRequest{Query: "kinase inhibitor"})
	if err != nil {
		t.Fatalf("GetLandscape: %v", err)
	}
	if ls.TotalPatents != 5000 {
		t.Errorf("TotalPatents: want 5000, got %d", ls.TotalPatents)
	}
	if len(ls.YearlyTrend) != 5 {
		t.Errorf("YearlyTrend: want 5, got %d", len(ls.YearlyTrend))
	}
	if ls.YearlyTrend[0].Year != 2020 || ls.YearlyTrend[0].Count != 800 {
		t.Errorf("YearlyTrend[0]: want 2020/800, got %d/%d", ls.YearlyTrend[0].Year, ls.YearlyTrend[0].Count)
	}
	if len(ls.TopApplicants) != 2 {
		t.Errorf("TopApplicants: want 2, got %d", len(ls.TopApplicants))
	}
	if ls.TopApplicants[0].Name != "Pfizer" {
		t.Errorf("TopApplicants[0].Name: want Pfizer, got %s", ls.TopApplicants[0].Name)
	}
	if len(ls.TopApplicants[0].TopIPCCodes) != 2 {
		t.Errorf("TopApplicants[0].TopIPCCodes: want 2, got %d", len(ls.TopApplicants[0].TopIPCCodes))
	}
	if len(ls.TopIPCCodes) != 2 {
		t.Errorf("TopIPCCodes: want 2, got %d", len(ls.TopIPCCodes))
	}
	if ls.TopIPCCodes[0].Description != "Heterocyclic compounds" {
		t.Errorf("TopIPCCodes[0].Description mismatch")
	}
	if len(ls.JurisdictionDistribution) != 2 {
		t.Errorf("JurisdictionDistribution: want 2, got %d", len(ls.JurisdictionDistribution))
	}
	if len(ls.TechnologyClusters) != 2 {
		t.Errorf("TechnologyClusters: want 2, got %d", len(ls.TechnologyClusters))
	}
	if ls.TechnologyClusters[0].CentralPatent != "US9000001B2" {
		t.Errorf("TechnologyClusters[0].CentralPatent: want US9000001B2, got %s", ls.TechnologyClusters[0].CentralPatent)
	}
	if len(ls.TechnologyClusters[0].Keywords) != 3 {
		t.Errorf("TechnologyClusters[0].Keywords: want 3, got %d", len(ls.TechnologyClusters[0].Keywords))
	}
}

func TestPatentGetLandscape_EmptyQuery(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetLandscape(context.Background(), &PatentLandscapeRequest{Query: ""})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGetLandscape_NilRequest(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetLandscape(context.Background(), nil)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGetLandscape_DefaultValues(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, patentLandscapeResp{Data: PatentLandscape{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.GetLandscape(context.Background(), &PatentLandscapeRequest{Query: "test"})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["top_applicants"] != float64(20) {
		t.Errorf("top_applicants: want 20, got %v", body["top_applicants"])
	}
	if body["top_ipc_codes"] != float64(20) {
		t.Errorf("top_ipc_codes: want 20, got %v", body["top_ipc_codes"])
	}
}

func TestPatentGetLandscape_CustomTopN(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, patentLandscapeResp{Data: PatentLandscape{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.GetLandscape(context.Background(), &PatentLandscapeRequest{
		Query: "test", TopApplicants: 50, TopIPCCodes: 30,
	})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["top_applicants"] != float64(50) {
		t.Errorf("top_applicants: want 50, got %v", body["top_applicants"])
	}
	if body["top_ipc_codes"] != float64(30) {
		t.Errorf("top_ipc_codes: want 30, got %v", body["top_ipc_codes"])
	}
}

func TestPatentGetLandscape_ZeroTopN(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, patentLandscapeResp{Data: PatentLandscape{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.GetLandscape(context.Background(), &PatentLandscapeRequest{
		Query: "test", TopApplicants: 0, TopIPCCodes: 0,
	})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["top_applicants"] != float64(20) {
		t.Errorf("top_applicants: want 20 (default), got %v", body["top_applicants"])
	}
	if body["top_ipc_codes"] != float64(20) {
		t.Errorf("top_ipc_codes: want 20 (default), got %v", body["top_ipc_codes"])
	}
}

func TestPatentGetLandscape_NegativeTopN(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, patentLandscapeResp{Data: PatentLandscape{}})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.GetLandscape(context.Background(), &PatentLandscapeRequest{
		Query: "test", TopApplicants: -5, TopIPCCodes: -10,
	})
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["top_applicants"] != float64(20) {
		t.Errorf("top_applicants: want 20 (default), got %v", body["top_applicants"])
	}
	if body["top_ipc_codes"] != float64(20) {
		t.Errorf("top_ipc_codes: want 20 (default), got %v", body["top_ipc_codes"])
	}
}

// ---------------------------------------------------------------------------
// GetLegalEvents
// ---------------------------------------------------------------------------

func TestPatentGetLegalEvents_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, legalEventListResp{Data: []LegalEvent{
			{Date: "2021-06-15", Code: "FILING", Description: "Application filed", Source: "CNIPA_API"},
			{Date: "2022-01-01", Code: "PUB", Description: "Application published", Source: "CNIPA_API"},
			{Date: "2022-07-20", Code: "EXAM_REQ", Description: "Substantive examination requested", Source: "CNIPA_API"},
			{Date: "2023-05-10", Code: "OA1", Description: "First office action", Source: "CNIPA_API"},
			{Date: "2023-11-15", Code: "OA_RESP", Description: "Response to office action", Source: "CNIPA_API"},
			{Date: "2024-03-01", Code: "GRANT", Description: "Patent granted", Source: "CNIPA_API"},
		}})
	})
	events, err := pc.GetLegalEvents(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetLegalEvents: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("want 6 events, got %d", len(events))
	}
	if events[0].Code != "FILING" {
		t.Errorf("events[0].Code: want FILING, got %s", events[0].Code)
	}
	if events[5].Code != "GRANT" {
		t.Errorf("events[5].Code: want GRANT, got %s", events[5].Code)
	}
	if events[3].Source != "CNIPA_API" {
		t.Errorf("events[3].Source: want CNIPA_API, got %s", events[3].Source)
	}
}

func TestPatentGetLegalEvents_EmptyNumber(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetLegalEvents(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetClaims
// ---------------------------------------------------------------------------

func TestPatentGetClaims_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		claims := make([]Claim, 10)
		for i := range claims {
			claims[i] = Claim{Number: i + 1, Text: fmt.Sprintf("Claim %d text", i+1)}
			if i == 0 || i == 4 || i == 7 {
				claims[i].Type = "independent"
				claims[i].DependsOn = []int{}
			} else {
				claims[i].Type = "dependent"
				// dependent claims reference the nearest preceding independent claim
				switch {
				case i < 4:
					claims[i].DependsOn = []int{1}
				case i < 7:
					claims[i].DependsOn = []int{5}
				default:
					claims[i].DependsOn = []int{8}
				}
			}
		}
		ptWriteJSON(t, w, 200, claimListResp{Data: claims})
	})
	claims, err := pc.GetClaims(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetClaims: %v", err)
	}
	if len(claims) != 10 {
		t.Fatalf("want 10 claims, got %d", len(claims))
	}
	if claims[0].Type != "independent" {
		t.Errorf("claims[0].Type: want independent, got %s", claims[0].Type)
	}
	if claims[1].Type != "dependent" {
		t.Errorf("claims[1].Type: want dependent, got %s", claims[1].Type)
	}
	if len(claims[1].DependsOn) != 1 || claims[1].DependsOn[0] != 1 {
		t.Errorf("claims[1].DependsOn: want [1], got %v", claims[1].DependsOn)
	}
}

func TestPatentGetClaims_EmptyNumber(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.GetClaims(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentGetClaims_IndependentClaim(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, claimListResp{Data: []Claim{
			{Number: 1, Text: "A compound...", Type: "independent", DependsOn: []int{}},
		}})
	})
	claims, err := pc.GetClaims(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetClaims: %v", err)
	}
	if len(claims) != 1 {
		t.Fatalf("want 1 claim, got %d", len(claims))
	}
	if claims[0].Type != "independent" {
		t.Errorf("Type: want independent, got %s", claims[0].Type)
	}
	if len(claims[0].DependsOn) != 0 {
		t.Errorf("DependsOn: want empty, got %v", claims[0].DependsOn)
	}
}

func TestPatentGetClaims_DependentClaim(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, claimListResp{Data: []Claim{
			{Number: 1, Text: "A compound...", Type: "independent", DependsOn: []int{}},
			{Number: 2, Text: "The compound of claim 1...", Type: "dependent", DependsOn: []int{1}},
			{Number: 3, Text: "The compound of claims 1 or 2...", Type: "dependent", DependsOn: []int{1, 2}},
		}})
	})
	claims, err := pc.GetClaims(context.Background(), "CN115000001B")
	if err != nil {
		t.Fatalf("GetClaims: %v", err)
	}
	if claims[2].Type != "dependent" {
		t.Errorf("claims[2].Type: want dependent, got %s", claims[2].Type)
	}
	if len(claims[2].DependsOn) != 2 {
		t.Fatalf("claims[2].DependsOn: want 2 elements, got %d", len(claims[2].DependsOn))
	}
	if claims[2].DependsOn[0] != 1 || claims[2].DependsOn[1] != 2 {
		t.Errorf("claims[2].DependsOn: want [1,2], got %v", claims[2].DependsOn)
	}
}

// ---------------------------------------------------------------------------
// SemanticSearch
// ---------------------------------------------------------------------------

func TestPatentSemanticSearch_Success(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := ptReadBody(t, r)
		if body["query_type"] != "semantic" {
			t.Errorf("query_type: want semantic, got %v", body["query_type"])
		}
		ptWriteJSON(t, w, 200, PatentSearchResult{
			Patents: []Patent{samplePatent()}, Total: 1, Page: 1, PageSize: 20, HasMore: false,
		})
	})
	res, err := pc.SemanticSearch(context.Background(), "EGFR tyrosine kinase inhibitor with improved selectivity", 1, 20)
	if err != nil {
		t.Fatalf("SemanticSearch: %v", err)
	}
	if len(res.Patents) != 1 {
		t.Errorf("Patents: want 1, got %d", len(res.Patents))
	}
}

func TestPatentSemanticSearch_EmptyText(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.SemanticSearch(context.Background(), "", 1, 20)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentSemanticSearch_RequestBody(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.SemanticSearch(context.Background(), "novel compound for treating cancer", 3, 50)
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["query"] != "novel compound for treating cancer" {
		t.Errorf("query: want 'novel compound for treating cancer', got %v", body["query"])
	}
	if body["query_type"] != "semantic" {
		t.Errorf("query_type: want semantic, got %v", body["query_type"])
	}
	if body["page"] != float64(3) {
		t.Errorf("page: want 3, got %v", body["page"])
	}
	if body["page_size"] != float64(50) {
		t.Errorf("page_size: want 50, got %v", body["page_size"])
	}
	if body["sort_order"] != "desc" {
		t.Errorf("sort_order: want desc, got %v", body["sort_order"])
	}
}

func TestPatentSemanticSearch_DefaultPageSize(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, _ = pc.SemanticSearch(context.Background(), "test query", 0, 0)
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	if body["page"] != float64(1) {
		t.Errorf("page: want 1 (default), got %v", body["page"])
	}
	if body["page_size"] != float64(20) {
		t.Errorf("page_size: want 20 (default), got %v", body["page_size"])
	}
}

// ---------------------------------------------------------------------------
// DateRange validation
// ---------------------------------------------------------------------------

func TestPatentSearch_DateRange_Valid(t *testing.T) {
	handler, _, getBody := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, err := pc.Search(context.Background(), &PatentSearchRequest{
		Query:     "test",
		DateRange: &DateRange{Field: "filing_date", From: "2020-01-01", To: "2024-01-01"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	var body map[string]interface{}
	json.Unmarshal(getBody(), &body)
	dr, _ := body["date_range"].(map[string]interface{})
	if dr["from"] != "2020-01-01" {
		t.Errorf("from: want 2020-01-01, got %v", dr["from"])
	}
	if dr["to"] != "2024-01-01" {
		t.Errorf("to: want 2024-01-01, got %v", dr["to"])
	}
}

func TestPatentSearch_DateRange_FromAfterTo(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.Search(context.Background(), &PatentSearchRequest{
		Query:     "test",
		DateRange: &DateRange{Field: "filing_date", From: "2024-01-01", To: "2020-01-01"},
	})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPatentSearch_DateRange_OnlyFrom(t *testing.T) {
	handler, _, _ := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, err := pc.Search(context.Background(), &PatentSearchRequest{
		Query:     "test",
		DateRange: &DateRange{Field: "filing_date", From: "2020-01-01"},
	})
	if err != nil {
		t.Fatalf("Search with only From should not error: %v", err)
	}
}

func TestPatentSearch_DateRange_OnlyTo(t *testing.T) {
	handler, _, _ := captureRequest(t, 200, PatentSearchResult{})
	pc := newTestPatentsClient(t, handler)
	_, err := pc.Search(context.Background(), &PatentSearchRequest{
		Query:     "test",
		DateRange: &DateRange{Field: "filing_date", To: "2024-01-01"},
	})
	if err != nil {
		t.Fatalf("Search with only To should not error: %v", err)
	}
}

func TestPatentSearch_DateRange_InvalidOrder(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := pc.Search(context.Background(), &PatentSearchRequest{
		Query:     "test",
		DateRange: &DateRange{Field: "filing_date", From: "2025-01-01", To: "2024-01-01"},
	})
	// Expect invalid argument error
	if err == nil || !strings.Contains(err.Error(), "must not be after") {
		t.Errorf("expected date range error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

func TestPatentsClient_ContextCancellation(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		ptWriteJSON(t, w, 200, PatentSearchResult{})
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pc.Search(ctx, &PatentSearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	}
}

func TestPatentsClient_ContextTimeout(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		ptWriteJSON(t, w, 200, PatentSearchResult{})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := pc.Search(ctx, &PatentSearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error from timed-out context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// URL path construction
// ---------------------------------------------------------------------------

func TestPatentsClient_URLPathConstruction(t *testing.T) {
	var mu sync.Mutex
	paths := make(map[string]bool)

	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		mu.Lock()
		paths[key] = true
		mu.Unlock()

		switch {
		case r.URL.Path == "/api/v1/patents/search" && r.Method == http.MethodPost:
			ptWriteJSON(t, w, 200, PatentSearchResult{})
		case r.URL.Path == "/api/v1/patents/pat-url-test" && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, patentDetailResp{Data: PatentDetail{}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/patents/by-number") && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, patentDetailResp{Data: PatentDetail{}})
		case r.URL.Path == "/api/v1/patents/CN115000001B/family" && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, familyMemberListResp{Data: []FamilyMember{}})
		case r.URL.Path == "/api/v1/patents/CN115000001B/citations" && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, citationListResp{Data: []Citation{}})
		case r.URL.Path == "/api/v1/patents/CN115000001B/cited-by" && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, citationListResp{Data: []Citation{}})
		case r.URL.Path == "/api/v1/patents/CN115000001B/molecules" && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, moleculeBriefListResp{Data: []MoleculeBrief{}})
		case r.URL.Path == "/api/v1/patents/evaluate" && r.Method == http.MethodPost:
			ptWriteJSON(t, w, 200, patentValueResp{Data: PatentValueResult{}})
		case r.URL.Path == "/api/v1/patents/landscape" && r.Method == http.MethodPost:
			ptWriteJSON(t, w, 200, patentLandscapeResp{Data: PatentLandscape{}})
		case r.URL.Path == "/api/v1/patents/CN115000001B/legal-events" && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, legalEventListResp{Data: []LegalEvent{}})
		case r.URL.Path == "/api/v1/patents/CN115000001B/claims" && r.Method == http.MethodGet:
			ptWriteJSON(t, w, 200, claimListResp{Data: []Claim{}})
		case r.URL.Path == "/api/v1/patents/semantic-search" && r.Method == http.MethodPost:
			ptWriteJSON(t, w, 200, PatentSearchResult{})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	})

	ctx := context.Background()

	_, _ = pc.Search(ctx, &PatentSearchRequest{Query: "test"})
	_, _ = pc.Get(ctx, "pat-url-test")
	_, _ = pc.GetByNumber(ctx, "CN115000001B")
	_, _ = pc.GetFamily(ctx, "CN115000001B")
	_, _ = pc.GetCitations(ctx, "CN115000001B")
	_, _ = pc.GetCitedBy(ctx, "CN115000001B")
	_, _, _ = pc.GetMolecules(ctx, "CN115000001B", 1, 20)
	_, _ = pc.EvaluateValue(ctx, &PatentValueRequest{PatentNumbers: []string{"CN115000001B"}})
	_, _ = pc.GetLandscape(ctx, &PatentLandscapeRequest{Query: "test"})
	_, _ = pc.GetLegalEvents(ctx, "CN115000001B")
	_, _ = pc.GetClaims(ctx, "CN115000001B")
	_, _ = pc.SemanticSearch(ctx, "test", 1, 20)

	mu.Lock()
	defer mu.Unlock()

	expected := []string{
		"POST /api/v1/patents/search",
		"GET /api/v1/patents/pat-url-test",
		"GET /api/v1/patents/by-number",
		"GET /api/v1/patents/CN115000001B/family",
		"GET /api/v1/patents/CN115000001B/citations",
		"GET /api/v1/patents/CN115000001B/cited-by",
		"GET /api/v1/patents/CN115000001B/molecules",
		"POST /api/v1/patents/evaluate",
		"POST /api/v1/patents/landscape",
		"GET /api/v1/patents/CN115000001B/legal-events",
		"GET /api/v1/patents/CN115000001B/claims",
		"POST /api/v1/patents/semantic-search",
	}
	for _, e := range expected {
		if !paths[e] {
			t.Errorf("expected path %q was not hit", e)
		}
	}
}

// ---------------------------------------------------------------------------
// HTTP methods
// ---------------------------------------------------------------------------

func TestPatentsClient_HTTPMethods(t *testing.T) {
	var mu sync.Mutex
	methods := make(map[string]string)

	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods[r.URL.Path] = r.Method
		mu.Unlock()

		switch {
		case r.URL.Path == "/api/v1/patents/search":
			ptWriteJSON(t, w, 200, PatentSearchResult{})
		case r.URL.Path == "/api/v1/patents/evaluate":
			ptWriteJSON(t, w, 200, patentValueResp{Data: PatentValueResult{}})
		case r.URL.Path == "/api/v1/patents/landscape":
			ptWriteJSON(t, w, 200, patentLandscapeResp{Data: PatentLandscape{}})
		case r.URL.Path == "/api/v1/patents/semantic-search":
			ptWriteJSON(t, w, 200, PatentSearchResult{})
		case r.URL.Path == "/api/v1/patents/method-test":
			ptWriteJSON(t, w, 200, patentDetailResp{Data: PatentDetail{}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/patents/by-number"):
			ptWriteJSON(t, w, 200, patentDetailResp{Data: PatentDetail{}})
		case strings.HasSuffix(r.URL.Path, "/family"):
			ptWriteJSON(t, w, 200, familyMemberListResp{Data: []FamilyMember{}})
		case strings.HasSuffix(r.URL.Path, "/citations"):
			ptWriteJSON(t, w, 200, citationListResp{Data: []Citation{}})
		case strings.HasSuffix(r.URL.Path, "/cited-by"):
			ptWriteJSON(t, w, 200, citationListResp{Data: []Citation{}})
		case strings.HasSuffix(r.URL.Path, "/molecules"):
			ptWriteJSON(t, w, 200, moleculeBriefListResp{Data: []MoleculeBrief{}})
		case strings.HasSuffix(r.URL.Path, "/legal-events"):
			ptWriteJSON(t, w, 200, legalEventListResp{Data: []LegalEvent{}})
		case strings.HasSuffix(r.URL.Path, "/claims"):
			ptWriteJSON(t, w, 200, claimListResp{Data: []Claim{}})
		default:
			w.WriteHeader(404)
		}
	})

	ctx := context.Background()

	// POST methods
	_, _ = pc.Search(ctx, &PatentSearchRequest{Query: "test"})
	_, _ = pc.EvaluateValue(ctx, &PatentValueRequest{PatentNumbers: []string{"P1"}})
	_, _ = pc.GetLandscape(ctx, &PatentLandscapeRequest{Query: "test"})
	_, _ = pc.SemanticSearch(ctx, "test", 1, 20)

	// GET methods
	_, _ = pc.Get(ctx, "method-test")
	_, _ = pc.GetByNumber(ctx, "CN115000001B")
	_, _ = pc.GetFamily(ctx, "CN115000001B")
	_, _ = pc.GetCitations(ctx, "CN115000001B")
	_, _ = pc.GetCitedBy(ctx, "CN115000001B")
	_, _, _ = pc.GetMolecules(ctx, "CN115000001B", 1, 20)
	_, _ = pc.GetLegalEvents(ctx, "CN115000001B")
	_, _ = pc.GetClaims(ctx, "CN115000001B")

	mu.Lock()
	defer mu.Unlock()

	postPaths := []string{
		"/api/v1/patents/search",
		"/api/v1/patents/evaluate",
		"/api/v1/patents/landscape",
		"/api/v1/patents/semantic-search",
	}
	for _, p := range postPaths {
		if methods[p] != http.MethodPost {
			t.Errorf("%s: want POST, got %s", p, methods[p])
		}
	}

	getPaths := []string{
		"/api/v1/patents/method-test",
		"/api/v1/patents/by-number",
		"/api/v1/patents/CN115000001B/family",
		"/api/v1/patents/CN115000001B/citations",
		"/api/v1/patents/CN115000001B/cited-by",
		"/api/v1/patents/CN115000001B/molecules",
		"/api/v1/patents/CN115000001B/legal-events",
		"/api/v1/patents/CN115000001B/claims",
	}
	for _, p := range getPaths {
		if methods[p] != http.MethodGet {
			t.Errorf("%s: want GET, got %s", p, methods[p])
		}
	}
}

// ---------------------------------------------------------------------------
// Suppress unused import warnings — these are used above.
// ---------------------------------------------------------------------------

var (
	_ = fmt.Sprintf
	_ = io.ReadAll
	_ = strings.Contains
	_ = sync.Mutex{}
	_ = time.Sleep
)

//Personal.AI order the ending
