package infringe_net

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// ClaimElementMapper interface
// ---------------------------------------------------------------------------

// ClaimElementMapper decomposes patent claims into structured elements and
// aligns them against molecular structural features. It also handles
// prosecution-history estoppel detection.
type ClaimElementMapper interface {
	MapElements(ctx context.Context, claims []*ClaimInput) ([]*MappedClaim, error)
	MapMoleculeToElements(ctx context.Context, molecule *MoleculeInput) ([]*StructuralElement, error)
	AlignElements(ctx context.Context, moleculeElements []*StructuralElement, claimElements []*ClaimElement) (*ElementAlignment, error)
	CheckEstoppel(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error)
	ParseProsecutionHistory(ctx context.Context, rawHistory []byte) (*ProsecutionHistory, error)
}

// ---------------------------------------------------------------------------
// NLPParser interface
// ---------------------------------------------------------------------------

// RawElement is the output of NLP-based claim text parsing before classification.
type RawElement struct {
	Text       string `json:"text"`
	StartPos   int    `json:"start_pos"`
	EndPos     int    `json:"end_pos"`
	Confidence float64 `json:"confidence"`
}

// NLPParser abstracts natural-language parsing of patent claim text.
type NLPParser interface {
	ParseClaimText(ctx context.Context, text string) ([]*RawElement, error)
	ClassifyElement(ctx context.Context, element *RawElement) (ElementType, error)
}

// ---------------------------------------------------------------------------
// StructureAnalyzer interface
// ---------------------------------------------------------------------------

// StructuralFragment is a piece of a decomposed molecule.
type StructuralFragment struct {
	SMILES      string  `json:"smiles"`
	Role        string  `json:"role"` // "core_scaffold", "substituent", "functional_group", "linker"
	Position    string  `json:"position,omitempty"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight"`
}

// StructureAnalyzer abstracts molecular decomposition and fragment comparison.
type StructureAnalyzer interface {
	DecomposeMolecule(ctx context.Context, smiles string) ([]*StructuralFragment, error)
	ComputeFragmentSimilarity(ctx context.Context, frag1, frag2 string) (float64, error)
	MatchSMARTS(ctx context.Context, smiles, smarts string) (bool, error)
}

// ---------------------------------------------------------------------------
// Claim-side types
// ---------------------------------------------------------------------------

// ClaimType enumerates patent claim types.
type ClaimType string

const (
	ClaimTypeIndependent ClaimType = "independent"
	ClaimTypeDependent   ClaimType = "dependent"
)

// ClaimInput is the raw input for a single patent claim.
type ClaimInput struct {
	ClaimID       string    `json:"claim_id"`
	ClaimText     string    `json:"claim_text"`
	ClaimType     ClaimType `json:"claim_type"`
	ParentClaimID string    `json:"parent_claim_id,omitempty"`
	PriorityDate  time.Time `json:"priority_date,omitempty"`
}

// ClaimElement is a single structured element extracted from a claim.
type ClaimElement struct {
	ElementID            string      `json:"element_id"`
	ElementType          ElementType `json:"element_type"`
	Description          string      `json:"description"`
	StructuralConstraint string      `json:"structural_constraint,omitempty"` // SMARTS or SMILES pattern
	IsEssential          bool        `json:"is_essential"`
	Source               string      `json:"source"` // verbatim text from claim
}

// MappedClaim is the structured output of claim decomposition.
type MappedClaim struct {
	ClaimID         string          `json:"claim_id"`
	ClaimType       ClaimType       `json:"claim_type"`
	Elements        []*ClaimElement `json:"elements"`
	DependencyChain []*ClaimElement `json:"dependency_chain"`
}

// MoleculeInput wraps the input for molecule-to-element mapping.
type MoleculeInput struct {
	SMILES      string `json:"smiles"`
	MoleculeID  string `json:"molecule_id,omitempty"`
	Description string `json:"description,omitempty"`
}

