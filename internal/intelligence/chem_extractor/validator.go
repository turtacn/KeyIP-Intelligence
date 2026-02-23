package chem_extractor

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ValidationResult holds the outcome of validating a single entity.
type ValidationResult struct {
	Entity             *RawChemicalEntity    `json:"entity"`
	IsValid            bool                  `json:"is_valid"`
	AdjustedConfidence float64               `json:"adjusted_confidence"`
	AdjustedType       ChemicalEntityType    `json:"adjusted_type"`
	Issues             []string              `json:"issues,omitempty"`
	Corrections        map[string]string     `json:"corrections,omitempty"`
}

// ---------------------------------------------------------------------------
// Known data sets
// ---------------------------------------------------------------------------

var knownDrugNames = map[string]bool{
	"aspirin": true, "ibuprofen": true, "acetaminophen": true, "paracetamol": true,
	"metformin": true, "atorvastatin": true, "omeprazole": true, "amoxicillin": true,
	"lisinopril": true, "amlodipine": true, "metoprolol": true, "losartan": true,
	"simvastatin": true, "azithromycin": true, "hydrochlorothiazide": true,
	"gabapentin": true, "sertraline": true, "montelukast": true, "escitalopram": true,
	"rosuvastatin": true, "levothyroxine": true, "pantoprazole": true,
	"fluoxetine": true, "clopidogrel": true, "tramadol": true, "prednisone": true,
	"albuterol": true, "duloxetine": true, "venlafaxine": true, "trazodone": true,
	"furosemide": true, "doxycycline": true, "ciprofloxacin": true,
	"caffeine": true, "ethanol": true, "methanol": true, "glucose": true,
	"sucrose": true, "cellulose": true, "benzene": true, "toluene": true,
	"acetone": true, "chloroform": true, "dimethylsulfoxide": true,
	"penicillin": true, "insulin": true, "morphine": true, "codeine": true,
	"warfarin": true, "heparin": true, "dopamine": true, "serotonin": true,
	"epinephrine": true, "norepinephrine": true, "testosterone": true,
	"estradiol": true, "progesterone": true, "cortisol": true, "cholesterol": true,
}

var knownBrandNames = map[string]bool{
	"Tylenol": true, "Advil": true, "Motrin": true, "Lipitor": true,
	"Prilosec": true, "Nexium": true, "Zocor": true, "Crestor": true,
	"Prozac": true, "Zoloft": true, "Plavix": true, "Synthroid": true,
	"Norvasc": true, "Glucophage": true, "Xanax": true, "Valium": true,
	"Viagra": true, "Cialis": true, "Humira": true, "Enbrel": true,
	"Remicade": true, "Herceptin": true, "Avastin": true, "Rituxan": true,
	"Keytruda": true, "Opdivo": true, "Revlimid": true, "Eliquis": true,
	"Jardiance": true, "Ozempic": true, "Dupixent": true, "Stelara": true,
}

var blacklistWords = map[string]bool{
	"method": true, "system": true, "device": true, "apparatus": true,
	"composition": true, "process": true, "step": true, "claim": true,
	"wherein": true, "comprising": true, "consisting": true, "including": true,
	"providing": true, "obtaining": true, "preparing": true, "producing": true,
	"forming": true, "applying": true, "administering": true, "treating": true,
	"figure": true, "table": true, "example": true, "embodiment": true,
	"invention": true, "present": true, "prior": true, "art": true,
	"abstract": true, "background": true, "summary": true, "description": true,
	"preferred": true, "particular": true, "general": true, "specific": true,
	"first": true, "second": true, "third": true, "fourth": true,
	"the": true, "and": true, "for": true, "with": true, "from": true,
}

var blacklistAbbreviations = map[string]bool{
	"PCT": true, "USPTO": true, "EPO": true, "WIPO": true,
	"IPC": true, "CPC": true, "WO": true, "US": true, "EP": true,
	"JP": true, "CN": true, "KR": true, "AU": true, "CA": true,
	"NMR": true, "HPLC": true, "GC": true, "MS": true, "IR": true,
	"UV": true, "TLC": true, "SDS": true, "PAGE": true, "PCR": true,
	"DNA": true, "RNA": true, "API": true, "FDA": true, "EMA": true,
	"ICH": true, "GMP": true, "GLP": true, "GCP": true, "SOP": true,
}

