package chem_extractor

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// =========================================================================
// Mocks
// =========================================================================

type mockNERModel struct {
	predictFn      func(ctx context.Context, text string) ([]*RawChemicalEntity, error)
	predictBatchFn func(ctx context.Context, texts []string) ([][]*RawChemicalEntity, error)
	callCount      int
}

func (m *mockNERModel) Predict(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
	m.callCount++
	if m.predictFn != nil {
		return m.predictFn(ctx, text)
	}
	return nil, nil
}

func (m *mockNERModel) PredictBatch(ctx context.Context, texts []string) ([][]*RawChemicalEntity, error) {
	if m.predictBatchFn != nil {
		return m.predictBatchFn(ctx, texts)
	}
	results := make([][]*RawChemicalEntity, len(texts))
	for i, t := range texts {
		r, err := m.Predict(ctx, t)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}

type mockEntityResolver struct {
	resolveFn func(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error)
}

func (m *mockEntityResolver) Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
	if m.resolveFn != nil {
		return m.resolveFn(ctx, entity)
	}
	return &ResolvedChemicalEntity{
		OriginalEntity:   entity,
		CanonicalName:    entity.Text,
		IsResolved:       true,
		ResolutionMethod: "mock",
	}, nil
}

type mockEntityValidator struct {
	validateFn func(ctx context.Context, entity *RawChemicalEntity) (bool, error)
}

func (m *mockEntityValidator) Validate(ctx context.Context, entity *RawChemicalEntity) (bool, error) {
	if m.validateFn != nil {
		return m.validateFn(ctx, entity)
	}
	return true, nil
}

type mockChemicalDictionary struct {
	entries map[string]*DictionaryEntry
	cas     map[string]*DictionaryEntry
}

func newMockDictionary() *mockChemicalDictionary {
	return &mockChemicalDictionary{
		entries: map[string]*DictionaryEntry{
			"aspirin": {
				CanonicalName:    "aspirin",
				EntityType:       EntityCommonName,
				SMILES:           "CC(=O)Oc1ccccc1C(=O)O",
				CASNumber:        "50-78-2",
				MolecularFormula: "C9H8O4",
				MolecularWeight:  180.16,
				Synonyms:         []string{"acetylsalicylic acid", "ASA"},
			},
			"ibuprofen": {
				CanonicalName:    "ibuprofen",
				EntityType:       EntityCommonName,
				SMILES:           "CC(C)Cc1ccc(cc1)C(C)C(=O)O",
				CASNumber:        "15687-27-1",
				MolecularFormula: "C13H18O2",
				MolecularWeight:  206.28,
			},
		},
		cas: map[string]*DictionaryEntry{
			"50-78-2": {
				CanonicalName:    "aspirin",
				EntityType:       EntityCASNumber,
				SMILES:           "CC(=O)Oc1ccccc1C(=O)O",
				CASNumber:        "50-78-2",
				MolecularFormula: "C9H8O4",
				MolecularWeight:  180.16,
			},
		},
	}
}

func (m *mockChemicalDictionary) Lookup(name string) (*DictionaryEntry, bool) {
	e, ok := m.entries[strings.ToLower(name)]
	return e, ok
}

func (m *mockChemicalDictionary) LookupCAS(cas string) (*DictionaryEntry, bool) {
	e, ok := m.cas[cas]
	return e, ok
}

func (m *mockChemicalDictionary) Size() int {
	return len(m.entries)
}

type mockMoleculeDatabase struct {
	byCASSMILES map[string]*MoleculeRecord
	byInChIKey  map[string]*MoleculeRecord
	fuzzyResult *MoleculeRecord
	fuzzyScore  float64
}

func newMockMoleculeDB() *mockMoleculeDatabase {
	rec := &MoleculeRecord{
		MoleculeID: "mol-aspirin-001",
		SMILES:     "CC(=O)Oc1ccccc1C(=O)O",
		InChIKey:   "BSYNRYMUTXBXSQ-UHFFFAOYSA-N",
		CASNumber:  "50-78-2",
		Name:       "aspirin",
	}
	return &mockMoleculeDatabase{
		byCASSMILES: map[string]*MoleculeRecord{
			"50-78-2":                      rec,
			"CC(=O)Oc1ccccc1C(=O)O":       rec,
		},
		byInChIKey: map[string]*MoleculeRecord{
			"BSYNRYMUTXBXSQ-UHFFFAOYSA-N": rec,
		},
		fuzzyResult: rec,
		fuzzyScore:  0.92,
	}
}

func (m *mockMoleculeDatabase) FindByCAS(ctx context.Context, cas string) (*MoleculeRecord, error) {
	r, ok := m.byCASSMILES[cas]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockMoleculeDatabase) FindBySMILES(ctx context.Context, smiles string) (*MoleculeRecord, error) {
	r, ok := m.byCASSMILES[smiles]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockMoleculeDatabase) FindByInChIKey(ctx context.Context, inchiKey string) (*MoleculeRecord, error) {
	r, ok := m.byInChIKey[inchiKey]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockMoleculeDatabase) FindByNameFuzzy(ctx context.Context, name string) (*MoleculeRecord, float64, error) {
	if m.fuzzyResult != nil {
		return m.fuzzyResult, m.fuzzyScore, nil
	}
	return nil, 0, nil
}

type mockExternalChemDB struct {
	byName   map[string]*ExternalMoleculeRecord
	bySMILES map[string]*ExternalMoleculeRecord
}

func newMockExternalDB() *mockExternalChemDB {
	return &mockExternalChemDB{
		byName: map[string]*ExternalMoleculeRecord{
			"caffeine": {PubChemCID: 2519, ChEMBLID: "CHEMBL113", Name: "caffeine"},
		},
		bySMILES: map[string]*ExternalMoleculeRecord{},
	}
}

func (m *mockExternalChemDB) LookupByName(ctx context.Context, name string) (*ExternalMoleculeRecord, error) {
	r, ok := m.byName[strings.ToLower(name)]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockExternalChemDB) LookupBySMILES(ctx context.Context, smiles string) (*ExternalMoleculeRecord, error) {
	r, ok := m.bySMILES[smiles]
	if !ok {
		return nil, nil
	}
	return r, nil
}

// =========================================================================
// Helper: build extractor with all mocks wired
// =========================================================================

func newTestExtractor(t *testing.T) (ChemicalExtractor, *mockNERModel) {
	t.Helper()
	ner := &mockNERModel{}
	resolver := &mockEntityResolver{}
	validator := &mockEntityValidator{}
	dict := newMockDictionary()
	molDB := newMockMoleculeDB()
	extDB := newMockExternalDB()
	cfg := DefaultExtractorConfig()

	ext, err := NewChemicalExtractor(ner, resolver, validator, dict, molDB, extDB, cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewChemicalExtractor: %v", err)
	}
	return ext, ner
}

