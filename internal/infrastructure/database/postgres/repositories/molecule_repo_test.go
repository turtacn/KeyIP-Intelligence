package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type MoleculeRepoTestSuite struct {
	suite.Suite
	mock   sqlmock.Sqlmock
	db     *sql.DB
	repo   molecule.MoleculeRepository
	logger logging.Logger
}

func (s *MoleculeRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	s.NoError(err)

	s.logger = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.logger)
	s.repo = NewPostgresMoleculeRepo(conn, s.logger)
}

func (s *MoleculeRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *MoleculeRepoTestSuite) TestFindByID_Found() {
	id := uuid.New()
	idStr := id.String()

	// Columns: id, smiles, canonical_smiles, inchi, inchi_key, molecular_formula, molecular_weight,
	// exact_mass, logp, tpsa, num_atoms, num_bonds, num_rings, num_aromatic_rings,
	// num_rotatable_bonds, status, name, aliases, source, source_reference, metadata,
	// created_at, updated_at, deleted_at

	cols := []string{
		"id", "smiles", "canonical_smiles", "inchi", "inchi_key", "molecular_formula", "molecular_weight",
		"exact_mass", "logp", "tpsa", "num_atoms", "num_bonds", "num_rings", "num_aromatic_rings",
		"num_rotatable_bonds", "status", "name", "aliases", "source", "source_reference", "metadata",
		"created_at", "updated_at", "deleted_at",
	}

	row := sqlmock.NewRows(cols).AddRow(
		id, "C", "C", "InChI=1S/CH4/h1H4", "VNWKTOKETHGBQD-UHFFFAOYSA-N", "CH4", 16.04,
		16.0313, 0.0, 0.0, 1, 4, 0, 0,
		0, "active", "Methane", []uint8("{}"), "manual", "", []byte("{}"),
		time.Now(), time.Now(), nil,
	)

	// Expect Main Query
	s.mock.ExpectQuery("SELECT \\* FROM molecules WHERE id = \\$1 AND deleted_at IS NULL").
		WithArgs(id).
		WillReturnRows(row)

	// Expect Fingerprints Query (Preload)
	s.mock.ExpectQuery("SELECT \\* FROM molecule_fingerprints WHERE molecule_id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{}))

	m, err := s.repo.FindByID(context.Background(), idStr)
	s.NoError(err)
	s.NotNil(m)
	s.Equal(id, m.ID)
	s.Equal("C", m.SMILES)
}

func (s *MoleculeRepoTestSuite) TestSave_Success() {
	id := uuid.New()
	mol := &molecule.Molecule{
		SMILES: "C",
		Status: "active",
	}

	s.mock.ExpectQuery("INSERT INTO molecules").
		WithArgs(mol.SMILES, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), mol.Status, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(id, time.Now(), time.Now()))

	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)
	s.Equal(id, mol.ID)
}

func (s *MoleculeRepoTestSuite) TestUpdate_Success() {
	id := uuid.New()
	mol := &molecule.Molecule{ID: id, Status: "active", Name: "Methane"}

	s.mock.ExpectExec("UPDATE molecules").
		WithArgs(mol.Status, mol.Name, sqlmock.AnyArg(), sqlmock.AnyArg(), mol.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.repo.Update(context.Background(), mol)
	s.NoError(err)
}

func (s *MoleculeRepoTestSuite) TestUpdate_NotFound() {
	id := uuid.New()
	mol := &molecule.Molecule{ID: id}

	s.mock.ExpectExec("UPDATE molecules").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := s.repo.Update(context.Background(), mol)
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *MoleculeRepoTestSuite) TestDelete_Success() {
	id := uuid.New()
	s.mock.ExpectExec("UPDATE molecules SET deleted_at = NOW\\(\\) WHERE id = \\$1").
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.repo.Delete(context.Background(), id.String())
	s.NoError(err)
}

func TestMoleculeRepoTestSuite(t *testing.T) {
	suite.Run(t, new(MoleculeRepoTestSuite))
}

//Personal.AI order the ending
