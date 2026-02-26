package molecule

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MoleculeStatus represents the lifecycle state of a molecule.
type MoleculeStatus string

const (
	MoleculeStatusPending  MoleculeStatus = "pending"
	MoleculeStatusActive   MoleculeStatus = "active"
	MoleculeStatusArchived MoleculeStatus = "archived"
	MoleculeStatusDeleted  MoleculeStatus = "deleted"
)

func (s MoleculeStatus) String() string {
	return string(s)
}

func (s MoleculeStatus) IsValid() bool {
	switch s {
	case MoleculeStatusPending, MoleculeStatusActive, MoleculeStatusArchived, MoleculeStatusDeleted:
		return true
	default:
		return false
	}
}

// Status alias for backward compatibility
type Status = MoleculeStatus

const (
	StatusPending  = MoleculeStatusPending
	StatusActive   = MoleculeStatusActive
	StatusArchived = MoleculeStatusArchived
	StatusDeleted  = MoleculeStatusDeleted
)

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

// MolecularProperty represents a physicochemical property value.
type MolecularProperty struct {
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	Unit       string  `json:"unit"`
	Condition  string  `json:"condition"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
}

// Property struct for backward compatibility with repository
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
	Name                  string         `json:"name"`
}

// PatentRelation struct for backward compatibility
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

// DistributionBucket for backward compatibility
type DistributionBucket struct {
	Bucket int
	Count  int64
	Min    float64
	Max    float64
}

// MoleculeWithScore for backward compatibility
type MoleculeWithScore struct {
	Molecule *Molecule
	Score    float64
}

// Molecule is the aggregate root for the molecule domain.
type Molecule struct {
	ID                uuid.UUID                        `json:"id"`
	SMILES            string                           `json:"smiles"`
	InChI             string                           `json:"inchi"`
	InChIKey          string                           `json:"inchi_key"`
	MolecularFormula  string                           `json:"molecular_formula"`
	MolecularWeight   float64                          `json:"molecular_weight"`
	CanonicalSMILES   string                           `json:"canonical_smiles"`
	ExactMass         float64                          `json:"exact_mass"`
	LogP              float64                          `json:"logp"`
	TPSA              float64                          `json:"tpsa"`
	NumAtoms          int                              `json:"num_atoms"`
	NumBonds          int                              `json:"num_bonds"`
	NumRings          int                              `json:"num_rings"`
	NumAromaticRings  int                              `json:"num_aromatic_rings"`
	NumRotatableBonds int                              `json:"num_rotatable_bonds"`
	Status            MoleculeStatus                   `json:"status"`
	Name              string                           `json:"name"`
	Aliases           []string                         `json:"aliases"`
	Source            MoleculeSource                   `json:"source"`
	SourceReference   string                           `json:"source_reference"` // Matches repo usage? Check if repo maps SourceRef to SourceReference. Yes scanMolecule: &m.SourceReference
	Tags              []string                         `json:"tags"`             // Repo might not use this field directly in scanMolecule? scanMolecule didn't seem to scan Tags/Aliases? Ah, aliases is there. Tags? Not in scanMolecule.
	Metadata          map[string]any                   `json:"metadata"`         // Repo uses map[string]any for metadata json
	Fingerprints      []*Fingerprint                   `json:"fingerprints"`     // For compatibility with repo GetFingerprints populating this slice
	FingerprintsMap   map[FingerprintType]*Fingerprint `json:"-"`                // Domain logic map
	Properties        []*Property                      `json:"properties"`       // For compatibility
	PropertiesMap     map[string]*MolecularProperty    `json:"-"`                // Domain logic map
	CreatedAt         time.Time                        `json:"created_at"`
	UpdatedAt         time.Time                        `json:"updated_at"`
	DeletedAt         *time.Time                       `json:"deleted_at"`
	Version           int64                            `json:"version"`
}

// Regular expressions for validation
var (
	inchiKeyRegex = regexp.MustCompile(`^[A-Z]{14}-[A-Z]{10}-[A-Z]$`)
	formulaRegex  = regexp.MustCompile(`^[A-Za-z0-9]+$`)
	smilesRegex   = regexp.MustCompile(`^[A-Za-z0-9@+\-\[\]()=#/\\%.]+$`)
)

// NewMolecule creates a new molecule in Pending state.
func NewMolecule(smiles string, source MoleculeSource, sourceRef string) (*Molecule, error) {
	if len(smiles) == 0 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "smiles cannot be empty")
	}
	if len(smiles) > 10000 {
		return nil, errors.New(errors.ErrCodeInvalidInput, "smiles exceeds maximum length")
	}
	if !checkBalancedParentheses(smiles) {
		return nil, errors.New(errors.ErrCodeInvalidInput, "unbalanced parentheses in smiles")
	}
	if !smilesRegex.MatchString(smiles) {
		return nil, errors.New(errors.ErrCodeInvalidInput, "smiles contains illegal characters")
	}

	return &Molecule{
		ID:              uuid.New(),
		SMILES:          smiles,
		Source:          source,
		SourceReference: sourceRef,
		Status:          MoleculeStatusPending,
		FingerprintsMap: make(map[FingerprintType]*Fingerprint),
		PropertiesMap:   make(map[string]*MolecularProperty),
		Metadata:        make(map[string]any),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		Version:         1,
	}, nil
}

