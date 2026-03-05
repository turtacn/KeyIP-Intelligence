package repositories_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type MoleculeRepoIntegrationTestSuite struct {
	suite.Suite
	db     *sql.DB
	conn   *postgres.Connection
	repo   molecule.MoleculeRepository
	logger logging.Logger
}

func (s *MoleculeRepoIntegrationTestSuite) SetupSuite() {
	s.logger = logging.NewNopLogger()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		s.T().Skip("TEST_DATABASE_URL not set, skipping integration test")
		return
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		s.T().Fatalf("Failed to connect to test db: %v", err)
	}
	s.db = db
	s.conn = postgres.NewConnectionWithDB(db, s.logger)
	s.repo = repositories.NewPostgresMoleculeRepo(s.conn, s.logger)

	// Ensure pgvector extension and schema exist for tests
	_, err = db.Exec(`CREATE EXTENSION IF NOT EXISTS vector;`)
	if err != nil {
		s.T().Fatalf("Failed to create vector extension: %v", err)
	}

	_, err = db.Exec(`
		DROP TABLE IF EXISTS molecule_fingerprints CASCADE;
		DROP TABLE IF EXISTS molecule_properties CASCADE;
		DROP TABLE IF EXISTS patent_molecule_relations CASCADE;
		DROP TABLE IF EXISTS molecules CASCADE;
		DROP TYPE IF EXISTS molecule_status;

		CREATE TYPE molecule_status AS ENUM ('active', 'archived', 'deleted', 'pending_review', 'pending');

		CREATE TABLE molecules (
			id UUID PRIMARY KEY,
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

		CREATE TABLE molecule_fingerprints (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			molecule_id UUID NOT NULL REFERENCES molecules(id) ON DELETE CASCADE,
			fingerprint_type VARCHAR(32) NOT NULL,
			fingerprint_bits BIT VARYING,
			fingerprint_vector VECTOR(512),
			fingerprint_hash VARCHAR(128),
			parameters JSONB,
			model_version VARCHAR(32),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(molecule_id, fingerprint_type, model_version)
		);
	`)
	if err != nil {
		s.T().Fatalf("Failed to setup test schema: %v", err)
	}
}

func (s *MoleculeRepoIntegrationTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *MoleculeRepoIntegrationTestSuite) SetupTest() {
	if s.db != nil {
		_, err := s.db.Exec(`TRUNCATE TABLE molecules CASCADE;`)
		s.NoError(err)
	}
}

func (s *MoleculeRepoIntegrationTestSuite) TestBatchSave() {
	mols := []*molecule.Molecule{
		{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
			Status:          "active",
			Source:          "manual",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              uuid.New(),
			SMILES:          "CC",
			CanonicalSMILES: "CC",
			InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
			Status:          "active",
			Source:          "manual",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}

	affected, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)
	s.Equal(2, affected)

	// Verify they were saved
	count, err := s.repo.Count(context.Background(), &molecule.MoleculeQuery{})
	s.NoError(err)
	s.Equal(int64(2), count)

	// Test conflicting insert
	affected, err = s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)
	s.Equal(0, affected) // DO NOTHING should result in 0 rows affected
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearchAndCount() {
	mols := []*molecule.Molecule{
		{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
			MolecularWeight: 16.04,
			Status:          "active",
			Source:          "manual",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
		{
			ID:              uuid.New(),
			SMILES:          "CC",
			CanonicalSMILES: "CC",
			InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
			MolecularWeight: 30.07,
			Status:          "pending",
			Source:          "patent",
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	minWeight := float64(20.0)
	query := &molecule.MoleculeQuery{
		MinMolecularWeight: &minWeight,
		Statuses:           []molecule.MoleculeStatus{"pending"},
	}

	count, err := s.repo.Count(context.Background(), query)
	s.NoError(err)
	s.Equal(int64(1), count)

	result, err := s.repo.Search(context.Background(), query)
	s.NoError(err)
	s.NotNil(result)
	s.Len(result.Molecules, 1)
	s.Equal("CC", result.Molecules[0].SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearchByVectorSimilarity() {
	mol := &molecule.Molecule{
		ID:              uuid.New(),
		SMILES:          "C",
		CanonicalSMILES: "C",
		Status:          "active",
		Source:          "manual",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	// Save a vector directly using executor
	vec := []float32{0.1, 0.2, 0.3}
	query := `
		INSERT INTO molecule_fingerprints (
			molecule_id, fingerprint_type, fingerprint_vector
		) VALUES ($1, 'gnn_embedding', $2)
	`
	// Assuming pgvector uses pgvector.Vector
	_, err = s.db.Exec(query, mol.ID, pgvector.NewVector(vec))
	s.NoError(err)

	// Test Search
	searchVec := []float32{0.1, 0.2, 0.3} // exact match

	// Check if cast to repo implementation is needed
	impl, ok := s.repo.(interface{
		SearchByVectorSimilarity(ctx context.Context, embedding []float32, topK int) ([]*molecule.MoleculeWithScore, error)
	})
	s.True(ok)

	res, err := impl.SearchByVectorSimilarity(context.Background(), searchVec, 10)
	s.NoError(err)
	s.Len(res, 1)
	s.InDelta(1.0, res[0].Score, 0.0001) // Score should be ~1.0 for exact match
	s.Equal(mol.ID, res[0].Molecule.ID)
}

func TestMoleculeRepoIntegration(t *testing.T) {
	suite.Run(t, new(MoleculeRepoIntegrationTestSuite))
}