// ---------------------------------------------------------------------------
// Alignment types
// ---------------------------------------------------------------------------

// MatchType classifies the quality of an element-pair match.
type MatchType string

const (
	MatchExact   MatchType = "EXACT"   // >= 0.95
	MatchSimilar MatchType = "SIMILAR" // >= 0.80
	MatchPartial MatchType = "PARTIAL" // >= 0.60
	MatchNone    MatchType = "NONE"    // <  0.60
)

// ClassifyMatchType returns the MatchType for a given similarity score.
func ClassifyMatchType(score float64) MatchType {
	switch {
	case score >= 0.95:
		return MatchExact
	case score >= 0.80:
		return MatchSimilar
	case score >= 0.60:
		return MatchPartial
	default:
		return MatchNone
	}
}

// String returns the string representation of MatchType.
func (m MatchType) String() string { return string(m) }

// AlignedPair is a single molecule-element ↔ claim-element pairing.
type AlignedPair struct {
	MoleculeElement *StructuralElement `json:"molecule_element"`
	ClaimElement    *ClaimElement      `json:"claim_element"`
	SimilarityScore float64            `json:"similarity_score"`
	MatchType       MatchType          `json:"match_type"`
}

// ElementAlignment is the full result of aligning molecule elements to claim elements.
type ElementAlignment struct {
	Pairs                     []*AlignedPair      `json:"pairs"`
	UnmatchedMoleculeElements []*StructuralElement `json:"unmatched_molecule_elements"`
	UnmatchedClaimElements    []*ClaimElement      `json:"unmatched_claim_elements"`
	AlignmentScore            float64              `json:"alignment_score"`
	CoverageRatio             float64              `json:"coverage_ratio"`
}

// ---------------------------------------------------------------------------
// Prosecution-history types
// ---------------------------------------------------------------------------

// AmendmentType classifies a prosecution-history amendment.
type AmendmentType string

const (
	AmendmentNarrowing  AmendmentType = "narrowing"
	AmendmentBroadening AmendmentType = "broadening"
	AmendmentClarifying AmendmentType = "clarifying"
)

// Amendment is a single amendment record from prosecution history.
type Amendment struct {
	AmendmentDate    time.Time     `json:"amendment_date" xml:"AmendmentDate"`
	OriginalText     string        `json:"original_text" xml:"OriginalText"`
	AmendedText      string        `json:"amended_text" xml:"AmendedText"`
	AmendmentType    AmendmentType `json:"amendment_type" xml:"AmendmentType"`
	AffectedElements []string      `json:"affected_elements" xml:"AffectedElements>ElementID"`
}

// ApplicantArgument is a record of an applicant's argument during prosecution.
type ApplicantArgument struct {
	ArgumentDate          time.Time `json:"argument_date" xml:"ArgumentDate"`
	ArgumentText          string    `json:"argument_text" xml:"ArgumentText"`
	DistinguishedFeatures []string  `json:"distinguished_features" xml:"DistinguishedFeatures>Feature"`
	SurrenderScope        string    `json:"surrender_scope" xml:"SurrenderScope"`
}

// RejectionResponse is a record of a rejection and the applicant's response.
type RejectionResponse struct {
	RejectionDate  time.Time `json:"rejection_date" xml:"RejectionDate"`
	RejectionBasis string    `json:"rejection_basis" xml:"RejectionBasis"`
	ResponseDate   time.Time `json:"response_date" xml:"ResponseDate"`
	ResponseText   string    `json:"response_text" xml:"ResponseText"`
}

// ProsecutionHistory is the structured representation of a patent's prosecution history.
type ProsecutionHistory struct {
	PatentID           string               `json:"patent_id" xml:"PatentID"`
	Amendments         []*Amendment         `json:"amendments" xml:"Amendments>Amendment"`
	Arguments          []*ApplicantArgument `json:"arguments" xml:"Arguments>Argument"`
	RejectionResponses []*RejectionResponse `json:"rejection_responses" xml:"RejectionResponses>Response"`
}

