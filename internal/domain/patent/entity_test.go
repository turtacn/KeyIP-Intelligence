package patent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatentStatus_String(t *testing.T) {
	assert.Equal(t, "Draft", PatentStatusDraft.String())
	assert.Equal(t, "Filed", PatentStatusFiled.String())
	assert.Equal(t, "Published", PatentStatusPublished.String())
	assert.Equal(t, "Unknown", PatentStatus(255).String())
}

func TestPatentStatus_IsValid(t *testing.T) {
	assert.True(t, PatentStatusFiled.IsValid())
	assert.False(t, PatentStatus(255).IsValid())
}

func TestPatentStatus_IsActive(t *testing.T) {
	assert.True(t, PatentStatusFiled.IsActive())
	assert.True(t, PatentStatusGranted.IsActive())
	assert.False(t, PatentStatusRejected.IsActive())
	assert.False(t, PatentStatusDraft.IsActive())
}

func TestPatentStatus_IsTerminal(t *testing.T) {
	assert.True(t, PatentStatusRejected.IsTerminal())
	assert.True(t, PatentStatusExpired.IsTerminal())
	assert.False(t, PatentStatusFiled.IsTerminal())
}

func TestPatentOffice_IsValid(t *testing.T) {
	assert.True(t, OfficeCNIPA.IsValid())
	assert.False(t, PatentOffice("INVALID").IsValid())
}

func TestIPCClassification_Validate_Success(t *testing.T) {
	ipc := IPCClassification{Full: "C09K 11/06"}
	assert.NoError(t, ipc.Validate())
}

func TestIPCClassification_Validate_EmptyFull(t *testing.T) {
	ipc := IPCClassification{Full: ""}
	assert.Error(t, ipc.Validate())
}

func TestIPCClassification_Validate_InvalidFormat(t *testing.T) {
	ipc := IPCClassification{Full: "INVALID"}
	assert.Error(t, ipc.Validate())
}

func TestPatentDate_RemainingLifeYears_Future(t *testing.T) {
	future := time.Now().AddDate(10, 0, 0)
	d := PatentDate{ExpiryDate: &future}
	life := d.RemainingLifeYears()
	assert.Greater(t, life, 9.9)
	assert.Less(t, life, 10.1)
}

func TestPatentDate_RemainingLifeYears_Past(t *testing.T) {
	past := time.Now().AddDate(-1, 0, 0)
	d := PatentDate{ExpiryDate: &past}
	assert.Equal(t, 0.0, d.RemainingLifeYears())
}

func TestPatentDate_RemainingLifeYears_NilExpiry(t *testing.T) {
	d := PatentDate{ExpiryDate: nil}
	assert.Equal(t, 0.0, d.RemainingLifeYears())
}

func TestPatentDate_Validate_Success(t *testing.T) {
	filing := time.Now().AddDate(-5, 0, 0)
	pub := filing.AddDate(0, 6, 0)
	grant := pub.AddDate(1, 0, 0)
	expiry := grant.AddDate(15, 0, 0)
	d := PatentDate{
		FilingDate:      &filing,
		PublicationDate: &pub,
		GrantDate:       &grant,
		ExpiryDate:      &expiry,
	}
	assert.NoError(t, d.Validate())
}

func TestPatentDate_Validate_PublicationBeforeFiling(t *testing.T) {
	filing := time.Now()
	pub := filing.AddDate(0, -1, 0)
	d := PatentDate{FilingDate: &filing, PublicationDate: &pub}
	assert.Error(t, d.Validate())
}

func TestPatentDate_Validate_GrantBeforePublication(t *testing.T) {
	filing := time.Now().AddDate(-2, 0, 0)
	pub := filing.AddDate(0, 6, 0)
	grant := pub.AddDate(0, -1, 0)
	d := PatentDate{FilingDate: &filing, PublicationDate: &pub, GrantDate: &grant}
	assert.Error(t, d.Validate())
}

func TestNewPatent_Success(t *testing.T) {
	p, err := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	require.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "CN123", p.PatentNumber)
	assert.Equal(t, PatentStatusFiled, p.Status)
	assert.Equal(t, 1, p.Version)
	assert.NotEmpty(t, p.ID)
}

