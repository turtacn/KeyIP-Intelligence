//go:build integration

// Package repositories_test provides integration tests for PostgreSQL repository
// implementations.  Tests require Docker and are gated behind the "integration"
// build tag.
package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

// noopLogger satisfies repositories.Logger without producing output.
type noopLogger struct{}

func (noopLogger) Debug(string, ...interface{}) {}
func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Warn(string, ...interface{})  {}
func (noopLogger) Error(string, ...interface{}) {}

// startPostgres launches a PostgreSQL 16 container and returns a connected pool.
func startPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "keyip_test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	dsn := fmt.Sprintf("postgres://test:test@%s:%s/keyip_test?sslmode=disable", host, port.Port())
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Run minimal schema migration.
	applyPatentSchema(t, pool)
	return pool
}

func applyPatentSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	ddl := `
	CREATE TABLE IF NOT EXISTS patents (
		id              TEXT PRIMARY KEY,
		tenant_id       TEXT NOT NULL DEFAULT '',
		patent_number   TEXT NOT NULL UNIQUE,
		title           TEXT NOT NULL DEFAULT '',
		abstract        TEXT NOT NULL DEFAULT '',
		description     TEXT NOT NULL DEFAULT '',
		applicants      TEXT[] NOT NULL DEFAULT '{}',
		inventors       TEXT[] NOT NULL DEFAULT '{}',
		ipc_codes       TEXT[] NOT NULL DEFAULT '{}',
		jurisdiction    TEXT NOT NULL DEFAULT '',
		filing_date     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		publication_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		grant_date      TIMESTAMPTZ,
		expiry_date     TIMESTAMPTZ,
		priority        TEXT[] NOT NULL DEFAULT '{}',
		family_id       TEXT NOT NULL DEFAULT '',
		status          TEXT NOT NULL DEFAULT 'pending',
		legal_status    TEXT NOT NULL DEFAULT '',
		citations       TEXT[] NOT NULL DEFAULT '{}',
		metadata        JSONB DEFAULT '{}',
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by      TEXT NOT NULL DEFAULT '',
		version         INT NOT NULL DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS patent_claims (
		id              TEXT PRIMARY KEY,
		patent_id       TEXT NOT NULL REFERENCES patents(id),
		claim_number    INT NOT NULL,
		claim_type      TEXT NOT NULL DEFAULT '',
		parent_id       TEXT,
		text            TEXT NOT NULL DEFAULT '',
		is_independent  BOOLEAN NOT NULL DEFAULT TRUE
	);

	CREATE TABLE IF NOT EXISTS markush_structures (
		id              TEXT PRIMARY KEY,
		patent_id       TEXT NOT NULL REFERENCES patents(id),
		smarts          TEXT NOT NULL DEFAULT '',
		description     TEXT NOT NULL DEFAULT '',
		r_groups        JSONB DEFAULT '{}'
	);
	`
	_, err := pool.Exec(ctx, ddl)
	require.NoError(t, err)
}

