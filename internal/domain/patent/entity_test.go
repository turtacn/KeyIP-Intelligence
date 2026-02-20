// Package patent_test provides comprehensive unit tests for the Patent
// aggregate root defined in internal/domain/patent/entity.go.
package patent_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func validFilingDate() time.Time {
	return time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
}

// newValidPatent creates a Patent using well-formed parameters so tests can
// focus on the single behaviour being exercised.
func newValidPatent(t *testing.T) *patent.Patent {
	t.Helper()
	p, err := patent.NewPatent(
		"CN202010001234A",
		"An OLED emitter material based on carbazole",
		"The present invention discloses an organic light-emitting diode material...",
		"ACME Chemical Corp",
		ptypes.JurisdictionCN,
		validFilingDate(),
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	return p
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewPatent
// ─────────────────────────────────────────────────────────────────────────────

func TestNewPatent_ValidParams_ReturnsPatent(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)

	assert.NotEmpty(t, string(p.ID))
	assert.Equal(t, "CN202010001234A", p.PatentNumber)
	assert.Equal(t, "An OLED emitter material based on carbazole", p.Title)
	assert.Equal(t, "ACME Chemical Corp", p.Applicant)
	assert.Equal(t, ptypes.JurisdictionCN, p.Jurisdiction)
	assert.Equal(t, ptypes.StatusFiled, p.Status)
	assert.False(t, p.CreatedAt.IsZero())
	assert.Equal(t, 1, p.Version)
}

func TestNewPatent_EmptyNumber_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := patent.NewPatent(
		"", "Title", "Abstract", "Applicant",
		ptypes.JurisdictionCN, validFilingDate(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "patent number")
}

func TestNewPatent_EmptyTitle_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := patent.NewPatent(
		"CN202010001234A", "", "Abstract", "Applicant",
		ptypes.JurisdictionCN, validFilingDate(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "title")
}

func TestNewPatent_EmptyAbstract_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := patent.NewPatent(
		"CN202010001234A", "Title", "", "Applicant",
		ptypes.JurisdictionCN, validFilingDate(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abstract")
}

func TestNewPatent_EmptyApplicant_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := patent.NewPatent(
		"CN202010001234A", "Title", "Abstract", "",
		ptypes.JurisdictionCN, validFilingDate(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "applicant")
}

func TestNewPatent_ZeroFilingDate_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := patent.NewPatent(
		"CN202010001234A", "Title", "Abstract", "Applicant",
		ptypes.JurisdictionCN, time.Time{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filing date")
}

func TestNewPatent_InvalidJurisdiction_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := patent.NewPatent(
		"CN202010001234A", "Title", "Abstract", "Applicant",
		ptypes.JurisdictionCode("XX"), validFilingDate(),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "jurisdiction")
}

func TestNewPatent_MismatchedNumberAndJurisdiction_ReturnsError(t *testing.T) {
	t.Parallel()

	// CN number format but US jurisdiction → should fail format check.
	_, err := patent.NewPatent(
		"CN202010001234A", "Title", "Abstract", "Applicant",
		ptypes.JurisdictionUS, validFilingDate(),
	)
	require.Error(t, err)
}

func TestNewPatent_USPatent_ValidParams(t *testing.T) {
	t.Parallel()

	p, err := patent.NewPatent(
		"US10123456B2", "Compound and method", "An invention...", "Corp",
		ptypes.JurisdictionUS, validFilingDate(),
	)
	require.NoError(t, err)
	assert.Equal(t, ptypes.JurisdictionUS, p.Jurisdiction)
}

func TestNewPatent_OtherJurisdiction_SkipsFormatCheck(t *testing.T) {
	t.Parallel()

	p, err := patent.NewPatent(
		"ANYTHING-123", "Title", "Abstract", "Applicant",
		ptypes.JurisdictionOther, validFilingDate(),
	)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAddClaim
// ─────────────────────────────────────────────────────────────────────────────

func TestAddClaim_ValidIndependentClaim_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	cl := patent.Claim{Number: 1, Text: "A compound comprising...", Type: ptypes.ClaimIndependent}

	require.NoError(t, p.AddClaim(cl))
	require.Len(t, p.Claims, 1)
	assert.Equal(t, 1, p.Claims[0].Number)
}

func TestAddClaim_DuplicateNumber_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	cl := patent.Claim{Number: 1, Text: "A compound comprising...", Type: ptypes.ClaimIndependent}

	require.NoError(t, p.AddClaim(cl))
	err := p.AddClaim(cl)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAddClaim_DependentClaimReferencesExistingParent_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	indep := patent.Claim{Number: 1, Text: "A compound...", Type: ptypes.ClaimIndependent}
	dep := patent.Claim{Number: 2, Text: "The compound of claim 1...", Type: ptypes.ClaimDependent, ParentClaimNumber: intPtr(1)}

	require.NoError(t, p.AddClaim(indep))
	require.NoError(t, p.AddClaim(dep))
	assert.Len(t, p.Claims, 2)
}

func TestAddClaim_DependentClaimReferencesNonExistentParent_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	dep := patent.Claim{Number: 2, Text: "Depends on 1...", Type: ptypes.ClaimDependent, ParentClaimNumber: intPtr(1)}

	err := p.AddClaim(dep)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-existent parent")
}

