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

// PubChemClient abstracts the PubChem REST API.
type PubChemClient interface {
	SearchByName(ctx context.Context, name string) (*PubChemCompound, error)
	SearchByCAS(ctx context.Context, cas string) (*PubChemCompound, error)
	SearchBySMILES(ctx context.Context, smiles string) (*PubChemCompound, error)
	GetCompound(ctx context.Context, cid int) (*PubChemCompound, error)
}

// RDKitService abstracts cheminformatics operations.
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

// InMemoryDictionary implements ChemicalDictionary interface.
type InMemoryDictionary struct {
	nameToSMILES map[string]string // lower-case name → SMILES
	casToSMILES  map[string]string // CAS number → SMILES
	brandToName  map[string]string // lower-case brand → lower-case common name
	mu           sync.RWMutex
}

// NewInMemoryDictionary creates an empty dictionary.
func NewInMemoryDictionary() *InMemoryDictionary {
	return &InMemoryDictionary{
		nameToSMILES: make(map[string]string),
		casToSMILES:  make(map[string]string),
		brandToName:  make(map[string]string),
	}
}

// AddName registers a name → SMILES mapping.
func (d *InMemoryDictionary) AddName(name, smiles string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nameToSMILES[strings.ToLower(strings.TrimSpace(name))] = smiles
}

// AddCAS registers a CAS → SMILES mapping.
func (d *InMemoryDictionary) AddCAS(cas, smiles string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.casToSMILES[strings.TrimSpace(cas)] = smiles
}

// AddBrand registers a brand → common-name mapping.
func (d *InMemoryDictionary) AddBrand(brand, commonName string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.brandToName[strings.ToLower(strings.TrimSpace(brand))] = strings.ToLower(strings.TrimSpace(commonName))
}

// Lookup implements ChemicalDictionary.Lookup.
func (d *InMemoryDictionary) Lookup(name string) (*DictionaryEntry, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.nameToSMILES[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, false
	}
	return &DictionaryEntry{
		CanonicalName: name,
		SMILES:        s,
		EntityType:    EntityCommonName,
	}, true
}

// LookupCAS implements ChemicalDictionary.LookupCAS.
func (d *InMemoryDictionary) LookupCAS(cas string) (*DictionaryEntry, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	s, ok := d.casToSMILES[strings.TrimSpace(cas)]
	if !ok {
		return nil, false
	}
	return &DictionaryEntry{
		CanonicalName: cas,
		CASNumber:     cas,
		SMILES:        s,
		EntityType:    EntityCASNumber,
	}, true
}

// LookupBrand implements ChemicalDictionary.LookupBrand.
func (d *InMemoryDictionary) LookupBrand(brand string) (*DictionaryEntry, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n, ok := d.brandToName[strings.ToLower(strings.TrimSpace(brand))]
	if !ok {
		return nil, false
	}
	s, hasSMILES := d.nameToSMILES[n]
	entry := &DictionaryEntry{
		CanonicalName: n,
		EntityType:    EntityBrandName,
		Synonyms:      []string{brand},
	}
	if hasSMILES {
		entry.SMILES = s
	}
	return entry, true
}

// Size implements ChemicalDictionary.Size.
func (d *InMemoryDictionary) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.nameToSMILES) + len(d.casToSMILES) + len(d.brandToName)
}

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

// Noop logger


// Rate limiter
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
	for i := 0; i < rps; i++ {
		rl.tokens <- struct{}{}
	}
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

const pubchemRPS = 5

type entityResolverImpl struct {
	dictionary    ChemicalDictionary
	pubchemClient PubChemClient
	rdkitService  RDKitService
	synonymDB     SynonymDatabase
	cache         ResolverCache
	config        ResolverConfig
	logger        Logger
	limiter       *rateLimiter
}

// NewEntityResolver creates a production EntityResolver.
func NewEntityResolver(
	dictionary ChemicalDictionary,
	pubchemClient PubChemClient,
	rdkitService RDKitService,
	synonymDB SynonymDatabase,
	cache ResolverCache,
	config ResolverConfig,
	logger Logger,
) EntityResolver {
	if dictionary == nil {
		dictionary = NewInMemoryDictionary()
	}
	if logger == nil {
		logger = &noopLogger{}
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

func (r *entityResolverImpl) Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
	if entity == nil {
		return nil, errors.NewInvalidInputError("entity is nil")
	}
	text := strings.TrimSpace(entity.Text)
	if text == "" {
		return nil, errors.NewInvalidInputError("entity text is empty")
	}

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

	resolved.OriginalEntity = entity

	if r.config.CacheEnabled && r.cache != nil && resolved.IsResolved {
		r.cache.Set(cacheKey, resolved)
	}

	return resolved, nil
}

func (r *entityResolverImpl) resolveCAS(ctx context.Context, cas string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{CASNumber: cas}

	if entry, ok := r.dictionary.LookupCAS(cas); ok {
		res.SMILES = entry.SMILES
		res.ResolutionMethod = "dictionary"
		return r.enrichFromSMILES(ctx, res)
	}

	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByCAS(c, cas)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionMethod = "pubchem_cas"
			return r.enrichFromSMILES(ctx, res)
		}
	}

	res.IsResolved = false
	res.ResolutionMethod = "not_found"
	return res, nil
}

