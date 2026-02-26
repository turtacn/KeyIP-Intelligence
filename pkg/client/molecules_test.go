// Phase 13 - SDK Molecules Client Tests (297/349)
// File: pkg/client/molecules_test.go
// Unit tests for molecules sub-client.

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

func newTestMoleculesClient(t *testing.T, handler http.HandlerFunc) *MoleculesClient {
	c := newTestClient(t, handler)
	return c.Molecules()
}

func TestMolecules_Search(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/molecules/search", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req MoleculeSearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "aspirin", req.Query)
		assert.Equal(t, 0.8, req.Similarity) // Default

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MoleculeSearchResult{
			Total: 1,
			Molecules: []Molecule{{ID: "m1", SMILES: "CC(=O)Oc1ccccc1C(=O)O"}},
		})
	}
	mc := newTestMoleculesClient(t, handler)
	res, err := mc.Search(context.Background(), &MoleculeSearchRequest{Query: "aspirin"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
	assert.Len(t, res.Molecules, 1)
}

func TestMolecules_Search_Validation(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {})

	_, err := mc.Search(context.Background(), nil)
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)

	_, err = mc.Search(context.Background(), &MoleculeSearchRequest{Query: ""})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)

	_, err = mc.Search(context.Background(), &MoleculeSearchRequest{Query: "q", Similarity: 1.1})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestMolecules_Get(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/molecules/m1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		// Wrapper format
		json.NewEncoder(w).Encode(APIResponse[MoleculeDetail]{
			Data: MoleculeDetail{Molecule: Molecule{ID: "m1"}},
		})
	}
	mc := newTestMoleculesClient(t, handler)
	res, err := mc.Get(context.Background(), "m1")
	require.NoError(t, err)
	assert.Equal(t, "m1", res.ID)
}

func TestMolecules_Get_Validation(t *testing.T) {
	mc := newTestMoleculesClient(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := mc.Get(context.Background(), "")
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestMolecules_GetBySMILES(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "C=C", r.URL.Query().Get("smiles"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[MoleculeDetail]{
			Data: MoleculeDetail{Molecule: Molecule{SMILES: "C=C"}},
		})
	}
	mc := newTestMoleculesClient(t, handler)
	res, err := mc.GetBySMILES(context.Background(), "C=C")
	require.NoError(t, err)
	assert.Equal(t, "C=C", res.SMILES)
}

func TestMolecules_GetByInChIKey(t *testing.T) {
	validKey := "BSYNRYMUTXBXSQ-UHFFFAOYSA-N"
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, validKey, r.URL.Query().Get("inchikey"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[MoleculeDetail]{
			Data: MoleculeDetail{Molecule: Molecule{InChIKey: validKey}},
		})
	}
	mc := newTestMoleculesClient(t, handler)

	// Valid
	res, err := mc.GetByInChIKey(context.Background(), validKey)
	require.NoError(t, err)
	assert.Equal(t, validKey, res.InChIKey)

	// Invalid Format
	_, err = mc.GetByInChIKey(context.Background(), "invalid")
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestMolecules_PredictProperties(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		var req MoleculePropertyRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "C", req.SMILES)
		assert.Equal(t, []string{"logp"}, req.Properties)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[MoleculePropertyResult]{
			Data: MoleculePropertyResult{SMILES: "C", Properties: map[string]interface{}{"logp": 1.0}},
		})
	}
	mc := newTestMoleculesClient(t, handler)
	res, err := mc.PredictProperties(context.Background(), &MoleculePropertyRequest{
		SMILES: "C", Properties: []string{"logp"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1.0, res.Properties["logp"])
}

func TestMolecules_BatchSearch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[BatchSearchResult]{
			Data: BatchSearchResult{TotalProcessed: 10},
		})
	}
	mc := newTestMoleculesClient(t, handler)

	// Success
	res, err := mc.BatchSearch(context.Background(), &BatchSearchRequest{Molecules: []string{"C"}})
	require.NoError(t, err)
	assert.Equal(t, 10, res.TotalProcessed)

	// Too many
	many := make([]string, 1001)
	_, err = mc.BatchSearch(context.Background(), &BatchSearchRequest{Molecules: many})
	assert.ErrorIs(t, err, keyiperrors.ErrInvalidArgument)
}

func TestMolecules_GetPatents(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1", r.URL.Query().Get("page"))
		assert.Equal(t, "20", r.URL.Query().Get("page_size"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[[]PatentBrief]{
			Data: []PatentBrief{{PatentNumber: "P1"}},
			Meta: &ResponseMeta{Total: 5},
		})
	}
	mc := newTestMoleculesClient(t, handler)
	res, total, err := mc.GetPatents(context.Background(), "m1", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, res, 1)
}

func TestMolecules_CompareMolecules(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "A", body["smiles_1"])
		assert.Equal(t, "B", body["smiles_2"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(APIResponse[MoleculeComparison]{
			Data: MoleculeComparison{TanimotoSimilarity: 0.5},
		})
	}
	mc := newTestMoleculesClient(t, handler)
	res, err := mc.CompareMolecules(context.Background(), "A", "B")
	require.NoError(t, err)
	assert.Equal(t, 0.5, res.TanimotoSimilarity)
}

//Personal.AI order the ending
