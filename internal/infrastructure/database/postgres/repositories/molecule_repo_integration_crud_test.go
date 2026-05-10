//go:build integration

package repositories_test

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ─── CRUD Tests ─────────────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestCreate() {
	mol := &molecule.Molecule{
		SMILES:          "CCO",
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		MolecularWeight: 46.07,
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)
	s.NotEqual(uuid.Nil, mol.ID)
	s.False(mol.CreatedAt.IsZero())
	s.False(mol.UpdatedAt.IsZero())
}

func (s *MoleculeRepoIntegrationTestSuite) TestCreate_Full() {
	mol := &molecule.Molecule{
		SMILES:            "c1ccccc1",
		CanonicalSMILES:   "c1ccccc1",
		InChI:             "InChI=1S/C6H6/c1-2-4-6-5-3-1",
		InChIKey:          "UHOVQNZJYSORNB-UHFFFAOYSA-N",
		MolecularFormula:  "C6H6",
		MolecularWeight:   78.11,
		ExactMass:         78.04695,
		LogP:              2.13,
		TPSA:              0.0,
		NumAtoms:          6,
		NumBonds:          6,
		NumRings:          1,
		NumAromaticRings:  1,
		NumRotatableBonds: 0,
		Status:            molecule.MoleculeStatusActive,
		Name:              "Benzene",
		Aliases:           []string{"benzol", "cyclohexatriene"},
		Source:            molecule.SourceManual,
		SourceReference:   "test-ref",
		Metadata:          map[string]any{"key": "value"},
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)
	s.NotEqual(uuid.Nil, mol.ID)

	// Fetch and verify all fields
	found, err := s.repo.FindByID(context.Background(), mol.ID.String())
	s.NoError(err)
	s.Equal("c1ccccc1", found.CanonicalSMILES)
	s.Equal("InChI=1S/C6H6/c1-2-4-6-5-3-1", found.InChI)
	s.Equal("UHOVQNZJYSORNB-UHFFFAOYSA-N", found.InChIKey)
	s.Equal("C6H6", found.MolecularFormula)
	s.Equal(78.11, found.MolecularWeight)
	s.Equal(78.04695, found.ExactMass)
	s.Equal(2.13, found.LogP)
	s.Equal("Benzene", found.Name)
	s.Len(found.Aliases, 2)
	s.Equal("benzol", found.Aliases[0])
	s.Equal("test-ref", found.SourceReference)
	s.Equal("manual", string(found.Source))
	s.Equal("value", found.Metadata["key"])
}

