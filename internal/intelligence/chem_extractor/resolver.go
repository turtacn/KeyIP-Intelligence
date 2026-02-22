package chem_extractor

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// EntityResolver resolves raw chemical entity text into standardised
// chemical representations (SMILES, InChI, molecular formula, etc.).
type EntityResolver interface {
	Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error)
	ResolveBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ResolvedChemicalEntity, error)
	ResolveByType(ctx context.Context, text string, entityType ChemicalEntityType) (*ResolvedChemicalEntity, error)
}

// PubChemClient abstracts the PubChem REST API.
type PubChemClient interface {
	SearchByName(ctx context.Context, name string) (*PubChemCompound, error)
	SearchByCAS(ctx context.Context, cas string) (*PubChemCompound, error)
	SearchBySMILES(ctx context.Context, smiles string) (*PubChemCompound, error)
	GetCompound(ctx context.Context, cid int) (*PubChemCompound, error)
}

// RDKitService abstracts cheminformatics operations (typically backed by a
// Python micro-service or CGo binding).
type RDKitService interface {
	ValidateSMILES(smiles string) (bool, error)
	CanonicalizeSMILES(smiles string) (string, error)
	SMILESToInChI(smiles string) (string, error)
	SMILESToMolecularFormula(smiles string) (string, error)
	ComputeMolecularWeight(smiles string) (float64, error)
	SMILESToInChIKey(smiles string) (string, error)
}

// SynonymDatabase maps chemical synonyms to canonical names.
type SynonymDatabase interface {
	FindSynonyms(name string) ([]string, error)
	FindCanonicalName(name string) (string, error)
	AddSynonym(canonical string, synonym string) error
}

// ResolverCache caches resolved entities.
type ResolverCache interface {
	Get(key string) (*ResolvedChemicalEntity, bool)
	Set(key string, entity *ResolvedChemicalEntity)
	Invalidate(key string)
}

// Logger is a minimal structured logger interface.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// PubChemCompound holds the essential fields returned by PubChem.
type PubChemCompound struct {
	CID              int      `json:"cid"`
	CanonicalSMILES  string   `json:"canonical_smiles"`
	IsomericSMILES   string   `json:"isomeric_smiles"`
	InChI            string   `json:"inchi"`
	InChIKey         string   `json:"inchi_key"`
	MolecularFormula string   `json:"molecular_formula"`
	MolecularWeight  float64  `json:"molecular_weight"`
	IUPACName        string   `json:"iupac_name"`
	Synonyms         []string `json:"synonyms"`
}

// RawChemicalEntity is the NER output that needs resolution.
type RawChemicalEntity struct {
	Text       string             `json:"text"`
	EntityType ChemicalEntityType `json:"entity_type"`
	StartPos   int                `json:"start_pos"`
	EndPos     int                `json:"end_pos"`
	Confidence float64            `json:"confidence"`
	Source     string             `json:"source,omitempty"`
}

// ResolvedChemicalEntity is the fully (or partially) resolved chemical entity.
type ResolvedChemicalEntity struct {
	OriginalText     string             `json:"original_text"`
	EntityType       ChemicalEntityType `json:"entity_type"`
	IsResolved       bool               `json:"is_resolved"`
	CanonicalSMILES  string             `json:"canonical_smiles,omitempty"`
	IsomericSMILES   string             `json:"isomeric_smiles,omitempty"`
	InChI            string             `json:"inchi,omitempty"`
	InChIKey         string             `json:"inchi_key,omitempty"`
	MolecularFormula string             `json:"molecular_formula,omitempty"`
	MolecularWeight  float64            `json:"molecular_weight,omitempty"`
	IUPACName        string             `json:"iupac_name,omitempty"`
	CommonName       string             `json:"common_name,omitempty"`
	CASNumber        string             `json:"cas_number,omitempty"`
	PubChemCID       int                `json:"pubchem_cid,omitempty"`
	Synonyms         []string           `json:"synonyms,omitempty"`
	IsAmbiguous      bool               `json:"is_ambiguous,omitempty"`
	AmbiguityNote    string             `json:"ambiguity_note,omitempty"`
	Constraints      []string           `json:"constraints,omitempty"`
	ResolutionPath   string             `json:"resolution_path,omitempty"`
	Confidence       float64            `json:"confidence"`
	ResolvedAt       time.Time          `json:"resolved_at"`
}

