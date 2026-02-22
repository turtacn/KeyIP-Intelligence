package chem_extractor

import (
	"context"
	"errors"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/claim_bert"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ChemicalEntityType defines the type of chemical entity.
type ChemicalEntityType string

const (
	EntityIUPACName       ChemicalEntityType = "IUPACName"
	EntityCommonName      ChemicalEntityType = "CommonName"
	EntityBrandName       ChemicalEntityType = "BrandName"
	EntityMolecularFormula ChemicalEntityType = "MolecularFormula"
	EntitySMILES          ChemicalEntityType = "SMILES"
	EntityInChI           ChemicalEntityType = "InChI"
	EntityCASNumber       ChemicalEntityType = "CASNumber"
	EntityGenericStructure ChemicalEntityType = "GenericStructure"
	EntityPolymer         ChemicalEntityType = "Polymer"
	EntityBiological      ChemicalEntityType = "Biological"
	EntityMarkushVariable ChemicalEntityType = "MarkushVariable"
)

// RawChemicalEntity represents an extracted raw entity.
type RawChemicalEntity struct {
	Text        string             `json:"text"`
	StartOffset int                `json:"start_offset"`
	EndOffset   int                `json:"end_offset"`
	EntityType  ChemicalEntityType `json:"entity_type"`
	Confidence  float64            `json:"confidence"`
	Context     string             `json:"context"`
	Source      string             `json:"source"`
}

// ResolvedChemicalEntity represents a resolved chemical entity.
type ResolvedChemicalEntity struct {
	OriginalEntity   *RawChemicalEntity `json:"original_entity"`
	CanonicalName    string             `json:"canonical_name"`
	SMILES           string             `json:"smiles"`
	InChI            string             `json:"inchi"`
	InChIKey         string             `json:"inchi_key"`
	MolecularFormula string             `json:"molecular_formula"`
	CASNumber        string             `json:"cas_number"`
	MolecularWeight  float64            `json:"molecular_weight"`
	IsResolved       bool               `json:"is_resolved"`
	ResolutionMethod string             `json:"resolution_method"`
	Synonyms         []string           `json:"synonyms"`
}

// MoleculeLink represents a link to the molecule database.
type MoleculeLink struct {
	Entity          *ResolvedChemicalEntity `json:"entity"`
	MoleculeID      string                  `json:"molecule_id"`
	PubChemCID      int                     `json:"pubchem_cid"`
	ChEMBLID        string                  `json:"chembl_id"`
	DrugBankID      string                  `json:"drugbank_id"`
	IsExactMatch    bool                    `json:"is_exact_match"`
	SimilarityScore float64                 `json:"similarity_score"`
}

// ExtractionResult represents the result of extraction.
type ExtractionResult struct {
	Entities         []*RawChemicalEntity `json:"entities"`
	EntityCount      int                  `json:"entity_count"`
	ProcessingTimeMs int64                `json:"processing_time_ms"`
	TextLength       int                  `json:"text_length"`
	Coverage         float64              `json:"coverage"`
}

// ClaimExtractionResult represents extraction from a claim.
type ClaimExtractionResult struct {
	ClaimNumber            int                                     `json:"claim_number"`
	Entities               []*RawChemicalEntity                    `json:"entities"`
	FeatureEntityMapping   map[string][]*RawChemicalEntity         `json:"feature_entity_mapping"`
	MarkushVariableMapping map[string][]*ResolvedChemicalEntity    `json:"markush_variable_mapping"`
}

// ExtractorConfig holds configuration for extractor.
type ExtractorConfig struct {
	ConfidenceThreshold float64 `json:"confidence_threshold"`
	EnableDictionary    bool    `json:"enable_dictionary"`
}

// ChemicalExtractor defines the interface for chemical entity extraction.
type ChemicalExtractor interface {
	Extract(ctx context.Context, text string) (*ExtractionResult, error)
	ExtractBatch(ctx context.Context, texts []string) ([]*ExtractionResult, error)
	ExtractFromClaim(ctx context.Context, claim *claim_bert.ParsedClaim) (*ClaimExtractionResult, error)
	Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error)
	LinkToMolecule(ctx context.Context, entity *ResolvedChemicalEntity) (*MoleculeLink, error)
}

// chemicalExtractorImpl implements ChemicalExtractor.
type chemicalExtractorImpl struct {
	ner       NERModel
	resolver  EntityResolver
	validator EntityValidator
	config    ExtractorConfig
	metrics   common.IntelligenceMetrics
	logger    logging.Logger
}

