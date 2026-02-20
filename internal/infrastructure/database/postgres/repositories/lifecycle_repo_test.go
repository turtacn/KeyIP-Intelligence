//go:build integration

package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Schema helper
// ─────────────────────────────────────────────────────────────────────────────

func applyLifecycleSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	ddl := `
	CREATE TABLE IF NOT EXISTS lifecycles (
		id                TEXT PRIMARY KEY,
		tenant_id         TEXT NOT NULL DEFAULT '',
		patent_id         TEXT NOT NULL DEFAULT '',
		phase             TEXT NOT NULL DEFAULT 'filing',
		deadlines         JSONB NOT NULL DEFAULT '[]',
		annuity_schedule  JSONB NOT NULL DEFAULT '[]',
		legal_status      JSONB NOT NULL DEFAULT '[]',
		events            JSONB NOT NULL DEFAULT '[]',
		metadata          JSONB DEFAULT '{}',
		created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by        TEXT NOT NULL DEFAULT '',
		version           INT NOT NULL DEFAULT 1
	);
	CREATE INDEX IF NOT EXISTS idx_lifecycles_patent_id ON lifecycles(patent_id);
	CREATE INDEX IF NOT EXISTS idx_lifecycles_phase ON lifecycles(phase);
	`
	_, err := pool.Exec(ctx, ddl)
	require.NoError(t, err)
}