// ChemicalEntityType enumerates the kinds of chemical entities the NER
// pipeline can produce.
type ChemicalEntityType string

const (
	EntityCASNumber        ChemicalEntityType = "CAS_NUMBER"
	EntitySMILES           ChemicalEntityType = "SMILES"
	EntityIUPACName        ChemicalEntityType = "IUPAC_NAME"
	EntityCommonName       ChemicalEntityType = "COMMON_NAME"
	EntityMolecularFormula ChemicalEntityType = "MOLECULAR_FORMULA"
	EntityInChI            ChemicalEntityType = "INCHI"
	EntityGenericStructure ChemicalEntityType = "GENERIC_STRUCTURE"
	EntityMarkushVariable  ChemicalEntityType = "MARKUSH_VARIABLE"
	EntityBrandName        ChemicalEntityType = "BRAND_NAME"
	EntityPolymer          ChemicalEntityType = "POLYMER"
	EntityBiological       ChemicalEntityType = "BIOLOGICAL"
)

// ChemicalDictionary is a simple in-memory name → SMILES lookup.
type ChemicalDictionary struct {
	nameToSMILES map[string]string // lower-case name → SMILES
	casToSMILES  map[string]string // CAS number → SMILES
	brandToName  map[string]string // lower-case brand → lower-case common name
	mu           sync.RWMutex
}

// NewChemicalDictionary creates an empty dictionary.
func NewChemicalDictionary() *ChemicalDictionary {
	return &ChemicalDictionary{
		nameToSMILES: make(map[string]string),
		casToSMILES:  make(map[string]string),
		brandToName:  make(map[string]string),
	}
}

// AddName registers a name → SMILES mapping.
func (d *ChemicalDictionary) AddName(name, smiles string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nameToSMILES[strings.ToLower(strings.TrimSpace(name))] = smiles
}

// AddCAS registers a CAS → SMILES mapping.
func (d *ChemicalDictionary) AddCAS(cas, smiles string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.casToSMILES[strings.TrimSpace(cas)] = smiles
}

// AddBrand registers a brand → common-name mapping.
func (d *ChemicalDictionary) AddBrand(brand, commonName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.brandToName[strings.ToLower(strings.TrimSpace(brand))] = strings.ToLower(strings.TrimSpace(commonName))
}

// LookupName returns the SMILES for a name (case-insensitive).
func (d *ChemicalDictionary) LookupName(name string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.nameToSMILES[strings.ToLower(strings.TrimSpace(name))]
	return s, ok
}

// LookupCAS returns the SMILES for a CAS number.
func (d *ChemicalDictionary) LookupCAS(cas string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.casToSMILES[strings.TrimSpace(cas)]
	return s, ok
}

// LookupBrand returns the common name for a brand name.
func (d *ChemicalDictionary) LookupBrand(brand string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n, ok := d.brandToName[strings.ToLower(strings.TrimSpace(brand))]
	return n, ok
}

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

// ResolverConfig tunes the resolver behaviour.
type ResolverConfig struct {
	CacheEnabled          bool          `json:"cache_enabled" yaml:"cache_enabled"`
	CacheTTL              time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
	ExternalLookupEnabled bool          `json:"external_lookup_enabled" yaml:"external_lookup_enabled"`
	ExternalLookupTimeout time.Duration `json:"external_lookup_timeout" yaml:"external_lookup_timeout"`
	MaxSynonyms           int           `json:"max_synonyms" yaml:"max_synonyms"`
	BatchConcurrency      int           `json:"batch_concurrency" yaml:"batch_concurrency"`
}

// DefaultResolverConfig returns production-ready defaults.
func DefaultResolverConfig() ResolverConfig {
	return ResolverConfig{
		CacheEnabled:          true,
		CacheTTL:              24 * time.Hour,
		ExternalLookupEnabled: true,
		ExternalLookupTimeout: 5 * time.Second,
		MaxSynonyms:           20,
		BatchConcurrency:      10,
	}
}