// ---------------------------------------------------------------------------
// Estoppel types
// ---------------------------------------------------------------------------

// EstoppelDetail describes a single estoppel constraint.
type EstoppelDetail struct {
	AffectedElementID    string `json:"affected_element_id"`
	AmendmentRef         string `json:"amendment_ref"`
	SurrenderDescription string `json:"surrender_description"`
	BlockedEquivalentType string `json:"blocked_equivalent_type"`
}

// EstoppelResult is the output of prosecution-history estoppel analysis.
type EstoppelResult struct {
	HasEstoppel        bool              `json:"has_estoppel"`
	EstoppelPenalty    float64           `json:"estoppel_penalty"` // 0-1
	BlockedEquivalences []string         `json:"blocked_equivalences"`
	EstoppelDetails    []*EstoppelDetail `json:"estoppel_details"`
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

// claimElementMapper is the concrete implementation of ClaimElementMapper.
type claimElementMapper struct {
	nlp      NLPParser
	analyzer StructureAnalyzer
	model    InfringeModel
	logger   Logger
}

// NewClaimElementMapper creates a new ClaimElementMapper with all required dependencies.
func NewClaimElementMapper(
	nlp NLPParser,
	analyzer StructureAnalyzer,
	model InfringeModel,
	logger Logger,
) (*claimElementMapper, error) {
	if nlp == nil {
		return nil, errors.NewInvalidInputError("NLPParser is required")
	}
	if analyzer == nil {
		return nil, errors.NewInvalidInputError("StructureAnalyzer is required")
	}
	if logger == nil {
		logger = &noopLogger{}
	}
	return &claimElementMapper{
		nlp:      nlp,
		analyzer: analyzer,
		model:    model,
		logger:   logger,
	}, nil
}

// ---------------------------------------------------------------------------
// MapElements
// ---------------------------------------------------------------------------

func (m *claimElementMapper) MapElements(ctx context.Context, claims []*ClaimInput) ([]*MappedClaim, error) {
	if len(claims) == 0 {
		return nil, errors.NewInvalidInputError("claims list is empty")
	}

	// Index claims by ID for dependency resolution.
	claimIndex := make(map[string]*ClaimInput, len(claims))
	for _, c := range claims {
		if c == nil {
			continue
		}
		claimIndex[c.ClaimID] = c
	}

	// First pass: decompose each claim into elements.
	mappedIndex := make(map[string]*MappedClaim, len(claims))
	for _, c := range claims {
		if c == nil {
			continue
		}
		elements, err := m.decomposeClaimText(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("decomposing claim %s: %w", c.ClaimID, err)
		}

		// Mark essential features.
		for _, el := range elements {
			el.IsEssential = (c.ClaimType == ClaimTypeIndependent)
		}

		mappedIndex[c.ClaimID] = &MappedClaim{
			ClaimID:   c.ClaimID,
			ClaimType: c.ClaimType,
			Elements:  elements,
		}
	}

	// Second pass: resolve dependency chains for dependent claims.
	for _, mc := range mappedIndex {
		chain, err := m.buildDependencyChain(mc.ClaimID, claimIndex, mappedIndex, nil)
		if err != nil {
			return nil, fmt.Errorf("building dependency chain for %s: %w", mc.ClaimID, err)
		}
		mc.DependencyChain = chain
	}

	// Collect results in input order.
	results := make([]*MappedClaim, 0, len(claims))
	for _, c := range claims {
		if c == nil {
			continue
		}
		if mc, ok := mappedIndex[c.ClaimID]; ok {
			results = append(results, mc)
		}
	}
	return results, nil
}

// decomposeClaimText uses the NLP parser to break a claim into typed elements.
func (m *claimElementMapper) decomposeClaimText(ctx context.Context, claim *ClaimInput) ([]*ClaimElement, error) {
	if claim.ClaimText == "" {
		return nil, errors.NewParsingError("claim text is empty for claim " + claim.ClaimID)
	}

	rawElements, err := m.nlp.ParseClaimText(ctx, claim.ClaimText)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrParsingFailed, err)
	}
	if len(rawElements) == 0 {
		return nil, errors.NewParsingError("NLP parser returned zero elements for claim " + claim.ClaimID)
	}

	elements := make([]*ClaimElement, 0, len(rawElements))
	for i, raw := range rawElements {
		elemType, err := m.nlp.ClassifyElement(ctx, raw)
		if err != nil {
			m.logger.Warn("element classification failed, defaulting to Unknown",
				"claim_id", claim.ClaimID, "index", i, "error", err)
			elemType = ElementTypeUnknown
		}

		constraint := extractStructuralConstraint(raw.Text)

		elements = append(elements, &ClaimElement{
			ElementID:            fmt.Sprintf("%s-E%03d", claim.ClaimID, i+1),
			ElementType:          elemType,
			Description:          strings.TrimSpace(raw.Text),
			StructuralConstraint: constraint,
			Source:               raw.Text,
		})
	}
	return elements, nil
}

