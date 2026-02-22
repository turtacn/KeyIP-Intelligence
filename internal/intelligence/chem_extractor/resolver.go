package chem_extractor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"golang.org/x/sync/errgroup"
)

// ResolverConfig holds configuration for the resolver.
type ResolverConfig struct {
	CacheEnabled          bool          `json:"cache_enabled"`
	CacheTTL              time.Duration `json:"cache_ttl"`
	ExternalLookupEnabled bool          `json:"external_lookup_enabled"`
	ExternalLookupTimeout time.Duration `json:"external_lookup_timeout"`
	MaxSynonyms           int           `json:"max_synonyms"`
}

// PubChemCompound represents a compound from PubChem.
type PubChemCompound struct {
	CID              int
	CanonicalSMILES  string
	IsomericSMILES   string
	InChI            string
	InChIKey         string
	MolecularFormula string
	MolecularWeight  float64
	IUPACName        string
	Synonyms         []string
}

// PubChemClient defines the interface for PubChem API.
type PubChemClient interface {
	SearchByName(ctx context.Context, name string) (*PubChemCompound, error)
	SearchByCAS(ctx context.Context, cas string) (*PubChemCompound, error)
	SearchBySMILES(ctx context.Context, smiles string) (*PubChemCompound, error)
	GetCompound(ctx context.Context, cid int) (*PubChemCompound, error)
}

// RDKitService defines the interface for RDKit operations.
type RDKitService interface {
	ValidateSMILES(smiles string) (bool, error)
	CanonicalizeSMILES(smiles string) (string, error)
	SMILESToInChI(smiles string) (string, error)
	SMILESToMolecularFormula(smiles string) (string, error)
	ComputeMolecularWeight(smiles string) (float64, error)
	SMILESToInChIKey(smiles string) (string, error)
}

// SynonymDatabase defines the interface for synonym lookup.
type SynonymDatabase interface {
	FindSynonyms(name string) ([]string, error)
	FindCanonicalName(name string) (string, error)
}

// ResolverCache defines the interface for caching.
type ResolverCache interface {
	Get(key string) (*ResolvedChemicalEntity, bool)
	Set(key string, entity *ResolvedChemicalEntity)
}

// EntityResolver defines the interface for resolving chemical entities.
type EntityResolver interface {
	Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error)
	ResolveBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ResolvedChemicalEntity, error)
	ResolveByType(ctx context.Context, text string, entityType ChemicalEntityType) (*ResolvedChemicalEntity, error)
}

// entityResolverImpl implements EntityResolver.
type entityResolverImpl struct {
	pubchem PubChemClient
	rdkit   RDKitService
	synonyms SynonymDatabase
	cache   ResolverCache
	config  ResolverConfig
	logger  logging.Logger
}

// NewEntityResolver creates a new EntityResolver.
func NewEntityResolver(
	pubchem PubChemClient,
	rdkit RDKitService,
	synonyms SynonymDatabase,
	cache ResolverCache,
	config ResolverConfig,
	logger logging.Logger,
) EntityResolver {
	return &entityResolverImpl{
		pubchem:  pubchem,
		rdkit:    rdkit,
		synonyms: synonyms,
		cache:    cache,
		config:   config,
		logger:   logger,
	}
}

func (r *entityResolverImpl) Resolve(ctx context.Context, entity *RawChemicalEntity) (*ResolvedChemicalEntity, error) {
	return r.ResolveByType(ctx, entity.Text, entity.EntityType)
}

