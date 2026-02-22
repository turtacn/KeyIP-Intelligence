package infringe_net

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ClaimInput represents input for claim mapping.
type ClaimInput struct {
	ClaimID      string
	ClaimText    string
	ClaimType    string // Independent/Dependent
	ParentClaimID string
}

// MappedClaim represents a mapped claim.
type MappedClaim struct {
	ClaimID         string
	ClaimType       string
	Elements        []*ClaimElement
	DependencyChain []*ClaimElement
}

// ClaimElement represents a structural element of a claim.
type ClaimElement struct {
	ElementID           string
	ElementType         ElementType
	Description         string
	StructuralConstraint string
	IsEssential         bool
	Source              string
}

// ElementAlignment represents the alignment between molecule and claim elements.
type ElementAlignment struct {
	Pairs                   []*AlignedPair
	UnmatchedMoleculeElements []*StructuralElement
	UnmatchedClaimElements    []*ClaimElement
	AlignmentScore          float64
	CoverageRatio           float64
}

// AlignedPair represents an aligned pair.
type AlignedPair struct {
	MoleculeElement *StructuralElement
	ClaimElement    *ClaimElement
	SimilarityScore float64
	MatchType       MatchType
}

// MatchType defines the quality of a match.
type MatchType string

const (
	MatchTypeExact   MatchType = "Exact"
	MatchTypeSimilar MatchType = "Similar"
	MatchTypePartial MatchType = "Partial"
	MatchTypeNone    MatchType = "None"
)

// ProsecutionHistory represents the history of patent prosecution.
type ProsecutionHistory struct {
	PatentID           string
	Amendments         []*Amendment
	Arguments          []*ApplicantArgument
}

// Amendment represents a claim amendment.
type Amendment struct {
	AmendmentDate    string
	OriginalText     string
	AmendedText      string
	AmendmentType    string // Narrowing/Broadening/Clarifying
	AffectedElements []string // ElementIDs
}

// ApplicantArgument represents an argument made by the applicant.
type ApplicantArgument struct {
	ArgumentDate         string
	ArgumentText         string
	DistinguishedFeatures []string
	SurrenderScope       string
}

// EstoppelResult represents the result of estoppel check.
type EstoppelResult struct {
	HasEstoppel        bool
	EstoppelPenalty    float64
	BlockedEquivalences []string
	EstoppelDetails    []*EstoppelDetail
}

// EstoppelDetail contains details about estoppel.
type EstoppelDetail struct {
	AffectedElementID      string
	AmendmentRef           string
	SurrenderDescription   string
	BlockedEquivalentType  string
}

// NLPParser defines the interface for NLP parsing.
type NLPParser interface {
	ParseClaimText(ctx context.Context, text string) ([]*RawElement, error)
	ClassifyElement(ctx context.Context, element *RawElement) (ElementType, error)
}

// RawElement represents a raw parsed element.
type RawElement struct {
	Text string
	Tags []string
}

// StructureAnalyzer defines the interface for structure analysis.
type StructureAnalyzer interface {
	DecomposeMolecule(ctx context.Context, smiles string) ([]*StructuralFragment, error)
	ComputeFragmentSimilarity(ctx context.Context, frag1, frag2 string) (float64, error)
	MatchSMARTS(ctx context.Context, smiles, smarts string) (bool, error)
}

// StructuralFragment represents a decomposed fragment.
type StructuralFragment struct {
	SMILES string
	Type   ElementType
}

// ClaimElementMapper defines the interface for mapping.
type ClaimElementMapper interface {
	MapElements(ctx context.Context, claims []*ClaimInput) ([]*MappedClaim, error)
	MapMoleculeToElements(ctx context.Context, moleculeSMILES string) ([]*StructuralElement, error)
	AlignElements(ctx context.Context, moleculeElements []*StructuralElement, claimElements []*ClaimElement) (*ElementAlignment, error)
	CheckEstoppel(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error)
	ParseProsecutionHistory(ctx context.Context, rawHistory []byte) (*ProsecutionHistory, error)
}

// claimElementMapper implements ClaimElementMapper.
type claimElementMapper struct {
	nlp      NLPParser
	analyzer StructureAnalyzer
	model    InfringeModel
	logger   logging.Logger
}

// NewClaimElementMapper creates a new ClaimElementMapper.
func NewClaimElementMapper(nlp NLPParser, analyzer StructureAnalyzer, model InfringeModel, logger logging.Logger) (ClaimElementMapper, error) {
	if nlp == nil || analyzer == nil {
		return nil, errors.New("dependencies cannot be nil")
	}
	return &claimElementMapper{
		nlp:      nlp,
		analyzer: analyzer,
		model:    model,
		logger:   logger,
	}, nil
}