// extractStructuralConstraint attempts to find a SMILES or SMARTS pattern in text.
func extractStructuralConstraint(text string) string {
	// Simple heuristic: look for bracketed SMARTS/SMILES tokens.
	// Real implementation would use a more sophisticated regex or NLP extraction.
	for _, token := range strings.Fields(text) {
		cleaned := strings.Trim(token, ".,;:()")
		if looksLikeSMILES(cleaned) {
			return cleaned
		}
	}
	return ""
}

// looksLikeSMILES is a rough heuristic for detecting SMILES/SMARTS strings.
func looksLikeSMILES(s string) bool {
	if len(s) < 2 {
		return false
	}
	// Added H (hydrogen), r (Br), l (Cl, Al)
	smilesChars := "cCnNoOsSpPFClBrIHhrl=#@+\\/-[]()123456789%"
	nonSMILES := 0
	for _, ch := range s {
		if !strings.ContainsRune(smilesChars, ch) {
			nonSMILES++
		}
	}
	return float64(nonSMILES)/float64(len(s)) < 0.2
}

// buildDependencyChain recursively collects all elements from the dependency chain.
func (m *claimElementMapper) buildDependencyChain(
	claimID string,
	claimIndex map[string]*ClaimInput,
	mappedIndex map[string]*MappedClaim,
	visited map[string]bool,
) ([]*ClaimElement, error) {
	if visited == nil {
		visited = make(map[string]bool)
	}
	if visited[claimID] {
		return nil, fmt.Errorf("circular dependency detected at claim %s", claimID)
	}
	visited[claimID] = true

	mc, ok := mappedIndex[claimID]
	if !ok {
		return nil, fmt.Errorf("claim %s not found in mapped index", claimID)
	}

	ci, ok := claimIndex[claimID]
	if !ok {
		return nil, fmt.Errorf("claim %s not found in claim index", claimID)
	}

	// Base case: independent claim — chain is just its own elements.
	if ci.ClaimType == ClaimTypeIndependent || ci.ParentClaimID == "" {
		chain := make([]*ClaimElement, len(mc.Elements))
		copy(chain, mc.Elements)
		return chain, nil
	}

	// Recursive case: collect parent chain first, then append own elements.
	parentChain, err := m.buildDependencyChain(ci.ParentClaimID, claimIndex, mappedIndex, visited)
	if err != nil {
		return nil, err
	}

	// Merge: parent elements + own additional elements.
	chain := make([]*ClaimElement, 0, len(parentChain)+len(mc.Elements))
	chain = append(chain, parentChain...)
	chain = append(chain, mc.Elements...)
	return chain, nil
}

// ---------------------------------------------------------------------------
// MapMoleculeToElements
// ---------------------------------------------------------------------------