// ---------------------------------------------------------------------------
// Noop logger
// ---------------------------------------------------------------------------

type noopLogger struct{}

func (noopLogger) Info(string, ...interface{})  {}
func (noopLogger) Warn(string, ...interface{})  {}
func (noopLogger) Error(string, ...interface{}) {}
func (noopLogger) Debug(string, ...interface{}) {}

// ---------------------------------------------------------------------------
// Rate limiter (token-bucket, 5 req/s for PubChem)
// ---------------------------------------------------------------------------

type rateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
	stop     chan struct{}
	once     sync.Once
}

func newRateLimiter(rps int) *rateLimiter {
	rl := &rateLimiter{
		tokens:   make(chan struct{}, rps),
		interval: time.Second / time.Duration(rps),
		stop:     make(chan struct{}),
	}
	// Pre-fill
	for i := 0; i < rps; i++ {
		rl.tokens <- struct{}{}
	}
	// Refill goroutine
	go func() {
		ticker := time.NewTicker(rl.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				select {
				case rl.tokens <- struct{}{}:
				default:
				}
			case <-rl.stop:
				return
			}
		}
	}()
	return rl
}

func (rl *rateLimiter) Acquire(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (rl *rateLimiter) Close() {
	rl.once.Do(func() { close(rl.stop) })
}

// ---------------------------------------------------------------------------
// entityResolverImpl
// ---------------------------------------------------------------------------

const pubchemRPS = 5

type entityResolverImpl struct {
	dictionary    *ChemicalDictionary
	pubchemClient PubChemClient
	rdkitService  RDKitService
	synonymDB     SynonymDatabase
	cache         ResolverCache
	config        ResolverConfig
	logger        Logger
	limiter       *rateLimiter
}

// NewEntityResolver creates a production EntityResolver.
//
// Any dependency may be nil; the resolver degrades gracefully when a
// component is unavailable.
func NewEntityResolver(
	dictionary *ChemicalDictionary,
	pubchemClient PubChemClient,
	rdkitService RDKitService,
	synonymDB SynonymDatabase,
	cache ResolverCache,
	config ResolverConfig,
	logger Logger,
) EntityResolver {
	if dictionary == nil {
		dictionary = NewChemicalDictionary()
	}
	if logger == nil {
		logger = noopLogger{}
	}
	return &entityResolverImpl{
		dictionary:    dictionary,
		pubchemClient: pubchemClient,
		rdkitService:  rdkitService,
		synonymDB:     synonymDB,
		cache:         cache,
		config:        config,
		logger:        logger,
		limiter:       newRateLimiter(pubchemRPS),
	}
}

// ---------------------------------------------------------------------------
// Resolve — single entity
// ---------------------------------------------------------------------------

func (r *entityResolverImpl) Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
	if entity == nil {
		return nil, errors.NewInvalidInputError("entity is nil")
	}
	text := strings.TrimSpace(entity.Text)
	if text == "" {
		return nil, errors.NewInvalidInputError("entity text is empty")
	}

	// Cache lookup
	cacheKey := r.cacheKey(text, entity.EntityType)
	if r.config.CacheEnabled && r.cache != nil {
		if cached, ok := r.cache.Get(cacheKey); ok {
			r.logger.Debug("cache hit", "key", cacheKey)
			return cached, nil
		}
	}

	var resolved *ResolvedChemicalEntity
	var err error

	switch entity.EntityType {
	case EntityCASNumber:
		resolved, err = r.resolveCAS(ctx, text)
	case EntitySMILES:
		resolved, err = r.resolveSMILES(ctx, text)
	case EntityIUPACName:
		resolved, err = r.resolveName(ctx, text, EntityIUPACName)
	case EntityCommonName:
		resolved, err = r.resolveName(ctx, text, EntityCommonName)
	case EntityMolecularFormula:
		resolved, err = r.resolveMolecularFormula(ctx, text)
	case EntityInChI:
		resolved, err = r.resolveInChI(ctx, text)
	case EntityGenericStructure:
		resolved, err = r.resolveGenericStructure(ctx, text)
	case EntityMarkushVariable:
		resolved, err = r.resolveMarkush(ctx, text)
	case EntityBrandName:
		resolved, err = r.resolveBrandName(ctx, text)
	case EntityPolymer:
		resolved, err = r.resolvePolymerOrBiological(ctx, text, EntityPolymer)
	case EntityBiological:
		resolved, err = r.resolvePolymerOrBiological(ctx, text, EntityBiological)
	default:
		resolved, err = r.resolveName(ctx, text, entity.EntityType)
	}

	if err != nil {
		return nil, err
	}

	resolved.OriginalText = text
	resolved.EntityType = entity.EntityType
	resolved.Confidence = entity.Confidence
	resolved.ResolvedAt = time.Now()

	// Cache store
	if r.config.CacheEnabled && r.cache != nil && resolved.IsResolved {
		r.cache.Set(cacheKey, resolved)
	}

	return resolved, nil
}

