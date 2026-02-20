// Package molecule provides the core domain model for molecular entities in the
// KeyIP-Intelligence platform.  The Molecule aggregate root encapsulates all
// chemical structure data, computed properties, and fingerprints needed for
// similarity search and patent mining workflows.
package molecule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	mtypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// Domain Events
// ─────────────────────────────────────────────────────────────────────────────

// DomainEvent is a marker interface for all molecule-related domain events.
type DomainEvent interface {
	EventType() string
}

// MoleculeCreatedEvent is published when a new molecule is successfully created.
type MoleculeCreatedEvent struct {
	MoleculeID common.ID
	SMILES     string
	Type       mtypes.MoleculeType
}

func (e MoleculeCreatedEvent) EventType() string { return "molecule.created" }

// FingerprintCalculatedEvent is published when a fingerprint is computed.
type FingerprintCalculatedEvent struct {
	MoleculeID      common.ID
	FingerprintType mtypes.FingerprintType
}

func (e FingerprintCalculatedEvent) EventType() string { return "molecule.fingerprint_calculated" }

// ─────────────────────────────────────────────────────────────────────────────
// Value Objects
// ─────────────────────────────────────────────────────────────────────────────

// MolecularProperties is a value object holding computed physicochemical
// descriptors for a molecule.  OLED-specific properties (HOMO, LUMO, BandGap)
// are populated only for TypeOLEDMaterial molecules.
type MolecularProperties struct {
	// Basic physicochemical properties
	LogP           float64 `json:"log_p"`
	TPSA           float64 `json:"tpsa"`
	HBondDonors    int     `json:"h_bond_donors"`
	HBondAcceptors int     `json:"h_bond_acceptors"`
	RotatableBonds int     `json:"rotatable_bonds"`
	AromaticRings  int     `json:"aromatic_rings"`

	// OLED-specific optoelectronic properties (nullable)
	HOMO    *float64 `json:"homo,omitempty"`
	LUMO    *float64 `json:"lumo,omitempty"`
	BandGap *float64 `json:"band_gap,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Molecule Aggregate Root
// ─────────────────────────────────────────────────────────────────────────────

// Molecule is the aggregate root for all chemical structure data in the platform.
// It encapsulates SMILES notation, computed identifiers (InChI, InChIKey),
// molecular properties, fingerprints for similarity search, and references to
// source patents from which the molecule was extracted.
type Molecule struct {
	common.BaseEntity

	// Core structure identifiers
	SMILES          string `json:"smiles"`
	CanonicalSMILES string `json:"canonical_smiles"` // RDKit-canonicalised form
	InChI           string `json:"inchi,omitempty"`
	InChIKey        string `json:"inchi_key"`

	// Basic descriptors
	MolecularFormula string  `json:"molecular_formula"`
	MolecularWeight  float64 `json:"molecular_weight"`

	// Nomenclature
	Name     string   `json:"name,omitempty"`
	Synonyms []string `json:"synonyms,omitempty"`

	// Classification
	Type mtypes.MoleculeType `json:"type"`

	// Computed properties
	Properties MolecularProperties `json:"properties"`

	// Fingerprints for similarity search (keyed by fingerprint type)
	Fingerprints map[mtypes.FingerprintType]*Fingerprint `json:"fingerprints,omitempty"`

	// Source patent references
	SourcePatentIDs []common.ID `json:"source_patent_ids,omitempty"`

	// Domain events (not persisted, cleared after publishing)
	events []DomainEvent
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory Function
// ─────────────────────────────────────────────────────────────────────────────

var (
	// validSMILESChars defines the allowed character set for SMILES notation.
	// This is a simplified check; full SMILES validation requires a parser.
	validSMILESChars = regexp.MustCompile(`^[A-Za-z0-9@+\-\[\]()=#/\\%.*]+$`)
)

// NewMolecule constructs a new Molecule aggregate from a SMILES string.
// It performs basic SMILES validation (character set, bracket matching),
// canonicalises the SMILES (simplified implementation using hash-based sorting),
// generates an InChIKey (simplified: SHA256 of canonical SMILES), and assigns
// a new UUID.
//
// Returns an error if the SMILES string is invalid.
func NewMolecule(smiles string, moleculeType mtypes.MoleculeType) (*Molecule, error) {
	smiles = strings.TrimSpace(smiles)
	if smiles == "" {
		return nil, errors.InvalidParam("SMILES string cannot be empty")
	}

	// Basic character set validation
	if !validSMILESChars.MatchString(smiles) {
		return nil, errors.InvalidParam("SMILES contains invalid characters").
			WithDetail(fmt.Sprintf("smiles=%s", smiles))
	}

	// Bracket matching validation
	if err := validateBrackets(smiles); err != nil {
		return nil, err
	}

	// Canonicalise SMILES (simplified: just use the input as canonical for now;
	// real implementation would call RDKit's Chem.CanonSmiles)
	canonicalSMILES := canonicaliseSMILES(smiles)

	// Generate InChIKey (simplified: SHA256 hash of canonical SMILES)
	inchiKey := generateInChIKey(canonicalSMILES)

	mol := &Molecule{
		BaseEntity: common.BaseEntity{
			ID: common.NewID(),
		},
		SMILES:          smiles,
		CanonicalSMILES: canonicalSMILES,
		InChIKey:        inchiKey,
		Type:            moleculeType,
		Fingerprints:    make(map[mtypes.FingerprintType]*Fingerprint),
		SourcePatentIDs: []common.ID{},
	}

	// Publish domain event
	mol.events = append(mol.events, MoleculeCreatedEvent{
		MoleculeID: mol.ID,
		SMILES:     mol.SMILES,
		Type:       mol.Type,
	})

	return mol, nil
}

// validateBrackets checks that all brackets in the SMILES string are balanced.
func validateBrackets(smiles string) error {
	var stack []rune
	pairs := map[rune]rune{
		'(': ')',
		'[': ']',
	}
	closers := map[rune]rune{
		')': '(',
		']': '[',
	}

	for _, ch := range smiles {
		if opener, ok := pairs[ch]; ok {
			_ = opener
			stack = append(stack, ch)
		} else if expected, ok := closers[ch]; ok {
			if len(stack) == 0 || stack[len(stack)-1] != expected {
				return errors.InvalidParam("unmatched brackets in SMILES").
					WithDetail(fmt.Sprintf("smiles=%s", smiles))
			}
			stack = stack[:len(stack)-1]
		}
	}

	if len(stack) != 0 {
		return errors.InvalidParam("unclosed brackets in SMILES").
			WithDetail(fmt.Sprintf("smiles=%s", smiles))
	}

	return nil
}

// canonicaliseSMILES produces a canonical (normalized) SMILES string.
// This is a placeholder implementation; a production system would use RDKit's
// Chem.MolToSmiles with canonical=True.
func canonicaliseSMILES(smiles string) string {
	// Simplified: just return the input in lowercase as "canonical"
	// Real implementation: call RDKit via cgo or Python subprocess
	return strings.ToLower(smiles)
}

// generateInChIKey generates a 27-character InChIKey-like identifier from
// the canonical SMILES.  This is a simplified implementation using SHA256;
// real InChIKey generation requires the InChI library.
func generateInChIKey(canonicalSMILES string) string {
	hash := sha256.Sum256([]byte(canonicalSMILES))
	// Take first 27 characters of the hex digest and format like an InChIKey
	hexStr := hex.EncodeToString(hash[:])
	key := strings.ToUpper(hexStr[:14]) + "-" + strings.ToUpper(hexStr[14:24]) + "-" + strings.ToUpper(hexStr[24:25])
	return key
}

// ─────────────────────────────────────────────────────────────────────────────
// Fingerprint Calculation
// ─────────────────────────────────────────────────────────────────────────────

// CalculateFingerprint computes and stores the specified fingerprint type for
// this molecule.  The computed fingerprint is stored in the Fingerprints map
// and can be retrieved later for similarity comparisons.
func (m *Molecule) CalculateFingerprint(fpType mtypes.FingerprintType) error {
	var fp *Fingerprint
	var err error

	switch fpType {
	case mtypes.FPMorgan:
		fp, err = CalculateMorganFingerprint(m.CanonicalSMILES, 2, 2048)
	case mtypes.FPMACCS:
		fp, err = CalculateMACCSFingerprint(m.CanonicalSMILES)
	case mtypes.FPTopological:
		fp, err = CalculateTopologicalFingerprint(m.CanonicalSMILES, 1, 7, 2048)
	case mtypes.FPAtomPair:
		// Atom-pair fingerprint not implemented in this simplified version
		return errors.New(errors.CodeNotImplemented, "atom-pair fingerprint not yet implemented")
	default:
		return errors.InvalidParam("unknown fingerprint type").
			WithDetail(fmt.Sprintf("type=%s", fpType))
	}

	if err != nil {
		return errors.Wrap(err, errors.CodeMoleculeInvalidSMILES, "fingerprint calculation failed")
	}

	m.Fingerprints[fpType] = fp

	// Publish domain event
	m.events = append(m.events, FingerprintCalculatedEvent{
		MoleculeID:      m.ID,
		FingerprintType: fpType,
	})

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Property Calculation
// ─────────────────────────────────────────────────────────────────────────────

// CalculateProperties computes basic physicochemical properties for the molecule.
// This is a simplified implementation; a production system would use RDKit
// descriptors or a dedicated cheminformatics library.
func (m *Molecule) CalculateProperties() error {
	// Simplified property calculation based on SMILES analysis
	smiles := m.CanonicalSMILES

	// Count aromatic rings (simple heuristic: count lowercase 'c' atoms)
	aromaticRings := strings.Count(smiles, "c") / 6 // rough estimate

	// Count rotatable bonds (simplified: count single bonds not in rings)
	rotatableBonds := strings.Count(smiles, "-")

	// Count H-bond donors (simplified: count N and O atoms)
	hDonors := strings.Count(smiles, "N") + strings.Count(smiles, "n") +
		strings.Count(smiles, "O") + strings.Count(smiles, "o")

	// Count H-bond acceptors (same as donors for this simplified model)
	hAcceptors := hDonors

	// Estimate molecular weight (simplified: count C, N, O atoms)
	// C=12, N=14, O=16
	cCount := strings.Count(smiles, "C") + strings.Count(smiles, "c")
	nCount := strings.Count(smiles, "N") + strings.Count(smiles, "n")
	oCount := strings.Count(smiles, "O") + strings.Count(smiles, "o")
	molWeight := float64(cCount)*12.0 + float64(nCount)*14.0 + float64(oCount)*16.0

	// Estimate LogP (simplified: aromatic rings increase LogP)
	logP := float64(aromaticRings) * 0.5

	// Estimate TPSA (simplified: based on O and N count)
	tpsa := float64(oCount+nCount) * 20.0

	m.Properties = MolecularProperties{
		LogP:           logP,
		TPSA:           tpsa,
		HBondDonors:    hDonors,
		HBondAcceptors: hAcceptors,
		RotatableBonds: rotatableBonds,
		AromaticRings:  aromaticRings,
	}

	// Generate molecular formula (simplified)
	m.MolecularFormula = fmt.Sprintf("C%dN%dO%d", cCount, nCount, oCount)
	m.MolecularWeight = molWeight

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Similarity Computation
// ─────────────────────────────────────────────────────────────────────────────

// SimilarityTo computes the Tanimoto similarity between this molecule and
// another molecule using the specified fingerprint type.  Both molecules must
// have the requested fingerprint already computed.
//
// Returns a value in [0.0, 1.0] where 1.0 indicates identical fingerprints.
func (m *Molecule) SimilarityTo(other *Molecule, fpType mtypes.FingerprintType) (float64, error) {
	fp1, ok := m.Fingerprints[fpType]
	if !ok {
		return 0, errors.New(errors.CodeMoleculeInvalidSMILES,
			"fingerprint not computed for source molecule").
			WithDetail(fmt.Sprintf("type=%s", fpType))
	}

	fp2, ok := other.Fingerprints[fpType]
	if !ok {
		return 0, errors.New(errors.CodeMoleculeInvalidSMILES,
			"fingerprint not computed for target molecule").
			WithDetail(fmt.Sprintf("type=%s", fpType))
	}

	return TanimotoSimilarity(fp1, fp2)
}

// ─────────────────────────────────────────────────────────────────────────────
// Source Patent Management
// ─────────────────────────────────────────────────────────────────────────────

// AddSourcePatent records that this molecule was extracted from the given patent.
// Duplicate patent IDs are silently ignored.
func (m *Molecule) AddSourcePatent(patentID common.ID) {
	// Check for duplicates
	for _, id := range m.SourcePatentIDs {
		if id == patentID {
			return
		}
	}
	m.SourcePatentIDs = append(m.SourcePatentIDs, patentID)
}

// ─────────────────────────────────────────────────────────────────────────────
// OLED Material Methods
// ─────────────────────────────────────────────────────────────────────────────

// IsOLEDMaterial returns true if this molecule is classified as an OLED material.
func (m *Molecule) IsOLEDMaterial() bool {
	return m.Type == mtypes.TypeOLEDMaterial
}

// SetOLEDProperties sets the HOMO, LUMO, and band gap energies for this molecule.
// This method should only be called for molecules with Type == TypeOLEDMaterial.
func (m *Molecule) SetOLEDProperties(homo, lumo, bandGap float64) {
	m.Properties.HOMO = &homo
	m.Properties.LUMO = &lumo
	m.Properties.BandGap = &bandGap
}

// ─────────────────────────────────────────────────────────────────────────────
// DTO Conversion
// ─────────────────────────────────────────────────────────────────────────────

// ToDTO converts the domain entity to a data transfer object suitable for
// cross-layer communication.
func (m *Molecule) ToDTO() mtypes.MoleculeDTO {
	dto := mtypes.MoleculeDTO{
		BaseEntity:       m.BaseEntity,
		SMILES:           m.SMILES,
		InChI:            m.InChI,
		InChIKey:         m.InChIKey,
		MolecularFormula: m.MolecularFormula,
		MolecularWeight:  m.MolecularWeight,
		Name:             m.Name,
		Synonyms:         m.Synonyms,
		Type:             m.Type,
		Properties: mtypes.MolecularProperties{
			LogP:           m.Properties.LogP,
			TPSA:           m.Properties.TPSA,
			HBondDonors:    m.Properties.HBondDonors,
			HBondAcceptors: m.Properties.HBondAcceptors,
			RotatableBonds: m.Properties.RotatableBonds,
			AromaticRings:  m.Properties.AromaticRings,
			HOMO:           m.Properties.HOMO,
			LUMO:           m.Properties.LUMO,
			BandGap:        m.Properties.BandGap,
		},
		SourcePatentIDs: m.SourcePatentIDs,
	}

	// Convert fingerprints to byte slices
	if len(m.Fingerprints) > 0 {
		dto.Fingerprints = make(map[mtypes.FingerprintType][]byte)
		for fpType, fp := range m.Fingerprints {
			dto.Fingerprints[fpType] = fp.ToBytes()
		}
	}

	return dto
}

// MoleculeFromDTO reconstructs a domain entity from a DTO.
func MoleculeFromDTO(dto mtypes.MoleculeDTO) *Molecule {
	mol := &Molecule{
		BaseEntity:       dto.BaseEntity,
		SMILES:           dto.SMILES,
		CanonicalSMILES:  dto.SMILES, // DTO doesn't distinguish; use SMILES
		InChI:            dto.InChI,
		InChIKey:         dto.InChIKey,
		MolecularFormula: dto.MolecularFormula,
		MolecularWeight:  dto.MolecularWeight,
		Name:             dto.Name,
		Synonyms:         dto.Synonyms,
		Type:             dto.Type,
		Properties: MolecularProperties{
			LogP:           dto.Properties.LogP,
			TPSA:           dto.Properties.TPSA,
			HBondDonors:    dto.Properties.HBondDonors,
			HBondAcceptors: dto.Properties.HBondAcceptors,
			RotatableBonds: dto.Properties.RotatableBonds,
			AromaticRings:  dto.Properties.AromaticRings,
			HOMO:           dto.Properties.HOMO,
			LUMO:           dto.Properties.LUMO,
			BandGap:        dto.Properties.BandGap,
		},
		SourcePatentIDs: dto.SourcePatentIDs,
		Fingerprints:    make(map[mtypes.FingerprintType]*Fingerprint),
	}

	// Reconstruct fingerprints from byte slices
	for fpType, bits := range dto.Fingerprints {
		// Fingerprint length is embedded in the serialized data
		// For now, assume 2048 bits for Morgan, 166 for MACCS, etc.
		var length int
		switch fpType {
		case mtypes.FPMorgan, mtypes.FPTopological:
			length = 2048
		case mtypes.FPMACCS:
			length = 166
		default:
			length = len(bits) * 8
		}
		mol.Fingerprints[fpType] = FingerprintFromBytes(fpType, bits, length)
	}

	return mol
}

// ─────────────────────────────────────────────────────────────────────────────
// Domain Event Management
// ─────────────────────────────────────────────────────────────────────────────

// Events returns all unpublished domain events and clears the internal event list.
func (m *Molecule) Events() []DomainEvent {
	events := m.events
	m.events = nil
	return events
}

