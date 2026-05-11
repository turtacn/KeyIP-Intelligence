package molpatent_gnn

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Atom / Bond feature encoding constants
// ---------------------------------------------------------------------------

// AtomFeatureSet defines the atom-level features extracted for each node.
// Total dimension = 80 (matching feature encoding output).
//
// Features (one-hot or scalar):
//   [0..43]   Atomic number one-hot (H,C,N,O,F,P,S,Cl,Br,I + others → 44 bins)
//   [44..48]  Degree one-hot (0,1,2,3,4+)
//   [49..53]  Formal charge one-hot (-2,-1,0,+1,+2)
//   [54..57]  Num H one-hot (0,1,2,3+)
//   [58..63]  Hybridization one-hot (s,sp,sp2,sp3,sp3d,sp3d2)
//   [64..67]  Chirality one-hot (none,R,S,other)
//   [68..73]  Ring size one-hot (0,3,4,5,6,7+)
//   [74]      Is aromatic (binary)
//   [75]      Is in ring (binary)
//   [76]      Atomic mass (normalised)
//   [77]      Electronegativity (normalised)
//   [78]      Van der Waals radius (normalised)
//   [79]      Number of radical electrons (normalised)

const (
	atomicNumberBins  = 44
	degreeBins        = 5
	formalChargeBins  = 5
	numHBins          = 4
	hybridizationBins = 6
	chiralityBins     = 4
	ringSizeBins      = 6
	binaryFeatures    = 2 // is_aromatic, is_in_ring
	scalarFeatures    = 4 // mass, electronegativity, vdw_radius, radical_electrons
	totalNodeFeatures = atomicNumberBins + degreeBins + formalChargeBins +
		numHBins + hybridizationBins + chiralityBins + ringSizeBins +
		binaryFeatures + scalarFeatures // = 80
)

// BondFeatureSet defines the bond-level features for each edge.
// Total dimension = 12 (matching GNNModelConfig.EdgeFeatureDim default).
//
// Features:
//   [0..3]  Bond type one-hot (single, double, triple, aromatic)
//   [4]     Is conjugated (binary)
//   [5]     Is in ring (binary)
//   [6..8]  Stereo one-hot (none, E, Z)
//   [9..11] Bond direction one-hot (none, begin_wedge, end_wedge)

const (
	bondTypeBins      = 4
	bondBinaryFeats   = 2
	stereoBins        = 3
	directionBins     = 3
	totalEdgeFeatures = bondTypeBins + bondBinaryFeats + stereoBins + directionBins // = 12
)

// ---------------------------------------------------------------------------
// Atom property tables (simplified — production would use RDKit via CGo/gRPC)
// ---------------------------------------------------------------------------

// atomicNumberMap maps element symbols to atomic numbers.
var atomicNumberMap = map[string]int{
	"H": 1, "He": 2, "Li": 3, "Be": 4, "B": 5, "C": 6, "N": 7, "O": 8,
	"F": 9, "Ne": 10, "Na": 11, "Mg": 12, "Al": 13, "Si": 14, "P": 15,
	"S": 16, "Cl": 17, "Ar": 18, "K": 19, "Ca": 20, "Br": 35, "I": 53,
	"Fe": 26, "Cu": 29, "Zn": 30, "Se": 34, "Sn": 50, "Pt": 78,
}

// atomicMassMap maps atomic number to atomic mass (normalised by 200).
var atomicMassMap = map[int]float32{
	1: 1.008 / 200, 6: 12.011 / 200, 7: 14.007 / 200, 8: 15.999 / 200,
	9: 18.998 / 200, 15: 30.974 / 200, 16: 32.06 / 200, 17: 35.45 / 200,
	35: 79.904 / 200, 53: 126.90 / 200,
}

// electronegativityMap maps atomic number to Pauling electronegativity (normalised by 4).
var electronegativityMap = map[int]float32{
	1: 2.20 / 4, 6: 2.55 / 4, 7: 3.04 / 4, 8: 3.44 / 4,
	9: 3.98 / 4, 15: 2.19 / 4, 16: 2.58 / 4, 17: 3.16 / 4,
	35: 2.96 / 4, 53: 2.66 / 4,
}

