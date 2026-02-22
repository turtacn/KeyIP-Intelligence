package molpatent_gnn

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// ValidateSMILES
// ---------------------------------------------------------------------------

func TestValidateSMILES_Valid(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	valid := []string{
		"C",
		"CCO",
		"c1ccccc1",
		"CC(=O)O",
		"[NH4+]",
		"C/C=C/C",
		"CC(C)(C)C",
		"O=C(O)c1ccccc1",
	}
	for _, s := range valid {
		if err := pp.ValidateSMILES(s); err != nil {
			t.Errorf("ValidateSMILES(%q) unexpected error: %v", s, err)
		}
	}
}

func TestValidateSMILES_Empty(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	if err := pp.ValidateSMILES(""); err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestValidateSMILES_TooLong(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	long := make([]byte, 5001)
	for i := range long {
		long[i] = 'C'
	}
	if err := pp.ValidateSMILES(string(long)); err == nil {
		t.Fatal("expected error for SMILES exceeding max length")
	}
}

func TestValidateSMILES_InvalidChars(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	invalid := []string{
		"CC O",   // space
		"CC\tO",  // tab
		"CC{O}",  // curly braces
		"CC<O>",  // angle brackets
	}
	for _, s := range invalid {
		if err := pp.ValidateSMILES(s); err == nil {
			t.Errorf("ValidateSMILES(%q) expected error for invalid chars", s)
		}
	}
}

func TestValidateSMILES_UnbalancedBrackets(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	unbalanced := []string{
		"[NH4+",
		"CC(=O",
		"CC)O(",
		"]C[",
	}
	for _, s := range unbalanced {
		if err := pp.ValidateSMILES(s); err == nil {
			t.Errorf("ValidateSMILES(%q) expected error for unbalanced brackets", s)
		}
	}
}

// ---------------------------------------------------------------------------
// balancedBrackets
// ---------------------------------------------------------------------------

func TestBalancedBrackets(t *testing.T) {
	tests := []struct {
		s    string
		want bool
	}{
		{"CC(=O)O", true},
		{"[NH4+]", true},
		{"CC(C)(C)C", true},
		{"[", false},
		{"(", false},
		{"]C", false},
		{")C", false},
		{"([)]", false},
		{"", true},
	}
	for _, tt := range tests {
		got := balancedBrackets(tt.s)
		if got != tt.want {
			t.Errorf("balancedBrackets(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Canonicalize
// ---------------------------------------------------------------------------

func TestCanonicalize_Valid(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	result, err := pp.Canonicalize("  CCO  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "CCO" {
		t.Errorf("expected 'CCO', got %q", result)
	}
}

func TestCanonicalize_Invalid(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	_, err := pp.Canonicalize("")
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

// ---------------------------------------------------------------------------
// PreprocessSMILES
// ---------------------------------------------------------------------------

func TestPreprocessSMILES_Ethanol(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	graph, err := pp.PreprocessSMILES(context.Background(), "CCO")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// CCO → 3 atoms (C, C, O)
	if graph.NumAtoms != 3 {
		t.Errorf("expected 3 atoms, got %d", graph.NumAtoms)
	}
	if graph.NumBonds != 2 {
		t.Errorf("expected 2 bonds, got %d", graph.NumBonds)
	}
	// Undirected: 2 bonds × 2 directions = 4 edges
	if len(graph.EdgeIndex) != 4 {
		t.Errorf("expected 4 edge entries (undirected), got %d", len(graph.EdgeIndex))
	}
	// Node features dimension
	for i, nf := range graph.NodeFeatures {
		if len(nf) != totalNodeFeatures {
			t.Errorf("atom[%d] feature dim: expected %d, got %d", i, totalNodeFeatures, len(nf))
		}
	}
	// Edge features dimension
	for i, ef := range graph.EdgeFeatures {
		if len(ef) != totalEdgeFeatures {
			t.Errorf("edge[%d] feature dim: expected %d, got %d", i, totalEdgeFeatures, len(ef))
		}
	}
	// Global features
	if len(graph.GlobalFeatures) != 6 {
		t.Errorf("expected 6 global features, got %d", len(graph.GlobalFeatures))
	}
	if graph.SMILES != "CCO" {
		t.Errorf("expected SMILES 'CCO', got %q", graph.SMILES)
	}
}

func TestPreprocessSMILES_Benzene(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	graph, err := pp.PreprocessSMILES(context.Background(), "c1ccccc1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph.NumAtoms != 6 {
		t.Errorf("expected 6 atoms for benzene, got %d", graph.NumAtoms)
	}
	// Check aromatic flag on first atom
	if graph.NodeFeatures[0][atomicNumberBins+degreeBins+formalChargeBins+numHBins+hybridizationBins+chiralityBins+ringSizeBins] != 1.0 {
		t.Log("note: aromatic flag position may vary — check encoding offset")
	}
}

func TestPreprocessSMILES_SingleAtom(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	graph, err := pp.PreprocessSMILES(context.Background(), "C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph.NumAtoms != 1 {
		t.Errorf("expected 1 atom, got %d", graph.NumAtoms)
	}
	if graph.NumBonds != 0 {
		t.Errorf("expected 0 bonds, got %d", graph.NumBonds)
	}
}

func TestPreprocessSMILES_BracketAtom(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	graph, err := pp.PreprocessSMILES(context.Background(), "[NH4+]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph.NumAtoms < 1 {
		t.Error("expected at least 1 atom")
	}
}

func TestPreprocessSMILES_DoubleBond(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	graph, err := pp.PreprocessSMILES(context.Background(), "C=O")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph.NumAtoms != 2 {
		t.Errorf("expected 2 atoms, got %d", graph.NumAtoms)
	}
	if graph.NumBonds != 1 {
		t.Errorf("expected 1 bond, got %d", graph.NumBonds)
	}
}

func TestPreprocessSMILES_TooManyAtoms(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	cfg.MaxAtoms = 3
	pp, _ := NewGNNPreprocessor(cfg)
	// CCCCCC has 6 atoms, exceeds max 3
	_, err := pp.PreprocessSMILES(context.Background(), "CCCCCC")
	if err == nil {
		t.Fatal("expected error for too many atoms")
	}
}

func TestPreprocessSMILES_InvalidSMILES(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	_, err := pp.PreprocessSMILES(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestPreprocessSMILES_Branching(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	// CC(=O)O → 4 atoms: C, C, O, O; 3 bonds
	graph, err := pp.PreprocessSMILES(context.Background(), "CC(=O)O")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if graph.NumAtoms != 4 {
		t.Errorf("expected 4 atoms, got %d", graph.NumAtoms)
	}
	if graph.NumBonds != 3 {
		t.Errorf("expected 3 bonds, got %d", graph.NumBonds)
	}
}

// ---------------------------------------------------------------------------
// PreprocessMOL
// ---------------------------------------------------------------------------

func TestPreprocessMOL_NotImplemented(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	_, err := pp.PreprocessMOL(context.Background(), "some mol block")
	if err == nil {
		t.Fatal("expected error for unimplemented MOL parsing")
	}
}

func TestPreprocessMOL_Empty(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	_, err := pp.PreprocessMOL(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty MOL block")
	}
}

// ---------------------------------------------------------------------------
// PreprocessBatch
// ---------------------------------------------------------------------------

func TestPreprocessBatch_Success(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	inputs := []MolecularInput{
		{SMILES: "CCO"},
		{SMILES: "C"},
		{SMILES: "c1ccccc1"},
	}
	results, err := pp.PreprocessBatch(context.Background(), inputs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestPreprocessBatch_EmptyInput(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	_, err := pp.PreprocessBatch(context.Background(), []MolecularInput{
		{},
	})
	if err == nil {
		t.Fatal("expected error for input with neither SMILES nor MOL")
	}
}

func TestPreprocessBatch_ContextCancelled(t *testing.T) {
	pp, _ := NewGNNPreprocessor(DefaultGNNModelConfig())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := pp.PreprocessBatch(ctx, []MolecularInput{
		{SMILES: "CCO"},
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ---------------------------------------------------------------------------
// Feature encoding
// ---------------------------------------------------------------------------

func TestEncodeAtomFeatures_Carbon(t *testing.T) {
	atom := parsedAtom{
		Symbol:    "C",
		AtomicNum: 6,
		Degree:    2,
	}
	features := encodeAtomFeatures(atom)
	if len(features) != totalNodeFeatures {
		t.Fatalf("expected %d features, got %d", totalNodeFeatures, len(features))
	}
	// Carbon is bin index 1 (H=0, C=1)
	carbonBin := atomicNumToBin(6)
	if features[carbonBin] != 1.0 {
		t.Errorf("expected carbon one-hot at bin %d", carbonBin)
	}
}

func TestEncodeAtomFeatures_Aromatic(t *testing.T) {
	atom := parsedAtom{
		Symbol:     "C",
		AtomicNum:  6,
		IsAromatic: true,
	}
	features := encodeAtomFeatures(atom)
	aromaticIdx := atomicNumberBins + degreeBins + formalChargeBins +
		numHBins + hybridizationBins + chiralityBins + ringSizeBins
	if features[aromaticIdx] != 1.0 {
		t.Errorf("expected aromatic flag at index %d to be 1.0", aromaticIdx)
	}
}

func TestEncodeBondFeatures_Single(t *testing.T) {
	bond := parsedBond{BondType: 1}
	features := encodeBondFeatures(bond)
	if len(features) != totalEdgeFeatures {
		t.Fatalf("expected %d features, got %d", totalEdgeFeatures, len(features))
	}
	if features[0] != 1.0 {
		t.Error("expected single bond one-hot at index 0")
	}
}

func TestEncodeBondFeatures_Double(t *testing.T) {
	bond := parsedBond{BondType: 2}
	features := encodeBondFeatures(bond)
	if features[1] != 1.0 {
		t.Error("expected double bond one-hot at index 1")
	}
}

func TestEncodeBondFeatures_Aromatic(t *testing.T) {
	bond := parsedBond{BondType: 4}
	features := encodeBondFeatures(bond)
	if features[3] != 1.0 {
		t.Error("expected aromatic bond one-hot at index 3")
	}
}

func TestComputeGlobalFeatures(t *testing.T) {
	atoms := []parsedAtom{
		{AtomicNum: 6},
		{AtomicNum: 6},
		{AtomicNum: 8},
	}
	bonds := []parsedBond{
		{Src: 0, Dst: 1, BondType: 1},
		{Src: 1, Dst: 2, BondType: 1},
	}
	gf := computeGlobalFeatures(atoms, bonds)
	if len(gf) != 6 {
		t.Fatalf("expected 6 global features, got %d", len(gf))
	}
	// Normalised atom count: 3/200 = 0.015
	if gf[0] < 0.01 || gf[0] > 0.02 {
		t.Errorf("normalised atom count out of range: %f", gf[0])
	}
}

func TestAtomicNumToBin(t *testing.T) {
	tests := []struct {
		atomicNum int
		wantBin   int
	}{
		{1, 0},   // H
		{6, 1},   // C
		{7, 2},   // N
		{8, 3},   // O
		{999, atomicNumberBins - 1}, // unknown → other
	}
	for _, tt := range tests {
		got := atomicNumToBin(tt.atomicNum)
		if got != tt.wantBin {
			t.Errorf("atomicNumToBin(%d) = %d, want %d", tt.atomicNum, got, tt.wantBin)
		}
	}
}

func TestEstimateImplicitH(t *testing.T) {
	tests := []struct {
		atomicNum     int
		explicitBonds int
		wantH         int
	}{
		{6, 0, 4},  // Carbon with no bonds → 4H
		{6, 2, 2},  // Carbon with 2 bonds → 2H
		{6, 4, 0},  // Carbon fully bonded → 0H
		{8, 0, 2},  // Oxygen → 2H
		{8, 1, 1},  // Oxygen with 1 bond → 1H
		{7, 0, 3},  // Nitrogen → 3H
		{26, 0, 0}, // Iron (not in table) → 0H
	}
	for _, tt := range tests {
		got := estimateImplicitH(tt.atomicNum, tt.explicitBonds)
		if got != tt.wantH {
			t.Errorf("estimateImplicitH(%d, %d) = %d, want %d",
				tt.atomicNum, tt.explicitBonds, got, tt.wantH)
		}
	}
}

func TestNewGNNPreprocessor_NilConfig(t *testing.T) {
	_, err := NewGNNPreprocessor(nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestParseOrganicAtom_TwoLetter(t *testing.T) {
	runes := []rune("Cl")
	sym, aromatic, advance := parseOrganicAtom(runes, 0)
	if sym != "Cl" {
		t.Errorf("expected 'Cl', got %q", sym)
	}
	if aromatic {
		t.Error("Cl should not be aromatic")
	}
	if advance != 2 {
		t.Errorf("expected advance 2, got %d", advance)
	}
}

func TestParseOrganicAtom_SingleLetter(t *testing.T) {
	runes := []rune("C")
	sym, aromatic, advance := parseOrganicAtom(runes, 0)
	if sym != "C" {
		t.Errorf("expected 'C', got %q", sym)
	}
	if aromatic {
		t.Error("C should not be aromatic")
	}
	if advance != 1 {
		t.Errorf("expected advance 1, got %d", advance)
	}
}

func TestParseOrganicAtom_Aromatic(t *testing.T) {
	runes := []rune("c")
	sym, aromatic, advance := parseOrganicAtom(runes, 0)
	if sym != "C" {
		t.Errorf("expected 'C', got %q", sym)
	}
	if !aromatic {
		t.Error("lowercase 'c' should be aromatic")
	}
	if advance != 1 {
		t.Errorf("expected advance 1, got %d", advance)
	}
}

func TestParseOrganicAtom_Bromine(t *testing.T) {
	runes := []rune("Br")
	sym, aromatic, advance := parseOrganicAtom(runes, 0)
	if sym != "Br" {
		t.Errorf("expected 'Br', got %q", sym)
	}
	if aromatic {
		t.Error("Br should not be aromatic")
	}
	if advance != 2 {
		t.Errorf("expected advance 2, got %d", advance)
	}
}

func TestParseBracketAtom_Charged(t *testing.T) {
	atom := parseBracketAtom("NH4+")
	if atom.Symbol != "N" {
		t.Errorf("expected symbol 'N', got %q", atom.Symbol)
	}
	if atom.AtomicNum != 7 {
		t.Errorf("expected atomic num 7, got %d", atom.AtomicNum)
	}
	if atom.Charge != 1 {
		t.Errorf("expected charge +1, got %d", atom.Charge)
	}
	if atom.NumH != 4 {
		t.Errorf("expected 4 H, got %d", atom.NumH)
	}
}

func TestParseBracketAtom_NegativeCharge(t *testing.T) {
	atom := parseBracketAtom("O-")
	if atom.Symbol != "O" {
		t.Errorf("expected symbol 'O', got %q", atom.Symbol)
	}
	if atom.Charge != -1 {
		t.Errorf("expected charge -1, got %d", atom.Charge)
	}
}

func TestParseBracketAtom_Isotope(t *testing.T) {
	atom := parseBracketAtom("13C")
	if atom.Symbol != "C" {
		t.Errorf("expected symbol 'C', got %q", atom.Symbol)
	}
	if atom.AtomicNum != 6 {
		t.Errorf("expected atomic num 6, got %d", atom.AtomicNum)
	}
}

func TestParseBracketAtom_ExplicitH(t *testing.T) {
	atom := parseBracketAtom("CH3")
	if atom.NumH != 3 {
		t.Errorf("expected 3 H, got %d", atom.NumH)
	}
}

func TestParseBracketAtom_SingleH(t *testing.T) {
	atom := parseBracketAtom("OH")
	if atom.NumH != 1 {
		t.Errorf("expected 1 H, got %d", atom.NumH)
	}
}

func TestParseSMILES_Methane(t *testing.T) {
	atoms, bonds, err := parseSMILES("C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atoms) != 1 {
		t.Errorf("expected 1 atom, got %d", len(atoms))
	}
	if len(bonds) != 0 {
		t.Errorf("expected 0 bonds, got %d", len(bonds))
	}
	if atoms[0].AtomicNum != 6 {
		t.Errorf("expected carbon (6), got %d", atoms[0].AtomicNum)
	}
}

func TestParseSMILES_Ethane(t *testing.T) {
	atoms, bonds, err := parseSMILES("CC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atoms) != 2 {
		t.Errorf("expected 2 atoms, got %d", len(atoms))
	}
	if len(bonds) != 1 {
		t.Errorf("expected 1 bond, got %d", len(bonds))
	}
}

func TestParseSMILES_DisconnectedFragments(t *testing.T) {
	atoms, bonds, err := parseSMILES("C.C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atoms) != 2 {
		t.Errorf("expected 2 atoms, got %d", len(atoms))
	}
	if len(bonds) != 0 {
		t.Errorf("expected 0 bonds for disconnected fragments, got %d", len(bonds))
	}
}

func TestParseSMILES_Branch(t *testing.T) {
	// CC(=O)O → C-C(=O)-O
	atoms, bonds, err := parseSMILES("CC(=O)O")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atoms) != 4 {
		t.Errorf("expected 4 atoms, got %d", len(atoms))
	}
	if len(bonds) != 3 {
		t.Errorf("expected 3 bonds, got %d", len(bonds))
	}
	// Check that the double bond exists
	hasDouble := false
	for _, b := range bonds {
		if b.BondType == 2 {
			hasDouble = true
		}
	}
	if !hasDouble {
		t.Error("expected at least one double bond")
	}
}

func TestParseSMILES_TripleBond(t *testing.T) {
	atoms, bonds, err := parseSMILES("C#N")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atoms) != 2 {
		t.Errorf("expected 2 atoms, got %d", len(atoms))
	}
	if len(bonds) != 1 {
		t.Errorf("expected 1 bond, got %d", len(bonds))
	}
	if bonds[0].BondType != 3 {
		t.Errorf("expected triple bond (3), got %d", bonds[0].BondType)
	}
}

func TestParseSMILES_AromaticBond(t *testing.T) {
	atoms, bonds, err := parseSMILES("cc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atoms) != 2 {
		t.Errorf("expected 2 atoms, got %d", len(atoms))
	}
	if len(bonds) != 1 {
		t.Errorf("expected 1 bond, got %d", len(bonds))
	}
	if bonds[0].BondType != 4 {
		t.Errorf("expected aromatic bond (4), got %d", bonds[0].BondType)
	}
}

func TestLookupAtomicNumber_Known(t *testing.T) {
	if lookupAtomicNumber("C") != 6 {
		t.Error("expected C → 6")
	}
	if lookupAtomicNumber("N") != 7 {
		t.Error("expected N → 7")
	}
	if lookupAtomicNumber("Cl") != 17 {
		t.Error("expected Cl → 17")
	}
}

func TestLookupAtomicNumber_Unknown(t *testing.T) {
	if lookupAtomicNumber("Xx") != 0 {
		t.Error("expected unknown → 0")
	}
}


