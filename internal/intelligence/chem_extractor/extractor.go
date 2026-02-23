package chem_extractor

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// ChemicalEntityType enumeration
// ---------------------------------------------------------------------------

// ChemicalEntityType classifies the kind of chemical entity recognised.
type ChemicalEntityType string

const (
	EntityIUPACName        ChemicalEntityType = "IUPAC_NAME"
	EntityCommonName       ChemicalEntityType = "COMMON_NAME"
	EntityBrandName        ChemicalEntityType = "BRAND_NAME"
	EntityMolecularFormula ChemicalEntityType = "MOLECULAR_FORMULA"
	EntitySMILES           ChemicalEntityType = "SMILES"
	EntityInChI            ChemicalEntityType = "INCHI"
	EntityCASNumber        ChemicalEntityType = "CAS_NUMBER"
	EntityGenericStructure ChemicalEntityType = "GENERIC_STRUCTURE"
	EntityPolymer          ChemicalEntityType = "POLYMER"
	EntityBiological       ChemicalEntityType = "BIOLOGICAL"
	EntityMarkushVariable  ChemicalEntityType = "MARKUSH_VARIABLE"
)

// ---------------------------------------------------------------------------
// Core data structures
// ---------------------------------------------------------------------------

// RawChemicalEntity is a single chemical mention found in text.
type RawChemicalEntity struct {
	Text        string             `json:"text"`
	StartOffset int                `json:"start_offset"`
	EndOffset   int                `json:"end_offset"`
	EntityType  ChemicalEntityType `json:"entity_type"`
	Confidence  float64            `json:"confidence"`
	Context     string             `json:"context"`
	Source      string             `json:"source"`
	IsNested    bool               `json:"is_nested,omitempty"`
	ParentText  string             `json:"parent_text,omitempty"`
}

// ResolvedChemicalEntity is the standardised representation after resolution.
type ResolvedChemicalEntity struct {
	OriginalEntity   *RawChemicalEntity `json:"original_entity"`
	CanonicalName    string             `json:"canonical_name"`
	SMILES           string             `json:"smiles,omitempty"`
	InChI            string             `json:"inchi,omitempty"`
	InChIKey         string             `json:"inchi_key,omitempty"`
	MolecularFormula string             `json:"molecular_formula,omitempty"`
	CASNumber        string             `json:"cas_number,omitempty"`
	MolecularWeight  float64            `json:"molecular_weight,omitempty"`
	IsResolved       bool               `json:"is_resolved"`
	ResolutionMethod string             `json:"resolution_method"`
	Synonyms         []string           `json:"synonyms,omitempty"`
}

// MoleculeLink connects a resolved entity to external molecule databases.
type MoleculeLink struct {
	Entity          *ResolvedChemicalEntity `json:"entity"`
	MoleculeID      string                  `json:"molecule_id,omitempty"`
	PubChemCID      int                     `json:"pubchem_cid,omitempty"`
	ChEMBLID        string                  `json:"chembl_id,omitempty"`
	DrugBankID      string                  `json:"drugbank_id,omitempty"`
	IsExactMatch    bool                    `json:"is_exact_match"`
	SimilarityScore float64                 `json:"similarity_score,omitempty"`
}

// ExtractionResult is the output of a single Extract call.
type ExtractionResult struct {
	Entities        []*RawChemicalEntity `json:"entities"`
	EntityCount     int                  `json:"entity_count"`
	ProcessingTimeMs int64              `json:"processing_time_ms"`
	TextLength      int                  `json:"text_length"`
	Coverage        float64              `json:"coverage"`
}

// ParsedClaim is a simplified representation of a patent claim (from claim_bert).
type ParsedClaim struct {
	ClaimNumber       int                `json:"claim_number"`
	ClaimText         string             `json:"claim_text"`
	TechnicalFeatures []*TechnicalFeature `json:"technical_features,omitempty"`
}

// TechnicalFeature is a single technical feature within a claim.
type TechnicalFeature struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
}

