package chem_extractor

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func makeRawEntity(text string, entityType ChemicalEntityType, confidence float64, ctxText string) *RawChemicalEntity {
	return &RawChemicalEntity{
		Text:       text,
		EntityType: entityType,
		Confidence: confidence,
		Context:    ctxText,
		StartPos:   0,
		EndPos:     len(text),
	}
}

func newValidator() EntityValidator {
	return NewEntityValidator()
}

// ---------------------------------------------------------------------------
// CAS Number tests
// ---------------------------------------------------------------------------

func TestValidate_CASNumber_ValidCheckDigit(t *testing.T) {
	v := newValidator()
	// Aspirin: 50-78-2
	// digits = "5078", check = 2
	// sum = 8*1 + 7*2 + 0*3 + 5*4 = 8+14+0+20 = 42, 42%10 = 2 ✓
	entity := makeRawEntity("50-78-2", EntityCASNumber, 0.80, "the compound 50-78-2 is aspirin")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid, issues: %v", result.Issues)
	}
	if result.AdjustedConfidence <= 0.80 {
		t.Errorf("expected confidence boost, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_CASNumber_InvalidCheckDigit(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("50-78-3", EntityCASNumber, 0.80, "compound 50-78-3")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for wrong check digit")
	}
}

func TestValidate_CASNumber_InvalidFormat(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("5078-2", EntityCASNumber, 0.80, "compound")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for bad CAS format")
	}
}

func TestValidate_CASNumber_ValidFormat_LongNumber(t *testing.T) {
	v := newValidator()
	// 9002-93-1: digits = "900293", check = 1
	// sum = 3*1 + 9*2 + 2*3 + 0*4 + 0*5 + 9*6 = 3+18+6+0+0+54 = 81, 81%10 = 1 ✓
	entity := makeRawEntity("9002-93-1", EntityCASNumber, 0.75, "the molecule 9002-93-1")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid for 9002-93-1, issues: %v", result.Issues)
	}
}

// ---------------------------------------------------------------------------
// SMILES tests
// ---------------------------------------------------------------------------

func TestValidate_SMILES_Valid(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("CC(=O)Oc1ccccc1C(=O)O", EntitySMILES, 0.75, "the compound structure CC(=O)Oc1ccccc1C(=O)O")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid SMILES, issues: %v", result.Issues)
	}
	if result.AdjustedConfidence <= 0.75 {
		t.Errorf("expected confidence boost, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_SMILES_UnbalancedParentheses(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("CC(=O", EntitySMILES, 0.80, "molecule CC(=O")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for unbalanced parentheses")
	}
	foundIssue := false
	for _, iss := range result.Issues {
		if contains(iss, "parentheses") {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Error("expected parentheses issue in Issues")
	}
}

func TestValidate_SMILES_InvalidAtom(t *testing.T) {
	v := newValidator()
	// X is not a valid organic subset atom outside brackets
	entity := makeRawEntity("CXC", EntitySMILES, 0.80, "molecule CXC")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for atom X outside brackets")
	}
}

func TestValidate_SMILES_UnmatchedRingClosure(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("C1CCC", EntitySMILES, 0.80, "molecule C1CCC")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for unmatched ring closure")
	}
}

// ---------------------------------------------------------------------------
// Molecular Formula tests
// ---------------------------------------------------------------------------

func TestValidate_MolecularFormula_Valid(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("C9H8O4", EntityMolecularFormula, 0.80, "molecular formula C9H8O4")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid formula, issues: %v", result.Issues)
	}
	if result.AdjustedConfidence <= 0.80 {
		t.Errorf("expected confidence boost, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_MolecularFormula_InvalidElement(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("C9H8Xx4", EntityMolecularFormula, 0.80, "formula")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for unknown element Xx")
	}
}

func TestValidate_MolecularFormula_UnreasonableCount(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("C99999H8O4", EntityMolecularFormula, 0.80, "formula")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for unreasonable atom count")
	}
}

func TestValidate_MolecularFormula_LowerCase(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("c9h8o4", EntityMolecularFormula, 0.80, "formula")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for lowercase formula")
	}
}

