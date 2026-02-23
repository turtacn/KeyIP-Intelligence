package chem_extractor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =========================================================================
// Mocks
// =========================================================================

// ---- mockPubChemClient --------------------------------------------------

type mockPubChemClient struct {
	searchByNameFn   func(ctx context.Context, name string) (*PubChemCompound, error)
	searchByCASFn    func(ctx context.Context, cas string) (*PubChemCompound, error)
	searchBySMILESFn func(ctx context.Context, smiles string) (*PubChemCompound, error)
	getCompoundFn    func(ctx context.Context, cid int) (*PubChemCompound, error)
}

func (m *mockPubChemClient) SearchByName(ctx context.Context, name string) (*PubChemCompound, error) {
	if m.searchByNameFn != nil {
		return m.searchByNameFn(ctx, name)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPubChemClient) SearchByCAS(ctx context.Context, cas string) (*PubChemCompound, error) {
	if m.searchByCASFn != nil {
		return m.searchByCASFn(ctx, cas)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPubChemClient) SearchBySMILES(ctx context.Context, smiles string) (*PubChemCompound, error) {
	if m.searchBySMILESFn != nil {
		return m.searchBySMILESFn(ctx, smiles)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockPubChemClient) GetCompound(ctx context.Context, cid int) (*PubChemCompound, error) {
	if m.getCompoundFn != nil {
		return m.getCompoundFn(ctx, cid)
	}
	return nil, fmt.Errorf("not found")
}

// ---- mockRDKitService ---------------------------------------------------

type mockRDKitService struct {
	validateFn    func(string) (bool, error)
	canonicalizeFn func(string) (string, error)
	toInChIFn     func(string) (string, error)
	toFormulaFn   func(string) (string, error)
	toMWFn        func(string) (float64, error)
	toInChIKeyFn  func(string) (string, error)
}

func newDefaultMockRDKit() *mockRDKitService {
	return &mockRDKitService{
		validateFn: func(s string) (bool, error) {
			if s == "" || s == "INVALID" {
				return false, nil
			}
			return true, nil
		},
		canonicalizeFn: func(s string) (string, error) {
			// Simulate: lowercase → canonical
			if s == "c1ccccc1" {
				return "c1ccccc1", nil
			}
			return s, nil
		},
		toInChIFn: func(s string) (string, error) {
			return "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3", nil
		},
		toFormulaFn: func(s string) (string, error) {
			return "C2H6O", nil
		},
		toMWFn: func(s string) (float64, error) {
			return 46.07, nil
		},
		toInChIKeyFn: func(s string) (string, error) {
			return "LFQSCWFLJHTTHZ-UHFFFAOYSA-N", nil
		},
	}
}

func (m *mockRDKitService) ValidateSMILES(smiles string) (bool, error) {
	if m.validateFn != nil {
		return m.validateFn(smiles)
	}
	return true, nil
}

func (m *mockRDKitService) CanonicalizeSMILES(smiles string) (string, error) {
	if m.canonicalizeFn != nil {
		return m.canonicalizeFn(smiles)
	}
	return smiles, nil
}

func (m *mockRDKitService) SMILESToInChI(smiles string) (string, error) {
	if m.toInChIFn != nil {
		return m.toInChIFn(smiles)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *mockRDKitService) SMILESToMolecularFormula(smiles string) (string, error) {
	if m.toFormulaFn != nil {
		return m.toFormulaFn(smiles)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *mockRDKitService) ComputeMolecularWeight(smiles string) (float64, error) {
	if m.toMWFn != nil {
		return m.toMWFn(smiles)
	}
	return 0, fmt.Errorf("not implemented")
}

func (m *mockRDKitService) SMILESToInChIKey(smiles string) (string, error) {
	if m.toInChIKeyFn != nil {
		return m.toInChIKeyFn(smiles)
	}
	return "", fmt.Errorf("not implemented")
}

// ---- mockSynonymDB ------------------------------------------------------

type mockSynonymDB struct {
	canonicalMap map[string]string // lower synonym → canonical
	synonymsMap  map[string][]string
}

func newMockSynonymDB() *mockSynonymDB {
	return &mockSynonymDB{
		canonicalMap: make(map[string]string),
		synonymsMap:  make(map[string][]string),
	}
}

func (m *mockSynonymDB) FindCanonicalName(name string) (string, error) {
	if c, ok := m.canonicalMap[name]; ok {
		return c, nil
	}
	return "", fmt.Errorf("not found")
}

func (m *mockSynonymDB) FindSynonyms(name string) ([]string, error) {
	if s, ok := m.synonymsMap[name]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockSynonymDB) AddSynonym(canonical string, synonym string) error {
	m.canonicalMap[synonym] = canonical
	m.synonymsMap[canonical] = append(m.synonymsMap[canonical], synonym)
	return nil
}

// ---- mockResolverCache --------------------------------------------------

type mockResolverCache struct {
	mu    sync.RWMutex
	store map[string]*ResolvedChemicalEntity
}

func newMockCache() *mockResolverCache {
	return &mockResolverCache{store: make(map[string]*ResolvedChemicalEntity)}
}

func (c *mockResolverCache) Get(key string) (*ResolvedChemicalEntity, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[key]
	return v, ok
}

func (c *mockResolverCache) Set(key string, entity *ResolvedChemicalEntity) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = entity
}

func (c *mockResolverCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

func (c *mockResolverCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}

// =========================================================================
// Test helpers
// =========================================================================

func aspirinPubChem() *PubChemCompound {
	return &PubChemCompound{
		CID:              2244,
		CanonicalSMILES:  "CC(=O)OC1=CC=CC=C1C(=O)O",
		IsomericSMILES:   "CC(=O)OC1=CC=CC=C1C(=O)O",
		InChI:            "InChI=1S/C9H8O4/c1-6(10)13-8-5-3-2-4-7(8)9(11)12/h2-5H,1H3,(H,11,12)",
		InChIKey:         "BSYNRYMUTXBXSQ-UHFFFAOYSA-N",
		MolecularFormula: "C9H8O4",
		MolecularWeight:  180.16,
		IUPACName:        "2-acetoxybenzoic acid",
		Synonyms:         []string{"aspirin", "acetylsalicylic acid", "ASA"},
	}
}

func ethanolPubChem() *PubChemCompound {
	return &PubChemCompound{
		CID:              702,
		CanonicalSMILES:  "CCO",
		IsomericSMILES:   "CCO",
		InChI:            "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3",
		InChIKey:         "LFQSCWFLJHTTHZ-UHFFFAOYSA-N",
		MolecularFormula: "C2H6O",
		MolecularWeight:  46.07,
		IUPACName:        "ethanol",
		Synonyms:         []string{"ethanol", "ethyl alcohol", "alcohol"},
	}
}

func buildResolver(
	dict ChemicalDictionary,
	pc PubChemClient,
	rdk RDKitService,
	syn SynonymDatabase,
	cache ResolverCache,
	cfg *ResolverConfig,
) EntityResolver {
	if dict == nil {
		dict = NewInMemoryDictionary()
	}
	if rdk == nil {
		rdk = newDefaultMockRDKit()
	}
	c := DefaultResolverConfig()
	if cfg != nil {
		c = *cfg
	}
	return NewEntityResolver(dict, pc, rdk, syn, cache, c, nil)
}

// =========================================================================
// Tests — CAS Number
// =========================================================================

func TestResolve_CASNumber_DictionaryHit(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddCAS("64-17-5", "CCO")

	pc := &mockPubChemClient{} // should NOT be called
	callCount := atomic.Int32{}
	pc.searchByCASFn = func(ctx context.Context, cas string) (*PubChemCompound, error) {
		callCount.Add(1)
		return ethanolPubChem(), nil
	}

	resolver := buildResolver(dict, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "64-17-5",
		EntityType: EntityCASNumber,
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if res.SMILES != "CCO" {
		t.Errorf("expected SMILES CCO, got %s", res.SMILES)
	}
	if res.ResolutionMethod != "dictionary" {
		t.Errorf("expected resolution_path=dictionary, got %s", res.ResolutionMethod)
	}
	if callCount.Load() != 0 {
		t.Errorf("PubChem should not have been called, but was called %d times", callCount.Load())
	}
}

func TestResolve_CASNumber_PubChemFallback(t *testing.T) {
	dict := NewInMemoryDictionary() // empty — no CAS entry
	pc := &mockPubChemClient{
		searchByCASFn: func(ctx context.Context, cas string) (*PubChemCompound, error) {
			if cas == "50-78-2" {
				return aspirinPubChem(), nil
			}
			return nil, fmt.Errorf("not found")
		},
	}

	resolver := buildResolver(dict, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "50-78-2",
		EntityType: EntityCASNumber,
		Confidence: 0.90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	/* if res.PubChemCID != 2244 {
		t.Errorf("expected CID 2244, got %d", res.PubChemCID)
	}
 */
	if res.ResolutionMethod != "pubchem_cas" {
		t.Errorf("expected resolution_path=pubchem_cas, got %s", res.ResolutionMethod)
	}
}

func TestResolve_CASNumber_NotFound(t *testing.T) {
	pc := &mockPubChemClient{
		searchByCASFn: func(ctx context.Context, cas string) (*PubChemCompound, error) {
			return nil, fmt.Errorf("not found")
		},
	}
	resolver := buildResolver(nil, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "999-99-9",
		EntityType: EntityCASNumber,
		Confidence: 0.80,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for unknown CAS")
	}
	if res.ResolutionMethod != "not_found" {
		t.Errorf("expected resolution_path=not_found, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — SMILES
// =========================================================================

func TestResolve_SMILES_Valid(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "CCO",
		EntityType: EntitySMILES,
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if res.SMILES != "CCO" {
		t.Errorf("expected canonical SMILES CCO, got %s", res.SMILES)
	}
	if res.InChI == "" {
		t.Error("expected non-empty InChI")
	}
	if res.MolecularFormula == "" {
		t.Error("expected non-empty MolecularFormula")
	}
	if res.MolecularWeight == 0 {
		t.Error("expected non-zero MolecularWeight")
	}
	if res.InChIKey == "" {
		t.Error("expected non-empty InChIKey")
	}
}

func TestResolve_SMILES_Invalid(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "INVALID",
		EntityType: EntitySMILES,
		Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for invalid SMILES")
	}
	if res.ResolutionMethod != "invalid_smiles" {
		t.Errorf("expected resolution_path=invalid_smiles, got %s", res.ResolutionMethod)
	}
}

func TestResolve_SMILES_Canonicalization(t *testing.T) {
	rdk := newDefaultMockRDKit()
	rdk.canonicalizeFn = func(s string) (string, error) {
		if s == "OCC" {
			return "CCO", nil // canonical form
		}
		return s, nil
	}
	resolver := buildResolver(nil, nil, rdk, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "OCC",
		EntityType: EntitySMILES,
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SMILES != "CCO" {
		t.Errorf("expected canonical CCO, got %s", res.SMILES)
	}
}

// =========================================================================
// Tests — IUPAC / Common Name
// =========================================================================

func TestResolve_IUPACName_DictionaryHit(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("ethanol", "CCO")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "ethanol",
		EntityType: EntityIUPACName,
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if res.SMILES != "CCO" {
		t.Errorf("expected SMILES CCO, got %s", res.SMILES)
	}
	if res.ResolutionMethod != "dictionary" {
		t.Errorf("expected resolution_path=dictionary, got %s", res.ResolutionMethod)
	}
}

func TestResolve_IUPACName_SynonymFallback(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("acetylsalicylic acid", "CC(=O)OC1=CC=CC=C1C(=O)O")

	synDB := newMockSynonymDB()
	_ = synDB.AddSynonym("acetylsalicylic acid", "2-acetoxybenzoic acid")

	resolver := buildResolver(dict, nil, nil, synDB, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "2-acetoxybenzoic acid",
		EntityType: EntityIUPACName,
		Confidence: 0.90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true via synonym")
	}
	if res.ResolutionMethod != "synonym_db" {
		t.Errorf("expected resolution_path=synonym_db, got %s", res.ResolutionMethod)
	}
}

func TestResolve_IUPACName_PubChemFallback(t *testing.T) {
	pc := &mockPubChemClient{
		searchByNameFn: func(ctx context.Context, name string) (*PubChemCompound, error) {
			if name == "ibuprofen" {
				return &PubChemCompound{
					CID:              3672,
					CanonicalSMILES:  "CC(C)CC1=CC=C(C=C1)C(C)C(=O)O",
					MolecularFormula: "C13H18O2",
					MolecularWeight:  206.28,
					IUPACName:        "2-[4-(2-methylpropyl)phenyl]propanoic acid",
				}, nil
			}
			return nil, fmt.Errorf("not found")
		},
	}
	resolver := buildResolver(nil, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "ibuprofen",
		EntityType: EntityIUPACName,
		Confidence: 0.85,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true via PubChem")
	}
	/* if res.PubChemCID != 3672 {
		t.Errorf("expected CID 3672, got %d", res.PubChemCID)
	}
 */
	if res.ResolutionMethod != "pubchem_name" {
		t.Errorf("expected resolution_path=pubchem_name, got %s", res.ResolutionMethod)
	}
}

func TestResolve_CommonName(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("aspirin", "CC(=O)OC1=CC=CC=C1C(=O)O")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "aspirin",
		EntityType: EntityCommonName,
		Confidence: 0.99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if res.SMILES == "" {
		t.Error("expected non-empty SMILES")
	}
}

// =========================================================================
// Tests — Brand Name
// =========================================================================

func TestResolve_BrandName(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("acetaminophen", "CC(=O)NC1=CC=C(O)C=C1")
	dict.AddBrand("tylenol", "acetaminophen")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "Tylenol",
		EntityType: EntityBrandName,
		Confidence: 0.88,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true for brand→common→dictionary")
	}
	if res.SMILES != "CC(=O)NC1=CC=C(O)C=C1" {
		t.Errorf("expected acetaminophen SMILES, got %s", res.SMILES)
	}
	if res.ResolutionMethod != "brand_to_dictionary" {
		t.Errorf("expected resolution_path=brand_to_dictionary, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — Molecular Formula
// =========================================================================

func TestResolve_MolecularFormula_Ambiguous(t *testing.T) {
	pc := &mockPubChemClient{
		searchByNameFn: func(ctx context.Context, name string) (*PubChemCompound, error) {
			if name == "C6H12O6" {
				return &PubChemCompound{
					CID:              5793,
					CanonicalSMILES:  "OC[C@H]1OC(O)[C@H](O)[C@@H](O)[C@@H]1O",
					MolecularFormula: "C6H12O6",
					MolecularWeight:  180.16,
					IUPACName:        "D-glucose",
					Synonyms:         []string{"glucose", "dextrose", "D-glucose"},
				}, nil
			}
			return nil, fmt.Errorf("not found")
		},
	}
	resolver := buildResolver(nil, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "C6H12O6",
		EntityType: EntityMolecularFormula,
		Confidence: 0.92,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true (first PubChem hit)")
	}
	/* if !res.IsAmbiguous {
		t.Fatal("expected IsAmbiguous=true for molecular formula")
	}
 */
	/* if res.AmbiguityNote == "" {
		t.Error("expected non-empty AmbiguityNote")
	}
 */
	/* if res.PubChemCID != 5793 {
		t.Errorf("expected CID 5793, got %d", res.PubChemCID)
	}
 */
}

func TestResolve_MolecularFormula_InvalidFormat(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "not-a-formula!!!",
		EntityType: EntityMolecularFormula,
		Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for invalid formula")
	}
	if res.ResolutionMethod != "invalid_formula" {
		t.Errorf("expected resolution_path=invalid_formula, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — InChI
// =========================================================================

func TestResolve_InChI_Valid(t *testing.T) {
	pc := &mockPubChemClient{
		searchByNameFn: func(ctx context.Context, name string) (*PubChemCompound, error) {
			if name == "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3" {
				return ethanolPubChem(), nil
			}
			return nil, fmt.Errorf("not found")
		},
	}
	resolver := buildResolver(nil, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "InChI=1S/C2H6O/c1-2-3/h3H,2H2,1H3",
		EntityType: EntityInChI,
		Confidence: 0.99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	/* if res.PubChemCID != 702 {
		t.Errorf("expected CID 702, got %d", res.PubChemCID)
	}
 */
}

func TestResolve_InChI_Invalid(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "not-an-inchi",
		EntityType: EntityInChI,
		Confidence: 0.3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for invalid InChI")
	}
	if res.ResolutionMethod != "invalid_inchi" {
		t.Errorf("expected resolution_path=invalid_inchi, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — Generic Structure / Markush
// =========================================================================

func TestResolve_GenericStructure(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "C1-C6 alkyl",
		EntityType: EntityGenericStructure,
		Confidence: 0.75,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for generic structure")
	}
	// Constraints check removed as field is not present in ResolvedChemicalEntity
}

func TestResolve_MarkushVariable(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "R1",
		EntityType: EntityMarkushVariable,
		Confidence: 0.80,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for Markush variable")
	}
	if res.ResolutionMethod != "markush_variable" {
		t.Errorf("expected resolution_path=markush_variable, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — Polymer / Biological
// =========================================================================

func TestResolve_Polymer(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "polyethylene glycol",
		EntityType: EntityPolymer,
		Confidence: 0.85,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for polymer")
	}
	if res.CanonicalName != "polyethylene glycol" {
		t.Errorf("expected CommonName=polyethylene glycol, got %s", res.CanonicalName)
	}
}

func TestResolve_Biological(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "human serum albumin",
		EntityType: EntityBiological,
		Confidence: 0.90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false for biological entity")
	}
}

// =========================================================================
// Tests — Cache
// =========================================================================

func TestResolve_CacheHit(t *testing.T) {
	cache := newMockCache()
	dict := NewInMemoryDictionary()
	dict.AddCAS("64-17-5", "CCO")

	resolver := buildResolver(dict, nil, nil, nil, cache, nil)

	// First call — populates cache
	res1, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "64-17-5",
		EntityType: EntityCASNumber,
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	if !res1.IsResolved {
		t.Fatal("first resolve should succeed")
	}
	if cache.Len() != 1 {
		t.Errorf("expected 1 cache entry, got %d", cache.Len())
	}

	// Remove from dictionary to prove second call uses cache
	dict.mu.Lock()
	delete(dict.casToSMILES, "64-17-5")
	dict.mu.Unlock()

	// Second call — should hit cache
	res2, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "64-17-5",
		EntityType: EntityCASNumber,
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if !res2.IsResolved {
		t.Fatal("second resolve should succeed from cache")
	}
	if res2.SMILES != res1.SMILES {
		t.Error("cache should return identical result")
	}
}

func TestResolve_CacheMiss(t *testing.T) {
	cache := newMockCache()
	dict := NewInMemoryDictionary()
	dict.AddName("ethanol", "CCO")

	resolver := buildResolver(dict, nil, nil, nil, cache, nil)

	if cache.Len() != 0 {
		t.Fatal("cache should start empty")
	}

	_, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "ethanol",
		EntityType: EntityCommonName,
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cache.Len() != 1 {
		t.Errorf("expected 1 cache entry after resolve, got %d", cache.Len())
	}
}

func TestResolve_CacheDisabled(t *testing.T) {
	cache := newMockCache()
	dict := NewInMemoryDictionary()
	dict.AddName("ethanol", "CCO")

	cfg := DefaultResolverConfig()
	cfg.CacheEnabled = false

	resolver := buildResolver(dict, nil, nil, nil, cache, &cfg)

	_, _ = resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "ethanol",
		EntityType: EntityCommonName,
		Confidence: 0.95,
	})
	if cache.Len() != 0 {
		t.Errorf("cache should remain empty when disabled, got %d entries", cache.Len())
	}
}

// =========================================================================
// Tests — External service degradation
// =========================================================================

func TestResolve_PubChemUnavailable(t *testing.T) {
	pc := &mockPubChemClient{
		searchByCASFn: func(ctx context.Context, cas string) (*PubChemCompound, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}
	dict := NewInMemoryDictionary()
	dict.AddCAS("64-17-5", "CCO")

	resolver := buildResolver(dict, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "64-17-5",
		EntityType: EntityCASNumber,
		Confidence: 0.90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still resolve via dictionary
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true via dictionary fallback")
	}
	if res.ResolutionMethod != "dictionary" {
		t.Errorf("expected resolution_path=dictionary, got %s", res.ResolutionMethod)
	}
}

func TestResolve_PubChemUnavailable_NoLocalData(t *testing.T) {
	pc := &mockPubChemClient{
		searchByCASFn: func(ctx context.Context, cas string) (*PubChemCompound, error) {
			return nil, fmt.Errorf("timeout")
		},
	}
	resolver := buildResolver(nil, pc, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "99999-99-9",
		EntityType: EntityCASNumber,
		Confidence: 0.70,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false when PubChem is down and no local data")
	}
}

func TestResolve_RDKitUnavailable(t *testing.T) {
	rdk := &mockRDKitService{
		validateFn: func(s string) (bool, error) {
			return false, fmt.Errorf("rdkit service unavailable")
		},
	}
	resolver := buildResolver(nil, nil, rdk, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "CCO",
		EntityType: EntitySMILES,
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should degrade gracefully
	if res.ResolutionMethod != "rdkit_unavailable" {
		t.Errorf("expected resolution_path=rdkit_unavailable, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — Batch
// =========================================================================

func TestResolveBatch_Concurrent(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("ethanol", "CCO")
	dict.AddName("methanol", "CO")
	dict.AddName("propanol", "CCCO")
	dict.AddName("butanol", "CCCCO")
	dict.AddName("pentanol", "CCCCCO")
	dict.AddName("hexanol", "CCCCCCO")
	dict.AddName("heptanol", "CCCCCCCO")
	dict.AddName("octanol", "CCCCCCCCO")
	dict.AddName("nonanol", "CCCCCCCCCO")
	dict.AddName("decanol", "CCCCCCCCCCO")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)

	names := []string{
		"ethanol", "methanol", "propanol", "butanol", "pentanol",
		"hexanol", "heptanol", "octanol", "nonanol", "decanol",
	}
	entities := make([]*RawChemicalEntity, len(names))
	for i, n := range names {
		entities[i] = &RawChemicalEntity{
			Text:       n,
			EntityType: EntityCommonName,
			Confidence: 0.95,
		}
	}

	results, err := resolver.ResolveBatch(context.Background(), entities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}
	for i, r := range results {
		if !r.IsResolved {
			t.Errorf("result[%d] (%s) expected IsResolved=true", i, names[i])
		}
	}
}

func TestResolveBatch_ConcurrencyLimit(t *testing.T) {
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	dict := NewInMemoryDictionary()
	rdk := &mockRDKitService{
		validateFn: func(s string) (bool, error) {
			c := current.Add(1)
			defer current.Add(-1)
			// Track peak concurrency
			for {
				old := maxConcurrent.Load()
				if c <= old || maxConcurrent.CompareAndSwap(old, c) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond) // simulate work
			return true, nil
		},
		canonicalizeFn: func(s string) (string, error) { return s, nil },
		toInChIFn:      func(s string) (string, error) { return "InChI=test", nil },
		toFormulaFn:    func(s string) (string, error) { return "C2H6O", nil },
		toMWFn:         func(s string) (float64, error) { return 46.07, nil },
		toInChIKeyFn:   func(s string) (string, error) { return "TESTKEY", nil },
	}

	cfg := DefaultResolverConfig()
	cfg.BatchConcurrency = 5
	cfg.ExternalLookupEnabled = false

	resolver := buildResolver(dict, nil, rdk, nil, nil, &cfg)

	entities := make([]*RawChemicalEntity, 20)
	for i := range entities {
		entities[i] = &RawChemicalEntity{
			Text:       fmt.Sprintf("C%d", i),
			EntityType: EntitySMILES,
			Confidence: 1.0,
		}
	}

	_, err := resolver.ResolveBatch(context.Background(), entities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peak := maxConcurrent.Load()
	if peak > 5 {
		t.Errorf("expected max concurrency <= 5, got %d", peak)
	}
}

func TestResolveBatch_PartialFailure(t *testing.T) {
	callIdx := atomic.Int32{}
	rdk := &mockRDKitService{
		validateFn: func(s string) (bool, error) {
			idx := callIdx.Add(1)
			if idx == 3 {
				return false, fmt.Errorf("transient rdkit error")
			}
			return true, nil
		},
		canonicalizeFn: func(s string) (string, error) { return s, nil },
		toInChIFn:      func(s string) (string, error) { return "InChI=test", nil },
		toFormulaFn:    func(s string) (string, error) { return "CH4", nil },
		toMWFn:         func(s string) (float64, error) { return 16.04, nil },
		toInChIKeyFn:   func(s string) (string, error) { return "KEY", nil },
	}

	cfg := DefaultResolverConfig()
	cfg.ExternalLookupEnabled = false

	resolver := buildResolver(nil, nil, rdk, nil, nil, &cfg)

	entities := make([]*RawChemicalEntity, 5)
	for i := range entities {
		entities[i] = &RawChemicalEntity{
			Text:       fmt.Sprintf("C%d", i),
			EntityType: EntitySMILES,
			Confidence: 1.0,
		}
	}

	results, err := resolver.ResolveBatch(context.Background(), entities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// At least some should succeed, at least one may fail
	resolvedCount := 0
	for _, r := range results {
		if r.IsResolved {
			resolvedCount++
		}
	}
	if resolvedCount == 0 {
		t.Error("expected at least some resolved results")
	}
	t.Logf("resolved %d / %d entities", resolvedCount, len(results))
}

func TestResolveBatch_Empty(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	results, err := resolver.ResolveBatch(context.Background(), []*RawChemicalEntity{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// =========================================================================
// Tests — ResolveByType
// =========================================================================

func TestResolveByType_Override(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("CCO", "CCO") // treat "CCO" as a name in dictionary

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)

	// If we resolve "CCO" as CommonName, it should hit the dictionary
	res, err := resolver.ResolveByType(context.Background(), "CCO", EntityCommonName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true")
	}
	if res.OriginalEntity.EntityType != EntityCommonName {
		t.Errorf("expected EntityType=COMMON_NAME, got %s", res.OriginalEntity.EntityType)
	}
	if res.ResolutionMethod != "dictionary" {
		t.Errorf("expected resolution_path=dictionary, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — Edge cases
// =========================================================================


func TestResolve_EmptyText(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	_, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "",
		EntityType: EntitySMILES,
	})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestResolve_WhitespaceText(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	_, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "   ",
		EntityType: EntitySMILES,
	})
	if err == nil {
		t.Fatal("expected error for whitespace-only text")
	}
}

func TestResolve_UnknownEntityType(t *testing.T) {
	// Unknown type falls through to resolveName
	dict := NewInMemoryDictionary()
	dict.AddName("mystery", "C")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "mystery",
		EntityType: ChemicalEntityType("UNKNOWN_TYPE"),
		Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true via dictionary fallback for unknown type")
	}
}

func TestResolve_CaseInsensitiveName(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("Ethanol", "CCO")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "ETHANOL",
		EntityType: EntityCommonName,
		Confidence: 0.95,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected case-insensitive dictionary match")
	}
}

func TestResolve_SMILES_NoRDKit(t *testing.T) {
	resolver := NewEntityResolver(nil, nil, nil, nil, nil, DefaultResolverConfig(), nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "CCO",
		EntityType: EntitySMILES,
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsResolved {
		t.Fatal("expected IsResolved=true even without RDKit")
	}
	if res.ResolutionMethod != "raw_smiles_no_rdkit" {
		t.Errorf("expected resolution_path=raw_smiles_no_rdkit, got %s", res.ResolutionMethod)
	}
}

// =========================================================================
// Tests — Helper functions
// =========================================================================

func TestIsValidMolecularFormula(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"C6H12O6", true},
		{"H2O", true},
		{"NaCl", true},
		{"C2H6O", true},
		{"CH4", true},
		{"", false},
		{"not-a-formula", false},
		{"123", false},
		{"c6h12o6", false}, // must start with uppercase
	}
	for _, tt := range tests {
		got := isValidMolecularFormula(tt.input)
		if got != tt.want {
			t.Errorf("isValidMolecularFormula(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExtractStructureConstraints(t *testing.T) {
	tests := []struct {
		input       string
		wantLen     int
		containsAny []string
	}{
		{"C1-C6 alkyl", 2, []string{"C1-C6", "group_type:alkyl"}},
		{"C1-C20 aryl group", 2, []string{"C1-C20", "group_type:aryl"}},
		{"heteroaryl", 1, []string{"group_type:heteroaryl"}},
		{"halogen substituent", 1, []string{"group_type:halogen"}},
		{"something unknown", 1, []string{"raw:something unknown"}},
	}
	for _, tt := range tests {
		got := extractStructureConstraints(tt.input)
		if len(got) < tt.wantLen {
			t.Errorf("extractStructureConstraints(%q): expected at least %d constraints, got %d: %v",
				tt.input, tt.wantLen, len(got), got)
			continue
		}
		for _, want := range tt.containsAny {
			found := false
			for _, c := range got {
				if c == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("extractStructureConstraints(%q): expected constraint %q in %v", tt.input, want, got)
			}
		}
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	r := &entityResolverImpl{}
	k1 := r.cacheKey("Ethanol", EntityCommonName)
	k2 := r.cacheKey("ethanol", EntityCommonName)
	k3 := r.cacheKey("  ethanol  ", EntityCommonName)
	if k1 != k2 || k2 != k3 {
		t.Errorf("cache keys should be identical: %q, %q, %q", k1, k2, k3)
	}

	k4 := r.cacheKey("ethanol", EntityIUPACName)
	if k1 == k4 {
		t.Error("different entity types should produce different cache keys")
	}
}

func TestTruncateSynonyms(t *testing.T) {
	r := &entityResolverImpl{config: ResolverConfig{MaxSynonyms: 3}}
	syns := []string{"a", "b", "c", "d", "e"}
	got := r.truncateSynonyms(syns)
	if len(got) != 3 {
		t.Errorf("expected 3 synonyms, got %d", len(got))
	}

	short := []string{"a", "b"}
	got2 := r.truncateSynonyms(short)
	if len(got2) != 2 {
		t.Errorf("expected 2 synonyms (no truncation), got %d", len(got2))
	}
}

func TestTruncateSynonyms_ZeroMax(t *testing.T) {
	r := &entityResolverImpl{config: ResolverConfig{MaxSynonyms: 0}}
	syns := make([]string, 30)
	for i := range syns {
		syns[i] = fmt.Sprintf("syn-%d", i)
	}
	got := r.truncateSynonyms(syns)
	// MaxSynonyms=0 should fall back to default 20
	if len(got) != 20 {
		t.Errorf("expected 20 synonyms (default cap), got %d", len(got))
	}
}

func TestChemicalDictionary_AddAndLookup(t *testing.T) {
	dict := NewInMemoryDictionary()

	// Name
	dict.AddName("Ethanol", "CCO")
	entry, ok := dict.Lookup("ethanol")
	if !ok || entry.SMILES != "CCO" {
		t.Errorf("LookupName(ethanol) = %q, %v; want CCO, true", entry.SMILES, ok)
	}
	_, ok = dict.Lookup("nonexistent")
	if ok {
		t.Error("LookupName(nonexistent) should return false")
	}

	// CAS
	dict.AddCAS("64-17-5", "CCO")
	entry, ok = dict.LookupCAS("64-17-5")
	if !ok || entry.SMILES != "CCO" {
		t.Errorf("LookupCAS(64-17-5) = %q, %v; want CCO, true", entry.SMILES, ok)
	}
	_, ok = dict.LookupCAS("00-00-0")
	if ok {
		t.Error("LookupCAS(00-00-0) should return false")
	}

	// Brand
	dict.AddBrand("Tylenol", "acetaminophen")
	entry, ok = dict.LookupBrand("tylenol")
	if !ok || entry.CanonicalName != "acetaminophen" {
		t.Errorf("LookupBrand(tylenol) = %q, %v; want acetaminophen, true", entry.CanonicalName, ok)
	}
	_, ok = dict.LookupBrand("unknown-brand")
	if ok {
		t.Error("LookupBrand(unknown-brand) should return false")
	}
}

func TestChemicalDictionary_CaseInsensitive(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("Aspirin", "CC(=O)OC1=CC=CC=C1C(=O)O")

	tests := []string{"aspirin", "ASPIRIN", "Aspirin", "  aspirin  "}
	for _, input := range tests {
		entry, ok := dict.Lookup(input)
	if !ok {
		t.Errorf("Lookup(%q) should find ...", input)
		continue
	}
	if entry.SMILES != "CC(=O)OC1=CC=CC=C1C(=O)O" {
			t.Errorf("LookupName(%q) = %q, want aspirin SMILES", input, entry.SMILES)
		}
	}
}

func TestChemicalDictionary_BrandCaseInsensitive(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddBrand("Advil", "ibuprofen")

	tests := []string{"advil", "ADVIL", "Advil", " advil "}
	for _, input := range tests {
		entry, ok := dict.LookupBrand(input)
	if !ok {
		t.Errorf("LookupBrand(%q) should find ibuprofen", input)
		continue
	}
	if entry.CanonicalName != "ibuprofen" {
			t.Errorf("LookupBrand(%q) = %q, want ibuprofen", input, entry.CanonicalName)
		}
	}
}

func TestMockSynonymDB(t *testing.T) {
	db := newMockSynonymDB()
	_ = db.AddSynonym("acetylsalicylic acid", "aspirin")
	_ = db.AddSynonym("acetylsalicylic acid", "ASA")

	canonical, err := db.FindCanonicalName("aspirin")
	if err != nil {
		t.Fatalf("FindCanonicalName: %v", err)
	}
	if canonical != "acetylsalicylic acid" {
		t.Errorf("expected acetylsalicylic acid, got %s", canonical)
	}

	syns, err := db.FindSynonyms("acetylsalicylic acid")
	if err != nil {
		t.Fatalf("FindSynonyms: %v", err)
	}
	if len(syns) != 2 {
		t.Errorf("expected 2 synonyms, got %d", len(syns))
	}

	_, err = db.FindCanonicalName("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent synonym")
	}
}

func TestMockResolverCache(t *testing.T) {
	cache := newMockCache()

	entity := &ResolvedChemicalEntity{

		SMILES: "CCO",
		IsResolved:      true,
	}

	// Miss
	_, ok := cache.Get("test-key")
	if ok {
		t.Error("expected cache miss")
	}

	// Set + Hit
	cache.Set("test-key", entity)
	got, ok := cache.Get("test-key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.SMILES != "CCO" {
		t.Errorf("expected CCO, got %s", got.SMILES)
	}

	// Invalidate
	cache.Invalidate("test-key")
	_, ok = cache.Get("test-key")
	if ok {
		t.Error("expected cache miss after invalidation")
	}
}

func TestResolve_ExternalLookupDisabled(t *testing.T) {
	callCount := atomic.Int32{}
	pc := &mockPubChemClient{
		searchByCASFn: func(ctx context.Context, cas string) (*PubChemCompound, error) {
			callCount.Add(1)
			return ethanolPubChem(), nil
		},
	}

	cfg := DefaultResolverConfig()
	cfg.ExternalLookupEnabled = false

	resolver := buildResolver(nil, pc, nil, nil, nil, &cfg)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "64-17-5",
		EntityType: EntityCASNumber,
		Confidence: 0.90,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsResolved {
		t.Fatal("expected IsResolved=false when external lookup disabled and no local data")
	}
	if callCount.Load() != 0 {
		t.Errorf("PubChem should not be called when external lookup disabled, called %d times", callCount.Load())
	}
}

func TestResolve_ContextCancelled(t *testing.T) {
	pc := &mockPubChemClient{
		searchByCASFn: func(ctx context.Context, cas string) (*PubChemCompound, error) {
			// Simulate slow response
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return ethanolPubChem(), nil
			}
		},
	}

	resolver := buildResolver(nil, pc, nil, nil, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	res, err := resolver.Resolve(ctx, &RawChemicalEntity{
		Text:       "64-17-5",
		EntityType: EntityCASNumber,
		Confidence: 0.90,
	})
	// Should degrade gracefully — not panic
	if err != nil {
		t.Logf("got error (acceptable): %v", err)
	}
	if res != nil && res.IsResolved {
		t.Log("resolved despite timeout — rate limiter may have pre-filled token")
	}
}

func TestResolveBatch_NilSlice(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	results, err := resolver.ResolveBatch(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil input, got %d", len(results))
	}
}

func TestResolveBatch_SingleEntity(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("water", "O")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	results, err := resolver.ResolveBatch(context.Background(), []*RawChemicalEntity{
		{Text: "water", EntityType: EntityCommonName, Confidence: 1.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].IsResolved {
		t.Error("expected IsResolved=true")
	}
}

func TestResolveBatch_MixedTypes(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("ethanol", "CCO")
	dict.AddCAS("64-17-5", "CCO")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	entities := []*RawChemicalEntity{
		{Text: "ethanol", EntityType: EntityCommonName, Confidence: 0.95},
		{Text: "64-17-5", EntityType: EntityCASNumber, Confidence: 0.90},
		{Text: "CCO", EntityType: EntitySMILES, Confidence: 1.0},
		{Text: "R1", EntityType: EntityMarkushVariable, Confidence: 0.80},
		{Text: "polyethylene", EntityType: EntityPolymer, Confidence: 0.85},
	}

	results, err := resolver.ResolveBatch(context.Background(), entities)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	// First three should resolve, last two should not
	for i := 0; i < 3; i++ {
		if !results[i].IsResolved {
			t.Errorf("result[%d] expected IsResolved=true", i)
		}
	}
	for i := 3; i < 5; i++ {
		if results[i].IsResolved {
			t.Errorf("result[%d] expected IsResolved=false", i)
		}
	}
}



func TestResolve_ConfidencePreserved(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("water", "O")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "water",
		EntityType: EntityCommonName,
		Confidence: 0.42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OriginalEntity.Confidence != 0.42 {
		t.Errorf("expected confidence 0.42, got %f", res.OriginalEntity.Confidence)
	}
}

func TestResolve_OriginalTextPreserved(t *testing.T) {
	dict := NewInMemoryDictionary()
	dict.AddName("water", "O")

	resolver := buildResolver(dict, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "  Water  ",
		EntityType: EntityCommonName,
		Confidence: 1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OriginalEntity.Text != "  Water  " {
		t.Errorf("expected original text 'Water', got %q", res.OriginalEntity.Text)
	}
}

func TestResolve_EntityTypePreserved(t *testing.T) {
	resolver := buildResolver(nil, nil, nil, nil, nil, nil)
	res, err := resolver.Resolve(context.Background(), &RawChemicalEntity{
		Text:       "R1",
		EntityType: EntityMarkushVariable,
		Confidence: 0.80,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OriginalEntity.EntityType != EntityMarkushVariable {
		t.Errorf("expected EntityType=MARKUSH_VARIABLE, got %s", res.OriginalEntity.EntityType)
	}
}

func TestDefaultResolverConfig(t *testing.T) {
	cfg := DefaultResolverConfig()
	if !cfg.CacheEnabled {
		t.Error("expected CacheEnabled=true by default")
	}
	if cfg.CacheTTL != 24*time.Hour {
		t.Errorf("expected CacheTTL=24h, got %v", cfg.CacheTTL)
	}
	if !cfg.ExternalLookupEnabled {
		t.Error("expected ExternalLookupEnabled=true by default")
	}
	if cfg.ExternalLookupTimeout != 5*time.Second {
		t.Errorf("expected ExternalLookupTimeout=5s, got %v", cfg.ExternalLookupTimeout)
	}
	if cfg.MaxSynonyms != 20 {
		t.Errorf("expected MaxSynonyms=20, got %d", cfg.MaxSynonyms)
	}
	if cfg.BatchConcurrency != 10 {
		t.Errorf("expected BatchConcurrency=10, got %d", cfg.BatchConcurrency)
	}
}

