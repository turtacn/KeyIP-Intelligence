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

func applyWorkspaceSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	ddl := `
	CREATE TABLE IF NOT EXISTS workspaces (
		id                TEXT PRIMARY KEY,
		tenant_id         TEXT NOT NULL DEFAULT '',
		name              TEXT NOT NULL DEFAULT '',
		description       TEXT NOT NULL DEFAULT '',
		owner_id          TEXT NOT NULL DEFAULT '',
		members           JSONB NOT NULL DEFAULT '[]',
		shared_resources  JSONB NOT NULL DEFAULT '[]',
		status            TEXT NOT NULL DEFAULT 'active',
		tags              TEXT[] NOT NULL DEFAULT '{}',
		settings          JSONB DEFAULT '{}',
		metadata          JSONB DEFAULT '{}',
		created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by        TEXT NOT NULL DEFAULT '',
		version           INT NOT NULL DEFAULT 1
	);
	CREATE INDEX IF NOT EXISTS idx_workspaces_owner ON workspaces(owner_id);
	CREATE INDEX IF NOT EXISTS idx_workspaces_members ON workspaces USING GIN(members);
	CREATE INDEX IF NOT EXISTS idx_workspaces_resources ON workspaces USING GIN(shared_resources);
	`
	_, err := pool.Exec(ctx, ddl)
	require.NoError(t, err)
}

func newTestWorkspace(suffix string) *repositories.Workspace {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &repositories.Workspace{
		ID:          common.NewID(),
		TenantID:    common.TenantID("tenant-test"),
		Name:        "Workspace-" + suffix,
		Description: "Test workspace " + suffix,
		OwnerID:     common.UserID("owner-" + suffix),
		Members: []repositories.WorkspaceMember{
			{
				UserID:    "owner-" + suffix,
				Role:      "owner",
				JoinedAt:  now,
				InvitedBy: "",
			},
			{
				UserID:    "member-a-" + suffix,
				Role:      "editor",
				JoinedAt:  now,
				InvitedBy: "owner-" + suffix,
			},
		},
		SharedResources: []repositories.SharedResource{
			{
				ResourceID:   "patent-res-" + suffix,
				ResourceType: "patent",
				SharedBy:     "owner-" + suffix,
				SharedAt:     now,
				Permissions:  "read",
			},
		},
		Status:   "active",
		Tags:     []string{"pharma", "collab"},
		Settings: map[string]interface{}{"notifications": true},
		Metadata: map[string]interface{}{"region": "US"},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: common.UserID("test-user"),
		Version:   1,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCollaborationRepository_SaveAndFindByID(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("001")
	require.NoError(t, repo.Save(ctx, ws))

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Equal(t, ws.Name, found.Name)
	assert.Len(t, found.Members, 2)
	assert.Len(t, found.SharedResources, 1)
	assert.Equal(t, "active", found.Status)
}

func TestCollaborationRepository_FindByOwner(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	owner := common.UserID("shared-owner")
	for i := 0; i < 3; i++ {
		ws := newTestWorkspace(fmt.Sprintf("own-%d", i))
		ws.OwnerID = owner
		require.NoError(t, repo.Save(ctx, ws))
	}

	results, total, err := repo.FindByOwner(ctx, owner, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, results, 3)
}

func TestCollaborationRepository_FindWorkspacesByUser(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	targetUser := "user-findable"

	ws := newTestWorkspace("byuser")
	ws.Members = append(ws.Members, repositories.WorkspaceMember{
		UserID:    targetUser,
		Role:      "viewer",
		JoinedAt:  time.Now().UTC(),
		InvitedBy: "owner-byuser",
	})
	require.NoError(t, repo.Save(ctx, ws))

	results, total, err := repo.FindWorkspacesByUser(ctx, common.UserID(targetUser), 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)

	// Verify the target user is in the members list.
	found := false
	for _, r := range results {
		for _, m := range r.Members {
			if m.UserID == targetUser {
				found = true
			}
		}
	}
	assert.True(t, found, "expected target user in workspace members")
}

func TestCollaborationRepository_FindWorkspacesByResource(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	targetResource := common.ID("resource-findable")

	ws := newTestWorkspace("byres")
	ws.SharedResources = append(ws.SharedResources, repositories.SharedResource{
		ResourceID:   string(targetResource),
		ResourceType: "portfolio",
		SharedBy:     "owner-byres",
		SharedAt:     time.Now().UTC(),
		Permissions:  "write",
	})
	require.NoError(t, repo.Save(ctx, ws))

	results, total, err := repo.FindWorkspacesByResource(ctx, targetResource, 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)

	found := false
	for _, r := range results {
		for _, sr := range r.SharedResources {
			if sr.ResourceID == string(targetResource) {
				found = true
			}
		}
	}
	assert.True(t, found, "expected target resource in workspace shared_resources")
}

func TestCollaborationRepository_AddMember(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("addmem")
	require.NoError(t, repo.Save(ctx, ws))

	newMember := repositories.WorkspaceMember{
		UserID:    "new-member-addmem",
		Role:      "editor",
		JoinedAt:  time.Now().UTC(),
		InvitedBy: string(ws.OwnerID),
	}
	require.NoError(t, repo.AddMember(ctx, ws.ID, newMember))

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, found.Members, 3) // 2 original + 1 new

	// Adding the same user again should fail.
	err = repo.AddMember(ctx, ws.ID, newMember)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already a member")
}

func TestCollaborationRepository_RemoveMember(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("rmmem")
	require.NoError(t, repo.Save(ctx, ws))

	targetUserID := common.UserID(ws.Members[1].UserID)
	require.NoError(t, repo.RemoveMember(ctx, ws.ID, targetUserID))

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, found.Members, 1) // only owner remains

	for _, m := range found.Members {
		assert.NotEqual(t, string(targetUserID), m.UserID)
	}
}