var validElements = map[string]bool{
	"H": true, "He": true, "Li": true, "Be": true, "B": true, "C": true,
	"N": true, "O": true, "F": true, "Ne": true, "Na": true, "Mg": true,
	"Al": true, "Si": true, "P": true, "S": true, "Cl": true, "Ar": true,
	"K": true, "Ca": true, "Sc": true, "Ti": true, "V": true, "Cr": true,
	"Mn": true, "Fe": true, "Co": true, "Ni": true, "Cu": true, "Zn": true,
	"Ga": true, "Ge": true, "As": true, "Se": true, "Br": true, "Kr": true,
	"Rb": true, "Sr": true, "Y": true, "Zr": true, "Nb": true, "Mo": true,
	"Tc": true, "Ru": true, "Rh": true, "Pd": true, "Ag": true, "Cd": true,
	"In": true, "Sn": true, "Sb": true, "Te": true, "I": true, "Xe": true,
	"Cs": true, "Ba": true, "La": true, "Ce": true, "Pr": true, "Nd": true,
	"Pm": true, "Sm": true, "Eu": true, "Gd": true, "Tb": true, "Dy": true,
	"Ho": true, "Er": true, "Tm": true, "Yb": true, "Lu": true, "Hf": true,
	"Ta": true, "W": true, "Re": true, "Os": true, "Ir": true, "Pt": true,
	"Au": true, "Hg": true, "Tl": true, "Pb": true, "Bi": true, "Po": true,
	"At": true, "Rn": true, "Fr": true, "Ra": true, "Ac": true, "Th": true,
	"Pa": true, "U": true, "Np": true, "Pu": true, "Am": true, "Cm": true,
	"Bk": true, "Cf": true, "Es": true, "Fm": true, "Md": true, "No": true,
	"Lr": true, "Rf": true, "Db": true, "Sg": true, "Bh": true, "Hs": true,
	"Mt": true, "Ds": true, "Rg": true, "Cn": true, "Nh": true, "Fl": true,
	"Mc": true, "Lv": true, "Ts": true, "Og": true,
}

var smilesValidAtoms = map[string]bool{
	"B": true, "C": true, "N": true, "O": true, "P": true, "S": true,
	"F": true, "Cl": true, "Br": true, "I": true,
	"c": true, "n": true, "o": true, "s": true, "p": true,
	"b": true,
	"Si": true, "Se": true, "Te": true, "As": true, "Ge": true,
	"Na": true, "Mg": true, "Al": true, "Ca": true, "Fe": true,
	"Cu": true, "Zn": true, "Ag": true, "Au": true, "Pt": true,
	"Pd": true, "Li": true, "K": true, "Ti": true, "Sn": true,
}

var chemicalContextWords = map[string]bool{
	"compound": true, "molecule": true, "formula": true, "structure": true,
	"synthesis": true, "reaction": true, "catalyst": true, "reagent": true,
	"solvent": true, "solution": true, "mixture": true, "derivative": true,
	"analog": true, "analogue": true, "isomer": true, "enantiomer": true,
	"salt": true, "ester": true, "ether": true, "acid": true, "base": true,
	"polymer": true, "monomer": true, "ligand": true, "substrate": true,
	"inhibitor": true, "agonist": true, "antagonist": true, "receptor": true,
	"enzyme": true, "protein": true, "peptide": true, "amino": true,
	"pharmaceutical": true, "drug": true, "therapeutic": true, "dosage": true,
	"chemical": true, "organic": true, "inorganic": true, "aromatic": true,
	"aliphatic": true, "heterocyclic": true, "substituent": true, "moiety": true,
	"functional": true, "group": true, "bond": true, "ring": true,
	"molecular": true, "weight": true, "concentration": true, "purity": true,
	"yield": true, "selectivity": true, "potency": true, "efficacy": true,
	"pharmacokinetic": true, "bioavailability": true, "metabolite": true,
	"prodrug": true, "excipient": true, "formulation": true, "tablet": true,
	"capsule": true, "injection": true, "infusion": true, "topical": true,
}

