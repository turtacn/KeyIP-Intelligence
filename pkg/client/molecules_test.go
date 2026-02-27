// Phase 13 - SDK Molecules Sub-Client Test (297/349)
// File: pkg/client/molecules_test.go
// Comprehensive unit tests for MoleculesClient.

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
	"testing"

	kerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestMoleculesClient(t *testing.T, handler http.HandlerFunc) *MoleculesClient {
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
	return c.Molecules()
}

func readBody(t *testing.T, r *http.Request) map[string]interface{} {
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

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func sampleMolecule() Molecule {
	return Molecule{
		ID:                "mol-001",
		SMILES:            "CC(=O)Oc1ccccc1C(=O)O",
		CanonicalSMILES:   "CC(=O)Oc1ccccc1C(=O)O",
		InChI:             "InChI=1S/C9H8O4/c1-6(10)13-8-5-3-2-4-7(8)9(11)12/h2-5H,1H3,(H,11,12)",
		InChIKey:          "BSYNRYMUTXBXSQ-UHFFFAOYSA-N",
		MolecularFormula:  "C9H8O4",
		MolecularWeight:   180.16,
		ExactMass:         180.0423,
		LogP:              1.2,
		TPSA:              63.6,
		HBondDonors:       1,
		HBondAcceptors:    4,
		RotatableBonds:    3,
		RingCount:         1,
		AromaticRingCount: 1,
		HeavyAtomCount:    13,
		Lipinski: &LipinskiRule{
			MolecularWeight: 180.16,
			LogP:            1.2,
			HBondDonors:     1,
			HBondAcceptors:  4,
			Violations:      0,
			Pass:            true,
		},
		PatentCount:     5,
		FirstPatentDate: "1899-02-27",
		Sources:         []string{"PubChem", "ChEMBL"},
		CreatedAt:       "2024-01-01T00:00:00Z",
		UpdatedAt:       "2024-06-01T00:00:00Z",
	}
}

func sampleDetail() MoleculeDetail {
	return MoleculeDetail{
		Molecule:         sampleMolecule(),
		Synonyms:         []string{"Aspirin", "Acetylsalicylic acid"},
		CASNumber:        "50-78-2",
		Scaffold:         "c1ccccc1",
		FunctionalGroups: []string{"carboxyl", "ester"},
		Stereochemistry:  "none",
		Patents: []PatentBrief{
			{PatentNumber: "US1234567", Title: "Aspirin formulation", Applicant: "Bayer", FilingDate: "1899-02-27", Relevance: 0.95},
		},
		Activities: []BioActivity{
			{Target: "COX-2", ActivityType: "IC50", Value: 50.0, Unit: "uM", Source: "ChEMBL"},
		},
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/molecules/search" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		writeJSON(t, w, 200, MoleculeSearchResult{
			Molecules:  []Molecule{sampleMolecule(), sampleMolecule()},
			Total:      2,
			Page:       1,
			PageSize:   20,
			HasMore:    false,
			SearchTime: 0.042,
		})
	})
	res, err := mc.Search(context.Background(), &MoleculeSearchRequest{Query: "aspirin"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Molecules) != 2 {
		t.Errorf("len(Molecules): want 2, got %d", len(res.Molecules))
	}
	if res.Molecules[0].Lipinski == nil || !res.Molecules[0].Lipinski.Pass {
		t.Error("Lipinski should be present and Pass=true")
	}
	if res.SearchTime != 0.042 {
		t.Errorf("SearchTime: want 0.042, got %f", res.SearchTime)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.Search(context.Background(), &MoleculeSearchRequest{Query: ""})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSearch_NilRequest(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.Search(context.Background(), nil)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSearch_DefaultValues(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		checks := map[string]interface{}{
			"search_mode": "exact",
			"fingerprint": "morgan",
			"sort_order":  "desc",
			"similarity":  0.8,
			"radius":      float64(2),
			"page":        float64(1),
			"page_size":   float64(20),
		}
		for k, want := range checks {
			if body[k] != want {
				t.Errorf("%s: want %v, got %v", k, want, body[k])
			}
		}
		writeJSON(t, w, 200, MoleculeSearchResult{})
	})
	_, _ = mc.Search(context.Background(), &MoleculeSearchRequest{Query: "test"})
}

func TestSearch_SimilarityOutOfRange(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.Search(context.Background(), &MoleculeSearchRequest{Query: "test", Similarity: 1.5})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSearch_SimilarityNegative(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.Search(context.Background(), &MoleculeSearchRequest{Query: "test", Similarity: -0.1})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestSearch_PageSizeClampToMin(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		// When PageSize <= 0, it defaults to DefaultPageSize (20)
		if body["page_size"] != float64(DefaultPageSize) {
			t.Errorf("page_size: want %d, got %v", DefaultPageSize, body["page_size"])
		}
		writeJSON(t, w, 200, MoleculeSearchResult{})
	})
	_, _ = mc.Search(context.Background(), &MoleculeSearchRequest{Query: "test", PageSize: -5})
}

func TestSearch_PageSizeClampToMax(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if body["page_size"] != float64(100) {
			t.Errorf("page_size: want 100, got %v", body["page_size"])
		}
		writeJSON(t, w, 200, MoleculeSearchResult{})
	})
	_, _ = mc.Search(context.Background(), &MoleculeSearchRequest{Query: "test", PageSize: 200})
}