func (r *entityResolverImpl) resolveSMILES(ctx context.Context, smiles string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{}

	if r.rdkitService == nil {
		res.SMILES = smiles
		res.IsResolved = true
		res.ResolutionMethod = "raw_smiles_no_rdkit"
		return res, nil
	}

	valid, err := r.rdkitService.ValidateSMILES(smiles)
	if err != nil {
		res.SMILES = smiles
		res.IsResolved = false
		res.ResolutionMethod = "rdkit_unavailable"
		return res, nil
	}
	if !valid {
		res.IsResolved = false
		res.ResolutionMethod = "invalid_smiles"
		return res, nil
	}

	canonical, err := r.rdkitService.CanonicalizeSMILES(smiles)
	if err != nil {
		canonical = smiles
	}
	res.SMILES = canonical
	res.ResolutionMethod = "smiles_direct"
	return r.enrichFromSMILES(ctx, res)
}

func (r *entityResolverImpl) resolveName(ctx context.Context, name string, eType ChemicalEntityType) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{CanonicalName: name}

	if entry, ok := r.dictionary.Lookup(name); ok {
		res.SMILES = entry.SMILES
		res.ResolutionMethod = "dictionary"
		return r.enrichFromSMILES(ctx, res)
	}

	if r.synonymDB != nil {
		canonical, err := r.synonymDB.FindCanonicalName(name)
		if err == nil && canonical != "" {
			if entry, ok := r.dictionary.Lookup(canonical); ok {
				res.SMILES = entry.SMILES
				res.CanonicalName = canonical
				res.ResolutionMethod = "synonym_db"
				return r.enrichFromSMILES(ctx, res)
			}
		}
	}

	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByName(c, name)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionMethod = "pubchem_name"
			return r.enrichFromSMILES(ctx, res)
		}
	}

	res.IsResolved = false
	res.ResolutionMethod = "not_found"
	return res, nil
}

func (r *entityResolverImpl) resolveMolecularFormula(ctx context.Context, formula string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{MolecularFormula: formula}

	if !isValidMolecularFormula(formula) {
		res.IsResolved = false
		res.ResolutionMethod = "invalid_formula"
		return res, nil
	}

	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByName(c, formula)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionMethod = "pubchem_formula"
			res.CanonicalName = fmt.Sprintf("Ambiguous from formula %s", formula)
			enriched, _ := r.enrichFromSMILES(ctx, res)
			return enriched, nil
		}
	}

	res.IsResolved = false
	res.ResolutionMethod = "formula_ambiguous"
	return res, nil
}

func (r *entityResolverImpl) resolveInChI(ctx context.Context, inchi string) (*ResolvedChemicalEntity, error) {
	res := &ResolvedChemicalEntity{InChI: inchi}

	if !strings.HasPrefix(inchi, "InChI=") {
		res.IsResolved = false
		res.ResolutionMethod = "invalid_inchi"
		return res, nil
	}

	if r.config.ExternalLookupEnabled && r.pubchemClient != nil {
		compound, err := r.pubchemLookup(ctx, func(c context.Context) (*PubChemCompound, error) {
			return r.pubchemClient.SearchByName(c, inchi)
		})
		if err == nil && compound != nil {
			r.applyPubChemCompound(res, compound)
			res.ResolutionMethod = "pubchem_inchi"
			return r.enrichFromSMILES(ctx, res)
		}
	}

	res.IsResolved = false
	res.ResolutionMethod = "inchi_not_resolved"
	return res, nil
}

func (r *entityResolverImpl) resolveGenericStructure(_ context.Context, text string) (*ResolvedChemicalEntity, error) {
	return &ResolvedChemicalEntity{
		IsResolved:     false,
		ResolutionMethod: "generic_structure",
		CanonicalName: text,
	}, nil
}

func (r *entityResolverImpl) resolveMarkush(_ context.Context, text string) (*ResolvedChemicalEntity, error) {
	return &ResolvedChemicalEntity{
		IsResolved:     false,
		ResolutionMethod: "markush_variable",
		CanonicalName: fmt.Sprintf("variable: %s", text),
	}, nil
}

func (r *entityResolverImpl) resolveBrandName(ctx context.Context, brand string) (*ResolvedChemicalEntity, error) {
	if entry, ok := r.dictionary.LookupBrand(brand); ok {
		res, err := r.resolveName(ctx, entry.CanonicalName, EntityCommonName)
		if err != nil {
			return nil, err
		}
		if res.ResolutionMethod != "" {
			res.ResolutionMethod = "brand_to_" + res.ResolutionMethod
		}
		return res, nil
	}

	return &ResolvedChemicalEntity{
		IsResolved:     false,
		ResolutionMethod: "brand_not_found",
	}, nil
}