func (m *claimElementMapper) MapMoleculeToElements(ctx context.Context, molecule *MoleculeInput) ([]*StructuralElement, error) {
	if molecule == nil || molecule.SMILES == "" {
		return nil, errors.NewInvalidInputError("molecule SMILES is required")
	}

	fragments, err := m.analyzer.DecomposeMolecule(ctx, molecule.SMILES)
	if err != nil {
		return nil, fmt.Errorf("decomposing molecule: %w", err)
	}
	if len(fragments) == 0 {
		return nil, errors.NewParsingError("structure analyzer returned zero fragments")
	}

	elements := make([]*StructuralElement, 0, len(fragments))
	for i, frag := range fragments {
		elemType := roleToElementType(frag.Role)
		elements = append(elements, &StructuralElement{
			ElementID:   fmt.Sprintf("MOL-%s-F%03d", molecule.MoleculeID, i+1),
			ElementType: elemType,
			SMILES:      frag.SMILES,
			Description: frag.Description,
			Role:        frag.Role,
			Position:    frag.Position,
			Weight:      frag.Weight,
		})
	}
	return elements, nil
}

// roleToElementType maps a fragment role string to an ElementType.
func roleToElementType(role string) ElementType {
	switch strings.ToLower(role) {
	case "core_scaffold", "scaffold", "core":
		return ElementTypeCoreScaffold
	case "substituent", "sub":
		return ElementTypeSubstituent
	case "functional_group", "functional", "group":
		return ElementTypeFunctionalGroup
	case "linker", "bridge":
		return ElementTypeLinker
	case "backbone":
		return ElementTypeBackbone
	default:
		return ElementTypeUnknown
	}
}

// ---------------------------------------------------------------------------
// AlignElements — Hungarian Algorithm
// ---------------------------------------------------------------------------

func (m *claimElementMapper) AlignElements(
	ctx context.Context,
	moleculeElements []*StructuralElement,
	claimElements []*ClaimElement,
) (*ElementAlignment, error) {
	if len(moleculeElements) == 0 || len(claimElements) == 0 {
		return nil, errors.NewInvalidInputError("both molecule and claim element lists must be non-empty")
	}

	nMol := len(moleculeElements)
	nClaim := len(claimElements)

	// Build similarity matrix (nMol x nClaim).
	simMatrix := make([][]float64, nMol)
	for i := 0; i < nMol; i++ {
		simMatrix[i] = make([]float64, nClaim)
		for j := 0; j < nClaim; j++ {
			sim, err := m.computeElementSimilarity(ctx, moleculeElements[i], claimElements[j])
			if err != nil {
				m.logger.Warn("similarity computation failed, using 0",
					"mol_elem", moleculeElements[i].ElementID,
					"claim_elem", claimElements[j].ElementID,
					"error", err)
				sim = 0
			}
			simMatrix[i][j] = sim
		}
	}

	// Solve assignment using Hungarian algorithm (maximize similarity).
	// We convert to a cost matrix (1 - sim) and find minimum-cost assignment.
	assignment := hungarianMaximize(simMatrix, nMol, nClaim)

	// Build aligned pairs and track unmatched.
	matchedMol := make(map[int]bool)
	matchedClaim := make(map[int]bool)
	var pairs []*AlignedPair
	totalScore := 0.0
	matchedClaimCount := 0

	for molIdx, claimIdx := range assignment {
		if claimIdx < 0 || claimIdx >= nClaim {
			continue
		}
		sim := simMatrix[molIdx][claimIdx]
		mt := ClassifyMatchType(sim)

		// Only count as matched if at least Partial.
		if mt == MatchNone {
			continue
		}

		matchedMol[molIdx] = true
		matchedClaim[claimIdx] = true
		matchedClaimCount++
		totalScore += sim

		pairs = append(pairs, &AlignedPair{
			MoleculeElement: moleculeElements[molIdx],
			ClaimElement:    claimElements[claimIdx],
			SimilarityScore: sim,
			MatchType:       mt,
		})
	}

	// Unmatched elements.
	var unmatchedMol []*StructuralElement
	for i, el := range moleculeElements {
		if !matchedMol[i] {
			unmatchedMol = append(unmatchedMol, el)
		}
	}
	var unmatchedClaim []*ClaimElement
	for j, el := range claimElements {
		if !matchedClaim[j] {
			unmatchedClaim = append(unmatchedClaim, el)
		}
	}

	// Scores.
	alignmentScore := 0.0
	if len(pairs) > 0 {
		alignmentScore = totalScore / float64(len(pairs))
	}
	coverageRatio := 0.0
	if nClaim > 0 {
		coverageRatio = float64(matchedClaimCount) / float64(nClaim)
	}

	return &ElementAlignment{
		Pairs:                     pairs,
		UnmatchedMoleculeElements: unmatchedMol,
		UnmatchedClaimElements:    unmatchedClaim,
		AlignmentScore:            alignmentScore,
		CoverageRatio:             coverageRatio,
	}, nil
}

