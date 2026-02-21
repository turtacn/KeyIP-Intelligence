package molecule

import (
	"strings"
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
	assert.False(t, MoleculeStatus(-1).IsValid())
	assert.False(t, MoleculeStatus(4).IsValid())
}

func TestNewMolecule(t *testing.T) {
	tests := []struct {
		name      string
		smiles    string
		source    MoleculeSource
		sourceRef string
		wantErr   bool
	}{
		{"valid_benzene", "c1ccccc1", SourcePatent, "US1234567", false},
		{"empty_smiles", "", SourcePatent, "US1234567", true},
		{"too_long_smiles", strings.Repeat("C", 10001), SourcePatent, "US1234567", true},
		{"invalid_chars", "C1=CC=CC=C1!", SourcePatent, "US1234567", true},
		{"unbalanced_brackets", "C(C(C)", SourcePatent, "US1234567", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mol, err := NewMolecule(tt.smiles, tt.source, tt.sourceRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMolecule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.NotNil(t, mol)
				assert.NotEmpty(t, mol.ID())
				assert.Equal(t, MoleculeStatusPending, mol.Status())
				assert.Equal(t, tt.source, mol.Source())
				assert.Equal(t, tt.sourceRef, mol.SourceRef())
			}
		})
	}
}

func TestMolecule_Validate(t *testing.T) {
	mol, _ := NewMolecule("C", SourceManual, "")

	t.Run("valid_minimal", func(t *testing.T) {
		assert.NoError(t, mol.Validate())
	})

	t.Run("invalid_weight", func(t *testing.T) {
		mol.molecularWeight = -1.0
		assert.Error(t, mol.Validate())
		mol.molecularWeight = 0 // reset
	})

	t.Run("invalid_inchiKey", func(t *testing.T) {
		mol.inchiKey = "INVALID"
		assert.Error(t, mol.Validate())
		mol.inchiKey = "" // reset
	})

	t.Run("invalid_property_confidence", func(t *testing.T) {
		err := mol.AddProperty(&MolecularProperty{Name: "p", Value: 1.0, Confidence: 1.5})
		assert.Error(t, err)
	})
}

func TestMolecule_StatusTransitions(t *testing.T) {
	mol, _ := NewMolecule("C", SourceManual, "")

	t.Run("activate_fail_no_inchiKey", func(t *testing.T) {
		err := mol.Activate()
		assert.Error(t, err)
	})

	t.Run("activate_success", func(t *testing.T) {
		_ = mol.SetStructureIdentifiers("C", "InChI=1S/CH4/h1H4", "VNWKTOKETHGBQD-UHFFFAOYSA-N", "CH4", 16.04)
		err := mol.Activate()
		assert.NoError(t, err)
		assert.True(t, mol.IsActive())
		assert.Equal(t, int64(3), mol.Version()) // 1(new) -> 2(set identifiers) -> 3(activate)
	})

	t.Run("archive_success", func(t *testing.T) {
		err := mol.Archive()
		assert.NoError(t, err)
		assert.True(t, mol.IsArchived())
	})

	t.Run("delete_success", func(t *testing.T) {
		err := mol.MarkDeleted()
		assert.NoError(t, err)
		assert.True(t, mol.IsDeleted())
	})

	t.Run("delete_fail_from_deleted", func(t *testing.T) {
		err := mol.MarkDeleted()
		assert.Error(t, err)
	})
}

func TestMolecule_FingerprintManagement(t *testing.T) {
	mol, _ := NewMolecule("C", SourceManual, "")
	fp, _ := NewBitFingerprint(FingerprintMorgan, []byte{0x01}, 8, 2)

	err := mol.AddFingerprint(fp)
	assert.NoError(t, err)
	assert.True(t, mol.HasFingerprint(FingerprintMorgan))

	got, ok := mol.GetFingerprint(FingerprintMorgan)
	assert.True(t, ok)
	assert.Equal(t, fp, got)

	_, ok = mol.GetFingerprint(FingerprintMACCS)
	assert.False(t, ok)
}

func TestMolecule_TagManagement(t *testing.T) {
	mol, _ := NewMolecule("C", SourceManual, "")

	_ = mol.AddTag("test")
	_ = mol.AddTag("test") // duplicate
	assert.Len(t, mol.Tags(), 1)

	mol.RemoveTag("test")
	assert.Len(t, mol.Tags(), 0)
}

func TestMolecule_GettersReturnCopies(t *testing.T) {
	mol, _ := NewMolecule("C", SourceManual, "")
	_ = mol.AddTag("tag1")

	tags := mol.Tags()
	tags[0] = "modified"
	assert.Equal(t, "tag1", mol.Tags()[0])

	mol.SetMetadata("k1", "v1")
	meta := mol.Metadata()
	meta["k1"] = "modified"
	assert.Equal(t, "v1", mol.Metadata()["k1"])
}

//Personal.AI order the ending
