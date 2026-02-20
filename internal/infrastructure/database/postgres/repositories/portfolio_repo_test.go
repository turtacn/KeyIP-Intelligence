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

func applyPortfolioSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	ddl := `
	CREATE TABLE IF NOT EXISTS portfolios (
		id          TEXT PRIMARY KEY,
		tenant_id   TEXT NOT NULL DEFAULT '',
		name        TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		owner_id    TEXT NOT NULL DEFAULT '',
		patent_ids  TEXT[] NOT NULL DEFAULT '{}',
		tags        TEXT[] NOT NULL DEFAULT '{}',
		total_value JSONB DEFAULT '{}',
		status      TEXT NOT NULL DEFAULT 'active',
		metadata    JSONB DEFAULT '{}',
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by  TEXT NOT NULL DEFAULT '',
		version     INT NOT NULL DEFAULT 1
	);
	`
	_, err := pool.Exec(ctx, ddl)
	require.NoError(t, err)
}

func newTestPortfolio(suffix string) *repositories.Portfolio {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &repositories.Portfolio{
		ID:          common.NewID(),
		TenantID:    common.TenantID("tenant-test"),
		Name:        "Portfolio-" + suffix,
		Description: "Test portfolio " + suffix,
		OwnerID:     common.UserID("owner-" + suffix),
		PatentIDs:   []common.ID{common.NewID(), common.NewID()},
		Tags:        []string{"pharma", "oncology"},
		TotalValue: &repositories.ValuationResult{
			TotalValue: 1500000.0,
			Currency:   "USD",
			Method:     "dcf",
			Confidence: 0.85,
		},
		Status:    "active",
		Metadata:  map[string]interface{}{"region": "APAC"},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: common.UserID("test-user"),
		Version:   1,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPortfolioRepository_SaveAndFindByID(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPortfolio("001")
	require.NoError(t, repo.Save(ctx, p))

	found, err := repo.FindByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.Name, found.Name)
	assert.Len(t, found.PatentIDs, 2)
	assert.NotNil(t, found.TotalValue)
	assert.Equal(t, 1500000.0, found.TotalValue.TotalValue)
}

func TestPortfolioRepository_FindByPatentID(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	sharedPatent := common.NewID()
	p := newTestPortfolio("002")
	p.PatentIDs = append(p.PatentIDs, sharedPatent)
	require.NoError(t, repo.Save(ctx, p))

	results, err := repo.FindByPatentID(ctx, sharedPatent)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, p.ID, results[0].ID)
}

func TestPortfolioRepository_FindByTag(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPortfolio("003")
	p.Tags = []string{"neuroscience", "rare-disease"}
	require.NoError(t, repo.Save(ctx, p))

	results, total, err := repo.FindByTag(ctx, "rare-disease", 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestPortfolioRepository_FindByOwner(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	owner := common.UserID("owner-shared")
	for i := 0; i < 3; i++ {
		p := newTestPortfolio(fmt.Sprintf("own-%d", i))
		p.OwnerID = owner
		require.NoError(t, repo.Save(ctx, p))
	}

	results, total, err := repo.FindByOwner(ctx, owner, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, results, 3)
}

func TestPortfolioRepository_UpdateOptimisticLock(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPortfolio("004")
	require.NoError(t, repo.Save(ctx, p))

	p.Name = "Updated Portfolio"
	require.NoError(t, repo.Update(ctx, p))
	assert.Equal(t, 2, p.Version)

	p.Version = 1
	err := repo.Update(ctx, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestPortfolioRepository_Search(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPortfolio("005")
	p.Name = "Oncology Drug Pipeline"
	p.Status = "active"
	p.Tags = []string{"oncology"}
	require.NoError(t, repo.Save(ctx, p))

	results, total, err := repo.Search(ctx, repositories.PortfolioSearchCriteria{
		Name:     "oncology",
		Status:   "active",
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestPortfolioRepository_Delete(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPortfolio("006")
	require.NoError(t, repo.Save(ctx, p))
	require.NoError(t, repo.Delete(ctx, p.ID))

	_, err := repo.FindByID(ctx, p.ID)
	require.Error(t, err)
}

func TestPortfolioRepository_Count(t *testing.T) {
	pool := startPostgres(t)
	applyPortfolioSchema(t, pool)
	repo := repositories.NewPortfolioRepository(pool, noopLogger{})
	ctx := context.Background()

	before, err := repo.Count(ctx)
	require.NoError(t, err)

	require.NoError(t, repo.Save(ctx, newTestPortfolio("007")))

	after, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+1, after)
}

