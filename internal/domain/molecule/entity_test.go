package molecule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoleculeStatus_String(t *testing.T) {
	assert.Equal(t, "pending", MoleculeStatusPending.String())
	assert.Equal(t, "active", MoleculeStatusActive.String())
	assert.Equal(t, "archived", MoleculeStatusArchived.String())
	assert.Equal(t, "deleted", MoleculeStatusDeleted.String())
}

func TestMoleculeStatus_IsValid(t *testing.T) {
	assert.True(t, MoleculeStatusPending.IsValid())
	assert.True(t, MoleculeStatusActive.IsValid())
	assert.True(t, MoleculeStatusArchived.IsValid())
	assert.True(t, MoleculeStatusDeleted.IsValid())
	assert.False(t, MoleculeStatus(99).IsValid())
}

func TestMoleculeSource_IsValid(t *testing.T) {
	assert.True(t, SourcePatent.IsValid())
	assert.True(t, SourceLiterature.IsValid())
	assert.True(t, SourceExperiment.IsValid())
	assert.True(t, SourcePrediction.IsValid())
	assert.True(t, SourceManual.IsValid())
	assert.False(t, MoleculeSource("invalid").IsValid())
}

func TestNewMolecule(t *testing.T) {
	tests := []struct {
		name      string
		smiles    string
		source    MoleculeSource
		sourceRef string
		wantErr   bool
	}{
		{
			name:      "valid_simple_smiles",
			smiles:    "c1ccccc1",
			source:    SourceManual,
			sourceRef: "ref1",
			wantErr:   false,
		},
		{
			name:      "valid_complex_smiles",
			smiles:    "CN1C=NC2=C1C(=O)N(C(=O)N2C)C",
			source:    SourcePatent,
			sourceRef: "US1234567",
			wantErr:   false,
		},
		{
			name:      "empty_smiles",
			smiles:    "",
			source:    SourceManual,
			sourceRef: "ref1",
			wantErr:   true,
		},
		{
			name:      "unbalanced_parentheses",
			smiles:    "C(C(C",
			source:    SourceManual,
			sourceRef: "ref1",
			wantErr:   true,
		},
		{
			name:      "illegal_characters",
			smiles:    "C!@#$",
			source:    SourceManual,
			sourceRef: "ref1",
			wantErr:   true,
		},
		{
			name:      "invalid_source",
			smiles:    "c1ccccc1",
			source:    "invalid",
			sourceRef: "ref1",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mol, err := NewMolecule(tt.smiles, tt.source, tt.sourceRef)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, mol)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, mol)
				assert.NotEmpty(t, mol.ID())
				assert.Equal(t, MoleculeStatusPending, mol.Status())
				assert.Equal(t, tt.source, mol.Source())
				assert.Equal(t, tt.sourceRef, mol.SourceRef())
				assert.Equal(t, int64(1), mol.Version())
			}
		})
	}
}

func TestMolecule_Validate(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")

	t.Run("valid_complete", func(t *testing.T) {
		assert.NoError(t, mol.Validate())
	})

	t.Run("missing_id", func(t *testing.T) {
		oldID := mol.id
		mol.id = ""
		assert.Error(t, mol.Validate())
		mol.id = oldID
	})

	t.Run("missing_smiles", func(t *testing.T) {
		oldSmiles := mol.smiles
		mol.smiles = ""
		assert.Error(t, mol.Validate())
		mol.smiles = oldSmiles
	})

	t.Run("negative_weight", func(t *testing.T) {
		oldWeight := mol.molecularWeight
		mol.molecularWeight = -1.0
		assert.Error(t, mol.Validate())
		mol.molecularWeight = oldWeight
	})

	t.Run("invalid_inchikey_format", func(t *testing.T) {
		oldKey := mol.inchiKey
		mol.inchiKey = "INVALID"
		assert.Error(t, mol.Validate())
		mol.inchiKey = oldKey
	})

	t.Run("valid_inchikey", func(t *testing.T) {
		mol.inchiKey = "AAAAAAAAAAAAAA-BBBBBBBBBB-C"
		assert.NoError(t, mol.Validate())
	})
}