var iupacSuffixes = []string{
	"-yl", "-ane", "-ene", "-yne", "-ol", "-al", "-one", "-oic acid",
	"-amine", "-amide", "-ate", "-ite", "-ide", "-ase", "-ose",
	"-oyl", "-oxo", "-thio", "-amino", "-hydroxy", "-methyl", "-ethyl",
	"-propyl", "-butyl", "-phenyl", "-benzyl", "-cyclo", "-nitro",
	"-chloro", "-bromo", "-fluoro", "-iodo", "-sulfo", "-phospho",
	"acid", "amine", "aldehyde", "ketone", "alcohol", "ether",
}

var genericStructureKeywords = []string{
	"alkyl", "aryl", "heteroaryl", "cyclo", "halo", "halogen",
	"alkoxy", "aryloxy", "amino", "alkenyl", "alkynyl",
	"acyl", "carboxyl", "hydroxyl", "thiol", "sulfonyl",
	"phosphoryl", "nitro", "cyano", "azido", "silyl",
	"heterocyclic", "bicyclic", "tricyclic", "polycyclic",
	"substituted", "unsubstituted", "optionally substituted",
	"lower alkyl", "C1-C6", "C1-C4", "C1-C12", "C2-C6",
}

// ---------------------------------------------------------------------------
// Compiled regexes
// ---------------------------------------------------------------------------

var (
	reCASFormat          = regexp.MustCompile(`^\d{2,7}-\d{2}-\d$`)
	reMolecularFormula   = regexp.MustCompile(`^([A-Z][a-z]?\d*)+$`)
	reFormulaElement     = regexp.MustCompile(`([A-Z][a-z]?)(\d*)`)
	rePureDigits         = regexp.MustCompile(`^\d+$`)
	reMarkushVar         = regexp.MustCompile(`^(?:R\d*|R'\d*|R''\d*|X|Y|Z|Ar|Het|Alk|Hal|Q|W|M|L)$`)
	reInChIPrefix        = regexp.MustCompile(`^InChI=`)
	reInChILayer         = regexp.MustCompile(`/[a-z]`)
)

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

type entityValidatorImpl struct {
	// rdkitAvailable indicates whether an external RDKit service is reachable.
	rdkitAvailable bool
}

// NewEntityValidator creates a new EntityValidator.
func NewEntityValidator() EntityValidator {
	return &entityValidatorImpl{
		rdkitAvailable: false,
	}
}

// NewEntityValidatorWithRDKit creates a validator that may call RDKit for SMILES validation.
func NewEntityValidatorWithRDKit(rdkitAvailable bool) EntityValidator {
	return &entityValidatorImpl{
		rdkitAvailable: rdkitAvailable,
	}
}

// Validate validates a single raw chemical entity.
func (v *entityValidatorImpl) Validate(ctx context.Context, entity *RawChemicalEntity) (*ValidationResult, error) {
	if entity == nil {
		return nil, fmt.Errorf("entity is nil")
	}

	result := &ValidationResult{
		Entity:             entity,
		IsValid:            true,
		AdjustedConfidence: entity.Confidence,
		AdjustedType:       entity.EntityType,
		Issues:             []string{},
		Corrections:        map[string]string{},
	}

		text := strings.TrimSpace(entity.Text)
	if text == "" {
		result.IsValid = false
		result.Issues = append(result.Issues, "empty text")
		result.AdjustedConfidence = 0.0
		return result, nil
	}

		// ---- Blacklist check ----
	if v.isBlacklisted(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "entity is blacklisted")
		return result, nil
	}

	// ---- Type correction before validation ----
	v.correctType(result, text)

	// ---- Type-specific validation ----
	switch result.AdjustedType {
	case EntityCASNumber:
		v.validateCASNumber(result, text)
	case EntitySMILES:
		v.validateSMILES(result, text)
	case EntityMolecularFormula:
		v.validateMolecularFormula(result, text)
	case EntityIUPACName:
		v.validateIUPACName(result, text)
	case EntityCommonName:
		v.validateCommonName(result, text)
	case EntityGenericStructure:
		v.validateGenericStructure(result, text)
	case EntityMarkushVariable:
		v.validateMarkushVariable(result, text)
	case EntityInChI:
		v.validateInChI(result, text)
	case EntityBrandName:
		v.validateBrandName(result, text)
	default:
		// Unknown type — run heuristic checks
		v.validateUnknownType(result, text)
	}

	// ---- Context validation (cross-type) ----
	v.validateContext(result, entity.Context)

	result.AdjustedConfidence = clampConfidence(result.AdjustedConfidence)
	return result, nil
}