// ---------------------------------------------------------------------------
// IUPAC Name tests
// ---------------------------------------------------------------------------

func TestValidate_IUPACName_Valid(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("2-acetoxybenzoic acid", EntityIUPACName, 0.75, "the compound 2-acetoxybenzoic acid")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid IUPAC name, issues: %v", result.Issues)
	}
	if result.AdjustedConfidence <= 0.75 {
		t.Errorf("expected confidence boost, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_IUPACName_TooShort(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("ab", EntityIUPACName, 0.80, "compound ab")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for too-short IUPAC name")
	}
}

func TestValidate_IUPACName_PureNumber(t *testing.T) {
	v := newValidator()
	// Pure digits are blacklisted before type-specific validation
	entity := makeRawEntity("12345", EntityIUPACName, 0.80, "compound 12345")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for pure number")
	}
}

func TestValidate_IUPACName_NoChemicalSuffix(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("something", EntityIUPACName, 0.80, "the compound something is used")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Still valid (IUPAC validation doesn't reject on missing suffix alone)
	// but confidence should not be boosted by the suffix check
	if result.AdjustedConfidence > 0.90 {
		t.Errorf("expected no suffix boost, got %f", result.AdjustedConfidence)
	}
	foundIssue := false
	for _, iss := range result.Issues {
		if contains(iss, "suffix") {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Error("expected suffix issue in Issues")
	}
}

// ---------------------------------------------------------------------------
// Common Name tests
// ---------------------------------------------------------------------------

func TestValidate_CommonName_KnownDrug(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("aspirin", EntityCommonName, 0.75, "the drug aspirin is a compound")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid for known drug, issues: %v", result.Issues)
	}
	if result.AdjustedConfidence <= 0.75 {
		t.Errorf("expected confidence boost for known drug, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_CommonName_CommonWord(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("method", EntityCommonName, 0.80, "the method of claim 1")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for blacklisted word 'method'")
	}
}

func TestValidate_CommonName_UnknownName(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("xyzabc", EntityCommonName, 0.75, "the compound xyzabc was synthesized")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Error("expected valid (unknown but not blacklisted)")
	}
	// Should not get the +0.10 boost for known drugs
	// Context has "compound" and "synthesized" so context boost applies (+0.05)
	if result.AdjustedConfidence > 0.85 {
		t.Errorf("expected no drug boost, got %f", result.AdjustedConfidence)
	}
}

// ---------------------------------------------------------------------------
// Generic Structure tests
// ---------------------------------------------------------------------------

func TestValidate_GenericStructure_Valid(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("C1-C6 alkyl", EntityGenericStructure, 0.70, "substituent is C1-C6 alkyl")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid generic structure, issues: %v", result.Issues)
	}
}

func TestValidate_GenericStructure_NoKeyword(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("some group", EntityGenericStructure, 0.70, "the some group")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for no generic keyword")
	}
}

// ---------------------------------------------------------------------------
// Markush Variable tests
// ---------------------------------------------------------------------------

func TestValidate_MarkushVariable_Valid(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("R1", EntityMarkushVariable, 0.70, "where R1 is alkyl")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid Markush variable, issues: %v", result.Issues)
	}
}

func TestValidate_MarkushVariable_Valid_Ar(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("Ar", EntityMarkushVariable, 0.70, "where Ar is aryl")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid for Ar, issues: %v", result.Issues)
	}
}

func TestValidate_MarkushVariable_Invalid(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("ABC", EntityMarkushVariable, 0.70, "where ABC is defined")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for non-Markush pattern")
	}
}

// ---------------------------------------------------------------------------
// InChI tests
// ---------------------------------------------------------------------------

func TestValidate_InChI_Valid(t *testing.T) {
	v := newValidator()
	inchi := "InChI=1S/C9H8O4/c1-6(10)13-8-5-3-2-4-7(8)9(11)12/h2-5H,1H3,(H,11,12)"
	entity := makeRawEntity(inchi, EntityInChI, 0.80, "the structure "+inchi)
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid InChI, issues: %v", result.Issues)
	}
	if result.AdjustedConfidence <= 0.80 {
		t.Errorf("expected confidence boost, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_InChI_InvalidPrefix(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("1S/C9H8O4/c1-6(10)", EntityInChI, 0.80, "structure")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for missing InChI= prefix")
	}
}

