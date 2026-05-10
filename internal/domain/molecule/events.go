package molecule

import (
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Molecule domain event type constants.
const (
	EventMoleculeRegistered common.EventType = "molecule.registered"
	EventMoleculeIndexed    common.EventType = "molecule.indexed"
	EventMoleculeArchived   common.EventType = "molecule.archived"
	EventMoleculeDeleted    common.EventType = "molecule.deleted"
)

// MoleculeRegisteredEvent is published when a new molecule is registered.
type MoleculeRegisteredEvent struct {
	common.BaseEvent
	SMILES    string         `json:"smiles"`
	InChIKey  string         `json:"inchi_key"`
	Source    MoleculeSource `json:"source"`
	SourceRef string         `json:"source_ref,omitempty"`
}

// NewMoleculeRegisteredEvent creates a MoleculeRegisteredEvent from a Molecule.
func NewMoleculeRegisteredEvent(m *Molecule) *MoleculeRegisteredEvent {
	return &MoleculeRegisteredEvent{
		BaseEvent: common.NewBaseEventWithVersion(
			EventMoleculeRegistered,
			m.ID.String(),
			int(m.Version),
		),
		SMILES:    m.SMILES,
		InChIKey:  m.InChIKey,
		Source:    m.Source,
		SourceRef: m.SourceReference,
	}
}

// MoleculeIndexedEvent is published when fingerprint/indexing operations complete.
type MoleculeIndexedEvent struct {
	common.BaseEvent
	MoleculeID       string   `json:"molecule_id"`
	InChIKey       string   `json:"inchi_key"`
	FingerprintTypes []string `json:"fingerprint_types,omitempty"`
}

// NewMoleculeIndexedEvent creates a MoleculeIndexedEvent.
func NewMoleculeIndexedEvent(m *Molecule) *MoleculeIndexedEvent {
	var fpTypes []string
	for fpType := range m.FingerprintsMap {
		fpTypes = append(fpTypes, string(fpType))
	}

	return &MoleculeIndexedEvent{
		BaseEvent: common.NewBaseEventWithVersion(
			EventMoleculeIndexed,
			m.ID.String(),
			int(m.Version),
		),
		MoleculeID:       m.ID.String(),
		InChIKey:         m.InChIKey,
		FingerprintTypes: fpTypes,
	}
}

// MoleculeArchivedEvent is published when a molecule is archived.
type MoleculeArchivedEvent struct {
	common.BaseEvent
	MoleculeID string `json:"molecule_id"`
	InChIKey   string `json:"inchi_key"`
}

// NewMoleculeArchivedEvent creates a MoleculeArchivedEvent.
func NewMoleculeArchivedEvent(m *Molecule) *MoleculeArchivedEvent {
	return &MoleculeArchivedEvent{
		BaseEvent: common.NewBaseEventWithVersion(
			EventMoleculeArchived,
			m.ID.String(),
			int(m.Version),
		),
		MoleculeID: m.ID.String(),
		InChIKey:   m.InChIKey,
	}
}

// MoleculeDeletedEvent is published when a molecule is deleted.
type MoleculeDeletedEvent struct {
	common.BaseEvent
	MoleculeID string `json:"molecule_id"`
	InChIKey   string `json:"inchi_key"`
}

// NewMoleculeDeletedEvent creates a MoleculeDeletedEvent.
func NewMoleculeDeletedEvent(m *Molecule) *MoleculeDeletedEvent {
	return &MoleculeDeletedEvent{
		BaseEvent: common.NewBaseEventWithVersion(
			EventMoleculeDeleted,
			m.ID.String(),
			int(m.Version),
		),
		MoleculeID: m.ID.String(),
		InChIKey:   m.InChIKey,
	}
}
