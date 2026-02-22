package chem_extractor

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// EntityValidator defines the interface for validation.
type EntityValidator interface {
	Validate(ctx context.Context, entity *RawChemicalEntity) (*ValidationResult, error)
	ValidateBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ValidationResult, error)
}

// ValidationResult represents the result of validation.
type ValidationResult struct {
	Entity             *RawChemicalEntity `json:"entity"`
	IsValid            bool               `json:"is_valid"`
	AdjustedConfidence float64            `json:"adjusted_confidence"`
	AdjustedType       ChemicalEntityType `json:"adjusted_type"`
	Issues             []string           `json:"issues"`
	Corrections        map[string]string  `json:"corrections"`
}

// entityValidatorImpl implements EntityValidator.
type entityValidatorImpl struct {
	rdkit RDKitService // Optional for advanced validation
}

// NewEntityValidator creates a new EntityValidator.
func NewEntityValidator(rdkit RDKitService) EntityValidator {
	return &entityValidatorImpl{
		rdkit: rdkit,
	}
}

func (v *entityValidatorImpl) Validate(ctx context.Context, entity *RawChemicalEntity) (*ValidationResult, error) {
	res := &ValidationResult{
		Entity:             entity,
		IsValid:            true,
		AdjustedConfidence: entity.Confidence,
		AdjustedType:       entity.EntityType,
		Corrections:        make(map[string]string),
	}

	// 1. Basic cleaning and type correction check
	text := strings.TrimSpace(entity.Text)

	// Check if blacklisted
	if isBlacklisted(text) {
		res.IsValid = false
		res.Issues = append(res.Issues, "Blacklisted term")
		return res, nil
	}

	// Heuristic type correction before validation
	if isCAS(text) && entity.EntityType != EntityCASNumber {
		res.AdjustedType = EntityCASNumber
		res.Issues = append(res.Issues, "Corrected type to CASNumber")
	} else if isFormula(text) && entity.EntityType == EntityIUPACName {
		res.AdjustedType = EntityMolecularFormula
		res.Issues = append(res.Issues, "Corrected type to MolecularFormula")
	}

	// 2. Specific validation logic
	switch res.AdjustedType {
	case EntityCASNumber:
		if !v.validateCAS(text) {
			res.IsValid = false
			res.Issues = append(res.Issues, "Invalid CAS format or checksum")
		} else {
			res.AdjustedConfidence += 0.1
		}
	case EntitySMILES:
		if !v.validateSMILES(text) {
			res.IsValid = false
			res.Issues = append(res.Issues, "Invalid SMILES syntax")
		} else {
			res.AdjustedConfidence += 0.15
		}
	case EntityMolecularFormula:
		if !v.validateFormula(text) {
			res.IsValid = false
			res.Issues = append(res.Issues, "Invalid molecular formula")
		} else {
			res.AdjustedConfidence += 0.1
		}
	case EntityIUPACName:
		if !v.validateIUPAC(text) {
			res.AdjustedConfidence -= 0.1
			// Not invalidating because names are hard
		} else {
			res.AdjustedConfidence += 0.05
		}
	case EntityCommonName, EntityBrandName:
		if !v.validateCommonName(text) {
			// Check if common word
			res.AdjustedConfidence -= 0.1
		} else {
			res.AdjustedConfidence += 0.1
		}
	}

	// 3. Context validation
	if hasChemicalContext(entity.Context) {
		res.AdjustedConfidence += 0.05
	} else {
		res.AdjustedConfidence -= 0.1
	}

	// Clamp confidence
	if res.AdjustedConfidence > 1.0 {
		res.AdjustedConfidence = 1.0
	}
	if res.AdjustedConfidence < 0.0 {
		res.AdjustedConfidence = 0.0
	}

	// Final validity check based on confidence drop
	if res.AdjustedConfidence < 0.3 {
		res.IsValid = false
		res.Issues = append(res.Issues, "Confidence too low after validation")
	}

	return res, nil
}

func (v *entityValidatorImpl) ValidateBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ValidationResult, error) {
	var results []*ValidationResult
	for _, e := range entities {
		res, err := v.Validate(ctx, e)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

// --- Helpers ---

func isBlacklisted(text string) bool {
	blacklist := map[string]bool{
		"method": true, "system": true, "device": true, "apparatus": true, "composition": true,
		"process": true, "step": true, "claim": true, "wherein": true, "comprising": true,
		"uspto": true, "epo": true, "wipo": true, "ipc": true, "cpc": true, "pct": true,
	}
	// Check if pure number
	if _, err := strconv.ParseFloat(text, 64); err == nil {
		// Only valid if type is CAS (handled separately? CAS has dashes).
		// If simple number, it's blacklisted unless context says otherwise.
		// Actually pure numbers are rarely chemical entities unless ID.
		return true
	}
	return blacklist[strings.ToLower(text)]
}

func isCAS(text string) bool {
	matched, _ := regexp.MatchString(`^\d{2,7}-\d{2}-\d$`, text)
	return matched
}

func (v *entityValidatorImpl) validateCAS(cas string) bool {
	if !isCAS(cas) {
		return false
	}
	// Checksum
	parts := strings.Split(cas, "-")
	digits := parts[0] + parts[1]
	checkDigit, _ := strconv.Atoi(parts[2])

	sum := 0
	for i := 0; i < len(digits); i++ {
		d := int(digits[len(digits)-1-i] - '0')
		sum += d * (i + 1)
	}
	return sum%10 == checkDigit
}

func (v *entityValidatorImpl) validateSMILES(smiles string) bool {
	if len(smiles) == 0 {
		return false
	}
	// Basic syntax check
	openP := strings.Count(smiles, "(")
	closeP := strings.Count(smiles, ")")
	if openP != closeP {
		return false
	}

	// Valid chars
	validChars := regexp.MustCompile(`^[A-Za-z0-9@\+\-\.=\#\$:/\(\)\[\]%\\]+$`)
	if !validChars.MatchString(smiles) {
		return false
	}

	// If RDKit available, use it
	if v.rdkit != nil {
		valid, err := v.rdkit.ValidateSMILES(smiles)
		if err == nil {
			return valid
		}
	}
	return true
}

func isFormula(text string) bool {
	matched, _ := regexp.MatchString(`^([A-Z][a-z]?\d*)+$`, text)
	return matched
}

func (v *entityValidatorImpl) validateFormula(text string) bool {
	return isFormula(text)
}

func (v *entityValidatorImpl) validateIUPAC(text string) bool {
	if len(text) < 3 {
		return false
	}
	// Heuristics: check for chemical suffixes/prefixes
	suffixes := []string{"yl", "ane", "ene", "yne", "ol", "al", "one", "oic acid", "amine", "ide", "ate", "ite"}
	lower := strings.ToLower(text)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	return false
}

func (v *entityValidatorImpl) validateCommonName(text string) bool {
	// Simple check: starts with uppercase if not element? No.
	// Check length and non-blacklisted.
	if len(text) < 3 {
		return false
	}
	// Check if it's a common English word (requires dictionary, here simplified)
	// Assume valid unless looks like garbage
	return true
}

func hasChemicalContext(ctx string) bool {
	keywords := []string{"compound", "molecule", "formula", "structure", "synthesis", "reaction", "mixture", "solution"}
	lower := strings.ToLower(ctx)
	for _, k := range keywords {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

//Personal.AI order the ending
