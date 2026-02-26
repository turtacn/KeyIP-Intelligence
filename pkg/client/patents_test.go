// Phase 13 - SDK Patents Client Tests (301/349)
// File: pkg/client/patents_test.go
// Unit tests for patents sub-client.

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	keyiperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func newTestPatentsClient(t *testing.T, handler http.HandlerFunc) *PatentsClient {
	c := newTestClient(t, handler)
	return c.Patents()
}

func TestPatents_Search(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/patents/search", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req PatentSearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "keyword", req.QueryType)
		assert.Equal(t, "OLED", req.Query)

		w.Header().Set("Content-Type", "application/json")
		// Assumed direct struct return based on implementation choice
		json.NewEncoder(w).Encode(PatentSearchResult{
			Total: 10,
			Patents: []Patent{{ID: "p1", Title: "OLED Device"}},
		})
	}
	pc := newTestPatentsClient(t, handler)
	res, err := pc.Search(context.Background(), &PatentSearchRequest{Query: "OLED"})
	require.NoError(t, err)
	assert.Equal(t, int64(10), res.Total)
	assert.Len(t, res.Patents, 1)
}

func TestPatents_Search_Validation(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {})

	_, err := pc.Search(context.Background(), nil)
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)

	_, err = pc.Search(context.Background(), &PatentSearchRequest{Query: ""})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestPatents_Search_DateRange(t *testing.T) {
	pc := newTestPatentsClient(t, func(w http.ResponseWriter, r *http.Request) {})
	// Invalid range
	_, err := pc.Search(context.Background(), &PatentSearchRequest{
		Query: "q",
		DateRange: &DateRange{From: "2024-01-01", To: "2023-01-01"},
	})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestPatents_Get(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/patents/p1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[PatentDetail]{
			Data: PatentDetail{Patent: Patent{ID: "p1"}},
		})
	}
	pc := newTestPatentsClient(t, handler)
	res, err := pc.Get(context.Background(), "p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", res.ID)
}

func TestPatents_GetByNumber(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "US123", r.URL.Query().Get("number"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[PatentDetail]{
			Data: PatentDetail{Patent: Patent{PatentNumber: "US123"}},
		})
	}
	pc := newTestPatentsClient(t, handler)
	res, err := pc.GetByNumber(context.Background(), "US123")
	require.NoError(t, err)
	assert.Equal(t, "US123", res.PatentNumber)
}

func TestPatents_GetFamily(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]FamilyMember]{
			Data: []FamilyMember{{PatentNumber: "CN1"}},
		})
	}
	pc := newTestPatentsClient(t, handler)
	res, err := pc.GetFamily(context.Background(), "US1")
	require.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestPatents_GetCitations(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]Citation]{
			Data: []Citation{{PatentNumber: "CN2"}},
		})
	}
	pc := newTestPatentsClient(t, handler)
	res, err := pc.GetCitations(context.Background(), "US1")
	require.NoError(t, err)
	assert.Len(t, res, 1)
}

func TestPatents_GetMolecules(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]MoleculeBrief]{
			Data: []MoleculeBrief{{ID: "m1"}},
			Meta: &ResponseMeta{Total: 5},
		})
	}
	pc := newTestPatentsClient(t, handler)
	res, total, err := pc.GetMolecules(context.Background(), "US1", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, res, 1)
}

func TestPatents_EvaluateValue(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[PatentValueResult]{
			Data: PatentValueResult{Evaluations: []PatentValuation{{OverallScore: 90}}},
		})
	}
	pc := newTestPatentsClient(t, handler)

	// Valid
	res, err := pc.EvaluateValue(context.Background(), &PatentValueRequest{PatentNumbers: []string{"US1"}})
	require.NoError(t, err)
	assert.Equal(t, 90.0, res.Evaluations[0].OverallScore)

	// Too many
	many := make([]string, 101)
	_, err = pc.EvaluateValue(context.Background(), &PatentValueRequest{PatentNumbers: many})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestPatents_GetLandscape(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[PatentLandscape]{
			Data: PatentLandscape{TotalPatents: 100},
		})
	}
	pc := newTestPatentsClient(t, handler)
	res, err := pc.GetLandscape(context.Background(), &PatentLandscapeRequest{Query: "tech"})
	require.NoError(t, err)
	assert.Equal(t, int64(100), res.TotalPatents)
}

func TestPatents_SemanticSearch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		var req PatentSearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "semantic", req.QueryType)
		assert.Equal(t, "text", req.Query)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PatentSearchResult{Total: 5})
	}
	pc := newTestPatentsClient(t, handler)
	res, err := pc.SemanticSearch(context.Background(), "text", 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(5), res.Total)
}

//Personal.AI order the ending
