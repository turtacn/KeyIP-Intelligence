package patent

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// SubstituentType defines the type of a chemical substituent.
type SubstituentType uint8

const (
	SubstituentTypeUnknown    SubstituentType = 0
	SubstituentTypeAlkyl      SubstituentType = 1
	SubstituentTypeAryl       SubstituentType = 2
	SubstituentTypeHeteroaryl SubstituentType = 3
	SubstituentTypeHalogen    SubstituentType = 4
	SubstituentTypeAlkoxy     SubstituentType = 5
	SubstituentTypeAmino      SubstituentType = 6
	SubstituentTypeCyano      SubstituentType = 7
	SubstituentTypeHydrogen   SubstituentType = 8
	SubstituentTypeCustom     SubstituentType = 9
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

// Substituent represents a specific substituent definition.
type Substituent struct {
	ID          string          `json:"id"`
	Type        SubstituentType `json:"type"`
	Name        string          `json:"name"`
	SMILES      string          `json:"smiles,omitempty"`
	CarbonRange [2]int          `json:"carbon_range,omitempty"` // [min, max]
	Description string          `json:"description,omitempty"`
	IsPreferred bool            `json:"is_preferred"`
}

func (s *Substituent) Validate() error {
	if s.ID == "" {
		return errors.NewValidation("substituent ID cannot be empty")
	}
	if s.Name == "" {
		return errors.NewValidation("substituent name cannot be empty")
	}
	if !s.Type.IsValid() {
		return errors.NewValidation("invalid substituent type")
	}
	if s.CarbonRange[0] > s.CarbonRange[1] {
		return errors.NewValidation("invalid carbon range: min > max")
	}
	return nil
}

// VariablePosition represents a variable position in a Markush structure.
type VariablePosition struct {
	Symbol          string        `json:"symbol"` // e.g., R1, X
	Description     string        `json:"description,omitempty"`
	Substituents    []Substituent `json:"substituents"`
	IsOptional      bool          `json:"is_optional"`
	RepeatRange     [2]int        `json:"repeat_range,omitempty"`
	LinkedPositions []string      `json:"linked_positions,omitempty"`
}

func (vp *VariablePosition) SubstituentCount() int {
	return len(vp.Substituents)
}

func (vp *VariablePosition) Validate() error {
	if vp.Symbol == "" {
		return errors.NewValidation("variable position symbol cannot be empty")
	}
	if len(vp.Substituents) == 0 && !vp.IsOptional {
		return errors.NewValidation("non-optional variable position must have substituents")
	}
	for _, sub := range vp.Substituents {
		if err := sub.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// MarkushStructure represents a complete Markush structure.
type MarkushStructure struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	CoreStructure     string             `json:"core_structure"` // SMILES with placeholders
	CoreDescription   string             `json:"core_description,omitempty"`
	Positions         []VariablePosition `json:"positions"`
	Constraints       []string           `json:"constraints,omitempty"`
	PreferredExamples []string           `json:"preferred_examples,omitempty"`
	ClaimNumber       int                `json:"claim_number"`
	TotalCombinations int64              `json:"total_combinations"`
	CreatedAt         time.Time          `json:"created_at"`
}

// NewMarkushStructure creates a new MarkushStructure.
func NewMarkushStructure(name, coreStructure string, claimNumber int) (*MarkushStructure, error) {
	if name == "" {
		return nil, errors.NewValidation("name cannot be empty")
	}
	if coreStructure == "" {
		return nil, errors.NewValidation("core structure cannot be empty")
	}
	if claimNumber <= 0 {
		return nil, errors.NewValidation("claim number must be positive")
	}

	return &MarkushStructure{
		ID:            uuid.New().String(),
		Name:          name,
		CoreStructure: coreStructure,
		ClaimNumber:   claimNumber,
		Positions:     make([]VariablePosition, 0),
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// AddPosition adds a variable position to the Markush structure.
func (m *MarkushStructure) AddPosition(pos VariablePosition) error {
	for _, p := range m.Positions {
		if p.Symbol == pos.Symbol {
			return errors.NewValidation(fmt.Sprintf("duplicate position symbol: %s", pos.Symbol))
		}
	}
	m.Positions = append(m.Positions, pos)
	return nil
}

// AddConstraint adds a global constraint.
func (m *MarkushStructure) AddConstraint(constraint string) error {
	if constraint == "" {
		return errors.NewValidation("constraint cannot be empty")
	}
	m.Constraints = append(m.Constraints, constraint)
	return nil
}

// CalculateCombinations calculates the theoretical number of compounds covered.
func (m *MarkushStructure) CalculateCombinations() int64 {
	if len(m.Positions) == 0 {
		return 0
	}

	total := int64(1)
	for _, pos := range m.Positions {
		count := int64(len(pos.Substituents))
		if pos.IsOptional {
			count++
		}

		repeat := int64(1)
		if pos.RepeatRange[1] > 0 {
			// Range length: e.g. 0-3 => 4 possibilities of repetition?
			// Usually repeat range implies the group appears n times.
			// But if it's a polymer-like repeat, it's different.
			// Assuming RepeatRange defines how many times the group *can* appear.
			// Spec says: "multiply by range span (RepeatRange[1] - RepeatRange[0] + 1)"
			span := int64(pos.RepeatRange[1] - pos.RepeatRange[0] + 1)
			if span > 0 {
				repeat = span
			}
		}

		// Multiplication with overflow check
		nextTotal := total
		// Check for overflow before multiply: total * count * repeat

		// Simplify: multiplier = count * repeat
		multiplier := count * repeat

		if multiplier == 0 {
			continue // Should not happen if validated, but safety
		}

		if math.MaxInt64/multiplier < nextTotal {
			m.TotalCombinations = math.MaxInt64
			return math.MaxInt64
		}
		total = nextTotal * multiplier
	}
	m.TotalCombinations = total
	return total
}

// MarkushMatcher interface for dependency injection of chemical matching logic.
type MarkushMatcher interface {
	IsSubstructure(core, molecule string) (bool, error)
	ExtractSubstituents(core, molecule string) (map[string]string, error)
	MatchSubstituent(actual string, allowed []Substituent) (bool, string, error)
}

// MatchesMolecule checks if a molecule falls within the Markush structure scope.
// Modified signature to accept matcher injection.
func (m *MarkushStructure) MatchesMolecule(smiles string, matcher MarkushMatcher) (bool, float64, error) {
	if smiles == "" {
		return false, 0, errors.NewValidation("smiles cannot be empty")
	}

	// 1. Check PreferredExamples (Exact match)
	for _, ex := range m.PreferredExamples {
		if ex == smiles {
			return true, 1.0, nil
		}
	}

	if matcher == nil {
		// Without matcher, we can only check exact examples
		return false, 0, nil
	}

	// 2. Check Core Structure
	isSub, err := matcher.IsSubstructure(m.CoreStructure, smiles)
	if err != nil {
		return false, 0, err
	}
	if !isSub {
		return false, 0, nil
	}

	// 3. Extract Substituents
	extracted, err := matcher.ExtractSubstituents(m.CoreStructure, smiles)
	if err != nil {
		return false, 0, err
	}

	// 4. Validate Substituents
	matchedCount := 0
	totalCount := len(m.Positions)
	if totalCount == 0 {
		// If core matches and no variable positions, it's a match?
		// Usually Markush has variables. If none, core structure IS the structure.
		return true, 1.0, nil
	}

	for _, pos := range m.Positions {
		actual, ok := extracted[pos.Symbol]
		if !ok {
			if pos.IsOptional {
				matchedCount++
				continue
			}
			return false, 0, nil // Missing required position
		}

		matched, _, err := matcher.MatchSubstituent(actual, pos.Substituents)
		if err != nil {
			return false, 0, err
		}
		if matched {
			matchedCount++
		} else {
			return false, 0, nil
		}
	}

	confidence := float64(matchedCount) / float64(totalCount)
	return true, confidence, nil
}

// GetPosition returns the variable position with the given symbol.
func (m *MarkushStructure) GetPosition(symbol string) (*VariablePosition, bool) {
	for i := range m.Positions {
		if m.Positions[i].Symbol == symbol {
			return &m.Positions[i], true
		}
	}
	return nil, false
}

// PositionCount returns the number of variable positions.
func (m *MarkushStructure) PositionCount() int {
	return len(m.Positions)
}

// Validate checks consistency of the Markush structure.
func (m *MarkushStructure) Validate() error {
	if m.Name == "" {
		return errors.NewValidation("name cannot be empty")
	}
	if m.CoreStructure == "" {
		return errors.NewValidation("core structure cannot be empty")
	}
	if m.ClaimNumber <= 0 {
		return errors.NewValidation("claim number must be positive")
	}
	if len(m.Positions) == 0 {
		return errors.NewValidation("must have at least one variable position")
	}

	seenSymbols := make(map[string]bool)
	for _, pos := range m.Positions {
		if seenSymbols[pos.Symbol] {
			return errors.NewValidation(fmt.Sprintf("duplicate position symbol: %s", pos.Symbol))
		}
		seenSymbols[pos.Symbol] = true
		if err := pos.Validate(); err != nil {
			return err
		}
	}

	for _, pos := range m.Positions {
		for _, linked := range pos.LinkedPositions {
			if !seenSymbols[linked] {
				return errors.NewValidation(fmt.Sprintf("linked position %s not found", linked))
			}
		}
	}

	return nil
}

// MoleculeMatchResult contains detailed match information.
type MoleculeMatchResult struct {
	MarkushID          string            `json:"markush_id"`
	MoleculeSMILES     string            `json:"molecule_smiles"`
	IsMatch            bool              `json:"is_match"`
	Confidence         float64           `json:"confidence"`
	MatchedPositions   map[string]string `json:"matched_positions,omitempty"`
	UnmatchedPositions []string          `json:"unmatched_positions,omitempty"`
	ConstraintViolations []string        `json:"constraint_violations,omitempty"`
	MatchedAt          time.Time         `json:"matched_at"`
}

// MarkushCoverageAnalysis represents aggregated coverage analysis.
type MarkushCoverageAnalysis struct {
	MarkushID         string         `json:"markush_id"`
	TotalCombinations int64          `json:"total_combinations"`
	SampledMolecules  int            `json:"sampled_molecules"`
	MatchedMolecules  int            `json:"matched_molecules"`
	CoverageRate      float64        `json:"coverage_rate"`
	PositionDiversity map[string]int `json:"position_diversity,omitempty"`
	AnalyzedAt        time.Time      `json:"analyzed_at"`
}

// ParseMarkushFromText attempts to extract Markush structure from text.
// This is a simplified implementation.
func ParseMarkushFromText(text string) (*MarkushStructure, error) {
	if text == "" {
		return nil, errors.NewValidation("text cannot be empty")
	}

	// Heuristic 1: Look for "formula (I)" or similar
	reFormula := regexp.MustCompile(`(?i)(formula|structure)\s*\(?([IVX]+|[A-Z0-9]+)\)?`)
	match := reFormula.FindStringSubmatch(text)
	if match == nil {
		return nil, errors.NewValidation("no markush structure identifier found")
	}
	name := fmt.Sprintf("Formula %s", match[2])

	// Heuristic 2: Look for variable definitions "R1 is..."
	// Simplified regex
	reVar := regexp.MustCompile(`(R\d+|Ar\d+|X|Y|Z)\s+(?:is|represents|selected from)\s+([^;.]+)`)
	matches := reVar.FindAllStringSubmatch(text, -1)

	if len(matches) == 0 {
		return nil, errors.NewValidation("no variable definitions found")
	}

	ms, _ := NewMarkushStructure(name, "[Core]", 1) // Dummy core and claim number

	for _, m := range matches {
		symbol := m[1]
		desc := m[2]

		// Split description by comma or "or" to get substituents
		// This is very rough
		parts := strings.FieldsFunc(desc, func(r rune) bool {
			return r == ',' || r == ';'
		})

		subs := []Substituent{}
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// Attempt to classify
			subType := SubstituentTypeCustom
			lowerP := strings.ToLower(p)
			if strings.Contains(lowerP, "alkyl") {
				subType = SubstituentTypeAlkyl
			} else if strings.Contains(lowerP, "aryl") || strings.Contains(lowerP, "phenyl") {
				subType = SubstituentTypeAryl
			} else if strings.Contains(lowerP, "hydrogen") {
				subType = SubstituentTypeHydrogen
			}

			subs = append(subs, Substituent{
				ID:   fmt.Sprintf("%s-%d", symbol, i),
				Type: subType,
				Name: p,
			})
		}

		ms.AddPosition(VariablePosition{
			Symbol:       symbol,
			Substituents: subs,
		})
	}

	return ms, nil
}

//Personal.AI order the ending
