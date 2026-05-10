package molecule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func newTestMolForEvents() *Molecule {
	mol, err := NewMolecule("c1ccccc1C", SourceManual, "test-ref")
	if err != nil {
		panic(err)
	}
	return mol
}

func TestMoleculeEventType_Constants(t *testing.T) {
	assert.Equal(t, common.EventType("molecule.registered"), EventMoleculeRegistered)
	assert.Equal(t, common.EventType("molecule.indexed"), EventMoleculeIndexed)
	assert.Equal(t, common.EventType("molecule.archived"), EventMoleculeArchived)
	assert.Equal(t, common.EventType("molecule.deleted"), EventMoleculeDeleted)
}

func TestNewMoleculeRegisteredEvent(t *testing.T) {
	m := newTestMolForEvents()
	event := NewMoleculeRegisteredEvent(m)

	assert.Equal(t, EventMoleculeRegistered, event.EventType())
	assert.Equal(t, m.ID.String(), event.AggregateID())
	assert.Equal(t, int(m.Version), event.Version())
	assert.Equal(t, m.SMILES, event.SMILES)
	assert.Equal(t, m.InChIKey, event.InChIKey)
	assert.Equal(t, m.Source, event.Source)
	assert.Equal(t, m.SourceReference, event.SourceRef)
	assert.False(t, event.OccurredAt().IsZero())
}

func TestNewMoleculeIndexedEvent(t *testing.T) {
	m := newTestMolForEvents()
	_ = m.AddFingerprint(&Fingerprint{Type: FingerprintMorgan})
	_ = m.AddFingerprint(&Fingerprint{Type: FingerprintMACCS})

	event := NewMoleculeIndexedEvent(m)

	assert.Equal(t, EventMoleculeIndexed, event.EventType())
	assert.Equal(t, m.ID.String(), event.MoleculeID)
	assert.Equal(t, m.InChIKey, event.InChIKey)
	assert.Contains(t, event.FingerprintTypes, string(FingerprintMorgan))
	assert.Contains(t, event.FingerprintTypes, string(FingerprintMACCS))
}

func TestNewMoleculeIndexedEvent_NoFingerprints(t *testing.T) {
	m := newTestMolForEvents()
	event := NewMoleculeIndexedEvent(m)

	assert.Equal(t, EventMoleculeIndexed, event.EventType())
	assert.Empty(t, event.FingerprintTypes)
}

func TestNewMoleculeArchivedEvent(t *testing.T) {
	m := newTestMolForEvents()
	now := time.Now().UTC()
	m.CreatedAt = now
	m.UpdatedAt = now
	_ = m.Activate()
	_ = m.Archive()

	event := NewMoleculeArchivedEvent(m)

	assert.Equal(t, EventMoleculeArchived, event.EventType())
	assert.Equal(t, m.ID.String(), event.MoleculeID)
	assert.Equal(t, m.InChIKey, event.InChIKey)
}

func TestNewMoleculeDeletedEvent(t *testing.T) {
	m := newTestMolForEvents()
	_ = m.Activate()
	_ = m.MarkDeleted()

	event := NewMoleculeDeletedEvent(m)

	assert.Equal(t, EventMoleculeDeleted, event.EventType())
	assert.Equal(t, m.ID.String(), event.MoleculeID)
	assert.Equal(t, m.InChIKey, event.InChIKey)
}