// checkBalancedParentheses checks if parentheses are balanced.
func checkBalancedParentheses(s string) bool {
	count := 0
	for _, c := range s {
		if c == '(' {
			count++
		} else if c == ')' {
			count--
			if count < 0 {
				return false
			}
		}
	}
	return count == 0
}

// Validate ensures all domain invariants are satisfied.
func (m *Molecule) Validate() error {
	if m.ID == uuid.Nil {
		return errors.New(errors.ErrCodeValidation, "id is required")
	}
	if m.SMILES == "" {
		return errors.New(errors.ErrCodeValidation, "smiles is required")
	}
	if !checkBalancedParentheses(m.SMILES) {
		return errors.New(errors.ErrCodeValidation, "unbalanced parentheses in smiles")
	}
	if m.MolecularWeight < 0 {
		return errors.New(errors.ErrCodeValidation, "molecular weight cannot be negative")
	}
	if m.InChIKey != "" && !inchiKeyRegex.MatchString(m.InChIKey) {
		return errors.New(errors.ErrCodeValidation, "invalid InChIKey format")
	}
	if !m.Status.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid status")
	}
	if !m.Source.IsValid() {
		return errors.New(errors.ErrCodeValidation, "invalid source")
	}
	if m.MolecularFormula != "" && !formulaRegex.MatchString(m.MolecularFormula) {
		return errors.New(errors.ErrCodeValidation, "invalid molecular formula format")
	}
	for name, p := range m.PropertiesMap {
		if p.Confidence < 0.0 || p.Confidence > 1.0 {
			return errors.New(errors.ErrCodeValidation, fmt.Sprintf("invalid confidence for property %s", name))
		}
	}
	return nil
}

// Activate transitions the molecule to Active status.
func (m *Molecule) Activate() error {
	if m.Status != MoleculeStatusPending {
		return errors.New(errors.ErrCodeStateChange, "molecule must be pending to activate")
	}
	if m.InChIKey == "" {
		return errors.New(errors.ErrCodeStateChange, "structure verification required (InChIKey missing)")
	}
	m.Status = MoleculeStatusActive
	m.UpdatedAt = time.Now().UTC()
	m.Version++
	return nil
}

// Archive transitions the molecule to Archived status.
func (m *Molecule) Archive() error {
	if m.Status != MoleculeStatusActive {
		return errors.New(errors.ErrCodeStateChange, "molecule must be active to archive")
	}
	m.Status = MoleculeStatusArchived
	m.UpdatedAt = time.Now().UTC()
	m.Version++
	return nil
}