// ---------------------------------------------------------------------------
// Blacklist
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) isBlacklisted(text string) bool {
	lower := strings.ToLower(text)
	if blacklistWords[lower] {
		return true
	}
	if blacklistAbbreviations[text] {
		return true
	}
	// Pure digits (not matching CAS format)
    if rePureDigits != nil && rePureDigits.MatchString(text) {
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Type correction
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) correctType(result *ValidationResult, text string) {
	original := result.AdjustedType

	// COMMON but looks like CAS number
	if original == EntityCommonName && reCASFormat.MatchString(text) {
		result.AdjustedType = EntityCASNumber
		result.Corrections["type"] = fmt.Sprintf("%s -> %s", original, EntityCASNumber)
	}

	// IUPAC but looks like molecular formula
	if original == EntityIUPACName && reMolecularFormula.MatchString(text) && !containsIUPACSuffix(text) {
		result.AdjustedType = EntityMolecularFormula
		result.Corrections["type"] = fmt.Sprintf("%s -> %s", original, EntityMolecularFormula)
	}

	// GENERIC but looks like Markush variable
	if original == EntityGenericStructure && reMarkushVar.MatchString(text) {
		result.AdjustedType = EntityMarkushVariable
		result.Corrections["type"] = fmt.Sprintf("%s -> %s", original, EntityMarkushVariable)
	}
}

// ---------------------------------------------------------------------------
// CAS Number validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateCASNumber(result *ValidationResult, text string) {
	if !reCASFormat.MatchString(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "CAS number format invalid: expected NN…N-NN-N")
		return
	}

	if !validateCASCheckDigit(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "CAS number check digit verification failed")
		return
	}

	// Passed
	result.AdjustedConfidence += 0.10
}

// validateCASCheckDigit implements the CAS Registry Number check-digit algorithm.
//
//	CAS format: XXXXXXX-YY-Z
//	digits = XXXXXXXYY (all digits except the check digit)
//	sum = Σ digit[len-1-i] * (i+1) for i in 0..len-1
//	valid if sum % 10 == Z
func validateCASCheckDigit(cas string) bool {
	parts := strings.Split(cas, "-")
	if len(parts) != 3 {
		return false
	}
	digits := parts[0] + parts[1]
	checkDigit, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}

	sum := 0
	n := len(digits)
	for i := 0; i < n; i++ {
		d, err := strconv.Atoi(string(digits[n-1-i]))
		if err != nil {
			return false
		}
		sum += d * (i + 1)
	}
	return sum%10 == checkDigit
}

// ---------------------------------------------------------------------------
// SMILES validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateSMILES(result *ValidationResult, text string) {
	// 1. Parentheses balance
	if !checkParenthesesBalance(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "SMILES has unbalanced parentheses")
		return
	}

	// 2. Bracket balance [ ]
	if !checkBracketBalance(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "SMILES has unbalanced brackets")
		return
	}

	// 3. Ring closure matching
	if !checkRingClosures(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "SMILES has unmatched ring closure digits")
		return
	}

	// 4. Valid atom symbols (lightweight check)
	if !checkSMILESAtoms(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "SMILES contains invalid atom symbol")
		return
	}

	// Passed basic checks
	result.AdjustedConfidence += 0.15
}