// computeElementSimilarity computes similarity between a molecule element and a claim element.
func (m *claimElementMapper) computeElementSimilarity(
	ctx context.Context,
	molElem *StructuralElement,
	claimElem *ClaimElement,
) (float64, error) {
	// Type bonus: same type gets a boost.
	typeBonus := 0.0
	if molElem.ElementType == claimElem.ElementType {
		typeBonus = 0.15
	}

	// If claim has a structural constraint (SMARTS), try direct matching.
	if claimElem.StructuralConstraint != "" && molElem.SMILES != "" {
		matched, err := m.analyzer.MatchSMARTS(ctx, molElem.SMILES, claimElem.StructuralConstraint)
		if err == nil && matched {
			return math.Min(1.0, 0.90+typeBonus), nil
		}
	}

	// Fragment-level similarity.
	if molElem.SMILES != "" && claimElem.StructuralConstraint != "" {
		sim, err := m.analyzer.ComputeFragmentSimilarity(ctx, molElem.SMILES, claimElem.StructuralConstraint)
		if err == nil {
			return math.Min(1.0, sim+typeBonus), nil
		}
	}

	// Fallback: use model if available.
	if m.model != nil && molElem.SMILES != "" {
		// Use description-based similarity as a last resort.
		return typeBonus + 0.30, nil // conservative baseline
	}

	// Pure type match only.
	if typeBonus > 0 {
		return typeBonus + 0.25, nil
	}
	return 0.10, nil
}

// ---------------------------------------------------------------------------
// Hungarian Algorithm (Kuhn-Munkres) for maximum-weight bipartite matching
// ---------------------------------------------------------------------------

