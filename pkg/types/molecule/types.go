package molecule

import (
	"fmt"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MoleculeFormat defines the format of the molecule data.
type MoleculeFormat string

const (
	FormatSMILES  MoleculeFormat = "smiles"
	FormatInChI   MoleculeFormat = "inchi"
	FormatMolfile MoleculeFormat = "molfile"
	FormatSDF     MoleculeFormat = "sdf"
	FormatCDXML   MoleculeFormat = "cdxml"
)

// IsValid checks if the molecule format is valid.
func (f MoleculeFormat) IsValid() bool {
	switch f {
	case FormatSMILES, FormatInChI, FormatMolfile, FormatSDF, FormatCDXML:
		return true
	default:
		return false
	}
}

// FingerprintType defines the algorithm used for fingerprint generation.
type FingerprintType string

const (
	FingerprintMorgan   FingerprintType = "morgan"
	FingerprintMACCS    FingerprintType = "maccs"
	FingerprintRDKit    FingerprintType = "rdkit"
	FingerprintAtomPair FingerprintType = "atom_pair"
	FingerprintFCFP     FingerprintType = "fcfp"
	FingerprintGNN      FingerprintType = "gnn"
)

// IsValid checks if the fingerprint type is valid.
func (t FingerprintType) IsValid() bool {
	switch t {
	case FingerprintMorgan, FingerprintMACCS, FingerprintRDKit, FingerprintAtomPair, FingerprintFCFP, FingerprintGNN:
		return true
	default:
		return false
	}
}

// MoleculeDTO represents a molecule data transfer object.
type MoleculeDTO struct {
	ID                common.ID              `json:"id"`
	SMILES            string                 `json:"smiles"`
	InChI             string                 `json:"inchi"`
	InChIKey          string                 `json:"inchi_key"`
	MolecularFormula  string                 `json:"molecular_formula"`
	MolecularWeight   float64                `json:"molecular_weight"`
	LogP              float64                `json:"log_p,omitempty"`
	TPSA              float64                `json:"tpsa,omitempty"`
	NumHeavyAtoms     int                    `json:"num_heavy_atoms"`
	NumRotatableBonds int                    `json:"num_rotatable_bonds"`
	Name              string                 `json:"name,omitempty"`
	Source            string                 `json:"source"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt         common.Timestamp       `json:"created_at"`
	UpdatedAt         common.Timestamp       `json:"updated_at"`
}

// FingerprintDTO represents a molecular fingerprint.
type FingerprintDTO struct {
	Type    FingerprintType `json:"type"`
	Bits    []byte          `json:"bits"`
	Radius  int             `json:"radius,omitempty"`
	NBits   int             `json:"n_bits"`
	Version string          `json:"version"`
}

// StructuralDiff represents the structural differences between two molecules.
type StructuralDiff struct {
	CommonScaffold string            `json:"common_scaffold"`
	Differences    []SubstituentDiff `json:"differences"`
}

// SubstituentDiff represents a difference in substituents.
type SubstituentDiff struct {
	Position     string `json:"position"`
	QueryGroup   string `json:"query_group"`
	TargetGroup  string `json:"target_group"`
	Significance string `json:"significance"`
}

// SimilarityResult represents the result of a similarity search.
type SimilarityResult struct {
	TargetMolecule       MoleculeDTO                 `json:"target_molecule"`
	Scores               map[FingerprintType]float64 `json:"scores"`
	WeightedScore        float64                     `json:"weighted_score"`
	StructuralComparison *StructuralDiff             `json:"structural_comparison,omitempty"`
}

// MoleculeInput is the input structure for molecule operations.
type MoleculeInput struct {
	Format MoleculeFormat `json:"format"`
	Value  string         `json:"value"`
	Name   string         `json:"name,omitempty"`
}

// Validate checks if the molecule input is valid.
func (m MoleculeInput) Validate() error {
	if !m.Format.IsValid() {
		return fmt.Errorf("invalid molecule format: %s", m.Format)
	}
	if m.Value == "" {
		return fmt.Errorf("molecule value cannot be empty")
	}
	return nil
}

// SimilaritySearchRequest represents a request for similarity search.
type SimilaritySearchRequest struct {
	Molecule                MoleculeInput       `json:"molecule"`
	FingerprintTypes        []FingerprintType   `json:"fingerprint_types"`
	Threshold               float64             `json:"threshold"`
	MaxResults              int                 `json:"max_results"`
	PatentOffices           []string            `json:"patent_offices,omitempty"`
	DateRange               *common.DateRange   `json:"date_range,omitempty"`
	AssigneesFilter         []string            `json:"assignees_filter,omitempty"`
	TechDomains             []string            `json:"tech_domains,omitempty"`
	IncludeClaimAnalysis    bool                `json:"include_claim_analysis"`
	IncludeInfringementRisk bool                `json:"include_infringement_risk"`
	IncludeDesignAround     bool                `json:"include_design_around"`
}

// Validate checks if the similarity search request is valid.
func (r SimilaritySearchRequest) Validate() error {
	if err := r.Molecule.Validate(); err != nil {
		return err
	}
	if r.Threshold < 0.0 || r.Threshold > 1.0 {
		return fmt.Errorf("threshold must be between 0.0 and 1.0")
	}
	if r.MaxResults < 1 || r.MaxResults > 500 {
		return fmt.Errorf("max_results must be between 1 and 500")
	}
	if len(r.FingerprintTypes) == 0 {
		return fmt.Errorf("at least one fingerprint type must be specified")
	}
	for _, ft := range r.FingerprintTypes {
		if !ft.IsValid() {
			return fmt.Errorf("invalid fingerprint type: %s", ft)
		}
	}
	if r.DateRange != nil {
		if err := r.DateRange.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// SimilaritySearchResponse represents the response of a similarity search.
type SimilaritySearchResponse struct {
	QueryMolecule          MoleculeDTO       `json:"query_molecule"`
	Results                []SimilarityResult `json:"results"`
	TotalSearched          int64             `json:"total_searched"`
	TotalMoleculesCompared int64             `json:"total_molecules_compared"`
	SearchTimeMs           int64             `json:"search_time_ms"`
	ModelVersions          map[string]string `json:"model_versions"`
}

// MaterialProperty represents a physical or chemical property of a material.
type MaterialProperty struct {
	PropertyType  string  `json:"property_type"`
	Value         float64 `json:"value"`
	Unit          string  `json:"unit"`
	TestCondition string  `json:"test_condition,omitempty"`
	Source        string  `json:"source"`
}

// MoleculeSource defines the source of a molecule.
type MoleculeSource string

const (
	SourcePatent     MoleculeSource = "patent"
	SourceExperiment MoleculeSource = "experiment"
	SourceLiterature MoleculeSource = "literature"
	SourceUserInput  MoleculeSource = "user_input"
)

// SignificanceLevel defines the significance of a difference.
type SignificanceLevel string

const (
	SignificanceLow      SignificanceLevel = "low"
	SignificanceModerate SignificanceLevel = "moderate"
	SignificanceHigh     SignificanceLevel = "high"
)

// Default weights and thresholds.
const (
	DefaultMorganWeight       = 0.30
	DefaultRDKitWeight        = 0.20
	DefaultAtomPairWeight     = 0.15
	DefaultGNNWeight          = 0.35
	DefaultSimilarityThreshold = 0.65
	DefaultMaxResults         = 50
	MaxResultsLimit           = 500
)

//Personal.AI order the ending