// ClaimExtractionResult is the output of ExtractFromClaim.
type ClaimExtractionResult struct {
	ClaimNumber            int                                       `json:"claim_number"`
	Entities               []*RawChemicalEntity                      `json:"entities"`
	FeatureEntityMapping   map[string][]*RawChemicalEntity           `json:"feature_entity_mapping"`
	MarkushVariableMapping map[string][]*ResolvedChemicalEntity      `json:"markush_variable_mapping"`
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// ExtractorConfig holds tuneable parameters for the extraction pipeline.
type ExtractorConfig struct {
	MinConfidence          float64 `json:"min_confidence" yaml:"min_confidence"`
	ContextWindowSize      int     `json:"context_window_size" yaml:"context_window_size"`
	EnableDictionaryLookup bool    `json:"enable_dictionary_lookup" yaml:"enable_dictionary_lookup"`
	EnableNER              bool    `json:"enable_ner" yaml:"enable_ner"`
	MaxTextLength          int     `json:"max_text_length" yaml:"max_text_length"`
	BatchConcurrency       int     `json:"batch_concurrency" yaml:"batch_concurrency"`
}

// DefaultExtractorConfig returns production-ready defaults.
func DefaultExtractorConfig() ExtractorConfig {
	return ExtractorConfig{
		MinConfidence:          0.60,
		ContextWindowSize:      50,
		EnableDictionaryLookup: true,
		EnableNER:              true,
		MaxTextLength:          500000,
		BatchConcurrency:       4,
	}
}

// ---------------------------------------------------------------------------
// Dependency interfaces
// ---------------------------------------------------------------------------

// NERModel performs named-entity recognition for chemical entities.
type NERModel interface {
	Predict(ctx context.Context, text string) ([]*RawChemicalEntity, error)
	PredictBatch(ctx context.Context, texts []string) ([][]*RawChemicalEntity, error)
}

// EntityResolver standardises a raw entity into a canonical representation.
type EntityResolver interface {
	Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error)
}

// EntityValidator checks whether a raw entity is plausible.
type EntityValidator interface {
	Validate(ctx context.Context, entity *RawChemicalEntity) (bool, error)
}

// ChemicalDictionary provides fast exact-match lookup for known compounds.
type ChemicalDictionary interface {
	Lookup(name string) (*DictionaryEntry, bool)
	LookupCAS(cas string) (*DictionaryEntry, bool)
	Size() int
}

// DictionaryEntry is a single record in the chemical dictionary.
type DictionaryEntry struct {
	CanonicalName    string             `json:"canonical_name"`
	EntityType       ChemicalEntityType `json:"entity_type"`
	SMILES           string             `json:"smiles,omitempty"`
	CASNumber        string             `json:"cas_number,omitempty"`
	MolecularFormula string             `json:"molecular_formula,omitempty"`
	MolecularWeight  float64            `json:"molecular_weight,omitempty"`
	Synonyms         []string           `json:"synonyms,omitempty"`
}

// MoleculeDatabase abstracts the internal molecule store.
type MoleculeDatabase interface {
	FindByCAS(ctx context.Context, cas string) (*MoleculeRecord, error)
	FindBySMILES(ctx context.Context, smiles string) (*MoleculeRecord, error)
	FindByInChIKey(ctx context.Context, inchiKey string) (*MoleculeRecord, error)
	FindByNameFuzzy(ctx context.Context, name string) (*MoleculeRecord, float64, error)
}

// ExternalChemDB abstracts external databases such as PubChem.
type ExternalChemDB interface {
	LookupByName(ctx context.Context, name string) (*ExternalMoleculeRecord, error)
	LookupBySMILES(ctx context.Context, smiles string) (*ExternalMoleculeRecord, error)
}

// MoleculeRecord is a row from the internal molecule database.
type MoleculeRecord struct {
	MoleculeID string  `json:"molecule_id"`
	SMILES     string  `json:"smiles"`
	InChIKey   string  `json:"inchi_key"`
	CASNumber  string  `json:"cas_number"`
	Name       string  `json:"name"`
}

// ExternalMoleculeRecord is a row from an external database.
type ExternalMoleculeRecord struct {
	PubChemCID int    `json:"pubchem_cid,omitempty"`
	ChEMBLID   string `json:"chembl_id,omitempty"`
	DrugBankID string `json:"drugbank_id,omitempty"`
	SMILES     string `json:"smiles,omitempty"`
	Name       string `json:"name,omitempty"`
}

// Metrics records operational telemetry.
type Metrics interface {
	RecordExtraction(ctx context.Context, entityCount int, durationMs float64)
	RecordResolution(ctx context.Context, method string, success bool)
	RecordLinkage(ctx context.Context, exact bool)
}

// Logger is a minimal structured logger.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// ---------------------------------------------------------------------------
// ChemicalExtractor interface
// ---------------------------------------------------------------------------

