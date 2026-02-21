package molecule

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MoleculeStatus defines the business state of a molecule.
type MoleculeStatus int8

const (
	MoleculeStatusPending  MoleculeStatus = 0
	MoleculeStatusActive   MoleculeStatus = 1
	MoleculeStatusArchived MoleculeStatus = 2
	MoleculeStatusDeleted  MoleculeStatus = 3
)

// String returns the string representation of the molecule status.
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
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// IsValid checks if the status is a valid enum value.
func (s MoleculeStatus) IsValid() bool {
	return s >= MoleculeStatusPending && s <= MoleculeStatusDeleted
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

// MolecularProperty represents a physicochemical or optoelectronic property.
type MolecularProperty struct {
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Condition  string  `json:"condition"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// Molecule is the aggregate root for molecular entities.
type Molecule struct {
	id               string
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
	version          int64
}

var (
	// Basic SMILES character set validation
	smilesRegex = regexp.MustCompile(`^[A-Za-z0-9@+\-\[\]()=#/\\%.*]+$`)
	// InChIKey format validation
	inchiKeyRegex = regexp.MustCompile(`^[A-Z]{14}-[A-Z]{10}-[A-Z]$`)
	// Molecular formula validation
	formulaRegex = regexp.MustCompile(`^[A-Z][a-z]?\d*([A-Z][a-z]?\d*)*$`)
)

// NewMolecule constructs a new Molecule entity in Pending status.
func NewMolecule(smiles string, source MoleculeSource, sourceRef string) (*Molecule, error) {
	smiles = strings.TrimSpace(smiles)
	if smiles == "" {
		return nil, errors.New(errors.ErrCodeValidation, "SMILES cannot be empty")
	}
	if len(smiles) > 10000 {
		return nil, errors.New(errors.ErrCodeValidation, "SMILES too long")
	}
	if !smilesRegex.MatchString(smiles) {
		return nil, errors.New(errors.ErrCodeMoleculeInvalidSMILES, "SMILES contains invalid characters")
	}
	if err := validateBrackets(smiles); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	return &Molecule{
		id:           uuid.New().String(),
		smiles:       smiles,
		source:       source,
		sourceRef:    sourceRef,
		status:       MoleculeStatusPending,
		fingerprints: make(map[FingerprintType]*Fingerprint),
		properties:   make(map[string]*MolecularProperty),
		metadata:     make(map[string]string),
		createdAt:    now,
		updatedAt:    now,
		version:      1,
	}, nil
}

func validateBrackets(s string) error {
	var stack []rune
	pairs := map[rune]rune{')': '(', ']': '[', '}': '{'}
	for _, r := range s {
		if r == '(' || r == '[' || r == '{' {
			stack = append(stack, r)
		} else if opening, ok := pairs[r]; ok {
			if len(stack) == 0 || stack[len(stack)-1] != opening {
				return errors.New(errors.ErrCodeMoleculeInvalidSMILES, "unbalanced brackets in SMILES")
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return errors.New(errors.ErrCodeMoleculeInvalidSMILES, "unclosed brackets in SMILES")
	}
	return nil
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
		return errors.New(errors.ErrCodeValidation, "invalid inchiKey format")
	}
	if !m.status.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid status")
	}
	if m.molecularFormula != "" && !formulaRegex.MatchString(m.molecularFormula) {
		return errors.New(errors.ErrCodeValidation, "invalid molecularFormula format")
	}
	for name, prop := range m.properties {
		if prop.Confidence < 0.0 || prop.Confidence > 1.0 {
			return errors.New(errors.ErrCodeValidation, fmt.Sprintf("invalid confidence for property %s", name))
		}
	}
	return nil
}

// Activate transitions the molecule to Active status.
func (m *Molecule) Activate() error {
	if m.status != MoleculeStatusPending {
		return errors.New(errors.ErrCodeValidation, "can only activate from pending status")
	}
	if m.inchiKey == "" {
		return errors.New(errors.ErrCodeValidation, "cannot activate without inchiKey")
	}
	m.status = MoleculeStatusActive
	m.updatedAt = time.Now().UTC()
	m.version++
	return nil
}

// Archive transitions the molecule to Archived status.
func (m *Molecule) Archive() error {
	if m.status != MoleculeStatusActive {
		return errors.New(errors.ErrCodeValidation, "can only archive from active status")
	}
	m.status = MoleculeStatusArchived
	m.updatedAt = time.Now().UTC()
	m.version++
	return nil
}

// MarkDeleted transitions the molecule to Deleted status.
func (m *Molecule) MarkDeleted() error {
	if m.status != MoleculeStatusActive && m.status != MoleculeStatusArchived {
		return errors.New(errors.ErrCodeValidation, "can only delete from active or archived status")
	}
	m.status = MoleculeStatusDeleted
	m.updatedAt = time.Now().UTC()
	m.version++
	return nil
}

// SetStructureIdentifiers sets computed chemical identifiers.
func (m *Molecule) SetStructureIdentifiers(canonicalSmiles, inchi, inchiKey, formula string, weight float64) error {
	if canonicalSmiles == "" || inchi == "" || inchiKey == "" || formula == "" {
		return errors.New(errors.ErrCodeValidation, "identifiers cannot be empty")
	}
	if !inchiKeyRegex.MatchString(inchiKey) {
		return errors.New(errors.ErrCodeValidation, "invalid inchiKey format")
	}
	if weight <= 0 {
		return errors.New(errors.ErrCodeValidation, "weight must be positive")
	}

	m.canonicalSmiles = canonicalSmiles
	m.inchi = inchi
	m.inchiKey = inchiKey
	m.molecularFormula = formula
	m.molecularWeight = weight
	m.updatedAt = time.Now().UTC()
	m.version++
	return nil
}

// AddFingerprint adds a calculated fingerprint to the molecule.
func (m *Molecule) AddFingerprint(fp *Fingerprint) error {
	if fp == nil {
		return errors.New(errors.ErrCodeValidation, "fingerprint cannot be nil")
	}
	if !fp.Type.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid fingerprint type")
	}
	m.fingerprints[fp.Type] = fp
	m.updatedAt = time.Now().UTC()
	return nil
}

// GetFingerprint retrieves a fingerprint by type.
func (m *Molecule) GetFingerprint(fpType FingerprintType) (*Fingerprint, bool) {
	fp, ok := m.fingerprints[fpType]
	return fp, ok
}

// HasFingerprint checks if a fingerprint type exists.
func (m *Molecule) HasFingerprint(fpType FingerprintType) bool {
	_, ok := m.fingerprints[fpType]
	return ok
}

// AddProperty adds a molecular property.
func (m *Molecule) AddProperty(prop *MolecularProperty) error {
	if prop.Name == "" {
		return errors.New(errors.ErrCodeValidation, "property name cannot be empty")
	}
	if prop.Confidence < 0.0 || prop.Confidence > 1.0 {
		return errors.New(errors.ErrCodeValidation, "confidence must be between 0.0 and 1.0")
	}
	m.properties[prop.Name] = prop
	m.updatedAt = time.Now().UTC()
	return nil
}

// GetProperty retrieves a property by name.
func (m *Molecule) GetProperty(name string) (*MolecularProperty, bool) {
	prop, ok := m.properties[name]
	return prop, ok
}

// AddTag adds a unique tag to the molecule.
func (m *Molecule) AddTag(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return errors.New(errors.ErrCodeValidation, "tag cannot be empty")
	}
	if len(tag) > 64 {
		return errors.New(errors.ErrCodeValidation, "tag too long")
	}
	for _, t := range m.tags {
		if t == tag {
			return nil
		}
	}
	m.tags = append(m.tags, tag)
	m.updatedAt = time.Now().UTC()
	return nil
}