// ---------------------------------------------------------------------------
// Brand Name tests
// ---------------------------------------------------------------------------

func TestValidate_BrandName_Known(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("Tylenol", EntityBrandName, 0.70, "the drug Tylenol is a pharmaceutical compound")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid brand name, issues: %v", result.Issues)
	}
	if result.AdjustedConfidence <= 0.70 {
		t.Errorf("expected confidence boost, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_BrandName_CommonNoun(t *testing.T) {
	v := newValidator()
	// "System" lowercased is "system" which is in blacklistWords
	entity := makeRawEntity("System", EntityBrandName, 0.70, "the System is described")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "system" is blacklisted at the top-level check
	if result.IsValid {
		t.Error("expected invalid for common noun 'System'")
	}
}

// ---------------------------------------------------------------------------
// Context validation tests
// ---------------------------------------------------------------------------

func TestValidate_ContextBoost_ChemicalContext(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("aspirin", EntityCommonName, 0.75, "the compound was tested in a reaction")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Known drug (+0.10) + context boost (+0.05) = 0.90
	if result.AdjustedConfidence < 0.85 {
		t.Errorf("expected context boost, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_ContextPenalty_NonChemicalContext(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("aspirin", EntityCommonName, 0.75, "the method of claim 1 wherein")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Known drug (+0.10) but context penalty (-0.10) = 0.75
	// "method", "claim", "wherein" are not in chemicalContextWords
	if result.AdjustedConfidence > 0.80 {
		t.Errorf("expected context penalty, got %f", result.AdjustedConfidence)
	}
}

// ---------------------------------------------------------------------------
// Type correction tests
// ---------------------------------------------------------------------------

func TestValidate_TypeCorrection_CASMislabeledAsCommon(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("50-78-2", EntityCommonName, 0.70, "compound 50-78-2")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AdjustedType != EntityCASNumber {
		t.Errorf("expected type correction to CAS, got %s", result.AdjustedType)
	}
	if _, ok := result.Corrections["type"]; !ok {
		t.Error("expected type correction entry in Corrections map")
	}
}

func TestValidate_TypeCorrection_FormulaMislabeledAsIUPAC(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("C9H8O4", EntityIUPACName, 0.70, "molecular formula C9H8O4")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AdjustedType != EntityMolecularFormula {
		t.Errorf("expected type correction to MolecularFormula, got %s", result.AdjustedType)
	}
}

// ---------------------------------------------------------------------------
// Blacklist tests
// ---------------------------------------------------------------------------

func TestValidate_Blacklist_Method(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("method", EntityCommonName, 0.80, "the method of claim 1")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for blacklisted 'method'")
	}
	if result.AdjustedConfidence != 0 {
		t.Errorf("expected confidence 0 for blacklisted, got %f", result.AdjustedConfidence)
	}
}

func TestValidate_Blacklist_USPTO(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("USPTO", EntityCommonName, 0.80, "filed with USPTO")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for blacklisted 'USPTO'")
	}
}

func TestValidate_Blacklist_PureNumber(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("12345", EntityCommonName, 0.80, "reference 12345")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for pure number")
	}
}

// ---------------------------------------------------------------------------
// Batch validation tests
// ---------------------------------------------------------------------------

func TestValidateBatch_Success(t *testing.T) {
	v := newValidator()
	entities := []*RawChemicalEntity{
		makeRawEntity("50-78-2", EntityCASNumber, 0.80, "compound 50-78-2"),
		makeRawEntity("aspirin", EntityCommonName, 0.75, "the drug aspirin"),
		makeRawEntity("C9H8O4", EntityMolecularFormula, 0.80, "formula C9H8O4"),
		makeRawEntity("CC(=O)O", EntitySMILES, 0.85, "structure CC(=O)O"),
		makeRawEntity("R1", EntityMarkushVariable, 0.70, "where R1 is alkyl"),
	}
	results, err := v.ValidateBatch(context.Background(), entities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Entity == nil {
			t.Errorf("result[%d] has nil entity", i)
		}
	}
}