func TestAddClaim_BumpsVersion(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	versionBefore := p.Version
	cl := patent.Claim{Number: 1, Text: "A compound...", Type: ptypes.ClaimIndependent}

	require.NoError(t, p.AddClaim(cl))
	assert.Greater(t, p.Version, versionBefore)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestUpdateStatus
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateStatus_FiledToPublished_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	assert.Equal(t, ptypes.StatusPublished, p.Status)
}

func TestUpdateStatus_FiledToAbandoned_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusAbandoned))
	assert.Equal(t, ptypes.StatusAbandoned, p.Status)
}

func TestUpdateStatus_PublishedToGranted_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))
	assert.Equal(t, ptypes.StatusGranted, p.Status)
}

func TestUpdateStatus_GrantedToExpired_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))
	require.NoError(t, p.UpdateStatus(ptypes.StatusExpired))
	assert.Equal(t, ptypes.StatusExpired, p.Status)
}

func TestUpdateStatus_GrantedToRevoked_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))
	require.NoError(t, p.UpdateStatus(ptypes.StatusRevoked))
	assert.Equal(t, ptypes.StatusRevoked, p.Status)
}

func TestUpdateStatus_GrantedToFiled_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))

	err := p.UpdateStatus(ptypes.StatusFiled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "illegal status transition")
}

func TestUpdateStatus_ExpiredToAny_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))
	require.NoError(t, p.UpdateStatus(ptypes.StatusExpired))

	for _, next := range []ptypes.PatentStatus{
		ptypes.StatusFiled, ptypes.StatusPublished, ptypes.StatusGranted,
		ptypes.StatusRevoked, ptypes.StatusAbandoned,
	} {
		err := p.UpdateStatus(next)
		require.Error(t, err, "expected error transitioning from Expired to %s", next)
	}
}

func TestUpdateStatus_AbandonedToAny_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusAbandoned))

	err := p.UpdateStatus(ptypes.StatusPublished)
	require.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestSetPublicationDate / TestSetGrantDate
// ─────────────────────────────────────────────────────────────────────────────

func TestSetPublicationDate_AfterFilingDate_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	pubDate := validFilingDate().AddDate(0, 18, 0) // 18 months after filing (typical)
	require.NoError(t, p.SetPublicationDate(pubDate))
	require.NotNil(t, p.PublicationDate)
	assert.Equal(t, pubDate, *p.PublicationDate)
}

func TestSetPublicationDate_BeforeFilingDate_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	err := p.SetPublicationDate(validFilingDate().AddDate(0, 0, -1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publication date")
}

func TestSetGrantDate_AfterPublicationDate_Succeeds(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	pubDate := validFilingDate().AddDate(0, 18, 0)
	grantDate := pubDate.AddDate(1, 0, 0)
	require.NoError(t, p.SetPublicationDate(pubDate))
	require.NoError(t, p.SetGrantDate(grantDate))
	require.NotNil(t, p.GrantDate)
	assert.Equal(t, grantDate, *p.GrantDate)
}

func TestSetGrantDate_WithoutPublicationDate_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	err := p.SetGrantDate(validFilingDate().AddDate(2, 0, 0))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publication date")
}

