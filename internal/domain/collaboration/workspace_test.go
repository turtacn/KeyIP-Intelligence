package collaboration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWorkspace_Success(t *testing.T) {
	w, err := NewWorkspace("My Workspace", "owner1")
	assert.NoError(t, err)
	assert.NotNil(t, w)
	assert.NotEmpty(t, w.ID)
	assert.Equal(t, "My Workspace", w.Name)
	assert.Equal(t, "owner1", w.OwnerID)
	assert.Equal(t, WorkspaceStatusActive, w.Status)
	assert.Equal(t, PlanFree, w.Plan)
	assert.Equal(t, 1, w.MemberCount)
}

func TestNewWorkspace_EmptyName(t *testing.T) {
	_, err := NewWorkspace("", "owner1")
	assert.Error(t, err)
}

func TestNewWorkspace_NameTooLong(t *testing.T) {
	name := ""
	for i := 0; i < 101; i++ {
		name += "a"
	}
	_, err := NewWorkspace(name, "owner1")
	assert.Error(t, err)
}

func TestNewWorkspace_EmptyOwnerID(t *testing.T) {
	_, err := NewWorkspace("My Workspace", "")
	assert.Error(t, err)
}

func TestNewWorkspace_DefaultSettings(t *testing.T) {
	w, _ := NewWorkspace("My Workspace", "owner1")
	assert.Equal(t, "USD", w.Settings.DefaultCurrency)
	assert.Equal(t, "US", w.Settings.DefaultJurisdiction)
	assert.Equal(t, []int{7, 30, 60}, w.Settings.AnnuityReminderDays)
	assert.Equal(t, 5, w.Settings.MaxPortfolios)
	assert.Equal(t, 3, w.Settings.MaxMembers)
}

func TestDefaultWorkspaceSettings(t *testing.T) {
	s := DefaultWorkspaceSettings()
	assert.Equal(t, "USD", s.DefaultCurrency)
	assert.Equal(t, 5, s.MaxPortfolios)
}

func TestPlanLimits_Free(t *testing.T) {
	s := PlanLimits(PlanFree)
	assert.Equal(t, 5, s.MaxPortfolios)
	assert.Equal(t, 3, s.MaxMembers)
	assert.False(t, s.AIFeaturesEnabled)
}

func TestPlanLimits_Pro(t *testing.T) {
	s := PlanLimits(PlanPro)
	assert.Equal(t, 50, s.MaxPortfolios)
	assert.Equal(t, 20, s.MaxMembers)
	assert.True(t, s.AIFeaturesEnabled)
}

func TestPlanLimits_Enterprise(t *testing.T) {
	s := PlanLimits(PlanEnterprise)
	assert.Equal(t, -1, s.MaxPortfolios)
	assert.Equal(t, -1, s.MaxMembers)
	assert.True(t, s.AIFeaturesEnabled)
}

func TestWorkspace_UpdateName_Success(t *testing.T) {
	w, _ := NewWorkspace("Old Name", "owner1")
	err := w.UpdateName("New Name")
	assert.NoError(t, err)
	assert.Equal(t, "New Name", w.Name)
}

func TestWorkspace_UpdateName_Empty(t *testing.T) {
	w, _ := NewWorkspace("Old Name", "owner1")
	err := w.UpdateName("")
	assert.Error(t, err)
}

func TestWorkspace_UpdateDescription_Success(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	err := w.UpdateDescription("A new description")
	assert.NoError(t, err)
	assert.Equal(t, "A new description", w.Description)
}

func TestWorkspace_UpdateSettings_Success(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	newSettings := DefaultWorkspaceSettings()
	newSettings.DefaultCurrency = "EUR"
	newSettings.MaxPortfolios = 100 // Should be ignored
	err := w.UpdateSettings(newSettings)
	assert.NoError(t, err)
	assert.Equal(t, "EUR", w.Settings.DefaultCurrency)
	assert.Equal(t, 5, w.Settings.MaxPortfolios) // Still 5 because of plan
}

func TestWorkspace_UpgradePlan_FreeToPro(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	err := w.UpgradePlan(PlanPro)
	assert.NoError(t, err)
	assert.Equal(t, PlanPro, w.Plan)
	assert.Equal(t, 50, w.Settings.MaxPortfolios)
	assert.Equal(t, 20, w.Settings.MaxMembers)
}

func TestWorkspace_UpgradePlan_Downgrade(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.UpgradePlan(PlanPro)
	err := w.UpgradePlan(PlanFree)
	assert.Error(t, err)
}

func TestWorkspace_AddPortfolio_Success(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	err := w.AddPortfolio("port1")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(w.PortfolioIDs))
}

func TestWorkspace_AddPortfolio_Duplicate(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.AddPortfolio("port1")
	err := w.AddPortfolio("port1")
	assert.Error(t, err)
}

func TestWorkspace_AddPortfolio_ExceedsLimit(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.Settings.MaxPortfolios = 1
	w.AddPortfolio("port1")
	err := w.AddPortfolio("port2")
	assert.Error(t, err)
}

func TestWorkspace_AddPortfolio_UnlimitedPlan(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.UpgradePlan(PlanPro)
	w.UpgradePlan(PlanEnterprise)
	for i := 0; i < 10; i++ {
		err := w.AddPortfolio(string(rune(i)))
		assert.NoError(t, err)
	}
}

func TestWorkspace_RemovePortfolio_Success(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.AddPortfolio("port1")
	err := w.RemovePortfolio("port1")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(w.PortfolioIDs))
}

func TestWorkspace_RemovePortfolio_NotFound(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	err := w.RemovePortfolio("port1")
	assert.Error(t, err)
}

func TestWorkspace_IncrementMemberCount(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.IncrementMemberCount()
	assert.Equal(t, 2, w.MemberCount)
}

func TestWorkspace_DecrementMemberCount_MinimumOne(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.DecrementMemberCount()
	assert.Equal(t, 1, w.MemberCount)
}

func TestWorkspace_CanAddMember_AtLimit(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.Settings.MaxMembers = 1
	assert.False(t, w.CanAddMember())
}

func TestWorkspace_Suspend_Success(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	err := w.Suspend("reason")
	assert.NoError(t, err)
	assert.Equal(t, WorkspaceStatusSuspended, w.Status)
}

func TestWorkspace_Archive_FromSuspended(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.Suspend("reason")
	err := w.Archive()
	assert.NoError(t, err)
	assert.Equal(t, WorkspaceStatusArchived, w.Status)
}

func TestWorkspace_TransferOwnership_Success(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	err := w.TransferOwnership("owner2")
	assert.NoError(t, err)
	assert.Equal(t, "owner2", w.OwnerID)
}

func TestWorkspace_Validate_Success(t *testing.T) {
	w, _ := NewWorkspace("Name", "owner1")
	w.Slug = "name"
	err := w.Validate()
	assert.NoError(t, err)
}

//Personal.AI order the ending