// RemoveTag removes a tag from the molecule.
func (m *Molecule) RemoveTag(tag string) {
	for i, t := range m.tags {
		if t == tag {
			m.tags = append(m.tags[:i], m.tags[i+1:]...)
			m.updatedAt = time.Now().UTC()
			return
		}
	}
}

// SetMetadata sets a metadata key-value pair.
func (m *Molecule) SetMetadata(key, value string) {
	m.metadata[key] = value
	m.updatedAt = time.Now().UTC()
}

// Getters

func (m *Molecule) ID() string               { return m.id }
func (m *Molecule) SMILES() string           { return m.smiles }
func (m *Molecule) InChI() string            { return m.inchi }
func (m *Molecule) InChIKey() string         { return m.inchiKey }
func (m *Molecule) MolecularFormula() string { return m.molecularFormula }
func (m *Molecule) MolecularWeight() float64 { return m.molecularWeight }
func (m *Molecule) CanonicalSmiles() string  { return m.canonicalSmiles }
func (m *Molecule) Source() MoleculeSource   { return m.source }
func (m *Molecule) SourceRef() string        { return m.sourceRef }
func (m *Molecule) Status() MoleculeStatus   { return m.status }
func (m *Molecule) CreatedAt() time.Time     { return m.createdAt }
func (m *Molecule) UpdatedAt() time.Time     { return m.updatedAt }
func (m *Molecule) Version() int64           { return m.version }

func (m *Molecule) Tags() []string {
	tags := make([]string, len(m.tags))
	copy(tags, m.tags)
	return tags
}

func (m *Molecule) Metadata() map[string]string {
	meta := make(map[string]string, len(m.metadata))
	for k, v := range m.metadata {
		meta[k] = v
	}
	return meta
}

func (m *Molecule) Fingerprints() map[FingerprintType]*Fingerprint {
	fps := make(map[FingerprintType]*Fingerprint, len(m.fingerprints))
	for k, v := range m.fingerprints {
		fps[k] = v
	}
	return fps
}

func (m *Molecule) Properties() map[string]*MolecularProperty {
	props := make(map[string]*MolecularProperty, len(m.properties))
	for k, v := range m.properties {
		props[k] = v
	}
	return props
}

// Status Helpers

func (m *Molecule) IsActive() bool   { return m.status == MoleculeStatusActive }
func (m *Molecule) IsArchived() bool { return m.status == MoleculeStatusArchived }
func (m *Molecule) IsDeleted() bool  { return m.status == MoleculeStatusDeleted }
func (m *Molecule) IsPending() bool  { return m.status == MoleculeStatusPending }

func (m *Molecule) String() string {
	return fmt.Sprintf("Molecule{id=%s, smiles=%s, status=%s}", m.id, m.smiles, m.status)
}

//Personal.AI order the ending