func TestSearch_SubstructureMode(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if body["search_mode"] != "substructure" {
			t.Errorf("search_mode: want substructure, got %v", body["search_mode"])
		}
		if body["query_type"] != "smarts" {
			t.Errorf("query_type: want smarts, got %v", body["query_type"])
		}
		writeJSON(t, w, 200, MoleculeSearchResult{})
	})
	_, _ = mc.Search(context.Background(), &MoleculeSearchRequest{
		Query: "[OH]c1ccccc1", QueryType: "smarts", SearchMode: "substructure",
	})
}

func TestSearch_SimilarityMode(t *testing.T) {
	mol := sampleMolecule()
	mol.Similarity = 0.92
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 200, MoleculeSearchResult{Molecules: []Molecule{mol}})
	})
	res, err := mc.Search(context.Background(), &MoleculeSearchRequest{
		Query: "CCO", SearchMode: "similarity", Similarity: 0.7,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Molecules[0].Similarity != 0.92 {
		t.Errorf("Similarity: want 0.92, got %f", res.Molecules[0].Similarity)
	}
}

func TestSearch_ServerError(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 500, map[string]string{"code": "internal_error", "message": "boom"})
	})
	_, err := mc.Search(context.Background(), &MoleculeSearchRequest{Query: "test"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsServerError() {
		t.Error("IsServerError should be true")
	}
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestGet_Success(t *testing.T) {
	detail := sampleDetail()
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/mol-001") {
			t.Errorf("path suffix: want /mol-001, got %s", r.URL.Path)
		}
		writeJSON(t, w, 200, moleculeDetailResp{Data: detail})
	})
	got, err := mc.Get(context.Background(), "mol-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != "mol-001" {
		t.Errorf("ID: want mol-001, got %s", got.ID)
	}
	if len(got.Patents) != 1 || got.Patents[0].PatentNumber != "US1234567" {
		t.Errorf("Patents mismatch")
	}
	if len(got.Activities) != 1 || got.Activities[0].Target != "COX-2" {
		t.Errorf("Activities mismatch")
	}
	if len(got.Synonyms) != 2 {
		t.Errorf("Synonyms: want 2, got %d", len(got.Synonyms))
	}
	if got.CASNumber != "50-78-2" {
		t.Errorf("CASNumber: want 50-78-2, got %s", got.CASNumber)
	}
}

func TestGet_EmptyID(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.Get(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestGet_NotFound(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 404, map[string]string{"code": "not_found", "message": "not found"})
	})
	_, err := mc.Get(context.Background(), "mol-999")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Error("IsNotFound should be true")
	}
}

// ---------------------------------------------------------------------------
// GetBySMILES
// ---------------------------------------------------------------------------