// ---------------------------------------------------------------------------
// ResolveBatch — concurrent with bounded parallelism
// ---------------------------------------------------------------------------

func (r *entityResolverImpl) ResolveBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ResolvedChemicalEntity, error) {
	if len(entities) == 0 {
		return []*ResolvedChemicalEntity{}, nil
	}

	concurrency := r.config.BatchConcurrency
	if concurrency <= 0 {
		concurrency = 10
	}

	results := make([]*ResolvedChemicalEntity, len(entities))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, ent := range entities {
		wg.Add(1)
		go func(idx int, e *RawChemicalEntity) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := r.Resolve(ctx, e)
			if err != nil {
				r.logger.Warn("batch resolve failed", "index", idx, "error", err)
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				// Store a partial / unresolved placeholder
				results[idx] = &ResolvedChemicalEntity{
					OriginalText: e.Text,
					EntityType:   e.EntityType,
					IsResolved:   false,
					ResolvedAt:   time.Now(),
				}
				return
			}
			results[idx] = res
		}(i, ent)
	}
	wg.Wait()

	return results, nil
}

// ---------------------------------------------------------------------------
// ResolveByType — explicit type override
// ---------------------------------------------------------------------------

func (r *entityResolverImpl) ResolveByType(ctx context.Context, text string, entityType ChemicalEntityType) (*ResolvedChemicalEntity, error) {
	return r.Resolve(ctx, &RawChemicalEntity{
		Text:       text,
		EntityType: entityType,
		Confidence: 1.0,
	})
}

// ---------------------------------------------------------------------------
// Type-specific resolution strategies
// ---------------------------------------------------------------------------

// resolveCAS: cache → dictionary → PubChem → RDKit enrichment
func (r *entityResolverImpl) resolveCAS(ctx context.Context, cas string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{CASNumber: cas}

	// 1. Dictionary
	if smiles, ok := r.dictionary.LookupCAS(cas); ok {
		res.CanonicalSMILES = smiles
		res.ResolutionPath = "dictionary"
		return r.enrichFromSMILES(ctx, res)
	}

	// 2. PubChem
	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByCAS(c, cas)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionPath = "pubchem_cas"
			return r.enrichFromSMILES(ctx, res)
		}
		if err != nil {
			r.logger.Warn("pubchem CAS lookup failed", "cas", cas, "error", err)
		}
	}

	// Not found
	res.IsResolved = false
	res.ResolutionPath = "not_found"
	return res, nil
}

// resolveSMILES: validate → canonicalize → compute InChI, formula, MW
func (r *entityResolverImpl) resolveSMILES(ctx context.Context, smiles string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{}

	if r.rdkitService == nil {
		res.CanonicalSMILES = smiles
		res.IsResolved = true
		res.ResolutionPath = "raw_smiles_no_rdkit"
		return res, nil
	}

	// 1. Validate
	valid, err := r.rdkitService.ValidateSMILES(smiles)
	if err != nil {
		r.logger.Warn("rdkit validate failed", "error", err)
		res.CanonicalSMILES = smiles
		res.IsResolved = false
		res.ResolutionPath = "rdkit_unavailable"
		return res, nil
	}
	if !valid {
		res.IsResolved = false
		res.ResolutionPath = "invalid_smiles"
		return res, nil
	}

	// 2. Canonicalize
	canonical, err := r.rdkitService.CanonicalizeSMILES(smiles)
	if err != nil {
		canonical = smiles
	}
	res.CanonicalSMILES = canonical

	// 3. Enrich
	res.ResolutionPath = "smiles_direct"
	enriched, err := r.enrichFromSMILES(ctx, res)
	if err != nil {
		return res, nil // partial is fine
	}

	// 4. Optionally look up name via InChIKey in PubChem
	if r.config.ExternalLookupEnabled && r.pubchemClient != nil && enriched.InChIKey != "" {
		compound, pcErr := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchBySMILES(c, canonical)
		})
		if pcErr == nil && compound != nil {
			if enriched.IUPACName == "" {
				enriched.IUPACName = compound.IUPACName
			}
			if enriched.PubChemCID == 0 {
				enriched.PubChemCID = compound.CID
			}
			enriched.Synonyms = r.truncateSynonyms(compound.Synonyms)
		}
	}

	return enriched, nil
}