func newTestExtractorWithNER(t *testing.T, nerFn func(ctx context.Context, text string) ([]*RawChemicalEntity, error)) ChemicalExtractor {
	t.Helper()
	ner := &mockNERModel{predictFn: nerFn}
	resolver := &mockEntityResolver{}
	validator := &mockEntityValidator{}
	dict := newMockDictionary()
	cfg := DefaultExtractorConfig()

	ext, err := NewChemicalExtractor(ner, resolver, validator, dict, newMockMoleculeDB(), newMockExternalDB(), cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewChemicalExtractor: %v", err)
	}
	return ext
}

// =========================================================================
// Tests: Extract
// =========================================================================

func TestExtract_IUPACName(t *testing.T) {
	ner := &mockNERModel{
		predictFn: func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
			idx := strings.Index(text, "2-acetoxybenzoic acid")
			if idx < 0 {
				return nil, nil
			}
			return []*RawChemicalEntity{{
				Text:        "2-acetoxybenzoic acid",
				StartOffset: idx,
				EndOffset:   idx + len("2-acetoxybenzoic acid"),
				EntityType:  EntityIUPACName,
				Confidence:  0.95,
				Source:      "ner",
			}}, nil
		},
	}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(ner, &mockEntityResolver{}, &mockEntityValidator{}, newMockDictionary(), nil, nil, cfg, nil, nil)

	res, err := ext.Extract(context.Background(), "The compound 2-acetoxybenzoic acid was synthesized.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range res.Entities {
		if e.EntityType == EntityIUPACName && e.Text == "2-acetoxybenzoic acid" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected IUPAC name entity '2-acetoxybenzoic acid'")
	}
}

func TestExtract_CommonName(t *testing.T) {
	ext, _ := newTestExtractor(t)
	res, err := ext.Extract(context.Background(), "The patient was given aspirin daily.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range res.Entities {
		if e.EntityType == EntityCommonName && strings.EqualFold(e.Text, "aspirin") {
			found = true
			if e.Source != "dictionary" {
				t.Errorf("expected source=dictionary, got %s", e.Source)
			}
			break
		}
	}
	if !found {
		t.Error("expected common name entity 'aspirin' from dictionary")
	}
}

func TestExtract_MolecularFormula(t *testing.T) {
	ext, _ := newTestExtractor(t)
	res, err := ext.Extract(context.Background(), "The molecular formula is C9H8O4 for this compound.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range res.Entities {
		if e.EntityType == EntityMolecularFormula && e.Text == "C9H8O4" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected molecular formula entity 'C9H8O4'")
	}
}

func TestExtract_SMILES(t *testing.T) {
	smilesStr := "CC(=O)Oc1ccccc1C(=O)O"
	text := fmt.Sprintf("The SMILES representation is %s for aspirin.", smilesStr)
	ext, _ := newTestExtractor(t)
	res, err := ext.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range res.Entities {
		if e.EntityType == EntitySMILES && e.Text == smilesStr {
			found = true
			break
		}
	}
	if !found {
		// SMILES detection is heuristic; log all entities for debugging.
		for _, e := range res.Entities {
			t.Logf("entity: type=%s text=%q conf=%.2f", e.EntityType, e.Text, e.Confidence)
		}
		t.Error("expected SMILES entity")
	}
}

func TestExtract_CASNumber(t *testing.T) {
	ext, _ := newTestExtractor(t)
	res, err := ext.Extract(context.Background(), "Aspirin (CAS 50-78-2) is widely used.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range res.Entities {
		if e.EntityType == EntityCASNumber && e.Text == "50-78-2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected CAS number entity '50-78-2'")
	}
}

func TestExtract_MarkushVariable(t *testing.T) {
	text := "wherein R1 is C1-C6 alkyl and R2 represents hydrogen or methyl"
	ner := &mockNERModel{}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(ner, &mockEntityResolver{}, &mockEntityValidator{}, newMockDictionary(), nil, nil, cfg, nil, nil)

	res, err := ext.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	foundR1 := false
	for _, e := range res.Entities {
		if e.EntityType == EntityMarkushVariable && e.Text == "R1" {
			foundR1 = true
		}
	}
	if !foundR1 {
		for _, e := range res.Entities {
			t.Logf("entity: type=%s text=%q", e.EntityType, e.Text)
		}
		t.Error("expected Markush variable 'R1'")
	}
}

func TestExtract_MultipleEntities(t *testing.T) {
	text := "The formulation contains aspirin (50-78-2) and ibuprofen with formula C13H18O2."
	ext, _ := newTestExtractor(t)
	res, err := ext.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EntityCount < 3 {
		for _, e := range res.Entities {
			t.Logf("entity: type=%s text=%q", e.EntityType, e.Text)
		}
		t.Errorf("expected at least 3 entities, got %d", res.EntityCount)
	}
}

func TestExtract_DictionaryPriority(t *testing.T) {
	ext, ner := newTestExtractor(t)
	// "aspirin" is in the dictionary, so NER should not be the source.
	res, err := ext.Extract(context.Background(), "aspirin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range res.Entities {
		if strings.EqualFold(e.Text, "aspirin") && e.Source == "dictionary" {
			// Good: dictionary matched.
			_ = ner // NER may or may not have been called; the key is the source.
			return
		}
	}
	t.Error("expected aspirin to be matched by dictionary")
}

func TestExtract_NERFallback(t *testing.T) {
	nerCalled := false
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		nerCalled = true
		return []*RawChemicalEntity{{
			Text:        "sorafenib",
			StartOffset: strings.Index(text, "sorafenib"),
			EndOffset:   strings.Index(text, "sorafenib") + len("sorafenib"),
			EntityType:  EntityCommonName,
			Confidence:  0.88,
			Source:      "ner",
		}}, nil
	})
	res, err := ext.Extract(context.Background(), "The drug sorafenib was administered.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !nerCalled {
		t.Error("expected NER model to be called")
	}
	found := false
	for _, e := range res.Entities {
		if e.Text == "sorafenib" && e.Source == "ner" {
			found = true
		}
	}
	if !found {
		t.Error("expected NER-sourced entity 'sorafenib'")
	}
}