func (m *claimElementMapper) MapElements(ctx context.Context, claims []*ClaimInput) ([]*MappedClaim, error) {
	if len(claims) == 0 {
		return nil, errors.New("claims cannot be empty")
	}

	var mappedClaims []*MappedClaim
	claimMap := make(map[string]*MappedClaim)

	for _, input := range claims {
		rawElements, err := m.nlp.ParseClaimText(ctx, input.ClaimText)
		if err != nil {
			return nil, err
		}

		var elements []*ClaimElement
		for i, raw := range rawElements {
			elType, err := m.nlp.ClassifyElement(ctx, raw)
			if err != nil {
				return nil, err
			}

			el := &ClaimElement{
				ElementID:   fmt.Sprintf("%s-el%d", input.ClaimID, i),
				ElementType: elType,
				Description: raw.Text,
				Source:      raw.Text,
				// StructuralConstraint extracted via regex or nlp
				IsEssential: input.ClaimType == "Independent", // Simplified
			}
			elements = append(elements, el)
		}

		mc := &MappedClaim{
			ClaimID:   input.ClaimID,
			ClaimType: input.ClaimType,
			Elements:  elements,
		}

		// Handle dependency
		if input.ClaimType == "Dependent" && input.ParentClaimID != "" {
			if parent, ok := claimMap[input.ParentClaimID]; ok {
				// Copy parent elements (simplified chain)
				mc.DependencyChain = append(mc.DependencyChain, parent.Elements...)
			}
		} else {
			// Independent claims are essential
			for _, el := range elements {
				el.IsEssential = true
			}
		}

		mappedClaims = append(mappedClaims, mc)
		claimMap[input.ClaimID] = mc
	}

	return mappedClaims, nil
}

func (m *claimElementMapper) MapMoleculeToElements(ctx context.Context, moleculeSMILES string) ([]*StructuralElement, error) {
	if moleculeSMILES == "" {
		return nil, errors.New("invalid smiles")
	}
	frags, err := m.analyzer.DecomposeMolecule(ctx, moleculeSMILES)
	if err != nil {
		return nil, err
	}

	var elements []*StructuralElement
	for i, f := range frags {
		elements = append(elements, &StructuralElement{
			ElementID:      fmt.Sprintf("mol-el%d", i),
			ElementType:    f.Type,
			SMILESFragment: f.SMILES,
			Description:    string(f.Type),
		})
	}
	return elements, nil
}

