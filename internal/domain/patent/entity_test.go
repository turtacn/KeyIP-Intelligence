package patent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPatentStatus_String(t *testing.T) {
	tests := []struct {
		val      PatentStatus
		expected string
	}{
		{PatentStatusDraft, "draft"},
		{PatentStatusFiled, "filed"},
		{PatentStatusPublished, "published"},
		{PatentStatusUnderExamination, "under_examination"},
		{PatentStatusGranted, "granted"},
		{PatentStatusRejected, "rejected"},
		{PatentStatusWithdrawn, "withdrawn"},
		{PatentStatusExpired, "expired"},
		{PatentStatusInvalidated, "invalidated"},
		{PatentStatusLapsed, "lapsed"},
		{PatentStatusUnknown, "unknown"},
		{PatentStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.val.String())
		})
	}
}

func TestPatentStatus_IsValid(t *testing.T) {
	assert.True(t, PatentStatusFiled.IsValid())
	assert.True(t, PatentStatusLapsed.IsValid())
	assert.False(t, PatentStatus(99).IsValid())
}

func TestPatentStatus_IsActive(t *testing.T) {
	assert.True(t, PatentStatusFiled.IsActive())
	assert.True(t, PatentStatusPublished.IsActive())
	assert.True(t, PatentStatusUnderExamination.IsActive())
	assert.True(t, PatentStatusGranted.IsActive())
	assert.False(t, PatentStatusDraft.IsActive())
	assert.False(t, PatentStatusExpired.IsActive())
}

func TestPatentOffice_IsValid(t *testing.T) {
	assert.True(t, OfficeCNIPA.IsValid())
	assert.True(t, OfficeUSPTO.IsValid())
	assert.True(t, OfficeEPO.IsValid())
	assert.True(t, OfficeJPO.IsValid())
	assert.True(t, OfficeKIPO.IsValid())
	assert.True(t, OfficeWIPO.IsValid())
	assert.False(t, PatentOffice("INVALID").IsValid())
}

func TestPatentDate_RemainingLifeYears(t *testing.T) {
	now := time.Now().UTC()

	// Future expiry
	expiry := now.AddDate(10, 0, 0)
	d := PatentDate{ExpiryDate: &expiry}
	years := d.RemainingLifeYears()
	assert.InDelta(t, 10.0, years, 0.1)

	// Past expiry
	expiryPast := now.AddDate(-1, 0, 0)
	dPast := PatentDate{ExpiryDate: &expiryPast}
	assert.Equal(t, 0.0, dPast.RemainingLifeYears())

	// No expiry
	dNil := PatentDate{ExpiryDate: nil}
	assert.Equal(t, 0.0, dNil.RemainingLifeYears())
}

func TestPatentDate_Validate(t *testing.T) {
	now := time.Now().UTC()
	d := PatentDate{FilingDate: &now}
	assert.NoError(t, d.Validate())

	dEmpty := PatentDate{}
	assert.Error(t, dEmpty.Validate())
}

func TestNewPatent_Success(t *testing.T) {
	now := time.Now().UTC()
	p, err := NewPatent("CN123456", "Test Patent", OfficeCNIPA, now)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "CN123456", p.PatentNumber)
	assert.Equal(t, "Test Patent", p.Title)
	assert.Equal(t, OfficeCNIPA, p.Office)
	assert.Equal(t, PatentStatusFiled, p.Status)
	assert.Equal(t, 1, p.Version)
	assert.False(t, p.CreatedAt.IsZero())
	assert.False(t, p.UpdatedAt.IsZero())
	assert.NotNil(t, p.Dates.FilingDate)
}

func TestNewPatent_EmptyNumber(t *testing.T) {
	now := time.Now().UTC()
	_, err := NewPatent("", "Title", OfficeCNIPA, now)
	assert.Error(t, err)
}

func TestNewPatent_EmptyTitle(t *testing.T) {
	now := time.Now().UTC()
	_, err := NewPatent("CN123", "", OfficeCNIPA, now)
	assert.Error(t, err)
}

func TestNewPatent_InvalidOffice(t *testing.T) {
	now := time.Now().UTC()
	_, err := NewPatent("CN123", "Title", PatentOffice("XX"), now)
	assert.Error(t, err)
}

func TestPatent_Validate(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	assert.NoError(t, p.Validate())

	p.PatentNumber = ""
	assert.Error(t, p.Validate())
}

