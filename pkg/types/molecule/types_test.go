// Package molecule_test provides unit tests for the molecule DTO types,
// enumerations, and request/response structures defined in types.go.
package molecule_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// Enum correctness
// ─────────────────────────────────────────────────────────────────────────────

func TestMoleculeType_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val  molecule.MoleculeType
		want string
	}{
		{molecule.TypeSmallMolecule, "small_molecule"},
		{molecule.TypePolymer, "polymer"},
		{molecule.TypeOLEDMaterial, "oled_material"},
		{molecule.TypeCatalyst, "catalyst"},
		{molecule.TypeIntermediate, "intermediate"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, molecule.MoleculeType(tc.want), tc.val)
		})
	}
}

func TestFingerprintType_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val  molecule.FingerprintType
		want string
	}{
		{molecule.FPMorgan, "morgan"},
		{molecule.FPMACCS, "maccs"},
		{molecule.FPTopological, "topological"},
		{molecule.FPAtomPair, "atom_pair"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, molecule.FingerprintType(tc.want), tc.val)
		})
	}
}

func TestMoleculeType_Distinct(t *testing.T) {
	t.Parallel()

	all := []molecule.MoleculeType{
		molecule.TypeSmallMolecule,
		molecule.TypePolymer,
		molecule.TypeOLEDMaterial,
		molecule.TypeCatalyst,
		molecule.TypeIntermediate,
	}
	seen := make(map[molecule.MoleculeType]bool)
	for _, v := range all {
		assert.False(t, seen[v], "duplicate MoleculeType value: %s", v)
		seen[v] = true
	}
}

func TestFingerprintType_Distinct(t *testing.T) {
	t.Parallel()

	all := []molecule.FingerprintType{
		molecule.FPMorgan,
		molecule.FPMACCS,
		molecule.FPTopological,
		molecule.FPAtomPair,
	}
	seen := make(map[molecule.FingerprintType]bool)
	for _, v := range all {
		assert.False(t, seen[v], "duplicate FingerprintType value: %s", v)
		seen[v] = true
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MoleculeDTO JSON serialisation / deserialisation
// ─────────────────────────────────────────────────────────────────────────────

func newTestMoleculeDTO() molecule.MoleculeDTO {
	homo := -5.4
	lumo := -2.1
	bandGap := lumo - homo // 2.9 eV (note: LUMO - HOMO may be negative in raw form)
	_ = bandGap
	gap := 3.3
	return molecule.MoleculeDTO{
		BaseEntity: common.BaseEntity{
			ID:       common.ID("mol-001"),
			TenantID: common.TenantID("tenant-abc"),
		},
		SMILES:           "c1ccc2[nH]ccc2c1",
		InChI:            "InChI=1S/C8H7N/c1-2-6-8-7(3-1)4-5-9-8/h1-6,9H",
		InChIKey:         "SIKJAQJRHWYJAI-UHFFFAOYSA-N",
		MolecularFormula: "C8H7N",
		MolecularWeight:  117.15,
		Name:             "indole",
		Synonyms:         []string{"1H-indole", "2,3-benzopyrrole"},
		Type:             molecule.TypeOLEDMaterial,
		Properties: molecule.MolecularProperties{
			LogP:           2.14,
			TPSA:           26.84,
			HBondDonors:    1,
			HBondAcceptors: 1,
			RotatableBonds: 0,
			AromaticRings:  2,
			HOMO:           &homo,
			LUMO:           &lumo,
			BandGap:        &gap,
		},
		SourcePatentIDs: []common.ID{"pat-001", "pat-002"},
	}
}

func TestMoleculeDTO_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := newTestMoleculeDTO()

	data, err := json.Marshal(original)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var decoded molecule.MoleculeDTO
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.SMILES, decoded.SMILES)
	assert.Equal(t, original.InChIKey, decoded.InChIKey)
	assert.Equal(t, original.MolecularFormula, decoded.MolecularFormula)
	assert.InDelta(t, original.MolecularWeight, decoded.MolecularWeight, 1e-9)
	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Synonyms, decoded.Synonyms)
	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.SourcePatentIDs, decoded.SourcePatentIDs)
}

func TestMoleculeDTO_JSONOmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()

	m := molecule.MoleculeDTO{
		SMILES:           "C",
		InChIKey:         "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		MolecularFormula: "CH4",
		MolecularWeight:  16.04,
		Type:             molecule.TypeSmallMolecule,
	}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	// Optional fields with omitempty should not appear in the output.
	assert.NotContains(t, string(data), `"name"`)
	assert.NotContains(t, string(data), `"synonyms"`)
	assert.NotContains(t, string(data), `"inchi"`)
	assert.NotContains(t, string(data), `"fingerprints"`)
	assert.NotContains(t, string(data), `"source_patent_ids"`)
}