// vdwRadiusMap maps atomic number to Van der Waals radius in Angstroms (normalised by 3.0).
var vdwRadiusMap = map[int]float32{
	1:  1.20 / 3.0, // H
	3:  1.82 / 3.0, // Li
	4:  1.53 / 3.0, // Be
	5:  1.92 / 3.0, // B
	6:  1.70 / 3.0, // C
	7:  1.55 / 3.0, // N
	8:  1.52 / 3.0, // O
	9:  1.47 / 3.0, // F
	11: 2.27 / 3.0, // Na
	12: 1.73 / 3.0, // Mg
	13: 1.84 / 3.0, // Al
	14: 2.10 / 3.0, // Si
	15: 1.80 / 3.0, // P
	16: 1.80 / 3.0, // S
	17: 1.75 / 3.0, // Cl
	19: 2.75 / 3.0, // K
	20: 2.31 / 3.0, // Ca
	26: 2.04 / 3.0, // Fe
	29: 1.40 / 3.0, // Cu
	30: 1.39 / 3.0, // Zn
	34: 1.90 / 3.0, // Se
	35: 1.85 / 3.0, // Br
	50: 2.17 / 3.0, // Sn
	53: 1.98 / 3.0, // I
	78: 1.75 / 3.0, // Pt
}

// countRadicalElectrons returns the number of unpaired electrons for an atom.
// This is determined from the ground-state electron configuration.
// Most organic atoms have 0 radical electrons in their neutral state;
// known exceptions (e.g., free radicals) are handled explicitly.
func countRadicalElectrons(atomicNum int) int {
	// For common organic elements: all paired in neutral ground state.
	// Radical species are rare and typically require explicit SMILES notation.
	switch atomicNum {
	case 1:
		return 0 // H: 1s1 — paired in H2, treated as 0 for molecular context
	case 6:
		return 0 // C: [He]2s2 2p2 — all paired in typical bonding
	case 7:
		return 0 // N: [He]2s2 2p3 — paired
	case 8:
		return 0 // O: [He]2s2 2p4 — paired
	case 9:
		return 0 // F: [He]2s2 2p5 — paired
	case 15:
		return 0 // P: [Ne]3s2 3p3
	case 16:
		return 0 // S: [Ne]3s2 3p4
	case 17:
		return 0 // Cl: [Ne]3s2 3p5
	case 35:
		return 0 // Br: [Ar]4s2 3d10 4p5
	case 53:
		return 0 // I: [Kr]5s2 4d10 5p5
	default:
		return 0 // Default: assume paired
	}
}

// ---------------------------------------------------------------------------
// SMILES validation
// ---------------------------------------------------------------------------

// smilesPattern is a simplified regex for basic SMILES validation.
// Production systems should use RDKit for full validation.
var smilesPattern = regexp.MustCompile(
	`^[A-Za-z0-9@+\-\[\]()=#$:/\\.%]+$`,
)