func TestGetBySMILES_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		smiles := r.URL.Query().Get("smiles")
		if smiles == "" {
			t.Error("smiles query param is empty")
		}
		writeJSON(t, w, 200, moleculeDetailResp{Data: sampleDetail()})
	})
	got, err := mc.GetBySMILES(context.Background(), "CC(=O)Oc1ccccc1C(=O)O")
	if err != nil {
		t.Fatalf("GetBySMILES: %v", err)
	}
	if got.ID != "mol-001" {
		t.Errorf("ID: want mol-001, got %s", got.ID)
	}
}

func TestGetBySMILES_EmptySMILES(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.GetBySMILES(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestGetBySMILES_SpecialCharacters(t *testing.T) {
	specialSMILES := "C#C/C=C\\[C@@H](O)[NH3+]"
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("smiles")
		if got != specialSMILES {
			t.Errorf("smiles decoded: want %q, got %q", specialSMILES, got)
		}
		writeJSON(t, w, 200, moleculeDetailResp{Data: sampleDetail()})
	})
	_, err := mc.GetBySMILES(context.Background(), specialSMILES)
	if err != nil {
		t.Fatalf("GetBySMILES: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetByInChIKey
// ---------------------------------------------------------------------------

func TestGetByInChIKey_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("inchikey")
		if key != "BSYNRYMUTXBXSQ-UHFFFAOYSA-N" {
			t.Errorf("inchikey: want BSYNRYMUTXBXSQ-UHFFFAOYSA-N, got %s", key)
		}
		writeJSON(t, w, 200, moleculeDetailResp{Data: sampleDetail()})
	})
	got, err := mc.GetByInChIKey(context.Background(), "BSYNRYMUTXBXSQ-UHFFFAOYSA-N")
	if err != nil {
		t.Fatalf("GetByInChIKey: %v", err)
	}
	if got.InChIKey != "BSYNRYMUTXBXSQ-UHFFFAOYSA-N" {
		t.Errorf("InChIKey mismatch")
	}
}

func TestGetByInChIKey_EmptyKey(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.GetByInChIKey(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestGetByInChIKey_InvalidFormat(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.GetByInChIKey(context.Background(), "invalid-key")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestGetByInChIKey_LowercaseRejected(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.GetByInChIKey(context.Background(), "bsynrymutxbxsq-uhfffaoysa-n")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// PredictProperties
// ---------------------------------------------------------------------------

func TestPredictProperties_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 200, MoleculePropertyResult{
			SMILES:          "CCO",
			CanonicalSMILES: "CCO",
			Properties: map[string]interface{}{
				"logp":       -0.31,
				"solubility": "high",
			},
			Lipinski: &LipinskiRule{
				MolecularWeight: 46.07, LogP: -0.31,
				HBondDonors: 1, HBondAcceptors: 1,
				Violations: 0, Pass: true,
			},
			DrugLikeness:           0.55,
			SyntheticAccessibility: 1.0,
		})
	})
	res, err := mc.PredictProperties(context.Background(), &MoleculePropertyRequest{
		SMILES: "CCO", Properties: []string{"logp", "solubility"},
	})
	if err != nil {
		t.Fatalf("PredictProperties: %v", err)
	}
	if res.SMILES != "CCO" {
		t.Errorf("SMILES: want CCO, got %s", res.SMILES)
	}
	if res.Lipinski == nil || !res.Lipinski.Pass {
		t.Error("Lipinski should be present and Pass=true")
	}
	if res.DrugLikeness != 0.55 {
		t.Errorf("DrugLikeness: want 0.55, got %f", res.DrugLikeness)
	}
}