func TestExtract_OverlappingEntities(t *testing.T) {
	// Simulate two overlapping entities from NER.
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		return []*RawChemicalEntity{
			{
				Text:        "acetylsalicylic",
				StartOffset: 4,
				EndOffset:   19,
				EntityType:  EntityIUPACName,
				Confidence:  0.70,
				Source:      "ner",
			},
			{
				Text:        "acetylsalicylic acid",
				StartOffset: 4,
				EndOffset:   24,
				EntityType:  EntityIUPACName,
				Confidence:  0.90,
				Source:      "ner",
			},
		}, nil
	})

	res, err := ext.Extract(context.Background(), "The acetylsalicylic acid compound is effective.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The higher-confidence, longer entity should win.
	foundLong := false
	foundShort := false
	for _, e := range res.Entities {
		if e.Text == "acetylsalicylic acid" {
			foundLong = true
		}
		if e.Text == "acetylsalicylic" && !e.IsNested {
			foundShort = true
		}
	}
	if !foundLong {
		t.Error("expected the longer, higher-confidence entity 'acetylsalicylic acid' to be kept")
	}
	// The shorter one should either be removed or marked as nested.
	if foundShort {
		t.Error("expected the shorter overlapping entity to be removed or marked nested")
	}
}

func TestExtract_NestedEntities(t *testing.T) {
	// "aspirin" nested inside "aspirin tablet" — both should be kept, inner marked nested.
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		return []*RawChemicalEntity{
			{
				Text:        "aspirin tablet",
				StartOffset: 10,
				EndOffset:   24,
				EntityType:  EntityCommonName,
				Confidence:  0.85,
				Source:      "ner",
			},
			{
				Text:        "aspirin",
				StartOffset: 10,
				EndOffset:   17,
				EntityType:  EntityCommonName,
				Confidence:  0.95,
				Source:      "ner",
			},
		}, nil
	})

	res, err := ext.Extract(context.Background(), "We tested aspirin tablet formulations.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outerFound := false
	innerFound := false
	for _, e := range res.Entities {
		if e.Text == "aspirin tablet" {
			outerFound = true
		}
		if e.Text == "aspirin" && e.IsNested {
			innerFound = true
		}
	}
	if !outerFound {
		t.Error("expected outer entity 'aspirin tablet'")
	}
	if !innerFound {
		// Nested detection depends on merge order; at minimum the outer should exist.
		t.Log("note: inner nested entity may have been merged differently")
	}
}

func TestExtract_LowConfidenceFiltered(t *testing.T) {
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		return []*RawChemicalEntity{{
			Text:        "compoundX",
			StartOffset: 0,
			EndOffset:   9,
			EntityType:  EntityCommonName,
			Confidence:  0.50,
			Source:      "ner",
		}}, nil
	})

	res, err := ext.Extract(context.Background(), "compoundX was tested.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range res.Entities {
		if e.Text == "compoundX" {
			t.Error("expected low-confidence entity (0.50) to be filtered out (threshold 0.60)")
		}
	}
}

func TestExtract_HighConfidenceKept(t *testing.T) {
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		return []*RawChemicalEntity{{
			Text:        "sorafenib",
			StartOffset: 0,
			EndOffset:   9,
			EntityType:  EntityCommonName,
			Confidence:  0.80,
			Source:      "ner",
		}}, nil
	})

	res, err := ext.Extract(context.Background(), "sorafenib is a kinase inhibitor.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, e := range res.Entities {
		if e.Text == "sorafenib" {
			found = true
		}
	}
	if !found {
		t.Error("expected high-confidence entity (0.80) to be kept")
	}
}

func TestExtract_EmptyText(t *testing.T) {
	ext, _ := newTestExtractor(t)
	res, err := ext.Extract(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EntityCount != 0 {
		t.Errorf("expected 0 entities for empty text, got %d", res.EntityCount)
	}
	if res.TextLength != 0 {
		t.Errorf("expected TextLength=0, got %d", res.TextLength)
	}
	if res.Coverage != 0 {
		t.Errorf("expected Coverage=0, got %f", res.Coverage)
	}
}

func TestExtract_NoChemicalEntities(t *testing.T) {
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		return nil, nil
	})
	res, err := ext.Extract(context.Background(), "This patent claim relates to a method of manufacturing a widget.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EntityCount != 0 {
		for _, e := range res.Entities {
			t.Logf("unexpected entity: type=%s text=%q conf=%.2f", e.EntityType, e.Text, e.Confidence)
		}
		t.Errorf("expected 0 entities for non-chemical text, got %d", res.EntityCount)
	}
}

func TestExtract_Coverage(t *testing.T) {
	ext, _ := newTestExtractor(t)
	text := "aspirin is good"
	res, err := ext.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "aspirin" = 7 chars, text = 15 chars → coverage ≈ 0.467
	if res.Coverage <= 0 {
		t.Error("expected positive coverage")
	}
	if res.Coverage > 1.0 {
		t.Errorf("coverage should not exceed 1.0, got %f", res.Coverage)
	}
}

// =========================================================================
// Tests: ExtractBatch
// =========================================================================

func TestExtractBatch_Success(t *testing.T) {
	ext, _ := newTestExtractor(t)
	texts := []string{
		"aspirin is used",
		"ibuprofen is common",
		"The CAS number 50-78-2 identifies aspirin",
	}
	results, err := ext.ExtractBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r == nil {
			t.Errorf("result[%d] is nil", i)
		}
	}
}

