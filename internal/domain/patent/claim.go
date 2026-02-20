// Package patent contains the patent aggregate root and all its constituent
// value objects, domain services, and repository interfaces.
package patent

import (
	"strings"
	"unicode"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// ClaimElement value object
// ─────────────────────────────────────────────────────────────────────────────

// ClaimElement represents an atomic technical element within a patent claim.
// A single claim is typically decomposed into multiple elements (the "all
// elements rule" used in infringement analysis requires every element of an
// asserted claim to be found in the accused product or process).
type ClaimElement struct {
	// ID is the globally unique identifier for this element.
	ID common.ID

	// Text is the raw text of the claim element as parsed from the claim body.
	Text string

	// IsStructural indicates whether this element describes a structural
	// feature (true) or a functional limitation (false).  The distinction
	// matters for means-plus-function claim interpretation under 35 U.S.C. § 112.
	IsStructural bool

	// ChemicalEntities lists the chemical entity names or SMILES strings
	// extracted from this element's text by the ChemExtractor NER pipeline.
	// An empty slice indicates that no chemical entities were identified.
	ChemicalEntities []string
}

// ─────────────────────────────────────────────────────────────────────────────
// Claim value object
// ─────────────────────────────────────────────────────────────────────────────

// Claim is an immutable value object representing a single patent claim.
// Independent claims stand alone; dependent claims incorporate by reference
// one or more earlier claims (identified by ParentClaimNumber).
//
// Claims are the legally operative part of a patent: only the claims define
// the scope of protection.  All FTO, infringement, and validity analyses
// operate at claim granularity.
type Claim struct {
	// ID is the platform-internal unique identifier for this claim record.
	ID common.ID

	// Number is the sequential claim number as it appears in the patent document
	// (e.g., 1, 2, … N).  Must be ≥ 1.
	Number int

	// Text is the full legal text of the claim.
	Text string

	// Type classifies the claim (independent, dependent, method, composition, etc.).
	Type ptypes.ClaimType

	// ParentClaimNumber is set for dependent claims and identifies the claim
	// number from which this claim depends.  Nil for independent claims.
	ParentClaimNumber *int

	// Elements is the ordered list of technical elements decomposed from the
	// claim text by the ClaimBERT parser.
	Elements []ClaimElement
}

// ─────────────────────────────────────────────────────────────────────────────
// Factory function
// ─────────────────────────────────────────────────────────────────────────────

// NewClaim constructs and validates a Claim value object.
//
// Validation rules:
//   - number must be > 0
//   - text must be non-empty
//   - dependent claims (type ptypes.ClaimTypeDependent) must supply a non-nil parentNumber
//   - parentNumber, when supplied, must be > 0 and < number
func NewClaim(
	number int,
	text string,
	claimType ptypes.ClaimType,
	parentNumber *int,
) (*Claim, error) {
	if number <= 0 {
		return nil, errors.InvalidParam("claim number must be greater than zero").
			WithDetail("number=" + itoa(number))
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, errors.InvalidParam("claim text must not be empty")
	}

	// Dependent claims must reference a parent claim.
	if claimType == ptypes.ClaimTypeDependent {
		if parentNumber == nil {
			return nil, errors.InvalidParam(
				"dependent claim must have a parent claim number")
		}
		if *parentNumber <= 0 {
			return nil, errors.InvalidParam("parent claim number must be greater than zero").
				WithDetail("parentNumber=" + itoa(*parentNumber))
		}
		if *parentNumber >= number {
			return nil, errors.InvalidParam(
				"parent claim number must be less than the dependent claim number").
				WithDetail("parentNumber=" + itoa(*parentNumber) + " number=" + itoa(number))
		}
	}

	// Independent claim types must not carry a parent reference.
	if claimType != ptypes.ClaimTypeDependent && parentNumber != nil {
		return nil, errors.InvalidParam(
			"non-dependent claim must not specify a parent claim number").
			WithDetail("claimType=" + string(claimType))
	}

	return &Claim{
		ID:                common.NewID(),
		Number:            number,
		Text:              trimmed,
		Type:              claimType,
		ParentClaimNumber: parentNumber,
		Elements:          make([]ClaimElement, 0),
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Domain methods
// ─────────────────────────────────────────────────────────────────────────────

// AddElement appends a ClaimElement to the claim's element list.
// Returns an error if the element text is empty.
func (c *Claim) AddElement(element ClaimElement) error {
	if strings.TrimSpace(element.Text) == "" {
		return errors.InvalidParam("claim element text must not be empty")
	}
	if element.ID == "" {
		element.ID = common.NewID()
	}
	c.Elements = append(c.Elements, element)
	return nil
}

// IsIndependent reports whether this claim stands alone without depending on
// any other claim.  Under patent law, infringement of an independent claim is
// assessed without reference to limitations found only in dependent claims.
func (c *Claim) IsIndependent() bool {
	return c.ParentClaimNumber == nil
}

// ContainsChemicalEntity reports whether any of this claim's elements contain
// at least one extracted chemical entity (name or SMILES).  Claims that contain
// chemical entities are candidates for Markush enumeration and molecular
// similarity-based FTO analysis.
func (c *Claim) ContainsChemicalEntity() bool {
	for _, el := range c.Elements {
		if len(el.ChemicalEntities) > 0 {
			return true
		}
	}
	return false
}

// ExtractKeyTerms performs lightweight key-term extraction from the claim text.
// The implementation tokenises the text, lowercases tokens, and filters out a
// curated list of patent claim stop-words (functional connectors, articles,
// prepositions, and common legal boilerplate).  The resulting terms are
// deduplicated and sorted.
//
// Note: this is a heuristic implementation suitable for indexing and search
// assistance.  For semantic analysis, use the ClaimBERT service instead.
func (c *Claim) ExtractKeyTerms() []string {
	// Tokenise on whitespace and punctuation.
	fields := strings.FieldsFunc(c.Text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})

	seen := make(map[string]struct{}, len(fields))
	result := make([]string, 0, len(fields)/2)

	for _, tok := range fields {
		lower := strings.ToLower(tok)
		if isStopWord(lower) {
			continue
		}
		if len(lower) < 3 {
			continue
		}
		if _, dup := seen[lower]; dup {
			continue
		}
		seen[lower] = struct{}{}
		result = append(result, lower)
	}

	return result
}

// ToDTO converts the Claim value object to the transport-layer ClaimDTO.
func (c *Claim) ToDTO() ptypes.ClaimDTO {
	elements := make([]ptypes.ClaimElementDTO, 0, len(c.Elements))
	for _, el := range c.Elements {
		elements = append(elements, ptypes.ClaimElementDTO{
			ID:               el.ID,
			Text:             el.Text,
			IsStructural:     el.IsStructural,
			ChemicalEntities: el.ChemicalEntities,
		})
	}

	return ptypes.ClaimDTO{
		ID:                c.ID,
		Number:            c.Number,
		Text:              c.Text,
		Type:              c.Type,
		ParentClaimNumber: c.ParentClaimNumber,
		Elements:          elements,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Package-level helpers
// ─────────────────────────────────────────────────────────────────────────────

// claimStopWords contains tokens that carry no discriminative value in patent
// claim text and should be excluded from key-term extraction.
var claimStopWords = map[string]struct{}{
	// Articles and pronouns
	"a": {}, "an": {}, "the": {}, "its": {}, "their": {},
	// Prepositions and conjunctions
	"of": {}, "in": {}, "on": {}, "at": {}, "by": {}, "to": {}, "for": {},
	"from": {}, "with": {}, "into": {}, "through": {}, "between": {},
	"and": {}, "or": {}, "but": {}, "nor": {}, "not": {}, "as": {}, "than": {},
	// Common patent boilerplate
	"claim": {}, "claims": {}, "wherein": {}, "comprising": {}, "comprising:": {},
	"consists": {}, "consisting": {}, "essentially": {}, "consists": {},
	"according": {}, "defined": {}, "described": {}, "said": {}, "thereof": {},
	"therein": {}, "whereby": {}, "wherein,": {}, "further": {}, "least": {},
	"one": {}, "two": {}, "more": {}, "plurality": {}, "set": {}, "group": {},
	"each": {}, "such": {}, "that": {}, "which": {}, "having": {}, "being": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "has": {},
	"have": {}, "had": {}, "do": {}, "does": {}, "did": {}, "will": {},
	"would": {}, "shall": {}, "should": {}, "may": {}, "might": {}, "can": {},
	"could": {}, "this": {}, "these": {}, "those": {}, "it": {}, "he": {},
}

// isStopWord reports whether a token should be filtered out during key-term extraction.
func isStopWord(token string) bool {
	_, ok := claimStopWords[token]
	return ok
}

// itoa converts an int to its decimal string representation without importing
// the strconv package at file level (strconv is imported in service.go anyway).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

//Personal.AI order the ending
