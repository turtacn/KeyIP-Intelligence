// chem_adapter.go — minimal ChemExtractor wiring for apiserver.
// Provides a regex-based ChemicalExtractor that performs pattern-based
// entity recognition without requiring a full NER model.
package main

import (
	"context"

	chemextractor "github.com/turtacn/KeyIP-Intelligence/internal/intelligence/chem_extractor"
)

// minimalEntityResolver provides a no-op entity resolution for entities
// found by the regex-based extractor. Chemical structure resolution
// requires external services (PubChem, RDKit) which are wired separately
// in production; this adapter enables extraction without resolution.
type minimalEntityResolver struct{}

func (r *minimalEntityResolver) Resolve(ctx context.Context, entity *chemextractor.RawChemicalEntity) (*chemextractor.ResolvedChemicalEntity, error) {
	return &chemextractor.ResolvedChemicalEntity{
		OriginalEntity:   entity,
		CanonicalName:    entity.Text,
		IsResolved:       false,
		ResolutionMethod: "identity",
	}, nil
}

func (r *minimalEntityResolver) ResolveBatch(ctx context.Context, entities []*chemextractor.RawChemicalEntity) ([]*chemextractor.ResolvedChemicalEntity, error) {
	result := make([]*chemextractor.ResolvedChemicalEntity, len(entities))
	for i, e := range entities {
		resolved, _ := r.Resolve(ctx, e)
		result[i] = resolved
	}
	return result, nil
}

func (r *minimalEntityResolver) ResolveByType(ctx context.Context, text string, entityType chemextractor.ChemicalEntityType) (*chemextractor.ResolvedChemicalEntity, error) {
	raw := &chemextractor.RawChemicalEntity{
		Text:       text,
		EntityType: entityType,
		Confidence: 1.0,
	}
	return r.Resolve(ctx, raw)
}

// newMinimalChemExtractor constructs a ChemicalExtractor using only the
// built-in regex patterns (CAS, SMILES, formula, Markush). NER is
// disabled to avoid model dependencies. Entity resolution is identity-only.
func newMinimalChemExtractor() (chemextractor.ChemicalExtractor, error) {
	config := chemextractor.ExtractorConfig{
		EnableNER:              false, // no NER model available
		EnableDictionaryLookup: false,
		MinConfidence:          0.5,
		ContextWindowSize:      80,
		MaxTextLength:          500000,
		BatchConcurrency:       1,
	}

	return chemextractor.NewChemicalExtractor(
		nil,                              // nerModel — disabled
		&minimalEntityResolver{},         // resolver — identity
		nil,                              // validator — not required
		nil,                              // dictionary — disabled
		nil,                              // moleculeDB — disabled
		nil,                              // externalDB — disabled
		config,
		nil, // metrics — noop
		nil, // logger — noop
	)
}

//Personal.AI order the ending