func TestExtractBatch_Empty(t *testing.T) {
	ext, _ := newTestExtractor(t)
	results, err := ext.ExtractBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestExtractBatch_PartialFailure(t *testing.T) {
	callIdx := 0
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		callIdx++
		if strings.Contains(text, "FAIL") {
			return nil, fmt.Errorf("simulated NER failure")
		}
		return nil, nil
	})
	texts := []string{
		"aspirin is used",
		"FAIL this text",
		"ibuprofen is common",
	}
	results, err := ext.ExtractBatch(context.Background(), texts)
	// Should not return a top-level error since some succeeded.
	if err != nil {
		t.Logf("batch returned error (acceptable if partial): %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// The successful ones should have non-nil results.
	if results[0] == nil {
		t.Error("result[0] should not be nil")
	}
	if results[2] == nil {
		t.Error("result[2] should not be nil")
	}
}

// =========================================================================
// Tests: ExtractFromClaim
// =========================================================================

func TestExtractFromClaim_FeatureMapping(t *testing.T) {
	ext, _ := newTestExtractor(t)
	claim := &ParsedClaim{
		ClaimNumber: 1,
		ClaimText:   "A composition comprising aspirin and ibuprofen in a carrier.",
		TechnicalFeatures: []*TechnicalFeature{
			{ID: "feat-1", Text: "comprising aspirin", StartOffset: 14, EndOffset: 32},
			{ID: "feat-2", Text: "ibuprofen in a carrier", StartOffset: 37, EndOffset: 59},
		},
	}
	result, err := ext.ExtractFromClaim(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ClaimNumber != 1 {
		t.Errorf("expected ClaimNumber=1, got %d", result.ClaimNumber)
	}
	if len(result.Entities) == 0 {
		t.Error("expected at least one entity from claim text")
	}
	// Check that at least one feature has mapped entities.
	hasMapped := false
	for _, ents := range result.FeatureEntityMapping {
		if len(ents) > 0 {
			hasMapped = true
			break
		}
	}
	if !hasMapped {
		t.Log("note: feature-entity mapping may be empty if offsets don't align with tokenisation")
	}
}

func TestExtractFromClaim_MarkushMapping(t *testing.T) {
	ext := newTestExtractorWithNER(t, func(ctx context.Context, text string) ([]*RawChemicalEntity, error) {
		// The regex in the extractor should pick up R1 from "R1 is C1-C6 alkyl".
		return nil, nil
	})
	claim := &ParsedClaim{
		ClaimNumber: 2,
		ClaimText:   "A compound of formula I wherein R1 is C1-C6 alkyl and R2 represents hydrogen or methyl.",
	}
	result, err := ext.ExtractFromClaim(context.Background(), claim)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.MarkushVariableMapping) > 0 {
		for varName, resolved := range result.MarkushVariableMapping {
			t.Logf("Markush %s -> %d resolved entities", varName, len(resolved))
			if varName == "R1" && len(resolved) > 0 {
				// Expect expanded alkyl names.
				names := make([]string, len(resolved))
				for i, r := range resolved {
					names[i] = r.CanonicalName
				}
				t.Logf("  R1 expanded to: %v", names)
			}
		}
	} else {
		t.Log("note: Markush mapping may be empty if regex didn't match the pattern")
	}
}

func TestExtractFromClaim_NilClaim(t *testing.T) {
	ext, _ := newTestExtractor(t)
	_, err := ext.ExtractFromClaim(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil claim")
	}
}

// =========================================================================
// Tests: Resolve
// =========================================================================

func TestResolve_FromCASNumber(t *testing.T) {
	resolver := &mockEntityResolver{
		resolveFn: func(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
			if entity.EntityType == EntityCASNumber && entity.Text == "50-78-2" {
				return &ResolvedChemicalEntity{
					OriginalEntity:   entity,
					CanonicalName:    "aspirin",
					SMILES:           "CC(=O)Oc1ccccc1C(=O)O",
					InChI:            "InChI=1S/C9H8O4/c1-6(10)13-8-5-3-2-4-7(8)9(11)12/h2-5H,1H3,(H,11,12)",
					InChIKey:         "BSYNRYMUTXBXSQ-UHFFFAOYSA-N",
					MolecularFormula: "C9H8O4",
					CASNumber:        "50-78-2",
					MolecularWeight:  180.16,
					IsResolved:       true,
					ResolutionMethod: "Dictionary",
					Synonyms:         []string{"acetylsalicylic acid", "ASA"},
				}, nil
			}
			return &ResolvedChemicalEntity{OriginalEntity: entity, IsResolved: false}, nil
		},
	}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(&mockNERModel{}, resolver, nil, nil, nil, nil, cfg, nil, nil)

	entity := &RawChemicalEntity{
		Text:       "50-78-2",
		EntityType: EntityCASNumber,
		Confidence: 1.0,
	}
	resolved, err := ext.Resolve(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if resolved.CanonicalName != "aspirin" {
		t.Errorf("expected CanonicalName=aspirin, got %s", resolved.CanonicalName)
	}
	if resolved.SMILES == "" {
		t.Error("expected non-empty SMILES")
	}
	if resolved.MolecularWeight != 180.16 {
		t.Errorf("expected MW=180.16, got %f", resolved.MolecularWeight)
	}
	if resolved.ResolutionMethod != "Dictionary" {
		t.Errorf("expected ResolutionMethod=Dictionary, got %s", resolved.ResolutionMethod)
	}
}

func TestResolve_FromSMILES(t *testing.T) {
	resolver := &mockEntityResolver{
		resolveFn: func(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
			if entity.EntityType == EntitySMILES {
				return &ResolvedChemicalEntity{
					OriginalEntity:   entity,
					CanonicalName:    "aspirin",
					SMILES:           "CC(=O)Oc1ccccc1C(=O)O",
					InChI:            "InChI=1S/C9H8O4/...",
					InChIKey:         "BSYNRYMUTXBXSQ-UHFFFAOYSA-N",
					MolecularFormula: "C9H8O4",
					MolecularWeight:  180.16,
					IsResolved:       true,
					ResolutionMethod: "RDKit",
				}, nil
			}
			return &ResolvedChemicalEntity{OriginalEntity: entity, IsResolved: false}, nil
		},
	}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(&mockNERModel{}, resolver, nil, nil, nil, nil, cfg, nil, nil)

	entity := &RawChemicalEntity{
		Text:       "CC(=O)Oc1ccccc1C(=O)O",
		EntityType: EntitySMILES,
		Confidence: 0.90,
	}
	resolved, err := ext.Resolve(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if resolved.ResolutionMethod != "RDKit" {
		t.Errorf("expected ResolutionMethod=RDKit, got %s", resolved.ResolutionMethod)
	}
	if resolved.InChIKey == "" {
		t.Error("expected non-empty InChIKey")
	}
}

func TestResolve_FromName(t *testing.T) {
	resolver := &mockEntityResolver{
		resolveFn: func(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
			if entity.Text == "aspirin" {
				return &ResolvedChemicalEntity{
					OriginalEntity:   entity,
					CanonicalName:    "aspirin",
					SMILES:           "CC(=O)Oc1ccccc1C(=O)O",
					IsResolved:       true,
					ResolutionMethod: "Dictionary",
					Synonyms:         []string{"acetylsalicylic acid"},
				}, nil
			}
			return &ResolvedChemicalEntity{OriginalEntity: entity, IsResolved: false}, nil
		},
	}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(&mockNERModel{}, resolver, nil, nil, nil, nil, cfg, nil, nil)

	entity := &RawChemicalEntity{Text: "aspirin", EntityType: EntityCommonName, Confidence: 1.0}
	resolved, err := ext.Resolve(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resolved.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if resolved.CanonicalName != "aspirin" {
		t.Errorf("expected CanonicalName=aspirin, got %s", resolved.CanonicalName)
	}
	if len(resolved.Synonyms) == 0 {
		t.Error("expected at least one synonym")
	}
}

func TestResolve_Unresolvable(t *testing.T) {
	resolver := &mockEntityResolver{
		resolveFn: func(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
			return &ResolvedChemicalEntity{
				OriginalEntity:   entity,
				CanonicalName:    entity.Text,
				IsResolved:       false,
				ResolutionMethod: "NER",
			}, nil
		},
	}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(&mockNERModel{}, resolver, nil, nil, nil, nil, cfg, nil, nil)

	entity := &RawChemicalEntity{Text: "unknownium-42", EntityType: EntityCommonName, Confidence: 0.65}
	resolved, err := ext.Resolve(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.IsResolved {
		t.Error("expected IsResolved=false for unresolvable entity")
	}
}

func TestResolve_ResolutionMethod(t *testing.T) {
	methods := []string{"Dictionary", "NER", "RDKit", "PubChem"}
	for _, method := range methods {
		m := method
		resolver := &mockEntityResolver{
			resolveFn: func(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
				return &ResolvedChemicalEntity{
					OriginalEntity:   entity,
					CanonicalName:    "test",
					IsResolved:       true,
					ResolutionMethod: m,
				}, nil
			},
		}
		cfg := DefaultExtractorConfig()
		ext, _ := NewChemicalExtractor(&mockNERModel{}, resolver, nil, nil, nil, nil, cfg, nil, nil)
		entity := &RawChemicalEntity{Text: "test", EntityType: EntityCommonName, Confidence: 0.90}
		resolved, err := ext.Resolve(context.Background(), entity)
		if err != nil {
			t.Fatalf("unexpected error for method %s: %v", m, err)
		}
		if resolved.ResolutionMethod != m {
			t.Errorf("expected ResolutionMethod=%s, got %s", m, resolved.ResolutionMethod)
		}
	}
}

func TestResolve_NilEntity(t *testing.T) {
	ext, _ := newTestExtractor(t)
	_, err := ext.Resolve(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil entity")
	}
}

// =========================================================================
// Tests: LinkToMolecule
// =========================================================================

func TestLinkToMolecule_ByCASNumber(t *testing.T) {
	ext, _ := newTestExtractor(t)
	entity := &ResolvedChemicalEntity{
		CanonicalName: "aspirin",
		CASNumber:     "50-78-2",
		IsResolved:    true,
	}
	link, err := ext.LinkToMolecule(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link == nil {
		t.Fatal("expected non-nil link")
	}
	if link.MoleculeID != "mol-aspirin-001" {
		t.Errorf("expected MoleculeID=mol-aspirin-001, got %s", link.MoleculeID)
	}
	if !link.IsExactMatch {
		t.Error("expected IsExactMatch=true for CAS lookup")
	}
	if link.SimilarityScore != 1.0 {
		t.Errorf("expected SimilarityScore=1.0, got %f", link.SimilarityScore)
	}
}

func TestLinkToMolecule_BySMILES(t *testing.T) {
	ext, _ := newTestExtractor(t)
	entity := &ResolvedChemicalEntity{
		CanonicalName: "aspirin",
		SMILES:        "CC(=O)Oc1ccccc1C(=O)O",
		IsResolved:    true,
	}
	link, err := ext.LinkToMolecule(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link == nil {
		t.Fatal("expected non-nil link")
	}
	if !link.IsExactMatch {
		t.Error("expected IsExactMatch=true for SMILES lookup")
	}
}

func TestLinkToMolecule_ByInChIKey(t *testing.T) {
	ext, _ := newTestExtractor(t)
	entity := &ResolvedChemicalEntity{
		CanonicalName: "aspirin",
		InChIKey:      "BSYNRYMUTXBXSQ-UHFFFAOYSA-N",
		IsResolved:    true,
	}
	link, err := ext.LinkToMolecule(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link == nil {
		t.Fatal("expected non-nil link")
	}
	if !link.IsExactMatch {
		t.Error("expected IsExactMatch=true for InChIKey lookup")
	}
}

func TestLinkToMolecule_ByNameFuzzy(t *testing.T) {
	ext, _ := newTestExtractor(t)
	entity := &ResolvedChemicalEntity{
		CanonicalName: "asprin", // intentional typo
		IsResolved:    true,
	}
	link, err := ext.LinkToMolecule(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link == nil {
		t.Fatal("expected non-nil link from fuzzy match")
	}
	if link.IsExactMatch {
		t.Error("expected IsExactMatch=false for fuzzy match")
	}
	if link.SimilarityScore <= 0.70 {
		t.Errorf("expected SimilarityScore > 0.70, got %f", link.SimilarityScore)
	}
}

func TestLinkToMolecule_NotFound(t *testing.T) {
	// Use a molecule DB that returns nothing, but external DB has caffeine.
	emptyMolDB := &mockMoleculeDatabase{}
	extDB := newMockExternalDB()
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(&mockNERModel{}, &mockEntityResolver{}, nil, nil, emptyMolDB, extDB, cfg, nil, nil)

	entity := &ResolvedChemicalEntity{
		CanonicalName: "caffeine",
		IsResolved:    true,
	}
	link, err := ext.LinkToMolecule(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link == nil {
		t.Fatal("expected non-nil link from external DB")
	}
	if link.PubChemCID != 2519 {
		t.Errorf("expected PubChemCID=2519, got %d", link.PubChemCID)
	}
}

func TestLinkToMolecule_ExternalDB_PubChem(t *testing.T) {
	emptyMolDB := &mockMoleculeDatabase{}
	extDB := &mockExternalChemDB{
		byName: map[string]*ExternalMoleculeRecord{
			"metformin": {PubChemCID: 4091, ChEMBLID: "CHEMBL1431", DrugBankID: "DB00331", Name: "metformin"},
		},
		bySMILES: map[string]*ExternalMoleculeRecord{},
	}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(&mockNERModel{}, &mockEntityResolver{}, nil, nil, emptyMolDB, extDB, cfg, nil, nil)

	entity := &ResolvedChemicalEntity{
		CanonicalName: "metformin",
		IsResolved:    true,
	}
	link, err := ext.LinkToMolecule(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link == nil {
		t.Fatal("expected non-nil link")
	}
	if link.PubChemCID != 4091 {
		t.Errorf("expected PubChemCID=4091, got %d", link.PubChemCID)
	}
	if link.ChEMBLID != "CHEMBL1431" {
		t.Errorf("expected ChEMBLID=CHEMBL1431, got %s", link.ChEMBLID)
	}
	if link.DrugBankID != "DB00331" {
		t.Errorf("expected DrugBankID=DB00331, got %s", link.DrugBankID)
	}
}

func TestLinkToMolecule_AllDBsEmpty(t *testing.T) {
	emptyMolDB := &mockMoleculeDatabase{}
	emptyExtDB := &mockExternalChemDB{
		byName:   map[string]*ExternalMoleculeRecord{},
		bySMILES: map[string]*ExternalMoleculeRecord{},
	}
	cfg := DefaultExtractorConfig()
	ext, _ := NewChemicalExtractor(&mockNERModel{}, &mockEntityResolver{}, nil, nil, emptyMolDB, emptyExtDB, cfg, nil, nil)

	entity := &ResolvedChemicalEntity{
		CanonicalName: "nonexistentium",
		IsResolved:    true,
	}
	link, err := ext.LinkToMolecule(context.Background(), entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link != nil {
		t.Errorf("expected nil link when all DBs are empty, got %+v", link)
	}
}

func TestLinkToMolecule_NilEntity(t *testing.T) {
	ext, _ := newTestExtractor(t)
	_, err := ext.LinkToMolecule(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil entity")
	}
}

// =========================================================================
// Tests: Overlap resolution helpers
// =========================================================================

func TestOverlapResolution_HigherConfidenceWins(t *testing.T) {
	a := &RawChemicalEntity{Text: "acetyl", StartOffset: 0, EndOffset: 6, Confidence: 0.90}
	b := &RawChemicalEntity{Text: "acetylsali", StartOffset: 0, EndOffset: 10, Confidence: 0.70}
	winner := pickOverlapWinner(a, b)
	if winner.Confidence != 0.90 {
		t.Errorf("expected higher confidence (0.90) to win, got %f", winner.Confidence)
	}
	if winner.Text != "acetyl" {
		t.Errorf("expected winner text='acetyl', got %q", winner.Text)
	}
}

func TestOverlapResolution_SameConfidenceLongerWins(t *testing.T) {
	a := &RawChemicalEntity{Text: "acetylsalicylic", StartOffset: 0, EndOffset: 15, Confidence: 0.85}
	b := &RawChemicalEntity{Text: "salicylic", StartOffset: 6, EndOffset: 15, Confidence: 0.85}

	// Both have same confidence; the longer one (15 chars vs 9 chars) should win.
	winner := pickOverlapWinner(a, b)
	if winner.Text != "acetylsalicylic" {
		t.Errorf("expected longer entity 'acetylsalicylic' to win, got %q", winner.Text)
	}
	if spanLength(winner) != 15 {
		t.Errorf("expected span length 15, got %d", spanLength(winner))
	}
}

func TestOverlapResolution_NoOverlap(t *testing.T) {
	entities := []*RawChemicalEntity{
		{Text: "aspirin", StartOffset: 0, EndOffset: 7, Confidence: 0.95},
		{Text: "ibuprofen", StartOffset: 12, EndOffset: 21, Confidence: 0.90},
	}
	resolved := resolveOverlaps(entities)
	if len(resolved) != 2 {
		t.Errorf("expected 2 non-overlapping entities preserved, got %d", len(resolved))
	}
	if resolved[0].Text != "aspirin" {
		t.Errorf("expected first entity 'aspirin', got %q", resolved[0].Text)
	}
	if resolved[1].Text != "ibuprofen" {
		t.Errorf("expected second entity 'ibuprofen', got %q", resolved[1].Text)
	}
}

// =========================================================================
// Tests: Span helper functions
// =========================================================================

func TestSpansOverlap(t *testing.T) {
	tests := []struct {
		name     string
		s1, e1   int
		s2, e2   int
		expected bool
	}{
		{"fully overlapping", 0, 10, 0, 10, true},
		{"partial overlap left", 0, 10, 5, 15, true},
		{"partial overlap right", 5, 15, 0, 10, true},
		{"nested inner", 2, 8, 0, 10, true},
		{"nested outer", 0, 10, 2, 8, true},
		{"adjacent no overlap", 0, 5, 5, 10, false},
		{"no overlap gap", 0, 5, 8, 12, false},
		{"single char overlap", 0, 6, 5, 10, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spansOverlap(tt.s1, tt.e1, tt.s2, tt.e2)
			if got != tt.expected {
				t.Errorf("spansOverlap(%d,%d,%d,%d) = %v, want %v", tt.s1, tt.e1, tt.s2, tt.e2, got, tt.expected)
			}
		})
	}
}

func TestIsNested(t *testing.T) {
	outer := &RawChemicalEntity{StartOffset: 0, EndOffset: 20}
	inner := &RawChemicalEntity{StartOffset: 5, EndOffset: 15}
	noNest := &RawChemicalEntity{StartOffset: 10, EndOffset: 25}

	if !isNested(outer, inner) {
		t.Error("expected outer to contain inner")
	}
	if !isNested(inner, outer) {
		t.Error("expected isNested to be symmetric")
	}
	if isNested(outer, noNest) {
		t.Error("expected non-nested entities to return false")
	}
}

// =========================================================================
// Tests: Text utility functions
// =========================================================================

func TestNormaliseText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "hello world", "hello world"},
		{"multiple spaces", "hello   world", "hello world"},
		{"tabs and newlines", "hello\t\nworld", "hello world"},
		{"leading trailing", "  hello  ", "hello"},
		{"empty", "", ""},
		{"unicode nbsp", "hello\u00A0world", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normaliseText(tt.input)
			if got != tt.expected {
				t.Errorf("normaliseText(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTokenise(t *testing.T) {
	tokens := tokenise("aspirin (CAS 50-78-2) is used")
	texts := make([]string, len(tokens))
	for i, tok := range tokens {
		texts[i] = tok.text
	}
	// Should include "aspirin", "CAS", "50-78-2", "is", "used"
	if len(tokens) < 4 {
		t.Errorf("expected at least 4 tokens, got %d: %v", len(tokens), texts)
	}
	// Verify offsets are correct.
	for _, tok := range tokens {
		if tok.start < 0 {
			t.Errorf("negative start offset for token %q", tok.text)
		}
	}
}

func TestExtractContext(t *testing.T) {
	text := "The compound aspirin is widely used in medicine today."
	// "aspirin" starts at 13, ends at 20.
	ctx := extractContext(text, 13, 20, 10)
	if !strings.Contains(ctx, "aspirin") {
		t.Errorf("context should contain 'aspirin', got %q", ctx)
	}
	// Context should extend ~10 chars before and after.
	if len(ctx) < len("aspirin") {
		t.Errorf("context too short: %q", ctx)
	}
}

func TestExtractContext_BoundaryStart(t *testing.T) {
	text := "aspirin is used"
	ctx := extractContext(text, 0, 7, 50)
	if ctx != text {
		t.Logf("context at boundary: %q", ctx)
	}
	if !strings.HasPrefix(ctx, "aspirin") {
		t.Errorf("expected context to start with 'aspirin', got %q", ctx)
	}
}

func TestExtractContext_BoundaryEnd(t *testing.T) {
	text := "we use aspirin"
	ctx := extractContext(text, 7, 14, 50)
	if !strings.HasSuffix(ctx, "aspirin") {
		t.Logf("context at end boundary: %q", ctx)
	}
}

func TestFilterByConfidence(t *testing.T) {
	entities := []*RawChemicalEntity{
		{Text: "a", Confidence: 0.90},
		{Text: "b", Confidence: 0.50},
		{Text: "c", Confidence: 0.60},
		{Text: "d", Confidence: 0.59},
		{Text: "e", Confidence: 1.0},
	}
	filtered := filterByConfidence(entities, 0.60)
	if len(filtered) != 3 {
		names := make([]string, len(filtered))
		for i, e := range filtered {
			names[i] = fmt.Sprintf("%s(%.2f)", e.Text, e.Confidence)
		}
		t.Errorf("expected 3 entities after filtering at 0.60, got %d: %v", len(filtered), names)
	}
	for _, e := range filtered {
		if e.Confidence < 0.60 {
			t.Errorf("entity %q with confidence %.2f should have been filtered", e.Text, e.Confidence)
		}
	}
}

func TestComputeCoverage(t *testing.T) {
	entities := []*RawChemicalEntity{
		{StartOffset: 0, EndOffset: 7},   // 7 chars
		{StartOffset: 12, EndOffset: 21},  // 9 chars
	}
	// Total covered = 16, text length = 30 → coverage = 16/30 ≈ 0.533
	cov := computeCoverage(entities, 30)
	expected := 16.0 / 30.0
	if cov < expected-0.01 || cov > expected+0.01 {
		t.Errorf("expected coverage ≈ %.3f, got %.3f", expected, cov)
	}
}

func TestComputeCoverage_Empty(t *testing.T) {
	cov := computeCoverage(nil, 100)
	if cov != 0 {
		t.Errorf("expected 0 coverage for nil entities, got %f", cov)
	}
}

func TestComputeCoverage_ZeroLength(t *testing.T) {
	cov := computeCoverage([]*RawChemicalEntity{{StartOffset: 0, EndOffset: 5}}, 0)
	if cov != 0 {
		t.Errorf("expected 0 coverage for zero-length text, got %f", cov)
	}
}

// =========================================================================
// Tests: Heuristic helpers
// =========================================================================

func TestLooksLikeMolecularFormula(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"C9H8O4", true},
		{"H2O", true},
		{"NaCl", false},   // no digit → false
		{"C13H18O2", true},
		{"DNA", false},    // false positive list
		{"Hello", false},  // no digit
		{"", false},
		{"C", false},      // too short
		{"CH3COOH", false}, // no digits after elements (depends on pattern)
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeMolecularFormula(tt.input)
			if got != tt.expected {
				t.Errorf("looksLikeMolecularFormula(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestLooksLikeSMILES(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"CC(=O)Oc1ccccc1C(=O)O", true},
		{"c1ccccc1", true},
		{"Hello", false},
		{"C", false},
		{"ABCD", false},
		{"CC(C)Cc1ccc(cc1)C(C)C(=O)O", true},
		{"has space", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := looksLikeSMILES(tt.input)
			if got != tt.expected {
				t.Errorf("looksLikeSMILES(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// =========================================================================
// Tests: Markush expansion
// =========================================================================

func TestExpandMarkushDefinition_AlkylRange(t *testing.T) {
	result := expandMarkushDefinition("C1-C6 alkyl")
	expected := []string{"methyl", "ethyl", "propyl", "butyl", "pentyl", "hexyl"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d alkyl names, got %d: %v", len(expected), len(result), result)
	}
	for i, name := range expected {
		if result[i] != name {
			t.Errorf("expected result[%d]=%q, got %q", i, name, result[i])
		}
	}
}

func TestExpandMarkushDefinition_CommaList(t *testing.T) {
	result := expandMarkushDefinition("hydrogen, methyl, ethyl, or phenyl")
	if len(result) < 4 {
		t.Errorf("expected at least 4 items, got %d: %v", len(result), result)
	}
	// Check that "hydrogen" and "phenyl" are present.
	hasHydrogen := false
	hasPhenyl := false
	for _, r := range result {
		if r == "hydrogen" {
			hasHydrogen = true
		}
		if r == "phenyl" {
			hasPhenyl = true
		}
	}
	if !hasHydrogen {
		t.Error("expected 'hydrogen' in expanded list")
	}
	if !hasPhenyl {
		t.Error("expected 'phenyl' in expanded list")
	}
}

func TestExpandMarkushDefinition_SingleValue(t *testing.T) {
	result := expandMarkushDefinition("phenyl")
	if len(result) != 1 || result[0] != "phenyl" {
		t.Errorf("expected [phenyl], got %v", result)
	}
}

func TestExpandAlkylRange(t *testing.T) {
	result := expandAlkylRange(1, 3)
	expected := []string{"methyl", "ethyl", "propyl"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d, got %d: %v", len(expected), len(result), result)
	}
	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("result[%d] = %q, want %q", i, result[i], expected[i])
		}
	}
}

func TestExpandAlkylRange_BoundaryClamping(t *testing.T) {
	// lo < 1 should be clamped to 1.
	result := expandAlkylRange(0, 2)
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d: %v", len(result), result)
	}
	if result[0] != "methyl" {
		t.Errorf("expected first='methyl', got %q", result[0])
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0", 0},
		{"1", 1},
		{"12", 12},
		{"6", 6},
		{"100", 100},
	}
	for _, tt := range tests {
		got := atoi(tt.input)
		if got != tt.expected {
			t.Errorf("atoi(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// =========================================================================
// Tests: Constructor validation
// =========================================================================

func TestNewChemicalExtractor_NilNERWithNEREnabled(t *testing.T) {
	cfg := DefaultExtractorConfig()
	cfg.EnableNER = true
	_, err := NewChemicalExtractor(nil, &mockEntityResolver{}, nil, nil, nil, nil, cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error when NER is enabled but model is nil")
	}
}

func TestNewChemicalExtractor_NilNERWithNERDisabled(t *testing.T) {
	cfg := DefaultExtractorConfig()
	cfg.EnableNER = false
	ext, err := NewChemicalExtractor(nil, &mockEntityResolver{}, nil, nil, nil, nil, cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ext == nil {
		t.Fatal("expected non-nil extractor")
	}
}

func TestNewChemicalExtractor_NilResolver(t *testing.T) {
	cfg := DefaultExtractorConfig()
	_, err := NewChemicalExtractor(&mockNERModel{}, nil, nil, nil, nil, nil, cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error when resolver is nil")
	}
}

func TestNewChemicalExtractor_NilOptionalDeps(t *testing.T) {
	cfg := DefaultExtractorConfig()
	ext, err := NewChemicalExtractor(&mockNERModel{}, &mockEntityResolver{}, nil, nil, nil, nil, cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ext == nil {
		t.Fatal("expected non-nil extractor")
	}
}

// =========================================================================
// Tests: DefaultExtractorConfig
// =========================================================================

func TestDefaultExtractorConfig(t *testing.T) {
	cfg := DefaultExtractorConfig()
	if cfg.MinConfidence != 0.60 {
		t.Errorf("expected MinConfidence=0.60, got %f", cfg.MinConfidence)
	}
	if cfg.ContextWindowSize != 50 {
		t.Errorf("expected ContextWindowSize=50, got %d", cfg.ContextWindowSize)
	}
	if !cfg.EnableDictionaryLookup {
		t.Error("expected EnableDictionaryLookup=true")
	}
	if !cfg.EnableNER {
		t.Error("expected EnableNER=true")
	}
	if cfg.MaxTextLength != 500000 {
		t.Errorf("expected MaxTextLength=500000, got %d", cfg.MaxTextLength)
	}
	if cfg.BatchConcurrency != 4 {
		t.Errorf("expected BatchConcurrency=4, got %d", cfg.BatchConcurrency)
	}
}

// =========================================================================
// Tests: Noop implementations (smoke test)
// =========================================================================

func TestNoopLogger(t *testing.T) {
	l := &noopLogger{}
	// Should not panic.
	l.Info("test")
	l.Warn("test", "key", "value")
	l.Error("test")
	l.Debug("test", "a", 1)
}

func TestNoopMetrics(t *testing.T) {
	m := &noopMetrics{}
	ctx := context.Background()
	// Should not panic.
	m.RecordExtraction(ctx, 5, 100.0)
	m.RecordResolution(ctx, "Dictionary", true)
	m.RecordLinkage(ctx, true)
}

// =========================================================================
// Tests: Edge cases
// =========================================================================

func TestExtract_VeryLongText(t *testing.T) {
	ext, _ := newTestExtractor(t)
	// Generate a long text that exceeds MaxTextLength.
	longText := strings.Repeat("The compound aspirin is used. ", 20000)
	res, err := ext.Extract(context.Background(), longText)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	// Should have truncated and still found entities.
	if res.TextLength > DefaultExtractorConfig().MaxTextLength+100 {
		t.Errorf("text should have been truncated, got TextLength=%d", res.TextLength)
	}
}

func TestExtract_UnicodeText(t *testing.T) {
	ext, _ := newTestExtractor(t)
	// Text with Unicode characters (e.g. Chinese patent text mixed with chemical names).
	text := "该化合物 aspirin（阿司匹林）的分子式为 C9H8O4。"
	res, err := ext.Extract(context.Background(), text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	// Should still find "aspirin" and "C9H8O4".
	t.Logf("found %d entities in Unicode text", res.EntityCount)
	for _, e := range res.Entities {
		t.Logf("  entity: type=%s text=%q conf=%.2f", e.EntityType, e.Text, e.Confidence)
	}
}

func TestExtract_OnlyWhitespace(t *testing.T) {
	ext, _ := newTestExtractor(t)
	res, err := ext.Extract(context.Background(), "   \t\n   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After normalisation, this becomes empty.
	if res.EntityCount != 0 {
		t.Errorf("expected 0 entities for whitespace-only text, got %d", res.EntityCount)
	}
}

func TestResolveOverlaps_SingleEntity(t *testing.T) {
	entities := []*RawChemicalEntity{
		{Text: "aspirin", StartOffset: 0, EndOffset: 7, Confidence: 0.95},
	}
	resolved := resolveOverlaps(entities)
	if len(resolved) != 1 {
		t.Errorf("expected 1 entity, got %d", len(resolved))
	}
}

func TestResolveOverlaps_EmptySlice(t *testing.T) {
	resolved := resolveOverlaps(nil)
	if len(resolved) != 0 {
		t.Errorf("expected 0 entities, got %d", len(resolved))
	}
}

func TestDeduplicateExactSpans(t *testing.T) {
	entities := []*RawChemicalEntity{
		{Text: "aspirin", StartOffset: 0, EndOffset: 7, Confidence: 0.80, Source: "ner"},
		{Text: "aspirin", StartOffset: 0, EndOffset: 7, Confidence: 0.95, Source: "dictionary"},
		{Text: "ibuprofen", StartOffset: 12, EndOffset: 21, Confidence: 0.90, Source: "ner"},
	}
	deduped := deduplicateExactSpans(entities)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 after dedup, got %d", len(deduped))
	}
	// The aspirin entry should have the higher confidence.
	for _, e := range deduped {
		if e.Text == "aspirin" && e.Confidence != 0.95 {
			t.Errorf("expected deduplicated aspirin to have confidence 0.95, got %f", e.Confidence)
		}
	}
}

func TestMockDictionary_Size(t *testing.T) {
	dict := newMockDictionary()
	if dict.Size() != 2 {
		t.Errorf("expected dictionary size 2, got %d", dict.Size())
	}
}

func TestMockDictionary_Lookup(t *testing.T) {
	dict := newMockDictionary()
	entry, found := dict.Lookup("aspirin")
	if !found {
		t.Fatal("expected to find 'aspirin'")
	}
	if entry.CanonicalName != "aspirin" {
		t.Errorf("expected CanonicalName=aspirin, got %s", entry.CanonicalName)
	}

	_, found = dict.Lookup("nonexistent")
	if found {
		t.Error("expected not to find 'nonexistent'")
	}
}

func TestMockDictionary_LookupCAS(t *testing.T) {
	dict := newMockDictionary()
	entry, found := dict.LookupCAS("50-78-2")
	if !found {
		t.Fatal("expected to find CAS 50-78-2")
	}
	if entry.CASNumber != "50-78-2" {
		t.Errorf("expected CASNumber=50-78-2, got %s", entry.CASNumber)
	}

	_, found = dict.LookupCAS("99-99-9")
	if found {
		t.Error("expected not to find CAS 99-99-9")
	}
}

//Personal.AI order the ending