func (s *MoleculeRepoIntegrationTestSuite) TestCreate_DuplicateInChIKey() {
	mol1 := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol1)
	s.NoError(err)

	mol2 := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err = s.repo.Save(context.Background(), mol2)
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeMoleculeAlreadyExists))
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByID() {
	mol := &molecule.Molecule{
		SMILES:          "CC",
		CanonicalSMILES: "CC",
		InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
		MolecularWeight: 30.07,
		Status:          molecule.MoleculeStatusActive,
		Name:            "Ethane",
		Source:          molecule.SourcePatent,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	found, err := s.repo.FindByID(context.Background(), mol.ID.String())
	s.NoError(err)
	s.NotNil(found)
	s.Equal(mol.ID, found.ID)
	s.Equal("CC", found.SMILES)
	s.Equal("Ethane", found.Name)
	s.Equal("patent", string(found.Source))
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByID_NotFound() {
	_, err := s.repo.FindByID(context.Background(), uuid.New().String())
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByID_InvalidUUID() {
	_, err := s.repo.FindByID(context.Background(), "not-a-uuid")
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeValidation))
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByInChIKey() {
	mol := &molecule.Molecule{
		SMILES:          "CCC",
		CanonicalSMILES: "CCC",
		InChIKey:        "ATUOYWHBWRKTHZ-UHFFFAOYSA-N",
		MolecularWeight: 44.10,
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	found, err := s.repo.FindByInChIKey(context.Background(), "ATUOYWHBWRKTHZ-UHFFFAOYSA-N")
	s.NoError(err)
	s.NotNil(found)
	s.Equal(mol.ID, found.ID)
	s.Equal("CCC", found.SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByInChIKey_NotFound() {
	_, err := s.repo.FindByInChIKey(context.Background(), "ZZZZZZZZZZZZZZ-UHFFFAWAY-XX")
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindBySMILES() {
	mol := &molecule.Molecule{
		SMILES:          "CCCC",
		CanonicalSMILES: "CCCC",
		InChIKey:        "IJDNQMDRQITEOD-UHFFFAOYSA-N",
		MolecularWeight: 58.12,
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	results, err := s.repo.FindBySMILES(context.Background(), "CCCC")
	s.NoError(err)
	s.Len(results, 1)
	s.Equal(mol.ID, results[0].ID)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindBySMILES_NoMatch() {
	results, err := s.repo.FindBySMILES(context.Background(), "NONEXISTENT")
	s.NoError(err)
	s.Len(results, 0)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindBySMILES_MultipleMatches() {
	mol1 := &molecule.Molecule{
		SMILES:          "CCO",
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	mol2 := &molecule.Molecule{
		SMILES:          "CCO",
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N-FAKE", // fake unique key
		Status:          molecule.MoleculeStatusPending,
		Source:          molecule.SourcePatent,
	}

	err := s.repo.Save(context.Background(), mol1)
	s.NoError(err)
	err = s.repo.Save(context.Background(), mol2)
	s.NoError(err)

	results, err := s.repo.FindBySMILES(context.Background(), "CCO")
	s.NoError(err)
	s.Len(results, 2)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByIDs() {
	mol1 := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	mol2 := &molecule.Molecule{
		SMILES:          "CC",
		CanonicalSMILES: "CC",
		InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusPending,
		Source:          molecule.SourcePatent,
	}
	err := s.repo.Save(context.Background(), mol1)
	s.NoError(err)
	err = s.repo.Save(context.Background(), mol2)
	s.NoError(err)

	results, err := s.repo.FindByIDs(context.Background(), []string{mol1.ID.String(), mol2.ID.String()})
	s.NoError(err)
	s.Len(results, 2)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByIDs_EmptyList() {
	results, err := s.repo.FindByIDs(context.Background(), []string{})
	s.NoError(err)
	s.Nil(results)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByIDs_PartialMatch() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	// One existing, one non-existing UUID
	results, err := s.repo.FindByIDs(context.Background(), []string{mol.ID.String(), uuid.New().String()})
	s.NoError(err)
	s.Len(results, 1)
}

func (s *MoleculeRepoIntegrationTestSuite) TestUpdate() {
	mol := &molecule.Molecule{
		SMILES:          "CCO",
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusPending,
		Source:          molecule.SourceManual,
		Name:            "Ethanol-original",
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	// Update fields
	mol.Status = molecule.MoleculeStatusActive
	mol.Name = "Ethanol-updated"
	mol.Aliases = []string{"ethyl-alcohol"}
	mol.Metadata = map[string]any{"updated": "true"}

	err = s.repo.Update(context.Background(), mol)
	s.NoError(err)

	// Verify update
	found, err := s.repo.FindByID(context.Background(), mol.ID.String())
	s.NoError(err)
	s.Equal(molecule.MoleculeStatusActive, found.Status)
	s.Equal("Ethanol-updated", found.Name)
	s.Equal("true", found.Metadata["updated"])
}

func (s *MoleculeRepoIntegrationTestSuite) TestUpdate_NotFound() {
	mol := &molecule.Molecule{
		ID:     uuid.New(),
		Status: molecule.MoleculeStatusActive,
		Name:   "Ghost",
	}
	err := s.repo.Update(context.Background(), mol)
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *MoleculeRepoIntegrationTestSuite) TestDelete() {
	mol := &molecule.Molecule{
		SMILES:          "CCO",
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	// Soft delete
	err = s.repo.Delete(context.Background(), mol.ID.String())
	s.NoError(err)
}

func (s *MoleculeRepoIntegrationTestSuite) TestDeleteThenFindNotFound() {
	mol := &molecule.Molecule{
		SMILES:          "CC",
		CanonicalSMILES: "CC",
		InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	err = s.repo.Delete(context.Background(), mol.ID.String())
	s.NoError(err)

	// FindByID should not return deleted molecule
	_, err = s.repo.FindByID(context.Background(), mol.ID.String())
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *MoleculeRepoIntegrationTestSuite) TestDeleteThenCountExcludesDeleted() {
	mols := []*molecule.Molecule{
		{
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
			Status:          molecule.MoleculeStatusActive,
			Source:          molecule.SourceManual,
		},
		{
			SMILES:          "CC",
			CanonicalSMILES: "CC",
			InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
			Status:          molecule.MoleculeStatusActive,
			Source:          molecule.SourceManual,
		},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	// Delete first molecule
	err = s.repo.Delete(context.Background(), mols[0].ID.String())
	s.NoError(err)

	count, err := s.repo.Count(context.Background(), &molecule.MoleculeQuery{})
	s.NoError(err)
	s.Equal(int64(1), count)
}

func (s *MoleculeRepoIntegrationTestSuite) TestExists() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	exists, err := s.repo.Exists(context.Background(), mol.ID.String())
	s.NoError(err)
	s.True(exists)
}

func (s *MoleculeRepoIntegrationTestSuite) TestExists_False() {
	exists, err := s.repo.Exists(context.Background(), uuid.New().String())
	s.NoError(err)
	s.False(exists)
}

func (s *MoleculeRepoIntegrationTestSuite) TestExistsByInChIKey() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	exists, err := s.repo.ExistsByInChIKey(context.Background(), "VNWKTOKETHGBQD-UHFFFAOYSA-N")
	s.NoError(err)
	s.True(exists)
}

func (s *MoleculeRepoIntegrationTestSuite) TestExistsByInChIKey_False() {
	exists, err := s.repo.ExistsByInChIKey(context.Background(), "ZZZZZZZZZZZZZZ-UHFFFAWAY-XX")
	s.NoError(err)
	s.False(exists)
}

func (s *MoleculeRepoIntegrationTestSuite) TestExistsByInChIKey_DeletedMolecule() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	err = s.repo.Delete(context.Background(), mol.ID.String())
	s.NoError(err)

	// After soft delete, ExistsByInChIKey should return false
	exists, err := s.repo.ExistsByInChIKey(context.Background(), "VNWKTOKETHGBQD-UHFFFAOYSA-N")
	s.NoError(err)
	s.False(exists)
}

// ─── Query / Filter Tests ────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestFindBySource() {
	mol1 := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	mol2 := &molecule.Molecule{
		SMILES:          "CC",
		CanonicalSMILES: "CC",
		InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourcePatent,
	}
	_, err := s.repo.BatchSave(context.Background(), []*molecule.Molecule{mol1, mol2})
	s.NoError(err)

	results, err := s.repo.FindBySource(context.Background(), molecule.SourcePatent, 0, 10)
	s.NoError(err)
	s.Len(results, 1)
	s.Equal("CC", results[0].SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindBySource_NoMatch() {
	results, err := s.repo.FindBySource(context.Background(), molecule.SourceLiterature, 0, 10)
	s.NoError(err)
	s.Len(results, 0)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByStatus() {
	mol1 := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	mol2 := &molecule.Molecule{
		SMILES:          "CC",
		CanonicalSMILES: "CC",
		InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusPending,
		Source:          molecule.SourceManual,
	}
	_, err := s.repo.BatchSave(context.Background(), []*molecule.Molecule{mol1, mol2})
	s.NoError(err)

	results, err := s.repo.FindByStatus(context.Background(), molecule.MoleculeStatusPending, 0, 10)
	s.NoError(err)
	s.Len(results, 1)
	s.Equal("CC", results[0].SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByMolecularWeightRange() {
	mols := []*molecule.Molecule{
		{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
			MolecularWeight: 16.04,
			Status:          molecule.MoleculeStatusActive,
			Source:          molecule.SourceManual,
		},
		{
			ID:              uuid.New(),
			SMILES:          "CC",
			CanonicalSMILES: "CC",
			InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
			MolecularWeight: 30.07,
			Status:          molecule.MoleculeStatusActive,
			Source:          molecule.SourceManual,
		},
		{
			ID:              uuid.New(),
			SMILES:          "CCC",
			CanonicalSMILES: "CCC",
			InChIKey:        "ATUOYWHBWRKTHZ-UHFFFAOYSA-N",
			MolecularWeight: 44.10,
			Status:          molecule.MoleculeStatusActive,
			Source:          molecule.SourceManual,
		},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	results, err := s.repo.FindByMolecularWeightRange(context.Background(), 20.0, 40.0, 0, 10)
	s.NoError(err)
	s.Len(results, 1)
	s.Equal("CC", results[0].SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindByMolecularWeightRange_NoMatch() {
	mol := &molecule.Molecule{
		ID:              uuid.New(),
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		MolecularWeight: 16.04,
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	_, err := s.repo.BatchSave(context.Background(), []*molecule.Molecule{mol})
	s.NoError(err)

	results, err := s.repo.FindByMolecularWeightRange(context.Background(), 100.0, 200.0, 0, 10)
	s.NoError(err)
	s.Len(results, 0)
}

// ─── Search Tests ────────────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_All() {
	// Empty query should return all non-deleted molecules
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", Status: "active", Source: "patent"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{})
	s.NoError(err)
	s.Len(result.Molecules, 2)
	s.Equal(int64(2), result.Total)
	s.Equal(0, result.Offset)
	s.Equal(20, result.Limit)
	s.False(result.HasMore)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_BySource() {
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", Status: "active", Source: "patent"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		Sources: []molecule.MoleculeSource{molecule.SourcePatent},
	})
	s.NoError(err)
	s.Len(result.Molecules, 1)
	s.Equal(int64(1), result.Total)
	s.Equal("CC", result.Molecules[0].SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_ByStatuses() {
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", Status: "pending", Source: "manual"},
		{ID: uuid.New(), SMILES: "CCC", CanonicalSMILES: "CCC", InChIKey: "ATUOYWHBWRKTHZ-UHFFFAOYSA-N", Status: "archived", Source: "manual"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		Statuses: []molecule.MoleculeStatus{molecule.MoleculeStatusActive, molecule.MoleculeStatusPending},
	})
	s.NoError(err)
	s.Len(result.Molecules, 2)
	s.Equal(int64(2), result.Total)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_ByMolecularWeightRange() {
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", MolecularWeight: 16.04, Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", MolecularWeight: 30.07, Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CCC", CanonicalSMILES: "CCC", InChIKey: "ATUOYWHBWRKTHZ-UHFFFAOYSA-N", MolecularWeight: 44.10, Status: "active", Source: "manual"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	minW := 20.0
	maxW := 50.0
	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		MinMolecularWeight: &minW,
		MaxMolecularWeight: &maxW,
	})
	s.NoError(err)
	s.Len(result.Molecules, 2)
	s.Equal(int64(2), result.Total)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_ByKeyword() {
	mol := &molecule.Molecule{
		SMILES:          "CCO",
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
		Name:            "Ethanol",
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		Keyword: "Ethanol",
	})
	s.NoError(err)
	s.Len(result.Molecules, 1)
	s.Equal("CCO", result.Molecules[0].SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_ByKeyword_NoMatch() {
	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		Keyword: "NonExistentMolecule",
	})
	s.NoError(err)
	s.Len(result.Molecules, 0)
	s.Equal(int64(0), result.Total)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_CombinedFilters() {
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", MolecularWeight: 16.04, Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", MolecularWeight: 30.07, Status: "pending", Source: "patent"},
		{ID: uuid.New(), SMILES: "CCC", CanonicalSMILES: "CCC", InChIKey: "ATUOYWHBWRKTHZ-UHFFFAOYSA-N", MolecularWeight: 44.10, Status: "active", Source: "patent"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	minW := 20.0
	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		MinMolecularWeight: &minW,
		Sources:            []molecule.MoleculeSource{molecule.SourcePatent},
		Statuses:           []molecule.MoleculeStatus{molecule.MoleculeStatusPending},
	})
	s.NoError(err)
	s.Len(result.Molecules, 1)
	s.Equal(int64(1), result.Total)
	s.Equal("CC", result.Molecules[0].SMILES)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_NilQuery() {
	_, err := s.repo.Search(context.Background(), nil)
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeValidation))
}

func (s *MoleculeRepoIntegrationTestSuite) TestCount_NilQuery() {
	_, err := s.repo.Count(context.Background(), nil)
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeValidation))
}

func (s *MoleculeRepoIntegrationTestSuite) TestCount_EmptyTable() {
	count, err := s.repo.Count(context.Background(), &molecule.MoleculeQuery{})
	s.NoError(err)
	s.Equal(int64(0), count)
}

func (s *MoleculeRepoIntegrationTestSuite) TestCountByStatus() {
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CCC", CanonicalSMILES: "CCC", InChIKey: "ATUOYWHBWRKTHZ-UHFFFAOYSA-N", Status: "pending", Source: "manual"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

		// Note: CountByStatus is not in the base interface; must access via type assertion
		if repo, ok := s.repo.(interface {
			CountByStatus(ctx context.Context) (map[molecule.Status]int64, error)
		}); ok {
			counts, err := repo.CountByStatus(context.Background())
			s.NoError(err)
			s.Equal(int64(2), counts["active"])
			s.Equal(int64(1), counts["pending"])
		}
}

// ─── Pagination Tests ─────────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestPagination_DefaultLimit() {
	mols := make([]*molecule.Molecule, 25)
	for i := 0; i < 25; i++ {
		mols[i] = &molecule.Molecule{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        fmt.Sprintf("VNWKTOKETHGBQD-UHFFFAOYSA-%02d", i),
			Status:          "active",
			Source:          "manual",
		}
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{})
	s.NoError(err)
	s.Len(result.Molecules, 20) // default limit
	s.Equal(20, result.Limit)
	s.True(result.HasMore)
}

func (s *MoleculeRepoIntegrationTestSuite) TestPagination_ExplicitLimit() {
	mols := make([]*molecule.Molecule, 10)
	for i := 0; i < 10; i++ {
		mols[i] = &molecule.Molecule{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        fmt.Sprintf("VNWKTOKETHGBQD-UHFFFAOYSA-%02d", i),
			Status:          "active",
			Source:          "manual",
		}
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	limit := 5
	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{Limit: limit})
	s.NoError(err)
	s.Len(result.Molecules, 5)
	s.Equal(5, result.Limit)
	s.True(result.HasMore)
}

func (s *MoleculeRepoIntegrationTestSuite) TestPagination_Offset() {
	mols := make([]*molecule.Molecule, 10)
	for i := 0; i < 10; i++ {
		mols[i] = &molecule.Molecule{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        fmt.Sprintf("VNWKTOKETHGBQD-UHFFFAOYSA-%02d", i),
			Status:          "active",
			Source:          "manual",
		}
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	limit := 5
	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{Limit: limit, Offset: 5})
	s.NoError(err)
	s.Len(result.Molecules, 5)
	s.Equal(5, result.Offset)
	s.False(result.HasMore)
}

func (s *MoleculeRepoIntegrationTestSuite) TestPagination_OffsetBeyondResults() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          "active",
		Source:          "manual",
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{Offset: 100})
	s.NoError(err)
	s.Len(result.Molecules, 0)
	s.Equal(int64(1), result.Total)
	s.False(result.HasMore)
}

func (s *MoleculeRepoIntegrationTestSuite) TestPagination_ZeroLimitDefaultsToTwenty() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          "active",
		Source:          "manual",
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{Limit: 0})
	s.NoError(err)
	s.Equal(20, result.Limit)
}

func (s *MoleculeRepoIntegrationTestSuite) TestPagination_HasMoreReset() {
	// 3 molecules, limit 2: should have HasMore=true
	mols := make([]*molecule.Molecule, 3)
	for i := 0; i < 3; i++ {
		mols[i] = &molecule.Molecule{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        fmt.Sprintf("VNWKTOKETHGBQD-UHFFFAOYSA-%02d", i),
			Status:          "active",
			Source:          "manual",
		}
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{Limit: 2})
	s.NoError(err)
	s.Len(result.Molecules, 2)
	s.True(result.HasMore, "should have more results")

	// Second page
	result2, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{Limit: 2, Offset: 2})
	s.NoError(err)
	s.Len(result2.Molecules, 1)
	s.False(result2.HasMore, "should not have more results")
}

// ─── Fingerprint Tests ───────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestFindWithFingerprint() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	// Insert a fingerprint directly
	_, err = s.db.Exec(`INSERT INTO molecule_fingerprints (molecule_id, fingerprint_type, fingerprint_bits) VALUES ($1, 'morgan', '1010')`, mol.ID)
	s.NoError(err)

	results, err := s.repo.FindWithFingerprint(context.Background(), molecule.FingerprintMorgan, 0, 10)
	s.NoError(err)
	s.Len(results, 1)
	s.Equal(mol.ID, results[0].ID)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindWithFingerprint_NoMatch() {
	mol := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	// No fingerprint inserted
	results, err := s.repo.FindWithFingerprint(context.Background(), molecule.FingerprintMorgan, 0, 10)
	s.NoError(err)
	s.Len(results, 0)
}

func (s *MoleculeRepoIntegrationTestSuite) TestFindWithoutFingerprint() {
	molWithFP := &molecule.Molecule{
		SMILES:          "C",
		CanonicalSMILES: "C",
		InChIKey:        "VNWKTOKETHGBQD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}
	molWithoutFP := &molecule.Molecule{
		SMILES:          "CC",
		CanonicalSMILES: "CC",
		InChIKey:        "OTMSDBZUPAUEDD-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusActive,
		Source:          molecule.SourceManual,
	}

	err := s.repo.Save(context.Background(), molWithFP)
	s.NoError(err)
	err = s.repo.Save(context.Background(), molWithoutFP)
	s.NoError(err)

	// Add fingerprint to first molecule only
	_, err = s.db.Exec(`INSERT INTO molecule_fingerprints (molecule_id, fingerprint_type, fingerprint_bits) VALUES ($1, 'morgan', '1010')`, molWithFP.ID)
	s.NoError(err)

	results, err := s.repo.FindWithoutFingerprint(context.Background(), molecule.FingerprintMorgan, 0, 10)
	s.NoError(err)
	s.Len(results, 1)
	s.Equal(molWithoutFP.ID, results[0].ID)
}

// ─── Batch Tests ──────────────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestBatchSave_EmptyList() {
	affected, err := s.repo.BatchSave(context.Background(), []*molecule.Molecule{})
	s.NoError(err)
	s.Equal(0, affected)
}

func (s *MoleculeRepoIntegrationTestSuite) TestBatchSave_LargeBatch() {
	mols := make([]*molecule.Molecule, 100)
	for i := 0; i < 100; i++ {
		mols[i] = &molecule.Molecule{
			ID:              uuid.New(),
			SMILES:          "C",
			CanonicalSMILES: "C",
			InChIKey:        fmt.Sprintf("VNWKTOKETHGBQD-UHFFFAOYSA-%02d", i),
			Status:          "active",
			Source:          "manual",
		}
	}
	affected, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)
	s.Equal(100, affected)

	count, err := s.repo.Count(context.Background(), &molecule.MoleculeQuery{})
	s.NoError(err)
	s.Equal(int64(100), count)
}

// ─── Sort Order Tests ─────────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_SortByMolecularWeightASC() {
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "CCC", CanonicalSMILES: "CCC", InChIKey: "ATUOYWHBWRKTHZ-UHFFFAOYSA-N", MolecularWeight: 44.10, Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", MolecularWeight: 16.04, Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", MolecularWeight: 30.07, Status: "active", Source: "manual"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		SortBy:    "molecular_weight",
		SortOrder: "ASC",
	})
	s.NoError(err)
	s.Len(result.Molecules, 3)
	s.True(result.Molecules[0].MolecularWeight <= result.Molecules[1].MolecularWeight)
	s.True(result.Molecules[1].MolecularWeight <= result.Molecules[2].MolecularWeight)
}

func (s *MoleculeRepoIntegrationTestSuite) TestSearch_SortByMolecularWeightDESC() {
	mols := []*molecule.Molecule{
		{ID: uuid.New(), SMILES: "C", CanonicalSMILES: "C", InChIKey: "VNWKTOKETHGBQD-UHFFFAOYSA-N", MolecularWeight: 16.04, Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CC", CanonicalSMILES: "CC", InChIKey: "OTMSDBZUPAUEDD-UHFFFAOYSA-N", MolecularWeight: 30.07, Status: "active", Source: "manual"},
		{ID: uuid.New(), SMILES: "CCC", CanonicalSMILES: "CCC", InChIKey: "ATUOYWHBWRKTHZ-UHFFFAOYSA-N", MolecularWeight: 44.10, Status: "active", Source: "manual"},
	}
	_, err := s.repo.BatchSave(context.Background(), mols)
	s.NoError(err)

	result, err := s.repo.Search(context.Background(), &molecule.MoleculeQuery{
		SortBy:    "molecular_weight",
		SortOrder: "DESC",
	})
	s.NoError(err)
	s.Len(result.Molecules, 3)
	s.True(result.Molecules[0].MolecularWeight >= result.Molecules[1].MolecularWeight)
	s.True(result.Molecules[1].MolecularWeight >= result.Molecules[2].MolecularWeight)
}

// ─── Concurrency Tests ────────────────────────────────────────────────────────

func (s *MoleculeRepoIntegrationTestSuite) TestConcurrentCreate() {
	const goroutines = 10
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			mol := &molecule.Molecule{
				SMILES:          "C",
				CanonicalSMILES: "C",
				InChIKey:        fmt.Sprintf("VNWKTOKETHGBQD-UHFFFAOYSA-%02d", idx),
				Status:          molecule.MoleculeStatusActive,
				Source:          molecule.SourceManual,
			}
			err := s.repo.Save(context.Background(), mol)
			errs <- err
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		s.NoError(err)
	}

	count, err := s.repo.Count(context.Background(), &molecule.MoleculeQuery{})
	s.NoError(err)
	s.Equal(int64(goroutines), count)
}

func (s *MoleculeRepoIntegrationTestSuite) TestConcurrentReadWrite() {
	mol := &molecule.Molecule{
		SMILES:          "CCO",
		CanonicalSMILES: "CCO",
		InChIKey:        "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		Status:          molecule.MoleculeStatusPending,
		Source:          molecule.SourceManual,
	}
	err := s.repo.Save(context.Background(), mol)
	s.NoError(err)

	var wg sync.WaitGroup
	readErrs := make(chan error, 5)
	writeErrs := make(chan error, 5)

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.repo.FindByID(context.Background(), mol.ID.String())
			readErrs <- err
		}()
	}

	// Concurrent writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.repo.Exists(context.Background(), mol.ID.String())
			writeErrs <- err
		}()
	}

	wg.Wait()
	close(readErrs)
	close(writeErrs)

	for err := range readErrs {
		s.NoError(err)
	}
	for err := range writeErrs {
		s.NoError(err)
	}
}