func TestValidateBatch_MixedResults(t *testing.T) {
	v := newValidator()
	entities := []*RawChemicalEntity{
		makeRawEntity("50-78-2", EntityCASNumber, 0.80, "compound 50-78-2"),
		makeRawEntity("method", EntityCommonName, 0.80, "the method"),
		makeRawEntity("CC(=O", EntitySMILES, 0.80, "molecule CC(=O"),
		makeRawEntity("aspirin", EntityCommonName, 0.75, "the drug aspirin"),
		makeRawEntity("12345", EntityIUPACName, 0.80, "reference 12345"),
	}
	results, err := v.ValidateBatch(context.Background(), entities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	validCount := 0
	invalidCount := 0
	for _, r := range results {
		if r.IsValid {
			validCount++
		} else {
			invalidCount++
		}
	}
	if validCount < 2 {
		t.Errorf("expected at least 2 valid results, got %d", validCount)
	}
	if invalidCount < 2 {
		t.Errorf("expected at least 2 invalid results, got %d", invalidCount)
	}
}

func TestValidateBatch_Empty(t *testing.T) {
	v := newValidator()
	results, err := v.ValidateBatch(context.Background(), []*RawChemicalEntity{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ---------------------------------------------------------------------------
// Confidence clamp test
// ---------------------------------------------------------------------------

func TestValidate_ConfidenceClamp(t *testing.T) {
	v := newValidator()
	// High initial confidence + multiple boosts should clamp to 1.0
	entity := makeRawEntity("50-78-2", EntityCASNumber, 0.95, "the compound 50-78-2 is a molecule used in synthesis")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AdjustedConfidence > 1.0 {
		t.Errorf("confidence should be clamped to 1.0, got %f", result.AdjustedConfidence)
	}
	if result.AdjustedConfidence < 0.0 {
		t.Errorf("confidence should not be negative, got %f", result.AdjustedConfidence)
	}

	// Low initial confidence + penalties should clamp to 0.0
	entity2 := makeRawEntity("xyzabc", EntityCommonName, 0.05, "the method of claim 1 wherein the step")
	result2, err := v.Validate(context.Background(), entity2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.AdjustedConfidence < 0.0 {
		t.Errorf("confidence should be clamped to 0.0, got %f", result2.AdjustedConfidence)
	}
	if result2.AdjustedConfidence > 1.0 {
		t.Errorf("confidence should be clamped to 1.0, got %f", result2.AdjustedConfidence)
	}
}

// ---------------------------------------------------------------------------
// Nil entity test
// ---------------------------------------------------------------------------

func TestValidate_NilEntity(t *testing.T) {
	v := newValidator()
	_, err := v.Validate(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil entity")
	}
}

// ---------------------------------------------------------------------------
// Empty text test
// ---------------------------------------------------------------------------

func TestValidate_EmptyText(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("", EntityCommonName, 0.80, "some context")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid {
		t.Error("expected invalid for empty text")
	}
	if result.AdjustedConfidence != 0 {
		t.Errorf("expected confidence 0 for empty text, got %f", result.AdjustedConfidence)
	}
}

// ---------------------------------------------------------------------------
// CAS check digit algorithm unit tests
// ---------------------------------------------------------------------------

func TestValidateCASCheckDigit_Aspirin(t *testing.T) {
	// 50-78-2: digits="5078", check=2
	// 8*1 + 7*2 + 0*3 + 5*4 = 8+14+0+20 = 42, 42%10=2 ✓
	if !validateCASCheckDigit("50-78-2") {
		t.Error("expected valid check digit for 50-78-2")
	}
}

func TestValidateCASCheckDigit_Water(t *testing.T) {
	// 7732-18-5: digits="773218", check=5
	// 8*1 + 1*2 + 2*3 + 3*4 + 7*5 + 7*6 = 8+2+6+12+35+42 = 105, 105%10=5 ✓
	if !validateCASCheckDigit("7732-18-5") {
		t.Error("expected valid check digit for 7732-18-5 (water)")
	}
}

func TestValidateCASCheckDigit_Invalid(t *testing.T) {
	if validateCASCheckDigit("50-78-3") {
		t.Error("expected invalid check digit for 50-78-3")
	}
}

func TestValidateCASCheckDigit_BadFormat(t *testing.T) {
	if validateCASCheckDigit("not-a-cas") {
		t.Error("expected false for non-numeric CAS")
	}
	if validateCASCheckDigit("") {
		t.Error("expected false for empty string")
	}
	if validateCASCheckDigit("12") {
		t.Error("expected false for incomplete CAS")
	}
}

// ---------------------------------------------------------------------------
// Parentheses / bracket balance unit tests
// ---------------------------------------------------------------------------

func TestCheckParenthesesBalance(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"CC(=O)O", true},
		{"CC(=O)(O)C", true},
		{"CC(=O", false},
		{"CC)O(", false},
		{"((()))", true},
		{"", true},
		{")(", false},
	}
	for _, tt := range tests {
		got := checkParenthesesBalance(tt.input)
		if got != tt.want {
			t.Errorf("checkParenthesesBalance(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestCheckBracketBalance(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"[Na+].[Cl-]", true},
		{"[Cu]", true},
		{"[Na+", false},
		{"]Na[", false},
		{"CC", true},
		{"", true},
	}
	for _, tt := range tests {
		got := checkBracketBalance(tt.input)
		if got != tt.want {
			t.Errorf("checkBracketBalance(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Ring closure unit tests
// ---------------------------------------------------------------------------

func TestCheckRingClosures(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"c1ccccc1", true},
		{"C1CCC1", true},
		{"C1CCC", false},
		{"C1CC2CCCC2C1", true},
		{"CC", true},
		{"", true},
	}
	for _, tt := range tests {
		got := checkRingClosures(tt.input)
		if got != tt.want {
			t.Errorf("checkRingClosures(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// SMILES atom check unit tests
// ---------------------------------------------------------------------------

func TestCheckSMILESAtoms(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"CCO", true},
		{"c1ccccc1", true},
		{"CC(=O)O", true},
		{"[Cu]CC", true},
		{"CXC", false},
		{"ClC", true},
		{"BrC", true},
		{"", true},
	}
	for _, tt := range tests {
		got := checkSMILESAtoms(tt.input)
		if got != tt.want {
			t.Errorf("checkSMILESAtoms(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// IUPAC suffix detection unit tests
// ---------------------------------------------------------------------------

func TestContainsIUPACSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2-acetoxybenzoic acid", true},
		{"methanol", true},
		{"ethylene", true},
		{"propyne", true},
		{"butylamine", true},
		{"something", false},
		{"random text", false},
		{"", false},
	}
	for _, tt := range tests {
		got := containsIUPACSuffix(tt.input)
		if got != tt.want {
			t.Errorf("containsIUPACSuffix(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Clamp confidence unit test
// ---------------------------------------------------------------------------

func TestClampConfidence(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{0.5, 0.5},
		{0.0, 0.0},
		{1.0, 1.0},
		{-0.5, 0.0},
		{1.5, 1.0},
		{-100.0, 0.0},
		{100.0, 1.0},
	}
	for _, tt := range tests {
		got := clampConfidence(tt.input)
		if got != tt.want {
			t.Errorf("clampConfidence(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// NewEntityValidator constructors
// ---------------------------------------------------------------------------

func TestNewEntityValidator(t *testing.T) {
	v := NewEntityValidator()
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestNewEntityValidatorWithRDKit(t *testing.T) {
	v := NewEntityValidatorWithRDKit(true)
	if v == nil {
		t.Fatal("expected non-nil validator")
	}
	impl, ok := v.(*entityValidatorImpl)
	if !ok {
		t.Fatal("expected *entityValidatorImpl")
	}
	if !impl.rdkitAvailable {
		t.Error("expected rdkitAvailable=true")
	}
}

// ---------------------------------------------------------------------------
// Type correction: GENERIC -> MARKUSH
// ---------------------------------------------------------------------------

func TestValidate_TypeCorrection_GenericToMarkush(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("R1", EntityGenericStructure, 0.70, "where R1 is alkyl")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AdjustedType != EntityMarkushVariable {
		t.Errorf("expected type correction to MarkushVariable, got %s", result.AdjustedType)
	}
}

// ---------------------------------------------------------------------------
// Context with empty string
// ---------------------------------------------------------------------------

func TestValidate_EmptyContext(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("aspirin", EntityCommonName, 0.75, "")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No context boost or penalty
	// Known drug: +0.10 → 0.85
	if result.AdjustedConfidence < 0.84 || result.AdjustedConfidence > 0.86 {
		t.Errorf("expected ~0.85 with no context effect, got %f", result.AdjustedConfidence)
	}
}

// ---------------------------------------------------------------------------
// Multiple SMILES valid structures
// ---------------------------------------------------------------------------

func TestValidate_SMILES_Benzene(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("c1ccccc1", EntitySMILES, 0.80, "the molecule c1ccccc1")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid benzene SMILES, issues: %v", result.Issues)
	}
}

func TestValidate_SMILES_Ethanol(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("CCO", EntitySMILES, 0.80, "the compound CCO is ethanol")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid ethanol SMILES, issues: %v", result.Issues)
	}
}

func TestValidate_SMILES_WithBracketAtom(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("[Na+].[Cl-]", EntitySMILES, 0.80, "the salt [Na+].[Cl-]")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid NaCl SMILES, issues: %v", result.Issues)
	}
}

// ---------------------------------------------------------------------------
// Molecular formula edge cases
// ---------------------------------------------------------------------------

func TestValidate_MolecularFormula_SingleElement(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("Fe", EntityMolecularFormula, 0.80, "the element Fe")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid single element, issues: %v", result.Issues)
	}
}

func TestValidate_MolecularFormula_Water(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("H2O", EntityMolecularFormula, 0.80, "the formula H2O")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid water formula, issues: %v", result.Issues)
	}
}

// ---------------------------------------------------------------------------
// InChI edge cases
// ---------------------------------------------------------------------------

func TestValidate_InChI_MinimalValid(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("InChI=1S/CH4/h1H4", EntityInChI, 0.80, "the structure InChI=1S/CH4/h1H4")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid minimal InChI, issues: %v", result.Issues)
	}
}

func TestValidate_InChI_NoLayers(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("InChI=1S", EntityInChI, 0.80, "structure InChI=1S")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Valid (has prefix) but should have issue about no layers
	if !result.IsValid {
		t.Errorf("expected valid (prefix present), issues: %v", result.Issues)
	}
	foundLayerIssue := false
	for _, iss := range result.Issues {
		if contains(iss, "layer") {
			foundLayerIssue = true
		}
	}
	if !foundLayerIssue {
		t.Error("expected layer issue for InChI with no layers")
	}
}

// ---------------------------------------------------------------------------
// Markush variable edge cases
// ---------------------------------------------------------------------------

func TestValidate_MarkushVariable_R(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("R", EntityMarkushVariable, 0.70, "where R is defined")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid for R, issues: %v", result.Issues)
	}
}

func TestValidate_MarkushVariable_X(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("X", EntityMarkushVariable, 0.70, "where X is halogen")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid for X, issues: %v", result.Issues)
	}
}

func TestValidate_MarkushVariable_Het(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("Het", EntityMarkushVariable, 0.70, "where Het is heterocyclic")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid for Het, issues: %v", result.Issues)
	}
}

// ---------------------------------------------------------------------------
// Brand name edge cases
// ---------------------------------------------------------------------------

func TestValidate_BrandName_Advil(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("Advil", EntityBrandName, 0.70, "the pharmaceutical Advil is a drug")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid {
		t.Errorf("expected valid for Advil, issues: %v", result.Issues)
	}
}

func TestValidate_BrandName_UnknownBrand(t *testing.T) {
	v := newValidator()
	entity := makeRawEntity("Xyzpharma", EntityBrandName, 0.70, "the drug Xyzpharma")
	result, err := v.Validate(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Valid but not in known database
	if !result.IsValid {
		t.Errorf("expected valid (not blacklisted), issues: %v", result.Issues)
	}
	foundIssue := false
	for _, iss := range result.Issues {
		if contains(iss, "brand name database") {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Error("expected 'not found in brand name database' issue")
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}