// resolveName: dictionary → synonym DB → PubChem name search
func (r *entityResolverImpl) resolveName(ctx context.Context, name string, eType ChemicalEntityType) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{}

	// 1. Dictionary
	if smiles, ok := r.dictionary.LookupName(name); ok {
		res.CanonicalSMILES = smiles
		if eType == EntityIUPACName {
			res.IUPACName = name
		} else {
			res.CommonName = name
		}
		res.ResolutionPath = "dictionary"
		return r.enrichFromSMILES(ctx, res)
	}

	// 2. Synonym DB
	if r.synonymDB != nil {
		canonical, err := r.synonymDB.FindCanonicalName(name)
		if err == nil && canonical != "" {
			if smiles, ok := r.dictionary.LookupName(canonical); ok {
				res.CanonicalSMILES = smiles
				res.CommonName = canonical
				res.ResolutionPath = "synonym_db"
				return r.enrichFromSMILES(ctx, res)
			}
		}
	}

	// 3. PubChem
	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByName(c, name)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionPath = "pubchem_name"
			return r.enrichFromSMILES(ctx, res)
		}
		if err != nil {
			r.logger.Warn("pubchem name lookup failed", "name", name, "error", err)
		}
	}

	res.IsResolved = false
	res.ResolutionPath = "not_found"
	return res, nil
}

// resolveMolecularFormula: validate format → PubChem → mark ambiguous
func (r *entityResolverImpl) resolveMolecularFormula(ctx context.Context, formula string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{MolecularFormula: formula}

	if !isValidMolecularFormula(formula) {
		res.IsResolved = false
		res.ResolutionPath = "invalid_formula"
		return res, nil
	}

	// PubChem search — may return multiple compounds
	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByName(c, formula)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionPath = "pubchem_formula"
			res.IsAmbiguous = true
			res.AmbiguityNote = fmt.Sprintf(
				"molecular formula %s does not uniquely identify a compound; "+
					"the first PubChem result (CID %d) was used", formula, compound.CID)
			enriched, _ := r.enrichFromSMILES(ctx, res)
			return enriched, nil
		}
	}

	// Partial resolution — we know the formula but nothing else
	res.IsResolved = false
	res.IsAmbiguous = true
	res.AmbiguityNote = fmt.Sprintf("molecular formula %s is ambiguous and could not be resolved to a single compound", formula)
	res.ResolutionPath = "formula_ambiguous"
	return res, nil
}

// resolveInChI: validate → InChIKey → PubChem
func (r *entityResolverImpl) resolveInChI(ctx context.Context, inchi string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{InChI: inchi}

	if !strings.HasPrefix(inchi, "InChI=") {
		res.IsResolved = false
		res.ResolutionPath = "invalid_inchi"
		return res, nil
	}

	// Try to derive InChIKey via RDKit (InChI → SMILES → InChIKey)
	// In practice this would be a direct InChI→InChIKey call; we approximate.
	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByName(c, inchi)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionPath = "pubchem_inchi"
			return r.enrichFromSMILES(ctx, res)
		}
	}

	res.IsResolved = false
	res.ResolutionPath = "inchi_not_resolved"
	return res, nil
}

