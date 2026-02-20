// Package molecule_test provides comprehensive unit tests for the Molecule
// domain entity and its associated behaviors.
package molecule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestNewMolecule
// ─────────────────────────────────────────────────────────────────────────────

func TestNewMolecule_ValidSMILES(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		smiles string
		typ    mtypes.MoleculeType
	}{
		{"benzene", "c1ccccc1", mtypes.TypeSmallMolecule},
		{"indole", "c1ccc2[nH]ccc2c1", mtypes.TypeOLEDMaterial},
		{"ethanol", "CCO", mtypes.TypeSmallMolecule},
		{"naphthalene", "c1ccc2ccccc2c1", mtypes.TypeOLEDMaterial},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mol, err := molecule.NewMolecule(tc.smiles, tc.typ)
			require.NoError(t, err)
			require.NotNil(t, mol)

			assert.Equal(t, tc.smiles, mol.SMILES)
			assert.NotEmpty(t, mol.CanonicalSMILES)
			assert.NotEmpty(t, mol.InChIKey)
			assert.NotEmpty(t, string(mol.ID))
			assert.Equal(t, tc.typ, mol.Type)
			assert.NotNil(t, mol.Fingerprints)
			assert.NotNil(t, mol.SourcePatentIDs)
			assert.Len(t, mol.SourcePatentIDs, 0)
		})
	}
}

func TestNewMolecule_EmptySMILES(t *testing.T) {
	t.Parallel()

	cases := []string{"", "   ", "\t", "\n"}
	for _, smiles := range cases {
		smiles := smiles
		t.Run("", func(t *testing.T) {
			t.Parallel()

			mol, err := molecule.NewMolecule(smiles, mtypes.TypeSmallMolecule)
			require.Error(t, err)
			assert.Nil(t, mol)
			assert.Contains(t, err.Error(), "empty")
		})
	}
}

func TestNewMolecule_InvalidCharacters(t *testing.T) {
	t.Parallel()

	cases := []string{
		"c1ccccc1!",     // exclamation mark not allowed
		"CCO$",          // dollar sign not allowed
		"c1ccccc1<>",    // angle brackets not allowed
		"benzene rings", // spaces and text not allowed
	}

	for _, smiles := range cases {
		smiles := smiles
		t.Run(smiles, func(t *testing.T) {
			t.Parallel()

			mol, err := molecule.NewMolecule(smiles, mtypes.TypeSmallMolecule)
			require.Error(t, err)
			assert.Nil(t, mol)
			assert.Contains(t, err.Error(), "invalid characters")
		})
	}
}

func TestNewMolecule_UnmatchedBrackets(t *testing.T) {
	t.Parallel()

	// Note: "c1ccc[NH]c1" actually has matched brackets,
	// so let's use actually unmatched ones
	unmatchedCases := []string{
		"c1ccccc1(",
		"c1ccccc1)",
		"C(C(C)C",
		"C[NH2",
	}

	for _, smiles := range unmatchedCases {
		smiles := smiles
		t.Run(smiles, func(t *testing.T) {
			t.Parallel()

			mol, err := molecule.NewMolecule(smiles, mtypes.TypeSmallMolecule)
			require.Error(t, err)
			assert.Nil(t, mol)
			assert.Contains(t, err.Error(), "bracket")
		})
	}
}