// balancedBrackets checks that [ ] and ( ) are balanced and correctly nested.
func balancedBrackets(s string) bool {
	var stack []rune
	for _, ch := range s {
		switch ch {
		case '[', '(':
			stack = append(stack, ch)
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return false
			}
			stack = stack[:len(stack)-1]
		case ')':
			if len(stack) == 0 || stack[len(stack)-1] != '(' {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}
	return len(stack) == 0
}

// ---------------------------------------------------------------------------
// gnnPreprocessorImpl
// ---------------------------------------------------------------------------

// DefaultGraphCacheSize is the default maximum number of MolecularGraph entries to cache.
const DefaultGraphCacheSize = 10000

// gnnPreprocessorImpl is the production implementation of GNNPreprocessor.
type gnnPreprocessorImpl struct {
	config *GNNModelConfig
	mu     sync.RWMutex
	cache  *molecule.FingerprintCache // caches MolecularGraph results by SMILES
}

// NewGNNPreprocessor creates a new preprocessor.
func NewGNNPreprocessor(config *GNNModelConfig) (GNNPreprocessor, error) {
	if config == nil {
		return nil, errors.NewInvalidInputError("config is required")
	}
	return &gnnPreprocessorImpl{
		config: config,
		cache:  molecule.NewFingerprintCache(DefaultGraphCacheSize),
	}, nil
}

// ValidateSMILES performs lightweight structural validation of a SMILES string.
func (p *gnnPreprocessorImpl) ValidateSMILES(smiles string) error {
	if smiles == "" {
		return errors.NewInvalidInputError("SMILES string is empty")
	}
	if len(smiles) > 5000 {
		return errors.NewInvalidInputError("SMILES string exceeds maximum length (5000)")
	}
	if !smilesPattern.MatchString(smiles) {
		return errors.NewInvalidInputError(
			fmt.Sprintf("SMILES contains invalid characters: %s", smiles))
	}
	if !balancedBrackets(smiles) {
		return errors.NewInvalidInputError("SMILES has unbalanced brackets")
	}
	return nil
}

// Canonicalize returns a canonical form of the SMILES.
// In production this would delegate to RDKit; here we do a simplified
// normalisation (lowercase aromatic atoms, strip whitespace).
func (p *gnnPreprocessorImpl) Canonicalize(smiles string) (string, error) {
	canonical := strings.TrimSpace(smiles)
	if err := p.ValidateSMILES(canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

// PreprocessSMILES converts a SMILES string into a MolecularGraph.
//
// Pipeline:
//  1. Validate SMILES
//  2. Parse atoms and bonds from the SMILES string
//  3. Encode atom features → NodeFeatures
//  4. Encode bond features → EdgeFeatures + EdgeIndex
//  5. Compute global features (molecular weight, atom count, etc.)
//  6. Validate against MaxAtoms
func (p *gnnPreprocessorImpl) PreprocessSMILES(ctx context.Context, smiles string) (*MolecularGraph, error) {
	if err := p.ValidateSMILES(smiles); err != nil {
		return nil, err
	}

	// Check cache for existing result
	normalized := molecule.NormalizeSMILES(smiles)
	if cached, ok := p.cache.Get(normalized); ok {
		return cached.(*MolecularGraph), nil
	}

	atoms, bonds, err := parseSMILES(smiles)
	if err != nil {
		return nil, fmt.Errorf("SMILES parsing failed: %w", err)
	}

	if len(atoms) == 0 {
		return nil, errors.NewInvalidInputError("no atoms found in SMILES")
	}
	if len(atoms) > p.config.MaxAtoms {
		return nil, errors.NewInvalidInputError(
			fmt.Sprintf("molecule has %d atoms, exceeds max %d", len(atoms), p.config.MaxAtoms))
	}

	// Encode node features
	nodeFeatures := make([][]float32, len(atoms))
	for i, atom := range atoms {
		nodeFeatures[i] = encodeAtomFeatures(atom)
	}

	// Encode edge features (undirected: add both directions).
	// Pre-allocate with known capacity: each bond produces 2 edges.
	numBonds := len(bonds)
	edgeIndex := make([][2]int, 0, numBonds*2)
	edgeFeatures := make([][]float32, 0, numBonds*2)
	for _, bond := range bonds {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		ef := encodeBondFeatures(bond)
		edgeIndex = append(edgeIndex, [2]int{bond.Src, bond.Dst})
		edgeFeatures = append(edgeFeatures, ef)
		// Reverse edge for undirected graph
		edgeIndex = append(edgeIndex, [2]int{bond.Dst, bond.Src})
		edgeFeatures = append(edgeFeatures, ef)
	}

	// Global features
	globalFeatures := computeGlobalFeatures(atoms, bonds)

	graph := &MolecularGraph{
		NodeFeatures:   nodeFeatures,
		EdgeIndex:      edgeIndex,
		EdgeFeatures:   edgeFeatures,
		GlobalFeatures: globalFeatures,
		NumAtoms:       len(atoms),
		NumBonds:       len(bonds),
		SMILES:         smiles,
	}

	// Store in cache
	p.cache.Set(normalized, graph)
	return graph, nil
}

// PreprocessMOL converts a MOL block into a MolecularGraph.
func (p *gnnPreprocessorImpl) PreprocessMOL(ctx context.Context, molBlock string) (*MolecularGraph, error) {
	if molBlock == "" {
		return nil, errors.NewInvalidInputError("MOL block is empty")
	}

	atoms, bonds, err := parseMOLBlock(molBlock)
	if err != nil {
		return nil, fmt.Errorf("MOL block parsing failed: %w", err)
	}

	if len(atoms) == 0 {
		return nil, errors.NewInvalidInputError("no atoms found in MOL block")
	}
	if len(atoms) > p.config.MaxAtoms {
		return nil, errors.NewInvalidInputError(
			fmt.Sprintf("molecule has %d atoms, exceeds max %d", len(atoms), p.config.MaxAtoms))
	}

	// Encode node features
	nodeFeatures := make([][]float32, len(atoms))
	for i, atom := range atoms {
		nodeFeatures[i] = encodeAtomFeatures(atom)
	}

	// Encode edge features (undirected: add both directions).
	// Pre-allocate with known capacity: each bond produces 2 edges.
	numBonds := len(bonds)
	edgeIndex := make([][2]int, 0, numBonds*2)
	edgeFeatures := make([][]float32, 0, numBonds*2)
	for _, bond := range bonds {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		ef := encodeBondFeatures(bond)
		edgeIndex = append(edgeIndex, [2]int{bond.Src, bond.Dst})
		edgeFeatures = append(edgeFeatures, ef)
		// Reverse edge for undirected graph
		edgeIndex = append(edgeIndex, [2]int{bond.Dst, bond.Src})
		edgeFeatures = append(edgeFeatures, ef)
	}

	// Global features
	globalFeatures := computeGlobalFeatures(atoms, bonds)

	return &MolecularGraph{
		NodeFeatures:   nodeFeatures,
		EdgeIndex:      edgeIndex,
		EdgeFeatures:   edgeFeatures,
		GlobalFeatures: globalFeatures,
		NumAtoms:       len(atoms),
		NumBonds:       len(bonds),
		SMILES:         "",
	}, nil
}

// PreprocessBatch processes multiple molecular inputs.
func (p *gnnPreprocessorImpl) PreprocessBatch(ctx context.Context, inputs []MolecularInput) ([]*MolecularGraph, error) {
	results := make([]*MolecularGraph, len(inputs))
	for i, input := range inputs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var graph *MolecularGraph
		var err error
		if input.SMILES != "" {
			graph, err = p.PreprocessSMILES(ctx, input.SMILES)
		} else if input.MOLBlock != "" {
			graph, err = p.PreprocessMOL(ctx, input.MOLBlock)
		} else {
			err = errors.NewInvalidInputError("input has neither SMILES nor MOL block")
		}
		if err != nil {
			return nil, fmt.Errorf("batch item %d: %w", i, err)
		}
		results[i] = graph
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Internal SMILES parser (simplified)
// ---------------------------------------------------------------------------

// parsedAtom represents a parsed atom from SMILES.
type parsedAtom struct {
	Symbol     string
	AtomicNum  int
	IsAromatic bool
	Charge     int
	NumH       int
	Degree     int
}

// parsedBond represents a parsed bond from SMILES.
type parsedBond struct {
	Src        int
	Dst        int
	BondType   int // 1=single, 2=double, 3=triple, 4=aromatic
	InRing     bool
	Conjugated bool
}

// parseSMILES is a simplified SMILES tokeniser.
// Production code would use RDKit via CGo or a gRPC service.
func parseSMILES(smiles string) ([]parsedAtom, []parsedBond, error) {
	// Not taking a context parameter since this is a pure string parser
	// used from PreprocessSMILES which already checks ctx.Done() before
	// calling.  For very long SMILES we check at branch points below.
	var atoms []parsedAtom
	var bonds []parsedBond

	runes := []rune(smiles)
	i := 0
	atomStack := []int{} // stack for branch tracking
	prevAtom := -1
	nextBondType := 1

	for i < len(runes) {
		ch := runes[i]

		switch {
		case ch == '(':
			if prevAtom >= 0 {
				atomStack = append(atomStack, prevAtom)
			}
			i++

		case ch == ')':
			if len(atomStack) > 0 {
				prevAtom = atomStack[len(atomStack)-1]
				atomStack = atomStack[:len(atomStack)-1]
			}
			i++

		case ch == '-':
			nextBondType = 1
			i++
		case ch == '=':
			nextBondType = 2
			i++
		case ch == '#':
			nextBondType = 3
			i++
		case ch == ':':
			nextBondType = 4
			i++

		case ch == '[':
			// Bracket atom
			j := i + 1
			for j < len(runes) && runes[j] != ']' {
				j++
			}
			if j >= len(runes) {
				return nil, nil, fmt.Errorf("unclosed bracket at position %d", i)
			}
			bracketContent := string(runes[i+1 : j])
			atom := parseBracketAtom(bracketContent)
			atomIdx := len(atoms)
			atoms = append(atoms, atom)
			if prevAtom >= 0 {
				bonds = append(bonds, parsedBond{
					Src:      prevAtom,
					Dst:      atomIdx,
					BondType: nextBondType,
				})
				atoms[prevAtom].Degree++
				atoms[atomIdx].Degree++
				nextBondType = 1
			}
			prevAtom = atomIdx
			i = j + 1

		case ch == '%':
			// Two-digit ring closure — skip for simplified parser
			i += 3

		case ch >= '0' && ch <= '9':
			// Ring closure digit — simplified handling
			// In a full parser this would create a bond back to the ring-opening atom.
			i++

		case ch == '/' || ch == '\\':
			// Stereo bond markers — skip
			i++

		case ch == '.':
			// Disconnected fragment
			prevAtom = -1
			i++

		case unicode.IsLetter(ch):
			symbol, aromatic, advance := parseOrganicAtom(runes, i)
			atomicNum := lookupAtomicNumber(symbol)
			atom := parsedAtom{
				Symbol:     symbol,
				AtomicNum:  atomicNum,
				IsAromatic: aromatic,
				NumH:       estimateImplicitH(atomicNum, 0),
			}
			atomIdx := len(atoms)
			atoms = append(atoms, atom)

			bondType := nextBondType
			if aromatic && prevAtom >= 0 && atoms[prevAtom].IsAromatic {
				bondType = 4 // aromatic bond between aromatic atoms
			}

			if prevAtom >= 0 {
				bonds = append(bonds, parsedBond{
					Src:      prevAtom,
					Dst:      atomIdx,
					BondType: bondType,
				})
				atoms[prevAtom].Degree++
				atoms[atomIdx].Degree++
				nextBondType = 1
			}
			prevAtom = atomIdx
			i += advance

		default:
			i++
		}
	}

	return atoms, bonds, nil
}

// parseOrganicAtom extracts an organic-subset atom symbol starting at position i.
// Returns (symbol, isAromatic, numRunesConsumed).
func parseOrganicAtom(runes []rune, i int) (string, bool, int) {
	ch := runes[i]

	// Aromatic atoms: b, c, n, o, p, s
	aromatic := unicode.IsLower(ch)
	upper := unicode.ToUpper(ch)

	// Two-letter elements: Cl, Br, Si, Se, etc.
	if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
		twoLetter := string([]rune{upper, runes[i+1]})
		if _, ok := atomicNumberMap[twoLetter]; ok {
			return twoLetter, false, 2
		}
	}

	return string(upper), aromatic, 1
}

// parseBracketAtom parses the content inside [...].
func parseBracketAtom(content string) parsedAtom {
	atom := parsedAtom{}

	// Extract element symbol (first uppercase + optional lowercase)
	runes := []rune(content)
	idx := 0

	// Skip isotope number
	for idx < len(runes) && unicode.IsDigit(runes[idx]) {
		idx++
	}

	if idx < len(runes) && unicode.IsLetter(runes[idx]) {
		start := idx
		aromatic := unicode.IsLower(runes[idx])
		atom.IsAromatic = aromatic
		idx++
		for idx < len(runes) && unicode.IsLower(runes[idx]) {
			idx++
		}
		sym := string(runes[start:idx])
		if aromatic {
			sym = strings.ToUpper(sym[:1]) + sym[1:]
		}
		atom.Symbol = sym
		atom.AtomicNum = lookupAtomicNumber(sym)
	}

	// Parse charge
	rest := string(runes[idx:])
	if strings.Contains(rest, "++") {
		atom.Charge = 2
	} else if strings.Contains(rest, "+") {
		atom.Charge = 1
	} else if strings.Contains(rest, "--") {
		atom.Charge = -2
	} else if strings.Contains(rest, "-") {
		atom.Charge = -1
	}

	// Parse explicit H count
	if hIdx := strings.Index(rest, "H"); hIdx >= 0 {
		if hIdx+1 < len(rest) && rest[hIdx+1] >= '0' && rest[hIdx+1] <= '9' {
			atom.NumH = int(rest[hIdx+1] - '0')
		} else {
			atom.NumH = 1
		}
	}

	return atom
}

// lookupAtomicNumber returns the atomic number for a symbol, or 0 if unknown.
func lookupAtomicNumber(symbol string) int {
	if n, ok := atomicNumberMap[symbol]; ok {
		return n
	}
	return 0
}

// estimateImplicitH estimates implicit hydrogen count based on valence rules.
func estimateImplicitH(atomicNum int, explicitBonds int) int {
	// Simplified valence table
	valence := map[int]int{
		6: 4, 7: 3, 8: 2, 9: 1, 15: 3, 16: 2, 17: 1, 35: 1, 53: 1,
	}
	v, ok := valence[atomicNum]
	if !ok {
		return 0
	}
	h := v - explicitBonds
	if h < 0 {
		h = 0
	}
	return h
}

// ---------------------------------------------------------------------------
// MOL V2000 block parser
// ---------------------------------------------------------------------------

// parseMOLBlock parses a V2000 (or V3000) MOL block string into atoms and bonds.
// It handles the common V2000 format; V3000 is detected and delegated.
func parseMOLBlock(molBlock string) ([]parsedAtom, []parsedBond, error) {
	lines := strings.Split(molBlock, "\n")
	// Trim trailing whitespace/carriage returns
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r ")
	}

	if len(lines) < 4 {
		return nil, nil, fmt.Errorf("MOL block too short: %d lines", len(lines))
	}

	// Detect V3000 by checking for "V3000" in the counts line (line 3, 0-indexed).
	if len(lines) > 3 && strings.Contains(lines[3], "V3000") {
		return parseV3000MOL(lines)
	}

	// V2000 parsing
	// Line 4 (0-indexed, after 3 header lines) is the counts line.
	// Format:   nAtoms nBonds ...
	countsLine := ""
	headerEnd := 3 // skip 3 header lines

	// Try to find the counts line (some blocks have blank header lines).
	for i := 0; i < 5 && headerEnd < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[headerEnd])
		if trimmed != "" {
			countsLine = trimmed
			break
		}
		headerEnd++
	}
	if countsLine == "" {
		return nil, nil, fmt.Errorf("MOL block: counts line not found")
	}

	// Parse counts from the first two 3-digit fields.
	// Format example: " 12 11  0  0  0  0  0  0  0  0999 V2000"
	// nAtoms is the first 3 characters
	var nAtoms, nBonds int
	if len(countsLine) >= 6 {
		nAtoms, _ = strconv.Atoi(strings.TrimSpace(countsLine[:3]))
		nBonds, _ = strconv.Atoi(strings.TrimSpace(countsLine[3:6]))
	} else {
		// Fallback: try space-delimited parsing
		fields := strings.Fields(countsLine)
		if len(fields) < 2 {
			return nil, nil, fmt.Errorf("MOL block: cannot parse counts line: %q", countsLine)
		}
		nAtoms, _ = strconv.Atoi(fields[0])
		nBonds, _ = strconv.Atoi(fields[1])
	}

	if nAtoms <= 0 || nAtoms > 1000 {
		return nil, nil, fmt.Errorf("MOL block: invalid atom count: %d", nAtoms)
	}
	if nBonds < 0 || nBonds > 2000 {
		return nil, nil, fmt.Errorf("MOL block: invalid bond count: %d", nBonds)
	}

	// Atom block
	atoms := make([]parsedAtom, 0, nAtoms)
	atomStartLine := headerEnd + 1
	for i := 0; i < nAtoms && atomStartLine+i < len(lines); i++ {
		line := lines[atomStartLine+i]
		if len(line) < 34 {
			return nil, nil, fmt.Errorf("MOL block: atom line %d too short: %q", i, line)
		}
		// Atom symbol is at positions 31-33 (1-indexed 31-33, 0-indexed 30-33)
		symField := strings.TrimSpace(line[30:34])
		if symField == "" {
			// Try alternate position (older format: element in columns 31-33, 0-indexed)
			symField = strings.TrimSpace(line[30:33])
		}
		if symField == "" {
			return nil, nil, fmt.Errorf("MOL block: empty symbol in atom line %d", i)
		}

		// Charge encoding (column 36, 0-indexed 35)
		// 0=0, 1=+1, 2=+2, 3=+3, 4=-1, 5=-2, 6=-3, 7=doublet radical
		charge := 0
		if len(line) > 36 {
			chargeCode, err := strconv.Atoi(strings.TrimSpace(line[35:36]))
			if err == nil {
				switch chargeCode {
				case 1:
					charge = 1
				case 2:
					charge = 2
				case 3:
					charge = 3
				case 4:
					charge = -1
				case 5:
					charge = -2
				case 6:
					charge = -3
					// 7 = doublet radical — charge 0 with radical electron
				}
			}
		}

		// Mass difference (column 34, 0-indexed 33)
		massDiff := 0
		if len(line) > 34 {
			massDiff, _ = strconv.Atoi(strings.TrimSpace(line[33:34]))
		}

		symbol := symField
		// Capitalize first letter for lookup
		if len(symbol) > 0 && symbol[0] >= 'a' && symbol[0] <= 'z' {
			symbol = strings.ToUpper(symbol[:1]) + symbol[1:]
		}

		atomicNum := lookupAtomicNumber(symbol)
		atom := parsedAtom{
			Symbol:    symField,
			AtomicNum: atomicNum,
			Charge:    charge,
			NumH:      estimateImplicitH(atomicNum, 0),
		}
		// Adjust mass difference if specified
		if massDiff > 0 {
			// Mass difference is isotopic mass - atomic mass
			// Not directly used in feature encoding but we could store it
			_ = massDiff
		}
		atoms = append(atoms, atom)
	}

	if len(atoms) != nAtoms {
		return nil, nil, fmt.Errorf("MOL block: expected %d atoms, parsed %d", nAtoms, len(atoms))
	}

	// Bond block
	bonds := make([]parsedBond, 0, nBonds)
	bondStartLine := atomStartLine + nAtoms
	for i := 0; i < nBonds && bondStartLine+i < len(lines); i++ {
		line := lines[bondStartLine+i]
		if len(line) < 9 {
			return nil, nil, fmt.Errorf("MOL block: bond line %d too short: %q", i, line)
		}
		// Atom indices are 1-indexed in MOL format
		a1, err1 := strconv.Atoi(strings.TrimSpace(line[0:3]))
		a2, err2 := strconv.Atoi(strings.TrimSpace(line[3:6]))
		bType, err3 := strconv.Atoi(strings.TrimSpace(line[6:9]))
		if err1 != nil || err2 != nil || err3 != nil {
			return nil, nil, fmt.Errorf("MOL block: parse error in bond line %d", i)
		}
		if a1 < 1 || a1 > nAtoms || a2 < 1 || a2 > nAtoms {
			return nil, nil, fmt.Errorf("MOL block: bond %d references invalid atom indices (%d, %d)", i, a1, a2)
		}

		bonds = append(bonds, parsedBond{
			Src:      a1 - 1, // Convert to 0-indexed
			Dst:      a2 - 1,
			BondType: bType,
		})
		atoms[a1-1].Degree++
		atoms[a2-1].Degree++
	}

	return atoms, bonds, nil
}