func TestPredictProperties_EmptySMILES(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.PredictProperties(context.Background(), &MoleculePropertyRequest{
		SMILES: "", Properties: []string{"logp"},
	})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPredictProperties_EmptyProperties(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.PredictProperties(context.Background(), &MoleculePropertyRequest{
		SMILES: "CCO", Properties: []string{},
	})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPredictProperties_NilRequest(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.PredictProperties(context.Background(), nil)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestPredictProperties_RequestBody(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if body["smiles"] != "CCO" {
			t.Errorf("smiles: want CCO, got %v", body["smiles"])
		}
		props, ok := body["properties"].([]interface{})
		if !ok || len(props) != 2 {
			t.Errorf("properties: want 2-element array, got %v", body["properties"])
		}
		writeJSON(t, w, 200, MoleculePropertyResult{SMILES: "CCO"})
	})
	_, _ = mc.PredictProperties(context.Background(), &MoleculePropertyRequest{
		SMILES: "CCO", Properties: []string{"logp", "tpsa"},
	})
}

// ---------------------------------------------------------------------------
// BatchSearch
// ---------------------------------------------------------------------------

func TestBatchSearch_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 200, BatchSearchResult{
			Results: []BatchSearchItem{
				{QuerySMILES: "CCO", Matches: []Molecule{sampleMolecule()}, MatchCount: 1},
				{QuerySMILES: "CC", Matches: []Molecule{}, MatchCount: 0},
				{QuerySMILES: "INVALID", Error: "parse error"},
			},
			TotalProcessed: 3, TotalMatched: 1, ProcessingTime: 1.23,
		})
	})
	res, err := mc.BatchSearch(context.Background(), &BatchSearchRequest{
		Molecules: []string{"CCO", "CC", "INVALID"},
	})
	if err != nil {
		t.Fatalf("BatchSearch: %v", err)
	}
	if len(res.Results) != 3 {
		t.Errorf("Results: want 3, got %d", len(res.Results))
	}
	if res.Results[2].Error != "parse error" {
		t.Errorf("Error: want 'parse error', got %q", res.Results[2].Error)
	}
}

func TestBatchSearch_EmptyMolecules(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.BatchSearch(context.Background(), &BatchSearchRequest{Molecules: []string{}})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestBatchSearch_NilRequest(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.BatchSearch(context.Background(), nil)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestBatchSearch_ExceedLimit(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	mols := make([]string, 1001)
	for i := range mols {
		mols[i] = fmt.Sprintf("C%d", i)
	}
	_, err := mc.BatchSearch(context.Background(), &BatchSearchRequest{Molecules: mols})
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestBatchSearch_ExactLimit(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 200, BatchSearchResult{TotalProcessed: 1000})
	})
	mols := make([]string, 1000)
	for i := range mols {
		mols[i] = fmt.Sprintf("C%d", i)
	}
	res, err := mc.BatchSearch(context.Background(), &BatchSearchRequest{Molecules: mols})
	if err != nil {
		t.Fatalf("BatchSearch: %v", err)
	}
	if res.TotalProcessed != 1000 {
		t.Errorf("TotalProcessed: want 1000, got %d", res.TotalProcessed)
	}
}

func TestBatchSearch_OverLimit(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	mols := make([]string, 1001)
	for i := range mols {
		mols[i] = fmt.Sprintf("C%d", i)
	}
	_, err := mc.BatchSearch(context.Background(), &BatchSearchRequest{Molecules: mols})
	// Check for kerrors.ErrInvalidArgument wrapped
	if err == nil || !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("expected error about limit, got %v", err)
	}
}

func TestGetBySMILES_URLEncoding(t *testing.T) {
	expectedSMILES := "C#C/C=C\\[C@@H](O)[NH3+]"
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		got := r.URL.Query().Get("smiles")
		if got != expectedSMILES {
			t.Errorf("smiles query: want %q, got %q", expectedSMILES, got)
		}
		writeJSON(t, w, 200, moleculeDetailResp{Data: MoleculeDetail{}})
	})
	_, err := mc.GetBySMILES(context.Background(), expectedSMILES)
	if err != nil {
		t.Fatalf("GetBySMILES: %v", err)
	}
}