// ChemicalExtractor is the top-level API for chemical entity extraction.
type ChemicalExtractor interface {
	Extract(ctx context.Context, text string) (*ExtractionResult, error)
	ExtractBatch(ctx context.Context, texts []string) ([]*ExtractionResult, error)
	ExtractFromClaim(ctx context.Context, claim *ParsedClaim) (*ClaimExtractionResult, error)
	Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error)
	LinkToMolecule(ctx context.Context, entity *ResolvedChemicalEntity) (*MoleculeLink, error)
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

type chemicalExtractorImpl struct {
	nerModel   NERModel
	resolver   EntityResolver
	validator  EntityValidator
	dictionary ChemicalDictionary
	moleculeDB MoleculeDatabase
	externalDB ExternalChemDB
	config     ExtractorConfig
	metrics    Metrics
	logger     Logger

	casRe      *regexp.Regexp
	formulaRe  *regexp.Regexp
	smilesRe   *regexp.Regexp
	markushRe  *regexp.Regexp
}

// NewChemicalExtractor constructs a fully-wired extractor.
func NewChemicalExtractor(
	nerModel NERModel,
	resolver EntityResolver,
	validator EntityValidator,
	dictionary ChemicalDictionary,
	moleculeDB MoleculeDatabase,
	externalDB ExternalChemDB,
	config ExtractorConfig,
	metrics Metrics,
	logger Logger,
) (ChemicalExtractor, error) {
	if nerModel == nil && config.EnableNER {
		return nil, errors.NewInvalidInputError("NER model is required when NER is enabled")
	}
	if resolver == nil {
		return nil, errors.NewInvalidInputError("entity resolver is required")
	}
	if logger == nil {
		logger = &noopLogger{}
	}
	if metrics == nil {
		metrics = &noopMetrics{}
	}

	return &chemicalExtractorImpl{
		nerModel:   nerModel,
		resolver:   resolver,
		validator:  validator,
		dictionary: dictionary,
		moleculeDB: moleculeDB,
		externalDB: externalDB,
		config:     config,
		metrics:    metrics,
		logger:     logger,
		casRe:      regexp.MustCompile(`\b\d{2,7}-\d{2}-\d\b`),
		formulaRe:  regexp.MustCompile(`\b([A-Z][a-z]?\d*){2,}\b`),
		smilesRe:   regexp.MustCompile(`(?:^|[\s(])([A-Za-z0-9@+\-\[\]\\\/()=#$.%]+)(?:[\s)]|$)`),
		markushRe:  regexp.MustCompile(`\b(?:R\d{0,2}|X|Y|Z|Ar|Het)\b`),
	}, nil
}

// ---------------------------------------------------------------------------
// Extract
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) Extract(ctx context.Context, text string) (*ExtractionResult, error) {
	if text == "" {
		return &ExtractionResult{
			Entities:    []*RawChemicalEntity{},
			EntityCount: 0,
			TextLength:  0,
			Coverage:    0,
		}, nil
	}

	start := time.Now()

	// 1. Text pre-processing: Unicode NFC normalisation, collapse whitespace.
	cleaned := normaliseText(text)
	if len(cleaned) > e.config.MaxTextLength {
		cleaned = cleaned[:e.config.MaxTextLength]
	}

	var dictEntities []*RawChemicalEntity
	var nerEntities []*RawChemicalEntity

	// 2. Dictionary fast-match (CAS regex + dictionary lookup).
	if e.config.EnableDictionaryLookup && e.dictionary != nil {
		dictEntities = e.dictionaryMatch(cleaned)
	}

	// 3. Regex-based pattern matching (CAS, formula, SMILES-like, Markush).
	regexEntities := e.regexMatch(cleaned)

	// 4. NER model inference for entities not covered by dictionary / regex.
	if e.config.EnableNER && e.nerModel != nil {
		var err error
		nerEntities, err = e.nerModel.Predict(ctx, cleaned)
		if err != nil {
			e.logger.Warn("NER prediction failed, continuing with dictionary/regex only", "error", err)
		}
	}

	// 5. Merge all sources, resolve overlaps.
	merged := e.mergeEntities(dictEntities, regexEntities, nerEntities)

	// 6. Attach context windows.
	for _, ent := range merged {
		ent.Context = extractContext(cleaned, ent.StartOffset, ent.EndOffset, e.config.ContextWindowSize)
	}

	// 7. Validate each entity.
	validated := e.validateEntities(ctx, merged)

	// 8. Filter by minimum confidence.
	filtered := filterByConfidence(validated, e.config.MinConfidence)

	// 9. Sort by offset for deterministic output.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartOffset < filtered[j].StartOffset
	})

	elapsed := time.Since(start).Milliseconds()
	coverage := computeCoverage(filtered, len(cleaned))

	e.metrics.RecordExtraction(ctx, len(filtered), float64(elapsed))

	return &ExtractionResult{
		Entities:         filtered,
		EntityCount:      len(filtered),
		ProcessingTimeMs: elapsed,
		TextLength:       len(cleaned),
		Coverage:         coverage,
	}, nil
}