func TestNewPatent_EmptyPatentNumber(t *testing.T) {
	_, err := NewPatent("", "Title", OfficeCNIPA, time.Now())
	assert.Error(t, err)
}

func TestNewPatent_InvalidOffice(t *testing.T) {
	_, err := NewPatent("CN123", "Title", PatentOffice("INVALID"), time.Now())
	assert.Error(t, err)
}

func TestPatent_Validate_FullyPopulated(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Applicants = []Applicant{{Name: "App1"}}
	p.Inventors = []Inventor{{Name: "Inv1"}}
	assert.NoError(t, p.Validate())
}

func TestPatent_Validate_NoApplicants(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Inventors = []Inventor{{Name: "Inv1"}}
	assert.Error(t, p.Validate())
}

func TestPatent_Publish_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	pubDate := time.Now().AddDate(0, 1, 0)
	err := p.Publish(pubDate)
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusPublished, p.Status)
	assert.Equal(t, pubDate, *p.Dates.PublicationDate)
}

func TestPatent_Publish_FromWrongState(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted // Simulate wrong state
	err := p.Publish(time.Now())
	assert.Error(t, err)
}

func TestPatent_EnterExamination_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())
	err := p.EnterExamination()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusUnderExamination, p.Status)
}

func TestPatent_Grant_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())
	p.EnterExamination()
	grantDate := time.Now()
	expiryDate := grantDate.AddDate(20, 0, 0)
	err := p.Grant(grantDate, expiryDate)
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusGranted, p.Status)
}

func TestPatent_Reject_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Publish(time.Now())
	p.EnterExamination()
	err := p.Reject()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusRejected, p.Status)
}

func TestPatent_Withdraw_FromFiled(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.Withdraw()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusWithdrawn, p.Status)
}

func TestPatent_Expire_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted
	err := p.Expire()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusExpired, p.Status)
}

func TestPatent_Invalidate_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted
	err := p.Invalidate()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusInvalidated, p.Status)
}

func TestPatent_Lapse_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.Status = PatentStatusGranted
	err := p.Lapse()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusLapsed, p.Status)
}

func TestPatent_StateTransition_FullLifecycle(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	require.Equal(t, PatentStatusFiled, p.Status)

	require.NoError(t, p.Publish(time.Now()))
	require.Equal(t, PatentStatusPublished, p.Status)

	require.NoError(t, p.EnterExamination())
	require.Equal(t, PatentStatusUnderExamination, p.Status)

	require.NoError(t, p.Grant(time.Now(), time.Now().AddDate(20, 0, 0)))
	require.Equal(t, PatentStatusGranted, p.Status)

	require.NoError(t, p.Expire())
	require.Equal(t, PatentStatusExpired, p.Status)
}

func TestPatent_AddMolecule_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	err := p.AddMolecule("M1")
	assert.NoError(t, err)
	assert.Contains(t, p.MoleculeIDs, "M1")
}

func TestPatent_AddMolecule_Duplicate(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.AddMolecule("M1")
	err := p.AddMolecule("M1")
	assert.Error(t, err)
}

func TestPatent_RemoveMolecule_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.AddMolecule("M1")
	err := p.RemoveMolecule("M1")
	assert.NoError(t, err)
	assert.NotContains(t, p.MoleculeIDs, "M1")
}

func TestPatent_SetClaims_Success(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	c, err := NewClaim(1, "A generic text for claim", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c}
	err = p.SetClaims(cs)
	assert.NoError(t, err)
	assert.NotEmpty(t, p.Claims)
}

func TestPatent_ClaimCount(t *testing.T) {
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	c1, err := NewClaim(1, "A generic text for claim", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2, err := NewClaim(2, "A generic text for claim", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2.SetDependencies([]int{1})
	p.SetClaims(ClaimSet{*c1, *c2})
	assert.Equal(t, 2, p.ClaimCount())
	assert.Equal(t, 1, p.IndependentClaimCount())
}

//Personal.AI order the ending
