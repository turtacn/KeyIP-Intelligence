package repositories

import (
	"context"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
)

func (s *MoleculeRepoTestSuite) TestFindByIDs() {
	id := uuid.New()
	idStr := id.String()

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

	s.mock.ExpectQuery("SELECT \\* FROM molecules WHERE id = ANY\\(\\$1\\)").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(row)

	mols, err := s.repo.FindByIDs(context.Background(), []string{idStr})
	s.NoError(err)
	s.Len(mols, 1)
	s.Equal(id, mols[0].ID)
}

func (s *MoleculeRepoTestSuite) TestExists() {
	id := uuid.New()
	s.mock.ExpectQuery("SELECT EXISTS").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := s.repo.Exists(context.Background(), id.String())
	s.NoError(err)
	s.True(exists)
}

func (s *MoleculeRepoTestSuite) TestExistsByInChIKey() {
	s.mock.ExpectQuery("SELECT EXISTS").
		WithArgs("VNWKTOKETHGBQD-UHFFFAOYSA-N").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	exists, err := s.repo.ExistsByInChIKey(context.Background(), "VNWKTOKETHGBQD-UHFFFAOYSA-N")
	s.NoError(err)
	s.True(exists)
}

func (s *MoleculeRepoTestSuite) TestFindBySource() {
	cols := []string{
		"id", "smiles", "canonical_smiles", "inchi", "inchi_key", "molecular_formula", "molecular_weight",
		"exact_mass", "logp", "tpsa", "num_atoms", "num_bonds", "num_rings", "num_aromatic_rings",
		"num_rotatable_bonds", "status", "name", "aliases", "source", "source_reference", "metadata",
		"created_at", "updated_at", "deleted_at",
	}

	id := uuid.New()
	row := sqlmock.NewRows(cols).AddRow(
		id, "C", "C", "InChI=1S/CH4/h1H4", "VNWKTOKETHGBQD-UHFFFAOYSA-N", "CH4", 16.04,
		16.0313, 0.0, 0.0, 1, 4, 0, 0,
		0, "active", "Methane", []uint8("{}"), "manual", "", []byte("{}"),
		time.Now(), time.Now(), nil,
	)

	s.mock.ExpectQuery("SELECT \\* FROM molecules WHERE source = \\$1").
		WithArgs("manual", 10, 0).
		WillReturnRows(row)

	mols, err := s.repo.FindBySource(context.Background(), "manual", 0, 10)
	s.NoError(err)
	s.Len(mols, 1)
}

func (s *MoleculeRepoTestSuite) TestAddProperty() {
	prop := &molecule.Property{
		MoleculeID:      uuid.New(),
		Type:            "boiling_point",
		Value:           100.0,
		Unit:            "C",
		DataSource:      "experiment",
		Confidence:      0.9,
		SourceReference: "ref1",
	}

	s.mock.ExpectQuery("INSERT INTO molecule_properties").
		WithArgs(prop.MoleculeID, prop.Type, prop.Value, prop.Unit, sqlmock.AnyArg(), prop.DataSource, prop.Confidence, prop.SourceReference).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(uuid.New(), time.Now()))

	// Use interface assertion to access the method not exposed in the generic MoleculeRepository interface
	repoExt, ok := s.repo.(interface {
		AddProperty(ctx context.Context, prop *molecule.Property) error
	})
	if !ok {
		s.T().Skip("Repository does not implement AddProperty")
	}

	err := repoExt.AddProperty(context.Background(), prop)
	s.NoError(err)
	s.NotEqual(uuid.Nil, prop.ID)
}

func (s *MoleculeRepoTestSuite) TestDeleteFingerprints() {
	molID := uuid.New()
	s.mock.ExpectExec("DELETE FROM molecule_fingerprints WHERE molecule_id = \\$1 AND fingerprint_type = \\$2").
		WithArgs(molID, "morgan").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Use interface assertion to access the method not exposed in the generic MoleculeRepository interface
	repoExt, ok := s.repo.(interface {
		DeleteFingerprints(ctx context.Context, moleculeID uuid.UUID, fpType string) error
	})
	if !ok {
		s.T().Skip("Repository does not implement DeleteFingerprints")
	}

	err := repoExt.DeleteFingerprints(context.Background(), molID, "morgan")
	s.NoError(err)
}
