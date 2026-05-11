//go:build integration

package repositories_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// benchmarkMoleculeSuite holds the shared DB connection and repo for molecule benchmarks.
type benchmarkMoleculeSuite struct {
	db   *sql.DB
	conn *postgres.Connection
	repo molecule.MoleculeRepository
}

// setupBenchmarkMolecule connects to the test database and creates the schema.
// It is called once per benchmark via b.Run with a setup step.
func setupBenchmarkMolecule() (*benchmarkMoleculeSuite, func()) {
	logger := logging.NewNopLogger()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/keyip_test?sslmode=disable"
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(fmt.Sprintf("failed to connect: %v", err))
	}

	conn := postgres.NewConnectionWithDB(db, logger)
	repo := repositories.NewPostgresMoleculeRepo(conn, logger)

	// Setup schema (molecules table only; benchmarks do not exercise fingerprint tables).
	mustExec(db, `
		DROP TABLE IF EXISTS molecule_fingerprints CASCADE;
		DROP TABLE IF EXISTS molecule_properties CASCADE;
		DROP TABLE IF EXISTS patent_molecule_relations CASCADE;
		DROP TABLE IF EXISTS molecules CASCADE;
		DROP TYPE IF EXISTS molecule_status;

		CREATE TYPE molecule_status AS ENUM ('active', 'archived', 'deleted', 'pending_review', 'pending');

		CREATE TABLE molecules (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			smiles TEXT NOT NULL,
			canonical_smiles TEXT NOT NULL,
			inchi TEXT,
			inchi_key VARCHAR(27) UNIQUE,
			molecular_formula VARCHAR(256),
			molecular_weight DOUBLE PRECISION,
			exact_mass DOUBLE PRECISION,
			logp DOUBLE PRECISION,
			tpsa DOUBLE PRECISION,
			num_atoms INTEGER,
			num_bonds INTEGER,
			num_rings INTEGER,
			num_aromatic_rings INTEGER,
			num_rotatable_bonds INTEGER,
			status molecule_status NOT NULL DEFAULT 'active',
			name VARCHAR(512),
			aliases TEXT[],
			source VARCHAR(32) NOT NULL DEFAULT 'manual',
			source_reference VARCHAR(512),
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			deleted_at TIMESTAMPTZ
		);
	`)

	cleanup := func() {
		db.Close()
	}

	return &benchmarkMoleculeSuite{db: db, conn: conn, repo: repo}, cleanup
}

func mustExec(db *sql.DB, query string, args ...interface{}) {
	if _, err := db.Exec(query, args...); err != nil {
		panic(fmt.Sprintf("exec failed: %v\nquery: %s", err, query))
	}
}

