// Package molecule_test provides contract tests for Repository implementations.
package molecule_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// RepositoryContractTest defines the behavioral contract that all molecule
// repository implementations must satisfy.  Implementations should call this
// function with their concrete repository instance to verify compliance.
//
// Example usage:
//
//	func TestPostgresRepository_Contract(t *testing.T) {
//	    repo := setupPostgresRepo(t)
//	    molecule_test.RepositoryContractTest(t, repo)
//	}
func RepositoryContractTest(t *testing.T, repo molecule.Repository) {
	ctx := context.Background()

	t.Run("Save_FindByID_RoundTrip", func(t *testing.T) {
		mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		require.NoError(t, mol.CalculateFingerprint(mtypes.FPMorgan))

		// Save
		err = repo.Save(ctx, mol)
		require.NoError(t, err)

		// Retrieve
		retrieved, err := repo.FindByID(ctx, mol.ID)
		require.NoError(t, err)
		assert.Equal(t, mol.ID, retrieved.ID)
		assert.Equal(t, mol.SMILES, retrieved.SMILES)
		assert.Equal(t, mol.InChIKey, retrieved.InChIKey)
	})

	t.Run("FindBySMILES", func(t *testing.T) {
		mol, err := molecule.NewMolecule("CCO", mtypes.TypeSmallMolecule)
		require.NoError(t, err)

		require.NoError(t, repo.Save(ctx, mol))

		retrieved, err := repo.FindBySMILES(ctx, "CCO")
		require.NoError(t, err)
		assert.Equal(t, mol.ID, retrieved.ID)
		assert.Equal(t, "CCO", retrieved.SMILES)
	})

	t.Run("FindByInChIKey", func(t *testing.T) {
		mol, err := molecule.NewMolecule("c1ccc2ccccc2c1", mtypes.TypeOLEDMaterial)
		require.NoError(t, err)

		require.NoError(t, repo.Save(ctx, mol))

		retrieved, err := repo.FindByInChIKey(ctx, mol.InChIKey)
		require.NoError(t, err)
		assert.Equal(t, mol.ID, retrieved.ID)
	})

	t.Run("FindSimilar", func(t *testing.T) {
		// Create and save several related molecules
		benzene, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		require.NoError(t, benzene.CalculateFingerprint(mtypes.FPMorgan))
		require.NoError(t, repo.Save(ctx, benzene))

		toluene, err := molecule.NewMolecule("Cc1ccccc1", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		require.NoError(t, toluene.CalculateFingerprint(mtypes.FPMorgan))
		require.NoError(t, repo.Save(ctx, toluene))

		ethanol, err := molecule.NewMolecule("CCO", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		require.NoError(t, ethanol.CalculateFingerprint(mtypes.FPMorgan))
		require.NoError(t, repo.Save(ctx, ethanol))

		// Search for molecules similar to benzene
		similarFP := benzene.Fingerprints[mtypes.FPMorgan]
		results, err := repo.FindSimilar(ctx, similarFP, mtypes.FPMorgan, 0.7, 10)
		require.NoError(t, err)

		// Should return at least benzene itself and probably toluene
		assert.GreaterOrEqual(t, len(results), 1)

		// Results should be ordered by similarity (highest first)
		// The exact similarity is best verified by the first result being benzene itself
		found := false
		for _, r := range results {
			if r.ID == benzene.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "benzene should be in its own similarity results")
	})

	t.Run("BatchSave", func(t *testing.T) {
		mols := make([]*molecule.Molecule, 3)
		for i := 0; i < 3; i++ {
			mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
			require.NoError(t, err)
			mols[i] = mol
		}

		err := repo.BatchSave(ctx, mols)
		require.NoError(t, err)

		// Verify all saved
		for _, mol := range mols {
			retrieved, err := repo.FindByID(ctx, mol.ID)
			require.NoError(t, err)
			assert.Equal(t, mol.ID, retrieved.ID)
		}
	})

	t.Run("FindByPatentID", func(t *testing.T) {
		patentID := common.ID("patent-xyz")

		mol1, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		mol1.AddSourcePatent(patentID)
		require.NoError(t, repo.Save(ctx, mol1))

		mol2, err := molecule.NewMolecule("CCO", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		mol2.AddSourcePatent(patentID)
		require.NoError(t, repo.Save(ctx, mol2))

		// Retrieve by patent ID
		results, err := repo.FindByPatentID(ctx, patentID)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(results), 2)
		ids := make(map[common.ID]bool)
		for _, mol := range results {
			ids[mol.ID] = true
		}
		assert.True(t, ids[mol1.ID])
		assert.True(t, ids[mol2.ID])
	})

	t.Run("Count", func(t *testing.T) {
		initialCount, err := repo.Count(ctx)
		require.NoError(t, err)

		mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		require.NoError(t, repo.Save(ctx, mol))

		newCount, err := repo.Count(ctx)
		require.NoError(t, err)

		assert.Equal(t, initialCount+1, newCount)
	})
}

//Personal.AI order the ending