func TestBatchSearch_DefaultValues(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if body["search_mode"] != "exact" {
			t.Errorf("search_mode: want exact, got %v", body["search_mode"])
		}
		if body["similarity"] != 0.8 {
			t.Errorf("similarity: want 0.8, got %v", body["similarity"])
		}
		writeJSON(t, w, 200, BatchSearchResult{})
	})
	_, _ = mc.BatchSearch(context.Background(), &BatchSearchRequest{
		Molecules: []string{"CCO"},
	})
}

// ---------------------------------------------------------------------------
// GetPatents
// ---------------------------------------------------------------------------

func TestGetPatents_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/mol-001/patents") {
			t.Errorf("path: want contains /mol-001/patents, got %s", r.URL.Path)
		}
		writeJSON(t, w, 200, patentBriefListResp{
			Data: []PatentBrief{
				{PatentNumber: "US111", Title: "Patent A", Applicant: "Corp", FilingDate: "2020-01-01", Relevance: 0.9},
				{PatentNumber: "US222", Title: "Patent B", Applicant: "Corp", FilingDate: "2021-01-01", Relevance: 0.7},
			},
			Meta: &ResponseMeta{Total: 42},
		})
	})
	patents, total, err := mc.GetPatents(context.Background(), "mol-001", 1, 20)
	if err != nil {
		t.Fatalf("GetPatents: %v", err)
	}
	if len(patents) != 2 {
		t.Errorf("patents: want 2, got %d", len(patents))
	}
	if total != 42 {
		t.Errorf("total: want 42, got %d", total)
	}
	if patents[0].PatentNumber != "US111" {
		t.Errorf("PatentNumber: want US111, got %s", patents[0].PatentNumber)
	}
	if patents[1].Relevance != 0.7 {
		t.Errorf("Relevance: want 0.7, got %f", patents[1].Relevance)
	}
}

func TestGetPatents_EmptyID(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, _, err := mc.GetPatents(context.Background(), "", 1, 20)
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestGetPatents_URLParams(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("page") != "3" {
			t.Errorf("page: want 3, got %s", q.Get("page"))
		}
		if q.Get("page_size") != "50" {
			t.Errorf("page_size: want 50, got %s", q.Get("page_size"))
		}
		if !strings.Contains(r.URL.Path, "/mol-abc/patents") {
			t.Errorf("path: want contains /mol-abc/patents, got %s", r.URL.Path)
		}
		writeJSON(t, w, 200, patentBriefListResp{
			Data: []PatentBrief{},
			Meta: &ResponseMeta{Total: 0},
		})
	})
	_, _, err := mc.GetPatents(context.Background(), "mol-abc", 3, 50)
	if err != nil {
		t.Fatalf("GetPatents: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetActivities
// ---------------------------------------------------------------------------

func TestGetActivities_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: want GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/mol-001/activities") {
			t.Errorf("path: want contains /mol-001/activities, got %s", r.URL.Path)
		}
		writeJSON(t, w, 200, bioActivityListResp{
			Data: []BioActivity{
				{Target: "COX-2", ActivityType: "IC50", Value: 50.0, Unit: "uM", Source: "ChEMBL"},
				{Target: "COX-1", ActivityType: "Ki", Value: 1.2, Unit: "nM", Source: "PubChem"},
			},
		})
	})
	acts, err := mc.GetActivities(context.Background(), "mol-001")
	if err != nil {
		t.Fatalf("GetActivities: %v", err)
	}
	if len(acts) != 2 {
		t.Errorf("activities: want 2, got %d", len(acts))
	}
	if acts[0].Target != "COX-2" {
		t.Errorf("Target: want COX-2, got %s", acts[0].Target)
	}
	if acts[0].ActivityType != "IC50" {
		t.Errorf("ActivityType: want IC50, got %s", acts[0].ActivityType)
	}
	if acts[1].Value != 1.2 {
		t.Errorf("Value: want 1.2, got %f", acts[1].Value)
	}
	if acts[1].Unit != "nM" {
		t.Errorf("Unit: want nM, got %s", acts[1].Unit)
	}
}

