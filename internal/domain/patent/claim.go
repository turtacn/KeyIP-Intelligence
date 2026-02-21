package patent

import (
	"fmt"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ClaimType defines whether a claim is independent or dependent.
type ClaimType uint8

const (
	ClaimTypeUnknown     ClaimType = 0
	ClaimTypeIndependent ClaimType = 1
	ClaimTypeDependent   ClaimType = 2
)

func (t ClaimType) String() string {
	switch t {
	case ClaimTypeIndependent:
		return "Independent"
	case ClaimTypeDependent:
		return "Dependent"
	default:
		return "Unknown"
	}
}

func (t ClaimType) IsValid() bool {
	return t == ClaimTypeIndependent || t == ClaimTypeDependent
}

// ClaimCategory classifies the subject matter of a claim.
type ClaimCategory uint8

const (
	ClaimCategoryUnknown ClaimCategory = 0
	ClaimCategoryProduct ClaimCategory = 1 // Compounds, compositions, devices
	ClaimCategoryMethod  ClaimCategory = 2 // Manufacturing, methods of use
	ClaimCategoryUse     ClaimCategory = 3 // Specific uses
)

func (c ClaimCategory) String() string {
	switch c {
	case ClaimCategoryProduct:
		return "Product"
	case ClaimCategoryMethod:
		return "Method"
	case ClaimCategoryUse:
		return "Use"
	default:
		return "Unknown"
	}
}

func (c ClaimCategory) IsValid() bool {
	return c >= ClaimCategoryProduct && c <= ClaimCategoryUse
}

// ClaimElementType classifies the nature of a technical feature.
type ClaimElementType uint8

const (
	ClaimElementTypeUnknown ClaimElementType = 0
	StructuralElement       ClaimElementType = 1
	FunctionalElement       ClaimElementType = 2
	ParameterElement        ClaimElementType = 3
	ProcessElement          ClaimElementType = 4
)

func (t ClaimElementType) String() string {
	switch t {
	case StructuralElement:
		return "Structural"
	case FunctionalElement:
		return "Functional"
	case ParameterElement:
		return "Parameter"
	case ProcessElement:
		return "Process"
	default:
		return "Unknown"
	}
}

func (t ClaimElementType) IsValid() bool {
	return t >= StructuralElement && t <= ProcessElement
}

// ClaimElement represents a single technical feature within a claim.
type ClaimElement struct {
	ID          string           `json:"id"`
	Text        string           `json:"text"`
	Type        ClaimElementType `json:"type"`
	IsEssential bool             `json:"is_essential"`
	MoleculeRef string           `json:"molecule_ref,omitempty"`
	MarkushRef  string           `json:"markush_ref,omitempty"`
	Constraints []string         `json:"constraints,omitempty"`
}

// Claim is a value object representing a single patent claim.
type Claim struct {
	Number               int            `json:"number"`
	Text                 string         `json:"text"`
	Type                 ClaimType      `json:"type"`
	Category             ClaimCategory  `json:"category"`
	DependsOn            []int          `json:"depends_on,omitempty"`
	Preamble             string         `json:"preamble,omitempty"`
	CharacterizingPortion string         `json:"characterizing_portion,omitempty"`
	Elements             []ClaimElement `json:"elements,omitempty"`
	MarkushStructures    []string       `json:"markush_structures,omitempty"`
	Language             string         `json:"language"`
}

// NewClaim constructs and validates a new Claim.
func NewClaim(number int, text string, claimType ClaimType, category ClaimCategory) (*Claim, error) {
	if number <= 0 {
		return nil, errors.InvalidParam("claim number must be greater than zero")
	}
	trimmedText := strings.TrimSpace(text)
	if len(trimmedText) < 10 || len(trimmedText) > 50000 {
		return nil, errors.InvalidParam("claim text length must be between 10 and 50000 characters")
	}
	if !claimType.IsValid() {
		return nil, errors.InvalidParam("invalid claim type")
	}
	if !category.IsValid() {
		return nil, errors.InvalidParam("invalid claim category")
	}

	return &Claim{
		Number:   number,
		Text:     trimmedText,
		Type:     claimType,
		Category: category,
		Language: "en", // Default language
	}, nil
}

// Validate checks the consistency of the claim.
func (c *Claim) Validate() error {
	if c.Type == ClaimTypeDependent {
		if len(c.DependsOn) == 0 {
			return errors.InvalidParam("dependent claim must have at least one dependency")
		}
		for _, dep := range c.DependsOn {
			if dep >= c.Number {
				return errors.InvalidParam("dependent claim cannot reference itself or forward claims")
			}
		}
	} else if c.Type == ClaimTypeIndependent {
		if len(c.DependsOn) > 0 {
			return errors.InvalidParam("independent claim should not have dependencies")
		}
	}

	if len(c.Elements) > 0 {
		hasEssential := false
		for _, el := range c.Elements {
			if el.IsEssential {
				hasEssential = true
				break
			}
		}
		if !hasEssential {
			return errors.InvalidParam("claim must have at least one essential element")
		}
	}

	// If preamble and characterizing portion are set, they shouldn't both be empty
	// (Note: this check only applies if they have been populated by structural analysis)
	// For now we don't enforce this during basic validation unless they were specifically meant to be set.

	return nil
}

// SetDependencies sets the parent claim numbers for a dependent claim.
func (c *Claim) SetDependencies(deps []int) error {
	if c.Type != ClaimTypeDependent {
		return errors.InvalidParam("only dependent claims can have dependencies")
	}

	seen := make(map[int]bool)
	for _, dep := range deps {
		if dep <= 0 {
			return errors.InvalidParam("dependency claim number must be greater than zero")
		}
		if dep >= c.Number {
			return errors.InvalidParam("dependency claim number must be less than current claim number")
		}
		if seen[dep] {
			return errors.InvalidParam("duplicate dependency claim number")
		}
		seen[dep] = true
	}

	c.DependsOn = deps
	return nil
}

// AddElement adds a technical feature element to the claim.
func (c *Claim) AddElement(elem ClaimElement) error {
	if elem.ID == "" {
		return errors.InvalidParam("element ID cannot be empty")
	}
	if strings.TrimSpace(elem.Text) == "" {
		return errors.InvalidParam("element text cannot be empty")
	}
	if !elem.Type.IsValid() {
		return errors.InvalidParam("invalid element type")
	}

	for _, existing := range c.Elements {
		if existing.ID == elem.ID {
			return errors.InvalidParam(fmt.Sprintf("duplicate element ID: %s", elem.ID))
		}
	}

	c.Elements = append(c.Elements, elem)
	return nil
}

// EssentialElements returns all essential technical features.
func (c *Claim) EssentialElements() []ClaimElement {
	var essential []ClaimElement
	for _, el := range c.Elements {
		if el.IsEssential {
			essential = append(essential, el)
		}
	}
	return essential
}

// HasMarkushStructure reports whether the claim contains Markush structures.
func (c *Claim) HasMarkushStructure() bool {
	return len(c.MarkushStructures) > 0
}

// ContainsMoleculeReference reports whether any element contains a molecule reference.
func (c *Claim) ContainsMoleculeReference() bool {
	for _, el := range c.Elements {
		if el.MoleculeRef != "" {
			return true
		}
	}
	return false
}

// ClaimSet is a collection of claims belonging to a single patent.
type ClaimSet []Claim

// IndependentClaims returns all independent claims in the set.
func (cs ClaimSet) IndependentClaims() []Claim {
	var independent []Claim
	for _, c := range cs {
		if c.Type == ClaimTypeIndependent {
			independent = append(independent, c)
		}
	}
	return independent
}

// DependentClaimsOf returns claims that directly depend on the specified claim number.
func (cs ClaimSet) DependentClaimsOf(number int) []Claim {
	var dependents []Claim
	for _, c := range cs {
		for _, dep := range c.DependsOn {
			if dep == number {
				dependents = append(dependents, c)
				break
			}
		}
	}
	return dependents
}

// ClaimTree returns the specified claim and all its direct and indirect dependents.
func (cs ClaimSet) ClaimTree(rootNumber int) []Claim {
	root, found := cs.FindByNumber(rootNumber)
	if !found {
		return nil
	}

	tree := []Claim{*root}
	toProcess := []int{rootNumber}
	processed := make(map[int]bool)

	for len(toProcess) > 0 {
		current := toProcess[0]
		toProcess = toProcess[1:]

		if processed[current] {
			continue
		}
		processed[current] = true

		deps := cs.DependentClaimsOf(current)
		for _, dep := range deps {
			if !processed[dep.Number] {
				tree = append(tree, dep)
				toProcess = append(toProcess, dep.Number)
			}
		}
	}

	return tree
}

// Validate checks the consistency of the entire claim set.
func (cs ClaimSet) Validate() error {
	if len(cs) == 0 {
		return errors.InvalidParam("claim set cannot be empty")
	}

	hasIndependent := false
	numbers := make(map[int]bool)
	maxNumber := 0

	for _, c := range cs {
		if c.Number <= 0 {
			return errors.InvalidParam("claim number must be positive")
		}
		if numbers[c.Number] {
			return errors.InvalidParam(fmt.Sprintf("duplicate claim number: %d", c.Number))
		}
		numbers[c.Number] = true
		if c.Number > maxNumber {
			maxNumber = c.Number
		}
		if c.Type == ClaimTypeIndependent {
			hasIndependent = true
		}

		if err := c.Validate(); err != nil {
			return fmt.Errorf("invalid claim %d: %w", c.Number, err)
		}

		for _, dep := range c.DependsOn {
			if !numbers[dep] {
				// Dependency must refer to a previous claim.
				// This assumes ClaimSet is ordered, but even if not,
				// the rule is usually that a claim can only depend on a lower-numbered claim.
				if dep >= c.Number {
					return errors.InvalidParam(fmt.Sprintf("claim %d depends on forward claim %d", c.Number, dep))
				}
				// We also need to check if the referenced claim exists at all in the set.
				exists := false
				for _, other := range cs {
					if other.Number == dep {
						exists = true
						break
					}
				}
				if !exists {
					return errors.InvalidParam(fmt.Sprintf("claim %d depends on non-existent claim %d", c.Number, dep))
				}
			}
		}
	}

	if !hasIndependent {
		return errors.InvalidParam("patent must have at least one independent claim")
	}

	// Check continuity from 1 to maxNumber
	for i := 1; i <= maxNumber; i++ {
		if !numbers[i] {
			return errors.InvalidParam(fmt.Sprintf("missing claim number: %d", i))
		}
	}

	return nil
}

// FindByNumber locates a claim by its number.
func (cs ClaimSet) FindByNumber(number int) (*Claim, bool) {
	for i := range cs {
		if cs[i].Number == number {
			return &cs[i], true
		}
	}
	return nil, false
}

//Personal.AI order the ending
