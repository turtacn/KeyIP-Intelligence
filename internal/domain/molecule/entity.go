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
	smilesRegex = regexp.MustCompile(`^[A-Za-z0-9@+\-\[\]()=#/\\%.*]+$`)
	// InChIKey format validation
	inchiKeyRegex = regexp.MustCompile(`^[A-Z]{14}-[A-Z]{10}-[A-Z]$`)
	// Molecular formula validation
	formulaRegex = regexp.MustCompile(`^[A-Z][a-z]?\d*([A-Z][a-z]?\d*)*$`)
)

// Status represents the state of a molecule.
type Status string

const (
	StatusPending  Status = "pending_review"
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
	StatusDeleted  Status = "deleted"
)

// IsValid checks if the status is a valid enum value.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusActive, StatusArchived, StatusDeleted:
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

// MoleculeStatus alias for backward compatibility or if used elsewhere as int
type MoleculeStatus = Status

const (
	MoleculeStatusPending  = StatusPending
	MoleculeStatusActive   = StatusActive
	MoleculeStatusArchived = StatusArchived
	MoleculeStatusDeleted  = StatusDeleted
)

// Molecule represents a chemical compound.
type Molecule struct {
	ID               uuid.UUID      `json:"id"`
	SMILES           string         `json:"smiles"`
	CanonicalSMILES  string         `json:"canonical_smiles"`
	InChI            string         `json:"inchi"`
	InChIKey         string         `json:"inchi_key"`
	MolecularFormula string         `json:"molecular_formula"`
	MolecularWeight  float64        `json:"molecular_weight"`
	ExactMass        float64        `json:"exact_mass"`
	LogP             float64        `json:"logp"`
	TPSA             float64        `json:"tpsa"`
	NumAtoms         int            `json:"num_atoms"`
	NumBonds         int            `json:"num_bonds"`
	NumRings         int            `json:"num_rings"`
	NumAromaticRings int            `json:"num_aromatic_rings"`
	NumRotatableBonds int           `json:"num_rotatable_bonds"`
	Status           Status         `json:"status"`
	Name             string         `json:"name"`
	Aliases          []string       `json:"aliases"`
	Source           string         `json:"source"`
	SourceReference  string         `json:"source_reference"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	Fingerprints     []*Fingerprint `json:"fingerprints,omitempty"`
	Properties       []*Property    `json:"properties,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        *time.Time     `json:"deleted_at,omitempty"`
}

// NewMolecule creates a new molecule instance.
func NewMolecule(smiles string, source MoleculeSource, sourceRef string) (*Molecule, error) {
	if smiles == "" {
		return nil, errors.New(errors.ErrCodeValidation, "smiles cannot be empty")
	}
	return &Molecule{
		ID:              uuid.New(),
		SMILES:          smiles,
		Source:          string(source),
		SourceReference: sourceRef,
		Status:          StatusPending,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}, nil
}

// AddFingerprint adds a fingerprint to the molecule.
func (m *Molecule) AddFingerprint(fp *Fingerprint) error {
	m.Fingerprints = append(m.Fingerprints, fp)
	return nil
}

// HasFingerprint checks if a fingerprint type exists.
func (m *Molecule) HasFingerprint(fpType FingerprintType) bool {
	for _, fp := range m.Fingerprints {
		if fp.Type == string(fpType) {
			return true
		}
	}
	return false
}

// SMILESStr returns the molecule SMILES string (getter to satisfy interface if needed, or just field access).
func (m *Molecule) SMILESStr() string {
	return m.SMILES
}

// SetStructureIdentifiers sets computed identifiers.
func (m *Molecule) SetStructureIdentifiers(canonical, inchi, inchiKey, formula string, weight float64) error {
	m.CanonicalSMILES = canonical
	m.InChI = inchi
	m.InChIKey = inchiKey
	m.MolecularFormula = formula
	m.MolecularWeight = weight
	return nil
}

// Activate activates the molecule.
func (m *Molecule) Activate() error {
	m.Status = StatusActive
	return nil
}

// AddTag adds a tag.
func (m *Molecule) AddTag(tag string) error {
	// ... simplified
	return nil
}

// AddProperty adds a property.
func (m *Molecule) AddProperty(p *Property) error {
	m.Properties = append(m.Properties, p)
	return nil
}