func TestMoleculeDTO_FingerprintsRoundTrip(t *testing.T) {
	t.Parallel()

	fp := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	m := molecule.MoleculeDTO{
		SMILES: "C",
		Type:   molecule.TypeSmallMolecule,
		Fingerprints: map[molecule.FingerprintType][]byte{
			molecule.FPMorgan: fp,
		},
	}

	data, err := json.Marshal(m)
	require.NoError(t, err)

	var decoded molecule.MoleculeDTO
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NotNil(t, decoded.Fingerprints)
	assert.Equal(t, fp, decoded.Fingerprints[molecule.FPMorgan])
}

// ─────────────────────────────────────────────────────────────────────────────
// MolecularProperties — OLED nullable fields
// ─────────────────────────────────────────────────────────────────────────────

func TestMolecularProperties_OLEDFieldsNilByDefault(t *testing.T) {
	t.Parallel()

	p := molecule.MolecularProperties{
		LogP:           1.5,
		TPSA:           60.0,
		HBondDonors:    2,
		HBondAcceptors: 4,
	}

	assert.Nil(t, p.HOMO, "HOMO should be nil when not set")
	assert.Nil(t, p.LUMO, "LUMO should be nil when not set")
	assert.Nil(t, p.BandGap, "BandGap should be nil when not set")
}

func TestMolecularProperties_OLEDFieldsSet(t *testing.T) {
	t.Parallel()

	homo := -5.4
	lumo := -2.1
	gap := 3.3

	p := molecule.MolecularProperties{
		HOMO:    &homo,
		LUMO:    &lumo,
		BandGap: &gap,
	}

	require.NotNil(t, p.HOMO)
	require.NotNil(t, p.LUMO)
	require.NotNil(t, p.BandGap)
	assert.InDelta(t, -5.4, *p.HOMO, 1e-9)
	assert.InDelta(t, -2.1, *p.LUMO, 1e-9)
	assert.InDelta(t, 3.3, *p.BandGap, 1e-9)
}

func TestMolecularProperties_OLEDFieldsOmittedFromJSON(t *testing.T) {
	t.Parallel()

	p := molecule.MolecularProperties{
		LogP: 2.0,
	}

	data, err := json.Marshal(p)
	require.NoError(t, err)

	assert.NotContains(t, string(data), `"homo"`)
	assert.NotContains(t, string(data), `"lumo"`)
	assert.NotContains(t, string(data), `"band_gap"`)
}