func TestGetActivities_EmptyID(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.GetActivities(context.Background(), "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// CompareMolecules
// ---------------------------------------------------------------------------

func TestCompareMolecules_Success(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/molecules/compare" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		writeJSON(t, w, 200, moleculeComparisonResp{
			Data: MoleculeComparison{
				Molecule1:                sampleMolecule(),
				Molecule2:                sampleMolecule(),
				TanimotoSimilarity:       0.85,
				DiceSimilarity:           0.92,
				CommonSubstructure:       "c1ccccc1",
				Molecule1UniqueFragments: []string{"C(=O)O"},
				Molecule2UniqueFragments: []string{"N"},
				PropertyDifferences: map[string][2]float64{
					"molecular_weight": {180.16, 93.13},
					"logp":             {1.2, 0.9},
				},
			},
		})
	})
	cmp, err := mc.CompareMolecules(context.Background(), "CC(=O)Oc1ccccc1C(=O)O", "c1ccc(N)cc1")
	if err != nil {
		t.Fatalf("CompareMolecules: %v", err)
	}
	if cmp.TanimotoSimilarity != 0.85 {
		t.Errorf("TanimotoSimilarity: want 0.85, got %f", cmp.TanimotoSimilarity)
	}
	if cmp.DiceSimilarity != 0.92 {
		t.Errorf("DiceSimilarity: want 0.92, got %f", cmp.DiceSimilarity)
	}
	if cmp.CommonSubstructure != "c1ccccc1" {
		t.Errorf("CommonSubstructure: want c1ccccc1, got %s", cmp.CommonSubstructure)
	}
	if len(cmp.Molecule1UniqueFragments) != 1 || cmp.Molecule1UniqueFragments[0] != "C(=O)O" {
		t.Errorf("Molecule1UniqueFragments mismatch: %v", cmp.Molecule1UniqueFragments)
	}
	if len(cmp.Molecule2UniqueFragments) != 1 || cmp.Molecule2UniqueFragments[0] != "N" {
		t.Errorf("Molecule2UniqueFragments mismatch: %v", cmp.Molecule2UniqueFragments)
	}
	mwDiff, ok := cmp.PropertyDifferences["molecular_weight"]
	if !ok {
		t.Fatal("PropertyDifferences missing molecular_weight")
	}
	if mwDiff[0] != 180.16 || mwDiff[1] != 93.13 {
		t.Errorf("molecular_weight diff: want [180.16, 93.13], got %v", mwDiff)
	}
	logpDiff, ok := cmp.PropertyDifferences["logp"]
	if !ok {
		t.Fatal("PropertyDifferences missing logp")
	}
	if logpDiff[0] != 1.2 || logpDiff[1] != 0.9 {
		t.Errorf("logp diff: want [1.2, 0.9], got %v", logpDiff)
	}
}