func TestSetGrantDate_BeforePublicationDate_ReturnsError(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	pubDate := validFilingDate().AddDate(0, 18, 0)
	require.NoError(t, p.SetPublicationDate(pubDate))

	err := p.SetGrantDate(pubDate.AddDate(0, 0, -1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "grant date")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCalculateExpiryDate
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateExpiryDate_CN_Returns20YearsFromFiling(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t) // CN, filing date 2020-01-15
	expiry := p.CalculateExpiryDate()

	expected := validFilingDate().AddDate(20, 0, 0) // 2040-01-15
	assert.Equal(t, expected, expiry)
}

func TestCalculateExpiryDate_US_Returns20Years(t *testing.T) {
	t.Parallel()

	filing := time.Date(2018, 6, 1, 0, 0, 0, 0, time.UTC)
	p, err := patent.NewPatent(
		"US10123456B2", "Title", "Abstract", "Corp",
		ptypes.JurisdictionUS, filing,
	)
	require.NoError(t, err)

	expiry := p.CalculateExpiryDate()
	assert.Equal(t, filing.AddDate(20, 0, 0), expiry)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIsExpired
// ─────────────────────────────────────────────────────────────────────────────

func TestIsExpired_RecentFiling_ReturnsFalse(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t) // filed 2020, expires 2040
	assert.False(t, p.IsExpired())
}

func TestIsExpired_OldFiling_ReturnsTrue(t *testing.T) {
	t.Parallel()

	ancientDate := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	p, err := patent.NewPatent(
		"CN199010001234A", "Old patent", "Abstract", "Corp",
		ptypes.JurisdictionCN, ancientDate,
	)
	require.NoError(t, err)
	// Statutory term: 1990 + 20 = 2010, well in the past.
	assert.True(t, p.IsExpired())
}

func TestIsExpired_StatusExpired_ReturnsTrue(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))
	require.NoError(t, p.UpdateStatus(ptypes.StatusExpired))

	assert.True(t, p.IsExpired())
}

func TestIsExpired_StatusRevoked_ReturnsTrue(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))
	require.NoError(t, p.UpdateStatus(ptypes.StatusRevoked))

	assert.True(t, p.IsExpired())
}

func TestIsExpired_StatusAbandoned_ReturnsTrue(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.UpdateStatus(ptypes.StatusAbandoned))
	assert.True(t, p.IsExpired())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetIndependentClaims / TestGetDependentClaims
// ─────────────────────────────────────────────────────────────────────────────

func TestGetIndependentClaims_ReturnsOnlyTopLevelClaims(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.AddClaim(patent.Claim{Number: 1, Text: "Indep 1", Type: ptypes.ClaimIndependent}))
	require.NoError(t, p.AddClaim(patent.Claim{Number: 2, Text: "Dep of 1", Type: ptypes.ClaimDependent, ParentClaimNumber: intPtr(1)}))
	require.NoError(t, p.AddClaim(patent.Claim{Number: 3, Text: "Indep 3", Type: ptypes.ClaimIndependent}))

	indep := p.GetIndependentClaims()
	require.Len(t, indep, 2)
	nums := []int{indep[0].Number, indep[1].Number}
	assert.Contains(t, nums, 1)
	assert.Contains(t, nums, 3)
}

func TestGetDependentClaims_ReturnsDirectDependents(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.AddClaim(patent.Claim{Number: 1, Text: "Indep", Type: ptypes.ClaimIndependent}))
	require.NoError(t, p.AddClaim(patent.Claim{Number: 2, Text: "Dep of 1", Type: ptypes.ClaimDependent, ParentClaimNumber: intPtr(1)}))
	require.NoError(t, p.AddClaim(patent.Claim{Number: 3, Text: "Dep of 1", Type: ptypes.ClaimDependent, ParentClaimNumber: intPtr(1)}))
	require.NoError(t, p.AddClaim(patent.Claim{Number: 4, Text: "Dep of 2", Type: ptypes.ClaimDependent, ParentClaimNumber: intPtr(2)}))

	deps := p.GetDependentClaims(1)
	require.Len(t, deps, 2)
	nums := []int{deps[0].Number, deps[1].Number}
	assert.Contains(t, nums, 2)
	assert.Contains(t, nums, 3)
	// Claim 4 depends on 2, not directly on 1.
	assert.NotContains(t, nums, 4)
}

func TestGetDependentClaims_NoDependents_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	require.NoError(t, p.AddClaim(patent.Claim{Number: 1, Text: "Indep", Type: ptypes.ClaimIndependent}))

	deps := p.GetDependentClaims(1)
	assert.Empty(t, deps)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestEvents
// ─────────────────────────────────────────────────────────────────────────────