// ---------------------------------------------------------------------------
// ExtractBatch
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) ExtractBatch(ctx context.Context, texts []string) ([]*ExtractionResult, error) {
	if len(texts) == 0 {
		return []*ExtractionResult{}, nil
	}

	results := make([]*ExtractionResult, len(texts))
	errs := make([]error, len(texts))

	concurrency := e.config.BatchConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, txt := range texts {
		wg.Add(1)
		go func(idx int, t string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := e.Extract(ctx, t)
			results[idx] = res
			errs[idx] = err
		}(i, txt)
	}
	wg.Wait()

	// If every single extraction failed, return the first error.
	allFailed := true
	for i := range results {
		if errs[i] == nil {
			allFailed = false
		} else if results[i] == nil {
			results[i] = &ExtractionResult{Entities: []*RawChemicalEntity{}}
		}
	}
	if allFailed {
		return results, fmt.Errorf("all %d extractions failed; first error: %w", len(texts), errs[0])
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// ExtractFromClaim
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) ExtractFromClaim(ctx context.Context, claim *ParsedClaim) (*ClaimExtractionResult, error) {
	if claim == nil {
		return nil, errors.NewInvalidInputError("claim is required")
	}

	// 1. Extract entities from the full claim text.
	exResult, err := e.Extract(ctx, claim.ClaimText)
	if err != nil {
		return nil, fmt.Errorf("extraction from claim text: %w", err)
	}

	// 2. Map entities to technical features by offset overlap.
	featureMapping := make(map[string][]*RawChemicalEntity)
	for _, feat := range claim.TechnicalFeatures {
		for _, ent := range exResult.Entities {
			if offsetOverlaps(ent.StartOffset, ent.EndOffset, feat.StartOffset, feat.EndOffset) {
				featureMapping[feat.ID] = append(featureMapping[feat.ID], ent)
			}
		}
	}

	// 3. Identify Markush variables and attempt resolution.
	markushMapping := make(map[string][]*ResolvedChemicalEntity)
	for _, ent := range exResult.Entities {
		if ent.EntityType == EntityMarkushVariable {
			resolved, resolveErr := e.resolveMarkushVariable(ctx, ent, claim.ClaimText)
			if resolveErr != nil {
				e.logger.Warn("markush resolution failed", "variable", ent.Text, "error", resolveErr)
				continue
			}
			markushMapping[ent.Text] = resolved
		}
	}

	return &ClaimExtractionResult{
		ClaimNumber:            claim.ClaimNumber,
		Entities:               exResult.Entities,
		FeatureEntityMapping:   featureMapping,
		MarkushVariableMapping: markushMapping,
	}, nil
}

// ---------------------------------------------------------------------------
// Resolve
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
	if entity == nil {
		return nil, errors.NewInvalidInputError("entity is required")
	}
	resolved, err := e.resolver.Resolve(ctx, entity)
	if err != nil {
		e.metrics.RecordResolution(ctx, "unknown", false)
		return nil, err
	}
	e.metrics.RecordResolution(ctx, resolved.ResolutionMethod, resolved.IsResolved)
	return resolved, nil
}

