// Package datasource defines normalized data models and interfaces for
// external patent and molecule data providers.
//
// Architecture:
//   - PatentDataSource / MoleculeDataSource are the core interfaces.
//   - Open-source implementations (PubChem, EPO OPS, USPTO) for dev/free tier.
//   - Paid implementations (Derwent, CAS, PatSnap) implement the same interfaces.
//   - DataSourceRegistry holds all configured sources; workers iterate them.
package datasource

import (
	"context"
	"time"
)

// PatentRecord is a normalized patent record from any data source.
type PatentRecord struct {
	SourceID        string    `json:"source_id"`
	PatentNumber    string    `json:"patent_number"`
	Title           string    `json:"title"`
	Abstract        string    `json:"abstract"`
	FilingDate      string    `json:"filing_date"`
	PublicationDate string    `json:"publication_date"`
	GrantDate       string    `json:"grant_date,omitempty"`
	LegalStatus     string    `json:"legal_status"`
	Assignee        string    `json:"assignee"`
	Inventors       []string  `json:"inventors"`
	IPCCodes        []string  `json:"ipc_codes"`
	CPCCodes        []string  `json:"cpc_codes"`
	Jurisdiction    string    `json:"jurisdiction"`
	Claims          []string  `json:"claims"`
	FamilyID        string    `json:"family_id,omitempty"`
	SourceName      string    `json:"source_name"`
	FetchedAt       time.Time `json:"fetched_at"`
	RawJSON         []byte    `json:"-"`
}

// MoleculeRecord is a normalized molecule record from any data source.
type MoleculeRecord struct {
	SourceID         string    `json:"source_id"`
	Name             string    `json:"name"`
	SMILES           string    `json:"smiles"`
	CanonicalSMILES  string    `json:"canonical_smiles,omitempty"`
	InChI            string    `json:"inchi,omitempty"`
	InChIKey         string    `json:"inchi_key,omitempty"`
	MolecularFormula string    `json:"molecular_formula"`
	MolecularWeight  float64   `json:"molecular_weight"`
	ExactMass        float64   `json:"exact_mass,omitempty"`
	LogP             float64   `json:"logp,omitempty"`
	TPSA             float64   `json:"tpsa,omitempty"`
	Synonyms         []string  `json:"synonyms,omitempty"`
	SourceName       string    `json:"source_name"`
	FetchedAt        time.Time `json:"fetched_at"`
}

// PatentDataSource defines the interface for patent data sources.
//
// Open-source implementations: EPO OPS, USPTO PatentsView, WIPO PATENTSCOPE, Lens.org
// Paid implementations: Derwent Innovation, CAS STN, PatSnap, Questel Orbit
type PatentDataSource interface {
	// Name returns the human-readable source name (e.g. "EPO OPS", "USPTO").
	Name() string
	// IsEnabled returns whether this source is configured and available.
	IsEnabled() bool
	// SearchPatents searches for patents by query string.
	SearchPatents(ctx context.Context, query string, maxResults int) ([]PatentRecord, error)
	// GetPatent retrieves a single patent by its number.
	GetPatent(ctx context.Context, patentNumber string) (*PatentRecord, error)
	// FetchByDateRange fetches patents within a date range (batch sync).
	FetchByDateRange(ctx context.Context, from, to time.Time, maxResults int) ([]PatentRecord, error)
	// RateLimit returns the configured rate limit for this source.
	RateLimit() int // requests per second
}

// MoleculeDataSource defines the interface for molecule/chemical data sources.
//
// Open-source: PubChem PUG REST, ChEMBL, ChEBI
// Paid: CAS SciFinder-n, Reaxys, GOSTAR
type MoleculeDataSource interface {
	Name() string
	IsEnabled() bool
	SearchMolecules(ctx context.Context, query string, maxResults int) ([]MoleculeRecord, error)
	// GetMolecule retrieves by CID, SMILES, or InChIKey.
	GetMolecule(ctx context.Context, identifier string) (*MoleculeRecord, error)
	GetBySMILES(ctx context.Context, smiles string) (*MoleculeRecord, error)
	RateLimit() int
}

// DataSourceRegistry holds all configured data sources.
type DataSourceRegistry struct {
	PatentSources   []PatentDataSource
	MoleculeSources []MoleculeDataSource
}

// NewRegistry creates an empty registry.
func NewRegistry() *DataSourceRegistry {
	return &DataSourceRegistry{
		PatentSources:   make([]PatentDataSource, 0),
		MoleculeSources: make([]MoleculeDataSource, 0),
	}
}

// AddPatent registers a patent data source.
func (r *DataSourceRegistry) AddPatent(src PatentDataSource) {
	r.PatentSources = append(r.PatentSources, src)
}

// AddMolecule registers a molecule data source.
func (r *DataSourceRegistry) AddMolecule(src MoleculeDataSource) {
	r.MoleculeSources = append(r.MoleculeSources, src)
}