func newTestLifecycle(suffix string) *repositories.Lifecycle {
	now := time.Now().UTC().Truncate(time.Microsecond)
	upcoming := now.Add(72 * time.Hour)
	overdue := now.Add(-48 * time.Hour)

	return &repositories.Lifecycle{
		ID:       common.NewID(),
		TenantID: common.TenantID("tenant-test"),
		PatentID: common.ID("patent-" + suffix),
		Phase:    "examination",
		Deadlines: []repositories.Deadline{
			{
				ID:          "dl-upcoming-" + suffix,
				Type:        "response",
				Description: "Office action response due",
				DueDate:     upcoming,
				Status:      "pending",
			},
			{
				ID:          "dl-overdue-" + suffix,
				Type:        "fee",
				Description: "Filing fee overdue",
				DueDate:     overdue,
				Status:      "pending",
			},
		},
		AnnuitySchedule: []repositories.AnnuityRecord{
			{
				ID:           "ann-" + suffix,
				Year:         3,
				DueDate:      upcoming,
				Amount:       1200.00,
				Currency:     "USD",
				Status:       "unpaid",
				Jurisdiction: "US",
			},
		},
		LegalStatus: []repositories.LegalStatusEntry{
			{
				Code:        "EXAM",
				Description: "Under examination",
				EffectiveAt: now.Add(-30 * 24 * time.Hour),
				Source:      "USPTO",
			},
		},
		Events: []repositories.LifecycleEvent{
			{
				ID:          "evt-" + suffix,
				Type:        "status_change",
				Description: "Moved to examination",
				OccurredAt:  now.Add(-30 * 24 * time.Hour),
				Actor:       "system",
			},
		},
		Metadata:  map[string]interface{}{"region": "US"},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: common.UserID("test-user"),
		Version:   1,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestLifecycleRepository_SaveAndFindByID(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("001")
	require.NoError(t, repo.Save(ctx, lc))

	found, err := repo.FindByID(ctx, lc.ID)
	require.NoError(t, err)
	assert.Equal(t, lc.Phase, found.Phase)
	assert.Len(t, found.Deadlines, 2)
	assert.Len(t, found.AnnuitySchedule, 1)
	assert.Len(t, found.LegalStatus, 1)
	assert.Len(t, found.Events, 1)
}

func TestLifecycleRepository_FindByPatentID(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("002")
	require.NoError(t, repo.Save(ctx, lc))

	found, err := repo.FindByPatentID(ctx, lc.PatentID)
	require.NoError(t, err)
	assert.Equal(t, lc.ID, found.ID)
}

func TestLifecycleRepository_FindByPhase(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("003")
	lc.Phase = "granted"
	require.NoError(t, repo.Save(ctx, lc))

	results, total, err := repo.FindByPhase(ctx, "granted", 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestLifecycleRepository_FindUpcomingDeadlines(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("004")
	require.NoError(t, repo.Save(ctx, lc))

	horizon := time.Now().UTC().Add(7 * 24 * time.Hour)
	results, total, err := repo.FindUpcomingDeadlines(ctx, horizon, 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)

	// Verify the returned lifecycle contains the upcoming deadline.
	foundUpcoming := false
	for _, r := range results {
		for _, d := range r.Deadlines {
			if d.Status == "pending" && d.DueDate.After(time.Now()) && d.DueDate.Before(horizon) {
				foundUpcoming = true
			}
		}
	}
	assert.True(t, foundUpcoming, "expected at least one upcoming deadline")
}

func TestLifecycleRepository_FindOverdueDeadlines(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("005")
	require.NoError(t, repo.Save(ctx, lc))

	results, total, err := repo.FindOverdueDeadlines(ctx, 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)

	// Verify the returned lifecycle contains the overdue deadline.
	foundOverdue := false
	for _, r := range results {
		for _, d := range r.Deadlines {
			if d.Status != "completed" && d.DueDate.Before(time.Now()) {
				foundOverdue = true
			}
		}
	}
	assert.True(t, foundOverdue, "expected at least one overdue deadline")
}

func TestLifecycleRepository_FindUnpaidAnnuities(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("006")
	// Ensure the annuity due date is in the past so it qualifies.
	lc.AnnuitySchedule[0].DueDate = time.Now().UTC().Add(-24 * time.Hour)
	require.NoError(t, repo.Save(ctx, lc))

	horizon := time.Now().UTC().Add(24 * time.Hour)
	results, total, err := repo.FindUnpaidAnnuities(ctx, horizon, 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestLifecycleRepository_AddDeadline(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("007")
	require.NoError(t, repo.Save(ctx, lc))

	newDL := repositories.Deadline{
		ID:          "dl-new-007",
		Type:        "maintenance",
		Description: "Maintenance fee due",
		DueDate:     time.Now().UTC().Add(180 * 24 * time.Hour),
		Status:      "pending",
	}
	require.NoError(t, repo.AddDeadline(ctx, lc.ID, newDL))

	found, err := repo.FindByID(ctx, lc.ID)
	require.NoError(t, err)
	assert.Len(t, found.Deadlines, 3) // 2 original + 1 new
}

func TestLifecycleRepository_CompleteDeadline(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("008")
	require.NoError(t, repo.Save(ctx, lc))

	targetDLID := lc.Deadlines[0].ID
	require.NoError(t, repo.CompleteDeadline(ctx, lc.ID, targetDLID))

	found, err := repo.FindByID(ctx, lc.ID)
	require.NoError(t, err)

	var completed bool
	for _, d := range found.Deadlines {
		if d.ID == targetDLID && d.Status == "completed" {
			completed = true
		}
	}
	assert.True(t, completed, "deadline should be marked completed")
}

func TestLifecycleRepository_RecordAnnuityPayment(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("009")
	require.NoError(t, repo.Save(ctx, lc))

	annuityID := lc.AnnuitySchedule[0].ID
	require.NoError(t, repo.RecordAnnuityPayment(ctx, lc.ID, annuityID, "PAY-REF-009"))

	found, err := repo.FindByID(ctx, lc.ID)
	require.NoError(t, err)

	var paid bool
	for _, a := range found.AnnuitySchedule {
		if a.ID == annuityID && a.Status == "paid" && a.PaymentRef == "PAY-REF-009" {
			paid = true
		}
	}
	assert.True(t, paid, "annuity should be marked paid")
}

func TestLifecycleRepository_AddEvent(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("010")
	require.NoError(t, repo.Save(ctx, lc))

	evt := repositories.LifecycleEvent{
		ID:          "evt-new-010",
		Type:        "grant",
		Description: "Patent granted",
		OccurredAt:  time.Now().UTC(),
		Actor:       "examiner",
	}
	require.NoError(t, repo.AddEvent(ctx, lc.ID, evt))

	found, err := repo.FindByID(ctx, lc.ID)
	require.NoError(t, err)
	assert.Len(t, found.Events, 2) // 1 original + 1 new
}

func TestLifecycleRepository_UpdateOptimisticLock(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("011")
	require.NoError(t, repo.Save(ctx, lc))

	lc.Phase = "granted"
	require.NoError(t, repo.Update(ctx, lc))
	assert.Equal(t, 2, lc.Version)

	// Stale version should fail.
	lc.Version = 1
	err := repo.Update(ctx, lc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestLifecycleRepository_Delete(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	lc := newTestLifecycle("012")
	require.NoError(t, repo.Save(ctx, lc))
	require.NoError(t, repo.Delete(ctx, lc.ID))

	_, err := repo.FindByID(ctx, lc.ID)
	require.Error(t, err)
}

func TestLifecycleRepository_Search(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		lc := newTestLifecycle(fmt.Sprintf("search-%d", i))
		lc.Phase = "filing"
		require.NoError(t, repo.Save(ctx, lc))
	}

	results, total, err := repo.Search(ctx, repositories.LifecycleSearchCriteria{
		Phase:    "filing",
		Page:     1,
		PageSize: 3,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(5))
	assert.Len(t, results, 3)
}

func TestLifecycleRepository_Count(t *testing.T) {
	pool := startPostgres(t)
	applyLifecycleSchema(t, pool)
	repo := repositories.NewLifecycleRepository(pool, noopLogger{})
	ctx := context.Background()

	before, err := repo.Count(ctx)
	require.NoError(t, err)

	require.NoError(t, repo.Save(ctx, newTestLifecycle("count")))

	after, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+1, after)
}

//Personal.AI order the ending