// ---------------------------------------------------------------------------
// LinkToMolecule
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) LinkToMolecule(ctx context.Context, entity *ResolvedChemicalEntity) (*MoleculeLink, error) {
	if entity == nil {
		return nil, errors.NewInvalidInputError("resolved entity is required")
	}

	link := &MoleculeLink{Entity: entity}

	// Strategy 1: CAS number exact match.
	if entity.CASNumber != "" && e.moleculeDB != nil {
		rec, err := e.moleculeDB.FindByCAS(ctx, entity.CASNumber)
		if err == nil && rec != nil {
			link.MoleculeID = rec.MoleculeID
			link.IsExactMatch = true
			link.SimilarityScore = 1.0
			e.metrics.RecordLinkage(ctx, true)
			return link, nil
		}
	}

	// Strategy 2: SMILES structural exact match.
	if entity.SMILES != "" && e.moleculeDB != nil {
		rec, err := e.moleculeDB.FindBySMILES(ctx, entity.SMILES)
		if err == nil && rec != nil {
			link.MoleculeID = rec.MoleculeID
			link.IsExactMatch = true
			link.SimilarityScore = 1.0
			e.metrics.RecordLinkage(ctx, true)
			return link, nil
		}
	}

	// Strategy 3: InChIKey exact match.
	if entity.InChIKey != "" && e.moleculeDB != nil {
		rec, err := e.moleculeDB.FindByInChIKey(ctx, entity.InChIKey)
		if err == nil && rec != nil {
			link.MoleculeID = rec.MoleculeID
			link.IsExactMatch = true
			link.SimilarityScore = 1.0
			e.metrics.RecordLinkage(ctx, true)
			return link, nil
		}
	}

	// Strategy 4: Name fuzzy match.
	if entity.CanonicalName != "" && e.moleculeDB != nil {
		rec, score, err := e.moleculeDB.FindByNameFuzzy(ctx, entity.CanonicalName)
		if err == nil && rec != nil && score > 0.70 {
			link.MoleculeID = rec.MoleculeID
			link.IsExactMatch = false
			link.SimilarityScore = score
			e.metrics.RecordLinkage(ctx, false)
			return link, nil
		}
	}

	// Strategy 5: External database (PubChem, etc.).
	if e.externalDB != nil {
		queryName := entity.CanonicalName
		if queryName == "" && entity.OriginalEntity != nil {
			queryName = entity.OriginalEntity.Text
		}
		if queryName != "" {
			ext, err := e.externalDB.LookupByName(ctx, queryName)
			if err == nil && ext != nil {
				link.PubChemCID = ext.PubChemCID
				link.ChEMBLID = ext.ChEMBLID
				link.DrugBankID = ext.DrugBankID
				link.IsExactMatch = false
				link.SimilarityScore = 0.0
				e.metrics.RecordLinkage(ctx, false)
				return link, nil
			}
		}
		if entity.SMILES != "" {
			ext, err := e.externalDB.LookupBySMILES(ctx, entity.SMILES)
			if err == nil && ext != nil {
				link.PubChemCID = ext.PubChemCID
				link.ChEMBLID = ext.ChEMBLID
				link.DrugBankID = ext.DrugBankID
				link.IsExactMatch = false
				link.SimilarityScore = 0.0
				e.metrics.RecordLinkage(ctx, false)
				return link, nil
			}
		}
	}

	// Nothing found anywhere.
	return nil, nil
}

// ---------------------------------------------------------------------------
// Dictionary matching
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) dictionaryMatch(text string) []*RawChemicalEntity {
	var entities []*RawChemicalEntity
	lower := strings.ToLower(text)

	// Scan every word boundary token and try dictionary lookup.
	words := tokenise(text)
	for _, w := range words {
		entry, found := e.dictionary.Lookup(strings.ToLower(w.text))
		if !found {
			continue
		}
		entities = append(entities, &RawChemicalEntity{
			Text:        w.text,
			StartOffset: w.start,
			EndOffset:   w.start + len(w.text),
			EntityType:  entry.EntityType,
			Confidence:  1.0,
			Source:      "dictionary",
		})
	}

	// Also scan for CAS numbers via regex and look them up.
	casMatches := e.casRe.FindAllStringIndex(lower, -1)
	for _, loc := range casMatches {
		cas := text[loc[0]:loc[1]]
		entry, found := e.dictionary.LookupCAS(cas)
		if !found {
			// Still a valid CAS pattern even if not in dictionary.
			entities = append(entities, &RawChemicalEntity{
				Text:        cas,
				StartOffset: loc[0],
				EndOffset:   loc[1],
				EntityType:  EntityCASNumber,
				Confidence:  0.90,
				Source:      "regex",
			})
			continue
		}
		entities = append(entities, &RawChemicalEntity{
			Text:        cas,
			StartOffset: loc[0],
			EndOffset:   loc[1],
			EntityType:  EntityCASNumber,
			Confidence:  1.0,
			Source:      "dictionary",
		})
		_ = entry
	}

	return entities
}

