// Package portfolio_test defines repository contract tests that can be run
// against any Repository implementation to verify correct behaviour.
package portfolio_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// RepositoryContractTest runs a suite of behavioural tests against a
// Repository implementation.  Call this from infrastructure-layer tests with
// a concrete repository instance.
func RepositoryContractTest(t *testing.T, repo portfolio.Repository) {
	ctx := context.Background()

	t.Run("Save_Success", func(t *testing.T) {
		p, err := portfolio.NewPortfolio("Test Portfolio", "desc", "user-1")
		require.NoError(t, err)

		err = repo.Save(ctx, p)
		assert.NoError(t, err)
	})

	t.Run("FindByID_NotFound", func(t *testing.T) {
		nonExistentID := common.NewID()
		_, err := repo.FindByID(ctx, nonExistentID)

		assert.Error(t, err)
		// Expect errors.CodePortfolioNotFound or equivalent not-found error.
	})

	t.Run("FindByID_Success", func(t *testing.T) {
		p, err := portfolio.NewPortfolio("Find Test", "desc", "user-2")
		require.NoError(t, err)
		require.NoError(t, repo.Save(ctx, p))

		found, err := repo.FindByID(ctx, p.ID)

		require.NoError(t, err)
		assert.Equal(t, p.ID, found.ID)
		assert.Equal(t, p.Name, found.Name)
	})

	t.Run("Update_Success", func(t *testing.T) {
		p, err := portfolio.NewPortfolio("Update Test", "original", "user-3")
		require.NoError(t, err)
		require.NoError(t, repo.Save(ctx, p))

		p.Description = "updated description"
		err = repo.Update(ctx, p)

		require.NoError(t, err)

		found, err := repo.FindByID(ctx, p.ID)
		require.NoError(t, err)
		assert.Equal(t, "updated description", found.Description)
	})

	t.Run("Delete_Success", func(t *testing.T) {
		p, err := portfolio.NewPortfolio("Delete Test", "desc", "user-4")
		require.NoError(t, err)
		require.NoError(t, repo.Save(ctx, p))

		err = repo.Delete(ctx, p.ID)
		assert.NoError(t, err)

		// Verify it cannot be found after deletion.
		_, err = repo.FindByID(ctx, p.ID)
		assert.Error(t, err)
	})

	t.Run("FindByOwner_Pagination", func(t *testing.T) {
		ownerID := common.UserID("owner-pagination")
		for i := 0; i < 5; i++ {
			p, err := portfolio.NewPortfolio("Portfolio "+string(rune('A'+i)), "desc", ownerID)
			require.NoError(t, err)
			require.NoError(t, repo.Save(ctx, p))
		}

		pageReq := common.PageRequest{Page: 1, PageSize: 3}
		resp, err := repo.FindByOwner(ctx, ownerID, pageReq)

		require.NoError(t, err)
		assert.LessOrEqual(t, len(resp.Items), 3)
		assert.GreaterOrEqual(t, resp.Total, int64(5))
	})

	t.Run("FindByPatentID_Success", func(t *testing.T) {
		patentID := common.NewID()
		p1, err := portfolio.NewPortfolio("Portfolio with Patent", "desc", "user-5")
		require.NoError(t, err)
		require.NoError(t, p1.AddPatent(patentID))
		require.NoError(t, repo.Save(ctx, p1))

		portfolios, err := repo.FindByPatentID(ctx, patentID)

		require.NoError(t, err)
		assert.NotEmpty(t, portfolios)
		found := false
		for _, pf := range portfolios {
			if pf.ID == p1.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("FindByTag_Success", func(t *testing.T) {
		tag := "test-tag-unique"
		p, err := portfolio.NewPortfolio("Tagged Portfolio", "desc", "user-6")
		require.NoError(t, err)
		p.Tags = []string{tag}
		require.NoError(t, repo.Save(ctx, p))

		pageReq := common.PageRequest{Page: 1, PageSize: 10}
		resp, err := repo.FindByTag(ctx, tag, pageReq)

		require.NoError(t, err)
		assert.Greater(t, resp.Total, int64(0))
	})
}

//Personal.AI order the ending