// MarkDeleted transitions the molecule to Deleted status.
func (m *Molecule) MarkDeleted() error {
	if m.Status == MoleculeStatusDeleted {
		return errors.New(errors.ErrCodeStateChange, "molecule is already deleted")
	}
	if m.Status == MoleculeStatusPending {
		return errors.New(errors.ErrCodeStateChange, "cannot delete pending molecule")
	}
	m.Status = MoleculeStatusDeleted
	m.UpdatedAt = time.Now().UTC()
	now := time.Now().UTC()
	m.DeletedAt = &now
	m.Version++
	return nil
}

// SetStructureIdentifiers sets computed structure identifiers.
func (m *Molecule) SetStructureIdentifiers(canonicalSmiles, inchi, inchiKey, formula string, weight float64) error {
	if canonicalSmiles == "" {
		return errors.New(errors.ErrCodeInvalidInput, "canonical SMILES cannot be empty")
	}
	if inchi == "" {
		return errors.New(errors.ErrCodeInvalidInput, "InChI cannot be empty")
	}
	if !inchiKeyRegex.MatchString(inchiKey) {
		return errors.New(errors.ErrCodeInvalidInput, "invalid InChIKey format")
	}
	if weight < 0 {
		return errors.New(errors.ErrCodeInvalidInput, "molecular weight cannot be negative")
	}

	m.CanonicalSMILES = canonicalSmiles
	m.InChI = inchi
	m.InChIKey = inchiKey
	m.MolecularFormula = formula
	m.MolecularWeight = weight
	m.UpdatedAt = time.Now().UTC()
	m.Version++
	return nil
}

// AddFingerprint adds or updates a fingerprint.
func (m *Molecule) AddFingerprint(fp *Fingerprint) error {
	if fp == nil {
		return errors.New(errors.ErrCodeInvalidInput, "fingerprint cannot be nil")
	}
	if m.FingerprintsMap == nil {
		m.FingerprintsMap = make(map[FingerprintType]*Fingerprint)
	}
	m.FingerprintsMap[fp.Type] = fp

	// Sync with slice for legacy support?
	// The repo doesn't save from slice, it saves via specific method or logic.
	// But `FindByID` populates `Fingerprints` slice.
	// If domain logic uses Map, we should sync.
	// However, `fp.MoleculeID` should be set.
	fp.MoleculeID = m.ID

	// Add to slice if not present or update
	found := false
	for i, existing := range m.Fingerprints {
		if existing.Type == fp.Type {
			m.Fingerprints[i] = fp
			found = true
			break
		}
	}
	if !found {
		m.Fingerprints = append(m.Fingerprints, fp)
	}

	m.UpdatedAt = time.Now().UTC()
	m.Version++
	return nil
}

// GetFingerprint retrieves a fingerprint by type.
func (m *Molecule) GetFingerprint(fpType FingerprintType) (*Fingerprint, bool) {
	if m.FingerprintsMap != nil {
		fp, ok := m.FingerprintsMap[fpType]
		if ok {
			return fp, true
		}
	}
	// Fallback to slice
	for _, fp := range m.Fingerprints {
		if fp.Type == fpType {
			// Populate map cache
			if m.FingerprintsMap == nil { m.FingerprintsMap = make(map[FingerprintType]*Fingerprint) }
			m.FingerprintsMap[fpType] = fp
			return fp, true
		}
	}
	return nil, false
}

// HasFingerprint checks if a fingerprint exists.
func (m *Molecule) HasFingerprint(fpType FingerprintType) bool {
	_, ok := m.GetFingerprint(fpType)
	return ok
}

// AddProperty adds or updates a property.
func (m *Molecule) AddProperty(prop *MolecularProperty) error {
	if prop == nil {
		return errors.New(errors.ErrCodeInvalidInput, "property cannot be nil")
	}
	if prop.Name == "" {
		return errors.New(errors.ErrCodeInvalidInput, "property name cannot be empty")
	}
	if prop.Confidence < 0.0 || prop.Confidence > 1.0 {
		return errors.New(errors.ErrCodeInvalidInput, "confidence must be between 0.0 and 1.0")
	}

	if m.PropertiesMap == nil {
		m.PropertiesMap = make(map[string]*MolecularProperty)
	}
	m.PropertiesMap[prop.Name] = prop
	m.UpdatedAt = time.Now().UTC()
	m.Version++
	return nil
}