func (m *claimElementMapper) AlignElements(ctx context.Context, moleculeElements []*StructuralElement, claimElements []*ClaimElement) (*ElementAlignment, error) {
	if len(claimElements) == 0 {
		return nil, errors.New("claim elements empty")
	}

	var pairs []*AlignedPair
	var unmatchedClaim []*ClaimElement
	var unmatchedMol []*StructuralElement = make([]*StructuralElement, len(moleculeElements))
	copy(unmatchedMol, moleculeElements)

	// Implementation using Global Sort-Greedy algorithm to approximate optimal matching.
	// 1. Calculate all pairwise scores between Claim elements and Molecule elements of same type.
	// 2. Sort all potential edges by score descending.
	// 3. Iterate edges and select if both endpoints are unmatched.

	type matchCandidate struct {
		claimEl *ClaimElement
		molEl   *StructuralElement
		score   float64
	}

	var candidates []matchCandidate

	// Pre-group molecule elements by type for faster lookup
	molByType := make(map[ElementType][]*StructuralElement)
	for _, me := range moleculeElements {
		molByType[me.ElementType] = append(molByType[me.ElementType], me)
	}

	// 1. Calculate scores
	for _, ce := range claimElements {
		potentialMatches := molByType[ce.ElementType]
		for _, me := range potentialMatches {
			score := 0.0

			// SMARTS constraint check (Hard constraint or max score)
			if ce.StructuralConstraint != "" {
				match, _ := m.analyzer.MatchSMARTS(ctx, me.SMILESFragment, ce.StructuralConstraint)
				if match {
					score = 1.0
				} else {
					// If SMARTS constraint exists but fails, score should be low?
					// Or we allow fallback to similarity? Usually structural constraint is strict.
					// Let's assume strict for constraint, but allow similarity if constraint is empty.
					// If constraint fails, score is 0.
					score = 0.0
				}
			} else {
				// Similarity check
				// Note: Description is text, SMILESFragment is structure.
				// Real implementation uses multi-modal embedding.
				// Here we assume ComputeFragmentSimilarity handles this.
				sim, _ := m.analyzer.ComputeFragmentSimilarity(ctx, me.SMILESFragment, ce.Description)
				score = sim
			}

			if score > 0.5 { // Min threshold
				candidates = append(candidates, matchCandidate{ce, me, score})
			}
		}
	}

	// 2. Sort descending
	// Simple bubble sort or standard sort (requires defining interface or closure)
	// Since slice is local, we can implement a simple sort
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].score > candidates[i].score {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// 3. Select matches
	matchedClaimIDs := make(map[string]bool)
	matchedMolIDs := make(map[string]bool)

	for _, cand := range candidates {
		if matchedClaimIDs[cand.claimEl.ElementID] || matchedMolIDs[cand.molEl.ElementID] {
			continue
		}

		matchType := MatchTypePartial
		if cand.score >= 0.95 {
			matchType = MatchTypeExact
		} else if cand.score >= 0.8 {
			matchType = MatchTypeSimilar
		}

		pairs = append(pairs, &AlignedPair{
			MoleculeElement: cand.molEl,
			ClaimElement:    cand.claimEl,
			SimilarityScore: cand.score,
			MatchType:       matchType,
		})

		matchedClaimIDs[cand.claimEl.ElementID] = true
		matchedMolIDs[cand.molEl.ElementID] = true
	}

	// Find unmatched
	for _, ce := range claimElements {
		if !matchedClaimIDs[ce.ElementID] {
			unmatchedClaim = append(unmatchedClaim, ce)
		}
	}

	// Update unmatchedMol
	var finalUnmatchedMol []*StructuralElement
	for _, me := range moleculeElements {
		if !matchedMolIDs[me.ElementID] {
			finalUnmatchedMol = append(finalUnmatchedMol, me)
		}
	}

	coverage := float64(len(pairs)) / float64(len(claimElements))

	// Alignment score: weighted by similarity
	totalSim := 0.0
	for _, p := range pairs {
		totalSim += p.SimilarityScore
	}
	alignmentScore := 0.0
	if len(pairs) > 0 {
		alignmentScore = totalSim / float64(len(claimElements)) // divide by total required
	}

	return &ElementAlignment{
		Pairs:                   pairs,
		UnmatchedMoleculeElements: finalUnmatchedMol,
		UnmatchedClaimElements:    unmatchedClaim,
		AlignmentScore:          alignmentScore,
		CoverageRatio:           coverage,
	}, nil
}

func (m *claimElementMapper) CheckEstoppel(ctx context.Context, alignment *ElementAlignment, history *ProsecutionHistory) (*EstoppelResult, error) {
	if history == nil {
		return &EstoppelResult{HasEstoppel: false}, nil
	}

	var details []*EstoppelDetail
	blocked := 0

	for _, pair := range alignment.Pairs {
		// Only check for equivalents (non-exact matches)
		if pair.MatchType == MatchTypeExact {
			continue
		}

		// Check if this element was amended narrowing scope
		for _, am := range history.Amendments {
			if am.AmendmentType == "Narrowing" {
				for _, affID := range am.AffectedElements {
					if affID == pair.ClaimElement.ElementID {
						// Found estoppel
						details = append(details, &EstoppelDetail{
							AffectedElementID: pair.ClaimElement.ElementID,
							AmendmentRef:      am.AmendmentDate, // ID ideally
							BlockedEquivalentType: "Narrowing Amendment",
						})
						blocked++
					}
				}
			}
		}

		// Check arguments
		for _, arg := range history.Arguments {
			// Simplified text check
			if strings.Contains(arg.SurrenderScope, pair.ClaimElement.Description) {
				details = append(details, &EstoppelDetail{
					AffectedElementID: pair.ClaimElement.ElementID,
					SurrenderDescription: arg.SurrenderScope,
					BlockedEquivalentType: "Applicant Argument",
				})
				blocked++
			}
		}
	}

	hasEstoppel := blocked > 0
	penalty := 0.0
	if len(alignment.Pairs) > 0 {
		penalty = float64(blocked) / float64(len(alignment.Pairs))
	}

	return &EstoppelResult{
		HasEstoppel:     hasEstoppel,
		EstoppelPenalty: penalty,
		EstoppelDetails: details,
	}, nil
}

func (m *claimElementMapper) ParseProsecutionHistory(ctx context.Context, rawHistory []byte) (*ProsecutionHistory, error) {
	if len(rawHistory) == 0 {
		return nil, errors.New("empty history")
	}
	var history ProsecutionHistory
	if err := json.Unmarshal(rawHistory, &history); err != nil {
		return nil, errors.New("parsing failed")
	}
	return &history, nil
}

//Personal.AI order the ending
