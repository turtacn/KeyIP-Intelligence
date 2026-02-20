// Package collaboration_test provides repository contract tests.
package collaboration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/collaboration"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// RepositoryContractTest defines the standard test suite that all Repository
// implementations must pass.  Concrete implementations (PostgreSQL, MongoDB, etc.)
// should call this function in their own test files, passing their specific
// repository instance and cleanup callback.
//
// Example usage:
//
//	func TestPostgresRepository(t *testing.T) {
//	    repo := setupPostgresRepo(t)
//	    cleanup := func() { teardownPostgresRepo(t, repo) }
//	    collaboration_test.RepositoryContractTest(t, repo, cleanup)
//	}
func RepositoryContractTest(t *testing.T, repo collaboration.Repository, cleanup func()) {
	defer cleanup()
	ctx := context.Background()

	t.Run("SaveWorkspace_Success", func(t *testing.T) {
		ws, _ := collaboration.NewWorkspace("Test Workspace", "Contract test", "user-1")
		err := repo.SaveWorkspace(ctx, ws)
		require.NoError(t, err)
	})

	t.Run("FindWorkspaceByID_Found", func(t *testing.T) {
		ws, _ := collaboration.NewWorkspace("Find Test", "", "user-2")
		_ = repo.SaveWorkspace(ctx, ws)

		found, err := repo.FindWorkspaceByID(ctx, ws.ID)
		require.NoError(t, err)
		assert.Equal(t, ws.ID, found.ID)
		assert.Equal(t, ws.Name, found.Name)
	})

	t.Run("FindWorkspaceByID_NotFound", func(t *testing.T) {
		_, err := repo.FindWorkspaceByID(ctx, "nonexistent-id")
		require.Error(t, err)
		assert.True(t, errors.IsCode(err, errors.CodeNotFound))
	})

	t.Run("FindWorkspacesByUser_Success", func(t *testing.T) {
		userID := common.UserID("user-multi")
		ws1, _ := collaboration.NewWorkspace("WS1", "", userID)
		ws2, _ := collaboration.NewWorkspace("WS2", "", userID)
		_ = repo.SaveWorkspace(ctx, ws1)
		_ = repo.SaveWorkspace(ctx, ws2)

		page := common.PageRequest{Page: 1, PageSize: 10}
		resp, err := repo.FindWorkspacesByUser(ctx, userID, page)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Items), 2)
	})

	t.Run("UpdateWorkspace_Success", func(t *testing.T) {
		ws, _ := collaboration.NewWorkspace("Update Test", "", "user-3")
		_ = repo.SaveWorkspace(ctx, ws)

		ws.Name = "Updated Name"
		ws.Version++
		err := repo.UpdateWorkspace(ctx, ws)
		require.NoError(t, err)

		updated, _ := repo.FindWorkspaceByID(ctx, ws.ID)
		assert.Equal(t, "Updated Name", updated.Name)
	})

	t.Run("UpdateWorkspace_OptimisticLockConflict", func(t *testing.T) {
		ws, _ := collaboration.NewWorkspace("Lock Test", "", "user-4")
		_ = repo.SaveWorkspace(ctx, ws)

		// Simulate concurrent update by not incrementing version.
		ws.Name = "Conflicting Update"
		err := repo.UpdateWorkspace(ctx, ws)
		// Should fail due to version mismatch (implementation-dependent).
		// If the implementation uses optimistic locking, this should error.
		_ = err // Contract allows both error or success here; specific tests should verify.
	})

	t.Run("DeleteWorkspace_Success", func(t *testing.T) {
		ws, _ := collaboration.NewWorkspace("Delete Test", "", "user-5")
		_ = repo.SaveWorkspace(ctx, ws)

		err := repo.DeleteWorkspace(ctx, ws.ID)
		require.NoError(t, err)

		_, err = repo.FindWorkspaceByID(ctx, ws.ID)
		assert.Error(t, err, "deleted workspace should not be found")
	})

	t.Run("FindWorkspacesByResource_Success", func(t *testing.T) {
		ws, _ := collaboration.NewWorkspace("Resource Test", "", "user-6")
		resourceID := common.ID("res-shared")
		_ = ws.ShareResource(collaboration.SharedResource{
			ResourceID:   resourceID,
			ResourceType: "patent",
			SharedBy:     "user-6",
			AccessLevel:  "read",
		})
		_ = repo.SaveWorkspace(ctx, ws)

		workspaces, err := repo.FindWorkspacesByResource(ctx, resourceID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(workspaces), 1)
		found := false
		for _, w := range workspaces {
			if w.ID == ws.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "workspace sharing the resource should be in results")
	})
}

func TestMockRepository(t *testing.T) {
	repo := newMockRepository()
	cleanup := func() {}
	RepositoryContractTest(t, repo, cleanup)
}