func (r *entityResolverImpl) ResolveBatch(ctx context.Context, entities []*RawChemicalEntity) ([]*ResolvedChemicalEntity, error) {
	results := make([]*ResolvedChemicalEntity, len(entities))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(10) // Concurrency limit

	for i, entity := range entities {
		i, entity := i, entity
		g.Go(func() error {
			res, err := r.Resolve(ctx, entity)
			if err != nil {
				// Log error but don't fail batch?
				r.logger.Warn("Failed to resolve entity", logging.String("text", entity.Text), logging.Err(err))
				results[i] = &ResolvedChemicalEntity{OriginalEntity: entity, IsResolved: false}
				return nil
			}
			results[i] = res
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func (r *entityResolverImpl) ResolveByType(ctx context.Context, text string, entityType ChemicalEntityType) (*ResolvedChemicalEntity, error) {
	// Check cache
	cacheKey := fmt.Sprintf("%s:%s", entityType, text)
	if r.config.CacheEnabled && r.cache != nil {
		if res, ok := r.cache.Get(cacheKey); ok {
			return res, nil
		}
	}

	var result *ResolvedChemicalEntity
	var err error

	switch entityType {
	case EntityCASNumber:
		result, err = r.resolveCAS(ctx, text)
	case EntitySMILES:
		result, err = r.resolveSMILES(ctx, text)
	case EntityIUPACName, EntityCommonName, EntityBrandName:
		result, err = r.resolveName(ctx, text)
	case EntityInChI:
		result, err = r.resolveInChI(ctx, text)
	case EntityMolecularFormula:
		result, err = r.resolveFormula(ctx, text)
	default:
		// Not resolvable to structure
		result = &ResolvedChemicalEntity{
			OriginalEntity: &RawChemicalEntity{Text: text, EntityType: entityType},
			IsResolved:     false,
		}
	}

	if err != nil {
		// Return partial result with error?
		return nil, err
	}

	// Update cache
	if r.config.CacheEnabled && r.cache != nil && result != nil && result.IsResolved {
		r.cache.Set(cacheKey, result)
	}

	return result, nil
}

func (r *entityResolverImpl) resolveCAS(ctx context.Context, cas string) (*ResolvedChemicalEntity, error) {
	if !r.config.ExternalLookupEnabled {
		return &ResolvedChemicalEntity{IsResolved: false}, nil
	}

	// PubChem Lookup
	pc, err := r.pubchem.SearchByCAS(ctx, cas)
	if err != nil {
		return nil, err
	}
	return r.convertPubChem(pc), nil
}

func (r *entityResolverImpl) resolveSMILES(ctx context.Context, smiles string) (*ResolvedChemicalEntity, error) {
	if r.rdkit == nil {
		return nil, errors.New("rdkit service unavailable")
	}

	valid, err := r.rdkit.ValidateSMILES(smiles)
	if err != nil || !valid {
		return &ResolvedChemicalEntity{IsResolved: false}, nil
	}

	canonical, _ := r.rdkit.CanonicalizeSMILES(smiles)
	inchi, _ := r.rdkit.SMILESToInChI(canonical)
	key, _ := r.rdkit.SMILESToInChIKey(canonical)
	mw, _ := r.rdkit.ComputeMolecularWeight(canonical)
	formula, _ := r.rdkit.SMILESToMolecularFormula(canonical)

	// Lookup name from PubChem via InChIKey if enabled
	name := ""
	if r.config.ExternalLookupEnabled {
		// Use SearchBySMILES or InChIKey logic if available
		// Simplified:
	}

	return &ResolvedChemicalEntity{
		SMILES:           canonical,
		InChI:            inchi,
		InChIKey:         key,
		MolecularWeight:  mw,
		MolecularFormula: formula,
		CanonicalName:    name,
		IsResolved:       true,
		ResolutionMethod: "RDKit",
	}, nil
}

func (r *entityResolverImpl) resolveName(ctx context.Context, name string) (*ResolvedChemicalEntity, error) {
	// Try synonyms DB first
	if r.synonyms != nil {
		// ...
	}

	if !r.config.ExternalLookupEnabled {
		return &ResolvedChemicalEntity{IsResolved: false}, nil
	}

	pc, err := r.pubchem.SearchByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return r.convertPubChem(pc), nil
}

func (r *entityResolverImpl) resolveInChI(ctx context.Context, inchi string) (*ResolvedChemicalEntity, error) {
	// ... logic to convert InChI to SMILES etc via RDKit
	return &ResolvedChemicalEntity{InChI: inchi, IsResolved: true}, nil
}

func (r *entityResolverImpl) resolveFormula(ctx context.Context, formula string) (*ResolvedChemicalEntity, error) {
	// Ambiguous
	return &ResolvedChemicalEntity{MolecularFormula: formula, IsResolved: false}, nil // Marked as partial
}

func (r *entityResolverImpl) convertPubChem(pc *PubChemCompound) *ResolvedChemicalEntity {
	return &ResolvedChemicalEntity{
		SMILES:           pc.CanonicalSMILES,
		InChI:            pc.InChI,
		InChIKey:         pc.InChIKey,
		MolecularFormula: pc.MolecularFormula,
		MolecularWeight:  pc.MolecularWeight,
		CanonicalName:    pc.IUPACName,
		CASNumber:        "", // PubChem doesn't always have CAS
		Synonyms:         pc.Synonyms,
		IsResolved:       true,
		ResolutionMethod: "PubChem",
	}
}

// In-memory cache implementation for testing
type InMemoryResolverCache struct {
	data sync.Map
}

func (c *InMemoryResolverCache) Get(key string) (*ResolvedChemicalEntity, bool) {
	val, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}
	return val.(*ResolvedChemicalEntity), true
}

func (c *InMemoryResolverCache) Set(key string, entity *ResolvedChemicalEntity) {
	c.data.Store(key, entity)
}

//Personal.AI order the ending