func TestNewMolecule_GeneratesUniqueIDs(t *testing.T) {
	t.Parallel()

	const n = 100
	ids := make(map[common.ID]bool)

	for i := 0; i < n; i++ {
		mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
		require.NoError(t, err)
		require.NotEmpty(t, mol.ID)

		assert.False(t, ids[mol.ID], "duplicate ID generated")
		ids[mol.ID] = true
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCalculateFingerprint
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateFingerprint_Morgan(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	err = mol.CalculateFingerprint(mtypes.FPMorgan)
	require.NoError(t, err)

	fp, exists := mol.Fingerprints[mtypes.FPMorgan]
	require.True(t, exists, "Morgan fingerprint should be stored")
	require.NotNil(t, fp)
	assert.Greater(t, fp.NumOnBits, 0, "fingerprint should have some bits set")
	assert.Equal(t, 2048, fp.Length)
}

func TestCalculateFingerprint_MACCS(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	err = mol.CalculateFingerprint(mtypes.FPMACCS)
	require.NoError(t, err)

	fp, exists := mol.Fingerprints[mtypes.FPMACCS]
	require.True(t, exists)
	require.NotNil(t, fp)
	assert.Greater(t, fp.NumOnBits, 0)
	assert.Equal(t, 166, fp.Length)
}

func TestCalculateFingerprint_Topological(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("CCO", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	err = mol.CalculateFingerprint(mtypes.FPTopological)
	require.NoError(t, err)

	fp, exists := mol.Fingerprints[mtypes.FPTopological]
	require.True(t, exists)
	require.NotNil(t, fp)
	assert.Greater(t, fp.NumOnBits, 0)
}

func TestCalculateFingerprint_MultipleTypes(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	// Calculate multiple fingerprint types
	require.NoError(t, mol.CalculateFingerprint(mtypes.FPMorgan))
	require.NoError(t, mol.CalculateFingerprint(mtypes.FPMACCS))

	assert.Len(t, mol.Fingerprints, 2)
	assert.NotNil(t, mol.Fingerprints[mtypes.FPMorgan])
	assert.NotNil(t, mol.Fingerprints[mtypes.FPMACCS])
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCalculateProperties
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateProperties(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	err = mol.CalculateProperties()
	require.NoError(t, err)

	assert.Greater(t, mol.MolecularWeight, 0.0, "molecular weight should be positive")
	assert.GreaterOrEqual(t, mol.Properties.AromaticRings, 0)
	assert.GreaterOrEqual(t, mol.Properties.HBondDonors, 0)
	assert.GreaterOrEqual(t, mol.Properties.HBondAcceptors, 0)
	assert.GreaterOrEqual(t, mol.Properties.RotatableBonds, 0)
	assert.NotEmpty(t, mol.MolecularFormula)
}

func TestCalculateProperties_BenzeneHasAromaticRing(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	err = mol.CalculateProperties()
	require.NoError(t, err)

	assert.Greater(t, mol.Properties.AromaticRings, 0,
		"benzene should have at least one aromatic ring")
}

func TestCalculateProperties_EthanolNoAromaticRings(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("CCO", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	err = mol.CalculateProperties()
	require.NoError(t, err)

	assert.Equal(t, 0, mol.Properties.AromaticRings,
		"ethanol should have no aromatic rings")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestSimilarityTo
// ─────────────────────────────────────────────────────────────────────────────

func TestSimilarityTo_IdenticalMolecules(t *testing.T) {
	t.Parallel()

	mol1, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)
	require.NoError(t, mol1.CalculateFingerprint(mtypes.FPMorgan))

	mol2, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)
	require.NoError(t, mol2.CalculateFingerprint(mtypes.FPMorgan))

	sim, err := mol1.SimilarityTo(mol2, mtypes.FPMorgan)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, sim, 0.01, "identical molecules should have similarity ~1.0")
}

func TestSimilarityTo_DifferentMolecules(t *testing.T) {
	t.Parallel()

	mol1, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule) // benzene
	require.NoError(t, err)
	require.NoError(t, mol1.CalculateFingerprint(mtypes.FPMorgan))

	mol2, err := molecule.NewMolecule("CCO", mtypes.TypeSmallMolecule) // ethanol
	require.NoError(t, err)
	require.NoError(t, mol2.CalculateFingerprint(mtypes.FPMorgan))

	sim, err := mol1.SimilarityTo(mol2, mtypes.FPMorgan)
	require.NoError(t, err)
	assert.Less(t, sim, 1.0, "different molecules should have similarity < 1.0")
	assert.GreaterOrEqual(t, sim, 0.0, "similarity should be non-negative")
}

func TestSimilarityTo_NoFingerprintComputed(t *testing.T) {
	t.Parallel()

	mol1, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)
	// Deliberately do NOT calculate fingerprint

	mol2, err := molecule.NewMolecule("CCO", mtypes.TypeSmallMolecule)
	require.NoError(t, err)
	require.NoError(t, mol2.CalculateFingerprint(mtypes.FPMorgan))

	_, err = mol1.SimilarityTo(mol2, mtypes.FPMorgan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fingerprint not computed")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAddSourcePatent
// ─────────────────────────────────────────────────────────────────────────────

func TestAddSourcePatent(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	patentID1 := common.ID("patent-001")
	patentID2 := common.ID("patent-002")

	mol.AddSourcePatent(patentID1)
	assert.Contains(t, mol.SourcePatentIDs, patentID1)
	assert.Len(t, mol.SourcePatentIDs, 1)

	mol.AddSourcePatent(patentID2)
	assert.Contains(t, mol.SourcePatentIDs, patentID2)
	assert.Len(t, mol.SourcePatentIDs, 2)
}

func TestAddSourcePatent_Deduplication(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeSmallMolecule)
	require.NoError(t, err)

	patentID := common.ID("patent-001")

	mol.AddSourcePatent(patentID)
	mol.AddSourcePatent(patentID) // add same ID again
	mol.AddSourcePatent(patentID) // and again

	assert.Len(t, mol.SourcePatentIDs, 1, "duplicate IDs should not be added")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIsOLEDMaterial
// ─────────────────────────────────────────────────────────────────────────────

func TestIsOLEDMaterial_True(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeOLEDMaterial)
	require.NoError(t, err)

	assert.True(t, mol.IsOLEDMaterial())
}

func TestIsOLEDMaterial_False(t *testing.T) {
	t.Parallel()

	cases := []mtypes.MoleculeType{
		mtypes.TypeSmallMolecule,
		mtypes.TypePolymer,
		mtypes.TypeCatalyst,
		mtypes.TypeIntermediate,
	}

	for _, typ := range cases {
		typ := typ
		t.Run(string(typ), func(t *testing.T) {
			t.Parallel()

			mol, err := molecule.NewMolecule("c1ccccc1", typ)
			require.NoError(t, err)

			assert.False(t, mol.IsOLEDMaterial())
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestSetOLEDProperties
// ─────────────────────────────────────────────────────────────────────────────

func TestSetOLEDProperties(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeOLEDMaterial)
	require.NoError(t, err)

	homo := -5.4
	lumo := -2.1
	bandGap := 3.3

	mol.SetOLEDProperties(homo, lumo, bandGap)

	require.NotNil(t, mol.Properties.HOMO)
	require.NotNil(t, mol.Properties.LUMO)
	require.NotNil(t, mol.Properties.BandGap)

	assert.InDelta(t, homo, *mol.Properties.HOMO, 1e-9)
	assert.InDelta(t, lumo, *mol.Properties.LUMO, 1e-9)
	assert.InDelta(t, bandGap, *mol.Properties.BandGap, 1e-9)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestToDTO and MoleculeFromDTO
// ─────────────────────────────────────────────────────────────────────────────

func TestToDTO(t *testing.T) {
	t.Parallel()

	mol, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeOLEDMaterial)
	require.NoError(t, err)

	mol.Name = "benzene"
	mol.Synonyms = []string{"cyclohexatriene", "benzol"}
	mol.AddSourcePatent(common.ID("patent-001"))
	require.NoError(t, mol.CalculateFingerprint(mtypes.FPMorgan))
	require.NoError(t, mol.CalculateProperties())

	homo := -5.4
	mol.SetOLEDProperties(homo, -2.1, 3.3)

	dto := mol.ToDTO()

	assert.Equal(t, mol.ID, dto.ID)
	assert.Equal(t, mol.SMILES, dto.SMILES)
	assert.Equal(t, mol.InChIKey, dto.InChIKey)
	assert.Equal(t, mol.Name, dto.Name)
	assert.Equal(t, mol.Synonyms, dto.Synonyms)
	assert.Equal(t, mol.Type, dto.Type)
	assert.Equal(t, mol.SourcePatentIDs, dto.SourcePatentIDs)
	assert.InDelta(t, mol.MolecularWeight, dto.MolecularWeight, 1e-9)
	assert.NotNil(t, dto.Properties.HOMO)
}

func TestMoleculeFromDTO_RoundTrip(t *testing.T) {
	t.Parallel()

	original, err := molecule.NewMolecule("c1ccccc1", mtypes.TypeOLEDMaterial)
	require.NoError(t, err)

	original.Name = "benzene"
	original.Synonyms = []string{"benzol"}
	original.AddSourcePatent(common.ID("patent-001"))
	require.NoError(t, original.CalculateFingerprint(mtypes.FPMorgan))
	require.NoError(t, original.CalculateProperties())
	original.SetOLEDProperties(-5.4, -2.1, 3.3)

	// Convert to DTO and back
	dto := original.ToDTO()
	reconstructed := molecule.MoleculeFromDTO(dto)

	assert.Equal(t, original.ID, reconstructed.ID)
	assert.Equal(t, original.SMILES, reconstructed.SMILES)
	assert.Equal(t, original.InChIKey, reconstructed.InChIKey)
	assert.Equal(t, original.Name, reconstructed.Name)
	assert.Equal(t, original.Type, reconstructed.Type)
	assert.Equal(t, original.SourcePatentIDs, reconstructed.SourcePatentIDs)
	assert.InDelta(t, original.MolecularWeight, reconstructed.MolecularWeight, 1e-9)

	// Check OLED properties
	require.NotNil(t, reconstructed.Properties.HOMO)
	assert.InDelta(t, *original.Properties.HOMO, *reconstructed.Properties.HOMO, 1e-9)
}

