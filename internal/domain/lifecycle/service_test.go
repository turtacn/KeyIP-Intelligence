// Package lifecycle_test provides unit tests for the lifecycle service.
package lifecycle_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestCreateLifecycle
// ─────────────────────────────────────────────────────────────────────────────

func TestService_CreateLifecycle_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	cmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	lc, err := svc.CreateLifecycle(ctx, cmd)
	require.NoError(t, err)
	require.NotNil(t, lc)
	assert.Equal(t, cmd.PatentNumber, lc.PatentNumber)
	assert.Equal(t, cmd.Jurisdiction, lc.Jurisdiction)
	assert.Equal(t, "pending", lc.LegalStatus.Current)
}

func TestService_CreateLifecycle_WithGrantDate(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	grantDate := time.Date(2022, 6, 1, 0, 0, 0, 0, time.UTC)
	cmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		GrantDate:    &grantDate,
	}

	lc, err := svc.CreateLifecycle(ctx, cmd)
	require.NoError(t, err)
	assert.Equal(t, "granted", lc.LegalStatus.Current)
	assert.NotNil(t, lc.GrantDate)
}

func TestService_CreateLifecycle_DuplicatePatentID(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	patentID := common.NewID()
	cmd := lifecycle.CreateLifecycleCommand{
		PatentID:     patentID,
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := svc.CreateLifecycle(ctx, cmd)
	require.NoError(t, err)

	// Try to create again with same patent ID.
	_, err = svc.CreateLifecycle(ctx, cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetLifecycle
// ─────────────────────────────────────────────────────────────────────────────

func TestService_GetLifecycle_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	cmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	created, _ := svc.CreateLifecycle(ctx, cmd)

	retrieved, err := svc.GetLifecycle(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
}

func TestService_GetLifecycleByPatentNumber_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	cmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	created, _ := svc.CreateLifecycle(ctx, cmd)

	retrieved, err := svc.GetLifecycleByPatentNumber(ctx, "CN202010000001A")
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAddDeadline
// ─────────────────────────────────────────────────────────────────────────────

func TestService_AddDeadline_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	// Create lifecycle.
	createCmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	lc, _ := svc.CreateLifecycle(ctx, createCmd)
	initialDeadlineCount := len(lc.Deadlines)

	// Add deadline.
	deadlineCmd := lifecycle.AddDeadlineCommand{
		Type:        lifecycle.DeadlineOAResponse,
		DueDate:     time.Now().UTC().AddDate(0, 0, 30),
		Priority:    lifecycle.PriorityHigh,
		Description: "Respond to office action",
	}
	err := svc.AddDeadline(ctx, lc.ID, deadlineCmd)
	require.NoError(t, err)

	// Verify.
	updated, _ := svc.GetLifecycle(ctx, lc.ID)
	assert.Greater(t, len(updated.Deadlines), initialDeadlineCount)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCompleteDeadline
// ─────────────────────────────────────────────────────────────────────────────

func TestService_CompleteDeadline_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	createCmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	lc, _ := svc.CreateLifecycle(ctx, createCmd)

	deadlineCmd := lifecycle.AddDeadlineCommand{
		Type:        lifecycle.DeadlineOAResponse,
		DueDate:     time.Now().UTC().AddDate(0, 0, 30),
		Priority:    lifecycle.PriorityHigh,
		Description: "Test deadline",
	}
	_ = svc.AddDeadline(ctx, lc.ID, deadlineCmd)

	updated, _ := svc.GetLifecycle(ctx, lc.ID)
	deadlineID := updated.Deadlines[len(updated.Deadlines)-1].ID

	err := svc.CompleteDeadline(ctx, lc.ID, deadlineID)
	require.NoError(t, err)

	final, _ := svc.GetLifecycle(ctx, lc.ID)
	for _, d := range final.Deadlines {
		if d.ID == deadlineID {
			assert.True(t, d.Completed)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestExtendDeadline
// ─────────────────────────────────────────────────────────────────────────────

func TestService_ExtendDeadline_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	createCmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	lc, _ := svc.CreateLifecycle(ctx, createCmd)

	dueDate := time.Now().UTC().AddDate(0, 0, 30)
	deadlineCmd := lifecycle.AddDeadlineCommand{
		Type:        lifecycle.DeadlineOAResponse,
		DueDate:     dueDate,
		Priority:    lifecycle.PriorityHigh,
		Description: "Test deadline",
	}
	_ = svc.AddDeadline(ctx, lc.ID, deadlineCmd)

	updated, _ := svc.GetLifecycle(ctx, lc.ID)
	deadlineID := updated.Deadlines[len(updated.Deadlines)-1].ID

	// Enable extension manually (normally set by domain rules).
	for i := range updated.Deadlines {
		if updated.Deadlines[i].ID == deadlineID {
			updated.Deadlines[i].ExtensionAvailable = true
			updated.Deadlines[i].MaxExtensionDays = 30
		}
	}
	_ = repo.Save(ctx, updated)

	err := svc.ExtendDeadline(ctx, lc.ID, deadlineID, 15)
	require.NoError(t, err)

	final, _ := svc.GetLifecycle(ctx, lc.ID)
	for _, d := range final.Deadlines {
		if d.ID == deadlineID {
			assert.NotNil(t, d.ExtendedTo)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRecordAnnuityPayment
// ─────────────────────────────────────────────────────────────────────────────

func TestService_RecordAnnuityPayment_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	createCmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	lc, _ := svc.CreateLifecycle(ctx, createCmd)

	next := lc.GetNextAnnuityPayment()
	require.NotNil(t, next)

	err := svc.RecordAnnuityPayment(ctx, lc.ID, next.ID, next.Amount)
	require.NoError(t, err)

	updated, _ := svc.GetLifecycle(ctx, lc.ID)
	for _, a := range updated.AnnuitySchedule {
		if a.ID == next.ID {
			assert.True(t, a.Paid)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestUpdateLegalStatus
// ─────────────────────────────────────────────────────────────────────────────

func TestService_UpdateLegalStatus_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	createCmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	lc, _ := svc.CreateLifecycle(ctx, createCmd)

	err := svc.UpdateLegalStatus(ctx, lc.ID, "granted", "examiner approved")
	require.NoError(t, err)

	updated, _ := svc.GetLifecycle(ctx, lc.ID)
	assert.Equal(t, "granted", updated.LegalStatus.Current)
	assert.Len(t, updated.LegalStatus.History, 2)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetUpcomingDeadlines
// ─────────────────────────────────────────────────────────────────────────────

func TestService_GetUpcomingDeadlines_Success(t *testing.T) {
	t.Parallel()

	repo := NewMockLifecycleRepository()
	svc := lifecycle.NewService(repo)
	ctx := context.Background()

	createCmd := lifecycle.CreateLifecycleCommand{
		PatentID:     common.NewID(),
		PatentNumber: "CN202010000001A",
		Jurisdiction: ptypes.JurisdictionCN,
		FilingDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	lc, _ := svc.CreateLifecycle(ctx, createCmd)

	deadlineCmd := lifecycle.AddDeadlineCommand{
		Type:        lifecycle.DeadlineOAResponse,
		DueDate:     time.Now().UTC().AddDate(0, 0, 5),
		Priority:    lifecycle.PriorityHigh,
		Description: "Upcoming deadline",
	}
	_ = svc.AddDeadline(ctx, lc.ID, deadlineCmd)

	results, err := svc.GetUpcomingDeadlines(ctx, 10, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

//Personal.AI order the ending
