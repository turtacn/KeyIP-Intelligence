package molecule

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	// Basic SMILES character set validation
	smilesRegex = regexp.MustCompile(`^[A-Za-z0-9@+\-\[\]\(\)\\\/%=#\.]+$`)
	// InChIKey format validation
	inchiKeyRegex = regexp.MustCompile(`^[A-Z]{14}-[A-Z]{10}-[A-Z]$`)
	// Molecular formula validation
	formulaRegex = regexp.MustCompile(`^[A-Z][a-z]?\d*([A-Z][a-z]?\d*)*$`)
)

// MoleculeStatus represents the state of a molecule.
type MoleculeStatus int8

const (
	MoleculeStatusPending  MoleculeStatus = iota // 0
	MoleculeStatusActive                         // 1
	MoleculeStatusArchived                       // 2
	MoleculeStatusDeleted                        // 3
)

// Status alias for backward compatibility
type Status = MoleculeStatus

func (s MoleculeStatus) String() string {
	switch s {
	case MoleculeStatusPending:
		return "pending"
	case MoleculeStatusActive:
		return "active"
	case MoleculeStatusArchived:
		return "archived"
	case MoleculeStatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

func (s MoleculeStatus) IsValid() bool {
	switch s {
	case MoleculeStatusPending, MoleculeStatusActive, MoleculeStatusArchived, MoleculeStatusDeleted:
		return true
	default:
		return false
	}
}

// MoleculeSource identifies the origin of the molecule data.
type MoleculeSource string

const (
	SourcePatent     MoleculeSource = "patent"
	SourceLiterature MoleculeSource = "literature"
	SourceExperiment MoleculeSource = "experiment"
	SourcePrediction MoleculeSource = "prediction"
	SourceManual     MoleculeSource = "manual"
)

func (s MoleculeSource) IsValid() bool {
	switch s {
	case SourcePatent, SourceLiterature, SourceExperiment, SourcePrediction, SourceManual:
		return true
	default:
		return false
	}
}

// MolecularProperty represents a physical or chemical property.
type MolecularProperty struct {
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Condition  string  `json:"condition"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// Property alias for backward compatibility
type Property = MolecularProperty

// PatentRelation represents a link between a molecule and a patent.
// Restored for compatibility with repository.
type PatentRelation struct {
	MoleculeID   string    `json:"molecule_id"`
	PatentNumber string    `json:"patent_number"`
	RelationType string    `json:"relation_type"` // e.g., "primary", "related"
	Confidence   float64   `json:"confidence"`
	CreatedAt    time.Time `json:"created_at"`
}

// DistributionBucket represents a histogram bucket for property distribution.
// Restored for compatibility with repository.
type DistributionBucket struct {
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Count int64   `json:"count"`
}

// MoleculeWithScore wraps a molecule with a similarity score.
// Restored for compatibility with repository.
type MoleculeWithScore struct {
	Molecule *Molecule `json:"molecule"`
	Score    float64   `json:"score"`
}

// Molecule represents a chemical compound as an Aggregate Root.
type Molecule struct {
	id               string // UUID v4 string
	smiles           string
	inchi            string
	inchiKey         string
	molecularFormula string
	molecularWeight  float64
	canonicalSmiles  string
	fingerprints     map[FingerprintType]*Fingerprint
	properties       map[string]*MolecularProperty
	source           MoleculeSource
	sourceRef        string
	tags             []string
	status           MoleculeStatus
	metadata         map[string]string
	createdAt        time.Time
	updatedAt        time.Time
	deletedAt        *time.Time // Added for soft delete support in repo
	version          int64

	// Legacy fields support (via methods)
	// exactMass float64 // removed, strictly follow new spec or add if needed
}

// NewMolecule creates a new molecule instance.
func NewMolecule(smiles string, source MoleculeSource, sourceRef string) (*Molecule, error) {
	if len(smiles) == 0 {
		return nil, errors.New(errors.ErrCodeValidation, "smiles cannot be empty")
	}
	if len(smiles) > 10000 {
		return nil, errors.New(errors.ErrCodeValidation, "smiles too long")
	}

	if !smilesRegex.MatchString(smiles) {
		return nil, errors.New(errors.ErrCodeValidation, "invalid characters in SMILES")
	}

	open := 0
	for _, c := range smiles {
		if c == '(' {
			open++
		} else if c == ')' {
			open--
		}
		if open < 0 {
			return nil, errors.New(errors.ErrCodeValidation, "unbalanced parentheses")
		}
	}
	if open != 0 {
		return nil, errors.New(errors.ErrCodeValidation, "unbalanced parentheses")
	}

	if !source.IsValid() {
		return nil, errors.New(errors.ErrCodeValidation, "invalid source")
	}

	return &Molecule{
		id:           uuid.New().String(),
		smiles:       smiles,
		source:       source,
		sourceRef:    sourceRef,
		status:       MoleculeStatusPending,
		fingerprints: make(map[FingerprintType]*Fingerprint),
		properties:   make(map[string]*MolecularProperty),
		metadata:     make(map[string]string),
		tags:         make([]string, 0),
		createdAt:    time.Now().UTC(),
		updatedAt:    time.Now().UTC(),
		version:      1,
	}, nil
}

// RestoreMolecule reconstructs a molecule from persistence.
// TRUSTED CALL: Should only be used by Repositories.
func RestoreMolecule(
	id, smiles, inchi, inchiKey, formula, canonicalSmiles string,
	weight float64,
	source MoleculeSource, sourceRef string,
	status MoleculeStatus,
	tags []string,
	metadata map[string]string,
	createdAt, updatedAt time.Time,
	deletedAt *time.Time,
	version int64,
) *Molecule {
	if tags == nil {
		tags = make([]string, 0)
	}
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &Molecule{
		id:               id,
		smiles:           smiles,
		inchi:            inchi,
		inchiKey:         inchiKey,
		molecularFormula: formula,
		molecularWeight:  weight,
		canonicalSmiles:  canonicalSmiles,
		source:           source,
		sourceRef:        sourceRef,
		status:           status,
		tags:             tags,
		metadata:         metadata,
		createdAt:        createdAt,
		updatedAt:        updatedAt,
		deletedAt:        deletedAt,
		version:          version,
		fingerprints:     make(map[FingerprintType]*Fingerprint),
		properties:       make(map[string]*MolecularProperty),
	}
}

// Validate ensures all domain invariants are satisfied.
func (m *Molecule) Validate() error {
	if m.id == "" {
		return errors.New(errors.ErrCodeValidation, "missing id")
	}
	if m.smiles == "" {
		return errors.New(errors.ErrCodeValidation, "missing smiles")
	}
	if m.molecularWeight < 0 {
		return errors.New(errors.ErrCodeValidation, "molecularWeight cannot be negative")
	}

	if m.inchiKey != "" && !inchiKeyRegex.MatchString(m.inchiKey) {
		return errors.New(errors.ErrCodeValidation, "invalid InChIKey format")
	}

	if !m.status.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid status")
	}
	if !m.source.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid source")
	}

	for _, p := range m.properties {
		if p.Confidence < 0.0 || p.Confidence > 1.0 {
			return errors.New(errors.ErrCodeValidation, fmt.Sprintf("invalid confidence for property %s", p.Name))
		}
	}
	return nil
}

// Activate activates the molecule.
func (m *Molecule) Activate() error {
	if m.status != MoleculeStatusPending {
		return errors.New(errors.ErrCodeValidation, "can only activate pending molecule")
	}
	if m.inchiKey == "" {
		return errors.New(errors.ErrCodeValidation, "cannot activate molecule without InChIKey")
	}
	m.status = MoleculeStatusActive
	m.update()
	return nil
}

// Archive archives the molecule.
func (m *Molecule) Archive() error {
	if m.status != MoleculeStatusActive {
		return errors.New(errors.ErrCodeValidation, "can only archive active molecule")
	}
	m.status = MoleculeStatusArchived
	m.update()
	return nil
}

// MarkDeleted marks the molecule as deleted.
func (m *Molecule) MarkDeleted() error {
	if m.status == MoleculeStatusDeleted {
		return errors.New(errors.ErrCodeValidation, "already deleted")
	}
	if m.status == MoleculeStatusPending {
		return errors.New(errors.ErrCodeValidation, "cannot delete pending molecule")
	}
	m.status = MoleculeStatusDeleted
	now := time.Now().UTC()
	m.deletedAt = &now
	m.update()
	return nil
}

// SetStructureIdentifiers sets computed identifiers.
func (m *Molecule) SetStructureIdentifiers(canonical, inchi, inchiKey, formula string, weight float64) error {
	if canonical == "" {
		return errors.New(errors.ErrCodeValidation, "canonical smiles cannot be empty")
	}
	if inchi == "" {
		return errors.New(errors.ErrCodeValidation, "inchi cannot be empty")
	}
	if !inchiKeyRegex.MatchString(inchiKey) {
		return errors.New(errors.ErrCodeValidation, "invalid inchiKey")
	}
	if weight <= 0 {
		return errors.New(errors.ErrCodeValidation, "molecular weight must be positive")
	}

	m.canonicalSmiles = canonical
	m.inchi = inchi
	m.inchiKey = inchiKey
	m.molecularFormula = formula
	m.molecularWeight = weight
	m.update()
	return nil
}

// AddFingerprint adds a fingerprint.
func (m *Molecule) AddFingerprint(fp *Fingerprint) error {
	if fp == nil {
		return errors.New(errors.ErrCodeValidation, "fingerprint cannot be nil")
	}
	if !fp.Type.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid fingerprint type")
	}
	m.fingerprints[fp.Type] = fp
	m.update()
	return nil
}

// GetFingerprint gets a fingerprint.
func (m *Molecule) GetFingerprint(fpType FingerprintType) (*Fingerprint, bool) {
	fp, ok := m.fingerprints[fpType]
	return fp, ok
}

// HasFingerprint checks if a fingerprint exists.
func (m *Molecule) HasFingerprint(fpType FingerprintType) bool {
	_, ok := m.fingerprints[fpType]
	return ok
}

// AddProperty adds a property.
func (m *Molecule) AddProperty(prop *MolecularProperty) error {
	if prop == nil {
		return errors.New(errors.ErrCodeValidation, "property cannot be nil")
	}
	if prop.Name == "" {
		return errors.New(errors.ErrCodeValidation, "property name cannot be empty")
	}
	if prop.Confidence < 0 || prop.Confidence > 1 {
		return errors.New(errors.ErrCodeValidation, "invalid confidence")
	}
	m.properties[prop.Name] = prop
	m.update()
	return nil
}

// GetProperty gets a property.
func (m *Molecule) GetProperty(name string) (*MolecularProperty, bool) {
	p, ok := m.properties[name]
	return p, ok
}

// AddTag adds a tag.
func (m *Molecule) AddTag(tag string) error {
	if tag == "" {
		return errors.New(errors.ErrCodeValidation, "tag cannot be empty")
	}
	if len(tag) > 64 {
		return errors.New(errors.ErrCodeValidation, "tag too long")
	}
	for _, t := range m.tags {
		if t == tag {
			return nil // Duplicate ignored
		}
	}
	m.tags = append(m.tags, tag)
	m.update()
	return nil
}

// RemoveTag removes a tag.
func (m *Molecule) RemoveTag(tag string) {
	for i, t := range m.tags {
		if t == tag {
			m.tags = append(m.tags[:i], m.tags[i+1:]...)
			m.update()
			return
		}
	}
}

// SetMetadata sets metadata.
func (m *Molecule) SetMetadata(key, value string) {
	m.metadata[key] = value
	m.update()
}

// Getters

func (m *Molecule) ID() string { return m.id }
func (m *Molecule) SMILES() string { return m.smiles }
func (m *Molecule) InChI() string { return m.inchi }
func (m *Molecule) InChIKey() string { return m.inchiKey }
func (m *Molecule) MolecularFormula() string { return m.molecularFormula }
func (m *Molecule) MolecularWeight() float64 { return m.molecularWeight }
func (m *Molecule) CanonicalSmiles() string { return m.canonicalSmiles }
func (m *Molecule) Source() MoleculeSource { return m.source }
func (m *Molecule) SourceRef() string { return m.sourceRef }
func (m *Molecule) Tags() []string {
	cp := make([]string, len(m.tags))
	copy(cp, m.tags)
	return cp
}
func (m *Molecule) Status() MoleculeStatus { return m.status }
func (m *Molecule) Metadata() map[string]string {
	cp := make(map[string]string)
	for k, v := range m.metadata {
		cp[k] = v
	}
	return cp
}
func (m *Molecule) CreatedAt() time.Time { return m.createdAt }
func (m *Molecule) UpdatedAt() time.Time { return m.updatedAt }
func (m *Molecule) DeletedAt() *time.Time { return m.deletedAt }
func (m *Molecule) Version() int64 { return m.version }
func (m *Molecule) Fingerprints() map[FingerprintType]*Fingerprint {
	cp := make(map[FingerprintType]*Fingerprint)
	for k, v := range m.fingerprints {
		cp[k] = v
	}
	return cp
}
func (m *Molecule) Properties() map[string]*MolecularProperty {
	cp := make(map[string]*MolecularProperty)
	for k, v := range m.properties {
		cp[k] = v
	}
	return cp
}

// Status helpers
func (m *Molecule) IsActive() bool { return m.status == MoleculeStatusActive }
func (m *Molecule) IsArchived() bool { return m.status == MoleculeStatusArchived }
func (m *Molecule) IsDeleted() bool { return m.status == MoleculeStatusDeleted }
func (m *Molecule) IsPending() bool { return m.status == MoleculeStatusPending }

func (m *Molecule) String() string {
	return fmt.Sprintf("Molecule{id=%s, smiles=%s, status=%s}", m.id, m.smiles, m.status)
}

func (m *Molecule) update() {
	m.updatedAt = time.Now().UTC()
	m.version++
}

//Personal.AI order the ending
