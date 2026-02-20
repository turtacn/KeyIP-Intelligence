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
	moleculeTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// Schema helper
// ─────────────────────────────────────────────────────────────────────────────

func applyMoleculeSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	ddl := `
	CREATE TABLE IF NOT EXISTS molecules (
		id                TEXT PRIMARY KEY,
		tenant_id         TEXT NOT NULL DEFAULT '',
		smiles            TEXT NOT NULL DEFAULT '',
		canonical_smiles  TEXT NOT NULL DEFAULT '',
		inchi             TEXT NOT NULL DEFAULT '',
		inchi_key         TEXT NOT NULL DEFAULT '',
		molecular_formula TEXT NOT NULL DEFAULT '',
		molecular_weight  DOUBLE PRECISION NOT NULL DEFAULT 0,
		name              TEXT NOT NULL DEFAULT '',
		synonyms          TEXT[] NOT NULL DEFAULT '{}',
		type              TEXT NOT NULL DEFAULT '',
		properties        JSONB DEFAULT '{}',
		fingerprints      JSONB DEFAULT '{}',
		source_patent_ids TEXT[] NOT NULL DEFAULT '{}',
		metadata          JSONB DEFAULT '{}',
		created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_by        TEXT NOT NULL DEFAULT '',
		version           INT NOT NULL DEFAULT 1
	);
	`
	_, err := pool.Exec(ctx, ddl)
	require.NoError(t, err)
}

func newTestMolecule(suffix string) *repositories.Molecule {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &repositories.Molecule{
		ID:               common.NewID(),
		TenantID:         common.TenantID("tenant-test"),
		SMILES:           "c1ccccc1" + suffix,
		CanonicalSMILES:  "c1ccccc1" + suffix,
		InChI:            "InChI=1S/C6H6/c1-2-4-6-5-3-1/h1-6H",
		InChIKey:         "UHOVQNZJYSORNB-UHFFFAOYSA-" + suffix,
		MolecularFormula: "C6H6",
		MolecularWeight:  78.11,
		Name:             "Benzene-" + suffix,
		Synonyms:         []string{"cyclohexatriene"},
		Type:             moleculeTypes.MoleculeTypeSmallMolecule,
		Properties:       moleculeTypes.MolecularProperties{LogP: 1.56},
		Fingerprints:     map[moleculeTypes.FingerprintType][]byte{"morgan": {0xFF, 0x0A}},
		SourcePatentIDs:  []common.ID{},
		Metadata:         map[string]interface{}{"source": "test"},
		CreatedAt:        now,
		UpdatedAt:        now,
		CreatedBy:        common.UserID("test-user"),
		Version:          1,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMoleculeRepository_SaveAndFindByID(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("001")
	require.NoError(t, repo.Save(ctx, m))

	found, err := repo.FindByID(ctx, m.ID)
	require.NoError(t, err)
	assert.Equal(t, m.Name, found.Name)
	assert.Equal(t, m.CanonicalSMILES, found.CanonicalSMILES)
}

func TestMoleculeRepository_FindBySMILES(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("002")
	require.NoError(t, repo.Save(ctx, m))

	found, err := repo.FindBySMILES(ctx, m.CanonicalSMILES)
	require.NoError(t, err)
	assert.Equal(t, m.ID, found.ID)
}

func TestMoleculeRepository_FindByInChIKey(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("003")
	require.NoError(t, repo.Save(ctx, m))

	found, err := repo.FindByInChIKey(ctx, m.InChIKey)
	require.NoError(t, err)
	assert.Equal(t, m.ID, found.ID)
}

func TestMoleculeRepository_Search(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("004")
	m.Name = "Aspirin-Derivative"
	require.NoError(t, repo.Save(ctx, m))

	results, total, err := repo.Search(ctx, repositories.MoleculeSearchCriteria{
		Name:     "aspirin",
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestMoleculeRepository_BatchSave(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	batch := make([]*repositories.Molecule, 100)
	for i := range batch {
		batch[i] = newTestMolecule(fmt.Sprintf("batch-%03d", i))
	}

	require.NoError(t, repo.BatchSave(ctx, batch))

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(100))
}

func TestMoleculeRepository_FindByPatentID(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	patentID := common.NewID()
	m := newTestMolecule("005")
	m.SourcePatentIDs = []common.ID{patentID}
	require.NoError(t, repo.Save(ctx, m))

	results, err := repo.FindByPatentID(ctx, patentID)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, m.ID, results[0].ID)
}

func TestMoleculeRepository_FindSimilar(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("006")
	m.Fingerprints = map[moleculeTypes.FingerprintType][]byte{
		"morgan": {0xFF, 0xFF, 0x0F, 0x00},
	}
	require.NoError(t, repo.Save(ctx, m))

	// Query with a fingerprint that has high overlap.
	target := []byte{0xFF, 0xFF, 0x0F, 0x01}
	results, err := repo.FindSimilar(ctx, target, 0.5, "", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
}

func TestMoleculeRepository_SubstructureSearch(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("007")
	m.CanonicalSMILES = "c1ccc(O)cc1"
	require.NoError(t, repo.Save(ctx, m))

	results, total, err := repo.SubstructureSearch(ctx, "c1ccc", 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	assert.NotEmpty(t, results)
}

func TestMoleculeRepository_UpdateOptimisticLock(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("008")
	require.NoError(t, repo.Save(ctx, m))

	m.Name = "Updated Name"
	require.NoError(t, repo.Update(ctx, m))
	assert.Equal(t, 2, m.Version)

	// Stale version should fail.
	m.Version = 1
	err := repo.Update(ctx, m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestMoleculeRepository_Delete(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	m := newTestMolecule("009")
	require.NoError(t, repo.Save(ctx, m))
	require.NoError(t, repo.Delete(ctx, m.ID))

	_, err := repo.FindByID(ctx, m.ID)
	require.Error(t, err)
}

func TestMoleculeRepository_Count(t *testing.T) {
	pool := startPostgres(t)
	applyMoleculeSchema(t, pool)
	repo := repositories.NewMoleculeRepository(pool, noopLogger{})
	ctx := context.Background()

	before, err := repo.Count(ctx)
	require.NoError(t, err)

	require.NoError(t, repo.Save(ctx, newTestMolecule("010")))

	after, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+1, after)
}

//Personal.AI order the ending