func TestEvents_NewPatent_ContainsPatentCreatedEvent(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	evts := p.Events()

	require.Len(t, evts, 1)
	assert.Equal(t, patent.PatentCreatedEventName, evts[0].EventName())
}

func TestEvents_AfterStatusChange_ContainsStatusChangedEvent(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	_ = p.Events() // drain the PatentCreated event

	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	evts := p.Events()

	require.Len(t, evts, 1)
	assert.Equal(t, patent.PatentStatusChangedEventName, evts[0].EventName())
}

func TestEvents_CallClearsBuffer(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	first := p.Events()
	require.NotEmpty(t, first, "first Events() call should return the PatentCreated event")

	second := p.Events()
	assert.Empty(t, second, "second Events() call should return an empty slice")
}

func TestEvents_MultipleStatusChanges_EachProducesEvent(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	_ = p.Events() // drain PatentCreated

	require.NoError(t, p.UpdateStatus(ptypes.StatusPublished))
	require.NoError(t, p.UpdateStatus(ptypes.StatusGranted))
	evts := p.Events()

	require.Len(t, evts, 2)
	assert.Equal(t, patent.PatentStatusChangedEventName, evts[0].EventName())
	assert.Equal(t, patent.PatentStatusChangedEventName, evts[1].EventName())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestToDTO / TestPatentFromDTO round-trip
// ─────────────────────────────────────────────────────────────────────────────

func TestToDTO_ContainsCorrectFields(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	p.Inventors = []string{"Alice", "Bob"}
	p.IPCCodes = []string{"C07D 403/04"}
	p.FamilyID = "FAM-001"

	dto := p.ToDTO()

	assert.Equal(t, p.ID, dto.ID)
	assert.Equal(t, p.PatentNumber, dto.PatentNumber)
	assert.Equal(t, p.Title, dto.Title)
	assert.Equal(t, p.Abstract, dto.Abstract)
	assert.Equal(t, p.Applicant, dto.Applicant)
	assert.Equal(t, p.Inventors, dto.Inventors)
	assert.Equal(t, p.Status, dto.Status)
	assert.Equal(t, p.Jurisdiction, dto.Jurisdiction)
	assert.Equal(t, p.IPCCodes, dto.IPCCodes)
	assert.Equal(t, p.FamilyID, dto.FamilyID)
}

func TestPatentFromDTO_RoundTrip_FieldsMatch(t *testing.T) {
	t.Parallel()

	original := newValidPatent(t)
	original.Inventors = []string{"Alice"}
	require.NoError(t, original.AddClaim(patent.Claim{
		Number: 1, Text: "A compound...", Type: ptypes.ClaimIndependent,
	}))
	require.NoError(t, original.UpdateStatus(ptypes.StatusPublished))
	_ = original.Events() // drain events — not part of DTO

	dto := original.ToDTO()
	reconstructed := patent.PatentFromDTO(dto)

	assert.Equal(t, original.ID, reconstructed.ID)
	assert.Equal(t, original.PatentNumber, reconstructed.PatentNumber)
	assert.Equal(t, original.Title, reconstructed.Title)
	assert.Equal(t, original.Abstract, reconstructed.Abstract)
	assert.Equal(t, original.Applicant, reconstructed.Applicant)
	assert.Equal(t, original.Inventors, reconstructed.Inventors)
	assert.Equal(t, original.Status, reconstructed.Status)
	assert.Equal(t, original.Jurisdiction, reconstructed.Jurisdiction)
	assert.Equal(t, original.Version, reconstructed.Version)
	require.Len(t, reconstructed.Claims, 1)
	assert.Equal(t, 1, reconstructed.Claims[0].Number)
}

func TestPatentFromDTO_EventsBufferIsEmpty(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	dto := p.ToDTO()
	reconstructed := patent.PatentFromDTO(dto)

	// Rehydration must not inject any spurious domain events.
	evts := reconstructed.Events()
	assert.Empty(t, evts)
}

func TestToDTO_PriorityEntries_Preserved(t *testing.T) {
	t.Parallel()

	p := newValidPatent(t)
	p.Priority = []patent.Priority{
		{Country: "US", Number: "US62123456", Date: validFilingDate().AddDate(0, -6, 0)},
	}

	dto := p.ToDTO()
	require.Len(t, dto.Priority, 1)
	assert.Equal(t, "US", dto.Priority[0].Country)
	assert.Equal(t, "US62123456", dto.Priority[0].Number)
}

//Personal.AI order the ending