// NewChemicalExtractor creates a new ChemicalExtractor.
func NewChemicalExtractor(
	ner NERModel,
	resolver EntityResolver,
	validator EntityValidator,
	config ExtractorConfig,
	metrics common.IntelligenceMetrics,
	logger logging.Logger,
) ChemicalExtractor {
	return &chemicalExtractorImpl{
		ner:       ner,
		resolver:  resolver,
		validator: validator,
		config:    config,
		metrics:   metrics,
		logger:    logger,
	}
}

func (e *chemicalExtractorImpl) Extract(ctx context.Context, text string) (*ExtractionResult, error) {
	start := time.Now()

	// 1. NER Prediction
	nerRes, err := e.ner.Predict(ctx, text)
	if err != nil {
		return nil, err
	}

	var entities []*RawChemicalEntity
	for _, nerEnt := range nerRes.Entities {
		raw := &RawChemicalEntity{
			Text:        nerEnt.Text,
			StartOffset: nerEnt.StartChar,
			EndOffset:   nerEnt.EndChar,
			EntityType:  mapLabelToType(nerEnt.Label),
			Confidence:  nerEnt.Score,
			Context:     getContext(text, nerEnt.StartChar, nerEnt.EndChar),
		}

		// 2. Validation
		if e.validator != nil {
			valRes, err := e.validator.Validate(ctx, raw)
			if err == nil && valRes.IsValid && valRes.AdjustedConfidence >= e.config.ConfidenceThreshold {
				raw.Confidence = valRes.AdjustedConfidence
				raw.EntityType = valRes.AdjustedType
				entities = append(entities, raw)
			}
		} else if raw.Confidence >= e.config.ConfidenceThreshold {
			entities = append(entities, raw)
		}
	}

	return &ExtractionResult{
		Entities:         entities,
		EntityCount:      len(entities),
		ProcessingTimeMs: time.Since(start).Milliseconds(),
		TextLength:       len(text),
		Coverage:         calculateCoverage(entities, len(text)),
	}, nil
}

func (e *chemicalExtractorImpl) ExtractBatch(ctx context.Context, texts []string) ([]*ExtractionResult, error) {
	var results []*ExtractionResult
	for _, text := range texts {
		res, err := e.Extract(ctx, text)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (e *chemicalExtractorImpl) ExtractFromClaim(ctx context.Context, claim *claim_bert.ParsedClaim) (*ClaimExtractionResult, error) {
	// Extract from body
	res, err := e.Extract(ctx, claim.Body)
	if err != nil {
		return nil, err
	}

	// Map to features
	mapping := make(map[string][]*RawChemicalEntity)
	// Simple mapping: check offset overlap or containment
	// ParsedClaim Features have offsets.

	for _, feat := range claim.Features {
		var matched []*RawChemicalEntity
		for _, ent := range res.Entities {
			// Offset check logic relative to body
			// Assuming offsets are consistent
			if ent.StartOffset >= feat.StartOffset && ent.EndOffset <= feat.EndOffset {
				matched = append(matched, ent)
			}
		}
		mapping[feat.ID] = matched
	}

	return &ClaimExtractionResult{
		ClaimNumber:          claim.ClaimNumber,
		Entities:             res.Entities,
		FeatureEntityMapping: mapping,
	}, nil
}

func (e *chemicalExtractorImpl) Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
	if e.resolver == nil {
		return nil, errors.New("resolver not configured")
	}
	return e.resolver.Resolve(ctx, entity)
}

func (e *chemicalExtractorImpl) LinkToMolecule(ctx context.Context, entity *ResolvedChemicalEntity) (*MoleculeLink, error) {
	// Mock linking logic
	if !entity.IsResolved {
		return nil, errors.New("entity not resolved")
	}
	return &MoleculeLink{
		Entity:          entity,
		IsExactMatch:    true,
		MoleculeID:      "mol-123", // Mock ID
		SimilarityScore: 1.0,
	}, nil
}

// Helpers
func mapLabelToType(label string) ChemicalEntityType {
	switch label {
	case "IUPAC": return EntityIUPACName
	case "COMMON": return EntityCommonName
	case "SMILES": return EntitySMILES
	case "FORMULA": return EntityMolecularFormula
	case "CAS": return EntityCASNumber
	case "MARKUSH": return EntityMarkushVariable
	default: return EntityCommonName
	}
}

func getContext(text string, start, end int) string {
	s := start - 50
	e := end + 50
	if s < 0 { s = 0 }
	if e > len(text) { e = len(text) }
	return text[s:e]
}

func calculateCoverage(entities []*RawChemicalEntity, textLen int) float64 {
	if textLen == 0 { return 0 }
	covered := 0
	for _, e := range entities {
		covered += (e.EndOffset - e.StartOffset)
	}
	return float64(covered) / float64(textLen)
}

//Personal.AI order the ending
