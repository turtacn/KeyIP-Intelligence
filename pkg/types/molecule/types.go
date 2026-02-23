package molecule

import (
	"fmt"

	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MoleculeID is a string alias for a molecule identifier.
type MoleculeID string

// MoleculeType categorises a molecule by its primary chemical function.
type MoleculeType string

const (
	TypeSmallMolecule MoleculeType = "small_molecule"
	TypePolymer       MoleculeType = "polymer"
	TypeOLEDMaterial  MoleculeType = "oled_material"
	TypeCatalyst      MoleculeType = "catalyst"
	TypeIntermediate  MoleculeType = "intermediate"
)

// MolecularProperties holds computed physicochemical descriptors for a molecule.
type MolecularProperties struct {
	LogP           float64  `json:"log_p"`
	TPSA           float64  `json:"tpsa"`
	HBondDonors    int      `json:"h_bond_donors"`
	HBondAcceptors int      `json:"h_bond_acceptors"`
	RotatableBonds int      `json:"rotatable_bonds"`
	AromaticRings  int      `json:"aromatic_rings"`
	HOMO           *float64 `json:"homo,omitempty"`
	LUMO           *float64 `json:"lumo,omitempty"`
	BandGap        *float64 `json:"band_gap,omitempty"`
}

// MoleculeFormat defines the input format for a molecule.
type MoleculeFormat string

const (
	FormatSMILES  MoleculeFormat = "smiles"
	FormatInChI   MoleculeFormat = "inchi"
	FormatMolfile MoleculeFormat = "molfile"
	FormatSDF     MoleculeFormat = "sdf"
	FormatCDXML   MoleculeFormat = "cdxml"
)

// IsValid checks if the MoleculeFormat is supported.
func (f MoleculeFormat) IsValid() bool {
	switch f {
	case FormatSMILES, FormatInChI, FormatMolfile, FormatSDF, FormatCDXML:
		return true
	default:
		return false
	}
}

// FingerprintType defines the algorithm used for molecular fingerprinting.
type FingerprintType string

const (
	FingerprintMorgan   FingerprintType = "morgan"
	FingerprintMACCS    FingerprintType = "maccs"
	FingerprintRDKit    FingerprintType = "rdkit"
	FingerprintAtomPair FingerprintType = "atom_pair"
	FingerprintFCFP     FingerprintType = "fcfp"
	FingerprintGNN      FingerprintType = "gnn"
)

// Aliases for backward compatibility
const (
	FPMorgan      = FingerprintMorgan
	FPMACCS       = FingerprintMACCS
	FPTopological = FingerprintRDKit
	FPAtomPair    = FingerprintAtomPair
)

// IsValid checks if the FingerprintType is supported.
func (f FingerprintType) IsValid() bool {
	switch f {
	case FingerprintMorgan, FingerprintMACCS, FingerprintRDKit, FingerprintAtomPair, FingerprintFCFP, FingerprintGNN:
		return true
	default:
		return false
	}
}

// MoleculeDTO represents a molecule with its properties and metadata.
type MoleculeDTO struct {
	common.BaseEntity
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
	Synonyms          []string               `json:"synonyms,omitempty"`
	Type              MoleculeType           `json:"type"`
	Source            string                 `json:"source"` // patent / experiment / literature / user_input
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Properties        MolecularProperties    `json:"properties"`
	Fingerprints      map[FingerprintType][]byte `json:"fingerprints,omitempty"`
	SourcePatentIDs   []common.ID            `json:"source_patent_ids,omitempty"`
}

// FingerprintDTO represents a molecular fingerprint.
type FingerprintDTO struct {
	Type   FingerprintType `json:"type"`
	Bits   []byte          `json:"bits"`
	Radius int             `json:"radius,omitempty"` // for Morgan
	NBits  int             `json:"nbits"`
	Version string         `json:"version"`
}

// SimilarityResult represents a result from a similarity search.
type SimilarityResult struct {
	TargetMolecule       MoleculeDTO                `json:"target_molecule"`
	Scores               map[FingerprintType]float64 `json:"scores"`
	WeightedScore        float64                    `json:"weighted_score"`
	StructuralComparison *StructuralDiff            `json:"structural_comparison,omitempty"`
}

// StructuralDiff represents structural differences between two molecules.
type StructuralDiff struct {
	CommonScaffold string            `json:"common_scaffold"`
	Differences    []SubstituentDiff `json:"differences"`
}

// SubstituentDiff represents a difference in a substituent.
type SubstituentDiff struct {
	Position    string `json:"position"`
	QueryGroup  string `json:"query_group"`
	TargetGroup string `json:"target_group"`
	Significance string `json:"significance"` // low / moderate / high
}

// MoleculeInput represents a molecule provided as input.
type MoleculeInput struct {
	Format MoleculeFormat `json:"format"`
	Value  string         `json:"value"`
	Name   string         `json:"name,omitempty"`
}

// Validate checks if the MoleculeInput is valid.
func (i MoleculeInput) Validate() error {
	if !i.Format.IsValid() {
		return fmt.Errorf("invalid molecule format: %s", i.Format)
	}
	if i.Value == "" {
		return fmt.Errorf("molecule value cannot be empty")
	}
	return nil
}

// SimilaritySearchRequest carries parameters for a molecular similarity search.
type SimilaritySearchRequest struct {
	Molecule                  MoleculeInput          `json:"molecule"`
	FingerprintTypes          []FingerprintType      `json:"fingerprint_types"`
	Threshold                 float64                `json:"threshold"`
	MaxResults                int                    `json:"max_results"`
	PatentOffices             []string               `json:"patent_offices,omitempty"`
	DateRange                 *common.DateRange      `json:"date_range,omitempty"`
	AssigneesFilter           []string               `json:"assignees_filter,omitempty"`
	TechDomains               []string               `json:"tech_domains,omitempty"`
	IncludeClaimAnalysis      bool                   `json:"include_claim_analysis"`
	IncludeInfringementRisk   bool                   `json:"include_infringement_risk"`
	IncludeDesignAround       bool                   `json:"include_design_around"`
}

// Validate checks if the SimilaritySearchRequest is valid.
func (r SimilaritySearchRequest) Validate() error {
	if err := r.Molecule.Validate(); err != nil {
		return err
	}
	if len(r.FingerprintTypes) == 0 {
		return fmt.Errorf("at least one fingerprint type must be specified")
	}
	for _, ft := range r.FingerprintTypes {
		if !ft.IsValid() {
			return fmt.Errorf("invalid fingerprint type: %s", ft)
		}
	}
	if r.Threshold < 0.0 || r.Threshold > 1.0 {
		return fmt.Errorf("threshold must be between 0.0 and 1.0")
	}
	if r.MaxResults < 1 || r.MaxResults > 500 {
		return fmt.Errorf("max_results must be between 1 and 500")
	}
	return nil
}

// MoleculeSearchRequest is an alias for SimilaritySearchRequest for backward compatibility.
type MoleculeSearchRequest = SimilaritySearchRequest

// MoleculeSearchResponse is a generic wrapper for molecule search results.
type MoleculeSearchResponse = common.PageResponse[MoleculeDTO]

// SimilaritySearchResponse represents the result of a similarity search.
type SimilaritySearchResponse struct {
	QueryMolecule           MoleculeDTO       `json:"query_molecule"`
	Results                 []SimilarityResult `json:"results"`
	TotalSearched           int64             `json:"total_searched"`
	TotalMoleculesCompared  int64             `json:"total_molecules_compared"`
	SearchTimeMs            int64             `json:"search_time_ms"`
	ModelVersions           map[string]string `json:"model_versions"`
}

// MaterialProperty represents a physical or chemical property of a material.
type MaterialProperty struct {
	PropertyType  string  `json:"property_type"`
	Value         float64 `json:"value"`
	Unit          string  `json:"unit"`
	TestCondition string  `json:"test_condition,omitempty"`
	Source        string  `json:"source"`
}

// MoleculeSource enums.
const (
	SourcePatent     = "patent"
	SourceExperiment = "experiment"
	SourceLiterature = "literature"
	SourceUserInput  = "user_input"
)

// SignificanceLevel enums.
const (
	SignificanceLow      = "low"
	SignificanceModerate = "moderate"
	SignificanceHigh     = "high"
)

// Default weights for similarity calculations.
const (
	DefaultMorganWeight      = 0.30
	DefaultRDKitWeight       = 0.20
	DefaultAtomPairWeight    = 0.15
	DefaultGNNWeight         = 0.35
	DefaultSimilarityThreshold = 0.65
	DefaultMaxResults        = 50
	MaxResultsLimit          = 500
)

// SimilarityLevel classifies a similarity score.
type SimilarityLevel string

const (
	SimilarityHigh   SimilarityLevel = "HIGH"
	SimilarityMedium SimilarityLevel = "MEDIUM"
	SimilarityLow    SimilarityLevel = "LOW"
	SimilarityNone   SimilarityLevel = "NONE"
)

//Personal.AI order the ending
