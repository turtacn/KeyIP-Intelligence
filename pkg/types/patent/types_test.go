// Package patent_test provides unit tests for the patent DTO and enumeration
// types defined in pkg/types/patent/types.go.
package patent_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// PatentStatus enum tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPatentStatus_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		status    patent.PatentStatus
		wantStr   string
		wantValid bool
	}{
		{"Filed", patent.StatusFiled, "FILED", true},
		{"Published", patent.StatusPublished, "PUBLISHED", true},
		{"Granted", patent.StatusGranted, "GRANTED", true},
		{"Expired", patent.StatusExpired, "EXPIRED", true},
		{"Abandoned", patent.StatusAbandoned, "ABANDONED", true},
		{"Revoked", patent.StatusRevoked, "REVOKED", true},
		{"Unknown", patent.PatentStatus("UNKNOWN"), "UNKNOWN", false},
		{"Empty", patent.PatentStatus(""), "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, patent.PatentStatus(tc.wantStr), tc.status,
				"status string value mismatch")
			assert.Equal(t, tc.wantValid, tc.status.IsValid(),
				"IsValid() mismatch for %q", tc.status)
		})
	}
}

func TestPatentStatus_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	statuses := []patent.PatentStatus{
		patent.StatusFiled,
		patent.StatusPublished,
		patent.StatusGranted,
		patent.StatusExpired,
		patent.StatusAbandoned,
		patent.StatusRevoked,
	}
	for _, s := range statuses {
		s := s
		t.Run(string(s), func(t *testing.T) {
			t.Parallel()
			b, err := json.Marshal(s)
			require.NoError(t, err)

			var got patent.PatentStatus
			require.NoError(t, json.Unmarshal(b, &got))
			assert.Equal(t, s, got)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ClaimType enum tests
// ─────────────────────────────────────────────────────────────────────────────

func TestClaimType_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		ct        patent.ClaimType
		wantStr   string
		wantValid bool
	}{
		{patent.ClaimIndependent, "INDEPENDENT", true},
		{patent.ClaimDependent, "DEPENDENT", true},
		{patent.ClaimType("OTHER"), "OTHER", false},
		{patent.ClaimType(""), "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.wantStr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, patent.ClaimType(tc.wantStr), tc.ct)
			assert.Equal(t, tc.wantValid, tc.ct.IsValid())
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// JurisdictionCode enum tests
// ─────────────────────────────────────────────────────────────────────────────

func TestJurisdictionCode_Values(t *testing.T) {
	t.Parallel()

	cases := []struct {
		jc        patent.JurisdictionCode
		wantStr   string
		wantValid bool
	}{
		{patent.JurisdictionCN, "CN", true},
		{patent.JurisdictionUS, "US", true},
		{patent.JurisdictionEP, "EP", true},
		{patent.JurisdictionJP, "JP", true},
		{patent.JurisdictionKR, "KR", true},
		{patent.JurisdictionWO, "WO", true},
		{patent.JurisdictionCode("DE"), "DE", false},
		{patent.JurisdictionCode(""), "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.wantStr, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, patent.JurisdictionCode(tc.wantStr), tc.jc)
			assert.Equal(t, tc.wantValid, tc.jc.IsValid())
		})
	}
}

func TestJurisdictionCode_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	codes := []patent.JurisdictionCode{
		patent.JurisdictionCN,
		patent.JurisdictionUS,
		patent.JurisdictionEP,
		patent.JurisdictionJP,
		patent.JurisdictionKR,
		patent.JurisdictionWO,
	}
	for _, jc := range codes {
		jc := jc
		t.Run(string(jc), func(t *testing.T) {
			t.Parallel()
			b, err := json.Marshal(jc)
			require.NoError(t, err)
			var got patent.JurisdictionCode
			require.NoError(t, json.Unmarshal(b, &got))
			assert.Equal(t, jc, got)
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PatentDTO JSON round-trip tests
// ─────────────────────────────────────────────────────────────────────────────

func makeTestPatentDTO() patent.PatentDTO {
	now := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	pub := now.Add(18 * 30 * 24 * time.Hour)
	grant := now.Add(36 * 30 * 24 * time.Hour)
	expiry := now.Add(20 * 365 * 24 * time.Hour)
	parentClaim := 1

	return patent.PatentDTO{
		BaseEntity: common.BaseEntity{
			ID:        common.ID("pat-001"),
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		},
		PatentNumber:    "CN202410001234A",
		Title:           "OLED Host Material Comprising Carbazole Moiety",
		Abstract:        "The present invention relates to an organic electroluminescent compound...",
		Applicant:       "Acme Chemical Co., Ltd.",
		Inventors:       []string{"Zhang Wei", "Li Fang"},
		FilingDate:      now,
		PublicationDate: &pub,
		GrantDate:       &grant,
		ExpiryDate:      &expiry,
		Status:          patent.StatusGranted,
		Jurisdiction:    patent.JurisdictionCN,
		IPCCodes:        []string{"C07D209/14", "C09K11/06"},
		CPCCodes:        []string{"C07D209/14"},
		FamilyID:        "family-xyz-001",
		Priority: []patent.PriorityDTO{
			{Country: "CN", Number: "CN202310001234", Date: now.Add(-365 * 24 * time.Hour)},
		},
		Claims: []patent.ClaimDTO{
			{
				ID:     common.ID("claim-001"),
				Number: 1,
				Text:   "A compound of formula (I) wherein ...",
				Type:   patent.ClaimIndependent,
				Elements: []patent.ClaimElementDTO{
					{
						ID:               common.ID("elem-001"),
						Text:             "a carbazole core",
						IsStructural:     true,
						ChemicalEntities: []string{"carbazole", "C1=CC2=CC=CC=C2N1"},
					},
				},
			},
			{
				ID:                common.ID("claim-002"),
				Number:            2,
				Text:              "The compound of claim 1 wherein R1 is methyl ...",
				Type:              patent.ClaimDependent,
				ParentClaimNumber: &parentClaim,
			},
		},
	}
}

func TestPatentDTO_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := makeTestPatentDTO()

	b, err := json.Marshal(original)
	require.NoError(t, err, "Marshal must not fail")
	require.NotEmpty(t, b)

	var recovered patent.PatentDTO
	require.NoError(t, json.Unmarshal(b, &recovered), "Unmarshal must not fail")

	// Check scalar fields.
	assert.Equal(t, original.PatentNumber, recovered.PatentNumber)
	assert.Equal(t, original.Title, recovered.Title)
	assert.Equal(t, original.Abstract, recovered.Abstract)
	assert.Equal(t, original.Applicant, recovered.Applicant)
	assert.Equal(t, original.Status, recovered.Status)
	assert.Equal(t, original.Jurisdiction, recovered.Jurisdiction)
	assert.Equal(t, original.FamilyID, recovered.FamilyID)
	assert.Equal(t, original.Version, recovered.Version)

	// Check slice fields.
	assert.Equal(t, original.Inventors, recovered.Inventors)
	assert.Equal(t, original.IPCCodes, recovered.IPCCodes)
	assert.Equal(t, original.CPCCodes, recovered.CPCCodes)

	// Check time fields (compare Unix timestamps to avoid TZ drift).
	assert.Equal(t, original.FilingDate.Unix(), recovered.FilingDate.Unix())
	require.NotNil(t, recovered.PublicationDate)
	assert.Equal(t, original.PublicationDate.Unix(), recovered.PublicationDate.Unix())
	require.NotNil(t, recovered.GrantDate)
	assert.Equal(t, original.GrantDate.Unix(), recovered.GrantDate.Unix())
	require.NotNil(t, recovered.ExpiryDate)
	assert.Equal(t, original.ExpiryDate.Unix(), recovered.ExpiryDate.Unix())

	// Check claims.
	require.Len(t, recovered.Claims, 2)
	assert.Equal(t, original.Claims[0].Number, recovered.Claims[0].Number)
	assert.Equal(t, original.Claims[0].Type, recovered.Claims[0].Type)
	assert.Nil(t, recovered.Claims[0].ParentClaimNumber)
	require.NotNil(t, recovered.Claims[1].ParentClaimNumber)
	assert.Equal(t, 1, *recovered.Claims[1].ParentClaimNumber)

	// Check claim elements.
	require.Len(t, recovered.Claims[0].Elements, 1)
	assert.True(t, recovered.Claims[0].Elements[0].IsStructural)
	assert.Equal(t, original.Claims[0].Elements[0].ChemicalEntities,
		recovered.Claims[0].Elements[0].ChemicalEntities)

	// Check priority.
	require.Len(t, recovered.Priority, 1)
	assert.Equal(t, "CN", recovered.Priority[0].Country)
}

func TestPatentDTO_OptionalNilFields(t *testing.T) {
	t.Parallel()

	// PatentDTO with only mandatory fields.
	dto := patent.PatentDTO{
		BaseEntity:   common.BaseEntity{ID: common.ID("pat-min")},
		PatentNumber: "WO2024000001",
		Status:       patent.StatusFiled,
		Jurisdiction: patent.JurisdictionWO,
		FilingDate:   time.Now().UTC(),
	}

	b, err := json.Marshal(dto)
	require.NoError(t, err)

	var recovered patent.PatentDTO
	require.NoError(t, json.Unmarshal(b, &recovered))

	assert.Nil(t, recovered.PublicationDate, "PublicationDate must be nil when not set")
	assert.Nil(t, recovered.GrantDate, "GrantDate must be nil when not set")
	assert.Nil(t, recovered.ExpiryDate, "ExpiryDate must be nil when not set")
	assert.Empty(t, recovered.Claims)
	assert.Empty(t, recovered.Priority)
}

// ─────────────────────────────────────────────────────────────────────────────
// MarkushDTO and RGroupDTO structure tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMarkushDTO_StructuralIntegrity(t *testing.T) {
	t.Parallel()

	m := patent.MarkushDTO{
		ID:            common.ID("markush-001"),
		PatentID:      common.ID("pat-001"),
		ClaimID:       common.ID("claim-001"),
		CoreStructure: "c1cc2[nH]c3ccccc3c2cc1", // carbazole scaffold
		Description:   "Carbazole-based host material with R1 at 2-position",
		RGroups: []patent.RGroupDTO{
			{
				Position:     "R1",
				Alternatives: []string{"C", "CC", "CCC", "~alkyl"},
				Description:  "Linear alkyl chain C1-C4 or generic alkyl",
			},
			{
				Position:     "R2",
				Alternatives: []string{"c1ccccc1", "c1ccncc1"},
				Description:  "Phenyl or pyridyl",
			},
		},
	}

	// Verify all fields are accessible.
	assert.Equal(t, common.ID("markush-001"), m.ID)
	assert.Equal(t, common.ID("pat-001"), m.PatentID)
	assert.Equal(t, common.ID("claim-001"), m.ClaimID)
	assert.NotEmpty(t, m.CoreStructure)
	assert.NotEmpty(t, m.Description)
	require.Len(t, m.RGroups, 2)

	// Verify R-group 0 (R1).
	assert.Equal(t, "R1", m.RGroups[0].Position)
	assert.Len(t, m.RGroups[0].Alternatives, 4)
	assert.Contains(t, m.RGroups[0].Alternatives, "~alkyl",
		"generic group notation must be preserved as-is")

	// Verify R-group 1 (R2).
	assert.Equal(t, "R2", m.RGroups[1].Position)
	assert.Len(t, m.RGroups[1].Alternatives, 2)
}

func TestMarkushDTO_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := patent.MarkushDTO{
		ID:            common.ID("markush-002"),
		PatentID:      common.ID("pat-002"),
		ClaimID:       common.ID("claim-002"),
		CoreStructure: "C1=CC=CC=C1",
		RGroups: []patent.RGroupDTO{
			{Position: "R1", Alternatives: []string{"F", "Cl", "Br"}},
		},
	}

	b, err := json.Marshal(original)
	require.NoError(t, err)

	var recovered patent.MarkushDTO
	require.NoError(t, json.Unmarshal(b, &recovered))

	assert.Equal(t, original.ID, recovered.ID)
	assert.Equal(t, original.PatentID, recovered.PatentID)
	assert.Equal(t, original.CoreStructure, recovered.CoreStructure)
	require.Len(t, recovered.RGroups, 1)
	assert.Equal(t, "R1", recovered.RGroups[0].Position)
	assert.Equal(t, original.RGroups[0].Alternatives, recovered.RGroups[0].Alternatives)
}

func TestRGroupDTO_EmptyAlternativesAllowed(t *testing.T) {
	t.Parallel()

	rg := patent.RGroupDTO{
		Position:    "R3",
		Description: "Position not yet enumerated",
	}

	b, err := json.Marshal(rg)
	require.NoError(t, err)

	var recovered patent.RGroupDTO
	require.NoError(t, json.Unmarshal(b, &recovered))

	assert.Equal(t, "R3", recovered.Position)
	assert.Empty(t, recovered.Alternatives)
	assert.Equal(t, "Position not yet enumerated", recovered.Description)
}

// ─────────────────────────────────────────────────────────────────────────────
// PatentSearchRequest — PageRequest.Validate() propagation
// ─────────────────────────────────────────────────────────────────────────────

func TestPatentSearchRequest_PageRequestValidation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		page      int
		size      int
		wantValid bool
	}{
		{"valid first page", 1, 20, true},
		{"valid large page", 10, 100, true},
		{"zero page number", 0, 20, false},
		{"negative size", 1, -1, false},
		{"zero size", 1, 0, false},
		{"oversized page", 1, 10001, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := patent.PatentSearchRequest{
				Query: "carbazole OLED",
				PageRequest: common.PageRequest{
					Page:     tc.page,
					PageSize: tc.size,
				},
			}
			err := req.PageRequest.Validate()
			if tc.wantValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestPatentSearchRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	jc := patent.JurisdictionCN
	applicant := "Acme Corp"
	ipc := "C07D"
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	original := patent.PatentSearchRequest{
		Query:        "OLED host material",
		Jurisdiction: &jc,
		DateFrom:     &from,
		DateTo:       &to,
		Applicant:    &applicant,
		IPCCode:      &ipc,
		PageRequest:  common.PageRequest{Page: 2, PageSize: 25},
	}

	b, err := json.Marshal(original)
	require.NoError(t, err)

	var recovered patent.PatentSearchRequest
	require.NoError(t, json.Unmarshal(b, &recovered))

	assert.Equal(t, original.Query, recovered.Query)
	require.NotNil(t, recovered.Jurisdiction)
	assert.Equal(t, patent.JurisdictionCN, *recovered.Jurisdiction)
	require.NotNil(t, recovered.Applicant)
	assert.Equal(t, applicant, *recovered.Applicant)
	require.NotNil(t, recovered.IPCCode)
	assert.Equal(t, ipc, *recovered.IPCCode)
	assert.Equal(t, original.PageRequest.Page, recovered.PageRequest.Page)
	assert.Equal(t, original.PageRequest.PageSize, recovered.PageRequest.PageSize)
}

// ─────────────────────────────────────────────────────────────────────────────
// SimilaritySearchRequest / Response tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSimilaritySearchRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	jc := patent.JurisdictionUS
	original := patent.SimilaritySearchRequest{
		SMILES:       "c1ccc2[nH]c3ccccc3c2c1",
		Threshold:    0.85,
		MaxResults:   100,
		Jurisdiction: &jc,
	}

	b, err := json.Marshal(original)
	require.NoError(t, err)

	var recovered patent.SimilaritySearchRequest
	require.NoError(t, json.Unmarshal(b, &recovered))

	assert.Equal(t, original.SMILES, recovered.SMILES)
	assert.InDelta(t, original.Threshold, recovered.Threshold, 1e-9)
	assert.Equal(t, original.MaxResults, recovered.MaxResults)
	require.NotNil(t, recovered.Jurisdiction)
	assert.Equal(t, patent.JurisdictionUS, *recovered.Jurisdiction)
}

func TestSimilaritySearchResponse_Structure(t *testing.T) {
	t.Parallel()

	response := patent.SimilaritySearchResponse{
		Results: []patent.SimilarityResult{
			{
				Patent:        makeTestPatentDTO(),
				Score:         0.92,
				MatchedClaims: []int{1, 3, 5},
			},
			{
				Patent:        patent.PatentDTO{PatentNumber: "US11111111B2"},
				Score:         0.78,
				MatchedClaims: []int{2},
			},
		},
	}

	require.Len(t, response.Results, 2)
	assert.InDelta(t, 0.92, response.Results[0].Score, 1e-9)
	assert.Equal(t, []int{1, 3, 5}, response.Results[0].MatchedClaims)
	assert.InDelta(t, 0.78, response.Results[1].Score, 1e-9)
}

func TestSimilaritySearchResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := patent.SimilaritySearchResponse{
		Results: []patent.SimilarityResult{
			{
				Patent:        patent.PatentDTO{PatentNumber: "EP3456789A1", Status: patent.StatusGranted},
				Score:         0.88,
				MatchedClaims: []int{1, 2},
			},
		},
	}

	b, err := json.Marshal(original)
	require.NoError(t, err)

	var recovered patent.SimilaritySearchResponse
	require.NoError(t, json.Unmarshal(b, &recovered))

	require.Len(t, recovered.Results, 1)
	assert.Equal(t, "EP3456789A1", recovered.Results[0].Patent.PatentNumber)
	assert.InDelta(t, 0.88, recovered.Results[0].Score, 1e-9)
	assert.Equal(t, []int{1, 2}, recovered.Results[0].MatchedClaims)
}

func TestSimilarityResult_NilMatchedClaims(t *testing.T) {
	t.Parallel()

	result := patent.SimilarityResult{
		Patent: patent.PatentDTO{PatentNumber: "WO2024999999"},
		Score:  0.71,
	}

	b, err := json.Marshal(result)
	require.NoError(t, err)

	var recovered patent.SimilarityResult
	require.NoError(t, json.Unmarshal(b, &recovered))

	assert.Empty(t, recovered.MatchedClaims,
		"nil MatchedClaims must round-trip as empty/nil")
}

// ─────────────────────────────────────────────────────────────────────────────
// PriorityDTO tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPriorityDTO_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	date := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)
	original := patent.PriorityDTO{
		Country: "CN",
		Number:  "CN202310001234",
		Date:    date,
	}

	b, err := json.Marshal(original)
	require.NoError(t, err)

	var recovered patent.PriorityDTO
	require.NoError(t, json.Unmarshal(b, &recovered))

	assert.Equal(t, original.Country, recovered.Country)
	assert.Equal(t, original.Number, recovered.Number)
	assert.Equal(t, original.Date.Unix(), recovered.Date.Unix())
}