func newTestPatent(suffix string) *repositories.Patent {
	now := time.Now().UTC().Truncate(time.Microsecond)
	expiry := now.AddDate(20, 0, 0)
	return &repositories.Patent{
		ID:              common.NewID(),
		TenantID:        common.TenantID("tenant-test"),
		PatentNumber:    "US-2024-" + suffix,
		Title:           "Test Patent " + suffix,
		Abstract:        "Abstract for patent " + suffix,
		Description:     "Full description for patent " + suffix,
		Applicants:      []string{"Acme Corp"},
		Inventors:       []string{"Alice", "Bob"},
		IPCCodes:        []string{"C07D", "A61K"},
		Jurisdiction:    "US",
		FilingDate:      now.AddDate(-2, 0, 0),
		PublicationDate: now.AddDate(-1, 0, 0),
		ExpiryDate:      &expiry,
		Priority:        []string{"US-PROV-001"},
		FamilyID:        "FAM-" + suffix,
		Status:          "granted",
		LegalStatus:     "active",
		Citations:       []string{},
		Claims: []repositories.Claim{
			{
				ID:            common.NewID(),
				ClaimNumber:   1,
				ClaimType:     "independent",
				Text:          "A compound comprising...",
				IsIndependent: true,
			},
		},
		Metadata:  map[string]interface{}{"source": "test"},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: common.UserID("test-user"),
		Version:   1,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Contract tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPatentRepository_SaveAndFindByID(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("001")
	require.NoError(t, repo.Save(ctx, p))

	found, err := repo.FindByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.PatentNumber, found.PatentNumber)
	assert.Equal(t, p.Title, found.Title)
	assert.Len(t, found.Claims, 1)
}

func TestPatentRepository_FindByNumber(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("002")
	require.NoError(t, repo.Save(ctx, p))

	found, err := repo.FindByNumber(ctx, p.PatentNumber)
	require.NoError(t, err)
	assert.Equal(t, p.ID, found.ID)
}

func TestPatentRepository_FindByFamilyID(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p1 := newTestPatent("003a")
	p1.FamilyID = "FAM-SHARED"
	p2 := newTestPatent("003b")
	p2.FamilyID = "FAM-SHARED"
	require.NoError(t, repo.Save(ctx, p1))
	require.NoError(t, repo.Save(ctx, p2))

	results, err := repo.FindByFamilyID(ctx, "FAM-SHARED")
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestPatentRepository_UpdateOptimisticLock(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("004")
	require.NoError(t, repo.Save(ctx, p))

	// First update succeeds.
	p.Title = "Updated Title"
	require.NoError(t, repo.Update(ctx, p))
	assert.Equal(t, 2, p.Version)

	// Simulate stale version.
	p.Version = 1
	err := repo.Update(ctx, p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestPatentRepository_SoftDelete(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("005")
	require.NoError(t, repo.Save(ctx, p))
	require.NoError(t, repo.Delete(ctx, p.ID, false))

	found, err := repo.FindByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "deleted", found.Status)
}

func TestPatentRepository_HardDelete(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("006")
	require.NoError(t, repo.Save(ctx, p))
	require.NoError(t, repo.Delete(ctx, p.ID, true))

	_, err := repo.FindByID(ctx, p.ID)
	require.Error(t, err)
}

func TestPatentRepository_Search_FullText(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("007")
	p.Title = "Novel OLED Emitter Compound"
	p.Abstract = "A thermally activated delayed fluorescence material for organic light-emitting diodes"
	require.NoError(t, repo.Save(ctx, p))

	results, total, err := repo.Search(ctx, repositories.PatentSearchCriteria{
		Keyword:  "OLED fluorescence",
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestPatentRepository_Search_JurisdictionFilter(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("008")
	p.Jurisdiction = "EP"
	require.NoError(t, repo.Save(ctx, p))

	results, _, err := repo.Search(ctx, repositories.PatentSearchCriteria{
		Jurisdiction: "EP",
		Page:         1,
		PageSize:     10,
	})
	require.NoError(t, err)
	for _, r := range results {
		assert.Equal(t, "EP", r.Jurisdiction)
	}
}

func TestPatentRepository_FindByApplicant(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("009")
	p.Applicants = []string{"Samsung SDI"}
	require.NoError(t, repo.Save(ctx, p))

	results, total, err := repo.FindByApplicant(ctx, "Samsung SDI", 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestPatentRepository_CountByStatus(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("010")
	require.NoError(t, repo.Save(ctx, p))

	counts, err := repo.CountByStatus(ctx)
	require.NoError(t, err)
	assert.Greater(t, counts["granted"], int64(0))
}

func TestPatentRepository_FindExpiring(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	p := newTestPatent("011")
	soon := time.Now().UTC().AddDate(0, 0, 30)
	p.ExpiryDate = &soon
	require.NoError(t, repo.Save(ctx, p))

	results, total, err := repo.FindExpiring(ctx, time.Now().UTC().AddDate(0, 0, 60), 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestPatentRepository_PaginationLargeDataset(t *testing.T) {
	pool := startPostgres(t)
	repo := repositories.NewPatentRepository(pool, noopLogger{})
	ctx := context.Background()

	// Insert 50 patents.
	for i := 0; i < 50; i++ {
		p := newTestPatent(fmt.Sprintf("bulk-%03d", i))
		p.Jurisdiction = "CN"
		require.NoError(t, repo.Save(ctx, p))
	}

	// Page 1.
	r1, total, err := repo.FindByJurisdiction(ctx, "CN", 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(50))
	assert.Len(t, r1, 10)

	// Page 5.
	r5, _, err := repo.FindByJurisdiction(ctx, "CN", 5, 10)
	require.NoError(t, err)
	assert.Len(t, r5, 10)

	// Ensure pages don't overlap.
	assert.NotEqual(t, r1[0].ID, r5[0].ID)
}

//Personal.AI order the ending
