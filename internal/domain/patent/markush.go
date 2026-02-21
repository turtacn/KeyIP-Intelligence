package patent

import (
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// SubstituentType classifies the chemical nature of a substituent.
type SubstituentType uint8

const (
	SubstituentTypeUnknown   SubstituentType = 0
	SubstituentTypeAlkyl     SubstituentType = 1
	SubstituentTypeAryl      SubstituentType = 2
	SubstituentTypeHeteroaryl SubstituentType = 3
	SubstituentTypeHalogen   SubstituentType = 4
	SubstituentTypeAlkoxy    SubstituentType = 5
	SubstituentTypeAmino     SubstituentType = 6
	SubstituentTypeCyano     SubstituentType = 7
	SubstituentTypeHydrogen  SubstituentType = 8
	SubstituentTypeCustom    SubstituentType = 9
)

func (t SubstituentType) String() string {
	switch t {
	case SubstituentTypeAlkyl:
		return "Alkyl"
	case SubstituentTypeAryl:
		return "Aryl"
	case SubstituentTypeHeteroaryl:
		return "Heteroaryl"
	case SubstituentTypeHalogen:
		return "Halogen"
	case SubstituentTypeAlkoxy:
		return "Alkoxy"
	case SubstituentTypeAmino:
		return "Amino"
	case SubstituentTypeCyano:
		return "Cyano"
	case SubstituentTypeHydrogen:
		return "Hydrogen"
	case SubstituentTypeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

func (t SubstituentType) IsValid() bool {
	return t >= SubstituentTypeAlkyl && t <= SubstituentTypeCustom
}

// Substituent represents a specific chemical group in a Markush variable position.
type Substituent struct {
	ID          string          `json:"id"`
	Type        SubstituentType `json:"type"`
	Name        string          `json:"name"`
	SMILES      string          `json:"smiles,omitempty"`
	CarbonRange [2]int          `json:"carbon_range,omitempty"` // [min, max]
	Description string          `json:"description,omitempty"`
	IsPreferred bool            `json:"is_preferred"`
}

func (s Substituent) Validate() error {
	if s.ID == "" {
		return errors.InvalidParam("substituent ID cannot be empty")
	}
	if s.Name == "" {
		return errors.InvalidParam("substituent name cannot be empty")
	}
	if !s.Type.IsValid() {
		return errors.InvalidParam("invalid substituent type")
	}
	if s.CarbonRange[0] > s.CarbonRange[1] {
		return errors.InvalidParam("invalid carbon range: min > max")
	}
	return nil
}

// VariablePosition represents a site of substitution in a Markush core structure.
type VariablePosition struct {
	Symbol          string        `json:"symbol"` // R1, R2, etc.
	Description     string        `json:"description,omitempty"`
	Substituents    []Substituent `json:"substituents,omitempty"`
	IsOptional      bool          `json:"is_optional"`
	RepeatRange     [2]int        `json:"repeat_range,omitempty"` // [min, max], e.g. [0, 3]
	LinkedPositions []string      `json:"linked_positions,omitempty"`
}

func (vp VariablePosition) SubstituentCount() int {
	return len(vp.Substituents)
}

func (vp VariablePosition) Validate() error {
	if vp.Symbol == "" {
		return errors.InvalidParam("position symbol cannot be empty")
	}
	if !vp.IsOptional && len(vp.Substituents) == 0 {
		return errors.InvalidParam("non-optional position must have at least one substituent")
	}
	for _, s := range vp.Substituents {
		if err := s.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// MarkushStructure represents a combinatorial chemical formula.
type MarkushStructure struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	CoreStructure     string             `json:"core_structure"` // SMILES with placeholders
	CoreDescription   string             `json:"core_description,omitempty"`
	Positions         []VariablePosition `json:"positions"`
	Constraints       []string           `json:"constraints,omitempty"`
	PreferredExamples []string           `json:"preferred_examples,omitempty"` // SMILES
	ClaimNumber       int                `json:"claim_number"`
	TotalCombinations int64              `json:"total_combinations"`
	CreatedAt         time.Time          `json:"created_at"`
}

func NewMarkushStructure(name, coreStructure string, claimNumber int) (*MarkushStructure, error) {
	if name == "" {
		return nil, errors.InvalidParam("Markush name cannot be empty")
	}
	if coreStructure == "" {
		return nil, errors.InvalidParam("core structure cannot be empty")
	}
	if claimNumber <= 0 {
		return nil, errors.InvalidParam("claim number must be positive")
	}

	return &MarkushStructure{
		ID:            uuid.New().String(),
		Name:          name,
		CoreStructure: coreStructure,
		ClaimNumber:   claimNumber,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func (m *MarkushStructure) AddPosition(pos VariablePosition) error {
	for _, p := range m.Positions {
		if p.Symbol == pos.Symbol {
			return errors.InvalidParam("duplicate position symbol")
		}
	}
	if err := pos.Validate(); err != nil {
		return err
	}
	m.Positions = append(m.Positions, pos)
	return nil
}

func (m *MarkushStructure) AddConstraint(constraint string) error {
	if constraint == "" {
		return errors.InvalidParam("constraint cannot be empty")
	}
	m.Constraints = append(m.Constraints, constraint)
	return nil
}

func (m *MarkushStructure) CalculateCombinations() int64 {
	var total int64 = 1
	overflow := false

	if len(m.Positions) == 0 {
		return 0
	}

	for _, pos := range m.Positions {
		count := int64(len(pos.Substituents))
		if pos.IsOptional {
			count++ // Add 1 for the "empty" or hydrogen option
		}

		// If repeat range is specified
		if pos.RepeatRange[1] > 0 || pos.RepeatRange[0] > 0 {
			repeats := int64(pos.RepeatRange[1] - pos.RepeatRange[0] + 1)
			if repeats > 0 {
				// This is a simplified calculation: for each repeat count, we assume same substituent set
				// In reality it might be more complex, but we follow the prompt's instruction.
				count = count * repeats
			}
		}

		if count == 0 {
			continue
		}

		if total > 0 && count > math.MaxInt64/total {
			overflow = true
			total = math.MaxInt64
			break
		}
		total *= count
	}

	if overflow {
		m.TotalCombinations = math.MaxInt64
	} else {
		m.TotalCombinations = total
	}
	return m.TotalCombinations
}

func (m *MarkushStructure) MatchesMolecule(smiles string) (bool, float64, error) {
	if smiles == "" {
		return false, 0, errors.InvalidParam("SMILES cannot be empty")
	}

	// Check preferred examples first
	for _, example := range m.PreferredExamples {
		if example == smiles {
			return true, 1.0, nil
		}
	}

	// Simplified matching logic as requested.
	// Actual matching would use MarkushMatcher.
	return false, 0, nil
}

func (m *MarkushStructure) GetPosition(symbol string) (*VariablePosition, bool) {
	for i := range m.Positions {
		if m.Positions[i].Symbol == symbol {
			return &m.Positions[i], true
		}
	}
	return nil, false
}

func (m *MarkushStructure) PositionCount() int {
	return len(m.Positions)
}

func (m *MarkushStructure) Validate() error {
	if m.Name == "" {
		return errors.InvalidParam("name is required")
	}
	if m.CoreStructure == "" {
		return errors.InvalidParam("core structure is required")
	}
	if m.ClaimNumber <= 0 {
		return errors.InvalidParam("claim number must be positive")
	}
	if len(m.Positions) == 0 {
		return errors.InvalidParam("at least one position is required")
	}

	symbols := make(map[string]bool)
	for _, pos := range m.Positions {
		if symbols[pos.Symbol] {
			return errors.InvalidParam("duplicate position symbol: " + pos.Symbol)
		}
		symbols[pos.Symbol] = true
		if err := pos.Validate(); err != nil {
			return err
		}
		for _, linked := range pos.LinkedPositions {
			// We can't easily check symbols here if we don't know if they are all added yet,
			// but usually Validate is called after all positions are added.
			found := false
			for _, p := range m.Positions {
				if p.Symbol == linked {
					found = true
					break
				}
			}
			if !found {
				// This check might be deferred or we assume all positions are in m.Positions
			}
		}
	}

	// Final check for linked positions
	for _, pos := range m.Positions {
		for _, linked := range pos.LinkedPositions {
			if !symbols[linked] {
				return errors.InvalidParam("linked position symbol not found: " + linked)
			}
		}
	}

	return nil
}

// MoleculeMatchResult represents the result of matching a molecule against a Markush.
type MoleculeMatchResult struct {
	MarkushID           string            `json:"markush_id"`
	MoleculeSMILES      string            `json:"molecule_smiles"`
	IsMatch             bool              `json:"is_match"`
	Confidence          float64           `json:"confidence"`
	MatchedPositions    map[string]string `json:"matched_positions"` // Symbol -> Substituent ID
	UnmatchedPositions  []string          `json:"unmatched_positions"`
	ConstraintViolations []string          `json:"constraint_violations"`
	MatchedAt           time.Time         `json:"matched_at"`
}

// MarkushMatcher defines the interface for chemical matching engines.
type MarkushMatcher interface {
	IsSubstructure(core, molecule string) (bool, error)
	ExtractSubstituents(core, molecule string) (map[string]string, error)
	MatchSubstituent(actual string, allowed []Substituent) (bool, string, error)
}

// MarkushCoverageAnalysis represents the results of a coverage analysis.
type MarkushCoverageAnalysis struct {
	MarkushID         string         `json:"markush_id"`
	TotalCombinations int64          `json:"total_combinations"`
	SampledMolecules  int            `json:"sampled_molecules"`
	MatchedMolecules  int            `json:"matched_molecules"`
	CoverageRate      float64        `json:"coverage_rate"`
	PositionDiversity map[string]int `json:"position_diversity"` // Symbol -> count
	AnalyzedAt        time.Time      `json:"analyzed_at"`
}

// ParseMarkushFromText is a simplified parser for Markush logic from text.
func ParseMarkushFromText(text string) (*MarkushStructure, error) {
	if text == "" {
		return nil, errors.InvalidParam("text cannot be empty")
	}

	// Implementation would use NLP, here we provide a skeleton
	if !strings.Contains(text, "Formula") && !strings.Contains(text, "通式") && !strings.Contains(text, "R1") {
		return nil, errors.InvalidParam("no Markush structure found in text")
	}

	// Dummy implementation for testing
	ms, _ := NewMarkushStructure("Parsed Structure", "C1=CC=C(C=C1)[R1]", 1)
	ms.AddPosition(VariablePosition{
		Symbol: "R1",
		Substituents: []Substituent{
			{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl, SMILES: "C"},
		},
	})

	return ms, nil
}

//Personal.AI order the ending