// ---------------------------------------------------------------------------
// Regex-based pattern matching
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) regexMatch(text string) []*RawChemicalEntity {
	var entities []*RawChemicalEntity

	// Molecular formula: e.g. C9H8O4
	formulaRe := regexp.MustCompile(`\b((?:[A-Z][a-z]?\d*){2,})\b`)
	for _, loc := range formulaRe.FindAllStringSubmatchIndex(text, -1) {
		candidate := text[loc[2]:loc[3]]
		if looksLikeMolecularFormula(candidate) {
			entities = append(entities, &RawChemicalEntity{
				Text:        candidate,
				StartOffset: loc[2],
				EndOffset:   loc[3],
				EntityType:  EntityMolecularFormula,
				Confidence:  0.85,
				Source:      "regex",
			})
		}
	}

	// SMILES-like strings (heuristic: contains ring digits, branches, bond symbols).
	smilesRe := regexp.MustCompile(`(?:^|[\s,(])([A-Za-z0-9@+\-\[\]\\\/()=#$.%]{5,})(?:[\s,)]|$)`)
	for _, loc := range smilesRe.FindAllStringSubmatchIndex(text, -1) {
		candidate := text[loc[2]:loc[3]]
		if looksLikeSMILES(candidate) {
			entities = append(entities, &RawChemicalEntity{
				Text:        candidate,
				StartOffset: loc[2],
				EndOffset:   loc[3],
				EntityType:  EntitySMILES,
				Confidence:  0.80,
				Source:      "regex",
			})
		}
	}

	// Markush variables: R1, R2, X, Y, Ar, Het
	markushRe := regexp.MustCompile(`\b(R\d{0,2}|X|Y|Z|Ar|Het)\s+(?:is|represents|denotes|=)\s+`)
	for _, loc := range markushRe.FindAllStringSubmatchIndex(text, -1) {
		varName := text[loc[2]:loc[3]]
		entities = append(entities, &RawChemicalEntity{
			Text:        varName,
			StartOffset: loc[2],
			EndOffset:   loc[3],
			EntityType:  EntityMarkushVariable,
			Confidence:  0.90,
			Source:      "regex",
		})
	}

	return entities
}

// ---------------------------------------------------------------------------
// Merge & overlap resolution
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) mergeEntities(sources ...[]*RawChemicalEntity) []*RawChemicalEntity {
	var all []*RawChemicalEntity
	for _, src := range sources {
		all = append(all, src...)
	}
	if len(all) == 0 {
		return nil
	}

	// Sort by start offset, then by descending length.
	sort.Slice(all, func(i, j int) bool {
		if all[i].StartOffset != all[j].StartOffset {
			return all[i].StartOffset < all[j].StartOffset
		}
		return (all[i].EndOffset - all[i].StartOffset) > (all[j].EndOffset - all[j].StartOffset)
	})

	// Deduplicate exact spans.
	deduped := deduplicateExactSpans(all)

	// Resolve overlapping spans.
	resolved := resolveOverlaps(deduped)

	return resolved
}

func deduplicateExactSpans(entities []*RawChemicalEntity) []*RawChemicalEntity {
	type spanKey struct {
		start int
		end   int
	}
	seen := make(map[spanKey]*RawChemicalEntity)
	for _, ent := range entities {
		key := spanKey{ent.StartOffset, ent.EndOffset}
		existing, ok := seen[key]
		if !ok || ent.Confidence > existing.Confidence {
			seen[key] = ent
		}
	}
	result := make([]*RawChemicalEntity, 0, len(seen))
	for _, ent := range seen {
		result = append(result, ent)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartOffset < result[j].StartOffset
	})
	return result
}

func resolveOverlaps(entities []*RawChemicalEntity) []*RawChemicalEntity {
	if len(entities) <= 1 {
		return entities
	}

	var result []*RawChemicalEntity

	for i := 0; i < len(entities); i++ {
		current := entities[i]
		if i+1 < len(entities) {
			next := entities[i+1]

			if spansOverlap(current.StartOffset, current.EndOffset, next.StartOffset, next.EndOffset) {
				// Check for nesting: one fully contains the other.
				if isNested(current, next) {
					// Keep both, mark the inner one as nested.
					outer, inner := current, next
					if spanLength(inner) > spanLength(outer) {
						outer, inner = inner, outer
					}
					inner.IsNested = true
					inner.ParentText = outer.Text
					result = append(result, outer, inner)
					i++ // skip next since we consumed it
					continue
				}

				// True overlap (not nesting): keep the one with higher confidence.
				winner := pickOverlapWinner(current, next)
				result = append(result, winner)
				i++ // skip next since we consumed it
				continue
			}
		}
		result = append(result, current)
	}

	return result
}