// resolveGenericStructure: extract constraints, mark unresolvable
func (r *entityResolverImpl) resolveGenericStructure(_ context.Context, text string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{
		IsResolved:     false,
		ResolutionPath: "generic_structure",
	}
	res.Constraints = extractStructureConstraints(text)
	return res, nil
}

// resolveMarkush: mark unresolvable
func (r *entityResolverImpl) resolveMarkush(_ context.Context, text string) (*ResolvedChemicalEntity, error) {
	return &ResolvedChemicalEntity{
		IsResolved:     false,
		ResolutionPath: "markush_variable",
		Constraints:    []string{fmt.Sprintf("variable: %s", text)},
	}, nil
}

// resolveBrandName: brand → common name → resolve as name
func (r *entityResolverImpl) resolveBrandName(ctx context.Context, brand string) (*ResolvedChemicalEntity, error) {
	// Dictionary brand lookup
	if commonName, ok := r.dictionary.LookupBrand(brand); ok {
		res, err := r.resolveName(ctx, commonName, EntityCommonName)
		if err != nil {
			return nil, err
		}
		if res.ResolutionPath != "" {
			res.ResolutionPath = "brand_to_" + res.ResolutionPath
		}
		return res, nil
	}

	// Synonym DB
	if r.synonymDB != nil {
		canonical, err := r.synonymDB.FindCanonicalName(brand)
		if err == nil && canonical != "" {
			res, err := r.resolveName(ctx, canonical, EntityCommonName)
			if err != nil {
				return nil, err
			}
			res.ResolutionPath = "brand_synonym_to_" + res.ResolutionPath
			return res, nil
		}
	}

	// PubChem as last resort
	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByName(c, brand)
		})
		if err == nil && compound != nil {
			res := &ResolvedChemicalEntity{}
			r.applyPubChemCompound(res, compound)
			res.ResolutionPath = "brand_pubchem"
			return r.enrichFromSMILES(ctx, res)
		}
	}

	return &ResolvedChemicalEntity{
		IsResolved:     false,
		ResolutionPath: "brand_not_found",
	}, nil
}

// resolvePolymerOrBiological: not resolvable to small-molecule SMILES
func (r *entityResolverImpl) resolvePolymerOrBiological(_ context.Context, text string, eType ChemicalEntityType) (*ResolvedChemicalEntity, error) {
	note := "polymers cannot be represented as a single SMILES"
	if eType == EntityBiological {
		note = "biological entities (e.g. proteins) cannot be represented as a single small-molecule SMILES"
	}
	return &ResolvedChemicalEntity{
		CommonName:     text,
		IsResolved:     false,
		ResolutionPath: string(eType),
		AmbiguityNote:  note,
	}, nil
}

// ---------------------------------------------------------------------------
// Enrichment & helpers
// ---------------------------------------------------------------------------

// enrichFromSMILES uses RDKitService to compute InChI, InChIKey, formula, MW.
// If RDKit is unavailable the entity is still marked resolved (with SMILES only).
func (r *entityResolverImpl) enrichFromSMILES(ctx context.Context, res *ResolvedChemicalEntity) (*ResolvedChemicalEntity, error) {
	if res.CanonicalSMILES == "" {
		return res, nil
	}
	res.IsResolved = true

	if r.rdkitService == nil {
		return res, nil
	}

	// Canonicalize if not already done
	if canonical, err := r.rdkitService.CanonicalizeSMILES(res.CanonicalSMILES); err == nil {
		res.CanonicalSMILES = canonical
	}

	if inchi, err := r.rdkitService.SMILESToInChI(res.CanonicalSMILES); err == nil {
		res.InChI = inchi
	} else {
		r.logger.Debug("SMILESToInChI failed", "smiles", res.CanonicalSMILES, "error", err)
	}

	if inchiKey, err := r.rdkitService.SMILESToInChIKey(res.CanonicalSMILES); err == nil {
		res.InChIKey = inchiKey
	} else {
		r.logger.Debug("SMILESToInChIKey failed", "smiles", res.CanonicalSMILES, "error", err)
	}

	if formula, err := r.rdkitService.SMILESToMolecularFormula(res.CanonicalSMILES); err == nil {
		res.MolecularFormula = formula
	} else {
		r.logger.Debug("SMILESToMolecularFormula failed", "smiles", res.CanonicalSMILES, "error", err)
	}

	if mw, err := r.rdkitService.ComputeMolecularWeight(res.CanonicalSMILES); err == nil {
		res.MolecularWeight = mw
	} else {
		r.logger.Debug("ComputeMolecularWeight failed", "smiles", res.CanonicalSMILES, "error", err)
	}

	return res, nil
}