// GetProperty retrieves a property by name.
func (m *Molecule) GetProperty(name string) (*MolecularProperty, bool) {
	if m.PropertiesMap != nil {
		p, ok := m.PropertiesMap[name]
		return p, ok
	}
	return nil, false
}

// AddTag adds a tag.
func (m *Molecule) AddTag(tag string) error {
	if tag == "" {
		return errors.New(errors.ErrCodeInvalidInput, "tag cannot be empty")
	}
	if len(tag) > 64 {
		return errors.New(errors.ErrCodeInvalidInput, "tag exceeds maximum length")
	}

	for _, t := range m.Tags {
		if t == tag {
			return nil
		}
	}
	m.Tags = append(m.Tags, tag)
	m.UpdatedAt = time.Now().UTC()
	m.Version++
	return nil
}

// RemoveTag removes a tag.
func (m *Molecule) RemoveTag(tag string) {
	newTags := make([]string, 0, len(m.Tags))
	found := false
	for _, t := range m.Tags {
		if t != tag {
			newTags = append(newTags, t)
		} else {
			found = true
		}
	}
	if found {
		m.Tags = newTags
		m.UpdatedAt = time.Now().UTC()
		m.Version++
	}
}

// SetMetadata sets a metadata key-value pair.
func (m *Molecule) SetMetadata(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]any)
	}
	m.Metadata[key] = value
	m.UpdatedAt = time.Now().UTC()
	m.Version++
}

// Getters

func (m *Molecule) GetID() string { return m.ID.String() }
func (m *Molecule) GetSMILES() string { return m.SMILES }
func (m *Molecule) GetCanonicalSMILES() string { return m.CanonicalSMILES }
func (m *Molecule) GetInChI() string { return m.InChI }
func (m *Molecule) GetInChIKey() string { return m.InChIKey }
func (m *Molecule) GetMolecularFormula() string { return m.MolecularFormula }
func (m *Molecule) GetMolecularWeight() float64 { return m.MolecularWeight }
func (m *Molecule) GetSource() MoleculeSource { return m.Source }
func (m *Molecule) GetSourceRef() string { return m.SourceReference }
func (m *Molecule) GetTags() []string {
	result := make([]string, len(m.Tags))
	copy(result, m.Tags)
	return result
}
func (m *Molecule) GetStatus() MoleculeStatus { return m.Status }
func (m *Molecule) GetMetadata() map[string]any {
	result := make(map[string]any, len(m.Metadata))
	for k, v := range m.Metadata {
		result[k] = v
	}
	return result
}
func (m *Molecule) GetCreatedAt() time.Time { return m.CreatedAt }
func (m *Molecule) GetUpdatedAt() time.Time { return m.UpdatedAt }
func (m *Molecule) GetVersion() int64 { return m.Version }

// IsActive checks if the molecule is active.
func (m *Molecule) IsActive() bool {
	return m.Status == MoleculeStatusActive
}

// IsArchived checks if the molecule is archived.
func (m *Molecule) IsArchived() bool {
	return m.Status == MoleculeStatusArchived
}

// IsDeleted checks if the molecule is deleted.
func (m *Molecule) IsDeleted() bool {
	return m.Status == MoleculeStatusDeleted
}

// IsPending checks if the molecule is pending.
func (m *Molecule) IsPending() bool {
	return m.Status == MoleculeStatusPending
}

// String returns a string representation of the molecule.
func (m *Molecule) String() string {
	return fmt.Sprintf("Molecule{id=%s, smiles=%s, status=%s}", m.ID, m.SMILES, m.Status)
}

//Personal.AI order the ending