// ─────────────────────────────────────────────────────────────────────────────
// ClaimDTO and ClaimElementDTO tests
// ─────────────────────────────────────────────────────────────────────────────

func TestClaimDTO_DependentClaimParentRef(t *testing.T) {
	t.Parallel()

	parentNum := 1
	claim := patent.ClaimDTO{
		ID:                common.ID("claim-dep-1"),
		Number:            2,
		Text:              "The compound of claim 1 wherein ...",
		Type:              patent.ClaimDependent,
		ParentClaimNumber: &parentNum,
	}

	assert.Equal(t, patent.ClaimDependent, claim.Type)
	require.NotNil(t, claim.ParentClaimNumber)
	assert.Equal(t, 1, *claim.ParentClaimNumber)
}

func TestClaimDTO_IndependentClaimNoParent(t *testing.T) {
	t.Parallel()

	claim := patent.ClaimDTO{
		ID:     common.ID("claim-ind-1"),
		Number: 1,
		Text:   "A compound comprising ...",
		Type:   patent.ClaimIndependent,
	}

	assert.Equal(t, patent.ClaimIndependent, claim.Type)
	assert.Nil(t, claim.ParentClaimNumber,
		"independent claim must have nil ParentClaimNumber")
}

func TestClaimElementDTO_StructuralFlag(t *testing.T) {
	t.Parallel()

	structural := patent.ClaimElementDTO{
		ID:           common.ID("elem-s"),
		Text:         "a carbazole core substituent at the 9-position",
		IsStructural: true,
	}

	functional := patent.ClaimElementDTO{
		ID:           common.ID("elem-f"),
		Text:         "capable of transporting holes",
		IsStructural: false,
	}

	assert.True(t, structural.IsStructural)
	assert.False(t, functional.IsStructural)
}