func pickOverlapWinner(a, b *RawChemicalEntity) *RawChemicalEntity {
	if a.Confidence > b.Confidence {
		return a
	}
	if b.Confidence > a.Confidence {
		return b
	}
	// Same confidence → keep the longer (more specific) entity.
	if spanLength(a) >= spanLength(b) {
		return a
	}
	return b
}

func isNested(a, b *RawChemicalEntity) bool {
	// a fully contains b, or b fully contains a.
	return (a.StartOffset <= b.StartOffset && a.EndOffset >= b.EndOffset) ||
		(b.StartOffset <= a.StartOffset && b.EndOffset >= a.EndOffset)
}

func spansOverlap(s1, e1, s2, e2 int) bool {
	return s1 < e2 && s2 < e1
}

func offsetOverlaps(s1, e1, s2, e2 int) bool {
	return spansOverlap(s1, e1, s2, e2)
}

func spanLength(e *RawChemicalEntity) int {
	return e.EndOffset - e.StartOffset
}

// ---------------------------------------------------------------------------
// Validation pass
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) validateEntities(ctx context.Context, entities []*RawChemicalEntity) []*RawChemicalEntity {
	if e.validator == nil {
		return entities
	}
	var valid []*RawChemicalEntity
	for _, ent := range entities {
		ok, err := e.validator.Validate(ctx, ent)
		if err != nil {
			e.logger.Debug("validation error, keeping entity", "text", ent.Text, "error", err)
			valid = append(valid, ent)
			continue
		}
		if ok {
			valid = append(valid, ent)
		} else {
			e.logger.Debug("entity failed validation, discarding", "text", ent.Text, "type", ent.EntityType)
		}
	}
	return valid
}

// ---------------------------------------------------------------------------
// Markush variable resolution
// ---------------------------------------------------------------------------

func (e *chemicalExtractorImpl) resolveMarkushVariable(ctx context.Context, entity *RawChemicalEntity, claimText string) ([]*ResolvedChemicalEntity, error) {
	// Find the definition clause after the variable, e.g. "R1 is C1-C6 alkyl".
	varName := entity.Text
	defPattern := regexp.MustCompile(
		fmt.Sprintf(`%s\s+(?:is|represents|denotes|=)\s+([^;.]+)`, regexp.QuoteMeta(varName)),
	)
	match := defPattern.FindStringSubmatch(claimText)
	if match == nil || len(match) < 2 {
		return nil, fmt.Errorf("no definition found for Markush variable %s", varName)
	}

	definition := strings.TrimSpace(match[1])

	// Expand common patterns like "C1-C6 alkyl" into individual substituents.
	expanded := expandMarkushDefinition(definition)

	var resolved []*ResolvedChemicalEntity
	for _, name := range expanded {
		raw := &RawChemicalEntity{
			Text:       name,
			EntityType: EntityGenericStructure,
			Confidence: 0.75,
			Source:     "markush_expansion",
		}
		res, err := e.resolver.Resolve(ctx, raw)
		if err != nil {
			e.logger.Debug("markush substituent resolution failed", "name", name, "error", err)
			// Still include an unresolved entry.
			resolved = append(resolved, &ResolvedChemicalEntity{
				OriginalEntity:   raw,
				CanonicalName:    name,
				IsResolved:       false,
				ResolutionMethod: "markush_expansion",
			})
			continue
		}
		resolved = append(resolved, res)
	}
	return resolved, nil
}

// expandMarkushDefinition expands shorthand like "C1-C6 alkyl" into
// ["methyl", "ethyl", "propyl", "butyl", "pentyl", "hexyl"].
func expandMarkushDefinition(definition string) []string {
	alkylRe := regexp.MustCompile(`C(\d+)\s*-\s*C(\d+)\s+alkyl`)
	if m := alkylRe.FindStringSubmatch(definition); m != nil {
		lo := atoi(m[1])
		hi := atoi(m[2])
		return expandAlkylRange(lo, hi)
	}

	// Comma / "or" separated list: "hydrogen, methyl, ethyl, or phenyl"
	parts := regexp.MustCompile(`[,;]\s*|\s+or\s+|\s+and\s+`).Split(definition, -1)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) > 0 {
		return result
	}

	return []string{definition}
}

var alkylNames = []string{
	"", "methyl", "ethyl", "propyl", "butyl", "pentyl",
	"hexyl", "heptyl", "octyl", "nonyl", "decyl",
	"undecyl", "dodecyl",
}