func TestCompareMolecules_EmptySMILES1(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.CompareMolecules(context.Background(), "", "CCO")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestCompareMolecules_EmptySMILES2(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not be called")
	})
	_, err := mc.CompareMolecules(context.Background(), "CCO", "")
	if !errors.Is(err, kerrors.ErrInvalidArgument) {
		t.Errorf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestCompareMolecules_RequestBody(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		if body["smiles_1"] != "CCO" {
			t.Errorf("smiles_1: want CCO, got %v", body["smiles_1"])
		}
		if body["smiles_2"] != "CC(=O)O" {
			t.Errorf("smiles_2: want CC(=O)O, got %v", body["smiles_2"])
		}
		writeJSON(t, w, 200, moleculeComparisonResp{Data: MoleculeComparison{}})
	})
	_, _ = mc.CompareMolecules(context.Background(), "CCO", "CC(=O)O")
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestMoleculesClient_ContextCancellation(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server — the cancelled context should prevent reaching here
		// in most cases, but the handler is still needed for the test server.
		writeJSON(t, w, 200, MoleculeSearchResult{})
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := mc.Search(ctx, &MoleculeSearchRequest{Query: "test"})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		// Some HTTP client wrappers wrap the error; check string as fallback
		if !strings.Contains(err.Error(), "context canceled") {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// URL path construction
// ---------------------------------------------------------------------------

func TestMoleculesClient_URLPathConstruction(t *testing.T) {
	paths := make(map[string]string)

	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {
		// Record the full path+query for each request
		key := r.Method + " " + r.URL.Path
		paths[key] = r.URL.RawQuery
		// Return appropriate responses based on path
		switch {
		case strings.Contains(r.URL.Path, "/patents"):
			writeJSON(t, w, 200, patentBriefListResp{
				Data: []PatentBrief{},
				Meta: &ResponseMeta{Total: 0},
			})
		case strings.Contains(r.URL.Path, "/activities"):
			writeJSON(t, w, 200, bioActivityListResp{Data: []BioActivity{}})
		case strings.Contains(r.URL.Path, "/search"):
			writeJSON(t, w, 200, MoleculeSearchResult{})
		case strings.Contains(r.URL.Path, "/predict"):
			writeJSON(t, w, 200, MoleculePropertyResult{SMILES: "C"})
		case strings.Contains(r.URL.Path, "/batch-search"):
			writeJSON(t, w, 200, BatchSearchResult{})
		case strings.Contains(r.URL.Path, "/compare"):
			writeJSON(t, w, 200, moleculeComparisonResp{Data: MoleculeComparison{}})
		case strings.Contains(r.URL.Path, "/by-smiles"):
			writeJSON(t, w, 200, moleculeDetailResp{Data: MoleculeDetail{}})
		case strings.Contains(r.URL.Path, "/by-inchikey"):
			writeJSON(t, w, 200, moleculeDetailResp{Data: MoleculeDetail{}})
		default:
			writeJSON(t, w, 200, moleculeDetailResp{Data: MoleculeDetail{}})
		}
	})

	ctx := context.Background()

	// Search
	_, _ = mc.Search(ctx, &MoleculeSearchRequest{Query: "test"})
	if _, ok := paths["POST /api/v1/molecules/search"]; !ok {
		t.Error("Search path not found")
	}

	// Get
	_, _ = mc.Get(ctx, "mol-xyz")
	if _, ok := paths["GET /api/v1/molecules/mol-xyz"]; !ok {
		t.Error("Get path not found")
	}

	// GetBySMILES
	_, _ = mc.GetBySMILES(ctx, "CCO")
	if _, ok := paths["GET /api/v1/molecules/by-smiles"]; !ok {
		t.Error("GetBySMILES path not found")
	}

	// GetByInChIKey
	_, _ = mc.GetByInChIKey(ctx, "BSYNRYMUTXBXSQ-UHFFFAOYSA-N")
	if _, ok := paths["GET /api/v1/molecules/by-inchikey"]; !ok {
		t.Error("GetByInChIKey path not found")
	}

	// PredictProperties
	_, _ = mc.PredictProperties(ctx, &MoleculePropertyRequest{
		SMILES: "C", Properties: []string{"logp"},
	})
	if _, ok := paths["POST /api/v1/molecules/predict"]; !ok {
		t.Error("PredictProperties path not found")
	}

	// BatchSearch
	_, _ = mc.BatchSearch(ctx, &BatchSearchRequest{Molecules: []string{"C"}})
	if _, ok := paths["POST /api/v1/molecules/batch-search"]; !ok {
		t.Error("BatchSearch path not found")
	}

	// GetPatents
	_, _, _ = mc.GetPatents(ctx, "mol-xyz", 1, 10)
	if _, ok := paths["GET /api/v1/molecules/mol-xyz/patents"]; !ok {
		t.Error("GetPatents path not found")
	}

	// GetActivities
	_, _ = mc.GetActivities(ctx, "mol-xyz")
	if _, ok := paths["GET /api/v1/molecules/mol-xyz/activities"]; !ok {
		t.Error("GetActivities path not found")
	}

	// CompareMolecules
	_, _ = mc.CompareMolecules(ctx, "CCO", "CC")
	if _, ok := paths["POST /api/v1/molecules/compare"]; !ok {
		t.Error("CompareMolecules path not found")
	}
}

//Personal.AI order the ending

