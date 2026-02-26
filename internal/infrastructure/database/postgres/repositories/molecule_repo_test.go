package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type MoleculeRepoTestSuite struct {
	suite.Suite
	db   *sql.DB
	mock sqlmock.Sqlmock
	repo molecule.MoleculeRepository
	log  logging.Logger
}

func (s *MoleculeRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	require.NoError(s.T(), err)

	s.log = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.log)
	s.repo = NewPostgresMoleculeRepo(conn, s.log)
}

func (s *MoleculeRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *MoleculeRepoTestSuite) TestSave_Success() {
	mol := &molecule.Molecule{
		ID:              uuid.New(),
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.StatusActive,
	}

	s.mock.ExpectQuery("INSERT INTO molecules").
		WithArgs(
			mol.ID, mol.SMILES, mol.CanonicalSMILES, mol.InChI, mol.InChIKey,
			mol.MolecularFormula, mol.MolecularWeight, mol.ExactMass, mol.LogP, mol.TPSA,
			mol.NumAtoms, mol.NumBonds, mol.NumRings, mol.NumAromaticRings, mol.NumRotatableBonds,
			mol.Status, mol.Name, pq.Array(mol.Aliases), mol.Source, mol.SourceReference, sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))

	err := s.repo.Save(context.Background(), mol)
	assert.NoError(s.T(), err)
}

func (s *MoleculeRepoTestSuite) TestFindByID_Found() {
	id := uuid.New()
	s.mock.ExpectQuery("SELECT .* FROM molecules WHERE id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "smiles", "canonical_smiles", "inchi", "inchi_key",
			"molecular_formula", "molecular_weight", "exact_mass", "logp", "tpsa",
			"num_atoms", "num_bonds", "num_rings", "num_aromatic_rings", "num_rotatable_bonds",
			"status", "name", "aliases", "source", "source_reference", "metadata",
			"created_at", "updated_at", "deleted_at",
		}).AddRow(
			id, "C", "C", "", "KEY",
			"CH4", 16.04, 16.03, 0.6, 0.0,
			5, 4, 0, 0, 0,
			molecule.StatusActive, "Methane", pq.Array([]string{}), "manual", "", nil,
			time.Now(), time.Now(), nil,
		))

	m, err := s.repo.FindByID(context.Background(), id.String())
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), id, m.ID)
}

func (s *MoleculeRepoTestSuite) TestFindBySMILES_Found() {
	smiles := "C"
	s.mock.ExpectQuery("SELECT .* FROM molecules WHERE canonical_smiles =").
		WithArgs(smiles).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "smiles", "canonical_smiles", "inchi", "inchi_key",
			"molecular_formula", "molecular_weight", "exact_mass", "logp", "tpsa",
			"num_atoms", "num_bonds", "num_rings", "num_aromatic_rings", "num_rotatable_bonds",
			"status", "name", "aliases", "source", "source_reference", "metadata",
			"created_at", "updated_at", "deleted_at",
		}).AddRow(
			uuid.New(), "C", "C", "", "KEY",
			"CH4", 16.04, 16.03, 0.6, 0.0,
			5, 4, 0, 0, 0,
			molecule.StatusActive, "Methane", pq.Array([]string{}), "manual", "", nil,
			time.Now(), time.Now(), nil,
		))

	mols, err := s.repo.FindBySMILES(context.Background(), smiles)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), mols, 1)
}

func (s *MoleculeRepoTestSuite) TestUpdate_Success() {
	mol := &molecule.Molecule{
		ID:     uuid.New(),
		SMILES: "CC",
		Status: molecule.StatusActive,
	}

	s.mock.ExpectExec("UPDATE molecules SET").
		WithArgs(
			mol.ID, mol.SMILES, mol.CanonicalSMILES, mol.InChI, mol.InChIKey,
			mol.MolecularFormula, mol.MolecularWeight, mol.ExactMass, mol.LogP, mol.TPSA,
			mol.NumAtoms, mol.NumBonds, mol.NumRings, mol.NumAromaticRings, mol.NumRotatableBonds,
			mol.Status, mol.Name, pq.Array(mol.Aliases), mol.Source, mol.SourceReference, sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.Update(context.Background(), mol)
	assert.NoError(s.T(), err)
}

func (s *MoleculeRepoTestSuite) TestDelete_Success() {
	id := uuid.New()
	s.mock.ExpectExec("UPDATE molecules SET deleted_at").
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.Delete(context.Background(), id.String())
	assert.NoError(s.T(), err)
}

func (s *MoleculeRepoTestSuite) TestExists_True() {
	id := uuid.New()
	s.mock.ExpectQuery("SELECT EXISTS").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := s.repo.Exists(context.Background(), id.String())
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *MoleculeRepoTestSuite) TestSearch_Keyword() {
	query := &molecule.MoleculeQuery{
		Keyword: "Methane",
		Limit:   10,
	}

	// Only 1 argument is passed, referenced twice as $1
	s.mock.ExpectQuery("SELECT COUNT").
		WithArgs("%Methane%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	s.mock.ExpectQuery("SELECT .* FROM molecules").
		WithArgs("%Methane%", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "smiles", "canonical_smiles", "inchi", "inchi_key",
			"molecular_formula", "molecular_weight", "exact_mass", "logp", "tpsa",
			"num_atoms", "num_bonds", "num_rings", "num_aromatic_rings", "num_rotatable_bonds",
			"status", "name", "aliases", "source", "source_reference", "metadata",
			"created_at", "updated_at", "deleted_at",
		}).AddRow(
			uuid.New(), "C", "C", "", "KEY",
			"CH4", 16.04, 16.03, 0.6, 0.0,
			5, 4, 0, 0, 0,
			molecule.StatusActive, "Methane", pq.Array([]string{}), "manual", "", nil,
			time.Now(), time.Now(), nil,
		))

	res, err := s.repo.Search(context.Background(), query)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), res)
	assert.Equal(s.T(), int64(1), res.Total)
	assert.Len(s.T(), res.Molecules, 1)
}

func (s *MoleculeRepoTestSuite) TestBatchSave_Success() {
	// Note: The implementation uses loop + transaction.
	// Mocking transaction is complex with sqlmock if we don't mock Begin.
	// But WithTx uses conn.DB().BeginTx.

	s.mock.ExpectBegin()
	s.mock.ExpectQuery("INSERT INTO molecules").WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(time.Now(), time.Now()))
	s.mock.ExpectQuery("INSERT INTO molecules").WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(time.Now(), time.Now()))
	s.mock.ExpectCommit()

	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C"},
		{ID: uuid.New(), SMILES: "CC"},
	}

	count, err := s.repo.BatchSave(context.Background(), mols)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 2, count)
}

func (s *MoleculeRepoTestSuite) TestFindWithFingerprint() {
	fpType := molecule.FingerprintTypeMorgan
	s.mock.ExpectQuery("SELECT m.* FROM molecules m JOIN molecule_fingerprints f").
		WithArgs(fpType, 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "smiles", "canonical_smiles", "inchi", "inchi_key",
			"molecular_formula", "molecular_weight", "exact_mass", "logp", "tpsa",
			"num_atoms", "num_bonds", "num_rings", "num_aromatic_rings", "num_rotatable_bonds",
			"status", "name", "aliases", "source", "source_reference", "metadata",
			"created_at", "updated_at", "deleted_at",
		}))

	mols, err := s.repo.FindWithFingerprint(context.Background(), fpType, 0, 10)
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), mols)
}

func TestMoleculeRepo(t *testing.T) {
	suite.Run(t, new(MoleculeRepoTestSuite))
}
//Personal.AI order the ending