func TestMolecule_StateTransitions(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")

	t.Run("Activate_NoInChIKey", func(t *testing.T) {
		err := mol.Activate()
		assert.Error(t, err)
	})

	mol.inchiKey = "AAAAAAAAAAAAAA-BBBBBBBBBB-C" // Set via internal access for setup

	t.Run("Activate_Success", func(t *testing.T) {
		err := mol.Activate()
		assert.NoError(t, err)
		assert.Equal(t, MoleculeStatusActive, mol.Status())
	})

	t.Run("Activate_WrongStatus", func(t *testing.T) {
		err := mol.Activate() // Already active
		assert.Error(t, err)
	})

	t.Run("Archive_Success", func(t *testing.T) {
		err := mol.Archive()
		assert.NoError(t, err)
		assert.Equal(t, MoleculeStatusArchived, mol.Status())
	})

	t.Run("MarkDeleted_FromArchived", func(t *testing.T) {
		err := mol.MarkDeleted()
		assert.NoError(t, err)
		assert.Equal(t, MoleculeStatusDeleted, mol.Status())
	})

	t.Run("MarkDeleted_FromDeleted", func(t *testing.T) {
		err := mol.MarkDeleted()
		assert.Error(t, err)
	})
}

func TestMolecule_SetStructureIdentifiers(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")

	err := mol.SetStructureIdentifiers("c1ccccc1", "InChI=...", "AAAAAAAAAAAAAA-BBBBBBBBBB-C", "C6H6", 78.11)
	assert.NoError(t, err)
	assert.Equal(t, "c1ccccc1", mol.CanonicalSmiles())
	assert.Equal(t, "InChI=...", mol.InChI())
	assert.Equal(t, "AAAAAAAAAAAAAA-BBBBBBBBBB-C", mol.InChIKey())
	assert.Equal(t, "C6H6", mol.MolecularFormula())
	assert.Equal(t, 78.11, mol.MolecularWeight())
}

func TestMolecule_FingerprintManagement(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")
	fp, _ := NewBitFingerprint(FingerprintMorgan, []byte{1}, 8, 2)

	t.Run("AddFingerprint_Success", func(t *testing.T) {
		err := mol.AddFingerprint(fp)
		assert.NoError(t, err)
		assert.True(t, mol.HasFingerprint(FingerprintMorgan))
	})

	t.Run("AddFingerprint_Nil", func(t *testing.T) {
		err := mol.AddFingerprint(nil)
		assert.Error(t, err)
	})

	t.Run("GetFingerprint_Exists", func(t *testing.T) {
		got, ok := mol.GetFingerprint(FingerprintMorgan)
		assert.True(t, ok)
		assert.Equal(t, fp, got)
	})

	t.Run("GetFingerprint_NotExists", func(t *testing.T) {
		_, ok := mol.GetFingerprint(FingerprintMACCS)
		assert.False(t, ok)
	})
}

func TestMolecule_PropertyManagement(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")
	prop := &MolecularProperty{Name: "logP", Value: 2.1, Confidence: 0.9}

	t.Run("AddProperty_Success", func(t *testing.T) {
		err := mol.AddProperty(prop)
		assert.NoError(t, err)
		got, ok := mol.GetProperty("logP")
		assert.True(t, ok)
		assert.Equal(t, prop, got)
	})

	t.Run("AddProperty_Invalid", func(t *testing.T) {
		err := mol.AddProperty(&MolecularProperty{Name: "bad", Confidence: 1.5})
		assert.Error(t, err)
	})
}

func TestMolecule_TagManagement(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")

	mol.AddTag("tag1")
	assert.Contains(t, mol.Tags(), "tag1")

	mol.AddTag("tag1") // Duplicate
	assert.Len(t, mol.Tags(), 1)

	mol.RemoveTag("tag1")
	assert.Empty(t, mol.Tags())
}

func TestMolecule_Helpers(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")
	assert.True(t, mol.IsPending())
	assert.False(t, mol.IsActive())

	mol.SetMetadata("key", "value")
	meta := mol.Metadata()
	assert.Equal(t, "value", meta["key"])

	// Test copy behavior
	meta["key"] = "changed"
	assert.Equal(t, "value", mol.Metadata()["key"])
}

func TestMolecule_String(t *testing.T) {
	mol, _ := NewMolecule("c1ccccc1", SourceManual, "ref")
	str := mol.String()
	assert.Contains(t, str, "pending")
	assert.Contains(t, str, "c1ccccc1")
}

//Personal.AI order the ending