func expandAlkylRange(lo, hi int) []string {
	if lo < 1 {
		lo = 1
	}
	if hi >= len(alkylNames) {
		hi = len(alkylNames) - 1
	}
	var result []string
	for i := lo; i <= hi; i++ {
		if i < len(alkylNames) {
			result = append(result, alkylNames[i])
		}
	}
	return result
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// Text utilities
// ---------------------------------------------------------------------------

func normaliseText(text string) string {
	// NFC normalisation.
	text = norm.NFC.String(text)
	// Replace non-breaking spaces and other whitespace variants with regular space.
	var b strings.Builder
	b.Grow(len(text))
	prevSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

type wordToken struct {
	text  string
	start int
}

func tokenise(text string) []wordToken {
	var tokens []wordToken
	inWord := false
	wordStart := 0
	for i, r := range text {
		isWordChar := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '\''
		if isWordChar {
			if !inWord {
				wordStart = i
				inWord = true
			}
		} else {
			if inWord {
				tokens = append(tokens, wordToken{text: text[wordStart:i], start: wordStart})
				inWord = false
			}
		}
	}
	if inWord {
		tokens = append(tokens, wordToken{text: text[wordStart:], start: wordStart})
	}
	return tokens
}

func extractContext(text string, start, end, windowSize int) string {
	ctxStart := start - windowSize
	if ctxStart < 0 {
		ctxStart = 0
	}
	ctxEnd := end + windowSize
	if ctxEnd > len(text) {
		ctxEnd = len(text)
	}
	return text[ctxStart:ctxEnd]
}

func filterByConfidence(entities []*RawChemicalEntity, minConf float64) []*RawChemicalEntity {
	var result []*RawChemicalEntity
	for _, e := range entities {
		if e.Confidence >= minConf {
			result = append(result, e)
		}
	}
	return result
}

func computeCoverage(entities []*RawChemicalEntity, textLen int) float64 {
	if textLen == 0 {
		return 0
	}
	covered := 0
	for _, e := range entities {
		length := e.EndOffset - e.StartOffset
		if length > 0 {
			covered += length
		}
	}
	cov := float64(covered) / float64(textLen)
	if cov > 1.0 {
		cov = 1.0
	}
	return cov
}

// ---------------------------------------------------------------------------
// Heuristic helpers for regex matching
// ---------------------------------------------------------------------------

func looksLikeMolecularFormula(s string) bool {
	if len(s) < 2 || len(s) > 50 {
		return false
	}
	hasElement := false
	hasDigit := false
	for _, r := range s {
		if unicode.IsUpper(r) {
			hasElement = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasElement {
		return false
	}
	// Must start with an uppercase letter.
	if !unicode.IsUpper(rune(s[0])) {
		return false
	}
	// Reject if it looks like a normal English word (all letters, no digits).
	if !hasDigit {
		return false
	}
	// Reject common false positives.
	upper := strings.ToUpper(s)
	falsePositives := map[string]bool{"DNA": true, "RNA": true, "ATP": true, "GTP": true, "USA": true}
	if falsePositives[upper] {
		return false
	}
	return true
}

func looksLikeSMILES(s string) bool {
	if len(s) < 5 {
		return false
	}
	// Must contain at least one of: ring digit, branch, double/triple bond, aromatic lowercase.
	indicators := 0
	for _, r := range s {
		switch {
		case r == '(' || r == ')':
			indicators++
		case r == '=' || r == '#':
			indicators++
		case r == '[' || r == ']':
			indicators++
		case r == '/' || r == '\\':
			indicators++
		case r >= '0' && r <= '9':
			indicators++
		case r == 'c' || r == 'n' || r == 'o' || r == 's':
			indicators++
		}
	}
	// Require at least 2 SMILES-specific indicators.
	if indicators < 2 {
		return false
	}
	// Reject if it contains spaces (SMILES never have spaces).
	if strings.Contains(s, " ") {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
// Noop implementations for optional dependencies
// ---------------------------------------------------------------------------

type noopLogger struct{}

func (n *noopLogger) Info(msg string, kv ...interface{})  {}
func (n *noopLogger) Warn(msg string, kv ...interface{})  {}
func (n *noopLogger) Error(msg string, kv ...interface{}) {}
func (n *noopLogger) Debug(msg string, kv ...interface{}) {}

type noopMetrics struct{}

func (n *noopMetrics) RecordExtraction(ctx context.Context, entityCount int, durationMs float64) {}
func (n *noopMetrics) RecordResolution(ctx context.Context, method string, success bool)         {}
func (n *noopMetrics) RecordLinkage(ctx context.Context, exact bool)                             {}

//Personal.AI order the ending