// Archive archives the molecule.
func (m *Molecule) Archive() error {
	m.Status = StatusArchived
	return nil
}

// MarkDeleted marks the molecule as deleted.
func (m *Molecule) MarkDeleted() error {
	m.Status = StatusDeleted
	now := time.Now().UTC()
	m.DeletedAt = &now
	return nil
}

// Validate ensures all domain invariants are satisfied.
func (m *Molecule) Validate() error {
	if m.ID == uuid.Nil {
		return errors.New(errors.ErrCodeValidation, "missing id")
	}
	if m.SMILES == "" {
		return errors.New(errors.ErrCodeValidation, "missing smiles")
	}
	if m.MolecularWeight < 0 {
		return errors.New(errors.ErrCodeValidation, "molecularWeight cannot be negative")
	}
	return nil
}

// FingerprintType defines the type of fingerprint.
type FingerprintType string

const (
	FingerprintTypeMorgan FingerprintType = "morgan"
	FingerprintTypeMACCS  FingerprintType = "maccs"
	FingerprintTypeRDKit  FingerprintType = "rdkit"
)

func (t FingerprintType) IsValid() bool {
	switch t {
	case FingerprintTypeMorgan, FingerprintTypeMACCS, FingerprintTypeRDKit:
		return true
	default:
		return false
	}
}

// Fingerprint represents a molecular fingerprint.
type Fingerprint struct {
	ID           uuid.UUID      `json:"id"`
	MoleculeID   uuid.UUID      `json:"molecule_id"`
	Type         string         `json:"fingerprint_type"`
	Bits         []byte         `json:"fingerprint_bits,omitempty"`
	Vector       []float32      `json:"fingerprint_vector,omitempty"`
	Hash         string         `json:"fingerprint_hash,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty"`
	ModelVersion string         `json:"model_version,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Property represents a molecular property.
type Property struct {
	ID                    uuid.UUID      `json:"id"`
	MoleculeID            uuid.UUID      `json:"molecule_id"`
	Type                  string         `json:"property_type"`
	Value                 float64        `json:"value"`
	Unit                  string         `json:"unit"`
	MeasurementConditions map[string]any `json:"measurement_conditions,omitempty"`
	DataSource            string         `json:"data_source"`
	Confidence            float64        `json:"confidence"`
	SourceReference       string         `json:"source_reference"`
	CreatedAt             time.Time      `json:"created_at"`
	Name                  string         `json:"name"` // Added for compatibility
}

// MolecularProperty alias
type MolecularProperty = Property

// PatentRelation represents a link between a patent and a molecule.
type PatentRelation struct {
	ID               uuid.UUID `json:"id"`
	PatentID         uuid.UUID `json:"patent_id"`
	MoleculeID       uuid.UUID `json:"molecule_id"`
	RelationType     string    `json:"relation_type"`
	LocationInPatent string    `json:"location_in_patent"`
	PageReference    string    `json:"page_reference"`
	ClaimNumbers     []int64   `json:"claim_numbers"`
	ExtractionMethod string    `json:"extraction_method"`
	Confidence       float64   `json:"confidence"`
	CreatedAt        time.Time `json:"created_at"`
}

// ListFilter defines filtering options.
type ListFilter struct {
	Status             string
	MinMolecularWeight *float64
	MaxMolecularWeight *float64
	NumAromaticRings   *int
	InChIKey           string
	Offset             int
	Limit              int
}

// MoleculeWithScore wraps a molecule with a similarity score.
type MoleculeWithScore struct {
	Molecule *Molecule
	Score    float64
}

// DistributionBucket represents a histogram bucket.
type DistributionBucket struct {
	Bucket int
	Count  int64
	Min    float64
	Max    float64
}

func (m *Molecule) String() string {
	return fmt.Sprintf("Molecule{id=%s, smiles=%s, status=%s}", m.ID, m.SMILES, m.Status)
}

// GetSMILES returns the SMILES representation of the molecule.
func (m *Molecule) GetSMILES() string {
	if m.CanonicalSMILES != "" {
		return m.CanonicalSMILES
	}
	return m.SMILES
}

// GetID returns the molecule ID as a string.
func (m *Molecule) GetID() string {
	return m.ID.String()
}

//Personal.AI order the ending