func (r *entityResolverImpl) resolvePolymerOrBiological(_ context.Context, text string, eType ChemicalEntityType) (*ResolvedChemicalEntity, error) {
	return &ResolvedChemicalEntity{
		CanonicalName:     text,
		IsResolved:     false,
		ResolutionMethod: string(eType),
	}, nil
}

func (r *entityResolverImpl) enrichFromSMILES(ctx context.Context, res *ResolvedChemicalEntity) (*ResolvedChemicalEntity, error) {
	if res.SMILES == "" {
		return res, nil
	}
	res.IsResolved = true

	if r.rdkitService == nil {
		return res, nil
	}

	if canonical, err := r.rdkitService.CanonicalizeSMILES(res.SMILES); err == nil {
		res.SMILES = canonical
	}

	if inchi, err := r.rdkitService.SMILESToInChI(res.SMILES); err == nil {
		res.InChI = inchi
	}

	if inchiKey, err := r.rdkitService.SMILESToInChIKey(res.SMILES); err == nil {
		res.InChIKey = inchiKey
	}

	if formula, err := r.rdkitService.SMILESToMolecularFormula(res.SMILES); err == nil {
		res.MolecularFormula = formula
	}

	if mw, err := r.rdkitService.ComputeMolecularWeight(res.SMILES); err == nil {
		res.MolecularWeight = mw
	}

	return res, nil
}

func (r *entityResolverImpl) applyPubChemCompound(res *ResolvedChemicalEntity, c *PubChemCompound) {
	if c == nil {
		return
	}
	// res.PubChemCID = c.CID // Field not in extractor.go struct? It was in resolver.go version.
	// extractor.go struct doesn't have PubChemCID.

	if res.SMILES == "" {
		res.SMILES = c.CanonicalSMILES
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
	if res.CanonicalName == "" {
		res.CanonicalName = c.IUPACName
	}
	res.Synonyms = r.truncateSynonyms(c.Synonyms)
}

func (r *entityResolverImpl) pubchemLookup(
	ctx context.Context,
	fn func(context.Context) (*PubChemCompound, error),
) (*PubChemCompound, error) {
	if err := r.limiter.Acquire(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter: %w", err)
	}

	timeout := r.config.ExternalLookupTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return fn(tctx)
}

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

func (r *entityResolverImpl) cacheKey(text string, eType ChemicalEntityType) string {
	return fmt.Sprintf("%s::%s", eType, strings.ToLower(strings.TrimSpace(text)))
}

var molecularFormulaRe = regexp.MustCompile(`^([A-Z][a-z]?\d*)+$`)

func isValidMolecularFormula(formula string) bool {
	return molecularFormulaRe.MatchString(strings.TrimSpace(formula))
}

var rangeConstraintRe = regexp.MustCompile(`([A-Z][a-z]?)(\d+)\s*-\s*([A-Z][a-z]?)(\d+)`)

func extractStructureConstraints(text string) []string {
	var constraints []string
	matches := rangeConstraintRe.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) >= 5 {
			constraints = append(constraints, fmt.Sprintf("%s%s-%s%s", m[1], m[2], m[3], m[4]))
		}
	}
	if strings.Contains(strings.ToLower(text), "alkyl") {
		constraints = append(constraints, "group_type:alkyl")
	}
	if strings.Contains(strings.ToLower(text), "aryl") {
		constraints = append(constraints, "group_type:aryl")
	}
	if strings.Contains(strings.ToLower(text), "heteroaryl") {
		constraints = append(constraints, "group_type:heteroaryl")
	}
	if strings.Contains(strings.ToLower(text), "halogen") {
		constraints = append(constraints, "group_type:halogen")
	}
	if len(constraints) == 0 {
		constraints = append(constraints, fmt.Sprintf("raw:%s", text))
	}
	return constraints
}

func (r *entityResolverImpl) ResolveBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ResolvedChemicalEntity, error) {
	if len(entities) == 0 {
		return []*ResolvedChemicalEntity{}, nil
	}

	results := make([]*ResolvedChemicalEntity, len(entities))
	errs := make([]error, len(entities))

	concurrency := r.config.BatchConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, ent := range entities {
		wg.Add(1)
		go func(idx int, e *RawChemicalEntity) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := r.Resolve(ctx, e)
			results[idx] = res
			errs[idx] = err
		}(i, ent)
	}
	wg.Wait()

	// If all failed, return error? Or partial results?
	// The test expects partial success.
	// We will return the results as is. The caller can check IsResolved.
	return results, nil
}

func (r *entityResolverImpl) ResolveByType(ctx context.Context, text string, entityType ChemicalEntityType) (*ResolvedChemicalEntity, error) {
	entity := &RawChemicalEntity{
		Text:       text,
		EntityType: entityType,
		Confidence: 1.0, // Assumption for manual resolution
	}
	return r.Resolve(ctx, entity)
}