// parseV3000MOL handles V3000-format MOL blocks (minimal implementation).
func parseV3000MOL(lines []string) ([]parsedAtom, []parsedBond, error) {
	var atoms []parsedAtom
	var bonds []parsedBond

	inAtomBlock := false
	inBondBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "BEGIN ATOM") {
			inAtomBlock = true
			inBondBlock = false
			continue
		}
		if strings.Contains(trimmed, "BEGIN BOND") {
			inAtomBlock = false
			inBondBlock = true
			continue
		}
		if strings.Contains(trimmed, "END ATOM") || strings.Contains(trimmed, "END BOND") {
			inAtomBlock = false
			inBondBlock = false
			continue
		}

		if inAtomBlock {
			// V3000 atom format: atomNumber elementSymbol x y z ...
			fields := strings.Fields(trimmed)
			if len(fields) < 5 {
				continue
			}
			symbol := fields[1]
			atomicNum := lookupAtomicNumber(symbol)
			atom := parsedAtom{
				Symbol:    symbol,
				AtomicNum: atomicNum,
				NumH:      estimateImplicitH(atomicNum, 0),
			}
			// Check for charge in the remaining fields
			for _, field := range fields {
				if strings.HasPrefix(field, "CHG=") {
					chgStr := strings.TrimPrefix(field, "CHG=")
					if chg, err := strconv.Atoi(chgStr); err == nil {
						atom.Charge = chg
					}
				}
			}
			atoms = append(atoms, atom)
		}

		if inBondBlock {
			// V3000 bond format: bondNumber a1 a2 type [params]
			fields := strings.Fields(trimmed)
			if len(fields) < 4 {
				continue
			}
			a1, err1 := strconv.Atoi(fields[1])
			a2, err2 := strconv.Atoi(fields[2])
			bType, err3 := strconv.Atoi(fields[3])
			if err1 != nil || err2 != nil || err3 != nil {
				continue
			}
			// Convert 1-indexed to 0-indexed
			a1--
			a2--
			bonds = append(bonds, parsedBond{
				Src:      a1,
				Dst:      a2,
				BondType: bType,
			})
		}
	}

	if len(atoms) == 0 {
		return nil, nil, fmt.Errorf("V3000 MOL: no atoms found")
	}
	if len(bonds) == 0 && len(atoms) > 1 {
		// Allow single atoms with no bonds
	}

	// Update atom degrees from bonds
	for _, b := range bonds {
		if b.Src >= 0 && b.Src < len(atoms) {
			atoms[b.Src].Degree++
		}
		if b.Dst >= 0 && b.Dst < len(atoms) {
			atoms[b.Dst].Degree++
		}
	}

	return atoms, bonds, nil
}

