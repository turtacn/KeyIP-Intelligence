package patent

import (
	"fmt"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// RGroup value object
// ─────────────────────────────────────────────────────────────────────────────

// RGroup describes a single variable substituent position in a Markush structure.
// Each position (e.g., "R1", "R2") may be occupied by any one of its
// Alternatives, giving rise to a large virtual combinatorial library.
type RGroup struct {
	// Position is the label used in the CoreStructure SMILES to mark the
	// variable attachment point, e.g., "R1", "R2", "*".
	Position string

	// Alternatives is the list of SMILES strings that may substitute for
	// Position in the core scaffold.  Must contain at least one entry.
	Alternatives []string

	// Description is an optional human-readable explanation of this R-group
	// as it appears in the patent claim or specification.
	Description string
}

// ─────────────────────────────────────────────────────────────────────────────
// Markush value object
// ─────────────────────────────────────────────────────────────────────────────

// Markush is a value object representing a Markush structure extracted from a
// chemical patent claim.  A Markush structure encodes a combinatorial library
// of compounds via a fixed core scaffold and one or more R-group positions.
//
// A single Markush claim can implicitly cover millions to billions of specific
// molecules, making Markush analysis the central challenge of chemical patent
// FTO work.
type Markush struct {
	// ID is the platform-internal unique identifier for this Markush record.
	ID common.ID

	// PatentID links this Markush structure to its parent patent aggregate.
	PatentID common.ID

	// ClaimID links this Markush structure to the specific claim in which it
	// appears.
	ClaimID common.ID

	// CoreStructure is the SMILES representation of the invariant scaffold.
	// Variable positions are denoted by the Position labels of the RGroups
	// (e.g., "[R1]", "*").
	CoreStructure string

	// RGroups lists all variable substituent positions and their alternatives.
	RGroups []RGroup

	// Description is a free-text summary of the Markush structure drawn from
	// the patent specification.
	Description string

	// EnumeratedCount caches the computed cardinality of the virtual library
	// (product of len(alternatives) for all R-groups).  Updated by
	// CalculateEnumeratedCount.
	EnumeratedCount int64
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewMarkush constructs and validates a Markush value object.
//
// Validation rules:
//   - patentID and claimID must be non-empty
//   - coreStructure must be a non-empty string that passes basic SMILES format
//     checks (contains at least one element symbol or ring atom)
//   - rGroups must contain at least one entry
//   - each RGroup must have a non-empty Position and at least one Alternative
func NewMarkush(
	patentID, claimID common.ID,
	coreStructure string,
	rGroups []RGroup,
) (*Markush, error) {
	if strings.TrimSpace(string(patentID)) == "" {
		return nil, errors.InvalidParam("patentID must not be empty")
	}
	if strings.TrimSpace(string(claimID)) == "" {
		return nil, errors.InvalidParam("claimID must not be empty")
	}

	core := strings.TrimSpace(coreStructure)
	if core == "" {
		return nil, errors.InvalidParam("Markush core structure (SMILES) must not be empty")
	}
	if err := validateSMILESBasic(core); err != nil {
		return nil, errors.InvalidParam("Markush core structure is not a valid SMILES").
			WithDetail(err.Error()).
			WithCause(err)
	}

	if len(rGroups) == 0 {
		return nil, errors.InvalidParam("Markush structure must have at least one R-group")
	}

	for i, rg := range rGroups {
		if strings.TrimSpace(rg.Position) == "" {
			return nil, errors.InvalidParam(
				fmt.Sprintf("R-group at index %d must have a non-empty Position", i))
		}
		if len(rg.Alternatives) == 0 {
			return nil, errors.InvalidParam(
				fmt.Sprintf("R-group %q must have at least one alternative", rg.Position))
		}
	}

	m := &Markush{
		ID:            common.NewID(),
		PatentID:      patentID,
		ClaimID:       claimID,
		CoreStructure: core,
		RGroups:       rGroups,
	}
	m.EnumeratedCount = m.CalculateEnumeratedCount()
	return m, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Domain methods
// ─────────────────────────────────────────────────────────────────────────────

// CalculateEnumeratedCount computes the total number of distinct compounds that
// can be generated from this Markush structure by taking the Cartesian product
// of all R-group alternative sets.
//
//	count = ∏ len(rg.Alternatives)  for all rg in RGroups
//
// Returns 0 if there are no R-groups (should not occur after successful
// construction via NewMarkush).
func (m *Markush) CalculateEnumeratedCount() int64 {
	if len(m.RGroups) == 0 {
		return 0
	}
	var count int64 = 1
	for _, rg := range m.RGroups {
		count *= int64(len(rg.Alternatives))
	}
	m.EnumeratedCount = count
	return count
}

// EnumerateExemplary generates up to maxCount representative SMILES by
// iterating through the Cartesian product of R-group alternatives in
// lexicographic order.  Each exemplary compound is produced by substituting
// the Position placeholder in the CoreStructure with a specific alternative.
//
// The substitution strategy is a simple string replacement:
//
//	strings.ReplaceAll(core, "["+position+"]", alternative)
//
// For production use the MolPatentGNN service performs proper SMILES assembly;
// this method is intended for quick enumeration in tests and UI previews.
//
// Returns at most maxCount compounds; if maxCount ≤ 0 it defaults to 10.
func (m *Markush) EnumerateExemplary(maxCount int) []string {
	if maxCount <= 0 {
		maxCount = 10
	}

	results := make([]string, 0, maxCount)
	m.enumerateRecursive(m.CoreStructure, m.RGroups, &results, maxCount)
	return results
}

// enumerateRecursive is the recursive helper for EnumerateExemplary.
func (m *Markush) enumerateRecursive(current string, remaining []RGroup, results *[]string, max int) {
	if len(*results) >= max {
		return
	}
	if len(remaining) == 0 {
		*results = append(*results, current)
		return
	}

	rg := remaining[0]
	rest := remaining[1:]
	placeholder := "[" + rg.Position + "]"

	for _, alt := range rg.Alternatives {
		if len(*results) >= max {
			return
		}
		substituted := strings.ReplaceAll(current, placeholder, alt)
		m.enumerateRecursive(substituted, rest, results, max)
	}
}

// ContainsMolecule performs a simplified structural membership check: it
// reports true if the given SMILES string contains the core scaffold of this
// Markush structure as a substring (case-insensitive, after stripping R-group
// placeholders from the core).
//
// IMPORTANT: This is a heuristic approximation.  Rigorous Markush membership
// testing requires full subgraph isomorphism matching performed by the
// MolPatentGNN service via its MarkushCoverageQuery RPC.
func (m *Markush) ContainsMolecule(smiles string) bool {
	if strings.TrimSpace(smiles) == "" {
		return false
	}

	// Strip R-group placeholders from the core to obtain the invariant scaffold.
	scaffold := m.CoreStructure
	for _, rg := range m.RGroups {
		scaffold = strings.ReplaceAll(scaffold, "["+rg.Position+"]", "")
	}
	scaffold = strings.TrimSpace(scaffold)
	if scaffold == "" {
		return false
	}

	return strings.Contains(
		strings.ToLower(smiles),
		strings.ToLower(scaffold),
	)
}

// ToDTO converts the Markush value object to the transport-layer MarkushDTO.
func (m *Markush) ToDTO() ptypes.MarkushDTO {
	rGroupDTOs := make([]ptypes.RGroupDTO, 0, len(m.RGroups))
	for _, rg := range m.RGroups {
		rGroupDTOs = append(rGroupDTOs, ptypes.RGroupDTO{
			Position:     rg.Position,
			Alternatives: rg.Alternatives,
			Description:  rg.Description,
		})
	}

	return ptypes.MarkushDTO{
		ID:              m.ID,
		PatentID:        m.PatentID,
		ClaimID:         m.ClaimID,
		CoreStructure:   m.CoreStructure,
		RGroups:         rGroupDTOs,
		Description:     m.Description,
		EnumeratedCount: m.EnumeratedCount,
	}
}

// MarkushFromDTO reconstructs a Markush value object from its DTO.
func MarkushFromDTO(dto ptypes.MarkushDTO) Markush {
	rGroups := make([]RGroup, len(dto.RGroups))
	for i, rg := range dto.RGroups {
		rGroups[i] = RGroup{
			Position:     rg.Position,
			Alternatives: rg.Alternatives,
			Description:  rg.Description,
		}
	}

	return Markush{
		ID:              dto.ID,
		PatentID:        dto.PatentID,
		ClaimID:         dto.ClaimID,
		CoreStructure:   dto.CoreStructure,
		RGroups:         rGroups,
		Description:     dto.Description,
		EnumeratedCount: dto.EnumeratedCount,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// validateSMILESBasic performs a lightweight syntactic check on a SMILES string.
// It verifies that:
//   - the string is non-empty after trimming
//   - balanced parentheses exist
//   - balanced square brackets exist
//   - the string contains at least one element character (letter or digit)
//
// Full SMILES validation (valence, ring closure, aromaticity) is delegated to
// RDKit in the cheminformatics service layer.
func validateSMILESBasic(smiles string) error {
	if smiles == "" {
		return fmt.Errorf("SMILES must not be empty")
	}

	parenDepth := 0
	bracketDepth := 0
	hasAtom := false

	for _, ch := range smiles {
		switch ch {
		case '(':
			parenDepth++
		case ')':
			parenDepth--
			if parenDepth < 0 {
				return fmt.Errorf("unbalanced parentheses in SMILES: %q", smiles)
			}
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return fmt.Errorf("unbalanced square brackets in SMILES: %q", smiles)
			}
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			hasAtom = true
		}
	}

	if parenDepth != 0 {
		return fmt.Errorf("unbalanced parentheses in SMILES: %q", smiles)
	}
	if bracketDepth != 0 {
		return fmt.Errorf("unbalanced square brackets in SMILES: %q", smiles)
	}
	if !hasAtom {
		return fmt.Errorf("SMILES contains no atom symbols: %q", smiles)
	}
	return nil
}

