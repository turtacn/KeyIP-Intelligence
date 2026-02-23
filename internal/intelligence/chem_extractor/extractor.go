package chem_extractor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ExtractionInput contains raw document bytes and processing options.
type ExtractionInput struct {
	Data        []byte   `json:"data"`
	Format      string   `json:"format"`
	EntityTypes []string `json:"entity_types"`
}

// ExtractedEntity represents a single extracted chemical entity.
type ExtractedEntity struct {
	Type       string  `json:"type"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Page       int     `json:"page"`
	Offset     int     `json:"offset"`
}

// Extractor defines the interface for chemical entity extraction, resolution, and validation.
type Extractor interface {
	Extract(ctx context.Context, input *ExtractionInput) ([]ExtractedEntity, error)
	Resolve(ctx context.Context, value string, entityType string) (*ResolvedChemicalEntity, error)
	Validate(ctx context.Context, smiles string) (bool, error)
}

// extractorImpl implements the Extractor interface by orchestrating NER, Resolver, and Validator.
type extractorImpl struct {
	ner       NERModel
	resolver  EntityResolver
	validator EntityValidator
	logger    common.Logger
}

// NewExtractor creates a new Extractor instance.
func NewExtractor(
	ner NERModel,
	resolver EntityResolver,
	validator EntityValidator,
	logger common.Logger,
) Extractor {
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	// Defaults if nil (though caller should provide)
	if validator == nil {
		validator = NewEntityValidator()
	}

	return &extractorImpl{
		ner:       ner,
		resolver:  resolver,
		validator: validator,
		logger:    logger,
	}
}

// Extract processes the input document/text and returns extracted entities.
func (e *extractorImpl) Extract(ctx context.Context, input *ExtractionInput) ([]ExtractedEntity, error) {
	if input == nil {
		return nil, errors.NewInvalidInputError("extraction input is nil")
	}

	// For now, we assume simple text conversion.
	// In a real system, we'd use a document parser (e.g., PDF/DOCX to text) based on input.Format.
	text := string(input.Data)
	if text == "" {
		return []ExtractedEntity{}, nil
	}

	// NER prediction
	prediction, err := e.ner.Predict(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("ner prediction failed: %w", err)
	}

	var results []ExtractedEntity
	for _, ent := range prediction.Entities {
		// Filter by requested EntityTypes if specified
		if len(input.EntityTypes) > 0 {
			found := false
			for _, t := range input.EntityTypes {
				if strings.EqualFold(t, ent.Label) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, ExtractedEntity{
			Type:       ent.Label,
			Value:      ent.Text,
			Confidence: ent.Score,
			Page:       1, // Default to page 1 for text
			Offset:     ent.StartChar,
		})
	}

	return results, nil
}

// Resolve attempts to resolve a raw entity string to a canonical chemical structure.
func (e *extractorImpl) Resolve(ctx context.Context, value string, entityType string) (*ResolvedChemicalEntity, error) {
	if e.resolver == nil {
		// Fallback if no resolver configured
		return &ResolvedChemicalEntity{
			OriginalText: value,
			EntityType:   ChemicalEntityType(entityType),
			IsResolved:   false,
			ResolvedAt:   time.Now(),
		}, nil
	}

	raw := &RawChemicalEntity{
		Text:       value,
		EntityType: ChemicalEntityType(entityType),
		Confidence: 1.0, // Assumed high confidence for manual resolution request
	}
	return e.resolver.Resolve(ctx, raw)
}

// Validate checks if a SMILES string represents a valid chemical structure.
func (e *extractorImpl) Validate(ctx context.Context, smiles string) (bool, error) {
	if e.validator == nil {
		return true, nil // Assume valid if no validator
	}

	// Wrap as RawChemicalEntity for validation
	raw := &RawChemicalEntity{
		Text:       smiles,
		EntityType: EntitySMILES,
		Confidence: 1.0,
	}

	res, err := e.validator.Validate(ctx, raw)
	if err != nil {
		return false, err
	}
	return res.IsValid, nil
}