func TestCollaborationRepository_UpdateMemberRole(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("uprole")
	require.NoError(t, repo.Save(ctx, ws))

	targetUserID := common.UserID(ws.Members[1].UserID)
	require.NoError(t, repo.UpdateMemberRole(ctx, ws.ID, targetUserID, "admin"))

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)

	var updatedRole string
	for _, m := range found.Members {
		if m.UserID == string(targetUserID) {
			updatedRole = m.Role
		}
	}
	assert.Equal(t, "admin", updatedRole, "member role should be updated to admin")
}

func TestCollaborationRepository_ShareResource(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("share")
	require.NoError(t, repo.Save(ctx, ws))

	newRes := repositories.SharedResource{
		ResourceID:   "new-resource-share",
		ResourceType: "molecule",
		SharedBy:     string(ws.OwnerID),
		SharedAt:     time.Now().UTC(),
		Permissions:  "write",
		Notes:        "Shared for review",
	}
	require.NoError(t, repo.ShareResource(ctx, ws.ID, newRes))

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, found.SharedResources, 2) // 1 original + 1 new

	// Sharing the same resource again should fail.
	err = repo.ShareResource(ctx, ws.ID, newRes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already shared")
}

func TestCollaborationRepository_UnshareResource(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("unshare")
	require.NoError(t, repo.Save(ctx, ws))

	targetResID := common.ID(ws.SharedResources[0].ResourceID)
	require.NoError(t, repo.UnshareResource(ctx, ws.ID, targetResID))

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, found.SharedResources, 0)

	for _, sr := range found.SharedResources {
		assert.NotEqual(t, string(targetResID), sr.ResourceID)
	}
}

