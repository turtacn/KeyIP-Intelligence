package molpatent_gnn

import (
	"context"
	"fmt"
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
// Total dimension = 39 (matching GNNModelConfig.NodeFeatureDim).
//
// Features (one-hot or scalar):
//   [0..9]    Atomic type one-hot (C, N, O, S, P, F, Cl, Br, I, Other) -> 10
//   [10..14]  Hybridization one-hot (SP, SP2, SP3, SP3D, SP3D2) -> 5
//   [15..19]  Formal charge one-hot (-2, -1, 0, 1, 2) -> 5
//   [20]      Is aromatic (binary) -> 1
//   [21..25]  Num H one-hot (0, 1, 2, 3, 4) -> 5
//   [26]      Is in ring (binary) -> 1
//   [27..32]  Degree one-hot (0, 1, 2, 3, 4, 5) -> 6
//   [33..35]  Chirality one-hot (R, S, None) -> 3
//   [36..38]  Radical electrons one-hot (0, 1, 2) -> 3

const (
	atomTypeBins      = 10
	hybridizationBins = 5
	formalChargeBins  = 5
	aromaticBins      = 1
	numHBins          = 5
	inRingBins        = 1
	degreeBins        = 6
	chiralityBins     = 3
	radicalBins       = 3
	totalNodeFeatures = 39
)

// BondFeatureSet defines the bond-level features for each edge.
// Total dimension = 10 (matching GNNModelConfig.EdgeFeatureDim).
//
// Features:
//   [0..3]  Bond type one-hot (Single, Double, Triple, Aromatic) -> 4
//   [4]     Is conjugated (binary) -> 1
//   [5]     Is in ring (binary) -> 1
//   [6..8]  Stereo one-hot (E, Z, None) -> 3
//   [9]     Is rotatable (binary) -> 1

const (
	bondTypeBins      = 4
	conjugatedBins    = 1
	bondInRingBins    = 1
	stereoBins        = 3
	rotatableBins     = 1
	totalEdgeFeatures = 10
)

// ---------------------------------------------------------------------------
// Atom property tables
// ---------------------------------------------------------------------------

// atomicNumberMap maps element symbols to atomic numbers.
var atomicNumberMap = map[string]int{
	"H": 1, "Be": 4, "B": 5, "C": 6, "N": 7, "O": 8,
	"F": 9, "Si": 14, "P": 15, "S": 16, "Cl": 17,
	"Br": 35, "I": 53,
}

// atomicMassMap maps atomic number to atomic mass (normalised by 200).
var atomicMassMap = map[int]float32{
	1: 1.008 / 200, 6: 12.011 / 200, 7: 14.007 / 200, 8: 15.999 / 200,
	9: 18.998 / 200, 15: 30.974 / 200, 16: 32.06 / 200, 17: 35.45 / 200,
	35: 79.904 / 200, 53: 126.90 / 200,
}

// ---------------------------------------------------------------------------
// SMILES validation
// ---------------------------------------------------------------------------

var smilesPattern = regexp.MustCompile(
	`^[A-Za-z0-9@+\-\[\]()=#$:/\\.%]+$`,
)

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

type gnnPreprocessorImpl struct {
	config *GNNModelConfig
	mu     sync.RWMutex
}

func NewGNNPreprocessor(config *GNNModelConfig) (GNNPreprocessor, error) {
	if config == nil {
		return nil, errors.NewInvalidInputError("config is required")
	}
	return &gnnPreprocessorImpl{config: config}, nil
}

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

func (p *gnnPreprocessorImpl) Canonicalize(smiles string) (string, error) {
	canonical := strings.TrimSpace(smiles)
	if err := p.ValidateSMILES(canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

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

	nodeFeatures := make([][]float32, len(atoms))
	for i, atom := range atoms {
		nodeFeatures[i] = encodeAtomFeatures(atom)
	}

	var edgeIndex [][2]int
	var edgeFeatures [][]float32
	for _, bond := range bonds {
		ef := encodeBondFeatures(bond)
		edgeIndex = append(edgeIndex, [2]int{bond.Src, bond.Dst})
		edgeFeatures = append(edgeFeatures, ef)
		edgeIndex = append(edgeIndex, [2]int{bond.Dst, bond.Src})
		edgeFeatures = append(edgeFeatures, ef)
	}

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

func (p *gnnPreprocessorImpl) PreprocessMOL(ctx context.Context, molBlock string) (*MolecularGraph, error) {
	if molBlock == "" {
		return nil, errors.NewInvalidInputError("MOL block is empty")
	}
	return nil, fmt.Errorf("MOL block parsing not yet implemented")
}

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

type parsedAtom struct {
	Symbol     string
	AtomicNum  int
	IsAromatic bool
	Charge     int
	NumH       int
	Degree     int
	Hybridization int // 0=SP, 1=SP2, 2=SP3, 3=SP3D, 4=SP3D2
	Chirality     int // 0=None, 1=R, 2=S
	Radical       int // 0, 1, 2
	InRing        bool
}

type parsedBond struct {
	Src       int
	Dst       int
	BondType  int // 1=single, 2=double, 3=triple, 4=aromatic
	InRing    bool
	Conjugated bool
	Stereo    int // 0=None, 1=E, 2=Z
}

func parseSMILES(smiles string) ([]parsedAtom, []parsedBond, error) {
	// Simplified parser - reuses existing logic but adapted for new features
	var atoms []parsedAtom
	var bonds []parsedBond

	runes := []rune(smiles)
	i := 0
	atomStack := []int{}
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
			j := i + 1
			for j < len(runes) && runes[j] != ']' {
				j++
			}
			if j >= len(runes) {
				return nil, nil, fmt.Errorf("unclosed bracket at position %d", i)
			}
			bracketContent := string(runes[i+1 : j])
			atom := parseBracketAtom(bracketContent)
			// Simple heuristic for hybridization/ring/etc
			atom.Hybridization = estimateHybridization(atom.AtomicNum, atom.Degree, atom.IsAromatic)

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
		case ch == '.':
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
				InRing:    aromatic, // simplified
			}
			// Heuristics
			atom.Hybridization = estimateHybridization(atomicNum, 0, aromatic) // will update degree later if needed

			atomIdx := len(atoms)
			atoms = append(atoms, atom)

			bondType := nextBondType
			if aromatic && prevAtom >= 0 && atoms[prevAtom].IsAromatic {
				bondType = 4
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

	// Refine heuristics after graph is built
	refineAtomProperties(atoms, bonds)

	return atoms, bonds, nil
}

func parseOrganicAtom(runes []rune, i int) (string, bool, int) {
	ch := runes[i]
	aromatic := unicode.IsLower(ch)
	upper := unicode.ToUpper(ch)
	if i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
		twoLetter := string([]rune{upper, runes[i+1]})
		if _, ok := atomicNumberMap[twoLetter]; ok {
			return twoLetter, false, 2
		}
	}
	return string(upper), aromatic, 1
}

func parseBracketAtom(content string) parsedAtom {
	atom := parsedAtom{}
	runes := []rune(content)
	idx := 0
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
	if hIdx := strings.Index(rest, "H"); hIdx >= 0 {
		if hIdx+1 < len(rest) && rest[hIdx+1] >= '0' && rest[hIdx+1] <= '9' {
			atom.NumH = int(rest[hIdx+1] - '0')
		} else {
			atom.NumH = 1
		}
	}
	// Simplified chirality handling: @ -> 1 (R), @@ -> 2 (S)
	if strings.Contains(rest, "@@") {
		atom.Chirality = 2
	} else if strings.Contains(rest, "@") {
		atom.Chirality = 1
	}
	return atom
}

func lookupAtomicNumber(symbol string) int {
	if n, ok := atomicNumberMap[symbol]; ok {
		return n
	}
	return 0
}

func estimateImplicitH(atomicNum int, explicitBonds int) int {
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

func estimateHybridization(atomicNum, degree int, aromatic bool) int {
	// Simple heuristics: 0=SP, 1=SP2, 2=SP3, 3=SP3D, 4=SP3D2
	if aromatic {
		return 1 // SP2
	}
	// Default to SP3 for standard organic atoms
	return 2
}

func refineAtomProperties(atoms []parsedAtom, bonds []parsedBond) {
	// Refine ring membership and hybridization based on bonds
	// This is a placeholder for more complex logic
	for _, b := range bonds {
		if b.BondType == 4 { // Aromatic implies ring
			atoms[b.Src].InRing = true
			atoms[b.Dst].InRing = true
		}
	}
}

// ---------------------------------------------------------------------------
// Feature encoding
// ---------------------------------------------------------------------------

func encodeAtomFeatures(atom parsedAtom) []float32 {
	features := make([]float32, totalNodeFeatures)
	offset := 0

	// 1. Atom Type (10)
	// C, N, O, S, P, F, Cl, Br, I, Other
	typeIdx := 9 // Other
	switch atom.AtomicNum {
	case 6: typeIdx = 0 // C
	case 7: typeIdx = 1 // N
	case 8: typeIdx = 2 // O
	case 16: typeIdx = 3 // S
	case 15: typeIdx = 4 // P
	case 9: typeIdx = 5 // F
	case 17: typeIdx = 6 // Cl
	case 35: typeIdx = 7 // Br
	case 53: typeIdx = 8 // I
	}
	features[offset+typeIdx] = 1.0
	offset += atomTypeBins

	// 2. Hybridization (5)
	hyb := atom.Hybridization
	if hyb >= hybridizationBins { hyb = hybridizationBins - 1 }
	features[offset+hyb] = 1.0
	offset += hybridizationBins

	// 3. Formal Charge (5): -2, -1, 0, 1, 2
	chg := atom.Charge + 2
	if chg < 0 { chg = 0 }
	if chg >= formalChargeBins { chg = formalChargeBins - 1 }
	features[offset+chg] = 1.0
	offset += formalChargeBins

	// 4. Aromatic (1)
	if atom.IsAromatic {
		features[offset] = 1.0
	}
	offset += aromaticBins

	// 5. Num H (5): 0, 1, 2, 3, 4
	nh := atom.NumH
	if nh >= numHBins { nh = numHBins - 1 }
	features[offset+nh] = 1.0
	offset += numHBins

	// 6. In Ring (1)
	if atom.InRing {
		features[offset] = 1.0
	}
	offset += inRingBins

	// 7. Degree (6): 0, 1, 2, 3, 4, 5
	deg := atom.Degree
	if deg >= degreeBins { deg = degreeBins - 1 }
	features[offset+deg] = 1.0
	offset += degreeBins

	// 8. Chirality (3): R, S, None
	// Mapping: 0=None -> idx 2, 1=R -> idx 0, 2=S -> idx 1 (arbitrary but consistent)
	chir := 2 // None
	if atom.Chirality == 1 { chir = 0 } // R
	if atom.Chirality == 2 { chir = 1 } // S
	features[offset+chir] = 1.0
	offset += chiralityBins

	// 9. Radical (3): 0, 1, 2
	rad := atom.Radical
	if rad >= radicalBins { rad = radicalBins - 1 }
	features[offset+rad] = 1.0

	return features
}

func encodeBondFeatures(bond parsedBond) []float32 {
	features := make([]float32, totalEdgeFeatures)
	offset := 0

	// 1. Bond Type (4)
	bt := bond.BondType - 1
	if bt < 0 { bt = 0 }
	if bt >= bondTypeBins { bt = bondTypeBins - 1 }
	features[offset+bt] = 1.0
	offset += bondTypeBins

	// 2. Conjugated (1)
	if bond.Conjugated {
		features[offset] = 1.0
	}
	offset += conjugatedBins

	// 3. In Ring (1)
	if bond.InRing {
		features[offset] = 1.0
	}
	offset += bondInRingBins

	// 4. Stereo (3): E, Z, None
	// 0=None -> idx 2, 1=E -> idx 0, 2=Z -> idx 1
	st := 2
	if bond.Stereo == 1 { st = 0 }
	if bond.Stereo == 2 { st = 1 }
	features[offset+st] = 1.0
	offset += stereoBins

	// 5. Rotatable (1)
	// Heuristic: Single bond, not in ring
	if bond.BondType == 1 && !bond.InRing {
		features[offset] = 1.0
	}

	return features
}

func computeGlobalFeatures(atoms []parsedAtom, bonds []parsedBond) []float32 {
	numAtoms := float32(len(atoms))
	numBonds := float32(len(bonds))

	var mw float32
	for _, a := range atoms {
		if mass, ok := atomicMassMap[a.AtomicNum]; ok {
			mw += mass * 200
		} else {
			mw += 12.0
		}
	}

	return []float32{
		numAtoms / 200.0,
		numBonds / 200.0,
		mw / 1000.0,
	}
}