// makeMol creates a molecule with synthetic data for benchmarks.
func makeMol(id uuid.UUID, idx int) *molecule.Molecule {
	smiles := fmt.Sprintf("C%d", idx)
	// InChIKey must be exactly 27 characters. We derive a unique key from the
	// input UUID, which itself is unique, so collisions are impossible.
	// UUID string is 36 characters: "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	// First 27 chars: "f47ac10b-58cc-4372-a567-0e0"
	inchiKey := id.String()[:27]
	return &molecule.Molecule{
		ID:                id,
		SMILES:            smiles,
		CanonicalSMILES:   smiles,
		InChI:             fmt.Sprintf("InChI=1S/CH%d/h1H", idx),
		InChIKey:          inchiKey,
		MolecularFormula:  fmt.Sprintf("C%d", idx),
		MolecularWeight:   12.01 + float64(idx)*1.0,
		ExactMass:         12.0 + float64(idx)*1.0,
		LogP:              0.5 + float64(idx)*0.1,
		TPSA:              float64(idx),
		NumAtoms:          idx + 1,
		NumBonds:          idx,
		NumRings:          0,
		NumAromaticRings:  0,
		NumRotatableBonds: 0,
		Status:            molecule.MoleculeStatusActive,
		Name:              fmt.Sprintf("Molecule_%d", idx),
		Aliases:           []string{fmt.Sprintf("alias_%d", idx)},
		Source:            molecule.SourceManual,
		SourceReference:   "benchmark",
		Metadata:          map[string]any{"bench": "mark"},
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

// BenchmarkMoleculeCreate benchmarks single molecule insertion.
func BenchmarkMoleculeCreate(b *testing.B) {
	s, cleanup := setupBenchmarkMolecule()
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Each iteration inserts a new molecule
		id := uuid.New()
		mol := makeMol(id, i)
		if err := s.repo.Save(ctx, mol); err != nil {
			b.Fatalf("Save failed: %v", err)
		}
	}
}

// BenchmarkMoleculeFindByID benchmarks primary key lookup.
func BenchmarkMoleculeFindByID(b *testing.B) {
	s, cleanup := setupBenchmarkMolecule()
	defer cleanup()

	ctx := context.Background()

	// Setup: insert N molecules
	// Note: Create() does NOT insert the pre-set ID; the DB auto-generates
	// one via gen_random_uuid() and overwrites mol.ID via RETURNING.  We
	// must capture mol.ID *after* Save() returns.
	const numMols = 100
	ids := make([]string, numMols)
	for i := 0; i < numMols; i++ {
		mol := makeMol(uuid.New(), i)
		if err := s.repo.Save(ctx, mol); err != nil {
			b.Fatalf("setup save failed: %v", err)
		}
		ids[i] = mol.ID.String()
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		idx := i % numMols
		mol, err := s.repo.FindByID(ctx, ids[idx])
		if err != nil {
			b.Fatalf("FindByID failed at idx %d/%d: %v", idx, numMols, err)
		}
		if mol == nil {
			b.Fatal("FindByID returned nil")
		}
	}
}

// BenchmarkMoleculeFindBySMILES benchmarks text search by SMILES string.
func BenchmarkMoleculeFindBySMILES(b *testing.B) {
	s, cleanup := setupBenchmarkMolecule()
	defer cleanup()

	ctx := context.Background()

	// Setup: insert N molecules
	// Note: after Save(), mol.ID is overwritten by the DB-generated UUID.
	// We must store IDs/data AFTER the call.
	const numMols = 100
	smilesStrings := make([]string, numMols)
	for i := 0; i < numMols; i++ {
		mol := makeMol(uuid.New(), i)
		if err := s.repo.Save(ctx, mol); err != nil {
			b.Fatalf("setup save failed: %v", err)
		}
		smilesStrings[i] = mol.CanonicalSMILES
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		smiles := smilesStrings[i%numMols]
		mols, err := s.repo.FindBySMILES(ctx, smiles)
		if err != nil {
			b.Fatalf("FindBySMILES failed: %v", err)
		}
		if len(mols) == 0 {
			b.Fatal("FindBySMILES returned empty")
		}
	}
}

// BenchmarkMoleculeBulkCreate100 benchmarks batch insert of 100 molecules.
func BenchmarkMoleculeBulkCreate100(b *testing.B) {
	benchmarkBulkCreate(b, 100)
}

// BenchmarkMoleculeBulkCreate1000 benchmarks batch insert of 1000 molecules.
func BenchmarkMoleculeBulkCreate1000(b *testing.B) {
	benchmarkBulkCreate(b, 1000)
}

func benchmarkBulkCreate(b *testing.B, batchSize int) {
	s, cleanup := setupBenchmarkMolecule()
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mols := make([]*molecule.Molecule, batchSize)
		for j := 0; j < batchSize; j++ {
			id := uuid.New()
			mols[j] = makeMol(id, i*batchSize+j)
		}

		affected, err := s.repo.BatchSave(ctx, mols)
		if err != nil {
			b.Fatalf("BatchSave failed: %v", err)
		}
		if affected != batchSize {
			b.Fatalf("expected %d affected, got %d", batchSize, affected)
		}
	}
}

// BenchmarkMoleculeSearch benchmarks a complex search query with status and source filters.
func BenchmarkMoleculeSearch(b *testing.B) {
	s, cleanup := setupBenchmarkMolecule()
	defer cleanup()

	ctx := context.Background()

	// Setup: insert mixed molecules
	const numMols = 500
	for i := 0; i < numMols; i++ {
		id := uuid.New()
		mol := makeMol(id, i)
		// Alternate status/source for realistic filtering
		if i%2 == 0 {
			mol.Status = molecule.MoleculeStatusActive
			mol.Source = molecule.SourceManual
		} else {
			mol.Status = molecule.MoleculeStatusPending
			mol.Source = molecule.SourcePatent
		}
		if err := s.repo.Save(ctx, mol); err != nil {
			b.Fatalf("setup save failed: %v", err)
		}
	}

	// Verify we have data
	count, err := s.repo.Count(ctx, &molecule.MoleculeQuery{})
	if err != nil {
		b.Fatalf("count failed: %v", err)
	}
	if count == 0 {
		b.Fatal("no data in table")
	}

	minWeight := float64(50.0)
	maxWeight := float64(500.0)
	query := &molecule.MoleculeQuery{
		Statuses:           []molecule.MoleculeStatus{molecule.MoleculeStatusActive},
		Sources:            []molecule.MoleculeSource{molecule.SourceManual},
		MinMolecularWeight: &minWeight,
		MaxMolecularWeight: &maxWeight,
		Limit:              50,
		Offset:             0,
		SortBy:             "molecular_weight",
		SortOrder:          "asc",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result, err := s.repo.Search(ctx, query)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
		if result == nil {
			b.Fatal("Search returned nil")
		}
	}
}
