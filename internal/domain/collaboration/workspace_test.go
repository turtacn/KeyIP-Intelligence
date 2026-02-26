package collaboration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWorkspace_Success(t *testing.T) {
	ws, err := NewWorkspace("My Workspace", "owner1")
	assert.NoError(t, err)
	assert.NotEmpty(t, ws.ID)
	assert.Equal(t, "My Workspace", ws.Name)
	assert.Equal(t, "owner1", ws.OwnerID)
	assert.Equal(t, PlanFree, ws.Plan)
	assert.Equal(t, 1, ws.MemberCount)
}

func TestPlanLimits(t *testing.T) {
	free := PlanLimits(PlanFree)
	assert.Equal(t, 5, free.MaxPortfolios)
	assert.Equal(t, 3, free.MaxMembers)

	pro := PlanLimits(PlanPro)
	assert.Equal(t, 50, pro.MaxPortfolios)

	ent := PlanLimits(PlanEnterprise)
	assert.Equal(t, -1, ent.MaxPortfolios)
}

func TestWorkspace_UpgradePlan(t *testing.T) {
	ws, _ := NewWorkspace("ws", "owner")

	err := ws.UpgradePlan(PlanPro)
	assert.NoError(t, err)
	assert.Equal(t, PlanPro, ws.Plan)
	assert.Equal(t, 50, ws.Settings.MaxPortfolios)

	err = ws.UpgradePlan(PlanFree)
	assert.Error(t, err) // Downgrade denied
}

func TestWorkspace_CanAdd(t *testing.T) {
	ws, _ := NewWorkspace("ws", "owner")
	// Free plan: max 5 portfolios, 3 members

	assert.True(t, ws.CanAddMember()) // 1 < 3
	ws.IncrementMemberCount() // 2
	ws.IncrementMemberCount() // 3
	assert.False(t, ws.CanAddMember()) // 3 !< 3

	assert.True(t, ws.CanAddPortfolio())
}

func TestWorkspace_StatusTransitions(t *testing.T) {
	ws, _ := NewWorkspace("ws", "owner")

	err := ws.Suspend("payment failed")
	assert.NoError(t, err)
	assert.Equal(t, WorkspaceStatusSuspended, ws.Status)

	err = ws.Reactivate()
	assert.NoError(t, err)
	assert.Equal(t, WorkspaceStatusActive, ws.Status)

	err = ws.Archive()
	assert.NoError(t, err)
	assert.Equal(t, WorkspaceStatusArchived, ws.Status)
}

//Personal.AI order the ending