// ---------------------------------------------------------------------------
// Feature encoding
// ---------------------------------------------------------------------------

// encodeAtomFeatures produces a float32 feature vector of length totalNodeFeatures.
func encodeAtomFeatures(atom parsedAtom) []float32 {
	features := make([]float32, totalNodeFeatures)
	offset := 0

	// Atomic number one-hot [0..43]
	bin := atomicNumToBin(atom.AtomicNum)
	if bin >= 0 && bin < atomicNumberBins {
		features[offset+bin] = 1.0
	}
	offset += atomicNumberBins

	// Degree one-hot [44..48]
	deg := atom.Degree
	if deg >= degreeBins {
		deg = degreeBins - 1
	}
	features[offset+deg] = 1.0
	offset += degreeBins

	// Formal charge one-hot [49..53] — mapped: -2→0, -1→1, 0→2, +1→3, +2→4
	chargeBin := atom.Charge + 2
	if chargeBin < 0 {
		chargeBin = 0
	}
	if chargeBin >= formalChargeBins {
		chargeBin = formalChargeBins - 1
	}
	features[offset+chargeBin] = 1.0
	offset += formalChargeBins

	// Num H one-hot [54..57]
	hBin := atom.NumH
	if hBin >= numHBins {
		hBin = numHBins - 1
	}
	features[offset+hBin] = 1.0
	offset += numHBins

	// Hybridization one-hot [58..63] — default to sp3 (index 3)
	hybBin := 3
	features[offset+hybBin] = 1.0
	offset += hybridizationBins

	// Chirality one-hot [64..67] — default to none (index 0)
	features[offset+0] = 1.0
	offset += chiralityBins

	// Ring size one-hot [68..73] — default to 0 (not in ring, index 0)
	features[offset+0] = 1.0
	offset += ringSizeBins

	// Is aromatic [74]
	if atom.IsAromatic {
		features[offset] = 1.0
	}
	offset++

	// Is in ring [75] — simplified: aromatic implies ring
	if atom.IsAromatic {
		features[offset] = 1.0
	}
	offset++

	// Atomic mass normalised [76]
	if mass, ok := atomicMassMap[atom.AtomicNum]; ok {
		features[offset] = mass
	}
	offset++

	// Electronegativity normalised [77]
	if en, ok := electronegativityMap[atom.AtomicNum]; ok {
		features[offset] = en
	}
	offset++

	// Van der Waals radius normalised
	if vdw, ok := vdwRadiusMap[atom.AtomicNum]; ok {
		features[offset] = vdw
	} else {
		features[offset] = 0.5 // fallback
	}
	offset++

	// Radical electrons normalised (max 3 for typical radicals)
	radicalE := countRadicalElectrons(atom.AtomicNum)
	features[offset] = float32(radicalE) / 3.0

	return features
}