func TestPatent_Publish(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	now := time.Now().UTC()

	err := p.Publish(now)
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusPublished, p.Status)
	assert.Equal(t, &now, p.Dates.PublicationDate)
	assert.Equal(t, 2, p.Version)
}

func TestPatent_Publish_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted // Manually force state
	err := p.Publish(time.Now())
	assert.Error(t, err)
}

func TestPatent_EnterExamination(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())

	err := p.EnterExamination()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusUnderExamination, p.Status)
}

func TestPatent_EnterExamination_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.EnterExamination()
	assert.Error(t, err)
}

func TestPatent_Grant(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())
	p.EnterExamination()

	grantDate := time.Now().UTC()
	expiryDate := grantDate.AddDate(20, 0, 0)

	err := p.Grant(grantDate, expiryDate)
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusGranted, p.Status)
	assert.Equal(t, &grantDate, p.Dates.GrantDate)
	assert.Equal(t, &expiryDate, p.Dates.ExpiryDate)
}

func TestPatent_Grant_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.Grant(time.Now(), time.Now())
	assert.Error(t, err)
}

func TestPatent_Reject(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())
	p.EnterExamination()

	err := p.Reject()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusRejected, p.Status)
}

func TestPatent_Reject_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.Reject()
	assert.Error(t, err)
}

func TestPatent_Withdraw(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.Withdraw()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusWithdrawn, p.Status)
}

func TestPatent_Withdraw_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted
	err := p.Withdraw()
	assert.Error(t, err)
}

func TestPatent_Expire(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted
	err := p.Expire()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusExpired, p.Status)
}

func TestPatent_Expire_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.Expire()
	assert.Error(t, err)
}

func TestPatent_Invalidate(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted
	err := p.Invalidate()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusInvalidated, p.Status)
}

func TestPatent_Invalidate_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.Invalidate()
	assert.Error(t, err)
}

func TestPatent_Lapse(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted
	err := p.Lapse()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusLapsed, p.Status)
}

func TestPatent_Lapse_InvalidState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.Lapse()
	assert.Error(t, err)
}

func TestPatent_AddMolecule(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.AddMolecule("MOL1")
	assert.Contains(t, p.MoleculeIDs, "MOL1")
	assert.Equal(t, 2, p.Version)
}

func TestPatent_AddCitation(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.AddCitation("US123")
	assert.Contains(t, p.Cites, "US123")
	assert.Equal(t, 2, p.Version)
}

func TestPatent_AddCitedBy(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.AddCitedBy("US999")
	assert.Contains(t, p.CitedBy, "US999")
	assert.Equal(t, 2, p.Version)
}

func TestPatent_SetClaims(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	claims := ClaimSet{
		{Number: 1, Text: "Claim 1", Type: ClaimTypeIndependent},
	}
	p.SetClaims(claims)
	assert.Equal(t, 1, len(p.Claims))
	assert.Equal(t, 2, p.Version)
}

func TestPatent_HelperGetters(t *testing.T) {
	now := time.Now().UTC()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, now)
	p.KeyIPTechCodes = []string{"TECH1"}
	p.Metadata = map[string]any{"value_score": 8.5}
	p.AssigneeName = "Acme Corp"
	p.AddMolecule("MOL1")

	assert.Equal(t, 0, p.ClaimCount())
	assert.Equal(t, "TECH1", p.GetPrimaryTechDomain())
	assert.Equal(t, 8.5, p.GetValueScore())
	assert.Equal(t, &now, p.GetFilingDate())
	assert.Equal(t, "filed", p.GetLegalStatus())
	assert.Equal(t, "Acme Corp", p.GetAssignee())
	assert.Equal(t, []string{"MOL1"}, p.GetMoleculeIDs())
	assert.NotEmpty(t, p.GetID())
	assert.Equal(t, "CN123", p.GetPatentNumber())
}

func TestPatent_AnalyzeClaims(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Claims = ClaimSet{
		{Number: 1, Type: ClaimTypeIndependent},
		{Number: 2, Type: ClaimTypeDependent, DependsOn: []int{1}},
	}

	tree := p.AnalyzeClaims()
	assert.NotNil(t, tree)
	assert.Equal(t, 1, len(tree.Roots))
	assert.Equal(t, 2, len(tree.AllNodes))
	assert.Equal(t, 1, tree.Roots[0].Claim.Number)
	assert.Equal(t, 1, len(tree.Roots[0].Children))
	assert.Equal(t, 2, tree.Roots[0].Children[0].Claim.Number)
}
