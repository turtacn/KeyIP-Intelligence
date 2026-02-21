package molecule

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStatus_String(t *testing.T) {
	assert.Equal(t, "pending_review", string(StatusPending))
	assert.Equal(t, "active", string(StatusActive))
	assert.Equal(t, "archived", string(StatusArchived))
	assert.Equal(t, "deleted", string(StatusDeleted))
}

func TestMolecule_New(t *testing.T) {
	mol, err := NewMolecule("C", SourceManual, "ref")
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, mol.ID)
	assert.Equal(t, "C", mol.SMILES)
	assert.Equal(t, StatusPending, mol.Status)
}

func TestMolecule_Activate(t *testing.T) {
	mol := &Molecule{Status: StatusPending}
	err := mol.Activate()
	assert.NoError(t, err)
	assert.Equal(t, StatusActive, mol.Status)
}

func TestMolecule_MarkDeleted(t *testing.T) {
	mol := &Molecule{Status: StatusActive}
	err := mol.MarkDeleted()
	assert.NoError(t, err)
	assert.Equal(t, StatusDeleted, mol.Status)
	assert.NotNil(t, mol.DeletedAt)
}

func TestMolecule_AddFingerprint(t *testing.T) {
	mol := &Molecule{ID: uuid.New()}
	fp := &Fingerprint{
		Type: "morgan",
		Hash: "abc",
	}
	err := mol.AddFingerprint(fp)
	assert.NoError(t, err)
	assert.True(t, mol.HasFingerprint("morgan"))
}

func TestFingerprintType_IsValid(t *testing.T) {
	assert.True(t, FingerprintTypeMorgan.IsValid())
	assert.False(t, FingerprintType("invalid").IsValid())
}

func TestMolecule_Fields(t *testing.T) {
	id := uuid.New()
	now := time.Now()
	mol := &Molecule{
		ID:              id,
		Status:          StatusActive,
		Source:          "manual",
		SourceReference: "ref1", // Was SourceRef in failed test
		CreatedAt:       now,
	}

	assert.Equal(t, id, mol.ID) // Direct access
	assert.Equal(t, StatusActive, mol.Status) // Direct access
	assert.Equal(t, "manual", mol.Source)
	assert.Equal(t, "ref1", mol.SourceReference)
}

//Personal.AI order the ending