func checkParenthesesBalance(s string) bool {
	depth := 0
	for _, ch := range s {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func checkBracketBalance(s string) bool {
	depth := 0
	for _, ch := range s {
		if ch == '[' {
			depth++
		} else if ch == ']' {
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func checkRingClosures(s string) bool {
	counts := make(map[string]int)

	inBracket := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '[' {
			inBracket = true
			continue
		}
		if ch == ']' {
			inBracket = false
			continue
		}
		if inBracket {
			continue
		}
		if ch == '%' && i+2 < len(s) {
			ring := string(s[i+1 : i+3])
			if isDigit(s[i+1]) && isDigit(s[i+2]) {
				counts[ring]++
				i += 2
			}
			continue
		}
		if isDigit(ch) {
			// Must not be preceded by another digit (that would be part of a count, e.g. H2)
			// In SMILES, ring digits follow atoms or bonds
			ring := string(ch)
			counts[ring]++
		}
	}

	for _, c := range counts {
		if c%2 != 0 {
			return false
		}
	}
	return true
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func checkSMILESAtoms(s string) bool {
	// Extract atoms outside brackets and check against known SMILES atoms.
	// Inside brackets [..] anything is allowed (explicit atoms).
	inBracket := false
	i := 0
	for i < len(s) {
		ch := s[i]
		if ch == '[' {
			inBracket = true
			i++
			continue
		}
		if ch == ']' {
			inBracket = false
			i++
			continue
		}
		if inBracket {
			i++
			continue
		}

		// Skip bonds, digits, dots, signs, parentheses, %, @, /, \, +, -
		if isSMILESSpecialChar(ch) {
			i++
			continue
		}

		// Try two-char atom first
		if i+1 < len(s) {
			twoChar := string(s[i : i+2])
			if smilesValidAtoms[twoChar] {
				i += 2
				continue
			}
		}
		// Try one-char atom
		oneChar := string(ch)
		if smilesValidAtoms[oneChar] {
			i++
			continue
		}

		// Uppercase letter not recognized
		if ch >= 'A' && ch <= 'Z' {
			return false
		}
		// Lowercase letter not recognized as aromatic atom
		if ch >= 'a' && ch <= 'z' {
			return false
		}

		i++
	}
	return true
}

func isSMILESSpecialChar(ch byte) bool {
	switch ch {
	case '(', ')', '.', '=', '#', '$', ':', '/', '\\', '@', '+', '-', '%',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ' ', '\t':
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Molecular Formula validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateMolecularFormula(result *ValidationResult, text string) {
	if !reMolecularFormula.MatchString(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "molecular formula format invalid")
		return
	}

	matches := reFormulaElement.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		result.IsValid = false
		result.Issues = append(result.Issues, "no elements found in molecular formula")
		return
	}

	for _, m := range matches {
		element := m[1]
		countStr := m[2]

		if !validElements[element] {
			result.IsValid = false
			result.Issues = append(result.Issues, fmt.Sprintf("unknown element: %s", element))
			return
		}

		if countStr != "" {
			count, err := strconv.Atoi(countStr)
			if err != nil {
				result.IsValid = false
				result.Issues = append(result.Issues, fmt.Sprintf("invalid count for %s: %s", element, countStr))
				return
			}
			if count > 1000 {
				result.IsValid = false
				result.Issues = append(result.Issues, fmt.Sprintf("unreasonable atom count for %s: %d", element, count))
				return
			}
		}
	}

	result.AdjustedConfidence += 0.10
}

// ---------------------------------------------------------------------------
// IUPAC Name validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateIUPACName(result *ValidationResult, text string) {
	if len(text) < 3 {
		result.IsValid = false
		result.Issues = append(result.Issues, "IUPAC name too short (< 3 chars)")
		return
	}

	if rePureDigits.MatchString(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "IUPAC name is pure digits")
		return
	}

	if containsIUPACSuffix(text) {
		result.AdjustedConfidence += 0.05
	} else {
		result.Issues = append(result.Issues, "no recognized IUPAC suffix found")
	}
}

func containsIUPACSuffix(text string) bool {
	lower := strings.ToLower(text)
	for _, suffix := range iupacSuffixes {
		if strings.HasPrefix(suffix, "-") {
			if strings.HasSuffix(lower, suffix[1:]) {
				return true
			}
		} else {
			if strings.Contains(lower, suffix) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Common Name validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateCommonName(result *ValidationResult, text string) {
	lower := strings.ToLower(text)

	// Check blacklist (common English words)
	if blacklistWords[lower] {
		result.IsValid = false
		result.Issues = append(result.Issues, fmt.Sprintf("common English word, not a chemical: %q", text))
		return
	}

	// Check known drug/compound names
	if knownDrugNames[lower] {
		result.AdjustedConfidence += 0.10
		return
	}

	// Not in known list — no boost, but still valid
	result.Issues = append(result.Issues, "not found in known compound name database")
}

// ---------------------------------------------------------------------------
// Generic Structure validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateGenericStructure(result *ValidationResult, text string) {
	lower := strings.ToLower(text)
	found := false
	for _, kw := range genericStructureKeywords {
		if strings.Contains(lower, kw) {
			found = true
			break
		}
	}
	if !found {
		result.IsValid = false
		result.Issues = append(result.Issues, "no generic structure keyword found")
		return
	}
	result.AdjustedConfidence += 0.05
}

// ---------------------------------------------------------------------------
// Markush Variable validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateMarkushVariable(result *ValidationResult, text string) {
	if reMarkushVar.MatchString(text) {
		result.AdjustedConfidence += 0.05
		return
	}
	result.IsValid = false
	result.Issues = append(result.Issues, "does not match any known Markush variable pattern")
}

// ---------------------------------------------------------------------------
// InChI validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateInChI(result *ValidationResult, text string) {
	if !reInChIPrefix.MatchString(text) {
		result.IsValid = false
		result.Issues = append(result.Issues, "InChI must start with 'InChI='")
		return
	}

	// Check for at least one layer
	layers := reInChILayer.FindAllString(text, -1)
	if len(layers) == 0 {
		result.Issues = append(result.Issues, "no InChI layers detected")
	}

	result.AdjustedConfidence += 0.15
}

// ---------------------------------------------------------------------------
// Brand Name validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateBrandName(result *ValidationResult, text string) {
	// Must start with uppercase
	if len(text) == 0 || !unicode.IsUpper(rune(text[0])) {
		result.Issues = append(result.Issues,"brand name should start with uppercase letter")
	}

	// Check blacklist for common proper nouns that are not brand names
	lower := strings.ToLower(text)
	if blacklistWords[lower] {
		result.IsValid = false
		result.Issues = append(result.Issues, fmt.Sprintf("common word mistaken for brand name: %q", text))
		return
	}

	// Known brand names
	if knownBrandNames[text] {
		result.AdjustedConfidence += 0.05
		return
	}

	// Not in known brand database
	result.Issues = append(result.Issues, "not found in known brand name database")
}

// ---------------------------------------------------------------------------
// Unknown type validation (heuristic)
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateUnknownType(result *ValidationResult, text string) {
	// Try to identify the type heuristically
	if reCASFormat.MatchString(text) && validateCASCheckDigit(text) {
		result.AdjustedType = EntityCASNumber
		result.AdjustedConfidence += 0.10
		result.Corrections["type"] = fmt.Sprintf("UNKNOWN -> %s", EntityCASNumber)
		return
	}
	if reInChIPrefix.MatchString(text) {
		result.AdjustedType = EntityInChI
		result.AdjustedConfidence += 0.10
		result.Corrections["type"] = fmt.Sprintf("UNKNOWN -> %s", EntityInChI)
		return
	}
	if reMarkushVar.MatchString(text) {
		result.AdjustedType = EntityMarkushVariable
		result.AdjustedConfidence += 0.05
		result.Corrections["type"] = fmt.Sprintf("UNKNOWN -> %s", EntityMarkushVariable)
		return
	}
	if reMolecularFormula.MatchString(text) {
		result.AdjustedType = EntityMolecularFormula
		result.AdjustedConfidence += 0.05
		result.Corrections["type"] = fmt.Sprintf("UNKNOWN -> %s", EntityMolecularFormula)
		return
	}
	// Fallback: leave as-is, slight penalty
	result.AdjustedConfidence -= 0.05
	result.Issues = append(result.Issues, "could not determine entity type heuristically")
}

// ---------------------------------------------------------------------------
// Context validation
// ---------------------------------------------------------------------------

func (v *entityValidatorImpl) validateContext(result *ValidationResult, contextText string) {
	if contextText == "" {
		return
	}

	lower := strings.ToLower(contextText)
	words := strings.Fields(lower)

	found := false
	for _, w := range words {
		// Strip punctuation from word edges
		cleaned := strings.Trim(w, ".,;:!?()[]{}\"'")
		if chemicalContextWords[cleaned] {
			found = true
			break
		}
	}

	if found {
		result.AdjustedConfidence += 0.05
	} else {
		result.AdjustedConfidence -= 0.10
		result.Issues = append(result.Issues, "surrounding context lacks chemical terminology")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func clampConfidence(c float64) float64 {
	return math.Max(0.0, math.Min(1.0, c))
}

func (v *entityValidatorImpl) ValidateBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ValidationResult, error) {
	results := make([]*ValidationResult, len(entities))
	for i, ent := range entities {
		res, err := v.Validate(ctx, ent)
		if err != nil {
			return nil, err
		}
		results[i] = res
	}
	return results, nil
}