func TestCollaborationRepository_UpdateOptimisticLock(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("optlock")
	require.NoError(t, repo.Save(ctx, ws))

	ws.Name = "Updated Workspace Name"
	require.NoError(t, repo.Update(ctx, ws))
	assert.Equal(t, 2, ws.Version)

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Workspace Name", found.Name)

	// Stale version should fail.
	ws.Version = 1
	err = repo.Update(ctx, ws)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestCollaborationRepository_Delete(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("del")
	require.NoError(t, repo.Save(ctx, ws))
	require.NoError(t, repo.Delete(ctx, ws.ID))

	_, err := repo.FindByID(ctx, ws.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCollaborationRepository_DeleteNotFound(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	err := repo.Delete(ctx, common.ID("nonexistent-ws"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCollaborationRepository_Search(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	// Seed workspaces with a recognisable name pattern.
	for i := 0; i < 5; i++ {
		ws := newTestWorkspace(fmt.Sprintf("search-%d", i))
		ws.Name = fmt.Sprintf("Pharma-Collab-%d", i)
		ws.Status = "active"
		require.NoError(t, repo.Save(ctx, ws))
	}

	// Search by name substring.
	results, total, err := repo.Search(ctx, repositories.WorkspaceSearchCriteria{
		Name:     "Pharma-Collab",
		Status:   "active",
		Page:     1,
		PageSize: 3,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(5))
	assert.Len(t, results, 3) // page size = 3

	// Page 2.
	results2, _, err := repo.Search(ctx, repositories.WorkspaceSearchCriteria{
		Name:     "Pharma-Collab",
		Status:   "active",
		Page:     2,
		PageSize: 3,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(results2), 2) // remaining items
}

func TestCollaborationRepository_SearchByTag(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("tag-search")
	ws.Tags = []string{"biotech", "priority"}
	require.NoError(t, repo.Save(ctx, ws))

	results, total, err := repo.Search(ctx, repositories.WorkspaceSearchCriteria{
		Tag:      "biotech",
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)

	found := false
	for _, r := range results {
		for _, tag := range r.Tags {
			if tag == "biotech" {
				found = true
			}
		}
	}
	assert.True(t, found, "expected workspace with 'biotech' tag")
}

func TestCollaborationRepository_Count(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	before, err := repo.Count(ctx)
	require.NoError(t, err)

	require.NoError(t, repo.Save(ctx, newTestWorkspace("count")))

	after, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+1, after)
}

func TestCollaborationRepository_FindByIDNotFound(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	_, err := repo.FindByID(ctx, common.ID("nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCollaborationRepository_FindWorkspacesByUser_Empty(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	results, total, err := repo.FindWorkspacesByUser(ctx, common.UserID("ghost-user"), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, results)
}

func TestCollaborationRepository_FindWorkspacesByResource_Empty(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	results, total, err := repo.FindWorkspacesByResource(ctx, common.ID("ghost-resource"), 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, results)
}

// ─────────────────────────────────────────────────────────────────────────────
// Contract-style: round-trip JSONB fidelity
// ─────────────────────────────────────────────────────────────────────────────

func TestCollaborationRepository_JSONBRoundTrip(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("jsonb-rt")
	ws.Settings = map[string]interface{}{
		"notifications": true,
		"theme":         "dark",
		"nested": map[string]interface{}{
			"level": float64(2),
			"items": []interface{}{"a", "b"},
		},
	}
	ws.Metadata = map[string]interface{}{
		"source":  "import",
		"version": float64(3),
	}
	require.NoError(t, repo.Save(ctx, ws))

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)

	// Verify top-level settings keys.
	assert.Equal(t, true, found.Settings["notifications"])
	assert.Equal(t, "dark", found.Settings["theme"])

	// Verify nested structure survived the round-trip.
	nested, ok := found.Settings["nested"].(map[string]interface{})
	require.True(t, ok, "nested should be a map")
	assert.Equal(t, float64(2), nested["level"])

	// Verify metadata.
	assert.Equal(t, "import", found.Metadata["source"])
	assert.Equal(t, float64(3), found.Metadata["version"])
}

// ─────────────────────────────────────────────────────────────────────────────
// Contract-style: member operations idempotency & edge cases
// ─────────────────────────────────────────────────────────────────────────────

func TestCollaborationRepository_MemberOperationsEdgeCases(t *testing.T) {
	pool := startPostgres(t)
	applyWorkspaceSchema(t, pool)
	repo := repositories.NewCollaborationRepository(pool, noopLogger{})
	ctx := context.Background()

	ws := newTestWorkspace("edge")
	require.NoError(t, repo.Save(ctx, ws))

	// Remove all non-owner members.
	for _, m := range ws.Members {
		if m.Role != "owner" {
			require.NoError(t, repo.RemoveMember(ctx, ws.ID, common.UserID(m.UserID)))
		}
	}

	found, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, found.Members, 1, "only owner should remain")

	// Add a member, update their role, then remove them.
	tempMember := repositories.WorkspaceMember{
		UserID:    "temp-user-edge",
		Role:      "viewer",
		JoinedAt:  time.Now().UTC(),
		InvitedBy: string(ws.OwnerID),
	}
	require.NoError(t, repo.AddMember(ctx, ws.ID, tempMember))
	require.NoError(t, repo.UpdateMemberRole(ctx, ws.ID, common.UserID("temp-user-edge"), "admin"))

	found2, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	for _, m := range found2.Members {
		if m.UserID == "temp-user-edge" {
			assert.Equal(t, "admin", m.Role)
		}
	}

	require.NoError(t, repo.RemoveMember(ctx, ws.ID, common.UserID("temp-user-edge")))

	found3, err := repo.FindByID(ctx, ws.ID)
	require.NoError(t, err)
	assert.Len(t, found3.Members, 1)
}


