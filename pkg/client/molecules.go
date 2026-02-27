// Phase 13 - SDK Molecules Sub-Client (296/349)
// File: pkg/client/molecules.go
// Molecule search, analysis, property prediction and patent correlation client.

package client

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	kerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

var inchiKeyRe = regexp.MustCompile(`^[A-Z]{14}-[A-Z]{10}-[A-Z]$`)

func invalidArg(msg string) error {
	return fmt.Errorf("%w: %s", kerrors.ErrInvalidArgument, msg)
}

// ---------------------------------------------------------------------------
// DTOs — request / response
// ---------------------------------------------------------------------------

// MoleculeSearchRequest describes a molecule search query.
type MoleculeSearchRequest struct {
	Query       string  `json:"query"`
	QueryType   string  `json:"query_type,omitempty"`
	Similarity  float64 `json:"similarity,omitempty"`
	SearchMode  string  `json:"search_mode,omitempty"`
	Fingerprint string  `json:"fingerprint,omitempty"`
	Radius      int     `json:"radius,omitempty"`
	Page        int     `json:"page,omitempty"`
	PageSize    int     `json:"page_size,omitempty"`
	SortBy      string  `json:"sort_by,omitempty"`
	SortOrder   string  `json:"sort_order,omitempty"`
}

// MoleculeSearchResult is the response envelope for Search.
type MoleculeSearchResult struct {
	Molecules  []Molecule `json:"molecules"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	HasMore    bool       `json:"has_more"`
	SearchTime float64    `json:"search_time"`
}

// Molecule represents a single molecule record.
type Molecule struct {
	ID                string        `json:"id"`
	SMILES            string        `json:"smiles"`
	CanonicalSMILES   string        `json:"canonical_smiles"`
	InChI             string        `json:"inchi"`
	InChIKey          string        `json:"inchi_key"`
	MolecularFormula  string        `json:"molecular_formula"`
	MolecularWeight   float64       `json:"molecular_weight"`
	ExactMass         float64       `json:"exact_mass"`
	LogP              float64       `json:"logp"`
	TPSA              float64       `json:"tpsa"`
	HBondDonors       int           `json:"hbond_donors"`
	HBondAcceptors    int           `json:"hbond_acceptors"`
	RotatableBonds    int           `json:"rotatable_bonds"`
	RingCount         int           `json:"ring_count"`
	AromaticRingCount int           `json:"aromatic_ring_count"`
	HeavyAtomCount    int           `json:"heavy_atom_count"`
	Lipinski          *LipinskiRule `json:"lipinski,omitempty"`
	Similarity        float64       `json:"similarity,omitempty"`
	PatentCount       int           `json:"patent_count"`
	FirstPatentDate   string        `json:"first_patent_date,omitempty"`
	Sources           []string      `json:"sources,omitempty"`
	CreatedAt         string        `json:"created_at"`
	UpdatedAt         string        `json:"updated_at"`
}

// LipinskiRule holds Lipinski's Rule-of-Five evaluation.
type LipinskiRule struct {
	MolecularWeight float64 `json:"molecular_weight"`
	LogP            float64 `json:"logp"`
	HBondDonors     int     `json:"hbond_donors"`
	HBondAcceptors  int     `json:"hbond_acceptors"`
	Violations      int     `json:"violations"`
	Pass            bool    `json:"pass"`
}

// MoleculeDetail extends Molecule with rich metadata.
type MoleculeDetail struct {
	Molecule
	Synonyms         []string       `json:"synonyms,omitempty"`
	CASNumber        string         `json:"cas_number,omitempty"`
	Scaffold         string         `json:"scaffold,omitempty"`
	FunctionalGroups []string       `json:"functional_groups,omitempty"`
	Stereochemistry  string         `json:"stereochemistry,omitempty"`
	Patents          []PatentBrief  `json:"patents,omitempty"`
	Activities       []BioActivity  `json:"activities,omitempty"`
}

// PatentBrief is a compact patent reference attached to a molecule.
type PatentBrief struct {
	PatentNumber string  `json:"patent_number"`
	Title        string  `json:"title"`
	Applicant    string  `json:"applicant"`
	FilingDate   string  `json:"filing_date"`
	Relevance    float64 `json:"relevance"`
}

// BioActivity represents a single bioactivity measurement.
type BioActivity struct {
	Target       string  `json:"target"`
	ActivityType string  `json:"activity_type"`
	Value        float64 `json:"value"`
	Unit         string  `json:"unit"`
	Source       string  `json:"source"`
}

// MoleculePropertyRequest describes a property prediction request.
type MoleculePropertyRequest struct {
	SMILES     string   `json:"smiles"`
	Properties []string `json:"properties"`
}

// MoleculePropertyResult is the response for PredictProperties.
type MoleculePropertyResult struct {
	SMILES                  string                 `json:"smiles"`
	CanonicalSMILES         string                 `json:"canonical_smiles"`
	Properties              map[string]interface{} `json:"properties"`
	Lipinski                *LipinskiRule          `json:"lipinski,omitempty"`
	DrugLikeness            float64                `json:"drug_likeness"`
	SyntheticAccessibility  float64                `json:"synthetic_accessibility"`
}

// BatchSearchRequest describes a batch molecule search.
type BatchSearchRequest struct {
	Molecules  []string `json:"molecules"`
	SearchMode string   `json:"search_mode,omitempty"`
	Similarity float64  `json:"similarity,omitempty"`
}

// BatchSearchResult is the response for BatchSearch.
type BatchSearchResult struct {
	Results        []BatchSearchItem `json:"results"`
	TotalProcessed int               `json:"total_processed"`
	TotalMatched   int               `json:"total_matched"`
	ProcessingTime float64           `json:"processing_time"`
}

// BatchSearchItem holds the result for a single molecule in a batch.
type BatchSearchItem struct {
	QuerySMILES string     `json:"query_smiles"`
	Matches     []Molecule `json:"matches"`
	MatchCount  int        `json:"match_count"`
	Error       string     `json:"error,omitempty"`
}

// MoleculeComparison is the response for CompareMolecules.
type MoleculeComparison struct {
	Molecule1               Molecule              `json:"molecule_1"`
	Molecule2               Molecule              `json:"molecule_2"`
	TanimotoSimilarity      float64               `json:"tanimoto_similarity"`
	DiceSimilarity          float64               `json:"dice_similarity"`
	CommonSubstructure      string                `json:"common_substructure"`
	Molecule1UniqueFragments []string             `json:"molecule_1_unique_fragments"`
	Molecule2UniqueFragments []string             `json:"molecule_2_unique_fragments"`
	PropertyDifferences     map[string][2]float64 `json:"property_differences"`
}

// ---------------------------------------------------------------------------
// Internal response wrappers (for endpoints that return {data:..., meta:...})
// ---------------------------------------------------------------------------

type moleculeDetailResp struct {
	Data MoleculeDetail `json:"data"`
}

type patentBriefListResp struct {
	Data []PatentBrief `json:"data"`
	Meta *ResponseMeta `json:"meta,omitempty"`
}

type bioActivityListResp struct {
	Data []BioActivity `json:"data"`
}

type moleculeComparisonResp struct {
	Data MoleculeComparison `json:"data"`
}

// ---------------------------------------------------------------------------
// MoleculesClient
// ---------------------------------------------------------------------------

// MoleculesClient provides access to molecule-related API endpoints.
type MoleculesClient struct {
	client *Client
}

func newMoleculesClient(c *Client) *MoleculesClient {
	return &MoleculesClient{client: c}
}

// ---------------------------------------------------------------------------
// Public methods
// ---------------------------------------------------------------------------

// Search performs a molecule search (exact, similarity, or substructure).
// POST /api/v1/molecules/search
func (mc *MoleculesClient) Search(ctx context.Context, req *MoleculeSearchRequest) (*MoleculeSearchResult, error) {
	if req == nil || req.Query == "" {
		return nil, invalidArg("query is required")
	}

	// Defaults
	if req.SearchMode == "" {
		req.SearchMode = "exact"
	}
	if req.Similarity == 0 {
		req.Similarity = 0.8
	}
	if req.Fingerprint == "" {
		req.Fingerprint = "morgan"
	}
	if req.Radius == 0 {
		req.Radius = 2
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = DefaultPageSize
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	// Clamp
	if req.PageSize > MaxPageSize {
		req.PageSize = MaxPageSize
	}
	if req.PageSize < 1 {
		req.PageSize = 1
	}

	// Validate
	if req.Similarity < 0 || req.Similarity > 1.0 {
		return nil, invalidArg("similarity must be between 0.0 and 1.0")
	}

	var result MoleculeSearchResult
	if err := mc.client.post(ctx, "/api/v1/molecules/search", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Get retrieves a molecule by its internal ID.
// GET /api/v1/molecules/{moleculeID}
func (mc *MoleculesClient) Get(ctx context.Context, moleculeID string) (*MoleculeDetail, error) {
	if moleculeID == "" {
		return nil, invalidArg("moleculeID is required")
	}
	var resp moleculeDetailResp
	if err := mc.client.get(ctx, "/api/v1/molecules/"+moleculeID, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetBySMILES retrieves a molecule by its SMILES string.
// GET /api/v1/molecules/by-smiles?smiles={url_encoded_smiles}
func (mc *MoleculesClient) GetBySMILES(ctx context.Context, smiles string) (*MoleculeDetail, error) {
	if smiles == "" {
		return nil, invalidArg("smiles is required")
	}
	path := "/api/v1/molecules/by-smiles?smiles=" + url.QueryEscape(smiles)
	var resp moleculeDetailResp
	if err := mc.client.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// GetByInChIKey retrieves a molecule by its InChIKey.
// GET /api/v1/molecules/by-inchikey?inchikey={inchiKey}
func (mc *MoleculesClient) GetByInChIKey(ctx context.Context, inchiKey string) (*MoleculeDetail, error) {
	if inchiKey == "" {
		return nil, invalidArg("inchiKey is required")
	}
	if !inchiKeyRe.MatchString(inchiKey) {
		return nil, invalidArg("inchiKey format invalid, expected XXXXXXXXXXXXXX-XXXXXXXXXX-X (uppercase)")
	}
	path := "/api/v1/molecules/by-inchikey?inchikey=" + url.QueryEscape(inchiKey)
	var resp moleculeDetailResp
	if err := mc.client.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// PredictProperties predicts molecular properties for a given SMILES.
// POST /api/v1/molecules/predict
func (mc *MoleculesClient) PredictProperties(ctx context.Context, req *MoleculePropertyRequest) (*MoleculePropertyResult, error) {
	if req == nil || req.SMILES == "" {
		return nil, invalidArg("smiles is required")
	}
	if len(req.Properties) == 0 {
		return nil, invalidArg("properties list is required")
	}
	var result MoleculePropertyResult
	if err := mc.client.post(ctx, "/api/v1/molecules/predict", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BatchSearch performs a batch molecule search.
// POST /api/v1/molecules/batch-search
func (mc *MoleculesClient) BatchSearch(ctx context.Context, req *BatchSearchRequest) (*BatchSearchResult, error) {
	if req == nil || len(req.Molecules) == 0 {
		return nil, invalidArg("molecules list is required")
	}
	if len(req.Molecules) > 1000 {
		return nil, invalidArg("molecules list exceeds maximum of 1000 entries")
	}
	if req.SearchMode == "" {
		req.SearchMode = "exact"
	}
	if req.Similarity == 0 {
		req.Similarity = 0.8
	}
	var result BatchSearchResult
	if err := mc.client.post(ctx, "/api/v1/molecules/batch-search", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetPatents returns patents associated with a molecule.
// GET /api/v1/molecules/{moleculeID}/patents?page={page}&page_size={pageSize}
func (mc *MoleculesClient) GetPatents(ctx context.Context, moleculeID string, page, pageSize int) ([]PatentBrief, int64, error) {
	if moleculeID == "" {
		return nil, 0, invalidArg("moleculeID is required")
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	path := fmt.Sprintf("/api/v1/molecules/%s/patents?page=%d&page_size=%d", moleculeID, page, pageSize)
	var resp patentBriefListResp
	if err := mc.client.get(ctx, path, &resp); err != nil {
		return nil, 0, err
	}
	var total int64
	if resp.Meta != nil {
		total = resp.Meta.Total
	}
	return resp.Data, total, nil
}

// GetActivities returns bioactivity data for a molecule.
// GET /api/v1/molecules/{moleculeID}/activities
func (mc *MoleculesClient) GetActivities(ctx context.Context, moleculeID string) ([]BioActivity, error) {
	if moleculeID == "" {
		return nil, invalidArg("moleculeID is required")
	}
	var resp bioActivityListResp
	if err := mc.client.get(ctx, "/api/v1/molecules/"+moleculeID+"/activities", &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// CompareMolecules compares two molecules by SMILES.
// POST /api/v1/molecules/compare
func (mc *MoleculesClient) CompareMolecules(ctx context.Context, smiles1, smiles2 string) (*MoleculeComparison, error) {
	if smiles1 == "" {
		return nil, invalidArg("smiles1 is required")
	}
	if smiles2 == "" {
		return nil, invalidArg("smiles2 is required")
	}
	body := map[string]string{
		"smiles_1": smiles1,
		"smiles_2": smiles2,
	}
	var resp moleculeComparisonResp
	if err := mc.client.post(ctx, "/api/v1/molecules/compare", body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

//Personal.AI order the ending