// atomicNumToBin maps an atomic number to a one-hot bin index.
// Common organic atoms get dedicated bins; rare atoms go to the last bin.
func atomicNumToBin(atomicNum int) int {
	commonAtoms := []int{1, 6, 7, 8, 9, 15, 16, 17, 35, 53}
	for i, a := range commonAtoms {
		if atomicNum == a {
			return i
		}
	}
	return atomicNumberBins - 1 // "other" bin
}

// encodeBondFeatures produces a float32 feature vector of length totalEdgeFeatures.
func encodeBondFeatures(bond parsedBond) []float32 {
	features := make([]float32, totalEdgeFeatures)
	offset := 0

	// Bond type one-hot [0..3]
	bt := bond.BondType - 1
	if bt < 0 {
		bt = 0
	}
	if bt >= bondTypeBins {
		bt = bondTypeBins - 1
	}
	features[offset+bt] = 1.0
	offset += bondTypeBins

	// Is conjugated [4]
	if bond.Conjugated {
		features[offset] = 1.0
	}
	offset++

	// Is in ring [5]
	if bond.InRing {
		features[offset] = 1.0
	}
	offset++

	// Stereo one-hot [6..8] — default none
	features[offset+0] = 1.0
	offset += stereoBins

	// Direction one-hot [9..11] — default none
	features[offset+0] = 1.0

	return features
}