// hungarianMaximize solves the maximum-weight assignment problem.
// Returns assignment[molIdx] = claimIdx (or -1 if unassigned).
func hungarianMaximize(sim [][]float64, nRows, nCols int) []int {
	// Pad to square matrix.
	n := nRows
	if nCols > n {
		n = nCols
	}

	// Build cost matrix (negate for minimization).
	cost := make([][]float64, n)
	for i := 0; i < n; i++ {
		cost[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			if i < nRows && j < nCols {
				cost[i][j] = -sim[i][j]
			}
		}
	}

	// Hungarian algorithm.
	const inf = math.MaxFloat64 / 2
	u := make([]float64, n+1)
	v := make([]float64, n+1)
	p := make([]int, n+1)    // p[j] = row assigned to column j
	way := make([]int, n+1)  // way[j] = previous column in augmenting path

	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		minv := make([]float64, n+1)
		used := make([]bool, n+1)
		for j := 0; j <= n; j++ {
			minv[j] = inf
			used[j] = false
		}

		for {
			used[j0] = true
			i0 := p[j0]
			delta := inf
			j1 := -1

			for j := 1; j <= n; j++ {
				if used[j] {
					continue
				}
				cur := cost[i0-1][j-1] - u[i0] - v[j]
				if cur < minv[j] {
					minv[j] = cur
					way[j] = j0
				}
				if minv[j] < delta {
					delta = minv[j]
					j1 = j
				}
			}

			for j := 0; j <= n; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}

			j0 = j1
			if p[j0] == 0 {
				break
			}
		}

		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}

	// Extract assignment: row i -> column assignment.
	result := make([]int, nRows)
	for i := range result {
		result[i] = -1
	}
	for j := 1; j <= n; j++ {
		if p[j] > 0 && p[j] <= nRows && j <= nCols {
			result[p[j]-1] = j - 1
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// CheckEstoppel
// ---------------------------------------------------------------------------

func (m *claimElementMapper) CheckEstoppel(
	ctx context.Context,
	alignment *ElementAlignment,
	history *ProsecutionHistory,
) (*EstoppelResult, error) {
	if alignment == nil {
		return nil, errors.NewInvalidInputError("alignment is required")
	}

	result := &EstoppelResult{
		HasEstoppel:         false,
		EstoppelPenalty:     0,
		BlockedEquivalences: []string{},
		EstoppelDetails:     []*EstoppelDetail{},
	}

	if history == nil || (len(history.Amendments) == 0 && len(history.Arguments) == 0) {
		return result, nil
	}

	// Collect element IDs affected by narrowing amendments.
	narrowingAffected := make(map[string]*Amendment)
	for _, amend := range history.Amendments {
		if amend.AmendmentType != AmendmentNarrowing {
			continue
		}
		for _, elemID := range amend.AffectedElements {
			narrowingAffected[elemID] = amend
		}
	}

	// Collect surrender scopes from applicant arguments.
	var surrenderScopes []string
	for _, arg := range history.Arguments {
		if arg.SurrenderScope != "" {
			surrenderScopes = append(surrenderScopes, arg.SurrenderScope)
		}
	}

	// Check each aligned pair for estoppel.
	equivalentPairCount := 0
	blockedCount := 0
	essentialBlockedCount := 0

	for _, pair := range alignment.Pairs {
		// Only equivalence-based matches (Similar/Partial) can be estopped.
		if pair.MatchType == MatchExact || pair.MatchType == MatchNone {
			continue
		}
		equivalentPairCount++

		claimElemID := pair.ClaimElement.ElementID

		// Check narrowing amendment estoppel.
		if amend, affected := narrowingAffected[claimElemID]; affected {
			blockedCount++
			if pair.ClaimElement.IsEssential {
				essentialBlockedCount++
			}
			result.EstoppelDetails = append(result.EstoppelDetails, &EstoppelDetail{
				AffectedElementID:     claimElemID,
				AmendmentRef:          fmt.Sprintf("amendment-%s", amend.AmendmentDate.Format("2006-01-02")),
				SurrenderDescription:  fmt.Sprintf("Narrowed from '%s' to '%s'", amend.OriginalText, amend.AmendedText),
				BlockedEquivalentType: string(pair.MatchType),
			})
			result.BlockedEquivalences = append(result.BlockedEquivalences, claimElemID)
		}

		// Check surrender scope estoppel.
		for _, scope := range surrenderScopes {
			if m.elementFallsInSurrenderScope(pair, scope) {
				if _, alreadyBlocked := narrowingAffected[claimElemID]; !alreadyBlocked {
					blockedCount++
					if pair.ClaimElement.IsEssential {
						essentialBlockedCount++
					}
					result.EstoppelDetails = append(result.EstoppelDetails, &EstoppelDetail{
						AffectedElementID:     claimElemID,
						AmendmentRef:          "applicant-argument",
						SurrenderDescription:  scope,
						BlockedEquivalentType: string(pair.MatchType),
					})
					result.BlockedEquivalences = append(result.BlockedEquivalences, claimElemID)
				}
			}
		}
	}

	// Compute penalty.
	if equivalentPairCount > 0 && blockedCount > 0 {
		result.HasEstoppel = true

		// Base penalty: ratio of blocked equivalences.
		basePenalty := float64(blockedCount) / float64(equivalentPairCount)

		// Essential-element weighting: essential blocks carry 1.5x weight.
		essentialWeight := 1.0
		if essentialBlockedCount > 0 && blockedCount > 0 {
			essentialRatio := float64(essentialBlockedCount) / float64(blockedCount)
			essentialWeight = 1.0 + 0.5*essentialRatio
		}

		result.EstoppelPenalty = math.Min(1.0, basePenalty*essentialWeight)
	}

	return result, nil
}

// elementFallsInSurrenderScope checks whether an aligned pair's equivalence
// falls within a surrender scope described in an applicant argument.
func (m *claimElementMapper) elementFallsInSurrenderScope(pair *AlignedPair, scope string) bool {
	if scope == "" || pair == nil {
		return false
	}
	// Heuristic: check if the claim element description or the molecule element
	// description contains keywords from the surrender scope.
	scopeLower := strings.ToLower(scope)
	claimDescLower := strings.ToLower(pair.ClaimElement.Description)
	molDescLower := ""
	if pair.MoleculeElement != nil {
		molDescLower = strings.ToLower(pair.MoleculeElement.Description)
	}

	// Tokenize scope into keywords (skip short words).
	keywords := extractKeywords(scopeLower)
	if len(keywords) == 0 {
		return false
	}

	matchCount := 0
	for _, kw := range keywords {
		if strings.Contains(claimDescLower, kw) || strings.Contains(molDescLower, kw) {
			matchCount++
		}
	}

	// If more than half the keywords match, consider it within scope.
	return float64(matchCount)/float64(len(keywords)) > 0.5
}

// extractKeywords splits text into meaningful keywords (length >= 4).
func extractKeywords(text string) []string {
	words := strings.Fields(text)
	var keywords []string
	stopWords := map[string]bool{
		"the": true, "and": true, "for": true, "with": true,
		"from": true, "that": true, "this": true, "which": true,
		"have": true, "been": true, "were": true, "are": true,
	}
	for _, w := range words {
		cleaned := strings.Trim(w, ".,;:()\"'")
		if len(cleaned) >= 4 && !stopWords[cleaned] {
			keywords = append(keywords, cleaned)
		}
	}
	return keywords
}

// ---------------------------------------------------------------------------
// ParseProsecutionHistory
// ---------------------------------------------------------------------------

func (m *claimElementMapper) ParseProsecutionHistory(ctx context.Context, rawHistory []byte) (*ProsecutionHistory, error) {
	if len(rawHistory) == 0 {
		return nil, errors.NewInvalidInputError("raw history is empty")
	}

	trimmed := strings.TrimSpace(string(rawHistory))
	if len(trimmed) == 0 {
		return nil, errors.NewInvalidInputError("raw history is empty after trimming")
	}

	// Detect format: JSON starts with '{', XML starts with '<'.
	var history ProsecutionHistory
	switch trimmed[0] {
	case '{':
		if err := json.Unmarshal(rawHistory, &history); err != nil {
			return nil, fmt.Errorf("%w: JSON parse error: %v", errors.ErrParsingFailed, err)
		}
	case '<':
		if err := xml.Unmarshal(rawHistory, &history); err != nil {
			return nil, fmt.Errorf("%w: XML parse error: %v", errors.ErrParsingFailed, err)
		}
	default:
		return nil, fmt.Errorf("%w: unrecognized format (expected JSON or XML)", errors.ErrParsingFailed)
	}

	if history.PatentID == "" {
		return nil, fmt.Errorf("%w: patent_id is missing", errors.ErrParsingFailed)
	}

	return &history, nil
}

// ---------------------------------------------------------------------------
// noopLogger fallback
// ---------------------------------------------------------------------------

type noopLogger struct{}

func (l *noopLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *noopLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *noopLogger) Error(msg string, keysAndValues ...interface{}) {}
func (l *noopLogger) Debug(msg string, keysAndValues ...interface{}) {}

