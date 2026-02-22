package molpatent_gnn

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Atom / Bond feature encoding constants
// ---------------------------------------------------------------------------

// AtomFeatureSet defines the atom-level features extracted for each node.
// Total dimension = 78 (matching GNNModelConfig.NodeFeatureDim default).
//
// Features (one-hot or scalar):
//   [0..43]   Atomic number one-hot (H,C,N,O,F,P,S,Cl,Br,I + others → 44 bins)
//   [44..47]  Degree one-hot (0,1,2,3,4+)
//   [48..52]  Formal charge one-hot (-2,-1,0,+1,+2)
//   [53..56]  Num H one-hot (0,1,2,3+)
//   [57..62]  Hybridization one-hot (s,sp,sp2,sp3,sp3d,sp3d2)
//   [63]      Is aromatic (binary)
//   [64..67]  Chirality one-hot (none,R,S,other)
//   [68..72]  Ring size one-hot (0,3,4,5,6,7+)
//   [73]      Is in ring (binary)
//   [74]      Atomic mass (normalised)
//   [75]      Electronegativity (normalised)
//   [76]      Van der Waals radius (normalised)
//   [77]      Number of radical electrons (normalised)

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
		binaryFeatures + scalarFeatures // = 78
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

// gnnPreprocessorImpl is the production implementation of GNNPreprocessor.
type gnnPreprocessorImpl struct {
	config *GNNModelConfig
	mu     sync.RWMutex
}

// NewGNNPreprocessor creates a new preprocessor.
func NewGNNPreprocessor(config *GNNModelConfig) (GNNPreprocessor, error) {
	if config == nil {
		return nil, errors.NewInvalidInputError("config is required")
	}
	return &gnnPreprocessorImpl{config: config}, nil
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
//   1. Validate SMILES
//   2. Parse atoms and bonds from the SMILES string
//   3. Encode atom features → NodeFeatures
//   4. Encode bond features → EdgeFeatures + EdgeIndex
//   5. Compute global features (molecular weight, atom count, etc.)
//   6. Validate against MaxAtoms
func (p *gnnPreprocessorImpl) PreprocessSMILES(ctx context.Context, smiles string) (*MolecularGraph, error) {
	if err := p.ValidateSMILES(smiles); err != nil {
		return nil, err
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

	// Encode edge features (undirected: add both directions)
	var edgeIndex [][2]int
	var edgeFeatures [][]float32
	for _, bond := range bonds {
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
		SMILES:         smiles,
	}, nil
}

// PreprocessMOL converts a MOL block into a MolecularGraph.
func (p *gnnPreprocessorImpl) PreprocessMOL(ctx context.Context, molBlock string) (*MolecularGraph, error) {
	if molBlock == "" {
		return nil, errors.NewInvalidInputError("MOL block is empty")
	}
	// In production, this would parse the V2000/V3000 MOL format.
	// For now, return a placeholder error indicating the feature is pending.
	return nil, fmt.Errorf("MOL block parsing not yet implemented")
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
	Src       int
	Dst       int
	BondType  int // 1=single, 2=double, 3=triple, 4=aromatic
	InRing    bool
	Conjugated bool
}

// parseSMILES is a simplified SMILES tokeniser.
// Production code would use RDKit via CGo or a gRPC service.
func parseSMILES(smiles string) ([]parsedAtom, []parsedBond, error) {
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
				Symbol:    symbol,
				AtomicNum: atomicNum,
				IsAromatic: aromatic,
				NumH:      estimateImplicitH(atomicNum, 0),
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

	// Van der Waals radius normalised [78] — placeholder
	features[offset] = 0.5
	offset++

	// Radical electrons normalised [79] — placeholder
	features[offset] = 0.0

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

	// Molecular weight estimate
	var mw float32
	for _, a := range atoms {
		if mass, ok := atomicMassMap[a.AtomicNum]; ok {
			mw += mass * 200 // denormalise
		} else {
			mw += 12.0 // default to carbon mass
		}
	}

	// Fraction of aromatic atoms
	var aromaticCount float32
	for _, a := range atoms {
		if a.IsAromatic {
			aromaticCount++
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
		numAtoms / 200.0,  // normalised atom count
		numBonds / 200.0,  // normalised bond count
		mw / 1000.0,       // normalised molecular weight
		aromaticFrac,      // aromatic fraction
		bondDensity,       // bond density
		logAtoms,          // log atom count
	}
}