// applyPubChemCompound copies PubChem fields into a ResolvedChemicalEntity.
func (r *entityResolverImpl) applyPubChemCompound(res *ResolvedChemicalEntity, c *PubChemCompound) {
	if c == nil {
		return
	}
	res.PubChemCID = c.CID
	if res.CanonicalSMILES == "" {
		res.CanonicalSMILES = c.CanonicalSMILES
	}
	if res.IsomericSMILES == "" {
		res.IsomericSMILES = c.IsomericSMILES
	}
	if res.InChI == "" {
		res.InChI = c.InChI
	}
	if res.InChIKey == "" {
		res.InChIKey = c.InChIKey
	}
	if res.MolecularFormula == "" {
		res.MolecularFormula = c.MolecularFormula
	}
	if res.MolecularWeight == 0 {
		res.MolecularWeight = c.MolecularWeight
	}
	if res.IUPACName == "" {
		res.IUPACName = c.IUPACName
	}
	res.Synonyms = r.truncateSynonyms(c.Synonyms)
}

// pubchemLookup wraps a PubChem call with rate-limiting and timeout.
func (r *entityResolverImpl) pubchemLookup(
	ctx context.Context,
	fn func(context.Context) (*PubChemCompound, error),
) (*PubChemCompound, error) {
	// Rate limit
	if err := r.limiter.Acquire(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	// Timeout
	timeout := r.config.ExternalLookupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return fn(tctx)
}

// truncateSynonyms caps the synonym list at MaxSynonyms.
func (r *entityResolverImpl) truncateSynonyms(syns []string) []string {
	max := r.config.MaxSynonyms
	if max <= 0 {
		max = 20
	}
	if len(syns) <= max {
		return syns
	}
	return syns[:max]
}

// cacheKey builds a deterministic cache key from text + type.
func (r *entityResolverImpl) cacheKey(text string, eType ChemicalEntityType) string {
	return fmt.Sprintf("%s::%s", eType, strings.ToLower(strings.TrimSpace(text)))
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

var molecularFormulaRe = regexp.MustCompile(`^([A-Z][a-z]?\d*)+$`)

func isValidMolecularFormula(formula string) bool {
	return molecularFormulaRe.MatchString(strings.TrimSpace(formula))
}

// extractStructureConstraints attempts to pull out range constraints from
// generic structure descriptions such as "C1-C6 alkyl", "C1-C20 aryl".
var rangeConstraintRe = regexp.MustCompile(`([A-Z][a-z]?)(\d+)\s*-\s*([A-Z][a-z]?)(\d+)`)

func extractStructureConstraints(text string) []string {
	var constraints []string
	matches := rangeConstraintRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) >= 5 {
			constraints = append(constraints, fmt.Sprintf("%s%s-%s%s", m[1], m[2], m[3], m[4]))
		}
	}
	// Also capture free-text tokens that look like variable definitions
	if strings.Contains(strings.ToLower(text), "alkyl") {
		constraints = append(constraints, "group_type:alkyl")
	}
	if strings.Contains(strings.ToLower(text), "aryl") {
		constraints = append(constraints, "group_type:aryl")
	}
	if strings.Contains(strings.ToLower(text), "heteroaryl") {
		constraints = append(constraints, "group_type:heteroaryl")
	}
	if strings.Contains(strings.ToLower(text), "cycloalkyl") {
		constraints = append(constraints, "group_type:cycloalkyl")
	}
	if strings.Contains(strings.ToLower(text), "halogen") || strings.Contains(strings.ToLower(text), "halo") {
		constraints = append(constraints, "group_type:halogen")
	}
	if len(constraints) == 0 {
		constraints = append(constraints, fmt.Sprintf("raw:%s", text))
	}
	return constraints
}

//Personal.AI order the ending
