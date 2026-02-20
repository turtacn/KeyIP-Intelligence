// Package molecule defines all molecule-domain Data Transfer Objects, enumerations,
// and request/response structures used across every layer of the KeyIP-Intelligence
// platform.  No domain logic lives here — only plain data types that are safe to
// import from any layer without creating circular dependencies.
package molecule

import (
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// MoleculeType — classification of a molecule's functional role
// ─────────────────────────────────────────────────────────────────────────────

// MoleculeType categorises a molecule by its primary chemical function within
// the patent corpus that KeyIP-Intelligence analyses.
type MoleculeType string

const (
	// TypeSmallMolecule covers drug-like, agrochemical, and general organic molecules.
	TypeSmallMolecule MoleculeType = "small_molecule"

	// TypePolymer covers macromolecular structures represented by repeating units.
	TypePolymer MoleculeType = "polymer"

	// TypeOLEDMaterial covers organic light-emitting diode host and emitter materials,
	// including thermally activated delayed fluorescence (TADF) compounds.
	TypeOLEDMaterial MoleculeType = "oled_material"

	// TypeCatalyst covers transition-metal complexes and organocatalysts.
	TypeCatalyst MoleculeType = "catalyst"

	// TypeIntermediate covers synthetic intermediates that appear in patent
	// synthesis routes but are not themselves end-use materials.
	TypeIntermediate MoleculeType = "intermediate"
)

// ─────────────────────────────────────────────────────────────────────────────
// FingerprintType — molecular fingerprint algorithm identifier
// ─────────────────────────────────────────────────────────────────────────────

// FingerprintType identifies which fingerprint algorithm was used to generate
// a particular bit-vector or count-vector for a molecule.
type FingerprintType string

const (
	// FPMorgan is the circular Morgan / ECFP fingerprint (default radius 2 → ECFP4).
	FPMorgan FingerprintType = "morgan"

	// FPMACCS is the 166-bit MACCS structural keys fingerprint.
	FPMACCS FingerprintType = "maccs"

	// FPTopological is the RDKit topological (Daylight-style) path fingerprint.
	FPTopological FingerprintType = "topological"

	// FPAtomPair is the atom-pair fingerprint (counts of atom-pair descriptors).
	FPAtomPair FingerprintType = "atom_pair"
)

// ─────────────────────────────────────────────────────────────────────────────
// MolecularProperties — physicochemical and optoelectronic descriptor set
// ─────────────────────────────────────────────────────────────────────────────

// MolecularProperties holds computed physicochemical descriptors for a molecule.
// Fields marked as pointer types are optional and may be nil when the property
// has not been computed or is not applicable to the molecule type.
//
// OLED-specific properties (HOMO, LUMO, BandGap) are computed only for
// TypeOLEDMaterial molecules and remain nil for all other types.
type MolecularProperties struct {
	// LogP is the calculated octanol-water partition coefficient (Crippen method).
	LogP float64 `json:"log_p"`

	// TPSA is the topological polar surface area in Å².
	TPSA float64 `json:"tpsa"`

	// HBondDonors is the number of hydrogen-bond donor groups (NH, OH).
	HBondDonors int `json:"h_bond_donors"`

	// HBondAcceptors is the number of hydrogen-bond acceptor groups (N, O).
	HBondAcceptors int `json:"h_bond_acceptors"`

	// RotatableBonds is the count of non-terminal, non-ring single bonds.
	RotatableBonds int `json:"rotatable_bonds"`

	// AromaticRings is the count of aromatic ring systems in the molecule.
	AromaticRings int `json:"aromatic_rings"`

	// HOMO is the highest occupied molecular orbital energy in eV.
	// Populated only for TypeOLEDMaterial; nil otherwise.
	HOMO *float64 `json:"homo,omitempty"`

	// LUMO is the lowest unoccupied molecular orbital energy in eV.
	// Populated only for TypeOLEDMaterial; nil otherwise.
	LUMO *float64 `json:"lumo,omitempty"`

	// BandGap is the HOMO-LUMO energy gap in eV (BandGap = LUMO − HOMO).
	// Populated only for TypeOLEDMaterial; nil otherwise.
	BandGap *float64 `json:"band_gap,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// MoleculeDTO — cross-layer data transfer object for a molecule
// ─────────────────────────────────────────────────────────────────────────────

// MoleculeDTO is the canonical molecule representation passed between the
// application, interface, and client layers.  It embeds common.BaseEntity so
// that it carries audit metadata (ID, created/updated timestamps, tenant ID)
// without duplicating field definitions.
//
// Fingerprints are stored as raw byte slices keyed by FingerprintType so that
// the transport layer can choose to include or omit them depending on the use
// case (e.g., HTTP responses omit fingerprints; Milvus indexing includes them).
type MoleculeDTO struct {
	// BaseEntity provides ID, CreatedAt, UpdatedAt, and TenantID.
	common.BaseEntity

	// SMILES is the canonical SMILES string (RDKit-canonicalised).
	SMILES string `json:"smiles"`

	// InChI is the IUPAC International Chemical Identifier.
	InChI string `json:"inchi,omitempty"`

	// InChIKey is the 27-character hashed InChI key used as a globally unique
	// identifier for structural de-duplication.
	InChIKey string `json:"inchi_key"`

	// MolecularFormula is the Hill-system molecular formula (e.g., "C12H10N2O").
	MolecularFormula string `json:"molecular_formula"`

	// MolecularWeight is the average molecular weight in g/mol.
	MolecularWeight float64 `json:"molecular_weight"`

	// Name is the preferred IUPAC or trade name for the molecule.
	Name string `json:"name,omitempty"`

	// Synonyms lists alternative names, registry numbers (CAS, PubChem CID, etc.),
	// and trade names.
	Synonyms []string `json:"synonyms,omitempty"`

	// Type classifies the molecule by functional role.
	Type MoleculeType `json:"type"`

	// Properties contains computed physicochemical and optoelectronic descriptors.
	Properties MolecularProperties `json:"properties"`

	// Fingerprints maps each computed fingerprint algorithm to its byte-encoded
	// bit-vector.  Omitted from JSON responses by default; populated internally
	// by the similarity-search and Milvus-indexing pipelines.
	Fingerprints map[FingerprintType][]byte `json:"fingerprints,omitempty"`

	// SourcePatentIDs lists the IDs of patents from which this molecule was
	// extracted by the ChemExtractor NER pipeline.
	SourcePatentIDs []common.ID `json:"source_patent_ids,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Search request / response types
// ─────────────────────────────────────────────────────────────────────────────

// MoleculeSearchRequest is the input DTO for paginated molecule search queries.
// All filter fields are optional pointers; a nil pointer means "no filter on
// this dimension".  At least one of SMILES or Name should be non-nil for
// meaningful results, but the service layer enforces that constraint.
type MoleculeSearchRequest struct {
	// SMILES, when set, triggers a fingerprint-similarity search against the
	// Milvus vector store using the specified FingerprintType.
	SMILES *string `json:"smiles,omitempty"`

	// Name, when set, performs a text search against molecule names and synonyms
	// in OpenSearch.
	Name *string `json:"name,omitempty"`

	// Type, when set, restricts results to molecules of the given classification.
	Type *MoleculeType `json:"type,omitempty"`

	// MinSimilarity is the minimum Tanimoto coefficient (0.0–1.0) required for
	// a molecule to be included in the similarity-search results.
	// Ignored when SMILES is nil.  Defaults to 0.7 in the service layer.
	MinSimilarity *float64 `json:"min_similarity,omitempty"`

	// FingerprintType selects which fingerprint algorithm to use for similarity
	// computation.  Defaults to FPMorgan when nil.
	FingerprintType *FingerprintType `json:"fingerprint_type,omitempty"`

	// PageRequest carries page number and page size for result pagination.
	common.PageRequest
}

// MoleculeSearchResponse is the paginated output DTO for molecule search queries.
// It reuses the generic common.PageResponse wrapper so that pagination metadata
// (total count, current page, page size) is consistent across all search APIs.
type MoleculeSearchResponse = common.PageResponse[MoleculeDTO]

// ─────────────────────────────────────────────────────────────────────────────
// Substructure search request / response
// ─────────────────────────────────────────────────────────────────────────────

// SubstructureSearchRequest is the input DTO for SMARTS-based substructure
// queries executed against the molecule corpus.
type SubstructureSearchRequest struct {
	// SMARTS is the query pattern expressed in SMILES-like SMARTS notation.
	// Example: "c1ccc2[nH]ccc2c1" (indole core).
	SMARTS string `json:"smarts"`

	// MaxResults caps the number of matching molecules returned.
	// Must be between 1 and 10 000; the service layer enforces this range.
	// Defaults to 100 when zero.
	MaxResults int `json:"max_results,omitempty"`
}

// SubstructureSearchResponse is the output DTO for SMARTS-based substructure
// search queries.
type SubstructureSearchResponse struct {
	// Results is the list of molecules whose structures contain the queried
	// SMARTS pattern as a subgraph.
	Results []MoleculeDTO `json:"results"`

	// Total is the total number of matching molecules in the corpus before the
	// MaxResults cap was applied.  Useful for informing callers that results
	// were truncated.
	Total int `json:"total"`
}

