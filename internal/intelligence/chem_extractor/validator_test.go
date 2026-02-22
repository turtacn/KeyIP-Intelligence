package chem_extractor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate_CAS_Valid(t *testing.T) {
	v := NewEntityValidator(nil)
	entity := &RawChemicalEntity{
		Text:       "50-78-2",
		EntityType: EntityCASNumber,
		Confidence: 0.8,
		Context:    "synthesis of compound", // Provide context to avoid penalty
	}
	res, err := v.Validate(context.Background(), entity)
	assert.NoError(t, err)
	assert.True(t, res.IsValid)
	assert.Greater(t, res.AdjustedConfidence, 0.8)
}

func TestValidate_CAS_InvalidChecksum(t *testing.T) {
	v := NewEntityValidator(nil)
	entity := &RawChemicalEntity{
		Text:       "50-78-3", // Invalid checksum
		EntityType: EntityCASNumber,
		Confidence: 0.8,
	}
	res, err := v.Validate(context.Background(), entity)
	assert.NoError(t, err)
	assert.False(t, res.IsValid)
	assert.Contains(t, res.Issues, "Invalid CAS format or checksum")
}

func TestValidate_TypeCorrection(t *testing.T) {
	v := NewEntityValidator(nil)
	entity := &RawChemicalEntity{
		Text:       "50-78-2",
		EntityType: EntityCommonName, // Wrong type
		Confidence: 0.8,
	}
	res, err := v.Validate(context.Background(), entity)
	assert.NoError(t, err)
	assert.Equal(t, EntityCASNumber, res.AdjustedType)
}

func TestValidate_Blacklist(t *testing.T) {
	v := NewEntityValidator(nil)
	entity := &RawChemicalEntity{
		Text:       "Method",
		EntityType: EntityCommonName,
		Confidence: 0.9,
	}
	res, err := v.Validate(context.Background(), entity)
	assert.NoError(t, err)
	assert.False(t, res.IsValid)
	assert.Contains(t, res.Issues, "Blacklisted term")
}

func TestValidate_SMILES(t *testing.T) {
	v := NewEntityValidator(nil)
	// Valid
	res, _ := v.Validate(context.Background(), &RawChemicalEntity{Text: "C", EntityType: EntitySMILES, Confidence: 0.8})
	assert.True(t, res.IsValid)

	// Invalid paren
	res, _ = v.Validate(context.Background(), &RawChemicalEntity{Text: "C(", EntityType: EntitySMILES, Confidence: 0.8})
	assert.False(t, res.IsValid)
}

func TestValidate_Context(t *testing.T) {
	v := NewEntityValidator(nil)
	entity := &RawChemicalEntity{
		Text:       "aspirin",
		EntityType: EntityCommonName,
		Confidence: 0.5,
		Context:    "The synthesis of aspirin was performed.",
	}
	res, _ := v.Validate(context.Background(), entity)
	// Should boost confidence
	assert.Greater(t, res.AdjustedConfidence, 0.5)
}
//Personal.AI order the ending