func TestMolecularProperties_OLEDFieldsPresentInJSON(t *testing.T) {
	t.Parallel()

	homo := -5.4
	lumo := -2.1
	gap := 3.3

	p := molecule.MolecularProperties{
		HOMO:    &homo,
		LUMO:    &lumo,
		BandGap: &gap,
	}

	data, err := json.Marshal(p)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"homo"`)
	assert.Contains(t, string(data), `"lumo"`)
	assert.Contains(t, string(data), `"band_gap"`)
}

func TestMolecularProperties_OLEDRoundTrip(t *testing.T) {
	t.Parallel()

	homo := -5.4
	lumo := -2.1
	gap := 3.3
	original := molecule.MolecularProperties{
		LogP:    2.14,
		HOMO:    &homo,
		LUMO:    &lumo,
		BandGap: &gap,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded molecule.MolecularProperties
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.NotNil(t, decoded.HOMO)
	require.NotNil(t, decoded.LUMO)
	require.NotNil(t, decoded.BandGap)
	assert.InDelta(t, *original.HOMO, *decoded.HOMO, 1e-9)
	assert.InDelta(t, *original.LUMO, *decoded.LUMO, 1e-9)
	assert.InDelta(t, *original.BandGap, *decoded.BandGap, 1e-9)
}

// ─────────────────────────────────────────────────────────────────────────────
// MoleculeSearchRequest — pagination parameter handling
// ─────────────────────────────────────────────────────────────────────────────

func TestMoleculeSearchRequest_DefaultPageRequest(t *testing.T) {
	t.Parallel()

	smiles := "c1ccccc1"
	req := molecule.MoleculeSearchRequest{
		SMILES: &smiles,
	}

	// Zero-value PageRequest fields are acceptable; service layer applies defaults.
	assert.Equal(t, 0, req.Page)
	assert.Equal(t, 0, req.PageSize)
}

func TestMoleculeSearchRequest_WithPagination(t *testing.T) {
	t.Parallel()

	name := "indole"
	fpType := molecule.FPMorgan
	minSim := 0.85

	req := molecule.MoleculeSearchRequest{
		Name:            &name,
		FingerprintType: &fpType,
		MinSimilarity:   &minSim,
		PageRequest: common.PageRequest{
			Page:     2,
			PageSize: 25,
		},
	}

	assert.Equal(t, 2, req.Page)
	assert.Equal(t, 25, req.PageSize)
	require.NotNil(t, req.Name)
	assert.Equal(t, "indole", *req.Name)
	require.NotNil(t, req.FingerprintType)
	assert.Equal(t, molecule.FPMorgan, *req.FingerprintType)
	require.NotNil(t, req.MinSimilarity)
	assert.InDelta(t, 0.85, *req.MinSimilarity, 1e-9)
}

func TestMoleculeSearchRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	smiles := "c1ccc2[nH]ccc2c1"
	molType := molecule.TypeOLEDMaterial
	minSim := 0.7
	fpType := molecule.FPAtomPair

	original := molecule.MoleculeSearchRequest{
		SMILES:          &smiles,
		Type:            &molType,
		MinSimilarity:   &minSim,
		FingerprintType: &fpType,
		PageRequest: common.PageRequest{
			Page:     1,
			PageSize: 50,
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded molecule.MoleculeSearchRequest
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.NotNil(t, decoded.SMILES)
	assert.Equal(t, smiles, *decoded.SMILES)
	require.NotNil(t, decoded.Type)
	assert.Equal(t, molType, *decoded.Type)
	require.NotNil(t, decoded.MinSimilarity)
	assert.InDelta(t, minSim, *decoded.MinSimilarity, 1e-9)
	require.NotNil(t, decoded.FingerprintType)
	assert.Equal(t, fpType, *decoded.FingerprintType)
	assert.Equal(t, 1, decoded.Page)
	assert.Equal(t, 50, decoded.PageSize)
}

func TestMoleculeSearchRequest_AllFiltersNilIsValid(t *testing.T) {
	t.Parallel()

	req := molecule.MoleculeSearchRequest{}

	assert.Nil(t, req.SMILES)
	assert.Nil(t, req.Name)
	assert.Nil(t, req.Type)
	assert.Nil(t, req.MinSimilarity)
	assert.Nil(t, req.FingerprintType)
}

// ─────────────────────────────────────────────────────────────────────────────
// SubstructureSearchRequest / SubstructureSearchResponse
// ─────────────────────────────────────────────────────────────────────────────

func TestSubstructureSearchRequest_Fields(t *testing.T) {
	t.Parallel()

	req := molecule.SubstructureSearchRequest{
		SMARTS:     "c1ccc2[nH]ccc2c1",
		MaxResults: 200,
	}

	assert.Equal(t, "c1ccc2[nH]ccc2c1", req.SMARTS)
	assert.Equal(t, 200, req.MaxResults)
}

func TestSubstructureSearchRequest_ZeroMaxResultsIsAccepted(t *testing.T) {
	t.Parallel()

	// Zero means "use service default"; not a validation error at the DTO level.
	req := molecule.SubstructureSearchRequest{SMARTS: "C"}
	assert.Equal(t, 0, req.MaxResults)
}

func TestSubstructureSearchResponse_EmptyResults(t *testing.T) {
	t.Parallel()

	resp := molecule.SubstructureSearchResponse{
		Results: []molecule.MoleculeDTO{},
		Total:   0,
	}

	assert.Empty(t, resp.Results)
	assert.Equal(t, 0, resp.Total)
}

func TestSubstructureSearchResponse_WithResults(t *testing.T) {
	t.Parallel()

	results := []molecule.MoleculeDTO{
		{SMILES: "c1ccc2[nH]ccc2c1", Type: molecule.TypeOLEDMaterial},
		{SMILES: "c1ccnc2[nH]ccc12", Type: molecule.TypeOLEDMaterial},
	}
	resp := molecule.SubstructureSearchResponse{
		Results: results,
		Total:   42, // total in corpus before MaxResults cap
	}

	assert.Len(t, resp.Results, 2)
	assert.Equal(t, 42, resp.Total)
}

func TestSubstructureSearchResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := molecule.SubstructureSearchResponse{
		Results: []molecule.MoleculeDTO{
			{
				SMILES:   "c1ccc2[nH]ccc2c1",
				Type:     molecule.TypeOLEDMaterial,
				InChIKey: "SIKJAQJRHWYJAI-UHFFFAOYSA-N",
			},
		},
		Total: 1,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded molecule.SubstructureSearchResponse
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Len(t, decoded.Results, 1)
	assert.Equal(t, original.Results[0].SMILES, decoded.Results[0].SMILES)
	assert.Equal(t, original.Total, decoded.Total)
}

//Personal.AI order the ending