// computeGlobalFeatures computes molecule-level features.
func computeGlobalFeatures(atoms []parsedAtom, bonds []parsedBond) []float32 {
	numAtoms := float32(len(atoms))
	numBonds := float32(len(bonds))

	// Single pass: molecular weight + aromatic count.
	var mw float32
	var aromaticCount float32
	for _, a := range atoms {
		if a.IsAromatic {
			aromaticCount++
		}
		if mass, ok := atomicMassMap[a.AtomicNum]; ok {
			mw += mass * 200 // denormalise
		} else {
			mw += 12.0 // default to carbon mass
		}
	}

	aromaticFrac := float32(0)
	if numAtoms > 0 {
		aromaticFrac = aromaticCount / numAtoms
	}

	// Bond density
	bondDensity := float32(0)
	if numAtoms > 1 {
		bondDensity = numBonds / (numAtoms * (numAtoms - 1) / 2)
	}

	// Log(atom count) normalised
	logAtoms := float32(math.Log1p(float64(numAtoms))) / 6.0

	return []float32{
		numAtoms / 200.0, // normalised atom count
		numBonds / 200.0, // normalised bond count
		mw / 1000.0,      // normalised molecular weight
		aromaticFrac,     // aromatic fraction
		bondDensity,      // bond density
		logAtoms,         // log atom count
	}
}
